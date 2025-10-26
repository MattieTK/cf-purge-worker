package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cloudflare/cf-delete-worker/pkg/types"
	"github.com/cloudflare/cloudflare-go"
)

// Client wraps the Cloudflare API client
type Client struct {
	cf        *cloudflare.API
	apiToken  string
	accountID string
	ctx       context.Context
}

// NewClient creates a new Cloudflare API client
func NewClient(apiToken, accountID string) (*Client, error) {
	cf, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloudflare client: %w", err)
	}

	return &Client{
		cf:        cf,
		apiToken:  apiToken,
		accountID: accountID,
		ctx:       context.Background(),
	}, nil
}

// GetAccountID retrieves the account ID if not provided
func (c *Client) GetAccountID() (string, error) {
	if c.accountID != "" {
		return c.accountID, nil
	}

	// List accounts and let user select
	accounts, _, err := c.cf.Accounts(c.ctx, cloudflare.AccountsListParams{})
	if err != nil {
		return "", fmt.Errorf("failed to list accounts: %w", err)
	}

	if len(accounts) == 0 {
		return "", fmt.Errorf("no accounts found")
	}

	if len(accounts) == 1 {
		c.accountID = accounts[0].ID
		return c.accountID, nil
	}

	// Multiple accounts - would need interactive selection
	// For now, return error and ask user to specify
	return "", fmt.Errorf("multiple accounts found, please specify --account-id")
}

// ListWorkers lists all workers in the account
func (c *Client) ListWorkers() ([]types.WorkerInfo, error) {
	rc := cloudflare.AccountIdentifier(c.accountID)

	params := cloudflare.ListWorkersParams{}
	workers, _, err := c.cf.ListWorkers(c.ctx, rc, params)
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	var result []types.WorkerInfo
	for _, w := range workers.WorkerList {
		result = append(result, types.WorkerInfo{
			Name:       w.ID,
			AccountID:  c.accountID,
			CreatedOn:  w.CreatedOn,
			ModifiedOn: w.ModifiedOn,
		})
	}

	return result, nil
}

// GetWorker retrieves details about a specific worker
func (c *Client) GetWorker(name string) (*types.WorkerInfo, error) {
	// First, verify the worker exists by listing all workers
	workers, err := c.ListWorkers()
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	var foundWorker *types.WorkerInfo
	for _, w := range workers {
		if w.Name == name {
			foundWorker = &w
			break
		}
	}

	if foundWorker == nil {
		return nil, fmt.Errorf("worker not found: %s", name)
	}

	// Get bindings from the settings endpoint
	bindings, err := c.GetWorkerBindings(name)
	if err != nil {
		// If we can't get bindings, continue with empty list
		// This allows the tool to still work for basic worker deletion
		foundWorker.Bindings = []types.Binding{}
	} else {
		foundWorker.Bindings = bindings
	}

	return foundWorker, nil
}

// GetWorkerBindings retrieves bindings for a worker using the settings endpoint
// This endpoint returns all binding information for a worker script
// See: https://developers.cloudflare.com/api/resources/workers/subresources/scripts/subresources/script_and_version_settings/methods/get/
func (c *Client) GetWorkerBindings(scriptName string) ([]types.Binding, error) {
	// Use the settings endpoint to get all bindings
	// GET /accounts/:account_id/workers/scripts/:script_name/settings
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/workers/scripts/%s/settings",
		c.accountID, scriptName)

	// Create HTTP request
	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication header
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get worker settings: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse JSON response
	var response struct {
		Result struct {
			Bindings []map[string]interface{} `json:"bindings"`
		} `json:"result"`
		Success bool              `json:"success"`
		Errors  []json.RawMessage `json:"errors"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !response.Success {
		return nil, fmt.Errorf("API request failed: %v", response.Errors)
	}

	// Parse bindings from the response
	var bindings []types.Binding
	for _, b := range response.Result.Bindings {
		binding := c.parseBinding(b)
		if binding != nil {
			bindings = append(bindings, *binding)
		}
	}

	return bindings, nil
}

// parseBinding converts a raw binding map to a typed Binding struct
func (c *Client) parseBinding(raw map[string]interface{}) *types.Binding {
	bindingType, ok := raw["type"].(string)
	if !ok {
		return nil
	}

	name, _ := raw["name"].(string)
	binding := &types.Binding{
		Name: name,
		Type: types.BindingType(bindingType),
	}

	// Parse type-specific fields
	switch bindingType {
	case "kv_namespace":
		if namespaceID, ok := raw["namespace_id"].(string); ok {
			binding.NamespaceID = namespaceID
		}

	case "r2_bucket":
		if bucketName, ok := raw["bucket_name"].(string); ok {
			binding.BucketName = bucketName
		}

	case "d1":
		if id, ok := raw["id"].(string); ok {
			binding.DatabaseID = id
		}

	case "durable_object_namespace":
		if className, ok := raw["class_name"].(string); ok {
			binding.ClassName = className
		}
		if scriptName, ok := raw["script_name"].(string); ok {
			binding.ScriptName = scriptName
		}

	case "service":
		if service, ok := raw["service"].(string); ok {
			binding.ScriptName = service
		}

	case "queue":
		if queueName, ok := raw["queue_name"].(string); ok {
			binding.QueueName = queueName
		}

	case "plain_text":
		binding.Type = types.BindingTypeEnvVar

	case "secret_text":
		binding.Type = types.BindingTypeSecret
	}

	return binding
}

// DeleteWorker deletes a worker script
func (c *Client) DeleteWorker(name string) error {
	rc := cloudflare.AccountIdentifier(c.accountID)

	params := cloudflare.DeleteWorkerParams{
		ScriptName: name,
	}

	if err := c.cf.DeleteWorker(c.ctx, rc, params); err != nil {
		return fmt.Errorf("failed to delete worker: %w", err)
	}

	return nil
}

// DeleteKVNamespace deletes a KV namespace
func (c *Client) DeleteKVNamespace(namespaceID string) error {
	rc := cloudflare.AccountIdentifier(c.accountID)

	_, err := c.cf.DeleteWorkersKVNamespace(c.ctx, rc, namespaceID)
	if err != nil {
		return fmt.Errorf("failed to delete KV namespace: %w", err)
	}

	return nil
}

// DeleteR2Bucket deletes an R2 bucket
func (c *Client) DeleteR2Bucket(bucketName string) error {
	rc := cloudflare.AccountIdentifier(c.accountID)

	if err := c.cf.DeleteR2Bucket(c.ctx, rc, bucketName); err != nil {
		return fmt.Errorf("failed to delete R2 bucket: %w", err)
	}

	return nil
}

// DeleteD1Database deletes a D1 database
func (c *Client) DeleteD1Database(databaseID string) error {
	rc := cloudflare.AccountIdentifier(c.accountID)

	if err := c.cf.DeleteD1Database(c.ctx, rc, databaseID); err != nil {
		return fmt.Errorf("failed to delete D1 database: %w", err)
	}

	return nil
}

// GetKVNamespaceTitle gets the title/name of a KV namespace
func (c *Client) GetKVNamespaceTitle(namespaceID string) (string, error) {
	rc := cloudflare.AccountIdentifier(c.accountID)

	namespaces, _, err := c.cf.ListWorkersKVNamespaces(c.ctx, rc, cloudflare.ListWorkersKVNamespacesParams{})
	if err != nil {
		return "", err
	}

	for _, ns := range namespaces {
		if ns.ID == namespaceID {
			return ns.Title, nil
		}
	}

	return "", fmt.Errorf("namespace not found")
}

// GetD1DatabaseName gets the name of a D1 database
func (c *Client) GetD1DatabaseName(databaseID string) (string, error) {
	rc := cloudflare.AccountIdentifier(c.accountID)

	databases, _, err := c.cf.ListD1Databases(c.ctx, rc, cloudflare.ListD1DatabasesParams{})
	if err != nil {
		return "", err
	}

	for _, db := range databases {
		if db.UUID == databaseID {
			return db.Name, nil
		}
	}

	return "", fmt.Errorf("database not found")
}

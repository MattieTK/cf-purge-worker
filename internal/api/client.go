package api

import (
	"context"
	"fmt"

	"github.com/cloudflare/cf-delete-worker/pkg/types"
	"github.com/cloudflare/cloudflare-go"
)

// Client wraps the Cloudflare API client
type Client struct {
	cf        *cloudflare.API
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

	// Try to get bindings (may fail, that's ok for MVP)
	bindings, err := c.GetWorkerBindings(name)
	if err != nil {
		// For MVP, we'll just have empty bindings if we can't fetch them
		foundWorker.Bindings = []types.Binding{}
	} else {
		foundWorker.Bindings = bindings
	}

	return foundWorker, nil
}

// GetWorkerBindings retrieves bindings for a worker
// Note: This is currently not implemented as it requires undocumented API endpoints
// For MVP, we'll focus on worker deletion only
func (c *Client) GetWorkerBindings(scriptName string) ([]types.Binding, error) {
	// TODO: Implement binding retrieval when proper API is available
	// For now, return empty list so the tool can still work for basic worker deletion
	return []types.Binding{}, nil
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

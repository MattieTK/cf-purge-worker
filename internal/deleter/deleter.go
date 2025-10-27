package deleter

import (
	"fmt"

	"github.com/cloudflare/cf-purge-worker/internal/api"
	"github.com/cloudflare/cf-purge-worker/pkg/types"
)

// Deleter handles deletion operations
type Deleter struct {
	client *api.Client
	dryRun bool
}

// NewDeleter creates a new deleter
func NewDeleter(client *api.Client, dryRun bool) *Deleter {
	return &Deleter{
		client: client,
		dryRun: dryRun,
	}
}

// Execute executes the deletion plan
func (d *Deleter) Execute(plan *types.DeletionPlan) (*types.DeletionResult, error) {
	result := &types.DeletionResult{
		Success:          true,
		WorkerDeleted:    false,
		ResourcesDeleted: []string{},
		ResourcesSkipped: []string{},
		Errors:           []error{},
	}

	if d.dryRun {
		// In dry-run mode, just simulate
		result.WorkerDeleted = true
		for _, resource := range plan.ResourcesToDelete {
			result.ResourcesDeleted = append(result.ResourcesDeleted, resource.ResourceName)
		}
		return result, nil
	}

	// Step 1: Delete the worker script
	if err := d.client.DeleteWorker(plan.Worker.Name); err != nil {
		result.Success = false
		result.Errors = append(result.Errors, fmt.Errorf("failed to delete worker: %w", err))
		return result, err
	}
	result.WorkerDeleted = true

	// Step 2: Delete resources
	for _, resource := range plan.ResourcesToDelete {
		// Skip shared resources if we're not supposed to delete them
		if !plan.DeleteShared && resource.RiskLevel != types.RiskLevelSafe {
			result.ResourcesSkipped = append(result.ResourcesSkipped, resource.ResourceName)
			continue
		}

		if err := d.deleteResource(resource); err != nil {
			result.Errors = append(result.Errors, err)
			result.ResourcesSkipped = append(result.ResourcesSkipped, resource.ResourceName)
			// Continue with other resources even if one fails
		} else {
			result.ResourcesDeleted = append(result.ResourcesDeleted, resource.ResourceName)
		}
	}

	// If any errors occurred, mark as not successful
	if len(result.Errors) > 0 {
		result.Success = false
	}

	return result, nil
}

// deleteResource deletes a specific resource based on its type
func (d *Deleter) deleteResource(resource types.ResourceUsage) error {
	switch resource.ResourceType {
	case types.BindingTypeKV:
		return d.client.DeleteKVNamespace(resource.ResourceID)

	case types.BindingTypeR2:
		return d.client.DeleteR2Bucket(resource.ResourceID)

	case types.BindingTypeD1:
		return d.client.DeleteD1Database(resource.ResourceID)

	case types.BindingTypeDurableObject:
		// Durable Objects are defined in worker scripts, not separate resources
		// No deletion needed
		return nil

	case types.BindingTypeService:
		// Service bindings point to other workers, don't delete
		return nil

	case types.BindingTypeQueue:
		// Queue deletion would require additional API calls
		// Skip for now
		return nil

	default:
		return fmt.Errorf("unsupported resource type: %s", resource.ResourceType)
	}
}

// DeleteWorkerOnly deletes only the worker script, not resources
func (d *Deleter) DeleteWorkerOnly(workerName string) error {
	if d.dryRun {
		return nil
	}
	return d.client.DeleteWorker(workerName)
}

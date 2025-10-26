package analyzer

import (
	"fmt"

	"github.com/cloudflare/cf-delete-worker/internal/api"
	"github.com/cloudflare/cf-delete-worker/pkg/types"
)

// ProgressCallback is called during analysis to report progress
type ProgressCallback func(current, total int, workerName string)

// Analyzer analyzes worker dependencies
type Analyzer struct {
	client *api.Client
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer(client *api.Client) *Analyzer {
	return &Analyzer{
		client: client,
	}
}

// AnalyzeDependencies analyzes which workers depend on which resources
func (a *Analyzer) AnalyzeDependencies(targetWorker *types.WorkerInfo, progressCallback ...ProgressCallback) ([]types.ResourceUsage, error) {
	// Get callback if provided
	var callback ProgressCallback
	if len(progressCallback) > 0 {
		callback = progressCallback[0]
	}

	// Get all workers in the account
	allWorkers, err := a.client.ListWorkers()
	if err != nil {
		return nil, fmt.Errorf("failed to list workers: %w", err)
	}

	totalWorkers := len(allWorkers)

	// Build a map of resources to workers that use them
	resourceMap := make(map[string]*types.ResourceUsage)

	// Process all workers to find resource usage
	for i, worker := range allWorkers {
		// Report progress if callback is provided
		if callback != nil {
			callback(i+1, totalWorkers, worker.Name)
		}

		// Get full worker details with bindings
		fullWorker, err := a.client.GetWorker(worker.Name)
		if err != nil {
			// Skip workers we can't read
			continue
		}

		// Process each binding
		for _, binding := range fullWorker.Bindings {
			resourceKey := a.getResourceKey(binding)
			if resourceKey == "" {
				continue
			}

			// Initialize resource usage if not exists
			if _, exists := resourceMap[resourceKey]; !exists {
				resourceMap[resourceKey] = &types.ResourceUsage{
					ResourceID:   a.getResourceID(binding),
					ResourceType: binding.Type,
					ResourceName: a.getResourceName(binding),
					UsedBy:       []string{},
				}
			}

			// Add this worker to the list of users
			resourceMap[resourceKey].UsedBy = append(resourceMap[resourceKey].UsedBy, worker.Name)
		}
	}

	// Now build the list of resources used by the target worker
	var result []types.ResourceUsage

	for _, binding := range targetWorker.Bindings {
		resourceKey := a.getResourceKey(binding)
		if resourceKey == "" {
			continue
		}

		usage, exists := resourceMap[resourceKey]
		if !exists {
			// Resource not tracked, create a minimal entry
			usage = &types.ResourceUsage{
				ResourceID:   a.getResourceID(binding),
				ResourceType: binding.Type,
				ResourceName: a.getResourceName(binding),
				UsedBy:       []string{targetWorker.Name},
			}
		}

		// Enrich with names if needed
		usage.ResourceName = a.enrichResourceName(binding, usage.ResourceName)

		// Calculate risk level
		usage.RiskLevel = a.calculateRiskLevel(usage.UsedBy, targetWorker.Name)

		result = append(result, *usage)
	}

	return result, nil
}

// getResourceKey returns a unique key for a resource
func (a *Analyzer) getResourceKey(binding types.Binding) string {
	switch binding.Type {
	case types.BindingTypeKV:
		return fmt.Sprintf("kv:%s", binding.NamespaceID)
	case types.BindingTypeR2:
		return fmt.Sprintf("r2:%s", binding.BucketName)
	case types.BindingTypeD1:
		return fmt.Sprintf("d1:%s", binding.DatabaseID)
	case types.BindingTypeDurableObject:
		return fmt.Sprintf("do:%s:%s", binding.ClassName, binding.ScriptName)
	case types.BindingTypeService:
		return fmt.Sprintf("service:%s", binding.ScriptName)
	case types.BindingTypeQueue:
		return fmt.Sprintf("queue:%s", binding.QueueName)
	default:
		return ""
	}
}

// getResourceID returns the resource ID
func (a *Analyzer) getResourceID(binding types.Binding) string {
	switch binding.Type {
	case types.BindingTypeKV:
		return binding.NamespaceID
	case types.BindingTypeR2:
		return binding.BucketName
	case types.BindingTypeD1:
		return binding.DatabaseID
	case types.BindingTypeDurableObject:
		return binding.ClassName
	case types.BindingTypeService:
		return binding.ScriptName
	case types.BindingTypeQueue:
		return binding.QueueName
	default:
		return binding.Name
	}
}

// getResourceName returns a display name for the resource
func (a *Analyzer) getResourceName(binding types.Binding) string {
	switch binding.Type {
	case types.BindingTypeKV:
		return binding.Name // Will enrich later
	case types.BindingTypeR2:
		return binding.BucketName
	case types.BindingTypeD1:
		return binding.DatabaseName
	case types.BindingTypeDurableObject:
		return binding.ClassName
	case types.BindingTypeService:
		return binding.ScriptName
	case types.BindingTypeQueue:
		return binding.QueueName
	default:
		return binding.Name
	}
}

// enrichResourceName fetches the actual resource name from the API
func (a *Analyzer) enrichResourceName(binding types.Binding, currentName string) string {
	if currentName != "" && currentName != binding.Name {
		return currentName
	}

	switch binding.Type {
	case types.BindingTypeKV:
		if title, err := a.client.GetKVNamespaceTitle(binding.NamespaceID); err == nil {
			return title
		}
	case types.BindingTypeD1:
		if name, err := a.client.GetD1DatabaseName(binding.DatabaseID); err == nil {
			return name
		}
	}

	return currentName
}

// calculateRiskLevel determines the risk level based on usage
func (a *Analyzer) calculateRiskLevel(usedBy []string, targetWorker string) types.RiskLevel {
	// Count other workers (excluding the target)
	otherCount := 0
	for _, worker := range usedBy {
		if worker != targetWorker {
			otherCount++
		}
	}

	if otherCount == 0 {
		return types.RiskLevelSafe // Exclusive to this worker
	} else if otherCount <= 2 {
		return types.RiskLevelCaution // Used by 1-2 other workers
	}
	return types.RiskLevelDanger // Used by 3+ workers
}

// CreateDeletionPlan creates a deletion plan based on analysis
func (a *Analyzer) CreateDeletionPlan(worker *types.WorkerInfo, resources []types.ResourceUsage, exclusiveOnly bool) *types.DeletionPlan {
	plan := &types.DeletionPlan{
		Worker:              *worker,
		ResourcesToDelete:   []types.ResourceUsage{},
		HasSharedResources:  false,
		DeleteExclusiveOnly: exclusiveOnly,
	}

	for _, resource := range resources {
		if resource.RiskLevel != types.RiskLevelSafe {
			plan.HasSharedResources = true
		}

		// If exclusive only mode, skip shared resources
		if exclusiveOnly && resource.RiskLevel != types.RiskLevelSafe {
			continue
		}

		plan.ResourcesToDelete = append(plan.ResourcesToDelete, resource)
	}

	return plan
}

package views

import (
	"fmt"
	"strings"

	"github.com/cloudflare/cf-purge-worker/internal/ui/styles"
	"github.com/cloudflare/cf-purge-worker/pkg/types"
)

// RenderHeader renders the application header
func RenderHeader() string {
	var b strings.Builder
	b.WriteString(styles.Header.Render("☁️  cf-purge-worker"))
	b.WriteString("\n")
	b.WriteString(styles.Subtitle.Render("Safely delete Cloudflare Workers and resources"))
	b.WriteString("\n")
	return b.String()
}

// RenderWorkerInfo renders worker information
func RenderWorkerInfo(worker *types.WorkerInfo) string {
	var b strings.Builder

	b.WriteString(styles.Section.Render("📦 Worker Details"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Name: %s\n", styles.Highlight.Render(worker.Name)))

	if !worker.CreatedOn.IsZero() {
		b.WriteString(fmt.Sprintf("  Created: %s\n", styles.Info.Render(worker.CreatedOn.Format("2006-01-02"))))
	}
	if !worker.ModifiedOn.IsZero() {
		b.WriteString(fmt.Sprintf("  Modified: %s\n", styles.Info.Render(worker.ModifiedOn.Format("2006-01-02"))))
	}

	b.WriteString(fmt.Sprintf("  Bindings: %s\n", styles.Info.Render(fmt.Sprintf("%d", len(worker.Bindings)))))
	b.WriteString("\n")

	return b.String()
}

// RenderDeletionPlan renders the deletion plan
func RenderDeletionPlan(plan *types.DeletionPlan) string {
	var b strings.Builder

	b.WriteString(styles.Box.Render(buildDeletionPlanContent(plan)))
	return b.String()
}

func buildDeletionPlanContent(plan *types.DeletionPlan) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Deletion Plan"))
	b.WriteString("\n\n")

	// Worker info
	b.WriteString(fmt.Sprintf("Worker: %s\n", styles.Highlight.Render(plan.Worker.Name)))
	if !plan.Worker.ModifiedOn.IsZero() {
		b.WriteString(fmt.Sprintf("Last Modified: %s\n", plan.Worker.ModifiedOn.Format("2006-01-02")))
	}
	b.WriteString("\n")

	// Group resources by type
	resourcesByType := groupResourcesByType(plan.ResourcesToDelete)

	if len(resourcesByType) == 0 {
		b.WriteString(styles.Muted.Render("No resources to delete"))
		b.WriteString("\n")
	} else {
		b.WriteString(styles.Section.Render("Resources to Delete:"))
		b.WriteString("\n\n")

		for resourceType, resources := range resourcesByType {
			b.WriteString(fmt.Sprintf("%s (%d):\n", styles.FormatResourceType(string(resourceType)), len(resources)))
			for _, resource := range resources {
				indicator := getRiskIndicator(resource.RiskLevel)
				b.WriteString(fmt.Sprintf("  %s %s", indicator, resource.ResourceName))

				// Show which other workers use this
				if resource.RiskLevel != types.RiskLevelSafe {
					otherWorkers := getOtherWorkers(resource.UsedBy, plan.Worker.Name)
					if len(otherWorkers) > 0 {
						b.WriteString(fmt.Sprintf(" %s", styles.Warning.Render(fmt.Sprintf("(used by %d other worker(s))", len(otherWorkers)))))
					}
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	// Warnings
	if plan.HasSharedResources {
		b.WriteString(styles.Warning.Render("⚠️  Warning: "))
		sharedCount := countSharedResources(plan.ResourcesToDelete)
		b.WriteString(fmt.Sprintf("%d shared resource(s) detected\n", sharedCount))
	}

	return b.String()
}

// RenderProgress renders a progress message
func RenderProgress(message string) string {
	return styles.Info.Render(fmt.Sprintf("⏳ %s...", message))
}

// RenderSuccess renders a success message
func RenderSuccess(message string) string {
	return styles.Success.Render(fmt.Sprintf("✓ %s", message))
}

// RenderError renders an error message
func RenderError(message string) string {
	return styles.Error.Render(fmt.Sprintf("✗ %s", message))
}

// RenderWarning renders a warning message
func RenderWarning(message string) string {
	return styles.Warning.Render(fmt.Sprintf("⚠️  %s", message))
}

// RenderDeletionResult renders the deletion result
func RenderDeletionResult(result *types.DeletionResult) string {
	var b strings.Builder

	if result.Success {
		b.WriteString(styles.SuccessBox.Render(buildSuccessContent(result)))
	} else {
		b.WriteString(styles.DangerBox.Render(buildErrorContent(result)))
	}

	return b.String()
}

func buildSuccessContent(result *types.DeletionResult) string {
	var b strings.Builder

	b.WriteString(styles.Success.Render("✨ Deletion Complete!"))
	b.WriteString("\n\n")

	if result.WorkerDeleted {
		b.WriteString("✓ Worker script deleted\n")
	}

	if len(result.ResourcesDeleted) > 0 {
		b.WriteString(fmt.Sprintf("✓ %d resource(s) deleted\n", len(result.ResourcesDeleted)))
	}

	if len(result.ResourcesSkipped) > 0 {
		b.WriteString(fmt.Sprintf("⊗ %d resource(s) preserved (shared)\n", len(result.ResourcesSkipped)))
	}

	return b.String()
}

func buildErrorContent(result *types.DeletionResult) string {
	var b strings.Builder

	b.WriteString(styles.Error.Render("✗ Deletion Failed"))
	b.WriteString("\n\n")

	if result.WorkerDeleted {
		b.WriteString("✓ Worker script deleted\n")
	} else {
		b.WriteString("✗ Worker script not deleted\n")
	}

	if len(result.ResourcesDeleted) > 0 {
		b.WriteString(fmt.Sprintf("✓ %d resource(s) deleted\n", len(result.ResourcesDeleted)))
	}

	if len(result.ResourcesSkipped) > 0 {
		b.WriteString(fmt.Sprintf("⊗ %d resource(s) skipped\n", len(result.ResourcesSkipped)))
	}

	if len(result.Errors) > 0 {
		b.WriteString("\nErrors:\n")
		for _, err := range result.Errors {
			b.WriteString(fmt.Sprintf("  • %s\n", err.Error()))
		}
	}

	return b.String()
}

// Helper functions

func groupResourcesByType(resources []types.ResourceUsage) map[types.BindingType][]types.ResourceUsage {
	grouped := make(map[types.BindingType][]types.ResourceUsage)
	for _, resource := range resources {
		grouped[resource.ResourceType] = append(grouped[resource.ResourceType], resource)
	}
	return grouped
}

func getRiskIndicator(level types.RiskLevel) string {
	switch level {
	case types.RiskLevelSafe:
		return styles.RiskIndicator("safe")
	case types.RiskLevelCaution:
		return styles.RiskIndicator("caution")
	case types.RiskLevelDanger:
		return styles.RiskIndicator("danger")
	default:
		return styles.RiskIndicator("")
	}
}

func getOtherWorkers(usedBy []string, currentWorker string) []string {
	var others []string
	for _, worker := range usedBy {
		if worker != currentWorker {
			others = append(others, worker)
		}
	}
	return others
}

func countSharedResources(resources []types.ResourceUsage) int {
	count := 0
	for _, resource := range resources {
		if resource.RiskLevel != types.RiskLevelSafe {
			count++
		}
	}
	return count
}

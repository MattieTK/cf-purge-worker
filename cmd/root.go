package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudflare/cf-delete-worker/internal/analyzer"
	"github.com/cloudflare/cf-delete-worker/internal/api"
	"github.com/cloudflare/cf-delete-worker/internal/auth"
	"github.com/cloudflare/cf-delete-worker/internal/deleter"
	"github.com/cloudflare/cf-delete-worker/internal/ui/models"
	"github.com/cloudflare/cf-delete-worker/internal/ui/views"
	"github.com/cloudflare/cf-delete-worker/pkg/types"
	"github.com/spf13/cobra"
)

var (
	config types.Config
	rootCmd = &cobra.Command{
		Use:   "cf-delete-worker [worker-name]",
		Short: "Safely delete Cloudflare Workers and their resources",
		Long: `cf-delete-worker is a CLI tool for safely deleting Cloudflare Workers
and their associated resources (KV namespaces, R2 buckets, D1 databases, etc.)
while preventing accidental deletion of shared resources.`,
		Version: "0.1.0",
		Args:    cobra.ExactArgs(1),
		RunE:    run,
	}
)

func init() {
	rootCmd.Flags().StringVar(&config.AccountID, "account-id", "", "Cloudflare account ID")
	rootCmd.Flags().BoolVarP(&config.DryRun, "dry-run", "d", false, "Show deletion plan without executing")
	rootCmd.Flags().BoolVarP(&config.Force, "force", "f", false, "Skip confirmation prompts (dangerous)")
	rootCmd.Flags().BoolVar(&config.ExclusiveOnly, "exclusive-only", false, "Only delete resources not shared with other workers")
	rootCmd.Flags().BoolVarP(&config.AutoYes, "yes", "y", false, "Answer yes to all prompts")
	rootCmd.Flags().BoolVarP(&config.Verbose, "verbose", "v", false, "Verbose logging")
	rootCmd.Flags().BoolVarP(&config.Quiet, "quiet", "q", false, "Minimal output")
	rootCmd.Flags().BoolVar(&config.JSONOutput, "json", false, "Output results in JSON format")

	// Hidden flag for updating API key
	var updateKey bool
	rootCmd.Flags().BoolVar(&updateKey, "update-key", false, "Update stored API key")
	rootCmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if updateKey {
			authMgr := auth.NewManager()
			return authMgr.UpdateAPIKey()
		}
		return nil
	}
}

func run(cmd *cobra.Command, args []string) error {
	workerName := args[0]

	// Get API key
	authMgr := auth.NewManager()
	apiKey, err := authMgr.GetAPIKey()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	config.APIKey = apiKey

	// Create API client
	client, err := api.NewClient(apiKey, config.AccountID)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Get account ID if not provided
	if config.AccountID == "" {
		accountID, err := client.GetAccountID()
		if err != nil {
			return err
		}
		config.AccountID = accountID
	}

	// Show progress
	if !config.Quiet {
		fmt.Println(views.RenderHeader())
		fmt.Println(views.RenderProgress(fmt.Sprintf("Analyzing worker: %s", workerName)))
	}

	// Get worker details
	worker, err := client.GetWorker(workerName)
	if err != nil {
		return fmt.Errorf("failed to get worker: %w", err)
	}

	if !config.Quiet {
		fmt.Println(views.RenderSuccess("Worker found"))
	}

	// Create analyzer and deleter
	a := analyzer.NewAnalyzer(client)
	d := deleter.NewDeleter(client, config.DryRun)

	// Interactive mode - run analysis inside TUI
	if !config.Force && !config.AutoYes && !config.DryRun && !config.JSONOutput {
		p := tea.NewProgram(models.NewModelWithAnalysis(worker, a, &config, d))
		finalModel, err := p.Run()
		if err != nil {
			return fmt.Errorf("UI error: %w", err)
		}

		// Check the final state
		m := finalModel.(models.Model)

		// If there's an error, return it
		if m.Err != nil {
			return fmt.Errorf("deletion failed: %w", m.Err)
		}

		// If result exists but not successful, exit with error
		if m.Result != nil && !m.Result.Success {
			os.Exit(1)
		}

		return nil
	}

	// Non-interactive mode - run analysis with progress indicators
	if !config.Quiet {
		fmt.Println(views.RenderProgress("Analyzing dependencies"))
	}

	var resources []types.ResourceUsage

	if !config.Quiet && !config.JSONOutput {
		// Show progress during analysis
		resources, err = a.AnalyzeDependencies(worker, func(current, total int, workerName string) {
			// Use ANSI escape code to clear the line instead of hardcoded padding
			fmt.Printf("\r\033[K%s Analyzing workers: %d/%d (%s)",
				views.RenderProgress(""),
				current, total, workerName)
			if current == total {
				fmt.Println() // New line when done
			}
		})
	} else {
		// No progress callback for quiet/JSON mode
		resources, err = a.AnalyzeDependencies(worker)
	}

	if err != nil {
		return fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	if !config.Quiet {
		fmt.Println(views.RenderSuccess(fmt.Sprintf("Found %d resource(s)", len(resources))))
		fmt.Println()
	}

	// Create deletion plan
	plan := a.CreateDeletionPlan(worker, resources, config.ExclusiveOnly)

	// If JSON output, print and exit
	if config.JSONOutput {
		return outputJSON(plan)
	}

	// In dry-run mode, just show the plan
	if config.DryRun {
		fmt.Println(views.RenderDeletionPlan(plan))
		fmt.Println(views.RenderWarning("DRY RUN - No changes were made"))
		return nil
	}

	// Non-interactive deletion mode
	if !config.Quiet {
		fmt.Println(views.RenderProgress("Deleting resources"))
	}

	// Set deletion flags based on config
	if config.ExclusiveOnly {
		plan.DeleteShared = false
	} else if config.Force || config.AutoYes {
		plan.DeleteShared = true
	}

	result, err := d.Execute(plan)
	if err != nil {
		return fmt.Errorf("deletion failed: %w", err)
	}

	// Show result
	if !config.Quiet {
		fmt.Println(views.RenderDeletionResult(result))
	}

	if !result.Success {
		os.Exit(1)
	}

	return nil
}

func outputJSON(plan *types.DeletionPlan) error {
	// TODO: Implement JSON output
	fmt.Println("JSON output not yet implemented")
	return nil
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

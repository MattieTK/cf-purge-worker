package models

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattietk/cf-purge-worker/internal/analyzer"
	"github.com/mattietk/cf-purge-worker/internal/deleter"
	"github.com/mattietk/cf-purge-worker/internal/ui/views"
	"github.com/mattietk/cf-purge-worker/pkg/types"
)

type sessionState int

const (
	stateLoading sessionState = iota
	stateConfirmDependencyCheck
	stateAnalyzing
	stateShowPlan
	stateConfirmDeletion
	stateConfirmShared
	stateDeleting
	stateComplete
	stateError
)

// progressTracker safely tracks analysis progress across goroutines
type progressTracker struct {
	mu         sync.RWMutex
	current    int
	total      int
	workerName string
}

func (p *progressTracker) update(current, total int, workerName string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = current
	p.total = total
	p.workerName = workerName
}

func (p *progressTracker) get() (int, int, string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.current, p.total, p.workerName
}

// Model represents the application state
type Model struct {
	state               sessionState
	worker              *types.WorkerInfo
	plan                *types.DeletionPlan
	Result              *types.DeletionResult
	Err                 error
	spinner             spinner.Model
	message             string
	config              *types.Config
	deleter             *deleter.Deleter
	analyzer            *analyzer.Analyzer
	confirmDeletion     bool
	confirmShared       bool
	skipShared          bool
	skipDependencyCheck bool
	// Analysis progress tracking
	analysisProgress int
	analysisTotal    int
	analysisWorker   string
	progressTracker  *progressTracker
}

// NewModel creates a new application model with a pre-computed plan
func NewModel(worker *types.WorkerInfo, plan *types.DeletionPlan, config *types.Config, d *deleter.Deleter) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		state:   stateShowPlan,
		worker:  worker,
		plan:    plan,
		config:  config,
		deleter: d,
		spinner: s,
	}
}

// NewModelWithAnalysis creates a new model that will run analysis interactively
func NewModelWithAnalysis(worker *types.WorkerInfo, a *analyzer.Analyzer, config *types.Config, d *deleter.Deleter) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	// Start with dependency check prompt unless flag is set
	initialState := stateConfirmDependencyCheck
	if config.SkipDependencyCheck {
		initialState = stateAnalyzing
	}

	return Model{
		state:               initialState,
		worker:              worker,
		config:              config,
		deleter:             d,
		analyzer:            a,
		spinner:             s,
		skipDependencyCheck: config.SkipDependencyCheck,
		progressTracker:     &progressTracker{},
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	if m.state == stateAnalyzing {
		return tea.Batch(
			m.spinner.Tick,
			m.runAnalysis(),
			m.pollProgress(),
		)
	}
	return m.spinner.Tick
}

// pollProgress creates a command that polls for progress updates
func (m Model) pollProgress() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return progressPollMsg{}
	})
}

// runAnalysis runs the dependency analysis in the background
func (m Model) runAnalysis() tea.Cmd {
	return func() tea.Msg {
		var resources []types.ResourceUsage
		var err error

		if m.skipDependencyCheck {
			// Fast path: just get target worker's resources without checking dependencies
			resources, err = m.analyzer.GetTargetWorkerResources(m.worker)
		} else {
			// Full analysis: check all workers for shared resources
			resources, err = m.analyzer.AnalyzeDependencies(m.worker, func(current, total int, workerName string) {
				// Update the progress tracker which will be polled by the UI
				m.progressTracker.update(current, total, workerName)
			})
		}

		if err != nil {
			return analysisErrorMsg{err: err}
		}

		// Create deletion plan
		plan := m.analyzer.CreateDeletionPlan(m.worker, resources, m.config.ExclusiveOnly)
		return analysisCompleteMsg{plan: plan}
	}
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		// Keep spinner running while in analyzing or deleting state
		if m.state == stateAnalyzing || m.state == stateDeleting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case progressPollMsg:
		// Poll the progress tracker and update our fields
		if m.state == stateAnalyzing && m.progressTracker != nil {
			current, total, workerName := m.progressTracker.get()
			m.analysisProgress = current
			m.analysisTotal = total
			m.analysisWorker = workerName
			// Schedule next poll
			return m, m.pollProgress()
		}

	case analysisCompleteMsg:
		m.plan = msg.plan
		m.state = stateShowPlan
		return m, nil

	case analysisErrorMsg:
		m.state = stateError
		m.Err = msg.err
		return m, tea.Quit

	case deletionCompleteMsg:
		m.state = stateComplete
		m.Result = msg.result
		return m, tea.Quit

	case deletionErrorMsg:
		m.state = stateError
		m.Err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Don't handle keys while deleting or analyzing
	if m.state == stateDeleting || m.state == stateAnalyzing {
		return m, nil
	}

	switch m.state {
	case stateConfirmDependencyCheck:
		return m.handleConfirmDependencyCheckKeyPress(msg)
	case stateShowPlan:
		return m.handlePlanKeyPress(msg)
	case stateConfirmDeletion:
		return m.handleConfirmDeletionKeyPress(msg)
	case stateConfirmShared:
		return m.handleConfirmSharedKeyPress(msg)
	}

	// Default: quit on ctrl+c or q
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleConfirmDependencyCheckKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit

	case "n", "N":
		// Skip dependency check
		m.skipDependencyCheck = true
		m.state = stateAnalyzing
		return m, tea.Batch(
			m.spinner.Tick,
			m.runAnalysis(),
			m.pollProgress(),
		)

	case "y", "Y", "enter":
		// Run full dependency analysis
		m.skipDependencyCheck = false
		m.state = stateAnalyzing
		return m, tea.Batch(
			m.spinner.Tick,
			m.runAnalysis(),
			m.pollProgress(),
		)
	}

	return m, nil
}

func (m Model) handlePlanKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc", "n", "N":
		return m, tea.Quit

	case "y", "Y", "enter":
		if m.config.AutoYes {
			// Skip confirmations
			return m.startDeletion()
		}
		m.state = stateConfirmDeletion
		return m, nil
	}

	return m, nil
}

func (m Model) handleConfirmDeletionKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc", "n", "N":
		return m, tea.Quit

	case "y", "Y", "enter":
		if m.plan.HasSharedResources && !m.config.ExclusiveOnly {
			m.state = stateConfirmShared
			return m, nil
		}
		return m.startDeletion()
	}

	return m, nil
}

func (m Model) handleConfirmSharedKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit

	case "n", "N":
		// Don't delete shared resources
		m.skipShared = true
		m.plan.DeleteShared = false
		return m.startDeletion()

	case "y", "Y", "enter":
		// Delete shared resources
		m.plan.DeleteShared = true
		return m.startDeletion()
	}

	return m, nil
}

func (m Model) startDeletion() (tea.Model, tea.Cmd) {
	m.state = stateDeleting
	return m, tea.Batch(
		m.spinner.Tick,
		func() tea.Msg {
			// Execute deletion in background
			result, err := m.deleter.Execute(m.plan)
			if err != nil {
				return deletionErrorMsg{err: err}
			}
			return deletionCompleteMsg{result: result}
		},
	)
}

// View renders the UI
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(views.RenderHeader())

	switch m.state {
	case stateLoading:
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), m.message))

	case stateConfirmDependencyCheck:
		b.WriteString(views.RenderWarning("Dependency analysis can take a long time on accounts with many workers."))
		b.WriteString("\n\n")
		b.WriteString("This checks if other workers share the same resources to prevent accidental deletion.\n")
		b.WriteString("If you're certain no other workers use these resources, you can skip this step.\n\n")
		b.WriteString("Run dependency analysis? [Y/n]: ")

	case stateAnalyzing:
		b.WriteString(fmt.Sprintf("%s Analyzing dependencies...\n", m.spinner.View()))
		if m.analysisTotal > 0 {
			percentage := float64(m.analysisProgress) / float64(m.analysisTotal) * 100
			b.WriteString(fmt.Sprintf("   Progress: %d/%d workers (%.0f%%) - Current: %s\n",
				m.analysisProgress, m.analysisTotal, percentage, m.analysisWorker))
		}

	case stateShowPlan:
		b.WriteString(views.RenderDeletionPlan(m.plan))
		b.WriteString("\n")
		b.WriteString("Proceed with deletion? [y/N]: ")

	case stateConfirmDeletion:
		b.WriteString(views.RenderDeletionPlan(m.plan))
		b.WriteString("\n")
		b.WriteString(views.RenderWarning("This action cannot be undone!"))
		b.WriteString("\n\n")
		b.WriteString("Are you sure? [y/N]: ")

	case stateConfirmShared:
		b.WriteString(views.RenderWarning("Shared resources will be deleted!"))
		b.WriteString("\n\n")
		b.WriteString("This may affect other workers. Continue? [y/N]: ")

	case stateDeleting:
		b.WriteString(fmt.Sprintf("%s Deleting resources...\n", m.spinner.View()))

	case stateComplete:
		b.WriteString(views.RenderDeletionResult(m.Result))
		b.WriteString("\n")

	case stateError:
		b.WriteString(views.RenderError(fmt.Sprintf("Error: %v", m.Err)))
		b.WriteString("\n")
	}

	return b.String()
}

// Messages
type deletionCompleteMsg struct {
	result *types.DeletionResult
}

type deletionErrorMsg struct {
	err error
}

type analysisProgressMsg struct {
	current    int
	total      int
	workerName string
}

type analysisCompleteMsg struct {
	plan *types.DeletionPlan
}

type analysisErrorMsg struct {
	err error
}

type progressPollMsg struct{}

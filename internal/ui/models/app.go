package models

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cloudflare/cf-delete-worker/internal/ui/views"
	"github.com/cloudflare/cf-delete-worker/pkg/types"
)

type sessionState int

const (
	stateLoading sessionState = iota
	stateShowPlan
	stateConfirmDeletion
	stateConfirmShared
	stateDeleting
	stateComplete
	stateError
)

// Model represents the application state
type Model struct {
	state            sessionState
	worker           *types.WorkerInfo
	plan             *types.DeletionPlan
	result           *types.DeletionResult
	err              error
	spinner          spinner.Model
	message          string
	config           *types.Config
	confirmDeletion  bool
	confirmShared    bool
	skipShared       bool
}

// NewModel creates a new application model
func NewModel(worker *types.WorkerInfo, plan *types.DeletionPlan, config *types.Config) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	return Model{
		state:   stateShowPlan,
		worker:  worker,
		plan:    plan,
		config:  config,
		spinner: s,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case deletionCompleteMsg:
		m.state = stateComplete
		m.result = msg.result
		return m, tea.Quit

	case deletionErrorMsg:
		m.state = stateError
		m.err = msg.err
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
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
	return m, func() tea.Msg {
		// This would normally call the deleter
		// For now, return a mock result
		return deletionCompleteMsg{
			result: &types.DeletionResult{
				Success:       true,
				WorkerDeleted: true,
			},
		}
	}
}

// View renders the UI
func (m Model) View() string {
	var b strings.Builder

	b.WriteString(views.RenderHeader())

	switch m.state {
	case stateLoading:
		b.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), m.message))

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
		b.WriteString(views.RenderDeletionResult(m.result))
		b.WriteString("\n")

	case stateError:
		b.WriteString(views.RenderError(fmt.Sprintf("Error: %v", m.err)))
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

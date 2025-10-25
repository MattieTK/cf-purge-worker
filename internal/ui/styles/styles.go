package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Cloudflare color palette
var (
	Orange    = lipgloss.Color("#F38020")
	DarkBlue  = lipgloss.Color("#1E3A8A")
	LightBlue = lipgloss.Color("#3B82F6")
	Green     = lipgloss.Color("#10B981")
	Yellow    = lipgloss.Color("#F59E0B")
	Red       = lipgloss.Color("#EF4444")
	White     = lipgloss.Color("#FFFFFF")
	Gray      = lipgloss.Color("#9CA3AF")
	DarkGray  = lipgloss.Color("#374151")
)

// Text styles
var (
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Orange).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
		Foreground(Gray).
		MarginBottom(1)

	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(Orange).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Orange).
		Padding(0, 2).
		MarginBottom(1)

	Section = lipgloss.NewStyle().
		Bold(true).
		Foreground(LightBlue).
		MarginTop(1).
		MarginBottom(1)

	Info = lipgloss.NewStyle().
		Foreground(White)

	Success = lipgloss.NewStyle().
		Foreground(Green).
		Bold(true)

	Warning = lipgloss.NewStyle().
		Foreground(Yellow).
		Bold(true)

	Error = lipgloss.NewStyle().
		Foreground(Red).
		Bold(true)

	Danger = lipgloss.NewStyle().
		Foreground(Red)

	Muted = lipgloss.NewStyle().
		Foreground(Gray)

	Highlight = lipgloss.NewStyle().
		Foreground(Orange).
		Bold(true)
)

// Box styles
var (
	Box = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(LightBlue).
		Padding(1, 2).
		MarginBottom(1)

	WarningBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Yellow).
		Padding(1, 2).
		MarginBottom(1)

	DangerBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Red).
		Padding(1, 2).
		MarginBottom(1)

	SuccessBox = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(Green).
		Padding(1, 2).
		MarginBottom(1)
)

// List item styles
var (
	ListItem = lipgloss.NewStyle().
		PaddingLeft(2)

	SelectedItem = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(Orange).
		Bold(true)
)

// Risk indicator styles
func RiskIndicator(level string) string {
	switch level {
	case "safe":
		return Success.Render("🟢")
	case "caution":
		return Warning.Render("🟡")
	case "danger":
		return Danger.Render("🔴")
	default:
		return Muted.Render("⚪")
	}
}

// FormatResourceType formats a resource type for display
func FormatResourceType(resourceType string) string {
	switch resourceType {
	case "kv_namespace":
		return "KV Namespace"
	case "r2_bucket":
		return "R2 Bucket"
	case "d1":
		return "D1 Database"
	case "durable_object_namespace":
		return "Durable Object"
	case "service":
		return "Service Binding"
	case "queue":
		return "Queue"
	case "hyperdrive":
		return "Hyperdrive"
	case "vectorize":
		return "Vectorize Index"
	default:
		return resourceType
	}
}

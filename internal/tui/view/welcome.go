// Package view provides view components for the TUI application.
package view

import (
	"strings"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// WelcomeView renders the welcome panel for new users.
// This is shown when there are no instances in the session.
type WelcomeView struct{}

// NewWelcomeView creates a new WelcomeView instance.
func NewWelcomeView() *WelcomeView {
	return &WelcomeView{}
}

// Render renders the welcome panel with the given width.
func (v *WelcomeView) Render(width int) string {
	var b strings.Builder

	// Welcome header with ASCII art-style text
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.PrimaryColor).
		Render("Welcome to Claudio")

	subtitle := styles.Muted.Render("Run multiple Claude instances in parallel")

	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(subtitle)
	b.WriteString("\n\n")

	// Getting Started section
	b.WriteString(v.renderSection("Getting Started", []string{
		styles.HelpKey.Render("[:a]") + "  Create a new Claude instance",
		styles.HelpKey.Render("[?]") + "  View all keyboard shortcuts",
		styles.HelpKey.Render("[:]") + "  Enter command mode",
	}))
	b.WriteString("\n")

	// Quick Commands section
	b.WriteString(v.renderSection("Quick Commands", []string{
		styles.HelpKey.Render(":tripleshot") + "  Run 3 parallel attempts with a judge",
		styles.HelpKey.Render(":adversarial") + "  Implementer + reviewer feedback loop",
		styles.HelpKey.Render(":ultraplan") + "  AI-orchestrated task breakdown",
	}))
	b.WriteString("\n")

	// Status Legend section
	b.WriteString(v.renderStatusLegend())

	// Tips section
	b.WriteString("\n")
	b.WriteString(v.renderTips())

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// sectionHeaderStyle returns the consistent style for section headers.
var sectionHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(styles.SecondaryColor)

// renderSection renders a titled section with a list of items.
func (v *WelcomeView) renderSection(title string, items []string) string {
	var b strings.Builder

	b.WriteString(sectionHeaderStyle.Render("▸ " + title))
	b.WriteString("\n")

	for _, item := range items {
		b.WriteString("   ")
		b.WriteString(item)
		b.WriteString("\n")
	}

	return b.String()
}

// renderStatusLegend renders the status icon legend.
func (v *WelcomeView) renderStatusLegend() string {
	var b strings.Builder

	b.WriteString(sectionHeaderStyle.Render("▸ Status Icons"))
	b.WriteString("\n")

	// Define status items with their colors and descriptions
	statuses := []struct {
		icon        string
		color       lipgloss.Color
		description string
	}{
		{"●", styles.StatusWorking, "Working - Claude is actively processing"},
		{"○", styles.StatusPending, "Pending - Waiting to start"},
		{"◐", styles.StatusPreparing, "Preparing - Setting up worktree"},
		{"?", styles.StatusInput, "Input Needed - Claude needs your input"},
		{"⏸", styles.StatusPaused, "Paused - Instance is paused"},
		{"✓", styles.StatusComplete, "Completed - Task finished successfully"},
		{"✗", styles.StatusError, "Error - Something went wrong"},
		{"⏱", styles.StatusStuck, "Stuck - No activity detected"},
	}

	for _, s := range statuses {
		icon := lipgloss.NewStyle().Foreground(s.color).Render(s.icon)
		desc := styles.Muted.Render(s.description)
		b.WriteString("   ")
		b.WriteString(icon)
		b.WriteString("  ")
		b.WriteString(desc)
		b.WriteString("\n")
	}

	return b.String()
}

// renderTips renders helpful tips for new users.
func (v *WelcomeView) renderTips() string {
	var b strings.Builder

	tipStyle := lipgloss.NewStyle().
		Foreground(styles.MutedColor).
		Italic(true)

	b.WriteString(tipStyle.Render("Tip: "))
	b.WriteString(styles.Muted.Render("Press "))
	b.WriteString(styles.HelpKey.Render("[i]"))
	b.WriteString(styles.Muted.Render(" while viewing an instance to interact with Claude directly"))

	return b.String()
}

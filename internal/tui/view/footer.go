package view

import (
	"strings"

	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// FooterState holds the state needed to render the footer.
type FooterState struct {
	// Session is the current orchestrator session.
	Session *orchestrator.Session

	// GroupedViewEnabled indicates whether the grouped view mode is active.
	GroupedViewEnabled bool

	// Width is the available width for the footer.
	Width int
}

// RenderGroupedModeFooter renders an enhanced footer for grouped view mode.
// Returns empty string if not in grouped mode or no groups exist.
// This should be displayed above or alongside the standard help bar.
func RenderGroupedModeFooter(state FooterState) string {
	if !state.GroupedViewEnabled || state.Session == nil || len(state.Session.Groups) == 0 {
		return ""
	}

	sgm := CalculateSessionGroupMetrics(state.Session)
	if sgm == nil {
		return ""
	}

	var parts []string

	// Overall progress with mini bar
	miniBar := RenderMiniProgressBar(sgm.TotalCompleted, sgm.TotalInstances, 5)
	percent := 0
	if sgm.TotalInstances > 0 {
		percent = (sgm.TotalCompleted * 100) / sgm.TotalInstances
	}

	progressPart := styles.Muted.Render("Groups: ") + miniBar
	if percent > 0 && percent < 100 {
		progressPart += styles.Muted.Render(" (") + styles.Secondary.Render(formatPercent(percent)) + styles.Muted.Render(")")
	} else if percent == 100 {
		progressPart += " " + styles.Secondary.Render("✓")
	}
	parts = append(parts, progressPart)

	// ETA if available
	if sgm.EstimatedETA > 0 {
		etaStr := FormatDuration(sgm.EstimatedETA)
		parts = append(parts, styles.Muted.Render("ETA: ~"+etaStr))
	}

	// Total cost if significant
	if sgm.TotalCost >= 0.01 {
		costStr := instmetrics.FormatCost(sgm.TotalCost)
		parts = append(parts, styles.Muted.Render(costStr))
	}

	// Group phases summary (how many completed/executing/pending)
	summary := renderGroupPhaseSummary(sgm)
	if summary != "" {
		parts = append(parts, summary)
	}

	return strings.Join(parts, "  ")
}

// formatPercent formats a percentage for display.
func formatPercent(percent int) string {
	if percent < 0 {
		return "0%"
	}
	if percent > 100 {
		return "100%"
	}
	return string(rune('0'+percent/100)) + string(rune('0'+(percent/10)%10)) + string(rune('0'+percent%10)) + "%"
}

// renderGroupPhaseSummary renders a summary of group phases.
// Example: "✓2 ●1 ○3" (2 completed, 1 executing, 3 pending)
func renderGroupPhaseSummary(sgm *SessionGroupMetrics) string {
	if sgm == nil || len(sgm.Groups) == 0 {
		return ""
	}

	completed := 0
	executing := 0
	pending := 0
	failed := 0

	for _, gm := range sgm.Groups {
		switch gm.Phase {
		case orchestrator.GroupPhaseCompleted:
			completed++
		case orchestrator.GroupPhaseExecuting:
			executing++
		case orchestrator.GroupPhasePending:
			pending++
		case orchestrator.GroupPhaseFailed:
			failed++
		}
	}

	var parts []string

	if failed > 0 {
		parts = append(parts, styles.Error.Render("✗"+formatInt(failed)))
	}
	if executing > 0 {
		parts = append(parts, styles.Warning.Render("●"+formatInt(executing)))
	}
	if completed > 0 {
		parts = append(parts, styles.Secondary.Render("✓"+formatInt(completed)))
	}
	if pending > 0 {
		parts = append(parts, styles.Muted.Render("○"+formatInt(pending)))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " ")
}

// formatInt formats an integer for compact display.
func formatInt(n int) string {
	if n < 0 {
		return "0"
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	if n < 100 {
		return string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	if n < 1000 {
		return string(rune('0'+n/100)) + string(rune('0'+(n/10)%10)) + string(rune('0'+n%10))
	}
	// For larger numbers, just show the value
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// RenderGroupNavigationHint renders navigation hints for group mode.
// Returns key hints like "[J/K] groups  [j/k] instances"
func RenderGroupNavigationHint() string {
	return styles.HelpKey.Render("[J/K]") + " groups  " +
		styles.HelpKey.Render("[j/k]") + " instances  " +
		styles.HelpKey.Render("[Space]") + " toggle"
}

// RenderGroupProgressBanner renders a prominent progress banner for group mode.
// This can be displayed at the top or bottom of the sidebar.
func RenderGroupProgressBanner(session *orchestrator.Session, width int) string {
	sgm := CalculateSessionGroupMetrics(session)
	if sgm == nil {
		return ""
	}

	var b strings.Builder

	// Progress bar
	barWidth := max(min(width-25, 20), 5)

	overallBar := RenderOverallProgressBar(sgm.TotalCompleted, sgm.TotalInstances, barWidth)
	b.WriteString(styles.Primary.Render("Progress: "))
	b.WriteString(overallBar)

	return b.String()
}

// FooterProvider is an interface for components that can provide group context
// for the footer. This allows the footer to be rendered without direct
// coupling to the TUI model.
type FooterProvider interface {
	// GetSession returns the current session.
	GetSession() *orchestrator.Session

	// IsGroupedViewEnabled returns whether grouped view mode is active.
	IsGroupedViewEnabled() bool
}

// RenderGroupedFooterFromProvider renders the grouped mode footer using a provider.
// This is a convenience function for integration with the TUI.
func RenderGroupedFooterFromProvider(provider FooterProvider, width int) string {
	if provider == nil {
		return ""
	}

	state := FooterState{
		Session:            provider.GetSession(),
		GroupedViewEnabled: provider.IsGroupedViewEnabled(),
		Width:              width,
	}

	return RenderGroupedModeFooter(state)
}

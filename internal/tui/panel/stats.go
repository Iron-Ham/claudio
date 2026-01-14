// Package panel provides interfaces and types for TUI panel rendering.
package panel

import (
	"fmt"
	"sort"
	"strings"

	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/util"
	"github.com/charmbracelet/lipgloss"
)

// StatsPanel renders session statistics and metrics.
// It displays token usage, cost information, and per-instance breakdowns.
type StatsPanel struct {
	height int
}

// NewStatsPanel creates a new StatsPanel.
func NewStatsPanel() *StatsPanel {
	return &StatsPanel{}
}

// Render produces the stats panel output.
func (p *StatsPanel) Render(state *RenderState) string {
	if err := state.ValidateBasic(); err != nil {
		return "[stats panel: render error]"
	}

	var b strings.Builder

	// Title
	title := "📊 Session Statistics"
	if state.Theme != nil {
		title = state.Theme.Primary().Render(title)
	}
	b.WriteString(title)
	b.WriteString("\n\n")

	// Check for no session
	if state.SessionMetrics == nil {
		noSession := "No active session"
		if state.Theme != nil {
			noSession = state.Theme.Muted().Render(noSession)
		}
		b.WriteString(noSession)
		p.height = 4
		return b.String()
	}

	metrics := state.SessionMetrics

	// Session summary
	subtitle := "Session Summary"
	if state.Theme != nil {
		subtitle = state.Theme.Secondary().Render(subtitle)
	}
	b.WriteString(subtitle)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Total Instances: %d (%d active)\n",
		metrics.InstanceCount, metrics.ActiveCount))
	if !state.SessionCreated.IsZero() {
		b.WriteString(fmt.Sprintf("  Session Started: %s\n",
			state.SessionCreated.Format("2006-01-02 15:04:05")))
	}
	b.WriteString("\n")

	// Token usage
	tokenTitle := "Token Usage"
	if state.Theme != nil {
		tokenTitle = state.Theme.Secondary().Render(tokenTitle)
	}
	b.WriteString(tokenTitle)
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Input:  %s\n", instmetrics.FormatTokens(metrics.TotalInputTokens)))
	b.WriteString(fmt.Sprintf("  Output: %s\n", instmetrics.FormatTokens(metrics.TotalOutputTokens)))
	totalTokens := metrics.TotalInputTokens + metrics.TotalOutputTokens
	b.WriteString(fmt.Sprintf("  Total:  %s\n", instmetrics.FormatTokens(totalTokens)))
	if metrics.TotalCacheRead > 0 || metrics.TotalCacheWrite > 0 {
		b.WriteString(fmt.Sprintf("  Cache:  %s read / %s write\n",
			instmetrics.FormatTokens(metrics.TotalCacheRead),
			instmetrics.FormatTokens(metrics.TotalCacheWrite)))
	}
	b.WriteString("\n")

	// Cost summary
	costTitle := "Estimated Cost"
	if state.Theme != nil {
		costTitle = state.Theme.Secondary().Render(costTitle)
	}
	b.WriteString(costTitle)
	b.WriteString("\n")
	costStr := instmetrics.FormatCost(metrics.TotalCost)
	if state.CostWarningThreshold > 0 && metrics.TotalCost >= state.CostWarningThreshold {
		warningText := fmt.Sprintf("  Total: %s (⚠ exceeds warning threshold)", costStr)
		if state.Theme != nil {
			warningText = state.Theme.Warning().Render(warningText)
		}
		b.WriteString(warningText)
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("  Total: %s\n", costStr))
	}
	if state.CostLimit > 0 {
		b.WriteString(fmt.Sprintf("  Limit: %s\n", instmetrics.FormatCost(state.CostLimit)))
	}
	b.WriteString("\n")

	// Per-instance breakdown
	instanceTitle := "Top Instances by Cost"
	if state.Theme != nil {
		instanceTitle = state.Theme.Secondary().Render(instanceTitle)
	}
	b.WriteString(instanceTitle)
	b.WriteString("\n")

	p.renderTopInstances(&b, state)

	b.WriteString("\n")
	hint := "Press [m] to close this view"
	if state.Theme != nil {
		hint = state.Theme.Muted().Render(hint)
	}
	b.WriteString(hint)

	// Calculate height
	p.height = strings.Count(b.String(), "\n") + 1

	return b.String()
}

// RenderWithBox renders the stats panel and wraps it in a styled box.
// This is the preferred method for rendering in the main TUI where
// the ContentBox style should wrap the panel content.
func (p *StatsPanel) RenderWithBox(state *RenderState, boxStyle lipgloss.Style) string {
	content := p.Render(state)
	return boxStyle.Width(state.Width - 4).Render(content)
}

// renderTopInstances renders the top instances by cost.
func (p *StatsPanel) renderTopInstances(b *strings.Builder, state *RenderState) {
	type instCost struct {
		id   string
		num  int
		task string
		cost float64
	}

	var costList []instCost
	for i, inst := range state.Instances {
		cost := 0.0
		if inst.Metrics != nil {
			cost = inst.Metrics.Cost
		}
		costList = append(costList, instCost{
			id:   inst.ID,
			num:  i + 1,
			task: inst.Task,
			cost: cost,
		})
	}

	// Sort descending by cost
	sort.Slice(costList, func(i, j int) bool {
		return costList[i].cost > costList[j].cost
	})

	// Show top 5
	shown := 0
	for _, ic := range costList {
		if shown >= 5 {
			break
		}
		if ic.cost > 0 {
			taskTrunc := util.TruncateString(ic.task, state.Width-25)
			fmt.Fprintf(b, "  %d. [%d] %s: %s\n",
				shown+1, ic.num, taskTrunc, instmetrics.FormatCost(ic.cost))
			shown++
		}
	}
	if shown == 0 {
		noData := "  No cost data available yet"
		if state.Theme != nil {
			noData = state.Theme.Muted().Render(noData)
		}
		b.WriteString(noData)
		b.WriteString("\n")
	}
}

// Height returns the rendered height of the panel.
func (p *StatsPanel) Height() int {
	return p.height
}

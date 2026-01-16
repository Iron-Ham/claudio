package ultraplan

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// PlanViewRenderer handles rendering of the detailed plan view.
type PlanViewRenderer struct {
	ctx *RenderContext
}

// NewPlanViewRenderer creates a new plan view renderer with the given context.
func NewPlanViewRenderer(ctx *RenderContext) *PlanViewRenderer {
	return &PlanViewRenderer{ctx: ctx}
}

// Render renders the detailed plan view.
func (p *PlanViewRenderer) Render(width int) string {
	if p.ctx.UltraPlan == nil || p.ctx.UltraPlan.Coordinator == nil {
		return "No plan available"
	}
	session := p.ctx.UltraPlan.Coordinator.Session()
	if session == nil || session.Plan == nil {
		return "No plan available"
	}

	plan := session.Plan
	var b strings.Builder

	// Multi-pass planning source header (if applicable)
	if session.Config.MultiPass && len(session.CandidatePlans) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Plan Source"))
		b.WriteString("\n")

		if session.SelectedPlanIndex == -1 {
			mergedStyle := lipgloss.NewStyle().Foreground(styles.PurpleColor)
			b.WriteString(mergedStyle.Render("⚡ Merged from multiple strategies"))
			b.WriteString("\n")
			strategyNames := orchestrator.GetMultiPassStrategyNames()
			contributingStrategies := []string{}
			for i := range session.CandidatePlans {
				if i < len(strategyNames) {
					contributingStrategies = append(contributingStrategies, strategyNames[i])
				}
			}
			if len(contributingStrategies) > 0 {
				b.WriteString(styles.Muted.Render("  Combined: " + strings.Join(contributingStrategies, ", ")))
				b.WriteString("\n")
			}
		} else if session.SelectedPlanIndex >= 0 {
			strategyNames := orchestrator.GetMultiPassStrategyNames()
			strategyName := "unknown"
			if session.SelectedPlanIndex < len(strategyNames) {
				strategyName = strategyNames[session.SelectedPlanIndex]
			}
			selectedStyle := lipgloss.NewStyle().Foreground(styles.GreenColor)
			b.WriteString(selectedStyle.Render(fmt.Sprintf("✓ Strategy: %s (selected)", strategyName)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Plan summary
	b.WriteString(styles.SidebarTitle.Render("Plan Summary"))
	b.WriteString("\n\n")
	b.WriteString(plan.Summary)
	b.WriteString("\n\n")

	// Insights
	if len(plan.Insights) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Key Insights"))
		b.WriteString("\n")
		for _, insight := range plan.Insights {
			b.WriteString(fmt.Sprintf("• %s\n", insight))
		}
		b.WriteString("\n")
	}

	// Constraints
	if len(plan.Constraints) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Constraints/Risks"))
		b.WriteString("\n")
		for _, constraint := range plan.Constraints {
			b.WriteString(fmt.Sprintf("⚠ %s\n", constraint))
		}
		b.WriteString("\n")
	}

	// Tasks by execution order
	b.WriteString(styles.SidebarTitle.Render("Execution Order"))
	b.WriteString("\n")
	for groupIdx, group := range plan.ExecutionOrder {
		b.WriteString(fmt.Sprintf("\nGroup %d (parallel):\n", groupIdx+1))
		for _, taskID := range group {
			task := session.GetTask(taskID)
			if task != nil {
				complexity := ComplexityIndicator(task.EstComplexity)
				b.WriteString(fmt.Sprintf("  [%s] %s %s\n", task.ID, complexity, task.Title))
				if len(task.Files) > 0 {
					b.WriteString(fmt.Sprintf("      Files: %s\n", strings.Join(task.Files, ", ")))
				}
			}
		}
	}

	return styles.OutputArea.Width(width - 2).Render(b.String())
}

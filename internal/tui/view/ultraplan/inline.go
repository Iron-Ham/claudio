package ultraplan

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// InlineRenderer handles rendering of ultraplan content for inline display within a group.
type InlineRenderer struct {
	ctx          *RenderContext
	taskRenderer *TaskRenderer
	status       *StatusRenderer
}

// NewInlineRenderer creates a new inline content renderer with the given context.
func NewInlineRenderer(ctx *RenderContext) *InlineRenderer {
	return &InlineRenderer{
		ctx:          ctx,
		taskRenderer: NewTaskRenderer(ctx),
		status:       NewStatusRenderer(ctx),
	}
}

// Render renders the ultraplan phase content for display inline within a group.
// This is a simplified version of RenderSidebar that shows the essential phase information
// without the full sidebar chrome. Used when ultraplan is shown as a collapsible group.
func (i *InlineRenderer) Render(width int, maxLines int) string {
	if i.ctx.UltraPlan == nil || i.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := i.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var b strings.Builder
	lineCount := 0

	// Indent for inline content within group
	indent := "  "

	// ========== GROUP DECISION SECTION (if awaiting decision) ==========
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		b.WriteString(indent)
		b.WriteString(lipgloss.NewStyle().Foreground(styles.YellowColor).Bold(true).Render("⚠ DECISION NEEDED"))
		b.WriteString("\n")
		lineCount++
		if lineCount >= maxLines {
			return b.String()
		}
	}

	// ========== PHASE STATUS ==========
	phaseStr := PhaseToString(session.Phase)
	pStyle := PhaseStyle(session.Phase)
	b.WriteString(indent)
	b.WriteString(pStyle.Render(fmt.Sprintf("Phase: %s", phaseStr)))
	b.WriteString("\n")
	lineCount++
	if lineCount >= maxLines {
		return b.String()
	}

	// ========== PLANNING SECTION ==========
	planningStatus := i.status.GetPhaseSectionStatus(orchestrator.PhasePlanning, session)
	b.WriteString(indent)
	b.WriteString(styles.Muted.Render(fmt.Sprintf("Planning %s", planningStatus)))
	b.WriteString("\n")
	lineCount++
	if lineCount >= maxLines {
		return b.String()
	}

	// Show coordinator(s) based on planning mode
	if session.Config.MultiPass {
		// Multi-pass mode: show all planning coordinators
		for idx, coordID := range session.PlanCoordinatorIDs {
			if lineCount >= maxLines {
				return b.String()
			}
			inst := i.ctx.GetInstance(coordID)
			selected := i.ctx.IsSelected(coordID)
			navigable := inst != nil && inst.Status != orchestrator.StatusPending
			line := i.taskRenderer.RenderPhaseInstanceLine(inst, fmt.Sprintf("Planner %d", idx+1), selected, navigable, width-6)
			b.WriteString(indent)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
		}
	} else if session.Phase == orchestrator.PhasePlanning && session.CoordinatorID != "" {
		// Single-pass mode: show single coordinator
		inst := i.ctx.GetInstance(session.CoordinatorID)
		selected := i.ctx.IsSelected(session.CoordinatorID)
		line := i.taskRenderer.RenderPhaseInstanceLine(inst, "Coordinator", selected, true, width-6)
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteString("\n")
		lineCount++
		if lineCount >= maxLines {
			return b.String()
		}
	}

	// ========== PLAN SELECTION SECTION (multi-pass only) ==========
	if session.Config.MultiPass && (session.Phase == orchestrator.PhasePlanSelection ||
		session.PlanManagerID != "" ||
		len(session.CandidatePlans) > 0) {
		b.WriteString(indent)
		selStatus := i.status.GetPhaseSectionStatus(orchestrator.PhasePlanSelection, session)
		b.WriteString(styles.Muted.Render(fmt.Sprintf("Plan Selection %s", selStatus)))
		b.WriteString("\n")
		lineCount++
		if lineCount >= maxLines {
			return b.String()
		}

		// Show Plan Manager (Plan Selector) if it exists
		if session.PlanManagerID != "" {
			inst := i.ctx.GetInstance(session.PlanManagerID)
			selected := i.ctx.IsSelected(session.PlanManagerID)
			navigable := inst != nil && inst.Status != orchestrator.StatusPending
			line := i.taskRenderer.RenderPhaseInstanceLine(inst, "Plan Selector", selected, navigable, width-6)
			b.WriteString(indent)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
			if lineCount >= maxLines {
				return b.String()
			}
		}
	}

	// ========== EXECUTION SECTION ==========
	if session.Plan != nil {
		executionStarted := session.Phase == orchestrator.PhaseExecuting ||
			session.Phase == orchestrator.PhaseSynthesis ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		execStatus := i.status.GetPhaseSectionStatus(orchestrator.PhaseExecuting, session)
		b.WriteString(indent)
		b.WriteString(styles.Muted.Render(fmt.Sprintf("Execution %s", execStatus)))
		b.WriteString("\n")
		lineCount++
		if lineCount >= maxLines {
			return b.String()
		}

		// Show execution groups with expand/collapse support
		for groupIdx, group := range session.Plan.ExecutionOrder {
			if lineCount >= maxLines-2 {
				b.WriteString(indent)
				b.WriteString(styles.Muted.Render("  ...more groups"))
				b.WriteString("\n")
				lineCount++
				break
			}

			// Check if this group is collapsed (default: collapsed unless it's the current group)
			isCollapsed := i.ctx.UltraPlan.IsGroupCollapsed(groupIdx, session.CurrentGroup)
			isGroupSelected := i.ctx.UltraPlan.GroupNavMode && i.ctx.UltraPlan.SelectedGroupIdx == groupIdx

			// Determine collapse indicator
			collapseIcon := "▼"
			if isCollapsed {
				collapseIcon = "▶"
			}

			// Build group header with collapse indicator and status
			groupStatus := i.taskRenderer.GetGroupStatus(session, group)
			var groupHeader string
			if isCollapsed {
				// Show summary stats when collapsed
				stats := i.taskRenderer.CalculateGroupStats(session, group)
				summary := i.taskRenderer.FormatGroupSummary(stats)
				groupHeader = fmt.Sprintf("  %s Group %d %s %s", collapseIcon, groupIdx+1, groupStatus, summary)
			} else {
				groupHeader = fmt.Sprintf("  %s Group %d %s", collapseIcon, groupIdx+1, groupStatus)
			}

			// Apply styling based on state
			b.WriteString(indent)
			if isGroupSelected {
				// Highlight selected group
				groupHeader = lipgloss.NewStyle().
					Background(styles.PrimaryColor).
					Foreground(styles.TextColor).
					Render(groupHeader)
				b.WriteString(groupHeader)
			} else if !executionStarted {
				b.WriteString(styles.Muted.Render(groupHeader))
			} else {
				b.WriteString(styles.Muted.Render(groupHeader))
			}
			b.WriteString("\n")
			lineCount++

			// Only render tasks if group is expanded
			if !isCollapsed {
				for _, taskID := range group {
					if lineCount >= maxLines-2 {
						break
					}

					task := session.GetTask(taskID)
					if task == nil {
						continue
					}

					instID := i.taskRenderer.FindInstanceIDForTask(session, taskID)
					selected := i.ctx.IsSelected(instID)
					navigable := instID != ""
					taskResult := i.taskRenderer.RenderExecutionTaskLine(session, task, instID, selected, navigable, width-8)
					b.WriteString(indent)
					b.WriteString(taskResult.Content)
					b.WriteString("\n")
					lineCount += taskResult.LineCount
				}

				// Render group consolidator if exists
				if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
					consolidatorID := session.GroupConsolidatorIDs[groupIdx]
					inst := i.ctx.GetInstance(consolidatorID)
					selected := i.ctx.IsSelected(consolidatorID)
					navigable := true
					consolidatorLine := i.taskRenderer.RenderGroupConsolidatorLine(inst, groupIdx, selected, navigable, width-8)
					b.WriteString(indent)
					b.WriteString(consolidatorLine)
					b.WriteString("\n")
					lineCount++
				}
			}
		}
	}

	// ========== SYNTHESIS SECTION ==========
	if session.SynthesisID != "" || session.Phase == orchestrator.PhaseSynthesis {
		synthStatus := i.status.GetPhaseSectionStatus(orchestrator.PhaseSynthesis, session)
		b.WriteString(indent)
		b.WriteString(styles.Muted.Render(fmt.Sprintf("Synthesis %s", synthStatus)))
		b.WriteString("\n")
		lineCount++
		if lineCount >= maxLines {
			return b.String()
		}
	}

	// ========== CONSOLIDATION SECTION ==========
	if session.ConsolidationID != "" || session.Phase == orchestrator.PhaseConsolidating {
		consStatus := i.status.GetPhaseSectionStatus(orchestrator.PhaseConsolidating, session)
		b.WriteString(indent)
		b.WriteString(styles.Muted.Render(fmt.Sprintf("Consolidation %s", consStatus)))
		b.WriteString("\n")
		lineCount++ //nolint:ineffassign // lineCount tracks rendering progress

		// Show PR count if any
		if len(session.PRUrls) > 0 {
			b.WriteString(indent)
			b.WriteString(styles.Muted.Render(fmt.Sprintf("  %d PR(s) created", len(session.PRUrls))))
			b.WriteString("\n")
		}
	}

	return b.String()
}

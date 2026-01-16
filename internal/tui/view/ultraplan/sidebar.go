package ultraplan

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// SidebarRenderer handles rendering of the unified sidebar showing all phases.
type SidebarRenderer struct {
	ctx          *RenderContext
	taskRenderer *TaskRenderer
	status       *StatusRenderer
}

// NewSidebarRenderer creates a new sidebar renderer with the given context.
func NewSidebarRenderer(ctx *RenderContext) *SidebarRenderer {
	return &SidebarRenderer{
		ctx:          ctx,
		taskRenderer: NewTaskRenderer(ctx),
		status:       NewStatusRenderer(ctx),
	}
}

// Render renders a unified sidebar showing all phases with their instances.
func (s *SidebarRenderer) Render(width int, height int) string {
	if s.ctx.UltraPlan == nil || s.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := s.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var b strings.Builder
	lineCount := 0
	availableLines := height - 4

	// ========== GROUP DECISION SECTION (if awaiting decision) ==========
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		decisionContent := s.renderGroupDecisionSection(session.GroupDecision, width-4)
		b.WriteString(decisionContent)
		b.WriteString("\n\n")
		lineCount += 12
	}

	// ========== PLANNING SECTION ==========
	planningComplete := session.Phase != orchestrator.PhasePlanning && session.Phase != orchestrator.PhasePlanSelection
	planningStatus := s.status.GetPhaseSectionStatus(orchestrator.PhasePlanning, session)
	planningHeader := fmt.Sprintf("▼ PLANNING %s", planningStatus)
	b.WriteString(styles.SidebarTitle.Render(planningHeader))
	b.WriteString("\n")
	lineCount++

	// Check if multi-pass mode is enabled
	if session.Config.MultiPass {
		lineCount += s.renderMultiPassPlanningSection(&b, session, width, availableLines-lineCount)
	} else {
		if session.CoordinatorID != "" && lineCount < availableLines {
			inst := s.ctx.GetInstance(session.CoordinatorID)
			selected := s.ctx.IsSelected(session.CoordinatorID)
			navigable := planningComplete || (inst != nil && inst.Status != orchestrator.StatusPending)
			line := s.taskRenderer.RenderPhaseInstanceLine(inst, "Coordinator", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
		}
	}
	b.WriteString("\n")
	lineCount++

	// ========== PLAN SELECTION SECTION (for multi-pass planning) ==========
	isMultiPassPlanning := len(session.PlanCoordinatorIDs) > 0 ||
		session.PlanManagerID != "" ||
		session.Phase == orchestrator.PhasePlanSelection

	if isMultiPassPlanning && lineCount < availableLines {
		selStatus := s.status.GetPhaseSectionStatus(orchestrator.PhasePlanSelection, session)
		selHeader := fmt.Sprintf("▼ PLAN SELECTION %s", selStatus)

		switch session.Phase {
		case orchestrator.PhasePlanSelection:
			b.WriteString(styles.SidebarTitle.Render(selHeader))
		case orchestrator.PhasePlanning:
			b.WriteString(styles.Muted.Render(selHeader))
		default:
			b.WriteString(styles.SidebarTitle.Render(selHeader))
		}
		b.WriteString("\n")
		lineCount++

		switch session.Phase {
		case orchestrator.PhasePlanning:
			b.WriteString(styles.Muted.Render("  ○ Awaiting candidate plans"))
		case orchestrator.PhasePlanSelection:
			numCandidates := len(session.CandidatePlans)
			if numCandidates > 0 {
				b.WriteString(styles.Muted.Render(fmt.Sprintf("  ⟳ Comparing %d plans...", numCandidates)))
			} else {
				b.WriteString(styles.Muted.Render("  ⟳ Comparing plans..."))
			}
		default:
			if session.SelectedPlanIndex >= 0 {
				b.WriteString(styles.Muted.Render(fmt.Sprintf("  ✓ Plan %d selected", session.SelectedPlanIndex+1)))
			} else {
				b.WriteString(styles.Muted.Render("  ✓ Best plan selected"))
			}
		}
		b.WriteString("\n")
		lineCount++

		if session.PlanManagerID != "" && lineCount < availableLines {
			inst := s.ctx.GetInstance(session.PlanManagerID)
			selected := s.ctx.IsSelected(session.PlanManagerID)
			navigable := inst != nil && inst.Status != orchestrator.StatusPending
			line := s.taskRenderer.RenderPhaseInstanceLine(inst, "Plan Manager", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
		}

		b.WriteString("\n")
		lineCount++
	}

	// ========== EXECUTION SECTION ==========
	if session.Plan != nil && lineCount < availableLines {
		lineCount += s.renderExecutionSection(&b, session, width, availableLines-lineCount)
	}

	// ========== SYNTHESIS SECTION ==========
	if lineCount < availableLines {
		lineCount += s.renderSynthesisSection(&b, session, width, availableLines-lineCount)
	}

	// ========== REVISION SECTION ==========
	if lineCount < availableLines {
		lineCount += s.renderRevisionSection(&b, session, width, availableLines-lineCount)
	}

	// ========== CONSOLIDATION SECTION ==========
	if session.Config.ConsolidationMode != "" && lineCount < availableLines {
		lineCount += s.renderConsolidationSection(&b, session, width, availableLines-lineCount)
	}
	_ = lineCount // Silence unused variable warning - lineCount tracked for potential future use

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderGroupDecisionSection renders the group decision dialog when user input is needed.
func (s *SidebarRenderer) renderGroupDecisionSection(decision *orchestrator.GroupDecisionState, maxWidth int) string {
	var b strings.Builder

	warningStyle := lipgloss.NewStyle().
		Foreground(styles.YellowColor).
		Bold(true)
	b.WriteString(warningStyle.Render("⚠ PARTIAL GROUP FAILURE"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Group %d has mixed results:\n", decision.GroupIndex+1))

	if len(decision.SucceededTasks) > 0 {
		successStyle := lipgloss.NewStyle().Foreground(styles.GreenColor)
		b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ %d task(s) succeeded", len(decision.SucceededTasks))))
		b.WriteString("\n")
	}

	if len(decision.FailedTasks) > 0 {
		failStyle := lipgloss.NewStyle().Foreground(styles.RedColor)
		b.WriteString(failStyle.Render(fmt.Sprintf("  ✗ %d task(s) failed", len(decision.FailedTasks))))
		b.WriteString("\n")

		maxToShow := 3
		for i, taskID := range decision.FailedTasks {
			if i >= maxToShow {
				remaining := len(decision.FailedTasks) - maxToShow
				b.WriteString(styles.Muted.Render(fmt.Sprintf("    ... +%d more", remaining)))
				b.WriteString("\n")
				break
			}
			taskDisplay := truncate(taskID, maxWidth-8)
			b.WriteString(styles.Muted.Render("    - " + taskDisplay))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	b.WriteString(styles.SidebarTitle.Render("Choose action:"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  [c] Continue with successful tasks"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  [r] Retry failed tasks"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  [q] Cancel ultraplan"))
	b.WriteString("\n")

	return b.String()
}

// renderExecutionSection renders the execution phase section.
func (s *SidebarRenderer) renderExecutionSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
	lineCount := 0

	executionStarted := session.Phase == orchestrator.PhaseExecuting ||
		session.Phase == orchestrator.PhaseSynthesis ||
		session.Phase == orchestrator.PhaseConsolidating ||
		session.Phase == orchestrator.PhaseComplete

	execStatus := s.status.GetPhaseSectionStatus(orchestrator.PhaseExecuting, session)
	execHeader := fmt.Sprintf("▼ EXECUTION %s", execStatus)
	b.WriteString(styles.SidebarTitle.Render(execHeader))
	b.WriteString("\n")
	lineCount++

	for groupIdx, group := range session.Plan.ExecutionOrder {
		if lineCount >= availableLines-4 {
			b.WriteString(styles.Muted.Render("  ...more"))
			b.WriteString("\n")
			lineCount++
			break
		}

		// Check if this group is collapsed (default: collapsed unless it's the current group)
		isCollapsed := s.ctx.UltraPlan.IsGroupCollapsed(groupIdx, session.CurrentGroup)
		isGroupSelected := s.ctx.UltraPlan.GroupNavMode && s.ctx.UltraPlan.SelectedGroupIdx == groupIdx

		// Determine collapse indicator
		collapseIcon := "▼"
		if isCollapsed {
			collapseIcon = "▶"
		}

		// Build group header with collapse indicator and status
		groupStatus := s.taskRenderer.GetGroupStatus(session, group)
		var groupHeader string
		if isCollapsed {
			// Show summary stats when collapsed
			stats := s.taskRenderer.CalculateGroupStats(session, group)
			summary := s.taskRenderer.FormatGroupSummary(stats)
			groupHeader = fmt.Sprintf("  %s Group %d %s %s", collapseIcon, groupIdx+1, groupStatus, summary)
		} else {
			groupHeader = fmt.Sprintf("  %s Group %d %s", collapseIcon, groupIdx+1, groupStatus)
		}

		// Apply styling based on state
		if isGroupSelected {
			// Highlight selected group
			groupHeader = lipgloss.NewStyle().
				Background(styles.PrimaryColor).
				Foreground(styles.TextColor).
				Render(groupHeader)
		} else if !executionStarted {
			groupHeader = styles.Muted.Render(groupHeader)
		}

		b.WriteString(groupHeader)
		b.WriteString("\n")
		lineCount++

		// Only render tasks if group is expanded
		if !isCollapsed {
			for _, taskID := range group {
				if lineCount >= availableLines-4 {
					break
				}

				task := session.GetTask(taskID)
				if task == nil {
					continue
				}

				instID := s.taskRenderer.FindInstanceIDForTask(session, taskID)
				selected := s.ctx.IsSelected(instID)
				navigable := instID != ""
				taskResult := s.taskRenderer.RenderExecutionTaskLine(session, task, instID, selected, navigable, width-6)
				b.WriteString(taskResult.Content)
				b.WriteString("\n")
				lineCount += taskResult.LineCount
			}

			if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
				consolidatorID := session.GroupConsolidatorIDs[groupIdx]
				inst := s.ctx.GetInstance(consolidatorID)
				selected := s.ctx.IsSelected(consolidatorID)
				navigable := true
				consolidatorLine := s.taskRenderer.RenderGroupConsolidatorLine(inst, groupIdx, selected, navigable, width-6)
				b.WriteString(consolidatorLine)
				b.WriteString("\n")
				lineCount++
			}
		}
	}
	b.WriteString("\n")
	lineCount++

	return lineCount
}

// renderSynthesisSection renders the synthesis phase section.
func (s *SidebarRenderer) renderSynthesisSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
	lineCount := 0

	synthesisStarted := session.Phase == orchestrator.PhaseSynthesis ||
		session.Phase == orchestrator.PhaseRevision ||
		session.Phase == orchestrator.PhaseConsolidating ||
		session.Phase == orchestrator.PhaseComplete

	synthStatus := s.status.GetPhaseSectionStatus(orchestrator.PhaseSynthesis, session)
	synthHeader := fmt.Sprintf("▼ SYNTHESIS %s", synthStatus)
	if !synthesisStarted && session.SynthesisID == "" {
		b.WriteString(styles.Muted.Render(synthHeader))
	} else {
		b.WriteString(styles.SidebarTitle.Render(synthHeader))
	}
	b.WriteString("\n")
	lineCount++

	if session.SynthesisID != "" && lineCount < availableLines {
		inst := s.ctx.GetInstance(session.SynthesisID)
		selected := s.ctx.IsSelected(session.SynthesisID)
		navigable := true
		line := s.taskRenderer.RenderPhaseInstanceLine(inst, "Reviewer", selected, navigable, width-4)
		b.WriteString(line)
		b.WriteString("\n")
		lineCount++

		if session.SynthesisAwaitingApproval && session.SynthesisCompletion != nil && lineCount < availableLines {
			issueCount := len(session.SynthesisCompletion.IssuesFound)
			if issueCount > 0 {
				issueText := fmt.Sprintf("  ⚠ %d issue(s) found", issueCount)
				b.WriteString(styles.Warning.Render(issueText))
			} else {
				b.WriteString(styles.SuccessMsg.Render("  ✓ No issues found"))
			}
			b.WriteString("\n")
			lineCount++

			b.WriteString(styles.Warning.Render("  Press [s] to approve"))
			b.WriteString("\n")
			lineCount++
		}
	} else if !synthesisStarted && lineCount < availableLines {
		b.WriteString(styles.Muted.Render("  ○ Pending"))
		b.WriteString("\n")
		lineCount++
	}
	b.WriteString("\n")
	lineCount++

	return lineCount
}

// renderRevisionSection renders the revision phase section.
func (s *SidebarRenderer) renderRevisionSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
	lineCount := 0

	revisionStarted := session.Phase == orchestrator.PhaseRevision ||
		session.Phase == orchestrator.PhaseConsolidating ||
		session.Phase == orchestrator.PhaseComplete

	hasRevision := session.Revision != nil && len(session.Revision.Issues) > 0

	if !hasRevision && session.Phase != orchestrator.PhaseRevision {
		return lineCount
	}

	revStatus := s.status.GetPhaseSectionStatus(orchestrator.PhaseRevision, session)
	revHeader := fmt.Sprintf("▼ REVISION %s", revStatus)
	if !revisionStarted && session.RevisionID == "" {
		b.WriteString(styles.Muted.Render(revHeader))
	} else {
		b.WriteString(styles.SidebarTitle.Render(revHeader))
	}
	b.WriteString("\n")
	lineCount++

	if session.Revision != nil && lineCount < availableLines {
		roundInfo := fmt.Sprintf("  Round %d/%d", session.Revision.RevisionRound, session.Revision.MaxRevisions)
		b.WriteString(styles.Muted.Render(roundInfo))
		b.WriteString("\n")
		lineCount++

		issueInfo := fmt.Sprintf("  %d issue(s) to address", len(session.Revision.Issues))
		b.WriteString(styles.Muted.Render(issueInfo))
		b.WriteString("\n")
		lineCount++

		if session.RevisionID != "" && lineCount < availableLines {
			inst := s.ctx.GetInstance(session.RevisionID)
			selected := s.ctx.IsSelected(session.RevisionID)
			navigable := true
			line := s.taskRenderer.RenderPhaseInstanceLine(inst, "Reviser", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
		}

		if len(session.Revision.TasksToRevise) > 0 && lineCount < availableLines {
			for _, taskID := range session.Revision.TasksToRevise {
				if lineCount >= availableLines-2 {
					break
				}
				task := session.GetTask(taskID)
				if task == nil {
					continue
				}
				revised := false
				for _, rt := range session.Revision.RevisedTasks {
					if rt == taskID {
						revised = true
						break
					}
				}
				icon := "○"
				if revised {
					icon = "✓"
				} else if session.Phase == orchestrator.PhaseRevision {
					icon = "⟳"
				}
				taskLine := fmt.Sprintf("    %s %s", icon, truncate(task.Title, width-10))
				b.WriteString(styles.Muted.Render(taskLine))
				b.WriteString("\n")
				lineCount++
			}
		}
	} else if !revisionStarted && lineCount < availableLines {
		b.WriteString(styles.Muted.Render("  ○ No issues"))
		b.WriteString("\n")
		lineCount++
	}
	b.WriteString("\n")
	lineCount++

	return lineCount
}

// renderConsolidationSection renders the consolidation phase section.
func (s *SidebarRenderer) renderConsolidationSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
	lineCount := 0

	consolidationStarted := session.Phase == orchestrator.PhaseConsolidating ||
		session.Phase == orchestrator.PhaseComplete

	consStatus := s.status.GetPhaseSectionStatus(orchestrator.PhaseConsolidating, session)
	consHeader := fmt.Sprintf("▼ CONSOLIDATION %s", consStatus)
	if !consolidationStarted && session.ConsolidationID == "" {
		b.WriteString(styles.Muted.Render(consHeader))
	} else {
		b.WriteString(styles.SidebarTitle.Render(consHeader))
	}
	b.WriteString("\n")
	lineCount++

	if session.ConsolidationID != "" && lineCount < availableLines {
		inst := s.ctx.GetInstance(session.ConsolidationID)
		selected := s.ctx.IsSelected(session.ConsolidationID)
		navigable := true
		line := s.taskRenderer.RenderPhaseInstanceLine(inst, "Consolidator", selected, navigable, width-4)
		b.WriteString(line)
		b.WriteString("\n")
		lineCount++

		if len(session.PRUrls) > 0 && lineCount < availableLines {
			prLine := fmt.Sprintf("    %d PR(s) created", len(session.PRUrls))
			b.WriteString(styles.Muted.Render(prLine))
			b.WriteString("\n")
		}
	} else if !consolidationStarted && lineCount < availableLines {
		b.WriteString(styles.Muted.Render("  ○ Pending"))
		b.WriteString("\n")
	}

	return lineCount
}

// renderMultiPassPlanningSection renders the multi-pass planning coordinators in the unified sidebar.
func (s *SidebarRenderer) renderMultiPassPlanningSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
	lineCount := 0

	strategyNames := []string{
		"maximize-parallelism",
		"minimize-complexity",
		"balanced-approach",
	}

	plansReady := len(session.CandidatePlans)
	totalCoordinators := len(session.PlanCoordinatorIDs)
	if totalCoordinators == 0 {
		totalCoordinators = 3
	}

	for i, strategy := range strategyNames {
		if lineCount >= availableLines {
			break
		}

		var statusIcon string
		var statusStyle lipgloss.Style

		if i < len(session.PlanCoordinatorIDs) {
			instID := session.PlanCoordinatorIDs[i]
			inst := s.ctx.GetInstance(instID)
			if inst != nil {
				switch inst.Status {
				case orchestrator.StatusWorking:
					statusIcon = "⟳"
					statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
				case orchestrator.StatusCompleted:
					statusIcon = "✓"
					statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
				case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
					statusIcon = "✗"
					statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				case orchestrator.StatusPending:
					statusIcon = "○"
					statusStyle = styles.Muted
				default:
					statusIcon = "◌"
					statusStyle = styles.Muted
				}
			} else {
				statusIcon = "○"
				statusStyle = styles.Muted
			}
		} else {
			statusIcon = "○"
			statusStyle = styles.Muted
		}

		isSelected := false
		if i < len(session.PlanCoordinatorIDs) {
			isSelected = s.ctx.IsSelected(session.PlanCoordinatorIDs[i])
		}

		strategyLine := fmt.Sprintf("  %s %s", statusStyle.Render(statusIcon), strategy)

		if isSelected {
			b.WriteString(lipgloss.NewStyle().
				Background(styles.PrimaryColor).
				Foreground(styles.TextColor).
				Render(strategyLine))
		} else {
			b.WriteString(strategyLine)
		}
		b.WriteString("\n")
		lineCount++
	}

	if lineCount < availableLines {
		plansCountLine := fmt.Sprintf("  %d/%d plans ready", plansReady, totalCoordinators)
		if plansReady == totalCoordinators {
			b.WriteString(lipgloss.NewStyle().Foreground(styles.GreenColor).Render(plansCountLine))
		} else {
			b.WriteString(styles.Muted.Render(plansCountLine))
		}
		b.WriteString("\n")
		lineCount++
	}

	if session.Phase == orchestrator.PhasePlanSelection && lineCount < availableLines {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.YellowColor).Bold(true).Render("  Manager comparing plans..."))
		b.WriteString("\n")
		lineCount++

		if session.PlanManagerID != "" && lineCount < availableLines {
			inst := s.ctx.GetInstance(session.PlanManagerID)
			if inst != nil {
				var managerIcon string
				var managerStyle lipgloss.Style
				switch inst.Status {
				case orchestrator.StatusWorking:
					managerIcon = "⟳"
					managerStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
				case orchestrator.StatusCompleted:
					managerIcon = "✓"
					managerStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
				case orchestrator.StatusError:
					managerIcon = "✗"
					managerStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				default:
					managerIcon = "○"
					managerStyle = styles.Muted
				}
				managerLine := fmt.Sprintf("    %s Manager", managerStyle.Render(managerIcon))
				b.WriteString(managerLine)
				b.WriteString("\n")
				lineCount++
			}
		}
	}

	return lineCount
}

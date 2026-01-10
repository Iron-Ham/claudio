package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// renderUltraPlanSidebar renders the sidebar for ultra-plan mode with phase-aware navigation.
// This is the main dispatcher that orchestrates rendering for all ultraplan phases.
// Phase-specific rendering is delegated to:
//   - ultraplan_sidebar_planning.go: Planning phase functions
//   - ultraplan_sidebar_execution.go: Execution phase functions
//   - ultraplan_sidebar_consolidation.go: Synthesis and consolidation phase functions
func (m Model) renderUltraPlanSidebar(width int, height int) string {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return m.renderSidebar(width, height)
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderSidebar(width, height)
	}

	var b strings.Builder
	lineCount := 0
	availableLines := height - 4

	// ========== GROUP DECISION SECTION (if awaiting decision) ==========
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		// Render prominent decision dialog
		decisionContent := m.renderGroupDecisionSection(session.GroupDecision, width-4)
		b.WriteString(decisionContent)
		b.WriteString("\n\n")
		lineCount += 12 // Reserve space for decision section
	}

	// ========== PLANNING SECTION ==========
	planningComplete := session.Phase != orchestrator.PhasePlanning && session.Phase != orchestrator.PhasePlanSelection
	planningStatus := m.getPhaseSectionStatus(orchestrator.PhasePlanning, session)
	planningHeader := fmt.Sprintf("▼ PLANNING %s", planningStatus)
	b.WriteString(styles.SidebarTitle.Render(planningHeader))
	b.WriteString("\n")
	lineCount++

	// Check if multi-pass mode is enabled
	if session.Config.MultiPass {
		// Multi-pass planning: show all 3 strategy coordinators
		lineCount += m.renderMultiPassPlanningSection(&b, session, width, availableLines-lineCount)
	} else {
		// Single-pass planning: show single coordinator instance
		if session.CoordinatorID != "" && lineCount < availableLines {
			inst := m.orchestrator.GetInstance(session.CoordinatorID)
			selected := m.isInstanceSelected(session.CoordinatorID)
			navigable := planningComplete || (inst != nil && inst.Status != orchestrator.StatusPending)
			line := m.renderPhaseInstanceLine(inst, "Coordinator", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
		}
	}
	b.WriteString("\n")
	lineCount++

	// ========== PLAN SELECTION SECTION (for multi-pass planning) ==========
	// Only show this section if multi-pass planning is being used (detected by presence of
	// PlanCoordinatorIDs, PlanManagerID, or being in PhasePlanSelection)
	isMultiPassPlanning := len(session.PlanCoordinatorIDs) > 0 ||
		session.PlanManagerID != "" ||
		session.Phase == orchestrator.PhasePlanSelection

	if isMultiPassPlanning && lineCount < availableLines {
		selStatus := m.getPhaseSectionStatus(orchestrator.PhasePlanSelection, session)
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

		// Show plan selection status with more detail
		switch session.Phase {
		case orchestrator.PhasePlanning:
			// Still planning - show pending status
			b.WriteString(styles.Muted.Render("  ○ Awaiting candidate plans"))
		case orchestrator.PhasePlanSelection:
			// Actively comparing plans
			numCandidates := len(session.CandidatePlans)
			if numCandidates > 0 {
				b.WriteString(styles.Muted.Render(fmt.Sprintf("  ⟳ Comparing %d plans...", numCandidates)))
			} else {
				b.WriteString(styles.Muted.Render("  ⟳ Comparing plans..."))
			}
		default:
			// Plan selected - show which one if available
			if session.SelectedPlanIndex >= 0 {
				b.WriteString(styles.Muted.Render(fmt.Sprintf("  ✓ Plan %d selected", session.SelectedPlanIndex+1)))
			} else {
				b.WriteString(styles.Muted.Render("  ✓ Best plan selected"))
			}
		}
		b.WriteString("\n")
		lineCount++

		// Show plan manager instance if present
		if session.PlanManagerID != "" && lineCount < availableLines {
			inst := m.orchestrator.GetInstance(session.PlanManagerID)
			selected := m.isInstanceSelected(session.PlanManagerID)
			navigable := inst != nil && inst.Status != orchestrator.StatusPending
			line := m.renderPhaseInstanceLine(inst, "Plan Manager", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++
		}

		b.WriteString("\n")
		lineCount++
	}

	// ========== EXECUTION SECTION ==========
	if session.Plan != nil && lineCount < availableLines {
		executionStarted := session.Phase == orchestrator.PhaseExecuting ||
			session.Phase == orchestrator.PhaseSynthesis ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		execStatus := m.getPhaseSectionStatus(orchestrator.PhaseExecuting, session)
		execHeader := fmt.Sprintf("▼ EXECUTION %s", execStatus)
		b.WriteString(styles.SidebarTitle.Render(execHeader))
		b.WriteString("\n")
		lineCount++

		// Show execution groups with tasks
		for groupIdx, group := range session.Plan.ExecutionOrder {
			if lineCount >= availableLines-4 { // Reserve space for synthesis/consolidation
				b.WriteString(styles.Muted.Render("  ...more"))
				b.WriteString("\n")
				lineCount++
				break
			}

			// Group header
			groupStatus := m.getGroupStatus(session, group)
			groupHeader := fmt.Sprintf("  Group %d %s", groupIdx+1, groupStatus)
			if !executionStarted {
				b.WriteString(styles.Muted.Render(groupHeader))
			} else {
				b.WriteString(groupHeader)
			}
			b.WriteString("\n")
			lineCount++

			// Tasks in group (compact view)
			for _, taskID := range group {
				if lineCount >= availableLines-4 {
					break
				}

				task := session.GetTask(taskID)
				if task == nil {
					continue
				}

				instID := m.findInstanceIDForTask(session, taskID)
				selected := m.isInstanceSelected(instID)
				navigable := instID != ""
				taskLine := m.renderExecutionTaskLine(session, task, instID, selected, navigable, width-6)
				b.WriteString(taskLine)
				b.WriteString("\n")
				lineCount++
			}

			// Show group consolidator instance if it exists
			if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
				consolidatorID := session.GroupConsolidatorIDs[groupIdx]
				inst := m.orchestrator.GetInstance(consolidatorID)
				selected := m.isInstanceSelected(consolidatorID)
				navigable := true // Always navigable once created
				consolidatorLine := m.renderGroupConsolidatorLine(inst, groupIdx, selected, navigable, width-6)
				b.WriteString(consolidatorLine)
				b.WriteString("\n")
				lineCount++
			}
		}
		b.WriteString("\n")
		lineCount++
	}

	// ========== SYNTHESIS SECTION ==========
	if lineCount < availableLines {
		synthesisStarted := session.Phase == orchestrator.PhaseSynthesis ||
			session.Phase == orchestrator.PhaseRevision ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		synthStatus := m.getPhaseSectionStatus(orchestrator.PhaseSynthesis, session)
		synthHeader := fmt.Sprintf("▼ SYNTHESIS %s", synthStatus)
		if !synthesisStarted && session.SynthesisID == "" {
			b.WriteString(styles.Muted.Render(synthHeader))
		} else {
			b.WriteString(styles.SidebarTitle.Render(synthHeader))
		}
		b.WriteString("\n")
		lineCount++

		// Show synthesis instance
		if session.SynthesisID != "" && lineCount < availableLines {
			inst := m.orchestrator.GetInstance(session.SynthesisID)
			selected := m.isInstanceSelected(session.SynthesisID)
			navigable := true // Always navigable once created
			line := m.renderPhaseInstanceLine(inst, "Reviewer", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++

			// Show synthesis findings when awaiting approval
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

				// Show prompt to approve
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
	}

	// ========== REVISION SECTION ==========
	if lineCount < availableLines {
		revisionStarted := session.Phase == orchestrator.PhaseRevision ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		// Only show revision section if there are/were issues
		hasRevision := session.Revision != nil && len(session.Revision.Issues) > 0

		if hasRevision || session.Phase == orchestrator.PhaseRevision {
			revStatus := m.getPhaseSectionStatus(orchestrator.PhaseRevision, session)
			revHeader := fmt.Sprintf("▼ REVISION %s", revStatus)
			if !revisionStarted && session.RevisionID == "" {
				b.WriteString(styles.Muted.Render(revHeader))
			} else {
				b.WriteString(styles.SidebarTitle.Render(revHeader))
			}
			b.WriteString("\n")
			lineCount++

			// Show revision info
			if session.Revision != nil && lineCount < availableLines {
				// Show round number
				roundInfo := fmt.Sprintf("  Round %d/%d", session.Revision.RevisionRound, session.Revision.MaxRevisions)
				b.WriteString(styles.Muted.Render(roundInfo))
				b.WriteString("\n")
				lineCount++

				// Show issues count
				issueInfo := fmt.Sprintf("  %d issue(s) to address", len(session.Revision.Issues))
				b.WriteString(styles.Muted.Render(issueInfo))
				b.WriteString("\n")
				lineCount++

				// Show revision instance if active
				if session.RevisionID != "" && lineCount < availableLines {
					inst := m.orchestrator.GetInstance(session.RevisionID)
					selected := m.isInstanceSelected(session.RevisionID)
					navigable := true
					line := m.renderPhaseInstanceLine(inst, "Reviser", selected, navigable, width-4)
					b.WriteString(line)
					b.WriteString("\n")
					lineCount++
				}

				// Show tasks being revised
				if len(session.Revision.TasksToRevise) > 0 && lineCount < availableLines {
					for _, taskID := range session.Revision.TasksToRevise {
						if lineCount >= availableLines-2 {
							break
						}
						task := session.GetTask(taskID)
						if task == nil {
							continue
						}
						// Check if revised
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
		}
	}

	// ========== CONSOLIDATION SECTION ==========
	if session.Config.ConsolidationMode != "" && lineCount < availableLines {
		consolidationStarted := session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		consStatus := m.getPhaseSectionStatus(orchestrator.PhaseConsolidating, session)
		consHeader := fmt.Sprintf("▼ CONSOLIDATION %s", consStatus)
		if !consolidationStarted && session.ConsolidationID == "" {
			b.WriteString(styles.Muted.Render(consHeader))
		} else {
			b.WriteString(styles.SidebarTitle.Render(consHeader))
		}
		b.WriteString("\n")
		lineCount++

		// Show consolidation instance
		if session.ConsolidationID != "" && lineCount < availableLines {
			inst := m.orchestrator.GetInstance(session.ConsolidationID)
			selected := m.isInstanceSelected(session.ConsolidationID)
			navigable := true // Always navigable once created
			line := m.renderPhaseInstanceLine(inst, "Consolidator", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++

			// Show PR count if available
			if len(session.PRUrls) > 0 && lineCount < availableLines {
				prLine := fmt.Sprintf("    %d PR(s) created", len(session.PRUrls))
				b.WriteString(styles.Muted.Render(prLine))
				b.WriteString("\n")
				lineCount++
			}
		} else if !consolidationStarted && lineCount < availableLines {
			b.WriteString(styles.Muted.Render("  ○ Pending"))
			b.WriteString("\n")
			lineCount++
		}
	}

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// getPhaseSectionStatus returns a status indicator for a phase section header.
// This is a common utility used across all phase sections.
func (m Model) getPhaseSectionStatus(phase orchestrator.UltraPlanPhase, session *orchestrator.UltraPlanSession) string {
	switch phase {
	case orchestrator.PhasePlanning:
		if session.Phase == orchestrator.PhasePlanning {
			return "[⟳]"
		}
		// Plan selection comes after planning but before refresh
		if session.Phase == orchestrator.PhasePlanSelection {
			return "[✓]"
		}
		if session.Plan != nil {
			return "[✓]"
		}
		return "[○]"

	case orchestrator.PhasePlanSelection:
		if session.Phase == orchestrator.PhasePlanSelection {
			return "[⟳]"
		}
		// Plan selection is complete once we move to refresh or later phases
		if session.Phase == orchestrator.PhaseRefresh ||
			session.Phase == orchestrator.PhaseExecuting ||
			session.Phase == orchestrator.PhaseSynthesis ||
			session.Phase == orchestrator.PhaseRevision ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete {
			return "[✓]"
		}
		return "[○]"

	case orchestrator.PhaseExecuting:
		if session.Plan == nil {
			return "[○]"
		}
		completed := len(session.CompletedTasks)
		total := len(session.Plan.Tasks)
		if session.Phase == orchestrator.PhaseExecuting {
			return fmt.Sprintf("[%d/%d]", completed, total)
		}
		if completed == total && total > 0 {
			return "[✓]"
		}
		if completed > 0 {
			return fmt.Sprintf("[%d/%d]", completed, total)
		}
		return "[○]"

	case orchestrator.PhaseSynthesis:
		if session.Phase == orchestrator.PhaseSynthesis {
			return "[⟳]"
		}
		if session.Phase == orchestrator.PhaseRevision || session.Phase == orchestrator.PhaseConsolidating || session.Phase == orchestrator.PhaseComplete {
			return "[✓]"
		}
		return "[○]"

	case orchestrator.PhaseRevision:
		if session.Phase == orchestrator.PhaseRevision {
			if session.Revision != nil {
				revised := len(session.Revision.RevisedTasks)
				total := len(session.Revision.TasksToRevise)
				return fmt.Sprintf("[%d/%d]", revised, total)
			}
			return "[⟳]"
		}
		if session.Phase == orchestrator.PhaseConsolidating || session.Phase == orchestrator.PhaseComplete {
			if session.Revision != nil && len(session.Revision.Issues) > 0 {
				return "[✓]"
			}
			return "[○]" // No revision was needed
		}
		return "[○]"

	case orchestrator.PhaseConsolidating:
		if session.Phase == orchestrator.PhaseConsolidating {
			return "[⟳]"
		}
		if session.Phase == orchestrator.PhaseComplete && len(session.PRUrls) > 0 {
			return "[✓]"
		}
		if session.Phase == orchestrator.PhaseComplete {
			return "[✓]"
		}
		return "[○]"

	default:
		return ""
	}
}

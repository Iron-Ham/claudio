package view

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// UltraPlanState holds ultra-plan specific UI state
type UltraPlanState struct {
	Coordinator            *orchestrator.Coordinator
	ShowPlanView           bool                            // Toggle between plan view and normal output view
	SelectedTaskIdx        int                             // Currently selected task index for navigation
	NeedsNotification      bool                            // Set when user input is needed (checked on tick)
	LastNotifiedPhase      orchestrator.UltraPlanPhase     // Prevent duplicate notifications for same phase
	LastConsolidationPhase orchestrator.ConsolidationPhase // Track consolidation phase for pause detection
	NotifiedGroupDecision  bool                            // Prevent repeated notifications while awaiting group decision

	// Phase-aware navigation state
	NavigableInstances []string // Ordered list of navigable instance IDs
	SelectedNavIdx     int      // Index into navigableInstances
}

// RenderContext provides the necessary context for rendering ultraplan views.
// This allows the view to access orchestrator and session state without
// direct coupling to the Model struct.
type RenderContext struct {
	Orchestrator  *orchestrator.Orchestrator
	Session       *orchestrator.Session
	UltraPlan     *UltraPlanState
	ActiveTab     int
	Width         int
	Height        int
	Outputs       map[string]string
	GetInstance   func(id string) *orchestrator.Instance
	IsSelected    func(instanceID string) bool
}

// UltraplanView handles rendering of ultra-plan UI components including
// phase display, task graph visualization, and progress indicators.
type UltraplanView struct {
	ctx *RenderContext
}

// NewUltraplanView creates a new ultraplan view with the given render context.
func NewUltraplanView(ctx *RenderContext) *UltraplanView {
	return &UltraplanView{ctx: ctx}
}

// Render renders the main ultraplan view based on the current state.
// Returns an empty string if not in ultraplan mode.
func (v *UltraplanView) Render() string {
	if v.ctx.UltraPlan == nil || v.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := v.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	// Build the complete ultraplan view
	var b strings.Builder
	b.WriteString(v.RenderHeader())
	b.WriteString("\n")
	// Content would be rendered separately based on context
	return b.String()
}

// RenderHeader renders the ultra-plan header with phase and progress
func (v *UltraplanView) RenderHeader() string {
	if v.ctx.UltraPlan == nil || v.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := v.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var b strings.Builder

	// Title with ultra-plan indicator
	title := fmt.Sprintf("Claudio Ultra-Plan: %s", truncate(session.Objective, 40))

	// Phase indicator
	phaseStr := PhaseToString(session.Phase)
	pStyle := PhaseStyle(session.Phase)

	// Build phase-appropriate progress display
	progressDisplay := v.buildProgressDisplay(session)

	// Combine
	header := fmt.Sprintf("%s  [%s]  %s",
		title,
		pStyle.Render(phaseStr),
		progressDisplay,
	)

	b.WriteString(styles.Header.Width(v.ctx.Width).Render(header))
	return b.String()
}

// buildProgressDisplay builds the progress display string based on current phase
func (v *UltraplanView) buildProgressDisplay(session *orchestrator.UltraPlanSession) string {
	switch session.Phase {
	case orchestrator.PhasePlanning:
		return "analyzing codebase..."

	case orchestrator.PhasePlanSelection:
		return "comparing plans..."

	case orchestrator.PhaseRefresh:
		return "plan ready"

	case orchestrator.PhaseExecuting:
		progress := session.Progress()
		progressBar := RenderProgressBar(int(progress), 20)
		return fmt.Sprintf("%s %.0f%%", progressBar, progress)

	case orchestrator.PhaseSynthesis:
		return "reviewing..."

	case orchestrator.PhaseRevision:
		if session.Revision != nil {
			revised := len(session.Revision.RevisedTasks)
			total := len(session.Revision.TasksToRevise)
			if total > 0 {
				pct := float64(revised) / float64(total) * 100
				progressBar := RenderProgressBar(int(pct), 20)
				return fmt.Sprintf("%s round %d (%d/%d)", progressBar, session.Revision.RevisionRound, revised, total)
			}
			return fmt.Sprintf("round %d", session.Revision.RevisionRound)
		}
		return "addressing issues..."

	case orchestrator.PhaseConsolidating:
		if session.Consolidation != nil && session.Consolidation.TotalGroups > 0 {
			pct := float64(session.Consolidation.CurrentGroup) / float64(session.Consolidation.TotalGroups) * 100
			progressBar := RenderProgressBar(int(pct), 20)
			return fmt.Sprintf("%s %.0f%%", progressBar, pct)
		}
		return "consolidating..."

	case orchestrator.PhaseComplete:
		if len(session.PRUrls) > 0 {
			return fmt.Sprintf("%d PR(s) created", len(session.PRUrls))
		}
		progressBar := RenderProgressBar(100, 20)
		return fmt.Sprintf("%s 100%%", progressBar)

	case orchestrator.PhaseFailed:
		return "failed"

	default:
		progress := session.Progress()
		progressBar := RenderProgressBar(int(progress), 20)
		return fmt.Sprintf("%s %.0f%%", progressBar, progress)
	}
}

// RenderSidebar renders a unified sidebar showing all phases with their instances
func (v *UltraplanView) RenderSidebar(width int, height int) string {
	if v.ctx.UltraPlan == nil || v.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := v.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var b strings.Builder
	lineCount := 0
	availableLines := height - 4

	// ========== GROUP DECISION SECTION (if awaiting decision) ==========
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		decisionContent := v.renderGroupDecisionSection(session.GroupDecision, width-4)
		b.WriteString(decisionContent)
		b.WriteString("\n\n")
		lineCount += 12
	}

	// ========== PLANNING SECTION ==========
	planningComplete := session.Phase != orchestrator.PhasePlanning && session.Phase != orchestrator.PhasePlanSelection
	planningStatus := v.getPhaseSectionStatus(orchestrator.PhasePlanning, session)
	planningHeader := fmt.Sprintf("▼ PLANNING %s", planningStatus)
	b.WriteString(styles.SidebarTitle.Render(planningHeader))
	b.WriteString("\n")
	lineCount++

	// Check if multi-pass mode is enabled
	if session.Config.MultiPass {
		lineCount += v.renderMultiPassPlanningSection(&b, session, width, availableLines-lineCount)
	} else {
		if session.CoordinatorID != "" && lineCount < availableLines {
			inst := v.ctx.GetInstance(session.CoordinatorID)
			selected := v.ctx.IsSelected(session.CoordinatorID)
			navigable := planningComplete || (inst != nil && inst.Status != orchestrator.StatusPending)
			line := v.renderPhaseInstanceLine(inst, "Coordinator", selected, navigable, width-4)
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
		selStatus := v.getPhaseSectionStatus(orchestrator.PhasePlanSelection, session)
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
			inst := v.ctx.GetInstance(session.PlanManagerID)
			selected := v.ctx.IsSelected(session.PlanManagerID)
			navigable := inst != nil && inst.Status != orchestrator.StatusPending
			line := v.renderPhaseInstanceLine(inst, "Plan Manager", selected, navigable, width-4)
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

		execStatus := v.getPhaseSectionStatus(orchestrator.PhaseExecuting, session)
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

			groupStatus := v.getGroupStatus(session, group)
			groupHeader := fmt.Sprintf("  Group %d %s", groupIdx+1, groupStatus)
			if !executionStarted {
				b.WriteString(styles.Muted.Render(groupHeader))
			} else {
				b.WriteString(groupHeader)
			}
			b.WriteString("\n")
			lineCount++

			for _, taskID := range group {
				if lineCount >= availableLines-4 {
					break
				}

				task := session.GetTask(taskID)
				if task == nil {
					continue
				}

				instID := v.findInstanceIDForTask(session, taskID)
				selected := v.ctx.IsSelected(instID)
				navigable := instID != ""
				taskLine := v.renderExecutionTaskLine(session, task, instID, selected, navigable, width-6)
				b.WriteString(taskLine)
				b.WriteString("\n")
				lineCount++
			}

			if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
				consolidatorID := session.GroupConsolidatorIDs[groupIdx]
				inst := v.ctx.GetInstance(consolidatorID)
				selected := v.ctx.IsSelected(consolidatorID)
				navigable := true
				consolidatorLine := v.renderGroupConsolidatorLine(inst, groupIdx, selected, navigable, width-6)
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

		synthStatus := v.getPhaseSectionStatus(orchestrator.PhaseSynthesis, session)
		synthHeader := fmt.Sprintf("▼ SYNTHESIS %s", synthStatus)
		if !synthesisStarted && session.SynthesisID == "" {
			b.WriteString(styles.Muted.Render(synthHeader))
		} else {
			b.WriteString(styles.SidebarTitle.Render(synthHeader))
		}
		b.WriteString("\n")
		lineCount++

		if session.SynthesisID != "" && lineCount < availableLines {
			inst := v.ctx.GetInstance(session.SynthesisID)
			selected := v.ctx.IsSelected(session.SynthesisID)
			navigable := true
			line := v.renderPhaseInstanceLine(inst, "Reviewer", selected, navigable, width-4)
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
	}

	// ========== REVISION SECTION ==========
	if lineCount < availableLines {
		revisionStarted := session.Phase == orchestrator.PhaseRevision ||
			session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		hasRevision := session.Revision != nil && len(session.Revision.Issues) > 0

		if hasRevision || session.Phase == orchestrator.PhaseRevision {
			revStatus := v.getPhaseSectionStatus(orchestrator.PhaseRevision, session)
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
					inst := v.ctx.GetInstance(session.RevisionID)
					selected := v.ctx.IsSelected(session.RevisionID)
					navigable := true
					line := v.renderPhaseInstanceLine(inst, "Reviser", selected, navigable, width-4)
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
		}
	}

	// ========== CONSOLIDATION SECTION ==========
	if session.Config.ConsolidationMode != "" && lineCount < availableLines {
		consolidationStarted := session.Phase == orchestrator.PhaseConsolidating ||
			session.Phase == orchestrator.PhaseComplete

		consStatus := v.getPhaseSectionStatus(orchestrator.PhaseConsolidating, session)
		consHeader := fmt.Sprintf("▼ CONSOLIDATION %s", consStatus)
		if !consolidationStarted && session.ConsolidationID == "" {
			b.WriteString(styles.Muted.Render(consHeader))
		} else {
			b.WriteString(styles.SidebarTitle.Render(consHeader))
		}
		b.WriteString("\n")
		lineCount++

		if session.ConsolidationID != "" && lineCount < availableLines {
			inst := v.ctx.GetInstance(session.ConsolidationID)
			selected := v.ctx.IsSelected(session.ConsolidationID)
			navigable := true
			line := v.renderPhaseInstanceLine(inst, "Consolidator", selected, navigable, width-4)
			b.WriteString(line)
			b.WriteString("\n")
			lineCount++

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

// renderGroupDecisionSection renders the group decision dialog when user input is needed
func (v *UltraplanView) renderGroupDecisionSection(decision *orchestrator.GroupDecisionState, maxWidth int) string {
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

// renderPhaseInstanceLine renders a line for a phase instance (coordinator, synthesis, consolidation)
func (v *UltraplanView) renderPhaseInstanceLine(inst *orchestrator.Instance, name string, selected, navigable bool, maxWidth int) string {
	var statusIcon string
	var statusStyle lipgloss.Style

	if inst == nil {
		statusIcon = "○"
		statusStyle = styles.Muted
	} else {
		switch inst.Status {
		case orchestrator.StatusWorking:
			statusIcon = "⟳"
			statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
		case orchestrator.StatusCompleted, orchestrator.StatusWaitingInput:
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
	}

	line := fmt.Sprintf("  %s %s", statusStyle.Render(statusIcon), name)

	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	} else if !navigable {
		line = styles.Muted.Render(line)
	}

	return line
}

// renderExecutionTaskLine renders a task line in the execution section
func (v *UltraplanView) renderExecutionTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, instanceID string, selected, navigable bool, maxWidth int) string {
	var statusIcon string
	var statusStyle lipgloss.Style

	for _, ct := range session.CompletedTasks {
		if ct == task.ID {
			statusIcon = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
			break
		}
	}

	if statusIcon == "" {
		for _, ft := range session.FailedTasks {
			if ft == task.ID {
				statusIcon = "✗"
				statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				break
			}
		}
	}

	if statusIcon == "" && instanceID != "" {
		statusIcon = "⟳"
		statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
	}

	if statusIcon == "" {
		statusIcon = "○"
		statusStyle = styles.Muted
	}

	titleLen := maxWidth - 6
	title := truncate(task.Title, titleLen)
	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), title)

	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	} else if !navigable {
		line = styles.Muted.Render(line)
	}

	return line
}

// renderGroupConsolidatorLine renders a consolidator line in the execution section
func (v *UltraplanView) renderGroupConsolidatorLine(inst *orchestrator.Instance, groupIndex int, selected, navigable bool, maxWidth int) string {
	var statusIcon string
	var statusStyle lipgloss.Style

	if inst == nil {
		statusIcon = "○"
		statusStyle = styles.Muted
	} else {
		switch inst.Status {
		case orchestrator.StatusCompleted:
			statusIcon = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
		case orchestrator.StatusError:
			statusIcon = "✗"
			statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
		case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
			statusIcon = "⟳"
			statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
		default:
			statusIcon = "○"
			statusStyle = styles.Muted
		}
	}

	title := fmt.Sprintf("Consolidator (Group %d)", groupIndex+1)
	titleLen := maxWidth - 6
	title = truncate(title, titleLen)
	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), title)

	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	} else if !navigable {
		line = styles.Muted.Render(line)
	}

	return line
}

// getGroupStatus returns a status indicator for a task group
func (v *UltraplanView) getGroupStatus(session *orchestrator.UltraPlanSession, group []string) string {
	allComplete := true
	anyRunning := false
	anyFailed := false

	for _, taskID := range group {
		completed := false
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				completed = true
				break
			}
		}

		failed := false
		for _, ft := range session.FailedTasks {
			if ft == taskID {
				failed = true
				break
			}
		}

		if failed {
			anyFailed = true
		}

		if !completed && !failed {
			allComplete = false
			if _, running := session.TaskToInstance[taskID]; running {
				anyRunning = true
			}
		}
	}

	if allComplete && !anyFailed {
		return "✓"
	}
	if anyFailed {
		return "✗"
	}
	if anyRunning {
		return "⟳"
	}
	return "○"
}

// getPhaseSectionStatus returns a status indicator for a phase section header
func (v *UltraplanView) getPhaseSectionStatus(phase orchestrator.UltraPlanPhase, session *orchestrator.UltraPlanSession) string {
	switch phase {
	case orchestrator.PhasePlanning:
		if session.Phase == orchestrator.PhasePlanning {
			return "[⟳]"
		}
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
			return "[○]"
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

// findInstanceIDForTask finds the instance ID associated with a task
func (v *UltraplanView) findInstanceIDForTask(session *orchestrator.UltraPlanSession, taskID string) string {
	if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
		return instID
	}

	for _, inst := range v.ctx.Session.Instances {
		if strings.Contains(inst.Task, taskID) {
			return inst.ID
		}
	}

	return ""
}

// renderMultiPassPlanningSection renders the multi-pass planning coordinators in the unified sidebar
func (v *UltraplanView) renderMultiPassPlanningSection(b *strings.Builder, session *orchestrator.UltraPlanSession, width int, availableLines int) int {
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
			inst := v.ctx.GetInstance(instID)
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
			isSelected = v.ctx.IsSelected(session.PlanCoordinatorIDs[i])
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
			inst := v.ctx.GetInstance(session.PlanManagerID)
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

// RenderPlanView renders the detailed plan view
func (v *UltraplanView) RenderPlanView(width int) string {
	session := v.ctx.UltraPlan.Coordinator.Session()
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

// RenderHelp renders the help bar for ultra-plan mode
func (v *UltraplanView) RenderHelp() string {
	if v.ctx.UltraPlan == nil {
		return ""
	}

	session := v.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var keys []string

	// Group decision mode takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		keys = append(keys, "[c] continue partial")
		keys = append(keys, "[r] retry failed")
		keys = append(keys, "[q] cancel")
		keys = append(keys, "[↑↓] nav")
		return styles.HelpBar.Width(v.ctx.Width).Render(strings.Join(keys, "  "))
	}

	// Common keys
	keys = append(keys, "[q] quit")
	keys = append(keys, "[↑↓] nav")

	// Phase-specific keys
	switch session.Phase {
	case orchestrator.PhasePlanning:
		keys = append(keys, "[p] parse plan")
		keys = append(keys, "[i] input mode")

	case orchestrator.PhasePlanSelection:
		keys = append(keys, "[v] toggle plan view")

	case orchestrator.PhaseRefresh:
		keys = append(keys, "[e] start execution")
		keys = append(keys, "[E] edit plan")

	case orchestrator.PhaseExecuting:
		keys = append(keys, "[tab] next task")
		keys = append(keys, "[1-9] select task")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[c] cancel")

	case orchestrator.PhaseSynthesis:
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		if session.SynthesisAwaitingApproval {
			keys = append(keys, "[s] approve → proceed")
		} else {
			keys = append(keys, "[s] skip → consolidate")
		}

	case orchestrator.PhaseRevision:
		keys = append(keys, "[tab] next instance")
		keys = append(keys, "[v] toggle plan view")
		if session.Revision != nil {
			keys = append(keys, fmt.Sprintf("round %d/%d", session.Revision.RevisionRound, session.Revision.MaxRevisions))
		}

	case orchestrator.PhaseConsolidating:
		keys = append(keys, "[v] toggle plan view")
		if session.Consolidation != nil && session.Consolidation.Phase == orchestrator.ConsolidationPaused {
			keys = append(keys, "[r] resume")
		}

	case orchestrator.PhaseComplete, orchestrator.PhaseFailed:
		keys = append(keys, "[v] view plan")
		if len(session.PRUrls) > 0 {
			keys = append(keys, "[o] open PR")
		}
	}

	return styles.HelpBar.Width(v.ctx.Width).Render(strings.Join(keys, "  "))
}

// RenderConsolidationSidebar renders the sidebar during the consolidation phase
func (v *UltraplanView) RenderConsolidationSidebar(width int, height int) string {
	session := v.ctx.UltraPlan.Coordinator.Session()
	if session == nil || session.Consolidation == nil {
		return ""
	}

	var b strings.Builder
	state := session.Consolidation

	// Title with phase indicator
	b.WriteString(styles.SidebarTitle.Render("Consolidation"))
	b.WriteString("\n\n")

	// Phase status
	phaseIcon := ConsolidationPhaseIcon(state.Phase)
	phaseDesc := ConsolidationPhaseDesc(state.Phase)
	statusLine := fmt.Sprintf("%s %s", phaseIcon, phaseDesc)
	b.WriteString(statusLine)
	b.WriteString("\n\n")

	// Progress: Groups
	if state.TotalGroups > 0 {
		groupProgress := fmt.Sprintf("Groups: %d/%d", state.CurrentGroup, state.TotalGroups)
		b.WriteString(styles.Muted.Render(groupProgress))
		b.WriteString("\n")

		for i, branch := range state.GroupBranches {
			prefix := "  ✓ "
			if i == state.CurrentGroup-1 && state.Phase != orchestrator.ConsolidationComplete {
				prefix = "  ⟳ "
			}
			branchDisplay := truncate(branch, width-8)
			b.WriteString(styles.Muted.Render(prefix + branchDisplay))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Current task being merged
	if state.CurrentTask != "" {
		b.WriteString(styles.Muted.Render("Merging:"))
		b.WriteString("\n")
		taskDisplay := truncate(state.CurrentTask, width-6)
		b.WriteString(styles.Muted.Render("  " + taskDisplay))
		b.WriteString("\n\n")
	}

	// PRs created
	if len(state.PRUrls) > 0 {
		b.WriteString(styles.SidebarTitle.Render("Pull Requests"))
		b.WriteString("\n")
		for i, prURL := range state.PRUrls {
			prefix := "  ✓ "
			prDisplay := fmt.Sprintf("PR #%d", i+1)
			if idx := strings.LastIndex(prURL, "/"); idx >= 0 {
				prDisplay = "PR " + prURL[idx+1:]
			}
			b.WriteString(styles.Muted.Render(prefix + prDisplay))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Conflict info (if paused)
	if state.Phase == orchestrator.ConsolidationPaused && len(state.ConflictFiles) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.RedColor).Render("⚠ Conflict Detected"))
		b.WriteString("\n\n")
		b.WriteString(styles.Muted.Render("Files:"))
		b.WriteString("\n")
		maxFiles := 5
		for i, file := range state.ConflictFiles {
			if i >= maxFiles {
				remaining := len(state.ConflictFiles) - maxFiles
				b.WriteString(styles.Muted.Render(fmt.Sprintf("  ... +%d more", remaining)))
				b.WriteString("\n")
				break
			}
			fileDisplay := truncate(file, width-6)
			b.WriteString(styles.Muted.Render("  " + fileDisplay))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [r] to resume"))
		b.WriteString("\n")
	}

	// Error message
	if state.Error != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(styles.RedColor).Render("Error:"))
		b.WriteString("\n")
		errDisplay := truncate(state.Error, width-4)
		b.WriteString(styles.Muted.Render(errDisplay))
		b.WriteString("\n")
	}

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// Helper functions

// RenderProgressBar renders a simple ASCII progress bar
func RenderProgressBar(percent int, width int) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := (percent * width) / 100
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s]", bar)
}

// PhaseToString converts a phase to a display string
func PhaseToString(phase orchestrator.UltraPlanPhase) string {
	switch phase {
	case orchestrator.PhasePlanning:
		return "PLANNING"
	case orchestrator.PhasePlanSelection:
		return "SELECTING PLAN"
	case orchestrator.PhaseRefresh:
		return "READY"
	case orchestrator.PhaseExecuting:
		return "EXECUTING"
	case orchestrator.PhaseSynthesis:
		return "SYNTHESIS"
	case orchestrator.PhaseRevision:
		return "REVISION"
	case orchestrator.PhaseConsolidating:
		return "CONSOLIDATING"
	case orchestrator.PhaseComplete:
		return "COMPLETE"
	case orchestrator.PhaseFailed:
		return "FAILED"
	default:
		return string(phase)
	}
}

// PhaseStyle returns the style for a phase indicator
func PhaseStyle(phase orchestrator.UltraPlanPhase) lipgloss.Style {
	switch phase {
	case orchestrator.PhasePlanning:
		return lipgloss.NewStyle().Foreground(styles.BlueColor)
	case orchestrator.PhasePlanSelection:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	case orchestrator.PhaseRefresh:
		return lipgloss.NewStyle().Foreground(styles.YellowColor)
	case orchestrator.PhaseExecuting:
		return lipgloss.NewStyle().Foreground(styles.BlueColor).Bold(true)
	case orchestrator.PhaseSynthesis:
		return lipgloss.NewStyle().Foreground(styles.PurpleColor)
	case orchestrator.PhaseRevision:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	case orchestrator.PhaseConsolidating:
		return lipgloss.NewStyle().Foreground(styles.YellowColor).Bold(true)
	case orchestrator.PhaseComplete:
		return lipgloss.NewStyle().Foreground(styles.GreenColor)
	case orchestrator.PhaseFailed:
		return lipgloss.NewStyle().Foreground(styles.RedColor)
	default:
		return lipgloss.NewStyle()
	}
}

// ComplexityIndicator returns a visual indicator for task complexity
func ComplexityIndicator(complexity orchestrator.TaskComplexity) string {
	switch complexity {
	case orchestrator.ComplexityLow:
		return "◦"
	case orchestrator.ComplexityMedium:
		return "◎"
	case orchestrator.ComplexityHigh:
		return "●"
	default:
		return "○"
	}
}

// ConsolidationPhaseIcon returns an icon for the consolidation phase
func ConsolidationPhaseIcon(phase orchestrator.ConsolidationPhase) string {
	switch phase {
	case orchestrator.ConsolidationIdle:
		return "○"
	case orchestrator.ConsolidationDetecting:
		return "⟳"
	case orchestrator.ConsolidationCreatingBranches:
		return "⟳"
	case orchestrator.ConsolidationMergingTasks:
		return "⟳"
	case orchestrator.ConsolidationPushing:
		return "⟳"
	case orchestrator.ConsolidationCreatingPRs:
		return "⟳"
	case orchestrator.ConsolidationPaused:
		return "⏸"
	case orchestrator.ConsolidationComplete:
		return "✓"
	case orchestrator.ConsolidationFailed:
		return "✗"
	default:
		return "○"
	}
}

// ConsolidationPhaseDesc returns a human-readable description for the consolidation phase
func ConsolidationPhaseDesc(phase orchestrator.ConsolidationPhase) string {
	switch phase {
	case orchestrator.ConsolidationIdle:
		return "Waiting..."
	case orchestrator.ConsolidationDetecting:
		return "Detecting conflicts..."
	case orchestrator.ConsolidationCreatingBranches:
		return "Creating branches..."
	case orchestrator.ConsolidationMergingTasks:
		return "Merging tasks..."
	case orchestrator.ConsolidationPushing:
		return "Pushing to remote..."
	case orchestrator.ConsolidationCreatingPRs:
		return "Creating PRs..."
	case orchestrator.ConsolidationPaused:
		return "Paused (conflict)"
	case orchestrator.ConsolidationComplete:
		return "Complete"
	case orchestrator.ConsolidationFailed:
		return "Failed"
	default:
		return string(phase)
	}
}

// OpenURL opens the given URL in the default browser
func OpenURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}


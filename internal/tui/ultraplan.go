package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UltraPlanState holds ultra-plan specific UI state
type UltraPlanState struct {
	coordinator            *orchestrator.Coordinator
	showPlanView           bool                            // Toggle between plan view and normal output view
	selectedTaskIdx        int                             // Currently selected task index for navigation
	needsNotification      bool                            // Set when user input is needed (checked on tick)
	lastNotifiedPhase      orchestrator.UltraPlanPhase     // Prevent duplicate notifications for same phase
	lastConsolidationPhase orchestrator.ConsolidationPhase // Track consolidation phase for pause detection
	notifiedGroupDecision  bool                            // Prevent repeated notifications while awaiting group decision

	// Phase-aware navigation state
	navigableInstances []string // Ordered list of navigable instance IDs
	selectedNavIdx     int      // Index into navigableInstances
}

// checkForPhaseNotification checks if the ultraplan phase changed to one that needs user attention
// and sets the notification flag if needed. Call this from the tick handler.
func (m *Model) checkForPhaseNotification() {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return
	}

	// Check for synthesis phase (user may want to review)
	if session.Phase == orchestrator.PhaseSynthesis && m.ultraPlan.lastNotifiedPhase != orchestrator.PhaseSynthesis {
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseSynthesis
		return
	}

	// Check for revision phase (issues were found, user may want to know)
	if session.Phase == orchestrator.PhaseRevision && m.ultraPlan.lastNotifiedPhase != orchestrator.PhaseRevision {
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRevision
		return
	}

	// Check for consolidation pause (conflict detected, needs user attention)
	if session.Phase == orchestrator.PhaseConsolidating && session.Consolidation != nil {
		if session.Consolidation.Phase == orchestrator.ConsolidationPaused &&
			m.ultraPlan.lastConsolidationPhase != orchestrator.ConsolidationPaused {
			m.ultraPlan.needsNotification = true
			m.ultraPlan.lastConsolidationPhase = orchestrator.ConsolidationPaused
			return
		}
		// Track consolidation phase changes
		m.ultraPlan.lastConsolidationPhase = session.Consolidation.Phase
	}

	// Check for group decision needed (partial success/failure)
	// Only notify once when we enter the awaiting decision state
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		if !m.ultraPlan.notifiedGroupDecision {
			m.ultraPlan.needsNotification = true
			m.ultraPlan.notifiedGroupDecision = true
		}
		return
	}

	// Reset group decision notification flag when no longer awaiting
	// (so we can notify again if another group decision occurs later)
	if m.ultraPlan.notifiedGroupDecision {
		m.ultraPlan.notifiedGroupDecision = false
	}
}

// renderUltraPlanHeader renders the ultra-plan header with phase and progress
func (m Model) renderUltraPlanHeader() string {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return m.renderHeader()
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderHeader()
	}

	var b strings.Builder

	// Title with ultra-plan indicator
	title := fmt.Sprintf("Claudio Ultra-Plan: %s", truncate(session.Objective, 40))

	// Phase indicator
	phaseStr := phaseToString(session.Phase)
	pStyle := phaseStyle(session.Phase)

	// Build phase-appropriate progress display
	var progressDisplay string
	switch session.Phase {
	case orchestrator.PhasePlanning:
		// During planning, show activity indicator instead of progress
		progressDisplay = "analyzing codebase..."

	case orchestrator.PhaseRefresh:
		// Plan ready, waiting for user to start execution
		progressDisplay = "plan ready"

	case orchestrator.PhaseExecuting:
		// During execution, show task progress
		progress := session.Progress()
		progressBar := renderProgressBar(int(progress), 20)
		progressDisplay = fmt.Sprintf("%s %.0f%%", progressBar, progress)

	case orchestrator.PhaseSynthesis:
		// During synthesis, show that review is in progress
		progressDisplay = "reviewing..."

	case orchestrator.PhaseRevision:
		// During revision, show revision progress
		if session.Revision != nil {
			revised := len(session.Revision.RevisedTasks)
			total := len(session.Revision.TasksToRevise)
			if total > 0 {
				pct := float64(revised) / float64(total) * 100
				progressBar := renderProgressBar(int(pct), 20)
				progressDisplay = fmt.Sprintf("%s round %d (%d/%d)", progressBar, session.Revision.RevisionRound, revised, total)
			} else {
				progressDisplay = fmt.Sprintf("round %d", session.Revision.RevisionRound)
			}
		} else {
			progressDisplay = "addressing issues..."
		}

	case orchestrator.PhaseConsolidating:
		// During consolidation, show consolidation progress if available
		if session.Consolidation != nil && session.Consolidation.TotalGroups > 0 {
			pct := float64(session.Consolidation.CurrentGroup) / float64(session.Consolidation.TotalGroups) * 100
			progressBar := renderProgressBar(int(pct), 20)
			progressDisplay = fmt.Sprintf("%s %.0f%%", progressBar, pct)
		} else {
			progressDisplay = "consolidating..."
		}

	case orchestrator.PhaseComplete:
		// Show completion with PR count if available
		if len(session.PRUrls) > 0 {
			progressDisplay = fmt.Sprintf("%d PR(s) created", len(session.PRUrls))
		} else {
			progressBar := renderProgressBar(100, 20)
			progressDisplay = fmt.Sprintf("%s 100%%", progressBar)
		}

	case orchestrator.PhaseFailed:
		progressDisplay = "failed"

	default:
		progress := session.Progress()
		progressBar := renderProgressBar(int(progress), 20)
		progressDisplay = fmt.Sprintf("%s %.0f%%", progressBar, progress)
	}

	// Combine
	header := fmt.Sprintf("%s  [%s]  %s",
		title,
		pStyle.Render(phaseStr),
		progressDisplay,
	)

	b.WriteString(styles.Header.Width(m.width).Render(header))
	return b.String()
}

// renderUltraPlanSidebar renders a unified sidebar showing all phases with their instances
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
	planningComplete := session.Phase != orchestrator.PhasePlanning
	planningStatus := m.getPhaseSectionStatus(orchestrator.PhasePlanning, session)
	planningHeader := fmt.Sprintf("▼ PLANNING %s", planningStatus)
	b.WriteString(styles.SidebarTitle.Render(planningHeader))
	b.WriteString("\n")
	lineCount++

	// Show coordinator instance
	if session.CoordinatorID != "" && lineCount < availableLines {
		inst := m.orchestrator.GetInstance(session.CoordinatorID)
		selected := m.isInstanceSelected(session.CoordinatorID)
		navigable := planningComplete || (inst != nil && inst.Status != orchestrator.StatusPending)
		line := m.renderPhaseInstanceLine(inst, "Coordinator", selected, navigable, width-4)
		b.WriteString(line)
		b.WriteString("\n")
		lineCount++
	}
	b.WriteString("\n")
	lineCount++

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

// getGroupStatus returns a status indicator for a task group
func (m Model) getGroupStatus(session *orchestrator.UltraPlanSession, group []string) string {
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
func (m Model) getPhaseSectionStatus(phase orchestrator.UltraPlanPhase, session *orchestrator.UltraPlanSession) string {
	switch phase {
	case orchestrator.PhasePlanning:
		if session.Phase == orchestrator.PhasePlanning {
			return "[⟳]"
		}
		if session.Plan != nil {
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

// findInstanceIDForTask finds the instance ID associated with a task.
// It first checks the active TaskToInstance mapping, then searches through
// all instances for completed/failed tasks that are no longer in the mapping.
func (m Model) findInstanceIDForTask(session *orchestrator.UltraPlanSession, taskID string) string {
	// First check the active TaskToInstance mapping
	if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
		return instID
	}

	// For completed/failed tasks, search through all instances
	for _, inst := range m.session.Instances {
		if strings.Contains(inst.Task, taskID) {
			return inst.ID
		}
	}

	return ""
}

// isInstanceSelected checks if the given instance ID is currently selected in the TUI
func (m Model) isInstanceSelected(instanceID string) bool {
	if instanceID == "" {
		return false
	}
	if m.activeTab >= 0 && m.activeTab < len(m.session.Instances) {
		return m.session.Instances[m.activeTab].ID == instanceID
	}
	return false
}

// renderPhaseInstanceLine renders a line for a phase instance (coordinator, synthesis, consolidation)
func (m Model) renderPhaseInstanceLine(inst *orchestrator.Instance, name string, selected, navigable bool, maxWidth int) string {
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

	// Apply styling based on navigability and selection
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
func (m Model) renderExecutionTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, instanceID string, selected, navigable bool, maxWidth int) string {
	// Determine task status
	var statusIcon string
	var statusStyle lipgloss.Style

	// Check if completed
	for _, ct := range session.CompletedTasks {
		if ct == task.ID {
			statusIcon = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
			break
		}
	}

	// Check if failed
	if statusIcon == "" {
		for _, ft := range session.FailedTasks {
			if ft == task.ID {
				statusIcon = "✗"
				statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
				break
			}
		}
	}

	// Check if running
	if statusIcon == "" && instanceID != "" {
		statusIcon = "⟳"
		statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
	}

	// Default: pending
	if statusIcon == "" {
		statusIcon = "○"
		statusStyle = styles.Muted
	}

	// Build line with truncated title
	titleLen := maxWidth - 6 // status + spaces
	title := truncate(task.Title, titleLen)
	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), title)

	// Apply styling
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
func (m Model) renderGroupConsolidatorLine(inst *orchestrator.Instance, groupIndex int, selected, navigable bool, maxWidth int) string {
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

	// Build line
	title := fmt.Sprintf("Consolidator (Group %d)", groupIndex+1)
	titleLen := maxWidth - 6
	title = truncate(title, titleLen)
	line := fmt.Sprintf("    %s %s", statusStyle.Render(statusIcon), title)

	// Apply styling
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

// getNavigableInstances returns an ordered list of instance IDs that can be navigated to.
// Only includes instances from phases that have started or completed.
// Order: Planning → Execution tasks (in order) → Synthesis → Consolidation
func (m *Model) getNavigableInstances() []string {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return nil
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return nil
	}

	var instances []string

	// Planning - navigable once started (has instance)
	if session.CoordinatorID != "" {
		inst := m.orchestrator.GetInstance(session.CoordinatorID)
		if inst != nil && inst.Status != orchestrator.StatusPending {
			instances = append(instances, session.CoordinatorID)
		}
	}

	// Execution - navigable for tasks with instances (started or completed)
	if session.Plan != nil {
		// Add in execution order
		for groupIdx, group := range session.Plan.ExecutionOrder {
			for _, taskID := range group {
				// Check if task has an instance (either still in TaskToInstance or was completed)
				if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
					instances = append(instances, instID)
				} else {
					// Task might be completed - find instance by checking completed tasks
					for _, completedTaskID := range session.CompletedTasks {
						if completedTaskID == taskID {
							// Find instance for this completed task
							for _, inst := range m.session.Instances {
								if strings.Contains(inst.Task, taskID) {
									instances = append(instances, inst.ID)
									break
								}
							}
							break
						}
					}
				}
			}

			// Add group consolidator instance if it exists
			if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
				instances = append(instances, session.GroupConsolidatorIDs[groupIdx])
			}
		}
	}

	// Synthesis - navigable once created
	if session.SynthesisID != "" {
		instances = append(instances, session.SynthesisID)
	}

	// Revision - navigable once created
	if session.RevisionID != "" {
		instances = append(instances, session.RevisionID)
	}

	// Consolidation - navigable once created
	if session.ConsolidationID != "" {
		instances = append(instances, session.ConsolidationID)
	}

	return instances
}

// updateNavigableInstances updates the list of navigable instances
func (m *Model) updateNavigableInstances() {
	if m.ultraPlan == nil {
		return
	}
	m.ultraPlan.navigableInstances = m.getNavigableInstances()
}

// navigateToNextInstance navigates to the next navigable instance
// direction: +1 for next, -1 for previous
func (m *Model) navigateToNextInstance(direction int) bool {
	if m.ultraPlan == nil {
		return false
	}

	// Update the navigable instances list
	m.updateNavigableInstances()
	instances := m.ultraPlan.navigableInstances

	if len(instances) == 0 {
		return false
	}

	// Find current position in the list
	currentIdx := -1
	if m.activeTab >= 0 && m.activeTab < len(m.session.Instances) {
		currentInstID := m.session.Instances[m.activeTab].ID
		for i, instID := range instances {
			if instID == currentInstID {
				currentIdx = i
				break
			}
		}
	}

	// Calculate next index with wrapping
	var nextIdx int
	if currentIdx < 0 {
		// Not currently on a navigable instance, start from beginning or end
		if direction > 0 {
			nextIdx = 0
		} else {
			nextIdx = len(instances) - 1
		}
	} else {
		nextIdx = (currentIdx + direction + len(instances)) % len(instances)
	}

	// Find the instance in session.Instances and switch to it
	targetInstID := instances[nextIdx]
	for i, inst := range m.session.Instances {
		if inst.ID == targetInstID {
			m.activeTab = i
			m.ultraPlan.selectedNavIdx = nextIdx
			m.ensureActiveVisible()
			return true
		}
	}

	return false
}

// selectInstanceByID selects an instance by its ID, if it's navigable
func (m *Model) selectInstanceByID(instanceID string) bool {
	if m.ultraPlan == nil || instanceID == "" {
		return false
	}

	// Update the navigable instances list
	m.updateNavigableInstances()
	instances := m.ultraPlan.navigableInstances

	// Check if this instance is navigable
	isNavigable := false
	navIdx := 0
	for i, instID := range instances {
		if instID == instanceID {
			isNavigable = true
			navIdx = i
			break
		}
	}

	if !isNavigable {
		return false
	}

	// Find and select the instance
	for i, inst := range m.session.Instances {
		if inst.ID == instanceID {
			m.activeTab = i
			m.ultraPlan.selectedNavIdx = navIdx
			m.ensureActiveVisible()
			return true
		}
	}

	return false
}

// renderGroupDecisionSection renders the group decision dialog when user input is needed
func (m Model) renderGroupDecisionSection(decision *orchestrator.GroupDecisionState, maxWidth int) string {
	var b strings.Builder

	// Warning header
	warningStyle := lipgloss.NewStyle().
		Foreground(styles.YellowColor).
		Bold(true)
	b.WriteString(warningStyle.Render("⚠ PARTIAL GROUP FAILURE"))
	b.WriteString("\n\n")

	// Group info
	b.WriteString(fmt.Sprintf("Group %d has mixed results:\n", decision.GroupIndex+1))

	// Succeeded tasks
	if len(decision.SucceededTasks) > 0 {
		successStyle := lipgloss.NewStyle().Foreground(styles.GreenColor)
		b.WriteString(successStyle.Render(fmt.Sprintf("  ✓ %d task(s) succeeded", len(decision.SucceededTasks))))
		b.WriteString("\n")
	}

	// Failed tasks
	if len(decision.FailedTasks) > 0 {
		failStyle := lipgloss.NewStyle().Foreground(styles.RedColor)
		b.WriteString(failStyle.Render(fmt.Sprintf("  ✗ %d task(s) failed", len(decision.FailedTasks))))
		b.WriteString("\n")

		// List failed task IDs (truncated)
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

	// Options
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

// renderTaskLine renders a single task line in the sidebar
func (m Model) renderTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, maxWidth int, selected bool) string {
	// Determine task status
	status := "○" // pending
	statusStyle := styles.Muted

	for _, ct := range session.CompletedTasks {
		if ct == task.ID {
			status = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.GreenColor)
			break
		}
	}

	for _, ft := range session.FailedTasks {
		if ft == task.ID {
			status = "✗"
			statusStyle = lipgloss.NewStyle().Foreground(styles.RedColor)
			break
		}
	}

	if _, running := session.TaskToInstance[task.ID]; running {
		status = "⟳"
		statusStyle = lipgloss.NewStyle().Foreground(styles.BlueColor)
	}

	// Build task line
	titleLen := maxWidth - 4 // status + space + padding
	title := truncate(task.Title, titleLen)

	line := fmt.Sprintf("  %s %s", statusStyle.Render(status), title)

	// Highlight selected task
	if selected {
		line = lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor).
			Render(line)
	}

	return line
}

// renderUltraPlanContent renders the content area for ultra-plan mode
func (m Model) renderUltraPlanContent(width int) string {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return m.renderContent(width)
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderContent(width)
	}

	// If plan editor is active, render the plan editor view
	if m.IsPlanEditorActive() && session.Plan != nil {
		return m.renderPlanEditorView(width)
	}

	// If showing plan view, render the plan
	if m.ultraPlan.showPlanView && session.Plan != nil {
		return m.renderPlanView(width)
	}

	// Otherwise, show normal instance output with ultra-plan context
	return m.renderContent(width)
}

// renderPlanView renders the detailed plan view
func (m Model) renderPlanView(width int) string {
	session := m.ultraPlan.coordinator.Session()
	if session == nil || session.Plan == nil {
		return "No plan available"
	}

	plan := session.Plan
	var b strings.Builder

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
				complexity := complexityIndicator(task.EstComplexity)
				b.WriteString(fmt.Sprintf("  [%s] %s %s\n", task.ID, complexity, task.Title))
				if len(task.Files) > 0 {
					b.WriteString(fmt.Sprintf("      Files: %s\n", strings.Join(task.Files, ", ")))
				}
			}
		}
	}

	return styles.OutputArea.Width(width - 2).Render(b.String())
}

// renderUltraPlanHelp renders the help bar for ultra-plan mode
func (m Model) renderUltraPlanHelp() string {
	if m.ultraPlan == nil {
		return m.renderHelp()
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return m.renderHelp()
	}

	var keys []string

	// Group decision mode takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		keys = append(keys, "[c] continue partial")
		keys = append(keys, "[r] retry failed")
		keys = append(keys, "[q] cancel")
		keys = append(keys, "[↑↓] nav")
		return styles.HelpBar.Width(m.width).Render(strings.Join(keys, "  "))
	}

	// Common keys
	keys = append(keys, "[q] quit")
	keys = append(keys, "[↑↓] nav")

	// Phase-specific keys
	switch session.Phase {
	case orchestrator.PhasePlanning:
		keys = append(keys, "[p] parse plan")
		keys = append(keys, "[i] input mode")

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
		keys = append(keys, "[s] done → consolidate")

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

	return styles.HelpBar.Width(m.width).Render(strings.Join(keys, "  "))
}

// renderProgressBar renders a simple ASCII progress bar
func renderProgressBar(percent int, width int) string {
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

// phaseToString converts a phase to a display string
func phaseToString(phase orchestrator.UltraPlanPhase) string {
	switch phase {
	case orchestrator.PhasePlanning:
		return "PLANNING"
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

// phaseStyle returns the style for a phase indicator
func phaseStyle(phase orchestrator.UltraPlanPhase) lipgloss.Style {
	switch phase {
	case orchestrator.PhasePlanning:
		return lipgloss.NewStyle().Foreground(styles.BlueColor)
	case orchestrator.PhaseRefresh:
		return lipgloss.NewStyle().Foreground(styles.YellowColor)
	case orchestrator.PhaseExecuting:
		return lipgloss.NewStyle().Foreground(styles.BlueColor).Bold(true)
	case orchestrator.PhaseSynthesis:
		return lipgloss.NewStyle().Foreground(styles.PurpleColor)
	case orchestrator.PhaseRevision:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true) // Orange for revision
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

// complexityIndicator returns a visual indicator for task complexity
func complexityIndicator(complexity orchestrator.TaskComplexity) string {
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

// handleUltraPlanKeypress handles ultra-plan specific key presses
// Returns (handled, model, cmd) where handled indicates if the key was processed
func (m Model) handleUltraPlanKeypress(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false, m, nil
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return false, m, nil
	}

	// Group decision handling takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		switch msg.String() {
		case "c":
			// Continue with partial work (successful tasks only)
			if err := m.ultraPlan.coordinator.ResumeWithPartialWork(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to continue: %v", err)
			} else {
				m.infoMessage = "Continuing with successful tasks..."
			}
			return true, m, nil

		case "r":
			// Retry failed tasks
			if err := m.ultraPlan.coordinator.RetryFailedTasks(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to retry: %v", err)
			} else {
				m.infoMessage = "Retrying failed tasks..."
			}
			return true, m, nil

		case "q":
			// Cancel the ultraplan
			m.ultraPlan.coordinator.Cancel()
			m.infoMessage = "Ultraplan cancelled"
			return true, m, nil
		}
	}

	switch msg.String() {
	case "v":
		// Toggle plan view (only when plan is available)
		if session.Plan != nil {
			m.ultraPlan.showPlanView = !m.ultraPlan.showPlanView
		}
		return true, m, nil

	case "p":
		// Parse plan from file or coordinator output (only during planning phase)
		if session.Phase == orchestrator.PhasePlanning {
			if session.CoordinatorID != "" {
				inst := m.orchestrator.GetInstance(session.CoordinatorID)
				if inst != nil {
					plan, err := m.tryParsePlan(inst, session)
					if err != nil {
						m.errorMessage = fmt.Sprintf("Failed to parse plan: %v", err)
					} else {
						if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
							m.errorMessage = fmt.Sprintf("Invalid plan: %v", err)
						} else {
							m.infoMessage = fmt.Sprintf("Plan parsed: %d tasks in %d groups", len(plan.Tasks), len(plan.ExecutionOrder))
						}
					}
				}
			}
		}
		return true, m, nil

	case "e":
		// Start execution (only during refresh phase when plan is ready)
		if session.Phase == orchestrator.PhaseRefresh && session.Plan != nil {
			// Validate plan before starting execution
			validation := orchestrator.ValidatePlanForEditor(session.Plan)
			if validation.HasErrors() {
				m.errorMessage = fmt.Sprintf("Cannot execute: plan has %d validation error(s). Press [E] to review.", validation.ErrorCount)
				return true, m, nil
			}
			if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start execution: %v", err)
			} else {
				m.infoMessage = "Execution started"
			}
		}
		return true, m, nil

	case "E":
		// Enter plan editor (only during refresh phase when plan is ready)
		if session.Phase == orchestrator.PhaseRefresh && session.Plan != nil {
			m.enterPlanEditor()
			m.infoMessage = "Plan editor opened. Use [enter] to confirm or [esc] to cancel."
		}
		return true, m, nil

	case "c":
		// Cancel execution (only during executing phase)
		if session.Phase == orchestrator.PhaseExecuting {
			m.ultraPlan.coordinator.Cancel()
			m.infoMessage = "Execution cancelled"
		}
		return true, m, nil

	case "tab", "l":
		// Navigate to next navigable instance across all phases
		if m.navigateToNextInstance(1) {
			m.infoMessage = ""
		}
		return true, m, nil

	case "shift+tab", "h":
		// Navigate to previous navigable instance across all phases
		if m.navigateToNextInstance(-1) {
			m.infoMessage = ""
		}
		return true, m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		// Jump to task by number during execution (only if task has instance)
		if session.Phase == orchestrator.PhaseExecuting && session.Plan != nil {
			idx := int(msg.String()[0] - '1')
			if idx < len(session.Plan.Tasks) {
				task := &session.Plan.Tasks[idx]
				if _, hasInstance := session.TaskToInstance[task.ID]; hasInstance {
					m.ultraPlan.selectedTaskIdx = idx
					m.selectTaskInstance(session)
				} else {
					m.infoMessage = fmt.Sprintf("Task %d not yet started (blocked)", idx+1)
				}
			}
		}
		return true, m, nil

	case "o":
		// Open first PR URL in browser (when PRs have been created)
		if len(session.PRUrls) > 0 {
			prURL := session.PRUrls[0]
			if err := openURL(prURL); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to open PR: %v", err)
			} else {
				m.infoMessage = fmt.Sprintf("Opened PR in browser")
			}
		}
		return true, m, nil

	case "r":
		// Resume paused consolidation
		if session.Phase == orchestrator.PhaseConsolidating &&
			session.Consolidation != nil &&
			session.Consolidation.Phase == orchestrator.ConsolidationPaused {
			// TODO: Implement resume functionality when coordinator exposes it
			m.infoMessage = "Resuming consolidation..."
		}
		return true, m, nil

	case "s":
		// Signal synthesis is done, proceed to consolidation
		if session.Phase == orchestrator.PhaseSynthesis {
			if err := m.ultraPlan.coordinator.TriggerConsolidation(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to proceed: %v", err)
			} else {
				m.infoMessage = "Proceeding to consolidation..."
			}
		}
		return true, m, nil
	}

	return false, m, nil
}

// findNextRunnableTask finds the next or previous task that has a running instance.
// direction: +1 for next, -1 for previous
// Returns the task index or -1 if no runnable task is found.
func (m *Model) findNextRunnableTask(session *orchestrator.UltraPlanSession, direction int) int {
	if session.Plan == nil || len(session.Plan.Tasks) == 0 {
		return -1
	}

	numTasks := len(session.Plan.Tasks)
	startIdx := m.ultraPlan.selectedTaskIdx

	// Search through all tasks in the given direction
	for i := 1; i <= numTasks; i++ {
		// Calculate next index with wrapping
		nextIdx := (startIdx + i*direction + numTasks) % numTasks

		task := &session.Plan.Tasks[nextIdx]
		if _, hasInstance := session.TaskToInstance[task.ID]; hasInstance {
			return nextIdx
		}
	}

	return -1
}

// selectTaskInstance switches to the instance associated with the currently selected task
func (m *Model) selectTaskInstance(session *orchestrator.UltraPlanSession) {
	if session.Plan == nil || m.ultraPlan.selectedTaskIdx >= len(session.Plan.Tasks) {
		return
	}

	task := &session.Plan.Tasks[m.ultraPlan.selectedTaskIdx]
	instanceID, ok := session.TaskToInstance[task.ID]
	if !ok {
		m.infoMessage = fmt.Sprintf("Task %s not yet started", task.Title)
		return
	}

	// Find the instance index in session.Instances
	for i, inst := range m.session.Instances {
		if inst.ID == instanceID {
			m.activeTab = i
			m.ensureActiveVisible()
			m.infoMessage = fmt.Sprintf("Viewing: %s", task.Title)
			return
		}
	}

	m.infoMessage = fmt.Sprintf("Instance for task %s not found", task.Title)
}

// handleUltraPlanCoordinatorCompletion handles auto-parsing when the planning coordinator completes
// Returns true if this was an ultra-plan coordinator completion that was handled
func (m *Model) handleUltraPlanCoordinatorCompletion(inst *orchestrator.Instance) bool {
	// Not in ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return false
	}

	// Only auto-parse during planning phase
	if session.Phase != orchestrator.PhasePlanning {
		return false
	}

	// Check if this is the coordinator instance
	if session.CoordinatorID != inst.ID {
		return false
	}

	// Try to parse the plan (file-based first, then output fallback)
	plan, err := m.tryParsePlan(inst, session)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Planning completed but failed to parse plan: %v", err)
		return true
	}

	if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
		m.errorMessage = fmt.Sprintf("Planning completed but plan is invalid: %v", err)
		return true
	}

	// Auto-start execution if configured
	if session.Config.AutoApprove {
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("Plan ready but failed to auto-start: %v", err)
		} else {
			m.infoMessage = fmt.Sprintf("Plan ready: %d tasks in %d groups. Auto-starting execution...",
				len(plan.Tasks), len(plan.ExecutionOrder))
		}
	} else {
		// Enter plan editor for interactive review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("Plan ready: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
			len(plan.Tasks), len(plan.ExecutionOrder))
		// Notify user that input is needed
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRefresh
	}

	return true
}

// tryParsePlan attempts to parse a plan, trying file-based parsing first, then output parsing
func (m *Model) tryParsePlan(inst *orchestrator.Instance, session *orchestrator.UltraPlanSession) (*orchestrator.PlanSpec, error) {
	// Try file-based parsing first (preferred)
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	if _, err := os.Stat(planPath); err == nil {
		plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
		if err == nil {
			return plan, nil
		}
		// File exists but parsing failed - report this specific error
		return nil, fmt.Errorf("plan file exists but is invalid: %w", err)
	}

	// Fall back to output parsing (for backwards compatibility)
	output := m.outputs[inst.ID]
	if output == "" {
		return nil, fmt.Errorf("no plan file found and no output available")
	}

	return orchestrator.ParsePlanFromOutput(output, session.Objective)
}

// checkForPlanFile checks if the plan file exists and parses it (called during tick updates)
// Returns true if a plan was found and successfully parsed
func (m *Model) checkForPlanFile() bool {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return false
	}

	session := m.ultraPlan.coordinator.Session()
	if session == nil || session.Phase != orchestrator.PhasePlanning || session.Plan != nil {
		return false
	}

	// Get the coordinator instance
	inst := m.orchestrator.GetInstance(session.CoordinatorID)
	if inst == nil {
		return false
	}

	// Check if plan file exists
	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	if _, err := os.Stat(planPath); err != nil {
		return false
	}

	// Parse the plan
	plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
	if err != nil {
		// Don't show error yet - file might be partially written
		return false
	}

	if err := m.ultraPlan.coordinator.SetPlan(plan); err != nil {
		return false
	}

	// Plan detected - stop the coordinator instance (it's done its job)
	_ = m.orchestrator.StopInstance(inst)

	// Auto-start execution if configured
	if session.Config.AutoApprove {
		if err := m.ultraPlan.coordinator.StartExecution(); err != nil {
			m.errorMessage = fmt.Sprintf("Plan detected but failed to auto-start: %v", err)
		} else {
			m.infoMessage = fmt.Sprintf("Plan detected: %d tasks in %d groups. Auto-starting execution...",
				len(plan.Tasks), len(plan.ExecutionOrder))
		}
	} else {
		// Enter plan editor for interactive review
		m.enterPlanEditor()
		m.infoMessage = fmt.Sprintf("Plan detected: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
			len(plan.Tasks), len(plan.ExecutionOrder))
		// Notify user that input is needed
		m.ultraPlan.needsNotification = true
		m.ultraPlan.lastNotifiedPhase = orchestrator.PhaseRefresh
	}

	return true
}

// renderPlanningSidebar renders a planning-specific sidebar during the planning phase
func (m Model) renderPlanningSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Planning Phase"))
	b.WriteString("\n\n")

	// Show coordinator status
	if session.CoordinatorID != "" {
		inst := m.orchestrator.GetInstance(session.CoordinatorID)
		if inst != nil {
			// Status indicator
			var statusIcon string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusIcon = "⟳"
			case orchestrator.StatusCompleted:
				statusIcon = "✓"
			case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
				statusIcon = "✗"
			case orchestrator.StatusPending:
				statusIcon = "○"
			default:
				statusIcon = "◌"
			}

			// Coordinator line
			coordLine := fmt.Sprintf("%s Coordinator", statusIcon)
			if m.activeTab == 0 { // Coordinator should be the first instance
				b.WriteString(styles.SidebarItemActive.Render(coordLine))
			} else {
				b.WriteString(styles.Muted.Render(coordLine))
			}
			b.WriteString("\n")

			// Status description
			var statusDesc string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusDesc = "Analyzing codebase..."
			case orchestrator.StatusCompleted:
				statusDesc = "Planning complete"
			case orchestrator.StatusError:
				statusDesc = "Planning failed"
			case orchestrator.StatusStuck:
				statusDesc = "Stuck - no activity"
			case orchestrator.StatusTimeout:
				statusDesc = "Timed out"
			case orchestrator.StatusPending:
				statusDesc = "Starting..."
			default:
				statusDesc = fmt.Sprintf("Status: %s", inst.Status)
			}
			b.WriteString(styles.Muted.Render("  " + statusDesc))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(styles.Muted.Render("Initializing..."))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Instructions
	b.WriteString(styles.Muted.Render("Claude is exploring the"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("codebase and creating"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("an execution plan."))

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderSynthesisSidebar renders a synthesis-specific sidebar during the synthesis phase
func (m Model) renderSynthesisSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Synthesis Phase"))
	b.WriteString("\n\n")

	// Show synthesis instance status
	if session.SynthesisID != "" {
		inst := m.orchestrator.GetInstance(session.SynthesisID)
		if inst != nil {
			// Status indicator
			var statusIcon string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusIcon = "⟳"
			case orchestrator.StatusCompleted:
				statusIcon = "✓"
			case orchestrator.StatusError, orchestrator.StatusStuck, orchestrator.StatusTimeout:
				statusIcon = "✗"
			case orchestrator.StatusPending:
				statusIcon = "○"
			default:
				statusIcon = "◌"
			}

			// Synthesis reviewer line - highlight if active
			reviewerLine := fmt.Sprintf("%s Reviewer", statusIcon)
			// Find index of synthesis instance to check if it's active
			isActive := false
			for i, sessionInst := range m.session.Instances {
				if sessionInst.ID == session.SynthesisID && m.activeTab == i {
					isActive = true
					break
				}
			}
			if isActive {
				b.WriteString(styles.SidebarItemActive.Render(reviewerLine))
			} else {
				b.WriteString(styles.Muted.Render(reviewerLine))
			}
			b.WriteString("\n")

			// Status description
			var statusDesc string
			switch inst.Status {
			case orchestrator.StatusWorking:
				statusDesc = "Reviewing completed work..."
			case orchestrator.StatusCompleted:
				statusDesc = "Review complete"
			case orchestrator.StatusWaitingInput:
				statusDesc = "Review complete"
			case orchestrator.StatusError:
				statusDesc = "Review failed"
			case orchestrator.StatusStuck:
				statusDesc = "Stuck - no activity"
			case orchestrator.StatusTimeout:
				statusDesc = "Timed out"
			case orchestrator.StatusPending:
				statusDesc = "Starting..."
			default:
				statusDesc = fmt.Sprintf("Status: %s", inst.Status)
			}
			b.WriteString(styles.Muted.Render("  " + statusDesc))
			b.WriteString("\n")
		}
	} else {
		b.WriteString(styles.Muted.Render("Initializing..."))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Completed tasks summary
	completedCount := len(session.CompletedTasks)
	totalTasks := 0
	if session.Plan != nil {
		totalTasks = len(session.Plan.Tasks)
	}
	b.WriteString(styles.Muted.Render(fmt.Sprintf("Tasks: %d/%d completed", completedCount, totalTasks)))
	b.WriteString("\n\n")

	// Instructions
	b.WriteString(styles.Muted.Render("Claude is reviewing all"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("completed work to ensure"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("the objective is met."))
	b.WriteString("\n\n")

	// Show what comes next
	if session.Config.ConsolidationMode != "" {
		b.WriteString(styles.Muted.Render("Next: Branch consolidation"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("      & PR creation"))
	}

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderConsolidationSidebar renders the sidebar during the consolidation phase
func (m Model) renderConsolidationSidebar(width int, height int, session *orchestrator.UltraPlanSession) string {
	var b strings.Builder
	state := session.Consolidation

	// Title with phase indicator
	b.WriteString(styles.SidebarTitle.Render("Consolidation"))
	b.WriteString("\n\n")

	// Phase status
	phaseIcon := consolidationPhaseIcon(state.Phase)
	phaseDesc := consolidationPhaseDesc(state.Phase)
	statusLine := fmt.Sprintf("%s %s", phaseIcon, phaseDesc)
	b.WriteString(statusLine)
	b.WriteString("\n\n")

	// Progress: Groups
	if state.TotalGroups > 0 {
		groupProgress := fmt.Sprintf("Groups: %d/%d", state.CurrentGroup, state.TotalGroups)
		b.WriteString(styles.Muted.Render(groupProgress))
		b.WriteString("\n")

		// Show group branches created
		for i, branch := range state.GroupBranches {
			prefix := "  ✓ "
			if i == state.CurrentGroup-1 && state.Phase != orchestrator.ConsolidationComplete {
				prefix = "  ⟳ "
			}
			// Truncate branch name to fit sidebar
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
			// Extract PR number from URL (e.g., ".../pull/123")
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

// consolidationPhaseIcon returns an icon for the consolidation phase
func consolidationPhaseIcon(phase orchestrator.ConsolidationPhase) string {
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

// consolidationPhaseDesc returns a human-readable description for the consolidation phase
func consolidationPhaseDesc(phase orchestrator.ConsolidationPhase) string {
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

// openURL opens the given URL in the default browser
func openURL(url string) error {
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

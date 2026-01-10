package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Navigation-related methods for ultra-plan mode.
// This file contains:
// - Instance navigation (getNavigableInstances, updateNavigableInstances, navigateToNextInstance)
// - Phase-aware selection (selectInstanceByID, selectTaskInstance, findNextRunnableTask)
// - Navigation-aware rendering (renderPhaseInstanceLine, renderExecutionTaskLine, renderGroupConsolidatorLine)

// getNavigableInstances returns an ordered list of instance IDs that can be navigated to.
// Only includes instances from phases that have started or completed.
// Order: Planning → Plan Selection → Execution tasks (in order) → Synthesis → Revision → Consolidation
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

	// Plan Selection (multi-pass) - plan coordinators and plan manager
	for _, coordID := range session.PlanCoordinatorIDs {
		if coordID != "" {
			inst := m.orchestrator.GetInstance(coordID)
			if inst != nil && inst.Status != orchestrator.StatusPending {
				instances = append(instances, coordID)
			}
		}
	}
	if session.PlanManagerID != "" {
		inst := m.orchestrator.GetInstance(session.PlanManagerID)
		if inst != nil && inst.Status != orchestrator.StatusPending {
			instances = append(instances, session.PlanManagerID)
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

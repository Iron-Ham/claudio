package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// planEditorView is the cached view instance for plan editor rendering
var planEditorView = view.NewPlanEditorView()

// buildPlanEditorViewState converts the TUI model state to the view.PlanEditorState format.
func (m Model) buildPlanEditorViewState() *view.PlanEditorState {
	if m.planEditor == nil {
		return nil
	}
	return &view.PlanEditorState{
		Active:                 m.planEditor.active,
		SelectedTaskIdx:        m.planEditor.selectedTaskIdx,
		EditingField:           m.planEditor.editingField,
		EditBuffer:             m.planEditor.editBuffer,
		EditCursor:             m.planEditor.editCursor,
		ScrollOffset:           m.planEditor.scrollOffset,
		Validation:             m.planEditor.validation,
		ShowValidationPanel:    m.planEditor.showValidationPanel,
		ValidationScrollOffset: m.planEditor.validationScrollOffset,
		TasksInCycle:           m.planEditor.tasksInCycle,
		CanConfirm:             m.canConfirmPlan(),
	}
}

// renderPlanEditorView renders the plan editor view with validation.
// Delegates to the view package for the actual rendering.
func (m Model) renderPlanEditorView(width int) string {
	plan := m.getPlanForEditor()
	if plan == nil {
		return "No plan available"
	}

	return planEditorView.Render(view.PlanEditorRenderParams{
		Plan:                           plan,
		State:                          m.buildPlanEditorViewState(),
		Width:                          width,
		Height:                         m.terminalManager.Height(),
		SelectedTaskValidationMessages: m.getValidationMessagesForSelectedTask(),
	})
}

// renderPlanEditorHelp renders the help bar for plan editor mode.
// Delegates to the view package for the actual rendering.
func (m Model) renderPlanEditorHelp() string {
	return planEditorView.RenderHelp(m.buildPlanEditorViewState(), m.terminalManager.Width())
}

// handlePlanEditorKeypress handles keyboard input for the plan editor mode.
// Returns (handled, model, cmd) where handled indicates if the key was processed.
func (m Model) handlePlanEditorKeypress(msg tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	if m.planEditor == nil || !m.planEditor.active {
		return false, m, nil
	}

	// Get the current plan from ultra-plan session
	plan := m.getPlanForEditor()
	if plan == nil {
		return false, m, nil
	}

	// Route to edit mode handling if currently editing a field
	if m.planEditor.editingField != "" {
		return m.handlePlanEditorEditMode(msg, plan)
	}

	// Navigation and command mode handling
	return m.handlePlanEditorNavigationMode(msg, plan)
}

// handlePlanEditorNavigationMode handles keypresses when not editing a field
func (m Model) handlePlanEditorNavigationMode(msg tea.KeyMsg, plan *orchestrator.PlanSpec) (bool, tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys (work in all contexts)
	switch key {
	case "q", "esc", "escape":
		// Exit plan editor without changes
		m.exitPlanEditor()
		m.infoMessage = "Plan editor closed"
		return true, m, nil

	case "s":
		// Save current edits to plan file
		if err := m.savePlanToFile(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to save plan: %v", err)
		} else {
			m.infoMessage = "Plan saved"
		}
		return true, m, nil

	case "e":
		// Confirm plan and start execution (only in review phase)
		if m.canStartExecution() {
			// Check validation before starting
			if !m.canConfirmPlan() {
				m.errorMessage = "Cannot start execution: fix validation errors first"
				return true, m, nil
			}
			if err := m.startPlanExecution(); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to start execution: %v", err)
			} else {
				m.exitPlanEditor()
				m.infoMessage = "Execution started"
			}
		}
		return true, m, nil

	case "v":
		// Toggle validation panel visibility
		m.planEditor.showValidationPanel = !m.planEditor.showValidationPanel
		return true, m, nil

	case "r":
		// Refresh validation
		m.updatePlanValidation()
		m.infoMessage = "Validation refreshed"
		return true, m, nil

	case "pgup":
		// Scroll validation panel up
		m.scrollValidationPanel(-1)
		return true, m, nil

	case "pgdown":
		// Scroll validation panel down
		m.scrollValidationPanel(1)
		return true, m, nil
	}

	// Navigation keys
	switch key {
	case "j", "down":
		// Move to next task
		m.planEditorMoveSelection(1, plan)
		return true, m, nil

	case "k", "up":
		// Move to previous task
		m.planEditorMoveSelection(-1, plan)
		return true, m, nil

	case "g":
		// Jump to first task
		m.planEditor.selectedTaskIdx = 0
		m.planEditorEnsureVisible(plan)
		return true, m, nil

	case "G":
		// Jump to last task
		if len(plan.Tasks) > 0 {
			m.planEditor.selectedTaskIdx = len(plan.Tasks) - 1
			m.planEditorEnsureVisible(plan)
		}
		return true, m, nil

	case "tab":
		// Cycle through tasks (wrapping)
		if len(plan.Tasks) > 0 {
			m.planEditor.selectedTaskIdx = (m.planEditor.selectedTaskIdx + 1) % len(plan.Tasks)
			m.planEditorEnsureVisible(plan)
		}
		return true, m, nil

	case "shift+tab":
		// Cycle backwards through tasks (wrapping)
		if len(plan.Tasks) > 0 {
			m.planEditor.selectedTaskIdx = (m.planEditor.selectedTaskIdx - 1 + len(plan.Tasks)) % len(plan.Tasks)
			m.planEditorEnsureVisible(plan)
		}
		return true, m, nil
	}

	// Task operation keys
	switch key {
	case "enter", "t":
		// Confirm plan if valid, otherwise edit task title
		if key == "enter" && m.canConfirmPlan() {
			// Handle inline mode separately
			if m.planEditor.inlineMode && m.inlinePlan != nil {
				if err := m.confirmInlinePlanAndExecute(); err != nil {
					m.errorMessage = fmt.Sprintf("Failed to start execution: %v", err)
				}
				return true, m, nil
			}

			// Ultra-plan mode
			m.exitPlanEditor()
			m.infoMessage = "Plan confirmed"
			// Trigger execution if in refresh phase
			if m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
				session := m.ultraPlan.Coordinator.Session()
				if session != nil && session.Phase == orchestrator.PhaseRefresh {
					if err := m.ultraPlan.Coordinator.StartExecution(); err != nil {
						m.errorMessage = fmt.Sprintf("Failed to start execution: %v", err)
					} else {
						m.infoMessage = "Plan confirmed. Execution started."
						// Log plan approval
						if m.logger != nil {
							m.logger.Info("user approved plan", "task_count", len(plan.Tasks))
						}
					}
				}
			}
			return true, m, nil
		} else if key == "enter" && !m.canConfirmPlan() {
			m.errorMessage = "Cannot confirm plan: fix validation errors first"
			return true, m, nil
		}
		// 't' key or enter when editing - edit task title
		m.startEditingField("title", plan)
		return true, m, nil

	case "d":
		// Edit task description
		m.startEditingField("description", plan)
		return true, m, nil

	case "f":
		// Edit files list (comma-separated)
		m.startEditingField("files", plan)
		return true, m, nil

	case "p":
		// Edit priority (number input)
		m.startEditingField("priority", plan)
		return true, m, nil

	case "c":
		// Cycle complexity (low -> medium -> high -> low)
		m.cycleTaskComplexity(plan)
		return true, m, nil

	case "x":
		// Edit dependencies (comma-separated task IDs)
		m.startEditingField("depends_on", plan)
		return true, m, nil

	case "D":
		// Delete task (with confirmation) - for now, delete directly
		// TODO: Add confirmation dialog in future
		if err := m.deleteSelectedTask(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to delete task: %v", err)
		} else {
			m.infoMessage = "Task deleted"
			// Refresh validation after deletion
			m.updatePlanValidation()
		}
		return true, m, nil

	case "n":
		// Add new task after current
		if err := m.addNewTaskAfterCurrent(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to add task: %v", err)
		} else {
			m.infoMessage = "New task added"
			// Start editing the title of the new task
			m.startEditingField("title", plan)
			// Refresh validation after addition
			m.updatePlanValidation()
		}
		return true, m, nil

	case "J":
		// Move task down (swap with next)
		if err := m.moveTaskDown(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to move task: %v", err)
		}
		return true, m, nil

	case "K":
		// Move task up (swap with previous)
		if err := m.moveTaskUp(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to move task: %v", err)
		}
		return true, m, nil
	}

	return false, m, nil
}

// handlePlanEditorEditMode handles keypresses when editing a field
func (m Model) handlePlanEditorEditMode(msg tea.KeyMsg, plan *orchestrator.PlanSpec) (bool, tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc", "escape":
		// Cancel edit and restore original value
		m.cancelFieldEdit()
		return true, m, nil

	case "enter":
		// Confirm edit and save
		if err := m.confirmFieldEdit(plan); err != nil {
			m.errorMessage = fmt.Sprintf("Failed to save: %v", err)
		} else {
			// Refresh validation after edit
			m.updatePlanValidation()
		}
		return true, m, nil

	case "left":
		// Move cursor left
		m.planEditorMoveCursor(-1)
		return true, m, nil

	case "right":
		// Move cursor right
		m.planEditorMoveCursor(1)
		return true, m, nil

	case "home", "ctrl+a":
		// Move to start of line/buffer
		m.planEditor.editCursor = 0
		return true, m, nil

	case "end", "ctrl+e":
		// Move to end of line/buffer
		m.planEditor.editCursor = len([]rune(m.planEditor.editBuffer))
		return true, m, nil

	case "backspace":
		// Delete character before cursor
		m.planEditorDeleteBack(1)
		return true, m, nil

	case "delete", "ctrl+d":
		// Delete character at cursor
		m.planEditorDeleteForward(1)
		return true, m, nil

	case "ctrl+k":
		// Delete from cursor to end of line
		runes := []rune(m.planEditor.editBuffer)
		m.planEditor.editBuffer = string(runes[:m.planEditor.editCursor])
		return true, m, nil

	case "ctrl+u":
		// Delete from start of line to cursor
		runes := []rune(m.planEditor.editBuffer)
		m.planEditor.editBuffer = string(runes[m.planEditor.editCursor:])
		m.planEditor.editCursor = 0
		return true, m, nil

	case "ctrl+w":
		// Delete word before cursor
		m.planEditorDeleteWord()
		return true, m, nil

	default:
		// Handle regular character input
		if len(key) == 1 || msg.Type == tea.KeyRunes {
			// Insert character(s) at cursor position
			runes := []rune(m.planEditor.editBuffer)
			insertRunes := msg.Runes
			if len(insertRunes) == 0 && len(key) == 1 {
				insertRunes = []rune(key)
			}
			newBuffer := string(runes[:m.planEditor.editCursor]) + string(insertRunes) + string(runes[m.planEditor.editCursor:])
			m.planEditor.editBuffer = newBuffer
			m.planEditor.editCursor += len(insertRunes)
			return true, m, nil
		}
	}

	return true, m, nil // Consume all keys in edit mode
}

// getPlanForEditor returns the current plan from the appropriate source.
// It checks inline plan mode first, then ultra-plan mode.
func (m Model) getPlanForEditor() *orchestrator.PlanSpec {
	// Check inline plan mode first
	if m.planEditor != nil && m.planEditor.inlineMode && m.inlinePlan != nil {
		session := m.inlinePlan.GetCurrentSession()
		if session != nil {
			return session.Plan
		}
	}

	// Fall back to ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return nil
	}
	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return nil
	}
	return session.Plan
}

// planEditorMoveSelection moves the selection by delta positions
func (m *Model) planEditorMoveSelection(delta int, plan *orchestrator.PlanSpec) {
	if len(plan.Tasks) == 0 {
		return
	}

	newIdx := m.planEditor.selectedTaskIdx + delta
	newIdx = max(0, newIdx)
	newIdx = min(newIdx, len(plan.Tasks)-1)
	m.planEditor.selectedTaskIdx = newIdx
	m.planEditorEnsureVisible(plan)
}

// planEditorEnsureVisible adjusts scroll offset to keep selected task visible
func (m *Model) planEditorEnsureVisible(plan *orchestrator.PlanSpec) {
	// Calculate visible area (assume ~5 lines per task in compact view)
	maxVisible := max(3, (m.terminalManager.Height()-10)/5)

	if m.planEditor.selectedTaskIdx < m.planEditor.scrollOffset {
		m.planEditor.scrollOffset = m.planEditor.selectedTaskIdx
	} else if m.planEditor.selectedTaskIdx >= m.planEditor.scrollOffset+maxVisible {
		m.planEditor.scrollOffset = m.planEditor.selectedTaskIdx - maxVisible + 1
	}

	// Clamp scroll offset
	m.planEditor.scrollOffset = max(0, m.planEditor.scrollOffset)
	maxOffset := max(0, len(plan.Tasks)-maxVisible)
	m.planEditor.scrollOffset = min(m.planEditor.scrollOffset, maxOffset)
}

// startEditingField initializes edit mode for a specific field
func (m *Model) startEditingField(field string, plan *orchestrator.PlanSpec) {
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) {
		return
	}

	task := &plan.Tasks[m.planEditor.selectedTaskIdx]
	m.planEditor.editingField = field

	// Initialize edit buffer with current value
	switch field {
	case "title":
		m.planEditor.editBuffer = task.Title
	case "description":
		m.planEditor.editBuffer = task.Description
	case "files":
		m.planEditor.editBuffer = strings.Join(task.Files, ", ")
	case "priority":
		m.planEditor.editBuffer = strconv.Itoa(task.Priority)
	case "depends_on":
		m.planEditor.editBuffer = strings.Join(task.DependsOn, ", ")
	default:
		m.planEditor.editingField = ""
		return
	}

	// Position cursor at end
	m.planEditor.editCursor = len([]rune(m.planEditor.editBuffer))
}

// cancelFieldEdit exits edit mode without saving
func (m *Model) cancelFieldEdit() {
	m.planEditor.editingField = ""
	m.planEditor.editBuffer = ""
	m.planEditor.editCursor = 0
}

// confirmFieldEdit saves the edited value and exits edit mode
func (m *Model) confirmFieldEdit(plan *orchestrator.PlanSpec) error {
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) {
		m.cancelFieldEdit()
		return nil
	}

	taskID := plan.Tasks[m.planEditor.selectedTaskIdx].ID
	value := m.planEditor.editBuffer
	editedField := m.planEditor.editingField

	var err error
	switch editedField {
	case "title":
		err = orchestrator.UpdateTaskTitle(plan, taskID, value)

	case "description":
		err = orchestrator.UpdateTaskDescription(plan, taskID, value)

	case "files":
		files := parseCommaSeparatedList(value)
		err = orchestrator.UpdateTaskFiles(plan, taskID, files)

	case "priority":
		priority, parseErr := strconv.Atoi(strings.TrimSpace(value))
		if parseErr != nil {
			err = fmt.Errorf("invalid priority value: %s", value)
		} else {
			err = orchestrator.UpdateTaskPriority(plan, taskID, priority)
		}

	case "depends_on":
		deps := parseCommaSeparatedList(value)
		err = orchestrator.UpdateTaskDependencies(plan, taskID, deps)
	}

	// Log the edit if successful
	if err == nil && m.logger != nil {
		m.logger.Info("user edited plan", "changes_made", editedField)
	}

	m.cancelFieldEdit()
	return err
}

// cycleTaskComplexity cycles through low -> medium -> high -> low
func (m *Model) cycleTaskComplexity(plan *orchestrator.PlanSpec) {
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) {
		return
	}

	task := &plan.Tasks[m.planEditor.selectedTaskIdx]
	var nextComplexity orchestrator.TaskComplexity

	switch task.EstComplexity {
	case orchestrator.ComplexityLow:
		nextComplexity = orchestrator.ComplexityMedium
	case orchestrator.ComplexityMedium:
		nextComplexity = orchestrator.ComplexityHigh
	case orchestrator.ComplexityHigh:
		nextComplexity = orchestrator.ComplexityLow
	default:
		nextComplexity = orchestrator.ComplexityLow
	}

	_ = orchestrator.UpdateTaskComplexity(plan, task.ID, nextComplexity)

	// Log complexity change
	if m.logger != nil {
		m.logger.Info("user edited plan", "changes_made", "complexity")
	}
}

// deleteSelectedTask removes the currently selected task
func (m *Model) deleteSelectedTask(plan *orchestrator.PlanSpec) error {
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) {
		return fmt.Errorf("no task selected")
	}

	taskID := plan.Tasks[m.planEditor.selectedTaskIdx].ID
	err := orchestrator.DeleteTask(plan, taskID)
	if err != nil {
		return err
	}

	// Log the deletion
	if m.logger != nil {
		m.logger.Info("user edited plan", "changes_made", "task_deleted")
	}

	// Adjust selection if needed
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) && len(plan.Tasks) > 0 {
		m.planEditor.selectedTaskIdx = len(plan.Tasks) - 1
	}
	if len(plan.Tasks) == 0 {
		m.planEditor.selectedTaskIdx = 0
	}

	return nil
}

// addNewTaskAfterCurrent adds a new task after the currently selected one
func (m *Model) addNewTaskAfterCurrent(plan *orchestrator.PlanSpec) error {
	// Generate a new task ID
	newID := fmt.Sprintf("task-%d", len(plan.Tasks)+1)

	// Check for ID collision and find unique ID
	for orchestrator.GetTaskByID(plan, newID) != nil {
		newID = fmt.Sprintf("task-%d-%d", len(plan.Tasks)+1, len(newID))
	}

	newTask := orchestrator.PlannedTask{
		ID:            newID,
		Title:         "New Task",
		Description:   "",
		Files:         nil,
		DependsOn:     nil,
		Priority:      0,
		EstComplexity: orchestrator.ComplexityMedium,
	}

	var afterTaskID string
	if m.planEditor.selectedTaskIdx < len(plan.Tasks) {
		afterTaskID = plan.Tasks[m.planEditor.selectedTaskIdx].ID
	}

	err := orchestrator.AddTask(plan, afterTaskID, newTask)
	if err != nil {
		return err
	}

	// Log the task addition
	if m.logger != nil {
		m.logger.Info("user edited plan", "changes_made", "task_added")
	}

	// Move selection to new task
	m.planEditor.selectedTaskIdx++
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) {
		m.planEditor.selectedTaskIdx = len(plan.Tasks) - 1
	}

	return nil
}

// moveTaskUp swaps the selected task with the previous one
func (m *Model) moveTaskUp(plan *orchestrator.PlanSpec) error {
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks) || m.planEditor.selectedTaskIdx == 0 {
		return nil // Already at top or invalid selection
	}

	taskID := plan.Tasks[m.planEditor.selectedTaskIdx].ID
	err := orchestrator.MoveTaskUp(plan, taskID)
	if err != nil {
		return err
	}

	m.planEditor.selectedTaskIdx--
	return nil
}

// moveTaskDown swaps the selected task with the next one
func (m *Model) moveTaskDown(plan *orchestrator.PlanSpec) error {
	if m.planEditor.selectedTaskIdx >= len(plan.Tasks)-1 {
		return nil // Already at bottom or invalid selection
	}

	taskID := plan.Tasks[m.planEditor.selectedTaskIdx].ID
	err := orchestrator.MoveTaskDown(plan, taskID)
	if err != nil {
		return err
	}

	m.planEditor.selectedTaskIdx++
	return nil
}

// planEditorMoveCursor moves the edit cursor by delta positions
func (m *Model) planEditorMoveCursor(delta int) {
	runes := []rune(m.planEditor.editBuffer)
	newPos := m.planEditor.editCursor + delta
	newPos = max(0, newPos)
	newPos = min(newPos, len(runes))
	m.planEditor.editCursor = newPos
}

// planEditorDeleteBack deletes n characters before the cursor
func (m *Model) planEditorDeleteBack(n int) {
	if m.planEditor.editCursor == 0 {
		return
	}
	runes := []rune(m.planEditor.editBuffer)
	deleteCount := min(n, m.planEditor.editCursor)
	m.planEditor.editBuffer = string(runes[:m.planEditor.editCursor-deleteCount]) + string(runes[m.planEditor.editCursor:])
	m.planEditor.editCursor -= deleteCount
}

// planEditorDeleteForward deletes n characters at/after the cursor
func (m *Model) planEditorDeleteForward(n int) {
	runes := []rune(m.planEditor.editBuffer)
	if m.planEditor.editCursor >= len(runes) {
		return
	}
	deleteCount := min(n, len(runes)-m.planEditor.editCursor)
	m.planEditor.editBuffer = string(runes[:m.planEditor.editCursor]) + string(runes[m.planEditor.editCursor+deleteCount:])
}

// planEditorDeleteWord deletes the word before the cursor
func (m *Model) planEditorDeleteWord() {
	if m.planEditor.editCursor == 0 {
		return
	}
	runes := []rune(m.planEditor.editBuffer)
	pos := m.planEditor.editCursor - 1

	// Skip whitespace
	for pos > 0 && (runes[pos] == ' ' || runes[pos] == '\t') {
		pos--
	}

	// Skip word characters
	for pos > 0 && runes[pos-1] != ' ' && runes[pos-1] != '\t' {
		pos--
	}

	m.planEditor.editBuffer = string(runes[:pos]) + string(runes[m.planEditor.editCursor:])
	m.planEditor.editCursor = pos
}

// savePlanToFile saves the current plan to the plan file
func (m *Model) savePlanToFile(plan *orchestrator.PlanSpec) error {
	// Inline plan mode doesn't have a worktree yet, so save to base repo directory
	if m.planEditor != nil && m.planEditor.inlineMode {
		if m.session == nil {
			return fmt.Errorf("no session")
		}
		planPath := orchestrator.PlanFilePath(m.session.BaseRepo)
		return orchestrator.SavePlanToFile(plan, planPath)
	}

	// Ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return fmt.Errorf("no ultra-plan session")
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil {
		return fmt.Errorf("no session")
	}

	// Get coordinator instance for worktree path
	inst := m.orchestrator.GetInstance(session.CoordinatorID)
	if inst == nil {
		return fmt.Errorf("coordinator instance not found")
	}

	planPath := orchestrator.PlanFilePath(inst.WorktreePath)
	return orchestrator.SavePlanToFile(plan, planPath)
}

// canStartExecution returns true if we can start plan execution
func (m *Model) canStartExecution() bool {
	// Inline plan mode - can always start if plan exists and valid
	if m.planEditor != nil && m.planEditor.inlineMode {
		session := m.getCurrentPlanSession()
		return session != nil && session.Plan != nil
	}

	// Ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return false
	}
	session := m.ultraPlan.Coordinator.Session()
	if session == nil || session.Plan == nil {
		return false
	}
	// Only allow starting from refresh phase
	return session.Phase == orchestrator.PhaseRefresh
}

// startPlanExecution triggers execution of the plan
func (m *Model) startPlanExecution() error {
	// Inline plan mode
	if m.planEditor != nil && m.planEditor.inlineMode {
		return m.startInlinePlanExecution()
	}

	// Ultra-plan mode
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return fmt.Errorf("no ultra-plan session")
	}
	return m.ultraPlan.Coordinator.StartExecution()
}

// scrollValidationPanel scrolls the validation panel by delta lines
func (m *Model) scrollValidationPanel(delta int) {
	if m.planEditor == nil || m.planEditor.validation == nil {
		return
	}

	newOffset := m.planEditor.validationScrollOffset + delta
	if newOffset < 0 {
		newOffset = 0
	}

	maxOffset := len(m.planEditor.validation.Messages) - 5
	if maxOffset < 0 {
		maxOffset = 0
	}
	if newOffset > maxOffset {
		newOffset = maxOffset
	}

	m.planEditor.validationScrollOffset = newOffset
}

// parseCommaSeparatedList parses a comma-separated string into a slice
func parseCommaSeparatedList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

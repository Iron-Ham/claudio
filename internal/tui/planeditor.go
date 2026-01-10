package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Validation panel styles
var (
	validationPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	validationErrorStyle = lipgloss.NewStyle().
		Foreground(styles.RedColor).
		Bold(true)

	validationWarningStyle = lipgloss.NewStyle().
		Foreground(styles.YellowColor)

	validationInfoStyle = lipgloss.NewStyle().
		Foreground(styles.BlueColor)

	validationHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Underline(true)

	validationTaskStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	cyclicTaskStyle = lipgloss.NewStyle().
		Foreground(styles.RedColor).
		Bold(true)
)

// renderValidationPanel renders the validation feedback panel at the bottom of the editor
func (m Model) renderValidationPanel(width int, maxHeight int) string {
	if m.planEditor == nil || m.planEditor.validation == nil {
		return ""
	}

	validation := m.planEditor.validation
	if len(validation.Messages) == 0 {
		// No validation messages - show a success indicator
		successMsg := lipgloss.NewStyle().
			Foreground(styles.GreenColor).
			Render("✓ Plan is valid")
		return validationPanelStyle.Width(width - 4).Render(successMsg)
	}

	var b strings.Builder

	// Header with counts
	headerParts := []string{}
	if validation.ErrorCount > 0 {
		headerParts = append(headerParts, validationErrorStyle.Render(fmt.Sprintf("%d error(s)", validation.ErrorCount)))
	}
	if validation.WarningCount > 0 {
		headerParts = append(headerParts, validationWarningStyle.Render(fmt.Sprintf("%d warning(s)", validation.WarningCount)))
	}
	if validation.InfoCount > 0 {
		headerParts = append(headerParts, validationInfoStyle.Render(fmt.Sprintf("%d info", validation.InfoCount)))
	}

	header := validationHeaderStyle.Render("Validation") + "  " + strings.Join(headerParts, " | ")
	b.WriteString(header)
	b.WriteString("\n")

	// Calculate available lines for messages
	availableLines := maxHeight - 3 // Header + borders
	if availableLines < 1 {
		availableLines = 1
	}

	// Render messages with scroll offset
	messages := validation.Messages
	startIdx := m.planEditor.validationScrollOffset
	if startIdx >= len(messages) {
		startIdx = 0
	}

	linesRendered := 0
	for i := startIdx; i < len(messages) && linesRendered < availableLines; i++ {
		msg := messages[i]
		line := m.renderValidationMessage(msg, width-6)
		b.WriteString(line)
		b.WriteString("\n")
		linesRendered++
	}

	// Show scroll indicator if there are more messages
	remaining := len(messages) - startIdx - linesRendered
	if remaining > 0 {
		scrollIndicator := styles.Muted.Render(fmt.Sprintf("  ↓ %d more...", remaining))
		b.WriteString(scrollIndicator)
	}

	return validationPanelStyle.Width(width - 4).Render(b.String())
}

// renderValidationMessage renders a single validation message
func (m Model) renderValidationMessage(msg orchestrator.ValidationMessage, maxWidth int) string {
	var icon string
	var msgStyle lipgloss.Style

	switch msg.Severity {
	case orchestrator.SeverityError:
		icon = "✗"
		msgStyle = validationErrorStyle
	case orchestrator.SeverityWarning:
		icon = "⚠"
		msgStyle = validationWarningStyle
	case orchestrator.SeverityInfo:
		icon = "ℹ"
		msgStyle = validationInfoStyle
	default:
		icon = "•"
		msgStyle = styles.Muted
	}

	// Build the message line
	var parts []string
	parts = append(parts, msgStyle.Render(icon))

	// Add task ID if present
	if msg.TaskID != "" {
		taskRef := validationTaskStyle.Render(fmt.Sprintf("[%s]", msg.TaskID))
		parts = append(parts, taskRef)
	}

	// Add the main message
	parts = append(parts, msgStyle.Render(msg.Message))

	line := strings.Join(parts, " ")

	// Truncate if needed
	if len(line) > maxWidth {
		line = line[:maxWidth-3] + "..."
	}

	return line
}

// renderValidationSummary renders a compact validation summary for the status bar
func (m Model) renderValidationSummary() string {
	if m.planEditor == nil || m.planEditor.validation == nil {
		return ""
	}

	validation := m.planEditor.validation
	if len(validation.Messages) == 0 {
		return lipgloss.NewStyle().Foreground(styles.GreenColor).Render("✓ Valid")
	}

	var parts []string
	if validation.ErrorCount > 0 {
		parts = append(parts, validationErrorStyle.Render(fmt.Sprintf("✗%d", validation.ErrorCount)))
	}
	if validation.WarningCount > 0 {
		parts = append(parts, validationWarningStyle.Render(fmt.Sprintf("⚠%d", validation.WarningCount)))
	}

	return strings.Join(parts, " ")
}

// renderTaskValidationIndicator returns a validation indicator for a specific task
func (m Model) renderTaskValidationIndicator(taskID string) string {
	if m.planEditor == nil || m.planEditor.validation == nil {
		return ""
	}

	// Check if task is in a cycle
	if m.planEditor.tasksInCycle[taskID] {
		return cyclicTaskStyle.Render("⟳")
	}

	// Get messages for this task
	messages := m.planEditor.validation.GetMessagesForTask(taskID)
	if len(messages) == 0 {
		return ""
	}

	// Count by severity
	hasError := false
	hasWarning := false
	for _, msg := range messages {
		switch msg.Severity {
		case orchestrator.SeverityError:
			hasError = true
		case orchestrator.SeverityWarning:
			hasWarning = true
		}
	}

	if hasError {
		return validationErrorStyle.Render("✗")
	}
	if hasWarning {
		return validationWarningStyle.Render("⚠")
	}
	return ""
}

// renderPlanEditorView renders the plan editor view with validation
func (m Model) renderPlanEditorView(width int) string {
	if m.ultraPlan == nil || m.ultraPlan.Coordinator == nil {
		return "No plan available"
	}

	session := m.ultraPlan.Coordinator.Session()
	if session == nil || session.Plan == nil {
		return "No plan available"
	}

	plan := session.Plan
	var b strings.Builder

	// Calculate layout heights
	totalHeight := m.height - 8 // Reserve space for header/footer
	validationPanelHeight := 0
	if m.planEditor != nil && m.planEditor.showValidationPanel && m.planEditor.validation != nil {
		if len(m.planEditor.validation.Messages) > 0 {
			validationPanelHeight = min(8, len(m.planEditor.validation.Messages)+3)
		} else {
			validationPanelHeight = 3 // Just show "valid" indicator
		}
	}
	mainContentHeight := totalHeight - validationPanelHeight

	// Plan summary header
	b.WriteString(styles.SidebarTitle.Render("Plan Editor"))
	b.WriteString("  ")
	b.WriteString(m.renderValidationSummary())
	b.WriteString("\n\n")

	// Task list with validation indicators
	b.WriteString(styles.SidebarTitle.Render("Tasks"))
	b.WriteString("\n")

	selectedIdx := 0
	if m.planEditor != nil {
		selectedIdx = m.planEditor.selectedTaskIdx
	}

	// Calculate visible task range with scroll
	scrollOffset := 0
	if m.planEditor != nil {
		scrollOffset = m.planEditor.scrollOffset
	}

	visibleTasks := mainContentHeight - 6 // Reserve space for headers
	if visibleTasks < 3 {
		visibleTasks = 3
	}

	startIdx := scrollOffset
	if startIdx > len(plan.Tasks)-visibleTasks {
		startIdx = len(plan.Tasks) - visibleTasks
	}
	if startIdx < 0 {
		startIdx = 0
	}

	endIdx := startIdx + visibleTasks
	if endIdx > len(plan.Tasks) {
		endIdx = len(plan.Tasks)
	}

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(styles.Muted.Render(fmt.Sprintf("  ↑ %d more above\n", startIdx)))
	}

	for i := startIdx; i < endIdx; i++ {
		task := &plan.Tasks[i]
		isSelected := i == selectedIdx

		// Validation indicator for this task
		validationIndicator := m.renderTaskValidationIndicator(task.ID)

		// Task status icon
		var statusIcon string
		if m.planEditor != nil && m.planEditor.tasksInCycle[task.ID] {
			statusIcon = cyclicTaskStyle.Render("⟳")
		} else {
			statusIcon = view.ComplexityIndicator(task.EstComplexity)
		}

		// Build task line
		taskNum := fmt.Sprintf("%d.", i+1)
		titleLen := width - 12 // Account for numbering, icons, and padding
		title := truncate(task.Title, titleLen)

		var line string
		if validationIndicator != "" {
			line = fmt.Sprintf("  %s %s %s %s", taskNum, statusIcon, validationIndicator, title)
		} else {
			line = fmt.Sprintf("  %s %s %s", taskNum, statusIcon, title)
		}

		// Apply selection styling
		if isSelected {
			line = lipgloss.NewStyle().
				Background(styles.PrimaryColor).
				Foreground(styles.TextColor).
				Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
	}

	// Show scroll indicator at bottom
	remaining := len(plan.Tasks) - endIdx
	if remaining > 0 {
		b.WriteString(styles.Muted.Render(fmt.Sprintf("  ↓ %d more below\n", remaining)))
	}

	// Selected task details
	if selectedIdx >= 0 && selectedIdx < len(plan.Tasks) {
		task := &plan.Tasks[selectedIdx]
		b.WriteString("\n")
		b.WriteString(styles.SidebarTitle.Render("Selected Task"))
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("ID: %s\n", task.ID))
		b.WriteString(fmt.Sprintf("Complexity: %s\n", task.EstComplexity))
		if len(task.DependsOn) > 0 {
			b.WriteString(fmt.Sprintf("Depends on: %s\n", strings.Join(task.DependsOn, ", ")))
		}
		if len(task.Files) > 0 {
			filesDisplay := strings.Join(task.Files, ", ")
			if len(filesDisplay) > width-10 {
				filesDisplay = filesDisplay[:width-13] + "..."
			}
			b.WriteString(fmt.Sprintf("Files: %s\n", filesDisplay))
		}

		// Show task-specific validation messages
		taskMessages := m.getValidationMessagesForSelectedTask()
		if len(taskMessages) > 0 {
			b.WriteString("\n")
			b.WriteString(validationWarningStyle.Render("Issues:"))
			b.WriteString("\n")
			for _, msg := range taskMessages {
				line := m.renderValidationMessage(msg, width-6)
				b.WriteString("  " + line + "\n")
			}
		}
	}

	// Render main content area
	mainContent := styles.OutputArea.Width(width - 2).Height(mainContentHeight).Render(b.String())

	// Render validation panel if enabled
	var validationPanel string
	if m.planEditor != nil && m.planEditor.showValidationPanel {
		validationPanel = m.renderValidationPanel(width, validationPanelHeight)
	}

	// Combine main content and validation panel
	if validationPanel != "" {
		return lipgloss.JoinVertical(lipgloss.Left, mainContent, validationPanel)
	}
	return mainContent
}

// renderPlanEditorHelp renders the help bar for plan editor mode
func (m Model) renderPlanEditorHelp() string {
	var keys []string

	keys = append(keys, "[↑↓] select task")
	keys = append(keys, "[e] edit")

	// Show confirm status based on validation
	if m.canConfirmPlan() {
		keys = append(keys, "[enter] confirm")
	} else {
		keys = append(keys, styles.Muted.Render("[enter] blocked"))
	}

	keys = append(keys, "[v] toggle validation")
	keys = append(keys, "[esc] exit")

	return styles.HelpBar.Width(m.width).Render(strings.Join(keys, "  "))
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
			m.exitPlanEditor()
			m.infoMessage = "Plan confirmed"
			// Trigger execution if in refresh phase
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

// getPlanForEditor returns the current plan from the ultra-plan session
func (m Model) getPlanForEditor() *orchestrator.PlanSpec {
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
	maxVisible := max(3, (m.height-10)/5)

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

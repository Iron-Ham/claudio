package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Validation panel styles (local to this file)
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

// PlanEditorState holds the rendering state for the plan editor view.
// This mirrors the state from tui.PlanEditorState but is defined here to avoid
// circular dependencies. The tui package creates and manages this state.
type PlanEditorState struct {
	// Active indicates whether the plan editor is currently shown
	Active bool

	// SelectedTaskIdx is the index of the currently selected task in the task list
	SelectedTaskIdx int

	// EditingField indicates which field is being edited (empty if not editing)
	EditingField string

	// EditBuffer holds the current edit buffer content when editing a field
	EditBuffer string

	// EditCursor is the cursor position within the edit buffer
	EditCursor int

	// ScrollOffset is the vertical scroll offset for the task list
	ScrollOffset int

	// Validation holds the current validation results for the plan
	Validation *orchestrator.ValidationResult

	// ShowValidationPanel controls whether the validation panel is visible
	ShowValidationPanel bool

	// ValidationScrollOffset is the scroll offset for the validation panel
	ValidationScrollOffset int

	// TasksInCycle contains task IDs that are part of a dependency cycle
	TasksInCycle map[string]bool

	// CanConfirm indicates whether the plan can be confirmed (no validation errors)
	CanConfirm bool
}

// PlanEditorRenderParams contains all parameters needed to render the plan editor view.
type PlanEditorRenderParams struct {
	// Plan is the plan spec being edited
	Plan *orchestrator.PlanSpec

	// State is the editor state
	State *PlanEditorState

	// Width is the available width for rendering
	Width int

	// Height is the available height for rendering
	Height int

	// ValidationMessages are messages for the currently selected task
	SelectedTaskValidationMessages []orchestrator.ValidationMessage
}

// PlanEditorView handles rendering of the plan editor interface.
// It provides methods for rendering the main view, validation panel, and help bar.
type PlanEditorView struct{}

// NewPlanEditorView creates a new PlanEditorView instance.
func NewPlanEditorView() *PlanEditorView {
	return &PlanEditorView{}
}

// Render renders the complete plan editor view.
func (v *PlanEditorView) Render(params PlanEditorRenderParams) string {
	if params.Plan == nil {
		return "No plan available"
	}

	state := params.State
	if state == nil {
		return "No editor state"
	}

	plan := params.Plan
	width := params.Width
	height := params.Height

	var b strings.Builder

	// Calculate layout heights
	totalHeight := height - 8 // Reserve space for header/footer
	validationPanelHeight := 0
	if state.ShowValidationPanel && state.Validation != nil {
		if len(state.Validation.Messages) > 0 {
			validationPanelHeight = min(8, len(state.Validation.Messages)+3)
		} else {
			validationPanelHeight = 3 // Just show "valid" indicator
		}
	}
	mainContentHeight := totalHeight - validationPanelHeight

	// Plan summary header
	b.WriteString(styles.SidebarTitle.Render("Plan Editor"))
	b.WriteString("  ")
	b.WriteString(v.renderValidationSummary(state))
	b.WriteString("\n\n")

	// Task list with validation indicators
	b.WriteString(styles.SidebarTitle.Render("Tasks"))
	b.WriteString("\n")

	selectedIdx := state.SelectedTaskIdx

	// Calculate visible task range with scroll
	scrollOffset := state.ScrollOffset

	visibleTasks := max(3, mainContentHeight-6) // Reserve space for headers

	startIdx := max(0, min(scrollOffset, len(plan.Tasks)-visibleTasks))
	endIdx := min(startIdx+visibleTasks, len(plan.Tasks))

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(styles.Muted.Render(fmt.Sprintf("  ↑ %d more above\n", startIdx)))
	}

	for i := startIdx; i < endIdx; i++ {
		task := &plan.Tasks[i]
		isSelected := i == selectedIdx

		// Validation indicator for this task
		validationIndicator := v.renderTaskValidationIndicator(task.ID, state)

		// Task status icon
		var statusIcon string
		if state.TasksInCycle[task.ID] {
			statusIcon = cyclicTaskStyle.Render("⟳")
		} else {
			statusIcon = complexityIndicator(task.EstComplexity)
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
		taskMessages := params.SelectedTaskValidationMessages
		if len(taskMessages) > 0 {
			b.WriteString("\n")
			b.WriteString(validationWarningStyle.Render("Issues:"))
			b.WriteString("\n")
			for _, msg := range taskMessages {
				line := v.renderValidationMessage(msg, width-6)
				b.WriteString("  " + line + "\n")
			}
		}
	}

	// Render main content area
	mainContent := styles.OutputArea.Width(width - 2).Height(mainContentHeight).Render(b.String())

	// Render validation panel if enabled
	var validationPanel string
	if state.ShowValidationPanel {
		validationPanel = v.renderValidationPanel(state, width, validationPanelHeight)
	}

	// Combine main content and validation panel
	if validationPanel != "" {
		return lipgloss.JoinVertical(lipgloss.Left, mainContent, validationPanel)
	}
	return mainContent
}

// RenderHelp renders the help bar for plan editor mode.
func (v *PlanEditorView) RenderHelp(state *PlanEditorState, width int) string {
	badge := styles.ModeBadgeNormal.Render("PLAN EDIT")
	var keys []string

	keys = append(keys, "[↑↓] select task")
	keys = append(keys, "[e] edit")

	// Show confirm status based on validation
	if state != nil && state.CanConfirm {
		keys = append(keys, "[enter] confirm")
	} else {
		keys = append(keys, styles.Muted.Render("[enter] blocked"))
	}

	keys = append(keys, "[v] toggle validation")
	keys = append(keys, "[esc] exit")

	return styles.HelpBar.Width(width).Render(badge + "  " + strings.Join(keys, "  "))
}

// renderValidationPanel renders the validation feedback panel at the bottom of the editor.
func (v *PlanEditorView) renderValidationPanel(state *PlanEditorState, width int, maxHeight int) string {
	if state == nil || state.Validation == nil {
		return ""
	}

	validation := state.Validation
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
	availableLines := max(1, maxHeight-3) // Header + borders

	// Render messages with scroll offset
	messages := validation.Messages
	startIdx := state.ValidationScrollOffset
	if startIdx >= len(messages) {
		startIdx = 0
	}

	linesRendered := 0
	for i := startIdx; i < len(messages) && linesRendered < availableLines; i++ {
		msg := messages[i]
		line := v.renderValidationMessage(msg, width-6)
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

// renderValidationMessage renders a single validation message.
func (v *PlanEditorView) renderValidationMessage(msg orchestrator.ValidationMessage, maxWidth int) string {
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

// renderValidationSummary renders a compact validation summary for the status bar.
func (v *PlanEditorView) renderValidationSummary(state *PlanEditorState) string {
	if state == nil || state.Validation == nil {
		return ""
	}

	validation := state.Validation
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

// renderTaskValidationIndicator returns a validation indicator for a specific task.
func (v *PlanEditorView) renderTaskValidationIndicator(taskID string, state *PlanEditorState) string {
	if state == nil || state.Validation == nil {
		return ""
	}

	// Check if task is in a cycle
	if state.TasksInCycle[taskID] {
		return cyclicTaskStyle.Render("⟳")
	}

	// Get messages for this task
	messages := state.Validation.GetMessagesForTask(taskID)
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

// Helper functions

// complexityIndicator returns a visual indicator for task complexity.
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

package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Plan editor rendering functions for the interactive plan review TUI.
// This file provides rendering logic for viewing and editing ultraplan execution plans
// before proceeding to execution.

// renderPlanEditor renders the main plan editor view
// It displays a header with task/group counts, plan summary, scrollable task list,
// and a footer with keyboard shortcuts.
func (m Model) renderPlanEditor(width, height int) string {
	if m.planEditor == nil || !m.planEditor.active {
		return ""
	}

	// Get the plan from ultra-plan session
	plan := m.getPlanForEditor()
	if plan == nil {
		return styles.Muted.Render("No plan available for editing")
	}

	var b strings.Builder

	// Calculate available heights for each section
	headerHeight := 3    // Title + stats line + blank line
	summaryHeight := 5   // Summary section with padding
	footerHeight := 3    // Instructions footer
	availableForTasks := max(height-headerHeight-summaryHeight-footerHeight, 5)

	// Render header
	b.WriteString(m.renderPlanEditorHeader(plan, width))
	b.WriteString("\n")

	// Render summary
	b.WriteString(m.renderPlanEditorSummary(plan, width))
	b.WriteString("\n")

	// Render task list
	b.WriteString(m.renderPlanEditorTaskList(plan, width, availableForTasks))
	b.WriteString("\n")

	// Render footer with keyboard shortcuts
	b.WriteString(m.renderPlanEditorFooter(width))

	return b.String()
}

// renderPlanEditorHeader renders the header showing 'Plan Review' with task/group counts
func (m Model) renderPlanEditorHeader(plan *orchestrator.PlanSpec, width int) string {
	// Title style
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.PrimaryColor)

	// Stats style
	statsStyle := lipgloss.NewStyle().
		Foreground(styles.MutedColor)

	// Build header line
	title := titleStyle.Render("Plan Review")

	// Calculate task and group counts
	taskCount := len(plan.Tasks)
	groupCount := len(plan.ExecutionOrder)

	stats := statsStyle.Render(fmt.Sprintf("  %d tasks in %d groups", taskCount, groupCount))

	header := title + stats

	// Add border below
	headerWithBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(styles.BorderColor).
		Width(width - 4).
		Render(header)

	return headerWithBorder
}

// renderPlanEditorSummary renders the plan summary section
func (m Model) renderPlanEditorSummary(plan *orchestrator.PlanSpec, width int) string {
	var b strings.Builder

	// Section title
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.BlueColor)

	b.WriteString(sectionStyle.Render("Summary"))
	b.WriteString("\n")

	// Truncate summary if too long
	summary := plan.Summary
	maxSummaryLen := width - 8
	if len(summary) > maxSummaryLen {
		// Truncate to fit available width, with ellipsis
		summary = truncate(summary, maxSummaryLen)
	}

	summaryStyle := lipgloss.NewStyle().
		Foreground(styles.TextColor).
		PaddingLeft(2)

	b.WriteString(summaryStyle.Render(summary))

	return b.String()
}

// renderPlanEditorTaskList renders the scrollable task list
func (m Model) renderPlanEditorTaskList(plan *orchestrator.PlanSpec, width, height int) string {
	if len(plan.Tasks) == 0 {
		return styles.Muted.Render("  No tasks in plan")
	}

	var b strings.Builder

	// Section title
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.BlueColor)

	b.WriteString(sectionStyle.Render("Tasks"))
	b.WriteString("\n")

	// Calculate visible range
	visibleStart, visibleEnd := m.getVisibleTaskRange(len(plan.Tasks), height-2) // -2 for title and potential scroll indicator

	// Show scroll indicator at top if needed
	if visibleStart > 0 {
		scrollUpIndicator := styles.Muted.Render(fmt.Sprintf("  ▲ %d more above", visibleStart))
		b.WriteString(scrollUpIndicator)
		b.WriteString("\n")
	}

	// Render visible tasks
	for i := visibleStart; i < visibleEnd && i < len(plan.Tasks); i++ {
		task := &plan.Tasks[i]
		isSelected := i == m.planEditor.selectedTaskIdx
		taskLine := m.renderPlanEditorTaskLine(task, isSelected, width-4)
		b.WriteString(taskLine)
		b.WriteString("\n")
	}

	// Show scroll indicator at bottom if needed
	if visibleEnd < len(plan.Tasks) {
		remaining := len(plan.Tasks) - visibleEnd
		scrollDownIndicator := styles.Muted.Render(fmt.Sprintf("  ▼ %d more below", remaining))
		b.WriteString(scrollDownIndicator)
	}

	return b.String()
}

// renderPlanEditorTaskLine renders a single task line in the task list
func (m Model) renderPlanEditorTaskLine(task *orchestrator.PlannedTask, selected bool, maxWidth int) string {
	var parts []string

	// Selection indicator
	if selected {
		parts = append(parts, "▶")
	} else {
		parts = append(parts, " ")
	}

	// Task ID (compact form)
	idStyle := lipgloss.NewStyle().Foreground(styles.PurpleColor)
	taskID := truncate(task.ID, 15)
	parts = append(parts, idStyle.Render(taskID))

	// Complexity indicator
	complexityIcon := complexityIndicator(task.EstComplexity)
	complexityStyle := complexityStyle(task.EstComplexity)
	parts = append(parts, complexityStyle.Render(complexityIcon))

	// Task title (takes remaining space)
	titleMaxLen := max(maxWidth-25, 10) // Reserve space for ID, complexity, badges
	title := truncate(task.Title, titleMaxLen)
	parts = append(parts, title)

	// Badges: dependency count and file count
	badges := m.renderTaskBadges(task)
	if badges != "" {
		parts = append(parts, badges)
	}

	// Join parts
	line := strings.Join(parts, " ")

	// Apply selected styling
	if selected {
		// Check if we're editing a field
		if m.planEditor.editingField != "" {
			// Highlight the editing field differently
			line = lipgloss.NewStyle().
				Background(styles.SurfaceColor).
				Foreground(styles.YellowColor).
				Bold(true).
				Render(line)
		} else {
			// Normal selection highlight
			line = lipgloss.NewStyle().
				Background(styles.PrimaryColor).
				Foreground(styles.TextColor).
				Bold(true).
				Render(line)
		}
	}

	return "  " + line
}

// renderTaskBadges renders the dependency count and file count badges for a task
func (m Model) renderTaskBadges(task *orchestrator.PlannedTask) string {
	var badges []string

	// Dependency badge
	depCount := len(task.DependsOn)
	if depCount > 0 {
		depStyle := lipgloss.NewStyle().
			Foreground(styles.BlueColor).
			Background(lipgloss.Color("#1e3a5f")).
			Padding(0, 1)
		badges = append(badges, depStyle.Render(fmt.Sprintf("↳%d", depCount)))
	}

	// File badge
	fileCount := len(task.Files)
	if fileCount > 0 {
		fileStyle := lipgloss.NewStyle().
			Foreground(styles.GreenColor).
			Background(lipgloss.Color("#1e3f1e")).
			Padding(0, 1)
		badges = append(badges, fileStyle.Render(fmt.Sprintf("📄%d", fileCount)))
	}

	return strings.Join(badges, " ")
}

// renderPlanEditorFooter renders the instructions footer with keyboard shortcuts
func (m Model) renderPlanEditorFooter(width int) string {
	var keys []string

	// Different keys depending on editing state
	if m.planEditor.editingField != "" {
		// Editing mode shortcuts
		keys = append(keys, "[enter] save")
		keys = append(keys, "[esc] cancel")
		keys = append(keys, "[←→] cursor")
	} else {
		// Normal mode shortcuts
		keys = append(keys, "[↑↓] select")
		keys = append(keys, "[enter] edit")
		keys = append(keys, "[tab] field")
		keys = append(keys, "[d] delete")
		keys = append(keys, "[a] add")
		keys = append(keys, "[esc] exit")
		keys = append(keys, "[e] execute")
	}

	helpText := strings.Join(keys, "  ")

	footerStyle := lipgloss.NewStyle().
		Foreground(styles.MutedColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(styles.BorderColor).
		Width(width - 4).
		MarginTop(1)

	return footerStyle.Render(helpText)
}

// renderTaskDetail renders the full task view when a task is selected
// This shows all task details including description, files, and dependencies
func (m Model) renderTaskDetail(task *orchestrator.PlannedTask, width int) string {
	if task == nil {
		return styles.Muted.Render("No task selected")
	}

	var b strings.Builder

	// Task header with ID and title
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.PrimaryColor)

	b.WriteString(headerStyle.Render(fmt.Sprintf("[%s] %s", task.ID, task.Title)))
	b.WriteString("\n\n")

	// Complexity and Priority
	infoStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	complexIcon := complexityIndicator(task.EstComplexity)
	complexStyle := complexityStyle(task.EstComplexity)
	b.WriteString(infoStyle.Render("Complexity: "))
	b.WriteString(complexStyle.Render(fmt.Sprintf("%s %s", complexIcon, task.EstComplexity)))
	b.WriteString(infoStyle.Render(fmt.Sprintf("  Priority: %d", task.Priority)))
	b.WriteString("\n\n")

	// Description
	sectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.BlueColor)

	b.WriteString(sectionStyle.Render("Description"))
	b.WriteString("\n")

	descStyle := lipgloss.NewStyle().
		Foreground(styles.TextColor).
		PaddingLeft(2).
		Width(width - 8)

	// Wrap description to fit width
	desc := task.Description
	if len(desc) > 500 {
		desc = desc[:497] + "..."
	}
	b.WriteString(descStyle.Render(desc))
	b.WriteString("\n\n")

	// Dependencies
	b.WriteString(sectionStyle.Render("Dependencies"))
	b.WriteString("\n")
	if len(task.DependsOn) == 0 {
		b.WriteString(styles.Muted.Render("  None (can run first)"))
	} else {
		for _, dep := range task.DependsOn {
			depLine := fmt.Sprintf("  ↳ %s", dep)
			b.WriteString(lipgloss.NewStyle().Foreground(styles.BlueColor).Render(depLine))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")

	// Files
	b.WriteString(sectionStyle.Render("Expected Files"))
	b.WriteString("\n")
	if len(task.Files) == 0 {
		b.WriteString(styles.Muted.Render("  Not specified"))
	} else {
		for _, file := range task.Files {
			fileLine := fmt.Sprintf("  📄 %s", file)
			b.WriteString(lipgloss.NewStyle().Foreground(styles.GreenColor).Render(fileLine))
			b.WriteString("\n")
		}
	}

	// Apply overall styling
	detailBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.BorderColor).
		Padding(1, 2).
		Width(width - 4)

	return detailBox.Render(b.String())
}

// renderEditField renders an inline edit field for the specified field
// Shows the current value with cursor position highlighted
func (m Model) renderEditField(fieldName, value string, cursor int, width int) string {
	// Label
	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.YellowColor)

	label := labelStyle.Render(fieldName + ":")

	// Calculate visible portion of the value if it's too long
	maxValueLen := max(width-len(fieldName)-10, 20)

	// Ensure cursor is within bounds
	cursor = max(0, min(cursor, len(value)))

	// Calculate visible window around cursor
	visibleStart := 0
	visibleEnd := len(value)
	if len(value) > maxValueLen {
		// Center the cursor in the visible window
		halfWindow := maxValueLen / 2
		visibleStart = max(cursor-halfWindow, 0)
		visibleEnd = visibleStart + maxValueLen
		if visibleEnd > len(value) {
			visibleEnd = len(value)
			visibleStart = max(visibleEnd-maxValueLen, 0)
		}
	}

	// Build the visible value with cursor indicator
	var displayValue strings.Builder
	visibleValue := value[visibleStart:visibleEnd]
	cursorInVisible := cursor - visibleStart

	// Show ellipsis if truncated at start
	if visibleStart > 0 {
		displayValue.WriteString("…")
		cursorInVisible-- // Adjust for ellipsis
	}

	// Render value with cursor
	valueStyle := lipgloss.NewStyle().Foreground(styles.TextColor)
	cursorStyle := lipgloss.NewStyle().
		Background(styles.YellowColor).
		Foreground(lipgloss.Color("#000000"))

	for i, r := range visibleValue {
		adjustedIndex := i
		if visibleStart > 0 {
			adjustedIndex++ // Account for leading ellipsis
		}
		if adjustedIndex == cursorInVisible {
			displayValue.WriteString(cursorStyle.Render(string(r)))
		} else {
			displayValue.WriteString(valueStyle.Render(string(r)))
		}
	}

	// If cursor is at end, show cursor block
	if cursorInVisible >= len(visibleValue) {
		displayValue.WriteString(cursorStyle.Render(" "))
	}

	// Show ellipsis if truncated at end
	if visibleEnd < len(value) {
		displayValue.WriteString("…")
	}

	// Combine label and value
	editLine := fmt.Sprintf("%s %s", label, displayValue.String())

	// Apply edit box styling
	editBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.YellowColor).
		Padding(0, 1).
		Width(width - 4)

	return editBoxStyle.Render(editLine)
}

// Helper functions

// getPlanForEditor retrieves the plan from the ultra-plan session
func (m Model) getPlanForEditor() *orchestrator.PlanSpec {
	if m.ultraPlan == nil || m.ultraPlan.coordinator == nil {
		return nil
	}
	session := m.ultraPlan.coordinator.Session()
	if session == nil {
		return nil
	}
	return session.Plan
}

// getVisibleTaskRange calculates the visible task index range based on scroll offset
// Returns (startIndex, endIndex) where endIndex is exclusive
func (m Model) getVisibleTaskRange(totalTasks, visibleHeight int) (int, int) {
	if m.planEditor == nil {
		return 0, min(totalTasks, visibleHeight)
	}

	// Adjust scroll offset to keep selected task visible
	selectedIdx := m.planEditor.selectedTaskIdx
	scrollOffset := m.planEditor.scrollOffset

	// Ensure selected task is visible
	if selectedIdx < scrollOffset {
		scrollOffset = selectedIdx
	}
	if selectedIdx >= scrollOffset+visibleHeight {
		scrollOffset = selectedIdx - visibleHeight + 1
	}

	// Clamp scroll offset
	scrollOffset = max(scrollOffset, 0)
	maxOffset := max(totalTasks-visibleHeight, 0)
	scrollOffset = min(scrollOffset, maxOffset)

	// Update scroll offset in state
	m.planEditor.scrollOffset = scrollOffset

	startIdx := scrollOffset
	endIdx := min(scrollOffset+visibleHeight, totalTasks)

	return startIdx, endIdx
}

// complexityStyle returns the lipgloss style for a complexity level
func complexityStyle(complexity orchestrator.TaskComplexity) lipgloss.Style {
	switch complexity {
	case orchestrator.ComplexityLow:
		return lipgloss.NewStyle().Foreground(styles.GreenColor)
	case orchestrator.ComplexityMedium:
		return lipgloss.NewStyle().Foreground(styles.YellowColor)
	case orchestrator.ComplexityHigh:
		return lipgloss.NewStyle().Foreground(styles.RedColor)
	default:
		return lipgloss.NewStyle().Foreground(styles.MutedColor)
	}
}

// IsEditing returns true if the plan editor is currently in field editing mode
func (s *PlanEditorState) IsEditing() bool {
	return s != nil && s.editingField != ""
}

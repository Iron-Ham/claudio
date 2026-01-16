package ultraplan

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// TaskRenderer handles rendering of task lists with status icons and text wrapping.
type TaskRenderer struct {
	ctx *RenderContext
}

// NewTaskRenderer creates a new task renderer with the given context.
func NewTaskRenderer(ctx *RenderContext) *TaskRenderer {
	return &TaskRenderer{ctx: ctx}
}

// ExecutionTaskResult holds the rendered task line(s) and the number of lines used.
type ExecutionTaskResult struct {
	Content   string // Rendered content (may contain newlines for wrapped text)
	LineCount int    // Number of lines this task occupies
}

// RenderExecutionTaskLine renders a task line in the execution section.
// When the task is selected, the title may wrap to multiple lines for better readability.
func (t *TaskRenderer) RenderExecutionTaskLine(session *orchestrator.UltraPlanSession, task *orchestrator.PlannedTask, instanceID string, selected, navigable bool, maxWidth int) ExecutionTaskResult {
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

	// Calculate available space for title
	// Format: "    X title" where X is the status icon (4 spaces indent + icon + space)
	titleLen := maxWidth - 6

	// For selected tasks, wrap the title across multiple lines instead of truncating
	if selected && len([]rune(task.Title)) > titleLen {
		return t.renderWrappedTaskLine(task.Title, statusIcon, titleLen, maxWidth)
	}

	// Standard single-line rendering (truncate if needed)
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

	return ExecutionTaskResult{Content: line, LineCount: 1}
}

// renderWrappedTaskLine renders a task title that wraps across multiple lines.
// Used for selected tasks to show the full title instead of truncating.
func (t *TaskRenderer) renderWrappedTaskLine(title, statusIcon string, firstLineLen, maxWidth int) ExecutionTaskResult {
	// Guard against pathologically small widths (e.g., during window resizing)
	// Note: We use plain icon (not statusStyle.Render) to avoid ANSI reset codes
	// that would break the background color when selectedStyle wraps the line.
	if firstLineLen <= 0 || maxWidth <= 6 {
		line := fmt.Sprintf("    %s %s", statusIcon, truncate(title, 3))
		selectedStyle := lipgloss.NewStyle().
			Background(styles.PrimaryColor).
			Foreground(styles.TextColor)
		return ExecutionTaskResult{Content: selectedStyle.Render(line), LineCount: 1}
	}

	selectedStyle := lipgloss.NewStyle().
		Background(styles.PrimaryColor).
		Foreground(styles.TextColor)

	remaining := []rune(title)
	var lines []string

	// First line: "    X <part of title>"
	// Note: We use the plain icon (not statusStyle.Render) because applying
	// statusStyle first would embed ANSI reset codes that break the background
	// color when selectedStyle wraps the entire line.
	firstPart := wrapAtWordBoundary(remaining, firstLineLen)
	firstLine := fmt.Sprintf("    %s %s", statusIcon, firstPart)
	lines = append(lines, selectedStyle.Render(padToWidth(firstLine, maxWidth)))

	remaining = trimLeadingSpaces(remaining[len([]rune(firstPart)):])

	// Continuation lines: indented to align with title text (4 spaces + icon + space = 6 characters)
	const continuationIndent = 6
	continuationLen := maxWidth - continuationIndent
	indent := strings.Repeat(" ", continuationIndent)

	for len(remaining) > 0 {
		chunk := wrapAtWordBoundary(remaining, continuationLen)
		if len(chunk) == 0 {
			// Safety: prevent infinite loop if wrapAtWordBoundary returns empty.
			// This should not happen under normal conditions since we guard against
			// small widths above. If triggered, it indicates a bug in the wrapping logic.
			break
		}
		remaining = trimLeadingSpaces(remaining[len([]rune(chunk)):])
		lines = append(lines, selectedStyle.Render(indent+padToWidth(chunk, continuationLen)))
	}

	return ExecutionTaskResult{
		Content:   strings.Join(lines, "\n"),
		LineCount: len(lines),
	}
}

// RenderPhaseInstanceLine renders a line for a phase instance (coordinator, synthesis, consolidation).
func (t *TaskRenderer) RenderPhaseInstanceLine(inst *orchestrator.Instance, name string, selected, navigable bool, maxWidth int) string {
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

// RenderGroupConsolidatorLine renders a consolidator line in the execution section.
func (t *TaskRenderer) RenderGroupConsolidatorLine(inst *orchestrator.Instance, groupIndex int, selected, navigable bool, maxWidth int) string {
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

// GroupStats holds statistics about a task group for collapsed display.
type GroupStats struct {
	Total     int  // Total number of tasks in group
	Completed int  // Number of completed tasks
	Failed    int  // Number of failed tasks
	Running   int  // Number of currently running tasks
	HasFailed bool // Whether any task has failed
}

// CalculateGroupStats calculates statistics for a task group.
func (t *TaskRenderer) CalculateGroupStats(session *orchestrator.UltraPlanSession, group []string) GroupStats {
	stats := GroupStats{Total: len(group)}

	for _, taskID := range group {
		// Check if completed
		for _, ct := range session.CompletedTasks {
			if ct == taskID {
				stats.Completed++
				break
			}
		}

		// Check if failed
		for _, ft := range session.FailedTasks {
			if ft == taskID {
				stats.Failed++
				stats.HasFailed = true
				break
			}
		}

		// Check if running (has instance but not completed/failed)
		if _, running := session.TaskToInstance[taskID]; running {
			// Only count as running if not already counted as completed/failed
			isCompleted := false
			for _, ct := range session.CompletedTasks {
				if ct == taskID {
					isCompleted = true
					break
				}
			}
			isFailed := false
			for _, ft := range session.FailedTasks {
				if ft == taskID {
					isFailed = true
					break
				}
			}
			if !isCompleted && !isFailed {
				stats.Running++
			}
		}
	}

	return stats
}

// FormatGroupSummary formats the summary statistics for a collapsed group.
func (t *TaskRenderer) FormatGroupSummary(stats GroupStats) string {
	if stats.Running > 0 {
		return fmt.Sprintf("[⟳ %d/%d]", stats.Completed, stats.Total)
	}
	if stats.HasFailed {
		return fmt.Sprintf("[✗ %d/%d]", stats.Completed, stats.Total)
	}
	if stats.Completed == stats.Total {
		return fmt.Sprintf("[✓ %d/%d]", stats.Completed, stats.Total)
	}
	return fmt.Sprintf("[%d/%d]", stats.Completed, stats.Total)
}

// GetGroupStatus returns a status indicator for a task group.
func (t *TaskRenderer) GetGroupStatus(session *orchestrator.UltraPlanSession, group []string) string {
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

// FindInstanceIDForTask finds the instance ID associated with a task.
// It uses the authoritative TaskToInstance map which tracks currently running tasks.
// Completed or pending tasks won't have entries (and shouldn't be highlighted as selected).
func (t *TaskRenderer) FindInstanceIDForTask(session *orchestrator.UltraPlanSession, taskID string) string {
	if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
		return instID
	}
	return ""
}

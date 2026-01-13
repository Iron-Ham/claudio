// Package view provides reusable view components for the TUI.
package view

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// InstanceView handles rendering of a single instance's detail view.
// It extracts and encapsulates all instance-specific rendering logic
// that was previously embedded in the main app.go Model.
type InstanceView struct {
	// Width is the available render width
	Width int
	// MaxOutputLines is the maximum number of output lines to display
	MaxOutputLines int
}

// NewInstanceView creates a new InstanceView with the given dimensions.
func NewInstanceView(width, maxOutputLines int) *InstanceView {
	return &InstanceView{
		Width:          width,
		MaxOutputLines: maxOutputLines,
	}
}

// RenderState holds the dynamic state needed for rendering an instance.
// This separates the render-time state from the persistent instance data.
type RenderState struct {
	// Output is the current output text for this instance
	Output string
	// IsRunning indicates if the instance manager is currently running
	IsRunning bool
	// InputMode indicates if the TUI is in input mode for this instance
	InputMode bool
	// ScrollOffset is the current scroll position (line number)
	ScrollOffset int
	// AutoScrollEnabled indicates if auto-scroll is enabled
	AutoScrollEnabled bool
	// HasNewOutput indicates new output arrived while scrolled up
	HasNewOutput bool
	// SearchPattern is the current search pattern (empty if no search)
	SearchPattern string
	// SearchRegex is the compiled search regex (nil if no search)
	SearchRegex *regexp.Regexp
	// SearchMatches are the line numbers with matches
	SearchMatches []int
	// SearchCurrent is the index of the current match
	SearchCurrent int
	// SearchMode indicates if the search input is active
	SearchMode bool
	// GroupedViewEnabled indicates if the grouped view mode is active
	GroupedViewEnabled bool
}

// GroupContext holds information about an instance's group membership.
// This is used to render the group status header in the instance view.
type GroupContext struct {
	// Group is the group this instance belongs to
	Group *orchestrator.InstanceGroup
	// TaskIndex is the 1-based index of this task within the group
	TaskIndex int
	// TotalTasks is the total number of tasks in the group
	TotalTasks int
	// SiblingTasks are the other instances in the same group
	SiblingTasks []*orchestrator.Instance
	// DependencyGroups are the groups this group depends on
	DependencyGroups []*orchestrator.InstanceGroup
	// PendingDependencies are dependency instances that haven't completed
	PendingDependencies []*orchestrator.Instance
	// AllDependenciesMet is true when all group dependencies are satisfied
	AllDependenciesMet bool
}

// Render renders the complete instance detail view.
// This is the main entry point for instance rendering.
// Use RenderWithSession if you need to display dependency information.
func (v *InstanceView) Render(inst *orchestrator.Instance, state RenderState) string {
	return v.RenderWithSession(inst, state, nil)
}

// RenderWithSession renders the complete instance detail view with session context.
// When session is provided, dependency information is displayed.
func (v *InstanceView) RenderWithSession(inst *orchestrator.Instance, state RenderState, session *orchestrator.Session) string {
	var b strings.Builder

	// Render header (status badge + branch info)
	b.WriteString(v.RenderHeader(inst))
	b.WriteString("\n")

	// Render group status header if in grouped mode
	if state.GroupedViewEnabled && session != nil {
		groupCtx := v.BuildGroupContext(inst, session)
		if groupCtx != nil {
			b.WriteString(v.RenderGroupStatusHeader(groupCtx))
			b.WriteString("\n")
		}
	}

	// Render task description
	b.WriteString(v.RenderTask(inst.Task))
	b.WriteString("\n")

	// Render dependency information if available (individual instance dependencies)
	if session != nil {
		depInfo := v.RenderDependencies(inst, session)
		if depInfo != "" {
			b.WriteString(depInfo)
		}
	}

	// Render metrics if enabled and available
	cfg := config.Get()
	if cfg.Resources.ShowMetricsInSidebar && inst.Metrics != nil {
		metricsLine := v.FormatMetrics(inst.Metrics)
		if metricsLine != "" {
			b.WriteString(styles.Muted.Render(metricsLine))
			b.WriteString("\n")
		}
	}

	// Render running/waiting state indicator
	b.WriteString(v.RenderStatusBanner(state.IsRunning, state.InputMode))
	b.WriteString("\n")

	// Render output area
	b.WriteString(v.RenderOutput(inst.ID, state))

	// Render search bar if in search mode or has active search
	if state.SearchMode || state.SearchPattern != "" {
		b.WriteString("\n")
		b.WriteString(v.RenderSearchBar(state))
	}

	return b.String()
}

// RenderHeader renders the instance header with status badge and branch info.
func (v *InstanceView) RenderHeader(inst *orchestrator.Instance) string {
	statusColor := styles.StatusColor(string(inst.Status))
	statusBadge := styles.StatusBadge.Background(statusColor).Render(string(inst.Status))
	info := fmt.Sprintf("%s  Branch: %s", statusBadge, inst.Branch)
	return styles.InstanceInfo.Render(info)
}

// BuildGroupContext builds the group context for an instance.
// Returns nil if the instance is not part of any group.
func (v *InstanceView) BuildGroupContext(inst *orchestrator.Instance, session *orchestrator.Session) *GroupContext {
	if session == nil || len(session.Groups) == 0 {
		return nil
	}

	// Find the group containing this instance
	group := session.GetGroupForInstance(inst.ID)
	if group == nil {
		return nil
	}

	ctx := &GroupContext{
		Group:      group,
		TotalTasks: len(group.Instances),
	}

	// Find task index within group and collect siblings
	for i, instID := range group.Instances {
		if instID == inst.ID {
			ctx.TaskIndex = i + 1 // 1-based
		}
		sibling := session.GetInstance(instID)
		if sibling != nil && sibling.ID != inst.ID {
			ctx.SiblingTasks = append(ctx.SiblingTasks, sibling)
		}
	}

	// Collect dependency groups and check their status
	ctx.AllDependenciesMet = true
	for _, depGroupID := range group.DependsOn {
		depGroup := session.GetGroup(depGroupID)
		if depGroup != nil {
			ctx.DependencyGroups = append(ctx.DependencyGroups, depGroup)
			// Check if this dependency group is complete
			if depGroup.Phase != orchestrator.GroupPhaseCompleted {
				ctx.AllDependenciesMet = false
				// Collect pending instances from this dependency group
				for _, depInstID := range depGroup.AllInstanceIDs() {
					depInst := session.GetInstance(depInstID)
					if depInst != nil && depInst.Status != orchestrator.StatusCompleted {
						ctx.PendingDependencies = append(ctx.PendingDependencies, depInst)
					}
				}
			}
		}
	}

	return ctx
}

// RenderGroupStatusHeader renders the group status line for an instance.
// Shows group membership, task position, and dependency status.
func (v *InstanceView) RenderGroupStatusHeader(ctx *GroupContext) string {
	if ctx == nil || ctx.Group == nil {
		return ""
	}

	var b strings.Builder

	// Build group info line: "Group 2: Core Logic • Task 1 of 3"
	phaseColor := PhaseColor(ctx.Group.Phase)
	phaseIcon := PhaseIndicator(ctx.Group.Phase)

	groupStyle := lipgloss.NewStyle().Foreground(phaseColor).Bold(true)
	b.WriteString(groupStyle.Render(ctx.Group.Name))

	// Add phase indicator
	b.WriteString(" ")
	b.WriteString(lipgloss.NewStyle().Foreground(phaseColor).Render(phaseIcon))

	// Add task position
	b.WriteString(styles.Muted.Render(fmt.Sprintf(" • Task %d of %d", ctx.TaskIndex, ctx.TotalTasks)))

	// Add dependency group names if any
	if len(ctx.DependencyGroups) > 0 {
		var depNames []string
		for _, dg := range ctx.DependencyGroups {
			depNames = append(depNames, dg.Name)
		}
		b.WriteString(styles.Muted.Render(" • Depends on: "))
		b.WriteString(styles.Muted.Render(strings.Join(depNames, ", ")))
	}

	b.WriteString("\n")

	// Show dependency status line
	if len(ctx.DependencyGroups) > 0 {
		if ctx.AllDependenciesMet {
			checkStyle := lipgloss.NewStyle().Foreground(styles.GreenColor)
			b.WriteString(checkStyle.Render("Dependencies satisfied ✓"))
		} else if len(ctx.PendingDependencies) > 0 {
			// Show waiting status with pending task names
			waitStyle := lipgloss.NewStyle().Foreground(styles.YellowColor)
			b.WriteString(waitStyle.Render("Waiting for: "))
			var pendingNames []string
			maxPending := 3 // Limit to avoid very long lines
			for i, dep := range ctx.PendingDependencies {
				if i >= maxPending {
					pendingNames = append(pendingNames, fmt.Sprintf("+%d more", len(ctx.PendingDependencies)-maxPending))
					break
				}
				pendingNames = append(pendingNames, truncateTask(dep.EffectiveName(), 25))
			}
			b.WriteString(styles.Muted.Render(strings.Join(pendingNames, ", ")))
		}
	}

	// Show sibling task connector if there are siblings
	if len(ctx.SiblingTasks) > 0 {
		b.WriteString("\n")
		b.WriteString(v.RenderSiblingConnector(ctx))
	}

	return b.String()
}

// RenderSiblingConnector renders a visual connector showing sibling tasks.
// This helps users understand the relationship between tasks in the same group.
func (v *InstanceView) RenderSiblingConnector(ctx *GroupContext) string {
	if ctx == nil || len(ctx.SiblingTasks) == 0 {
		return ""
	}

	var b strings.Builder

	// Use box-drawing characters for visual connection
	connectorStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	// Show "Siblings:" label with sibling count
	b.WriteString(connectorStyle.Render("├─ Siblings: "))

	// Show sibling tasks with status indicators
	var siblingStrs []string
	maxSiblings := 4 // Limit display
	for i, sib := range ctx.SiblingTasks {
		if i >= maxSiblings {
			siblingStrs = append(siblingStrs, fmt.Sprintf("+%d more", len(ctx.SiblingTasks)-maxSiblings))
			break
		}
		statusIcon := styles.StatusIcon(string(sib.Status))
		statusColor := styles.StatusColor(string(sib.Status))
		iconStyle := lipgloss.NewStyle().Foreground(statusColor)
		name := truncateTask(sib.EffectiveName(), 20)
		siblingStrs = append(siblingStrs, iconStyle.Render(statusIcon)+" "+name)
	}

	b.WriteString(strings.Join(siblingStrs, connectorStyle.Render(" │ ")))

	return b.String()
}

// RenderDependencies renders the dependency chain information for an instance.
// Returns empty string if the instance has no dependencies or dependents.
func (v *InstanceView) RenderDependencies(inst *orchestrator.Instance, session *orchestrator.Session) string {
	if len(inst.DependsOn) == 0 && len(inst.Dependents) == 0 {
		return ""
	}

	var b strings.Builder

	// Show upstream dependencies (what this instance waits for)
	if len(inst.DependsOn) > 0 {
		b.WriteString(styles.Muted.Render("Depends on: "))
		var deps []string
		for _, depID := range inst.DependsOn {
			depInst := session.GetInstance(depID)
			if depInst != nil {
				var statusIcon string
				switch depInst.Status {
				case orchestrator.StatusCompleted:
					statusIcon = "✓"
				case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
					statusIcon = "●"
				case orchestrator.StatusError, orchestrator.StatusTimeout:
					statusIcon = "✗"
				default:
					statusIcon = "○" // pending
				}
				deps = append(deps, fmt.Sprintf("%s %s", statusIcon, truncateTask(depInst.Task, 20)))
			} else {
				deps = append(deps, depID+" (not found)")
			}
		}
		b.WriteString(strings.Join(deps, ", "))
		b.WriteString("\n")
	}

	// Show downstream dependents (what instances wait for this one)
	if len(inst.Dependents) > 0 {
		b.WriteString(styles.Muted.Render("Dependents: "))
		var deps []string
		for _, depID := range inst.Dependents {
			depInst := session.GetInstance(depID)
			if depInst != nil {
				deps = append(deps, truncateTask(depInst.Task, 20))
			} else {
				deps = append(deps, depID)
			}
		}
		b.WriteString(strings.Join(deps, ", "))
		b.WriteString("\n")
	}

	return b.String()
}

// truncateTask truncates a task description to maxLen characters.
func truncateTask(task string, maxLen int) string {
	// Remove newlines
	task = strings.ReplaceAll(task, "\n", " ")
	if len(task) <= maxLen {
		return task
	}
	return task[:maxLen-3] + "..."
}

// MaxTaskDisplayLines is the maximum number of lines shown for task descriptions.
// Task text exceeding this limit is truncated with "...".
const MaxTaskDisplayLines = 5

// OverheadParams contains the parameters needed to calculate overhead lines.
// These are the variable factors that affect how many lines are used above the output area.
type OverheadParams struct {
	// Task is the instance task description
	Task string
	// HasDependencies indicates if the instance has dependencies to display
	HasDependencies bool
	// HasDependents indicates if the instance has dependents to display
	HasDependents bool
	// ShowMetrics indicates if metrics display is enabled in config.
	// Metrics only contribute to overhead when both ShowMetrics AND HasMetrics are true.
	ShowMetrics bool
	// HasMetrics indicates if the instance has metrics data available
	HasMetrics bool
	// IsRunning indicates if the instance is currently running
	IsRunning bool
	// HasSearchActive indicates if a search pattern is active
	HasSearchActive bool
	// HasScrollIndicator indicates if the output needs a scroll indicator
	HasScrollIndicator bool
	// HasGroupHeader indicates if the group status header should be shown
	HasGroupHeader bool
	// HasGroupDependencies indicates if the group has dependencies (adds dependency status line)
	HasGroupDependencies bool
	// HasSiblings indicates if the instance has sibling tasks in the same group
	HasSiblings bool
}

// CalculateOverheadLines calculates the number of lines used by the instance view
// above the output area. This accounts for header, group status, task, dependencies,
// metrics, status banner, scroll indicator, and search bar.
func (v *InstanceView) CalculateOverheadLines(params OverheadParams) int {
	lines := 0

	// Header section: 1 line rendered + 1 for newline separator
	lines += 2

	// Group status header (if in grouped mode and instance is part of a group)
	if params.HasGroupHeader {
		// Group name line + newline
		lines += 2
		// Dependency status line (if group has dependencies)
		if params.HasGroupDependencies {
			lines++
		}
		// Sibling connector line (if instance has siblings)
		if params.HasSiblings {
			lines++
		}
	}

	// Task section: 1 to MaxTaskDisplayLines+1 lines (extra for "..." when truncated), plus newline
	taskLines := strings.Count(params.Task, "\n") + 1
	if taskLines > MaxTaskDisplayLines {
		taskLines = MaxTaskDisplayLines + 1 // +1 for the "..." line
	}
	lines += taskLines + 1 // +1 for the newline after task

	// Dependencies: 1 line each if present
	if params.HasDependencies {
		lines++
	}
	if params.HasDependents {
		lines++
	}

	// Metrics: 1 line + newline if enabled and has data
	if params.ShowMetrics && params.HasMetrics {
		lines += 2
	}

	// Status banner: 1 line + newline if running
	if params.IsRunning {
		lines += 2
	} else {
		// Empty banner still adds a newline
		lines++
	}

	// Scroll indicator: 1 line + newline if output is scrollable
	if params.HasScrollIndicator {
		lines += 2
	}

	// Search bar: 1 line + newline if search is active
	if params.HasSearchActive {
		lines += 2
	}

	return lines
}

// RenderTask renders the task description, truncated to MaxTaskDisplayLines.
func (v *InstanceView) RenderTask(task string) string {
	taskDisplay := truncateLines(task, MaxTaskDisplayLines)
	return styles.Subtitle.Render("Task: " + taskDisplay)
}

// FormatMetrics formats instance metrics for display.
// Returns an empty string if no metrics are available.
func (v *InstanceView) FormatMetrics(metrics *orchestrator.Metrics) string {
	if metrics == nil {
		return ""
	}

	var parts []string

	// Token usage
	if metrics.InputTokens > 0 || metrics.OutputTokens > 0 {
		parts = append(parts, fmt.Sprintf("Tokens: %s in / %s out",
			instmetrics.FormatTokens(metrics.InputTokens),
			instmetrics.FormatTokens(metrics.OutputTokens)))
	}

	// Cost
	if metrics.Cost > 0 {
		parts = append(parts, instmetrics.FormatCost(metrics.Cost))
	}

	// Duration
	if duration := metrics.Duration(); duration > 0 {
		parts = append(parts, FormatDuration(duration))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "  │  ")
}

// RenderStatusBanner renders the running/input mode status banner.
func (v *InstanceView) RenderStatusBanner(isRunning, inputMode bool) string {
	if !isRunning {
		return ""
	}

	if inputMode {
		// Show active input mode indicator
		inputBanner := lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.TextColor).
			Background(styles.WarningColor).
			Padding(0, 1).
			Render("INPUT MODE")
		return inputBanner + "  " + styles.Muted.Render("Press ") +
			styles.HelpKey.Render("Ctrl+]") + styles.Muted.Render(" to exit")
	}

	// Show hint to enter input mode
	runningBanner := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.TextColor).
		Background(styles.SecondaryColor).
		Padding(0, 1).
		Render("RUNNING")
	return runningBanner + "  " + styles.Muted.Render("Press ") +
		styles.HelpKey.Render("[i]") + styles.Muted.Render(" to interact  ") +
		styles.HelpKey.Render("[:tmux]") + styles.Muted.Render(" for tmux attach cmd")
}

// RenderOutput renders the output area with scrolling support.
func (v *InstanceView) RenderOutput(instanceID string, state RenderState) string {
	var b strings.Builder

	output := state.Output
	if output == "" {
		output = "No output yet. Press [s] to start this instance."
		outputBox := styles.OutputArea.
			Width(v.Width - 4).
			Height(v.MaxOutputLines).
			Render(output)
		b.WriteString(outputBox)
		return b.String()
	}

	// Split output into lines and apply scroll
	lines := strings.Split(output, "\n")
	totalLines := len(lines)
	maxLines := v.MaxOutputLines

	// Clamp scroll position
	scrollOffset := state.ScrollOffset
	maxScroll := v.getMaxScroll(totalLines)
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}

	// Calculate visible range
	startLine := scrollOffset
	endLine := min(startLine+maxLines, totalLines)

	// Get visible lines
	var visibleLines []string
	if totalLines <= maxLines {
		// No scrolling needed, show all
		visibleLines = lines
	} else {
		visibleLines = lines[startLine:endLine]
	}

	// Apply search highlighting
	if state.SearchRegex != nil && state.SearchPattern != "" {
		visibleLines = v.highlightSearchMatches(visibleLines, startLine, state)
	}

	visibleOutput := strings.Join(visibleLines, "\n")

	// Build scroll indicator
	if totalLines > maxLines {
		scrollIndicator := v.buildScrollIndicator(
			scrollOffset, startLine, endLine, totalLines, maxScroll,
			state.AutoScrollEnabled, state.HasNewOutput,
		)
		b.WriteString(scrollIndicator)
		b.WriteString("\n")
	}

	outputBox := styles.OutputArea.
		Width(v.Width - 4).
		Height(maxLines).
		Render(visibleOutput)

	b.WriteString(outputBox)

	return b.String()
}

// getMaxScroll calculates the maximum scroll offset for the given total lines.
func (v *InstanceView) getMaxScroll(totalLines int) int {
	return max(totalLines-v.MaxOutputLines, 0)
}

// buildScrollIndicator builds the scroll position indicator string.
func (v *InstanceView) buildScrollIndicator(
	scrollOffset, startLine, endLine, totalLines, maxScroll int,
	autoScrollEnabled, hasNew bool,
) string {
	if hasNew && !autoScrollEnabled {
		// New output arrived while scrolled up
		return styles.Warning.Render(fmt.Sprintf("▲ NEW OUTPUT - Line %d/%d", startLine+1, totalLines)) +
			"  " + styles.Muted.Render("[G] jump to latest")
	}

	if scrollOffset == 0 && !autoScrollEnabled {
		// At top
		return styles.Muted.Render(fmt.Sprintf("▲ TOP - Line 1/%d", totalLines)) +
			"  " + styles.Muted.Render("[j/↓] down  [G] bottom")
	}

	if autoScrollEnabled {
		// Auto-scrolling (at bottom)
		return styles.Secondary.Render(fmt.Sprintf("▼ FOLLOWING - Line %d/%d", endLine, totalLines)) +
			"  " + styles.Muted.Render("[k/↑] scroll up")
	}

	// Scrolled somewhere in the middle
	percent := 0
	if maxScroll > 0 {
		percent = scrollOffset * 100 / maxScroll
	}
	return styles.Muted.Render(fmt.Sprintf("Line %d-%d/%d (%d%%)", startLine+1, endLine, totalLines, percent)) +
		"  " + styles.Muted.Render("[j/k] scroll  [g/G] top/bottom")
}

// highlightSearchMatches highlights search matches in visible lines.
func (v *InstanceView) highlightSearchMatches(lines []string, startLine int, state RenderState) []string {
	if state.SearchRegex == nil {
		return lines
	}

	result := make([]string, len(lines))
	for i, line := range lines {
		lineNum := startLine + i
		isCurrentMatchLine := false

		// Check if this line contains the current match
		if len(state.SearchMatches) > 0 && state.SearchCurrent < len(state.SearchMatches) {
			if lineNum == state.SearchMatches[state.SearchCurrent] {
				isCurrentMatchLine = true
			}
		}

		// Find and highlight all matches in this line
		matches := state.SearchRegex.FindAllStringIndex(line, -1)
		if len(matches) == 0 {
			result[i] = line
			continue
		}

		var highlighted strings.Builder
		lastEnd := 0
		for j, match := range matches {
			// Add text before match
			highlighted.WriteString(line[lastEnd:match[0]])

			// Highlight the match
			matchText := line[match[0]:match[1]]
			if isCurrentMatchLine && j == 0 {
				// Current match gets special highlighting
				highlighted.WriteString(styles.SearchCurrentMatch.Render(matchText))
			} else {
				highlighted.WriteString(styles.SearchMatch.Render(matchText))
			}
			lastEnd = match[1]
		}
		// Add remaining text after last match
		highlighted.WriteString(line[lastEnd:])
		result[i] = highlighted.String()
	}

	return result
}

// RenderSearchBar renders the search input bar.
func (v *InstanceView) RenderSearchBar(state RenderState) string {
	var b strings.Builder

	// Search prompt
	b.WriteString(styles.SearchPrompt.Render("/"))
	b.WriteString(styles.SearchInput.Render(state.SearchPattern))

	if state.SearchMode {
		b.WriteString("█") // Cursor
	}

	// Match info
	if state.SearchPattern != "" {
		if len(state.SearchMatches) > 0 {
			info := fmt.Sprintf(" [%d/%d]", state.SearchCurrent+1, len(state.SearchMatches))
			b.WriteString(styles.SearchInfo.Render(info))
			b.WriteString(styles.Muted.Render("  n/N next/prev"))
		} else if !state.SearchMode {
			b.WriteString(styles.SearchInfo.Render(" No matches"))
		}
		if !state.SearchMode {
			b.WriteString(styles.Muted.Render("  Ctrl+/ clear"))
		}
	}

	return styles.SearchBar.Render(b.String())
}

// RenderWaitingState renders a waiting state indicator for instances
// that are waiting for user input or in a stuck state.
func (v *InstanceView) RenderWaitingState(status orchestrator.InstanceStatus) string {
	switch status {
	case orchestrator.StatusWaitingInput:
		return styles.Warning.Render("⏳ Waiting for user input...")
	case orchestrator.StatusStuck:
		return styles.Warning.Render("⚠ Instance appears stuck - no activity detected")
	case orchestrator.StatusTimeout:
		return styles.Error.Render("⏰ Instance timed out")
	case orchestrator.StatusPaused:
		return styles.Muted.Render("⏸ Instance paused")
	default:
		return ""
	}
}

// Helper functions

// truncateLines limits text to maxLines, adding ellipsis if truncated.
func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

// FormatDuration formats a duration for display.
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

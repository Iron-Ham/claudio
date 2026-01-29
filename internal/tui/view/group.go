package view

import (
	"fmt"
	"strings"

	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// GroupViewState holds the state needed for rendering the grouped sidebar view.
type GroupViewState struct {
	// CollapsedGroups tracks which group IDs are collapsed (hidden).
	CollapsedGroups map[string]bool

	// AutoExpandedGroups tracks groups that were automatically expanded when
	// navigating to an instance inside them. These groups should auto-collapse
	// when the user navigates away from them (unlike manually expanded groups).
	AutoExpandedGroups map[string]bool

	// LockedCollapsed tracks groups that are programmatically collapsed and should
	// NOT be auto-expanded during navigation. This is used for groups like the
	// "Planning Instances" sub-group in multiplan mode, where the instances should
	// remain hidden and not be navigable via tab/shift-tab.
	LockedCollapsed map[string]bool

	// SelectedGroupID is the ID of the currently focused group (for J/K navigation).
	// Empty string means no group is selected (instance navigation mode).
	SelectedGroupID string

	// SelectedInstanceIdx is the index of the currently selected instance.
	// This is the global index across all visible (non-collapsed) instances.
	SelectedInstanceIdx int
}

// NewGroupViewState creates a new GroupViewState with default values.
func NewGroupViewState() *GroupViewState {
	return &GroupViewState{
		CollapsedGroups:     make(map[string]bool),
		AutoExpandedGroups:  make(map[string]bool),
		LockedCollapsed:     make(map[string]bool),
		SelectedGroupID:     "",
		SelectedInstanceIdx: 0,
	}
}

// ToggleCollapse toggles the collapsed state of a group.
// When manually toggled, the group is no longer considered auto-expanded,
// and any locked-collapsed state is also cleared (user action takes precedence).
func (s *GroupViewState) ToggleCollapse(groupID string) {
	s.CollapsedGroups[groupID] = !s.CollapsedGroups[groupID]
	// Clear auto-expanded state since this is a manual toggle
	delete(s.AutoExpandedGroups, groupID)
	// Clear locked state - manual user action unlocks the group
	delete(s.LockedCollapsed, groupID)
}

// IsCollapsed returns whether a group is collapsed.
func (s *GroupViewState) IsCollapsed(groupID string) bool {
	return s.CollapsedGroups[groupID]
}

// AutoExpand expands a collapsed group and marks it as auto-expanded.
// Auto-expanded groups will automatically collapse when navigating away.
// Returns true if the group was expanded (was collapsed), false if already expanded
// or if the group is locked collapsed (cannot be auto-expanded).
func (s *GroupViewState) AutoExpand(groupID string) bool {
	if !s.CollapsedGroups[groupID] {
		return false // Already expanded
	}
	// Don't auto-expand locked collapsed groups (like multiplan planner sub-groups)
	if s.IsLockedCollapsed(groupID) {
		return false
	}
	s.CollapsedGroups[groupID] = false
	s.AutoExpandedGroups[groupID] = true
	return true
}

// AutoCollapse collapses a group if it was auto-expanded.
// Does nothing if the group was manually expanded or is already collapsed.
// Returns true if the group was collapsed.
func (s *GroupViewState) AutoCollapse(groupID string) bool {
	if !s.AutoExpandedGroups[groupID] {
		return false // Not auto-expanded, don't collapse
	}
	s.CollapsedGroups[groupID] = true
	delete(s.AutoExpandedGroups, groupID)
	return true
}

// IsAutoExpanded returns whether a group was auto-expanded.
func (s *GroupViewState) IsAutoExpanded(groupID string) bool {
	return s.AutoExpandedGroups[groupID]
}

// SetLockedCollapsed marks a group as locked collapsed. Locked collapsed groups
// cannot be auto-expanded during navigation - they remain collapsed until
// explicitly unlocked. This is used for groups like multiplan "Planning Instances"
// sub-groups where the instances should not be navigable via tab/shift-tab.
func (s *GroupViewState) SetLockedCollapsed(groupID string, locked bool) {
	if s.LockedCollapsed == nil {
		s.LockedCollapsed = make(map[string]bool)
	}
	if locked {
		s.LockedCollapsed[groupID] = true
		// Ensure the group is also collapsed
		s.CollapsedGroups[groupID] = true
	} else {
		delete(s.LockedCollapsed, groupID)
	}
}

// IsLockedCollapsed returns whether a group is locked collapsed.
// Locked collapsed groups cannot be auto-expanded during navigation.
func (s *GroupViewState) IsLockedCollapsed(groupID string) bool {
	if s.LockedCollapsed == nil {
		return false
	}
	return s.LockedCollapsed[groupID]
}

// GroupProgress holds the completion progress for a group.
type GroupProgress struct {
	Completed int
	Total     int
}

// CalculateGroupProgress calculates the completion progress for a group.
// A group's progress includes all instances in the group and its sub-groups.
// For orchestrated session types (tripleshot, adversarial, ralph), it uses
// workflow-specific completion tracking rather than just Instance.Status.
func CalculateGroupProgress(group *orchestrator.InstanceGroup, session *orchestrator.Session) GroupProgress {
	if group == nil || session == nil {
		return GroupProgress{}
	}

	// Check if this is a workflow-specific group that tracks completion differently
	sessionType := orchestrator.GetSessionType(group)
	switch sessionType {
	case orchestrator.SessionTypeTripleShot:
		return calculateTripleShotProgress(group, session)
	case orchestrator.SessionTypeAdversarial:
		return calculateAdversarialProgress(group, session)
	case orchestrator.SessionTypeRalph:
		return calculateRalphProgress(group, session)
	}

	// Default: use instance status
	progress := GroupProgress{}
	calculateGroupProgressRecursive(group, session, &progress)
	return progress
}

func calculateGroupProgressRecursive(group *orchestrator.InstanceGroup, session *orchestrator.Session, progress *GroupProgress) {
	for _, instID := range group.Instances {
		progress.Total++
		inst := session.GetInstance(instID)
		if inst != nil && isInstanceCompleted(inst.Status) {
			progress.Completed++
		}
	}

	for _, subGroup := range group.SubGroups {
		calculateGroupProgressRecursive(subGroup, session, progress)
	}
}

// isInstanceCompleted returns true if the instance status represents a terminal state.
// This includes successful completion and various failure/error states.
func isInstanceCompleted(status orchestrator.InstanceStatus) bool {
	switch status {
	case orchestrator.StatusCompleted,
		orchestrator.StatusError,
		orchestrator.StatusStuck,
		orchestrator.StatusTimeout,
		orchestrator.StatusInterrupted:
		return true
	default:
		return false
	}
}

// findTripleShotSession finds the tripleshot session for a group.
func findTripleShotSession(groupID string, session *orchestrator.Session) *tripleshot.Session {
	for _, s := range session.TripleShots {
		if s.GroupID == groupID {
			return s
		}
	}
	return nil
}

// findAdversarialSession finds the adversarial session for a group.
func findAdversarialSession(groupID string, session *orchestrator.Session) *adversarial.Session {
	for _, s := range session.AdversarialSessions {
		if s.GroupID == groupID {
			return s
		}
	}
	return nil
}

// findRalphSession finds the ralph session for a group.
func findRalphSession(groupID string, session *orchestrator.Session) *ralph.Session {
	for _, s := range session.RalphSessions {
		if s.GroupID == groupID {
			return s
		}
	}
	return nil
}

// defaultGroupProgress returns the standard instance-based progress for a group.
func defaultGroupProgress(group *orchestrator.InstanceGroup, session *orchestrator.Session) GroupProgress {
	progress := GroupProgress{}
	calculateGroupProgressRecursive(group, session, &progress)
	return progress
}

// calculateTripleShotProgress calculates progress for a tripleshot group.
// It uses the tripleshot session's Attempt.Status rather than Instance.Status
// to accurately reflect work completion before instances fully exit.
func calculateTripleShotProgress(group *orchestrator.InstanceGroup, session *orchestrator.Session) GroupProgress {
	ts := findTripleShotSession(group.ID, session)
	if ts == nil {
		return defaultGroupProgress(group, session)
	}

	progress := GroupProgress{}

	// Track which instance IDs we've already counted to avoid double-counting
	countedIDs := make(map[string]bool)

	// Count attempts by their workflow status
	for _, attempt := range ts.Attempts {
		if attempt.InstanceID != "" {
			progress.Total++
			countedIDs[attempt.InstanceID] = true
			if isTripleShotAttemptCompleted(attempt.Status) {
				progress.Completed++
			}
		}
		// Count adversarial reviewers - they use instance status since they don't have
		// separate workflow tracking like attempts do. Only count if the instance exists.
		if attempt.ReviewerID != "" {
			inst := session.GetInstance(attempt.ReviewerID)
			if inst != nil {
				progress.Total++
				countedIDs[attempt.ReviewerID] = true
				if isInstanceCompleted(inst.Status) {
					progress.Completed++
				}
			}
		}
	}

	// Count judge if present
	if ts.JudgeID != "" {
		progress.Total++
		countedIDs[ts.JudgeID] = true
		if ts.Phase == tripleshot.PhaseComplete || ts.Evaluation != nil {
			progress.Completed++
		}
	}

	// Count any additional instances in the group not accounted for above.
	// Only count instances that actually exist to avoid unreachable progress.
	for _, instID := range group.Instances {
		if countedIDs[instID] {
			continue // Already counted
		}
		inst := session.GetInstance(instID)
		if inst == nil {
			continue // Instance doesn't exist, don't count it
		}
		progress.Total++
		if isInstanceCompleted(inst.Status) {
			progress.Completed++
		}
	}

	return progress
}

// isTripleShotAttemptCompleted returns true if the attempt has reached a terminal state.
func isTripleShotAttemptCompleted(status tripleshot.AttemptStatus) bool {
	switch status {
	case tripleshot.AttemptStatusCompleted, tripleshot.AttemptStatusFailed:
		return true
	default:
		return false
	}
}

// calculateAdversarialProgress calculates progress for an adversarial group.
// Adversarial sessions track progress via rounds completed, not instance status.
func calculateAdversarialProgress(group *orchestrator.InstanceGroup, session *orchestrator.Session) GroupProgress {
	as := findAdversarialSession(group.ID, session)
	if as == nil {
		return defaultGroupProgress(group, session)
	}

	progress := GroupProgress{}

	// Count all instances in the group (including sub-groups)
	allInstIDs := group.AllInstanceIDs()
	progress.Total = len(allInstIDs)

	// For adversarial, completion is determined by session phase
	switch as.Phase {
	case adversarial.PhaseComplete, adversarial.PhaseApproved:
		// All work is done
		progress.Completed = progress.Total
	case adversarial.PhaseFailed, adversarial.PhaseStuck:
		// Session ended but not successfully - count any completed instances
		for _, instID := range allInstIDs {
			inst := session.GetInstance(instID)
			if inst != nil && isInstanceCompleted(inst.Status) {
				progress.Completed++
			}
		}
	default:
		// Session in progress - count completed rounds' instances.
		// Each round has an implementer and reviewer.
		completedRounds := 0
		for _, round := range as.History {
			if round.Review != nil {
				// A round with a review is complete (whether approved or not)
				completedRounds++
			}
		}
		// 2 instances per completed round (implementer + reviewer)
		progress.Completed = completedRounds * 2

		// Check if current round's implementer is done (increment ready but not yet reviewed)
		if len(as.History) > 0 {
			currentRound := as.History[len(as.History)-1]
			if currentRound.Increment != nil && currentRound.Review == nil {
				progress.Completed++
			}
		}

		// Cap at total to handle edge cases
		if progress.Completed > progress.Total {
			progress.Completed = progress.Total
		}
	}

	return progress
}

// calculateRalphProgress calculates progress for a ralph group.
// Ralph sessions iterate with a single instance, tracking progress via iteration count.
func calculateRalphProgress(group *orchestrator.InstanceGroup, session *orchestrator.Session) GroupProgress {
	rs := findRalphSession(group.ID, session)
	if rs == nil {
		return defaultGroupProgress(group, session)
	}

	progress := GroupProgress{}

	// Ralph creates a new instance each iteration
	allInstIDs := group.AllInstanceIDs()
	progress.Total = len(allInstIDs)

	switch rs.Phase {
	case ralph.PhaseComplete, ralph.PhaseMaxIterations, ralph.PhaseCancelled, ralph.PhaseError:
		// Session is done - all instances are complete
		progress.Completed = progress.Total
	default:
		// Session in progress - all but current instance are complete
		if progress.Total > 0 {
			progress.Completed = progress.Total - 1
		}
	}

	return progress
}

// PhaseIndicator returns the visual indicator for a group phase.
func PhaseIndicator(phase orchestrator.GroupPhase) string {
	switch phase {
	case orchestrator.GroupPhaseCompleted:
		return "\u2713" // checkmark
	case orchestrator.GroupPhaseExecuting:
		return "\u25cf" // filled circle
	case orchestrator.GroupPhaseFailed:
		return "\u2717" // X mark
	default:
		return " "
	}
}

// PhaseColor returns the lipgloss color for a group phase.
func PhaseColor(phase orchestrator.GroupPhase) lipgloss.Color {
	switch phase {
	case orchestrator.GroupPhasePending:
		return styles.MutedColor
	case orchestrator.GroupPhaseExecuting:
		return styles.YellowColor
	case orchestrator.GroupPhaseCompleted:
		return styles.GreenColor
	case orchestrator.GroupPhaseFailed:
		return styles.RedColor
	default:
		return styles.MutedColor
	}
}

// RenderGroupHeader renders the header line for a group.
// Example: "▾ ⚡ Refactor auth [2/5] ●"
// For multi-line wrapped headers, use RenderGroupHeaderWrapped instead.
func RenderGroupHeader(group *orchestrator.InstanceGroup, progress GroupProgress, collapsed bool, isSelected bool, width int) string {
	lines := RenderGroupHeaderWrapped(group, progress, collapsed, isSelected, width)
	return strings.Join(lines, "\n")
}

// RenderGroupHeaderItem renders a group header from a GroupHeaderItem.
// This version supports RoundInfo display for adversarial groups.
// Example for adversarial: "▾ ⚔️ Refactor auth (Round 3) [2/2] ●"
func RenderGroupHeaderItem(item GroupHeaderItem, width int) string {
	lines := RenderGroupHeaderItemWrapped(item, width)
	return strings.Join(lines, "\n")
}

// RenderGroupHeaderItemWrapped renders a group header with round info support.
// For adversarial groups, appends " (Round N)" to the group name.
func RenderGroupHeaderItemWrapped(item GroupHeaderItem, width int) []string {
	group := item.Group
	if group == nil {
		return []string{}
	}

	// Build display name with optional round info
	displayName := group.Name
	if item.RoundInfo != "" {
		displayName = fmt.Sprintf("%s (%s)", group.Name, item.RoundInfo)
	}

	// Create a temporary copy of the group with modified name for rendering
	// (we don't want to modify the original group)
	tempGroup := *group
	tempGroup.Name = displayName

	return RenderGroupHeaderWrapped(&tempGroup, item.Progress, item.Collapsed, item.IsSelected, width)
}

// RenderGroupHeaderWrapped renders a group header with word-wrapped name support.
// Returns a slice of lines where the first line contains the collapse indicator,
// icon, and start of name, and subsequent lines contain wrapped name continuation.
// The progress indicator and phase indicator are placed on the first line.
// Example first line: "▾ ⚡ Refactor auth module [2/5] ●"
// Example continuation: "     for better security"
func RenderGroupHeaderWrapped(group *orchestrator.InstanceGroup, progress GroupProgress, collapsed bool, isSelected bool, width int) []string {
	// Collapse indicator
	collapseChar := styles.IconGroupExpand // down-pointing triangle (expanded)
	if collapsed {
		collapseChar = styles.IconGroupCollapse // right-pointing triangle (collapsed)
	}

	// Session type icon - use GetSessionType to get the typed SessionType with methods
	sessionType := orchestrator.GetSessionType(group)
	sessionIcon := sessionType.Icon()
	hasIcon := group.SessionType != "" && sessionType != orchestrator.SessionTypeStandard

	// Phase styling
	phaseColor := PhaseColor(group.Phase)
	phaseIndicator := PhaseIndicator(group.Phase)

	// Session type color (use for the icon)
	sessionColor := styles.SessionTypeColor(group.SessionType)

	// Build the header components
	progressStr := fmt.Sprintf("[%d/%d]", progress.Completed, progress.Total)

	// Calculate prefix length for indentation of continuation lines
	// Prefix: "V " or "V I " depending on whether there's an icon
	prefixLen := 2 // collapse + space
	if hasIcon {
		prefixLen += len([]rune(sessionIcon)) + 1 // icon + space
	}

	// Calculate how much space we have for the name on the first line
	// Format: "V I <name> [x/y] P" where V=collapse, I=session icon, P=phase indicator
	// Width deductions:
	// - 2 chars: sidebar Padding(1, 1) horizontal
	// - 2 chars: safety buffer for border/edge alignment
	suffixLen := 1 + len(progressStr) + 1 + 1 // space + progress + space + indicator
	maxFirstLineNameLen := width - prefixLen - suffixLen - 4

	// Calculate max name length for continuation lines (full width minus indent)
	// Same deductions: sidebar padding (2) + buffer (2)
	maxContinuationNameLen := width - prefixLen - 4

	// Wrap the group name with different widths for first line vs continuation
	nameLines := wrapGroupNameWithWidths(group.Name, maxFirstLineNameLen, maxContinuationNameLen)

	// Apply styling
	var headerStyle lipgloss.Style
	if isSelected {
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.TextColor).
			Background(phaseColor)
	} else {
		headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(phaseColor)
	}

	// Build styles
	collapseStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	iconStyle := lipgloss.NewStyle().Foreground(sessionColor)
	progressStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	indicatorStyle := lipgloss.NewStyle().Foreground(phaseColor)

	var result []string

	// Build first line
	var firstLine strings.Builder
	firstLine.WriteString(collapseStyle.Render(collapseChar))
	firstLine.WriteString(" ")
	if hasIcon {
		firstLine.WriteString(iconStyle.Render(sessionIcon))
		firstLine.WriteString(" ")
	}
	if len(nameLines) > 0 {
		firstLine.WriteString(headerStyle.Render(nameLines[0]))
	}
	firstLine.WriteString(" ")
	firstLine.WriteString(progressStyle.Render(progressStr))
	firstLine.WriteString(" ")
	firstLine.WriteString(indicatorStyle.Render(phaseIndicator))
	result = append(result, firstLine.String())

	// Build continuation lines (if any)
	indent := strings.Repeat(" ", prefixLen)
	for i := 1; i < len(nameLines); i++ {
		result = append(result, indent+headerStyle.Render(nameLines[i]))
	}

	return result
}

// truncateGroupName truncates a group name to fit within maxLen.
func truncateGroupName(name string, maxLen int) string {
	if maxLen <= 3 {
		return name
	}
	runes := []rune(name)
	if len(runes) <= maxLen {
		return name
	}
	return string(runes[:maxLen-3]) + "..."
}

// wrapGroupName wraps a group name across multiple lines to fit within maxLen per line.
// Returns a slice of lines. Word boundaries are preferred for wrapping.
func wrapGroupName(name string, maxLen int) []string {
	return wrapGroupNameWithWidths(name, maxLen, maxLen)
}

// wrapGroupNameWithWidths wraps a group name with different max lengths for first vs subsequent lines.
// This is useful when the first line has additional elements (icons, progress) taking up space.
func wrapGroupNameWithWidths(name string, firstLineMax, continuationMax int) []string {
	if firstLineMax <= 0 {
		return []string{name}
	}

	runes := []rune(name)
	if len(runes) <= firstLineMax {
		return []string{name}
	}

	var lines []string
	var currentLine []rune
	isFirstLine := true

	// Split into words for better wrapping
	words := strings.Fields(name)
	if len(words) == 0 {
		return []string{name}
	}

	currentMax := firstLineMax

	for _, word := range words {
		wordRunes := []rune(word)

		// If adding this word would exceed the line limit
		if len(currentLine) > 0 && len(currentLine)+1+len(wordRunes) > currentMax {
			// Finish current line
			lines = append(lines, string(currentLine))
			currentLine = wordRunes
			if isFirstLine {
				isFirstLine = false
				currentMax = continuationMax
			}
		} else if len(currentLine) == 0 {
			// Start of line
			currentLine = wordRunes
		} else {
			// Add word with space
			currentLine = append(currentLine, ' ')
			currentLine = append(currentLine, wordRunes...)
		}

		// Handle case where a single word is longer than currentMax
		for len(currentLine) > currentMax {
			lines = append(lines, string(currentLine[:currentMax]))
			currentLine = currentLine[currentMax:]
			if isFirstLine {
				isFirstLine = false
				currentMax = continuationMax
			}
		}
	}

	// Don't forget the last line
	if len(currentLine) > 0 {
		lines = append(lines, string(currentLine))
	}

	return lines
}

// GroupedInstance represents an instance within a group, with rendering context.
type GroupedInstance struct {
	Instance    *orchestrator.Instance
	GroupID     string
	Depth       int  // 0 = top-level group instance, 1 = sub-group instance, etc.
	IsLast      bool // Is this the last instance in its group/sub-group?
	GlobalIdx   int  // Global index for selection (across all visible instances)
	AbsoluteIdx int  // Absolute index in session.Instances for stable display numbering
}

// FlattenGroupsForDisplay flattens groups into a list of renderable items.
// This respects collapsed state - collapsed groups won't have their instances expanded.
// Returns both the group headers and the instances in display order.
// Ungrouped instances are displayed first, followed by groups.
func FlattenGroupsForDisplay(session *orchestrator.Session, state *GroupViewState) []any {
	if session == nil {
		return nil
	}

	var items []any
	globalIdx := 0

	// Build grouped sidebar data to identify ungrouped instances
	data := BuildGroupedSidebarData(session)

	// Add ungrouped instances first (they don't have a group header)
	for i, inst := range data.UngroupedInstances {
		isLast := i == len(data.UngroupedInstances)-1 && !session.HasGroups()
		items = append(items, GroupedInstance{
			Instance:    inst,
			GroupID:     "", // No group
			Depth:       -1, // Special depth to indicate ungrouped (no tree connector)
			IsLast:      isLast,
			GlobalIdx:   globalIdx,
			AbsoluteIdx: findInstanceIndex(session, inst.ID),
		})
		globalIdx++
	}

	// Add groups (thread-safe)
	for _, group := range session.GetGroups() {
		items = append(items, flattenGroupRecursive(group, session, state, 0, &globalIdx)...)
	}

	return items
}

// GroupHeaderItem represents a group header in the flattened display list.
type GroupHeaderItem struct {
	Group      *orchestrator.InstanceGroup
	Progress   GroupProgress
	Collapsed  bool
	IsSelected bool
	Depth      int
	RoundInfo  string // Optional round info for adversarial groups, e.g., "Round 3"
}

func flattenGroupRecursive(group *orchestrator.InstanceGroup, session *orchestrator.Session, state *GroupViewState, depth int, globalIdx *int) []any {
	var items []any

	// Add group header
	progress := CalculateGroupProgress(group, session)
	collapsed := state.IsCollapsed(group.ID)
	isSelected := state.SelectedGroupID == group.ID

	items = append(items, GroupHeaderItem{
		Group:      group,
		Progress:   progress,
		Collapsed:  collapsed,
		IsSelected: isSelected,
		Depth:      depth,
	})

	// If collapsed, don't add instances or sub-groups
	if collapsed {
		return items
	}

	// Add instances from this group
	for i, instID := range group.Instances {
		inst := session.GetInstance(instID)
		if inst == nil {
			continue
		}
		isLast := i == len(group.Instances)-1 && len(group.SubGroups) == 0
		items = append(items, GroupedInstance{
			Instance:    inst,
			GroupID:     group.ID,
			Depth:       depth,
			IsLast:      isLast,
			GlobalIdx:   *globalIdx,
			AbsoluteIdx: findInstanceIndex(session, instID),
		})
		*globalIdx++
	}

	// Add sub-groups
	for _, subGroup := range group.SubGroups {
		items = append(items, flattenGroupRecursive(subGroup, session, state, depth+1, globalIdx)...)
	}

	return items
}

// RenderGroupedInstance renders a single instance within a group.
func RenderGroupedInstance(gi GroupedInstance, isActiveInstance bool, hasConflict bool, width int) string {
	inst := gi.Instance

	// For ungrouped instances (Depth == -1), render without tree connector
	var indent, connector string
	if gi.Depth < 0 {
		// Ungrouped instance - no tree connector, minimal indent
		indent = ""
		connector = ""
	} else {
		// Calculate indentation based on depth
		// depth 0 = 2 spaces, depth 1 = 4 spaces, etc.
		indent = strings.Repeat("  ", gi.Depth+1)

		// Tree connector
		connector = "\u2502" // vertical line |
		if gi.IsLast {
			connector = "\u2514" // corner L
		}
	}

	// Status color for the instance
	statusColor := styles.StatusColor(string(inst.Status))

	// Instance number - use AbsoluteIdx for stable numbering, fall back to GlobalIdx
	displayIdx := gi.AbsoluteIdx
	if displayIdx < 0 {
		displayIdx = gi.GlobalIdx
	}
	idxStr := fmt.Sprintf("[%d]", displayIdx+1)

	// Calculate overhead for width: indent + connector + space + idx + space
	overhead := len(indent) + len(idxStr) + 1
	if connector != "" {
		overhead += len([]rune(connector)) + 1
	}
	maxNameLen := width - overhead - 4

	displayName := truncate(inst.EffectiveName(), maxNameLen)

	// Name style: active (selected) vs inactive
	nameStyle := styles.SidebarItem.Foreground(styles.MutedColor)
	if isActiveInstance {
		nameStyle = styles.SidebarItemActive
	}

	// Build the first line: indent connector [N] name
	var b strings.Builder
	b.WriteString(indent)
	if connector != "" {
		b.WriteString(styles.Muted.Render(connector))
		b.WriteString(" ")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(styles.MutedColor).Render(idxStr))
	b.WriteString(" ")
	b.WriteString(nameStyle.Render(displayName))
	b.WriteString("\n")

	// Second line: status aligned under the name, with context info
	b.WriteString(strings.Repeat(" ", overhead))
	statusStr := lipgloss.NewStyle().Foreground(statusColor).Render("●" + instanceStatusAbbrev(inst.Status))
	b.WriteString(statusStr)

	// Add context info (duration, cost, files) for more informative display
	contextParts := []string{}

	// Add duration for running/completed instances
	if inst.Metrics != nil && inst.Metrics.Duration() > 0 {
		contextParts = append(contextParts, FormatDurationCompact(inst.Metrics.Duration()))
	}

	// Add cost if significant
	if inst.Metrics != nil && inst.Metrics.Cost > 0.01 {
		contextParts = append(contextParts, instmetrics.FormatCost(inst.Metrics.Cost))
	}

	// Add files modified count (abbreviated for compact display)
	if len(inst.FilesModified) > 0 {
		contextParts = append(contextParts, fmt.Sprintf("%df", len(inst.FilesModified)))
	}

	// Render context info if we have any and there's space
	if len(contextParts) > 0 {
		contextStr := strings.Join(contextParts, "|")
		// Calculate available space
		statusLen := len("●") + len(instanceStatusAbbrev(inst.Status))
		available := width - overhead - statusLen - 2
		if available > 5 && len(contextStr) <= available {
			b.WriteString(" ")
			b.WriteString(styles.Muted.Render(contextStr))
		}
	}

	return b.String()
}

// instanceStatusAbbrev returns a short status abbreviation.
func instanceStatusAbbrev(status orchestrator.InstanceStatus) string {
	switch status {
	case orchestrator.StatusPending:
		return "PEND"
	case orchestrator.StatusPreparing:
		return "PREP"
	case orchestrator.StatusWorking:
		return "WORK"
	case orchestrator.StatusWaitingInput:
		return "WAIT"
	case orchestrator.StatusPaused:
		return "PAUS"
	case orchestrator.StatusCompleted:
		return "DONE"
	case orchestrator.StatusError:
		return "ERR!"
	case orchestrator.StatusCreatingPR:
		return "PR.."
	case orchestrator.StatusStuck:
		return "STUK"
	case orchestrator.StatusTimeout:
		return "TIME"
	case orchestrator.StatusInterrupted:
		return "INT!"
	default:
		return "????"
	}
}

// GetVisibleInstanceCount returns the count of visible (non-collapsed) instances.
// This includes both ungrouped instances and instances in expanded groups.
func GetVisibleInstanceCount(session *orchestrator.Session, state *GroupViewState) int {
	if session == nil {
		return 0
	}

	if !session.HasGroups() {
		return len(session.Instances) // Fall back to flat list
	}

	// Count ungrouped instances
	data := BuildGroupedSidebarData(session)
	count := len(data.UngroupedInstances)

	// Count instances in groups (respecting collapsed state, thread-safe)
	for _, group := range session.GetGroups() {
		count += countVisibleInstancesRecursive(group, session, state)
	}
	return count
}

func countVisibleInstancesRecursive(group *orchestrator.InstanceGroup, session *orchestrator.Session, state *GroupViewState) int {
	if state.IsCollapsed(group.ID) {
		return 0
	}

	count := len(group.Instances)
	for _, subGroup := range group.SubGroups {
		count += countVisibleInstancesRecursive(subGroup, session, state)
	}
	return count
}

// GetGroupIDs returns all group IDs in display order (for J/K navigation).
// This function is thread-safe with respect to session.Groups access.
// Note: This returns ALL groups including hidden subgroups. For navigation
// that respects collapse state, use GetVisibleGroupIDs instead.
func GetGroupIDs(session *orchestrator.Session) []string {
	if session == nil || !session.HasGroups() {
		return nil
	}

	var ids []string
	for _, group := range session.GetGroups() {
		ids = append(ids, getGroupIDsRecursive(group)...)
	}
	return ids
}

func getGroupIDsRecursive(group *orchestrator.InstanceGroup) []string {
	ids := []string{group.ID}
	for _, subGroup := range group.SubGroups {
		ids = append(ids, getGroupIDsRecursive(subGroup)...)
	}
	return ids
}

// GetVisibleGroupIDs returns group IDs that are currently visible in the UI.
// A group is visible if all of its ancestor groups are expanded (not collapsed).
// This respects the collapse state and should be used for navigation.
func GetVisibleGroupIDs(session *orchestrator.Session, state *GroupViewState) []string {
	if session == nil || !session.HasGroups() || state == nil {
		return nil
	}

	var ids []string
	for _, group := range session.GetGroups() {
		ids = append(ids, getVisibleGroupIDsRecursive(group, state)...)
	}
	return ids
}

func getVisibleGroupIDsRecursive(group *orchestrator.InstanceGroup, state *GroupViewState) []string {
	// Always include the current group (it's visible if we got here)
	ids := []string{group.ID}

	// Only include subgroups if this group is expanded
	if !state.IsCollapsed(group.ID) {
		for _, subGroup := range group.SubGroups {
			ids = append(ids, getVisibleGroupIDsRecursive(subGroup, state)...)
		}
	}

	return ids
}

// FindInstanceByGlobalIndex finds the instance at the given global index.
// Returns nil if the index is out of bounds.
// The global index includes ungrouped instances first, then grouped instances.
func FindInstanceByGlobalIndex(session *orchestrator.Session, state *GroupViewState, targetIdx int) *orchestrator.Instance {
	if session == nil || targetIdx < 0 {
		return nil
	}

	// If no groups, use flat list
	if !session.HasGroups() {
		if targetIdx < len(session.Instances) {
			return session.Instances[targetIdx]
		}
		return nil
	}

	// Check ungrouped instances first
	data := BuildGroupedSidebarData(session)
	if targetIdx < len(data.UngroupedInstances) {
		return data.UngroupedInstances[targetIdx]
	}

	// Adjust index for grouped instances (thread-safe)
	currentIdx := len(data.UngroupedInstances)
	for _, group := range session.GetGroups() {
		inst := findInstanceInGroup(group, session, state, targetIdx, &currentIdx)
		if inst != nil {
			return inst
		}
	}
	return nil
}

func findInstanceInGroup(group *orchestrator.InstanceGroup, session *orchestrator.Session, state *GroupViewState, targetIdx int, currentIdx *int) *orchestrator.Instance {
	if state.IsCollapsed(group.ID) {
		return nil
	}

	for _, instID := range group.Instances {
		if *currentIdx == targetIdx {
			return session.GetInstance(instID)
		}
		*currentIdx++
	}

	for _, subGroup := range group.SubGroups {
		inst := findInstanceInGroup(subGroup, session, state, targetIdx, currentIdx)
		if inst != nil {
			return inst
		}
	}
	return nil
}

// findInstanceIndex returns the index of an instance in session.Instances by ID.
// Returns -1 if not found or if session is nil.
func findInstanceIndex(session *orchestrator.Session, instID string) int {
	if session == nil {
		return -1
	}
	for i, inst := range session.Instances {
		if inst.ID == instID {
			return i
		}
	}
	return -1
}

// FindGroupContainingInstance finds the group (and all ancestor groups) that contains
// the given instance ID. Returns a slice of group IDs from outermost to innermost
// (i.e., top-level group first, immediate parent last).
// Returns nil if the instance is not in any group or is ungrouped.
func FindGroupContainingInstance(session *orchestrator.Session, instanceID string) []string {
	if session == nil || !session.HasGroups() {
		return nil
	}

	for _, group := range session.GetGroups() {
		if path := findGroupContainingInstanceRecursive(group, instanceID); path != nil {
			return path
		}
	}
	return nil
}

func findGroupContainingInstanceRecursive(group *orchestrator.InstanceGroup, instanceID string) []string {
	// Check if this group directly contains the instance
	for _, instID := range group.Instances {
		if instID == instanceID {
			return []string{group.ID}
		}
	}

	// Check sub-groups
	for _, subGroup := range group.SubGroups {
		if path := findGroupContainingInstanceRecursive(subGroup, instanceID); path != nil {
			// Prepend this group's ID to the path
			return append([]string{group.ID}, path...)
		}
	}

	return nil
}

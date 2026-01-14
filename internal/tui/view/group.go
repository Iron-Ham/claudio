package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// GroupViewState holds the state needed for rendering the grouped sidebar view.
type GroupViewState struct {
	// CollapsedGroups tracks which group IDs are collapsed (hidden).
	CollapsedGroups map[string]bool

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
		SelectedGroupID:     "",
		SelectedInstanceIdx: 0,
	}
}

// ToggleCollapse toggles the collapsed state of a group.
func (s *GroupViewState) ToggleCollapse(groupID string) {
	s.CollapsedGroups[groupID] = !s.CollapsedGroups[groupID]
}

// IsCollapsed returns whether a group is collapsed.
func (s *GroupViewState) IsCollapsed(groupID string) bool {
	return s.CollapsedGroups[groupID]
}

// GroupProgress holds the completion progress for a group.
type GroupProgress struct {
	Completed int
	Total     int
}

// CalculateGroupProgress calculates the completion progress for a group.
// A group's progress includes all instances in the group and its sub-groups.
func CalculateGroupProgress(group *orchestrator.InstanceGroup, session *orchestrator.Session) GroupProgress {
	progress := GroupProgress{}
	calculateGroupProgressRecursive(group, session, &progress)
	return progress
}

func calculateGroupProgressRecursive(group *orchestrator.InstanceGroup, session *orchestrator.Session, progress *GroupProgress) {
	for _, instID := range group.Instances {
		progress.Total++
		inst := session.GetInstance(instID)
		if inst != nil && inst.Status == orchestrator.StatusCompleted {
			progress.Completed++
		}
	}

	for _, subGroup := range group.SubGroups {
		calculateGroupProgressRecursive(subGroup, session, progress)
	}
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
func RenderGroupHeader(group *orchestrator.InstanceGroup, progress GroupProgress, collapsed bool, isSelected bool, width int) string {
	// Collapse indicator
	collapseChar := styles.IconGroupExpand // down-pointing triangle (expanded)
	if collapsed {
		collapseChar = styles.IconGroupCollapse // right-pointing triangle (collapsed)
	}

	// Session type icon
	sessionIcon := group.SessionType.Icon()

	// Phase styling
	phaseColor := PhaseColor(group.Phase)
	phaseIndicator := PhaseIndicator(group.Phase)

	// Session type color (use for the icon)
	sessionColor := styles.SessionTypeColor(string(group.SessionType))

	// Build the header components
	progressStr := fmt.Sprintf("[%d/%d]", progress.Completed, progress.Total)

	// Calculate how much space we have for the name
	// Format: "V I <name> [x/y] P" where V=collapse, I=session icon, P=phase indicator
	// overhead: collapse(1) + space(1) + icon(1-2) + space(1) + progress(varies) + space(1) + indicator(1)
	iconLen := len([]rune(sessionIcon))
	overhead := 1 + 1 + iconLen + 1 + len(progressStr) + 1 + 1
	maxNameLen := width - overhead - 4 // padding

	displayName := truncateGroupName(group.Name, maxNameLen)

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

	// Build the line
	collapseStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	iconStyle := lipgloss.NewStyle().Foreground(sessionColor)
	progressStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	indicatorStyle := lipgloss.NewStyle().Foreground(phaseColor)

	var b strings.Builder
	b.WriteString(collapseStyle.Render(collapseChar))
	b.WriteString(" ")
	// Only show session icon if not standard (standard instances don't need an icon prefix)
	if group.SessionType != "" && group.SessionType != orchestrator.SessionTypeStandard {
		b.WriteString(iconStyle.Render(sessionIcon))
		b.WriteString(" ")
	}
	b.WriteString(headerStyle.Render(displayName))
	b.WriteString(" ")
	b.WriteString(progressStyle.Render(progressStr))
	b.WriteString(" ")
	b.WriteString(indicatorStyle.Render(phaseIndicator))

	return b.String()
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
func FlattenGroupsForDisplay(session *orchestrator.Session, state *GroupViewState) []any {
	if session == nil || len(session.Groups) == 0 {
		return nil
	}

	var items []any
	globalIdx := 0

	for _, group := range session.Groups {
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

	// Calculate indentation based on depth
	// depth 0 = 2 spaces, depth 1 = 4 spaces, etc.
	indent := strings.Repeat("  ", gi.Depth+1)

	// Tree connector
	connector := "\u2502" // vertical line |
	if gi.IsLast {
		connector = "\u2514" // corner L
	}

	// Status indicator
	statusColor := styles.StatusColor(string(inst.Status))
	dot := lipgloss.NewStyle().Foreground(statusColor).Render("\u25cf")

	// Instance number (for identification) - use AbsoluteIdx for stable numbering
	// Fall back to GlobalIdx if AbsoluteIdx is invalid (should not happen in practice)
	displayIdx := gi.AbsoluteIdx
	if displayIdx < 0 {
		displayIdx = gi.GlobalIdx
	}
	idxStr := fmt.Sprintf("[%d]", displayIdx+1)
	idxStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	// Status abbreviation
	statusAbbrev := instanceStatusAbbrev(inst.Status)

	// Calculate available width for name
	// Format: "indent connector [N] name STAT"
	overhead := len(indent) + 2 + 1 + len(idxStr) + 1 + 1 + 4 // indent + connector + space + idx + space + dot + space + status(4)
	maxNameLen := width - overhead - 4                        // padding

	displayName := truncate(inst.EffectiveName(), maxNameLen)

	// Choose style
	var nameStyle lipgloss.Style
	if isActiveInstance {
		if hasConflict || inst.Status == orchestrator.StatusWaitingInput {
			nameStyle = styles.SidebarItemInputNeeded
		} else {
			nameStyle = styles.SidebarItemActive
		}
	} else {
		nameStyle = styles.SidebarItem
		if hasConflict || inst.Status == orchestrator.StatusWaitingInput {
			nameStyle = nameStyle.Foreground(styles.WarningColor)
		} else {
			nameStyle = nameStyle.Foreground(styles.MutedColor)
		}
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor)

	var b strings.Builder
	b.WriteString(indent)
	b.WriteString(lipgloss.NewStyle().Foreground(styles.MutedColor).Render(connector))
	b.WriteString(" ")
	b.WriteString(idxStyle.Render(idxStr))
	b.WriteString(" ")
	b.WriteString(nameStyle.Render(displayName))
	b.WriteString(" ")
	b.WriteString(dot)
	b.WriteString(statusStyle.Render(statusAbbrev))

	return b.String()
}

// instanceStatusAbbrev returns a short status abbreviation.
func instanceStatusAbbrev(status orchestrator.InstanceStatus) string {
	switch status {
	case orchestrator.StatusPending:
		return "PEND"
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
func GetVisibleInstanceCount(session *orchestrator.Session, state *GroupViewState) int {
	if session == nil || len(session.Groups) == 0 {
		return len(session.Instances) // Fall back to flat list
	}

	count := 0
	for _, group := range session.Groups {
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
func GetGroupIDs(session *orchestrator.Session) []string {
	if session == nil || len(session.Groups) == 0 {
		return nil
	}

	var ids []string
	for _, group := range session.Groups {
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

// FindInstanceByGlobalIndex finds the instance at the given global index.
// Returns nil if the index is out of bounds.
func FindInstanceByGlobalIndex(session *orchestrator.Session, state *GroupViewState, targetIdx int) *orchestrator.Instance {
	if session == nil {
		return nil
	}

	// If no groups, use flat list
	if len(session.Groups) == 0 {
		if targetIdx >= 0 && targetIdx < len(session.Instances) {
			return session.Instances[targetIdx]
		}
		return nil
	}

	currentIdx := 0
	for _, group := range session.Groups {
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

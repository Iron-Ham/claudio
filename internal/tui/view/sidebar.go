package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// SidebarMode represents the display mode of the sidebar.
type SidebarMode int

const (
	// SidebarModeFlat displays instances as a flat list (default).
	SidebarModeFlat SidebarMode = iota
	// SidebarModeGrouped displays instances organized by groups.
	SidebarModeGrouped
	// SidebarModeGraph displays instances as a dependency graph (DAG).
	SidebarModeGraph
)

// SidebarState provides the state needed for sidebar rendering with group support.
// This extends DashboardState with group-specific state.
type SidebarState interface {
	DashboardState

	// GroupViewState returns the current group view state.
	// Returns nil if not in grouped mode.
	GroupViewState() *GroupViewState

	// SidebarMode returns the current display mode.
	SidebarMode() SidebarMode

	// UltraPlanState returns the ultraplan state if in ultraplan mode.
	// Returns nil if not in ultraplan mode.
	UltraPlanState() *UltraPlanState

	// MultiPlanState returns the multiplan state for tracking multiple concurrent multiplan sessions.
	// Returns nil if no multiplan sessions are active.
	MultiPlanState() *MultiPlanState

	// PlanStateData returns the plan state for tracking multiple concurrent plan sessions.
	// Returns nil if no plan sessions are active.
	PlanStateData() *PlanState

	// Orchestrator returns the orchestrator for instance lookups.
	Orchestrator() *orchestrator.Orchestrator

	// IsInstanceSelected returns true if the given instance ID is currently selected.
	IsInstanceSelected(instanceID string) bool
}

// SidebarView handles rendering of the sidebar in both flat and grouped modes.
type SidebarView struct {
	dashboard *DashboardView
}

// NewSidebarView creates a new SidebarView.
func NewSidebarView() *SidebarView {
	return &SidebarView{
		dashboard: NewDashboardView(),
	}
}

// RenderSidebar renders the sidebar based on the current mode.
// If the state implements SidebarState and is in grouped mode, renders grouped view.
// If in graph mode, renders the dependency graph view.
// Otherwise, falls back to the flat dashboard view.
func (sv *SidebarView) RenderSidebar(state DashboardState, width, height int) string {
	// Check if we have a SidebarState with group support
	ss, ok := state.(SidebarState)
	if !ok {
		// Fall back to flat view
		return sv.dashboard.RenderSidebar(state, width, height)
	}

	// Handle different sidebar modes
	switch ss.SidebarMode() {
	case SidebarModeGraph:
		// Render dependency graph view
		session := state.Session()
		if session != nil && len(session.Instances) > 0 {
			graphView := NewGraphView(session)
			return graphView.RenderGraphSidebar(state, width, height)
		}
		// No instances, fall back to flat view
		return sv.dashboard.RenderSidebar(state, width, height)

	case SidebarModeGrouped:
		// Check if session has groups
		session := state.Session()
		if ss.GroupViewState() != nil && session != nil && session.HasGroups() {
			return sv.RenderGroupedSidebar(ss, width, height)
		}
		// No groups defined, fall back to flat view
		return sv.dashboard.RenderSidebar(state, width, height)

	default:
		// SidebarModeFlat or unknown
		return sv.dashboard.RenderSidebar(state, width, height)
	}
}

// RenderGroupedSidebar renders the sidebar with groups.
func (sv *SidebarView) RenderGroupedSidebar(state SidebarState, width, height int) string {
	var b strings.Builder

	// Sidebar title
	b.WriteString(styles.SidebarTitle.Render("Instances"))
	b.WriteString("\n")

	session := state.Session()
	groupState := state.GroupViewState()
	isAddingTask := state.IsAddingTask()

	// Build conflict map
	conflicts := state.Conflicts()
	conflictingInstances := sv.buildConflictMap(conflicts)

	// Calculate available lines for content (not slots - actual lines!)
	// Reserve: 1 for title, 1 for blank line, 1 for hint, 2 for scroll indicators, plus padding
	reservedLines := 6
	if isAddingTask {
		reservedLines += 2 // "New Task" entry takes 2 lines
	}
	availableLines := max(height-reservedLines, 5)

	// Get flattened items for display
	items := FlattenGroupsForDisplay(session, groupState)
	if len(items) == 0 && !isAddingTask {
		b.WriteString(styles.Muted.Render("No instances"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [:a] to add"))
	} else {
		// Calculate scroll offset for grouped view
		scrollOffset := state.SidebarScrollOffset()
		hasMoreAbove := scrollOffset > 0

		// Show scroll up indicator (counts as 1 line)
		if hasMoreAbove {
			scrollUp := styles.Muted.Render(fmt.Sprintf("\u25b2 %d more above", scrollOffset))
			b.WriteString(scrollUp)
			b.WriteString("\n")
			availableLines-- // Account for scroll indicator line
		}

		// Reserve space for scroll down indicator (1 line)
		availableLines--

		// Track actual lines used and where we stop rendering
		linesUsed := 0
		lastRenderedIdx := scrollOffset - 1 // Will be updated as we render

		// Render visible items, tracking actual line usage
		// ActiveTab returns the index in session.Instances, so compare with AbsoluteIdx
		// When adding a task, no instance should be highlighted (use -1 to match nothing)
		activeInstanceIdx := -1
		if !isAddingTask {
			activeInstanceIdx = state.ActiveTab()
		}

		for i := scrollOffset; i < len(items); i++ {
			item := items[i]

			// Calculate how many lines this item will take
			var itemLines int
			var renderedContent string

			switch v := item.(type) {
			case GroupHeaderItem:
				indent := strings.Repeat("  ", v.Depth)
				header := RenderGroupHeader(v.Group, v.Progress, v.Collapsed, v.IsSelected, width-len(indent))
				renderedContent = indent + header
				// Count lines: header can be multi-line due to word wrapping
				itemLines = strings.Count(renderedContent, "\n") + 1 // +1 for the line itself

			case GroupedInstance:
				isActive := v.AbsoluteIdx == activeInstanceIdx
				hasConflict := conflictingInstances[v.Instance.ID]
				renderedContent = RenderGroupedInstance(v, isActive, hasConflict, width)
				// RenderGroupedInstance returns 2 lines (name + status with internal \n)
				itemLines = strings.Count(renderedContent, "\n") + 1
			}

			// Check if adding this item would exceed available lines
			if linesUsed+itemLines > availableLines {
				break
			}

			// Render the item
			b.WriteString(renderedContent)
			b.WriteString("\n")
			linesUsed += itemLines
			lastRenderedIdx = i
		}

		// Show scroll down indicator if there are more items
		hasMoreBelow := lastRenderedIdx < len(items)-1
		if hasMoreBelow {
			remaining := len(items) - lastRenderedIdx - 1
			scrollDown := styles.Muted.Render(fmt.Sprintf("\u25bc %d more below", remaining))
			b.WriteString(scrollDown)
			b.WriteString("\n")
		}

		// Show "New Task" entry when adding
		if isAddingTask {
			totalInstances := GetVisibleInstanceCount(session, groupState)
			newTaskLabel := fmt.Sprintf("%d New Task", totalInstances+1)
			dot := lipgloss.NewStyle().Foreground(styles.PrimaryColor).Render("\u25cf")
			b.WriteString(dot + " " + styles.SidebarItemActive.Render(newTaskLabel))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	// Help hints
	if len(items) > 0 {
		hintStyle := styles.Muted
		helpHint := hintStyle.Render("[h/l]") + " " + hintStyle.Render("nav") + "  " +
			hintStyle.Render("[J/K]") + " " + hintStyle.Render("scroll") + "  " +
			hintStyle.Render("[gc]") + " " + hintStyle.Render("toggle")
		b.WriteString(helpHint)
	} else {
		addHint := styles.Muted.Render("[:a]") + " " + styles.Muted.Render("Add new")
		b.WriteString(addHint)
	}

	// Wrap in sidebar box
	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// buildConflictMap creates a map of instance IDs that have conflicts.
func (sv *SidebarView) buildConflictMap(conflicts []conflict.FileConflict) map[string]bool {
	conflictingInstances := make(map[string]bool)
	for _, c := range conflicts {
		for _, instID := range c.Instances {
			conflictingInstances[instID] = true
		}
	}
	return conflictingInstances
}

// GroupNavigator handles keyboard navigation for the grouped sidebar view.
type GroupNavigator struct {
	session    *orchestrator.Session
	groupState *GroupViewState
}

// NewGroupNavigator creates a new GroupNavigator.
func NewGroupNavigator(session *orchestrator.Session, groupState *GroupViewState) *GroupNavigator {
	return &GroupNavigator{
		session:    session,
		groupState: groupState,
	}
}

// MoveToNextGroup moves selection to the next visible group (gn).
// Returns the new selected group ID.
// Only navigates to groups that are visible (not hidden by a collapsed parent).
func (n *GroupNavigator) MoveToNextGroup() string {
	if n.session == nil || !n.session.HasGroups() {
		return ""
	}

	groupIDs := GetVisibleGroupIDs(n.session, n.groupState)
	if len(groupIDs) == 0 {
		return ""
	}

	currentID := n.groupState.SelectedGroupID

	// If no group selected, select the first one
	if currentID == "" {
		n.groupState.SelectedGroupID = groupIDs[0]
		return groupIDs[0]
	}

	// Find current index and move to next
	for i, id := range groupIDs {
		if id == currentID {
			if i+1 < len(groupIDs) {
				n.groupState.SelectedGroupID = groupIDs[i+1]
				return groupIDs[i+1]
			}
			// Already at last group, stay there
			return currentID
		}
	}

	// Current group not found (may have been hidden), select first visible
	n.groupState.SelectedGroupID = groupIDs[0]
	return groupIDs[0]
}

// MoveToPrevGroup moves selection to the previous visible group (gp).
// Returns the new selected group ID.
// Only navigates to groups that are visible (not hidden by a collapsed parent).
func (n *GroupNavigator) MoveToPrevGroup() string {
	if n.session == nil || !n.session.HasGroups() {
		return ""
	}

	groupIDs := GetVisibleGroupIDs(n.session, n.groupState)
	if len(groupIDs) == 0 {
		return ""
	}

	currentID := n.groupState.SelectedGroupID

	// If no group selected, select the last one
	if currentID == "" {
		n.groupState.SelectedGroupID = groupIDs[len(groupIDs)-1]
		return groupIDs[len(groupIDs)-1]
	}

	// Find current index and move to previous
	for i, id := range groupIDs {
		if id == currentID {
			if i > 0 {
				n.groupState.SelectedGroupID = groupIDs[i-1]
				return groupIDs[i-1]
			}
			// Already at first group, stay there
			return currentID
		}
	}

	// Current group not found (may have been hidden), select last visible
	n.groupState.SelectedGroupID = groupIDs[len(groupIDs)-1]
	return groupIDs[len(groupIDs)-1]
}

// ToggleSelectedGroup toggles the collapse state of the currently selected group.
// Returns true if a group was toggled.
func (n *GroupNavigator) ToggleSelectedGroup() bool {
	if n.groupState.SelectedGroupID == "" {
		return false
	}

	n.groupState.ToggleCollapse(n.groupState.SelectedGroupID)
	return true
}

// ClearGroupSelection clears the group selection (returns to instance navigation mode).
func (n *GroupNavigator) ClearGroupSelection() {
	n.groupState.SelectedGroupID = ""
}

// GetSelectedGroupID returns the currently selected group ID.
func (n *GroupNavigator) GetSelectedGroupID() string {
	return n.groupState.SelectedGroupID
}

// MoveToNextInstance moves to the next visible instance (j key in grouped mode).
// Returns the new instance index.
func (n *GroupNavigator) MoveToNextInstance(currentIdx int) int {
	if n.session == nil {
		return currentIdx
	}

	// Clear group selection when using instance navigation
	n.ClearGroupSelection()

	totalVisible := GetVisibleInstanceCount(n.session, n.groupState)
	if currentIdx+1 < totalVisible {
		return currentIdx + 1
	}
	return currentIdx
}

// MoveToPrevInstance moves to the previous visible instance (k key in grouped mode).
// Returns the new instance index.
func (n *GroupNavigator) MoveToPrevInstance(currentIdx int) int {
	// Clear group selection when using instance navigation
	n.ClearGroupSelection()

	if currentIdx > 0 {
		return currentIdx - 1
	}
	return 0
}

// GetInstanceAtIndex returns the instance at the given global index.
func (n *GroupNavigator) GetInstanceAtIndex(idx int) *orchestrator.Instance {
	return FindInstanceByGlobalIndex(n.session, n.groupState, idx)
}

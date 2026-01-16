package tui

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// GroupKeyHandler handles group-related keyboard shortcuts.
// It implements a vim-style "g prefix" pattern where pressing 'g' enters
// a group command mode, and the following key determines the action.
type GroupKeyHandler struct {
	session    *orchestrator.Session
	groupState *view.GroupViewState
	navigator  *view.GroupNavigator
}

// NewGroupKeyHandler creates a new GroupKeyHandler.
func NewGroupKeyHandler(session *orchestrator.Session, groupState *view.GroupViewState) *GroupKeyHandler {
	return &GroupKeyHandler{
		session:    session,
		groupState: groupState,
		navigator:  view.NewGroupNavigator(session, groupState),
	}
}

// GroupAction represents the result of a group key action.
type GroupAction int

const (
	// GroupActionNone indicates no action was taken.
	GroupActionNone GroupAction = iota
	// GroupActionToggleCollapse toggles collapse state of current group.
	GroupActionToggleCollapse
	// GroupActionCollapseAll collapses or expands all groups.
	GroupActionCollapseAll
	// GroupActionNextGroup moves to the next group.
	GroupActionNextGroup
	// GroupActionPrevGroup moves to the previous group.
	GroupActionPrevGroup
	// GroupActionSkipGroup marks all pending tasks in current group as skipped.
	GroupActionSkipGroup
	// GroupActionRetryGroup retries failed tasks in current group.
	GroupActionRetryGroup
	// GroupActionForceStart force-starts the next group ignoring dependencies.
	GroupActionForceStart
	// GroupActionDismissGroup removes all instances in the current group.
	GroupActionDismissGroup
)

// GroupKeyResult represents the result of handling a group key.
type GroupKeyResult struct {
	Action       GroupAction
	Handled      bool
	GroupID      string   // The affected group ID (if any)
	InstanceIDs  []string // Instance IDs affected (for skip/retry actions)
	AllCollapsed bool     // For CollapseAll: true=collapse all, false=expand all
}

// HandleGroupKey handles a key press in group command mode (after 'g' prefix).
// Returns the action to take and whether the key was handled.
func (h *GroupKeyHandler) HandleGroupKey(key tea.KeyMsg) GroupKeyResult {
	if h.session == nil || !h.session.HasGroups() || h.groupState == nil {
		return GroupKeyResult{Handled: false}
	}

	keyStr := key.String()
	switch keyStr {
	case "c":
		// gc - collapse/expand current group
		return h.handleToggleCollapse()

	case "C":
		// gC - collapse/expand all groups
		return h.handleCollapseAll()

	case "n":
		// gn - jump to next group
		return h.handleNextGroup()

	case "p":
		// gp - jump to previous group
		return h.handlePrevGroup()

	case "s":
		// gs - skip current group (mark all pending as skipped)
		return h.handleSkipGroup()

	case "r":
		// gr - retry failed tasks in current group
		return h.handleRetryGroup()

	case "f":
		// gf - force-start next group (ignore dependencies)
		return h.handleForceStart()

	case "q":
		// gq - dismiss/remove all instances in current group
		return h.handleDismissGroup()

	default:
		return GroupKeyResult{Handled: false}
	}
}

// handleToggleCollapse toggles the collapse state of the currently selected group.
func (h *GroupKeyHandler) handleToggleCollapse() GroupKeyResult {
	groupID := h.groupState.SelectedGroupID
	if groupID == "" {
		// If no group is selected, try to select the first visible group and toggle it
		groupIDs := view.GetVisibleGroupIDs(h.session, h.groupState)
		if len(groupIDs) > 0 {
			groupID = groupIDs[0]
			h.groupState.SelectedGroupID = groupID
		}
	}

	if groupID != "" {
		h.groupState.ToggleCollapse(groupID)
		return GroupKeyResult{
			Action:  GroupActionToggleCollapse,
			Handled: true,
			GroupID: groupID,
		}
	}
	return GroupKeyResult{Handled: false}
}

// handleCollapseAll toggles collapse state for all groups.
// If any group is expanded, collapses all. Otherwise, expands all.
func (h *GroupKeyHandler) handleCollapseAll() GroupKeyResult {
	groupIDs := view.GetGroupIDs(h.session)
	if len(groupIDs) == 0 {
		return GroupKeyResult{Handled: false}
	}

	// Check if any group is currently expanded
	anyExpanded := false
	for _, id := range groupIDs {
		if !h.groupState.IsCollapsed(id) {
			anyExpanded = true
			break
		}
	}

	// Toggle all groups
	for _, id := range groupIDs {
		if anyExpanded {
			// Collapse all
			h.groupState.CollapsedGroups[id] = true
		} else {
			// Expand all
			h.groupState.CollapsedGroups[id] = false
		}
	}

	return GroupKeyResult{
		Action:       GroupActionCollapseAll,
		Handled:      true,
		AllCollapsed: anyExpanded, // true if we collapsed all
	}
}

// handleNextGroup moves selection to the next group.
func (h *GroupKeyHandler) handleNextGroup() GroupKeyResult {
	newGroupID := h.navigator.MoveToNextGroup()
	return GroupKeyResult{
		Action:  GroupActionNextGroup,
		Handled: true,
		GroupID: newGroupID,
	}
}

// handlePrevGroup moves selection to the previous group.
func (h *GroupKeyHandler) handlePrevGroup() GroupKeyResult {
	newGroupID := h.navigator.MoveToPrevGroup()
	return GroupKeyResult{
		Action:  GroupActionPrevGroup,
		Handled: true,
		GroupID: newGroupID,
	}
}

// handleSkipGroup marks all pending tasks in the current group as skipped.
func (h *GroupKeyHandler) handleSkipGroup() GroupKeyResult {
	groupID := h.groupState.SelectedGroupID
	if groupID == "" {
		return GroupKeyResult{Handled: false}
	}

	// Find the group and collect pending instance IDs
	group := h.findGroup(groupID)
	if group == nil {
		return GroupKeyResult{Handled: false}
	}

	var pendingIDs []string
	for _, instID := range group.Instances {
		inst := h.session.GetInstance(instID)
		if inst != nil && inst.Status == orchestrator.StatusPending {
			pendingIDs = append(pendingIDs, instID)
		}
	}

	if len(pendingIDs) == 0 {
		return GroupKeyResult{Handled: false}
	}

	return GroupKeyResult{
		Action:      GroupActionSkipGroup,
		Handled:     true,
		GroupID:     groupID,
		InstanceIDs: pendingIDs,
	}
}

// handleRetryGroup retries failed or interrupted tasks in the current group.
func (h *GroupKeyHandler) handleRetryGroup() GroupKeyResult {
	groupID := h.groupState.SelectedGroupID
	if groupID == "" {
		return GroupKeyResult{Handled: false}
	}

	// Find the group and collect restartable instance IDs
	group := h.findGroup(groupID)
	if group == nil {
		return GroupKeyResult{Handled: false}
	}

	var restartableIDs []string
	for _, instID := range group.Instances {
		inst := h.session.GetInstance(instID)
		if inst != nil && isRestartableStatus(inst.Status) {
			restartableIDs = append(restartableIDs, instID)
		}
	}

	if len(restartableIDs) == 0 {
		return GroupKeyResult{Handled: false}
	}

	return GroupKeyResult{
		Action:      GroupActionRetryGroup,
		Handled:     true,
		GroupID:     groupID,
		InstanceIDs: restartableIDs,
	}
}

// handleForceStart returns a force-start action to bypass group dependencies.
func (h *GroupKeyHandler) handleForceStart() GroupKeyResult {
	// Find the next pending group (thread-safe)
	for _, group := range h.session.GetGroups() {
		if group.Phase == orchestrator.GroupPhasePending {
			return GroupKeyResult{
				Action:  GroupActionForceStart,
				Handled: true,
				GroupID: group.ID,
			}
		}
	}
	return GroupKeyResult{Handled: false}
}

// handleDismissGroup removes all instances in the current group.
func (h *GroupKeyHandler) handleDismissGroup() GroupKeyResult {
	groupID := h.groupState.SelectedGroupID
	if groupID == "" {
		return GroupKeyResult{Handled: false}
	}

	// Find the group and collect all instance IDs
	group := h.findGroup(groupID)
	if group == nil {
		return GroupKeyResult{Handled: false}
	}

	if len(group.Instances) == 0 {
		return GroupKeyResult{Handled: false}
	}

	// Return all instance IDs in the group for dismissal
	return GroupKeyResult{
		Action:      GroupActionDismissGroup,
		Handled:     true,
		GroupID:     groupID,
		InstanceIDs: group.Instances,
	}
}

// findGroup finds a group by ID in the session.
func (h *GroupKeyHandler) findGroup(groupID string) *orchestrator.InstanceGroup {
	for _, group := range h.session.GetGroups() {
		if group.ID == groupID {
			return group
		}
		// Check subgroups
		if found := h.findGroupRecursive(group.SubGroups, groupID); found != nil {
			return found
		}
	}
	return nil
}

// findGroupRecursive recursively searches for a group in subgroups.
func (h *GroupKeyHandler) findGroupRecursive(groups []*orchestrator.InstanceGroup, groupID string) *orchestrator.InstanceGroup {
	for _, group := range groups {
		if group.ID == groupID {
			return group
		}
		if found := h.findGroupRecursive(group.SubGroups, groupID); found != nil {
			return found
		}
	}
	return nil
}

// isFailedStatus returns true if the status indicates a failed instance.
func isFailedStatus(status orchestrator.InstanceStatus) bool {
	return status == orchestrator.StatusError ||
		status == orchestrator.StatusStuck ||
		status == orchestrator.StatusTimeout
}

// isRestartableStatus returns true if the status indicates an instance that can be restarted.
// This includes interrupted instances (from Claudio exit) and failed instances.
func isRestartableStatus(status orchestrator.InstanceStatus) bool {
	return status == orchestrator.StatusInterrupted ||
		status == orchestrator.StatusPaused ||
		status == orchestrator.StatusStuck ||
		status == orchestrator.StatusTimeout ||
		status == orchestrator.StatusError
}

// -----------------------------------------------------------------------------
// Model extensions for group key handling
// -----------------------------------------------------------------------------

// HasGroupViewState returns true if the model has groups available in the session.
// This is used to determine if group-related keyboard shortcuts should be active.
func (m Model) HasGroupViewState() bool {
	return m.session != nil && m.session.HasGroups()
}

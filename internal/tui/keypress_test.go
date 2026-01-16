package tui

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewGroupKeyHandler(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)

	if handler == nil {
		t.Fatal("NewGroupKeyHandler returned nil")
	}
	if handler.session != session {
		t.Error("handler.session not set correctly")
	}
	if handler.groupState != groupState {
		t.Error("handler.groupState not set correctly")
	}
	if handler.navigator == nil {
		t.Error("handler.navigator is nil")
	}
}

func TestGroupKeyHandler_NilSession(t *testing.T) {
	handler := NewGroupKeyHandler(nil, view.NewGroupViewState())
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if result.Handled {
		t.Error("expected Handled=false for nil session")
	}
}

func TestGroupKeyHandler_EmptyGroups(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{},
	}
	handler := NewGroupKeyHandler(session, view.NewGroupViewState())
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if result.Handled {
		t.Error("expected Handled=false for empty groups")
	}
}

func TestGroupKeyHandler_ToggleCollapse(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()
	groupState.SelectedGroupID = session.Groups[0].ID

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionToggleCollapse {
		t.Errorf("expected Action=%v, got %v", GroupActionToggleCollapse, result.Action)
	}
	if result.GroupID != session.Groups[0].ID {
		t.Errorf("expected GroupID=%s, got %s", session.Groups[0].ID, result.GroupID)
	}
	// Verify the group was toggled
	if !groupState.IsCollapsed(session.Groups[0].ID) {
		t.Error("expected group to be collapsed after toggle")
	}
}

func TestGroupKeyHandler_ToggleCollapse_NoSelection(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()
	// No group selected

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Should select the first group and toggle it
	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.GroupID != session.Groups[0].ID {
		t.Errorf("expected first group to be selected")
	}
}

func TestGroupKeyHandler_ToggleCollapse_Subgroup(t *testing.T) {
	session := createTestSessionWithSubgroups()
	groupState := view.NewGroupViewState()

	// Select the subgroup directly
	groupState.SelectedGroupID = "subgroup-1"

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if !result.Handled {
		t.Error("expected Handled=true for subgroup")
	}
	if result.Action != GroupActionToggleCollapse {
		t.Errorf("expected Action=%v, got %v", GroupActionToggleCollapse, result.Action)
	}
	if result.GroupID != "subgroup-1" {
		t.Errorf("expected GroupID=subgroup-1, got %s", result.GroupID)
	}
	// Verify the subgroup was toggled
	if !groupState.IsCollapsed("subgroup-1") {
		t.Error("expected subgroup to be collapsed after toggle")
	}
	// Verify parent group was NOT toggled
	if groupState.IsCollapsed("group-1") {
		t.Error("expected parent group to remain expanded")
	}
}

func TestGroupKeyHandler_CollapseAll(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionCollapseAll {
		t.Errorf("expected Action=%v, got %v", GroupActionCollapseAll, result.Action)
	}
	if !result.AllCollapsed {
		t.Error("expected AllCollapsed=true since groups were expanded")
	}

	// All groups should be collapsed
	for _, group := range session.Groups {
		if !groupState.IsCollapsed(group.ID) {
			t.Errorf("expected group %s to be collapsed", group.ID)
		}
	}

	// Toggle again to expand all
	result = handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	if result.AllCollapsed {
		t.Error("expected AllCollapsed=false since groups were collapsed")
	}

	// All groups should be expanded
	for _, group := range session.Groups {
		if groupState.IsCollapsed(group.ID) {
			t.Errorf("expected group %s to be expanded", group.ID)
		}
	}
}

func TestGroupKeyHandler_NextGroup(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionNextGroup {
		t.Errorf("expected Action=%v, got %v", GroupActionNextGroup, result.Action)
	}
	// First group should be selected since none was selected
	if result.GroupID != session.Groups[0].ID {
		t.Errorf("expected first group to be selected")
	}

	// Move to next
	result = handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result.GroupID != session.Groups[1].ID {
		t.Errorf("expected second group to be selected")
	}
}

func TestGroupKeyHandler_NextGroup_Subgroups(t *testing.T) {
	session := createTestSessionWithSubgroups()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)

	// First 'gn' should select group-1
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result.GroupID != "group-1" {
		t.Errorf("expected group-1, got %s", result.GroupID)
	}

	// Second 'gn' should select subgroup-1 (child of group-1)
	result = handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result.GroupID != "subgroup-1" {
		t.Errorf("expected subgroup-1, got %s", result.GroupID)
	}

	// Third 'gn' should select group-2
	result = handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result.GroupID != "group-2" {
		t.Errorf("expected group-2, got %s", result.GroupID)
	}
}

func TestGroupKeyHandler_NextGroup_ParentCollapsed(t *testing.T) {
	session := createTestSessionWithSubgroups()
	groupState := view.NewGroupViewState()

	// Collapse the parent group
	groupState.CollapsedGroups["group-1"] = true

	handler := NewGroupKeyHandler(session, groupState)

	// First 'gn' should select group-1 (the collapsed parent)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result.GroupID != "group-1" {
		t.Errorf("expected group-1, got %s", result.GroupID)
	}

	// When parent is collapsed, 'gn' should skip the subgroup and go to group-2
	result = handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if result.GroupID != "group-2" {
		t.Errorf("expected group-2 (skip hidden subgroup), got %s", result.GroupID)
	}
}

func TestGroupKeyHandler_PrevGroup_ParentCollapsed(t *testing.T) {
	session := createTestSessionWithSubgroups()
	groupState := view.NewGroupViewState()

	// Collapse the parent group
	groupState.CollapsedGroups["group-1"] = true
	// Start at group-2
	groupState.SelectedGroupID = "group-2"

	handler := NewGroupKeyHandler(session, groupState)

	// 'gp' from group-2 should skip hidden subgroup-1 and go to group-1
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if result.GroupID != "group-1" {
		t.Errorf("expected group-1 (skip hidden subgroup), got %s", result.GroupID)
	}
}

func TestGroupKeyHandler_PrevGroup(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)

	// First, go to the end (prev with nothing selected goes to last)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionPrevGroup {
		t.Errorf("expected Action=%v, got %v", GroupActionPrevGroup, result.Action)
	}
	// Last group should be selected since none was selected
	if result.GroupID != session.Groups[len(session.Groups)-1].ID {
		t.Errorf("expected last group to be selected")
	}
}

func TestGroupKeyHandler_SkipGroup(t *testing.T) {
	session := createTestSessionWithPendingInstances()
	groupState := view.NewGroupViewState()
	groupState.SelectedGroupID = session.Groups[0].ID

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionSkipGroup {
		t.Errorf("expected Action=%v, got %v", GroupActionSkipGroup, result.Action)
	}
	if len(result.InstanceIDs) == 0 {
		t.Error("expected at least one pending instance ID")
	}
}

func TestGroupKeyHandler_SkipGroup_NoSelection(t *testing.T) {
	session := createTestSessionWithPendingInstances()
	groupState := view.NewGroupViewState()
	// No group selected

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if result.Handled {
		t.Error("expected Handled=false when no group is selected")
	}
}

func TestGroupKeyHandler_RetryGroup(t *testing.T) {
	session := createTestSessionWithFailedInstances()
	groupState := view.NewGroupViewState()
	groupState.SelectedGroupID = session.Groups[0].ID

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionRetryGroup {
		t.Errorf("expected Action=%v, got %v", GroupActionRetryGroup, result.Action)
	}
	if len(result.InstanceIDs) == 0 {
		t.Error("expected at least one failed instance ID")
	}
}

func TestGroupKeyHandler_RetryGroup_NoFailedInstances(t *testing.T) {
	session := createTestSession() // No failed instances
	groupState := view.NewGroupViewState()
	groupState.SelectedGroupID = session.Groups[0].ID

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if result.Handled {
		t.Error("expected Handled=false when no failed instances")
	}
}

func TestGroupKeyHandler_ForceStart(t *testing.T) {
	session := createTestSessionWithPendingGroup()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionForceStart {
		t.Errorf("expected Action=%v, got %v", GroupActionForceStart, result.Action)
	}
	if result.GroupID == "" {
		t.Error("expected GroupID to be set")
	}
}

func TestGroupKeyHandler_ForceStart_NoPendingGroup(t *testing.T) {
	session := createTestSessionAllCompleted()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})

	if result.Handled {
		t.Error("expected Handled=false when no pending groups")
	}
}

func TestGroupKeyHandler_DismissGroup(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()
	groupState.SelectedGroupID = session.Groups[0].ID

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if !result.Handled {
		t.Error("expected Handled=true")
	}
	if result.Action != GroupActionDismissGroup {
		t.Errorf("expected Action=%v, got %v", GroupActionDismissGroup, result.Action)
	}
	if result.GroupID != session.Groups[0].ID {
		t.Errorf("expected GroupID=%s, got %s", session.Groups[0].ID, result.GroupID)
	}
	// Verify all instances from the group are returned
	if len(result.InstanceIDs) != 2 {
		t.Errorf("expected 2 instance IDs, got %d", len(result.InstanceIDs))
	}
	// Verify the instance IDs match
	expectedIDs := map[string]bool{"inst-1": true, "inst-2": true}
	for _, id := range result.InstanceIDs {
		if !expectedIDs[id] {
			t.Errorf("unexpected instance ID: %s", id)
		}
	}
}

func TestGroupKeyHandler_DismissGroup_NoSelection(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()
	// No group selected

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if result.Handled {
		t.Error("expected Handled=false when no group is selected")
	}
}

func TestGroupKeyHandler_DismissGroup_EmptyGroup(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "empty-group",
				Name:      "Empty Group",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{}, // Empty
			},
		},
	}
	groupState := view.NewGroupViewState()
	groupState.SelectedGroupID = "empty-group"

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if result.Handled {
		t.Error("expected Handled=false for empty group")
	}
}

func TestGroupKeyHandler_DismissGroup_Subgroup(t *testing.T) {
	session := createTestSessionWithSubgroups()
	groupState := view.NewGroupViewState()

	// Select the subgroup
	groupState.SelectedGroupID = "subgroup-1"

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if !result.Handled {
		t.Error("expected Handled=true for subgroup")
	}
	if result.Action != GroupActionDismissGroup {
		t.Errorf("expected Action=%v, got %v", GroupActionDismissGroup, result.Action)
	}
	if result.GroupID != "subgroup-1" {
		t.Errorf("expected GroupID=subgroup-1, got %s", result.GroupID)
	}
	// Verify only subgroup instances are returned
	if len(result.InstanceIDs) != 1 {
		t.Errorf("expected 1 instance ID for subgroup, got %d", len(result.InstanceIDs))
	}
	if result.InstanceIDs[0] != "inst-2" {
		t.Errorf("expected inst-2, got %s", result.InstanceIDs[0])
	}
}

func TestGroupKeyHandler_UnknownKey(t *testing.T) {
	session := createTestSession()
	groupState := view.NewGroupViewState()

	handler := NewGroupKeyHandler(session, groupState)
	result := handler.HandleGroupKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if result.Handled {
		t.Error("expected Handled=false for unknown key")
	}
}

func TestIsFailedStatus(t *testing.T) {
	tests := []struct {
		status   orchestrator.InstanceStatus
		expected bool
	}{
		{orchestrator.StatusError, true},
		{orchestrator.StatusStuck, true},
		{orchestrator.StatusTimeout, true},
		{orchestrator.StatusPending, false},
		{orchestrator.StatusWorking, false},
		{orchestrator.StatusCompleted, false},
		{orchestrator.StatusPaused, false},
		{orchestrator.StatusInterrupted, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := isFailedStatus(tt.status)
			if got != tt.expected {
				t.Errorf("isFailedStatus(%s) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

func TestIsRestartableStatus(t *testing.T) {
	tests := []struct {
		status   orchestrator.InstanceStatus
		expected bool
	}{
		{orchestrator.StatusInterrupted, true},
		{orchestrator.StatusPaused, true},
		{orchestrator.StatusStuck, true},
		{orchestrator.StatusTimeout, true},
		{orchestrator.StatusError, true},
		{orchestrator.StatusPending, false},
		{orchestrator.StatusWorking, false},
		{orchestrator.StatusCompleted, false},
		{orchestrator.StatusWaitingInput, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := isRestartableStatus(tt.status)
			if got != tt.expected {
				t.Errorf("isRestartableStatus(%s) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Test helpers
// -----------------------------------------------------------------------------

func createTestSession() *orchestrator.Session {
	return &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-1", "inst-2"},
			},
			{
				ID:        "group-2",
				Name:      "Group 2",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-3"},
			},
		},
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusWorking},
			{ID: "inst-2", Status: orchestrator.StatusWorking},
			{ID: "inst-3", Status: orchestrator.StatusWorking},
		},
	}
}

func createTestSessionWithPendingInstances() *orchestrator.Session {
	now := time.Now()
	return &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-1", "inst-2"},
			},
		},
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusPending, Created: now},
			{ID: "inst-2", Status: orchestrator.StatusPending, Created: now},
		},
	}
}

func createTestSessionWithFailedInstances() *orchestrator.Session {
	now := time.Now()
	return &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseFailed,
				Instances: []string{"inst-1", "inst-2"},
			},
		},
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusError, Created: now},
			{ID: "inst-2", Status: orchestrator.StatusTimeout, Created: now},
		},
	}
}

func createTestSessionWithPendingGroup() *orchestrator.Session {
	now := time.Now()
	return &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst-1"},
			},
			{
				ID:        "group-2",
				Name:      "Group 2",
				Phase:     orchestrator.GroupPhasePending,
				Instances: []string{"inst-2"},
			},
		},
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusCompleted, Created: now},
			{ID: "inst-2", Status: orchestrator.StatusPending, Created: now},
		},
	}
}

func createTestSessionAllCompleted() *orchestrator.Session {
	now := time.Now()
	return &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst-1"},
			},
		},
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusCompleted, Created: now},
		},
	}
}

func createTestSessionWithSubgroups() *orchestrator.Session {
	return &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-1"},
				SubGroups: []*orchestrator.InstanceGroup{
					{
						ID:        "subgroup-1",
						Name:      "Subgroup 1",
						Phase:     orchestrator.GroupPhaseExecuting,
						Instances: []string{"inst-2"},
						ParentID:  "group-1",
					},
				},
			},
			{
				ID:        "group-2",
				Name:      "Group 2",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-3"},
			},
		},
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusWorking},
			{ID: "inst-2", Status: orchestrator.StatusWorking},
			{ID: "inst-3", Status: orchestrator.StatusWorking},
		},
	}
}

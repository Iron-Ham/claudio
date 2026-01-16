package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// mockSidebarState implements SidebarState for testing grouped views.
type mockSidebarState struct {
	session                  *orchestrator.Session
	activeTab                int
	sidebarScrollOffset      int
	conflicts                []conflict.FileConflict
	terminalWidth            int
	terminalHeight           int
	isAddingTask             bool
	intelligentNamingEnabled bool
	groupViewState           *GroupViewState
	sidebarMode              SidebarMode
}

func (m *mockSidebarState) Session() *orchestrator.Session     { return m.session }
func (m *mockSidebarState) ActiveTab() int                     { return m.activeTab }
func (m *mockSidebarState) SidebarScrollOffset() int           { return m.sidebarScrollOffset }
func (m *mockSidebarState) Conflicts() []conflict.FileConflict { return m.conflicts }
func (m *mockSidebarState) TerminalWidth() int                 { return m.terminalWidth }
func (m *mockSidebarState) TerminalHeight() int                { return m.terminalHeight }
func (m *mockSidebarState) IsAddingTask() bool                 { return m.isAddingTask }
func (m *mockSidebarState) IntelligentNamingEnabled() bool     { return m.intelligentNamingEnabled }
func (m *mockSidebarState) GroupViewState() *GroupViewState    { return m.groupViewState }
func (m *mockSidebarState) SidebarMode() SidebarMode           { return m.sidebarMode }
func (m *mockSidebarState) UltraPlanState() *UltraPlanState    { return nil }
func (m *mockSidebarState) MultiPlanState() *MultiPlanState    { return nil }
func (m *mockSidebarState) PlanStateData() *PlanState          { return nil }
func (m *mockSidebarState) Orchestrator() *orchestrator.Orchestrator {
	return nil
}
func (m *mockSidebarState) IsInstanceSelected(instanceID string) bool {
	if m.session == nil || m.activeTab < 0 || m.activeTab >= len(m.session.Instances) {
		return false
	}
	return m.session.Instances[m.activeTab].ID == instanceID
}

func TestSidebarView_FlatModeFallback(t *testing.T) {
	// When in flat mode, should render like DashboardView
	state := &mockSidebarState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-1", Task: "Test task 1", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Task: "Test task 2", Status: orchestrator.StatusPending},
			},
		},
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 24,
		sidebarMode:    SidebarModeFlat,
		groupViewState: nil,
	}

	sv := NewSidebarView()
	result := sv.RenderSidebar(state, 30, 20)

	// Should contain instance tasks (flat view)
	if !strings.Contains(result, "Test task 1") {
		t.Error("flat mode should show first instance task")
	}
	if !strings.Contains(result, "Test task 2") {
		t.Error("flat mode should show second instance task")
	}
}

func TestSidebarView_GroupedModeWithNoGroups(t *testing.T) {
	// When in grouped mode but no groups defined, should fall back to flat
	state := &mockSidebarState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-1", Task: "Test task", Status: orchestrator.StatusWorking},
			},
			Groups: nil, // No groups
		},
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 24,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: NewGroupViewState(),
	}

	sv := NewSidebarView()
	result := sv.RenderSidebar(state, 30, 20)

	// Should fall back to flat view
	if !strings.Contains(result, "Test task") {
		t.Error("should fall back to flat view when no groups defined")
	}
}

func TestSidebarView_GroupedModeWithGroups(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Setup auth", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Task: "Create migrations", Status: orchestrator.StatusCompleted},
			{ID: "inst-3", Task: "Auth service", Status: orchestrator.StatusWorking},
			{ID: "inst-4", Task: "Token handler", Status: orchestrator.StatusPending},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1: Setup",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst-1", "inst-2"},
			},
			{
				ID:        "group-2",
				Name:      "Group 2: Core Logic",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-3", "inst-4"},
			},
		},
	}

	state := &mockSidebarState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: NewGroupViewState(),
	}

	sv := NewSidebarView()
	result := sv.RenderSidebar(state, 40, 25)

	// Should contain group names
	if !strings.Contains(result, "Group 1") {
		t.Errorf("grouped mode should show group 1, got:\n%s", result)
	}
	if !strings.Contains(result, "Group 2") {
		t.Errorf("grouped mode should show group 2, got:\n%s", result)
	}

	// Should contain progress indicators
	if !strings.Contains(result, "[2/2]") {
		t.Errorf("should show progress [2/2] for completed group, got:\n%s", result)
	}

	// Should contain instance names
	if !strings.Contains(result, "Setup auth") {
		t.Errorf("should show instance task 'Setup auth', got:\n%s", result)
	}
}

func TestSidebarView_CollapsedGroup(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusWorking},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Collapsed Group",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst-1"},
			},
			{
				ID:        "group-2",
				Name:      "Expanded Group",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-2"},
			},
		},
	}

	groupState := NewGroupViewState()
	groupState.CollapsedGroups["group-1"] = true // Collapse first group

	state := &mockSidebarState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: groupState,
	}

	sv := NewSidebarView()
	result := sv.RenderSidebar(state, 40, 25)

	// Should show collapsed group header
	if !strings.Contains(result, "Collapsed Group") {
		t.Errorf("should show collapsed group header, got:\n%s", result)
	}

	// Should NOT show instances from collapsed group
	if strings.Contains(result, "Task 1") {
		t.Errorf("should NOT show instances from collapsed group, got:\n%s", result)
	}

	// Should show expanded group instances
	if !strings.Contains(result, "Task 2") {
		t.Errorf("should show instances from expanded group, got:\n%s", result)
	}
}

func TestSidebarView_GroupNavHints(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task", Status: orchestrator.StatusWorking},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-1"},
			},
		},
	}

	state := &mockSidebarState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: NewGroupViewState(),
	}

	sv := NewSidebarView()
	result := sv.RenderSidebar(state, 50, 25)

	// Should contain group navigation hints
	if !strings.Contains(result, "[gn/gp]") {
		t.Errorf("should show [gn/gp] hint for group navigation, got:\n%s", result)
	}
	if !strings.Contains(result, "[gc]") || !strings.Contains(result, "toggle") {
		t.Errorf("should show [gc] toggle hint, got:\n%s", result)
	}
}

func TestGroupViewState_ToggleCollapse(t *testing.T) {
	state := NewGroupViewState()

	// Initially not collapsed
	if state.IsCollapsed("group-1") {
		t.Error("group should not be collapsed initially")
	}

	// Toggle to collapsed
	state.ToggleCollapse("group-1")
	if !state.IsCollapsed("group-1") {
		t.Error("group should be collapsed after toggle")
	}

	// Toggle back to expanded
	state.ToggleCollapse("group-1")
	if state.IsCollapsed("group-1") {
		t.Error("group should be expanded after second toggle")
	}
}

func TestCalculateGroupProgress(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Status: orchestrator.StatusCompleted},
			{ID: "inst-3", Status: orchestrator.StatusWorking},
			{ID: "inst-4", Status: orchestrator.StatusPending},
		},
	}

	tests := []struct {
		name          string
		group         *orchestrator.InstanceGroup
		wantCompleted int
		wantTotal     int
	}{
		{
			name: "all completed",
			group: &orchestrator.InstanceGroup{
				Instances: []string{"inst-1", "inst-2"},
			},
			wantCompleted: 2,
			wantTotal:     2,
		},
		{
			name: "mixed status",
			group: &orchestrator.InstanceGroup{
				Instances: []string{"inst-1", "inst-3", "inst-4"},
			},
			wantCompleted: 1,
			wantTotal:     3,
		},
		{
			name: "with sub-groups",
			group: &orchestrator.InstanceGroup{
				Instances: []string{"inst-1"},
				SubGroups: []*orchestrator.InstanceGroup{
					{Instances: []string{"inst-2", "inst-3"}},
				},
			},
			wantCompleted: 2, // inst-1 and inst-2
			wantTotal:     3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			progress := CalculateGroupProgress(tt.group, session)
			if progress.Completed != tt.wantCompleted {
				t.Errorf("Completed = %d, want %d", progress.Completed, tt.wantCompleted)
			}
			if progress.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", progress.Total, tt.wantTotal)
			}
		})
	}
}

func TestPhaseIndicator(t *testing.T) {
	tests := []struct {
		phase    orchestrator.GroupPhase
		expected string
	}{
		{orchestrator.GroupPhasePending, " "},
		{orchestrator.GroupPhaseExecuting, "\u25cf"}, // filled circle
		{orchestrator.GroupPhaseCompleted, "\u2713"}, // checkmark
		{orchestrator.GroupPhaseFailed, "\u2717"},    // X mark
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			result := PhaseIndicator(tt.phase)
			if result != tt.expected {
				t.Errorf("PhaseIndicator(%q) = %q, want %q", tt.phase, result, tt.expected)
			}
		})
	}
}

func TestFlattenGroupsForDisplay(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusWorking},
			{ID: "inst-3", Task: "Task 3", Status: orchestrator.StatusPending},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Phase:     orchestrator.GroupPhaseCompleted,
				Instances: []string{"inst-1"},
				SubGroups: []*orchestrator.InstanceGroup{
					{
						ID:        "subgroup-1",
						Name:      "SubGroup 1",
						Phase:     orchestrator.GroupPhaseExecuting,
						Instances: []string{"inst-2"},
					},
				},
			},
			{
				ID:        "group-2",
				Name:      "Group 2",
				Phase:     orchestrator.GroupPhasePending,
				Instances: []string{"inst-3"},
			},
		},
	}

	state := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, state)

	// Expected order:
	// 1. GroupHeader (group-1)
	// 2. GroupedInstance (inst-1)
	// 3. GroupHeader (subgroup-1)
	// 4. GroupedInstance (inst-2)
	// 5. GroupHeader (group-2)
	// 6. GroupedInstance (inst-3)

	if len(items) != 6 {
		t.Fatalf("expected 6 items, got %d", len(items))
	}

	// Check types
	if _, ok := items[0].(GroupHeaderItem); !ok {
		t.Error("item 0 should be GroupHeaderItem")
	}
	if gi, ok := items[1].(GroupedInstance); !ok || gi.Instance.ID != "inst-1" {
		t.Error("item 1 should be GroupedInstance for inst-1")
	}
	if _, ok := items[2].(GroupHeaderItem); !ok {
		t.Error("item 2 should be GroupHeaderItem (subgroup)")
	}
	if gi, ok := items[3].(GroupedInstance); !ok || gi.Instance.ID != "inst-2" {
		t.Error("item 3 should be GroupedInstance for inst-2")
	}
}

func TestFlattenGroupsForDisplay_CollapsedGroup(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusWorking},
		},
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
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-2"},
			},
		},
	}

	state := NewGroupViewState()
	state.CollapsedGroups["group-1"] = true // Collapse first group

	items := FlattenGroupsForDisplay(session, state)

	// Expected order:
	// 1. GroupHeader (group-1) - collapsed
	// 2. GroupHeader (group-2)
	// 3. GroupedInstance (inst-2)

	if len(items) != 3 {
		t.Fatalf("expected 3 items (collapsed group hides instances), got %d", len(items))
	}

	// First item should be collapsed group header
	if gh, ok := items[0].(GroupHeaderItem); !ok || !gh.Collapsed {
		t.Error("item 0 should be collapsed GroupHeaderItem")
	}
}

func TestGetVisibleInstanceCount(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1"},
			{ID: "inst-2"},
			{ID: "inst-3"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Instances: []string{"inst-1", "inst-2"},
			},
			{
				ID:        "group-2",
				Instances: []string{"inst-3"},
			},
		},
	}

	t.Run("all expanded", func(t *testing.T) {
		state := NewGroupViewState()
		count := GetVisibleInstanceCount(session, state)
		if count != 3 {
			t.Errorf("expected 3 visible instances, got %d", count)
		}
	})

	t.Run("one collapsed", func(t *testing.T) {
		state := NewGroupViewState()
		state.CollapsedGroups["group-1"] = true
		count := GetVisibleInstanceCount(session, state)
		if count != 1 {
			t.Errorf("expected 1 visible instance (group-1 collapsed), got %d", count)
		}
	})

	t.Run("all collapsed", func(t *testing.T) {
		state := NewGroupViewState()
		state.CollapsedGroups["group-1"] = true
		state.CollapsedGroups["group-2"] = true
		count := GetVisibleInstanceCount(session, state)
		if count != 0 {
			t.Errorf("expected 0 visible instances (all collapsed), got %d", count)
		}
	})
}

func TestGroupNavigator_MoveToNextGroup(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Name: "Group 1"},
			{ID: "group-2", Name: "Group 2"},
			{ID: "group-3", Name: "Group 3"},
		},
	}

	state := NewGroupViewState()
	nav := NewGroupNavigator(session, state)

	// First call should select first group
	id := nav.MoveToNextGroup()
	if id != "group-1" {
		t.Errorf("first MoveToNextGroup() = %q, want %q", id, "group-1")
	}

	// Second call should select second group
	id = nav.MoveToNextGroup()
	if id != "group-2" {
		t.Errorf("second MoveToNextGroup() = %q, want %q", id, "group-2")
	}

	// Third call should select third group
	id = nav.MoveToNextGroup()
	if id != "group-3" {
		t.Errorf("third MoveToNextGroup() = %q, want %q", id, "group-3")
	}

	// Fourth call should stay at third group (last one)
	id = nav.MoveToNextGroup()
	if id != "group-3" {
		t.Errorf("fourth MoveToNextGroup() = %q, want %q (should stay at last)", id, "group-3")
	}
}

func TestGroupNavigator_MoveToPrevGroup(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Name: "Group 1"},
			{ID: "group-2", Name: "Group 2"},
			{ID: "group-3", Name: "Group 3"},
		},
	}

	state := NewGroupViewState()
	nav := NewGroupNavigator(session, state)

	// First call with no selection should select last group
	id := nav.MoveToPrevGroup()
	if id != "group-3" {
		t.Errorf("first MoveToPrevGroup() = %q, want %q", id, "group-3")
	}

	// Move back
	id = nav.MoveToPrevGroup()
	if id != "group-2" {
		t.Errorf("second MoveToPrevGroup() = %q, want %q", id, "group-2")
	}

	id = nav.MoveToPrevGroup()
	if id != "group-1" {
		t.Errorf("third MoveToPrevGroup() = %q, want %q", id, "group-1")
	}

	// Should stay at first
	id = nav.MoveToPrevGroup()
	if id != "group-1" {
		t.Errorf("fourth MoveToPrevGroup() = %q, want %q (should stay at first)", id, "group-1")
	}
}

func TestGroupNavigator_ToggleSelectedGroup(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Name: "Group 1"},
		},
	}

	state := NewGroupViewState()
	nav := NewGroupNavigator(session, state)

	// No group selected - toggle should fail
	if nav.ToggleSelectedGroup() {
		t.Error("ToggleSelectedGroup() should return false when no group selected")
	}

	// Select a group
	nav.MoveToNextGroup()

	// Now toggle should work
	if !nav.ToggleSelectedGroup() {
		t.Error("ToggleSelectedGroup() should return true when group selected")
	}

	// Group should be collapsed
	if !state.IsCollapsed("group-1") {
		t.Error("group should be collapsed after toggle")
	}

	// Toggle again
	nav.ToggleSelectedGroup()
	if state.IsCollapsed("group-1") {
		t.Error("group should be expanded after second toggle")
	}
}

func TestGroupNavigator_MoveToNextInstance(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1"},
			{ID: "inst-2"},
			{ID: "inst-3"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Instances: []string{"inst-1", "inst-2", "inst-3"}},
		},
	}

	state := NewGroupViewState()
	state.SelectedGroupID = "group-1" // Start with group selected
	nav := NewGroupNavigator(session, state)

	// Move to next instance should clear group selection
	idx := nav.MoveToNextInstance(0)
	if idx != 1 {
		t.Errorf("MoveToNextInstance(0) = %d, want 1", idx)
	}
	if state.SelectedGroupID != "" {
		t.Error("group selection should be cleared after instance navigation")
	}

	// Continue moving
	idx = nav.MoveToNextInstance(idx)
	if idx != 2 {
		t.Errorf("MoveToNextInstance(1) = %d, want 2", idx)
	}

	// At last instance, should stay
	idx = nav.MoveToNextInstance(idx)
	if idx != 2 {
		t.Errorf("MoveToNextInstance(2) = %d, want 2 (stay at last)", idx)
	}
}

func TestFindInstanceByGlobalIndex(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
			{ID: "inst-3", Task: "Task 3"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Instances: []string{"inst-1"},
			},
			{
				ID:        "group-2",
				Instances: []string{"inst-2", "inst-3"},
			},
		},
	}

	state := NewGroupViewState()

	tests := []struct {
		idx     int
		wantID  string
		wantNil bool
	}{
		{0, "inst-1", false},
		{1, "inst-2", false},
		{2, "inst-3", false},
		{3, "", true},  // Out of bounds
		{-1, "", true}, // Negative
	}

	for _, tt := range tests {
		t.Run(tt.wantID, func(t *testing.T) {
			inst := FindInstanceByGlobalIndex(session, state, tt.idx)
			if tt.wantNil {
				if inst != nil {
					t.Errorf("FindInstanceByGlobalIndex(%d) = %q, want nil", tt.idx, inst.ID)
				}
			} else {
				if inst == nil || inst.ID != tt.wantID {
					var gotID string
					if inst != nil {
						gotID = inst.ID
					}
					t.Errorf("FindInstanceByGlobalIndex(%d) = %q, want %q", tt.idx, gotID, tt.wantID)
				}
			}
		})
	}
}

func TestFindInstanceByGlobalIndex_CollapsedGroup(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
			{ID: "inst-3", Task: "Task 3"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Instances: []string{"inst-1", "inst-2"},
			},
			{
				ID:        "group-2",
				Instances: []string{"inst-3"},
			},
		},
	}

	state := NewGroupViewState()
	state.CollapsedGroups["group-1"] = true // Collapse first group

	// Index 0 should now be inst-3 (only visible instance)
	inst := FindInstanceByGlobalIndex(session, state, 0)
	if inst == nil || inst.ID != "inst-3" {
		var gotID string
		if inst != nil {
			gotID = inst.ID
		}
		t.Errorf("FindInstanceByGlobalIndex(0) with collapsed group = %q, want %q", gotID, "inst-3")
	}

	// Index 1 should be nil (only 1 visible instance)
	inst = FindInstanceByGlobalIndex(session, state, 1)
	if inst != nil {
		t.Errorf("FindInstanceByGlobalIndex(1) with collapsed group = %q, want nil", inst.ID)
	}
}

func TestRenderGroupHeader(t *testing.T) {
	group := &orchestrator.InstanceGroup{
		ID:    "group-1",
		Name:  "Test Group",
		Phase: orchestrator.GroupPhaseExecuting,
	}
	progress := GroupProgress{Completed: 2, Total: 5}

	// Test expanded
	result := RenderGroupHeader(group, progress, false, false, 40)
	if !strings.Contains(result, "Test Group") {
		t.Errorf("should contain group name, got: %s", result)
	}
	if !strings.Contains(result, "[2/5]") {
		t.Errorf("should contain progress [2/5], got: %s", result)
	}
	// Should have expanded indicator (down triangle)
	if !strings.Contains(result, styles.IconGroupExpand) {
		t.Errorf("expanded group should have down triangle, got: %s", result)
	}

	// Test collapsed
	result = RenderGroupHeader(group, progress, true, false, 40)
	// Should have collapsed indicator (right triangle)
	if !strings.Contains(result, styles.IconGroupCollapse) {
		t.Errorf("collapsed group should have right triangle, got: %s", result)
	}
}

func TestInstanceStatusAbbrev(t *testing.T) {
	tests := []struct {
		status   orchestrator.InstanceStatus
		expected string
	}{
		{orchestrator.StatusPending, "PEND"},
		{orchestrator.StatusWorking, "WORK"},
		{orchestrator.StatusWaitingInput, "WAIT"},
		{orchestrator.StatusPaused, "PAUS"},
		{orchestrator.StatusCompleted, "DONE"},
		{orchestrator.StatusError, "ERR!"},
		{orchestrator.StatusCreatingPR, "PR.."},
		{orchestrator.StatusStuck, "STUK"},
		{orchestrator.StatusTimeout, "TIME"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := instanceStatusAbbrev(tt.status)
			if result != tt.expected {
				t.Errorf("instanceStatusAbbrev(%q) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestGetGroupIDs(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID: "group-1",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "subgroup-1"},
					{ID: "subgroup-2"},
				},
			},
			{ID: "group-2"},
		},
	}

	ids := GetGroupIDs(session)

	expected := []string{"group-1", "subgroup-1", "subgroup-2", "group-2"}
	if len(ids) != len(expected) {
		t.Fatalf("GetGroupIDs() returned %d ids, want %d", len(ids), len(expected))
	}

	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("GetGroupIDs()[%d] = %q, want %q", i, id, expected[i])
		}
	}
}

func TestGetGroupIDs_EmptySession(t *testing.T) {
	// nil session
	ids := GetGroupIDs(nil)
	if ids != nil {
		t.Errorf("GetGroupIDs(nil) = %v, want nil", ids)
	}

	// session with no groups
	session := &orchestrator.Session{}
	ids = GetGroupIDs(session)
	if ids != nil {
		t.Errorf("GetGroupIDs(empty) = %v, want nil", ids)
	}
}

func TestGetVisibleGroupIDs(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID: "group-1",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "subgroup-1"},
					{ID: "subgroup-2"},
				},
			},
			{ID: "group-2"},
		},
	}

	t.Run("all expanded", func(t *testing.T) {
		state := NewGroupViewState()
		ids := GetVisibleGroupIDs(session, state)

		expected := []string{"group-1", "subgroup-1", "subgroup-2", "group-2"}
		if len(ids) != len(expected) {
			t.Fatalf("GetVisibleGroupIDs() returned %d ids, want %d", len(ids), len(expected))
		}
		for i, id := range ids {
			if id != expected[i] {
				t.Errorf("GetVisibleGroupIDs()[%d] = %q, want %q", i, id, expected[i])
			}
		}
	})

	t.Run("parent collapsed hides subgroups", func(t *testing.T) {
		state := NewGroupViewState()
		state.CollapsedGroups["group-1"] = true

		ids := GetVisibleGroupIDs(session, state)

		// Subgroups should be hidden when parent is collapsed
		expected := []string{"group-1", "group-2"}
		if len(ids) != len(expected) {
			t.Fatalf("GetVisibleGroupIDs() returned %d ids, want %d", len(ids), len(expected))
		}
		for i, id := range ids {
			if id != expected[i] {
				t.Errorf("GetVisibleGroupIDs()[%d] = %q, want %q", i, id, expected[i])
			}
		}
	})

	t.Run("nil state returns nil", func(t *testing.T) {
		ids := GetVisibleGroupIDs(session, nil)
		if ids != nil {
			t.Errorf("GetVisibleGroupIDs(session, nil) = %v, want nil", ids)
		}
	})

	t.Run("deeply nested subgroups", func(t *testing.T) {
		deepSession := &orchestrator.Session{
			Groups: []*orchestrator.InstanceGroup{
				{
					ID: "level-1",
					SubGroups: []*orchestrator.InstanceGroup{
						{
							ID: "level-2",
							SubGroups: []*orchestrator.InstanceGroup{
								{ID: "level-3"},
							},
						},
					},
				},
			},
		}

		t.Run("all expanded shows all levels", func(t *testing.T) {
			state := NewGroupViewState()
			ids := GetVisibleGroupIDs(deepSession, state)
			expected := []string{"level-1", "level-2", "level-3"}
			if len(ids) != len(expected) {
				t.Fatalf("GetVisibleGroupIDs() returned %d ids, want %d", len(ids), len(expected))
			}
			for i, id := range ids {
				if id != expected[i] {
					t.Errorf("GetVisibleGroupIDs()[%d] = %q, want %q", i, id, expected[i])
				}
			}
		})

		t.Run("collapsing level-1 hides level-2 and level-3", func(t *testing.T) {
			state := NewGroupViewState()
			state.CollapsedGroups["level-1"] = true
			ids := GetVisibleGroupIDs(deepSession, state)
			expected := []string{"level-1"}
			if len(ids) != len(expected) {
				t.Fatalf("GetVisibleGroupIDs() returned %d ids, want %d", len(ids), len(expected))
			}
			if ids[0] != expected[0] {
				t.Errorf("GetVisibleGroupIDs()[0] = %q, want %q", ids[0], expected[0])
			}
		})

		t.Run("collapsing level-2 hides only level-3", func(t *testing.T) {
			state := NewGroupViewState()
			state.CollapsedGroups["level-2"] = true
			ids := GetVisibleGroupIDs(deepSession, state)
			expected := []string{"level-1", "level-2"}
			if len(ids) != len(expected) {
				t.Fatalf("GetVisibleGroupIDs() returned %d ids, want %d", len(ids), len(expected))
			}
			for i, id := range ids {
				if id != expected[i] {
					t.Errorf("GetVisibleGroupIDs()[%d] = %q, want %q", i, id, expected[i])
				}
			}
		})
	})
}

func TestGroupNavigator_NextGroup_SelectedGroupHidden(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{
				ID: "group-1",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "subgroup-1"},
				},
			},
			{ID: "group-2"},
		},
	}

	state := NewGroupViewState()
	// Select a subgroup
	state.SelectedGroupID = "subgroup-1"
	// Collapse the parent (making subgroup-1 invisible)
	state.CollapsedGroups["group-1"] = true

	nav := NewGroupNavigator(session, state)

	// Navigate should reset to first visible group since current selection is hidden
	id := nav.MoveToNextGroup()
	if id != "group-1" {
		t.Errorf("MoveToNextGroup() when selected is hidden = %q, want %q (first visible)", id, "group-1")
	}
}

func TestGroupNavigator_PrevGroup_SelectedGroupHidden(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1"},
			{
				ID: "group-2",
				SubGroups: []*orchestrator.InstanceGroup{
					{ID: "subgroup-2"},
				},
			},
		},
	}

	state := NewGroupViewState()
	// Select a subgroup
	state.SelectedGroupID = "subgroup-2"
	// Collapse the parent (making subgroup-2 invisible)
	state.CollapsedGroups["group-2"] = true

	nav := NewGroupNavigator(session, state)

	// Navigate should reset to last visible group since current selection is hidden
	id := nav.MoveToPrevGroup()
	if id != "group-2" {
		t.Errorf("MoveToPrevGroup() when selected is hidden = %q, want %q (last visible)", id, "group-2")
	}
}

func TestGroupNavigator_MoveToPrevInstance(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1"},
			{ID: "inst-2"},
			{ID: "inst-3"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Instances: []string{"inst-1", "inst-2", "inst-3"}},
		},
	}

	state := NewGroupViewState()
	state.SelectedGroupID = "group-1" // Start with group selected
	nav := NewGroupNavigator(session, state)

	// Move to prev instance from index 2
	idx := nav.MoveToPrevInstance(2)
	if idx != 1 {
		t.Errorf("MoveToPrevInstance(2) = %d, want 1", idx)
	}
	if state.SelectedGroupID != "" {
		t.Error("group selection should be cleared after instance navigation")
	}

	// Continue moving
	idx = nav.MoveToPrevInstance(idx)
	if idx != 0 {
		t.Errorf("MoveToPrevInstance(1) = %d, want 0", idx)
	}

	// At first instance, should stay
	idx = nav.MoveToPrevInstance(idx)
	if idx != 0 {
		t.Errorf("MoveToPrevInstance(0) = %d, want 0 (stay at first)", idx)
	}
}

func TestGroupNavigator_GetInstanceAtIndex(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Instances: []string{"inst-1", "inst-2"}},
		},
	}

	state := NewGroupViewState()
	nav := NewGroupNavigator(session, state)

	// Get instance at valid index
	inst := nav.GetInstanceAtIndex(0)
	if inst == nil || inst.ID != "inst-1" {
		t.Errorf("GetInstanceAtIndex(0) = %v, want inst-1", inst)
	}

	inst = nav.GetInstanceAtIndex(1)
	if inst == nil || inst.ID != "inst-2" {
		t.Errorf("GetInstanceAtIndex(1) = %v, want inst-2", inst)
	}

	// Get instance at invalid index
	inst = nav.GetInstanceAtIndex(99)
	if inst != nil {
		t.Errorf("GetInstanceAtIndex(99) = %v, want nil", inst)
	}
}

func TestGroupNavigator_GetSelectedGroupID(t *testing.T) {
	session := &orchestrator.Session{
		Groups: []*orchestrator.InstanceGroup{
			{ID: "group-1", Name: "Group 1"},
		},
	}

	state := NewGroupViewState()
	nav := NewGroupNavigator(session, state)

	// Initially no selection
	if nav.GetSelectedGroupID() != "" {
		t.Errorf("initial GetSelectedGroupID() = %q, want empty", nav.GetSelectedGroupID())
	}

	// After selecting
	nav.MoveToNextGroup()
	if nav.GetSelectedGroupID() != "group-1" {
		t.Errorf("GetSelectedGroupID() after select = %q, want %q", nav.GetSelectedGroupID(), "group-1")
	}

	// After clearing
	nav.ClearGroupSelection()
	if nav.GetSelectedGroupID() != "" {
		t.Errorf("GetSelectedGroupID() after clear = %q, want empty", nav.GetSelectedGroupID())
	}
}

func TestPhaseColor(t *testing.T) {
	// Test that all phases have colors defined
	phases := []orchestrator.GroupPhase{
		orchestrator.GroupPhasePending,
		orchestrator.GroupPhaseExecuting,
		orchestrator.GroupPhaseCompleted,
		orchestrator.GroupPhaseFailed,
	}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			color := PhaseColor(phase)
			// Just verify it returns a non-empty color
			if color == "" {
				t.Errorf("PhaseColor(%q) returned empty color", phase)
			}
		})
	}

	// Test unknown phase
	unknownColor := PhaseColor("unknown")
	if unknownColor != styles.MutedColor {
		t.Errorf("PhaseColor(unknown) = %v, want MutedColor", unknownColor)
	}
}

func TestFindInstanceIndex(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-a", Task: "Task A"},
			{ID: "inst-b", Task: "Task B"},
			{ID: "inst-c", Task: "Task C"},
		},
	}

	tests := []struct {
		name     string
		session  *orchestrator.Session
		instID   string
		expected int
	}{
		{"first instance", session, "inst-a", 0},
		{"middle instance", session, "inst-b", 1},
		{"last instance", session, "inst-c", 2},
		{"non-existent instance", session, "inst-x", -1},
		{"nil session", nil, "inst-a", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findInstanceIndex(tt.session, tt.instID)
			if result != tt.expected {
				t.Errorf("findInstanceIndex(%q) = %d, want %d", tt.instID, result, tt.expected)
			}
		})
	}
}

func TestGroupedInstance_AbsoluteIdx(t *testing.T) {
	// Test that AbsoluteIdx is correctly set to the instance's position in session.Instances
	// regardless of group structure or collapse state
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusWorking},
			{ID: "inst-3", Task: "Task 3", Status: orchestrator.StatusPending},
			{ID: "inst-4", Task: "Task 4", Status: orchestrator.StatusPending},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Instances: []string{"inst-1", "inst-2"},
			},
			{
				ID:        "group-2",
				Name:      "Group 2",
				Instances: []string{"inst-3", "inst-4"},
			},
		},
	}

	state := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, state)

	// Extract GroupedInstance items and verify AbsoluteIdx
	expectedAbsoluteIdxs := map[string]int{
		"inst-1": 0,
		"inst-2": 1,
		"inst-3": 2,
		"inst-4": 3,
	}

	for _, item := range items {
		if gi, ok := item.(GroupedInstance); ok {
			expected, exists := expectedAbsoluteIdxs[gi.Instance.ID]
			if !exists {
				t.Errorf("unexpected instance ID: %s", gi.Instance.ID)
				continue
			}
			if gi.AbsoluteIdx != expected {
				t.Errorf("AbsoluteIdx for %s = %d, want %d", gi.Instance.ID, gi.AbsoluteIdx, expected)
			}
		}
	}

	// Now collapse group-1 and verify AbsoluteIdx remains stable
	state.CollapsedGroups["group-1"] = true
	items = FlattenGroupsForDisplay(session, state)

	// Only group-2 instances should be visible, but their AbsoluteIdx should remain 2 and 3
	for _, item := range items {
		if gi, ok := item.(GroupedInstance); ok {
			expected := expectedAbsoluteIdxs[gi.Instance.ID]
			if gi.AbsoluteIdx != expected {
				t.Errorf("after collapse, AbsoluteIdx for %s = %d, want %d", gi.Instance.ID, gi.AbsoluteIdx, expected)
			}
		}
	}
}

func TestGroupedInstance_AbsoluteIdx_OutOfOrder(t *testing.T) {
	// Test that AbsoluteIdx reflects session.Instances order, not group order
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusCompleted}, // index 0
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusWorking},   // index 1
			{ID: "inst-3", Task: "Task 3", Status: orchestrator.StatusPending},   // index 2
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Instances: []string{"inst-3", "inst-1"}, // Reverse order from session.Instances
			},
		},
	}

	state := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, state)

	// AbsoluteIdx should reflect position in session.Instances, not group order
	// inst-2 is ungrouped and should appear first with AbsoluteIdx=1
	expectedAbsoluteIdxs := map[string]int{
		"inst-1": 0, // First in session.Instances
		"inst-2": 1, // Second in session.Instances (ungrouped)
		"inst-3": 2, // Third in session.Instances
	}

	for _, item := range items {
		if gi, ok := item.(GroupedInstance); ok {
			expected := expectedAbsoluteIdxs[gi.Instance.ID]
			if gi.AbsoluteIdx != expected {
				t.Errorf("AbsoluteIdx for %s = %d, want %d (session.Instances position)", gi.Instance.ID, gi.AbsoluteIdx, expected)
			}
		}
	}
}

func TestFlattenGroupsForDisplay_UngroupedInstances(t *testing.T) {
	// Test that ungrouped instances appear in the sidebar along with grouped instances
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Grouped Task 1", Status: orchestrator.StatusCompleted},
			{ID: "inst-2", Task: "Ungrouped Task 1", Status: orchestrator.StatusWorking},
			{ID: "inst-3", Task: "Grouped Task 2", Status: orchestrator.StatusPending},
			{ID: "inst-4", Task: "Ungrouped Task 2", Status: orchestrator.StatusPending},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "tripleshot-1",
				Name:      "Tripleshot",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-1", "inst-3"}, // Only inst-1 and inst-3 are grouped
			},
		},
	}

	state := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, state)

	// Expected order:
	// 1. GroupedInstance (inst-2) - ungrouped, shown first
	// 2. GroupedInstance (inst-4) - ungrouped, shown first
	// 3. GroupHeader (tripleshot-1)
	// 4. GroupedInstance (inst-1) - in group
	// 5. GroupedInstance (inst-3) - in group

	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}

	// First two should be ungrouped instances with Depth=-1
	gi0, ok := items[0].(GroupedInstance)
	if !ok {
		t.Error("item 0 should be GroupedInstance")
	} else {
		if gi0.Instance.ID != "inst-2" {
			t.Errorf("item 0 should be inst-2, got %s", gi0.Instance.ID)
		}
		if gi0.Depth != -1 {
			t.Errorf("ungrouped instance should have Depth=-1, got %d", gi0.Depth)
		}
		if gi0.GroupID != "" {
			t.Errorf("ungrouped instance should have empty GroupID, got %q", gi0.GroupID)
		}
	}

	gi1, ok := items[1].(GroupedInstance)
	if !ok {
		t.Error("item 1 should be GroupedInstance")
	} else {
		if gi1.Instance.ID != "inst-4" {
			t.Errorf("item 1 should be inst-4, got %s", gi1.Instance.ID)
		}
		if gi1.Depth != -1 {
			t.Errorf("ungrouped instance should have Depth=-1, got %d", gi1.Depth)
		}
	}

	// Third should be group header
	if _, ok := items[2].(GroupHeaderItem); !ok {
		t.Error("item 2 should be GroupHeaderItem")
	}

	// Fourth and fifth should be grouped instances with Depth >= 0
	gi3, ok := items[3].(GroupedInstance)
	if !ok {
		t.Error("item 3 should be GroupedInstance")
	} else {
		if gi3.Instance.ID != "inst-1" {
			t.Errorf("item 3 should be inst-1, got %s", gi3.Instance.ID)
		}
		if gi3.Depth < 0 {
			t.Errorf("grouped instance should have Depth >= 0, got %d", gi3.Depth)
		}
		if gi3.GroupID != "tripleshot-1" {
			t.Errorf("grouped instance should have GroupID=tripleshot-1, got %q", gi3.GroupID)
		}
	}
}

func TestFlattenGroupsForDisplay_OnlyUngroupedInstances(t *testing.T) {
	// Test with no groups - should still show all instances
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusWorking},
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusPending},
		},
		Groups: nil,
	}

	state := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, state)

	// With no groups, instances should still be shown (as ungrouped)
	if len(items) != 2 {
		t.Fatalf("expected 2 items for ungrouped instances, got %d", len(items))
	}

	for i, item := range items {
		gi, ok := item.(GroupedInstance)
		if !ok {
			t.Errorf("item %d should be GroupedInstance", i)
			continue
		}
		if gi.Depth != -1 {
			t.Errorf("item %d should have Depth=-1 (ungrouped), got %d", i, gi.Depth)
		}
	}
}

func TestGetVisibleInstanceCount_WithUngroupedInstances(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1"},
			{ID: "inst-2"},
			{ID: "inst-3"},
			{ID: "inst-4"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Instances: []string{"inst-1", "inst-2"},
			},
		},
	}

	// inst-3 and inst-4 are ungrouped
	state := NewGroupViewState()

	t.Run("all expanded with ungrouped", func(t *testing.T) {
		count := GetVisibleInstanceCount(session, state)
		// Should count 2 grouped + 2 ungrouped = 4
		if count != 4 {
			t.Errorf("expected 4 visible instances, got %d", count)
		}
	})

	t.Run("group collapsed with ungrouped", func(t *testing.T) {
		state := NewGroupViewState()
		state.CollapsedGroups["group-1"] = true
		count := GetVisibleInstanceCount(session, state)
		// Should count 0 grouped (collapsed) + 2 ungrouped = 2
		if count != 2 {
			t.Errorf("expected 2 visible instances (group collapsed), got %d", count)
		}
	})
}

func TestFindInstanceByGlobalIndex_WithUngroupedInstances(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Grouped 1"},
			{ID: "inst-2", Task: "Ungrouped 1"},
			{ID: "inst-3", Task: "Grouped 2"},
			{ID: "inst-4", Task: "Ungrouped 2"},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Instances: []string{"inst-1", "inst-3"},
			},
		},
	}

	state := NewGroupViewState()

	// Global indices: 0=inst-2 (ungrouped), 1=inst-4 (ungrouped), 2=inst-1 (grouped), 3=inst-3 (grouped)
	tests := []struct {
		idx    int
		wantID string
	}{
		{0, "inst-2"}, // First ungrouped
		{1, "inst-4"}, // Second ungrouped
		{2, "inst-1"}, // First grouped
		{3, "inst-3"}, // Second grouped
	}

	for _, tt := range tests {
		t.Run(tt.wantID, func(t *testing.T) {
			inst := FindInstanceByGlobalIndex(session, state, tt.idx)
			if inst == nil {
				t.Errorf("FindInstanceByGlobalIndex(%d) = nil, want %s", tt.idx, tt.wantID)
			} else if inst.ID != tt.wantID {
				t.Errorf("FindInstanceByGlobalIndex(%d) = %s, want %s", tt.idx, inst.ID, tt.wantID)
			}
		})
	}
}

func TestSidebarView_GroupedModeShowsUngroupedInstances(t *testing.T) {
	// This is the main test for the bug fix - tripleshot mode should show ungrouped instances
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Tripleshot Task 1", Status: orchestrator.StatusWorking},
			{ID: "inst-2", Task: "Tripleshot Task 2", Status: orchestrator.StatusPending},
			{ID: "inst-3", Task: "Regular Instance", Status: orchestrator.StatusWorking},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:          "tripleshot-group",
				Name:        "Tripleshot",
				Phase:       orchestrator.GroupPhaseExecuting,
				SessionType: orchestrator.SessionTypeTripleShot,
				Instances:   []string{"inst-1", "inst-2"},
			},
		},
	}

	state := &mockSidebarState{
		session:        session,
		activeTab:      2, // inst-3 is active (the ungrouped one)
		terminalWidth:  80,
		terminalHeight: 30,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: NewGroupViewState(),
	}

	sv := NewSidebarView()
	result := sv.RenderSidebar(state, 40, 25)

	// Should contain the ungrouped instance
	if !strings.Contains(result, "Regular Instance") {
		t.Errorf("grouped mode should show ungrouped instance 'Regular Instance', got:\n%s", result)
	}

	// Should also contain tripleshot group and its instances
	if !strings.Contains(result, "Tripleshot") {
		t.Errorf("grouped mode should show tripleshot group, got:\n%s", result)
	}
	if !strings.Contains(result, "Tripleshot Task 1") {
		t.Errorf("grouped mode should show tripleshot instance 'Tripleshot Task 1', got:\n%s", result)
	}
}

func TestRenderGroupedInstance_UngroupedInstance(t *testing.T) {
	// Test that ungrouped instances (Depth=-1) render without tree connectors
	gi := GroupedInstance{
		Instance: &orchestrator.Instance{
			ID:     "inst-1",
			Task:   "Ungrouped Task",
			Status: orchestrator.StatusWorking,
		},
		GroupID:     "", // Ungrouped
		Depth:       -1, // Special ungrouped depth
		IsLast:      true,
		GlobalIdx:   0,
		AbsoluteIdx: 0,
	}

	result := RenderGroupedInstance(gi, false, false, 40)

	// Should not contain tree connector characters (vertical line or corner)
	if strings.Contains(result, "\u2502") || strings.Contains(result, "\u2514") {
		t.Errorf("ungrouped instance should not have tree connectors, got: %s", result)
	}

	// Should contain the instance number
	if !strings.Contains(result, "[1]") {
		t.Errorf("should contain instance number [1], got: %s", result)
	}

	// Should contain the task name
	if !strings.Contains(result, "Ungrouped Task") {
		t.Errorf("should contain task name, got: %s", result)
	}

	// Should contain status abbreviation
	if !strings.Contains(result, "WORK") {
		t.Errorf("should contain status abbreviation WORK, got: %s", result)
	}
}

func TestSidebarView_GroupedModeNoHighlightWhenAddingTask(t *testing.T) {
	// Regression test: when adding a task, no instance should be highlighted
	// even though activeTab points to an existing instance
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1", Status: orchestrator.StatusWorking},
			{ID: "inst-2", Task: "Task 2", Status: orchestrator.StatusPending},
		},
		Groups: []*orchestrator.InstanceGroup{
			{
				ID:        "group-1",
				Name:      "Test Group",
				Phase:     orchestrator.GroupPhaseExecuting,
				Instances: []string{"inst-1", "inst-2"},
			},
		},
	}

	groupState := NewGroupViewState()

	// Test with isAddingTask=true: no instance should be highlighted
	stateAddingTask := &mockSidebarState{
		session:        session,
		activeTab:      0, // Points to inst-1, but should NOT be highlighted
		terminalWidth:  80,
		terminalHeight: 30,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: groupState,
		isAddingTask:   true,
	}

	sv := NewSidebarView()
	resultAddingTask := sv.RenderSidebar(stateAddingTask, 40, 25)

	if !strings.Contains(resultAddingTask, "New Task") {
		t.Errorf("should show 'New Task' when adding task, got:\n%s", resultAddingTask)
	}
	if !strings.Contains(resultAddingTask, "Task 1") {
		t.Errorf("should show instance Task 1, got:\n%s", resultAddingTask)
	}

	// Test with isAddingTask=false: instance should be highlighted
	stateNotAddingTask := &mockSidebarState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
		sidebarMode:    SidebarModeGrouped,
		groupViewState: groupState,
		isAddingTask:   false,
	}

	resultNotAddingTask := sv.RenderSidebar(stateNotAddingTask, 40, 25)

	if strings.Contains(resultNotAddingTask, "New Task") {
		t.Errorf("should NOT show 'New Task' when not adding task, got:\n%s", resultNotAddingTask)
	}

	// Verify active vs inactive styling by comparing RenderGroupedInstance output directly.
	// When adding task, Task 1 should be rendered as inactive (isActive=false).
	// When not adding task, Task 1 should be rendered as active (isActive=true).
	gi := GroupedInstance{
		Instance:    session.Instances[0],
		GroupID:     "group-1",
		Depth:       0,
		IsLast:      false,
		GlobalIdx:   0,
		AbsoluteIdx: 0,
	}
	activeRendered := RenderGroupedInstance(gi, true, false, 40)
	inactiveRendered := RenderGroupedInstance(gi, false, false, 40)

	if !strings.Contains(resultNotAddingTask, activeRendered) {
		t.Errorf("when not adding task, expected Task 1 with active styling")
	}
	if !strings.Contains(resultAddingTask, inactiveRendered) {
		t.Errorf("when adding task, expected Task 1 with inactive styling")
	}
}

func TestRenderGroupedInstance_GroupedInstance(t *testing.T) {
	// Test that grouped instances (Depth>=0) render with tree connectors
	gi := GroupedInstance{
		Instance: &orchestrator.Instance{
			ID:     "inst-1",
			Task:   "Grouped Task",
			Status: orchestrator.StatusCompleted,
		},
		GroupID:     "group-1",
		Depth:       0, // Top-level group instance
		IsLast:      false,
		GlobalIdx:   0,
		AbsoluteIdx: 0,
	}

	result := RenderGroupedInstance(gi, false, false, 40)

	// Should contain tree connector (vertical line for non-last)
	if !strings.Contains(result, "\u2502") {
		t.Errorf("grouped non-last instance should have vertical line connector, got: %s", result)
	}

	// Test last instance in group (should have corner connector)
	gi.IsLast = true
	result = RenderGroupedInstance(gi, false, false, 40)

	if !strings.Contains(result, "\u2514") {
		t.Errorf("grouped last instance should have corner connector, got: %s", result)
	}
}

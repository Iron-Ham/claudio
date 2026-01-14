package group

import (
	"sync"
	"testing"
)

// mockManagerSession implements ManagerSessionData for testing.
type mockManagerSession struct {
	groups    []*InstanceGroup
	idCounter int
	mu        sync.Mutex
}

func newMockManagerSession() *mockManagerSession {
	return &mockManagerSession{
		groups: make([]*InstanceGroup, 0),
	}
}

func (m *mockManagerSession) GetGroups() []*InstanceGroup {
	return m.groups
}

func (m *mockManagerSession) SetGroups(groups []*InstanceGroup) {
	m.groups = groups
}

func (m *mockManagerSession) GenerateID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.idCounter++
	return "test-id-" + string(rune('0'+m.idCounter))
}

func TestNewManager(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
	if manager.session != session {
		t.Error("manager.session does not match provided session")
	}
}

func TestNewManager_NilSession(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when session is nil")
		}
	}()
	NewManager(nil)
}

func TestCreateGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	instances := []string{"inst-1", "inst-2"}
	group := manager.CreateGroup("Test Group", instances)

	if group == nil {
		t.Fatal("CreateGroup returned nil")
	}
	if group.Name != "Test Group" {
		t.Errorf("group.Name = %q, want %q", group.Name, "Test Group")
	}
	if group.Phase != GroupPhasePending {
		t.Errorf("group.Phase = %q, want %q", group.Phase, GroupPhasePending)
	}
	if len(group.Instances) != 2 {
		t.Errorf("len(group.Instances) = %d, want 2", len(group.Instances))
	}
	if group.ExecutionOrder != 0 {
		t.Errorf("group.ExecutionOrder = %d, want 0", group.ExecutionOrder)
	}
	if group.Created.IsZero() {
		t.Error("group.Created should not be zero")
	}

	// Verify group is added to session
	groups := session.GetGroups()
	if len(groups) != 1 {
		t.Errorf("len(groups) = %d, want 1", len(groups))
	}
}

func TestCreateGroup_ExecutionOrder(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)
	group3 := manager.CreateGroup("Group 3", nil)

	if group1.ExecutionOrder != 0 {
		t.Errorf("group1.ExecutionOrder = %d, want 0", group1.ExecutionOrder)
	}
	if group2.ExecutionOrder != 1 {
		t.Errorf("group2.ExecutionOrder = %d, want 1", group2.ExecutionOrder)
	}
	if group3.ExecutionOrder != 2 {
		t.Errorf("group3.ExecutionOrder = %d, want 2", group3.ExecutionOrder)
	}
}

func TestCreateGroup_InstancesCopied(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	instances := []string{"inst-1", "inst-2"}
	group := manager.CreateGroup("Test Group", instances)

	// Modify original slice
	instances[0] = "modified"

	// Group should have original values
	if group.Instances[0] != "inst-1" {
		t.Error("group.Instances was not copied - shares memory with input")
	}
}

func TestCreateSubGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", nil)
	subGroup := manager.CreateSubGroup(parent.ID, "SubGroup", []string{"inst-1"})

	if subGroup == nil {
		t.Fatal("CreateSubGroup returned nil")
	}
	if subGroup.Name != "SubGroup" {
		t.Errorf("subGroup.Name = %q, want %q", subGroup.Name, "SubGroup")
	}
	if subGroup.ParentID != parent.ID {
		t.Errorf("subGroup.ParentID = %q, want %q", subGroup.ParentID, parent.ID)
	}
	if len(parent.SubGroups) != 1 {
		t.Errorf("len(parent.SubGroups) = %d, want 1", len(parent.SubGroups))
	}
	if parent.SubGroups[0] != subGroup {
		t.Error("subGroup not added to parent.SubGroups")
	}
}

func TestCreateSubGroup_NonExistentParent(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	subGroup := manager.CreateSubGroup("non-existent", "SubGroup", nil)
	if subGroup != nil {
		t.Error("CreateSubGroup should return nil for non-existent parent")
	}
}

func TestCreateSubGroup_ExecutionOrder(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", nil)
	sub1 := manager.CreateSubGroup(parent.ID, "Sub1", nil)
	sub2 := manager.CreateSubGroup(parent.ID, "Sub2", nil)
	sub3 := manager.CreateSubGroup(parent.ID, "Sub3", nil)

	if sub1.ExecutionOrder != 0 {
		t.Errorf("sub1.ExecutionOrder = %d, want 0", sub1.ExecutionOrder)
	}
	if sub2.ExecutionOrder != 1 {
		t.Errorf("sub2.ExecutionOrder = %d, want 1", sub2.ExecutionOrder)
	}
	if sub3.ExecutionOrder != 2 {
		t.Errorf("sub3.ExecutionOrder = %d, want 2", sub3.ExecutionOrder)
	}
}

func TestMoveInstanceToGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", []string{"inst-1", "inst-2"})
	group2 := manager.CreateGroup("Group 2", nil)

	manager.MoveInstanceToGroup("inst-1", group2.ID)

	// inst-1 should be removed from group1
	if len(group1.Instances) != 1 || group1.Instances[0] != "inst-2" {
		t.Errorf("inst-1 not removed from group1: %v", group1.Instances)
	}

	// inst-1 should be added to group2
	if len(group2.Instances) != 1 || group2.Instances[0] != "inst-1" {
		t.Errorf("inst-1 not added to group2: %v", group2.Instances)
	}
}

func TestMoveInstanceToGroup_NonExistentTarget(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", []string{"inst-1"})

	// This should do nothing
	manager.MoveInstanceToGroup("inst-1", "non-existent")

	// inst-1 should still be in group1
	if len(group1.Instances) != 1 || group1.Instances[0] != "inst-1" {
		t.Errorf("inst-1 was moved when target doesn't exist: %v", group1.Instances)
	}
}

func TestMoveInstanceToGroup_Ungrouped(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group := manager.CreateGroup("Group", nil)

	// Move an instance that isn't in any group
	manager.MoveInstanceToGroup("new-inst", group.ID)

	if len(group.Instances) != 1 || group.Instances[0] != "new-inst" {
		t.Errorf("ungrouped instance not added: %v", group.Instances)
	}
}

func TestMoveInstanceToGroup_FromSubGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", nil)
	subGroup := manager.CreateSubGroup(parent.ID, "SubGroup", []string{"inst-1"})
	target := manager.CreateGroup("Target", nil)

	manager.MoveInstanceToGroup("inst-1", target.ID)

	// inst-1 should be removed from subGroup
	if len(subGroup.Instances) != 0 {
		t.Errorf("inst-1 not removed from subGroup: %v", subGroup.Instances)
	}

	// inst-1 should be added to target
	if len(target.Instances) != 1 || target.Instances[0] != "inst-1" {
		t.Errorf("inst-1 not added to target: %v", target.Instances)
	}
}

func TestGetGroupForInstance(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", []string{"inst-1", "inst-2"})
	manager.CreateGroup("Group 2", []string{"inst-3"})

	found := manager.GetGroupForInstance("inst-1")
	if found == nil {
		t.Fatal("GetGroupForInstance returned nil")
	}
	if found.ID != group1.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, group1.ID)
	}
}

func TestGetGroupForInstance_NotFound(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	manager.CreateGroup("Group", []string{"inst-1"})

	found := manager.GetGroupForInstance("non-existent")
	if found != nil {
		t.Error("GetGroupForInstance should return nil for non-existent instance")
	}
}

func TestGetGroupForInstance_InSubGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", []string{"inst-1"})
	subGroup := manager.CreateSubGroup(parent.ID, "SubGroup", []string{"inst-2"})

	found := manager.GetGroupForInstance("inst-2")
	if found == nil {
		t.Fatal("GetGroupForInstance returned nil")
	}
	if found.ID != subGroup.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, subGroup.ID)
	}
}

func TestAdvanceGroupPhase(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group := manager.CreateGroup("Group", nil)

	if group.Phase != GroupPhasePending {
		t.Fatalf("initial phase = %q, want %q", group.Phase, GroupPhasePending)
	}

	manager.AdvanceGroupPhase(group.ID, GroupPhaseExecuting)
	if group.Phase != GroupPhaseExecuting {
		t.Errorf("phase after advance = %q, want %q", group.Phase, GroupPhaseExecuting)
	}

	manager.AdvanceGroupPhase(group.ID, GroupPhaseCompleted)
	if group.Phase != GroupPhaseCompleted {
		t.Errorf("phase after second advance = %q, want %q", group.Phase, GroupPhaseCompleted)
	}
}

func TestAdvanceGroupPhase_NonExistent(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	// Should not panic
	manager.AdvanceGroupPhase("non-existent", GroupPhaseExecuting)
}

func TestAdvanceGroupPhase_SubGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", nil)
	subGroup := manager.CreateSubGroup(parent.ID, "SubGroup", nil)

	manager.AdvanceGroupPhase(subGroup.ID, GroupPhaseExecuting)
	if subGroup.Phase != GroupPhaseExecuting {
		t.Errorf("subGroup.Phase = %q, want %q", subGroup.Phase, GroupPhaseExecuting)
	}
}

func TestReorderGroups(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)
	group3 := manager.CreateGroup("Group 3", nil)

	// Reorder: 3, 1, 2
	manager.ReorderGroups([]string{group3.ID, group1.ID, group2.ID})

	groups := manager.GetAllGroups()
	if groups[0].ID != group3.ID {
		t.Errorf("groups[0].ID = %q, want %q", groups[0].ID, group3.ID)
	}
	if groups[1].ID != group1.ID {
		t.Errorf("groups[1].ID = %q, want %q", groups[1].ID, group1.ID)
	}
	if groups[2].ID != group2.ID {
		t.Errorf("groups[2].ID = %q, want %q", groups[2].ID, group2.ID)
	}

	// Check execution orders updated
	if groups[0].ExecutionOrder != 0 {
		t.Errorf("groups[0].ExecutionOrder = %d, want 0", groups[0].ExecutionOrder)
	}
	if groups[1].ExecutionOrder != 1 {
		t.Errorf("groups[1].ExecutionOrder = %d, want 1", groups[1].ExecutionOrder)
	}
	if groups[2].ExecutionOrder != 2 {
		t.Errorf("groups[2].ExecutionOrder = %d, want 2", groups[2].ExecutionOrder)
	}
}

func TestReorderGroups_PartialOrder(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)
	group3 := manager.CreateGroup("Group 3", nil)

	// Only specify group3 - others should be appended
	manager.ReorderGroups([]string{group3.ID})

	groups := manager.GetAllGroups()
	if groups[0].ID != group3.ID {
		t.Errorf("groups[0].ID = %q, want %q", groups[0].ID, group3.ID)
	}
	// group1 and group2 should follow in original order
	if groups[1].ID != group1.ID {
		t.Errorf("groups[1].ID = %q, want %q", groups[1].ID, group1.ID)
	}
	if groups[2].ID != group2.ID {
		t.Errorf("groups[2].ID = %q, want %q", groups[2].ID, group2.ID)
	}
}

func TestReorderGroups_NonExistentIDs(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)

	// Include non-existent ID
	manager.ReorderGroups([]string{"non-existent", group2.ID, group1.ID})

	groups := manager.GetAllGroups()
	// Non-existent should be ignored
	if groups[0].ID != group2.ID {
		t.Errorf("groups[0].ID = %q, want %q", groups[0].ID, group2.ID)
	}
	if groups[1].ID != group1.ID {
		t.Errorf("groups[1].ID = %q, want %q", groups[1].ID, group1.ID)
	}
}

func TestReorderGroups_Empty(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	// Should not panic
	manager.ReorderGroups([]string{})
}

func TestReorderGroups_DuplicateIDs(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)

	// Duplicate group1
	manager.ReorderGroups([]string{group1.ID, group1.ID, group2.ID})

	groups := manager.GetAllGroups()
	// Should only appear once
	if len(groups) != 2 {
		t.Errorf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ID != group1.ID {
		t.Errorf("groups[0].ID = %q, want %q", groups[0].ID, group1.ID)
	}
	if groups[1].ID != group2.ID {
		t.Errorf("groups[1].ID = %q, want %q", groups[1].ID, group2.ID)
	}
}

func TestFlattenGroups(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	manager.CreateGroup("Group 1", []string{"inst-1", "inst-2"})
	manager.CreateGroup("Group 2", []string{"inst-3"})

	flat := manager.FlattenGroups()

	expected := []string{"inst-1", "inst-2", "inst-3"}
	if len(flat) != len(expected) {
		t.Fatalf("len(flat) = %d, want %d", len(flat), len(expected))
	}
	for i, id := range expected {
		if flat[i] != id {
			t.Errorf("flat[%d] = %q, want %q", i, flat[i], id)
		}
	}
}

func TestFlattenGroups_WithSubGroups(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", []string{"inst-1"})
	manager.CreateSubGroup(parent.ID, "SubGroup", []string{"inst-2", "inst-3"})

	flat := manager.FlattenGroups()

	// Parent instances first, then sub-group instances
	expected := []string{"inst-1", "inst-2", "inst-3"}
	if len(flat) != len(expected) {
		t.Fatalf("len(flat) = %d, want %d", len(flat), len(expected))
	}
	for i, id := range expected {
		if flat[i] != id {
			t.Errorf("flat[%d] = %q, want %q", i, flat[i], id)
		}
	}
}

func TestFlattenGroups_RespectsExecutionOrder(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", []string{"inst-1"})
	group2 := manager.CreateGroup("Group 2", []string{"inst-2"})
	manager.CreateGroup("Group 3", []string{"inst-3"})

	// Reorder to: 2, 3, 1
	manager.ReorderGroups([]string{group2.ID, session.groups[2].ID, group1.ID})

	flat := manager.FlattenGroups()

	expected := []string{"inst-2", "inst-3", "inst-1"}
	if len(flat) != len(expected) {
		t.Fatalf("len(flat) = %d, want %d", len(flat), len(expected))
	}
	for i, id := range expected {
		if flat[i] != id {
			t.Errorf("flat[%d] = %q, want %q", i, flat[i], id)
		}
	}
}

func TestFlattenGroups_Empty(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	flat := manager.FlattenGroups()
	if flat != nil {
		t.Errorf("FlattenGroups on empty session = %v, want nil", flat)
	}
}

func TestFlattenGroups_NestedSubGroups(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", []string{"inst-1"})
	sub1 := manager.CreateSubGroup(parent.ID, "Sub1", []string{"inst-2"})
	manager.CreateSubGroup(sub1.ID, "Sub1-1", []string{"inst-3"})

	flat := manager.FlattenGroups()

	// Order: parent instances, sub1 instances, sub1-1 instances
	expected := []string{"inst-1", "inst-2", "inst-3"}
	if len(flat) != len(expected) {
		t.Fatalf("len(flat) = %d, want %d", len(flat), len(expected))
	}
	for i, id := range expected {
		if flat[i] != id {
			t.Errorf("flat[%d] = %q, want %q", i, flat[i], id)
		}
	}
}

func TestGetGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group := manager.CreateGroup("Group", nil)
	found := manager.GetGroup(group.ID)

	if found == nil {
		t.Fatal("GetGroup returned nil")
	}
	if found.ID != group.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, group.ID)
	}
}

func TestGetGroup_SubGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", nil)
	subGroup := manager.CreateSubGroup(parent.ID, "SubGroup", nil)

	found := manager.GetGroup(subGroup.ID)
	if found == nil {
		t.Fatal("GetGroup returned nil for sub-group")
	}
	if found.ID != subGroup.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, subGroup.ID)
	}
}

func TestGetGroup_NotFound(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	found := manager.GetGroup("non-existent")
	if found != nil {
		t.Error("GetGroup should return nil for non-existent ID")
	}
}

func TestGetAllGroups(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)

	groups := manager.GetAllGroups()

	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}
	if groups[0].ID != group1.ID {
		t.Errorf("groups[0].ID = %q, want %q", groups[0].ID, group1.ID)
	}
	if groups[1].ID != group2.ID {
		t.Errorf("groups[1].ID = %q, want %q", groups[1].ID, group2.ID)
	}
}

func TestGetAllGroups_Empty(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	groups := manager.GetAllGroups()
	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}

func TestRemoveGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group := manager.CreateGroup("Group", nil)

	removed := manager.RemoveGroup(group.ID)
	if !removed {
		t.Error("RemoveGroup returned false")
	}

	groups := manager.GetAllGroups()
	if len(groups) != 0 {
		t.Errorf("len(groups) = %d, want 0", len(groups))
	}
}

func TestRemoveGroup_SubGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	parent := manager.CreateGroup("Parent", nil)
	subGroup := manager.CreateSubGroup(parent.ID, "SubGroup", nil)

	removed := manager.RemoveGroup(subGroup.ID)
	if !removed {
		t.Error("RemoveGroup returned false for sub-group")
	}

	if len(parent.SubGroups) != 0 {
		t.Errorf("len(parent.SubGroups) = %d, want 0", len(parent.SubGroups))
	}
}

func TestRemoveGroup_NonExistent(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	removed := manager.RemoveGroup("non-existent")
	if removed {
		t.Error("RemoveGroup should return false for non-existent ID")
	}
}

func TestSetGroupDependencies(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group1 := manager.CreateGroup("Group 1", nil)
	group2 := manager.CreateGroup("Group 2", nil)

	manager.SetGroupDependencies(group2.ID, []string{group1.ID})

	if len(group2.DependsOn) != 1 {
		t.Fatalf("len(group2.DependsOn) = %d, want 1", len(group2.DependsOn))
	}
	if group2.DependsOn[0] != group1.ID {
		t.Errorf("group2.DependsOn[0] = %q, want %q", group2.DependsOn[0], group1.ID)
	}
}

func TestSetGroupDependencies_NonExistent(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	// Should not panic
	manager.SetGroupDependencies("non-existent", []string{"dep-1"})
}

func TestSetGroupDependencies_ReplacesPrevious(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	group := manager.CreateGroup("Group", nil)
	manager.SetGroupDependencies(group.ID, []string{"dep-1", "dep-2"})
	manager.SetGroupDependencies(group.ID, []string{"dep-3"})

	if len(group.DependsOn) != 1 {
		t.Fatalf("len(group.DependsOn) = %d, want 1", len(group.DependsOn))
	}
	if group.DependsOn[0] != "dep-3" {
		t.Errorf("group.DependsOn[0] = %q, want %q", group.DependsOn[0], "dep-3")
	}
}

func TestConcurrency_CreateGroup(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	var wg sync.WaitGroup
	numGroups := 100

	for i := 0; i < numGroups; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			manager.CreateGroup("Group", []string{"inst-" + string(rune('a'+idx%26))})
		}(i)
	}

	wg.Wait()

	groups := manager.GetAllGroups()
	if len(groups) != numGroups {
		t.Errorf("len(groups) = %d, want %d", len(groups), numGroups)
	}
}

func TestConcurrency_MixedOperations(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	// Create initial groups
	group := manager.CreateGroup("Initial", []string{"inst-1", "inst-2", "inst-3"})

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.GetGroupForInstance("inst-1")
			manager.GetAllGroups()
			manager.FlattenGroups()
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			manager.AdvanceGroupPhase(group.ID, GroupPhaseExecuting)
		}()
	}

	wg.Wait()

	// Verify no corruption
	found := manager.GetGroupForInstance("inst-1")
	if found == nil {
		t.Error("instance not found after concurrent operations")
	}
}

func TestDeeplyNestedSubGroups(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	// Create deeply nested structure
	root := manager.CreateGroup("Root", []string{"root-inst"})
	level1 := manager.CreateSubGroup(root.ID, "Level1", []string{"l1-inst"})
	level2 := manager.CreateSubGroup(level1.ID, "Level2", []string{"l2-inst"})
	level3 := manager.CreateSubGroup(level2.ID, "Level3", []string{"l3-inst"})

	// Find instance in deepest level
	found := manager.GetGroupForInstance("l3-inst")
	if found == nil {
		t.Fatal("GetGroupForInstance returned nil for deeply nested instance")
	}
	if found.ID != level3.ID {
		t.Errorf("found.ID = %q, want %q", found.ID, level3.ID)
	}

	// Flatten should include all instances
	flat := manager.FlattenGroups()
	expected := []string{"root-inst", "l1-inst", "l2-inst", "l3-inst"}
	if len(flat) != len(expected) {
		t.Fatalf("len(flat) = %d, want %d", len(flat), len(expected))
	}

	// Find deeply nested group
	foundGroup := manager.GetGroup(level3.ID)
	if foundGroup == nil {
		t.Error("GetGroup returned nil for deeply nested group")
	}

	// Remove deeply nested group
	removed := manager.RemoveGroup(level3.ID)
	if !removed {
		t.Error("RemoveGroup returned false for deeply nested group")
	}
	if len(level2.SubGroups) != 0 {
		t.Error("deeply nested group not removed from parent")
	}
}

func TestComplexScenario_WorkflowSimulation(t *testing.T) {
	session := newMockManagerSession()
	manager := NewManager(session)

	// Simulate a multi-group workflow

	// 1. Create groups for different phases
	foundation := manager.CreateGroup("Foundation", []string{"setup-db", "setup-auth"})
	features := manager.CreateGroup("Features", []string{"feat-1", "feat-2", "feat-3"})
	integration := manager.CreateGroup("Integration", []string{"int-1"})

	// 2. Set dependencies: integration depends on features, features depends on foundation
	manager.SetGroupDependencies(features.ID, []string{foundation.ID})
	manager.SetGroupDependencies(integration.ID, []string{features.ID})

	// 3. Create sub-groups for feature branches
	manager.CreateSubGroup(features.ID, "Feature Subset A", []string{"feat-a1", "feat-a2"})
	manager.CreateSubGroup(features.ID, "Feature Subset B", []string{"feat-b1"})

	// 4. Advance phases
	manager.AdvanceGroupPhase(foundation.ID, GroupPhaseExecuting)

	// 5. Move an instance between groups
	manager.MoveInstanceToGroup("feat-1", integration.ID)

	// Verify final state
	flat := manager.FlattenGroups()
	t.Logf("Flattened order: %v", flat)

	// foundation instances should come first
	if flat[0] != "setup-db" || flat[1] != "setup-auth" {
		t.Error("foundation instances not at start")
	}

	// feat-1 should now be in integration (at end)
	integrationGroup := manager.GetGroup(integration.ID)
	foundInIntegration := false
	for _, id := range integrationGroup.Instances {
		if id == "feat-1" {
			foundInIntegration = true
			break
		}
	}
	if !foundInIntegration {
		t.Error("feat-1 not found in integration group after move")
	}

	// Verify dependencies
	if len(features.DependsOn) != 1 || features.DependsOn[0] != foundation.ID {
		t.Errorf("features.DependsOn = %v, want [%s]", features.DependsOn, foundation.ID)
	}
	if len(integration.DependsOn) != 1 || integration.DependsOn[0] != features.ID {
		t.Errorf("integration.DependsOn = %v, want [%s]", integration.DependsOn, features.ID)
	}
}

func TestRemoveInstanceFromGroup(t *testing.T) {
	t.Run("removes instance and cleans up empty group", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		manager.CreateGroup("Single Instance Group", []string{"inst-1"})

		removed := manager.RemoveInstanceFromGroup("inst-1")
		if !removed {
			t.Error("RemoveInstanceFromGroup should return true when instance found")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 0 {
			t.Errorf("expected 0 groups after removing last instance, got %d", len(groups))
		}
	})

	t.Run("removes instance but keeps group with remaining instances", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		manager.CreateGroup("Multi Instance Group", []string{"inst-1", "inst-2"})

		removed := manager.RemoveInstanceFromGroup("inst-1")
		if !removed {
			t.Error("RemoveInstanceFromGroup should return true when instance found")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 1 {
			t.Errorf("expected 1 group remaining, got %d", len(groups))
		}

		if len(groups[0].Instances) != 1 || groups[0].Instances[0] != "inst-2" {
			t.Errorf("expected only inst-2 to remain, got %v", groups[0].Instances)
		}
	})

	t.Run("removes instance from sub-group and cleans up empty hierarchy", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		parent := manager.CreateGroup("Parent", nil)
		manager.CreateSubGroup(parent.ID, "Child", []string{"inst-1"})

		removed := manager.RemoveInstanceFromGroup("inst-1")
		if !removed {
			t.Error("RemoveInstanceFromGroup should return true when instance found")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 0 {
			t.Errorf("expected 0 groups after removing instance from sub-group, got %d", len(groups))
		}
	})

	t.Run("keeps parent group when sibling sub-group has instances", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		parent := manager.CreateGroup("Parent", nil)
		manager.CreateSubGroup(parent.ID, "Child 1", []string{"inst-1"})
		manager.CreateSubGroup(parent.ID, "Child 2", []string{"inst-2"})

		removed := manager.RemoveInstanceFromGroup("inst-1")
		if !removed {
			t.Error("RemoveInstanceFromGroup should return true when instance found")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 1 {
			t.Errorf("expected 1 top-level group, got %d", len(groups))
		}

		if len(groups[0].SubGroups) != 1 {
			t.Errorf("expected 1 sub-group remaining, got %d", len(groups[0].SubGroups))
		}

		if groups[0].SubGroups[0].Name != "Child 2" {
			t.Errorf("expected Child 2 to remain, got %s", groups[0].SubGroups[0].Name)
		}
	})

	t.Run("returns false for non-existent instance", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		manager.CreateGroup("Group", []string{"inst-1"})

		removed := manager.RemoveInstanceFromGroup("nonexistent")
		if removed {
			t.Error("RemoveInstanceFromGroup should return false for non-existent instance")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 1 || len(groups[0].Instances) != 1 {
			t.Error("group should remain unchanged when removing non-existent instance")
		}
	})

	t.Run("cleans up deeply nested empty hierarchy", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		root := manager.CreateGroup("Root", nil)
		level1 := manager.CreateSubGroup(root.ID, "Level 1", nil)
		level2 := manager.CreateSubGroup(level1.ID, "Level 2", nil)
		manager.CreateSubGroup(level2.ID, "Level 3", []string{"inst-deep"})

		removed := manager.RemoveInstanceFromGroup("inst-deep")
		if !removed {
			t.Error("RemoveInstanceFromGroup should return true for deeply nested instance")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 0 {
			t.Errorf("expected 0 groups after removing last instance from deep hierarchy, got %d", len(groups))
		}
	})

	t.Run("keeps parent with direct instances when sub-group becomes empty", func(t *testing.T) {
		session := newMockManagerSession()
		manager := NewManager(session)

		parent := manager.CreateGroup("Parent", []string{"parent-inst"})
		manager.CreateSubGroup(parent.ID, "Child", []string{"child-inst"})

		removed := manager.RemoveInstanceFromGroup("child-inst")
		if !removed {
			t.Error("RemoveInstanceFromGroup should return true")
		}

		groups := manager.GetAllGroups()
		if len(groups) != 1 {
			t.Errorf("expected 1 group remaining, got %d", len(groups))
		}

		if len(groups[0].SubGroups) != 0 {
			t.Error("empty sub-group should be removed")
		}

		if len(groups[0].Instances) != 1 || groups[0].Instances[0] != "parent-inst" {
			t.Error("parent instance should remain")
		}
	})
}

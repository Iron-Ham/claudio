package orchestrator

import (
	"testing"
	"time"
)

func TestGroupPhase_Constants(t *testing.T) {
	tests := []struct {
		phase    GroupPhase
		expected string
	}{
		{GroupPhasePending, "pending"},
		{GroupPhaseExecuting, "executing"},
		{GroupPhaseCompleted, "completed"},
		{GroupPhaseFailed, "failed"},
	}

	for _, tt := range tests {
		if string(tt.phase) != tt.expected {
			t.Errorf("GroupPhase constant = %q, want %q", tt.phase, tt.expected)
		}
	}
}

func TestNewInstanceGroup(t *testing.T) {
	tests := []struct {
		name         string
		groupName    string
		expectedName string
	}{
		{
			name:         "with custom name",
			groupName:    "Group 1: Foundation",
			expectedName: "Group 1: Foundation",
		},
		{
			name:         "with empty name",
			groupName:    "",
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := NewInstanceGroup(tt.groupName)

			if group == nil {
				t.Fatal("NewInstanceGroup returned nil")
			}

			if group.ID == "" {
				t.Error("group ID should not be empty")
			}

			if group.Name != tt.expectedName {
				t.Errorf("group.Name = %q, want %q", group.Name, tt.expectedName)
			}

			if group.Phase != GroupPhasePending {
				t.Errorf("group.Phase = %q, want %q", group.Phase, GroupPhasePending)
			}

			if group.Instances == nil {
				t.Error("group.Instances should not be nil")
			}

			if len(group.Instances) != 0 {
				t.Errorf("group.Instances should be empty, got %d", len(group.Instances))
			}

			if group.SubGroups == nil {
				t.Error("group.SubGroups should not be nil")
			}

			if group.DependsOn == nil {
				t.Error("group.DependsOn should not be nil")
			}

			if group.Created.IsZero() {
				t.Error("group.Created should be set")
			}
		})
	}
}

func TestInstanceGroup_Created_Timestamp(t *testing.T) {
	before := time.Now()
	group := NewInstanceGroup("test group")
	after := time.Now()

	if group.Created.Before(before) || group.Created.After(after) {
		t.Errorf("group.Created = %v, should be between %v and %v", group.Created, before, after)
	}
}

func TestInstanceGroup_AddInstance(t *testing.T) {
	group := NewInstanceGroup("test group")

	group.AddInstance("inst-1")
	group.AddInstance("inst-2")
	group.AddInstance("inst-3")

	if len(group.Instances) != 3 {
		t.Errorf("len(group.Instances) = %d, want 3", len(group.Instances))
	}

	expected := []string{"inst-1", "inst-2", "inst-3"}
	for i, id := range expected {
		if group.Instances[i] != id {
			t.Errorf("group.Instances[%d] = %q, want %q", i, group.Instances[i], id)
		}
	}
}

func TestInstanceGroup_RemoveInstance(t *testing.T) {
	group := NewInstanceGroup("test group")
	group.AddInstance("inst-1")
	group.AddInstance("inst-2")
	group.AddInstance("inst-3")

	tests := []struct {
		name        string
		removeID    string
		expectedLen int
		expectedIDs []string
	}{
		{
			name:        "remove middle instance",
			removeID:    "inst-2",
			expectedLen: 2,
			expectedIDs: []string{"inst-1", "inst-3"},
		},
		{
			name:        "remove first instance",
			removeID:    "inst-1",
			expectedLen: 1,
			expectedIDs: []string{"inst-3"},
		},
		{
			name:        "remove non-existent instance",
			removeID:    "inst-999",
			expectedLen: 1,
			expectedIDs: []string{"inst-3"},
		},
		{
			name:        "remove last instance",
			removeID:    "inst-3",
			expectedLen: 0,
			expectedIDs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group.RemoveInstance(tt.removeID)
			if len(group.Instances) != tt.expectedLen {
				t.Errorf("len(group.Instances) = %d, want %d", len(group.Instances), tt.expectedLen)
			}
			for i, id := range tt.expectedIDs {
				if i >= len(group.Instances) || group.Instances[i] != id {
					t.Errorf("after removing %q, expected Instances[%d] = %q", tt.removeID, i, id)
				}
			}
		})
	}
}

func TestInstanceGroup_HasInstance(t *testing.T) {
	group := NewInstanceGroup("test group")
	group.AddInstance("inst-1")
	group.AddInstance("inst-2")

	tests := []struct {
		name       string
		instanceID string
		expected   bool
	}{
		{"has first instance", "inst-1", true},
		{"has second instance", "inst-2", true},
		{"does not have instance", "inst-3", false},
		{"empty id", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := group.HasInstance(tt.instanceID)
			if result != tt.expected {
				t.Errorf("HasInstance(%q) = %v, want %v", tt.instanceID, result, tt.expected)
			}
		})
	}
}

func TestInstanceGroup_SubGroups(t *testing.T) {
	parent := NewInstanceGroup("Parent Group")
	child1 := NewInstanceGroup("Child 1")
	child2 := NewInstanceGroup("Child 2")

	parent.AddSubGroup(child1)
	parent.AddSubGroup(child2)

	if len(parent.SubGroups) != 2 {
		t.Errorf("len(parent.SubGroups) = %d, want 2", len(parent.SubGroups))
	}

	// Verify parent ID is set
	if child1.ParentID != parent.ID {
		t.Errorf("child1.ParentID = %q, want %q", child1.ParentID, parent.ID)
	}
	if child2.ParentID != parent.ID {
		t.Errorf("child2.ParentID = %q, want %q", child2.ParentID, parent.ID)
	}
}

func TestInstanceGroup_GetSubGroup(t *testing.T) {
	parent := NewInstanceGroup("Parent Group")
	child1 := NewInstanceGroup("Child 1")
	child2 := NewInstanceGroup("Child 2")
	child1ID := child1.ID
	child2ID := child2.ID

	parent.AddSubGroup(child1)
	parent.AddSubGroup(child2)

	tests := []struct {
		name     string
		id       string
		expected *InstanceGroup
	}{
		{"find first child", child1ID, child1},
		{"find second child", child2ID, child2},
		{"not found", "nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parent.GetSubGroup(tt.id)
			if result != tt.expected {
				t.Errorf("GetSubGroup(%q) = %v, want %v", tt.id, result, tt.expected)
			}
		})
	}
}

func TestInstanceGroup_AllInstanceIDs(t *testing.T) {
	parent := NewInstanceGroup("Parent")
	parent.AddInstance("inst-1")
	parent.AddInstance("inst-2")

	child := NewInstanceGroup("Child")
	child.AddInstance("inst-3")
	child.AddInstance("inst-4")

	grandchild := NewInstanceGroup("Grandchild")
	grandchild.AddInstance("inst-5")

	child.AddSubGroup(grandchild)
	parent.AddSubGroup(child)

	allIDs := parent.AllInstanceIDs()

	if len(allIDs) != 5 {
		t.Errorf("len(AllInstanceIDs()) = %d, want 5", len(allIDs))
	}

	expectedIDs := map[string]bool{
		"inst-1": true,
		"inst-2": true,
		"inst-3": true,
		"inst-4": true,
		"inst-5": true,
	}

	for _, id := range allIDs {
		if !expectedIDs[id] {
			t.Errorf("unexpected instance ID in AllInstanceIDs(): %q", id)
		}
	}
}

func TestInstanceGroup_InstanceCount(t *testing.T) {
	parent := NewInstanceGroup("Parent")
	parent.AddInstance("inst-1")
	parent.AddInstance("inst-2")

	child := NewInstanceGroup("Child")
	child.AddInstance("inst-3")

	parent.AddSubGroup(child)

	if count := parent.InstanceCount(); count != 3 {
		t.Errorf("InstanceCount() = %d, want 3", count)
	}

	if count := child.InstanceCount(); count != 1 {
		t.Errorf("child.InstanceCount() = %d, want 1", count)
	}
}

func TestInstanceGroup_IsTopLevel(t *testing.T) {
	parent := NewInstanceGroup("Parent")
	child := NewInstanceGroup("Child")

	parent.AddSubGroup(child)

	if !parent.IsTopLevel() {
		t.Error("parent.IsTopLevel() should be true")
	}

	if child.IsTopLevel() {
		t.Error("child.IsTopLevel() should be false")
	}
}

func TestSession_Groups_Field(t *testing.T) {
	session := NewSession("test", "/repo")

	// Groups should be nil initially (optional field)
	if session.Groups != nil {
		t.Errorf("session.Groups should be nil initially, got %v", session.Groups)
	}

	// Create and add groups
	group1 := NewInstanceGroup("Group 1")
	group2 := NewInstanceGroup("Group 2")

	session.AddGroup(group1)
	session.AddGroup(group2)

	if len(session.Groups) != 2 {
		t.Errorf("len(session.Groups) = %d, want 2", len(session.Groups))
	}
}

func TestSession_GetGroup(t *testing.T) {
	session := NewSession("test", "/repo")

	group1 := NewInstanceGroup("Group 1")
	group2 := NewInstanceGroup("Group 2")
	subGroup := NewInstanceGroup("Sub Group")
	group2.AddSubGroup(subGroup)

	session.AddGroup(group1)
	session.AddGroup(group2)

	tests := []struct {
		name     string
		id       string
		expected *InstanceGroup
	}{
		{"find top-level group", group1.ID, group1},
		{"find another top-level group", group2.ID, group2},
		{"find sub-group", subGroup.ID, subGroup},
		{"not found", "nonexistent", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := session.GetGroup(tt.id)
			if result != tt.expected {
				t.Errorf("GetGroup(%q) = %v, want %v", tt.id, result, tt.expected)
			}
		})
	}
}

func TestSession_GetGroupForInstance(t *testing.T) {
	session := NewSession("test", "/repo")

	group1 := NewInstanceGroup("Group 1")
	group1.AddInstance("inst-1")
	group1.AddInstance("inst-2")

	group2 := NewInstanceGroup("Group 2")
	group2.AddInstance("inst-3")

	subGroup := NewInstanceGroup("Sub Group")
	subGroup.AddInstance("inst-4")
	group2.AddSubGroup(subGroup)

	session.AddGroup(group1)
	session.AddGroup(group2)

	tests := []struct {
		name       string
		instanceID string
		expected   *InstanceGroup
	}{
		{"instance in group1", "inst-1", group1},
		{"another instance in group1", "inst-2", group1},
		{"instance in group2", "inst-3", group2},
		{"instance in sub-group", "inst-4", subGroup},
		{"instance not in any group", "inst-999", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := session.GetGroupForInstance(tt.instanceID)
			if result != tt.expected {
				t.Errorf("GetGroupForInstance(%q) = %v, want %v", tt.instanceID, result, tt.expected)
			}
		})
	}
}

func TestSession_RemoveGroup(t *testing.T) {
	session := NewSession("test", "/repo")

	group1 := NewInstanceGroup("Group 1")
	group2 := NewInstanceGroup("Group 2")
	group3 := NewInstanceGroup("Group 3")

	session.AddGroup(group1)
	session.AddGroup(group2)
	session.AddGroup(group3)

	session.RemoveGroup(group2.ID)

	if len(session.Groups) != 2 {
		t.Errorf("len(session.Groups) = %d, want 2", len(session.Groups))
	}

	if session.GetGroup(group2.ID) != nil {
		t.Error("group2 should have been removed")
	}

	// Remove non-existent group (should not panic)
	session.RemoveGroup("nonexistent")
	if len(session.Groups) != 2 {
		t.Error("removing non-existent group should not change length")
	}
}

func TestSession_GetGroupsByPhase(t *testing.T) {
	session := NewSession("test", "/repo")

	group1 := NewInstanceGroup("Group 1")
	group1.Phase = GroupPhasePending

	group2 := NewInstanceGroup("Group 2")
	group2.Phase = GroupPhaseExecuting

	group3 := NewInstanceGroup("Group 3")
	group3.Phase = GroupPhasePending

	group4 := NewInstanceGroup("Group 4")
	group4.Phase = GroupPhaseCompleted

	session.AddGroup(group1)
	session.AddGroup(group2)
	session.AddGroup(group3)
	session.AddGroup(group4)

	tests := []struct {
		name          string
		phase         GroupPhase
		expectedCount int
	}{
		{"pending groups", GroupPhasePending, 2},
		{"executing groups", GroupPhaseExecuting, 1},
		{"completed groups", GroupPhaseCompleted, 1},
		{"failed groups", GroupPhaseFailed, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := session.GetGroupsByPhase(tt.phase)
			if len(groups) != tt.expectedCount {
				t.Errorf("GetGroupsByPhase(%q) returned %d groups, want %d",
					tt.phase, len(groups), tt.expectedCount)
			}
		})
	}
}

func TestSession_AreGroupDependenciesMet(t *testing.T) {
	session := NewSession("test", "/repo")

	group1 := NewInstanceGroup("Group 1")
	group1.Phase = GroupPhaseCompleted

	group2 := NewInstanceGroup("Group 2")
	group2.Phase = GroupPhaseExecuting

	group3 := NewInstanceGroup("Group 3")
	group3.DependsOn = []string{group1.ID, group2.ID}

	group4 := NewInstanceGroup("Group 4")
	group4.DependsOn = []string{group1.ID}

	group5 := NewInstanceGroup("Group 5")
	// No dependencies

	group6 := NewInstanceGroup("Group 6")
	group6.DependsOn = []string{"nonexistent"}

	session.AddGroup(group1)
	session.AddGroup(group2)
	session.AddGroup(group3)
	session.AddGroup(group4)
	session.AddGroup(group5)
	session.AddGroup(group6)

	tests := []struct {
		name     string
		group    *InstanceGroup
		expected bool
	}{
		{"no dependencies", group5, true},
		{"all dependencies completed", group4, true},
		{"some dependencies not completed", group3, false},
		{"dependency on non-existent group", group6, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := session.AreGroupDependenciesMet(tt.group)
			if result != tt.expected {
				t.Errorf("AreGroupDependenciesMet() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSession_GetReadyGroups(t *testing.T) {
	session := NewSession("test", "/repo")

	// Group 1: completed (can't be ready)
	group1 := NewInstanceGroup("Group 1")
	group1.Phase = GroupPhaseCompleted

	// Group 2: pending, no dependencies (ready)
	group2 := NewInstanceGroup("Group 2")
	group2.Phase = GroupPhasePending

	// Group 3: pending, depends on completed group1 (ready)
	group3 := NewInstanceGroup("Group 3")
	group3.Phase = GroupPhasePending
	group3.DependsOn = []string{group1.ID}

	// Group 4: pending, depends on non-completed group2 (not ready)
	group4 := NewInstanceGroup("Group 4")
	group4.Phase = GroupPhasePending
	group4.DependsOn = []string{group2.ID}

	// Group 5: executing (can't be ready)
	group5 := NewInstanceGroup("Group 5")
	group5.Phase = GroupPhaseExecuting

	session.AddGroup(group1)
	session.AddGroup(group2)
	session.AddGroup(group3)
	session.AddGroup(group4)
	session.AddGroup(group5)

	ready := session.GetReadyGroups()

	if len(ready) != 2 {
		t.Errorf("GetReadyGroups() returned %d groups, want 2", len(ready))
	}

	// Verify that group2 and group3 are in the ready list
	foundGroup2, foundGroup3 := false, false
	for _, g := range ready {
		if g.ID == group2.ID {
			foundGroup2 = true
		}
		if g.ID == group3.ID {
			foundGroup3 = true
		}
	}

	if !foundGroup2 {
		t.Error("group2 should be in ready groups")
	}
	if !foundGroup3 {
		t.Error("group3 should be in ready groups")
	}
}

func TestInstanceGroup_ExecutionOrder(t *testing.T) {
	group1 := NewInstanceGroup("Group 1")
	group1.ExecutionOrder = 0

	group2 := NewInstanceGroup("Group 2")
	group2.ExecutionOrder = 1

	group3 := NewInstanceGroup("Group 3")
	group3.ExecutionOrder = 2

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

func TestInstanceGroup_DeepNesting(t *testing.T) {
	// Test deeply nested sub-groups
	root := NewInstanceGroup("Root")
	root.AddInstance("inst-root")

	level1 := NewInstanceGroup("Level 1")
	level1.AddInstance("inst-1")
	root.AddSubGroup(level1)

	level2 := NewInstanceGroup("Level 2")
	level2.AddInstance("inst-2")
	level1.AddSubGroup(level2)

	level3 := NewInstanceGroup("Level 3")
	level3.AddInstance("inst-3")
	level2.AddSubGroup(level3)

	// Test AllInstanceIDs includes all levels
	allIDs := root.AllInstanceIDs()
	if len(allIDs) != 4 {
		t.Errorf("len(AllInstanceIDs()) = %d, want 4", len(allIDs))
	}

	// Test InstanceCount
	if count := root.InstanceCount(); count != 4 {
		t.Errorf("root.InstanceCount() = %d, want 4", count)
	}

	// Test GetSubGroup doesn't find deeply nested groups (only direct children)
	if found := root.GetSubGroup(level3.ID); found != nil {
		t.Error("GetSubGroup should only return direct children")
	}
}

func TestSession_GetGroup_DeepRecursion(t *testing.T) {
	session := NewSession("test", "/repo")

	root := NewInstanceGroup("Root")
	level1 := NewInstanceGroup("Level 1")
	level2 := NewInstanceGroup("Level 2")
	level3 := NewInstanceGroup("Level 3")

	level2.AddSubGroup(level3)
	level1.AddSubGroup(level2)
	root.AddSubGroup(level1)
	session.AddGroup(root)

	// Session.GetGroup should find deeply nested groups
	tests := []struct {
		name     string
		id       string
		expected *InstanceGroup
	}{
		{"find root", root.ID, root},
		{"find level1", level1.ID, level1},
		{"find level2", level2.ID, level2},
		{"find level3 (deeply nested)", level3.ID, level3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := session.GetGroup(tt.id)
			if result != tt.expected {
				t.Errorf("GetGroup(%q) = %v, want %v", tt.id, result, tt.expected)
			}
		})
	}
}

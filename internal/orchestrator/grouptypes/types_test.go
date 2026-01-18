package grouptypes

import (
	"testing"
	"time"
)

func TestNewInstanceGroup(t *testing.T) {
	g := NewInstanceGroup("test-id", "Test Group")

	if g.ID != "test-id" {
		t.Errorf("ID = %q, want %q", g.ID, "test-id")
	}
	if g.Name != "Test Group" {
		t.Errorf("Name = %q, want %q", g.Name, "Test Group")
	}
	if g.Phase != GroupPhasePending {
		t.Errorf("Phase = %q, want %q", g.Phase, GroupPhasePending)
	}
	if len(g.Instances) != 0 {
		t.Errorf("Instances should be empty, got %d", len(g.Instances))
	}
	if len(g.SubGroups) != 0 {
		t.Errorf("SubGroups should be empty, got %d", len(g.SubGroups))
	}
	if len(g.DependsOn) != 0 {
		t.Errorf("DependsOn should be empty, got %d", len(g.DependsOn))
	}
	if g.Created.IsZero() {
		t.Error("Created should not be zero")
	}
}

func TestNewInstanceGroupWithType(t *testing.T) {
	g := NewInstanceGroupWithType("test-id", "Test Group", "ultraplan", "Build a feature")

	if g.ID != "test-id" {
		t.Errorf("ID = %q, want %q", g.ID, "test-id")
	}
	if g.SessionType != "ultraplan" {
		t.Errorf("SessionType = %q, want %q", g.SessionType, "ultraplan")
	}
	if g.Objective != "Build a feature" {
		t.Errorf("Objective = %q, want %q", g.Objective, "Build a feature")
	}
}

func TestInstanceGroup_AddInstance(t *testing.T) {
	g := NewInstanceGroup("test-id", "Test")
	g.AddInstance("inst-1")
	g.AddInstance("inst-2")

	if len(g.Instances) != 2 {
		t.Errorf("Instances length = %d, want 2", len(g.Instances))
	}
	if g.Instances[0] != "inst-1" || g.Instances[1] != "inst-2" {
		t.Errorf("Instances = %v, want [inst-1, inst-2]", g.Instances)
	}
}

func TestInstanceGroup_RemoveInstance(t *testing.T) {
	g := NewInstanceGroup("test-id", "Test")
	g.AddInstance("inst-1")
	g.AddInstance("inst-2")
	g.AddInstance("inst-3")

	g.RemoveInstance("inst-2")

	if len(g.Instances) != 2 {
		t.Errorf("Instances length = %d, want 2", len(g.Instances))
	}
	if g.Instances[0] != "inst-1" || g.Instances[1] != "inst-3" {
		t.Errorf("Instances = %v, want [inst-1, inst-3]", g.Instances)
	}

	// Remove non-existent should be a no-op
	g.RemoveInstance("inst-999")
	if len(g.Instances) != 2 {
		t.Errorf("Instances length = %d, want 2", len(g.Instances))
	}
}

func TestInstanceGroup_HasInstance(t *testing.T) {
	g := NewInstanceGroup("test-id", "Test")
	g.AddInstance("inst-1")

	if !g.HasInstance("inst-1") {
		t.Error("HasInstance(inst-1) = false, want true")
	}
	if g.HasInstance("inst-2") {
		t.Error("HasInstance(inst-2) = true, want false")
	}
}

func TestInstanceGroup_AddSubGroup(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	child := NewInstanceGroup("child", "Child")

	parent.AddSubGroup(child)

	if len(parent.SubGroups) != 1 {
		t.Errorf("SubGroups length = %d, want 1", len(parent.SubGroups))
	}
	if child.ParentID != "parent" {
		t.Errorf("Child ParentID = %q, want %q", child.ParentID, "parent")
	}
}

func TestInstanceGroup_GetSubGroup(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	child1 := NewInstanceGroup("child1", "Child 1")
	child2 := NewInstanceGroup("child2", "Child 2")

	parent.AddSubGroup(child1)
	parent.AddSubGroup(child2)

	got := parent.GetSubGroup("child1")
	if got != child1 {
		t.Error("GetSubGroup(child1) returned wrong group")
	}

	got = parent.GetSubGroup("child2")
	if got != child2 {
		t.Error("GetSubGroup(child2) returned wrong group")
	}

	got = parent.GetSubGroup("nonexistent")
	if got != nil {
		t.Error("GetSubGroup(nonexistent) should return nil")
	}
}

func TestInstanceGroup_AllInstanceIDs(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	parent.AddInstance("inst-1")
	parent.AddInstance("inst-2")

	child := NewInstanceGroup("child", "Child")
	child.AddInstance("inst-3")
	child.AddInstance("inst-4")

	parent.AddSubGroup(child)

	ids := parent.AllInstanceIDs()
	if len(ids) != 4 {
		t.Errorf("AllInstanceIDs length = %d, want 4", len(ids))
	}

	expected := map[string]bool{"inst-1": true, "inst-2": true, "inst-3": true, "inst-4": true}
	for _, id := range ids {
		if !expected[id] {
			t.Errorf("Unexpected instance ID: %s", id)
		}
	}
}

func TestInstanceGroup_InstanceCount(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	parent.AddInstance("inst-1")

	child := NewInstanceGroup("child", "Child")
	child.AddInstance("inst-2")
	child.AddInstance("inst-3")

	parent.AddSubGroup(child)

	if parent.InstanceCount() != 3 {
		t.Errorf("InstanceCount = %d, want 3", parent.InstanceCount())
	}
	if child.InstanceCount() != 2 {
		t.Errorf("Child InstanceCount = %d, want 2", child.InstanceCount())
	}
}

func TestInstanceGroup_IsEmpty(t *testing.T) {
	g := NewInstanceGroup("test", "Test")
	if !g.IsEmpty() {
		t.Error("Empty group should return true for IsEmpty()")
	}

	g.AddInstance("inst-1")
	if g.IsEmpty() {
		t.Error("Group with instance should return false for IsEmpty()")
	}

	// Test with sub-groups
	parent := NewInstanceGroup("parent", "Parent")
	child := NewInstanceGroup("child", "Child")
	parent.AddSubGroup(child)

	if !parent.IsEmpty() {
		t.Error("Parent with empty child should be empty")
	}

	child.AddInstance("inst-1")
	if parent.IsEmpty() {
		t.Error("Parent with non-empty child should not be empty")
	}
}

func TestInstanceGroup_IsTopLevel(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	child := NewInstanceGroup("child", "Child")

	if !parent.IsTopLevel() {
		t.Error("Parent should be top-level")
	}

	parent.AddSubGroup(child)

	if child.IsTopLevel() {
		t.Error("Child should not be top-level")
	}
}

func TestInstanceGroup_FindGroup(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	child := NewInstanceGroup("child", "Child")
	grandchild := NewInstanceGroup("grandchild", "Grandchild")

	parent.AddSubGroup(child)
	child.AddSubGroup(grandchild)

	// Find self
	if parent.FindGroup("parent") != parent {
		t.Error("FindGroup should find parent itself")
	}

	// Find direct child
	if parent.FindGroup("child") != child {
		t.Error("FindGroup should find direct child")
	}

	// Find grandchild
	if parent.FindGroup("grandchild") != grandchild {
		t.Error("FindGroup should find grandchild")
	}

	// Not found
	if parent.FindGroup("nonexistent") != nil {
		t.Error("FindGroup should return nil for nonexistent group")
	}
}

func TestInstanceGroup_FindGroupContainingInstance(t *testing.T) {
	parent := NewInstanceGroup("parent", "Parent")
	parent.AddInstance("inst-parent")

	child := NewInstanceGroup("child", "Child")
	child.AddInstance("inst-child")

	parent.AddSubGroup(child)

	// Find in parent
	if parent.FindGroupContainingInstance("inst-parent") != parent {
		t.Error("Should find instance in parent")
	}

	// Find in child
	if parent.FindGroupContainingInstance("inst-child") != child {
		t.Error("Should find instance in child")
	}

	// Not found
	if parent.FindGroupContainingInstance("nonexistent") != nil {
		t.Error("Should return nil for nonexistent instance")
	}
}

func TestInstanceGroup_Clone(t *testing.T) {
	original := NewInstanceGroupWithType("test-id", "Test Group", "ultraplan", "Build feature")
	original.Phase = GroupPhaseExecuting
	original.ExecutionOrder = 5
	original.AddInstance("inst-1")
	original.AddInstance("inst-2")
	original.DependsOn = []string{"dep-1", "dep-2"}

	child := NewInstanceGroup("child", "Child")
	child.AddInstance("child-inst")
	original.AddSubGroup(child)

	clone := original.Clone()

	// Verify clone has same values
	if clone.ID != original.ID {
		t.Errorf("Clone ID = %q, want %q", clone.ID, original.ID)
	}
	if clone.Name != original.Name {
		t.Errorf("Clone Name = %q, want %q", clone.Name, original.Name)
	}
	if clone.Phase != original.Phase {
		t.Errorf("Clone Phase = %q, want %q", clone.Phase, original.Phase)
	}
	if clone.SessionType != original.SessionType {
		t.Errorf("Clone SessionType = %q, want %q", clone.SessionType, original.SessionType)
	}
	if clone.Objective != original.Objective {
		t.Errorf("Clone Objective = %q, want %q", clone.Objective, original.Objective)
	}
	if clone.ExecutionOrder != original.ExecutionOrder {
		t.Errorf("Clone ExecutionOrder = %d, want %d", clone.ExecutionOrder, original.ExecutionOrder)
	}
	if len(clone.Instances) != len(original.Instances) {
		t.Errorf("Clone Instances length = %d, want %d", len(clone.Instances), len(original.Instances))
	}
	if len(clone.DependsOn) != len(original.DependsOn) {
		t.Errorf("Clone DependsOn length = %d, want %d", len(clone.DependsOn), len(original.DependsOn))
	}
	if len(clone.SubGroups) != len(original.SubGroups) {
		t.Errorf("Clone SubGroups length = %d, want %d", len(clone.SubGroups), len(original.SubGroups))
	}

	// Verify deep copy - modifying original shouldn't affect clone
	original.Instances[0] = "modified"
	if clone.Instances[0] == "modified" {
		t.Error("Clone Instances should be independent of original")
	}

	original.SubGroups[0].Name = "Modified Child"
	if clone.SubGroups[0].Name == "Modified Child" {
		t.Error("Clone SubGroups should be independent of original")
	}

	// Test nil clone
	var nilGroup *InstanceGroup
	if nilGroup.Clone() != nil {
		t.Error("Clone of nil should return nil")
	}
}

func TestGroupPhaseConstants(t *testing.T) {
	// Verify phase constants have expected values
	tests := []struct {
		phase GroupPhase
		want  string
	}{
		{GroupPhasePending, "pending"},
		{GroupPhaseExecuting, "executing"},
		{GroupPhaseCompleted, "completed"},
		{GroupPhaseFailed, "failed"},
	}

	for _, tt := range tests {
		if string(tt.phase) != tt.want {
			t.Errorf("GroupPhase %v = %q, want %q", tt.phase, string(tt.phase), tt.want)
		}
	}
}

func TestInstanceGroup_CreatedTime(t *testing.T) {
	before := time.Now()
	g := NewInstanceGroup("test", "Test")
	after := time.Now()

	if g.Created.Before(before) || g.Created.After(after) {
		t.Error("Created time should be between test start and end")
	}
}

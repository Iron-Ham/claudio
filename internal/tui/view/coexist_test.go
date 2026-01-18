package view

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// TestUltraplanCoexistenceWithStandardInstances tests that ultraplan groups
// can coexist with standard (ungrouped) instances in the sidebar.
func TestUltraplanCoexistenceWithStandardInstances(t *testing.T) {
	// Create ultraplan group with instances
	ultraGroup := orchestrator.NewInstanceGroupWithType("Auth Task", orchestrator.SessionTypeUltraPlan, "Add auth")
	ultraGroup.Instances = []string{"ultra-inst-1", "ultra-inst-2"}

	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "ultra-inst-1", Task: "Ultraplan Task 1"},
			{ID: "ultra-inst-2", Task: "Ultraplan Task 2"},
			{ID: "std-inst-1", Task: "Standard Task 1"}, // This should be ungrouped
			{ID: "std-inst-2", Task: "Standard Task 2"}, // This should be ungrouped
		},
		Groups: []*orchestrator.InstanceGroup{ultraGroup},
	}

	// Test BuildGroupedSidebarData
	data := BuildGroupedSidebarData(session)

	// Verify ungrouped instances are correctly identified
	if len(data.UngroupedInstances) != 2 {
		t.Errorf("Expected 2 ungrouped instances, got %d", len(data.UngroupedInstances))
	}

	// Verify the ungrouped instances are the standard ones
	ungroupedIDs := make(map[string]bool)
	for _, inst := range data.UngroupedInstances {
		ungroupedIDs[inst.ID] = true
	}
	if !ungroupedIDs["std-inst-1"] {
		t.Error("std-inst-1 should be ungrouped but was not found")
	}
	if !ungroupedIDs["std-inst-2"] {
		t.Error("std-inst-2 should be ungrouped but was not found")
	}

	// Verify ultraplan group is in data.Groups
	if len(data.Groups) != 1 {
		t.Errorf("Expected 1 group (ultraplan), got %d", len(data.Groups))
	}
	if orchestrator.GetSessionType(data.Groups[0]) != orchestrator.SessionTypeUltraPlan {
		t.Errorf("Expected ultraplan group, got %s", orchestrator.GetSessionType(data.Groups[0]))
	}

	// Test FlattenGroupsForDisplay
	groupState := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, groupState)

	// Count instances and headers
	var instanceCount, headerCount int
	var ungroupedCount, groupedCount int
	for _, item := range items {
		switch v := item.(type) {
		case GroupHeaderItem:
			headerCount++
		case GroupedInstance:
			instanceCount++
			if v.Depth == -1 {
				ungroupedCount++
			} else {
				groupedCount++
			}
		}
	}

	// Verify counts
	if instanceCount != 4 {
		t.Errorf("Expected 4 instances in flattened list, got %d", instanceCount)
	}
	if headerCount != 1 {
		t.Errorf("Expected 1 group header, got %d", headerCount)
	}
	if ungroupedCount != 2 {
		t.Errorf("Expected 2 ungrouped instances (depth=-1), got %d", ungroupedCount)
	}
	if groupedCount != 2 {
		t.Errorf("Expected 2 grouped instances, got %d", groupedCount)
	}

	// Verify ungrouped instances come first
	firstItem := items[0]
	if gi, ok := firstItem.(GroupedInstance); !ok || gi.Depth != -1 {
		t.Error("Expected first item to be an ungrouped instance (depth=-1)")
	}
	secondItem := items[1]
	if gi, ok := secondItem.(GroupedInstance); !ok || gi.Depth != -1 {
		t.Error("Expected second item to be an ungrouped instance (depth=-1)")
	}

	// Verify group header comes after ungrouped instances
	thirdItem := items[2]
	if _, ok := thirdItem.(GroupHeaderItem); !ok {
		t.Error("Expected third item to be a group header")
	}

	// Verify AbsoluteIdx is correct for navigation
	for _, item := range items {
		if gi, ok := item.(GroupedInstance); ok {
			// Find the instance in session.Instances
			found := false
			for i, inst := range session.Instances {
				if inst.ID == gi.Instance.ID {
					if gi.AbsoluteIdx != i {
						t.Errorf("Instance %s has AbsoluteIdx=%d but is at position %d in session.Instances",
							inst.ID, gi.AbsoluteIdx, i)
					}
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Instance %s not found in session.Instances", gi.Instance.ID)
			}
		}
	}
}

// TestActiveTabMatchesAbsoluteIdx verifies that activeTab (index in session.Instances)
// correctly matches AbsoluteIdx for proper selection highlighting.
func TestActiveTabMatchesAbsoluteIdx(t *testing.T) {
	// Create a mixed session with grouped and ungrouped instances
	ultraGroup := orchestrator.NewInstanceGroupWithType("Feature", orchestrator.SessionTypeUltraPlan, "Add feature")
	ultraGroup.Instances = []string{"inst-1"}

	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Grouped Task"},   // index 0
			{ID: "inst-2", Task: "Ungrouped Task"}, // index 1
		},
		Groups: []*orchestrator.InstanceGroup{ultraGroup},
	}

	groupState := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, groupState)

	// Display order: ungrouped first (inst-2), then group header, then grouped (inst-1)
	// But AbsoluteIdx should match position in session.Instances
	for _, item := range items {
		if gi, ok := item.(GroupedInstance); ok {
			switch gi.Instance.ID {
			case "inst-1":
				if gi.AbsoluteIdx != 0 {
					t.Errorf("inst-1 should have AbsoluteIdx=0, got %d", gi.AbsoluteIdx)
				}
			case "inst-2":
				if gi.AbsoluteIdx != 1 {
					t.Errorf("inst-2 should have AbsoluteIdx=1, got %d", gi.AbsoluteIdx)
				}
			}
		}
	}
}

// TestAddingInstanceDuringUltraplan simulates the scenario where a user
// adds a standard instance while an ultraplan is running.
func TestAddingInstanceDuringUltraplan(t *testing.T) {
	// Initial state: ultraplan group with 2 instances
	ultraGroup := orchestrator.NewInstanceGroupWithType("Auth Feature", orchestrator.SessionTypeUltraPlan, "Add auth")
	ultraGroup.Instances = []string{"ultra-1", "ultra-2"}

	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "ultra-1", Task: "Add login form"},
			{ID: "ultra-2", Task: "Add logout button"},
		},
		Groups: []*orchestrator.InstanceGroup{ultraGroup},
	}

	// Simulate user adding a new instance via :a command
	// This adds to session.Instances but NOT to any group
	newInstance := &orchestrator.Instance{ID: "std-1", Task: "Fix bug in header"}
	session.Instances = append(session.Instances, newInstance)

	// Verify the new instance is identified as ungrouped
	data := BuildGroupedSidebarData(session)
	if len(data.UngroupedInstances) != 1 {
		t.Errorf("Expected 1 ungrouped instance, got %d", len(data.UngroupedInstances))
	}
	if data.UngroupedInstances[0].ID != "std-1" {
		t.Errorf("Expected ungrouped instance std-1, got %s", data.UngroupedInstances[0].ID)
	}

	// Verify the ultraplan group still has its instances
	if len(data.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(data.Groups))
	}

	// Verify FlattenGroupsForDisplay shows all instances
	groupState := NewGroupViewState()
	items := FlattenGroupsForDisplay(session, groupState)

	// Count instances
	instanceCount := 0
	for _, item := range items {
		if _, ok := item.(GroupedInstance); ok {
			instanceCount++
		}
	}
	if instanceCount != 3 {
		t.Errorf("Expected 3 instances in flattened list, got %d", instanceCount)
	}

	// Verify the new instance has correct AbsoluteIdx (should be 2, the last index)
	for _, item := range items {
		if gi, ok := item.(GroupedInstance); ok && gi.Instance.ID == "std-1" {
			if gi.AbsoluteIdx != 2 {
				t.Errorf("std-1 should have AbsoluteIdx=2, got %d", gi.AbsoluteIdx)
			}
			if gi.Depth != -1 {
				t.Errorf("std-1 should have Depth=-1 (ungrouped), got %d", gi.Depth)
			}
		}
	}
}

// TestVisibleInstanceCount verifies GetVisibleInstanceCount includes both
// grouped and ungrouped instances.
func TestVisibleInstanceCount(t *testing.T) {
	ultraGroup := orchestrator.NewInstanceGroupWithType("Feature", orchestrator.SessionTypeUltraPlan, "Add feature")
	ultraGroup.Instances = []string{"inst-1", "inst-2"}

	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
			{ID: "inst-3", Task: "Ungrouped Task"}, // ungrouped
		},
		Groups: []*orchestrator.InstanceGroup{ultraGroup},
	}

	groupState := NewGroupViewState()
	count := GetVisibleInstanceCount(session, groupState)

	// All groups expanded: 1 ungrouped + 2 in group = 3 visible
	if count != 3 {
		t.Errorf("Expected 3 visible instances, got %d", count)
	}

	// Collapse the group
	groupState.ToggleCollapse(ultraGroup.ID)
	count = GetVisibleInstanceCount(session, groupState)

	// Group collapsed: 1 ungrouped + 0 (collapsed group) = 1 visible
	if count != 1 {
		t.Errorf("Expected 1 visible instance when group is collapsed, got %d", count)
	}
}

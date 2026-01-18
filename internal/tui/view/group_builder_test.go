package view

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestBuildGroupedSidebarData_NilSession(t *testing.T) {
	data := BuildGroupedSidebarData(nil)

	if data == nil {
		t.Fatal("Expected non-nil data")
	}
	if len(data.UngroupedInstances) != 0 {
		t.Errorf("Expected 0 ungrouped instances, got %d", len(data.UngroupedInstances))
	}
	if len(data.Groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(data.Groups))
	}
}

func TestBuildGroupedSidebarData_UngroupedInstances(t *testing.T) {
	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
		},
		Groups: nil,
	}

	data := BuildGroupedSidebarData(session)

	if len(data.UngroupedInstances) != 2 {
		t.Errorf("Expected 2 ungrouped instances, got %d", len(data.UngroupedInstances))
	}
	if len(data.Groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(data.Groups))
	}
}

func TestBuildGroupedSidebarData_WithGroups(t *testing.T) {
	ultraGroup := orchestrator.NewInstanceGroupWithType("Ultraplan Task", orchestrator.SessionTypeUltraPlan, "Do something")
	ultraGroup.Instances = []string{"inst-1", "inst-2"}

	planGroup := orchestrator.NewInstanceGroupWithType("Plans", orchestrator.SessionTypePlan, "")
	planGroup.Instances = []string{"inst-3"}

	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
			{ID: "inst-3", Task: "Task 3"},
			{ID: "inst-4", Task: "Ungrouped Task"},
		},
		Groups: []*orchestrator.InstanceGroup{ultraGroup, planGroup},
	}

	data := BuildGroupedSidebarData(session)

	// inst-4 should be ungrouped
	if len(data.UngroupedInstances) != 1 {
		t.Errorf("Expected 1 ungrouped instance, got %d", len(data.UngroupedInstances))
	}
	if data.UngroupedInstances[0].ID != "inst-4" {
		t.Errorf("Expected ungrouped instance inst-4, got %s", data.UngroupedInstances[0].ID)
	}

	// ultraplan should be in Groups (own type)
	if len(data.Groups) != 1 {
		t.Errorf("Expected 1 own group, got %d", len(data.Groups))
	}
	if orchestrator.GetSessionType(data.Groups[0]) != orchestrator.SessionTypeUltraPlan {
		t.Errorf("Expected SessionTypeUltraPlan, got %s", orchestrator.GetSessionType(data.Groups[0]))
	}

	// plan should be in SharedGroups (shared type)
	if len(data.SharedGroups) != 1 {
		t.Errorf("Expected 1 shared group, got %d", len(data.SharedGroups))
	}
	if orchestrator.GetSessionType(data.SharedGroups[0]) != orchestrator.SessionTypePlan {
		t.Errorf("Expected SessionTypePlan, got %s", orchestrator.GetSessionType(data.SharedGroups[0]))
	}
}

func TestGroupedSidebarData_HasGroups(t *testing.T) {
	tests := []struct {
		name     string
		data     *GroupedSidebarData
		expected bool
	}{
		{
			name:     "no groups",
			data:     &GroupedSidebarData{},
			expected: false,
		},
		{
			name: "with regular groups",
			data: &GroupedSidebarData{
				Groups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Test"),
				},
			},
			expected: true,
		},
		{
			name: "with shared groups",
			data: &GroupedSidebarData{
				SharedGroups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Plans"),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.HasGroups()
			if got != tt.expected {
				t.Errorf("HasGroups() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGroupedSidebarData_TotalGroupCount(t *testing.T) {
	tests := []struct {
		name     string
		data     *GroupedSidebarData
		expected int
	}{
		{
			name:     "no groups",
			data:     &GroupedSidebarData{},
			expected: 0,
		},
		{
			name: "only regular groups",
			data: &GroupedSidebarData{
				Groups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Test"),
				},
			},
			expected: 1,
		},
		{
			name: "only shared groups",
			data: &GroupedSidebarData{
				SharedGroups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Plans"),
				},
			},
			expected: 1,
		},
		{
			name: "both types",
			data: &GroupedSidebarData{
				Groups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Ultra"),
					orchestrator.NewInstanceGroup("Triple"),
				},
				SharedGroups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Plans"),
				},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.data.TotalGroupCount()
			if got != tt.expected {
				t.Errorf("TotalGroupCount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildSidebarSections(t *testing.T) {
	ultraGroup := orchestrator.NewInstanceGroupWithType("Auth Task", orchestrator.SessionTypeUltraPlan, "Add auth")
	ultraGroup.Instances = []string{"inst-1"}

	session := &orchestrator.Session{
		ID: "test-session",
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Ungrouped"},
		},
		Groups: []*orchestrator.InstanceGroup{ultraGroup},
	}

	sections := BuildSidebarSections(session)

	// Should have 2 sections: ungrouped instances and ultraplan group
	if len(sections) != 2 {
		t.Fatalf("Expected 2 sections, got %d", len(sections))
	}

	// First section should be ungrouped instances
	if sections[0].Type != SectionTypeUngrouped {
		t.Errorf("First section type = %v, want SectionTypeUngrouped", sections[0].Type)
	}
	if sections[0].Title != "INSTANCES" {
		t.Errorf("First section title = %q, want INSTANCES", sections[0].Title)
	}
	if len(sections[0].Instances) != 1 {
		t.Errorf("Expected 1 ungrouped instance, got %d", len(sections[0].Instances))
	}

	// Second section should be ultraplan group
	if sections[1].Type != SectionTypeGroup {
		t.Errorf("Second section type = %v, want SectionTypeGroup", sections[1].Type)
	}
	if sections[1].Icon != orchestrator.SessionTypeUltraPlan.Icon() {
		t.Errorf("Second section icon = %q, want %q", sections[1].Icon, orchestrator.SessionTypeUltraPlan.Icon())
	}
}

func TestShouldUseGroupedMode(t *testing.T) {
	tests := []struct {
		name     string
		session  *orchestrator.Session
		expected bool
	}{
		{
			name:     "nil session",
			session:  nil,
			expected: false,
		},
		{
			name: "no groups",
			session: &orchestrator.Session{
				ID:     "test",
				Groups: nil,
			},
			expected: false,
		},
		{
			name: "empty groups",
			session: &orchestrator.Session{
				ID:     "test",
				Groups: []*orchestrator.InstanceGroup{},
			},
			expected: false,
		},
		{
			name: "with groups",
			session: &orchestrator.Session{
				ID: "test",
				Groups: []*orchestrator.InstanceGroup{
					orchestrator.NewInstanceGroup("Test"),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldUseGroupedMode(tt.session)
			if got != tt.expected {
				t.Errorf("ShouldUseGroupedMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

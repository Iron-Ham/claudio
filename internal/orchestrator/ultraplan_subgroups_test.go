package orchestrator

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/grouptypes"
)

func TestDetermineSubgroupType(t *testing.T) {
	tests := []struct {
		name       string
		session    *UltraPlanSession
		instanceID string
		want       SubgroupType
	}{
		{
			name:       "nil session returns unknown",
			session:    nil,
			instanceID: "inst-1",
			want:       SubgroupTypeUnknown,
		},
		{
			name:       "empty instanceID returns unknown",
			session:    &UltraPlanSession{},
			instanceID: "",
			want:       SubgroupTypeUnknown,
		},
		{
			name: "coordinator ID returns planning",
			session: &UltraPlanSession{
				CoordinatorID: "coord-1",
			},
			instanceID: "coord-1",
			want:       SubgroupTypePlanning,
		},
		{
			name: "plan coordinator ID returns planning",
			session: &UltraPlanSession{
				PlanCoordinatorIDs: []string{"plan-1", "plan-2"},
			},
			instanceID: "plan-2",
			want:       SubgroupTypePlanning,
		},
		{
			name: "plan manager ID returns plan selection",
			session: &UltraPlanSession{
				PlanManagerID: "manager-1",
			},
			instanceID: "manager-1",
			want:       SubgroupTypePlanSelection,
		},
		{
			name: "task instance returns execution",
			session: &UltraPlanSession{
				TaskToInstance: map[string]string{
					"task-1": "task-inst-1",
					"task-2": "task-inst-2",
				},
			},
			instanceID: "task-inst-2",
			want:       SubgroupTypeExecution,
		},
		{
			name: "group consolidator returns consolidator",
			session: &UltraPlanSession{
				GroupConsolidatorIDs: []string{"consol-0", "consol-1"},
			},
			instanceID: "consol-1",
			want:       SubgroupTypeConsolidator,
		},
		{
			name: "synthesis ID returns synthesis",
			session: &UltraPlanSession{
				SynthesisID: "synth-1",
			},
			instanceID: "synth-1",
			want:       SubgroupTypeSynthesis,
		},
		{
			name: "revision ID returns revision",
			session: &UltraPlanSession{
				RevisionID: "rev-1",
			},
			instanceID: "rev-1",
			want:       SubgroupTypeRevision,
		},
		{
			name: "consolidation ID returns final consolidation",
			session: &UltraPlanSession{
				ConsolidationID: "final-1",
			},
			instanceID: "final-1",
			want:       SubgroupTypeFinalConsolidation,
		},
		{
			name: "unknown instance returns unknown",
			session: &UltraPlanSession{
				CoordinatorID: "coord-1",
				SynthesisID:   "synth-1",
			},
			instanceID: "unknown-inst",
			want:       SubgroupTypeUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineSubgroupType(tt.session, tt.instanceID)
			if got != tt.want {
				t.Errorf("determineSubgroupType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTaskGroupIndex(t *testing.T) {
	tests := []struct {
		name       string
		session    *UltraPlanSession
		instanceID string
		want       int
	}{
		{
			name:       "nil session returns -1",
			session:    nil,
			instanceID: "inst-1",
			want:       -1,
		},
		{
			name: "nil plan returns -1",
			session: &UltraPlanSession{
				Plan: nil,
				TaskToInstance: map[string]string{
					"task-1": "inst-1",
				},
			},
			instanceID: "inst-1",
			want:       -1,
		},
		{
			name: "instance not in TaskToInstance returns -1",
			session: &UltraPlanSession{
				Plan: &PlanSpec{
					ExecutionOrder: [][]string{{"task-1"}},
				},
				TaskToInstance: map[string]string{
					"task-1": "inst-1",
				},
			},
			instanceID: "unknown-inst",
			want:       -1,
		},
		{
			name: "task in first group returns 0",
			session: &UltraPlanSession{
				Plan: &PlanSpec{
					ExecutionOrder: [][]string{
						{"task-1", "task-2"},
						{"task-3"},
					},
				},
				TaskToInstance: map[string]string{
					"task-1": "inst-1",
					"task-2": "inst-2",
					"task-3": "inst-3",
				},
			},
			instanceID: "inst-1",
			want:       0,
		},
		{
			name: "task in second group returns 1",
			session: &UltraPlanSession{
				Plan: &PlanSpec{
					ExecutionOrder: [][]string{
						{"task-1", "task-2"},
						{"task-3"},
					},
				},
				TaskToInstance: map[string]string{
					"task-1": "inst-1",
					"task-2": "inst-2",
					"task-3": "inst-3",
				},
			},
			instanceID: "inst-3",
			want:       1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTaskGroupIndex(tt.session, tt.instanceID)
			if got != tt.want {
				t.Errorf("getTaskGroupIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetConsolidatorGroupIndex(t *testing.T) {
	tests := []struct {
		name       string
		session    *UltraPlanSession
		instanceID string
		want       int
	}{
		{
			name:       "nil session returns -1",
			session:    nil,
			instanceID: "consol-0",
			want:       -1,
		},
		{
			name: "first consolidator returns 0",
			session: &UltraPlanSession{
				GroupConsolidatorIDs: []string{"consol-0", "consol-1"},
			},
			instanceID: "consol-0",
			want:       0,
		},
		{
			name: "second consolidator returns 1",
			session: &UltraPlanSession{
				GroupConsolidatorIDs: []string{"consol-0", "consol-1"},
			},
			instanceID: "consol-1",
			want:       1,
		},
		{
			name: "unknown consolidator returns -1",
			session: &UltraPlanSession{
				GroupConsolidatorIDs: []string{"consol-0"},
			},
			instanceID: "consol-unknown",
			want:       -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getConsolidatorGroupIndex(tt.session, tt.instanceID)
			if got != tt.want {
				t.Errorf("getConsolidatorGroupIndex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecutionGroupSubgroupName(t *testing.T) {
	tests := []struct {
		groupIndex int
		want       string
	}{
		{0, "Group 1"},
		{1, "Group 2"},
		{9, "Group 10"},
	}

	for _, tt := range tests {
		got := executionGroupSubgroupName(tt.groupIndex)
		if got != tt.want {
			t.Errorf("executionGroupSubgroupName(%d) = %q, want %q", tt.groupIndex, got, tt.want)
		}
	}
}

func TestSanitizeSubgroupID(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Planning", "planning"},
		{"Group 1", "group-1"},
		{"Plan Selection", "plan-selection"},
		{"Synthesis", "synthesis"},
	}

	for _, tt := range tests {
		got := sanitizeSubgroupID(tt.name)
		if got != tt.want {
			t.Errorf("sanitizeSubgroupID(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestGetOrCreateSubgroup(t *testing.T) {
	t.Run("nil parent returns nil", func(t *testing.T) {
		got := getOrCreateSubgroup(nil, "Planning")
		if got != nil {
			t.Errorf("getOrCreateSubgroup(nil, ...) = %v, want nil", got)
		}
	})

	t.Run("creates new subgroup", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		subgroup := getOrCreateSubgroup(parent, "Planning")

		if subgroup == nil {
			t.Fatal("getOrCreateSubgroup() returned nil")
		}
		if subgroup.Name != "Planning" {
			t.Errorf("subgroup.Name = %q, want %q", subgroup.Name, "Planning")
		}
		if subgroup.ID != "parent-1-planning" {
			t.Errorf("subgroup.ID = %q, want %q", subgroup.ID, "parent-1-planning")
		}
		if len(parent.SubGroups) != 1 {
			t.Errorf("len(parent.SubGroups) = %d, want 1", len(parent.SubGroups))
		}
	})

	t.Run("returns existing subgroup", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		first := getOrCreateSubgroup(parent, "Planning")
		second := getOrCreateSubgroup(parent, "Planning")

		if first != second {
			t.Error("getOrCreateSubgroup() did not return existing subgroup")
		}
		if len(parent.SubGroups) != 1 {
			t.Errorf("len(parent.SubGroups) = %d, want 1", len(parent.SubGroups))
		}
	})
}

func TestAddInstanceToSubgroup(t *testing.T) {
	t.Run("nil parent returns false", func(t *testing.T) {
		session := &UltraPlanSession{CoordinatorID: "coord-1"}
		if addInstanceToSubgroup(nil, session, "coord-1") {
			t.Error("addInstanceToSubgroup(nil, ...) = true, want false")
		}
	})

	t.Run("nil session returns false", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		if addInstanceToSubgroup(parent, nil, "inst-1") {
			t.Error("addInstanceToSubgroup(..., nil, ...) = true, want false")
		}
	})

	t.Run("empty instanceID returns false", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		session := &UltraPlanSession{}
		if addInstanceToSubgroup(parent, session, "") {
			t.Error("addInstanceToSubgroup(..., \"\") = true, want false")
		}
	})

	t.Run("planning instance added to Planning subgroup", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		session := &UltraPlanSession{CoordinatorID: "coord-1"}

		if !addInstanceToSubgroup(parent, session, "coord-1") {
			t.Error("addInstanceToSubgroup() = false, want true")
		}

		// Verify subgroup was created and instance added
		if len(parent.SubGroups) != 1 {
			t.Fatalf("len(parent.SubGroups) = %d, want 1", len(parent.SubGroups))
		}
		subgroup := parent.SubGroups[0]
		if subgroup.Name != "Planning" {
			t.Errorf("subgroup.Name = %q, want %q", subgroup.Name, "Planning")
		}
		if !subgroup.HasInstance("coord-1") {
			t.Error("subgroup does not contain coord-1")
		}
	})

	t.Run("task instance added to correct execution group", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		session := &UltraPlanSession{
			Plan: &PlanSpec{
				ExecutionOrder: [][]string{
					{"task-1"},
					{"task-2"},
				},
			},
			TaskToInstance: map[string]string{
				"task-1": "inst-1",
				"task-2": "inst-2",
			},
		}

		// Add instance from second group
		if !addInstanceToSubgroup(parent, session, "inst-2") {
			t.Error("addInstanceToSubgroup() = false, want true")
		}

		// Verify correct subgroup
		var found bool
		for _, sg := range parent.SubGroups {
			if sg.Name == "Group 2" && sg.HasInstance("inst-2") {
				found = true
				break
			}
		}
		if !found {
			t.Error("inst-2 not found in Group 2 subgroup")
		}
	})

	t.Run("synthesis instance added to Synthesis subgroup", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		session := &UltraPlanSession{SynthesisID: "synth-1"}

		if !addInstanceToSubgroup(parent, session, "synth-1") {
			t.Error("addInstanceToSubgroup() = false, want true")
		}

		// Verify subgroup
		var found bool
		for _, sg := range parent.SubGroups {
			if sg.Name == "Synthesis" && sg.HasInstance("synth-1") {
				found = true
				break
			}
		}
		if !found {
			t.Error("synth-1 not found in Synthesis subgroup")
		}
	})

	t.Run("unknown instance added to parent group", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		session := &UltraPlanSession{}

		if !addInstanceToSubgroup(parent, session, "unknown-inst") {
			t.Error("addInstanceToSubgroup() = false, want true")
		}

		if !parent.HasInstance("unknown-inst") {
			t.Error("unknown-inst not added to parent group")
		}
	})
}

func TestSubgroupRouter(t *testing.T) {
	t.Run("AddInstance routes to correct subgroup", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		session := &UltraPlanSession{
			CoordinatorID: "coord-1",
			SynthesisID:   "synth-1",
		}
		router := NewSubgroupRouter(parent, session)

		router.AddInstance("coord-1")
		router.AddInstance("synth-1")

		// Verify Planning subgroup has coord-1
		planning := parent.GetSubGroup("parent-1-planning")
		if planning == nil {
			t.Fatal("Planning subgroup not created")
		}
		if !planning.HasInstance("coord-1") {
			t.Error("coord-1 not in Planning subgroup")
		}

		// Verify Synthesis subgroup has synth-1
		synthesis := parent.GetSubGroup("parent-1-synthesis")
		if synthesis == nil {
			t.Fatal("Synthesis subgroup not created")
		}
		if !synthesis.HasInstance("synth-1") {
			t.Error("synth-1 not in Synthesis subgroup")
		}
	})

	t.Run("AddInstance with nil parent does nothing", func(t *testing.T) {
		router := NewSubgroupRouter(nil, &UltraPlanSession{})
		router.AddInstance("inst-1") // Should not panic
	})

	t.Run("AddInstance with empty ID does nothing", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		router := NewSubgroupRouter(parent, &UltraPlanSession{})
		router.AddInstance("")
		if len(parent.Instances) != 0 || len(parent.SubGroups) != 0 {
			t.Error("Empty ID should not add anything")
		}
	})

	t.Run("AddInstance falls back to parent when session nil", func(t *testing.T) {
		parent := grouptypes.NewInstanceGroup("parent-1", "Parent")
		router := NewSubgroupRouter(parent, nil)
		router.AddInstance("inst-1")
		if !parent.HasInstance("inst-1") {
			t.Error("inst-1 not added to parent group when session nil")
		}
	})
}

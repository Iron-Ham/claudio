package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestRalphWiggumState_HasActiveCoordinators(t *testing.T) {
	t.Run("nil state returns false", func(t *testing.T) {
		var state *RalphWiggumState
		if state.HasActiveCoordinators() {
			t.Error("expected false for nil state")
		}
	})

	t.Run("empty state returns false", func(t *testing.T) {
		state := &RalphWiggumState{}
		if state.HasActiveCoordinators() {
			t.Error("expected false for empty state")
		}
	})

	t.Run("state with empty Coordinators map returns false", func(t *testing.T) {
		state := &RalphWiggumState{
			Coordinators: make(map[string]*orchestrator.RalphWiggumCoordinator),
		}
		if state.HasActiveCoordinators() {
			t.Error("expected false for empty coordinators map")
		}
	})

	t.Run("state with coordinators in map returns true", func(t *testing.T) {
		state := &RalphWiggumState{
			Coordinators: map[string]*orchestrator.RalphWiggumCoordinator{
				"group-1": nil, // Even nil coordinator counts as entry
			},
		}
		if !state.HasActiveCoordinators() {
			t.Error("expected true when coordinators map has entries")
		}
	})
}

func TestRalphWiggumState_GetAllCoordinators(t *testing.T) {
	t.Run("nil state returns nil", func(t *testing.T) {
		var state *RalphWiggumState
		if state.GetAllCoordinators() != nil {
			t.Error("expected nil for nil state")
		}
	})

	t.Run("empty state returns nil", func(t *testing.T) {
		state := &RalphWiggumState{}
		if state.GetAllCoordinators() != nil {
			t.Error("expected nil for empty state")
		}
	})

	t.Run("returns coordinators from map", func(t *testing.T) {
		coord1 := &orchestrator.RalphWiggumCoordinator{}
		coord2 := &orchestrator.RalphWiggumCoordinator{}
		state := &RalphWiggumState{
			Coordinators: map[string]*orchestrator.RalphWiggumCoordinator{
				"group-1": coord1,
				"group-2": coord2,
			},
		}

		result := state.GetAllCoordinators()
		if len(result) != 2 {
			t.Errorf("expected 2 coordinators, got %d", len(result))
		}
	})

	t.Run("returns deterministic order by group ID", func(t *testing.T) {
		coord1 := &orchestrator.RalphWiggumCoordinator{}
		coord2 := &orchestrator.RalphWiggumCoordinator{}
		coord3 := &orchestrator.RalphWiggumCoordinator{}
		state := &RalphWiggumState{
			Coordinators: map[string]*orchestrator.RalphWiggumCoordinator{
				"z-group": coord1,
				"a-group": coord2,
				"m-group": coord3,
			},
		}

		// Call multiple times to verify order is deterministic
		for i := 0; i < 10; i++ {
			result := state.GetAllCoordinators()
			// Should be sorted alphabetically: a-group, m-group, z-group
			if result[0] != coord2 || result[1] != coord3 || result[2] != coord1 {
				t.Error("expected coordinators to be returned in sorted key order")
			}
		}
	})

	t.Run("returns nil when map empty", func(t *testing.T) {
		state := &RalphWiggumState{
			Coordinators: make(map[string]*orchestrator.RalphWiggumCoordinator),
		}

		result := state.GetAllCoordinators()
		if result != nil {
			t.Errorf("expected nil for empty map, got %d coordinators", len(result))
		}
	})
}

func TestRalphWiggumState_GetCoordinatorForGroup(t *testing.T) {
	t.Run("returns coordinator from map when found", func(t *testing.T) {
		coord := &orchestrator.RalphWiggumCoordinator{}
		state := &RalphWiggumState{
			Coordinators: map[string]*orchestrator.RalphWiggumCoordinator{
				"target-group": coord,
			},
		}

		result := state.GetCoordinatorForGroup("target-group")
		if result != coord {
			t.Error("expected to find coordinator by group ID")
		}
	})

	t.Run("returns nil when not found", func(t *testing.T) {
		state := &RalphWiggumState{
			Coordinators: map[string]*orchestrator.RalphWiggumCoordinator{
				"other-group": &orchestrator.RalphWiggumCoordinator{},
			},
		}

		result := state.GetCoordinatorForGroup("missing-group")
		if result != nil {
			t.Error("expected nil for missing group")
		}
	})

	t.Run("returns nil when coordinators map is nil", func(t *testing.T) {
		state := &RalphWiggumState{}

		result := state.GetCoordinatorForGroup("any-group")
		if result != nil {
			t.Error("expected nil when coordinators map is nil")
		}
	})

	t.Run("nil state returns nil", func(t *testing.T) {
		var state *RalphWiggumState
		result := state.GetCoordinatorForGroup("any-group")
		if result != nil {
			t.Error("expected nil for nil state")
		}
	})
}

func TestRenderRalphWiggumHeader(t *testing.T) {
	t.Run("returns empty for nil state", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			RalphWiggum: nil,
		}
		result := RenderRalphWiggumHeader(ctx)
		if result != "" {
			t.Errorf("expected empty string for nil state, got: %q", result)
		}
	})

	t.Run("returns empty for empty coordinators", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			RalphWiggum: &RalphWiggumState{
				Coordinators: make(map[string]*orchestrator.RalphWiggumCoordinator),
			},
		}
		result := RenderRalphWiggumHeader(ctx)
		if result != "" {
			t.Errorf("expected empty string for empty coordinators, got: %q", result)
		}
	})
}

func TestRenderRalphWiggumSidebarSection(t *testing.T) {
	t.Run("returns empty for nil state", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			RalphWiggum: nil,
		}
		result := RenderRalphWiggumSidebarSection(ctx, 40)
		if result != "" {
			t.Errorf("expected empty string for nil state, got: %q", result)
		}
	})

	t.Run("returns empty for empty coordinators", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			RalphWiggum: &RalphWiggumState{
				Coordinators: make(map[string]*orchestrator.RalphWiggumCoordinator),
			},
		}
		result := RenderRalphWiggumSidebarSection(ctx, 40)
		if result != "" {
			t.Errorf("expected empty string for empty coordinators, got: %q", result)
		}
	})
}

func TestRenderRalphWiggumHelp(t *testing.T) {
	t.Run("renders help bar with expected keys", func(t *testing.T) {
		state := &HelpBarState{}
		result := RenderRalphWiggumHelp(state)

		// Check for expected help keys
		if !strings.Contains(result, "c") {
			t.Error("expected 'c' key in help")
		}
		if !strings.Contains(result, "continue") {
			t.Error("expected 'continue' action in help")
		}
		if !strings.Contains(result, "p") {
			t.Error("expected 'p' key in help")
		}
		if !strings.Contains(result, "pause") {
			t.Error("expected 'pause' action in help")
		}
		if !strings.Contains(result, "x") {
			t.Error("expected 'x' key in help")
		}
		if !strings.Contains(result, "stop") {
			t.Error("expected 'stop' action in help")
		}
	})
}

func TestFindRalphWiggumForActiveInstance(t *testing.T) {
	t.Run("returns nil for nil session", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			Session:     nil,
			RalphWiggum: &RalphWiggumState{},
		}
		result := FindRalphWiggumForActiveInstance(ctx)
		if result != nil {
			t.Error("expected nil for nil session")
		}
	})

	t.Run("returns nil when activeTab out of bounds", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{},
			},
			ActiveTab:   5,
			RalphWiggum: &RalphWiggumState{},
		}
		result := FindRalphWiggumForActiveInstance(ctx)
		if result != nil {
			t.Error("expected nil when activeTab out of bounds")
		}
	})

	t.Run("returns nil when no matching ralph wiggum session", func(t *testing.T) {
		ctx := RalphWiggumRenderContext{
			Session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst-1"},
				},
			},
			ActiveTab: 0,
			RalphWiggum: &RalphWiggumState{
				Coordinators: make(map[string]*orchestrator.RalphWiggumCoordinator),
			},
		}
		result := FindRalphWiggumForActiveInstance(ctx)
		if result != nil {
			t.Error("expected nil when no matching ralph wiggum session")
		}
	})
}

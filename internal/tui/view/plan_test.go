package view

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestPlanState_NewPlanState(t *testing.T) {
	state := NewPlanState()
	if state == nil {
		t.Fatal("NewPlanState() returned nil")
	}
	if state.Sessions == nil {
		t.Error("Sessions map should be initialized")
	}
	if len(state.Sessions) != 0 {
		t.Errorf("Sessions map should be empty, got %d", len(state.Sessions))
	}
}

func TestPlanState_AddSession(t *testing.T) {
	state := NewPlanState()
	session := &PlanSession{
		GroupID:   "group-1",
		Objective: "Test objective",
		Phase:     PlanPhasePlanning,
	}

	state.AddSession(session)

	if len(state.Sessions) != 1 {
		t.Errorf("Sessions count = %d, want 1", len(state.Sessions))
	}
	if state.Sessions["group-1"] != session {
		t.Error("Session not stored correctly")
	}
}

func TestPlanState_GetSession(t *testing.T) {
	state := NewPlanState()
	session := &PlanSession{
		GroupID:   "group-1",
		Objective: "Test objective",
	}
	state.AddSession(session)

	// Test getting existing session
	got := state.GetSession("group-1")
	if got != session {
		t.Error("GetSession should return the added session")
	}

	// Test getting non-existent session
	got = state.GetSession("non-existent")
	if got != nil {
		t.Error("GetSession should return nil for non-existent session")
	}

	// Test nil state
	var nilState *PlanState
	got = nilState.GetSession("group-1")
	if got != nil {
		t.Error("GetSession on nil state should return nil")
	}
}

func TestPlanState_RemoveSession(t *testing.T) {
	state := NewPlanState()
	session := &PlanSession{
		GroupID: "group-1",
	}
	state.AddSession(session)

	state.RemoveSession("group-1")

	if len(state.Sessions) != 0 {
		t.Errorf("Sessions count = %d after removal, want 0", len(state.Sessions))
	}

	// Removing non-existent session should not panic
	state.RemoveSession("non-existent")
}

func TestPlanState_GetAllSessions(t *testing.T) {
	t.Run("empty state", func(t *testing.T) {
		state := NewPlanState()
		sessions := state.GetAllSessions()
		if sessions != nil {
			t.Error("GetAllSessions should return nil for empty state")
		}
	})

	t.Run("nil state", func(t *testing.T) {
		var nilState *PlanState
		sessions := nilState.GetAllSessions()
		if sessions != nil {
			t.Error("GetAllSessions should return nil for nil state")
		}
	})

	t.Run("multiple sessions sorted by group ID", func(t *testing.T) {
		state := NewPlanState()
		state.AddSession(&PlanSession{GroupID: "group-c", Objective: "C"})
		state.AddSession(&PlanSession{GroupID: "group-a", Objective: "A"})
		state.AddSession(&PlanSession{GroupID: "group-b", Objective: "B"})

		sessions := state.GetAllSessions()
		if len(sessions) != 3 {
			t.Fatalf("GetAllSessions count = %d, want 3", len(sessions))
		}

		// Verify sorted order
		expectedOrder := []string{"group-a", "group-b", "group-c"}
		for i, sess := range sessions {
			if sess.GroupID != expectedOrder[i] {
				t.Errorf("Session %d GroupID = %s, want %s", i, sess.GroupID, expectedOrder[i])
			}
		}
	})
}

func TestPlanState_HasActiveSessions(t *testing.T) {
	t.Run("nil state", func(t *testing.T) {
		var nilState *PlanState
		if nilState.HasActiveSessions() {
			t.Error("HasActiveSessions should return false for nil state")
		}
	})

	t.Run("empty state", func(t *testing.T) {
		state := NewPlanState()
		if state.HasActiveSessions() {
			t.Error("HasActiveSessions should return false for empty state")
		}
	})

	t.Run("with sessions", func(t *testing.T) {
		state := NewPlanState()
		state.AddSession(&PlanSession{GroupID: "group-1"})
		if !state.HasActiveSessions() {
			t.Error("HasActiveSessions should return true when sessions exist")
		}
	})
}

func TestPlanState_SessionCount(t *testing.T) {
	state := NewPlanState()
	if state.SessionCount() != 0 {
		t.Errorf("SessionCount = %d for empty state, want 0", state.SessionCount())
	}

	state.AddSession(&PlanSession{GroupID: "group-1"})
	state.AddSession(&PlanSession{GroupID: "group-2"})

	if state.SessionCount() != 2 {
		t.Errorf("SessionCount = %d, want 2", state.SessionCount())
	}

	var nilState *PlanState
	if nilState.SessionCount() != 0 {
		t.Errorf("SessionCount = %d for nil state, want 0", nilState.SessionCount())
	}
}

func TestRenderPlanHeader(t *testing.T) {
	t.Run("nil plan state", func(t *testing.T) {
		ctx := PlanRenderContext{
			Plan: nil,
		}
		result := RenderPlanHeader(ctx)
		if result != "" {
			t.Errorf("RenderPlanHeader with nil state = %q, want empty", result)
		}
	})

	t.Run("empty plan state", func(t *testing.T) {
		ctx := PlanRenderContext{
			Plan: NewPlanState(),
		}
		result := RenderPlanHeader(ctx)
		if result != "" {
			t.Errorf("RenderPlanHeader with empty state = %q, want empty", result)
		}
	})

	t.Run("single session planning phase", func(t *testing.T) {
		state := NewPlanState()
		state.AddSession(&PlanSession{
			GroupID:       "group-1",
			Objective:     "Test",
			Phase:         PlanPhasePlanning,
			PlannerStatus: PlannerStatusWorking,
		})
		ctx := PlanRenderContext{
			Plan: state,
		}
		result := RenderPlanHeader(ctx)
		if result == "" {
			t.Error("RenderPlanHeader should return non-empty for single session")
		}
	})

	t.Run("multiple sessions", func(t *testing.T) {
		state := NewPlanState()
		state.AddSession(&PlanSession{
			GroupID: "group-1",
			Phase:   PlanPhasePlanning,
		})
		state.AddSession(&PlanSession{
			GroupID: "group-2",
			Phase:   PlanPhaseComplete,
		})
		ctx := PlanRenderContext{
			Plan: state,
		}
		result := RenderPlanHeader(ctx)
		if result == "" {
			t.Error("RenderPlanHeader should return non-empty for multiple sessions")
		}
	})
}

func TestRenderPlanSidebarSection(t *testing.T) {
	t.Run("nil plan state", func(t *testing.T) {
		ctx := PlanRenderContext{
			Plan: nil,
		}
		result := RenderPlanSidebarSection(ctx, 40)
		if result != "" {
			t.Errorf("RenderPlanSidebarSection with nil state = %q, want empty", result)
		}
	})

	t.Run("single session", func(t *testing.T) {
		state := NewPlanState()
		state.AddSession(&PlanSession{
			GroupID:       "group-1",
			Objective:     "Implement feature X",
			Phase:         PlanPhasePlanning,
			PlannerStatus: PlannerStatusWorking,
			PlannerID:     "planner-1",
		})
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "planner-1", Status: orchestrator.StatusWorking},
			},
		}
		ctx := PlanRenderContext{
			Plan:    state,
			Session: session,
			Width:   50,
		}
		result := RenderPlanSidebarSection(ctx, 50)
		if result == "" {
			t.Error("RenderPlanSidebarSection should return non-empty for active session")
		}
	})

	t.Run("multiple sessions", func(t *testing.T) {
		state := NewPlanState()
		state.AddSession(&PlanSession{
			GroupID:       "group-1",
			Objective:     "First objective",
			Phase:         PlanPhasePlanning,
			PlannerStatus: PlannerStatusWorking,
		})
		state.AddSession(&PlanSession{
			GroupID:       "group-2",
			Objective:     "Second objective",
			Phase:         PlanPhaseComplete,
			PlannerStatus: PlannerStatusCompleted,
		})
		ctx := PlanRenderContext{
			Plan:  state,
			Width: 50,
		}
		result := RenderPlanSidebarSection(ctx, 50)
		if result == "" {
			t.Error("RenderPlanSidebarSection should return non-empty for multiple sessions")
		}
	})
}

func TestPlanPhase(t *testing.T) {
	phases := []struct {
		phase    PlanPhase
		expected string
	}{
		{PlanPhasePlanning, "planning"},
		{PlanPhaseReady, "ready"},
		{PlanPhaseExecuting, "executing"},
		{PlanPhaseComplete, "complete"},
		{PlanPhaseFailed, "failed"},
	}

	for _, tc := range phases {
		if string(tc.phase) != tc.expected {
			t.Errorf("PlanPhase %q != %q", tc.phase, tc.expected)
		}
	}
}

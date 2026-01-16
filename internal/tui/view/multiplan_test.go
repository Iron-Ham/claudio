package view

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestMultiPlanState_NewMultiPlanState(t *testing.T) {
	state := NewMultiPlanState()
	if state == nil {
		t.Fatal("NewMultiPlanState() returned nil")
	}
	if state.Sessions == nil {
		t.Error("Sessions map should be initialized")
	}
	if len(state.Sessions) != 0 {
		t.Errorf("Sessions map should be empty, got %d", len(state.Sessions))
	}
}

func TestMultiPlanState_AddSession(t *testing.T) {
	state := NewMultiPlanState()
	session := &MultiPlanSession{
		GroupID:   "group-1",
		Objective: "Test objective",
		Phase:     MultiPlanPhasePlanning,
	}

	state.AddSession(session)

	if len(state.Sessions) != 1 {
		t.Errorf("Sessions count = %d, want 1", len(state.Sessions))
	}
	if state.Sessions["group-1"] != session {
		t.Error("Session not stored correctly")
	}
}

func TestMultiPlanState_GetSession(t *testing.T) {
	state := NewMultiPlanState()
	session := &MultiPlanSession{
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
	var nilState *MultiPlanState
	got = nilState.GetSession("group-1")
	if got != nil {
		t.Error("GetSession on nil state should return nil")
	}
}

func TestMultiPlanState_RemoveSession(t *testing.T) {
	state := NewMultiPlanState()
	session := &MultiPlanSession{
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

func TestMultiPlanState_GetAllSessions(t *testing.T) {
	t.Run("empty state", func(t *testing.T) {
		state := NewMultiPlanState()
		sessions := state.GetAllSessions()
		if sessions != nil {
			t.Error("GetAllSessions should return nil for empty state")
		}
	})

	t.Run("nil state", func(t *testing.T) {
		var nilState *MultiPlanState
		sessions := nilState.GetAllSessions()
		if sessions != nil {
			t.Error("GetAllSessions should return nil for nil state")
		}
	})

	t.Run("multiple sessions sorted by group ID", func(t *testing.T) {
		state := NewMultiPlanState()
		state.AddSession(&MultiPlanSession{GroupID: "group-c", Objective: "C"})
		state.AddSession(&MultiPlanSession{GroupID: "group-a", Objective: "A"})
		state.AddSession(&MultiPlanSession{GroupID: "group-b", Objective: "B"})

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

func TestMultiPlanState_HasActiveSessions(t *testing.T) {
	t.Run("nil state", func(t *testing.T) {
		var nilState *MultiPlanState
		if nilState.HasActiveSessions() {
			t.Error("HasActiveSessions should return false for nil state")
		}
	})

	t.Run("empty state", func(t *testing.T) {
		state := NewMultiPlanState()
		if state.HasActiveSessions() {
			t.Error("HasActiveSessions should return false for empty state")
		}
	})

	t.Run("with sessions", func(t *testing.T) {
		state := NewMultiPlanState()
		state.AddSession(&MultiPlanSession{GroupID: "group-1"})
		if !state.HasActiveSessions() {
			t.Error("HasActiveSessions should return true when sessions exist")
		}
	})
}

func TestMultiPlanState_SessionCount(t *testing.T) {
	state := NewMultiPlanState()
	if state.SessionCount() != 0 {
		t.Errorf("SessionCount = %d for empty state, want 0", state.SessionCount())
	}

	state.AddSession(&MultiPlanSession{GroupID: "group-1"})
	state.AddSession(&MultiPlanSession{GroupID: "group-2"})

	if state.SessionCount() != 2 {
		t.Errorf("SessionCount = %d, want 2", state.SessionCount())
	}

	var nilState *MultiPlanState
	if nilState.SessionCount() != 0 {
		t.Errorf("SessionCount = %d for nil state, want 0", nilState.SessionCount())
	}
}

func TestRenderMultiPlanHeader(t *testing.T) {
	t.Run("nil multiplan state", func(t *testing.T) {
		ctx := MultiPlanRenderContext{
			MultiPlan: nil,
		}
		result := RenderMultiPlanHeader(ctx)
		if result != "" {
			t.Errorf("RenderMultiPlanHeader with nil state = %q, want empty", result)
		}
	})

	t.Run("empty multiplan state", func(t *testing.T) {
		ctx := MultiPlanRenderContext{
			MultiPlan: NewMultiPlanState(),
		}
		result := RenderMultiPlanHeader(ctx)
		if result != "" {
			t.Errorf("RenderMultiPlanHeader with empty state = %q, want empty", result)
		}
	})

	t.Run("single session planning phase", func(t *testing.T) {
		state := NewMultiPlanState()
		state.AddSession(&MultiPlanSession{
			GroupID:         "group-1",
			Objective:       "Test",
			Phase:           MultiPlanPhasePlanning,
			PlannerStatuses: []PlannerStatus{PlannerStatusWorking, PlannerStatusPending, PlannerStatusPending},
		})
		ctx := MultiPlanRenderContext{
			MultiPlan: state,
		}
		result := RenderMultiPlanHeader(ctx)
		if result == "" {
			t.Error("RenderMultiPlanHeader should return non-empty for single session")
		}
	})

	t.Run("multiple sessions", func(t *testing.T) {
		state := NewMultiPlanState()
		state.AddSession(&MultiPlanSession{
			GroupID: "group-1",
			Phase:   MultiPlanPhasePlanning,
		})
		state.AddSession(&MultiPlanSession{
			GroupID: "group-2",
			Phase:   MultiPlanPhaseComplete,
		})
		ctx := MultiPlanRenderContext{
			MultiPlan: state,
		}
		result := RenderMultiPlanHeader(ctx)
		if result == "" {
			t.Error("RenderMultiPlanHeader should return non-empty for multiple sessions")
		}
	})
}

func TestRenderMultiPlanSidebarSection(t *testing.T) {
	t.Run("nil multiplan state", func(t *testing.T) {
		ctx := MultiPlanRenderContext{
			MultiPlan: nil,
		}
		result := RenderMultiPlanSidebarSection(ctx, 40)
		if result != "" {
			t.Errorf("RenderMultiPlanSidebarSection with nil state = %q, want empty", result)
		}
	})

	t.Run("single session", func(t *testing.T) {
		state := NewMultiPlanState()
		state.AddSession(&MultiPlanSession{
			GroupID:         "group-1",
			Objective:       "Implement feature X",
			Phase:           MultiPlanPhasePlanning,
			PlannerStatuses: []PlannerStatus{PlannerStatusWorking, PlannerStatusCompleted, PlannerStatusPending},
			PlannerIDs:      []string{"planner-1", "planner-2", "planner-3"},
		})
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "planner-1", Status: orchestrator.StatusWorking},
				{ID: "planner-2", Status: orchestrator.StatusCompleted},
				{ID: "planner-3", Status: orchestrator.StatusPending},
			},
		}
		ctx := MultiPlanRenderContext{
			MultiPlan: state,
			Session:   session,
			Width:     50,
		}
		result := RenderMultiPlanSidebarSection(ctx, 50)
		if result == "" {
			t.Error("RenderMultiPlanSidebarSection should return non-empty for active session")
		}
	})

	t.Run("multiple sessions", func(t *testing.T) {
		state := NewMultiPlanState()
		state.AddSession(&MultiPlanSession{
			GroupID:         "group-1",
			Objective:       "First objective",
			Phase:           MultiPlanPhasePlanning,
			PlannerStatuses: []PlannerStatus{PlannerStatusWorking, PlannerStatusWorking, PlannerStatusWorking},
		})
		state.AddSession(&MultiPlanSession{
			GroupID:         "group-2",
			Objective:       "Second objective",
			Phase:           MultiPlanPhaseComplete,
			PlannerStatuses: []PlannerStatus{PlannerStatusCompleted, PlannerStatusCompleted, PlannerStatusCompleted},
		})
		ctx := MultiPlanRenderContext{
			MultiPlan: state,
			Width:     50,
		}
		result := RenderMultiPlanSidebarSection(ctx, 50)
		if result == "" {
			t.Error("RenderMultiPlanSidebarSection should return non-empty for multiple sessions")
		}
	})
}

func TestMultiPlanPhase(t *testing.T) {
	phases := []struct {
		phase    MultiPlanPhase
		expected string
	}{
		{MultiPlanPhasePlanning, "planning"},
		{MultiPlanPhaseEvaluating, "evaluating"},
		{MultiPlanPhaseReady, "ready"},
		{MultiPlanPhaseExecuting, "executing"},
		{MultiPlanPhaseComplete, "complete"},
		{MultiPlanPhaseFailed, "failed"},
	}

	for _, tc := range phases {
		if string(tc.phase) != tc.expected {
			t.Errorf("MultiPlanPhase %q != %q", tc.phase, tc.expected)
		}
	}
}

func TestPlannerStatus(t *testing.T) {
	statuses := []struct {
		status   PlannerStatus
		expected string
	}{
		{PlannerStatusPending, "pending"},
		{PlannerStatusWorking, "working"},
		{PlannerStatusCompleted, "completed"},
		{PlannerStatusFailed, "failed"},
	}

	for _, tc := range statuses {
		if string(tc.status) != tc.expected {
			t.Errorf("PlannerStatus %q != %q", tc.status, tc.expected)
		}
	}
}

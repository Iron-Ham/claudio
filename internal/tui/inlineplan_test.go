package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

// Helper to create a session-based InlinePlanState for tests
func newTestInlinePlanState(session *InlinePlanSession, groupID string) *InlinePlanState {
	state := NewInlinePlanState()
	if session != nil {
		state.AddSession(groupID, session)
	}
	return state
}

func TestDispatchInlineMultiPlanFileChecks_NilInlinePlan(t *testing.T) {
	m := Model{
		inlinePlan: nil,
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when inlinePlan is nil, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_NotMultiPass(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            false,
		AwaitingPlanCreation: true,
	}
	m := Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when not in multipass mode, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_NotAwaitingPlanCreation(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: false,
	}
	m := Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when not awaiting plan creation, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_NoPlannerIDs(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{},
	}
	m := Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	if cmds != nil {
		t.Errorf("expected nil when no planner IDs, got %v", cmds)
	}
}

func TestDispatchInlineMultiPlanFileChecks_SkipsProcessedPlanners(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-1", "planner-2", "planner-3"},
		ProcessedPlanners: map[int]bool{
			0: true, // planner-1 already processed
			1: true, // planner-2 already processed
			2: true, // planner-3 already processed
		},
		Objective: "test objective",
	}
	m := Model{
		inlinePlan:   newTestInlinePlanState(session, "test-group"),
		orchestrator: nil, // Will cause GetInstance to return nil
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	// All planners are processed, so no commands should be returned
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands when all planners processed, got %d", len(cmds))
	}
}

func TestDispatchInlineMultiPlanFileChecks_CreatesCommandsForUnprocessedPlanners(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-1", "planner-2", "planner-3"},
		ProcessedPlanners: map[int]bool{
			0: true, // Only planner-1 is processed
		},
		Objective: "test objective",
	}
	m := Model{
		inlinePlan:   newTestInlinePlanState(session, "test-group"),
		orchestrator: nil, // Commands will return nil when GetInstance fails
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	// Should create commands for planner-2 and planner-3
	if len(cmds) != 2 {
		t.Errorf("expected 2 commands for unprocessed planners, got %d", len(cmds))
	}
}

func TestHandleInlineMultiPlanFileCheckResult_NilInlinePlan(t *testing.T) {
	m := &Model{
		inlinePlan: nil,
	}

	msg := tuimsg.InlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         &orchestrator.PlanSpec{},
		StrategyName: "test",
		GroupID:      "test-group",
	}

	result, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
	if cmd != nil {
		t.Error("expected nil command when inlinePlan is nil")
	}
	resultModel := result.(*Model)
	if resultModel.inlinePlan != nil {
		t.Error("expected inlinePlan to remain nil")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_NotMultiPass(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            false,
		AwaitingPlanCreation: true,
	}
	m := &Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	msg := tuimsg.InlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         &orchestrator.PlanSpec{},
		StrategyName: "test",
		GroupID:      "test-group",
	}

	_, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
	if cmd != nil {
		t.Error("expected nil command when not in multipass mode")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_InvalidIndex(t *testing.T) {
	tests := []struct {
		name  string
		index int
	}{
		{"negative index", -1},
		{"index out of bounds", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &InlinePlanSession{
				MultiPass:            true,
				AwaitingPlanCreation: true,
				PlanningInstanceIDs:  []string{"planner-1"},
				ProcessedPlanners:    make(map[int]bool),
				CandidatePlans:       make([]*orchestrator.PlanSpec, 1),
			}
			m := &Model{
				inlinePlan: newTestInlinePlanState(session, "test-group"),
			}

			msg := tuimsg.InlineMultiPlanFileCheckResultMsg{
				Index:        tt.index,
				Plan:         &orchestrator.PlanSpec{},
				StrategyName: "test",
				GroupID:      "test-group",
			}

			_, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
			if cmd != nil {
				t.Error("expected nil command for invalid index")
			}
		})
	}
}

func TestHandleInlineMultiPlanFileCheckResult_SkipsAlreadyProcessed(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-1"},
		ProcessedPlanners: map[int]bool{
			0: true, // Already processed
		},
		CandidatePlans: make([]*orchestrator.PlanSpec, 1),
	}
	m := &Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	msg := tuimsg.InlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{{ID: "new"}}},
		StrategyName: "test",
		GroupID:      "test-group",
	}

	_, cmd := m.handleInlineMultiPlanFileCheckResult(msg)
	if cmd != nil {
		t.Error("expected nil command for already processed planner")
	}
	// Plan should not be updated
	if session.CandidatePlans[0] != nil {
		t.Error("plan should not be updated for already processed planner")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_StoresPlan(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-1", "planner-2", "planner-3"},
		ProcessedPlanners:    make(map[int]bool),
		CandidatePlans:       make([]*orchestrator.PlanSpec, 3),
		Objective:            "test",
	}
	m := &Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	testPlan := &orchestrator.PlanSpec{
		Summary: "test plan",
		Tasks:   []orchestrator.PlannedTask{{ID: "task-1", Title: "Test Task"}},
	}

	msg := tuimsg.InlineMultiPlanFileCheckResultMsg{
		Index:        1,
		Plan:         testPlan,
		StrategyName: "minimize-complexity",
		GroupID:      "test-group",
	}

	result, _ := m.handleInlineMultiPlanFileCheckResult(msg)
	resultModel := result.(*Model)

	// Check planner was marked as processed
	resultSession := resultModel.inlinePlan.GetSession("test-group")
	if !resultSession.ProcessedPlanners[1] {
		t.Error("planner should be marked as processed")
	}

	// Check plan was stored
	if resultSession.CandidatePlans[1] != testPlan {
		t.Error("plan should be stored in CandidatePlans")
	}

	// Check info message was updated
	if resultModel.infoMessage == "" {
		t.Error("info message should be updated")
	}
}

func TestHandleInlineMultiPlanFileCheckResult_AllPlansCollectedWithNoValidPlans(t *testing.T) {
	session := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-1"},
		ProcessedPlanners:    make(map[int]bool),
		CandidatePlans:       make([]*orchestrator.PlanSpec, 1),
		Objective:            "test",
	}
	m := &Model{
		inlinePlan: newTestInlinePlanState(session, "test-group"),
	}

	// Send a nil plan (simulating parse failure)
	msg := tuimsg.InlineMultiPlanFileCheckResultMsg{
		Index:        0,
		Plan:         nil,
		StrategyName: "test",
		GroupID:      "test-group",
	}

	result, _ := m.handleInlineMultiPlanFileCheckResult(msg)
	resultModel := result.(*Model)

	// Session should be removed because all planners failed
	if resultModel.inlinePlan.GetSession("test-group") != nil {
		t.Error("session should be removed when all planners fail")
	}

	// Error message should be set
	if resultModel.errorMessage == "" {
		t.Error("error message should be set when all planners fail")
	}
}

// Coverage: checkInlineMultiPlanFileAsync is not directly tested because:
// 1. It requires a full orchestrator setup with instances
// 2. It's an internal async function that's tested indirectly through integration
// 3. The dispatch function already tests the command creation

func TestStatFileFunction(t *testing.T) {
	// Test that statFile is a function that wraps os.Stat
	// This is the hook for testing file operations
	_, err := statFile("/nonexistent/path/that/should/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// Test for multiple concurrent sessions
func TestDispatchInlineMultiPlanFileChecks_MultipleSessions(t *testing.T) {
	// Create two multiplan sessions
	session1 := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-1a", "planner-1b"},
		ProcessedPlanners:    make(map[int]bool),
		Objective:            "objective 1",
	}
	session2 := &InlinePlanSession{
		MultiPass:            true,
		AwaitingPlanCreation: true,
		PlanningInstanceIDs:  []string{"planner-2a", "planner-2b", "planner-2c"},
		ProcessedPlanners: map[int]bool{
			0: true, // One planner already processed
		},
		Objective: "objective 2",
	}

	state := NewInlinePlanState()
	state.AddSession("group-1", session1)
	state.AddSession("group-2", session2)

	m := Model{
		inlinePlan:   state,
		orchestrator: nil,
	}

	cmds := m.dispatchInlineMultiPlanFileChecks()
	// Should create commands for:
	// - 2 planners from session1 (none processed)
	// - 2 planners from session2 (1 processed, 2 unprocessed)
	if len(cmds) != 4 {
		t.Errorf("expected 4 commands for multiple sessions, got %d", len(cmds))
	}
}

func TestInlinePlanState_SessionManagement(t *testing.T) {
	state := NewInlinePlanState()

	// Test adding sessions
	session1 := &InlinePlanSession{Objective: "obj1"}
	state.AddSession("group-1", session1)

	if state.GetSessionCount() != 1 {
		t.Errorf("expected 1 session, got %d", state.GetSessionCount())
	}

	// Test current session is set correctly
	if state.CurrentSessionID != "group-1" {
		t.Errorf("expected current session to be group-1, got %s", state.CurrentSessionID)
	}

	// Add another session
	session2 := &InlinePlanSession{Objective: "obj2"}
	state.AddSession("group-2", session2)

	if state.GetSessionCount() != 2 {
		t.Errorf("expected 2 sessions, got %d", state.GetSessionCount())
	}

	// Current session should be updated to the latest
	if state.CurrentSessionID != "group-2" {
		t.Errorf("expected current session to be group-2, got %s", state.CurrentSessionID)
	}

	// Test GetCurrentSession
	currentSession := state.GetCurrentSession()
	if currentSession != session2 {
		t.Error("GetCurrentSession should return the current session")
	}

	// Test GetSession
	if state.GetSession("group-1") != session1 {
		t.Error("GetSession should return correct session")
	}

	// Test RemoveSession
	state.RemoveSession("group-2")
	if state.GetSessionCount() != 1 {
		t.Errorf("expected 1 session after removal, got %d", state.GetSessionCount())
	}
	if state.GetSession("group-2") != nil {
		t.Error("removed session should be nil")
	}

	// Test HasActiveSessions
	if !state.HasActiveSessions() {
		t.Error("HasActiveSessions should return true when sessions exist")
	}

	state.RemoveSession("group-1")
	if state.HasActiveSessions() {
		t.Error("HasActiveSessions should return false when no sessions")
	}
}

func TestInlinePlanState_GetAwaitingObjectiveSession(t *testing.T) {
	state := NewInlinePlanState()

	// No sessions awaiting objective
	session1 := &InlinePlanSession{
		AwaitingObjective: false,
		Objective:         "already has objective",
	}
	state.AddSession("group-1", session1)

	if state.GetAwaitingObjectiveSession() != nil {
		t.Error("should return nil when no session awaiting objective")
	}

	// Add session awaiting objective
	session2 := &InlinePlanSession{
		AwaitingObjective: true,
	}
	state.AddSession("group-2", session2)

	result := state.GetAwaitingObjectiveSession()
	if result != session2 {
		t.Error("should return session awaiting objective")
	}
}

func TestToggleGraphView_FromFlatToGraph(t *testing.T) {
	m := &Model{
		sidebarMode: view.SidebarModeFlat,
	}

	m.toggleGraphView()

	if m.sidebarMode != view.SidebarModeGraph {
		t.Errorf("sidebarMode = %v, want %v", m.sidebarMode, view.SidebarModeGraph)
	}
	if m.previousSidebarMode != view.SidebarModeFlat {
		t.Errorf("previousSidebarMode = %v, want %v", m.previousSidebarMode, view.SidebarModeFlat)
	}
	if m.infoMessage != "Dependency graph view enabled" {
		t.Errorf("infoMessage = %q, want %q", m.infoMessage, "Dependency graph view enabled")
	}
}

func TestToggleGraphView_FromGroupedToGraph(t *testing.T) {
	m := &Model{
		sidebarMode: view.SidebarModeGrouped,
	}

	m.toggleGraphView()

	if m.sidebarMode != view.SidebarModeGraph {
		t.Errorf("sidebarMode = %v, want %v", m.sidebarMode, view.SidebarModeGraph)
	}
	if m.previousSidebarMode != view.SidebarModeGrouped {
		t.Errorf("previousSidebarMode = %v, want %v", m.previousSidebarMode, view.SidebarModeGrouped)
	}
	if m.infoMessage != "Dependency graph view enabled" {
		t.Errorf("infoMessage = %q, want %q", m.infoMessage, "Dependency graph view enabled")
	}
}

func TestToggleGraphView_FromGraphBackToFlat(t *testing.T) {
	m := &Model{
		sidebarMode:         view.SidebarModeGraph,
		previousSidebarMode: view.SidebarModeFlat,
	}

	m.toggleGraphView()

	if m.sidebarMode != view.SidebarModeFlat {
		t.Errorf("sidebarMode = %v, want %v", m.sidebarMode, view.SidebarModeFlat)
	}
	if m.infoMessage != "List view enabled" {
		t.Errorf("infoMessage = %q, want %q", m.infoMessage, "List view enabled")
	}
}

func TestToggleGraphView_FromGraphBackToGrouped(t *testing.T) {
	m := &Model{
		sidebarMode:         view.SidebarModeGraph,
		previousSidebarMode: view.SidebarModeGrouped,
	}

	m.toggleGraphView()

	if m.sidebarMode != view.SidebarModeGrouped {
		t.Errorf("sidebarMode = %v, want %v", m.sidebarMode, view.SidebarModeGrouped)
	}
	if m.infoMessage != "Grouped view enabled" {
		t.Errorf("infoMessage = %q, want %q", m.infoMessage, "Grouped view enabled")
	}
}

func TestToggleGraphView_RoundTripFromGrouped(t *testing.T) {
	// This test verifies the full round-trip: grouped -> graph -> grouped
	// This is the bug scenario reported by the user
	m := &Model{
		sidebarMode: view.SidebarModeGrouped,
	}

	// Toggle to graph view
	m.toggleGraphView()
	if m.sidebarMode != view.SidebarModeGraph {
		t.Errorf("after first toggle: sidebarMode = %v, want %v", m.sidebarMode, view.SidebarModeGraph)
	}

	// Toggle back - should return to grouped, not flat
	m.toggleGraphView()
	if m.sidebarMode != view.SidebarModeGrouped {
		t.Errorf("after second toggle: sidebarMode = %v, want %v (groups should be preserved)", m.sidebarMode, view.SidebarModeGrouped)
	}
}

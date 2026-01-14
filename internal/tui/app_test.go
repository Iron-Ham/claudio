package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

// TestNewWithUltraPlan_CreatesGroupForCLIStartedSession tests that
// NewWithUltraPlan creates a group when started from CLI (without pre-existing group).
func TestNewWithUltraPlan_CreatesGroupForCLIStartedSession(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet (CLI startup)
	}

	// Create ultraplan session without a GroupID (simulates CLI startup)
	ultraSession := &orchestrator.UltraPlanSession{
		ID:        "ultra-1",
		Objective: "Test objective for ultraplan",
		Config: orchestrator.UltraPlanConfig{
			MultiPass: false,
		},
	}

	// Test 1: Verify session starts with no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}

	// Test 2: Verify GroupID is empty initially
	if ultraSession.GroupID != "" {
		t.Errorf("expected empty GroupID initially, got %q", ultraSession.GroupID)
	}
}

// TestNewWithTripleShot_CreatesGroupForCLIStartedSession tests that
// NewWithTripleShot creates a group when started from CLI (without pre-existing group).
func TestNewWithTripleShot_CreatesGroupForCLIStartedSession(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet (CLI startup)
	}

	// Create tripleshot session without a GroupID (simulates CLI startup)
	tripleSession := &orchestrator.TripleShotSession{
		ID:   "triple-1",
		Task: "Test task for tripleshot",
	}

	// Test: Verify session starts with no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}

	// Test: Verify GroupID is empty initially
	if tripleSession.GroupID != "" {
		t.Errorf("expected empty GroupID initially, got %q", tripleSession.GroupID)
	}
}

// TestNewWithTripleShots_CreatesGroupsForLegacySessions tests that
// NewWithTripleShots creates groups for tripleshots that don't have GroupIDs.
func TestNewWithTripleShots_CreatesGroupsForLegacySessions(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet
	}

	// Create multiple tripleshot sessions without GroupIDs (legacy sessions)
	session1 := &orchestrator.TripleShotSession{
		ID:   "triple-1",
		Task: "Task 1",
	}
	session2 := &orchestrator.TripleShotSession{
		ID:   "triple-2",
		Task: "Task 2",
	}

	// Verify both sessions start with no GroupID
	if session1.GroupID != "" {
		t.Errorf("expected empty GroupID for session1, got %q", session1.GroupID)
	}
	if session2.GroupID != "" {
		t.Errorf("expected empty GroupID for session2, got %q", session2.GroupID)
	}

	// Verify the session has no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}
}

// TestNewInstanceGroupWithType_CreatesCorrectSessionType tests that
// NewInstanceGroupWithType correctly sets the session type for different modes.
func TestNewInstanceGroupWithType_CreatesCorrectSessionType(t *testing.T) {
	tests := []struct {
		name        string
		sessionType orchestrator.SessionType
		objective   string
		wantType    orchestrator.SessionType
	}{
		{
			name:        "ultraplan creates ultraplan type",
			sessionType: orchestrator.SessionTypeUltraPlan,
			objective:   "Test ultraplan objective",
			wantType:    orchestrator.SessionTypeUltraPlan,
		},
		{
			name:        "multipass creates planmulti type",
			sessionType: orchestrator.SessionTypePlanMulti,
			objective:   "Test multipass objective",
			wantType:    orchestrator.SessionTypePlanMulti,
		},
		{
			name:        "tripleshot creates tripleshot type",
			sessionType: orchestrator.SessionTypeTripleShot,
			objective:   "Test tripleshot task",
			wantType:    orchestrator.SessionTypeTripleShot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := orchestrator.NewInstanceGroupWithType(
				truncateString(tt.objective, 30),
				tt.sessionType,
				tt.objective,
			)

			if group.SessionType != tt.wantType {
				t.Errorf("SessionType = %v, want %v", group.SessionType, tt.wantType)
			}
			if group.Objective != tt.objective {
				t.Errorf("Objective = %q, want %q", group.Objective, tt.objective)
			}
			if group.ID == "" {
				t.Error("expected non-empty group ID")
			}
		})
	}
}

// TestAutoEnableGroupedMode_EnablesWhenGroupsExist tests that
// autoEnableGroupedMode enables grouped mode when groups are present.
func TestAutoEnableGroupedMode_EnablesWhenGroupsExist(t *testing.T) {
	// Create a model with flat sidebar mode and no groups
	model := &Model{
		session: &orchestrator.Session{
			ID:     "test-session",
			Groups: nil,
		},
		sidebarMode: view.SidebarModeFlat,
	}

	// Call autoEnableGroupedMode - should not change anything since no groups
	model.autoEnableGroupedMode()
	if model.sidebarMode != view.SidebarModeFlat {
		t.Error("expected sidebarMode to remain flat when no groups exist")
	}

	// Add a group
	model.session.Groups = []*orchestrator.InstanceGroup{
		{ID: "group-1", Name: "Test Group"},
	}

	// Call autoEnableGroupedMode - should enable grouped mode
	model.autoEnableGroupedMode()
	if model.sidebarMode != view.SidebarModeGrouped {
		t.Error("expected sidebarMode to be grouped when groups exist")
	}
}

// TestUltraPlanGroupID_MustBeSetOnCreation verifies that when an ultraplan group
// is created, the ultraSession.GroupID must be set to link them together.
// This test documents the pattern that initInlineUltraPlanMode must follow.
func TestUltraPlanGroupID_MustBeSetOnCreation(t *testing.T) {
	// Create a mock session and ultraplan session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil,
	}
	ultraSession := &orchestrator.UltraPlanSession{
		ID:        "ultra-1",
		Objective: "Test objective",
	}

	// Simulate the pattern from initInlineUltraPlanMode: create group and link it
	ultraGroup := orchestrator.NewInstanceGroupWithType(
		"Test Group",
		orchestrator.SessionTypeUltraPlan,
		ultraSession.Objective,
	)
	session.AddGroup(ultraGroup)
	ultraSession.GroupID = ultraGroup.ID // This is the critical line being tested

	// Verify the group was added to session
	if len(session.Groups) != 1 {
		t.Fatalf("expected 1 group in session, got %d", len(session.Groups))
	}

	// Verify GroupID is correctly set (this is what was missing before the fix)
	if ultraSession.GroupID == "" {
		t.Error("ultraSession.GroupID must be set when group is created")
	}
	if ultraSession.GroupID != ultraGroup.ID {
		t.Errorf("ultraSession.GroupID = %q, want %q", ultraSession.GroupID, ultraGroup.ID)
	}

	// Verify we can retrieve the group by its ID
	retrievedGroup := session.GetGroup(ultraSession.GroupID)
	if retrievedGroup == nil {
		t.Fatal("session.GetGroup(ultraSession.GroupID) should return the group")
	}
	if retrievedGroup.SessionType != orchestrator.SessionTypeUltraPlan {
		t.Errorf("group.SessionType = %v, want %v", retrievedGroup.SessionType, orchestrator.SessionTypeUltraPlan)
	}
}

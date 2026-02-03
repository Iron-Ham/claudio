package tripleshot

import (
	"encoding/json"
	"os"
	"testing"
)

// mockGroupWithSubGroups implements GroupWithSubGroupsInterface for testing
type mockGroupWithSubGroups struct {
	id        string
	instances []string
	subGroups map[string]*mockGroupWithSubGroups
}

func newMockGroupWithSubGroups(id string) *mockGroupWithSubGroups {
	return &mockGroupWithSubGroups{
		id:        id,
		instances: []string{},
		subGroups: make(map[string]*mockGroupWithSubGroups),
	}
}

func (g *mockGroupWithSubGroups) AddInstance(instanceID string) {
	g.instances = append(g.instances, instanceID)
}

func (g *mockGroupWithSubGroups) AddSubGroup(subGroup GroupInterface) {
	sg := subGroup.(*mockGroupWithSubGroups)
	g.subGroups[sg.id] = sg
}

func (g *mockGroupWithSubGroups) GetInstances() []string {
	return g.instances
}

func (g *mockGroupWithSubGroups) SetInstances(instances []string) {
	g.instances = instances
}

func (g *mockGroupWithSubGroups) GetID() string {
	return g.id
}

func (g *mockGroupWithSubGroups) RemoveInstance(instanceID string) {
	filtered := make([]string, 0, len(g.instances))
	for _, id := range g.instances {
		if id != instanceID {
			filtered = append(filtered, id)
		}
	}
	g.instances = filtered
}

func (g *mockGroupWithSubGroups) GetOrCreateSubGroup(id, name string) GroupInterface {
	// First check if sub-group with this name exists
	for _, sg := range g.subGroups {
		if sg.id == id {
			return sg
		}
	}
	// Create new
	sg := newMockGroupWithSubGroups(id)
	g.subGroups[id] = sg
	return sg
}

func (g *mockGroupWithSubGroups) GetSubGroupByID(id string) GroupInterface {
	if sg, ok := g.subGroups[id]; ok {
		return sg
	}
	return nil
}

func (g *mockGroupWithSubGroups) MoveSubGroupUnder(subGroupID, targetID, targetName string) bool {
	// Find sub-group to move
	sg, ok := g.subGroups[subGroupID]
	if !ok {
		return false
	}

	// Find or create target
	target, ok := g.subGroups[targetID]
	if !ok {
		target = newMockGroupWithSubGroups(targetID)
		g.subGroups[targetID] = target
	}

	// Move
	delete(g.subGroups, subGroupID)
	target.subGroups[subGroupID] = sg

	return true
}

// mockBaseSessionWithSubGroups creates a base session with sub-group support
type mockBaseSessionWithSubGroups struct {
	groups    map[string]GroupInterface
	instances map[string]InstanceInterface
}

func newMockBaseSessionWithSubGroups() *mockBaseSessionWithSubGroups {
	return &mockBaseSessionWithSubGroups{
		groups:    make(map[string]GroupInterface),
		instances: make(map[string]InstanceInterface),
	}
}

func (m *mockBaseSessionWithSubGroups) GetGroup(id string) GroupInterface {
	return m.groups[id]
}

func (m *mockBaseSessionWithSubGroups) GetGroupBySessionType(sessionType string) GroupInterface {
	return m.groups[sessionType]
}

func (m *mockBaseSessionWithSubGroups) GetInstance(id string) InstanceInterface {
	return m.instances[id]
}

func TestCoordinator_GetOrCreateAttemptSubGroup(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Adversarial = true
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	baseSession := newMockBaseSessionWithSubGroups()
	tripleGroup := newMockGroupWithSubGroups("tripleshot-group")
	baseSession.groups["tripleshot-group"] = tripleGroup

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Test creating sub-group for attempt 0
	subGroup := coord.getOrCreateAttemptSubGroup(tripleGroup, 0)
	if subGroup == nil {
		t.Fatal("getOrCreateAttemptSubGroup() returned nil")
	}

	// Verify the attempt has the sub-group ID set
	if session.Attempts[0].AttemptGroupID == "" {
		t.Error("AttemptGroupID should be set after creating sub-group")
	}

	// Test idempotency - calling again should return the same sub-group
	subGroup2 := coord.getOrCreateAttemptSubGroup(tripleGroup, 0)
	if subGroup2.GetID() != subGroup.GetID() {
		t.Error("getOrCreateAttemptSubGroup should return the same sub-group on repeated calls")
	}

	// Test creating sub-groups for different attempts
	for i := 1; i < 3; i++ {
		sg := coord.getOrCreateAttemptSubGroup(tripleGroup, i)
		if sg == nil {
			t.Errorf("getOrCreateAttemptSubGroup(%d) returned nil", i)
		}
		if session.Attempts[i].AttemptGroupID == "" {
			t.Errorf("Attempts[%d].AttemptGroupID should be set", i)
		}
	}
}

func TestCoordinator_MovePreviousRoundToSubGroup(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Adversarial = true
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	baseSession := newMockBaseSessionWithSubGroups()
	tripleGroup := newMockGroupWithSubGroups("tripleshot-group")
	baseSession.groups["tripleshot-group"] = tripleGroup

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Setup attempt group
	attemptGroup := coord.getOrCreateAttemptSubGroup(tripleGroup, 0)
	attemptGroupMock := attemptGroup.(*mockGroupWithSubGroups)

	// Add round 1 instances
	attemptGroupMock.AddInstance("impl-1")
	attemptGroupMock.AddInstance("reviewer-1")

	// Record round 1 history
	session.Attempts[0].RoundHistory = []AttemptRoundHistory{
		{Round: 1, ImplementerID: "impl-1", ReviewerID: "reviewer-1"},
	}

	// Move round 1 to sub-group
	coord.movePreviousRoundToSubGroup(attemptGroup, 0, 1)

	// Verify instances were moved out of main attempt group
	if len(attemptGroupMock.instances) != 0 {
		t.Errorf("attempt group should have 0 instances after move, got %d", len(attemptGroupMock.instances))
	}

	// Verify history was updated with sub-group ID
	if session.Attempts[0].RoundHistory[0].SubGroupID == "" {
		t.Error("SubGroupID should be set in round history after move")
	}

	// Verify Previous Rounds group ID was set
	if session.Attempts[0].PreviousRoundsGroupID == "" {
		t.Error("PreviousRoundsGroupID should be set")
	}
}

func TestCoordinator_GetCurrentRoundGroupForAttempt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Adversarial = true
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	baseSession := newMockBaseSessionWithSubGroups()
	tripleGroup := newMockGroupWithSubGroups("tripleshot-group")
	baseSession.groups["tripleshot-group"] = tripleGroup

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	attemptGroup := coord.getOrCreateAttemptSubGroup(tripleGroup, 0)

	// Round 1 - should return the attempt group directly
	group := coord.getCurrentRoundGroupForAttempt(attemptGroup, 0, 1)
	if group == nil {
		t.Fatal("getCurrentRoundGroupForAttempt(1) returned nil")
	}
	if group.GetID() != attemptGroup.GetID() {
		t.Error("Round 1 should return the attempt group directly")
	}

	// Setup round 1 history
	attemptGroupMock := attemptGroup.(*mockGroupWithSubGroups)
	attemptGroupMock.AddInstance("impl-1")
	attemptGroupMock.AddInstance("reviewer-1")
	session.Attempts[0].RoundHistory = []AttemptRoundHistory{
		{Round: 1, ImplementerID: "impl-1", ReviewerID: "reviewer-1"},
	}

	// Round 2 - should move round 1 to sub-group and return attempt group
	group2 := coord.getCurrentRoundGroupForAttempt(attemptGroup, 0, 2)
	if group2 == nil {
		t.Fatal("getCurrentRoundGroupForAttempt(2) returned nil")
	}

	// After moving, attempt group should have no instances
	if len(attemptGroupMock.instances) != 0 {
		t.Errorf("after round 2 call, attempt group should have 0 instances, got %d", len(attemptGroupMock.instances))
	}
}

func TestCoordinator_RecordCurrentRoundHistory(t *testing.T) {
	session := NewSession("test task", DefaultConfig())

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Record round 1 implementer
	coord.recordCurrentRoundHistory(0, 1, "impl-1", "")

	if len(session.Attempts[0].RoundHistory) != 1 {
		t.Errorf("RoundHistory should have 1 entry, got %d", len(session.Attempts[0].RoundHistory))
	}
	if session.Attempts[0].RoundHistory[0].ImplementerID != "impl-1" {
		t.Errorf("ImplementerID = %q, want %q", session.Attempts[0].RoundHistory[0].ImplementerID, "impl-1")
	}

	// Record round 1 reviewer
	coord.recordCurrentRoundHistory(0, 1, "", "reviewer-1")
	if session.Attempts[0].RoundHistory[0].ReviewerID != "reviewer-1" {
		t.Errorf("ReviewerID = %q, want %q", session.Attempts[0].RoundHistory[0].ReviewerID, "reviewer-1")
	}

	// Record round 2
	coord.recordCurrentRoundHistory(0, 2, "impl-2", "")
	if len(session.Attempts[0].RoundHistory) != 2 {
		t.Errorf("RoundHistory should have 2 entries, got %d", len(session.Attempts[0].RoundHistory))
	}
	if session.Attempts[0].RoundHistory[1].ImplementerID != "impl-2" {
		t.Errorf("Round 2 ImplementerID = %q, want %q", session.Attempts[0].RoundHistory[1].ImplementerID, "impl-2")
	}
}

func TestCoordinator_AdversarialFlow_SubGroupStructure(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.Adversarial = true
	cfg.MaxAdversarialRounds = 3
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	orch := newMockOrchestrator()
	baseSession := newMockBaseSessionWithSubGroups()
	tripleGroup := newMockGroupWithSubGroups("tripleshot-group")
	baseSession.groups["tripleshot-group"] = tripleGroup

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptStart:    func(attemptIndex int, instanceID string) {},
		OnReviewerStart:   func(attemptIndex int, instanceID string) {},
		OnReviewApproved:  func(attemptIndex int, score int) {},
		OnReviewRejected:  func(attemptIndex int, score int, issues []string) {},
		OnAttemptComplete: func(attemptIndex int) {},
		OnPhaseChange:     func(phase Phase) {},
	})

	// Start attempts - should create attempt sub-groups
	err := coord.StartAttempts()
	if err != nil {
		t.Fatalf("StartAttempts() error = %v", err)
	}

	// Verify attempt sub-groups were created
	for i := 0; i < 3; i++ {
		if session.Attempts[i].AttemptGroupID == "" {
			t.Errorf("Attempts[%d].AttemptGroupID should be set", i)
		}
	}

	// Simulate attempt 0 completion file
	completion := CompletionFile{
		AttemptIndex: 0,
		Status:       "complete",
		Summary:      "Implementation done",
		Approach:     "Test approach",
	}
	data, _ := json.MarshalIndent(completion, "", "  ")

	// Update worktree path for attempt 0
	session.Attempts[0].WorktreePath = tmpDir
	if err := os.WriteFile(CompletionFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	// Process attempt completion - should start reviewer and update history
	err = coord.ProcessAttemptCompletion(0)
	if err != nil {
		t.Fatalf("ProcessAttemptCompletion() error = %v", err)
	}

	// Verify round history has implementer
	if len(session.Attempts[0].RoundHistory) == 0 {
		t.Fatal("RoundHistory should have at least 1 entry")
	}
	if session.Attempts[0].RoundHistory[0].ImplementerID == "" {
		t.Error("Round 1 should have implementer ID recorded")
	}
}

func TestCoordinator_NonAdversarial_NoSubGroups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Adversarial = false // Non-adversarial mode
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()
	group := &mockGroup{id: "tripleshot-group"}
	baseSession.groups["tripleshot-group"] = group

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptStart: func(attemptIndex int, instanceID string) {},
	})

	// Start attempts
	err := coord.StartAttempts()
	if err != nil {
		t.Fatalf("StartAttempts() error = %v", err)
	}

	// Verify instances were added directly to main group (no sub-groups)
	if len(group.instances) != 3 {
		t.Errorf("main group should have 3 instances, got %d", len(group.instances))
	}

	// Verify no AttemptGroupIDs were set
	for i := 0; i < 3; i++ {
		if session.Attempts[i].AttemptGroupID != "" {
			t.Errorf("Attempts[%d].AttemptGroupID should be empty in non-adversarial mode, got %q",
				i, session.Attempts[i].AttemptGroupID)
		}
	}
}

func TestGetPreviousRoundsGroupIDForAttempt(t *testing.T) {
	tests := []struct {
		name         string
		session      *Session
		attemptIndex int
		want         string
	}{
		{
			name:         "nil session",
			session:      nil,
			attemptIndex: 0,
			want:         "",
		},
		{
			name: "valid attempt with ID",
			session: &Session{
				Attempts: [3]Attempt{
					{PreviousRoundsGroupID: "test-id"},
					{},
					{},
				},
			},
			attemptIndex: 0,
			want:         "test-id",
		},
		{
			name: "invalid attempt index (negative)",
			session: &Session{
				Attempts: [3]Attempt{
					{PreviousRoundsGroupID: "test-id"},
					{},
					{},
				},
			},
			attemptIndex: -1,
			want:         "",
		},
		{
			name: "invalid attempt index (too high)",
			session: &Session{
				Attempts: [3]Attempt{
					{PreviousRoundsGroupID: "test-id"},
					{},
					{},
				},
			},
			attemptIndex: 3,
			want:         "",
		},
		{
			name: "attempt without ID",
			session: &Session{
				Attempts: [3]Attempt{
					{},
					{},
					{},
				},
			},
			attemptIndex: 0,
			want:         "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPreviousRoundsGroupIDForAttempt(tt.session, tt.attemptIndex)
			if got != tt.want {
				t.Errorf("GetPreviousRoundsGroupIDForAttempt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCoordinator_CreateAttemptStubs_Adversarial_CreatesSubGroups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Adversarial = true
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	orch := newMockOrchestrator()
	baseSession := newMockBaseSessionWithSubGroups()
	tripleGroup := newMockGroupWithSubGroups("tripleshot-group")
	baseSession.groups["tripleshot-group"] = tripleGroup

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Create stubs - should create attempt sub-groups immediately
	instanceIDs, err := coord.CreateAttemptStubs()
	if err != nil {
		t.Fatalf("CreateAttemptStubs() error = %v", err)
	}

	// Verify all instance IDs are non-empty
	for i, id := range instanceIDs {
		if id == "" {
			t.Errorf("instanceIDs[%d] is empty", i)
		}
	}

	// Verify attempt sub-groups were created for all 3 attempts
	for i := 0; i < 3; i++ {
		if session.Attempts[i].AttemptGroupID == "" {
			t.Errorf("Attempts[%d].AttemptGroupID should be set in adversarial mode", i)
		}
	}

	// Verify sub-groups exist in the tripleGroup
	if len(tripleGroup.subGroups) != 3 {
		t.Errorf("tripleGroup should have 3 sub-groups, got %d", len(tripleGroup.subGroups))
	}

	// Verify round history was recorded for each attempt
	for i := 0; i < 3; i++ {
		if len(session.Attempts[i].RoundHistory) == 0 {
			t.Errorf("Attempts[%d].RoundHistory should have at least 1 entry", i)
		}
		if session.Attempts[i].RoundHistory[0].ImplementerID != instanceIDs[i] {
			t.Errorf("Attempts[%d].RoundHistory[0].ImplementerID = %q, want %q",
				i, session.Attempts[i].RoundHistory[0].ImplementerID, instanceIDs[i])
		}
	}

	// Verify instances are in sub-groups, not in main tripleGroup
	if len(tripleGroup.instances) != 0 {
		t.Errorf("main tripleGroup should have 0 direct instances in adversarial mode, got %d", len(tripleGroup.instances))
	}

	// Verify each sub-group has exactly 1 instance
	for attemptGroupID, sg := range tripleGroup.subGroups {
		if len(sg.instances) != 1 {
			t.Errorf("sub-group %q should have 1 instance, got %d", attemptGroupID, len(sg.instances))
		}
	}
}

func TestCoordinator_CreateAttemptStubs_NonAdversarial_NoSubGroups(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Adversarial = false // Non-adversarial mode
	session := NewSession("test task", cfg)
	session.GroupID = "tripleshot-group"

	orch := newMockOrchestrator()
	baseSession := newMockBaseSessionWithSubGroups()
	tripleGroup := newMockGroupWithSubGroups("tripleshot-group")
	baseSession.groups["tripleshot-group"] = tripleGroup

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Create stubs - should NOT create attempt sub-groups
	_, err := coord.CreateAttemptStubs()
	if err != nil {
		t.Fatalf("CreateAttemptStubs() error = %v", err)
	}

	// Verify NO attempt sub-groups were created
	for i := 0; i < 3; i++ {
		if session.Attempts[i].AttemptGroupID != "" {
			t.Errorf("Attempts[%d].AttemptGroupID should be empty in non-adversarial mode, got %q",
				i, session.Attempts[i].AttemptGroupID)
		}
	}

	// Verify no sub-groups were created
	if len(tripleGroup.subGroups) != 0 {
		t.Errorf("tripleGroup should have 0 sub-groups in non-adversarial mode, got %d", len(tripleGroup.subGroups))
	}

	// Verify instances are directly in tripleGroup
	if len(tripleGroup.instances) != 3 {
		t.Errorf("main tripleGroup should have 3 direct instances, got %d", len(tripleGroup.instances))
	}
}

func TestAttemptRoundHistory_SerializationRoundTrip(t *testing.T) {
	// Test that AttemptRoundHistory serializes and deserializes correctly
	original := AttemptRoundHistory{
		Round:         1,
		ImplementerID: "impl-1",
		ReviewerID:    "reviewer-1",
		SubGroupID:    "subgroup-1",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded AttemptRoundHistory
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Round != original.Round {
		t.Errorf("Round = %d, want %d", decoded.Round, original.Round)
	}
	if decoded.ImplementerID != original.ImplementerID {
		t.Errorf("ImplementerID = %q, want %q", decoded.ImplementerID, original.ImplementerID)
	}
	if decoded.ReviewerID != original.ReviewerID {
		t.Errorf("ReviewerID = %q, want %q", decoded.ReviewerID, original.ReviewerID)
	}
	if decoded.SubGroupID != original.SubGroupID {
		t.Errorf("SubGroupID = %q, want %q", decoded.SubGroupID, original.SubGroupID)
	}
}

package adversarial

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// Mock implementations for testing

type mockOrchestrator struct {
	addInstanceFunc           func(session SessionInterface, task string) (InstanceInterface, error)
	addInstanceToWorktreeFunc func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error)
	startInstanceFunc         func(inst InstanceInterface) error
	saveSessionFunc           func() error
}

func (m *mockOrchestrator) AddInstance(session SessionInterface, task string) (InstanceInterface, error) {
	if m.addInstanceFunc != nil {
		return m.addInstanceFunc(session, task)
	}
	return &mockInstance{id: "test-inst", worktreePath: "/tmp/test", branch: "test-branch"}, nil
}

func (m *mockOrchestrator) AddInstanceToWorktree(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
	if m.addInstanceToWorktreeFunc != nil {
		return m.addInstanceToWorktreeFunc(session, task, worktreePath, branch)
	}
	return &mockInstance{id: "test-inst", worktreePath: worktreePath, branch: "test-branch"}, nil
}

func (m *mockOrchestrator) StartInstance(inst InstanceInterface) error {
	if m.startInstanceFunc != nil {
		return m.startInstanceFunc(inst)
	}
	return nil
}

func (m *mockOrchestrator) SaveSession() error {
	if m.saveSessionFunc != nil {
		return m.saveSessionFunc()
	}
	return nil
}

type mockSession struct {
	groups    map[string]GroupInterface
	instances map[string]InstanceInterface
}

func newMockSession() *mockSession {
	return &mockSession{
		groups:    make(map[string]GroupInterface),
		instances: make(map[string]InstanceInterface),
	}
}

func (m *mockSession) GetGroup(id string) GroupInterface {
	return m.groups[id]
}

func (m *mockSession) GetGroupBySessionType(sessionType string) GroupInterface {
	return m.groups[sessionType]
}

func (m *mockSession) GetInstance(id string) InstanceInterface {
	return m.instances[id]
}

type mockInstance struct {
	id           string
	worktreePath string
	branch       string
}

func (m *mockInstance) GetID() string           { return m.id }
func (m *mockInstance) GetWorktreePath() string { return m.worktreePath }
func (m *mockInstance) GetBranch() string       { return m.branch }

type mockGroup struct {
	instances []string
}

func (m *mockGroup) AddInstance(instanceID string) {
	m.instances = append(m.instances, instanceID)
}

// Tests

func TestNewCoordinator(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	mockOrch := &mockOrchestrator{}
	baseSession := newMockSession()

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		Logger:       nil, // Uses NopLogger
		SessionType:  "adversarial",
	}

	coord := NewCoordinator(cfg)

	if coord == nil {
		t.Fatal("NewCoordinator returned nil")
	}
	if coord.Session() != advSession {
		t.Error("Session() should return the same session")
	}
	if coord.Manager() == nil {
		t.Error("Manager() should not be nil")
	}
}

func TestCoordinator_SetCallbacks(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	phaseChangeCalled := false
	cb := &CoordinatorCallbacks{
		OnPhaseChange: func(phase Phase) {
			phaseChangeCalled = true
		},
	}

	coord.SetCallbacks(cb)

	// Trigger a phase change through notifyPhaseChange
	coord.notifyPhaseChange(PhaseReviewing)

	if !phaseChangeCalled {
		t.Error("OnPhaseChange callback should have been called")
	}
}

func TestCoordinator_NotifyImplementerStart(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	var receivedRound int
	var receivedInstanceID string

	cb := &CoordinatorCallbacks{
		OnImplementerStart: func(round int, instanceID string) {
			receivedRound = round
			receivedInstanceID = instanceID
		},
	}
	coord.SetCallbacks(cb)

	coord.notifyImplementerStart(1, "inst-123")

	if receivedRound != 1 {
		t.Errorf("received round = %d, want 1", receivedRound)
	}
	if receivedInstanceID != "inst-123" {
		t.Errorf("received instanceID = %q, want %q", receivedInstanceID, "inst-123")
	}
}

func TestCoordinator_NotifyIncrementReady(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Start a round first so there's history to record to
	coord.Manager().StartRound()

	var receivedRound int
	var receivedIncrement *IncrementFile

	cb := &CoordinatorCallbacks{
		OnIncrementReady: func(round int, increment *IncrementFile) {
			receivedRound = round
			receivedIncrement = increment
		},
	}
	coord.SetCallbacks(cb)

	increment := &IncrementFile{
		Round:   1,
		Status:  "ready_for_review",
		Summary: "Test increment",
	}

	coord.notifyIncrementReady(1, increment)

	if receivedRound != 1 {
		t.Errorf("received round = %d, want 1", receivedRound)
	}
	if receivedIncrement == nil {
		t.Error("received increment should not be nil")
	}
}

func TestCoordinator_NotifyReviewerStart(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	var receivedRound int
	var receivedInstanceID string

	cb := &CoordinatorCallbacks{
		OnReviewerStart: func(round int, instanceID string) {
			receivedRound = round
			receivedInstanceID = instanceID
		},
	}
	coord.SetCallbacks(cb)

	coord.notifyReviewerStart(2, "reviewer-123")

	if receivedRound != 2 {
		t.Errorf("received round = %d, want 2", receivedRound)
	}
	if receivedInstanceID != "reviewer-123" {
		t.Errorf("received instanceID = %q, want %q", receivedInstanceID, "reviewer-123")
	}
}

func TestCoordinator_NotifyReviewReady_Approved(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Start a round first
	coord.Manager().StartRound()

	var reviewReadyCalled bool
	var approvedCalled bool
	var rejectedCalled bool

	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {
			reviewReadyCalled = true
		},
		OnApproved: func(round int, review *ReviewFile) {
			approvedCalled = true
		},
		OnRejected: func(round int, review *ReviewFile) {
			rejectedCalled = true
		},
	}
	coord.SetCallbacks(cb)

	review := &ReviewFile{
		Round:    1,
		Approved: true,
		Score:    9,
		Summary:  "Good work",
	}

	coord.notifyReviewReady(1, review)

	if !reviewReadyCalled {
		t.Error("OnReviewReady should have been called")
	}
	if !approvedCalled {
		t.Error("OnApproved should have been called")
	}
	if rejectedCalled {
		t.Error("OnRejected should not have been called for approved review")
	}
}

func TestCoordinator_NotifyReviewReady_Rejected(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Start a round first
	coord.Manager().StartRound()

	var reviewReadyCalled bool
	var approvedCalled bool
	var rejectedCalled bool

	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {
			reviewReadyCalled = true
		},
		OnApproved: func(round int, review *ReviewFile) {
			approvedCalled = true
		},
		OnRejected: func(round int, review *ReviewFile) {
			rejectedCalled = true
		},
	}
	coord.SetCallbacks(cb)

	review := &ReviewFile{
		Round:    1,
		Approved: false,
		Score:    5,
		Summary:  "Needs improvement",
	}

	coord.notifyReviewReady(1, review)

	if !reviewReadyCalled {
		t.Error("OnReviewReady should have been called")
	}
	if approvedCalled {
		t.Error("OnApproved should not have been called for rejected review")
	}
	if !rejectedCalled {
		t.Error("OnRejected should have been called")
	}
}

func TestCoordinator_NotifyComplete(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	var receivedSuccess bool
	var receivedSummary string

	cb := &CoordinatorCallbacks{
		OnComplete: func(success bool, summary string) {
			receivedSuccess = success
			receivedSummary = summary
		},
	}
	coord.SetCallbacks(cb)

	coord.notifyComplete(true, "All done")

	if !receivedSuccess {
		t.Error("expected success = true")
	}
	if receivedSummary != "All done" {
		t.Errorf("expected summary = %q, got %q", "All done", receivedSummary)
	}
}

func TestCoordinator_CheckIncrementReady(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Set the worktree path
	coord.SetWorktrees(tmpDir)

	// Initially no increment file
	ready, err := coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Error("expected ready = false when no increment file exists")
	}

	// Write an increment file
	incrementPath := IncrementFilePath(tmpDir)
	if err := os.WriteFile(incrementPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	ready, err = coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Error("expected ready = true when increment file exists")
	}
}

func TestCoordinator_CheckIncrementReady_EmptyWorktree(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Don't set worktree - should return false
	ready, err := coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Error("expected ready = false when worktree is empty")
	}
}

func TestCoordinator_CheckReviewReady(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Set the worktree path
	coord.SetWorktrees(tmpDir)

	// Initially no review file
	ready, err := coord.CheckReviewReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Error("expected ready = false when no review file exists")
	}

	// Write a review file
	reviewPath := ReviewFilePath(tmpDir)
	if err := os.WriteFile(reviewPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	ready, err = coord.CheckReviewReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ready {
		t.Error("expected ready = true when review file exists")
	}
}

func TestCoordinator_CheckReviewReady_EmptyWorktree(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Don't set worktree - should return false
	ready, err := coord.CheckReviewReady()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ready {
		t.Error("expected ready = false when worktree is empty")
	}
}

func TestCoordinator_GetAndSetWorktrees(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Initially empty
	if coord.GetImplementerWorktree() != "" {
		t.Error("expected empty implementer worktree initially")
	}

	// Set worktree
	coord.SetWorktrees("/tmp/test-worktree")

	if coord.GetImplementerWorktree() != "/tmp/test-worktree" {
		t.Errorf("expected worktree = %q, got %q", "/tmp/test-worktree", coord.GetImplementerWorktree())
	}
}

func TestCoordinator_Stop(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Stop should not panic and should return
	coord.Stop()
}

func TestCoordinator_StartImplementer_FirstRound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	mockOrch := &mockOrchestrator{
		addInstanceFunc: func(session SessionInterface, task string) (InstanceInterface, error) {
			return &mockInstance{id: "impl-inst", worktreePath: tmpDir, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	implementerStartCalled := false
	cb := &CoordinatorCallbacks{
		OnImplementerStart: func(round int, instanceID string) {
			implementerStartCalled = true
		},
		OnPhaseChange: func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	err = coord.StartImplementer()
	if err != nil {
		t.Fatalf("StartImplementer failed: %v", err)
	}

	if !implementerStartCalled {
		t.Error("OnImplementerStart should have been called")
	}
	if advSession.ImplementerID != "impl-inst" {
		t.Errorf("expected ImplementerID = %q, got %q", "impl-inst", advSession.ImplementerID)
	}
	if coord.GetImplementerWorktree() != tmpDir {
		t.Errorf("expected worktree = %q, got %q", tmpDir, coord.GetImplementerWorktree())
	}
	// Verify instance was added to group
	if len(group.instances) != 1 || group.instances[0] != "impl-inst" {
		t.Errorf("expected instance to be added to group, got %v", group.instances)
	}
}

func TestCoordinator_StartImplementer_SubsequentRound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	addInstanceToWorktreeCalled := false
	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			addInstanceToWorktreeCalled = true
			return &mockInstance{id: "impl-inst-r2", worktreePath: worktreePath, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Simulate having already done round 1
	coord.SetWorktrees(tmpDir)
	advSession.CurrentRound = 2
	advSession.History = append(advSession.History, Round{Round: 1, Review: &ReviewFile{
		Round:    1,
		Approved: false,
		Score:    5,
		Summary:  "Needs work",
		Issues:   []string{"Missing tests"},
	}})

	cb := &CoordinatorCallbacks{
		OnImplementerStart: func(round int, instanceID string) {},
		OnPhaseChange:      func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	err = coord.StartImplementer()
	if err != nil {
		t.Fatalf("StartImplementer failed: %v", err)
	}

	if !addInstanceToWorktreeCalled {
		t.Error("AddInstanceToWorktree should have been called for subsequent rounds")
	}
}

func TestCoordinator_StartImplementer_Error(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	expectedErr := errors.New("instance creation failed")
	mockOrch := &mockOrchestrator{
		addInstanceFunc: func(session SessionInterface, task string) (InstanceInterface, error) {
			return nil, expectedErr
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	err := coord.StartImplementer()
	if err == nil {
		t.Error("expected error from StartImplementer")
	}
}

func TestCoordinator_StartReviewer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			return &mockInstance{id: "reviewer-inst", worktreePath: worktreePath, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Set up the worktree as if implementer had run
	coord.SetWorktrees(tmpDir)

	reviewerStartCalled := false
	cb := &CoordinatorCallbacks{
		OnReviewerStart: func(round int, instanceID string) {
			reviewerStartCalled = true
		},
		OnPhaseChange: func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	increment := &IncrementFile{
		Round:   1,
		Status:  "ready_for_review",
		Summary: "Implemented feature",
	}

	err = coord.StartReviewer(increment)
	if err != nil {
		t.Fatalf("StartReviewer failed: %v", err)
	}

	if !reviewerStartCalled {
		t.Error("OnReviewerStart should have been called")
	}
	if advSession.ReviewerID != "reviewer-inst" {
		t.Errorf("expected ReviewerID = %q, got %q", "reviewer-inst", advSession.ReviewerID)
	}
}

func TestCoordinator_StartReviewer_Error(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	expectedErr := errors.New("reviewer creation failed")
	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			return nil, expectedErr
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	increment := &IncrementFile{
		Round:  1,
		Status: "ready_for_review",
	}

	err = coord.StartReviewer(increment)
	if err == nil {
		t.Error("expected error from StartReviewer")
	}
}

func TestCoordinator_ProcessIncrementCompletion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			return &mockInstance{id: "reviewer-inst", worktreePath: worktreePath, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	cb := &CoordinatorCallbacks{
		OnIncrementReady: func(round int, increment *IncrementFile) {},
		OnReviewerStart:  func(round int, instanceID string) {},
		OnPhaseChange:    func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	// Start a round
	coord.Manager().StartRound()

	// Write an increment file
	increment := IncrementFile{
		Round:         1,
		Status:        "ready_for_review",
		Summary:       "Test increment",
		FilesModified: []string{"test.go"},
		Approach:      "Test approach",
	}
	data, _ := json.MarshalIndent(increment, "", "  ")
	if err := os.WriteFile(IncrementFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	err = coord.ProcessIncrementCompletion()
	if err != nil {
		t.Fatalf("ProcessIncrementCompletion failed: %v", err)
	}

	// Verify reviewer was started
	if advSession.ReviewerID == "" {
		t.Error("expected ReviewerID to be set")
	}
}

func TestCoordinator_ProcessIncrementCompletion_Failed(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	phaseChanges := []Phase{}
	cb := &CoordinatorCallbacks{
		OnPhaseChange: func(phase Phase) {
			phaseChanges = append(phaseChanges, phase)
		},
	}
	coord.SetCallbacks(cb)

	// Write an increment file with failed status
	increment := IncrementFile{
		Round:   1,
		Status:  "failed",
		Summary: "Could not complete task",
	}
	data, _ := json.MarshalIndent(increment, "", "  ")
	if err := os.WriteFile(IncrementFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	err = coord.ProcessIncrementCompletion()
	if err == nil {
		t.Error("expected error when increment status is 'failed'")
	}

	// Should have transitioned to failed phase
	if len(phaseChanges) == 0 {
		t.Error("expected phase change to failed")
	}
}

func TestCoordinator_ProcessReviewCompletion_Approved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	baseSession := newMockSession()

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start a round
	coord.Manager().StartRound()

	var completeCalled bool
	var completeSuccess bool
	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {},
		OnApproved:    func(round int, review *ReviewFile) {},
		OnPhaseChange: func(phase Phase) {},
		OnComplete: func(success bool, summary string) {
			completeCalled = true
			completeSuccess = success
		},
	}
	coord.SetCallbacks(cb)

	// Write a review file with approval
	review := ReviewFile{
		Round:    1,
		Approved: true,
		Score:    9,
		Summary:  "Excellent work",
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessReviewCompletion()
	if err != nil {
		t.Fatalf("ProcessReviewCompletion failed: %v", err)
	}

	if !completeCalled {
		t.Error("OnComplete should have been called")
	}
	if !completeSuccess {
		t.Error("expected success = true for approved review")
	}
	if advSession.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestCoordinator_ProcessReviewCompletion_Rejected(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Config.MaxIterations = 5 // Allow more iterations
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	// For round 2+, AddInstanceToWorktree is used (not AddInstance)
	addInstanceToWorktreeCalled := false
	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			addInstanceToWorktreeCalled = true
			return &mockInstance{id: "impl-inst-r2", worktreePath: worktreePath, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start a round
	coord.Manager().StartRound()

	cb := &CoordinatorCallbacks{
		OnReviewReady:      func(round int, review *ReviewFile) {},
		OnRejected:         func(round int, review *ReviewFile) {},
		OnPhaseChange:      func(phase Phase) {},
		OnImplementerStart: func(round int, instanceID string) {},
	}
	coord.SetCallbacks(cb)

	// Write a review file with rejection
	review := ReviewFile{
		Round:           1,
		Approved:        false,
		Score:           5,
		Summary:         "Needs improvement",
		Issues:          []string{"Missing tests"},
		RequiredChanges: []string{"Add unit tests"},
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessReviewCompletion()
	if err != nil {
		t.Fatalf("ProcessReviewCompletion failed: %v", err)
	}

	// Should have advanced to next round and started implementer again
	if advSession.CurrentRound != 2 {
		t.Errorf("expected round = 2, got %d", advSession.CurrentRound)
	}
	if !addInstanceToWorktreeCalled {
		t.Error("AddInstanceToWorktree should have been called for next round")
	}
}

func TestCoordinator_ProcessReviewCompletion_MaxIterationsReached(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Config.MaxIterations = 1 // Allow only 1 iteration
	advSession.CurrentRound = 2         // Already exceeded

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start a round
	coord.Manager().StartRound()

	var failedPhaseCalled bool
	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {},
		OnRejected:    func(round int, review *ReviewFile) {},
		OnPhaseChange: func(phase Phase) {
			if phase == PhaseFailed {
				failedPhaseCalled = true
			}
		},
		OnComplete: func(success bool, summary string) {},
	}
	coord.SetCallbacks(cb)

	// Write a review file with rejection
	review := ReviewFile{
		Round:    2,
		Approved: false,
		Score:    5,
		Summary:  "Still needs work",
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessReviewCompletion()
	if err == nil {
		t.Error("expected error when max iterations reached")
	}

	if !failedPhaseCalled {
		t.Error("expected phase to transition to failed")
	}
}

func TestCoordinator_ProcessReviewCompletion_ScoreEnforcement(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Config.MinPassingScore = 8 // Score must be >= 8
	advSession.Config.MaxIterations = 5
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	mockOrch := &mockOrchestrator{
		addInstanceFunc: func(session SessionInterface, task string) (InstanceInterface, error) {
			return &mockInstance{id: "impl-inst-r2", worktreePath: tmpDir, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start a round
	coord.Manager().StartRound()

	var rejectedCalled bool
	var approvedCalled bool
	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {},
		OnRejected: func(round int, review *ReviewFile) {
			rejectedCalled = true
		},
		OnApproved: func(round int, review *ReviewFile) {
			approvedCalled = true
		},
		OnPhaseChange:      func(phase Phase) {},
		OnImplementerStart: func(round int, instanceID string) {},
	}
	coord.SetCallbacks(cb)

	// Write a review file that says approved but has a score below minimum
	review := ReviewFile{
		Round:    1,
		Approved: true, // Says approved
		Score:    6,    // But score is below 8
		Summary:  "Good but not great",
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessReviewCompletion()
	if err != nil {
		t.Fatalf("ProcessReviewCompletion failed: %v", err)
	}

	// Should have been treated as rejection due to score enforcement
	if !rejectedCalled {
		t.Error("OnRejected should have been called due to score enforcement")
	}
	if approvedCalled {
		t.Error("OnApproved should not have been called when score is below minimum")
	}
}

func TestCoordinator_NilCallbacks(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Don't set callbacks - all notify methods should not panic

	coord.notifyPhaseChange(PhaseReviewing)
	coord.notifyImplementerStart(1, "inst-123")
	coord.notifyReviewerStart(1, "reviewer-123")
	coord.notifyComplete(true, "Done")

	// Start a round for increment/review notifications
	coord.Manager().StartRound()

	coord.notifyIncrementReady(1, &IncrementFile{Round: 1, Status: "ready_for_review"})
	coord.notifyReviewReady(1, &ReviewFile{Round: 1, Score: 8, Approved: true})

	// If we got here without panic, the test passes
}

func TestCoordinator_ProcessRejectionAfterApproval_RestartsWorkflow(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Config.MaxIterations = 5
	advSession.CurrentRound = 1
	advSession.Phase = PhaseComplete // Simulate already approved/complete state
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	var addInstanceToWorktreeCalled bool
	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			addInstanceToWorktreeCalled = true
			return &mockInstance{id: "impl-inst-r2", worktreePath: tmpDir, branch: "test-branch"}, nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start the first round (so we have history)
	coord.Manager().StartRound()

	var rejectedCalled bool
	var implementerStartCalled bool
	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {},
		OnRejected: func(round int, review *ReviewFile) {
			rejectedCalled = true
		},
		OnPhaseChange:      func(phase Phase) {},
		OnImplementerStart: func(round int, instanceID string) { implementerStartCalled = true },
	}
	coord.SetCallbacks(cb)

	// Write a review file with rejection (user rejected the approval)
	review := ReviewFile{
		Round:           1,
		Approved:        false,
		Score:           5,
		Summary:         "Actually, this doesn't meet requirements",
		Issues:          []string{"Missing critical feature"},
		RequiredChanges: []string{"Add the missing feature"},
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessRejectionAfterApproval()
	if err != nil {
		t.Fatalf("ProcessRejectionAfterApproval failed: %v", err)
	}

	// Should have advanced to next round
	if advSession.CurrentRound != 2 {
		t.Errorf("expected round = 2, got %d", advSession.CurrentRound)
	}

	// Should have started implementer
	if !addInstanceToWorktreeCalled {
		t.Error("AddInstanceToWorktree should have been called for next round")
	}

	// Should have called rejection callback
	if !rejectedCalled {
		t.Error("OnRejected should have been called")
	}

	// Should have called implementer start callback
	if !implementerStartCalled {
		t.Error("OnImplementerStart should have been called")
	}

	// Phase should be implementing
	if advSession.Phase != PhaseImplementing {
		t.Errorf("expected phase = implementing, got %s", advSession.Phase)
	}

	// CompletedAt should be cleared
	if advSession.CompletedAt != nil {
		t.Error("CompletedAt should be nil after rejection")
	}

	// Review file should be removed
	if _, err := os.Stat(ReviewFilePath(tmpDir)); !os.IsNotExist(err) {
		t.Error("review file should have been removed after processing")
	}
}

func TestCoordinator_ProcessRejectionAfterApproval_IgnoresApproval(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseComplete
	advSession.CurrentRound = 1

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start a round so we have history
	coord.Manager().StartRound()

	var rejectedCalled bool
	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {},
		OnRejected: func(round int, review *ReviewFile) {
			rejectedCalled = true
		},
		OnPhaseChange: func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	// Write a review file that is still approved (user didn't reject)
	review := ReviewFile{
		Round:    1,
		Approved: true,
		Score:    9,
		Summary:  "Still approved",
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessRejectionAfterApproval()
	if err != nil {
		t.Fatalf("ProcessRejectionAfterApproval failed: %v", err)
	}

	// Should NOT have advanced to next round
	if advSession.CurrentRound != 1 {
		t.Errorf("expected round = 1, got %d", advSession.CurrentRound)
	}

	// Should NOT have called rejection callback
	if rejectedCalled {
		t.Error("OnRejected should NOT have been called for still-approved review")
	}

	// Phase should remain complete
	if advSession.Phase != PhaseComplete {
		t.Errorf("expected phase = complete, got %s", advSession.Phase)
	}

	// Review file should be removed even if still approved (to avoid re-processing)
	if _, err := os.Stat(ReviewFilePath(tmpDir)); !os.IsNotExist(err) {
		t.Error("review file should have been removed after processing")
	}
}

func TestCoordinator_ProcessRejectionAfterApproval_MaxIterationsReached(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Config.MaxIterations = 1 // Allow only 1 iteration
	advSession.CurrentRound = 2         // Already exceeded
	advSession.Phase = PhaseComplete

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Start a round for history
	coord.Manager().StartRound()

	var failedPhaseCalled bool
	cb := &CoordinatorCallbacks{
		OnReviewReady: func(round int, review *ReviewFile) {},
		OnRejected:    func(round int, review *ReviewFile) {},
		OnPhaseChange: func(phase Phase) {
			if phase == PhaseFailed {
				failedPhaseCalled = true
			}
		},
		OnComplete: func(success bool, summary string) {},
	}
	coord.SetCallbacks(cb)

	// Write a review file with rejection
	review := ReviewFile{
		Round:    2,
		Approved: false,
		Score:    5,
		Summary:  "Still needs work",
	}
	data, _ := json.MarshalIndent(review, "", "  ")
	if err := os.WriteFile(ReviewFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	err = coord.ProcessRejectionAfterApproval()
	if err == nil {
		t.Error("expected error when max iterations reached")
	}

	if !failedPhaseCalled {
		t.Error("expected phase to transition to failed")
	}
}

package adversarial

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
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

func (m *mockGroup) GetInstances() []string {
	return m.instances
}

func (m *mockGroup) RemoveInstance(instanceID string) {
	filtered := make([]string, 0, len(m.instances))
	for _, id := range m.instances {
		if id != instanceID {
			filtered = append(filtered, id)
		}
	}
	m.instances = filtered
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

// mockGroupWithSubGroups implements GroupWithSubGroupsInterface for testing
type mockGroupWithSubGroups struct {
	mockGroup
	subGroups    map[string]GroupInterface
	subGroupByID map[string]GroupInterface

	// Track MoveSubGroupUnder calls for test verification
	moveSubGroupUnderCalls []moveSubGroupUnderCall
}

// moveSubGroupUnderCall records a call to MoveSubGroupUnder for test assertions
type moveSubGroupUnderCall struct {
	SubGroupID string
	ParentID   string
	ParentName string
}

func newMockGroupWithSubGroups() *mockGroupWithSubGroups {
	return &mockGroupWithSubGroups{
		mockGroup:              mockGroup{instances: []string{}},
		subGroups:              make(map[string]GroupInterface),
		subGroupByID:           make(map[string]GroupInterface),
		moveSubGroupUnderCalls: []moveSubGroupUnderCall{},
	}
}

func (m *mockGroupWithSubGroups) GetOrCreateSubGroup(id, name string) GroupInterface {
	// First check if exists by name
	if existing, ok := m.subGroups[name]; ok {
		return existing
	}
	// Create new
	newSubGroup := &mockGroup{instances: []string{}}
	m.subGroups[name] = newSubGroup
	m.subGroupByID[id] = newSubGroup
	return newSubGroup
}

func (m *mockGroupWithSubGroups) GetSubGroupByName(name string) GroupInterface {
	return m.subGroups[name]
}

func (m *mockGroupWithSubGroups) GetSubGroupByID(id string) GroupInterface {
	return m.subGroupByID[id]
}

func (m *mockGroupWithSubGroups) MoveSubGroupUnder(subGroupID, parentID, parentName string) bool {
	// Record the call for test verification
	m.moveSubGroupUnderCalls = append(m.moveSubGroupUnderCalls, moveSubGroupUnderCall{
		SubGroupID: subGroupID,
		ParentID:   parentID,
		ParentName: parentName,
	})

	// Return true if the sub-group exists
	if _, ok := m.subGroupByID[subGroupID]; ok {
		return true
	}
	return false
}

func TestCoordinator_GetCurrentRoundGroup_EmptyGroupIDFallback(t *testing.T) {
	// Test that current round instances go directly in the main group (no sub-group)

	advSession := NewSession("test-session-123", "test task", DefaultConfig())
	// Ensure GroupID is empty
	advSession.GroupID = ""

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mockGroup := newMockGroupWithSubGroups()

	// Call getCurrentRoundGroup
	resultGroup := coord.getCurrentRoundGroup(mockGroup, 1)

	// Current round's instances go in the main group (which is returned)
	if resultGroup == nil {
		t.Fatal("expected group to be returned")
	}

	// Should return the main group directly
	if resultGroup != mockGroup {
		t.Error("expected main group to be returned for current round")
	}
}

func TestCoordinator_GetCurrentRoundGroup_WithGroupID(t *testing.T) {
	// Test that current round instances go in the main group regardless of GroupID

	advSession := NewSession("test-session-123", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-456"

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mockGroup := newMockGroupWithSubGroups()

	resultGroup := coord.getCurrentRoundGroup(mockGroup, 2)

	// Current round's instances go in the main group (which is returned)
	if resultGroup == nil {
		t.Fatal("expected group to be returned")
	}

	// Should return the main group directly
	if resultGroup != mockGroup {
		t.Error("expected main group to be returned for current round")
	}
}

func TestCoordinator_GetCurrentRoundGroup_Idempotent(t *testing.T) {
	// Test that calling getCurrentRoundGroup multiple times returns the same result

	advSession := NewSession("test-session-123", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-456"

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mockGroup := newMockGroupWithSubGroups()

	// Call getCurrentRoundGroup multiple times
	resultGroup1 := coord.getCurrentRoundGroup(mockGroup, 1)
	resultGroup2 := coord.getCurrentRoundGroup(mockGroup, 1)

	if resultGroup1 != resultGroup2 {
		t.Error("getCurrentRoundGroup should return the same group on repeated calls")
	}

	// Both should be the main group
	if resultGroup1 != mockGroup {
		t.Error("expected main group to be returned")
	}
}

func TestCoordinator_GetCurrentRoundGroup_GroupWithoutSubGroupSupport(t *testing.T) {
	// Test that groups without sub-group support return the main group

	advSession := NewSession("test-session-123", "test task", DefaultConfig())

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Use mockGroup which doesn't implement GroupWithSubGroupsInterface
	basicGroup := &mockGroup{}

	resultGroup := coord.getCurrentRoundGroup(basicGroup, 1)

	// Should return the original group
	if resultGroup != basicGroup {
		t.Error("should return the original group when sub-groups not supported")
	}
}

func TestCoordinator_HandleInstanceCompletion_ImplementerStuck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-stuck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.ImplementerID = "impl-123"
	advSession.Phase = PhaseImplementing

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	var stuckCallbackCalled bool
	var stuckRole StuckRole
	cb := &CoordinatorCallbacks{
		OnStuck: func(role StuckRole, instanceID string) {
			stuckCallbackCalled = true
			stuckRole = role
		},
		OnPhaseChange: func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	// First call starts the grace period - should NOT be stuck yet
	wasStuck := coord.HandleInstanceCompletion("impl-123", true, false)
	if wasStuck {
		t.Error("first call should return false (grace period started)")
	}
	if stuckCallbackCalled {
		t.Error("OnStuck callback should not be called during grace period")
	}

	// Simulate grace period elapsed by setting first completion time to the past
	pastTime := time.Now().Add(-StuckDetectionGracePeriod - time.Second)
	coord.implementerFirstCompleted = &pastTime

	// Second call after grace period - should be stuck now
	wasStuck = coord.HandleInstanceCompletion("impl-123", true, false)

	if !wasStuck {
		t.Error("expected HandleInstanceCompletion to return true for stuck condition after grace period")
	}

	if !stuckCallbackCalled {
		t.Error("OnStuck callback should have been called")
	}

	if stuckRole != StuckRoleImplementer {
		t.Errorf("expected stuckRole = %q, got %q", StuckRoleImplementer, stuckRole)
	}

	if advSession.Phase != PhaseStuck {
		t.Errorf("expected phase = %q, got %q", PhaseStuck, advSession.Phase)
	}

	if advSession.StuckRole != string(StuckRoleImplementer) {
		t.Errorf("expected StuckRole = %q, got %q", StuckRoleImplementer, advSession.StuckRole)
	}
}

func TestCoordinator_HandleInstanceCompletion_ReviewerStuck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-stuck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.ReviewerID = "reviewer-123"
	advSession.Phase = PhaseReviewing

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	var stuckCallbackCalled bool
	var stuckRole StuckRole
	cb := &CoordinatorCallbacks{
		OnStuck: func(role StuckRole, instanceID string) {
			stuckCallbackCalled = true
			stuckRole = role
		},
		OnPhaseChange: func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	// First call starts the grace period - should NOT be stuck yet
	wasStuck := coord.HandleInstanceCompletion("reviewer-123", true, false)
	if wasStuck {
		t.Error("first call should return false (grace period started)")
	}
	if stuckCallbackCalled {
		t.Error("OnStuck callback should not be called during grace period")
	}

	// Simulate grace period elapsed by setting first completion time to the past
	pastTime := time.Now().Add(-StuckDetectionGracePeriod - time.Second)
	coord.reviewerFirstCompleted = &pastTime

	// Second call after grace period - should be stuck now
	wasStuck = coord.HandleInstanceCompletion("reviewer-123", true, false)

	if !wasStuck {
		t.Error("expected HandleInstanceCompletion to return true for stuck condition after grace period")
	}

	if !stuckCallbackCalled {
		t.Error("OnStuck callback should have been called")
	}

	if stuckRole != StuckRoleReviewer {
		t.Errorf("expected stuckRole = %q, got %q", StuckRoleReviewer, stuckRole)
	}

	if advSession.Phase != PhaseStuck {
		t.Errorf("expected phase = %q, got %q", PhaseStuck, advSession.Phase)
	}
}

func TestCoordinator_HandleInstanceCompletion_NotStuck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-stuck-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.ImplementerID = "impl-123"
	advSession.Phase = PhaseImplementing

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// Write an increment file so the instance isn't stuck
	increment := IncrementFile{
		Round:         1,
		Status:        "ready_for_review",
		Summary:       "Implemented feature",
		FilesModified: []string{"file.go"},
		Approach:      "Test approach",
		Notes:         "Test notes",
	}
	data, _ := json.MarshalIndent(increment, "", "  ")
	if err := os.WriteFile(IncrementFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	// Simulate implementer completion WITH increment file
	wasStuck := coord.HandleInstanceCompletion("impl-123", true, false)

	if wasStuck {
		t.Error("expected HandleInstanceCompletion to return false when file exists")
	}

	if advSession.Phase != PhaseImplementing {
		t.Errorf("expected phase = %q, got %q", PhaseImplementing, advSession.Phase)
	}
}

func TestCoordinator_HandleInstanceCompletion_UnrelatedInstance(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.ImplementerID = "impl-123"
	advSession.Phase = PhaseImplementing

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Simulate completion of an unrelated instance
	wasStuck := coord.HandleInstanceCompletion("other-123", true, false)

	if wasStuck {
		t.Error("expected HandleInstanceCompletion to return false for unrelated instance")
	}
}

func TestCoordinator_HandleInstanceCompletion_GracePeriodResetOnFileAppear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-grace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.ImplementerID = "impl-123"
	advSession.Phase = PhaseImplementing

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// First call starts the grace period
	wasStuck := coord.HandleInstanceCompletion("impl-123", true, false)
	if wasStuck {
		t.Error("first call should return false (grace period started)")
	}
	if coord.implementerFirstCompleted == nil {
		t.Error("implementerFirstCompleted should be set after first call")
	}

	// Now write the increment file
	increment := IncrementFile{
		Round:         1,
		Status:        "ready_for_review",
		Summary:       "Implemented feature",
		FilesModified: []string{"file.go"},
		Approach:      "Test approach",
		Notes:         "Test notes",
	}
	data, _ := json.MarshalIndent(increment, "", "  ")
	if err := os.WriteFile(IncrementFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	// Call again - should reset grace period since file now exists
	wasStuck = coord.HandleInstanceCompletion("impl-123", true, false)
	if wasStuck {
		t.Error("should not be stuck when file exists")
	}
	if coord.implementerFirstCompleted != nil {
		t.Error("implementerFirstCompleted should be reset when file appears")
	}
}

func TestCoordinator_HandleInstanceCompletion_GracePeriodResetOnNotCompleted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-grace-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.ImplementerID = "impl-123"
	advSession.Phase = PhaseImplementing

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// First call with completed=true starts the grace period
	wasStuck := coord.HandleInstanceCompletion("impl-123", true, false)
	if wasStuck {
		t.Error("first call should return false (grace period started)")
	}
	if coord.implementerFirstCompleted == nil {
		t.Error("implementerFirstCompleted should be set after first call")
	}

	// Call again with completed=false (instance is working again) - should reset
	wasStuck = coord.HandleInstanceCompletion("impl-123", false, false)
	if wasStuck {
		t.Error("should not be stuck when not completed")
	}
	if coord.implementerFirstCompleted != nil {
		t.Error("implementerFirstCompleted should be reset when instance is not completed")
	}
}

func TestCoordinator_RestartStuckRole_Implementer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-restart-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = string(StuckRoleImplementer)
	advSession.WorktreePath = tmpDir
	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	var addInstanceCalled bool
	mockOrch := &mockOrchestrator{
		addInstanceFunc: func(session SessionInterface, task string) (InstanceInterface, error) {
			addInstanceCalled = true
			return &mockInstance{id: "new-impl", worktreePath: tmpDir, branch: "test-branch"}, nil
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
		OnPhaseChange:      func(phase Phase) {},
		OnImplementerStart: func(round int, instanceID string) {},
	}
	coord.SetCallbacks(cb)

	err = coord.RestartStuckRole()
	if err != nil {
		t.Fatalf("RestartStuckRole failed: %v", err)
	}

	if !addInstanceCalled {
		t.Error("AddInstance should have been called to restart implementer")
	}

	if advSession.Phase != PhaseImplementing {
		t.Errorf("expected phase = %q, got %q", PhaseImplementing, advSession.Phase)
	}

	if advSession.StuckRole != "" {
		t.Errorf("expected StuckRole to be cleared, got %q", advSession.StuckRole)
	}
}

func TestCoordinator_RestartStuckRole_NotStuck(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseImplementing // Not stuck

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	err := coord.RestartStuckRole()
	if err == nil {
		t.Error("expected error when trying to restart non-stuck session")
	}
}

func TestCoordinator_IsStuck(t *testing.T) {
	tests := []struct {
		name     string
		phase    Phase
		expected bool
	}{
		{"implementing", PhaseImplementing, false},
		{"reviewing", PhaseReviewing, false},
		{"approved", PhaseApproved, false},
		{"complete", PhaseComplete, false},
		{"failed", PhaseFailed, false},
		{"stuck", PhaseStuck, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			advSession := NewSession("test-id", "test task", DefaultConfig())
			advSession.Phase = tt.phase

			cfg := CoordinatorConfig{
				Orchestrator: &mockOrchestrator{},
				BaseSession:  newMockSession(),
				AdvSession:   advSession,
				SessionType:  "adversarial",
			}
			coord := NewCoordinator(cfg)

			if coord.IsStuck() != tt.expected {
				t.Errorf("IsStuck() = %v, want %v", coord.IsStuck(), tt.expected)
			}
		})
	}
}

func TestCoordinator_GetStuckRole(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = string(StuckRoleReviewer)

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	role := coord.GetStuckRole()
	if role != StuckRoleReviewer {
		t.Errorf("GetStuckRole() = %q, want %q", role, StuckRoleReviewer)
	}
}

func TestCoordinator_RestartStuckRole_Reviewer(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-restart-reviewer-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = string(StuckRoleReviewer)
	advSession.CurrentRound = 1
	advSession.WorktreePath = tmpDir
	// Add history with an increment (required for reviewer restart)
	advSession.History = []Round{
		{
			Round: 1,
			Increment: &IncrementFile{
				Round:         1,
				Status:        "ready_for_review",
				Summary:       "Test implementation",
				FilesModified: []string{"test.go"},
				Approach:      "Test approach",
				Notes:         "",
			},
		},
	}

	baseSession := newMockSession()

	// Create a group for the session type
	group := &mockGroup{}
	baseSession.groups["adversarial"] = group

	var addInstanceToWorktreeCalled bool
	var receivedWorktreePath string
	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			addInstanceToWorktreeCalled = true
			receivedWorktreePath = worktreePath
			return &mockInstance{id: "new-reviewer", worktreePath: worktreePath, branch: "test-branch"}, nil
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

	var reviewerStartCalled bool
	cb := &CoordinatorCallbacks{
		OnPhaseChange:   func(phase Phase) {},
		OnReviewerStart: func(round int, instanceID string) { reviewerStartCalled = true },
	}
	coord.SetCallbacks(cb)

	err = coord.RestartStuckRole()
	if err != nil {
		t.Fatalf("RestartStuckRole failed: %v", err)
	}

	if !addInstanceToWorktreeCalled {
		t.Error("AddInstanceToWorktree should have been called to restart reviewer")
	}

	if receivedWorktreePath != tmpDir {
		t.Errorf("expected worktree path = %q, got %q", tmpDir, receivedWorktreePath)
	}

	if !reviewerStartCalled {
		t.Error("OnReviewerStart callback should have been called")
	}

	if advSession.Phase != PhaseReviewing {
		t.Errorf("expected phase = %q, got %q", PhaseReviewing, advSession.Phase)
	}

	if advSession.StuckRole != "" {
		t.Errorf("expected StuckRole to be cleared, got %q", advSession.StuckRole)
	}

	if advSession.ReviewerID != "new-reviewer" {
		t.Errorf("expected ReviewerID = %q, got %q", "new-reviewer", advSession.ReviewerID)
	}
}

func TestCoordinator_RestartStuckRole_Reviewer_EmptyHistory(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = string(StuckRoleReviewer)
	advSession.CurrentRound = 1
	// Empty history - should fail
	advSession.History = []Round{}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	err := coord.RestartStuckRole()
	if err == nil {
		t.Error("expected error when trying to restart reviewer with empty history")
	}

	expectedErr := "no increment found to restart reviewer"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestCoordinator_RestartStuckRole_Reviewer_NilIncrement(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = string(StuckRoleReviewer)
	advSession.CurrentRound = 1
	// History with nil increment - should fail
	advSession.History = []Round{
		{
			Round:     1,
			Increment: nil, // Increment is nil
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	err := coord.RestartStuckRole()
	if err == nil {
		t.Error("expected error when trying to restart reviewer with nil increment")
	}

	expectedErr := "no increment found to restart reviewer"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestCoordinator_RestartStuckRole_NoStuckRole(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = "" // No stuck role recorded

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	err := coord.RestartStuckRole()
	if err == nil {
		t.Error("expected error when no stuck role is recorded")
	}

	expectedErr := "no stuck role recorded"
	if err.Error() != expectedErr {
		t.Errorf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestCoordinator_RestartStuckRole_PreservesStateOnFailure(t *testing.T) {
	// Test that if restart fails, the stuck state is preserved so user can retry
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.Phase = PhaseStuck
	advSession.StuckRole = string(StuckRoleImplementer)
	advSession.Error = "Original error message"
	advSession.CurrentRound = 1

	// Create an orchestrator that always fails to add instances
	// Note: For round 1, StartImplementer uses AddInstance (not AddInstanceToWorktree)
	mockOrch := &mockOrchestrator{
		addInstanceFunc: func(session SessionInterface, task string) (InstanceInterface, error) {
			return nil, fmt.Errorf("orchestrator failure")
		},
	}

	baseSession := newMockSession()
	// Create a group so the restart can find the adversarial group
	baseSession.groups["adversarial"] = &mockGroup{}

	cfg := CoordinatorConfig{
		Orchestrator: mockOrch,
		BaseSession:  baseSession,
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees("/tmp/test-worktree")

	// Attempt restart - should fail
	err := coord.RestartStuckRole()
	if err == nil {
		t.Fatal("expected error when orchestrator fails")
	}

	// Verify stuck state is preserved (not cleared)
	if advSession.Error == "" {
		t.Error("Error field should be preserved when restart fails")
	}

	if advSession.StuckRole == "" {
		t.Error("StuckRole should be preserved when restart fails")
	}

	// Note: Phase transitions to PhaseImplementing before the error occurs in StartImplementer,
	// so we can't rely on phase remaining PhaseStuck. The key invariant is that Error and
	// StuckRole are preserved so the user can see what happened and retry.
}

// Tests for getCurrentRoundGroup and movePreviousRoundInstancesToSubGroup

func TestCoordinator_GetCurrentRoundGroup_Round1(t *testing.T) {
	// For round 1, getCurrentRoundGroup should return the main group without
	// calling movePreviousRoundInstancesToSubGroup

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()

	result := coord.getCurrentRoundGroup(mainGroup, 1)

	// Should return the main group
	if result != mainGroup {
		t.Error("round 1 should return the main group directly")
	}

	// No sub-groups should be created for round 1
	if len(mainGroup.subGroups) != 0 {
		t.Errorf("no sub-groups should be created for round 1, got %d", len(mainGroup.subGroups))
	}
}

func TestCoordinator_GetCurrentRoundGroup_Round2_TriggersMoveOperation(t *testing.T) {
	// For round 2, getCurrentRoundGroup should call movePreviousRoundInstancesToSubGroup
	// to move round 1 instances to a sub-group

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.ID = "session-123"

	// Add round 1 to history with instance IDs
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
			ReviewerID:    "reviewer-1",
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	// Add round 1 instances to main group
	mainGroup.AddInstance("impl-1")
	mainGroup.AddInstance("reviewer-1")

	result := coord.getCurrentRoundGroup(mainGroup, 2)

	// Should return the main group
	if result == nil {
		t.Fatal("expected non-nil group")
	}

	// Round 1 instances should be moved to sub-group
	// The main group should no longer have round 1 instances
	for _, instID := range mainGroup.GetInstances() {
		if instID == "impl-1" || instID == "reviewer-1" {
			t.Errorf("round 1 instance %q should have been moved from main group", instID)
		}
	}

	// SubGroupID should be recorded in history
	if advSession.History[0].SubGroupID != "adv-group-1-round-1" {
		t.Errorf("expected SubGroupID = %q, got %q", "adv-group-1-round-1", advSession.History[0].SubGroupID)
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_NoSubGroupSupport(t *testing.T) {
	// When the group doesn't support sub-groups, the function should return early

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	// Use basic mockGroup that doesn't implement GroupWithSubGroupsInterface
	basicGroup := &mockGroup{instances: []string{"impl-1"}}

	// This should not panic and should leave instances unchanged
	coord.movePreviousRoundInstancesToSubGroup(basicGroup, 1)

	// Instance should still be in the group
	if len(basicGroup.GetInstances()) != 1 || basicGroup.GetInstances()[0] != "impl-1" {
		t.Error("instances should be unchanged when sub-groups not supported")
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_EmptyHistory(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.History = []Round{} // Empty history

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("some-instance")

	// Should return early without doing anything
	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// Instance should still be in main group
	if len(mainGroup.GetInstances()) != 1 {
		t.Error("instances should be unchanged with empty history")
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_InvalidRound(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.History = []Round{
		{Round: 1, ImplementerID: "impl-1"},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("impl-1")

	// Round 0 (invalid) should return early
	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 0)
	if len(mainGroup.GetInstances()) != 1 {
		t.Error("instances should be unchanged for round 0")
	}

	// Round 5 (beyond history) should return early
	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 5)
	if len(mainGroup.GetInstances()) != 1 {
		t.Error("instances should be unchanged for round beyond history")
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_NoInstances(t *testing.T) {
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	// History entry with no instance IDs
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "",
			ReviewerID:    "",
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()

	// Should return early without creating sub-groups
	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// No sub-groups should be created
	if len(mainGroup.subGroups) != 0 {
		t.Errorf("no sub-groups should be created when there are no instances, got %d", len(mainGroup.subGroups))
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_UsesSessionIDWhenGroupIDEmpty(t *testing.T) {
	advSession := NewSession("test-session-id", "test task", DefaultConfig())
	advSession.GroupID = "" // Empty GroupID, should fallback to session.ID
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("impl-1")

	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// The sub-group ID should use session.ID as prefix
	expectedSubGroupID := "test-session-id-round-1"
	if advSession.History[0].SubGroupID != expectedSubGroupID {
		t.Errorf("expected SubGroupID = %q, got %q", expectedSubGroupID, advSession.History[0].SubGroupID)
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_OnlyImplementer(t *testing.T) {
	// Test when only implementer ran (reviewer was never started)
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
			ReviewerID:    "", // No reviewer
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("impl-1")

	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// Implementer should be moved to sub-group
	if len(mainGroup.GetInstances()) != 0 {
		t.Errorf("main group should be empty, got %v", mainGroup.GetInstances())
	}

	// Sub-group should be created with the implementer
	subGroup := mainGroup.GetSubGroupByName("Round 1")
	if subGroup == nil {
		t.Fatal("expected sub-group 'Round 1' to be created")
	}

	subGroupInstances := subGroup.GetInstances()
	if len(subGroupInstances) != 1 || subGroupInstances[0] != "impl-1" {
		t.Errorf("expected sub-group to have impl-1, got %v", subGroupInstances)
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_MovesToPreviousRoundsContainer(t *testing.T) {
	// Test that the round sub-group is moved under "Previous Rounds" container
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
			ReviewerID:    "reviewer-1",
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("impl-1")
	mainGroup.AddInstance("reviewer-1")

	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// Verify SubGroupID is recorded in history
	if advSession.History[0].SubGroupID == "" {
		t.Error("SubGroupID should be recorded in history")
	}

	// Verify MoveSubGroupUnder was called with correct parameters
	if len(mainGroup.moveSubGroupUnderCalls) != 1 {
		t.Fatalf("expected 1 MoveSubGroupUnder call, got %d", len(mainGroup.moveSubGroupUnderCalls))
	}

	call := mainGroup.moveSubGroupUnderCalls[0]
	expectedSubGroupID := "adv-group-1-round-1"
	expectedParentID := "adv-group-1-previous-rounds"
	expectedParentName := PreviousRoundsGroupName

	if call.SubGroupID != expectedSubGroupID {
		t.Errorf("MoveSubGroupUnder SubGroupID = %q, want %q", call.SubGroupID, expectedSubGroupID)
	}
	if call.ParentID != expectedParentID {
		t.Errorf("MoveSubGroupUnder ParentID = %q, want %q", call.ParentID, expectedParentID)
	}
	if call.ParentName != expectedParentName {
		t.Errorf("MoveSubGroupUnder ParentName = %q, want %q", call.ParentName, expectedParentName)
	}
}

func TestCoordinator_MovePreviousRoundInstancesToSubGroup_IdempotencyGuard(t *testing.T) {
	// Test that calling movePreviousRoundInstancesToSubGroup twice for the same round
	// only creates one sub-group (prevents duplicate "Round N" entries in Previous Rounds).
	// This bug manifested when both StartImplementer and StartReviewer called
	// getCurrentRoundGroup for the same round > 1.
	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
			ReviewerID:    "reviewer-1",
		},
	}

	cfg := CoordinatorConfig{
		Orchestrator: &mockOrchestrator{},
		BaseSession:  newMockSession(),
		AdvSession:   advSession,
		SessionType:  "adversarial",
	}
	coord := NewCoordinator(cfg)

	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("impl-1")
	mainGroup.AddInstance("reviewer-1")

	// First call - should move instances and record SubGroupID
	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// Verify first call worked
	if advSession.History[0].SubGroupID == "" {
		t.Fatal("SubGroupID should be recorded after first call")
	}
	firstCallCount := len(mainGroup.moveSubGroupUnderCalls)
	if firstCallCount != 1 {
		t.Fatalf("expected 1 MoveSubGroupUnder call after first invocation, got %d", firstCallCount)
	}

	// Second call - should be a no-op due to idempotency guard (SubGroupID already set)
	coord.movePreviousRoundInstancesToSubGroup(mainGroup, 1)

	// Verify second call was a no-op - MoveSubGroupUnder should NOT be called again
	secondCallCount := len(mainGroup.moveSubGroupUnderCalls)
	if secondCallCount != 1 {
		t.Errorf("expected still 1 MoveSubGroupUnder call after second invocation (idempotency guard should prevent duplicate), got %d", secondCallCount)
	}
}

func TestCoordinator_StartImplementerThenReviewer_NoDuplicatePreviousRounds(t *testing.T) {
	// Integration test: verify that when StartImplementer and StartReviewer are both
	// called for round > 1, the previous round is only moved to "Previous Rounds" once.
	// This exercises the full public API flow rather than the internal function directly.
	tmpDir, err := os.MkdirTemp("", "adversarial-coord-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	advSession.GroupID = "adv-group-1"
	advSession.CurrentRound = 2
	// Round 1 history with a rejection review (to trigger round 2)
	advSession.History = []Round{
		{
			Round:         1,
			ImplementerID: "impl-1",
			ReviewerID:    "reviewer-1",
			Review: &ReviewFile{
				Round:    1,
				Approved: false,
				Score:    5,
				Summary:  "Needs work",
				Issues:   []string{"Missing tests"},
			},
		},
	}

	baseSession := newMockSession()
	mainGroup := newMockGroupWithSubGroups()
	mainGroup.AddInstance("impl-1")
	mainGroup.AddInstance("reviewer-1")
	baseSession.groups["adv-group-1"] = mainGroup

	mockOrch := &mockOrchestrator{
		addInstanceToWorktreeFunc: func(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error) {
			return &mockInstance{id: "new-inst", worktreePath: worktreePath, branch: "test-branch"}, nil
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
		OnImplementerStart: func(round int, instanceID string) {},
		OnReviewerStart:    func(round int, instanceID string) {},
		OnPhaseChange:      func(phase Phase) {},
	}
	coord.SetCallbacks(cb)

	// Start implementer for round 2 - this should move round 1 to Previous Rounds
	err = coord.StartImplementer()
	if err != nil {
		t.Fatalf("StartImplementer failed: %v", err)
	}

	moveCallsAfterImpl := len(mainGroup.moveSubGroupUnderCalls)
	if moveCallsAfterImpl != 1 {
		t.Fatalf("expected 1 MoveSubGroupUnder call after StartImplementer, got %d", moveCallsAfterImpl)
	}

	// Start reviewer for round 2 - this should NOT move round 1 again (idempotency)
	increment := &IncrementFile{
		Round:   2,
		Status:  "ready_for_review",
		Summary: "Implemented feature",
	}
	err = coord.StartReviewer(increment)
	if err != nil {
		t.Fatalf("StartReviewer failed: %v", err)
	}

	// Verify MoveSubGroupUnder was only called once total (not twice)
	moveCallsAfterReviewer := len(mainGroup.moveSubGroupUnderCalls)
	if moveCallsAfterReviewer != 1 {
		t.Errorf("expected still 1 MoveSubGroupUnder call after StartReviewer (idempotency guard), got %d", moveCallsAfterReviewer)
	}

	// Verify SubGroupID was set correctly
	if advSession.History[0].SubGroupID != "adv-group-1-round-1" {
		t.Errorf("expected SubGroupID = %q, got %q", "adv-group-1-round-1", advSession.History[0].SubGroupID)
	}
}

func TestCoordinator_CheckIncrementReady_CachesLocation(t *testing.T) {
	// Test that when an increment file is found in a non-standard location,
	// subsequent checks use the cached location for fast access.
	tmpDir, err := os.MkdirTemp("", "adversarial-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create increment file in a subdirectory (non-standard location)
	subDir := tmpDir + "/subdir"
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	incrementPath := subDir + "/" + IncrementFileName
	content := `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": ["a.go"], "approach": "test", "notes": ""}`
	if err := os.WriteFile(incrementPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		AdvSession: advSession,
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// First check - should find file in subdirectory via full search
	ready, err := coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("first CheckIncrementReady failed: %v", err)
	}
	if !ready {
		t.Fatal("expected increment file to be found on first check")
	}

	// Verify cache was populated
	coord.mu.Lock()
	cachedDir := coord.incrementFileDir
	coord.mu.Unlock()
	if cachedDir != subDir {
		t.Errorf("expected cached dir = %q, got %q", subDir, cachedDir)
	}

	// Second check - should use cache (fast path)
	ready, err = coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("second CheckIncrementReady failed: %v", err)
	}
	if !ready {
		t.Fatal("expected increment file to be found on second check (from cache)")
	}
}

func TestCoordinator_CheckIncrementReady_RateLimitsFullSearch(t *testing.T) {
	// Test that full directory searches are rate-limited to prevent UI lag.
	tmpDir, err := os.MkdirTemp("", "adversarial-ratelimit-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		AdvSession: advSession,
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// First check - file doesn't exist, triggers full search
	ready, err := coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("first CheckIncrementReady failed: %v", err)
	}
	if ready {
		t.Fatal("expected increment file to not be found")
	}

	// Verify full search timestamp was set
	coord.mu.Lock()
	firstSearchTime := coord.lastIncrementFullSearch
	coord.mu.Unlock()
	if firstSearchTime.IsZero() {
		t.Fatal("expected lastIncrementFullSearch to be set after first check")
	}

	// Immediate second check - should skip full search due to rate limiting
	ready, err = coord.CheckIncrementReady()
	if err != nil {
		t.Fatalf("second CheckIncrementReady failed: %v", err)
	}
	if ready {
		t.Fatal("expected increment file to not be found")
	}

	// Timestamp should be unchanged (no new full search)
	coord.mu.Lock()
	secondSearchTime := coord.lastIncrementFullSearch
	coord.mu.Unlock()
	if !secondSearchTime.Equal(firstSearchTime) {
		t.Error("expected lastIncrementFullSearch to be unchanged within rate limit window")
	}
}

func TestCoordinator_CheckIncrementReady_ClearsStaleCache(t *testing.T) {
	// Test that if a cached file location becomes invalid (file deleted),
	// the cache is cleared and full search resumes.
	tmpDir, err := os.MkdirTemp("", "adversarial-cache-clear-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create increment file in a subdirectory
	subDir := tmpDir + "/subdir"
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	incrementPath := subDir + "/" + IncrementFileName
	content := `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": ["a.go"], "approach": "test", "notes": ""}`
	if err := os.WriteFile(incrementPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		AdvSession: advSession,
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// First check - find and cache the location
	ready, _ := coord.CheckIncrementReady()
	if !ready {
		t.Fatal("expected increment file to be found")
	}

	// Remove the file
	if err := os.Remove(incrementPath); err != nil {
		t.Fatalf("failed to remove increment file: %v", err)
	}

	// Second check - should clear cache and return false
	ready, _ = coord.CheckIncrementReady()
	if ready {
		t.Fatal("expected increment file to not be found after deletion")
	}

	// Verify cache was cleared
	coord.mu.Lock()
	cachedDir := coord.incrementFileDir
	coord.mu.Unlock()
	if cachedDir != "" {
		t.Errorf("expected cache to be cleared, got %q", cachedDir)
	}
}

func TestCoordinator_CheckReviewReady_CachesLocation(t *testing.T) {
	// Test that when a review file is found in a non-standard location,
	// subsequent checks use the cached location for fast access.
	tmpDir, err := os.MkdirTemp("", "adversarial-review-cache-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create review file in a subdirectory (non-standard location)
	subDir := tmpDir + "/subdir"
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	reviewPath := subDir + "/" + ReviewFileName
	content := `{"round": 1, "approved": true, "score": 8, "strengths": ["good"], "issues": [], "suggestions": [], "summary": "test", "required_changes": []}`
	if err := os.WriteFile(reviewPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	advSession := NewSession("test-id", "test task", DefaultConfig())
	cfg := CoordinatorConfig{
		AdvSession: advSession,
	}
	coord := NewCoordinator(cfg)
	coord.SetWorktrees(tmpDir)

	// First check - should find file in subdirectory via full search
	ready, err := coord.CheckReviewReady()
	if err != nil {
		t.Fatalf("first CheckReviewReady failed: %v", err)
	}
	if !ready {
		t.Fatal("expected review file to be found on first check")
	}

	// Verify cache was populated
	coord.mu.Lock()
	cachedDir := coord.reviewFileDir
	coord.mu.Unlock()
	if cachedDir != subDir {
		t.Errorf("expected cached dir = %q, got %q", subDir, cachedDir)
	}

	// Second check - should use cache (fast path)
	ready, err = coord.CheckReviewReady()
	if err != nil {
		t.Fatalf("second CheckReviewReady failed: %v", err)
	}
	if !ready {
		t.Fatal("expected review file to be found on second check (from cache)")
	}
}

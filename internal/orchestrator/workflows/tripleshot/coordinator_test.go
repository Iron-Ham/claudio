package tripleshot

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

// Mock implementations for coordinator tests

type mockOrchestrator struct {
	addInstanceFunc   func(session SessionInterface, task string) (InstanceInterface, error)
	startInstanceFunc func(inst InstanceInterface) error
	saveSessionFunc   func() error

	addInstanceCalls   int
	startInstanceCalls int
	saveSessionCalls   int

	lastAddedInstances []InstanceInterface
}

func newMockOrchestrator() *mockOrchestrator {
	return &mockOrchestrator{
		lastAddedInstances: make([]InstanceInterface, 0),
	}
}

func (m *mockOrchestrator) AddInstance(session SessionInterface, task string) (InstanceInterface, error) {
	m.addInstanceCalls++
	if m.addInstanceFunc != nil {
		return m.addInstanceFunc(session, task)
	}
	inst := &mockInstance{
		id:           "mock-inst-" + string(rune('0'+m.addInstanceCalls)),
		worktreePath: "/tmp/mock-worktree-" + string(rune('0'+m.addInstanceCalls)),
		branch:       "mock-branch-" + string(rune('0'+m.addInstanceCalls)),
	}
	m.lastAddedInstances = append(m.lastAddedInstances, inst)
	return inst, nil
}

func (m *mockOrchestrator) StartInstance(inst InstanceInterface) error {
	m.startInstanceCalls++
	if m.startInstanceFunc != nil {
		return m.startInstanceFunc(inst)
	}
	return nil
}

func (m *mockOrchestrator) SaveSession() error {
	m.saveSessionCalls++
	if m.saveSessionFunc != nil {
		return m.saveSessionFunc()
	}
	return nil
}

type mockBaseSession struct {
	groups    map[string]GroupInterface
	instances map[string]InstanceInterface
}

func newMockBaseSession() *mockBaseSession {
	return &mockBaseSession{
		groups:    make(map[string]GroupInterface),
		instances: make(map[string]InstanceInterface),
	}
}

func (m *mockBaseSession) GetGroup(id string) GroupInterface {
	return m.groups[id]
}

func (m *mockBaseSession) GetGroupBySessionType(sessionType string) GroupInterface {
	return m.groups[sessionType]
}

func (m *mockBaseSession) GetInstance(id string) InstanceInterface {
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
	id          string
	instances   []string
	subGroups   []GroupInterface
	sessionType string
}

func (m *mockGroup) AddInstance(instanceID string) {
	m.instances = append(m.instances, instanceID)
}

func (m *mockGroup) AddSubGroup(subGroup GroupInterface) {
	m.subGroups = append(m.subGroups, subGroup)
}

func (m *mockGroup) GetInstances() []string {
	return m.instances
}

func (m *mockGroup) SetInstances(instances []string) {
	m.instances = instances
}

func (m *mockGroup) GetID() string {
	return m.id
}

// Test cases

func TestNewCoordinator(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()

	cfg := CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		Logger:        nil,
		SessionType:   "tripleshot",
	}

	coord := NewCoordinator(cfg)

	if coord == nil {
		t.Fatal("NewCoordinator returned nil")
	}
	if coord.Manager() == nil {
		t.Error("Manager() should not be nil")
	}
	if coord.Session() != session {
		t.Error("Session() should return the same session")
	}
}

func TestCoordinator_SetCallbacks(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var phaseChangeCalled bool
	callbacks := &CoordinatorCallbacks{
		OnPhaseChange: func(phase Phase) {
			phaseChangeCalled = true
		},
	}

	coord.SetCallbacks(callbacks)

	// Trigger callback
	coord.notifyPhaseChange(PhaseEvaluating)

	if !phaseChangeCalled {
		t.Error("OnPhaseChange callback was not called")
	}
}

func TestCoordinator_Manager(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	manager := coord.Manager()

	if manager == nil {
		t.Error("Manager() should not return nil")
	}
	if manager.Session() != session {
		t.Error("Manager should contain the correct session")
	}
}

func TestCoordinator_Session(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	got := coord.Session()

	if got != session {
		t.Error("Session() should return the correct session")
	}
}

func TestCoordinator_NotifyPhaseChange(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedPhase Phase
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnPhaseChange: func(phase Phase) {
			receivedPhase = phase
		},
	})

	coord.notifyPhaseChange(PhaseEvaluating)

	if session.Phase != PhaseEvaluating {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseEvaluating)
	}
	if receivedPhase != PhaseEvaluating {
		t.Errorf("callback received phase = %v, want %v", receivedPhase, PhaseEvaluating)
	}
	if orch.saveSessionCalls == 0 {
		t.Error("SaveSession should have been called")
	}
}

func TestCoordinator_NotifyAttemptStart(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedIndex int
	var receivedInstanceID string
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptStart: func(attemptIndex int, instanceID string) {
			receivedIndex = attemptIndex
			receivedInstanceID = instanceID
		},
	})

	coord.notifyAttemptStart(1, "inst-123")

	if receivedIndex != 1 {
		t.Errorf("callback received attemptIndex = %d, want 1", receivedIndex)
	}
	if receivedInstanceID != "inst-123" {
		t.Errorf("callback received instanceID = %q, want %q", receivedInstanceID, "inst-123")
	}
}

func TestCoordinator_NotifyAttemptComplete(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", Status: AttemptStatusWorking}
	orch := newMockOrchestrator()
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedIndex int
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptComplete: func(attemptIndex int) {
			receivedIndex = attemptIndex
		},
	})

	coord.notifyAttemptComplete(0)

	if receivedIndex != 0 {
		t.Errorf("callback received attemptIndex = %d, want 0", receivedIndex)
	}
	if orch.saveSessionCalls == 0 {
		t.Error("SaveSession should have been called")
	}
}

func TestCoordinator_NotifyAttemptFailed(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[1] = Attempt{InstanceID: "inst-2", Status: AttemptStatusWorking}
	orch := newMockOrchestrator()
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedIndex int
	var receivedReason string
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptFailed: func(attemptIndex int, reason string) {
			receivedIndex = attemptIndex
			receivedReason = reason
		},
	})

	coord.notifyAttemptFailed(1, "timeout exceeded")

	if receivedIndex != 1 {
		t.Errorf("callback received attemptIndex = %d, want 1", receivedIndex)
	}
	if receivedReason != "timeout exceeded" {
		t.Errorf("callback received reason = %q, want %q", receivedReason, "timeout exceeded")
	}
	if orch.saveSessionCalls == 0 {
		t.Error("SaveSession should have been called")
	}
}

func TestCoordinator_NotifyJudgeStart(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedInstanceID string
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnJudgeStart: func(instanceID string) {
			receivedInstanceID = instanceID
		},
	})

	coord.notifyJudgeStart("judge-inst")

	if receivedInstanceID != "judge-inst" {
		t.Errorf("callback received instanceID = %q, want %q", receivedInstanceID, "judge-inst")
	}
}

func TestCoordinator_NotifyEvaluationReady(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedEval *Evaluation
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnEvaluationReady: func(evaluation *Evaluation) {
			receivedEval = evaluation
		},
	})

	eval := &Evaluation{
		WinnerIndex:   1,
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Test reasoning",
	}

	coord.notifyEvaluationReady(eval)

	if receivedEval == nil {
		t.Error("callback should receive evaluation")
	}
	if receivedEval.WinnerIndex != 1 {
		t.Errorf("evaluation WinnerIndex = %d, want 1", receivedEval.WinnerIndex)
	}
	if orch.saveSessionCalls == 0 {
		t.Error("SaveSession should have been called")
	}
}

func TestCoordinator_NotifyComplete(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var receivedSuccess bool
	var receivedSummary string
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnComplete: func(success bool, summary string) {
			receivedSuccess = success
			receivedSummary = summary
		},
	})

	coord.notifyComplete(true, "Triple-shot completed successfully")

	if !receivedSuccess {
		t.Error("callback should receive success=true")
	}
	if receivedSummary != "Triple-shot completed successfully" {
		t.Errorf("callback received summary = %q, want %q", receivedSummary, "Triple-shot completed successfully")
	}
}

func TestCoordinator_StartAttempts(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()
	group := &mockGroup{}
	baseSession.groups["tripleshot"] = group

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var attemptStarts []int
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptStart: func(attemptIndex int, instanceID string) {
			attemptStarts = append(attemptStarts, attemptIndex)
		},
	})

	err := coord.StartAttempts()

	if err != nil {
		t.Fatalf("StartAttempts() error = %v", err)
	}
	if orch.addInstanceCalls != 3 {
		t.Errorf("AddInstance called %d times, want 3", orch.addInstanceCalls)
	}
	if orch.startInstanceCalls != 3 {
		t.Errorf("StartInstance called %d times, want 3", orch.startInstanceCalls)
	}
	if len(attemptStarts) != 3 {
		t.Errorf("OnAttemptStart called %d times, want 3", len(attemptStarts))
	}
	if len(group.instances) != 3 {
		t.Errorf("group should have 3 instances, got %d", len(group.instances))
	}
}

func TestCoordinator_StartAttempts_AddInstanceError(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()
	orch.addInstanceFunc = func(session SessionInterface, task string) (InstanceInterface, error) {
		return nil, errors.New("failed to create instance")
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	err := coord.StartAttempts()

	if err == nil {
		t.Error("StartAttempts() should return error")
	}
}

func TestCoordinator_StartAttempts_StartInstanceError(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()
	orch.startInstanceFunc = func(inst InstanceInterface) error {
		return errors.New("failed to start instance")
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	err := coord.StartAttempts()

	if err == nil {
		t.Error("StartAttempts() should return error")
	}
}

func TestCoordinator_CheckAttemptCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: tmpDir, Status: AttemptStatusWorking}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// No file yet
	complete, err := coord.CheckAttemptCompletion(0)
	if err != nil {
		t.Fatalf("CheckAttemptCompletion() error = %v", err)
	}
	if complete {
		t.Error("should not be complete when file doesn't exist")
	}

	// Create completion file
	completionPath := CompletionFilePath(tmpDir)
	if err := os.WriteFile(completionPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	complete, err = coord.CheckAttemptCompletion(0)
	if err != nil {
		t.Fatalf("CheckAttemptCompletion() error = %v", err)
	}
	if !complete {
		t.Error("should be complete when file exists")
	}
}

func TestCoordinator_CheckAttemptCompletion_InvalidIndex(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	_, err := coord.CheckAttemptCompletion(-1)
	if err == nil {
		t.Error("expected error for negative index")
	}

	_, err = coord.CheckAttemptCompletion(3)
	if err == nil {
		t.Error("expected error for index >= 3")
	}
}

func TestCoordinator_CheckAttemptCompletion_EmptyWorktree(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: "", Status: AttemptStatusWorking}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	complete, err := coord.CheckAttemptCompletion(0)

	if err != nil {
		t.Fatalf("CheckAttemptCompletion() error = %v", err)
	}
	if complete {
		t.Error("should not be complete when worktree is empty")
	}
}

func TestCoordinator_CheckJudgeCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.JudgeID = "judge-inst"

	baseSession := newMockBaseSession()
	baseSession.instances["judge-inst"] = &mockInstance{
		id:           "judge-inst",
		worktreePath: tmpDir,
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// No file yet
	complete, err := coord.CheckJudgeCompletion()
	if err != nil {
		t.Fatalf("CheckJudgeCompletion() error = %v", err)
	}
	if complete {
		t.Error("should not be complete when file doesn't exist")
	}

	// Create evaluation file
	evalPath := EvaluationFilePath(tmpDir)
	if err := os.WriteFile(evalPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	complete, err = coord.CheckJudgeCompletion()
	if err != nil {
		t.Fatalf("CheckJudgeCompletion() error = %v", err)
	}
	if !complete {
		t.Error("should be complete when file exists")
	}
}

func TestCoordinator_CheckJudgeCompletion_NoJudgeID(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.JudgeID = "" // No judge yet

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	complete, err := coord.CheckJudgeCompletion()

	if err != nil {
		t.Fatalf("CheckJudgeCompletion() error = %v", err)
	}
	if complete {
		t.Error("should not be complete when no judge ID")
	}
}

func TestCoordinator_CheckJudgeCompletion_JudgeNotFound(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.JudgeID = "nonexistent-judge"

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	_, err := coord.CheckJudgeCompletion()

	if err == nil {
		t.Error("expected error when judge instance not found")
	}
}

func TestCoordinator_ProcessAttemptCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: tmpDir, Status: AttemptStatusWorking}

	orch := newMockOrchestrator()
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var attemptCompleteCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptComplete: func(attemptIndex int) {
			attemptCompleteCalled = true
		},
	})

	// Write completion file
	completion := CompletionFile{
		AttemptIndex: 0,
		Status:       "complete",
		Summary:      "Test summary",
	}
	data, _ := json.MarshalIndent(completion, "", "  ")
	if err := os.WriteFile(CompletionFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	err := coord.ProcessAttemptCompletion(0)

	if err != nil {
		t.Fatalf("ProcessAttemptCompletion() error = %v", err)
	}
	if !attemptCompleteCalled {
		t.Error("OnAttemptComplete callback should have been called")
	}
}

func TestCoordinator_ProcessAttemptCompletion_Failed(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.Attempts[1] = Attempt{InstanceID: "inst-2", WorktreePath: tmpDir, Status: AttemptStatusWorking}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var attemptFailedCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptFailed: func(attemptIndex int, reason string) {
			attemptFailedCalled = true
		},
	})

	// Write completion file with failed status
	completion := CompletionFile{
		AttemptIndex: 1,
		Status:       "failed",
		Summary:      "Could not complete",
	}
	data, _ := json.MarshalIndent(completion, "", "  ")
	if err := os.WriteFile(CompletionFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	err := coord.ProcessAttemptCompletion(1)

	if err != nil {
		t.Fatalf("ProcessAttemptCompletion() error = %v", err)
	}
	if !attemptFailedCalled {
		t.Error("OnAttemptFailed callback should have been called")
	}
}

func TestCoordinator_ProcessAttemptCompletion_InvalidIndex(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	err := coord.ProcessAttemptCompletion(-1)
	if err == nil {
		t.Error("expected error for negative index")
	}

	err = coord.ProcessAttemptCompletion(3)
	if err == nil {
		t.Error("expected error for index >= 3")
	}
}

func TestCoordinator_ProcessAttemptCompletion_AllComplete(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	// First two attempts already complete
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: tmpDir, Status: AttemptStatusCompleted}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", WorktreePath: tmpDir, Status: AttemptStatusCompleted}
	// Third attempt working
	session.Attempts[2] = Attempt{InstanceID: "inst-3", WorktreePath: tmpDir, Status: AttemptStatusWorking}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptComplete: func(attemptIndex int) {},
	})

	// Write completion file for third attempt
	completion := CompletionFile{
		AttemptIndex: 2,
		Status:       "complete",
		Summary:      "Test summary",
	}
	data, _ := json.MarshalIndent(completion, "", "  ")
	if err := os.WriteFile(CompletionFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	err := coord.ProcessAttemptCompletion(2)

	if err != nil {
		t.Fatalf("ProcessAttemptCompletion() error = %v", err)
	}
	// Manager should have emitted AllAttemptsReady
}

func TestCoordinator_ProcessAttemptCompletion_NotEnoughSuccessful(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	// First two attempts failed
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: tmpDir, Status: AttemptStatusFailed}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", WorktreePath: tmpDir, Status: AttemptStatusFailed}
	// Third attempt working
	session.Attempts[2] = Attempt{InstanceID: "inst-3", WorktreePath: tmpDir, Status: AttemptStatusWorking}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var phaseChangeCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptFailed: func(attemptIndex int, reason string) {},
		OnPhaseChange: func(phase Phase) {
			phaseChangeCalled = true
		},
	})

	// Write completion file for third attempt (also failed)
	completion := CompletionFile{
		AttemptIndex: 2,
		Status:       "failed",
		Summary:      "Could not complete",
	}
	data, _ := json.MarshalIndent(completion, "", "  ")
	if err := os.WriteFile(CompletionFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	err := coord.ProcessAttemptCompletion(2)

	if err == nil {
		t.Error("expected error when fewer than 2 attempts succeeded")
	}
	if !phaseChangeCalled {
		t.Error("OnPhaseChange callback should have been called")
	}
}

func TestCoordinator_ProcessJudgeCompletion(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.JudgeID = "judge-inst"
	session.Attempts[0] = Attempt{InstanceID: "inst-1", Branch: "branch-1", Status: AttemptStatusCompleted}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", Branch: "branch-2", Status: AttemptStatusCompleted}
	session.Attempts[2] = Attempt{InstanceID: "inst-3", Branch: "branch-3", Status: AttemptStatusCompleted}

	baseSession := newMockBaseSession()
	baseSession.instances["judge-inst"] = &mockInstance{
		id:           "judge-inst",
		worktreePath: tmpDir,
	}

	orch := newMockOrchestrator()
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	var evalReadyCalled bool
	var completeCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnEvaluationReady: func(evaluation *Evaluation) {
			evalReadyCalled = true
		},
		OnPhaseChange: func(phase Phase) {},
		OnComplete: func(success bool, summary string) {
			completeCalled = true
		},
	})

	// Write evaluation file
	evaluation := Evaluation{
		WinnerIndex:   1,
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Attempt 2 is the best",
	}
	data, _ := json.MarshalIndent(evaluation, "", "  ")
	if err := os.WriteFile(EvaluationFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	err := coord.ProcessJudgeCompletion()

	if err != nil {
		t.Fatalf("ProcessJudgeCompletion() error = %v", err)
	}
	if !evalReadyCalled {
		t.Error("OnEvaluationReady callback should have been called")
	}
	if !completeCalled {
		t.Error("OnComplete callback should have been called")
	}
	if session.Phase != PhaseComplete {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseComplete)
	}
}

func TestCoordinator_ProcessJudgeCompletion_JudgeNotFound(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.JudgeID = "nonexistent-judge"

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	err := coord.ProcessJudgeCompletion()

	if err == nil {
		t.Error("expected error when judge instance not found")
	}
}

func TestCoordinator_ProcessJudgeCompletion_InvalidWinnerIndex(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.JudgeID = "judge-inst"
	session.Attempts[0] = Attempt{InstanceID: "inst-1", Branch: "branch-1", Status: AttemptStatusCompleted}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", Branch: "branch-2", Status: AttemptStatusCompleted}
	session.Attempts[2] = Attempt{InstanceID: "inst-3", Branch: "branch-3", Status: AttemptStatusCompleted}

	baseSession := newMockBaseSession()
	baseSession.instances["judge-inst"] = &mockInstance{
		id:           "judge-inst",
		worktreePath: tmpDir,
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnEvaluationReady: func(evaluation *Evaluation) {},
		OnPhaseChange:     func(phase Phase) {},
	})

	// Write evaluation file with invalid winner index
	evaluation := Evaluation{
		WinnerIndex:   5, // Invalid
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Invalid winner",
	}
	data, _ := json.MarshalIndent(evaluation, "", "  ")
	if err := os.WriteFile(EvaluationFilePath(tmpDir), data, 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	err := coord.ProcessJudgeCompletion()

	if err == nil {
		t.Error("expected error for invalid winner index")
	}
	if session.Phase != PhaseFailed {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseFailed)
	}
}

func TestCoordinator_Stop(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Should not panic
	coord.Stop()
}

func TestCoordinator_GetAttemptInstanceID(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	orch := newMockOrchestrator()

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Start attempts to populate runningAttempts
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptStart: func(attemptIndex int, instanceID string) {},
	})
	_ = coord.StartAttempts()

	// Check that we can get attempt instance IDs
	for i := range 3 {
		id := coord.GetAttemptInstanceID(i)
		if id == "" {
			t.Errorf("GetAttemptInstanceID(%d) returned empty string", i)
		}
	}
}

func TestCoordinator_GetWinningBranch(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{Branch: "branch-0"}
	session.Attempts[1] = Attempt{Branch: "branch-1"}
	session.Attempts[2] = Attempt{Branch: "branch-2"}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// No evaluation yet
	branch := coord.GetWinningBranch()
	if branch != "" {
		t.Errorf("GetWinningBranch() = %q, want empty string when no evaluation", branch)
	}

	// Set evaluation with select strategy
	session.Evaluation = &Evaluation{
		WinnerIndex:   1,
		MergeStrategy: MergeStrategySelect,
	}

	branch = coord.GetWinningBranch()
	if branch != "branch-1" {
		t.Errorf("GetWinningBranch() = %q, want %q", branch, "branch-1")
	}

	// Test with merge strategy (no winner)
	session.Evaluation.MergeStrategy = MergeStrategyMerge
	branch = coord.GetWinningBranch()
	if branch != "" {
		t.Errorf("GetWinningBranch() = %q, want empty string for merge strategy", branch)
	}
}

func TestCoordinator_GetWinningBranch_InvalidIndex(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{Branch: "branch-0"}
	session.Attempts[1] = Attempt{Branch: "branch-1"}
	session.Attempts[2] = Attempt{Branch: "branch-2"}
	session.Evaluation = &Evaluation{
		WinnerIndex:   -1, // Invalid
		MergeStrategy: MergeStrategySelect,
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	branch := coord.GetWinningBranch()
	if branch != "" {
		t.Errorf("GetWinningBranch() = %q, want empty string for invalid index", branch)
	}
}

func TestCoordinator_NilCallbacks(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  newMockOrchestrator(),
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	// Should not panic with nil callbacks
	coord.notifyAttemptStart(0, "inst-1")
	coord.notifyAttemptComplete(0)
	coord.notifyAttemptFailed(0, "test")
	coord.notifyJudgeStart("judge-inst")
	coord.notifyEvaluationReady(&Evaluation{})
	coord.notifyComplete(true, "test")
}

func TestCoordinator_StartAttempts_WithGroupID(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.GroupID = "specific-group-id"

	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()
	group := &mockGroup{}
	baseSession.groups["specific-group-id"] = group

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	coord.SetCallbacks(&CoordinatorCallbacks{
		OnAttemptStart: func(attemptIndex int, instanceID string) {},
	})

	err := coord.StartAttempts()

	if err != nil {
		t.Fatalf("StartAttempts() error = %v", err)
	}
	if len(group.instances) != 3 {
		t.Errorf("group should have 3 instances, got %d", len(group.instances))
	}
}

func TestCoordinator_StartJudge(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: tmpDir, Branch: "branch-1", Status: AttemptStatusCompleted}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", WorktreePath: tmpDir, Branch: "branch-2", Status: AttemptStatusCompleted}
	session.Attempts[2] = Attempt{InstanceID: "inst-3", WorktreePath: tmpDir, Branch: "branch-3", Status: AttemptStatusCompleted}

	// Write completion files for all attempts
	for i := range 3 {
		completion := CompletionFile{
			AttemptIndex: i,
			Status:       "complete",
			Summary:      "Test summary",
			Approach:     "Test approach",
		}
		data, _ := json.MarshalIndent(completion, "", "  ")
		if err := os.WriteFile(CompletionFilePath(tmpDir), data, 0644); err != nil {
			t.Fatalf("failed to write completion file: %v", err)
		}
	}

	orch := newMockOrchestrator()
	baseSession := newMockBaseSession()
	group := &mockGroup{instances: []string{"inst-1", "inst-2", "inst-3"}}
	baseSession.groups["tripleshot"] = group

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   baseSession,
		TripleSession: session,
		SessionType:   "tripleshot",
		NewGroup: func(name string) GroupInterface {
			return &mockGroup{}
		},
		SetSessionType: func(g GroupInterface, sessionType string) {
			if mg, ok := g.(*mockGroup); ok {
				mg.sessionType = sessionType
			}
		},
	})

	var judgeStartCalled bool
	coord.SetCallbacks(&CoordinatorCallbacks{
		OnPhaseChange: func(phase Phase) {},
		OnJudgeStart: func(instanceID string) {
			judgeStartCalled = true
		},
	})

	err := coord.StartJudge()

	if err != nil {
		t.Fatalf("StartJudge() error = %v", err)
	}
	if !judgeStartCalled {
		t.Error("OnJudgeStart callback should have been called")
	}
	if session.JudgeID == "" {
		t.Error("session.JudgeID should be set")
	}
	if session.Phase != PhaseEvaluating {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseEvaluating)
	}
}

func TestCoordinator_StartJudge_Error(t *testing.T) {
	tmpDir := t.TempDir()

	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", WorktreePath: tmpDir, Branch: "branch-1", Status: AttemptStatusCompleted}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", WorktreePath: tmpDir, Branch: "branch-2", Status: AttemptStatusCompleted}
	session.Attempts[2] = Attempt{InstanceID: "inst-3", WorktreePath: tmpDir, Branch: "branch-3", Status: AttemptStatusCompleted}

	// Write completion files
	for i := range 3 {
		completion := CompletionFile{AttemptIndex: i, Status: "complete", Summary: "Test"}
		data, _ := json.MarshalIndent(completion, "", "  ")
		_ = os.WriteFile(CompletionFilePath(tmpDir), data, 0644)
	}

	orch := newMockOrchestrator()
	orch.addInstanceFunc = func(session SessionInterface, task string) (InstanceInterface, error) {
		return nil, errors.New("failed to create judge")
	}

	coord := NewCoordinator(CoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   newMockBaseSession(),
		TripleSession: session,
		SessionType:   "tripleshot",
	})

	err := coord.StartJudge()

	if err == nil {
		t.Error("StartJudge() should return error")
	}
}

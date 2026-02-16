package teamwire

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	ts "github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

// --- Test helpers and mocks ---

// testOrch provides a controllable OrchestratorInterface for tests.
// It creates mock instances and tracks calls.
type testOrch struct {
	mu        sync.Mutex
	instances map[string]*mockInstance
	nextID    int
	createErr error
	startErr  error
	tempDirFn func() string // injected from test via t.TempDir for auto-cleanup
}

func newTestOrch(t *testing.T) *testOrch {
	return &testOrch{instances: make(map[string]*mockInstance), tempDirFn: t.TempDir}
}

func (o *testOrch) AddInstance(session ts.SessionInterface, _ string) (ts.InstanceInterface, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.createErr != nil {
		return nil, o.createErr
	}

	id := o.nextInstanceID()
	dir := o.tempDirFn()
	inst := &mockInstance{
		id:           id,
		worktreePath: dir,
		branch:       "branch-" + id,
	}
	o.instances[id] = inst

	// Also register in session for lookup.
	if ts, ok := session.(*testSession); ok {
		ts.mu.Lock()
		ts.instances[id] = inst
		ts.mu.Unlock()
	}

	return inst, nil
}

func (o *testOrch) AddInstanceToWorktree(_ ts.SessionInterface, _, _, _ string) (ts.InstanceInterface, error) {
	return nil, nil
}

func (o *testOrch) StartInstance(_ ts.InstanceInterface) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.startErr
}

func (o *testOrch) SaveSession() error { return nil }

func (o *testOrch) AddInstanceStub(_ ts.SessionInterface, _ string) (ts.InstanceInterface, error) {
	return nil, nil
}

func (o *testOrch) CompleteInstanceSetupByID(_ ts.SessionInterface, _ string) error {
	return nil
}

func (o *testOrch) nextInstanceID() string {
	o.nextID++
	return "inst-" + string(rune('a'+o.nextID-1))
}

func (o *testOrch) getInstances() map[string]*mockInstance {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make(map[string]*mockInstance, len(o.instances))
	for k, v := range o.instances {
		out[k] = v
	}
	return out
}

// testSession provides a thread-safe SessionInterface for tests.
type testSession struct {
	mu        sync.Mutex
	instances map[string]*mockInstance
	groups    map[string]*mockGroup
}

func newTestSession() *testSession {
	return &testSession{
		instances: make(map[string]*mockInstance),
		groups:    make(map[string]*mockGroup),
	}
}

func (s *testSession) GetGroup(id string) ts.GroupInterface {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.groups[id]
	if g == nil {
		return nil
	}
	return g
}
func (s *testSession) GetGroupBySessionType(_ string) ts.GroupInterface { return nil }
func (s *testSession) GetInstance(id string) ts.InstanceInterface {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst := s.instances[id]
	if inst == nil {
		return nil
	}
	return inst
}

// mockGroup implements both ts.GroupInterface and ts.GroupWithSubGroupsInterface.
// Not thread-safe — only use in single-goroutine tests (e.g., direct method calls).
// For concurrent/full-lifecycle tests, add a sync.Mutex.
type mockGroup struct {
	id        string
	instances []string
	subGroups map[string]*mockGroup
}

func newMockGroup(id string) *mockGroup {
	return &mockGroup{id: id, subGroups: make(map[string]*mockGroup)}
}

func (g *mockGroup) GetID() string             { return g.id }
func (g *mockGroup) GetInstances() []string    { return g.instances }
func (g *mockGroup) SetInstances(ids []string) { g.instances = ids }
func (g *mockGroup) AddInstance(id string)     { g.instances = append(g.instances, id) }
func (g *mockGroup) AddSubGroup(sub ts.GroupInterface) {
	mg := sub.(*mockGroup)
	g.subGroups[mg.id] = mg
}
func (g *mockGroup) GetOrCreateSubGroup(id, _ string) ts.GroupInterface {
	if sg, ok := g.subGroups[id]; ok {
		return sg
	}
	sg := newMockGroup(id)
	g.subGroups[id] = sg
	return sg
}
func (g *mockGroup) GetSubGroupByID(id string) ts.GroupInterface {
	if sg, ok := g.subGroups[id]; ok {
		return sg
	}
	return nil
}
func (g *mockGroup) MoveSubGroupUnder(_, _, _ string) bool { return false }
func (g *mockGroup) RemoveInstance(instanceID string) {
	filtered := make([]string, 0, len(g.instances))
	for _, id := range g.instances {
		if id != instanceID {
			filtered = append(filtered, id)
		}
	}
	g.instances = filtered
}

// writeCompletionFile writes a tripleshot completion sentinel file.
func writeCompletionFile(t *testing.T, dir string, status string) {
	t.Helper()
	completion := ts.CompletionFile{
		Status:        status,
		Summary:       "test summary",
		Approach:      "test approach",
		FilesModified: []string{"test.go"},
	}
	data, _ := json.Marshal(completion)
	if err := os.WriteFile(filepath.Join(dir, ts.CompletionFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeEvaluationFile writes a tripleshot evaluation sentinel file.
func writeEvaluationFile(t *testing.T, dir string, winner int) {
	t.Helper()
	evaluation := ts.Evaluation{
		WinnerIndex:   winner,
		MergeStrategy: ts.MergeStrategySelect,
		Reasoning:     "test reasoning",
	}
	data, _ := json.Marshal(evaluation)
	if err := os.WriteFile(filepath.Join(dir, ts.EvaluationFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- Constructor validation tests ---

func TestNewTeamCoordinator_Validation(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tests := []struct {
		name string
		cfg  TeamCoordinatorConfig
		want string
	}{
		{
			name: "missing orchestrator",
			cfg:  TeamCoordinatorConfig{Bus: bus, BaseSession: session, BaseDir: "/tmp", Task: "test"},
			want: "Orchestrator is required",
		},
		{
			name: "missing base session",
			cfg:  TeamCoordinatorConfig{Orchestrator: orch, Bus: bus, BaseDir: "/tmp", Task: "test"},
			want: "BaseSession is required",
		},
		{
			name: "missing bus",
			cfg:  TeamCoordinatorConfig{Orchestrator: orch, BaseSession: session, BaseDir: "/tmp", Task: "test"},
			want: "Bus is required",
		},
		{
			name: "missing base dir",
			cfg:  TeamCoordinatorConfig{Orchestrator: orch, BaseSession: session, Bus: bus, Task: "test"},
			want: "BaseDir is required",
		},
		{
			name: "missing task",
			cfg:  TeamCoordinatorConfig{Orchestrator: orch, BaseSession: session, Bus: bus, BaseDir: "/tmp"},
			want: "Task is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTeamCoordinator(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Errorf("error = %q, want containing %q", got, tt.want)
			}
		})
	}
}

func TestNewTeamCoordinator_Valid(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, err := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "implement feature",
	})
	if err != nil {
		t.Fatalf("NewTeamCoordinator: %v", err)
	}

	s := tc.Session()
	if s.Task != "implement feature" {
		t.Errorf("Session.Task = %q, want %q", s.Task, "implement feature")
	}
	if s.Phase != ts.PhaseWorking {
		t.Errorf("Session.Phase = %q, want %q", s.Phase, ts.PhaseWorking)
	}
}

// --- Start/Stop lifecycle tests ---

func TestTeamCoordinator_DoubleStart(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   session,
		Bus:           bus,
		BaseDir:       t.TempDir(),
		Task:          "test task",
		HubOptions:    []coordination.Option{coordination.WithRebalanceInterval(-1)},
		BridgeOptions: []bridge.Option{bridge.WithPollInterval(10 * time.Millisecond)},
	})

	ctx := context.Background()
	if err := tc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tc.Stop()

	err := tc.Start(ctx)
	if err == nil {
		t.Fatal("expected error on double start")
	}
}

func TestTeamCoordinator_StopIdempotent(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   session,
		Bus:           bus,
		BaseDir:       t.TempDir(),
		Task:          "test task",
		HubOptions:    []coordination.Option{coordination.WithRebalanceInterval(-1)},
		BridgeOptions: []bridge.Option{bridge.WithPollInterval(10 * time.Millisecond)},
	})

	ctx := context.Background()
	if err := tc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Stop multiple times — should not panic.
	tc.Stop()
	tc.Stop()
}

func TestTeamCoordinator_StopBeforeStart(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	// Should not panic.
	tc.Stop()
}

// --- Callback tests ---

func TestTeamCoordinator_SetCallbacks(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   session,
		Bus:           bus,
		BaseDir:       t.TempDir(),
		Task:          "test task",
		HubOptions:    []coordination.Option{coordination.WithRebalanceInterval(-1)},
		BridgeOptions: []bridge.Option{bridge.WithPollInterval(10 * time.Millisecond)},
	})

	phaseChanges := make(chan ts.Phase, 10)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange: func(phase ts.Phase) {
			phaseChanges <- phase
		},
	})

	ctx := context.Background()
	if err := tc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tc.Stop()

	// Should receive working phase change from Start.
	select {
	case phase := <-phaseChanges:
		if phase != ts.PhaseWorking {
			t.Errorf("expected PhaseWorking, got %v", phase)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for phase change")
	}
}

// --- Attempt lifecycle tests ---

func TestTeamCoordinator_AttemptStartCallbacks(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   session,
		Bus:           bus,
		BaseDir:       t.TempDir(),
		Task:          "implement feature",
		HubOptions:    []coordination.Option{coordination.WithRebalanceInterval(-1)},
		BridgeOptions: []bridge.Option{bridge.WithPollInterval(10 * time.Millisecond)},
	})

	starts := make(chan int, 3)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange:  func(_ ts.Phase) {},
		OnAttemptStart: func(idx int, _ string) { starts <- idx },
	})

	ctx := context.Background()
	if err := tc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tc.Stop()

	// Wait for all 3 attempts to start.
	started := make(map[int]bool)
	timeout := time.After(5 * time.Second)
	for len(started) < 3 {
		select {
		case idx := <-starts:
			started[idx] = true
		case <-timeout:
			t.Fatalf("timed out waiting for attempt starts, got %d", len(started))
		}
	}
}

// --- Full lifecycle test (attempts + judge) ---

func TestTeamCoordinator_FullLifecycle(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   session,
		Bus:           bus,
		BaseDir:       t.TempDir(),
		Task:          "implement feature",
		HubOptions:    []coordination.Option{coordination.WithRebalanceInterval(-1)},
		BridgeOptions: []bridge.Option{bridge.WithPollInterval(10 * time.Millisecond)},
	})

	completeCh := make(chan string, 1)
	phaseCh := make(chan ts.Phase, 10)
	evalCh := make(chan *ts.Evaluation, 1)
	attemptStarts := make(chan startInfo, 3)
	judgeStarts := make(chan string, 1)

	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange: func(p ts.Phase) { phaseCh <- p },
		OnAttemptStart: func(idx int, instanceID string) {
			attemptStarts <- startInfo{idx: idx, instanceID: instanceID}
		},
		OnAttemptComplete: func(_ int) {},
		OnAttemptFailed:   func(_ int, _ string) {},
		OnJudgeStart:      func(instanceID string) { judgeStarts <- instanceID },
		OnEvaluationReady: func(eval *ts.Evaluation) { evalCh <- eval },
		OnComplete: func(success bool, summary string) {
			if success {
				completeCh <- summary
			} else {
				completeCh <- "FAILED: " + summary
			}
		},
	})

	ctx := context.Background()
	if err := tc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tc.Stop()

	// Wait for all 3 attempt starts via callbacks (not bus events, since
	// events may fire before we can subscribe).
	instanceIDs := make([]string, 0, 3)
	timeout := time.After(5 * time.Second)
	for range 3 {
		select {
		case info := <-attemptStarts:
			instanceIDs = append(instanceIDs, info.instanceID)
		case <-timeout:
			t.Fatalf("timed out waiting for attempt starts, got %d", len(instanceIDs))
		}
	}

	// Write completion files for all 3 attempts.
	instances := orch.getInstances()
	for _, id := range instanceIDs {
		inst := instances[id]
		if inst == nil {
			t.Fatalf("instance %q not found in orch", id)
		}
		writeCompletionFile(t, inst.worktreePath, "complete")
	}

	// Wait for evaluating phase.
	waitForPhase(t, phaseCh, ts.PhaseEvaluating, 10*time.Second)

	// Wait for judge start via callback.
	var judgeInstanceID string
	select {
	case judgeInstanceID = <-judgeStarts:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for judge start")
	}

	judgeInstances := orch.getInstances()
	judgeInst := judgeInstances[judgeInstanceID]
	if judgeInst == nil {
		t.Fatal("judge instance not found")
	}

	writeEvaluationFile(t, judgeInst.worktreePath, 1)

	// Wait for completion.
	select {
	case summary := <-completeCh:
		if strings.Contains(summary, "FAILED") {
			t.Fatalf("expected success, got: %s", summary)
		}
		t.Logf("completed: %s", summary)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for completion")
	}

	// Verify evaluation was received.
	select {
	case eval := <-evalCh:
		if eval.WinnerIndex != 1 {
			t.Errorf("WinnerIndex = %d, want 1", eval.WinnerIndex)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for evaluation")
	}

	// Verify session state.
	s := tc.Session()
	if s.Phase != ts.PhaseComplete {
		t.Errorf("Session.Phase = %q, want %q", s.Phase, ts.PhaseComplete)
	}
	if s.Evaluation == nil {
		t.Error("Session.Evaluation is nil")
	}
	if s.CompletedAt == nil {
		t.Error("Session.CompletedAt is nil")
	}
}

// --- Tripleshot-specific event tests ---

func TestTeamCoordinator_TripleShotAttemptEvents(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator:  orch,
		BaseSession:   session,
		Bus:           bus,
		BaseDir:       t.TempDir(),
		Task:          "test task",
		HubOptions:    []coordination.Option{coordination.WithRebalanceInterval(-1)},
		BridgeOptions: []bridge.Option{bridge.WithPollInterval(10 * time.Millisecond)},
	})

	// Collect tripleshot attempt completed events — subscribe BEFORE Start
	// so we don't miss any events from the synchronous bus.
	attemptEvents := make(chan event.TripleShotAttemptCompletedEvent, 3)
	bus.Subscribe("tripleshot.attempt_completed", func(e event.Event) {
		if ace, ok := e.(event.TripleShotAttemptCompletedEvent); ok {
			attemptEvents <- ace
		}
	})

	// Use callbacks (set before Start) to learn when attempts start.
	attemptStartIDs := make(chan string, 3)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange:     func(_ ts.Phase) {},
		OnAttemptStart:    func(_ int, id string) { attemptStartIDs <- id },
		OnAttemptComplete: func(_ int) {},
	})

	ctx := context.Background()
	if err := tc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer tc.Stop()

	// Wait for attempts to start via callbacks, then write completion files.
	timeout := time.After(5 * time.Second)
	for range 3 {
		var instanceID string
		select {
		case instanceID = <-attemptStartIDs:
		case <-timeout:
			t.Fatal("timed out waiting for attempt starts")
		}
		instances := orch.getInstances()
		inst := instances[instanceID]
		writeCompletionFile(t, inst.worktreePath, "complete")
	}

	// Wait for all 3 tripleshot attempt events.
	received := 0
	timeout2 := time.After(10 * time.Second)
	for received < 3 {
		select {
		case ace := <-attemptEvents:
			if !ace.Success {
				t.Errorf("attempt %d not marked as success", ace.AttemptIndex)
			}
			received++
		case <-timeout2:
			t.Fatalf("timed out waiting for attempt events, got %d", received)
		}
	}
}

// --- GetWinningBranch tests ---

func TestTeamCoordinator_GetWinningBranch(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test",
	})

	// No evaluation yet.
	if b := tc.GetWinningBranch(); b != "" {
		t.Errorf("expected empty branch, got %q", b)
	}

	// Set up evaluation.
	s := tc.Session()
	s.Attempts[1].Branch = "winner-branch"
	s.Evaluation = &ts.Evaluation{
		WinnerIndex:   1,
		MergeStrategy: ts.MergeStrategySelect,
	}
	if b := tc.GetWinningBranch(); b != "winner-branch" {
		t.Errorf("expected %q, got %q", "winner-branch", b)
	}

	// Merge strategy should return empty.
	s.Evaluation.MergeStrategy = ts.MergeStrategyMerge
	if b := tc.GetWinningBranch(); b != "" {
		t.Errorf("expected empty for merge strategy, got %q", b)
	}

	// Out-of-range winner index.
	s.Evaluation.MergeStrategy = ts.MergeStrategySelect
	s.Evaluation.WinnerIndex = 5
	if b := tc.GetWinningBranch(); b != "" {
		t.Errorf("expected empty for invalid index, got %q", b)
	}

	// Negative winner index.
	s.Evaluation.WinnerIndex = -1
	if b := tc.GetWinningBranch(); b != "" {
		t.Errorf("expected empty for negative index, got %q", b)
	}
}

// --- Attempt failure tests ---

// --- Handler-level tests for failure paths ---

func TestTeamCoordinator_OnBridgeTaskCompleted_AttemptFailed(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	failedCh := make(chan string, 1)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnAttemptFailed: func(idx int, reason string) {
			failedCh <- fmt.Sprintf("%d:%s", idx, reason)
		},
	})

	// Manually set started=true and attemptTeamIDs so handlers work.
	tc.mu.Lock()
	tc.started = true
	tc.attemptTeamIDs = [3]string{"attempt-0", "attempt-1", "attempt-2"}
	tc.mu.Unlock()

	// Simulate bridge task_completed for attempt-1 with failure.
	bce := event.NewBridgeTaskCompletedEvent("attempt-1", "task-1", "inst-1", false, 0, "verification failed")
	tc.onBridgeTaskCompleted(bce)

	select {
	case result := <-failedCh:
		if !strings.Contains(result, "1:") || !strings.Contains(result, "verification failed") {
			t.Errorf("unexpected failure callback: %s", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for failure callback")
	}

	// Verify session state.
	s := tc.Session()
	if s.Attempts[1].Status != ts.AttemptStatusFailed {
		t.Errorf("Attempts[1].Status = %q, want %q", s.Attempts[1].Status, ts.AttemptStatusFailed)
	}
}

func TestTeamCoordinator_OnJudgeCompleted_Failure(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	doneCh := make(chan string, 1)
	phaseCh := make(chan ts.Phase, 5)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange: func(p ts.Phase) { phaseCh <- p },
		OnComplete:    func(success bool, summary string) { doneCh <- fmt.Sprintf("%v:%s", success, summary) },
	})

	// Set started=true and ensure event not treated as attempt.
	tc.mu.Lock()
	tc.started = true
	tc.mu.Unlock()

	// Collect tripleshot judge event.
	judgeEvents := make(chan event.TripleShotJudgeCompletedEvent, 1)
	bus.Subscribe("tripleshot.judge_completed", func(e event.Event) {
		if jce, ok := e.(event.TripleShotJudgeCompletedEvent); ok {
			judgeEvents <- jce
		}
	})

	// Simulate judge bridge failure (not an attempt team, falls through to onJudgeCompleted).
	bce := event.NewBridgeTaskCompletedEvent("judge", "judge-task", "judge-inst", false, 0, "judge crashed")
	tc.onBridgeTaskCompleted(bce)

	select {
	case result := <-doneCh:
		if !strings.Contains(result, "false") {
			t.Errorf("expected failure, got: %s", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for judge failure callback")
	}

	// Verify the TripleShotJudgeCompletedEvent was published.
	select {
	case jce := <-judgeEvents:
		if jce.Success {
			t.Error("expected Success=false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for judge completed event")
	}

	s := tc.Session()
	if s.Phase != ts.PhaseFailed {
		t.Errorf("Session.Phase = %q, want %q", s.Phase, ts.PhaseFailed)
	}
}

func TestTeamCoordinator_OnJudgeCompleted_InstanceNotFound(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	doneCh := make(chan string, 1)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange: func(_ ts.Phase) {},
		OnComplete:    func(success bool, _ string) { doneCh <- fmt.Sprintf("%v", success) },
	})

	tc.mu.Lock()
	tc.started = true
	tc.mu.Unlock()

	// Collect tripleshot judge event.
	judgeEvents := make(chan event.TripleShotJudgeCompletedEvent, 1)
	bus.Subscribe("tripleshot.judge_completed", func(e event.Event) {
		if jce, ok := e.(event.TripleShotJudgeCompletedEvent); ok {
			judgeEvents <- jce
		}
	})

	// Simulate successful judge completion, but instance not found in session.
	bce := event.NewBridgeTaskCompletedEvent("judge", "judge-task", "nonexistent-inst", true, 0, "")
	tc.onBridgeTaskCompleted(bce)

	select {
	case result := <-doneCh:
		if !strings.Contains(result, "false") {
			t.Errorf("expected failure, got: %s", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion callback")
	}

	// Verify the TripleShotJudgeCompletedEvent was published.
	select {
	case jce := <-judgeEvents:
		if jce.Success {
			t.Error("expected Success=false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for judge completed event")
	}

	s := tc.Session()
	if s.Phase != ts.PhaseFailed {
		t.Errorf("Session.Phase = %q, want %q", s.Phase, ts.PhaseFailed)
	}
}

func TestTeamCoordinator_OnJudgeCompleted_BadEvaluation(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	doneCh := make(chan string, 1)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange: func(_ ts.Phase) {},
		OnComplete:    func(success bool, summary string) { doneCh <- fmt.Sprintf("%v:%s", success, summary) },
	})

	tc.mu.Lock()
	tc.started = true
	tc.mu.Unlock()

	// Collect tripleshot judge event.
	judgeEvents := make(chan event.TripleShotJudgeCompletedEvent, 1)
	bus.Subscribe("tripleshot.judge_completed", func(e event.Event) {
		if jce, ok := e.(event.TripleShotJudgeCompletedEvent); ok {
			judgeEvents <- jce
		}
	})

	// Create a mock instance in session (so GetInstance works) but with
	// no evaluation file (so ParseEvaluationFile fails).
	instDir := t.TempDir()
	inst := &mockInstance{id: "judge-inst", worktreePath: instDir, branch: "judge-branch"}
	session.mu.Lock()
	session.instances["judge-inst"] = inst
	session.mu.Unlock()

	bce := event.NewBridgeTaskCompletedEvent("judge", "judge-task", "judge-inst", true, 0, "")
	tc.onBridgeTaskCompleted(bce)

	select {
	case result := <-doneCh:
		if !strings.Contains(result, "false") {
			t.Errorf("expected failure, got: %s", result)
		}
		if !strings.Contains(result, "parse evaluation") {
			t.Errorf("expected parse error in result, got: %s", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion callback")
	}

	// Verify the TripleShotJudgeCompletedEvent was published.
	select {
	case jce := <-judgeEvents:
		if jce.Success {
			t.Error("expected Success=false")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for judge completed event")
	}

	s := tc.Session()
	if s.Phase != ts.PhaseFailed {
		t.Errorf("Session.Phase = %q, want %q", s.Phase, ts.PhaseFailed)
	}
}

func TestTeamCoordinator_StartJudge_TooFewSuccesses(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	doneCh := make(chan string, 1)
	phaseCh := make(chan ts.Phase, 5)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnPhaseChange: func(p ts.Phase) { phaseCh <- p },
		OnComplete: func(success bool, summary string) {
			doneCh <- fmt.Sprintf("%v:%s", success, summary)
		},
	})

	tc.mu.Lock()
	tc.started = true
	tc.mu.Unlock()

	// Manually set session: only 1 out of 3 completed.
	s := tc.Session()
	s.Attempts[0].Status = ts.AttemptStatusCompleted
	s.Attempts[1].Status = ts.AttemptStatusFailed
	s.Attempts[2].Status = ts.AttemptStatusFailed

	// Call startJudge directly — it should detect <2 successes and fail.
	tc.startJudge()

	select {
	case result := <-doneCh:
		if !strings.Contains(result, "false") {
			t.Errorf("expected failure, got: %s", result)
		}
		if !strings.Contains(result, "fewer than 2") {
			t.Errorf("expected 'fewer than 2' in result, got: %s", result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for failure callback")
	}

	if s.Phase != ts.PhaseFailed {
		t.Errorf("Session.Phase = %q, want %q", s.Phase, ts.PhaseFailed)
	}
}

// TestTeamCoordinator_ReorganizeGroupForJudge verifies that startJudge()
// creates an "Implementers" sub-group, moves attempt instances into it,
// clears the parent group's direct instances, and sets ImplementersGroupID.
func TestTeamCoordinator_ReorganizeGroupForJudge(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	// Set up a mock group with 3 attempt instance IDs.
	group := newMockGroup("ts-group-1")
	group.AddInstance("inst-a")
	group.AddInstance("inst-b")
	group.AddInstance("inst-c")
	session.mu.Lock()
	session.groups["ts-group-1"] = group
	session.mu.Unlock()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	tc.mu.Lock()
	tc.started = true
	tc.attemptTeamIDs = [3]string{"attempt-0", "attempt-1", "attempt-2"}
	tc.mu.Unlock()

	// Set up session state: all 3 attempts completed, and link GroupID.
	s := tc.Session()
	s.GroupID = "ts-group-1"
	for i := range s.Attempts {
		s.Attempts[i].Status = ts.AttemptStatusCompleted
		s.Attempts[i].InstanceID = fmt.Sprintf("inst-%c", 'a'+i)
		dir := t.TempDir()
		s.Attempts[i].WorktreePath = dir
		s.Attempts[i].Branch = fmt.Sprintf("branch-%d", i)
		writeCompletionFile(t, dir, "complete")
	}

	// Call reorganizeGroupForJudge directly.
	tc.reorganizeGroupForJudge()

	// Verify ImplementersGroupID is set.
	if s.ImplementersGroupID == "" {
		t.Fatal("ImplementersGroupID is empty after reorganizeGroupForJudge")
	}

	// Verify the parent group's direct instances are cleared.
	if got := group.GetInstances(); len(got) != 0 {
		t.Errorf("parent group instances = %v, want empty", got)
	}

	// Verify the implementers sub-group exists and contains the 3 attempt IDs.
	implGroup := group.subGroups[s.ImplementersGroupID]
	if implGroup == nil {
		t.Fatalf("implementers sub-group %q not found in parent group", s.ImplementersGroupID)
	}
	gotInstances := implGroup.GetInstances()
	if len(gotInstances) != 3 {
		t.Fatalf("implementers sub-group instances = %v, want 3 entries", gotInstances)
	}
	for i, want := range []string{"inst-a", "inst-b", "inst-c"} {
		if gotInstances[i] != want {
			t.Errorf("implementers sub-group instances[%d] = %q, want %q", i, gotInstances[i], want)
		}
	}
}

// TestTeamCoordinator_ReorganizeGroupForJudge_NoGroup verifies graceful
// handling when the tripleshot group is not found in the session.
func TestTeamCoordinator_ReorganizeGroupForJudge_NoGroup(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	tc.mu.Lock()
	tc.started = true
	tc.mu.Unlock()

	s := tc.Session()
	s.GroupID = "nonexistent-group"

	// Should not panic; ImplementersGroupID should remain empty.
	tc.reorganizeGroupForJudge()

	if s.ImplementersGroupID != "" {
		t.Errorf("ImplementersGroupID = %q, want empty", s.ImplementersGroupID)
	}
}

func TestTeamCoordinator_OnTeamCompleted_NotStarted(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	// Not started — handler should return immediately without panicking.
	tce := event.NewTeamCompletedEvent("attempt-0", "Attempt 1", true, 1, 0)
	tc.onTeamCompleted(tce)

	// Verify no attempt was counted.
	tc.mu.Lock()
	count := tc.completedAttempts
	tc.mu.Unlock()
	if count != 0 {
		t.Errorf("completedAttempts = %d, want 0", count)
	}
}

func TestTeamCoordinator_OnTeamCompleted_UnknownTeam(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	tc.mu.Lock()
	tc.started = true
	tc.mu.Unlock()

	// Unknown team ID — should be ignored.
	tce := event.NewTeamCompletedEvent("unknown-team", "Unknown", true, 1, 0)
	tc.onTeamCompleted(tce)

	tc.mu.Lock()
	count := tc.completedAttempts
	tc.mu.Unlock()
	if count != 0 {
		t.Errorf("completedAttempts = %d, want 0", count)
	}
}

// TestTeamCoordinator_OnTeamCompleted_SetsAttemptStatus verifies that
// onTeamCompleted sets the attempt status eagerly. Without this, startJudge
// (dispatched as a goroutine from onTeamCompleted) races with
// onBridgeTaskCompleted and may snapshot the last attempt as "working".
func TestTeamCoordinator_OnTeamCompleted_SetsAttemptStatus(t *testing.T) {
	setup := func(t *testing.T) *TeamCoordinator {
		t.Helper()
		bus := event.NewBus()
		orch := newTestOrch(t)
		session := newTestSession()

		tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
			Orchestrator: orch,
			BaseSession:  session,
			Bus:          bus,
			BaseDir:      t.TempDir(),
			Task:         "test task",
		})

		tc.mu.Lock()
		tc.started = true
		tc.attemptTeamIDs = [3]string{"attempt-0", "attempt-1", "attempt-2"}
		tc.mu.Unlock()
		return tc
	}

	t.Run("success", func(t *testing.T) {
		tc := setup(t)

		tce := event.NewTeamCompletedEvent("attempt-1", "Attempt 2", true, 1, 0)
		tc.onTeamCompleted(tce)

		// The status should be set immediately by onTeamCompleted, without
		// needing onBridgeTaskCompleted to fire.
		s := tc.Session()
		if s.Attempts[1].Status != ts.AttemptStatusCompleted {
			t.Errorf("Attempts[1].Status = %q, want %q", s.Attempts[1].Status, ts.AttemptStatusCompleted)
		}
		if s.Attempts[1].CompletedAt == nil {
			t.Error("Attempts[1].CompletedAt is nil, want non-nil")
		}
		// Unrelated attempts should be unaffected.
		if s.Attempts[0].Status != "" {
			t.Errorf("Attempts[0].Status = %q, want empty", s.Attempts[0].Status)
		}
	})

	t.Run("failure", func(t *testing.T) {
		tc := setup(t)

		tce := event.NewTeamCompletedEvent("attempt-2", "Attempt 3", false, 0, 1)
		tc.onTeamCompleted(tce)

		s := tc.Session()
		if s.Attempts[2].Status != ts.AttemptStatusFailed {
			t.Errorf("Attempts[2].Status = %q, want %q", s.Attempts[2].Status, ts.AttemptStatusFailed)
		}
		if s.Attempts[2].CompletedAt == nil {
			t.Error("Attempts[2].CompletedAt is nil, want non-nil")
		}
	})
}

// TestTeamCoordinator_OnBridgeTaskCompleted_SkipsTerminalStatus verifies that
// onBridgeTaskCompleted does not overwrite status/CompletedAt when the attempt
// is already in a terminal state (set earlier by onTeamCompleted).
func TestTeamCoordinator_OnBridgeTaskCompleted_SkipsTerminalStatus(t *testing.T) {
	bus := event.NewBus()
	orch := newTestOrch(t)
	session := newTestSession()

	tc, _ := NewTeamCoordinator(TeamCoordinatorConfig{
		Orchestrator: orch,
		BaseSession:  session,
		Bus:          bus,
		BaseDir:      t.TempDir(),
		Task:         "test task",
	})

	completedCh := make(chan int, 1)
	tc.SetCallbacks(&ts.CoordinatorCallbacks{
		OnAttemptComplete: func(idx int) { completedCh <- idx },
	})

	tc.mu.Lock()
	tc.started = true
	tc.attemptTeamIDs = [3]string{"attempt-0", "attempt-1", "attempt-2"}
	tc.mu.Unlock()

	// Pre-set attempt-1 as completed (simulates onTeamCompleted having run first).
	s := tc.Session()
	earlyTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	s.Attempts[1].Status = ts.AttemptStatusCompleted
	s.Attempts[1].CompletedAt = &earlyTime

	// Fire onBridgeTaskCompleted — should NOT overwrite the earlier timestamp.
	bce := event.NewBridgeTaskCompletedEvent("attempt-1", "task-1", "inst-1", true, 1, "")
	tc.onBridgeTaskCompleted(bce)

	// Callback should still fire.
	select {
	case idx := <-completedCh:
		if idx != 1 {
			t.Errorf("callback index = %d, want 1", idx)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for callback")
	}

	// CompletedAt should retain the earlier timestamp.
	if !s.Attempts[1].CompletedAt.Equal(earlyTime) {
		t.Errorf("CompletedAt was overwritten: got %v, want %v", s.Attempts[1].CompletedAt, earlyTime)
	}
}

// startInfo captures an attempt start event for test assertions.
type startInfo struct {
	idx        int
	instanceID string
}

// --- Helpers ---

func waitForPhase(t *testing.T, ch <-chan ts.Phase, want ts.Phase, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case got := <-ch:
			if got == want {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for phase %q", want)
		}
	}
}

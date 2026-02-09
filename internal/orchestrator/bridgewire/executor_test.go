package bridgewire

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/pipeline"
	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// --- Mock types for E2E tests ------------------------------------------------

// nopFactory is a minimal InstanceFactory used in validation/lifecycle tests
// where bridges are never actually created.
type nopFactory struct{}

func (nopFactory) CreateInstance(string) (bridge.Instance, error) { return nil, nil }
func (nopFactory) StartInstance(bridge.Instance) error            { return nil }

// nopChecker is a minimal CompletionChecker used in validation/lifecycle tests.
type nopChecker struct{}

func (nopChecker) CheckCompletion(string) (bool, error)            { return false, nil }
func (nopChecker) VerifyWork(_, _, _, _ string) (bool, int, error) { return false, 0, nil }

// autoCompleteFactory creates mock instances and immediately triggers
// completion on StartInstance. This simulates instant-completion Claude Code
// instances for E2E testing.
type autoCompleteFactory struct {
	mu      sync.Mutex
	checker *autoCompleteChecker
	counter int
	created []string // prompts received
}

func newAutoCompleteFactory(checker *autoCompleteChecker) *autoCompleteFactory {
	return &autoCompleteFactory{checker: checker}
}

func (f *autoCompleteFactory) CreateInstance(prompt string) (bridge.Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.counter++
	id := fmt.Sprintf("inst-%d", f.counter)
	f.created = append(f.created, prompt)

	return &mockInst{
		id:           id,
		worktreePath: fmt.Sprintf("/tmp/wt-%d", f.counter),
		branch:       fmt.Sprintf("branch-%d", f.counter),
	}, nil
}

func (f *autoCompleteFactory) StartInstance(inst bridge.Instance) error {
	// Trigger immediate completion so the bridge's monitor detects it.
	f.checker.MarkComplete(inst.WorktreePath())
	return nil
}

func (f *autoCompleteFactory) Created() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.created))
	copy(out, f.created)
	return out
}

// failingFactory always returns an error on CreateInstance.
type failingFactory struct {
	err error
}

func (f *failingFactory) CreateInstance(string) (bridge.Instance, error) {
	return nil, f.err
}

func (f *failingFactory) StartInstance(bridge.Instance) error { return nil }

// autoCompleteChecker tracks which worktree paths are "complete" and always
// verifies work successfully.
type autoCompleteChecker struct {
	mu          sync.Mutex
	completions map[string]bool
	verifyOK    bool
	commitCount int
}

func newAutoCompleteChecker() *autoCompleteChecker {
	return &autoCompleteChecker{
		completions: make(map[string]bool),
		verifyOK:    true,
		commitCount: 1,
	}
}

func (c *autoCompleteChecker) MarkComplete(worktreePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completions[worktreePath] = true
}

func (c *autoCompleteChecker) CheckCompletion(worktreePath string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.completions[worktreePath], nil
}

func (c *autoCompleteChecker) VerifyWork(_, _, _, _ string) (bool, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.verifyOK, c.commitCount, nil
}

// mockInst implements bridge.Instance.
type mockInst struct {
	id           string
	worktreePath string
	branch       string
}

func (m *mockInst) ID() string           { return m.id }
func (m *mockInst) WorktreePath() string { return m.worktreePath }
func (m *mockInst) Branch() string       { return m.branch }

// trackingRecorder records AssignTask/RecordCompletion/RecordFailure calls
// with channels for synchronization in E2E tests.
type trackingRecorder struct {
	mu        sync.Mutex
	assigned  map[string]string // taskID → instanceID
	completed map[string]int    // taskID → commitCount
	failed    map[string]string // taskID → reason

	assignCh   chan string // taskID sent on AssignTask
	completeCh chan string // taskID sent on RecordCompletion
	failCh     chan string // taskID sent on RecordFailure
}

func newTrackingRecorder() *trackingRecorder {
	return &trackingRecorder{
		assigned:   make(map[string]string),
		completed:  make(map[string]int),
		failed:     make(map[string]string),
		assignCh:   make(chan string, 20),
		completeCh: make(chan string, 20),
		failCh:     make(chan string, 20),
	}
}

func (r *trackingRecorder) AssignTask(taskID, instanceID string) {
	r.mu.Lock()
	r.assigned[taskID] = instanceID
	r.mu.Unlock()
	select {
	case r.assignCh <- taskID:
	default:
	}
}

func (r *trackingRecorder) RecordCompletion(taskID string, commitCount int) {
	r.mu.Lock()
	r.completed[taskID] = commitCount
	r.mu.Unlock()
	select {
	case r.completeCh <- taskID:
	default:
	}
}

func (r *trackingRecorder) RecordFailure(taskID, reason string) {
	r.mu.Lock()
	r.failed[taskID] = reason
	r.mu.Unlock()
	select {
	case r.failCh <- taskID:
	default:
	}
}

func (r *trackingRecorder) Assigned() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.assigned))
	for k, v := range r.assigned {
		out[k] = v
	}
	return out
}

func (r *trackingRecorder) Completed() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.completed))
	for k, v := range r.completed {
		out[k] = v
	}
	return out
}

func (r *trackingRecorder) Failed() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.failed))
	for k, v := range r.failed {
		out[k] = v
	}
	return out
}

// --- E2E test helper ---------------------------------------------------------

// waitForBusEvent subscribes to the bus and waits for the given event type,
// failing the test on timeout.
func waitForBusEvent(t *testing.T, bus *event.Bus, eventType string, timeout time.Duration) event.Event {
	t.Helper()
	ch := make(chan event.Event, 1)
	subID := bus.Subscribe(eventType, func(e event.Event) {
		select {
		case ch <- e:
		default:
		}
	})
	defer bus.Unsubscribe(subID)

	select {
	case e := <-ch:
		return e
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %q event", eventType)
		return nil
	}
}

// --- Validation tests -------------------------------------------------------

func TestNewPipelineExecutor_Validation(t *testing.T) {
	bus := event.NewBus()

	tests := []struct {
		name    string
		cfg     PipelineExecutorConfig
		wantErr string
	}{
		{
			name:    "missing factory",
			cfg:     PipelineExecutorConfig{},
			wantErr: "Factory is required",
		},
		{
			name: "missing checker",
			cfg: PipelineExecutorConfig{
				Factory: nopFactory{},
			},
			wantErr: "Checker is required",
		},
		{
			name: "missing bus",
			cfg: PipelineExecutorConfig{
				Factory: nopFactory{},
				Checker: nopChecker{},
			},
			wantErr: "Bus is required",
		},
		{
			name: "missing pipeline",
			cfg: PipelineExecutorConfig{
				Factory: nopFactory{},
				Checker: nopChecker{},
				Bus:     bus,
			},
			wantErr: "Pipeline is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPipelineExecutor(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPipelineExecutor_DoubleStart(t *testing.T) {
	bus := event.NewBus()
	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Factory:  nopFactory{},
		Checker:  nopChecker{},
		Bus:      bus,
		Pipeline: &pipeline.Pipeline{},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}

	ctx := t.Context()
	if err := pe.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer pe.Stop()

	if err := pe.Start(ctx); err == nil {
		t.Error("second Start should return error")
	}
}

func TestPipelineExecutor_StopBeforeStart(t *testing.T) {
	bus := event.NewBus()
	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Factory:  nopFactory{},
		Checker:  nopChecker{},
		Bus:      bus,
		Pipeline: &pipeline.Pipeline{},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}
	pe.Stop() // should not panic
}

func TestPipelineExecutor_BridgesEmpty(t *testing.T) {
	bus := event.NewBus()
	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Factory:  nopFactory{},
		Checker:  nopChecker{},
		Bus:      bus,
		Pipeline: &pipeline.Pipeline{},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}

	bridges := pe.Bridges()
	if len(bridges) != 0 {
		t.Errorf("Bridges() = %d, want 0 before start", len(bridges))
	}
}

// --- E2E integration tests ---------------------------------------------------

// newE2EPipeline creates a Pipeline + PipelineExecutor wired with auto-complete
// mocks and fast polling. Returns everything needed for an E2E test, including
// the DecomposeResult so callers can modify team specs (e.g., DependsOn)
// before Start.
func newE2EPipeline(
	t *testing.T,
	plan *ultraplan.PlanSpec,
	dcfg pipeline.DecomposeConfig,
	factory bridge.InstanceFactory,
	checker bridge.CompletionChecker,
	recorder bridge.SessionRecorder,
) (*pipeline.Pipeline, *PipelineExecutor, *event.Bus, *pipeline.DecomposeResult) {
	t.Helper()

	bus := event.NewBus()
	pipe, err := pipeline.NewPipeline(pipeline.PipelineConfig{
		Bus:     bus,
		BaseDir: t.TempDir(),
		Plan:    plan,
	}, pipeline.WithHubOptions(coordination.WithRebalanceInterval(-1)))
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}

	result, err := pipe.Decompose(dcfg)
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	pe, err := NewPipelineExecutor(PipelineExecutorConfig{
		Factory:  factory,
		Checker:  checker,
		Bus:      bus,
		Pipeline: pipe,
		Recorder: recorder,
		BridgeOpts: []bridge.Option{
			bridge.WithPollInterval(10 * time.Millisecond),
		},
	})
	if err != nil {
		t.Fatalf("NewPipelineExecutor: %v", err)
	}

	return pipe, pe, bus, result
}

func TestPipelineExecutor_E2E_SingleTeam(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-plan",
		Objective: "E2E single team test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Do thing 1", Files: []string{"a.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := newAutoCompleteFactory(checker)
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, factory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Subscribe to events before starting so we don't miss them.
	bridgeStarted := make(chan event.Event, 5)
	bus.Subscribe("bridge.task_started", func(e event.Event) {
		bridgeStarted <- e
	})
	bridgeCompleted := make(chan event.Event, 5)
	bus.Subscribe("bridge.task_completed", func(e event.Event) {
		bridgeCompleted <- e
	})
	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Wait for bridge to start the task.
	select {
	case e := <-bridgeStarted:
		started := e.(event.BridgeTaskStartedEvent)
		if started.TaskID != "t1" {
			t.Errorf("started.TaskID = %q, want %q", started.TaskID, "t1")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for bridge.task_started")
	}

	// Wait for bridge to complete the task (auto-complete fires on StartInstance).
	select {
	case e := <-bridgeCompleted:
		completed := e.(event.BridgeTaskCompletedEvent)
		if completed.TaskID != "t1" {
			t.Errorf("completed.TaskID = %q, want %q", completed.TaskID, "t1")
		}
		if !completed.Success {
			t.Errorf("completed.Success = false, want true")
		}
		if completed.CommitCount != 1 {
			t.Errorf("completed.CommitCount = %d, want 1", completed.CommitCount)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for bridge.task_completed")
	}

	// Wait for pipeline to complete.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pipeline.completed")
	}

	// Verify recorder state.
	assigned := recorder.Assigned()
	if _, ok := assigned["t1"]; !ok {
		t.Error("recorder.AssignTask not called for t1")
	}
	completed := recorder.Completed()
	if count, ok := completed["t1"]; !ok || count != 1 {
		t.Errorf("recorder.Completed[t1] = %d, want 1", count)
	}

	// Verify exactly 1 bridge was created.
	bridges := pe.Bridges()
	if len(bridges) != 1 {
		t.Errorf("pe.Bridges() = %d, want 1", len(bridges))
	}
}

func TestPipelineExecutor_E2E_MultiTeam(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-multi",
		Objective: "E2E multi-team test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "First", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Description: "Second", Files: []string{"b.go"}},
			{ID: "t3", Title: "Task 3", Description: "Third", Files: []string{"c.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := newAutoCompleteFactory(checker)
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, factory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Wait for pipeline to complete — all 3 tasks auto-complete.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for pipeline.completed")
	}

	// Verify 3 bridges were created (3 disjoint-file tasks = 3 teams).
	bridges := pe.Bridges()
	if len(bridges) != 3 {
		t.Errorf("pe.Bridges() = %d, want 3", len(bridges))
	}

	// Verify all 3 tasks were assigned and completed.
	assigned := recorder.Assigned()
	for _, id := range []string{"t1", "t2", "t3"} {
		if _, ok := assigned[id]; !ok {
			t.Errorf("recorder.AssignTask not called for %s", id)
		}
	}
	completed := recorder.Completed()
	for _, id := range []string{"t1", "t2", "t3"} {
		if _, ok := completed[id]; !ok {
			t.Errorf("recorder.RecordCompletion not called for %s", id)
		}
	}

	// Verify factory created 3 instances.
	created := factory.Created()
	if len(created) != 3 {
		t.Errorf("factory.Created() = %d, want 3", len(created))
	}
}

func TestPipelineExecutor_E2E_FailurePropagation(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-fail",
		Objective: "E2E failure test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Will fail", Files: []string{"a.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := &failingFactory{err: fmt.Errorf("out of resources")}
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, factory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Pipeline should fail because all CreateInstance calls fail, eventually
	// exhausting retries and marking the task as failed.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if pce.Success {
			t.Error("pipeline should have failed")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for pipeline.completed")
	}

	if pipe.Phase() != pipeline.PhaseFailed {
		t.Errorf("Phase = %v, want %v", pipe.Phase(), pipeline.PhaseFailed)
	}
}

func TestPipelineExecutor_E2E_AllPhases(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-allphases",
		Objective: "E2E all-phases test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Main task", Files: []string{"a.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := newAutoCompleteFactory(checker)
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{
		PlanningTeam:      true,
		ReviewTeam:        true,
		ConsolidationTeam: true,
	}, factory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	phaseChanges := make(chan event.Event, 30)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})
	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Planning phase — no bridges; complete tasks manually.
	waitForPhaseEvent(t, phaseChanges, "planning", 3*time.Second)
	completeAllTeamTasks(t, pipe, pipeline.PhasePlanning)

	// Execution phase — bridges auto-complete via factory.
	waitForPhaseEvent(t, phaseChanges, "execution", 3*time.Second)
	// Bridges auto-complete, no manual intervention needed.

	// Review phase — no bridges; complete tasks manually.
	waitForPhaseEvent(t, phaseChanges, "review", 5*time.Second)
	completeAllTeamTasks(t, pipe, pipeline.PhaseReview)

	// Consolidation phase — no bridges; complete tasks manually.
	waitForPhaseEvent(t, phaseChanges, "consolidation", 3*time.Second)
	completeAllTeamTasks(t, pipe, pipeline.PhaseConsolidation)

	// Pipeline should complete successfully.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
		if pce.PhasesRun != 4 {
			t.Errorf("PhasesRun = %d, want 4", pce.PhasesRun)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for pipeline.completed")
	}

	// Verify the execution task was recorded.
	completed := recorder.Completed()
	if _, ok := completed["t1"]; !ok {
		t.Error("recorder.RecordCompletion not called for execution task t1")
	}
}

func TestPipelineExecutor_E2E_StopCleanup(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-stop",
		Objective: "E2E stop cleanup test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Task", Files: []string{"a.go"}},
		},
	}

	// Use a checker that never completes, so the bridge stays running.
	checker := newAutoCompleteChecker()
	var createCount atomic.Int32
	slowFactory := &slowCompleteFactory{
		createCount: &createCount,
	}
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, slowFactory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}

	// Wait for bridge to start a task (instance created, monitor running).
	waitForBusEvent(t, bus, "bridge.task_started", 5*time.Second)

	// Verify a bridge exists.
	bridges := pe.Bridges()
	if len(bridges) != 1 {
		t.Fatalf("pe.Bridges() = %d, want 1", len(bridges))
	}

	// Stop the executor mid-flight.
	done := make(chan struct{})
	go func() {
		pe.Stop()
		close(done)
	}()

	select {
	case <-done:
		// good — stop returned
	case <-time.After(5 * time.Second):
		t.Fatal("PipelineExecutor.Stop() did not return within timeout")
	}

	// After stop, bridges should be cleared.
	bridges = pe.Bridges()
	if len(bridges) != 0 {
		t.Errorf("pe.Bridges() = %d after Stop, want 0", len(bridges))
	}
}

// slowCompleteFactory creates instances but does NOT auto-complete them.
// Used in the StopCleanup test to keep the bridge's monitor running.
type slowCompleteFactory struct {
	createCount *atomic.Int32
}

func (f *slowCompleteFactory) CreateInstance(prompt string) (bridge.Instance, error) {
	n := f.createCount.Add(1)
	return &mockInst{
		id:           fmt.Sprintf("inst-%d", n),
		worktreePath: fmt.Sprintf("/tmp/wt-%d", n),
		branch:       fmt.Sprintf("branch-%d", n),
	}, nil
}

func (f *slowCompleteFactory) StartInstance(bridge.Instance) error {
	// Deliberately do NOT mark complete — the test stops the executor mid-flight.
	return nil
}

// --- Shared helpers ----------------------------------------------------------

// waitForPhaseEvent waits for a pipeline phase change event with the given phase.
func waitForPhaseEvent(t *testing.T, ch <-chan event.Event, phase string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case e := <-ch:
			pce := e.(event.PipelinePhaseChangedEvent)
			if pce.CurrentPhase == phase {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for pipeline phase %q", phase)
		}
	}
}

// selectiveFactory delegates to autoCompleteFactory for most instances, but
// returns an error for tasks matching any key in failFor.
type selectiveFactory struct {
	mu      sync.Mutex
	checker *autoCompleteChecker
	counter int
	failFor map[string]bool // prompt substrings that trigger failure
	failErr error
}

func (f *selectiveFactory) CreateInstance(prompt string) (bridge.Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for substr := range f.failFor {
		if strings.Contains(prompt, substr) {
			return nil, f.failErr
		}
	}

	f.counter++
	id := fmt.Sprintf("inst-%d", f.counter)
	return &mockInst{
		id:           id,
		worktreePath: fmt.Sprintf("/tmp/wt-%d", f.counter),
		branch:       fmt.Sprintf("branch-%d", f.counter),
	}, nil
}

func (f *selectiveFactory) StartInstance(inst bridge.Instance) error {
	f.checker.MarkComplete(inst.WorktreePath())
	return nil
}

// completeAllTeamTasks completes all tasks in all teams for the given phase.
// Adapted from pipeline_test.go.
func completeAllTeamTasks(t *testing.T, p *pipeline.Pipeline, phase pipeline.PipelinePhase) {
	t.Helper()

	deadline := time.After(5 * time.Second)
	for {
		m := p.Manager(phase)
		if m == nil {
			select {
			case <-deadline:
				t.Fatalf("timed out waiting for manager in phase %s", phase)
			default:
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		for _, s := range m.AllStatuses() {
			tm := m.Team(s.ID)
			if tm == nil {
				continue
			}
			eq := tm.Hub().EventQueue()
			for {
				task, err := eq.ClaimNext("test-instance")
				if err != nil || task == nil {
					break
				}
				if err := eq.MarkRunning(task.ID); err != nil {
					t.Fatalf("MarkRunning(%s): %v", task.ID, err)
				}
				if _, err := eq.Complete(task.ID); err != nil {
					t.Fatalf("Complete(%s): %v", task.ID, err)
				}
			}
		}

		allDone := true
		for _, s := range m.AllStatuses() {
			if !s.Phase.IsTerminal() {
				allDone = false
				break
			}
		}
		if allDone {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("timed out completing tasks in phase %s", phase)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// --- E2E: Dependency Ordering ------------------------------------------------

func TestPipelineExecutor_E2E_DependencyOrdering(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-deps",
		Objective: "E2E dependency ordering test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "First", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Description: "Second", Files: []string{"b.go"}},
			{ID: "t3", Title: "Task 3", Description: "Third", Files: []string{"c.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := newAutoCompleteFactory(checker)
	recorder := newTrackingRecorder()

	pipe, pe, bus, result := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, factory, checker, recorder)

	// Set up a linear dependency chain: exec-0 → exec-1 → exec-2
	result.ExecutionTeams[1].DependsOn = []string{"exec-0"}
	result.ExecutionTeams[2].DependsOn = []string{"exec-1"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Collect the order in which teams start working.
	var workingMu sync.Mutex
	var workingOrder []string
	bus.Subscribe("team.phase_changed", func(e event.Event) {
		pce := e.(event.TeamPhaseChangedEvent)
		if pce.CurrentPhase == string(team.PhaseWorking) {
			workingMu.Lock()
			workingOrder = append(workingOrder, pce.TeamID)
			workingMu.Unlock()
		}
	})

	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Pipeline should complete — auto-complete triggers cascade.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for pipeline.completed; phase=%s", pipe.Phase())
	}

	// Verify ordering: exec-0 must start before exec-1, exec-1 before exec-2.
	workingMu.Lock()
	order := make([]string, len(workingOrder))
	copy(order, workingOrder)
	workingMu.Unlock()

	indexOf := func(id string) int {
		for i, v := range order {
			if v == id {
				return i
			}
		}
		return -1
	}

	i0 := indexOf("exec-0")
	i1 := indexOf("exec-1")
	i2 := indexOf("exec-2")

	if i0 < 0 || i1 < 0 || i2 < 0 {
		t.Fatalf("not all teams reached working: order = %v", order)
	}
	if i0 >= i1 {
		t.Errorf("exec-0 (index %d) should start before exec-1 (index %d)", i0, i1)
	}
	if i1 >= i2 {
		t.Errorf("exec-1 (index %d) should start before exec-2 (index %d)", i1, i2)
	}

	// Verify all tasks completed.
	completed := recorder.Completed()
	for _, id := range []string{"t1", "t2", "t3"} {
		if _, ok := completed[id]; !ok {
			t.Errorf("recorder.RecordCompletion not called for %s", id)
		}
	}
}

// --- E2E: Dependency Failure Cascade -----------------------------------------

func TestPipelineExecutor_E2E_DependencyFailureCascade(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-depcascade",
		Objective: "E2E dependency failure cascade test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Will fail", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Description: "Depends on t1", Files: []string{"b.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := &failingFactory{err: fmt.Errorf("instance creation failed")}
	recorder := newTrackingRecorder()

	pipe, pe, bus, result := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, factory, checker, recorder)

	// exec-1 depends on exec-0; exec-0 will fail because all instance creation fails.
	result.ExecutionTeams[1].DependsOn = []string{"exec-0"}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Pipeline should fail: exec-0 fails → exec-1 cascades to PhaseFailed.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if pce.Success {
			t.Error("pipeline should have failed")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for pipeline.completed; phase=%s", pipe.Phase())
	}

	if pipe.Phase() != pipeline.PhaseFailed {
		t.Errorf("Phase = %v, want %v", pipe.Phase(), pipeline.PhaseFailed)
	}
}

// --- E2E: Partial Failure (some teams fail, others succeed) ------------------

func TestPipelineExecutor_E2E_PartialFailure(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-partial",
		Objective: "E2E partial failure test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Will succeed", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Description: "Will fail", Files: []string{"b.go"}},
			{ID: "t3", Title: "Task 3", Description: "Will succeed", Files: []string{"c.go"}},
		},
	}

	checker := newAutoCompleteChecker()
	factory := &selectiveFactory{
		checker: checker,
		failFor: map[string]bool{"Task 2": true},
		failErr: fmt.Errorf("selective failure"),
	}
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, factory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Pipeline should fail because one team (t2) fails.
	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if pce.Success {
			t.Error("pipeline should have failed due to partial failure")
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for pipeline.completed; phase=%s", pipe.Phase())
	}

	if pipe.Phase() != pipeline.PhaseFailed {
		t.Errorf("Phase = %v, want %v", pipe.Phase(), pipeline.PhaseFailed)
	}

	// Verify t1 and t3 succeeded (completed via recorder).
	// Note: t2 fails at CreateInstance, so recorder.RecordFailure is NOT called
	// (the bridge only records via the recorder after instance creation succeeds).
	// The failure is instead reflected through gate.Fail() → task queue → team fails.
	completed := recorder.Completed()
	if _, ok := completed["t1"]; !ok {
		t.Error("recorder.RecordCompletion not called for t1")
	}
	if _, ok := completed["t3"]; !ok {
		t.Error("recorder.RecordCompletion not called for t3")
	}
}

// --- E2E: Budget Exhaustion Event -------------------------------------------

func TestPipelineExecutor_E2E_BudgetExhaustedEvent(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-budget",
		Objective: "E2E budget exhaustion test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Budget test", Files: []string{"a.go"}},
		},
	}

	// Use slowCompleteFactory so the task stays in-flight while we record
	// budget usage. The auto-complete factory would finish the task before
	// we can call BudgetTracker.Record().
	checker := newAutoCompleteChecker()
	var createCount atomic.Int32
	slowFactory := &slowCompleteFactory{createCount: &createCount}
	recorder := newTrackingRecorder()

	pipe, pe, bus, result := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, slowFactory, checker, recorder)

	// Set a budget limit on the execution team.
	result.ExecutionTeams[0].Budget = team.TokenBudget{MaxTotalCost: 100.0}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	budgetExhausted := make(chan event.Event, 5)
	bus.Subscribe("team.budget_exhausted", func(e event.Event) {
		budgetExhausted <- e
	})

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}
	defer pe.Stop()

	// Wait for the bridge to start a task — this means the team is working
	// and the budget tracker is active.
	waitForBusEvent(t, bus, "bridge.task_started", 5*time.Second)

	// Get the execution phase manager.
	mgr := pipe.Manager(pipeline.PhaseExecution)
	if mgr == nil {
		t.Fatal("execution phase manager is nil after bridge started")
	}

	// Record usage that exceeds the budget.
	tm := mgr.Team("exec-0")
	if tm == nil {
		t.Fatal("Team(exec-0) = nil")
	}
	tm.BudgetTracker().Record(0, 0, 150.0)

	// Verify budget exhaustion event fires.
	select {
	case e := <-budgetExhausted:
		be := e.(event.TeamBudgetExhaustedEvent)
		if be.TeamID != "exec-0" {
			t.Errorf("TeamID = %q, want %q", be.TeamID, "exec-0")
		}
		if be.MaxTotalCost != 100.0 {
			t.Errorf("MaxTotalCost = %f, want 100.0", be.MaxTotalCost)
		}
		if be.UsedCost != 150.0 {
			t.Errorf("UsedCost = %f, want 150.0", be.UsedCost)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for team.budget_exhausted event")
	}

	// Mark the instance as complete so the pipeline can finish.
	checker.MarkComplete(fmt.Sprintf("/tmp/wt-%d", createCount.Load()))

	// Pipeline should still complete successfully (budget exhaustion is advisory).
	pipelineCompleted := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		pipelineCompleted <- e
	})

	select {
	case e := <-pipelineCompleted:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should succeed even with budget exhaustion")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for pipeline.completed")
	}
}

// --- E2E: Context Cancellation -----------------------------------------------

func TestPipelineExecutor_E2E_ContextCancel(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "e2e-cancel",
		Objective: "E2E context cancellation test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Description: "Task", Files: []string{"a.go"}},
		},
	}

	// Use a checker that never completes so the bridge stays running.
	checker := newAutoCompleteChecker()
	var createCount atomic.Int32
	slowFactory := &slowCompleteFactory{
		createCount: &createCount,
	}
	recorder := newTrackingRecorder()

	pipe, pe, bus, _ := newE2EPipeline(t, plan, pipeline.DecomposeConfig{}, slowFactory, checker, recorder)

	ctx, cancel := context.WithCancel(context.Background())

	if err := pipe.Start(ctx); err != nil {
		t.Fatalf("Pipeline.Start: %v", err)
	}
	defer func() { _ = pipe.Stop() }()

	if err := pe.Start(ctx); err != nil {
		t.Fatalf("PipelineExecutor.Start: %v", err)
	}

	// Wait for bridge to start a task (instance created, monitor running).
	waitForBusEvent(t, bus, "bridge.task_started", 5*time.Second)

	// Cancel the context to trigger shutdown.
	cancel()

	// Stop should be safe and idempotent after cancel.
	done := make(chan struct{})
	go func() {
		pe.Stop()
		close(done)
	}()

	select {
	case <-done:
		// good — stop returned cleanly after context cancel
	case <-time.After(5 * time.Second):
		t.Fatal("PipelineExecutor.Stop() did not return within timeout after context cancel")
	}

	// Calling Stop again should be idempotent.
	pe.Stop()
}

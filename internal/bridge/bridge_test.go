package bridge_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// --- Mock implementations ------------------------------------------------

type mockInstance struct {
	id           string
	worktreePath string
	branch       string
}

func (m *mockInstance) ID() string           { return m.id }
func (m *mockInstance) WorktreePath() string { return m.worktreePath }
func (m *mockInstance) Branch() string       { return m.branch }

type mockFactory struct {
	mu        sync.Mutex
	created   []string // prompts
	instances map[string]*mockInstance
	createErr error
	startErr  error
}

func newMockFactory() *mockFactory {
	return &mockFactory{
		instances: make(map[string]*mockInstance),
	}
}

func (f *mockFactory) CreateInstance(prompt string) (bridge.Instance, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.createErr != nil {
		return nil, f.createErr
	}

	id := prompt[:min(8, len(prompt))]
	inst := &mockInstance{
		id:           "inst-" + id,
		worktreePath: "/tmp/wt-" + id,
		branch:       "branch-" + id,
	}
	f.created = append(f.created, prompt)
	f.instances[inst.id] = inst
	return inst, nil
}

func (f *mockFactory) StartInstance(inst bridge.Instance) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startErr
}

func (f *mockFactory) Created() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.created))
	copy(out, f.created)
	return out
}

type mockChecker struct {
	mu          sync.Mutex
	completions map[string]bool // worktreePath → done
	verifyOK    bool
	commitCount int
	verifyErr   error
}

func newMockChecker() *mockChecker {
	return &mockChecker{
		completions: make(map[string]bool),
		verifyOK:    true,
		commitCount: 1,
	}
}

func (c *mockChecker) CheckCompletion(worktreePath string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.completions[worktreePath], nil
}

func (c *mockChecker) VerifyWork(_, _, _, _ string) (bool, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.verifyOK, c.commitCount, c.verifyErr
}

func (c *mockChecker) MarkComplete(worktreePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completions[worktreePath] = true
}

type mockRecorder struct {
	mu        sync.Mutex
	assigned  map[string]string // taskID → instanceID
	completed map[string]int    // taskID → commitCount
	failed    map[string]string // taskID → reason
}

func newMockRecorder() *mockRecorder {
	return &mockRecorder{
		assigned:  make(map[string]string),
		completed: make(map[string]int),
		failed:    make(map[string]string),
	}
}

func (r *mockRecorder) AssignTask(taskID, instanceID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assigned[taskID] = instanceID
}

func (r *mockRecorder) RecordCompletion(taskID string, commitCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.completed[taskID] = commitCount
}

func (r *mockRecorder) RecordFailure(taskID, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed[taskID] = reason
}

func (r *mockRecorder) Assigned() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.assigned))
	for k, v := range r.assigned {
		out[k] = v
	}
	return out
}

func (r *mockRecorder) Completed() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]int, len(r.completed))
	for k, v := range r.completed {
		out[k] = v
	}
	return out
}

func (r *mockRecorder) Failed() map[string]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(r.failed))
	for k, v := range r.failed {
		out[k] = v
	}
	return out
}

// --- Helpers -------------------------------------------------------------

// newTestTeam creates a team.Manager with one team, starts it, and returns the Team pointer.
func newTestTeam(t *testing.T, bus *event.Bus, tasks []ultraplan.PlannedTask) *team.Team {
	t.Helper()

	mgr, err := team.NewManager(team.ManagerConfig{
		Bus:     bus,
		BaseDir: t.TempDir(),
	}, team.WithHubOptions(coordination.WithRebalanceInterval(-1)))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	spec := team.Spec{
		ID:       "test-team",
		Name:     "Test Team",
		Role:     team.RoleExecution,
		Tasks:    tasks,
		TeamSize: 1,
	}
	if err := mgr.AddTeam(spec); err != nil {
		t.Fatalf("AddTeam: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Stop() })

	tt := mgr.Team("test-team")
	if tt == nil {
		t.Fatal("team not found after AddTeam + Start")
	}
	return tt
}

func waitForEvent(t *testing.T, bus *event.Bus, eventType string, timeout time.Duration) event.Event {
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

// stopWithTimeout calls b.Stop() and fails the test if it doesn't return
// within the deadline. This replaces time.Sleep for error-path tests where
// the claim loop exits via IsComplete after a terminal task failure.
func stopWithTimeout(t *testing.T, b *bridge.Bridge, timeout time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		b.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("bridge.Stop() did not return within timeout")
	}
}

// --- Tests ---------------------------------------------------------------

func TestBridge_ClaimAndComplete(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1", Files: []string{"a.go"}},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Stop()

	// Wait for the bridge to claim and start the task.
	e := waitForEvent(t, bus, "bridge.task_started", 2*time.Second)
	started := e.(event.BridgeTaskStartedEvent)
	if started.TaskID != "t1" {
		t.Errorf("started.TaskID = %q, want %q", started.TaskID, "t1")
	}
	if started.TeamID != "test-team" {
		t.Errorf("started.TeamID = %q, want %q", started.TeamID, "test-team")
	}

	// Verify the factory was called.
	created := factory.Created()
	if len(created) != 1 {
		t.Fatalf("factory.Created() = %d prompts, want 1", len(created))
	}

	// Verify the recorder was called.
	assigned := recorder.Assigned()
	if _, ok := assigned["t1"]; !ok {
		t.Error("recorder.AssignTask not called for t1")
	}

	// Verify the task is in the running map.
	running := b.Running()
	if _, ok := running["t1"]; !ok {
		t.Error("Running() should contain t1")
	}

	// Now signal completion via the mock checker.
	factory.mu.Lock()
	var worktreePath string
	for _, inst := range factory.instances {
		worktreePath = inst.worktreePath
		break
	}
	factory.mu.Unlock()

	checker.MarkComplete(worktreePath)

	// Wait for the bridge to complete the task.
	ce := waitForEvent(t, bus, "bridge.task_completed", 2*time.Second)
	completed := ce.(event.BridgeTaskCompletedEvent)
	if completed.TaskID != "t1" {
		t.Errorf("completed.TaskID = %q, want %q", completed.TaskID, "t1")
	}
	if !completed.Success {
		t.Error("completed.Success = false, want true")
	}
	if completed.CommitCount != 1 {
		t.Errorf("completed.CommitCount = %d, want 1", completed.CommitCount)
	}

	// Verify recorder got the completion.
	completedTasks := recorder.Completed()
	if count, ok := completedTasks["t1"]; !ok || count != 1 {
		t.Errorf("recorder.Completed[t1] = %d, want 1", count)
	}

	// Verify the running map is cleared.
	running = b.Running()
	if len(running) != 0 {
		t.Errorf("Running() = %v, want empty", running)
	}
}

func TestBridge_VerificationFailure(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	checker.verifyOK = false
	checker.verifyErr = errors.New("no commits")
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Stop()

	// Wait for task to start.
	waitForEvent(t, bus, "bridge.task_started", 2*time.Second)

	// Get worktree path and mark complete so monitor picks it up.
	factory.mu.Lock()
	var wtp string
	for _, inst := range factory.instances {
		wtp = inst.worktreePath
		break
	}
	factory.mu.Unlock()
	checker.MarkComplete(wtp)

	// Wait for bridge to report failure.
	ce := waitForEvent(t, bus, "bridge.task_completed", 2*time.Second)
	completed := ce.(event.BridgeTaskCompletedEvent)
	if completed.Success {
		t.Error("completed.Success = true, want false")
	}
	if completed.Error != "no commits" {
		t.Errorf("completed.Error = %q, want %q", completed.Error, "no commits")
	}

	// Verify recorder got the failure.
	failed := recorder.Failed()
	if reason, ok := failed["t1"]; !ok || reason != "no commits" {
		t.Errorf("recorder.Failed[t1] = %q, want %q", reason, "no commits")
	}
}

func TestBridge_CreateInstanceError(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	factory.createErr = errors.New("out of resources")
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// When CreateInstance fails, the bridge calls gate.Fail() making the task
	// terminal. The claim loop then sees IsComplete → exits. Stop blocks until
	// the claim loop goroutine finishes, so it returns once the error is processed.
	stopWithTimeout(t, b, 3*time.Second)

	// The task should not be in the running map.
	running := b.Running()
	if len(running) != 0 {
		t.Errorf("Running() = %v, want empty", running)
	}
}

func TestBridge_StartInstanceError(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	factory.startErr = errors.New("tmux unavailable")
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Same pattern as CreateInstanceError: the task becomes terminal via
	// gate.Fail(), so Stop returns once the claim loop exits.
	stopWithTimeout(t, b, 3*time.Second)

	running := b.Running()
	if len(running) != 0 {
		t.Errorf("Running() = %v, want empty", running)
	}
}

func TestBridge_DoubleStart(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus)

	ctx := context.Background()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Stop()

	if err := b.Start(ctx); err == nil {
		t.Error("second Start should return error")
	}
}

func TestBridge_StopBeforeStart(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus)
	// Should not panic.
	b.Stop()
	_ = tt
}

func TestBridge_ContextCancellation(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the task to start so the monitor goroutine is running.
	waitForEvent(t, bus, "bridge.task_started", 2*time.Second)

	// Cancel the context — the bridge should stop gracefully.
	cancel()
	b.Stop() // Should return quickly without hanging.
}

func TestBridge_MultipleTasks(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "First"},
		{ID: "t2", Title: "Task 2", Description: "Second"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Stop()

	// Collect two started events.
	started := make(chan event.Event, 2)
	subID := bus.Subscribe("bridge.task_started", func(e event.Event) {
		started <- e
	})
	defer bus.Unsubscribe(subID)

	// Wait for both tasks to be claimed and started.
	deadline := time.After(3 * time.Second)
	count := 0
	for count < 2 {
		select {
		case <-started:
			count++
		case <-deadline:
			t.Fatalf("only got %d/2 task_started events", count)
		}
	}

	// Both tasks should be running.
	running := b.Running()
	if len(running) != 2 {
		t.Errorf("Running() has %d entries, want 2", len(running))
	}

	// Complete both tasks.
	factory.mu.Lock()
	for _, inst := range factory.instances {
		checker.MarkComplete(inst.worktreePath)
	}
	factory.mu.Unlock()

	// Collect two completed events.
	completed := make(chan event.Event, 2)
	subID2 := bus.Subscribe("bridge.task_completed", func(e event.Event) {
		completed <- e
	})
	defer bus.Unsubscribe(subID2)

	deadline = time.After(3 * time.Second)
	count = 0
	for count < 2 {
		select {
		case <-completed:
			count++
		case <-deadline:
			t.Fatalf("only got %d/2 task_completed events", count)
		}
	}

	// Running map should be clear.
	running = b.Running()
	if len(running) != 0 {
		t.Errorf("Running() = %v, want empty", running)
	}
}

func TestBridge_WithLogger(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	logger := logging.NopLogger()
	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
		bridge.WithLogger(logger),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for task to start, then complete it.
	waitForEvent(t, bus, "bridge.task_started", 2*time.Second)

	factory.mu.Lock()
	for _, inst := range factory.instances {
		checker.MarkComplete(inst.worktreePath)
	}
	factory.mu.Unlock()

	waitForEvent(t, bus, "bridge.task_completed", 2*time.Second)
	b.Stop()
}

func TestBridge_IsCompleteExit(t *testing.T) {
	bus := event.NewBus()
	// Single task — once it completes, IsComplete should be true
	// and the claim loop should exit on its own.
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
	)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for task to start.
	waitForEvent(t, bus, "bridge.task_started", 2*time.Second)

	// Complete the task.
	factory.mu.Lock()
	for _, inst := range factory.instances {
		checker.MarkComplete(inst.worktreePath)
	}
	factory.mu.Unlock()

	// Wait for completion event.
	waitForEvent(t, bus, "bridge.task_completed", 2*time.Second)

	// The claim loop should exit on its own via IsComplete(). Stop
	// should return quickly without needing context cancellation.
	done := make(chan struct{})
	go func() {
		b.Stop()
		close(done)
	}()

	select {
	case <-done:
		// good — bridge stopped via IsComplete exit
	case <-time.After(3 * time.Second):
		t.Error("bridge.Stop() did not return within timeout; IsComplete exit may not be working")
	}
}

func TestBridge_NilConstructorPanics(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	tests := []struct {
		name  string
		build func()
	}{
		{"nil team", func() { bridge.New(nil, factory, checker, recorder, bus) }},
		{"nil factory", func() { bridge.New(tt, nil, checker, recorder, bus) }},
		{"nil checker", func() { bridge.New(tt, factory, nil, recorder, bus) }},
		{"nil recorder", func() { bridge.New(tt, factory, checker, nil, bus) }},
		{"nil bus", func() { bridge.New(tt, factory, checker, recorder, nil) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic for nil argument")
				}
			}()
			tc.build()
		})
	}
}

func TestBridge_WithLoggerNil(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	factory := newMockFactory()
	checker := newMockChecker()
	recorder := newMockRecorder()

	// WithLogger(nil) should not cause a nil-pointer panic.
	b := bridge.New(tt, factory, checker, recorder, bus,
		bridge.WithPollInterval(10*time.Millisecond),
		bridge.WithLogger(nil),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for task to start, then complete it.
	waitForEvent(t, bus, "bridge.task_started", 2*time.Second)

	factory.mu.Lock()
	for _, inst := range factory.instances {
		checker.MarkComplete(inst.worktreePath)
	}
	factory.mu.Unlock()

	waitForEvent(t, bus, "bridge.task_completed", 2*time.Second)
	b.Stop()
}

func TestBridge_MaxCheckErrorsFailsTask(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1", Description: "Do thing 1"},
	}
	tt := newTestTeam(t, bus, tasks)

	// Disable retries so the first gate.Fail() makes the task permanently
	// failed. Without this, the retry logic returns the task to pending and
	// the claim loop re-claims it, racing with the monitor's cleanup.
	if err := tt.Hub().TaskQueue().SetMaxRetries("t1", 0); err != nil {
		t.Fatalf("SetMaxRetries: %v", err)
	}

	factory := newMockFactory()

	// Use a signaling recorder so we can wait for RecordFailure before stopping.
	failureCh := make(chan string, 1)
	recorder := &signalingRecorder{
		inner:     newMockRecorder(),
		failureCh: failureCh,
	}

	// Use a checker that always errors on CheckCompletion.
	errChecker := &errorChecker{err: errors.New("disk I/O error")}

	b := bridge.New(tt, factory, errChecker, recorder, bus,
		bridge.WithPollInterval(time.Millisecond),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Stop()

	// Wait for the recorder to receive the failure signal.
	select {
	case taskID := <-failureCh:
		if taskID != "t1" {
			t.Errorf("failed task = %q, want %q", taskID, "t1")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for recorder failure")
	}

	// Running map should be empty.
	running := b.Running()
	if len(running) != 0 {
		t.Errorf("Running() = %v, want empty", running)
	}
}

// errorChecker is a CompletionChecker whose CheckCompletion always returns
// an error. The factory still succeeds, so the bridge starts the monitor.
type errorChecker struct {
	err error
}

func (c *errorChecker) CheckCompletion(_ string) (bool, error) {
	return false, c.err
}

func (c *errorChecker) VerifyWork(_, _, _, _ string) (bool, int, error) {
	return false, 0, c.err
}

// signalingRecorder wraps a mockRecorder and sends the task ID on a channel
// when RecordFailure is called. Used to synchronize tests without time.Sleep.
type signalingRecorder struct {
	inner     *mockRecorder
	failureCh chan<- string
}

func (r *signalingRecorder) AssignTask(taskID, instanceID string) {
	r.inner.AssignTask(taskID, instanceID)
}

func (r *signalingRecorder) RecordCompletion(taskID string, commitCount int) {
	r.inner.RecordCompletion(taskID, commitCount)
}

func (r *signalingRecorder) RecordFailure(taskID, reason string) {
	r.inner.RecordFailure(taskID, reason)
	select {
	case r.failureCh <- taskID:
	default:
	}
}

func TestBuildTaskPrompt(t *testing.T) {
	prompt := bridge.BuildTaskPrompt(
		"Add auth middleware",
		"Implement JWT validation in the API gateway.",
		[]string{"api/middleware.go", "api/auth.go"},
	)

	if !strings.Contains(prompt, "Add auth middleware") {
		t.Error("prompt missing title")
	}
	if !strings.Contains(prompt, "Implement JWT validation") {
		t.Error("prompt missing description")
	}
	if !strings.Contains(prompt, "api/middleware.go") {
		t.Error("prompt missing file list")
	}
}

func TestBuildTaskPrompt_NoFiles(t *testing.T) {
	prompt := bridge.BuildTaskPrompt("Review", "Review all changes.", nil)

	if !strings.Contains(prompt, "Review") {
		t.Error("prompt missing title")
	}
	if strings.Contains(prompt, "## Files") {
		t.Error("prompt should not contain Files section when no files specified")
	}
}

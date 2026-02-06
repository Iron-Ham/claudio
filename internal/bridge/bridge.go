package bridge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/team"
)

// Bridge connects a single team's Hub to real Claude Code instances.
//
// It claims tasks from the team's Gate, creates instances via the
// InstanceFactory, monitors them for completion, and reports outcomes
// back through the Gate and SessionRecorder.
type Bridge struct {
	team     *team.Team
	factory  InstanceFactory
	checker  CompletionChecker
	recorder SessionRecorder
	bus      *event.Bus
	logger   *logging.Logger

	pollInterval time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.RWMutex
	running map[string]string // taskID → instanceID
	started bool
}

// New creates a Bridge for the given team.
//
// All interface arguments (factory, checker, recorder) and the bus must be
// non-nil. Passing nil will panic early to surface wiring bugs immediately.
func New(t *team.Team, factory InstanceFactory, checker CompletionChecker, recorder SessionRecorder, bus *event.Bus, opts ...Option) *Bridge {
	if t == nil {
		panic("bridge: team must not be nil")
	}
	if factory == nil {
		panic("bridge: InstanceFactory must not be nil")
	}
	if checker == nil {
		panic("bridge: CompletionChecker must not be nil")
	}
	if recorder == nil {
		panic("bridge: SessionRecorder must not be nil")
	}
	if bus == nil {
		panic("bridge: event.Bus must not be nil")
	}

	cfg := &config{
		pollInterval: defaultPollInterval,
		logger:       logging.NopLogger(),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.pollInterval <= 0 {
		cfg.pollInterval = defaultPollInterval
	}
	if cfg.logger == nil {
		cfg.logger = logging.NopLogger()
	}

	return &Bridge{
		team:         t,
		factory:      factory,
		checker:      checker,
		recorder:     recorder,
		bus:          bus,
		logger:       cfg.logger,
		pollInterval: cfg.pollInterval,
		running:      make(map[string]string),
	}
}

// Start begins the claim loop. It returns immediately; the loop runs in
// a background goroutine. Call Stop to shut down.
func (b *Bridge) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return fmt.Errorf("bridge: already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	b.ctx = ctx
	b.cancel = cancel
	b.started = true

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		b.claimLoop()
	}()

	return nil
}

// Stop cancels the context and waits for all goroutines to finish.
// It is safe to call multiple times.
func (b *Bridge) Stop() {
	b.mu.Lock()
	if !b.started {
		b.mu.Unlock()
		return
	}
	b.cancel()
	b.mu.Unlock()

	b.wg.Wait()

	b.mu.Lock()
	b.started = false
	b.mu.Unlock()
}

// Running returns the current mapping of taskID → instanceID for active tasks.
func (b *Bridge) Running() map[string]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	out := make(map[string]string, len(b.running))
	for k, v := range b.running {
		out[k] = v
	}
	return out
}

// claimLoop continuously claims tasks from the team's Gate and spawns
// monitor goroutines for each one. It exits when the context is cancelled.
func (b *Bridge) claimLoop() {
	// Subscribe to queue depth changes so we can wake up when new tasks appear.
	wake := make(chan struct{}, 1)
	subID := b.bus.Subscribe("queue.depth_changed", func(_ event.Event) {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	defer b.bus.Unsubscribe(subID)

	for {
		if err := b.ctx.Err(); err != nil {
			return
		}

		gate := b.team.Hub().Gate()

		// Use the team ID as a claim identifier for traceability.
		// The real instance ID is recorded after CreateInstance.
		claimID := fmt.Sprintf("bridge-%s", b.team.Spec().ID)

		task, err := gate.ClaimNext(claimID)
		if err != nil {
			b.logger.Error("bridge claim failed", "team", b.team.Spec().ID, "error", err)
			b.waitForWake(wake)
			continue
		}

		if task == nil {
			if gate.IsComplete() {
				return
			}
			b.waitForWake(wake)
			continue
		}

		// Build a prompt and create an instance.
		prompt := BuildTaskPrompt(task.Title, task.Description, task.Files)
		inst, err := b.factory.CreateInstance(prompt)
		if err != nil {
			b.logger.Error("bridge: failed to create instance",
				"team", b.team.Spec().ID, "task", task.ID, "error", err)
			if failErr := gate.Fail(task.ID, fmt.Sprintf("create instance: %v", err)); failErr != nil {
				b.logger.Error("bridge: gate.Fail also failed",
					"task", task.ID, "error", failErr)
			}
			continue
		}

		if err := b.factory.StartInstance(inst); err != nil {
			b.logger.Error("bridge: failed to start instance",
				"team", b.team.Spec().ID, "task", task.ID, "error", err)
			if failErr := gate.Fail(task.ID, fmt.Sprintf("start instance: %v", err)); failErr != nil {
				b.logger.Error("bridge: gate.Fail also failed",
					"task", task.ID, "error", failErr)
			}
			continue
		}

		// Transition the task to running.
		if err := gate.MarkRunning(task.ID); err != nil {
			b.logger.Error("bridge: failed to mark running",
				"team", b.team.Spec().ID, "task", task.ID, "error", err)
			if failErr := gate.Fail(task.ID, fmt.Sprintf("mark running: %v", err)); failErr != nil {
				b.logger.Error("bridge: gate.Fail also failed",
					"task", task.ID, "error", failErr)
			}
			continue
		}

		// Record assignment and publish event.
		b.recorder.AssignTask(task.ID, inst.ID())

		b.mu.Lock()
		b.running[task.ID] = inst.ID()
		b.mu.Unlock()

		b.bus.Publish(event.NewBridgeTaskStartedEvent(
			b.team.Spec().ID, task.ID, inst.ID(),
		))

		// Spawn a monitor goroutine for this task.
		b.wg.Add(1)
		go func(taskID string, inst Instance) {
			defer b.wg.Done()
			b.monitorInstance(taskID, inst)
		}(task.ID, inst)
	}
}

// waitForWake blocks until either the wake channel fires or the context is cancelled.
func (b *Bridge) waitForWake(wake <-chan struct{}) {
	select {
	case <-b.ctx.Done():
	case <-wake:
	}
}

// maxCheckErrors is the number of consecutive completion check failures before
// the bridge gives up and fails the task. This prevents indefinite retries
// when the worktree path is invalid or the filesystem is unhealthy.
const maxCheckErrors = 10

// monitorInstance polls for instance completion and reports the result.
func (b *Bridge) monitorInstance(taskID string, inst Instance) {
	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()

	consecutiveErrors := 0

	for {
		select {
		case <-b.ctx.Done():
			b.logger.Info("bridge: monitor cancelled, cleaning up",
				"task", taskID)
			b.mu.Lock()
			delete(b.running, taskID)
			b.mu.Unlock()
			return
		case <-ticker.C:
		}

		done, err := b.checker.CheckCompletion(inst.WorktreePath())
		if err != nil {
			consecutiveErrors++
			b.logger.Warn("bridge: completion check error",
				"task", taskID, "error", err,
				"consecutive", consecutiveErrors)
			if consecutiveErrors >= maxCheckErrors {
				b.logger.Error("bridge: max check errors reached, failing task",
					"task", taskID, "limit", maxCheckErrors)
				gate := b.team.Hub().Gate()
				reason := fmt.Sprintf("completion check failed %d times: %v", maxCheckErrors, err)
				if failErr := gate.Fail(taskID, reason); failErr != nil {
					b.logger.Error("bridge: gate.Fail failed after check errors",
						"task", taskID, "error", failErr)
				}
				// Clean up running map before recording, so observers see
				// a consistent state when the recorder callback fires.
				b.mu.Lock()
				delete(b.running, taskID)
				b.mu.Unlock()
				b.recorder.RecordFailure(taskID, reason)
				return
			}
			continue
		}
		consecutiveErrors = 0

		if !done {
			continue
		}

		// Instance wrote its sentinel file. Verify the work.
		success, commitCount, verifyErr := b.checker.VerifyWork(
			taskID, inst.ID(), inst.WorktreePath(), inst.Branch(),
		)

		gate := b.team.Hub().Gate()
		teamID := b.team.Spec().ID

		// Clean up running map before recording/publishing so observers see
		// consistent state when callbacks or event handlers fire.
		b.mu.Lock()
		delete(b.running, taskID)
		b.mu.Unlock()

		if success {
			if _, completeErr := gate.Complete(taskID); completeErr != nil {
				b.logger.Error("bridge: failed to complete task",
					"task", taskID, "error", completeErr)
			}
			b.recorder.RecordCompletion(taskID, commitCount)
			b.bus.Publish(event.NewBridgeTaskCompletedEvent(
				teamID, taskID, inst.ID(), true, commitCount, "",
			))
		} else {
			reason := "verification failed"
			if verifyErr != nil {
				reason = verifyErr.Error()
			}
			if failErr := gate.Fail(taskID, reason); failErr != nil {
				b.logger.Error("bridge: failed to fail task",
					"task", taskID, "error", failErr)
			}
			b.recorder.RecordFailure(taskID, reason)
			b.bus.Publish(event.NewBridgeTaskCompletedEvent(
				teamID, taskID, inst.ID(), false, commitCount, reason,
			))
		}

		return
	}
}

// BuildTaskPrompt formats task fields into a prompt string for a Claude Code instance.
// Accepts basic fields rather than a concrete type to avoid import cycles.
// The coordinator adapters may use the orchestrator's prompt.TaskBuilder for
// richer formatting; this is the bridge-level default.
func BuildTaskPrompt(title, description string, files []string) string {
	var sb strings.Builder
	sb.WriteString("# Task: ")
	sb.WriteString(title)
	sb.WriteString("\n\n")
	sb.WriteString(description)

	if len(files) > 0 {
		sb.WriteString("\n\n## Files\n")
		for _, f := range files {
			sb.WriteString("- ")
			sb.WriteString(f)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

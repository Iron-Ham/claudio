package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/util"
)

// RalphCoordinatorCallbacks holds callbacks for coordinator events.
type RalphCoordinatorCallbacks struct {
	// OnIterationStart is called when a new iteration begins.
	OnIterationStart func(iteration int, instanceID string)

	// OnIterationComplete is called when an iteration finishes.
	OnIterationComplete func(iteration int)

	// OnPromiseFound is called when the completion promise is detected.
	OnPromiseFound func(iteration int)

	// OnMaxIterations is called when max iterations limit is reached.
	OnMaxIterations func(iteration int)

	// OnComplete is called when the ralph session completes (for any reason).
	OnComplete func(phase RalphPhase, summary string)
}

// RalphCoordinator orchestrates the execution of a Ralph Wiggum iterative loop.
type RalphCoordinator struct {
	orch        *Orchestrator
	baseSession *Session
	session     *RalphSession
	callbacks   *RalphCoordinatorCallbacks
	logger      *logging.Logger

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// State tracking
	worktree string // Worktree path for all iterations
}

// NewRalphCoordinator creates a new coordinator for a Ralph Wiggum loop session.
func NewRalphCoordinator(orch *Orchestrator, baseSession *Session, ralphSession *RalphSession, logger *logging.Logger) *RalphCoordinator {
	if logger == nil {
		logger = logging.NopLogger()
	}

	ctx, cancel := context.WithCancel(context.Background())

	sessionLogger := logger.WithPhase("ralph-coordinator")

	return &RalphCoordinator{
		orch:        orch,
		baseSession: baseSession,
		session:     ralphSession,
		logger:      sessionLogger,
		ctx:         ctx,
		cancelFunc:  cancel,
	}
}

// SetCallbacks sets the coordinator callbacks.
func (c *RalphCoordinator) SetCallbacks(cb *RalphCoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Session returns the ralph session.
func (c *RalphCoordinator) Session() *RalphSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

// notifyIterationStart notifies callbacks of iteration start.
func (c *RalphCoordinator) notifyIterationStart(iteration int, instanceID string) {
	c.logger.Info("iteration started",
		"iteration", iteration,
		"instance_id", instanceID,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnIterationStart != nil {
		cb.OnIterationStart(iteration, instanceID)
	}
}

// notifyIterationComplete notifies callbacks of iteration completion.
func (c *RalphCoordinator) notifyIterationComplete(iteration int) {
	c.logger.Info("iteration complete", "iteration", iteration)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnIterationComplete != nil {
		cb.OnIterationComplete(iteration)
	}
}

// notifyPromiseFound notifies callbacks that completion promise was found.
func (c *RalphCoordinator) notifyPromiseFound(iteration int) {
	c.logger.Info("completion promise found", "iteration", iteration)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPromiseFound != nil {
		cb.OnPromiseFound(iteration)
	}
}

// notifyMaxIterations notifies callbacks that max iterations was reached.
func (c *RalphCoordinator) notifyMaxIterations(iteration int) {
	c.logger.Info("max iterations reached", "iteration", iteration)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnMaxIterations != nil {
		cb.OnMaxIterations(iteration)
	}
}

// notifyComplete notifies callbacks of session completion.
func (c *RalphCoordinator) notifyComplete(phase RalphPhase, summary string) {
	c.logger.Info("ralph session complete",
		"phase", string(phase),
		"summary", summary,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnComplete != nil {
		cb.OnComplete(phase, summary)
	}
}

// StartIteration starts a new iteration of the ralph loop.
func (c *RalphCoordinator) StartIteration() error {
	c.mu.Lock()
	session := c.session

	// Check if we should continue (while holding lock)
	if !session.ShouldContinue() {
		phase := session.Phase
		c.mu.Unlock()
		if phase == PhaseRalphCancelled {
			return fmt.Errorf("ralph loop was cancelled")
		}
		if phase == PhaseRalphComplete {
			return fmt.Errorf("ralph loop already complete")
		}
		return fmt.Errorf("ralph loop cannot continue (phase: %s)", phase)
	}

	// Increment iteration counter while holding lock
	session.IncrementIteration()
	iteration := session.CurrentIteration

	// Initialize timing on first iteration
	if session.StartedAt.IsZero() {
		session.StartedAt = time.Now()
	}

	// Capture values needed for logging and operations
	prompt := session.Prompt
	maxIterations := session.Config.MaxIterations
	groupID := session.GroupID

	// Release lock before external operations
	c.mu.Unlock()

	c.logger.Info("starting iteration",
		"prompt", util.TruncateString(prompt, 100),
		"iteration", iteration,
		"max_iterations", maxIterations,
	)

	// Find the ralph group to add instances to
	var ralphGroup *InstanceGroup
	if groupID != "" {
		ralphGroup = c.baseSession.GetGroup(groupID)
	}
	if ralphGroup == nil {
		ralphGroup = c.baseSession.GetGroupBySessionType(SessionTypeRalph)
	}

	// Create or reuse instance
	var inst *Instance
	var err error

	if iteration == 1 {
		// First iteration - create a fresh worktree
		inst, err = c.orch.AddInstance(c.baseSession, prompt)
		if err != nil {
			c.mu.Lock()
			session.MarkError(err)
			c.mu.Unlock()
			return fmt.Errorf("failed to create ralph instance: %w", err)
		}
		c.mu.Lock()
		c.worktree = inst.WorktreePath
		c.mu.Unlock()
	} else {
		// Subsequent iterations - reuse the same worktree
		c.mu.RLock()
		worktree := c.worktree
		c.mu.RUnlock()

		inst, err = c.orch.AddInstanceToWorktree(c.baseSession, prompt, worktree, "")
		if err != nil {
			c.mu.Lock()
			session.MarkError(err)
			c.mu.Unlock()
			return fmt.Errorf("failed to create ralph instance for iteration %d: %w", iteration, err)
		}
	}

	// Add instance to the ralph group for sidebar display
	if ralphGroup != nil {
		ralphGroup.AddInstance(inst.ID)
	}

	// Store current instance ID (needs lock for session modification)
	c.mu.Lock()
	session.SetInstanceID(inst.ID)
	c.mu.Unlock()

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		c.mu.Lock()
		session.MarkError(err)
		c.mu.Unlock()
		return fmt.Errorf("failed to start ralph iteration: %w", err)
	}

	c.notifyIterationStart(iteration, inst.ID)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting iteration", "error", err)
	}

	return nil
}

// CheckCompletionInOutput checks if the completion promise is in the given output.
// Returns true if the promise was found and the session should stop.
func (c *RalphCoordinator) CheckCompletionInOutput(output string) bool {
	c.mu.RLock()
	session := c.session
	c.mu.RUnlock()

	return session.CheckCompletionPromise(output)
}

// ProcessIterationCompletion handles when the current iteration's instance completes.
// Returns true if another iteration should be started, false if the loop is done.
func (c *RalphCoordinator) ProcessIterationCompletion(output string) (continueLoop bool, err error) {
	c.mu.Lock()
	session := c.session
	iteration := session.CurrentIteration
	maxIterations := session.Config.MaxIterations
	promiseFound := session.CheckCompletionPromise(output)
	c.mu.Unlock()

	c.notifyIterationComplete(iteration)

	// Check for completion promise in output
	if promiseFound {
		c.mu.Lock()
		session.MarkComplete()
		c.mu.Unlock()

		c.notifyPromiseFound(iteration)

		summary := fmt.Sprintf("Ralph loop completed after %d iteration(s) - completion promise found", iteration)
		c.notifyComplete(PhaseRalphComplete, summary)

		// Persist completion
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist completion", "error", err)
		}

		return false, nil
	}

	// Check if max iterations reached
	if maxIterations > 0 && iteration >= maxIterations {
		c.mu.Lock()
		session.MarkMaxIterationsReached()
		c.mu.Unlock()

		c.notifyMaxIterations(iteration)

		summary := fmt.Sprintf("Ralph loop stopped after %d iteration(s) - max iterations reached", iteration)
		c.notifyComplete(PhaseRalphMaxIterations, summary)

		// Persist state
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist max iterations state", "error", err)
		}

		return false, nil
	}

	// Continue the loop - start next iteration
	return true, nil
}

// Cancel cancels the ralph loop.
func (c *RalphCoordinator) Cancel() {
	c.mu.Lock()
	session := c.session
	session.MarkCancelled()
	iteration := session.CurrentIteration
	c.mu.Unlock()

	c.cancelFunc()

	summary := fmt.Sprintf("Ralph loop cancelled after %d iteration(s)", iteration)
	c.notifyComplete(PhaseRalphCancelled, summary)

	// Persist cancellation
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist cancellation", "error", err)
	}
}

// Stop stops the ralph coordinator.
func (c *RalphCoordinator) Stop() {
	c.cancelFunc()
	c.wg.Wait()
}

// GetWorktree returns the worktree path for the ralph loop.
func (c *RalphCoordinator) GetWorktree() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.worktree
}

// SetWorktree sets the worktree path (used when restoring session).
func (c *RalphCoordinator) SetWorktree(worktree string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.worktree = worktree
}

// GetCurrentInstanceID returns the ID of the current active instance.
func (c *RalphCoordinator) GetCurrentInstanceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session.InstanceID
}

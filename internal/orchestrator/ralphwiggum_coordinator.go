package orchestrator

import (
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/util"
)

// RalphWiggumCoordinatorCallbacks holds callbacks for coordinator events
type RalphWiggumCoordinatorCallbacks struct {
	// OnPhaseChange is called when the phase changes
	OnPhaseChange func(phase RalphWiggumPhase)

	// OnIterationStart is called when a new iteration begins
	OnIterationStart func(iteration int)

	// OnIterationComplete is called when an iteration completes
	OnIterationComplete func(iteration int, hasCommits bool)

	// OnPromiseFound is called when the completion promise is detected
	OnPromiseFound func(promise string)

	// OnMaxIterations is called when max iterations is reached
	OnMaxIterations func(iteration int)

	// OnComplete is called when the session completes
	OnComplete func(success bool, summary string)
}

// RalphWiggumCoordinator orchestrates the execution of a Ralph Wiggum session
type RalphWiggumCoordinator struct {
	manager     *RalphWiggumManager
	orch        *Orchestrator
	baseSession *Session
	callbacks   *RalphWiggumCoordinatorCallbacks
	logger      *logging.Logger

	// Running state
	mu      sync.RWMutex
	stopped bool

	// Instance tracking
	instanceID string

	// Iteration control
	pendingContinue bool   // Set when iteration completes and waiting to continue
	lastOutputCheck string // Last output checked for promise
}

// NewRalphWiggumCoordinator creates a new coordinator for a Ralph Wiggum session
func NewRalphWiggumCoordinator(orch *Orchestrator, baseSession *Session, ralphSession *RalphWiggumSession, logger *logging.Logger) *RalphWiggumCoordinator {
	if logger == nil {
		logger = logging.NopLogger()
	}
	manager := NewRalphWiggumManager(orch, baseSession, ralphSession, logger)

	sessionLogger := logger.WithSession(ralphSession.ID).WithPhase("ralphwiggum-coordinator")

	return &RalphWiggumCoordinator{
		manager:     manager,
		orch:        orch,
		baseSession: baseSession,
		logger:      sessionLogger,
	}
}

// SetCallbacks sets the coordinator callbacks
func (c *RalphWiggumCoordinator) SetCallbacks(cb *RalphWiggumCoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Manager returns the underlying Ralph Wiggum manager
func (c *RalphWiggumCoordinator) Manager() *RalphWiggumManager {
	return c.manager
}

// Session returns the Ralph Wiggum session
func (c *RalphWiggumCoordinator) Session() *RalphWiggumSession {
	return c.manager.Session()
}

// notifyPhaseChange notifies callbacks of phase change
func (c *RalphWiggumCoordinator) notifyPhaseChange(phase RalphWiggumPhase) {
	session := c.Session()
	fromPhase := session.Phase

	c.manager.SetPhase(phase)

	c.logger.Info("phase changed",
		"from_phase", string(fromPhase),
		"to_phase", string(phase),
		"session_id", session.ID,
	)

	// Persist the phase change
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist phase change", "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPhaseChange != nil {
		cb.OnPhaseChange(phase)
	}
}

// notifyIterationStart notifies callbacks of iteration start
func (c *RalphWiggumCoordinator) notifyIterationStart(iteration int) {
	c.manager.StartIteration()

	c.logger.Info("iteration started", "iteration", iteration)

	// Persist iteration start
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist iteration start", "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnIterationStart != nil {
		cb.OnIterationStart(iteration)
	}
}

// notifyIterationComplete notifies callbacks of iteration completion
func (c *RalphWiggumCoordinator) notifyIterationComplete(iteration int, hasCommits bool) {
	c.manager.CompleteIteration(hasCommits)

	// Persist iteration completion
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist iteration completion", "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnIterationComplete != nil {
		cb.OnIterationComplete(iteration, hasCommits)
	}
}

// notifyPromiseFound notifies callbacks when completion promise is found
func (c *RalphWiggumCoordinator) notifyPromiseFound(promise string) {
	c.logger.Info("completion promise found", "promise", promise)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPromiseFound != nil {
		cb.OnPromiseFound(promise)
	}
}

// notifyMaxIterations notifies callbacks when max iterations reached
func (c *RalphWiggumCoordinator) notifyMaxIterations(iteration int) {
	c.logger.Info("max iterations reached", "iteration", iteration)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnMaxIterations != nil {
		cb.OnMaxIterations(iteration)
	}
}

// notifyComplete notifies callbacks of completion
func (c *RalphWiggumCoordinator) notifyComplete(success bool, summary string) {
	c.logger.Info("ralph wiggum complete",
		"success", success,
		"summary", summary,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnComplete != nil {
		cb.OnComplete(success, summary)
	}
}

// StartFirstIteration starts the first iteration of the Ralph Wiggum loop
func (c *RalphWiggumCoordinator) StartFirstIteration() error {
	session := c.Session()
	task := session.Task
	config := session.Config

	c.logger.Info("starting ralph wiggum loop",
		"task", util.TruncateString(task, 100),
		"completion_promise", config.CompletionPromise,
		"max_iterations", config.MaxIterations,
	)

	now := time.Now()
	session.StartedAt = &now

	// Find the Ralph Wiggum group to add the instance to
	var ralphGroup *InstanceGroup
	if session.GroupID != "" {
		ralphGroup = c.baseSession.GetGroup(session.GroupID)
	}

	// Build the first iteration prompt
	prompt := c.buildIterationPrompt(1)

	// Create instance for the loop
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create ralph wiggum instance: %w", err)
	}

	// Add instance to the group for sidebar display
	if ralphGroup != nil {
		ralphGroup.AddInstance(inst.ID)
	}

	session.InstanceID = inst.ID

	c.mu.Lock()
	c.instanceID = inst.ID
	c.mu.Unlock()

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start ralph wiggum instance: %w", err)
	}

	c.notifyIterationStart(1)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting", "error", err)
	}

	return nil
}

// buildIterationPrompt builds the prompt for a specific iteration
func (c *RalphWiggumCoordinator) buildIterationPrompt(iteration int) string {
	session := c.Session()

	var extra string
	if iteration == 1 {
		extra = RalphWiggumFirstIterationExtra
	} else {
		extra = RalphWiggumContinueIterationExtra
	}

	return fmt.Sprintf(RalphWiggumPromptTemplate,
		session.Task,
		session.Config.CompletionPromise,
		iteration,
		extra,
	)
}

// CheckInstanceCompletion checks if the instance has completed and whether the promise was found
// Returns (instanceComplete, promiseFound, error)
func (c *RalphWiggumCoordinator) CheckInstanceCompletion() (bool, bool, error) {
	session := c.Session()
	if session.InstanceID == "" {
		return false, false, nil
	}

	// Get the instance
	inst := c.baseSession.GetInstance(session.InstanceID)
	if inst == nil {
		return false, false, fmt.Errorf("instance %s not found", session.InstanceID)
	}

	// Check if instance has completed
	if inst.Status != StatusCompleted && inst.Status != StatusError {
		return false, false, nil
	}

	// Get instance output and check for completion promise
	mgr := c.orch.GetInstanceManager(inst.ID)
	if mgr == nil {
		return true, false, nil // Instance complete but can't check output
	}

	output := string(mgr.GetOutput())

	// Only check new output since last check
	c.mu.Lock()
	lastCheck := c.lastOutputCheck
	c.lastOutputCheck = output
	c.mu.Unlock()

	if output == lastCheck {
		// No new output, instance is complete
		return true, false, nil
	}

	// Check for completion promise in output
	promiseFound := CheckOutputForCompletionPromise(output, session.Config.CompletionPromise)

	return true, promiseFound, nil
}

// ProcessIterationComplete handles when an iteration completes
func (c *RalphWiggumCoordinator) ProcessIterationComplete(promiseFound bool) error {
	session := c.Session()
	currentIteration := session.CurrentIteration()

	// Check if instance made commits (simplified - could be enhanced)
	hasCommits := false // Would need to check git log

	c.notifyIterationComplete(currentIteration, hasCommits)

	if promiseFound {
		// Task complete!
		c.notifyPromiseFound(session.Config.CompletionPromise)
		c.notifyPhaseChange(PhaseRalphWiggumComplete)

		now := time.Now()
		session.CompletedAt = &now

		summary := fmt.Sprintf("Completed after %d iterations", currentIteration)
		c.notifyComplete(true, summary)

		return nil
	}

	// Check if we've hit max iterations
	if session.Config.MaxIterations > 0 && currentIteration >= session.Config.MaxIterations {
		c.notifyMaxIterations(currentIteration)
		c.notifyPhaseChange(PhaseRalphWiggumMaxIterations)

		now := time.Now()
		session.CompletedAt = &now

		summary := fmt.Sprintf("Reached maximum iterations (%d) without completion", currentIteration)
		c.notifyComplete(false, summary)

		return nil
	}

	// Mark that we're ready for another iteration
	c.mu.Lock()
	c.pendingContinue = true
	c.mu.Unlock()

	return nil
}

// ContinueIteration starts the next iteration of the loop
func (c *RalphWiggumCoordinator) ContinueIteration() error {
	session := c.Session()
	if session.IsComplete() {
		return fmt.Errorf("session is already complete")
	}

	nextIteration := session.CurrentIteration() + 1

	c.logger.Info("continuing to next iteration", "iteration", nextIteration)

	// Build the prompt for this iteration
	prompt := c.buildIterationPrompt(nextIteration)

	// Get the existing instance and send the prompt
	inst := c.baseSession.GetInstance(session.InstanceID)
	if inst == nil {
		return fmt.Errorf("instance %s not found", session.InstanceID)
	}

	// Send the prompt to the instance via tmux
	mgr := c.orch.GetInstanceManager(inst.ID)
	if mgr == nil {
		return fmt.Errorf("instance manager not found for %s", inst.ID)
	}

	// Reset the instance status to working
	inst.Status = StatusWorking

	// Send the prompt as input to continue
	mgr.SendInput([]byte(prompt + "\n"))

	c.mu.Lock()
	c.pendingContinue = false
	c.mu.Unlock()

	c.notifyIterationStart(nextIteration)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after continuing", "error", err)
	}

	return nil
}

// IsPendingContinue returns true if waiting for user to continue
func (c *RalphWiggumCoordinator) IsPendingContinue() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pendingContinue
}

// Pause pauses the Ralph Wiggum loop
func (c *RalphWiggumCoordinator) Pause() {
	c.notifyPhaseChange(PhaseRalphWiggumPaused)
}

// Resume resumes a paused Ralph Wiggum loop
func (c *RalphWiggumCoordinator) Resume() error {
	session := c.Session()
	if session.Phase != PhaseRalphWiggumPaused {
		return fmt.Errorf("session is not paused")
	}
	c.notifyPhaseChange(PhaseRalphWiggumIterating)
	return nil
}

// Stop stops the Ralph Wiggum execution
func (c *RalphWiggumCoordinator) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = true
}

// GetInstanceID returns the instance ID for the Ralph Wiggum loop
func (c *RalphWiggumCoordinator) GetInstanceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.instanceID
}

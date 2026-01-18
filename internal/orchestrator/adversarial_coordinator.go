package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/util"
)

// AdversarialCoordinatorCallbacks holds callbacks for coordinator events
type AdversarialCoordinatorCallbacks struct {
	// OnPhaseChange is called when the adversarial phase changes
	OnPhaseChange func(phase AdversarialPhase)

	// OnImplementerStart is called when the implementer begins a round
	OnImplementerStart func(round int, instanceID string)

	// OnIncrementReady is called when the implementer submits work for review
	OnIncrementReady func(round int, increment *AdversarialIncrementFile)

	// OnReviewerStart is called when the reviewer begins review
	OnReviewerStart func(round int, instanceID string)

	// OnReviewReady is called when the reviewer completes review
	OnReviewReady func(round int, review *AdversarialReviewFile)

	// OnApproved is called when the reviewer approves the implementation
	OnApproved func(round int, review *AdversarialReviewFile)

	// OnRejected is called when the reviewer rejects and requests changes
	OnRejected func(round int, review *AdversarialReviewFile)

	// OnComplete is called when the adversarial session completes
	OnComplete func(success bool, summary string)
}

// AdversarialCoordinator orchestrates the execution of an adversarial review session
type AdversarialCoordinator struct {
	manager     *AdversarialManager
	orch        *Orchestrator
	baseSession *Session
	callbacks   *AdversarialCoordinatorCallbacks
	logger      *logging.Logger

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// State tracking
	implementerWorktree string // Worktree path for implementer
	reviewerWorktree    string // Worktree path for reviewer (same as implementer)
}

// NewAdversarialCoordinator creates a new coordinator for an adversarial session
func NewAdversarialCoordinator(orch *Orchestrator, baseSession *Session, advSession *AdversarialSession, logger *logging.Logger) *AdversarialCoordinator {
	if logger == nil {
		logger = logging.NopLogger()
	}
	manager := NewAdversarialManager(orch, baseSession, advSession, logger)

	ctx, cancel := context.WithCancel(context.Background())

	sessionLogger := logger.WithSession(advSession.ID).WithPhase("adversarial-coordinator")

	return &AdversarialCoordinator{
		manager:     manager,
		orch:        orch,
		baseSession: baseSession,
		logger:      sessionLogger,
		ctx:         ctx,
		cancelFunc:  cancel,
	}
}

// SetCallbacks sets the coordinator callbacks
func (c *AdversarialCoordinator) SetCallbacks(cb *AdversarialCoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Manager returns the underlying adversarial manager
func (c *AdversarialCoordinator) Manager() *AdversarialManager {
	return c.manager
}

// Session returns the adversarial session
func (c *AdversarialCoordinator) Session() *AdversarialSession {
	return c.manager.Session()
}

// notifyPhaseChange notifies callbacks of phase change
func (c *AdversarialCoordinator) notifyPhaseChange(phase AdversarialPhase) {
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

// notifyImplementerStart notifies callbacks of implementer start
func (c *AdversarialCoordinator) notifyImplementerStart(round int, instanceID string) {
	c.logger.Info("implementer started",
		"round", round,
		"instance_id", instanceID,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnImplementerStart != nil {
		cb.OnImplementerStart(round, instanceID)
	}
}

// notifyIncrementReady notifies callbacks of increment completion
func (c *AdversarialCoordinator) notifyIncrementReady(round int, increment *AdversarialIncrementFile) {
	c.manager.RecordIncrement(increment)

	// Persist increment
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist increment", "round", round, "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnIncrementReady != nil {
		cb.OnIncrementReady(round, increment)
	}
}

// notifyReviewerStart notifies callbacks of reviewer start
func (c *AdversarialCoordinator) notifyReviewerStart(round int, instanceID string) {
	c.logger.Info("reviewer started",
		"round", round,
		"instance_id", instanceID,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnReviewerStart != nil {
		cb.OnReviewerStart(round, instanceID)
	}
}

// notifyReviewReady notifies callbacks of review completion
func (c *AdversarialCoordinator) notifyReviewReady(round int, review *AdversarialReviewFile) {
	c.manager.RecordReview(review)

	// Persist review
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist review", "round", round, "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnReviewReady != nil {
		cb.OnReviewReady(round, review)
	}

	// Also notify approved/rejected
	if review.Approved {
		if cb != nil && cb.OnApproved != nil {
			cb.OnApproved(round, review)
		}
	} else {
		if cb != nil && cb.OnRejected != nil {
			cb.OnRejected(round, review)
		}
	}
}

// notifyComplete notifies callbacks of completion
func (c *AdversarialCoordinator) notifyComplete(success bool, summary string) {
	c.logger.Info("adversarial session complete",
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

// StartImplementer starts the implementer instance for the current round
func (c *AdversarialCoordinator) StartImplementer() error {
	session := c.Session()
	task := session.Task
	round := session.CurrentRound

	c.logger.Info("starting implementer",
		"task", util.TruncateString(task, 100),
		"round", round,
	)

	// Initialize timing
	if session.StartedAt == nil {
		now := time.Now()
		session.StartedAt = &now
	}

	// Get previous review feedback if this isn't the first round
	// Must be done BEFORE StartRound() which appends a new round to History
	var previousReview *AdversarialReviewFile
	if round > 1 && len(session.History) > 0 {
		previousReview = session.History[len(session.History)-1].Review
	}

	// Start a new round
	c.manager.StartRound()

	// Build the implementer prompt
	prompt := FormatAdversarialImplementerPrompt(task, round, previousReview)

	// Find the adversarial group to add instances to
	var advGroup *InstanceGroup
	if session.GroupID != "" {
		advGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if advGroup == nil {
		advGroup = c.baseSession.GetGroupBySessionType(SessionTypeAdversarial)
	}

	// Create instance for implementer
	var inst *Instance
	var err error

	if round == 1 {
		// First round - create a fresh worktree
		inst, err = c.orch.AddInstance(c.baseSession, prompt)
		if err != nil {
			return fmt.Errorf("failed to create implementer instance: %w", err)
		}
		c.implementerWorktree = inst.WorktreePath
		c.reviewerWorktree = inst.WorktreePath // Reviewer will use same worktree
	} else {
		// Subsequent rounds - reuse the same worktree
		inst, err = c.orch.AddInstanceToWorktree(c.baseSession, prompt, c.implementerWorktree, "")
		if err != nil {
			return fmt.Errorf("failed to create implementer instance for round %d: %w", round, err)
		}
	}

	// Add instance to the adversarial group for sidebar display
	if advGroup != nil {
		advGroup.AddInstance(inst.ID)
	}

	// Store implementer ID
	session.ImplementerID = inst.ID

	// Transition to implementing phase
	c.notifyPhaseChange(PhaseAdversarialImplementing)

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start implementer: %w", err)
	}

	c.notifyImplementerStart(round, inst.ID)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting implementer", "error", err)
	}

	return nil
}

// StartReviewer starts the reviewer instance for the current round
func (c *AdversarialCoordinator) StartReviewer(increment *AdversarialIncrementFile) error {
	session := c.Session()
	task := session.Task
	round := session.CurrentRound

	c.logger.Info("starting reviewer", "round", round)

	// Build the reviewer prompt with configurable minimum passing score
	minPassingScore := session.Config.MinPassingScore
	if minPassingScore < 1 || minPassingScore > 10 {
		minPassingScore = 8 // Fallback to default if invalid
	}
	prompt := FormatAdversarialReviewerPrompt(task, round, increment, minPassingScore)

	// Find the adversarial group
	var advGroup *InstanceGroup
	if session.GroupID != "" {
		advGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if advGroup == nil {
		advGroup = c.baseSession.GetGroupBySessionType(SessionTypeAdversarial)
	}

	// Create reviewer instance in the same worktree
	inst, err := c.orch.AddInstanceToWorktree(c.baseSession, prompt, c.reviewerWorktree, "")
	if err != nil {
		return fmt.Errorf("failed to create reviewer instance: %w", err)
	}

	// Add instance to the adversarial group
	if advGroup != nil {
		advGroup.AddInstance(inst.ID)
	}

	// Store reviewer ID
	session.ReviewerID = inst.ID

	// Transition to reviewing phase
	c.notifyPhaseChange(PhaseAdversarialReviewing)

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start reviewer: %w", err)
	}

	c.notifyReviewerStart(round, inst.ID)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting reviewer", "error", err)
	}

	return nil
}

// CheckIncrementReady checks if the implementer has written their increment file
func (c *AdversarialCoordinator) CheckIncrementReady() (bool, error) {
	if c.implementerWorktree == "" {
		return false, nil
	}

	incrementPath := AdversarialIncrementFilePath(c.implementerWorktree)
	_, err := os.Stat(incrementPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check increment file: %w", err)
}

// CheckReviewReady checks if the reviewer has written their review file
func (c *AdversarialCoordinator) CheckReviewReady() (bool, error) {
	if c.reviewerWorktree == "" {
		return false, nil
	}

	reviewPath := AdversarialReviewFilePath(c.reviewerWorktree)
	_, err := os.Stat(reviewPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check review file: %w", err)
}

// ProcessIncrementCompletion handles when the implementer completes
func (c *AdversarialCoordinator) ProcessIncrementCompletion() error {
	session := c.Session()
	round := session.CurrentRound

	// Parse the increment file
	increment, err := ParseAdversarialIncrementFile(c.implementerWorktree)
	if err != nil {
		c.notifyPhaseChange(PhaseAdversarialFailed)
		session.Error = fmt.Sprintf("failed to parse increment file: %v", err)
		return err
	}

	// Validate round number
	if increment.Round != round {
		c.logger.Warn("increment round mismatch",
			"expected", round,
			"got", increment.Round,
		)
	}

	// Check for failure status
	if increment.Status == "failed" {
		c.notifyPhaseChange(PhaseAdversarialFailed)
		session.Error = fmt.Sprintf("implementer failed: %s", increment.Summary)
		return fmt.Errorf("implementer reported failure: %s", increment.Summary)
	}

	c.notifyIncrementReady(round, increment)

	// Clear the increment file so it's fresh for next round
	if err := os.Remove(AdversarialIncrementFilePath(c.implementerWorktree)); err != nil && !os.IsNotExist(err) {
		c.logger.Warn("failed to remove increment file", "error", err)
	}

	// Start the reviewer
	return c.StartReviewer(increment)
}

// ProcessReviewCompletion handles when the reviewer completes
func (c *AdversarialCoordinator) ProcessReviewCompletion() error {
	session := c.Session()
	round := session.CurrentRound

	// Parse the review file
	review, err := ParseAdversarialReviewFile(c.reviewerWorktree)
	if err != nil {
		c.notifyPhaseChange(PhaseAdversarialFailed)
		session.Error = fmt.Sprintf("failed to parse review file: %v", err)
		return err
	}

	// Validate round number
	if review.Round != round {
		c.logger.Warn("review round mismatch",
			"expected", round,
			"got", review.Round,
		)
	}

	c.notifyReviewReady(round, review)

	// Clear the review file so it's fresh for next round
	if err := os.Remove(AdversarialReviewFilePath(c.reviewerWorktree)); err != nil && !os.IsNotExist(err) {
		c.logger.Warn("failed to remove review file", "error", err)
	}

	// Enforce score/approval consistency: if approved but score < minPassingScore, treat as rejection
	minScore := session.Config.MinPassingScore
	if minScore < 1 || minScore > 10 {
		minScore = 8 // Fallback to default
	}
	if review.Approved && review.Score < minScore {
		c.logger.Warn("review marked approved but score below minimum",
			"score", review.Score,
			"min_passing_score", minScore,
			"overriding_to", "rejected",
		)
		review.Approved = false
		if len(review.RequiredChanges) == 0 {
			review.RequiredChanges = []string{
				fmt.Sprintf("Score of %d is below the minimum passing score of %d", review.Score, minScore),
			}
		}
	}

	if review.Approved {
		// Work approved - complete the session
		now := time.Now()
		session.CompletedAt = &now
		c.notifyPhaseChange(PhaseAdversarialApproved)
		c.notifyPhaseChange(PhaseAdversarialComplete)

		summary := fmt.Sprintf("Approved after %d round(s). Final score: %d/10. %s",
			round, review.Score, review.Summary)
		c.notifyComplete(true, summary)
	} else {
		// Work rejected - check if we should continue
		if c.manager.IsMaxIterationsReached() {
			c.notifyPhaseChange(PhaseAdversarialFailed)
			session.Error = fmt.Sprintf("max iterations reached (%d) without approval", session.Config.MaxIterations)
			c.notifyComplete(false, session.Error)
			return fmt.Errorf("%s", session.Error)
		}

		// Advance to next round
		c.manager.NextRound()

		// Start implementer again with the review feedback
		return c.StartImplementer()
	}

	return nil
}

// Stop stops the adversarial execution
func (c *AdversarialCoordinator) Stop() {
	c.cancelFunc()
	c.wg.Wait()
}

// GetImplementerWorktree returns the implementer's worktree path
func (c *AdversarialCoordinator) GetImplementerWorktree() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.implementerWorktree
}

// SetWorktrees sets the worktree paths (used when restoring session)
func (c *AdversarialCoordinator) SetWorktrees(worktree string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.implementerWorktree = worktree
	c.reviewerWorktree = worktree
}

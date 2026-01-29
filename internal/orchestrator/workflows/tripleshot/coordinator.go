package tripleshot

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/util"
)

// OrchestratorInterface defines the methods needed from the Orchestrator.
// This interface allows the coordinator to work without importing the orchestrator package directly.
type OrchestratorInterface interface {
	AddInstance(session SessionInterface, task string) (InstanceInterface, error)
	AddInstanceToWorktree(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error)
	StartInstance(inst InstanceInterface) error
	SaveSession() error
	// Async stub methods for responsive UI during worktree creation
	AddInstanceStub(session SessionInterface, task string) (InstanceInterface, error)
	CompleteInstanceSetupByID(session SessionInterface, instanceID string) error
}

// SessionInterface defines the methods needed from the Session.
type SessionInterface interface {
	GetGroup(id string) GroupInterface
	GetGroupBySessionType(sessionType string) GroupInterface
	GetInstance(id string) InstanceInterface
}

// InstanceInterface defines the methods needed from an Instance.
type InstanceInterface interface {
	GetID() string
	GetWorktreePath() string
	GetBranch() string
}

// GroupInterface defines the methods needed from an InstanceGroup.
type GroupInterface interface {
	AddInstance(instanceID string)
	AddSubGroup(subGroup GroupInterface)
	GetInstances() []string
	SetInstances(instances []string)
	GetID() string
}

// NewGroupFunc is a function type for creating new instance groups.
type NewGroupFunc func(name string) GroupInterface

// SetSessionTypeFunc is a function type for setting session type on a group.
type SetSessionTypeFunc func(g GroupInterface, sessionType string)

// CoordinatorCallbacks holds callbacks for coordinator events
type CoordinatorCallbacks struct {
	// OnPhaseChange is called when the triple-shot phase changes
	OnPhaseChange func(phase Phase)

	// OnAttemptStart is called when an attempt begins
	OnAttemptStart func(attemptIndex int, instanceID string)

	// OnAttemptComplete is called when an attempt completes
	OnAttemptComplete func(attemptIndex int)

	// OnAttemptFailed is called when an attempt fails
	OnAttemptFailed func(attemptIndex int, reason string)

	// OnJudgeStart is called when the judge starts evaluation
	OnJudgeStart func(instanceID string)

	// OnEvaluationReady is called when evaluation is complete
	OnEvaluationReady func(evaluation *Evaluation)

	// OnComplete is called when the entire triple-shot completes
	OnComplete func(success bool, summary string)

	// Adversarial review callbacks
	// OnReviewerStart is called when an adversarial reviewer starts
	OnReviewerStart func(attemptIndex int, instanceID string)

	// OnReviewApproved is called when a reviewer approves an attempt
	OnReviewApproved func(attemptIndex int, score int)

	// OnReviewRejected is called when a reviewer rejects an attempt
	OnReviewRejected func(attemptIndex int, score int, issues []string)
}

// Coordinator orchestrates the execution of a triple-shot session
type Coordinator struct {
	manager     *Manager
	orch        OrchestratorInterface
	baseSession SessionInterface
	callbacks   *CoordinatorCallbacks
	logger      *logging.Logger

	// Session type constant for this workflow
	sessionType string

	// Function to create new groups
	newGroup NewGroupFunc

	// Function to set session type on a group
	setSessionType SetSessionTypeFunc

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Tracking
	runningAttempts map[int]string // attemptIndex -> instanceID
}

// CoordinatorConfig holds configuration for creating a Coordinator
type CoordinatorConfig struct {
	Orchestrator   OrchestratorInterface
	BaseSession    SessionInterface
	TripleSession  *Session
	Logger         *logging.Logger
	SessionType    string
	NewGroup       NewGroupFunc
	SetSessionType SetSessionTypeFunc
}

// NewCoordinator creates a new coordinator for a triple-shot session
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	logger := cfg.Logger
	if logger == nil {
		logger = logging.NopLogger()
	}
	manager := NewManager(cfg.TripleSession, logger)

	ctx, cancel := context.WithCancel(context.Background())

	sessionLogger := logger.WithSession(cfg.TripleSession.ID).WithPhase("tripleshot-coordinator")

	return &Coordinator{
		manager:         manager,
		orch:            cfg.Orchestrator,
		baseSession:     cfg.BaseSession,
		logger:          sessionLogger,
		sessionType:     cfg.SessionType,
		newGroup:        cfg.NewGroup,
		setSessionType:  cfg.SetSessionType,
		ctx:             ctx,
		cancelFunc:      cancel,
		runningAttempts: make(map[int]string),
	}
}

// SetCallbacks sets the coordinator callbacks
func (c *Coordinator) SetCallbacks(cb *CoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Manager returns the underlying triple-shot manager
func (c *Coordinator) Manager() *Manager {
	return c.manager
}

// Session returns the triple-shot session
func (c *Coordinator) Session() *Session {
	return c.manager.Session()
}

// notifyPhaseChange notifies callbacks of phase change
func (c *Coordinator) notifyPhaseChange(phase Phase) {
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

// findTripleGroup returns the instance group for this triple-shot session.
// It first checks GroupID for multi-tripleshot support, then falls back
// to finding the group by session type for backward compatibility.
func (c *Coordinator) findTripleGroup() GroupInterface {
	session := c.Session()
	if session.GroupID != "" {
		if group := c.baseSession.GetGroup(session.GroupID); group != nil {
			return group
		}
	}
	return c.baseSession.GetGroupBySessionType(c.sessionType)
}

// notifyAttemptStart notifies callbacks of attempt start
func (c *Coordinator) notifyAttemptStart(attemptIndex int, instanceID string) {
	c.logger.Info("attempt started",
		"attempt_index", attemptIndex,
		"instance_id", instanceID,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnAttemptStart != nil {
		cb.OnAttemptStart(attemptIndex, instanceID)
	}
}

// notifyAttemptComplete notifies callbacks of attempt completion
func (c *Coordinator) notifyAttemptComplete(attemptIndex int) {
	c.manager.MarkAttemptComplete(attemptIndex)

	// Persist completion
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist attempt completion", "attempt_index", attemptIndex, "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnAttemptComplete != nil {
		cb.OnAttemptComplete(attemptIndex)
	}
}

// notifyAttemptFailed notifies callbacks of attempt failure
func (c *Coordinator) notifyAttemptFailed(attemptIndex int, reason string) {
	c.manager.MarkAttemptFailed(attemptIndex, reason)

	// Persist failure
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist attempt failure", "attempt_index", attemptIndex, "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnAttemptFailed != nil {
		cb.OnAttemptFailed(attemptIndex, reason)
	}
}

// notifyJudgeStart notifies callbacks of judge start
func (c *Coordinator) notifyJudgeStart(instanceID string) {
	c.logger.Info("judge started", "instance_id", instanceID)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnJudgeStart != nil {
		cb.OnJudgeStart(instanceID)
	}
}

// notifyEvaluationReady notifies callbacks of evaluation completion
func (c *Coordinator) notifyEvaluationReady(evaluation *Evaluation) {
	c.manager.SetEvaluation(evaluation)

	// Persist evaluation
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist evaluation", "error", err)
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnEvaluationReady != nil {
		cb.OnEvaluationReady(evaluation)
	}
}

// notifyComplete notifies callbacks of completion
func (c *Coordinator) notifyComplete(success bool, summary string) {
	c.logger.Info("triple-shot complete",
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

// StartAttempts starts all three attempt instances in parallel
func (c *Coordinator) StartAttempts() error {
	session := c.Session()
	task := session.Task

	c.logger.Info("starting triple-shot attempts", "task", util.TruncateString(task, 100))

	now := time.Now()
	session.StartedAt = &now

	tripleGroup := c.findTripleGroup()

	// Create and start all three attempts
	for i := range 3 {
		prompt := fmt.Sprintf(AttemptPromptTemplate, task, i)

		// Create instance for this attempt
		inst, err := c.orch.AddInstance(c.baseSession, prompt)
		if err != nil {
			return fmt.Errorf("failed to create attempt %d instance: %w", i, err)
		}

		// Add instance to the triple-shot group for sidebar display
		if tripleGroup != nil {
			tripleGroup.AddInstance(inst.GetID())
		}

		// Record attempt info
		session.Attempts[i] = Attempt{
			InstanceID:   inst.GetID(),
			WorktreePath: inst.GetWorktreePath(),
			Branch:       inst.GetBranch(),
			Status:       AttemptStatusWorking,
			StartedAt:    &now,
		}

		c.mu.Lock()
		c.runningAttempts[i] = inst.GetID()
		c.mu.Unlock()

		// Start the instance
		if err := c.orch.StartInstance(inst); err != nil {
			return fmt.Errorf("failed to start attempt %d: %w", i, err)
		}

		c.notifyAttemptStart(i, inst.GetID())
	}

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting attempts", "error", err)
	}

	return nil
}

// StartJudge starts the judge instance to evaluate the three attempts
func (c *Coordinator) StartJudge() error {
	session := c.Session()

	// Build the completion summaries for each attempt
	var summaries [3]string
	for i, attempt := range session.Attempts {
		completion, err := ParseCompletionFile(attempt.WorktreePath)
		if err != nil {
			c.logger.Warn("failed to read completion file for attempt",
				"attempt_index", i,
				"worktree_path", attempt.WorktreePath,
				"error", err,
			)
			summaries[i] = fmt.Sprintf("(Unable to read completion file: %v)", err)
		} else {
			summaries[i] = fmt.Sprintf("Status: %s\nSummary: %s\nApproach: %s\nFiles Modified: %v",
				completion.Status, completion.Summary, completion.Approach, completion.FilesModified)
		}
	}

	// Build the judge prompt
	prompt := fmt.Sprintf(JudgePromptTemplate,
		session.Task,
		// Attempt 1
		session.Attempts[0].InstanceID, session.Attempts[0].Branch, session.Attempts[0].WorktreePath, summaries[0],
		// Attempt 2
		session.Attempts[1].InstanceID, session.Attempts[1].Branch, session.Attempts[1].WorktreePath, summaries[1],
		// Attempt 3
		session.Attempts[2].InstanceID, session.Attempts[2].Branch, session.Attempts[2].WorktreePath, summaries[2],
	)

	// Create judge instance
	inst, err := c.orch.AddInstance(c.baseSession, prompt)
	if err != nil {
		return fmt.Errorf("failed to create judge instance: %w", err)
	}

	// Reorganize the triple-shot group: move implementers to a sub-group, add judge to parent
	tripleGroup := c.findTripleGroup()
	if tripleGroup != nil && c.newGroup != nil {
		// Create a sub-group for the implementers
		implementersGroup := c.newGroup("Implementers")
		if c.setSessionType != nil {
			c.setSessionType(implementersGroup, c.sessionType)
		}

		// Move existing instances (the 3 implementers) to the sub-group
		for _, instID := range tripleGroup.GetInstances() {
			implementersGroup.AddInstance(instID)
		}

		// Clear the parent group's instances and add the sub-group
		tripleGroup.SetInstances(nil)
		tripleGroup.AddSubGroup(implementersGroup)

		// Store the implementers group ID for TUI collapse behavior
		session.ImplementersGroupID = implementersGroup.GetID()

		// Add the judge to the parent group (so it appears at the top level)
		tripleGroup.AddInstance(inst.GetID())
	} else {
		c.logger.Warn("triple-shot group not found, judge will not be grouped with implementers",
			"session_id", session.ID,
			"group_id", session.GroupID,
		)
	}

	session.JudgeID = inst.GetID()

	// Transition to evaluating phase
	c.notifyPhaseChange(PhaseEvaluating)

	// Start the judge instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start judge instance: %w", err)
	}

	c.notifyJudgeStart(inst.GetID())

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting judge", "error", err)
	}

	return nil
}

// CheckAttemptCompletion checks if an attempt has written its completion file.
// It searches both the worktree root and immediate subdirectories to handle
// cases where Claude instances write the file from a subdirectory.
func (c *Coordinator) CheckAttemptCompletion(attemptIndex int) (bool, error) {
	session := c.Session()
	if attemptIndex < 0 || attemptIndex >= 3 {
		return false, fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	attempt := session.Attempts[attemptIndex]
	if attempt.WorktreePath == "" {
		return false, nil
	}

	return CompletionFileExists(attempt.WorktreePath), nil
}

// CheckJudgeCompletion checks if the judge has written its evaluation file.
// It searches both the worktree root and immediate subdirectories to handle
// cases where the judge writes the file from a subdirectory.
func (c *Coordinator) CheckJudgeCompletion() (bool, error) {
	session := c.Session()
	if session.JudgeID == "" {
		return false, nil
	}

	// Find the judge instance to get its worktree path
	judgeInst := c.baseSession.GetInstance(session.JudgeID)
	if judgeInst == nil {
		return false, fmt.Errorf("judge instance %s not found", session.JudgeID)
	}

	return EvaluationFileExists(judgeInst.GetWorktreePath()), nil
}

// ProcessAttemptCompletion handles when an attempt completes
func (c *Coordinator) ProcessAttemptCompletion(attemptIndex int) error {
	if attemptIndex < 0 || attemptIndex >= 3 {
		return fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	session := c.Session()
	attempt := &session.Attempts[attemptIndex]

	// Parse the completion file
	completion, err := ParseCompletionFile(attempt.WorktreePath)
	if err != nil {
		c.notifyAttemptFailed(attemptIndex, fmt.Sprintf("failed to parse completion file: %v", err))
		return err
	}

	if completion.Status != "complete" {
		c.notifyAttemptFailed(attemptIndex, completion.Summary)
		return c.checkAllAttemptsAndProceed()
	}

	// If adversarial mode is enabled, spawn a reviewer instead of marking complete
	if session.Config.Adversarial {
		c.logger.Info("adversarial mode enabled, spawning reviewer",
			"attempt_index", attemptIndex,
			"worktree_path", attempt.WorktreePath,
		)

		// Mark attempt as under review
		attempt.Status = AttemptStatusUnderReview
		attempt.ReviewRound = 1

		// Transition to adversarial review phase if this is the first attempt under review
		if session.Phase == PhaseWorking {
			c.notifyPhaseChange(PhaseAdversarialReview)
		}

		// Start the adversarial reviewer
		if err := c.StartAdversarialReviewer(attemptIndex, completion); err != nil {
			c.logger.Error("failed to start adversarial reviewer",
				"attempt_index", attemptIndex,
				"error", err,
			)
			c.notifyAttemptFailed(attemptIndex, fmt.Sprintf("failed to start reviewer: %v", err))
			return err
		}
		return nil
	}

	// Non-adversarial mode: mark complete immediately
	c.notifyAttemptComplete(attemptIndex)
	return c.checkAllAttemptsAndProceed()
}

// checkAllAttemptsAndProceed checks if all attempts are complete and proceeds to judge if ready.
// Returns an error if all attempts are complete but fewer than 2 succeeded.
func (c *Coordinator) checkAllAttemptsAndProceed() error {
	session := c.Session()

	// Check if all attempts are complete (including adversarial review approval)
	if session.AllAttemptsComplete() {
		c.logger.Info("all attempts complete", "successful_count", session.SuccessfulAttemptCount())

		// Only proceed to judge if we have at least 2 successful attempts
		if session.SuccessfulAttemptCount() >= 2 {
			c.manager.EmitAllAttemptsReady()
		} else {
			c.notifyPhaseChange(PhaseFailed)
			session.Error = "Fewer than 2 attempts succeeded"
			return fmt.Errorf("fewer than 2 attempts succeeded")
		}
	}
	return nil
}

// StartAdversarialReviewer starts a reviewer instance for an attempt
func (c *Coordinator) StartAdversarialReviewer(attemptIndex int, completion *CompletionFile) error {
	session := c.Session()
	attempt := &session.Attempts[attemptIndex]

	// Use configured minimum passing score, or default to 8
	minPassingScore := session.Config.MinPassingScore
	if minPassingScore <= 0 {
		minPassingScore = 8
	}

	// Build the reviewer prompt
	prompt := FormatAdversarialReviewerPrompt(
		session.Task,
		attemptIndex,
		attempt.ReviewRound,
		completion,
		minPassingScore,
	)

	// Create reviewer instance in the same worktree
	inst, err := c.orch.AddInstanceToWorktree(c.baseSession, prompt, attempt.WorktreePath, "")
	if err != nil {
		return fmt.Errorf("failed to create reviewer instance: %w", err)
	}

	// Store reviewer ID
	attempt.ReviewerID = inst.GetID()

	// Add reviewer to the triple-shot group
	if tripleGroup := c.findTripleGroup(); tripleGroup != nil {
		tripleGroup.AddInstance(inst.GetID())
	}

	// Start the reviewer instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start reviewer: %w", err)
	}

	c.notifyReviewerStart(attemptIndex, inst.GetID())

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting reviewer", "error", err)
	}

	return nil
}

// notifyReviewerStart notifies callbacks of reviewer start
func (c *Coordinator) notifyReviewerStart(attemptIndex int, instanceID string) {
	c.logger.Info("adversarial reviewer started",
		"attempt_index", attemptIndex,
		"instance_id", instanceID,
	)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnReviewerStart != nil {
		cb.OnReviewerStart(attemptIndex, instanceID)
	}
}

// CheckAdversarialReviewCompletion checks if a reviewer has written its review file
func (c *Coordinator) CheckAdversarialReviewCompletion(attemptIndex int) (bool, error) {
	session := c.Session()
	if attemptIndex < 0 || attemptIndex >= 3 {
		return false, fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	attempt := session.Attempts[attemptIndex]
	if attempt.Status != AttemptStatusUnderReview {
		return false, nil
	}
	if attempt.WorktreePath == "" {
		return false, nil
	}

	return AdversarialReviewFileExists(attempt.WorktreePath), nil
}

// ProcessAdversarialReviewCompletion handles when a reviewer completes
func (c *Coordinator) ProcessAdversarialReviewCompletion(attemptIndex int) error {
	if attemptIndex < 0 || attemptIndex >= 3 {
		return fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	session := c.Session()
	attempt := &session.Attempts[attemptIndex]

	// Parse the review file
	review, err := ParseAdversarialReviewFile(attempt.WorktreePath)
	if err != nil {
		c.logger.Error("failed to parse adversarial review file",
			"attempt_index", attemptIndex,
			"error", err,
		)
		c.notifyAttemptFailed(attemptIndex, fmt.Sprintf("failed to parse review file: %v", err))
		return err
	}

	// Clean up review file when done (regardless of outcome)
	defer func() {
		if err := removeFile(AdversarialReviewFilePath(attempt.WorktreePath)); err != nil {
			c.logger.Warn("failed to clean up adversarial review file",
				"attempt_index", attemptIndex,
				"worktree_path", attempt.WorktreePath,
				"error", err,
			)
		}
	}()

	// Store review results
	attempt.ReviewScore = review.Score
	attempt.ReviewApproved = review.Approved

	if review.Approved {
		c.logger.Info("adversarial review approved",
			"attempt_index", attemptIndex,
			"score", review.Score,
		)
		c.notifyAttemptComplete(attemptIndex)
		c.notifyReviewApproved(attemptIndex, review.Score)
		return c.checkAllAttemptsAndProceed()
	}

	// Review rejected
	c.logger.Info("adversarial review rejected",
		"attempt_index", attemptIndex,
		"score", review.Score,
		"issues_count", len(review.Issues),
	)

	maxRounds := session.Config.MaxAdversarialRounds
	if maxRounds <= 0 {
		maxRounds = 3 // Fallback default
	}

	if attempt.ReviewRound >= maxRounds {
		// Exhausted iterations - permanent failure
		c.notifyAttemptFailed(attemptIndex,
			fmt.Sprintf("Exhausted %d adversarial rounds without approval (final score: %d/10)",
				maxRounds, review.Score))
		c.notifyReviewRejected(attemptIndex, review.Score, review.Issues)
		return c.checkAllAttemptsAndProceed()
	}

	// Restart implementer with feedback
	c.notifyReviewRejected(attemptIndex, review.Score, review.Issues)
	if err := c.RestartImplementerWithFeedback(attemptIndex, review); err != nil {
		c.logger.Error("failed to restart implementer with feedback",
			"attempt_index", attemptIndex,
			"error", err,
		)
		c.notifyAttemptFailed(attemptIndex, fmt.Sprintf("Failed to restart implementer: %v", err))
	}
	// Don't check for judge yet - implementer is restarting
	return nil
}

// notifyReviewApproved notifies callbacks of review approval
func (c *Coordinator) notifyReviewApproved(attemptIndex int, score int) {
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnReviewApproved != nil {
		cb.OnReviewApproved(attemptIndex, score)
	}
}

// notifyReviewRejected notifies callbacks of review rejection
func (c *Coordinator) notifyReviewRejected(attemptIndex int, score int, issues []string) {
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnReviewRejected != nil {
		cb.OnReviewRejected(attemptIndex, score, issues)
	}
}

// RestartImplementerWithFeedback restarts an implementer instance with feedback from the previous review.
// This is called when a reviewer rejects an attempt and we haven't exhausted max rounds yet.
func (c *Coordinator) RestartImplementerWithFeedback(attemptIndex int, review *AdversarialReviewFile) error {
	session := c.Session()
	attempt := &session.Attempts[attemptIndex]

	// Increment review round
	attempt.ReviewRound++

	c.logger.Info("restarting implementer with feedback",
		"attempt_index", attemptIndex,
		"new_round", attempt.ReviewRound,
		"previous_score", review.Score,
	)

	// Set status back to working
	attempt.Status = AttemptStatusWorking

	// Build prompt with feedback
	prompt := FormatImplementerPromptWithFeedback(session.Task, attemptIndex, attempt.ReviewRound, review)

	// Delete the old completion file so we can detect the new one
	if err := removeFile(CompletionFilePath(attempt.WorktreePath)); err != nil {
		c.logger.Warn("failed to remove old completion file",
			"attempt_index", attemptIndex,
			"error", err,
		)
	}

	// Create new instance in the same worktree
	inst, err := c.orch.AddInstanceToWorktree(c.baseSession, prompt, attempt.WorktreePath, "")
	if err != nil {
		return fmt.Errorf("failed to create new implementer instance: %w", err)
	}

	// Update attempt with new instance ID
	oldInstanceID := attempt.InstanceID
	attempt.InstanceID = inst.GetID()
	attempt.ReviewerID = "" // Clear the old reviewer ID

	// Add new instance to the triple-shot group
	if tripleGroup := c.findTripleGroup(); tripleGroup != nil {
		tripleGroup.AddInstance(inst.GetID())
	}

	// Start the new instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start new implementer: %w", err)
	}

	c.logger.Info("implementer restarted with feedback",
		"attempt_index", attemptIndex,
		"old_instance_id", oldInstanceID,
		"new_instance_id", inst.GetID(),
		"round", attempt.ReviewRound,
	)

	c.notifyAttemptStart(attemptIndex, inst.GetID())

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after restarting implementer", "error", err)
	}

	return nil
}

// removeFile is a helper to remove a file, ignoring not-exist errors
func removeFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ProcessJudgeCompletion handles when the judge completes
func (c *Coordinator) ProcessJudgeCompletion() error {
	session := c.Session()

	// Find the judge instance
	judgeInst := c.baseSession.GetInstance(session.JudgeID)
	if judgeInst == nil {
		return fmt.Errorf("judge instance not found")
	}

	// Parse the evaluation file
	evaluation, err := ParseEvaluationFile(judgeInst.GetWorktreePath())
	if err != nil {
		c.notifyPhaseChange(PhaseFailed)
		session.Error = fmt.Sprintf("failed to parse evaluation: %v", err)
		return err
	}

	c.notifyEvaluationReady(evaluation)

	// Transition to complete phase
	now := time.Now()
	session.CompletedAt = &now
	c.notifyPhaseChange(PhaseComplete)

	// Build summary
	var summary string
	if evaluation.MergeStrategy == MergeStrategySelect {
		if evaluation.WinnerIndex < 0 || evaluation.WinnerIndex >= 3 {
			c.notifyPhaseChange(PhaseFailed)
			session.Error = fmt.Sprintf("invalid winner index: %d", evaluation.WinnerIndex)
			return fmt.Errorf("invalid winner index in evaluation: %d", evaluation.WinnerIndex)
		}
		winnerAttempt := session.Attempts[evaluation.WinnerIndex]
		summary = fmt.Sprintf("Selected attempt %d (branch: %s). Reasoning: %s",
			evaluation.WinnerIndex+1, winnerAttempt.Branch, evaluation.Reasoning)
	} else {
		summary = fmt.Sprintf("Strategy: %s. Reasoning: %s", evaluation.MergeStrategy, evaluation.Reasoning)
	}

	c.notifyComplete(true, summary)

	return nil
}

// CreateAttemptStubs creates stub instances for all three attempts immediately.
// This is the fast first phase of async tripleshot startup - it creates instance
// metadata without blocking on worktree creation. Returns the instance IDs.
// The UI can show these instances in "preparing" status while worktrees are created.
func (c *Coordinator) CreateAttemptStubs() ([3]string, error) {
	session := c.Session()
	task := session.Task

	c.logger.Info("creating triple-shot attempt stubs", "task", util.TruncateString(task, 100))

	now := time.Now()
	session.StartedAt = &now

	tripleGroup := c.findTripleGroup()

	var instanceIDs [3]string

	// Create stub instances for all three attempts
	for i := range 3 {
		prompt := fmt.Sprintf(AttemptPromptTemplate, task, i)

		// Create stub instance (fast - no worktree yet)
		inst, err := c.orch.AddInstanceStub(c.baseSession, prompt)
		if err != nil {
			return instanceIDs, fmt.Errorf("failed to create attempt %d stub: %w", i, err)
		}

		instanceIDs[i] = inst.GetID()

		// Add instance to the triple-shot group for sidebar display
		if tripleGroup != nil {
			tripleGroup.AddInstance(inst.GetID())
		}

		// Record attempt info with preparing status
		session.Attempts[i] = Attempt{
			InstanceID: inst.GetID(),
			Status:     AttemptStatusPreparing,
			StartedAt:  &now,
		}

		c.mu.Lock()
		c.runningAttempts[i] = inst.GetID()
		c.mu.Unlock()
	}

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after creating stubs", "error", err)
	}

	return instanceIDs, nil
}

// CompleteAttemptSetup finishes the setup for a single attempt by creating its worktree
// and starting the instance. This is the slow second phase that should be called from
// a goroutine for each attempt in parallel.
func (c *Coordinator) CompleteAttemptSetup(attemptIndex int) error {
	if attemptIndex < 0 || attemptIndex >= 3 {
		return fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	session := c.Session()
	attempt := &session.Attempts[attemptIndex]

	c.logger.Info("completing attempt setup",
		"attempt_index", attemptIndex,
		"instance_id", attempt.InstanceID,
	)

	// markFailed is a helper to set failure status and persist
	markFailed := func() {
		attempt.Status = AttemptStatusFailed
		if saveErr := c.orch.SaveSession(); saveErr != nil {
			c.logger.Error("failed to persist attempt failure",
				"attempt_index", attemptIndex,
				"error", saveErr,
			)
		}
	}

	// Complete the instance setup (creates worktree - slow)
	if err := c.orch.CompleteInstanceSetupByID(c.baseSession, attempt.InstanceID); err != nil {
		markFailed()
		return fmt.Errorf("failed to complete setup for attempt %d: %w", attemptIndex, err)
	}

	// Get the updated instance info to retrieve worktree path and branch
	inst := c.baseSession.GetInstance(attempt.InstanceID)
	if inst == nil {
		markFailed()
		return fmt.Errorf("instance %s not found after setup", attempt.InstanceID)
	}

	// Update attempt with worktree info
	attempt.WorktreePath = inst.GetWorktreePath()
	attempt.Branch = inst.GetBranch()
	attempt.Status = AttemptStatusWorking

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		markFailed()
		return fmt.Errorf("failed to start attempt %d: %w", attemptIndex, err)
	}

	c.notifyAttemptStart(attemptIndex, attempt.InstanceID)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after completing attempt setup",
			"attempt_index", attemptIndex,
			"error", err,
		)
	}

	return nil
}

// AllAttemptsReady returns true when all three attempts have completed their setup
// (moved from preparing to working or failed status).
func (c *Coordinator) AllAttemptsReady() bool {
	session := c.Session()
	for _, attempt := range session.Attempts {
		if attempt.Status == AttemptStatusPreparing || attempt.Status == AttemptStatusPending {
			return false
		}
	}
	return true
}

// Stop stops the triple-shot execution
func (c *Coordinator) Stop() {
	c.cancelFunc()
	c.wg.Wait()
}

// GetAttemptInstanceID returns the instance ID for an attempt
func (c *Coordinator) GetAttemptInstanceID(attemptIndex int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runningAttempts[attemptIndex]
}

// GetWinningBranch returns the branch name of the winning solution
// Returns empty string if evaluation is not complete or strategy is merge
func (c *Coordinator) GetWinningBranch() string {
	session := c.Session()
	if session.Evaluation == nil {
		return ""
	}
	if session.Evaluation.MergeStrategy != MergeStrategySelect {
		return ""
	}
	if session.Evaluation.WinnerIndex < 0 || session.Evaluation.WinnerIndex >= 3 {
		return ""
	}
	return session.Attempts[session.Evaluation.WinnerIndex].Branch
}

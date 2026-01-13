package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// TripleShotCoordinatorCallbacks holds callbacks for coordinator events
type TripleShotCoordinatorCallbacks struct {
	// OnPhaseChange is called when the triple-shot phase changes
	OnPhaseChange func(phase TripleShotPhase)

	// OnAttemptStart is called when an attempt begins
	OnAttemptStart func(attemptIndex int, instanceID string)

	// OnAttemptComplete is called when an attempt completes
	OnAttemptComplete func(attemptIndex int)

	// OnAttemptFailed is called when an attempt fails
	OnAttemptFailed func(attemptIndex int, reason string)

	// OnJudgeStart is called when the judge starts evaluation
	OnJudgeStart func(instanceID string)

	// OnEvaluationReady is called when evaluation is complete
	OnEvaluationReady func(evaluation *TripleShotEvaluation)

	// OnComplete is called when the entire triple-shot completes
	OnComplete func(success bool, summary string)
}

// TripleShotCoordinator orchestrates the execution of a triple-shot session
type TripleShotCoordinator struct {
	manager     *TripleShotManager
	orch        *Orchestrator
	baseSession *Session
	callbacks   *TripleShotCoordinatorCallbacks
	logger      *logging.Logger

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// Tracking
	runningAttempts map[int]string // attemptIndex -> instanceID
}

// NewTripleShotCoordinator creates a new coordinator for a triple-shot session
func NewTripleShotCoordinator(orch *Orchestrator, baseSession *Session, tripleSession *TripleShotSession, logger *logging.Logger) *TripleShotCoordinator {
	if logger == nil {
		logger = logging.NopLogger()
	}
	manager := NewTripleShotManager(orch, baseSession, tripleSession, logger)

	ctx, cancel := context.WithCancel(context.Background())

	sessionLogger := logger.WithSession(tripleSession.ID).WithPhase("tripleshot-coordinator")

	return &TripleShotCoordinator{
		manager:         manager,
		orch:            orch,
		baseSession:     baseSession,
		logger:          sessionLogger,
		ctx:             ctx,
		cancelFunc:      cancel,
		runningAttempts: make(map[int]string),
	}
}

// SetCallbacks sets the coordinator callbacks
func (c *TripleShotCoordinator) SetCallbacks(cb *TripleShotCoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Manager returns the underlying triple-shot manager
func (c *TripleShotCoordinator) Manager() *TripleShotManager {
	return c.manager
}

// Session returns the triple-shot session
func (c *TripleShotCoordinator) Session() *TripleShotSession {
	return c.manager.Session()
}

// notifyPhaseChange notifies callbacks of phase change
func (c *TripleShotCoordinator) notifyPhaseChange(phase TripleShotPhase) {
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

// notifyAttemptStart notifies callbacks of attempt start
func (c *TripleShotCoordinator) notifyAttemptStart(attemptIndex int, instanceID string) {
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
func (c *TripleShotCoordinator) notifyAttemptComplete(attemptIndex int) {
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
func (c *TripleShotCoordinator) notifyAttemptFailed(attemptIndex int, reason string) {
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
func (c *TripleShotCoordinator) notifyJudgeStart(instanceID string) {
	c.logger.Info("judge started", "instance_id", instanceID)

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnJudgeStart != nil {
		cb.OnJudgeStart(instanceID)
	}
}

// notifyEvaluationReady notifies callbacks of evaluation completion
func (c *TripleShotCoordinator) notifyEvaluationReady(evaluation *TripleShotEvaluation) {
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
func (c *TripleShotCoordinator) notifyComplete(success bool, summary string) {
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
func (c *TripleShotCoordinator) StartAttempts() error {
	session := c.Session()
	task := session.Task

	c.logger.Info("starting triple-shot attempts", "task", truncateString(task, 100))

	now := time.Now()
	session.StartedAt = &now

	// Create and start all three attempts
	for i := range 3 {
		prompt := fmt.Sprintf(TripleShotAttemptPromptTemplate, task, i)

		// Create instance for this attempt
		inst, err := c.orch.AddInstance(c.baseSession, prompt)
		if err != nil {
			return fmt.Errorf("failed to create attempt %d instance: %w", i, err)
		}

		// Record attempt info
		session.Attempts[i] = TripleShotAttempt{
			InstanceID:   inst.ID,
			WorktreePath: inst.WorktreePath,
			Branch:       inst.Branch,
			Status:       AttemptStatusWorking,
			StartedAt:    &now,
		}

		c.mu.Lock()
		c.runningAttempts[i] = inst.ID
		c.mu.Unlock()

		// Start the instance
		if err := c.orch.StartInstance(inst); err != nil {
			return fmt.Errorf("failed to start attempt %d: %w", i, err)
		}

		c.notifyAttemptStart(i, inst.ID)
	}

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting attempts", "error", err)
	}

	return nil
}

// StartJudge starts the judge instance to evaluate the three attempts
func (c *TripleShotCoordinator) StartJudge() error {
	session := c.Session()

	// Build the completion summaries for each attempt
	var summaries [3]string
	for i, attempt := range session.Attempts {
		completion, err := ParseTripleShotCompletionFile(attempt.WorktreePath)
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
	prompt := fmt.Sprintf(TripleShotJudgePromptTemplate,
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

	session.JudgeID = inst.ID

	// Transition to evaluating phase
	c.notifyPhaseChange(PhaseTripleShotEvaluating)

	// Start the judge instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start judge instance: %w", err)
	}

	c.notifyJudgeStart(inst.ID)

	// Persist session state
	if err := c.orch.SaveSession(); err != nil {
		c.logger.Error("failed to persist session after starting judge", "error", err)
	}

	return nil
}

// CheckAttemptCompletion checks if an attempt has written its completion file
func (c *TripleShotCoordinator) CheckAttemptCompletion(attemptIndex int) (bool, error) {
	session := c.Session()
	if attemptIndex < 0 || attemptIndex >= 3 {
		return false, fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	attempt := session.Attempts[attemptIndex]
	if attempt.WorktreePath == "" {
		return false, nil
	}

	completionPath := TripleShotCompletionFilePath(attempt.WorktreePath)
	_, err := os.Stat(completionPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	// Propagate actual errors (permission denied, I/O errors, etc.)
	return false, fmt.Errorf("failed to check completion file: %w", err)
}

// CheckJudgeCompletion checks if the judge has written its evaluation file
func (c *TripleShotCoordinator) CheckJudgeCompletion() (bool, error) {
	session := c.Session()
	if session.JudgeID == "" {
		return false, nil
	}

	// Find the judge instance to get its worktree path
	judgeInst := c.baseSession.GetInstance(session.JudgeID)
	if judgeInst == nil {
		return false, fmt.Errorf("judge instance %s not found", session.JudgeID)
	}

	evalPath := TripleShotEvaluationFilePath(judgeInst.WorktreePath)
	_, err := os.Stat(evalPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	// Propagate actual errors (permission denied, I/O errors, etc.)
	return false, fmt.Errorf("failed to check evaluation file: %w", err)
}

// ProcessAttemptCompletion handles when an attempt completes
func (c *TripleShotCoordinator) ProcessAttemptCompletion(attemptIndex int) error {
	if attemptIndex < 0 || attemptIndex >= 3 {
		return fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	session := c.Session()
	attempt := session.Attempts[attemptIndex]

	// Parse the completion file
	completion, err := ParseTripleShotCompletionFile(attempt.WorktreePath)
	if err != nil {
		c.notifyAttemptFailed(attemptIndex, fmt.Sprintf("failed to parse completion file: %v", err))
		return err
	}

	if completion.Status == "complete" {
		c.notifyAttemptComplete(attemptIndex)
	} else {
		c.notifyAttemptFailed(attemptIndex, completion.Summary)
	}

	// Check if all attempts are complete
	if session.AllAttemptsComplete() {
		c.logger.Info("all attempts complete", "successful_count", session.SuccessfulAttemptCount())

		// Only proceed to judge if we have at least 2 successful attempts
		if session.SuccessfulAttemptCount() >= 2 {
			c.manager.emitEvent(TripleShotEvent{
				Type:    EventTripleShotAllAttemptsReady,
				Message: "All attempts complete, ready for evaluation",
			})
		} else {
			c.notifyPhaseChange(PhaseTripleShotFailed)
			session.Error = "Fewer than 2 attempts succeeded"
			return fmt.Errorf("fewer than 2 attempts succeeded")
		}
	}

	return nil
}

// ProcessJudgeCompletion handles when the judge completes
func (c *TripleShotCoordinator) ProcessJudgeCompletion() error {
	session := c.Session()

	// Find the judge instance
	judgeInst := c.baseSession.GetInstance(session.JudgeID)
	if judgeInst == nil {
		return fmt.Errorf("judge instance not found")
	}

	// Parse the evaluation file
	evaluation, err := ParseTripleShotEvaluationFile(judgeInst.WorktreePath)
	if err != nil {
		c.notifyPhaseChange(PhaseTripleShotFailed)
		session.Error = fmt.Sprintf("failed to parse evaluation: %v", err)
		return err
	}

	c.notifyEvaluationReady(evaluation)

	// Transition to complete phase
	now := time.Now()
	session.CompletedAt = &now
	c.notifyPhaseChange(PhaseTripleShotComplete)

	// Build summary
	var summary string
	if evaluation.MergeStrategy == MergeStrategySelect {
		if evaluation.WinnerIndex < 0 || evaluation.WinnerIndex >= 3 {
			c.notifyPhaseChange(PhaseTripleShotFailed)
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

// Stop stops the triple-shot execution
func (c *TripleShotCoordinator) Stop() {
	c.cancelFunc()
	c.wg.Wait()
}

// GetAttemptInstanceID returns the instance ID for an attempt
func (c *TripleShotCoordinator) GetAttemptInstanceID(attemptIndex int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runningAttempts[attemptIndex]
}

// GetWinningBranch returns the branch name of the winning solution
// Returns empty string if evaluation is not complete or strategy is merge
func (c *TripleShotCoordinator) GetWinningBranch() string {
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

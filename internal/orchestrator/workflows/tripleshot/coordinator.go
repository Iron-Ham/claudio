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
	StartInstance(inst InstanceInterface) error
	SaveSession() error
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

	// Find the triple-shot group to add instances to
	// Use GroupID for multi-tripleshot support; fall back to session type for backward compatibility
	var tripleGroup GroupInterface
	if session.GroupID != "" {
		tripleGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if tripleGroup == nil {
		tripleGroup = c.baseSession.GetGroupBySessionType(c.sessionType)
	}

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
	// Use GroupID for multi-tripleshot support; fall back to session type for backward compatibility
	var tripleGroup GroupInterface
	if session.GroupID != "" {
		tripleGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if tripleGroup == nil {
		tripleGroup = c.baseSession.GetGroupBySessionType(c.sessionType)
	}
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

// CheckAttemptCompletion checks if an attempt has written its completion file
func (c *Coordinator) CheckAttemptCompletion(attemptIndex int) (bool, error) {
	session := c.Session()
	if attemptIndex < 0 || attemptIndex >= 3 {
		return false, fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	attempt := session.Attempts[attemptIndex]
	if attempt.WorktreePath == "" {
		return false, nil
	}

	completionPath := CompletionFilePath(attempt.WorktreePath)
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

	evalPath := EvaluationFilePath(judgeInst.GetWorktreePath())
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
func (c *Coordinator) ProcessAttemptCompletion(attemptIndex int) error {
	if attemptIndex < 0 || attemptIndex >= 3 {
		return fmt.Errorf("invalid attempt index: %d", attemptIndex)
	}

	session := c.Session()
	attempt := session.Attempts[attemptIndex]

	// Parse the completion file
	completion, err := ParseCompletionFile(attempt.WorktreePath)
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
			c.manager.EmitAllAttemptsReady()
		} else {
			c.notifyPhaseChange(PhaseFailed)
			session.Error = "Fewer than 2 attempts succeeded"
			return fmt.Errorf("fewer than 2 attempts succeeded")
		}
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

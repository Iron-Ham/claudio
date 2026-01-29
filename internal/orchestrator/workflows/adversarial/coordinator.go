package adversarial

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
type OrchestratorInterface interface {
	AddInstance(session SessionInterface, task string) (InstanceInterface, error)
	AddInstanceToWorktree(session SessionInterface, task, worktreePath, branch string) (InstanceInterface, error)
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
	GetInstances() []string
	RemoveInstance(instanceID string)
}

// GroupWithSubGroupsInterface extends GroupInterface with sub-group support.
// Groups that implement this interface can organize instances into sub-groups
// (e.g., one sub-group per adversarial round).
type GroupWithSubGroupsInterface interface {
	GroupInterface
	// GetOrCreateSubGroup finds or creates a sub-group with the given name.
	// If a sub-group with the name already exists, it returns that sub-group.
	// Otherwise, it creates a new sub-group with the given ID and name.
	GetOrCreateSubGroup(id, name string) GroupInterface
	// GetSubGroupByName returns a sub-group by name, or nil if not found.
	GetSubGroupByName(name string) GroupInterface
	// GetSubGroupByID returns a sub-group by ID, or nil if not found.
	GetSubGroupByID(id string) GroupInterface
	// MoveSubGroupUnder moves a sub-group (identified by ID) to become a child
	// of another sub-group (target, identified by ID). Returns true if successful.
	// If target doesn't exist, it will be created with the given targetName.
	MoveSubGroupUnder(subGroupID, targetID, targetName string) bool
}

// CoordinatorCallbacks holds callbacks for coordinator events
type CoordinatorCallbacks struct {
	// OnPhaseChange is called when the adversarial phase changes
	OnPhaseChange func(phase Phase)

	// OnImplementerStart is called when the implementer begins a round
	OnImplementerStart func(round int, instanceID string)

	// OnIncrementReady is called when the implementer submits work for review
	OnIncrementReady func(round int, increment *IncrementFile)

	// OnReviewerStart is called when the reviewer begins review
	OnReviewerStart func(round int, instanceID string)

	// OnReviewReady is called when the reviewer completes review
	OnReviewReady func(round int, review *ReviewFile)

	// OnApproved is called when the reviewer approves the implementation
	OnApproved func(round int, review *ReviewFile)

	// OnRejected is called when the reviewer rejects and requests changes
	OnRejected func(round int, review *ReviewFile)

	// OnComplete is called when the adversarial session completes
	OnComplete func(success bool, summary string)

	// OnStuck is called when an instance completes without writing its required file
	OnStuck func(role StuckRole, instanceID string)
}

// Coordinator orchestrates the execution of an adversarial review session
type Coordinator struct {
	manager     *Manager
	orch        OrchestratorInterface
	baseSession SessionInterface
	callbacks   *CoordinatorCallbacks
	logger      *logging.Logger

	// Session type constant for this workflow
	sessionType string

	// Running state
	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex

	// State tracking
	implementerWorktree string // Worktree path for implementer
	reviewerWorktree    string // Worktree path for reviewer (same as implementer)
}

// CoordinatorConfig holds configuration for creating a Coordinator
type CoordinatorConfig struct {
	Orchestrator OrchestratorInterface
	BaseSession  SessionInterface
	AdvSession   *Session
	Logger       *logging.Logger
	SessionType  string
}

// NewCoordinator creates a new coordinator for an adversarial session
func NewCoordinator(cfg CoordinatorConfig) *Coordinator {
	logger := cfg.Logger
	if logger == nil {
		logger = logging.NopLogger()
	}
	manager := NewManager(cfg.AdvSession, logger)

	ctx, cancel := context.WithCancel(context.Background())

	sessionLogger := logger.WithSession(cfg.AdvSession.ID).WithPhase("adversarial-coordinator")

	return &Coordinator{
		manager:     manager,
		orch:        cfg.Orchestrator,
		baseSession: cfg.BaseSession,
		logger:      sessionLogger,
		sessionType: cfg.SessionType,
		ctx:         ctx,
		cancelFunc:  cancel,
	}
}

// SetCallbacks sets the coordinator callbacks
func (c *Coordinator) SetCallbacks(cb *CoordinatorCallbacks) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = cb
}

// Manager returns the underlying adversarial manager
func (c *Coordinator) Manager() *Manager {
	return c.manager
}

// Session returns the adversarial session
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
	if c.orch != nil {
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist phase change", "error", err)
		}
	} else {
		c.logger.Warn("orchestrator is nil, skipping session persistence")
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnPhaseChange != nil {
		cb.OnPhaseChange(phase)
	}
}

// notifyImplementerStart notifies callbacks of implementer start
func (c *Coordinator) notifyImplementerStart(round int, instanceID string) {
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
func (c *Coordinator) notifyIncrementReady(round int, increment *IncrementFile) {
	c.manager.RecordIncrement(increment)

	// Persist increment
	if c.orch != nil {
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist increment", "round", round, "error", err)
		}
	} else {
		c.logger.Warn("orchestrator is nil, skipping session persistence")
	}

	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnIncrementReady != nil {
		cb.OnIncrementReady(round, increment)
	}
}

// notifyReviewerStart notifies callbacks of reviewer start
func (c *Coordinator) notifyReviewerStart(round int, instanceID string) {
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
func (c *Coordinator) notifyReviewReady(round int, review *ReviewFile) {
	c.manager.RecordReview(review)

	// Persist review
	if c.orch != nil {
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist review", "round", round, "error", err)
		}
	} else {
		c.logger.Warn("orchestrator is nil, skipping session persistence")
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
func (c *Coordinator) notifyComplete(success bool, summary string) {
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

// PreviousRoundsGroupName is the display name for the "Previous Rounds" container group.
const PreviousRoundsGroupName = "Previous Rounds"

// getCurrentRoundGroup returns the group where current round's instances should be added.
// Current round instances are added directly to the main adversarial group (not a sub-group).
// When round > 1, this first moves the previous round's instances to a sub-group.
func (c *Coordinator) getCurrentRoundGroup(advGroup GroupInterface, round int) GroupInterface {
	// For round > 1, first move previous round's instances to a sub-group
	if round > 1 {
		c.movePreviousRoundInstancesToSubGroup(advGroup, round-1)
	}

	// Current round's instances go directly in the main group
	return advGroup
}

// movePreviousRoundInstancesToSubGroup moves the previous round's instances from the main
// group into a "Round N" sub-group, then moves that sub-group under "Previous Rounds".
// This keeps the main group clean with only current round instances visible.
func (c *Coordinator) movePreviousRoundInstancesToSubGroup(advGroup GroupInterface, prevRound int) {
	// Check if the group supports sub-groups
	groupWithSubGroups, ok := advGroup.(GroupWithSubGroupsInterface)
	if !ok {
		return
	}

	session := c.Session()
	groupIDPrefix := session.GroupID
	if groupIDPrefix == "" {
		groupIDPrefix = session.ID
	}

	// Get instance IDs from the previous round's history
	if prevRound <= 0 || prevRound > len(session.History) {
		return
	}
	prevRoundHistory := session.History[prevRound-1]

	// Collect instance IDs from the previous round
	var prevInstanceIDs []string
	if prevRoundHistory.ImplementerID != "" {
		prevInstanceIDs = append(prevInstanceIDs, prevRoundHistory.ImplementerID)
	}
	if prevRoundHistory.ReviewerID != "" {
		prevInstanceIDs = append(prevInstanceIDs, prevRoundHistory.ReviewerID)
	}

	if len(prevInstanceIDs) == 0 {
		return // No instances to move
	}

	// Create a sub-group for the previous round
	subGroupID := fmt.Sprintf("%s-round-%d", groupIDPrefix, prevRound)
	subGroupName := fmt.Sprintf("Round %d", prevRound)
	prevRoundSubGroup := groupWithSubGroups.GetOrCreateSubGroup(subGroupID, subGroupName)

	// Move instances from main group to the sub-group
	for _, instID := range prevInstanceIDs {
		// Remove from main group
		advGroup.RemoveInstance(instID)
		// Add to sub-group
		prevRoundSubGroup.AddInstance(instID)
	}

	// Update the history to record the sub-group ID
	session.History[prevRound-1].SubGroupID = subGroupID

	// Move the sub-group under "Previous Rounds" container
	previousRoundsID := fmt.Sprintf("%s-previous-rounds", groupIDPrefix)
	if groupWithSubGroups.MoveSubGroupUnder(subGroupID, previousRoundsID, PreviousRoundsGroupName) {
		c.logger.Info("moved previous round instances to container",
			"round", prevRound,
			"instance_count", len(prevInstanceIDs),
			"sub_group_id", subGroupID,
			"container_id", previousRoundsID,
		)
	}
}

// GetPreviousRoundsGroupID returns the ID of the "Previous Rounds" container group
// for a given session, or empty string if not applicable.
func GetPreviousRoundsGroupID(session *Session) string {
	if session == nil {
		return ""
	}
	groupIDPrefix := session.GroupID
	if groupIDPrefix == "" {
		groupIDPrefix = session.ID
	}
	return fmt.Sprintf("%s-previous-rounds", groupIDPrefix)
}

// StartImplementer starts the implementer instance for the current round
func (c *Coordinator) StartImplementer() error {
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
	var previousReview *ReviewFile
	if round > 1 && len(session.History) > 0 {
		previousReview = session.History[len(session.History)-1].Review
	}

	// Start a new round
	c.manager.StartRound()

	// Build the implementer prompt
	prompt := FormatImplementerPrompt(task, round, previousReview)

	// Find the adversarial group to add instances to
	var advGroup GroupInterface
	if session.GroupID != "" {
		advGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if advGroup == nil {
		advGroup = c.baseSession.GetGroupBySessionType(c.sessionType)
	}

	// Get the group for the current round - instances go directly in the main group
	// (previous rounds are moved to sub-groups automatically when starting a new round)
	var targetGroup GroupInterface
	if advGroup != nil {
		targetGroup = c.getCurrentRoundGroup(advGroup, round)
	}

	// Create instance for implementer
	var inst InstanceInterface
	var err error

	if round == 1 {
		// First round - create a fresh worktree
		inst, err = c.orch.AddInstance(c.baseSession, prompt)
		if err != nil {
			return fmt.Errorf("failed to create implementer instance: %w", err)
		}
		c.implementerWorktree = inst.GetWorktreePath()
		c.reviewerWorktree = inst.GetWorktreePath() // Reviewer will use same worktree
		// Persist worktree path in session for reliable restoration
		session.WorktreePath = c.implementerWorktree
	} else {
		// Subsequent rounds - reuse the same worktree
		inst, err = c.orch.AddInstanceToWorktree(c.baseSession, prompt, c.implementerWorktree, "")
		if err != nil {
			return fmt.Errorf("failed to create implementer instance for round %d: %w", round, err)
		}
	}

	// Add instance to the main adversarial group (current round instances are at top level)
	if targetGroup != nil {
		targetGroup.AddInstance(inst.GetID())
	}

	// Store implementer ID in both session and current round history
	// Note: SubGroupID is no longer set for current round since instances are in main group
	session.ImplementerID = inst.GetID()
	if len(session.History) > 0 {
		session.History[len(session.History)-1].ImplementerID = inst.GetID()
	}

	// Transition to implementing phase
	c.notifyPhaseChange(PhaseImplementing)

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start implementer: %w", err)
	}

	c.notifyImplementerStart(round, inst.GetID())

	// Persist session state
	if c.orch != nil {
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist session after starting implementer", "error", err)
		}
	} else {
		c.logger.Warn("orchestrator is nil, skipping session persistence")
	}

	return nil
}

// StartReviewer starts the reviewer instance for the current round
func (c *Coordinator) StartReviewer(increment *IncrementFile) error {
	session := c.Session()
	task := session.Task
	round := session.CurrentRound

	c.logger.Info("starting reviewer", "round", round)

	// Build the reviewer prompt with configurable minimum passing score
	minPassingScore := session.Config.MinPassingScore
	if minPassingScore < 1 || minPassingScore > 10 {
		minPassingScore = 8 // Fallback to default if invalid
	}
	prompt := FormatReviewerPrompt(task, round, increment, minPassingScore)

	// Find the adversarial group
	var advGroup GroupInterface
	if session.GroupID != "" {
		advGroup = c.baseSession.GetGroup(session.GroupID)
	}
	if advGroup == nil {
		advGroup = c.baseSession.GetGroupBySessionType(c.sessionType)
	}

	// Get the main group for adding the reviewer instance
	// (current round instances go directly in the main adversarial group)
	var targetGroup GroupInterface
	if advGroup != nil {
		targetGroup = c.getCurrentRoundGroup(advGroup, round)
	}

	// Create reviewer instance in the same worktree
	inst, err := c.orch.AddInstanceToWorktree(c.baseSession, prompt, c.reviewerWorktree, "")
	if err != nil {
		return fmt.Errorf("failed to create reviewer instance: %w", err)
	}

	// Add instance to the main adversarial group (current round instances are at top level)
	if targetGroup != nil {
		targetGroup.AddInstance(inst.GetID())
	}

	// Store reviewer ID in both session and current round history
	session.ReviewerID = inst.GetID()
	if len(session.History) > 0 {
		session.History[len(session.History)-1].ReviewerID = inst.GetID()
	}

	// Transition to reviewing phase
	c.notifyPhaseChange(PhaseReviewing)

	// Start the instance
	if err := c.orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start reviewer: %w", err)
	}

	c.notifyReviewerStart(round, inst.GetID())

	// Persist session state
	if c.orch != nil {
		if err := c.orch.SaveSession(); err != nil {
			c.logger.Error("failed to persist session after starting reviewer", "error", err)
		}
	} else {
		c.logger.Warn("orchestrator is nil, skipping session persistence")
	}

	return nil
}

// CheckIncrementReady checks if the implementer has written their increment file
func (c *Coordinator) CheckIncrementReady() (bool, error) {
	if c.implementerWorktree == "" {
		return false, nil
	}

	incrementPath := IncrementFilePath(c.implementerWorktree)
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
func (c *Coordinator) CheckReviewReady() (bool, error) {
	if c.reviewerWorktree == "" {
		return false, nil
	}

	reviewPath := ReviewFilePath(c.reviewerWorktree)
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
func (c *Coordinator) ProcessIncrementCompletion() error {
	session := c.Session()
	round := session.CurrentRound

	// Parse the increment file
	increment, err := ParseIncrementFile(c.implementerWorktree)
	if err != nil {
		c.notifyPhaseChange(PhaseFailed)
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
		c.notifyPhaseChange(PhaseFailed)
		session.Error = fmt.Sprintf("implementer failed: %s", increment.Summary)
		return fmt.Errorf("implementer reported failure: %s", increment.Summary)
	}

	c.notifyIncrementReady(round, increment)

	// Clear the increment file so it's fresh for next round
	if err := os.Remove(IncrementFilePath(c.implementerWorktree)); err != nil && !os.IsNotExist(err) {
		c.logger.Warn("failed to remove increment file", "error", err)
	}

	// Start the reviewer
	return c.StartReviewer(increment)
}

// ProcessReviewCompletion handles when the reviewer completes
func (c *Coordinator) ProcessReviewCompletion() error {
	session := c.Session()
	round := session.CurrentRound

	// Parse the review file
	review, err := ParseReviewFile(c.reviewerWorktree)
	if err != nil {
		c.notifyPhaseChange(PhaseFailed)
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

	// Enforce score/approval consistency: if approved but score < minPassingScore, treat as rejection
	// This MUST happen before notifyReviewReady so callbacks receive the enforced state
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

	c.notifyReviewReady(round, review)

	// Clear the review file so it's fresh for next round
	if err := os.Remove(ReviewFilePath(c.reviewerWorktree)); err != nil && !os.IsNotExist(err) {
		c.logger.Warn("failed to remove review file", "error", err)
	}

	if review.Approved {
		// Work approved - complete the session
		now := time.Now()
		session.CompletedAt = &now
		c.notifyPhaseChange(PhaseApproved)
		c.notifyPhaseChange(PhaseComplete)

		summary := fmt.Sprintf("Approved after %d round(s). Final score: %d/10. %s",
			round, review.Score, review.Summary)
		c.notifyComplete(true, summary)
	} else {
		// Work rejected - check if we should continue
		if c.manager.IsMaxIterationsReached() {
			c.notifyPhaseChange(PhaseFailed)
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

// ProcessRejectionAfterApproval handles the case where a user rejects an approved
// result by having the reviewer write a new failing review file. This allows the
// workflow to continue from an approved/complete state.
func (c *Coordinator) ProcessRejectionAfterApproval() error {
	session := c.Session()

	// Parse the new review file
	review, err := ParseReviewFile(c.reviewerWorktree)
	if err != nil {
		return fmt.Errorf("failed to parse rejection review file: %w", err)
	}

	// Only process if this is actually a rejection
	if review.Approved {
		c.logger.Info("review file found after approval but it's still approved, ignoring",
			"score", review.Score,
		)
		// Remove the file so we don't keep re-processing it
		if err := os.Remove(ReviewFilePath(c.reviewerWorktree)); err != nil && !os.IsNotExist(err) {
			c.logger.Warn("failed to remove review file", "error", err)
		}
		return nil
	}

	c.logger.Info("processing rejection after approval",
		"score", review.Score,
		"issues_count", len(review.Issues),
		"required_changes_count", len(review.RequiredChanges),
	)

	// Clear completed state
	session.CompletedAt = nil
	session.Error = ""

	// Record the rejection
	c.notifyReviewReady(session.CurrentRound, review)

	// Remove the review file so it's fresh for next round
	if err := os.Remove(ReviewFilePath(c.reviewerWorktree)); err != nil && !os.IsNotExist(err) {
		c.logger.Warn("failed to remove review file", "error", err)
	}

	// Check iteration limits
	if c.manager.IsMaxIterationsReached() {
		c.notifyPhaseChange(PhaseFailed)
		session.Error = fmt.Sprintf("max iterations reached (%d) without approval", session.Config.MaxIterations)
		c.notifyComplete(false, session.Error)
		return fmt.Errorf("%s", session.Error)
	}

	// Advance to next round
	c.manager.NextRound()

	// Start implementer again with the review feedback
	return c.StartImplementer()
}

// Stop stops the adversarial execution
func (c *Coordinator) Stop() {
	c.cancelFunc()
	c.wg.Wait()
}

// GetImplementerWorktree returns the implementer's worktree path
func (c *Coordinator) GetImplementerWorktree() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.implementerWorktree
}

// SetWorktrees sets the worktree paths (used when restoring session)
func (c *Coordinator) SetWorktrees(worktree string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.implementerWorktree = worktree
	c.reviewerWorktree = worktree
}

// HandleInstanceCompletion checks if an adversarial instance completed without writing
// its required file. This should be called when the orchestrator detects that an instance
// has transitioned to a completed or waiting-for-input state.
//
// Returns true if this instance belongs to this coordinator and the stuck condition was
// detected and handled.
func (c *Coordinator) HandleInstanceCompletion(instanceID string, isCompleted bool, isWaitingInput bool) bool {
	session := c.Session()
	if session == nil {
		return false
	}

	// Check if this instance belongs to this session
	isImplementer := session.ImplementerID == instanceID
	isReviewer := session.ReviewerID == instanceID

	if !isImplementer && !isReviewer {
		return false // Instance doesn't belong to this session
	}

	// Only check stuck condition if instance completed/waiting and we're in the right phase
	if !isCompleted && !isWaitingInput {
		return false
	}

	c.logger.Debug("checking for stuck instance",
		"instance_id", instanceID,
		"is_implementer", isImplementer,
		"is_reviewer", isReviewer,
		"phase", string(session.Phase),
		"is_completed", isCompleted,
		"is_waiting_input", isWaitingInput,
	)

	// Check for stuck implementer
	if isImplementer && session.Phase == PhaseImplementing {
		// Check if increment file exists
		ready, err := c.CheckIncrementReady()
		if err != nil {
			// Filesystem error - cannot reliably determine stuck state
			c.logger.Warn("error checking increment file during stuck detection",
				"instance_id", instanceID,
				"error", err,
			)
			return false
		}
		if !ready {
			c.handleStuckImplementer(instanceID)
			return true
		}
	}

	// Check for stuck reviewer
	if isReviewer && session.Phase == PhaseReviewing {
		// Check if review file exists
		ready, err := c.CheckReviewReady()
		if err != nil {
			// Filesystem error - cannot reliably determine stuck state
			c.logger.Warn("error checking review file during stuck detection",
				"instance_id", instanceID,
				"error", err,
			)
			return false
		}
		if !ready {
			c.handleStuckReviewer(instanceID)
			return true
		}
	}

	return false
}

// handleStuckImplementer handles the case where the implementer completed without
// writing the increment file.
func (c *Coordinator) handleStuckImplementer(instanceID string) {
	session := c.Session()
	c.logger.Warn("implementer stuck - completed without writing increment file",
		"instance_id", instanceID,
		"round", session.CurrentRound,
	)

	// Set phase to stuck and record error
	session.Error = "Implementer completed without writing increment file. Use recovery options to continue."
	session.StuckRole = string(StuckRoleImplementer)
	c.notifyPhaseChange(PhaseStuck)

	// Notify via callback
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnStuck != nil {
		cb.OnStuck(StuckRoleImplementer, instanceID)
	}
}

// handleStuckReviewer handles the case where the reviewer completed without
// writing the review file.
func (c *Coordinator) handleStuckReviewer(instanceID string) {
	session := c.Session()
	c.logger.Warn("reviewer stuck - completed without writing review file",
		"instance_id", instanceID,
		"round", session.CurrentRound,
	)

	// Set phase to stuck and record error
	session.Error = "Reviewer completed without writing review file. Use recovery options to continue."
	session.StuckRole = string(StuckRoleReviewer)
	c.notifyPhaseChange(PhaseStuck)

	// Notify via callback
	c.mu.RLock()
	cb := c.callbacks
	c.mu.RUnlock()

	if cb != nil && cb.OnStuck != nil {
		cb.OnStuck(StuckRoleReviewer, instanceID)
	}
}

// RestartStuckRole restarts the stuck role (implementer or reviewer).
// This creates a new instance with a nudge prompt to write the required file.
// Returns the new instance ID or an error.
func (c *Coordinator) RestartStuckRole() error {
	session := c.Session()
	if session == nil {
		return fmt.Errorf("no active session")
	}

	if session.Phase != PhaseStuck {
		return fmt.Errorf("session is not stuck (phase: %s)", session.Phase)
	}

	stuckRole := StuckRole(session.StuckRole)
	if stuckRole == "" {
		return fmt.Errorf("no stuck role recorded")
	}

	c.logger.Info("restarting stuck role",
		"role", string(stuckRole),
		"round", session.CurrentRound,
	)

	// Attempt restart - only clear stuck state on success
	var err error
	switch stuckRole {
	case StuckRoleImplementer:
		err = c.StartImplementer()

	case StuckRoleReviewer:
		// We need the last increment to restart the reviewer
		if len(session.History) == 0 || session.History[len(session.History)-1].Increment == nil {
			return fmt.Errorf("no increment found to restart reviewer")
		}
		lastIncrement := session.History[len(session.History)-1].Increment
		err = c.StartReviewer(lastIncrement)

	default:
		return fmt.Errorf("unknown stuck role: %s", stuckRole)
	}

	// Only clear stuck state after successful restart
	if err == nil {
		session.Error = ""
		session.StuckRole = ""
	}

	return err
}

// GetStuckRole returns the stuck role if the session is stuck, or empty string otherwise.
func (c *Coordinator) GetStuckRole() StuckRole {
	session := c.Session()
	if session == nil || session.Phase != PhaseStuck {
		return ""
	}
	return StuckRole(session.StuckRole)
}

// IsStuck returns true if the session is in a stuck state.
func (c *Coordinator) IsStuck() bool {
	session := c.Session()
	return session != nil && session.Phase == PhaseStuck
}

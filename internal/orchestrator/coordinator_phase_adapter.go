package orchestrator

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/phase"
)

// coordinatorManagerAdapter adapts UltraPlanManager to the phase.UltraPlanManagerInterface.
// This adapter bridges the coordinator's manager to the interface expected by phase executors,
// enabling them to access session state and emit phase events without tight coupling.
type coordinatorManagerAdapter struct {
	c *Coordinator
}

// newCoordinatorManagerAdapter creates a new manager adapter for the given coordinator.
func newCoordinatorManagerAdapter(c *Coordinator) *coordinatorManagerAdapter {
	return &coordinatorManagerAdapter{c: c}
}

// Session returns the current ultra-plan session wrapped in the interface type.
func (a *coordinatorManagerAdapter) Session() phase.UltraPlanSessionInterface {
	if a.c == nil || a.c.manager == nil {
		return nil
	}
	session := a.c.manager.Session()
	if session == nil {
		return nil
	}
	return newCoordinatorSessionAdapter(a.c, session)
}

// SetPhase updates the session phase and emits a phase change event.
func (a *coordinatorManagerAdapter) SetPhase(p phase.UltraPlanPhase) {
	if a.c == nil || a.c.manager == nil {
		return
	}
	// Convert phase.UltraPlanPhase to orchestrator.UltraPlanPhase
	a.c.manager.SetPhase(UltraPlanPhase(p))
}

// SetPlan sets the plan on the session.
// The plan parameter is expected to be a *PlanSpec but accepts any for interface flexibility.
func (a *coordinatorManagerAdapter) SetPlan(plan any) {
	if a.c == nil || a.c.manager == nil {
		return
	}
	if planSpec, ok := plan.(*PlanSpec); ok {
		a.c.manager.session.Plan = planSpec
	}
}

// MarkTaskComplete marks a task as completed in the session state.
func (a *coordinatorManagerAdapter) MarkTaskComplete(taskID string) {
	if a.c == nil || a.c.manager == nil {
		return
	}
	a.c.manager.MarkTaskComplete(taskID)
}

// MarkTaskFailed marks a task as failed with the given reason.
func (a *coordinatorManagerAdapter) MarkTaskFailed(taskID string, reason string) {
	if a.c == nil || a.c.manager == nil {
		return
	}
	a.c.manager.MarkTaskFailed(taskID, reason)
}

// AssignTaskToInstance records the mapping from task to instance.
func (a *coordinatorManagerAdapter) AssignTaskToInstance(taskID, instanceID string) {
	if a.c == nil || a.c.manager == nil {
		return
	}
	a.c.manager.AssignTaskToInstance(taskID, instanceID)
}

// Stop signals the ultra-plan execution to stop.
func (a *coordinatorManagerAdapter) Stop() {
	if a.c == nil || a.c.manager == nil {
		return
	}
	a.c.manager.Stop()
}

// coordinatorOrchestratorAdapter adapts the Orchestrator to phase.OrchestratorInterface.
// This adapter provides phase executors with access to instance management, worktree operations,
// and session persistence without exposing the full Orchestrator implementation.
type coordinatorOrchestratorAdapter struct {
	c *Coordinator
}

// newCoordinatorOrchestratorAdapter creates a new orchestrator adapter.
func newCoordinatorOrchestratorAdapter(c *Coordinator) *coordinatorOrchestratorAdapter {
	return &coordinatorOrchestratorAdapter{c: c}
}

// AddInstance creates a new instance in the session with the given task prompt.
// Returns the instance wrapped as any for interface flexibility.
func (a *coordinatorOrchestratorAdapter) AddInstance(session any, task string) (any, error) {
	if a.c == nil || a.c.orch == nil {
		return nil, ErrNilCoordinator
	}
	// The session parameter is the base Session, not UltraPlanSession
	if sess, ok := session.(*Session); ok {
		return a.c.orch.AddInstance(sess, task)
	}
	// Fallback to coordinator's base session
	return a.c.orch.AddInstance(a.c.baseSession, task)
}

// StartInstance starts a Claude process for the given instance.
func (a *coordinatorOrchestratorAdapter) StartInstance(inst any) error {
	if a.c == nil || a.c.orch == nil {
		return ErrNilCoordinator
	}
	if instance, ok := inst.(*Instance); ok {
		return a.c.orch.StartInstance(instance)
	}
	return ErrInstanceTypeAssertion
}

// SaveSession persists the session state to disk.
func (a *coordinatorOrchestratorAdapter) SaveSession() error {
	if a.c == nil || a.c.orch == nil {
		return ErrNilCoordinator
	}
	return a.c.orch.SaveSession()
}

// GetInstanceManager returns the manager for the given instance ID.
func (a *coordinatorOrchestratorAdapter) GetInstanceManager(id string) any {
	if a.c == nil || a.c.orch == nil {
		return nil
	}
	return a.c.orch.GetInstanceManager(id)
}

// BranchPrefix returns the configured branch prefix for worktree branches.
func (a *coordinatorOrchestratorAdapter) BranchPrefix() string {
	if a.c == nil || a.c.orch == nil {
		return ""
	}
	return a.c.orch.BranchPrefix()
}

// GetInstance returns an instance by ID wrapped in the InstanceInterface.
func (a *coordinatorOrchestratorAdapter) GetInstance(id string) phase.InstanceInterface {
	if a.c == nil || a.c.orch == nil {
		return nil
	}
	inst := a.c.orch.GetInstance(id)
	if inst == nil {
		return nil
	}
	return newInstanceInterfaceAdapter(inst)
}

// instanceInterfaceAdapter adapts *Instance to phase.InstanceInterface.
type instanceInterfaceAdapter struct {
	inst *Instance
}

// newInstanceInterfaceAdapter creates a new instance interface adapter.
func newInstanceInterfaceAdapter(inst *Instance) *instanceInterfaceAdapter {
	return &instanceInterfaceAdapter{inst: inst}
}

// GetID returns the instance ID.
func (a *instanceInterfaceAdapter) GetID() string {
	if a.inst == nil {
		return ""
	}
	return a.inst.ID
}

// GetWorktreePath returns the path to the instance's worktree.
func (a *instanceInterfaceAdapter) GetWorktreePath() string {
	if a.inst == nil {
		return ""
	}
	return a.inst.WorktreePath
}

// GetBranch returns the branch name for the instance.
func (a *instanceInterfaceAdapter) GetBranch() string {
	if a.inst == nil {
		return ""
	}
	return a.inst.Branch
}

// GetStatus returns the current status of the instance.
func (a *instanceInterfaceAdapter) GetStatus() phase.InstanceStatus {
	if a.inst == nil {
		return phase.StatusPending
	}
	return phase.InstanceStatus(a.inst.Status)
}

// GetFilesModified returns the list of files modified by this instance.
func (a *instanceInterfaceAdapter) GetFilesModified() []string {
	if a.inst == nil {
		return nil
	}
	return a.inst.FilesModified
}

// coordinatorSessionAdapter adapts UltraPlanSession to phase.UltraPlanSessionInterface.
// This provides phase executors with access to session state like task lists and group progress.
type coordinatorSessionAdapter struct {
	c       *Coordinator
	session *UltraPlanSession
}

// newCoordinatorSessionAdapter creates a new session adapter.
func newCoordinatorSessionAdapter(c *Coordinator, session *UltraPlanSession) *coordinatorSessionAdapter {
	return &coordinatorSessionAdapter{c: c, session: session}
}

// GetTask returns a planned task by ID.
// Returns the task wrapped as any, or nil if not found.
func (a *coordinatorSessionAdapter) GetTask(taskID string) any {
	if a.session == nil {
		return nil
	}
	return a.session.GetTask(taskID)
}

// GetReadyTasks returns all task IDs that are ready to execute.
// A task is ready when all its dependencies are complete and it hasn't started yet.
func (a *coordinatorSessionAdapter) GetReadyTasks() []string {
	if a.session == nil {
		return nil
	}
	return a.session.GetReadyTasks()
}

// IsCurrentGroupComplete returns true if all tasks in the current group are done.
func (a *coordinatorSessionAdapter) IsCurrentGroupComplete() bool {
	if a.session == nil {
		return false
	}
	return a.session.IsCurrentGroupComplete()
}

// AdvanceGroupIfComplete advances to the next execution group if the current one is complete.
// Returns whether advancement occurred and the previous group index.
func (a *coordinatorSessionAdapter) AdvanceGroupIfComplete() (advanced bool, previousGroup int) {
	if a.session == nil {
		return false, 0
	}
	return a.session.AdvanceGroupIfComplete()
}

// HasMoreGroups returns true if there are more execution groups to process.
func (a *coordinatorSessionAdapter) HasMoreGroups() bool {
	if a.session == nil {
		return false
	}
	return a.session.HasMoreGroups()
}

// Progress returns the completion progress as a percentage (0.0 to 1.0).
func (a *coordinatorSessionAdapter) Progress() float64 {
	if a.session == nil {
		return 0.0
	}
	return a.session.Progress()
}

// GetObjective returns the original user objective for the ultra-plan.
func (a *coordinatorSessionAdapter) GetObjective() string {
	if a.session == nil {
		return ""
	}
	return a.session.Objective
}

// GetCompletedTasks returns the list of completed task IDs.
func (a *coordinatorSessionAdapter) GetCompletedTasks() []string {
	if a.session == nil {
		return nil
	}
	return a.session.CompletedTasks
}

// GetTaskToInstance returns the mapping of task IDs to instance IDs.
func (a *coordinatorSessionAdapter) GetTaskToInstance() map[string]string {
	if a.session == nil {
		return nil
	}
	return a.session.TaskToInstance
}

// GetTaskCommitCounts returns the commit counts for each task.
func (a *coordinatorSessionAdapter) GetTaskCommitCounts() map[string]int {
	if a.session == nil {
		return nil
	}
	return a.session.TaskCommitCounts
}

// GetSynthesisID returns the ID of the synthesis instance, or empty if not set.
func (a *coordinatorSessionAdapter) GetSynthesisID() string {
	if a.session == nil {
		return ""
	}
	return a.session.SynthesisID
}

// SetSynthesisID sets the ID of the synthesis instance.
func (a *coordinatorSessionAdapter) SetSynthesisID(id string) {
	if a.session == nil {
		return
	}
	a.session.SynthesisID = id
}

// GetRevisionRound returns the current revision round (0 for first synthesis).
func (a *coordinatorSessionAdapter) GetRevisionRound() int {
	if a.session == nil || a.session.Revision == nil {
		return 0
	}
	return a.session.Revision.RevisionRound
}

// SetSynthesisAwaitingApproval sets whether synthesis is waiting for approval.
func (a *coordinatorSessionAdapter) SetSynthesisAwaitingApproval(awaiting bool) {
	if a.session == nil {
		return
	}
	a.session.SynthesisAwaitingApproval = awaiting
}

// IsSynthesisAwaitingApproval returns true if synthesis is waiting for approval.
func (a *coordinatorSessionAdapter) IsSynthesisAwaitingApproval() bool {
	if a.session == nil {
		return false
	}
	return a.session.SynthesisAwaitingApproval
}

// SetSynthesisCompletion sets the synthesis completion data.
func (a *coordinatorSessionAdapter) SetSynthesisCompletion(completion *phase.SynthesisCompletionFile) {
	if a.session == nil || completion == nil {
		return
	}
	// Convert phase.SynthesisCompletionFile to orchestrator.SynthesisCompletionFile
	orchCompletion := &SynthesisCompletionFile{
		Status:           completion.Status,
		RevisionRound:    completion.RevisionRound,
		TasksAffected:    completion.TasksAffected,
		IntegrationNotes: completion.IntegrationNotes,
		Recommendations:  completion.Recommendations,
	}
	// Convert issues
	for _, issue := range completion.IssuesFound {
		orchCompletion.IssuesFound = append(orchCompletion.IssuesFound, RevisionIssue{
			TaskID:      issue.TaskID,
			Description: issue.Description,
			Files:       issue.Files,
			Severity:    issue.Severity,
			Suggestion:  issue.Suggestion,
		})
	}
	a.session.SynthesisCompletion = orchCompletion
}

// GetPhase returns the current phase of the ultra-plan.
func (a *coordinatorSessionAdapter) GetPhase() phase.UltraPlanPhase {
	if a.session == nil {
		return phase.PhasePlanning
	}
	return phase.UltraPlanPhase(a.session.Phase)
}

// SetPhase sets the current phase of the ultra-plan.
func (a *coordinatorSessionAdapter) SetPhase(p phase.UltraPlanPhase) {
	if a.session == nil {
		return
	}
	a.session.Phase = UltraPlanPhase(p)
}

// SetError sets an error message on the session.
func (a *coordinatorSessionAdapter) SetError(err string) {
	if a.session == nil {
		return
	}
	a.session.Error = err
}

// GetConfig returns the ultra-plan configuration.
func (a *coordinatorSessionAdapter) GetConfig() phase.UltraPlanConfigInterface {
	if a.session == nil {
		return nil
	}
	return &configAdapter{config: &a.session.Config}
}

// configAdapter adapts UltraPlanConfig to phase.UltraPlanConfigInterface.
type configAdapter struct {
	config *UltraPlanConfig
}

// IsMultiPass returns true if multi-pass planning is enabled.
func (a *configAdapter) IsMultiPass() bool {
	if a.config == nil {
		return false
	}
	return a.config.MultiPass
}

// coordinatorCallbacksAdapter adapts CoordinatorCallbacks to phase.CoordinatorCallbacksInterface.
// This allows phase executors to emit events (phase changes, task completion, progress)
// back to the coordinator's callback handlers.
type coordinatorCallbacksAdapter struct {
	c *Coordinator
}

// newCoordinatorCallbacksAdapter creates a new callbacks adapter.
func newCoordinatorCallbacksAdapter(c *Coordinator) *coordinatorCallbacksAdapter {
	return &coordinatorCallbacksAdapter{c: c}
}

// OnPhaseChange is called when the ultra-plan phase changes.
func (a *coordinatorCallbacksAdapter) OnPhaseChange(p phase.UltraPlanPhase) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnPhaseChange != nil {
		cb.OnPhaseChange(UltraPlanPhase(p))
	}
}

// OnTaskStart is called when a task begins execution.
func (a *coordinatorCallbacksAdapter) OnTaskStart(taskID, instanceID string) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnTaskStart != nil {
		cb.OnTaskStart(taskID, instanceID)
	}
}

// OnTaskComplete is called when a task completes successfully.
func (a *coordinatorCallbacksAdapter) OnTaskComplete(taskID string) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnTaskComplete != nil {
		cb.OnTaskComplete(taskID)
	}
}

// OnTaskFailed is called when a task fails.
func (a *coordinatorCallbacksAdapter) OnTaskFailed(taskID, reason string) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnTaskFailed != nil {
		cb.OnTaskFailed(taskID, reason)
	}
}

// OnGroupComplete is called when an execution group completes.
func (a *coordinatorCallbacksAdapter) OnGroupComplete(groupIndex int) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnGroupComplete != nil {
		cb.OnGroupComplete(groupIndex)
	}
}

// OnPlanReady is called when the plan is ready.
func (a *coordinatorCallbacksAdapter) OnPlanReady(plan any) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnPlanReady != nil {
		if planSpec, ok := plan.(*PlanSpec); ok {
			cb.OnPlanReady(planSpec)
		}
	}
}

// OnProgress is called periodically with progress updates.
func (a *coordinatorCallbacksAdapter) OnProgress(completed, total int, p phase.UltraPlanPhase) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnProgress != nil {
		cb.OnProgress(completed, total, UltraPlanPhase(p))
	}
}

// OnComplete is called when the entire ultra-plan completes.
func (a *coordinatorCallbacksAdapter) OnComplete(success bool, summary string) {
	if a.c == nil {
		return
	}
	a.c.mu.RLock()
	cb := a.c.callbacks
	a.c.mu.RUnlock()
	if cb != nil && cb.OnComplete != nil {
		cb.OnComplete(success, summary)
	}
}

// Error sentinel for adapter type assertions
var ErrInstanceTypeAssertion = newAdapterError("instance type assertion failed")

// adapterError represents an error from adapter operations.
type adapterError struct {
	message string
}

func newAdapterError(msg string) *adapterError {
	return &adapterError{message: msg}
}

func (e *adapterError) Error() string {
	return "coordinator phase adapter: " + e.message
}

// BuildPhaseContext creates a phase.PhaseContext configured with adapters
// that bridge the Coordinator's state to the phase executor interfaces.
// This is the primary entry point for phase orchestrators to obtain their dependencies.
//
// The returned PhaseContext provides:
//   - Manager: adapter to UltraPlanManager for session/phase management
//   - Orchestrator: adapter for instance lifecycle and persistence
//   - Session: adapter for task and group state queries
//   - Logger: the coordinator's logger (or NopLogger if nil)
//   - Callbacks: adapter for event notification (nil-safe)
//
// Returns an error if the coordinator or its required dependencies are nil.
func (c *Coordinator) BuildPhaseContext() (*phase.PhaseContext, error) {
	if c == nil {
		return nil, ErrNilCoordinator
	}
	if c.manager == nil {
		return nil, ErrNilManager
	}
	session := c.manager.Session()
	if session == nil {
		return nil, ErrNilSession
	}

	logger := c.logger
	if logger == nil {
		logger = logging.NopLogger()
	}

	ctx := &phase.PhaseContext{
		Manager:      newCoordinatorManagerAdapter(c),
		Orchestrator: newCoordinatorOrchestratorAdapter(c),
		Session:      newCoordinatorSessionAdapter(c, session),
		BaseSession:  newBaseSessionAdapter(c),
		Logger:       logger,
		Callbacks:    newCoordinatorCallbacksAdapter(c),
	}

	if err := ctx.Validate(); err != nil {
		return nil, err
	}

	return ctx, nil
}

// baseSessionAdapter implements phase.BaseSessionInterface.
// It provides access to the base session for instance grouping.
type baseSessionAdapter struct {
	c *Coordinator
}

// newBaseSessionAdapter creates a new base session adapter.
func newBaseSessionAdapter(c *Coordinator) *baseSessionAdapter {
	return &baseSessionAdapter{c: c}
}

// GetGroupBySessionType returns the instance group for a session type.
func (a *baseSessionAdapter) GetGroupBySessionType(sessionType string) phase.InstanceGroupInterface {
	if a.c == nil || a.c.baseSession == nil {
		return nil
	}
	group := a.c.baseSession.GetGroupBySessionType(SessionType(sessionType))
	if group == nil {
		return nil
	}
	return &instanceGroupAdapter{group: group}
}

// GetInstances returns all instances in the session.
func (a *baseSessionAdapter) GetInstances() []phase.InstanceInterface {
	if a.c == nil || a.c.baseSession == nil {
		return nil
	}
	instances := a.c.baseSession.Instances
	result := make([]phase.InstanceInterface, len(instances))
	for i, inst := range instances {
		result[i] = newInstanceInterfaceAdapter(inst)
	}
	return result
}

// instanceGroupAdapter implements phase.InstanceGroupInterface.
type instanceGroupAdapter struct {
	group *InstanceGroup
}

// AddInstance adds an instance to the group.
func (a *instanceGroupAdapter) AddInstance(instanceID string) {
	if a.group == nil {
		return
	}
	a.group.AddInstance(instanceID)
}

// GetBaseSession returns the base Session for use by phase orchestrators.
// This provides access to instances, worktrees, and other session-level state
// that is shared across all phases.
func (c *Coordinator) GetBaseSession() *Session {
	if c == nil {
		return nil
	}
	return c.baseSession
}

// GetOrchestrator returns the underlying Orchestrator for use by phase orchestrators.
// This provides access to instance management, worktree operations, and git operations.
func (c *Coordinator) GetOrchestrator() *Orchestrator {
	if c == nil {
		return nil
	}
	return c.orch
}

// GetLogger returns the Coordinator's logger for use by phase orchestrators.
// Returns a NopLogger if the coordinator's logger is nil.
func (c *Coordinator) GetLogger() *logging.Logger {
	if c == nil || c.logger == nil {
		return logging.NopLogger()
	}
	return c.logger
}

// GetContext returns the Coordinator's context for cancellation propagation.
// Phase orchestrators should use this to respect cancellation requests.
func (c *Coordinator) GetContext() any {
	if c == nil {
		return nil
	}
	return c.ctx
}

// EmitEvent emits a CoordinatorEvent through the manager's event system.
// This allows phase orchestrators to emit events using the existing infrastructure.
func (c *Coordinator) EmitEvent(event CoordinatorEvent) {
	if c == nil || c.manager == nil {
		return
	}
	c.manager.emitEvent(event)
}

// GetVerifier returns the task verifier for use by phase orchestrators.
// The verifier handles checking completion files and verifying task work.
func (c *Coordinator) GetVerifier() Verifier {
	if c == nil {
		return nil
	}
	return c.verifier
}

// GetBaseBranchForGroup returns the base branch that tasks in the given group
// should be created from. For group 0, this returns empty (use HEAD/main).
// For later groups, returns the consolidated branch from the previous group.
func (c *Coordinator) GetBaseBranchForGroup(groupIndex int) string {
	if c == nil {
		return ""
	}
	return c.getBaseBranchForGroup(groupIndex)
}

// AddRunningTask registers a task as running with the given instance ID.
// This is used by execution orchestrators to track task state.
func (c *Coordinator) AddRunningTask(taskID, instanceID string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runningTasks == nil {
		c.runningTasks = make(map[string]string)
	}
	c.runningTasks[taskID] = instanceID
	c.runningCount++
}

// RemoveRunningTask unregisters a task from the running state.
// Returns true if the task was being tracked.
func (c *Coordinator) RemoveRunningTask(taskID string) bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.runningTasks[taskID]; exists {
		delete(c.runningTasks, taskID)
		c.runningCount--
		return true
	}
	return false
}

// GetRunningTaskCount returns the number of currently running tasks.
func (c *Coordinator) GetRunningTaskCount() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runningCount
}

// IsTaskRunning returns true if the given task is currently running.
func (c *Coordinator) IsTaskRunning(taskID string) bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.runningTasks[taskID]
	return exists
}

// GetRunningTaskInstance returns the instance ID for a running task, or empty if not running.
func (c *Coordinator) GetRunningTaskInstance(taskID string) string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.runningTasks[taskID]
}

// SyncRetryState syncs retry state to the session for persistence.
// This should be called after verification operations that update retry state.
func (c *Coordinator) SyncRetryState() {
	if c == nil {
		return
	}
	c.syncRetryState()
}

// initializeOrchestrators creates the phase orchestrators if they haven't been created yet.
// This uses lazy initialization because BuildPhaseContext depends on the coordinator
// being fully constructed, which isn't the case during NewCoordinator.
// This method is thread-safe and idempotent.
func (c *Coordinator) initializeOrchestrators() error {
	if c == nil {
		return ErrNilCoordinator
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already initialized (any one being non-nil means they're all initialized)
	if c.planningOrchestrator != nil {
		return nil
	}

	// Build the shared PhaseContext
	phaseCtx, err := c.buildPhaseContextLocked()
	if err != nil {
		return err
	}

	// Create the planning orchestrator
	c.planningOrchestrator, err = phase.NewPlanningOrchestrator(phaseCtx)
	if err != nil {
		return fmt.Errorf("failed to create planning orchestrator: %w", err)
	}

	// Create the execution orchestrator
	c.executionOrchestrator, err = phase.NewExecutionOrchestrator(phaseCtx)
	if err != nil {
		return fmt.Errorf("failed to create execution orchestrator: %w", err)
	}

	// Create the synthesis orchestrator
	c.synthesisOrchestrator, err = phase.NewSynthesisOrchestrator(phaseCtx)
	if err != nil {
		return fmt.Errorf("failed to create synthesis orchestrator: %w", err)
	}

	// Create the consolidation orchestrator (doesn't return error)
	c.consolidationOrchestrator = phase.NewConsolidationOrchestrator(phaseCtx)

	return nil
}

// buildPhaseContextLocked creates a PhaseContext without acquiring the mutex.
// The caller must hold the mutex. This is used by initializeOrchestrators.
func (c *Coordinator) buildPhaseContextLocked() (*phase.PhaseContext, error) {
	if c.manager == nil {
		return nil, ErrNilManager
	}
	session := c.manager.Session()
	if session == nil {
		return nil, ErrNilSession
	}

	logger := c.logger
	if logger == nil {
		logger = logging.NopLogger()
	}

	ctx := &phase.PhaseContext{
		Manager:      newCoordinatorManagerAdapter(c),
		Orchestrator: newCoordinatorOrchestratorAdapter(c),
		Session:      newCoordinatorSessionAdapter(c, session),
		BaseSession:  newBaseSessionAdapter(c),
		Logger:       logger,
		Callbacks:    newCoordinatorCallbacksAdapter(c),
	}

	if err := ctx.Validate(); err != nil {
		return nil, err
	}

	return ctx, nil
}

// PlanningOrchestrator returns the planning phase orchestrator.
// The orchestrator is created lazily on first access.
// Returns nil and logs an error if initialization fails.
func (c *Coordinator) PlanningOrchestrator() *phase.PlanningOrchestrator {
	if c == nil {
		return nil
	}
	if err := c.initializeOrchestrators(); err != nil {
		c.logger.Error("failed to initialize orchestrators", "error", err)
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.planningOrchestrator
}

// ExecutionOrchestrator returns the execution phase orchestrator.
// The orchestrator is created lazily on first access.
// Returns nil and logs an error if initialization fails.
func (c *Coordinator) ExecutionOrchestrator() *phase.ExecutionOrchestrator {
	if c == nil {
		return nil
	}
	if err := c.initializeOrchestrators(); err != nil {
		c.logger.Error("failed to initialize orchestrators", "error", err)
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.executionOrchestrator
}

// SynthesisOrchestrator returns the synthesis phase orchestrator.
// The orchestrator is created lazily on first access.
// Returns nil and logs an error if initialization fails.
func (c *Coordinator) SynthesisOrchestrator() *phase.SynthesisOrchestrator {
	if c == nil {
		return nil
	}
	if err := c.initializeOrchestrators(); err != nil {
		c.logger.Error("failed to initialize orchestrators", "error", err)
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.synthesisOrchestrator
}

// ConsolidationOrchestrator returns the consolidation phase orchestrator.
// The orchestrator is created lazily on first access.
// Returns nil and logs an error if initialization fails.
func (c *Coordinator) ConsolidationOrchestrator() *phase.ConsolidationOrchestrator {
	if c == nil {
		return nil
	}
	if err := c.initializeOrchestrators(); err != nil {
		c.logger.Error("failed to initialize orchestrators", "error", err)
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.consolidationOrchestrator
}

// ============================================================================
// Execution Phase Adapters
// ============================================================================

// executionCoordinatorAdapter implements phase.ExecutionCoordinatorInterface.
// This adapter bridges the Coordinator to the execution orchestrator's
// coordinator interface, enabling the execution phase to be fully delegated.
type executionCoordinatorAdapter struct {
	c *Coordinator
}

// newExecutionCoordinatorAdapter creates a new execution coordinator adapter.
func newExecutionCoordinatorAdapter(c *Coordinator) *executionCoordinatorAdapter {
	return &executionCoordinatorAdapter{c: c}
}

// GetBaseBranchForGroup returns the base branch for a given group index.
func (a *executionCoordinatorAdapter) GetBaseBranchForGroup(groupIndex int) string {
	if a.c == nil {
		return ""
	}
	return a.c.getBaseBranchForGroup(groupIndex)
}

// AddRunningTask registers a task as running with the given instance ID.
func (a *executionCoordinatorAdapter) AddRunningTask(taskID, instanceID string) {
	if a.c == nil {
		return
	}
	a.c.AddRunningTask(taskID, instanceID)
}

// RemoveRunningTask unregisters a task from the running state.
func (a *executionCoordinatorAdapter) RemoveRunningTask(taskID string) bool {
	if a.c == nil {
		return false
	}
	return a.c.RemoveRunningTask(taskID)
}

// GetRunningTaskCount returns the number of currently running tasks.
func (a *executionCoordinatorAdapter) GetRunningTaskCount() int {
	if a.c == nil {
		return 0
	}
	return a.c.GetRunningTaskCount()
}

// IsTaskRunning returns true if the given task is currently running.
func (a *executionCoordinatorAdapter) IsTaskRunning(taskID string) bool {
	if a.c == nil {
		return false
	}
	return a.c.IsTaskRunning(taskID)
}

// GetBaseSession returns the base session for instance group management.
func (a *executionCoordinatorAdapter) GetBaseSession() any {
	if a.c == nil {
		return nil
	}
	return a.c.baseSession
}

// GetTaskGroupIndex returns the group index for a given task ID.
func (a *executionCoordinatorAdapter) GetTaskGroupIndex(taskID string) int {
	if a.c == nil {
		return 0
	}
	return a.c.getTaskGroupIndex(taskID)
}

// VerifyTaskWork checks if a task produced actual commits.
func (a *executionCoordinatorAdapter) VerifyTaskWork(taskID string, inst any) phase.TaskCompletion {
	if a.c == nil {
		return phase.TaskCompletion{TaskID: taskID, Success: false, Error: "nil coordinator"}
	}
	instance, ok := inst.(*Instance)
	if !ok {
		return phase.TaskCompletion{TaskID: taskID, Success: false, Error: "invalid instance type"}
	}
	result := a.c.verifyTaskWork(taskID, instance)
	return phase.TaskCompletion{
		TaskID:      result.taskID,
		InstanceID:  result.instanceID,
		Success:     result.success,
		Error:       result.error,
		NeedsRetry:  result.needsRetry,
		CommitCount: result.commitCount,
	}
}

// CheckForTaskCompletionFile checks if the task has written its completion sentinel file.
func (a *executionCoordinatorAdapter) CheckForTaskCompletionFile(inst any) bool {
	if a.c == nil {
		return false
	}
	if instance, ok := inst.(*Instance); ok {
		return a.c.checkForTaskCompletionFile(instance)
	}
	return false
}

// HandleTaskCompletion processes a task completion notification.
func (a *executionCoordinatorAdapter) HandleTaskCompletion(completion phase.TaskCompletion) {
	if a.c == nil {
		return
	}
	a.c.handleTaskCompletion(taskCompletion{
		taskID:      completion.TaskID,
		instanceID:  completion.InstanceID,
		success:     completion.Success,
		error:       completion.Error,
		needsRetry:  completion.NeedsRetry,
		commitCount: completion.CommitCount,
	})
}

// PollTaskCompletions checks for task completions that monitoring goroutines may have missed.
// It polls synchronously and forwards any found completions to the provided channel.
func (a *executionCoordinatorAdapter) PollTaskCompletions(completionChan chan<- phase.TaskCompletion) {
	if a.c == nil {
		return
	}
	// Create a local channel to receive completions
	localChan := make(chan taskCompletion, 100)

	// Start a goroutine that polls and then closes the channel when done.
	// This ensures the forwarding goroutine below won't block forever.
	go func() {
		a.c.pollTaskCompletions(localChan)
		close(localChan) // Signal that no more completions will be sent
	}()

	// Forward any completions to the phase completion channel.
	// This goroutine will exit when localChan is closed.
	go func() {
		for tc := range localChan {
			completionChan <- phase.TaskCompletion{
				TaskID:      tc.taskID,
				InstanceID:  tc.instanceID,
				Success:     tc.success,
				Error:       tc.error,
				NeedsRetry:  tc.needsRetry,
				CommitCount: tc.commitCount,
			}
		}
	}()
}

// NotifyTaskStart notifies callbacks that a task has started.
func (a *executionCoordinatorAdapter) NotifyTaskStart(taskID, instanceID string) {
	if a.c == nil {
		return
	}
	a.c.notifyTaskStart(taskID, instanceID)
}

// NotifyTaskFailed notifies callbacks that a task has failed.
func (a *executionCoordinatorAdapter) NotifyTaskFailed(taskID, reason string) {
	if a.c == nil {
		return
	}
	a.c.notifyTaskFailed(taskID, reason)
}

// NotifyProgress notifies callbacks of progress updates.
func (a *executionCoordinatorAdapter) NotifyProgress() {
	if a.c == nil {
		return
	}
	a.c.notifyProgress()
}

// FinishExecution performs cleanup after execution completes.
func (a *executionCoordinatorAdapter) FinishExecution() {
	if a.c == nil {
		return
	}
	a.c.finishExecution()
}

// AddInstanceToGroup adds an instance to the appropriate ultra-plan group.
func (a *executionCoordinatorAdapter) AddInstanceToGroup(instanceID string, isMultiPass bool) {
	if a.c == nil || a.c.baseSession == nil {
		return
	}
	sessionType := SessionTypeUltraPlan
	if isMultiPass {
		sessionType = SessionTypePlanMulti
	}
	if ultraGroup := a.c.baseSession.GetGroupBySessionType(sessionType); ultraGroup != nil {
		ultraGroup.AddInstance(instanceID)
	}
}

// StartGroupConsolidation starts the group consolidation process.
func (a *executionCoordinatorAdapter) StartGroupConsolidation(groupIndex int) error {
	if a.c == nil {
		return fmt.Errorf("nil coordinator")
	}
	return a.c.startGroupConsolidatorSession(groupIndex)
}

// HandlePartialGroupFailure handles a group with mixed success/failure.
func (a *executionCoordinatorAdapter) HandlePartialGroupFailure(groupIndex int) {
	if a.c == nil {
		return
	}
	a.c.handlePartialGroupFailure(groupIndex)
}

// ClearTaskFromInstance removes the task-to-instance mapping for retry.
func (a *executionCoordinatorAdapter) ClearTaskFromInstance(taskID string) {
	if a.c == nil {
		return
	}
	session := a.c.Session()
	if session != nil {
		a.c.mu.Lock()
		delete(session.TaskToInstance, taskID)
		a.c.mu.Unlock()
	}
}

// SaveSession persists the current session state.
func (a *executionCoordinatorAdapter) SaveSession() error {
	if a.c == nil || a.c.orch == nil {
		return fmt.Errorf("nil coordinator or orchestrator")
	}
	return a.c.orch.SaveSession()
}

// RunSynthesis starts the synthesis phase.
func (a *executionCoordinatorAdapter) RunSynthesis() error {
	if a.c == nil {
		return fmt.Errorf("nil coordinator")
	}
	return a.c.RunSynthesis()
}

// NotifyComplete notifies callbacks of overall completion.
func (a *executionCoordinatorAdapter) NotifyComplete(success bool, summary string) {
	if a.c == nil {
		return
	}
	a.c.notifyComplete(success, summary)
}

// SetSessionPhase sets the session phase.
func (a *executionCoordinatorAdapter) SetSessionPhase(p phase.UltraPlanPhase) {
	if a.c == nil {
		return
	}
	session := a.c.Session()
	if session != nil {
		a.c.mu.Lock()
		session.Phase = UltraPlanPhase(p)
		a.c.mu.Unlock()
	}
}

// SetSessionError sets the session error message.
func (a *executionCoordinatorAdapter) SetSessionError(err string) {
	if a.c == nil {
		return
	}
	session := a.c.Session()
	if session != nil {
		a.c.mu.Lock()
		session.Error = err
		a.c.mu.Unlock()
	}
}

// GetNoSynthesis returns true if synthesis phase should be skipped.
func (a *executionCoordinatorAdapter) GetNoSynthesis() bool {
	if a.c == nil {
		return false
	}
	session := a.c.Session()
	if session == nil {
		return false
	}
	return session.Config.NoSynthesis
}

// RecordTaskCommitCount records the commit count for a completed task.
func (a *executionCoordinatorAdapter) RecordTaskCommitCount(taskID string, count int) {
	if a.c == nil {
		return
	}
	session := a.c.Session()
	if session != nil {
		a.c.mu.Lock()
		if session.TaskCommitCounts == nil {
			session.TaskCommitCounts = make(map[string]int)
		}
		session.TaskCommitCounts[taskID] = count
		a.c.mu.Unlock()
	}
}

// ConsolidateGroupWithVerification consolidates a group and verifies commits exist.
func (a *executionCoordinatorAdapter) ConsolidateGroupWithVerification(groupIndex int) error {
	if a.c == nil {
		return fmt.Errorf("nil coordinator")
	}
	return a.c.consolidateGroupWithVerification(groupIndex)
}

// EmitEvent emits a coordinator event for UI notification.
func (a *executionCoordinatorAdapter) EmitEvent(eventType, message string) {
	if a.c == nil || a.c.manager == nil {
		return
	}
	a.c.manager.emitEvent(CoordinatorEvent{
		Type:    CoordinatorEventType(eventType),
		Message: message,
	})
}

// StartExecutionLoop restarts the execution loop (used by RetriggerGroup).
func (a *executionCoordinatorAdapter) StartExecutionLoop() {
	if a.c == nil {
		return
	}
	a.c.wg.Add(1)
	go a.c.executionLoop()
}

// BuildExecutionContext creates an ExecutionContext for the ExecutionOrchestrator.
// This provides all the adapters needed for full execution phase delegation.
func (c *Coordinator) BuildExecutionContext() (*phase.ExecutionContext, error) {
	phaseCtx, err := c.BuildPhaseContext()
	if err != nil {
		return nil, err
	}

	return &phase.ExecutionContext{
		PhaseContext: phaseCtx,
		Coordinator:  newExecutionCoordinatorAdapter(c),
	}, nil
}

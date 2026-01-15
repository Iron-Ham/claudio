package orchestrator

import (
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
		Logger:       logger,
		Callbacks:    newCoordinatorCallbacksAdapter(c),
	}

	if err := ctx.Validate(); err != nil {
		return nil, err
	}

	return ctx, nil
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

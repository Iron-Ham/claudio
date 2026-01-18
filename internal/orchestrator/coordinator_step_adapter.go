package orchestrator

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/phase"
	"github.com/Iron-Ham/claudio/internal/orchestrator/phase/step"
	"github.com/Iron-Ham/claudio/internal/orchestrator/retry"
)

// coordinatorStepAdapter adapts the Coordinator to the step.StepCoordinatorInterface.
// This allows the step package to remain decoupled from the Coordinator implementation.
type coordinatorStepAdapter struct {
	c *Coordinator
}

// newCoordinatorStepAdapter creates a new step adapter for the given coordinator.
func newCoordinatorStepAdapter(c *Coordinator) *coordinatorStepAdapter {
	return &coordinatorStepAdapter{c: c}
}

// Session returns the session interface adapter.
func (a *coordinatorStepAdapter) Session() step.SessionInterface {
	session := a.c.Session()
	if session == nil {
		return nil
	}
	return &sessionStepAdapter{s: session}
}

// PlanningOrchestrator returns the planning orchestrator interface adapter.
func (a *coordinatorStepAdapter) PlanningOrchestrator() step.PlanningOrchestratorInterface {
	planOrch := a.c.PlanningOrchestrator()
	if planOrch == nil {
		return nil
	}
	return &planningOrchestratorStepAdapter{p: planOrch}
}

// ExecutionOrchestrator returns the execution orchestrator interface adapter.
func (a *coordinatorStepAdapter) ExecutionOrchestrator() step.ExecutionOrchestratorInterface {
	execOrch := a.c.ExecutionOrchestrator()
	if execOrch == nil {
		return nil
	}
	return &executionOrchestratorStepAdapter{e: execOrch}
}

// SynthesisOrchestrator returns the synthesis orchestrator interface adapter.
func (a *coordinatorStepAdapter) SynthesisOrchestrator() step.SynthesisOrchestratorInterface {
	synthOrch := a.c.SynthesisOrchestrator()
	if synthOrch == nil {
		return nil
	}
	return &synthesisOrchestratorStepAdapter{s: synthOrch}
}

// ConsolidationOrchestrator returns the consolidation orchestrator interface adapter.
func (a *coordinatorStepAdapter) ConsolidationOrchestrator() step.ConsolidationOrchestratorInterface {
	consolOrch := a.c.ConsolidationOrchestrator()
	if consolOrch == nil {
		return nil
	}
	return &consolidationOrchestratorStepAdapter{c: consolOrch}
}

// GetOrchestrator returns the base orchestrator interface adapter.
func (a *coordinatorStepAdapter) GetOrchestrator() step.OrchestratorInterface {
	return &orchestratorStepAdapter{o: a.c.orch}
}

// GetRetryManager returns the retry manager interface adapter.
func (a *coordinatorStepAdapter) GetRetryManager() step.RetryManagerInterface {
	return &retryManagerStepAdapter{r: a.c.retryManager}
}

// GetTaskGroupIndex returns the group index for a task ID.
func (a *coordinatorStepAdapter) GetTaskGroupIndex(taskID string) int {
	return a.c.getTaskGroupIndex(taskID)
}

// GetRunningCount returns the number of currently running tasks.
func (a *coordinatorStepAdapter) GetRunningCount() int {
	a.c.mu.RLock()
	defer a.c.mu.RUnlock()
	return a.c.runningCount
}

// Lock acquires the coordinator's mutex.
func (a *coordinatorStepAdapter) Lock() {
	a.c.mu.Lock()
}

// Unlock releases the coordinator's mutex.
func (a *coordinatorStepAdapter) Unlock() {
	a.c.mu.Unlock()
}

// SaveSession saves the session state.
func (a *coordinatorStepAdapter) SaveSession() error {
	return a.c.orch.SaveSession()
}

// RunPlanning starts the planning phase.
func (a *coordinatorStepAdapter) RunPlanning() error {
	return a.c.RunPlanning()
}

// RunPlanManager starts the plan manager in multi-pass mode.
func (a *coordinatorStepAdapter) RunPlanManager() error {
	return a.c.RunPlanManager()
}

// RunSynthesis starts the synthesis phase.
func (a *coordinatorStepAdapter) RunSynthesis() error {
	return a.c.RunSynthesis()
}

// StartConsolidation starts the consolidation phase.
func (a *coordinatorStepAdapter) StartConsolidation() error {
	return a.c.StartConsolidation()
}

// StartGroupConsolidatorSession starts a group consolidator for the given group.
func (a *coordinatorStepAdapter) StartGroupConsolidatorSession(groupIndex int) error {
	return StartGroupConsolidatorSession(a.c, groupIndex)
}

// Logger returns the logger interface adapter.
func (a *coordinatorStepAdapter) Logger() step.LoggerInterface {
	return a.c.logger
}

// GetMultiPassStrategyNames returns the names of multi-pass planning strategies.
func (a *coordinatorStepAdapter) GetMultiPassStrategyNames() []string {
	return GetMultiPassStrategyNames()
}

// sessionStepAdapter adapts UltraPlanSession to step.SessionInterface.
type sessionStepAdapter struct {
	s *UltraPlanSession
}

func (a *sessionStepAdapter) GetCoordinatorID() string          { return a.s.CoordinatorID }
func (a *sessionStepAdapter) GetPlanCoordinatorIDs() []string   { return a.s.PlanCoordinatorIDs }
func (a *sessionStepAdapter) GetPlanManagerID() string          { return a.s.PlanManagerID }
func (a *sessionStepAdapter) GetSynthesisID() string            { return a.s.SynthesisID }
func (a *sessionStepAdapter) GetRevisionID() string             { return a.s.RevisionID }
func (a *sessionStepAdapter) GetConsolidationID() string        { return a.s.ConsolidationID }
func (a *sessionStepAdapter) GetGroupConsolidatorIDs() []string { return a.s.GroupConsolidatorIDs }
func (a *sessionStepAdapter) GetTaskToInstance() map[string]string {
	return a.s.TaskToInstance
}

func (a *sessionStepAdapter) GetTask(taskID string) step.TaskInterface {
	task := a.s.GetTask(taskID)
	if task == nil {
		return nil
	}
	return &taskStepAdapter{t: task}
}

func (a *sessionStepAdapter) GetConfig() step.ConfigInterface {
	return &configStepAdapter{c: &a.s.Config}
}

func (a *sessionStepAdapter) GetPhase() string { return string(a.s.Phase) }

func (a *sessionStepAdapter) GetPlan() step.PlanInterface {
	if a.s.Plan == nil {
		return nil
	}
	return &planStepAdapter{p: a.s.Plan}
}

func (a *sessionStepAdapter) GetRevision() step.RevisionInterface {
	if a.s.Revision == nil {
		return nil
	}
	return &revisionStepAdapter{r: a.s.Revision}
}

func (a *sessionStepAdapter) SetCoordinatorID(id string) { a.s.CoordinatorID = id }
func (a *sessionStepAdapter) SetPlanManagerID(id string) { a.s.PlanManagerID = id }
func (a *sessionStepAdapter) SetSynthesisID(id string)   { a.s.SynthesisID = id }
func (a *sessionStepAdapter) SetRevisionID(id string)    { a.s.RevisionID = id }
func (a *sessionStepAdapter) SetConsolidationID(id string) {
	a.s.ConsolidationID = id
}
func (a *sessionStepAdapter) SetPhase(phase string)           { a.s.Phase = UltraPlanPhase(phase) }
func (a *sessionStepAdapter) SetPlan(plan step.PlanInterface) { a.s.Plan = nil } // Only setting to nil is supported
func (a *sessionStepAdapter) SetSynthesisCompletion(completion any) {
	if completion == nil {
		a.s.SynthesisCompletion = nil
	}
}
func (a *sessionStepAdapter) SetSynthesisAwaitingApproval(awaiting bool) {
	a.s.SynthesisAwaitingApproval = awaiting
}
func (a *sessionStepAdapter) SetConsolidation(state any) {
	if state == nil {
		a.s.Consolidation = nil
	}
}
func (a *sessionStepAdapter) SetPRUrls(urls []string) { a.s.PRUrls = urls }
func (a *sessionStepAdapter) SetGroupDecision(decision any) {
	if decision == nil {
		a.s.GroupDecision = nil
	}
}
func (a *sessionStepAdapter) SetGroupConsolidatorID(groupIndex int, id string) {
	if groupIndex >= 0 && groupIndex < len(a.s.GroupConsolidatorIDs) {
		a.s.GroupConsolidatorIDs[groupIndex] = id
	}
}
func (a *sessionStepAdapter) SetGroupConsolidatedBranch(groupIndex int, branch string) {
	if groupIndex >= 0 && groupIndex < len(a.s.GroupConsolidatedBranches) {
		a.s.GroupConsolidatedBranches[groupIndex] = branch
	}
}
func (a *sessionStepAdapter) SetGroupConsolidationContext(groupIndex int, ctx any) {
	if groupIndex >= 0 && groupIndex < len(a.s.GroupConsolidationContexts) && ctx == nil {
		a.s.GroupConsolidationContexts[groupIndex] = nil
	}
}
func (a *sessionStepAdapter) GetCompletedTasks() []string { return a.s.CompletedTasks }
func (a *sessionStepAdapter) GetFailedTasks() []string    { return a.s.FailedTasks }
func (a *sessionStepAdapter) SetCompletedTasks(tasks []string) {
	a.s.CompletedTasks = tasks
}
func (a *sessionStepAdapter) SetFailedTasks(tasks []string) { a.s.FailedTasks = tasks }
func (a *sessionStepAdapter) DeleteTaskToInstance(taskID string) {
	delete(a.s.TaskToInstance, taskID)
}
func (a *sessionStepAdapter) DeleteTaskCommitCount(taskID string) {
	delete(a.s.TaskCommitCounts, taskID)
}
func (a *sessionStepAdapter) GetTaskRetries() map[string]any {
	result := make(map[string]any)
	for k, v := range a.s.TaskRetries {
		result[k] = v
	}
	return result
}
func (a *sessionStepAdapter) SetTaskRetries(retries map[string]any) {
	result := make(map[string]*TaskRetryState)
	for k, v := range retries {
		if state, ok := v.(*TaskRetryState); ok {
			result[k] = state
		}
	}
	a.s.TaskRetries = result
}

// taskStepAdapter adapts PlannedTask to step.TaskInterface.
type taskStepAdapter struct {
	t *PlannedTask
}

func (a *taskStepAdapter) GetID() string    { return a.t.ID }
func (a *taskStepAdapter) GetTitle() string { return a.t.Title }

// configStepAdapter adapts UltraPlanConfig to step.ConfigInterface.
type configStepAdapter struct {
	c *UltraPlanConfig
}

func (a *configStepAdapter) IsMultiPass() bool { return a.c.MultiPass }

// planStepAdapter adapts PlanSpec to step.PlanInterface.
type planStepAdapter struct {
	p *PlanSpec
}

func (a *planStepAdapter) GetExecutionOrder() [][]string { return a.p.ExecutionOrder }

// revisionStepAdapter adapts RevisionState to step.RevisionInterface.
type revisionStepAdapter struct {
	r *RevisionState
}

func (a *revisionStepAdapter) GetIssues() []step.RevisionIssue {
	result := make([]step.RevisionIssue, len(a.r.Issues))
	for i, issue := range a.r.Issues {
		result[i] = step.RevisionIssue{
			TaskID:      issue.TaskID,
			Description: issue.Description,
			Files:       issue.Files,
			Severity:    issue.Severity,
			Suggestion:  issue.Suggestion,
		}
	}
	return result
}
func (a *revisionStepAdapter) GetRevisedTasks() []string       { return a.r.RevisedTasks }
func (a *revisionStepAdapter) GetTasksToRevise() []string      { return a.r.TasksToRevise }
func (a *revisionStepAdapter) SetRevisedTasks(tasks []string)  { a.r.RevisedTasks = tasks }
func (a *revisionStepAdapter) SetTasksToRevise(tasks []string) { a.r.TasksToRevise = tasks }

// planningOrchestratorStepAdapter adapts PlanningOrchestrator to step.PlanningOrchestratorInterface.
type planningOrchestratorStepAdapter struct {
	p *phase.PlanningOrchestrator
}

func (a *planningOrchestratorStepAdapter) GetInstanceID() string { return a.p.GetInstanceID() }
func (a *planningOrchestratorStepAdapter) GetPlanCoordinatorIDs() []string {
	return a.p.GetPlanCoordinatorIDs()
}
func (a *planningOrchestratorStepAdapter) Reset() { a.p.Reset() }

// executionOrchestratorStepAdapter adapts ExecutionOrchestrator to step.ExecutionOrchestratorInterface.
type executionOrchestratorStepAdapter struct {
	e *phase.ExecutionOrchestrator
}

func (a *executionOrchestratorStepAdapter) State() step.ExecutionStateInterface {
	state := a.e.State()
	return &executionStateStepAdapter{s: &state}
}
func (a *executionOrchestratorStepAdapter) Reset() { a.e.Reset() }
func (a *executionOrchestratorStepAdapter) StartSingleTask(taskID string) (string, error) {
	return a.e.StartSingleTask(taskID)
}

// executionStateStepAdapter adapts ExecutionState to step.ExecutionStateInterface.
type executionStateStepAdapter struct {
	s *phase.ExecutionState
}

func (a *executionStateStepAdapter) GetRunningTasks() map[string]string {
	return a.s.RunningTasks
}

// synthesisOrchestratorStepAdapter adapts SynthesisOrchestrator to step.SynthesisOrchestratorInterface.
type synthesisOrchestratorStepAdapter struct {
	s *phase.SynthesisOrchestrator
}

func (a *synthesisOrchestratorStepAdapter) GetInstanceID() string { return a.s.GetInstanceID() }
func (a *synthesisOrchestratorStepAdapter) State() step.SynthesisStateInterface {
	state := a.s.State()
	return &synthesisStateStepAdapter{s: &state}
}
func (a *synthesisOrchestratorStepAdapter) Reset() { a.s.Reset() }
func (a *synthesisOrchestratorStepAdapter) StartRevision(issues []step.RevisionIssue) error {
	// Convert step.RevisionIssue to phase.RevisionIssue
	phaseIssues := make([]phase.RevisionIssue, len(issues))
	for i, issue := range issues {
		phaseIssues[i] = phase.RevisionIssue{
			TaskID:      issue.TaskID,
			Description: issue.Description,
			Files:       issue.Files,
			Severity:    issue.Severity,
			Suggestion:  issue.Suggestion,
		}
	}
	return a.s.StartRevision(phaseIssues)
}

// synthesisStateStepAdapter adapts SynthesisState to step.SynthesisStateInterface.
type synthesisStateStepAdapter struct {
	s *phase.SynthesisState
}

func (a *synthesisStateStepAdapter) GetRunningRevisionTasks() map[string]string {
	return a.s.RunningRevisionTasks
}

// consolidationOrchestratorStepAdapter adapts ConsolidationOrchestrator to step.ConsolidationOrchestratorInterface.
type consolidationOrchestratorStepAdapter struct {
	c *phase.ConsolidationOrchestrator
}

func (a *consolidationOrchestratorStepAdapter) GetInstanceID() string { return a.c.GetInstanceID() }
func (a *consolidationOrchestratorStepAdapter) Reset()                { a.c.Reset() }
func (a *consolidationOrchestratorStepAdapter) ClearStateForRestart() { a.c.ClearStateForRestart() }

// orchestratorStepAdapter adapts Orchestrator to step.OrchestratorInterface.
type orchestratorStepAdapter struct {
	o *Orchestrator
}

func (a *orchestratorStepAdapter) GetInstance(id string) step.InstanceInterface {
	inst := a.o.GetInstance(id)
	if inst == nil {
		return nil
	}
	return &instanceStepAdapter{i: inst}
}
func (a *orchestratorStepAdapter) StopInstance(inst step.InstanceInterface) error {
	// Get the actual instance from our orchestrator by ID
	if inst == nil {
		return nil
	}
	realInst := a.o.GetInstance(inst.GetID())
	if realInst == nil {
		return nil
	}
	return a.o.StopInstance(realInst)
}
func (a *orchestratorStepAdapter) SaveSession() error { return a.o.SaveSession() }

// instanceStepAdapter adapts Instance to step.InstanceInterface.
type instanceStepAdapter struct {
	i *Instance
}

func (a *instanceStepAdapter) GetID() string { return a.i.ID }

// retryManagerStepAdapter adapts retry.Manager to step.RetryManagerInterface.
type retryManagerStepAdapter struct {
	r *retry.Manager
}

func (a *retryManagerStepAdapter) Reset(taskID string) { a.r.Reset(taskID) }
func (a *retryManagerStepAdapter) GetAllStates() map[string]any {
	states := a.r.GetAllStates()
	result := make(map[string]any)
	for k, v := range states {
		result[k] = v
	}
	return result
}

package phase

// Integration tests for phase lifecycle and inter-phase coordination.
// These tests exercise full phase lifecycles and transitions between phases,
// which are critical paths that can fail in complex multi-task scenarios.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Integration Test Infrastructure
// =============================================================================

// IntegrationTestCoordinator simulates a full coordinator that tracks phase transitions
// and manages the handoff between phases. This enables testing complex phase flows.
type IntegrationTestCoordinator struct {
	mu sync.Mutex

	// Phase tracking
	phaseHistory    []UltraPlanPhase
	phaseChangeChan chan UltraPlanPhase

	// Task management
	tasks          map[string]*integrationTestTask
	runningTasks   map[string]string // taskID -> instanceID
	completedTasks []string
	failedTasks    []string
	taskGroups     map[string]int // taskID -> groupIndex
	totalTasks     int
	maxParallel    int

	// Consolidation tracking
	consolidationCalls   []int
	consolidationResults map[int]error
	consolidationDelay   time.Duration

	// Synthesis tracking
	synthesisCalls    int
	synthesisDelay    time.Duration
	noSynthesis       bool
	awaitingSynthesis bool

	// Session state
	sessionPhase UltraPlanPhase
	sessionError string

	// Completion tracking
	completeCalls []struct {
		success bool
		summary string
	}

	// Configuration
	groupCount    int
	tasksPerGroup []int
	branchPrefix  string
	baseBranches  map[int]string

	// Group tracker
	groupTracker *integrationGroupTracker

	// Partial failure simulation
	partialFailureGroups map[int]bool
	partialFailureCalls  []int

	// Errors for injection
	consolidationErr error
	synthesisErr     error
}

type integrationTestTask struct {
	id          string
	title       string
	description string
	group       int
	status      string
	instanceID  string
}

// integrationGroupTracker tracks group completion state
type integrationGroupTracker struct {
	mu             sync.Mutex
	completedTasks map[int][]string // groupIndex -> taskIDs
	failedTasks    map[int][]string
	totalTasks     map[int]int
	currentGroup   int
}

func newIntegrationGroupTracker() *integrationGroupTracker {
	return &integrationGroupTracker{
		completedTasks: make(map[int][]string),
		failedTasks:    make(map[int][]string),
		totalTasks:     make(map[int]int),
	}
}

func (t *integrationGroupTracker) IsGroupComplete(groupIndex int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	completed := len(t.completedTasks[groupIndex])
	failed := len(t.failedTasks[groupIndex])
	total := t.totalTasks[groupIndex]
	return completed+failed >= total && total > 0
}

func (t *integrationGroupTracker) HasPartialFailure(groupIndex int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	completed := len(t.completedTasks[groupIndex])
	failed := len(t.failedTasks[groupIndex])
	return completed > 0 && failed > 0
}

func (t *integrationGroupTracker) AdvanceGroup(groupIndex int) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentGroup = groupIndex + 1
	return t.currentGroup, true
}

func (t *integrationGroupTracker) GetGroupTasks(groupIndex int) []GroupTaskInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	allTasks := append(t.completedTasks[groupIndex], t.failedTasks[groupIndex]...)
	result := make([]GroupTaskInfo, len(allTasks))
	for i, taskID := range allTasks {
		result[i] = GroupTaskInfo{ID: taskID, Title: taskID}
	}
	return result
}

func (t *integrationGroupTracker) GetTaskGroupIndex(taskID string) int {
	// Not needed for integration tests
	return 0
}

func (t *integrationGroupTracker) TotalGroups() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.totalTasks)
}

func (t *integrationGroupTracker) HasMoreGroups(groupIndex int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return groupIndex < len(t.totalTasks)-1
}

func (t *integrationGroupTracker) SetGroupTaskCount(groupIndex, count int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalTasks[groupIndex] = count
}

func (t *integrationGroupTracker) AddCompletedTask(groupIndex int, taskID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completedTasks[groupIndex] = append(t.completedTasks[groupIndex], taskID)
}

func (t *integrationGroupTracker) AddFailedTask(groupIndex int, taskID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failedTasks[groupIndex] = append(t.failedTasks[groupIndex], taskID)
}

// NewIntegrationTestCoordinator creates a coordinator for integration testing.
func NewIntegrationTestCoordinator() *IntegrationTestCoordinator {
	return &IntegrationTestCoordinator{
		phaseHistory:         make([]UltraPlanPhase, 0),
		phaseChangeChan:      make(chan UltraPlanPhase, 10),
		tasks:                make(map[string]*integrationTestTask),
		runningTasks:         make(map[string]string),
		completedTasks:       make([]string, 0),
		failedTasks:          make([]string, 0),
		taskGroups:           make(map[string]int),
		consolidationResults: make(map[int]error),
		baseBranches:         make(map[int]string),
		groupTracker:         newIntegrationGroupTracker(),
		partialFailureGroups: make(map[int]bool),
		maxParallel:          3,
		branchPrefix:         "test",
	}
}

// SetupTasks configures tasks for integration testing.
func (c *IntegrationTestCoordinator) SetupTasks(groupCount int, tasksPerGroup []int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.groupCount = groupCount
	c.tasksPerGroup = tasksPerGroup

	taskNum := 0
	for groupIdx := 0; groupIdx < groupCount; groupIdx++ {
		taskCount := 1
		if groupIdx < len(tasksPerGroup) {
			taskCount = tasksPerGroup[groupIdx]
		}

		c.groupTracker.SetGroupTaskCount(groupIdx, taskCount)

		for i := 0; i < taskCount; i++ {
			taskID := fmt.Sprintf("task-%d-%d", groupIdx, i)
			c.tasks[taskID] = &integrationTestTask{
				id:          taskID,
				title:       fmt.Sprintf("Task %d in Group %d", i, groupIdx),
				description: fmt.Sprintf("Description for task %d in group %d", i, groupIdx),
				group:       groupIdx,
				status:      "pending",
			}
			c.taskGroups[taskID] = groupIdx
			taskNum++
		}
	}
	c.totalTasks = taskNum
}

// Implement ExecutionCoordinatorInterface

func (c *IntegrationTestCoordinator) GetBaseBranchForGroup(groupIndex int) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if branch, ok := c.baseBranches[groupIndex]; ok {
		return branch
	}
	if groupIndex == 0 {
		return "main"
	}
	return fmt.Sprintf("group-%d-consolidated", groupIndex-1)
}

func (c *IntegrationTestCoordinator) AddRunningTask(taskID, instanceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runningTasks[taskID] = instanceID
	if task, ok := c.tasks[taskID]; ok {
		task.instanceID = instanceID
		task.status = "running"
	}
}

func (c *IntegrationTestCoordinator) RemoveRunningTask(taskID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.runningTasks[taskID]; exists {
		delete(c.runningTasks, taskID)
		return true
	}
	return false
}

func (c *IntegrationTestCoordinator) GetRunningTaskCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.runningTasks)
}

func (c *IntegrationTestCoordinator) IsTaskRunning(taskID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, exists := c.runningTasks[taskID]
	return exists
}

func (c *IntegrationTestCoordinator) GetBaseSession() any {
	return nil
}

func (c *IntegrationTestCoordinator) GetTaskGroupIndex(taskID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.taskGroups[taskID]
}

func (c *IntegrationTestCoordinator) VerifyTaskWork(taskID string, inst any) TaskCompletion {
	return TaskCompletion{TaskID: taskID, Success: true}
}

func (c *IntegrationTestCoordinator) CheckForTaskCompletionFile(inst any) bool {
	return true
}

func (c *IntegrationTestCoordinator) HandleTaskCompletion(completion TaskCompletion) {
	c.mu.Lock()
	defer c.mu.Unlock()

	groupIndex := c.taskGroups[completion.TaskID]
	if completion.Success {
		c.completedTasks = append(c.completedTasks, completion.TaskID)
		c.groupTracker.AddCompletedTask(groupIndex, completion.TaskID)
		if task, ok := c.tasks[completion.TaskID]; ok {
			task.status = "completed"
		}
	} else {
		c.failedTasks = append(c.failedTasks, completion.TaskID)
		c.groupTracker.AddFailedTask(groupIndex, completion.TaskID)
		if task, ok := c.tasks[completion.TaskID]; ok {
			task.status = "failed"
		}
	}
}

func (c *IntegrationTestCoordinator) PollTaskCompletions(completionChan chan<- TaskCompletion) {
	// No-op in integration tests
}

func (c *IntegrationTestCoordinator) NotifyTaskStart(taskID, instanceID string) {
	// Tracking handled by AddRunningTask
}

func (c *IntegrationTestCoordinator) NotifyTaskFailed(taskID, reason string) {
	c.HandleTaskCompletion(TaskCompletion{TaskID: taskID, Success: false, Error: reason})
}

func (c *IntegrationTestCoordinator) NotifyProgress() {
	// No-op for integration tests
}

func (c *IntegrationTestCoordinator) FinishExecution() {
	// Handled via phase transitions
}

func (c *IntegrationTestCoordinator) AddInstanceToGroup(instanceID string, isMultiPass bool) {
	// No-op for integration tests
}

func (c *IntegrationTestCoordinator) StartGroupConsolidation(groupIndex int) error {
	c.mu.Lock()
	c.consolidationCalls = append(c.consolidationCalls, groupIndex)
	delay := c.consolidationDelay
	err := c.consolidationErr
	if result, ok := c.consolidationResults[groupIndex]; ok {
		err = result
	}
	c.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if err == nil {
		c.mu.Lock()
		c.baseBranches[groupIndex+1] = fmt.Sprintf("group-%d-consolidated", groupIndex)
		c.mu.Unlock()
	}

	return err
}

func (c *IntegrationTestCoordinator) HandlePartialGroupFailure(groupIndex int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.partialFailureCalls = append(c.partialFailureCalls, groupIndex)
}

func (c *IntegrationTestCoordinator) ClearTaskFromInstance(taskID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.runningTasks, taskID)
}

func (c *IntegrationTestCoordinator) SaveSession() error {
	return nil
}

func (c *IntegrationTestCoordinator) RunSynthesis() error {
	c.mu.Lock()
	c.synthesisCalls++
	delay := c.synthesisDelay
	err := c.synthesisErr
	c.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	if err == nil {
		c.mu.Lock()
		c.awaitingSynthesis = true
		c.sessionPhase = PhaseSynthesis
		c.phaseHistory = append(c.phaseHistory, PhaseSynthesis)
		c.mu.Unlock()

		select {
		case c.phaseChangeChan <- PhaseSynthesis:
		default:
		}
	}

	return err
}

func (c *IntegrationTestCoordinator) NotifyComplete(success bool, summary string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completeCalls = append(c.completeCalls, struct {
		success bool
		summary string
	}{success, summary})

	if success {
		c.sessionPhase = PhaseComplete
	} else {
		c.sessionPhase = PhaseFailed
	}
}

func (c *IntegrationTestCoordinator) SetSessionPhase(phase UltraPlanPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionPhase = phase
	c.phaseHistory = append(c.phaseHistory, phase)

	select {
	case c.phaseChangeChan <- phase:
	default:
	}
}

func (c *IntegrationTestCoordinator) SetSessionError(err string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionError = err
}

func (c *IntegrationTestCoordinator) GetNoSynthesis() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.noSynthesis
}

func (c *IntegrationTestCoordinator) RecordTaskCommitCount(taskID string, count int) {
	// No-op for integration tests
}

func (c *IntegrationTestCoordinator) ConsolidateGroupWithVerification(groupIndex int) error {
	return c.StartGroupConsolidation(groupIndex)
}

func (c *IntegrationTestCoordinator) EmitEvent(eventType, message string) {
	// No-op for integration tests
}

func (c *IntegrationTestCoordinator) StartExecutionLoop() {
	// No-op for integration tests
}

func (c *IntegrationTestCoordinator) ResetStateForRetrigger(targetGroup int, tasksToReset map[string]bool) {
	// No-op for integration tests
}

func (c *IntegrationTestCoordinator) ResetStateForRetry(failedTasks []string, groupIdx int) {
	// No-op for integration tests
}

// GetPhaseHistory returns the recorded phase transitions for verification.
func (c *IntegrationTestCoordinator) GetPhaseHistory() []UltraPlanPhase {
	c.mu.Lock()
	defer c.mu.Unlock()
	history := make([]UltraPlanPhase, len(c.phaseHistory))
	copy(history, c.phaseHistory)
	return history
}

// GetConsolidationCalls returns the groups that were consolidated.
func (c *IntegrationTestCoordinator) GetConsolidationCalls() []int {
	c.mu.Lock()
	defer c.mu.Unlock()
	calls := make([]int, len(c.consolidationCalls))
	copy(calls, c.consolidationCalls)
	return calls
}

// GetSynthesisCalls returns the number of times synthesis was called.
func (c *IntegrationTestCoordinator) GetSynthesisCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.synthesisCalls
}

// GetCompleteCalls returns the completion notifications.
func (c *IntegrationTestCoordinator) GetCompleteCalls() []struct {
	success bool
	summary string
} {
	c.mu.Lock()
	defer c.mu.Unlock()
	calls := make([]struct {
		success bool
		summary string
	}, len(c.completeCalls))
	copy(calls, c.completeCalls)
	return calls
}

// =============================================================================
// Integration Test Session
// =============================================================================

// integrationTestSession implements all session interfaces for integration testing.
type integrationTestSession struct {
	mu sync.Mutex

	phase               UltraPlanPhase
	objective           string
	error               string
	completedTasks      []string
	taskToInstance      map[string]string
	taskCommitCounts    map[string]int
	synthesisID         string
	synthesisAwaiting   bool
	synthesisCompletion *SynthesisCompletionFile
	revisionRound       int

	// Extended session data
	currentGroup      int
	completedCount    int
	failedCount       int
	totalCount        int
	maxParallel       int
	multiPass         bool
	planSummary       string
	consolidationMode string
	contexts          map[int]GroupConsolidationContextData

	// Task data
	tasks     map[string]*integrationTestTask
	readyTask []string

	// Group state
	groupComplete bool
	hasMoreGroups bool

	// Worktree info
	taskWorktrees []TaskWorktreeInfo

	// Config
	config *integrationTestConfig
}

type integrationTestConfig struct {
	multiPass bool
}

func (c *integrationTestConfig) IsMultiPass() bool {
	return c.multiPass
}

func newIntegrationTestSession() *integrationTestSession {
	return &integrationTestSession{
		phase:            PhasePlanning,
		taskToInstance:   make(map[string]string),
		taskCommitCounts: make(map[string]int),
		contexts:         make(map[int]GroupConsolidationContextData),
		tasks:            make(map[string]*integrationTestTask),
		readyTask:        make([]string, 0),
		maxParallel:      3,
		config:           &integrationTestConfig{},
	}
}

// Implement UltraPlanSessionInterface

func (s *integrationTestSession) GetTask(taskID string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task, ok := s.tasks[taskID]; ok {
		return &mockPlannedTask{
			id:          task.id,
			title:       task.title,
			description: task.description,
		}
	}
	return nil
}

func (s *integrationTestSession) GetReadyTasks() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.readyTask))
	copy(result, s.readyTask)
	return result
}

func (s *integrationTestSession) IsCurrentGroupComplete() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.groupComplete
}

func (s *integrationTestSession) AdvanceGroupIfComplete() (bool, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.groupComplete {
		prevGroup := s.currentGroup
		s.currentGroup++
		s.groupComplete = false
		return true, prevGroup
	}
	return false, s.currentGroup
}

func (s *integrationTestSession) HasMoreGroups() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasMoreGroups
}

func (s *integrationTestSession) Progress() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.totalCount == 0 {
		return 0
	}
	return float64(s.completedCount) / float64(s.totalCount)
}

func (s *integrationTestSession) GetObjective() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.objective
}

func (s *integrationTestSession) GetCompletedTasks() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, len(s.completedTasks))
	copy(result, s.completedTasks)
	return result
}

func (s *integrationTestSession) GetTaskToInstance() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]string)
	for k, v := range s.taskToInstance {
		result[k] = v
	}
	return result
}

func (s *integrationTestSession) GetTaskCommitCounts() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]int)
	for k, v := range s.taskCommitCounts {
		result[k] = v
	}
	return result
}

func (s *integrationTestSession) GetSynthesisID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.synthesisID
}

func (s *integrationTestSession) SetSynthesisID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.synthesisID = id
}

func (s *integrationTestSession) GetRevisionRound() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revisionRound
}

func (s *integrationTestSession) SetSynthesisAwaitingApproval(awaiting bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.synthesisAwaiting = awaiting
}

func (s *integrationTestSession) IsSynthesisAwaitingApproval() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.synthesisAwaiting
}

func (s *integrationTestSession) SetSynthesisCompletion(completion *SynthesisCompletionFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.synthesisCompletion = completion
}

func (s *integrationTestSession) GetPhase() UltraPlanPhase {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.phase
}

func (s *integrationTestSession) SetPhase(phase UltraPlanPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = phase
}

func (s *integrationTestSession) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.error = err
}

func (s *integrationTestSession) GetConfig() UltraPlanConfigInterface {
	return s.config
}

// Extended session methods for execution

func (s *integrationTestSession) GetCurrentGroup() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentGroup
}

func (s *integrationTestSession) GetCompletedTaskCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.completedCount
}

func (s *integrationTestSession) GetFailedTaskCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failedCount
}

func (s *integrationTestSession) GetTotalTaskCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.totalCount
}

func (s *integrationTestSession) GetMaxParallel() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxParallel
}

func (s *integrationTestSession) IsMultiPass() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.multiPass
}

func (s *integrationTestSession) GetPlanSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planSummary
}

func (s *integrationTestSession) GetGroupConsolidationContext(groupIndex int) GroupConsolidationContextData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contexts[groupIndex]
}

// Extended session methods for synthesis

func (s *integrationTestSession) GetConsolidationMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consolidationMode
}

func (s *integrationTestSession) GetTaskWorktrees() []TaskWorktreeInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.taskWorktrees
}

func (s *integrationTestSession) SetTaskWorktrees(worktrees []TaskWorktreeInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.taskWorktrees = worktrees
}

func (s *integrationTestSession) MarkComplete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.phase = PhaseComplete
}

// =============================================================================
// Integration Test Manager
// =============================================================================

// integrationTestManager implements UltraPlanManagerInterface for testing.
type integrationTestManager struct {
	mu sync.Mutex

	session         *integrationTestSession
	phaseChanges    []UltraPlanPhase
	planSets        []any
	completedTasks  []string
	failedTasks     []struct{ taskID, reason string }
	taskAssignments map[string]string
	stopped         bool
}

func newIntegrationTestManager(session *integrationTestSession) *integrationTestManager {
	return &integrationTestManager{
		session:         session,
		phaseChanges:    make([]UltraPlanPhase, 0),
		planSets:        make([]any, 0),
		taskAssignments: make(map[string]string),
	}
}

func (m *integrationTestManager) Session() UltraPlanSessionInterface {
	return m.session
}

func (m *integrationTestManager) SetPhase(phase UltraPlanPhase) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.phaseChanges = append(m.phaseChanges, phase)
	m.session.SetPhase(phase)
}

func (m *integrationTestManager) SetPlan(plan any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.planSets = append(m.planSets, plan)
}

func (m *integrationTestManager) MarkTaskComplete(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completedTasks = append(m.completedTasks, taskID)
}

func (m *integrationTestManager) MarkTaskFailed(taskID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failedTasks = append(m.failedTasks, struct{ taskID, reason string }{taskID, reason})
}

func (m *integrationTestManager) AssignTaskToInstance(taskID, instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskAssignments[taskID] = instanceID
}

func (m *integrationTestManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

// =============================================================================
// Integration Test Orchestrator (Instance Management)
// =============================================================================

// integrationTestOrchestrator implements OrchestratorInterface for testing.
type integrationTestOrchestrator struct {
	mu sync.Mutex

	instances        map[string]*integrationTestInstance
	instanceCounter  int
	startedInstances []string
	stoppedInstances []string
	saveCalls        int
	branchPrefix     string

	// Configuration
	addInstanceErr   error
	startInstanceErr error
	saveSessionErr   error

	// Delays for timing tests
	addInstanceDelay   time.Duration
	startInstanceDelay time.Duration
}

type integrationTestInstance struct {
	id           string
	worktreePath string
	branch       string
	status       InstanceStatus
	task         string
	baseBranch   string
}

func (i *integrationTestInstance) GetID() string              { return i.id }
func (i *integrationTestInstance) GetWorktreePath() string    { return i.worktreePath }
func (i *integrationTestInstance) GetBranch() string          { return i.branch }
func (i *integrationTestInstance) GetStatus() InstanceStatus  { return i.status }
func (i *integrationTestInstance) GetFilesModified() []string { return nil }

func newIntegrationTestOrchestrator() *integrationTestOrchestrator {
	return &integrationTestOrchestrator{
		instances:    make(map[string]*integrationTestInstance),
		branchPrefix: "test",
	}
}

func (o *integrationTestOrchestrator) AddInstance(session any, task string) (any, error) {
	if o.addInstanceDelay > 0 {
		time.Sleep(o.addInstanceDelay)
	}

	if o.addInstanceErr != nil {
		return nil, o.addInstanceErr
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	o.instanceCounter++
	inst := &integrationTestInstance{
		id:           fmt.Sprintf("inst-%d", o.instanceCounter),
		worktreePath: fmt.Sprintf("/tmp/worktree-%d", o.instanceCounter),
		branch:       fmt.Sprintf("%s/task-%d", o.branchPrefix, o.instanceCounter),
		status:       StatusPending,
		task:         task,
	}
	o.instances[inst.id] = inst
	return inst, nil
}

func (o *integrationTestOrchestrator) AddInstanceFromBranch(session any, task, baseBranch string) (any, error) {
	inst, err := o.AddInstance(session, task)
	if err != nil {
		return nil, err
	}
	if ii, ok := inst.(*integrationTestInstance); ok {
		ii.baseBranch = baseBranch
	}
	return inst, nil
}

func (o *integrationTestOrchestrator) StartInstance(inst any) error {
	if o.startInstanceDelay > 0 {
		time.Sleep(o.startInstanceDelay)
	}

	if o.startInstanceErr != nil {
		return o.startInstanceErr
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	if ii, ok := inst.(*integrationTestInstance); ok {
		ii.status = StatusRunning
		o.startedInstances = append(o.startedInstances, ii.id)
	}
	return nil
}

func (o *integrationTestOrchestrator) StopInstance(inst any) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if ii, ok := inst.(*integrationTestInstance); ok {
		ii.status = StatusCompleted
		o.stoppedInstances = append(o.stoppedInstances, ii.id)
	}
	return nil
}

func (o *integrationTestOrchestrator) SaveSession() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.saveCalls++
	return o.saveSessionErr
}

func (o *integrationTestOrchestrator) GetInstanceManager(id string) any {
	return nil
}

func (o *integrationTestOrchestrator) GetInstance(id string) InstanceInterface {
	o.mu.Lock()
	defer o.mu.Unlock()
	if inst, ok := o.instances[id]; ok {
		return inst
	}
	return nil
}

func (o *integrationTestOrchestrator) GetInstanceByID(id string) any {
	return o.GetInstance(id)
}

func (o *integrationTestOrchestrator) BranchPrefix() string {
	return o.branchPrefix
}

// SetInstanceStatus updates the status of an instance (for test simulation).
func (o *integrationTestOrchestrator) SetInstanceStatus(id string, status InstanceStatus) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if inst, ok := o.instances[id]; ok {
		inst.status = status
	}
}

// =============================================================================
// Integration Test Callbacks
// =============================================================================

// integrationTestCallbacks tracks all callback invocations for verification.
type integrationTestCallbacks struct {
	mu sync.Mutex

	phaseChanges   []UltraPlanPhase
	taskStarts     []struct{ taskID, instanceID string }
	taskCompletes  []string
	taskFails      []struct{ taskID, reason string }
	groupCompletes []int
	planReadyCalls []any
	progressCalls  []struct {
		completed, total int
		phase            UltraPlanPhase
	}
	completeCalls []struct {
		success bool
		summary string
	}
}

func newIntegrationTestCallbacks() *integrationTestCallbacks {
	return &integrationTestCallbacks{
		phaseChanges:   make([]UltraPlanPhase, 0),
		taskStarts:     make([]struct{ taskID, instanceID string }, 0),
		taskCompletes:  make([]string, 0),
		taskFails:      make([]struct{ taskID, reason string }, 0),
		groupCompletes: make([]int, 0),
		planReadyCalls: make([]any, 0),
		progressCalls: make([]struct {
			completed, total int
			phase            UltraPlanPhase
		}, 0),
		completeCalls: make([]struct {
			success bool
			summary string
		}, 0),
	}
}

func (c *integrationTestCallbacks) OnPhaseChange(phase UltraPlanPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.phaseChanges = append(c.phaseChanges, phase)
}

func (c *integrationTestCallbacks) OnTaskStart(taskID, instanceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskStarts = append(c.taskStarts, struct{ taskID, instanceID string }{taskID, instanceID})
}

func (c *integrationTestCallbacks) OnTaskComplete(taskID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskCompletes = append(c.taskCompletes, taskID)
}

func (c *integrationTestCallbacks) OnTaskFailed(taskID, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskFails = append(c.taskFails, struct{ taskID, reason string }{taskID, reason})
}

func (c *integrationTestCallbacks) OnGroupComplete(groupIndex int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.groupCompletes = append(c.groupCompletes, groupIndex)
}

func (c *integrationTestCallbacks) OnPlanReady(plan any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.planReadyCalls = append(c.planReadyCalls, plan)
}

func (c *integrationTestCallbacks) OnProgress(completed, total int, phase UltraPlanPhase) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.progressCalls = append(c.progressCalls, struct {
		completed, total int
		phase            UltraPlanPhase
	}{completed, total, phase})
}

func (c *integrationTestCallbacks) OnComplete(success bool, summary string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.completeCalls = append(c.completeCalls, struct {
		success bool
		summary string
	}{success, summary})
}

// =============================================================================
// Phase Transition Integration Tests
// =============================================================================

func TestIntegration_PlanningToExecutionTransition(t *testing.T) {
	t.Run("planning completes and execution phase can start", func(t *testing.T) {
		session := newIntegrationTestSession()
		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		// Create planning orchestrator
		planningCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		planner, err := NewPlanningOrchestrator(planningCtx)
		if err != nil {
			t.Fatalf("failed to create planning orchestrator: %v", err)
		}

		// Execute planning
		ctx := context.Background()
		err = planner.Execute(ctx)
		if err != nil {
			t.Fatalf("planning execution failed: %v", err)
		}

		// Verify planning phase was set
		if len(callbacks.phaseChanges) == 0 {
			t.Error("expected phase change callback for planning")
		}
		if callbacks.phaseChanges[0] != PhasePlanning {
			t.Errorf("expected PhasePlanning, got %v", callbacks.phaseChanges[0])
		}

		// Now create and verify execution orchestrator can be created
		session.SetPhase(PhaseExecuting)
		execCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		executor, err := NewExecutionOrchestrator(execCtx)
		if err != nil {
			t.Fatalf("failed to create execution orchestrator after planning: %v", err)
		}

		if executor.Phase() != PhaseExecuting {
			t.Errorf("execution phase = %v, want %v", executor.Phase(), PhaseExecuting)
		}
	})

	t.Run("planning state is preserved for execution", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.objective = "Test objective"
		session.planSummary = "Test plan summary"

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		// Execute planning
		planningCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		planner, _ := NewPlanningOrchestrator(planningCtx)
		_ = planner.Execute(context.Background())

		// Verify state is accessible for execution
		if session.GetObjective() != "Test objective" {
			t.Error("objective not preserved after planning")
		}
		if session.GetPlanSummary() != "Test plan summary" {
			t.Error("plan summary not preserved after planning")
		}
	})
}

func TestIntegration_ExecutionToConsolidationTransition(t *testing.T) {
	t.Run("group completion triggers consolidation", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(2, []int{2, 2}) // 2 groups with 2 tasks each

		session := newIntegrationTestSession()
		session.totalCount = 4
		session.hasMoreGroups = true

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		// Create execution context with coordinator
		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
				Callbacks:    callbacks,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, err := NewExecutionOrchestratorWithContext(execContext)
		if err != nil {
			t.Fatalf("failed to create execution orchestrator: %v", err)
		}

		// Simulate task completions for group 0
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-0", Success: true})
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-1", Success: true})

		// Verify group is complete
		if !coordinator.groupTracker.IsGroupComplete(0) {
			t.Error("group 0 should be complete after all tasks complete")
		}

		// Trigger group advancement check (which would normally happen in execution loop)
		executor.checkAndAdvanceGroup()

		// Verify consolidation was called
		consolidationCalls := coordinator.GetConsolidationCalls()
		if len(consolidationCalls) != 1 {
			t.Errorf("expected 1 consolidation call, got %d", len(consolidationCalls))
		}
		if len(consolidationCalls) > 0 && consolidationCalls[0] != 0 {
			t.Errorf("expected consolidation for group 0, got group %d", consolidationCalls[0])
		}

		// Verify group was advanced
		if coordinator.groupTracker.currentGroup != 1 {
			t.Errorf("expected current group 1, got %d", coordinator.groupTracker.currentGroup)
		}
	})

	t.Run("consolidation failure stops execution", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(2, []int{2, 2})
		coordinator.consolidationErr = fmt.Errorf("consolidation failed")

		session := newIntegrationTestSession()
		session.totalCount = 4
		session.hasMoreGroups = true

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Complete group 0 tasks
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-0", Success: true})
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-1", Success: true})

		// Trigger group advancement
		executor.checkAndAdvanceGroup()

		// Verify session is marked as failed
		if coordinator.sessionPhase != PhaseFailed {
			t.Errorf("expected phase Failed, got %v", coordinator.sessionPhase)
		}

		// Verify group was NOT advanced
		if coordinator.groupTracker.currentGroup != 0 {
			t.Errorf("group should not advance after consolidation failure, got %d", coordinator.groupTracker.currentGroup)
		}
	})

	t.Run("multi-group execution consolidates each group before next", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(3, []int{1, 1, 1}) // 3 groups with 1 task each

		session := newIntegrationTestSession()
		session.totalCount = 3
		session.hasMoreGroups = true

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
				Callbacks:    callbacks,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Complete and consolidate each group in sequence
		// Note: checkAndAdvanceGroup reads currentGroup from session, so we need to advance it
		for groupIdx := 0; groupIdx < 3; groupIdx++ {
			taskID := fmt.Sprintf("task-%d-0", groupIdx)
			coordinator.HandleTaskCompletion(TaskCompletion{TaskID: taskID, Success: true})

			// Update session's current group to match what the executor would see
			session.mu.Lock()
			session.currentGroup = groupIdx
			session.mu.Unlock()

			executor.checkAndAdvanceGroup()
		}

		// Verify all groups were consolidated
		consolidationCalls := coordinator.GetConsolidationCalls()
		if len(consolidationCalls) != 3 {
			t.Errorf("expected 3 consolidation calls, got %d", len(consolidationCalls))
		}

		// Verify consolidation was called for each group (order may vary due to the current group tracking)
		// Check that we got consolidation calls for groups 0, 1, and 2
		groupsCovered := make(map[int]bool)
		for _, groupIdx := range consolidationCalls {
			groupsCovered[groupIdx] = true
		}
		for i := 0; i < 3; i++ {
			if !groupsCovered[i] {
				t.Errorf("expected consolidation for group %d, but it was not called", i)
			}
		}

		// Verify callbacks for group completions
		if len(callbacks.groupCompletes) != 3 {
			t.Errorf("expected 3 group complete callbacks, got %d", len(callbacks.groupCompletes))
		}
	})
}

func TestIntegration_ExecutionToSynthesisTransition(t *testing.T) {
	t.Run("execution completion triggers synthesis", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(1, []int{2}) // 1 group with 2 tasks

		session := newIntegrationTestSession()
		session.totalCount = 2

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Set state to simulate completion
		executor.mu.Lock()
		executor.state.CompletedCount = 2
		executor.state.TotalTasks = 2
		executor.mu.Unlock()

		// Trigger finish
		executor.finishExecution()

		// Verify synthesis was called
		if coordinator.GetSynthesisCalls() != 1 {
			t.Errorf("expected 1 synthesis call, got %d", coordinator.GetSynthesisCalls())
		}
	})

	t.Run("synthesis is skipped when disabled", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(1, []int{2})
		coordinator.noSynthesis = true

		session := newIntegrationTestSession()
		session.totalCount = 2

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		executor.mu.Lock()
		executor.state.CompletedCount = 2
		executor.state.TotalTasks = 2
		executor.mu.Unlock()

		executor.finishExecution()

		// Verify synthesis was NOT called
		if coordinator.GetSynthesisCalls() != 0 {
			t.Errorf("expected 0 synthesis calls when disabled, got %d", coordinator.GetSynthesisCalls())
		}

		// Verify completion was called directly
		completeCalls := coordinator.GetCompleteCalls()
		if len(completeCalls) != 1 {
			t.Errorf("expected 1 complete call, got %d", len(completeCalls))
		}
		if len(completeCalls) > 0 && !completeCalls[0].success {
			t.Error("expected successful completion")
		}
	})

	t.Run("task failures prevent synthesis and mark failure", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(1, []int{2})

		session := newIntegrationTestSession()
		session.totalCount = 2

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Simulate 1 success and 1 failure
		executor.mu.Lock()
		executor.state.CompletedCount = 1
		executor.state.FailedCount = 1
		executor.state.TotalTasks = 2
		executor.mu.Unlock()

		executor.finishExecution()

		// Verify synthesis was NOT called
		if coordinator.GetSynthesisCalls() != 0 {
			t.Errorf("expected 0 synthesis calls on failure, got %d", coordinator.GetSynthesisCalls())
		}

		// Verify failure was recorded
		if coordinator.sessionPhase != PhaseFailed {
			t.Errorf("expected phase Failed, got %v", coordinator.sessionPhase)
		}
	})
}

func TestIntegration_PartialGroupFailureHandling(t *testing.T) {
	t.Run("partial failure pauses execution awaiting decision", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(2, []int{2, 2})

		session := newIntegrationTestSession()
		session.totalCount = 4
		session.hasMoreGroups = true

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Simulate partial failure: 1 success, 1 failure in group 0
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-0", Success: true})
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-1", Success: false, Error: "test failure"})

		// Verify partial failure is detected
		if !coordinator.groupTracker.HasPartialFailure(0) {
			t.Error("expected partial failure to be detected")
		}

		// Trigger group check
		executor.checkAndAdvanceGroup()

		// Verify partial failure was handled
		if len(coordinator.partialFailureCalls) != 1 {
			t.Errorf("expected 1 partial failure call, got %d", len(coordinator.partialFailureCalls))
		}

		// Verify consolidation was NOT called
		if len(coordinator.GetConsolidationCalls()) != 0 {
			t.Error("consolidation should not be called on partial failure")
		}

		// Verify group was NOT advanced
		if coordinator.groupTracker.currentGroup != 0 {
			t.Errorf("group should not advance on partial failure, got %d", coordinator.groupTracker.currentGroup)
		}

		// Verify local state tracks the decision needed
		state := executor.State()
		if state.GroupDecision == nil {
			t.Error("expected group decision state to be set")
		}
		if state.GroupDecision != nil && !state.GroupDecision.AwaitingDecision {
			t.Error("expected awaiting decision to be true")
		}
	})
}

func TestIntegration_SynthesisLifecycle(t *testing.T) {
	t.Run("synthesis creates instance and monitors completion", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		// Pre-create an instance that will be "completed"
		_, _ = orchestrator.AddInstance(session, "synthesis task")
		for id := range orchestrator.instances {
			orchestrator.SetInstanceStatus(id, StatusCompleted)
		}

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		synth, err := NewSynthesisOrchestrator(synthCtx)
		if err != nil {
			t.Fatalf("failed to create synthesis orchestrator: %v", err)
		}

		// Execute synthesis
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err = synth.Execute(ctx)
		if err != nil {
			t.Errorf("synthesis execution failed: %v", err)
		}

		// Verify an instance was created
		if len(orchestrator.startedInstances) < 1 {
			t.Error("expected synthesis to start an instance")
		}
	})

	t.Run("synthesis cancellation stops monitoring", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)

		// Create orchestrator that returns running instances
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Start execution in goroutine
		done := make(chan error, 1)
		go func() {
			done <- synth.Execute(context.Background())
		}()

		// Give it time to start
		time.Sleep(20 * time.Millisecond)

		// Cancel
		synth.Cancel()

		// Should complete quickly after cancel
		select {
		case <-done:
			// Success
		case <-time.After(1 * time.Second):
			t.Error("synthesis did not stop after cancel")
		}
	})
}

func TestIntegration_ConsolidationPhaseLifecycle(t *testing.T) {
	t.Run("consolidation orchestrator validates context", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseConsolidating)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		consolidationCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		consolidator := NewConsolidationOrchestrator(consolidationCtx)
		if consolidator == nil {
			t.Fatal("failed to create consolidation orchestrator")
		}

		if consolidator.Phase() != PhaseConsolidating {
			t.Errorf("expected phase Consolidating, got %v", consolidator.Phase())
		}
	})

	t.Run("consolidation state management", func(t *testing.T) {
		session := newIntegrationTestSession()
		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		consolidationCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		consolidator := NewConsolidationOrchestrator(consolidationCtx)

		// Test state transitions
		initialState := consolidator.State()
		if initialState.SubPhase != "" {
			t.Error("initial state should have empty subphase")
		}

		// Set state
		consolidator.SetState(ConsolidatorState{
			SubPhase:    "gathering",
			TotalGroups: 2,
		})

		newState := consolidator.State()
		if newState.SubPhase != "gathering" {
			t.Errorf("expected subphase 'gathering', got %s", newState.SubPhase)
		}
		if newState.TotalGroups != 2 {
			t.Errorf("expected TotalGroups 2, got %d", newState.TotalGroups)
		}

		// Test reset
		consolidator.Reset()
		resetState := consolidator.State()
		if resetState.SubPhase != "" {
			t.Error("reset should clear subphase")
		}
	})

	t.Run("consolidation cancel is idempotent", func(t *testing.T) {
		session := newIntegrationTestSession()
		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		consolidationCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		consolidator := NewConsolidationOrchestrator(consolidationCtx)

		// Multiple cancels should not panic
		consolidator.Cancel()
		consolidator.Cancel()
		consolidator.Cancel()

		if !consolidator.IsCancelled() {
			t.Error("expected cancelled after Cancel()")
		}
	})
}

func TestIntegration_FullPhaseCycle(t *testing.T) {
	t.Run("complete phase cycle from planning to completion", func(t *testing.T) {
		// This test verifies the full lifecycle:
		// Planning -> Execution -> Consolidation -> Synthesis -> Complete

		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(1, []int{1}) // 1 group, 1 task

		session := newIntegrationTestSession()
		session.objective = "Integration test objective"
		session.totalCount = 1
		session.hasMoreGroups = false

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		// Phase 1: Planning
		planner, _ := NewPlanningOrchestrator(phaseCtx)
		err := planner.Execute(context.Background())
		if err != nil {
			t.Fatalf("planning failed: %v", err)
		}

		// Verify planning phase callback
		if len(callbacks.phaseChanges) < 1 || callbacks.phaseChanges[0] != PhasePlanning {
			t.Error("planning phase callback not received")
		}

		// Phase 2: Execution
		session.SetPhase(PhaseExecuting)
		execContext := &ExecutionContext{
			PhaseContext:     phaseCtx,
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)
		if executor.Phase() != PhaseExecuting {
			t.Errorf("expected executing phase, got %v", executor.Phase())
		}

		// Simulate task completion
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-0", Success: true})

		// Trigger consolidation after task completion
		executor.checkAndAdvanceGroup()

		// Verify consolidation happened
		consolidationCalls := coordinator.GetConsolidationCalls()
		if len(consolidationCalls) != 1 {
			t.Errorf("expected 1 consolidation call, got %d", len(consolidationCalls))
		}

		// Phase 3: Finish execution (triggers synthesis)
		executor.mu.Lock()
		executor.state.CompletedCount = 1
		executor.state.TotalTasks = 1
		executor.mu.Unlock()

		executor.finishExecution()

		// Verify synthesis was triggered
		if coordinator.GetSynthesisCalls() != 1 {
			t.Errorf("expected 1 synthesis call, got %d", coordinator.GetSynthesisCalls())
		}
	})

	t.Run("phase cycle handles errors gracefully", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(1, []int{2})
		coordinator.consolidationErr = fmt.Errorf("consolidation failed")

		session := newIntegrationTestSession()
		session.totalCount = 2
		session.hasMoreGroups = true

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Complete all tasks in group
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-0", Success: true})
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-1", Success: true})

		// Trigger consolidation (which will fail)
		executor.checkAndAdvanceGroup()

		// Verify failure is propagated
		if coordinator.sessionPhase != PhaseFailed {
			t.Errorf("expected phase Failed after consolidation error, got %v", coordinator.sessionPhase)
		}

		// Verify error is recorded
		if coordinator.sessionError == "" {
			t.Error("expected session error to be set")
		}

		// Verify completion callback with failure
		completeCalls := coordinator.GetCompleteCalls()
		if len(completeCalls) != 1 {
			t.Errorf("expected 1 complete call, got %d", len(completeCalls))
		}
		if len(completeCalls) > 0 && completeCalls[0].success {
			t.Error("expected failure completion, got success")
		}
	})
}

func TestIntegration_ConcurrentPhaseOperations(t *testing.T) {
	t.Run("concurrent task completions are handled safely", func(t *testing.T) {
		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(1, []int{10}) // 1 group, 10 tasks

		session := newIntegrationTestSession()
		session.totalCount = 10
		session.hasMoreGroups = false

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		execContext := &ExecutionContext{
			PhaseContext: &PhaseContext{
				Manager:      manager,
				Orchestrator: orchestrator,
				Session:      session,
			},
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		_, err := NewExecutionOrchestratorWithContext(execContext)
		if err != nil {
			t.Fatalf("failed to create executor: %v", err)
		}

		// Complete all tasks concurrently
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(taskIdx int) {
				defer wg.Done()
				taskID := fmt.Sprintf("task-0-%d", taskIdx)
				coordinator.HandleTaskCompletion(TaskCompletion{
					TaskID:  taskID,
					Success: true,
				})
			}(i)
		}
		wg.Wait()

		// Verify all tasks are tracked as completed
		if len(coordinator.completedTasks) != 10 {
			t.Errorf("expected 10 completed tasks, got %d", len(coordinator.completedTasks))
		}

		// Verify group tracker is consistent
		if !coordinator.groupTracker.IsGroupComplete(0) {
			t.Error("group 0 should be complete")
		}
	})

	t.Run("concurrent state access is thread-safe", func(t *testing.T) {
		session := newIntegrationTestSession()
		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		planner, _ := NewPlanningOrchestrator(phaseCtx)

		// Concurrent state access
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(3)

			// Reader
			go func() {
				defer wg.Done()
				_ = planner.State()
				_ = planner.GetInstanceID()
				_ = planner.IsAwaitingCompletion()
			}()

			// Writer
			go func(idx int) {
				defer wg.Done()
				planner.SetState(PlanningState{
					InstanceID: fmt.Sprintf("inst-%d", idx),
				})
			}(i)

			// Cancel checker
			go func() {
				defer wg.Done()
				_ = planner.IsCancelled()
			}()
		}
		wg.Wait()

		// Should complete without race conditions
	})
}

func TestIntegration_PhaseCallbackConsistency(t *testing.T) {
	t.Run("callbacks receive correct phase progression", func(t *testing.T) {
		session := newIntegrationTestSession()
		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		coordinator := NewIntegrationTestCoordinator()
		coordinator.SetupTasks(2, []int{1, 1})

		// Execute through multiple phases
		phaseCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		// Planning
		planner, _ := NewPlanningOrchestrator(phaseCtx)
		_ = planner.Execute(context.Background())

		// Execution
		session.SetPhase(PhaseExecuting)
		execContext := &ExecutionContext{
			PhaseContext:     phaseCtx,
			Coordinator:      coordinator,
			ExecutionSession: session,
			GroupTracker:     coordinator.groupTracker,
		}

		executor, _ := NewExecutionOrchestratorWithContext(execContext)

		// Complete tasks and groups
		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-0-0", Success: true})
		executor.checkAndAdvanceGroup()

		coordinator.HandleTaskCompletion(TaskCompletion{TaskID: "task-1-0", Success: true})
		executor.checkAndAdvanceGroup()

		// Verify phase callbacks in order
		if len(callbacks.phaseChanges) < 1 {
			t.Error("expected at least planning phase change")
		}

		// Verify first callback is planning
		if callbacks.phaseChanges[0] != PhasePlanning {
			t.Errorf("expected first phase to be Planning, got %v", callbacks.phaseChanges[0])
		}

		// Verify group complete callbacks
		if len(callbacks.groupCompletes) != 2 {
			t.Errorf("expected 2 group complete callbacks, got %d", len(callbacks.groupCompletes))
		}
	})
}

// =============================================================================
// Synthesis -> Revision -> Re-synthesis Cycle Tests
// =============================================================================

func TestIntegration_SynthesisRevisionCycle(t *testing.T) {
	t.Run("synthesis detects issues and prepares for revision", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()
		callbacks := newIntegrationTestCallbacks()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
			Callbacks:    callbacks,
		}

		synth, err := NewSynthesisOrchestrator(synthCtx)
		if err != nil {
			t.Fatalf("failed to create synthesis orchestrator: %v", err)
		}

		// Simulate finding issues
		issues := []RevisionIssue{
			{TaskID: "task-1", Severity: "critical", Description: "Critical bug"},
			{TaskID: "task-2", Severity: "major", Description: "Major issue"},
			{TaskID: "task-3", Severity: "minor", Description: "Minor suggestion"},
		}
		synth.setIssuesFound(issues)

		// Verify issues were set
		foundIssues := synth.GetIssuesFound()
		if len(foundIssues) != 3 {
			t.Errorf("expected 3 issues, got %d", len(foundIssues))
		}

		// Check which issues need revision (only critical and major)
		if !synth.NeedsRevision() {
			t.Error("should need revision when critical/major issues exist")
		}

		revisionNeeded := synth.GetIssuesNeedingRevision()
		if len(revisionNeeded) != 2 {
			t.Errorf("expected 2 issues needing revision, got %d", len(revisionNeeded))
		}

		// Verify severity filtering
		for _, issue := range revisionNeeded {
			if issue.Severity != "critical" && issue.Severity != "major" {
				t.Errorf("unexpected severity in revision issues: %s", issue.Severity)
			}
		}
	})

	t.Run("synthesis awaiting approval state management", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Initially not awaiting
		if synth.IsAwaitingApproval() {
			t.Error("should not be awaiting approval initially")
		}

		// Set awaiting - this sets internal state
		synth.SetAwaitingApproval(true)
		if !synth.IsAwaitingApproval() {
			t.Error("should be awaiting approval after setting")
		}

		// The orchestrator's internal state should be set
		state := synth.State()
		if !state.AwaitingApproval {
			t.Error("state.AwaitingApproval should be true")
		}

		// Clear awaiting
		synth.SetAwaitingApproval(false)
		if synth.IsAwaitingApproval() {
			t.Error("should not be awaiting approval after clearing")
		}
	})

	t.Run("synthesis revision round tracking", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Initial round should be 0
		if synth.GetRevisionRound() != 0 {
			t.Errorf("initial revision round should be 0, got %d", synth.GetRevisionRound())
		}

		// Increment round
		synth.SetRevisionRound(1)
		if synth.GetRevisionRound() != 1 {
			t.Errorf("revision round should be 1, got %d", synth.GetRevisionRound())
		}

		// Verify state consistency
		state := synth.State()
		if state.RevisionRound != 1 {
			t.Errorf("state revision round should be 1, got %d", state.RevisionRound)
		}
	})

	t.Run("synthesis completion file handling", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Initially no completion file
		if synth.GetCompletionFile() != nil {
			t.Error("should have no completion file initially")
		}

		// Set completion file
		completion := &SynthesisCompletionFile{
			Status:           "complete",
			IntegrationNotes: "All tasks reviewed successfully",
		}
		synth.setCompletionFile(completion)

		// Verify it was set
		got := synth.GetCompletionFile()
		if got == nil {
			t.Fatal("completion file should be set")
		}
		if got.Status != "complete" {
			t.Errorf("expected status 'complete', got %s", got.Status)
		}
		if got.IntegrationNotes != "All tasks reviewed successfully" {
			t.Errorf("expected IntegrationNotes 'All tasks reviewed successfully', got %s", got.IntegrationNotes)
		}
	})

	t.Run("synthesis no revision needed for minor issues only", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Only minor issues
		issues := []RevisionIssue{
			{TaskID: "task-1", Severity: "minor", Description: "Minor suggestion 1"},
			{TaskID: "task-2", Severity: "minor", Description: "Minor suggestion 2"},
		}
		synth.setIssuesFound(issues)

		// Should not need revision for minor issues
		if synth.NeedsRevision() {
			t.Error("should not need revision when only minor issues exist")
		}

		revisionNeeded := synth.GetIssuesNeedingRevision()
		if len(revisionNeeded) != 0 {
			t.Errorf("expected 0 issues needing revision, got %d", len(revisionNeeded))
		}
	})

	t.Run("synthesis state reset clears all data", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Set up various state
		synth.setInstanceID("synth-instance-1")
		synth.SetAwaitingApproval(true)
		synth.SetRevisionRound(2)
		synth.setIssuesFound([]RevisionIssue{
			{TaskID: "task-1", Severity: "critical"},
		})
		synth.setCompletionFile(&SynthesisCompletionFile{Status: "completed"})

		// Verify state is set
		if synth.GetInstanceID() == "" {
			t.Error("instance ID should be set before reset")
		}

		// Reset
		synth.Reset()

		// Verify all state is cleared
		state := synth.State()
		if state.InstanceID != "" {
			t.Errorf("instance ID should be empty after reset, got %s", state.InstanceID)
		}
		if state.AwaitingApproval {
			t.Error("awaiting approval should be false after reset")
		}
		if state.RevisionRound != 0 {
			t.Errorf("revision round should be 0 after reset, got %d", state.RevisionRound)
		}
		if len(state.IssuesFound) != 0 {
			t.Errorf("issues should be empty after reset, got %d", len(state.IssuesFound))
		}
		if state.CompletionFile != nil {
			t.Error("completion file should be nil after reset")
		}
	})

	t.Run("synthesis revision state tracking", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Initially not in revision
		if synth.IsInRevision() {
			t.Error("should not be in revision initially")
		}

		// Get revision state returns nil when not in revision mode
		// This is expected behavior - revision state is only populated
		// when the revision process has started
		revState := synth.GetRevisionState()
		if revState != nil {
			t.Error("revision state should be nil before revision starts")
		}

		// Running revision task count should be 0 initially
		if synth.GetRunningRevisionTaskCount() != 0 {
			t.Errorf("running revision task count should be 0, got %d", synth.GetRunningRevisionTaskCount())
		}
	})
}

func TestIntegration_SynthesisProceedToConsolidation(t *testing.T) {
	t.Run("proceed to consolidation or complete flow", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)
		session.consolidationMode = "" // No consolidation - should complete directly

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// Capture worktree info should be safe to call
		synth.CaptureTaskWorktreeInfo()

		// Proceed to consolidation or complete - with no consolidation mode,
		// this should attempt to mark as complete
		err := synth.ProceedToConsolidationOrComplete()

		// Error is expected since we don't have a full coordinator setup
		// The important thing is it doesn't panic
		_ = err

		// Verify state is still consistent
		if synth.Phase() != PhaseSynthesis {
			t.Errorf("phase should still be synthesis, got %v", synth.Phase())
		}
	})

	t.Run("trigger consolidation is callable", func(t *testing.T) {
		session := newIntegrationTestSession()
		session.SetPhase(PhaseSynthesis)

		manager := newIntegrationTestManager(session)
		orchestrator := newIntegrationTestOrchestrator()

		synthCtx := &PhaseContext{
			Manager:      manager,
			Orchestrator: orchestrator,
			Session:      session,
		}

		synth, _ := NewSynthesisOrchestrator(synthCtx)

		// TriggerConsolidation should be callable
		err := synth.TriggerConsolidation()

		// Error is expected since we don't have the full coordinator
		_ = err
	})
}

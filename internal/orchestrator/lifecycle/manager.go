// Package lifecycle provides instance lifecycle management for the orchestrator.
//
// This package encapsulates the creation, starting, stopping, and monitoring
// of Claude Code instances within an orchestration session.
//
// Group-Aware Lifecycle:
// The Manager supports group-based execution ordering, where instances in later
// groups are blocked from starting until earlier groups complete. This is used
// by Plan and UltraPlan workflows to enforce sequential execution of task groups.
//
// Key concepts:
//   - GroupProvider: Interface for querying group membership and dependencies
//   - CanStartInstance: Checks if an instance's group dependencies are met
//   - GroupCompletionState: Tracks group-level completion status
//   - Group phase callbacks: Enables TUI reactivity to group state changes
package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// InstanceStatus represents the current status of an instance.
type InstanceStatus string

const (
	StatusPending      InstanceStatus = "pending"
	StatusWorking      InstanceStatus = "working"
	StatusWaitingInput InstanceStatus = "waiting_input"
	StatusPaused       InstanceStatus = "paused"
	StatusCompleted    InstanceStatus = "completed"
	StatusError        InstanceStatus = "error"
	StatusCreatingPR   InstanceStatus = "creating_pr"
	StatusStuck        InstanceStatus = "stuck"
	StatusTimeout      InstanceStatus = "timeout"
)

// Instance represents a managed Claude Code instance.
type Instance struct {
	ID            string         `json:"id"`
	WorktreePath  string         `json:"worktree_path"`
	Branch        string         `json:"branch"`
	Task          string         `json:"task"`
	Status        InstanceStatus `json:"status"`
	PID           int            `json:"pid,omitempty"`
	FilesModified []string       `json:"files_modified,omitempty"`
	TmuxSession   string         `json:"tmux_session"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

// Callbacks holds callback functions for instance lifecycle events.
type Callbacks struct {
	// OnStatusChange is called when an instance's status changes.
	OnStatusChange func(instanceID string, oldStatus, newStatus InstanceStatus)

	// OnPRComplete is called when an instance completes creating a PR.
	OnPRComplete func(instanceID, prURL string)

	// OnTimeout is called when an instance times out.
	OnTimeout func(instanceID string)

	// OnBell is called when an instance triggers a terminal bell.
	OnBell func(instanceID string)

	// OnError is called when an instance encounters an error.
	OnError func(instanceID string, err error)

	// OnGroupPhaseChange is called when a group's phase changes.
	// This enables TUI reactivity to group state transitions.
	OnGroupPhaseChange func(groupID, groupName string, oldPhase, newPhase GroupPhase)

	// OnGroupComplete is called when all instances in a group have finished.
	// success is true only if all instances completed successfully.
	OnGroupComplete func(groupID, groupName string, success bool, failedCount, successCount int)
}

// GroupPhase represents the current phase of an instance group.
type GroupPhase string

const (
	GroupPhasePending   GroupPhase = "pending"
	GroupPhaseExecuting GroupPhase = "executing"
	GroupPhaseCompleted GroupPhase = "completed"
	GroupPhaseFailed    GroupPhase = "failed"
)

// GroupInfo contains information about a group.
// This is a minimal representation used by the lifecycle manager.
type GroupInfo struct {
	ID             string     // Unique group identifier
	Name           string     // Human-readable name
	Phase          GroupPhase // Current phase
	ExecutionOrder int        // Order of execution (0 = first)
	InstanceIDs    []string   // IDs of instances in this group
	DependsOn      []string   // IDs of groups this group depends on
}

// GroupProvider is an interface for querying group information.
// This allows the lifecycle manager to check group dependencies without
// directly depending on the group package.
type GroupProvider interface {
	// GetGroupForInstance returns the group containing the given instance.
	// Returns nil if the instance is not in any group.
	GetGroupForInstance(instanceID string) *GroupInfo

	// GetGroup returns a group by ID.
	// Returns nil if the group doesn't exist.
	GetGroup(groupID string) *GroupInfo

	// GetAllGroups returns all groups sorted by execution order.
	GetAllGroups() []*GroupInfo

	// AreGroupDependenciesMet checks if all dependencies for a group have completed.
	AreGroupDependenciesMet(groupID string) bool

	// AdvanceGroupPhase updates the phase of the specified group.
	AdvanceGroupPhase(groupID string, phase GroupPhase)
}

// GroupCompletionState tracks the completion status of a group.
type GroupCompletionState struct {
	GroupID      string     // Group identifier
	GroupName    string     // Human-readable name
	Phase        GroupPhase // Current phase
	TotalCount   int        // Total instances in group
	PendingCount int        // Instances not yet started
	RunningCount int        // Instances currently working
	SuccessCount int        // Instances that completed successfully
	FailedCount  int        // Instances that failed or errored
}

// IsComplete returns true if all instances in the group have finished.
func (s *GroupCompletionState) IsComplete() bool {
	return s.PendingCount == 0 && s.RunningCount == 0
}

// AllSucceeded returns true if all instances completed successfully.
func (s *GroupCompletionState) AllSucceeded() bool {
	return s.IsComplete() && s.FailedCount == 0 && s.SuccessCount > 0
}

// HasFailures returns true if any instance failed.
func (s *GroupCompletionState) HasFailures() bool {
	return s.FailedCount > 0
}

// Config holds configuration for the lifecycle manager.
type Config struct {
	// TmuxSessionPrefix is the prefix for tmux session names.
	TmuxSessionPrefix string

	// DefaultTermWidth is the default terminal width.
	DefaultTermWidth int

	// DefaultTermHeight is the default terminal height.
	DefaultTermHeight int

	// ActivityTimeout is how long to wait for activity before timing out.
	ActivityTimeout time.Duration

	// CompletionTimeout is how long to wait for completion detection.
	CompletionTimeout time.Duration
}

// DefaultConfig returns sensible defaults for lifecycle configuration.
func DefaultConfig() Config {
	return Config{
		TmuxSessionPrefix: "claudio",
		DefaultTermWidth:  200,
		DefaultTermHeight: 30,
		ActivityTimeout:   5 * time.Minute,
		CompletionTimeout: 30 * time.Second,
	}
}

// Manager manages the lifecycle of Claude Code instances.
type Manager struct {
	config    Config
	callbacks Callbacks
	logger    *logging.Logger

	mu            sync.RWMutex
	instances     map[string]*Instance
	stopChan      chan struct{}
	stopped       bool
	wg            sync.WaitGroup
	groupProvider GroupProvider // Optional: for group-aware lifecycle management
}

// NewManager creates a new lifecycle manager.
func NewManager(config Config, callbacks Callbacks, logger *logging.Logger) *Manager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &Manager{
		config:    config,
		callbacks: callbacks,
		logger:    logger.WithPhase("lifecycle"),
		instances: make(map[string]*Instance),
		stopChan:  make(chan struct{}),
	}
}

// CreateInstance creates a new instance but does not start it.
func (m *Manager) CreateInstance(id, worktreePath, branch, task string) (*Instance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; exists {
		return nil, fmt.Errorf("instance %s already exists", id)
	}

	inst := &Instance{
		ID:           id,
		WorktreePath: worktreePath,
		Branch:       branch,
		Task:         task,
		Status:       StatusPending,
		TmuxSession:  fmt.Sprintf("%s-%s", m.config.TmuxSessionPrefix, id),
	}

	m.instances[id] = inst

	m.logger.Info("instance created",
		"instance_id", id,
		"worktree", worktreePath,
		"branch", branch,
	)

	return inst, nil
}

// StartInstance starts a previously created instance.
func (m *Manager) StartInstance(ctx context.Context, id string) error {
	m.mu.Lock()
	inst, exists := m.instances[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	if inst.Status != StatusPending {
		m.mu.Unlock()
		return fmt.Errorf("instance %s is not in pending status", id)
	}
	m.mu.Unlock()

	// Start the instance (implementation depends on process management)
	// This is a placeholder - actual implementation would use process.TmuxProcess
	now := time.Now()

	m.mu.Lock()
	inst.Status = StatusWorking
	inst.StartedAt = &now
	m.mu.Unlock()

	if m.callbacks.OnStatusChange != nil {
		m.callbacks.OnStatusChange(id, StatusPending, StatusWorking)
	}

	m.logger.Info("instance started",
		"instance_id", id,
	)

	return nil
}

// StopInstance stops a running instance.
func (m *Manager) StopInstance(id string) error {
	m.mu.Lock()
	inst, exists := m.instances[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	oldStatus := inst.Status
	m.mu.Unlock()

	// Stop the instance (implementation depends on process management)
	// This is a placeholder - actual implementation would stop the tmux session

	now := time.Now()

	m.mu.Lock()
	inst.Status = StatusCompleted
	inst.CompletedAt = &now
	m.mu.Unlock()

	if m.callbacks.OnStatusChange != nil {
		m.callbacks.OnStatusChange(id, oldStatus, StatusCompleted)
	}

	m.logger.Info("instance stopped",
		"instance_id", id,
	)

	return nil
}

// GetInstance returns an instance by ID.
func (m *Manager) GetInstance(id string) (*Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, exists := m.instances[id]
	if !exists {
		return nil, false
	}
	// Return a copy
	instCopy := *inst
	return &instCopy, true
}

// ListInstances returns all instances.
func (m *Manager) ListInstances() []*Instance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Instance, 0, len(m.instances))
	for _, inst := range m.instances {
		instCopy := *inst
		result = append(result, &instCopy)
	}
	return result
}

// UpdateStatus updates an instance's status.
func (m *Manager) UpdateStatus(id string, status InstanceStatus) error {
	m.mu.Lock()
	inst, exists := m.instances[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("instance %s not found", id)
	}
	oldStatus := inst.Status
	inst.Status = status

	if status == StatusCompleted || status == StatusError || status == StatusTimeout {
		now := time.Now()
		inst.CompletedAt = &now
	}
	m.mu.Unlock()

	if m.callbacks.OnStatusChange != nil && oldStatus != status {
		m.callbacks.OnStatusChange(id, oldStatus, status)
	}

	m.logger.Debug("instance status updated",
		"instance_id", id,
		"old_status", string(oldStatus),
		"new_status", string(status),
	)

	return nil
}

// RemoveInstance removes an instance from management.
func (m *Manager) RemoveInstance(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.instances[id]; !exists {
		return fmt.Errorf("instance %s not found", id)
	}

	delete(m.instances, id)

	m.logger.Info("instance removed",
		"instance_id", id,
	)

	return nil
}

// GetRunningCount returns the number of currently running instances.
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, inst := range m.instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			count++
		}
	}
	return count
}

// Stop stops the lifecycle manager and all running instances.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	close(m.stopChan)

	// Collect IDs to stop while holding lock to avoid race condition
	var toStop []string
	for id, inst := range m.instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			toStop = append(toStop, id)
		}
	}
	m.mu.Unlock()

	// Stop all running instances (without holding lock)
	var stopErrors []error
	for _, id := range toStop {
		if err := m.StopInstance(id); err != nil {
			m.logger.Error("failed to stop instance during shutdown",
				"instance_id", id,
				"error", err.Error(),
			)
			stopErrors = append(stopErrors, err)
		}
	}

	if len(stopErrors) > 0 {
		m.logger.Warn("shutdown completed with errors",
			"failed_instances", len(stopErrors),
		)
	}

	m.wg.Wait()
}

// -----------------------------------------------------------------------------
// Group-Aware Lifecycle Management
// -----------------------------------------------------------------------------

// SetGroupProvider sets the group provider for group-aware lifecycle management.
// If set, the manager will check group dependencies before starting instances.
func (m *Manager) SetGroupProvider(provider GroupProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groupProvider = provider
}

// CanStartInstance checks if an instance can be started based on group dependencies.
// Returns (true, "") if the instance can start, or (false, reason) if not.
//
// An instance can start if:
//   - No group provider is set (ungrouped mode)
//   - The instance is not in any group
//   - The instance's group has all dependencies met (earlier groups completed)
func (m *Manager) CanStartInstance(id string) (bool, string) {
	m.mu.RLock()
	provider := m.groupProvider
	inst, exists := m.instances[id]
	m.mu.RUnlock()

	if !exists {
		return false, "instance not found"
	}

	if inst.Status != StatusPending {
		return false, fmt.Sprintf("instance is not pending (status: %s)", inst.Status)
	}

	// If no group provider, always allow starting
	if provider == nil {
		return true, ""
	}

	// Get the group for this instance
	group := provider.GetGroupForInstance(id)
	if group == nil {
		// Instance is not in any group, allow starting
		return true, ""
	}

	// Check if this group's dependencies are met
	if !provider.AreGroupDependenciesMet(group.ID) {
		return false, fmt.Sprintf("group %q dependencies not met", group.Name)
	}

	return true, ""
}

// StartInstanceIfReady starts an instance only if its group dependencies are met.
// Returns an error if the instance cannot be started.
//
// If the instance starts successfully and its group was pending, the group
// phase is advanced to "executing" and the OnGroupPhaseChange callback is invoked.
func (m *Manager) StartInstanceIfReady(ctx context.Context, id string) error {
	canStart, reason := m.CanStartInstance(id)
	if !canStart {
		return fmt.Errorf("cannot start instance %s: %s", id, reason)
	}

	// Start the instance
	if err := m.StartInstance(ctx, id); err != nil {
		return err
	}

	// Advance group phase to executing if this is the first instance in the group
	m.advanceGroupPhaseOnStart(id)

	return nil
}

// advanceGroupPhaseOnStart advances a group's phase to "executing" when its
// first instance starts. Must be called after the instance is started.
func (m *Manager) advanceGroupPhaseOnStart(instanceID string) {
	m.mu.RLock()
	provider := m.groupProvider
	m.mu.RUnlock()

	if provider == nil {
		return
	}

	group := provider.GetGroupForInstance(instanceID)
	if group == nil || group.Phase != GroupPhasePending {
		return
	}

	// Advance to executing
	provider.AdvanceGroupPhase(group.ID, GroupPhaseExecuting)

	// Notify via callback
	if m.callbacks.OnGroupPhaseChange != nil {
		m.callbacks.OnGroupPhaseChange(group.ID, group.Name, GroupPhasePending, GroupPhaseExecuting)
	}

	m.logger.Info("group phase advanced to executing",
		"group_id", group.ID,
		"group_name", group.Name,
	)
}

// GetGroupCompletionState returns the completion state for a group.
// Returns nil if the group doesn't exist or no group provider is set.
func (m *Manager) GetGroupCompletionState(groupID string) *GroupCompletionState {
	m.mu.RLock()
	provider := m.groupProvider
	instances := m.instances
	m.mu.RUnlock()

	if provider == nil {
		return nil
	}

	group := provider.GetGroup(groupID)
	if group == nil {
		return nil
	}

	state := &GroupCompletionState{
		GroupID:    group.ID,
		GroupName:  group.Name,
		Phase:      group.Phase,
		TotalCount: len(group.InstanceIDs),
	}

	// Count instances by status
	for _, instID := range group.InstanceIDs {
		inst, exists := instances[instID]
		if !exists {
			state.PendingCount++
			continue
		}

		switch inst.Status {
		case StatusPending:
			state.PendingCount++
		case StatusWorking, StatusWaitingInput, StatusCreatingPR:
			state.RunningCount++
		case StatusCompleted:
			state.SuccessCount++
		case StatusError, StatusStuck, StatusTimeout:
			state.FailedCount++
		case StatusPaused:
			// Paused instances count as pending (can be resumed)
			state.PendingCount++
		}
	}

	return state
}

// CheckGroupCompletion checks if a group has completed after an instance status change.
// If complete, it advances the group phase and invokes the OnGroupComplete callback.
// Returns the completion state, or nil if the group is not complete.
func (m *Manager) CheckGroupCompletion(instanceID string) *GroupCompletionState {
	m.mu.RLock()
	provider := m.groupProvider
	m.mu.RUnlock()

	if provider == nil {
		return nil
	}

	group := provider.GetGroupForInstance(instanceID)
	if group == nil {
		return nil
	}

	state := m.GetGroupCompletionState(group.ID)
	if state == nil || !state.IsComplete() {
		return nil
	}

	// Group is complete - determine final phase
	oldPhase := state.Phase
	var newPhase GroupPhase
	if state.AllSucceeded() {
		newPhase = GroupPhaseCompleted
	} else {
		newPhase = GroupPhaseFailed
	}

	// Only advance if phase actually changes
	if oldPhase != newPhase {
		provider.AdvanceGroupPhase(group.ID, newPhase)
		state.Phase = newPhase

		// Notify via phase change callback
		if m.callbacks.OnGroupPhaseChange != nil {
			m.callbacks.OnGroupPhaseChange(group.ID, state.GroupName, oldPhase, newPhase)
		}

		m.logger.Info("group completed",
			"group_id", group.ID,
			"group_name", state.GroupName,
			"phase", string(newPhase),
			"success_count", state.SuccessCount,
			"failed_count", state.FailedCount,
		)
	}

	// Notify via completion callback
	if m.callbacks.OnGroupComplete != nil {
		m.callbacks.OnGroupComplete(group.ID, state.GroupName, state.AllSucceeded(), state.FailedCount, state.SuccessCount)
	}

	return state
}

// GetReadyGroups returns groups that are pending and have all dependencies met.
// These groups can have their instances started.
func (m *Manager) GetReadyGroups() []*GroupInfo {
	m.mu.RLock()
	provider := m.groupProvider
	m.mu.RUnlock()

	if provider == nil {
		return nil
	}

	allGroups := provider.GetAllGroups()
	var ready []*GroupInfo

	for _, group := range allGroups {
		if group.Phase == GroupPhasePending && provider.AreGroupDependenciesMet(group.ID) {
			ready = append(ready, group)
		}
	}

	return ready
}

// StartNextGroup starts all pending instances in the next ready group.
// Returns the number of instances started, or 0 if no groups are ready.
func (m *Manager) StartNextGroup(ctx context.Context) (int, error) {
	readyGroups := m.GetReadyGroups()
	if len(readyGroups) == 0 {
		return 0, nil
	}

	// Start the first ready group (lowest execution order)
	group := readyGroups[0]
	started := 0

	for _, instID := range group.InstanceIDs {
		if err := m.StartInstanceIfReady(ctx, instID); err != nil {
			// Log but continue with other instances
			m.logger.Warn("failed to start instance in group",
				"instance_id", instID,
				"group_id", group.ID,
				"error", err.Error(),
			)
			continue
		}
		started++
	}

	return started, nil
}

// StartAllReadyInstances starts all instances whose groups are ready.
// Returns the total number of instances started.
func (m *Manager) StartAllReadyInstances(ctx context.Context) (int, error) {
	readyGroups := m.GetReadyGroups()
	if len(readyGroups) == 0 {
		return 0, nil
	}

	totalStarted := 0
	for _, group := range readyGroups {
		for _, instID := range group.InstanceIDs {
			if err := m.StartInstanceIfReady(ctx, instID); err != nil {
				// Log but continue with other instances
				m.logger.Warn("failed to start instance in group",
					"instance_id", instID,
					"group_id", group.ID,
					"error", err.Error(),
				)
				continue
			}
			totalStarted++
		}
	}

	return totalStarted, nil
}

// OnInstanceComplete should be called when an instance finishes to check
// group completion and potentially auto-start the next group.
// Returns the IDs of instances that were auto-started (if any).
func (m *Manager) OnInstanceComplete(ctx context.Context, instanceID string) []string {
	// Check if the instance's group has completed
	completionState := m.CheckGroupCompletion(instanceID)
	if completionState == nil {
		return nil
	}

	// If the group completed successfully, check for next ready groups
	if !completionState.AllSucceeded() {
		return nil
	}

	readyGroups := m.GetReadyGroups()
	if len(readyGroups) == 0 {
		return nil
	}

	// Auto-start instances in the next ready group
	var startedIDs []string
	for _, group := range readyGroups {
		for _, instID := range group.InstanceIDs {
			inst, exists := m.GetInstance(instID)
			if !exists || inst.Status != StatusPending {
				continue
			}

			if err := m.StartInstanceIfReady(ctx, instID); err != nil {
				m.logger.Warn("failed to auto-start instance after group completion",
					"instance_id", instID,
					"group_id", group.ID,
					"error", err.Error(),
				)
				continue
			}
			startedIDs = append(startedIDs, instID)
		}
	}

	if len(startedIDs) > 0 {
		m.logger.Info("auto-started instances after group completion",
			"started_count", len(startedIDs),
			"triggered_by", instanceID,
		)
	}

	return startedIDs
}

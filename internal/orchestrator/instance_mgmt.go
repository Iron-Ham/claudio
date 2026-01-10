package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Iron-Ham/claudio/internal/instance"
)

// AddInstance adds a new Claude instance to the session
func (o *Orchestrator) AddInstance(session *Session, task string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance
	inst := NewInstance(task)

	// Generate branch name from task using configured naming convention
	branchSlug := slugify(task)
	inst.Branch = o.generateBranchName(inst.ID, branchSlug)

	// Create worktree
	wtPath := filepath.Join(o.worktreeDir, inst.ID)
	if err := o.wt.Create(wtPath, inst.Branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}
	inst.WorktreePath = wtPath

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, task)
	o.instances[inst.ID] = mgr

	// Register with conflict detector
	if err := o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to watch instance for conflicts: %v\n", err)
	}

	// Update shared context
	if err := o.updateContext(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	if err := o.saveSession(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return inst, nil
}

// AddInstanceToWorktree adds a new instance that uses an existing worktree
// This is used for revision tasks that need to work in the same worktree as the original task
func (o *Orchestrator) AddInstanceToWorktree(session *Session, task string, worktreePath string, branch string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance with pre-set worktree info
	inst := NewInstance(task)
	inst.WorktreePath = worktreePath
	inst.Branch = branch

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, task)
	o.instances[inst.ID] = mgr

	// Save session
	if err := o.saveSession(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return inst, nil
}

// AddInstanceFromBranch adds a new Claude instance with a worktree branched from a specific base branch.
// This is used for ultraplan tasks where the next group should build on the consolidated branch from the previous group.
func (o *Orchestrator) AddInstanceFromBranch(session *Session, task string, baseBranch string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance
	inst := NewInstance(task)

	// Generate branch name from task using configured naming convention
	branchSlug := slugify(task)
	inst.Branch = o.generateBranchName(inst.ID, branchSlug)

	// Create worktree from the specified base branch
	wtPath := filepath.Join(o.worktreeDir, inst.ID)
	if err := o.wt.CreateFromBranch(wtPath, inst.Branch, baseBranch); err != nil {
		return nil, fmt.Errorf("failed to create worktree from branch %s: %w", baseBranch, err)
	}
	inst.WorktreePath = wtPath

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, task)
	o.instances[inst.ID] = mgr

	// Register with conflict detector
	if err := o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to watch instance for conflicts: %v\n", err)
	}

	// Update shared context
	if err := o.updateContext(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	if err := o.saveSession(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return inst, nil
}

// StartInstance starts a Claude process for an instance
func (o *Orchestrator) StartInstance(inst *Instance) error {
	o.mu.Lock()
	mgr, ok := o.instances[inst.ID]
	o.mu.Unlock()

	if !ok {
		mgr = o.newInstanceManager(inst.ID, inst.WorktreePath, inst.Task)
		o.mu.Lock()
		o.instances[inst.ID] = mgr
		o.mu.Unlock()
	}

	// Configure state change callback for notifications
	mgr.SetStateCallback(func(id string, state instance.WaitingState) {
		switch state {
		case instance.StateCompleted:
			o.handleInstanceExit(id)
		case instance.StateWaitingInput, instance.StateWaitingQuestion, instance.StateWaitingPermission:
			o.handleInstanceWaitingInput(id)
		case instance.StatePROpened:
			o.handleInstancePROpened(id)
		}
	})

	// Configure metrics callback for resource tracking
	mgr.SetMetricsCallback(func(id string, metrics *instance.ParsedMetrics) {
		o.handleInstanceMetrics(id, metrics)
	})

	// Configure timeout callback
	mgr.SetTimeoutCallback(func(id string, timeoutType instance.TimeoutType) {
		o.handleInstanceTimeout(id, timeoutType)
	})

	// Configure bell callback to forward terminal bells
	mgr.SetBellCallback(func(id string) {
		o.handleInstanceBell(id)
	})

	if err := mgr.Start(); err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	inst.Status = StatusWorking
	inst.PID = mgr.PID()
	inst.TmuxSession = mgr.SessionName() // Save tmux session name for recovery

	// Initialize metrics with start time
	now := mgr.StartTime()
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{StartTime: now}
	} else {
		inst.Metrics.StartTime = now
	}

	return o.saveSession()
}

// StopInstance stops a running Claude instance
func (o *Orchestrator) StopInstance(inst *Instance) error {
	o.mu.RLock()
	mgr, ok := o.instances[inst.ID]
	o.mu.RUnlock()

	if !ok {
		return nil // Already stopped
	}

	if err := mgr.Stop(); err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	inst.Status = StatusCompleted
	inst.PID = 0

	return o.saveSession()
}

// RemoveInstance stops and removes a specific instance, including its worktree and branch
func (o *Orchestrator) RemoveInstance(session *Session, instanceID string, force bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Find the instance
	var inst *Instance
	var instIndex int
	for i, instance := range session.Instances {
		if instance.ID == instanceID {
			inst = instance
			instIndex = i
			break
		}
	}

	if inst == nil {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	// Check for uncommitted changes if not forcing
	if !force {
		hasChanges, err := o.wt.HasUncommittedChanges(inst.WorktreePath)
		if err == nil && hasChanges {
			return fmt.Errorf("instance %s has uncommitted changes. Use --force to remove anyway", instanceID)
		}
	}

	// Stop the instance if running
	if mgr, ok := o.instances[inst.ID]; ok {
		_ = mgr.Stop()
		delete(o.instances, inst.ID)
	}

	// Stop PR workflow if running
	if workflow, ok := o.prWorkflows[inst.ID]; ok {
		_ = workflow.Stop()
		delete(o.prWorkflows, inst.ID)
	}

	// Remove worktree
	if err := o.wt.Remove(inst.WorktreePath); err != nil {
		// Log but don't fail - the directory might already be gone
		fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
	}

	// Delete branch
	if err := o.wt.DeleteBranch(inst.Branch); err != nil {
		// Log but don't fail - the branch might already be gone
		fmt.Fprintf(os.Stderr, "Warning: failed to delete branch: %v\n", err)
	}

	// Remove from session
	session.Instances = append(session.Instances[:instIndex], session.Instances[instIndex+1:]...)

	// Update context
	if err := o.updateContext(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	return o.saveSession()
}

// GetInstance returns an instance by ID from the current session
func (o *Orchestrator) GetInstance(id string) *Instance {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.session == nil {
		return nil
	}

	for _, inst := range o.session.Instances {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}

// ReconnectInstance attempts to reconnect to a stopped or paused instance
// If the tmux session still exists, it reconnects to it
// If not, it restarts Claude with the same task in the existing worktree
func (o *Orchestrator) ReconnectInstance(inst *Instance) error {
	o.mu.Lock()
	mgr, ok := o.instances[inst.ID]
	o.mu.Unlock()

	// If no manager exists yet, create one
	if !ok {
		mgr = o.newInstanceManager(inst.ID, inst.WorktreePath, inst.Task)
		o.mu.Lock()
		o.instances[inst.ID] = mgr
		o.mu.Unlock()
	}

	// Configure state change callback
	mgr.SetStateCallback(func(id string, state instance.WaitingState) {
		switch state {
		case instance.StateCompleted:
			o.handleInstanceExit(id)
		case instance.StateWaitingInput, instance.StateWaitingQuestion, instance.StateWaitingPermission:
			o.handleInstanceWaitingInput(id)
		}
	})

	// Configure metrics callback
	mgr.SetMetricsCallback(func(id string, metrics *instance.ParsedMetrics) {
		o.handleInstanceMetrics(id, metrics)
	})

	// Configure timeout callback
	mgr.SetTimeoutCallback(func(id string, timeoutType instance.TimeoutType) {
		o.handleInstanceTimeout(id, timeoutType)
	})

	// Configure bell callback to forward terminal bells
	mgr.SetBellCallback(func(id string) {
		o.handleInstanceBell(id)
	})

	// Check if the tmux session still exists
	if mgr.TmuxSessionExists() {
		// Reconnect to the existing session
		if err := mgr.Reconnect(); err != nil {
			return fmt.Errorf("failed to reconnect to existing session: %w", err)
		}
	} else {
		// Session doesn't exist - start a fresh one with the same task
		if err := mgr.Start(); err != nil {
			return fmt.Errorf("failed to restart instance: %w", err)
		}
	}

	inst.Status = StatusWorking
	inst.PID = mgr.PID()
	inst.TmuxSession = mgr.SessionName()

	// Update start time for metrics
	now := mgr.StartTime()
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{StartTime: now}
	} else {
		inst.Metrics.StartTime = now
		inst.Metrics.EndTime = nil // Clear end time since we're restarting
	}

	return o.saveSession()
}

// ClearCompletedInstances removes all instances with StatusCompleted from the session
// Returns the number of instances removed and any error encountered
func (o *Orchestrator) ClearCompletedInstances(session *Session) (int, error) {
	// Collect IDs of completed instances first (to avoid modifying slice while iterating)
	var completedIDs []string
	for _, inst := range session.Instances {
		if inst.Status == StatusCompleted {
			completedIDs = append(completedIDs, inst.ID)
		}
	}

	if len(completedIDs) == 0 {
		return 0, nil
	}

	// Remove each completed instance (force=true since they're already completed)
	removed := 0
	for _, id := range completedIDs {
		if err := o.RemoveInstance(session, id, true); err != nil {
			// Log warning but continue with other removals
			fmt.Fprintf(os.Stderr, "Warning: failed to remove instance %s: %v\n", id, err)
			continue
		}
		removed++
	}

	return removed, nil
}

// GetInstanceDiff returns the git diff for an instance against main
func (o *Orchestrator) GetInstanceDiff(worktreePath string) (string, error) {
	return o.wt.GetDiffAgainstMain(worktreePath)
}

// GetInstanceManager returns the manager for an instance
func (o *Orchestrator) GetInstanceManager(id string) *instance.Manager {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.instances[id]
}

// GetInstanceMetrics returns the current metrics for a specific instance
func (o *Orchestrator) GetInstanceMetrics(id string) *Metrics {
	inst := o.GetInstance(id)
	if inst == nil {
		return nil
	}
	return inst.Metrics
}

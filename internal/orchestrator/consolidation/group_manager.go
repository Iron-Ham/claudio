// Package consolidation provides group consolidation management for the coordinator.
// It encapsulates the logic for consolidating multiple task branches within an
// execution group into a single stable branch.
package consolidation

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// AggregatedTaskContext holds consolidated information from all task completion files.
type AggregatedTaskContext struct {
	// TaskSummaries maps task ID to its completion summary
	TaskSummaries map[string]string

	// AllIssues aggregates issues from all tasks (prefixed with task ID)
	AllIssues []string

	// AllSuggestions aggregates suggestions from all tasks
	AllSuggestions []string

	// Dependencies aggregates unique dependencies across tasks
	Dependencies []string

	// Notes aggregates implementation notes from tasks
	Notes []string
}

// TaskWorktreeInfo holds information about a task's worktree for consolidation.
type TaskWorktreeInfo struct {
	TaskID       string
	TaskTitle    string
	WorktreePath string
	Branch       string
}

// GroupConsolidationCompletion holds the result of a group consolidation.
type GroupConsolidationCompletion struct {
	GroupIndex         int
	Status             string // "complete" or "failed"
	BranchName         string
	TasksConsolidated  []string
	ConflictsResolved  []ConflictResolution
	Verification       VerificationResult
	Notes              string
	IssuesForNextGroup []string
}

// ConflictResolution describes how a merge conflict was resolved.
type ConflictResolution struct {
	File       string
	Resolution string
}

// VerificationResult holds the result of running verification commands.
type VerificationResult struct {
	ProjectType    string
	CommandsRun    []CommandResult
	OverallSuccess bool
	Summary        string
}

// CommandResult holds the result of a single verification command.
type CommandResult struct {
	Name    string
	Command string
	Success bool
}

// SessionState defines session state operations needed by the group manager.
type SessionState interface {
	// GetPlanSummary returns the plan summary
	GetPlanSummary() string

	// GetExecutionOrder returns the execution order (groups of task IDs)
	GetExecutionOrder() [][]string

	// GetTaskCommitCounts returns the commit counts for each task
	GetTaskCommitCounts() map[string]int

	// GetTask returns a task by ID
	GetTask(taskID string) any

	// GetGroupConsolidatorIDs returns group consolidator instance IDs
	GetGroupConsolidatorIDs() []string

	// SetGroupConsolidatorID sets a group consolidator instance ID
	SetGroupConsolidatorID(groupIndex int, instanceID string)

	// GetGroupConsolidatedBranches returns consolidated branch names per group
	GetGroupConsolidatedBranches() []string

	// SetGroupConsolidatedBranch sets a consolidated branch name
	SetGroupConsolidatedBranch(groupIndex int, branchName string)

	// GetGroupConsolidationContexts returns consolidation contexts per group
	GetGroupConsolidationContexts() []*GroupConsolidationCompletion

	// SetGroupConsolidationContext sets a consolidation context
	SetGroupConsolidationContext(groupIndex int, context *GroupConsolidationCompletion)

	// GetBranchPrefix returns the configured branch prefix
	GetBranchPrefix() string

	// GetSessionID returns the session ID
	GetSessionID() string

	// IsMultiPass returns true if multi-pass mode is enabled
	IsMultiPass() bool
}

// InstanceInfo provides information about instances.
type InstanceInfo interface {
	GetID() string
	GetWorktreePath() string
	GetBranch() string
	GetTask() string
}

// InstanceStore provides access to instances.
type InstanceStore interface {
	// GetInstances returns all instances
	GetInstances() []InstanceInfo
}

// InstanceOperations provides instance management operations.
type InstanceOperations interface {
	// AddInstance creates a new instance
	AddInstance(session any, task string) (any, error)
	// AddInstanceFromBranch creates a new instance from a specific branch
	AddInstanceFromBranch(session any, task string, baseBranch string) (any, error)
	// StartInstance starts an instance
	StartInstance(inst any) error
	// StopInstance stops an instance
	StopInstance(inst any) error
	// GetInstance returns an instance by ID
	GetInstance(id string) any
}

// WorktreeOperations provides git worktree operations.
type WorktreeOperations interface {
	// FindMainBranch returns the main branch name
	FindMainBranch() string
	// CreateBranchFrom creates a new branch from a base
	CreateBranchFrom(branchName, baseBranch string) error
	// CreateWorktreeFromBranch creates a worktree from a branch
	CreateWorktreeFromBranch(worktreePath, branchName string) error
	// Remove removes a worktree
	Remove(worktreePath string) error
	// CherryPickBranch cherry-picks all commits from a branch
	CherryPickBranch(worktreePath, sourceBranch string) error
	// AbortCherryPick aborts a failed cherry-pick
	AbortCherryPick(worktreePath string) error
	// CountCommitsBetween counts commits between two refs
	CountCommitsBetween(worktreePath, baseRef, headRef string) (int, error)
	// Push pushes the branch to remote
	Push(worktreePath string, force bool) error
}

// CompletionFileParser parses task completion files.
type CompletionFileParser interface {
	// ParseTaskCompletionFile parses a task completion file
	ParseTaskCompletionFile(worktreePath string) (*TaskCompletionData, error)
	// ParseGroupConsolidationCompletionFile parses a group consolidation completion file
	ParseGroupConsolidationCompletionFile(worktreePath string) (*GroupConsolidationCompletion, error)
	// GroupConsolidationCompletionFilePath returns the path to the completion file
	GroupConsolidationCompletionFilePath(worktreePath string) string
}

// TaskCompletionData holds parsed task completion information.
type TaskCompletionData struct {
	Summary      string
	Issues       []string
	Suggestions  []string
	Dependencies []string
	Notes        string
}

// InstanceGroupOperations provides instance group management.
type InstanceGroupOperations interface {
	// AddInstanceToGroup adds an instance to the appropriate group
	AddInstanceToGroup(instanceID string, isMultiPass bool)
}

// PersistenceOperations provides session persistence.
type PersistenceOperations interface {
	// SaveSession persists the session state
	SaveSession() error
}

// EventEmitter emits coordinator events.
type EventEmitter interface {
	// EmitEvent emits a coordinator event
	EmitEvent(eventType, message string)
}

// InstanceManagerChecker checks tmux session state.
type InstanceManagerChecker interface {
	// TmuxSessionExists returns true if the tmux session exists
	TmuxSessionExists() bool
}

// Context holds all dependencies needed by the group manager.
type Context struct {
	Session          SessionState
	InstanceStore    InstanceStore
	Instances        InstanceOperations
	Worktree         WorktreeOperations
	CompletionParser CompletionFileParser
	InstanceGroups   InstanceGroupOperations
	Persistence      PersistenceOperations
	EventEmitter     EventEmitter
	ClaudioDir       string
	Logger           *logging.Logger
}

// GroupManager handles group consolidation operations.
// It encapsulates the logic for consolidating task branches within
// execution groups and managing consolidator instances.
type GroupManager struct {
	ctx    *Context
	logger *logging.Logger
	mu     sync.Mutex
}

// NewGroupManager creates a new group manager with the given context.
func NewGroupManager(ctx *Context) *GroupManager {
	logger := logging.NopLogger()
	if ctx != nil && ctx.Logger != nil {
		logger = ctx.Logger.WithPhase("group-consolidation")
	}
	return &GroupManager{
		ctx:    ctx,
		logger: logger,
	}
}

// GetBaseBranchForGroup returns the base branch for tasks in a group.
// For group 0, this is the main branch. For other groups, it's the
// consolidated branch from the previous group.
func (m *GroupManager) GetBaseBranchForGroup(groupIndex int) string {
	if m.ctx == nil || m.ctx.Session == nil {
		return ""
	}

	if groupIndex == 0 {
		return "" // Use default (HEAD/main)
	}

	// Check if we have a consolidated branch from the previous group
	previousGroupIndex := groupIndex - 1
	branches := m.ctx.Session.GetGroupConsolidatedBranches()
	if previousGroupIndex < len(branches) {
		consolidatedBranch := branches[previousGroupIndex]
		if consolidatedBranch != "" {
			return consolidatedBranch
		}
	}

	return "" // Use default
}

// GatherTaskCompletionContextForGroup reads completion files from all completed
// tasks in a group and aggregates the context for the group consolidator.
func (m *GroupManager) GatherTaskCompletionContextForGroup(groupIndex int) *AggregatedTaskContext {
	ctx := &AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      make([]string, 0),
		AllSuggestions: make([]string, 0),
		Dependencies:   make([]string, 0),
		Notes:          make([]string, 0),
	}

	if m.ctx == nil || m.ctx.Session == nil || m.ctx.CompletionParser == nil {
		return ctx
	}

	executionOrder := m.ctx.Session.GetExecutionOrder()
	if groupIndex >= len(executionOrder) {
		return ctx
	}

	taskIDs := executionOrder[groupIndex]
	seenDeps := make(map[string]bool)

	for _, taskID := range taskIDs {
		// Find the instance for this task
		var inst InstanceInfo
		if m.ctx.InstanceStore != nil {
			for _, i := range m.ctx.InstanceStore.GetInstances() {
				if strings.Contains(i.GetTask(), taskID) {
					inst = i
					break
				}
			}
		}
		if inst == nil || inst.GetWorktreePath() == "" {
			continue
		}

		// Try to read the completion file
		completion, err := m.ctx.CompletionParser.ParseTaskCompletionFile(inst.GetWorktreePath())
		if err != nil {
			continue // No completion file or invalid
		}

		// Store task summary
		ctx.TaskSummaries[taskID] = completion.Summary

		// Aggregate issues (prefix with task ID for context)
		for _, issue := range completion.Issues {
			if issue != "" {
				ctx.AllIssues = append(ctx.AllIssues, fmt.Sprintf("[%s] %s", taskID, issue))
			}
		}

		// Aggregate suggestions
		for _, suggestion := range completion.Suggestions {
			if suggestion != "" {
				ctx.AllSuggestions = append(ctx.AllSuggestions, fmt.Sprintf("[%s] %s", taskID, suggestion))
			}
		}

		// Aggregate dependencies (deduplicated)
		for _, dep := range completion.Dependencies {
			if dep != "" && !seenDeps[dep] {
				seenDeps[dep] = true
				ctx.Dependencies = append(ctx.Dependencies, dep)
			}
		}

		// Collect notes
		if completion.Notes != "" {
			ctx.Notes = append(ctx.Notes, fmt.Sprintf("**%s**: %s", taskID, completion.Notes))
		}
	}

	return ctx
}

// GetTaskBranchesForGroup returns the branches for all tasks in a group.
func (m *GroupManager) GetTaskBranchesForGroup(groupIndex int) []TaskWorktreeInfo {
	if m.ctx == nil || m.ctx.Session == nil {
		return nil
	}

	executionOrder := m.ctx.Session.GetExecutionOrder()
	if groupIndex >= len(executionOrder) {
		return nil
	}

	taskIDs := executionOrder[groupIndex]
	var branches []TaskWorktreeInfo

	for _, taskID := range taskIDs {
		task := m.ctx.Session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Get task title
		taskTitle := taskID
		if titler, ok := task.(interface{ GetTitle() string }); ok {
			taskTitle = titler.GetTitle()
		}

		// Find the instance for this task
		if m.ctx.InstanceStore != nil {
			for _, inst := range m.ctx.InstanceStore.GetInstances() {
				if strings.Contains(inst.GetTask(), taskID) {
					branches = append(branches, TaskWorktreeInfo{
						TaskID:       taskID,
						TaskTitle:    taskTitle,
						WorktreePath: inst.GetWorktreePath(),
						Branch:       inst.GetBranch(),
					})
					break
				}
			}
		}
	}

	return branches
}

// ConsolidateGroupWithVerification consolidates a group and verifies commits exist.
// This performs the consolidation without starting a Claude session.
// Coverage: Requires git worktree operations, branch creation, and cherry-pick infrastructure.
func (m *GroupManager) ConsolidateGroupWithVerification(groupIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx == nil || m.ctx.Session == nil || m.ctx.Worktree == nil {
		return fmt.Errorf("no session or worktree")
	}

	executionOrder := m.ctx.Session.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := executionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil // Empty group, nothing to consolidate
	}

	// Collect task branches for this group, filtering to only those with verified commits
	commitCounts := m.ctx.Session.GetTaskCommitCounts()
	var taskBranches []string
	var activeTasks []string

	for _, taskID := range taskIDs {
		// Skip tasks that failed or have no commits
		commitCount, ok := commitCounts[taskID]
		if !ok || commitCount == 0 {
			continue
		}

		task := m.ctx.Session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Get task title for branch matching
		taskTitle := taskID
		if titler, ok := task.(interface{ GetTitle() string }); ok {
			taskTitle = titler.GetTitle()
		}

		// Find the instance that executed this task
		if m.ctx.InstanceStore != nil {
			for _, inst := range m.ctx.InstanceStore.GetInstances() {
				if strings.Contains(inst.GetTask(), taskID) || strings.Contains(inst.GetBranch(), slugify(taskTitle)) {
					taskBranches = append(taskBranches, inst.GetBranch())
					activeTasks = append(activeTasks, taskID)
					break
				}
			}
		}
	}

	if len(taskBranches) == 0 {
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	// Generate consolidated branch name
	branchPrefix := m.ctx.Session.GetBranchPrefix()
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}
	planID := m.ctx.Session.GetSessionID()
	if len(planID) > 8 {
		planID = planID[:8]
	}
	consolidatedBranch := fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)

	// Determine base branch
	var baseBranch string
	if groupIndex == 0 {
		baseBranch = m.ctx.Worktree.FindMainBranch()
	} else {
		branches := m.ctx.Session.GetGroupConsolidatedBranches()
		if groupIndex-1 < len(branches) {
			baseBranch = branches[groupIndex-1]
		} else {
			baseBranch = m.ctx.Worktree.FindMainBranch()
		}
	}

	// Create the consolidated branch from the base
	if err := m.ctx.Worktree.CreateBranchFrom(consolidatedBranch, baseBranch); err != nil {
		return fmt.Errorf("failed to create consolidated branch %s: %w", consolidatedBranch, err)
	}

	// Create a temporary worktree for cherry-picking
	worktreeBase := fmt.Sprintf("%s/consolidation-group-%d", m.ctx.ClaudioDir, groupIndex)
	if err := m.ctx.Worktree.CreateWorktreeFromBranch(worktreeBase, consolidatedBranch); err != nil {
		return fmt.Errorf("failed to create consolidation worktree: %w", err)
	}
	defer func() {
		if err := m.ctx.Worktree.Remove(worktreeBase); err != nil {
			m.logger.Warn("failed to clean up consolidation worktree",
				"worktree_path", worktreeBase,
				"group_index", groupIndex,
				"error", err)
		}
	}()

	// Cherry-pick commits from each task branch - failures are blocking
	for i, branch := range taskBranches {
		if err := m.ctx.Worktree.CherryPickBranch(worktreeBase, branch); err != nil {
			if abortErr := m.ctx.Worktree.AbortCherryPick(worktreeBase); abortErr != nil {
				m.logger.Warn("failed to abort cherry-pick after error",
					"cherry_pick_error", err,
					"abort_error", abortErr,
					"worktree", worktreeBase)
			}
			return fmt.Errorf("failed to cherry-pick task %s (branch %s): %w", activeTasks[i], branch, err)
		}
	}

	// Verify the consolidated branch has commits
	consolidatedCommitCount, err := m.ctx.Worktree.CountCommitsBetween(worktreeBase, baseBranch, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to verify consolidated branch commits: %w", err)
	}

	if consolidatedCommitCount == 0 {
		return fmt.Errorf("consolidated branch has no commits after cherry-picking %d branches", len(taskBranches))
	}

	// Push the consolidated branch
	if err := m.ctx.Worktree.Push(worktreeBase, false); err != nil {
		if m.ctx.EventEmitter != nil {
			m.ctx.EventEmitter.EmitEvent("group_complete",
				fmt.Sprintf("Warning: failed to push consolidated branch %s: %v", consolidatedBranch, err))
		}
		// Not fatal - branch exists locally
	}

	// Store the consolidated branch
	m.ctx.Session.SetGroupConsolidatedBranch(groupIndex, consolidatedBranch)

	if m.ctx.EventEmitter != nil {
		m.ctx.EventEmitter.EmitEvent("group_complete",
			fmt.Sprintf("Group %d consolidated into %s (%d commits from %d tasks)",
				groupIndex+1, consolidatedBranch, consolidatedCommitCount, len(taskBranches)))
	}

	return nil
}

// StartGroupConsolidatorSession creates and starts a Claude session for consolidating a group.
// Coverage: Requires real instance management, worktree creation, and session spawning infrastructure.
func (m *GroupManager) StartGroupConsolidatorSession(groupIndex int, baseSession any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx == nil || m.ctx.Session == nil || m.ctx.Instances == nil {
		return fmt.Errorf("no session or instances")
	}

	executionOrder := m.ctx.Session.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := executionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil // Empty group, nothing to consolidate
	}

	// Check if there are any tasks with verified commits
	commitCounts := m.ctx.Session.GetTaskCommitCounts()
	var activeTasks []string
	for _, taskID := range taskIDs {
		commitCount, ok := commitCounts[taskID]
		if ok && commitCount > 0 {
			activeTasks = append(activeTasks, taskID)
		}
	}

	if len(activeTasks) == 0 {
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	// Build the consolidator prompt
	prompt := m.buildGroupConsolidatorPrompt(groupIndex)

	// Determine base branch for the consolidator's worktree
	baseBranch := m.GetBaseBranchForGroup(groupIndex)

	// Create the consolidator instance
	var inst any
	var err error
	if baseBranch != "" {
		inst, err = m.ctx.Instances.AddInstanceFromBranch(baseSession, prompt, baseBranch)
	} else {
		inst, err = m.ctx.Instances.AddInstance(baseSession, prompt)
	}
	if err != nil {
		return fmt.Errorf("failed to create group consolidator instance: %w", err)
	}

	// Get instance ID
	instanceID := ""
	if getter, ok := inst.(interface{ GetID() string }); ok {
		instanceID = getter.GetID()
	}

	// Add consolidator instance to the ultraplan group for sidebar display
	if m.ctx.InstanceGroups != nil {
		m.ctx.InstanceGroups.AddInstanceToGroup(instanceID, m.ctx.Session.IsMultiPass())
	}

	// Store the consolidator instance ID
	m.ctx.Session.SetGroupConsolidatorID(groupIndex, instanceID)

	// Save state
	if m.ctx.Persistence != nil {
		if err := m.ctx.Persistence.SaveSession(); err != nil {
			m.logger.Warn("failed to persist session after storing consolidator ID",
				"error", err,
				"group_index", groupIndex,
				"instance_id", instanceID)
		}
	}

	// Emit event
	if m.ctx.EventEmitter != nil {
		m.ctx.EventEmitter.EmitEvent("group_complete",
			fmt.Sprintf("Starting group %d consolidator session", groupIndex+1))
	}

	// Start the instance
	if err := m.ctx.Instances.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start group consolidator instance: %w", err)
	}

	// Monitor the consolidator synchronously (blocks until completion)
	return m.monitorGroupConsolidator(groupIndex, instanceID)
}

// monitorGroupConsolidator monitors the group consolidator instance and waits for completion.
// Coverage: Requires running instance, file system polling, and real-time completion monitoring.
func (m *GroupManager) monitorGroupConsolidator(groupIndex int, instanceID string) error {
	if m.ctx == nil || m.ctx.CompletionParser == nil || m.ctx.Instances == nil {
		return fmt.Errorf("missing required context")
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("consolidator timeout")

		case <-ticker.C:
			inst := m.ctx.Instances.GetInstance(instanceID)
			if inst == nil {
				return fmt.Errorf("consolidator instance not found")
			}

			// Get worktree path
			worktreePath := ""
			if getter, ok := inst.(interface{ GetWorktreePath() string }); ok {
				worktreePath = getter.GetWorktreePath()
			}

			// Check for the completion file
			if worktreePath != "" {
				completionPath := m.ctx.CompletionParser.GroupConsolidationCompletionFilePath(worktreePath)
				if _, err := os.Stat(completionPath); err == nil {
					// Parse the completion file
					completion, err := m.ctx.CompletionParser.ParseGroupConsolidationCompletionFile(worktreePath)
					if err != nil {
						// File exists but is invalid/incomplete - might still be writing
						continue
					}

					// Check status
					if completion.Status == "failed" {
						if err := m.ctx.Instances.StopInstance(inst); err != nil {
							m.logger.Warn("failed to stop consolidator instance after failure",
								"group_index", groupIndex,
								"instance_id", instanceID,
								"error", err)
						}
						return fmt.Errorf("group %d consolidation failed: %s", groupIndex+1, completion.Notes)
					}

					// Store the consolidated branch
					m.ctx.Session.SetGroupConsolidatedBranch(groupIndex, completion.BranchName)

					// Store the consolidation context for the next group
					m.ctx.Session.SetGroupConsolidationContext(groupIndex, completion)

					// Persist state
					if m.ctx.Persistence != nil {
						if err := m.ctx.Persistence.SaveSession(); err != nil {
							m.logger.Warn("failed to persist session after group consolidation",
								"group_index", groupIndex,
								"error", err)
						}
					}

					// Stop the consolidator instance to free up resources
					if err := m.ctx.Instances.StopInstance(inst); err != nil {
						m.logger.Warn("failed to stop consolidator instance after success",
							"group_index", groupIndex,
							"instance_id", instanceID,
							"error", err)
					}

					// Emit success event
					if m.ctx.EventEmitter != nil {
						m.ctx.EventEmitter.EmitEvent("group_complete",
							fmt.Sprintf("Group %d consolidated into %s (verification: %v)",
								groupIndex+1, completion.BranchName, completion.Verification.OverallSuccess))
					}

					return nil
				}
			}

			// Check if instance has failed/exited
			status := ""
			if getter, ok := inst.(interface{ GetStatus() string }); ok {
				status = getter.GetStatus()
			}

			switch status {
			case "error":
				return fmt.Errorf("consolidator instance failed with error")
			case "completed":
				// Instance completed without writing completion file - might need to check tmux
				// For now, continue monitoring briefly then fail
				continue
			}
		}
	}
}

// buildGroupConsolidatorPrompt builds the prompt for a per-group consolidator session.
func (m *GroupManager) buildGroupConsolidatorPrompt(groupIndex int) string {
	if m.ctx == nil || m.ctx.Session == nil {
		return ""
	}

	taskContext := m.GatherTaskCompletionContextForGroup(groupIndex)
	taskBranches := m.GetTaskBranchesForGroup(groupIndex)

	// Determine base branch
	var baseBranch string
	if groupIndex == 0 {
		if m.ctx.Worktree != nil {
			baseBranch = m.ctx.Worktree.FindMainBranch()
		}
		// baseBranch remains empty if no Worktree (will use default)
	} else {
		branches := m.ctx.Session.GetGroupConsolidatedBranches()
		previousGroupIndex := groupIndex - 1
		if previousGroupIndex >= 0 && previousGroupIndex < len(branches) {
			baseBranch = branches[previousGroupIndex]
		} else if m.ctx.Worktree != nil {
			baseBranch = m.ctx.Worktree.FindMainBranch()
		}
	}

	// Generate consolidated branch name
	branchPrefix := m.ctx.Session.GetBranchPrefix()
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}
	planID := m.ctx.Session.GetSessionID()
	if len(planID) > 8 {
		planID = planID[:8]
	}
	consolidatedBranch := fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Group %d Consolidation\n\n", groupIndex+1))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", m.ctx.Session.GetPlanSummary()))

	sb.WriteString("## Objective\n\n")
	sb.WriteString("Consolidate all completed task branches from this group into a single stable branch.\n")
	sb.WriteString("You must resolve any merge conflicts, verify the consolidated code works, and pass context to the next group.\n\n")

	// Tasks completed in this group
	sb.WriteString("## Tasks Completed in This Group\n\n")
	for _, branch := range taskBranches {
		sb.WriteString(fmt.Sprintf("### %s: %s\n", branch.TaskID, branch.TaskTitle))
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", branch.Branch))
		sb.WriteString(fmt.Sprintf("- Worktree: `%s`\n", branch.WorktreePath))
		if summary, ok := taskContext.TaskSummaries[branch.TaskID]; ok && summary != "" {
			sb.WriteString(fmt.Sprintf("- Summary: %s\n", summary))
		}
		sb.WriteString("\n")
	}

	// Context from task completion files
	if len(taskContext.Notes) > 0 {
		sb.WriteString("## Implementation Notes from Tasks\n\n")
		for _, note := range taskContext.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
		sb.WriteString("\n")
	}

	if len(taskContext.AllIssues) > 0 {
		sb.WriteString("## Issues Raised by Tasks\n\n")
		for _, issue := range taskContext.AllIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	if len(taskContext.AllSuggestions) > 0 {
		sb.WriteString("## Integration Suggestions from Tasks\n\n")
		for _, suggestion := range taskContext.AllSuggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
		sb.WriteString("\n")
	}

	// Context from previous group's consolidator
	if groupIndex > 0 {
		contexts := m.ctx.Session.GetGroupConsolidationContexts()
		if groupIndex-1 < len(contexts) {
			prevContext := contexts[groupIndex-1]
			if prevContext != nil {
				sb.WriteString("## Context from Previous Group's Consolidator\n\n")
				if prevContext.Notes != "" {
					sb.WriteString(fmt.Sprintf("**Notes**: %s\n\n", prevContext.Notes))
				}
				if len(prevContext.IssuesForNextGroup) > 0 {
					sb.WriteString("**Issues/Warnings to Address**:\n")
					for _, issue := range prevContext.IssuesForNextGroup {
						sb.WriteString(fmt.Sprintf("- %s\n", issue))
					}
					sb.WriteString("\n")
				}
			}
		}
	}

	// Branch configuration
	sb.WriteString("## Branch Configuration\n\n")
	sb.WriteString(fmt.Sprintf("- **Base branch**: `%s`\n", baseBranch))
	sb.WriteString(fmt.Sprintf("- **Target consolidated branch**: `%s`\n", consolidatedBranch))
	sb.WriteString(fmt.Sprintf("- **Task branches to consolidate**: %d\n\n", len(taskBranches)))

	// Instructions
	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Create the consolidated branch** from the base branch:\n")
	sb.WriteString(fmt.Sprintf("   ```bash\n   git checkout -b %s %s\n   ```\n\n", consolidatedBranch, baseBranch))

	sb.WriteString("2. **Cherry-pick commits** from each task branch in order.\n\n")
	sb.WriteString("3. **Run verification** to ensure the consolidated code is stable.\n\n")
	sb.WriteString("4. **Push the consolidated branch** to the remote.\n\n")
	sb.WriteString("5. **Write the completion file** to signal success.\n\n")

	return sb.String()
}

// slugify converts a string to a URL-friendly slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}

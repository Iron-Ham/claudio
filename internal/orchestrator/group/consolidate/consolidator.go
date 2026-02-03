package consolidate

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

// Consolidator handles group consolidation logic for ultra-plan workflows.
type Consolidator struct {
	coord CoordinatorInterface
}

// NewConsolidator creates a new group consolidator.
func NewConsolidator(coord CoordinatorInterface) *Consolidator {
	return &Consolidator{coord: coord}
}

// ConsolidateWithVerification consolidates a group and verifies commits exist.
func (c *Consolidator) ConsolidateWithVerification(groupIndex int) error {
	session := c.coord.Session()
	if session == nil || session.GetPlan() == nil {
		return fmt.Errorf("no session or plan")
	}

	plan := session.GetPlan()
	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := executionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil // Empty group, nothing to consolidate
	}

	// Collect task branches for this group
	var taskBranches []string
	var activeTasks []string
	commitCounts := session.GetTaskCommitCounts()

	for _, taskID := range taskIDs {
		// Skip tasks that failed or have no commits
		commitCount, ok := commitCounts[taskID]
		if !ok || commitCount == 0 {
			continue
		}

		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Find the instance that executed this task
		baseSession := c.coord.BaseSession()
		for _, inst := range baseSession.GetInstances() {
			taskTitle := task.GetTitle()
			if strings.Contains(inst.GetTask(), taskID) || strings.Contains(inst.GetBranch(), slugify(taskTitle)) {
				taskBranches = append(taskBranches, inst.GetBranch())
				activeTasks = append(activeTasks, taskID)
				break
			}
		}
	}

	if len(taskBranches) == 0 {
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	// Generate consolidated branch name
	orch := c.coord.Orchestrator()
	wt := orch.Worktree()

	branchPrefix := session.GetConfig().GetBranchPrefix()
	if branchPrefix == "" {
		branchPrefix = orch.GetBranchPrefix()
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}
	planID := session.GetID()
	if len(planID) > 8 {
		planID = planID[:8]
	}
	consolidatedBranch := fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)

	// Determine base branch
	var baseBranch string
	consolidatedBranches := session.GetGroupConsolidatedBranches()
	if groupIndex == 0 {
		baseBranch = wt.FindMainBranch()
	} else if groupIndex-1 < len(consolidatedBranches) {
		baseBranch = consolidatedBranches[groupIndex-1]
	} else {
		baseBranch = wt.FindMainBranch()
	}

	// Create the consolidated branch from the base
	if err := wt.CreateBranchFrom(consolidatedBranch, baseBranch); err != nil {
		return fmt.Errorf("failed to create consolidated branch %s: %w", consolidatedBranch, err)
	}

	// Create a temporary worktree for cherry-picking
	worktreeBase := fmt.Sprintf("%s/consolidation-group-%d", orch.GetClaudioDir(), groupIndex)
	if err := wt.CreateWorktreeFromBranch(worktreeBase, consolidatedBranch); err != nil {
		return fmt.Errorf("failed to create consolidation worktree: %w", err)
	}
	defer func() {
		_ = wt.Remove(worktreeBase)
	}()

	// Cherry-pick commits from each task branch
	for i, branch := range taskBranches {
		if err := wt.CherryPickBranch(worktreeBase, branch); err != nil {
			_ = wt.AbortCherryPick(worktreeBase)
			return fmt.Errorf("failed to cherry-pick task %s (branch %s): %w", activeTasks[i], branch, err)
		}
	}

	// Verify the consolidated branch has commits
	consolidatedCommitCount, err := wt.CountCommitsBetween(worktreeBase, baseBranch, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to verify consolidated branch commits: %w", err)
	}

	if consolidatedCommitCount == 0 {
		return fmt.Errorf("consolidated branch has no commits after cherry-picking %d branches", len(taskBranches))
	}

	// Push the consolidated branch
	if err := wt.Push(worktreeBase, false); err != nil {
		c.coord.Manager().EmitEvent(EventGroupComplete,
			fmt.Sprintf("Warning: failed to push consolidated branch %s: %v", consolidatedBranch, err))
	}

	// Store the consolidated branch
	c.coord.Lock()
	session.EnsureGroupArraysCapacity(groupIndex)
	session.SetGroupConsolidatedBranch(groupIndex, consolidatedBranch)
	c.coord.Unlock()

	c.coord.Manager().EmitEvent(EventGroupComplete,
		fmt.Sprintf("Group %d consolidated into %s (%d commits from %d tasks)",
			groupIndex+1, consolidatedBranch, consolidatedCommitCount, len(taskBranches)))

	return nil
}

// GetBaseBranchForGroup returns the base branch for tasks in a group.
func (c *Consolidator) GetBaseBranchForGroup(groupIndex int) string {
	session := c.coord.Session()

	if groupIndex == 0 {
		return "" // Use default (HEAD/main)
	}

	consolidatedBranches := session.GetGroupConsolidatedBranches()
	previousGroupIndex := groupIndex - 1
	if session != nil && previousGroupIndex < len(consolidatedBranches) {
		consolidatedBranch := consolidatedBranches[previousGroupIndex]
		if consolidatedBranch != "" {
			return consolidatedBranch
		}
	}

	return "" // Use default
}

// GatherTaskCompletionContext gathers context from completed tasks in a group.
func (c *Consolidator) GatherTaskCompletionContext(groupIndex int) *types.AggregatedTaskContext {
	session := c.coord.Session()
	if session == nil || session.GetPlan() == nil {
		return &types.AggregatedTaskContext{TaskSummaries: make(map[string]string)}
	}

	plan := session.GetPlan()
	executionOrder := plan.GetExecutionOrder()
	if groupIndex >= len(executionOrder) {
		return &types.AggregatedTaskContext{TaskSummaries: make(map[string]string)}
	}

	taskIDs := executionOrder[groupIndex]
	context := &types.AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      make([]string, 0),
		AllSuggestions: make([]string, 0),
		Dependencies:   make([]string, 0),
		Notes:          make([]string, 0),
	}

	seenDeps := make(map[string]bool)
	baseSession := c.coord.BaseSession()

	for _, taskID := range taskIDs {
		var inst InstanceInterface
		for _, i := range baseSession.GetInstances() {
			if strings.Contains(i.GetTask(), taskID) {
				inst = i
				break
			}
		}
		if inst == nil || inst.GetWorktreePath() == "" {
			continue
		}

		completion, err := types.ParseTaskCompletionFile(inst.GetWorktreePath())
		if err != nil {
			continue
		}

		context.TaskSummaries[taskID] = completion.Summary

		for _, issue := range completion.Issues {
			if issue != "" {
				context.AllIssues = append(context.AllIssues, fmt.Sprintf("[%s] %s", taskID, issue))
			}
		}

		for _, suggestion := range completion.Suggestions {
			if suggestion != "" {
				context.AllSuggestions = append(context.AllSuggestions, fmt.Sprintf("[%s] %s", taskID, suggestion))
			}
		}

		for _, dep := range completion.Dependencies {
			if dep != "" && !seenDeps[dep] {
				seenDeps[dep] = true
				context.Dependencies = append(context.Dependencies, dep)
			}
		}

		if completion.Notes != "" {
			context.Notes = append(context.Notes, fmt.Sprintf("**%s**: %s", taskID, completion.Notes))
		}
	}

	return context
}

// GetTaskBranches returns branches and commit counts for tasks in a group.
func (c *Consolidator) GetTaskBranches(groupIndex int) []TaskWorktreeInfo {
	session := c.coord.Session()
	if session == nil || session.GetPlan() == nil {
		return nil
	}

	plan := session.GetPlan()
	executionOrder := plan.GetExecutionOrder()
	if groupIndex >= len(executionOrder) {
		return nil
	}

	taskIDs := executionOrder[groupIndex]
	var branches []TaskWorktreeInfo
	baseSession := c.coord.BaseSession()

	for _, taskID := range taskIDs {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		for _, inst := range baseSession.GetInstances() {
			if strings.Contains(inst.GetTask(), taskID) {
				branches = append(branches, TaskWorktreeInfo{
					TaskID:       taskID,
					TaskTitle:    task.GetTitle(),
					WorktreePath: inst.GetWorktreePath(),
					Branch:       inst.GetBranch(),
				})
				break
			}
		}
	}

	return branches
}

// BuildPrompt builds the prompt for a per-group consolidator session.
func (c *Consolidator) BuildPrompt(groupIndex int) string {
	session := c.coord.Session()
	if session == nil || session.GetPlan() == nil {
		return ""
	}

	plan := session.GetPlan()
	taskContext := c.GatherTaskCompletionContext(groupIndex)
	taskBranches := c.GetTaskBranches(groupIndex)

	orch := c.coord.Orchestrator()
	wt := orch.Worktree()

	// Determine base branch
	var baseBranch string
	consolidatedBranches := session.GetGroupConsolidatedBranches()
	if groupIndex == 0 {
		baseBranch = wt.FindMainBranch()
	} else if groupIndex-1 < len(consolidatedBranches) {
		baseBranch = consolidatedBranches[groupIndex-1]
	} else {
		baseBranch = wt.FindMainBranch()
	}

	// Generate consolidated branch name
	branchPrefix := session.GetConfig().GetBranchPrefix()
	if branchPrefix == "" {
		branchPrefix = orch.GetBranchPrefix()
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}
	planID := session.GetID()
	if len(planID) > 8 {
		planID = planID[:8]
	}
	consolidatedBranch := fmt.Sprintf("%s/ultraplan-%s-group-%d", branchPrefix, planID, groupIndex+1)

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Group %d Consolidation\n\n", groupIndex+1))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", plan.GetSummary()))

	sb.WriteString("## Objective\n\n")
	sb.WriteString("Consolidate all completed task branches from this group into a single stable branch.\n")
	sb.WriteString("You must resolve any merge conflicts, verify the consolidated code works, and pass context to the next group.\n\n")

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
	contexts := session.GetGroupConsolidationContexts()
	if groupIndex > 0 && groupIndex-1 < len(contexts) {
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

	sb.WriteString("## Branch Configuration\n\n")
	sb.WriteString(fmt.Sprintf("- **Base branch**: `%s`\n", baseBranch))
	sb.WriteString(fmt.Sprintf("- **Target consolidated branch**: `%s`\n", consolidatedBranch))
	sb.WriteString(fmt.Sprintf("- **Task branches to consolidate**: %d\n\n", len(taskBranches)))

	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Create the consolidated branch** from the base branch:\n")
	sb.WriteString(fmt.Sprintf("   ```bash\n   git checkout -b %s %s\n   ```\n\n", consolidatedBranch, baseBranch))

	sb.WriteString("2. **Cherry-pick commits** from each task branch in order.\n\n")
	sb.WriteString("3. **Run verification** to ensure the consolidated code is stable.\n\n")
	sb.WriteString("4. **Push the consolidated branch** to the remote.\n\n")
	sb.WriteString("5. **Write the completion file** to signal success.\n\n")

	sb.WriteString("## Conflict Resolution Guidelines\n\n")
	sb.WriteString("- Prefer changes that preserve functionality from all tasks\n")
	sb.WriteString("- If there are conflicting approaches, choose the more robust one\n\n")

	sb.WriteString("## Completion Protocol\n\n")
	sb.WriteString("Write `.claudio-group-consolidation-complete.json` when done.\n")

	return sb.String()
}

// StartSession creates and starts a backend session for consolidating a group.
func (c *Consolidator) StartSession(groupIndex int) error {
	session := c.coord.Session()
	if session == nil || session.GetPlan() == nil {
		return fmt.Errorf("no session or plan")
	}

	plan := session.GetPlan()
	executionOrder := plan.GetExecutionOrder()
	if groupIndex < 0 || groupIndex >= len(executionOrder) {
		return fmt.Errorf("invalid group index: %d", groupIndex)
	}

	taskIDs := executionOrder[groupIndex]
	if len(taskIDs) == 0 {
		return nil
	}

	// Check if there are any tasks with verified commits
	var activeTasks []string
	commitCounts := session.GetTaskCommitCounts()
	for _, taskID := range taskIDs {
		commitCount, ok := commitCounts[taskID]
		if ok && commitCount > 0 {
			activeTasks = append(activeTasks, taskID)
		}
	}

	if len(activeTasks) == 0 {
		return fmt.Errorf("no task branches with verified commits found for group %d", groupIndex)
	}

	prompt := c.BuildPrompt(groupIndex)
	baseBranch := c.GetBaseBranchForGroup(groupIndex)

	orch := c.coord.Orchestrator()
	baseSession := c.coord.BaseSession()

	var inst InstanceInterface
	var err error
	if baseBranch != "" {
		inst, err = orch.AddInstanceFromBranch(baseSession, prompt, baseBranch)
	} else {
		inst, err = orch.AddInstance(baseSession, prompt)
	}
	if err != nil {
		return fmt.Errorf("failed to create group consolidator instance: %w", err)
	}

	// Add to ultraplan group for display
	sessionType := SessionTypeUltraPlan
	if session.GetConfig().IsMultiPass() {
		sessionType = SessionTypePlanMulti
	}
	if ultraGroup := baseSession.GetGroupBySessionType(sessionType); ultraGroup != nil {
		ultraGroup.AddInstance(inst.GetID())
	}

	// Store the consolidator instance ID
	c.coord.Lock()
	session.EnsureGroupArraysCapacity(groupIndex)
	session.SetGroupConsolidatorID(groupIndex, inst.GetID())
	c.coord.Unlock()

	_ = orch.SaveSession()

	c.coord.Manager().EmitEvent(EventGroupComplete,
		fmt.Sprintf("Starting group %d consolidator session", groupIndex+1))

	if err := orch.StartInstance(inst); err != nil {
		return fmt.Errorf("failed to start group consolidator instance: %w", err)
	}

	return c.Monitor(groupIndex, inst.GetID())
}

// Monitor monitors the group consolidator instance and waits for completion.
func (c *Consolidator) Monitor(groupIndex int, instanceID string) error {
	session := c.coord.Session()
	orch := c.coord.Orchestrator()
	ctx := c.coord.Context()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled")

		case <-ticker.C:
			inst := orch.GetInstance(instanceID)
			if inst == nil {
				return fmt.Errorf("consolidator instance not found")
			}

			worktreePath := inst.GetWorktreePath()
			if worktreePath != "" {
				completionPath := types.GroupConsolidationCompletionFilePath(worktreePath)
				if _, err := os.Stat(completionPath); err == nil {
					completion, err := types.ParseGroupConsolidationCompletionFile(worktreePath)
					if err != nil {
						continue
					}

					if completion.Status == "failed" {
						_ = orch.StopInstance(inst)
						return fmt.Errorf("group %d consolidation failed: %s", groupIndex+1, completion.Notes)
					}

					c.coord.Lock()
					session.EnsureGroupArraysCapacity(groupIndex)
					session.SetGroupConsolidatedBranch(groupIndex, completion.BranchName)
					session.SetGroupConsolidationContext(groupIndex, completion)
					c.coord.Unlock()

					_ = orch.SaveSession()
					_ = orch.StopInstance(inst)

					c.coord.Manager().EmitEvent(EventGroupComplete,
						fmt.Sprintf("Group %d consolidated into %s (verification: %v)",
							groupIndex+1, completion.BranchName, completion.Verification.OverallSuccess))

					return nil
				}
			}

			status := inst.GetStatus()
			switch status {
			case StatusError:
				return fmt.Errorf("consolidator instance failed with error")
			case StatusCompleted:
				mgr := orch.GetInstanceManager(instanceID)
				if mgr != nil && mgr.TmuxSessionExists() {
					continue
				}
				return fmt.Errorf("consolidator completed without writing completion file")
			}
		}
	}
}

// slugify converts a string to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

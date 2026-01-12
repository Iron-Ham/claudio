// Package context provides utilities for gathering and aggregating context
// from various phases of ultraplan execution. The ContextGatherer extracts
// information from task completion files, synthesis results, and consolidation
// state to build prompts and context for subsequent phases.
package context

import (
	"fmt"
	"strings"
)

// TaskContext holds the context gathered for a single task execution.
// This includes information about the task's worktree, branch, and any
// completion data that may be available.
type TaskContext struct {
	TaskID       string
	TaskTitle    string
	Description  string
	WorktreePath string
	Branch       string

	// Completion data (nil if task hasn't completed or no completion file found)
	Summary       string
	FilesModified []string
	Notes         string
	Issues        []string
	Suggestions   []string
	Dependencies  []string
}

// SynthesisContext holds aggregated context for the synthesis phase.
// This includes information from all completed tasks needed to review
// and verify the work across the entire ultraplan.
type SynthesisContext struct {
	Objective      string
	TotalTasks     int
	CompletedTasks []TaskSummary
	TaskWorktrees  []TaskWorktreeInfo
	CommitCounts   map[string]int
}

// TaskSummary provides a brief overview of a completed task.
type TaskSummary struct {
	TaskID      string
	Title       string
	CommitCount int
	Summary     string
}

// TaskWorktreeInfo holds information about a task's worktree for consolidation.
type TaskWorktreeInfo struct {
	TaskID       string
	TaskTitle    string
	WorktreePath string
	Branch       string
}

// ConsolidationContext holds aggregated context for the consolidation phase.
// This combines synthesis results with task completion context.
type ConsolidationContext struct {
	Objective          string
	SynthesisStatus    string
	SynthesisNotes     string
	Recommendations    []string
	IssuesFound        []SynthesisIssue
	GroupBranches      []string
	GroupContexts      []*GroupContext
	HasPreconsolidated bool
}

// SynthesisIssue represents an issue found during synthesis.
type SynthesisIssue struct {
	TaskID      string
	Description string
	Severity    string
	Suggestion  string
}

// GroupContext holds context from a per-group consolidator.
type GroupContext struct {
	GroupIndex     int
	Branch         string
	Notes          string
	IssuesForNext  []string
	VerificationOK bool
}

// AggregatedTaskContext holds the aggregated context from task completion files.
// This is used to build prompts for consolidation and synthesis phases.
type AggregatedTaskContext struct {
	TaskSummaries  map[string]string // taskID -> summary
	AllIssues      []string          // All issues from all tasks
	AllSuggestions []string          // All suggestions from all tasks
	Dependencies   []string          // Deduplicated list of new dependencies
	Notes          []string          // Implementation notes from all tasks
}

// HasContent returns true if there is any aggregated context worth displaying.
func (a *AggregatedTaskContext) HasContent() bool {
	return len(a.AllIssues) > 0 || len(a.AllSuggestions) > 0 || len(a.Dependencies) > 0 || len(a.Notes) > 0
}

// FormatForPR formats the aggregated context for inclusion in a PR description.
func (a *AggregatedTaskContext) FormatForPR() string {
	var sb strings.Builder

	if len(a.Notes) > 0 {
		sb.WriteString("\n## Implementation Notes\n\n")
		for _, note := range a.Notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
	}

	if len(a.AllIssues) > 0 {
		sb.WriteString("\n## Issues/Concerns Flagged\n\n")
		for _, issue := range a.AllIssues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
	}

	if len(a.AllSuggestions) > 0 {
		sb.WriteString("\n## Integration Suggestions\n\n")
		for _, suggestion := range a.AllSuggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
	}

	if len(a.Dependencies) > 0 {
		sb.WriteString("\n## New Dependencies\n\n")
		for _, dep := range a.Dependencies {
			sb.WriteString(fmt.Sprintf("- `%s`\n", dep))
		}
	}

	return sb.String()
}

// InstanceFinder provides an interface for finding instances by various criteria.
// This abstraction allows the ContextGatherer to work without direct dependency
// on the orchestrator package's concrete types.
type InstanceFinder interface {
	// FindInstanceByTaskID returns instance info for a given task ID.
	// Returns nil if no instance is found.
	FindInstanceByTaskID(taskID string) *InstanceInfo

	// GetTaskInfo returns task information by ID.
	// Returns nil if task not found.
	GetTaskInfo(taskID string) *TaskInfo
}

// InstanceInfo holds the instance information needed for context gathering.
type InstanceInfo struct {
	ID           string
	WorktreePath string
	Branch       string
	Task         string
}

// TaskInfo holds the task information needed for context gathering.
type TaskInfo struct {
	ID          string
	Title       string
	Description string
	DependsOn   []string
}

// CompletionFileReader provides an interface for reading task completion files.
// This abstraction allows testing without filesystem access.
type CompletionFileReader interface {
	// ReadTaskCompletion reads and parses a task completion file from a worktree.
	// Returns nil if the file doesn't exist or is invalid.
	ReadTaskCompletion(worktreePath string) *TaskCompletionData

	// ReadSynthesisCompletion reads and parses a synthesis completion file.
	// Returns nil if the file doesn't exist or is invalid.
	ReadSynthesisCompletion(worktreePath string) *SynthesisCompletionData

	// ReadGroupConsolidationCompletion reads and parses a group consolidation completion file.
	// Returns nil if the file doesn't exist or is invalid.
	ReadGroupConsolidationCompletion(worktreePath string) *GroupConsolidationData
}

// TaskCompletionData represents the data from a task completion file.
type TaskCompletionData struct {
	TaskID        string
	Status        string
	Summary       string
	FilesModified []string
	Notes         string
	Issues        []string
	Suggestions   []string
	Dependencies  []string
}

// SynthesisCompletionData represents the data from a synthesis completion file.
type SynthesisCompletionData struct {
	Status           string
	RevisionRound    int
	IssuesFound      []SynthesisIssue
	TasksAffected    []string
	IntegrationNotes string
	Recommendations  []string
}

// GroupConsolidationData represents the data from a group consolidation completion file.
type GroupConsolidationData struct {
	GroupIndex         int
	Status             string
	BranchName         string
	TasksConsolidated  []string
	Notes              string
	IssuesForNextGroup []string
	VerificationOK     bool
}

// Gatherer collects and aggregates context from various ultraplan phases.
// It uses the provided InstanceFinder and CompletionFileReader to access
// instance and completion data without directly depending on the orchestrator types.
type Gatherer struct {
	finder InstanceFinder
	reader CompletionFileReader
}

// NewGatherer creates a new context gatherer with the provided dependencies.
func NewGatherer(finder InstanceFinder, reader CompletionFileReader) *Gatherer {
	return &Gatherer{
		finder: finder,
		reader: reader,
	}
}

// GatherTaskContext gathers context for a specific task given its worktree information.
// This method collects task metadata and any completion data available.
func (g *Gatherer) GatherTaskContext(taskID string, worktrees []TaskWorktreeInfo) *TaskContext {
	ctx := &TaskContext{
		TaskID: taskID,
	}

	// Get task info from finder
	if taskInfo := g.finder.GetTaskInfo(taskID); taskInfo != nil {
		ctx.TaskTitle = taskInfo.Title
		ctx.Description = taskInfo.Description
	}

	// Find worktree info for this task
	for _, wt := range worktrees {
		if wt.TaskID == taskID {
			ctx.WorktreePath = wt.WorktreePath
			ctx.Branch = wt.Branch
			if ctx.TaskTitle == "" {
				ctx.TaskTitle = wt.TaskTitle
			}
			break
		}
	}

	// Try to read completion data if we have a worktree path
	if ctx.WorktreePath != "" && g.reader != nil {
		if completion := g.reader.ReadTaskCompletion(ctx.WorktreePath); completion != nil {
			ctx.Summary = completion.Summary
			ctx.FilesModified = completion.FilesModified
			ctx.Notes = completion.Notes
			ctx.Issues = completion.Issues
			ctx.Suggestions = completion.Suggestions
			ctx.Dependencies = completion.Dependencies
		}
	}

	return ctx
}

// GatherAggregatedTaskContext aggregates context from multiple tasks' completion files.
// This is useful for building prompts that need information from multiple completed tasks.
func (g *Gatherer) GatherAggregatedTaskContext(taskIDs []string) *AggregatedTaskContext {
	ctx := &AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      make([]string, 0),
		AllSuggestions: make([]string, 0),
		Dependencies:   make([]string, 0),
		Notes:          make([]string, 0),
	}

	seenDeps := make(map[string]bool)

	for _, taskID := range taskIDs {
		// Find the instance for this task
		inst := g.finder.FindInstanceByTaskID(taskID)
		if inst == nil || inst.WorktreePath == "" {
			continue
		}

		// Try to read the completion file
		completion := g.reader.ReadTaskCompletion(inst.WorktreePath)
		if completion == nil {
			continue
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

// GatherSynthesisContext collects context needed for the synthesis phase.
// This aggregates information from all completed tasks to enable review
// and verification of the work.
func (g *Gatherer) GatherSynthesisContext(
	objective string,
	completedTaskIDs []string,
	commitCounts map[string]int,
) *SynthesisContext {
	ctx := &SynthesisContext{
		Objective:      objective,
		TotalTasks:     len(completedTaskIDs),
		CompletedTasks: make([]TaskSummary, 0, len(completedTaskIDs)),
		TaskWorktrees:  make([]TaskWorktreeInfo, 0, len(completedTaskIDs)),
		CommitCounts:   commitCounts,
	}

	for _, taskID := range completedTaskIDs {
		taskInfo := g.finder.GetTaskInfo(taskID)
		inst := g.finder.FindInstanceByTaskID(taskID)

		summary := TaskSummary{
			TaskID: taskID,
		}

		if taskInfo != nil {
			summary.Title = taskInfo.Title
		}

		if count, ok := commitCounts[taskID]; ok {
			summary.CommitCount = count
		}

		// Try to get summary from completion file
		if inst != nil && inst.WorktreePath != "" {
			ctx.TaskWorktrees = append(ctx.TaskWorktrees, TaskWorktreeInfo{
				TaskID:       taskID,
				TaskTitle:    summary.Title,
				WorktreePath: inst.WorktreePath,
				Branch:       inst.Branch,
			})

			if completion := g.reader.ReadTaskCompletion(inst.WorktreePath); completion != nil {
				summary.Summary = completion.Summary
			}
		}

		ctx.CompletedTasks = append(ctx.CompletedTasks, summary)
	}

	return ctx
}

// GatherConsolidationContext collects context needed for the consolidation phase.
// This combines synthesis results with task completion data and group consolidation
// context from previous groups.
func (g *Gatherer) GatherConsolidationContext(
	objective string,
	synthesisWorktree string,
	groupBranches []string,
	groupConsolidatorWorktrees []string,
) *ConsolidationContext {
	ctx := &ConsolidationContext{
		Objective:          objective,
		GroupBranches:      groupBranches,
		GroupContexts:      make([]*GroupContext, 0),
		HasPreconsolidated: len(groupBranches) > 0,
	}

	// Read synthesis completion if available
	if synthesisWorktree != "" && g.reader != nil {
		if synth := g.reader.ReadSynthesisCompletion(synthesisWorktree); synth != nil {
			ctx.SynthesisStatus = synth.Status
			ctx.SynthesisNotes = synth.IntegrationNotes
			ctx.Recommendations = synth.Recommendations
			ctx.IssuesFound = synth.IssuesFound
		}
	}

	// Read group consolidation contexts
	for i, worktree := range groupConsolidatorWorktrees {
		if worktree == "" {
			continue
		}
		if groupData := g.reader.ReadGroupConsolidationCompletion(worktree); groupData != nil {
			ctx.GroupContexts = append(ctx.GroupContexts, &GroupContext{
				GroupIndex:     i,
				Branch:         groupData.BranchName,
				Notes:          groupData.Notes,
				IssuesForNext:  groupData.IssuesForNextGroup,
				VerificationOK: groupData.VerificationOK,
			})
		}
	}

	return ctx
}

// GetTaskWorktreeInfo builds a list of TaskWorktreeInfo from completed task IDs.
// This is useful for providing worktree information to consolidation prompts.
func (g *Gatherer) GetTaskWorktreeInfo(taskIDs []string) []TaskWorktreeInfo {
	result := make([]TaskWorktreeInfo, 0, len(taskIDs))

	for _, taskID := range taskIDs {
		taskInfo := g.finder.GetTaskInfo(taskID)
		inst := g.finder.FindInstanceByTaskID(taskID)

		if inst == nil {
			continue
		}

		info := TaskWorktreeInfo{
			TaskID:       taskID,
			WorktreePath: inst.WorktreePath,
			Branch:       inst.Branch,
		}

		if taskInfo != nil {
			info.TaskTitle = taskInfo.Title
		}

		result = append(result, info)
	}

	return result
}

// FormatTaskListForPrompt formats a list of completed tasks for inclusion in a prompt.
// It includes commit counts and warnings for tasks without commits.
func (g *Gatherer) FormatTaskListForPrompt(completedTaskIDs []string, commitCounts map[string]int) string {
	var sb strings.Builder

	for _, taskID := range completedTaskIDs {
		taskInfo := g.finder.GetTaskInfo(taskID)
		title := taskID
		if taskInfo != nil {
			title = taskInfo.Title
		}

		commitCount := 0
		if count, ok := commitCounts[taskID]; ok {
			commitCount = count
		}

		if commitCount > 0 {
			sb.WriteString(fmt.Sprintf("- [%s] %s (%d commits)\n", taskID, title, commitCount))
		} else {
			sb.WriteString(fmt.Sprintf("- [%s] %s (NO COMMITS - verify this task)\n", taskID, title))
		}
	}

	return sb.String()
}

// FormatWorktreeInfoForPrompt formats worktree information for inclusion in a prompt.
func (g *Gatherer) FormatWorktreeInfoForPrompt(worktrees []TaskWorktreeInfo, taskContext *AggregatedTaskContext) string {
	var sb strings.Builder

	for _, wt := range worktrees {
		sb.WriteString(fmt.Sprintf("### %s: %s\n", wt.TaskID, wt.TaskTitle))
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", wt.Branch))
		sb.WriteString(fmt.Sprintf("- Worktree: `%s`\n", wt.WorktreePath))

		if taskContext != nil {
			if summary, ok := taskContext.TaskSummaries[wt.TaskID]; ok && summary != "" {
				sb.WriteString(fmt.Sprintf("- Summary: %s\n", summary))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatPreviousGroupContext formats context from a previous group's consolidator.
func (g *Gatherer) FormatPreviousGroupContext(groupCtx *GroupContext) string {
	if groupCtx == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Context from Previous Group's Consolidator\n\n")

	if groupCtx.Notes != "" {
		sb.WriteString(fmt.Sprintf("**Notes**: %s\n\n", groupCtx.Notes))
	}

	if len(groupCtx.IssuesForNext) > 0 {
		sb.WriteString("**Issues/Warnings to Address**:\n")
		for _, issue := range groupCtx.IssuesForNext {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatSynthesisContextForPrompt formats synthesis completion data for prompts.
func (g *Gatherer) FormatSynthesisContextForPrompt(synth *SynthesisCompletionData) string {
	if synth == nil {
		return "No synthesis context available (synthesis may have used legacy mode)\n"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Status: %s\n", synth.Status))

	if synth.IntegrationNotes != "" {
		sb.WriteString(fmt.Sprintf("Integration Notes: %s\n", synth.IntegrationNotes))
	}

	if len(synth.Recommendations) > 0 {
		sb.WriteString("Recommendations:\n")
		for _, rec := range synth.Recommendations {
			sb.WriteString(fmt.Sprintf("- %s\n", rec))
		}
	}

	if len(synth.IssuesFound) > 0 {
		sb.WriteString(fmt.Sprintf("Issues Found: %d\n", len(synth.IssuesFound)))
		for _, issue := range synth.IssuesFound {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", issue.Severity, issue.Description))
		}
	}

	return sb.String()
}

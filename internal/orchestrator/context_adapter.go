package orchestrator

import (
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator/context"
)

// sessionInstanceFinder adapts Session and UltraPlanSession to the context.InstanceFinder interface.
// This adapter bridges the orchestrator's session types with the context package's interface.
type sessionInstanceFinder struct {
	baseSession *Session
	ultraPlan   *UltraPlanSession
}

// newSessionInstanceFinder creates a new instance finder that searches across sessions.
func newSessionInstanceFinder(baseSession *Session, ultraPlan *UltraPlanSession) *sessionInstanceFinder {
	return &sessionInstanceFinder{
		baseSession: baseSession,
		ultraPlan:   ultraPlan,
	}
}

// FindInstanceByTaskID returns instance info for a given task ID.
// It searches through the base session's instances to find one matching the task.
func (f *sessionInstanceFinder) FindInstanceByTaskID(taskID string) *context.InstanceInfo {
	if f.baseSession == nil {
		return nil
	}

	for _, inst := range f.baseSession.Instances {
		if strings.Contains(inst.Task, taskID) || f.matchTaskToInstance(taskID, inst) {
			return &context.InstanceInfo{
				ID:           inst.ID,
				WorktreePath: inst.WorktreePath,
				Branch:       inst.Branch,
				Task:         inst.Task,
			}
		}
	}
	return nil
}

// GetTaskInfo returns task information by ID.
func (f *sessionInstanceFinder) GetTaskInfo(taskID string) *context.TaskInfo {
	if f.ultraPlan == nil {
		return nil
	}

	task := f.ultraPlan.GetTask(taskID)
	if task == nil {
		return nil
	}

	return &context.TaskInfo{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		DependsOn:   task.DependsOn,
	}
}

// matchTaskToInstance attempts to match a task ID to an instance by comparing
// branch names with slugified task titles.
func (f *sessionInstanceFinder) matchTaskToInstance(taskID string, inst *Instance) bool {
	if f.ultraPlan == nil {
		return false
	}

	task := f.ultraPlan.GetTask(taskID)
	if task == nil {
		return false
	}

	// Check if the instance branch contains a slug version of the task title
	titleSlug := slugify(task.Title)
	return strings.Contains(inst.Branch, titleSlug)
}

// fileCompletionReader implements context.CompletionFileReader using the
// existing ParseXxxCompletionFile functions from the orchestrator package.
type fileCompletionReader struct{}

// newFileCompletionReader creates a new completion file reader.
func newFileCompletionReader() *fileCompletionReader {
	return &fileCompletionReader{}
}

// ReadTaskCompletion reads and parses a task completion file from a worktree.
func (r *fileCompletionReader) ReadTaskCompletion(worktreePath string) *context.TaskCompletionData {
	completion, err := ParseTaskCompletionFile(worktreePath)
	if err != nil {
		return nil
	}

	return &context.TaskCompletionData{
		TaskID:        completion.TaskID,
		Status:        completion.Status,
		Summary:       completion.Summary,
		FilesModified: completion.FilesModified,
		Notes:         completion.Notes.String(),
		Issues:        completion.Issues,
		Suggestions:   completion.Suggestions,
		Dependencies:  completion.Dependencies,
	}
}

// ReadSynthesisCompletion reads and parses a synthesis completion file.
func (r *fileCompletionReader) ReadSynthesisCompletion(worktreePath string) *context.SynthesisCompletionData {
	completion, err := ParseSynthesisCompletionFile(worktreePath)
	if err != nil {
		return nil
	}

	// Convert issues
	issues := make([]context.SynthesisIssue, len(completion.IssuesFound))
	for i, issue := range completion.IssuesFound {
		issues[i] = context.SynthesisIssue{
			TaskID:      issue.TaskID,
			Description: issue.Description,
			Severity:    issue.Severity,
			Suggestion:  issue.Suggestion,
		}
	}

	return &context.SynthesisCompletionData{
		Status:           completion.Status,
		RevisionRound:    completion.RevisionRound,
		IssuesFound:      issues,
		TasksAffected:    completion.TasksAffected,
		IntegrationNotes: completion.IntegrationNotes,
		Recommendations:  completion.Recommendations,
	}
}

// ReadGroupConsolidationCompletion reads and parses a group consolidation completion file.
func (r *fileCompletionReader) ReadGroupConsolidationCompletion(worktreePath string) *context.GroupConsolidationData {
	completion, err := ParseGroupConsolidationCompletionFile(worktreePath)
	if err != nil {
		return nil
	}

	return &context.GroupConsolidationData{
		GroupIndex:         completion.GroupIndex,
		Status:             completion.Status,
		BranchName:         completion.BranchName,
		TasksConsolidated:  completion.TasksConsolidated,
		Notes:              completion.Notes,
		IssuesForNextGroup: completion.IssuesForNextGroup,
		VerificationOK:     completion.Verification.OverallSuccess,
	}
}

// NewContextGatherer creates a new context.Gatherer configured to work with
// the given sessions. This is the main entry point for creating a gatherer
// that's connected to the orchestrator's data.
func NewContextGatherer(baseSession *Session, ultraPlan *UltraPlanSession) *context.Gatherer {
	finder := newSessionInstanceFinder(baseSession, ultraPlan)
	reader := newFileCompletionReader()
	return context.NewGatherer(finder, reader)
}

// TaskWorktreeInfoFromContext converts context.TaskWorktreeInfo to the orchestrator's TaskWorktreeInfo.
// This is useful when the context package types need to be used with orchestrator functions
// that expect the orchestrator's types.
func TaskWorktreeInfoFromContext(ctxInfo []context.TaskWorktreeInfo) []TaskWorktreeInfo {
	result := make([]TaskWorktreeInfo, len(ctxInfo))
	for i, info := range ctxInfo {
		result[i] = TaskWorktreeInfo{
			TaskID:       info.TaskID,
			TaskTitle:    info.TaskTitle,
			WorktreePath: info.WorktreePath,
			Branch:       info.Branch,
		}
	}
	return result
}

// TaskWorktreeInfoToContext converts orchestrator's TaskWorktreeInfo to context.TaskWorktreeInfo.
func TaskWorktreeInfoToContext(orchInfo []TaskWorktreeInfo) []context.TaskWorktreeInfo {
	result := make([]context.TaskWorktreeInfo, len(orchInfo))
	for i, info := range orchInfo {
		result[i] = context.TaskWorktreeInfo{
			TaskID:       info.TaskID,
			TaskTitle:    info.TaskTitle,
			WorktreePath: info.WorktreePath,
			Branch:       info.Branch,
		}
	}
	return result
}

// AggregatedTaskContextFromContext converts context.AggregatedTaskContext to AggregatedTaskContext.
func AggregatedTaskContextFromContext(ctxAgg *context.AggregatedTaskContext) *AggregatedTaskContext {
	if ctxAgg == nil {
		return nil
	}
	return &AggregatedTaskContext{
		TaskSummaries:  ctxAgg.TaskSummaries,
		AllIssues:      ctxAgg.AllIssues,
		AllSuggestions: ctxAgg.AllSuggestions,
		Dependencies:   ctxAgg.Dependencies,
		Notes:          ctxAgg.Notes,
	}
}

// Package types provides shared type definitions for the orchestrator and its subpackages.
// These types are extracted here to avoid circular imports between orchestrator, phase,
// verify, and context packages.
package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TaskCompletionFileName is the sentinel file that tasks write when complete.
const TaskCompletionFileName = ".claudio-task-complete.json"

// FlexibleString can unmarshal from either a string or a string array.
// When unmarshaling a string array, the elements are joined with newlines.
// This handles both simple string notes and structured array notes.
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler for FlexibleString.
// It handles both string and string array inputs.
func (f *FlexibleString) UnmarshalJSON(data []byte) error {
	// Try string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleString(s)
		return nil
	}

	// Try string array
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = FlexibleString(strings.Join(arr, "\n"))
		return nil
	}

	// If both fail, treat as empty
	*f = ""
	return nil
}

// String returns the underlying string value.
func (f FlexibleString) String() string {
	return string(f)
}

// TaskCompletionFile represents the completion report written by a task.
// This file serves as both a sentinel (existence = task done) and a context carrier.
type TaskCompletionFile struct {
	TaskID        string   `json:"task_id"`
	Status        string   `json:"status"` // "complete", "blocked", or "failed"
	Summary       string   `json:"summary"`
	FilesModified []string `json:"files_modified"`
	// Rich context for consolidation
	Notes        FlexibleString `json:"notes,omitempty"`        // Free-form implementation notes (accepts string or array)
	Issues       []string       `json:"issues,omitempty"`       // Blocking issues or concerns found
	Suggestions  []string       `json:"suggestions,omitempty"`  // Integration suggestions for other tasks
	Dependencies []string       `json:"dependencies,omitempty"` // Runtime dependencies added
}

// AggregatedTaskContext holds the aggregated context from all task completion files.
// This is used to provide rich context for consolidation prompts and PR descriptions.
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

// ConsolidationTaskWorktreeInfo holds information about a task's worktree for consolidation.
type ConsolidationTaskWorktreeInfo struct {
	TaskID       string // Task ID
	TaskTitle    string // Human-readable task title
	WorktreePath string // Path to the git worktree
	Branch       string // Branch name for this task
}

// ConflictResolution describes how a merge conflict was resolved.
type ConflictResolution struct {
	File       string `json:"file"`       // File that had the conflict
	Resolution string `json:"resolution"` // Description of how it was resolved
}

// VerificationResult holds the results of build/lint/test verification.
// The consolidator determines appropriate commands based on project type.
type VerificationResult struct {
	ProjectType    string             `json:"project_type,omitempty"` // Detected: "go", "node", "ios", "python", etc.
	CommandsRun    []VerificationStep `json:"commands_run"`
	OverallSuccess bool               `json:"overall_success"`
	Summary        string             `json:"summary,omitempty"` // Brief summary of verification outcome
}

// VerificationStep represents a single verification command and its result.
type VerificationStep struct {
	Name    string `json:"name"`    // e.g., "build", "lint", "test"
	Command string `json:"command"` // Actual command run
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"` // Truncated output on failure
}

// GroupConsolidationCompletionFileName is the sentinel file that per-group
// consolidators write when complete.
const GroupConsolidationCompletionFileName = ".claudio-group-consolidation-complete.json"

// GroupConsolidationCompletionFile is written by the per-group consolidator session
// when it finishes consolidating a group's task branches.
type GroupConsolidationCompletionFile struct {
	GroupIndex         int                    `json:"group_index"`
	Status             string                 `json:"status"` // "complete", "failed"
	BranchName         string                 `json:"branch_name"`
	TasksConsolidated  []string               `json:"tasks_consolidated"`
	ConflictsResolved  []ConflictResolution   `json:"conflicts_resolved,omitempty"`
	Verification       VerificationResult     `json:"verification"`
	AggregatedContext  *AggregatedTaskContext `json:"aggregated_context,omitempty"`
	Notes              string                 `json:"notes,omitempty"`                 // Consolidator's observations
	IssuesForNextGroup []string               `json:"issues_for_next_group,omitempty"` // Warnings/concerns to pass forward
}

// GetNotes returns the consolidator's observations about the consolidated code.
// This method enables GroupConsolidationCompletionFile to satisfy prompt.GroupContextLike interface.
func (g *GroupConsolidationCompletionFile) GetNotes() string { return g.Notes }

// GetIssuesForNextGroup returns warnings or concerns to pass to the next group.
// This method enables GroupConsolidationCompletionFile to satisfy prompt.GroupContextLike interface.
func (g *GroupConsolidationCompletionFile) GetIssuesForNextGroup() []string {
	return g.IssuesForNextGroup
}

// IsVerificationSuccess returns true if the verification (build/lint/tests) passed.
// This method enables GroupConsolidationCompletionFile to satisfy prompt.GroupContextLike interface.
func (g *GroupConsolidationCompletionFile) IsVerificationSuccess() bool {
	return g.Verification.OverallSuccess
}

// TaskCompletionFilePath returns the full path to the task completion file for a given worktree.
func TaskCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, TaskCompletionFileName)
}

// ParseTaskCompletionFile reads and parses a task completion file.
func ParseTaskCompletionFile(worktreePath string) (*TaskCompletionFile, error) {
	completionPath := TaskCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion TaskCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse task completion JSON: %w", err)
	}

	return &completion, nil
}

// GroupConsolidationCompletionFilePath returns the full path to the group consolidation completion file.
func GroupConsolidationCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, GroupConsolidationCompletionFileName)
}

// ParseGroupConsolidationCompletionFile reads and parses a group consolidation completion file.
func ParseGroupConsolidationCompletionFile(worktreePath string) (*GroupConsolidationCompletionFile, error) {
	completionPath := GroupConsolidationCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion GroupConsolidationCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse group consolidation completion JSON: %w", err)
	}

	return &completion, nil
}

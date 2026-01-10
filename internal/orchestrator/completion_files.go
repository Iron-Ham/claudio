// Package orchestrator provides coordination and lifecycle management for Claude instances.
package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Completion file name constants.
// These are sentinel files written by Claude instances to signal phase completion.
const (
	// TaskCompletionFileName is the sentinel file that tasks write when complete.
	TaskCompletionFileName = ".claudio-task-complete.json"

	// SynthesisCompletionFileName is the sentinel file that synthesis writes when complete.
	SynthesisCompletionFileName = ".claudio-synthesis-complete.json"

	// RevisionCompletionFileName is the sentinel file that revision tasks write when complete.
	RevisionCompletionFileName = ".claudio-revision-complete.json"

	// ConsolidationCompletionFileName is the sentinel file that consolidation writes when complete.
	ConsolidationCompletionFileName = ".claudio-consolidation-complete.json"

	// GroupConsolidationCompletionFileName is the sentinel file that per-group consolidators write when complete.
	GroupConsolidationCompletionFileName = ".claudio-group-consolidation-complete.json"
)

// =============================================================================
// FlexibleString - Helper type for JSON unmarshaling
// =============================================================================

// FlexibleString is a type that can unmarshal from either a JSON string or an array of strings.
// When unmarshaling an array, the strings are joined with newlines.
// This provides flexibility for Claude instances that may write notes as either format.
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler for FlexibleString.
func (f *FlexibleString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleString(s)
		return nil
	}

	// Try to unmarshal as an array of strings
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

// =============================================================================
// Task Completion
// =============================================================================

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

// =============================================================================
// Synthesis Completion
// =============================================================================

// SynthesisCompletionFile represents the completion report from the synthesis phase.
type SynthesisCompletionFile struct {
	Status           string          `json:"status"`            // "complete", "needs_revision"
	RevisionRound    int             `json:"revision_round"`    // Current round (0 for first synthesis)
	IssuesFound      []RevisionIssue `json:"issues_found"`      // All issues identified
	TasksAffected    []string        `json:"tasks_affected"`    // Task IDs needing revision
	IntegrationNotes string          `json:"integration_notes"` // Free-form observations about integration
	Recommendations  []string        `json:"recommendations"`   // Suggestions for consolidation phase
}

// SynthesisCompletionFilePath returns the full path to the synthesis completion file.
func SynthesisCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, SynthesisCompletionFileName)
}

// ParseSynthesisCompletionFile reads and parses a synthesis completion file.
func ParseSynthesisCompletionFile(worktreePath string) (*SynthesisCompletionFile, error) {
	completionPath := SynthesisCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion SynthesisCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse synthesis completion JSON: %w", err)
	}

	return &completion, nil
}

// =============================================================================
// Revision Completion
// =============================================================================

// RevisionCompletionFile represents the completion report from a revision task.
type RevisionCompletionFile struct {
	TaskID          string   `json:"task_id"`
	RevisionRound   int      `json:"revision_round"`
	IssuesAddressed []string `json:"issues_addressed"` // Issue descriptions that were fixed
	Summary         string   `json:"summary"`          // What was changed
	FilesModified   []string `json:"files_modified"`
	RemainingIssues []string `json:"remaining_issues"` // Issues that couldn't be fixed
}

// RevisionCompletionFilePath returns the full path to the revision completion file.
func RevisionCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, RevisionCompletionFileName)
}

// ParseRevisionCompletionFile reads and parses a revision completion file.
func ParseRevisionCompletionFile(worktreePath string) (*RevisionCompletionFile, error) {
	completionPath := RevisionCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion RevisionCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse revision completion JSON: %w", err)
	}

	return &completion, nil
}

// =============================================================================
// Consolidation Completion
// =============================================================================

// ConsolidationCompletionFile represents the completion report from consolidation.
type ConsolidationCompletionFile struct {
	Status           string                   `json:"status"` // "complete", "partial", "failed"
	Mode             string                   `json:"mode"`   // "stacked" or "single"
	GroupResults     []GroupConsolidationInfo `json:"group_results"`
	PRsCreated       []PRInfo                 `json:"prs_created"`
	SynthesisContext *SynthesisCompletionFile `json:"synthesis_context,omitempty"`
	TotalCommits     int                      `json:"total_commits"`
	FilesChanged     []string                 `json:"files_changed"`
}

// GroupConsolidationInfo holds info about a consolidated group.
type GroupConsolidationInfo struct {
	GroupIndex    int      `json:"group_index"`
	BranchName    string   `json:"branch_name"`
	TasksIncluded []string `json:"tasks_included"`
	CommitCount   int      `json:"commit_count"`
	Success       bool     `json:"success"`
}

// PRInfo holds information about a created PR.
type PRInfo struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	GroupIndex int    `json:"group_index"`
}

// ConsolidationCompletionFilePath returns the full path to the consolidation completion file.
func ConsolidationCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, ConsolidationCompletionFileName)
}

// ParseConsolidationCompletionFile reads and parses a consolidation completion file.
func ParseConsolidationCompletionFile(worktreePath string) (*ConsolidationCompletionFile, error) {
	completionPath := ConsolidationCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion ConsolidationCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse consolidation completion JSON: %w", err)
	}

	return &completion, nil
}

// =============================================================================
// Group Consolidation Completion
// =============================================================================

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
	Name    string `json:"name"`             // e.g., "build", "lint", "test"
	Command string `json:"command"`          // Actual command run
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"` // Truncated output on failure
}

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

// =============================================================================
// Aggregated Task Context
// =============================================================================

// AggregatedTaskContext holds the aggregated context from all task completion files.
type AggregatedTaskContext struct {
	TaskSummaries  map[string]string `json:"task_summaries,omitempty"`  // taskID -> summary
	AllIssues      []string          `json:"all_issues,omitempty"`      // All issues from all tasks
	AllSuggestions []string          `json:"all_suggestions,omitempty"` // All suggestions from all tasks
	Dependencies   []string          `json:"dependencies,omitempty"`    // Deduplicated list of new dependencies
	Notes          []string          `json:"notes,omitempty"`           // Implementation notes from all tasks
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

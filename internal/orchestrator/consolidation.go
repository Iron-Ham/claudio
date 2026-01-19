package orchestrator

import (
	"time"
)

// ConsolidationMode defines how work is consolidated after ultraplan execution
type ConsolidationMode string

const (
	// ModeStackedPRs creates one PR per execution group, stacked on each other
	ModeStackedPRs ConsolidationMode = "stacked"
	// ModeSinglePR consolidates all work into a single PR
	ModeSinglePR ConsolidationMode = "single"
)

// ConsolidationPhase represents sub-phases within consolidation
type ConsolidationPhase string

const (
	ConsolidationIdle             ConsolidationPhase = "idle"
	ConsolidationDetecting        ConsolidationPhase = "detecting_conflicts"
	ConsolidationCreatingBranches ConsolidationPhase = "creating_branches"
	ConsolidationMergingTasks     ConsolidationPhase = "merging_tasks"
	ConsolidationPushing          ConsolidationPhase = "pushing"
	ConsolidationCreatingPRs      ConsolidationPhase = "creating_prs"
	ConsolidationPaused           ConsolidationPhase = "paused"
	ConsolidationComplete         ConsolidationPhase = "complete"
	ConsolidationFailed           ConsolidationPhase = "failed"
)

// ConsolidatorState tracks the progress of consolidation
type ConsolidatorState struct {
	Phase            ConsolidationPhase `json:"phase"`
	CurrentGroup     int                `json:"current_group"`
	TotalGroups      int                `json:"total_groups"`
	CurrentTask      string             `json:"current_task,omitempty"`
	GroupBranches    []string           `json:"group_branches"`
	PRUrls           []string           `json:"pr_urls"`
	ConflictFiles    []string           `json:"conflict_files,omitempty"`
	ConflictTaskID   string             `json:"conflict_task_id,omitempty"`
	ConflictWorktree string             `json:"conflict_worktree,omitempty"`
	Error            string             `json:"error,omitempty"`
	StartedAt        *time.Time         `json:"started_at,omitempty"`
	CompletedAt      *time.Time         `json:"completed_at,omitempty"`
}

// HasConflict returns true if consolidation is paused due to a conflict.
func (s *ConsolidatorState) HasConflict() bool {
	return s.Phase == ConsolidationPaused && len(s.ConflictFiles) > 0
}

// GroupConsolidationResult holds the result of consolidating one group
type GroupConsolidationResult struct {
	GroupIndex   int      `json:"group_index"`
	TaskIDs      []string `json:"task_ids"`
	BranchName   string   `json:"branch_name"`
	CommitCount  int      `json:"commit_count"`
	FilesChanged []string `json:"files_changed"`
	PRUrl        string   `json:"pr_url,omitempty"`
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
}

// ConsolidationConfig holds configuration for branch consolidation
type ConsolidationConfig struct {
	Mode           ConsolidationMode
	BranchPrefix   string
	CreateDraftPRs bool
	PRLabels       []string
}

// ConsolidationEventType represents events during consolidation
type ConsolidationEventType string

const (
	EventConsolidationStarted       ConsolidationEventType = "consolidation_started"
	EventConsolidationGroupStarted  ConsolidationEventType = "consolidation_group_started"
	EventConsolidationTaskMerging   ConsolidationEventType = "consolidation_task_merging"
	EventConsolidationTaskMerged    ConsolidationEventType = "consolidation_task_merged"
	EventConsolidationGroupComplete ConsolidationEventType = "consolidation_group_complete"
	EventConsolidationPRCreating    ConsolidationEventType = "consolidation_pr_creating"
	EventConsolidationPRCreated     ConsolidationEventType = "consolidation_pr_created"
	EventConsolidationConflict      ConsolidationEventType = "consolidation_conflict"
	EventConsolidationComplete      ConsolidationEventType = "consolidation_complete"
	EventConsolidationFailed        ConsolidationEventType = "consolidation_failed"
)

// ConsolidationEvent represents an event during consolidation
type ConsolidationEvent struct {
	Type      ConsolidationEventType `json:"type"`
	GroupIdx  int                    `json:"group_idx,omitempty"`
	TaskID    string                 `json:"task_id,omitempty"`
	Message   string                 `json:"message,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// PRContent holds PR title and body
type PRContent struct {
	Title string
	Body  string
}

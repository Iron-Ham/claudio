// Package completion provides interfaces and types for detecting and handling
// completion of orchestrated Claude instances. This abstraction separates
// completion detection logic from the coordinator, enabling testability and
// supporting different detection strategies.
package completion

import (
	"context"
	"time"
)

// FileType represents the different types of completion files that Claude
// instances write to signal task completion. Each phase of the orchestration
// pipeline uses a distinct file type.
type FileType int

const (
	// FileTypeTask signals completion of a regular task execution.
	// Written to .claudio-task-complete.json
	FileTypeTask FileType = iota

	// FileTypeSynthesis signals completion of the synthesis phase.
	// Written to .claudio-synthesis-complete.json
	FileTypeSynthesis

	// FileTypeRevision signals completion of a revision task.
	// Written to .claudio-revision-complete.json
	FileTypeRevision

	// FileTypeConsolidation signals completion of the final consolidation.
	// Written to .claudio-consolidation-complete.json
	FileTypeConsolidation

	// FileTypeGroupConsolidation signals completion of per-group consolidation.
	// Written to .claudio-group-consolidation-complete.json
	FileTypeGroupConsolidation

	// FileTypePlan signals completion of the planning phase.
	// Written to .claudio-plan.json
	FileTypePlan
)

// String returns the human-readable name for the file type.
func (ft FileType) String() string {
	switch ft {
	case FileTypeTask:
		return "task"
	case FileTypeSynthesis:
		return "synthesis"
	case FileTypeRevision:
		return "revision"
	case FileTypeConsolidation:
		return "consolidation"
	case FileTypeGroupConsolidation:
		return "group_consolidation"
	case FileTypePlan:
		return "plan"
	default:
		return "unknown"
	}
}

// FileName returns the sentinel file name for this completion type.
func (ft FileType) FileName() string {
	switch ft {
	case FileTypeTask:
		return ".claudio-task-complete.json"
	case FileTypeSynthesis:
		return ".claudio-synthesis-complete.json"
	case FileTypeRevision:
		return ".claudio-revision-complete.json"
	case FileTypeConsolidation:
		return ".claudio-consolidation-complete.json"
	case FileTypeGroupConsolidation:
		return ".claudio-group-consolidation-complete.json"
	case FileTypePlan:
		return ".claudio-plan.json"
	default:
		return ""
	}
}

// Status represents the outcome status of a completion.
type Status string

const (
	// StatusComplete indicates successful completion.
	StatusComplete Status = "complete"

	// StatusBlocked indicates the task is blocked and cannot proceed.
	StatusBlocked Status = "blocked"

	// StatusFailed indicates the task failed.
	StatusFailed Status = "failed"

	// StatusNeedsRevision indicates synthesis found issues requiring revision.
	StatusNeedsRevision Status = "needs_revision"

	// StatusPartial indicates partial completion (some work done but not all).
	StatusPartial Status = "partial"
)

// Info contains standardized completion information extracted from any
// completion file type. This provides a unified view for the coordinator
// while preserving type-specific details in the Details field.
type Info struct {
	// Type identifies what kind of completion this represents.
	Type FileType

	// Success indicates whether the completion represents successful work.
	// This is derived from Status but provided for convenience.
	Success bool

	// Status is the raw status string from the completion file.
	Status Status

	// OutputPath is the path to the output or artifact produced.
	// For tasks, this is typically the worktree path.
	// For consolidation, this might be a branch name.
	OutputPath string

	// Issues contains any problems found during execution.
	// For synthesis, these are RevisionIssue descriptions.
	// For tasks, these are blocking issues or concerns.
	Issues []string

	// Timestamp records when the completion was detected.
	Timestamp time.Time

	// TaskID identifies the task (for task/revision completions).
	TaskID string

	// Summary provides a brief description of what was accomplished.
	Summary string

	// FilesModified lists files that were changed.
	FilesModified []string

	// Details holds the type-specific completion data.
	// Cast to the appropriate concrete type based on Info.Type:
	//   - FileTypeTask: *TaskDetails
	//   - FileTypeSynthesis: *SynthesisDetails
	//   - FileTypeRevision: *RevisionDetails
	//   - FileTypeConsolidation: *ConsolidationDetails
	//   - FileTypeGroupConsolidation: *GroupConsolidationDetails
	Details any
}

// TaskDetails contains task-specific completion information.
type TaskDetails struct {
	Notes        string   // Free-form implementation notes
	Suggestions  []string // Integration suggestions for other tasks
	Dependencies []string // Runtime dependencies added
}

// SynthesisDetails contains synthesis-specific completion information.
type SynthesisDetails struct {
	RevisionRound    int      // Current round (0 for first synthesis)
	TasksAffected    []string // Task IDs needing revision
	IntegrationNotes string   // Observations about integration
	Recommendations  []string // Suggestions for consolidation phase
}

// RevisionDetails contains revision-specific completion information.
type RevisionDetails struct {
	RevisionRound   int      // Which revision round this was
	IssuesAddressed []string // Issue descriptions that were fixed
	RemainingIssues []string // Issues that couldn't be fixed
}

// ConsolidationDetails contains final consolidation information.
type ConsolidationDetails struct {
	Mode         string              // "stacked" or "single"
	GroupResults []GroupResult       // Results from each group consolidation
	PRsCreated   []PRInfo            // Pull requests that were created
	TotalCommits int                 // Total number of commits consolidated
	FilesChanged []string            // All files changed across consolidation
}

// GroupConsolidationDetails contains per-group consolidation information.
type GroupConsolidationDetails struct {
	GroupIndex        int                  // Which group this consolidation was for
	BranchName        string               // Name of the consolidated branch
	TasksConsolidated []string             // Task IDs that were consolidated
	ConflictsResolved []ConflictResolution // How merge conflicts were handled
	Verification      VerificationResult   // Build/test verification results
	Notes             string               // Consolidator's observations
	IssuesForNextGroup []string            // Warnings to pass to next group
}

// GroupResult summarizes a single group's consolidation outcome.
type GroupResult struct {
	GroupIndex    int      // Index of the group
	BranchName    string   // Resulting branch name
	TasksIncluded []string // Task IDs included in this group
	CommitCount   int      // Number of commits in the consolidated branch
	Success       bool     // Whether consolidation succeeded
}

// PRInfo describes a pull request that was created.
type PRInfo struct {
	URL        string // Full URL to the PR
	Title      string // PR title
	GroupIndex int    // Which group this PR is for (-1 for single PR mode)
}

// ConflictResolution describes how a merge conflict was resolved.
type ConflictResolution struct {
	File       string // File that had the conflict
	Resolution string // Description of how it was resolved
}

// VerificationResult holds build/lint/test verification outcomes.
type VerificationResult struct {
	ProjectType    string             // Detected: "go", "node", "ios", "python", etc.
	CommandsRun    []VerificationStep // Individual verification steps
	OverallSuccess bool               // Whether all verification passed
	Summary        string             // Brief summary of verification outcome
}

// VerificationStep represents a single verification command and its result.
type VerificationStep struct {
	Name    string // e.g., "build", "lint", "test"
	Command string // Actual command that was run
	Success bool   // Whether the command succeeded
	Output  string // Truncated output (especially on failure)
}

// Detector is the interface for detecting completion of Claude instances.
// Implementations may use file-based detection, status polling, or other
// mechanisms to determine when work is complete.
type Detector interface {
	// CheckCompletion checks if the instance has completed and returns
	// completion information if available. Returns (false, nil, nil) if
	// not yet complete, (true, info, nil) on completion, or (false, nil, err)
	// on error.
	CheckCompletion(ctx context.Context, instanceID string) (bool, *Info, error)

	// WatchForCompletion starts watching an instance for completion and
	// calls the callback when completion is detected. The watch continues
	// until the context is cancelled or completion is detected.
	// The callback receives the completion info when triggered.
	WatchForCompletion(ctx context.Context, instanceID string, callback func(*Info))

	// ParseCompletionFile parses a completion file at the given path and
	// returns the extracted completion information. The fileType parameter
	// specifies what type of completion file to expect.
	ParseCompletionFile(path string, fileType FileType) (*Info, error)

	// GetFilePath returns the expected completion file path for a given
	// worktree and completion type.
	GetFilePath(worktreePath string, fileType FileType) string
}

// Config contains configuration options for completion detection.
type Config struct {
	// PollInterval specifies how frequently to check for completion files
	// when using polling-based detection. Defaults to 500ms.
	PollInterval time.Duration

	// RequireCommits when true requires task completions to have associated
	// git commits. Tasks that complete without commits will be marked as
	// needing retry. Defaults to true.
	RequireCommits bool

	// MaxRetries specifies how many times to retry a task that completes
	// without producing commits (when RequireCommits is true). Defaults to 2.
	MaxRetries int

	// VerifyTmuxExit when true verifies that the tmux session has actually
	// exited before considering status-based completion as final.
	// This prevents false positives from UI state detection.
	VerifyTmuxExit bool

	// AllowedFileTypes restricts which completion file types this detector
	// will recognize. If empty, all file types are allowed.
	AllowedFileTypes []FileType

	// WorktreeBasePath is the base path for locating worktrees.
	// Used by GetFilePath to construct full paths.
	WorktreeBasePath string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		PollInterval:   500 * time.Millisecond,
		RequireCommits: true,
		MaxRetries:     2,
		VerifyTmuxExit: true,
	}
}

// Option is a functional option for configuring a Detector.
type Option func(*Config)

// WithPollInterval sets the polling interval for completion detection.
func WithPollInterval(d time.Duration) Option {
	return func(c *Config) {
		c.PollInterval = d
	}
}

// WithRequireCommits configures whether commits are required for task completion.
func WithRequireCommits(require bool) Option {
	return func(c *Config) {
		c.RequireCommits = require
	}
}

// WithMaxRetries sets the maximum retry count for tasks without commits.
func WithMaxRetries(n int) Option {
	return func(c *Config) {
		c.MaxRetries = n
	}
}

// WithVerifyTmuxExit configures tmux session exit verification.
func WithVerifyTmuxExit(verify bool) Option {
	return func(c *Config) {
		c.VerifyTmuxExit = verify
	}
}

// WithAllowedFileTypes restricts which completion types are recognized.
func WithAllowedFileTypes(types ...FileType) Option {
	return func(c *Config) {
		c.AllowedFileTypes = types
	}
}

// WithWorktreeBasePath sets the base path for worktree location.
func WithWorktreeBasePath(path string) Option {
	return func(c *Config) {
		c.WorktreeBasePath = path
	}
}

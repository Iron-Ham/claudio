// Package prompt provides interfaces and types for prompt generation in the orchestrator.
// It abstracts prompt building from the coordinator, enabling customizable templates
// and consistent prompt generation across different phases of ultra-plan execution.
package prompt

import "time"

// PromptBuilder defines the interface for generating prompts across all orchestration phases.
// Implementations handle the construction of prompts for task execution, plan management,
// synthesis review, revision requests, and consolidation operations.
type PromptBuilder interface {
	// BuildTaskPrompt generates a prompt for executing a specific task.
	// The prompt includes task details, plan context, and completion protocol.
	BuildTaskPrompt(task Task, ctx TaskContext) string

	// BuildPlanManagerPrompt generates a prompt for the plan manager to evaluate
	// and select from multiple candidate plans in multi-pass planning mode.
	BuildPlanManagerPrompt(plans []Plan, ctx PlanContext) string

	// BuildSynthesisPrompt generates a prompt for reviewing completed work,
	// identifying integration issues, and determining if revisions are needed.
	BuildSynthesisPrompt(results []TaskResult, ctx SynthesisContext) string

	// BuildRevisionPrompt generates a prompt for addressing specific issues
	// identified during synthesis review.
	BuildRevisionPrompt(task Task, issues []Issue, ctx RevisionContext) string

	// BuildConsolidationPrompt generates a prompt for consolidating task branches
	// into PRs, handling merge conflicts, and running verification.
	BuildConsolidationPrompt(groups []TaskGroup, ctx ConsolidationContext) string

	// BuildGroupConsolidatorPrompt generates a prompt for per-group incremental
	// consolidation, merging parallel task branches within a single execution group.
	BuildGroupConsolidatorPrompt(groupIndex int, tasks []Task, ctx GroupConsolidationContext) string

	// BuildPlanningPrompt generates the initial prompt for plan decomposition.
	// The strategy parameter allows for different planning approaches (e.g., multi-pass).
	BuildPlanningPrompt(objective string, ctx PlanningContext, strategy PlanningStrategy) string
}

// Task represents a planned task for prompt generation.
// This is a simplified view of PlannedTask focused on prompt requirements.
type Task struct {
	ID          string
	Title       string
	Description string
	Files       []string // Expected files to be modified
	DependsOn   []string // Task IDs this task depends on
	Priority    int
	Complexity  string // "low", "medium", "high"
}

// Plan represents a plan specification for prompt generation.
// This is a simplified view of PlanSpec focused on prompt requirements.
type Plan struct {
	ID             string
	Objective      string
	Summary        string
	Tasks          []Task
	ExecutionOrder [][]string // Groups of parallelizable task IDs
	Insights       []string
	Constraints    []string
	Strategy       string // Planning strategy that generated this plan
}

// TaskResult represents the outcome of a completed task.
type TaskResult struct {
	TaskID        string
	TaskTitle     string
	Status        string // "complete", "blocked", "failed"
	Summary       string
	FilesModified []string
	CommitCount   int
	Issues        []string
	Suggestions   []string
	Notes         string
}

// Issue represents a problem identified during synthesis that needs revision.
type Issue struct {
	TaskID      string // Task ID that needs revision (empty for cross-cutting issues)
	Description string
	Files       []string // Files affected by the issue
	Severity    string   // "critical", "major", "minor"
	Suggestion  string   // Suggested fix
}

// TaskGroup represents a group of tasks that can execute in parallel.
type TaskGroup struct {
	Index       int
	TaskIDs     []string
	Tasks       []Task
	BranchName  string // Consolidated branch name for this group
	PRUrl       string // PR URL if created
	CommitCount int
}

// TaskContext provides context for building task prompts.
type TaskContext struct {
	// Plan context
	PlanObjective string
	PlanSummary   string
	TotalTasks    int
	CurrentGroup  int
	TotalGroups   int

	// Task position information
	GroupIndex    int
	TasksInGroup  []string
	CompletedDeps []string // IDs of completed dependency tasks

	// Previous group context (for incremental consolidation)
	PreviousGroupContext *GroupContext

	// Worktree information
	WorktreePath string
	BranchName   string

	// Configuration
	Config TaskPromptConfig
}

// GroupContext provides context from a previous group's consolidation.
type GroupContext struct {
	GroupIndex         int
	ConsolidatorNotes  string
	IssuesForNextGroup []string
	VerificationStatus string // "passed", "failed", "skipped"
	BranchName         string
}

// PlanContext provides context for building plan manager prompts.
type PlanContext struct {
	Objective     string
	StrategyNames []string // Names of strategies used to generate candidate plans
	CurrentRound  int      // For iterative refinement
	MaxCandidates int
	Config        PlanManagerPromptConfig
}

// SynthesisContext provides context for synthesis prompt generation.
type SynthesisContext struct {
	Objective      string
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	RevisionRound  int // 0 for first synthesis, >0 for post-revision re-synthesis
	TaskWorktrees  []TaskWorktreeInfo
	Config         SynthesisPromptConfig
}

// TaskWorktreeInfo holds information about a task's worktree for synthesis/consolidation.
type TaskWorktreeInfo struct {
	TaskID       string
	TaskTitle    string
	WorktreePath string
	Branch       string
}

// RevisionContext provides context for revision prompt generation.
type RevisionContext struct {
	Objective     string
	PlanSummary   string
	RevisionRound int
	MaxRevisions  int
	TotalIssues   int
	IssuesForTask int
	WorktreePath  string
	Config        RevisionPromptConfig
}

// ConsolidationContext provides context for consolidation prompt generation.
type ConsolidationContext struct {
	Objective        string
	Mode             string // "stacked" or "single"
	BranchPrefix     string
	MainBranch       string
	CreateDraftPRs   bool
	PRLabels         []string
	TaskWorktrees    []TaskWorktreeInfo
	SynthesisContext *SynthesisCompletionContext
	Config           ConsolidationPromptConfig
}

// SynthesisCompletionContext contains synthesis results for consolidation context.
type SynthesisCompletionContext struct {
	Status           string
	IntegrationNotes string
	Recommendations  []string
}

// GroupConsolidationContext provides context for per-group consolidation.
type GroupConsolidationContext struct {
	Objective         string
	TotalGroups       int
	BranchPrefix      string
	MainBranch        string
	BaseBranch        string // Branch to base this group on (main or previous group branch)
	TaskWorktrees     []TaskWorktreeInfo
	AggregatedContext *AggregatedContext
	PreviousGroupCtx  *GroupContext
	Config            GroupConsolidationPromptConfig
}

// AggregatedContext holds aggregated information from task completion files.
type AggregatedContext struct {
	TaskSummaries  map[string]string // taskID -> summary
	AllIssues      []string
	AllSuggestions []string
	Dependencies   []string // Deduplicated list of new dependencies
	Notes          []string
}

// PlanningContext provides context for initial planning prompts.
type PlanningContext struct {
	WorktreePath   string
	MainBranch     string
	MultiPass      bool
	CandidateIndex int // For multi-pass: which candidate this is (0, 1, 2)
	Config         PlanningPromptConfig
}

// PlanningStrategy defines different approaches to plan generation.
type PlanningStrategy string

const (
	// StrategyBalanced uses a balanced approach to task decomposition.
	StrategyBalanced PlanningStrategy = "balanced"

	// StrategyMaxParallelism optimizes for maximum parallel execution.
	StrategyMaxParallelism PlanningStrategy = "maximize-parallelism"

	// StrategyMinComplexity optimizes for minimal task complexity.
	StrategyMinComplexity PlanningStrategy = "minimize-complexity"
)

// PromptTemplate represents a customizable prompt template.
type PromptTemplate struct {
	Name        string
	Description string
	Template    string // Go text/template format string
	Variables   []TemplateVariable
	Phase       PromptPhase
	Version     string
}

// TemplateVariable describes a variable used in a prompt template.
type TemplateVariable struct {
	Name        string
	Description string
	Type        string // "string", "[]string", "int", "bool", "object"
	Required    bool
	Default     any
}

// PromptPhase identifies which orchestration phase a prompt is for.
type PromptPhase string

const (
	PhasePlanning           PromptPhase = "planning"
	PhasePlanSelection      PromptPhase = "plan_selection"
	PhaseTaskExecution      PromptPhase = "task_execution"
	PhaseSynthesis          PromptPhase = "synthesis"
	PhaseRevision           PromptPhase = "revision"
	PhaseConsolidation      PromptPhase = "consolidation"
	PhaseGroupConsolidation PromptPhase = "group_consolidation"
)

// PromptConfig holds configuration options for prompt generation.
type PromptConfig struct {
	// Template customization
	CustomTemplates map[PromptPhase]*PromptTemplate

	// Output format preferences
	OutputFormat    OutputFormat
	IncludeExamples bool

	// Completion protocol configuration
	CompletionFilePrefix string // e.g., ".claudio"
	RequireJSONOutput    bool

	// Context injection settings
	MaxContextLength     int  // Maximum characters for injected context
	IncludePreviousGroup bool // Include previous group's consolidator notes

	// Per-phase configurations
	Planning           PlanningPromptConfig
	PlanManager        PlanManagerPromptConfig
	Task               TaskPromptConfig
	Synthesis          SynthesisPromptConfig
	Revision           RevisionPromptConfig
	Consolidation      ConsolidationPromptConfig
	GroupConsolidation GroupConsolidationPromptConfig
}

// OutputFormat specifies the expected output format for prompts.
type OutputFormat string

const (
	OutputFormatMarkdown OutputFormat = "markdown"
	OutputFormatJSON     OutputFormat = "json"
	OutputFormatXML      OutputFormat = "xml"
)

// PlanningPromptConfig holds planning-specific prompt configuration.
type PlanningPromptConfig struct {
	// Task decomposition preferences
	PreferSmallTasks bool
	MaxTasksPerGroup int
	TargetComplexity string // "low", "medium", "high"

	// File ownership settings
	EnforceFileOwnership bool // Discourage multiple tasks touching same files

	// Output requirements
	RequireInsights    bool
	RequireConstraints bool
}

// PlanManagerPromptConfig holds configuration for plan manager (multi-pass selection) prompts.
type PlanManagerPromptConfig struct {
	// Evaluation criteria
	EvaluateParallelism   bool
	EvaluateGranularity   bool
	EvaluateDependencies  bool
	EvaluateFileOwnership bool
	EvaluateCompleteness  bool

	// Decision options
	AllowMerge       bool // Allow merging best elements from multiple plans
	RequireScores    bool // Require numeric scores for each plan
	RequireReasoning bool // Require explanation for selection

	// Output format
	DecisionTagName string // XML tag name for decision (e.g., "plan_decision")
}

// TaskPromptConfig holds task-specific prompt configuration.
type TaskPromptConfig struct {
	// Context inclusion
	IncludePlanContext       bool
	IncludeExpectedFiles     bool
	IncludeDependencyContext bool

	// Completion requirements
	RequireCommits       bool
	CompletionFileName   string
	CompletionFileSchema string
}

// SynthesisPromptConfig holds synthesis-specific prompt configuration.
type SynthesisPromptConfig struct {
	// Review depth
	CheckIntegration  bool
	CheckConflicts    bool
	CheckCompleteness bool

	// Issue reporting
	RequireSeverity    bool
	RequireSuggestions bool

	// Thresholds
	MaxIssuesPerTask   int
	CompletionFileName string
}

// RevisionPromptConfig holds revision-specific prompt configuration.
type RevisionPromptConfig struct {
	// Scope control
	FocusOnSpecificIssues bool
	AllowNewChanges       bool

	// Completion requirements
	RequireIssueMapping bool // Map addressed issues to original issue list
	CompletionFileName  string
}

// ConsolidationPromptConfig holds consolidation-specific prompt configuration.
type ConsolidationPromptConfig struct {
	// Merge strategy
	PreferRebase        bool
	SquashCommits       bool
	PreserveCommitOrder bool

	// PR creation
	PRTitleTemplate     string
	PRBodyTemplate      string
	IncludeSynthesisCtx bool

	// Verification
	RunVerification     bool
	VerificationTimeout time.Duration

	CompletionFileName string
}

// GroupConsolidationPromptConfig holds per-group consolidation configuration.
type GroupConsolidationPromptConfig struct {
	// Context propagation
	PropagateIssues       bool
	PropagateNotes        bool
	AggregateDependencies bool

	// Verification settings
	RunVerification      bool
	VerificationCommands []string

	// Completion requirements
	CompletionFileName string
}

// DefaultPromptConfig returns the default configuration for prompt generation.
func DefaultPromptConfig() PromptConfig {
	return PromptConfig{
		CustomTemplates:      make(map[PromptPhase]*PromptTemplate),
		OutputFormat:         OutputFormatMarkdown,
		IncludeExamples:      false,
		CompletionFilePrefix: ".claudio",
		RequireJSONOutput:    true,
		MaxContextLength:     10000,
		IncludePreviousGroup: true,
		Planning: PlanningPromptConfig{
			PreferSmallTasks:     true,
			MaxTasksPerGroup:     5,
			TargetComplexity:     "low",
			EnforceFileOwnership: true,
			RequireInsights:      true,
			RequireConstraints:   true,
		},
		PlanManager: PlanManagerPromptConfig{
			EvaluateParallelism:   true,
			EvaluateGranularity:   true,
			EvaluateDependencies:  true,
			EvaluateFileOwnership: true,
			EvaluateCompleteness:  true,
			AllowMerge:            true,
			RequireScores:         true,
			RequireReasoning:      true,
			DecisionTagName:       "plan_decision",
		},
		Task: TaskPromptConfig{
			IncludePlanContext:       true,
			IncludeExpectedFiles:     true,
			IncludeDependencyContext: true,
			RequireCommits:           true,
			CompletionFileName:       ".claudio-task-complete.json",
		},
		Synthesis: SynthesisPromptConfig{
			CheckIntegration:   true,
			CheckConflicts:     true,
			CheckCompleteness:  true,
			RequireSeverity:    true,
			RequireSuggestions: true,
			MaxIssuesPerTask:   10,
			CompletionFileName: ".claudio-synthesis-complete.json",
		},
		Revision: RevisionPromptConfig{
			FocusOnSpecificIssues: true,
			AllowNewChanges:       false,
			RequireIssueMapping:   true,
			CompletionFileName:    ".claudio-revision-complete.json",
		},
		Consolidation: ConsolidationPromptConfig{
			PreferRebase:        false,
			SquashCommits:       false,
			PreserveCommitOrder: true,
			IncludeSynthesisCtx: true,
			RunVerification:     true,
			VerificationTimeout: 5 * time.Minute,
			CompletionFileName:  ".claudio-consolidation-complete.json",
		},
		GroupConsolidation: GroupConsolidationPromptConfig{
			PropagateIssues:       true,
			PropagateNotes:        true,
			AggregateDependencies: true,
			RunVerification:       true,
			CompletionFileName:    ".claudio-group-consolidation-complete.json",
		},
	}
}

// PromptResult represents the result of prompt generation.
type PromptResult struct {
	Prompt           string
	Phase            PromptPhase
	TemplateUsed     string
	VariablesApplied map[string]any
	Warnings         []string
}

// ValidationError represents an error in prompt configuration or context.
type ValidationError struct {
	Field   string
	Message string
}

// ContextValidator validates that required context is present for prompt generation.
type ContextValidator interface {
	// ValidateTaskContext ensures all required fields are present for task prompts.
	ValidateTaskContext(ctx TaskContext) []ValidationError

	// ValidatePlanContext ensures all required fields are present for plan prompts.
	ValidatePlanContext(ctx PlanContext) []ValidationError

	// ValidateSynthesisContext ensures all required fields are present for synthesis prompts.
	ValidateSynthesisContext(ctx SynthesisContext) []ValidationError

	// ValidateRevisionContext ensures all required fields are present for revision prompts.
	ValidateRevisionContext(ctx RevisionContext) []ValidationError

	// ValidateConsolidationContext ensures all required fields are present for consolidation prompts.
	ValidateConsolidationContext(ctx ConsolidationContext) []ValidationError
}

// TemplateRegistry manages prompt templates for customization.
type TemplateRegistry interface {
	// Register adds a custom template for a specific phase.
	Register(phase PromptPhase, template *PromptTemplate) error

	// Get retrieves the template for a phase (custom or default).
	Get(phase PromptPhase) *PromptTemplate

	// List returns all registered templates.
	List() map[PromptPhase]*PromptTemplate

	// Reset restores default templates for all phases.
	Reset()
}

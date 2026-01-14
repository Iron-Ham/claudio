package decomposition

import (
	"slices"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// -----------------------------------------------------------------------------
// Analysis Types
// -----------------------------------------------------------------------------

// Analysis holds the results of analyzing a plan for enhanced decomposition.
type Analysis struct {
	// TaskAnalyses contains per-task analysis results.
	TaskAnalyses map[string]*TaskAnalysis `json:"task_analyses"`

	// InferredDependencies are dependencies discovered through code analysis
	// that weren't explicitly declared in the original plan.
	InferredDependencies []InferredDependency `json:"inferred_dependencies"`

	// CriticalPath is the longest chain of tasks through the dependency graph.
	// Optimizing this path reduces total execution time.
	CriticalPath []string `json:"critical_path"`

	// CriticalPathLength is the estimated "weight" of the critical path
	// (sum of task complexity weights along the path).
	CriticalPathLength float64 `json:"critical_path_length"`

	// ParallelismScore indicates how well the plan utilizes parallelism (0-100).
	// 100 = perfect parallelism (all tasks in one group), lower = more sequential.
	ParallelismScore int `json:"parallelism_score"`

	// AverageGroupSize is the mean number of tasks per execution group.
	AverageGroupSize float64 `json:"average_group_size"`

	// BottleneckGroups identifies execution groups with only 1 task (sequential bottlenecks).
	BottleneckGroups []int `json:"bottleneck_groups"`

	// FileConflictClusters groups tasks that share files (potential merge conflicts).
	FileConflictClusters []FileConflictCluster `json:"file_conflict_clusters"`

	// PackageHotspots identifies packages modified by multiple tasks.
	PackageHotspots []PackageHotspot `json:"package_hotspots"`

	// RiskDistribution shows how tasks are distributed across risk levels.
	RiskDistribution RiskDistribution `json:"risk_distribution"`

	// Suggestions contains actionable recommendations for improving the plan.
	Suggestions []Suggestion `json:"suggestions"`

	// PackageGraph represents the analyzed package dependency structure.
	PackageGraph *PackageGraph `json:"package_graph,omitempty"`

	// OptimizedExecutionOrder suggests improved execution order.
	// Uses risk-based prioritization within groups.
	OptimizedExecutionOrder [][]string `json:"optimized_execution_order"`

	// OptimizedDependencies contains the reduced dependency graph after
	// transitive reduction (if enabled).
	OptimizedDependencies map[string][]string `json:"optimized_dependencies,omitempty"`

	// RemovedDependencies lists dependencies removed by transitive reduction.
	RemovedDependencies []TransitiveEdge `json:"removed_dependencies,omitempty"`
}

// TransitiveEdge represents a redundant dependency removed by transitive reduction.
type TransitiveEdge struct {
	// From is the dependent task.
	From string `json:"from"`

	// To is the task being depended upon.
	To string `json:"to"`

	// Reason explains why this dependency is redundant.
	Reason string `json:"reason"`
}

// TaskAnalysis holds analysis results for a single task.
type TaskAnalysis struct {
	// TaskID is the identifier of the analyzed task.
	TaskID string `json:"task_id"`

	// RiskScore is the computed risk level (0-100, higher = riskier).
	RiskScore int `json:"risk_score"`

	// RiskLevel is the categorized risk (low/medium/high/critical).
	RiskLevel RiskLevel `json:"risk_level"`

	// RiskFactors explains what contributed to the risk score.
	RiskFactors []RiskFactor `json:"risk_factors"`

	// ImpactScore indicates how much this task affects other tasks (0-100).
	// Tasks with high impact should complete early to unblock dependents.
	ImpactScore int `json:"impact_score"`

	// BlockingScore combines risk and impact for prioritization.
	// Higher blocking scores should run earlier in their group.
	// Formula: (RiskScore * 40 + ImpactScore * 60) / 100
	BlockingScore int `json:"blocking_score"`

	// DependentCount is the number of tasks that directly depend on this one.
	DependentCount int `json:"dependent_count"`

	// TransitiveDependentCount includes indirect dependents.
	TransitiveDependentCount int `json:"transitive_dependent_count"`

	// AffectedPackages lists packages that this task's files belong to.
	AffectedPackages []string `json:"affected_packages"`

	// FileCategories groups this task's files by type/purpose.
	FileCategories map[string][]string `json:"file_categories"`

	// Centrality measures how "central" this task is in the codebase.
	// High centrality means many other files depend on files this task touches.
	Centrality float64 `json:"centrality"`

	// IsCriticalPath indicates if this task is on the critical path.
	IsCriticalPath bool `json:"is_critical_path"`

	// CriticalPathPosition indicates position on the critical path.
	// 0 = not on critical path, higher values = further along the path.
	CriticalPathPosition int `json:"critical_path_position"`

	// SuggestedPriority is the recommended priority adjustment.
	// Negative values mean higher priority (run earlier).
	SuggestedPriority int `json:"suggested_priority"`

	// ParallelizationSafe indicates if this task can safely run in parallel
	// with other tasks in its group based on code analysis.
	ParallelizationSafe bool `json:"parallelization_safe"`

	// FileAnalysis contains per-file analysis within this task.
	FileAnalysis []FileAnalysis `json:"file_analysis,omitempty"`

	// SuggestedSplits recommends ways to split this task if too complex.
	SuggestedSplits []SplitSuggestion `json:"suggested_splits,omitempty"`
}

// FileAnalysis holds analysis results for a single file.
type FileAnalysis struct {
	// Path is the file path relative to repository root.
	Path string `json:"path"`

	// Package is the Go package this file belongs to.
	Package string `json:"package,omitempty"`

	// ImportCount is the number of files that import this file's package.
	ImportCount int `json:"import_count"`

	// IsTestFile indicates if this is a test file.
	IsTestFile bool `json:"is_test_file"`

	// IsInterface indicates if this file defines interfaces used elsewhere.
	IsInterface bool `json:"is_interface"`

	// ChangeFrequency is how often this file changes (if history is available).
	ChangeFrequency float64 `json:"change_frequency,omitempty"`
}

// SplitSuggestion recommends how to split a complex task.
type SplitSuggestion struct {
	// Reason explains why this split is recommended.
	Reason string `json:"reason"`

	// SplitBy describes the splitting strategy.
	// Values: "file-type", "package", "functionality", "risk-isolation"
	SplitBy string `json:"split_by"`

	// ProposedTasks describes the suggested task breakdown.
	ProposedTasks []ProposedTask `json:"proposed_tasks"`
}

// ProposedTask describes a suggested task from splitting.
type ProposedTask struct {
	// Title is a suggested title for the new task.
	Title string `json:"title"`

	// Files lists the files this task would handle.
	Files []string `json:"files"`

	// EstComplexity is the estimated complexity after split.
	EstComplexity ultraplan.TaskComplexity `json:"est_complexity"`
}

// FileConflictCluster groups tasks that modify overlapping files.
type FileConflictCluster struct {
	// Files lists the conflicting files.
	Files []string `json:"files"`

	// TaskIDs lists tasks in this conflict cluster.
	TaskIDs []string `json:"task_ids"`

	// InSameGroup indicates if tasks could run in parallel (more risky).
	InSameGroup bool `json:"in_same_group"`

	// Severity indicates conflict risk: "low", "medium", "high".
	Severity string `json:"severity"`
}

// PackageHotspot identifies a package with high modification activity.
type PackageHotspot struct {
	// Package is the package/directory path.
	Package string `json:"package"`

	// TaskCount is the number of tasks modifying this package.
	TaskCount int `json:"task_count"`

	// TaskIDs lists the tasks.
	TaskIDs []string `json:"task_ids"`

	// FileCount is the total files modified in this package.
	FileCount int `json:"file_count"`
}

// -----------------------------------------------------------------------------
// Risk Types
// -----------------------------------------------------------------------------

// RiskLevel categorizes the risk of a task.
type RiskLevel string

const (
	// RiskLevelLow indicates minimal risk - safe for aggressive parallelization.
	RiskLevelLow RiskLevel = "low"

	// RiskLevelMedium indicates moderate risk - standard parallelization.
	RiskLevelMedium RiskLevel = "medium"

	// RiskLevelHigh indicates significant risk - consider sequential execution.
	RiskLevelHigh RiskLevel = "high"

	// RiskLevelCritical indicates critical risk - should run early and alone.
	RiskLevelCritical RiskLevel = "critical"
)

// RiskFactor describes a specific contributor to risk score.
type RiskFactor struct {
	// Name identifies the risk factor type.
	Name string `json:"name"`

	// Description explains this specific risk.
	Description string `json:"description"`

	// Score is the contribution to total risk score (0-25 typically).
	Score int `json:"score"`

	// Mitigation suggests how to reduce this risk.
	Mitigation string `json:"mitigation,omitempty"`
}

// RiskDistribution shows task counts at each risk level.
type RiskDistribution struct {
	Low      int `json:"low"`
	Medium   int `json:"medium"`
	High     int `json:"high"`
	Critical int `json:"critical"`
}

// Well-known risk factor names.
const (
	RiskFactorFileCount     = "file_count"
	RiskFactorComplexity    = "complexity"
	RiskFactorCentrality    = "centrality"
	RiskFactorSharedFiles   = "shared_files"
	RiskFactorCorePackage   = "core_package"
	RiskFactorInterfaceFile = "interface_file"
	RiskFactorNoTests       = "no_tests"
	RiskFactorHighChurn     = "high_churn"
	RiskFactorCrossPackage  = "cross_package"
	RiskFactorDependencyFan = "dependency_fan"
)

// -----------------------------------------------------------------------------
// Dependency Inference Types
// -----------------------------------------------------------------------------

// InferredDependency represents a dependency discovered through code analysis.
type InferredDependency struct {
	// FromTaskID is the task that should run first.
	FromTaskID string `json:"from_task_id"`

	// ToTaskID is the task that depends on FromTaskID.
	ToTaskID string `json:"to_task_id"`

	// Reason explains why this dependency was inferred.
	Reason string `json:"reason"`

	// Confidence is how confident we are in this inference (0-100).
	Confidence int `json:"confidence"`

	// Type categorizes the kind of dependency.
	Type DependencyType `json:"type"`

	// IsHard indicates if this is a hard dependency (must be respected)
	// or soft (recommendation only).
	IsHard bool `json:"is_hard"`
}

// DependencyType categorizes inferred dependencies.
type DependencyType string

const (
	// DependencyTypeImport indicates dependency due to import relationship.
	DependencyTypeImport DependencyType = "import"

	// DependencyTypeInterface indicates dependency due to shared interface.
	DependencyTypeInterface DependencyType = "interface"

	// DependencyTypeSamePackage indicates tasks modify the same package.
	DependencyTypeSamePackage DependencyType = "same_package"

	// DependencyTypeSharedFile indicates tasks modify the same file.
	DependencyTypeSharedFile DependencyType = "shared_file"

	// DependencyTypeTestDependency indicates test file depends on implementation.
	DependencyTypeTestDependency DependencyType = "test_dependency"
)

// -----------------------------------------------------------------------------
// Suggestion Types
// -----------------------------------------------------------------------------

// Suggestion represents an actionable recommendation for improving the plan.
type Suggestion struct {
	// Type categorizes the suggestion.
	Type SuggestionType `json:"type"`

	// Priority indicates urgency (1=highest, 5=lowest).
	Priority int `json:"priority"`

	// Title is a short description of the suggestion.
	Title string `json:"title"`

	// Description provides detailed explanation.
	Description string `json:"description"`

	// AffectedTasks lists task IDs this suggestion relates to.
	AffectedTasks []string `json:"affected_tasks,omitempty"`

	// Impact describes the expected benefit of implementing this suggestion.
	Impact string `json:"impact,omitempty"`
}

// SuggestionType categorizes suggestions.
type SuggestionType string

const (
	// SuggestionSplitTask recommends splitting a task into smaller units.
	SuggestionSplitTask SuggestionType = "split_task"

	// SuggestionMergeTasks recommends combining tasks that are too small.
	SuggestionMergeTasks SuggestionType = "merge_tasks"

	// SuggestionAddDependency recommends adding a dependency between tasks.
	SuggestionAddDependency SuggestionType = "add_dependency"

	// SuggestionRemoveDependency recommends removing an unnecessary dependency.
	SuggestionRemoveDependency SuggestionType = "remove_dependency"

	// SuggestionReorderTasks recommends changing task priorities.
	SuggestionReorderTasks SuggestionType = "reorder_tasks"

	// SuggestionIsolateRisky recommends isolating a risky task.
	SuggestionIsolateRisky SuggestionType = "isolate_risky"

	// SuggestionParallelize recommends parallelizing sequential tasks.
	SuggestionParallelize SuggestionType = "parallelize"
)

// -----------------------------------------------------------------------------
// Package Graph Types
// -----------------------------------------------------------------------------

// PackageGraph represents the import dependency structure of the codebase.
type PackageGraph struct {
	// Packages maps package path to package info.
	Packages map[string]*PackageInfo `json:"packages"`

	// Edges maps package path to its direct dependencies.
	Edges map[string][]string `json:"edges"`
}

// PackageInfo holds metadata about a Go package.
type PackageInfo struct {
	// Path is the import path of the package.
	Path string `json:"path"`

	// Name is the package name (usually last component of path).
	Name string `json:"name"`

	// Files lists files in this package.
	Files []string `json:"files"`

	// ImportedBy lists packages that import this package.
	ImportedBy []string `json:"imported_by"`

	// Imports lists packages this package imports.
	Imports []string `json:"imports"`

	// IsInternal indicates if this is an internal package.
	IsInternal bool `json:"is_internal"`

	// Centrality is a measure of this package's importance in the graph.
	Centrality float64 `json:"centrality"`
}

// -----------------------------------------------------------------------------
// Enhanced Execution Order
// -----------------------------------------------------------------------------

// EnhancedExecutionOrder extends the basic execution order with risk-aware grouping.
type EnhancedExecutionOrder struct {
	// Groups contains task IDs organized for execution.
	// Unlike the basic ExecutionOrder, this accounts for risk.
	Groups []ExecutionGroup `json:"groups"`

	// CriticalPathTasks are tasks that must complete for others to proceed.
	CriticalPathTasks []string `json:"critical_path_tasks"`

	// EstimatedParallelism is the average parallelism achievable.
	EstimatedParallelism float64 `json:"estimated_parallelism"`
}

// ExecutionGroup represents a set of tasks that can execute together.
type ExecutionGroup struct {
	// Index is the group number (0-indexed).
	Index int `json:"index"`

	// Tasks lists task IDs in this group.
	Tasks []string `json:"tasks"`

	// IsolatedTasks are tasks that should run alone due to risk.
	// These run sequentially within the group before parallel tasks.
	IsolatedTasks []string `json:"isolated_tasks,omitempty"`

	// ParallelTasks are tasks safe to run concurrently.
	ParallelTasks []string `json:"parallel_tasks,omitempty"`

	// MaxConcurrency is the recommended max parallel tasks for this group.
	MaxConcurrency int `json:"max_concurrency"`
}

// -----------------------------------------------------------------------------
// Analyzer Configuration
// -----------------------------------------------------------------------------

// AnalyzerConfig configures the decomposition analyzer behavior.
type AnalyzerConfig struct {
	// EnableGitHistory enables analysis of git history for churn detection.
	EnableGitHistory bool `json:"enable_git_history"`

	// GitHistoryDepth is how many commits to analyze (0 = all available).
	GitHistoryDepth int `json:"git_history_depth"`

	// RiskThresholds customize risk level boundaries.
	RiskThresholds RiskThresholds `json:"risk_thresholds"`

	// InferDependencies enables automatic dependency inference.
	InferDependencies bool `json:"infer_dependencies"`

	// DependencyConfidenceThreshold is the minimum confidence to accept
	// an inferred dependency (0-100).
	DependencyConfidenceThreshold int `json:"dependency_confidence_threshold"`

	// CorePackagePatterns are patterns that identify core/critical packages.
	// Tasks touching these packages get higher risk scores.
	CorePackagePatterns []string `json:"core_package_patterns"`

	// EnableTransitiveReduction enables simplification of dependency graphs.
	// Removes redundant dependencies (e.g., if A→B→C exists, A→C is redundant).
	EnableTransitiveReduction bool `json:"enable_transitive_reduction"`
}

// RiskThresholds defines score boundaries for risk levels.
type RiskThresholds struct {
	Low    int `json:"low"`    // Below this = low risk
	Medium int `json:"medium"` // Below this = medium risk
	High   int `json:"high"`   // Below this = high risk
	// Above High = critical
}

// DefaultAnalyzerConfig returns sensible defaults for the analyzer.
func DefaultAnalyzerConfig() AnalyzerConfig {
	return AnalyzerConfig{
		EnableGitHistory:              true,
		GitHistoryDepth:               100,
		InferDependencies:             true,
		DependencyConfidenceThreshold: 70,
		RiskThresholds: RiskThresholds{
			Low:    25,
			Medium: 50,
			High:   75,
		},
		CorePackagePatterns: []string{
			"internal/config",
			"internal/core",
			"pkg/api",
			"cmd/",
		},
		EnableTransitiveReduction: true,
	}
}

// -----------------------------------------------------------------------------
// Apply Analysis to Plan
// -----------------------------------------------------------------------------

// Apply applies the analysis results to enhance a plan.
// Returns a new PlanSpec with improved dependencies and execution order.
func (a *Analysis) Apply(plan *ultraplan.PlanSpec) *ultraplan.PlanSpec {
	if plan == nil || a == nil {
		return plan
	}

	// Create a copy of the plan
	enhanced := &ultraplan.PlanSpec{
		ID:              plan.ID,
		Objective:       plan.Objective,
		Summary:         plan.Summary,
		Tasks:           make([]ultraplan.PlannedTask, len(plan.Tasks)),
		DependencyGraph: make(map[string][]string),
		Insights:        plan.Insights,
		Constraints:     plan.Constraints,
		CreatedAt:       plan.CreatedAt,
	}

	// Copy tasks and apply priority adjustments
	for i, task := range plan.Tasks {
		enhanced.Tasks[i] = task
		if analysis, ok := a.TaskAnalyses[task.ID]; ok {
			enhanced.Tasks[i].Priority = analysis.SuggestedPriority
		}
	}

	// Build enhanced dependency graph
	// Start with original dependencies
	for taskID, deps := range plan.DependencyGraph {
		enhanced.DependencyGraph[taskID] = append([]string{}, deps...)
	}

	// Add hard inferred dependencies
	for _, dep := range a.InferredDependencies {
		if dep.IsHard {
			existing := enhanced.DependencyGraph[dep.ToTaskID]
			// Check if already present
			if !slices.Contains(existing, dep.FromTaskID) {
				enhanced.DependencyGraph[dep.ToTaskID] = append(existing, dep.FromTaskID)
				// Also update the task's DependsOn
				for i := range enhanced.Tasks {
					if enhanced.Tasks[i].ID == dep.ToTaskID {
						enhanced.Tasks[i].DependsOn = enhanced.DependencyGraph[dep.ToTaskID]
						break
					}
				}
			}
		}
	}

	// Recalculate execution order with the enhanced dependencies
	enhanced.ExecutionOrder = ultraplan.CalculateExecutionOrder(enhanced.Tasks, enhanced.DependencyGraph)

	// Add analysis insights to plan constraints
	if len(a.CriticalPath) > 0 {
		enhanced.Constraints = append(enhanced.Constraints,
			"Critical path identified: tasks on this path determine minimum execution time")
	}

	if a.RiskDistribution.Critical > 0 {
		enhanced.Constraints = append(enhanced.Constraints,
			"Contains critical-risk tasks that should be monitored closely")
	}

	return enhanced
}

// GetEnhancedExecutionOrder returns a risk-aware execution order.
func (a *Analysis) GetEnhancedExecutionOrder(plan *ultraplan.PlanSpec) *EnhancedExecutionOrder {
	if plan == nil || a == nil {
		return nil
	}

	enhanced := &EnhancedExecutionOrder{
		Groups:            make([]ExecutionGroup, len(plan.ExecutionOrder)),
		CriticalPathTasks: a.CriticalPath,
	}

	totalParallelism := 0.0
	for i, group := range plan.ExecutionOrder {
		eg := ExecutionGroup{
			Index:          i,
			Tasks:          group,
			MaxConcurrency: len(group),
		}

		// Separate isolated (high-risk) and parallel-safe tasks
		for _, taskID := range group {
			if analysis, ok := a.TaskAnalyses[taskID]; ok {
				if analysis.RiskLevel == RiskLevelCritical || analysis.RiskLevel == RiskLevelHigh {
					eg.IsolatedTasks = append(eg.IsolatedTasks, taskID)
				} else if analysis.ParallelizationSafe {
					eg.ParallelTasks = append(eg.ParallelTasks, taskID)
				} else {
					// Medium risk - can parallel but with lower concurrency
					eg.ParallelTasks = append(eg.ParallelTasks, taskID)
				}
			} else {
				// No analysis - assume parallel safe
				eg.ParallelTasks = append(eg.ParallelTasks, taskID)
			}
		}

		// Adjust max concurrency based on risk distribution
		if len(eg.IsolatedTasks) > 0 {
			// If we have isolated tasks, they run first with concurrency 1
			// Then parallel tasks run with full concurrency
			eg.MaxConcurrency = max(1, len(eg.ParallelTasks))
		}

		enhanced.Groups[i] = eg
		totalParallelism += float64(len(eg.ParallelTasks))
	}

	if len(plan.ExecutionOrder) > 0 {
		enhanced.EstimatedParallelism = totalParallelism / float64(len(plan.ExecutionOrder))
	}

	return enhanced
}

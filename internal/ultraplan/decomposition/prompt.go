package decomposition

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// EnhancedPlanningGuidance generates additional planning guidance based on analysis.
// This can be appended to the base planning prompt to provide code-aware hints.
func (a *Analyzer) EnhancedPlanningGuidance(analysis *Analysis) string {
	var sb strings.Builder

	sb.WriteString("\n## Code-Aware Decomposition Guidance\n\n")
	sb.WriteString("Based on analysis of the planned tasks, consider the following:\n\n")

	// Risk summary
	riskSummary := analysis.GetRiskSummary()
	if len(riskSummary) > 0 {
		sb.WriteString("### Risk Assessment\n")
		for risk, count := range riskSummary {
			sb.WriteString(fmt.Sprintf("- %s risk: %d tasks\n", risk, count))
		}
		sb.WriteString("\n")
	}

	// High-risk tasks
	highRisk := analysis.GetHighRiskTasks()
	if len(highRisk) > 0 {
		sb.WriteString("### High-Risk Tasks (Consider Sequential Execution)\n")
		for _, taskID := range highRisk {
			sb.WriteString(fmt.Sprintf("- `%s`\n", taskID))
		}
		sb.WriteString("\n")
	}

	// Critical path
	if len(analysis.CriticalPath) > 0 {
		sb.WriteString("### Critical Path\n")
		sb.WriteString("The longest dependency chain is:\n")
		sb.WriteString(strings.Join(analysis.CriticalPath, " → "))
		sb.WriteString("\n\n")
	}

	// File conflict warnings
	if len(analysis.FileConflictClusters) > 0 {
		var highSeverity []FileConflictCluster
		for _, c := range analysis.FileConflictClusters {
			if c.Severity == "high" || c.Severity == "medium" {
				highSeverity = append(highSeverity, c)
			}
		}
		if len(highSeverity) > 0 {
			sb.WriteString("### File Coupling Warnings\n")
			sb.WriteString("These task groups share files and may cause conflicts:\n")
			for _, c := range highSeverity {
				sb.WriteString(fmt.Sprintf("- Tasks %v share %d files (severity: %s)\n",
					c.TaskIDs, len(c.Files), c.Severity))
			}
			sb.WriteString("\n")
		}
	}

	// Suggestions
	if len(analysis.Suggestions) > 0 {
		sb.WriteString("### Decomposition Suggestions\n")
		for _, suggestion := range analysis.Suggestions {
			if suggestion.Priority <= 2 { // Only high-priority suggestions
				sb.WriteString(fmt.Sprintf("- **%s**: %s\n", suggestion.Title, suggestion.Description))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// GenerateRiskAwarePlanningPrompt creates an enhanced planning prompt
// that incorporates risk-based decomposition guidance.
func GenerateRiskAwarePlanningPrompt(objective string, strategy string) string {
	basePrompt := ultraplan.GetMultiPassPlanningPrompt(strategy, objective)

	riskGuidance := `

## Risk-Based Decomposition Guidelines

When decomposing tasks, consider these risk factors:

### High-Risk Indicators
Tasks with these characteristics should be kept small and may need sequential execution:
1. **Configuration/Schema Changes** - Files like config.*, schema.*, migrations/*
2. **Core Module Modifications** - main.go, core business logic
3. **Cross-Cutting Concerns** - Changes spanning 3+ packages/directories
4. **High File Count** - Tasks touching more than 5 files
5. **Test Files** - *_test.go files (may indicate integration points)

### Safe Parallelization
Tasks are safer to run in parallel when they:
1. Touch completely separate files (no overlap)
2. Work on isolated packages/modules
3. Have low complexity with clear boundaries
4. Don't modify shared configuration

### Dependency Recommendations
- Add explicit dependencies between tasks that share files
- Use transitive dependencies sparingly - prefer direct dependencies
- Tasks on the critical path should have minimal dependencies
- High-risk tasks should depend on foundation tasks completing first

### Task Sizing Guidelines
- Prefer many small tasks over few large ones
- Target 1-3 files per task for easy parallel execution
- Split cross-package changes into per-package tasks
- Configuration changes should be their own dedicated task

### Merge Conflict Prevention
When multiple tasks might touch the same file:
1. Consider if they can be combined into one task
2. Add explicit dependencies to serialize the changes
3. Assign clear ownership - one "owner" task, others depend on it
4. For test files, consider a dedicated "update tests" task at the end
`

	return basePrompt + riskGuidance
}

// GeneratePostAnalysisRecommendations creates recommendations after plan analysis.
// This is useful for the plan review/selection phase.
func GeneratePostAnalysisRecommendations(plan *ultraplan.PlanSpec, analysis *Analysis) string {
	var sb strings.Builder

	sb.WriteString("## Plan Analysis Recommendations\n\n")

	// Parallelization efficiency
	parallelTasks := 0
	totalTasks := len(plan.Tasks)
	for _, group := range plan.ExecutionOrder {
		if len(group) > 1 {
			parallelTasks += len(group) - 1
		}
	}
	parallelRatio := float64(0)
	if totalTasks > 0 {
		parallelRatio = float64(parallelTasks) / float64(totalTasks) * 100
	}
	sb.WriteString(fmt.Sprintf("### Parallelization Efficiency: %.1f%%\n", parallelRatio))
	sb.WriteString(fmt.Sprintf("- Total tasks: %d\n", totalTasks))
	sb.WriteString(fmt.Sprintf("- Execution groups: %d\n", len(plan.ExecutionOrder)))
	sb.WriteString(fmt.Sprintf("- Parallelism score: %d/100\n\n", analysis.ParallelismScore))

	// Risk distribution
	sb.WriteString("### Risk Distribution\n")
	total := analysis.RiskDistribution.Low + analysis.RiskDistribution.Medium +
		analysis.RiskDistribution.High + analysis.RiskDistribution.Critical
	if total > 0 {
		sb.WriteString(fmt.Sprintf("- Low risk: %d tasks (%.1f%%)\n",
			analysis.RiskDistribution.Low, float64(analysis.RiskDistribution.Low)/float64(total)*100))
		sb.WriteString(fmt.Sprintf("- Medium risk: %d tasks (%.1f%%)\n",
			analysis.RiskDistribution.Medium, float64(analysis.RiskDistribution.Medium)/float64(total)*100))
		sb.WriteString(fmt.Sprintf("- High risk: %d tasks (%.1f%%)\n",
			analysis.RiskDistribution.High, float64(analysis.RiskDistribution.High)/float64(total)*100))
		sb.WriteString(fmt.Sprintf("- Critical risk: %d tasks (%.1f%%)\n\n",
			analysis.RiskDistribution.Critical, float64(analysis.RiskDistribution.Critical)/float64(total)*100))
	}

	// Bottleneck analysis
	if len(analysis.BottleneckGroups) > 0 {
		sb.WriteString("### Sequential Bottlenecks\n")
		sb.WriteString(fmt.Sprintf("Found %d single-task execution groups that may slow down execution.\n\n",
			len(analysis.BottleneckGroups)))
	}

	// Critical path analysis
	if len(analysis.CriticalPath) > 0 {
		sb.WriteString("### Critical Path Analysis\n")
		sb.WriteString(fmt.Sprintf("Longest dependency chain: %d tasks\n", len(analysis.CriticalPath)))
		sb.WriteString("Path: " + strings.Join(analysis.CriticalPath, " → ") + "\n\n")
		if len(analysis.CriticalPath) > 3 {
			sb.WriteString("Consider breaking dependencies on the critical path to improve parallelism.\n\n")
		}
	}

	// Package hotspots
	if len(analysis.PackageHotspots) > 0 {
		sb.WriteString("### Package Hotspots\n")
		sb.WriteString("These packages are modified by multiple tasks (potential for conflicts):\n")
		for _, hotspot := range analysis.PackageHotspots {
			if hotspot.TaskCount > 2 {
				sb.WriteString(fmt.Sprintf("- `%s`: %d tasks, %d files\n",
					hotspot.Package, hotspot.TaskCount, hotspot.FileCount))
			}
		}
		sb.WriteString("\n")
	}

	// Actionable improvements from suggestions
	if len(analysis.Suggestions) > 0 {
		sb.WriteString("### Suggested Improvements\n")
		for i, suggestion := range analysis.Suggestions {
			if i >= 5 { // Limit to top 5
				break
			}
			sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, suggestion.Title))
			if suggestion.Description != "" {
				sb.WriteString(fmt.Sprintf("   %s\n", suggestion.Description))
			}
		}
	}

	return sb.String()
}

// OptimizePlanDependencies applies transitive reduction to simplify a plan's dependencies.
// Returns a new plan with optimized dependencies.
func OptimizePlanDependencies(plan *ultraplan.PlanSpec, removedDeps []DependencyEdge) *ultraplan.PlanSpec {
	if len(removedDeps) == 0 {
		return plan
	}

	// Build set of edges to remove
	removeSet := make(map[string]map[string]bool)
	for _, edge := range removedDeps {
		if removeSet[edge.From] == nil {
			removeSet[edge.From] = make(map[string]bool)
		}
		removeSet[edge.From][edge.To] = true
	}

	// Create a copy of the plan with optimized dependencies
	optimized := &ultraplan.PlanSpec{
		ID:              plan.ID,
		Objective:       plan.Objective,
		Summary:         plan.Summary,
		Tasks:           make([]ultraplan.PlannedTask, len(plan.Tasks)),
		DependencyGraph: make(map[string][]string),
		Insights:        plan.Insights,
		Constraints:     plan.Constraints,
		CreatedAt:       plan.CreatedAt,
	}

	// Copy tasks with optimized dependencies
	for i, task := range plan.Tasks {
		newTask := task
		var filteredDeps []string
		for _, dep := range task.DependsOn {
			// Keep dependency if not in remove set
			if removeSet[task.ID] == nil || !removeSet[task.ID][dep] {
				filteredDeps = append(filteredDeps, dep)
			}
		}
		newTask.DependsOn = filteredDeps
		optimized.Tasks[i] = newTask
		optimized.DependencyGraph[newTask.ID] = filteredDeps
	}

	// Recalculate execution order
	optimized.ExecutionOrder = ultraplan.CalculateExecutionOrder(optimized.Tasks, optimized.DependencyGraph)

	return optimized
}

// DependencyEdge represents a dependency relationship between tasks.
type DependencyEdge struct {
	// From is the dependent task.
	From string

	// To is the task being depended upon.
	To string
}

// GetRiskSummary returns a map of risk level to count.
func (a *Analysis) GetRiskSummary() map[string]int {
	return map[string]int{
		"low":      a.RiskDistribution.Low,
		"medium":   a.RiskDistribution.Medium,
		"high":     a.RiskDistribution.High,
		"critical": a.RiskDistribution.Critical,
	}
}

// GetHighRiskTasks returns task IDs with high or critical risk.
func (a *Analysis) GetHighRiskTasks() []string {
	var highRisk []string
	for taskID, ta := range a.TaskAnalyses {
		if ta.RiskLevel == RiskLevelHigh || ta.RiskLevel == RiskLevelCritical {
			highRisk = append(highRisk, taskID)
		}
	}
	return highRisk
}

// Package ultraplan provides types and utilities for Ultra-Plan orchestration.
package ultraplan

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// PlanFileName is the default filename for plan files.
const PlanFileName = ".claudio-plan.json"

// ParsePlanFromOutput parses a plan from Claude's output.
// It looks for JSON wrapped in <plan></plan> tags.
func ParsePlanFromOutput(output string, objective string) (*PlanSpec, error) {
	// Look for <plan>...</plan> tags
	re := regexp.MustCompile(`(?s)<plan>\s*(.*?)\s*</plan>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no plan found in output (expected <plan>JSON</plan>)")
	}

	jsonStr := strings.TrimSpace(matches[1])

	// Parse the JSON
	var rawPlan struct {
		Summary     string        `json:"summary"`
		Tasks       []PlannedTask `json:"tasks"`
		Insights    []string      `json:"insights"`
		Constraints []string      `json:"constraints"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if len(rawPlan.Tasks) == 0 {
		return nil, fmt.Errorf("plan contains no tasks")
	}

	// Build the PlanSpec
	plan := &PlanSpec{
		ID:              generateID(),
		Objective:       objective,
		Summary:         rawPlan.Summary,
		Tasks:           rawPlan.Tasks,
		Insights:        rawPlan.Insights,
		Constraints:     rawPlan.Constraints,
		DependencyGraph: make(map[string][]string),
		CreatedAt:       time.Now(),
	}

	// Build dependency graph
	for _, task := range plan.Tasks {
		plan.DependencyGraph[task.ID] = task.DependsOn
	}

	// Calculate execution order (topological sort with parallel grouping)
	plan.ExecutionOrder = CalculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	return plan, nil
}

// ParsePlanFromFile reads and parses a plan from a JSON file.
// It supports two formats:
//  1. Root-level format: {"summary": "...", "tasks": [...]}
//  2. Nested format: {"plan": {"summary": "...", "tasks": [...]}}
//
// It also handles alternative field names that Claude may generate:
//   - "depends" as alias for "depends_on"
//   - "complexity" as alias for "est_complexity"
func ParsePlanFromFile(filePath string, objective string) (*PlanSpec, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	// flexibleTask handles alternative field names that Claude may generate
	type flexibleTask struct {
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Description   string   `json:"description"`
		Files         []string `json:"files,omitempty"`
		DependsOn     []string `json:"depends_on"`
		Depends       []string `json:"depends"` // Alternative name
		Priority      int      `json:"priority"`
		EstComplexity string   `json:"est_complexity"`
		Complexity    string   `json:"complexity"` // Alternative name
	}

	type planContent struct {
		Summary     string         `json:"summary"`
		Tasks       []flexibleTask `json:"tasks"`
		Insights    []string       `json:"insights"`
		Constraints []string       `json:"constraints"`
	}

	// Try parsing as root-level format first
	var rawPlan planContent
	if err := json.Unmarshal(data, &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// If no tasks found, try nested "plan" wrapper format
	if len(rawPlan.Tasks) == 0 {
		var wrapped struct {
			Plan planContent `json:"plan"`
		}
		if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Plan.Tasks) > 0 {
			rawPlan = wrapped.Plan
		}
	}

	if len(rawPlan.Tasks) == 0 {
		return nil, fmt.Errorf("plan contains no tasks")
	}

	// Convert flexible tasks to PlannedTask, handling alternative field names
	tasks := make([]PlannedTask, len(rawPlan.Tasks))
	for i, ft := range rawPlan.Tasks {
		// Use depends_on if set, otherwise fall back to depends
		dependsOn := ft.DependsOn
		if len(dependsOn) == 0 && len(ft.Depends) > 0 {
			dependsOn = ft.Depends
		}

		// Use est_complexity if set, otherwise fall back to complexity
		complexity := ft.EstComplexity
		if complexity == "" && ft.Complexity != "" {
			complexity = ft.Complexity
		}

		tasks[i] = PlannedTask{
			ID:            ft.ID,
			Title:         ft.Title,
			Description:   ft.Description,
			Files:         ft.Files,
			DependsOn:     dependsOn,
			Priority:      ft.Priority,
			EstComplexity: TaskComplexity(complexity),
		}
	}

	// Build the PlanSpec
	plan := &PlanSpec{
		ID:              generateID(),
		Objective:       objective,
		Summary:         rawPlan.Summary,
		Tasks:           tasks,
		Insights:        rawPlan.Insights,
		Constraints:     rawPlan.Constraints,
		DependencyGraph: make(map[string][]string),
		CreatedAt:       time.Now(),
	}

	// Build dependency graph
	for _, task := range plan.Tasks {
		plan.DependencyGraph[task.ID] = task.DependsOn
	}

	// Calculate execution order (topological sort with parallel grouping)
	plan.ExecutionOrder = CalculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	return plan, nil
}

// PlanFilePath returns the full path to the plan file for a given worktree.
func PlanFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, PlanFileName)
}

// CalculateExecutionOrder performs a topological sort and groups tasks that can run in parallel.
func CalculateExecutionOrder(tasks []PlannedTask, deps map[string][]string) [][]string {
	// Build in-degree map
	inDegree := make(map[string]int)
	for _, task := range tasks {
		inDegree[task.ID] = len(task.DependsOn)
	}

	// Find tasks with no dependencies (in-degree 0)
	var groups [][]string
	completed := make(map[string]bool)

	for len(completed) < len(tasks) {
		var currentGroup []string

		// Find all tasks that can run now (in-degree 0 and not completed)
		for _, task := range tasks {
			if completed[task.ID] {
				continue
			}
			if inDegree[task.ID] == 0 {
				currentGroup = append(currentGroup, task.ID)
			}
		}

		if len(currentGroup) == 0 {
			// Cycle detected or invalid graph
			break
		}

		// Sort by priority within the group
		taskPriority := make(map[string]int)
		for _, task := range tasks {
			taskPriority[task.ID] = task.Priority
		}
		sort.Slice(currentGroup, func(i, j int) bool {
			return taskPriority[currentGroup[i]] < taskPriority[currentGroup[j]]
		})

		groups = append(groups, currentGroup)

		// Mark these tasks as completed and update in-degrees
		for _, taskID := range currentGroup {
			completed[taskID] = true
			// Reduce in-degree for tasks that depend on this one
			for _, task := range tasks {
				for _, depID := range task.DependsOn {
					if depID == taskID {
						inDegree[task.ID]--
					}
				}
			}
		}
	}

	return groups
}

// ParseRevisionIssuesFromOutput extracts revision issues from synthesis output.
// It looks for JSON wrapped in <revision_issues></revision_issues> tags.
func ParseRevisionIssuesFromOutput(output string) ([]RevisionIssue, error) {
	// Look for <revision_issues>...</revision_issues> tags
	re := regexp.MustCompile(`(?s)<revision_issues>\s*(.*?)\s*</revision_issues>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		// No revision issues block found - assume no issues
		return nil, nil
	}

	jsonStr := strings.TrimSpace(matches[1])

	// Handle empty array
	if jsonStr == "[]" || jsonStr == "" {
		return nil, nil
	}

	// Parse the JSON array
	var issues []RevisionIssue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse revision issues JSON: %w", err)
	}

	// Filter to only include issues with actual content
	var validIssues []RevisionIssue
	for _, issue := range issues {
		if issue.Description != "" {
			validIssues = append(validIssues, issue)
		}
	}

	return validIssues, nil
}

// ParsePlanDecisionFromOutput extracts the plan decision from coordinator-manager output.
// It looks for JSON wrapped in <plan_decision></plan_decision> tags.
func ParsePlanDecisionFromOutput(output string) (*PlanDecision, error) {
	// Look for <plan_decision>...</plan_decision> tags
	re := regexp.MustCompile(`(?s)<plan_decision>\s*(.*?)\s*</plan_decision>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no plan decision found in output (expected <plan_decision>JSON</plan_decision>)")
	}

	jsonStr := strings.TrimSpace(matches[1])

	if jsonStr == "" {
		return nil, fmt.Errorf("empty plan decision block")
	}

	// Parse the JSON
	var decision PlanDecision
	if err := json.Unmarshal([]byte(jsonStr), &decision); err != nil {
		return nil, fmt.Errorf("failed to parse plan decision JSON: %w", err)
	}

	// Validate the decision
	if decision.Action != "select" && decision.Action != "merge" {
		return nil, fmt.Errorf("invalid plan decision action: %q (expected \"select\" or \"merge\")", decision.Action)
	}

	// Validate selected_index against the number of planning strategies
	maxIndex := len(PlanningStrategies) - 1
	if decision.Action == "select" && (decision.SelectedIndex < 0 || decision.SelectedIndex > maxIndex) {
		return nil, fmt.Errorf("invalid selected_index for select action: %d (expected 0-%d)", decision.SelectedIndex, maxIndex)
	}

	if decision.Action == "merge" && decision.SelectedIndex != -1 {
		return nil, fmt.Errorf("selected_index should be -1 for merge action, got %d", decision.SelectedIndex)
	}

	return &decision, nil
}

// ValidatePlanSimple checks the plan for basic validity (no cycles, valid dependencies).
// This is a simpler validation that returns just an error for quick checks.
// Use ValidatePlan for comprehensive validation with detailed messages.
func ValidatePlanSimple(plan *PlanSpec) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}

	if len(plan.Tasks) == 0 {
		return fmt.Errorf("plan has no tasks")
	}

	// Check for valid task IDs in dependencies
	taskSet := make(map[string]bool)
	for _, task := range plan.Tasks {
		taskSet[task.ID] = true
	}

	for _, task := range plan.Tasks {
		for _, depID := range task.DependsOn {
			if !taskSet[depID] {
				return fmt.Errorf("task %s depends on unknown task %s", task.ID, depID)
			}
		}
	}

	// Check for cycles by verifying all tasks appear in execution order
	if plan.ExecutionOrder != nil {
		scheduledTasks := 0
		for _, group := range plan.ExecutionOrder {
			scheduledTasks += len(group)
		}
		if scheduledTasks < len(plan.Tasks) {
			return fmt.Errorf("dependency cycle detected: only %d of %d tasks can be scheduled",
				scheduledTasks, len(plan.Tasks))
		}
	}

	return nil
}

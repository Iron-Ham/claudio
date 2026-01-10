package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// Config holds configuration for plan-only mode
type Config struct {
	MultiPass    bool
	Labels       []string
	DryRun       bool
	OutputFile   string
	OutputFormat string // "json", "issues", or "both"
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		MultiPass:    false,
		Labels:       nil,
		DryRun:       false,
		OutputFile:   "",
		OutputFormat: "issues",
	}
}

// PlanningPrompt is the prompt template for CLI-based planning
// This is a simplified version of ultraplan's prompt for synchronous execution
const PlanningPrompt = `You are a senior software architect planning a complex task.

## Objective
{{.Objective}}

## Instructions

1. **Explore** the codebase to understand its structure and patterns
2. **Decompose** the objective into discrete, parallelizable tasks
3. **Output your plan** as a JSON object

## Plan JSON Schema

Output a JSON object with this structure:
- "summary": Brief executive summary (string)
- "tasks": Array of task objects, each with:
  - "id": Unique identifier like "task-1-setup" (string)
  - "title": Short title (string)
  - "description": Detailed instructions for another developer to execute independently (string)
  - "files": Files this task will modify (array of strings)
  - "depends_on": IDs of tasks that must complete first (array of strings, empty for independent tasks)
  - "priority": Lower = higher priority within dependency level (number)
  - "est_complexity": "low", "medium", or "high" (string)
- "insights": Key findings about the codebase (array of strings)
- "constraints": Risks or constraints to consider (array of strings)

## Guidelines

- **Prefer many small tasks over fewer large ones** - 10 small tasks are better than 3 medium/large tasks
- Each task should be completable within a single focused session
- Target "low" complexity tasks; split "medium" or "high" complexity work into multiple smaller tasks
- Prefer granular tasks that can run in parallel over large sequential ones
- Assign clear file ownership to avoid merge conflicts
- Each task description should be complete enough for independent execution

## Response Format

Respond ONLY with valid JSON. Do not include any text before or after the JSON object.
Do not wrap the JSON in markdown code blocks.
`

// MultiPassStrategy defines a strategic approach for planning
type MultiPassStrategy struct {
	Name        string
	Description string
	ExtraPrompt string
}

// MultiPassStrategies are the three planning strategies
var MultiPassStrategies = []MultiPassStrategy{
	{
		Name:        "maximize-parallelism",
		Description: "Optimize for maximum parallel execution",
		ExtraPrompt: `
## Strategic Focus: Maximize Parallelism

Prioritize these principles:
1. Minimize Dependencies: Structure tasks to have as few inter-task dependencies as possible
2. Prefer Smaller Tasks: Break work into many small, independent units
3. Isolate File Ownership: Assign each file to exactly one task where possible
4. Flatten the Dependency Graph: Aim for a wide, shallow execution graph
`,
	},
	{
		Name:        "minimize-complexity",
		Description: "Optimize for simplicity and clarity",
		ExtraPrompt: `
## Strategic Focus: Minimize Complexity

Prioritize these principles:
1. Single Responsibility: Each task should do exactly one thing well
2. Clear Boundaries: Tasks should have well-defined inputs and outputs
3. Natural Code Boundaries: Align task boundaries with codebase structure
4. Explicit Over Implicit: Make dependencies explicit even if it reduces parallelism
`,
	},
	{
		Name:        "balanced-approach",
		Description: "Balance parallelism, complexity, and dependencies",
		ExtraPrompt: `
## Strategic Focus: Balanced Approach

Balance these concerns:
1. Respect Natural Structure: Follow the codebase's existing architecture
2. Pragmatic Dependencies: Include dependencies that reflect genuine execution order
3. Right-Sized Tasks: Tasks should be large enough to be meaningful but small enough to complete quickly
4. Consider Integration: Group changes that will need to be tested together
`,
	},
}

// RunPlanningSync runs the planning phase synchronously using claude --print
func RunPlanningSync(objective string, multiPass bool) (*orchestrator.PlanSpec, error) {
	if multiPass {
		return runMultiPassPlanningSync(objective)
	}
	return runSinglePassPlanningSync(objective)
}

func runSinglePassPlanningSync(objective string) (*orchestrator.PlanSpec, error) {
	prompt, err := buildPlanningPrompt(objective, "")
	if err != nil {
		return nil, fmt.Errorf("failed to build planning prompt: %w", err)
	}

	output, err := runClaude(prompt)
	if err != nil {
		return nil, fmt.Errorf("planning failed: %w", err)
	}

	return parsePlanFromOutput(output, objective)
}

func runMultiPassPlanningSync(objective string) (*orchestrator.PlanSpec, error) {
	// Run all three strategies and collect plans
	var plans []*orchestrator.PlanSpec

	for _, strategy := range MultiPassStrategies {
		fmt.Printf("Planning with %s strategy...\n", strategy.Name)

		prompt, err := buildPlanningPrompt(objective, strategy.ExtraPrompt)
		if err != nil {
			return nil, fmt.Errorf("failed to build planning prompt for %s: %w", strategy.Name, err)
		}

		output, err := runClaude(prompt)
		if err != nil {
			fmt.Printf("Warning: %s strategy failed: %v\n", strategy.Name, err)
			continue
		}

		plan, err := parsePlanFromOutput(output, objective)
		if err != nil {
			fmt.Printf("Warning: failed to parse %s plan: %v\n", strategy.Name, err)
			continue
		}

		plans = append(plans, plan)
	}

	if len(plans) == 0 {
		return nil, fmt.Errorf("all planning strategies failed")
	}

	// For simplicity in CLI mode, select the plan with the most tasks
	// (In TUI mode, we would use the plan manager to evaluate and select)
	best := plans[0]
	for _, p := range plans[1:] {
		if len(p.Tasks) > len(best.Tasks) {
			best = p
		}
	}

	return best, nil
}

func buildPlanningPrompt(objective, extraPrompt string) (string, error) {
	tmpl, err := template.New("planning").Parse(PlanningPrompt)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"Objective": objective}); err != nil {
		return "", err
	}

	prompt := buf.String()
	if extraPrompt != "" {
		prompt += "\n" + extraPrompt
	}

	return prompt, nil
}

func runClaude(prompt string) (string, error) {
	cmd := exec.Command("claude", "--print", prompt)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("claude command failed: %w\nstderr: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("failed to run claude: %w", err)
	}

	return string(output), nil
}

func parsePlanFromOutput(output, objective string) (*orchestrator.PlanSpec, error) {
	// Clean up the output - remove any markdown code blocks
	output = strings.TrimSpace(output)
	output = strings.TrimPrefix(output, "```json")
	output = strings.TrimPrefix(output, "```")
	output = strings.TrimSuffix(output, "```")
	output = strings.TrimSpace(output)

	// Find JSON boundaries
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON object found in output")
	}
	output = output[start : end+1]

	// Parse the JSON
	var rawPlan struct {
		Summary     string                    `json:"summary"`
		Tasks       []orchestrator.PlannedTask `json:"tasks"`
		Insights    []string                  `json:"insights"`
		Constraints []string                  `json:"constraints"`
	}

	if err := json.Unmarshal([]byte(output), &rawPlan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w\nraw output: %s", err, output)
	}

	// Build the full PlanSpec
	plan := &orchestrator.PlanSpec{
		ID:              orchestrator.GenerateID(),
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

	// Calculate execution order using topological sort
	plan.ExecutionOrder = calculateExecutionOrder(plan.Tasks, plan.DependencyGraph)

	return plan, nil
}

// calculateExecutionOrder creates groups of tasks that can run in parallel
// This is a simplified version of orchestrator.calculateExecutionOrder
func calculateExecutionOrder(tasks []orchestrator.PlannedTask, _ map[string][]string) [][]string {
	// Build in-degree map
	inDegree := make(map[string]int)
	for _, task := range tasks {
		inDegree[task.ID] = len(task.DependsOn)
	}

	var groups [][]string
	completed := make(map[string]bool)

	for len(completed) < len(tasks) {
		var currentGroup []string

		// Find all tasks with in-degree 0
		for _, task := range tasks {
			if !completed[task.ID] && inDegree[task.ID] == 0 {
				currentGroup = append(currentGroup, task.ID)
			}
		}

		if len(currentGroup) == 0 {
			// Cycle detected or invalid graph - add remaining tasks
			for _, task := range tasks {
				if !completed[task.ID] {
					currentGroup = append(currentGroup, task.ID)
				}
			}
			groups = append(groups, currentGroup)
			break
		}

		groups = append(groups, currentGroup)

		// Mark as completed and update in-degrees
		for _, taskID := range currentGroup {
			completed[taskID] = true
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

// SavePlanToFile saves a plan to a JSON file
func SavePlanToFile(plan *orchestrator.PlanSpec, filePath string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}

	return nil
}

// CreateIssuesFromPlan creates GitHub issues from a plan
func CreateIssuesFromPlan(plan *orchestrator.PlanSpec, labels []string) (*IssueCreationResult, error) {
	result := &IssueCreationResult{
		SubIssueNumbers: make(map[string]int),
		SubIssueURLs:    make(map[string]string),
	}

	// First, create a placeholder parent issue (we'll update it after sub-issues are created)
	parentNum, parentURL, err := CreateIssue(IssueOptions{
		Title:  fmt.Sprintf("Plan: %s", truncateTitle(plan.Objective, 60)),
		Body:   fmt.Sprintf("## Summary\n\n%s\n\n*Creating sub-issues...*", plan.Summary),
		Labels: labels,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create parent issue: %w", err)
	}
	result.ParentIssueNumber = parentNum
	result.ParentIssueURL = parentURL

	fmt.Printf("Created parent issue: #%d\n", parentNum)

	// Create sub-issues in dependency order
	// We need to track created issues to resolve dependency references
	dependencyInfo := make(map[string]DependencyInfo)

	for groupIdx, group := range plan.ExecutionOrder {
		fmt.Printf("Creating issues for group %d...\n", groupIdx+1)

		for _, taskID := range group {
			// Find the task
			var task orchestrator.PlannedTask
			for _, t := range plan.Tasks {
				if t.ID == taskID {
					task = t
					break
				}
			}

			// Render sub-issue body
			body, err := RenderSubIssueBody(task, parentNum, dependencyInfo)
			if err != nil {
				return nil, fmt.Errorf("failed to render body for task %s: %w", taskID, err)
			}

			// Create the sub-issue
			subNum, subURL, err := CreateIssue(IssueOptions{
				Title:  task.Title,
				Body:   body,
				Labels: labels,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create sub-issue for task %s: %w", taskID, err)
			}

			result.SubIssueNumbers[taskID] = subNum
			result.SubIssueURLs[taskID] = subURL
			dependencyInfo[taskID] = DependencyInfo{
				IssueNumber: subNum,
				Title:       task.Title,
			}

			fmt.Printf("  Created sub-issue #%d: %s\n", subNum, task.Title)
		}
	}

	// Now update the parent issue with links to all sub-issues
	parentBody, err := RenderParentIssueBody(plan, result.SubIssueNumbers)
	if err != nil {
		return nil, fmt.Errorf("failed to render parent issue body: %w", err)
	}

	if err := UpdateIssueBody(parentNum, parentBody); err != nil {
		return nil, fmt.Errorf("failed to update parent issue: %w", err)
	}

	fmt.Printf("Updated parent issue #%d with sub-issue links\n", parentNum)

	return result, nil
}

func truncateTitle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

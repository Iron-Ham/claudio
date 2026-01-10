package plan

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// SubIssueData holds data for rendering a sub-issue body
type SubIssueData struct {
	Title             string
	Description       string
	Files             []string
	Complexity        string
	Dependencies      []DependencyInfo
	ParentIssueNumber int
}

// DependencyInfo holds info about a dependency for template rendering
type DependencyInfo struct {
	IssueNumber int
	Title       string
}

// GroupedTask holds task info with its issue number for parent template
type GroupedTask struct {
	TaskID      string
	Title       string
	IssueNumber int
}

// ParentIssueData holds data for rendering the parent issue body
type ParentIssueData struct {
	Objective    string
	Summary      string
	Insights     []string
	Constraints  []string
	GroupedTasks [][]GroupedTask // tasks grouped by execution order
}

const parentIssueBodyTemplate = `## Summary

{{.Summary}}

{{if .Insights}}## Analysis

{{range .Insights}}- {{.}}
{{end}}
{{end}}
{{if .Constraints}}## Constraints

{{range .Constraints}}- {{.}}
{{end}}
{{end}}
## Sub-Issues
{{range $groupIdx, $group := .GroupedTasks}}
### Group {{add $groupIdx 1}}{{if eq $groupIdx 0}} (can start immediately){{else}} (depends on previous groups){{end}}
{{range $group}}- [ ] #{{.IssueNumber}} - **{{.Title}}**
{{end}}
{{end}}
## Execution Order

Tasks are grouped by dependencies. All tasks within a group can be worked on in parallel.
Complete each group before starting the next.

## Acceptance Criteria

- [ ] All sub-issues completed
- [ ] Integration verified
`

const subIssueBodyTemplate = `## Task

{{.Description}}
{{if .Files}}
## Files to Modify

{{range .Files}}- ` + "`{{.}}`" + `
{{end}}{{end}}
{{if .Dependencies}}## Dependencies

Complete these issues first:
{{range .Dependencies}}- #{{.IssueNumber}} - {{.Title}}
{{end}}
{{end}}## Complexity

Estimated: **{{.Complexity}}**

---
*Part of #{{.ParentIssueNumber}}*
`

// Template helper functions
var templateFuncs = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
}

// RenderParentIssueBody renders the parent issue body from plan data
func RenderParentIssueBody(plan *orchestrator.PlanSpec, subIssueNumbers map[string]int) (string, error) {
	// Build grouped tasks from execution order
	groupedTasks := make([][]GroupedTask, len(plan.ExecutionOrder))
	for groupIdx, group := range plan.ExecutionOrder {
		groupedTasks[groupIdx] = make([]GroupedTask, 0, len(group))
		for _, taskID := range group {
			// Find the task
			for _, task := range plan.Tasks {
				if task.ID == taskID {
					groupedTasks[groupIdx] = append(groupedTasks[groupIdx], GroupedTask{
						TaskID:      taskID,
						Title:       task.Title,
						IssueNumber: subIssueNumbers[taskID],
					})
					break
				}
			}
		}
	}

	data := ParentIssueData{
		Objective:    plan.Objective,
		Summary:      plan.Summary,
		Insights:     plan.Insights,
		Constraints:  plan.Constraints,
		GroupedTasks: groupedTasks,
	}

	tmpl, err := template.New("parent-issue").Funcs(templateFuncs).Parse(parentIssueBodyTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse parent issue template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render parent issue template: %w", err)
	}

	return buf.String(), nil
}

// RenderSubIssueBody renders a sub-issue body from task data
func RenderSubIssueBody(task orchestrator.PlannedTask, parentIssueNumber int, dependencyInfo map[string]DependencyInfo) (string, error) {
	// Build dependencies list
	var deps []DependencyInfo
	for _, depID := range task.DependsOn {
		if info, ok := dependencyInfo[depID]; ok {
			deps = append(deps, info)
		}
	}

	data := SubIssueData{
		Title:             task.Title,
		Description:       task.Description,
		Files:             task.Files,
		Complexity:        string(task.EstComplexity),
		Dependencies:      deps,
		ParentIssueNumber: parentIssueNumber,
	}

	tmpl, err := template.New("sub-issue").Parse(subIssueBodyTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse sub-issue template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render sub-issue template: %w", err)
	}

	return buf.String(), nil
}

// FormatPlanForDisplay formats a plan for terminal display
func FormatPlanForDisplay(plan *orchestrator.PlanSpec) string {
	var sb strings.Builder

	sb.WriteString("Plan Summary\n")
	sb.WriteString("============\n\n")

	if plan.Summary != "" {
		sb.WriteString(plan.Summary)
		sb.WriteString("\n\n")
	}

	// Tasks by execution group
	sb.WriteString(fmt.Sprintf("Tasks: %d total in %d execution groups\n\n", len(plan.Tasks), len(plan.ExecutionOrder)))

	for groupIdx, group := range plan.ExecutionOrder {
		if groupIdx == 0 {
			sb.WriteString(fmt.Sprintf("Group %d (can start immediately):\n", groupIdx+1))
		} else {
			sb.WriteString(fmt.Sprintf("Group %d (depends on previous):\n", groupIdx+1))
		}

		for _, taskID := range group {
			for _, task := range plan.Tasks {
				if task.ID == taskID {
					sb.WriteString(fmt.Sprintf("  - [%s] %s (%s)\n", task.ID, task.Title, task.EstComplexity))
					if len(task.Files) > 0 {
						sb.WriteString(fmt.Sprintf("    Files: %s\n", strings.Join(task.Files, ", ")))
					}
					break
				}
			}
		}
		sb.WriteString("\n")
	}

	// Insights
	if len(plan.Insights) > 0 {
		sb.WriteString("Key Insights:\n")
		for _, insight := range plan.Insights {
			sb.WriteString(fmt.Sprintf("  - %s\n", insight))
		}
		sb.WriteString("\n")
	}

	// Constraints
	if len(plan.Constraints) > 0 {
		sb.WriteString("Constraints:\n")
		for _, constraint := range plan.Constraints {
			sb.WriteString(fmt.Sprintf("  - %s\n", constraint))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

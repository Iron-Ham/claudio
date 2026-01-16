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

// ParentChild represents either a group issue or a direct task in the parent
type ParentChild struct {
	Title       string
	IssueNumber int
	IsGroup     bool // true if this is a group issue, false if direct task
	TaskCount   int  // number of tasks in the group (only relevant for groups)
}

// ParentIssueData holds data for rendering the parent issue body
type ParentIssueData struct {
	Objective   string
	Summary     string
	Insights    []string
	Constraints []string
	Children    []ParentChild // hierarchical children (groups or direct tasks)
}

// GroupIssueData holds data for rendering a group issue body
type GroupIssueData struct {
	Summary           string
	DependsOnGroups   []int // group numbers this depends on
	Tasks             []GroupedTask
	ParentIssueNumber int
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

{{range .Children}}- [ ] #{{.IssueNumber}} - **{{.Title}}**{{if .IsGroup}} ({{.TaskCount}} tasks){{end}}
{{end}}
## Execution Order

Groups/tasks are ordered by dependencies. Complete each before starting those that depend on it.

## Acceptance Criteria

- [ ] All sub-issues completed
- [ ] Integration verified
`

const groupIssueBodyTemplate = `## Summary

{{.Summary}}

{{if gt (len .DependsOnGroups) 0}}## Dependencies

Complete these groups first:
{{range .DependsOnGroups}}- Group {{.}}
{{end}}
{{end}}
## Sub-Issues

{{range .Tasks}}- [ ] #{{.IssueNumber}} - **{{.Title}}**
{{end}}

## Acceptance Criteria

- [ ] All {{len .Tasks}} sub-issues completed

---
*Part of #{{.ParentIssueNumber}}*
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

// RenderParentIssueBodyHierarchical renders the parent issue body with hierarchical children
// Children can be either group issues (for groups with >1 task) or direct task issues (for single-task groups)
func RenderParentIssueBodyHierarchical(plan *orchestrator.PlanSpec, children []ParentChild) (string, error) {
	data := ParentIssueData{
		Objective:   plan.Objective,
		Summary:     plan.Summary,
		Insights:    plan.Insights,
		Constraints: plan.Constraints,
		Children:    children,
	}

	tmpl, err := template.New("parent-issue").Parse(parentIssueBodyTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse parent issue template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render parent issue template: %w", err)
	}

	return buf.String(), nil
}

// RenderGroupIssueBody renders a group issue body
func RenderGroupIssueBody(data GroupIssueData) (string, error) {
	// Validate required fields
	if len(data.Tasks) == 0 {
		return "", fmt.Errorf("group must have at least one task")
	}
	if data.ParentIssueNumber < 1 {
		return "", fmt.Errorf("invalid parent issue number: %d", data.ParentIssueNumber)
	}

	tmpl, err := template.New("group-issue").Parse(groupIssueBodyTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse group issue template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render group issue template: %w", err)
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

package plan

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestRenderSubIssueBody(t *testing.T) {
	task := orchestrator.PlannedTask{
		ID:            "task-1",
		Title:         "Test Task",
		Description:   "This is a test task description",
		Files:         []string{"main.go", "utils.go"},
		EstComplexity: "low",
		DependsOn:     []string{},
	}

	body, err := RenderSubIssueBody(task, 42, nil)
	if err != nil {
		t.Fatalf("RenderSubIssueBody() error = %v", err)
	}

	// Check required sections
	if !strings.Contains(body, "This is a test task description") {
		t.Error("Body missing description")
	}
	if !strings.Contains(body, "`main.go`") {
		t.Error("Body missing file")
	}
	if !strings.Contains(body, "**low**") {
		t.Error("Body missing complexity")
	}
	if !strings.Contains(body, "#42") {
		t.Error("Body missing parent issue reference")
	}
}

func TestRenderSubIssueBodyWithDependencies(t *testing.T) {
	task := orchestrator.PlannedTask{
		ID:            "task-2",
		Title:         "Dependent Task",
		Description:   "This task depends on another",
		Files:         []string{"api.go"},
		EstComplexity: "medium",
		DependsOn:     []string{"task-1"},
	}

	depInfo := map[string]DependencyInfo{
		"task-1": {IssueNumber: 100, Title: "First Task"},
	}

	body, err := RenderSubIssueBody(task, 42, depInfo)
	if err != nil {
		t.Fatalf("RenderSubIssueBody() error = %v", err)
	}

	if !strings.Contains(body, "Complete these issues first") {
		t.Error("Body missing dependency section")
	}
	if !strings.Contains(body, "#100") {
		t.Error("Body missing dependency issue number")
	}
	if !strings.Contains(body, "First Task") {
		t.Error("Body missing dependency title")
	}
}

func TestRenderParentIssueBody(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Objective: "Test objective",
		Summary:   "Test summary for the parent issue",
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "First Task"},
			{ID: "task-2", Title: "Second Task", DependsOn: []string{"task-1"}},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
		Insights:       []string{"Key insight about codebase"},
		Constraints:    []string{"Important constraint"},
	}

	subIssueNumbers := map[string]int{
		"task-1": 101,
		"task-2": 102,
	}

	body, err := RenderParentIssueBody(plan, subIssueNumbers)
	if err != nil {
		t.Fatalf("RenderParentIssueBody() error = %v", err)
	}

	// Check required sections
	if !strings.Contains(body, "Test summary for the parent issue") {
		t.Error("Body missing summary")
	}
	if !strings.Contains(body, "Key insight about codebase") {
		t.Error("Body missing insights")
	}
	if !strings.Contains(body, "Important constraint") {
		t.Error("Body missing constraints")
	}
	if !strings.Contains(body, "#101") || !strings.Contains(body, "#102") {
		t.Error("Body missing sub-issue references")
	}
	if !strings.Contains(body, "Group 1") {
		t.Error("Body missing execution groups")
	}
	if !strings.Contains(body, "can start immediately") {
		t.Error("Body missing group annotation")
	}
}

func TestRenderParentIssueBodyEmptySections(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Objective: "Test objective",
		Summary:   "Test summary",
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Only Task"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
		Insights:       nil,
		Constraints:    nil,
	}

	subIssueNumbers := map[string]int{
		"task-1": 101,
	}

	body, err := RenderParentIssueBody(plan, subIssueNumbers)
	if err != nil {
		t.Fatalf("RenderParentIssueBody() error = %v", err)
	}

	// Should still render without insights/constraints sections
	if !strings.Contains(body, "Test summary") {
		t.Error("Body missing summary")
	}
}

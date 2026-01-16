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

func TestRenderParentIssueBodyHierarchical(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Objective:   "Test objective",
		Summary:     "Test summary for hierarchical parent",
		Insights:    []string{"Insight 1"},
		Constraints: []string{"Constraint 1"},
	}

	children := []ParentChild{
		{Title: "Group 1: Foundation", IssueNumber: 101, IsGroup: true, TaskCount: 5},
		{Title: "Single Task", IssueNumber: 102, IsGroup: false, TaskCount: 1},
		{Title: "Group 2: Implementation", IssueNumber: 103, IsGroup: true, TaskCount: 3},
	}

	body, err := RenderParentIssueBodyHierarchical(plan, children)
	if err != nil {
		t.Fatalf("RenderParentIssueBodyHierarchical() error = %v", err)
	}

	// Check summary
	if !strings.Contains(body, "Test summary for hierarchical parent") {
		t.Error("Body missing summary")
	}

	// Check group references with task counts
	if !strings.Contains(body, "#101") {
		t.Error("Body missing group 1 reference")
	}
	if !strings.Contains(body, "(5 tasks)") {
		t.Error("Body missing group 1 task count")
	}

	// Check single task (no task count annotation)
	if !strings.Contains(body, "#102") {
		t.Error("Body missing single task reference")
	}
	if !strings.Contains(body, "Single Task") {
		t.Error("Body missing single task title")
	}

	// Check insights and constraints
	if !strings.Contains(body, "Insight 1") {
		t.Error("Body missing insights")
	}
	if !strings.Contains(body, "Constraint 1") {
		t.Error("Body missing constraints")
	}
}

func TestRenderParentIssueBodyHierarchicalEmptySections(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Objective:   "Test objective",
		Summary:     "Test summary",
		Insights:    nil,
		Constraints: nil,
	}

	children := []ParentChild{
		{Title: "Only Task", IssueNumber: 101, IsGroup: false, TaskCount: 1},
	}

	body, err := RenderParentIssueBodyHierarchical(plan, children)
	if err != nil {
		t.Fatalf("RenderParentIssueBodyHierarchical() error = %v", err)
	}

	// Should still render without insights/constraints sections
	if !strings.Contains(body, "Test summary") {
		t.Error("Body missing summary")
	}
	if !strings.Contains(body, "#101") {
		t.Error("Body missing child reference")
	}
}

func TestRenderGroupIssueBody(t *testing.T) {
	data := GroupIssueData{
		Summary:         "This group can start immediately.",
		DependsOnGroups: nil,
		Tasks: []GroupedTask{
			{TaskID: "task-1", Title: "First Task", IssueNumber: 201},
			{TaskID: "task-2", Title: "Second Task", IssueNumber: 202},
		},
		ParentIssueNumber: 100,
	}

	body, err := RenderGroupIssueBody(data)
	if err != nil {
		t.Fatalf("RenderGroupIssueBody() error = %v", err)
	}

	// Check summary
	if !strings.Contains(body, "This group can start immediately.") {
		t.Error("Body missing summary")
	}

	// Check task references
	if !strings.Contains(body, "#201") || !strings.Contains(body, "#202") {
		t.Error("Body missing task references")
	}
	if !strings.Contains(body, "First Task") || !strings.Contains(body, "Second Task") {
		t.Error("Body missing task titles")
	}

	// Check parent reference
	if !strings.Contains(body, "#100") {
		t.Error("Body missing parent reference")
	}

	// Check acceptance criteria
	if !strings.Contains(body, "All 2 sub-issues completed") {
		t.Error("Body missing acceptance criteria with task count")
	}
}

func TestRenderGroupIssueBodyWithDependencies(t *testing.T) {
	data := GroupIssueData{
		Summary:         "This group depends on previous groups.",
		DependsOnGroups: []int{1},
		Tasks: []GroupedTask{
			{TaskID: "task-3", Title: "Third Task", IssueNumber: 203},
		},
		ParentIssueNumber: 100,
	}

	body, err := RenderGroupIssueBody(data)
	if err != nil {
		t.Fatalf("RenderGroupIssueBody() error = %v", err)
	}

	// Check dependencies section
	if !strings.Contains(body, "Complete these groups first") {
		t.Error("Body missing dependencies section")
	}
	if !strings.Contains(body, "Group 1") {
		t.Error("Body missing dependency group reference")
	}
}

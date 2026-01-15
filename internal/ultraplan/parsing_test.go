package ultraplan

import (
	"os"
	"testing"
)

func TestParsePlanFromOutput(t *testing.T) {
	output := `Here is the plan:
<plan>
{
  "summary": "Test plan summary",
  "tasks": [
    {
      "id": "task-1",
      "title": "First task",
      "description": "Do the first thing",
      "files": ["file1.go"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "low"
    },
    {
      "id": "task-2",
      "title": "Second task",
      "description": "Do the second thing",
      "files": ["file2.go"],
      "depends_on": ["task-1"],
      "priority": 2,
      "est_complexity": "medium"
    }
  ],
  "insights": ["Found existing code patterns"],
  "constraints": ["Must maintain backward compatibility"]
}
</plan>
Done!`

	plan, err := ParsePlanFromOutput(output, "Test objective")
	if err != nil {
		t.Fatalf("ParsePlanFromOutput failed: %v", err)
	}

	if plan.Objective != "Test objective" {
		t.Errorf("Expected objective 'Test objective', got '%s'", plan.Objective)
	}

	if plan.Summary != "Test plan summary" {
		t.Errorf("Expected summary 'Test plan summary', got '%s'", plan.Summary)
	}

	if len(plan.Tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(plan.Tasks))
	}

	if plan.Tasks[0].ID != "task-1" {
		t.Errorf("Expected first task ID 'task-1', got '%s'", plan.Tasks[0].ID)
	}

	if len(plan.Tasks[1].DependsOn) != 1 || plan.Tasks[1].DependsOn[0] != "task-1" {
		t.Error("Task 2 should depend on task-1")
	}

	if len(plan.ExecutionOrder) == 0 {
		t.Error("ExecutionOrder should be calculated")
	}
}

func TestParsePlanFromOutput_NoPlanTag(t *testing.T) {
	output := "No plan here, just some text"

	_, err := ParsePlanFromOutput(output, "Test")
	if err == nil {
		t.Error("Expected error for missing plan tag")
	}
}

func TestParsePlanFromOutput_InvalidJSON(t *testing.T) {
	output := `<plan>not valid json</plan>`

	_, err := ParsePlanFromOutput(output, "Test")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParsePlanFromOutput_EmptyTasks(t *testing.T) {
	output := `<plan>{"summary": "Empty plan", "tasks": []}</plan>`

	_, err := ParsePlanFromOutput(output, "Test")
	if err == nil {
		t.Error("Expected error for empty tasks")
	}
}

func TestCalculateExecutionOrder(t *testing.T) {
	tasks := []PlannedTask{
		{ID: "task-1", DependsOn: []string{}, Priority: 1},
		{ID: "task-2", DependsOn: []string{}, Priority: 2},
		{ID: "task-3", DependsOn: []string{"task-1"}, Priority: 1},
		{ID: "task-4", DependsOn: []string{"task-1", "task-2"}, Priority: 1},
	}

	deps := make(map[string][]string)
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}

	order := CalculateExecutionOrder(tasks, deps)

	if len(order) != 2 {
		t.Fatalf("Expected 2 groups, got %d", len(order))
	}

	// First group should have task-1 and task-2 (no deps)
	group0 := make(map[string]bool)
	for _, id := range order[0] {
		group0[id] = true
	}
	if !group0["task-1"] || !group0["task-2"] {
		t.Error("First group should contain task-1 and task-2")
	}

	// Second group should have task-3 and task-4
	group1 := make(map[string]bool)
	for _, id := range order[1] {
		group1[id] = true
	}
	if !group1["task-3"] || !group1["task-4"] {
		t.Error("Second group should contain task-3 and task-4")
	}
}

func TestParseRevisionIssuesFromOutput(t *testing.T) {
	output := `Review complete.
<revision_issues>
[
  {
    "task_id": "task-1",
    "description": "Missing error handling",
    "files": ["main.go"],
    "severity": "major",
    "suggestion": "Add error checks"
  }
]
</revision_issues>
Done.`

	issues, err := ParseRevisionIssuesFromOutput(output)
	if err != nil {
		t.Fatalf("ParseRevisionIssuesFromOutput failed: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	if issues[0].TaskID != "task-1" {
		t.Errorf("Expected task ID 'task-1', got '%s'", issues[0].TaskID)
	}

	if issues[0].Severity != "major" {
		t.Errorf("Expected severity 'major', got '%s'", issues[0].Severity)
	}
}

func TestParseRevisionIssuesFromOutput_NoIssues(t *testing.T) {
	output := `Review complete. No issues found.`

	issues, err := ParseRevisionIssuesFromOutput(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(issues) > 0 {
		t.Error("Expected no issues when tag is missing")
	}
}

func TestParseRevisionIssuesFromOutput_EmptyArray(t *testing.T) {
	output := `<revision_issues>[]</revision_issues>`

	issues, err := ParseRevisionIssuesFromOutput(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(issues) != 0 {
		t.Errorf("Expected 0 issues, got %d", len(issues))
	}
}

func TestParsePlanDecisionFromOutput(t *testing.T) {
	output := `After reviewing all plans:
<plan_decision>
{
  "action": "select",
  "selected_index": 1,
  "reasoning": "Plan 2 has better parallelism",
  "plan_scores": [
    {"strategy": "strategy-1", "score": 7, "strengths": "Good", "weaknesses": "Complex"},
    {"strategy": "strategy-2", "score": 9, "strengths": "Simple", "weaknesses": "None"}
  ]
}
</plan_decision>`

	decision, err := ParsePlanDecisionFromOutput(output)
	if err != nil {
		t.Fatalf("ParsePlanDecisionFromOutput failed: %v", err)
	}

	if decision.Action != "select" {
		t.Errorf("Expected action 'select', got '%s'", decision.Action)
	}

	if decision.SelectedIndex != 1 {
		t.Errorf("Expected selected_index 1, got %d", decision.SelectedIndex)
	}

	if len(decision.PlanScores) != 2 {
		t.Errorf("Expected 2 plan scores, got %d", len(decision.PlanScores))
	}
}

func TestParsePlanDecisionFromOutput_Merge(t *testing.T) {
	output := `<plan_decision>
{
  "action": "merge",
  "selected_index": -1,
  "reasoning": "Combining best parts of all plans"
}
</plan_decision>`

	decision, err := ParsePlanDecisionFromOutput(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if decision.Action != "merge" {
		t.Errorf("Expected action 'merge', got '%s'", decision.Action)
	}

	if decision.SelectedIndex != -1 {
		t.Errorf("Expected selected_index -1, got %d", decision.SelectedIndex)
	}
}

func TestParsePlanDecisionFromOutput_InvalidAction(t *testing.T) {
	output := `<plan_decision>{"action": "invalid", "selected_index": 0}</plan_decision>`

	_, err := ParsePlanDecisionFromOutput(output)
	if err == nil {
		t.Error("Expected error for invalid action")
	}
}

func TestValidatePlanSimple(t *testing.T) {
	plan := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", DependsOn: []string{}},
			{ID: "task-2", DependsOn: []string{"task-1"}},
		},
		ExecutionOrder: [][]string{
			{"task-1"},
			{"task-2"},
		},
	}

	err := ValidatePlanSimple(plan)
	if err != nil {
		t.Errorf("Expected valid plan, got error: %v", err)
	}
}

func TestValidatePlanSimple_NilPlan(t *testing.T) {
	err := ValidatePlanSimple(nil)
	if err == nil {
		t.Error("Expected error for nil plan")
	}
}

func TestValidatePlanSimple_EmptyTasks(t *testing.T) {
	plan := &PlanSpec{Tasks: []PlannedTask{}}
	err := ValidatePlanSimple(plan)
	if err == nil {
		t.Error("Expected error for empty tasks")
	}
}

func TestValidatePlanSimple_InvalidDependency(t *testing.T) {
	plan := &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", DependsOn: []string{"nonexistent"}},
		},
	}

	err := ValidatePlanSimple(plan)
	if err == nil {
		t.Error("Expected error for invalid dependency")
	}
}

func TestParsePlanFromFile_RootLevelFormat(t *testing.T) {
	// Create a temp file with root-level JSON format
	tmpFile := t.TempDir() + "/plan.json"
	content := `{
  "summary": "Root level plan",
  "tasks": [
    {
      "id": "task-1",
      "title": "First task",
      "description": "Do first thing",
      "files": ["file1.go"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "low"
    }
  ],
  "insights": ["insight1"],
  "constraints": ["constraint1"]
}`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	plan, err := ParsePlanFromFile(tmpFile, "Test objective")
	if err != nil {
		t.Fatalf("ParsePlanFromFile failed: %v", err)
	}

	if plan.Summary != "Root level plan" {
		t.Errorf("Expected summary 'Root level plan', got '%s'", plan.Summary)
	}

	if len(plan.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(plan.Tasks))
	}

	if plan.Tasks[0].ID != "task-1" {
		t.Errorf("Expected task ID 'task-1', got '%s'", plan.Tasks[0].ID)
	}
}

func TestParsePlanFromFile_NestedPlanFormat(t *testing.T) {
	// Create a temp file with nested "plan" wrapper format
	tmpFile := t.TempDir() + "/plan.json"
	content := `{
  "plan": {
    "summary": "Nested format plan",
    "tasks": [
      {
        "id": "task-nested",
        "title": "Nested task",
        "description": "Do nested thing",
        "files": ["nested.go"],
        "depends_on": [],
        "priority": 1,
        "est_complexity": "medium"
      }
    ],
    "insights": ["nested insight"],
    "constraints": ["nested constraint"]
  }
}`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	plan, err := ParsePlanFromFile(tmpFile, "Test nested objective")
	if err != nil {
		t.Fatalf("ParsePlanFromFile failed for nested format: %v", err)
	}

	if plan.Summary != "Nested format plan" {
		t.Errorf("Expected summary 'Nested format plan', got '%s'", plan.Summary)
	}

	if len(plan.Tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(plan.Tasks))
	}

	if plan.Tasks[0].ID != "task-nested" {
		t.Errorf("Expected task ID 'task-nested', got '%s'", plan.Tasks[0].ID)
	}
}

func TestParsePlanFromFile_AlternativeFieldNames(t *testing.T) {
	// Test "depends" instead of "depends_on" and "complexity" instead of "est_complexity"
	tmpFile := t.TempDir() + "/plan.json"
	content := `{
  "plan": {
    "summary": "Alternative field names",
    "tasks": [
      {
        "id": "task-alt-1",
        "title": "Alt task 1",
        "description": "First alt task",
        "files": ["alt1.go"],
        "depends": [],
        "priority": 1,
        "complexity": "low"
      },
      {
        "id": "task-alt-2",
        "title": "Alt task 2",
        "description": "Second alt task",
        "files": ["alt2.go"],
        "depends": ["task-alt-1"],
        "priority": 2,
        "complexity": "high"
      }
    ],
    "insights": [],
    "constraints": []
  }
}`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	plan, err := ParsePlanFromFile(tmpFile, "Test alternative fields")
	if err != nil {
		t.Fatalf("ParsePlanFromFile failed for alternative field names: %v", err)
	}

	if len(plan.Tasks) != 2 {
		t.Fatalf("Expected 2 tasks, got %d", len(plan.Tasks))
	}

	// Check that "depends" was mapped to DependsOn
	if len(plan.Tasks[1].DependsOn) != 1 || plan.Tasks[1].DependsOn[0] != "task-alt-1" {
		t.Errorf("Expected task-alt-2 to depend on task-alt-1, got %v", plan.Tasks[1].DependsOn)
	}

	// Check that "complexity" was mapped to EstComplexity
	if plan.Tasks[0].EstComplexity != "low" {
		t.Errorf("Expected est_complexity 'low', got '%s'", plan.Tasks[0].EstComplexity)
	}
	if plan.Tasks[1].EstComplexity != "high" {
		t.Errorf("Expected est_complexity 'high', got '%s'", plan.Tasks[1].EstComplexity)
	}
}

func TestParsePlanFromFile_MixedFieldNames(t *testing.T) {
	// Test that standard field names take precedence over alternatives
	tmpFile := t.TempDir() + "/plan.json"
	content := `{
  "summary": "Mixed fields",
  "tasks": [
    {
      "id": "task-mix",
      "title": "Mixed task",
      "description": "Mixed field task",
      "files": [],
      "depends_on": ["dep-standard"],
      "depends": ["dep-alt"],
      "priority": 1,
      "est_complexity": "medium",
      "complexity": "low"
    }
  ]
}`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	plan, err := ParsePlanFromFile(tmpFile, "Test mixed fields")
	if err != nil {
		t.Fatalf("ParsePlanFromFile failed: %v", err)
	}

	// Standard field names should take precedence
	if len(plan.Tasks[0].DependsOn) != 1 || plan.Tasks[0].DependsOn[0] != "dep-standard" {
		t.Errorf("Expected depends_on to take precedence, got %v", plan.Tasks[0].DependsOn)
	}
	if plan.Tasks[0].EstComplexity != "medium" {
		t.Errorf("Expected est_complexity to take precedence, got '%s'", plan.Tasks[0].EstComplexity)
	}
}

func TestParsePlanFromFile_EmptyTasks(t *testing.T) {
	tmpFile := t.TempDir() + "/plan.json"
	content := `{"summary": "Empty plan", "tasks": []}`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParsePlanFromFile(tmpFile, "Test")
	if err == nil {
		t.Error("Expected error for empty tasks")
	}
}

func TestParsePlanFromFile_NestedEmptyTasks(t *testing.T) {
	tmpFile := t.TempDir() + "/plan.json"
	content := `{"plan": {"summary": "Empty nested plan", "tasks": []}}`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParsePlanFromFile(tmpFile, "Test")
	if err == nil {
		t.Error("Expected error for empty tasks in nested format")
	}
}

func TestParsePlanFromFile_InvalidJSON(t *testing.T) {
	tmpFile := t.TempDir() + "/plan.json"
	content := `not valid json at all`
	if err := writeTestFile(tmpFile, content); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := ParsePlanFromFile(tmpFile, "Test")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestParsePlanFromFile_FileNotFound(t *testing.T) {
	_, err := ParsePlanFromFile("/nonexistent/path/plan.json", "Test")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

// writeTestFile is a helper to write content to a file for testing
func writeTestFile(path, content string) error {
	return writeFile(path, []byte(content))
}

// writeFile wraps os.WriteFile for testing
var writeFile = func(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

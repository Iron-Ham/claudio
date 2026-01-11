package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParsePlanFromOutput(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		objective string
		wantErr   bool
		wantTasks int
	}{
		{
			name: "valid plan with tasks",
			output: `Here is my analysis of the codebase...

<plan>
{
  "summary": "Add OAuth2 authentication to the API",
  "tasks": [
    {
      "id": "task-1",
      "title": "Add OAuth2 dependencies",
      "description": "Add the necessary OAuth2 libraries to go.mod",
      "files": ["go.mod", "go.sum"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "low"
    },
    {
      "id": "task-2",
      "title": "Create auth models",
      "description": "Create the authentication data models",
      "files": ["internal/auth/models.go"],
      "depends_on": ["task-1"],
      "priority": 2,
      "est_complexity": "medium"
    }
  ],
  "insights": ["The codebase uses clean architecture"],
  "constraints": ["Must maintain backward compatibility"]
}
</plan>

That's my plan!`,
			objective: "Add OAuth2 authentication",
			wantErr:   false,
			wantTasks: 2,
		},
		{
			name:      "no plan tags",
			output:    "Here is my analysis but no plan tags",
			objective: "Test",
			wantErr:   true,
		},
		{
			name: "invalid JSON",
			output: `<plan>
{ this is not valid json }
</plan>`,
			objective: "Test",
			wantErr:   true,
		},
		{
			name: "empty tasks",
			output: `<plan>
{
  "summary": "Empty plan",
  "tasks": []
}
</plan>`,
			objective: "Test",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := ParsePlanFromOutput(tt.output, tt.objective)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePlanFromOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(plan.Tasks) != tt.wantTasks {
				t.Errorf("ParsePlanFromOutput() got %d tasks, want %d", len(plan.Tasks), tt.wantTasks)
			}
		})
	}
}

func TestCalculateExecutionOrder(t *testing.T) {
	tests := []struct {
		name       string
		tasks      []PlannedTask
		wantGroups int
	}{
		{
			name: "all independent tasks",
			tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{}},
				{ID: "task-3", DependsOn: []string{}},
			},
			wantGroups: 1, // All in one parallel group
		},
		{
			name: "linear dependencies",
			tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{"task-1"}},
				{ID: "task-3", DependsOn: []string{"task-2"}},
			},
			wantGroups: 3, // One task per group
		},
		{
			name: "diamond dependency",
			tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{"task-1"}},
				{ID: "task-3", DependsOn: []string{"task-1"}},
				{ID: "task-4", DependsOn: []string{"task-2", "task-3"}},
			},
			wantGroups: 3, // task-1 | task-2,task-3 | task-4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := make(map[string][]string)
			for _, task := range tt.tasks {
				deps[task.ID] = task.DependsOn
			}
			order := calculateExecutionOrder(tt.tasks, deps)
			if len(order) != tt.wantGroups {
				t.Errorf("calculateExecutionOrder() got %d groups, want %d", len(order), tt.wantGroups)
			}
		})
	}
}

func TestValidatePlan(t *testing.T) {
	tests := []struct {
		name    string
		plan    *PlanSpec
		wantErr bool
	}{
		{
			name:    "nil plan",
			plan:    nil,
			wantErr: true,
		},
		{
			name: "empty tasks",
			plan: &PlanSpec{
				Tasks: []PlannedTask{},
			},
			wantErr: true,
		},
		{
			name: "invalid dependency reference",
			plan: &PlanSpec{
				Tasks: []PlannedTask{
					{ID: "task-1", DependsOn: []string{"nonexistent"}},
				},
			},
			wantErr: true,
		},
		{
			name: "valid plan",
			plan: &PlanSpec{
				Tasks: []PlannedTask{
					{ID: "task-1", DependsOn: []string{}},
					{ID: "task-2", DependsOn: []string{"task-1"}},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlan(tt.plan)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePlan() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUltraPlanSession_IsTaskReady(t *testing.T) {
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{"task-1"}},
				{ID: "task-3", DependsOn: []string{"task-1", "task-2"}},
			},
		},
		CompletedTasks: []string{},
	}

	// task-1 should be ready (no dependencies)
	if !session.IsTaskReady("task-1") {
		t.Error("task-1 should be ready")
	}

	// task-2 should not be ready (depends on task-1)
	if session.IsTaskReady("task-2") {
		t.Error("task-2 should not be ready")
	}

	// Mark task-1 as completed
	session.CompletedTasks = []string{"task-1"}

	// Now task-2 should be ready
	if !session.IsTaskReady("task-2") {
		t.Error("task-2 should be ready after task-1 completes")
	}

	// task-3 still should not be ready (needs task-2 too)
	if session.IsTaskReady("task-3") {
		t.Error("task-3 should not be ready yet")
	}

	// Mark task-2 as completed
	session.CompletedTasks = []string{"task-1", "task-2"}

	// Now task-3 should be ready
	if !session.IsTaskReady("task-3") {
		t.Error("task-3 should be ready after task-1 and task-2 complete")
	}
}

func TestUltraPlanSession_Progress(t *testing.T) {
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1"},
				{ID: "task-2"},
				{ID: "task-3"},
				{ID: "task-4"},
			},
		},
		CompletedTasks: []string{},
	}

	// 0% progress
	if session.Progress() != 0 {
		t.Errorf("Progress() = %f, want 0", session.Progress())
	}

	// 25% progress
	session.CompletedTasks = []string{"task-1"}
	if session.Progress() != 25 {
		t.Errorf("Progress() = %f, want 25", session.Progress())
	}

	// 50% progress
	session.CompletedTasks = []string{"task-1", "task-2"}
	if session.Progress() != 50 {
		t.Errorf("Progress() = %f, want 50", session.Progress())
	}

	// 100% progress
	session.CompletedTasks = []string{"task-1", "task-2", "task-3", "task-4"}
	if session.Progress() != 100 {
		t.Errorf("Progress() = %f, want 100", session.Progress())
	}
}

func TestUltraPlanSession_GetReadyTasks(t *testing.T) {
	// Test with execution order groups - tasks should respect group boundaries
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{}},
				{ID: "task-3", DependsOn: []string{"task-1"}},
				{ID: "task-4", DependsOn: []string{"task-2"}},
			},
			// Group 1: task-1, task-2 (both independent)
			// Group 2: task-3, task-4 (depend on group 1)
			ExecutionOrder: [][]string{
				{"task-1", "task-2"},
				{"task-3", "task-4"},
			},
		},
		CompletedTasks: []string{},
		FailedTasks:    []string{},
		TaskToInstance: make(map[string]string),
		CurrentGroup:   0,
	}

	// Initially, only task-1 and task-2 (group 1) should be ready
	ready := session.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 2", len(ready))
	}

	// Start task-1
	session.TaskToInstance["task-1"] = "inst-1"
	ready = session.GetReadyTasks()
	if len(ready) != 1 { // Only task-2 should be ready (from group 1)
		t.Errorf("GetReadyTasks() returned %d tasks, want 1 (task-2 only)", len(ready))
	}

	// Complete task-1 but NOT task-2 - should NOT get group 2 tasks
	delete(session.TaskToInstance, "task-1")
	session.CompletedTasks = []string{"task-1"}
	ready = session.GetReadyTasks()
	// With group-aware logic, only task-2 is ready (still in group 1)
	if len(ready) != 1 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 1 (task-2 from current group)", len(ready))
	}
	if len(ready) > 0 && ready[0] != "task-2" {
		t.Errorf("GetReadyTasks() returned %s, want task-2", ready[0])
	}

	// Complete task-2, advancing group - now group 2 tasks should be ready
	session.CompletedTasks = []string{"task-1", "task-2"}
	session.CurrentGroup = 1 // Manually advance for this test (normally done by coordinator)
	ready = session.GetReadyTasks()
	if len(ready) != 2 { // task-3 and task-4 from group 2
		t.Errorf("GetReadyTasks() returned %d tasks, want 2 (task-3 and task-4)", len(ready))
	}
}

func TestUltraPlanSession_GetReadyTasks_NoExecutionOrder(t *testing.T) {
	// Test fallback behavior when no ExecutionOrder is defined (dependency-only)
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{}},
				{ID: "task-3", DependsOn: []string{"task-1"}},
			},
			// No ExecutionOrder defined - falls back to dependency-only logic
		},
		CompletedTasks: []string{},
		FailedTasks:    []string{},
		TaskToInstance: make(map[string]string),
	}

	// Initially, task-1 and task-2 should be ready (no dependencies)
	ready := session.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 2", len(ready))
	}

	// Complete task-1 - task-3 should become ready (dependency-only logic)
	session.CompletedTasks = []string{"task-1"}
	ready = session.GetReadyTasks()
	if len(ready) != 2 { // task-2 and task-3 should be ready
		t.Errorf("GetReadyTasks() returned %d tasks, want 2", len(ready))
	}
}

func TestUltraPlanSession_IsCurrentGroupComplete(t *testing.T) {
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{}},
				{ID: "task-3", DependsOn: []string{"task-1", "task-2"}},
			},
			ExecutionOrder: [][]string{
				{"task-1", "task-2"},
				{"task-3"},
			},
		},
		CompletedTasks: []string{},
		FailedTasks:    []string{},
		CurrentGroup:   0,
	}

	// Initially, group 0 is not complete
	if session.IsCurrentGroupComplete() {
		t.Error("IsCurrentGroupComplete() = true, want false (no tasks completed)")
	}

	// Complete only task-1 - group still not complete
	session.CompletedTasks = []string{"task-1"}
	if session.IsCurrentGroupComplete() {
		t.Error("IsCurrentGroupComplete() = true, want false (task-2 not complete)")
	}

	// Complete task-2 - now group 0 is complete
	session.CompletedTasks = []string{"task-1", "task-2"}
	if !session.IsCurrentGroupComplete() {
		t.Error("IsCurrentGroupComplete() = false, want true (both tasks complete)")
	}

	// Test that failed tasks also count as "done" for group completion
	session.CompletedTasks = []string{"task-1"}
	session.FailedTasks = []string{"task-2"}
	if !session.IsCurrentGroupComplete() {
		t.Error("IsCurrentGroupComplete() = false, want true (task-2 failed counts as done)")
	}
}

func TestUltraPlanSession_AdvanceGroupIfComplete(t *testing.T) {
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{"task-1"}},
			},
			ExecutionOrder: [][]string{
				{"task-1"},
				{"task-2"},
			},
		},
		CompletedTasks: []string{},
		CurrentGroup:   0,
	}

	// Group not complete - should not advance
	advanced, prevGroup := session.AdvanceGroupIfComplete()
	if advanced {
		t.Error("AdvanceGroupIfComplete() advanced when group not complete")
	}
	if prevGroup != 0 {
		t.Errorf("AdvanceGroupIfComplete() prevGroup = %d, want 0", prevGroup)
	}

	// Complete task-1 - should advance to group 1
	session.CompletedTasks = []string{"task-1"}
	advanced, prevGroup = session.AdvanceGroupIfComplete()
	if !advanced {
		t.Error("AdvanceGroupIfComplete() did not advance when group complete")
	}
	if prevGroup != 0 {
		t.Errorf("AdvanceGroupIfComplete() prevGroup = %d, want 0", prevGroup)
	}
	if session.CurrentGroup != 1 {
		t.Errorf("CurrentGroup = %d, want 1", session.CurrentGroup)
	}

	// Complete task-2 - should advance past last group
	session.CompletedTasks = []string{"task-1", "task-2"}
	advanced, _ = session.AdvanceGroupIfComplete()
	if !advanced {
		t.Error("AdvanceGroupIfComplete() did not advance when final group complete")
	}
	if session.CurrentGroup != 2 {
		t.Errorf("CurrentGroup = %d, want 2 (past last group)", session.CurrentGroup)
	}
}

func TestUltraPlanSession_GetReadyTasks_AwaitingDecision(t *testing.T) {
	// Test that GetReadyTasks returns nil when awaiting a decision about partial failure.
	// This is critical to prevent starting the next group's tasks before consolidation.
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{}},
				// Group 1 tasks only depend on task-1, not task-2
				// This simulates a scenario where we can continue with partial work
				{ID: "task-3", DependsOn: []string{"task-1"}},
				{ID: "task-4", DependsOn: []string{"task-1"}},
			},
			ExecutionOrder: [][]string{
				{"task-1", "task-2"}, // Group 0
				{"task-3", "task-4"}, // Group 1
			},
		},
		CompletedTasks: []string{"task-1"}, // task-1 succeeded
		FailedTasks:    []string{"task-2"}, // task-2 failed - partial failure
		TaskToInstance: make(map[string]string),
		CurrentGroup:   0, // Still at group 0 (not advanced)
		GroupDecision: &GroupDecisionState{
			GroupIndex:       0,
			SucceededTasks:   []string{"task-1"},
			FailedTasks:      []string{"task-2"},
			AwaitingDecision: true, // User must decide what to do
		},
	}

	// With AwaitingDecision = true, GetReadyTasks MUST return nil
	// This prevents the next group from starting before consolidation
	ready := session.GetReadyTasks()
	if len(ready) > 0 {
		t.Errorf("GetReadyTasks() = %v, want empty when AwaitingDecision is true", ready)
	}

	// Clear the decision state (simulating user chose to continue with partial work)
	session.GroupDecision = nil
	session.CurrentGroup = 1 // Advance to group 1 after consolidation

	// Now group 1 tasks should be ready (they only depend on task-1 which succeeded)
	ready = session.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 2 (task-3, task-4)", len(ready))
	}
}

func TestFlexibleString_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected string
	}{
		{
			name:     "string value",
			json:     `{"notes": "simple note"}`,
			expected: "simple note",
		},
		{
			name:     "array of strings",
			json:     `{"notes": ["note 1", "note 2", "note 3"]}`,
			expected: "note 1\nnote 2\nnote 3",
		},
		{
			name:     "empty string",
			json:     `{"notes": ""}`,
			expected: "",
		},
		{
			name:     "empty array",
			json:     `{"notes": []}`,
			expected: "",
		},
		{
			name:     "null value",
			json:     `{"notes": null}`,
			expected: "",
		},
		{
			name:     "missing field",
			json:     `{}`,
			expected: "",
		},
		{
			name:     "single element array",
			json:     `{"notes": ["only one"]}`,
			expected: "only one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result struct {
				Notes FlexibleString `json:"notes"`
			}
			if err := json.Unmarshal([]byte(tt.json), &result); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if string(result.Notes) != tt.expected {
				t.Errorf("Notes = %q, want %q", result.Notes, tt.expected)
			}
		})
	}
}

func TestTaskCompletionFile_ParseWithArrayNotes(t *testing.T) {
	// This tests the real-world scenario where Claude writes notes as an array
	jsonData := `{
		"task_id": "task-12-tests",
		"status": "complete",
		"summary": "Added tests",
		"files_modified": ["test.go"],
		"notes": ["First note about tests", "Second note about implementation"]
	}`

	var completion TaskCompletionFile
	if err := json.Unmarshal([]byte(jsonData), &completion); err != nil {
		t.Fatalf("Failed to parse completion file with array notes: %v", err)
	}

	if completion.TaskID != "task-12-tests" {
		t.Errorf("TaskID = %q, want %q", completion.TaskID, "task-12-tests")
	}
	if completion.Status != "complete" {
		t.Errorf("Status = %q, want %q", completion.Status, "complete")
	}
	expectedNotes := "First note about tests\nSecond note about implementation"
	if string(completion.Notes) != expectedNotes {
		t.Errorf("Notes = %q, want %q", completion.Notes, expectedNotes)
	}
}

func TestParseTaskCompletionFile_RealWorldArrayNotes(t *testing.T) {
	// Test parsing a real completion file that has notes as an array
	// This is a regression test for the bug where notes as array caused parse failure
	worktreePath := "/Users/zer0/Developer/oss/Claudio/.claudio/worktrees/173e1fdc"

	completion, err := ParseTaskCompletionFile(worktreePath)
	if err != nil {
		t.Skipf("Skipping real file test - file not found: %v", err)
		return
	}

	if completion.TaskID != "task-12-tests" {
		t.Errorf("TaskID = %q, want %q", completion.TaskID, "task-12-tests")
	}
	if completion.Status != "complete" {
		t.Errorf("Status = %q, want %q", completion.Status, "complete")
	}
	if completion.Status == "" {
		t.Error("Status is empty - checkForTaskCompletionFile would return false!")
	}

	// Notes should be non-empty since the real file has notes as an array
	if completion.Notes == "" {
		t.Error("Notes is empty - expected array of notes to be joined")
	}
	t.Logf("Notes length: %d bytes", len(completion.Notes))
}

func TestParsePlanDecisionFromOutput(t *testing.T) {
	tests := []struct {
		name              string
		output            string
		wantErr           bool
		wantErrContains   string
		wantAction        string
		wantSelectedIndex int
	}{
		{
			name: "valid select action - index 0",
			output: `Here is my analysis of the three plans...

<plan_decision>
{
  "action": "select",
  "selected_index": 0,
  "reasoning": "Plan 0 has the best task organization and clearest dependencies",
  "plan_scores": [
    {"strategy": "bottom-up", "score": 85, "strengths": "Good modularity", "weaknesses": "None"},
    {"strategy": "top-down", "score": 70, "strengths": "Clear flow", "weaknesses": "Too complex"},
    {"strategy": "risk-first", "score": 65, "strengths": "Safe", "weaknesses": "Slow"}
  ]
}
</plan_decision>

That concludes my evaluation.`,
			wantErr:           false,
			wantAction:        "select",
			wantSelectedIndex: 0,
		},
		{
			name: "valid select action - index 2",
			output: `<plan_decision>
{
  "action": "select",
  "selected_index": 2,
  "reasoning": "Risk-first approach is best for this critical system",
  "plan_scores": [
    {"strategy": "bottom-up", "score": 60, "strengths": "Fast", "weaknesses": "Risky"},
    {"strategy": "top-down", "score": 70, "strengths": "Structured", "weaknesses": "Slow"},
    {"strategy": "risk-first", "score": 90, "strengths": "Safe and thorough", "weaknesses": "None"}
  ]
}
</plan_decision>`,
			wantErr:           false,
			wantAction:        "select",
			wantSelectedIndex: 2,
		},
		{
			name: "valid merge action",
			output: `After careful analysis...

<plan_decision>
{
  "action": "merge",
  "selected_index": -1,
  "reasoning": "Each plan has unique strengths that should be combined",
  "plan_scores": [
    {"strategy": "bottom-up", "score": 75, "strengths": "Good foundation tasks", "weaknesses": "Missing tests"},
    {"strategy": "top-down", "score": 75, "strengths": "Clear integration tests", "weaknesses": "Missing setup"},
    {"strategy": "risk-first", "score": 75, "strengths": "Good error handling", "weaknesses": "Missing features"}
  ]
}
</plan_decision>`,
			wantErr:           false,
			wantAction:        "merge",
			wantSelectedIndex: -1,
		},
		{
			name:            "missing decision block",
			output:          "Here is my analysis of the three plans. They are all good.",
			wantErr:         true,
			wantErrContains: "no plan decision found",
		},
		{
			name:            "missing closing tag",
			output:          "<plan_decision>{\"action\": \"select\", \"selected_index\": 0}",
			wantErr:         true,
			wantErrContains: "no plan decision found",
		},
		{
			name: "empty decision block",
			output: `<plan_decision>
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "empty plan decision block",
		},
		{
			name: "whitespace-only decision block",
			output: `<plan_decision>

</plan_decision>`,
			wantErr:         true,
			wantErrContains: "empty plan decision block",
		},
		{
			name: "invalid action",
			output: `<plan_decision>
{
  "action": "combine",
  "selected_index": 0,
  "reasoning": "Test"
}
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "invalid plan decision action",
		},
		{
			name: "select with invalid index - negative",
			output: `<plan_decision>
{
  "action": "select",
  "selected_index": -1,
  "reasoning": "Test"
}
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "invalid selected_index for select action",
		},
		{
			name: "select with invalid index - too high",
			output: `<plan_decision>
{
  "action": "select",
  "selected_index": 5,
  "reasoning": "Test"
}
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "invalid selected_index for select action",
		},
		{
			name: "merge with non-negative-one index",
			output: `<plan_decision>
{
  "action": "merge",
  "selected_index": 0,
  "reasoning": "Test"
}
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "selected_index should be -1 for merge action",
		},
		{
			name: "malformed JSON",
			output: `<plan_decision>
{ this is not valid json }
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "failed to parse plan decision JSON",
		},
		{
			name: "incomplete JSON",
			output: `<plan_decision>
{"action": "select", "selected_index":
</plan_decision>`,
			wantErr:         true,
			wantErrContains: "failed to parse plan decision JSON",
		},
		{
			name: "valid decision with extra whitespace in tags",
			output: `<plan_decision>
{
  "action": "select",
  "selected_index": 1,
  "reasoning": "Middle plan is best"
}
   </plan_decision>`,
			wantErr:           false,
			wantAction:        "select",
			wantSelectedIndex: 1,
		},
		{
			name:              "valid decision with minimal fields",
			output:            `<plan_decision>{"action":"select","selected_index":0,"reasoning":"","plan_scores":[]}</plan_decision>`,
			wantErr:           false,
			wantAction:        "select",
			wantSelectedIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := ParsePlanDecisionFromOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePlanDecisionFromOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.wantErrContains != "" && err != nil {
					if !strings.Contains(err.Error(), tt.wantErrContains) {
						t.Errorf("ParsePlanDecisionFromOutput() error = %q, want error containing %q", err.Error(), tt.wantErrContains)
					}
				}
				return
			}
			if decision.Action != tt.wantAction {
				t.Errorf("ParsePlanDecisionFromOutput() action = %q, want %q", decision.Action, tt.wantAction)
			}
			if decision.SelectedIndex != tt.wantSelectedIndex {
				t.Errorf("ParsePlanDecisionFromOutput() selectedIndex = %d, want %d", decision.SelectedIndex, tt.wantSelectedIndex)
			}
		})
	}
}

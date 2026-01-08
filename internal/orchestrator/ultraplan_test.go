package orchestrator

import (
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
	session := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{}},
				{ID: "task-3", DependsOn: []string{"task-1"}},
				{ID: "task-4", DependsOn: []string{"task-2"}},
			},
		},
		CompletedTasks: []string{},
		FailedTasks:    []string{},
		TaskToInstance: make(map[string]string),
	}

	// Initially, task-1 and task-2 should be ready
	ready := session.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("GetReadyTasks() returned %d tasks, want 2", len(ready))
	}

	// Start task-1
	session.TaskToInstance["task-1"] = "inst-1"
	ready = session.GetReadyTasks()
	if len(ready) != 1 { // Only task-2 should be ready now
		t.Errorf("GetReadyTasks() returned %d tasks, want 1 (task-2 only)", len(ready))
	}

	// Complete task-1
	delete(session.TaskToInstance, "task-1")
	session.CompletedTasks = []string{"task-1"}
	ready = session.GetReadyTasks()
	if len(ready) != 2 { // task-2 and task-3 should be ready
		t.Errorf("GetReadyTasks() returned %d tasks, want 2", len(ready))
	}
}

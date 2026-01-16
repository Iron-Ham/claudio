package prompt

import (
	"reflect"
	"testing"
)

// mockPlannedTask implements PlannedTaskLike for testing
type mockPlannedTask struct {
	id            string
	title         string
	description   string
	files         []string
	dependsOn     []string
	priority      int
	estComplexity string
}

func (m mockPlannedTask) GetID() string            { return m.id }
func (m mockPlannedTask) GetTitle() string         { return m.title }
func (m mockPlannedTask) GetDescription() string   { return m.description }
func (m mockPlannedTask) GetFiles() []string       { return m.files }
func (m mockPlannedTask) GetDependsOn() []string   { return m.dependsOn }
func (m mockPlannedTask) GetPriority() int         { return m.priority }
func (m mockPlannedTask) GetEstComplexity() string { return m.estComplexity }

// mockPlanSpec implements PlanSpecLike for testing
type mockPlanSpec struct {
	summary        string
	tasks          []PlannedTaskLike
	executionOrder [][]string
	insights       []string
	constraints    []string
}

func (m mockPlanSpec) GetSummary() string            { return m.summary }
func (m mockPlanSpec) GetTasks() []PlannedTaskLike   { return m.tasks }
func (m mockPlanSpec) GetExecutionOrder() [][]string { return m.executionOrder }
func (m mockPlanSpec) GetInsights() []string         { return m.insights }
func (m mockPlanSpec) GetConstraints() []string      { return m.constraints }

// mockGroupConsolidation implements GroupConsolidationLike for testing
type mockGroupConsolidation struct {
	notes               string
	issuesForNextGroup  []string
	verificationSuccess bool
}

func (m mockGroupConsolidation) GetNotes() string                { return m.notes }
func (m mockGroupConsolidation) GetIssuesForNextGroup() []string { return m.issuesForNextGroup }
func (m mockGroupConsolidation) IsVerificationSuccess() bool     { return m.verificationSuccess }

func TestConvertPlannedTaskToTaskInfo(t *testing.T) {
	tests := []struct {
		name     string
		task     PlannedTaskLike
		expected TaskInfo
	}{
		{
			name:     "nil task returns zero value",
			task:     nil,
			expected: TaskInfo{},
		},
		{
			name: "basic task conversion",
			task: mockPlannedTask{
				id:            "task-1",
				title:         "Implement feature",
				description:   "Detailed description here",
				files:         []string{"file1.go", "file2.go"},
				dependsOn:     []string{"task-0"},
				priority:      1,
				estComplexity: "medium",
			},
			expected: TaskInfo{
				ID:            "task-1",
				Title:         "Implement feature",
				Description:   "Detailed description here",
				Files:         []string{"file1.go", "file2.go"},
				DependsOn:     []string{"task-0"},
				Priority:      1,
				EstComplexity: "medium",
			},
		},
		{
			name: "task with empty slices",
			task: mockPlannedTask{
				id:            "task-2",
				title:         "Independent task",
				description:   "No dependencies",
				files:         []string{},
				dependsOn:     []string{},
				priority:      0,
				estComplexity: "low",
			},
			expected: TaskInfo{
				ID:            "task-2",
				Title:         "Independent task",
				Description:   "No dependencies",
				Files:         []string{},
				DependsOn:     []string{},
				Priority:      0,
				EstComplexity: "low",
			},
		},
		{
			name: "task with nil slices",
			task: mockPlannedTask{
				id:            "task-3",
				title:         "Minimal task",
				description:   "Minimal",
				files:         nil,
				dependsOn:     nil,
				priority:      0,
				estComplexity: "low",
			},
			expected: TaskInfo{
				ID:            "task-3",
				Title:         "Minimal task",
				Description:   "Minimal",
				Files:         nil,
				DependsOn:     nil,
				Priority:      0,
				EstComplexity: "low",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertPlannedTaskToTaskInfo(tt.task)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertPlannedTaskToTaskInfo() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestConvertPlannedTaskToTaskInfo_DefensiveCopy(t *testing.T) {
	originalFiles := []string{"file1.go", "file2.go"}
	originalDeps := []string{"task-0"}

	task := mockPlannedTask{
		id:        "task-1",
		files:     originalFiles,
		dependsOn: originalDeps,
	}

	result := ConvertPlannedTaskToTaskInfo(task)

	// Modify the original slices
	originalFiles[0] = "modified.go"
	originalDeps[0] = "modified-task"

	// Result should not be affected
	if result.Files[0] == "modified.go" {
		t.Error("ConvertPlannedTaskToTaskInfo did not create defensive copy of Files")
	}
	if result.DependsOn[0] == "modified-task" {
		t.Error("ConvertPlannedTaskToTaskInfo did not create defensive copy of DependsOn")
	}
}

func TestConvertPlanSpecToPlanInfo(t *testing.T) {
	tests := []struct {
		name     string
		plan     PlanSpecLike
		expected *PlanInfo
	}{
		{
			name:     "nil plan returns nil",
			plan:     nil,
			expected: nil,
		},
		{
			name: "basic plan conversion",
			plan: mockPlanSpec{
				summary: "Plan summary",
				tasks: []PlannedTaskLike{
					mockPlannedTask{
						id:            "task-1",
						title:         "First task",
						description:   "Do first thing",
						estComplexity: "low",
					},
					mockPlannedTask{
						id:            "task-2",
						title:         "Second task",
						description:   "Do second thing",
						dependsOn:     []string{"task-1"},
						estComplexity: "medium",
					},
				},
				executionOrder: [][]string{{"task-1"}, {"task-2"}},
				insights:       []string{"Found existing pattern"},
				constraints:    []string{"Must maintain backwards compatibility"},
			},
			expected: &PlanInfo{
				Summary: "Plan summary",
				Tasks: []TaskInfo{
					{
						ID:            "task-1",
						Title:         "First task",
						Description:   "Do first thing",
						EstComplexity: "low",
					},
					{
						ID:            "task-2",
						Title:         "Second task",
						Description:   "Do second thing",
						DependsOn:     []string{"task-1"},
						EstComplexity: "medium",
					},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"Found existing pattern"},
				Constraints:    []string{"Must maintain backwards compatibility"},
			},
		},
		{
			name: "plan with no tasks",
			plan: mockPlanSpec{
				summary:        "Empty plan",
				tasks:          []PlannedTaskLike{},
				executionOrder: nil,
				insights:       nil,
				constraints:    nil,
			},
			expected: &PlanInfo{
				Summary:        "Empty plan",
				Tasks:          []TaskInfo{},
				ExecutionOrder: nil,
				Insights:       nil,
				Constraints:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertPlanSpecToPlanInfo(tt.plan)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertPlanSpecToPlanInfo() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestConvertPlanSpecToPlanInfo_DefensiveCopy(t *testing.T) {
	originalExecutionOrder := [][]string{{"task-1", "task-2"}, {"task-3"}}
	originalInsights := []string{"insight-1"}
	originalConstraints := []string{"constraint-1"}

	plan := mockPlanSpec{
		summary:        "Test plan",
		tasks:          []PlannedTaskLike{},
		executionOrder: originalExecutionOrder,
		insights:       originalInsights,
		constraints:    originalConstraints,
	}

	result := ConvertPlanSpecToPlanInfo(plan)

	// Modify the originals
	originalExecutionOrder[0][0] = "modified"
	originalInsights[0] = "modified"
	originalConstraints[0] = "modified"

	// Result should not be affected
	if result.ExecutionOrder[0][0] == "modified" {
		t.Error("ConvertPlanSpecToPlanInfo did not create defensive copy of ExecutionOrder")
	}
	if result.Insights[0] == "modified" {
		t.Error("ConvertPlanSpecToPlanInfo did not create defensive copy of Insights")
	}
	if result.Constraints[0] == "modified" {
		t.Error("ConvertPlanSpecToPlanInfo did not create defensive copy of Constraints")
	}
}

func TestConvertPlanSpecsToCandidatePlans(t *testing.T) {
	tests := []struct {
		name       string
		plans      []PlanSpecLike
		strategies []string
		expected   []CandidatePlanInfo
	}{
		{
			name:       "nil plans returns nil",
			plans:      nil,
			strategies: []string{"strategy-1"},
			expected:   nil,
		},
		{
			name:       "empty plans returns nil",
			plans:      []PlanSpecLike{},
			strategies: []string{"strategy-1"},
			expected:   nil,
		},
		{
			name: "single plan with strategy",
			plans: []PlanSpecLike{
				mockPlanSpec{
					summary: "Plan A summary",
					tasks: []PlannedTaskLike{
						mockPlannedTask{id: "task-1", title: "Task 1"},
					},
					executionOrder: [][]string{{"task-1"}},
					insights:       []string{"insight"},
					constraints:    []string{"constraint"},
				},
			},
			strategies: []string{"maximize-parallelism"},
			expected: []CandidatePlanInfo{
				{
					Strategy: "maximize-parallelism",
					Summary:  "Plan A summary",
					Tasks: []TaskInfo{
						{ID: "task-1", Title: "Task 1"},
					},
					ExecutionOrder: [][]string{{"task-1"}},
					Insights:       []string{"insight"},
					Constraints:    []string{"constraint"},
				},
			},
		},
		{
			name: "multiple plans with strategies",
			plans: []PlanSpecLike{
				mockPlanSpec{summary: "Plan A"},
				mockPlanSpec{summary: "Plan B"},
				mockPlanSpec{summary: "Plan C"},
			},
			strategies: []string{"maximize-parallelism", "minimize-complexity", "balanced-approach"},
			expected: []CandidatePlanInfo{
				{Strategy: "maximize-parallelism", Summary: "Plan A", Tasks: []TaskInfo{}},
				{Strategy: "minimize-complexity", Summary: "Plan B", Tasks: []TaskInfo{}},
				{Strategy: "balanced-approach", Summary: "Plan C", Tasks: []TaskInfo{}},
			},
		},
		{
			name: "more plans than strategies",
			plans: []PlanSpecLike{
				mockPlanSpec{summary: "Plan A"},
				mockPlanSpec{summary: "Plan B"},
				mockPlanSpec{summary: "Plan C"},
			},
			strategies: []string{"strategy-1"},
			expected: []CandidatePlanInfo{
				{Strategy: "strategy-1", Summary: "Plan A", Tasks: []TaskInfo{}},
				{Strategy: "", Summary: "Plan B", Tasks: []TaskInfo{}},
				{Strategy: "", Summary: "Plan C", Tasks: []TaskInfo{}},
			},
		},
		{
			name: "nil plan in slice",
			plans: []PlanSpecLike{
				mockPlanSpec{summary: "Plan A"},
				nil,
				mockPlanSpec{summary: "Plan C"},
			},
			strategies: []string{"s1", "s2", "s3"},
			expected: []CandidatePlanInfo{
				{Strategy: "s1", Summary: "Plan A", Tasks: []TaskInfo{}},
				{Strategy: "s2"},
				{Strategy: "s3", Summary: "Plan C", Tasks: []TaskInfo{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertPlanSpecsToCandidatePlans(tt.plans, tt.strategies)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertPlanSpecsToCandidatePlans() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestConvertGroupConsolidationToGroupContext(t *testing.T) {
	tests := []struct {
		name       string
		gc         GroupConsolidationLike
		groupIndex int
		expected   *GroupContext
	}{
		{
			name:       "nil consolidation returns nil",
			gc:         nil,
			groupIndex: 0,
			expected:   nil,
		},
		{
			name: "basic consolidation conversion",
			gc: mockGroupConsolidation{
				notes:               "Merged successfully",
				issuesForNextGroup:  []string{"Watch for file X changes"},
				verificationSuccess: true,
			},
			groupIndex: 2,
			expected: &GroupContext{
				GroupIndex:         2,
				Notes:              "Merged successfully",
				IssuesForNextGroup: []string{"Watch for file X changes"},
				VerificationPassed: true,
			},
		},
		{
			name: "consolidation with failed verification",
			gc: mockGroupConsolidation{
				notes:               "Build failed",
				issuesForNextGroup:  []string{"Fix build errors"},
				verificationSuccess: false,
			},
			groupIndex: 0,
			expected: &GroupContext{
				GroupIndex:         0,
				Notes:              "Build failed",
				IssuesForNextGroup: []string{"Fix build errors"},
				VerificationPassed: false,
			},
		},
		{
			name: "consolidation with nil issues",
			gc: mockGroupConsolidation{
				notes:               "Clean merge",
				issuesForNextGroup:  nil,
				verificationSuccess: true,
			},
			groupIndex: 1,
			expected: &GroupContext{
				GroupIndex:         1,
				Notes:              "Clean merge",
				IssuesForNextGroup: nil,
				VerificationPassed: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConvertGroupConsolidationToGroupContext(tt.gc, tt.groupIndex)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ConvertGroupConsolidationToGroupContext() = %+v, want %+v", got, tt.expected)
			}
		})
	}
}

func TestConvertGroupConsolidationToGroupContext_DefensiveCopy(t *testing.T) {
	originalIssues := []string{"issue-1", "issue-2"}

	gc := mockGroupConsolidation{
		notes:               "Notes",
		issuesForNextGroup:  originalIssues,
		verificationSuccess: true,
	}

	result := ConvertGroupConsolidationToGroupContext(gc, 0)

	// Modify the original
	originalIssues[0] = "modified"

	// Result should not be affected
	if result.IssuesForNextGroup[0] == "modified" {
		t.Error("ConvertGroupConsolidationToGroupContext did not create defensive copy of IssuesForNextGroup")
	}
}

func TestCopyStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "nil slice returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice returns empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "populated slice returns copy",
			input:    []string{"a", "b", "c"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copyStringSlice(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("copyStringSlice() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCopyStringSlice_Independence(t *testing.T) {
	original := []string{"a", "b"}
	copied := copyStringSlice(original)

	// Modify original
	original[0] = "modified"

	// Copy should be unchanged
	if copied[0] == "modified" {
		t.Error("copyStringSlice did not create independent copy")
	}
}

func TestCopyStringSliceSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    [][]string
		expected [][]string
	}{
		{
			name:     "nil slice returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice returns empty slice",
			input:    [][]string{},
			expected: [][]string{},
		},
		{
			name:     "populated slice returns copy",
			input:    [][]string{{"a", "b"}, {"c"}},
			expected: [][]string{{"a", "b"}, {"c"}},
		},
		{
			name:     "slice with nil inner slice",
			input:    [][]string{{"a"}, nil, {"b"}},
			expected: [][]string{{"a"}, nil, {"b"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copyStringSliceSlice(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("copyStringSliceSlice() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCopyStringSliceSlice_Independence(t *testing.T) {
	original := [][]string{{"a", "b"}, {"c"}}
	copied := copyStringSliceSlice(original)

	// Modify original at both levels
	original[0][0] = "modified"
	original[1] = []string{"replaced"}

	// Copy should be unchanged
	if copied[0][0] == "modified" {
		t.Error("copyStringSliceSlice did not create independent copy of inner slice")
	}
	if len(copied[1]) != 1 || copied[1][0] != "c" {
		t.Error("copyStringSliceSlice did not create independent copy of outer slice")
	}
}

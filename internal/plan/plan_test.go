package plan

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestParseIssueNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "standard github url",
			input:   "https://github.com/owner/repo/issues/123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "url with trailing newline",
			input:   "https://github.com/owner/repo/issues/456\n",
			want:    456,
			wantErr: false,
		},
		{
			name:    "url with extra path",
			input:   "https://github.com/owner/repo/issues/789/comments",
			want:    789,
			wantErr: false,
		},
		{
			name:    "invalid url - no issues path",
			input:   "https://github.com/owner/repo/pull/123",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid url - no number",
			input:   "https://github.com/owner/repo/issues/",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIssueNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseIssueNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateExecutionOrder(t *testing.T) {
	tests := []struct {
		name  string
		tasks []orchestrator.PlannedTask
		want  int // number of groups expected
	}{
		{
			name: "no dependencies - single group",
			tasks: []orchestrator.PlannedTask{
				{ID: "task-1"},
				{ID: "task-2"},
				{ID: "task-3"},
			},
			want: 1,
		},
		{
			name: "linear dependencies - 3 groups",
			tasks: []orchestrator.PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2", DependsOn: []string{"task-1"}},
				{ID: "task-3", DependsOn: []string{"task-2"}},
			},
			want: 3,
		},
		{
			name: "diamond dependencies - 3 groups",
			tasks: []orchestrator.PlannedTask{
				{ID: "task-1", DependsOn: []string{}},
				{ID: "task-2a", DependsOn: []string{"task-1"}},
				{ID: "task-2b", DependsOn: []string{"task-1"}},
				{ID: "task-3", DependsOn: []string{"task-2a", "task-2b"}},
			},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := make(map[string][]string)
			for _, task := range tt.tasks {
				deps[task.ID] = task.DependsOn
			}

			got := calculateExecutionOrder(tt.tasks, deps)
			if len(got) != tt.want {
				t.Errorf("calculateExecutionOrder() got %d groups, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFormatPlanForDisplay(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Summary: "Test plan summary",
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Setup", EstComplexity: "low", Files: []string{"main.go"}},
			{ID: "task-2", Title: "Implement", EstComplexity: "medium", DependsOn: []string{"task-1"}},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
		Insights:       []string{"Codebase uses standard patterns"},
		Constraints:    []string{"Must maintain backwards compatibility"},
	}

	output := FormatPlanForDisplay(plan)

	// Check that key sections are present
	if output == "" {
		t.Error("FormatPlanForDisplay() returned empty string")
	}

	// Should contain summary
	if !containsString(output, "Test plan summary") {
		t.Error("FormatPlanForDisplay() missing summary")
	}

	// Should contain task count
	if !containsString(output, "2 total") {
		t.Error("FormatPlanForDisplay() missing task count")
	}

	// Should contain insights
	if !containsString(output, "Codebase uses standard patterns") {
		t.Error("FormatPlanForDisplay() missing insights")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly 10", 10, "exactly 10"},
		{"this is a longer title", 10, "this is..."},
	}

	for _, tt := range tests {
		got := truncateTitle(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncateTitle(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestGenerateGroupTitle(t *testing.T) {
	tests := []struct {
		name     string
		groupNum int
		taskIDs  []string
		plan     *orchestrator.PlanSpec
		want     string
	}{
		{
			name:     "empty tasks",
			groupNum: 1,
			taskIDs:  []string{},
			plan:     &orchestrator.PlanSpec{Tasks: []orchestrator.PlannedTask{}},
			want:     "Tasks",
		},
		{
			name:     "majority extract prefix",
			groupNum: 1,
			taskIDs:  []string{"task-1", "task-2", "task-3"},
			plan: &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Extract component A"},
					{ID: "task-2", Title: "Extract component B"},
					{ID: "task-3", Title: "Extract component C"},
				},
			},
			want: "Extract Components",
		},
		{
			name:     "majority implement prefix",
			groupNum: 2,
			taskIDs:  []string{"task-1", "task-2"},
			plan: &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Implement feature X"},
					{ID: "task-2", Title: "Implement feature Y"},
				},
			},
			want: "Implement Components",
		},
		{
			name:     "no common prefix - group 1 fallback",
			groupNum: 1,
			taskIDs:  []string{"task-1", "task-2"},
			plan: &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Setup environment"},
					{ID: "task-2", Title: "Configure database"},
				},
			},
			want: "Foundation",
		},
		{
			name:     "no common prefix - group 2 fallback",
			groupNum: 2,
			taskIDs:  []string{"task-1", "task-2"},
			plan: &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Add tests"},
					{ID: "task-2", Title: "Update docs"},
				},
			},
			want: "Core Implementation",
		},
		{
			name:     "no common prefix - group 3 fallback",
			groupNum: 3,
			taskIDs:  []string{"task-1", "task-2"},
			plan: &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Final review"},
					{ID: "task-2", Title: "Integration check"},
				},
			},
			want: "Integration",
		},
		{
			name:     "no common prefix - group 4+ fallback",
			groupNum: 4,
			taskIDs:  []string{"task-1", "task-2"},
			plan: &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Cleanup code"},
					{ID: "task-2", Title: "Deploy changes"},
				},
			},
			want: "Phase 4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateGroupTitle(tt.groupNum, tt.taskIDs, tt.plan)
			if got != tt.want {
				t.Errorf("generateGroupTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateGroupSummary(t *testing.T) {
	tests := []struct {
		name      string
		groupIdx  int
		taskCount int
		wantStart string // check that it starts with expected text
	}{
		{
			name:      "first group - no dependencies",
			groupIdx:  0,
			taskCount: 5,
			wantStart: "This group contains 5 tasks that can start immediately",
		},
		{
			name:      "later group - has dependencies",
			groupIdx:  1,
			taskCount: 3,
			wantStart: "This group contains 3 tasks that depend on",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateGroupSummary(tt.groupIdx, tt.taskCount)
			if !containsString(got, tt.wantStart) {
				t.Errorf("generateGroupSummary() = %q, want to contain %q", got, tt.wantStart)
			}
		})
	}
}

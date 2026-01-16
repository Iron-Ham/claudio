package ultraplan

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestTaskRenderer_CalculateGroupStats(t *testing.T) {
	tests := []struct {
		name           string
		completedTasks []string
		failedTasks    []string
		runningTasks   map[string]string
		group          []string
		expected       GroupStats
	}{
		{
			name:           "all pending",
			completedTasks: []string{},
			failedTasks:    []string{},
			runningTasks:   map[string]string{},
			group:          []string{"task-1", "task-2", "task-3"},
			expected: GroupStats{
				Total:     3,
				Completed: 0,
				Failed:    0,
				Running:   0,
				HasFailed: false,
			},
		},
		{
			name:           "all completed",
			completedTasks: []string{"task-1", "task-2", "task-3"},
			failedTasks:    []string{},
			runningTasks:   map[string]string{},
			group:          []string{"task-1", "task-2", "task-3"},
			expected: GroupStats{
				Total:     3,
				Completed: 3,
				Failed:    0,
				Running:   0,
				HasFailed: false,
			},
		},
		{
			name:           "mixed with failures",
			completedTasks: []string{"task-1"},
			failedTasks:    []string{"task-2"},
			runningTasks:   map[string]string{},
			group:          []string{"task-1", "task-2", "task-3"},
			expected: GroupStats{
				Total:     3,
				Completed: 1,
				Failed:    1,
				Running:   0,
				HasFailed: true,
			},
		},
		{
			name:           "some running",
			completedTasks: []string{"task-1"},
			failedTasks:    []string{},
			runningTasks:   map[string]string{"task-2": "inst-2"},
			group:          []string{"task-1", "task-2", "task-3"},
			expected: GroupStats{
				Total:     3,
				Completed: 1,
				Failed:    0,
				Running:   1,
				HasFailed: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.UltraPlanSession{
				CompletedTasks: tt.completedTasks,
				FailedTasks:    tt.failedTasks,
				TaskToInstance: tt.runningTasks,
			}

			ctx := &RenderContext{
				UltraPlan: &State{},
			}
			r := NewTaskRenderer(ctx)
			got := r.CalculateGroupStats(session, tt.group)

			if got.Total != tt.expected.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.expected.Total)
			}
			if got.Completed != tt.expected.Completed {
				t.Errorf("Completed = %d, want %d", got.Completed, tt.expected.Completed)
			}
			if got.Failed != tt.expected.Failed {
				t.Errorf("Failed = %d, want %d", got.Failed, tt.expected.Failed)
			}
			if got.Running != tt.expected.Running {
				t.Errorf("Running = %d, want %d", got.Running, tt.expected.Running)
			}
			if got.HasFailed != tt.expected.HasFailed {
				t.Errorf("HasFailed = %v, want %v", got.HasFailed, tt.expected.HasFailed)
			}
		})
	}
}

func TestTaskRenderer_FormatGroupSummary(t *testing.T) {
	tests := []struct {
		name     string
		stats    GroupStats
		expected string
	}{
		{
			name:     "all pending",
			stats:    GroupStats{Total: 4, Completed: 0, Failed: 0, Running: 0, HasFailed: false},
			expected: "[0/4]",
		},
		{
			name:     "all completed",
			stats:    GroupStats{Total: 4, Completed: 4, Failed: 0, Running: 0, HasFailed: false},
			expected: "[✓ 4/4]",
		},
		{
			name:     "some running",
			stats:    GroupStats{Total: 4, Completed: 2, Failed: 0, Running: 1, HasFailed: false},
			expected: "[⟳ 2/4]",
		},
		{
			name:     "has failures",
			stats:    GroupStats{Total: 4, Completed: 2, Failed: 1, Running: 0, HasFailed: true},
			expected: "[✗ 2/4]",
		},
	}

	ctx := &RenderContext{
		UltraPlan: &State{},
	}
	r := NewTaskRenderer(ctx)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.FormatGroupSummary(tt.stats)
			if got != tt.expected {
				t.Errorf("FormatGroupSummary() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTaskRenderer_GetGroupStatus(t *testing.T) {
	tests := []struct {
		name           string
		completedTasks []string
		failedTasks    []string
		runningTasks   map[string]string
		group          []string
		expected       string
	}{
		{
			name:           "all complete",
			completedTasks: []string{"task-1", "task-2"},
			failedTasks:    []string{},
			runningTasks:   map[string]string{},
			group:          []string{"task-1", "task-2"},
			expected:       "✓",
		},
		{
			name:           "has failures",
			completedTasks: []string{"task-1"},
			failedTasks:    []string{"task-2"},
			runningTasks:   map[string]string{},
			group:          []string{"task-1", "task-2"},
			expected:       "✗",
		},
		{
			name:           "running",
			completedTasks: []string{},
			failedTasks:    []string{},
			runningTasks:   map[string]string{"task-1": "inst-1"},
			group:          []string{"task-1", "task-2"},
			expected:       "⟳",
		},
		{
			name:           "pending",
			completedTasks: []string{},
			failedTasks:    []string{},
			runningTasks:   map[string]string{},
			group:          []string{"task-1", "task-2"},
			expected:       "○",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.UltraPlanSession{
				CompletedTasks: tt.completedTasks,
				FailedTasks:    tt.failedTasks,
				TaskToInstance: tt.runningTasks,
			}

			ctx := &RenderContext{
				UltraPlan: &State{},
			}
			r := NewTaskRenderer(ctx)
			got := r.GetGroupStatus(session, tt.group)
			if got != tt.expected {
				t.Errorf("GetGroupStatus() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTaskRenderer_FindInstanceIDForTask(t *testing.T) {
	tests := []struct {
		name       string
		taskMap    map[string]string
		taskID     string
		wantInstID string
	}{
		{
			name:       "task found",
			taskMap:    map[string]string{"task-1": "inst-abc"},
			taskID:     "task-1",
			wantInstID: "inst-abc",
		},
		{
			name:       "task not found",
			taskMap:    map[string]string{},
			taskID:     "task-1",
			wantInstID: "",
		},
		{
			name:       "empty instance ID",
			taskMap:    map[string]string{"task-1": ""},
			taskID:     "task-1",
			wantInstID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.UltraPlanSession{
				TaskToInstance: tt.taskMap,
			}

			ctx := &RenderContext{
				UltraPlan: &State{},
			}
			r := NewTaskRenderer(ctx)
			got := r.FindInstanceIDForTask(session, tt.taskID)
			if got != tt.wantInstID {
				t.Errorf("FindInstanceIDForTask() = %q, want %q", got, tt.wantInstID)
			}
		})
	}
}

func TestTaskRenderer_RenderExecutionTaskLine(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &State{},
	}
	r := NewTaskRenderer(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "Short task",
	}

	// Test non-selected task
	result := r.RenderExecutionTaskLine(session, task, "", false, false, 40)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for short task", result.LineCount)
	}

	// Test long task wrapping when selected
	longTask := &orchestrator.PlannedTask{
		ID:    "task-2",
		Title: "This is a very long task title that should wrap",
	}
	result = r.RenderExecutionTaskLine(session, longTask, "", true, true, 30)
	if result.LineCount <= 1 {
		t.Errorf("LineCount = %d, want > 1 for selected long task", result.LineCount)
	}

	// Verify line count matches actual newlines
	actualLines := strings.Count(result.Content, "\n") + 1
	if result.LineCount != actualLines {
		t.Errorf("LineCount = %d but content has %d lines", result.LineCount, actualLines)
	}
}

func TestTaskRenderer_RenderPhaseInstanceLine(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &State{},
	}
	r := NewTaskRenderer(ctx)

	// Test nil instance
	line := r.RenderPhaseInstanceLine(nil, "Test", false, false, 40)
	if line == "" {
		t.Error("Expected non-empty line for nil instance")
	}

	// Test working instance
	inst := &orchestrator.Instance{
		Status: orchestrator.StatusWorking,
	}
	line = r.RenderPhaseInstanceLine(inst, "Working", false, true, 40)
	if line == "" {
		t.Error("Expected non-empty line for working instance")
	}

	// Test selected instance
	line = r.RenderPhaseInstanceLine(inst, "Selected", true, true, 40)
	if line == "" {
		t.Error("Expected non-empty line for selected instance")
	}
}

func TestTaskRenderer_RenderGroupConsolidatorLine(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &State{},
	}
	r := NewTaskRenderer(ctx)

	// Test nil instance
	line := r.RenderGroupConsolidatorLine(nil, 0, false, false, 40)
	if line == "" {
		t.Error("Expected non-empty line for nil instance")
	}

	// Test completed consolidator
	inst := &orchestrator.Instance{
		Status: orchestrator.StatusCompleted,
	}
	line = r.RenderGroupConsolidatorLine(inst, 0, false, true, 40)
	if line == "" {
		t.Error("Expected non-empty line for completed consolidator")
	}
}

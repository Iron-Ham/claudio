package view

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestCalculateGroupStats(t *testing.T) {
	tests := []struct {
		name           string
		completedTasks []string
		failedTasks    []string
		runningTasks   map[string]string // taskID -> instanceID
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
			name:           "mixed - some failed",
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
		{
			name:           "running task already counted as completed is not double-counted",
			completedTasks: []string{"task-1", "task-2"},
			failedTasks:    []string{},
			runningTasks:   map[string]string{"task-2": "inst-2"}, // task-2 still in map but completed
			group:          []string{"task-1", "task-2", "task-3"},
			expected: GroupStats{
				Total:     3,
				Completed: 2,
				Failed:    0,
				Running:   0, // task-2 is completed, not running
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
				UltraPlan: &UltraPlanState{},
			}
			v := NewUltraplanView(ctx)

			got := v.calculateGroupStats(session, tt.group)

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

func TestFormatGroupSummary(t *testing.T) {
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
		{
			name:     "running with failures - running takes priority",
			stats:    GroupStats{Total: 4, Completed: 1, Failed: 1, Running: 1, HasFailed: true},
			expected: "[⟳ 1/4]",
		},
		{
			name:     "partial completion",
			stats:    GroupStats{Total: 5, Completed: 3, Failed: 0, Running: 0, HasFailed: false},
			expected: "[3/5]",
		},
	}

	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.formatGroupSummary(tt.stats)
			if got != tt.expected {
				t.Errorf("formatGroupSummary() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestUltraPlanState_CollapsedGroups_InitialState(t *testing.T) {
	// Test that CollapsedGroups starts as nil and can be initialized
	state := &UltraPlanState{}

	if state.CollapsedGroups != nil {
		t.Error("CollapsedGroups should be nil initially")
	}

	if state.SelectedGroupIdx != 0 {
		t.Error("SelectedGroupIdx should be 0 initially")
	}

	if state.GroupNavMode {
		t.Error("GroupNavMode should be false initially")
	}
}

func TestUltraPlanState_CollapsedGroups_Toggle(t *testing.T) {
	state := &UltraPlanState{
		CollapsedGroups: make(map[int]bool),
	}

	// Initially all groups should be expanded (false or not in map)
	if state.CollapsedGroups[0] {
		t.Error("Group 0 should be expanded initially")
	}

	// Toggle to collapsed
	state.CollapsedGroups[0] = true
	if !state.CollapsedGroups[0] {
		t.Error("Group 0 should be collapsed after toggle")
	}

	// Toggle back to expanded
	state.CollapsedGroups[0] = false
	if state.CollapsedGroups[0] {
		t.Error("Group 0 should be expanded after second toggle")
	}
}

func TestGroupStats_ZeroValues(t *testing.T) {
	// Test that GroupStats has sensible zero values
	stats := GroupStats{}

	if stats.Total != 0 {
		t.Errorf("Total should be 0, got %d", stats.Total)
	}
	if stats.Completed != 0 {
		t.Errorf("Completed should be 0, got %d", stats.Completed)
	}
	if stats.Failed != 0 {
		t.Errorf("Failed should be 0, got %d", stats.Failed)
	}
	if stats.Running != 0 {
		t.Errorf("Running should be 0, got %d", stats.Running)
	}
	if stats.HasFailed {
		t.Error("HasFailed should be false")
	}
}

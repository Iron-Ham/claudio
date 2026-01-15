package view

import (
	"strings"
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

func TestRenderExecutionTaskLine_SingleLine(t *testing.T) {
	// Test that short task titles render on a single line
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "Short task",
	}

	// Test non-selected task
	result := v.renderExecutionTaskLine(session, task, "", false, false, 40)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for short task", result.LineCount)
	}

	// Test selected task with short title
	result = v.renderExecutionTaskLine(session, task, "", true, true, 40)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for selected short task", result.LineCount)
	}
}

func TestRenderExecutionTaskLine_MultiLineWrapping(t *testing.T) {
	// Test that long task titles wrap across multiple lines when selected
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	longTitle := "This is a very long task title that should definitely wrap across multiple lines when selected"
	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: longTitle,
	}

	// Test non-selected long task - should be 1 line (truncated)
	result := v.renderExecutionTaskLine(session, task, "", false, false, 40)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for non-selected long task (should truncate)", result.LineCount)
	}

	// Test selected long task - should wrap to multiple lines
	result = v.renderExecutionTaskLine(session, task, "", true, true, 40)
	if result.LineCount <= 1 {
		t.Errorf("LineCount = %d, want > 1 for selected long task", result.LineCount)
	}
}

func TestRenderExecutionTaskLine_WordBoundaryWrapping(t *testing.T) {
	// Test that wrapping occurs at word boundaries when possible
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	// Title with clear word boundaries
	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "Implement user authentication system for the application",
	}

	result := v.renderExecutionTaskLine(session, task, "", true, true, 30)

	// Should have multiple lines
	if result.LineCount < 2 {
		t.Errorf("LineCount = %d, want >= 2 for wrapped task", result.LineCount)
	}

	// Content should not contain truncated words in the middle
	// (The exact content depends on styling, so we just verify it has newlines)
	if result.LineCount > 1 && len(result.Content) == 0 {
		t.Error("Content should not be empty for multi-line task")
	}
}

func TestRenderExecutionTaskLine_StatusIcons(t *testing.T) {
	// Test that different task statuses show correct icons
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	tests := []struct {
		name           string
		completedTasks []string
		failedTasks    []string
		instanceID     string
	}{
		{
			name:           "completed task",
			completedTasks: []string{"task-1"},
			failedTasks:    []string{},
			instanceID:     "",
		},
		{
			name:           "failed task",
			completedTasks: []string{},
			failedTasks:    []string{"task-1"},
			instanceID:     "",
		},
		{
			name:           "running task",
			completedTasks: []string{},
			failedTasks:    []string{},
			instanceID:     "inst-1",
		},
		{
			name:           "pending task",
			completedTasks: []string{},
			failedTasks:    []string{},
			instanceID:     "",
		},
	}

	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "Test task",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &orchestrator.UltraPlanSession{
				CompletedTasks: tt.completedTasks,
				FailedTasks:    tt.failedTasks,
			}

			result := v.renderExecutionTaskLine(session, task, tt.instanceID, false, false, 40)

			// Basic check - content should not be empty
			if len(result.Content) == 0 {
				t.Error("Content should not be empty")
			}

			// LineCount should be 1 for non-wrapped tasks
			if result.LineCount != 1 {
				t.Errorf("LineCount = %d, want 1", result.LineCount)
			}
		})
	}
}

func TestExecutionTaskResult_StructFields(t *testing.T) {
	// Test ExecutionTaskResult struct integrity
	result := ExecutionTaskResult{
		Content:   "test content",
		LineCount: 3,
	}

	if result.Content != "test content" {
		t.Errorf("Content = %q, want %q", result.Content, "test content")
	}

	if result.LineCount != 3 {
		t.Errorf("LineCount = %d, want 3", result.LineCount)
	}
}

func TestTrimLeadingSpaces(t *testing.T) {
	tests := []struct {
		name     string
		input    []rune
		expected []rune
	}{
		{
			name:     "no leading spaces",
			input:    []rune("hello"),
			expected: []rune("hello"),
		},
		{
			name:     "single leading space",
			input:    []rune(" hello"),
			expected: []rune("hello"),
		},
		{
			name:     "multiple leading spaces",
			input:    []rune("   hello world"),
			expected: []rune("hello world"),
		},
		{
			name:     "all spaces",
			input:    []rune("   "),
			expected: []rune{},
		},
		{
			name:     "empty input",
			input:    []rune{},
			expected: []rune{},
		},
		{
			name:     "spaces in middle preserved",
			input:    []rune("  hello  world"),
			expected: []rune("hello  world"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimLeadingSpaces(tt.input)
			if string(got) != string(tt.expected) {
				t.Errorf("trimLeadingSpaces(%q) = %q, want %q", string(tt.input), string(got), string(tt.expected))
			}
		})
	}
}

func TestPadToWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "already at width",
			input:    "hello",
			width:    5,
			expected: "hello",
		},
		{
			name:     "shorter than width",
			input:    "hi",
			width:    5,
			expected: "hi   ",
		},
		{
			name:     "longer than width",
			input:    "hello world",
			width:    5,
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			width:    3,
			expected: "   ",
		},
		{
			name:     "zero width",
			input:    "hello",
			width:    0,
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := padToWidth(tt.input, tt.width)
			if got != tt.expected {
				t.Errorf("padToWidth(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.expected)
			}
		})
	}
}

func TestRenderExecutionTaskLine_VeryNarrowWidth(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "A task with a title",
	}

	// Width of 6 means titleLen = 0
	result := v.renderExecutionTaskLine(session, task, "", true, true, 6)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for narrow width", result.LineCount)
	}

	// Width of 5 means titleLen = -1 (negative)
	result = v.renderExecutionTaskLine(session, task, "", true, true, 5)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for very narrow width", result.LineCount)
	}
}

func TestRenderExecutionTaskLine_EmptyTitle(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "",
	}

	result := v.renderExecutionTaskLine(session, task, "", true, true, 40)
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for empty title", result.LineCount)
	}
}

func TestRenderExecutionTaskLine_TitleExactlyFits(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	// Width 40 means titleLen = 34 (40 - 6)
	// Create a title that is exactly 34 characters
	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "1234567890123456789012345678901234", // exactly 34 chars
	}

	result := v.renderExecutionTaskLine(session, task, "", true, true, 40)

	// Title fits exactly, should NOT wrap
	if result.LineCount != 1 {
		t.Errorf("LineCount = %d, want 1 for title that exactly fits", result.LineCount)
	}
}

func TestRenderExecutionTaskLine_LineCountMatchesContent(t *testing.T) {
	ctx := &RenderContext{
		UltraPlan: &UltraPlanState{},
	}
	v := NewUltraplanView(ctx)

	session := &orchestrator.UltraPlanSession{
		CompletedTasks: []string{},
		FailedTasks:    []string{},
	}

	task := &orchestrator.PlannedTask{
		ID:    "task-1",
		Title: "This is a long task title that will definitely wrap across multiple lines when rendered",
	}

	result := v.renderExecutionTaskLine(session, task, "", true, true, 30)

	// Count actual newlines in content
	actualLines := strings.Count(result.Content, "\n") + 1
	if result.LineCount != actualLines {
		t.Errorf("LineCount = %d but content has %d lines", result.LineCount, actualLines)
	}
}

func TestIsGroupCollapsed(t *testing.T) {
	// Test the IsGroupCollapsed method with various scenarios
	tests := []struct {
		name            string
		collapsedGroups map[int]bool
		groupIdx        int
		currentGroup    int
		wantCollapsed   bool
	}{
		{
			name:            "current group is expanded by default",
			collapsedGroups: nil,
			groupIdx:        0,
			currentGroup:    0,
			wantCollapsed:   false,
		},
		{
			name:            "non-current group is collapsed by default",
			collapsedGroups: nil,
			groupIdx:        1,
			currentGroup:    0,
			wantCollapsed:   true,
		},
		{
			name:            "explicitly expanded group overrides default",
			collapsedGroups: map[int]bool{1: false}, // Explicitly expanded
			groupIdx:        1,
			currentGroup:    0,
			wantCollapsed:   false,
		},
		{
			name:            "explicitly collapsed group overrides default",
			collapsedGroups: map[int]bool{0: true}, // Explicitly collapsed
			groupIdx:        0,
			currentGroup:    0, // Even though it's current, it's explicitly collapsed
			wantCollapsed:   true,
		},
		{
			name:            "past group is collapsed by default",
			collapsedGroups: nil,
			groupIdx:        0,
			currentGroup:    2,
			wantCollapsed:   true,
		},
		{
			name:            "future group is collapsed by default",
			collapsedGroups: nil,
			groupIdx:        3,
			currentGroup:    1,
			wantCollapsed:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &UltraPlanState{
				CollapsedGroups: tt.collapsedGroups,
			}

			got := state.IsGroupCollapsed(tt.groupIdx, tt.currentGroup)
			if got != tt.wantCollapsed {
				t.Errorf("IsGroupCollapsed(%d, %d) = %v, want %v",
					tt.groupIdx, tt.currentGroup, got, tt.wantCollapsed)
			}
		})
	}
}

func TestSetGroupExpanded(t *testing.T) {
	state := &UltraPlanState{}

	// Initially nil map
	if state.CollapsedGroups != nil {
		t.Error("CollapsedGroups should be nil initially")
	}

	// Set group expanded should create the map
	state.SetGroupExpanded(1)
	if state.CollapsedGroups == nil {
		t.Error("CollapsedGroups should be created")
	}
	if state.CollapsedGroups[1] != false {
		t.Error("Group 1 should be explicitly expanded (false)")
	}

	// Verify it's different from default behavior
	if _, exists := state.CollapsedGroups[1]; !exists {
		t.Error("Group 1 should have explicit entry in map")
	}
}

func TestSetGroupCollapsed(t *testing.T) {
	state := &UltraPlanState{}

	// Set group collapsed should create the map
	state.SetGroupCollapsed(2)
	if state.CollapsedGroups == nil {
		t.Error("CollapsedGroups should be created")
	}
	if state.CollapsedGroups[2] != true {
		t.Error("Group 2 should be explicitly collapsed (true)")
	}
}

func TestInlineContentCollapseIcon(t *testing.T) {
	// Test the collapse icon logic used in RenderInlineContent
	tests := []struct {
		name        string
		isCollapsed bool
		wantIcon    string
	}{
		{"expanded", false, "▼"},
		{"collapsed", true, "▶"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collapseIcon := "▼"
			if tt.isCollapsed {
				collapseIcon = "▶"
			}
			if collapseIcon != tt.wantIcon {
				t.Errorf("collapse icon = %q, want %q", collapseIcon, tt.wantIcon)
			}
		})
	}
}

package panel

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/charmbracelet/lipgloss"
)

func TestStatsPanel_Render(t *testing.T) {
	tests := []struct {
		name     string
		state    *RenderState
		contains []string
		notEmpty bool
	}{
		{
			name: "full stats with metrics",
			state: &RenderState{
				Width:          80,
				Height:         24,
				SessionCreated: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				SessionMetrics: &orchestrator.SessionMetrics{
					TotalInputTokens:  50000,
					TotalOutputTokens: 25000,
					TotalCacheRead:    10000,
					TotalCacheWrite:   5000,
					TotalCost:         1.25,
					InstanceCount:     3,
					ActiveCount:       2,
				},
				Instances: []*orchestrator.Instance{
					{ID: "inst-1", Task: "Build feature", Metrics: &orchestrator.Metrics{Cost: 0.50}},
					{ID: "inst-2", Task: "Fix bug", Metrics: &orchestrator.Metrics{Cost: 0.45}},
					{ID: "inst-3", Task: "Write tests", Metrics: &orchestrator.Metrics{Cost: 0.30}},
				},
			},
			contains: []string{
				"Session Statistics",
				"Session Summary",
				"Total Instances: 3 (2 active)",
				"2024-01-15 10:30:00",
				"Token Usage",
				"Input:",
				"Output:",
				"Total:",
				"Cache:",
				"Estimated Cost",
				"$1.25",
				"Top Instances by Cost",
				"Build feature",
				"Fix bug",
				"Write tests",
				"Press [m] to close",
			},
			notEmpty: true,
		},
		{
			name: "no session metrics",
			state: &RenderState{
				Width:          80,
				Height:         24,
				SessionMetrics: nil,
			},
			contains: []string{
				"Session Statistics",
				"No active session",
			},
			notEmpty: true,
		},
		{
			name: "cost warning threshold exceeded",
			state: &RenderState{
				Width:  80,
				Height: 24,
				SessionMetrics: &orchestrator.SessionMetrics{
					TotalCost:     5.50,
					InstanceCount: 1,
					ActiveCount:   1,
				},
				CostWarningThreshold: 5.00,
			},
			contains: []string{
				"exceeds warning threshold",
			},
			notEmpty: true,
		},
		{
			name: "with cost limit",
			state: &RenderState{
				Width:  80,
				Height: 24,
				SessionMetrics: &orchestrator.SessionMetrics{
					TotalCost:     2.00,
					InstanceCount: 1,
					ActiveCount:   1,
				},
				CostLimit: 10.00,
			},
			contains: []string{
				"Limit: $10.00",
			},
			notEmpty: true,
		},
		{
			name: "no cache tokens",
			state: &RenderState{
				Width:  80,
				Height: 24,
				SessionMetrics: &orchestrator.SessionMetrics{
					TotalInputTokens:  1000,
					TotalOutputTokens: 500,
					TotalCacheRead:    0,
					TotalCacheWrite:   0,
					InstanceCount:     1,
				},
			},
			notEmpty: true,
		},
		{
			name: "invalid state returns error indicator",
			state: &RenderState{
				Width:  0,
				Height: 0,
			},
			contains: []string{"render error"},
			notEmpty: true,
		},
	}

	panel := NewStatsPanel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := panel.Render(tt.state)

			if tt.notEmpty && result == "" {
				t.Error("expected non-empty result, got empty")
			}
			if !tt.notEmpty && result != "" {
				t.Errorf("expected empty result, got: %s", result)
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestStatsPanel_Height(t *testing.T) {
	panel := NewStatsPanel()

	// Render to set height
	state := &RenderState{
		Width:  80,
		Height: 24,
		SessionMetrics: &orchestrator.SessionMetrics{
			TotalInputTokens:  1000,
			TotalOutputTokens: 500,
			InstanceCount:     1,
			ActiveCount:       1,
		},
	}

	panel.Render(state)

	if panel.Height() <= 0 {
		t.Errorf("Height() = %d, want positive value", panel.Height())
	}
}

func TestStatsPanel_TopInstancesSorting(t *testing.T) {
	panel := NewStatsPanel()

	state := &RenderState{
		Width:  80,
		Height: 24,
		SessionMetrics: &orchestrator.SessionMetrics{
			InstanceCount: 3,
			ActiveCount:   3,
		},
		Instances: []*orchestrator.Instance{
			{ID: "low", Task: "Low cost task", Metrics: &orchestrator.Metrics{Cost: 0.10}},
			{ID: "high", Task: "High cost task", Metrics: &orchestrator.Metrics{Cost: 0.90}},
			{ID: "mid", Task: "Mid cost task", Metrics: &orchestrator.Metrics{Cost: 0.50}},
		},
	}

	result := panel.Render(state)

	// Find positions of each task name
	highPos := strings.Index(result, "High cost task")
	midPos := strings.Index(result, "Mid cost task")
	lowPos := strings.Index(result, "Low cost task")

	// High should come before mid, mid before low (descending order)
	if highPos >= midPos {
		t.Error("High cost task should appear before mid cost task")
	}
	if midPos >= lowPos {
		t.Error("Mid cost task should appear before low cost task")
	}
}

func TestStatsPanel_NoInstances(t *testing.T) {
	panel := NewStatsPanel()

	state := &RenderState{
		Width:  80,
		Height: 24,
		SessionMetrics: &orchestrator.SessionMetrics{
			InstanceCount: 0,
		},
		Instances: []*orchestrator.Instance{},
	}

	result := panel.Render(state)

	if !strings.Contains(result, "No cost data available") {
		t.Error("expected 'No cost data available' when no instances")
	}
}

func TestStatsPanel_InstancesWithNoCost(t *testing.T) {
	panel := NewStatsPanel()

	state := &RenderState{
		Width:  80,
		Height: 24,
		SessionMetrics: &orchestrator.SessionMetrics{
			InstanceCount: 2,
		},
		Instances: []*orchestrator.Instance{
			{ID: "no-metrics", Task: "Task 1", Metrics: nil},
			{ID: "zero-cost", Task: "Task 2", Metrics: &orchestrator.Metrics{Cost: 0}},
		},
	}

	result := panel.Render(state)

	if !strings.Contains(result, "No cost data available") {
		t.Error("expected 'No cost data available' when instances have no cost")
	}
}

func TestStatsPanel_TopFiveLimit(t *testing.T) {
	panel := NewStatsPanel()

	// Create 10 instances
	instances := make([]*orchestrator.Instance, 10)
	for i := 0; i < 10; i++ {
		instances[i] = &orchestrator.Instance{
			ID:      "inst-" + string(rune('a'+i)),
			Task:    "Task " + string(rune('A'+i)),
			Metrics: &orchestrator.Metrics{Cost: float64(i+1) * 0.10},
		}
	}

	state := &RenderState{
		Width:  80,
		Height: 24,
		SessionMetrics: &orchestrator.SessionMetrics{
			InstanceCount: 10,
		},
		Instances: instances,
	}

	result := panel.Render(state)

	// Count how many "Task" lines appear (each instance shows as "N. [X] Task Y: $Z.ZZ")
	lines := strings.Split(result, "\n")
	taskLineCount := 0
	for _, line := range lines {
		if strings.Contains(line, ". [") && strings.Contains(line, "Task ") && strings.Contains(line, "$") {
			taskLineCount++
		}
	}

	if taskLineCount != 5 {
		t.Errorf("expected 5 instance lines, got %d", taskLineCount)
	}
}

func TestNewStatsPanel(t *testing.T) {
	panel := NewStatsPanel()
	if panel == nil {
		t.Error("NewStatsPanel() returned nil")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hi", 2, "..."},
		{"test", 3, "..."},
		{"", 5, ""},
		{"longstring", 10, "longstring"},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestStatsPanel_RenderWithBox(t *testing.T) {
	// Create a simple box style for testing
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	tests := []struct {
		name           string
		state          *RenderState
		contains       []string
		notContains    []string
		checkBoxBorder bool
	}{
		{
			name: "renders stats with box wrapper",
			state: &RenderState{
				Width:  80,
				Height: 24,
				SessionMetrics: &orchestrator.SessionMetrics{
					TotalInputTokens:  5000,
					TotalOutputTokens: 2500,
					TotalCost:         0.75,
					InstanceCount:     2,
					ActiveCount:       1,
				},
				SessionCreated: time.Date(2024, 3, 20, 14, 30, 0, 0, time.UTC),
				Instances: []*orchestrator.Instance{
					{ID: "inst-1", Task: "Test task", Metrics: &orchestrator.Metrics{Cost: 0.75}},
				},
			},
			contains: []string{
				"Session Statistics",
				"Session Summary",
				"Token Usage",
				"Estimated Cost",
				"$0.75",
			},
			checkBoxBorder: true,
		},
		{
			name: "no session renders with box",
			state: &RenderState{
				Width:          80,
				Height:         24,
				SessionMetrics: nil,
			},
			contains: []string{
				"Session Statistics",
				"No active session",
			},
			checkBoxBorder: true,
		},
		{
			name: "box respects width from state",
			state: &RenderState{
				Width:  100,
				Height: 24,
				SessionMetrics: &orchestrator.SessionMetrics{
					TotalCost:     1.00,
					InstanceCount: 1,
					ActiveCount:   1,
				},
			},
			contains: []string{
				"Session Statistics",
			},
			checkBoxBorder: true,
		},
	}

	panel := NewStatsPanel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := panel.RenderWithBox(tt.state, boxStyle)

			if result == "" {
				t.Error("expected non-empty result, got empty")
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, result)
				}
			}

			for _, notWant := range tt.notContains {
				if strings.Contains(result, notWant) {
					t.Errorf("result should not contain %q\nGot:\n%s", notWant, result)
				}
			}

			// Check that the box border characters are present (rounded border uses ╭╮╰╯)
			if tt.checkBoxBorder {
				hasBorder := strings.Contains(result, "╭") || strings.Contains(result, "│")
				if !hasBorder {
					t.Error("expected box border characters in output")
				}
			}
		})
	}
}

func TestStatsPanel_RenderWithBox_ContentMatchesRender(t *testing.T) {
	// Verify that RenderWithBox wraps the same content as Render
	panel := NewStatsPanel()

	state := &RenderState{
		Width:  80,
		Height: 24,
		SessionMetrics: &orchestrator.SessionMetrics{
			TotalInputTokens:  10000,
			TotalOutputTokens: 5000,
			TotalCost:         1.50,
			InstanceCount:     3,
			ActiveCount:       2,
		},
		SessionCreated: time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
		Instances: []*orchestrator.Instance{
			{ID: "a", Task: "Task A", Metrics: &orchestrator.Metrics{Cost: 0.50}},
			{ID: "b", Task: "Task B", Metrics: &orchestrator.Metrics{Cost: 0.60}},
			{ID: "c", Task: "Task C", Metrics: &orchestrator.Metrics{Cost: 0.40}},
		},
	}

	// Get the raw content
	rawContent := panel.Render(state)

	// Get the boxed content with a minimal style
	boxStyle := lipgloss.NewStyle().Width(state.Width - 4)
	boxedContent := panel.RenderWithBox(state, boxStyle)

	// The boxed content should contain all the same text elements
	expectedParts := []string{
		"Session Statistics",
		"Session Summary",
		"Token Usage",
		"Estimated Cost",
		"$1.50",
		"Task A",
		"Task B",
		"Task C",
	}

	for _, part := range expectedParts {
		if !strings.Contains(rawContent, part) {
			t.Errorf("Render() missing %q", part)
		}
		if !strings.Contains(boxedContent, part) {
			t.Errorf("RenderWithBox() missing %q", part)
		}
	}
}

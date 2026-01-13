package view

import (
	"testing"
)

func TestCalculateOverheadLines(t *testing.T) {
	tests := []struct {
		name     string
		params   OverheadParams
		expected int
	}{
		{
			name: "minimal instance - not running, single line task",
			params: OverheadParams{
				Task:               "Simple task",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (1 line + 1 newline = 2) + Empty banner (1) = 5
			expected: 5,
		},
		{
			name: "running instance with scroll indicator",
			params: OverheadParams{
				Task:               "Simple task",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          true,
				HasSearchActive:    false,
				HasScrollIndicator: true,
			},
			// Header (2) + Task (2) + Banner (2) + Scroll (2) = 8
			expected: 8,
		},
		{
			name: "instance with dependencies",
			params: OverheadParams{
				Task:               "Task with deps",
				HasDependencies:    true,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (2) + Dependencies (1) + Empty banner (1) = 6
			expected: 6,
		},
		{
			name: "instance with dependents",
			params: OverheadParams{
				Task:               "Task with dependents",
				HasDependencies:    false,
				HasDependents:      true,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (2) + Dependents (1) + Empty banner (1) = 6
			expected: 6,
		},
		{
			name: "instance with both dependencies and dependents",
			params: OverheadParams{
				Task:               "Task",
				HasDependencies:    true,
				HasDependents:      true,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (2) + Dependencies (1) + Dependents (1) + Empty banner (1) = 7
			expected: 7,
		},
		{
			name: "instance with metrics enabled and available",
			params: OverheadParams{
				Task:               "Task",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        true,
				HasMetrics:         true,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (2) + Metrics (2) + Empty banner (1) = 7
			expected: 7,
		},
		{
			name: "instance with metrics enabled but no data",
			params: OverheadParams{
				Task:               "Task",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        true,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (2) + Empty banner (1) = 5
			expected: 5,
		},
		{
			name: "instance with search active",
			params: OverheadParams{
				Task:               "Task",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    true,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (2) + Empty banner (1) + Search (2) = 7
			expected: 7,
		},
		{
			name: "multi-line task (3 lines)",
			params: OverheadParams{
				Task:               "Line 1\nLine 2\nLine 3",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (3 lines + 1 newline = 4) + Empty banner (1) = 7
			expected: 7,
		},
		{
			name: "task at exact max lines (5 lines, no truncation needed)",
			params: OverheadParams{
				Task:               "Line 1\nLine 2\nLine 3\nLine 4\nLine 5",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (5 lines + 1 newline = 6) + Empty banner (1) = 9
			// No "..." line added since we're at exactly the max
			expected: 9,
		},
		{
			name: "task with trailing newline",
			params: OverheadParams{
				Task:               "Line 1\nLine 2\n",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Trailing newline counts as 3 lines (Line 1, Line 2, empty)
			// Header (2) + Task (3 lines + 1 newline = 4) + Empty banner (1) = 7
			expected: 7,
		},
		{
			name: "task exceeds max lines (6 lines, max is 5)",
			params: OverheadParams{
				Task:               "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6",
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Task (6 lines but capped to 5+1 for "..." = 7) + Empty banner (1) = 10
			expected: 10,
		},
		{
			name: "maximum overhead - everything enabled",
			params: OverheadParams{
				Task:               "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6",
				HasDependencies:    true,
				HasDependents:      true,
				ShowMetrics:        true,
				HasMetrics:         true,
				IsRunning:          true,
				HasSearchActive:    true,
				HasScrollIndicator: true,
			},
			// Header (2) + Task (7) + Deps (1) + Dependents (1) + Metrics (2) + Banner (2) + Scroll (2) + Search (2) = 19
			expected: 19,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewInstanceView(80, 20) // Dimensions don't affect overhead calculation
			result := v.CalculateOverheadLines(tt.params)
			if result != tt.expected {
				t.Errorf("CalculateOverheadLines() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCalculateOverheadLinesConsistency(t *testing.T) {
	// This test ensures that increasing complexity only increases overhead
	// (i.e., the function is monotonic with respect to added features)

	v := NewInstanceView(80, 20)

	baseParams := OverheadParams{
		Task:               "Simple task",
		HasDependencies:    false,
		HasDependents:      false,
		ShowMetrics:        false,
		HasMetrics:         false,
		IsRunning:          false,
		HasSearchActive:    false,
		HasScrollIndicator: false,
	}
	baseOverhead := v.CalculateOverheadLines(baseParams)

	// Adding dependencies should increase overhead
	withDeps := baseParams
	withDeps.HasDependencies = true
	depsOverhead := v.CalculateOverheadLines(withDeps)
	if depsOverhead <= baseOverhead {
		t.Errorf("Adding dependencies should increase overhead: base=%d, withDeps=%d", baseOverhead, depsOverhead)
	}

	// Adding running status should increase overhead
	withRunning := baseParams
	withRunning.IsRunning = true
	runningOverhead := v.CalculateOverheadLines(withRunning)
	if runningOverhead <= baseOverhead {
		t.Errorf("Adding running status should increase overhead: base=%d, withRunning=%d", baseOverhead, runningOverhead)
	}

	// Adding scroll indicator should increase overhead
	withScroll := baseParams
	withScroll.HasScrollIndicator = true
	scrollOverhead := v.CalculateOverheadLines(withScroll)
	if scrollOverhead <= baseOverhead {
		t.Errorf("Adding scroll indicator should increase overhead: base=%d, withScroll=%d", baseOverhead, scrollOverhead)
	}

	// Adding search should increase overhead
	withSearch := baseParams
	withSearch.HasSearchActive = true
	searchOverhead := v.CalculateOverheadLines(withSearch)
	if searchOverhead <= baseOverhead {
		t.Errorf("Adding search should increase overhead: base=%d, withSearch=%d", baseOverhead, searchOverhead)
	}
}

func TestOverheadAtLeastMinimum(t *testing.T) {
	// Ensure overhead is always at least a reasonable minimum
	// (header + single-line task + newlines)
	minExpectedOverhead := 5

	v := NewInstanceView(80, 20)

	// Even with empty task, should have minimum overhead
	params := OverheadParams{
		Task: "",
	}
	result := v.CalculateOverheadLines(params)
	if result < minExpectedOverhead {
		t.Errorf("Minimum overhead should be at least %d, got %d", minExpectedOverhead, result)
	}
}

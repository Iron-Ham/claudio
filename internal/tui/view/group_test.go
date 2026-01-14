package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestWrapGroupName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected []string
	}{
		{
			name:     "short name fits on one line",
			input:    "Short name",
			maxLen:   20,
			expected: []string{"Short name"},
		},
		{
			name:     "exact fit",
			input:    "Exact fit",
			maxLen:   9,
			expected: []string{"Exact fit"},
		},
		{
			name:     "wraps at word boundary",
			input:    "This is a long name that wraps",
			maxLen:   15,
			expected: []string{"This is a long", "name that wraps"},
		},
		{
			name:     "handles single long word",
			input:    "Superlongwordthatexceedslimit",
			maxLen:   10,
			expected: []string{"Superlongw", "ordthatexc", "eedslimit"},
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   20,
			expected: []string{""},
		},
		{
			name:     "zero max length returns original",
			input:    "Test",
			maxLen:   0,
			expected: []string{"Test"},
		},
		{
			name:     "negative max length returns original",
			input:    "Test",
			maxLen:   -5,
			expected: []string{"Test"},
		},
		{
			name:     "short string with spaces preserved",
			input:    "Word   another",
			maxLen:   20,
			expected: []string{"Word   another"}, // Original returned since it fits within maxLen
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapGroupName(tt.input, tt.maxLen)
			if len(result) != len(tt.expected) {
				t.Errorf("wrapGroupName() returned %d lines, want %d lines", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: got %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestWrapGroupNameWithWidths(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		firstLineMax    int
		continuationMax int
		expected        []string
	}{
		{
			name:            "fits on first line",
			input:           "Short name",
			firstLineMax:    20,
			continuationMax: 30,
			expected:        []string{"Short name"},
		},
		{
			name:            "wraps with different widths",
			input:           "This is a longer name for the group header",
			firstLineMax:    15,
			continuationMax: 25,
			expected:        []string{"This is a", "longer name for the group", "header"},
		},
		{
			name:            "first line narrower than continuation",
			input:           "A test of narrow first line",
			firstLineMax:    10,
			continuationMax: 20,
			expected:        []string{"A test of", "narrow first line"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapGroupNameWithWidths(tt.input, tt.firstLineMax, tt.continuationMax)
			if len(result) != len(tt.expected) {
				t.Errorf("wrapGroupNameWithWidths() returned %d lines, want %d lines", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			for i, line := range result {
				if line != tt.expected[i] {
					t.Errorf("line %d: got %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestTruncateGroupName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short name unchanged",
			input:    "Short",
			maxLen:   10,
			expected: "Short",
		},
		{
			name:     "exact fit unchanged",
			input:    "Exact",
			maxLen:   5,
			expected: "Exact",
		},
		{
			name:     "long name truncated with ellipsis",
			input:    "This is a very long name",
			maxLen:   15,
			expected: "This is a ve...",
		},
		{
			name:     "very short maxLen returns original",
			input:    "Test",
			maxLen:   2,
			expected: "Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateGroupName(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateGroupName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRenderGroupHeaderWrapped(t *testing.T) {
	tests := []struct {
		name       string
		group      *orchestrator.InstanceGroup
		progress   GroupProgress
		collapsed  bool
		isSelected bool
		width      int
		checkLines int // minimum expected number of lines
	}{
		{
			name: "short name single line",
			group: &orchestrator.InstanceGroup{
				ID:          "test-1",
				Name:        "Short",
				Phase:       orchestrator.GroupPhasePending,
				SessionType: orchestrator.SessionTypeTripleShot,
			},
			progress:   GroupProgress{Completed: 1, Total: 3},
			collapsed:  false,
			isSelected: false,
			width:      80,
			checkLines: 1,
		},
		{
			name: "long name wraps to multiple lines",
			group: &orchestrator.InstanceGroup{
				ID:          "test-2",
				Name:        "This is a very long group name that should wrap to multiple lines in the sidebar",
				Phase:       orchestrator.GroupPhaseExecuting,
				SessionType: orchestrator.SessionTypeTripleShot,
			},
			progress:   GroupProgress{Completed: 2, Total: 5},
			collapsed:  false,
			isSelected: false,
			width:      40,
			checkLines: 2, // should be at least 2 lines
		},
		{
			name: "selected group styling",
			group: &orchestrator.InstanceGroup{
				ID:          "test-3",
				Name:        "Selected Group",
				Phase:       orchestrator.GroupPhaseCompleted,
				SessionType: orchestrator.SessionTypePlan,
			},
			progress:   GroupProgress{Completed: 3, Total: 3},
			collapsed:  true,
			isSelected: true,
			width:      60,
			checkLines: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderGroupHeaderWrapped(tt.group, tt.progress, tt.collapsed, tt.isSelected, tt.width)
			if len(result) < tt.checkLines {
				t.Errorf("RenderGroupHeaderWrapped() returned %d lines, want at least %d", len(result), tt.checkLines)
			}
			// First line should contain progress indicator
			if len(result) > 0 && !strings.Contains(result[0], "[") {
				t.Errorf("First line should contain progress indicator, got: %s", result[0])
			}
		})
	}
}

func TestRenderGroupHeader_WrappedOutput(t *testing.T) {
	group := &orchestrator.InstanceGroup{
		ID:          "test-1",
		Name:        "Test Group",
		Phase:       orchestrator.GroupPhasePending,
		SessionType: orchestrator.SessionTypeTripleShot,
	}
	progress := GroupProgress{Completed: 1, Total: 3}

	result := RenderGroupHeader(group, progress, false, false, 80)

	// Should be a single string (possibly with newlines)
	if result == "" {
		t.Error("RenderGroupHeader() returned empty string")
	}

	// Should contain the group name
	if !strings.Contains(result, "Test Group") {
		t.Errorf("RenderGroupHeader() should contain group name, got: %s", result)
	}

	// Should contain progress
	if !strings.Contains(result, "[1/3]") {
		t.Errorf("RenderGroupHeader() should contain progress [1/3], got: %s", result)
	}
}

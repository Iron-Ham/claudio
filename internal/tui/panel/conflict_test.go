package panel

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestConflictPanel_Render(t *testing.T) {
	tests := []struct {
		name     string
		state    *RenderState
		contains []string
		isEmpty  bool
	}{
		{
			name: "renders conflicts with instance info",
			state: &RenderState{
				Width:  80,
				Height: 50,
				Instances: []*orchestrator.Instance{
					{ID: "inst-1", Task: "Feature A"},
					{ID: "inst-2", Task: "Feature B"},
				},
				Conflicts: []conflict.FileConflict{
					{
						RelativePath: "src/main.go",
						Instances:    []string{"inst-1", "inst-2"},
						LastModified: time.Now(),
					},
				},
			},
			contains: []string{
				"src/main.go",
			},
			isEmpty: false,
		},
		{
			name: "returns empty when no conflicts",
			state: &RenderState{
				Width:     80,
				Height:    50,
				Conflicts: nil,
			},
			isEmpty: true,
		},
		{
			name: "returns empty when empty conflicts slice",
			state: &RenderState{
				Width:     80,
				Height:    50,
				Conflicts: []conflict.FileConflict{},
			},
			isEmpty: true,
		},
		{
			name: "invalid state returns error indicator",
			state: &RenderState{
				Width:  0,
				Height: 0,
			},
			contains: []string{"render error"},
			isEmpty:  false,
		},
	}

	panel := NewConflictPanel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := panel.Render(tt.state)

			if tt.isEmpty && result != "" {
				t.Errorf("expected empty result, got: %s", result)
			}
			if !tt.isEmpty && result == "" {
				t.Error("expected non-empty result, got empty")
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestConflictPanel_Height(t *testing.T) {
	panel := NewConflictPanel()

	// Test with conflicts
	state := &RenderState{
		Width:  80,
		Height: 50,
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Task 1"},
		},
		Conflicts: []conflict.FileConflict{
			{
				RelativePath: "file.go",
				Instances:    []string{"inst-1"},
			},
		},
	}

	panel.Render(state)

	if panel.Height() <= 0 {
		t.Errorf("Height() = %d, want positive value", panel.Height())
	}
}

func TestConflictPanel_HeightWithNoConflicts(t *testing.T) {
	panel := NewConflictPanel()

	state := &RenderState{
		Width:     80,
		Height:    50,
		Conflicts: nil,
	}

	panel.Render(state)

	if panel.Height() != 0 {
		t.Errorf("Height() = %d, want 0 with no conflicts", panel.Height())
	}
}

func TestNewConflictPanel(t *testing.T) {
	panel := NewConflictPanel()
	if panel == nil {
		t.Error("NewConflictPanel() returned nil")
	}
}

func TestCountNewlines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"no newlines", 0},
		{"one\n", 1},
		{"two\nlines\n", 2},
		{"\n\n\n", 3},
		{"line1\nline2\nline3", 2},
	}

	for _, tt := range tests {
		got := countNewlines(tt.input)
		if got != tt.want {
			t.Errorf("countNewlines(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestConflictPanel_MultipleConflicts(t *testing.T) {
	panel := NewConflictPanel()

	state := &RenderState{
		Width:  80,
		Height: 50,
		Instances: []*orchestrator.Instance{
			{ID: "inst-1", Task: "Feature A"},
			{ID: "inst-2", Task: "Feature B"},
			{ID: "inst-3", Task: "Feature C"},
		},
		Conflicts: []conflict.FileConflict{
			{
				RelativePath: "src/main.go",
				Instances:    []string{"inst-1", "inst-2"},
			},
			{
				RelativePath: "src/utils.go",
				Instances:    []string{"inst-2", "inst-3"},
			},
			{
				RelativePath: "src/config.go",
				Instances:    []string{"inst-1", "inst-3"},
			},
		},
	}

	result := panel.Render(state)

	expectedFiles := []string{"src/main.go", "src/utils.go", "src/config.go"}
	for _, file := range expectedFiles {
		if !strings.Contains(result, file) {
			t.Errorf("result missing conflict file %q", file)
		}
	}
}

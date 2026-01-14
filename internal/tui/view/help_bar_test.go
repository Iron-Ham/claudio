package view

import (
	"strings"
	"testing"
)

func TestRenderCommandModeHelp(t *testing.T) {
	tests := []struct {
		name     string
		state    *HelpBarState
		contains []string
	}{
		{
			name:     "nil state returns empty",
			state:    nil,
			contains: []string{},
		},
		{
			name: "renders command prompt with buffer",
			state: &HelpBarState{
				CommandMode:   true,
				CommandBuffer: "quit",
			},
			contains: []string{":", "quit", "Enter", "Esc"},
		},
		{
			name: "renders with empty buffer",
			state: &HelpBarState{
				CommandMode:   true,
				CommandBuffer: "",
			},
			contains: []string{":", "Enter", "Esc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderCommandModeHelp(tt.state)

			if tt.state == nil {
				if result != "" {
					t.Errorf("expected empty string for nil state, got: %s", result)
				}
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q, got: %s", want, result)
				}
			}
		})
	}
}

func TestRenderHelp(t *testing.T) {
	tests := []struct {
		name     string
		state    *HelpBarState
		contains []string
	}{
		{
			name:     "nil state returns empty",
			state:    nil,
			contains: []string{},
		},
		{
			name: "input mode shows input mode help",
			state: &HelpBarState{
				InputMode: true,
			},
			contains: []string{"INPUT MODE", "Ctrl+]"},
		},
		{
			name: "terminal focused shows terminal help",
			state: &HelpBarState{
				TerminalFocused: true,
				TerminalDirMode: "invoke",
			},
			contains: []string{"TERMINAL", "Ctrl+]", "invoke"},
		},
		{
			name: "terminal focused with worktree mode",
			state: &HelpBarState{
				TerminalFocused: true,
				TerminalDirMode: "worktree",
			},
			contains: []string{"TERMINAL", "worktree"},
		},
		{
			name: "diff view shows diff help",
			state: &HelpBarState{
				ShowDiff: true,
			},
			contains: []string{"DIFF VIEW", "j/k", "scroll"},
		},
		{
			name: "filter mode shows filter help",
			state: &HelpBarState{
				FilterMode: true,
			},
			contains: []string{"FILTER MODE", "toggle categories"},
		},
		{
			name: "search mode shows search help",
			state: &HelpBarState{
				SearchMode: true,
			},
			contains: []string{"SEARCH", "pattern", "regex"},
		},
		{
			name:     "normal mode shows default keys",
			state:    &HelpBarState{},
			contains: []string{"cmd", "scroll", "switch", "help", "quit"},
		},
		{
			name: "terminal visible shows hide option",
			state: &HelpBarState{
				TerminalVisible: true,
			},
			contains: []string{"hide"},
		},
		{
			name: "conflicts present shows conflict indicator",
			state: &HelpBarState{
				ConflictCount: 3,
			},
			contains: []string{"conflicts"},
		},
		{
			name: "search matches shows match count",
			state: &HelpBarState{
				SearchHasMatches:   true,
				SearchCurrentIndex: 2,
				SearchMatchCount:   5,
			},
			contains: []string{"3/5"}, // 2+1 = 3 (0-indexed to 1-indexed)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderHelp(tt.state)

			if tt.state == nil {
				if result != "" {
					t.Errorf("expected empty string for nil state, got: %s", result)
				}
				return
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("expected output to contain %q, got: %s", want, result)
				}
			}
		})
	}
}

func TestRenderTripleShotHelp(t *testing.T) {
	result := RenderTripleShotHelp()

	// Should contain standard navigation keys
	expectedKeys := []string{"cmd", "scroll", "switch", "search", "help", "quit"}
	for _, key := range expectedKeys {
		if !strings.Contains(result, key) {
			t.Errorf("expected triple-shot help to contain %q, got: %s", key, result)
		}
	}

	// Should not contain input mode key (triple-shot specific)
	if strings.Contains(result, "input") {
		t.Errorf("triple-shot help should not contain input key, got: %s", result)
	}
}

func TestHelpBarView(t *testing.T) {
	t.Run("NewHelpBarView creates non-nil view", func(t *testing.T) {
		v := NewHelpBarView()
		if v == nil {
			t.Error("NewHelpBarView returned nil")
		}
	})

	t.Run("view methods match package functions", func(t *testing.T) {
		v := NewHelpBarView()
		state := &HelpBarState{
			CommandMode:   true,
			CommandBuffer: "test",
		}

		viewResult := v.RenderCommandModeHelp(state)
		pkgResult := RenderCommandModeHelp(state)

		if viewResult != pkgResult {
			t.Errorf("view method and package function should return same result")
		}
	})
}

package view

import (
	"strings"
	"testing"
)

func TestModeIndicatorView_GetModeInfo_NilState(t *testing.T) {
	v := NewModeIndicatorView()
	info := v.GetModeInfo(nil)
	if info != nil {
		t.Error("GetModeInfo(nil) should return nil")
	}
}

func TestModeIndicatorView_GetModeInfo_NormalMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{}
	info := v.GetModeInfo(state)
	if info != nil {
		t.Error("GetModeInfo for normal mode should return nil (no indicator)")
	}
}

func TestModeIndicatorView_GetModeInfo_InputMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{InputMode: true}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo for input mode should not return nil")
	}
	if info.Label != "INPUT" {
		t.Errorf("GetModeInfo Label = %q, want %q", info.Label, "INPUT")
	}
	if !info.IsHighPriority {
		t.Error("Input mode should be high priority")
	}
}

func TestModeIndicatorView_GetModeInfo_TerminalMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{TerminalFocused: true}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo for terminal mode should not return nil")
	}
	if info.Label != "TERMINAL" {
		t.Errorf("GetModeInfo Label = %q, want %q", info.Label, "TERMINAL")
	}
	if !info.IsHighPriority {
		t.Error("Terminal mode should be high priority")
	}
}

func TestModeIndicatorView_GetModeInfo_SearchMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{SearchMode: true}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo for search mode should not return nil")
	}
	if info.Label != "SEARCH" {
		t.Errorf("GetModeInfo Label = %q, want %q", info.Label, "SEARCH")
	}
	if info.IsHighPriority {
		t.Error("Search mode should not be high priority")
	}
}

func TestModeIndicatorView_GetModeInfo_FilterMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{FilterMode: true}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo for filter mode should not return nil")
	}
	if info.Label != "FILTER" {
		t.Errorf("GetModeInfo Label = %q, want %q", info.Label, "FILTER")
	}
}

func TestModeIndicatorView_GetModeInfo_CommandMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{CommandMode: true}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo for command mode should not return nil")
	}
	if info.Label != "COMMAND" {
		t.Errorf("GetModeInfo Label = %q, want %q", info.Label, "COMMAND")
	}
}

func TestModeIndicatorView_GetModeInfo_TaskInputMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{AddingTask: true}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo for task input mode should not return nil")
	}
	if info.Label != "NEW TASK" {
		t.Errorf("GetModeInfo Label = %q, want %q", info.Label, "NEW TASK")
	}
}

func TestModeIndicatorView_GetModeInfo_Priority(t *testing.T) {
	// Test that InputMode takes precedence over other modes
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{
		InputMode:   true,
		CommandMode: true,
		SearchMode:  true,
	}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo should not return nil")
	}
	if info.Label != "INPUT" {
		t.Errorf("InputMode should take precedence, got Label = %q", info.Label)
	}
}

func TestModeIndicatorView_GetModeInfo_TerminalPriorityOverSearch(t *testing.T) {
	// Test that TerminalFocused takes precedence over search/command
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{
		TerminalFocused: true,
		SearchMode:      true,
		CommandMode:     true,
	}
	info := v.GetModeInfo(state)

	if info == nil {
		t.Fatal("GetModeInfo should not return nil")
	}
	if info.Label != "TERMINAL" {
		t.Errorf("TerminalFocused should take precedence over search/command, got Label = %q", info.Label)
	}
}

func TestModeIndicatorView_Render_NormalMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{}
	result := v.Render(state)

	if result != "" {
		t.Errorf("Render for normal mode should return empty string, got %q", result)
	}
}

func TestModeIndicatorView_Render_InputMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{InputMode: true}
	result := v.Render(state)

	if result == "" {
		t.Error("Render for input mode should not return empty string")
	}
	if !strings.Contains(result, "INPUT") {
		t.Errorf("Render result should contain 'INPUT', got %q", result)
	}
}

func TestModeIndicatorView_RenderWithExitHint_InputMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{InputMode: true}
	result := v.RenderWithExitHint(state)

	if result == "" {
		t.Error("RenderWithExitHint for input mode should not return empty string")
	}
	if !strings.Contains(result, "INPUT") {
		t.Errorf("RenderWithExitHint result should contain 'INPUT', got %q", result)
	}
	if !strings.Contains(result, "Ctrl+]") {
		t.Errorf("RenderWithExitHint for high priority mode should contain exit hint, got %q", result)
	}
}

func TestModeIndicatorView_RenderWithExitHint_SearchMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{SearchMode: true}
	result := v.RenderWithExitHint(state)

	if result == "" {
		t.Error("RenderWithExitHint for search mode should not return empty string")
	}
	// Search is not high priority, so no exit hint
	if strings.Contains(result, "Ctrl+]") {
		t.Errorf("RenderWithExitHint for non-high-priority mode should not contain Ctrl+] hint, got %q", result)
	}
}

func TestModeIndicatorView_RenderWithExitHint_NormalMode(t *testing.T) {
	v := NewModeIndicatorView()
	state := &ModeIndicatorState{}
	result := v.RenderWithExitHint(state)

	if result != "" {
		t.Errorf("RenderWithExitHint for normal mode should return empty string, got %q", result)
	}
}

// Test package-level convenience functions
func TestRenderModeIndicator(t *testing.T) {
	state := &ModeIndicatorState{CommandMode: true}
	result := RenderModeIndicator(state)

	if result == "" {
		t.Error("RenderModeIndicator should not return empty string for command mode")
	}
	if !strings.Contains(result, "COMMAND") {
		t.Errorf("RenderModeIndicator should contain 'COMMAND', got %q", result)
	}
}

func TestRenderModeIndicatorWithHint(t *testing.T) {
	state := &ModeIndicatorState{TerminalFocused: true}
	result := RenderModeIndicatorWithHint(state)

	if result == "" {
		t.Error("RenderModeIndicatorWithHint should not return empty string for terminal mode")
	}
	if !strings.Contains(result, "TERMINAL") {
		t.Errorf("RenderModeIndicatorWithHint should contain 'TERMINAL', got %q", result)
	}
	if !strings.Contains(result, "Ctrl+]") {
		t.Errorf("RenderModeIndicatorWithHint should contain exit hint for terminal mode, got %q", result)
	}
}

func TestGetCurrentModeInfo(t *testing.T) {
	state := &ModeIndicatorState{FilterMode: true}
	info := GetCurrentModeInfo(state)

	if info == nil {
		t.Fatal("GetCurrentModeInfo should not return nil for filter mode")
	}
	if info.Label != "FILTER" {
		t.Errorf("GetCurrentModeInfo Label = %q, want %q", info.Label, "FILTER")
	}
}

func TestModeIndicatorView_AllModes_HaveNonEmptyLabels(t *testing.T) {
	tests := []struct {
		name  string
		state *ModeIndicatorState
	}{
		{"InputMode", &ModeIndicatorState{InputMode: true}},
		{"TerminalFocused", &ModeIndicatorState{TerminalFocused: true}},
		{"SearchMode", &ModeIndicatorState{SearchMode: true}},
		{"FilterMode", &ModeIndicatorState{FilterMode: true}},
		{"CommandMode", &ModeIndicatorState{CommandMode: true}},
		{"AddingTask", &ModeIndicatorState{AddingTask: true}},
	}

	v := NewModeIndicatorView()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := v.GetModeInfo(tt.state)
			if info == nil {
				t.Fatalf("GetModeInfo for %s should not return nil", tt.name)
			}
			if info.Label == "" {
				t.Errorf("GetModeInfo for %s has empty label", tt.name)
			}
			// Verify that a style is set by rendering the label
			// lipgloss styles are not easily comparable, so we just verify the render works
			rendered := info.Style.Render(info.Label)
			if rendered == "" {
				t.Errorf("GetModeInfo for %s style produced empty render", tt.name)
			}
		})
	}
}

package filter

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew(t *testing.T) {
	f := New()

	// All categories should be enabled by default
	for _, cat := range Categories {
		if !f.IsCategoryEnabled(cat.Key) {
			t.Errorf("category %q should be enabled by default", cat.Key)
		}
	}

	// No custom pattern should be set
	if f.CustomPattern() != "" {
		t.Errorf("CustomPattern() = %q, want empty", f.CustomPattern())
	}

	if f.CustomRegex() != nil {
		t.Error("CustomRegex() should be nil by default")
	}
}

func TestNewWithCategories(t *testing.T) {
	categories := map[string]bool{
		"errors":   false,
		"warnings": true,
		"tools":    false,
	}

	f := NewWithCategories(categories)

	// Check specified categories
	if f.IsCategoryEnabled("errors") {
		t.Error("errors should be disabled")
	}
	if !f.IsCategoryEnabled("warnings") {
		t.Error("warnings should be enabled")
	}
	if f.IsCategoryEnabled("tools") {
		t.Error("tools should be disabled")
	}

	// Unspecified categories should default to enabled
	if !f.IsCategoryEnabled("thinking") {
		t.Error("thinking should default to enabled")
	}
	if !f.IsCategoryEnabled("progress") {
		t.Error("progress should default to enabled")
	}
}

func TestToggleCategory(t *testing.T) {
	f := New()

	// Initial state - all enabled
	if !f.IsCategoryEnabled("errors") {
		t.Error("errors should be enabled initially")
	}

	// Toggle off
	f.ToggleCategory("errors")
	if f.IsCategoryEnabled("errors") {
		t.Error("errors should be disabled after toggle")
	}

	// Toggle back on
	f.ToggleCategory("errors")
	if !f.IsCategoryEnabled("errors") {
		t.Error("errors should be enabled after second toggle")
	}
}

func TestToggleAll(t *testing.T) {
	f := New()

	// All enabled initially, so ToggleAll should disable all
	f.ToggleAll()
	for _, cat := range Categories {
		if f.IsCategoryEnabled(cat.Key) {
			t.Errorf("category %q should be disabled after ToggleAll", cat.Key)
		}
	}

	// ToggleAll again should enable all
	f.ToggleAll()
	for _, cat := range Categories {
		if !f.IsCategoryEnabled(cat.Key) {
			t.Errorf("category %q should be enabled after second ToggleAll", cat.Key)
		}
	}
}

func TestToggleAllPartial(t *testing.T) {
	f := New()

	// Disable one category
	f.ToggleCategory("errors")

	// ToggleAll with partial state should enable all
	f.ToggleAll()
	for _, cat := range Categories {
		if !f.IsCategoryEnabled(cat.Key) {
			t.Errorf("category %q should be enabled after ToggleAll from partial state", cat.Key)
		}
	}
}

func TestAllEnabled(t *testing.T) {
	f := New()

	if !f.AllEnabled() {
		t.Error("AllEnabled() should return true when all categories are enabled")
	}

	f.ToggleCategory("errors")
	if f.AllEnabled() {
		t.Error("AllEnabled() should return false when a category is disabled")
	}
}

func TestCustomPattern(t *testing.T) {
	f := New()

	f.SetCustomPattern("test")
	if f.CustomPattern() != "test" {
		t.Errorf("CustomPattern() = %q, want %q", f.CustomPattern(), "test")
	}
	if f.CustomRegex() == nil {
		t.Error("CustomRegex() should not be nil after setting pattern")
	}

	// Test case-insensitive matching
	if !f.CustomRegex().MatchString("TEST") {
		t.Error("regex should match case-insensitively")
	}
	if !f.CustomRegex().MatchString("Test") {
		t.Error("regex should match case-insensitively")
	}
}

func TestInvalidCustomPattern(t *testing.T) {
	f := New()

	// Invalid regex pattern
	f.SetCustomPattern("[")
	if f.CustomRegex() != nil {
		t.Error("CustomRegex() should be nil for invalid pattern")
	}
	// Pattern should still be stored
	if f.CustomPattern() != "[" {
		t.Errorf("CustomPattern() = %q, want %q", f.CustomPattern(), "[")
	}
}

func TestClearCustomPattern(t *testing.T) {
	f := New()

	f.SetCustomPattern("test")
	f.ClearCustomPattern()

	if f.CustomPattern() != "" {
		t.Errorf("CustomPattern() = %q, want empty after clear", f.CustomPattern())
	}
	if f.CustomRegex() != nil {
		t.Error("CustomRegex() should be nil after clear")
	}
}

func TestAppendToPattern(t *testing.T) {
	f := New()

	f.AppendToPattern("t")
	f.AppendToPattern("e")
	f.AppendToPattern("s")
	f.AppendToPattern("t")

	if f.CustomPattern() != "test" {
		t.Errorf("CustomPattern() = %q, want %q", f.CustomPattern(), "test")
	}
}

func TestBackspacePattern(t *testing.T) {
	f := New()

	f.SetCustomPattern("test")
	f.BackspacePattern()

	if f.CustomPattern() != "tes" {
		t.Errorf("CustomPattern() = %q, want %q", f.CustomPattern(), "tes")
	}

	// Backspace on empty pattern should be safe
	f.ClearCustomPattern()
	f.BackspacePattern()
	if f.CustomPattern() != "" {
		t.Errorf("CustomPattern() = %q, want empty", f.CustomPattern())
	}
}

func TestHasActiveFilter(t *testing.T) {
	f := New()

	// No active filter initially
	if f.HasActiveFilter() {
		t.Error("HasActiveFilter() should return false initially")
	}

	// Disable a category
	f.ToggleCategory("errors")
	if !f.HasActiveFilter() {
		t.Error("HasActiveFilter() should return true when category disabled")
	}

	// Re-enable and set custom pattern
	f.ToggleCategory("errors")
	f.SetCustomPattern("test")
	if !f.HasActiveFilter() {
		t.Error("HasActiveFilter() should return true when custom pattern set")
	}
}

func TestApplyNoFilter(t *testing.T) {
	f := New()

	input := "line 1\nline 2\nline 3"
	output := f.Apply(input)

	if output != input {
		t.Errorf("Apply() should return input unchanged when no filter active")
	}
}

func TestApplyCustomPattern(t *testing.T) {
	f := New()
	f.SetCustomPattern("error")

	input := "normal line\nerror occurred\nanother line\nError: something"
	output := f.Apply(input)

	expected := "error occurred\nError: something"
	if output != expected {
		t.Errorf("Apply() = %q, want %q", output, expected)
	}
}

func TestApplyCategoryFilters(t *testing.T) {
	tests := []struct {
		name             string
		disableCategory  string
		input            string
		shouldBeFiltered string // Line that should be filtered out
	}{
		{
			name:             "filter errors",
			disableCategory:  "errors",
			input:            "normal line\nError: something failed\nok",
			shouldBeFiltered: "Error: something failed",
		},
		{
			name:             "filter warnings",
			disableCategory:  "warnings",
			input:            "normal line\nwarning: deprecated\nok",
			shouldBeFiltered: "warning: deprecated",
		},
		{
			name:             "filter tools",
			disableCategory:  "tools",
			input:            "normal line\nread file foo.txt\nok",
			shouldBeFiltered: "read file foo.txt",
		},
		{
			name:             "filter thinking",
			disableCategory:  "thinking",
			input:            "normal line\nlet me think about this\nok",
			shouldBeFiltered: "let me think about this",
		},
		{
			name:             "filter progress",
			disableCategory:  "progress",
			input:            "normal line\nloading...\nok",
			shouldBeFiltered: "loading...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := New()
			f.ToggleCategory(tt.disableCategory)

			output := f.Apply(tt.input)

			if output == tt.input {
				t.Error("Apply() should filter output when category disabled")
			}
			if containsLine(output, tt.shouldBeFiltered) {
				t.Errorf("Output should not contain filtered line %q", tt.shouldBeFiltered)
			}
		})
	}
}

func containsLine(output, line string) bool {
	lines := splitLines(output)
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func TestShouldShowLineErrors(t *testing.T) {
	f := New()
	f.ToggleCategory("errors")

	tests := []struct {
		line string
		show bool
	}{
		{"normal line", true},
		{"Error occurred", false},
		{"error: something", false},
		{"failed to connect", false},
		{"exception thrown", false},
		{"panic: runtime error", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := f.ShouldShowLine(tt.line); got != tt.show {
				t.Errorf("ShouldShowLine(%q) = %v, want %v", tt.line, got, tt.show)
			}
		})
	}
}

func TestShouldShowLineTools(t *testing.T) {
	f := New()
	f.ToggleCategory("tools")

	tests := []struct {
		line string
		show bool
	}{
		{"normal line", true},
		{"read file foo.txt", false},
		{"Write file bar.txt", false},
		{"running bash command", false},
		{"bash echo hello", false},
		{"  function(arg) → result", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := f.ShouldShowLine(tt.line); got != tt.show {
				t.Errorf("ShouldShowLine(%q) = %v, want %v", tt.line, got, tt.show)
			}
		})
	}
}

func TestShouldShowLineProgress(t *testing.T) {
	f := New()
	f.ToggleCategory("progress")

	tests := []struct {
		line string
		show bool
	}{
		{"normal line", true},
		{"loading...", false},
		{"✓ done", false},
		{"█░░░░░░░░░", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := f.ShouldShowLine(tt.line); got != tt.show {
				t.Errorf("ShouldShowLine(%q) = %v, want %v", tt.line, got, tt.show)
			}
		})
	}
}

func TestShouldShowLineCustomRegexPrecedence(t *testing.T) {
	f := New()
	f.ToggleCategory("errors")  // Disable errors
	f.SetCustomPattern("error") // But set custom pattern matching "error"

	// Custom pattern should take precedence - line should be shown because it matches the pattern
	if !f.ShouldShowLine("error occurred") {
		t.Error("custom pattern should take precedence over category filters")
	}
}

func TestHandleKeyExit(t *testing.T) {
	tests := []struct {
		key      string
		exitMode bool
	}{
		{"esc", true},
		{"F", true},
		{"q", true},
		{"e", false},
		{"a", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			f := New()
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if tt.key == "esc" {
				msg = tea.KeyMsg{Type: tea.KeyEscape}
			}

			result := f.HandleKey(msg)
			if result.ExitMode != tt.exitMode {
				t.Errorf("HandleKey(%q).ExitMode = %v, want %v", tt.key, result.ExitMode, tt.exitMode)
			}
		})
	}
}

func TestHandleKeyCategoryToggles(t *testing.T) {
	tests := []struct {
		key      string
		category string
	}{
		{"e", "errors"},
		{"1", "errors"},
		{"w", "warnings"},
		{"2", "warnings"},
		{"t", "tools"},
		{"3", "tools"},
		{"h", "thinking"},
		{"4", "thinking"},
		{"p", "progress"},
		{"5", "progress"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			f := New()
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}

			// Initially enabled
			if !f.IsCategoryEnabled(tt.category) {
				t.Errorf("category %q should be enabled initially", tt.category)
			}

			f.HandleKey(msg)

			// After handling key, should be disabled
			if f.IsCategoryEnabled(tt.category) {
				t.Errorf("category %q should be disabled after key %q", tt.category, tt.key)
			}
		})
	}
}

func TestHandleKeyToggleAll(t *testing.T) {
	f := New()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}

	f.HandleKey(msg)

	for _, cat := range Categories {
		if f.IsCategoryEnabled(cat.Key) {
			t.Errorf("category %q should be disabled after 'a' key", cat.Key)
		}
	}
}

func TestHandleKeyClearCustom(t *testing.T) {
	f := New()
	f.SetCustomPattern("test")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")}
	f.HandleKey(msg)

	if f.CustomPattern() != "" {
		t.Errorf("CustomPattern() = %q, want empty after 'c' key", f.CustomPattern())
	}
}

func TestHandleKeyBackspace(t *testing.T) {
	f := New()
	f.SetCustomPattern("test")

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	f.HandleKey(msg)

	if f.CustomPattern() != "tes" {
		t.Errorf("CustomPattern() = %q, want %q after backspace", f.CustomPattern(), "tes")
	}
}

func TestHandleKeySpace(t *testing.T) {
	f := New()
	f.SetCustomPattern("hello")

	msg := tea.KeyMsg{Type: tea.KeySpace}
	f.HandleKey(msg)

	if f.CustomPattern() != "hello " {
		t.Errorf("CustomPattern() = %q, want %q after space", f.CustomPattern(), "hello ")
	}
}

func TestHandleKeyRunes(t *testing.T) {
	f := New()

	// Type characters that are NOT shortcuts
	for _, char := range []string{"x", "y", "z", "0"} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(char)}
		f.HandleKey(msg)
	}

	if f.CustomPattern() != "xyz0" {
		t.Errorf("CustomPattern() = %q, want %q", f.CustomPattern(), "xyz0")
	}
}

func TestHandleKeyRunesIgnoresShortcuts(t *testing.T) {
	f := New()

	// These characters are shortcuts and should not be added to pattern
	shortcuts := []string{"e", "w", "t", "h", "p", "a", "c"}

	for _, char := range shortcuts {
		// Save initial state
		f.ClearCustomPattern()

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(char)}
		f.HandleKey(msg)

		// Pattern should still be empty (character not added)
		if f.CustomPattern() != "" {
			t.Errorf("shortcut %q should not be added to pattern", char)
		}
	}
}

func TestCategories(t *testing.T) {
	f := New()

	cats := f.Categories()

	// Modifying returned map should not affect filter
	cats["errors"] = false

	if !f.IsCategoryEnabled("errors") {
		t.Error("modifying returned Categories map should not affect filter state")
	}
}

func TestSetCategories(t *testing.T) {
	f := New()

	f.SetCategories(map[string]bool{
		"errors":   false,
		"warnings": false,
	})

	if f.IsCategoryEnabled("errors") {
		t.Error("errors should be disabled after SetCategories")
	}
	if f.IsCategoryEnabled("warnings") {
		t.Error("warnings should be disabled after SetCategories")
	}
	// Other categories should remain unchanged
	if !f.IsCategoryEnabled("tools") {
		t.Error("tools should still be enabled")
	}
}

func TestRenderPanel(t *testing.T) {
	f := New()

	panel := RenderPanel(f, 80)

	// Basic sanity checks - panel should contain expected elements
	if panel == "" {
		t.Error("RenderPanel() returned empty string")
	}

	// Panel should contain title
	if len(panel) < 10 {
		t.Error("RenderPanel() returned suspiciously short content")
	}
}

func TestRenderPanelWithCustomPattern(t *testing.T) {
	f := New()
	f.SetCustomPattern("test")

	panel := RenderPanel(f, 80)

	// Panel should render without error
	if panel == "" {
		t.Error("RenderPanel() with custom pattern returned empty string")
	}
}

func TestRenderPanelWithDisabledCategory(t *testing.T) {
	f := New()
	f.ToggleCategory("errors")

	panel := RenderPanel(f, 80)

	// Panel should render without error
	if panel == "" {
		t.Error("RenderPanel() with disabled category returned empty string")
	}
}

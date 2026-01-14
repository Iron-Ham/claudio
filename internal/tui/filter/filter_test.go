package filter

import (
	"slices"
	"strings"
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
	lines := strings.Split(output, "\n")
	return slices.Contains(lines, line)
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

// Additional edge case tests

func TestBackspacePatternToEmpty(t *testing.T) {
	// This tests the compileRegex branch where pattern becomes empty
	f := New()
	f.SetCustomPattern("a")

	// Backspace to empty string
	f.BackspacePattern()

	if f.CustomPattern() != "" {
		t.Errorf("CustomPattern() = %q, want empty after backspace", f.CustomPattern())
	}
	if f.CustomRegex() != nil {
		t.Error("CustomRegex() should be nil when pattern becomes empty via backspace")
	}
}

func TestShouldShowLineWarnings(t *testing.T) {
	f := New()
	f.ToggleCategory("warnings")

	tests := []struct {
		line string
		show bool
	}{
		{"normal line", true},
		{"warning: something", false},
		{"Warning message", false},
		{"WARN: deprecated", false},
		{"warn: use X instead", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := f.ShouldShowLine(tt.line); got != tt.show {
				t.Errorf("ShouldShowLine(%q) = %v, want %v", tt.line, got, tt.show)
			}
		})
	}
}

func TestShouldShowLineThinking(t *testing.T) {
	f := New()
	f.ToggleCategory("thinking")

	tests := []struct {
		line string
		show bool
	}{
		{"normal line", true},
		{"I'm thinking about this", false},
		{"Let me check the code", false},
		{"I'll review the changes", false},
		{"I will implement this", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := f.ShouldShowLine(tt.line); got != tt.show {
				t.Errorf("ShouldShowLine(%q) = %v, want %v", tt.line, got, tt.show)
			}
		})
	}
}

func TestApplyEmptyInput(t *testing.T) {
	f := New()
	f.SetCustomPattern("test")

	// Empty input should return empty output
	output := f.Apply("")
	if output != "" {
		t.Errorf("Apply(\"\") = %q, want empty", output)
	}
}

func TestApplySingleLineNoMatch(t *testing.T) {
	f := New()
	f.SetCustomPattern("error")

	// Single line that doesn't match
	output := f.Apply("normal line")
	if output != "" {
		t.Errorf("Apply() = %q, want empty when no match", output)
	}
}

func TestHandleKeyUnhandledType(t *testing.T) {
	f := New()

	// Test an unhandled key type (e.g., KeyUp)
	msg := tea.KeyMsg{Type: tea.KeyUp}
	result := f.HandleKey(msg)

	// Should return empty result (no exit, no changes)
	if result.ExitMode {
		t.Error("unhandled key should not exit mode")
	}
}

func TestNewWithCategoriesEmpty(t *testing.T) {
	// Pass empty map - all categories should default to enabled
	f := NewWithCategories(map[string]bool{})

	for _, cat := range Categories {
		if !f.IsCategoryEnabled(cat.Key) {
			t.Errorf("category %q should default to enabled with empty map", cat.Key)
		}
	}
}

func TestToggleCategoryUnknown(t *testing.T) {
	f := New()

	// Toggle an unknown category - should not panic
	f.ToggleCategory("unknown")

	// After toggling unknown, it should be true (toggled from false default)
	if !f.IsCategoryEnabled("unknown") {
		t.Error("unknown category should be true after first toggle")
	}
}

func TestSetCategoriesPartial(t *testing.T) {
	f := New()

	// Disable one category first
	f.ToggleCategory("errors")

	// Set only warnings - errors should remain disabled
	f.SetCategories(map[string]bool{
		"warnings": false,
	})

	if f.IsCategoryEnabled("errors") {
		t.Error("errors should remain disabled")
	}
	if f.IsCategoryEnabled("warnings") {
		t.Error("warnings should be disabled after SetCategories")
	}
}

func TestApplyMultipleCategoriesDisabled(t *testing.T) {
	f := New()
	f.ToggleCategory("errors")
	f.ToggleCategory("warnings")

	input := "normal line\nerror occurred\nwarning issued\nok"
	output := f.Apply(input)

	if containsLine(output, "error occurred") {
		t.Error("output should not contain error line")
	}
	if containsLine(output, "warning issued") {
		t.Error("output should not contain warning line")
	}
	if !containsLine(output, "normal line") {
		t.Error("output should contain normal line")
	}
	if !containsLine(output, "ok") {
		t.Error("output should contain ok line")
	}
}

func TestCustomPatternRegexSpecialChars(t *testing.T) {
	f := New()

	// Test regex with special characters that are valid
	f.SetCustomPattern("\\d+") // Match digits
	if f.CustomRegex() == nil {
		t.Error("CustomRegex() should compile valid regex with special chars")
	}
	if !f.CustomRegex().MatchString("123") {
		t.Error("regex should match digits")
	}
	if f.CustomRegex().MatchString("abc") {
		t.Error("regex should not match non-digits")
	}
}

func TestApplyWithRegexPattern(t *testing.T) {
	f := New()
	f.SetCustomPattern("^\\[.*\\]") // Lines starting with brackets

	input := "[INFO] message\nnormal line\n[ERROR] bad thing\nplain text"
	output := f.Apply(input)

	if !containsLine(output, "[INFO] message") {
		t.Error("output should contain [INFO] line")
	}
	if !containsLine(output, "[ERROR] bad thing") {
		t.Error("output should contain [ERROR] line")
	}
	if containsLine(output, "normal line") {
		t.Error("output should not contain normal line")
	}
	if containsLine(output, "plain text") {
		t.Error("output should not contain plain text line")
	}
}

func TestShouldShowLineToolsWithParenthesis(t *testing.T) {
	f := New()
	f.ToggleCategory("tools")

	// Test the specific pattern: lines starting with spaces and containing (
	tests := []struct {
		line string
		show bool
	}{
		{"  someFunc(arg)", false},       // Tool call pattern
		{"someFunc(arg)", true},          // Not indented - shows
		{"  plain text no parens", true}, // Indented but no parens
		{"  has arrow → here", false},    // Tool result pattern
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			if got := f.ShouldShowLine(tt.line); got != tt.show {
				t.Errorf("ShouldShowLine(%q) = %v, want %v", tt.line, got, tt.show)
			}
		})
	}
}

func TestCategoriesGlobalVariable(t *testing.T) {
	// Ensure Categories global has expected length and structure
	if len(Categories) != 5 {
		t.Errorf("Categories should have 5 items, got %d", len(Categories))
	}

	expectedKeys := []string{"errors", "warnings", "tools", "thinking", "progress"}
	for i, key := range expectedKeys {
		if Categories[i].Key != key {
			t.Errorf("Categories[%d].Key = %q, want %q", i, Categories[i].Key, key)
		}
	}
}

func TestAllEnabledEmptyCategories(t *testing.T) {
	// Create filter with empty categories map directly
	f := &Filter{
		categories: make(map[string]bool),
	}

	// Empty map should return true (no disabled categories)
	if !f.AllEnabled() {
		t.Error("AllEnabled() should return true for empty categories map")
	}
}

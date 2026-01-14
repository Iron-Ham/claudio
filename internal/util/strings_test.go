package util

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string truncated",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "very small maxLen returns ellipsis",
			input:    "hello",
			maxLen:   3,
			expected: "...",
		},
		{
			name:     "maxLen of 2 returns ellipsis",
			input:    "hello",
			maxLen:   2,
			expected: "...",
		},
		{
			name:     "maxLen of 1 returns ellipsis",
			input:    "hello",
			maxLen:   1,
			expected: "...",
		},
		{
			name:     "maxLen of 0 returns ellipsis",
			input:    "hello",
			maxLen:   0,
			expected: "...",
		},
		{
			name:     "negative maxLen returns ellipsis",
			input:    "hello",
			maxLen:   -5,
			expected: "...",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "maxLen of 4 shows one char plus ellipsis",
			input:    "hello",
			maxLen:   4,
			expected: "h...",
		},
		{
			name:     "unicode characters counted correctly",
			input:    "日本語テスト",
			maxLen:   5,
			expected: "日本...",
		},
		{
			name:     "unicode exact length unchanged",
			input:    "日本語",
			maxLen:   3,
			expected: "...",
		},
		{
			name:     "unicode short string unchanged",
			input:    "日本語",
			maxLen:   10,
			expected: "日本語",
		},
		{
			name:     "mixed ascii and unicode",
			input:    "hello日本語world",
			maxLen:   10,
			expected: "hello日本...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateString(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("TruncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

func TestTruncateANSI(t *testing.T) {
	// Create styled strings for testing
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	boldStyle := lipgloss.NewStyle().Bold(true)

	tests := []struct {
		name     string
		input    string
		maxWidth int
		check    func(t *testing.T, result string)
	}{
		{
			name:     "short plain string unchanged",
			input:    "hello",
			maxWidth: 10,
			check: func(t *testing.T, result string) {
				if result != "hello" {
					t.Errorf("expected 'hello', got %q", result)
				}
			},
		},
		{
			name:     "plain string truncated",
			input:    "hello world",
			maxWidth: 8,
			check: func(t *testing.T, result string) {
				width := lipgloss.Width(result)
				if width > 8 {
					t.Errorf("result width %d exceeds maxWidth 8", width)
				}
				if result != "hello..." {
					t.Errorf("expected 'hello...', got %q", result)
				}
			},
		},
		{
			name:     "very small maxWidth returns ellipsis",
			input:    "hello",
			maxWidth: 3,
			check: func(t *testing.T, result string) {
				if result != "..." {
					t.Errorf("expected '...', got %q", result)
				}
			},
		},
		{
			name:     "maxWidth of 2 returns ellipsis",
			input:    "hello",
			maxWidth: 2,
			check: func(t *testing.T, result string) {
				if result != "..." {
					t.Errorf("expected '...', got %q", result)
				}
			},
		},
		{
			name:     "styled string preserves style when not truncated",
			input:    redStyle.Render("hi"),
			maxWidth: 10,
			check: func(t *testing.T, result string) {
				if lipgloss.Width(result) > 10 {
					t.Errorf("result width exceeds maxWidth")
				}
				// The styled string should be preserved
				if result != redStyle.Render("hi") {
					t.Errorf("styled string was modified when it shouldn't be")
				}
			},
		},
		{
			name:     "styled string truncated respects width",
			input:    redStyle.Render("hello world"),
			maxWidth: 8,
			check: func(t *testing.T, result string) {
				width := lipgloss.Width(result)
				if width > 8 {
					t.Errorf("result width %d exceeds maxWidth 8", width)
				}
			},
		},
		{
			name:     "bold styled string truncated",
			input:    boldStyle.Render("hello world"),
			maxWidth: 8,
			check: func(t *testing.T, result string) {
				width := lipgloss.Width(result)
				if width > 8 {
					t.Errorf("result width %d exceeds maxWidth 8", width)
				}
			},
		},
		{
			name:     "empty string unchanged",
			input:    "",
			maxWidth: 10,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			},
		},
		{
			name:     "wide characters counted by visual width",
			input:    "日本語テスト",
			maxWidth: 8,
			check: func(t *testing.T, result string) {
				width := lipgloss.Width(result)
				if width > 8 {
					t.Errorf("result width %d exceeds maxWidth 8", width)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateANSI(tt.input, tt.maxWidth)
			tt.check(t, result)
		})
	}
}

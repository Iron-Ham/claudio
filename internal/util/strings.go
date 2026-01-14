// Package util provides shared utility functions used across the codebase.
package util

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TruncateString truncates a string to maxLen runes, adding "..." if truncated.
// This is a simple truncation that does not account for ANSI escape codes or
// wide characters. For terminal output with styling, use TruncateANSI instead.
func TruncateString(s string, maxLen int) string {
	if maxLen <= 3 {
		return "..."
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// TruncateANSI truncates a string to maxWidth visual columns, adding "..." if truncated.
// This function properly handles ANSI escape codes and wide characters, making it
// suitable for terminal output with styling.
func TruncateANSI(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Use ANSI-aware truncation to preserve escape sequences
	// ansi.Truncate includes the tail in the final width calculation
	return ansi.Truncate(s, maxWidth, "...")
}

package ultraplan

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// truncate truncates a string to max length, adding ellipsis if needed.
// Uses runes to properly handle Unicode characters.
func truncate(s string, max int) string {
	if max <= 3 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

// wrapAtWordBoundary extracts up to maxLen characters from the rune slice,
// breaking at the last space if possible to avoid splitting words. If no
// space is found, or if the last space is within the first 1/3 of maxLen
// (to avoid very short lines), it falls back to character-based breaking.
func wrapAtWordBoundary(runes []rune, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(runes) <= maxLen {
		return string(runes)
	}

	// Look for the last space within the available length
	lastSpace := -1
	for i := maxLen - 1; i >= 0; i-- {
		if runes[i] == ' ' {
			lastSpace = i
			break
		}
	}

	// If we found a space and it's not too early in the string (at least 1/3 of available space),
	// break at the word boundary
	if lastSpace > maxLen/3 {
		return string(runes[:lastSpace])
	}

	// No suitable word boundary found, fall back to character-based breaking
	return string(runes[:maxLen])
}

// trimLeadingSpaces removes leading space characters from a rune slice.
func trimLeadingSpaces(runes []rune) []rune {
	for len(runes) > 0 && runes[0] == ' ' {
		runes = runes[1:]
	}
	return runes
}

// padToWidth pads a string with spaces to reach the target width.
func padToWidth(s string, width int) string {
	return PadToWidth(s, width)
}

// PadToWidth pads a string with spaces to reach the target width.
// This is exported for use in the parent view package for backward compatibility.
func PadToWidth(s string, width int) string {
	currentWidth := lipgloss.Width(s)
	if currentWidth >= width {
		return s
	}
	return s + strings.Repeat(" ", width-currentWidth)
}

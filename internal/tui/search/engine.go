// Package search provides search functionality for the TUI output buffers.
package search

import (
	"regexp"
	"strings"
)

// Result represents a single search match with its position information.
type Result struct {
	// LineNumber is the 0-indexed line number containing the match
	LineNumber int
	// StartIndex is the byte offset within the line where the match begins
	StartIndex int
	// EndIndex is the byte offset within the line where the match ends
	EndIndex int
}

// Engine provides search functionality for text buffers.
// It supports both literal and regex search patterns, tracks results,
// and manages navigation through matches.
type Engine struct {
	pattern string         // Current search pattern
	regex   *regexp.Regexp // Compiled regex (nil for no active search)
	matches []Result       // All found matches
	current int            // Index of current match in matches slice
	content string         // Content being searched (cached for re-searching)
}

// NewEngine creates a new search engine instance.
func NewEngine() *Engine {
	return &Engine{
		matches: make([]Result, 0),
	}
}

// Search performs a search on the given content with the specified pattern.
// Patterns starting with "r:" are treated as regex; otherwise literal (case-insensitive).
// Returns the list of results.
func (e *Engine) Search(pattern string, content string) []Result {
	e.content = content
	e.pattern = pattern
	e.matches = nil
	e.current = 0
	e.regex = nil

	if pattern == "" || content == "" {
		return nil
	}

	// Compile the pattern
	var re *regexp.Regexp
	var err error

	if strings.HasPrefix(pattern, "r:") {
		// Regex mode: pattern after "r:" prefix
		regexPattern := pattern[2:]
		if regexPattern == "" {
			return nil
		}
		re, err = regexp.Compile("(?i)" + regexPattern)
		if err != nil {
			// Invalid regex, no matches
			return nil
		}
	} else {
		// Literal mode: escape special characters, case-insensitive
		re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(pattern))
	}

	e.regex = re

	// Find all matches by line
	lines := strings.Split(content, "\n")
	for lineNum, line := range lines {
		indices := re.FindAllStringIndex(line, -1)
		for _, idx := range indices {
			e.matches = append(e.matches, Result{
				LineNumber: lineNum,
				StartIndex: idx[0],
				EndIndex:   idx[1],
			})
		}
	}

	return e.matches
}

// Next moves to the next search result and returns it.
// Returns nil if there are no matches.
// Navigation wraps around to the first match after the last.
func (e *Engine) Next() *Result {
	if len(e.matches) == 0 {
		return nil
	}

	e.current = (e.current + 1) % len(e.matches)
	return &e.matches[e.current]
}

// Previous moves to the previous search result and returns it.
// Returns nil if there are no matches.
// Navigation wraps around to the last match before the first.
func (e *Engine) Previous() *Result {
	if len(e.matches) == 0 {
		return nil
	}

	e.current = (e.current - 1 + len(e.matches)) % len(e.matches)
	return &e.matches[e.current]
}

// Current returns the current search result without changing position.
// Returns nil if there are no matches.
func (e *Engine) Current() *Result {
	if len(e.matches) == 0 || e.current >= len(e.matches) {
		return nil
	}
	return &e.matches[e.current]
}

// CurrentIndex returns the index of the current match (0-indexed).
// Returns -1 if there are no matches.
func (e *Engine) CurrentIndex() int {
	if len(e.matches) == 0 {
		return -1
	}
	return e.current
}

// Clear resets all search state.
func (e *Engine) Clear() {
	e.pattern = ""
	e.regex = nil
	e.matches = nil
	e.current = 0
	e.content = ""
}

// Pattern returns the current search pattern.
func (e *Engine) Pattern() string {
	return e.pattern
}

// Regex returns the compiled regex for the current search.
// Returns nil if no search is active or the pattern was invalid.
func (e *Engine) Regex() *regexp.Regexp {
	return e.regex
}

// Results returns all current search results.
func (e *Engine) Results() []Result {
	return e.matches
}

// MatchCount returns the total number of matches found.
func (e *Engine) MatchCount() int {
	return len(e.matches)
}

// HasMatches returns true if there are any search matches.
func (e *Engine) HasMatches() bool {
	return len(e.matches) > 0
}

// MatchingLines returns the unique line numbers that contain matches.
// The returned slice is sorted in ascending order.
func (e *Engine) MatchingLines() []int {
	if len(e.matches) == 0 {
		return nil
	}

	// Use a map to track unique lines
	seen := make(map[int]bool)
	var lines []int

	for _, m := range e.matches {
		if !seen[m.LineNumber] {
			seen[m.LineNumber] = true
			lines = append(lines, m.LineNumber)
		}
	}

	return lines
}

// Highlight applies highlighting to text based on the current search pattern.
// It takes the text to highlight and two style functions:
// - matchStyle: applied to non-current matches
// - currentStyle: applied to the current match (if isCurrentLine is true)
// The isCurrentLine parameter indicates if this line contains the current match.
// Returns the text with highlighting applied.
func (e *Engine) Highlight(text string, isCurrentLine bool, matchStyle, currentStyle func(string) string) string {
	if e.regex == nil || matchStyle == nil {
		return text
	}

	// Use default style if currentStyle not provided
	if currentStyle == nil {
		currentStyle = matchStyle
	}

	matches := e.regex.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return text
	}

	var result strings.Builder
	lastEnd := 0

	for i, match := range matches {
		// Add text before this match
		result.WriteString(text[lastEnd:match[0]])

		// Highlight the match
		matchText := text[match[0]:match[1]]
		if isCurrentLine && i == 0 {
			// First match on current line gets current-match style
			result.WriteString(currentStyle(matchText))
		} else {
			result.WriteString(matchStyle(matchText))
		}
		lastEnd = match[1]
	}

	// Add remaining text after last match
	result.WriteString(text[lastEnd:])

	return result.String()
}

// HighlightLine is a convenience method that highlights all matches in a line.
// It determines if the line is the current match line based on lineNumber.
func (e *Engine) HighlightLine(text string, lineNumber int, matchStyle, currentStyle func(string) string) string {
	isCurrentLine := false
	if current := e.Current(); current != nil {
		isCurrentLine = current.LineNumber == lineNumber
	}
	return e.Highlight(text, isCurrentLine, matchStyle, currentStyle)
}

// SetCurrent sets the current match index directly.
// This is useful for jumping to a specific match.
// The index is bounds-checked to stay within valid range.
func (e *Engine) SetCurrent(index int) {
	if len(e.matches) == 0 {
		e.current = 0
		return
	}

	if index < 0 {
		e.current = 0
	} else if index >= len(e.matches) {
		e.current = len(e.matches) - 1
	} else {
		e.current = index
	}
}

// JumpToLine sets the current match to the first match on or after the given line.
// Returns true if a match was found, false otherwise.
func (e *Engine) JumpToLine(lineNumber int) bool {
	for i, m := range e.matches {
		if m.LineNumber >= lineNumber {
			e.current = i
			return true
		}
	}
	return false
}

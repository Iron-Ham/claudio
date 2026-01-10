package tui

import (
	"regexp"
	"strings"
)

// SearchState encapsulates all search-related state for the TUI.
// This includes the search mode flag, pattern, compiled regex, match positions,
// and current match index for navigation.
type SearchState struct {
	// mode indicates whether search mode is active (user is typing a pattern)
	mode bool

	// pattern is the current search pattern entered by the user
	pattern string

	// regex is the compiled regex (nil for literal search or invalid pattern)
	regex *regexp.Regexp

	// matches contains line numbers that match the search pattern
	matches []int

	// current is the current match index for n/N navigation
	current int

	// outputScroll tracks the scroll position for search navigation
	// This is separate from the per-instance output scroll state
	outputScroll int
}

// NewSearchState creates a new SearchState with default values
func NewSearchState() *SearchState {
	return &SearchState{}
}

// IsActive returns true if search mode is currently active
func (s *SearchState) IsActive() bool {
	if s == nil {
		return false
	}
	return s.mode
}

// SetActive enables or disables search mode
func (s *SearchState) SetActive(active bool) {
	if s == nil {
		return
	}
	s.mode = active
}

// Pattern returns the current search pattern
func (s *SearchState) Pattern() string {
	if s == nil {
		return ""
	}
	return s.pattern
}

// SetPattern sets the search pattern
func (s *SearchState) SetPattern(pattern string) {
	if s == nil {
		return
	}
	s.pattern = pattern
}

// AppendToPattern appends text to the current search pattern
func (s *SearchState) AppendToPattern(text string) {
	if s == nil {
		return
	}
	s.pattern += text
}

// TruncatePattern removes the last character from the pattern
func (s *SearchState) TruncatePattern() {
	if s == nil || len(s.pattern) == 0 {
		return
	}
	s.pattern = s.pattern[:len(s.pattern)-1]
}

// Regex returns the compiled search regex (may be nil)
func (s *SearchState) Regex() *regexp.Regexp {
	if s == nil {
		return nil
	}
	return s.regex
}

// Matches returns the slice of matching line numbers
func (s *SearchState) Matches() []int {
	if s == nil {
		return nil
	}
	return s.matches
}

// MatchCount returns the number of matches found
func (s *SearchState) MatchCount() int {
	if s == nil {
		return 0
	}
	return len(s.matches)
}

// CurrentMatchIndex returns the current match index for navigation
func (s *SearchState) CurrentMatchIndex() int {
	if s == nil {
		return 0
	}
	return s.current
}

// CurrentMatchLine returns the line number of the current match, or -1 if no matches
func (s *SearchState) CurrentMatchLine() int {
	if s == nil || len(s.matches) == 0 || s.current >= len(s.matches) {
		return -1
	}
	return s.matches[s.current]
}

// HasMatches returns true if there are any matches
func (s *SearchState) HasMatches() bool {
	return s != nil && len(s.matches) > 0
}

// OutputScroll returns the current output scroll position for search
func (s *SearchState) OutputScroll() int {
	if s == nil {
		return 0
	}
	return s.outputScroll
}

// SetOutputScroll sets the output scroll position
func (s *SearchState) SetOutputScroll(scroll int) {
	if s == nil {
		return
	}
	s.outputScroll = max(scroll, 0)
}

// NextMatch advances to the next match, wrapping around if necessary
func (s *SearchState) NextMatch() {
	if s == nil || len(s.matches) == 0 {
		return
	}
	s.current = (s.current + 1) % len(s.matches)
}

// PrevMatch goes to the previous match, wrapping around if necessary
func (s *SearchState) PrevMatch() {
	if s == nil || len(s.matches) == 0 {
		return
	}
	s.current = (s.current - 1 + len(s.matches)) % len(s.matches)
}

// Execute compiles the search pattern and finds all matches in the given output.
// If the pattern starts with "r:", it's treated as a regex; otherwise it's a literal search.
// Returns the scroll position that would center the first match.
func (s *SearchState) Execute(output string, viewportHeight int) int {
	if s == nil {
		return 0
	}
	if s.pattern == "" {
		s.matches = nil
		s.regex = nil
		return 0
	}

	if output == "" {
		return 0
	}

	// Try to compile as regex if it starts with r:
	if strings.HasPrefix(s.pattern, "r:") {
		pattern := s.pattern[2:]
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			s.regex = nil
			s.matches = nil
			return 0
		}
		s.regex = re
	} else {
		// Literal search (case-insensitive)
		s.regex = regexp.MustCompile("(?i)" + regexp.QuoteMeta(s.pattern))
	}

	// Find all matching lines
	lines := strings.Split(output, "\n")
	s.matches = nil
	for i, line := range lines {
		if s.regex.MatchString(line) {
			s.matches = append(s.matches, i)
		}
	}

	// Set current match and calculate scroll position
	if len(s.matches) > 0 {
		s.current = 0
		return s.calculateScrollForMatch(viewportHeight)
	}

	return 0
}

// calculateScrollForMatch returns the scroll position to center the current match
func (s *SearchState) calculateScrollForMatch(viewportHeight int) int {
	matchLine := s.CurrentMatchLine()
	if matchLine < 0 {
		return 0
	}

	// Center the match in the visible area
	return max(matchLine-viewportHeight/2, 0)
}

// ScrollToCurrentMatch returns the scroll position needed to show the current match
func (s *SearchState) ScrollToCurrentMatch(viewportHeight int) int {
	if s == nil {
		return 0
	}
	return s.calculateScrollForMatch(viewportHeight)
}

// Clear resets all search state
func (s *SearchState) Clear() {
	if s == nil {
		return
	}
	s.pattern = ""
	s.regex = nil
	s.matches = nil
	s.current = 0
	s.outputScroll = 0
}

// HasPattern returns true if there's an active search pattern (even if not in search mode)
func (s *SearchState) HasPattern() bool {
	return s != nil && s.pattern != ""
}

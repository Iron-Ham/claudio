package tui

import (
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Search and filter state handling for the TUI.
// This file contains all search mode and filter mode logic extracted from app.go.

// handleSearchInput handles keyboard input when in search mode
func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel search mode (keep existing pattern if any)
		m.searchMode = false
		return m, nil

	case tea.KeyEnter:
		// Execute search and exit search mode
		m.executeSearch()
		m.searchMode = false
		return m, nil

	case tea.KeyBackspace:
		if len(m.searchPattern) > 0 {
			m.searchPattern = m.searchPattern[:len(m.searchPattern)-1]
			// Live search as user types
			m.executeSearch()
		}
		return m, nil

	case tea.KeyRunes:
		m.searchPattern += string(msg.Runes)
		// Live search as user types
		m.executeSearch()
		return m, nil

	case tea.KeySpace:
		m.searchPattern += " "
		m.executeSearch()
		return m, nil
	}

	return m, nil
}

// handleFilterInput handles keyboard input when in filter mode
func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "F", "q":
		m.filterMode = false
		return m, nil

	case "e", "1":
		m.filterCategories["errors"] = !m.filterCategories["errors"]
		return m, nil

	case "w", "2":
		m.filterCategories["warnings"] = !m.filterCategories["warnings"]
		return m, nil

	case "t", "3":
		m.filterCategories["tools"] = !m.filterCategories["tools"]
		return m, nil

	case "h", "4":
		m.filterCategories["thinking"] = !m.filterCategories["thinking"]
		return m, nil

	case "p", "5":
		m.filterCategories["progress"] = !m.filterCategories["progress"]
		return m, nil

	case "a":
		// Toggle all categories
		allEnabled := true
		for _, v := range m.filterCategories {
			if !v {
				allEnabled = false
				break
			}
		}
		for k := range m.filterCategories {
			m.filterCategories[k] = !allEnabled
		}
		return m, nil

	case "c":
		// Clear custom filter
		m.filterCustom = ""
		m.filterRegex = nil
		return m, nil
	}

	// Handle custom filter input
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.filterCustom) > 0 {
			m.filterCustom = m.filterCustom[:len(m.filterCustom)-1]
			m.compileFilterRegex()
		}
		return m, nil

	case tea.KeyRunes:
		// Check if it's not a shortcut key
		char := string(msg.Runes)
		if char != "e" && char != "w" && char != "t" && char != "h" && char != "p" && char != "a" && char != "c" {
			m.filterCustom += char
			m.compileFilterRegex()
		}
		return m, nil

	case tea.KeySpace:
		m.filterCustom += " "
		m.compileFilterRegex()
		return m, nil
	}

	return m, nil
}

// executeSearch compiles the search pattern and finds all matches
func (m *Model) executeSearch() {
	if m.searchPattern == "" {
		m.searchMatches = nil
		m.searchRegex = nil
		return
	}

	inst := m.activeInstance()
	if inst == nil {
		return
	}

	output := m.outputs[inst.ID]
	if output == "" {
		return
	}

	// Try to compile as regex if it starts with r:
	if strings.HasPrefix(m.searchPattern, "r:") {
		pattern := m.searchPattern[2:]
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			m.searchRegex = nil
			m.searchMatches = nil
			return
		}
		m.searchRegex = re
	} else {
		// Literal search (case-insensitive)
		m.searchRegex = regexp.MustCompile("(?i)" + regexp.QuoteMeta(m.searchPattern))
	}

	// Find all matching lines
	lines := strings.Split(output, "\n")
	m.searchMatches = nil
	for i, line := range lines {
		if m.searchRegex.MatchString(line) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}

	// Set current match
	if len(m.searchMatches) > 0 {
		m.searchCurrent = 0
		m.scrollToMatch()
	}
}

// clearSearch clears the current search
func (m *Model) clearSearch() {
	m.searchPattern = ""
	m.searchRegex = nil
	m.searchMatches = nil
	m.searchCurrent = 0
	m.outputScroll = 0
}

// scrollToMatch adjusts output scroll to show the current match
func (m *Model) scrollToMatch() {
	if len(m.searchMatches) == 0 || m.searchCurrent >= len(m.searchMatches) {
		return
	}

	matchLine := m.searchMatches[m.searchCurrent]
	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}

	// Center the match in the visible area
	m.outputScroll = matchLine - maxLines/2
	if m.outputScroll < 0 {
		m.outputScroll = 0
	}
}

// compileFilterRegex compiles the custom filter pattern
func (m *Model) compileFilterRegex() {
	if m.filterCustom == "" {
		m.filterRegex = nil
		return
	}

	re, err := regexp.Compile("(?i)" + m.filterCustom)
	if err != nil {
		m.filterRegex = nil
		return
	}
	m.filterRegex = re
}

// filterOutput applies category and custom filters to output
func (m *Model) filterOutput(output string) string {
	// If all categories enabled and no custom filter, return as-is
	allEnabled := true
	for _, v := range m.filterCategories {
		if !v {
			allEnabled = false
			break
		}
	}
	if allEnabled && m.filterRegex == nil {
		return output
	}

	lines := strings.Split(output, "\n")
	var filtered []string

	for _, line := range lines {
		if m.shouldShowLine(line) {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
}

// shouldShowLine determines if a line should be shown based on filters
func (m *Model) shouldShowLine(line string) bool {
	// Custom filter takes precedence
	if m.filterRegex != nil {
		return m.filterRegex.MatchString(line)
	}

	lineLower := strings.ToLower(line)

	// Check category filters
	if !m.filterCategories["errors"] {
		if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "failed") ||
			strings.Contains(lineLower, "exception") || strings.Contains(lineLower, "panic") {
			return false
		}
	}

	if !m.filterCategories["warnings"] {
		if strings.Contains(lineLower, "warning") || strings.Contains(lineLower, "warn") {
			return false
		}
	}

	if !m.filterCategories["tools"] {
		// Common Claude tool call patterns
		if strings.Contains(lineLower, "read file") || strings.Contains(lineLower, "write file") ||
			strings.Contains(lineLower, "bash") || strings.Contains(lineLower, "running") ||
			strings.HasPrefix(line, "  ") && (strings.Contains(line, "(") || strings.Contains(line, "→")) {
			return false
		}
	}

	if !m.filterCategories["thinking"] {
		if strings.Contains(lineLower, "thinking") || strings.Contains(lineLower, "let me") ||
			strings.Contains(lineLower, "i'll") || strings.Contains(lineLower, "i will") {
			return false
		}
	}

	if !m.filterCategories["progress"] {
		if strings.Contains(line, "...") || strings.Contains(line, "✓") ||
			strings.Contains(line, "█") || strings.Contains(line, "░") {
			return false
		}
	}

	return true
}

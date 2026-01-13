package config

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTotalLines(t *testing.T) {
	m := New()

	// Each category has: 1 header line + N items + 1 blank line
	// Count expected lines from the actual categories
	expected := 0
	for _, cat := range m.categories {
		expected++ // header
		expected += len(cat.Items)
		expected++ // blank line
	}

	got := m.totalLines()
	if got != expected {
		t.Errorf("totalLines() = %d, want %d", got, expected)
	}
}

func TestCurrentSelectionLine(t *testing.T) {
	m := New()

	tests := []struct {
		name          string
		categoryIndex int
		itemIndex     int
		wantLine      int
	}{
		{
			name:          "first item in first category",
			categoryIndex: 0,
			itemIndex:     0,
			wantLine:      1, // after category header
		},
		{
			name:          "second item in first category",
			categoryIndex: 0,
			itemIndex:     1,
			wantLine:      2, // header + 1 item
		},
		{
			name:          "first item in second category",
			categoryIndex: 1,
			itemIndex:     0,
			// First category: 1 header + items + 1 blank + 1 header for second
			wantLine: len(m.categories[0].Items) + 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.categoryIndex = tt.categoryIndex
			m.itemIndex = tt.itemIndex
			got := m.currentSelectionLine()
			if got != tt.wantLine {
				t.Errorf("currentSelectionLine() = %d, want %d", got, tt.wantLine)
			}
		})
	}
}

func TestEnsureSelectionVisible(t *testing.T) {
	tests := []struct {
		name           string
		scrollOffset   int
		categoryIndex  int
		itemIndex      int
		availableLines int
	}{
		{
			name:           "selection at top stays visible",
			scrollOffset:   0,
			categoryIndex:  0,
			itemIndex:      0,
			availableLines: 10,
		},
		{
			name:           "scroll down when selection below viewport",
			scrollOffset:   0,
			categoryIndex:  2, // Instance category
			itemIndex:      0,
			availableLines: 5,
		},
		{
			name:           "scroll up when selection above viewport",
			scrollOffset:   20,
			categoryIndex:  0,
			itemIndex:      0,
			availableLines: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := New()
			m.scrollOffset = tt.scrollOffset
			m.categoryIndex = tt.categoryIndex
			m.itemIndex = tt.itemIndex

			m.ensureSelectionVisible(tt.availableLines)

			// Allow some flexibility since the exact offset depends on category sizes
			if m.scrollOffset < 0 {
				t.Errorf("scrollOffset should not be negative, got %d", m.scrollOffset)
			}

			// Verify selection is within viewport
			selectionLine := m.currentSelectionLine()
			if selectionLine < m.scrollOffset || selectionLine >= m.scrollOffset+tt.availableLines {
				t.Errorf("selection line %d not in viewport [%d, %d)",
					selectionLine, m.scrollOffset, m.scrollOffset+tt.availableLines)
			}
		})
	}
}

func TestNavigationUpdatesScroll(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 20 // Small height to force scrolling

	// Navigate down multiple times
	for i := 0; i < 15; i++ {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = newModel.(Model)
	}

	// Verify scroll offset was updated
	if m.scrollOffset == 0 {
		t.Error("scrollOffset should have been updated after navigating down")
	}

	// Navigate back up
	for i := 0; i < 15; i++ {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		m = newModel.(Model)
	}

	// Should be back near the top
	if m.scrollOffset > 5 {
		t.Errorf("scrollOffset should be near top after navigating up, got %d", m.scrollOffset)
	}
}

func TestPageDownNavigation(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 20

	initialCategory := m.categoryIndex
	initialItem := m.itemIndex

	// Page down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = newModel.(Model)

	// Should have moved down significantly
	moved := false
	if m.categoryIndex > initialCategory {
		moved = true
	} else if m.categoryIndex == initialCategory && m.itemIndex > initialItem {
		moved = true
	}

	if !moved {
		t.Error("page down should have moved selection down")
	}
}

func TestPageUpNavigation(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 20

	// First go to bottom
	m.categoryIndex = len(m.categories) - 1
	m.itemIndex = len(m.categories[m.categoryIndex].Items) - 1

	initialCategory := m.categoryIndex
	initialItem := m.itemIndex

	// Page up
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m = newModel.(Model)

	// Should have moved up significantly
	moved := false
	if m.categoryIndex < initialCategory {
		moved = true
	} else if m.categoryIndex == initialCategory && m.itemIndex < initialItem {
		moved = true
	}

	if !moved {
		t.Error("page up should have moved selection up")
	}
}

func TestGoToTopAndBottom(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 20

	// Go to bottom with G
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = newModel.(Model)

	if m.categoryIndex != len(m.categories)-1 {
		t.Errorf("G should go to last category, got %d", m.categoryIndex)
	}
	lastCat := m.categories[m.categoryIndex]
	if m.itemIndex != len(lastCat.Items)-1 {
		t.Errorf("G should go to last item, got %d", m.itemIndex)
	}

	// Go to top with g
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	m = newModel.(Model)

	if m.categoryIndex != 0 || m.itemIndex != 0 {
		t.Errorf("g should go to first item, got category=%d item=%d", m.categoryIndex, m.itemIndex)
	}
}

func TestViewRendersScrollIndicators(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 15 // Small height to force scrolling

	// Render at top
	view := m.View()
	if strings.Contains(view, "▲") {
		t.Error("should not show up arrow when at top")
	}
	if !strings.Contains(view, "▼") {
		t.Error("should show down arrow when content below")
	}

	// Navigate to middle
	m.scrollOffset = 10
	m.categoryIndex = 3
	m.itemIndex = 0

	view = m.View()
	if !strings.Contains(view, "▲") {
		t.Error("should show up arrow when content above")
	}
	if !strings.Contains(view, "▼") {
		t.Error("should show down arrow when content below")
	}
}

func TestWindowResizeUpdatesScroll(t *testing.T) {
	m := New()
	m.width = 80
	m.height = 50

	// Navigate to bottom
	m.categoryIndex = len(m.categories) - 1
	m.itemIndex = len(m.categories[m.categoryIndex].Items) - 1
	m.scrollOffset = 30

	// Resize to smaller window
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})
	m = newModel.(Model)

	// Verify selection is still visible
	availableLines := m.height - 12
	if availableLines < 5 {
		availableLines = 5
	}
	selectionLine := m.currentSelectionLine()
	if selectionLine < m.scrollOffset || selectionLine >= m.scrollOffset+availableLines {
		t.Error("selection should remain visible after resize")
	}
}

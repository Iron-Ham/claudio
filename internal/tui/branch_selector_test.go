package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleBranchSelector(t *testing.T) {
	t.Run("escape closes selector and clears search", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchSearchInput = "test"
		m.branchFiltered = []string{"test-branch"}
		m.branchScrollOffset = 5

		msg := tea.KeyMsg{Type: tea.KeyEsc}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.showBranchSelector {
			t.Error("expected showBranchSelector to be false")
		}
		if model.branchSearchInput != "" {
			t.Error("expected branchSearchInput to be cleared")
		}
		if model.branchFiltered != nil {
			t.Error("expected branchFiltered to be nil")
		}
		if model.branchScrollOffset != 0 {
			t.Error("expected branchScrollOffset to be 0")
		}
	})

	t.Run("enter selects highlighted branch", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = []string{"main", "feature-1", "feature-2"}
		m.branchSelected = 1

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.selectedBaseBranch != "feature-1" {
			t.Errorf("expected selectedBaseBranch to be %q, got %q", "feature-1", model.selectedBaseBranch)
		}
		if model.showBranchSelector {
			t.Error("expected showBranchSelector to be false after selection")
		}
	})

	t.Run("tab selects highlighted branch", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = []string{"main", "feature-1"}
		m.branchSelected = 0

		msg := tea.KeyMsg{Type: tea.KeyTab}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.selectedBaseBranch != "main" {
			t.Errorf("expected selectedBaseBranch to be %q, got %q", "main", model.selectedBaseBranch)
		}
	})

	t.Run("up arrow moves selection up", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = []string{"main", "feature-1", "feature-2"}
		m.branchSelected = 2
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyUp}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSelected != 1 {
			t.Errorf("expected branchSelected to be 1, got %d", model.branchSelected)
		}
	})

	t.Run("down arrow moves selection down", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = []string{"main", "feature-1", "feature-2"}
		m.branchSelected = 0
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyDown}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSelected != 1 {
			t.Errorf("expected branchSelected to be 1, got %d", model.branchSelected)
		}
	})

	t.Run("up at top stays at top", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = []string{"main", "feature-1"}
		m.branchSelected = 0
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyUp}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSelected != 0 {
			t.Errorf("expected branchSelected to stay at 0, got %d", model.branchSelected)
		}
	})

	t.Run("down at bottom stays at bottom", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = []string{"main", "feature-1"}
		m.branchSelected = 1
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyDown}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSelected != 1 {
			t.Errorf("expected branchSelected to stay at 1, got %d", model.branchSelected)
		}
	})

	t.Run("typing adds to search and filters branches", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchList = []string{"main", "feature-1", "feature-2", "bugfix-1"}
		m.branchFiltered = m.branchList
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("feat")}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSearchInput != "feat" {
			t.Errorf("expected branchSearchInput to be %q, got %q", "feat", model.branchSearchInput)
		}
		if len(model.branchFiltered) != 2 {
			t.Errorf("expected 2 filtered branches, got %d", len(model.branchFiltered))
		}
	})

	t.Run("backspace removes from search", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchSearchInput = "feat"
		m.branchList = []string{"main", "feature-1"}
		m.branchFiltered = []string{"feature-1"}
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSearchInput != "fea" {
			t.Errorf("expected branchSearchInput to be %q, got %q", "fea", model.branchSearchInput)
		}
	})

	t.Run("space adds to search", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchSearchInput = "my"
		m.branchList = []string{"my branch", "other"}
		m.branchFiltered = []string{"my branch"}
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeySpace}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSearchInput != "my " {
			t.Errorf("expected branchSearchInput to be %q, got %q", "my ", model.branchSearchInput)
		}
	})

	t.Run("page down moves by viewport height", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = make([]string, 50)
		for i := range m.branchFiltered {
			m.branchFiltered[i] = "branch"
		}
		m.branchSelected = 0
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyPgDown}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSelected != 10 {
			t.Errorf("expected branchSelected to be 10, got %d", model.branchSelected)
		}
	})

	t.Run("page up moves by viewport height", func(t *testing.T) {
		m := testModel()
		m.showBranchSelector = true
		m.branchFiltered = make([]string, 50)
		for i := range m.branchFiltered {
			m.branchFiltered[i] = "branch"
		}
		m.branchSelected = 20
		m.branchSelectorHeight = 10

		msg := tea.KeyMsg{Type: tea.KeyPgUp}
		result, _ := m.handleBranchSelector(msg)
		model := result.(Model)

		if model.branchSelected != 10 {
			t.Errorf("expected branchSelected to be 10, got %d", model.branchSelected)
		}
	})
}

func TestApplyBranchFilter(t *testing.T) {
	t.Run("empty search shows all branches", func(t *testing.T) {
		m := testModel()
		m.branchList = []string{"main", "feature-1", "bugfix-1"}
		m.branchSearchInput = ""
		m.branchSelectorHeight = 10

		result := m.applyBranchFilter()

		if len(result.branchFiltered) != 3 {
			t.Errorf("expected 3 branches, got %d", len(result.branchFiltered))
		}
	})

	t.Run("filter is case insensitive", func(t *testing.T) {
		m := testModel()
		m.branchList = []string{"main", "Feature-1", "BUGFIX-1"}
		m.branchSearchInput = "feature"
		m.branchSelectorHeight = 10

		result := m.applyBranchFilter()

		if len(result.branchFiltered) != 1 {
			t.Errorf("expected 1 branch, got %d", len(result.branchFiltered))
		}
		if result.branchFiltered[0] != "Feature-1" {
			t.Errorf("expected %q, got %q", "Feature-1", result.branchFiltered[0])
		}
	})

	t.Run("filter resets selection to first item", func(t *testing.T) {
		m := testModel()
		m.branchList = []string{"main", "feature-1", "feature-2"}
		m.branchSearchInput = "feat"
		m.branchSelected = 5
		m.branchScrollOffset = 10
		m.branchSelectorHeight = 10

		result := m.applyBranchFilter()

		if result.branchSelected != 0 {
			t.Errorf("expected branchSelected to be 0, got %d", result.branchSelected)
		}
		if result.branchScrollOffset != 0 {
			t.Errorf("expected branchScrollOffset to be 0, got %d", result.branchScrollOffset)
		}
	})

	t.Run("preserves previously selected branch if still visible", func(t *testing.T) {
		m := testModel()
		m.branchList = []string{"main", "feature-1", "feature-2", "bugfix-1"}
		m.branchSearchInput = "feature"
		m.selectedBaseBranch = "feature-2"
		m.branchSelectorHeight = 10

		result := m.applyBranchFilter()

		// feature-2 is the second match (index 1)
		if result.branchSelected != 1 {
			t.Errorf("expected branchSelected to be 1, got %d", result.branchSelected)
		}
	})
}

func TestAdjustBranchScroll(t *testing.T) {
	t.Run("scrolls up when selection above viewport", func(t *testing.T) {
		m := testModel()
		m.branchFiltered = make([]string, 50)
		m.branchSelected = 2
		m.branchScrollOffset = 10
		m.branchSelectorHeight = 5

		result := m.adjustBranchScroll()

		if result.branchScrollOffset != 2 {
			t.Errorf("expected branchScrollOffset to be 2, got %d", result.branchScrollOffset)
		}
	})

	t.Run("scrolls down when selection below viewport", func(t *testing.T) {
		m := testModel()
		m.branchFiltered = make([]string, 50)
		m.branchSelected = 15
		m.branchScrollOffset = 5
		m.branchSelectorHeight = 5

		result := m.adjustBranchScroll()

		// Selection at 15 with height 5 means scroll should be 11 (15-5+1)
		if result.branchScrollOffset != 11 {
			t.Errorf("expected branchScrollOffset to be 11, got %d", result.branchScrollOffset)
		}
	})

	t.Run("clamps scroll offset to max", func(t *testing.T) {
		m := testModel()
		m.branchFiltered = make([]string, 10)
		m.branchSelected = 9
		m.branchScrollOffset = 100 // Way too high
		m.branchSelectorHeight = 5

		result := m.adjustBranchScroll()

		// Max scroll is 10-5=5
		if result.branchScrollOffset != 5 {
			t.Errorf("expected branchScrollOffset to be 5, got %d", result.branchScrollOffset)
		}
	})

	t.Run("clamps scroll offset to zero minimum", func(t *testing.T) {
		m := testModel()
		m.branchFiltered = make([]string, 10)
		m.branchSelected = 0
		m.branchScrollOffset = -5
		m.branchSelectorHeight = 5

		result := m.adjustBranchScroll()

		if result.branchScrollOffset != 0 {
			t.Errorf("expected branchScrollOffset to be 0, got %d", result.branchScrollOffset)
		}
	})

	t.Run("handles zero height gracefully", func(t *testing.T) {
		m := testModel()
		m.branchFiltered = make([]string, 10)
		m.branchScrollOffset = 5
		m.branchSelectorHeight = 0

		result := m.adjustBranchScroll()

		// Should return unchanged when height is 0
		if result.branchScrollOffset != 5 {
			t.Errorf("expected branchScrollOffset to remain 5, got %d", result.branchScrollOffset)
		}
	})
}

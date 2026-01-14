package view

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockBranchProvider implements BranchProvider for testing.
type mockBranchProvider struct {
	branches []string
	err      error
}

func (m *mockBranchProvider) ListBranches() ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.branches, nil
}

func TestNewBranchSelectorState(t *testing.T) {
	state := NewBranchSelectorState()

	if state.Height != 10 {
		t.Errorf("expected default Height to be 10, got %d", state.Height)
	}
	if state.Visible {
		t.Error("expected Visible to be false by default")
	}
}

func TestBranchSelector_Open(t *testing.T) {
	t.Run("initializes state correctly", func(t *testing.T) {
		bs := NewBranchSelector()
		state := NewBranchSelectorState()
		provider := &mockBranchProvider{
			branches: []string{"main", "feature-1", "feature-2"},
		}

		_, err := bs.Open(state, provider, 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !state.Visible {
			t.Error("expected Visible to be true")
		}
		if len(state.BranchList) != 3 {
			t.Errorf("expected 3 branches, got %d", len(state.BranchList))
		}
		if len(state.Filtered) != 3 {
			t.Errorf("expected 3 filtered branches, got %d", len(state.Filtered))
		}
		if state.SearchInput != "" {
			t.Error("expected SearchInput to be empty")
		}
		if state.ScrollOffset != 0 {
			t.Error("expected ScrollOffset to be 0")
		}
	})

	t.Run("returns error from provider", func(t *testing.T) {
		bs := NewBranchSelector()
		state := NewBranchSelectorState()
		provider := &mockBranchProvider{
			err: errors.New("failed to list branches"),
		}

		_, err := bs.Open(state, provider, 30)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// State should not be marked visible on error
		if state.Visible {
			t.Error("expected Visible to remain false on error")
		}
	})

	t.Run("selects previously selected branch", func(t *testing.T) {
		bs := NewBranchSelector()
		state := NewBranchSelectorState()
		state.SelectedBranch = "feature-2"
		provider := &mockBranchProvider{
			branches: []string{"main", "feature-1", "feature-2", "feature-3"},
		}

		_, err := bs.Open(state, provider, 30)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if state.Selected != 2 {
			t.Errorf("expected Selected to be 2, got %d", state.Selected)
		}
	})

	t.Run("calculates height correctly", func(t *testing.T) {
		bs := NewBranchSelector()

		// Test minimum height clamp
		state1 := NewBranchSelectorState()
		provider := &mockBranchProvider{branches: []string{"main"}}
		_, _ = bs.Open(state1, provider, 10)
		if state1.Height != 5 {
			t.Errorf("expected minimum height of 5, got %d", state1.Height)
		}

		// Test maximum height clamp
		state2 := NewBranchSelectorState()
		_, _ = bs.Open(state2, provider, 100)
		if state2.Height != 15 {
			t.Errorf("expected maximum height of 15, got %d", state2.Height)
		}

		// Test normal height calculation
		state3 := NewBranchSelectorState()
		_, _ = bs.Open(state3, provider, 22) // 22 - 10 = 12
		if state3.Height != 12 {
			t.Errorf("expected height of 12, got %d", state3.Height)
		}
	})
}

func TestBranchSelector_Close(t *testing.T) {
	bs := NewBranchSelector()
	state := &BranchSelectorState{
		Visible:      true,
		SearchInput:  "test",
		Filtered:     []string{"main"},
		ScrollOffset: 5,
	}

	bs.Close(state)

	if state.Visible {
		t.Error("expected Visible to be false")
	}
	if state.SearchInput != "" {
		t.Error("expected SearchInput to be cleared")
	}
	if state.Filtered != nil {
		t.Error("expected Filtered to be nil")
	}
	if state.ScrollOffset != 0 {
		t.Error("expected ScrollOffset to be 0")
	}
}

func TestBranchSelector_HandleKey(t *testing.T) {
	t.Run("escape closes selector", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:      true,
			SearchInput:  "test",
			Filtered:     []string{"test-branch"},
			ScrollOffset: 5,
		}

		msg := tea.KeyMsg{Type: tea.KeyEsc}
		selectionMade, _, _ := bs.HandleKey(state, msg)

		if selectionMade {
			t.Error("expected selectionMade to be false")
		}
		if state.Visible {
			t.Error("expected Visible to be false")
		}
		if state.SearchInput != "" {
			t.Error("expected SearchInput to be cleared")
		}
	})

	t.Run("enter selects highlighted branch", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1", "feature-2"},
			Selected: 1,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		selectionMade, selectedBranch, _ := bs.HandleKey(state, msg)

		if !selectionMade {
			t.Error("expected selectionMade to be true")
		}
		if selectedBranch != "feature-1" {
			t.Errorf("expected selectedBranch to be %q, got %q", "feature-1", selectedBranch)
		}
		if state.SelectedBranch != "feature-1" {
			t.Errorf("expected SelectedBranch to be %q, got %q", "feature-1", state.SelectedBranch)
		}
		if state.Visible {
			t.Error("expected Visible to be false after selection")
		}
	})

	t.Run("tab selects highlighted branch", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1"},
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyTab}
		selectionMade, selectedBranch, _ := bs.HandleKey(state, msg)

		if !selectionMade {
			t.Error("expected selectionMade to be true")
		}
		if selectedBranch != "main" {
			t.Errorf("expected selectedBranch to be %q, got %q", "main", selectedBranch)
		}
	})

	t.Run("up arrow moves selection up", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1", "feature-2"},
			Selected: 2,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyUp}
		selectionMade, _, _ := bs.HandleKey(state, msg)

		if selectionMade {
			t.Error("expected selectionMade to be false")
		}
		if state.Selected != 1 {
			t.Errorf("expected Selected to be 1, got %d", state.Selected)
		}
	})

	t.Run("down arrow moves selection down", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1", "feature-2"},
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyDown}
		selectionMade, _, _ := bs.HandleKey(state, msg)

		if selectionMade {
			t.Error("expected selectionMade to be false")
		}
		if state.Selected != 1 {
			t.Errorf("expected Selected to be 1, got %d", state.Selected)
		}
	})

	t.Run("up at top stays at top", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1"},
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyUp}
		bs.HandleKey(state, msg)

		if state.Selected != 0 {
			t.Errorf("expected Selected to stay at 0, got %d", state.Selected)
		}
	})

	t.Run("down at bottom stays at bottom", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1"},
			Selected: 1,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyDown}
		bs.HandleKey(state, msg)

		if state.Selected != 1 {
			t.Errorf("expected Selected to stay at 1, got %d", state.Selected)
		}
	})

	t.Run("typing adds to search and filters branches", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:    true,
			BranchList: []string{"main", "feature-1", "feature-2", "bugfix-1"},
			Filtered:   []string{"main", "feature-1", "feature-2", "bugfix-1"},
			Height:     10,
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("feat")}
		bs.HandleKey(state, msg)

		if state.SearchInput != "feat" {
			t.Errorf("expected SearchInput to be %q, got %q", "feat", state.SearchInput)
		}
		if len(state.Filtered) != 2 {
			t.Errorf("expected 2 filtered branches, got %d", len(state.Filtered))
		}
	})

	t.Run("backspace removes from search", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:     true,
			SearchInput: "feat",
			BranchList:  []string{"main", "feature-1"},
			Filtered:    []string{"feature-1"},
			Height:      10,
		}

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		bs.HandleKey(state, msg)

		if state.SearchInput != "fea" {
			t.Errorf("expected SearchInput to be %q, got %q", "fea", state.SearchInput)
		}
	})

	t.Run("space adds to search", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:     true,
			SearchInput: "my",
			BranchList:  []string{"my branch", "other"},
			Filtered:    []string{"my branch"},
			Height:      10,
		}

		msg := tea.KeyMsg{Type: tea.KeySpace}
		bs.HandleKey(state, msg)

		if state.SearchInput != "my " {
			t.Errorf("expected SearchInput to be %q, got %q", "my ", state.SearchInput)
		}
	})

	t.Run("page down moves by viewport height", func(t *testing.T) {
		bs := NewBranchSelector()
		branches := make([]string, 50)
		for i := range branches {
			branches[i] = "branch"
		}
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: branches,
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyPgDown}
		bs.HandleKey(state, msg)

		if state.Selected != 10 {
			t.Errorf("expected Selected to be 10, got %d", state.Selected)
		}
	})

	t.Run("page up moves by viewport height", func(t *testing.T) {
		bs := NewBranchSelector()
		branches := make([]string, 50)
		for i := range branches {
			branches[i] = "branch"
		}
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: branches,
			Selected: 20,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyPgUp}
		bs.HandleKey(state, msg)

		if state.Selected != 10 {
			t.Errorf("expected Selected to be 10, got %d", state.Selected)
		}
	})

	t.Run("page down clamps to last item", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1", "feature-2"},
			Selected: 1,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyPgDown}
		bs.HandleKey(state, msg)

		if state.Selected != 2 {
			t.Errorf("expected Selected to be 2, got %d", state.Selected)
		}
	})

	t.Run("page up clamps to first item", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{"main", "feature-1", "feature-2"},
			Selected: 1,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyPgUp}
		bs.HandleKey(state, msg)

		if state.Selected != 0 {
			t.Errorf("expected Selected to be 0, got %d", state.Selected)
		}
	})

	t.Run("ctrl+u works like page up", func(t *testing.T) {
		bs := NewBranchSelector()
		branches := make([]string, 50)
		for i := range branches {
			branches[i] = "branch"
		}
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: branches,
			Selected: 20,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyCtrlU}
		bs.HandleKey(state, msg)

		if state.Selected != 10 {
			t.Errorf("expected Selected to be 10, got %d", state.Selected)
		}
	})

	t.Run("ctrl+d works like page down", func(t *testing.T) {
		bs := NewBranchSelector()
		branches := make([]string, 50)
		for i := range branches {
			branches[i] = "branch"
		}
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: branches,
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyCtrlD}
		bs.HandleKey(state, msg)

		if state.Selected != 10 {
			t.Errorf("expected Selected to be 10, got %d", state.Selected)
		}
	})
}

func TestBranchSelector_ApplyFilter(t *testing.T) {
	t.Run("empty search shows all branches", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			BranchList:  []string{"main", "feature-1", "bugfix-1"},
			SearchInput: "",
			Height:      10,
		}

		bs.ApplyFilter(state)

		if len(state.Filtered) != 3 {
			t.Errorf("expected 3 branches, got %d", len(state.Filtered))
		}
	})

	t.Run("filter is case insensitive", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			BranchList:  []string{"main", "Feature-1", "BUGFIX-1"},
			SearchInput: "feature",
			Height:      10,
		}

		bs.ApplyFilter(state)

		if len(state.Filtered) != 1 {
			t.Errorf("expected 1 branch, got %d", len(state.Filtered))
		}
		if state.Filtered[0] != "Feature-1" {
			t.Errorf("expected %q, got %q", "Feature-1", state.Filtered[0])
		}
	})

	t.Run("filter resets selection to first item", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			BranchList:   []string{"main", "feature-1", "feature-2"},
			SearchInput:  "feat",
			Selected:     5,
			ScrollOffset: 10,
			Height:       10,
		}

		bs.ApplyFilter(state)

		if state.Selected != 0 {
			t.Errorf("expected Selected to be 0, got %d", state.Selected)
		}
		if state.ScrollOffset != 0 {
			t.Errorf("expected ScrollOffset to be 0, got %d", state.ScrollOffset)
		}
	})

	t.Run("preserves previously selected branch if still visible", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			BranchList:     []string{"main", "feature-1", "feature-2", "bugfix-1"},
			SearchInput:    "feature",
			SelectedBranch: "feature-2",
			Height:         10,
		}

		bs.ApplyFilter(state)

		// feature-2 is the second match (index 1)
		if state.Selected != 1 {
			t.Errorf("expected Selected to be 1, got %d", state.Selected)
		}
	})
}

func TestBranchSelector_AdjustScroll(t *testing.T) {
	t.Run("scrolls up when selection above viewport", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Filtered:     make([]string, 50),
			Selected:     2,
			ScrollOffset: 10,
			Height:       5,
		}

		bs.AdjustScroll(state)

		if state.ScrollOffset != 2 {
			t.Errorf("expected ScrollOffset to be 2, got %d", state.ScrollOffset)
		}
	})

	t.Run("scrolls down when selection below viewport", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Filtered:     make([]string, 50),
			Selected:     15,
			ScrollOffset: 5,
			Height:       5,
		}

		bs.AdjustScroll(state)

		// Selection at 15 with height 5 means scroll should be 11 (15-5+1)
		if state.ScrollOffset != 11 {
			t.Errorf("expected ScrollOffset to be 11, got %d", state.ScrollOffset)
		}
	})

	t.Run("clamps scroll offset to max", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Filtered:     make([]string, 10),
			Selected:     9,
			ScrollOffset: 100, // Way too high
			Height:       5,
		}

		bs.AdjustScroll(state)

		// Max scroll is 10-5=5
		if state.ScrollOffset != 5 {
			t.Errorf("expected ScrollOffset to be 5, got %d", state.ScrollOffset)
		}
	})

	t.Run("clamps scroll offset to zero minimum", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Filtered:     make([]string, 10),
			Selected:     0,
			ScrollOffset: -5,
			Height:       5,
		}

		bs.AdjustScroll(state)

		if state.ScrollOffset != 0 {
			t.Errorf("expected ScrollOffset to be 0, got %d", state.ScrollOffset)
		}
	})

	t.Run("handles zero height gracefully", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Filtered:     make([]string, 10),
			ScrollOffset: 5,
			Height:       0,
		}

		bs.AdjustScroll(state)

		// Should return unchanged when height is 0
		if state.ScrollOffset != 5 {
			t.Errorf("expected ScrollOffset to remain 5, got %d", state.ScrollOffset)
		}
	})
}

func TestBranchSelectorState_ToInputState(t *testing.T) {
	t.Run("converts state correctly", func(t *testing.T) {
		state := &BranchSelectorState{
			Visible:        true,
			Filtered:       []string{"main", "feature-1", "develop"},
			Selected:       1,
			ScrollOffset:   0,
			SearchInput:    "feat",
			SelectedBranch: "feature-1",
			Height:         10,
		}

		fields := state.ToInputState("main")

		if !fields.ShowBranchSelector {
			t.Error("expected ShowBranchSelector to be true")
		}
		if len(fields.Branches) != 3 {
			t.Errorf("expected 3 branches, got %d", len(fields.Branches))
		}
		if !fields.Branches[0].IsMain {
			t.Error("expected first branch to be marked as main")
		}
		if fields.Branches[1].IsMain {
			t.Error("expected second branch to not be marked as main")
		}
		if fields.BranchSelected != 1 {
			t.Errorf("expected BranchSelected to be 1, got %d", fields.BranchSelected)
		}
		if fields.BranchSearchInput != "feat" {
			t.Errorf("expected BranchSearchInput to be %q, got %q", "feat", fields.BranchSearchInput)
		}
		if fields.SelectedBranch != "feature-1" {
			t.Errorf("expected SelectedBranch to be %q, got %q", "feature-1", fields.SelectedBranch)
		}
		if fields.BranchSelectorHeight != 10 {
			t.Errorf("expected BranchSelectorHeight to be 10, got %d", fields.BranchSelectorHeight)
		}
	})

	t.Run("handles empty filtered list", func(t *testing.T) {
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{},
		}

		fields := state.ToInputState("main")

		if len(fields.Branches) != 0 {
			t.Errorf("expected 0 branches, got %d", len(fields.Branches))
		}
	})
}

func TestBranchSelector_EmptyFilteredList(t *testing.T) {
	t.Run("enter with empty list does nothing", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{},
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		selectionMade, selectedBranch, _ := bs.HandleKey(state, msg)

		if selectionMade {
			t.Error("expected selectionMade to be false with empty list")
		}
		if selectedBranch != "" {
			t.Error("expected selectedBranch to be empty")
		}
	})

	t.Run("page down with empty list handles gracefully", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:  true,
			Filtered: []string{},
			Selected: 0,
			Height:   10,
		}

		msg := tea.KeyMsg{Type: tea.KeyPgDown}
		bs.HandleKey(state, msg)

		// Selected should be clamped to 0 (or -1, but max(..., 0) should make it 0)
		if state.Selected != 0 {
			t.Errorf("expected Selected to be 0, got %d", state.Selected)
		}
	})
}

func TestBranchSelector_UnicodeHandling(t *testing.T) {
	t.Run("backspace handles unicode correctly", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:     true,
			SearchInput: "日本語",
			BranchList:  []string{"日本語-branch"},
			Filtered:    []string{"日本語-branch"},
			Height:      10,
		}

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		bs.HandleKey(state, msg)

		if state.SearchInput != "日本" {
			t.Errorf("expected SearchInput to be %q, got %q", "日本", state.SearchInput)
		}
	})

	t.Run("typing unicode characters works", func(t *testing.T) {
		bs := NewBranchSelector()
		state := &BranchSelectorState{
			Visible:     true,
			SearchInput: "",
			BranchList:  []string{"日本語-branch", "other"},
			Filtered:    []string{"日本語-branch", "other"},
			Height:      10,
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("日本")}
		bs.HandleKey(state, msg)

		if state.SearchInput != "日本" {
			t.Errorf("expected SearchInput to be %q, got %q", "日本", state.SearchInput)
		}
		if len(state.Filtered) != 1 {
			t.Errorf("expected 1 filtered branch, got %d", len(state.Filtered))
		}
	})
}

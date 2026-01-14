package view

import (
	"strings"
	"testing"
)

func TestInputView_RenderBranchSelector(t *testing.T) {
	iv := NewInputView()

	t.Run("renders search input with cursor", func(t *testing.T) {
		state := &InputState{
			ShowBranchSelector:   true,
			BranchSearchInput:    "feat",
			Branches:             []BranchItem{{Name: "feature-branch"}},
			BranchSelected:       0,
			BranchSelectorHeight: 10,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "Search:") {
			t.Error("should show search label")
		}
		if !strings.Contains(result, "feat") {
			t.Error("should show search input text")
		}
	})

	t.Run("shows no matching branches message when filter yields no results", func(t *testing.T) {
		state := &InputState{
			ShowBranchSelector:   true,
			BranchSearchInput:    "nonexistent",
			Branches:             []BranchItem{},
			BranchSelectorHeight: 10,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "No matching branches") {
			t.Error("should show 'no matching branches' when search yields no results")
		}
	})

	t.Run("shows no branches available when no branches exist", func(t *testing.T) {
		state := &InputState{
			ShowBranchSelector:   true,
			BranchSearchInput:    "",
			Branches:             []BranchItem{},
			BranchSelectorHeight: 10,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "No branches available") {
			t.Error("should show 'no branches available' when list is empty")
		}
	})

	t.Run("renders branch list with scroll indicators", func(t *testing.T) {
		branches := make([]BranchItem, 20)
		for i := 0; i < 20; i++ {
			branches[i] = BranchItem{Name: "branch-" + string(rune('a'+i))}
		}

		state := &InputState{
			ShowBranchSelector:   true,
			Branches:             branches,
			BranchSelected:       10,
			BranchScrollOffset:   5,
			BranchSelectorHeight: 5,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "more above") {
			t.Error("should show 'more above' indicator when scrolled down")
		}
		if !strings.Contains(result, "more below") {
			t.Error("should show 'more below' indicator when more items exist")
		}
	})

	t.Run("marks main branch with default suffix", func(t *testing.T) {
		state := &InputState{
			ShowBranchSelector: true,
			Branches: []BranchItem{
				{Name: "main", IsMain: true},
				{Name: "feature-branch"},
			},
			BranchSelected:       0,
			BranchSelectorHeight: 10,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "(default)") {
			t.Error("should mark main branch with (default) suffix")
		}
	})

	t.Run("shows branch count", func(t *testing.T) {
		state := &InputState{
			ShowBranchSelector: true,
			Branches: []BranchItem{
				{Name: "branch-1"},
				{Name: "branch-2"},
				{Name: "branch-3"},
			},
			BranchSelected:       0,
			BranchSelectorHeight: 10,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "3 branches") {
			t.Error("should show branch count")
		}
	})
}

func TestInputView_RenderHints_BranchSelector(t *testing.T) {
	iv := NewInputView()

	t.Run("shows filter hint for branch selector", func(t *testing.T) {
		state := &InputState{
			ShowBranchSelector:   true,
			BranchSelectorHeight: 10,
		}

		result := iv.Render(state, 80)

		if !strings.Contains(result, "filter") {
			t.Error("should show filter hint in branch selector mode")
		}
		if !strings.Contains(result, "navigate") {
			t.Error("should show navigate hint in branch selector mode")
		}
		if !strings.Contains(result, "select") {
			t.Error("should show select hint in branch selector mode")
		}
	})
}

func TestFormatCount(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{100, "100"},
		{999, "999"},
	}

	for _, tt := range tests {
		result := formatCount(tt.input)
		if result != tt.expected {
			t.Errorf("formatCount(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

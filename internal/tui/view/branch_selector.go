package view

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// BranchSelectorState holds the state for the branch selector dropdown.
// This is the canonical state representation used by the branch selector logic.
type BranchSelectorState struct {
	// Visible indicates whether the branch selector is currently shown
	Visible bool

	// BranchList is the full list of branch names (unfiltered)
	BranchList []string

	// Filtered is the filtered list of branch names based on search
	Filtered []string

	// Selected is the index of the currently highlighted branch in the filtered list
	Selected int

	// ScrollOffset is the scroll offset for the branch list viewport
	ScrollOffset int

	// SearchInput is the current search/filter text
	SearchInput string

	// SelectedBranch is the currently selected branch name (persisted across open/close)
	SelectedBranch string

	// Height is the maximum number of visible branches in the selector
	Height int
}

// NewBranchSelectorState creates a new BranchSelectorState with default values.
func NewBranchSelectorState() *BranchSelectorState {
	return &BranchSelectorState{
		Height: 10, // Default height
	}
}

// BranchProvider is an interface for fetching branch information.
// This allows the branch selector to be decoupled from the orchestrator.
type BranchProvider interface {
	// ListBranches returns a list of available branch names.
	// Returns an error if branches cannot be fetched.
	ListBranches() ([]string, error)
}

// BranchSelector encapsulates the logic for the branch selection dropdown.
// It operates on BranchSelectorState and returns updated state.
// This is a stateless component - all state is passed in and returned.
type BranchSelector struct{}

// NewBranchSelector creates a new BranchSelector.
func NewBranchSelector() *BranchSelector {
	return &BranchSelector{}
}

// Open initializes and opens the branch selector.
// It fetches branches from the provider and initializes the state.
// Returns the updated state and any error message.
func (bs *BranchSelector) Open(state *BranchSelectorState, provider BranchProvider, viewportHeight int) (string, error) {
	// Fetch branches from the provider
	branches, err := provider.ListBranches()
	if err != nil {
		return "", err
	}

	// Initialize state
	state.BranchList = branches
	state.SearchInput = ""
	state.Filtered = branches
	state.ScrollOffset = 0

	// Calculate visible height for branch selector (reserve space for UI elements)
	// Reserve: search line, scroll indicators, count line, padding
	state.Height = min(max(viewportHeight-10, 5), 15)

	// Find the index of the currently selected branch (if any)
	selectedIdx := 0
	if state.SelectedBranch != "" {
		for i, name := range state.Filtered {
			if name == state.SelectedBranch {
				selectedIdx = i
				break
			}
		}
	}

	state.Visible = true
	state.Selected = selectedIdx
	bs.AdjustScroll(state)

	return "", nil
}

// Close resets the branch selector state when closing.
// Returns the updated state with visibility cleared.
func (bs *BranchSelector) Close(state *BranchSelectorState) {
	state.Visible = false
	state.SearchInput = ""
	state.Filtered = nil
	state.ScrollOffset = 0
}

// HandleKey processes keyboard input when the branch selector is visible.
// Returns the updated state, selected branch (if selection was made), and a tea.Cmd.
// If selectionMade is true, the caller should close the selector and use the selectedBranch.
func (bs *BranchSelector) HandleKey(state *BranchSelectorState, msg tea.KeyMsg) (selectionMade bool, selectedBranch string, cmd tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		bs.Close(state)
		return false, "", nil

	case tea.KeyEnter, tea.KeyTab:
		// Select the highlighted branch from the filtered list
		if len(state.Filtered) > 0 && state.Selected < len(state.Filtered) {
			selectedBranch = state.Filtered[state.Selected]
			state.SelectedBranch = selectedBranch
			bs.Close(state)
			return true, selectedBranch, nil
		}
		// Empty list - just close without selecting
		bs.Close(state)
		return false, "", nil

	case tea.KeyUp:
		if state.Selected > 0 {
			state.Selected--
			bs.AdjustScroll(state)
		}
		return false, "", nil

	case tea.KeyDown:
		if state.Selected < len(state.Filtered)-1 {
			state.Selected++
			bs.AdjustScroll(state)
		}
		return false, "", nil

	case tea.KeyPgUp, tea.KeyCtrlU:
		// Page up
		state.Selected -= state.Height
		if state.Selected < 0 {
			state.Selected = 0
		}
		bs.AdjustScroll(state)
		return false, "", nil

	case tea.KeyPgDown, tea.KeyCtrlD:
		// Page down
		state.Selected += state.Height
		if state.Selected >= len(state.Filtered) {
			state.Selected = len(state.Filtered) - 1
		}
		if state.Selected < 0 {
			state.Selected = 0
		}
		bs.AdjustScroll(state)
		return false, "", nil

	case tea.KeyBackspace:
		// Remove last character from search
		if len(state.SearchInput) > 0 {
			runes := []rune(state.SearchInput)
			state.SearchInput = string(runes[:len(runes)-1])
			bs.ApplyFilter(state)
		}
		return false, "", nil

	case tea.KeyRunes:
		// Add typed characters to search
		state.SearchInput += string(msg.Runes)
		bs.ApplyFilter(state)
		return false, "", nil

	case tea.KeySpace:
		// Add space to search
		state.SearchInput += " "
		bs.ApplyFilter(state)
		return false, "", nil
	}

	return false, "", nil
}

// ApplyFilter filters the branch list based on search input.
// Updates the state with the filtered results.
func (bs *BranchSelector) ApplyFilter(state *BranchSelectorState) {
	if state.SearchInput == "" {
		state.Filtered = state.BranchList
	} else {
		searchLower := strings.ToLower(state.SearchInput)
		state.Filtered = nil
		for _, name := range state.BranchList {
			if strings.Contains(strings.ToLower(name), searchLower) {
				state.Filtered = append(state.Filtered, name)
			}
		}
	}

	// Reset selection to first item when filter changes
	state.Selected = 0
	state.ScrollOffset = 0

	// Try to keep previously selected branch selected if it's still visible
	if state.SelectedBranch != "" {
		for i, name := range state.Filtered {
			if name == state.SelectedBranch {
				state.Selected = i
				break
			}
		}
	}

	bs.AdjustScroll(state)
}

// AdjustScroll adjusts scroll offset to keep selection visible.
func (bs *BranchSelector) AdjustScroll(state *BranchSelectorState) {
	if state.Height <= 0 {
		return
	}

	// If selection is above viewport, scroll up
	if state.Selected < state.ScrollOffset {
		state.ScrollOffset = state.Selected
	}

	// If selection is below viewport, scroll down
	if state.Selected >= state.ScrollOffset+state.Height {
		state.ScrollOffset = state.Selected - state.Height + 1
	}

	// Clamp scroll offset to valid range [0, maxScroll]
	maxScroll := max(len(state.Filtered)-state.Height, 0)
	state.ScrollOffset = max(min(state.ScrollOffset, maxScroll), 0)
}

// ToInputState converts BranchSelectorState to the fields needed by InputState.
// This is a helper for bridging between the state types.
func (state *BranchSelectorState) ToInputState(mainBranch string) InputStateBranchFields {
	branches := make([]BranchItem, len(state.Filtered))
	for i, name := range state.Filtered {
		branches[i] = BranchItem{
			Name:   name,
			IsMain: name == mainBranch,
		}
	}

	return InputStateBranchFields{
		ShowBranchSelector:   state.Visible,
		Branches:             branches,
		BranchSelected:       state.Selected,
		BranchScrollOffset:   state.ScrollOffset,
		BranchSearchInput:    state.SearchInput,
		SelectedBranch:       state.SelectedBranch,
		BranchSelectorHeight: state.Height,
	}
}

// InputStateBranchFields contains the branch-related fields for InputState.
// This is used to populate InputState from BranchSelectorState.
type InputStateBranchFields struct {
	ShowBranchSelector   bool
	Branches             []BranchItem
	BranchSelected       int
	BranchScrollOffset   int
	BranchSearchInput    string
	SelectedBranch       string
	BranchSelectorHeight int
}

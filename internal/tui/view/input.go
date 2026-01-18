// Package view provides view components for the TUI.
// Views are responsible for rendering state to strings.
// They do not hold state themselves - state is passed in.
package view

import (
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// TemplateItem represents a task template for display in the dropdown.
type TemplateItem struct {
	Command string // The slash command (e.g., "test", "docs")
	Name    string // Display name (e.g., "Run Tests")
}

// BranchItem represents a git branch for display in the branch selector.
type BranchItem struct {
	Name   string // Branch name
	IsMain bool   // Whether this is the main/master branch
}

// InputState holds the state needed to render the input dialog.
// This struct is populated by the Model and passed to the view for rendering.
type InputState struct {
	// Text input state
	Text   string // The current input text
	Cursor int    // Cursor position (0 = before first char)

	// Title and subtitle (optional - uses defaults if empty)
	Title    string // Title shown at top (default: "New Task")
	Subtitle string // Subtitle shown below title (default: "Enter task description:")

	// Template dropdown state
	ShowTemplates    bool           // Whether the template dropdown is visible
	Templates        []TemplateItem // Filtered templates to display
	TemplateSelected int            // Currently highlighted template index

	// Branch selector state
	ShowBranchSelector   bool         // Whether the branch selector is visible
	Branches             []BranchItem // Available branches to select from (filtered)
	BranchSelected       int          // Currently highlighted branch index in filtered list
	BranchScrollOffset   int          // Scroll offset for branch list viewport
	BranchSearchInput    string       // Current search/filter text
	SelectedBranch       string       // The currently selected branch name (shown in UI)
	BranchSelectorHeight int          // Maximum visible branches in the selector
}

// InputView renders the task input dialog.
// It is stateless - all state is passed in via InputState.
type InputView struct{}

// NewInputView creates a new InputView instance.
func NewInputView() *InputView {
	return &InputView{}
}

// Render renders the input dialog to a string.
// The width parameter controls the content box width.
func (v *InputView) Render(state *InputState, width int) string {
	var b strings.Builder

	// Title (use default if not set)
	title := state.Title
	if title == "" {
		title = "New Task"
	}
	b.WriteString(styles.Title.Render(title))
	b.WriteString("\n\n")

	// Show branch selector line
	branchLabel := "Base branch: "
	branchName := state.SelectedBranch
	if branchName == "" {
		branchName = "(default)"
	}
	b.WriteString(styles.Muted.Render(branchLabel))
	b.WriteString(styles.DropdownCommand.Render(branchName))
	b.WriteString("  ")
	b.WriteString(styles.Muted.Render("[Tab to change]"))
	b.WriteString("\n\n")

	// Subtitle (use default if not set)
	subtitle := state.Subtitle
	if subtitle == "" {
		subtitle = "Enter task description:"
	}
	b.WriteString(subtitle + "\n\n")

	// Render text input with cursor
	b.WriteString(v.renderTextInput(state))

	// Show branch selector dropdown if active
	if state.ShowBranchSelector {
		b.WriteString("\n")
		b.WriteString(v.renderBranchSelector(state))
	} else if state.ShowTemplates {
		// Show template dropdown if active (but not when branch selector is open)
		b.WriteString("\n")
		b.WriteString(v.renderTemplateDropdown(state))
	}

	// Help hints
	b.WriteString("\n\n")
	b.WriteString(v.renderHints(state))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// renderTextInput renders the multi-line text input field with cursor.
func (v *InputView) renderTextInput(state *InputState) string {
	var b strings.Builder

	// Convert to runes for proper unicode handling
	runes := []rune(state.Text)

	// Clamp cursor to valid bounds as a safety measure
	cursor := state.Cursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}

	beforeCursor := string(runes[:cursor])
	afterCursor := string(runes[cursor:])

	// Build the display with cursor indicator (block cursor)
	// The cursor line gets a ">" prefix, other lines get "  " prefix
	fullText := beforeCursor + "█" + afterCursor
	lines := strings.Split(fullText, "\n")

	// Find which line contains the cursor
	cursorLineIdx := strings.Count(beforeCursor, "\n")

	for i, line := range lines {
		if i == cursorLineIdx {
			b.WriteString("> " + line)
		} else {
			b.WriteString("  " + line)
		}
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}

	return b.String()
}

// renderTemplateDropdown renders the template selection dropdown.
func (v *InputView) renderTemplateDropdown(state *InputState) string {
	if len(state.Templates) == 0 {
		return styles.Muted.Render("  No matching templates")
	}

	var items []string
	for i, t := range state.Templates {
		cmd := "/" + t.Command
		name := " - " + t.Name

		var item string
		if i == state.TemplateSelected {
			// Selected item - highlight the whole row
			item = styles.DropdownItemSelected.Render(cmd + name)
		} else {
			// Normal item - color command and name differently
			item = styles.DropdownItem.Render(
				styles.DropdownCommand.Render(cmd) +
					styles.DropdownName.Render(name),
			)
		}
		items = append(items, item)
	}

	content := strings.Join(items, "\n")
	return styles.DropdownContainer.Render(content)
}

// renderBranchSelector renders the branch selection dropdown with search and scrolling.
func (v *InputView) renderBranchSelector(state *InputState) string {
	var b strings.Builder

	// Search input field
	b.WriteString(styles.Muted.Render("Search: "))
	b.WriteString(state.BranchSearchInput)
	b.WriteString(styles.Secondary.Render("█")) // Cursor
	b.WriteString("\n")

	if len(state.Branches) == 0 {
		if state.BranchSearchInput != "" {
			b.WriteString(styles.Muted.Render("  No matching branches"))
		} else {
			b.WriteString(styles.Muted.Render("  No branches available"))
		}
		return styles.DropdownContainer.Render(b.String())
	}

	// Calculate visible range with scrolling
	maxVisible := state.BranchSelectorHeight
	if maxVisible <= 0 {
		maxVisible = 10 // Default fallback
	}

	totalBranches := len(state.Branches)
	scrollOffset := state.BranchScrollOffset

	// Clamp scroll offset
	maxScroll := totalBranches - maxVisible
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scrollOffset > maxScroll {
		scrollOffset = maxScroll
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Show scroll indicator if there's content above
	if scrollOffset > 0 {
		b.WriteString(styles.Muted.Render("  ▲ " + formatCount(scrollOffset) + " more above\n"))
	}

	// Render visible branches
	endIdx := scrollOffset + maxVisible
	if endIdx > totalBranches {
		endIdx = totalBranches
	}

	for i := scrollOffset; i < endIdx; i++ {
		branch := state.Branches[i]
		name := branch.Name
		var suffix string
		if branch.IsMain {
			suffix = " (default)"
		}

		var item string
		if i == state.BranchSelected {
			// Selected item - highlight the whole row
			item = styles.DropdownItemSelected.Render(name + suffix)
		} else {
			// Normal item
			item = styles.DropdownItem.Render(
				styles.DropdownCommand.Render(name) +
					styles.Muted.Render(suffix),
			)
		}
		b.WriteString(item)
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Show scroll indicator if there's content below
	remaining := totalBranches - endIdx
	if remaining > 0 {
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("  ▼ " + formatCount(remaining) + " more below"))
	}

	// Show match count
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  " + formatCount(totalBranches) + " branches"))

	return styles.DropdownContainer.Render(b.String())
}

// formatCount formats a count for display.
func formatCount(n int) string {
	return strconv.Itoa(n)
}

// renderHints renders the context-aware keyboard hints.
func (v *InputView) renderHints(state *InputState) string {
	if state.ShowBranchSelector {
		return styles.Muted.Render("↑/↓") + " navigate  " +
			styles.Muted.Render("Type") + " filter  " +
			styles.Muted.Render("Enter/Tab") + " select  " +
			styles.Muted.Render("Esc") + " close"
	}
	if state.ShowTemplates {
		return styles.Muted.Render("↑/↓") + " navigate  " +
			styles.Muted.Render("Enter/Tab") + " select  " +
			styles.Muted.Render("Esc") + " close  " +
			styles.Muted.Render("Type") + " filter"
	}
	return styles.Muted.Render("Enter") + " submit  " +
		styles.Muted.Render("Shift+Enter") + " newline  " +
		styles.Muted.Render("Tab") + " branch  " +
		styles.Muted.Render("/") + " templates  " +
		styles.Muted.Render("Esc") + " cancel"
}

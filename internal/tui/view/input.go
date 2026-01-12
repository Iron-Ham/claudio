// Package view provides view components for the TUI.
// Views are responsible for rendering state to strings.
// They do not hold state themselves - state is passed in.
package view

import (
	"strings"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// TemplateItem represents a task template for display in the dropdown.
type TemplateItem struct {
	Command string // The slash command (e.g., "test", "docs")
	Name    string // Display name (e.g., "Run Tests")
}

// InputState holds the state needed to render the input dialog.
// This struct is populated by the Model and passed to the view for rendering.
type InputState struct {
	// Text input state
	Text   string // The current input text
	Cursor int    // Cursor position (0 = before first char)

	// Title and subtitle (optional - uses defaults if empty)
	Title    string // Title shown at top (default: "Add New Instance")
	Subtitle string // Subtitle shown below title (default: "Enter task description:")

	// Template dropdown state
	ShowTemplates    bool           // Whether the template dropdown is visible
	Templates        []TemplateItem // Filtered templates to display
	TemplateSelected int            // Currently highlighted template index
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
		title = "Add New Instance"
	}
	b.WriteString(styles.Title.Render(title))
	b.WriteString("\n\n")

	// Subtitle (use default if not set)
	subtitle := state.Subtitle
	if subtitle == "" {
		subtitle = "Enter task description:"
	}
	b.WriteString(subtitle + "\n\n")

	// Render text input with cursor
	b.WriteString(v.renderTextInput(state))

	// Show template dropdown if active
	if state.ShowTemplates {
		b.WriteString("\n")
		b.WriteString(v.renderTemplateDropdown(state))
	}

	// Help hints
	b.WriteString("\n\n")
	b.WriteString(v.renderHints(state.ShowTemplates))

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

// renderHints renders the context-aware keyboard hints.
func (v *InputView) renderHints(showTemplates bool) string {
	if showTemplates {
		return styles.Muted.Render("↑/↓") + " navigate  " +
			styles.Muted.Render("Enter/Tab") + " select  " +
			styles.Muted.Render("Esc") + " close  " +
			styles.Muted.Render("Type") + " filter"
	}
	return styles.Muted.Render("Enter") + " submit  " +
		styles.Muted.Render("Shift+Enter") + " newline  " +
		styles.Muted.Render("/") + " templates  " +
		styles.Muted.Render("Esc") + " cancel"
}

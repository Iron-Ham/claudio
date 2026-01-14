package view

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// TemplateDropdownState holds the state needed for the template dropdown.
// This state is owned by the Model and passed to the handler for processing.
type TemplateDropdownState struct {
	ShowTemplates    bool   // Whether the template dropdown is visible
	TemplateFilter   string // Current filter text (after the "/")
	TemplateSelected int    // Currently highlighted template index
	TemplateSuffix   string // Suffix to append on submission (from selected template)
	TaskInput        string // Current task input text
	TaskInputCursor  int    // Cursor position within task input
}

// Template represents a task template for filtering and selection.
// This mirrors the tui.TaskTemplate type to avoid circular imports.
type Template struct {
	Command     string // The slash command (e.g., "test", "docs")
	Name        string // Display name (e.g., "Run Tests")
	Description string // Task description shown to user
	Suffix      string // Optional suffix appended on submission
}

// TemplateFilterFunc is a function that filters templates based on input.
type TemplateFilterFunc func(filter string) []Template

// TemplateDropdownHandler handles keyboard events for the template dropdown.
// It encapsulates the logic for navigating, filtering, and selecting templates.
type TemplateDropdownHandler struct {
	state      *TemplateDropdownState
	filterFunc TemplateFilterFunc
}

// NewTemplateDropdownHandler creates a new handler for template dropdown interactions.
func NewTemplateDropdownHandler(state *TemplateDropdownState, filterFunc TemplateFilterFunc) *TemplateDropdownHandler {
	return &TemplateDropdownHandler{
		state:      state,
		filterFunc: filterFunc,
	}
}

// HandleKey processes a key event for the template dropdown.
// Returns true if the event was handled, false if it should be passed through.
func (h *TemplateDropdownHandler) HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	templates := h.filterFunc(h.state.TemplateFilter)

	switch msg.Type {
	case tea.KeyEsc:
		// Close dropdown but keep the "/" and filter in input
		h.state.ShowTemplates = false
		h.state.TemplateFilter = ""
		h.state.TemplateSelected = 0
		return true, nil

	case tea.KeyEnter, tea.KeyTab:
		// Select the highlighted template
		if len(templates) > 0 && h.state.TemplateSelected < len(templates) {
			selected := templates[h.state.TemplateSelected]
			// Replace the "/" and filter with the template description
			// Find where the "/" starts (could be at beginning or after newline)
			lastNewline := strings.LastIndex(h.state.TaskInput, "\n")
			if lastNewline == -1 {
				// "/" is at the beginning
				h.state.TaskInput = selected.Description
			} else {
				// "/" is after a newline
				h.state.TaskInput = h.state.TaskInput[:lastNewline+1] + selected.Description
			}
			h.state.TaskInputCursor = len([]rune(h.state.TaskInput))
			// Store the suffix to append on submission
			h.state.TemplateSuffix = selected.Suffix
		}
		h.state.ShowTemplates = false
		h.state.TemplateFilter = ""
		h.state.TemplateSelected = 0
		return true, nil

	case tea.KeyUp:
		if h.state.TemplateSelected > 0 {
			h.state.TemplateSelected--
		}
		return true, nil

	case tea.KeyDown:
		if h.state.TemplateSelected < len(templates)-1 {
			h.state.TemplateSelected++
		}
		return true, nil

	case tea.KeyBackspace:
		if len(h.state.TemplateFilter) > 0 {
			// Remove from both filter and taskInput
			h.state.TemplateFilter = h.state.TemplateFilter[:len(h.state.TemplateFilter)-1]
			if len(h.state.TaskInput) > 0 {
				h.state.TaskInput = h.state.TaskInput[:len(h.state.TaskInput)-1]
				h.state.TaskInputCursor = len([]rune(h.state.TaskInput))
			}
			h.state.TemplateSelected = 0 // Reset selection on filter change
		} else {
			// Remove the "/" and close dropdown
			if len(h.state.TaskInput) > 0 {
				h.state.TaskInput = h.state.TaskInput[:len(h.state.TaskInput)-1]
				h.state.TaskInputCursor = len([]rune(h.state.TaskInput))
			}
			h.state.ShowTemplates = false
		}
		return true, nil

	case tea.KeyRunes:
		char := string(msg.Runes)
		// Space closes dropdown and keeps current input, adds space
		if char == " " {
			h.state.ShowTemplates = false
			h.state.TaskInput += " "
			h.state.TaskInputCursor = len([]rune(h.state.TaskInput))
			h.state.TemplateFilter = ""
			h.state.TemplateSelected = 0
			return true, nil
		}
		// Add to both filter and taskInput
		h.state.TemplateFilter += char
		h.state.TaskInput += char
		h.state.TaskInputCursor = len([]rune(h.state.TaskInput))
		h.state.TemplateSelected = 0 // Reset selection on filter change
		// If no templates match, close dropdown
		if len(h.filterFunc(h.state.TemplateFilter)) == 0 {
			h.state.ShowTemplates = false
			h.state.TemplateFilter = ""
		}
		return true, nil
	}

	return false, nil
}

// OpenDropdown opens the template dropdown and resets its state.
func (h *TemplateDropdownHandler) OpenDropdown() {
	h.state.ShowTemplates = true
	h.state.TemplateFilter = ""
	h.state.TemplateSelected = 0
}

// CloseDropdown closes the template dropdown and resets its state.
func (h *TemplateDropdownHandler) CloseDropdown() {
	h.state.ShowTemplates = false
	h.state.TemplateFilter = ""
	h.state.TemplateSelected = 0
}

// ClearSuffix clears the stored template suffix.
func (h *TemplateDropdownHandler) ClearSuffix() {
	h.state.TemplateSuffix = ""
}

// GetSuffix returns the stored template suffix.
func (h *TemplateDropdownHandler) GetSuffix() string {
	return h.state.TemplateSuffix
}

// IsOpen returns whether the template dropdown is currently visible.
func (h *TemplateDropdownHandler) IsOpen() bool {
	return h.state.ShowTemplates
}

// BuildTemplateItems converts filtered templates to TemplateItem for rendering.
func BuildTemplateItems(templates []Template) []TemplateItem {
	items := make([]TemplateItem, len(templates))
	for i, t := range templates {
		items[i] = TemplateItem{
			Command: t.Command,
			Name:    t.Name,
		}
	}
	return items
}

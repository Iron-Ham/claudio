package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// TemplateHandler manages the template dropdown state and logic.
// It provides a filterable dropdown of task templates that can be
// triggered by typing "/" at the start of a line in the task input.
type TemplateHandler struct {
	// visible indicates whether the template dropdown is currently shown
	visible bool

	// filter is the current filter text (characters typed after "/")
	filter string

	// selectedIdx is the index of the currently highlighted template
	selectedIdx int
}

// NewTemplateHandler creates a new TemplateHandler instance.
func NewTemplateHandler() *TemplateHandler {
	return &TemplateHandler{}
}

// IsVisible returns whether the template dropdown is currently visible.
// Returns false if the handler is nil.
func (h *TemplateHandler) IsVisible() bool {
	if h == nil {
		return false
	}
	return h.visible
}

// Filter returns the current filter text.
// Returns empty string if the handler is nil.
func (h *TemplateHandler) Filter() string {
	if h == nil {
		return ""
	}
	return h.filter
}

// SelectedIndex returns the index of the currently selected template.
// Returns 0 if the handler is nil.
func (h *TemplateHandler) SelectedIndex() int {
	if h == nil {
		return 0
	}
	return h.selectedIdx
}

// Show activates the template dropdown and resets filter/selection state.
// No-op if the handler is nil.
func (h *TemplateHandler) Show() {
	if h == nil {
		return
	}
	h.visible = true
	h.filter = ""
	h.selectedIdx = 0
}

// Hide deactivates the template dropdown and resets all state.
// No-op if the handler is nil.
func (h *TemplateHandler) Hide() {
	if h == nil {
		return
	}
	h.visible = false
	h.filter = ""
	h.selectedIdx = 0
}

// FilteredTemplates returns the list of templates matching the current filter.
// Returns all templates if the handler is nil.
func (h *TemplateHandler) FilteredTemplates() []TaskTemplate {
	if h == nil {
		return FilterTemplates("")
	}
	return FilterTemplates(h.filter)
}

// TemplateHandlerResult contains the result of handling a key event.
type TemplateHandlerResult struct {
	// Handled indicates whether the key was consumed by the handler
	Handled bool

	// SelectedTemplate is non-nil when a template was selected
	SelectedTemplate *TaskTemplate

	// FilterChanged indicates the filter text was modified
	FilterChanged bool

	// Closed indicates the dropdown was closed
	Closed bool

	// InputAppend is text to append to the task input
	InputAppend string

	// InputReplace is true when the task input should be replaced with SelectedTemplate.Description
	InputReplace bool
}

// HandleKey processes a key event when the template dropdown is visible.
// It returns a result indicating what action was taken.
// The taskInput parameter is needed to properly replace "/" with template content.
// Returns an unhandled result if the handler is nil.
func (h *TemplateHandler) HandleKey(msg tea.KeyMsg, taskInput string) TemplateHandlerResult {
	if h == nil {
		return TemplateHandlerResult{Handled: false}
	}
	templates := h.FilteredTemplates()

	switch msg.Type {
	case tea.KeyEsc:
		// Close dropdown but keep the "/" and filter in input
		h.Hide()
		return TemplateHandlerResult{Handled: true, Closed: true}

	case tea.KeyEnter, tea.KeyTab:
		// Select the highlighted template
		if len(templates) > 0 && h.selectedIdx < len(templates) {
			selected := templates[h.selectedIdx]
			h.Hide()
			return TemplateHandlerResult{
				Handled:          true,
				SelectedTemplate: &selected,
				InputReplace:     true,
				Closed:           true,
			}
		}
		h.Hide()
		return TemplateHandlerResult{Handled: true, Closed: true}

	case tea.KeyUp:
		if h.selectedIdx > 0 {
			h.selectedIdx--
		}
		return TemplateHandlerResult{Handled: true}

	case tea.KeyDown:
		if h.selectedIdx < len(templates)-1 {
			h.selectedIdx++
		}
		return TemplateHandlerResult{Handled: true}

	case tea.KeyBackspace:
		if len(h.filter) > 0 {
			// Remove from filter
			h.filter = h.filter[:len(h.filter)-1]
			h.selectedIdx = 0 // Reset selection on filter change
			return TemplateHandlerResult{Handled: true, FilterChanged: true}
		}
		// No filter text, closing dropdown (caller should also remove "/")
		h.Hide()
		return TemplateHandlerResult{Handled: true, Closed: true, FilterChanged: true}

	case tea.KeyRunes:
		char := string(msg.Runes)
		// Space closes dropdown and keeps current input, adds space
		if char == " " {
			h.Hide()
			return TemplateHandlerResult{
				Handled:     true,
				Closed:      true,
				InputAppend: " ",
			}
		}
		// Add to filter
		h.filter += char
		h.selectedIdx = 0 // Reset selection on filter change

		// If no templates match, close dropdown
		if len(FilterTemplates(h.filter)) == 0 {
			h.Hide()
			return TemplateHandlerResult{
				Handled:       true,
				Closed:        true,
				FilterChanged: true,
			}
		}
		return TemplateHandlerResult{Handled: true, FilterChanged: true}
	}

	return TemplateHandlerResult{Handled: false}
}

// ComputeReplacementInput calculates the new task input when a template is selected.
// It replaces the "/" and any filter text with the template description.
// Returns empty string if template is nil.
func (h *TemplateHandler) ComputeReplacementInput(taskInput string, template *TaskTemplate) string {
	if template == nil {
		return ""
	}
	// Find where the "/" starts (could be at beginning or after newline)
	lastNewline := strings.LastIndex(taskInput, "\n")
	if lastNewline == -1 {
		// "/" is at the beginning
		return template.Description
	}
	// "/" is after a newline
	return taskInput[:lastNewline+1] + template.Description
}

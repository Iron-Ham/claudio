package filter

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// Category defines a filter category with its display properties.
type Category struct {
	Key      string // Internal key (e.g., "errors")
	Label    string // Display label (e.g., "Errors")
	Shortcut string // Keyboard shortcut (e.g., "e/1")
}

// Categories is the standard set of filter categories.
var Categories = []Category{
	{Key: "errors", Label: "Errors", Shortcut: "e/1"},
	{Key: "warnings", Label: "Warnings", Shortcut: "w/2"},
	{Key: "tools", Label: "Tool calls", Shortcut: "t/3"},
	{Key: "thinking", Label: "Thinking", Shortcut: "h/4"},
	{Key: "progress", Label: "Progress", Shortcut: "p/5"},
}

// Filter manages category-based and regex-based output filtering.
type Filter struct {
	categories    map[string]bool
	customPattern string
	customRegex   *regexp.Regexp
}

// New creates a new Filter with all categories enabled by default.
func New() *Filter {
	f := &Filter{
		categories: make(map[string]bool),
	}
	for _, cat := range Categories {
		f.categories[cat.Key] = true
	}
	return f
}

// NewWithCategories creates a new Filter with the specified category states.
func NewWithCategories(categories map[string]bool) *Filter {
	f := &Filter{
		categories: make(map[string]bool),
	}
	for _, cat := range Categories {
		if enabled, ok := categories[cat.Key]; ok {
			f.categories[cat.Key] = enabled
		} else {
			f.categories[cat.Key] = true
		}
	}
	return f
}

// Categories returns a copy of the current category states.
func (f *Filter) Categories() map[string]bool {
	result := make(map[string]bool)
	for k, v := range f.categories {
		result[k] = v
	}
	return result
}

// SetCategories sets the category states directly.
func (f *Filter) SetCategories(categories map[string]bool) {
	for k, v := range categories {
		f.categories[k] = v
	}
}

// IsCategoryEnabled returns whether a specific category is enabled.
func (f *Filter) IsCategoryEnabled(key string) bool {
	return f.categories[key]
}

// ToggleCategory toggles the enabled state of a category.
func (f *Filter) ToggleCategory(key string) {
	f.categories[key] = !f.categories[key]
}

// ToggleAll toggles all categories at once.
// If all are enabled, disables all; otherwise enables all.
func (f *Filter) ToggleAll() {
	allEnabled := f.AllEnabled()
	for k := range f.categories {
		f.categories[k] = !allEnabled
	}
}

// AllEnabled returns true if all categories are enabled.
func (f *Filter) AllEnabled() bool {
	for _, v := range f.categories {
		if !v {
			return false
		}
	}
	return true
}

// CustomPattern returns the current custom filter pattern.
func (f *Filter) CustomPattern() string {
	return f.customPattern
}

// CustomRegex returns the compiled regex for the custom pattern.
// Returns nil if no pattern is set or the pattern is invalid.
func (f *Filter) CustomRegex() *regexp.Regexp {
	return f.customRegex
}

// SetCustomPattern sets and compiles the custom filter pattern.
// The pattern is compiled as case-insensitive.
// Invalid patterns result in a nil regex.
func (f *Filter) SetCustomPattern(pattern string) {
	f.customPattern = pattern
	f.compileRegex()
}

// ClearCustomPattern clears the custom filter pattern.
func (f *Filter) ClearCustomPattern() {
	f.customPattern = ""
	f.customRegex = nil
}

// AppendToPattern appends a character to the custom pattern.
func (f *Filter) AppendToPattern(char string) {
	f.customPattern += char
	f.compileRegex()
}

// BackspacePattern removes the last character from the custom pattern.
func (f *Filter) BackspacePattern() {
	if len(f.customPattern) > 0 {
		f.customPattern = f.customPattern[:len(f.customPattern)-1]
		f.compileRegex()
	}
}

// compileRegex compiles the custom filter pattern.
func (f *Filter) compileRegex() {
	if f.customPattern == "" {
		f.customRegex = nil
		return
	}

	re, err := regexp.Compile("(?i)" + f.customPattern)
	if err != nil {
		f.customRegex = nil
		return
	}
	f.customRegex = re
}

// HasActiveFilter returns true if any filtering is active.
// Returns false if all categories are enabled and no custom pattern is set.
func (f *Filter) HasActiveFilter() bool {
	return !f.AllEnabled() || f.customRegex != nil
}

// Apply applies the filter to the given output and returns the filtered result.
// If all categories are enabled and no custom pattern is set, returns output unchanged.
func (f *Filter) Apply(output string) string {
	if f.AllEnabled() && f.customRegex == nil {
		return output
	}

	lines := strings.Split(output, "\n")
	var filtered []string

	for _, line := range lines {
		if f.ShouldShowLine(line) {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
}

// ShouldShowLine determines if a line should be shown based on current filters.
func (f *Filter) ShouldShowLine(line string) bool {
	// Custom filter takes precedence
	if f.customRegex != nil {
		return f.customRegex.MatchString(line)
	}

	lineLower := strings.ToLower(line)

	// Check category filters
	if !f.categories["errors"] {
		if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "failed") ||
			strings.Contains(lineLower, "exception") || strings.Contains(lineLower, "panic") {
			return false
		}
	}

	if !f.categories["warnings"] {
		if strings.Contains(lineLower, "warning") || strings.Contains(lineLower, "warn") {
			return false
		}
	}

	if !f.categories["tools"] {
		// Common Claude tool call patterns
		if strings.Contains(lineLower, "read file") || strings.Contains(lineLower, "write file") ||
			strings.Contains(lineLower, "bash") || strings.Contains(lineLower, "running") ||
			strings.HasPrefix(line, "  ") && (strings.Contains(line, "(") || strings.Contains(line, "→")) {
			return false
		}
	}

	if !f.categories["thinking"] {
		if strings.Contains(lineLower, "thinking") || strings.Contains(lineLower, "let me") ||
			strings.Contains(lineLower, "i'll") || strings.Contains(lineLower, "i will") {
			return false
		}
	}

	if !f.categories["progress"] {
		if strings.Contains(line, "...") || strings.Contains(line, "✓") ||
			strings.Contains(line, "█") || strings.Contains(line, "░") {
			return false
		}
	}

	return true
}

// InputResult captures the result of handling a key press in filter mode.
type InputResult struct {
	ExitMode bool // Whether to exit filter mode
}

// HandleKey handles keyboard input when in filter mode.
// Returns an InputResult indicating whether to exit filter mode.
func (f *Filter) HandleKey(msg tea.KeyMsg) InputResult {
	switch msg.String() {
	case "esc", "F", "q":
		return InputResult{ExitMode: true}

	case "e", "1":
		f.ToggleCategory("errors")
		return InputResult{}

	case "w", "2":
		f.ToggleCategory("warnings")
		return InputResult{}

	case "t", "3":
		f.ToggleCategory("tools")
		return InputResult{}

	case "h", "4":
		f.ToggleCategory("thinking")
		return InputResult{}

	case "p", "5":
		f.ToggleCategory("progress")
		return InputResult{}

	case "a":
		f.ToggleAll()
		return InputResult{}

	case "c":
		f.ClearCustomPattern()
		return InputResult{}
	}

	// Handle custom filter input
	switch msg.Type {
	case tea.KeyBackspace:
		f.BackspacePattern()
		return InputResult{}

	case tea.KeyRunes:
		// Check if it's not a shortcut key
		char := string(msg.Runes)
		if char != "e" && char != "w" && char != "t" && char != "h" && char != "p" && char != "a" && char != "c" {
			f.AppendToPattern(char)
		}
		return InputResult{}

	case tea.KeySpace:
		f.AppendToPattern(" ")
		return InputResult{}
	}

	return InputResult{}
}

// RenderPanel renders the filter configuration panel.
func RenderPanel(f *Filter, width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Output Filters"))
	b.WriteString("\n\n")
	b.WriteString(styles.Muted.Render("Toggle categories to show/hide specific output types:"))
	b.WriteString("\n\n")

	// Category checkboxes
	for _, cat := range Categories {
		var checkbox string
		var labelStyle lipgloss.Style
		if f.IsCategoryEnabled(cat.Key) {
			checkbox = styles.FilterCheckbox.Render("[✓]")
			labelStyle = styles.FilterCategoryEnabled
		} else {
			checkbox = styles.FilterCheckboxEmpty.Render("[ ]")
			labelStyle = styles.FilterCategoryDisabled
		}

		line := fmt.Sprintf("%s %s %s",
			checkbox,
			labelStyle.Render(cat.Label),
			styles.Muted.Render("("+cat.Shortcut+")"))
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("[a] Toggle all  [c] Clear custom filter"))
	b.WriteString("\n\n")

	// Custom filter input
	b.WriteString(styles.Secondary.Render("Custom filter:"))
	b.WriteString(" ")
	if f.CustomPattern() != "" {
		b.WriteString(styles.SearchInput.Render(f.CustomPattern()))
	} else {
		b.WriteString(styles.Muted.Render("(type to filter by pattern)"))
	}
	b.WriteString("\n\n")

	// Help text
	b.WriteString(styles.Muted.Render("Category descriptions:"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Errors: Stack traces, error messages, failures"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Warnings: Warning indicators"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Tool calls: File operations, bash commands"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Thinking: Claude's reasoning phrases"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Progress: Progress indicators, spinners"))
	b.WriteString("\n\n")

	b.WriteString(styles.Muted.Render("Press [Esc] or [F] to close"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

package styles

import "github.com/charmbracelet/lipgloss"

// Theme implements panel.Theme by wrapping the styles package.
// This allows panels to use consistent styling with the rest of the app.
//
// The interface is defined in internal/tui/panel/renderer.go to avoid
// circular imports between styles and panel packages.
type Theme struct{}

// NewTheme creates a new Theme instance.
func NewTheme() *Theme {
	return &Theme{}
}

func (t *Theme) Primary() lipgloss.Style   { return Primary }
func (t *Theme) Secondary() lipgloss.Style { return HelpKey } // Use HelpKey for better visibility
func (t *Theme) Muted() lipgloss.Style     { return Muted }
func (t *Theme) Error() lipgloss.Style     { return Error }
func (t *Theme) Warning() lipgloss.Style   { return Warning }
func (t *Theme) Surface() lipgloss.Style   { return Surface }

func (t *Theme) Border() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(BorderColor)
}

func (t *Theme) DiffAdd() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(GreenColor)
}

func (t *Theme) DiffRemove() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(RedColor)
}

func (t *Theme) DiffHeader() lipgloss.Style { return Primary }

func (t *Theme) DiffHunk() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(BlueColor)
}

func (t *Theme) DiffContext() lipgloss.Style { return Muted }

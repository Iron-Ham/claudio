package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// renderHeader renders the header bar with the application title
// and optionally the session name.
func (m Model) renderHeader() string {
	title := "Claudio"
	if m.session != nil && m.session.Name != "" {
		title = fmt.Sprintf("Claudio: %s", m.session.Name)
	}

	return styles.Header.Width(m.width).Render(title)
}

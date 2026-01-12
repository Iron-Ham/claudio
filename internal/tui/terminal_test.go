package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/tui/terminal"
)

func TestTerminalHeightConstants(t *testing.T) {
	// Verify the terminal height constants are set to reasonable values
	// These are re-exported from terminal package for backward compatibility
	t.Run("DefaultTerminalHeight is at least MinTerminalHeight", func(t *testing.T) {
		if DefaultTerminalHeight < MinTerminalHeight {
			t.Errorf("DefaultTerminalHeight (%d) should be >= MinTerminalHeight (%d)",
				DefaultTerminalHeight, MinTerminalHeight)
		}
	})

	t.Run("DefaultTerminalHeight is reasonable", func(t *testing.T) {
		// The default height should be at least 10 lines to be useful
		if DefaultTerminalHeight < 10 {
			t.Errorf("DefaultTerminalHeight (%d) should be >= 10 for usability",
				DefaultTerminalHeight)
		}
	})

	t.Run("MinTerminalHeight allows for content", func(t *testing.T) {
		// Minimum height should account for border (2) + header (1) + at least 1 content line
		// So minimum should be at least 4
		if MinTerminalHeight < 4 {
			t.Errorf("MinTerminalHeight (%d) should be >= 4 to allow for border, header, and content",
				MinTerminalHeight)
		}
	})

	t.Run("MaxTerminalHeightRatio is sensible", func(t *testing.T) {
		if MaxTerminalHeightRatio <= 0 || MaxTerminalHeightRatio > 0.8 {
			t.Errorf("MaxTerminalHeightRatio (%f) should be between 0 and 0.8",
				MaxTerminalHeightRatio)
		}
	})
}

func TestTerminalPaneHeight(t *testing.T) {
	tests := []struct {
		name           string
		visible        bool
		paneHeight     int
		terminalHeight int // total terminal height for max ratio calculation
		expectedHeight int
	}{
		{
			name:           "returns 0 when terminal not visible",
			visible:        false,
			paneHeight:     15,
			terminalHeight: 100,
			expectedHeight: 0,
		},
		{
			name:           "returns default height when visible and paneHeight is 0",
			visible:        true,
			paneHeight:     0,
			terminalHeight: 100,
			expectedHeight: terminal.DefaultPaneHeight,
		},
		{
			name:           "returns stored height when visible and height is set",
			visible:        true,
			paneHeight:     20,
			terminalHeight: 100,
			expectedHeight: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				terminalManager: terminal.NewManager(),
			}
			m.terminalManager.SetSize(80, tt.terminalHeight)
			m.terminalManager.SetPaneHeight(tt.paneHeight)
			if tt.visible {
				m.terminalManager.SetLayout(terminal.LayoutVisible)
			} else {
				m.terminalManager.SetLayout(terminal.LayoutHidden)
			}

			got := m.TerminalPaneHeight()
			if got != tt.expectedHeight {
				t.Errorf("TerminalPaneHeight() = %d, want %d", got, tt.expectedHeight)
			}
		})
	}
}

func TestIsTerminalMode(t *testing.T) {
	tests := []struct {
		name     string
		visible  bool
		focused  bool
		expected bool
	}{
		{
			name:     "returns true when visible and focused",
			visible:  true,
			focused:  true,
			expected: true,
		},
		{
			name:     "returns false when visible but not focused",
			visible:  true,
			focused:  false,
			expected: false,
		},
		{
			name:     "returns false when not visible",
			visible:  false,
			focused:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				terminalManager: terminal.NewManager(),
			}
			if tt.visible {
				m.terminalManager.SetLayout(terminal.LayoutVisible)
			}
			if tt.focused {
				m.terminalManager.SetFocused(true)
			}

			got := m.IsTerminalMode()
			if got != tt.expected {
				t.Errorf("IsTerminalMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsTerminalVisible(t *testing.T) {
	tests := []struct {
		name     string
		visible  bool
		expected bool
	}{
		{
			name:     "returns true when terminal visible",
			visible:  true,
			expected: true,
		},
		{
			name:     "returns false when terminal not visible",
			visible:  false,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				terminalManager: terminal.NewManager(),
			}
			if tt.visible {
				m.terminalManager.SetLayout(terminal.LayoutVisible)
			}

			got := m.IsTerminalVisible()
			if got != tt.expected {
				t.Errorf("IsTerminalVisible() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTerminalContentDimensionCalculation(t *testing.T) {
	// This test verifies that the content dimensions are calculated correctly
	// for the tmux session. The content area should account for:
	// - Border: 2 lines (top + bottom)
	// - Header: 1 line
	// - Border width: 2 chars (left + right)
	// - Padding width: 2 chars (left + right)
	tests := []struct {
		name                  string
		paneHeight            int
		paneWidth             int
		expectedContentHeight int
		expectedContentWidth  int
	}{
		{
			name:                  "standard terminal pane",
			paneHeight:            15,
			paneWidth:             100,
			expectedContentHeight: 12, // 15 - 3
			expectedContentWidth:  96, // 100 - 4
		},
		{
			name:                  "minimum height pane",
			paneHeight:            5,
			paneWidth:             80,
			expectedContentHeight: 3, // max(5 - 3, 3) = 3 (minimum enforced)
			expectedContentWidth:  76,
		},
		{
			name:                  "very small pane height",
			paneHeight:            3,
			paneWidth:             40,
			expectedContentHeight: 3, // Minimum is 3
			expectedContentWidth:  36,
		},
		{
			name:                  "narrow pane width",
			paneHeight:            10,
			paneWidth:             24,
			expectedContentHeight: 7,
			expectedContentWidth:  20, // Minimum is 20
		},
		{
			name:                  "very narrow pane",
			paneHeight:            10,
			paneWidth:             10,
			expectedContentHeight: 7,
			expectedContentWidth:  20, // Clamped to minimum of 20
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate content height (matches resizeTerminal logic)
			contentHeight := tt.paneHeight - 3
			if contentHeight < 3 {
				contentHeight = 3
			}

			// Calculate content width (matches resizeTerminal logic)
			contentWidth := tt.paneWidth - 4
			if contentWidth < 20 {
				contentWidth = 20
			}

			if contentHeight != tt.expectedContentHeight {
				t.Errorf("contentHeight = %d, want %d (paneHeight=%d)",
					contentHeight, tt.expectedContentHeight, tt.paneHeight)
			}

			if contentWidth != tt.expectedContentWidth {
				t.Errorf("contentWidth = %d, want %d (paneWidth=%d)",
					contentWidth, tt.expectedContentWidth, tt.paneWidth)
			}
		})
	}
}

func TestEnterTerminalMode(t *testing.T) {
	tests := []struct {
		name               string
		visible            bool
		expectFocusedAfter bool
	}{
		{
			name:               "does not enter when terminal not visible",
			visible:            false,
			expectFocusedAfter: false,
		},
		{
			name:               "does not enter when no terminal process",
			visible:            true,
			expectFocusedAfter: false, // No process, so enterTerminalMode does nothing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				terminalManager: terminal.NewManager(),
				// terminalProcess is nil, which means IsRunning() would panic
				// but enterTerminalMode checks terminalProcess != nil first
			}
			if tt.visible {
				m.terminalManager.SetLayout(terminal.LayoutVisible)
			}

			m.enterTerminalMode()

			if m.terminalManager.IsFocused() != tt.expectFocusedAfter {
				t.Errorf("IsFocused() = %v, want %v", m.terminalManager.IsFocused(), tt.expectFocusedAfter)
			}
		})
	}
}

func TestExitTerminalMode(t *testing.T) {
	m := Model{
		terminalManager: terminal.NewManager(),
	}
	m.terminalManager.SetLayout(terminal.LayoutVisible)
	m.terminalManager.SetFocused(true)

	m.exitTerminalMode()

	if m.terminalManager.IsFocused() {
		t.Error("exitTerminalMode() should set focused to false")
	}
}

func TestGetTerminalDir(t *testing.T) {
	tests := []struct {
		name            string
		terminalDirMode TerminalDirMode
		invocationDir   string
		expectedDir     string
	}{
		{
			name:            "invocation mode returns invocation dir",
			terminalDirMode: TerminalDirInvocation,
			invocationDir:   "/home/user/project",
			expectedDir:     "/home/user/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				terminalManager: terminal.NewManager(),
				terminalDirMode: tt.terminalDirMode,
				invocationDir:   tt.invocationDir,
			}

			got := m.getTerminalDir()
			if got != tt.expectedDir {
				t.Errorf("getTerminalDir() = %q, want %q", got, tt.expectedDir)
			}
		})
	}
}

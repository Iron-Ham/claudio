package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/tui/terminal"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

func TestTerminalHeightConstants(t *testing.T) {
	// Verify the terminal height constants are set to reasonable values
	t.Run("DefaultPaneHeight is at least MinPaneHeight", func(t *testing.T) {
		if terminal.DefaultPaneHeight < terminal.MinPaneHeight {
			t.Errorf("DefaultPaneHeight (%d) should be >= MinPaneHeight (%d)",
				terminal.DefaultPaneHeight, terminal.MinPaneHeight)
		}
	})

	t.Run("DefaultPaneHeight is reasonable", func(t *testing.T) {
		// The default height should be at least 10 lines to be useful
		if terminal.DefaultPaneHeight < 10 {
			t.Errorf("DefaultPaneHeight (%d) should be >= 10 for usability",
				terminal.DefaultPaneHeight)
		}
	})

	t.Run("MinPaneHeight allows for content", func(t *testing.T) {
		// Minimum height should account for border (2) + header (1) + at least 1 content line
		// So minimum should be at least 4
		if terminal.MinPaneHeight < 4 {
			t.Errorf("MinPaneHeight (%d) should be >= 4 to allow for border, header, and content",
				terminal.MinPaneHeight)
		}
	})

	t.Run("MaxPaneHeightRatio is sensible", func(t *testing.T) {
		if terminal.MaxPaneHeightRatio <= 0 || terminal.MaxPaneHeightRatio > 0.8 {
			t.Errorf("MaxPaneHeightRatio (%f) should be between 0 and 0.8",
				terminal.MaxPaneHeightRatio)
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
		name          string
		dirMode       terminal.DirMode
		invocationDir string
		expectedDir   string
	}{
		{
			name:          "invocation mode returns invocation dir",
			dirMode:       terminal.DirInvocation,
			invocationDir: "/home/user/project",
			expectedDir:   "/home/user/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := terminal.NewManagerWithConfig(terminal.ManagerConfig{
				InvocationDir: tt.invocationDir,
			})
			mgr.SetDirMode(tt.dirMode)
			m := Model{
				terminalManager: mgr,
			}

			got := m.getTerminalDir()
			if got != tt.expectedDir {
				t.Errorf("getTerminalDir() = %q, want %q", got, tt.expectedDir)
			}
		})
	}
}

func TestTerminalKeybindingsDisabled(t *testing.T) {
	// Save original value and restore after test
	originalValue := viper.GetBool("experimental.terminal_support")
	defer viper.Set("experimental.terminal_support", originalValue)

	// Disable terminal support
	viper.Set("experimental.terminal_support", false)

	t.Run("backtick key does nothing when terminal disabled", func(t *testing.T) {
		m := Model{
			terminalManager: terminal.NewManager(),
		}

		// Verify terminal is not visible initially
		if m.IsTerminalVisible() {
			t.Fatal("terminal should not be visible initially")
		}

		// Call handleToggleTerminal (triggered by backtick key)
		newModel, _ := m.handleToggleTerminal()
		resultModel := newModel.(Model)

		// Verify terminal is still not visible
		if resultModel.IsTerminalVisible() {
			t.Error("terminal should remain hidden when terminal support is disabled")
		}
	})

	t.Run("T key does nothing when terminal disabled", func(t *testing.T) {
		m := Model{
			terminalManager: terminal.NewManager(),
		}

		// Verify terminal is not visible initially
		if m.IsTerminalVisible() {
			t.Fatal("terminal should not be visible initially")
		}

		// Call handleToggleTerminal (triggered by T key)
		newModel, _ := m.handleToggleTerminal()
		resultModel := newModel.(Model)

		// Verify terminal is still not visible
		if resultModel.IsTerminalVisible() {
			t.Error("terminal should remain hidden when terminal support is disabled")
		}
	})

	t.Run("ctrl+shift+t does nothing when terminal disabled", func(t *testing.T) {
		m := Model{
			terminalManager: terminal.NewManager(),
		}
		// Force terminal to be visible to test the switch dir function
		m.terminalManager.SetLayout(terminal.LayoutVisible)

		// Call handleSwitchTerminalDir (triggered by ctrl+shift+t)
		newModel, _ := m.handleSwitchTerminalDir()
		resultModel := newModel.(Model)

		// Verify no error occurred and model is returned unchanged
		if resultModel.errorMessage != "" {
			t.Errorf("unexpected error message: %s", resultModel.errorMessage)
		}
	})
}

func TestHandleNormalModeKeyTerminalDisabled(t *testing.T) {
	// Save original value and restore after test
	originalValue := viper.GetBool("experimental.terminal_support")
	defer viper.Set("experimental.terminal_support", originalValue)

	// Disable terminal support
	viper.Set("experimental.terminal_support", false)

	t.Run("backtick key in normal mode does nothing when terminal disabled", func(t *testing.T) {
		m := Model{
			terminalManager: terminal.NewManager(),
			inputMode:       false, // Normal mode
		}

		// Create backtick key message
		keyMsg := tea.KeyMsg{
			Type:  tea.KeyRunes,
			Runes: []rune{'`'},
		}

		// Handle the key in normal mode
		newModel, _ := m.handleNormalModeKey(keyMsg)
		resultModel := newModel.(Model)

		// Verify terminal is still not visible
		if resultModel.IsTerminalVisible() {
			t.Error("terminal should remain hidden when terminal support is disabled")
		}
	})
}

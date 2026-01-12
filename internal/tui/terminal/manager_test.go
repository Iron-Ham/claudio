package terminal

import "testing"

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m.paneHeight != DefaultPaneHeight {
		t.Errorf("NewManager().paneHeight = %d, want %d", m.paneHeight, DefaultPaneHeight)
	}
	if m.layout != LayoutHidden {
		t.Errorf("NewManager().layout = %v, want LayoutHidden", m.layout)
	}
	if m.focused {
		t.Error("NewManager().focused = true, want false")
	}
}

func TestSetSize(t *testing.T) {
	m := NewManager()
	m.SetSize(120, 40)

	if m.Width() != 120 {
		t.Errorf("Width() = %d, want 120", m.Width())
	}
	if m.Height() != 40 {
		t.Errorf("Height() = %d, want 40", m.Height())
	}
}

func TestGetPaneDimensions_Hidden(t *testing.T) {
	m := NewManager()
	m.SetSize(120, 40)
	m.SetLayout(LayoutHidden)

	dims := m.GetPaneDimensions()

	if dims.TerminalPaneHeight != 0 {
		t.Errorf("TerminalPaneHeight = %d, want 0 when hidden", dims.TerminalPaneHeight)
	}
	if dims.TerminalPaneContentHeight != 0 {
		t.Errorf("TerminalPaneContentHeight = %d, want 0 when hidden", dims.TerminalPaneContentHeight)
	}
	if dims.TerminalPaneContentWidth != 0 {
		t.Errorf("TerminalPaneContentWidth = %d, want 0 when hidden", dims.TerminalPaneContentWidth)
	}

	// Main area should be full height minus reserved space
	expectedMainArea := 40 - 6 // height - headerFooterReserved
	if dims.MainAreaHeight != expectedMainArea {
		t.Errorf("MainAreaHeight = %d, want %d", dims.MainAreaHeight, expectedMainArea)
	}
}

func TestGetPaneDimensions_Visible(t *testing.T) {
	m := NewManager()
	m.SetSize(120, 40)
	m.SetLayout(LayoutVisible)

	dims := m.GetPaneDimensions()

	// Terminal pane should have default height
	if dims.TerminalPaneHeight != DefaultPaneHeight {
		t.Errorf("TerminalPaneHeight = %d, want %d", dims.TerminalPaneHeight, DefaultPaneHeight)
	}

	// Content height = pane height - 3 (borders + header)
	expectedContentHeight := DefaultPaneHeight - 3
	if dims.TerminalPaneContentHeight != expectedContentHeight {
		t.Errorf("TerminalPaneContentHeight = %d, want %d", dims.TerminalPaneContentHeight, expectedContentHeight)
	}

	// Content width = terminal width - 4 (borders + padding)
	expectedContentWidth := 120 - 4
	if dims.TerminalPaneContentWidth != expectedContentWidth {
		t.Errorf("TerminalPaneContentWidth = %d, want %d", dims.TerminalPaneContentWidth, expectedContentWidth)
	}

	// Main area should be reduced by terminal pane height + spacing
	expectedMainArea := 40 - 6 - DefaultPaneHeight - TerminalPaneSpacing
	if dims.MainAreaHeight != expectedMainArea {
		t.Errorf("MainAreaHeight = %d, want %d", dims.MainAreaHeight, expectedMainArea)
	}
}

func TestGetPaneDimensions_MinMainAreaHeight(t *testing.T) {
	m := NewManager()
	// Very short terminal where main area would be negative without minimum
	m.SetSize(80, 20)
	m.SetLayout(LayoutVisible)
	m.SetPaneHeight(30) // Try to set huge pane height

	dims := m.GetPaneDimensions()

	// Main area should be at least 10
	if dims.MainAreaHeight < 10 {
		t.Errorf("MainAreaHeight = %d, want >= 10", dims.MainAreaHeight)
	}
}

func TestGetPaneDimensions_MinContentDimensions(t *testing.T) {
	m := NewManager()
	// Very small terminal
	m.SetSize(15, 15)
	m.SetLayout(LayoutVisible)
	m.SetPaneHeight(MinPaneHeight)

	dims := m.GetPaneDimensions()

	// Content height should be at least 3
	if dims.TerminalPaneContentHeight < 3 {
		t.Errorf("TerminalPaneContentHeight = %d, want >= 3", dims.TerminalPaneContentHeight)
	}

	// Content width should be at least 20
	if dims.TerminalPaneContentWidth < 20 {
		t.Errorf("TerminalPaneContentWidth = %d, want >= 20", dims.TerminalPaneContentWidth)
	}
}

func TestToggleFocus(t *testing.T) {
	tests := []struct {
		name           string
		initialLayout  LayoutMode
		initialFocused bool
		wantFocused    bool
	}{
		{
			name:           "toggle focus when visible and unfocused",
			initialLayout:  LayoutVisible,
			initialFocused: false,
			wantFocused:    true,
		},
		{
			name:           "toggle focus when visible and focused",
			initialLayout:  LayoutVisible,
			initialFocused: true,
			wantFocused:    false,
		},
		{
			name:           "cannot focus when hidden",
			initialLayout:  LayoutHidden,
			initialFocused: false,
			wantFocused:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetLayout(tt.initialLayout)
			if tt.initialFocused {
				m.SetFocused(true)
			}

			got := m.ToggleFocus()
			if got != tt.wantFocused {
				t.Errorf("ToggleFocus() = %v, want %v", got, tt.wantFocused)
			}
			if m.IsFocused() != tt.wantFocused {
				t.Errorf("IsFocused() = %v, want %v", m.IsFocused(), tt.wantFocused)
			}
		})
	}
}

func TestSetFocused(t *testing.T) {
	tests := []struct {
		name          string
		layout        LayoutMode
		setFocused    bool
		expectFocused bool
	}{
		{
			name:          "set focused when visible",
			layout:        LayoutVisible,
			setFocused:    true,
			expectFocused: true,
		},
		{
			name:          "clear focus when visible",
			layout:        LayoutVisible,
			setFocused:    false,
			expectFocused: false,
		},
		{
			name:          "cannot focus when hidden",
			layout:        LayoutHidden,
			setFocused:    true,
			expectFocused: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetLayout(tt.layout)
			m.SetFocused(tt.setFocused)

			if m.IsFocused() != tt.expectFocused {
				t.Errorf("IsFocused() = %v, want %v", m.IsFocused(), tt.expectFocused)
			}
		})
	}
}

func TestSetLayout(t *testing.T) {
	m := NewManager()

	// Initially hidden
	if m.Layout() != LayoutHidden {
		t.Errorf("initial Layout() = %v, want LayoutHidden", m.Layout())
	}
	if m.IsVisible() {
		t.Error("initial IsVisible() = true, want false")
	}

	// Set to visible
	m.SetLayout(LayoutVisible)
	if m.Layout() != LayoutVisible {
		t.Errorf("Layout() = %v, want LayoutVisible", m.Layout())
	}
	if !m.IsVisible() {
		t.Error("IsVisible() = false, want true")
	}

	// Focus then hide should clear focus
	m.SetFocused(true)
	if !m.IsFocused() {
		t.Error("IsFocused() = false, want true after SetFocused(true)")
	}

	m.SetLayout(LayoutHidden)
	if m.IsFocused() {
		t.Error("IsFocused() = true, want false after hiding")
	}
}

func TestToggleVisibility(t *testing.T) {
	m := NewManager()

	// Initially hidden
	if m.IsVisible() {
		t.Error("initial IsVisible() = true, want false")
	}

	// Toggle to visible
	visible := m.ToggleVisibility()
	if !visible {
		t.Error("ToggleVisibility() = false, want true")
	}
	if !m.IsVisible() {
		t.Error("IsVisible() = false, want true")
	}

	// Set focus then toggle to hidden - should clear focus
	m.SetFocused(true)
	visible = m.ToggleVisibility()
	if visible {
		t.Error("ToggleVisibility() = true, want false")
	}
	if m.IsVisible() {
		t.Error("IsVisible() = true, want false")
	}
	if m.IsFocused() {
		t.Error("IsFocused() = true after toggle to hidden, want false")
	}
}

func TestSetPaneHeight(t *testing.T) {
	m := NewManager()

	m.SetPaneHeight(20)
	if m.PaneHeight() != 20 {
		t.Errorf("PaneHeight() = %d, want 20", m.PaneHeight())
	}
}

func TestEffectivePaneHeight(t *testing.T) {
	tests := []struct {
		name            string
		terminalHeight  int
		setPaneHeight   int
		expectedEffectv int
	}{
		{
			name:            "default height when zero",
			terminalHeight:  100,
			setPaneHeight:   0,
			expectedEffectv: DefaultPaneHeight,
		},
		{
			name:            "custom height within bounds",
			terminalHeight:  100,
			setPaneHeight:   20,
			expectedEffectv: 20,
		},
		{
			name:            "clamp to minimum",
			terminalHeight:  100,
			setPaneHeight:   2,
			expectedEffectv: MinPaneHeight,
		},
		{
			name:            "clamp to maximum ratio",
			terminalHeight:  40,
			setPaneHeight:   30,
			expectedEffectv: 20, // 40 * 0.5 = 20
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetSize(80, tt.terminalHeight)
			m.SetPaneHeight(tt.setPaneHeight)
			m.SetLayout(LayoutVisible)

			dims := m.GetPaneDimensions()
			if dims.TerminalPaneHeight != tt.expectedEffectv {
				t.Errorf("TerminalPaneHeight = %d, want %d", dims.TerminalPaneHeight, tt.expectedEffectv)
			}
		})
	}
}

func TestResizePaneHeight(t *testing.T) {
	tests := []struct {
		name           string
		initialHeight  int
		delta          int
		expectedHeight int
	}{
		{
			name:           "increase height",
			initialHeight:  15,
			delta:          5,
			expectedHeight: 20,
		},
		{
			name:           "decrease height",
			initialHeight:  20,
			delta:          -5,
			expectedHeight: 15,
		},
		{
			name:           "clamp to minimum when decreasing",
			initialHeight:  10,
			delta:          -10,
			expectedHeight: MinPaneHeight,
		},
		{
			name:           "clamp to minimum with large negative delta",
			initialHeight:  15,
			delta:          -100,
			expectedHeight: MinPaneHeight,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetPaneHeight(tt.initialHeight)
			m.ResizePaneHeight(tt.delta)

			if m.PaneHeight() != tt.expectedHeight {
				t.Errorf("PaneHeight() = %d, want %d", m.PaneHeight(), tt.expectedHeight)
			}
		})
	}
}

func TestLayoutModeConstants(t *testing.T) {
	// Ensure LayoutHidden is zero value (default)
	if LayoutHidden != 0 {
		t.Errorf("LayoutHidden = %d, want 0", LayoutHidden)
	}
	if LayoutVisible != 1 {
		t.Errorf("LayoutVisible = %d, want 1", LayoutVisible)
	}
}

func TestPaneDimensions_TerminalDimensions(t *testing.T) {
	m := NewManager()
	m.SetSize(120, 40)

	dims := m.GetPaneDimensions()

	if dims.TerminalWidth != 120 {
		t.Errorf("TerminalWidth = %d, want 120", dims.TerminalWidth)
	}
	if dims.TerminalHeight != 40 {
		t.Errorf("TerminalHeight = %d, want 40", dims.TerminalHeight)
	}
}

func TestIsFocused_RequiresBothFocusAndVisible(t *testing.T) {
	m := NewManager()

	// Not focused, not visible
	if m.IsFocused() {
		t.Error("IsFocused() = true when not focused and not visible")
	}

	// Set visible but not focused
	m.SetLayout(LayoutVisible)
	if m.IsFocused() {
		t.Error("IsFocused() = true when visible but not focused")
	}

	// Set focused and visible
	m.SetFocused(true)
	if !m.IsFocused() {
		t.Error("IsFocused() = false when focused and visible")
	}

	// Hide but keep focused flag (should return false)
	m.SetLayout(LayoutHidden)
	// Note: SetLayout clears focused when hiding
	if m.IsFocused() {
		t.Error("IsFocused() = true when focused flag set but hidden")
	}
}

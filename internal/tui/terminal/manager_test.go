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

	dims := m.GetPaneDimensions(0)

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

	dims := m.GetPaneDimensions(0)

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

	dims := m.GetPaneDimensions(0)

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

	dims := m.GetPaneDimensions(0)

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

			dims := m.GetPaneDimensions(0)
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

	dims := m.GetPaneDimensions(0)

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

func TestGetPaneDimensions_WithExtraFooterLines(t *testing.T) {
	tests := []struct {
		name             string
		terminalHeight   int
		extraFooterLines int
		expectedMainArea int
	}{
		{
			name:             "no extra lines",
			terminalHeight:   40,
			extraFooterLines: 0,
			expectedMainArea: 40 - 6, // height - base headerFooterReserved
		},
		{
			name:             "one extra line for error message",
			terminalHeight:   40,
			extraFooterLines: 1,
			expectedMainArea: 40 - 6 - 1,
		},
		{
			name:             "two extra lines for error and conflicts",
			terminalHeight:   40,
			extraFooterLines: 2,
			expectedMainArea: 40 - 6 - 2,
		},
		{
			name:             "three extra lines for verbose help",
			terminalHeight:   40,
			extraFooterLines: 3,
			expectedMainArea: 40 - 6 - 3,
		},
		{
			name:             "negative lines clamped to zero",
			terminalHeight:   40,
			extraFooterLines: -5,
			expectedMainArea: 40 - 6, // negative clamped to 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetSize(80, tt.terminalHeight)
			m.SetLayout(LayoutHidden) // Keep terminal pane hidden for simplicity

			dims := m.GetPaneDimensions(tt.extraFooterLines)

			if dims.MainAreaHeight != tt.expectedMainArea {
				t.Errorf("MainAreaHeight = %d, want %d", dims.MainAreaHeight, tt.expectedMainArea)
			}
		})
	}
}

func TestGetPaneDimensions_ExtraFooterLinesWithTerminalPane(t *testing.T) {
	m := NewManager()
	m.SetSize(120, 50)
	m.SetLayout(LayoutVisible)

	dims := m.GetPaneDimensions(2) // error message + conflict warning

	// Expected: height - headerFooterReserved - extraFooterLines - terminalPaneHeight - spacing
	// 50 - 6 - 2 - 15 - 1 = 26
	expectedMainArea := 50 - 6 - 2 - DefaultPaneHeight - TerminalPaneSpacing
	if dims.MainAreaHeight != expectedMainArea {
		t.Errorf("MainAreaHeight = %d, want %d", dims.MainAreaHeight, expectedMainArea)
	}
}

// -----------------------------------------------------------------------------
// Tests for new Manager methods
// -----------------------------------------------------------------------------

func TestNewManagerWithConfig(t *testing.T) {
	cfg := ManagerConfig{
		InvocationDir: "/home/user/project",
		Logger:        nil,
	}

	m := NewManagerWithConfig(cfg)

	if m.invocationDir != cfg.InvocationDir {
		t.Errorf("invocationDir = %q, want %q", m.invocationDir, cfg.InvocationDir)
	}
	if m.paneHeight != DefaultPaneHeight {
		t.Errorf("paneHeight = %d, want %d", m.paneHeight, DefaultPaneHeight)
	}
	if m.layout != LayoutHidden {
		t.Errorf("layout = %v, want LayoutHidden", m.layout)
	}
	if m.dirMode != DirInvocation {
		t.Errorf("dirMode = %v, want DirInvocation", m.dirMode)
	}
}

func TestSetInvocationDir(t *testing.T) {
	m := NewManager()
	m.SetInvocationDir("/test/dir")

	if m.invocationDir != "/test/dir" {
		t.Errorf("invocationDir = %q, want %q", m.invocationDir, "/test/dir")
	}
}

func TestDirMode(t *testing.T) {
	m := NewManager()

	// Initially in invocation mode
	if m.DirMode() != DirInvocation {
		t.Errorf("initial DirMode() = %v, want DirInvocation", m.DirMode())
	}

	// Set to worktree mode
	m.SetDirMode(DirWorktree)
	if m.DirMode() != DirWorktree {
		t.Errorf("DirMode() = %v, want DirWorktree", m.DirMode())
	}

	// Set back to invocation mode
	m.SetDirMode(DirInvocation)
	if m.DirMode() != DirInvocation {
		t.Errorf("DirMode() = %v, want DirInvocation", m.DirMode())
	}
}

// mockInstanceProvider implements ActiveInstanceProvider for testing
type mockInstanceProvider struct {
	worktreePath string
}

func (p mockInstanceProvider) WorktreePath() string {
	return p.worktreePath
}

func TestGetDir(t *testing.T) {
	tests := []struct {
		name          string
		dirMode       DirMode
		invocationDir string
		worktreePath  string
		expectedDir   string
	}{
		{
			name:          "invocation mode returns invocation dir",
			dirMode:       DirInvocation,
			invocationDir: "/home/user/project",
			worktreePath:  "/tmp/worktree",
			expectedDir:   "/home/user/project",
		},
		{
			name:          "worktree mode returns worktree path",
			dirMode:       DirWorktree,
			invocationDir: "/home/user/project",
			worktreePath:  "/tmp/worktree",
			expectedDir:   "/tmp/worktree",
		},
		{
			name:          "worktree mode with empty path falls back to invocation",
			dirMode:       DirWorktree,
			invocationDir: "/home/user/project",
			worktreePath:  "",
			expectedDir:   "/home/user/project",
		},
		{
			name:          "worktree mode with nil provider falls back to invocation",
			dirMode:       DirWorktree,
			invocationDir: "/home/user/project",
			worktreePath:  "", // nil provider scenario
			expectedDir:   "/home/user/project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManagerWithConfig(ManagerConfig{
				InvocationDir: tt.invocationDir,
			})
			m.SetDirMode(tt.dirMode)

			var provider ActiveInstanceProvider
			if tt.worktreePath != "" {
				provider = mockInstanceProvider{worktreePath: tt.worktreePath}
			}

			got := m.GetDir(provider)
			if got != tt.expectedDir {
				t.Errorf("GetDir() = %q, want %q", got, tt.expectedDir)
			}
		})
	}
}

func TestEnterMode(t *testing.T) {
	m := NewManager()

	// Cannot enter mode when not visible
	m.EnterMode()
	if m.IsFocused() {
		t.Error("EnterMode() should not focus when pane is hidden")
	}

	// Make visible but no process - still cannot enter
	m.SetLayout(LayoutVisible)
	m.EnterMode()
	if m.IsFocused() {
		t.Error("EnterMode() should not focus when no process exists")
	}
}

func TestExitMode(t *testing.T) {
	m := NewManager()
	m.SetLayout(LayoutVisible)
	m.SetFocused(true)

	if !m.IsFocused() {
		t.Error("precondition: manager should be focused")
	}

	m.ExitMode()
	if m.IsFocused() {
		t.Error("ExitMode() should clear focus")
	}
}

func TestSwitchDir(t *testing.T) {
	tests := []struct {
		name            string
		initialMode     DirMode
		expectedMode    DirMode
		expectedInfoMsg string
	}{
		{
			name:            "switch from invocation to worktree without process",
			initialMode:     DirInvocation,
			expectedMode:    DirWorktree,
			expectedInfoMsg: "Terminal will use worktree when opened",
		},
		{
			name:            "switch from worktree to invocation without process",
			initialMode:     DirWorktree,
			expectedMode:    DirInvocation,
			expectedInfoMsg: "Terminal will use invocation directory when opened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetDirMode(tt.initialMode)
			m.SetInvocationDir("/home/user/project")

			infoMsg, errMsg := m.SwitchDir(nil)

			if m.DirMode() != tt.expectedMode {
				t.Errorf("DirMode() = %v, want %v", m.DirMode(), tt.expectedMode)
			}
			if errMsg != "" {
				t.Errorf("SwitchDir() errMsg = %q, want empty", errMsg)
			}
			if infoMsg != tt.expectedInfoMsg {
				t.Errorf("SwitchDir() infoMsg = %q, want %q", infoMsg, tt.expectedInfoMsg)
			}
		})
	}
}

func TestUpdateOutput_NoProcess(t *testing.T) {
	m := NewManager()

	// Should not panic with no process
	m.UpdateOutput()

	if m.Output() != "" {
		t.Error("Output() should be empty when no process exists")
	}
}

func TestOutput(t *testing.T) {
	m := NewManager()

	// Initially empty
	if m.Output() != "" {
		t.Error("initial Output() should be empty")
	}

	// Set output directly (simulating what UpdateOutput would do)
	m.output = "test output"
	if m.Output() != "test output" {
		t.Errorf("Output() = %q, want %q", m.Output(), "test output")
	}
}

func TestResize_NoProcess(t *testing.T) {
	m := NewManager()

	// Should not panic with no process
	m.Resize()
}

func TestCleanup_NoProcess(t *testing.T) {
	m := NewManager()

	// Should not panic with no process
	m.Cleanup()
}

func TestProcess(t *testing.T) {
	m := NewManager()

	// Initially nil
	if m.Process() != nil {
		t.Error("Process() should be nil initially")
	}
}

func TestUpdateOnInstanceChange_NotInWorktreeMode(t *testing.T) {
	m := NewManager()
	m.SetDirMode(DirInvocation)

	// Should return empty string and not attempt any changes
	errMsg := m.UpdateOnInstanceChange(nil)
	if errMsg != "" {
		t.Errorf("UpdateOnInstanceChange() = %q, want empty", errMsg)
	}
}

func TestUpdateOnInstanceChange_NoProcess(t *testing.T) {
	m := NewManager()
	m.SetDirMode(DirWorktree)

	// Should return empty string when no process
	errMsg := m.UpdateOnInstanceChange(nil)
	if errMsg != "" {
		t.Errorf("UpdateOnInstanceChange() = %q, want empty", errMsg)
	}
}

func TestDirModeConstants(t *testing.T) {
	// Verify DirInvocation is zero value (default)
	if DirInvocation != 0 {
		t.Errorf("DirInvocation = %d, want 0", DirInvocation)
	}
	if DirWorktree != 1 {
		t.Errorf("DirWorktree = %d, want 1", DirWorktree)
	}
}

func TestSendPaste_NoProcess(t *testing.T) {
	m := NewManager()

	err := m.SendPaste("test text")
	if err != ErrNotRunning {
		t.Errorf("SendPaste() error = %v, want ErrNotRunning", err)
	}
}

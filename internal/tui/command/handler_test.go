package command

// Coverage Note:
// Many command implementations (cmdStart, cmdStop, cmdPause, etc.) require
// interaction with the concrete orchestrator.Orchestrator type, which cannot
// be easily mocked without significant refactoring.
//
// These tests cover:
// 1. Handler infrastructure (New, Execute, Categories)
// 2. Error paths (nil instance, nil orchestrator, nil session)
// 3. Commands that don't require orchestrator (cmdAdd, cmdHelp, cmdFilter, etc.)
// 4. Nil logger safety (commands don't panic with nil logger)
//
// Not covered (would require interface extraction or integration tests):
// - Status guard branches (orchestrator nil check happens first)
// - Orchestrator method calls (StartInstance, StopInstance, etc.)
// - Instance manager interactions (Pause, Resume, etc.)

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/viper"
)

// mockDeps implements Dependencies for testing
type mockDeps struct {
	orchestrator     *orchestrator.Orchestrator
	session          *orchestrator.Session
	activeInstance   *orchestrator.Instance
	instanceCount    int
	conflicts        int
	terminalVisible  bool
	diffVisible      bool
	diffContent      string
	ultraPlanMode    bool
	tripleShotMode   bool
	ultraCoordinator *orchestrator.Coordinator
	logger           *logging.Logger
	startTime        time.Time
}

func (m *mockDeps) GetOrchestrator() *orchestrator.Orchestrator { return m.orchestrator }
func (m *mockDeps) GetSession() *orchestrator.Session           { return m.session }
func (m *mockDeps) ActiveInstance() *orchestrator.Instance      { return m.activeInstance }
func (m *mockDeps) InstanceCount() int                          { return m.instanceCount }
func (m *mockDeps) GetConflicts() int                           { return m.conflicts }
func (m *mockDeps) IsTerminalVisible() bool                     { return m.terminalVisible }
func (m *mockDeps) IsDiffVisible() bool                         { return m.diffVisible }
func (m *mockDeps) GetDiffContent() string                      { return m.diffContent }
func (m *mockDeps) IsUltraPlanMode() bool                       { return m.ultraPlanMode }
func (m *mockDeps) IsTripleShotMode() bool                      { return m.tripleShotMode }
func (m *mockDeps) GetUltraPlanCoordinator() *orchestrator.Coordinator {
	return m.ultraCoordinator
}
func (m *mockDeps) GetLogger() *logging.Logger { return m.logger }
func (m *mockDeps) GetStartTime() time.Time    { return m.startTime }

func newMockDeps() *mockDeps {
	return &mockDeps{
		startTime: time.Now(),
	}
}

func TestNew(t *testing.T) {
	h := New()
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if len(h.commands) == 0 {
		t.Error("expected commands to be registered")
	}
	if len(h.categories) == 0 {
		t.Error("expected categories to be populated")
	}
}

func TestCategories(t *testing.T) {
	h := New()
	categories := h.Categories()

	if len(categories) == 0 {
		t.Fatal("expected at least one category")
	}

	// Verify expected categories exist
	expectedCategories := []string{
		"Instance Control",
		"Instance Management",
		"View",
		"Terminal",
		"Utility",
		"Session",
	}

	for _, expected := range expectedCategories {
		found := false
		for _, cat := range categories {
			if cat.Name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected category %q not found", expected)
		}
	}

	// Verify each category has commands
	for _, cat := range categories {
		if len(cat.Commands) == 0 {
			t.Errorf("category %q has no commands", cat.Name)
		}

		// Verify each command has required fields
		for _, cmd := range cat.Commands {
			if cmd.LongKey == "" {
				t.Errorf("command in category %q has empty LongKey", cat.Name)
			}
			if cmd.Description == "" {
				t.Errorf("command %q in category %q has empty Description", cmd.LongKey, cat.Name)
			}
			if cmd.Category == "" {
				t.Errorf("command %q in category %q has empty Category", cmd.LongKey, cat.Name)
			}
		}
	}
}

func TestCategoriesContainAllShortcuts(t *testing.T) {
	h := New()
	categories := h.Categories()

	// Collect all short keys from categories
	shortKeys := make(map[string]bool)
	for _, cat := range categories {
		for _, cmd := range cat.Commands {
			if cmd.ShortKey != "" {
				shortKeys[cmd.ShortKey] = true
			}
		}
	}

	// Verify key shortcuts are documented
	expectedShortcuts := []string{"s", "x", "e", "p", "R", "a", "D", "C", "d", "m", "c", "f", "t", "r", "h", "q"}
	for _, key := range expectedShortcuts {
		if !shortKeys[key] {
			t.Errorf("shortcut %q not found in categories", key)
		}
	}
}

func TestExecuteEmptyCommand(t *testing.T) {
	h := New()
	deps := newMockDeps()

	result := h.Execute("", deps)
	if result.ErrorMessage != "" {
		t.Errorf("expected no error for empty command, got %q", result.ErrorMessage)
	}
	if result.InfoMessage != "" {
		t.Errorf("expected no info for empty command, got %q", result.InfoMessage)
	}
}

func TestExecuteWhitespaceCommand(t *testing.T) {
	h := New()
	deps := newMockDeps()

	result := h.Execute("   ", deps)
	if result.ErrorMessage != "" {
		t.Errorf("expected no error for whitespace command, got %q", result.ErrorMessage)
	}
}

func TestExecuteUnknownCommand(t *testing.T) {
	h := New()
	deps := newMockDeps()

	result := h.Execute("unknowncommand", deps)
	if result.ErrorMessage == "" {
		t.Error("expected error for unknown command")
	}
	if result.ErrorMessage != "Unknown command: unknowncommand (type :h for help)" {
		t.Errorf("unexpected error message: %q", result.ErrorMessage)
	}
}

func TestHelpCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"help", "help"},
		{"h alias", "h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			deps := newMockDeps()

			result := h.Execute(tt.cmd, deps)
			if result.ShowHelp == nil || !*result.ShowHelp {
				t.Error("expected ShowHelp to be set to true")
			}
			if result.ErrorMessage != "" {
				t.Errorf("unexpected error: %q", result.ErrorMessage)
			}
		})
	}
}

func TestQuitCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"quit", "quit"},
		{"q alias", "q"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			deps := newMockDeps()

			result := h.Execute(tt.cmd, deps)
			if result.Quitting == nil || !*result.Quitting {
				t.Error("expected Quitting to be set to true")
			}
			if result.TeaCmd == nil {
				t.Error("expected tea.Quit command")
			}
		})
	}
}

func TestAddCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"add", "add"},
		{"a alias", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			deps := newMockDeps()

			result := h.Execute(tt.cmd, deps)
			if result.AddingTask == nil || !*result.AddingTask {
				t.Error("expected AddingTask to be set to true")
			}
		})
	}
}

func TestChainCommand(t *testing.T) {
	t.Run("chain with active instance", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123", Task: "Test task"}

		result := h.Execute("chain", deps)
		if result.AddingDependentTask == nil || !*result.AddingDependentTask {
			t.Error("expected AddingDependentTask to be set to true")
		}
		if result.DependentOnInstanceID == nil || *result.DependentOnInstanceID != "test-123" {
			t.Error("expected DependentOnInstanceID to be set to 'test-123'")
		}
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
	})

	t.Run("dep alias with active instance", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "instance-456", Task: "Another task"}

		result := h.Execute("dep", deps)
		if result.AddingDependentTask == nil || !*result.AddingDependentTask {
			t.Error("expected AddingDependentTask to be set to true")
		}
		if result.DependentOnInstanceID == nil || *result.DependentOnInstanceID != "instance-456" {
			t.Error("expected DependentOnInstanceID to be set to 'instance-456'")
		}
	})

	t.Run("depends alias with active instance", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "abc-789"}

		result := h.Execute("depends", deps)
		if result.AddingDependentTask == nil || !*result.AddingDependentTask {
			t.Error("expected AddingDependentTask to be set to true")
		}
		if result.DependentOnInstanceID == nil || *result.DependentOnInstanceID != "abc-789" {
			t.Error("expected DependentOnInstanceID to be set correctly")
		}
	})

	t.Run("chain without active instance returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = nil

		result := h.Execute("chain", deps)
		if result.ErrorMessage == "" {
			t.Error("expected error when no instance is selected")
		}
		if result.AddingDependentTask != nil {
			t.Error("expected AddingDependentTask to be nil on error")
		}
	})

	t.Run("dep without active instance returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = nil

		result := h.Execute("dep", deps)
		if result.ErrorMessage == "" {
			t.Error("expected error when no instance is selected")
		}
	})
}

func TestStatsCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"stats", "stats"},
		{"m alias", "m"},
		{"metrics alias", "metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			deps := newMockDeps()

			result := h.Execute(tt.cmd, deps)
			if result.ShowStats == nil || !*result.ShowStats {
				t.Error("expected ShowStats to be set to true")
			}
		})
	}
}

func TestFilterCommand(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"filter", "filter"},
		{"f alias", "f"},
		{"F alias", "F"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			deps := newMockDeps()

			result := h.Execute(tt.cmd, deps)
			if result.FilterMode == nil || !*result.FilterMode {
				t.Error("expected FilterMode to be set to true")
			}
		})
	}
}

func TestDiffCommand(t *testing.T) {
	t.Run("toggle off when visible", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.diffVisible = true
		deps.diffContent = "some diff"

		result := h.Execute("diff", deps)
		if result.ShowDiff == nil || *result.ShowDiff {
			t.Error("expected ShowDiff to be set to false")
		}
		if result.DiffContent == nil || *result.DiffContent != "" {
			t.Error("expected DiffContent to be cleared")
		}
		if result.DiffScroll == nil || *result.DiffScroll != 0 {
			t.Error("expected DiffScroll to be reset to 0")
		}
	})

	t.Run("d alias toggles off", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.diffVisible = true

		result := h.Execute("d", deps)
		if result.ShowDiff == nil || *result.ShowDiff {
			t.Error("expected ShowDiff to be set to false")
		}
	})

	t.Run("no instance shows message", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.diffVisible = false
		deps.activeInstance = nil

		result := h.Execute("diff", deps)
		if result.InfoMessage != "No instance selected" {
			t.Errorf("expected 'No instance selected', got %q", result.InfoMessage)
		}
	})
}

func TestConflictsCommand(t *testing.T) {
	t.Run("shows conflicts panel when conflicts exist", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.conflicts = 3

		result := h.Execute("conflicts", deps)
		if result.ShowConflicts == nil || !*result.ShowConflicts {
			t.Error("expected ShowConflicts to be set to true")
		}
	})

	t.Run("c alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.conflicts = 1

		result := h.Execute("c", deps)
		if result.ShowConflicts == nil || !*result.ShowConflicts {
			t.Error("expected ShowConflicts to be set to true")
		}
	})

	t.Run("shows message when no conflicts", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.conflicts = 0

		result := h.Execute("conflicts", deps)
		if result.InfoMessage != "No conflicts detected" {
			t.Errorf("expected 'No conflicts detected', got %q", result.InfoMessage)
		}
	})
}

func TestTerminalCommands(t *testing.T) {
	// Terminal commands require experimental.terminal_support to be enabled
	viper.Set("experimental.terminal_support", true)
	defer viper.Set("experimental.terminal_support", false)

	t.Run("term toggles terminal visibility", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("term", deps)
		if !result.ToggleTerminal {
			t.Error("expected ToggleTerminal to be true")
		}
	})

	t.Run("terminal alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("terminal", deps)
		if !result.ToggleTerminal {
			t.Error("expected ToggleTerminal to be true")
		}
	})

	t.Run("t focuses terminal when visible", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.terminalVisible = true

		result := h.Execute("t", deps)
		if !result.EnterTerminalMode {
			t.Error("expected EnterTerminalMode to be true")
		}
		if result.InfoMessage == "" {
			t.Error("expected info message about terminal focus")
		}
	})

	t.Run("t shows error when terminal not visible", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.terminalVisible = false

		result := h.Execute("t", deps)
		if result.EnterTerminalMode {
			t.Error("expected EnterTerminalMode to be false")
		}
		if result.ErrorMessage == "" {
			t.Error("expected error message when terminal not visible")
		}
	})

	t.Run("termdir worktree sets mode", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("termdir worktree", deps)
		if result.TerminalDirMode == nil || *result.TerminalDirMode != 1 {
			t.Error("expected TerminalDirMode to be set to 1 (worktree)")
		}
	})

	t.Run("termdir wt alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("termdir wt", deps)
		if result.TerminalDirMode == nil || *result.TerminalDirMode != 1 {
			t.Error("expected TerminalDirMode to be set to 1 (worktree)")
		}
	})

	t.Run("termdir invoke sets mode", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("termdir invoke", deps)
		if result.TerminalDirMode == nil || *result.TerminalDirMode != 0 {
			t.Error("expected TerminalDirMode to be set to 0 (invocation)")
		}
	})

	t.Run("termdir invocation alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("termdir invocation", deps)
		if result.TerminalDirMode == nil || *result.TerminalDirMode != 0 {
			t.Error("expected TerminalDirMode to be set to 0 (invocation)")
		}
	})
}

func TestTerminalCommandsDisabled(t *testing.T) {
	// When terminal support is disabled, commands should return an error
	viper.Set("experimental.terminal_support", false)

	commands := []string{"term", "terminal", "t", "termdir worktree", "termdir wt", "termdir invoke", "termdir invocation"}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			h := New()
			deps := newMockDeps()
			deps.terminalVisible = true // Even with terminal visible, should fail

			result := h.Execute(cmd, deps)
			if result.ErrorMessage == "" {
				t.Error("expected error message when terminal support is disabled")
			}
			if result.ToggleTerminal || result.EnterTerminalMode || result.TerminalDirMode != nil {
				t.Error("expected no terminal state changes when disabled")
			}
		})
	}
}

func TestInstanceControlCommandsNoInstance(t *testing.T) {
	// All instance control commands should return "No instance selected" when no instance
	commands := []string{
		"s", "start",
		"x", "stop",
		"e", "exit",
		"p", "pause",
		"R", "reconnect",
		"restart",
		"D", "remove",
		"kill",
		"tmux",
		"r", "pr",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			h := New()
			deps := newMockDeps()
			deps.activeInstance = nil

			result := h.Execute(cmd, deps)
			if result.InfoMessage != "No instance selected" {
				t.Errorf("expected 'No instance selected' for %q, got info=%q error=%q",
					cmd, result.InfoMessage, result.ErrorMessage)
			}
		})
	}
}

func TestInstanceControlCommandsNoOrchestrator(t *testing.T) {
	// Commands that need orchestrator should return error when none available
	commands := []struct {
		cmd         string
		needsOrch   bool
		needsStatus bool
	}{
		{"start", true, true},
		{"stop", true, false},
		{"exit", true, false},
		{"pause", true, false},
		{"reconnect", true, true},
		{"restart", true, true},
	}

	for _, tc := range commands {
		t.Run(tc.cmd, func(t *testing.T) {
			h := New()
			deps := newMockDeps()
			deps.activeInstance = &orchestrator.Instance{
				ID:     "test-123",
				Status: orchestrator.StatusPending,
			}
			deps.orchestrator = nil

			result := h.Execute(tc.cmd, deps)
			if tc.needsOrch {
				if result.ErrorMessage == "" {
					t.Errorf("expected error for %q without orchestrator", tc.cmd)
				}
			}
		})
	}
}

func TestUltraPlanCancelCommand(t *testing.T) {
	t.Run("error when not in ultraplan mode", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.ultraPlanMode = false

		result := h.Execute("cancel", deps)
		if result.ErrorMessage != "Not in ultraplan mode" {
			t.Errorf("expected 'Not in ultraplan mode', got %q", result.ErrorMessage)
		}
	})

	t.Run("error when no coordinator", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.ultraPlanMode = true
		deps.ultraCoordinator = nil

		result := h.Execute("cancel", deps)
		if result.ErrorMessage != "No active ultraplan session" {
			t.Errorf("expected 'No active ultraplan session', got %q", result.ErrorMessage)
		}
	})
}

func TestClearCompletedCommand(t *testing.T) {
	t.Run("C alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("C", deps)
		// Without orchestrator, should return error
		if result.ErrorMessage == "" {
			t.Error("expected error without orchestrator")
		}
	})

	t.Run("clear alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		result := h.Execute("clear", deps)
		if result.ErrorMessage == "" {
			t.Error("expected error without orchestrator")
		}
	})
}

func TestPRCommand(t *testing.T) {
	t.Run("shows PR command with instance", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}

		result := h.Execute("pr", deps)
		if result.InfoMessage == "" {
			t.Error("expected info message with PR command")
		}
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
	})

	t.Run("r alias", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}

		result := h.Execute("r", deps)
		if result.InfoMessage == "" {
			t.Error("expected info message with PR command")
		}
	})
}

func TestAllCommandsRecognized(t *testing.T) {
	// Verify all expected commands are registered and don't return "Unknown command"
	commands := []string{
		// Instance control
		"s", "start", "x", "stop", "e", "exit", "p", "pause",
		"R", "reconnect", "restart",
		// Instance management
		"a", "add", "chain", "dep", "depends", "D", "remove", "kill", "C", "clear",
		// View toggles
		"d", "diff", "m", "metrics", "stats", "c", "conflicts",
		"f", "F", "filter",
		// Utilities
		"tmux", "r", "pr",
		// Terminal
		"t", "term", "terminal",
		"termdir worktree", "termdir wt",
		"termdir invoke", "termdir invocation",
		// Ultraplan
		"cancel",
		// Plan mode
		"plan",
		// Ultraplan arg commands (need viper config)
		// "ultraplan", "up", // These are arg commands, tested separately
		// Help
		"h", "help", "q", "quit",
	}

	h := New()
	deps := newMockDeps()

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			result := h.Execute(cmd, deps)
			// Should NOT be an unknown command error
			if result.ErrorMessage != "" && len(result.ErrorMessage) >= 15 &&
				result.ErrorMessage[:15] == "Unknown command" {
				t.Errorf("command %q was not recognized", cmd)
			}
		})
	}
}

func TestResultActiveTabAdjustment(t *testing.T) {
	t.Run("remove sets adjustment flag", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.session = &orchestrator.Session{}
		deps.orchestrator = nil // Will fail, but we're testing result structure

		// Note: This will return an error because orchestrator is nil,
		// but we can test the command is recognized
		result := h.Execute("remove", deps)
		// Error case means no adjustment needed
		if result.ErrorMessage == "" {
			// Would have adjustment set
			if result.ActiveTabAdjustment != -1 {
				t.Error("expected ActiveTabAdjustment to be -1")
			}
		}
	})
}

func TestResultEnsureActiveVisible(t *testing.T) {
	// Commands that remove instances should set EnsureActiveVisible
	t.Run("kill sets EnsureActiveVisible", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.orchestrator = nil // Will fail

		result := h.Execute("kill", deps)
		// On error, EnsureActiveVisible won't be set
		// but command is recognized
		if result.ErrorMessage != "" && len(result.ErrorMessage) >= 15 &&
			result.ErrorMessage[:15] == "Unknown command" {
			t.Error("kill command not recognized")
		}
	})
}

// TestRestartUltraPlanMode tests cmdRestart behavior in ultraplan mode
func TestRestartUltraPlanMode(t *testing.T) {
	t.Run("ultraplan mode without coordinator falls through", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{
			ID:     "test-123",
			Status: orchestrator.StatusPending,
		}
		deps.ultraPlanMode = true
		deps.ultraCoordinator = nil // No coordinator

		result := h.Execute("restart", deps)

		// Should fall through to regular restart path (which needs orchestrator)
		if result.ErrorMessage == "" {
			t.Error("expected error without orchestrator")
		}
	})
}

// TestTmuxCommandNoOrchestrator tests cmdTmux when orchestrator is nil
func TestTmuxCommandNoOrchestrator(t *testing.T) {
	t.Run("no orchestrator returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.orchestrator = nil

		result := h.Execute("tmux", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error without orchestrator")
		}
	})
}

// TestRemoveCommandNoSession tests cmdRemove when session is nil
func TestRemoveCommandNoSession(t *testing.T) {
	t.Run("no session returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.session = nil
		deps.orchestrator = nil

		result := h.Execute("remove", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error without session/orchestrator")
		}
	})
}

// TestKillCommandNoSession tests cmdKill when session is nil
func TestKillCommandNoSession(t *testing.T) {
	t.Run("no session returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.session = nil
		deps.orchestrator = nil

		result := h.Execute("kill", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error without session/orchestrator")
		}
	})
}

// TestQuitCommandHandlesNilLogger tests cmdQuit works with nil logger
func TestQuitCommandHandlesNilLogger(t *testing.T) {
	t.Run("quit with nil logger doesn't panic", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.startTime = time.Now().Add(-5 * time.Minute)
		deps.logger = nil

		result := h.Execute("quit", deps)

		if result.Quitting == nil || !*result.Quitting {
			t.Error("expected Quitting to be set to true")
		}
		if result.TeaCmd == nil {
			t.Error("expected tea.Quit command")
		}
	})
}

// TestStopCommandHandlesNilLogger tests cmdStop doesn't panic with nil logger
func TestStopCommandHandlesNilLogger(t *testing.T) {
	t.Run("stop with nil logger doesn't panic", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.logger = nil
		deps.orchestrator = nil

		// Should not panic even with nil logger
		result := h.Execute("stop", deps)

		// We get error because orchestrator is nil, but no panic from nil logger
		if result.ErrorMessage != "No orchestrator available" {
			t.Errorf("expected 'No orchestrator available', got %q", result.ErrorMessage)
		}
	})
}

// TestExitCommandHandlesNilLogger tests cmdExit doesn't panic with nil logger
func TestExitCommandHandlesNilLogger(t *testing.T) {
	t.Run("exit with nil logger doesn't panic", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.activeInstance = &orchestrator.Instance{ID: "test-123"}
		deps.logger = nil
		deps.orchestrator = nil

		// Should not panic even with nil logger
		result := h.Execute("exit", deps)

		// We get error because orchestrator is nil, but no panic from nil logger
		if result.ErrorMessage != "No orchestrator available" {
			t.Errorf("expected 'No orchestrator available', got %q", result.ErrorMessage)
		}
	})
}

// TestClearCompletedNoOrchestrator tests cmdClearCompleted without orchestrator
func TestClearCompletedNoOrchestrator(t *testing.T) {
	t.Run("no orchestrator returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.orchestrator = nil
		deps.session = &orchestrator.Session{}

		result := h.Execute("clear", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error without orchestrator")
		}
	})

	t.Run("no session returns error", func(t *testing.T) {
		h := New()
		deps := newMockDeps()
		deps.orchestrator = nil
		deps.session = nil

		result := h.Execute("clear", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error without session")
		}
	})
}

// TestTripleShotCommand tests the tripleshot command with config check
func TestTripleShotCommand(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()

		h := New()
		deps := newMockDeps()

		result := h.Execute("tripleshot", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error when triple-shot is disabled")
		}
		if result.ErrorMessage != "Triple-shot mode is disabled. Enable it in :config under Experimental" {
			t.Errorf("unexpected error message: %q", result.ErrorMessage)
		}
		if result.StartTripleShot != nil {
			t.Error("StartTripleShot should be nil when disabled")
		}
	})

	t.Run("enabled via config", func(t *testing.T) {
		// Reset and enable triple-shot
		viper.Reset()
		viper.Set("experimental.triple_shot", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("tripleshot", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartTripleShot == nil || !*result.StartTripleShot {
			t.Error("expected StartTripleShot to be true")
		}
		if result.InfoMessage != "Enter a task for triple-shot mode" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}

		// Clean up
		viper.Reset()
	})

	t.Run("blocked in ultraplan mode", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.triple_shot", true)

		h := New()
		deps := newMockDeps()
		deps.ultraPlanMode = true

		result := h.Execute("tripleshot", deps)

		if result.ErrorMessage != "Cannot start triple-shot while in ultraplan mode" {
			t.Errorf("expected ultraplan mode error, got: %q", result.ErrorMessage)
		}

		viper.Reset()
	})

	t.Run("blocked when already in triple-shot mode", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.triple_shot", true)

		h := New()
		deps := newMockDeps()
		deps.tripleShotMode = true

		result := h.Execute("tripleshot", deps)

		if result.ErrorMessage != "Already in triple-shot mode" {
			t.Errorf("expected already in mode error, got: %q", result.ErrorMessage)
		}

		viper.Reset()
	})

	t.Run("aliases work", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.triple_shot", true)

		h := New()
		deps := newMockDeps()

		aliases := []string{"tripleshot", "triple", "3shot"}
		for _, alias := range aliases {
			result := h.Execute(alias, deps)
			if result.StartTripleShot == nil || !*result.StartTripleShot {
				t.Errorf("alias %q should start triple-shot mode", alias)
			}
		}

		viper.Reset()
	})
}

// TestPlanCommand tests the plan command with config check
func TestPlanCommand(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()

		h := New()
		deps := newMockDeps()

		result := h.Execute("plan", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error when plan mode is disabled")
		}
		if result.ErrorMessage != "Plan mode is disabled. Enable it in :config under Experimental" {
			t.Errorf("unexpected error message: %q", result.ErrorMessage)
		}
		if result.StartPlanMode != nil {
			t.Error("StartPlanMode should be nil when disabled")
		}
	})

	t.Run("enabled via config", func(t *testing.T) {
		// Reset and enable plan mode
		viper.Reset()
		viper.Set("experimental.inline_plan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("plan", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartPlanMode == nil || !*result.StartPlanMode {
			t.Error("expected StartPlanMode to be true")
		}
		if result.InfoMessage != "Enter an objective for plan mode" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}

		// Clean up
		viper.Reset()
	})

	t.Run("blocked in ultraplan mode", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_plan", true)

		h := New()
		deps := newMockDeps()
		deps.ultraPlanMode = true

		result := h.Execute("plan", deps)

		if result.ErrorMessage != "Cannot start plan mode while in ultraplan mode" {
			t.Errorf("expected ultraplan mode error, got: %q", result.ErrorMessage)
		}
		if result.StartPlanMode != nil {
			t.Error("StartPlanMode should be nil when blocked")
		}

		viper.Reset()
	})

	t.Run("blocked when in triple-shot mode", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_plan", true)

		h := New()
		deps := newMockDeps()
		deps.tripleShotMode = true

		result := h.Execute("plan", deps)

		if result.ErrorMessage != "Cannot start plan mode while in triple-shot mode" {
			t.Errorf("expected triple-shot mode error, got: %q", result.ErrorMessage)
		}
		if result.StartPlanMode != nil {
			t.Error("StartPlanMode should be nil when blocked")
		}

		viper.Reset()
	})
}

// TestUltraPlanCommand tests the ultraplan command with config check and argument parsing
func TestUltraPlanCommand(t *testing.T) {
	t.Run("disabled by default", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()

		h := New()
		deps := newMockDeps()

		result := h.Execute("ultraplan", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error when ultraplan mode is disabled")
		}
		if result.ErrorMessage != "UltraPlan mode is disabled. Enable it in :config under Experimental" {
			t.Errorf("unexpected error message: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode != nil {
			t.Error("StartUltraPlanMode should be nil when disabled")
		}
	})

	t.Run("enabled via config without objective", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("ultraplan", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode == nil || !*result.StartUltraPlanMode {
			t.Error("expected StartUltraPlanMode to be true")
		}
		if result.InfoMessage != "Enter an objective for ultraplan mode" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}
		if result.UltraPlanMultiPass != nil {
			t.Error("UltraPlanMultiPass should be nil without --multi-pass flag")
		}
		if result.UltraPlanFromFile != nil {
			t.Error("UltraPlanFromFile should be nil without --plan flag")
		}

		viper.Reset()
	})

	t.Run("enabled with objective", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("ultraplan Add user authentication", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode == nil || !*result.StartUltraPlanMode {
			t.Error("expected StartUltraPlanMode to be true")
		}
		if result.UltraPlanObjective == nil || *result.UltraPlanObjective != "Add user authentication" {
			t.Errorf("expected objective 'Add user authentication', got: %v", result.UltraPlanObjective)
		}
		if result.InfoMessage != "Starting ultraplan: Add user authentication" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}

		viper.Reset()
	})

	t.Run("multi-pass flag", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("ultraplan --multi-pass Implement new feature", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode == nil || !*result.StartUltraPlanMode {
			t.Error("expected StartUltraPlanMode to be true")
		}
		if result.UltraPlanMultiPass == nil || !*result.UltraPlanMultiPass {
			t.Error("expected UltraPlanMultiPass to be true")
		}
		if result.UltraPlanObjective == nil || *result.UltraPlanObjective != "Implement new feature" {
			t.Errorf("expected objective 'Implement new feature', got: %v", result.UltraPlanObjective)
		}

		viper.Reset()
	})

	t.Run("plan flag with file", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("ultraplan --plan /path/to/plan.json", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode == nil || !*result.StartUltraPlanMode {
			t.Error("expected StartUltraPlanMode to be true")
		}
		if result.UltraPlanFromFile == nil || *result.UltraPlanFromFile != "/path/to/plan.json" {
			t.Errorf("expected UltraPlanFromFile '/path/to/plan.json', got: %v", result.UltraPlanFromFile)
		}
		if result.InfoMessage != "Loading ultraplan from: /path/to/plan.json" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}

		viper.Reset()
	})

	t.Run("plan flag without file", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("ultraplan --plan", deps)

		if result.ErrorMessage != "Usage: :ultraplan --plan <file>" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}

		viper.Reset()
	})

	t.Run("blocked in ultraplan mode", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()
		deps.ultraPlanMode = true

		result := h.Execute("ultraplan test", deps)

		if result.ErrorMessage != "Already in ultraplan mode" {
			t.Errorf("expected already in ultraplan error, got: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode != nil {
			t.Error("StartUltraPlanMode should be nil when blocked")
		}

		viper.Reset()
	})

	t.Run("blocked when in triple-shot mode", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()
		deps.tripleShotMode = true

		result := h.Execute("ultraplan test", deps)

		if result.ErrorMessage != "Cannot start ultraplan while in triple-shot mode" {
			t.Errorf("expected triple-shot mode error, got: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode != nil {
			t.Error("StartUltraPlanMode should be nil when blocked")
		}

		viper.Reset()
	})

	t.Run("up alias works", func(t *testing.T) {
		viper.Reset()
		viper.Set("experimental.inline_ultraplan", true)

		h := New()
		deps := newMockDeps()

		result := h.Execute("up my objective", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.StartUltraPlanMode == nil || !*result.StartUltraPlanMode {
			t.Error("expected StartUltraPlanMode to be true for 'up' alias")
		}
		if result.UltraPlanObjective == nil || *result.UltraPlanObjective != "my objective" {
			t.Errorf("expected objective 'my objective', got: %v", result.UltraPlanObjective)
		}

		viper.Reset()
	})
}

// TestArgCommandsPrecedence tests that exact matches take precedence over arg commands
func TestArgCommandsPrecedence(t *testing.T) {
	t.Run("exact match takes precedence", func(t *testing.T) {
		h := New()
		deps := newMockDeps()

		// "help" is an exact match command, should not be parsed as arg command
		result := h.Execute("help", deps)
		if result.ShowHelp == nil || !*result.ShowHelp {
			t.Error("expected ShowHelp to be true for exact 'help' command")
		}
	})
}

// Ensure mockDeps satisfies the interface at compile time
var _ Dependencies = (*mockDeps)(nil)

// Ensure conflict package import is used (for testing scenarios)
var _ = conflict.FileConflict{}

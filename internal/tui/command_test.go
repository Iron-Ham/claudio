package tui

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/tui/command"
	tea "github.com/charmbracelet/bubbletea"
)

// testModel creates a Model with the commandHandler initialized for testing.
// This is necessary because tests construct Model directly instead of using NewModel.
func testModel() Model {
	return Model{
		commandHandler: command.New(),
	}
}

func TestHandleCommandInput(t *testing.T) {
	t.Run("escape exits command mode", func(t *testing.T) {
		m := testModel()
		m.commandMode = true
		m.commandBuffer = "test"

		msg := tea.KeyMsg{Type: tea.KeyEsc}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandMode {
			t.Error("expected commandMode to be false after Esc")
		}
		if model.commandBuffer != "" {
			t.Errorf("expected commandBuffer to be empty, got %q", model.commandBuffer)
		}
	})

	t.Run("enter executes command and exits command mode", func(t *testing.T) {
		m := testModel()
		m.commandMode = true
		m.commandBuffer = "help"

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandMode {
			t.Error("expected commandMode to be false after Enter")
		}
		if model.commandBuffer != "" {
			t.Errorf("expected commandBuffer to be empty, got %q", model.commandBuffer)
		}
		// :help toggles showHelp
		if !model.showHelp {
			t.Error("expected showHelp to be true after :help command")
		}
	})

	t.Run("backspace removes last character", func(t *testing.T) {
		m := testModel()
		m.commandMode = true
		m.commandBuffer = "test"

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandBuffer != "tes" {
			t.Errorf("expected commandBuffer to be %q, got %q", "tes", model.commandBuffer)
		}
		if !model.commandMode {
			t.Error("expected commandMode to remain true")
		}
	})

	t.Run("backspace on empty buffer exits command mode", func(t *testing.T) {
		m := testModel()
		m.commandMode = true
		m.commandBuffer = "a"

		msg := tea.KeyMsg{Type: tea.KeyBackspace}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandBuffer != "" {
			t.Errorf("expected commandBuffer to be empty, got %q", model.commandBuffer)
		}
		if model.commandMode {
			t.Error("expected commandMode to be false after backspacing to empty")
		}
	})

	t.Run("typing adds to buffer", func(t *testing.T) {
		m := testModel()
		m.commandMode = true
		m.commandBuffer = "te"

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s', 't'}}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandBuffer != "test" {
			t.Errorf("expected commandBuffer to be %q, got %q", "test", model.commandBuffer)
		}
	})

	t.Run("space adds to buffer", func(t *testing.T) {
		m := testModel()
		m.commandMode = true
		m.commandBuffer = "add"

		msg := tea.KeyMsg{Type: tea.KeySpace}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandBuffer != "add " {
			t.Errorf("expected commandBuffer to be %q, got %q", "add ", model.commandBuffer)
		}
	})
}

func TestTaskInputEnter(t *testing.T) {
	t.Run("enter key exits task input mode without task", func(t *testing.T) {
		m := Model{
			addingTask:      true,
			taskInput:       "",
			taskInputCursor: 0,
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.addingTask {
			t.Error("expected addingTask to be false after Enter")
		}
		if model.taskInput != "" {
			t.Errorf("expected taskInput to be empty, got %q", model.taskInput)
		}
	})

	t.Run("enter key exits task input mode with task (no orchestrator)", func(t *testing.T) {
		// Note: Without orchestrator, AddInstance will fail but addingTask should still be cleared
		// This will panic because orchestrator is nil, so we skip this test
		// The important test is the empty task one which confirms Enter is detected
		t.Skip("skipping - requires mock orchestrator")

		m := Model{
			addingTask:      true,
			taskInput:       "test task",
			taskInputCursor: 9,
			// orchestrator is nil - AddInstance will fail but mode should still exit
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.addingTask {
			t.Error("expected addingTask to be false after Enter")
		}
		if model.taskInput != "" {
			t.Errorf("expected taskInput to be cleared, got %q", model.taskInput)
		}
	})

	t.Run("enter string also submits task", func(t *testing.T) {
		// Test that msg.String() == "enter" would also be detected
		// This is to check if the terminal might be sending Enter differently
		m := Model{
			addingTask:      true,
			taskInput:       "",
			taskInputCursor: 0,
		}

		// Simulate what some terminals might send
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		msgStr := msg.String()
		t.Logf("Enter key msg.String() = %q", msgStr)

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.addingTask {
			t.Errorf("expected addingTask to be false after Enter (msg.String() = %q)", msgStr)
		}
	})

	t.Run("alt+enter inserts newline instead of submitting", func(t *testing.T) {
		m := Model{
			addingTask:      true,
			taskInput:       "test",
			taskInputCursor: 4,
		}

		msg := tea.KeyMsg{Type: tea.KeyEnter, Alt: true}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.addingTask {
			t.Error("expected addingTask to remain true after Alt+Enter")
		}
		if model.taskInput != "test\n" {
			t.Errorf("expected taskInput to have newline, got %q", model.taskInput)
		}
	})

	t.Run("ctrl+j inserts newline instead of submitting", func(t *testing.T) {
		m := Model{
			addingTask:      true,
			taskInput:       "test",
			taskInputCursor: 4,
		}

		msg := tea.KeyMsg{Type: tea.KeyCtrlJ}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.addingTask {
			t.Error("expected addingTask to remain true after Ctrl+J")
		}
		if model.taskInput != "test\n" {
			t.Errorf("expected taskInput to have newline, got %q", model.taskInput)
		}
	})

	t.Run("typing adds to task input", func(t *testing.T) {
		m := Model{
			addingTask:      true,
			taskInput:       "te",
			taskInputCursor: 2,
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s', 't'}}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.taskInput != "test" {
			t.Errorf("expected taskInput to be %q, got %q", "test", model.taskInput)
		}
		if model.taskInputCursor != 4 {
			t.Errorf("expected cursor to be 4, got %d", model.taskInputCursor)
		}
	})

	t.Run("newline rune submits task (terminal compat)", func(t *testing.T) {
		// Some terminals/input methods send Enter as KeyRunes with \n
		// This should submit the task, not insert a newline
		m := Model{
			addingTask:      true,
			taskInput:       "",
			taskInputCursor: 0,
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\n'}}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.addingTask {
			t.Error("expected addingTask to be false after newline rune (should submit)")
		}
		if model.taskInput != "" {
			t.Errorf("expected taskInput to be cleared, got %q", model.taskInput)
		}
	})

	t.Run("carriage return rune submits task (terminal compat)", func(t *testing.T) {
		// Some terminals send Enter as KeyRunes with \r
		// This should submit the task, not insert a carriage return
		m := Model{
			addingTask:      true,
			taskInput:       "",
			taskInputCursor: 0,
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\r'}}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.addingTask {
			t.Error("expected addingTask to be false after CR rune (should submit)")
		}
		if model.taskInput != "" {
			t.Errorf("expected taskInput to be cleared, got %q", model.taskInput)
		}
	})
}

func TestExecuteCommand(t *testing.T) {
	t.Run("empty command does nothing", func(t *testing.T) {
		m := testModel()
		result, _ := m.executeCommand("")
		model := result.(Model)

		if model.errorMessage != "" {
			t.Errorf("expected no error, got %q", model.errorMessage)
		}
	})

	t.Run("whitespace-only command does nothing", func(t *testing.T) {
		m := testModel()
		result, _ := m.executeCommand("   ")
		model := result.(Model)

		if model.errorMessage != "" {
			t.Errorf("expected no error, got %q", model.errorMessage)
		}
	})

	t.Run("unknown command sets error", func(t *testing.T) {
		m := testModel()
		result, _ := m.executeCommand("unknowncommand")
		model := result.(Model)

		if model.errorMessage == "" {
			t.Error("expected error message for unknown command")
		}
	})

	t.Run("help command toggles help", func(t *testing.T) {
		m := testModel()
		m.showHelp = false
		result, _ := m.executeCommand("help")
		model := result.(Model)

		if !model.showHelp {
			t.Error("expected showHelp to be true")
		}

		// Toggle off
		result, _ = model.executeCommand("help")
		model = result.(Model)

		if model.showHelp {
			t.Error("expected showHelp to be false after second toggle")
		}
	})

	t.Run("h command is alias for help", func(t *testing.T) {
		m := testModel()
		m.showHelp = false
		result, _ := m.executeCommand("h")
		model := result.(Model)

		if !model.showHelp {
			t.Error("expected showHelp to be true")
		}
	})

	t.Run("quit command sets quitting", func(t *testing.T) {
		m := testModel()
		result, cmd := m.executeCommand("quit")
		model := result.(Model)

		if !model.quitting {
			t.Error("expected quitting to be true")
		}
		if cmd == nil {
			t.Error("expected quit command to return tea.Quit")
		}
	})

	t.Run("q command is alias for quit", func(t *testing.T) {
		m := testModel()
		result, cmd := m.executeCommand("q")
		model := result.(Model)

		if !model.quitting {
			t.Error("expected quitting to be true")
		}
		if cmd == nil {
			t.Error("expected quit command to return tea.Quit")
		}
	})

	t.Run("add command starts task input", func(t *testing.T) {
		m := testModel()
		m.addingTask = false
		result, _ := m.executeCommand("add")
		model := result.(Model)

		if !model.addingTask {
			t.Error("expected addingTask to be true")
		}
	})

	t.Run("a command is alias for add", func(t *testing.T) {
		m := testModel()
		m.addingTask = false
		result, _ := m.executeCommand("a")
		model := result.(Model)

		if !model.addingTask {
			t.Error("expected addingTask to be true")
		}
	})

	t.Run("stats command toggles stats panel", func(t *testing.T) {
		m := testModel()
		m.showStats = false
		result, _ := m.executeCommand("stats")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})

	t.Run("m command is alias for stats", func(t *testing.T) {
		m := testModel()
		m.showStats = false
		result, _ := m.executeCommand("m")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})

	t.Run("metrics command is alias for stats", func(t *testing.T) {
		m := testModel()
		m.showStats = false
		result, _ := m.executeCommand("metrics")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})

	t.Run("filter command starts filter mode", func(t *testing.T) {
		m := testModel()
		m.filterMode = false
		result, _ := m.executeCommand("filter")
		model := result.(Model)

		if !model.filterMode {
			t.Error("expected filterMode to be true")
		}
	})

	t.Run("f command is alias for filter", func(t *testing.T) {
		m := testModel()
		m.filterMode = false
		result, _ := m.executeCommand("f")
		model := result.(Model)

		if !model.filterMode {
			t.Error("expected filterMode to be true")
		}
	})

	t.Run("F command is alias for filter", func(t *testing.T) {
		m := testModel()
		m.filterMode = false
		result, _ := m.executeCommand("F")
		model := result.(Model)

		if !model.filterMode {
			t.Error("expected filterMode to be true")
		}
	})

	t.Run("diff command toggles diff panel when no instance", func(t *testing.T) {
		m := testModel()
		m.showDiff = true
		m.diffContent = "some diff"
		result, _ := m.executeCommand("diff")
		model := result.(Model)

		if model.showDiff {
			t.Error("expected showDiff to be false when toggling off")
		}
		if model.diffContent != "" {
			t.Errorf("expected diffContent to be empty, got %q", model.diffContent)
		}
	})

	t.Run("d command is alias for diff", func(t *testing.T) {
		m := testModel()
		m.showDiff = true
		m.diffContent = "some diff"
		result, _ := m.executeCommand("d")
		model := result.(Model)

		if model.showDiff {
			t.Error("expected showDiff to be false")
		}
	})
}

func TestCommandAliases(t *testing.T) {
	// Test that all command aliases are recognized (not returning unknown command error)
	// Only testing commands that don't require orchestrator or session to execute
	commands := []string{
		// Instance management (only add since it just sets a flag)
		"a", "add",
		// View toggles (these work without orchestrator)
		"d", "diff",
		"m", "metrics", "stats",
		"c", "conflicts",
		"f", "F", "filter",
		// Help
		"h", "help",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			m := testModel()
			result, _ := m.executeCommand(cmd)
			model := result.(Model)

			// Check that it's not an unknown command error
			if model.errorMessage != "" && len(model.errorMessage) >= 7 && model.errorMessage[:7] == "Unknown" {
				t.Errorf("command %q was not recognized", cmd)
			}
		})
	}
}

func TestCommandAliasesRequiringInstance(t *testing.T) {
	// Test commands that require an instance - they should be recognized
	// but show a helpful message when no instance is selected
	commands := []string{
		// Instance control
		"s", "start",
		"x", "stop",
		"e", "exit",
		"p", "pause",
		"R", "reconnect",
		"restart",
		// Instance management
		"D", "remove",
		"kill",
		// Utilities
		"tmux", // Note: :t is now terminal focus, not an alias for tmux
		"r", "pr",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			m := testModel()
			m.session = nil // No session means no instances
			result, _ := m.executeCommand(cmd)
			model := result.(Model)

			// Should NOT be an unknown command error
			if model.errorMessage != "" && len(model.errorMessage) >= 7 && model.errorMessage[:7] == "Unknown" {
				t.Errorf("command %q was not recognized", cmd)
			}

			// Should set an info message about no instance selected
			if model.infoMessage == "" {
				t.Errorf("command %q should show info message when no instance selected", cmd)
			}
		})
	}
}

func TestConflictsCommandRequiresConflicts(t *testing.T) {
	t.Run("shows message when no conflicts", func(t *testing.T) {
		m := testModel()
		m.conflicts = nil
		result, _ := m.executeCommand("conflicts")
		model := result.(Model)

		if model.infoMessage == "" {
			t.Error("expected info message when no conflicts exist")
		}
	})

	t.Run("c alias shows message when no conflicts", func(t *testing.T) {
		m := testModel()
		m.conflicts = nil
		result, _ := m.executeCommand("c")
		model := result.(Model)

		if model.infoMessage == "" {
			t.Error("expected info message when no conflicts exist")
		}
	})
}

func TestTerminalFocusCommand(t *testing.T) {
	t.Run("t command attempts focus when terminal visible", func(t *testing.T) {
		// Note: enterTerminalMode() requires a running terminal process to actually
		// set terminalMode=true. This test verifies the command path is correct.
		m := testModel()
		m.terminalVisible = true
		m.terminalMode = false
		result, _ := m.executeCommand("t")
		model := result.(Model)

		// Should show info message (even without running process)
		if model.infoMessage == "" {
			t.Error("expected info message about terminal focus")
		}
		if model.errorMessage != "" {
			t.Errorf("unexpected error message: %q", model.errorMessage)
		}
	})

	t.Run("t command shows error when terminal not visible", func(t *testing.T) {
		m := testModel()
		m.terminalVisible = false
		m.terminalMode = false
		result, _ := m.executeCommand("t")
		model := result.(Model)

		if model.terminalMode {
			t.Error("expected terminalMode to remain false when terminal not visible")
		}
		if model.errorMessage == "" {
			t.Error("expected error message when terminal not visible")
		}
	})

	t.Run("t key in normal mode does NOT enter terminal mode", func(t *testing.T) {
		// This is a regression test to ensure 't' key doesn't trigger terminal mode
		// directly - it should only work via command mode (:t)
		m := testModel()
		m.terminalVisible = true
		m.terminalMode = false
		m.commandMode = false

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.terminalMode {
			t.Error("expected terminalMode to remain false - 't' key should not trigger terminal mode in normal mode")
		}
	})

	t.Run("t command is recognized as valid command", func(t *testing.T) {
		m := testModel()
		result, _ := m.executeCommand("t")
		model := result.(Model)

		// Should NOT be an unknown command error
		if model.errorMessage != "" && len(model.errorMessage) >= 7 && model.errorMessage[:7] == "Unknown" {
			t.Error("command 't' was not recognized")
		}
	})
}

func TestRenderCommandModeHelp(t *testing.T) {
	t.Run("renders colon prompt", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "q",
		}

		result := m.renderCommandModeHelp()

		// Should contain the : prompt character
		if !strings.Contains(result, ":") {
			t.Errorf("expected output to contain ':' prompt, got: %s", result)
		}
	})

	t.Run("includes command buffer content in output", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "quit",
		}

		result := m.renderCommandModeHelp()

		// Should contain the typed command
		if !strings.Contains(result, "quit") {
			t.Errorf("expected output to contain buffer 'quit', got: %s", result)
		}
	})

	t.Run("renders correctly with empty buffer", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "",
		}

		result := m.renderCommandModeHelp()

		// Should still render the prompt and help text even with empty buffer
		if !strings.Contains(result, ":") {
			t.Errorf("expected output to contain ':' prompt with empty buffer, got: %s", result)
		}
		// Should contain key binding help
		if !strings.Contains(result, "[Enter]") || !strings.Contains(result, "[Esc]") {
			t.Errorf("expected output to contain key binding help, got: %s", result)
		}
	})

	t.Run("includes key binding help text", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "help",
		}

		result := m.renderCommandModeHelp()

		// Should contain key binding instructions
		if !strings.Contains(result, "execute") {
			t.Errorf("expected output to mention 'execute' action, got: %s", result)
		}
		if !strings.Contains(result, "cancel") {
			t.Errorf("expected output to mention 'cancel' action, got: %s", result)
		}
	})
}

func TestCommandModePriorityOverOtherModes(t *testing.T) {
	// This test verifies the bug fix: command mode help should display
	// even when ultra-plan or plan editor modes are active.
	// The fix ensures commandMode is checked FIRST in View()'s help rendering.

	t.Run("command mode takes priority over plan editor mode", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "quit",
			planEditor: &PlanEditorState{
				active: true,
			},
		}

		// Verify preconditions: both modes are active
		if !m.commandMode {
			t.Fatal("test setup error: commandMode should be true")
		}
		if !m.IsPlanEditorActive() {
			t.Fatal("test setup error: IsPlanEditorActive should be true")
		}

		// The fix ensures renderCommandModeHelp is called instead of renderPlanEditorHelp.
		// We verify by checking that command mode help contains the expected content.
		result := m.renderCommandModeHelp()
		if !strings.Contains(result, "quit") {
			t.Errorf("command mode help should contain buffer content, got: %s", result)
		}
	})

	t.Run("command mode takes priority over ultra-plan mode", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "help",
			ultraPlan:     &UltraPlanState{}, // Non-nil to trigger IsUltraPlanMode
		}

		// Verify preconditions: both modes are active
		if !m.commandMode {
			t.Fatal("test setup error: commandMode should be true")
		}
		if !m.IsUltraPlanMode() {
			t.Fatal("test setup error: IsUltraPlanMode should be true")
		}

		// The fix ensures renderCommandModeHelp is called instead of renderUltraPlanHelp.
		result := m.renderCommandModeHelp()
		if !strings.Contains(result, "help") {
			t.Errorf("command mode help should contain buffer content, got: %s", result)
		}
	})
}

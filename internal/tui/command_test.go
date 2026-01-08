package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHandleCommandInput(t *testing.T) {
	t.Run("escape exits command mode", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "test",
		}

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
		m := Model{
			commandMode:   true,
			commandBuffer: "help",
		}

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
		m := Model{
			commandMode:   true,
			commandBuffer: "test",
		}

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
		m := Model{
			commandMode:   true,
			commandBuffer: "a",
		}

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
		m := Model{
			commandMode:   true,
			commandBuffer: "te",
		}

		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s', 't'}}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandBuffer != "test" {
			t.Errorf("expected commandBuffer to be %q, got %q", "test", model.commandBuffer)
		}
	})

	t.Run("space adds to buffer", func(t *testing.T) {
		m := Model{
			commandMode:   true,
			commandBuffer: "add",
		}

		msg := tea.KeyMsg{Type: tea.KeySpace}
		result, _ := m.handleCommandInput(msg)
		model := result.(Model)

		if model.commandBuffer != "add " {
			t.Errorf("expected commandBuffer to be %q, got %q", "add ", model.commandBuffer)
		}
	})
}

func TestExecuteCommand(t *testing.T) {
	t.Run("empty command does nothing", func(t *testing.T) {
		m := Model{}
		result, _ := m.executeCommand("")
		model := result.(Model)

		if model.errorMessage != "" {
			t.Errorf("expected no error, got %q", model.errorMessage)
		}
	})

	t.Run("whitespace-only command does nothing", func(t *testing.T) {
		m := Model{}
		result, _ := m.executeCommand("   ")
		model := result.(Model)

		if model.errorMessage != "" {
			t.Errorf("expected no error, got %q", model.errorMessage)
		}
	})

	t.Run("unknown command sets error", func(t *testing.T) {
		m := Model{}
		result, _ := m.executeCommand("unknowncommand")
		model := result.(Model)

		if model.errorMessage == "" {
			t.Error("expected error message for unknown command")
		}
	})

	t.Run("help command toggles help", func(t *testing.T) {
		m := Model{showHelp: false}
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
		m := Model{showHelp: false}
		result, _ := m.executeCommand("h")
		model := result.(Model)

		if !model.showHelp {
			t.Error("expected showHelp to be true")
		}
	})

	t.Run("quit command sets quitting", func(t *testing.T) {
		m := Model{}
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
		m := Model{}
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
		m := Model{addingTask: false}
		result, _ := m.executeCommand("add")
		model := result.(Model)

		if !model.addingTask {
			t.Error("expected addingTask to be true")
		}
	})

	t.Run("a command is alias for add", func(t *testing.T) {
		m := Model{addingTask: false}
		result, _ := m.executeCommand("a")
		model := result.(Model)

		if !model.addingTask {
			t.Error("expected addingTask to be true")
		}
	})

	t.Run("stats command toggles stats panel", func(t *testing.T) {
		m := Model{showStats: false}
		result, _ := m.executeCommand("stats")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})

	t.Run("m command is alias for stats", func(t *testing.T) {
		m := Model{showStats: false}
		result, _ := m.executeCommand("m")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})

	t.Run("metrics command is alias for stats", func(t *testing.T) {
		m := Model{showStats: false}
		result, _ := m.executeCommand("metrics")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})

	t.Run("filter command starts filter mode", func(t *testing.T) {
		m := Model{filterMode: false}
		result, _ := m.executeCommand("filter")
		model := result.(Model)

		if !model.filterMode {
			t.Error("expected filterMode to be true")
		}
	})

	t.Run("f command is alias for filter", func(t *testing.T) {
		m := Model{filterMode: false}
		result, _ := m.executeCommand("f")
		model := result.(Model)

		if !model.filterMode {
			t.Error("expected filterMode to be true")
		}
	})

	t.Run("F command is alias for filter", func(t *testing.T) {
		m := Model{filterMode: false}
		result, _ := m.executeCommand("F")
		model := result.(Model)

		if !model.filterMode {
			t.Error("expected filterMode to be true")
		}
	})

	t.Run("diff command toggles diff panel when no instance", func(t *testing.T) {
		m := Model{showDiff: true, diffContent: "some diff"}
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
		m := Model{showDiff: true, diffContent: "some diff"}
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
			m := Model{}
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
		"p", "pause",
		"R", "reconnect",
		"restart",
		// Instance management
		"D", "remove",
		"kill",
		// Utilities
		"t", "tmux",
		"r", "pr",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			m := Model{
				session: nil, // No session means no instances
			}
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
		m := Model{
			conflicts: nil,
		}
		result, _ := m.executeCommand("conflicts")
		model := result.(Model)

		if model.infoMessage == "" {
			t.Error("expected info message when no conflicts exist")
		}
	})

	t.Run("c alias shows message when no conflicts", func(t *testing.T) {
		m := Model{
			conflicts: nil,
		}
		result, _ := m.executeCommand("c")
		model := result.(Model)

		if model.infoMessage == "" {
			t.Error("expected info message when no conflicts exist")
		}
	})
}

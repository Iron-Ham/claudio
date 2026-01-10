package tui

import (
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

// tickMsg is sent periodically to update the UI with latest output
type tickMsg time.Time

// outputMsg contains new output data from an instance
type outputMsg struct {
	instanceID string
	data       []byte
}

// errMsg wraps an error for display in the UI
type errMsg struct {
	err error
}

// prCompleteMsg is sent when a PR workflow completes for an instance
type prCompleteMsg struct {
	instanceID string
	success    bool
}

// prOpenedMsg is sent when a PR URL is detected in instance output
type prOpenedMsg struct {
	instanceID string
}

// timeoutMsg is sent when an instance times out or becomes stuck
type timeoutMsg struct {
	instanceID  string
	timeoutType instance.TimeoutType
}

// bellMsg is sent when a terminal bell is detected in a tmux session
type bellMsg struct {
	instanceID string
}

// taskAddedMsg is sent when async task addition completes
type taskAddedMsg struct {
	instance *orchestrator.Instance
	err      error
}

// ultraPlanInitMsg signals that ultra-plan mode should initialize
type ultraPlanInitMsg struct{}

// Commands

// tick returns a command that sends a tickMsg after a short delay.
// This drives the periodic UI updates for reading instance output.
func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ringBell returns a command that outputs a terminal bell character.
// This forwards bells from tmux sessions to the parent terminal.
func ringBell() tea.Cmd {
	return func() tea.Msg {
		// Write the bell character directly to stdout
		// This works even when Bubbletea is in alt-screen mode
		_, _ = os.Stdout.Write([]byte{'\a'})
		return nil
	}
}

// notifyUser returns a command that notifies the user via bell and optional sound.
// Used to alert the user when ultraplan needs input (e.g., plan ready, synthesis ready).
func notifyUser() tea.Cmd {
	return func() tea.Msg {
		if !viper.GetBool("ultraplan.notifications.enabled") {
			return nil
		}

		// Always ring terminal bell
		_, _ = os.Stdout.Write([]byte{'\a'})

		// Optionally play system sound on macOS
		if runtime.GOOS == "darwin" && viper.GetBool("ultraplan.notifications.use_sound") {
			soundPath := viper.GetString("ultraplan.notifications.sound_path")
			if soundPath == "" {
				soundPath = "/System/Library/Sounds/Glass.aiff"
			}
			// Start in background so it doesn't block
			_ = exec.Command("afplay", soundPath).Start()
		}
		return nil
	}
}

// addTaskAsync returns a command that adds a task asynchronously.
// This prevents the UI from blocking while git creates the worktree.
func addTaskAsync(o *orchestrator.Orchestrator, session *orchestrator.Session, task string) tea.Cmd {
	return func() tea.Msg {
		inst, err := o.AddInstance(session, task)
		return taskAddedMsg{instance: inst, err: err}
	}
}

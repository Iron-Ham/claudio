package instance

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// TmuxConfig holds configuration for tmux session creation
type TmuxConfig struct {
	Width  int
	Height int
}

// DefaultTmuxConfig returns sensible defaults for tmux sessions
func DefaultTmuxConfig() TmuxConfig {
	return TmuxConfig{
		Width:  200,
		Height: 30,
	}
}

// createTmuxSession creates a new detached tmux session with the given name and working directory.
// It configures the session for color support and bell monitoring.
func createTmuxSession(sessionName, workdir string, cfg TmuxConfig) error {
	// Kill any existing session with this name (cleanup from previous run)
	_ = exec.Command("tmux", "kill-session", "-t", sessionName).Run()

	// Create a new detached tmux session with color support
	createCmd := exec.Command("tmux",
		"new-session",
		"-d",                              // detached
		"-s", sessionName,                 // session name
		"-x", fmt.Sprintf("%d", cfg.Width),  // width
		"-y", fmt.Sprintf("%d", cfg.Height), // height
	)
	createCmd.Dir = workdir
	// Inherit full environment (required for Claude credentials) and ensure TERM supports colors
	createCmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set up the tmux session for color support and large history
	_ = exec.Command("tmux", "set-option", "-t", sessionName, "history-limit", "10000").Run()
	_ = exec.Command("tmux", "set-option", "-t", sessionName, "default-terminal", "xterm-256color").Run()
	// Enable bell monitoring so we can detect and forward terminal bells
	_ = exec.Command("tmux", "set-option", "-t", sessionName, "-w", "monitor-bell", "on").Run()

	return nil
}

// killTmuxSession terminates a tmux session by name
func killTmuxSession(sessionName string) error {
	return exec.Command("tmux", "kill-session", "-t", sessionName).Run()
}

// sendTmuxKeys sends keys to a tmux session.
// If literal is true, keys are sent without interpretation using the -l flag.
func sendTmuxKeys(sessionName, keys string, literal bool) error {
	args := []string{"send-keys", "-t", sessionName}
	if literal {
		args = append(args, "-l")
	}
	args = append(args, keys)
	return exec.Command("tmux", args...).Run()
}

// sendTmuxSpecialKey sends a special key (Enter, Tab, Escape, etc.) to a tmux session
func sendTmuxSpecialKey(sessionName, key string) error {
	return exec.Command("tmux", "send-keys", "-t", sessionName, key).Run()
}

// captureTmuxOutput captures the entire visible pane plus scrollback from a tmux session.
// It preserves ANSI escape sequences for color support.
func captureTmuxOutput(sessionName string) ([]byte, error) {
	// Capture the entire visible pane plus scrollback
	// -p prints to stdout, -S - starts from beginning of history
	// -e preserves ANSI escape sequences (colors)
	captureCmd := exec.Command("tmux",
		"capture-pane",
		"-t", sessionName,
		"-p",      // print to stdout
		"-e",      // preserve escape sequences (colors)
		"-S", "-", // start from beginning of scrollback
		"-E", "-", // end at bottom of scrollback
	)
	return captureCmd.Output()
}

// tmuxSessionExists checks if a tmux session with the given name exists
func tmuxSessionExists(sessionName string) bool {
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	return cmd.Run() == nil
}

// tmuxDisplayMessage queries a tmux format variable from a session
func tmuxDisplayMessage(sessionName, format string) (string, error) {
	cmd := exec.Command("tmux", "display-message", "-t", sessionName, "-p", format)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// tmuxGetBellFlag returns whether the bell flag is set for the session's window
func tmuxGetBellFlag(sessionName string) (bool, error) {
	result, err := tmuxDisplayMessage(sessionName, "#{window_bell_flag}")
	if err != nil {
		return false, err
	}
	return result == "1", nil
}

// tmuxGetPanePID returns the process ID of the shell in the tmux session's pane
func tmuxGetPanePID(sessionName string) (int, error) {
	result, err := tmuxDisplayMessage(sessionName, "#{pane_pid}")
	if err != nil {
		return 0, err
	}

	var pid int
	_, _ = fmt.Sscanf(result, "%d", &pid)
	return pid, nil
}

// resizeTmuxWindow changes the dimensions of a tmux window
func resizeTmuxWindow(sessionName string, width, height int) error {
	resizeCmd := exec.Command("tmux",
		"resize-window",
		"-t", sessionName,
		"-x", fmt.Sprintf("%d", width),
		"-y", fmt.Sprintf("%d", height),
	)
	if err := resizeCmd.Run(); err != nil {
		return fmt.Errorf("failed to resize tmux session: %w", err)
	}
	return nil
}

// setTmuxOption sets a tmux option for a session
func setTmuxOption(sessionName, option, value string) error {
	return exec.Command("tmux", "set-option", "-t", sessionName, option, value).Run()
}

// setTmuxWindowOption sets a window-level tmux option for a session
func setTmuxWindowOption(sessionName, option, value string) error {
	return exec.Command("tmux", "set-option", "-t", sessionName, "-w", option, value).Run()
}

// listTmuxSessions returns all tmux session names
func listTmuxSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// No sessions or tmux not running
		return nil, nil
	}

	var sessions []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			sessions = append(sessions, line)
		}
	}
	return sessions, nil
}

// listClaudioTmuxSessions returns all tmux sessions with the claudio- prefix
func listClaudioTmuxSessions() ([]string, error) {
	allSessions, err := listTmuxSessions()
	if err != nil {
		return nil, err
	}

	var sessions []string
	for _, s := range allSessions {
		if strings.HasPrefix(s, "claudio-") {
			sessions = append(sessions, s)
		}
	}
	return sessions, nil
}

// sendBracketedPaste sends text with bracketed paste mode escape sequences.
// This preserves the paste context for applications that support bracketed paste mode.
func sendBracketedPaste(sessionName, text string) error {
	// Bracketed paste mode escape sequences
	// Start: ESC[200~ End: ESC[201~
	pasteStart := "\x1b[200~"
	pasteEnd := "\x1b[201~"

	// Send bracketed paste start
	if err := sendTmuxKeys(sessionName, pasteStart, true); err != nil {
		return err
	}
	// Send the pasted content
	if err := sendTmuxKeys(sessionName, text, true); err != nil {
		return err
	}
	// Send bracketed paste end
	return sendTmuxKeys(sessionName, pasteEnd, true)
}

// mapRuneToTmuxKey converts a rune to the appropriate tmux key representation
func mapRuneToTmuxKey(r rune) string {
	switch r {
	case '\r', '\n':
		return "Enter"
	case '\t':
		return "Tab"
	case '\x7f', '\b': // backspace
		return "BSpace"
	case '\x1b': // escape
		return "Escape"
	case ' ':
		return "Space"
	default:
		if r < 32 {
			// Control character: Ctrl+letter
			return fmt.Sprintf("C-%c", r+'a'-1)
		}
		// Regular character
		return string(r)
	}
}

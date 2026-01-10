package instance

import (
	"os/exec"
	"strings"
	"sync"
)

// BellCallback is called when a terminal bell is detected in the tmux session
type BellCallback func(instanceID string)

// BellHandler manages terminal bell detection and notification for a tmux session.
// It uses edge detection to trigger callbacks only on the transition from no-bell
// to bell state, preventing continuous firing while the flag remains set.
type BellHandler struct {
	mu            sync.RWMutex
	callback      BellCallback
	lastBellState bool // Track last bell flag state to detect transitions
}

// NewBellHandler creates a new BellHandler instance
func NewBellHandler() *BellHandler {
	return &BellHandler{
		lastBellState: false,
	}
}

// SetCallback sets the callback that will be invoked when a terminal bell is detected
func (h *BellHandler) SetCallback(cb BellCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callback = cb
}

// Reset resets the bell state (useful for reconnection scenarios)
func (h *BellHandler) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastBellState = false
}

// CheckAndForward checks for terminal bells in the tmux session and triggers
// the callback if a bell transition is detected. It uses edge detection to
// ensure we only fire once per bell, not continuously while the flag is set.
func (h *BellHandler) CheckAndForward(sessionName, instanceID string) {
	bellActive, err := h.queryBellFlag(sessionName)
	if err != nil {
		return
	}

	h.mu.Lock()
	lastBellState := h.lastBellState
	callback := h.callback
	h.lastBellState = bellActive
	h.mu.Unlock()

	// Trigger callback on transition from no-bell to bell (edge detection)
	// This ensures we only fire once per bell, not continuously while the flag is set
	if bellActive && !lastBellState && callback != nil {
		callback(instanceID)
	}
}

// queryBellFlag queries the window_bell_flag from tmux for the given session
func (h *BellHandler) queryBellFlag(sessionName string) (bool, error) {
	bellCmd := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{window_bell_flag}")
	output, err := bellCmd.Output()
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(string(output)) == "1", nil
}

// EnableBellMonitoring ensures the tmux session has bell monitoring enabled.
// This should be called when creating or reconnecting to a session.
func EnableBellMonitoring(sessionName string) error {
	return setTmuxWindowOption(sessionName, "monitor-bell", "on")
}

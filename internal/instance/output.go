package instance

import (
	"os/exec"
	"strings"
	"time"
)

// Output capture, polling, and processing logic for the instance manager.
// This file contains the capture loop and related output processing functions
// that periodically poll the tmux session and process the captured output.

// captureLoop periodically captures output from the tmux session
func (m *Manager) captureLoop() {
	// Track last output hash to detect changes
	var lastOutput string

	for {
		select {
		case <-m.doneChan:
			return
		case <-m.captureTick.C:
			m.mu.RLock()
			if !m.running || m.paused {
				m.mu.RUnlock()
				continue
			}
			sessionName := m.sessionName
			timedOut := m.timedOut
			m.mu.RUnlock()

			// Skip processing if already timed out
			if timedOut {
				continue
			}

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
			output, err := captureCmd.Output()
			if err != nil {
				continue
			}

			// Always update if content changed
			currentOutput := string(output)
			if currentOutput != lastOutput {
				m.outputBuf.Reset()
				_, _ = m.outputBuf.Write(output)

				// Update activity tracking
				m.mu.Lock()
				m.lastActivityTime = time.Now()
				m.lastOutputHash = lastOutput
				m.repeatedOutputCount = 0
				m.mu.Unlock()

				lastOutput = currentOutput

				// Detect waiting state from the new output
				m.detectAndNotifyState(output)
			} else {
				// Output hasn't changed - check for stale detection
				m.mu.Lock()
				if m.config.StaleDetection {
					m.repeatedOutputCount++
				}
				m.mu.Unlock()
			}

			// Check for timeout conditions
			m.checkTimeouts()

			// Check for terminal bells and forward them
			m.checkAndForwardBell(sessionName)

			// Check if the session is still running
			checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended - notify completion and stop
				m.mu.Lock()
				m.running = false
				callback := m.stateCallback
				instanceID := m.id
				m.currentState = StateCompleted
				m.mu.Unlock()

				// Fire the completion callback so coordinator knows task is done
				if callback != nil {
					callback(instanceID, StateCompleted)
				}
				return
			}
		}
	}
}

// checkTimeouts checks for various timeout conditions and triggers callbacks
func (m *Manager) checkTimeouts() {
	m.mu.Lock()
	if m.timedOut || !m.running || m.paused {
		m.mu.Unlock()
		return
	}

	now := time.Now()
	callback := m.timeoutCallback
	instanceID := m.id
	var triggeredTimeout *TimeoutType

	// Check completion timeout (total runtime)
	if m.config.CompletionTimeoutMinutes > 0 && m.startTime != nil {
		completionTimeout := time.Duration(m.config.CompletionTimeoutMinutes) * time.Minute
		if now.Sub(*m.startTime) > completionTimeout {
			t := TimeoutCompletion
			triggeredTimeout = &t
			m.timedOut = true
			m.timeoutType = TimeoutCompletion
		}
	}

	// Check activity timeout (no output changes)
	if triggeredTimeout == nil && m.config.ActivityTimeoutMinutes > 0 {
		activityTimeout := time.Duration(m.config.ActivityTimeoutMinutes) * time.Minute
		if now.Sub(m.lastActivityTime) > activityTimeout {
			t := TimeoutActivity
			triggeredTimeout = &t
			m.timedOut = true
			m.timeoutType = TimeoutActivity
		}
	}

	// Check for stale detection (repeated identical output)
	// Trigger if we've seen the same output 3000 times (5 minutes at 100ms interval)
	// This catches stuck loops producing identical output while allowing time for
	// legitimate long-running operations like planning and exploration
	if triggeredTimeout == nil && m.config.StaleDetection && m.repeatedOutputCount > 3000 {
		t := TimeoutStale
		triggeredTimeout = &t
		m.timedOut = true
		m.timeoutType = TimeoutStale
	}

	m.mu.Unlock()

	// Invoke callback outside of lock to prevent deadlocks
	if triggeredTimeout != nil && callback != nil {
		callback(instanceID, *triggeredTimeout)
	}
}

// checkAndForwardBell checks for terminal bells and triggers the callback if detected
func (m *Manager) checkAndForwardBell(sessionName string) {
	// Query the window_bell_flag from tmux
	bellCmd := exec.Command("tmux", "display-message", "-t", sessionName, "-p", "#{window_bell_flag}")
	output, err := bellCmd.Output()
	if err != nil {
		return
	}

	bellActive := strings.TrimSpace(string(output)) == "1"

	m.mu.Lock()
	lastBellState := m.lastBellState
	callback := m.bellCallback
	instanceID := m.id
	m.lastBellState = bellActive
	m.mu.Unlock()

	// Trigger callback on transition from no-bell to bell (edge detection)
	// This ensures we only fire once per bell, not continuously while the flag is set
	if bellActive && !lastBellState && callback != nil {
		callback(instanceID)
	}
}

// detectAndNotifyState analyzes output and notifies if state changed
func (m *Manager) detectAndNotifyState(output []byte) {
	newState := m.detector.Detect(output)

	m.mu.Lock()
	oldState := m.currentState
	callback := m.stateCallback
	instanceID := m.id

	if newState != oldState {
		m.currentState = newState
	}
	m.mu.Unlock()

	// Invoke callback outside of lock to prevent deadlocks
	if newState != oldState && callback != nil {
		callback(instanceID, newState)
	}

	// Parse and notify about metrics changes
	m.parseAndNotifyMetrics(output)
}

// parseAndNotifyMetrics parses metrics from output and notifies if changed
func (m *Manager) parseAndNotifyMetrics(output []byte) {
	newMetrics := m.metricsParser.Parse(output)
	if newMetrics == nil {
		return
	}

	m.mu.Lock()
	oldMetrics := m.currentMetrics
	callback := m.metricsCallback
	instanceID := m.id

	// Check if metrics changed (simple comparison)
	metricsChanged := oldMetrics == nil ||
		newMetrics.InputTokens != oldMetrics.InputTokens ||
		newMetrics.OutputTokens != oldMetrics.OutputTokens ||
		newMetrics.Cost != oldMetrics.Cost

	if metricsChanged {
		m.currentMetrics = newMetrics
	}
	m.mu.Unlock()

	// Invoke callback outside of lock to prevent deadlocks
	if metricsChanged && callback != nil {
		callback(instanceID, newMetrics)
	}
}

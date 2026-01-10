package instance

import (
	"os/exec"
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
			instanceID := m.id
			m.mu.RUnlock()

			// Skip processing if already timed out
			timedOut, _ := m.timeoutHandler.TimedOut()
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

				// Update activity tracking via timeout handler
				m.timeoutHandler.RecordActivity(lastOutput)

				lastOutput = currentOutput

				// Detect waiting state from the new output
				m.detectAndNotifyState(output)
			} else {
				// Output hasn't changed - record for stale detection
				m.timeoutHandler.RecordRepeatedOutput()
			}

			// Check for timeout conditions via timeout handler
			m.mu.RLock()
			running := m.running
			paused := m.paused
			m.mu.RUnlock()
			m.timeoutHandler.CheckTimeouts(instanceID, running, paused)

			// Check for terminal bells and forward them
			m.bellHandler.CheckAndForward(sessionName, m.id)

			// Check if the session is still running
			checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended - notify completion and stop
				m.mu.Lock()
				m.running = false
				callback := m.stateCallback
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

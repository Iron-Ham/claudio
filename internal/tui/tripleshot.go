package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// TripleShotState is an alias to the view package's TripleShotState.
// This allows the main tui package to use the state without importing view.
type TripleShotState = view.TripleShotState

// dispatchTripleShotCompletionChecks returns commands to check tripleshot completion
// files asynchronously. This avoids blocking the UI event loop with file I/O.
func (m *Model) dispatchTripleShotCompletionChecks() []tea.Cmd {
	if m.tripleShot == nil || !m.tripleShot.HasActiveCoordinators() {
		return nil
	}

	var cmds []tea.Cmd

	// Dispatch async check for each coordinator
	for groupID, coordinator := range m.tripleShot.Coordinators {
		session := coordinator.Session()
		if session == nil {
			continue
		}

		// Only check if in a phase that requires polling
		switch session.Phase {
		case orchestrator.PhaseTripleShotWorking, orchestrator.PhaseTripleShotEvaluating:
			cmds = append(cmds, tuimsg.CheckTripleShotCompletionAsync(coordinator, groupID))
		}
	}

	return cmds
}

// handleTripleShotCheckResult processes the async completion check results.
// This is called when a checkTripleShotCompletionAsync command completes.
func (m *Model) handleTripleShotCheckResult(msg tuimsg.TripleShotCheckResultMsg) (tea.Model, tea.Cmd) {
	if m.tripleShot == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.tripleShot.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("tripleshot check result for unknown coordinator",
				"group_id", msg.GroupID,
				"phase", msg.Phase,
			)
		}
		return m, nil
	}

	session := coordinator.Session()
	if session == nil {
		if m.logger != nil {
			m.logger.Warn("tripleshot coordinator has nil session",
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	switch msg.Phase {
	case orchestrator.PhaseTripleShotWorking:
		return m.processAttemptCheckResults(coordinator, session, msg)

	case orchestrator.PhaseTripleShotEvaluating:
		return m.processJudgeCheckResult(coordinator, msg)

	case orchestrator.PhaseTripleShotFailed:
		// Show error message for failed tripleshot
		if session.Error != "" && m.errorMessage == "" {
			m.errorMessage = "Triple-shot failed: " + session.Error
		}
		return m, nil
	}

	return m, nil
}

// processAttemptCheckResults handles completion check results for attempts.
// Returns async commands to process any completed attempts without blocking the UI.
func (m *Model) processAttemptCheckResults(
	coordinator *orchestrator.TripleShotCoordinator,
	session *orchestrator.TripleShotSession,
	msg tuimsg.TripleShotCheckResultMsg,
) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Dispatch async processing for each completed attempt
	for i, complete := range msg.AttemptResults {
		if err, hasErr := msg.AttemptErrors[i]; hasErr && err != nil {
			if m.logger != nil {
				m.logger.Warn("failed to check attempt completion",
					"attempt_index", i,
					"error", err,
				)
			}
			continue
		}

		if complete {
			// Dispatch async command to process the completion file
			// This avoids blocking the UI with file I/O
			cmds = append(cmds, tuimsg.ProcessAttemptCompletionAsync(coordinator, msg.GroupID, i))
		}
	}

	if len(cmds) > 0 {
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// handleTripleShotAttemptProcessed handles the result of async attempt completion processing.
func (m *Model) handleTripleShotAttemptProcessed(msg tuimsg.TripleShotAttemptProcessedMsg) (tea.Model, tea.Cmd) {
	if m.tripleShot == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.tripleShot.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process attempt completion",
				"attempt_index", msg.AttemptIndex,
				"error", msg.Err,
			)
		}
		m.errorMessage = fmt.Sprintf("Failed to process attempt %d completion", msg.AttemptIndex+1)
		return m, nil
	}

	m.infoMessage = "Attempt completed - checking progress..."

	// Check if all attempts are complete and we should start the judge
	session := coordinator.Session()
	if session != nil && session.AllAttemptsComplete() && session.JudgeID == "" {
		if session.SuccessfulAttemptCount() >= 2 {
			// Return a command to start the judge in a goroutine
			return m, func() tea.Msg {
				if err := coordinator.StartJudge(); err != nil {
					return tuimsg.TripleShotErrorMsg{Err: fmt.Errorf("failed to start judge: %w", err)}
				}
				return tuimsg.TripleShotJudgeStartedMsg{}
			}
		}
	}

	return m, nil
}

// processJudgeCheckResult handles completion check results for the judge.
// Returns an async command to process the judge completion file without blocking the UI.
func (m *Model) processJudgeCheckResult(
	coordinator *orchestrator.TripleShotCoordinator,
	msg tuimsg.TripleShotCheckResultMsg,
) (tea.Model, tea.Cmd) {
	if msg.JudgeError != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check judge completion", "error", msg.JudgeError, "group_id", msg.GroupID)
		}
		return m, nil
	}

	if msg.JudgeComplete {
		// Dispatch async command to process the judge completion file
		// This avoids blocking the UI with file I/O
		return m, tuimsg.ProcessJudgeCompletionAsync(coordinator, msg.GroupID)
	}

	return m, nil
}

// handleTripleShotJudgeProcessed handles the result of async judge completion processing.
func (m *Model) handleTripleShotJudgeProcessed(msg tuimsg.TripleShotJudgeProcessedMsg) (tea.Model, tea.Cmd) {
	if m.tripleShot == nil {
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process judge completion", "error", msg.Err)
		}
		m.errorMessage = "Failed to process judge evaluation"
		return m, nil
	}

	m.infoMessage = fmt.Sprintf("Triple-shot complete! (%s)", msg.TaskPreview)
	m.tripleShot.NeedsNotification = true

	return m, nil
}

// handleTripleShotJudgeStopped handles cleanup when a triple-shot judge instance is stopped.
// This cleans up the entire triple-shot session associated with the judge.
func (m *Model) handleTripleShotJudgeStopped(judgeID string) {
	if m.tripleShot == nil {
		return
	}

	// Find the coordinator whose session has this judge
	var coordinatorToStop *orchestrator.TripleShotCoordinator
	var groupIDToRemove string
	for groupID, coord := range m.tripleShot.Coordinators {
		session := coord.Session()
		if session != nil && session.JudgeID == judgeID {
			coordinatorToStop = coord
			groupIDToRemove = groupID
			break
		}
	}

	if coordinatorToStop == nil {
		if m.logger != nil {
			m.logger.Warn("triple-shot judge stop requested but no matching coordinator found",
				"judge_id", judgeID,
				"coordinators_count", len(m.tripleShot.Coordinators),
			)
		}
		return
	}

	if m.logger != nil {
		m.logger.Info("cleaning up triple-shot session after judge stopped", "judge_id", judgeID)
	}

	// Stop the coordinator
	coordinatorToStop.Stop()

	// Remove from coordinators map
	if groupIDToRemove != "" {
		delete(m.tripleShot.Coordinators, groupIDToRemove)
	}

	// If no more coordinators, fully clean up tripleshot state
	if !m.tripleShot.HasActiveCoordinators() {
		m.cleanupTripleShot()
		m.infoMessage = "Triple-shot session ended"
	} else {
		m.infoMessage = "Triple-shot session ended (other sessions still active)"
	}
}

// cleanupTripleShot stops all tripleshot coordinators and clears the tripleshot state.
func (m *Model) cleanupTripleShot() {
	if m.tripleShot == nil {
		return
	}

	// Stop all coordinators to cancel their contexts
	for _, coordinator := range m.tripleShot.Coordinators {
		if coordinator != nil {
			coordinator.Stop()
		}
	}

	// Clear TUI-level state
	m.tripleShot = nil

	// Clear session-level tripleshot state
	if m.session != nil {
		m.session.TripleShots = nil
	}
}

package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// TripleShotState is an alias to the view package's TripleShotState.
// This allows the main tui package to use the state without importing view.
type TripleShotState = view.TripleShotState

// handleTripleShotAccept handles accepting the winning triple-shot solution.
// For multiple tripleshots, accepts the one whose instance is currently focused.
// On error, the user remains in triple-shot mode so they can investigate or retry.
func (m *Model) handleTripleShotAccept() {
	if m.tripleShot == nil || !m.tripleShot.HasActiveCoordinators() {
		if m.logger != nil {
			m.logger.Warn("triple-shot accept failed", "reason", "no active session")
		}
		m.errorMessage = "No active triple-shot session"
		return
	}

	// Find the tripleshot session for the currently active instance
	session := m.findActiveTripleShotSession()
	if session == nil {
		m.errorMessage = "No triple-shot session found for the selected instance"
		return
	}

	if session.Evaluation == nil {
		if m.logger != nil {
			m.logger.Warn("triple-shot accept failed", "reason", "no evaluation available")
		}
		m.errorMessage = "No evaluation available for this triple-shot"
		return
	}

	eval := session.Evaluation

	// Handle different merge strategies
	switch eval.MergeStrategy {
	case orchestrator.MergeStrategySelect:
		// Direct selection - identify the winning branch
		if eval.WinnerIndex < 0 || eval.WinnerIndex >= 3 {
			if m.logger != nil {
				m.logger.Warn("triple-shot accept failed",
					"reason", "invalid winner index",
					"winner_index", eval.WinnerIndex,
				)
			}
			m.errorMessage = "Invalid winner index in evaluation"
			return
		}

		winnerAttempt := session.Attempts[eval.WinnerIndex]
		winningBranch := winnerAttempt.Branch

		if m.logger != nil {
			m.logger.Info("accepting triple-shot solution",
				"strategy", eval.MergeStrategy,
				"winner_index", eval.WinnerIndex,
				"branch", winningBranch,
			)
		}

		m.infoMessage = fmt.Sprintf("Accepted winning solution from attempt %d. Use 'git checkout %s' to switch to the winning branch, or create a PR with 'claudio pr %s'",
			eval.WinnerIndex+1, winningBranch, winnerAttempt.InstanceID)

	case orchestrator.MergeStrategyMerge, orchestrator.MergeStrategyCombine:
		// Merge/combine strategies require manual intervention
		if m.logger != nil {
			m.logger.Info("accepting triple-shot solution",
				"strategy", eval.MergeStrategy,
			)
		}

		m.infoMessage = fmt.Sprintf("Evaluation recommends %s strategy. Review the attempts and suggested changes manually.",
			eval.MergeStrategy)

	default:
		if m.logger != nil {
			m.logger.Warn("triple-shot accept failed",
				"reason", "unknown merge strategy",
				"strategy", eval.MergeStrategy,
			)
		}
		m.errorMessage = fmt.Sprintf("Unknown merge strategy: %s", eval.MergeStrategy)
		return
	}

	// Find and stop the coordinator for this session
	var coordinatorToStop *orchestrator.TripleShotCoordinator
	for groupID, coord := range m.tripleShot.Coordinators {
		if coord.Session() == session {
			coordinatorToStop = coord
			delete(m.tripleShot.Coordinators, groupID)
			break
		}
	}

	// Stop the coordinator's context
	if coordinatorToStop != nil {
		coordinatorToStop.Stop()
	}

	// If no more coordinators, fully clean up tripleshot state
	if !m.tripleShot.HasActiveCoordinators() {
		m.cleanupTripleShot()
	}
}

// findActiveTripleShotSession finds the tripleshot session that contains
// the currently active instance (based on activeTab). Falls back to the
// first coordinator's session if no match is found or no instance is selected.
func (m *Model) findActiveTripleShotSession() *orchestrator.TripleShotSession {
	if m.tripleShot == nil {
		return nil
	}

	if m.session == nil || m.activeTab >= len(m.session.Instances) {
		// If no instance is selected, return the first coordinator's session
		coords := m.tripleShot.GetAllCoordinators()
		if len(coords) > 0 {
			return coords[0].Session()
		}
		return nil
	}

	activeInst := m.session.Instances[m.activeTab]
	if activeInst == nil {
		return nil
	}

	// Search through all coordinators to find which tripleshot owns this instance
	for _, coord := range m.tripleShot.GetAllCoordinators() {
		session := coord.Session()
		if session == nil {
			continue
		}

		// Check if active instance is one of the attempts
		for _, attempt := range session.Attempts {
			if attempt.InstanceID == activeInst.ID {
				return session
			}
		}

		// Check if active instance is the judge
		if session.JudgeID == activeInst.ID {
			return session
		}
	}

	// Default to first coordinator's session if no match
	coords := m.tripleShot.GetAllCoordinators()
	if len(coords) > 0 {
		return coords[0].Session()
	}
	return nil
}

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
			cmds = append(cmds, checkTripleShotCompletionAsync(coordinator, groupID))
		}
	}

	// Also check the deprecated single Coordinator for backward compatibility
	if m.tripleShot.Coordinator != nil {
		// Only process if it's not already in Coordinators map
		alreadyProcessed := false
		for _, coord := range m.tripleShot.Coordinators {
			if coord == m.tripleShot.Coordinator {
				alreadyProcessed = true
				break
			}
		}
		if !alreadyProcessed {
			session := m.tripleShot.Coordinator.Session()
			if session != nil {
				switch session.Phase {
				case orchestrator.PhaseTripleShotWorking, orchestrator.PhaseTripleShotEvaluating:
					cmds = append(cmds, checkTripleShotCompletionAsync(m.tripleShot.Coordinator, ""))
				}
			}
		}
	}

	return cmds
}

// handleTripleShotCheckResult processes the async completion check results.
// This is called when a checkTripleShotCompletionAsync command completes.
func (m *Model) handleTripleShotCheckResult(msg tripleShotCheckResultMsg) (tea.Model, tea.Cmd) {
	if m.tripleShot == nil {
		return m, nil
	}

	// Find the coordinator for this result
	var coordinator *orchestrator.TripleShotCoordinator
	if msg.GroupID != "" {
		coordinator = m.tripleShot.Coordinators[msg.GroupID]
	} else {
		coordinator = m.tripleShot.Coordinator
	}

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
func (m *Model) processAttemptCheckResults(
	coordinator *orchestrator.TripleShotCoordinator,
	session *orchestrator.TripleShotSession,
	msg tripleShotCheckResultMsg,
) (tea.Model, tea.Cmd) {
	// Process each attempt result
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
			// Process the completion result now that we know the file exists.
			// Note: This still performs I/O to read/parse the completion file.
			if err := coordinator.ProcessAttemptCompletion(i); err != nil {
				if m.logger != nil {
					m.logger.Error("failed to process attempt completion",
						"attempt_index", i,
						"error", err,
					)
				}
				m.errorMessage = fmt.Sprintf("Failed to process attempt %d completion", i+1)
			} else {
				m.infoMessage = "Attempt completed - checking progress..."
			}
		}
	}

	// Check if all attempts are complete and we should start the judge
	if session.AllAttemptsComplete() && session.JudgeID == "" {
		if session.SuccessfulAttemptCount() >= 2 {
			// Return a command to start the judge in a goroutine
			return m, func() tea.Msg {
				if err := coordinator.StartJudge(); err != nil {
					return tripleShotErrorMsg{err: fmt.Errorf("failed to start judge: %w", err)}
				}
				return tripleShotJudgeStartedMsg{}
			}
		}
	}

	return m, nil
}

// processJudgeCheckResult handles completion check results for the judge.
func (m *Model) processJudgeCheckResult(
	coordinator *orchestrator.TripleShotCoordinator,
	msg tripleShotCheckResultMsg,
) (tea.Model, tea.Cmd) {
	if msg.JudgeError != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check judge completion", "error", msg.JudgeError, "group_id", msg.GroupID)
		}
		return m, nil
	}

	if msg.JudgeComplete {
		// Process the judge completion result now that we know the file exists.
		// Note: This still performs I/O to read/parse the evaluation file.
		if err := coordinator.ProcessJudgeCompletion(); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to process judge completion", "error", err)
			}
			m.errorMessage = "Failed to process judge evaluation"
		} else {
			session := coordinator.Session()
			taskPreview := ""
			if session != nil && len(session.Task) > 30 {
				taskPreview = session.Task[:27] + "..."
			} else if session != nil {
				taskPreview = session.Task
			}
			m.infoMessage = fmt.Sprintf("Triple-shot complete! (%s) Use :accept to apply the solution", taskPreview)
			m.tripleShot.NeedsNotification = true
		}
	}

	return m, nil
}

// cleanupTripleShot stops all tripleshot coordinators and clears the tripleshot state.
// This handles both the Coordinators map and the deprecated single Coordinator field.
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

	// Stop the deprecated single coordinator if not already in the map
	if m.tripleShot.Coordinator != nil {
		alreadyStopped := false
		for _, coord := range m.tripleShot.Coordinators {
			if coord == m.tripleShot.Coordinator {
				alreadyStopped = true
				break
			}
		}
		if !alreadyStopped {
			m.tripleShot.Coordinator.Stop()
		}
	}

	// Clear TUI-level state
	m.tripleShot = nil

	// Clear session-level tripleshot state
	if m.session != nil {
		m.session.TripleShot = nil
		m.session.TripleShots = nil
	}
}

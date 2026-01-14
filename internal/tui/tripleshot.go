package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/view"
)

// TripleShotState is an alias to the view package's TripleShotState.
// This allows the main tui package to use the state without importing view.
type TripleShotState = view.TripleShotState

// checkForTripleShotCompletion checks if any triple-shot attempts or the judge have completed.
// This is called from the tick handler to poll for completion files.
// For multiple tripleshots, iterates through all active coordinators.
func (m *Model) checkForTripleShotCompletion() {
	if m.tripleShot == nil || !m.tripleShot.HasActiveCoordinators() {
		return
	}

	// Process each coordinator
	for groupID, coordinator := range m.tripleShot.Coordinators {
		session := coordinator.Session()
		if session == nil {
			continue
		}

		switch session.Phase {
		case orchestrator.PhaseTripleShotWorking:
			m.checkTripleShotAttempts(coordinator, session)
		case orchestrator.PhaseTripleShotEvaluating:
			m.checkTripleShotJudge(coordinator, groupID)
		case orchestrator.PhaseTripleShotComplete:
			m.handleTripleShotComplete(session)
		case orchestrator.PhaseTripleShotFailed:
			m.handleTripleShotFailed(session)
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
				case orchestrator.PhaseTripleShotWorking:
					m.checkTripleShotAttempts(m.tripleShot.Coordinator, session)
				case orchestrator.PhaseTripleShotEvaluating:
					m.checkTripleShotJudge(m.tripleShot.Coordinator, "")
				case orchestrator.PhaseTripleShotComplete:
					m.handleTripleShotComplete(session)
				case orchestrator.PhaseTripleShotFailed:
					m.handleTripleShotFailed(session)
				}
			}
		}
	}
}

// checkTripleShotAttempts checks if any attempts have completed their work
func (m *Model) checkTripleShotAttempts(coordinator *orchestrator.TripleShotCoordinator, session *orchestrator.TripleShotSession) {
	allComplete := true
	for i := range 3 {
		attempt := session.Attempts[i]
		if attempt.Status == orchestrator.AttemptStatusWorking {
			complete, err := coordinator.CheckAttemptCompletion(i)
			if err != nil {
				if m.logger != nil {
					m.logger.Warn("failed to check attempt completion",
						"attempt_index", i,
						"error", err,
					)
				}
				continue
			}
			if complete {
				if err := coordinator.ProcessAttemptCompletion(i); err != nil {
					if m.logger != nil {
						m.logger.Error("failed to process attempt completion",
							"attempt_index", i,
							"error", err,
						)
					}
				} else {
					m.infoMessage = "Attempt completed - checking progress..."
				}
			} else {
				allComplete = false
			}
		}
	}

	// If all attempts are complete and we haven't started the judge yet, start it
	if allComplete && session.AllAttemptsComplete() && session.JudgeID == "" {
		if session.SuccessfulAttemptCount() >= 2 {
			if err := coordinator.StartJudge(); err != nil {
				if m.logger != nil {
					m.logger.Error("failed to start judge", "error", err)
				}
				m.errorMessage = "Failed to start judge evaluation"
			} else {
				m.infoMessage = "All attempts complete - judge is evaluating solutions..."
			}
		}
	}
}

// checkTripleShotJudge checks if the judge has completed evaluation
func (m *Model) checkTripleShotJudge(coordinator *orchestrator.TripleShotCoordinator, groupID string) {
	complete, err := coordinator.CheckJudgeCompletion()
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check judge completion", "error", err, "group_id", groupID)
		}
		return
	}

	if complete {
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
}

// handleTripleShotComplete handles when triple-shot has completed successfully
func (m *Model) handleTripleShotComplete(session *orchestrator.TripleShotSession) {
	// Nothing to poll for - the user should take action
	// The notification was already shown when we transitioned to complete
}

// handleTripleShotFailed handles when triple-shot has failed
func (m *Model) handleTripleShotFailed(session *orchestrator.TripleShotSession) {
	// Show error message if we have one
	if session.Error != "" && m.errorMessage == "" {
		m.errorMessage = "Triple-shot failed: " + session.Error
	}
}

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

	// Remove this tripleshot from the coordinators map
	if session.GroupID != "" && m.tripleShot.Coordinators != nil {
		delete(m.tripleShot.Coordinators, session.GroupID)
	}

	// If no more coordinators, clear tripleshot state entirely
	if !m.tripleShot.HasActiveCoordinators() {
		m.tripleShot = nil
	}
}

// findActiveTripleShotSession finds the tripleshot session that contains
// the currently active instance (based on activeTab).
func (m *Model) findActiveTripleShotSession() *orchestrator.TripleShotSession {
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

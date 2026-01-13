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
func (m *Model) checkForTripleShotCompletion() {
	if m.tripleShot == nil || m.tripleShot.Coordinator == nil {
		return
	}

	coordinator := m.tripleShot.Coordinator
	session := coordinator.Session()
	if session == nil {
		return
	}

	switch session.Phase {
	case orchestrator.PhaseTripleShotWorking:
		m.checkTripleShotAttempts(coordinator, session)
	case orchestrator.PhaseTripleShotEvaluating:
		m.checkTripleShotJudge(coordinator)
	case orchestrator.PhaseTripleShotComplete:
		m.handleTripleShotComplete(session)
	case orchestrator.PhaseTripleShotFailed:
		m.handleTripleShotFailed(session)
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
func (m *Model) checkTripleShotJudge(coordinator *orchestrator.TripleShotCoordinator) {
	complete, err := coordinator.CheckJudgeCompletion()
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check judge completion", "error", err)
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
			m.infoMessage = "Triple-shot complete! Use :accept to apply the winning solution"
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
// It identifies the winning attempt, displays acceptance info, and exits triple-shot mode.
// On error, the user remains in triple-shot mode so they can investigate or retry.
func (m *Model) handleTripleShotAccept() {
	if m.tripleShot == nil || m.tripleShot.Coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("triple-shot accept failed", "reason", "no active session")
		}
		m.errorMessage = "No active triple-shot session"
		return
	}

	coordinator := m.tripleShot.Coordinator
	session := coordinator.Session()
	if session == nil || session.Evaluation == nil {
		if m.logger != nil {
			m.logger.Warn("triple-shot accept failed", "reason", "no evaluation available")
		}
		m.errorMessage = "No evaluation available"
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

	// Exit triple-shot mode only on successful acceptance
	m.tripleShot = nil
}

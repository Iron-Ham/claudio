package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
)

// AdversarialState is an alias to the view package's AdversarialState.
// This allows the main tui package to use the state without importing view.
type AdversarialState = view.AdversarialState

// dispatchAdversarialCompletionChecks returns commands to check adversarial completion
// files asynchronously. This avoids blocking the UI event loop with file I/O.
func (m *Model) dispatchAdversarialCompletionChecks() []tea.Cmd {
	if m.adversarial == nil || !m.adversarial.HasActiveCoordinators() {
		return nil
	}

	var cmds []tea.Cmd

	// Dispatch async check for each coordinator
	for groupID, coordinator := range m.adversarial.Coordinators {
		session := coordinator.Session()
		if session == nil {
			continue
		}

		// Only check if in a phase that requires polling
		switch session.Phase {
		case orchestrator.PhaseAdversarialImplementing, orchestrator.PhaseAdversarialReviewing:
			cmds = append(cmds, tuimsg.CheckAdversarialCompletionAsync(coordinator, groupID))
		}
	}

	return cmds
}

// handleAdversarialCheckResult processes the async completion check results.
// This is called when a CheckAdversarialCompletionAsync command completes.
func (m *Model) handleAdversarialCheckResult(msg tuimsg.AdversarialCheckResultMsg) (tea.Model, tea.Cmd) {
	if m.adversarial == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.adversarial.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("adversarial check result for unknown coordinator",
				"group_id", msg.GroupID,
				"phase", msg.Phase,
			)
		}
		return m, nil
	}

	session := coordinator.Session()
	if session == nil {
		if m.logger != nil {
			m.logger.Warn("adversarial coordinator has nil session",
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	switch msg.Phase {
	case orchestrator.PhaseAdversarialImplementing:
		return m.processAdversarialIncrementCheck(coordinator, msg)

	case orchestrator.PhaseAdversarialReviewing:
		return m.processAdversarialReviewCheck(coordinator, msg)

	case orchestrator.PhaseAdversarialFailed:
		// Show error message for failed adversarial session
		if session.Error != "" && m.errorMessage == "" {
			m.errorMessage = "Adversarial review failed: " + session.Error
		}
		return m, nil
	}

	return m, nil
}

// processAdversarialIncrementCheck handles completion check results for the increment file.
// Returns async command to process the increment file if ready.
func (m *Model) processAdversarialIncrementCheck(
	coordinator *orchestrator.AdversarialCoordinator,
	msg tuimsg.AdversarialCheckResultMsg,
) (tea.Model, tea.Cmd) {
	if msg.IncrementError != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check increment completion",
				"error", msg.IncrementError,
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.IncrementReady {
		// Dispatch async command to process the increment file
		return m, tuimsg.ProcessAdversarialIncrementAsync(coordinator, msg.GroupID)
	}

	return m, nil
}

// processAdversarialReviewCheck handles completion check results for the review file.
// Returns async command to process the review file if ready.
func (m *Model) processAdversarialReviewCheck(
	coordinator *orchestrator.AdversarialCoordinator,
	msg tuimsg.AdversarialCheckResultMsg,
) (tea.Model, tea.Cmd) {
	if msg.ReviewError != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check review completion",
				"error", msg.ReviewError,
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.ReviewReady {
		// Dispatch async command to process the review file
		return m, tuimsg.ProcessAdversarialReviewAsync(coordinator, msg.GroupID)
	}

	return m, nil
}

// handleAdversarialIncrementProcessed handles the result of async increment file processing.
func (m *Model) handleAdversarialIncrementProcessed(msg tuimsg.AdversarialIncrementProcessedMsg) (tea.Model, tea.Cmd) {
	if m.adversarial == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.adversarial.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("adversarial increment processed for unknown coordinator",
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process increment file",
				"error", msg.Err,
				"group_id", msg.GroupID,
			)
		}
		m.errorMessage = "Failed to process implementer submission"
		return m, nil
	}

	// The coordinator's ProcessIncrementCompletion automatically starts the reviewer
	session := coordinator.Session()
	if session != nil {
		m.infoMessage = fmt.Sprintf("Round %d: Implementation submitted, reviewer starting...", session.CurrentRound)
	}

	return m, nil
}

// handleAdversarialReviewProcessed handles the result of async review file processing.
func (m *Model) handleAdversarialReviewProcessed(msg tuimsg.AdversarialReviewProcessedMsg) (tea.Model, tea.Cmd) {
	if m.adversarial == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.adversarial.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("adversarial review processed for unknown coordinator",
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process review file",
				"error", msg.Err,
				"group_id", msg.GroupID,
			)
		}
		m.errorMessage = "Failed to process review"
		return m, nil
	}

	session := coordinator.Session()
	if session == nil {
		if m.logger != nil {
			m.logger.Warn("adversarial coordinator has nil session after review",
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.Approved {
		m.infoMessage = fmt.Sprintf("Adversarial review complete! Approved after %d round(s) with score %d/10",
			session.CurrentRound, msg.Score)
		m.adversarial.NeedsNotification = true
	} else {
		m.infoMessage = fmt.Sprintf("Round %d review: Score %d/10 - Changes requested, starting next round...",
			session.CurrentRound-1, msg.Score) // -1 because NextRound was already called
	}

	return m, nil
}

// cleanupAdversarial stops all adversarial coordinators and clears the adversarial state.
func (m *Model) cleanupAdversarial() {
	if m.adversarial == nil {
		return
	}

	// Stop all coordinators to cancel their contexts
	for _, coordinator := range m.adversarial.Coordinators {
		if coordinator != nil {
			coordinator.Stop()
		}
	}

	// Clear TUI-level state
	m.adversarial = nil

	// Clear session-level adversarial state
	if m.session != nil {
		m.session.AdversarialSessions = nil
	}
}

// GetAdversarialCoordinators returns all active adversarial coordinators.
func (m Model) GetAdversarialCoordinators() []*orchestrator.AdversarialCoordinator {
	if m.adversarial == nil {
		return nil
	}
	return m.adversarial.GetAllCoordinators()
}

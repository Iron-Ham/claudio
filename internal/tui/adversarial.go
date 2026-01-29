package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
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

		// Check during active phases and also approved/complete phases
		// (to allow users to reject an approved result by having the reviewer
		// write a new failing review file)
		switch session.Phase {
		case adversarial.PhaseImplementing, adversarial.PhaseReviewing,
			adversarial.PhaseApproved, adversarial.PhaseComplete:
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
	case adversarial.PhaseImplementing:
		return m.processAdversarialIncrementCheck(coordinator, msg)

	case adversarial.PhaseReviewing:
		return m.processAdversarialReviewCheck(coordinator, msg)

	case adversarial.PhaseApproved, adversarial.PhaseComplete:
		// Check if user rejected an approved result by having the reviewer
		// write a new failing review file
		return m.processAdversarialRejectionAfterApprovalCheck(coordinator, msg)

	case adversarial.PhaseFailed:
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
	coordinator *adversarial.Coordinator,
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
	coordinator *adversarial.Coordinator,
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

// processAdversarialRejectionAfterApprovalCheck handles completion check results when in
// approved/complete phase. If a review file is found, it dispatches async processing to
// potentially restart the workflow if the user rejected the approval.
func (m *Model) processAdversarialRejectionAfterApprovalCheck(
	coordinator *adversarial.Coordinator,
	msg tuimsg.AdversarialCheckResultMsg,
) (tea.Model, tea.Cmd) {
	if msg.ReviewError != nil {
		if m.logger != nil {
			m.logger.Warn("failed to check review file in approved phase",
				"error", msg.ReviewError,
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.ReviewReady {
		// A new review file was found after approval - process it to potentially
		// restart the workflow
		return m, tuimsg.ProcessAdversarialRejectionAfterApprovalAsync(coordinator, msg.GroupID)
	}

	return m, nil
}

// handleAdversarialRejectionAfterApprovalProcessed handles the result of processing a
// rejection that occurred after an initial approval.
func (m *Model) handleAdversarialRejectionAfterApprovalProcessed(msg tuimsg.AdversarialRejectionAfterApprovalMsg) (tea.Model, tea.Cmd) {
	if m.adversarial == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.adversarial.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("adversarial rejection after approval for unknown coordinator",
				"group_id", msg.GroupID,
			)
		}
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process rejection after approval",
				"error", msg.Err,
				"group_id", msg.GroupID,
			)
		}
		m.errorMessage = "Failed to process reviewer rejection"
		return m, nil
	}

	session := coordinator.Session()
	if session == nil {
		return m, nil
	}

	// Only show message if the implementer is restarting (rejection was processed)
	if session.Phase == adversarial.PhaseImplementing {
		m.infoMessage = fmt.Sprintf("Approval rejected! Score %d/10 - Starting round %d...",
			msg.Score, session.CurrentRound)
		// Clear the needs notification flag since we're restarting
		m.adversarial.NeedsNotification = false
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
		// Don't collapse the final approved round's sub-group (leave it expanded)
	} else {
		completedRound := session.CurrentRound - 1 // NextRound was already called
		m.infoMessage = fmt.Sprintf("Round %d review: Score %d/10 - Changes requested, starting next round...",
			completedRound, msg.Score)

		// Auto-collapse the completed round's sub-group
		m.collapseAdversarialRound(session, completedRound)
	}

	return m, nil
}

// collapseAdversarialRound collapses the sub-group for a completed adversarial round.
// This keeps the UI clean by hiding completed rounds while preserving the ability
// for users to expand them manually if needed.
func (m *Model) collapseAdversarialRound(session *adversarial.Session, round int) {
	if session == nil {
		return
	}

	// Initialize groupViewState if needed
	if m.groupViewState == nil {
		m.groupViewState = view.NewGroupViewState()
	}

	// Find the round in history
	if round < 1 || round > len(session.History) {
		return
	}

	// Get the sub-group ID for this round
	subGroupID := session.History[round-1].SubGroupID
	if subGroupID == "" {
		return
	}

	// Collapse the sub-group (user can toggle to expand)
	m.groupViewState.CollapsedGroups[subGroupID] = true

	if m.logger != nil {
		m.logger.Info("auto-collapsed adversarial round sub-group",
			"round", round,
			"sub_group_id", subGroupID,
		)
	}
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
func (m Model) GetAdversarialCoordinators() []*adversarial.Coordinator {
	if m.adversarial == nil {
		return nil
	}
	return m.adversarial.GetAllCoordinators()
}

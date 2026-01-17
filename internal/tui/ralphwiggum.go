package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	tea "github.com/charmbracelet/bubbletea"
)

// handleRalphWiggumInit initializes Ralph Wiggum mode by starting the first iteration.
func (m Model) handleRalphWiggumInit() (tea.Model, tea.Cmd) {
	if m.ralphWiggum == nil || !m.ralphWiggum.HasActiveCoordinators() {
		return m, nil
	}

	// Start first iteration for each coordinator that hasn't started
	var cmds []tea.Cmd
	for _, coordinator := range m.ralphWiggum.GetAllCoordinators() {
		session := coordinator.Session()
		if session == nil {
			continue
		}

		// Only start if no instance yet
		if session.Phase == orchestrator.PhaseRalphWiggumIterating && session.InstanceID == "" {
			// Capture loop variables for closure
			coord := coordinator
			groupID := session.GroupID
			cmds = append(cmds, func() tea.Msg {
				if err := coord.StartFirstIteration(); err != nil {
					return tuimsg.RalphWiggumErrorMsg{Err: err}
				}
				return tuimsg.RalphWiggumIterationStartedMsg{
					GroupID:   groupID,
					Iteration: 1,
				}
			})
		}
	}

	if len(cmds) > 0 {
		m.infoMessage = "Ralph Wiggum starting..."
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// handleRalphWiggumIterationComplete handles when an iteration completes.
func (m Model) handleRalphWiggumIterationComplete(msg tuimsg.RalphWiggumIterationCompleteMsg) (tea.Model, tea.Cmd) {
	if m.ralphWiggum == nil {
		return m, nil
	}

	coordinator := m.ralphWiggum.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		return m, nil
	}

	session := coordinator.Session()
	if session == nil {
		return m, nil
	}

	if msg.PromiseFound {
		m.infoMessage = fmt.Sprintf("Ralph Wiggum complete! Promise found after %d iterations.", msg.Iteration)
		m.ralphWiggum.NeedsNotification = true
		return m, nil
	}

	// Check if we should continue
	if session.ShouldContinue() {
		if session.Config.AutoContinue {
			// Capture values before creating closure
			nextIteration := session.CurrentIteration() + 1
			// Auto-continue to next iteration
			return m, func() tea.Msg {
				if err := coordinator.ContinueIteration(); err != nil {
					return tuimsg.RalphWiggumErrorMsg{GroupID: msg.GroupID, Err: err}
				}
				return tuimsg.RalphWiggumIterationStartedMsg{
					GroupID:   msg.GroupID,
					Iteration: nextIteration,
				}
			}
		}
		// Waiting for user to continue
		m.infoMessage = fmt.Sprintf("Iteration %d complete. Press [c] to continue.", msg.Iteration)
	} else {
		// Max iterations reached
		m.infoMessage = fmt.Sprintf("Ralph Wiggum stopped: max iterations (%d) reached.", session.Config.MaxIterations)
	}

	return m, nil
}

// handleRalphWiggumCheckResult processes async completion check results.
func (m Model) handleRalphWiggumCheckResult(msg tuimsg.RalphWiggumCheckResultMsg) (tea.Model, tea.Cmd) {
	if m.ralphWiggum == nil {
		return m, nil
	}

	coordinator := m.ralphWiggum.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Warn("ralph wiggum check error", "group_id", msg.GroupID, "error", msg.Err)
		}
		return m, nil
	}

	// If instance complete, process the iteration
	if msg.InstanceComplete {
		session := coordinator.Session()
		if session == nil {
			return m, nil
		}

		// Process the iteration completion
		if err := coordinator.ProcessIterationComplete(msg.PromiseFound); err != nil {
			if m.logger != nil {
				m.logger.Error("failed to process iteration completion", "error", err)
			}
		}

		// Capture iteration count before creating closure
		currentIteration := session.CurrentIteration()
		return m, func() tea.Msg {
			return tuimsg.RalphWiggumIterationCompleteMsg{
				GroupID:      msg.GroupID,
				Iteration:    currentIteration,
				PromiseFound: msg.PromiseFound,
			}
		}
	}

	return m, nil
}

// dispatchRalphWiggumCompletionChecks returns commands to check Ralph Wiggum instance
// status asynchronously. This avoids blocking the UI event loop.
func (m *Model) dispatchRalphWiggumCompletionChecks() []tea.Cmd {
	if m.ralphWiggum == nil || !m.ralphWiggum.HasActiveCoordinators() {
		return nil
	}

	var cmds []tea.Cmd

	// Dispatch async check for each coordinator
	for groupID, coordinator := range m.ralphWiggum.Coordinators {
		session := coordinator.Session()
		if session == nil {
			continue
		}

		// Only check if in an active phase
		if session.Phase == orchestrator.PhaseRalphWiggumIterating {
			// Capture for closure
			gid := groupID
			coord := coordinator

			cmds = append(cmds, func() tea.Msg {
				complete, promiseFound, err := coord.CheckInstanceCompletion()
				return tuimsg.RalphWiggumCheckResultMsg{
					GroupID:          gid,
					InstanceComplete: complete,
					PromiseFound:     promiseFound,
					Err:              err,
				}
			})
		}
	}

	return cmds
}

// CleanupRalphWiggum stops all Ralph Wiggum coordinators and clears the state.
// Exported for use by the command handler when stopping sessions.
func (m *Model) CleanupRalphWiggum() {
	if m.ralphWiggum == nil {
		return
	}

	// Stop all coordinators
	for _, coordinator := range m.ralphWiggum.Coordinators {
		if coordinator != nil {
			coordinator.Stop()
		}
	}

	// Clear TUI-level state
	m.ralphWiggum = nil

	// Clear session-level state
	if m.session != nil {
		m.session.RalphWiggums = nil
	}
}

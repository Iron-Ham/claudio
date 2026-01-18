package tui

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	"github.com/Iron-Ham/claudio/internal/util"
	tea "github.com/charmbracelet/bubbletea"
)

// RalphState tracks multiple concurrent Ralph Wiggum loop sessions.
// Each ralph session has its own group and coordinator.
type RalphState struct {
	// Coordinators maps group IDs to their ralph coordinators.
	Coordinators map[string]*orchestrator.RalphCoordinator

	// NeedsNotification is set when user notification is needed (checked on tick).
	NeedsNotification bool
}

// HasActiveCoordinators returns true if there are any active ralph coordinators.
func (s *RalphState) HasActiveCoordinators() bool {
	if s == nil || s.Coordinators == nil {
		return false
	}
	for _, coord := range s.Coordinators {
		if coord != nil && coord.Session() != nil && coord.Session().IsActive() {
			return true
		}
	}
	return false
}

// GetCoordinatorForGroup returns the coordinator for a specific group ID.
func (s *RalphState) GetCoordinatorForGroup(groupID string) *orchestrator.RalphCoordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	return s.Coordinators[groupID]
}

// GetAllCoordinators returns all active ralph coordinators.
func (s *RalphState) GetAllCoordinators() []*orchestrator.RalphCoordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	coordinators := make([]*orchestrator.RalphCoordinator, 0, len(s.Coordinators))
	for _, coord := range s.Coordinators {
		if coord != nil {
			coordinators = append(coordinators, coord)
		}
	}
	return coordinators
}

// GetCoordinatorForInstance returns the coordinator that owns the given instance ID.
func (s *RalphState) GetCoordinatorForInstance(instanceID string) *orchestrator.RalphCoordinator {
	if s == nil || s.Coordinators == nil {
		return nil
	}
	for _, coord := range s.Coordinators {
		if coord == nil {
			continue
		}
		session := coord.Session()
		if session == nil {
			continue
		}
		// Check current instance
		if session.InstanceID == instanceID {
			return coord
		}
		// Check all instance IDs in the session
		for _, id := range session.InstanceIDs {
			if id == instanceID {
				return coord
			}
		}
	}
	return nil
}

// dispatchRalphCompletionChecks returns commands to check ralph instance completion
// asynchronously. This avoids blocking the UI event loop with completion checks.
func (m *Model) dispatchRalphCompletionChecks() []tea.Cmd {
	if m.ralph == nil || !m.ralph.HasActiveCoordinators() {
		return nil
	}

	var cmds []tea.Cmd

	// For each active coordinator, check if the current instance has completed
	for groupID, coordinator := range m.ralph.Coordinators {
		session := coordinator.Session()
		if session == nil || !session.IsActive() {
			continue
		}

		// Get current instance
		inst := m.session.GetInstance(session.InstanceID)
		if inst == nil {
			continue
		}

		// Check if instance has completed
		if inst.Status == orchestrator.StatusCompleted {
			// Dispatch async command to process completion
			cmds = append(cmds, tuimsg.ProcessRalphCompletionAsync(coordinator, groupID, inst.ID, m.outputManager))
		}
	}

	return cmds
}

// handleRalphCompletionProcessed handles the result of processing a ralph iteration completion.
func (m *Model) handleRalphCompletionProcessed(msg tuimsg.RalphCompletionProcessedMsg) (tea.Model, tea.Cmd) {
	if m.ralph == nil {
		return m, nil
	}

	// Find the coordinator for this result
	coordinator := m.ralph.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		if m.logger != nil {
			m.logger.Warn("ralph completion result for unknown coordinator", "group_id", msg.GroupID)
		}
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process ralph completion", "error", msg.Err)
		}
		m.errorMessage = fmt.Sprintf("Ralph loop error: %v", msg.Err)
		return m, nil
	}

	session := coordinator.Session()
	if session == nil {
		return m, nil
	}

	// Check the session phase
	switch session.Phase {
	case orchestrator.PhaseRalphComplete:
		m.infoMessage = fmt.Sprintf("Ralph loop completed! Found completion promise after %d iteration(s)", session.CurrentIteration)
		m.ralph.NeedsNotification = true

	case orchestrator.PhaseRalphMaxIterations:
		m.infoMessage = fmt.Sprintf("Ralph loop stopped: max iterations (%d) reached", session.Config.MaxIterations)
		m.ralph.NeedsNotification = true

	case orchestrator.PhaseRalphCancelled:
		m.infoMessage = "Ralph loop cancelled"

	case orchestrator.PhaseRalphError:
		m.errorMessage = fmt.Sprintf("Ralph loop failed: %s", session.Error)

	case orchestrator.PhaseRalphWorking:
		// Continue loop - start next iteration
		if msg.ContinueLoop {
			m.infoMessage = fmt.Sprintf("Ralph loop iteration %d starting...", session.CurrentIteration+1)
			return m, func() tea.Msg {
				if err := coordinator.StartIteration(); err != nil {
					return tuimsg.RalphErrorMsg{Err: err, GroupID: msg.GroupID}
				}
				return tuimsg.RalphIterationStartedMsg{GroupID: msg.GroupID, Iteration: session.CurrentIteration}
			}
		}
	}

	return m, nil
}

// handleRalphError handles ralph loop errors.
func (m *Model) handleRalphError(msg tuimsg.RalphErrorMsg) (tea.Model, tea.Cmd) {
	m.errorMessage = fmt.Sprintf("Ralph loop error: %v", msg.Err)
	if m.logger != nil {
		m.logger.Error("ralph loop error", "error", msg.Err, "group_id", msg.GroupID)
	}
	return m, nil
}

// handleRalphIterationStarted handles the start of a new ralph iteration.
func (m *Model) handleRalphIterationStarted(msg tuimsg.RalphIterationStartedMsg) (tea.Model, tea.Cmd) {
	m.infoMessage = fmt.Sprintf("Ralph loop iteration %d started", msg.Iteration)
	if m.logger != nil {
		m.logger.Info("ralph iteration started", "iteration", msg.Iteration, "group_id", msg.GroupID)
	}
	return m, nil
}

// CleanupRalph stops all ralph coordinators and clears the ralph state.
func (m *Model) CleanupRalph() {
	if m.ralph == nil {
		return
	}

	// Stop all coordinators
	for _, coordinator := range m.ralph.Coordinators {
		if coordinator != nil {
			coordinator.Stop()
		}
	}

	// Clear TUI-level state
	m.ralph = nil

	// Clear session-level ralph state
	if m.session != nil {
		m.session.RalphSessions = nil
	}
}

// initiateRalphMode creates and starts a Ralph Wiggum loop session.
func (m Model) initiateRalphMode(prompt string, maxIterations int, completionPromise string) (Model, tea.Cmd) {
	// Validate required dependencies
	if m.session == nil {
		m.errorMessage = "Cannot start ralph loop: no active session"
		if m.logger != nil {
			m.logger.Error("initiateRalphMode called with nil session")
		}
		return m, nil
	}
	if m.orchestrator == nil {
		m.errorMessage = "Cannot start ralph loop: orchestrator not available"
		if m.logger != nil {
			m.logger.Error("initiateRalphMode called with nil orchestrator")
		}
		return m, nil
	}

	// Create a group for this ralph session
	ralphGroup := orchestrator.NewInstanceGroupWithType(
		fmt.Sprintf("Ralph: %s", util.TruncateString(prompt, 25)),
		orchestrator.SessionTypeRalph,
		prompt,
	)
	m.session.AddGroup(ralphGroup)

	// Request intelligent name generation for the group
	m.orchestrator.RequestGroupRename(ralphGroup.ID, prompt)

	// Create ralph session config
	config := orchestrator.DefaultRalphConfig()
	config.CompletionPromise = completionPromise
	if maxIterations > 0 {
		config.MaxIterations = maxIterations
	}

	// Create ralph session
	ralphSession := orchestrator.NewRalphSession(prompt, config)
	ralphSession.GroupID = ralphGroup.ID

	// Add to RalphSessions slice for persistence
	m.session.RalphSessions = append(m.session.RalphSessions, ralphSession)

	// Create coordinator
	coordinator := orchestrator.NewRalphCoordinator(m.orchestrator, m.session, ralphSession, m.logger)

	// Auto-enable grouped sidebar mode
	m.autoEnableGroupedMode()

	// Initialize ralph state if needed
	if m.ralph == nil {
		m.ralph = &RalphState{
			Coordinators: make(map[string]*orchestrator.RalphCoordinator),
		}
	} else if m.ralph.Coordinators == nil {
		m.ralph.Coordinators = make(map[string]*orchestrator.RalphCoordinator)
	}

	// Add coordinator to the map
	m.ralph.Coordinators[ralphGroup.ID] = coordinator

	m.infoMessage = fmt.Sprintf("Starting Ralph loop (max %d iterations, looking for '%s')...",
		config.MaxIterations, util.TruncateString(completionPromise, 20))

	// Start first iteration asynchronously
	return m, func() tea.Msg {
		if err := coordinator.StartIteration(); err != nil {
			return tuimsg.RalphErrorMsg{Err: err, GroupID: ralphGroup.ID}
		}
		return tuimsg.RalphIterationStartedMsg{GroupID: ralphGroup.ID, Iteration: 1}
	}
}

// GetRalphViewState converts the internal ralph state to a view-compatible state.
// Exported for use by view components.
func (m Model) GetRalphViewState() *view.RalphState {
	if m.ralph == nil {
		return nil
	}
	return &view.RalphState{
		Coordinators:      m.ralph.Coordinators,
		NeedsNotification: m.ralph.NeedsNotification,
	}
}

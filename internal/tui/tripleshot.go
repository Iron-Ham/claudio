package tui

import (
	"context"
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot/teamwire"
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

	// Skip polling when using teamwire (callbacks handle completion)
	if m.tripleShot.UseTeamwire {
		return nil
	}

	// Dispatch async check for each legacy coordinator
	for groupID, runner := range m.tripleShot.Runners {
		coordinator, ok := runner.(*tripleshot.Coordinator)
		if !ok {
			continue // Skip non-legacy runners
		}
		session := coordinator.Session()
		if session == nil {
			continue
		}

		// Only check if in a phase that requires polling
		switch session.Phase {
		case tripleshot.PhaseWorking, tripleshot.PhaseAdversarialReview, tripleshot.PhaseEvaluating:
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
	case tripleshot.PhaseWorking, tripleshot.PhaseAdversarialReview:
		return m.processAttemptCheckResults(coordinator, session, msg)

	case tripleshot.PhaseEvaluating:
		return m.processJudgeCheckResult(coordinator, msg)

	case tripleshot.PhaseFailed:
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
// During PhaseAdversarialReview, also processes reviewer completions.
func (m *Model) processAttemptCheckResults(
	coordinator *tripleshot.Coordinator,
	session *tripleshot.Session,
	msg tuimsg.TripleShotCheckResultMsg,
) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Dispatch async processing for each completed attempt (implementer completions)
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

	// During adversarial review phase, also process reviewer completions
	for i, complete := range msg.ReviewResults {
		if err, hasErr := msg.ReviewErrors[i]; hasErr && err != nil {
			if m.logger != nil {
				m.logger.Warn("failed to check review completion",
					"attempt_index", i,
					"error", err,
				)
			}
			continue
		}

		if complete {
			// Dispatch async command to process the review file
			cmds = append(cmds, tuimsg.ProcessAdversarialReviewCompletionAsync(coordinator, msg.GroupID, i))
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
	} else {
		m.infoMessage = "Attempt completed - checking progress..."
	}

	// Check if we should start the judge (even on error - attempt may have been marked failed)
	return m, m.maybeStartJudgeCmd(coordinator)
}

// processJudgeCheckResult handles completion check results for the judge.
// Returns an async command to process the judge completion file without blocking the UI.
func (m *Model) processJudgeCheckResult(
	coordinator *tripleshot.Coordinator,
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

// handleTripleShotReviewProcessed handles the result of async adversarial review processing.
func (m *Model) handleTripleShotReviewProcessed(msg tuimsg.TripleShotReviewProcessedMsg) (tea.Model, tea.Cmd) {
	if m.tripleShot == nil {
		return m, nil
	}

	coordinator := m.tripleShot.GetCoordinatorForGroup(msg.GroupID)
	if coordinator == nil {
		return m, nil
	}

	if msg.Err != nil {
		if m.logger != nil {
			m.logger.Error("failed to process adversarial review",
				"attempt_index", msg.AttemptIndex,
				"error", msg.Err,
			)
		}
		m.errorMessage = fmt.Sprintf("Failed to process review for attempt %d", msg.AttemptIndex+1)
	}

	// Check if we should start the judge (even on error - attempt may have been marked failed)
	return m, m.maybeStartJudgeCmd(coordinator)
}

// maybeStartJudgeCmd returns a command to start the judge if all attempts are complete
// and at least 2 succeeded. Returns nil if conditions are not met.
func (m *Model) maybeStartJudgeCmd(coordinator *tripleshot.Coordinator) tea.Cmd {
	session := coordinator.Session()
	if session == nil || !session.AllAttemptsComplete() || session.JudgeID != "" {
		return nil
	}
	if session.SuccessfulAttemptCount() < 2 {
		return nil
	}

	return func() tea.Msg {
		if err := coordinator.StartJudge(); err != nil {
			return tuimsg.TripleShotErrorMsg{Err: fmt.Errorf("failed to start judge: %w", err)}
		}
		implementersGroupID := coordinator.Session().ImplementersGroupID
		return tuimsg.TripleShotJudgeStartedMsg{
			ImplementersGroupID: implementersGroupID,
		}
	}
}

// handleTripleShotJudgeStopped handles cleanup when a triple-shot judge instance is stopped.
// This cleans up the entire triple-shot session associated with the judge.
func (m *Model) handleTripleShotJudgeStopped(judgeID string) {
	if m.tripleShot == nil {
		return
	}

	// Find the runner whose session has this judge
	var runnerToStop tripleshot.Runner
	var groupIDToRemove string
	for groupID, runner := range m.tripleShot.Runners {
		session := runner.Session()
		if session != nil && session.JudgeID == judgeID {
			runnerToStop = runner
			groupIDToRemove = groupID
			break
		}
	}

	if runnerToStop == nil {
		if m.logger != nil {
			m.logger.Warn("triple-shot judge stop requested but no matching runner found",
				"judge_id", judgeID,
				"runners_count", len(m.tripleShot.Runners),
			)
		}
		return
	}

	if m.logger != nil {
		m.logger.Info("cleaning up triple-shot session after judge stopped", "judge_id", judgeID)
	}

	// Stop the runner
	runnerToStop.Stop()

	// Remove from runners map
	if groupIDToRemove != "" {
		delete(m.tripleShot.Runners, groupIDToRemove)
	}

	// If no more coordinators, fully clean up tripleshot state
	if !m.tripleShot.HasActiveCoordinators() {
		m.cleanupTripleShot()
		m.infoMessage = "Triple-shot session ended"
	} else {
		m.infoMessage = "Triple-shot session ended (other sessions still active)"
	}
}

// initiateTeamwireTripleShot starts a triple-shot session using the Orch 2.0
// teamwire path. Callbacks from the TeamCoordinator are bridged into the
// Bubble Tea event loop via a buffered channel.
func (m Model) initiateTeamwireTripleShot(
	task string,
	group *orchestrator.InstanceGroup,
	tsConfig tripleshot.Config,
) (Model, tea.Cmd) {
	// Build adapters (avoids import cycle: orchestrator → teamwire)
	orchAdapter, sessAdapter := orchestrator.NewTripleShotAdapters(m.orchestrator, m.session)

	coordinator, err := teamwire.NewTeamCoordinator(teamwire.TeamCoordinatorConfig{
		Orchestrator: orchAdapter,
		BaseSession:  sessAdapter,
		Task:         task,
		Config:       tsConfig,
		Bus:          m.orchestrator.EventBus(),
		BaseDir:      m.session.BaseRepo,
		Logger:       m.logger,
	})
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to create teamwire coordinator: %v", err)
		if m.logger != nil {
			m.logger.Error("failed to create teamwire coordinator", "error", err)
		}
		return m, nil
	}

	// Link group ID to the tripleshot session before Start (no concurrency yet)
	coordinator.Session().GroupID = group.ID

	// Persist tripleshot session for potential restore
	m.session.TripleShots = append(m.session.TripleShots, coordinator.Session())

	// Create buffered channel for callback → Bubble Tea bridge
	eventCh := make(chan tea.Msg, 16)
	groupID := group.ID

	// Register callbacks that write to the event channel
	coordinator.SetCallbacks(&tripleshot.CoordinatorCallbacks{
		OnPhaseChange: func(phase tripleshot.Phase) {
			eventCh <- tuimsg.TeamwirePhaseChangedMsg{GroupID: groupID, Phase: phase}
		},
		OnAttemptStart: func(attemptIndex int, instanceID string) {
			eventCh <- tuimsg.TeamwireAttemptStartedMsg{
				GroupID: groupID, AttemptIndex: attemptIndex, InstanceID: instanceID,
			}
		},
		OnAttemptComplete: func(attemptIndex int) {
			eventCh <- tuimsg.TeamwireAttemptCompletedMsg{GroupID: groupID, AttemptIndex: attemptIndex}
		},
		OnAttemptFailed: func(attemptIndex int, reason string) {
			eventCh <- tuimsg.TeamwireAttemptFailedMsg{
				GroupID: groupID, AttemptIndex: attemptIndex, Reason: reason,
			}
		},
		OnJudgeStart: func(instanceID string) {
			eventCh <- tuimsg.TeamwireJudgeStartedMsg{GroupID: groupID, InstanceID: instanceID}
		},
		OnComplete: func(success bool, summary string) {
			eventCh <- tuimsg.TeamwireCompletedMsg{GroupID: groupID, Success: success, Summary: summary}
		},
	})

	// Store coordinator in runners map BEFORE starting so event handlers
	// can find it when callbacks fire during Start().
	if m.tripleShot == nil {
		m.tripleShot = &TripleShotState{
			Runners:     make(map[string]tripleshot.Runner),
			UseTeamwire: true,
		}
	} else if m.tripleShot.Runners == nil {
		m.tripleShot.Runners = make(map[string]tripleshot.Runner)
	}
	m.tripleShot.Runners[groupID] = coordinator
	m.tripleShot.UseTeamwire = true

	// Close any previous event channel to avoid goroutine leaks from the
	// old ListenTeamwireEvents reader still blocked on the orphaned channel.
	if ch := m.teamwireEventCh; ch != nil {
		close(ch)
	}
	m.teamwireEventCh = eventCh

	numActive := len(m.tripleShot.Runners)
	if numActive > 1 {
		m.infoMessage = fmt.Sprintf("Starting triple-shot #%d (teamwire)...", numActive)
	} else {
		m.infoMessage = "Starting triple-shot mode (teamwire)..."
	}

	if m.logger != nil {
		m.logger.Info("teamwire triple-shot starting", "group_id", groupID)
	}

	// Start coordinator asynchronously to avoid blocking the TUI event loop.
	// coordinator.Start() creates bridges that trigger claim loops, which
	// create git worktrees (I/O-heavy). Running it synchronously freezes the UI.
	startCmd := func() tea.Msg {
		if err := coordinator.Start(context.Background()); err != nil {
			return tuimsg.TeamwireStartResultMsg{GroupID: groupID, Err: err}
		}
		return tuimsg.TeamwireStartResultMsg{GroupID: groupID}
	}

	// Listen for callback events and start the coordinator concurrently.
	return m, tea.Batch(startCmd, tuimsg.ListenTeamwireEvents(eventCh))
}

// handleTeamwirePhaseChanged handles a phase change from the teamwire coordinator.
func (m *Model) handleTeamwirePhaseChanged(msg tuimsg.TeamwirePhaseChangedMsg) (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Info("teamwire phase changed", "group_id", msg.GroupID, "phase", msg.Phase)
	}

	switch msg.Phase {
	case tripleshot.PhaseWorking:
		m.infoMessage = "Triple-shot: attempts working..."
	case tripleshot.PhaseEvaluating:
		m.infoMessage = "All attempts complete - judge is evaluating solutions..."
	case tripleshot.PhaseFailed:
		errMsg := "Triple-shot failed"
		if m.tripleShot != nil {
			if runner := m.tripleShot.GetRunnerForGroup(msg.GroupID); runner != nil {
				if sess := runner.Session(); sess != nil && sess.Error != "" {
					errMsg = "Triple-shot failed: " + sess.Error
				}
			}
		}
		m.errorMessage = errMsg
	case tripleshot.PhaseComplete:
		// OnComplete callback handles this
	}

	return m, tuimsg.ListenTeamwireEvents(m.teamwireEventCh)
}

// handleTeamwireAttemptStarted handles an attempt start from the teamwire coordinator.
func (m *Model) handleTeamwireAttemptStarted(msg tuimsg.TeamwireAttemptStartedMsg) (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Info("teamwire attempt started",
			"group_id", msg.GroupID,
			"attempt_index", msg.AttemptIndex,
			"instance_id", msg.InstanceID,
		)
	}

	// Add the instance to the group for sidebar rendering
	if m.session != nil {
		group := m.session.GetGroup(msg.GroupID)
		if group != nil {
			group.AddInstance(msg.InstanceID)
		}
	}

	m.infoMessage = fmt.Sprintf("Attempt %d started", msg.AttemptIndex+1)
	return m, tuimsg.ListenTeamwireEvents(m.teamwireEventCh)
}

// handleTeamwireAttemptCompleted handles an attempt completion from the teamwire coordinator.
func (m *Model) handleTeamwireAttemptCompleted(msg tuimsg.TeamwireAttemptCompletedMsg) (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Info("teamwire attempt completed",
			"group_id", msg.GroupID,
			"attempt_index", msg.AttemptIndex,
		)
	}

	m.infoMessage = fmt.Sprintf("Attempt %d completed", msg.AttemptIndex+1)
	return m, tuimsg.ListenTeamwireEvents(m.teamwireEventCh)
}

// handleTeamwireAttemptFailed handles an attempt failure from the teamwire coordinator.
func (m *Model) handleTeamwireAttemptFailed(msg tuimsg.TeamwireAttemptFailedMsg) (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Warn("teamwire attempt failed",
			"group_id", msg.GroupID,
			"attempt_index", msg.AttemptIndex,
			"reason", msg.Reason,
		)
	}

	m.errorMessage = fmt.Sprintf("Attempt %d failed: %s", msg.AttemptIndex+1, msg.Reason)
	return m, tuimsg.ListenTeamwireEvents(m.teamwireEventCh)
}

// handleTeamwireJudgeStarted handles judge start from the teamwire coordinator.
func (m *Model) handleTeamwireJudgeStarted(msg tuimsg.TeamwireJudgeStartedMsg) (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Info("teamwire judge started",
			"group_id", msg.GroupID,
			"instance_id", msg.InstanceID,
		)
	}

	m.infoMessage = "All attempts complete - judge is evaluating solutions..."

	// Auto-collapse the implementers when the judge starts
	if m.tripleShot != nil {
		runner := m.tripleShot.GetRunnerForGroup(msg.GroupID)
		if runner != nil {
			session := runner.Session()
			if session != nil && session.ImplementersGroupID != "" {
				if m.groupViewState == nil {
					m.groupViewState = view.NewGroupViewState()
				}
				m.groupViewState.SetLockedCollapsed(session.ImplementersGroupID, true)
			}
		}
	}

	return m, tuimsg.ListenTeamwireEvents(m.teamwireEventCh)
}

// handleTeamwireCompleted handles completion of the teamwire triple-shot.
func (m *Model) handleTeamwireCompleted(msg tuimsg.TeamwireCompletedMsg) (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Info("teamwire triple-shot completed",
			"group_id", msg.GroupID,
			"success", msg.Success,
			"summary", msg.Summary,
		)
	}

	if msg.Success {
		taskPreview := msg.Summary
		if len(taskPreview) > 30 {
			taskPreview = taskPreview[:27] + "..."
		}
		m.infoMessage = fmt.Sprintf("Triple-shot complete! (%s)", taskPreview)
	} else {
		m.errorMessage = fmt.Sprintf("Triple-shot failed: %s", msg.Summary)
	}

	if m.tripleShot != nil {
		m.tripleShot.NeedsNotification = true
	}

	// Don't re-subscribe — the session is done. The channel will be closed
	// during cleanup. Returning nil avoids a goroutine leak from a reader
	// permanently blocked on a channel that will never receive another message.
	return m, nil
}

// handleTeamwireChannelClosed handles the teamwire event channel being closed.
func (m *Model) handleTeamwireChannelClosed() (tea.Model, tea.Cmd) {
	if m.logger != nil {
		m.logger.Info("teamwire event channel closed")
	}
	m.teamwireEventCh = nil
	// Don't re-subscribe — the channel is gone
	return m, nil
}

// handleTeamwireStartResult processes the result of the async coordinator start.
// On success this is a no-op (callbacks handle progress). On failure it cleans
// up the pre-registered coordinator state.
func (m *Model) handleTeamwireStartResult(msg tuimsg.TeamwireStartResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err == nil {
		if m.logger != nil {
			m.logger.Info("teamwire coordinator started", "group_id", msg.GroupID)
		}
		return m, nil
	}

	m.errorMessage = fmt.Sprintf("Failed to start teamwire coordinator: %v", msg.Err)
	if m.logger != nil {
		m.logger.Error("failed to start teamwire coordinator", "error", msg.Err)
	}

	// Remove the failed runner and clean up. Stop() is safe even if the
	// coordinator never fully started (it checks tc.started).
	if m.tripleShot != nil {
		if runner, ok := m.tripleShot.Runners[msg.GroupID]; ok {
			runner.Stop()
			delete(m.tripleShot.Runners, msg.GroupID)
		}
		if !m.tripleShot.HasActiveCoordinators() {
			m.cleanupTripleShot()
		}
	}

	return m, nil
}

// cleanupTripleShot stops all tripleshot runners and clears the tripleshot state.
func (m *Model) cleanupTripleShot() {
	if m.tripleShot == nil {
		return
	}

	// Stop all runners to cancel their contexts.
	// Stop() cancels the context, unsubscribes events, and waits for goroutines.
	for _, runner := range m.tripleShot.Runners {
		if runner != nil {
			runner.Stop()
		}
	}

	// Clear TUI-level state
	m.tripleShot = nil

	// Close the teamwire event channel if open. Nil-guard prevents double-close
	// panic if the channel was already closed (e.g., from error path in
	// initiateTeamwireTripleShot or from handleTeamwireChannelClosed).
	if ch := m.teamwireEventCh; ch != nil {
		m.teamwireEventCh = nil
		close(ch)
	}

	// Clear session-level tripleshot state
	if m.session != nil {
		m.session.TripleShots = nil
	}
}

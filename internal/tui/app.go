package tui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/command"
	"github.com/Iron-Ham/claudio/internal/tui/filter"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/panel"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/tui/terminal"
	"github.com/Iron-Ham/claudio/internal/tui/update"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
	"github.com/Iron-Ham/claudio/internal/util"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

// App wraps the Bubbletea program
type App struct {
	program      *tea.Program
	model        Model
	orchestrator *orchestrator.Orchestrator
	session      *orchestrator.Session
}

// New creates a new TUI application
func New(orch *orchestrator.Orchestrator, session *orchestrator.Session, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithUltraPlan creates a new TUI application in ultra-plan mode
func NewWithUltraPlan(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinator *orchestrator.Coordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)

	// Create a group for this ultraplan session if one doesn't exist.
	// Uses the shared helper from the ultraplan package to ensure consistent
	// group creation logic across all entry points (CLI, TUI inline, etc.)
	ultraSession := coordinator.Session()
	if ultraSession != nil && ultraSession.GroupID == "" {
		ultraplan.CreateAndLinkUltraPlanGroup(session, ultraSession, ultraSession.Config.MultiPass)

		// Auto-enable grouped sidebar mode
		model.autoEnableGroupedMode()
	}

	model.ultraPlan = &UltraPlanState{
		Coordinator:           coordinator,
		ShowPlanView:          false,
		LastAutoExpandedGroup: -1, // Sentinel value to trigger initial expansion
	}
	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithTripleShot creates a new TUI application in triple-shot mode
func NewWithTripleShot(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinator *tripleshot.Coordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	model.tripleShot = &TripleShotState{
		Coordinators: make(map[string]*tripleshot.Coordinator),
	}
	// Add the coordinator to the map keyed by its group ID for multiple tripleshot support
	if coordinator != nil {
		tripleSession := coordinator.Session()
		if tripleSession != nil {
			// Create a group if one doesn't exist (CLI-started tripleshots)
			if tripleSession.GroupID == "" {
				tripleGroup := orchestrator.NewInstanceGroupWithType(
					util.TruncateString(tripleSession.Task, 30),
					orchestrator.SessionTypeTripleShot,
					tripleSession.Task,
				)
				session.AddGroup(tripleGroup)
				tripleSession.GroupID = tripleGroup.ID

				// Auto-enable grouped sidebar mode
				model.autoEnableGroupedMode()
			}
			model.tripleShot.Coordinators[tripleSession.GroupID] = coordinator
		}
	}
	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithAdversarial creates a new TUI application in adversarial review mode
func NewWithAdversarial(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinator *adversarial.Coordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	model.adversarial = &view.AdversarialState{
		Coordinators: make(map[string]*adversarial.Coordinator),
	}
	// Add the coordinator to the map keyed by its group ID for multiple session support
	if coordinator != nil {
		advSession := coordinator.Session()
		if advSession != nil {
			// Create a group if one doesn't exist (CLI-started sessions)
			if advSession.GroupID == "" {
				advGroup := orchestrator.NewInstanceGroupWithType(
					util.TruncateString(advSession.Task, 30),
					orchestrator.SessionTypeAdversarial,
					advSession.Task,
				)
				session.AddGroup(advGroup)
				advSession.GroupID = advGroup.ID

				// Auto-enable grouped sidebar mode
				model.autoEnableGroupedMode()
			}
			model.adversarial.Coordinators[advSession.GroupID] = coordinator
		}
	}
	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithTripleShots creates a new TUI application with multiple tripleshot coordinators.
// This is used when restoring a session that had multiple concurrent tripleshots.
func NewWithTripleShots(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinators []*tripleshot.Coordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	model.tripleShot = &TripleShotState{
		Coordinators: make(map[string]*tripleshot.Coordinator),
	}

	// Add all coordinators to the map keyed by their group IDs
	createdGroup := false
	for _, coordinator := range coordinators {
		if coordinator != nil {
			tripleSession := coordinator.Session()
			if tripleSession != nil {
				// Create a group if one doesn't exist (legacy sessions)
				if tripleSession.GroupID == "" {
					tripleGroup := orchestrator.NewInstanceGroupWithType(
						util.TruncateString(tripleSession.Task, 30),
						orchestrator.SessionTypeTripleShot,
						tripleSession.Task,
					)
					session.AddGroup(tripleGroup)
					tripleSession.GroupID = tripleGroup.ID
					createdGroup = true
				}
				model.tripleShot.Coordinators[tripleSession.GroupID] = coordinator
			}
		}
	}

	// Auto-enable grouped sidebar mode if we created any groups
	if createdGroup {
		model.autoEnableGroupedMode()
	}

	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithAdversarials creates a new TUI application with multiple adversarial coordinators.
// This is used when restoring a session that had multiple concurrent adversarial sessions.
func NewWithAdversarials(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinators []*adversarial.Coordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	model.adversarial = &view.AdversarialState{
		Coordinators: make(map[string]*adversarial.Coordinator),
	}

	// Add all coordinators to the map keyed by their group IDs
	createdGroup := false
	for _, coordinator := range coordinators {
		if coordinator != nil {
			advSession := coordinator.Session()
			if advSession != nil {
				// Create a group if one doesn't exist (legacy sessions)
				if advSession.GroupID == "" {
					advGroup := orchestrator.NewInstanceGroupWithType(
						util.TruncateString(advSession.Task, 30),
						orchestrator.SessionTypeAdversarial,
						advSession.Task,
					)
					session.AddGroup(advGroup)
					advSession.GroupID = advGroup.ID
					createdGroup = true
				}
				model.adversarial.Coordinators[advSession.GroupID] = coordinator
			}
		}
	}

	// Auto-enable grouped sidebar mode if we created any groups
	if createdGroup {
		model.autoEnableGroupedMode()
	}

	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// Run starts the TUI application
func (a *App) Run() error {
	// Ensure session lock is released when TUI exits (both normal and signal-based)
	defer func() { _ = a.orchestrator.ReleaseLock() }()

	a.program = tea.NewProgram(
		a.model,
		tea.WithAltScreen(),
	)

	// Set up signal handling for graceful shutdown
	// This ensures session state is preserved when the process is terminated
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigChan
		// Send quit message to the TUI
		if a.program != nil {
			a.program.Send(tea.Quit())
		}
	}()

	// Subscribe to events from the event bus
	eventBus := a.orchestrator.EventBus()
	var subscriptionIDs []string

	// Subscribe to PR complete events
	subID := eventBus.Subscribe("pr.completed", func(e event.Event) {
		prEvent, ok := e.(event.PRCompleteEvent)
		if !ok {
			// Log unexpected event type for debugging
			return
		}
		a.program.Send(tuimsg.PRCompleteMsg{
			InstanceID: prEvent.InstanceID,
			Success:    prEvent.Success,
		})
	})
	subscriptionIDs = append(subscriptionIDs, subID)

	// Subscribe to PR opened events (inline PR creation detected in instance output)
	subID = eventBus.Subscribe("pr.opened", func(e event.Event) {
		prEvent, ok := e.(event.PROpenedEvent)
		if !ok {
			return
		}
		a.program.Send(tuimsg.PROpenedMsg{
			InstanceID: prEvent.InstanceID,
		})
	})
	subscriptionIDs = append(subscriptionIDs, subID)

	// Subscribe to timeout events
	subID = eventBus.Subscribe("instance.timeout", func(e event.Event) {
		timeoutEvent, ok := e.(event.TimeoutEvent)
		if !ok {
			return
		}
		// Convert event.TimeoutType to instance.TimeoutType
		var timeoutType instance.TimeoutType
		switch timeoutEvent.TimeoutType {
		case event.TimeoutActivity:
			timeoutType = instance.TimeoutActivity
		case event.TimeoutCompletion:
			timeoutType = instance.TimeoutCompletion
		case event.TimeoutStale:
			timeoutType = instance.TimeoutStale
		default:
			// Unknown timeout type - default to activity timeout
			timeoutType = instance.TimeoutActivity
		}
		a.program.Send(tuimsg.TimeoutMsg{
			InstanceID:  timeoutEvent.InstanceID,
			TimeoutType: timeoutType,
		})
	})
	subscriptionIDs = append(subscriptionIDs, subID)

	// Subscribe to bell events
	subID = eventBus.Subscribe("instance.bell", func(e event.Event) {
		bellEvent, ok := e.(event.BellEvent)
		if !ok {
			return
		}
		a.program.Send(tuimsg.BellMsg{InstanceID: bellEvent.InstanceID})
	})
	subscriptionIDs = append(subscriptionIDs, subID)

	_, err := a.program.Run()

	// Clean up signal handler
	signal.Stop(sigChan)

	// Unsubscribe only this component's event handlers
	for _, id := range subscriptionIDs {
		eventBus.Unsubscribe(id)
	}

	return err
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tuimsg.Tick()}

	// Schedule ultra-plan initialization if needed
	if m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
		session := m.ultraPlan.Coordinator.Session()
		if session != nil && session.Phase == orchestrator.PhasePlanning && session.CoordinatorID == "" {
			cmds = append(cmds, func() tea.Msg { return tuimsg.UltraPlanInitMsg{} })
		}
	}

	// Schedule adversarial initialization if needed
	if m.adversarial != nil && m.adversarial.HasActiveCoordinators() {
		for _, coordinator := range m.adversarial.Coordinators {
			session := coordinator.Session()
			// Start implementer if session is new (no implementer started yet)
			if session != nil && session.Phase == adversarial.PhaseImplementing && session.ImplementerID == "" {
				// Capture coordinator for closure
				coord := coordinator
				cmds = append(cmds, func() tea.Msg {
					if err := coord.StartImplementer(); err != nil {
						return tuimsg.AdversarialErrorMsg{Err: err}
					}
					return tuimsg.AdversarialStartedMsg{}
				})
			}
		}
	}

	return tea.Batch(cmds...)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeypress(msg)

	case tea.WindowSizeMsg:
		wasReady := m.ready
		m.terminalManager.SetSize(msg.Width, msg.Height)
		m.ready = true

		// Calculate the content area dimensions and resize tmux sessions
		// Use the configured sidebar width to ensure tmux panels match the UI layout
		cfg := config.Get()
		contentWidth, contentHeight := CalculateContentDimensionsWithSidebarWidth(
			m.terminalManager.Width(), m.terminalManager.Height(), cfg.TUI.SidebarWidth)
		if m.orchestrator != nil && contentWidth > 0 && contentHeight > 0 {
			m.orchestrator.ResizeAllInstances(contentWidth, contentHeight)
		}

		// Resize terminal pane if visible
		if m.terminalManager.IsVisible() {
			m.resizeTerminal()
		}

		// Ensure active instance is still visible after resize
		m.ensureActiveVisible()

		// On first ready (TUI just started), check if we should open plan editor
		// This handles the case when --plan FILE --review is provided
		if !wasReady && m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
			session := m.ultraPlan.Coordinator.Session()
			if session != nil && session.Phase == orchestrator.PhaseRefresh && session.Plan != nil && session.Config.Review {
				m.enterPlanEditor()
				m.infoMessage = fmt.Sprintf("Plan loaded: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
					len(session.Plan.Tasks), len(session.Plan.ExecutionOrder))
			}
		}

		// Force a full screen redraw after resize to prevent display artifacts
		// This helps with terminal emulators that don't properly clear stale content
		return m, tea.ClearScreen

	case tuimsg.TickMsg:
		// Update outputs from instances
		m.updateOutputs()
		// Update terminal pane output if visible
		if m.terminalManager.IsVisible() {
			m.updateTerminalOutput()
		}
		// Check for phase changes that need notification (synthesis, consolidation pause)
		m.checkForPhaseNotification()

		// Auto-dismiss info/error messages after timeout
		m.autoDismissMessages()

		// Build commands for this tick
		var cmds []tea.Cmd
		cmds = append(cmds, tuimsg.Tick())

		// Dispatch async commands to check tripleshot completion files
		// This avoids blocking the UI with file I/O
		cmds = append(cmds, m.dispatchTripleShotCompletionChecks()...)

		// Dispatch async commands to check adversarial completion files
		// This avoids blocking the UI with file I/O
		cmds = append(cmds, m.dispatchAdversarialCompletionChecks()...)

		// Dispatch async commands to check ralph loop completions
		// This avoids blocking the UI with file I/O
		cmds = append(cmds, m.dispatchRalphCompletionChecks()...)

		// Dispatch async commands to check ultraplan files
		// This avoids blocking the UI with file I/O during planning phases
		cmds = append(cmds, m.dispatchUltraPlanFileChecks()...)

		// Dispatch async commands to check inline multiplan files
		// This polls for plan file creation during :multiplan command
		cmds = append(cmds, m.dispatchInlineMultiPlanFileChecks()...)

		// Check if ultraplan needs user notification
		if m.ultraPlan != nil && m.ultraPlan.NeedsNotification {
			m.ultraPlan.NeedsNotification = false
			cmds = append(cmds, tuimsg.NotifyUser())
		}

		// Check if adversarial needs user notification
		if m.adversarial != nil && m.adversarial.NeedsNotification {
			m.adversarial.NeedsNotification = false
			cmds = append(cmds, tuimsg.NotifyUser())
		}

		// Check if ralph needs user notification
		if m.ralph != nil && m.ralph.NeedsNotification {
			m.ralph.NeedsNotification = false
			cmds = append(cmds, tuimsg.NotifyUser())
		}

		// Update ultraplan group collapse state when current group changes
		m.updateGroupCollapseState()

		return m, tea.Batch(cmds...)

	case tuimsg.UltraPlanInitMsg:
		// Initialize ultra-plan mode by starting the planning phase
		if m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
			session := m.ultraPlan.Coordinator.Session()
			if session != nil && session.Phase == orchestrator.PhasePlanning && session.CoordinatorID == "" {
				if err := m.ultraPlan.Coordinator.RunPlanning(); err != nil {
					m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
				} else {
					m.infoMessage = "Planning started. Claude is analyzing the codebase..."
					// Pause the old active instance before switching
					if oldInst := m.activeInstance(); oldInst != nil {
						m.pauseInstance(oldInst.ID)
					}
					// Select the coordinator instance so user can see the output
					for i, inst := range m.session.Instances {
						if inst.ID == session.CoordinatorID {
							m.activeTab = i
							break
						}
					}
					// Resume the new active instance's capture
					m.resumeActiveInstance()
				}
			}
		}
		return m, nil

	case tuimsg.OutputMsg:
		// Delegate to update handler
		update.HandleOutput(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.ErrMsg:
		// Delegate to update handler
		update.HandleError(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.PRCompleteMsg:
		// Delegate to update handler for PR workflow completion
		update.HandlePRComplete(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.PROpenedMsg:
		// Delegate to update handler for PR opened notification
		update.HandlePROpened(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.TimeoutMsg:
		// Delegate to update handler for instance timeout notification
		update.HandleTimeout(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.BellMsg:
		// Terminal bell detected in a tmux session - forward it to the parent terminal
		return m, tuimsg.RingBell()

	case tuimsg.TaskAddedMsg:
		// Delegate to update handler for async task addition
		update.HandleTaskAdded(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.DependentTaskAddedMsg:
		// Delegate to update handler for async dependent task addition
		update.HandleDependentTaskAdded(m.newUpdateContext(), msg)
		return m, nil

	case tuimsg.TripleShotStartedMsg:
		// Triple-shot attempts started successfully
		m.infoMessage = "Triple-shot started: 3 instances working on the task"
		if m.logger != nil {
			m.logger.Info("triple-shot attempts started")
		}
		return m, nil

	case tuimsg.TripleShotJudgeStartedMsg:
		// Judge started evaluating the attempts
		m.infoMessage = "All attempts complete - judge is evaluating solutions..."
		if m.logger != nil {
			m.logger.Info("triple-shot judge started")
		}
		return m, nil

	case tuimsg.TripleShotErrorMsg:
		// Triple-shot failed to start
		m.errorMessage = fmt.Sprintf("Failed to start triple-shot: %v", msg.Err)
		// Clean up triple-shot state on error
		m.cleanupTripleShot()
		if m.logger != nil {
			m.logger.Error("failed to start triple-shot", "error", msg.Err)
		}
		return m, nil

	case tuimsg.TripleShotCheckResultMsg:
		// Handle async completion check results
		return m.handleTripleShotCheckResult(msg)

	case tuimsg.TripleShotAttemptProcessedMsg:
		// Handle async attempt completion processing result
		return m.handleTripleShotAttemptProcessed(msg)

	case tuimsg.TripleShotJudgeProcessedMsg:
		// Handle async judge completion processing result
		return m.handleTripleShotJudgeProcessed(msg)

	case tuimsg.PlanFileCheckResultMsg:
		// Handle async plan file check result (single-pass mode)
		return m.handlePlanFileCheckResult(msg)

	case tuimsg.MultiPassPlanFileCheckResultMsg:
		// Handle async multi-pass plan file check result
		return m.handleMultiPassPlanFileCheckResult(msg)

	case tuimsg.PlanManagerFileCheckResultMsg:
		// Handle async plan manager file check result
		return m.handlePlanManagerFileCheckResult(msg)

	case tuimsg.InlineMultiPlanFileCheckResultMsg:
		// Handle async inline multiplan file check result
		return m.handleInlineMultiPlanFileCheckResult(msg)

	// Adversarial mode message handlers
	case tuimsg.AdversarialStartedMsg:
		m.infoMessage = "Adversarial review started"
		return m, nil

	case tuimsg.AdversarialErrorMsg:
		m.errorMessage = "Adversarial review error: " + msg.Err.Error()
		// Clean up adversarial state on error
		m.cleanupAdversarial()
		if m.logger != nil {
			m.logger.Error("adversarial review error", "error", msg.Err)
		}
		return m, nil

	case tuimsg.AdversarialCheckResultMsg:
		// Handle async completion check results
		return m.handleAdversarialCheckResult(msg)

	case tuimsg.AdversarialIncrementProcessedMsg:
		// Handle async increment file processing result
		return m.handleAdversarialIncrementProcessed(msg)

	case tuimsg.AdversarialReviewProcessedMsg:
		// Handle async review file processing result
		return m.handleAdversarialReviewProcessed(msg)

	case tuimsg.AdversarialRejectionAfterApprovalMsg:
		// Handle async rejection-after-approval processing result
		return m.handleAdversarialRejectionAfterApprovalProcessed(msg)

	// Ralph loop message handlers
	case tuimsg.RalphIterationStartedMsg:
		return m.handleRalphIterationStarted(msg)

	case tuimsg.RalphErrorMsg:
		return m.handleRalphError(msg)

	case tuimsg.RalphCompletionProcessedMsg:
		return m.handleRalphCompletionProcessed(msg)

	// Async instance operation message handlers
	case tuimsg.InstanceRemovedMsg:
		return m.handleInstanceRemoved(msg)

	case tuimsg.DiffLoadedMsg:
		return m.handleDiffLoaded(msg)
	}

	return m, nil
}

// executeCommand parses and executes a vim-style command
func (m Model) executeCommand(cmd string) (tea.Model, tea.Cmd) {
	// Clear messages before executing
	m.infoMessage = ""
	m.errorMessage = ""

	// Delegate to command handler
	result := m.commandHandler.Execute(cmd, &m)

	// Apply result to model state
	m.applyCommandResult(result)

	return m, result.TeaCmd
}

// applyCommandResult applies the result of a command execution to the model state.
// This method modifies the model based on the Result struct returned by the handler.
func (m *Model) applyCommandResult(result command.Result) {
	// Apply messages
	if result.InfoMessage != "" {
		m.infoMessage = result.InfoMessage
	}
	if result.ErrorMessage != "" {
		m.errorMessage = result.ErrorMessage
	}

	// Apply state changes (only if pointer is non-nil, meaning the value was set)
	if result.ShowHelp != nil {
		// Toggle help (handler sets to true, we toggle)
		m.showHelp = !m.showHelp
		if !m.showHelp {
			m.helpScroll = 0
		}
	}
	if result.ShowStats != nil {
		// Toggle stats
		m.showStats = !m.showStats
	}
	if result.ShowDiff != nil {
		m.showDiff = *result.ShowDiff
	}
	if result.DiffContent != nil {
		m.diffContent = *result.DiffContent
	}
	if result.DiffScroll != nil {
		m.diffScroll = *result.DiffScroll
	}
	if result.ShowConflicts != nil {
		// Toggle conflicts
		m.showConflicts = !m.showConflicts
	}
	if result.Quitting != nil {
		m.quitting = *result.Quitting
		// Cleanup terminal pane if running
		m.cleanupTerminal()
	}
	if result.AddingTask != nil {
		m.addingTask = *result.AddingTask
		m.taskInput = ""
		m.taskInputCursor = 0
	}
	if result.AddingDependentTask != nil && result.DependentOnInstanceID != nil {
		m.addingDependentTask = *result.AddingDependentTask
		m.dependentOnInstanceID = *result.DependentOnInstanceID
		m.addingTask = true // Reuse the task input UI
		m.taskInput = ""
		m.taskInputCursor = 0
	}
	if result.FilterMode != nil {
		m.filterMode = *result.FilterMode
	}

	// Handle terminal-related state changes
	if result.EnterTerminalMode {
		m.enterTerminalMode()
	}
	if result.ToggleTerminal {
		sessionID := ""
		if m.orchestrator != nil {
			sessionID = m.orchestrator.SessionID()
		}
		m.toggleTerminalVisibility(sessionID)
		if m.terminalManager.IsVisible() {
			m.infoMessage = "Terminal pane opened. Press [:t] to focus, [`] to hide."
		} else {
			m.infoMessage = "Terminal pane closed."
		}
	}
	if result.TerminalDirMode != nil {
		newMode := terminal.DirMode(*result.TerminalDirMode)
		currentMode := m.terminalManager.DirMode()
		if newMode != currentMode {
			m.terminalManager.SetDirMode(newMode)
			process := m.terminalManager.Process()
			if process != nil && process.IsRunning() {
				targetDir := m.getTerminalDir()
				if err := process.ChangeDirectory(targetDir); err != nil {
					m.errorMessage = "Failed to change directory: " + err.Error()
				} else {
					if newMode == terminal.DirWorktree {
						m.infoMessage = "Terminal: switched to worktree"
					} else {
						m.infoMessage = "Terminal: switched to invocation directory"
					}
				}
			} else {
				if newMode == terminal.DirWorktree {
					m.infoMessage = "Terminal will use worktree when opened."
				} else {
					m.infoMessage = "Terminal will use invocation directory when opened."
				}
			}
		} else {
			// Already in the requested mode
			if newMode == terminal.DirWorktree {
				m.infoMessage = "Terminal is already in worktree mode."
			} else {
				m.infoMessage = "Terminal is already in invocation directory mode."
			}
		}
	}

	// Handle active tab adjustment after instance removal
	if result.ActiveTabAdjustment != 0 {
		if m.activeTab >= m.instanceCount() {
			m.activeTab = m.instanceCount() - 1
			if m.activeTab < 0 {
				m.activeTab = 0
			}
		}
		// Resume the new active instance's capture (it may have been paused)
		m.resumeActiveInstance()
	}
	if result.EnsureActiveVisible {
		m.ensureActiveVisible()
	}

	// Handle triple-shot mode transition
	if result.StartTripleShot != nil && *result.StartTripleShot {
		m.startingTripleShot = true
		m.addingTask = true
		m.taskInput = ""
		m.taskInputCursor = 0
	}

	// Handle adversarial mode transition
	if result.StartAdversarial != nil && *result.StartAdversarial {
		m.startingAdversarial = true
		m.addingTask = true
		m.taskInput = ""
		m.taskInputCursor = 0
	}

	// Handle ralph loop start
	if result.StartRalphLoop != nil && *result.StartRalphLoop {
		// If we have prompt and completion promise, start immediately
		if result.RalphPrompt != nil && result.RalphCompletionPromise != nil {
			maxIter := 50 // Default
			if result.RalphMaxIterations != nil {
				maxIter = *result.RalphMaxIterations
			}
			*m, _ = m.initiateRalphMode(*result.RalphPrompt, maxIter, *result.RalphCompletionPromise)
		}
		// Otherwise, the model will show a prompt for user input (not implemented yet)
	}

	// Handle ralph loop cancel
	if result.CancelRalphLoop != nil && *result.CancelRalphLoop {
		if m.ralph != nil {
			for _, coordinator := range m.ralph.Coordinators {
				if coordinator != nil {
					coordinator.Cancel()
				}
			}
			m.infoMessage = "All ralph loops cancelled"
		}
	}

	// Handle triple-shot judge stopped - clean up the entire triple-shot session
	if result.StoppedTripleShotJudgeID != nil {
		m.handleTripleShotJudgeStopped(*result.StoppedTripleShotJudgeID)
	}

	// Handle inline plan mode transition
	if result.StartPlanMode != nil && *result.StartPlanMode {
		m.initInlinePlanMode()
	}

	// Handle inline multi-pass plan mode transition
	if result.StartMultiPlanMode != nil && *result.StartMultiPlanMode {
		m.initInlineMultiPlanMode()
	}

	// Handle inline ultraplan mode transition
	if result.StartUltraPlanMode != nil && *result.StartUltraPlanMode {
		m.initInlineUltraPlanMode(result)
	}

	// Handle grouped view toggle
	if result.ToggleGroupedView != nil && *result.ToggleGroupedView {
		m.toggleGroupedView()
	}
}

// shouldShowLine determines if a line should be shown based on filters
func (m *Model) shouldShowLine(line string) bool {
	// Custom filter takes precedence
	if m.filterRegex != nil {
		return m.filterRegex.MatchString(line)
	}

	lineLower := strings.ToLower(line)

	// Check category filters
	if !m.filterCategories["errors"] {
		if strings.Contains(lineLower, "error") || strings.Contains(lineLower, "failed") ||
			strings.Contains(lineLower, "exception") || strings.Contains(lineLower, "panic") {
			return false
		}
	}

	if !m.filterCategories["warnings"] {
		if strings.Contains(lineLower, "warning") || strings.Contains(lineLower, "warn") {
			return false
		}
	}

	if !m.filterCategories["tools"] {
		// Common Claude tool call patterns
		if strings.Contains(lineLower, "read file") || strings.Contains(lineLower, "write file") ||
			strings.Contains(lineLower, "bash") || strings.Contains(lineLower, "running") ||
			strings.HasPrefix(line, "  ") && (strings.Contains(line, "(") || strings.Contains(line, "→")) {
			return false
		}
	}

	if !m.filterCategories["thinking"] {
		if strings.Contains(lineLower, "thinking") || strings.Contains(lineLower, "let me") ||
			strings.Contains(lineLower, "i'll") || strings.Contains(lineLower, "i will") {
			return false
		}
	}

	if !m.filterCategories["progress"] {
		if strings.Contains(line, "...") || strings.Contains(line, "✓") ||
			strings.Contains(line, "█") || strings.Contains(line, "░") {
			return false
		}
	}

	return true
}

// updateOutputs fetches latest output from all instances, updates their status, and checks for conflicts
func (m *Model) updateOutputs() {
	if m.session == nil {
		return
	}

	for _, inst := range m.session.Instances {
		// Check for PR workflow first (when instance is in PR creation state)
		if inst.Status == orchestrator.StatusCreatingPR {
			workflow := m.orchestrator.GetPRWorkflow(inst.ID)
			if workflow != nil {
				output := workflow.GetOutput()
				if len(output) > 0 {
					m.outputManager.SetOutput(inst.ID, string(output))
				}
			}
			continue
		}

		mgr := m.orchestrator.GetInstanceManager(inst.ID)
		if mgr != nil {
			output := mgr.GetOutput()
			if len(output) > 0 {
				m.outputManager.SetOutput(inst.ID, string(output))
				// Update scroll position (auto-scroll if enabled)
				m.updateOutputScroll(inst.ID)
			}

			// Update instance status based on detected waiting state
			// Check when working OR waiting for input (to detect completion after waiting)
			if inst.Status == orchestrator.StatusWorking || inst.Status == orchestrator.StatusWaitingInput {
				m.updateInstanceStatus(inst, mgr)
			}
		}
	}

	// Check for file conflicts
	detector := m.orchestrator.GetConflictDetector()
	if detector != nil {
		m.conflicts = detector.GetConflicts()
	}
}

// updateInstanceStatus updates an instance's status based on detected waiting state
func (m *Model) updateInstanceStatus(inst *orchestrator.Instance, mgr *instance.Manager) {
	state := mgr.CurrentState()
	previousStatus := inst.Status

	switch state {
	case detect.StateWaitingPermission, detect.StateWaitingQuestion, detect.StateWaitingInput:
		inst.Status = orchestrator.StatusWaitingInput
	case detect.StateCompleted:
		inst.Status = orchestrator.StatusCompleted
		// If just completed (status changed), check completion action
		if previousStatus != orchestrator.StatusCompleted {
			m.handleInstanceCompleted(inst)
		}
	case detect.StateError:
		inst.Status = orchestrator.StatusError
	case detect.StateWorking:
		// If currently marked as waiting but now working, go back to working
		if inst.Status == orchestrator.StatusWaitingInput {
			inst.Status = orchestrator.StatusWorking
		}
	}
}

// handleInstanceCompleted handles post-completion actions based on config
func (m *Model) handleInstanceCompleted(inst *orchestrator.Instance) {
	// Check if this is an ultra-plan coordinator instance completing
	if m.handleUltraPlanCoordinatorCompletion(inst) {
		return
	}

	// Check if this is an inline multiplan instance completing
	if m.handleInlineMultiPlanCompletion(inst) {
		return
	}

	cfg := config.Get()

	switch cfg.Completion.DefaultAction {
	case "auto_pr":
		// Prompt user to create PR (we can't run it in TUI without freezing)
		m.infoMessage = fmt.Sprintf("Instance %s completed! Create PR: claudio pr %s", inst.ID, inst.ID)
	case "prompt":
		// Show a generic completion message
		m.infoMessage = fmt.Sprintf("Instance %s completed. Press [r] for PR options.", inst.ID)
	default:
		// For other actions (keep_branch, merge_staging, merge_main), just note completion
		m.infoMessage = fmt.Sprintf("Instance %s completed.", inst.ID)
	}
}

// messageDismissTimeout is how long info/error messages stay visible before auto-dismissing
const messageDismissTimeout = 5 * time.Second

// autoDismissMessages clears info/error messages after they've been displayed for a while.
// This prevents stale warnings from cluttering the UI indefinitely.
func (m *Model) autoDismissMessages() {
	// Build current message key to detect changes
	currentKey := m.errorMessage + "|" + m.infoMessage

	// If message changed, record the new timestamp
	if currentKey != m.lastMessageKey {
		m.lastMessageKey = currentKey
		if currentKey != "|" { // Only set timestamp if there's actually a message
			m.messageSetAt = time.Now()
		}
		return
	}

	// If there's a message and it's been displayed long enough, clear it
	if currentKey != "|" && !m.messageSetAt.IsZero() {
		if time.Since(m.messageSetAt) > messageDismissTimeout {
			m.errorMessage = ""
			m.infoMessage = ""
			m.lastMessageKey = "|"
		}
	}
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	// Header - unified header shows all active workflows
	header := m.renderUnifiedHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Get pane dimensions, accounting for dynamic footer elements
	dims := m.terminalManager.GetPaneDimensions(m.calculateExtraFooterLines())
	cfg := config.Get()
	effectiveSidebarWidth := CalculateEffectiveSidebarWidthWithConfig(dims.TerminalWidth, cfg.TUI.SidebarWidth)
	mainContentWidth := dims.TerminalWidth - effectiveSidebarWidth - 3 // 3 for gap between panels

	// Main area height is pre-calculated by terminal manager
	// (accounts for header, footer, and terminal pane)
	mainAreaHeight := dims.MainAreaHeight

	// Sidebar + Content area (horizontal layout)
	// Use view component for sidebar rendering - handles all modes including ultraplan
	// The SidebarView automatically handles flat, grouped, and ultraplan modes by
	// rendering ultraplan content inline within its group when expanded
	var sidebar, content string
	if m.IsTripleShotMode() {
		sidebar = m.renderTripleShotSidebar(effectiveSidebarWidth, mainAreaHeight)
		content = m.renderContent(mainContentWidth) // Reuse normal content for now
	} else {
		sidebarView := view.NewSidebarView()
		sidebar = sidebarView.RenderSidebar(m, effectiveSidebarWidth, mainAreaHeight)
		// Use ultraplan content renderer when in ultraplan mode
		if m.IsUltraPlanMode() {
			content = m.renderUltraPlanContent(mainContentWidth)
		} else {
			content = m.renderContent(mainContentWidth)
		}
	}

	// Apply height constraints to both panels and join horizontally
	// Using MaxHeight to ensure content doesn't overflow bounds
	sidebarStyled := lipgloss.NewStyle().
		Width(effectiveSidebarWidth).
		Height(mainAreaHeight).
		MaxHeight(mainAreaHeight).
		Render(sidebar)

	contentStyled := lipgloss.NewStyle().
		Width(mainContentWidth).
		Height(mainAreaHeight).
		MaxHeight(mainAreaHeight).
		Render(content)

	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebarStyled, " ", contentStyled)
	b.WriteString(mainArea)

	// Terminal pane (if visible)
	if m.terminalManager.IsVisible() {
		b.WriteString("\n")
		b.WriteString(m.renderTerminalPane())
	}

	// Conflict warning banner (always show if conflicts exist)
	if len(m.conflicts) > 0 {
		b.WriteString("\n")
		b.WriteString(m.renderConflictWarning())
	}

	// Info or error message if any
	if m.infoMessage != "" {
		b.WriteString("\n")
		b.WriteString(styles.Secondary.Render("ℹ " + m.infoMessage))
	} else if m.errorMessage != "" {
		b.WriteString("\n")
		b.WriteString(styles.ErrorMsg.Render("Error: " + m.errorMessage))
	}

	// Help/status bar - use appropriate help based on mode
	// Command mode takes priority over all other modes to show the : prompt
	b.WriteString("\n")
	if m.commandMode {
		b.WriteString(m.renderCommandModeHelp())
	} else if m.IsPlanEditorActive() {
		b.WriteString(m.renderPlanEditorHelp())
	} else if m.IsUltraPlanMode() {
		b.WriteString(m.renderUltraPlanHelp())
	} else if m.IsTripleShotMode() {
		b.WriteString(m.renderTripleShotHelp())
	} else if m.IsAdversarialMode() {
		b.WriteString(m.renderAdversarialHelp())
	} else {
		b.WriteString(m.renderHelp())
	}

	return b.String()
}

// renderUnifiedHeader renders the header with unified workflow status.
// This shows all active workflows (ultraplan, tripleshot, adversarial) simultaneously,
// solving the problem where only one workflow type was visible at a time.
func (m Model) renderUnifiedHeader() string {
	// Build workflow status state from all active workflows
	workflowState := m.buildWorkflowStatusState()

	// Build title - include ultraplan objective if ultraplan is active
	title := "Claudio"
	if objective := workflowState.GetUltraPlanObjective(); objective != "" {
		// Truncate objective to fit header
		if len(objective) > 40 {
			objective = objective[:37] + "..."
		}
		title = fmt.Sprintf("Claudio Ultra-Plan: %s", objective)
	} else if m.session != nil && m.session.Name != "" {
		title = fmt.Sprintf("Claudio: %s", m.session.Name)
	}

	// Build mode indicator state
	modeState := &view.ModeIndicatorState{
		CommandMode:     m.commandMode,
		SearchMode:      m.searchMode,
		FilterMode:      m.filterMode,
		InputMode:       m.inputMode,
		TerminalFocused: m.terminalManager.IsFocused(),
		AddingTask:      m.addingTask,
	}

	// Get the workflow status and mode indicator
	workflowStatus := view.RenderWorkflowStatus(workflowState)
	modeIndicator := view.RenderModeIndicator(modeState)

	// Calculate available width for layout
	termWidth := m.terminalManager.Width()

	// If no workflow status and no mode indicator, render simple header
	if workflowStatus == "" && modeIndicator == "" {
		return styles.Header.Width(termWidth).Render(title)
	}

	// Build the right side content (workflow status + mode indicator)
	var rightContent string
	if workflowStatus != "" && modeIndicator != "" {
		rightContent = workflowStatus + "  " + modeIndicator
	} else if workflowStatus != "" {
		rightContent = workflowStatus
	} else {
		rightContent = modeIndicator
	}

	// Calculate widths for left-right layout
	rightWidth := lipgloss.Width(rightContent)
	titleWidth := termWidth - rightWidth - 2 // 2 for spacing

	// Style for title (left side)
	titleStyled := lipgloss.NewStyle().
		Bold(true).
		Foreground(styles.PrimaryColor).
		Width(titleWidth).
		Render(title)

	// Join title and right content
	content := lipgloss.JoinHorizontal(lipgloss.Center, titleStyled, " ", rightContent)

	// Apply the header border styling
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(styles.BorderColor).
		MarginBottom(1).
		PaddingBottom(1).
		Width(termWidth).
		Render(content)
}

// buildWorkflowStatusState builds the workflow status state from the model.
func (m Model) buildWorkflowStatusState() *view.WorkflowStatusState {
	return &view.WorkflowStatusState{
		UltraPlan:   m.ultraPlan,
		TripleShot:  m.tripleShot,
		Adversarial: m.adversarial,
	}
}

// renderTerminalPane renders the terminal pane at the bottom of the screen.
func (m Model) renderTerminalPane() string {
	dims := m.terminalManager.GetPaneDimensions(m.calculateExtraFooterLines())
	if dims.TerminalPaneHeight == 0 {
		return ""
	}

	// Build the terminal state for the view
	process := m.terminalManager.Process()
	state := view.TerminalState{
		Output:         m.terminalManager.Output(),
		TerminalMode:   m.terminalManager.IsFocused(),
		InvocationDir:  m.terminalManager.GetDir(nil), // Pass nil to get invocation dir
		IsWorktreeMode: m.terminalManager.DirMode() == terminal.DirWorktree,
	}

	// Set current directory
	if process != nil {
		state.CurrentDir = process.CurrentDir()
	} else {
		state.CurrentDir = state.InvocationDir
	}

	// Set instance ID if in worktree mode
	if state.IsWorktreeMode {
		if inst := m.activeInstance(); inst != nil {
			state.InstanceID = inst.ID
		}
	}

	termView := view.NewTerminalView(dims.TerminalWidth, dims.TerminalPaneHeight)
	return termView.Render(state)
}

// renderContent renders the main content area
func (m Model) renderContent(width int) string {
	if m.addingTask {
		return m.renderAddTask(width)
	}

	if m.showHelp {
		return m.renderHelpPanel(width)
	}

	if m.showDiff {
		return m.renderDiffPanel(width)
	}

	if m.showConflicts && len(m.conflicts) > 0 {
		return m.renderConflictPanel(width)
	}

	if m.showStats {
		return m.renderStatsPanel(width)
	}

	if m.filterMode {
		return m.renderFilterPanel(width)
	}

	inst := m.activeInstance()
	if inst == nil {
		return styles.ContentBox.Width(width - 4).Render(
			"No instance selected.\n\nPress [:a] to add a new Claude instance.",
		)
	}

	return m.renderInstance(inst, width)
}

// renderInstance renders the active instance view
func (m Model) renderInstance(inst *orchestrator.Instance, width int) string {
	// Build render state for the view component
	mgr := m.orchestrator.GetInstanceManager(inst.ID)
	isRunning := mgr != nil && mgr.Running()

	// Apply filters to output
	output := m.outputManager.GetOutput(inst.ID)
	if output != "" {
		output = m.filterOutput(output)
	}

	renderState := view.RenderState{
		Output:            output,
		IsRunning:         isRunning,
		InputMode:         m.inputMode,
		ScrollOffset:      m.outputManager.GetScrollOffset(inst.ID),
		AutoScrollEnabled: m.isOutputAutoScroll(inst.ID),
		HasNewOutput:      m.hasNewOutput(inst.ID),
		SearchPattern:     m.searchInput,
		SearchRegex:       m.searchEngine.Regex(),
		SearchMatches:     m.searchEngine.MatchingLines(),
		SearchCurrent:     m.searchEngine.CurrentIndex(),
		SearchMode:        m.searchMode,
	}

	instanceView := view.NewInstanceView(width, m.getOutputMaxLines())
	return instanceView.RenderWithSession(inst, renderState, m.session)
}

// renderFilterPanel renders the filter configuration panel.
// Delegates to filter.RenderPanel for rendering using the filter package.
func (m Model) renderFilterPanel(width int) string {
	// Build a Filter from the model's current filter state
	f := filter.NewWithCategories(m.filterCategories)
	f.SetCustomPattern(m.filterCustom)
	return filter.RenderPanel(f, width)
}

// renderAddTask renders the add task input
func (m Model) renderAddTask(width int) string {
	inputState := &view.InputState{
		Text:                 m.taskInput,
		Cursor:               m.taskInputCursor,
		ShowTemplates:        m.showTemplates,
		Templates:            m.buildTemplateItems(),
		TemplateSelected:     m.templateSelected,
		ShowBranchSelector:   m.showBranchSelector,
		Branches:             m.buildBranchItems(),
		BranchSelected:       m.branchSelected,
		BranchScrollOffset:   m.branchScrollOffset,
		BranchSearchInput:    m.branchSearchInput,
		SelectedBranch:       m.selectedBaseBranch,
		BranchSelectorHeight: m.branchSelectorHeight,
	}

	// Customize title/subtitle based on the mode
	switch {
	case m.startingTripleShot:
		inputState.Title = "Triple-Shot"
		inputState.Subtitle = "Three instances will compete to solve your task:"
	case m.startingAdversarial:
		inputState.Title = "Adversarial Review"
		inputState.Subtitle = "Implementer and reviewer iterate until approved:"
	case m.addingDependentTask && m.dependentOnInstanceID != "":
		inputState.Title = "Chain Task"
		// Find the parent task name for context
		parentTask := m.dependentOnInstanceID
		for _, inst := range m.session.Instances {
			if inst.ID == m.dependentOnInstanceID {
				parentTask = inst.Task
				if len(parentTask) > 50 {
					parentTask = parentTask[:50] + "..."
				}
				break
			}
		}
		inputState.Subtitle = "This task will auto-start when \"" + parentTask + "\" completes:"
	default:
		inputState.Title = "New Task"
	}

	inputView := view.NewInputView()
	return inputView.Render(inputState, width)
}

// buildTemplateItems converts filtered templates to view template items.
// Uses view.BuildTemplateItems for the conversion logic.
func (m Model) buildTemplateItems() []view.TemplateItem {
	templates := FilterTemplates(m.templateFilter)
	viewTemplates := make([]view.Template, len(templates))
	for i, t := range templates {
		viewTemplates[i] = view.Template{
			Command:     t.Command,
			Name:        t.Name,
			Description: t.Description,
			Suffix:      t.Suffix,
		}
	}
	return view.BuildTemplateItems(viewTemplates)
}

// buildBranchItems converts the filtered branch list to view branch items
func (m Model) buildBranchItems() []view.BranchItem {
	// Use filtered list if available, otherwise full list
	branchList := m.branchFiltered
	if len(branchList) == 0 && len(m.branchList) > 0 && m.branchSearchInput == "" {
		branchList = m.branchList
	}
	if len(branchList) == 0 {
		return nil
	}

	// Get main branch name from orchestrator
	mainBranch := m.orchestrator.GetMainBranch()

	items := make([]view.BranchItem, len(branchList))
	for i, name := range branchList {
		items[i] = view.BranchItem{
			Name:   name,
			IsMain: name == mainBranch,
		}
	}
	return items
}

// renderHelpPanel renders the help overlay using the panel package.
// Help content is sourced from panel.DefaultHelpSections() for single source of truth.
func (m Model) renderHelpPanel(width int) string {
	helpPanel := panel.NewHelpPanel()
	state := &panel.RenderState{
		Width:        width - 4, // Account for content box padding
		Height:       m.terminalManager.Height() - 4,
		ScrollOffset: m.helpScroll,
		Theme:        styles.NewTheme(),
	}

	content := helpPanel.Render(state)
	return styles.ContentBox.Width(width - 4).Render(content)
}

// renderDiffPanel renders the diff preview panel using the panel package.
// Diff rendering and syntax highlighting are sourced from panel.DiffPanel
// for single source of truth.
func (m Model) renderDiffPanel(width int) string {
	diffPanel := panel.NewDiffPanel()
	state := &panel.RenderState{
		Width:          width - 4, // Account for content box padding
		Height:         m.terminalManager.Height() - 4,
		ScrollOffset:   m.diffScroll,
		Theme:          styles.NewTheme(),
		ActiveInstance: m.activeInstance(),
		DiffContent:    m.diffContent,
	}

	content := diffPanel.Render(state)
	return styles.ContentBox.Width(width - 4).Render(content)
}

// calculateExtraFooterLines returns the number of extra lines needed in the footer
// beyond the base help bar. This accounts for conflict warnings and error/info messages.
func (m Model) calculateExtraFooterLines() int {
	extra := 0

	// Conflict warning adds 1 line when present
	if len(m.conflicts) > 0 {
		extra++
	}

	// Error or info message adds 1 line when present (they are mutually exclusive)
	if m.errorMessage != "" || m.infoMessage != "" {
		extra++
	}

	// Verbose command help adds 2 extra lines (3 total vs 1 base)
	if m.commandMode && viper.GetBool("tui.verbose_command_help") {
		extra += 2
	}

	return extra
}

// renderConflictWarning renders the file conflict warning banner
func (m Model) renderConflictWarning() string {
	conflictsView := view.NewConflictsView(m.conflicts, m.buildInstanceInfoList())
	return conflictsView.RenderWarningBanner()
}

// renderConflictPanel renders a detailed conflict view showing all files and instances
func (m Model) renderConflictPanel(width int) string {
	conflictsView := view.NewConflictsView(m.conflicts, m.buildInstanceInfoList())
	return conflictsView.Render(width)
}

// buildInstanceInfoList builds a list of instance info for view components
func (m Model) buildInstanceInfoList() []view.InstanceInfo {
	if m.session == nil {
		return nil
	}
	instances := make([]view.InstanceInfo, len(m.session.Instances))
	for i, inst := range m.session.Instances {
		instances[i] = view.InstanceInfo{
			ID:   inst.ID,
			Task: inst.Task,
		}
	}
	return instances
}

// buildHelpBarState creates the view.HelpBarState from the current model state.
func (m Model) buildHelpBarState() *view.HelpBarState {
	state := &view.HelpBarState{
		CommandMode:   m.commandMode,
		CommandBuffer: m.commandBuffer,
		InputMode:     m.inputMode,
		ShowDiff:      m.showDiff,
		FilterMode:    m.filterMode,
		SearchMode:    m.searchMode,
		ConflictCount: len(m.conflicts),
	}

	// Terminal manager may be nil in tests
	if m.terminalManager != nil {
		state.TerminalFocused = m.terminalManager.IsFocused()
		state.TerminalVisible = m.terminalManager.IsVisible()
		if m.terminalManager.DirMode() == terminal.DirWorktree {
			state.TerminalDirMode = "worktree"
		} else {
			state.TerminalDirMode = "invoke"
		}
	}

	// Search engine may be nil in tests
	if m.searchEngine != nil {
		state.SearchHasMatches = m.searchEngine.HasMatches()
		state.SearchCurrentIndex = m.searchEngine.CurrentIndex()
		state.SearchMatchCount = m.searchEngine.MatchCount()
	}

	return state
}

// renderCommandModeHelp renders the help bar when in command mode.
// This is separate so it can take priority in all modes (normal, ultra-plan, plan editor).
// Delegates to view.RenderCommandModeHelp for the actual rendering.
func (m Model) renderCommandModeHelp() string {
	return view.RenderCommandModeHelp(m.buildHelpBarState())
}

// renderHelp renders the help bar.
// Delegates to view.RenderHelp for the actual rendering.
func (m Model) renderHelp() string {
	return view.RenderHelp(m.buildHelpBarState())
}

// renderStatsPanel renders the session statistics/metrics panel.
// Delegates to the panel package's StatsPanel for rendering.
func (m Model) renderStatsPanel(width int) string {
	cfg := config.Get()

	// Build render state for the panel
	state := &panel.RenderState{
		Width:                width,
		Height:               m.terminalManager.Height(),
		Theme:                styles.NewTheme(),
		CostWarningThreshold: cfg.Resources.CostWarningThreshold,
		CostLimit:            cfg.Resources.CostLimit,
	}

	// Add session data if available
	if m.session != nil {
		state.SessionCreated = m.session.Created
		state.SessionMetrics = m.orchestrator.GetSessionMetrics()
		state.Instances = m.session.Instances
	}

	statsPanel := panel.NewStatsPanel()
	return statsPanel.RenderWithBox(state, styles.ContentBox)
}

// renderTripleShotSidebar renders the sidebar for triple-shot mode.
// Uses normal sidebar view to ensure instances created during triple-shot are visible.
func (m Model) renderTripleShotSidebar(width, height int) string {
	return view.NewSidebarView().RenderSidebar(m, width, height)
}

// renderTripleShotHelp renders the help bar for triple-shot mode.
// Delegates to view.RenderTripleShotHelp for the actual rendering.
func (m Model) renderTripleShotHelp() string {
	return view.RenderTripleShotHelp(m.buildHelpBarState())
}

// renderAdversarialHelp renders the help bar for adversarial mode.
// Delegates to view.RenderAdversarialHelp for the actual rendering.
func (m Model) renderAdversarialHelp() string {
	return view.RenderAdversarialHelp(m.buildHelpBarState())
}

// initiateTripleShotMode creates and starts a triple-shot session.
// Supports multiple concurrent tripleshots by adding to the Coordinators map.
func (m Model) initiateTripleShotMode(task string) (Model, tea.Cmd) {
	// Create a group for this triple-shot session FIRST to get its ID
	tripleGroup := orchestrator.NewInstanceGroupWithType(
		util.TruncateString(task, 30),
		orchestrator.SessionTypeTripleShot,
		task,
	)
	m.session.AddGroup(tripleGroup)

	// Request intelligent name generation for the group
	m.orchestrator.RequestGroupRename(tripleGroup.ID, task)

	// Create triple-shot session with default config
	tripleConfig := orchestrator.DefaultTripleShotConfig()
	tripleSession := orchestrator.NewTripleShotSession(task, tripleConfig)

	// Link group ID to session for multi-tripleshot support
	tripleSession.GroupID = tripleGroup.ID

	// Add to TripleShots slice for persistence (supports multiple)
	m.session.TripleShots = append(m.session.TripleShots, tripleSession)

	// Create coordinator
	coordinator := orchestrator.NewTripleShotCoordinator(m.orchestrator, m.session, tripleSession, m.logger)

	// Auto-enable grouped sidebar mode
	m.autoEnableGroupedMode()

	// Initialize triple-shot state if needed, or add to existing coordinators
	if m.tripleShot == nil {
		m.tripleShot = &TripleShotState{
			Coordinators: make(map[string]*tripleshot.Coordinator),
		}
	} else if m.tripleShot.Coordinators == nil {
		m.tripleShot.Coordinators = make(map[string]*tripleshot.Coordinator)
	}

	// Add coordinator to the map keyed by group ID
	m.tripleShot.Coordinators[tripleGroup.ID] = coordinator

	numActive := len(m.tripleShot.Coordinators)
	if numActive > 1 {
		m.infoMessage = fmt.Sprintf("Starting triple-shot #%d...", numActive)
	} else {
		m.infoMessage = "Starting triple-shot mode..."
	}

	// Start attempts asynchronously
	return m, func() tea.Msg {
		if err := coordinator.StartAttempts(); err != nil {
			return tuimsg.TripleShotErrorMsg{Err: err}
		}
		return tuimsg.TripleShotStartedMsg{}
	}
}

// initiateAdversarialMode creates and starts an adversarial session.
// Supports multiple concurrent adversarial sessions by adding to the Coordinators map.
func (m Model) initiateAdversarialMode(task string) (Model, tea.Cmd) {
	// Validate required dependencies
	if m.session == nil {
		m.errorMessage = "Cannot start adversarial mode: no active session"
		if m.logger != nil {
			m.logger.Error("initiateAdversarialMode called with nil session")
		}
		return m, nil
	}
	if m.orchestrator == nil {
		m.errorMessage = "Cannot start adversarial mode: orchestrator not available"
		if m.logger != nil {
			m.logger.Error("initiateAdversarialMode called with nil orchestrator")
		}
		return m, nil
	}

	// Create a group for this adversarial session FIRST to get its ID
	advGroup := orchestrator.NewInstanceGroupWithType(
		util.TruncateString(task, 30),
		orchestrator.SessionTypeAdversarial,
		task,
	)
	m.session.AddGroup(advGroup)

	// Request intelligent name generation for the group
	m.orchestrator.RequestGroupRename(advGroup.ID, task)

	// Create adversarial session with default config
	advConfig := orchestrator.DefaultAdversarialConfig()
	advSession := orchestrator.NewAdversarialSession(task, advConfig)

	// Link group ID to session for multi-adversarial support
	advSession.GroupID = advGroup.ID

	// Add to AdversarialSessions slice for persistence (supports multiple)
	m.session.AdversarialSessions = append(m.session.AdversarialSessions, advSession)

	// Create coordinator
	coordinator := orchestrator.NewAdversarialCoordinator(m.orchestrator, m.session, advSession, m.logger)

	// Auto-enable grouped sidebar mode
	m.autoEnableGroupedMode()

	// Initialize adversarial state if needed, or add to existing coordinators
	if m.adversarial == nil {
		m.adversarial = &AdversarialState{
			Coordinators: make(map[string]*adversarial.Coordinator),
		}
	} else if m.adversarial.Coordinators == nil {
		m.adversarial.Coordinators = make(map[string]*adversarial.Coordinator)
	}

	// Add coordinator to the map keyed by group ID
	m.adversarial.Coordinators[advGroup.ID] = coordinator

	numActive := len(m.adversarial.Coordinators)
	if numActive > 1 {
		m.infoMessage = fmt.Sprintf("Starting adversarial session #%d...", numActive)
	} else {
		m.infoMessage = "Starting adversarial mode..."
	}

	// Start implementer asynchronously
	return m, func() tea.Msg {
		if err := coordinator.StartImplementer(); err != nil {
			return tuimsg.AdversarialErrorMsg{Err: err}
		}
		return tuimsg.AdversarialStartedMsg{}
	}
}

// handleInstanceRemoved processes the result of async instance removal.
func (m Model) handleInstanceRemoved(msg tuimsg.InstanceRemovedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.errorMessage = fmt.Sprintf("Failed to remove instance: %v", msg.Err)
		return m, nil
	}

	m.infoMessage = fmt.Sprintf("Removed instance %s", msg.InstanceID)

	// Adjust active tab if needed (instance count decreased)
	if m.activeTab >= m.instanceCount() {
		m.activeTab = m.instanceCount() - 1
		if m.activeTab < 0 {
			m.activeTab = 0
		}
	}

	m.resumeActiveInstance()
	m.ensureActiveVisible()

	return m, nil
}

// handleDiffLoaded processes the result of async diff loading.
func (m Model) handleDiffLoaded(msg tuimsg.DiffLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.errorMessage = fmt.Sprintf("Failed to get diff: %v", msg.Err)
		return m, nil
	}

	if msg.DiffContent == "" {
		m.infoMessage = "No changes to show"
		return m, nil
	}

	m.showDiff = true
	m.diffContent = msg.DiffContent
	m.diffScroll = 0
	m.infoMessage = "" // Clear "Loading diff..." message

	return m, nil
}

package tui

import (
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/command"
	"github.com/Iron-Ham/claudio/internal/tui/input"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/panel"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/tui/terminal"
	"github.com/Iron-Ham/claudio/internal/tui/view"
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

	// Create a group for this ultraplan session (similar to inline mode)
	ultraSession := coordinator.Session()
	if ultraSession != nil && ultraSession.GroupID == "" {
		sessionType := orchestrator.SessionTypeUltraPlan
		if ultraSession.Config.MultiPass {
			sessionType = orchestrator.SessionTypePlanMulti
		}
		ultraGroup := orchestrator.NewInstanceGroupWithType(
			truncateString(ultraSession.Objective, 30),
			sessionType,
			ultraSession.Objective,
		)
		session.AddGroup(ultraGroup)
		ultraSession.GroupID = ultraGroup.ID

		// Auto-enable grouped sidebar mode
		model.autoEnableGroupedMode()
	}

	model.ultraPlan = &UltraPlanState{
		Coordinator:  coordinator,
		ShowPlanView: false,
	}
	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithTripleShot creates a new TUI application in triple-shot mode
func NewWithTripleShot(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinator *orchestrator.TripleShotCoordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	model.tripleShot = &TripleShotState{
		Coordinator:  coordinator,
		Coordinators: make(map[string]*orchestrator.TripleShotCoordinator),
	}
	// Add the coordinator to the map keyed by its group ID for multiple tripleshot support
	if coordinator != nil {
		tripleSession := coordinator.Session()
		if tripleSession != nil {
			// Create a group if one doesn't exist (CLI-started tripleshots)
			if tripleSession.GroupID == "" {
				tripleGroup := orchestrator.NewInstanceGroupWithType(
					truncateString(tripleSession.Task, 30),
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

// NewWithTripleShots creates a new TUI application with multiple tripleshot coordinators.
// This is used when restoring a session that had multiple concurrent tripleshots.
func NewWithTripleShots(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinators []*orchestrator.TripleShotCoordinator, logger *logging.Logger) *App {
	model := NewModel(orch, session, logger)
	model.tripleShot = &TripleShotState{
		Coordinators: make(map[string]*orchestrator.TripleShotCoordinator),
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
						truncateString(tripleSession.Task, 30),
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

	// Set the first coordinator as the deprecated single Coordinator for backward compatibility
	if len(coordinators) > 0 {
		model.tripleShot.Coordinator = coordinators[0]
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
		contentWidth, contentHeight := CalculateContentDimensions(m.terminalManager.Width(), m.terminalManager.Height())
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
		m.outputManager.AddOutput(msg.InstanceID, string(msg.Data))
		return m, nil

	case tuimsg.ErrMsg:
		m.errorMessage = msg.Err.Error()
		return m, nil

	case tuimsg.PRCompleteMsg:
		// PR workflow completed - remove the instance
		inst := m.session.GetInstance(msg.InstanceID)
		if inst != nil {
			if err := m.orchestrator.RemoveInstance(m.session, msg.InstanceID, true); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to remove instance after PR: %v", err)
			} else if msg.Success {
				m.infoMessage = fmt.Sprintf("PR created and instance %s removed", msg.InstanceID)
			} else {
				m.infoMessage = fmt.Sprintf("PR workflow finished (may have failed) - instance %s removed", msg.InstanceID)
			}
		}
		return m, nil

	case tuimsg.PROpenedMsg:
		// PR URL detected in instance output - notify user but keep instance for potential review tools
		inst := m.session.GetInstance(msg.InstanceID)
		if inst != nil {
			m.infoMessage = fmt.Sprintf("PR opened for instance %s - use :D to remove or run review tools", inst.ID)
		}
		return m, nil

	case tuimsg.TimeoutMsg:
		// Instance timeout detected - notify user
		inst := m.session.GetInstance(msg.InstanceID)
		if inst != nil {
			var statusText string
			switch msg.TimeoutType {
			case instance.TimeoutActivity:
				statusText = "stuck (no activity)"
			case instance.TimeoutCompletion:
				statusText = "timed out (max runtime exceeded)"
			case instance.TimeoutStale:
				statusText = "stuck (repeated output)"
			}
			m.infoMessage = fmt.Sprintf("Instance %s is %s - use Ctrl+R to restart or Ctrl+K to kill", inst.ID, statusText)
		}
		return m, nil

	case tuimsg.BellMsg:
		// Terminal bell detected in a tmux session - forward it to the parent terminal
		return m, tuimsg.RingBell()

	case tuimsg.TaskAddedMsg:
		// Async task addition completed
		m.infoMessage = "" // Clear the "Adding task..." message
		if msg.Err != nil {
			m.errorMessage = msg.Err.Error()
			if m.logger != nil {
				m.logger.Error("failed to add task", "error", msg.Err.Error())
			}
		} else {
			// Pause the old active instance before switching (new instance starts unpaused)
			if oldInst := m.activeInstance(); oldInst != nil {
				m.pauseInstance(oldInst.ID)
			}
			// Switch to the newly added task and ensure it's visible in sidebar
			m.activeTab = len(m.session.Instances) - 1
			m.ensureActiveVisible()
			// Log user adding instance
			if m.logger != nil && msg.Instance != nil {
				m.logger.Info("user added instance", "task", msg.Instance.Task)
			}
		}
		return m, nil

	case tuimsg.DependentTaskAddedMsg:
		// Async dependent task addition completed
		m.infoMessage = "" // Clear the "Adding dependent task..." message
		if msg.Err != nil {
			m.errorMessage = msg.Err.Error()
			if m.logger != nil {
				m.logger.Error("failed to add dependent task",
					"depends_on", msg.DependsOn,
					"error", msg.Err.Error(),
				)
			}
		} else {
			// Pause the old active instance before switching (new instance starts unpaused)
			if oldInst := m.activeInstance(); oldInst != nil {
				m.pauseInstance(oldInst.ID)
			}
			// Switch to the newly added task and ensure it's visible in sidebar
			m.activeTab = len(m.session.Instances) - 1
			m.ensureActiveVisible()
			// Find the parent instance name for a better message
			parentTask := msg.DependsOn
			for _, inst := range m.session.Instances {
				if inst.ID == msg.DependsOn {
					parentTask = inst.Task
					if len(parentTask) > 50 {
						parentTask = parentTask[:50] + "..."
					}
					break
				}
			}
			m.infoMessage = fmt.Sprintf("Chained task added. Will auto-start when \"%s\" completes.", parentTask)
			// Log user adding dependent instance
			if m.logger != nil && msg.Instance != nil {
				m.logger.Info("user added dependent instance",
					"task", msg.Instance.Task,
					"depends_on", msg.DependsOn,
				)
			}
		}
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
	}

	return m, nil
}

// handleKeypress processes keyboard input
func (m Model) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Sync router state for mode tracking
	m.syncRouterState()

	// Handle search mode - typing search pattern
	if m.searchMode {
		return m.handleSearchInput(msg)
	}

	// Handle filter mode - selecting categories
	if m.filterMode {
		return m.handleFilterInput(msg)
	}

	// Handle input mode - forward keys to the active instance's tmux session
	if m.inputMode {
		// Ctrl+] exits input mode (traditional telnet escape)
		if msg.Type == tea.KeyCtrlCloseBracket {
			m.inputMode = false
			return m, nil
		}

		// Forward the key to the active instance's tmux session
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil && mgr.Running() {
				// Check if this is a paste operation
				// Note: msg is tea.KeyMsg which embeds tea.Key, so we can access Paste directly
				if msg.Paste && msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
					// Send pasted text with bracketed paste sequences
					// This preserves paste context for Claude Code
					mgr.SendPaste(string(msg.Runes))
				} else {
					input.SendKeyToTmux(mgr, msg)
				}
			}
		}
		return m, nil
	}

	// Handle terminal mode - forward keys to the terminal pane's tmux session
	if m.terminalManager.IsFocused() {
		// Ctrl+] exits terminal mode (same escape as input mode)
		if msg.Type == tea.KeyCtrlCloseBracket {
			m.exitTerminalMode()
			return m, nil
		}

		// Forward the key to the terminal pane's tmux session
		// Check if this is a paste operation
		if msg.Paste && msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
			if err := m.terminalManager.SendPaste(string(msg.Runes)); err != nil {
				if m.logger != nil {
					m.logger.Warn("failed to paste to terminal", "error", err)
				}
			}
		} else {
			m.terminalManager.SendKey(msg)
		}
		return m, nil
	}

	// Handle task input mode
	if m.addingTask {
		// Handle branch selector dropdown if visible
		if m.showBranchSelector {
			return m.handleBranchSelector(msg)
		}

		// Handle template dropdown if visible
		if m.showTemplates {
			return m.handleTemplateDropdown(msg)
		}

		// Check for newline shortcuts (shift+enter, alt+enter, or ctrl+j)
		// Note: shift+enter only works in terminals that support extended keyboard
		// protocols (Kitty, iTerm2, WezTerm, Ghostty). Alt+Enter and Ctrl+J work
		// universally as fallbacks.
		if msg.Type == tea.KeyEnter && msg.Alt {
			m.taskInputInsert("\n")
			return m, nil
		}
		if msg.String() == "shift+enter" {
			m.taskInputInsert("\n")
			return m, nil
		}
		if msg.Type == tea.KeyCtrlJ {
			m.taskInputInsert("\n")
			return m, nil
		}

		// Handle Opt+Arrow (word navigation) and Cmd+Arrow (line navigation)
		// On macOS, terminals typically report these as:
		// - Opt+Arrow: "alt+left", "alt+right", etc.
		// - Cmd+Arrow: May vary by terminal, often reported as special sequences
		keyStr := msg.String()
		switch keyStr {
		case "alt+left":
			// Opt+Left: Move to previous word boundary
			m.taskInputCursor = m.taskInputFindPrevWordBoundary()
			return m, nil
		case "alt+right":
			// Opt+Right: Move to next word boundary
			m.taskInputCursor = m.taskInputFindNextWordBoundary()
			return m, nil
		case "alt+up", "ctrl+a": // Cmd+Up often reported as ctrl+a in some terminals
			// Move to start of input
			m.taskInputCursor = 0
			return m, nil
		case "alt+down", "ctrl+e": // Cmd+Down often reported as ctrl+e in some terminals
			// Move to end of input
			m.taskInputCursor = len([]rune(m.taskInput))
			return m, nil
		case "alt+backspace", "ctrl+w":
			// Opt+Backspace: Delete previous word
			prevWord := m.taskInputFindPrevWordBoundary()
			m.taskInputDeleteBack(m.taskInputCursor - prevWord)
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEsc:
			m.addingTask = false
			m.addingDependentTask = false
			m.dependentOnInstanceID = ""
			m.startingTripleShot = false
			m.taskInput = ""
			m.taskInputCursor = 0
			m.templateSuffix = ""        // Clear suffix on cancel
			m.selectedBaseBranch = ""    // Clear base branch selection
			m.branchList = nil           // Clear cached branches
			m.showBranchSelector = false // Ensure dropdown is closed
			return m, nil
		case tea.KeyEnter:
			if m.taskInput != "" {
				// Capture task and clear input state first
				// Append template suffix if one was set (e.g., /plan instructions)
				task := m.taskInput + m.templateSuffix
				isDependent := m.addingDependentTask
				dependsOn := m.dependentOnInstanceID
				baseBranch := m.selectedBaseBranch
				isTripleShot := m.startingTripleShot
				m.addingTask = false
				m.addingDependentTask = false
				m.dependentOnInstanceID = ""
				m.startingTripleShot = false
				m.taskInput = ""
				m.taskInputCursor = 0
				m.templateSuffix = ""        // Clear suffix after use
				m.selectedBaseBranch = ""    // Clear base branch selection
				m.branchList = nil           // Clear cached branches
				m.showBranchSelector = false // Ensure dropdown is closed

				// Handle inline plan/ultraplan/multiplan objective submission
				if m.inlinePlan != nil && m.inlinePlan.AwaitingObjective {
					if m.inlinePlan.IsUltraPlan {
						// This is an ultraplan objective - create the full ultraplan coordinator
						m.handleUltraPlanObjectiveSubmit(task)
					} else if m.inlinePlan.MultiPass {
						// This is a multiplan objective - create parallel planning instances
						m.handleMultiPlanObjectiveSubmit(task)
					} else {
						// This is a regular plan objective
						m.handleInlinePlanObjectiveSubmit(task)
					}
					return m, nil
				}

				// Handle triple-shot mode initiation
				if isTripleShot {
					return m.initiateTripleShotMode(task)
				}

				// Add instance asynchronously to avoid blocking UI during git worktree creation
				if isDependent && dependsOn != "" {
					m.infoMessage = "Adding dependent task..."
					return m, tuimsg.AddDependentTaskAsync(m.orchestrator, m.session, task, dependsOn)
				}

				// Use selected base branch if specified, otherwise use default (current HEAD)
				if baseBranch != "" {
					m.infoMessage = "Adding task from branch " + baseBranch + "..."
					return m, tuimsg.AddTaskFromBranchAsync(m.orchestrator, m.session, task, baseBranch)
				}
				m.infoMessage = "Adding task..."
				return m, tuimsg.AddTaskAsync(m.orchestrator, m.session, task)
			}
			m.addingTask = false
			m.addingDependentTask = false
			m.dependentOnInstanceID = ""
			m.startingTripleShot = false
			m.taskInput = ""
			m.taskInputCursor = 0
			m.templateSuffix = ""        // Clear suffix on cancel
			m.selectedBaseBranch = ""    // Clear base branch selection
			m.branchList = nil           // Clear cached branches
			m.showBranchSelector = false // Ensure dropdown is closed
			return m, nil
		case tea.KeyBackspace:
			m.taskInputDeleteBack(1)
			return m, nil
		case tea.KeyDelete:
			m.taskInputDeleteForward(1)
			return m, nil
		case tea.KeyLeft:
			m.taskInputMoveCursor(-1)
			return m, nil
		case tea.KeyRight:
			m.taskInputMoveCursor(1)
			return m, nil
		case tea.KeyHome:
			// Move to start of current line
			m.taskInputCursor = m.taskInputFindLineStart()
			return m, nil
		case tea.KeyEnd:
			// Move to end of current line
			m.taskInputCursor = m.taskInputFindLineEnd()
			return m, nil
		case tea.KeyCtrlU:
			// Cmd+Backspace equivalent: Delete from cursor to start of line
			lineStart := m.taskInputFindLineStart()
			m.taskInputDeleteBack(m.taskInputCursor - lineStart)
			return m, nil
		case tea.KeyCtrlK:
			// Delete from cursor to end of line
			lineEnd := m.taskInputFindLineEnd()
			m.taskInputDeleteForward(lineEnd - m.taskInputCursor)
			return m, nil
		case tea.KeySpace:
			m.taskInputInsert(" ")
			return m, nil
		case tea.KeyTab:
			// Open branch selector
			return m.openBranchSelector()
		case tea.KeyRunes:
			char := string(msg.Runes)
			// Handle Enter sent as rune (some terminals/input methods send \n or \r as runes)
			if char == "\n" || char == "\r" {
				if m.taskInput != "" {
					// Capture task and clear input state first
					task := m.taskInput + m.templateSuffix
					isDependent := m.addingDependentTask
					dependsOn := m.dependentOnInstanceID
					baseBranch := m.selectedBaseBranch
					isTripleShot := m.startingTripleShot
					m.addingTask = false
					m.addingDependentTask = false
					m.dependentOnInstanceID = ""
					m.startingTripleShot = false
					m.taskInput = ""
					m.taskInputCursor = 0
					m.templateSuffix = ""        // Clear suffix after use
					m.selectedBaseBranch = ""    // Clear base branch selection
					m.branchList = nil           // Clear cached branches
					m.showBranchSelector = false // Ensure dropdown is closed

					// Handle triple-shot mode initiation
					if isTripleShot {
						return m.initiateTripleShotMode(task)
					}

					// Add instance asynchronously to avoid blocking UI during git worktree creation
					if isDependent && dependsOn != "" {
						m.infoMessage = "Adding dependent task..."
						return m, tuimsg.AddDependentTaskAsync(m.orchestrator, m.session, task, dependsOn)
					}

					// Use selected base branch if specified, otherwise use default (current HEAD)
					if baseBranch != "" {
						m.infoMessage = "Adding task from branch " + baseBranch + "..."
						return m, tuimsg.AddTaskFromBranchAsync(m.orchestrator, m.session, task, baseBranch)
					}
					m.infoMessage = "Adding task..."
					return m, tuimsg.AddTaskAsync(m.orchestrator, m.session, task)
				}
				m.addingTask = false
				m.addingDependentTask = false
				m.dependentOnInstanceID = ""
				m.startingTripleShot = false
				m.taskInput = ""
				m.taskInputCursor = 0
				m.templateSuffix = ""        // Clear suffix on cancel
				m.selectedBaseBranch = ""    // Clear base branch selection
				m.branchList = nil           // Clear cached branches
				m.showBranchSelector = false // Ensure dropdown is closed
				return m, nil
			}
			// Detect "/" at start of input or after newline to show templates
			cursorAtLineStart := m.taskInputCursor == 0 ||
				(m.taskInputCursor > 0 && []rune(m.taskInput)[m.taskInputCursor-1] == '\n')
			if char == "/" && cursorAtLineStart {
				m.showTemplates = true
				m.templateFilter = ""
				m.templateSelected = 0
				m.taskInputInsert(char)
				return m, nil
			}
			m.taskInputInsert(char)
			return m, nil
		}
		return m, nil
	}

	// Handle command mode (vim-style ex commands with ':' prefix)
	if m.commandMode {
		return m.handleCommandInput(msg)
	}

	// Normal mode - clear info message on most actions
	m.infoMessage = ""

	// Handle plan editor mode specific keys first (highest priority in ultra-plan)
	if m.IsPlanEditorActive() {
		handled, model, cmd := m.handlePlanEditorKeypress(msg)
		if handled {
			return model, cmd
		}
	}

	// Handle ultra-plan mode specific keys
	if m.IsUltraPlanMode() {
		handled, model, cmd := m.handleUltraPlanKeypress(msg)
		if handled {
			return model, cmd
		}
	}

	// Handle group command mode (vim-style 'g' prefix)
	// When groupCommandPending is true, the next key is interpreted as a group command
	if m.inputRouter != nil && m.inputRouter.IsGroupCommandPending() {
		m.inputRouter.SetGroupCommandPending(false) // Clear pending state
		groupHandler := NewGroupKeyHandler(m.session, m.groupViewState)
		result := groupHandler.HandleGroupKey(msg)
		if result.Handled {
			// Apply result actions based on the Action type
			switch result.Action {
			case GroupActionToggleCollapse:
				if result.GroupID != "" {
					m.groupViewState.ToggleCollapse(result.GroupID)
				}
			case GroupActionCollapseAll:
				if result.AllCollapsed {
					// Collapse all groups
					for _, g := range m.session.Groups {
						if !m.groupViewState.IsCollapsed(g.ID) {
							m.groupViewState.ToggleCollapse(g.ID)
						}
					}
				} else {
					// Expand all groups
					for _, g := range m.session.Groups {
						if m.groupViewState.IsCollapsed(g.ID) {
							m.groupViewState.ToggleCollapse(g.ID)
						}
					}
				}
			case GroupActionNextGroup, GroupActionPrevGroup:
				if result.GroupID != "" {
					m.groupViewState.SelectedGroupID = result.GroupID
				}
			case GroupActionSkipGroup:
				m.infoMessage = "Group skipped"
			case GroupActionRetryGroup:
				m.infoMessage = "Retrying failed tasks in group"
			case GroupActionForceStart:
				m.infoMessage = "Force-starting next group"
			}
			return m, nil
		}
		// If not handled, fall through to normal key handling
	}

	switch msg.String() {
	case ":":
		// Enter command mode (vim-style)
		m.commandMode = true
		m.commandBuffer = ""
		return m, nil

	case "?":
		m.showHelp = !m.showHelp
		if !m.showHelp {
			m.helpScroll = 0
		}
		return m, nil

	case "tab", "l":
		if m.instanceCount() > 0 {
			newTab := (m.activeTab + 1) % m.instanceCount()
			m.switchToInstance(newTab)
			m.ensureActiveVisible()
			m.updateTerminalOnInstanceChange()
			// Log focus change
			if m.logger != nil {
				if inst := m.activeInstance(); inst != nil {
					m.logger.Info("user focused instance", "instance_id", inst.ID)
				}
			}
		}
		return m, nil

	case "shift+tab", "h":
		if m.instanceCount() > 0 {
			newTab := (m.activeTab - 1 + m.instanceCount()) % m.instanceCount()
			m.switchToInstance(newTab)
			m.ensureActiveVisible()
			m.updateTerminalOnInstanceChange()
			// Log focus change
			if m.logger != nil {
				if inst := m.activeInstance(); inst != nil {
					m.logger.Info("user focused instance", "instance_id", inst.ID)
				}
			}
		}
		return m, nil

	case "enter", "i":
		// Enter input mode for the active instance
		// Allow input if tmux session exists (running or waiting for input)
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil && mgr.TmuxSessionExists() {
				m.inputMode = true
			}
		}
		return m, nil

	case "`", "T":
		// Toggle terminal pane visibility
		sessionID := ""
		if m.orchestrator != nil {
			sessionID = m.orchestrator.SessionID()
		}
		m.toggleTerminalVisibility(sessionID)
		return m, nil

	case "ctrl+shift+t":
		// Switch terminal directory mode (worktree <-> invocation)
		if m.terminalManager.IsVisible() {
			m.switchTerminalDir()
		}
		return m, nil

	case "esc":
		// Close diff panel if open
		if m.showDiff {
			m.showDiff = false
			m.diffContent = ""
			m.diffScroll = 0
			return m, nil
		}
		return m, nil

	case "j", "down":
		// Scroll down in diff view, help panel, output view, or navigate to next instance
		if m.showDiff {
			m.diffScroll++
			return m, nil
		}
		if m.showHelp {
			m.helpScroll++
			return m, nil
		}
		if m.showConflicts {
			// Don't scroll output when conflict panel is shown
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			// Scroll output down
			m.scrollOutputDown(inst.ID, 1)
			return m, nil
		}
		return m, nil

	case "k", "up":
		// Scroll up in diff view, help panel, output view, or navigate to previous instance
		if m.showDiff {
			if m.diffScroll > 0 {
				m.diffScroll--
			}
			return m, nil
		}
		if m.showHelp {
			if m.helpScroll > 0 {
				m.helpScroll--
			}
			return m, nil
		}
		if m.showConflicts {
			// Don't scroll output when conflict panel is shown
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			// Scroll output up
			m.scrollOutputUp(inst.ID, 1)
			return m, nil
		}
		return m, nil

	case "ctrl+u":
		// Scroll up half page in help panel or output view
		if m.showHelp {
			m.helpScroll -= 10
			if m.helpScroll < 0 {
				m.helpScroll = 0
			}
			return m, nil
		}
		if m.showDiff || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			halfPage := m.getOutputMaxLines() / 2
			m.scrollOutputUp(inst.ID, halfPage)
		}
		return m, nil

	case "ctrl+d":
		// Scroll down half page in help panel or output view
		if m.showHelp {
			m.helpScroll += 10
			return m, nil
		}
		if m.showDiff || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			halfPage := m.getOutputMaxLines() / 2
			m.scrollOutputDown(inst.ID, halfPage)
		}
		return m, nil

	case "ctrl+b":
		// Scroll up full page in help panel or output view
		if m.showHelp {
			m.helpScroll -= 20
			if m.helpScroll < 0 {
				m.helpScroll = 0
			}
			return m, nil
		}
		if m.showDiff || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			fullPage := m.getOutputMaxLines()
			m.scrollOutputUp(inst.ID, fullPage)
		}
		return m, nil

	case "ctrl+f":
		// Scroll down full page in help panel or output view
		if m.showHelp {
			m.helpScroll += 20
			return m, nil
		}
		if m.showDiff || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			fullPage := m.getOutputMaxLines()
			m.scrollOutputDown(inst.ID, fullPage)
		}
		return m, nil

	case "ctrl+r":
		// Restart instance with same task (useful for stuck/timeout instances)
		if inst := m.activeInstance(); inst != nil {
			// Only allow restarting stuck, timeout, completed, paused, or error instances
			switch inst.Status {
			case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
				m.infoMessage = "Instance is running. Use [:x] to stop it first, or [:p] to pause."
				return m, nil
			case orchestrator.StatusCreatingPR:
				m.infoMessage = "Instance is creating PR. Wait for it to complete."
				return m, nil
			}

			// Stop the instance if it's still running in tmux
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil {
				_ = mgr.Stop()
				mgr.ClearTimeout() // Reset timeout state
			}

			// Restart with same task
			if err := m.orchestrator.ReconnectInstance(inst); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to restart instance: %v", err)
			} else {
				m.infoMessage = fmt.Sprintf("Instance %s restarted with same task", inst.ID)
			}
		}
		return m, nil

	case "ctrl+k":
		// Kill and remove instance (force remove)
		if inst := m.activeInstance(); inst != nil {
			// Stop the instance first
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil {
				_ = mgr.Stop()
			}

			// Remove the instance
			if err := m.orchestrator.RemoveInstance(m.session, inst.ID, true); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to remove instance: %v", err)
			} else {
				m.infoMessage = fmt.Sprintf("Instance %s killed and removed", inst.ID)
				// Adjust active tab if needed
				if m.activeTab >= len(m.session.Instances) && m.activeTab > 0 {
					m.activeTab--
				}
				// Resume the new active instance's capture (it may have been paused)
				m.resumeActiveInstance()
			}
		}
		return m, nil

	case "g":
		// In grouped sidebar mode with groups, enter group command mode
		if m.sidebarMode == view.SidebarModeGrouped && m.session != nil && len(m.session.Groups) > 0 {
			m.inputRouter.SetGroupCommandPending(true)
			return m, nil
		}
		// Otherwise, go to top of diff, help panel, or output
		if m.showDiff {
			m.diffScroll = 0
			return m, nil
		}
		if m.showHelp {
			m.helpScroll = 0
			return m, nil
		}
		if m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			m.scrollOutputToTop(inst.ID)
		}
		return m, nil

	case "G":
		// Go to bottom of diff, help panel, or output (re-enables auto-scroll)
		if m.showDiff {
			lines := strings.Split(m.diffContent, "\n")
			maxLines := m.terminalManager.Height() - 14
			if maxLines < 5 {
				maxLines = 5
			}
			maxScroll := len(lines) - maxLines
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.diffScroll = maxScroll
			return m, nil
		}
		if m.showHelp {
			// Jump to bottom of help (will be clamped in render)
			m.helpScroll = 1000
			return m, nil
		}
		if m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			m.scrollOutputToBottom(inst.ID)
		}
		return m, nil

	case "/":
		// Enter search mode
		m.searchMode = true
		m.searchEngine.Clear()
		return m, nil

	case "n":
		// Next search match
		m.SearchHandler().NextMatch()
		return m, nil

	case "N":
		// Previous search match
		m.SearchHandler().PreviousMatch()
		return m, nil

	case "ctrl+/":
		// Clear search
		m.SearchHandler().Clear()
		return m, nil
	}

	return m, nil
}

// CurrentInputMode returns the current input mode based on the model's state.
// This is useful for status line displays and debugging.
func (m Model) CurrentInputMode() input.Mode {
	if m.inputRouter != nil {
		return m.inputRouter.Mode()
	}
	// Fallback if router not initialized
	switch {
	case m.searchMode:
		return input.ModeSearch
	case m.filterMode:
		return input.ModeFilter
	case m.inputMode:
		return input.ModeInput
	case m.terminalManager.IsFocused():
		return input.ModeTerminal
	case m.addingTask:
		return input.ModeTaskInput
	case m.commandMode:
		return input.ModeCommand
	default:
		return input.ModeNormal
	}
}

// handleCommandInput handles keystrokes when in command mode (after pressing ':')
func (m Model) handleCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Exit command mode without executing
		m.commandMode = false
		m.commandBuffer = ""
		return m, nil

	case tea.KeyEnter:
		// Execute the command and exit command mode
		m.commandMode = false
		cmd := m.commandBuffer
		m.commandBuffer = ""
		return m.executeCommand(cmd)

	case tea.KeyBackspace, tea.KeyDelete:
		// Delete last character from command buffer
		if len(m.commandBuffer) > 0 {
			m.commandBuffer = m.commandBuffer[:len(m.commandBuffer)-1]
		}
		// If buffer is empty after backspace, exit command mode
		if len(m.commandBuffer) == 0 {
			m.commandMode = false
		}
		return m, nil

	case tea.KeySpace:
		m.commandBuffer += " "
		return m, nil

	case tea.KeyRunes:
		// Add typed characters to the command buffer
		m.commandBuffer += string(msg.Runes)
		return m, nil
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

// handleTemplateDropdown handles keyboard input when the template dropdown is visible.
// Delegates to view.TemplateDropdownHandler for the actual key processing logic.
func (m Model) handleTemplateDropdown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Create state struct to pass to handler
	state := &view.TemplateDropdownState{
		ShowTemplates:    m.showTemplates,
		TemplateFilter:   m.templateFilter,
		TemplateSelected: m.templateSelected,
		TemplateSuffix:   m.templateSuffix,
		TaskInput:        m.taskInput,
		TaskInputCursor:  m.taskInputCursor,
	}

	// Create handler with filter function adapter
	filterFunc := func(filter string) []view.Template {
		templates := FilterTemplates(filter)
		result := make([]view.Template, len(templates))
		for i, t := range templates {
			result[i] = view.Template{
				Command:     t.Command,
				Name:        t.Name,
				Description: t.Description,
				Suffix:      t.Suffix,
			}
		}
		return result
	}

	handler := view.NewTemplateDropdownHandler(state, filterFunc)
	handled, cmd := handler.HandleKey(msg)

	// Sync state back to model
	m.showTemplates = state.ShowTemplates
	m.templateFilter = state.TemplateFilter
	m.templateSelected = state.TemplateSelected
	m.templateSuffix = state.TemplateSuffix
	m.taskInput = state.TaskInput
	m.taskInputCursor = state.TaskInputCursor

	if handled {
		return m, cmd
	}
	return m, nil
}

// openBranchSelector opens the branch selector dropdown and populates the branch list.
func (m Model) openBranchSelector() (tea.Model, tea.Cmd) {
	// Fetch branches from the orchestrator
	branches, err := m.orchestrator.ListBranches()
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to list branches", "error", err)
		}
		m.errorMessage = "Failed to list branches: " + err.Error()
		return m, nil
	}

	// Convert to string list for the model
	m.branchList = make([]string, len(branches))
	for i, b := range branches {
		m.branchList[i] = b.Name
	}

	// Initialize filter state - start with all branches visible
	m.branchSearchInput = ""
	m.branchFiltered = m.branchList
	m.branchScrollOffset = 0

	// Calculate visible height for branch selector (reserve space for UI elements)
	dims := m.terminalManager.GetPaneDimensions(m.calculateExtraFooterLines())
	// Reserve: search line, scroll indicators, count line, padding
	m.branchSelectorHeight = dims.MainAreaHeight - 10
	if m.branchSelectorHeight < 5 {
		m.branchSelectorHeight = 5
	}
	if m.branchSelectorHeight > 15 {
		m.branchSelectorHeight = 15 // Cap at reasonable max
	}

	// Find the index of the currently selected branch (if any)
	selectedIdx := 0
	if m.selectedBaseBranch != "" {
		for i, name := range m.branchFiltered {
			if name == m.selectedBaseBranch {
				selectedIdx = i
				break
			}
		}
	}

	m.showBranchSelector = true
	m.branchSelected = selectedIdx
	m = m.adjustBranchScroll()

	return m, nil
}

// closeBranchSelector resets the branch selector state.
func (m Model) closeBranchSelector() Model {
	m.showBranchSelector = false
	m.branchSearchInput = ""
	m.branchFiltered = nil
	m.branchScrollOffset = 0
	return m
}

// handleBranchSelector handles keyboard input when the branch selector is visible.
func (m Model) handleBranchSelector(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.closeBranchSelector(), nil

	case tea.KeyEnter, tea.KeyTab:
		// Select the highlighted branch from the filtered list
		if len(m.branchFiltered) > 0 && m.branchSelected < len(m.branchFiltered) {
			m.selectedBaseBranch = m.branchFiltered[m.branchSelected]
		}
		return m.closeBranchSelector(), nil

	case tea.KeyUp:
		if m.branchSelected > 0 {
			m.branchSelected--
			m = m.adjustBranchScroll()
		}
		return m, nil

	case tea.KeyDown:
		if m.branchSelected < len(m.branchFiltered)-1 {
			m.branchSelected++
			m = m.adjustBranchScroll()
		}
		return m, nil

	case tea.KeyPgUp, tea.KeyCtrlU:
		// Page up
		m.branchSelected -= m.branchSelectorHeight
		if m.branchSelected < 0 {
			m.branchSelected = 0
		}
		m = m.adjustBranchScroll()
		return m, nil

	case tea.KeyPgDown, tea.KeyCtrlD:
		// Page down
		m.branchSelected += m.branchSelectorHeight
		if m.branchSelected >= len(m.branchFiltered) {
			m.branchSelected = len(m.branchFiltered) - 1
		}
		if m.branchSelected < 0 {
			m.branchSelected = 0
		}
		m = m.adjustBranchScroll()
		return m, nil

	case tea.KeyBackspace:
		// Remove last character from search
		if len(m.branchSearchInput) > 0 {
			runes := []rune(m.branchSearchInput)
			m.branchSearchInput = string(runes[:len(runes)-1])
			m = m.applyBranchFilter()
		}
		return m, nil

	case tea.KeyRunes:
		// Add typed characters to search
		m.branchSearchInput += string(msg.Runes)
		m = m.applyBranchFilter()
		return m, nil

	case tea.KeySpace:
		// Add space to search
		m.branchSearchInput += " "
		m = m.applyBranchFilter()
		return m, nil
	}

	return m, nil
}

// applyBranchFilter filters the branch list based on search input.
// Returns a new Model with the filter applied.
func (m Model) applyBranchFilter() Model {
	if m.branchSearchInput == "" {
		m.branchFiltered = m.branchList
	} else {
		searchLower := strings.ToLower(m.branchSearchInput)
		m.branchFiltered = nil
		for _, name := range m.branchList {
			if strings.Contains(strings.ToLower(name), searchLower) {
				m.branchFiltered = append(m.branchFiltered, name)
			}
		}
	}

	// Reset selection to first item when filter changes
	m.branchSelected = 0
	m.branchScrollOffset = 0

	// Try to keep previously selected branch selected if it's still visible
	if m.selectedBaseBranch != "" {
		for i, name := range m.branchFiltered {
			if name == m.selectedBaseBranch {
				m.branchSelected = i
				break
			}
		}
	}

	return m.adjustBranchScroll()
}

// adjustBranchScroll adjusts scroll offset to keep selection visible.
// Returns a new Model with the scroll adjusted.
func (m Model) adjustBranchScroll() Model {
	if m.branchSelectorHeight <= 0 {
		return m
	}

	// If selection is above viewport, scroll up
	if m.branchSelected < m.branchScrollOffset {
		m.branchScrollOffset = m.branchSelected
	}

	// If selection is below viewport, scroll down
	if m.branchSelected >= m.branchScrollOffset+m.branchSelectorHeight {
		m.branchScrollOffset = m.branchSelected - m.branchSelectorHeight + 1
	}

	// Clamp scroll offset
	maxScroll := len(m.branchFiltered) - m.branchSelectorHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.branchScrollOffset > maxScroll {
		m.branchScrollOffset = maxScroll
	}
	if m.branchScrollOffset < 0 {
		m.branchScrollOffset = 0
	}

	return m
}

// handleSearchInput handles keyboard input when in search mode.
// Delegates to search.Handler for actual search operations.
func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	handler := m.SearchHandler()

	switch msg.Type {
	case tea.KeyEsc:
		// Cancel search mode (keep existing pattern if any)
		m.searchMode = false
		return m, nil

	case tea.KeyEnter:
		// Execute search and exit search mode
		handler.Execute()
		m.searchMode = false
		return m, nil

	case tea.KeyBackspace:
		handler.HandleBackspace()
		return m, nil

	case tea.KeyRunes:
		handler.HandleRunes(string(msg.Runes))
		return m, nil

	case tea.KeySpace:
		handler.HandleRunes(" ")
		return m, nil
	}

	return m, nil
}

// handleFilterInput handles keyboard input when in filter mode
func (m Model) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "F", "q":
		m.filterMode = false
		return m, nil

	case "e", "1":
		m.filterCategories["errors"] = !m.filterCategories["errors"]
		return m, nil

	case "w", "2":
		m.filterCategories["warnings"] = !m.filterCategories["warnings"]
		return m, nil

	case "t", "3":
		m.filterCategories["tools"] = !m.filterCategories["tools"]
		return m, nil

	case "h", "4":
		m.filterCategories["thinking"] = !m.filterCategories["thinking"]
		return m, nil

	case "p", "5":
		m.filterCategories["progress"] = !m.filterCategories["progress"]
		return m, nil

	case "a":
		// Toggle all categories
		allEnabled := true
		for _, v := range m.filterCategories {
			if !v {
				allEnabled = false
				break
			}
		}
		for k := range m.filterCategories {
			m.filterCategories[k] = !allEnabled
		}
		return m, nil

	case "c":
		// Clear custom filter
		m.filterCustom = ""
		m.filterRegex = nil
		return m, nil
	}

	// Handle custom filter input
	switch msg.Type {
	case tea.KeyBackspace:
		if len(m.filterCustom) > 0 {
			m.filterCustom = m.filterCustom[:len(m.filterCustom)-1]
			m.compileFilterRegex()
		}
		return m, nil

	case tea.KeyRunes:
		// Check if it's not a shortcut key
		char := string(msg.Runes)
		if char != "e" && char != "w" && char != "t" && char != "h" && char != "p" && char != "a" && char != "c" {
			m.filterCustom += char
			m.compileFilterRegex()
		}
		return m, nil

	case tea.KeySpace:
		m.filterCustom += " "
		m.compileFilterRegex()
		return m, nil
	}

	return m, nil
}

// compileFilterRegex compiles the custom filter pattern
func (m *Model) compileFilterRegex() {
	if m.filterCustom == "" {
		m.filterRegex = nil
		return
	}

	re, err := regexp.Compile("(?i)" + m.filterCustom)
	if err != nil {
		m.filterRegex = nil
		return
	}
	m.filterRegex = re
}

// filterOutput applies category and custom filters to output
func (m *Model) filterOutput(output string) string {
	// If all categories enabled and no custom filter, return as-is
	allEnabled := true
	for _, v := range m.filterCategories {
		if !v {
			allEnabled = false
			break
		}
	}
	if allEnabled && m.filterRegex == nil {
		return output
	}

	lines := strings.Split(output, "\n")
	var filtered []string

	for _, line := range lines {
		if m.shouldShowLine(line) {
			filtered = append(filtered, line)
		}
	}

	return strings.Join(filtered, "\n")
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

	// Header - use mode-specific header if applicable
	var header string
	if m.IsUltraPlanMode() {
		header = m.renderUltraPlanHeader()
	} else if m.IsTripleShotMode() {
		header = m.renderTripleShotHeader()
	} else {
		header = m.renderHeader()
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Get pane dimensions, accounting for dynamic footer elements
	dims := m.terminalManager.GetPaneDimensions(m.calculateExtraFooterLines())
	effectiveSidebarWidth := SidebarWidth
	if dims.TerminalWidth < 80 {
		effectiveSidebarWidth = SidebarMinWidth
	}
	mainContentWidth := dims.TerminalWidth - effectiveSidebarWidth - 3 // 3 for gap between panels

	// Main area height is pre-calculated by terminal manager
	// (accounts for header, footer, and terminal pane)
	mainAreaHeight := dims.MainAreaHeight

	// Sidebar + Content area (horizontal layout)
	// Use mode-specific rendering if applicable
	var sidebar, content string
	if m.IsUltraPlanMode() {
		sidebar = m.renderUltraPlanSidebar(effectiveSidebarWidth, mainAreaHeight)
		content = m.renderUltraPlanContent(mainContentWidth)
	} else if m.IsTripleShotMode() {
		sidebar = m.renderTripleShotSidebar(effectiveSidebarWidth, mainAreaHeight)
		content = m.renderContent(mainContentWidth) // Reuse normal content for now
	} else {
		// Use view component for sidebar rendering
		// SidebarView automatically handles both flat and grouped modes
		sidebarView := view.NewSidebarView()
		sidebar = sidebarView.RenderSidebar(m, effectiveSidebarWidth, mainAreaHeight)
		content = m.renderContent(mainContentWidth)
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
	} else {
		b.WriteString(m.renderHelp())
	}

	return b.String()
}

// renderHeader renders the header bar
func (m Model) renderHeader() string {
	title := "Claudio"
	if m.session != nil && m.session.Name != "" {
		title = fmt.Sprintf("Claudio: %s", m.session.Name)
	}

	return styles.Header.Width(m.terminalManager.Width()).Render(title)
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

// renderFilterPanel renders the filter configuration panel
func (m Model) renderFilterPanel(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Output Filters"))
	b.WriteString("\n\n")
	b.WriteString(styles.Muted.Render("Toggle categories to show/hide specific output types:"))
	b.WriteString("\n\n")

	// Category checkboxes
	categories := []struct {
		key      string
		label    string
		shortcut string
	}{
		{"errors", "Errors", "e/1"},
		{"warnings", "Warnings", "w/2"},
		{"tools", "Tool calls", "t/3"},
		{"thinking", "Thinking", "h/4"},
		{"progress", "Progress", "p/5"},
	}

	for _, cat := range categories {
		var checkbox string
		var labelStyle lipgloss.Style
		if m.filterCategories[cat.key] {
			checkbox = styles.FilterCheckbox.Render("[✓]")
			labelStyle = styles.FilterCategoryEnabled
		} else {
			checkbox = styles.FilterCheckboxEmpty.Render("[ ]")
			labelStyle = styles.FilterCategoryDisabled
		}

		line := fmt.Sprintf("%s %s %s",
			checkbox,
			labelStyle.Render(cat.label),
			styles.Muted.Render("("+cat.shortcut+")"))
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("[a] Toggle all  [c] Clear custom filter"))
	b.WriteString("\n\n")

	// Custom filter input
	b.WriteString(styles.Secondary.Render("Custom filter:"))
	b.WriteString(" ")
	if m.filterCustom != "" {
		b.WriteString(styles.SearchInput.Render(m.filterCustom))
	} else {
		b.WriteString(styles.Muted.Render("(type to filter by pattern)"))
	}
	b.WriteString("\n\n")

	// Help text
	b.WriteString(styles.Muted.Render("Category descriptions:"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Errors: Stack traces, error messages, failures"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Warnings: Warning indicators"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Tool calls: File operations, bash commands"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Thinking: Claude's reasoning phrases"))
	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("  • Progress: Progress indicators, spinners"))
	b.WriteString("\n\n")

	b.WriteString(styles.Muted.Render("Press [Esc] or [F] to close"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
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

	// Customize title/subtitle for dependent task mode
	if m.addingDependentTask && m.dependentOnInstanceID != "" {
		inputState.Title = "Chain New Task"
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

// renderTripleShotHeader renders the header for triple-shot mode
func (m Model) renderTripleShotHeader() string {
	ctx := view.TripleShotRenderContext{
		Orchestrator: m.orchestrator,
		Session:      m.session,
		TripleShot:   m.tripleShot,
		ActiveTab:    m.activeTab,
		Width:        m.terminalManager.Width(),
		Height:       m.terminalManager.Height(),
	}
	return view.RenderTripleShotHeader(ctx)
}

// renderTripleShotSidebar renders the sidebar for triple-shot mode.
// Uses normal sidebar view to ensure instances created during triple-shot are visible.
func (m Model) renderTripleShotSidebar(width, height int) string {
	return view.NewSidebarView().RenderSidebar(m, width, height)
}

// renderTripleShotHelp renders the help bar for triple-shot mode.
// Delegates to view.RenderTripleShotHelp for the actual rendering.
func (m Model) renderTripleShotHelp() string {
	return view.RenderTripleShotHelp()
}

// initiateTripleShotMode creates and starts a triple-shot session.
// Supports multiple concurrent tripleshots by adding to the Coordinators map.
func (m Model) initiateTripleShotMode(task string) (Model, tea.Cmd) {
	// Create a group for this triple-shot session FIRST to get its ID
	tripleGroup := orchestrator.NewInstanceGroupWithType(
		truncateString(task, 30),
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

	// Also set single TripleShot field for backward compatibility
	m.session.TripleShot = tripleSession

	// Create coordinator
	coordinator := orchestrator.NewTripleShotCoordinator(m.orchestrator, m.session, tripleSession, m.logger)

	// Auto-enable grouped sidebar mode
	m.autoEnableGroupedMode()

	// Initialize triple-shot state if needed, or add to existing coordinators
	if m.tripleShot == nil {
		m.tripleShot = &TripleShotState{
			Coordinators: make(map[string]*orchestrator.TripleShotCoordinator),
		}
	} else if m.tripleShot.Coordinators == nil {
		m.tripleShot.Coordinators = make(map[string]*orchestrator.TripleShotCoordinator)
	}

	// Add coordinator to the map keyed by group ID
	m.tripleShot.Coordinators[tripleGroup.ID] = coordinator

	// Also set the deprecated single Coordinator for backward compatibility
	m.tripleShot.Coordinator = coordinator

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

package tui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// App wraps the Bubbletea program
type App struct {
	program      *tea.Program
	model        Model
	orchestrator *orchestrator.Orchestrator
	session      *orchestrator.Session
}

// New creates a new TUI application
func New(orch *orchestrator.Orchestrator, session *orchestrator.Session) *App {
	model := NewModel(orch, session)
	return &App{
		model:        model,
		orchestrator: orch,
		session:      session,
	}
}

// NewWithUltraPlan creates a new TUI application in ultra-plan mode
func NewWithUltraPlan(orch *orchestrator.Orchestrator, session *orchestrator.Session, coordinator *orchestrator.Coordinator) *App {
	model := NewModel(orch, session)
	model.ultraPlan = &UltraPlanState{
		coordinator:  coordinator,
		showPlanView: false,
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
	defer a.orchestrator.ReleaseLock()

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

	// Set up PR workflow completion callback to send message to program
	a.orchestrator.SetPRCompleteCallback(func(instanceID string, success bool) {
		a.program.Send(prCompleteMsg{
			instanceID: instanceID,
			success:    success,
		})
	})

	// Set up PR opened callback (for inline PR creation detected in instance output)
	a.orchestrator.SetPROpenedCallback(func(instanceID string) {
		a.program.Send(prOpenedMsg{
			instanceID: instanceID,
		})
	})

	// Set up timeout callback for stuck/timeout detection
	a.orchestrator.SetTimeoutCallback(func(instanceID string, timeoutType instance.TimeoutType) {
		a.program.Send(timeoutMsg{
			instanceID:  instanceID,
			timeoutType: timeoutType,
		})
	})

	// Set up bell callback to forward terminal bells to the parent terminal
	a.orchestrator.SetBellCallback(func(instanceID string) {
		a.program.Send(bellMsg{instanceID: instanceID})
	})

	_, err := a.program.Run()

	// Clean up signal handler
	signal.Stop(sigChan)

	return err
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}

	// Schedule ultra-plan initialization if needed
	if m.ultraPlan != nil && m.ultraPlan.coordinator != nil {
		session := m.ultraPlan.coordinator.Session()
		if session != nil && session.Phase == orchestrator.PhasePlanning && session.CoordinatorID == "" {
			cmds = append(cmds, func() tea.Msg { return ultraPlanInitMsg{} })
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
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Calculate the content area dimensions and resize tmux sessions
		contentWidth, contentHeight := CalculateContentDimensions(m.width, m.height)
		if m.orchestrator != nil && contentWidth > 0 && contentHeight > 0 {
			m.orchestrator.ResizeAllInstances(contentWidth, contentHeight)
		}

		// Ensure active instance is still visible after resize
		m.ensureActiveVisible()

		// On first ready (TUI just started), check if we should open plan editor
		// This handles the case when --plan FILE --review is provided
		if !wasReady && m.ultraPlan != nil && m.ultraPlan.coordinator != nil {
			session := m.ultraPlan.coordinator.Session()
			if session != nil && session.Phase == orchestrator.PhaseRefresh && session.Plan != nil && session.Config.Review {
				m.enterPlanEditor()
				m.infoMessage = fmt.Sprintf("Plan loaded: %d tasks in %d groups. Review and press [enter] to execute, or [esc] to cancel.",
					len(session.Plan.Tasks), len(session.Plan.ExecutionOrder))
			}
		}

		return m, nil

	case tickMsg:
		// Update outputs from instances
		m.updateOutputs()
		// Check for plan file during planning phase (proactive detection)
		m.checkForPlanFile()
		// Check for multi-pass plan files (most reliable detection for multi-pass mode)
		m.checkForMultiPassPlanFiles()
		// Check for plan manager plan file during plan selection phase (most reliable detection)
		m.checkForPlanManagerPlanFile()
		// Check for phase changes that need notification (synthesis, consolidation pause)
		m.checkForPhaseNotification()

		// Check if ultraplan needs user notification
		var cmds []tea.Cmd
		cmds = append(cmds, tick())
		if m.ultraPlan != nil && m.ultraPlan.needsNotification {
			m.ultraPlan.needsNotification = false
			cmds = append(cmds, notifyUser())
		}
		return m, tea.Batch(cmds...)

	case ultraPlanInitMsg:
		// Initialize ultra-plan mode by starting the planning phase
		if m.ultraPlan != nil && m.ultraPlan.coordinator != nil {
			session := m.ultraPlan.coordinator.Session()
			if session != nil && session.Phase == orchestrator.PhasePlanning && session.CoordinatorID == "" {
				if err := m.ultraPlan.coordinator.RunPlanning(); err != nil {
					m.errorMessage = fmt.Sprintf("Failed to start planning: %v", err)
				} else {
					m.infoMessage = "Planning started. Claude is analyzing the codebase..."
					// Select the coordinator instance so user can see the output
					for i, inst := range m.session.Instances {
						if inst.ID == session.CoordinatorID {
							m.activeTab = i
							break
						}
					}
				}
			}
		}
		return m, nil

	case outputMsg:
		m.outputManager.AppendOutput(msg.instanceID, string(msg.data))
		return m, nil

	case errMsg:
		m.errorMessage = msg.err.Error()
		return m, nil

	case prCompleteMsg:
		// PR workflow completed - remove the instance
		inst := m.session.GetInstance(msg.instanceID)
		if inst != nil {
			if err := m.orchestrator.RemoveInstance(m.session, msg.instanceID, true); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to remove instance after PR: %v", err)
			} else if msg.success {
				m.infoMessage = fmt.Sprintf("PR created and instance %s removed", msg.instanceID)
			} else {
				m.infoMessage = fmt.Sprintf("PR workflow finished (may have failed) - instance %s removed", msg.instanceID)
			}
		}
		return m, nil

	case prOpenedMsg:
		// PR URL detected in instance output - remove the instance
		inst := m.session.GetInstance(msg.instanceID)
		if inst != nil {
			if err := m.orchestrator.RemoveInstance(m.session, msg.instanceID, true); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to remove instance after PR opened: %v", err)
			} else {
				m.infoMessage = fmt.Sprintf("PR opened - instance %s removed", msg.instanceID)
			}
		}
		return m, nil

	case timeoutMsg:
		// Instance timeout detected - notify user
		inst := m.session.GetInstance(msg.instanceID)
		if inst != nil {
			var statusText string
			switch msg.timeoutType {
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

	case bellMsg:
		// Terminal bell detected in a tmux session - forward it to the parent terminal
		return m, ringBell()

	case taskAddedMsg:
		// Async task addition completed
		m.infoMessage = "" // Clear the "Adding task..." message
		if msg.err != nil {
			m.errorMessage = msg.err.Error()
		} else {
			// Switch to the newly added task and ensure it's visible in sidebar
			m.activeTab = len(m.session.Instances) - 1
			m.ensureActiveVisible()
		}
		return m, nil
	}

	return m, nil
}

// handleKeypress processes keyboard input
func (m Model) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode - typing search pattern
	if m.search.IsActive() {
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
					m.sendKeyToTmux(mgr, msg)
				}
			}
		}
		return m, nil
	}

	// Handle task input mode
	if m.addingTask {
		// Handle template dropdown if visible
		if m.templateHandler.IsVisible() {
			return m.handleTemplateDropdown(msg)
		}

		// Check for newline shortcuts (shift+enter, alt+enter, or ctrl+j)
		// Note: shift+enter only works in terminals that support extended keyboard
		// protocols (Kitty, iTerm2, WezTerm, Ghostty). Alt+Enter and Ctrl+J work
		// universally as fallbacks.
		if msg.Type == tea.KeyEnter && msg.Alt {
			m.taskInput.Insert("\n")
			return m, nil
		}
		if msg.String() == "shift+enter" {
			m.taskInput.Insert("\n")
			return m, nil
		}
		if msg.Type == tea.KeyCtrlJ {
			m.taskInput.Insert("\n")
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
			m.taskInput.MoveToPrevWord()
			return m, nil
		case "alt+right":
			// Opt+Right: Move to next word boundary
			m.taskInput.MoveToNextWord()
			return m, nil
		case "alt+up", "ctrl+a": // Cmd+Up often reported as ctrl+a in some terminals
			// Move to start of input
			m.taskInput.MoveToStart()
			return m, nil
		case "alt+down", "ctrl+e": // Cmd+Down often reported as ctrl+e in some terminals
			// Move to end of input
			m.taskInput.MoveToEnd()
			return m, nil
		case "alt+backspace", "ctrl+w":
			// Opt+Backspace: Delete previous word
			m.taskInput.DeleteWord()
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEsc:
			m.addingTask = false
			m.taskInput.Clear()
			return m, nil
		case tea.KeyEnter:
			if !m.taskInput.IsEmpty() {
				// Capture task and clear input state first
				task := m.taskInput.Buffer()
				m.addingTask = false
				m.taskInput.Clear()
				m.infoMessage = "Adding task..."
				// Add instance asynchronously to avoid blocking UI during git worktree creation
				return m, addTaskAsync(m.orchestrator, m.session, task)
			}
			m.addingTask = false
			m.taskInput.Clear()
			return m, nil
		case tea.KeyBackspace:
			m.taskInput.DeleteBack(1)
			return m, nil
		case tea.KeyDelete:
			m.taskInput.DeleteForward(1)
			return m, nil
		case tea.KeyLeft:
			m.taskInput.MoveCursor(-1)
			return m, nil
		case tea.KeyRight:
			m.taskInput.MoveCursor(1)
			return m, nil
		case tea.KeyHome:
			// Move to start of current line
			m.taskInput.MoveToLineStart()
			return m, nil
		case tea.KeyEnd:
			// Move to end of current line
			m.taskInput.MoveToLineEnd()
			return m, nil
		case tea.KeyCtrlU:
			// Cmd+Backspace equivalent: Delete from cursor to start of line
			m.taskInput.DeleteToLineStart()
			return m, nil
		case tea.KeyCtrlK:
			// Delete from cursor to end of line
			m.taskInput.DeleteToLineEnd()
			return m, nil
		case tea.KeySpace:
			m.taskInput.Insert(" ")
			return m, nil
		case tea.KeyRunes:
			char := string(msg.Runes)
			// Handle Enter sent as rune (some terminals/input methods send \n or \r as runes)
			if char == "\n" || char == "\r" {
				if !m.taskInput.IsEmpty() {
					// Capture task and clear input state first
					task := m.taskInput.Buffer()
					m.addingTask = false
					m.taskInput.Clear()
					m.infoMessage = "Adding task..."
					// Add instance asynchronously to avoid blocking UI during git worktree creation
					return m, addTaskAsync(m.orchestrator, m.session, task)
				}
				m.addingTask = false
				m.taskInput.Clear()
				return m, nil
			}
			// Detect "/" at start of input or after newline to show templates
			if char == "/" && m.taskInput.IsAtLineStart() {
				m.templateHandler.Show()
				m.taskInput.Insert(char)
				return m, nil
			}
			m.taskInput.Insert(char)
			return m, nil
		}
		return m, nil
	}

	// Handle command mode (vim-style ex commands with ':' prefix)
	// Check both the handler (if available) and legacy field for backward compatibility
	if (m.commandHandler != nil && m.commandHandler.IsActive()) || m.commandMode {
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

	switch msg.String() {
	case ":":
		// Enter command mode (vim-style)
		if m.commandHandler != nil {
			m.commandHandler.Enter()
		}
		// Sync legacy fields for backward compatibility
		m.commandMode = true
		m.commandBuffer = ""
		return m, nil

	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "tab", "l":
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab + 1) % m.instanceCount()
			m.ensureActiveVisible()
		}
		return m, nil

	case "shift+tab", "h":
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab - 1 + m.instanceCount()) % m.instanceCount()
			m.ensureActiveVisible()
		}
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '1')
		if idx < m.instanceCount() {
			m.activeTab = idx
			m.ensureActiveVisible()
		}
		return m, nil

	case "enter", "i":
		// Enter input mode for the active instance
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil && mgr.Running() {
				m.inputMode = true
			}
		}
		return m, nil

	case "esc":
		// Close diff panel if open
		if m.getDiffState().IsVisible() {
			m.getDiffState().Hide()
			return m, nil
		}
		return m, nil

	case "j", "down":
		// Scroll down in diff view, output view, or navigate to next instance
		if m.getDiffState().IsVisible() {
			maxLines := m.height - 14
			if maxLines < 5 {
				maxLines = 5
			}
			m.getDiffState().ScrollDown(1, m.getDiffState().CalculateMaxScroll(maxLines))
			return m, nil
		}
		if m.showHelp || m.showConflicts {
			// Don't scroll output when other panels are shown
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			// Scroll output down
			m.scrollOutputDown(inst.ID, 1)
			return m, nil
		}
		return m, nil

	case "k", "up":
		// Scroll up in diff view, output view, or navigate to previous instance
		if m.getDiffState().IsVisible() {
			m.getDiffState().ScrollUp(1)
			return m, nil
		}
		if m.showHelp || m.showConflicts {
			// Don't scroll output when other panels are shown
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			// Scroll output up
			m.scrollOutputUp(inst.ID, 1)
			return m, nil
		}
		return m, nil

	case "ctrl+u":
		// Scroll up half page in output view
		if m.getDiffState().IsVisible() || m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			halfPage := m.getOutputMaxLines() / 2
			m.scrollOutputUp(inst.ID, halfPage)
		}
		return m, nil

	case "ctrl+d":
		// Scroll down half page in output view
		if m.getDiffState().IsVisible() || m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			halfPage := m.getOutputMaxLines() / 2
			m.scrollOutputDown(inst.ID, halfPage)
		}
		return m, nil

	case "ctrl+b":
		// Scroll up full page in output view
		if m.getDiffState().IsVisible() || m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			fullPage := m.getOutputMaxLines()
			m.scrollOutputUp(inst.ID, fullPage)
		}
		return m, nil

	case "ctrl+f":
		// Scroll down full page in output view
		if m.getDiffState().IsVisible() || m.showHelp || m.showConflicts {
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
			}
		}
		return m, nil

	case "g":
		// Go to top of diff or output
		if m.getDiffState().IsVisible() {
			m.getDiffState().ScrollToTop()
			return m, nil
		}
		if m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			m.scrollOutputToTop(inst.ID)
		}
		return m, nil

	case "G":
		// Go to bottom of diff or output (re-enables auto-scroll)
		if m.getDiffState().IsVisible() {
			maxLines := m.height - 14
			if maxLines < 5 {
				maxLines = 5
			}
			m.getDiffState().ScrollToBottom(m.getDiffState().CalculateMaxScroll(maxLines))
			return m, nil
		}
		if m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			m.scrollOutputToBottom(inst.ID)
		}
		return m, nil

	case "/":
		// Enter search mode
		m.search.SetActive(true)
		m.search.Clear()
		return m, nil

	case "n":
		// Next search match
		if m.search.HasPattern() && m.search.HasMatches() {
			m.search.NextMatch()
			m.scrollToMatch()
		}
		return m, nil

	case "N":
		// Previous search match
		if m.search.HasPattern() && m.search.HasMatches() {
			m.search.PrevMatch()
			m.scrollToMatch()
		}
		return m, nil

	case "ctrl+/":
		// Clear search
		m.clearSearch()
		return m, nil
	}

	return m, nil
}


// sendKeyToTmux sends a key event to the tmux session
func (m Model) sendKeyToTmux(mgr *instance.Manager, msg tea.KeyMsg) {
	var key string
	literal := false

	switch msg.Type {
	// Basic keys
	case tea.KeyEnter:
		key = "Enter"
	case tea.KeyBackspace:
		key = "BSpace"
	case tea.KeyTab:
		key = "Tab"
	case tea.KeyShiftTab:
		key = "BTab" // Back-tab in tmux
	case tea.KeySpace:
		key = " " // Send literal space
		literal = true
	case tea.KeyEsc:
		key = "Escape"

	// Arrow keys
	case tea.KeyUp:
		key = "Up"
	case tea.KeyDown:
		key = "Down"
	case tea.KeyRight:
		key = "Right"
	case tea.KeyLeft:
		key = "Left"

	// Navigation keys
	case tea.KeyPgUp:
		key = "PageUp"
	case tea.KeyPgDown:
		key = "PageDown"
	case tea.KeyHome:
		key = "Home"
	case tea.KeyEnd:
		key = "End"
	case tea.KeyDelete:
		key = "DC" // Delete character in tmux
	case tea.KeyInsert:
		key = "IC" // Insert character in tmux

	// All Ctrl+letter combinations (Claude Code uses many of these)
	case tea.KeyCtrlA:
		key = "C-a"
	case tea.KeyCtrlB:
		key = "C-b"
	case tea.KeyCtrlC:
		key = "C-c"
	case tea.KeyCtrlD:
		key = "C-d"
	case tea.KeyCtrlE:
		key = "C-e"
	case tea.KeyCtrlF:
		key = "C-f"
	case tea.KeyCtrlG:
		key = "C-g"
	case tea.KeyCtrlH:
		key = "C-h"
	// Note: KeyCtrlI (ASCII 9) is the same as KeyTab - handled above
	case tea.KeyCtrlJ:
		key = "C-j" // Note: also used for newline in some contexts
	case tea.KeyCtrlK:
		key = "C-k"
	case tea.KeyCtrlL:
		key = "C-l"
	// Note: KeyCtrlM (ASCII 13) is the same as KeyEnter - handled above
	case tea.KeyCtrlN:
		key = "C-n"
	case tea.KeyCtrlO:
		key = "C-o" // Used by Claude Code for file operations
	case tea.KeyCtrlP:
		key = "C-p"
	case tea.KeyCtrlQ:
		key = "C-q"
	case tea.KeyCtrlR:
		key = "C-r" // Used by Claude Code for reverse search
	case tea.KeyCtrlS:
		key = "C-s"
	case tea.KeyCtrlT:
		key = "C-t"
	case tea.KeyCtrlU:
		key = "C-u" // Used by Claude Code to clear line
	case tea.KeyCtrlV:
		key = "C-v"
	case tea.KeyCtrlW:
		key = "C-w" // Used by Claude Code to delete word
	case tea.KeyCtrlX:
		key = "C-x"
	case tea.KeyCtrlY:
		key = "C-y"
	case tea.KeyCtrlZ:
		key = "C-z"

	// Function keys (F1-F12)
	case tea.KeyF1:
		key = "F1"
	case tea.KeyF2:
		key = "F2"
	case tea.KeyF3:
		key = "F3"
	case tea.KeyF4:
		key = "F4"
	case tea.KeyF5:
		key = "F5"
	case tea.KeyF6:
		key = "F6"
	case tea.KeyF7:
		key = "F7"
	case tea.KeyF8:
		key = "F8"
	case tea.KeyF9:
		key = "F9"
	case tea.KeyF10:
		key = "F10"
	case tea.KeyF11:
		key = "F11"
	case tea.KeyF12:
		key = "F12"

	case tea.KeyRunes:
		// Send literal characters
		// Handle Alt+key combinations
		if msg.Alt {
			// For alt combinations, tmux uses M- prefix or Escape followed by char
			key = string(msg.Runes)
			mgr.SendKey("Escape") // Send escape first
			mgr.SendLiteral(key)  // Then send the character
			return
		}
		key = string(msg.Runes)
		literal = true

	default:
		// Try to handle other keys by their string representation
		keyStr := msg.String()

		// Handle known string patterns that might not have direct KeyType
		switch {
		case strings.HasPrefix(keyStr, "shift+"):
			// Try to map shift combinations
			baseKey := strings.TrimPrefix(keyStr, "shift+")
			switch baseKey {
			case "up":
				key = "S-Up"
			case "down":
				key = "S-Down"
			case "left":
				key = "S-Left"
			case "right":
				key = "S-Right"
			case "home":
				key = "S-Home"
			case "end":
				key = "S-End"
			default:
				key = keyStr
			}
		case strings.HasPrefix(keyStr, "alt+"):
			// Alt combinations: send Escape then the key
			baseKey := strings.TrimPrefix(keyStr, "alt+")
			mgr.SendKey("Escape")
			if len(baseKey) == 1 {
				mgr.SendLiteral(baseKey)
			} else {
				mgr.SendKey(baseKey)
			}
			return
		case strings.HasPrefix(keyStr, "ctrl+"):
			// Try to handle ctrl combinations not caught above
			baseKey := strings.TrimPrefix(keyStr, "ctrl+")
			if len(baseKey) == 1 {
				key = "C-" + baseKey
			} else {
				key = keyStr
			}
		default:
			key = keyStr
			if len(key) == 1 {
				literal = true
			}
		}
	}

	if key != "" {
		if literal {
			mgr.SendLiteral(key)
		} else {
			mgr.SendKey(key)
		}
	}
}

// handleTemplateDropdown handles keyboard input when the template dropdown is visible.
// It delegates to TemplateHandler and handles the result by updating task input accordingly.
func (m Model) handleTemplateDropdown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	result := m.templateHandler.HandleKey(msg, m.taskInput.Buffer())

	if !result.Handled {
		return m, nil
	}

	// Handle template selection - replace "/" and filter with template description
	if result.SelectedTemplate != nil && result.InputReplace {
		newInput := m.templateHandler.ComputeReplacementInput(m.taskInput.Buffer(), result.SelectedTemplate)
		m.taskInput.SetBuffer(newInput)
		m.taskInput.MoveToEnd()
		return m, nil
	}

	// Handle filter changes - sync taskInput with filter
	if result.FilterChanged {
		if msg.Type == tea.KeyBackspace {
			// Remove character from taskInput
			m.taskInput.DeleteBack(1)
		} else if msg.Type == tea.KeyRunes {
			// Add character to taskInput
			m.taskInput.Insert(string(msg.Runes))
		}
		return m, nil
	}

	// Handle space - append space to input
	if result.InputAppend != "" {
		m.taskInput.Insert(result.InputAppend)
		return m, nil
	}

	return m, nil
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
	case instance.StateWaitingPermission, instance.StateWaitingQuestion, instance.StateWaitingInput:
		inst.Status = orchestrator.StatusWaitingInput
	case instance.StateCompleted:
		inst.Status = orchestrator.StatusCompleted
		// If just completed (status changed), check completion action
		if previousStatus != orchestrator.StatusCompleted {
			m.handleInstanceCompleted(inst)
		}
	case instance.StateError:
		inst.Status = orchestrator.StatusError
	case instance.StateWorking:
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

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	var b strings.Builder

	// Header - use ultra-plan header if in ultra-plan mode
	var header string
	if m.IsUltraPlanMode() {
		header = m.renderUltraPlanHeader()
	} else {
		header = m.renderHeader()
	}
	b.WriteString(header)
	b.WriteString("\n")

	// Calculate widths for sidebar and main content
	effectiveSidebarWidth := SidebarWidth
	if m.width < 80 {
		effectiveSidebarWidth = SidebarMinWidth
	}
	mainContentWidth := m.width - effectiveSidebarWidth - 3 // 3 for gap between panels

	// Calculate available height for the main area
	mainAreaHeight := m.height - 6 // Header + help bar + margins

	// Sidebar + Content area (horizontal layout)
	// Use ultra-plan specific rendering if in ultra-plan mode
	var sidebar, content string
	if m.IsUltraPlanMode() {
		sidebar = m.renderUltraPlanSidebar(effectiveSidebarWidth, mainAreaHeight)
		content = m.renderUltraPlanContent(mainContentWidth)
	} else {
		sidebar = m.renderSidebar(effectiveSidebarWidth, mainAreaHeight)
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
	b.WriteString("\n")
	if m.IsPlanEditorActive() {
		b.WriteString(m.renderPlanEditorHelp())
	} else if m.IsUltraPlanMode() {
		b.WriteString(m.renderUltraPlanHelp())
	} else {
		b.WriteString(m.renderHelp())
	}

	return b.String()
}

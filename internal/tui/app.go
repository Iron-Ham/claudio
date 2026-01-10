package tui

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
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

// Layout constants
const (
	SidebarWidth    = 30 // Fixed sidebar width
	SidebarMinWidth = 20 // Minimum sidebar width

	// Layout offsets for content area calculation
	ContentWidthOffset  = 7  // sidebar gap (3) + output area margin (4)
	ContentHeightOffset = 12 // header + help bar + instance info + task + status + scroll indicator
)

// CalculateContentDimensions returns the effective content area dimensions
// given the terminal width and height. This accounts for the sidebar and other UI elements.
func CalculateContentDimensions(termWidth, termHeight int) (contentWidth, contentHeight int) {
	effectiveSidebarWidth := SidebarWidth
	if termWidth < 80 {
		effectiveSidebarWidth = SidebarMinWidth
	}
	contentWidth = termWidth - effectiveSidebarWidth - ContentWidthOffset
	contentHeight = termHeight - ContentHeightOffset
	return contentWidth, contentHeight
}

// Messages

type tickMsg time.Time
type outputMsg struct {
	instanceID string
	data       []byte
}
type errMsg struct {
	err error
}
type prCompleteMsg struct {
	instanceID string
	success    bool
}

type prOpenedMsg struct {
	instanceID string
}

type timeoutMsg struct {
	instanceID  string
	timeoutType instance.TimeoutType
}

type bellMsg struct {
	instanceID string
}

// taskAddedMsg is sent when async task addition completes
type taskAddedMsg struct {
	instance *orchestrator.Instance
	err      error
}

// Commands

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// ringBell returns a command that outputs a terminal bell character
// This forwards bells from tmux sessions to the parent terminal
func ringBell() tea.Cmd {
	return func() tea.Msg {
		// Write the bell character directly to stdout
		// This works even when Bubbletea is in alt-screen mode
		_, _ = os.Stdout.Write([]byte{'\a'})
		return nil
	}
}

// notifyUser returns a command that notifies the user via bell and optional sound
// Used to alert the user when ultraplan needs input (e.g., plan ready, synthesis ready)
func notifyUser() tea.Cmd {
	return func() tea.Msg {
		if !viper.GetBool("ultraplan.notifications.enabled") {
			return nil
		}

		// Always ring terminal bell
		_, _ = os.Stdout.Write([]byte{'\a'})

		// Optionally play system sound on macOS
		if runtime.GOOS == "darwin" && viper.GetBool("ultraplan.notifications.use_sound") {
			soundPath := viper.GetString("ultraplan.notifications.sound_path")
			if soundPath == "" {
				soundPath = "/System/Library/Sounds/Glass.aiff"
			}
			// Start in background so it doesn't block
			_ = exec.Command("afplay", soundPath).Start()
		}
		return nil
	}
}

// addTaskAsync returns a command that adds a task asynchronously
// This prevents the UI from blocking while git creates the worktree
func addTaskAsync(o *orchestrator.Orchestrator, session *orchestrator.Session, task string) tea.Cmd {
	return func() tea.Msg {
		inst, err := o.AddInstance(session, task)
		return taskAddedMsg{instance: inst, err: err}
	}
}

// ultraPlanInitMsg signals that ultra-plan mode should initialize
type ultraPlanInitMsg struct{}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}

	// Schedule ultra-plan initialization if needed
	if m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
		session := m.ultraPlan.Coordinator.Session()
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
		if !wasReady && m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
			session := m.ultraPlan.Coordinator.Session()
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
		if m.ultraPlan != nil && m.ultraPlan.NeedsNotification {
			m.ultraPlan.NeedsNotification = false
			cmds = append(cmds, notifyUser())
		}
		return m, tea.Batch(cmds...)

	case ultraPlanInitMsg:
		// Initialize ultra-plan mode by starting the planning phase
		if m.ultraPlan != nil && m.ultraPlan.Coordinator != nil {
			session := m.ultraPlan.Coordinator.Session()
			if session != nil && session.Phase == orchestrator.PhasePlanning && session.CoordinatorID == "" {
				if err := m.ultraPlan.Coordinator.RunPlanning(); err != nil {
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
		if m.outputs == nil {
			m.outputs = make(map[string]string)
		}
		m.outputs[msg.instanceID] += string(msg.data)
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
		// PR URL detected in instance output - notify user but keep instance for potential review tools
		inst := m.session.GetInstance(msg.instanceID)
		if inst != nil {
			m.infoMessage = fmt.Sprintf("PR opened for instance %s - use :D to remove or run review tools", inst.ID)
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
			// Log user adding instance
			if m.logger != nil && msg.instance != nil {
				m.logger.Info("user added instance", "task", msg.instance.Task)
			}
		}
		return m, nil
	}

	return m, nil
}

// handleKeypress processes keyboard input
func (m Model) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
					m.sendKeyToTmux(mgr, msg)
				}
			}
		}
		return m, nil
	}

	// Handle task input mode
	if m.addingTask {
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
			m.taskInput = ""
			m.taskInputCursor = 0
			m.templateSuffix = "" // Clear suffix on cancel
			return m, nil
		case tea.KeyEnter:
			if m.taskInput != "" {
				// Capture task and clear input state first
				// Append template suffix if one was set (e.g., /plan instructions)
				task := m.taskInput + m.templateSuffix
				m.addingTask = false
				m.taskInput = ""
				m.taskInputCursor = 0
				m.templateSuffix = "" // Clear suffix after use
				m.infoMessage = "Adding task..."
				// Add instance asynchronously to avoid blocking UI during git worktree creation
				return m, addTaskAsync(m.orchestrator, m.session, task)
			}
			m.addingTask = false
			m.taskInput = ""
			m.taskInputCursor = 0
			m.templateSuffix = "" // Clear suffix on cancel
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
		case tea.KeyRunes:
			char := string(msg.Runes)
			// Handle Enter sent as rune (some terminals/input methods send \n or \r as runes)
			if char == "\n" || char == "\r" {
				if m.taskInput != "" {
					// Capture task and clear input state first
					task := m.taskInput
					m.addingTask = false
					m.taskInput = ""
					m.taskInputCursor = 0
					m.infoMessage = "Adding task..."
					// Add instance asynchronously to avoid blocking UI during git worktree creation
					return m, addTaskAsync(m.orchestrator, m.session, task)
				}
				m.addingTask = false
				m.taskInput = ""
				m.taskInputCursor = 0
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

	switch msg.String() {
	case ":":
		// Enter command mode (vim-style)
		m.commandMode = true
		m.commandBuffer = ""
		return m, nil

	case "q", "ctrl+c":
		m.quitting = true
		// Log session end with duration
		if m.logger != nil {
			duration := time.Since(m.startTime)
			m.logger.Info("TUI session ended", "duration_ms", duration.Milliseconds())
		}
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "tab", "l":
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab + 1) % m.instanceCount()
			m.ensureActiveVisible()
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
			m.activeTab = (m.activeTab - 1 + m.instanceCount()) % m.instanceCount()
			m.ensureActiveVisible()
			// Log focus change
			if m.logger != nil {
				if inst := m.activeInstance(); inst != nil {
					m.logger.Info("user focused instance", "instance_id", inst.ID)
				}
			}
		}
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '1')
		if idx < m.instanceCount() {
			m.activeTab = idx
			m.ensureActiveVisible()
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
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil && mgr.Running() {
				m.inputMode = true
			}
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
		// Scroll down in diff view, output view, or navigate to next instance
		if m.showDiff {
			m.diffScroll++
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
		if m.showDiff {
			if m.diffScroll > 0 {
				m.diffScroll--
			}
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
		if m.showDiff || m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			halfPage := m.getOutputMaxLines() / 2
			m.scrollOutputUp(inst.ID, halfPage)
		}
		return m, nil

	case "ctrl+d":
		// Scroll down half page in output view
		if m.showDiff || m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			halfPage := m.getOutputMaxLines() / 2
			m.scrollOutputDown(inst.ID, halfPage)
		}
		return m, nil

	case "ctrl+b":
		// Scroll up full page in output view
		if m.showDiff || m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			fullPage := m.getOutputMaxLines()
			m.scrollOutputUp(inst.ID, fullPage)
		}
		return m, nil

	case "ctrl+f":
		// Scroll down full page in output view
		if m.showDiff || m.showHelp || m.showConflicts {
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
		if m.showDiff {
			m.diffScroll = 0
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
		if m.showDiff {
			lines := strings.Split(m.diffContent, "\n")
			maxLines := m.height - 14
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
		if m.showHelp || m.showConflicts {
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
			m.scrollOutputToBottom(inst.ID)
		}
		return m, nil

	case "/":
		// Enter search mode
		m.searchMode = true
		m.searchPattern = ""
		m.searchMatches = nil
		m.searchCurrent = 0
		return m, nil

	case "n":
		// Next search match
		if m.searchPattern != "" && len(m.searchMatches) > 0 {
			m.searchCurrent = (m.searchCurrent + 1) % len(m.searchMatches)
			m.scrollToMatch()
		}
		return m, nil

	case "N":
		// Previous search match
		if m.searchPattern != "" && len(m.searchMatches) > 0 {
			m.searchCurrent = (m.searchCurrent - 1 + len(m.searchMatches)) % len(m.searchMatches)
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
	// Trim whitespace
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return m, nil
	}

	// Clear messages before executing
	m.infoMessage = ""
	m.errorMessage = ""

	// Parse command (support both short and long forms)
	switch cmd {
	// Instance control commands
	case "s", "start":
		return m.cmdStart()
	case "x", "stop":
		return m.cmdStop()
	case "e", "exit":
		return m.cmdExit()
	case "p", "pause":
		return m.cmdPause()
	case "R", "reconnect":
		return m.cmdReconnect()
	case "restart":
		return m.cmdRestart()

	// Instance management commands
	case "a", "add":
		return m.cmdAdd()
	case "D", "remove":
		return m.cmdRemove()
	case "kill":
		return m.cmdKill()
	case "C", "clear":
		return m.cmdClearCompleted()

	// View toggle commands
	case "d", "diff":
		return m.cmdDiff()
	case "m", "metrics", "stats":
		return m.cmdStats()
	case "c", "conflicts":
		return m.cmdConflicts()
	case "f", "F", "filter":
		return m.cmdFilter()

	// Utility commands
	case "t", "tmux":
		return m.cmdTmux()
	case "r", "pr":
		return m.cmdPR()

	// Help commands
	case "h", "help":
		m.showHelp = !m.showHelp
		return m, nil
	case "q", "quit":
		m.quitting = true
		// Log session end with duration
		if m.logger != nil {
			duration := time.Since(m.startTime)
			m.logger.Info("TUI session ended", "duration_ms", duration.Milliseconds())
		}
		return m, tea.Quit

	default:
		m.errorMessage = fmt.Sprintf("Unknown command: %s (type :h for help)", cmd)
		return m, nil
	}
}

// Command implementations

func (m Model) cmdStart() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	// Guard against starting already-running instances
	if inst.Status == orchestrator.StatusWorking || inst.Status == orchestrator.StatusWaitingInput {
		m.infoMessage = "Instance is already running. Use :p to pause/resume or :x to stop."
		return m, nil
	}
	if inst.Status == orchestrator.StatusCreatingPR {
		m.infoMessage = "Instance is creating PR. Wait for it to complete."
		return m, nil
	}

	if err := m.orchestrator.StartInstance(inst); err != nil {
		m.errorMessage = err.Error()
	} else {
		m.infoMessage = fmt.Sprintf("Started instance %s", inst.ID)
	}
	return m, nil
}

func (m Model) cmdStop() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	// Log user stopping instance
	if m.logger != nil {
		m.logger.Info("user stopped instance", "instance_id", inst.ID)
	}

	prStarted, err := m.orchestrator.StopInstanceWithAutoPR(inst)
	if err != nil {
		m.errorMessage = err.Error()
	} else if prStarted {
		m.infoMessage = fmt.Sprintf("Instance stopped. Creating PR for %s...", inst.ID)
	} else {
		m.infoMessage = fmt.Sprintf("Instance stopped. Create PR with: claudio pr %s", inst.ID)
	}
	return m, nil
}

func (m Model) cmdExit() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	// Log user exiting instance
	if m.logger != nil {
		m.logger.Info("user exited instance (no auto-PR)", "instance_id", inst.ID)
	}

	// Stop without auto-PR workflow
	if err := m.orchestrator.StopInstance(inst); err != nil {
		m.errorMessage = err.Error()
	} else {
		m.infoMessage = fmt.Sprintf("Instance %s stopped (no PR workflow). Create PR manually with: claudio pr %s", inst.ID, inst.ID)
	}
	return m, nil
}

func (m Model) cmdPause() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	mgr := m.orchestrator.GetInstanceManager(inst.ID)
	if mgr == nil {
		m.infoMessage = "Instance has no manager"
		return m, nil
	}

	switch inst.Status {
	case orchestrator.StatusPaused:
		_ = mgr.Resume()
		inst.Status = orchestrator.StatusWorking
		m.infoMessage = fmt.Sprintf("Resumed instance %s", inst.ID)
	case orchestrator.StatusWorking:
		_ = mgr.Pause()
		inst.Status = orchestrator.StatusPaused
		m.infoMessage = fmt.Sprintf("Paused instance %s", inst.ID)
	default:
		m.infoMessage = "Instance is not in a pausable state"
	}
	return m, nil
}

func (m Model) cmdReconnect() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	// Only allow reconnecting to non-running instances
	if inst.Status == orchestrator.StatusWorking || inst.Status == orchestrator.StatusWaitingInput {
		m.infoMessage = "Instance is already running. Use :p to pause/resume or :x to stop."
		return m, nil
	}
	if inst.Status == orchestrator.StatusCreatingPR {
		m.infoMessage = "Instance is creating PR. Wait for it to complete."
		return m, nil
	}

	if err := m.orchestrator.ReconnectInstance(inst); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to reconnect: %v", err)
	} else {
		m.infoMessage = fmt.Sprintf("Reconnected to instance %s", inst.ID)
	}
	return m, nil
}

func (m Model) cmdRestart() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	// Only allow restarting non-running instances
	switch inst.Status {
	case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
		m.infoMessage = "Instance is running. Use :x to stop it first, or :p to pause."
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
	return m, nil
}

func (m Model) cmdAdd() (tea.Model, tea.Cmd) {
	m.addingTask = true
	m.taskInput = ""
	m.taskInputCursor = 0
	return m, nil
}

func (m Model) cmdRemove() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	instanceID := inst.ID
	if err := m.orchestrator.RemoveInstance(m.session, instanceID, true); err != nil {
		m.errorMessage = fmt.Sprintf("Failed to remove instance: %v", err)
	} else {
		m.infoMessage = fmt.Sprintf("Removed instance %s", instanceID)
		// Adjust active tab if needed
		if m.activeTab >= m.instanceCount() {
			m.activeTab = m.instanceCount() - 1
			if m.activeTab < 0 {
				m.activeTab = 0
			}
		}
		m.ensureActiveVisible()
	}
	return m, nil
}

func (m Model) cmdKill() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

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
	return m, nil
}

func (m Model) cmdClearCompleted() (tea.Model, tea.Cmd) {
	removed, err := m.orchestrator.ClearCompletedInstances(m.session)
	if err != nil {
		m.errorMessage = err.Error()
	} else if removed == 0 {
		m.infoMessage = "No completed instances to clear"
	} else {
		m.infoMessage = fmt.Sprintf("Cleared %d completed instance(s)", removed)
		// Adjust active tab if needed
		if m.activeTab >= m.instanceCount() {
			m.activeTab = m.instanceCount() - 1
			if m.activeTab < 0 {
				m.activeTab = 0
			}
		}
		m.ensureActiveVisible()
	}
	return m, nil
}

func (m Model) cmdDiff() (tea.Model, tea.Cmd) {
	if m.showDiff {
		m.showDiff = false
		m.diffContent = ""
		m.diffScroll = 0
		return m, nil
	}

	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	diff, err := m.orchestrator.GetInstanceDiff(inst.WorktreePath)
	if err != nil {
		m.errorMessage = fmt.Sprintf("Failed to get diff: %v", err)
	} else if diff == "" {
		m.infoMessage = "No changes to show"
	} else {
		m.diffContent = diff
		m.showDiff = true
		m.diffScroll = 0
	}
	return m, nil
}

func (m Model) cmdStats() (tea.Model, tea.Cmd) {
	m.showStats = !m.showStats
	return m, nil
}

func (m Model) cmdConflicts() (tea.Model, tea.Cmd) {
	if len(m.conflicts) > 0 {
		m.showConflicts = !m.showConflicts
	} else {
		m.infoMessage = "No conflicts detected"
	}
	return m, nil
}

func (m Model) cmdFilter() (tea.Model, tea.Cmd) {
	m.filterMode = true
	return m, nil
}

func (m Model) cmdTmux() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	mgr := m.orchestrator.GetInstanceManager(inst.ID)
	if mgr == nil {
		m.infoMessage = "Instance has no manager"
		return m, nil
	}

	m.infoMessage = "Attach with: " + mgr.AttachCommand()
	return m, nil
}

func (m Model) cmdPR() (tea.Model, tea.Cmd) {
	inst := m.activeInstance()
	if inst == nil {
		m.infoMessage = "No instance selected"
		return m, nil
	}

	m.infoMessage = fmt.Sprintf("Create PR: claudio pr %s  (add --draft for draft PR)", inst.ID)
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

// handleTemplateDropdown handles keyboard input when the template dropdown is visible
func (m Model) handleTemplateDropdown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	templates := FilterTemplates(m.templateFilter)

	switch msg.Type {
	case tea.KeyEsc:
		// Close dropdown but keep the "/" and filter in input
		m.showTemplates = false
		m.templateFilter = ""
		m.templateSelected = 0
		return m, nil

	case tea.KeyEnter, tea.KeyTab:
		// Select the highlighted template
		if len(templates) > 0 && m.templateSelected < len(templates) {
			selected := templates[m.templateSelected]
			// Replace the "/" and filter with the template description
			// Find where the "/" starts (could be at beginning or after newline)
			lastNewline := strings.LastIndex(m.taskInput, "\n")
			if lastNewline == -1 {
				// "/" is at the beginning
				m.taskInput = selected.Description
			} else {
				// "/" is after a newline
				m.taskInput = m.taskInput[:lastNewline+1] + selected.Description
			}
			m.taskInputCursor = len([]rune(m.taskInput))
			// Store the suffix to append on submission
			m.templateSuffix = selected.Suffix
		}
		m.showTemplates = false
		m.templateFilter = ""
		m.templateSelected = 0
		return m, nil

	case tea.KeyUp:
		if m.templateSelected > 0 {
			m.templateSelected--
		}
		return m, nil

	case tea.KeyDown:
		if m.templateSelected < len(templates)-1 {
			m.templateSelected++
		}
		return m, nil

	case tea.KeyBackspace:
		if len(m.templateFilter) > 0 {
			// Remove from both filter and taskInput
			m.templateFilter = m.templateFilter[:len(m.templateFilter)-1]
			if len(m.taskInput) > 0 {
				m.taskInput = m.taskInput[:len(m.taskInput)-1]
				m.taskInputCursor = len([]rune(m.taskInput))
			}
			m.templateSelected = 0 // Reset selection on filter change
		} else {
			// Remove the "/" and close dropdown
			if len(m.taskInput) > 0 {
				m.taskInput = m.taskInput[:len(m.taskInput)-1]
				m.taskInputCursor = len([]rune(m.taskInput))
			}
			m.showTemplates = false
		}
		return m, nil

	case tea.KeyRunes:
		char := string(msg.Runes)
		// Space closes dropdown and keeps current input, adds space
		if char == " " {
			m.showTemplates = false
			m.taskInput += " "
			m.taskInputCursor = len([]rune(m.taskInput))
			m.templateFilter = ""
			m.templateSelected = 0
			return m, nil
		}
		// Add to both filter and taskInput
		m.templateFilter += char
		m.taskInput += char
		m.taskInputCursor = len([]rune(m.taskInput))
		m.templateSelected = 0 // Reset selection on filter change
		// If no templates match, close dropdown
		if len(FilterTemplates(m.templateFilter)) == 0 {
			m.showTemplates = false
			m.templateFilter = ""
		}
		return m, nil
	}

	return m, nil
}

// handleSearchInput handles keyboard input when in search mode
func (m Model) handleSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		// Cancel search mode (keep existing pattern if any)
		m.searchMode = false
		return m, nil

	case tea.KeyEnter:
		// Execute search and exit search mode
		m.executeSearch()
		m.searchMode = false
		return m, nil

	case tea.KeyBackspace:
		if len(m.searchPattern) > 0 {
			m.searchPattern = m.searchPattern[:len(m.searchPattern)-1]
			// Live search as user types
			m.executeSearch()
		}
		return m, nil

	case tea.KeyRunes:
		m.searchPattern += string(msg.Runes)
		// Live search as user types
		m.executeSearch()
		return m, nil

	case tea.KeySpace:
		m.searchPattern += " "
		m.executeSearch()
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

// executeSearch compiles the search pattern and finds all matches
func (m *Model) executeSearch() {
	if m.searchPattern == "" {
		m.searchMatches = nil
		m.searchRegex = nil
		return
	}

	inst := m.activeInstance()
	if inst == nil {
		return
	}

	output := m.outputs[inst.ID]
	if output == "" {
		return
	}

	// Try to compile as regex if it starts with r:
	if strings.HasPrefix(m.searchPattern, "r:") {
		pattern := m.searchPattern[2:]
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			m.searchRegex = nil
			m.searchMatches = nil
			return
		}
		m.searchRegex = re
	} else {
		// Literal search (case-insensitive)
		m.searchRegex = regexp.MustCompile("(?i)" + regexp.QuoteMeta(m.searchPattern))
	}

	// Find all matching lines
	lines := strings.Split(output, "\n")
	m.searchMatches = nil
	for i, line := range lines {
		if m.searchRegex.MatchString(line) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}

	// Set current match
	if len(m.searchMatches) > 0 {
		m.searchCurrent = 0
		m.scrollToMatch()
	}
}

// clearSearch clears the current search
func (m *Model) clearSearch() {
	m.searchPattern = ""
	m.searchRegex = nil
	m.searchMatches = nil
	m.searchCurrent = 0
	m.outputScroll = 0
}

// scrollToMatch adjusts output scroll to show the current match
func (m *Model) scrollToMatch() {
	if len(m.searchMatches) == 0 || m.searchCurrent >= len(m.searchMatches) {
		return
	}

	matchLine := m.searchMatches[m.searchCurrent]
	maxLines := m.height - 12
	if maxLines < 5 {
		maxLines = 5
	}

	// Center the match in the visible area
	m.outputScroll = matchLine - maxLines/2
	if m.outputScroll < 0 {
		m.outputScroll = 0
	}
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
					m.outputs[inst.ID] = string(output)
				}
			}
			continue
		}

		mgr := m.orchestrator.GetInstanceManager(inst.ID)
		if mgr != nil {
			output := mgr.GetOutput()
			if len(output) > 0 {
				m.outputs[inst.ID] = string(output)
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
		// Use view component for sidebar rendering
		dashboardView := view.NewDashboardView()
		sidebar = dashboardView.RenderSidebar(m, effectiveSidebarWidth, mainAreaHeight)
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

// renderHeader renders the header bar
func (m Model) renderHeader() string {
	title := "Claudio"
	if m.session != nil && m.session.Name != "" {
		title = fmt.Sprintf("Claudio: %s", m.session.Name)
	}

	return styles.Header.Width(m.width).Render(title)
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
			"No instance selected.\n\nPress [a] to add a new Claude instance.",
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
	output := m.outputs[inst.ID]
	if output != "" {
		output = m.filterOutput(output)
	}

	renderState := view.RenderState{
		Output:            output,
		IsRunning:         isRunning,
		InputMode:         m.inputMode,
		ScrollOffset:      m.outputScrolls[inst.ID],
		AutoScrollEnabled: m.isOutputAutoScroll(inst.ID),
		HasNewOutput:      m.hasNewOutput(inst.ID),
		SearchPattern:     m.searchPattern,
		SearchRegex:       m.searchRegex,
		SearchMatches:     m.searchMatches,
		SearchCurrent:     m.searchCurrent,
		SearchMode:        m.searchMode,
	}

	instanceView := view.NewInstanceView(width, m.getOutputMaxLines())
	return instanceView.Render(inst, renderState)
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
		Text:             m.taskInput,
		Cursor:           m.taskInputCursor,
		ShowTemplates:    m.showTemplates,
		Templates:        m.buildTemplateItems(),
		TemplateSelected: m.templateSelected,
	}
	inputView := view.NewInputView()
	return inputView.Render(inputState, width)
}

// buildTemplateItems converts filtered templates to view template items
func (m Model) buildTemplateItems() []view.TemplateItem {
	templates := FilterTemplates(m.templateFilter)
	items := make([]view.TemplateItem, len(templates))
	for i, t := range templates {
		items[i] = view.TemplateItem{
			Command: t.Command,
			Name:    t.Name,
		}
	}
	return items
}

// renderHelpPanel renders the help overlay
func (m Model) renderHelpPanel(width int) string {
	help := `
Claudio uses vim-style commands. Press : to enter command mode.

Navigation (always available):
  Tab / l      Next instance
  Shift+Tab/h  Previous instance
  1-9          Select instance by number
  j / ↓        Scroll down one line
  k / ↑        Scroll up one line
  Ctrl+D/U     Scroll half page down/up
  Ctrl+F/B     Scroll full page down/up
  g / G        Jump to top / bottom

Instance Commands (press : first, then type command):
  :s :start      Start a stopped/new instance (runs Claude)
  :x :stop       Stop instance and trigger auto-PR workflow (if enabled)
  :e :exit       Stop instance without auto-PR (for manual PR later)
  :p :pause      Pause/resume a running instance (SIGSTOP/SIGCONT)
  :R :reconnect  Reattach to a stopped instance's tmux session
  :restart       Restart a stuck or timed-out instance
  :a :add        Create and add a new instance to the session
  :D :remove     Remove instance from session (keeps branch)
  :kill          Force kill instance process and remove from session
  :C :clear      Remove all completed instances from the list

View Commands:
  :d :diff       Toggle diff preview panel (show git changes)
  :m :stats      Toggle metrics panel (tokens, cost, timing)
  :c :conflicts  Toggle conflict view (show merge conflicts)
  :f :filter     Open filter panel (filter output by category)
  :t :tmux       Show tmux attach command for direct access
  :r :pr         Show PR creation command for the instance

Session Commands:
  :h :help       Toggle this help panel
  :q :quit       Quit Claudio (instances continue in tmux)

Input Mode:
  i / Enter      Enter input mode (interact with Claude)
  Ctrl+]         Exit input mode back to navigation

Search (vim-style):
  /              Start search (type pattern, Enter to confirm)
  n / N          Next / previous match
  Ctrl+/         Clear search
  r:pattern      Use regex (prefix with r:)

Search Tips:
  • Search is case-insensitive by default
  • Use r: prefix for regex (e.g. /r:error.*file)
  • Matches highlighted in yellow, current in orange

Filter Categories (toggle in filter panel :f):
  • Errors: Stack traces, error messages
  • Warnings: Warning indicators
  • Tool calls: File operations, bash commands
  • Thinking: Claude's reasoning
  • Progress: Progress indicators

Input Mode Details:
  All keystrokes are forwarded to Claude, including:
  • Ctrl+key combinations (Ctrl+O, Ctrl+R, Ctrl+W, etc.)
  • Function keys (F1-F12)
  • Navigation keys (Page Up/Down, Home, End)
  • Pasted text with bracketed paste support
  Press Ctrl+] to return to navigation mode.

General:
  ?              Quick toggle help
  q              Quick quit
  Auto-scroll follows new output. Scroll up to pause,
  press G to resume. "NEW OUTPUT" appears when paused.
`
	return styles.ContentBox.Width(width - 4).Render(help)
}

// renderDiffPanel renders the diff preview panel with syntax highlighting
func (m Model) renderDiffPanel(width int) string {
	var b strings.Builder

	// Header
	inst := m.activeInstance()
	if inst != nil {
		b.WriteString(styles.Title.Render(fmt.Sprintf("Diff Preview: %s", inst.Branch)))
	} else {
		b.WriteString(styles.Title.Render("Diff Preview"))
	}
	b.WriteString("\n")

	if m.diffContent == "" {
		b.WriteString(styles.Muted.Render("No changes to display"))
		return styles.ContentBox.Width(width - 4).Render(b.String())
	}

	// Calculate available height for diff content
	maxLines := m.height - 14
	if maxLines < 5 {
		maxLines = 5
	}

	// Split diff into lines and apply scroll
	lines := strings.Split(m.diffContent, "\n")
	totalLines := len(lines)

	// Clamp scroll position
	maxScroll := totalLines - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.diffScroll > maxScroll {
		m.diffScroll = maxScroll
	}

	// Get visible lines
	startLine := m.diffScroll
	endLine := startLine + maxLines
	if endLine > totalLines {
		endLine = totalLines
	}

	visibleLines := lines[startLine:endLine]

	// Apply syntax highlighting to each visible line
	var highlighted []string
	for _, line := range visibleLines {
		highlighted = append(highlighted, m.highlightDiffLine(line))
	}

	// Show scroll indicator
	scrollInfo := fmt.Sprintf("Lines %d-%d of %d", startLine+1, endLine, totalLines)
	if totalLines > maxLines {
		scrollInfo += "  " + styles.Muted.Render("[j/k scroll, g/G top/bottom, d/Esc close]")
	} else {
		scrollInfo += "  " + styles.Muted.Render("[d/Esc close]")
	}
	b.WriteString(styles.Muted.Render(scrollInfo))
	b.WriteString("\n\n")

	// Add the diff content
	b.WriteString(strings.Join(highlighted, "\n"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// highlightDiffLine applies syntax highlighting to a single diff line
func (m Model) highlightDiffLine(line string) string {
	if len(line) == 0 {
		return line
	}

	// Check line prefix for diff syntax highlighting
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "@@"):
		return styles.DiffHunk.Render(line)
	case strings.HasPrefix(line, "+"):
		return styles.DiffAdd.Render(line)
	case strings.HasPrefix(line, "-"):
		return styles.DiffRemove.Render(line)
	case strings.HasPrefix(line, "diff "):
		return styles.DiffHeader.Render(line)
	case strings.HasPrefix(line, "index "):
		return styles.DiffHeader.Render(line)
	default:
		return styles.DiffContext.Render(line)
	}
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

// renderHelp renders the help bar
func (m Model) renderHelp() string {
	if m.inputMode {
		return styles.HelpBar.Render(
			styles.Warning.Bold(true).Render("INPUT MODE") + "  " +
				styles.HelpKey.Render("[Ctrl+]]") + " exit input mode  " +
				"All keystrokes forwarded to Claude",
		)
	}

	if m.commandMode {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render(":") + styles.Primary.Render(m.commandBuffer) +
				styles.Muted.Render("█") + "  " +
				styles.HelpKey.Render("[Enter]") + " execute  " +
				styles.HelpKey.Render("[Esc]") + " cancel  " +
				styles.Muted.Render("Type command (e.g., :s :a :d :h)"),
		)
	}

	if m.showDiff {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("DIFF VIEW") + "  " +
				styles.HelpKey.Render("[j/k]") + " scroll  " +
				styles.HelpKey.Render("[g/G]") + " top/bottom  " +
				styles.HelpKey.Render("[:d/Esc]") + " close",
		)
	}

	if m.filterMode {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("FILTER MODE") + "  " +
				styles.HelpKey.Render("[e/w/t/h/p]") + " toggle categories  " +
				styles.HelpKey.Render("[a]") + " all  " +
				styles.HelpKey.Render("[c]") + " clear  " +
				styles.HelpKey.Render("[Esc]") + " close",
		)
	}

	if m.searchMode {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("SEARCH") + "  " +
				"Type pattern  " +
				styles.HelpKey.Render("[Enter]") + " confirm  " +
				styles.HelpKey.Render("[Esc]") + " cancel  " +
				styles.Muted.Render("r:pattern for regex"),
		)
	}

	keys := []string{
		styles.HelpKey.Render("[:]") + " cmd",
		styles.HelpKey.Render("[j/k]") + " scroll",
		styles.HelpKey.Render("[Tab]") + " switch",
		styles.HelpKey.Render("[i]") + " input",
		styles.HelpKey.Render("[/]") + " search",
		styles.HelpKey.Render("[?]") + " help",
		styles.HelpKey.Render("[q]") + " quit",
	}

	// Add conflict indicator when conflicts exist
	if len(m.conflicts) > 0 {
		conflictKey := styles.Warning.Bold(true).Render("[:c]") + styles.Warning.Render(" conflicts")
		keys = append([]string{conflictKey}, keys...)
	}

	// Add search status indicator if search is active
	if m.searchPattern != "" && len(m.searchMatches) > 0 {
		searchStatus := styles.Secondary.Render(fmt.Sprintf("[%d/%d]", m.searchCurrent+1, len(m.searchMatches)))
		keys = append(keys, searchStatus+" "+styles.HelpKey.Render("[n/N]")+" match")
	}

	return styles.HelpBar.Render(strings.Join(keys, "  "))
}

// Helper functions

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// renderStatsPanel renders the session statistics/metrics panel
func (m Model) renderStatsPanel(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("📊 Session Statistics"))
	b.WriteString("\n\n")

	if m.session == nil {
		b.WriteString(styles.Muted.Render("No active session"))
		return styles.ContentBox.Width(width - 4).Render(b.String())
	}

	// Get aggregated session metrics
	sessionMetrics := m.orchestrator.GetSessionMetrics()

	// Session summary
	b.WriteString(styles.Subtitle.Render("Session Summary"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Total Instances: %d (%d active)\n",
		sessionMetrics.InstanceCount, sessionMetrics.ActiveCount))
	b.WriteString(fmt.Sprintf("  Session Started: %s\n",
		m.session.Created.Format("2006-01-02 15:04:05")))
	b.WriteString("\n")

	// Token usage
	b.WriteString(styles.Subtitle.Render("Token Usage"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Input:  %s\n", instmetrics.FormatTokens(sessionMetrics.TotalInputTokens)))
	b.WriteString(fmt.Sprintf("  Output: %s\n", instmetrics.FormatTokens(sessionMetrics.TotalOutputTokens)))
	totalTokens := sessionMetrics.TotalInputTokens + sessionMetrics.TotalOutputTokens
	b.WriteString(fmt.Sprintf("  Total:  %s\n", instmetrics.FormatTokens(totalTokens)))
	if sessionMetrics.TotalCacheRead > 0 || sessionMetrics.TotalCacheWrite > 0 {
		b.WriteString(fmt.Sprintf("  Cache:  %s read / %s write\n",
			instmetrics.FormatTokens(sessionMetrics.TotalCacheRead),
			instmetrics.FormatTokens(sessionMetrics.TotalCacheWrite)))
	}
	b.WriteString("\n")

	// Cost summary
	b.WriteString(styles.Subtitle.Render("Estimated Cost"))
	b.WriteString("\n")
	costStr := instmetrics.FormatCost(sessionMetrics.TotalCost)
	cfg := config.Get()
	if cfg.Resources.CostWarningThreshold > 0 && sessionMetrics.TotalCost >= cfg.Resources.CostWarningThreshold {
		b.WriteString(styles.Warning.Render(fmt.Sprintf("  Total: %s (⚠ exceeds warning threshold)", costStr)))
	} else {
		b.WriteString(fmt.Sprintf("  Total: %s\n", costStr))
	}
	if cfg.Resources.CostLimit > 0 {
		b.WriteString(fmt.Sprintf("  Limit: %s\n", instmetrics.FormatCost(cfg.Resources.CostLimit)))
	}
	b.WriteString("\n")

	// Per-instance breakdown
	b.WriteString(styles.Subtitle.Render("Top Instances by Cost"))
	b.WriteString("\n")

	// Sort instances by cost (simple bubble for small lists)
	type instCost struct {
		id   string
		num  int
		task string
		cost float64
	}
	var costList []instCost
	for i, inst := range m.session.Instances {
		cost := 0.0
		if inst.Metrics != nil {
			cost = inst.Metrics.Cost
		}
		costList = append(costList, instCost{
			id:   inst.ID,
			num:  i + 1,
			task: inst.Task,
			cost: cost,
		})
	}
	// Sort descending by cost
	for i := 0; i < len(costList)-1; i++ {
		for j := i + 1; j < len(costList); j++ {
			if costList[j].cost > costList[i].cost {
				costList[i], costList[j] = costList[j], costList[i]
			}
		}
	}

	// Show top 5
	shown := 0
	for _, ic := range costList {
		if shown >= 5 {
			break
		}
		if ic.cost > 0 {
			taskTrunc := truncate(ic.task, width-25)
			b.WriteString(fmt.Sprintf("  %d. [%d] %s: %s\n",
				shown+1, ic.num, taskTrunc, instmetrics.FormatCost(ic.cost)))
			shown++
		}
	}
	if shown == 0 {
		b.WriteString(styles.Muted.Render("  No cost data available yet"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("Press [m] to close this view"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

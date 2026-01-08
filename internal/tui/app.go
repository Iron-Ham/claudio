package tui

import (
	"fmt"
	"strings"
	"time"

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

// Run starts the TUI application
func (a *App) Run() error {
	a.program = tea.NewProgram(
		a.model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := a.program.Run()
	return err
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

// Commands

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tick(),
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeypress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tickMsg:
		// Update outputs from instances
		m.updateOutputs()
		// Clear info message after display (will show for ~100ms per tick, so a few ticks)
		// We'll let it persist for a bit by not clearing immediately
		return m, tick()

	case outputMsg:
		if m.outputs == nil {
			m.outputs = make(map[string]string)
		}
		m.outputs[msg.instanceID] += string(msg.data)
		return m, nil

	case errMsg:
		m.errorMessage = msg.err.Error()
		return m, nil
	}

	return m, nil
}

// handleKeypress processes keyboard input
func (m Model) handleKeypress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
				m.sendKeyToTmux(mgr, msg)
			}
		}
		return m, nil
	}

	// Handle task input mode
	if m.addingTask {
		// Check for Shift+Enter first (adds newline)
		if msg.Type == tea.KeyEnter && msg.Alt {
			// Alt+Enter as fallback for terminals that don't support Shift+Enter
			m.taskInput += "\n"
			return m, nil
		}
		if msg.String() == "shift+enter" {
			m.taskInput += "\n"
			return m, nil
		}

		switch msg.Type {
		case tea.KeyEsc:
			m.addingTask = false
			m.taskInput = ""
			return m, nil
		case tea.KeyEnter:
			if m.taskInput != "" {
				// Add new instance
				_, err := m.orchestrator.AddInstance(m.session, m.taskInput)
				if err != nil {
					m.errorMessage = err.Error()
				} else {
					// Switch to the newly added task
					m.activeTab = len(m.session.Instances) - 1
				}
			}
			m.addingTask = false
			m.taskInput = ""
			return m, nil
		case tea.KeyBackspace:
			if len(m.taskInput) > 0 {
				m.taskInput = m.taskInput[:len(m.taskInput)-1]
			}
			return m, nil
		case tea.KeySpace:
			m.taskInput += " "
			return m, nil
		case tea.KeyRunes:
			m.taskInput += string(msg.Runes)
			return m, nil
		}
		return m, nil
	}

	// Normal mode - clear info message on most actions
	m.infoMessage = ""

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "a":
		m.addingTask = true
		m.taskInput = ""
		return m, nil

	case "tab", "l":
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab + 1) % m.instanceCount()
		}
		return m, nil

	case "shift+tab", "h":
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab - 1 + m.instanceCount()) % m.instanceCount()
		}
		return m, nil

	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '1')
		if idx < m.instanceCount() {
			m.activeTab = idx
		}
		return m, nil

	case "s":
		// Start active instance
		if inst := m.activeInstance(); inst != nil {
			if err := m.orchestrator.StartInstance(inst); err != nil {
				m.errorMessage = err.Error()
			}
		}
		return m, nil

	case "p":
		// Pause/resume active instance
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil {
				if inst.Status == orchestrator.StatusPaused {
					mgr.Resume()
					inst.Status = orchestrator.StatusWorking
				} else if inst.Status == orchestrator.StatusWorking {
					mgr.Pause()
					inst.Status = orchestrator.StatusPaused
				}
			}
		}
		return m, nil

	case "x":
		// Stop active instance
		if inst := m.activeInstance(); inst != nil {
			if err := m.orchestrator.StopInstance(inst); err != nil {
				m.errorMessage = err.Error()
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

	case "t":
		// Show tmux attach command for the active instance
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil {
				m.infoMessage = "Attach with: " + mgr.AttachCommand()
				m.errorMessage = "" // Clear any error
			}
		}
		return m, nil
	}

	return m, nil
}

// sendKeyToTmux sends a key event to the tmux session
func (m Model) sendKeyToTmux(mgr *instance.Manager, msg tea.KeyMsg) {
	var key string
	literal := false

	switch msg.Type {
	case tea.KeyEnter:
		key = "Enter"
	case tea.KeyBackspace:
		key = "BSpace"
	case tea.KeyTab:
		key = "Tab"
	case tea.KeySpace:
		key = " " // Send literal space
		literal = true
	case tea.KeyEsc:
		key = "Escape"
	case tea.KeyUp:
		key = "Up"
	case tea.KeyDown:
		key = "Down"
	case tea.KeyRight:
		key = "Right"
	case tea.KeyLeft:
		key = "Left"
	case tea.KeyCtrlC:
		key = "C-c"
	case tea.KeyCtrlD:
		key = "C-d"
	case tea.KeyCtrlZ:
		key = "C-z"
	case tea.KeyRunes:
		// Send literal characters
		key = string(msg.Runes)
		literal = true
	default:
		// Try to handle other keys by their string representation
		key = msg.String()
		if len(key) == 1 {
			literal = true
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

// updateOutputs fetches latest output from all instances and checks for conflicts
func (m *Model) updateOutputs() {
	if m.session == nil {
		return
	}

	for _, inst := range m.session.Instances {
		mgr := m.orchestrator.GetInstanceManager(inst.ID)
		if mgr != nil {
			output := mgr.GetOutput()
			if len(output) > 0 {
				m.outputs[inst.ID] = string(output)
			}
		}
	}

	// Check for file conflicts
	detector := m.orchestrator.GetConflictDetector()
	if detector != nil {
		m.conflicts = detector.GetConflicts()
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

	// Header
	header := m.renderHeader()
	b.WriteString(header)
	b.WriteString("\n")

	// Tabs
	tabs := m.renderTabs()
	b.WriteString(tabs)
	b.WriteString("\n\n")

	// Content area
	content := m.renderContent()
	b.WriteString(content)

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

	// Help/status bar
	b.WriteString("\n")
	b.WriteString(m.renderHelp())

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

// renderTabs renders the instance tabs
func (m Model) renderTabs() string {
	if m.instanceCount() == 0 {
		return styles.Muted.Render("No instances. Press [a] to add one.")
	}

	// Build a set of instance IDs that have conflicts
	conflictingInstances := make(map[string]bool)
	for _, c := range m.conflicts {
		for _, instID := range c.Instances {
			conflictingInstances[instID] = true
		}
	}

	var tabs []string
	for i, inst := range m.session.Instances {
		// Add conflict indicator if instance has conflicts
		label := fmt.Sprintf("[%d] %s", i+1, truncate(inst.Task, 20))
		if conflictingInstances[inst.ID] {
			label = fmt.Sprintf("[%d] ⚠ %s", i+1, truncate(inst.Task, 18))
		}

		var style lipgloss.Style
		if i == m.activeTab {
			if conflictingInstances[inst.ID] {
				// Active tab with conflict - use warning background
				style = styles.TabInputNeeded
			} else if inst.Status == orchestrator.StatusWaitingInput {
				style = styles.TabInputNeeded
			} else {
				style = styles.TabActive
			}
		} else {
			if conflictingInstances[inst.ID] {
				// Inactive but has conflict - use warning color
				style = styles.TabInactive.Copy().Foreground(styles.WarningColor)
			} else if inst.Status == orchestrator.StatusWaitingInput {
				// Inactive but needs input - use warning color
				style = styles.TabInactive.Copy().Foreground(styles.WarningColor)
			} else {
				style = styles.TabInactive
			}
		}

		tabs = append(tabs, style.Render(label))
	}

	// Add "+" tab
	tabs = append(tabs, styles.TabInactive.Render("[+] Add"))

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderContent renders the main content area
func (m Model) renderContent() string {
	if m.addingTask {
		return m.renderAddTask()
	}

	if m.showHelp {
		return m.renderHelpPanel()
	}

	inst := m.activeInstance()
	if inst == nil {
		return styles.ContentBox.Width(m.width - 4).Render(
			"No instance selected.\n\nPress [a] to add a new Claude instance.",
		)
	}

	return m.renderInstance(inst)
}

// renderInstance renders the active instance view
func (m Model) renderInstance(inst *orchestrator.Instance) string {
	var b strings.Builder

	// Instance info
	statusColor := styles.StatusColor(string(inst.Status))
	statusBadge := styles.StatusBadge.Background(statusColor).Render(string(inst.Status))

	info := fmt.Sprintf("%s  Branch: %s", statusBadge, inst.Branch)
	b.WriteString(styles.InstanceInfo.Render(info))
	b.WriteString("\n")

	// Task
	b.WriteString(styles.Subtitle.Render("Task: " + inst.Task))
	b.WriteString("\n")

	// Show running/input mode status
	mgr := m.orchestrator.GetInstanceManager(inst.ID)
	if mgr != nil && mgr.Running() {
		if m.inputMode {
			// Show active input mode indicator
			inputBanner := lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.WarningColor).
				Padding(0, 1).
				Render("INPUT MODE")
			hint := inputBanner + "  " + styles.Muted.Render("Press ") +
				styles.HelpKey.Render("Ctrl+]") + styles.Muted.Render(" to exit")
			b.WriteString(hint)
		} else {
			// Show hint to enter input mode
			runningBanner := lipgloss.NewStyle().
				Bold(true).
				Foreground(styles.TextColor).
				Background(styles.SecondaryColor).
				Padding(0, 1).
				Render("RUNNING")
			hint := runningBanner + "  " + styles.Muted.Render("Press ") +
				styles.HelpKey.Render("[i]") + styles.Muted.Render(" to interact  ") +
				styles.HelpKey.Render("[t]") + styles.Muted.Render(" for tmux attach cmd")
			b.WriteString(hint)
		}
	}
	b.WriteString("\n")

	// Output
	output := m.outputs[inst.ID]
	if output == "" {
		output = "No output yet. Press [s] to start this instance."
	}

	// Limit output to visible area
	maxLines := m.height - 16
	if maxLines < 5 {
		maxLines = 5
	}
	output = lastNLines(output, maxLines)

	outputBox := styles.OutputArea.
		Width(m.width - 6).
		Height(maxLines).
		Render(output)

	b.WriteString(outputBox)

	return b.String()
}

// renderAddTask renders the add task input
func (m Model) renderAddTask() string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("Add New Instance"))
	b.WriteString("\n\n")
	b.WriteString("Enter task description:\n\n")

	// Handle multiline display
	lines := strings.Split(m.taskInput, "\n")
	for i, line := range lines {
		if i == len(lines)-1 {
			b.WriteString("> " + line + "█")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(styles.Muted.Render("Enter") + " submit  " +
		styles.Muted.Render("Shift+Enter") + " newline  " +
		styles.Muted.Render("Esc") + " cancel")

	return styles.ContentBox.Width(m.width - 4).Render(b.String())
}

// renderHelpPanel renders the help overlay
func (m Model) renderHelpPanel() string {
	help := `
Keyboard Shortcuts

Navigation:
  1-9        Select instance by number
  Tab / l    Next instance
  Shift+Tab  Previous instance

Instance Control:
  a          Add new instance
  s          Start selected instance
  p          Pause/resume instance
  x          Stop instance

Input Mode:
  i / Enter  Enter input mode (interact with Claude)
  Ctrl+]     Exit input mode
  t          Show tmux attach command

General:
  ?          Toggle help
  q          Quit

Each Claude instance runs in its own tmux session.
In input mode, ALL keystrokes are forwarded to Claude.
Press Ctrl+] to return to navigation mode.

You can also attach directly to a session with:
  tmux attach -t claudio-<instance-id>
`
	return styles.ContentBox.Width(m.width - 4).Render(help)
}

// renderConflictWarning renders the file conflict warning banner
func (m Model) renderConflictWarning() string {
	if len(m.conflicts) == 0 {
		return ""
	}

	var b strings.Builder

	// Banner header
	banner := styles.ConflictBanner.Render("⚠ FILE CONFLICT DETECTED")
	b.WriteString(banner)
	b.WriteString("  ")

	// Build conflict details
	var conflictDetails []string
	for _, c := range m.conflicts {
		// Find instance names/numbers for the conflicting instances
		var instanceLabels []string
		for _, instID := range c.Instances {
			// Find the instance index
			for i, inst := range m.session.Instances {
				if inst.ID == instID {
					instanceLabels = append(instanceLabels, fmt.Sprintf("[%d]", i+1))
					break
				}
			}
		}
		detail := fmt.Sprintf("%s (instances %s)", c.RelativePath, strings.Join(instanceLabels, ", "))
		conflictDetails = append(conflictDetails, detail)
	}

	// Show conflict files
	if len(conflictDetails) <= 2 {
		b.WriteString(styles.Warning.Render(strings.Join(conflictDetails, "; ")))
	} else {
		// Show count and first file
		b.WriteString(styles.Warning.Render(fmt.Sprintf("%d files: %s, ...", len(conflictDetails), conflictDetails[0])))
	}

	return b.String()
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

	keys := []string{
		styles.HelpKey.Render("[1-9]") + " select",
		styles.HelpKey.Render("[a]") + " add",
		styles.HelpKey.Render("[s]") + " start",
		styles.HelpKey.Render("[i]") + " input",
		styles.HelpKey.Render("[p]") + " pause",
		styles.HelpKey.Render("[x]") + " stop",
		styles.HelpKey.Render("[?]") + " help",
		styles.HelpKey.Render("[q]") + " quit",
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

func lastNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

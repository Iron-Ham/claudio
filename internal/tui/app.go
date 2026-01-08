package tui

import (
	"fmt"
	"strings"
	"time"

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
	// Handle task input mode
	if m.addingTask {
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
		default:
			if msg.Type == tea.KeyRunes {
				m.taskInput += string(msg.Runes)
			}
			return m, nil
		}
	}

	// Normal mode
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

	}

	return m, nil
}

// updateOutputs fetches latest output from all instances
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

	// Error message if any
	if m.errorMessage != "" {
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

	var tabs []string
	for i, inst := range m.session.Instances {
		label := fmt.Sprintf("[%d] %s", i+1, truncate(inst.Task, 20))

		var style lipgloss.Style
		if i == m.activeTab {
			if inst.Status == orchestrator.StatusWaitingInput {
				style = styles.TabInputNeeded
			} else {
				style = styles.TabActive
			}
		} else {
			if inst.Status == orchestrator.StatusWaitingInput {
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

	// Show running status
	mgr := m.orchestrator.GetInstanceManager(inst.ID)
	if mgr != nil && mgr.Running() {
		runningBanner := lipgloss.NewStyle().
			Bold(true).
			Foreground(styles.TextColor).
			Background(styles.SecondaryColor).
			Padding(0, 1).
			Render("RUNNING")
		b.WriteString(runningBanner + "  " + styles.Muted.Render("Claude is working autonomously"))
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
	b.WriteString("> " + m.taskInput + "█")
	b.WriteString("\n\n")
	b.WriteString(styles.Muted.Render("Press Enter to add, Esc to cancel"))

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

General:
  ?          Toggle help
  q          Quit

Note: Instances run autonomously in --print mode.
Each instance works independently on its assigned task.
`
	return styles.ContentBox.Width(m.width - 4).Render(help)
}

// renderHelp renders the help bar
func (m Model) renderHelp() string {
	keys := []string{
		styles.HelpKey.Render("[1-9]") + " select",
		styles.HelpKey.Render("[a]") + " add",
		styles.HelpKey.Render("[s]") + " start",
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

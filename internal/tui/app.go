package tui

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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

// Run starts the TUI application
func (a *App) Run() error {
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
	ContentWidthOffset  = 5 // sidebar gap (3) + border chars (2)
	ContentHeightOffset = 6 // header + help bar + margins
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

		// Calculate the content area dimensions and resize tmux sessions
		contentWidth, contentHeight := CalculateContentDimensions(m.width, m.height)
		if m.orchestrator != nil && contentWidth > 0 && contentHeight > 0 {
			m.orchestrator.ResizeAllInstances(contentWidth, contentHeight)
		}

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
		// Handle template dropdown if visible
		if m.showTemplates {
			return m.handleTemplateDropdown(msg)
		}

		// Check for newline shortcuts (shift+enter, alt+enter, or ctrl+j)
		// Note: shift+enter only works in terminals that support extended keyboard
		// protocols (Kitty, iTerm2, WezTerm, Ghostty). Alt+Enter and Ctrl+J work
		// universally as fallbacks.
		if msg.Type == tea.KeyEnter && msg.Alt {
			m.taskInput += "\n"
			return m, nil
		}
		if msg.String() == "shift+enter" {
			m.taskInput += "\n"
			return m, nil
		}
		if msg.Type == tea.KeyCtrlJ {
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
			char := string(msg.Runes)
			// Detect "/" at start of input or after newline to show templates
			if char == "/" && (m.taskInput == "" || strings.HasSuffix(m.taskInput, "\n")) {
				m.showTemplates = true
				m.templateFilter = ""
				m.templateSelected = 0
				m.taskInput += char
				return m, nil
			}
			m.taskInput += char
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

	case "c":
		// Toggle conflict detail view (only if conflicts exist)
		if len(m.conflicts) > 0 {
			m.showConflicts = !m.showConflicts
		}
		return m, nil

	case "a":
		m.addingTask = true
		m.taskInput = ""
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
			} else {
				// Suggest creating a PR after stopping
				m.infoMessage = fmt.Sprintf("Instance stopped. Create PR with: claudio pr %s", inst.ID)
			}
		}
		return m, nil

	case "r":
		// Show PR creation command for active instance
		if inst := m.activeInstance(); inst != nil {
			m.infoMessage = fmt.Sprintf("Create PR: claudio pr %s  (add --draft for draft PR)", inst.ID)
			m.errorMessage = "" // Clear any error
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

	case "d":
		// Toggle diff preview for the active instance
		if m.showDiff {
			m.showDiff = false
			m.diffContent = ""
			m.diffScroll = 0
			return m, nil
		}
		if inst := m.activeInstance(); inst != nil {
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
		// Scroll down in diff view, or navigate to next instance
		if m.showDiff {
			m.diffScroll++
			return m, nil
		}
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab + 1) % m.instanceCount()
			m.ensureActiveVisible()
		}
		return m, nil

	case "k", "up":
		// Scroll up in diff view, or navigate to previous instance
		if m.showDiff {
			if m.diffScroll > 0 {
				m.diffScroll--
			}
			return m, nil
		}
		if m.instanceCount() > 0 {
			m.activeTab = (m.activeTab - 1 + m.instanceCount()) % m.instanceCount()
			m.ensureActiveVisible()
		}
		return m, nil

	case "g":
		// Go to top of diff
		if m.showDiff {
			m.diffScroll = 0
			return m, nil
		}
		return m, nil

	case "G":
		// Go to bottom of diff (handled in render by maxScroll)
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
		return m, nil

	case "C":
		// Clear all completed instances
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
		}
		m.errorMessage = "" // Clear any error
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
			}
			m.templateSelected = 0 // Reset selection on filter change
		} else {
			// Remove the "/" and close dropdown
			if len(m.taskInput) > 0 {
				m.taskInput = m.taskInput[:len(m.taskInput)-1]
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
			m.templateFilter = ""
			m.templateSelected = 0
			return m, nil
		}
		// Add to both filter and taskInput
		m.templateFilter += char
		m.taskInput += char
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

// updateOutputs fetches latest output from all instances, updates their status, and checks for conflicts
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

			// Update instance status based on detected waiting state
			// Only update if the instance is currently working (not paused, completed, etc.)
			if inst.Status == orchestrator.StatusWorking {
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

	// Header
	header := m.renderHeader()
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
	sidebar := m.renderSidebar(effectiveSidebarWidth, mainAreaHeight)
	content := m.renderContent(mainContentWidth)

	// Apply height to both panels and join horizontally
	sidebarStyled := lipgloss.NewStyle().
		Width(effectiveSidebarWidth).
		Height(mainAreaHeight).
		Render(sidebar)

	contentStyled := lipgloss.NewStyle().
		Width(mainContentWidth).
		Height(mainAreaHeight).
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

// renderSidebar renders the instance sidebar with pagination support
func (m Model) renderSidebar(width int, height int) string {
	var b strings.Builder

	// Sidebar title
	b.WriteString(styles.SidebarTitle.Render("Instances"))
	b.WriteString("\n")

	if m.instanceCount() == 0 {
		b.WriteString(styles.Muted.Render("No instances"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [a] to add"))
	} else {
		// Calculate available slots for instances
		// Reserve: 1 for title, 1 for blank line, 1 for add hint, 2 for scroll indicators, plus border padding
		reservedLines := 6
		availableSlots := height - reservedLines
		if availableSlots < 3 {
			availableSlots = 3 // Minimum to show at least a few instances
		}

		totalInstances := m.instanceCount()
		hasMoreAbove := m.sidebarScrollOffset > 0
		hasMoreBelow := m.sidebarScrollOffset+availableSlots < totalInstances

		// Show scroll up indicator if there are instances above
		if hasMoreAbove {
			scrollUp := styles.Muted.Render(fmt.Sprintf("▲ %d more above", m.sidebarScrollOffset))
			b.WriteString(scrollUp)
			b.WriteString("\n")
		}

		// Build a set of instance IDs that have conflicts
		conflictingInstances := make(map[string]bool)
		for _, c := range m.conflicts {
			for _, instID := range c.Instances {
				conflictingInstances[instID] = true
			}
		}

		// Calculate the visible range
		startIdx := m.sidebarScrollOffset
		endIdx := m.sidebarScrollOffset + availableSlots
		if endIdx > totalInstances {
			endIdx = totalInstances
		}

		// Render visible instances using helper
		for i := startIdx; i < endIdx; i++ {
			inst := m.session.Instances[i]
			b.WriteString(m.renderSidebarInstance(i, inst, conflictingInstances, width))
			b.WriteString("\n")
		}

		// Show scroll down indicator if there are instances below
		if hasMoreBelow {
			remaining := totalInstances - endIdx
			scrollDown := styles.Muted.Render(fmt.Sprintf("▼ %d more below", remaining))
			b.WriteString(scrollDown)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	// Add instance hint with navigation help when paginated
	if m.instanceCount() > 0 {
		addHint := styles.Muted.Render("[a]") + " " + styles.Muted.Render("add") + "  " +
			styles.Muted.Render("[↑↓]") + " " + styles.Muted.Render("nav")
		b.WriteString(addHint)
	} else {
		addHint := styles.Muted.Render("[a]") + " " + styles.Muted.Render("Add new")
		b.WriteString(addHint)
	}

	// Wrap in sidebar box
	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderSidebarInstance renders a single instance item in the sidebar
func (m Model) renderSidebarInstance(i int, inst *orchestrator.Instance, conflictingInstances map[string]bool, width int) string {
	// Status indicator (colored dot)
	statusColor := styles.StatusColor(string(inst.Status))
	dot := lipgloss.NewStyle().Foreground(statusColor).Render("●")

	// Instance number and truncated task
	maxTaskLen := width - 8 // Account for number, dot, padding
	if maxTaskLen < 10 {
		maxTaskLen = 10
	}
	label := fmt.Sprintf("%d %s", i+1, truncate(inst.Task, maxTaskLen))
	// Add conflict indicator if instance has conflicts
	if conflictingInstances[inst.ID] {
		label = fmt.Sprintf("%d ⚠ %s", i+1, truncate(inst.Task, maxTaskLen-2))
	}

	// Choose style based on active state and status
	var itemStyle lipgloss.Style
	if i == m.activeTab {
		if conflictingInstances[inst.ID] {
			// Active item with conflict - use warning background
			itemStyle = styles.SidebarItemInputNeeded
		} else if inst.Status == orchestrator.StatusWaitingInput {
			itemStyle = styles.SidebarItemInputNeeded
		} else {
			itemStyle = styles.SidebarItemActive
		}
	} else {
		itemStyle = styles.SidebarItem
		if conflictingInstances[inst.ID] {
			// Inactive but has conflict - use warning color
			itemStyle = itemStyle.Copy().Foreground(styles.WarningColor)
		} else if inst.Status == orchestrator.StatusWaitingInput {
			itemStyle = itemStyle.Copy().Foreground(styles.WarningColor)
		} else {
			itemStyle = itemStyle.Copy().Foreground(styles.MutedColor)
		}
	}

	// Combine dot and label
	return dot + " " + itemStyle.Render(label)
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
	maxLines := m.height - 12 // Adjusted for sidebar layout
	if maxLines < 5 {
		maxLines = 5
	}
	output = lastNLines(output, maxLines)

	outputBox := styles.OutputArea.
		Width(width - 4).
		Height(maxLines).
		Render(output)

	b.WriteString(outputBox)

	return b.String()
}

// renderAddTask renders the add task input
func (m Model) renderAddTask(width int) string {
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

	// Show template dropdown if active
	if m.showTemplates {
		b.WriteString("\n")
		b.WriteString(m.renderTemplateDropdown())
	}

	b.WriteString("\n\n")
	if m.showTemplates {
		b.WriteString(styles.Muted.Render("↑/↓") + " navigate  " +
			styles.Muted.Render("Enter/Tab") + " select  " +
			styles.Muted.Render("Esc") + " close  " +
			styles.Muted.Render("Type") + " filter")
	} else {
		b.WriteString(styles.Muted.Render("Enter") + " submit  " +
			styles.Muted.Render("Shift+Enter") + " newline  " +
			styles.Muted.Render("/") + " templates  " +
			styles.Muted.Render("Esc") + " cancel")
	}

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// renderTemplateDropdown renders the template selection dropdown
func (m Model) renderTemplateDropdown() string {
	templates := FilterTemplates(m.templateFilter)
	if len(templates) == 0 {
		return styles.Muted.Render("  No matching templates")
	}

	var items []string
	for i, t := range templates {
		cmd := "/" + t.Command
		name := " - " + t.Name

		var item string
		if i == m.templateSelected {
			// Selected item - highlight the whole row
			item = styles.DropdownItemSelected.Render(cmd + name)
		} else {
			// Normal item - color command and name differently
			item = styles.DropdownItem.Render(
				styles.DropdownCommand.Render(cmd) +
					styles.DropdownName.Render(name),
			)
		}
		items = append(items, item)
	}

	content := strings.Join(items, "\n")
	return styles.DropdownContainer.Render(content)
}

// renderHelpPanel renders the help overlay
func (m Model) renderHelpPanel(width int) string {
	help := `
Keyboard Shortcuts

Navigation:
  ↑ / k      Previous instance
  ↓ / j      Next instance
  Tab / l    Next instance
  Shift+Tab  Previous instance
  1-9        Select instance by number

Instance Control:
  a          Add new instance
  s          Start selected instance
  p          Pause/resume instance
  x          Stop instance
  C          Clear completed instances
  r          Show PR creation command
  d          Show diff preview

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

Creating Pull Requests:
  After completing work, use: claudio pr <instance-id>
  This will use Claude to generate a meaningful PR title and description,
  rebase on main, and create the PR on GitHub.
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
	if len(m.conflicts) == 0 {
		return ""
	}

	var b strings.Builder

	// Banner header with hint that it's interactive
	banner := styles.ConflictBanner.Render("⚠ FILE CONFLICT DETECTED")
	b.WriteString(banner)
	b.WriteString("  ")
	b.WriteString(styles.Muted.Render("(press [c] for details)"))
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

// renderConflictPanel renders a detailed conflict view showing all files and instances
func (m Model) renderConflictPanel(width int) string {
	var b strings.Builder

	b.WriteString(styles.Title.Render("⚠ File Conflicts"))
	b.WriteString("\n\n")

	b.WriteString(styles.Muted.Render("The following files have been modified by multiple instances:"))
	b.WriteString("\n\n")

	// Build instance ID to number mapping
	instanceNum := make(map[string]int)
	instanceTask := make(map[string]string)
	for i, inst := range m.session.Instances {
		instanceNum[inst.ID] = i + 1
		instanceTask[inst.ID] = inst.Task
	}

	// Render each conflict
	for i, c := range m.conflicts {
		// File path in warning color
		fileLine := styles.Warning.Bold(true).Render(c.RelativePath)
		b.WriteString(fileLine)
		b.WriteString("\n")

		// List the instances that modified this file
		b.WriteString(styles.Muted.Render("  Modified by:"))
		b.WriteString("\n")
		for _, instID := range c.Instances {
			num := instanceNum[instID]
			task := instanceTask[instID]
			// Truncate task if too long
			maxTaskLen := width - 15
			if maxTaskLen < 20 {
				maxTaskLen = 20
			}
			if len(task) > maxTaskLen {
				task = task[:maxTaskLen-3] + "..."
			}
			instanceLine := fmt.Sprintf("    [%d] %s", num, task)
			b.WriteString(styles.Text.Render(instanceLine))
			b.WriteString("\n")
		}

		// Add spacing between conflicts except for the last one
		if i < len(m.conflicts)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("Press [c] to close this view"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
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

	if m.showDiff {
		return styles.HelpBar.Render(
			styles.Primary.Bold(true).Render("DIFF VIEW") + "  " +
				styles.HelpKey.Render("[j/k]") + " scroll  " +
				styles.HelpKey.Render("[g/G]") + " top/bottom  " +
				styles.HelpKey.Render("[d/Esc]") + " close",
		)
	}

	keys := []string{
		styles.HelpKey.Render("[↑↓/jk]") + " nav",
		styles.HelpKey.Render("[a]") + " add",
		styles.HelpKey.Render("[s]") + " start",
		styles.HelpKey.Render("[i]") + " input",
		styles.HelpKey.Render("[p]") + " pause",
		styles.HelpKey.Render("[x]") + " stop",
		styles.HelpKey.Render("[C]") + " clear",
		styles.HelpKey.Render("[d]") + " diff",
		styles.HelpKey.Render("[r]") + " pr",
		styles.HelpKey.Render("[?]") + " help",
		styles.HelpKey.Render("[q]") + " quit",
	}

	// Add conflict shortcut when conflicts exist
	if len(m.conflicts) > 0 {
		conflictKey := styles.Warning.Bold(true).Render("[c]") + styles.Warning.Render(" conflicts")
		keys = append([]string{conflictKey}, keys...)
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

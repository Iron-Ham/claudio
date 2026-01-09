package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
)

// EventRouter handles keyboard and mouse event dispatching for the TUI.
// It routes events to appropriate handlers based on the current application mode.
type EventRouter struct{}

// NewEventRouter creates a new EventRouter instance.
func NewEventRouter() *EventRouter {
	return &EventRouter{}
}

// RouteKeyEvent dispatches a key event to the appropriate handler based on current mode.
// This is the main entry point for all keyboard events in the application.
func (r *EventRouter) RouteKeyEvent(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode - typing search pattern
	if m.searchMode {
		return r.handleSearchInput(m, msg)
	}

	// Handle filter mode - selecting categories
	if m.filterMode {
		return r.handleFilterInput(m, msg)
	}

	// Handle input mode - forward keys to the active instance's tmux session
	if m.inputMode {
		return r.handleInputMode(m, msg)
	}

	// Handle task input mode
	if m.addingTask {
		return r.handleTaskInput(m, msg)
	}

	// Handle command mode (vim-style ex commands with ':' prefix)
	if m.commandMode {
		return r.handleCommandInput(m, msg)
	}

	// Normal mode - handle regular navigation and commands
	return r.handleNormalMode(m, msg)
}

// handleInputMode processes keys when in input mode (forwarding to tmux).
func (r *EventRouter) handleInputMode(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
			if msg.Paste && msg.Type == tea.KeyRunes && len(msg.Runes) > 0 {
				mgr.SendPaste(string(msg.Runes))
			} else {
				// Use the Model's sendKeyToTmux method for key mapping
				m.sendKeyToTmux(mgr, msg)
			}
		}
	}
	return m, nil
}

// handleTaskInput processes keys when adding a new task.
func (r *EventRouter) handleTaskInput(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle template dropdown if visible
	if m.showTemplates {
		return r.handleTemplateDropdown(m, msg)
	}

	// Check for newline shortcuts (shift+enter, alt+enter, or ctrl+j)
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
	keyStr := msg.String()
	switch keyStr {
	case "alt+left":
		m.taskInputCursor = m.taskInputFindPrevWordBoundary()
		return m, nil
	case "alt+right":
		m.taskInputCursor = m.taskInputFindNextWordBoundary()
		return m, nil
	case "alt+up", "ctrl+a":
		m.taskInputCursor = 0
		return m, nil
	case "alt+down", "ctrl+e":
		m.taskInputCursor = len([]rune(m.taskInput))
		return m, nil
	case "alt+backspace", "ctrl+w":
		prevWord := m.taskInputFindPrevWordBoundary()
		m.taskInputDeleteBack(m.taskInputCursor - prevWord)
		return m, nil
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.addingTask = false
		m.taskInput = ""
		m.taskInputCursor = 0
		return m, nil
	case tea.KeyEnter:
		if m.taskInput != "" {
			_, err := m.orchestrator.AddInstance(m.session, m.taskInput)
			if err != nil {
				m.errorMessage = err.Error()
			} else {
				m.activeTab = len(m.session.Instances) - 1
				m.ensureActiveVisible()
			}
		}
		m.addingTask = false
		m.taskInput = ""
		m.taskInputCursor = 0
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
		m.taskInputCursor = m.taskInputFindLineStart()
		return m, nil
	case tea.KeyEnd:
		m.taskInputCursor = m.taskInputFindLineEnd()
		return m, nil
	case tea.KeyCtrlU:
		lineStart := m.taskInputFindLineStart()
		m.taskInputDeleteBack(m.taskInputCursor - lineStart)
		return m, nil
	case tea.KeyCtrlK:
		lineEnd := m.taskInputFindLineEnd()
		m.taskInputDeleteForward(lineEnd - m.taskInputCursor)
		return m, nil
	case tea.KeySpace:
		m.taskInputInsert(" ")
		return m, nil
	case tea.KeyRunes:
		char := string(msg.Runes)
		if char == "\n" || char == "\r" {
			if m.taskInput != "" {
				_, err := m.orchestrator.AddInstance(m.session, m.taskInput)
				if err != nil {
					m.errorMessage = err.Error()
				} else {
					m.activeTab = len(m.session.Instances) - 1
					m.ensureActiveVisible()
				}
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

// handleTemplateDropdown handles keyboard input when the template dropdown is visible.
func (r *EventRouter) handleTemplateDropdown(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	templates := FilterTemplates(m.templateFilter)

	switch msg.Type {
	case tea.KeyEsc:
		m.showTemplates = false
		m.templateFilter = ""
		m.templateSelected = 0
		return m, nil

	case tea.KeyEnter, tea.KeyTab:
		if len(templates) > 0 && m.templateSelected < len(templates) {
			selected := templates[m.templateSelected]
			lastNewline := strings.LastIndex(m.taskInput, "\n")
			if lastNewline == -1 {
				m.taskInput = selected.Description
			} else {
				m.taskInput = m.taskInput[:lastNewline+1] + selected.Description
			}
			m.taskInputCursor = len([]rune(m.taskInput))
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
			m.templateFilter = m.templateFilter[:len(m.templateFilter)-1]
			if len(m.taskInput) > 0 {
				m.taskInput = m.taskInput[:len(m.taskInput)-1]
				m.taskInputCursor = len([]rune(m.taskInput))
			}
			m.templateSelected = 0
		} else {
			if len(m.taskInput) > 0 {
				m.taskInput = m.taskInput[:len(m.taskInput)-1]
				m.taskInputCursor = len([]rune(m.taskInput))
			}
			m.showTemplates = false
		}
		return m, nil

	case tea.KeyRunes:
		char := string(msg.Runes)
		if char == " " {
			m.showTemplates = false
			m.taskInput += " "
			m.taskInputCursor = len([]rune(m.taskInput))
			m.templateFilter = ""
			m.templateSelected = 0
			return m, nil
		}
		m.templateFilter += char
		m.taskInput += char
		m.taskInputCursor = len([]rune(m.taskInput))
		m.templateSelected = 0
		if len(FilterTemplates(m.templateFilter)) == 0 {
			m.showTemplates = false
			m.templateFilter = ""
		}
		return m, nil
	}

	return m, nil
}

// handleSearchInput handles keyboard input when in search mode.
func (r *EventRouter) handleSearchInput(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.searchMode = false
		return m, nil

	case tea.KeyEnter:
		m.executeSearch()
		m.searchMode = false
		return m, nil

	case tea.KeyBackspace:
		if len(m.searchPattern) > 0 {
			m.searchPattern = m.searchPattern[:len(m.searchPattern)-1]
			m.executeSearch()
		}
		return m, nil

	case tea.KeyRunes:
		m.searchPattern += string(msg.Runes)
		m.executeSearch()
		return m, nil

	case tea.KeySpace:
		m.searchPattern += " "
		m.executeSearch()
		return m, nil
	}

	return m, nil
}

// handleFilterInput handles keyboard input when in filter mode.
func (r *EventRouter) handleFilterInput(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// handleCommandInput handles keystrokes when in command mode (after pressing ':').
func (r *EventRouter) handleCommandInput(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.commandMode = false
		m.commandBuffer = ""
		return m, nil

	case tea.KeyEnter:
		m.commandMode = false
		cmd := m.commandBuffer
		m.commandBuffer = ""
		return m.executeCommand(cmd)

	case tea.KeyBackspace, tea.KeyDelete:
		if len(m.commandBuffer) > 0 {
			m.commandBuffer = m.commandBuffer[:len(m.commandBuffer)-1]
		}
		if len(m.commandBuffer) == 0 {
			m.commandMode = false
		}
		return m, nil

	case tea.KeySpace:
		m.commandBuffer += " "
		return m, nil

	case tea.KeyRunes:
		m.commandBuffer += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

// handleNormalMode handles keys when in normal navigation mode.
func (r *EventRouter) handleNormalMode(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Clear info message on most actions
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

	return r.handleNormalModeKeys(m, msg)
}

// handleNormalModeKeys handles the standard navigation and action keys.
func (r *EventRouter) handleNormalModeKeys(m Model, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case ":":
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
		if inst := m.activeInstance(); inst != nil {
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil && mgr.Running() {
				m.inputMode = true
			}
		}
		return m, nil

	case "esc":
		if m.showDiff {
			m.showDiff = false
			m.diffContent = ""
			m.diffScroll = 0
			return m, nil
		}
		return m, nil

	case "j", "down":
		if m.showDiff {
			m.diffScroll++
		} else if !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputDown(inst.ID, 1)
			}
		}
		return m, nil

	case "k", "up":
		if m.showDiff {
			if m.diffScroll > 0 {
				m.diffScroll--
			}
		} else if !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputUp(inst.ID, 1)
			}
		}
		return m, nil

	case "ctrl+u":
		if !m.showDiff && !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputUp(inst.ID, m.getOutputMaxLines()/2)
			}
		}
		return m, nil

	case "ctrl+d":
		if !m.showDiff && !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputDown(inst.ID, m.getOutputMaxLines()/2)
			}
		}
		return m, nil

	case "ctrl+b":
		if !m.showDiff && !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputUp(inst.ID, m.getOutputMaxLines())
			}
		}
		return m, nil

	case "ctrl+f":
		if !m.showDiff && !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputDown(inst.ID, m.getOutputMaxLines())
			}
		}
		return m, nil

	case "ctrl+r":
		// Restart instance with same task (useful for stuck/timeout instances)
		if inst := m.activeInstance(); inst != nil {
			switch inst.Status {
			case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
				m.infoMessage = "Instance is running. Use [:x] to stop it first, or [:p] to pause."
				return m, nil
			case orchestrator.StatusCreatingPR:
				m.infoMessage = "Instance is creating PR. Wait for it to complete."
				return m, nil
			}
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil {
				_ = mgr.Stop()
				mgr.ClearTimeout()
			}
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
			mgr := m.orchestrator.GetInstanceManager(inst.ID)
			if mgr != nil {
				_ = mgr.Stop()
			}
			if err := m.orchestrator.RemoveInstance(m.session, inst.ID, true); err != nil {
				m.errorMessage = fmt.Sprintf("Failed to remove instance: %v", err)
			} else {
				m.infoMessage = fmt.Sprintf("Instance %s killed and removed", inst.ID)
				if m.activeTab >= len(m.session.Instances) && m.activeTab > 0 {
					m.activeTab--
				}
			}
		}
		return m, nil

	case "g":
		if m.showDiff {
			m.diffScroll = 0
		} else if !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputToTop(inst.ID)
			}
		}
		return m, nil

	case "G":
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
		} else if !m.showHelp && !m.showConflicts {
			if inst := m.activeInstance(); inst != nil {
				m.scrollOutputToBottom(inst.ID)
			}
		}
		return m, nil

	case "/":
		m.searchMode = true
		m.searchPattern = ""
		m.searchMatches = nil
		m.searchCurrent = 0
		return m, nil

	case "n":
		if m.searchPattern != "" && len(m.searchMatches) > 0 {
			m.searchCurrent = (m.searchCurrent + 1) % len(m.searchMatches)
			m.scrollToMatch()
		}
		return m, nil

	case "N":
		if m.searchPattern != "" && len(m.searchMatches) > 0 {
			m.searchCurrent = (m.searchCurrent - 1 + len(m.searchMatches)) % len(m.searchMatches)
			m.scrollToMatch()
		}
		return m, nil

	case "ctrl+/":
		m.clearSearch()
		return m, nil
	}

	return m, nil
}

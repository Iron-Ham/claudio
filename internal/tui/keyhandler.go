package tui

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/input"
	tuimsg "github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

// -----------------------------------------------------------------------------
// Main Keypress Handler
// -----------------------------------------------------------------------------

// handleKeypress processes keyboard input and routes to appropriate mode handlers.
// This is the main entry point for all keyboard input in the TUI.
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
		return m.handleInputMode(msg)
	}

	// Handle terminal mode - forward keys to the terminal pane's tmux session
	if m.terminalManager.IsFocused() {
		return m.handleTerminalMode(msg)
	}

	// Handle task input mode
	if m.addingTask {
		return m.handleTaskInput(msg)
	}

	// Handle command mode (vim-style ex commands with ':' prefix)
	if m.commandMode {
		return m.handleCommandInput(msg)
	}

	// Normal mode
	return m.handleNormalMode(msg)
}

// -----------------------------------------------------------------------------
// Input Mode Handler (tmux passthrough)
// -----------------------------------------------------------------------------

// handleInputMode forwards keys to the active instance's tmux session.
func (m Model) handleInputMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// -----------------------------------------------------------------------------
// Terminal Mode Handler (terminal pane passthrough)
// -----------------------------------------------------------------------------

// handleTerminalMode forwards keys to the terminal pane's tmux session.
func (m Model) handleTerminalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// -----------------------------------------------------------------------------
// Task Input Mode Handler
// -----------------------------------------------------------------------------

// handleTaskInput handles keyboard input when in task input mode.
func (m Model) handleTaskInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		return m.cancelTaskInput(), nil
	case tea.KeyEnter:
		return m.submitTaskInput()
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
		return m.handleTaskInputRunes(msg)
	}
	return m, nil
}

// cancelTaskInput resets all task input state.
func (m Model) cancelTaskInput() Model {
	m.addingTask = false
	m.addingDependentTask = false
	m.dependentOnInstanceID = ""
	m.startingTripleShot = false
	m.startingAdversarial = false
	m.taskInput = ""
	m.taskInputCursor = 0
	m.templateSuffix = ""        // Clear suffix on cancel
	m.selectedBaseBranch = ""    // Clear base branch selection
	m.branchList = nil           // Clear cached branches
	m.showBranchSelector = false // Ensure dropdown is closed
	return m
}

// submitTaskInput processes the task input and creates a new task.
func (m Model) submitTaskInput() (tea.Model, tea.Cmd) {
	if m.taskInput != "" {
		// Capture task and clear input state first
		// Append template suffix if one was set (e.g., /plan instructions)
		task := m.taskInput + m.templateSuffix
		isDependent := m.addingDependentTask
		dependsOn := m.dependentOnInstanceID
		baseBranch := m.selectedBaseBranch
		isTripleShot := m.startingTripleShot
		isAdversarial := m.startingAdversarial
		m = m.cancelTaskInput()

		// Handle inline plan/ultraplan/multiplan objective submission
		if m.inlinePlan != nil {
			session := m.inlinePlan.GetAwaitingObjectiveSession()
			if session != nil {
				if session.IsUltraPlan {
					// This is an ultraplan objective - create the full ultraplan coordinator
					m.handleUltraPlanObjectiveSubmit(task)
				} else if session.MultiPass {
					// This is a multiplan objective - create parallel planning instances
					m.handleMultiPlanObjectiveSubmit(task)
				} else {
					// This is a regular plan objective
					m.handleInlinePlanObjectiveSubmit(task)
				}
				return m, nil
			}
		}

		// Handle triple-shot mode initiation
		if isTripleShot {
			return m.initiateTripleShotMode(task)
		}

		// Handle adversarial mode initiation
		if isAdversarial {
			return m.initiateAdversarialMode(task)
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

		// Use two-phase async task addition for faster UI feedback:
		// Phase 1: Create stub immediately (fast) - instance appears with "preparing" status
		// Phase 2: Create worktree in background (slow) - then auto-start if configured
		m.infoMessage = "Adding task..."
		return m, tuimsg.AddTaskStubAsync(m.orchestrator, m.session, task)
	}
	return m.cancelTaskInput(), nil
}

// handleTaskInputRunes handles rune input in task input mode.
func (m Model) handleTaskInputRunes(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	char := string(msg.Runes)
	// Handle Enter sent as rune (some terminals/input methods send \n or \r as runes)
	if char == "\n" || char == "\r" {
		return m.submitTaskInput()
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

// -----------------------------------------------------------------------------
// Command Mode Handler
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Normal Mode Handler
// -----------------------------------------------------------------------------

// handleNormalMode handles keyboard input in normal navigation mode.
func (m Model) handleNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

	// Handle group command mode (vim-style 'g' prefix)
	// When groupCommandPending is true, the next key is interpreted as a group command
	if m.inputRouter != nil && m.inputRouter.IsGroupCommandPending() {
		return m.handleGroupCommand(msg)
	}

	return m.handleNormalModeKey(msg)
}

// handleGroupCommand processes a key after the 'g' prefix in group command mode.
func (m Model) handleGroupCommand(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.inputRouter.SetGroupCommandPending(false) // Clear pending state
	groupHandler := NewGroupKeyHandler(m.session, m.groupViewState)
	result := groupHandler.HandleGroupKey(msg)
	if result.Handled {
		// Apply result actions based on the Action type
		switch result.Action {
		case GroupActionToggleCollapse:
			// Toggle already performed by GroupKeyHandler.handleToggleCollapse()
			// No additional action needed here
		case GroupActionCollapseAll:
			// Get a thread-safe snapshot of groups
			groups := m.session.GetGroups()
			if result.AllCollapsed {
				// Collapse all groups
				for _, g := range groups {
					if !m.groupViewState.IsCollapsed(g.ID) {
						m.groupViewState.ToggleCollapse(g.ID)
					}
				}
			} else {
				// Expand all groups
				for _, g := range groups {
					if m.groupViewState.IsCollapsed(g.ID) {
						m.groupViewState.ToggleCollapse(g.ID)
					}
				}
			}
		case GroupActionNextGroup, GroupActionPrevGroup:
			if result.GroupID != "" {
				m.groupViewState.SelectedGroupID = result.GroupID
				// Ensure the selected group is visible in the sidebar
				m.ensureSelectedGroupVisible()
			}
		case GroupActionSkipGroup:
			m.infoMessage = "Group skipped"
		case GroupActionRetryGroup:
			m.infoMessage = "Retrying failed tasks in group"
		case GroupActionForceStart:
			m.infoMessage = "Force-starting next group"
		case GroupActionDismissGroup:
			// Remove all instances in the group
			removed := 0
			for _, instID := range result.InstanceIDs {
				if err := m.orchestrator.RemoveInstance(m.session, instID, true); err != nil {
					if m.logger != nil {
						m.logger.Warn("failed to remove instance during group dismiss", "instance_id", instID, "error", err)
					}
				} else {
					removed++
				}
			}
			if removed > 0 {
				m.infoMessage = fmt.Sprintf("Dismissed %d instance(s) from group", removed)
				// Adjust active tab if needed
				if m.activeTab >= len(m.session.Instances) && m.activeTab > 0 {
					m.activeTab = len(m.session.Instances) - 1
				}
				if m.activeTab < 0 {
					m.activeTab = 0
				}
				m.ensureActiveVisible()
			} else {
				m.infoMessage = "No instances to dismiss"
			}
		}
		return m, nil
	}
	// If not handled, fall through to normal key handling
	return m.handleNormalModeKey(msg)
}

// handleNormalModeKey handles individual key presses in normal mode.
func (m Model) handleNormalModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		return m.handleNextInstance()

	case "shift+tab", "h":
		return m.handlePrevInstance()

	case "enter", "i":
		return m.handleEnterInputMode()

	case "`", "T":
		return m.handleToggleTerminal()

	case "ctrl+shift+t":
		return m.handleSwitchTerminalDir()

	case "esc":
		return m.handleEscape()

	case "j", "down":
		return m.handleScrollDown()

	case "k", "up":
		return m.handleScrollUp()

	case "J":
		return m.handleSidebarScrollDown()

	case "K":
		return m.handleSidebarScrollUp()

	case "ctrl+u":
		return m.handleHalfPageUp()

	case "ctrl+d":
		return m.handleHalfPageDown()

	case "ctrl+b":
		return m.handleFullPageUp()

	case "ctrl+f":
		return m.handleFullPageDown()

	case "ctrl+r":
		return m.handleRestartInstance()

	case "ctrl+k":
		return m.handleKillInstance()

	case "0":
		return m.handleGoToTop()

	case "g":
		// Enter group command mode (for gc, gn, gp, etc.)
		if m.sidebarMode == view.SidebarModeGrouped && m.session != nil && m.session.HasGroups() {
			m.inputRouter.SetGroupCommandPending(true)
		}
		return m, nil

	case "G":
		return m.handleGoToBottom()

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

	case "d":
		// Toggle dependency graph view
		m.toggleGraphView()
		return m, nil
	}

	return m, nil
}

// -----------------------------------------------------------------------------
// Normal Mode Key Handlers
// -----------------------------------------------------------------------------

// handleNextInstance switches to the next instance in display order.
// This navigates based on how instances appear in the sidebar, not by creation order.
func (m Model) handleNextInstance() (tea.Model, tea.Cmd) {
	displayOrder := m.getInstanceDisplayOrder()
	if len(displayOrder) == 0 {
		return m, nil
	}

	// Find current instance's position in display order
	currentInst := m.activeInstance()
	if currentInst == nil {
		return m, nil
	}

	currentDisplayIdx := -1
	for i, id := range displayOrder {
		if id == currentInst.ID {
			currentDisplayIdx = i
			break
		}
	}

	if currentDisplayIdx == -1 {
		// Current instance not found in display order, fall back to first
		currentDisplayIdx = 0
	}

	// Move to next in display order (with wrap-around)
	nextDisplayIdx := (currentDisplayIdx + 1) % len(displayOrder)
	nextID := displayOrder[nextDisplayIdx]

	// Find the session.Instances index for the target instance
	newTab := m.findInstanceIndexByID(nextID)
	if newTab >= 0 {
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
}

// handlePrevInstance switches to the previous instance in display order.
// This navigates based on how instances appear in the sidebar, not by creation order.
func (m Model) handlePrevInstance() (tea.Model, tea.Cmd) {
	displayOrder := m.getInstanceDisplayOrder()
	if len(displayOrder) == 0 {
		return m, nil
	}

	// Find current instance's position in display order
	currentInst := m.activeInstance()
	if currentInst == nil {
		return m, nil
	}

	currentDisplayIdx := -1
	for i, id := range displayOrder {
		if id == currentInst.ID {
			currentDisplayIdx = i
			break
		}
	}

	if currentDisplayIdx == -1 {
		// Current instance not found in display order, fall back to last
		currentDisplayIdx = len(displayOrder) - 1
	}

	// Move to previous in display order (with wrap-around)
	prevDisplayIdx := (currentDisplayIdx - 1 + len(displayOrder)) % len(displayOrder)
	prevID := displayOrder[prevDisplayIdx]

	// Find the session.Instances index for the target instance
	newTab := m.findInstanceIndexByID(prevID)
	if newTab >= 0 {
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
}

// handleEnterInputMode enters input mode for the active instance.
func (m Model) handleEnterInputMode() (tea.Model, tea.Cmd) {
	// Enter input mode for the active instance
	// Allow input if tmux session exists (running or waiting for input)
	if inst := m.activeInstance(); inst != nil {
		mgr := m.orchestrator.GetInstanceManager(inst.ID)
		if mgr != nil && mgr.TmuxSessionExists() {
			m.inputMode = true
		}
	}
	return m, nil
}

// handleToggleTerminal toggles terminal pane visibility.
func (m Model) handleToggleTerminal() (tea.Model, tea.Cmd) {
	// Check if terminal support is enabled
	if !viper.GetBool("experimental.terminal_support") {
		return m, nil
	}
	sessionID := ""
	if m.orchestrator != nil {
		sessionID = m.orchestrator.SessionID()
	}
	m.toggleTerminalVisibility(sessionID)
	return m, nil
}

// handleSwitchTerminalDir switches terminal directory mode.
func (m Model) handleSwitchTerminalDir() (tea.Model, tea.Cmd) {
	// Check if terminal support is enabled
	if !viper.GetBool("experimental.terminal_support") {
		return m, nil
	}
	if m.terminalManager.IsVisible() {
		m.switchTerminalDir()
	}
	return m, nil
}

// handleEscape handles the escape key in normal mode.
func (m Model) handleEscape() (tea.Model, tea.Cmd) {
	// Close diff panel if open
	if m.showDiff {
		m.showDiff = false
		m.diffContent = ""
		m.diffScroll = 0
		return m, nil
	}
	return m, nil
}

// handleScrollDown scrolls down in diff view, help panel, output view, or navigates.
func (m Model) handleScrollDown() (tea.Model, tea.Cmd) {
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
}

// handleScrollUp scrolls up in diff view, help panel, output view, or navigates.
func (m Model) handleScrollUp() (tea.Model, tea.Cmd) {
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
}

// handleHalfPageUp scrolls up half page in help panel or output view.
func (m Model) handleHalfPageUp() (tea.Model, tea.Cmd) {
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
}

// handleHalfPageDown scrolls down half page in help panel or output view.
func (m Model) handleHalfPageDown() (tea.Model, tea.Cmd) {
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
}

// handleFullPageUp scrolls up full page in help panel or output view.
func (m Model) handleFullPageUp() (tea.Model, tea.Cmd) {
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
}

// handleFullPageDown scrolls down full page in help panel or output view.
func (m Model) handleFullPageDown() (tea.Model, tea.Cmd) {
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
}

// handleRestartInstance restarts the active instance with the same task.
func (m Model) handleRestartInstance() (tea.Model, tea.Cmd) {
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
}

// handleKillInstance kills and removes the active instance.
func (m Model) handleKillInstance() (tea.Model, tea.Cmd) {
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
}

// handleGoToTop goes to the top of diff, help panel, or output.
func (m Model) handleGoToTop() (tea.Model, tea.Cmd) {
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
}

// handleGoToBottom goes to the bottom of diff, help panel, or output.
func (m Model) handleGoToBottom() (tea.Model, tea.Cmd) {
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
}

// handleSidebarScrollUp scrolls the sidebar viewport up without changing selection.
// This allows viewing instances above the current viewport while keeping the
// currently selected instance unchanged.
func (m Model) handleSidebarScrollUp() (tea.Model, tea.Cmd) {
	if m.sidebarScrollOffset > 0 {
		m.sidebarScrollOffset--
	}
	return m, nil
}

// handleSidebarScrollDown scrolls the sidebar viewport down without changing selection.
// This allows viewing instances below the current viewport while keeping the
// currently selected instance unchanged.
func (m Model) handleSidebarScrollDown() (tea.Model, tea.Cmd) {
	// Calculate max scroll offset based on number of items and visible space
	maxOffset := m.calculateSidebarMaxScrollOffset()
	if m.sidebarScrollOffset < maxOffset {
		m.sidebarScrollOffset++
	}
	return m, nil
}

// -----------------------------------------------------------------------------
// Search Input Handler
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Filter Input Handler
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Branch Selector Handler
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Template Dropdown Handler
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Input Mode Query
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Filter Output
// -----------------------------------------------------------------------------

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

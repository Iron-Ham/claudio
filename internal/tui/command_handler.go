package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
)

// CommandHandler manages vim-style command mode state and execution.
// It handles commands entered after pressing ':' in the TUI.
type CommandHandler struct {
	// active indicates command mode is currently active (user pressed ':')
	active bool

	// buffer holds the command being typed (without the ':' prefix)
	buffer string
}

// NewCommandHandler creates a new CommandHandler in inactive state.
func NewCommandHandler() *CommandHandler {
	return &CommandHandler{
		active: false,
		buffer: "",
	}
}

// IsActive returns true if command mode is currently active.
func (h *CommandHandler) IsActive() bool {
	return h.active
}

// Buffer returns the current command buffer contents.
func (h *CommandHandler) Buffer() string {
	return h.buffer
}

// Enter activates command mode and clears the buffer.
func (h *CommandHandler) Enter() {
	h.active = true
	h.buffer = ""
}

// Exit deactivates command mode and clears the buffer.
func (h *CommandHandler) Exit() {
	h.active = false
	h.buffer = ""
}

// HandleInput processes keyboard input while in command mode.
// Returns (command, done) where command is the entered command if Enter was pressed,
// and done indicates whether command mode should exit.
func (h *CommandHandler) HandleInput(msg tea.KeyMsg) (command string, done bool) {
	switch msg.Type {
	case tea.KeyEsc:
		// Exit command mode without executing
		h.Exit()
		return "", true

	case tea.KeyEnter:
		// Execute the command and exit command mode
		cmd := h.buffer
		h.Exit()
		return cmd, true

	case tea.KeyBackspace, tea.KeyDelete:
		// Delete last character from command buffer
		if len(h.buffer) > 0 {
			h.buffer = h.buffer[:len(h.buffer)-1]
		}
		// If buffer is empty after backspace, exit command mode
		if len(h.buffer) == 0 {
			h.Exit()
			return "", true
		}
		return "", false

	case tea.KeySpace:
		h.buffer += " "
		return "", false

	case tea.KeyRunes:
		// Add typed characters to the command buffer
		h.buffer += string(msg.Runes)
		return "", false
	}

	return "", false
}

// handleCommandInput handles keystrokes when in command mode (after pressing ':')
// This method is called from Model and delegates to the CommandHandler.
func (m Model) handleCommandInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If handler is available, use it
	if m.commandHandler != nil {
		command, done := m.commandHandler.HandleInput(msg)

		// Sync legacy fields for backward compatibility with tests
		m.commandMode = m.commandHandler.IsActive()
		m.commandBuffer = m.commandHandler.Buffer()

		if done && command != "" {
			return m.executeCommand(command)
		}
		return m, nil
	}

	// Fallback: handle directly using legacy fields (for backward compatibility with tests)
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

// executeCommand parses and executes a vim-style command.
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
		return m, tea.Quit

	default:
		m.errorMessage = fmt.Sprintf("Unknown command: %s (type :h for help)", cmd)
		return m, nil
	}
}

// Command implementations - Instance control

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

// Command implementations - Instance management

func (m Model) cmdAdd() (tea.Model, tea.Cmd) {
	m.addingTask = true
	m.taskInput.Clear()
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

// Command implementations - View toggles

func (m Model) cmdDiff() (tea.Model, tea.Cmd) {
	if m.getDiffState().IsVisible() {
		m.getDiffState().Hide()
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
		m.getDiffState().Show(diff)
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

// Command implementations - Utilities

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

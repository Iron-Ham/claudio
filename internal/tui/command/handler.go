// Package command provides command handling for the TUI.
// It extracts and encapsulates the vim-style command processing from the main TUI model.
package command

import (
	"fmt"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
)

// Dependencies defines the interface for dependencies that the CommandHandler needs.
// This allows the handler to be decoupled from the TUI Model while still accessing
// necessary state and services.
type Dependencies interface {
	// Orchestrator access
	GetOrchestrator() *orchestrator.Orchestrator
	GetSession() *orchestrator.Session

	// Instance access
	ActiveInstance() *orchestrator.Instance
	InstanceCount() int

	// State queries
	GetConflicts() int
	IsTerminalVisible() bool
	IsDiffVisible() bool
	GetDiffContent() string
	IsUltraPlanMode() bool
	GetUltraPlanCoordinator() *orchestrator.Coordinator

	// Logger access
	GetLogger() *logging.Logger
	GetStartTime() time.Time
}

// Result represents the outcome of executing a command.
// It contains state changes that should be applied to the Model.
type Result struct {
	// InfoMessage is a non-error status message to display
	InfoMessage string

	// ErrorMessage is an error message to display
	ErrorMessage string

	// TeaCmd is an optional tea.Cmd to return (e.g., tea.Quit)
	TeaCmd tea.Cmd

	// State changes (use pointers to distinguish "not set" from "set to false/zero")
	ShowHelp      *bool
	ShowStats     *bool
	ShowDiff      *bool
	ShowConflicts *bool
	Quitting      *bool
	AddingTask    *bool
	FilterMode    *bool
	DiffContent   *string
	DiffScroll    *int

	// ActiveTabAdjustment indicates how to adjust the active tab after instance removal
	// -1 = decrement if needed, 0 = no change needed, positive = specific check needed
	ActiveTabAdjustment int
	EnsureActiveVisible bool

	// Terminal-related state changes
	EnterTerminalMode bool
	ToggleTerminal    bool // signals that terminal visibility should be toggled
	TerminalDirMode   *int // 0 = invocation, 1 = worktree
}

// Handler processes vim-style commands for the TUI.
type Handler struct {
	commands map[string]commandFunc
}

// commandFunc is the signature for command implementations.
// It receives the dependencies interface and returns a Result.
type commandFunc func(deps Dependencies) Result

// New creates a new CommandHandler with all commands registered.
func New() *Handler {
	h := &Handler{
		commands: make(map[string]commandFunc),
	}
	h.registerCommands()
	return h
}

// Execute parses and executes a command string.
// Returns a Result containing state changes and any tea.Cmd to execute.
func (h *Handler) Execute(cmd string, deps Dependencies) Result {
	// Trim whitespace
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return Result{}
	}

	// Look up the command
	if fn, ok := h.commands[cmd]; ok {
		return fn(deps)
	}

	// Unknown command
	return Result{
		ErrorMessage: fmt.Sprintf("Unknown command: %s (type :h for help)", cmd),
	}
}

// registerCommands sets up all command mappings.
func (h *Handler) registerCommands() {
	// Instance control commands
	h.commands["s"] = cmdStart
	h.commands["start"] = cmdStart
	h.commands["x"] = cmdStop
	h.commands["stop"] = cmdStop
	h.commands["e"] = cmdExit
	h.commands["exit"] = cmdExit
	h.commands["p"] = cmdPause
	h.commands["pause"] = cmdPause
	h.commands["R"] = cmdReconnect
	h.commands["reconnect"] = cmdReconnect
	h.commands["restart"] = cmdRestart

	// Instance management commands
	h.commands["a"] = cmdAdd
	h.commands["add"] = cmdAdd
	h.commands["D"] = cmdRemove
	h.commands["remove"] = cmdRemove
	h.commands["kill"] = cmdKill
	h.commands["C"] = cmdClearCompleted
	h.commands["clear"] = cmdClearCompleted

	// View toggle commands
	h.commands["d"] = cmdDiff
	h.commands["diff"] = cmdDiff
	h.commands["m"] = cmdStats
	h.commands["metrics"] = cmdStats
	h.commands["stats"] = cmdStats
	h.commands["c"] = cmdConflicts
	h.commands["conflicts"] = cmdConflicts
	h.commands["f"] = cmdFilter
	h.commands["F"] = cmdFilter
	h.commands["filter"] = cmdFilter

	// Utility commands
	h.commands["tmux"] = cmdTmux
	h.commands["r"] = cmdPR
	h.commands["pr"] = cmdPR

	// Terminal pane commands
	h.commands["t"] = cmdTerminalFocus
	h.commands["term"] = cmdTerminal
	h.commands["terminal"] = cmdTerminal
	h.commands["termdir worktree"] = cmdTerminalDirWorktree
	h.commands["termdir wt"] = cmdTerminalDirWorktree
	h.commands["termdir invoke"] = cmdTerminalDirInvocation
	h.commands["termdir invocation"] = cmdTerminalDirInvocation

	// Ultraplan commands
	h.commands["cancel"] = cmdUltraPlanCancel

	// Help commands
	h.commands["h"] = cmdHelp
	h.commands["help"] = cmdHelp
	h.commands["q"] = cmdQuit
	h.commands["quit"] = cmdQuit
}

// Command implementations

func cmdStart(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	// Guard against starting already-running instances
	if inst.Status == orchestrator.StatusWorking || inst.Status == orchestrator.StatusWaitingInput {
		return Result{InfoMessage: "Instance is already running. Use :p to pause/resume or :x to stop."}
	}
	if inst.Status == orchestrator.StatusCreatingPR {
		return Result{InfoMessage: "Instance is creating PR. Wait for it to complete."}
	}

	if err := orch.StartInstance(inst); err != nil {
		return Result{ErrorMessage: err.Error()}
	}
	return Result{InfoMessage: fmt.Sprintf("Started instance %s", inst.ID)}
}

func cmdStop(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	// Log user stopping instance
	if logger := deps.GetLogger(); logger != nil {
		logger.Info("user stopped instance", "instance_id", inst.ID)
	}

	prStarted, err := orch.StopInstanceWithAutoPR(inst)
	if err != nil {
		return Result{ErrorMessage: err.Error()}
	}
	if prStarted {
		return Result{InfoMessage: fmt.Sprintf("Instance stopped. Creating PR for %s...", inst.ID)}
	}
	return Result{InfoMessage: fmt.Sprintf("Instance stopped. Create PR with: claudio pr %s", inst.ID)}
}

func cmdExit(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	// Log user exiting instance
	if logger := deps.GetLogger(); logger != nil {
		logger.Info("user exited instance (no auto-PR)", "instance_id", inst.ID)
	}

	// Stop without auto-PR workflow
	if err := orch.StopInstance(inst); err != nil {
		return Result{ErrorMessage: err.Error()}
	}
	return Result{InfoMessage: fmt.Sprintf("Instance %s stopped (no PR workflow). Create PR manually with: claudio pr %s", inst.ID, inst.ID)}
}

func cmdPause(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	mgr := orch.GetInstanceManager(inst.ID)
	if mgr == nil {
		return Result{InfoMessage: "Instance has no manager"}
	}

	switch inst.Status {
	case orchestrator.StatusPaused:
		_ = mgr.Resume()
		inst.Status = orchestrator.StatusWorking
		return Result{InfoMessage: fmt.Sprintf("Resumed instance %s", inst.ID)}
	case orchestrator.StatusWorking:
		_ = mgr.Pause()
		inst.Status = orchestrator.StatusPaused
		return Result{InfoMessage: fmt.Sprintf("Paused instance %s", inst.ID)}
	default:
		return Result{InfoMessage: "Instance is not in a pausable state"}
	}
}

func cmdReconnect(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	// Only allow reconnecting to non-running instances
	if inst.Status == orchestrator.StatusWorking || inst.Status == orchestrator.StatusWaitingInput {
		return Result{InfoMessage: "Instance is already running. Use :p to pause/resume or :x to stop."}
	}
	if inst.Status == orchestrator.StatusCreatingPR {
		return Result{InfoMessage: "Instance is creating PR. Wait for it to complete."}
	}

	if err := orch.ReconnectInstance(inst); err != nil {
		return Result{ErrorMessage: fmt.Sprintf("Failed to reconnect: %v", err)}
	}
	return Result{InfoMessage: fmt.Sprintf("Reconnected to instance %s", inst.ID)}
}

func cmdRestart(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	// Only allow restarting non-running instances
	switch inst.Status {
	case orchestrator.StatusWorking, orchestrator.StatusWaitingInput:
		return Result{InfoMessage: "Instance is running. Use :x to stop it first, or :p to pause."}
	case orchestrator.StatusCreatingPR:
		return Result{InfoMessage: "Instance is creating PR. Wait for it to complete."}
	}

	// Stop the instance if it's still running in tmux
	mgr := orch.GetInstanceManager(inst.ID)
	if mgr != nil {
		_ = mgr.Stop()
		mgr.ClearTimeout() // Reset timeout state
	}

	// Restart with same task
	if err := orch.ReconnectInstance(inst); err != nil {
		return Result{ErrorMessage: fmt.Sprintf("Failed to restart instance: %v", err)}
	}
	return Result{InfoMessage: fmt.Sprintf("Instance %s restarted with same task", inst.ID)}
}

func cmdAdd(_ Dependencies) Result {
	addingTask := true
	return Result{AddingTask: &addingTask}
}

func cmdRemove(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	session := deps.GetSession()
	if orch == nil || session == nil {
		return Result{ErrorMessage: "No orchestrator or session available"}
	}

	instanceID := inst.ID
	if err := orch.RemoveInstance(session, instanceID, true); err != nil {
		return Result{ErrorMessage: fmt.Sprintf("Failed to remove instance: %v", err)}
	}

	return Result{
		InfoMessage:         fmt.Sprintf("Removed instance %s", instanceID),
		ActiveTabAdjustment: -1,
		EnsureActiveVisible: true,
	}
}

func cmdKill(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	session := deps.GetSession()
	if orch == nil || session == nil {
		return Result{ErrorMessage: "No orchestrator or session available"}
	}

	// Stop the instance first
	mgr := orch.GetInstanceManager(inst.ID)
	if mgr != nil {
		_ = mgr.Stop()
	}

	// Remove the instance
	if err := orch.RemoveInstance(session, inst.ID, true); err != nil {
		return Result{ErrorMessage: fmt.Sprintf("Failed to remove instance: %v", err)}
	}

	return Result{
		InfoMessage:         fmt.Sprintf("Instance %s killed and removed", inst.ID),
		ActiveTabAdjustment: -1,
		EnsureActiveVisible: true,
	}
}

func cmdClearCompleted(deps Dependencies) Result {
	orch := deps.GetOrchestrator()
	session := deps.GetSession()
	if orch == nil || session == nil {
		return Result{ErrorMessage: "No orchestrator or session available"}
	}

	removed, err := orch.ClearCompletedInstances(session)
	if err != nil {
		return Result{ErrorMessage: err.Error()}
	}
	if removed == 0 {
		return Result{InfoMessage: "No completed instances to clear"}
	}

	return Result{
		InfoMessage:         fmt.Sprintf("Cleared %d completed instance(s)", removed),
		ActiveTabAdjustment: -1,
		EnsureActiveVisible: true,
	}
}

func cmdDiff(deps Dependencies) Result {
	// If diff is currently visible, hide it
	if deps.IsDiffVisible() {
		showDiff := false
		diffContent := ""
		diffScroll := 0
		return Result{
			ShowDiff:    &showDiff,
			DiffContent: &diffContent,
			DiffScroll:  &diffScroll,
		}
	}

	// Otherwise, show diff for active instance
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	diff, err := orch.GetInstanceDiff(inst.WorktreePath)
	if err != nil {
		return Result{ErrorMessage: fmt.Sprintf("Failed to get diff: %v", err)}
	}
	if diff == "" {
		return Result{InfoMessage: "No changes to show"}
	}

	showDiff := true
	diffScroll := 0
	return Result{
		ShowDiff:    &showDiff,
		DiffContent: &diff,
		DiffScroll:  &diffScroll,
	}
}

func cmdStats(_ Dependencies) Result {
	showStats := true
	return Result{ShowStats: &showStats}
}

func cmdConflicts(deps Dependencies) Result {
	if deps.GetConflicts() > 0 {
		showConflicts := true
		return Result{ShowConflicts: &showConflicts}
	}
	return Result{InfoMessage: "No conflicts detected"}
}

func cmdFilter(_ Dependencies) Result {
	filterMode := true
	return Result{FilterMode: &filterMode}
}

func cmdTerminal(_ Dependencies) Result {
	return Result{ToggleTerminal: true}
}

func cmdTerminalFocus(deps Dependencies) Result {
	if deps.IsTerminalVisible() {
		return Result{
			EnterTerminalMode: true,
			InfoMessage:       "Terminal focused. Press Ctrl+] to exit.",
		}
	}
	return Result{ErrorMessage: "Terminal not visible. Use :term to open it first."}
}

func cmdTerminalDirWorktree(_ Dependencies) Result {
	mode := 1 // TerminalDirWorktree
	return Result{TerminalDirMode: &mode}
}

func cmdTerminalDirInvocation(_ Dependencies) Result {
	mode := 0 // TerminalDirInvocation
	return Result{TerminalDirMode: &mode}
}

func cmdTmux(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	mgr := orch.GetInstanceManager(inst.ID)
	if mgr == nil {
		return Result{InfoMessage: "Instance has no manager"}
	}

	return Result{InfoMessage: "Attach with: " + mgr.AttachCommand()}
}

func cmdPR(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	return Result{InfoMessage: fmt.Sprintf("Create PR: claudio pr %s  (add --draft for draft PR)", inst.ID)}
}

func cmdUltraPlanCancel(deps Dependencies) Result {
	if !deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Not in ultraplan mode"}
	}

	coordinator := deps.GetUltraPlanCoordinator()
	if coordinator == nil {
		return Result{ErrorMessage: "No active ultraplan session"}
	}

	session := coordinator.Session()
	if session == nil {
		return Result{ErrorMessage: "No active ultraplan session"}
	}

	// Only allow cancellation during executing phase
	if session.Phase != orchestrator.PhaseExecuting {
		return Result{ErrorMessage: "Can only cancel during execution phase"}
	}

	coordinator.Cancel()

	// Log user decision
	if logger := deps.GetLogger(); logger != nil {
		logger.Info("user cancelled ultraplan execution via command mode")
	}

	return Result{InfoMessage: "Execution cancelled"}
}

func cmdHelp(_ Dependencies) Result {
	showHelp := true
	return Result{ShowHelp: &showHelp}
}

func cmdQuit(deps Dependencies) Result {
	quitting := true

	// Log session end with duration
	if logger := deps.GetLogger(); logger != nil {
		duration := time.Since(deps.GetStartTime())
		logger.Info("TUI session ended", "duration_ms", duration.Milliseconds())
	}

	return Result{
		Quitting: &quitting,
		TeaCmd:   tea.Quit,
	}
}

// Ensure Handler doesn't unexpectedly reference instance package types
// that would break if imported elsewhere
var _ = instance.TimeoutActivity // compile-time check that instance is available

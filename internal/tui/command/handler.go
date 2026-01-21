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
	"github.com/Iron-Ham/claudio/internal/orchestrator/prworkflow"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/msg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
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
	IsTripleShotMode() bool
	IsAdversarialMode() bool
	GetUltraPlanCoordinator() *orchestrator.Coordinator
	GetTripleShotCoordinators() []*tripleshot.Coordinator // Returns all active tripleshot coordinators

	// Logger access
	GetLogger() *logging.Logger
	GetStartTime() time.Time

	// IsInstanceTripleShotJudge checks if an instance is a judge in any active triple-shot session
	IsInstanceTripleShotJudge(instanceID string) bool

	// IsRalphMode returns true if there are any active ralph loop sessions
	IsRalphMode() bool

	// GetRalphCoordinators returns all active ralph coordinators
	GetRalphCoordinators() []*ralph.Coordinator
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

	// AddingDependentTask signals entering dependent task input mode
	// DependentOnInstanceID is the ID of the instance the new task will depend on
	AddingDependentTask   *bool
	DependentOnInstanceID *string

	// ActiveTabAdjustment indicates how to adjust the active tab after instance removal
	// -1 = decrement if needed, 0 = no change needed, positive = specific check needed
	ActiveTabAdjustment int
	EnsureActiveVisible bool

	// Terminal-related state changes
	EnterTerminalMode bool
	ToggleTerminal    bool // signals that terminal visibility should be toggled
	TerminalDirMode   *int // 0 = invocation, 1 = worktree

	// Mode transition - Triple-Shot
	StartTripleShot *bool // Request to switch to triple-shot mode

	// Mode transition - Adversarial
	StartAdversarial *bool // Request to switch to adversarial mode

	// StoppedTripleShotJudgeID is set when a stopped instance was a triple-shot judge.
	// The TUI should clean up the corresponding triple-shot session.
	StoppedTripleShotJudgeID *string

	// Mode transition - Plan Mode
	StartPlanMode      *bool // Request to switch to inline plan mode
	StartMultiPlanMode *bool // Request to switch to inline multi-pass plan mode (3 planners + 1 assessor)

	// Mode transition - UltraPlan Mode
	StartUltraPlanMode *bool   // Request to switch to inline ultraplan mode
	UltraPlanMultiPass *bool   // If true, use multi-pass planning
	UltraPlanFromFile  *string // If set, load plan from this file path
	UltraPlanObjective *string // Optional objective (if not loading from file)

	// View transition - Grouped View
	ToggleGroupedView *bool // Request to toggle grouped instance view on/off

	// Group PR workflow
	StartGroupPR   *bool                   // Request to start a group PR workflow
	GroupPRMode    *prworkflow.GroupPRMode // Mode for group PR creation (stacked, consolidated, single)
	GroupPRGroupID *string                 // Target group ID for single group PR mode

	// Mode transition - Ralph Wiggum Loop
	StartRalphLoop         *bool   // Request to start a ralph loop
	CancelRalphLoop        *bool   // Request to cancel active ralph loop
	RalphPrompt            *string // The prompt for ralph loop
	RalphMaxIterations     *int    // Max iterations (safety limit)
	RalphCompletionPromise *string // Phrase that signals completion
}

// CommandInfo contains metadata about a command for help display.
type CommandInfo struct {
	// ShortKey is the single-letter shortcut (e.g., "s", "x")
	ShortKey string
	// LongKey is the full command name (e.g., "start", "stop")
	LongKey string
	// Description is a brief description of what the command does
	Description string
	// Category groups related commands together
	Category string
}

// CommandCategory represents a group of related commands.
type CommandCategory struct {
	Name     string
	Commands []CommandInfo
}

// CommandFlagInfo documents a flag for a command that accepts arguments.
// These flags are used in the TUI command mode (e.g., ":ultraplan --multi-pass").
type CommandFlagInfo struct {
	// Command is the base command (e.g., "ultraplan", "pr")
	Command string
	// Flag is the flag syntax as it should appear in help (e.g., "--multi-pass", "--plan <file>")
	Flag string
	// Description is a brief description of what the flag does
	Description string
}

// Handler processes vim-style commands for the TUI.
type Handler struct {
	commands    map[string]commandFunc
	argCommands map[string]commandArgFunc // Commands that accept arguments
	categories  []CommandCategory
	flags       []CommandFlagInfo
}

// commandFunc is the signature for command implementations.
// It receives the dependencies interface and returns a Result.
type commandFunc func(deps Dependencies) Result

// commandArgFunc is the signature for commands that accept arguments.
// It receives the dependencies interface and the argument string after the command name.
type commandArgFunc func(deps Dependencies, args string) Result

// New creates a new CommandHandler with all commands registered.
func New() *Handler {
	h := &Handler{
		commands:    make(map[string]commandFunc),
		argCommands: make(map[string]commandArgFunc),
	}
	h.registerCommands()
	h.buildCategories()
	h.buildFlags()
	return h
}

// Categories returns the command categories for help display.
func (h *Handler) Categories() []CommandCategory {
	return h.categories
}

// Flags returns the documented command flags for help display validation.
func (h *Handler) Flags() []CommandFlagInfo {
	return h.flags
}

// Execute parses and executes a command string.
// Returns a Result containing state changes and any tea.Cmd to execute.
func (h *Handler) Execute(cmd string, deps Dependencies) Result {
	// Trim whitespace
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return Result{}
	}

	// Look up exact command match first
	if fn, ok := h.commands[cmd]; ok {
		return fn(deps)
	}

	// Check for arg-based commands (command word + optional arguments)
	// Split into command word and arguments
	parts := strings.SplitN(cmd, " ", 2)
	cmdWord := parts[0]
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	// Look up arg-based command
	if fn, ok := h.argCommands[cmdWord]; ok {
		return fn(deps, args)
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
	h.argCommands["chain"] = cmdChain
	h.argCommands["dep"] = cmdChain
	h.argCommands["depends"] = cmdChain
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
	h.argCommands["r"] = cmdPRWithArgs
	h.argCommands["pr"] = cmdPRWithArgs

	// Terminal pane commands
	h.commands["t"] = cmdTerminalFocus
	h.commands["term"] = cmdTerminal
	h.commands["terminal"] = cmdTerminal
	h.commands["termdir worktree"] = cmdTerminalDirWorktree
	h.commands["termdir wt"] = cmdTerminalDirWorktree
	h.commands["termdir project"] = cmdTerminalDirProject
	h.commands["termdir proj"] = cmdTerminalDirProject
	// Legacy aliases for backward compatibility
	h.commands["termdir invoke"] = cmdTerminalDirProject
	h.commands["termdir invocation"] = cmdTerminalDirProject

	// Ultraplan commands
	h.commands["cancel"] = cmdUltraPlanCancel
	h.argCommands["ultraplan"] = cmdUltraPlan
	h.argCommands["up"] = cmdUltraPlan

	// Triple-shot commands
	h.commands["tripleshot"] = cmdTripleShot
	h.commands["triple"] = cmdTripleShot
	h.commands["3shot"] = cmdTripleShot

	// Adversarial commands
	h.commands["adversarial"] = cmdAdversarial
	h.commands["adv"] = cmdAdversarial

	// Plan mode commands
	h.commands["plan"] = cmdPlan
	h.commands["multiplan"] = cmdMultiPlan
	h.commands["mp"] = cmdMultiPlan

	// Ralph Wiggum loop commands
	h.argCommands["ralph"] = cmdRalph
	h.argCommands["ralph-loop"] = cmdRalph
	h.commands["cancel-ralph"] = cmdCancelRalph
	h.commands["ralph-cancel"] = cmdCancelRalph

	// Group management commands
	h.argCommands["group"] = func(deps Dependencies, args string) Result {
		return executeGroupCommand(args, deps)
	}

	// Help commands
	h.commands["h"] = cmdHelp
	h.commands["help"] = cmdHelp
	h.commands["q"] = cmdQuit
	h.commands["quit"] = cmdQuit
	h.commands["q!"] = cmdQuitForce
	h.commands["quit!"] = cmdQuitForce
}

// buildCategories populates the categories slice with command metadata for help display.
func (h *Handler) buildCategories() {
	h.categories = []CommandCategory{
		{
			Name: "Instance Control",
			Commands: []CommandInfo{
				{ShortKey: "s", LongKey: "start", Description: "Start a stopped/new instance", Category: "control"},
				{ShortKey: "x", LongKey: "stop", Description: "Stop instance and trigger auto-PR workflow", Category: "control"},
				{ShortKey: "e", LongKey: "exit", Description: "Stop instance without auto-PR", Category: "control"},
				{ShortKey: "p", LongKey: "pause", Description: "Pause/resume a running instance", Category: "control"},
				{ShortKey: "R", LongKey: "reconnect", Description: "Reattach to a stopped instance's tmux session", Category: "control"},
				{ShortKey: "", LongKey: "restart", Description: "Restart a stuck or timed-out instance", Category: "control"},
			},
		},
		{
			Name: "Instance Management",
			Commands: []CommandInfo{
				{ShortKey: "a", LongKey: "add", Description: "Create and add a new instance", Category: "management"},
				{ShortKey: "", LongKey: "chain", Description: "Add task that auto-starts after selected instance", Category: "management"},
				{ShortKey: "D", LongKey: "remove", Description: "Remove instance from session", Category: "management"},
				{ShortKey: "", LongKey: "kill", Description: "Force kill instance process and remove from session", Category: "management"},
				{ShortKey: "C", LongKey: "clear", Description: "Remove all completed instances", Category: "management"},
			},
		},
		{
			Name: "View",
			Commands: []CommandInfo{
				{ShortKey: "d", LongKey: "diff", Description: "Toggle diff preview panel", Category: "view"},
				{ShortKey: "m", LongKey: "stats", Description: "Toggle metrics panel", Category: "view"},
				{ShortKey: "c", LongKey: "conflicts", Description: "Toggle conflict view", Category: "view"},
				{ShortKey: "f", LongKey: "filter", Description: "Open filter panel", Category: "view"},
			},
		},
		{
			Name: "Terminal",
			Commands: []CommandInfo{
				{ShortKey: "t", LongKey: "term", Description: "Focus/toggle terminal pane", Category: "terminal"},
				{ShortKey: "", LongKey: "tmux", Description: "Show tmux attach command", Category: "terminal"},
			},
		},
		{
			Name: "Utility",
			Commands: []CommandInfo{
				{ShortKey: "r", LongKey: "pr", Description: "Show PR creation command", Category: "utility"},
				{ShortKey: "", LongKey: "pr --group", Description: "Create stacked PRs for all groups", Category: "utility"},
				{ShortKey: "", LongKey: "pr --group=all", Description: "Create consolidated PR from all groups", Category: "utility"},
				{ShortKey: "", LongKey: "pr --group=single", Description: "Create PR for current group only", Category: "utility"},
				{ShortKey: "", LongKey: "cancel", Description: "Cancel ultra-plan execution", Category: "utility"},
				{ShortKey: "", LongKey: "tripleshot", Description: "Start triple-shot mode (3 parallel attempts + judge)", Category: "utility"},
				{ShortKey: "", LongKey: "adversarial", Description: "Start adversarial mode (implementer + reviewer feedback loop)", Category: "utility"},
				{ShortKey: "", LongKey: "plan", Description: "Start inline plan mode for structured task planning", Category: "utility"},
				{ShortKey: "", LongKey: "multiplan", Description: "Start multi-pass plan mode (3 planners + 1 assessor)", Category: "utility"},
				{ShortKey: "", LongKey: "ultraplan", Description: "Start ultraplan mode (use --multi-pass or --plan flags)", Category: "utility"},
				{ShortKey: "", LongKey: "ralph", Description: "Start Ralph Wiggum iterative loop (requires --completion-promise)", Category: "utility"},
				{ShortKey: "", LongKey: "cancel-ralph", Description: "Cancel active Ralph loop", Category: "utility"},
			},
		},
		{
			Name: "Session",
			Commands: []CommandInfo{
				{ShortKey: "h", LongKey: "help", Description: "Toggle help panel", Category: "session"},
				{ShortKey: "q", LongKey: "quit", Description: "Quit Claudio", Category: "session"},
				{ShortKey: "q!", LongKey: "quit!", Description: "Force quit: stop all instances, cleanup worktrees, exit", Category: "session"},
			},
		},
		{
			Name: "Group Management",
			Commands: []CommandInfo{
				{ShortKey: "", LongKey: "group create", Description: "Create a new empty group", Category: "group"},
				{ShortKey: "", LongKey: "group add", Description: "Add instance to a group", Category: "group"},
				{ShortKey: "", LongKey: "group remove", Description: "Remove instance from its group", Category: "group"},
				{ShortKey: "", LongKey: "group move", Description: "Move instance to a different group", Category: "group"},
				{ShortKey: "", LongKey: "group order", Description: "Reorder group execution sequence", Category: "group"},
				{ShortKey: "", LongKey: "group delete", Description: "Delete an empty group", Category: "group"},
				{ShortKey: "", LongKey: "group show", Description: "Toggle grouped instance view on/off", Category: "group"},
			},
		},
	}
}

// buildFlags populates the flags slice with all documented command flags.
// These flags should be documented in the help panel (DefaultHelpSections).
//
// IMPORTANT: When adding new flags to TUI commands, add them here too.
// The TestDefaultHelpSectionsContainsAllFlags test will fail if any
// flags listed here are not documented in the help panel.
func (h *Handler) buildFlags() {
	h.flags = []CommandFlagInfo{
		// Ultraplan flags
		{Command: "ultraplan", Flag: "--multi-pass", Description: "Use multi-pass planning (3 strategies)"},
		{Command: "ultraplan", Flag: "--plan <file>", Description: "Load plan from existing file"},

		// Ralph Wiggum loop flags
		{Command: "ralph", Flag: "--completion-promise <text>", Description: "Phrase that signals loop completion (required)"},
		{Command: "ralph", Flag: "--max-iterations <n>", Description: "Safety limit on iterations (default: 50)"},

		// PR flags (these are already documented as separate commands in buildCategories,
		// so we only need to add truly new flags here that aren't full commands)
	}
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

	// Check if this is a triple-shot judge before stopping
	isJudge := deps.IsInstanceTripleShotJudge(inst.ID)

	// Log user stopping instance
	if logger := deps.GetLogger(); logger != nil {
		logger.Info("user stopped instance", "instance_id", inst.ID)
	}

	prStarted, err := orch.StopInstanceWithAutoPR(inst)
	if err != nil {
		return Result{ErrorMessage: err.Error()}
	}

	result := Result{}
	if prStarted {
		result.InfoMessage = fmt.Sprintf("Instance stopped. Creating PR for %s...", inst.ID)
	} else {
		result.InfoMessage = fmt.Sprintf("Instance stopped. Create PR with: claudio pr %s", inst.ID)
	}

	// If this was a triple-shot judge, signal to clean up the triple-shot session
	if isJudge {
		result.StoppedTripleShotJudgeID = &inst.ID
	}

	return result
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

	// Check if this is a triple-shot judge before stopping
	isJudge := deps.IsInstanceTripleShotJudge(inst.ID)

	// Log user exiting instance
	if logger := deps.GetLogger(); logger != nil {
		logger.Info("user exited instance (no auto-PR)", "instance_id", inst.ID)
	}

	// Stop without auto-PR workflow
	if err := orch.StopInstance(inst); err != nil {
		return Result{ErrorMessage: err.Error()}
	}

	result := Result{
		InfoMessage: fmt.Sprintf("Instance %s stopped (no PR workflow). Create PR manually with: claudio pr %s", inst.ID, inst.ID),
	}

	// If this was a triple-shot judge, signal to clean up the triple-shot session
	if isJudge {
		result.StoppedTripleShotJudgeID = &inst.ID
	}

	return result
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

	// Check if we're in ultraplan mode - if so, use step restart
	if deps.IsUltraPlanMode() {
		coordinator := deps.GetUltraPlanCoordinator()
		if coordinator == nil {
			// Log inconsistency - ultraplan mode active but no coordinator
			if logger := deps.GetLogger(); logger != nil {
				logger.Warn("ultraplan mode active but coordinator is nil", "instance_id", inst.ID)
			}
			// Fall through to regular restart
		} else {
			stepInfo := orchestrator.GetStepInfo(coordinator, inst.ID)
			if stepInfo == nil {
				// Instance doesn't match any ultraplan step - inform user and fall through
				return Result{InfoMessage: "Instance is not an ultraplan step. Using regular restart."}
			}
			newInstID, err := orchestrator.RestartStep(coordinator, stepInfo)
			if err != nil {
				return Result{ErrorMessage: fmt.Sprintf("Failed to restart %s: %v", stepInfo.Label, err)}
			}
			return Result{InfoMessage: fmt.Sprintf("%s restarted (new instance: %s)", stepInfo.Label, newInstID)}
		}
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

func cmdChain(deps Dependencies, args string) Result {
	var inst *orchestrator.Instance

	args = strings.TrimSpace(args)
	if args == "" {
		// No argument - use currently selected instance
		inst = deps.ActiveInstance()
		if inst == nil {
			return Result{ErrorMessage: "No instance selected. Select an instance first, or specify an instance number (e.g., :chain 2)."}
		}
	} else {
		// Resolve instance by sidebar number, ID, or task name
		session := deps.GetSession()
		if session == nil {
			return Result{ErrorMessage: "No session available"}
		}

		orch := deps.GetOrchestrator()
		if orch == nil {
			return Result{ErrorMessage: "No orchestrator available"}
		}

		resolved, err := orch.ResolveInstanceReference(session, args)
		if err != nil {
			return Result{ErrorMessage: fmt.Sprintf("Cannot find instance: %v", err)}
		}
		inst = resolved
	}

	addingDepTask := true
	instanceID := inst.ID
	return Result{
		AddingDependentTask:   &addingDepTask,
		DependentOnInstanceID: &instanceID,
	}
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
	return Result{
		InfoMessage: fmt.Sprintf("Removing instance %s...", instanceID),
		TeaCmd:      msg.RemoveInstanceAsync(orch, session, instanceID),
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
	// If diff is currently visible, hide it (no I/O needed)
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

	// Otherwise, start async loading
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	orch := deps.GetOrchestrator()
	if orch == nil {
		return Result{ErrorMessage: "No orchestrator available"}
	}

	return Result{
		InfoMessage: "Loading diff...",
		TeaCmd:      msg.LoadDiffAsync(orch, inst.WorktreePath, inst.ID),
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

// terminalDisabledError is the error message when terminal support is disabled.
const terminalDisabledError = "Terminal support is disabled. Enable it in :config under Experimental"

// isTerminalEnabled checks if the experimental terminal support feature is enabled.
func isTerminalEnabled() bool {
	return viper.GetBool("experimental.terminal_support")
}

func cmdTerminal(_ Dependencies) Result {
	if !isTerminalEnabled() {
		return Result{ErrorMessage: terminalDisabledError}
	}
	return Result{ToggleTerminal: true}
}

func cmdTerminalFocus(deps Dependencies) Result {
	if !isTerminalEnabled() {
		return Result{ErrorMessage: terminalDisabledError}
	}
	if deps.IsTerminalVisible() {
		return Result{
			EnterTerminalMode: true,
			InfoMessage:       "Terminal focused. Press Ctrl+] to exit.",
		}
	}
	return Result{ErrorMessage: "Terminal not visible. Use :term to open it first."}
}

func cmdTerminalDirWorktree(_ Dependencies) Result {
	if !isTerminalEnabled() {
		return Result{ErrorMessage: terminalDisabledError}
	}
	mode := 1 // TerminalDirWorktree
	return Result{TerminalDirMode: &mode}
}

func cmdTerminalDirProject(_ Dependencies) Result {
	if !isTerminalEnabled() {
		return Result{ErrorMessage: terminalDisabledError}
	}
	mode := 0 // TerminalDirProject (the directory where Claudio was started)
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

// cmdPRWithArgs handles the :pr command with optional arguments.
// Usage:
//   - :pr                  - Show PR command for active instance
//   - :pr --group          - Create PRs for all groups (stacked)
//   - :pr --group=stacked  - Create one PR per group with stacked dependencies
//   - :pr --group=single   - Create PR for current group only
//   - :pr --group=all      - Create single consolidated PR for all groups
//   - :pr --group <name>   - Create PR for specific group
func cmdPRWithArgs(deps Dependencies, args string) Result {
	args = strings.TrimSpace(args)

	// No args - show traditional PR command
	if args == "" {
		return cmdPRDefault(deps)
	}

	// Parse --group flag
	if strings.HasPrefix(args, "--group") || strings.HasPrefix(args, "-g") {
		return cmdPRGroup(deps, args)
	}

	// Unknown argument, show help
	return Result{
		ErrorMessage: fmt.Sprintf("Unknown PR argument: %s. Usage: :pr [--group[=stacked|single|all] [name]]", args),
	}
}

// cmdPRDefault shows the traditional PR creation command.
func cmdPRDefault(deps Dependencies) Result {
	inst := deps.ActiveInstance()
	if inst == nil {
		return Result{InfoMessage: "No instance selected"}
	}

	return Result{InfoMessage: fmt.Sprintf("Create PR: claudio pr %s  (add --draft for draft PR)", inst.ID)}
}

// cmdPRGroup handles group-based PR creation.
func cmdPRGroup(deps Dependencies, args string) Result {
	// Try to get group dependencies
	groupDeps, ok := deps.(GroupDependencies)
	if !ok {
		return Result{ErrorMessage: "Group PR commands not available in this context"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	// Check if there are any groups
	if len(session.Groups) == 0 {
		return Result{ErrorMessage: "No groups defined. Use :group create to create groups first."}
	}

	// Parse the argument to determine mode and target
	mode, targetGroupID, err := parsePRGroupArgs(args, session, groupDeps)
	if err != nil {
		return Result{ErrorMessage: err.Error()}
	}

	// Return result indicating we want to start a group PR workflow
	startGroupPR := true
	return Result{
		StartGroupPR:   &startGroupPR,
		GroupPRMode:    &mode,
		GroupPRGroupID: targetGroupID,
		InfoMessage:    formatGroupPRMessage(mode, targetGroupID, session),
	}
}

// parsePRGroupArgs parses the --group argument to determine mode and target.
func parsePRGroupArgs(args string, session *orchestrator.Session, deps GroupDependencies) (prworkflow.GroupPRMode, *string, error) {
	// Strip the --group or -g prefix
	var rest string
	var found bool
	if rest, found = strings.CutPrefix(args, "--group"); !found {
		rest, _ = strings.CutPrefix(args, "-g")
	}
	rest = strings.TrimSpace(rest)

	// Check for =mode syntax
	var modeStr string
	if modeStr, found = strings.CutPrefix(rest, "="); found {
		// Check for mode=value followed by optional group name
		parts := strings.SplitN(modeStr, " ", 2)
		modeStr = strings.ToLower(parts[0])

		switch modeStr {
		case "stacked":
			return prworkflow.GroupPRModeStacked, nil, nil
		case "single":
			// If a group name follows, use it; otherwise use current group
			if len(parts) > 1 {
				groupName := strings.TrimSpace(parts[1])
				grp := resolveGroup(groupName, session)
				if grp == nil {
					return 0, nil, fmt.Errorf("group not found: %s", groupName)
				}
				return prworkflow.GroupPRModeSingle, &grp.ID, nil
			}
			// Use current group (from active instance)
			inst := deps.ActiveInstance()
			if inst != nil {
				grp := session.GetGroupForInstance(inst.ID)
				if grp != nil {
					return prworkflow.GroupPRModeSingle, &grp.ID, nil
				}
			}
			return 0, nil, fmt.Errorf("no group selected, select an instance in a group or specify a group name")
		case "all", "consolidated":
			return prworkflow.GroupPRModeConsolidated, nil, nil
		default:
			return 0, nil, fmt.Errorf("unknown PR mode: %s (use stacked, single, or all)", modeStr)
		}
	}

	// No = syntax, check for group name after space
	if rest != "" {
		// Treat rest as a group name for single mode
		grp := resolveGroup(rest, session)
		if grp == nil {
			return 0, nil, fmt.Errorf("group not found: %s", rest)
		}
		return prworkflow.GroupPRModeSingle, &grp.ID, nil
	}

	// Default: stacked mode for all groups
	return prworkflow.GroupPRModeStacked, nil, nil
}

// formatGroupPRMessage creates an info message describing the group PR operation.
func formatGroupPRMessage(mode prworkflow.GroupPRMode, targetGroupID *string, session *orchestrator.Session) string {
	switch mode {
	case prworkflow.GroupPRModeStacked:
		return fmt.Sprintf("Creating stacked PRs for %d groups...", len(session.Groups))
	case prworkflow.GroupPRModeConsolidated:
		return fmt.Sprintf("Creating consolidated PR from %d groups...", len(session.Groups))
	case prworkflow.GroupPRModeSingle:
		if targetGroupID != nil {
			grp := session.GetGroup(*targetGroupID)
			if grp != nil {
				return fmt.Sprintf("Creating PR for group: %s", grp.Name)
			}
		}
		return "Creating PR for selected group..."
	default:
		return "Creating group PR..."
	}
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

func cmdTripleShot(deps Dependencies) Result {
	// Don't allow starting triple-shot if in ultraplan mode
	if deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Cannot start triple-shot while in ultraplan mode"}
	}

	// Multiple tripleshots are now allowed - no check for IsTripleShotMode()

	// Signal to the model that we want to enter triple-shot mode
	// The model will handle prompting for the task
	startTripleShot := true
	infoMsg := "Enter a task for triple-shot mode"
	if deps.IsTripleShotMode() {
		infoMsg = "Enter a task for additional triple-shot"
	}
	return Result{
		StartTripleShot: &startTripleShot,
		InfoMessage:     infoMsg,
	}
}

func cmdAdversarial(deps Dependencies) Result {
	// Don't allow starting adversarial if in ultraplan mode
	if deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Cannot start adversarial mode while in ultraplan mode"}
	}

	// Multiple adversarial sessions are allowed - no check for IsAdversarialMode()

	// Signal to the model that we want to enter adversarial mode
	// The model will handle prompting for the task
	startAdversarial := true
	infoMsg := "Enter a task for adversarial mode (implementer + reviewer)"
	if deps.IsAdversarialMode() {
		infoMsg = "Enter a task for additional adversarial session"
	}
	return Result{
		StartAdversarial: &startAdversarial,
		InfoMessage:      infoMsg,
	}
}

func cmdPlan(deps Dependencies) Result {
	// Don't allow starting plan mode if already in ultraplan mode
	if deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Cannot start plan mode while in ultraplan mode"}
	}

	// Plan mode is allowed in triple-shot mode - plans appear as separate groups in the sidebar

	// Signal to the model that we want to enter plan mode
	// The model will handle prompting for the objective if not provided
	startPlanMode := true
	return Result{
		StartPlanMode: &startPlanMode,
		InfoMessage:   "Enter an objective for plan mode",
	}
}

func cmdMultiPlan(deps Dependencies) Result {
	// Check if inline plan is enabled in config (multiplan remains experimental)
	if !viper.GetBool("experimental.inline_plan") {
		return Result{ErrorMessage: "MultiPlan mode is disabled. Enable it in :config under Experimental"}
	}

	// Don't allow starting multiplan mode if already in ultraplan mode
	if deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Cannot start multiplan mode while in ultraplan mode"}
	}

	// Multiplan mode is allowed in triple-shot mode - plans appear as separate groups in the sidebar

	// Signal to the model that we want to enter multi-pass plan mode
	// This will create 3 parallel planners + 1 plan assessor
	startMultiPlanMode := true
	return Result{
		StartMultiPlanMode: &startMultiPlanMode,
		InfoMessage:        "Enter an objective for multiplan mode (3 planners + 1 assessor)",
	}
}

// cmdUltraPlan handles the :ultraplan command with arguments.
// Usage:
//   - :ultraplan [objective]           - Start ultraplan with objective
//   - :ultraplan --multi-pass [obj]    - Use multi-pass planning
//   - :ultraplan --plan <file>         - Load existing plan file
func cmdUltraPlan(deps Dependencies, args string) Result {
	// Check if inline ultraplan is enabled in config
	if !viper.GetBool("experimental.inline_ultraplan") {
		return Result{ErrorMessage: "UltraPlan mode is disabled. Enable it in :config under Experimental"}
	}

	// Don't allow starting another ultraplan if already in ultraplan mode
	if deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Already in ultraplan mode"}
	}
	// Don't allow ultraplan in triple-shot mode - ultraplan has its own dedicated UI
	// Use :plan instead for simpler planning within triple-shot
	if deps.IsTripleShotMode() {
		return Result{ErrorMessage: "Cannot start ultraplan while in triple-shot mode. Use :plan instead"}
	}

	// Parse arguments
	args = strings.TrimSpace(args)
	multiPass := false
	planFile := ""
	objective := ""

	// Parse flags
	if rest, found := strings.CutPrefix(args, "--multi-pass"); found {
		multiPass = true
		objective = strings.TrimSpace(rest)
	} else if rest, found := strings.CutPrefix(args, "--plan"); found {
		args = strings.TrimSpace(rest)
		// The next word should be the file path
		parts := strings.SplitN(args, " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			return Result{ErrorMessage: "Usage: :ultraplan --plan <file>"}
		}
		planFile = parts[0]
		// Any remaining args are ignored for --plan mode
	} else {
		// No flags, treat entire args as objective
		objective = args
	}

	// Signal to the model that we want to enter ultraplan mode
	startUltraPlan := true
	result := Result{
		StartUltraPlanMode: &startUltraPlan,
	}

	if multiPass {
		mp := true
		result.UltraPlanMultiPass = &mp
	}

	if planFile != "" {
		result.UltraPlanFromFile = &planFile
	}

	if objective != "" {
		result.UltraPlanObjective = &objective
		result.InfoMessage = fmt.Sprintf("Starting ultraplan: %s", objective)
	} else if planFile != "" {
		result.InfoMessage = fmt.Sprintf("Loading ultraplan from: %s", planFile)
	} else {
		result.InfoMessage = "Enter an objective for ultraplan mode"
	}

	return result
}

// cmdRalph handles the :ralph command with arguments.
// Usage:
//   - :ralph "<prompt>" --max-iterations N --completion-promise "TEXT"
//   - :ralph-loop "<prompt>" --completion-promise "DONE"
func cmdRalph(deps Dependencies, args string) Result {
	// Don't allow starting ralph if in ultraplan mode
	if deps.IsUltraPlanMode() {
		return Result{ErrorMessage: "Cannot start ralph loop while in ultraplan mode"}
	}

	// Parse arguments
	args = strings.TrimSpace(args)
	if args == "" {
		// No args - prompt for task input
		startRalph := true
		return Result{
			StartRalphLoop: &startRalph,
			InfoMessage:    "Enter a prompt for ralph loop",
		}
	}

	// Parse flags and prompt
	prompt, maxIterations, completionPromise, err := parseRalphArgs(args)
	if err != nil {
		return Result{ErrorMessage: err.Error()}
	}

	// Require completion promise
	if completionPromise == "" {
		return Result{ErrorMessage: "Ralph loop requires --completion-promise to know when to stop"}
	}

	// Signal to start ralph loop
	startRalph := true
	result := Result{
		StartRalphLoop:         &startRalph,
		RalphPrompt:            &prompt,
		RalphCompletionPromise: &completionPromise,
	}

	if maxIterations > 0 {
		result.RalphMaxIterations = &maxIterations
	}

	return result
}

// parseRalphArgs parses the arguments for the ralph command.
// Returns: prompt, maxIterations, completionPromise, error
func parseRalphArgs(args string) (string, int, string, error) {
	var prompt string
	var maxIterations int
	var completionPromise string

	// Check for flags
	remaining := args

	// Parse --max-iterations
	if idx := strings.Index(remaining, "--max-iterations"); idx >= 0 {
		beforeFlag := strings.TrimSpace(remaining[:idx])
		afterFlag := strings.TrimPrefix(remaining[idx:], "--max-iterations")
		afterFlag = strings.TrimSpace(afterFlag)

		// Extract the number
		parts := strings.SplitN(afterFlag, " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			return "", 0, "", fmt.Errorf("--max-iterations requires a number")
		}
		n, err := parseNumber(parts[0])
		if err != nil {
			return "", 0, "", fmt.Errorf("invalid --max-iterations value: %w", err)
		}
		maxIterations = n

		// Rebuild remaining
		if len(parts) > 1 {
			remaining = beforeFlag + " " + strings.TrimSpace(parts[1])
		} else {
			remaining = beforeFlag
		}
	}

	// Parse --completion-promise
	if idx := strings.Index(remaining, "--completion-promise"); idx >= 0 {
		beforeFlag := strings.TrimSpace(remaining[:idx])
		afterFlag := strings.TrimPrefix(remaining[idx:], "--completion-promise")
		afterFlag = strings.TrimSpace(afterFlag)

		// Extract the promise (handle quoted strings)
		promise, rest, err := parseQuotedOrWord(afterFlag)
		if err != nil {
			return "", 0, "", fmt.Errorf("invalid --completion-promise: %w", err)
		}
		completionPromise = promise
		remaining = beforeFlag + " " + rest
	}

	// The remaining text is the prompt (remove surrounding quotes if present)
	prompt = strings.TrimSpace(remaining)
	if (strings.HasPrefix(prompt, "\"") && strings.HasSuffix(prompt, "\"")) ||
		(strings.HasPrefix(prompt, "'") && strings.HasSuffix(prompt, "'")) {
		prompt = prompt[1 : len(prompt)-1]
	}

	return prompt, maxIterations, completionPromise, nil
}

// parseNumber parses an integer from a string.
func parseNumber(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// parseQuotedOrWord parses a quoted string or single word from the input.
// Returns the value, the remaining string, and any error.
func parseQuotedOrWord(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", fmt.Errorf("expected value")
	}

	// Check for quoted string
	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		endIdx := strings.IndexByte(s[1:], quote)
		if endIdx == -1 {
			return "", "", fmt.Errorf("unclosed quote")
		}
		value := s[1 : endIdx+1]
		rest := strings.TrimSpace(s[endIdx+2:])
		return value, rest, nil
	}

	// Single word
	parts := strings.SplitN(s, " ", 2)
	if len(parts) == 1 {
		return parts[0], "", nil
	}
	return parts[0], strings.TrimSpace(parts[1]), nil
}

// cmdCancelRalph handles the :cancel-ralph command.
func cmdCancelRalph(deps Dependencies) Result {
	if !deps.IsRalphMode() {
		return Result{ErrorMessage: "Not in ralph loop mode"}
	}

	cancelRalph := true
	return Result{
		CancelRalphLoop: &cancelRalph,
		InfoMessage:     "Ralph loop cancelled",
	}
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

func cmdQuitForce(deps Dependencies) Result {
	orch := deps.GetOrchestrator()
	session := deps.GetSession()

	// Log session force quit with duration
	if logger := deps.GetLogger(); logger != nil {
		duration := time.Since(deps.GetStartTime())
		logger.Info("TUI session force quit initiated",
			"duration_ms", duration.Milliseconds(),
			"instance_count", deps.InstanceCount(),
		)
	}

	quitting := true

	// Handle case where orchestrator or session is unavailable
	if orch == nil || session == nil {
		if logger := deps.GetLogger(); logger != nil {
			logger.Warn("force quit with nil orchestrator or session - cleanup skipped",
				"orch_nil", orch == nil,
				"session_nil", session == nil,
			)
		}
		return Result{
			Quitting:    &quitting,
			TeaCmd:      tea.Quit,
			InfoMessage: "Force quit: exiting (no active session to clean up)",
		}
	}

	// Force stop all instances and clean up worktrees
	var cleanupErr error
	if err := orch.StopSession(session, true); err != nil {
		cleanupErr = err
		if logger := deps.GetLogger(); logger != nil {
			logger.Warn("error during force quit cleanup", "error", err)
		}
	}

	infoMsg := "Force quit: stopped all instances and cleaned up worktrees"
	if cleanupErr != nil {
		infoMsg = "Force quit: exiting (cleanup had errors - some worktrees may remain)"
	}

	return Result{
		Quitting:    &quitting,
		TeaCmd:      tea.Quit,
		InfoMessage: infoMsg,
	}
}

// Ensure Handler doesn't unexpectedly reference instance package types
// that would break if imported elsewhere
var _ = instance.TimeoutActivity // compile-time check that instance is available

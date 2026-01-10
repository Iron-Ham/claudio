// Package commands provides the command pattern infrastructure for the TUI.
// It defines interfaces and types for implementing vim-style commands that
// can be registered, discovered, and executed through a dispatcher.
package commands

import (
	"fmt"
	"io"
)

// Command represents an executable TUI command.
// Commands are registered with a dispatcher and invoked by name.
type Command interface {
	// Name returns the primary name of the command (e.g., "quit", "help").
	Name() string

	// Execute runs the command with the given context.
	// It returns a result containing any outputs or errors.
	Execute(ctx *CommandContext) (CommandResult, error)

	// Description returns a human-readable description of what the command does.
	Description() string
}

// AliasedCommand extends Command with support for aliases.
// Commands implementing this interface can be invoked by multiple names.
type AliasedCommand interface {
	Command
	// Aliases returns alternative names for the command (e.g., "q" for "quit").
	Aliases() []string
}

// ModelAccessor provides read-only access to TUI model state.
// This interface decouples commands from the concrete Model type,
// allowing commands to be tested and evolved independently.
type ModelAccessor interface {
	// IsUltraPlanMode returns true if in ultra-plan coordination mode.
	IsUltraPlanMode() bool

	// IsPlanEditorActive returns true if the plan editor is currently active.
	IsPlanEditorActive() bool

	// InstanceCount returns the number of Claude instances in the session.
	InstanceCount() int

	// ActiveInstanceID returns the ID of the currently selected instance,
	// or empty string if no instance is selected.
	ActiveInstanceID() string

	// HasConflicts returns true if there are unresolved file conflicts.
	HasConflicts() bool
}

// ModelMutator provides methods to modify TUI model state.
// Commands use this interface to effect changes in the TUI.
type ModelMutator interface {
	// SetErrorMessage displays an error message to the user.
	SetErrorMessage(msg string)

	// SetInfoMessage displays an informational message to the user.
	SetInfoMessage(msg string)

	// ClearMessages clears both error and info messages.
	ClearMessages()

	// SetShowHelp toggles the help panel visibility.
	SetShowHelp(show bool)

	// SetQuitting initiates application shutdown.
	SetQuitting(quitting bool)

	// SetAddingTask enters or exits task input mode.
	SetAddingTask(adding bool)

	// SetShowStats toggles the stats/metrics panel visibility.
	SetShowStats(show bool)

	// SetShowDiff toggles the diff panel visibility.
	SetShowDiff(show bool)

	// SetShowConflicts toggles the conflicts view visibility.
	SetShowConflicts(show bool)

	// SetFilterMode enters or exits output filter mode.
	SetFilterMode(mode bool)
}

// CommandContext provides the execution context for commands.
// It contains all dependencies and state a command needs to execute.
type CommandContext struct {
	// Model provides access to the TUI model state.
	// Use Accessor for reading state and Mutator for modifying it.
	Accessor ModelAccessor
	Mutator  ModelMutator

	// Args contains any arguments passed to the command.
	// For example, ":add my task" would have Args = ["my", "task"].
	Args []string

	// Output is an optional writer for command output.
	// Commands can write additional feedback here (e.g., for verbose modes).
	// May be nil if no output capture is desired.
	Output io.Writer

	// Orchestrator provides access to instance management operations.
	// This is an opaque interface that commands can type-assert to
	// access orchestrator functionality.
	Orchestrator any
}

// CommandResult represents the outcome of command execution.
type CommandResult struct {
	// Message is an optional message to display after execution.
	// This is typically shown as an info message.
	Message string

	// ShouldQuit indicates the application should exit.
	ShouldQuit bool

	// RequiresRedraw indicates the UI needs to be refreshed.
	RequiresRedraw bool

	// Data contains any command-specific result data.
	// Commands can use this to pass structured data back to callers.
	Data any
}

// CommandDispatcher manages command registration and execution.
// It provides the central registry for all available commands.
type CommandDispatcher interface {
	// Register adds a command to the dispatcher.
	// If a command with the same name already exists, it is replaced.
	Register(cmd Command)

	// Execute runs a command by name with the given context.
	// Returns UnknownCommandError if the command is not found.
	Execute(name string, args []string, ctx *CommandContext) (CommandResult, error)

	// List returns all registered commands.
	// The returned slice is safe to iterate; modifying it does not affect the dispatcher.
	List() []Command

	// Get retrieves a command by name or alias.
	// Returns the command and true if found, nil and false otherwise.
	Get(name string) (Command, bool)
}

// Error types for command execution

// UnknownCommandError is returned when attempting to execute an unregistered command.
type UnknownCommandError struct {
	// Name is the command name that was not found.
	Name string
}

func (e *UnknownCommandError) Error() string {
	return fmt.Sprintf("unknown command: %s (type :h for help)", e.Name)
}

// InvalidArgsError is returned when a command receives invalid arguments.
type InvalidArgsError struct {
	// Command is the name of the command that received invalid arguments.
	Command string

	// Message describes what was wrong with the arguments.
	Message string

	// Expected describes the expected argument format.
	Expected string
}

func (e *InvalidArgsError) Error() string {
	if e.Expected != "" {
		return fmt.Sprintf("%s: %s (expected: %s)", e.Command, e.Message, e.Expected)
	}
	return fmt.Sprintf("%s: %s", e.Command, e.Message)
}

// CommandNotApplicableError is returned when a command cannot be executed
// in the current context (e.g., instance command when no instance is selected).
type CommandNotApplicableError struct {
	// Command is the name of the command that couldn't be executed.
	Command string

	// Reason explains why the command cannot be executed.
	Reason string
}

func (e *CommandNotApplicableError) Error() string {
	return fmt.Sprintf("%s: %s", e.Command, e.Reason)
}

// CommandCategory groups related commands for help display and organization.
type CommandCategory string

const (
	// CategoryInstanceControl contains commands for controlling instance state (start, stop, pause).
	CategoryInstanceControl CommandCategory = "Instance Control"

	// CategoryInstanceManagement contains commands for managing instances (add, remove, kill).
	CategoryInstanceManagement CommandCategory = "Instance Management"

	// CategoryView contains commands for toggling UI views (diff, stats, conflicts).
	CategoryView CommandCategory = "View"

	// CategoryUtility contains utility commands (tmux, pr).
	CategoryUtility CommandCategory = "Utility"

	// CategoryHelp contains help and quit commands.
	CategoryHelp CommandCategory = "Help"
)

// CategorizedCommand extends Command with category information for organization.
type CategorizedCommand interface {
	Command
	// Category returns the command's category for grouping in help displays.
	Category() CommandCategory
}

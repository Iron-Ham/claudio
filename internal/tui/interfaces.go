// Package tui provides the terminal user interface for Claudio.
//
// This file defines core interfaces that enable testable, composable TUI components.
// These interfaces follow Go's interface segregation principle - small, focused
// interfaces that can be composed as needed rather than one large interface.
package tui

import tea "github.com/charmbracelet/bubbletea"

// Dimensions represents the width and height of a UI region.
type Dimensions struct {
	Width  int
	Height int
}

// Component defines the core interface for TUI components.
//
// This interface is inspired by but not identical to tea.Model.
// It adds explicit size management which is essential for composing
// nested components. Components implementing this interface can be
// tested independently of the Bubbletea runtime.
//
// Example usage:
//
//	type MyComponent struct { ... }
//	func (c *MyComponent) Init() tea.Cmd { return nil }
//	func (c *MyComponent) Update(msg tea.Msg) (Component, tea.Cmd) { ... }
//	func (c *MyComponent) View() string { return "..." }
//	func (c *MyComponent) SetSize(width, height int) { ... }
type Component interface {
	// Init initializes the component and returns any initial commands.
	// This is called once when the component is first created.
	Init() tea.Cmd

	// Update handles incoming messages and returns the updated component
	// along with any commands to execute. The returned Component may be
	// the same instance (mutated) or a new instance.
	Update(msg tea.Msg) (Component, tea.Cmd)

	// View renders the component to a string for display.
	// The output should respect the dimensions set via SetSize.
	View() string

	// SetSize updates the component's available dimensions.
	// Components should adjust their rendering accordingly.
	SetSize(width, height int)
}

// EventHandler processes user input events.
//
// This interface separates event handling concerns from the main component
// logic, allowing event handlers to be tested and composed independently.
// Different modes (normal, command, search) can implement this interface
// with their own handling logic.
type EventHandler interface {
	// HandleKey processes keyboard input and returns any resulting command.
	// Returns true if the event was handled (consumed), false to propagate.
	HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd)

	// HandleMouse processes mouse input and returns any resulting command.
	// Returns true if the event was handled (consumed), false to propagate.
	HandleMouse(msg tea.MouseMsg) (handled bool, cmd tea.Cmd)

	// HandleResize processes terminal resize events.
	HandleResize(width, height int)
}

// FocusableEventHandler extends EventHandler with focus management.
//
// Components that can receive and lose focus should implement this interface.
// Focus state affects how events are routed - focused components receive
// events before unfocused ones in a typical event bubbling model.
type FocusableEventHandler interface {
	EventHandler

	// Focus is called when the handler gains focus.
	Focus()

	// Blur is called when the handler loses focus.
	Blur()

	// Focused returns whether the handler currently has focus.
	Focused() bool
}

// Renderer provides view rendering capabilities.
//
// This interface abstracts the rendering logic, making it possible to
// test view output independently and to compose multiple renderers
// for different regions of the UI.
type Renderer interface {
	// Render produces the visual output as a string.
	// The output should respect the current dimensions.
	Render() string

	// SetStyles configures the visual styling for the renderer.
	// The styles parameter is any to allow flexibility in
	// style configuration (could be lipgloss styles, theme config, etc).
	SetStyles(styles any)

	// GetDimensions returns the current rendering dimensions.
	GetDimensions() Dimensions
}

// ContentRenderer extends Renderer with content-specific methods.
//
// This is useful for renderers that display scrollable content
// like output panels, log views, or help screens.
type ContentRenderer interface {
	Renderer

	// SetContent updates the content to be rendered.
	SetContent(content string)

	// GetContentHeight returns the total height of the content in lines.
	// This is useful for implementing scroll indicators.
	GetContentHeight() int
}

// StateProvider enables read access to component state.
//
// This interface supports a reactive pattern where components can
// subscribe to state changes. It's particularly useful for:
// - Cross-component communication without tight coupling
// - Testing state transitions
// - Debugging and logging state changes
type StateProvider[T any] interface {
	// GetState returns the current state.
	GetState() T

	// Subscribe registers a callback to be notified of state changes.
	// Returns an ID that can be used to unsubscribe.
	Subscribe(callback func(T)) (subscriptionID string)

	// Unsubscribe removes a previously registered callback.
	Unsubscribe(subscriptionID string)
}

// StateNotifier provides state change notifications.
//
// This is the write-side complement to StateProvider.
// Components that own state implement this to notify subscribers.
type StateNotifier[T any] interface {
	// NotifyStateChange triggers all registered callbacks with the new state.
	NotifyStateChange(newState T)
}

// StatefulComponent combines StateProvider and StateNotifier for components
// that both hold state and need to notify others of changes.
type StatefulComponent[T any] interface {
	StateProvider[T]
	StateNotifier[T]
}

// CommandExecutor handles execution of user commands.
//
// This interface abstracts command execution, enabling:
// - Testing command behavior in isolation
// - Command history and undo functionality
// - Permission checking before execution
// - Consistent help text generation
//
// Commands are typically entered via the ':' command mode (vim-style).
type CommandExecutor interface {
	// Execute runs a command by name with the given arguments.
	// Returns any tea.Cmd to execute and an error if the command failed.
	Execute(name string, args []string) (tea.Cmd, error)

	// CanExecute returns true if the command can be executed in the current state.
	// This allows UI to show disabled states for unavailable commands.
	CanExecute(name string) bool

	// GetHelp returns help text for a specific command, or all commands if name is empty.
	GetHelp(name string) string
}

// CommandInfo describes a single command's metadata.
type CommandInfo struct {
	// Name is the primary command name (e.g., "start", "stop").
	Name string

	// Aliases are alternative names for the command (e.g., "s" for "start").
	Aliases []string

	// Description is a brief description for help text.
	Description string

	// Usage shows the command syntax (e.g., "start [instance-id]").
	Usage string
}

// CommandRegistry provides access to available commands.
//
// This interface is separate from CommandExecutor to allow components
// to discover available commands without needing execution capability.
type CommandRegistry interface {
	// ListCommands returns all registered commands.
	ListCommands() []CommandInfo

	// FindCommand looks up a command by name or alias.
	// Returns nil if no command matches.
	FindCommand(nameOrAlias string) *CommandInfo
}

// Scrollable defines behavior for components with scrollable content.
//
// Many TUI components need scroll functionality - output panels, help screens,
// task lists, etc. This interface provides a consistent API for scroll control.
type Scrollable interface {
	// ScrollUp moves the viewport up by n lines.
	ScrollUp(n int)

	// ScrollDown moves the viewport down by n lines.
	ScrollDown(n int)

	// ScrollToTop moves to the beginning of the content.
	ScrollToTop()

	// ScrollToBottom moves to the end of the content.
	ScrollToBottom()

	// GetScrollOffset returns the current scroll position (0 = top).
	GetScrollOffset() int

	// GetMaxScroll returns the maximum valid scroll offset.
	GetMaxScroll() int

	// SetAutoScroll enables or disables auto-scrolling to new content.
	SetAutoScroll(enabled bool)

	// IsAutoScroll returns whether auto-scrolling is enabled.
	IsAutoScroll() bool
}

// Searchable defines behavior for components that support text search.
//
// This interface enables consistent search functionality across different
// content types (output, help text, task lists, etc.).
type Searchable interface {
	// Search initiates a search with the given pattern.
	// Returns the number of matches found.
	Search(pattern string) int

	// NextMatch moves to the next search match.
	// Returns false if there are no more matches.
	NextMatch() bool

	// PrevMatch moves to the previous search match.
	// Returns false if there are no previous matches.
	PrevMatch() bool

	// ClearSearch clears the current search state.
	ClearSearch()

	// GetMatchCount returns the total number of matches.
	GetMatchCount() int

	// GetCurrentMatch returns the current match index (0-based).
	// Returns -1 if no matches or search not active.
	GetCurrentMatch() int
}

// Filterable defines behavior for components that support content filtering.
//
// Filtering differs from searching in that it hides non-matching content
// rather than just highlighting matches.
type Filterable interface {
	// SetFilter applies a filter pattern to the content.
	SetFilter(pattern string)

	// SetCategories enables or disables predefined filter categories.
	// Category names are component-specific (e.g., "errors", "warnings").
	SetCategories(enabled map[string]bool)

	// GetCategories returns the available filter categories and their state.
	GetCategories() map[string]bool

	// ClearFilter removes any active filters.
	ClearFilter()
}

// LifecycleAware defines hooks for component lifecycle events.
//
// Components that need setup/cleanup operations should implement this
// interface. This is particularly useful for components that manage
// external resources (goroutines, file handles, network connections).
type LifecycleAware interface {
	// OnMount is called when the component is added to the UI tree.
	OnMount()

	// OnUnmount is called when the component is removed from the UI tree.
	// Implementations should clean up any resources.
	OnUnmount()
}

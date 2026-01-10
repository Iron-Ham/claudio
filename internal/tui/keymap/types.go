// Package keymap provides key binding definitions and lookup for the TUI.
// It extracts key binding logic from app.go's Update method into a declarative,
// mode-aware configuration system.
package keymap

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// Mode represents the current input mode of the TUI.
// Different modes have different key bindings active.
type Mode string

const (
	ModeNormal     Mode = "normal"      // Default viewing mode
	ModeSearch     Mode = "search"      // Typing search pattern (after /)
	ModeFilter     Mode = "filter"      // Filtering output categories (after :f)
	ModeCommand    Mode = "command"     // Vim-style ex commands (after :)
	ModeInput      Mode = "input"       // Keys forwarded to tmux session
	ModeAddTask    Mode = "add_task"    // Entering new task description
	ModeTemplate   Mode = "template"    // Selecting from task templates
	ModeUltraPlan  Mode = "ultra_plan"  // Ultra-plan orchestration mode
	ModePlanEditor Mode = "plan_editor" // Interactive plan editing mode
)

// Command represents a named action that can be triggered by a key binding.
type Command string

// Normal mode commands
const (
	// Navigation
	CmdNextInstance     Command = "next_instance"
	CmdPrevInstance     Command = "prev_instance"
	CmdJumpToInstance   Command = "jump_to_instance" // 1-9 keys
	CmdEnterInputMode   Command = "enter_input_mode"
	CmdScrollDown       Command = "scroll_down"
	CmdScrollUp         Command = "scroll_up"
	CmdScrollHalfPageUp Command = "scroll_half_page_up"
	CmdScrollHalfPageDn Command = "scroll_half_page_down"
	CmdScrollPageUp     Command = "scroll_page_up"
	CmdScrollPageDown   Command = "scroll_page_down"
	CmdScrollToTop      Command = "scroll_to_top"
	CmdScrollToBottom   Command = "scroll_to_bottom"

	// Mode entry
	CmdEnterCommandMode Command = "enter_command_mode"
	CmdEnterSearchMode  Command = "enter_search_mode"
	CmdToggleHelp       Command = "toggle_help"

	// Search
	CmdNextSearchMatch  Command = "next_search_match"
	CmdPrevSearchMatch  Command = "prev_search_match"
	CmdClearSearch      Command = "clear_search"

	// Instance control
	CmdRestartInstance Command = "restart_instance"
	CmdKillInstance    Command = "kill_instance"

	// View toggles
	CmdCloseDiffPanel Command = "close_diff_panel"

	// Exit
	CmdQuit Command = "quit"
)

// Command mode commands (ex commands after :)
const (
	CmdExStart       Command = "ex_start"       // :s, :start
	CmdExStop        Command = "ex_stop"        // :x, :stop
	CmdExPause       Command = "ex_pause"       // :p, :pause
	CmdExReconnect   Command = "ex_reconnect"   // :R, :reconnect
	CmdExRestart     Command = "ex_restart"     // :restart
	CmdExAdd         Command = "ex_add"         // :a, :add
	CmdExRemove      Command = "ex_remove"      // :D, :remove
	CmdExKill        Command = "ex_kill"        // :kill
	CmdExClear       Command = "ex_clear"       // :C, :clear
	CmdExDiff        Command = "ex_diff"        // :d, :diff
	CmdExMetrics     Command = "ex_metrics"     // :m, :metrics, :stats
	CmdExConflicts   Command = "ex_conflicts"   // :c, :conflicts
	CmdExFilter      Command = "ex_filter"      // :f, :F, :filter
	CmdExTmux        Command = "ex_tmux"        // :t, :tmux
	CmdExPR          Command = "ex_pr"          // :r, :pr
	CmdExHelp        Command = "ex_help"        // :h, :help
	CmdExQuit        Command = "ex_quit"        // :q, :quit
)

// Input mode commands (text editing)
const (
	CmdCancel              Command = "cancel"
	CmdConfirm             Command = "confirm"
	CmdDeleteBack          Command = "delete_back"
	CmdDeleteForward       Command = "delete_forward"
	CmdMoveCursorLeft      Command = "move_cursor_left"
	CmdMoveCursorRight     Command = "move_cursor_right"
	CmdMoveToLineStart     Command = "move_to_line_start"
	CmdMoveToLineEnd       Command = "move_to_line_end"
	CmdDeleteToLineStart   Command = "delete_to_line_start"
	CmdDeleteToLineEnd     Command = "delete_to_line_end"
	CmdInsertNewline       Command = "insert_newline"
	CmdPrevWordBoundary    Command = "prev_word_boundary"
	CmdNextWordBoundary    Command = "next_word_boundary"
	CmdDeletePrevWord      Command = "delete_prev_word"
	CmdMoveToInputStart    Command = "move_to_input_start"
	CmdMoveToInputEnd      Command = "move_to_input_end"
	CmdShowTemplates       Command = "show_templates"
	CmdInsertChar          Command = "insert_char"
	CmdInsertSpace         Command = "insert_space"
)

// Filter mode commands
const (
	CmdToggleErrors   Command = "toggle_errors"
	CmdToggleWarnings Command = "toggle_warnings"
	CmdToggleTools    Command = "toggle_tools"
	CmdToggleThinking Command = "toggle_thinking"
	CmdToggleProgress Command = "toggle_progress"
	CmdToggleAll      Command = "toggle_all"
	CmdClearFilter    Command = "clear_custom_filter"
	CmdExitFilter     Command = "exit_filter"
)

// Search mode commands
const (
	CmdExecuteSearch Command = "execute_search"
	CmdCancelSearch  Command = "cancel_search"
)

// Template dropdown commands
const (
	CmdCloseDropdown    Command = "close_dropdown"
	CmdSelectTemplate   Command = "select_template"
	CmdPrevTemplate     Command = "prev_template"
	CmdNextTemplate     Command = "next_template"
	CmdFilterTemplates  Command = "filter_templates"
	CmdCloseAddSpace    Command = "close_add_space"
)

// Ultra-plan mode commands
const (
	CmdUPContinue        Command = "up_continue"         // Continue with partial work
	CmdUPRetry           Command = "up_retry"            // Retry failed tasks
	CmdUPCancel          Command = "up_cancel"           // Cancel ultraplan
	CmdUPTogglePlanView  Command = "up_toggle_plan_view" // Toggle plan view
	CmdUPParsePlan       Command = "up_parse_plan"       // Parse plan from file
	CmdUPStartExecution  Command = "up_start_execution"  // Start execution
	CmdUPEnterPlanEditor Command = "up_enter_plan_editor"
	CmdUPCancelExecution Command = "up_cancel_execution"
	CmdUPSignalSynthesis Command = "up_signal_synthesis"
	CmdUPResumeConsol    Command = "up_resume_consolidation"
	CmdUPOpenPR          Command = "up_open_pr"
)

// Plan editor mode commands
const (
	CmdPEExit             Command = "pe_exit"
	CmdPENextTask         Command = "pe_next_task"
	CmdPEPrevTask         Command = "pe_prev_task"
	CmdPEFirstTask        Command = "pe_first_task"
	CmdPELastTask         Command = "pe_last_task"
	CmdPECycleForward     Command = "pe_cycle_forward"
	CmdPECycleBackward    Command = "pe_cycle_backward"
	CmdPEToggleValidation Command = "pe_toggle_validation"
	CmdPERefreshValid     Command = "pe_refresh_validation"
	CmdPEScrollValidUp    Command = "pe_scroll_valid_up"
	CmdPEScrollValidDown  Command = "pe_scroll_valid_down"
	CmdPEEditTitle        Command = "pe_edit_title"
	CmdPEEditDescription  Command = "pe_edit_description"
	CmdPEEditFiles        Command = "pe_edit_files"
	CmdPEEditPriority     Command = "pe_edit_priority"
	CmdPECycleComplexity  Command = "pe_cycle_complexity"
	CmdPEEditDependencies Command = "pe_edit_dependencies"
	CmdPEDeleteTask       Command = "pe_delete_task"
	CmdPEAddTask          Command = "pe_add_task"
	CmdPEMoveTaskDown     Command = "pe_move_task_down"
	CmdPEMoveTaskUp       Command = "pe_move_task_up"
	CmdPESavePlan         Command = "pe_save_plan"
	CmdPEConfirmExecute   Command = "pe_confirm_execute"
)

// Tmux forwarding commands
const (
	CmdExitInputMode Command = "exit_input_mode"
	CmdForwardToTmux Command = "forward_to_tmux"
)

// Modifier represents keyboard modifiers (Ctrl, Alt, Shift).
type Modifier uint8

const (
	ModNone  Modifier = 0
	ModCtrl  Modifier = 1 << iota
	ModAlt
	ModShift
)

// String returns a human-readable representation of modifiers.
func (m Modifier) String() string {
	if m == ModNone {
		return ""
	}
	var s string
	if m&ModCtrl != 0 {
		s += "ctrl+"
	}
	if m&ModAlt != 0 {
		s += "alt+"
	}
	if m&ModShift != 0 {
		s += "shift+"
	}
	return s
}

// KeyBinding represents a single key binding configuration.
type KeyBinding struct {
	// Key is the primary key for this binding.
	// For special keys, use tea.KeyType constants (e.g., tea.KeyEnter).
	// For rune keys, use tea.KeyRunes and set Rune field.
	KeyType tea.KeyType

	// Rune is the character for rune-based keys (when KeyType is tea.KeyRunes).
	Rune rune

	// Modifiers contains the modifier keys that must be pressed.
	Modifiers Modifier

	// Command is the action to execute when this binding is triggered.
	Command Command

	// Description is a human-readable description for help display.
	Description string

	// Category groups related bindings together in help display.
	Category string
}

// Matches checks if a tea.KeyMsg matches this binding.
func (kb KeyBinding) Matches(msg tea.KeyMsg) bool {
	// Check modifiers
	wantAlt := kb.Modifiers&ModAlt != 0
	if msg.Alt != wantAlt {
		return false
	}

	// For special keys (not runes), match the key type directly
	if kb.KeyType != tea.KeyRunes {
		return msg.Type == kb.KeyType
	}

	// For rune keys, check the rune value
	if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 {
		return false
	}

	// If Rune is 0, this is a catch-all binding for any rune
	if kb.Rune == 0 {
		return true
	}

	return msg.Runes[0] == kb.Rune
}

// String returns a human-readable representation of the key binding.
func (kb KeyBinding) String() string {
	prefix := kb.Modifiers.String()

	if kb.KeyType != tea.KeyRunes {
		return prefix + kb.KeyType.String()
	}

	// Handle special display cases
	switch kb.Rune {
	case ' ':
		return prefix + "space"
	default:
		return prefix + string(kb.Rune)
	}
}

// ModeBindings holds all key bindings for a specific mode.
type ModeBindings struct {
	Mode     Mode
	Bindings []KeyBinding
}

// GetBinding looks up a command for a key in this mode.
// Returns the command and true if found, or empty command and false if not.
func (mb *ModeBindings) GetBinding(msg tea.KeyMsg) (Command, bool) {
	for _, binding := range mb.Bindings {
		if binding.Matches(msg) {
			return binding.Command, true
		}
	}
	return "", false
}

// Keymap contains all key bindings organized by mode.
type Keymap struct {
	// Name identifies this keymap (e.g., "default", "vim", "emacs").
	Name string

	// Description provides a human-readable description.
	Description string

	// Modes maps each mode to its bindings.
	Modes map[Mode]*ModeBindings
}

// GetBinding looks up a command for a key in a specific mode.
// Returns the command and true if found, or empty command and false if not.
func (km *Keymap) GetBinding(msg tea.KeyMsg, mode Mode) (Command, bool) {
	mb, ok := km.Modes[mode]
	if !ok {
		return "", false
	}
	return mb.GetBinding(msg)
}

// GetModeBindings returns all bindings for a specific mode.
func (km *Keymap) GetModeBindings(mode Mode) []KeyBinding {
	mb, ok := km.Modes[mode]
	if !ok {
		return nil
	}
	return mb.Bindings
}

// GetBindingsForCommand returns all bindings that trigger a specific command.
// Useful for displaying "Press X or Y to do Z" in help.
func (km *Keymap) GetBindingsForCommand(cmd Command, mode Mode) []KeyBinding {
	mb, ok := km.Modes[mode]
	if !ok {
		return nil
	}

	var result []KeyBinding
	for _, binding := range mb.Bindings {
		if binding.Command == cmd {
			result = append(result, binding)
		}
	}
	return result
}

// GetCategories returns all unique categories in a mode's bindings.
func (km *Keymap) GetCategories(mode Mode) []string {
	mb, ok := km.Modes[mode]
	if !ok {
		return nil
	}

	seen := make(map[string]bool)
	var categories []string

	for _, binding := range mb.Bindings {
		if binding.Category != "" && !seen[binding.Category] {
			seen[binding.Category] = true
			categories = append(categories, binding.Category)
		}
	}
	return categories
}

// GetBindingsByCategory returns bindings grouped by category for a mode.
func (km *Keymap) GetBindingsByCategory(mode Mode) map[string][]KeyBinding {
	mb, ok := km.Modes[mode]
	if !ok {
		return nil
	}

	result := make(map[string][]KeyBinding)
	for _, binding := range mb.Bindings {
		cat := binding.Category
		if cat == "" {
			cat = "Other"
		}
		result[cat] = append(result[cat], binding)
	}
	return result
}

// KeymapLoader is an interface for loading keymaps from various sources.
type KeymapLoader interface {
	// Load loads a keymap by name.
	// Returns the keymap and any error encountered.
	Load(name string) (*Keymap, error)

	// List returns available keymap names.
	List() []string
}

// KeymapConfig represents a serializable keymap configuration.
// This can be used for loading keymaps from JSON/YAML config files.
type KeymapConfig struct {
	Name        string                      `json:"name" yaml:"name"`
	Description string                      `json:"description" yaml:"description"`
	Extends     string                      `json:"extends,omitempty" yaml:"extends,omitempty"`
	Modes       map[string][]KeyBindingSpec `json:"modes" yaml:"modes"`
}

// KeyBindingSpec is a serializable key binding specification.
type KeyBindingSpec struct {
	Key         string `json:"key" yaml:"key"`                                    // e.g., "ctrl+r", "j", "enter"
	Command     string `json:"command" yaml:"command"`                            // Command name
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Category    string `json:"category,omitempty" yaml:"category,omitempty"`
}

// ParseKeySpec parses a key specification string into KeyType, Rune, and Modifiers.
// Examples: "ctrl+r", "shift+tab", "j", "enter", "alt+left"
func ParseKeySpec(spec string) (keyType tea.KeyType, r rune, mods Modifier, err error) {
	// This is a simplified parser - a full implementation would handle
	// all possible key combinations

	// Check for modifiers
	remaining := spec
	for {
		switch {
		case len(remaining) > 5 && remaining[:5] == "ctrl+":
			mods |= ModCtrl
			remaining = remaining[5:]
		case len(remaining) > 4 && remaining[:4] == "alt+":
			mods |= ModAlt
			remaining = remaining[4:]
		case len(remaining) > 6 && remaining[:6] == "shift+":
			mods |= ModShift
			remaining = remaining[6:]
		default:
			goto parseKey
		}
	}

parseKey:
	// Handle special keys
	switch remaining {
	case "enter":
		return tea.KeyEnter, 0, mods, nil
	case "tab":
		if mods&ModShift != 0 {
			return tea.KeyShiftTab, 0, mods &^ ModShift, nil
		}
		return tea.KeyTab, 0, mods, nil
	case "esc", "escape":
		return tea.KeyEsc, 0, mods, nil
	case "space":
		return tea.KeySpace, 0, mods, nil
	case "backspace":
		return tea.KeyBackspace, 0, mods, nil
	case "delete":
		return tea.KeyDelete, 0, mods, nil
	case "up":
		return tea.KeyUp, 0, mods, nil
	case "down":
		return tea.KeyDown, 0, mods, nil
	case "left":
		return tea.KeyLeft, 0, mods, nil
	case "right":
		return tea.KeyRight, 0, mods, nil
	case "home":
		return tea.KeyHome, 0, mods, nil
	case "end":
		return tea.KeyEnd, 0, mods, nil
	case "pgup", "pageup":
		return tea.KeyPgUp, 0, mods, nil
	case "pgdown", "pagedown":
		return tea.KeyPgDown, 0, mods, nil
	case "insert":
		return tea.KeyInsert, 0, mods, nil
	}

	// Handle ctrl+letter combinations
	if mods&ModCtrl != 0 && len(remaining) == 1 {
		ch := remaining[0]
		if ch >= 'a' && ch <= 'z' {
			// Map to tea.KeyCtrlA through tea.KeyCtrlZ
			ctrlKey := tea.KeyCtrlA + tea.KeyType(ch-'a')
			return ctrlKey, 0, mods &^ ModCtrl, nil // Remove ctrl from mods since it's in the key type
		}
	}

	// Handle function keys
	if len(remaining) >= 2 && remaining[0] == 'f' {
		var fNum int
		if _, err := fmt.Sscanf(remaining, "f%d", &fNum); err == nil && fNum >= 1 && fNum <= 20 {
			// Function keys are defined as separate constants, use a map
			fKeys := map[int]tea.KeyType{
				1: tea.KeyF1, 2: tea.KeyF2, 3: tea.KeyF3, 4: tea.KeyF4, 5: tea.KeyF5,
				6: tea.KeyF6, 7: tea.KeyF7, 8: tea.KeyF8, 9: tea.KeyF9, 10: tea.KeyF10,
				11: tea.KeyF11, 12: tea.KeyF12, 13: tea.KeyF13, 14: tea.KeyF14, 15: tea.KeyF15,
				16: tea.KeyF16, 17: tea.KeyF17, 18: tea.KeyF18, 19: tea.KeyF19, 20: tea.KeyF20,
			}
			if keyType, ok := fKeys[fNum]; ok {
				return keyType, 0, mods, nil
			}
		}
	}

	// Single character - it's a rune
	if len(remaining) == 1 {
		return tea.KeyRunes, rune(remaining[0]), mods, nil
	}

	return 0, 0, 0, fmt.Errorf("unrecognized key spec: %s", spec)
}

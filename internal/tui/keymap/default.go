package keymap

import tea "github.com/charmbracelet/bubbletea"

// DefaultKeymap returns the default keymap configuration that matches
// the current key bindings in app.go.
func DefaultKeymap() *Keymap {
	return &Keymap{
		Name:        "default",
		Description: "Default Claudio TUI key bindings",
		Modes: map[Mode]*ModeBindings{
			ModeNormal:     defaultNormalBindings(),
			ModeCommand:    defaultCommandBindings(),
			ModeSearch:     defaultSearchBindings(),
			ModeFilter:     defaultFilterBindings(),
			ModeAddTask:    defaultAddTaskBindings(),
			ModeTemplate:   defaultTemplateBindings(),
			ModeInput:      defaultInputModeBindings(),
			ModeUltraPlan:  defaultUltraPlanBindings(),
			ModePlanEditor: defaultPlanEditorBindings(),
		},
	}
}

func defaultNormalBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeNormal,
		Bindings: []KeyBinding{
			// Instance navigation
			{KeyType: tea.KeyTab, Command: CmdNextInstance, Description: "Next instance", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'l', Command: CmdNextInstance, Description: "Next instance", Category: "Navigation"},
			{KeyType: tea.KeyShiftTab, Command: CmdPrevInstance, Description: "Previous instance", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'h', Command: CmdPrevInstance, Description: "Previous instance", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '1', Command: CmdJumpToInstance, Description: "Jump to instance 1", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '2', Command: CmdJumpToInstance, Description: "Jump to instance 2", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '3', Command: CmdJumpToInstance, Description: "Jump to instance 3", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '4', Command: CmdJumpToInstance, Description: "Jump to instance 4", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '5', Command: CmdJumpToInstance, Description: "Jump to instance 5", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '6', Command: CmdJumpToInstance, Description: "Jump to instance 6", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '7', Command: CmdJumpToInstance, Description: "Jump to instance 7", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '8', Command: CmdJumpToInstance, Description: "Jump to instance 8", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '9', Command: CmdJumpToInstance, Description: "Jump to instance 9", Category: "Navigation"},
			{KeyType: tea.KeyEnter, Command: CmdEnterInputMode, Description: "Enter input mode", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'i', Command: CmdEnterInputMode, Description: "Enter input mode", Category: "Navigation"},

			// Output navigation
			{KeyType: tea.KeyRunes, Rune: 'j', Command: CmdScrollDown, Description: "Scroll down", Category: "Scrolling"},
			{KeyType: tea.KeyDown, Command: CmdScrollDown, Description: "Scroll down", Category: "Scrolling"},
			{KeyType: tea.KeyRunes, Rune: 'k', Command: CmdScrollUp, Description: "Scroll up", Category: "Scrolling"},
			{KeyType: tea.KeyUp, Command: CmdScrollUp, Description: "Scroll up", Category: "Scrolling"},
			{KeyType: tea.KeyCtrlU, Command: CmdScrollHalfPageUp, Description: "Scroll half page up", Category: "Scrolling"},
			{KeyType: tea.KeyCtrlD, Command: CmdScrollHalfPageDn, Description: "Scroll half page down", Category: "Scrolling"},
			{KeyType: tea.KeyCtrlB, Command: CmdScrollPageUp, Description: "Scroll page up", Category: "Scrolling"},
			{KeyType: tea.KeyCtrlF, Command: CmdScrollPageDown, Description: "Scroll page down", Category: "Scrolling"},
			{KeyType: tea.KeyRunes, Rune: 'g', Command: CmdScrollToTop, Description: "Go to top", Category: "Scrolling"},
			{KeyType: tea.KeyRunes, Rune: 'G', Command: CmdScrollToBottom, Description: "Go to bottom", Category: "Scrolling"},

			// Mode entry
			{KeyType: tea.KeyRunes, Rune: ':', Command: CmdEnterCommandMode, Description: "Enter command mode", Category: "Modes"},
			{KeyType: tea.KeyRunes, Rune: '/', Command: CmdEnterSearchMode, Description: "Enter search mode", Category: "Modes"},
			{KeyType: tea.KeyRunes, Rune: '?', Command: CmdToggleHelp, Description: "Toggle help", Category: "Modes"},

			// Search
			{KeyType: tea.KeyRunes, Rune: 'n', Command: CmdNextSearchMatch, Description: "Next search match", Category: "Search"},
			{KeyType: tea.KeyRunes, Rune: 'N', Command: CmdPrevSearchMatch, Description: "Previous search match", Category: "Search"},
			{KeyType: tea.KeyCtrlUnderscore, Command: CmdClearSearch, Description: "Clear search", Category: "Search"}, // Ctrl+/

			// Instance control
			{KeyType: tea.KeyCtrlR, Command: CmdRestartInstance, Description: "Restart instance", Category: "Instance Control"},
			{KeyType: tea.KeyCtrlK, Command: CmdKillInstance, Description: "Kill instance", Category: "Instance Control"},

			// View toggles
			{KeyType: tea.KeyEsc, Command: CmdCloseDiffPanel, Description: "Close diff panel", Category: "View"},

			// Exit
			{KeyType: tea.KeyRunes, Rune: 'q', Command: CmdQuit, Description: "Quit", Category: "Application"},
			{KeyType: tea.KeyCtrlC, Command: CmdQuit, Description: "Quit", Category: "Application"},
		},
	}
}

func defaultCommandBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeCommand,
		Bindings: []KeyBinding{
			{KeyType: tea.KeyEsc, Command: CmdCancel, Description: "Exit command mode", Category: "Control"},
			{KeyType: tea.KeyEnter, Command: CmdConfirm, Description: "Execute command", Category: "Control"},
			{KeyType: tea.KeyBackspace, Command: CmdDeleteBack, Description: "Delete character", Category: "Editing"},
			{KeyType: tea.KeyDelete, Command: CmdDeleteBack, Description: "Delete character", Category: "Editing"},
			{KeyType: tea.KeySpace, Command: CmdInsertSpace, Description: "Add space", Category: "Editing"},
			{KeyType: tea.KeyRunes, Command: CmdInsertChar, Description: "Add character", Category: "Editing"},
		},
	}
}

func defaultSearchBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeSearch,
		Bindings: []KeyBinding{
			{KeyType: tea.KeyEsc, Command: CmdCancelSearch, Description: "Cancel search", Category: "Control"},
			{KeyType: tea.KeyEnter, Command: CmdExecuteSearch, Description: "Execute search", Category: "Control"},
			{KeyType: tea.KeyBackspace, Command: CmdDeleteBack, Description: "Delete character (live search)", Category: "Editing"},
			{KeyType: tea.KeySpace, Command: CmdInsertSpace, Description: "Add space", Category: "Editing"},
			{KeyType: tea.KeyRunes, Command: CmdInsertChar, Description: "Add character (live search)", Category: "Editing"},
		},
	}
}

func defaultFilterBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeFilter,
		Bindings: []KeyBinding{
			// Category toggles
			{KeyType: tea.KeyRunes, Rune: 'e', Command: CmdToggleErrors, Description: "Toggle errors", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: '1', Command: CmdToggleErrors, Description: "Toggle errors", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: 'w', Command: CmdToggleWarnings, Description: "Toggle warnings", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: '2', Command: CmdToggleWarnings, Description: "Toggle warnings", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: 't', Command: CmdToggleTools, Description: "Toggle tools", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: '3', Command: CmdToggleTools, Description: "Toggle tools", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: 'h', Command: CmdToggleThinking, Description: "Toggle thinking", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: '4', Command: CmdToggleThinking, Description: "Toggle thinking", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: 'p', Command: CmdToggleProgress, Description: "Toggle progress", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: '5', Command: CmdToggleProgress, Description: "Toggle progress", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: 'a', Command: CmdToggleAll, Description: "Toggle all", Category: "Categories"},
			{KeyType: tea.KeyRunes, Rune: 'c', Command: CmdClearFilter, Description: "Clear custom filter", Category: "Categories"},

			// Navigation
			{KeyType: tea.KeyEsc, Command: CmdExitFilter, Description: "Exit filter mode", Category: "Control"},
			{KeyType: tea.KeyRunes, Rune: 'F', Command: CmdExitFilter, Description: "Exit filter mode", Category: "Control"},
			{KeyType: tea.KeyRunes, Rune: 'q', Command: CmdExitFilter, Description: "Exit filter mode", Category: "Control"},

			// Custom filter editing
			{KeyType: tea.KeySpace, Command: CmdInsertSpace, Description: "Add space to filter", Category: "Editing"},
			{KeyType: tea.KeyBackspace, Command: CmdDeleteBack, Description: "Remove character", Category: "Editing"},
			{KeyType: tea.KeyRunes, Command: CmdInsertChar, Description: "Custom filter regex", Category: "Editing"},
		},
	}
}

func defaultAddTaskBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeAddTask,
		Bindings: []KeyBinding{
			// Control
			{KeyType: tea.KeyEsc, Command: CmdCancel, Description: "Cancel", Category: "Control"},
			{KeyType: tea.KeyEnter, Command: CmdConfirm, Description: "Submit task", Category: "Control"},

			// Basic editing
			{KeyType: tea.KeyBackspace, Command: CmdDeleteBack, Description: "Delete back", Category: "Editing"},
			{KeyType: tea.KeyDelete, Command: CmdDeleteForward, Description: "Delete forward", Category: "Editing"},
			{KeyType: tea.KeyLeft, Command: CmdMoveCursorLeft, Description: "Move cursor left", Category: "Navigation"},
			{KeyType: tea.KeyRight, Command: CmdMoveCursorRight, Description: "Move cursor right", Category: "Navigation"},
			{KeyType: tea.KeyHome, Command: CmdMoveToLineStart, Description: "Start of line", Category: "Navigation"},
			{KeyType: tea.KeyEnd, Command: CmdMoveToLineEnd, Description: "End of line", Category: "Navigation"},

			// Line manipulation
			{KeyType: tea.KeyCtrlU, Command: CmdDeleteToLineStart, Description: "Delete to line start", Category: "Editing"},
			{KeyType: tea.KeyCtrlK, Command: CmdDeleteToLineEnd, Description: "Delete to line end", Category: "Editing"},
			{KeyType: tea.KeyCtrlJ, Command: CmdInsertNewline, Description: "Insert newline", Category: "Editing"},
			// Note: Shift+Enter and Alt+Enter also insert newlines (handled specially in app.go)

			// Word navigation
			{KeyType: tea.KeyLeft, Modifiers: ModAlt, Command: CmdPrevWordBoundary, Description: "Previous word", Category: "Navigation"},
			{KeyType: tea.KeyRight, Modifiers: ModAlt, Command: CmdNextWordBoundary, Description: "Next word", Category: "Navigation"},
			{KeyType: tea.KeyBackspace, Modifiers: ModAlt, Command: CmdDeletePrevWord, Description: "Delete previous word", Category: "Editing"},
			{KeyType: tea.KeyCtrlW, Command: CmdDeletePrevWord, Description: "Delete previous word", Category: "Editing"},

			// Multi-line navigation
			{KeyType: tea.KeyUp, Modifiers: ModAlt, Command: CmdMoveToInputStart, Description: "Start of input", Category: "Navigation"},
			{KeyType: tea.KeyCtrlA, Command: CmdMoveToInputStart, Description: "Start of input", Category: "Navigation"},
			{KeyType: tea.KeyDown, Modifiers: ModAlt, Command: CmdMoveToInputEnd, Description: "End of input", Category: "Navigation"},
			{KeyType: tea.KeyCtrlE, Command: CmdMoveToInputEnd, Description: "End of input", Category: "Navigation"},

			// Special
			{KeyType: tea.KeyRunes, Rune: '/', Command: CmdShowTemplates, Description: "Show templates (at line start)", Category: "Special"},
			{KeyType: tea.KeySpace, Command: CmdInsertSpace, Description: "Insert space", Category: "Editing"},
			{KeyType: tea.KeyRunes, Command: CmdInsertChar, Description: "Insert character", Category: "Editing"},
		},
	}
}

func defaultTemplateBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeTemplate,
		Bindings: []KeyBinding{
			{KeyType: tea.KeyEsc, Command: CmdCloseDropdown, Description: "Close dropdown", Category: "Control"},
			{KeyType: tea.KeyEnter, Command: CmdSelectTemplate, Description: "Select template", Category: "Control"},
			{KeyType: tea.KeyTab, Command: CmdSelectTemplate, Description: "Select template", Category: "Control"},
			{KeyType: tea.KeyUp, Command: CmdPrevTemplate, Description: "Previous template", Category: "Navigation"},
			{KeyType: tea.KeyDown, Command: CmdNextTemplate, Description: "Next template", Category: "Navigation"},
			{KeyType: tea.KeyBackspace, Command: CmdDeleteBack, Description: "Remove filter character", Category: "Editing"},
			{KeyType: tea.KeySpace, Command: CmdCloseAddSpace, Description: "Close and add space", Category: "Control"},
			{KeyType: tea.KeyRunes, Command: CmdFilterTemplates, Description: "Filter templates", Category: "Editing"},
		},
	}
}

func defaultInputModeBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeInput,
		Bindings: []KeyBinding{
			// The only binding that doesn't forward to tmux
			{KeyType: tea.KeyCtrlCloseBracket, Command: CmdExitInputMode, Description: "Exit input mode (Ctrl+])", Category: "Control"},
			// All other keys are forwarded to tmux via CmdForwardToTmux
			// This is handled specially in the key handler
		},
	}
}

func defaultUltraPlanBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModeUltraPlan,
		Bindings: []KeyBinding{
			// Group decision handling (context-dependent)
			{KeyType: tea.KeyRunes, Rune: 'c', Command: CmdUPContinue, Description: "Continue with partial work", Category: "Decision"},
			{KeyType: tea.KeyRunes, Rune: 'r', Command: CmdUPRetry, Description: "Retry failed tasks", Category: "Decision"},
			{KeyType: tea.KeyRunes, Rune: 'q', Command: CmdUPCancel, Description: "Cancel ultraplan", Category: "Decision"},

			// Plan management
			{KeyType: tea.KeyRunes, Rune: 'v', Command: CmdUPTogglePlanView, Description: "Toggle plan view", Category: "View"},
			{KeyType: tea.KeyRunes, Rune: 'p', Command: CmdUPParsePlan, Description: "Parse plan from file", Category: "Planning"},
			{KeyType: tea.KeyRunes, Rune: 'e', Command: CmdUPStartExecution, Description: "Start execution", Category: "Execution"},
			{KeyType: tea.KeyRunes, Rune: 'E', Command: CmdUPEnterPlanEditor, Description: "Enter plan editor", Category: "Execution"},
			// Note: 'c' is overloaded - CmdUPCancelExecution in executing phase
			{KeyType: tea.KeyRunes, Rune: 's', Command: CmdUPSignalSynthesis, Description: "Signal synthesis done", Category: "Execution"},
			// Note: 'r' is overloaded - CmdUPResumeConsol in consolidation phase

			// Navigation (same as normal mode)
			{KeyType: tea.KeyTab, Command: CmdNextInstance, Description: "Next instance", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'l', Command: CmdNextInstance, Description: "Next instance", Category: "Navigation"},
			{KeyType: tea.KeyShiftTab, Command: CmdPrevInstance, Description: "Previous instance", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'h', Command: CmdPrevInstance, Description: "Previous instance", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '1', Command: CmdJumpToInstance, Description: "Jump to task 1", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '2', Command: CmdJumpToInstance, Description: "Jump to task 2", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '3', Command: CmdJumpToInstance, Description: "Jump to task 3", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '4', Command: CmdJumpToInstance, Description: "Jump to task 4", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '5', Command: CmdJumpToInstance, Description: "Jump to task 5", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '6', Command: CmdJumpToInstance, Description: "Jump to task 6", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '7', Command: CmdJumpToInstance, Description: "Jump to task 7", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '8', Command: CmdJumpToInstance, Description: "Jump to task 8", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: '9', Command: CmdJumpToInstance, Description: "Jump to task 9", Category: "Navigation"},

			// Other
			{KeyType: tea.KeyRunes, Rune: 'o', Command: CmdUPOpenPR, Description: "Open first PR in browser", Category: "Actions"},
		},
	}
}

func defaultPlanEditorBindings() *ModeBindings {
	return &ModeBindings{
		Mode: ModePlanEditor,
		Bindings: []KeyBinding{
			// Global navigation
			{KeyType: tea.KeyRunes, Rune: 'q', Command: CmdPEExit, Description: "Exit editor", Category: "Control"},
			{KeyType: tea.KeyEsc, Command: CmdPEExit, Description: "Exit editor", Category: "Control"},
			{KeyType: tea.KeyRunes, Rune: 'j', Command: CmdPENextTask, Description: "Next task", Category: "Navigation"},
			{KeyType: tea.KeyDown, Command: CmdPENextTask, Description: "Next task", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'k', Command: CmdPEPrevTask, Description: "Previous task", Category: "Navigation"},
			{KeyType: tea.KeyUp, Command: CmdPEPrevTask, Description: "Previous task", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'g', Command: CmdPEFirstTask, Description: "First task", Category: "Navigation"},
			{KeyType: tea.KeyRunes, Rune: 'G', Command: CmdPELastTask, Description: "Last task", Category: "Navigation"},
			{KeyType: tea.KeyTab, Command: CmdPECycleForward, Description: "Cycle forward", Category: "Navigation"},
			{KeyType: tea.KeyShiftTab, Command: CmdPECycleBackward, Description: "Cycle backward", Category: "Navigation"},

			// View controls
			{KeyType: tea.KeyRunes, Rune: 'v', Command: CmdPEToggleValidation, Description: "Toggle validation panel", Category: "View"},
			{KeyType: tea.KeyRunes, Rune: 'r', Command: CmdPERefreshValid, Description: "Refresh validation", Category: "View"},
			{KeyType: tea.KeyPgUp, Command: CmdPEScrollValidUp, Description: "Scroll validation up", Category: "View"},
			{KeyType: tea.KeyPgDown, Command: CmdPEScrollValidDown, Description: "Scroll validation down", Category: "View"},

			// Task operations
			{KeyType: tea.KeyEnter, Command: CmdPEEditTitle, Description: "Edit title (or confirm)", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 't', Command: CmdPEEditTitle, Description: "Edit title", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'd', Command: CmdPEEditDescription, Description: "Edit description", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'f', Command: CmdPEEditFiles, Description: "Edit files list", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'p', Command: CmdPEEditPriority, Description: "Edit priority", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'c', Command: CmdPECycleComplexity, Description: "Cycle complexity", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'x', Command: CmdPEEditDependencies, Description: "Edit dependencies", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'D', Command: CmdPEDeleteTask, Description: "Delete task", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'n', Command: CmdPEAddTask, Description: "Add new task", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'J', Command: CmdPEMoveTaskDown, Description: "Move task down", Category: "Editing"},
			{KeyType: tea.KeyRunes, Rune: 'K', Command: CmdPEMoveTaskUp, Description: "Move task up", Category: "Editing"},

			// File operations
			{KeyType: tea.KeyRunes, Rune: 's', Command: CmdPESavePlan, Description: "Save plan", Category: "File"},
			{KeyType: tea.KeyRunes, Rune: 'e', Command: CmdPEConfirmExecute, Description: "Confirm and execute", Category: "File"},
		},
	}
}

// ExCommands maps ex command strings to their Command constants.
// This is used by command mode to look up the action for a typed command.
var ExCommands = map[string]Command{
	// Instance control
	"s":         CmdExStart,
	"start":     CmdExStart,
	"x":         CmdExStop,
	"stop":      CmdExStop,
	"p":         CmdExPause,
	"pause":     CmdExPause,
	"R":         CmdExReconnect,
	"reconnect": CmdExReconnect,
	"restart":   CmdExRestart,

	// Instance management
	"a":      CmdExAdd,
	"add":    CmdExAdd,
	"D":      CmdExRemove,
	"remove": CmdExRemove,
	"kill":   CmdExKill,
	"C":      CmdExClear,
	"clear":  CmdExClear,

	// View commands
	"d":         CmdExDiff,
	"diff":      CmdExDiff,
	"m":         CmdExMetrics,
	"metrics":   CmdExMetrics,
	"stats":     CmdExMetrics,
	"c":         CmdExConflicts,
	"conflicts": CmdExConflicts,
	"f":         CmdExFilter,
	"F":         CmdExFilter,
	"filter":    CmdExFilter,

	// Utility
	"t":    CmdExTmux,
	"tmux": CmdExTmux,
	"r":    CmdExPR,
	"pr":   CmdExPR,
	"h":    CmdExHelp,
	"help": CmdExHelp,
	"q":    CmdExQuit,
	"quit": CmdExQuit,
}

// LookupExCommand looks up an ex command by its string representation.
func LookupExCommand(cmd string) (Command, bool) {
	c, ok := ExCommands[cmd]
	return c, ok
}

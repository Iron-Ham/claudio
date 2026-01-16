package ultraplan

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// RenderContext provides the necessary context for rendering ultraplan views.
// This allows the view to access orchestrator and session state without
// direct coupling to the Model struct.
type RenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	UltraPlan    *State
	ActiveTab    int
	Width        int
	Height       int
	Outputs      map[string]string
	GetInstance  func(id string) *orchestrator.Instance
	IsSelected   func(instanceID string) bool

	// InputMode indicates whether input forwarding mode is active.
	// Used by help bar rendering to show appropriate mode badge.
	InputMode bool

	// TerminalFocused indicates whether the terminal pane has focus.
	TerminalFocused bool

	// TerminalDirMode is the current terminal directory mode ("invoke" or "worktree").
	TerminalDirMode string
}

// State holds ultra-plan specific UI state.
type State struct {
	Coordinator            *orchestrator.Coordinator
	ShowPlanView           bool                            // Toggle between plan view and normal output view
	SelectedTaskIdx        int                             // Currently selected task index for navigation
	NeedsNotification      bool                            // Set when user input is needed (checked on tick)
	LastNotifiedPhase      orchestrator.UltraPlanPhase     // Prevent duplicate notifications for same phase
	LastConsolidationPhase orchestrator.ConsolidationPhase // Track consolidation phase for pause detection
	NotifiedGroupDecision  bool                            // Prevent repeated notifications while awaiting group decision

	// Phase-aware navigation state
	NavigableInstances []string // Ordered list of navigable instance IDs
	SelectedNavIdx     int      // Index into navigableInstances

	// Group re-trigger mode
	RetriggerMode bool // When true, next digit key triggers group re-trigger

	// Collapsible group state
	CollapsedGroups  map[int]bool // Track explicit collapse state (true = collapsed, false = expanded)
	SelectedGroupIdx int          // Currently selected group index for group-level navigation (0 = first group)
	GroupNavMode     bool         // When true, arrow keys navigate groups instead of tasks
	// LastAutoExpandedGroup tracks which group was last auto-expanded to detect changes.
	// Initialized to -1 as a sentinel value to ensure the first active group is
	// always auto-expanded on initial render.
	LastAutoExpandedGroup int
}

// IsGroupCollapsed returns whether a group should be displayed as collapsed.
// Default behavior: groups are collapsed unless they are the current active group.
// When currentGroup is -1 (no active group), all groups default to collapsed.
// Users can explicitly expand/collapse groups, overriding the default.
func (s *State) IsGroupCollapsed(groupIdx, currentGroup int) bool {
	// If there's an explicit state set, use it
	if s.CollapsedGroups != nil {
		if explicit, ok := s.CollapsedGroups[groupIdx]; ok {
			return explicit
		}
	}
	// Default: collapsed unless it's the current group
	return groupIdx != currentGroup
}

// SetGroupExpanded explicitly sets a group to expanded state.
func (s *State) SetGroupExpanded(groupIdx int) {
	if s.CollapsedGroups == nil {
		s.CollapsedGroups = make(map[int]bool)
	}
	s.CollapsedGroups[groupIdx] = false
}

// SetGroupCollapsed explicitly sets a group to collapsed state.
func (s *State) SetGroupCollapsed(groupIdx int) {
	if s.CollapsedGroups == nil {
		s.CollapsedGroups = make(map[int]bool)
	}
	s.CollapsedGroups[groupIdx] = true
}

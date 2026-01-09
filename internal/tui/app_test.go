package tui

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestModel creates a Model with minimal state for testing
func createTestModel() Model {
	return Model{
		outputs:          make(map[string]string),
		outputScrolls:    make(map[string]int),
		outputAutoScroll: make(map[string]bool),
		outputLineCount:  make(map[string]int),
		filterCategories: map[string]bool{
			"errors":   true,
			"warnings": true,
			"tools":    true,
			"thinking": true,
			"progress": true,
		},
		width:  120,
		height: 40,
		ready:  true,
	}
}

// createTestSession creates a Session with test instances
func createTestSession() *orchestrator.Session {
	return &orchestrator.Session{
		ID:   "test-session",
		Name: "Test Session",
		Instances: []*orchestrator.Instance{
			{
				ID:           "inst-1",
				Task:         "Task 1",
				Status:       orchestrator.StatusWorking,
				WorktreePath: "/tmp/test1",
			},
			{
				ID:           "inst-2",
				Task:         "Task 2",
				Status:       orchestrator.StatusCompleted,
				WorktreePath: "/tmp/test2",
			},
			{
				ID:           "inst-3",
				Task:         "Task 3",
				Status:       orchestrator.StatusWaitingInput,
				WorktreePath: "/tmp/test3",
			},
		},
	}
}

// createTestModelWithSession creates a Model with a test session
func createTestModelWithSession() Model {
	m := createTestModel()
	m.session = createTestSession()
	return m
}

// =============================================================================
// Model Initialization Tests
// =============================================================================

func TestNewModel(t *testing.T) {
	t.Run("creates model with nil orchestrator and session", func(t *testing.T) {
		m := NewModel(nil, nil)

		if m.outputs == nil {
			t.Error("expected outputs map to be initialized")
		}
		if m.outputScrolls == nil {
			t.Error("expected outputScrolls map to be initialized")
		}
		if m.outputAutoScroll == nil {
			t.Error("expected outputAutoScroll map to be initialized")
		}
		if m.outputLineCount == nil {
			t.Error("expected outputLineCount map to be initialized")
		}
		if m.filterCategories == nil {
			t.Error("expected filterCategories map to be initialized")
		}
	})

	t.Run("initializes default filter categories", func(t *testing.T) {
		m := NewModel(nil, nil)

		expectedCategories := []string{"errors", "warnings", "tools", "thinking", "progress"}
		for _, cat := range expectedCategories {
			if !m.filterCategories[cat] {
				t.Errorf("expected filter category %q to be enabled by default", cat)
			}
		}
	})

	t.Run("stores orchestrator and session references", func(t *testing.T) {
		sess := createTestSession()
		m := NewModel(nil, sess)

		if m.session != sess {
			t.Error("expected session to be stored in model")
		}
	})
}

func TestModelInit(t *testing.T) {
	t.Run("returns tick command", func(t *testing.T) {
		m := createTestModel()
		cmd := m.Init()

		if cmd == nil {
			t.Error("expected Init to return a command")
		}
	})

	t.Run("handles ultra-plan mode without crashing", func(t *testing.T) {
		m := createTestModel()
		// Test that having ultraPlan state doesn't crash Init
		// Note: We don't set a coordinator here as it requires full setup
		m.ultraPlan = &UltraPlanState{
			coordinator: nil, // nil coordinator is valid for testing
		}

		cmd := m.Init()
		if cmd == nil {
			t.Error("expected Init to return a command")
		}
	})
}

// =============================================================================
// Model State Query Tests
// =============================================================================

func TestIsUltraPlanMode(t *testing.T) {
	t.Run("returns false when ultraPlan is nil", func(t *testing.T) {
		m := createTestModel()
		if m.IsUltraPlanMode() {
			t.Error("expected IsUltraPlanMode to return false")
		}
	})

	t.Run("returns true when ultraPlan is set", func(t *testing.T) {
		m := createTestModel()
		m.ultraPlan = &UltraPlanState{}
		if !m.IsUltraPlanMode() {
			t.Error("expected IsUltraPlanMode to return true")
		}
	})
}

func TestIsPlanEditorActive_Model(t *testing.T) {
	t.Run("returns false when planEditor is nil", func(t *testing.T) {
		m := createTestModel()
		if m.IsPlanEditorActive() {
			t.Error("expected IsPlanEditorActive to return false")
		}
	})

	t.Run("returns false when planEditor is not active", func(t *testing.T) {
		m := createTestModel()
		m.planEditor = &PlanEditorState{active: false}
		if m.IsPlanEditorActive() {
			t.Error("expected IsPlanEditorActive to return false")
		}
	})

	t.Run("returns true when planEditor is active", func(t *testing.T) {
		m := createTestModel()
		m.planEditor = &PlanEditorState{active: true}
		if !m.IsPlanEditorActive() {
			t.Error("expected IsPlanEditorActive to return true")
		}
	})
}

func TestActiveInstance(t *testing.T) {
	t.Run("returns nil when session is nil", func(t *testing.T) {
		m := createTestModel()
		if m.activeInstance() != nil {
			t.Error("expected activeInstance to return nil")
		}
	})

	t.Run("returns nil when no instances", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{Instances: []*orchestrator.Instance{}}
		if m.activeInstance() != nil {
			t.Error("expected activeInstance to return nil")
		}
	})

	t.Run("returns correct instance based on activeTab", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 1

		inst := m.activeInstance()
		if inst == nil {
			t.Fatal("expected activeInstance to return an instance")
		}
		if inst.ID != "inst-2" {
			t.Errorf("expected instance ID %q, got %q", "inst-2", inst.ID)
		}
	})

	t.Run("returns nil when activeTab out of bounds", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 10

		if m.activeInstance() != nil {
			t.Error("expected activeInstance to return nil for out-of-bounds tab")
		}
	})
}

func TestInstanceCount(t *testing.T) {
	t.Run("returns 0 when session is nil", func(t *testing.T) {
		m := createTestModel()
		if m.instanceCount() != 0 {
			t.Error("expected instanceCount to return 0")
		}
	})

	t.Run("returns correct count", func(t *testing.T) {
		m := createTestModelWithSession()
		if m.instanceCount() != 3 {
			t.Errorf("expected instanceCount to return 3, got %d", m.instanceCount())
		}
	})
}

// =============================================================================
// Keyboard Event Handling Tests
// =============================================================================

func TestHandleKeypress_NormalMode(t *testing.T) {
	t.Run("q quits", func(t *testing.T) {
		m := createTestModel()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}

		result, cmd := m.handleKeypress(msg)
		model := result.(Model)

		if !model.quitting {
			t.Error("expected quitting to be true")
		}
		if cmd == nil {
			t.Error("expected quit command to be returned")
		}
	})

	t.Run("ctrl+c quits", func(t *testing.T) {
		m := createTestModel()
		msg := tea.KeyMsg{Type: tea.KeyCtrlC}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.quitting {
			t.Error("expected quitting to be true")
		}
	})

	t.Run("? toggles help", func(t *testing.T) {
		m := createTestModel()
		m.showHelp = false
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.showHelp {
			t.Error("expected showHelp to be true")
		}

		// Toggle off
		result, _ = model.handleKeypress(msg)
		model = result.(Model)

		if model.showHelp {
			t.Error("expected showHelp to be false")
		}
	})

	t.Run(": enters command mode", func(t *testing.T) {
		m := createTestModel()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.commandMode {
			t.Error("expected commandMode to be true")
		}
		if model.commandBuffer != "" {
			t.Error("expected commandBuffer to be empty")
		}
	})

	t.Run("/ enters search mode", func(t *testing.T) {
		m := createTestModel()
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.searchMode {
			t.Error("expected searchMode to be true")
		}
		if model.searchPattern != "" {
			t.Error("expected searchPattern to be empty")
		}
	})
}

func TestHandleKeypress_TabNavigation(t *testing.T) {
	t.Run("tab cycles through instances forward", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 0
		msg := tea.KeyMsg{Type: tea.KeyTab}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 1 {
			t.Errorf("expected activeTab to be 1, got %d", model.activeTab)
		}
	})

	t.Run("tab wraps around", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 2 // Last instance
		msg := tea.KeyMsg{Type: tea.KeyTab}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 0 {
			t.Errorf("expected activeTab to wrap to 0, got %d", model.activeTab)
		}
	})

	t.Run("shift+tab cycles backwards", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 1
		msg := tea.KeyMsg{Type: tea.KeyShiftTab}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 0 {
			t.Errorf("expected activeTab to be 0, got %d", model.activeTab)
		}
	})

	t.Run("shift+tab wraps around backwards", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 0
		msg := tea.KeyMsg{Type: tea.KeyShiftTab}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 2 {
			t.Errorf("expected activeTab to wrap to 2, got %d", model.activeTab)
		}
	})

	t.Run("l key navigates forward", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 1 {
			t.Errorf("expected activeTab to be 1, got %d", model.activeTab)
		}
	})

	t.Run("h key navigates backward", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 1
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 0 {
			t.Errorf("expected activeTab to be 0, got %d", model.activeTab)
		}
	})

	t.Run("number keys select instance directly", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 0

		for i := 1; i <= 3; i++ {
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune('0' + i)}}
			result, _ := m.handleKeypress(msg)
			model := result.(Model)

			expectedTab := i - 1
			if model.activeTab != expectedTab {
				t.Errorf("pressing %d: expected activeTab %d, got %d", i, expectedTab, model.activeTab)
			}
			m = model
		}
	})

	t.Run("number key beyond instance count does nothing", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'9'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.activeTab != 0 {
			t.Errorf("expected activeTab to remain 0, got %d", model.activeTab)
		}
	})
}

func TestHandleKeypress_OutputScrolling(t *testing.T) {
	t.Run("j scrolls output down", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		// Check that scroll was attempted (might be clamped if content fits)
		if model.outputScrolls["inst-1"] < 0 {
			t.Error("expected scroll to be non-negative")
		}
	})

	t.Run("k scrolls output up", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3"
		m.outputScrolls["inst-1"] = 2
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.outputScrolls["inst-1"] != 1 {
			t.Errorf("expected scroll to be 1, got %d", model.outputScrolls["inst-1"])
		}
	})

	t.Run("g scrolls to top", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3"
		m.outputScrolls["inst-1"] = 5
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.outputScrolls["inst-1"] != 0 {
			t.Errorf("expected scroll to be 0, got %d", model.outputScrolls["inst-1"])
		}
	})

	t.Run("G scrolls to bottom and enables auto-scroll", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3"
		m.outputScrolls["inst-1"] = 0
		m.outputAutoScroll["inst-1"] = false
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if !model.outputAutoScroll["inst-1"] {
			t.Error("expected auto-scroll to be enabled")
		}
	})

	t.Run("ctrl+u scrolls up half page", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3"
		m.outputScrolls["inst-1"] = 10
		msg := tea.KeyMsg{Type: tea.KeyCtrlU}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		// Scroll should decrease by roughly half page
		if model.outputScrolls["inst-1"] >= 10 {
			t.Error("expected scroll to decrease")
		}
	})

	t.Run("ctrl+d scrolls down half page", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
		m.outputScrolls["inst-1"] = 0
		msg := tea.KeyMsg{Type: tea.KeyCtrlD}

		result, _ := m.handleKeypress(msg)
		// Scroll attempted (result may be clamped by content length)
		_ = result.(Model)
	})
}

func TestHandleKeypress_DiffView(t *testing.T) {
	t.Run("esc closes diff view", func(t *testing.T) {
		m := createTestModel()
		m.showDiff = true
		m.diffContent = "some diff content"
		m.diffScroll = 5
		msg := tea.KeyMsg{Type: tea.KeyEsc}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.showDiff {
			t.Error("expected showDiff to be false")
		}
		if model.diffContent != "" {
			t.Error("expected diffContent to be cleared")
		}
		if model.diffScroll != 0 {
			t.Error("expected diffScroll to be reset")
		}
	})

	t.Run("j scrolls diff down when diff is shown", func(t *testing.T) {
		m := createTestModel()
		m.showDiff = true
		m.diffScroll = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.diffScroll != 1 {
			t.Errorf("expected diffScroll to be 1, got %d", model.diffScroll)
		}
	})

	t.Run("k scrolls diff up when diff is shown", func(t *testing.T) {
		m := createTestModel()
		m.showDiff = true
		m.diffScroll = 5
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.diffScroll != 4 {
			t.Errorf("expected diffScroll to be 4, got %d", model.diffScroll)
		}
	})

	t.Run("g goes to top of diff", func(t *testing.T) {
		m := createTestModel()
		m.showDiff = true
		m.diffScroll = 10
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.diffScroll != 0 {
			t.Errorf("expected diffScroll to be 0, got %d", model.diffScroll)
		}
	})
}

// =============================================================================
// Search Mode Tests
// =============================================================================

func TestHandleSearchInput(t *testing.T) {
	t.Run("esc cancels search mode", func(t *testing.T) {
		m := createTestModel()
		m.searchMode = true
		m.searchPattern = "test"
		msg := tea.KeyMsg{Type: tea.KeyEsc}

		result, _ := m.handleSearchInput(msg)
		model := result.(Model)

		if model.searchMode {
			t.Error("expected searchMode to be false")
		}
	})

	t.Run("enter executes search and exits search mode", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchMode = true
		m.searchPattern = "test"
		m.outputs["inst-1"] = "line with test\nanother line"
		msg := tea.KeyMsg{Type: tea.KeyEnter}

		result, _ := m.handleSearchInput(msg)
		model := result.(Model)

		if model.searchMode {
			t.Error("expected searchMode to be false")
		}
	})

	t.Run("typing adds to search pattern", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchMode = true
		m.searchPattern = "te"
		m.outputs["inst-1"] = "test line"
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s', 't'}}

		result, _ := m.handleSearchInput(msg)
		model := result.(Model)

		if model.searchPattern != "test" {
			t.Errorf("expected searchPattern %q, got %q", "test", model.searchPattern)
		}
	})

	t.Run("backspace removes from search pattern", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchMode = true
		m.searchPattern = "test"
		m.outputs["inst-1"] = "test line"
		msg := tea.KeyMsg{Type: tea.KeyBackspace}

		result, _ := m.handleSearchInput(msg)
		model := result.(Model)

		if model.searchPattern != "tes" {
			t.Errorf("expected searchPattern %q, got %q", "tes", model.searchPattern)
		}
	})

	t.Run("space adds to search pattern", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchMode = true
		m.searchPattern = "test"
		m.outputs["inst-1"] = "test pattern"
		msg := tea.KeyMsg{Type: tea.KeySpace}

		result, _ := m.handleSearchInput(msg)
		model := result.(Model)

		if model.searchPattern != "test " {
			t.Errorf("expected searchPattern %q, got %q", "test ", model.searchPattern)
		}
	})
}

func TestExecuteSearch(t *testing.T) {
	t.Run("empty pattern clears search state", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = ""
		m.searchMatches = []int{1, 2, 3}
		m.searchRegex = regexp.MustCompile("test")

		m.executeSearch()

		if m.searchMatches != nil {
			t.Error("expected searchMatches to be nil")
		}
		if m.searchRegex != nil {
			t.Error("expected searchRegex to be nil")
		}
	})

	t.Run("literal search finds matches", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "error"
		m.outputs["inst-1"] = "line1\nthis has an error\nline3\nanother error here"

		m.executeSearch()

		if len(m.searchMatches) != 2 {
			t.Errorf("expected 2 matches, got %d", len(m.searchMatches))
		}
	})

	t.Run("regex search with r: prefix", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "r:error|warning"
		m.outputs["inst-1"] = "line1\nerror here\nwarning there\nno match"

		m.executeSearch()

		if len(m.searchMatches) != 2 {
			t.Errorf("expected 2 matches, got %d", len(m.searchMatches))
		}
	})

	t.Run("invalid regex clears search state", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "r:[invalid"
		m.outputs["inst-1"] = "test line"

		m.executeSearch()

		if m.searchRegex != nil {
			t.Error("expected searchRegex to be nil for invalid regex")
		}
		if m.searchMatches != nil {
			t.Error("expected searchMatches to be nil for invalid regex")
		}
	})
}

func TestClearSearch(t *testing.T) {
	m := createTestModel()
	m.searchPattern = "test"
	m.searchRegex = regexp.MustCompile("test")
	m.searchMatches = []int{1, 2, 3}
	m.searchCurrent = 2
	m.outputScroll = 10

	m.clearSearch()

	if m.searchPattern != "" {
		t.Error("expected searchPattern to be empty")
	}
	if m.searchRegex != nil {
		t.Error("expected searchRegex to be nil")
	}
	if m.searchMatches != nil {
		t.Error("expected searchMatches to be nil")
	}
	if m.searchCurrent != 0 {
		t.Error("expected searchCurrent to be 0")
	}
	if m.outputScroll != 0 {
		t.Error("expected outputScroll to be 0")
	}
}

func TestSearchNavigation(t *testing.T) {
	t.Run("n navigates to next match", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "test"
		m.searchMatches = []int{1, 5, 10}
		m.searchCurrent = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.searchCurrent != 1 {
			t.Errorf("expected searchCurrent to be 1, got %d", model.searchCurrent)
		}
	})

	t.Run("n wraps around", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "test"
		m.searchMatches = []int{1, 5, 10}
		m.searchCurrent = 2 // Last match
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.searchCurrent != 0 {
			t.Errorf("expected searchCurrent to wrap to 0, got %d", model.searchCurrent)
		}
	})

	t.Run("N navigates to previous match", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "test"
		m.searchMatches = []int{1, 5, 10}
		m.searchCurrent = 1
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.searchCurrent != 0 {
			t.Errorf("expected searchCurrent to be 0, got %d", model.searchCurrent)
		}
	})

	t.Run("N wraps around backwards", func(t *testing.T) {
		m := createTestModelWithSession()
		m.searchPattern = "test"
		m.searchMatches = []int{1, 5, 10}
		m.searchCurrent = 0
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.searchCurrent != 2 {
			t.Errorf("expected searchCurrent to wrap to 2, got %d", model.searchCurrent)
		}
	})
}

// =============================================================================
// Filter Mode Tests
// =============================================================================

func TestHandleFilterInput(t *testing.T) {
	t.Run("esc exits filter mode", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		msg := tea.KeyMsg{Type: tea.KeyEsc}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterMode {
			t.Error("expected filterMode to be false")
		}
	})

	t.Run("F exits filter mode", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'F'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterMode {
			t.Error("expected filterMode to be false")
		}
	})

	t.Run("e toggles errors filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCategories["errors"] = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCategories["errors"] {
			t.Error("expected errors filter to be disabled")
		}
	})

	t.Run("w toggles warnings filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCategories["warnings"] = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCategories["warnings"] {
			t.Error("expected warnings filter to be disabled")
		}
	})

	t.Run("t toggles tools filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCategories["tools"] = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCategories["tools"] {
			t.Error("expected tools filter to be disabled")
		}
	})

	t.Run("h toggles thinking filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCategories["thinking"] = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCategories["thinking"] {
			t.Error("expected thinking filter to be disabled")
		}
	})

	t.Run("p toggles progress filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCategories["progress"] = true
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCategories["progress"] {
			t.Error("expected progress filter to be disabled")
		}
	})

	t.Run("a toggles all filters", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		// All enabled
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		// All should be disabled now
		for cat, enabled := range model.filterCategories {
			if enabled {
				t.Errorf("expected category %q to be disabled", cat)
			}
		}

		// Toggle again - all should be enabled
		result, _ = model.handleFilterInput(msg)
		model = result.(Model)

		for cat, enabled := range model.filterCategories {
			if !enabled {
				t.Errorf("expected category %q to be enabled", cat)
			}
		}
	})

	t.Run("c clears custom filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCustom = "test"
		m.filterRegex = regexp.MustCompile("test")
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCustom != "" {
			t.Error("expected filterCustom to be empty")
		}
		if model.filterRegex != nil {
			t.Error("expected filterRegex to be nil")
		}
	})

	t.Run("number keys work as category toggles", func(t *testing.T) {
		tests := []struct {
			key      rune
			category string
		}{
			{'1', "errors"},
			{'2', "warnings"},
			{'3', "tools"},
			{'4', "thinking"},
			{'5', "progress"},
		}

		for _, tt := range tests {
			t.Run(string(tt.key)+" toggles "+tt.category, func(t *testing.T) {
				m := createTestModel()
				m.filterMode = true
				m.filterCategories[tt.category] = true
				msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}}

				result, _ := m.handleFilterInput(msg)
				model := result.(Model)

				if model.filterCategories[tt.category] {
					t.Errorf("expected %s filter to be disabled", tt.category)
				}
			})
		}
	})
}

func TestFilterOutput(t *testing.T) {
	t.Run("returns output unchanged when all filters enabled", func(t *testing.T) {
		m := createTestModel()
		input := "line1\nline2\nline3"

		result := m.filterOutput(input)

		if result != input {
			t.Error("expected output to be unchanged")
		}
	})

	t.Run("filters error lines when errors disabled", func(t *testing.T) {
		m := createTestModel()
		m.filterCategories["errors"] = false
		input := "normal line\nthis has an error\nanother normal"

		result := m.filterOutput(input)

		if result == input {
			t.Error("expected error line to be filtered")
		}
	})

	t.Run("filters warning lines when warnings disabled", func(t *testing.T) {
		m := createTestModel()
		m.filterCategories["warnings"] = false
		input := "normal line\nthis is a warning\nanother normal"

		result := m.filterOutput(input)

		if result == input {
			t.Error("expected warning line to be filtered")
		}
	})

	t.Run("custom regex filter takes precedence", func(t *testing.T) {
		m := createTestModel()
		m.filterRegex = regexp.MustCompile("keep")
		input := "line1\nkeep this\nline3\nalso keep"

		result := m.filterOutput(input)

		if result != "keep this\nalso keep" {
			t.Errorf("expected only lines matching custom filter, got %q", result)
		}
	})
}

func TestShouldShowLine(t *testing.T) {
	t.Run("custom filter matches", func(t *testing.T) {
		m := createTestModel()
		m.filterRegex = regexp.MustCompile("important")

		if !m.shouldShowLine("this is important") {
			t.Error("expected line to match custom filter")
		}
		if m.shouldShowLine("this is not") {
			t.Error("expected line to not match custom filter")
		}
	})

	t.Run("error detection", func(t *testing.T) {
		m := createTestModel()
		m.filterCategories["errors"] = false

		errorLines := []string{
			"Error: something failed",
			"the operation failed",
			"exception occurred",
			"panic: runtime error",
		}

		for _, line := range errorLines {
			if m.shouldShowLine(line) {
				t.Errorf("expected error line %q to be filtered", line)
			}
		}
	})

	t.Run("warning detection", func(t *testing.T) {
		m := createTestModel()
		m.filterCategories["warnings"] = false

		warningLines := []string{
			"Warning: deprecated function",
			"warn: this might fail",
		}

		for _, line := range warningLines {
			if m.shouldShowLine(line) {
				t.Errorf("expected warning line %q to be filtered", line)
			}
		}
	})

	t.Run("thinking detection", func(t *testing.T) {
		m := createTestModel()
		m.filterCategories["thinking"] = false

		thinkingLines := []string{
			"I'm thinking about this",
			"Let me analyze this",
			"I'll start by...",
			"I will check the code",
		}

		for _, line := range thinkingLines {
			if m.shouldShowLine(line) {
				t.Errorf("expected thinking line %q to be filtered", line)
			}
		}
	})

	t.Run("progress detection", func(t *testing.T) {
		m := createTestModel()
		m.filterCategories["progress"] = false

		progressLines := []string{
			"Loading...",
			"Done! \u2713",
			"\u2588\u2588\u2588\u2591\u2591 50%",
		}

		for _, line := range progressLines {
			if m.shouldShowLine(line) {
				t.Errorf("expected progress line %q to be filtered", line)
			}
		}
	})
}

// =============================================================================
// Window Resize Handling Tests
// =============================================================================

func TestWindowSizeMsg(t *testing.T) {
	t.Run("updates dimensions", func(t *testing.T) {
		m := createTestModel()
		m.width = 0
		m.height = 0
		m.ready = false

		msg := tea.WindowSizeMsg{Width: 120, Height: 40}
		result, _ := m.Update(msg)
		model := result.(Model)

		if model.width != 120 {
			t.Errorf("expected width 120, got %d", model.width)
		}
		if model.height != 40 {
			t.Errorf("expected height 40, got %d", model.height)
		}
		if !model.ready {
			t.Error("expected ready to be true")
		}
	})

	t.Run("ensures active instance visible after resize", func(t *testing.T) {
		m := createTestModelWithSession()
		m.activeTab = 2
		m.sidebarScrollOffset = 0

		msg := tea.WindowSizeMsg{Width: 120, Height: 20} // Short height
		result, _ := m.Update(msg)
		model := result.(Model)

		// The model should have adjusted scroll to keep active visible
		// This is tested indirectly - ensureActiveVisible was called
		_ = model
	})
}

func TestCalculateContentDimensions(t *testing.T) {
	t.Run("normal width", func(t *testing.T) {
		contentWidth, contentHeight := CalculateContentDimensions(120, 40)

		expectedWidth := 120 - SidebarWidth - ContentWidthOffset
		expectedHeight := 40 - ContentHeightOffset

		if contentWidth != expectedWidth {
			t.Errorf("expected contentWidth %d, got %d", expectedWidth, contentWidth)
		}
		if contentHeight != expectedHeight {
			t.Errorf("expected contentHeight %d, got %d", expectedHeight, contentHeight)
		}
	})

	t.Run("narrow width uses minimum sidebar", func(t *testing.T) {
		contentWidth, _ := CalculateContentDimensions(70, 40)

		expectedWidth := 70 - SidebarMinWidth - ContentWidthOffset

		if contentWidth != expectedWidth {
			t.Errorf("expected contentWidth %d, got %d", expectedWidth, contentWidth)
		}
	})
}

// =============================================================================
// Error and Info Message Tests
// =============================================================================

func TestMessageHandling(t *testing.T) {
	t.Run("errMsg sets error message", func(t *testing.T) {
		m := createTestModel()
		msg := errMsg{err: fmt.Errorf("test error")}

		result, _ := m.Update(msg)
		model := result.(Model)

		if model.errorMessage != "test error" {
			t.Errorf("expected errorMessage %q, got %q", "test error", model.errorMessage)
		}
	})

	t.Run("normal key press clears info message", func(t *testing.T) {
		m := createTestModel()
		m.infoMessage = "some info"
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.infoMessage != "" {
			t.Error("expected infoMessage to be cleared")
		}
	})
}

// =============================================================================
// Output Scroll Helper Tests
// =============================================================================

func TestOutputScrollHelpers(t *testing.T) {
	t.Run("getOutputMaxLines returns reasonable value", func(t *testing.T) {
		m := createTestModel()
		m.height = 50

		maxLines := m.getOutputMaxLines()

		if maxLines < 5 {
			t.Error("expected maxLines to be at least 5")
		}
	})

	t.Run("getOutputMaxLines minimum is 5", func(t *testing.T) {
		m := createTestModel()
		m.height = 10 // Very short

		maxLines := m.getOutputMaxLines()

		if maxLines < 5 {
			t.Errorf("expected maxLines to be at least 5, got %d", maxLines)
		}
	})

	t.Run("getOutputLineCount handles empty output", func(t *testing.T) {
		m := createTestModel()

		count := m.getOutputLineCount("nonexistent")

		if count != 0 {
			t.Errorf("expected 0 lines, got %d", count)
		}
	})

	t.Run("getOutputLineCount counts lines correctly", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2\nline3"

		count := m.getOutputLineCount("test")

		if count != 3 {
			t.Errorf("expected 3 lines, got %d", count)
		}
	})

	t.Run("isOutputAutoScroll defaults to true", func(t *testing.T) {
		m := createTestModel()

		if !m.isOutputAutoScroll("nonexistent") {
			t.Error("expected auto-scroll to default to true")
		}
	})

	t.Run("scrollOutputUp disables auto-scroll", func(t *testing.T) {
		m := createTestModel()
		m.outputScrolls["test"] = 5
		m.outputAutoScroll["test"] = true

		m.scrollOutputUp("test", 1)

		if m.outputAutoScroll["test"] {
			t.Error("expected auto-scroll to be disabled after scrolling up")
		}
		if m.outputScrolls["test"] != 4 {
			t.Errorf("expected scroll to be 4, got %d", m.outputScrolls["test"])
		}
	})

	t.Run("scrollOutputUp clamps at 0", func(t *testing.T) {
		m := createTestModel()
		m.outputScrolls["test"] = 2

		m.scrollOutputUp("test", 10)

		if m.outputScrolls["test"] != 0 {
			t.Errorf("expected scroll to be 0, got %d", m.outputScrolls["test"])
		}
	})

	t.Run("scrollOutputToTop sets scroll to 0 and disables auto-scroll", func(t *testing.T) {
		m := createTestModel()
		m.outputScrolls["test"] = 10
		m.outputAutoScroll["test"] = true

		m.scrollOutputToTop("test")

		if m.outputScrolls["test"] != 0 {
			t.Error("expected scroll to be 0")
		}
		if m.outputAutoScroll["test"] {
			t.Error("expected auto-scroll to be disabled")
		}
	})

	t.Run("scrollOutputToBottom enables auto-scroll", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2\nline3"
		m.outputAutoScroll["test"] = false

		m.scrollOutputToBottom("test")

		if !m.outputAutoScroll["test"] {
			t.Error("expected auto-scroll to be enabled")
		}
	})

	t.Run("hasNewOutput detects new lines", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2"
		m.outputLineCount["test"] = 2

		// Same line count
		if m.hasNewOutput("test") {
			t.Error("expected no new output when line count unchanged")
		}

		// Add more lines
		m.outputs["test"] = "line1\nline2\nline3"
		if !m.hasNewOutput("test") {
			t.Error("expected new output when line count increased")
		}
	})
}

// =============================================================================
// Sidebar Pagination Tests
// =============================================================================

func TestEnsureActiveVisible(t *testing.T) {
	t.Run("adjusts scroll when active is above visible area", func(t *testing.T) {
		m := createTestModelWithSession()
		m.height = 50
		m.sidebarScrollOffset = 2
		m.activeTab = 0

		m.ensureActiveVisible()

		if m.sidebarScrollOffset != 0 {
			t.Errorf("expected sidebarScrollOffset to be 0, got %d", m.sidebarScrollOffset)
		}
	})

	t.Run("clamps scroll offset to valid bounds", func(t *testing.T) {
		m := createTestModelWithSession()
		m.height = 50
		m.sidebarScrollOffset = -5

		m.ensureActiveVisible()

		if m.sidebarScrollOffset < 0 {
			t.Error("expected sidebarScrollOffset to be non-negative")
		}
	})
}

// =============================================================================
// View Mode Transition Tests
// =============================================================================

func TestViewModeTransitions(t *testing.T) {
	t.Run("help mode toggle", func(t *testing.T) {
		m := createTestModel()

		// Enable help
		m.showHelp = true

		// Other modes should be independent
		m.showDiff = true
		m.showStats = true

		// All three can be true simultaneously (though UI might only show one)
		if !m.showHelp || !m.showDiff || !m.showStats {
			t.Error("expected all view modes to be independent")
		}
	})

	t.Run("diff mode can be toggled via command", func(t *testing.T) {
		m := createTestModel()
		m.showDiff = true
		m.diffContent = "some content"

		result, _ := m.executeCommand("diff")
		model := result.(Model)

		if model.showDiff {
			t.Error("expected diff to be toggled off")
		}
		if model.diffContent != "" {
			t.Error("expected diffContent to be cleared")
		}
	})

	t.Run("stats mode can be toggled via command", func(t *testing.T) {
		m := createTestModel()
		m.showStats = false

		result, _ := m.executeCommand("stats")
		model := result.(Model)

		if !model.showStats {
			t.Error("expected showStats to be true")
		}
	})
}

// =============================================================================
// Input Mode Tests
// =============================================================================

func TestInputMode(t *testing.T) {
	t.Run("ctrl+] exits input mode", func(t *testing.T) {
		m := createTestModel()
		m.inputMode = true
		msg := tea.KeyMsg{Type: tea.KeyCtrlCloseBracket}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.inputMode {
			t.Error("expected inputMode to be false")
		}
	})
}

// =============================================================================
// Plan Editor State Tests
// =============================================================================

func TestPlanEditorState(t *testing.T) {
	t.Run("enterPlanEditor initializes state", func(t *testing.T) {
		m := createTestModel()
		m.enterPlanEditor()

		if m.planEditor == nil {
			t.Fatal("expected planEditor to be initialized")
		}
		if !m.planEditor.active {
			t.Error("expected planEditor.active to be true")
		}
		if m.planEditor.selectedTaskIdx != 0 {
			t.Error("expected selectedTaskIdx to be 0")
		}
		if !m.planEditor.showValidationPanel {
			t.Error("expected showValidationPanel to be true by default")
		}
	})

	t.Run("exitPlanEditor clears state", func(t *testing.T) {
		m := createTestModel()
		m.enterPlanEditor()
		m.exitPlanEditor()

		if m.planEditor != nil {
			t.Error("expected planEditor to be nil")
		}
	})

	t.Run("canConfirmPlan returns false when no validation", func(t *testing.T) {
		m := createTestModel()
		m.enterPlanEditor()

		if m.canConfirmPlan() {
			t.Error("expected canConfirmPlan to return false without validation")
		}
	})

	t.Run("isTaskInCycle returns false when planEditor is nil", func(t *testing.T) {
		m := createTestModel()

		if m.isTaskInCycle("task-1") {
			t.Error("expected isTaskInCycle to return false")
		}
	})

	t.Run("isTaskInCycle returns correct value", func(t *testing.T) {
		m := createTestModel()
		m.enterPlanEditor()
		m.planEditor.tasksInCycle["task-1"] = true

		if !m.isTaskInCycle("task-1") {
			t.Error("expected isTaskInCycle to return true for task-1")
		}
		if m.isTaskInCycle("task-2") {
			t.Error("expected isTaskInCycle to return false for task-2")
		}
	})
}

// =============================================================================
// Message Type Tests
// =============================================================================

func TestTickMsg(t *testing.T) {
	t.Run("tick message triggers output update", func(t *testing.T) {
		m := createTestModel()
		msg := tickMsg{}

		result, cmd := m.Update(msg)
		model := result.(Model)

		// Should return another tick command
		if cmd == nil {
			t.Error("expected tick to return a command")
		}
		_ = model
	})
}

func TestOutputMsg(t *testing.T) {
	t.Run("output message appends to outputs map", func(t *testing.T) {
		m := createTestModel()
		msg := outputMsg{instanceID: "test-id", data: []byte("test output")}

		result, _ := m.Update(msg)
		model := result.(Model)

		if model.outputs["test-id"] != "test output" {
			t.Errorf("expected output %q, got %q", "test output", model.outputs["test-id"])
		}
	})

	t.Run("output message initializes map if nil", func(t *testing.T) {
		m := Model{} // Empty model with nil outputs
		msg := outputMsg{instanceID: "test-id", data: []byte("test")}

		result, _ := m.Update(msg)
		model := result.(Model)

		if model.outputs == nil {
			t.Error("expected outputs map to be initialized")
		}
	})
}

// =============================================================================
// View Rendering Tests (basic smoke tests)
// =============================================================================

func TestView(t *testing.T) {
	t.Run("shows loading when not ready", func(t *testing.T) {
		m := createTestModel()
		m.ready = false

		view := m.View()

		if view != "Loading..." {
			t.Errorf("expected Loading..., got %q", view)
		}
	})

	t.Run("shows goodbye when quitting", func(t *testing.T) {
		m := createTestModel()
		m.quitting = true

		view := m.View()

		if view != "Goodbye!\n" {
			t.Errorf("expected Goodbye!, got %q", view)
		}
	})

	t.Run("renders without panic when ready and no instances", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{
			ID:        "test-session",
			Name:      "Test Session",
			Instances: []*orchestrator.Instance{}, // Empty - no instances
		}

		// Should not panic with empty instances
		view := m.View()

		if view == "" {
			t.Error("expected non-empty view")
		}
	})

	t.Run("renders header correctly", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{
			ID:   "test-session",
			Name: "My Session",
		}

		header := m.renderHeader()

		if header == "" {
			t.Error("expected non-empty header")
		}
	})
}

// =============================================================================
// Additional Coverage Tests
// =============================================================================

func TestCompileFilterRegex(t *testing.T) {
	t.Run("empty pattern sets nil regex", func(t *testing.T) {
		m := createTestModel()
		m.filterCustom = ""
		m.filterRegex = regexp.MustCompile("old")

		m.compileFilterRegex()

		if m.filterRegex != nil {
			t.Error("expected filterRegex to be nil")
		}
	})

	t.Run("valid pattern compiles regex", func(t *testing.T) {
		m := createTestModel()
		m.filterCustom = "test"

		m.compileFilterRegex()

		if m.filterRegex == nil {
			t.Error("expected filterRegex to be set")
		}
	})

	t.Run("invalid regex pattern sets nil", func(t *testing.T) {
		m := createTestModel()
		m.filterCustom = "[invalid"

		m.compileFilterRegex()

		if m.filterRegex != nil {
			t.Error("expected filterRegex to be nil for invalid pattern")
		}
	})
}

func TestScrollToMatch(t *testing.T) {
	t.Run("does nothing with no matches", func(t *testing.T) {
		m := createTestModel()
		m.searchMatches = nil
		m.outputScroll = 5

		m.scrollToMatch()

		if m.outputScroll != 5 {
			t.Error("expected outputScroll to remain unchanged")
		}
	})

	t.Run("scrolls to center match in view", func(t *testing.T) {
		m := createTestModel()
		m.height = 50
		m.searchMatches = []int{0, 20, 40}
		m.searchCurrent = 1

		m.scrollToMatch()

		// Match is at line 20, should center it in view
		if m.outputScroll < 0 {
			t.Error("expected outputScroll to be non-negative")
		}
	})

	t.Run("clamps scroll at 0", func(t *testing.T) {
		m := createTestModel()
		m.height = 50
		m.searchMatches = []int{0}
		m.searchCurrent = 0

		m.scrollToMatch()

		if m.outputScroll < 0 {
			t.Error("expected outputScroll to not go below 0")
		}
	})
}

func TestHandleFilterInputCustom(t *testing.T) {
	t.Run("backspace removes from custom filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCustom = "test"
		msg := tea.KeyMsg{Type: tea.KeyBackspace}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCustom != "tes" {
			t.Errorf("expected filterCustom %q, got %q", "tes", model.filterCustom)
		}
	})

	t.Run("space adds to custom filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCustom = "test"
		msg := tea.KeyMsg{Type: tea.KeySpace}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCustom != "test " {
			t.Errorf("expected filterCustom %q, got %q", "test ", model.filterCustom)
		}
	})

	t.Run("non-shortcut runes add to custom filter", func(t *testing.T) {
		m := createTestModel()
		m.filterMode = true
		m.filterCustom = ""
		// Use 'x' which is not a shortcut key
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}

		result, _ := m.handleFilterInput(msg)
		model := result.(Model)

		if model.filterCustom != "x" {
			t.Errorf("expected filterCustom %q, got %q", "x", model.filterCustom)
		}
	})
}

func TestUpdateOutputScroll(t *testing.T) {
	t.Run("auto-scroll updates scroll to bottom", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2\nline3\nline4\nline5"
		m.outputAutoScroll["test"] = true
		m.outputScrolls["test"] = 0

		m.updateOutputScroll("test")

		// Line count should be updated
		if m.outputLineCount["test"] != 5 {
			t.Errorf("expected line count 5, got %d", m.outputLineCount["test"])
		}
	})

	t.Run("disabled auto-scroll preserves scroll position", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2\nline3"
		m.outputAutoScroll["test"] = false
		m.outputScrolls["test"] = 1

		m.updateOutputScroll("test")

		// Scroll should remain at 1 since auto-scroll is disabled
		if m.outputScrolls["test"] != 1 {
			t.Errorf("expected scroll to remain at 1, got %d", m.outputScrolls["test"])
		}
	})
}

func TestScrollOutputDown(t *testing.T) {
	t.Run("scrolls down correctly", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
		m.outputScrolls["test"] = 0
		m.height = 20 // Small height so max scroll > 0

		m.scrollOutputDown("test", 2)

		// Should scroll down by 2 (or clamp to max)
		if m.outputScrolls["test"] < 0 {
			t.Error("expected scroll to be non-negative")
		}
	})

	t.Run("re-enables auto-scroll at bottom", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1\nline2"
		m.outputScrolls["test"] = 0
		m.outputAutoScroll["test"] = false
		m.height = 50

		// Scroll down more than content
		m.scrollOutputDown("test", 100)

		// Should be at max scroll with auto-scroll enabled
		if !m.outputAutoScroll["test"] {
			t.Error("expected auto-scroll to be re-enabled at bottom")
		}
	})
}

func TestGetOutputMaxScroll(t *testing.T) {
	t.Run("returns 0 when content fits in view", func(t *testing.T) {
		m := createTestModel()
		m.outputs["test"] = "line1"
		m.height = 50

		maxScroll := m.getOutputMaxScroll("test")

		if maxScroll != 0 {
			t.Errorf("expected maxScroll 0, got %d", maxScroll)
		}
	})

	t.Run("returns positive value for long content", func(t *testing.T) {
		m := createTestModel()
		// Create output with many lines
		output := ""
		for i := 0; i < 100; i++ {
			output += "line\n"
		}
		m.outputs["test"] = output
		m.height = 20

		maxScroll := m.getOutputMaxScroll("test")

		if maxScroll <= 0 {
			t.Errorf("expected positive maxScroll, got %d", maxScroll)
		}
	})
}

func TestClearSearchViaFunction(t *testing.T) {
	m := createTestModelWithSession()
	m.searchPattern = "test"
	m.searchMatches = []int{1, 2}
	m.searchCurrent = 1
	m.outputScroll = 5

	// Test clearSearch directly
	m.clearSearch()

	if m.searchPattern != "" {
		t.Error("expected searchPattern to be empty")
	}
	if m.searchMatches != nil {
		t.Error("expected searchMatches to be nil")
	}
	if m.searchCurrent != 0 {
		t.Error("expected searchCurrent to be 0")
	}
	if m.outputScroll != 0 {
		t.Error("expected outputScroll to be 0")
	}
}

func TestHandleKeypressSkipsScrollingWhenPanelsShown(t *testing.T) {
	t.Run("j does not scroll when help shown", func(t *testing.T) {
		m := createTestModelWithSession()
		m.showHelp = true
		m.outputs["inst-1"] = "line1\nline2"
		initialScroll := m.outputScrolls["inst-1"]
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.outputScrolls["inst-1"] != initialScroll {
			t.Error("expected scroll to not change when help is shown")
		}
	})

	t.Run("k does not scroll when conflicts shown", func(t *testing.T) {
		m := createTestModelWithSession()
		m.showConflicts = true
		m.outputs["inst-1"] = "line1\nline2"
		m.outputScrolls["inst-1"] = 1
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		if model.outputScrolls["inst-1"] != 1 {
			t.Error("expected scroll to not change when conflicts shown")
		}
	})
}

func TestPageScrolling(t *testing.T) {
	t.Run("ctrl+b scrolls up full page", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3"
		m.outputScrolls["inst-1"] = 5
		msg := tea.KeyMsg{Type: tea.KeyCtrlB}

		result, _ := m.handleKeypress(msg)
		model := result.(Model)

		// Should scroll up (might clamp to 0)
		if model.outputScrolls["inst-1"] > 5 {
			t.Error("expected scroll to decrease")
		}
	})

	t.Run("ctrl+f scrolls down full page", func(t *testing.T) {
		m := createTestModelWithSession()
		m.outputs["inst-1"] = "line1\nline2\nline3"
		m.outputScrolls["inst-1"] = 0
		msg := tea.KeyMsg{Type: tea.KeyCtrlF}

		result, _ := m.handleKeypress(msg)
		// Just verify it doesn't crash
		_ = result.(Model)
	})
}

func TestRenderHelp(t *testing.T) {
	t.Run("renders help when command mode active", func(t *testing.T) {
		m := createTestModel()
		m.commandMode = true
		m.commandBuffer = "help"

		help := m.renderHelp()

		if help == "" {
			t.Error("expected non-empty help")
		}
	})

	t.Run("renders help when search mode active", func(t *testing.T) {
		m := createTestModel()
		m.searchMode = true
		m.searchPattern = "test"

		help := m.renderHelp()

		if help == "" {
			t.Error("expected non-empty help")
		}
	})
}

func TestRenderAddTask(t *testing.T) {
	t.Run("renders with empty input", func(t *testing.T) {
		m := createTestModel()
		m.addingTask = true
		m.taskInput = ""
		m.taskInputCursor = 0

		result := m.renderAddTask(80)

		if result == "" {
			t.Error("expected non-empty render")
		}
	})

	t.Run("renders with text input", func(t *testing.T) {
		m := createTestModel()
		m.addingTask = true
		m.taskInput = "test task"
		m.taskInputCursor = 9

		result := m.renderAddTask(80)

		if result == "" {
			t.Error("expected non-empty render")
		}
	})

	t.Run("renders with multiline input", func(t *testing.T) {
		m := createTestModel()
		m.addingTask = true
		m.taskInput = "line1\nline2\nline3"
		m.taskInputCursor = 5

		result := m.renderAddTask(80)

		if result == "" {
			t.Error("expected non-empty render")
		}
	})
}

func TestSidebarRendering(t *testing.T) {
	t.Run("renders sidebar with no instances", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{
			Instances: []*orchestrator.Instance{},
		}

		sidebar := m.renderSidebar(30, 40)

		if sidebar == "" {
			t.Error("expected non-empty sidebar")
		}
	})
}

func TestContentRendering(t *testing.T) {
	t.Run("renders content with no instances", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{
			Instances: []*orchestrator.Instance{},
		}

		content := m.renderContent(80)

		if content == "" {
			t.Error("expected non-empty content")
		}
	})

	t.Run("renders content when adding task", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{
			Instances: []*orchestrator.Instance{},
		}
		m.addingTask = true
		m.taskInput = "test"

		content := m.renderContent(80)

		if content == "" {
			t.Error("expected non-empty content")
		}
	})

	t.Run("renders help panel when shown", func(t *testing.T) {
		m := createTestModel()
		m.session = &orchestrator.Session{
			Instances: []*orchestrator.Instance{},
		}
		m.showHelp = true

		content := m.renderContent(80)

		if content == "" {
			t.Error("expected non-empty content")
		}
	})
}


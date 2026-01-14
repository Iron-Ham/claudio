package view

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockTemplates returns a fixed set of templates for testing.
var mockTemplates = []Template{
	{Command: "test", Name: "Run Tests", Description: "Run the test suite", Suffix: ""},
	{Command: "docs", Name: "Add Documentation", Description: "Add or improve docs", Suffix: ""},
	{Command: "fix", Name: "Fix Bug", Description: "Fix the bug:\n\n", Suffix: ""},
	{Command: "plan", Name: "Create Plan", Description: "Create a plan:\n\n", Suffix: "\n\nPlan instructions..."},
}

// mockFilterFunc simulates the FilterTemplates function.
func mockFilterFunc(filter string) []Template {
	if filter == "" {
		return mockTemplates
	}
	var matches []Template
	for _, t := range mockTemplates {
		if contains(t.Command, filter) || contains(t.Name, filter) {
			matches = append(matches, t)
		}
	}
	return matches
}

// contains checks if s contains substr (case-sensitive for test simplicity).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestTemplateDropdownHandler_HandleKey_Escape(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "te",
		TemplateSelected: 1,
		TaskInput:        "/te",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected Escape to be handled")
	}
	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false after Escape")
	}
	if state.TemplateFilter != "" {
		t.Errorf("expected TemplateFilter to be empty, got %q", state.TemplateFilter)
	}
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to be 0, got %d", state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_HandleKey_Enter_SelectsTemplate(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 0, // "test" template
		TaskInput:        "/",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected Enter to be handled")
	}
	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false after selection")
	}
	if state.TaskInput != "Run the test suite" {
		t.Errorf("expected TaskInput to be template description, got %q", state.TaskInput)
	}
	if state.TaskInputCursor != len([]rune("Run the test suite")) {
		t.Errorf("expected cursor at end, got %d", state.TaskInputCursor)
	}
}

func TestTemplateDropdownHandler_HandleKey_Tab_SelectsTemplate(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 3, // "plan" template with suffix
		TaskInput:        "/",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyTab}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected Tab to be handled")
	}
	if state.TemplateSuffix != "\n\nPlan instructions..." {
		t.Errorf("expected suffix to be stored, got %q", state.TemplateSuffix)
	}
}

func TestTemplateDropdownHandler_HandleKey_Navigation(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 0,
		TaskInput:        "/",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	// Test Down navigation
	msg := tea.KeyMsg{Type: tea.KeyDown}
	handled, _ := handler.HandleKey(msg)
	if !handled {
		t.Error("expected Down to be handled")
	}
	if state.TemplateSelected != 1 {
		t.Errorf("expected TemplateSelected to be 1, got %d", state.TemplateSelected)
	}

	// Test Up navigation
	msg = tea.KeyMsg{Type: tea.KeyUp}
	handled, _ = handler.HandleKey(msg)
	if !handled {
		t.Error("expected Up to be handled")
	}
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to be 0, got %d", state.TemplateSelected)
	}

	// Test Up at boundary (should stay at 0)
	msg = tea.KeyMsg{Type: tea.KeyUp}
	handler.HandleKey(msg)
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to stay at 0, got %d", state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_HandleKey_Down_AtBoundary(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: len(mockTemplates) - 1, // Last item
		TaskInput:        "/",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyDown}
	handler.HandleKey(msg)

	// Should stay at last item
	if state.TemplateSelected != len(mockTemplates)-1 {
		t.Errorf("expected TemplateSelected to stay at %d, got %d", len(mockTemplates)-1, state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_HandleKey_Backspace_WithFilter(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "te",
		TemplateSelected: 1,
		TaskInput:        "/te",
		TaskInputCursor:  3,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected Backspace to be handled")
	}
	if state.TemplateFilter != "t" {
		t.Errorf("expected TemplateFilter to be 't', got %q", state.TemplateFilter)
	}
	if state.TaskInput != "/t" {
		t.Errorf("expected TaskInput to be '/t', got %q", state.TaskInput)
	}
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to reset to 0, got %d", state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_HandleKey_Backspace_EmptyFilter(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 0,
		TaskInput:        "/",
		TaskInputCursor:  1,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected Backspace to be handled")
	}
	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false when backspace removes '/'")
	}
	if state.TaskInput != "" {
		t.Errorf("expected TaskInput to be empty, got %q", state.TaskInput)
	}
}

func TestTemplateDropdownHandler_HandleKey_Runes_FilterText(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 1,
		TaskInput:        "/",
		TaskInputCursor:  1,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected rune input to be handled")
	}
	if state.TemplateFilter != "t" {
		t.Errorf("expected TemplateFilter to be 't', got %q", state.TemplateFilter)
	}
	if state.TaskInput != "/t" {
		t.Errorf("expected TaskInput to be '/t', got %q", state.TaskInput)
	}
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to reset to 0, got %d", state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_HandleKey_Space_ClosesDropdown(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "te",
		TemplateSelected: 1,
		TaskInput:        "/te",
		TaskInputCursor:  3,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected space to be handled")
	}
	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false after space")
	}
	if state.TaskInput != "/te " {
		t.Errorf("expected TaskInput to have space added, got %q", state.TaskInput)
	}
	if state.TemplateFilter != "" {
		t.Errorf("expected TemplateFilter to be empty, got %q", state.TemplateFilter)
	}
}

func TestTemplateDropdownHandler_HandleKey_NoMatchClosesDropdown(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 0,
		TaskInput:        "/",
		TaskInputCursor:  1,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	// Type characters that don't match any template
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
	handler.HandleKey(msg)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
	handler.HandleKey(msg)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}}
	handler.HandleKey(msg)

	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false when no templates match")
	}
}

func TestTemplateDropdownHandler_HandleKey_Enter_AfterNewline(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 0, // "test" template
		TaskInput:        "Some text\n/",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	handler.HandleKey(msg)

	expected := "Some text\nRun the test suite"
	if state.TaskInput != expected {
		t.Errorf("expected TaskInput to be %q, got %q", expected, state.TaskInput)
	}
}

func TestTemplateDropdownHandler_OpenDropdown(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    false,
		TemplateFilter:   "old",
		TemplateSelected: 5,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	handler.OpenDropdown()

	if !state.ShowTemplates {
		t.Error("expected ShowTemplates to be true")
	}
	if state.TemplateFilter != "" {
		t.Errorf("expected TemplateFilter to be empty, got %q", state.TemplateFilter)
	}
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to be 0, got %d", state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_CloseDropdown(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "test",
		TemplateSelected: 2,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	handler.CloseDropdown()

	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false")
	}
	if state.TemplateFilter != "" {
		t.Errorf("expected TemplateFilter to be empty, got %q", state.TemplateFilter)
	}
	if state.TemplateSelected != 0 {
		t.Errorf("expected TemplateSelected to be 0, got %d", state.TemplateSelected)
	}
}

func TestTemplateDropdownHandler_SuffixMethods(t *testing.T) {
	state := &TemplateDropdownState{
		TemplateSuffix: "test suffix",
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	if handler.GetSuffix() != "test suffix" {
		t.Errorf("expected GetSuffix to return 'test suffix', got %q", handler.GetSuffix())
	}

	handler.ClearSuffix()
	if state.TemplateSuffix != "" {
		t.Error("expected TemplateSuffix to be empty after ClearSuffix")
	}
}

func TestTemplateDropdownHandler_IsOpen(t *testing.T) {
	state := &TemplateDropdownState{ShowTemplates: false}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	if handler.IsOpen() {
		t.Error("expected IsOpen to return false")
	}

	state.ShowTemplates = true
	if !handler.IsOpen() {
		t.Error("expected IsOpen to return true")
	}
}

func TestBuildTemplateItems(t *testing.T) {
	templates := []Template{
		{Command: "test", Name: "Run Tests", Description: "desc1", Suffix: "suf1"},
		{Command: "docs", Name: "Add Docs", Description: "desc2", Suffix: ""},
	}

	items := BuildTemplateItems(templates)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Command != "test" || items[0].Name != "Run Tests" {
		t.Errorf("unexpected first item: %+v", items[0])
	}
	if items[1].Command != "docs" || items[1].Name != "Add Docs" {
		t.Errorf("unexpected second item: %+v", items[1])
	}
}

func TestBuildTemplateItems_Empty(t *testing.T) {
	items := BuildTemplateItems(nil)
	if len(items) != 0 {
		t.Errorf("expected 0 items for nil input, got %d", len(items))
	}

	items = BuildTemplateItems([]Template{})
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty input, got %d", len(items))
	}
}

func TestTemplateDropdownHandler_HandleKey_UnhandledKey(t *testing.T) {
	state := &TemplateDropdownState{
		ShowTemplates: true,
	}
	handler := NewTemplateDropdownHandler(state, mockFilterFunc)

	// F1 key should not be handled
	msg := tea.KeyMsg{Type: tea.KeyF1}
	handled, _ := handler.HandleKey(msg)

	if handled {
		t.Error("expected F1 key to not be handled")
	}
}

func TestTemplateDropdownHandler_HandleKey_Enter_EmptyTemplates(t *testing.T) {
	// Filter that matches nothing
	emptyFilter := func(filter string) []Template {
		return nil
	}

	state := &TemplateDropdownState{
		ShowTemplates:    true,
		TemplateFilter:   "",
		TemplateSelected: 0,
		TaskInput:        "/",
	}
	handler := NewTemplateDropdownHandler(state, emptyFilter)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	handled, _ := handler.HandleKey(msg)

	if !handled {
		t.Error("expected Enter to be handled even with no templates")
	}
	// TaskInput should remain unchanged since no template was selected
	if state.TaskInput != "/" {
		t.Errorf("expected TaskInput to remain '/', got %q", state.TaskInput)
	}
	if state.ShowTemplates {
		t.Error("expected ShowTemplates to be false")
	}
}

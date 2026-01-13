package input

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMode_String(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected string
	}{
		{ModeNormal, "normal"},
		{ModeCommand, "command"},
		{ModeSearch, "search"},
		{ModeFilter, "filter"},
		{ModeInput, "input"},
		{ModeTerminal, "terminal"},
		{ModeTaskInput, "task-input"},
		{ModePlanEditor, "plan-editor"},
		{ModeUltraPlan, "ultra-plan"},
		{Mode(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.expected {
				t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestNewRouter(t *testing.T) {
	r := NewRouter()

	if r.Mode() != ModeNormal {
		t.Errorf("NewRouter().Mode() = %v, want ModeNormal", r.Mode())
	}

	if r.Buffer != "" {
		t.Errorf("NewRouter().Buffer = %q, want empty", r.Buffer)
	}
}

func TestRouter_SetMode(t *testing.T) {
	r := NewRouter()

	modes := []Mode{ModeCommand, ModeSearch, ModeFilter, ModeInput, ModeTerminal, ModeTaskInput, ModeNormal}

	for _, mode := range modes {
		r.SetMode(mode)
		if r.Mode() != mode {
			t.Errorf("after SetMode(%v), Mode() = %v", mode, r.Mode())
		}
	}
}

func TestRouter_Buffer(t *testing.T) {
	r := NewRouter()

	// Test AppendToBuffer
	r.AppendToBuffer("hello")
	if r.Buffer != "hello" {
		t.Errorf("after AppendToBuffer(\"hello\"), Buffer = %q, want \"hello\"", r.Buffer)
	}

	r.AppendToBuffer(" world")
	if r.Buffer != "hello world" {
		t.Errorf("after AppendToBuffer(\" world\"), Buffer = %q, want \"hello world\"", r.Buffer)
	}

	// Test DeleteFromBuffer
	deleted := r.DeleteFromBuffer()
	if !deleted {
		t.Error("DeleteFromBuffer() returned false, want true")
	}
	if r.Buffer != "hello worl" {
		t.Errorf("after DeleteFromBuffer(), Buffer = %q, want \"hello worl\"", r.Buffer)
	}

	// Test ClearBuffer
	r.ClearBuffer()
	if r.Buffer != "" {
		t.Errorf("after ClearBuffer(), Buffer = %q, want empty", r.Buffer)
	}

	// Test DeleteFromBuffer on empty buffer
	deleted = r.DeleteFromBuffer()
	if deleted {
		t.Error("DeleteFromBuffer() on empty buffer returned true, want false")
	}
}

func TestRouter_RegisterHandler(t *testing.T) {
	r := NewRouter()
	called := false

	// Register handler using interface
	handler := HandlerFunc(func(msg tea.KeyMsg) Result {
		called = true
		return NewResult()
	})
	r.RegisterHandler(ModeCommand, handler)

	// Set mode and route
	r.SetMode(ModeCommand)
	result := r.Route(tea.KeyMsg{Type: tea.KeyEnter})

	if !called {
		t.Error("Handler was not called")
	}
	if !result.Handled {
		t.Error("Result.Handled = false, want true")
	}
}

func TestRouter_RegisterHandlerFunc(t *testing.T) {
	r := NewRouter()
	called := false

	r.RegisterHandlerFunc(ModeSearch, func(msg tea.KeyMsg) Result {
		called = true
		return NewResult()
	})

	r.SetMode(ModeSearch)
	r.Route(tea.KeyMsg{Type: tea.KeyEnter})

	if !called {
		t.Error("Handler function was not called")
	}
}

func TestRouter_Route_NoHandler(t *testing.T) {
	r := NewRouter()
	r.SetMode(ModeCommand) // No handler registered

	result := r.Route(tea.KeyMsg{Type: tea.KeyEnter})

	if result.Handled {
		t.Error("Route without handler returned Handled=true, want false")
	}
}

func TestRouter_Route_ModeTransition(t *testing.T) {
	r := NewRouter()

	r.RegisterHandlerFunc(ModeNormal, func(msg tea.KeyMsg) Result {
		if msg.String() == ":" {
			return NewResult().WithModeChange(ModeCommand)
		}
		return NotHandled()
	})

	result := r.Route(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})

	if r.Mode() != ModeCommand {
		t.Errorf("after routing ':', Mode() = %v, want ModeCommand", r.Mode())
	}
	if !result.Handled {
		t.Error("Result.Handled = false, want true")
	}
}

func TestRouter_Route_BufferClear(t *testing.T) {
	r := NewRouter()
	r.Buffer = "test content"

	r.RegisterHandlerFunc(ModeCommand, func(msg tea.KeyMsg) Result {
		if msg.Type == tea.KeyEsc {
			return NewResult().WithBufferClear().WithModeChange(ModeNormal)
		}
		return NotHandled()
	})

	r.SetMode(ModeCommand)
	r.Route(tea.KeyMsg{Type: tea.KeyEsc})

	if r.Buffer != "" {
		t.Errorf("after Esc with BufferClear, Buffer = %q, want empty", r.Buffer)
	}
	if r.Mode() != ModeNormal {
		t.Errorf("after Esc, Mode() = %v, want ModeNormal", r.Mode())
	}
}

func TestResult_Builder(t *testing.T) {
	// Test NewResult
	r := NewResult()
	if !r.Handled {
		t.Error("NewResult().Handled = false, want true")
	}

	// Test WithCmd
	cmd := func() tea.Msg { return nil }
	r = r.WithCmd(cmd)
	if r.Cmd == nil {
		t.Error("WithCmd() did not set Cmd")
	}

	// Test WithModeChange
	r = NewResult().WithModeChange(ModeSearch)
	if r.NextMode == nil || *r.NextMode != ModeSearch {
		t.Errorf("WithModeChange(ModeSearch): NextMode = %v, want ModeSearch", r.NextMode)
	}

	// Test WithBufferClear
	r = NewResult().WithBufferClear()
	if !r.ClearBuffer {
		t.Error("WithBufferClear() did not set ClearBuffer")
	}

	// Test NotHandled
	r = NotHandled()
	if r.Handled {
		t.Error("NotHandled().Handled = true, want false")
	}
}

func TestRouter_Transitions(t *testing.T) {
	r := NewRouter()

	transitions := []struct {
		name     string
		fn       func()
		expected Mode
	}{
		{"TransitionToCommand", r.TransitionToCommand, ModeCommand},
		{"TransitionToSearch", r.TransitionToSearch, ModeSearch},
		{"TransitionToFilter", r.TransitionToFilter, ModeFilter},
		{"TransitionToInput", r.TransitionToInput, ModeInput},
		{"TransitionToTerminal", r.TransitionToTerminal, ModeTerminal},
		{"TransitionToTaskInput", r.TransitionToTaskInput, ModeTaskInput},
		{"TransitionToNormal", r.TransitionToNormal, ModeNormal},
	}

	for _, tt := range transitions {
		t.Run(tt.name, func(t *testing.T) {
			// Set some buffer content first
			r.Buffer = "test"
			tt.fn()

			if r.Mode() != tt.expected {
				t.Errorf("%s: Mode() = %v, want %v", tt.name, r.Mode(), tt.expected)
			}
		})
	}
}

func TestRouter_TransitionClearsBuffer(t *testing.T) {
	tests := []struct {
		name         string
		transition   func(*Router)
		clearsBuffer bool
	}{
		{"TransitionToNormal", (*Router).TransitionToNormal, true},
		{"TransitionToCommand", (*Router).TransitionToCommand, true},
		{"TransitionToSearch", (*Router).TransitionToSearch, true},
		{"TransitionToTaskInput", (*Router).TransitionToTaskInput, true},
		{"TransitionToFilter", (*Router).TransitionToFilter, false},
		{"TransitionToInput", (*Router).TransitionToInput, false},
		{"TransitionToTerminal", (*Router).TransitionToTerminal, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRouter()
			r.Buffer = "test content"
			tt.transition(r)

			if tt.clearsBuffer && r.Buffer != "" {
				t.Errorf("%s should clear buffer, but Buffer = %q", tt.name, r.Buffer)
			}
			if !tt.clearsBuffer && r.Buffer == "" {
				t.Errorf("%s should not clear buffer, but buffer was cleared", tt.name)
			}
		})
	}
}

func TestRouter_ShouldExitModeOnEscape(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected bool
	}{
		{ModeNormal, false},
		{ModeCommand, true},
		{ModeSearch, true},
		{ModeFilter, true},
		{ModeInput, false},
		{ModeTerminal, false},
		{ModeTaskInput, true},
		{ModePlanEditor, false},
		{ModeUltraPlan, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			r := NewRouter()
			r.SetMode(tt.mode)

			got := r.ShouldExitModeOnEscape()
			if got != tt.expected {
				t.Errorf("ShouldExitModeOnEscape() in %v = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestRouter_ShouldExitModeOnCtrlBracket(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected bool
	}{
		{ModeNormal, false},
		{ModeCommand, false},
		{ModeSearch, false},
		{ModeFilter, false},
		{ModeInput, true},
		{ModeTerminal, true},
		{ModeTaskInput, false},
		{ModePlanEditor, false},
		{ModeUltraPlan, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			r := NewRouter()
			r.SetMode(tt.mode)

			got := r.ShouldExitModeOnCtrlBracket()
			if got != tt.expected {
				t.Errorf("ShouldExitModeOnCtrlBracket() in %v = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestRouter_IsBufferedMode(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected bool
	}{
		{ModeNormal, false},
		{ModeCommand, true},
		{ModeSearch, true},
		{ModeFilter, true},
		{ModeInput, false},
		{ModeTerminal, false},
		{ModeTaskInput, true},
		{ModePlanEditor, false},
		{ModeUltraPlan, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			r := NewRouter()
			r.SetMode(tt.mode)

			got := r.IsBufferedMode()
			if got != tt.expected {
				t.Errorf("IsBufferedMode() in %v = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestRouter_IsForwardingMode(t *testing.T) {
	tests := []struct {
		mode     Mode
		expected bool
	}{
		{ModeNormal, false},
		{ModeCommand, false},
		{ModeSearch, false},
		{ModeFilter, false},
		{ModeInput, true},
		{ModeTerminal, true},
		{ModeTaskInput, false},
		{ModePlanEditor, false},
		{ModeUltraPlan, false},
	}

	for _, tt := range tests {
		t.Run(tt.mode.String(), func(t *testing.T) {
			r := NewRouter()
			r.SetMode(tt.mode)

			got := r.IsForwardingMode()
			if got != tt.expected {
				t.Errorf("IsForwardingMode() in %v = %v, want %v", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestRouter_UltraPlanActive(t *testing.T) {
	r := NewRouter()

	if r.IsUltraPlanActive() {
		t.Error("Initial IsUltraPlanActive() = true, want false")
	}

	r.SetUltraPlanActive(true)
	if !r.IsUltraPlanActive() {
		t.Error("After SetUltraPlanActive(true), IsUltraPlanActive() = false")
	}

	r.SetUltraPlanActive(false)
	if r.IsUltraPlanActive() {
		t.Error("After SetUltraPlanActive(false), IsUltraPlanActive() = true")
	}
}

func TestRouter_PlanEditorActive(t *testing.T) {
	r := NewRouter()

	if r.IsPlanEditorActive() {
		t.Error("Initial IsPlanEditorActive() = true, want false")
	}

	r.SetPlanEditorActive(true)
	if !r.IsPlanEditorActive() {
		t.Error("After SetPlanEditorActive(true), IsPlanEditorActive() = false")
	}

	r.SetPlanEditorActive(false)
	if r.IsPlanEditorActive() {
		t.Error("After SetPlanEditorActive(false), IsPlanEditorActive() = true")
	}
}

func TestRouter_TemplateDropdown(t *testing.T) {
	r := NewRouter()

	if r.IsTemplateDropdownVisible() {
		t.Error("Initial IsTemplateDropdownVisible() = true, want false")
	}

	r.SetTemplateDropdown(true)
	if !r.IsTemplateDropdownVisible() {
		t.Error("After SetTemplateDropdown(true), IsTemplateDropdownVisible() = false")
	}

	r.SetTemplateDropdown(false)
	if r.IsTemplateDropdownVisible() {
		t.Error("After SetTemplateDropdown(false), IsTemplateDropdownVisible() = true")
	}
}

func TestRouter_GroupDecisionMode(t *testing.T) {
	r := NewRouter()

	if r.IsGroupDecisionMode() {
		t.Error("Initial IsGroupDecisionMode() = true, want false")
	}

	r.SetGroupDecisionMode(true)
	if !r.IsGroupDecisionMode() {
		t.Error("After SetGroupDecisionMode(true), IsGroupDecisionMode() = false")
	}

	r.SetGroupDecisionMode(false)
	if r.IsGroupDecisionMode() {
		t.Error("After SetGroupDecisionMode(false), IsGroupDecisionMode() = true")
	}
}

func TestRouter_RetriggerMode(t *testing.T) {
	r := NewRouter()

	if r.IsRetriggerMode() {
		t.Error("Initial IsRetriggerMode() = true, want false")
	}

	r.SetRetriggerMode(true)
	if !r.IsRetriggerMode() {
		t.Error("After SetRetriggerMode(true), IsRetriggerMode() = false")
	}

	r.SetRetriggerMode(false)
	if r.IsRetriggerMode() {
		t.Error("After SetRetriggerMode(false), IsRetriggerMode() = true")
	}
}

func TestRouter_EffectiveMode_Priority(t *testing.T) {
	r := NewRouter()

	// Register handlers for modes we want to test
	calledMode := ModeNormal
	makeHandler := func(mode Mode) Handler {
		return HandlerFunc(func(msg tea.KeyMsg) Result {
			calledMode = mode
			return NewResult()
		})
	}

	r.RegisterHandler(ModeNormal, makeHandler(ModeNormal))
	r.RegisterHandler(ModeSearch, makeHandler(ModeSearch))
	r.RegisterHandler(ModeFilter, makeHandler(ModeFilter))
	r.RegisterHandler(ModeInput, makeHandler(ModeInput))
	r.RegisterHandler(ModeTerminal, makeHandler(ModeTerminal))
	r.RegisterHandler(ModeTaskInput, makeHandler(ModeTaskInput))
	r.RegisterHandler(ModeCommand, makeHandler(ModeCommand))
	r.RegisterHandler(ModePlanEditor, makeHandler(ModePlanEditor))
	r.RegisterHandler(ModeUltraPlan, makeHandler(ModeUltraPlan))

	msg := tea.KeyMsg{Type: tea.KeyEnter}

	// Test search mode has high priority
	r.SetMode(ModeSearch)
	r.SetUltraPlanActive(true) // Should not affect search mode
	r.Route(msg)
	if calledMode != ModeSearch {
		t.Errorf("Search mode: called %v, want ModeSearch", calledMode)
	}

	// Test filter mode
	r.SetMode(ModeFilter)
	r.Route(msg)
	if calledMode != ModeFilter {
		t.Errorf("Filter mode: called %v, want ModeFilter", calledMode)
	}

	// Test input mode
	r.SetMode(ModeInput)
	r.Route(msg)
	if calledMode != ModeInput {
		t.Errorf("Input mode: called %v, want ModeInput", calledMode)
	}

	// Test terminal mode
	r.SetMode(ModeTerminal)
	r.Route(msg)
	if calledMode != ModeTerminal {
		t.Errorf("Terminal mode: called %v, want ModeTerminal", calledMode)
	}

	// Test task input mode
	r.SetMode(ModeTaskInput)
	r.Route(msg)
	if calledMode != ModeTaskInput {
		t.Errorf("Task input mode: called %v, want ModeTaskInput", calledMode)
	}

	// Test command mode
	r.SetMode(ModeCommand)
	r.Route(msg)
	if calledMode != ModeCommand {
		t.Errorf("Command mode: called %v, want ModeCommand", calledMode)
	}

	// Test normal mode with plan editor active
	r.SetMode(ModeNormal)
	r.SetUltraPlanActive(false)
	r.SetPlanEditorActive(true)
	r.Route(msg)
	if calledMode != ModePlanEditor {
		t.Errorf("Normal mode with plan editor: called %v, want ModePlanEditor", calledMode)
	}

	// Test normal mode with ultra-plan active (but not plan editor)
	r.SetPlanEditorActive(false)
	r.SetUltraPlanActive(true)
	r.Route(msg)
	if calledMode != ModeUltraPlan {
		t.Errorf("Normal mode with ultra-plan: called %v, want ModeUltraPlan", calledMode)
	}

	// Test pure normal mode
	r.SetUltraPlanActive(false)
	r.Route(msg)
	if calledMode != ModeNormal {
		t.Errorf("Pure normal mode: called %v, want ModeNormal", calledMode)
	}
}

func TestRouter_Route_WithCmd(t *testing.T) {
	r := NewRouter()

	expectedMsg := "test message"
	r.RegisterHandlerFunc(ModeNormal, func(msg tea.KeyMsg) Result {
		cmd := func() tea.Msg { return expectedMsg }
		return NewResult().WithCmd(cmd)
	})

	result := r.Route(tea.KeyMsg{Type: tea.KeyEnter})

	if result.Cmd == nil {
		t.Fatal("Result.Cmd is nil, want a command")
	}

	// Execute the command and check the result
	msg := result.Cmd()
	if msg != expectedMsg {
		t.Errorf("Cmd() = %v, want %q", msg, expectedMsg)
	}
}

func TestHandlerFunc(t *testing.T) {
	called := false
	var receivedMsg tea.KeyMsg

	f := HandlerFunc(func(msg tea.KeyMsg) Result {
		called = true
		receivedMsg = msg
		return NewResult()
	})

	testMsg := tea.KeyMsg{Type: tea.KeyEnter}
	result := f.HandleKey(testMsg)

	if !called {
		t.Error("HandlerFunc was not called")
	}
	if receivedMsg.Type != testMsg.Type {
		t.Errorf("HandlerFunc received msg.Type=%v, want %v", receivedMsg.Type, testMsg.Type)
	}
	if !result.Handled {
		t.Error("HandlerFunc returned Handled=false, want true")
	}
}

func TestRouter_GroupCommandPending(t *testing.T) {
	r := NewRouter()

	if r.IsGroupCommandPending() {
		t.Error("Initial IsGroupCommandPending() = true, want false")
	}

	r.SetGroupCommandPending(true)
	if !r.IsGroupCommandPending() {
		t.Error("After SetGroupCommandPending(true), IsGroupCommandPending() = false")
	}

	r.SetGroupCommandPending(false)
	if r.IsGroupCommandPending() {
		t.Error("After SetGroupCommandPending(false), IsGroupCommandPending() = true")
	}
}

func TestRouter_GroupedViewActive(t *testing.T) {
	r := NewRouter()

	if r.IsGroupedViewActive() {
		t.Error("Initial IsGroupedViewActive() = true, want false")
	}

	r.SetGroupedViewActive(true)
	if !r.IsGroupedViewActive() {
		t.Error("After SetGroupedViewActive(true), IsGroupedViewActive() = false")
	}

	r.SetGroupedViewActive(false)
	if r.IsGroupedViewActive() {
		t.Error("After SetGroupedViewActive(false), IsGroupedViewActive() = true")
	}
}

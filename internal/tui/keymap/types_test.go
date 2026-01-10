package keymap

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyBindingMatches(t *testing.T) {
	tests := []struct {
		name     string
		binding  KeyBinding
		msg      tea.KeyMsg
		expected bool
	}{
		{
			name: "simple rune match",
			binding: KeyBinding{
				KeyType: tea.KeyRunes,
				Rune:    'j',
			},
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'j'},
			},
			expected: true,
		},
		{
			name: "simple rune mismatch",
			binding: KeyBinding{
				KeyType: tea.KeyRunes,
				Rune:    'j',
			},
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'k'},
			},
			expected: false,
		},
		{
			name: "special key match",
			binding: KeyBinding{
				KeyType: tea.KeyEnter,
			},
			msg: tea.KeyMsg{
				Type: tea.KeyEnter,
			},
			expected: true,
		},
		{
			name: "special key mismatch",
			binding: KeyBinding{
				KeyType: tea.KeyEnter,
			},
			msg: tea.KeyMsg{
				Type: tea.KeyEsc,
			},
			expected: false,
		},
		{
			name: "alt modifier match",
			binding: KeyBinding{
				KeyType:   tea.KeyRunes,
				Rune:      'x',
				Modifiers: ModAlt,
			},
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'x'},
				Alt:   true,
			},
			expected: true,
		},
		{
			name: "alt modifier mismatch - binding wants alt",
			binding: KeyBinding{
				KeyType:   tea.KeyRunes,
				Rune:      'x',
				Modifiers: ModAlt,
			},
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'x'},
				Alt:   false,
			},
			expected: false,
		},
		{
			name: "alt modifier mismatch - binding doesn't want alt",
			binding: KeyBinding{
				KeyType: tea.KeyRunes,
				Rune:    'x',
			},
			msg: tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{'x'},
				Alt:   true,
			},
			expected: false,
		},
		{
			name: "ctrl key type",
			binding: KeyBinding{
				KeyType: tea.KeyCtrlR,
			},
			msg: tea.KeyMsg{
				Type: tea.KeyCtrlR,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.binding.Matches(tt.msg)
			if result != tt.expected {
				t.Errorf("KeyBinding.Matches() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestKeymapGetBinding(t *testing.T) {
	km := DefaultKeymap()

	// Test normal mode 'j' for scroll down
	msg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'j'},
	}
	cmd, found := km.GetBinding(msg, ModeNormal)
	if !found {
		t.Error("Expected to find binding for 'j' in normal mode")
	}
	if cmd != CmdScrollDown {
		t.Errorf("Expected CmdScrollDown, got %s", cmd)
	}

	// Test that 'j' in search mode doesn't match scroll (it should be InsertChar)
	cmd, found = km.GetBinding(msg, ModeSearch)
	if !found {
		t.Error("Expected to find binding for 'j' in search mode")
	}
	if cmd != CmdInsertChar {
		t.Errorf("Expected CmdInsertChar in search mode, got %s", cmd)
	}
}

func TestModifiersString(t *testing.T) {
	tests := []struct {
		mods     Modifier
		expected string
	}{
		{ModNone, ""},
		{ModCtrl, "ctrl+"},
		{ModAlt, "alt+"},
		{ModShift, "shift+"},
		{ModCtrl | ModAlt, "ctrl+alt+"},
		{ModCtrl | ModShift, "ctrl+shift+"},
		{ModAlt | ModShift, "alt+shift+"},
		{ModCtrl | ModAlt | ModShift, "ctrl+alt+shift+"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.mods.String()
			if result != tt.expected {
				t.Errorf("Modifier.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestKeyBindingString(t *testing.T) {
	tests := []struct {
		binding  KeyBinding
		expected string
	}{
		{
			binding:  KeyBinding{KeyType: tea.KeyEnter},
			expected: "enter",
		},
		{
			binding:  KeyBinding{KeyType: tea.KeyRunes, Rune: 'j'},
			expected: "j",
		},
		{
			binding:  KeyBinding{KeyType: tea.KeyRunes, Rune: ' '},
			expected: "space",
		},
		{
			binding:  KeyBinding{KeyType: tea.KeyRunes, Rune: 'x', Modifiers: ModAlt},
			expected: "alt+x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.binding.String()
			if result != tt.expected {
				t.Errorf("KeyBinding.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestParseKeySpec(t *testing.T) {
	tests := []struct {
		spec        string
		wantKeyType tea.KeyType
		wantRune    rune
		wantMods    Modifier
		wantErr     bool
	}{
		{"enter", tea.KeyEnter, 0, ModNone, false},
		{"esc", tea.KeyEsc, 0, ModNone, false},
		{"escape", tea.KeyEsc, 0, ModNone, false},
		{"tab", tea.KeyTab, 0, ModNone, false},
		{"shift+tab", tea.KeyShiftTab, 0, ModNone, false},
		{"j", tea.KeyRunes, 'j', ModNone, false},
		{"ctrl+r", tea.KeyCtrlR, 0, ModNone, false},
		{"ctrl+a", tea.KeyCtrlA, 0, ModNone, false},
		{"alt+x", tea.KeyRunes, 'x', ModAlt, false},
		{"f1", tea.KeyF1, 0, ModNone, false},
		{"f12", tea.KeyF12, 0, ModNone, false},
	}

	for _, tt := range tests {
		t.Run(tt.spec, func(t *testing.T) {
			keyType, r, mods, err := ParseKeySpec(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseKeySpec(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
				return
			}
			if keyType != tt.wantKeyType {
				t.Errorf("ParseKeySpec(%q) keyType = %v, want %v", tt.spec, keyType, tt.wantKeyType)
			}
			if r != tt.wantRune {
				t.Errorf("ParseKeySpec(%q) rune = %q, want %q", tt.spec, r, tt.wantRune)
			}
			if mods != tt.wantMods {
				t.Errorf("ParseKeySpec(%q) mods = %v, want %v", tt.spec, mods, tt.wantMods)
			}
		})
	}
}

func TestLookupExCommand(t *testing.T) {
	tests := []struct {
		cmd      string
		expected Command
		found    bool
	}{
		{"s", CmdExStart, true},
		{"start", CmdExStart, true},
		{"q", CmdExQuit, true},
		{"quit", CmdExQuit, true},
		{"notacommand", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			cmd, found := LookupExCommand(tt.cmd)
			if found != tt.found {
				t.Errorf("LookupExCommand(%q) found = %v, want %v", tt.cmd, found, tt.found)
			}
			if cmd != tt.expected {
				t.Errorf("LookupExCommand(%q) = %v, want %v", tt.cmd, cmd, tt.expected)
			}
		})
	}
}

func TestGetBindingsForCommand(t *testing.T) {
	km := DefaultKeymap()

	// CmdScrollDown should have multiple bindings in normal mode (j and down arrow)
	bindings := km.GetBindingsForCommand(CmdScrollDown, ModeNormal)
	if len(bindings) < 2 {
		t.Errorf("Expected at least 2 bindings for CmdScrollDown, got %d", len(bindings))
	}

	// Check that we got 'j' and down arrow
	hasJ := false
	hasDown := false
	for _, b := range bindings {
		if b.KeyType == tea.KeyRunes && b.Rune == 'j' {
			hasJ = true
		}
		if b.KeyType == tea.KeyDown {
			hasDown = true
		}
	}
	if !hasJ {
		t.Error("Expected 'j' binding for CmdScrollDown")
	}
	if !hasDown {
		t.Error("Expected down arrow binding for CmdScrollDown")
	}
}

func TestGetCategories(t *testing.T) {
	km := DefaultKeymap()

	categories := km.GetCategories(ModeNormal)
	if len(categories) == 0 {
		t.Error("Expected at least one category in normal mode")
	}

	// Check for expected categories
	categorySet := make(map[string]bool)
	for _, cat := range categories {
		categorySet[cat] = true
	}

	expectedCategories := []string{"Navigation", "Scrolling", "Search", "Modes"}
	for _, expected := range expectedCategories {
		if !categorySet[expected] {
			t.Errorf("Expected category %q in normal mode", expected)
		}
	}
}

func TestDefaultKeymapCompleteness(t *testing.T) {
	km := DefaultKeymap()

	// Verify all expected modes are present
	expectedModes := []Mode{
		ModeNormal,
		ModeCommand,
		ModeSearch,
		ModeFilter,
		ModeAddTask,
		ModeTemplate,
		ModeInput,
		ModeUltraPlan,
		ModePlanEditor,
	}

	for _, mode := range expectedModes {
		if _, ok := km.Modes[mode]; !ok {
			t.Errorf("Default keymap missing mode: %s", mode)
		}
	}

	// Verify normal mode has essential bindings
	normalBindings := km.GetModeBindings(ModeNormal)
	if len(normalBindings) < 20 {
		t.Errorf("Normal mode seems incomplete, only %d bindings", len(normalBindings))
	}
}

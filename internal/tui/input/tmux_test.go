package input

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// mockKeySender records key sends for testing.
type mockKeySender struct {
	keys     []string
	literals []string
}

func (m *mockKeySender) SendKey(key string) {
	m.keys = append(m.keys, key)
}

func (m *mockKeySender) SendLiteral(text string) {
	m.literals = append(m.literals, text)
}

func TestSendKeyToTmux_BasicKeys(t *testing.T) {
	tests := []struct {
		name          string
		keyType       tea.KeyType
		expectedKey   string
		expectLiteral bool
	}{
		{"enter", tea.KeyEnter, "Enter", false},
		{"backspace", tea.KeyBackspace, "BSpace", false},
		{"tab", tea.KeyTab, "Tab", false},
		{"shift-tab", tea.KeyShiftTab, "BTab", false},
		{"space", tea.KeySpace, " ", true},
		{"escape", tea.KeyEsc, "Escape", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tt.keyType}
			SendKeyToTmux(sender, msg)

			if tt.expectLiteral {
				if len(sender.literals) != 1 || sender.literals[0] != tt.expectedKey {
					t.Errorf("SendKeyToTmux() sent literal = %v, want %v", sender.literals, []string{tt.expectedKey})
				}
				if len(sender.keys) != 0 {
					t.Errorf("SendKeyToTmux() unexpectedly sent keys = %v", sender.keys)
				}
			} else {
				if len(sender.keys) != 1 || sender.keys[0] != tt.expectedKey {
					t.Errorf("SendKeyToTmux() sent key = %v, want %v", sender.keys, []string{tt.expectedKey})
				}
				if len(sender.literals) != 0 {
					t.Errorf("SendKeyToTmux() unexpectedly sent literals = %v", sender.literals)
				}
			}
		})
	}
}

func TestSendKeyToTmux_ArrowKeys(t *testing.T) {
	tests := []struct {
		name        string
		keyType     tea.KeyType
		expectedKey string
	}{
		{"up", tea.KeyUp, "Up"},
		{"down", tea.KeyDown, "Down"},
		{"right", tea.KeyRight, "Right"},
		{"left", tea.KeyLeft, "Left"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tt.keyType}
			SendKeyToTmux(sender, msg)

			if len(sender.keys) != 1 || sender.keys[0] != tt.expectedKey {
				t.Errorf("SendKeyToTmux() sent key = %v, want %v", sender.keys, []string{tt.expectedKey})
			}
		})
	}
}

func TestSendKeyToTmux_NavigationKeys(t *testing.T) {
	tests := []struct {
		name        string
		keyType     tea.KeyType
		expectedKey string
	}{
		{"page-up", tea.KeyPgUp, "PageUp"},
		{"page-down", tea.KeyPgDown, "PageDown"},
		{"home", tea.KeyHome, "Home"},
		{"end", tea.KeyEnd, "End"},
		{"delete", tea.KeyDelete, "DC"},
		{"insert", tea.KeyInsert, "IC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tt.keyType}
			SendKeyToTmux(sender, msg)

			if len(sender.keys) != 1 || sender.keys[0] != tt.expectedKey {
				t.Errorf("SendKeyToTmux() sent key = %v, want %v", sender.keys, []string{tt.expectedKey})
			}
		})
	}
}

func TestSendKeyToTmux_CtrlKeys(t *testing.T) {
	tests := []struct {
		name        string
		keyType     tea.KeyType
		expectedKey string
	}{
		{"ctrl-a", tea.KeyCtrlA, "C-a"},
		{"ctrl-b", tea.KeyCtrlB, "C-b"},
		{"ctrl-c", tea.KeyCtrlC, "C-c"},
		{"ctrl-d", tea.KeyCtrlD, "C-d"},
		{"ctrl-e", tea.KeyCtrlE, "C-e"},
		{"ctrl-f", tea.KeyCtrlF, "C-f"},
		{"ctrl-g", tea.KeyCtrlG, "C-g"},
		{"ctrl-h", tea.KeyCtrlH, "C-h"},
		{"ctrl-j", tea.KeyCtrlJ, "C-j"},
		{"ctrl-k", tea.KeyCtrlK, "C-k"},
		{"ctrl-l", tea.KeyCtrlL, "C-l"},
		{"ctrl-n", tea.KeyCtrlN, "C-n"},
		{"ctrl-o", tea.KeyCtrlO, "C-o"},
		{"ctrl-p", tea.KeyCtrlP, "C-p"},
		{"ctrl-q", tea.KeyCtrlQ, "C-q"},
		{"ctrl-r", tea.KeyCtrlR, "C-r"},
		{"ctrl-s", tea.KeyCtrlS, "C-s"},
		{"ctrl-t", tea.KeyCtrlT, "C-t"},
		{"ctrl-u", tea.KeyCtrlU, "C-u"},
		{"ctrl-v", tea.KeyCtrlV, "C-v"},
		{"ctrl-w", tea.KeyCtrlW, "C-w"},
		{"ctrl-x", tea.KeyCtrlX, "C-x"},
		{"ctrl-y", tea.KeyCtrlY, "C-y"},
		{"ctrl-z", tea.KeyCtrlZ, "C-z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tt.keyType}
			SendKeyToTmux(sender, msg)

			if len(sender.keys) != 1 || sender.keys[0] != tt.expectedKey {
				t.Errorf("SendKeyToTmux() sent key = %v, want %v", sender.keys, []string{tt.expectedKey})
			}
		})
	}
}

func TestSendKeyToTmux_FunctionKeys(t *testing.T) {
	tests := []struct {
		name        string
		keyType     tea.KeyType
		expectedKey string
	}{
		{"f1", tea.KeyF1, "F1"},
		{"f2", tea.KeyF2, "F2"},
		{"f3", tea.KeyF3, "F3"},
		{"f4", tea.KeyF4, "F4"},
		{"f5", tea.KeyF5, "F5"},
		{"f6", tea.KeyF6, "F6"},
		{"f7", tea.KeyF7, "F7"},
		{"f8", tea.KeyF8, "F8"},
		{"f9", tea.KeyF9, "F9"},
		{"f10", tea.KeyF10, "F10"},
		{"f11", tea.KeyF11, "F11"},
		{"f12", tea.KeyF12, "F12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tt.keyType}
			SendKeyToTmux(sender, msg)

			if len(sender.keys) != 1 || sender.keys[0] != tt.expectedKey {
				t.Errorf("SendKeyToTmux() sent key = %v, want %v", sender.keys, []string{tt.expectedKey})
			}
		})
	}
}

func TestSendKeyToTmux_Runes(t *testing.T) {
	tests := []struct {
		name            string
		runes           []rune
		expectedLiteral string
	}{
		{"single char", []rune{'a'}, "a"},
		{"uppercase", []rune{'A'}, "A"},
		{"digit", []rune{'5'}, "5"},
		{"special char", []rune{'@'}, "@"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: tt.runes}
			SendKeyToTmux(sender, msg)

			if len(sender.literals) != 1 || sender.literals[0] != tt.expectedLiteral {
				t.Errorf("SendKeyToTmux() sent literal = %v, want %v", sender.literals, []string{tt.expectedLiteral})
			}
			if len(sender.keys) != 0 {
				t.Errorf("SendKeyToTmux() unexpectedly sent keys = %v", sender.keys)
			}
		})
	}
}

func TestSendKeyToTmux_AltRunes(t *testing.T) {
	sender := &mockKeySender{}
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: true}
	SendKeyToTmux(sender, msg)

	// Alt+key sends M- prefixed key (Meta key in tmux)
	if len(sender.keys) != 1 || sender.keys[0] != "M-x" {
		t.Errorf("SendKeyToTmux() sent keys = %v, want [M-x]", sender.keys)
	}
	if len(sender.literals) != 0 {
		t.Errorf("SendKeyToTmux() unexpectedly sent literals = %v", sender.literals)
	}
}

func TestSendKeyToTmux_AltArrowKeys(t *testing.T) {
	tests := []struct {
		name        string
		keyType     tea.KeyType
		expectedKey string
	}{
		{"alt-up", tea.KeyUp, "M-Up"},
		{"alt-down", tea.KeyDown, "M-Down"},
		{"alt-left", tea.KeyLeft, "M-Left"},
		{"alt-right", tea.KeyRight, "M-Right"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sender := &mockKeySender{}
			msg := tea.KeyMsg{Type: tt.keyType, Alt: true}
			SendKeyToTmux(sender, msg)

			if len(sender.keys) != 1 || sender.keys[0] != tt.expectedKey {
				t.Errorf("SendKeyToTmux() sent keys = %v, want [%v]", sender.keys, tt.expectedKey)
			}
			if len(sender.literals) != 0 {
				t.Errorf("SendKeyToTmux() unexpectedly sent literals = %v", sender.literals)
			}
		})
	}
}

func TestSendKeyToTmux_AltBackspace(t *testing.T) {
	sender := &mockKeySender{}
	msg := tea.KeyMsg{Type: tea.KeyBackspace, Alt: true}
	SendKeyToTmux(sender, msg)

	// Alt+Backspace (Opt+Backspace on macOS) sends M-BSpace (Meta+Backspace in tmux)
	if len(sender.keys) != 1 || sender.keys[0] != "M-BSpace" {
		t.Errorf("SendKeyToTmux() sent keys = %v, want [M-BSpace]", sender.keys)
	}
	if len(sender.literals) != 0 {
		t.Errorf("SendKeyToTmux() unexpectedly sent literals = %v", sender.literals)
	}
}

func TestKeySender_Interface(t *testing.T) {
	// This test verifies that the KeySender interface is satisfied by the mock.
	var _ KeySender = (*mockKeySender)(nil)
}

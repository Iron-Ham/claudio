// Package input provides input routing for the TUI.
// This file handles the translation of Bubble Tea key events to tmux key codes.
package input

import (
	"strings"

	"github.com/Iron-Ham/claudio/internal/tmux"
	tea "github.com/charmbracelet/bubbletea"
)

// KeySender defines the interface for sending keys to a tmux session.
// This allows for dependency injection and easier testing.
type KeySender interface {
	// SendKey sends a named key (e.g., "Enter", "Up", "C-a") to the tmux session.
	SendKey(key string)
	// SendLiteral sends literal text to the tmux session.
	SendLiteral(text string)
}

// SendKeyToTmux translates a Bubble Tea key event to tmux key codes and sends
// it to the given KeySender. This handles the complex mapping between the two
// key representations including special keys, control sequences, and modifiers.
func SendKeyToTmux(sender KeySender, msg tea.KeyMsg) {
	var key string
	literal := false

	switch msg.Type {
	// Basic keys
	case tea.KeyEnter:
		key = "Enter"
	case tea.KeyBackspace:
		// Check for Alt modifier (Opt+Backspace on macOS)
		if msg.Alt {
			sender.SendKey("M-BSpace")
			return
		}
		key = "BSpace"
	case tea.KeyTab:
		key = "Tab"
	case tea.KeyShiftTab:
		key = "BTab" // Back-tab in tmux
	case tea.KeySpace:
		key = " " // Send literal space
		literal = true
	case tea.KeyEsc:
		key = "Escape"

	// Arrow keys - check for Alt modifier (Opt+Arrow on macOS)
	// Use M- prefix (Meta) for Alt combinations - tmux sends this as a single key event
	case tea.KeyUp:
		if msg.Alt {
			sender.SendKey("M-Up")
			return
		}
		key = "Up"
	case tea.KeyDown:
		if msg.Alt {
			sender.SendKey("M-Down")
			return
		}
		key = "Down"
	case tea.KeyRight:
		if msg.Alt {
			sender.SendKey("M-Right")
			return
		}
		key = "Right"
	case tea.KeyLeft:
		if msg.Alt {
			sender.SendKey("M-Left")
			return
		}
		key = "Left"

	// Navigation keys
	case tea.KeyPgUp:
		key = "PageUp"
	case tea.KeyPgDown:
		key = "PageDown"
	case tea.KeyHome:
		key = "Home"
	case tea.KeyEnd:
		key = "End"
	case tea.KeyDelete:
		key = "DC" // Delete character in tmux
	case tea.KeyInsert:
		key = "IC" // Insert character in tmux

	// All Ctrl+letter combinations (Claude Code uses many of these)
	case tea.KeyCtrlA:
		key = "C-a"
	case tea.KeyCtrlB:
		key = "C-b"
	case tea.KeyCtrlC:
		key = "C-c"
	case tea.KeyCtrlD:
		key = "C-d"
	case tea.KeyCtrlE:
		key = "C-e"
	case tea.KeyCtrlF:
		key = "C-f"
	case tea.KeyCtrlG:
		key = "C-g"
	case tea.KeyCtrlH:
		key = "C-h"
	// Note: KeyCtrlI (ASCII 9) is the same as KeyTab - handled above
	case tea.KeyCtrlJ:
		key = "C-j" // Note: also used for newline in some contexts
	case tea.KeyCtrlK:
		key = "C-k"
	case tea.KeyCtrlL:
		key = "C-l"
	// Note: KeyCtrlM (ASCII 13) is the same as KeyEnter - handled above
	case tea.KeyCtrlN:
		key = "C-n"
	case tea.KeyCtrlO:
		key = "C-o" // Used by Claude Code for file operations
	case tea.KeyCtrlP:
		key = "C-p"
	case tea.KeyCtrlQ:
		key = "C-q"
	case tea.KeyCtrlR:
		key = "C-r" // Used by Claude Code for reverse search
	case tea.KeyCtrlS:
		key = "C-s"
	case tea.KeyCtrlT:
		key = "C-t"
	case tea.KeyCtrlU:
		key = "C-u" // Used by Claude Code to clear line
	case tea.KeyCtrlV:
		key = "C-v"
	case tea.KeyCtrlW:
		key = "C-w" // Used by Claude Code to delete word
	case tea.KeyCtrlX:
		key = "C-x"
	case tea.KeyCtrlY:
		key = "C-y"
	case tea.KeyCtrlZ:
		key = "C-z"

	// Function keys (F1-F12)
	case tea.KeyF1:
		key = "F1"
	case tea.KeyF2:
		key = "F2"
	case tea.KeyF3:
		key = "F3"
	case tea.KeyF4:
		key = "F4"
	case tea.KeyF5:
		key = "F5"
	case tea.KeyF6:
		key = "F6"
	case tea.KeyF7:
		key = "F7"
	case tea.KeyF8:
		key = "F8"
	case tea.KeyF9:
		key = "F9"
	case tea.KeyF10:
		key = "F10"
	case tea.KeyF11:
		key = "F11"
	case tea.KeyF12:
		key = "F12"

	case tea.KeyRunes:
		// Send literal characters
		// Handle Alt+key combinations
		if msg.Alt {
			// For alt combinations, tmux uses M- prefix for Meta key
			key = string(msg.Runes)
			sender.SendKey("M-" + key)
			return
		}
		key = string(msg.Runes)
		literal = true

	default:
		// Try to handle other keys by their string representation
		keyStr := msg.String()

		// Handle known string patterns that might not have direct KeyType
		switch {
		case strings.HasPrefix(keyStr, "shift+"):
			// Try to map shift combinations
			baseKey := strings.TrimPrefix(keyStr, "shift+")
			switch baseKey {
			case "up":
				key = "S-Up"
			case "down":
				key = "S-Down"
			case "left":
				key = "S-Left"
			case "right":
				key = "S-Right"
			case "home":
				key = "S-Home"
			case "end":
				key = "S-End"
			default:
				key = keyStr
			}
		case strings.HasPrefix(keyStr, "alt+"):
			// Alt combinations: use M- prefix (Meta) for tmux
			baseKey := strings.TrimPrefix(keyStr, "alt+")
			// Map Bubble Tea key names to tmux key names and add M- prefix
			tmuxKey := tmux.MapKeyToTmux(baseKey)
			sender.SendKey("M-" + tmuxKey)
			return
		case strings.HasPrefix(keyStr, "ctrl+"):
			// Try to handle ctrl combinations not caught above
			baseKey := strings.TrimPrefix(keyStr, "ctrl+")
			if len(baseKey) == 1 {
				key = "C-" + baseKey
			} else {
				key = keyStr
			}
		default:
			key = keyStr
			if len(key) == 1 {
				literal = true
			}
		}
	}

	if key != "" {
		if literal {
			sender.SendLiteral(key)
		} else {
			sender.SendKey(key)
		}
	}
}

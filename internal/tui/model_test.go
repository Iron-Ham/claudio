package tui

import (
	"testing"
)

func TestInputHandlerInsert(t *testing.T) {
	tests := []struct {
		name           string
		initialInput   string
		initialCursor  int
		insertText     string
		expectedInput  string
		expectedCursor int
	}{
		{
			name:           "insert at beginning of empty string",
			initialInput:   "",
			initialCursor:  0,
			insertText:     "hello",
			expectedInput:  "hello",
			expectedCursor: 5,
		},
		{
			name:           "insert at end of string",
			initialInput:   "hello",
			initialCursor:  5,
			insertText:     " world",
			expectedInput:  "hello world",
			expectedCursor: 11,
		},
		{
			name:           "insert in middle of string",
			initialInput:   "helloworld",
			initialCursor:  5,
			insertText:     " ",
			expectedInput:  "hello world",
			expectedCursor: 6,
		},
		{
			name:           "insert unicode characters",
			initialInput:   "hello",
			initialCursor:  5,
			insertText:     " 世界",
			expectedInput:  "hello 世界",
			expectedCursor: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.initialInput)
			h.SetCursor(tt.initialCursor)
			h.Insert(tt.insertText)
			if h.Buffer() != tt.expectedInput {
				t.Errorf("Buffer() = %q, want %q", h.Buffer(), tt.expectedInput)
			}
			if h.Cursor() != tt.expectedCursor {
				t.Errorf("Cursor() = %d, want %d", h.Cursor(), tt.expectedCursor)
			}
		})
	}
}

func TestInputHandlerDeleteBack(t *testing.T) {
	tests := []struct {
		name           string
		initialInput   string
		initialCursor  int
		deleteCount    int
		expectedInput  string
		expectedCursor int
	}{
		{
			name:           "delete from end",
			initialInput:   "hello",
			initialCursor:  5,
			deleteCount:    1,
			expectedInput:  "hell",
			expectedCursor: 4,
		},
		{
			name:           "delete multiple from end",
			initialInput:   "hello",
			initialCursor:  5,
			deleteCount:    3,
			expectedInput:  "he",
			expectedCursor: 2,
		},
		{
			name:           "delete from middle",
			initialInput:   "hello world",
			initialCursor:  6,
			deleteCount:    1,
			expectedInput:  "helloworld",
			expectedCursor: 5,
		},
		{
			name:           "delete from beginning does nothing",
			initialInput:   "hello",
			initialCursor:  0,
			deleteCount:    1,
			expectedInput:  "hello",
			expectedCursor: 0,
		},
		{
			name:           "delete more than available",
			initialInput:   "hello",
			initialCursor:  3,
			deleteCount:    10,
			expectedInput:  "lo",
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.initialInput)
			h.SetCursor(tt.initialCursor)
			h.DeleteBack(tt.deleteCount)
			if h.Buffer() != tt.expectedInput {
				t.Errorf("Buffer() = %q, want %q", h.Buffer(), tt.expectedInput)
			}
			if h.Cursor() != tt.expectedCursor {
				t.Errorf("Cursor() = %d, want %d", h.Cursor(), tt.expectedCursor)
			}
		})
	}
}

func TestInputHandlerDeleteForward(t *testing.T) {
	tests := []struct {
		name           string
		initialInput   string
		initialCursor  int
		deleteCount    int
		expectedInput  string
		expectedCursor int
	}{
		{
			name:           "delete from beginning",
			initialInput:   "hello",
			initialCursor:  0,
			deleteCount:    1,
			expectedInput:  "ello",
			expectedCursor: 0,
		},
		{
			name:           "delete multiple from beginning",
			initialInput:   "hello",
			initialCursor:  0,
			deleteCount:    3,
			expectedInput:  "lo",
			expectedCursor: 0,
		},
		{
			name:           "delete from middle",
			initialInput:   "hello world",
			initialCursor:  5,
			deleteCount:    1,
			expectedInput:  "helloworld",
			expectedCursor: 5,
		},
		{
			name:           "delete at end does nothing",
			initialInput:   "hello",
			initialCursor:  5,
			deleteCount:    1,
			expectedInput:  "hello",
			expectedCursor: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.initialInput)
			h.SetCursor(tt.initialCursor)
			h.DeleteForward(tt.deleteCount)
			if h.Buffer() != tt.expectedInput {
				t.Errorf("Buffer() = %q, want %q", h.Buffer(), tt.expectedInput)
			}
			if h.Cursor() != tt.expectedCursor {
				t.Errorf("Cursor() = %d, want %d", h.Cursor(), tt.expectedCursor)
			}
		})
	}
}

func TestInputHandlerMoveCursor(t *testing.T) {
	tests := []struct {
		name           string
		initialInput   string
		initialCursor  int
		move           int
		expectedCursor int
	}{
		{
			name:           "move right",
			initialInput:   "hello",
			initialCursor:  0,
			move:           1,
			expectedCursor: 1,
		},
		{
			name:           "move left",
			initialInput:   "hello",
			initialCursor:  3,
			move:           -1,
			expectedCursor: 2,
		},
		{
			name:           "move beyond end clamps",
			initialInput:   "hello",
			initialCursor:  3,
			move:           10,
			expectedCursor: 5,
		},
		{
			name:           "move before beginning clamps",
			initialInput:   "hello",
			initialCursor:  2,
			move:           -10,
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.initialInput)
			h.SetCursor(tt.initialCursor)
			h.MoveCursor(tt.move)
			if h.Cursor() != tt.expectedCursor {
				t.Errorf("Cursor() = %d, want %d", h.Cursor(), tt.expectedCursor)
			}
		})
	}
}

func TestInputHandlerFindPrevWordBoundary(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		cursor        int
		expectedBound int
	}{
		{
			name:          "at word end, find word start",
			input:         "hello world",
			cursor:        5,
			expectedBound: 0,
		},
		{
			name:          "after space, find previous word start",
			input:         "hello world",
			cursor:        6,
			expectedBound: 0,
		},
		{
			name:          "at end, find last word start",
			input:         "hello world",
			cursor:        11,
			expectedBound: 6,
		},
		{
			name:          "at beginning",
			input:         "hello",
			cursor:        0,
			expectedBound: 0,
		},
		{
			name:          "multiple spaces",
			input:         "hello   world",
			cursor:        13,
			expectedBound: 8,
		},
		{
			name:          "with punctuation",
			input:         "hello, world",
			cursor:        12,
			expectedBound: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.input)
			h.SetCursor(tt.cursor)
			got := h.FindPrevWordBoundary()
			if got != tt.expectedBound {
				t.Errorf("FindPrevWordBoundary() = %d, want %d", got, tt.expectedBound)
			}
		})
	}
}

func TestInputHandlerFindNextWordBoundary(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		cursor        int
		expectedBound int
	}{
		{
			name:          "at beginning, find next word",
			input:         "hello world",
			cursor:        0,
			expectedBound: 6,
		},
		{
			name:          "in middle of word, skip to next",
			input:         "hello world",
			cursor:        2,
			expectedBound: 6,
		},
		{
			name:          "at space, skip to next word",
			input:         "hello world",
			cursor:        5,
			expectedBound: 6,
		},
		{
			name:          "at end",
			input:         "hello",
			cursor:        5,
			expectedBound: 5,
		},
		{
			name:          "multiple spaces",
			input:         "hello   world",
			cursor:        0,
			expectedBound: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.input)
			h.SetCursor(tt.cursor)
			got := h.FindNextWordBoundary()
			if got != tt.expectedBound {
				t.Errorf("FindNextWordBoundary() = %d, want %d", got, tt.expectedBound)
			}
		})
	}
}

func TestInputHandlerFindLineStart(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		cursor        int
		expectedStart int
	}{
		{
			name:          "single line at end",
			input:         "hello world",
			cursor:        11,
			expectedStart: 0,
		},
		{
			name:          "single line at beginning",
			input:         "hello world",
			cursor:        0,
			expectedStart: 0,
		},
		{
			name:          "multiline on second line",
			input:         "hello\nworld",
			cursor:        8,
			expectedStart: 6,
		},
		{
			name:          "multiline at newline char",
			input:         "hello\nworld",
			cursor:        6,
			expectedStart: 6,
		},
		{
			name:          "multiline on first line",
			input:         "hello\nworld",
			cursor:        3,
			expectedStart: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.input)
			h.SetCursor(tt.cursor)
			got := h.FindLineStart()
			if got != tt.expectedStart {
				t.Errorf("FindLineStart() = %d, want %d", got, tt.expectedStart)
			}
		})
	}
}

func TestInputHandlerFindLineEnd(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		cursor      int
		expectedEnd int
	}{
		{
			name:        "single line at beginning",
			input:       "hello world",
			cursor:      0,
			expectedEnd: 11,
		},
		{
			name:        "single line at end",
			input:       "hello world",
			cursor:      11,
			expectedEnd: 11,
		},
		{
			name:        "multiline on first line",
			input:       "hello\nworld",
			cursor:      3,
			expectedEnd: 5,
		},
		{
			name:        "multiline on second line",
			input:       "hello\nworld",
			cursor:      8,
			expectedEnd: 11,
		},
		{
			name:        "multiline at newline",
			input:       "hello\nworld",
			cursor:      5,
			expectedEnd: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.input)
			h.SetCursor(tt.cursor)
			got := h.FindLineEnd()
			if got != tt.expectedEnd {
				t.Errorf("FindLineEnd() = %d, want %d", got, tt.expectedEnd)
			}
		})
	}
}

func TestIsWordChar(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'a', true},
		{'z', true},
		{'A', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'_', true},
		{' ', false},
		{'.', false},
		{',', false},
		{'\n', false},
		{'-', false},
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			got := isWordChar(tt.char)
			if got != tt.expected {
				t.Errorf("isWordChar(%q) = %v, want %v", tt.char, got, tt.expected)
			}
		})
	}
}

func TestRenderAddTaskCursorBounds(t *testing.T) {
	// This test verifies that renderAddTask doesn't panic when cursor is out of bounds
	// Regression test for: https://github.com/Iron-Ham/claudio/issues/XX
	// The bug occurred when backspacing in slash-command mode caused cursor > len(text)
	tests := []struct {
		name   string
		input  string
		cursor int
	}{
		{
			name:   "cursor beyond empty string",
			input:  "",
			cursor: 1, // This was the panic case: [1:0]
		},
		{
			name:   "cursor way beyond string length",
			input:  "ab",
			cursor: 10,
		},
		{
			name:   "negative cursor",
			input:  "hello",
			cursor: -5,
		},
		{
			name:   "cursor at exact length (valid)",
			input:  "hello",
			cursor: 5,
		},
		{
			name:   "cursor at beginning (valid)",
			input:  "hello",
			cursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				taskInput: NewInputHandler(),
			}
			m.taskInput.SetBuffer(tt.input)
			m.taskInput.SetCursor(tt.cursor) // SetCursor clamps to valid bounds
			// This should not panic
			result := m.renderAddTask(80)
			if result == "" {
				t.Error("renderAddTask returned empty string")
			}
		})
	}
}

func TestInputHandlerClear(t *testing.T) {
	h := NewInputHandler()
	h.SetBuffer("hello world")
	h.SetCursor(5)

	h.Clear()

	if h.Buffer() != "" {
		t.Errorf("Buffer() = %q, want empty", h.Buffer())
	}
	if h.Cursor() != 0 {
		t.Errorf("Cursor() = %d, want 0", h.Cursor())
	}
}

func TestInputHandlerIsEmpty(t *testing.T) {
	h := NewInputHandler()

	if !h.IsEmpty() {
		t.Error("IsEmpty() = false, want true for new handler")
	}

	h.Insert("x")
	if h.IsEmpty() {
		t.Error("IsEmpty() = true, want false after insert")
	}

	h.Clear()
	if !h.IsEmpty() {
		t.Error("IsEmpty() = false, want true after clear")
	}
}

func TestInputHandlerIsAtLineStart(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cursor   int
		expected bool
	}{
		{
			name:     "at beginning of empty string",
			input:    "",
			cursor:   0,
			expected: true,
		},
		{
			name:     "at beginning of non-empty string",
			input:    "hello",
			cursor:   0,
			expected: true,
		},
		{
			name:     "after newline",
			input:    "hello\nworld",
			cursor:   6,
			expected: true,
		},
		{
			name:     "in middle of line",
			input:    "hello",
			cursor:   3,
			expected: false,
		},
		{
			name:     "at end of line before newline",
			input:    "hello\nworld",
			cursor:   5,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewInputHandler()
			h.SetBuffer(tt.input)
			h.SetCursor(tt.cursor)
			got := h.IsAtLineStart()
			if got != tt.expected {
				t.Errorf("IsAtLineStart() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInputHandlerLen(t *testing.T) {
	h := NewInputHandler()

	if h.Len() != 0 {
		t.Errorf("Len() = %d, want 0 for empty handler", h.Len())
	}

	h.Insert("hello")
	if h.Len() != 5 {
		t.Errorf("Len() = %d, want 5", h.Len())
	}

	h.Insert(" 世界") // adds 3 runes
	if h.Len() != 8 {
		t.Errorf("Len() = %d, want 8 (unicode aware)", h.Len())
	}
}

func TestInputHandlerMoveToStart(t *testing.T) {
	h := NewInputHandler()
	h.SetBuffer("hello world")
	h.SetCursor(7)

	h.MoveToStart()

	if h.Cursor() != 0 {
		t.Errorf("Cursor() = %d, want 0", h.Cursor())
	}
}

func TestInputHandlerMoveToEnd(t *testing.T) {
	h := NewInputHandler()
	h.SetBuffer("hello world")
	h.SetCursor(3)

	h.MoveToEnd()

	if h.Cursor() != 11 {
		t.Errorf("Cursor() = %d, want 11", h.Cursor())
	}
}

func TestInputHandlerDeleteWord(t *testing.T) {
	h := NewInputHandler()
	h.SetBuffer("hello world")
	h.SetCursor(11) // at end

	h.DeleteWord()

	if h.Buffer() != "hello " {
		t.Errorf("Buffer() = %q, want %q", h.Buffer(), "hello ")
	}
	if h.Cursor() != 6 {
		t.Errorf("Cursor() = %d, want 6", h.Cursor())
	}
}

func TestInputHandlerDeleteToLineStart(t *testing.T) {
	h := NewInputHandler()
	h.SetBuffer("hello\nworld test")
	h.SetCursor(12) // at 't' in "test" (index: h=0,e=1,l=2,l=3,o=4,\n=5,w=6,o=7,r=8,l=9,d=10, =11,t=12)

	h.DeleteToLineStart()

	// Deletes from cursor (12) to line start (6), removing "world "
	if h.Buffer() != "hello\ntest" {
		t.Errorf("Buffer() = %q, want %q", h.Buffer(), "hello\ntest")
	}
	if h.Cursor() != 6 {
		t.Errorf("Cursor() = %d, want 6", h.Cursor())
	}
}

func TestInputHandlerDeleteToLineEnd(t *testing.T) {
	h := NewInputHandler()
	h.SetBuffer("hello world\ntest")
	h.SetCursor(6) // at "w" in "world"

	h.DeleteToLineEnd()

	if h.Buffer() != "hello \ntest" {
		t.Errorf("Buffer() = %q, want %q", h.Buffer(), "hello \ntest")
	}
	if h.Cursor() != 6 {
		t.Errorf("Cursor() = %d, want 6", h.Cursor())
	}
}

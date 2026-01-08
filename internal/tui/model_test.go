package tui

import (
	"testing"
)

func TestTaskInputInsert(t *testing.T) {
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
			m := Model{
				taskInput:       tt.initialInput,
				taskInputCursor: tt.initialCursor,
			}
			m.taskInputInsert(tt.insertText)
			if m.taskInput != tt.expectedInput {
				t.Errorf("taskInput = %q, want %q", m.taskInput, tt.expectedInput)
			}
			if m.taskInputCursor != tt.expectedCursor {
				t.Errorf("taskInputCursor = %d, want %d", m.taskInputCursor, tt.expectedCursor)
			}
		})
	}
}

func TestTaskInputDeleteBack(t *testing.T) {
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
			m := Model{
				taskInput:       tt.initialInput,
				taskInputCursor: tt.initialCursor,
			}
			m.taskInputDeleteBack(tt.deleteCount)
			if m.taskInput != tt.expectedInput {
				t.Errorf("taskInput = %q, want %q", m.taskInput, tt.expectedInput)
			}
			if m.taskInputCursor != tt.expectedCursor {
				t.Errorf("taskInputCursor = %d, want %d", m.taskInputCursor, tt.expectedCursor)
			}
		})
	}
}

func TestTaskInputDeleteForward(t *testing.T) {
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
			m := Model{
				taskInput:       tt.initialInput,
				taskInputCursor: tt.initialCursor,
			}
			m.taskInputDeleteForward(tt.deleteCount)
			if m.taskInput != tt.expectedInput {
				t.Errorf("taskInput = %q, want %q", m.taskInput, tt.expectedInput)
			}
			if m.taskInputCursor != tt.expectedCursor {
				t.Errorf("taskInputCursor = %d, want %d", m.taskInputCursor, tt.expectedCursor)
			}
		})
	}
}

func TestTaskInputMoveCursor(t *testing.T) {
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
			m := Model{
				taskInput:       tt.initialInput,
				taskInputCursor: tt.initialCursor,
			}
			m.taskInputMoveCursor(tt.move)
			if m.taskInputCursor != tt.expectedCursor {
				t.Errorf("taskInputCursor = %d, want %d", m.taskInputCursor, tt.expectedCursor)
			}
		})
	}
}

func TestTaskInputFindPrevWordBoundary(t *testing.T) {
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
			m := Model{
				taskInput:       tt.input,
				taskInputCursor: tt.cursor,
			}
			got := m.taskInputFindPrevWordBoundary()
			if got != tt.expectedBound {
				t.Errorf("taskInputFindPrevWordBoundary() = %d, want %d", got, tt.expectedBound)
			}
		})
	}
}

func TestTaskInputFindNextWordBoundary(t *testing.T) {
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
			m := Model{
				taskInput:       tt.input,
				taskInputCursor: tt.cursor,
			}
			got := m.taskInputFindNextWordBoundary()
			if got != tt.expectedBound {
				t.Errorf("taskInputFindNextWordBoundary() = %d, want %d", got, tt.expectedBound)
			}
		})
	}
}

func TestTaskInputFindLineStart(t *testing.T) {
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
			m := Model{
				taskInput:       tt.input,
				taskInputCursor: tt.cursor,
			}
			got := m.taskInputFindLineStart()
			if got != tt.expectedStart {
				t.Errorf("taskInputFindLineStart() = %d, want %d", got, tt.expectedStart)
			}
		})
	}
}

func TestTaskInputFindLineEnd(t *testing.T) {
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
			m := Model{
				taskInput:       tt.input,
				taskInputCursor: tt.cursor,
			}
			got := m.taskInputFindLineEnd()
			if got != tt.expectedEnd {
				t.Errorf("taskInputFindLineEnd() = %d, want %d", got, tt.expectedEnd)
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
				taskInput:       tt.input,
				taskInputCursor: tt.cursor,
			}
			// This should not panic
			result := m.renderAddTask(80)
			if result == "" {
				t.Error("renderAddTask returned empty string")
			}
		})
	}
}

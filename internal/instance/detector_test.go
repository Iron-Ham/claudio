package instance

import (
	"strings"
	"testing"
)

func TestWaitingState_String(t *testing.T) {
	tests := []struct {
		state    WaitingState
		expected string
	}{
		{StateWorking, "working"},
		{StateWaitingPermission, "waiting_permission"},
		{StateWaitingQuestion, "waiting_question"},
		{StateWaitingInput, "waiting_input"},
		{StateCompleted, "completed"},
		{StateError, "error"},
		{WaitingState(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("WaitingState(%d).String() = %q, want %q", tt.state, got, tt.expected)
		}
	}
}

func TestWaitingState_IsWaiting(t *testing.T) {
	tests := []struct {
		state    WaitingState
		expected bool
	}{
		{StateWorking, false},
		{StateWaitingPermission, true},
		{StateWaitingQuestion, true},
		{StateWaitingInput, true},
		{StateCompleted, false},
		{StateError, false},
	}

	for _, tt := range tests {
		if got := tt.state.IsWaiting(); got != tt.expected {
			t.Errorf("WaitingState(%d).IsWaiting() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}

func TestDetector_PermissionPatterns(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		output   string
		expected WaitingState
	}{
		{
			name:     "Y/N prompt",
			output:   "Do you want to proceed? [Y/N]",
			expected: StateWaitingPermission,
		},
		{
			name:     "yes/no prompt lowercase",
			output:   "Continue with changes? (y/n)",
			expected: StateWaitingPermission,
		},
		{
			name:     "shall I proceed",
			output:   "Shall I proceed with this operation?",
			expected: StateWaitingPermission,
		},
		{
			name:     "should I continue",
			output:   "Should I continue with the changes?",
			expected: StateWaitingPermission,
		},
		{
			name:     "press y to confirm",
			output:   "Press y to confirm the deletion",
			expected: StateWaitingPermission,
		},
		{
			name:     "type yes to confirm",
			output:   "Type 'yes' to confirm this action",
			expected: StateWaitingPermission,
		},
		{
			name:     "waiting for approval",
			output:   "Waiting for your approval to make changes",
			expected: StateWaitingPermission,
		},
		{
			name:     "requires permission",
			output:   "This action requires your permission",
			expected: StateWaitingPermission,
		},
		{
			name:     "do you want me to run",
			output:   "Do you want me to run the tests?",
			expected: StateWaitingPermission,
		},
		{
			name:     "can I go ahead",
			output:   "Can I go ahead with the refactoring?",
			expected: StateWaitingPermission,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.Detect([]byte(tt.output))
			if got != tt.expected {
				t.Errorf("Detect(%q) = %v, want %v", tt.output, got, tt.expected)
			}
		})
	}
}

func TestDetector_QuestionPatterns(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		output   string
		expected WaitingState
	}{
		{
			name:     "question mark at end",
			output:   "What file should I modify?",
			expected: StateWaitingQuestion,
		},
		{
			name:     "which would you like",
			output:   "Which option would you prefer?",
			expected: StateWaitingQuestion,
		},
		{
			name:     "please specify",
			output:   "Please specify the target directory",
			expected: StateWaitingQuestion,
		},
		{
			name:     "could you tell me",
			output:   "Could you tell me more about the requirements?",
			expected: StateWaitingQuestion,
		},
		{
			name:     "I need to know",
			output:   "I need to know which database to use",
			expected: StateWaitingQuestion,
		},
		{
			name:     "select one",
			output:   "Select one of the following options:",
			expected: StateWaitingQuestion,
		},
		{
			name:     "waiting for input",
			output:   "Waiting for your input",
			expected: StateWaitingQuestion,
		},
		{
			name:     "enter the path",
			output:   "Enter the file path:",
			expected: StateWaitingQuestion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.Detect([]byte(tt.output))
			if got != tt.expected {
				t.Errorf("Detect(%q) = %v, want %v", tt.output, got, tt.expected)
			}
		})
	}
}

func TestDetector_CompletionPatterns(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		output   string
		expected WaitingState
	}{
		{
			name:     "task complete",
			output:   "Task complete!",
			expected: StateCompleted,
		},
		{
			name:     "I've completed",
			output:   "I've completed all the requested changes",
			expected: StateCompleted,
		},
		{
			name:     "all done",
			output:   "All done with the implementation",
			expected: StateCompleted,
		},
		{
			name:     "let me know if anything else",
			output:   "Let me know if you need anything else",
			expected: StateCompleted,
		},
		{
			name:     "is there anything else",
			output:   "Is there anything else I can help with?",
			expected: StateCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.Detect([]byte(tt.output))
			if got != tt.expected {
				t.Errorf("Detect(%q) = %v, want %v", tt.output, got, tt.expected)
			}
		})
	}
}

func TestDetector_ErrorPatterns(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		output   string
		expected WaitingState
	}{
		{
			name:     "error colon",
			output:   "Error: could not find file",
			expected: StateError,
		},
		{
			name:     "could not complete",
			output:   "Could not complete the operation",
			expected: StateError,
		},
		{
			name:     "unable to proceed",
			output:   "Unable to proceed with the task",
			expected: StateError,
		},
		{
			name:     "fatal error",
			output:   "Fatal error encountered",
			expected: StateError,
		},
		{
			name:     "process crashed",
			output:   "The process crashed unexpectedly",
			expected: StateError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.Detect([]byte(tt.output))
			if got != tt.expected {
				t.Errorf("Detect(%q) = %v, want %v", tt.output, got, tt.expected)
			}
		})
	}
}

func TestDetector_WorkingPatterns(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name     string
		output   string
		expected WaitingState
	}{
		{
			name:     "reading file",
			output:   "Reading file...",
			expected: StateWorking,
		},
		{
			name:     "let me check",
			output:   "Let me check the code",
			expected: StateWorking,
		},
		{
			name:     "let me analyze",
			output:   "Let me analyze the codebase",
			expected: StateWorking,
		},
		{
			name:     "going to check",
			output:   "Going to check the implementation",
			expected: StateWorking,
		},
		{
			name:     "working on",
			output:   "Working on the feature",
			expected: StateWorking,
		},
		{
			name:     "spinner character",
			output:   "Processing ⠋",
			expected: StateWorking,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.Detect([]byte(tt.output))
			if got != tt.expected {
				t.Errorf("Detect(%q) = %v, want %v", tt.output, got, tt.expected)
			}
		})
	}
}

func TestDetector_WorkingOverridesWaiting(t *testing.T) {
	d := NewDetector()

	// If there's a question in history but we're currently working,
	// should report as working
	output := `What file should I modify?

Let me analyze the codebase...`

	got := d.Detect([]byte(output))
	if got != StateWorking {
		t.Errorf("Detect with working indicator after question = %v, want StateWorking", got)
	}
}

func TestDetector_EmptyOutput(t *testing.T) {
	d := NewDetector()

	got := d.Detect([]byte{})
	if got != StateWorking {
		t.Errorf("Detect(empty) = %v, want StateWorking", got)
	}

	got = d.Detect(nil)
	if got != StateWorking {
		t.Errorf("Detect(nil) = %v, want StateWorking", got)
	}
}

func TestDetector_LargeOutput(t *testing.T) {
	d := NewDetector()

	// Create large output with question at the end
	large := strings.Repeat("Some content\n", 1000)
	large += "What file should I modify?"

	got := d.Detect([]byte(large))
	if got != StateWaitingQuestion {
		t.Errorf("Detect(large output with question) = %v, want StateWaitingQuestion", got)
	}
}

func TestDetector_AnsiStripping(t *testing.T) {
	d := NewDetector()

	// Output with ANSI escape codes
	output := "\x1b[32mDo you want to proceed?\x1b[0m [Y/N]"

	got := d.Detect([]byte(output))
	if got != StateWaitingPermission {
		t.Errorf("Detect(ANSI output) = %v, want StateWaitingPermission", got)
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "\x1b[32mgreen\x1b[0m",
			expected: "green",
		},
		{
			input:    "\x1b[1;34mbold blue\x1b[0m",
			expected: "bold blue",
		},
		{
			input:    "no escape codes",
			expected: "no escape codes",
		},
		{
			input:    "\x1b]0;title\x07text",
			expected: "text",
		},
	}

	for _, tt := range tests {
		got := stripAnsi(tt.input)
		if got != tt.expected {
			t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetLastNonEmptyLines(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		n        int
		expected []string
	}{
		{
			name:     "basic",
			lines:    []string{"a", "b", "c"},
			n:        2,
			expected: []string{"b", "c"},
		},
		{
			name:     "with empty lines",
			lines:    []string{"a", "", "b", "", "c", ""},
			n:        2,
			expected: []string{"b", "c"},
		},
		{
			name:     "request more than available",
			lines:    []string{"a", "b"},
			n:        5,
			expected: []string{"a", "b"},
		},
		{
			name:     "all empty",
			lines:    []string{"", "", ""},
			n:        2,
			expected: []string{},
		},
		{
			name:     "whitespace only lines",
			lines:    []string{"a", "   ", "\t", "b"},
			n:        2,
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLastNonEmptyLines(tt.lines, tt.n)
			if len(got) != len(tt.expected) {
				t.Errorf("getLastNonEmptyLines() length = %d, want %d", len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("getLastNonEmptyLines()[%d] = %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func BenchmarkDetector_Detect(b *testing.B) {
	d := NewDetector()
	output := []byte("This is some sample output\nWith multiple lines\nDo you want to proceed? [Y/N]")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Detect(output)
	}
}

func BenchmarkDetector_LargeOutput(b *testing.B) {
	d := NewDetector()
	large := []byte(strings.Repeat("Some content line\n", 500) + "What file should I modify?")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		d.Detect(large)
	}
}

func BenchmarkStripAnsi(b *testing.B) {
	text := "\x1b[32mThis is \x1b[1;34msome colored\x1b[0m text with \x1b[33mmultiple\x1b[0m escape codes"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stripAnsi(text)
	}
}

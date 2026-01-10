package detect

import (
	"testing"
)

func TestWaitingState_String(t *testing.T) {
	tests := []struct {
		state WaitingState
		want  string
	}{
		{StateWorking, "working"},
		{StateWaitingPermission, "waiting_permission"},
		{StateWaitingQuestion, "waiting_question"},
		{StateWaitingInput, "waiting_input"},
		{StateCompleted, "completed"},
		{StateError, "error"},
		{StatePROpened, "pr_opened"},
		{WaitingState(99), "unknown"},
	}

	for _, tc := range tests {
		got := tc.state.String()
		if got != tc.want {
			t.Errorf("WaitingState(%d).String() = %q, want %q", tc.state, got, tc.want)
		}
	}
}

func TestWaitingState_IsWaiting(t *testing.T) {
	tests := []struct {
		state WaitingState
		want  bool
	}{
		{StateWorking, false},
		{StateWaitingPermission, true},
		{StateWaitingQuestion, true},
		{StateWaitingInput, true},
		{StateCompleted, false},
		{StateError, false},
		{StatePROpened, false},
	}

	for _, tc := range tests {
		got := tc.state.IsWaiting()
		if got != tc.want {
			t.Errorf("%s.IsWaiting() = %v, want %v", tc.state, got, tc.want)
		}
	}
}

func TestDetector_Detect_Empty(t *testing.T) {
	d := NewDetector()

	if got := d.Detect(nil); got != StateWorking {
		t.Errorf("Detect(nil) = %v, want StateWorking", got)
	}

	if got := d.Detect([]byte{}); got != StateWorking {
		t.Errorf("Detect([]) = %v, want StateWorking", got)
	}
}

func TestDetector_Detect_PermissionPrompts(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "Y/N prompt",
			output: "Do you want to proceed? [Y/N]",
		},
		{
			name:   "Yes/No prompt",
			output: "Continue with changes? [Yes/No]",
		},
		{
			name:   "lowercase y/n",
			output: "Apply changes? (y/n)",
		},
		{
			name:   "shall I proceed",
			output: "The changes are ready. Shall I proceed with the commit?",
		},
		{
			name:   "should I continue",
			output: "This will modify 5 files. Should I continue?",
		},
		{
			name:   "can I proceed",
			output: "Can I proceed with the refactoring?",
		},
		{
			name:   "may I apply",
			output: "May I apply these changes?",
		},
		{
			name:   "allow this action",
			output: "Please allow this action to continue",
		},
		{
			name:   "press y to confirm",
			output: "Press y to confirm the operation",
		},
		{
			name:   "type yes to confirm",
			output: "Type 'yes' to confirm deletion",
		},
		{
			name:   "waiting for approval",
			output: "Waiting for your approval to proceed",
		},
		{
			name:   "requires permission",
			output: "This operation requires your permission",
		},
		{
			name:   "do you want me to proceed",
			output: "Do you want me to proceed with the changes?",
		},
		{
			name:   "do you want to run",
			output: "Do you want to run the tests?",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Detect([]byte(tc.output))
			if got != StateWaitingPermission {
				t.Errorf("Detect(%q) = %v, want StateWaitingPermission", tc.output, got)
			}
		})
	}
}

func TestDetector_Detect_Questions(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "question mark",
			output: "What is the target directory?",
		},
		{
			name:   "what would you",
			output: "What would you like me to name the file?",
		},
		{
			name:   "which do you",
			output: "Which do you prefer?",
		},
		{
			name:   "how should I",
			output: "How should I structure the code?",
		},
		{
			name:   "could you tell me",
			output: "Could you tell me more about the requirements?",
		},
		{
			name:   "please specify",
			output: "Please specify the output format",
		},
		{
			name:   "please clarify",
			output: "Please clarify which version you need",
		},
		{
			name:   "please provide",
			output: "Please provide the API endpoint URL",
		},
		{
			name:   "I need to know",
			output: "I need to know which database to use",
		},
		{
			name:   "select one",
			output: "Select one of the following options:",
		},
		{
			name:   "choose from",
			output: "Choose from these possibilities:",
		},
		{
			name:   "waiting for input",
			output: "Waiting for your input to continue",
		},
		{
			name:   "enter your",
			output: "Enter your preferred configuration:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Detect([]byte(tc.output))
			if got != StateWaitingQuestion {
				t.Errorf("Detect(%q) = %v, want StateWaitingQuestion", tc.output, got)
			}
		})
	}
}

func TestDetector_Detect_InputWaiting(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "bypass permissions indicator",
			output: "⏵⏵ bypass permissions",
		},
		{
			name:   "send indicator",
			output: "> Write a test function ↵ send",
		},
		{
			name:   "mode cycling hint",
			output: "Use (shift+tab to cycle) between modes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Detect([]byte(tc.output))
			if got != StateWaitingInput {
				t.Errorf("Detect(%q) = %v, want StateWaitingInput", tc.output, got)
			}
		})
	}
}

func TestDetector_Detect_Errors(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "session error",
			output: "Error: session expired or invalid",
		},
		{
			name:   "connection error",
			output: "Error: connection failed to API server",
		},
		{
			name:   "authentication error",
			output: "Error: authentication failed, check your API key",
		},
		{
			name:   "rate limit exceeded",
			output: "Rate limit exceeded, please wait",
		},
		{
			name:   "quota reached",
			output: "API quota reached for this billing period",
		},
		{
			name:   "SIGTERM",
			output: "claude terminated with signal: SIGTERM",
		},
		{
			name:   "SIGKILL",
			output: "Process killed: SIGKILL",
		},
		{
			name:   "API error 429",
			output: "API error: 429 Too Many Requests",
		},
		{
			name:   "request failed 500",
			output: "Request failed with status 500",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Detect([]byte(tc.output))
			if got != StateError {
				t.Errorf("Detect(%q) = %v, want StateError", tc.output, got)
			}
		})
	}
}

func TestDetector_Detect_PROpened(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "simple PR URL",
			output: "https://github.com/owner/repo/pull/123",
		},
		{
			name:   "PR URL in sentence",
			output: "Created PR at https://github.com/Iron-Ham/claudio/pull/456",
		},
		{
			name:   "PR URL with high number",
			output: "Pull request opened: https://github.com/org/project/pull/99999",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Detect([]byte(tc.output))
			if got != StatePROpened {
				t.Errorf("Detect(%q) = %v, want StatePROpened", tc.output, got)
			}
		})
	}
}

func TestDetector_Detect_Working(t *testing.T) {
	d := NewDetector()

	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "reading indicator",
			output: "Reading...",
		},
		{
			name:   "writing indicator",
			output: "Writing...",
		},
		{
			name:   "analyzing indicator",
			output: "Analyzing...",
		},
		{
			name:   "executing indicator",
			output: "Executing...",
		},
		{
			name:   "let me check",
			output: "Let me check the file contents",
		},
		{
			name:   "let me look",
			output: "Let me look at the implementation",
		},
		{
			name:   "i'll check",
			output: "I'll check the dependencies first",
		},
		{
			name:   "going to analyze",
			output: "Going to analyze the error logs",
		},
		{
			name:   "about to start",
			output: "About to start the test suite",
		},
		{
			name:   "working on",
			output: "Working on the implementation...",
		},
		{
			name:   "processing",
			output: "Processing the request",
		},
		{
			name:   "spinner char braille 1",
			output: "⠋ Loading data",
		},
		{
			name:   "spinner char braille 2",
			output: "⠙ Processing",
		},
		{
			name:   "spinner char braille 3",
			output: "⠹ Compiling",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := d.Detect([]byte(tc.output))
			if got != StateWorking {
				t.Errorf("Detect(%q) = %v, want StateWorking", tc.output, got)
			}
		})
	}
}

func TestDetector_Detect_WorkingOverridesQuestion(t *testing.T) {
	d := NewDetector()

	// If there's a question in the history but we're currently working,
	// we should detect as working
	output := `What file would you like me to create?
Actually let me check the existing files first...
Reading...`

	got := d.Detect([]byte(output))
	if got != StateWorking {
		t.Errorf("Detect() = %v, want StateWorking (working should override historical question)", got)
	}
}

func TestDetector_Detect_Priority(t *testing.T) {
	d := NewDetector()

	// Test that PR opened takes priority over questions
	// (since PR URL might appear alongside explanatory text with questions)
	output := "Would you like me to review the changes? https://github.com/owner/repo/pull/123"
	got := d.Detect([]byte(output))
	if got != StatePROpened {
		t.Errorf("Detect() = %v, want StatePROpened (PR should take priority)", got)
	}

	// Test that error takes priority over questions
	// Using rate limit pattern which doesn't require ^ anchor
	output = "What went wrong?\nRate limit exceeded, please try again later"
	got = d.Detect([]byte(output))
	if got != StateError {
		t.Errorf("Detect() = %v, want StateError (error should take priority)", got)
	}

	// Test that permission takes priority over general questions
	output = "What files should I modify? Do you want me to proceed? [Y/N]"
	got = d.Detect([]byte(output))
	if got != StateWaitingPermission {
		t.Errorf("Detect() = %v, want StateWaitingPermission (permission should take priority over question)", got)
	}
}

func TestDetector_Detect_ANSICodes(t *testing.T) {
	d := NewDetector()

	// Test that ANSI codes are properly stripped
	// ANSI codes: ESC[32m (green) and ESC[0m (reset)
	output := "\x1b[32mDo you want to proceed? [Y/N]\x1b[0m"
	got := d.Detect([]byte(output))
	if got != StateWaitingPermission {
		t.Errorf("Detect() with ANSI codes = %v, want StateWaitingPermission", got)
	}
}

func TestDetector_Detect_LongOutput(t *testing.T) {
	d := NewDetector()

	// Create very long output with a question at the end
	longPrefix := make([]byte, 5000)
	for i := range longPrefix {
		longPrefix[i] = 'x'
	}
	output := append(longPrefix, []byte("\nDo you want to proceed? [Y/N]")...)

	got := d.Detect(output)
	if got != StateWaitingPermission {
		t.Errorf("Detect() on long output = %v, want StateWaitingPermission", got)
	}
}

func TestDetector_Detect_DefaultWorking(t *testing.T) {
	d := NewDetector()

	// Regular code output should be detected as working
	output := `func main() {
    fmt.Println("Hello, World!")
}`
	got := d.Detect([]byte(output))
	if got != StateWorking {
		t.Errorf("Detect() on code = %v, want StateWorking", got)
	}
}

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no ansi",
			input: "plain text",
			want:  "plain text",
		},
		{
			name:  "color codes",
			input: "\x1b[32mgreen\x1b[0m",
			want:  "green",
		},
		{
			name:  "cursor movement",
			input: "\x1b[2Jclear screen",
			want:  "clear screen",
		},
		{
			name:  "multiple codes",
			input: "\x1b[1m\x1b[32mBold Green\x1b[0m Normal",
			want:  "Bold Green Normal",
		},
		{
			name:  "OSC sequence",
			input: "\x1b]0;Title\x07content",
			want:  "content",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := StripAnsi(tc.input)
			if got != tc.want {
				t.Errorf("StripAnsi(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestGetLastNonEmptyLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		n     int
		want  []string
	}{
		{
			name:  "basic",
			lines: []string{"a", "b", "c"},
			n:     2,
			want:  []string{"b", "c"},
		},
		{
			name:  "with empty lines",
			lines: []string{"a", "", "b", "", "c", ""},
			n:     3,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "fewer than n",
			lines: []string{"a", "b"},
			n:     5,
			want:  []string{"a", "b"},
		},
		{
			name:  "all empty",
			lines: []string{"", "", ""},
			n:     2,
			want:  []string{},
		},
		{
			name:  "whitespace lines",
			lines: []string{"  ", "a", "\t", "b"},
			n:     2,
			want:  []string{"a", "b"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetLastNonEmptyLines(tc.lines, tc.n)
			if len(got) != len(tc.want) {
				t.Errorf("GetLastNonEmptyLines() returned %d lines, want %d", len(got), len(tc.want))
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("GetLastNonEmptyLines()[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDetector_Interface(t *testing.T) {
	// Verify Detector implements StateDetector
	var _ StateDetector = NewDetector()
}

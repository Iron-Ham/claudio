package detect

import (
	"testing"
)

func TestSessionDetector_DetectSessionID(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "empty output",
			output:   "",
			expected: "",
		},
		{
			name:     "no session ID",
			output:   "Claude is working on your task...",
			expected: "",
		},
		{
			name:     "session ID in UUID format",
			output:   "Session: 12345678-1234-1234-1234-123456789012",
			expected: "12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "session ID with lowercase hex",
			output:   "Session: abcdef12-abcd-abcd-abcd-abcdef123456",
			expected: "abcdef12-abcd-abcd-abcd-abcdef123456",
		},
		{
			name:     "resuming session message",
			output:   "Resuming session abcdef12-1234-5678-9abc-def012345678",
			expected: "abcdef12-1234-5678-9abc-def012345678",
		},
		{
			name:     "conversation ID in JSON format",
			output:   `{"conversation_id": "abcdef12-1234-5678-9abc-def012345678"}`,
			expected: "abcdef12-1234-5678-9abc-def012345678",
		},
		{
			name:     "session ID with spaces around",
			output:   "Session:   12345678-1234-1234-1234-123456789012  \n",
			expected: "12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "session ID in longer output",
			output:   "Welcome to Claude Code!\nSession: 12345678-1234-1234-1234-123456789012\nStarting task...",
			expected: "12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "session ID with ANSI codes",
			output:   "\x1b[32mSession:\x1b[0m 12345678-1234-1234-1234-123456789012",
			expected: "12345678-1234-1234-1234-123456789012",
		},
		{
			name:     "hex string without dashes (32 chars)",
			output:   "session: 12345678123412341234123456789012",
			expected: "12345678123412341234123456789012",
		},
	}

	detector := NewSessionDetector()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset detector before each test
			detector.Reset()

			got := detector.DetectSessionID([]byte(tt.output))
			if got != tt.expected {
				t.Errorf("DetectSessionID() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSessionDetector_ProcessOutput_NewIDOnly(t *testing.T) {
	detector := NewSessionDetector()

	// First call should return the session ID
	output1 := "Session: 12345678-1234-1234-1234-123456789012"
	got1 := detector.ProcessOutput([]byte(output1))
	if got1 != "12345678-1234-1234-1234-123456789012" {
		t.Errorf("First ProcessOutput() = %q, want session ID", got1)
	}

	// Second call with same ID should return empty (not new)
	got2 := detector.ProcessOutput([]byte(output1))
	if got2 != "" {
		t.Errorf("Second ProcessOutput() with same ID = %q, want empty string", got2)
	}

	// Third call with different ID should return the new ID
	output3 := "Session: aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	got3 := detector.ProcessOutput([]byte(output3))
	if got3 != "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee" {
		t.Errorf("Third ProcessOutput() with new ID = %q, want new session ID", got3)
	}
}

func TestSessionDetector_Reset(t *testing.T) {
	detector := NewSessionDetector()

	// First call
	output := "Session: 12345678-1234-1234-1234-123456789012"
	got1 := detector.ProcessOutput([]byte(output))
	if got1 != "12345678-1234-1234-1234-123456789012" {
		t.Errorf("First ProcessOutput() = %q, want session ID", got1)
	}

	// Same ID should be cached
	got2 := detector.ProcessOutput([]byte(output))
	if got2 != "" {
		t.Errorf("Cached ProcessOutput() = %q, want empty string", got2)
	}

	// After reset, same ID should be returned again
	detector.Reset()
	got3 := detector.ProcessOutput([]byte(output))
	if got3 != "12345678-1234-1234-1234-123456789012" {
		t.Errorf("After Reset() ProcessOutput() = %q, want session ID", got3)
	}
}

func TestSessionDetector_LastDetectedID(t *testing.T) {
	detector := NewSessionDetector()

	// Initially empty
	if got := detector.LastDetectedID(); got != "" {
		t.Errorf("Initial LastDetectedID() = %q, want empty string", got)
	}

	// After detection
	output := "Session: 12345678-1234-1234-1234-123456789012"
	detector.ProcessOutput([]byte(output))
	if got := detector.LastDetectedID(); got != "12345678-1234-1234-1234-123456789012" {
		t.Errorf("After detection LastDetectedID() = %q, want session ID", got)
	}
}

func TestIsValidSessionID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		expected bool
	}{
		{"empty string", "", false},
		{"too short", "abc", false},
		{"UUID with dashes", "12345678-1234-1234-1234-123456789012", true},
		{"hex without dashes", "12345678123412341234123456789012", true},
		{"uppercase hex", "ABCDEF12-1234-5678-9ABC-DEF012345678", true},
		{"mixed case", "AbCdEf12-1234-5678-9abc-DEF012345678", true},
		{"invalid characters", "gggggggg-1234-1234-1234-123456789012", false},
		{"spaces in ID", "12345678 1234 1234 1234 123456789012", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSessionID(tt.id)
			if got != tt.expected {
				t.Errorf("isValidSessionID(%q) = %v, want %v", tt.id, got, tt.expected)
			}
		})
	}
}

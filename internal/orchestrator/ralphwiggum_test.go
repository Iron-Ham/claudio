package orchestrator

import (
	"testing"
	"time"
)

func TestNewRalphWiggumSession(t *testing.T) {
	task := "Fix all failing tests"
	config := DefaultRalphWiggumConfig()
	config.MaxIterations = 10

	session := NewRalphWiggumSession(task, config)

	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.Task != task {
		t.Errorf("expected task %q, got %q", task, session.Task)
	}
	if session.Phase != PhaseRalphWiggumIterating {
		t.Errorf("expected phase %q, got %q", PhaseRalphWiggumIterating, session.Phase)
	}
	if session.Config.MaxIterations != 10 {
		t.Errorf("expected max iterations 10, got %d", session.Config.MaxIterations)
	}
	if len(session.Iterations) != 0 {
		t.Errorf("expected 0 iterations, got %d", len(session.Iterations))
	}
}

func TestDefaultRalphWiggumConfig(t *testing.T) {
	config := DefaultRalphWiggumConfig()

	if config.CompletionPromise != "DONE" {
		t.Errorf("expected completion promise %q, got %q", "DONE", config.CompletionPromise)
	}
	if config.MaxIterations != 50 {
		t.Errorf("expected max iterations 50, got %d", config.MaxIterations)
	}
	if !config.AutoContinue {
		t.Error("expected auto continue to be true")
	}
}

func TestRalphWiggumSession_CurrentIteration(t *testing.T) {
	session := NewRalphWiggumSession("test", DefaultRalphWiggumConfig())

	if session.CurrentIteration() != 0 {
		t.Errorf("expected current iteration 0, got %d", session.CurrentIteration())
	}

	// Add iterations
	session.Iterations = append(session.Iterations, RalphWiggumIteration{Index: 0})
	if session.CurrentIteration() != 1 {
		t.Errorf("expected current iteration 1, got %d", session.CurrentIteration())
	}

	session.Iterations = append(session.Iterations, RalphWiggumIteration{Index: 1})
	if session.CurrentIteration() != 2 {
		t.Errorf("expected current iteration 2, got %d", session.CurrentIteration())
	}
}

func TestRalphWiggumSession_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		phase    RalphWiggumPhase
		expected bool
	}{
		{"iterating", PhaseRalphWiggumIterating, false},
		{"paused", PhaseRalphWiggumPaused, false},
		{"complete", PhaseRalphWiggumComplete, true},
		{"max iterations", PhaseRalphWiggumMaxIterations, true},
		{"failed", PhaseRalphWiggumFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewRalphWiggumSession("test", DefaultRalphWiggumConfig())
			session.Phase = tt.phase

			if session.IsComplete() != tt.expected {
				t.Errorf("expected IsComplete() = %v for phase %q", tt.expected, tt.phase)
			}
		})
	}
}

func TestRalphWiggumSession_ShouldContinue(t *testing.T) {
	tests := []struct {
		name          string
		phase         RalphWiggumPhase
		iterations    int
		maxIterations int
		expected      bool
	}{
		{"iterating with room", PhaseRalphWiggumIterating, 5, 10, true},
		{"iterating at max", PhaseRalphWiggumIterating, 10, 10, false},
		{"iterating unlimited", PhaseRalphWiggumIterating, 100, 0, true},
		{"paused", PhaseRalphWiggumPaused, 5, 10, false},
		{"complete", PhaseRalphWiggumComplete, 5, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultRalphWiggumConfig()
			config.MaxIterations = tt.maxIterations

			session := NewRalphWiggumSession("test", config)
			session.Phase = tt.phase
			for i := 0; i < tt.iterations; i++ {
				session.Iterations = append(session.Iterations, RalphWiggumIteration{Index: i})
			}

			if session.ShouldContinue() != tt.expected {
				t.Errorf("expected ShouldContinue() = %v", tt.expected)
			}
		})
	}
}

func TestCheckOutputForCompletionPromise(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		promise  string
		expected bool
	}{
		{
			name:     "simple match",
			output:   "Working on task...\n<promise>DONE</promise>\nTask complete.",
			promise:  "DONE",
			expected: true,
		},
		{
			name:     "case insensitive",
			output:   "Working on task...\n<Promise>done</Promise>",
			promise:  "DONE",
			expected: true,
		},
		{
			name:     "with whitespace",
			output:   "<promise>  COMPLETE  </promise>",
			promise:  "COMPLETE",
			expected: true,
		},
		{
			name:     "no match",
			output:   "Still working...",
			promise:  "DONE",
			expected: false,
		},
		{
			name:     "wrong promise",
			output:   "<promise>NOT_DONE</promise>",
			promise:  "DONE",
			expected: false,
		},
		{
			name:     "empty promise",
			output:   "<promise>DONE</promise>",
			promise:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckOutputForCompletionPromise(tt.output, tt.promise)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractPromiseFromOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "simple extraction",
			output:   "<promise>DONE</promise>",
			expected: "DONE",
		},
		{
			name:     "with surrounding text",
			output:   "Some text\n<promise>COMPLETE</promise>\nMore text",
			expected: "COMPLETE",
		},
		{
			name:     "with whitespace",
			output:   "<promise>  ALL_TESTS_PASSING  </promise>",
			expected: "ALL_TESTS_PASSING",
		},
		{
			name:     "no promise",
			output:   "Just regular text",
			expected: "",
		},
		{
			name:     "case variations",
			output:   "<PROMISE>Test</PROMISE>",
			expected: "Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractPromiseFromOutput(tt.output)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRalphWiggumManager_SetPhase(t *testing.T) {
	session := NewRalphWiggumSession("test", DefaultRalphWiggumConfig())
	manager := NewRalphWiggumManager(nil, nil, session, nil)

	if session.Phase != PhaseRalphWiggumIterating {
		t.Errorf("expected initial phase %q", PhaseRalphWiggumIterating)
	}

	manager.SetPhase(PhaseRalphWiggumPaused)
	if session.Phase != PhaseRalphWiggumPaused {
		t.Errorf("expected phase %q after SetPhase", PhaseRalphWiggumPaused)
	}
}

func TestRalphWiggumManager_StartIteration(t *testing.T) {
	session := NewRalphWiggumSession("test", DefaultRalphWiggumConfig())
	manager := NewRalphWiggumManager(nil, nil, session, nil)

	if len(session.Iterations) != 0 {
		t.Errorf("expected 0 iterations initially")
	}

	manager.StartIteration()

	if len(session.Iterations) != 1 {
		t.Errorf("expected 1 iteration after StartIteration")
	}
	if session.Iterations[0].Index != 0 {
		t.Errorf("expected iteration index 0, got %d", session.Iterations[0].Index)
	}
}

func TestRalphWiggumManager_CompleteIteration(t *testing.T) {
	session := NewRalphWiggumSession("test", DefaultRalphWiggumConfig())
	manager := NewRalphWiggumManager(nil, nil, session, nil)

	manager.StartIteration()
	manager.CompleteIteration(true)

	if session.Iterations[0].CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
	if !session.Iterations[0].HasCommits {
		t.Error("expected HasCommits to be true")
	}
}

func TestRalphWiggumManager_EventCallback(t *testing.T) {
	session := NewRalphWiggumSession("test", DefaultRalphWiggumConfig())
	manager := NewRalphWiggumManager(nil, nil, session, nil)

	var receivedEvent RalphWiggumEvent
	manager.SetEventCallback(func(event RalphWiggumEvent) {
		receivedEvent = event
	})

	manager.SetPhase(PhaseRalphWiggumComplete)

	// Give the event time to be processed (synchronous in this implementation)
	time.Sleep(10 * time.Millisecond)

	if receivedEvent.Type != EventRalphWiggumPhaseChange {
		t.Errorf("expected event type %q, got %q", EventRalphWiggumPhaseChange, receivedEvent.Type)
	}
	if receivedEvent.Message != string(PhaseRalphWiggumComplete) {
		t.Errorf("expected event message %q, got %q", PhaseRalphWiggumComplete, receivedEvent.Message)
	}
}

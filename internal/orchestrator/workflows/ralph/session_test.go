package ralph

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	prompt := "Test prompt"
	config := &Config{
		MaxIterations:     10,
		CompletionPromise: "DONE",
	}

	session := NewSession(prompt, config)

	if session.Prompt != prompt {
		t.Errorf("expected prompt %q, got %q", prompt, session.Prompt)
	}
	if session.Config != config {
		t.Error("expected config to be set")
	}
	if session.CurrentIteration != 0 {
		t.Errorf("expected initial iteration 0, got %d", session.CurrentIteration)
	}
	if session.Phase != PhaseWorking {
		t.Errorf("expected phase %q, got %q", PhaseWorking, session.Phase)
	}
	if session.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestNewSessionWithNilConfig(t *testing.T) {
	session := NewSession("Test", nil)

	if session.Config == nil {
		t.Error("expected default config to be created")
	}
	if session.Config.MaxIterations != 50 {
		t.Errorf("expected default MaxIterations 50, got %d", session.Config.MaxIterations)
	}
}

func TestSessionIsActive(t *testing.T) {
	tests := []struct {
		name     string
		phase    Phase
		expected bool
	}{
		{"working", PhaseWorking, true},
		{"complete", PhaseComplete, false},
		{"max iterations", PhaseMaxIterations, false},
		{"cancelled", PhaseCancelled, false},
		{"error", PhaseError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession("Test", nil)
			session.Phase = tt.phase
			if session.IsActive() != tt.expected {
				t.Errorf("expected IsActive() = %v for phase %q", tt.expected, tt.phase)
			}
		})
	}
}

func TestSessionShouldContinue(t *testing.T) {
	tests := []struct {
		name           string
		phase          Phase
		currentIter    int
		maxIter        int
		shouldContinue bool
	}{
		{"working below max", PhaseWorking, 1, 10, true},
		{"working at max", PhaseWorking, 10, 10, false},
		{"working above max", PhaseWorking, 11, 10, false},
		{"working no limit", PhaseWorking, 100, 0, true},
		{"complete", PhaseComplete, 1, 10, false},
		{"cancelled", PhaseCancelled, 1, 10, false},
		{"error", PhaseError, 1, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession("Test", &Config{MaxIterations: tt.maxIter})
			session.Phase = tt.phase
			session.CurrentIteration = tt.currentIter
			if session.ShouldContinue() != tt.shouldContinue {
				t.Errorf("expected ShouldContinue() = %v", tt.shouldContinue)
			}
		})
	}
}

func TestSessionCheckCompletionPromise(t *testing.T) {
	tests := []struct {
		name     string
		promise  string
		output   string
		expected bool
	}{
		{"found exact", "DONE", "Task completed. DONE", true},
		{"found partial", "DONE", "DONE: all finished", true},
		{"not found", "DONE", "Still working on it", false},
		{"empty promise", "", "DONE", false},
		{"case sensitive", "done", "DONE", false},
		{"empty output", "DONE", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession("Test", &Config{CompletionPromise: tt.promise})
			if session.CheckCompletionPromise(tt.output) != tt.expected {
				t.Errorf("expected CheckCompletionPromise() = %v", tt.expected)
			}
		})
	}
}

func TestSessionMarkComplete(t *testing.T) {
	session := NewSession("Test", nil)

	session.MarkComplete()

	if session.Phase != PhaseComplete {
		t.Errorf("expected phase %q, got %q", PhaseComplete, session.Phase)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSessionMarkMaxIterationsReached(t *testing.T) {
	session := NewSession("Test", nil)

	session.MarkMaxIterationsReached()

	if session.Phase != PhaseMaxIterations {
		t.Errorf("expected phase %q, got %q", PhaseMaxIterations, session.Phase)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSessionMarkCancelled(t *testing.T) {
	session := NewSession("Test", nil)

	session.MarkCancelled()

	if session.Phase != PhaseCancelled {
		t.Errorf("expected phase %q, got %q", PhaseCancelled, session.Phase)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSessionMarkError(t *testing.T) {
	session := NewSession("Test", nil)
	testErr := errGenericTest

	session.MarkError(testErr)

	if session.Phase != PhaseError {
		t.Errorf("expected phase %q, got %q", PhaseError, session.Phase)
	}
	if session.Error != testErr.Error() {
		t.Errorf("expected error %q, got %q", testErr.Error(), session.Error)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestSessionIncrementIteration(t *testing.T) {
	session := NewSession("Test", nil)

	session.IncrementIteration()
	if session.CurrentIteration != 1 {
		t.Errorf("expected iteration 1, got %d", session.CurrentIteration)
	}

	session.IncrementIteration()
	if session.CurrentIteration != 2 {
		t.Errorf("expected iteration 2, got %d", session.CurrentIteration)
	}
}

func TestSessionSetInstanceID(t *testing.T) {
	session := NewSession("Test", nil)

	session.SetInstanceID("inst-1")
	if session.InstanceID != "inst-1" {
		t.Errorf("expected instance ID %q, got %q", "inst-1", session.InstanceID)
	}
	if len(session.InstanceIDs) != 1 || session.InstanceIDs[0] != "inst-1" {
		t.Error("expected inst-1 to be tracked in InstanceIDs")
	}

	session.SetInstanceID("inst-2")
	if session.InstanceID != "inst-2" {
		t.Errorf("expected instance ID %q, got %q", "inst-2", session.InstanceID)
	}
	if len(session.InstanceIDs) != 2 {
		t.Errorf("expected 2 tracked instance IDs, got %d", len(session.InstanceIDs))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxIterations != 50 {
		t.Errorf("expected MaxIterations 50, got %d", config.MaxIterations)
	}
	if config.CompletionPromise != "" {
		t.Errorf("expected empty CompletionPromise, got %q", config.CompletionPromise)
	}
}

var errGenericTest = errGeneric("test error")

type errGeneric string

func (e errGeneric) Error() string {
	return string(e)
}

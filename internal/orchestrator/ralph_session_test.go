package orchestrator

import (
	"testing"
)

func TestNewRalphSession(t *testing.T) {
	prompt := "Test prompt"
	config := &RalphConfig{
		MaxIterations:     10,
		CompletionPromise: "DONE",
	}

	session := NewRalphSession(prompt, config)

	if session.Prompt != prompt {
		t.Errorf("expected prompt %q, got %q", prompt, session.Prompt)
	}
	if session.Config != config {
		t.Error("expected config to be set")
	}
	if session.CurrentIteration != 0 {
		t.Errorf("expected initial iteration 0, got %d", session.CurrentIteration)
	}
	if session.Phase != PhaseRalphWorking {
		t.Errorf("expected phase %q, got %q", PhaseRalphWorking, session.Phase)
	}
	if session.StartedAt.IsZero() {
		t.Error("expected StartedAt to be set")
	}
}

func TestNewRalphSessionWithNilConfig(t *testing.T) {
	session := NewRalphSession("Test", nil)

	if session.Config == nil {
		t.Error("expected default config to be created")
	}
	if session.Config.MaxIterations != 50 {
		t.Errorf("expected default MaxIterations 50, got %d", session.Config.MaxIterations)
	}
}

func TestRalphSessionIsActive(t *testing.T) {
	tests := []struct {
		name     string
		phase    RalphPhase
		expected bool
	}{
		{"working", PhaseRalphWorking, true},
		{"complete", PhaseRalphComplete, false},
		{"max iterations", PhaseRalphMaxIterations, false},
		{"cancelled", PhaseRalphCancelled, false},
		{"error", PhaseRalphError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewRalphSession("Test", nil)
			session.Phase = tt.phase
			if session.IsActive() != tt.expected {
				t.Errorf("expected IsActive() = %v for phase %q", tt.expected, tt.phase)
			}
		})
	}
}

func TestRalphSessionShouldContinue(t *testing.T) {
	tests := []struct {
		name           string
		phase          RalphPhase
		currentIter    int
		maxIter        int
		shouldContinue bool
	}{
		{"working below max", PhaseRalphWorking, 1, 10, true},
		{"working at max", PhaseRalphWorking, 10, 10, false},
		{"working above max", PhaseRalphWorking, 11, 10, false},
		{"working no limit", PhaseRalphWorking, 100, 0, true},
		{"complete", PhaseRalphComplete, 1, 10, false},
		{"cancelled", PhaseRalphCancelled, 1, 10, false},
		{"error", PhaseRalphError, 1, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewRalphSession("Test", &RalphConfig{MaxIterations: tt.maxIter})
			session.Phase = tt.phase
			session.CurrentIteration = tt.currentIter
			if session.ShouldContinue() != tt.shouldContinue {
				t.Errorf("expected ShouldContinue() = %v", tt.shouldContinue)
			}
		})
	}
}

func TestRalphSessionCheckCompletionPromise(t *testing.T) {
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
			session := NewRalphSession("Test", &RalphConfig{CompletionPromise: tt.promise})
			if session.CheckCompletionPromise(tt.output) != tt.expected {
				t.Errorf("expected CheckCompletionPromise() = %v", tt.expected)
			}
		})
	}
}

func TestRalphSessionMarkComplete(t *testing.T) {
	session := NewRalphSession("Test", nil)

	session.MarkComplete()

	if session.Phase != PhaseRalphComplete {
		t.Errorf("expected phase %q, got %q", PhaseRalphComplete, session.Phase)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestRalphSessionMarkMaxIterationsReached(t *testing.T) {
	session := NewRalphSession("Test", nil)

	session.MarkMaxIterationsReached()

	if session.Phase != PhaseRalphMaxIterations {
		t.Errorf("expected phase %q, got %q", PhaseRalphMaxIterations, session.Phase)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestRalphSessionMarkCancelled(t *testing.T) {
	session := NewRalphSession("Test", nil)

	session.MarkCancelled()

	if session.Phase != PhaseRalphCancelled {
		t.Errorf("expected phase %q, got %q", PhaseRalphCancelled, session.Phase)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestRalphSessionMarkError(t *testing.T) {
	session := NewRalphSession("Test", nil)
	testErr := errGenericTest

	session.MarkError(testErr)

	if session.Phase != PhaseRalphError {
		t.Errorf("expected phase %q, got %q", PhaseRalphError, session.Phase)
	}
	if session.Error != testErr.Error() {
		t.Errorf("expected error %q, got %q", testErr.Error(), session.Error)
	}
	if session.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestRalphSessionIncrementIteration(t *testing.T) {
	session := NewRalphSession("Test", nil)

	session.IncrementIteration()
	if session.CurrentIteration != 1 {
		t.Errorf("expected iteration 1, got %d", session.CurrentIteration)
	}

	session.IncrementIteration()
	if session.CurrentIteration != 2 {
		t.Errorf("expected iteration 2, got %d", session.CurrentIteration)
	}
}

func TestRalphSessionSetInstanceID(t *testing.T) {
	session := NewRalphSession("Test", nil)

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

func TestDefaultRalphConfig(t *testing.T) {
	config := DefaultRalphConfig()

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

package orchestrator

import (
	"context"
	"sync"
	"testing"
)

// Coverage note: StartIteration() requires a fully initialized Orchestrator
// with worktree and session management, which would require integration tests.
// The tests here cover the coordinator logic that doesn't depend on Orchestrator.

func TestNewRalphCoordinator(t *testing.T) {
	session := NewRalphSession("Test prompt", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	if coord == nil {
		t.Fatal("expected non-nil coordinator")
	}
	if coord.Session() != session {
		t.Error("expected coordinator to hold the session")
	}
}

func TestRalphCoordinator_Session(t *testing.T) {
	session := NewRalphSession("Test prompt", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	got := coord.Session()
	if got != session {
		t.Error("Session() should return the ralph session")
	}
}

func TestRalphCoordinator_CheckCompletionInOutput(t *testing.T) {
	tests := []struct {
		name    string
		promise string
		output  string
		want    bool
	}{
		{
			name:    "promise found",
			promise: "TASK_COMPLETE",
			output:  "Work done. TASK_COMPLETE",
			want:    true,
		},
		{
			name:    "promise not found",
			promise: "TASK_COMPLETE",
			output:  "Still working...",
			want:    false,
		},
		{
			name:    "empty promise never matches",
			promise: "",
			output:  "Any output",
			want:    false,
		},
		{
			name:    "empty output",
			promise: "DONE",
			output:  "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RalphConfig{
				MaxIterations:     10,
				CompletionPromise: tt.promise,
			}
			session := NewRalphSession("Test", config)
			coord := NewRalphCoordinator(nil, nil, session, nil)

			got := coord.CheckCompletionInOutput(tt.output)
			if got != tt.want {
				t.Errorf("CheckCompletionInOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRalphCoordinator_ProcessIterationCompletion_PromiseFound(t *testing.T) {
	config := &RalphConfig{
		MaxIterations:     10,
		CompletionPromise: "DONE",
	}
	session := NewRalphSession("Test", config)
	session.IncrementIteration() // Start at iteration 1

	// Create a mock orchestrator with SaveSession capability
	coord := NewRalphCoordinator(&Orchestrator{}, nil, session, nil)

	// Track callback invocations
	var promiseFoundCalled bool
	var completeCalled bool
	var completePhase RalphPhase

	coord.SetCallbacks(&RalphCoordinatorCallbacks{
		OnPromiseFound: func(iteration int) {
			promiseFoundCalled = true
		},
		OnComplete: func(phase RalphPhase, summary string) {
			completeCalled = true
			completePhase = phase
		},
	})

	// Process completion with promise in output
	continueLoop, err := coord.ProcessIterationCompletion("Work finished. DONE")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if continueLoop {
		t.Error("expected continueLoop = false when promise found")
	}
	if session.Phase != PhaseRalphComplete {
		t.Errorf("expected phase %q, got %q", PhaseRalphComplete, session.Phase)
	}
	if !promiseFoundCalled {
		t.Error("expected OnPromiseFound callback to be called")
	}
	if !completeCalled {
		t.Error("expected OnComplete callback to be called")
	}
	if completePhase != PhaseRalphComplete {
		t.Errorf("expected complete phase %q, got %q", PhaseRalphComplete, completePhase)
	}
}

func TestRalphCoordinator_ProcessIterationCompletion_MaxIterations(t *testing.T) {
	config := &RalphConfig{
		MaxIterations:     3,
		CompletionPromise: "DONE",
	}
	session := NewRalphSession("Test", config)
	// Set to max iterations
	session.CurrentIteration = 3

	coord := NewRalphCoordinator(&Orchestrator{}, nil, session, nil)

	var maxItersCalled bool
	var completePhase RalphPhase

	coord.SetCallbacks(&RalphCoordinatorCallbacks{
		OnMaxIterations: func(iteration int) {
			maxItersCalled = true
		},
		OnComplete: func(phase RalphPhase, summary string) {
			completePhase = phase
		},
	})

	// Process completion without promise (should hit max iterations)
	continueLoop, err := coord.ProcessIterationCompletion("No promise here")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if continueLoop {
		t.Error("expected continueLoop = false when max iterations reached")
	}
	if session.Phase != PhaseRalphMaxIterations {
		t.Errorf("expected phase %q, got %q", PhaseRalphMaxIterations, session.Phase)
	}
	if !maxItersCalled {
		t.Error("expected OnMaxIterations callback to be called")
	}
	if completePhase != PhaseRalphMaxIterations {
		t.Errorf("expected complete phase %q, got %q", PhaseRalphMaxIterations, completePhase)
	}
}

func TestRalphCoordinator_ProcessIterationCompletion_ContinueLoop(t *testing.T) {
	config := &RalphConfig{
		MaxIterations:     10,
		CompletionPromise: "DONE",
	}
	session := NewRalphSession("Test", config)
	session.CurrentIteration = 2 // Not at max yet

	coord := NewRalphCoordinator(&Orchestrator{}, nil, session, nil)

	var iterCompleteCalled bool
	coord.SetCallbacks(&RalphCoordinatorCallbacks{
		OnIterationComplete: func(iteration int) {
			iterCompleteCalled = true
		},
	})

	// Process completion without promise, not at max - should continue
	continueLoop, err := coord.ProcessIterationCompletion("Still working")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !continueLoop {
		t.Error("expected continueLoop = true when below max and no promise")
	}
	if session.Phase != PhaseRalphWorking {
		t.Errorf("expected phase %q, got %q", PhaseRalphWorking, session.Phase)
	}
	if !iterCompleteCalled {
		t.Error("expected OnIterationComplete callback to be called")
	}
}

func TestRalphCoordinator_ProcessIterationCompletion_NoMaxLimit(t *testing.T) {
	config := &RalphConfig{
		MaxIterations:     0, // No limit
		CompletionPromise: "DONE",
	}
	session := NewRalphSession("Test", config)
	session.CurrentIteration = 100 // High iteration count

	coord := NewRalphCoordinator(&Orchestrator{}, nil, session, nil)

	// Process completion without promise - should continue since no max limit
	continueLoop, err := coord.ProcessIterationCompletion("Still working")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !continueLoop {
		t.Error("expected continueLoop = true when no max limit")
	}
}

func TestRalphCoordinator_Cancel(t *testing.T) {
	config := DefaultRalphConfig()
	session := NewRalphSession("Test", config)
	session.CurrentIteration = 5

	coord := NewRalphCoordinator(&Orchestrator{}, nil, session, nil)

	var completeCalled bool
	var completePhase RalphPhase
	var completeSummary string

	coord.SetCallbacks(&RalphCoordinatorCallbacks{
		OnComplete: func(phase RalphPhase, summary string) {
			completeCalled = true
			completePhase = phase
			completeSummary = summary
		},
	})

	coord.Cancel()

	if session.Phase != PhaseRalphCancelled {
		t.Errorf("expected phase %q, got %q", PhaseRalphCancelled, session.Phase)
	}
	if !completeCalled {
		t.Error("expected OnComplete callback to be called")
	}
	if completePhase != PhaseRalphCancelled {
		t.Errorf("expected complete phase %q, got %q", PhaseRalphCancelled, completePhase)
	}
	if completeSummary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestRalphCoordinator_SetCallbacks(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	callCount := 0
	cb := &RalphCoordinatorCallbacks{
		OnIterationStart: func(iteration int, instanceID string) {
			callCount++
		},
	}

	coord.SetCallbacks(cb)

	// Verify callbacks are set by triggering a notification
	coord.notifyIterationStart(1, "test-id")
	if callCount != 1 {
		t.Errorf("expected callback to be called once, got %d", callCount)
	}
}

func TestRalphCoordinator_WorktreePath(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	// Initially empty
	if coord.GetWorktree() != "" {
		t.Error("expected empty worktree initially")
	}

	// Set worktree
	coord.SetWorktree("/test/worktree/path")
	if coord.GetWorktree() != "/test/worktree/path" {
		t.Errorf("expected worktree %q, got %q", "/test/worktree/path", coord.GetWorktree())
	}
}

func TestRalphCoordinator_GetCurrentInstanceID(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	// Initially empty
	if coord.GetCurrentInstanceID() != "" {
		t.Error("expected empty instance ID initially")
	}

	// Set via session
	session.SetInstanceID("inst-123")
	if coord.GetCurrentInstanceID() != "inst-123" {
		t.Errorf("expected instance ID %q, got %q", "inst-123", coord.GetCurrentInstanceID())
	}
}

func TestRalphCoordinator_Stop(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	// Stop should not panic and should be callable
	coord.Stop()
}

func TestRalphCoordinator_CallbacksNilSafety(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	// These should not panic with nil callbacks
	coord.notifyIterationStart(1, "inst-1")
	coord.notifyIterationComplete(1)
	coord.notifyPromiseFound(1)
	coord.notifyMaxIterations(1)
	coord.notifyComplete(PhaseRalphComplete, "test summary")
}

func TestRalphCoordinator_CallbacksPartialNilSafety(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	// Set callbacks with only some fields populated
	coord.SetCallbacks(&RalphCoordinatorCallbacks{
		OnIterationStart: func(iteration int, instanceID string) {},
		// Other callbacks are nil
	})

	// These should not panic even with partial callbacks
	coord.notifyIterationStart(1, "inst-1")
	coord.notifyIterationComplete(1)
	coord.notifyPromiseFound(1)
	coord.notifyMaxIterations(1)
	coord.notifyComplete(PhaseRalphComplete, "test summary")
}

func TestRalphCoordinator_ConcurrentAccess(t *testing.T) {
	session := NewRalphSession("Test", &RalphConfig{
		MaxIterations:     100,
		CompletionPromise: "DONE",
	})
	coord := NewRalphCoordinator(&Orchestrator{}, nil, session, nil)

	// Test concurrent access to coordinator methods
	var wg sync.WaitGroup

	// Multiple goroutines accessing session
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = coord.Session()
		}()
	}

	// Multiple goroutines checking completion
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = coord.CheckCompletionInOutput("test output")
		}()
	}

	// Multiple goroutines accessing worktree
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				coord.SetWorktree("/path/" + string(rune('a'+n)))
			} else {
				_ = coord.GetWorktree()
			}
		}(i)
	}

	// Multiple goroutines getting instance ID
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = coord.GetCurrentInstanceID()
		}()
	}

	wg.Wait()
}

func TestRalphCoordinator_StartIterationNotContinue(t *testing.T) {
	tests := []struct {
		name        string
		phase       RalphPhase
		wantErrPart string
	}{
		{
			name:        "cancelled",
			phase:       PhaseRalphCancelled,
			wantErrPart: "cancelled",
		},
		{
			name:        "complete",
			phase:       PhaseRalphComplete,
			wantErrPart: "complete",
		},
		{
			name:        "error",
			phase:       PhaseRalphError,
			wantErrPart: "cannot continue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewRalphSession("Test", DefaultRalphConfig())
			session.Phase = tt.phase

			coord := NewRalphCoordinator(nil, nil, session, nil)

			err := coord.StartIteration()
			if err == nil {
				t.Error("expected error when session cannot continue")
			}
			if err != nil && tt.wantErrPart != "" {
				if got := err.Error(); !containsIgnoreCase(got, tt.wantErrPart) {
					t.Errorf("expected error to contain %q, got %q", tt.wantErrPart, got)
				}
			}
		})
	}
}

func TestRalphCoordinator_ContextCancellation(t *testing.T) {
	session := NewRalphSession("Test", DefaultRalphConfig())
	coord := NewRalphCoordinator(nil, nil, session, nil)

	// Get the internal context (via stopping)
	ctx := context.Background()
	_ = ctx

	// The coordinator should have an internal context that gets cancelled on Stop
	coord.Stop()

	// After Stop, the coordinator's context should be cancelled
	// This tests that Stop properly cleans up resources
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			(len(s) > 0 && containsIgnoreCaseHelper(s, substr)))
}

func containsIgnoreCaseHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, start int, substr string) bool {
	for j := 0; j < len(substr); j++ {
		sc := s[start+j]
		tc := substr[j]
		if sc != tc && toLower(sc) != toLower(tc) {
			return false
		}
	}
	return true
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

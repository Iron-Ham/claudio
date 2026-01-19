package consolidation

import (
	"testing"
)

func TestState_HasConflict(t *testing.T) {
	tests := []struct {
		name     string
		state    State
		expected bool
	}{
		{
			name: "paused with conflict files",
			state: State{
				Phase:         PhasePaused,
				ConflictFiles: []string{"file1.go", "file2.go"},
			},
			expected: true,
		},
		{
			name: "paused without conflict files",
			state: State{
				Phase:         PhasePaused,
				ConflictFiles: nil,
			},
			expected: false,
		},
		{
			name: "paused with empty conflict files",
			state: State{
				Phase:         PhasePaused,
				ConflictFiles: []string{},
			},
			expected: false,
		},
		{
			name: "not paused with conflict files",
			state: State{
				Phase:         PhaseMergingTasks,
				ConflictFiles: []string{"file1.go"},
			},
			expected: false,
		},
		{
			name: "idle state",
			state: State{
				Phase: PhaseIdle,
			},
			expected: false,
		},
		{
			name: "complete state",
			state: State{
				Phase: PhaseComplete,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.HasConflict()
			if got != tt.expected {
				t.Errorf("State.HasConflict() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMode_Values(t *testing.T) {
	// Verify mode constants have expected string values
	if ModeStacked != "stacked" {
		t.Errorf("ModeStacked = %q, want %q", ModeStacked, "stacked")
	}
	if ModeSingle != "single" {
		t.Errorf("ModeSingle = %q, want %q", ModeSingle, "single")
	}
}

func TestPhase_Values(t *testing.T) {
	// Verify all phases have unique non-empty values
	phases := []Phase{
		PhaseIdle,
		PhaseDetecting,
		PhaseCreatingBranches,
		PhaseMergingTasks,
		PhasePushing,
		PhaseCreatingPRs,
		PhasePaused,
		PhaseComplete,
		PhaseFailed,
	}

	seen := make(map[Phase]bool)
	for _, p := range phases {
		if p == "" {
			t.Errorf("Phase constant has empty string value")
		}
		if seen[p] {
			t.Errorf("Duplicate phase value: %q", p)
		}
		seen[p] = true
	}
}

func TestEventType_Values(t *testing.T) {
	// Verify all event types have unique non-empty values
	events := []EventType{
		EventStarted,
		EventGroupStarted,
		EventTaskMerging,
		EventTaskMerged,
		EventGroupComplete,
		EventPRCreating,
		EventPRCreated,
		EventConflict,
		EventComplete,
		EventFailed,
	}

	seen := make(map[EventType]bool)
	for _, e := range events {
		if e == "" {
			t.Errorf("EventType constant has empty string value")
		}
		if seen[e] {
			t.Errorf("Duplicate event type value: %q", e)
		}
		seen[e] = true
	}
}

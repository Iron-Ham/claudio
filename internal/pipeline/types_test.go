package pipeline

import "testing"

func TestPipelinePhase_String(t *testing.T) {
	tests := []struct {
		phase PipelinePhase
		want  string
	}{
		{PhasePlanning, "planning"},
		{PhaseExecution, "execution"},
		{PhaseReview, "review"},
		{PhaseConsolidation, "consolidation"},
		{PhaseDone, "done"},
		{PhaseFailed, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPipelinePhase_IsTerminal(t *testing.T) {
	tests := []struct {
		phase    PipelinePhase
		terminal bool
	}{
		{PhasePlanning, false},
		{PhaseExecution, false},
		{PhaseReview, false},
		{PhaseConsolidation, false},
		{PhaseDone, true},
		{PhaseFailed, true},
	}
	for _, tt := range tests {
		t.Run(tt.phase.String(), func(t *testing.T) {
			if got := tt.phase.IsTerminal(); got != tt.terminal {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.terminal)
			}
		})
	}
}

func TestDecomposeConfig_Defaults(t *testing.T) {
	t.Run("zero MinTeamSize becomes 1", func(t *testing.T) {
		cfg := DecomposeConfig{MinTeamSize: 0}
		got := cfg.defaults()
		if got.MinTeamSize != 1 {
			t.Errorf("MinTeamSize = %d, want 1", got.MinTeamSize)
		}
	})

	t.Run("negative MinTeamSize becomes 1", func(t *testing.T) {
		cfg := DecomposeConfig{MinTeamSize: -5}
		got := cfg.defaults()
		if got.MinTeamSize != 1 {
			t.Errorf("MinTeamSize = %d, want 1", got.MinTeamSize)
		}
	})

	t.Run("positive MinTeamSize preserved", func(t *testing.T) {
		cfg := DecomposeConfig{MinTeamSize: 3}
		got := cfg.defaults()
		if got.MinTeamSize != 3 {
			t.Errorf("MinTeamSize = %d, want 3", got.MinTeamSize)
		}
	})
}

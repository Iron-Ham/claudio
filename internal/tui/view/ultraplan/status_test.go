package ultraplan

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestRenderProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		percent  int
		width    int
		expected string
	}{
		{"0 percent", 0, 10, "[░░░░░░░░░░]"},
		{"50 percent", 50, 10, "[█████░░░░░]"},
		{"100 percent", 100, 10, "[██████████]"},
		{"negative clamped to 0", -10, 10, "[░░░░░░░░░░]"},
		{"over 100 clamped to 100", 150, 10, "[██████████]"},
		{"narrow width", 50, 4, "[██░░]"},
		{"single width", 50, 2, "[█░]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderProgressBar(tt.percent, tt.width)
			if got != tt.expected {
				t.Errorf("RenderProgressBar(%d, %d) = %q, want %q", tt.percent, tt.width, got, tt.expected)
			}
		})
	}
}

func TestPhaseToString(t *testing.T) {
	tests := []struct {
		phase    orchestrator.UltraPlanPhase
		expected string
	}{
		{orchestrator.PhasePlanning, "PLANNING"},
		{orchestrator.PhasePlanSelection, "SELECTING PLAN"},
		{orchestrator.PhaseRefresh, "READY"},
		{orchestrator.PhaseExecuting, "EXECUTING"},
		{orchestrator.PhaseSynthesis, "SYNTHESIS"},
		{orchestrator.PhaseRevision, "REVISION"},
		{orchestrator.PhaseConsolidating, "CONSOLIDATING"},
		{orchestrator.PhaseComplete, "COMPLETE"},
		{orchestrator.PhaseFailed, "FAILED"},
	}

	for _, tt := range tests {
		t.Run(string(tt.phase), func(t *testing.T) {
			got := PhaseToString(tt.phase)
			if got != tt.expected {
				t.Errorf("PhaseToString(%v) = %q, want %q", tt.phase, got, tt.expected)
			}
		})
	}
}

func TestPhaseStyle(t *testing.T) {
	// Test that PhaseStyle returns non-nil styles for all phases
	phases := []orchestrator.UltraPlanPhase{
		orchestrator.PhasePlanning,
		orchestrator.PhasePlanSelection,
		orchestrator.PhaseRefresh,
		orchestrator.PhaseExecuting,
		orchestrator.PhaseSynthesis,
		orchestrator.PhaseRevision,
		orchestrator.PhaseConsolidating,
		orchestrator.PhaseComplete,
		orchestrator.PhaseFailed,
	}

	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			style := PhaseStyle(phase)
			// Just verify we can render with it (non-nil)
			result := style.Render("test")
			if result == "" {
				t.Errorf("PhaseStyle(%v).Render returned empty string", phase)
			}
		})
	}
}

func TestComplexityIndicator(t *testing.T) {
	tests := []struct {
		complexity orchestrator.TaskComplexity
		expected   string
	}{
		{orchestrator.ComplexityLow, "◦"},
		{orchestrator.ComplexityMedium, "◎"},
		{orchestrator.ComplexityHigh, "●"},
		{orchestrator.TaskComplexity("unknown"), "○"},
	}

	for _, tt := range tests {
		t.Run(string(tt.complexity), func(t *testing.T) {
			got := ComplexityIndicator(tt.complexity)
			if got != tt.expected {
				t.Errorf("ComplexityIndicator(%v) = %q, want %q", tt.complexity, got, tt.expected)
			}
		})
	}
}

func TestStatusRenderer_GetPhaseSectionStatus(t *testing.T) {
	tests := []struct {
		name     string
		phase    orchestrator.UltraPlanPhase
		session  *orchestrator.UltraPlanSession
		expected string
	}{
		{
			name:  "planning in progress",
			phase: orchestrator.PhasePlanning,
			session: &orchestrator.UltraPlanSession{
				Phase: orchestrator.PhasePlanning,
			},
			expected: "[⟳]",
		},
		{
			name:  "planning complete with plan",
			phase: orchestrator.PhasePlanning,
			session: &orchestrator.UltraPlanSession{
				Phase: orchestrator.PhaseExecuting,
				Plan:  &orchestrator.PlanSpec{},
			},
			expected: "[✓]",
		},
		{
			name:  "execution pending",
			phase: orchestrator.PhaseExecuting,
			session: &orchestrator.UltraPlanSession{
				Phase: orchestrator.PhasePlanning,
				Plan:  nil,
			},
			expected: "[○]",
		},
		{
			name:  "execution in progress",
			phase: orchestrator.PhaseExecuting,
			session: &orchestrator.UltraPlanSession{
				Phase: orchestrator.PhaseExecuting,
				Plan: &orchestrator.PlanSpec{
					Tasks: []orchestrator.PlannedTask{{ID: "1"}, {ID: "2"}},
				},
				CompletedTasks: []string{"1"},
			},
			expected: "[1/2]",
		},
		{
			name:  "synthesis in progress",
			phase: orchestrator.PhaseSynthesis,
			session: &orchestrator.UltraPlanSession{
				Phase: orchestrator.PhaseSynthesis,
			},
			expected: "[⟳]",
		},
		{
			name:  "consolidation complete",
			phase: orchestrator.PhaseConsolidating,
			session: &orchestrator.UltraPlanSession{
				Phase:  orchestrator.PhaseComplete,
				PRUrls: []string{"https://github.com/example/pr/1"},
			},
			expected: "[✓]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &RenderContext{
				UltraPlan: &State{},
			}
			s := NewStatusRenderer(ctx)
			got := s.GetPhaseSectionStatus(tt.phase, tt.session)
			if got != tt.expected {
				t.Errorf("GetPhaseSectionStatus(%v) = %q, want %q", tt.phase, got, tt.expected)
			}
		})
	}
}

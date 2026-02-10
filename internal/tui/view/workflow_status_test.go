package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

func TestWorkflowStatusState_HasActiveWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		state    *WorkflowStatusState
		expected bool
	}{
		{
			name:     "nil state returns false",
			state:    nil,
			expected: false,
		},
		{
			name:     "empty state returns false",
			state:    &WorkflowStatusState{},
			expected: false,
		},
		{
			name: "ultraplan state without coordinator returns false",
			state: &WorkflowStatusState{
				UltraPlan: &UltraPlanState{},
			},
			expected: false,
		},
		{
			name: "tripleshot active returns true",
			state: &WorkflowStatusState{
				TripleShot: &TripleShotState{
					Runners: map[string]tripleshot.Runner{
						"group1": createMockTripleShotCoordinator(),
					},
				},
			},
			expected: true,
		},
		{
			name: "adversarial active returns true",
			state: &WorkflowStatusState{
				Adversarial: &AdversarialState{
					Coordinators: map[string]*adversarial.Coordinator{
						"group1": {},
					},
				},
			},
			expected: true,
		},
		{
			name: "tripleshot and adversarial active returns true",
			state: &WorkflowStatusState{
				TripleShot: &TripleShotState{
					Runners: map[string]tripleshot.Runner{
						"group1": createMockTripleShotCoordinator(),
					},
				},
				Adversarial: &AdversarialState{
					Coordinators: map[string]*adversarial.Coordinator{
						"group1": {},
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.HasActiveWorkflows()
			if got != tt.expected {
				t.Errorf("HasActiveWorkflows() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkflowStatusState_GetIndicators(t *testing.T) {
	tests := []struct {
		name          string
		state         *WorkflowStatusState
		expectedCount int
	}{
		{
			name:          "nil state returns empty",
			state:         nil,
			expectedCount: 0,
		},
		{
			name:          "empty state returns empty",
			state:         &WorkflowStatusState{},
			expectedCount: 0,
		},
		{
			name: "single tripleshot returns one indicator",
			state: &WorkflowStatusState{
				TripleShot: createMockTripleShotState(1),
			},
			expectedCount: 1,
		},
		{
			name: "single adversarial returns one indicator",
			state: &WorkflowStatusState{
				Adversarial: createMockAdversarialState(1),
			},
			expectedCount: 1,
		},
		{
			name: "tripleshot and adversarial active returns two indicators",
			state: &WorkflowStatusState{
				TripleShot:  createMockTripleShotState(1),
				Adversarial: createMockAdversarialState(1),
			},
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			indicators := tt.state.GetIndicators()
			if len(indicators) != tt.expectedCount {
				t.Errorf("GetIndicators() returned %d indicators, want %d", len(indicators), tt.expectedCount)
			}
		})
	}
}

func TestWorkflowStatusState_IndicatorOrder(t *testing.T) {
	// Verify indicators are returned in priority order: Pipeline, UltraPlan, TripleShot, Adversarial
	// This test verifies TripleShot and Adversarial order; see TestGetUltraPlanIndicator for UltraPlan tests
	state := &WorkflowStatusState{
		TripleShot:  createMockTripleShotState(1),
		Adversarial: createMockAdversarialState(1),
	}

	indicators := state.GetIndicators()
	if len(indicators) != 2 {
		t.Fatalf("Expected 2 indicators, got %d", len(indicators))
	}

	// First should be TripleShot (△ icon)
	if indicators[0].Icon != "△" {
		t.Errorf("First indicator should be TripleShot (△), got %s", indicators[0].Icon)
	}

	// Second should be Adversarial (🔨, 🔍, ✓, or ✗ icon)
	advIcons := []string{"🔨", "🔍", "✓", "✗"}
	found := false
	for _, icon := range advIcons {
		if indicators[1].Icon == icon {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Second indicator should be Adversarial icon, got %s", indicators[1].Icon)
	}
}

func TestWorkflowStatusState_HasActiveWorkflows_WithPipeline(t *testing.T) {
	tests := []struct {
		name     string
		state    *WorkflowStatusState
		expected bool
	}{
		{
			name: "active pipeline returns true",
			state: &WorkflowStatusState{
				Pipeline: &PipelineState{Phase: "execution"},
			},
			expected: true,
		},
		{
			name: "completed pipeline returns false",
			state: &WorkflowStatusState{
				Pipeline: &PipelineState{Phase: "done"},
			},
			expected: false,
		},
		{
			name: "pipeline with no phase returns false",
			state: &WorkflowStatusState{
				Pipeline: &PipelineState{},
			},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.HasActiveWorkflows(); got != tt.expected {
				t.Errorf("HasActiveWorkflows() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestWorkflowStatusState_PipelineIndicatorFirst(t *testing.T) {
	state := &WorkflowStatusState{
		Pipeline:   &PipelineState{Phase: "execution"},
		TripleShot: createMockTripleShotState(1),
	}

	indicators := state.GetIndicators()
	if len(indicators) != 2 {
		t.Fatalf("Expected 2 indicators, got %d", len(indicators))
	}

	// Pipeline should be first
	if indicators[0].Icon != styles.IconPipeline {
		t.Errorf("First indicator should be Pipeline (%s), got %s", styles.IconPipeline, indicators[0].Icon)
	}

	// TripleShot should be second
	if indicators[1].Icon != "△" {
		t.Errorf("Second indicator should be TripleShot (△), got %s", indicators[1].Icon)
	}
}

func TestRenderWorkflowStatus_WithPipeline(t *testing.T) {
	state := &WorkflowStatusState{
		Pipeline:   &PipelineState{Phase: "planning"},
		TripleShot: createMockTripleShotState(1),
	}

	result := RenderWorkflowStatus(state)
	if !strings.Contains(result, styles.IconPipeline) {
		t.Error("Expected Pipeline icon in result")
	}
	if !strings.Contains(result, "△") {
		t.Error("Expected TripleShot icon in result")
	}
	if !strings.Contains(result, "│") {
		t.Error("Expected separator between Pipeline and TripleShot")
	}
}

func TestRenderWorkflowStatus(t *testing.T) {
	tests := []struct {
		name            string
		state           *WorkflowStatusState
		expectEmpty     bool
		expectContains  []string
		expectSeparator bool
	}{
		{
			name:        "nil state returns empty",
			state:       nil,
			expectEmpty: true,
		},
		{
			name:        "empty state returns empty",
			state:       &WorkflowStatusState{},
			expectEmpty: true,
		},
		{
			name: "single tripleshot shows no separator",
			state: &WorkflowStatusState{
				TripleShot: createMockTripleShotState(1),
			},
			expectEmpty:     false,
			expectContains:  []string{"△"},
			expectSeparator: false,
		},
		{
			name: "multiple workflows shows separator",
			state: &WorkflowStatusState{
				TripleShot:  createMockTripleShotState(1),
				Adversarial: createMockAdversarialState(1),
			},
			expectEmpty:     false,
			expectContains:  []string{"△", "🔨", "│"},
			expectSeparator: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderWorkflowStatus(tt.state)

			if tt.expectEmpty {
				if result != "" {
					t.Errorf("Expected empty string, got %q", result)
				}
				return
			}

			if result == "" {
				t.Error("Expected non-empty string, got empty")
				return
			}

			for _, contains := range tt.expectContains {
				if !strings.Contains(result, contains) {
					t.Errorf("Expected result to contain %q, got %q", contains, result)
				}
			}

			hasSeparator := strings.Contains(result, "│")
			if tt.expectSeparator != hasSeparator {
				t.Errorf("Separator expectation mismatch: expected %v, got %v in %q", tt.expectSeparator, hasSeparator, result)
			}
		})
	}
}

func TestTripleShotIndicator_MultipleSessionsSummary(t *testing.T) {
	// Test that multiple tripleshot sessions show a summary count
	state := &WorkflowStatusState{
		TripleShot: createMockTripleShotState(3),
	}

	indicators := state.GetIndicators()
	if len(indicators) != 1 {
		t.Fatalf("Expected 1 indicator, got %d", len(indicators))
	}

	ind := indicators[0]
	if ind.Count != 3 {
		t.Errorf("Expected count of 3, got %d", ind.Count)
	}
}

func TestAdversarialIndicator_MultipleSessionsSummary(t *testing.T) {
	// Test that multiple adversarial sessions show a summary count
	state := &WorkflowStatusState{
		Adversarial: createMockAdversarialState(2),
	}

	indicators := state.GetIndicators()
	if len(indicators) != 1 {
		t.Fatalf("Expected 1 indicator, got %d", len(indicators))
	}

	ind := indicators[0]
	if ind.Count != 2 {
		t.Errorf("Expected count of 2, got %d", ind.Count)
	}
}

func TestWorkflowStatusView_Render(t *testing.T) {
	v := NewWorkflowStatusView()

	state := &WorkflowStatusState{
		TripleShot: createMockTripleShotState(1),
	}

	result := v.Render(state)
	if result == "" {
		t.Error("Expected non-empty render result")
	}

	if !strings.Contains(result, "△") {
		t.Error("Expected TripleShot indicator icon in render result")
	}
}

// Helper functions to create mock states for testing

func createMockTripleShotState(count int) *TripleShotState {
	runners := make(map[string]tripleshot.Runner)
	for i := 0; i < count; i++ {
		groupID := string(rune('a' + i))
		runners[groupID] = createMockTripleShotCoordinator()
	}
	return &TripleShotState{
		Runners: runners,
	}
}

func createMockTripleShotCoordinator() *tripleshot.Coordinator {
	session := &tripleshot.Session{
		Phase: tripleshot.PhaseWorking,
		Attempts: [3]tripleshot.Attempt{
			{Status: tripleshot.AttemptStatusWorking},
			{Status: tripleshot.AttemptStatusPending},
			{Status: tripleshot.AttemptStatusPending},
		},
	}
	cfg := tripleshot.CoordinatorConfig{
		TripleSession: session,
	}
	return tripleshot.NewCoordinator(cfg)
}

func createMockAdversarialState(count int) *AdversarialState {
	coordinators := make(map[string]*adversarial.Coordinator)
	for i := 0; i < count; i++ {
		groupID := string(rune('a' + i))
		coordinators[groupID] = createMockAdversarialCoordinator()
	}
	return &AdversarialState{
		Coordinators: coordinators,
	}
}

func createMockAdversarialCoordinator() *adversarial.Coordinator {
	session := &adversarial.Session{
		Phase:        adversarial.PhaseImplementing,
		CurrentRound: 1,
		Config: adversarial.Config{
			MaxIterations: 5,
		},
	}
	cfg := adversarial.CoordinatorConfig{
		AdvSession: session,
	}
	return adversarial.NewCoordinator(cfg)
}

// createMockUltraPlanState creates a mock UltraPlanState with a coordinator
// that has a session with the given phase and objective.
func createMockUltraPlanState(phase orchestrator.UltraPlanPhase, objective string) *UltraPlanState {
	ultraSession := &orchestrator.UltraPlanSession{
		ID:        "test-ultraplan-session",
		Phase:     phase,
		Objective: objective,
	}
	coordinator := orchestrator.NewCoordinatorForTesting(ultraSession)
	return &UltraPlanState{
		Coordinator: coordinator,
	}
}

// createMockUltraPlanStateWithConsolidation creates a mock UltraPlanState
// with consolidation progress information.
func createMockUltraPlanStateWithConsolidation(currentGroup, totalGroups int, objective string) *UltraPlanState {
	ultraSession := &orchestrator.UltraPlanSession{
		ID:        "test-ultraplan-session",
		Phase:     orchestrator.PhaseConsolidating,
		Objective: objective,
		Consolidation: &orchestrator.ConsolidatorState{
			CurrentGroup: currentGroup,
			TotalGroups:  totalGroups,
		},
	}
	coordinator := orchestrator.NewCoordinatorForTesting(ultraSession)
	return &UltraPlanState{
		Coordinator: coordinator,
	}
}

func TestGetUltraPlanIndicator(t *testing.T) {
	tests := []struct {
		name              string
		state             *WorkflowStatusState
		expectNil         bool
		expectIcon        string
		expectLabelPrefix string
		expectObjective   string
	}{
		{
			name:      "nil state returns nil",
			state:     nil,
			expectNil: true,
		},
		{
			name:      "empty state returns nil",
			state:     &WorkflowStatusState{},
			expectNil: true,
		},
		{
			name: "ultraplan without coordinator returns nil",
			state: &WorkflowStatusState{
				UltraPlan: &UltraPlanState{},
			},
			expectNil: true,
		},
		{
			name: "planning phase shows correct indicator",
			state: &WorkflowStatusState{
				UltraPlan: createMockUltraPlanState(orchestrator.PhasePlanning, "Implement user auth"),
			},
			expectNil:         false,
			expectIcon:        styles.IconUltraPlan,
			expectLabelPrefix: "PLANNING",
			expectObjective:   "Implement user auth",
		},
		{
			name: "executing phase shows progress",
			state: &WorkflowStatusState{
				UltraPlan: createMockUltraPlanState(orchestrator.PhaseExecuting, "Add new feature"),
			},
			expectNil:         false,
			expectIcon:        styles.IconUltraPlan,
			expectLabelPrefix: "EXECUTING",
			expectObjective:   "Add new feature",
		},
		{
			name: "consolidating phase shows group progress",
			state: &WorkflowStatusState{
				UltraPlan: createMockUltraPlanStateWithConsolidation(2, 5, "Refactor API"),
			},
			expectNil:         false,
			expectIcon:        styles.IconUltraPlan,
			expectLabelPrefix: "CONSOLIDATING",
			expectObjective:   "Refactor API",
		},
		{
			name: "complete phase shows done",
			state: &WorkflowStatusState{
				UltraPlan: createMockUltraPlanState(orchestrator.PhaseComplete, "Fix bugs"),
			},
			expectNil:         false,
			expectIcon:        styles.IconUltraPlan,
			expectLabelPrefix: "COMPLETE",
			expectObjective:   "Fix bugs",
		},
		{
			name: "failed phase shows failed",
			state: &WorkflowStatusState{
				UltraPlan: createMockUltraPlanState(orchestrator.PhaseFailed, "Update tests"),
			},
			expectNil:         false,
			expectIcon:        styles.IconUltraPlan,
			expectLabelPrefix: "FAILED",
			expectObjective:   "Update tests",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var indicator *WorkflowIndicator
			if tt.state != nil {
				indicator = tt.state.getUltraPlanIndicator()
			}

			if tt.expectNil {
				if indicator != nil {
					t.Errorf("expected nil indicator, got %+v", indicator)
				}
				return
			}

			if indicator == nil {
				t.Fatal("expected non-nil indicator, got nil")
			}

			if indicator.Icon != tt.expectIcon {
				t.Errorf("Icon = %q, want %q", indicator.Icon, tt.expectIcon)
			}

			if !strings.HasPrefix(indicator.Label, tt.expectLabelPrefix) {
				t.Errorf("Label = %q, want prefix %q", indicator.Label, tt.expectLabelPrefix)
			}

			if indicator.Objective != tt.expectObjective {
				t.Errorf("Objective = %q, want %q", indicator.Objective, tt.expectObjective)
			}

			// Ultraplan is always single-session, so Count should be 0
			if indicator.Count != 0 {
				t.Errorf("Count = %d, want 0 (ultraplan is single-session)", indicator.Count)
			}
		})
	}
}

func TestGetUltraPlanObjective(t *testing.T) {
	tests := []struct {
		name     string
		state    *WorkflowStatusState
		expected string
	}{
		{
			name:     "nil state returns empty",
			state:    nil,
			expected: "",
		},
		{
			name:     "empty state returns empty",
			state:    &WorkflowStatusState{},
			expected: "",
		},
		{
			name: "ultraplan without coordinator returns empty",
			state: &WorkflowStatusState{
				UltraPlan: &UltraPlanState{},
			},
			expected: "",
		},
		{
			name: "ultraplan with coordinator returns objective",
			state: &WorkflowStatusState{
				UltraPlan: createMockUltraPlanState(orchestrator.PhasePlanning, "Implement feature X"),
			},
			expected: "Implement feature X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if tt.state != nil {
				got = tt.state.GetUltraPlanObjective()
			}
			if got != tt.expected {
				t.Errorf("GetUltraPlanObjective() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestUltraPlanIndicatorInGetIndicators(t *testing.T) {
	// Verify that ultraplan indicator appears after pipeline but before tripleshot/adversarial
	state := &WorkflowStatusState{
		UltraPlan:   createMockUltraPlanState(orchestrator.PhaseExecuting, "Test objective"),
		TripleShot:  createMockTripleShotState(1),
		Adversarial: createMockAdversarialState(1),
	}

	indicators := state.GetIndicators()
	if len(indicators) != 3 {
		t.Fatalf("Expected 3 indicators, got %d", len(indicators))
	}

	// First should be UltraPlan (no pipeline active)
	if indicators[0].Icon != styles.IconUltraPlan {
		t.Errorf("First indicator should be UltraPlan (%s), got %s", styles.IconUltraPlan, indicators[0].Icon)
	}

	// Second should be TripleShot (△ icon)
	if indicators[1].Icon != "△" {
		t.Errorf("Second indicator should be TripleShot (△), got %s", indicators[1].Icon)
	}

	// Third should be Adversarial
	advIcons := []string{"🔨", "🔍", "✓", "✗"}
	found := false
	for _, icon := range advIcons {
		if indicators[2].Icon == icon {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Third indicator should be Adversarial icon, got %s", indicators[2].Icon)
	}
}

func TestAllFourWorkflowsInGetIndicators(t *testing.T) {
	// Verify order: Pipeline, UltraPlan, TripleShot, Adversarial
	state := &WorkflowStatusState{
		Pipeline:    &PipelineState{Phase: "execution"},
		UltraPlan:   createMockUltraPlanState(orchestrator.PhaseExecuting, "Test"),
		TripleShot:  createMockTripleShotState(1),
		Adversarial: createMockAdversarialState(1),
	}

	indicators := state.GetIndicators()
	if len(indicators) != 4 {
		t.Fatalf("Expected 4 indicators, got %d", len(indicators))
	}

	if indicators[0].Icon != styles.IconPipeline {
		t.Errorf("First indicator should be Pipeline (%s), got %s", styles.IconPipeline, indicators[0].Icon)
	}
	if indicators[1].Icon != styles.IconUltraPlan {
		t.Errorf("Second indicator should be UltraPlan (%s), got %s", styles.IconUltraPlan, indicators[1].Icon)
	}
	if indicators[2].Icon != "△" {
		t.Errorf("Third indicator should be TripleShot (△), got %s", indicators[2].Icon)
	}
}

// Phase-parameterized mock helpers for testing mixed-phase scenarios

func createMockTripleShotCoordinatorWithPhase(phase tripleshot.Phase) *tripleshot.Coordinator {
	session := &tripleshot.Session{
		Phase: phase,
		Attempts: [3]tripleshot.Attempt{
			{Status: tripleshot.AttemptStatusWorking},
			{Status: tripleshot.AttemptStatusPending},
			{Status: tripleshot.AttemptStatusPending},
		},
	}
	cfg := tripleshot.CoordinatorConfig{
		TripleSession: session,
	}
	return tripleshot.NewCoordinator(cfg)
}

func createMockAdversarialCoordinatorWithPhase(phase adversarial.Phase) *adversarial.Coordinator {
	session := &adversarial.Session{
		Phase:        phase,
		CurrentRound: 1,
		Config: adversarial.Config{
			MaxIterations: 5,
		},
	}
	cfg := adversarial.CoordinatorConfig{
		AdvSession: session,
	}
	return adversarial.NewCoordinator(cfg)
}

func TestTripleShotIndicator_MixedPhaseAggregation(t *testing.T) {
	tests := []struct {
		name        string
		phases      []tripleshot.Phase
		expectLabel string
		expectStyle string // "highlight", "warning", "success", "error"
	}{
		{
			name:        "working and evaluating shows active count",
			phases:      []tripleshot.Phase{tripleshot.PhaseWorking, tripleshot.PhaseEvaluating},
			expectLabel: "2 active",
			expectStyle: "highlight",
		},
		{
			name:        "all evaluating shows active count",
			phases:      []tripleshot.Phase{tripleshot.PhaseEvaluating, tripleshot.PhaseEvaluating},
			expectLabel: "2 active",
			expectStyle: "highlight",
		},
		{
			name:        "all complete shows done count",
			phases:      []tripleshot.Phase{tripleshot.PhaseComplete, tripleshot.PhaseComplete, tripleshot.PhaseComplete},
			expectLabel: "3 done",
			expectStyle: "success",
		},
		{
			name:        "all failed shows failed",
			phases:      []tripleshot.Phase{tripleshot.PhaseFailed, tripleshot.PhaseFailed},
			expectLabel: "failed",
			expectStyle: "error",
		},
		{
			name:        "mixed complete and failed shows done count",
			phases:      []tripleshot.Phase{tripleshot.PhaseComplete, tripleshot.PhaseFailed},
			expectLabel: "1 done",
			expectStyle: "success",
		},
		{
			name:        "working takes priority over complete",
			phases:      []tripleshot.Phase{tripleshot.PhaseWorking, tripleshot.PhaseComplete, tripleshot.PhaseComplete},
			expectLabel: "1 active",
			expectStyle: "highlight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runners := make(map[string]tripleshot.Runner)
			for i, phase := range tt.phases {
				groupID := string(rune('a' + i))
				runners[groupID] = createMockTripleShotCoordinatorWithPhase(phase)
			}
			state := &WorkflowStatusState{
				TripleShot: &TripleShotState{
					Runners: runners,
				},
			}

			indicators := state.GetIndicators()
			if len(indicators) != 1 {
				t.Fatalf("Expected 1 indicator, got %d", len(indicators))
			}

			ind := indicators[0]
			if ind.Label != tt.expectLabel {
				t.Errorf("Label = %q, want %q", ind.Label, tt.expectLabel)
			}
		})
	}
}

func TestAdversarialIndicator_MixedPhaseAggregation(t *testing.T) {
	tests := []struct {
		name        string
		phases      []adversarial.Phase
		expectIcon  string
		expectLabel string
	}{
		{
			name:        "implementing takes priority over reviewing",
			phases:      []adversarial.Phase{adversarial.PhaseImplementing, adversarial.PhaseReviewing, adversarial.PhaseReviewing},
			expectIcon:  "🔨",
			expectLabel: "1 impl",
		},
		{
			name:        "reviewing shown when no implementing",
			phases:      []adversarial.Phase{adversarial.PhaseReviewing, adversarial.PhaseReviewing},
			expectIcon:  "🔍",
			expectLabel: "2 review",
		},
		{
			name:        "all approved shows done count",
			phases:      []adversarial.Phase{adversarial.PhaseApproved, adversarial.PhaseApproved, adversarial.PhaseComplete},
			expectIcon:  "✓",
			expectLabel: "3 done",
		},
		{
			name:        "all failed shows failed",
			phases:      []adversarial.Phase{adversarial.PhaseFailed, adversarial.PhaseFailed},
			expectIcon:  "✗",
			expectLabel: "failed",
		},
		{
			name:        "mixed approved and failed shows done count",
			phases:      []adversarial.Phase{adversarial.PhaseApproved, adversarial.PhaseFailed},
			expectIcon:  "✓",
			expectLabel: "1 done",
		},
		{
			name:        "implementing takes priority over approved",
			phases:      []adversarial.Phase{adversarial.PhaseImplementing, adversarial.PhaseApproved, adversarial.PhaseApproved},
			expectIcon:  "🔨",
			expectLabel: "1 impl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coordinators := make(map[string]*adversarial.Coordinator)
			for i, phase := range tt.phases {
				groupID := string(rune('a' + i))
				coordinators[groupID] = createMockAdversarialCoordinatorWithPhase(phase)
			}
			state := &WorkflowStatusState{
				Adversarial: &AdversarialState{
					Coordinators: coordinators,
				},
			}

			indicators := state.GetIndicators()
			if len(indicators) != 1 {
				t.Fatalf("Expected 1 indicator, got %d", len(indicators))
			}

			ind := indicators[0]
			if ind.Icon != tt.expectIcon {
				t.Errorf("Icon = %q, want %q", ind.Icon, tt.expectIcon)
			}
			if ind.Label != tt.expectLabel {
				t.Errorf("Label = %q, want %q", ind.Label, tt.expectLabel)
			}
		})
	}
}

func TestRenderIndicator_WithCount(t *testing.T) {
	tests := []struct {
		name           string
		indicator      WorkflowIndicator
		expectContains []string
	}{
		{
			name: "indicator without count shows no parentheses",
			indicator: WorkflowIndicator{
				Icon:  "🔨",
				Label: "working",
				Count: 0,
			},
			expectContains: []string{"🔨", "working"},
		},
		{
			name: "indicator with count shows formatted count",
			indicator: WorkflowIndicator{
				Icon:  "🔨",
				Label: "2 impl",
				Count: 3,
			},
			expectContains: []string{"🔨", "(3)", "2 impl"},
		},
		{
			name: "indicator with large count",
			indicator: WorkflowIndicator{
				Icon:  "△",
				Label: "5 active",
				Count: 10,
			},
			expectContains: []string{"△", "(10)", "5 active"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := renderIndicator(tt.indicator)

			for _, expected := range tt.expectContains {
				if !strings.Contains(result, expected) {
					t.Errorf("renderIndicator() = %q, want to contain %q", result, expected)
				}
			}

			// Verify count parentheses only appear when Count > 0
			hasParens := strings.Contains(result, "(")
			if tt.indicator.Count > 0 && !hasParens {
				t.Errorf("Expected parentheses for Count=%d, got %q", tt.indicator.Count, result)
			}
			if tt.indicator.Count == 0 && hasParens {
				t.Errorf("Unexpected parentheses for Count=0, got %q", result)
			}
		})
	}
}

func TestRenderWorkflowStatus_AllThreeWorkflows(t *testing.T) {
	// Test that all three workflows together renders correctly with two separators
	state := &WorkflowStatusState{
		UltraPlan:   createMockUltraPlanState(orchestrator.PhaseExecuting, "Test objective"),
		TripleShot:  createMockTripleShotState(1),
		Adversarial: createMockAdversarialState(1),
	}

	result := RenderWorkflowStatus(state)

	// Should contain indicators for all three
	if !strings.Contains(result, styles.IconUltraPlan) {
		t.Error("Expected UltraPlan icon in result")
	}
	if !strings.Contains(result, "△") {
		t.Error("Expected TripleShot icon in result")
	}
	if !strings.Contains(result, "🔨") {
		t.Error("Expected Adversarial icon in result")
	}

	// Should have exactly 2 separators for 3 workflows
	separatorCount := strings.Count(result, "│")
	if separatorCount != 2 {
		t.Errorf("Expected 2 separators for 3 workflows, got %d in %q", separatorCount, result)
	}
}

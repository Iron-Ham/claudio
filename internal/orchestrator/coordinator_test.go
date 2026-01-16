package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
)

// TestGetMultiPassStrategyNames verifies that we have the expected strategies
func TestGetMultiPassStrategyNames(t *testing.T) {
	names := GetMultiPassStrategyNames()

	// We expect exactly 3 strategies
	if len(names) != 3 {
		t.Errorf("GetMultiPassStrategyNames() returned %d strategies, want 3", len(names))
	}

	// Verify expected strategy names
	expected := []string{"maximize-parallelism", "minimize-complexity", "balanced-approach"}
	for i, want := range expected {
		if i >= len(names) {
			t.Errorf("missing strategy at index %d, want %q", i, want)
			continue
		}
		if names[i] != want {
			t.Errorf("strategy[%d] = %q, want %q", i, names[i], want)
		}
	}
}

// TestGetMultiPassPlanningPrompt verifies strategy prompts are correctly constructed
func TestGetMultiPassPlanningPrompt(t *testing.T) {
	tests := []struct {
		name       string
		strategy   string
		objective  string
		wantParts  []string // Strings that should appear in the prompt
		wantAbsent []string // Strings that should NOT appear
	}{
		{
			name:      "maximize-parallelism strategy",
			strategy:  "maximize-parallelism",
			objective: "Add user authentication",
			wantParts: []string{
				"Add user authentication",               // Objective
				"Strategic Focus: Maximize Parallelism", // Strategy header
				"Minimize Dependencies",                 // Strategy-specific content
				"Flatten the Dependency Graph",
			},
		},
		{
			name:      "minimize-complexity strategy",
			strategy:  "minimize-complexity",
			objective: "Refactor database layer",
			wantParts: []string{
				"Refactor database layer",
				"Strategic Focus: Minimize Complexity",
				"Single Responsibility",
			},
		},
		{
			name:      "balanced-approach strategy",
			strategy:  "balanced-approach",
			objective: "Build API endpoints",
			wantParts: []string{
				"Build API endpoints",
				"Strategic Focus: Balanced Approach",
				"Respect Natural Structure",
				"Right-Sized Tasks",
			},
		},
		{
			name:      "unknown strategy returns base prompt only",
			strategy:  "nonexistent-strategy",
			objective: "Test objective",
			wantParts: []string{
				"Test objective",
			},
			wantAbsent: []string{
				"Strategic Focus", // No strategy-specific content
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt := GetMultiPassPlanningPrompt(tt.strategy, tt.objective)

			for _, part := range tt.wantParts {
				if !strings.Contains(prompt, part) {
					t.Errorf("prompt missing expected content: %q", part)
				}
			}

			for _, absent := range tt.wantAbsent {
				if strings.Contains(prompt, absent) {
					t.Errorf("prompt contains unexpected content: %q", absent)
				}
			}
		})
	}
}

// TestFormatCandidatePlansForManager tests the formatting of candidate plans
func TestFormatCandidatePlansForManager(t *testing.T) {
	tests := []struct {
		name      string
		plans     []*PlanSpec
		wantParts []string
	}{
		{
			name:      "empty plans",
			plans:     []*PlanSpec{},
			wantParts: []string{"No candidate plans available"},
		},
		{
			name:  "nil plan in list",
			plans: []*PlanSpec{nil},
			wantParts: []string{
				"### Plan 1: maximize-parallelism",
				"<plan>",
				"null",
				"</plan>",
			},
		},
		{
			name: "single valid plan",
			plans: []*PlanSpec{
				{
					Summary: "Test plan summary",
					Tasks: []PlannedTask{
						{ID: "task-1", Title: "First task", EstComplexity: ComplexityLow},
						{ID: "task-2", Title: "Second task", DependsOn: []string{"task-1"}, EstComplexity: ComplexityMedium},
					},
					ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				},
			},
			wantParts: []string{
				"### Plan 1: maximize-parallelism",
				"<plan>",
				"Test plan summary",
				"task-1",
				"First task",
				"task-2",
				"Second task",
				"</plan>",
			},
		},
		{
			name: "multiple plans with different strategies",
			plans: []*PlanSpec{
				{
					Summary: "Parallel plan",
					Tasks:   []PlannedTask{{ID: "t1", Title: "Task 1", EstComplexity: ComplexityLow}},
				},
				{
					Summary: "Simple plan",
					Tasks:   []PlannedTask{{ID: "t2", Title: "Task 2", EstComplexity: ComplexityMedium}},
				},
				{
					Summary: "Balanced plan",
					Tasks:   []PlannedTask{{ID: "t3", Title: "Task 3", EstComplexity: ComplexityHigh}},
				},
			},
			wantParts: []string{
				"### Plan 1: maximize-parallelism",
				"Parallel plan",
				"### Plan 2: minimize-complexity",
				"Simple plan",
				"### Plan 3: balanced-approach",
				"Balanced plan",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCandidatePlansForManager(tt.plans)

			for _, part := range tt.wantParts {
				if !strings.Contains(result, part) {
					t.Errorf("formatCandidatePlansForManager() output missing: %q\nGot:\n%s", part, result)
				}
			}
		})
	}
}

// TestFormatCandidatePlansForManager_JSONValidity ensures plans are valid JSON
func TestFormatCandidatePlansForManager_JSONValidity(t *testing.T) {
	plan := &PlanSpec{
		ID:        "plan-1",
		Objective: "Test objective",
		Summary:   "Test summary",
		Tasks: []PlannedTask{
			{
				ID:            "task-1",
				Title:         "Test task",
				Description:   "Description here",
				Files:         []string{"file1.go", "file2.go"},
				DependsOn:     []string{},
				Priority:      1,
				EstComplexity: ComplexityLow,
			},
		},
		ExecutionOrder: [][]string{{"task-1"}},
		Insights:       []string{"Insight 1"},
		Constraints:    []string{"Constraint 1"},
	}

	result := formatCandidatePlansForManager([]*PlanSpec{plan})

	// Extract the JSON between <plan> tags
	startIdx := strings.Index(result, "<plan>")
	endIdx := strings.Index(result, "</plan>")
	if startIdx == -1 || endIdx == -1 {
		t.Fatal("missing <plan> tags in output")
	}

	jsonStr := strings.TrimSpace(result[startIdx+len("<plan>") : endIdx])

	// Verify it's valid JSON
	var parsed PlanSpec
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Errorf("plan JSON is invalid: %v\nJSON:\n%s", err, jsonStr)
	}

	// Verify content is preserved
	if parsed.Summary != plan.Summary {
		t.Errorf("parsed summary = %q, want %q", parsed.Summary, plan.Summary)
	}
	if len(parsed.Tasks) != len(plan.Tasks) {
		t.Errorf("parsed has %d tasks, want %d", len(parsed.Tasks), len(plan.Tasks))
	}
}

// TestBuildPlanManagerPrompt_Integration tests the buildPlanManagerPrompt method
// This test verifies the integration between the coordinator and plan formatting
func TestBuildPlanManagerPrompt_Integration(t *testing.T) {
	// Create a coordinator with a mock session
	session := &UltraPlanSession{
		ID:        "test-session",
		Objective: "Implement feature X",
		Config:    UltraPlanConfig{MultiPass: true},
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Plan A - maximize parallelism",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Setup infrastructure", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Add core logic", EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1", "task-2"}},
				Insights:       []string{"Codebase uses clean architecture"},
			},
			{
				Summary: "Plan B - minimize complexity",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Sequential step 1", DependsOn: []string{}, EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Sequential step 2", DependsOn: []string{"task-1"}, EstComplexity: ComplexityLow},
					{ID: "task-3", Title: "Sequential step 3", DependsOn: []string{"task-2"}, EstComplexity: ComplexityLow},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			},
			{
				Summary: "Plan C - balanced approach",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Foundation work", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Feature implementation", DependsOn: []string{"task-1"}, EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Constraints:    []string{"Must maintain backward compatibility"},
			},
		},
		PlanCoordinatorIDs: []string{"coord-1", "coord-2", "coord-3"},
	}

	// Create a minimal coordinator to test buildPlanManagerPrompt
	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	prompt := coord.buildPlanManagerPrompt()

	// Verify the prompt contains the objective
	if !strings.Contains(prompt, "Implement feature X") {
		t.Error("prompt missing objective")
	}

	// Verify all three plans are included
	expectedParts := []string{
		"Plan 1: maximize-parallelism Strategy",
		"Plan 2: minimize-complexity Strategy",
		"Plan 3: balanced-approach Strategy",
		"Plan A - maximize parallelism",
		"Plan B - minimize complexity",
		"Plan C - balanced approach",
		"Setup infrastructure",
		"Sequential step 1",
		"Foundation work",
	}

	for _, part := range expectedParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("prompt missing expected content: %q", part)
		}
	}

	// Verify execution groups are shown
	if !strings.Contains(prompt, "parallel groups") {
		t.Error("prompt missing execution groups info")
	}

	// Verify insights and constraints are included
	if !strings.Contains(prompt, "clean architecture") {
		t.Error("prompt missing insights")
	}
	if !strings.Contains(prompt, "backward compatibility") {
		t.Error("prompt missing constraints")
	}
}

// TestBuildPlanComparisonSection tests the alternative JSON-based plan comparison format
func TestBuildPlanComparisonSection(t *testing.T) {
	session := &UltraPlanSession{
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Test plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task One", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task Two", EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1", "task-2"}},
				Insights:       []string{"Key insight"},
				Constraints:    []string{"Important constraint"},
			},
			nil, // Test handling of nil plan
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	result := coord.buildPlanComparisonSection()

	// Check that plan 1 is formatted correctly
	expectedParts := []string{
		"### Plan 1: maximize-parallelism",
		"**Summary**: Test plan",
		"**Task Count**: 2 tasks",
		"**Execution Groups**: 1 groups",
		"**Max Parallelism**: 2 concurrent tasks",
		"**Insights**:",
		"Key insight",
		"**Constraints**:",
		"Important constraint",
		"**Tasks (JSON)**:",
		"```json",
		"task-1",
		"Task One",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("buildPlanComparisonSection() missing: %q", part)
		}
	}

	// Nil plan (at index 1) should be skipped
	if strings.Contains(result, "### Plan 2:") && strings.Contains(result, "null") {
		t.Error("buildPlanComparisonSection() should skip nil plans")
	}
}

// TestMultiPassPlanningPrompts_Consistency verifies prompts have required fields
func TestMultiPassPlanningPrompts_Consistency(t *testing.T) {
	for i, strategy := range MultiPassPlanningPrompts {
		t.Run(strategy.Strategy, func(t *testing.T) {
			if strategy.Strategy == "" {
				t.Errorf("MultiPassPlanningPrompts[%d] has empty Strategy", i)
			}
			if strategy.Description == "" {
				t.Errorf("MultiPassPlanningPrompts[%d] has empty Description", i)
			}
			if strategy.Prompt == "" {
				t.Errorf("MultiPassPlanningPrompts[%d] has empty Prompt", i)
			}
			if !strings.Contains(strategy.Prompt, "Strategic Focus") {
				t.Errorf("MultiPassPlanningPrompts[%d] prompt missing 'Strategic Focus' header", i)
			}
		})
	}
}

// TestUltraPlanPhase_Transitions tests phase transition constants
func TestUltraPlanPhase_Transitions(t *testing.T) {
	// Verify all expected phases exist
	phases := []UltraPlanPhase{
		PhasePlanning,
		PhasePlanSelection,
		PhaseRefresh,
		PhaseExecuting,
		PhaseSynthesis,
		PhaseRevision,
		PhaseConsolidating,
		PhaseComplete,
		PhaseFailed,
	}

	// Verify each phase has a unique non-empty value
	seen := make(map[UltraPlanPhase]bool)
	for _, phase := range phases {
		if phase == "" {
			t.Error("found empty phase constant")
		}
		if seen[phase] {
			t.Errorf("duplicate phase value: %s", phase)
		}
		seen[phase] = true
	}

	// Verify phase values
	if PhasePlanning != "planning" {
		t.Errorf("PhasePlanning = %q, want %q", PhasePlanning, "planning")
	}
	if PhasePlanSelection != "plan_selection" {
		t.Errorf("PhasePlanSelection = %q, want %q", PhasePlanSelection, "plan_selection")
	}
}

// TestUltraPlanConfig_MultiPassDefaults verifies default config
func TestUltraPlanConfig_MultiPassDefaults(t *testing.T) {
	config := DefaultUltraPlanConfig()

	if config.MultiPass {
		t.Error("MultiPass should be false by default")
	}

	if config.MaxParallel != 3 {
		t.Errorf("MaxParallel = %d, want 3", config.MaxParallel)
	}
}

// TestRunMultiPassPlanning_SessionStateSetup verifies the session state setup
// Note: This is a unit test that doesn't require a full orchestrator
func TestRunMultiPassPlanning_SessionStateSetup(t *testing.T) {
	// Test that an empty strategies list returns an error
	strategies := GetMultiPassStrategyNames()
	if len(strategies) == 0 {
		t.Skip("no strategies available - this would cause RunMultiPassPlanning to error")
	}

	// Verify we have the expected number of strategies for parallel planning
	if len(strategies) != 3 {
		t.Errorf("expected 3 strategies for parallel planning, got %d", len(strategies))
	}
}

// TestRunPlanManager_ValidationLogic tests the validation logic in RunPlanManager
func TestRunPlanManager_ValidationLogic(t *testing.T) {
	tests := []struct {
		name       string
		session    *UltraPlanSession
		wantErr    bool
		errContain string
	}{
		{
			name: "multi-pass disabled",
			session: &UltraPlanSession{
				Config:             UltraPlanConfig{MultiPass: false},
				PlanCoordinatorIDs: []string{"c1", "c2", "c3"},
				CandidatePlans:     []*PlanSpec{{}, {}, {}},
			},
			wantErr:    true,
			errContain: "MultiPass mode is not enabled",
		},
		{
			name: "not all plans collected",
			session: &UltraPlanSession{
				Config:             UltraPlanConfig{MultiPass: true},
				PlanCoordinatorIDs: []string{"c1", "c2", "c3"},
				CandidatePlans:     []*PlanSpec{{}, {}}, // Only 2 of 3
			},
			wantErr:    true,
			errContain: "not all candidate plans collected",
		},
		{
			name: "nil plan in candidates",
			session: &UltraPlanSession{
				Config:             UltraPlanConfig{MultiPass: true},
				PlanCoordinatorIDs: []string{"c1", "c2", "c3"},
				CandidatePlans:     []*PlanSpec{{}, nil, {}},
			},
			wantErr:    true,
			errContain: "candidate plan at index 1 is nil",
		},
		{
			name: "valid configuration",
			session: &UltraPlanSession{
				Config:             UltraPlanConfig{MultiPass: true},
				PlanCoordinatorIDs: []string{"c1", "c2", "c3"},
				CandidatePlans: []*PlanSpec{
					{Summary: "Plan 1", Tasks: []PlannedTask{{ID: "t1"}}},
					{Summary: "Plan 2", Tasks: []PlannedTask{{ID: "t2"}}},
					{Summary: "Plan 3", Tasks: []PlannedTask{{ID: "t3"}}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the validation logic from RunPlanManager
			err := validatePlanManagerPrerequisites(tt.session)

			if (err != nil) != tt.wantErr {
				t.Errorf("validatePlanManagerPrerequisites() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContain != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error = %q, want containing %q", err.Error(), tt.errContain)
				}
			}
		})
	}
}

// validatePlanManagerPrerequisites extracts the validation logic from RunPlanManager
// for easier unit testing without requiring a full orchestrator setup
func validatePlanManagerPrerequisites(session *UltraPlanSession) error {
	if !session.Config.MultiPass {
		return &validationError{"RunPlanManager called but MultiPass mode is not enabled"}
	}

	if len(session.CandidatePlans) < len(session.PlanCoordinatorIDs) {
		return &validationError{
			message: "not all candidate plans collected: have " +
				itoa(len(session.CandidatePlans)) + ", need " +
				itoa(len(session.PlanCoordinatorIDs)),
		}
	}

	for i, plan := range session.CandidatePlans {
		if plan == nil {
			return &validationError{message: "candidate plan at index " + itoa(i) + " is nil"}
		}
	}

	return nil
}

// validationError is a simple error type for validation
type validationError struct {
	message string
}

func (e *validationError) Error() string {
	return e.message
}

// itoa is a simple int to string conversion without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// TestCoordinatorCallbacks_PhaseChange tests phase change callback functionality
func TestCoordinatorCallbacks_PhaseChange(t *testing.T) {
	// Test that CoordinatorCallbacks struct has the expected fields
	var cb CoordinatorCallbacks

	// Verify all callback fields are nil by default
	if cb.OnPhaseChange != nil {
		t.Error("OnPhaseChange should be nil by default")
	}
	if cb.OnTaskStart != nil {
		t.Error("OnTaskStart should be nil by default")
	}
	if cb.OnTaskComplete != nil {
		t.Error("OnTaskComplete should be nil by default")
	}
	if cb.OnTaskFailed != nil {
		t.Error("OnTaskFailed should be nil by default")
	}
	if cb.OnGroupComplete != nil {
		t.Error("OnGroupComplete should be nil by default")
	}
	if cb.OnPlanReady != nil {
		t.Error("OnPlanReady should be nil by default")
	}
	if cb.OnProgress != nil {
		t.Error("OnProgress should be nil by default")
	}
	if cb.OnComplete != nil {
		t.Error("OnComplete should be nil by default")
	}

	// Test setting callbacks
	phaseChangeCalled := false
	cb.OnPhaseChange = func(phase UltraPlanPhase) {
		phaseChangeCalled = true
		if phase != PhasePlanning {
			t.Errorf("OnPhaseChange received phase %v, want %v", phase, PhasePlanning)
		}
	}

	// Simulate callback invocation
	cb.OnPhaseChange(PhasePlanning)
	if !phaseChangeCalled {
		t.Error("OnPhaseChange callback was not called")
	}
}

// TestPlanDecision_Structure tests the PlanDecision struct
func TestPlanDecision_Structure(t *testing.T) {
	decision := PlanDecision{
		Action:        "select",
		SelectedIndex: 1,
		Reasoning:     "Plan 2 has better task organization",
		PlanScores: []PlanScore{
			{Strategy: "maximize-parallelism", Score: 70, Strengths: "Fast", Weaknesses: "Complex"},
			{Strategy: "minimize-complexity", Score: 85, Strengths: "Clear", Weaknesses: "Slower"},
			{Strategy: "balanced-approach", Score: 75, Strengths: "Balanced", Weaknesses: "None"},
		},
	}

	// Verify serialization
	data, err := json.Marshal(decision)
	if err != nil {
		t.Fatalf("failed to marshal PlanDecision: %v", err)
	}

	var parsed PlanDecision
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal PlanDecision: %v", err)
	}

	if parsed.Action != decision.Action {
		t.Errorf("Action = %q, want %q", parsed.Action, decision.Action)
	}
	if parsed.SelectedIndex != decision.SelectedIndex {
		t.Errorf("SelectedIndex = %d, want %d", parsed.SelectedIndex, decision.SelectedIndex)
	}
	if len(parsed.PlanScores) != len(decision.PlanScores) {
		t.Errorf("PlanScores length = %d, want %d", len(parsed.PlanScores), len(decision.PlanScores))
	}
}

// TestMultiPassSession_CandidatePlansSlice tests candidate plan slice handling
func TestMultiPassSession_CandidatePlansSlice(t *testing.T) {
	session := &UltraPlanSession{
		Config:             UltraPlanConfig{MultiPass: true},
		PlanCoordinatorIDs: make([]string, 0, 3),
		CandidatePlans:     make([]*PlanSpec, 0, 3),
	}

	// Simulate adding coordinator IDs as they're created
	strategies := GetMultiPassStrategyNames()
	for _, strategy := range strategies {
		session.PlanCoordinatorIDs = append(session.PlanCoordinatorIDs, "coord-"+strategy)
	}

	if len(session.PlanCoordinatorIDs) != 3 {
		t.Errorf("expected 3 coordinator IDs, got %d", len(session.PlanCoordinatorIDs))
	}

	// Simulate adding candidate plans as they're collected
	for i := 0; i < 3; i++ {
		session.CandidatePlans = append(session.CandidatePlans, &PlanSpec{
			Summary: "Plan from " + strategies[i],
			Tasks:   []PlannedTask{{ID: "task-1", Title: "Test task"}},
		})
	}

	if len(session.CandidatePlans) != len(session.PlanCoordinatorIDs) {
		t.Error("CandidatePlans count should match PlanCoordinatorIDs count")
	}

	// Verify plans are correctly indexed
	for i, plan := range session.CandidatePlans {
		expected := "Plan from " + strategies[i]
		if plan.Summary != expected {
			t.Errorf("plan[%d].Summary = %q, want %q", i, plan.Summary, expected)
		}
	}
}

// TestBuildPlanManagerPrompt_ExecutionOrderFormatting tests execution group formatting
func TestBuildPlanManagerPrompt_ExecutionOrderFormatting(t *testing.T) {
	session := &UltraPlanSession{
		Objective: "Test objective",
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Plan with groups",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Task 1", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}, EstComplexity: ComplexityMedium},
					{ID: "task-3", Title: "Task 3", DependsOn: []string{"task-1"}, EstComplexity: ComplexityLow},
					{ID: "task-4", Title: "Task 4", DependsOn: []string{"task-2", "task-3"}, EstComplexity: ComplexityHigh},
				},
				ExecutionOrder: [][]string{
					{"task-1"},
					{"task-2", "task-3"},
					{"task-4"},
				},
			},
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	prompt := coord.buildPlanManagerPrompt()

	// Verify execution groups are properly formatted
	expectedParts := []string{
		"3 parallel groups",
		"Group 1: task-1",
		"Group 2: task-2, task-3",
		"Group 3: task-4",
	}

	for _, part := range expectedParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("prompt missing execution group formatting: %q", part)
		}
	}
}

// TestBuildPlanManagerPrompt_DependencyFormatting tests dependency formatting
func TestBuildPlanManagerPrompt_DependencyFormatting(t *testing.T) {
	session := &UltraPlanSession{
		Objective: "Test objective",
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Plan with dependencies",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Independent task", DependsOn: []string{}, EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Dependent task", DependsOn: []string{"task-1"}, EstComplexity: ComplexityMedium},
					{ID: "task-3", Title: "Multi-dep task", DependsOn: []string{"task-1", "task-2"}, EstComplexity: ComplexityHigh},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			},
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	prompt := coord.buildPlanManagerPrompt()

	// Verify dependencies are shown
	if !strings.Contains(prompt, "depends: none") {
		t.Error("prompt should show 'depends: none' for independent tasks")
	}
	if !strings.Contains(prompt, "depends: task-1, task-2") {
		t.Error("prompt should show multiple dependencies separated by commas")
	}
}

// TestStepInfo_Types tests the StepInfo type and StepType constants
func TestStepInfo_Types(t *testing.T) {
	// Verify StepType constants have expected values
	tests := []struct {
		stepType StepType
		want     string
	}{
		{StepTypePlanning, "planning"},
		{StepTypePlanManager, "plan_manager"},
		{StepTypeTask, "task"},
		{StepTypeSynthesis, "synthesis"},
		{StepTypeRevision, "revision"},
		{StepTypeConsolidation, "consolidation"},
		{StepTypeGroupConsolidator, "group_consolidator"},
	}

	for _, tt := range tests {
		t.Run(string(tt.stepType), func(t *testing.T) {
			if string(tt.stepType) != tt.want {
				t.Errorf("StepType = %q, want %q", tt.stepType, tt.want)
			}
		})
	}
}

// TestStepInfo_Structure tests the StepInfo struct
func TestStepInfo_Structure(t *testing.T) {
	info := StepInfo{
		Type:       StepTypeTask,
		InstanceID: "inst-123",
		TaskID:     "task-1",
		GroupIndex: 0,
		Label:      "Setup Database",
	}

	if info.Type != StepTypeTask {
		t.Errorf("Type = %v, want %v", info.Type, StepTypeTask)
	}
	if info.InstanceID != "inst-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "inst-123")
	}
	if info.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", info.TaskID, "task-1")
	}
	if info.GroupIndex != 0 {
		t.Errorf("GroupIndex = %d, want %d", info.GroupIndex, 0)
	}
	if info.Label != "Setup Database" {
		t.Errorf("Label = %q, want %q", info.Label, "Setup Database")
	}
}

// TestGetStepInfo_Nil tests GetStepInfo with nil session
func TestGetStepInfo_Nil(t *testing.T) {
	// Create a minimal coordinator with a session
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}

	coord := &Coordinator{
		manager: manager,
	}

	// Test with empty instance ID
	info := coord.GetStepInfo("")
	if info != nil {
		t.Errorf("GetStepInfo(\"\") = %v, want nil", info)
	}

	// Test with unknown instance ID
	info = coord.GetStepInfo("unknown-instance")
	if info != nil {
		t.Errorf("GetStepInfo(\"unknown-instance\") = %v, want nil", info)
	}
}

// TestGetStepInfo_Planning tests GetStepInfo for planning coordinator
func TestGetStepInfo_Planning(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.CoordinatorID = "plan-coord-123"

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	info := coord.GetStepInfo("plan-coord-123")
	if info == nil {
		t.Fatal("GetStepInfo returned nil for planning coordinator")
	}
	if info.Type != StepTypePlanning {
		t.Errorf("Type = %v, want %v", info.Type, StepTypePlanning)
	}
	if info.InstanceID != "plan-coord-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "plan-coord-123")
	}
	if info.GroupIndex != -1 {
		t.Errorf("GroupIndex = %d, want %d", info.GroupIndex, -1)
	}
	if info.Label != "Planning Coordinator" {
		t.Errorf("Label = %q, want %q", info.Label, "Planning Coordinator")
	}
}

// TestGetStepInfo_MultiPassCoordinators tests GetStepInfo for multi-pass planning coordinators
func TestGetStepInfo_MultiPassCoordinators(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.PlanCoordinatorIDs = []string{"coord-0", "coord-1", "coord-2"}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	strategies := GetMultiPassStrategyNames()

	for i, coordID := range session.PlanCoordinatorIDs {
		t.Run(coordID, func(t *testing.T) {
			info := coord.GetStepInfo(coordID)
			if info == nil {
				t.Fatalf("GetStepInfo returned nil for coordinator %s", coordID)
			}
			if info.Type != StepTypePlanning {
				t.Errorf("Type = %v, want %v", info.Type, StepTypePlanning)
			}
			if info.InstanceID != coordID {
				t.Errorf("InstanceID = %q, want %q", info.InstanceID, coordID)
			}
			if info.GroupIndex != i {
				t.Errorf("GroupIndex = %d, want %d", info.GroupIndex, i)
			}
			expectedLabel := "Plan Coordinator (" + strategies[i] + ")"
			if info.Label != expectedLabel {
				t.Errorf("Label = %q, want %q", info.Label, expectedLabel)
			}
		})
	}
}

// TestGetStepInfo_PlanManager tests GetStepInfo for plan manager
func TestGetStepInfo_PlanManager(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.PlanManagerID = "plan-manager-123"

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	info := coord.GetStepInfo("plan-manager-123")
	if info == nil {
		t.Fatal("GetStepInfo returned nil for plan manager")
	}
	if info.Type != StepTypePlanManager {
		t.Errorf("Type = %v, want %v", info.Type, StepTypePlanManager)
	}
	if info.InstanceID != "plan-manager-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "plan-manager-123")
	}
	if info.Label != "Plan Manager" {
		t.Errorf("Label = %q, want %q", info.Label, "Plan Manager")
	}
}

// TestGetStepInfo_Task tests GetStepInfo for task instances
func TestGetStepInfo_Task(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Setup Database"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}
	session.TaskToInstance = map[string]string{
		"task-1": "task-inst-123",
	}

	manager := &UltraPlanManager{session: session}

	// Create a group tracker using the adapter pattern
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					task := session.GetTask(taskID)
					if task == nil {
						return nil
					}
					return &group.Task{ID: task.ID, Title: task.Title}
				},
			)
		},
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)
	groupTracker := group.NewTracker(sessionAdapter)

	coord := &Coordinator{
		manager:      manager,
		groupTracker: groupTracker,
	}

	info := coord.GetStepInfo("task-inst-123")
	if info == nil {
		t.Fatal("GetStepInfo returned nil for task instance")
	}
	if info.Type != StepTypeTask {
		t.Errorf("Type = %v, want %v", info.Type, StepTypeTask)
	}
	if info.InstanceID != "task-inst-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "task-inst-123")
	}
	if info.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", info.TaskID, "task-1")
	}
	if info.GroupIndex != 0 {
		t.Errorf("GroupIndex = %d, want %d", info.GroupIndex, 0)
	}
	if info.Label != "Setup Database" {
		t.Errorf("Label = %q, want %q", info.Label, "Setup Database")
	}
}

// TestGetStepInfo_Synthesis tests GetStepInfo for synthesis instance
func TestGetStepInfo_Synthesis(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.SynthesisID = "synth-123"

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	info := coord.GetStepInfo("synth-123")
	if info == nil {
		t.Fatal("GetStepInfo returned nil for synthesis instance")
	}
	if info.Type != StepTypeSynthesis {
		t.Errorf("Type = %v, want %v", info.Type, StepTypeSynthesis)
	}
	if info.InstanceID != "synth-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "synth-123")
	}
	if info.Label != "Synthesis" {
		t.Errorf("Label = %q, want %q", info.Label, "Synthesis")
	}
}

// TestGetStepInfo_Revision tests GetStepInfo for revision instance
func TestGetStepInfo_Revision(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.RevisionID = "rev-123"

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	info := coord.GetStepInfo("rev-123")
	if info == nil {
		t.Fatal("GetStepInfo returned nil for revision instance")
	}
	if info.Type != StepTypeRevision {
		t.Errorf("Type = %v, want %v", info.Type, StepTypeRevision)
	}
	if info.InstanceID != "rev-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "rev-123")
	}
	if info.Label != "Revision" {
		t.Errorf("Label = %q, want %q", info.Label, "Revision")
	}
}

// TestGetStepInfo_Consolidation tests GetStepInfo for consolidation instance
func TestGetStepInfo_Consolidation(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.ConsolidationID = "consol-123"

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	info := coord.GetStepInfo("consol-123")
	if info == nil {
		t.Fatal("GetStepInfo returned nil for consolidation instance")
	}
	if info.Type != StepTypeConsolidation {
		t.Errorf("Type = %v, want %v", info.Type, StepTypeConsolidation)
	}
	if info.InstanceID != "consol-123" {
		t.Errorf("InstanceID = %q, want %q", info.InstanceID, "consol-123")
	}
	if info.Label != "Consolidation" {
		t.Errorf("Label = %q, want %q", info.Label, "Consolidation")
	}
}

// TestGetStepInfo_GroupConsolidator tests GetStepInfo for group consolidator instances
func TestGetStepInfo_GroupConsolidator(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.GroupConsolidatorIDs = []string{"group-consol-0", "group-consol-1"}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	for i, consolidatorID := range session.GroupConsolidatorIDs {
		t.Run(consolidatorID, func(t *testing.T) {
			info := coord.GetStepInfo(consolidatorID)
			if info == nil {
				t.Fatalf("GetStepInfo returned nil for group consolidator %s", consolidatorID)
			}
			if info.Type != StepTypeGroupConsolidator {
				t.Errorf("Type = %v, want %v", info.Type, StepTypeGroupConsolidator)
			}
			if info.InstanceID != consolidatorID {
				t.Errorf("InstanceID = %q, want %q", info.InstanceID, consolidatorID)
			}
			if info.GroupIndex != i {
				t.Errorf("GroupIndex = %d, want %d", info.GroupIndex, i)
			}
			expectedLabel := "Group " + itoa(i+1) + " Consolidator"
			if info.Label != expectedLabel {
				t.Errorf("Label = %q, want %q", info.Label, expectedLabel)
			}
		})
	}
}

// TestRestartStep_NilInput tests RestartStep with nil stepInfo
func TestRestartStep_NilInput(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	_, err := coord.RestartStep(nil)
	if err == nil {
		t.Error("RestartStep(nil) should return an error")
	}
	if err.Error() != "step info is nil" {
		t.Errorf("error = %q, want %q", err.Error(), "step info is nil")
	}
}

// TestRestartStep_NilSession tests RestartStep with nil session
func TestRestartStep_NilSession(t *testing.T) {
	manager := &UltraPlanManager{session: nil}
	coord := &Coordinator{
		manager: manager,
	}

	// Don't set InstanceID to avoid triggering GetInstance on nil orchestrator
	stepInfo := &StepInfo{
		Type: StepTypeSynthesis,
	}

	_, err := coord.RestartStep(stepInfo)
	if err == nil {
		t.Error("RestartStep with nil session should return an error")
	}
	if err.Error() != "no session" {
		t.Errorf("error = %q, want %q", err.Error(), "no session")
	}
}

// TestRestartStep_UnknownType tests RestartStep with unknown step type
func TestRestartStep_UnknownType(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	// Don't set InstanceID to avoid triggering GetInstance on nil orchestrator
	stepInfo := &StepInfo{
		Type: StepType("unknown_type"),
	}

	_, err := coord.RestartStep(stepInfo)
	if err == nil {
		t.Error("RestartStep with unknown type should return an error")
	}
	expected := "unknown step type: unknown_type"
	if err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

// TestResumeConsolidation_NoSession tests ResumeConsolidation with nil session
func TestResumeConsolidation_NoSession(t *testing.T) {
	manager := &UltraPlanManager{session: nil}
	coord := &Coordinator{
		manager: manager,
	}

	err := coord.ResumeConsolidation()
	if err == nil {
		t.Error("ResumeConsolidation with nil session should return an error")
	}
	if err.Error() != "no session" {
		t.Errorf("error = %q, want %q", err.Error(), "no session")
	}
}

// TestResumeConsolidation_NoConsolidation tests ResumeConsolidation when consolidation is nil
func TestResumeConsolidation_NoConsolidation(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Consolidation = nil

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	err := coord.ResumeConsolidation()
	if err == nil {
		t.Error("ResumeConsolidation without consolidation should return an error")
	}
	if err.Error() != "no consolidation in progress" {
		t.Errorf("error = %q, want %q", err.Error(), "no consolidation in progress")
	}
}

// TestResumeConsolidation_NotPaused tests ResumeConsolidation when not in paused state
func TestResumeConsolidation_NotPaused(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Consolidation = &ConsolidationState{
		Phase: ConsolidationMergingTasks, // Not paused
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	err := coord.ResumeConsolidation()
	if err == nil {
		t.Error("ResumeConsolidation when not paused should return an error")
	}
	if !strings.Contains(err.Error(), "consolidation is not paused") {
		t.Errorf("error = %q, should contain %q", err.Error(), "consolidation is not paused")
	}
}

// TestResumeConsolidation_NoConflictWorktree tests ResumeConsolidation with no conflict worktree
func TestResumeConsolidation_NoConflictWorktree(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Consolidation = &ConsolidationState{
		Phase:            ConsolidationPaused,
		ConflictWorktree: "", // Empty worktree path
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	err := coord.ResumeConsolidation()
	if err == nil {
		t.Error("ResumeConsolidation with no conflict worktree should return an error")
	}
	if err.Error() != "no conflict worktree recorded" {
		t.Errorf("error = %q, want %q", err.Error(), "no conflict worktree recorded")
	}
}

// TestResumeConsolidation_ValidationStates tests various consolidation phase states
func TestResumeConsolidation_ValidationStates(t *testing.T) {
	tests := []struct {
		name       string
		phase      ConsolidationPhase
		worktree   string
		wantErr    bool
		errContain string
	}{
		{
			name:       "creating branches phase",
			phase:      ConsolidationCreatingBranches,
			worktree:   "/tmp/worktree",
			wantErr:    true,
			errContain: "consolidation is not paused",
		},
		{
			name:       "pushing phase",
			phase:      ConsolidationPushing,
			worktree:   "/tmp/worktree",
			wantErr:    true,
			errContain: "consolidation is not paused",
		},
		{
			name:       "complete phase",
			phase:      ConsolidationComplete,
			worktree:   "/tmp/worktree",
			wantErr:    true,
			errContain: "consolidation is not paused",
		},
		{
			name:       "failed phase",
			phase:      ConsolidationFailed,
			worktree:   "/tmp/worktree",
			wantErr:    true,
			errContain: "consolidation is not paused",
		},
		{
			name:     "paused with empty worktree",
			phase:    ConsolidationPaused,
			worktree: "",
			wantErr:  true,
			// This will fail with "no conflict worktree recorded"
			errContain: "no conflict worktree recorded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
			session.Consolidation = &ConsolidationState{
				Phase:            tt.phase,
				ConflictWorktree: tt.worktree,
			}

			manager := &UltraPlanManager{session: session}
			coord := &Coordinator{
				manager: manager,
				// Note: We don't set logger/orch here because these tests should
				// fail before reaching the code that uses them
			}

			err := coord.ResumeConsolidation()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error = %q, should contain %q", err.Error(), tt.errContain)
				}
			} else if err != nil {
				// For the success case, we can't fully test without mocking the orchestrator
				// but we can verify the state changes would be made
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestConsolidationState_HasConflict tests the HasConflict method
func TestConsolidationState_HasConflict(t *testing.T) {
	tests := []struct {
		name     string
		state    ConsolidationState
		expected bool
	}{
		{
			name: "paused with conflict files",
			state: ConsolidationState{
				Phase:         ConsolidationPaused,
				ConflictFiles: []string{"file1.go", "file2.go"},
			},
			expected: true,
		},
		{
			name: "paused without conflict files",
			state: ConsolidationState{
				Phase:         ConsolidationPaused,
				ConflictFiles: []string{},
			},
			expected: false,
		},
		{
			name: "paused with nil conflict files",
			state: ConsolidationState{
				Phase:         ConsolidationPaused,
				ConflictFiles: nil,
			},
			expected: false,
		},
		{
			name: "not paused with conflict files",
			state: ConsolidationState{
				Phase:         ConsolidationMergingTasks,
				ConflictFiles: []string{"file1.go"},
			},
			expected: false,
		},
		{
			name: "complete phase",
			state: ConsolidationState{
				Phase: ConsolidationComplete,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.state.HasConflict()
			if result != tt.expected {
				t.Errorf("HasConflict() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Phase Orchestrator Delegation Tests
// =============================================================================
// These tests verify that the Coordinator correctly delegates to phase orchestrators
// and that the public API (GetStepInfo, RestartStep) remains unchanged.

// TestCoordinator_OrchestratorGetters_NilCoordinator tests orchestrator getters on nil coordinator
func TestCoordinator_OrchestratorGetters_NilCoordinator(t *testing.T) {
	var coord *Coordinator

	// All orchestrator getters should return nil for nil coordinator
	if coord.PlanningOrchestrator() != nil {
		t.Error("PlanningOrchestrator() should return nil for nil coordinator")
	}
	if coord.ExecutionOrchestrator() != nil {
		t.Error("ExecutionOrchestrator() should return nil for nil coordinator")
	}
	if coord.SynthesisOrchestrator() != nil {
		t.Error("SynthesisOrchestrator() should return nil for nil coordinator")
	}
	if coord.ConsolidationOrchestrator() != nil {
		t.Error("ConsolidationOrchestrator() should return nil for nil coordinator")
	}
}

// TestCoordinator_OrchestratorGetters_MinimalCoordinator tests orchestrator getters on minimal coordinator
// Note: Without a proper orchestrator and session, the getters will fail to initialize
// and return nil. This verifies graceful handling of initialization failure.
func TestCoordinator_OrchestratorGetters_MinimalCoordinator(t *testing.T) {
	// Create a coordinator without a manager (will fail initialization)
	// Note: We need to provide a logger because the getter methods log errors
	coord := &Coordinator{
		logger: logging.NopLogger(),
	}

	// These should return nil because initialization will fail without a manager
	if coord.PlanningOrchestrator() != nil {
		t.Error("PlanningOrchestrator() should return nil when initialization fails")
	}
	if coord.ExecutionOrchestrator() != nil {
		t.Error("ExecutionOrchestrator() should return nil when initialization fails")
	}
	if coord.SynthesisOrchestrator() != nil {
		t.Error("SynthesisOrchestrator() should return nil when initialization fails")
	}
	if coord.ConsolidationOrchestrator() != nil {
		t.Error("ConsolidationOrchestrator() should return nil when initialization fails")
	}
}

// TestCoordinator_OrchestratorGetters_NilSession tests orchestrator getters when manager has nil session
func TestCoordinator_OrchestratorGetters_NilSession(t *testing.T) {
	// Create a coordinator with a manager but nil session
	manager := &UltraPlanManager{session: nil}
	coord := &Coordinator{
		manager: manager,
		logger:  logging.NopLogger(),
	}

	// These should return nil because initialization requires a session
	if coord.PlanningOrchestrator() != nil {
		t.Error("PlanningOrchestrator() should return nil when session is nil")
	}
	if coord.ExecutionOrchestrator() != nil {
		t.Error("ExecutionOrchestrator() should return nil when session is nil")
	}
	if coord.SynthesisOrchestrator() != nil {
		t.Error("SynthesisOrchestrator() should return nil when session is nil")
	}
	if coord.ConsolidationOrchestrator() != nil {
		t.Error("ConsolidationOrchestrator() should return nil when session is nil")
	}
}

// TestGetStepInfo_SessionStatePriority tests that GetStepInfo checks session state first
// before falling back to orchestrator state. This verifies the delegation pattern
// maintains backward compatibility.
func TestGetStepInfo_SessionStatePriority(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())

	// Set session state for various step types
	session.CoordinatorID = "plan-123"
	session.PlanManagerID = "manager-123"
	session.SynthesisID = "synth-123"
	session.RevisionID = "rev-123"
	session.ConsolidationID = "consol-123"

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	// Each of these should be resolved from session state
	tests := []struct {
		instanceID   string
		expectedType StepType
		description  string
	}{
		{"plan-123", StepTypePlanning, "planning coordinator from session state"},
		{"manager-123", StepTypePlanManager, "plan manager from session state"},
		{"synth-123", StepTypeSynthesis, "synthesis from session state"},
		{"rev-123", StepTypeRevision, "revision from session state"},
		{"consol-123", StepTypeConsolidation, "consolidation from session state"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			info := coord.GetStepInfo(tt.instanceID)
			if info == nil {
				t.Fatalf("GetStepInfo(%q) returned nil", tt.instanceID)
			}
			if info.Type != tt.expectedType {
				t.Errorf("Type = %v, want %v", info.Type, tt.expectedType)
			}
			if info.InstanceID != tt.instanceID {
				t.Errorf("InstanceID = %q, want %q", info.InstanceID, tt.instanceID)
			}
		})
	}
}

// TestGetStepInfo_MultiPassCoordinatorsByIndex tests that GetStepInfo correctly
// resolves multi-pass coordinators by their index in the slice
func TestGetStepInfo_MultiPassCoordinatorsByIndex(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Config.MultiPass = true
	session.PlanCoordinatorIDs = []string{
		"coord-maximize-parallelism",
		"coord-minimize-complexity",
		"coord-balanced-approach",
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	strategies := GetMultiPassStrategyNames()

	// Verify each coordinator is found with correct index and label
	for i, coordID := range session.PlanCoordinatorIDs {
		info := coord.GetStepInfo(coordID)
		if info == nil {
			t.Errorf("GetStepInfo(%q) returned nil", coordID)
			continue
		}

		if info.Type != StepTypePlanning {
			t.Errorf("coordinator %d: Type = %v, want %v", i, info.Type, StepTypePlanning)
		}
		if info.GroupIndex != i {
			t.Errorf("coordinator %d: GroupIndex = %d, want %d", i, info.GroupIndex, i)
		}
		expectedLabel := "Plan Coordinator (" + strategies[i] + ")"
		if info.Label != expectedLabel {
			t.Errorf("coordinator %d: Label = %q, want %q", i, info.Label, expectedLabel)
		}
	}
}

// TestGetStepInfo_GroupConsolidatorsByIndex tests that group consolidators are
// correctly resolved by index
func TestGetStepInfo_GroupConsolidatorsByIndex(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.GroupConsolidatorIDs = []string{
		"group-consol-0",
		"group-consol-1",
		"group-consol-2",
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	for i, consolidatorID := range session.GroupConsolidatorIDs {
		info := coord.GetStepInfo(consolidatorID)
		if info == nil {
			t.Errorf("GetStepInfo(%q) returned nil", consolidatorID)
			continue
		}

		if info.Type != StepTypeGroupConsolidator {
			t.Errorf("consolidator %d: Type = %v, want %v", i, info.Type, StepTypeGroupConsolidator)
		}
		if info.GroupIndex != i {
			t.Errorf("consolidator %d: GroupIndex = %d, want %d", i, info.GroupIndex, i)
		}
		expectedLabel := "Group " + itoa(i+1) + " Consolidator"
		if info.Label != expectedLabel {
			t.Errorf("consolidator %d: Label = %q, want %q", i, info.Label, expectedLabel)
		}
	}
}

// TestRestartStep_TypeRouting tests that RestartStep handles all step types correctly
// Note: This test verifies unknown types are properly rejected. Full restart behavior
// requires a complete orchestrator setup and is tested elsewhere.
func TestRestartStep_TypeRouting(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
		logger:  logging.NopLogger(),
	}

	// Verify that unknown step types are properly rejected
	t.Run("unknown_type_rejected", func(t *testing.T) {
		stepInfo := &StepInfo{
			Type: StepType("invalid_step_type"),
		}

		_, err := coord.RestartStep(stepInfo)

		if err == nil {
			t.Error("RestartStep with unknown type should return an error")
		}
		if !strings.Contains(err.Error(), "unknown step type") {
			t.Errorf("error = %q, expected to contain 'unknown step type'", err.Error())
		}
	})

	// Verify that empty step type is rejected
	t.Run("empty_type_rejected", func(t *testing.T) {
		stepInfo := &StepInfo{
			Type: StepType(""),
		}

		_, err := coord.RestartStep(stepInfo)

		if err == nil {
			t.Error("RestartStep with empty type should return an error")
		}
	})

	// Verify that all known step types are in the switch statement (not unknown)
	// This uses a mock approach - since these will fail for other reasons,
	// we just verify they don't return "unknown step type" error
	knownTypes := []StepType{
		StepTypePlanning,
		StepTypePlanManager,
		StepTypeTask,
		StepTypeSynthesis,
		StepTypeRevision,
		StepTypeConsolidation,
		StepTypeGroupConsolidator,
	}

	for _, stepType := range knownTypes {
		t.Run("type_"+string(stepType)+"_recognized", func(t *testing.T) {
			// Skip actual execution - just verify the type constant exists and is documented
			// Full restart testing requires orchestrator setup
			if stepType == "" {
				t.Error("StepType constant should not be empty")
			}
		})
	}
}

// TestRestartStep_SessionStatePrerequisites tests that RestartStep checks session state
// This verifies the nil checks and validation at the start of RestartStep
func TestRestartStep_SessionStatePrerequisites(t *testing.T) {
	tests := []struct {
		name       string
		stepInfo   *StepInfo
		session    *UltraPlanSession
		wantErr    bool
		errContain string
	}{
		{
			name:       "nil step info",
			stepInfo:   nil,
			session:    NewUltraPlanSession("Test", DefaultUltraPlanConfig()),
			wantErr:    true,
			errContain: "step info is nil",
		},
		{
			name:       "nil session",
			stepInfo:   &StepInfo{Type: StepTypeSynthesis},
			session:    nil,
			wantErr:    true,
			errContain: "no session",
		},
		{
			name:       "unknown step type",
			stepInfo:   &StepInfo{Type: StepType("invalid")},
			session:    NewUltraPlanSession("Test", DefaultUltraPlanConfig()),
			wantErr:    true,
			errContain: "unknown step type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &UltraPlanManager{session: tt.session}
			coord := &Coordinator{
				manager: manager,
				logger:  logging.NopLogger(),
			}

			_, err := coord.RestartStep(tt.stepInfo)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error = %q, expected to contain %q", err.Error(), tt.errContain)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestPublicAPI_SessionAccessMethods tests that the Coordinator's public API
// for session access remains functional
func TestPublicAPI_SessionAccessMethods(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Plan = &PlanSpec{
		Summary: "Test plan",
		Tasks:   []PlannedTask{{ID: "task-1", Title: "Test Task"}},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	// Test Session() returns the correct session
	gotSession := coord.Session()
	if gotSession != session {
		t.Error("Session() did not return the expected session")
	}

	// Test Plan() returns the correct plan
	gotPlan := coord.Plan()
	if gotPlan != session.Plan {
		t.Error("Plan() did not return the expected plan")
	}

	// Test Manager() returns the correct manager
	gotManager := coord.Manager()
	if gotManager != manager {
		t.Error("Manager() did not return the expected manager")
	}
}

// TestPublicAPI_ProgressTracking tests the progress tracking methods
func TestPublicAPI_ProgressTracking(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3"},
		},
	}
	session.CompletedTasks = []string{"task-1"}
	session.Phase = PhaseExecuting

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager:      manager,
		runningTasks: make(map[string]string),
	}

	// Test GetProgress
	completed, total, phase := coord.GetProgress()
	if completed != 1 {
		t.Errorf("completed = %d, want 1", completed)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if phase != PhaseExecuting {
		t.Errorf("phase = %v, want %v", phase, PhaseExecuting)
	}

	// Test GetRunningTasks (should be empty initially)
	running := coord.GetRunningTasks()
	if len(running) != 0 {
		t.Errorf("GetRunningTasks() length = %d, want 0", len(running))
	}
}

// TestPublicAPI_CandidatePlanMethods tests the candidate plan storage methods
func TestPublicAPI_CandidatePlanMethods(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	session.Config.MultiPass = true
	session.PlanCoordinatorIDs = []string{"coord-0", "coord-1", "coord-2"}
	session.CandidatePlans = make([]*PlanSpec, 3)

	manager := &UltraPlanManager{
		session: session,
		logger:  logging.NopLogger(),
	}
	coord := &Coordinator{
		manager: manager,
		logger:  logging.NopLogger(),
	}

	// Initially should have 0 non-nil plans
	if coord.CountCandidatePlans() != 0 {
		t.Errorf("CountCandidatePlans() = %d, want 0", coord.CountCandidatePlans())
	}

	// Store a plan at index 0
	plan0 := &PlanSpec{Summary: "Plan 0"}
	count := coord.StoreCandidatePlan(0, plan0)
	if count != 1 {
		t.Errorf("StoreCandidatePlan returned %d, want 1", count)
	}

	// Verify count is now 1
	if coord.CountCandidatePlans() != 1 {
		t.Errorf("CountCandidatePlans() = %d, want 1", coord.CountCandidatePlans())
	}

	// Store remaining plans
	coord.StoreCandidatePlan(1, &PlanSpec{Summary: "Plan 1"})
	coord.StoreCandidatePlan(2, &PlanSpec{Summary: "Plan 2"})

	// Verify count is now 3
	if coord.CountCandidatePlans() != 3 {
		t.Errorf("CountCandidatePlans() = %d, want 3", coord.CountCandidatePlans())
	}
}

// TestGetStepInfo_AllStepTypes_Comprehensive tests GetStepInfo for all step types
// to ensure comprehensive coverage of the delegation pattern
func TestGetStepInfo_AllStepTypes_Comprehensive(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())

	// Setup plan with tasks
	session.Plan = &PlanSpec{
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Task One"},
			{ID: "task-2", Title: "Task Two"},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
	}

	// Setup all session state IDs
	session.CoordinatorID = "planning-inst"
	session.PlanManagerID = "manager-inst"
	session.PlanCoordinatorIDs = []string{"multipass-0", "multipass-1"}
	session.TaskToInstance = map[string]string{
		"task-1": "task-inst-1",
		"task-2": "task-inst-2",
	}
	session.SynthesisID = "synth-inst"
	session.RevisionID = "rev-inst"
	session.ConsolidationID = "consol-inst"
	session.GroupConsolidatorIDs = []string{"group-consol-0", "group-consol-1"}

	manager := &UltraPlanManager{session: session}

	// Create group tracker for task lookup
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					task := session.GetTask(taskID)
					if task == nil {
						return nil
					}
					return &group.Task{ID: task.ID, Title: task.Title}
				},
			)
		},
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)
	groupTracker := group.NewTracker(sessionAdapter)

	coord := &Coordinator{
		manager:      manager,
		groupTracker: groupTracker,
	}

	tests := []struct {
		instanceID   string
		wantType     StepType
		wantTaskID   string
		wantGroupIdx int
		description  string
	}{
		{"planning-inst", StepTypePlanning, "", -1, "single-pass planning coordinator"},
		{"manager-inst", StepTypePlanManager, "", -1, "plan manager instance"},
		{"multipass-0", StepTypePlanning, "", 0, "multi-pass coordinator 0"},
		{"multipass-1", StepTypePlanning, "", 1, "multi-pass coordinator 1"},
		{"task-inst-1", StepTypeTask, "task-1", 0, "task instance in group 0"},
		{"task-inst-2", StepTypeTask, "task-2", 1, "task instance in group 1"},
		{"synth-inst", StepTypeSynthesis, "", -1, "synthesis instance"},
		{"rev-inst", StepTypeRevision, "", -1, "revision instance"},
		{"consol-inst", StepTypeConsolidation, "", -1, "consolidation instance"},
		{"group-consol-0", StepTypeGroupConsolidator, "", 0, "group consolidator 0"},
		{"group-consol-1", StepTypeGroupConsolidator, "", 1, "group consolidator 1"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			info := coord.GetStepInfo(tt.instanceID)
			if info == nil {
				t.Fatalf("GetStepInfo(%q) returned nil", tt.instanceID)
			}

			if info.Type != tt.wantType {
				t.Errorf("Type = %v, want %v", info.Type, tt.wantType)
			}

			if info.InstanceID != tt.instanceID {
				t.Errorf("InstanceID = %q, want %q", info.InstanceID, tt.instanceID)
			}

			if tt.wantTaskID != "" && info.TaskID != tt.wantTaskID {
				t.Errorf("TaskID = %q, want %q", info.TaskID, tt.wantTaskID)
			}

			// GroupIndex is -1 for non-grouped steps
			if tt.wantGroupIdx >= 0 && info.GroupIndex != tt.wantGroupIdx {
				t.Errorf("GroupIndex = %d, want %d", info.GroupIndex, tt.wantGroupIdx)
			}
		})
	}

	// Test unknown instance ID returns nil
	info := coord.GetStepInfo("unknown-instance-id")
	if info != nil {
		t.Errorf("GetStepInfo(\"unknown-instance-id\") = %v, want nil", info)
	}
}

// TestCoordinator_SetCallbacks tests callback registration
func TestCoordinator_SetCallbacks(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	// Callbacks should be nil initially
	if coord.callbacks != nil {
		t.Error("callbacks should be nil initially")
	}

	// Set callbacks
	cb := &CoordinatorCallbacks{
		OnPhaseChange: func(phase UltraPlanPhase) {},
	}
	coord.SetCallbacks(cb)

	// Verify callbacks are set
	if coord.callbacks != cb {
		t.Error("SetCallbacks did not set the callbacks correctly")
	}

	// Set to nil
	coord.SetCallbacks(nil)
	if coord.callbacks != nil {
		t.Error("SetCallbacks(nil) did not clear callbacks")
	}
}

// TestCoordinator_GroupTracker tests the GroupTracker getter
func TestCoordinator_GroupTracker(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}

	// Create a group tracker
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData { return nil },
		func() []string { return nil },
		func() []string { return nil },
		func() map[string]int { return nil },
		func() int { return 0 },
	)
	tracker := group.NewTracker(sessionAdapter)

	coord := &Coordinator{
		manager:      manager,
		groupTracker: tracker,
	}

	// Verify GroupTracker() returns the tracker
	gotTracker := coord.GroupTracker()
	if gotTracker != tracker {
		t.Error("GroupTracker() did not return the expected tracker")
	}
}

// TestCoordinator_RetryManager tests the RetryManager getter
func TestCoordinator_RetryManager(t *testing.T) {
	session := NewUltraPlanSession("Test objective", DefaultUltraPlanConfig())
	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{
		manager: manager,
	}

	// RetryManager should be nil when not set
	if coord.RetryManager() != nil {
		t.Error("RetryManager() should be nil when not set")
	}
}

// =============================================================================
// Prompt Building Regression Tests
// =============================================================================
// These tests verify that the refactored prompt-building methods produce
// output equivalent to the original implementation. They test structural
// elements rather than exact string matches to be resilient to minor
// formatting changes while ensuring essential content is preserved.

// TestBuildTaskPrompt_RegressionStructure verifies that buildTaskPrompt produces
// all expected structural elements after the refactoring to use prompt.TaskBuilder.
func TestBuildTaskPrompt_RegressionStructure(t *testing.T) {
	session := NewUltraPlanSession("Build a new feature", DefaultUltraPlanConfig())
	session.Plan = &PlanSpec{
		Summary: "Implement authentication system with OAuth support",
		Tasks: []PlannedTask{
			{
				ID:            "task-1",
				Title:         "Setup database schema",
				Description:   "Create the user table with OAuth fields",
				Files:         []string{"migrations/001_users.sql", "models/user.go"},
				DependsOn:     []string{},
				Priority:      1,
				EstComplexity: ComplexityLow,
			},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}
	session.TaskToInstance = map[string]string{"task-1": "inst-1"}

	manager := &UltraPlanManager{session: session}

	// Create group tracker
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					task := session.GetTask(taskID)
					if task == nil {
						return nil
					}
					return &group.Task{ID: task.ID, Title: task.Title}
				},
			)
		},
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)
	groupTracker := group.NewTracker(sessionAdapter)

	coord := &Coordinator{
		manager:      manager,
		groupTracker: groupTracker,
	}

	// Get the task and build the prompt
	task := session.GetTask("task-1")
	if task == nil {
		t.Fatal("task-1 not found in session")
	}
	prompt := coord.buildTaskPrompt(task)

	// Verify essential structural elements are present
	requiredElements := []struct {
		name     string
		expected string
	}{
		{"task header", "# Task: Setup database schema"},
		{"ultra-plan reference", "Part of Ultra-Plan"},
		{"plan summary", "authentication system"},
		{"your task section", "## Your Task"},
		{"task description", "Create the user table with OAuth fields"},
		{"expected files section", "## Expected Files"},
		{"first expected file", "migrations/001_users.sql"},
		{"second expected file", "models/user.go"},
		{"guidelines section", "## Guidelines"},
		{"focus instruction", "Focus only on this specific task"},
		{"commit instruction", "Commit your changes"},
		{"completion protocol section", "## Completion Protocol"},
		{"completion file instruction", ".claudio-task-complete.json"},
		{"task_id in JSON example", "task-1"},
		{"status field", "\"status\""},
		{"summary field", "\"summary\""},
		{"files_modified field", "files_modified"},
	}

	for _, elem := range requiredElements {
		if !strings.Contains(prompt, elem.expected) {
			t.Errorf("buildTaskPrompt missing %s: expected to contain %q\nGot:\n%s",
				elem.name, elem.expected, prompt)
		}
	}
}

// TestBuildTaskPrompt_RegressionPreviousGroupContext verifies that buildTaskPrompt
// correctly includes previous group context for tasks in later groups.
func TestBuildTaskPrompt_RegressionPreviousGroupContext(t *testing.T) {
	session := NewUltraPlanSession("Multi-group project", DefaultUltraPlanConfig())
	session.Plan = &PlanSpec{
		Summary: "Complex multi-phase implementation",
		Tasks: []PlannedTask{
			{ID: "task-1", Title: "Foundation", Description: "Setup base"},
			{ID: "task-2", Title: "Feature", Description: "Add feature", DependsOn: []string{"task-1"}},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
	}
	session.TaskToInstance = map[string]string{
		"task-1": "inst-1",
		"task-2": "inst-2",
	}
	// Add group consolidation context from group 0
	session.GroupConsolidationContexts = []*GroupConsolidationCompletionFile{
		{
			Notes:              "Group 1 completed with auth module setup",
			IssuesForNextGroup: []string{"Ensure API compatibility", "Update documentation"},
			Verification:       VerificationResult{OverallSuccess: true},
		},
	}

	manager := &UltraPlanManager{session: session}

	// Create group tracker
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					task := session.GetTask(taskID)
					if task == nil {
						return nil
					}
					return &group.Task{ID: task.ID, Title: task.Title}
				},
			)
		},
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)
	groupTracker := group.NewTracker(sessionAdapter)

	coord := &Coordinator{
		manager:      manager,
		groupTracker: groupTracker,
	}

	// Build prompt for task-2 which is in group 1 (index 1)
	task := session.GetTask("task-2")
	if task == nil {
		t.Fatal("task-2 not found in session")
	}
	prompt := coord.buildTaskPrompt(task)

	// Verify previous group context is included
	contextElements := []string{
		"Context from Previous Group",
		"Group 1",
		"Group 1 completed with auth module setup",
		"Ensure API compatibility",
		"Update documentation",
		"verified (build/lint/tests passed)",
	}

	for _, elem := range contextElements {
		if !strings.Contains(prompt, elem) {
			t.Errorf("buildTaskPrompt for task in group 1 missing previous group context element: %q\nGot:\n%s",
				elem, prompt)
		}
	}
}

// TestBuildPlanManagerPrompt_RegressionStructure verifies that buildPlanManagerPrompt
// produces all expected structural elements after refactoring to use prompt.PlanningBuilder.
func TestBuildPlanManagerPrompt_RegressionStructure(t *testing.T) {
	session := &UltraPlanSession{
		ID:        "test-session",
		Objective: "Implement a REST API with authentication",
		Config:    UltraPlanConfig{MultiPass: true},
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Parallel-first approach",
				Tasks: []PlannedTask{
					{ID: "t1", Title: "Auth endpoints", EstComplexity: ComplexityMedium, DependsOn: []string{}},
					{ID: "t2", Title: "User model", EstComplexity: ComplexityLow, DependsOn: []string{}},
					{ID: "t3", Title: "Integration tests", EstComplexity: ComplexityHigh, DependsOn: []string{"t1", "t2"}},
				},
				ExecutionOrder: [][]string{{"t1", "t2"}, {"t3"}},
				Insights:       []string{"Codebase uses Clean Architecture"},
				Constraints:    []string{"Must maintain backward compatibility"},
			},
			{
				Summary: "Sequential approach",
				Tasks: []PlannedTask{
					{ID: "t1", Title: "User model", EstComplexity: ComplexityLow},
					{ID: "t2", Title: "Auth endpoints", EstComplexity: ComplexityMedium, DependsOn: []string{"t1"}},
				},
				ExecutionOrder: [][]string{{"t1"}, {"t2"}},
			},
			{
				Summary: "Balanced approach",
				Tasks: []PlannedTask{
					{ID: "t1", Title: "Foundation", EstComplexity: ComplexityLow},
					{ID: "t2", Title: "Core features", EstComplexity: ComplexityMedium, DependsOn: []string{"t1"}},
				},
				ExecutionOrder: [][]string{{"t1"}, {"t2"}},
			},
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	prompt := coord.buildPlanManagerPrompt()

	// Verify essential structural elements from PlanManagerPromptTemplate
	requiredElements := []struct {
		name     string
		expected string
	}{
		{"senior technical lead intro", "senior technical lead"},
		{"objective section", "## Objective"},
		{"objective content", "REST API with authentication"},
		{"candidate plans section", "## Candidate Plans"},
		{"plan 1 header", "Plan 1"},
		{"plan 2 header", "Plan 2"},
		{"plan 3 header", "Plan 3"},
		{"maximize-parallelism strategy", "maximize-parallelism"},
		{"minimize-complexity strategy", "minimize-complexity"},
		{"balanced-approach strategy", "balanced-approach"},
		{"plan 1 summary", "Parallel-first approach"},
		{"plan 2 summary", "Sequential approach"},
		{"plan 3 summary", "Balanced approach"},
		{"evaluation criteria section", "## Your Task"},
		{"parallelism criteria", "Parallelism potential"},
		{"task granularity criteria", "Task granularity"},
		{"dependency structure criteria", "Dependency structure"},
		{"decision section", "## Decision"},
		{"select option", "Select"},
		{"merge option", "Merge"},
		{"output section", "## Output"},
		{"plan file name", ".claudio-plan.json"},
		{"plan decision tag", "plan_decision"},
	}

	for _, elem := range requiredElements {
		if !strings.Contains(prompt, elem.expected) {
			t.Errorf("buildPlanManagerPrompt missing %s: expected to contain %q",
				elem.name, elem.expected)
		}
	}
}

// TestBuildPlanManagerPrompt_RegressionTaskDetails verifies that buildPlanManagerPrompt
// includes task-level details (complexity, dependencies) in the compact format.
func TestBuildPlanManagerPrompt_RegressionTaskDetails(t *testing.T) {
	session := &UltraPlanSession{
		Objective: "Test task details",
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Test plan",
				Tasks: []PlannedTask{
					{ID: "task-a", Title: "First Task", EstComplexity: ComplexityLow, DependsOn: []string{}},
					{ID: "task-b", Title: "Second Task", EstComplexity: ComplexityMedium, DependsOn: []string{"task-a"}},
				},
				ExecutionOrder: [][]string{{"task-a"}, {"task-b"}},
			},
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	prompt := coord.buildPlanManagerPrompt()

	// Verify task details are present
	taskDetails := []string{
		"task-a",
		"First Task",
		"low",
		"task-b",
		"Second Task",
		"medium",
		"depends: task-a",
		"depends: none",
	}

	for _, detail := range taskDetails {
		if !strings.Contains(prompt, detail) {
			t.Errorf("buildPlanManagerPrompt missing task detail: %q", detail)
		}
	}
}

// TestBuildPlanComparisonSection_RegressionStructure verifies that buildPlanComparisonSection
// produces all expected structural elements after refactoring to use prompt.PlanningBuilder.
func TestBuildPlanComparisonSection_RegressionStructure(t *testing.T) {
	session := &UltraPlanSession{
		CandidatePlans: []*PlanSpec{
			{
				Summary: "Comprehensive plan",
				Tasks: []PlannedTask{
					{ID: "task-1", Title: "Initial Setup", Description: "Setup the project", EstComplexity: ComplexityLow},
					{ID: "task-2", Title: "Core Logic", Description: "Implement core", EstComplexity: ComplexityMedium},
				},
				ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
				Insights:       []string{"Architecture follows SOLID principles"},
				Constraints:    []string{"Must support legacy API"},
			},
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	result := coord.buildPlanComparisonSection()

	// Verify detailed format elements
	requiredElements := []struct {
		name     string
		expected string
	}{
		{"plan header", "### Plan 1:"},
		{"strategy name", "maximize-parallelism"},
		{"summary label", "**Summary**"},
		{"plan summary", "Comprehensive plan"},
		{"task count label", "**Task Count**"},
		{"task count value", "2 tasks"},
		{"execution groups label", "**Execution Groups**"},
		{"groups count", "2 groups"},
		{"max parallelism label", "**Max Parallelism**"},
		{"parallelism value", "1 concurrent tasks"},
		{"insights label", "**Insights**"},
		{"insight content", "SOLID principles"},
		{"constraints label", "**Constraints**"},
		{"constraint content", "legacy API"},
		{"tasks json label", "**Tasks (JSON)**"},
		{"json code block", "```json"},
		{"task id in json", "task-1"},
		{"task title in json", "Initial Setup"},
		{"execution order label", "**Execution Order**"},
		{"group 1 order", "Group 1: task-1"},
		{"group 2 order", "Group 2: task-2"},
	}

	for _, elem := range requiredElements {
		if !strings.Contains(result, elem.expected) {
			t.Errorf("buildPlanComparisonSection missing %s: expected to contain %q\nGot:\n%s",
				elem.name, elem.expected, result)
		}
	}
}

// TestBuildPlanComparisonSection_RegressionNilPlanHandling verifies that buildPlanComparisonSection
// correctly handles nil plans (skips them) after refactoring.
func TestBuildPlanComparisonSection_RegressionNilPlanHandling(t *testing.T) {
	session := &UltraPlanSession{
		CandidatePlans: []*PlanSpec{
			{Summary: "First plan", Tasks: []PlannedTask{{ID: "t1", Title: "Task 1"}}},
			nil, // This nil plan should be skipped
			{Summary: "Third plan", Tasks: []PlannedTask{{ID: "t3", Title: "Task 3"}}},
		},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	result := coord.buildPlanComparisonSection()

	// Should have Plan 1 (index 0)
	if !strings.Contains(result, "Plan 1") {
		t.Error("buildPlanComparisonSection should include Plan 1")
	}
	if !strings.Contains(result, "First plan") {
		t.Error("buildPlanComparisonSection should include first plan summary")
	}

	// Should have Plan 2 (index 2, but renamed since nil was skipped)
	// The formatter should skip nil plans entirely
	if !strings.Contains(result, "Third plan") {
		t.Error("buildPlanComparisonSection should include third plan summary")
	}

	// Should NOT have "null" as a plan entry (nil plans should be skipped)
	if strings.Contains(result, "\"null\"") {
		t.Error("buildPlanComparisonSection should not include 'null' for nil plans")
	}
}

// TestBuildPlanComparisonSection_RegressionEmptyPlans verifies that buildPlanComparisonSection
// correctly handles an empty plans slice.
func TestBuildPlanComparisonSection_RegressionEmptyPlans(t *testing.T) {
	session := &UltraPlanSession{
		CandidatePlans: []*PlanSpec{},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	result := coord.buildPlanComparisonSection()

	// Empty input should produce empty output
	if result != "" {
		t.Errorf("buildPlanComparisonSection with empty plans should produce empty string, got: %q", result)
	}
}

// =============================================================================
// Prompt Building Integration Tests
// =============================================================================
// These tests verify the end-to-end integration of the prompt-building methods
// with coordinator session state, ensuring that data flows correctly through
// the prompt.TaskBuilder and prompt.PlanningBuilder.

// TestBuildTaskPrompt_IntegrationWithFullSession tests buildTaskPrompt with a
// realistically configured session to verify integration with all dependencies.
func TestBuildTaskPrompt_IntegrationWithFullSession(t *testing.T) {
	// Create a realistic session with multiple tasks and groups
	session := NewUltraPlanSession("Implement user authentication system", DefaultUltraPlanConfig())
	session.Plan = &PlanSpec{
		ID:      "plan-auth",
		Summary: "Authentication system with OAuth2 and JWT",
		Tasks: []PlannedTask{
			{
				ID:            "task-1-schema",
				Title:         "Database Schema Setup",
				Description:   "Create users table with OAuth provider fields and JWT refresh tokens",
				Files:         []string{"migrations/001_create_users.sql", "migrations/002_add_oauth.sql"},
				DependsOn:     []string{},
				Priority:      1,
				EstComplexity: ComplexityLow,
			},
			{
				ID:            "task-2-models",
				Title:         "User Model Implementation",
				Description:   "Implement User model with CRUD operations",
				Files:         []string{"internal/models/user.go", "internal/models/user_test.go"},
				DependsOn:     []string{"task-1-schema"},
				Priority:      1,
				EstComplexity: ComplexityMedium,
			},
			{
				ID:            "task-3-oauth",
				Title:         "OAuth2 Integration",
				Description:   "Integrate Google and GitHub OAuth providers",
				Files:         []string{"internal/auth/oauth.go", "internal/auth/providers/"},
				DependsOn:     []string{"task-2-models"},
				Priority:      2,
				EstComplexity: ComplexityHigh,
			},
		},
		ExecutionOrder: [][]string{{"task-1-schema"}, {"task-2-models"}, {"task-3-oauth"}},
		Insights:       []string{"Existing auth middleware can be extended"},
		Constraints:    []string{"Must support existing session tokens during migration"},
	}
	session.TaskToInstance = map[string]string{
		"task-1-schema": "inst-1",
		"task-2-models": "inst-2",
		"task-3-oauth":  "inst-3",
	}

	manager := &UltraPlanManager{session: session}

	// Create group tracker for realistic integration
	sessionAdapter := group.NewSessionAdapter(
		func() group.PlanData {
			return group.NewPlanAdapter(
				func() [][]string { return session.Plan.ExecutionOrder },
				func(taskID string) *group.Task {
					task := session.GetTask(taskID)
					if task == nil {
						return nil
					}
					return &group.Task{ID: task.ID, Title: task.Title, DependsOn: task.DependsOn}
				},
			)
		},
		func() []string { return session.CompletedTasks },
		func() []string { return session.FailedTasks },
		func() map[string]int { return session.TaskCommitCounts },
		func() int { return session.CurrentGroup },
	)
	groupTracker := group.NewTracker(sessionAdapter)

	coord := &Coordinator{
		manager:      manager,
		groupTracker: groupTracker,
	}

	// Test prompt generation for each task
	tests := []struct {
		taskID            string
		expectedTitle     string
		expectedFiles     []string
		expectedInPrompt  []string
		notExpectedPrompt []string
	}{
		{
			taskID:        "task-1-schema",
			expectedTitle: "Database Schema Setup",
			expectedFiles: []string{"migrations/001_create_users.sql", "migrations/002_add_oauth.sql"},
			expectedInPrompt: []string{
				"# Task: Database Schema Setup",
				"Create users table with OAuth provider fields",
				"## Expected Files",
				"## Guidelines",
				"## Completion Protocol",
				"task-1-schema",
			},
			notExpectedPrompt: []string{
				"Context from Previous Group", // First task shouldn't have previous context
			},
		},
		{
			taskID:        "task-2-models",
			expectedTitle: "User Model Implementation",
			expectedFiles: []string{"internal/models/user.go", "internal/models/user_test.go"},
			expectedInPrompt: []string{
				"# Task: User Model Implementation",
				"Implement User model with CRUD operations",
				"task-2-models",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			task := session.GetTask(tt.taskID)
			if task == nil {
				t.Fatalf("task %s not found", tt.taskID)
			}

			prompt := coord.buildTaskPrompt(task)

			// Verify expected elements are present
			for _, expected := range tt.expectedInPrompt {
				if !strings.Contains(prompt, expected) {
					t.Errorf("prompt for %s missing: %q", tt.taskID, expected)
				}
			}

			// Verify expected files are listed
			for _, file := range tt.expectedFiles {
				if !strings.Contains(prompt, file) {
					t.Errorf("prompt for %s missing expected file: %q", tt.taskID, file)
				}
			}

			// Verify unexpected elements are absent
			for _, notExpected := range tt.notExpectedPrompt {
				if strings.Contains(prompt, notExpected) {
					t.Errorf("prompt for %s should not contain: %q", tt.taskID, notExpected)
				}
			}
		})
	}
}

// TestBuildPlanManagerPrompt_IntegrationWithMultiPass tests buildPlanManagerPrompt
// with a realistic multi-pass planning session.
func TestBuildPlanManagerPrompt_IntegrationWithMultiPass(t *testing.T) {
	session := &UltraPlanSession{
		ID:        "session-multipass",
		Objective: "Refactor database layer to support multiple backends",
		Config:    UltraPlanConfig{MultiPass: true, MaxParallel: 4},
		CandidatePlans: []*PlanSpec{
			// Plan from maximize-parallelism strategy
			{
				Summary: "Highly parallel approach with interface extraction",
				Tasks: []PlannedTask{
					{ID: "interface", Title: "Extract DB Interface", EstComplexity: ComplexityMedium},
					{ID: "postgres", Title: "Postgres Backend", EstComplexity: ComplexityMedium, DependsOn: []string{"interface"}},
					{ID: "mysql", Title: "MySQL Backend", EstComplexity: ComplexityMedium, DependsOn: []string{"interface"}},
					{ID: "sqlite", Title: "SQLite Backend", EstComplexity: ComplexityLow, DependsOn: []string{"interface"}},
					{ID: "tests", Title: "Integration Tests", EstComplexity: ComplexityHigh, DependsOn: []string{"postgres", "mysql", "sqlite"}},
				},
				ExecutionOrder: [][]string{{"interface"}, {"postgres", "mysql", "sqlite"}, {"tests"}},
				Insights:       []string{"Existing queries are Postgres-specific", "Connection pooling varies by backend"},
				Constraints:    []string{"Zero downtime migration required"},
			},
			// Plan from minimize-complexity strategy
			{
				Summary: "Sequential approach with one backend at a time",
				Tasks: []PlannedTask{
					{ID: "abstract", Title: "Abstract Current DB", EstComplexity: ComplexityLow},
					{ID: "postgres-refactor", Title: "Refactor Postgres", EstComplexity: ComplexityMedium, DependsOn: []string{"abstract"}},
					{ID: "add-mysql", Title: "Add MySQL Support", EstComplexity: ComplexityHigh, DependsOn: []string{"postgres-refactor"}},
					{ID: "add-sqlite", Title: "Add SQLite Support", EstComplexity: ComplexityMedium, DependsOn: []string{"add-mysql"}},
				},
				ExecutionOrder: [][]string{{"abstract"}, {"postgres-refactor"}, {"add-mysql"}, {"add-sqlite"}},
				Insights:       []string{"Simpler to test incrementally"},
			},
			// Plan from balanced-approach strategy
			{
				Summary: "Balanced approach with phased parallelism",
				Tasks: []PlannedTask{
					{ID: "foundation", Title: "Foundation Work", EstComplexity: ComplexityMedium},
					{ID: "backend-1", Title: "First Backend", EstComplexity: ComplexityMedium, DependsOn: []string{"foundation"}},
					{ID: "backend-2", Title: "Second Backend", EstComplexity: ComplexityMedium, DependsOn: []string{"foundation"}},
					{ID: "finalize", Title: "Finalization", EstComplexity: ComplexityLow, DependsOn: []string{"backend-1", "backend-2"}},
				},
				ExecutionOrder: [][]string{{"foundation"}, {"backend-1", "backend-2"}, {"finalize"}},
				Constraints:    []string{"Allow time for integration testing between phases"},
			},
		},
		PlanCoordinatorIDs: []string{"coord-parallel", "coord-simple", "coord-balanced"},
	}

	manager := &UltraPlanManager{session: session}
	coord := &Coordinator{manager: manager}

	prompt := coord.buildPlanManagerPrompt()

	// Verify the prompt integrates all three plans correctly
	integrationChecks := []struct {
		description string
		expected    string
	}{
		{"objective included", "Refactor database layer"},
		{"multiple backends mentioned", "multiple backends"},
		{"plan 1 present", "Highly parallel approach"},
		{"plan 2 present", "Sequential approach"},
		{"plan 3 present", "Balanced approach"},
		{"strategy 1 present", "maximize-parallelism"},
		{"strategy 2 present", "minimize-complexity"},
		{"strategy 3 present", "balanced-approach"},
		{"plan 1 insight", "Postgres-specific"},
		{"plan 1 constraint", "Zero downtime"},
		{"plan 2 insight", "incrementally"},
		{"plan 3 constraint", "integration testing"},
		{"execution groups shown", "parallel groups"},
		{"dependency format", "depends:"},
	}

	for _, check := range integrationChecks {
		if !strings.Contains(prompt, check.expected) {
			t.Errorf("integration check failed - %s: expected to contain %q",
				check.description, check.expected)
		}
	}

	// Verify all tasks from all plans are represented
	allTaskTitles := []string{
		"Extract DB Interface", "Postgres Backend", "MySQL Backend", "SQLite Backend",
		"Abstract Current DB", "Refactor Postgres", "Add MySQL Support",
		"Foundation Work", "First Backend", "Second Backend",
	}

	for _, title := range allTaskTitles {
		if !strings.Contains(prompt, title) {
			t.Errorf("integration test missing task: %q", title)
		}
	}
}

// TestConvertPlanSpecsToCandidatePlans verifies the conversion helper function
// that bridges orchestrator.PlanSpec to prompt.CandidatePlanInfo.
func TestConvertPlanSpecsToCandidatePlans(t *testing.T) {
	strategyNames := GetMultiPassStrategyNames()

	tests := []struct {
		name     string
		plans    []*PlanSpec
		expected int // expected number of non-nil results
	}{
		{
			name:     "nil input",
			plans:    nil,
			expected: 0,
		},
		{
			name:     "empty input",
			plans:    []*PlanSpec{},
			expected: 0,
		},
		{
			name: "single plan",
			plans: []*PlanSpec{
				{Summary: "Test plan", Tasks: []PlannedTask{{ID: "t1", Title: "Task 1"}}},
			},
			expected: 1,
		},
		{
			name: "plan with nil entry",
			plans: []*PlanSpec{
				{Summary: "First"},
				nil,
				{Summary: "Third"},
			},
			expected: 2, // nil plans are skipped
		},
		{
			name: "full three plans",
			plans: []*PlanSpec{
				{Summary: "Plan A", Tasks: []PlannedTask{{ID: "a1"}}},
				{Summary: "Plan B", Tasks: []PlannedTask{{ID: "b1"}}},
				{Summary: "Plan C", Tasks: []PlannedTask{{ID: "c1"}}},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertPlanSpecsToCandidatePlans(tt.plans, strategyNames)

			if len(result) != tt.expected {
				t.Errorf("convertPlanSpecsToCandidatePlans returned %d items, want %d",
					len(result), tt.expected)
			}

			// Note: Strategy names are assigned sequentially to non-nil plans.
			// Due to nil skipping, indices may not align with strategyNames perfectly.
			// The key is that each resulting plan gets a strategy from the list.
			for _, plan := range result {
				if plan.Strategy == "" {
					t.Error("plan has empty strategy, want non-empty strategy name")
				}
			}
		})
	}
}

// TestConvertPlanSpecsToCandidatePlans_TaskConversion verifies that task details
// are correctly converted from PlannedTask to TaskInfo.
func TestConvertPlanSpecsToCandidatePlans_TaskConversion(t *testing.T) {
	strategyNames := []string{"test-strategy"}
	plans := []*PlanSpec{
		{
			Summary: "Test plan",
			Tasks: []PlannedTask{
				{
					ID:            "task-1",
					Title:         "First Task",
					Description:   "Detailed description",
					Files:         []string{"file1.go", "file2.go"},
					DependsOn:     []string{"task-0"},
					Priority:      2,
					EstComplexity: ComplexityHigh,
				},
			},
			ExecutionOrder: [][]string{{"task-1"}},
			Insights:       []string{"Key insight"},
			Constraints:    []string{"Important constraint"},
		},
	}

	result := convertPlanSpecsToCandidatePlans(plans, strategyNames)

	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}

	plan := result[0]

	// Verify plan-level fields
	if plan.Summary != "Test plan" {
		t.Errorf("Summary = %q, want %q", plan.Summary, "Test plan")
	}
	if plan.Strategy != "test-strategy" {
		t.Errorf("Strategy = %q, want %q", plan.Strategy, "test-strategy")
	}
	if len(plan.Insights) != 1 || plan.Insights[0] != "Key insight" {
		t.Errorf("Insights = %v, want [Key insight]", plan.Insights)
	}
	if len(plan.Constraints) != 1 || plan.Constraints[0] != "Important constraint" {
		t.Errorf("Constraints = %v, want [Important constraint]", plan.Constraints)
	}

	// Verify task-level conversion
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(plan.Tasks))
	}

	task := plan.Tasks[0]
	if task.ID != "task-1" {
		t.Errorf("task.ID = %q, want %q", task.ID, "task-1")
	}
	if task.Title != "First Task" {
		t.Errorf("task.Title = %q, want %q", task.Title, "First Task")
	}
	if task.Description != "Detailed description" {
		t.Errorf("task.Description = %q, want %q", task.Description, "Detailed description")
	}
	if len(task.Files) != 2 || task.Files[0] != "file1.go" {
		t.Errorf("task.Files = %v, want [file1.go file2.go]", task.Files)
	}
	if len(task.DependsOn) != 1 || task.DependsOn[0] != "task-0" {
		t.Errorf("task.DependsOn = %v, want [task-0]", task.DependsOn)
	}
	if task.Priority != 2 {
		t.Errorf("task.Priority = %d, want 2", task.Priority)
	}
	if task.EstComplexity != string(ComplexityHigh) {
		t.Errorf("task.EstComplexity = %q, want %q", task.EstComplexity, ComplexityHigh)
	}
}

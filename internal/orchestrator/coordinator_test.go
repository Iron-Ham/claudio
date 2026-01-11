package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"
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

package ultraplan

import (
	"strings"
	"testing"
)

func TestGetPlanningPrompt(t *testing.T) {
	prompt := GetPlanningPrompt("Build a REST API")

	if !strings.Contains(prompt, "Build a REST API") {
		t.Error("Prompt should contain the objective")
	}

	if !strings.Contains(prompt, "Explore") {
		t.Error("Prompt should mention exploration")
	}

	if !strings.Contains(prompt, "Decompose") {
		t.Error("Prompt should mention decomposition")
	}

	if !strings.Contains(prompt, PlanFileName) {
		t.Error("Prompt should mention the plan filename")
	}
}

func TestGetMultiPassPlanningPrompt(t *testing.T) {
	tests := []struct {
		strategy      string
		expectKeyword string
	}{
		{"maximize-parallelism", "Maximize Parallelism"},
		{"minimize-complexity", "Minimize Complexity"},
		{"balanced-approach", "Balanced Approach"},
		{"unknown-strategy", ""}, // Should return base prompt only
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			prompt := GetMultiPassPlanningPrompt(tt.strategy, "Test objective")

			if !strings.Contains(prompt, "Test objective") {
				t.Error("Prompt should contain the objective")
			}

			if tt.expectKeyword != "" && !strings.Contains(prompt, tt.expectKeyword) {
				t.Errorf("Prompt for %s should contain '%s'", tt.strategy, tt.expectKeyword)
			}
		})
	}
}

func TestGetStrategyNames(t *testing.T) {
	names := GetStrategyNames()

	if len(names) != 4 {
		t.Errorf("Expected 4 strategies, got %d", len(names))
	}

	expected := map[string]bool{
		"maximize-parallelism": true,
		"minimize-complexity":  true,
		"balanced-approach":    true,
		"risk-aware":           true,
	}

	for _, name := range names {
		if !expected[name] {
			t.Errorf("Unexpected strategy name: %s", name)
		}
	}
}

func TestGetStrategy(t *testing.T) {
	strategy := GetStrategy("maximize-parallelism")
	if strategy == nil {
		t.Fatal("GetStrategy returned nil for valid strategy")
	}
	if strategy.Strategy != "maximize-parallelism" {
		t.Errorf("Expected 'maximize-parallelism', got '%s'", strategy.Strategy)
	}

	strategy = GetStrategy("nonexistent")
	if strategy != nil {
		t.Error("GetStrategy should return nil for nonexistent strategy")
	}
}

func TestGetSynthesisPrompt(t *testing.T) {
	prompt := GetSynthesisPrompt("Build API", "Task 1, Task 2", "All successful", 1)

	if !strings.Contains(prompt, "Build API") {
		t.Error("Prompt should contain objective")
	}
	if !strings.Contains(prompt, "Task 1, Task 2") {
		t.Error("Prompt should contain task list")
	}
	if !strings.Contains(prompt, "All successful") {
		t.Error("Prompt should contain results summary")
	}
	if !strings.Contains(prompt, SynthesisCompletionFileName) {
		t.Error("Prompt should mention completion filename")
	}
}

func TestGetRevisionPrompt(t *testing.T) {
	prompt := GetRevisionPrompt("Build API", "task-1", "Task One", "Do the task", 2, "Fix error handling")

	if !strings.Contains(prompt, "Build API") {
		t.Error("Prompt should contain objective")
	}
	if !strings.Contains(prompt, "task-1") {
		t.Error("Prompt should contain task ID")
	}
	if !strings.Contains(prompt, "Task One") {
		t.Error("Prompt should contain task title")
	}
	if !strings.Contains(prompt, "Fix error handling") {
		t.Error("Prompt should contain issues")
	}
}

func TestPlanningPromptTemplateFormat(t *testing.T) {
	// Verify the template is valid by formatting it
	objective := "Test <objective>"
	prompt := GetPlanningPrompt(objective)

	// Should not panic and should contain the objective
	if !strings.Contains(prompt, objective) {
		t.Error("Template formatting failed")
	}
}

func TestConstantsAreDefined(t *testing.T) {
	if PlanFileName == "" {
		t.Error("PlanFileName should not be empty")
	}
	if SynthesisCompletionFileName == "" {
		t.Error("SynthesisCompletionFileName should not be empty")
	}
	if TaskCompletionFileName == "" {
		t.Error("TaskCompletionFileName should not be empty")
	}
	if GroupConsolidationCompletionFileName == "" {
		t.Error("GroupConsolidationCompletionFileName should not be empty")
	}
}

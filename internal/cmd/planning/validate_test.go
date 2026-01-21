package planning

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// captureValidateOutput captures stdout during function execution by temporarily
// redirecting os.Stdout to a pipe. Used for testing output of validation commands.
// Panics if pipe operations fail to ensure test infrastructure issues are visible.
func captureValidateOutput(f func()) string {
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		panic("failed to create pipe: " + err.Error())
	}
	os.Stdout = w

	f()

	if err := w.Close(); err != nil {
		panic("failed to close pipe writer: " + err.Error())
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		panic("failed to copy pipe output: " + err.Error())
	}
	return buf.String()
}

func TestConvertToUltraplanSpec(t *testing.T) {
	// Test nil plan
	result := convertToUltraplanSpec(nil)
	if result != nil {
		t.Error("Expected nil for nil input")
	}

	// Test valid conversion
	plan := &orchestrator.PlanSpec{
		ID:        "test-plan",
		Objective: "Test objective",
		Summary:   "Test summary",
		Tasks: []orchestrator.PlannedTask{
			{
				ID:            "task-1",
				Title:         "Task 1",
				Description:   "First task",
				Files:         []string{"file1.go"},
				DependsOn:     []string{},
				Priority:      0,
				EstComplexity: orchestrator.ComplexityLow,
				IssueURL:      "https://github.com/example/issues/1",
				NoCode:        false,
			},
			{
				ID:            "task-2",
				Title:         "Task 2",
				Description:   "Second task",
				Files:         []string{"file2.go"},
				DependsOn:     []string{"task-1"},
				Priority:      1,
				EstComplexity: orchestrator.ComplexityMedium,
				NoCode:        true,
			},
		},
		DependencyGraph: map[string][]string{
			"task-1": {},
			"task-2": {"task-1"},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
		Insights:       []string{"insight 1"},
		Constraints:    []string{"constraint 1"},
		CreatedAt:      time.Now(),
	}

	ultraplanSpec := convertToUltraplanSpec(plan)

	if ultraplanSpec.ID != "test-plan" {
		t.Errorf("Expected ID 'test-plan', got %q", ultraplanSpec.ID)
	}
	if ultraplanSpec.Objective != "Test objective" {
		t.Errorf("Expected Objective 'Test objective', got %q", ultraplanSpec.Objective)
	}
	if len(ultraplanSpec.Tasks) != 2 {
		t.Errorf("Expected 2 tasks, got %d", len(ultraplanSpec.Tasks))
	}

	// Check task conversion
	task1 := ultraplanSpec.Tasks[0]
	if task1.ID != "task-1" {
		t.Errorf("Expected task ID 'task-1', got %q", task1.ID)
	}
	if task1.IssueURL != "https://github.com/example/issues/1" {
		t.Errorf("Expected IssueURL preserved, got %q", task1.IssueURL)
	}
	if task1.EstComplexity != ultraplan.ComplexityLow {
		t.Errorf("Expected ComplexityLow, got %q", task1.EstComplexity)
	}

	task2 := ultraplanSpec.Tasks[1]
	if !task2.NoCode {
		t.Error("Expected NoCode to be true for task-2")
	}
	if len(task2.DependsOn) != 1 || task2.DependsOn[0] != "task-1" {
		t.Errorf("Expected DependsOn ['task-1'], got %v", task2.DependsOn)
	}
}

func TestRunValidate_FileNotFound(t *testing.T) {
	// Ensure validateJSON is false for this test
	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	err := runValidate(validateCmd, []string{"nonexistent.json"})
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Expected 'file not found' error, got: %v", err)
	}
}

func TestRunValidate_InvalidJSON(t *testing.T) {
	// Create a temp file with invalid JSON
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	err := runValidate(validateCmd, []string{invalidFile})
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("Expected 'invalid JSON' error, got: %v", err)
	}
}

func TestRunValidate_EmptyPlan(t *testing.T) {
	// Create a temp file with valid JSON but no tasks
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")
	content := `{"summary": "Empty plan", "tasks": []}`
	if err := os.WriteFile(emptyFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	err := runValidate(validateCmd, []string{emptyFile})
	if err == nil {
		t.Error("Expected error for empty plan")
	}
	// Empty plan should fail parsing due to no tasks
	if !strings.Contains(err.Error(), "no tasks") {
		t.Errorf("Expected 'no tasks' error, got: %v", err)
	}
}

func TestRunValidate_ValidPlan(t *testing.T) {
	// Create a temp file with a valid plan
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "valid.json")
	content := `{
		"summary": "Test plan",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"files": ["file1.go"],
				"depends_on": [],
				"priority": 0,
				"est_complexity": "low"
			},
			{
				"id": "task-2",
				"title": "Task 2",
				"description": "Second task",
				"files": ["file2.go"],
				"depends_on": ["task-1"],
				"priority": 0,
				"est_complexity": "medium"
			}
		],
		"insights": ["insight 1"],
		"constraints": ["constraint 1"]
	}`
	if err := os.WriteFile(validFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{validFile})
		if err != nil {
			t.Errorf("Expected no error for valid plan, got: %v", err)
		}
	})

	// Check output contains expected content
	expectedContent := []string{
		"Validating:",
		"Tasks: 2",
		"Execution Groups: 2",
		"Status: VALID",
	}
	for _, expected := range expectedContent {
		if !strings.Contains(output, expected) {
			t.Errorf("Output missing expected content %q, got:\n%s", expected, output)
		}
	}
}

func TestRunValidate_PlanWithCycle(t *testing.T) {
	// Create a temp file with a cyclic dependency
	tmpDir := t.TempDir()
	cycleFile := filepath.Join(tmpDir, "cycle.json")
	content := `{
		"summary": "Cyclic plan",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"depends_on": ["task-2"],
				"est_complexity": "low"
			},
			{
				"id": "task-2",
				"title": "Task 2",
				"description": "Second task",
				"depends_on": ["task-1"],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(cycleFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{cycleFile})
		if err == nil {
			t.Error("Expected error for cyclic plan")
		}
	})

	// Check that cycle is reported
	if !strings.Contains(output, "INVALID") {
		t.Errorf("Output should indicate INVALID status, got:\n%s", output)
	}
	if !strings.Contains(output, "cycle") {
		t.Errorf("Output should mention cycle, got:\n%s", output)
	}
}

func TestRunValidate_JSONOutput_ValidPlan(t *testing.T) {
	// Create a temp file with a valid plan
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "valid.json")
	content := `{
		"summary": "Test plan",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"depends_on": [],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(validFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = true
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{validFile})
		if err != nil {
			t.Errorf("Expected no error for valid plan, got: %v", err)
		}
	})

	// Parse and verify JSON output
	var result ValidationOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput:\n%s", err, output)
	}

	if !result.Valid {
		t.Error("Expected Valid=true for valid plan")
	}
	if result.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", result.ErrorCount)
	}
}

func TestRunValidate_JSONOutput_InvalidPlan(t *testing.T) {
	// Create a temp file with a cyclic plan
	tmpDir := t.TempDir()
	cycleFile := filepath.Join(tmpDir, "cycle.json")
	content := `{
		"summary": "Cyclic plan",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"depends_on": ["task-2"],
				"est_complexity": "low"
			},
			{
				"id": "task-2",
				"title": "Task 2",
				"description": "Second task",
				"depends_on": ["task-1"],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(cycleFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = true
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{cycleFile})
		// Expect silentError for invalid plan in JSON mode
		if err == nil {
			t.Error("Expected error for invalid plan in JSON mode")
		}
	})

	// Parse and verify JSON output
	var result ValidationOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput:\n%s", err, output)
	}

	if result.Valid {
		t.Error("Expected Valid=false for cyclic plan")
	}
	if result.ErrorCount == 0 {
		t.Error("Expected at least 1 error for cyclic plan")
	}
}

func TestRunValidate_JSONOutput_FileNotFound(t *testing.T) {
	originalJSON := validateJSON
	validateJSON = true
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{"nonexistent.json"})
		// Expect silentError for file not found in JSON mode
		if err == nil {
			t.Error("Expected error for nonexistent file in JSON mode")
		}
	})

	// Parse and verify JSON output
	var result ValidationOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput:\n%s", err, output)
	}

	if result.Valid {
		t.Error("Expected Valid=false for file not found")
	}
	if result.ParseError == "" {
		t.Error("Expected ParseError to be set for file not found")
	}
}

func TestSilentError(t *testing.T) {
	err := &silentError{}
	expected := "validation failed"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestRunValidate_PlanWithWarnings(t *testing.T) {
	// Create a plan with high complexity warnings
	tmpDir := t.TempDir()
	warningFile := filepath.Join(tmpDir, "warnings.json")
	content := `{
		"summary": "Plan with warnings",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "A high complexity task",
				"depends_on": [],
				"est_complexity": "high"
			}
		]
	}`
	if err := os.WriteFile(warningFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{warningFile})
		// Valid with warnings should not return error
		if err != nil {
			t.Errorf("Expected no error for plan with warnings, got: %v", err)
		}
	})

	// Should show VALID status but with warnings
	if !strings.Contains(output, "VALID") {
		t.Errorf("Output should show VALID, got:\n%s", output)
	}
	if !strings.Contains(output, "Warnings:") {
		t.Errorf("Output should show Warnings section, got:\n%s", output)
	}
}

func TestRunValidate_NestedPlanFormat(t *testing.T) {
	// Test that nested "plan" format is also supported
	tmpDir := t.TempDir()
	nestedFile := filepath.Join(tmpDir, "nested.json")
	content := `{
		"plan": {
			"summary": "Nested plan",
			"tasks": [
				{
					"id": "task-1",
					"title": "Task 1",
					"description": "First task",
					"depends_on": [],
					"est_complexity": "low"
				}
			]
		}
	}`
	if err := os.WriteFile(nestedFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{nestedFile})
		if err != nil {
			t.Errorf("Expected no error for nested plan format, got: %v", err)
		}
	})

	if !strings.Contains(output, "VALID") {
		t.Errorf("Output should show VALID for nested format, got:\n%s", output)
	}
}

func TestRunValidate_AlternativeFieldNames(t *testing.T) {
	// Test that alternative field names are supported
	tmpDir := t.TempDir()
	altFile := filepath.Join(tmpDir, "alternative.json")
	content := `{
		"summary": "Plan with alternative fields",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"depends": ["task-2"],
				"complexity": "low"
			},
			{
				"id": "task-2",
				"title": "Task 2",
				"description": "Second task",
				"depends": [],
				"complexity": "medium"
			}
		]
	}`
	if err := os.WriteFile(altFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	// The "depends" field is an alias for "depends_on", which is supported by ParsePlanFromFile.
	// The plan is valid: task-1 depends on task-2, and task-2 has no dependencies.
	err := runValidate(validateCmd, []string{altFile})
	if err != nil {
		t.Errorf("Expected no error for plan with alternative field names, got: %v", err)
	}
}

func TestPrintMessage(t *testing.T) {
	output := captureValidateOutput(func() {
		msg := ultraplan.ValidationMessage{
			Severity:   ultraplan.SeverityError,
			TaskID:     "task-1",
			Message:    "Test error message",
			Suggestion: "Fix the error",
		}
		printMessage(msg)
	})

	if !strings.Contains(output, "[task-1]") {
		t.Errorf("Output should contain task ID prefix, got:\n%s", output)
	}
	if !strings.Contains(output, "Test error message") {
		t.Errorf("Output should contain message, got:\n%s", output)
	}
	if !strings.Contains(output, "Suggestion: Fix the error") {
		t.Errorf("Output should contain suggestion, got:\n%s", output)
	}
}

func TestPrintMessage_NoTaskID(t *testing.T) {
	output := captureValidateOutput(func() {
		msg := ultraplan.ValidationMessage{
			Severity: ultraplan.SeverityError,
			Message:  "Global error message",
		}
		printMessage(msg)
	})

	if strings.Contains(output, "[") {
		t.Errorf("Output should not contain brackets when no task ID, got:\n%s", output)
	}
	if !strings.Contains(output, "- Global error message") {
		t.Errorf("Output should contain message with dash prefix, got:\n%s", output)
	}
}

func TestRunValidate_DefaultFilePath(t *testing.T) {
	// Test that when no arguments are provided, the default file path is used
	// The default path is orchestrator.PlanFileName (.claudio-plan.json)

	// Save and restore current directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	// Create a temp directory with a valid plan at the default filename
	tmpDir := t.TempDir()
	defaultPlanFile := filepath.Join(tmpDir, orchestrator.PlanFileName)
	content := `{
		"summary": "Test plan with default path",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"depends_on": [],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(defaultPlanFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Change to the temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	// Run validate with no arguments (empty args slice)
	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{})
		if err != nil {
			t.Errorf("Expected no error when default file exists, got: %v", err)
		}
	})

	// Verify the output shows the default filename was used
	if !strings.Contains(output, orchestrator.PlanFileName) {
		t.Errorf("Output should reference default filename %q, got:\n%s", orchestrator.PlanFileName, output)
	}
	if !strings.Contains(output, "VALID") {
		t.Errorf("Output should show VALID status, got:\n%s", output)
	}
}

func TestRunValidate_DefaultFilePath_FileNotFound(t *testing.T) {
	// Test that when no arguments are provided and default file doesn't exist,
	// the error mentions the default filename

	// Save and restore current directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Fatalf("Failed to restore working directory: %v", err)
		}
	}()

	// Create an empty temp directory (no plan file)
	tmpDir := t.TempDir()

	// Change to the temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	// Run validate with no arguments (empty args slice)
	err = runValidate(validateCmd, []string{})
	if err == nil {
		t.Error("Expected error when default file doesn't exist")
	}

	// Verify the error mentions the default filename
	if !strings.Contains(err.Error(), orchestrator.PlanFileName) {
		t.Errorf("Error should mention default filename %q, got: %v", orchestrator.PlanFileName, err)
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("Error should mention 'file not found', got: %v", err)
	}
}

func TestRunValidate_JSONOutput_InvalidJSON(t *testing.T) {
	// Test that invalid JSON produces valid JSON output with --json flag
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not valid json {{{"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = true
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{invalidFile})
		if err == nil {
			t.Error("Expected error for invalid JSON in JSON mode")
		}
	})

	// Verify output is valid JSON
	var result ValidationOutput
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput:\n%s", err, output)
	}

	if result.Valid {
		t.Error("Expected Valid=false for invalid JSON")
	}
	if !strings.Contains(result.ParseError, "invalid JSON") {
		t.Errorf("Expected ParseError to contain 'invalid JSON', got %q", result.ParseError)
	}
}

func TestRunValidate_FileConflictWarning(t *testing.T) {
	// Test that file conflicts between parallel tasks produce warnings
	tmpDir := t.TempDir()
	conflictFile := filepath.Join(tmpDir, "conflict.json")
	content := `{
		"summary": "Plan with file conflicts",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "First task",
				"files": ["shared.go", "other.go"],
				"depends_on": [],
				"est_complexity": "low"
			},
			{
				"id": "task-2",
				"title": "Task 2",
				"description": "Second task (parallel with task-1)",
				"files": ["shared.go"],
				"depends_on": [],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(conflictFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{conflictFile})
		// Plan with warnings (but no errors) should not return error
		if err != nil {
			t.Errorf("Expected no error for plan with file conflict warnings, got: %v", err)
		}
	})

	// Should be VALID (file conflicts are warnings, not errors)
	if !strings.Contains(output, "VALID") {
		t.Errorf("Output should show VALID status, got:\n%s", output)
	}
	// Should have warnings about the shared file
	if !strings.Contains(output, "shared.go") || !strings.Contains(output, "multiple") {
		t.Errorf("Output should warn about file conflict for shared.go, got:\n%s", output)
	}
	if !strings.Contains(output, "Warnings:") {
		t.Errorf("Output should show Warnings section, got:\n%s", output)
	}
}

func TestRunValidate_SelfDependency(t *testing.T) {
	// Test that self-dependency produces an error
	tmpDir := t.TempDir()
	selfDepFile := filepath.Join(tmpDir, "selfdep.json")
	content := `{
		"summary": "Plan with self-dependency",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "Self-dependent task",
				"depends_on": ["task-1"],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(selfDepFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{selfDepFile})
		if err == nil {
			t.Error("Expected error for self-dependent task")
		}
	})

	if !strings.Contains(output, "INVALID") {
		t.Errorf("Output should show INVALID status, got:\n%s", output)
	}
	if !strings.Contains(output, "depends on itself") {
		t.Errorf("Output should mention self-dependency, got:\n%s", output)
	}
}

func TestRunValidate_UnknownDependency(t *testing.T) {
	// Test that referencing a non-existent task produces an error
	tmpDir := t.TempDir()
	unknownDepFile := filepath.Join(tmpDir, "unknowndep.json")
	content := `{
		"summary": "Plan with unknown dependency",
		"tasks": [
			{
				"id": "task-1",
				"title": "Task 1",
				"description": "Task with unknown dependency",
				"depends_on": ["nonexistent-task"],
				"est_complexity": "low"
			}
		]
	}`
	if err := os.WriteFile(unknownDepFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	originalJSON := validateJSON
	validateJSON = false
	defer func() { validateJSON = originalJSON }()

	output := captureValidateOutput(func() {
		err := runValidate(validateCmd, []string{unknownDepFile})
		if err == nil {
			t.Error("Expected error for unknown dependency")
		}
	})

	if !strings.Contains(output, "INVALID") {
		t.Errorf("Output should show INVALID status, got:\n%s", output)
	}
	if !strings.Contains(output, "unknown task") || !strings.Contains(output, "nonexistent-task") {
		t.Errorf("Output should mention unknown dependency, got:\n%s", output)
	}
}

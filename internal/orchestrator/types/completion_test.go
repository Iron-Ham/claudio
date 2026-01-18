package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFlexibleString_UnmarshalJSON_String(t *testing.T) {
	input := `"hello world"`

	var fs FlexibleString
	err := json.Unmarshal([]byte(input), &fs)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if fs != "hello world" {
		t.Errorf("FlexibleString = %q, want %q", fs, "hello world")
	}
}

func TestFlexibleString_UnmarshalJSON_StringArray(t *testing.T) {
	input := `["line 1", "line 2", "line 3"]`

	var fs FlexibleString
	err := json.Unmarshal([]byte(input), &fs)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	expected := "line 1\nline 2\nline 3"
	if fs != FlexibleString(expected) {
		t.Errorf("FlexibleString = %q, want %q", fs, expected)
	}
}

func TestFlexibleString_UnmarshalJSON_EmptyString(t *testing.T) {
	input := `""`

	var fs FlexibleString
	err := json.Unmarshal([]byte(input), &fs)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if fs != "" {
		t.Errorf("FlexibleString = %q, want empty string", fs)
	}
}

func TestFlexibleString_UnmarshalJSON_EmptyArray(t *testing.T) {
	input := `[]`

	var fs FlexibleString
	err := json.Unmarshal([]byte(input), &fs)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if fs != "" {
		t.Errorf("FlexibleString = %q, want empty string", fs)
	}
}

func TestFlexibleString_UnmarshalJSON_InvalidJSON(t *testing.T) {
	// When both string and array parsing fail, should return empty
	input := `123` // neither string nor array

	var fs FlexibleString
	err := json.Unmarshal([]byte(input), &fs)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	// Should be empty when neither format works
	if fs != "" {
		t.Errorf("FlexibleString = %q, want empty string for invalid input", fs)
	}
}

func TestFlexibleString_UnmarshalJSON_Null(t *testing.T) {
	input := `null`

	var fs FlexibleString
	err := json.Unmarshal([]byte(input), &fs)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	// null should become empty string
	if fs != "" {
		t.Errorf("FlexibleString = %q, want empty string for null", fs)
	}
}

func TestFlexibleString_String(t *testing.T) {
	fs := FlexibleString("test value")

	if fs.String() != "test value" {
		t.Errorf("String() = %q, want %q", fs.String(), "test value")
	}
}

func TestTaskCompletionFile_JSONRoundTrip(t *testing.T) {
	original := TaskCompletionFile{
		TaskID:        "task-123",
		Status:        "complete",
		Summary:       "Implemented feature X",
		FilesModified: []string{"a.go", "b.go"},
		Notes:         "Some notes",
		Issues:        []string{"issue-1"},
		Suggestions:   []string{"suggestion-1"},
		Dependencies:  []string{"dep-1"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var parsed TaskCompletionFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if parsed.TaskID != original.TaskID {
		t.Errorf("TaskID = %q, want %q", parsed.TaskID, original.TaskID)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, original.Status)
	}
	if parsed.Summary != original.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, original.Summary)
	}
	if string(parsed.Notes) != string(original.Notes) {
		t.Errorf("Notes = %q, want %q", parsed.Notes, original.Notes)
	}
	if len(parsed.FilesModified) != len(original.FilesModified) {
		t.Errorf("len(FilesModified) = %d, want %d", len(parsed.FilesModified), len(original.FilesModified))
	}
}

func TestTaskCompletionFile_NotesAsArray(t *testing.T) {
	// Test that Notes can be parsed from a JSON array
	jsonInput := `{
		"task_id": "task-123",
		"status": "complete",
		"summary": "Test",
		"files_modified": [],
		"notes": ["note 1", "note 2", "note 3"]
	}`

	var parsed TaskCompletionFile
	if err := json.Unmarshal([]byte(jsonInput), &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	expectedNotes := "note 1\nnote 2\nnote 3"
	if string(parsed.Notes) != expectedNotes {
		t.Errorf("Notes = %q, want %q", parsed.Notes, expectedNotes)
	}
}

func TestAggregatedTaskContext_HasContent(t *testing.T) {
	tests := []struct {
		name    string
		context AggregatedTaskContext
		want    bool
	}{
		{
			name:    "empty context",
			context: AggregatedTaskContext{},
			want:    false,
		},
		{
			name: "only task summaries (no displayable content)",
			context: AggregatedTaskContext{
				TaskSummaries: map[string]string{"task-1": "summary"},
			},
			want: false,
		},
		{
			name: "has issues",
			context: AggregatedTaskContext{
				AllIssues: []string{"issue 1"},
			},
			want: true,
		},
		{
			name: "has suggestions",
			context: AggregatedTaskContext{
				AllSuggestions: []string{"suggestion 1"},
			},
			want: true,
		},
		{
			name: "has dependencies",
			context: AggregatedTaskContext{
				Dependencies: []string{"dep-1"},
			},
			want: true,
		},
		{
			name: "has notes",
			context: AggregatedTaskContext{
				Notes: []string{"note 1"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.context.HasContent()
			if got != tt.want {
				t.Errorf("HasContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAggregatedTaskContext_FormatForPR(t *testing.T) {
	context := AggregatedTaskContext{
		Notes:          []string{"Implementation note 1", "Implementation note 2"},
		AllIssues:      []string{"Issue 1", "Issue 2"},
		AllSuggestions: []string{"Suggestion 1"},
		Dependencies:   []string{"github.com/example/pkg"},
	}

	result := context.FormatForPR()

	// Check all sections are present
	if !strings.Contains(result, "## Implementation Notes") {
		t.Error("result should contain Implementation Notes section")
	}
	if !strings.Contains(result, "Implementation note 1") {
		t.Error("result should contain notes")
	}
	if !strings.Contains(result, "## Issues/Concerns Flagged") {
		t.Error("result should contain Issues section")
	}
	if !strings.Contains(result, "Issue 1") {
		t.Error("result should contain issues")
	}
	if !strings.Contains(result, "## Integration Suggestions") {
		t.Error("result should contain Suggestions section")
	}
	if !strings.Contains(result, "## New Dependencies") {
		t.Error("result should contain Dependencies section")
	}
	if !strings.Contains(result, "`github.com/example/pkg`") {
		t.Error("result should contain dependency with backticks")
	}
}

func TestAggregatedTaskContext_FormatForPR_Empty(t *testing.T) {
	context := AggregatedTaskContext{}

	result := context.FormatForPR()

	if result != "" {
		t.Errorf("FormatForPR() = %q, want empty string for empty context", result)
	}
}

func TestConsolidationTaskWorktreeInfo(t *testing.T) {
	info := ConsolidationTaskWorktreeInfo{
		TaskID:       "task-1",
		TaskTitle:    "Implement feature X",
		WorktreePath: "/tmp/worktree",
		Branch:       "feature-branch",
	}

	if info.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", info.TaskID, "task-1")
	}
	if info.TaskTitle != "Implement feature X" {
		t.Errorf("TaskTitle = %q, want %q", info.TaskTitle, "Implement feature X")
	}
	if info.WorktreePath != "/tmp/worktree" {
		t.Errorf("WorktreePath = %q, want %q", info.WorktreePath, "/tmp/worktree")
	}
	if info.Branch != "feature-branch" {
		t.Errorf("Branch = %q, want %q", info.Branch, "feature-branch")
	}
}

func TestVerificationResult_Fields(t *testing.T) {
	result := VerificationResult{
		ProjectType: "go",
		CommandsRun: []VerificationStep{
			{Name: "build", Command: "go build ./...", Success: true},
			{Name: "test", Command: "go test ./...", Success: false, Output: "test failure"},
		},
		OverallSuccess: false,
		Summary:        "Build passed, tests failed",
	}

	if result.ProjectType != "go" {
		t.Errorf("ProjectType = %q, want %q", result.ProjectType, "go")
	}
	if len(result.CommandsRun) != 2 {
		t.Errorf("len(CommandsRun) = %d, want 2", len(result.CommandsRun))
	}
	if result.OverallSuccess {
		t.Error("OverallSuccess should be false")
	}
}

func TestGroupConsolidationCompletionFile_Methods(t *testing.T) {
	file := GroupConsolidationCompletionFile{
		GroupIndex:        1,
		Status:            "complete",
		BranchName:        "consolidated-branch",
		TasksConsolidated: []string{"task-1", "task-2"},
		Verification: VerificationResult{
			OverallSuccess: true,
		},
		Notes:              "Consolidation notes",
		IssuesForNextGroup: []string{"Watch out for X"},
	}

	// Test GetNotes
	if file.GetNotes() != "Consolidation notes" {
		t.Errorf("GetNotes() = %q, want %q", file.GetNotes(), "Consolidation notes")
	}

	// Test GetIssuesForNextGroup
	issues := file.GetIssuesForNextGroup()
	if len(issues) != 1 || issues[0] != "Watch out for X" {
		t.Errorf("GetIssuesForNextGroup() = %v, want [Watch out for X]", issues)
	}

	// Test IsVerificationSuccess
	if !file.IsVerificationSuccess() {
		t.Error("IsVerificationSuccess() should return true")
	}
}

func TestGroupConsolidationCompletionFile_IsVerificationSuccess_False(t *testing.T) {
	file := GroupConsolidationCompletionFile{
		Verification: VerificationResult{
			OverallSuccess: false,
		},
	}

	if file.IsVerificationSuccess() {
		t.Error("IsVerificationSuccess() should return false")
	}
}

func TestConstants(t *testing.T) {
	if TaskCompletionFileName != ".claudio-task-complete.json" {
		t.Errorf("TaskCompletionFileName = %q, want %q",
			TaskCompletionFileName, ".claudio-task-complete.json")
	}

	if GroupConsolidationCompletionFileName != ".claudio-group-consolidation-complete.json" {
		t.Errorf("GroupConsolidationCompletionFileName = %q, want %q",
			GroupConsolidationCompletionFileName, ".claudio-group-consolidation-complete.json")
	}
}

func TestConflictResolution_JSONRoundTrip(t *testing.T) {
	original := ConflictResolution{
		File:       "main.go",
		Resolution: "Kept both versions and merged manually",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var parsed ConflictResolution
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if parsed.File != original.File {
		t.Errorf("File = %q, want %q", parsed.File, original.File)
	}
	if parsed.Resolution != original.Resolution {
		t.Errorf("Resolution = %q, want %q", parsed.Resolution, original.Resolution)
	}
}

func TestVerificationStep_Fields(t *testing.T) {
	step := VerificationStep{
		Name:    "lint",
		Command: "golangci-lint run",
		Success: true,
		Output:  "",
	}

	if step.Name != "lint" {
		t.Errorf("Name = %q, want %q", step.Name, "lint")
	}
	if step.Command != "golangci-lint run" {
		t.Errorf("Command = %q, want %q", step.Command, "golangci-lint run")
	}
	if !step.Success {
		t.Error("Success should be true")
	}
}

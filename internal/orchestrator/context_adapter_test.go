package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/context"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

func TestSessionInstanceFinderFindInstanceByTaskID(t *testing.T) {
	baseSession := &Session{
		Instances: []*Instance{
			{ID: "inst-1", WorktreePath: "/wt1", Branch: "feature-1", Task: "task-1: Do something"},
			{ID: "inst-2", WorktreePath: "/wt2", Branch: "feature-2", Task: "task-2: Do other"},
		},
	}

	ultraPlan := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", Title: "Do something"},
				{ID: "task-2", Title: "Do other"},
			},
		},
	}

	finder := newSessionInstanceFinder(baseSession, ultraPlan)

	tests := []struct {
		name    string
		taskID  string
		wantID  string
		wantNil bool
	}{
		{
			name:   "finds by task ID in task string",
			taskID: "task-1",
			wantID: "inst-1",
		},
		{
			name:   "finds second task",
			taskID: "task-2",
			wantID: "inst-2",
		},
		{
			name:    "returns nil for missing task",
			taskID:  "task-missing",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := finder.FindInstanceByTaskID(tt.taskID)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %+v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("expected non-nil result")
			}

			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
		})
	}
}

func TestSessionInstanceFinderFindByBranchSlug(t *testing.T) {
	baseSession := &Session{
		Instances: []*Instance{
			{ID: "inst-1", WorktreePath: "/wt1", Branch: "feature/add-user-auth", Task: "Some unrelated task"},
		},
	}

	ultraPlan := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", Title: "Add User Auth"},
			},
		},
	}

	finder := newSessionInstanceFinder(baseSession, ultraPlan)

	// The task string doesn't contain "task-1", but the branch contains a slugified version
	// of the task title "Add User Auth" -> "add-user-auth"
	result := finder.FindInstanceByTaskID("task-1")

	if result == nil {
		t.Fatal("expected to find instance by branch slug")
	}

	if result.ID != "inst-1" {
		t.Errorf("ID = %q, want inst-1", result.ID)
	}
}

func TestSessionInstanceFinderGetTaskInfo(t *testing.T) {
	ultraPlan := &UltraPlanSession{
		Plan: &PlanSpec{
			Tasks: []PlannedTask{
				{ID: "task-1", Title: "Task One", Description: "Do task one", DependsOn: []string{"task-0"}},
			},
		},
	}

	finder := newSessionInstanceFinder(nil, ultraPlan)

	// Test existing task
	info := finder.GetTaskInfo("task-1")
	if info == nil {
		t.Fatal("expected non-nil task info")
	}
	if info.ID != "task-1" {
		t.Errorf("ID = %q, want task-1", info.ID)
	}
	if info.Title != "Task One" {
		t.Errorf("Title = %q, want Task One", info.Title)
	}
	if info.Description != "Do task one" {
		t.Errorf("Description = %q, want Do task one", info.Description)
	}
	if len(info.DependsOn) != 1 || info.DependsOn[0] != "task-0" {
		t.Errorf("DependsOn = %v, want [task-0]", info.DependsOn)
	}

	// Test missing task
	info = finder.GetTaskInfo("task-missing")
	if info != nil {
		t.Errorf("expected nil for missing task, got %+v", info)
	}

	// Test nil ultra plan
	finder = newSessionInstanceFinder(nil, nil)
	info = finder.GetTaskInfo("task-1")
	if info != nil {
		t.Errorf("expected nil with nil ultra plan, got %+v", info)
	}
}

func TestFileCompletionReaderReadTaskCompletion(t *testing.T) {
	// Create a temp directory for test files
	tempDir, err := os.MkdirTemp("", "context_adapter_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create a valid task completion file
	completion := types.TaskCompletionFile{
		TaskID:        "task-1",
		Status:        "complete",
		Summary:       "Task completed successfully",
		FilesModified: []string{"file1.go", "file2.go"},
		Notes:         "Some notes",
		Issues:        []string{"Issue 1"},
		Suggestions:   []string{"Suggestion 1"},
		Dependencies:  []string{"dep1"},
	}

	completionPath := filepath.Join(tempDir, types.TaskCompletionFileName)
	data, err := json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	reader := newFileCompletionReader()

	// Test reading valid file
	result := reader.ReadTaskCompletion(tempDir)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", result.TaskID)
	}
	if result.Summary != "Task completed successfully" {
		t.Errorf("Summary = %q, want 'Task completed successfully'", result.Summary)
	}
	if result.Notes != "Some notes" {
		t.Errorf("Notes = %q, want 'Some notes'", result.Notes)
	}

	// Test reading non-existent file
	result = reader.ReadTaskCompletion("/nonexistent/path")
	if result != nil {
		t.Errorf("expected nil for non-existent path, got %+v", result)
	}
}

func TestFileCompletionReaderReadSynthesisCompletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "context_adapter_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	completion := SynthesisCompletionFile{
		Status:        "complete",
		RevisionRound: 1,
		IssuesFound: []RevisionIssue{
			{TaskID: "task-1", Description: "Issue 1", Severity: "major", Suggestion: "Fix it"},
		},
		TasksAffected:    []string{"task-1"},
		IntegrationNotes: "All good",
		Recommendations:  []string{"Merge in order"},
	}

	completionPath := filepath.Join(tempDir, SynthesisCompletionFileName)
	data, err := json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	reader := newFileCompletionReader()

	result := reader.ReadSynthesisCompletion(tempDir)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Status != "complete" {
		t.Errorf("Status = %q, want complete", result.Status)
	}
	if result.RevisionRound != 1 {
		t.Errorf("RevisionRound = %d, want 1", result.RevisionRound)
	}
	if len(result.IssuesFound) != 1 {
		t.Errorf("IssuesFound count = %d, want 1", len(result.IssuesFound))
	}
	if result.IssuesFound[0].TaskID != "task-1" {
		t.Errorf("IssuesFound[0].TaskID = %q, want task-1", result.IssuesFound[0].TaskID)
	}
	if result.IntegrationNotes != "All good" {
		t.Errorf("IntegrationNotes = %q, want 'All good'", result.IntegrationNotes)
	}
}

func TestFileCompletionReaderReadGroupConsolidationCompletion(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "context_adapter_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	completion := types.GroupConsolidationCompletionFile{
		GroupIndex:         0,
		Status:             "complete",
		BranchName:         "group-1-branch",
		TasksConsolidated:  []string{"task-1", "task-2"},
		Notes:              "Group notes",
		IssuesForNextGroup: []string{"Watch for X"},
		Verification: types.VerificationResult{
			OverallSuccess: true,
		},
	}

	completionPath := filepath.Join(tempDir, types.GroupConsolidationCompletionFileName)
	data, err := json.Marshal(completion)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	reader := newFileCompletionReader()

	result := reader.ReadGroupConsolidationCompletion(tempDir)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.GroupIndex != 0 {
		t.Errorf("GroupIndex = %d, want 0", result.GroupIndex)
	}
	if result.BranchName != "group-1-branch" {
		t.Errorf("BranchName = %q, want group-1-branch", result.BranchName)
	}
	if !result.VerificationOK {
		t.Error("VerificationOK = false, want true")
	}
	if len(result.IssuesForNextGroup) != 1 {
		t.Errorf("IssuesForNextGroup count = %d, want 1", len(result.IssuesForNextGroup))
	}
}

func TestNewContextGatherer(t *testing.T) {
	baseSession := &Session{}
	ultraPlan := &UltraPlanSession{}

	gatherer := NewContextGatherer(baseSession, ultraPlan)

	if gatherer == nil {
		t.Fatal("expected non-nil gatherer")
	}
}

func TestTaskWorktreeInfoConversions(t *testing.T) {
	// Test context to orchestrator conversion
	ctxInfo := []context.TaskWorktreeInfo{
		{TaskID: "task-1", TaskTitle: "Task One", WorktreePath: "/wt1", Branch: "b1"},
		{TaskID: "task-2", TaskTitle: "Task Two", WorktreePath: "/wt2", Branch: "b2"},
	}

	orchInfo := TaskWorktreeInfoFromContext(ctxInfo)

	if len(orchInfo) != 2 {
		t.Fatalf("expected 2 items, got %d", len(orchInfo))
	}
	if orchInfo[0].TaskID != "task-1" {
		t.Errorf("TaskID = %q, want task-1", orchInfo[0].TaskID)
	}

	// Test orchestrator to context conversion
	orchInfo2 := []TaskWorktreeInfo{
		{TaskID: "task-3", TaskTitle: "Task Three", WorktreePath: "/wt3", Branch: "b3"},
	}

	ctxInfo2 := TaskWorktreeInfoToContext(orchInfo2)

	if len(ctxInfo2) != 1 {
		t.Fatalf("expected 1 item, got %d", len(ctxInfo2))
	}
	if ctxInfo2[0].TaskID != "task-3" {
		t.Errorf("TaskID = %q, want task-3", ctxInfo2[0].TaskID)
	}
}

func TestAggregatedTaskContextFromContext(t *testing.T) {
	// Test nil conversion
	result := AggregatedTaskContextFromContext(nil)
	if result != nil {
		t.Errorf("nil input should return nil, got %+v", result)
	}

	// Test with data
	ctxAgg := &types.AggregatedTaskContext{
		TaskSummaries:  map[string]string{"task-1": "Summary 1"},
		AllIssues:      []string{"Issue 1"},
		AllSuggestions: []string{"Suggestion 1"},
		Dependencies:   []string{"dep1"},
		Notes:          []string{"Note 1"},
	}

	result = AggregatedTaskContextFromContext(ctxAgg)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.TaskSummaries["task-1"] != "Summary 1" {
		t.Errorf("TaskSummaries[task-1] = %q, want 'Summary 1'", result.TaskSummaries["task-1"])
	}
	if len(result.AllIssues) != 1 || result.AllIssues[0] != "Issue 1" {
		t.Errorf("AllIssues = %v, want [Issue 1]", result.AllIssues)
	}
	if len(result.Dependencies) != 1 || result.Dependencies[0] != "dep1" {
		t.Errorf("Dependencies = %v, want [dep1]", result.Dependencies)
	}
}

func TestSessionInstanceFinderNilSessions(t *testing.T) {
	// Test with nil base session
	finder := newSessionInstanceFinder(nil, nil)

	result := finder.FindInstanceByTaskID("task-1")
	if result != nil {
		t.Errorf("expected nil with nil base session, got %+v", result)
	}

	info := finder.GetTaskInfo("task-1")
	if info != nil {
		t.Errorf("expected nil with nil ultra plan, got %+v", info)
	}
}

package context

import (
	"strings"
	"testing"
)

// mockInstanceFinder is a test double for InstanceFinder
type mockInstanceFinder struct {
	instances map[string]*InstanceInfo
	tasks     map[string]*TaskInfo
}

func newMockInstanceFinder() *mockInstanceFinder {
	return &mockInstanceFinder{
		instances: make(map[string]*InstanceInfo),
		tasks:     make(map[string]*TaskInfo),
	}
}

func (m *mockInstanceFinder) FindInstanceByTaskID(taskID string) *InstanceInfo {
	return m.instances[taskID]
}

func (m *mockInstanceFinder) GetTaskInfo(taskID string) *TaskInfo {
	return m.tasks[taskID]
}

func (m *mockInstanceFinder) addInstance(taskID string, info *InstanceInfo) {
	m.instances[taskID] = info
}

func (m *mockInstanceFinder) addTask(taskID string, info *TaskInfo) {
	m.tasks[taskID] = info
}

// mockCompletionFileReader is a test double for CompletionFileReader
type mockCompletionFileReader struct {
	taskCompletions  map[string]*TaskCompletionData
	synthCompletions map[string]*SynthesisCompletionData
	groupCompletions map[string]*GroupConsolidationData
}

func newMockCompletionFileReader() *mockCompletionFileReader {
	return &mockCompletionFileReader{
		taskCompletions:  make(map[string]*TaskCompletionData),
		synthCompletions: make(map[string]*SynthesisCompletionData),
		groupCompletions: make(map[string]*GroupConsolidationData),
	}
}

func (m *mockCompletionFileReader) ReadTaskCompletion(worktreePath string) *TaskCompletionData {
	return m.taskCompletions[worktreePath]
}

func (m *mockCompletionFileReader) ReadSynthesisCompletion(worktreePath string) *SynthesisCompletionData {
	return m.synthCompletions[worktreePath]
}

func (m *mockCompletionFileReader) ReadGroupConsolidationCompletion(worktreePath string) *GroupConsolidationData {
	return m.groupCompletions[worktreePath]
}

func (m *mockCompletionFileReader) addTaskCompletion(worktreePath string, data *TaskCompletionData) {
	m.taskCompletions[worktreePath] = data
}

func (m *mockCompletionFileReader) addSynthesisCompletion(worktreePath string, data *SynthesisCompletionData) {
	m.synthCompletions[worktreePath] = data
}

func (m *mockCompletionFileReader) addGroupCompletion(worktreePath string, data *GroupConsolidationData) {
	m.groupCompletions[worktreePath] = data
}

func TestNewGatherer(t *testing.T) {
	finder := newMockInstanceFinder()
	reader := newMockCompletionFileReader()

	g := NewGatherer(finder, reader)

	if g == nil {
		t.Fatal("NewGatherer returned nil")
	}
	if g.finder != finder {
		t.Error("finder not set correctly")
	}
	if g.reader != reader {
		t.Error("reader not set correctly")
	}
}

func TestGatherTaskContext(t *testing.T) {
	tests := []struct {
		name         string
		taskID       string
		worktrees    []TaskWorktreeInfo
		setupFinder  func(*mockInstanceFinder)
		setupReader  func(*mockCompletionFileReader)
		wantTitle    string
		wantWorktree string
		wantBranch   string
		wantSummary  string
		wantNotes    string
	}{
		{
			name:   "basic task context from worktrees",
			taskID: "task-1",
			worktrees: []TaskWorktreeInfo{
				{TaskID: "task-1", TaskTitle: "Task One", WorktreePath: "/path/to/wt", Branch: "feature-1"},
			},
			setupFinder: func(f *mockInstanceFinder) {
				f.addTask("task-1", &TaskInfo{
					ID:          "task-1",
					Title:       "Task One Override",
					Description: "Do something",
				})
			},
			setupReader:  func(r *mockCompletionFileReader) {},
			wantTitle:    "Task One Override", // TaskInfo takes precedence
			wantWorktree: "/path/to/wt",
			wantBranch:   "feature-1",
		},
		{
			name:   "task context with completion data",
			taskID: "task-2",
			worktrees: []TaskWorktreeInfo{
				{TaskID: "task-2", TaskTitle: "Task Two", WorktreePath: "/path/to/wt2", Branch: "feature-2"},
			},
			setupFinder: func(f *mockInstanceFinder) {
				f.addTask("task-2", &TaskInfo{ID: "task-2", Title: "Task Two"})
			},
			setupReader: func(r *mockCompletionFileReader) {
				r.addTaskCompletion("/path/to/wt2", &TaskCompletionData{
					TaskID:        "task-2",
					Status:        "complete",
					Summary:       "Completed task successfully",
					FilesModified: []string{"file1.go", "file2.go"},
					Notes:         "Some implementation notes",
					Issues:        []string{"Issue 1"},
					Suggestions:   []string{"Suggestion 1"},
					Dependencies:  []string{"github.com/foo/bar"},
				})
			},
			wantTitle:    "Task Two",
			wantWorktree: "/path/to/wt2",
			wantBranch:   "feature-2",
			wantSummary:  "Completed task successfully",
			wantNotes:    "Some implementation notes",
		},
		{
			name:   "task not in worktrees uses finder info",
			taskID: "task-3",
			worktrees: []TaskWorktreeInfo{
				{TaskID: "task-other", TaskTitle: "Other", WorktreePath: "/other", Branch: "other"},
			},
			setupFinder: func(f *mockInstanceFinder) {
				f.addTask("task-3", &TaskInfo{ID: "task-3", Title: "Task Three", Description: "Third task"})
			},
			setupReader:  func(r *mockCompletionFileReader) {},
			wantTitle:    "Task Three",
			wantWorktree: "",
			wantBranch:   "",
		},
		{
			name:   "task with no finder info uses worktree title",
			taskID: "task-4",
			worktrees: []TaskWorktreeInfo{
				{TaskID: "task-4", TaskTitle: "Worktree Title", WorktreePath: "/wt", Branch: "br"},
			},
			setupFinder: func(f *mockInstanceFinder) {
				// No task info added
			},
			setupReader:  func(r *mockCompletionFileReader) {},
			wantTitle:    "Worktree Title",
			wantWorktree: "/wt",
			wantBranch:   "br",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := newMockInstanceFinder()
			reader := newMockCompletionFileReader()
			tt.setupFinder(finder)
			tt.setupReader(reader)

			g := NewGatherer(finder, reader)
			ctx := g.GatherTaskContext(tt.taskID, tt.worktrees)

			if ctx.TaskID != tt.taskID {
				t.Errorf("TaskID = %q, want %q", ctx.TaskID, tt.taskID)
			}
			if ctx.TaskTitle != tt.wantTitle {
				t.Errorf("TaskTitle = %q, want %q", ctx.TaskTitle, tt.wantTitle)
			}
			if ctx.WorktreePath != tt.wantWorktree {
				t.Errorf("WorktreePath = %q, want %q", ctx.WorktreePath, tt.wantWorktree)
			}
			if ctx.Branch != tt.wantBranch {
				t.Errorf("Branch = %q, want %q", ctx.Branch, tt.wantBranch)
			}
			if ctx.Summary != tt.wantSummary {
				t.Errorf("Summary = %q, want %q", ctx.Summary, tt.wantSummary)
			}
			if ctx.Notes != tt.wantNotes {
				t.Errorf("Notes = %q, want %q", ctx.Notes, tt.wantNotes)
			}
		})
	}
}

func TestGatherAggregatedTaskContext(t *testing.T) {
	tests := []struct {
		name           string
		taskIDs        []string
		setupFinder    func(*mockInstanceFinder)
		setupReader    func(*mockCompletionFileReader)
		wantSummaries  map[string]string
		wantIssueCount int
		wantSuggCount  int
		wantDepCount   int
		wantNoteCount  int
	}{
		{
			name:    "aggregates multiple tasks",
			taskIDs: []string{"task-1", "task-2"},
			setupFinder: func(f *mockInstanceFinder) {
				f.addInstance("task-1", &InstanceInfo{ID: "inst-1", WorktreePath: "/wt1"})
				f.addInstance("task-2", &InstanceInfo{ID: "inst-2", WorktreePath: "/wt2"})
			},
			setupReader: func(r *mockCompletionFileReader) {
				r.addTaskCompletion("/wt1", &TaskCompletionData{
					TaskID:       "task-1",
					Summary:      "Summary 1",
					Issues:       []string{"Issue 1"},
					Suggestions:  []string{"Suggestion 1"},
					Dependencies: []string{"dep1"},
					Notes:        "Note 1",
				})
				r.addTaskCompletion("/wt2", &TaskCompletionData{
					TaskID:       "task-2",
					Summary:      "Summary 2",
					Issues:       []string{"Issue 2a", "Issue 2b"},
					Suggestions:  []string{"Suggestion 2"},
					Dependencies: []string{"dep1", "dep2"}, // dep1 is duplicate
					Notes:        "Note 2",
				})
			},
			wantSummaries:  map[string]string{"task-1": "Summary 1", "task-2": "Summary 2"},
			wantIssueCount: 3,
			wantSuggCount:  2,
			wantDepCount:   2, // dep1 deduplicated
			wantNoteCount:  2,
		},
		{
			name:    "handles missing completion files",
			taskIDs: []string{"task-1", "task-2"},
			setupFinder: func(f *mockInstanceFinder) {
				f.addInstance("task-1", &InstanceInfo{ID: "inst-1", WorktreePath: "/wt1"})
				f.addInstance("task-2", &InstanceInfo{ID: "inst-2", WorktreePath: "/wt2"})
			},
			setupReader: func(r *mockCompletionFileReader) {
				// Only task-1 has completion
				r.addTaskCompletion("/wt1", &TaskCompletionData{
					TaskID:  "task-1",
					Summary: "Only summary",
				})
			},
			wantSummaries:  map[string]string{"task-1": "Only summary"},
			wantIssueCount: 0,
			wantSuggCount:  0,
			wantDepCount:   0,
			wantNoteCount:  0,
		},
		{
			name:    "handles missing instances",
			taskIDs: []string{"task-1", "task-missing"},
			setupFinder: func(f *mockInstanceFinder) {
				f.addInstance("task-1", &InstanceInfo{ID: "inst-1", WorktreePath: "/wt1"})
				// task-missing has no instance
			},
			setupReader: func(r *mockCompletionFileReader) {
				r.addTaskCompletion("/wt1", &TaskCompletionData{
					TaskID:  "task-1",
					Summary: "Task 1 summary",
				})
			},
			wantSummaries:  map[string]string{"task-1": "Task 1 summary"},
			wantIssueCount: 0,
			wantSuggCount:  0,
			wantDepCount:   0,
			wantNoteCount:  0,
		},
		{
			name:    "filters empty strings",
			taskIDs: []string{"task-1"},
			setupFinder: func(f *mockInstanceFinder) {
				f.addInstance("task-1", &InstanceInfo{ID: "inst-1", WorktreePath: "/wt1"})
			},
			setupReader: func(r *mockCompletionFileReader) {
				r.addTaskCompletion("/wt1", &TaskCompletionData{
					TaskID:       "task-1",
					Summary:      "Summary",
					Issues:       []string{"", "Real issue", ""},
					Suggestions:  []string{"Real suggestion", ""},
					Dependencies: []string{"", "real-dep"},
				})
			},
			wantSummaries:  map[string]string{"task-1": "Summary"},
			wantIssueCount: 1,
			wantSuggCount:  1,
			wantDepCount:   1,
			wantNoteCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := newMockInstanceFinder()
			reader := newMockCompletionFileReader()
			tt.setupFinder(finder)
			tt.setupReader(reader)

			g := NewGatherer(finder, reader)
			ctx := g.GatherAggregatedTaskContext(tt.taskIDs)

			// Check summaries
			for taskID, wantSummary := range tt.wantSummaries {
				if ctx.TaskSummaries[taskID] != wantSummary {
					t.Errorf("TaskSummaries[%s] = %q, want %q", taskID, ctx.TaskSummaries[taskID], wantSummary)
				}
			}

			if len(ctx.AllIssues) != tt.wantIssueCount {
				t.Errorf("AllIssues count = %d, want %d", len(ctx.AllIssues), tt.wantIssueCount)
			}
			if len(ctx.AllSuggestions) != tt.wantSuggCount {
				t.Errorf("AllSuggestions count = %d, want %d", len(ctx.AllSuggestions), tt.wantSuggCount)
			}
			if len(ctx.Dependencies) != tt.wantDepCount {
				t.Errorf("Dependencies count = %d, want %d", len(ctx.Dependencies), tt.wantDepCount)
			}
			if len(ctx.Notes) != tt.wantNoteCount {
				t.Errorf("Notes count = %d, want %d", len(ctx.Notes), tt.wantNoteCount)
			}
		})
	}
}

func TestGatherSynthesisContext(t *testing.T) {
	tests := []struct {
		name           string
		objective      string
		completedTasks []string
		commitCounts   map[string]int
		setupFinder    func(*mockInstanceFinder)
		setupReader    func(*mockCompletionFileReader)
		wantTaskCount  int
		wantWorktrees  int
	}{
		{
			name:           "gathers synthesis context",
			objective:      "Test objective",
			completedTasks: []string{"task-1", "task-2"},
			commitCounts:   map[string]int{"task-1": 3, "task-2": 2},
			setupFinder: func(f *mockInstanceFinder) {
				f.addTask("task-1", &TaskInfo{ID: "task-1", Title: "Task One"})
				f.addTask("task-2", &TaskInfo{ID: "task-2", Title: "Task Two"})
				f.addInstance("task-1", &InstanceInfo{ID: "i1", WorktreePath: "/wt1", Branch: "b1"})
				f.addInstance("task-2", &InstanceInfo{ID: "i2", WorktreePath: "/wt2", Branch: "b2"})
			},
			setupReader: func(r *mockCompletionFileReader) {
				r.addTaskCompletion("/wt1", &TaskCompletionData{Summary: "Task 1 done"})
				r.addTaskCompletion("/wt2", &TaskCompletionData{Summary: "Task 2 done"})
			},
			wantTaskCount: 2,
			wantWorktrees: 2,
		},
		{
			name:           "handles missing instances",
			objective:      "Partial objective",
			completedTasks: []string{"task-1", "task-missing"},
			commitCounts:   map[string]int{"task-1": 1},
			setupFinder: func(f *mockInstanceFinder) {
				f.addTask("task-1", &TaskInfo{ID: "task-1", Title: "Task One"})
				f.addInstance("task-1", &InstanceInfo{ID: "i1", WorktreePath: "/wt1", Branch: "b1"})
			},
			setupReader: func(r *mockCompletionFileReader) {
				r.addTaskCompletion("/wt1", &TaskCompletionData{Summary: "Done"})
			},
			wantTaskCount: 2,
			wantWorktrees: 1, // Only one has instance
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := newMockInstanceFinder()
			reader := newMockCompletionFileReader()
			tt.setupFinder(finder)
			tt.setupReader(reader)

			g := NewGatherer(finder, reader)
			ctx := g.GatherSynthesisContext(tt.objective, tt.completedTasks, tt.commitCounts)

			if ctx.Objective != tt.objective {
				t.Errorf("Objective = %q, want %q", ctx.Objective, tt.objective)
			}
			if ctx.TotalTasks != tt.wantTaskCount {
				t.Errorf("TotalTasks = %d, want %d", ctx.TotalTasks, tt.wantTaskCount)
			}
			if len(ctx.CompletedTasks) != tt.wantTaskCount {
				t.Errorf("CompletedTasks count = %d, want %d", len(ctx.CompletedTasks), tt.wantTaskCount)
			}
			if len(ctx.TaskWorktrees) != tt.wantWorktrees {
				t.Errorf("TaskWorktrees count = %d, want %d", len(ctx.TaskWorktrees), tt.wantWorktrees)
			}
		})
	}
}

func TestGatherConsolidationContext(t *testing.T) {
	tests := []struct {
		name                string
		objective           string
		synthWorktree       string
		groupBranches       []string
		groupWorktrees      []string
		setupReader         func(*mockCompletionFileReader)
		wantSynthStatus     string
		wantGroupContexts   int
		wantPreconsolidated bool
	}{
		{
			name:           "full consolidation context",
			objective:      "Build feature",
			synthWorktree:  "/synth",
			groupBranches:  []string{"group-1", "group-2"},
			groupWorktrees: []string{"/g1", "/g2"},
			setupReader: func(r *mockCompletionFileReader) {
				r.addSynthesisCompletion("/synth", &SynthesisCompletionData{
					Status:           "complete",
					IntegrationNotes: "All good",
					Recommendations:  []string{"Merge in order"},
				})
				r.addGroupCompletion("/g1", &GroupConsolidationData{
					GroupIndex:     0,
					BranchName:     "group-1",
					Notes:          "Group 1 notes",
					VerificationOK: true,
				})
				r.addGroupCompletion("/g2", &GroupConsolidationData{
					GroupIndex:         1,
					BranchName:         "group-2",
					Notes:              "Group 2 notes",
					IssuesForNextGroup: []string{"Watch for conflicts"},
					VerificationOK:     true,
				})
			},
			wantSynthStatus:     "complete",
			wantGroupContexts:   2,
			wantPreconsolidated: true,
		},
		{
			name:           "no preconsolidated branches",
			objective:      "Simple task",
			synthWorktree:  "/synth",
			groupBranches:  []string{},
			groupWorktrees: []string{},
			setupReader: func(r *mockCompletionFileReader) {
				r.addSynthesisCompletion("/synth", &SynthesisCompletionData{
					Status: "needs_revision",
				})
			},
			wantSynthStatus:     "needs_revision",
			wantGroupContexts:   0,
			wantPreconsolidated: false,
		},
		{
			name:           "no synthesis worktree",
			objective:      "No synth",
			synthWorktree:  "",
			groupBranches:  []string{"g1"},
			groupWorktrees: []string{"/g1"},
			setupReader: func(r *mockCompletionFileReader) {
				r.addGroupCompletion("/g1", &GroupConsolidationData{
					GroupIndex: 0,
					BranchName: "g1",
				})
			},
			wantSynthStatus:     "",
			wantGroupContexts:   1,
			wantPreconsolidated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			finder := newMockInstanceFinder()
			reader := newMockCompletionFileReader()
			tt.setupReader(reader)

			g := NewGatherer(finder, reader)
			ctx := g.GatherConsolidationContext(tt.objective, tt.synthWorktree, tt.groupBranches, tt.groupWorktrees)

			if ctx.Objective != tt.objective {
				t.Errorf("Objective = %q, want %q", ctx.Objective, tt.objective)
			}
			if ctx.SynthesisStatus != tt.wantSynthStatus {
				t.Errorf("SynthesisStatus = %q, want %q", ctx.SynthesisStatus, tt.wantSynthStatus)
			}
			if len(ctx.GroupContexts) != tt.wantGroupContexts {
				t.Errorf("GroupContexts count = %d, want %d", len(ctx.GroupContexts), tt.wantGroupContexts)
			}
			if ctx.HasPreconsolidated != tt.wantPreconsolidated {
				t.Errorf("HasPreconsolidated = %v, want %v", ctx.HasPreconsolidated, tt.wantPreconsolidated)
			}
		})
	}
}

func TestGetTaskWorktreeInfo(t *testing.T) {
	finder := newMockInstanceFinder()
	finder.addTask("task-1", &TaskInfo{ID: "task-1", Title: "Task One"})
	finder.addTask("task-2", &TaskInfo{ID: "task-2", Title: "Task Two"})
	finder.addInstance("task-1", &InstanceInfo{ID: "i1", WorktreePath: "/wt1", Branch: "b1"})
	finder.addInstance("task-2", &InstanceInfo{ID: "i2", WorktreePath: "/wt2", Branch: "b2"})

	reader := newMockCompletionFileReader()
	g := NewGatherer(finder, reader)

	result := g.GetTaskWorktreeInfo([]string{"task-1", "task-2", "task-missing"})

	if len(result) != 2 {
		t.Errorf("result count = %d, want 2", len(result))
	}

	if result[0].TaskID != "task-1" || result[0].TaskTitle != "Task One" {
		t.Errorf("first result = %+v, want task-1/Task One", result[0])
	}
	if result[1].TaskID != "task-2" || result[1].TaskTitle != "Task Two" {
		t.Errorf("second result = %+v, want task-2/Task Two", result[1])
	}
}

func TestFormatTaskListForPrompt(t *testing.T) {
	finder := newMockInstanceFinder()
	finder.addTask("task-1", &TaskInfo{ID: "task-1", Title: "Task One"})
	finder.addTask("task-2", &TaskInfo{ID: "task-2", Title: "Task Two"})

	reader := newMockCompletionFileReader()
	g := NewGatherer(finder, reader)

	commitCounts := map[string]int{"task-1": 3, "task-2": 0}
	result := g.FormatTaskListForPrompt([]string{"task-1", "task-2"}, commitCounts)

	if !strings.Contains(result, "[task-1] Task One (3 commits)") {
		t.Errorf("result missing task-1 with commits: %s", result)
	}
	if !strings.Contains(result, "[task-2] Task Two (NO COMMITS - verify this task)") {
		t.Errorf("result missing task-2 warning: %s", result)
	}
}

func TestFormatWorktreeInfoForPrompt(t *testing.T) {
	finder := newMockInstanceFinder()
	reader := newMockCompletionFileReader()
	g := NewGatherer(finder, reader)

	worktrees := []TaskWorktreeInfo{
		{TaskID: "task-1", TaskTitle: "Task One", WorktreePath: "/path/to/wt", Branch: "feature-1"},
	}
	taskContext := &AggregatedTaskContext{
		TaskSummaries: map[string]string{"task-1": "Completed successfully"},
	}

	result := g.FormatWorktreeInfoForPrompt(worktrees, taskContext)

	if !strings.Contains(result, "### task-1: Task One") {
		t.Errorf("result missing task header: %s", result)
	}
	if !strings.Contains(result, "Branch: `feature-1`") {
		t.Errorf("result missing branch: %s", result)
	}
	if !strings.Contains(result, "Worktree: `/path/to/wt`") {
		t.Errorf("result missing worktree: %s", result)
	}
	if !strings.Contains(result, "Summary: Completed successfully") {
		t.Errorf("result missing summary: %s", result)
	}
}

func TestFormatPreviousGroupContext(t *testing.T) {
	finder := newMockInstanceFinder()
	reader := newMockCompletionFileReader()
	g := NewGatherer(finder, reader)

	// Test with nil
	result := g.FormatPreviousGroupContext(nil)
	if result != "" {
		t.Errorf("nil context should return empty string, got: %s", result)
	}

	// Test with content
	groupCtx := &GroupContext{
		GroupIndex:    0,
		Branch:        "group-1",
		Notes:         "Important notes",
		IssuesForNext: []string{"Watch for X", "Check Y"},
	}

	result = g.FormatPreviousGroupContext(groupCtx)

	if !strings.Contains(result, "Context from Previous Group") {
		t.Errorf("result missing header: %s", result)
	}
	if !strings.Contains(result, "**Notes**: Important notes") {
		t.Errorf("result missing notes: %s", result)
	}
	if !strings.Contains(result, "Watch for X") {
		t.Errorf("result missing issue 1: %s", result)
	}
	if !strings.Contains(result, "Check Y") {
		t.Errorf("result missing issue 2: %s", result)
	}
}

func TestFormatSynthesisContextForPrompt(t *testing.T) {
	finder := newMockInstanceFinder()
	reader := newMockCompletionFileReader()
	g := NewGatherer(finder, reader)

	// Test with nil
	result := g.FormatSynthesisContextForPrompt(nil)
	if !strings.Contains(result, "No synthesis context available") {
		t.Errorf("nil should return default message, got: %s", result)
	}

	// Test with content
	synth := &SynthesisCompletionData{
		Status:           "complete",
		IntegrationNotes: "Integration went well",
		Recommendations:  []string{"Rec 1", "Rec 2"},
		IssuesFound: []SynthesisIssue{
			{Severity: "major", Description: "Found a problem"},
		},
	}

	result = g.FormatSynthesisContextForPrompt(synth)

	if !strings.Contains(result, "Status: complete") {
		t.Errorf("result missing status: %s", result)
	}
	if !strings.Contains(result, "Integration Notes: Integration went well") {
		t.Errorf("result missing integration notes: %s", result)
	}
	if !strings.Contains(result, "Rec 1") {
		t.Errorf("result missing recommendation: %s", result)
	}
	if !strings.Contains(result, "[major] Found a problem") {
		t.Errorf("result missing issue: %s", result)
	}
}

func TestAggregatedTaskContextHasContent(t *testing.T) {
	tests := []struct {
		name string
		ctx  *AggregatedTaskContext
		want bool
	}{
		{
			name: "empty context",
			ctx: &AggregatedTaskContext{
				TaskSummaries:  make(map[string]string),
				AllIssues:      []string{},
				AllSuggestions: []string{},
				Dependencies:   []string{},
				Notes:          []string{},
			},
			want: false,
		},
		{
			name: "has issues",
			ctx: &AggregatedTaskContext{
				AllIssues: []string{"Issue 1"},
			},
			want: true,
		},
		{
			name: "has suggestions",
			ctx: &AggregatedTaskContext{
				AllSuggestions: []string{"Suggestion 1"},
			},
			want: true,
		},
		{
			name: "has dependencies",
			ctx: &AggregatedTaskContext{
				Dependencies: []string{"dep1"},
			},
			want: true,
		},
		{
			name: "has notes",
			ctx: &AggregatedTaskContext{
				Notes: []string{"Note 1"},
			},
			want: true,
		},
		{
			name: "summaries only is not content",
			ctx: &AggregatedTaskContext{
				TaskSummaries: map[string]string{"task-1": "Summary"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.HasContent(); got != tt.want {
				t.Errorf("HasContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAggregatedTaskContextFormatForPR(t *testing.T) {
	ctx := &AggregatedTaskContext{
		TaskSummaries:  map[string]string{"task-1": "Summary"},
		AllIssues:      []string{"[task-1] Issue 1"},
		AllSuggestions: []string{"[task-1] Suggestion 1"},
		Dependencies:   []string{"github.com/foo/bar"},
		Notes:          []string{"**task-1**: Note 1"},
	}

	result := ctx.FormatForPR()

	if !strings.Contains(result, "## Implementation Notes") {
		t.Errorf("result missing notes section: %s", result)
	}
	if !strings.Contains(result, "## Issues/Concerns Flagged") {
		t.Errorf("result missing issues section: %s", result)
	}
	if !strings.Contains(result, "## Integration Suggestions") {
		t.Errorf("result missing suggestions section: %s", result)
	}
	if !strings.Contains(result, "## New Dependencies") {
		t.Errorf("result missing dependencies section: %s", result)
	}
	if !strings.Contains(result, "`github.com/foo/bar`") {
		t.Errorf("result missing dependency with backticks: %s", result)
	}
}

func TestAggregatedTaskContextFormatForPREmpty(t *testing.T) {
	ctx := &AggregatedTaskContext{
		TaskSummaries:  make(map[string]string),
		AllIssues:      []string{},
		AllSuggestions: []string{},
		Dependencies:   []string{},
		Notes:          []string{},
	}

	result := ctx.FormatForPR()

	if result != "" {
		t.Errorf("empty context should format to empty string, got: %s", result)
	}
}

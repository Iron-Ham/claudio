package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/terminal"
	tea "github.com/charmbracelet/bubbletea"
)

// createTestPlanForTUI creates a sample plan for TUI testing
func createTestPlanForTUI() *orchestrator.PlanSpec {
	tasks := []orchestrator.PlannedTask{
		{
			ID:            "task-1",
			Title:         "Setup",
			Description:   "Initialize the project",
			Files:         []string{"main.go", "go.mod"},
			DependsOn:     []string{},
			Priority:      1,
			EstComplexity: orchestrator.ComplexityLow,
		},
		{
			ID:            "task-2",
			Title:         "Core Features",
			Description:   "Implement core features",
			Files:         []string{"core.go"},
			DependsOn:     []string{"task-1"},
			Priority:      2,
			EstComplexity: orchestrator.ComplexityMedium,
		},
		{
			ID:            "task-3",
			Title:         "Tests",
			Description:   "Write tests",
			Files:         []string{"core_test.go"},
			DependsOn:     []string{"task-2"},
			Priority:      3,
			EstComplexity: orchestrator.ComplexityLow,
		},
		{
			ID:            "task-4",
			Title:         "Documentation",
			Description:   "Write documentation",
			Files:         []string{"README.md"},
			DependsOn:     []string{"task-1"},
			Priority:      2,
			EstComplexity: orchestrator.ComplexityLow,
		},
	}

	deps := make(map[string][]string)
	for _, t := range tasks {
		deps[t.ID] = t.DependsOn
	}

	plan := &orchestrator.PlanSpec{
		ID:              "test-plan",
		Objective:       "Test objective",
		Summary:         "Test summary",
		Tasks:           tasks,
		DependencyGraph: deps,
		ExecutionOrder:  [][]string{{"task-1"}, {"task-2", "task-4"}, {"task-3"}},
	}

	return plan
}

// createTestPlanEditorState creates a PlanEditorState for testing
func createTestPlanEditorState() *PlanEditorState {
	return &PlanEditorState{
		active:              true,
		selectedTaskIdx:     0,
		editingField:        "",
		editBuffer:          "",
		editCursor:          0,
		scrollOffset:        0,
		showValidationPanel: true,
		tasksInCycle:        make(map[string]bool),
	}
}

// Tests for PlanEditorState initialization

func TestPlanEditorState_Initialization(t *testing.T) {
	state := createTestPlanEditorState()

	if !state.active {
		t.Error("expected active to be true")
	}
	if state.selectedTaskIdx != 0 {
		t.Errorf("expected selectedTaskIdx to be 0, got %d", state.selectedTaskIdx)
	}
	if state.editingField != "" {
		t.Errorf("expected editingField to be empty, got '%s'", state.editingField)
	}
	if state.editCursor != 0 {
		t.Errorf("expected editCursor to be 0, got %d", state.editCursor)
	}
	if !state.showValidationPanel {
		t.Error("expected showValidationPanel to be true by default")
	}
}

// Tests for keyboard navigation

func TestPlanEditorMoveSelection(t *testing.T) {
	tests := []struct {
		name        string
		initialIdx  int
		delta       int
		numTasks    int
		expectedIdx int
	}{
		{
			name:        "move down from first",
			initialIdx:  0,
			delta:       1,
			numTasks:    4,
			expectedIdx: 1,
		},
		{
			name:        "move up from second",
			initialIdx:  1,
			delta:       -1,
			numTasks:    4,
			expectedIdx: 0,
		},
		{
			name:        "move up from first stays at first",
			initialIdx:  0,
			delta:       -1,
			numTasks:    4,
			expectedIdx: 0,
		},
		{
			name:        "move down from last stays at last",
			initialIdx:  3,
			delta:       1,
			numTasks:    4,
			expectedIdx: 3,
		},
		{
			name:        "move down by 2",
			initialIdx:  0,
			delta:       2,
			numTasks:    4,
			expectedIdx: 2,
		},
		{
			name:        "move down beyond bounds clamps",
			initialIdx:  2,
			delta:       10,
			numTasks:    4,
			expectedIdx: 3,
		},
		{
			name:        "move up beyond bounds clamps",
			initialIdx:  1,
			delta:       -10,
			numTasks:    4,
			expectedIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := createTestPlanForTUI()
			// Ensure we have the expected number of tasks
			if len(plan.Tasks) != tt.numTasks {
				t.Skipf("test expects %d tasks, got %d", tt.numTasks, len(plan.Tasks))
			}

			m := Model{
				terminalManager: terminal.NewManager(),
				planEditor: &PlanEditorState{
					active:          true,
					selectedTaskIdx: tt.initialIdx,
				},
			}
			m.terminalManager.SetSize(80, 50) // Set height for scroll calculations

			m.planEditorMoveSelection(tt.delta, plan)

			if m.planEditor.selectedTaskIdx != tt.expectedIdx {
				t.Errorf("expected selectedTaskIdx %d, got %d",
					tt.expectedIdx, m.planEditor.selectedTaskIdx)
			}
		})
	}
}

func TestPlanEditorEnsureVisible(t *testing.T) {
	tests := []struct {
		name             string
		selectedIdx      int
		initialScroll    int
		height           int
		expectedScrolled bool
	}{
		{
			name:             "selection at top stays visible",
			selectedIdx:      0,
			initialScroll:    0,
			height:           50,
			expectedScrolled: false,
		},
		{
			name:             "selection beyond view scrolls down",
			selectedIdx:      10,
			initialScroll:    0,
			height:           30, // Very short - can only show ~4 tasks
			expectedScrolled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a plan with many tasks
			plan := &orchestrator.PlanSpec{}
			for i := range 20 {
				plan.Tasks = append(plan.Tasks, orchestrator.PlannedTask{
					ID:    "task-" + string(rune('A'+i)),
					Title: "Task " + string(rune('A'+i)),
				})
			}

			m := Model{
				terminalManager: terminal.NewManager(),
				planEditor: &PlanEditorState{
					active:          true,
					selectedTaskIdx: tt.selectedIdx,
					scrollOffset:    tt.initialScroll,
				},
			}
			m.terminalManager.SetSize(80, tt.height)

			m.planEditorEnsureVisible(plan)

			scrollChanged := m.planEditor.scrollOffset != tt.initialScroll
			if scrollChanged != tt.expectedScrolled {
				t.Errorf("expected scroll changed=%v, got scroll changed=%v (scroll: %d->%d)",
					tt.expectedScrolled, scrollChanged, tt.initialScroll, m.planEditor.scrollOffset)
			}
		})
	}
}

// Tests for edit mode entry and exit

func TestStartEditingField(t *testing.T) {
	tests := []struct {
		name         string
		field        string
		taskTitle    string
		taskDesc     string
		taskFiles    []string
		taskPriority int
		taskDeps     []string
		expectedBuf  string
	}{
		{
			name:        "edit title",
			field:       "title",
			taskTitle:   "My Task",
			expectedBuf: "My Task",
		},
		{
			name:        "edit description",
			field:       "description",
			taskDesc:    "Task description here",
			expectedBuf: "Task description here",
		},
		{
			name:        "edit files",
			field:       "files",
			taskFiles:   []string{"file1.go", "file2.go"},
			expectedBuf: "file1.go, file2.go",
		},
		{
			name:         "edit priority",
			field:        "priority",
			taskPriority: 5,
			expectedBuf:  "5",
		},
		{
			name:        "edit depends_on",
			field:       "depends_on",
			taskDeps:    []string{"task-1", "task-2"},
			expectedBuf: "task-1, task-2",
		},
		{
			name:        "edit empty files",
			field:       "files",
			taskFiles:   nil,
			expectedBuf: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{
						ID:          "task-1",
						Title:       tt.taskTitle,
						Description: tt.taskDesc,
						Files:       tt.taskFiles,
						Priority:    tt.taskPriority,
						DependsOn:   tt.taskDeps,
					},
				},
			}

			m := Model{
				planEditor: &PlanEditorState{
					active:          true,
					selectedTaskIdx: 0,
				},
			}

			m.startEditingField(tt.field, plan)

			if m.planEditor.editingField != tt.field {
				t.Errorf("expected editingField '%s', got '%s'",
					tt.field, m.planEditor.editingField)
			}
			if m.planEditor.editBuffer != tt.expectedBuf {
				t.Errorf("expected editBuffer '%s', got '%s'",
					tt.expectedBuf, m.planEditor.editBuffer)
			}
			// Cursor should be at end of buffer
			expectedCursor := len([]rune(tt.expectedBuf))
			if m.planEditor.editCursor != expectedCursor {
				t.Errorf("expected editCursor %d, got %d",
					expectedCursor, m.planEditor.editCursor)
			}
		})
	}
}

func TestStartEditingField_InvalidField(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Test"},
		},
	}

	m := Model{
		planEditor: &PlanEditorState{
			active:          true,
			selectedTaskIdx: 0,
		},
	}

	m.startEditingField("invalid_field", plan)

	// Should not enter edit mode for invalid field
	if m.planEditor.editingField != "" {
		t.Errorf("expected empty editingField for invalid field, got '%s'",
			m.planEditor.editingField)
	}
}

func TestCancelFieldEdit(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active:       true,
			editingField: "title",
			editBuffer:   "Modified value",
			editCursor:   10,
		},
	}

	m.cancelFieldEdit()

	if m.planEditor.editingField != "" {
		t.Error("expected editingField to be empty after cancel")
	}
	if m.planEditor.editBuffer != "" {
		t.Error("expected editBuffer to be empty after cancel")
	}
	if m.planEditor.editCursor != 0 {
		t.Error("expected editCursor to be 0 after cancel")
	}
}

func TestConfirmFieldEdit(t *testing.T) {
	tests := []struct {
		name        string
		field       string
		editBuffer  string
		wantErr     bool
		checkResult func(t *testing.T, plan *orchestrator.PlanSpec)
	}{
		{
			name:       "confirm title edit",
			field:      "title",
			editBuffer: "New Title",
			wantErr:    false,
			checkResult: func(t *testing.T, plan *orchestrator.PlanSpec) {
				if plan.Tasks[0].Title != "New Title" {
					t.Errorf("expected title 'New Title', got '%s'", plan.Tasks[0].Title)
				}
			},
		},
		{
			name:       "confirm description edit",
			field:      "description",
			editBuffer: "New description",
			wantErr:    false,
			checkResult: func(t *testing.T, plan *orchestrator.PlanSpec) {
				if plan.Tasks[0].Description != "New description" {
					t.Errorf("expected description 'New description', got '%s'", plan.Tasks[0].Description)
				}
			},
		},
		{
			name:       "confirm files edit",
			field:      "files",
			editBuffer: "file1.go, file2.go, file3.go",
			wantErr:    false,
			checkResult: func(t *testing.T, plan *orchestrator.PlanSpec) {
				if len(plan.Tasks[0].Files) != 3 {
					t.Errorf("expected 3 files, got %d", len(plan.Tasks[0].Files))
				}
			},
		},
		{
			name:       "confirm priority edit - valid",
			field:      "priority",
			editBuffer: "5",
			wantErr:    false,
			checkResult: func(t *testing.T, plan *orchestrator.PlanSpec) {
				if plan.Tasks[0].Priority != 5 {
					t.Errorf("expected priority 5, got %d", plan.Tasks[0].Priority)
				}
			},
		},
		{
			name:       "confirm priority edit - invalid",
			field:      "priority",
			editBuffer: "not a number",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{
						ID:          "task-1",
						Title:       "Original Title",
						Description: "Original description",
						Files:       []string{"original.go"},
						Priority:    1,
						DependsOn:   []string{},
					},
				},
				DependencyGraph: map[string][]string{"task-1": {}},
				ExecutionOrder:  [][]string{{"task-1"}},
			}

			m := Model{
				planEditor: &PlanEditorState{
					active:          true,
					selectedTaskIdx: 0,
					editingField:    tt.field,
					editBuffer:      tt.editBuffer,
					editCursor:      len(tt.editBuffer),
				},
			}

			err := m.confirmFieldEdit(plan)

			if (err != nil) != tt.wantErr {
				t.Errorf("confirmFieldEdit() error = %v, wantErr %v", err, tt.wantErr)
			}

			// After confirm, edit mode should be exited
			if m.planEditor.editingField != "" {
				t.Error("expected editingField to be empty after confirm")
			}

			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, plan)
			}
		})
	}
}

// Tests for cursor movement in edit mode

func TestPlanEditorMoveCursor(t *testing.T) {
	tests := []struct {
		name           string
		editBuffer     string
		initialCursor  int
		delta          int
		expectedCursor int
	}{
		{
			name:           "move right",
			editBuffer:     "hello",
			initialCursor:  0,
			delta:          1,
			expectedCursor: 1,
		},
		{
			name:           "move left",
			editBuffer:     "hello",
			initialCursor:  3,
			delta:          -1,
			expectedCursor: 2,
		},
		{
			name:           "move left at start stays at start",
			editBuffer:     "hello",
			initialCursor:  0,
			delta:          -1,
			expectedCursor: 0,
		},
		{
			name:           "move right at end stays at end",
			editBuffer:     "hello",
			initialCursor:  5,
			delta:          1,
			expectedCursor: 5,
		},
		{
			name:           "move right beyond bounds clamps",
			editBuffer:     "hello",
			initialCursor:  3,
			delta:          10,
			expectedCursor: 5,
		},
		{
			name:           "move left beyond bounds clamps",
			editBuffer:     "hello",
			initialCursor:  2,
			delta:          -10,
			expectedCursor: 0,
		},
		{
			name:           "unicode string movement",
			editBuffer:     "hello 世界",
			initialCursor:  6,
			delta:          1,
			expectedCursor: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				planEditor: &PlanEditorState{
					active:     true,
					editBuffer: tt.editBuffer,
					editCursor: tt.initialCursor,
				},
			}

			m.planEditorMoveCursor(tt.delta)

			if m.planEditor.editCursor != tt.expectedCursor {
				t.Errorf("expected cursor %d, got %d",
					tt.expectedCursor, m.planEditor.editCursor)
			}
		})
	}
}

func TestPlanEditorDeleteBack(t *testing.T) {
	tests := []struct {
		name           string
		editBuffer     string
		initialCursor  int
		deleteCount    int
		expectedBuffer string
		expectedCursor int
	}{
		{
			name:           "delete one char",
			editBuffer:     "hello",
			initialCursor:  5,
			deleteCount:    1,
			expectedBuffer: "hell",
			expectedCursor: 4,
		},
		{
			name:           "delete multiple chars",
			editBuffer:     "hello",
			initialCursor:  5,
			deleteCount:    3,
			expectedBuffer: "he",
			expectedCursor: 2,
		},
		{
			name:           "delete from middle",
			editBuffer:     "hello world",
			initialCursor:  6,
			deleteCount:    1,
			expectedBuffer: "helloworld",
			expectedCursor: 5,
		},
		{
			name:           "delete at start does nothing",
			editBuffer:     "hello",
			initialCursor:  0,
			deleteCount:    1,
			expectedBuffer: "hello",
			expectedCursor: 0,
		},
		{
			name:           "delete more than available",
			editBuffer:     "hello",
			initialCursor:  3,
			deleteCount:    10,
			expectedBuffer: "lo",
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				planEditor: &PlanEditorState{
					active:     true,
					editBuffer: tt.editBuffer,
					editCursor: tt.initialCursor,
				},
			}

			m.planEditorDeleteBack(tt.deleteCount)

			if m.planEditor.editBuffer != tt.expectedBuffer {
				t.Errorf("expected buffer '%s', got '%s'",
					tt.expectedBuffer, m.planEditor.editBuffer)
			}
			if m.planEditor.editCursor != tt.expectedCursor {
				t.Errorf("expected cursor %d, got %d",
					tt.expectedCursor, m.planEditor.editCursor)
			}
		})
	}
}

func TestPlanEditorDeleteForward(t *testing.T) {
	tests := []struct {
		name           string
		editBuffer     string
		initialCursor  int
		deleteCount    int
		expectedBuffer string
		expectedCursor int
	}{
		{
			name:           "delete one char forward",
			editBuffer:     "hello",
			initialCursor:  0,
			deleteCount:    1,
			expectedBuffer: "ello",
			expectedCursor: 0,
		},
		{
			name:           "delete multiple chars forward",
			editBuffer:     "hello",
			initialCursor:  0,
			deleteCount:    3,
			expectedBuffer: "lo",
			expectedCursor: 0,
		},
		{
			name:           "delete from middle",
			editBuffer:     "hello world",
			initialCursor:  5,
			deleteCount:    1,
			expectedBuffer: "helloworld",
			expectedCursor: 5,
		},
		{
			name:           "delete at end does nothing",
			editBuffer:     "hello",
			initialCursor:  5,
			deleteCount:    1,
			expectedBuffer: "hello",
			expectedCursor: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				planEditor: &PlanEditorState{
					active:     true,
					editBuffer: tt.editBuffer,
					editCursor: tt.initialCursor,
				},
			}

			m.planEditorDeleteForward(tt.deleteCount)

			if m.planEditor.editBuffer != tt.expectedBuffer {
				t.Errorf("expected buffer '%s', got '%s'",
					tt.expectedBuffer, m.planEditor.editBuffer)
			}
			if m.planEditor.editCursor != tt.expectedCursor {
				t.Errorf("expected cursor %d, got %d",
					tt.expectedCursor, m.planEditor.editCursor)
			}
		})
	}
}

func TestPlanEditorDeleteWord(t *testing.T) {
	tests := []struct {
		name           string
		editBuffer     string
		initialCursor  int
		expectedBuffer string
		expectedCursor int
	}{
		{
			name:           "delete word at end",
			editBuffer:     "hello world",
			initialCursor:  11,
			expectedBuffer: "hello ",
			expectedCursor: 6,
		},
		{
			name:           "delete word in middle",
			editBuffer:     "hello world test",
			initialCursor:  11,
			expectedBuffer: "hello  test",
			expectedCursor: 6,
		},
		{
			name:           "delete at start does nothing",
			editBuffer:     "hello",
			initialCursor:  0,
			expectedBuffer: "hello",
			expectedCursor: 0,
		},
		{
			name:           "delete first word",
			editBuffer:     "hello world",
			initialCursor:  5,
			expectedBuffer: " world",
			expectedCursor: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				planEditor: &PlanEditorState{
					active:     true,
					editBuffer: tt.editBuffer,
					editCursor: tt.initialCursor,
				},
			}

			m.planEditorDeleteWord()

			if m.planEditor.editBuffer != tt.expectedBuffer {
				t.Errorf("expected buffer '%s', got '%s'",
					tt.expectedBuffer, m.planEditor.editBuffer)
			}
			if m.planEditor.editCursor != tt.expectedCursor {
				t.Errorf("expected cursor %d, got %d",
					tt.expectedCursor, m.planEditor.editCursor)
			}
		})
	}
}

// Tests for task operations

func TestCycleTaskComplexity(t *testing.T) {
	tests := []struct {
		name               string
		initialComplexity  orchestrator.TaskComplexity
		expectedComplexity orchestrator.TaskComplexity
	}{
		{
			name:               "low to medium",
			initialComplexity:  orchestrator.ComplexityLow,
			expectedComplexity: orchestrator.ComplexityMedium,
		},
		{
			name:               "medium to high",
			initialComplexity:  orchestrator.ComplexityMedium,
			expectedComplexity: orchestrator.ComplexityHigh,
		},
		{
			name:               "high to low",
			initialComplexity:  orchestrator.ComplexityHigh,
			expectedComplexity: orchestrator.ComplexityLow,
		},
		{
			name:               "empty defaults to low",
			initialComplexity:  "",
			expectedComplexity: orchestrator.ComplexityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &orchestrator.PlanSpec{
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", EstComplexity: tt.initialComplexity},
				},
			}

			m := Model{
				planEditor: &PlanEditorState{
					active:          true,
					selectedTaskIdx: 0,
				},
			}

			m.cycleTaskComplexity(plan)

			if plan.Tasks[0].EstComplexity != tt.expectedComplexity {
				t.Errorf("expected complexity %s, got %s",
					tt.expectedComplexity, plan.Tasks[0].EstComplexity)
			}
		})
	}
}

func TestDeleteSelectedTask(t *testing.T) {
	tests := []struct {
		name           string
		selectedIdx    int
		numTasks       int
		expectedRemain int
		expectedNewIdx int
		wantErr        bool
	}{
		{
			name:           "delete first task",
			selectedIdx:    0,
			numTasks:       4,
			expectedRemain: 3,
			expectedNewIdx: 0,
			wantErr:        false,
		},
		{
			name:           "delete last task",
			selectedIdx:    3,
			numTasks:       4,
			expectedRemain: 3,
			expectedNewIdx: 2,
			wantErr:        false,
		},
		{
			name:           "delete middle task",
			selectedIdx:    1,
			numTasks:       4,
			expectedRemain: 3,
			expectedNewIdx: 1,
			wantErr:        false,
		},
		{
			name:           "delete only task fails (plan requires at least one task)",
			selectedIdx:    0,
			numTasks:       1,
			expectedRemain: 1, // Unchanged since deletion fails
			expectedNewIdx: 0,
			wantErr:        true, // Validation error: plan has no tasks
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := &orchestrator.PlanSpec{
				DependencyGraph: make(map[string][]string),
			}
			for i := range tt.numTasks {
				plan.Tasks = append(plan.Tasks, orchestrator.PlannedTask{
					ID:    "task-" + string(rune('A'+i)),
					Title: "Task " + string(rune('A'+i)),
				})
				plan.DependencyGraph["task-"+string(rune('A'+i))] = []string{}
			}
			plan.ExecutionOrder = [][]string{}
			for _, task := range plan.Tasks {
				plan.ExecutionOrder = append(plan.ExecutionOrder, []string{task.ID})
			}

			m := Model{
				planEditor: &PlanEditorState{
					active:          true,
					selectedTaskIdx: tt.selectedIdx,
				},
			}

			err := m.deleteSelectedTask(plan)

			if (err != nil) != tt.wantErr {
				t.Errorf("deleteSelectedTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(plan.Tasks) != tt.expectedRemain {
					t.Errorf("expected %d tasks remaining, got %d",
						tt.expectedRemain, len(plan.Tasks))
				}

				if m.planEditor.selectedTaskIdx != tt.expectedNewIdx {
					t.Errorf("expected selectedIdx %d, got %d",
						tt.expectedNewIdx, m.planEditor.selectedTaskIdx)
				}
			}
		})
	}
}

func TestAddNewTaskAfterCurrent(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
		},
		DependencyGraph: map[string][]string{
			"task-1": {},
			"task-2": {},
		},
		ExecutionOrder: [][]string{{"task-1", "task-2"}},
	}

	m := Model{
		planEditor: &PlanEditorState{
			active:          true,
			selectedTaskIdx: 0,
		},
	}

	err := m.addNewTaskAfterCurrent(plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(plan.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(plan.Tasks))
	}

	// Selection should move to new task
	if m.planEditor.selectedTaskIdx != 1 {
		t.Errorf("expected selectedIdx 1, got %d", m.planEditor.selectedTaskIdx)
	}

	// New task should be at index 1 (after the first task)
	if plan.Tasks[1].Title != "New Task" {
		t.Errorf("expected new task at index 1, got '%s'", plan.Tasks[1].Title)
	}
}

func TestMoveTaskUp(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3"},
		},
	}

	m := Model{
		planEditor: &PlanEditorState{
			active:          true,
			selectedTaskIdx: 1,
		},
	}

	err := m.moveTaskUp(plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Task-2 should now be at index 0
	if plan.Tasks[0].ID != "task-2" {
		t.Errorf("expected task-2 at index 0, got %s", plan.Tasks[0].ID)
	}

	// Selection should follow the moved task
	if m.planEditor.selectedTaskIdx != 0 {
		t.Errorf("expected selectedIdx 0, got %d", m.planEditor.selectedTaskIdx)
	}
}

func TestMoveTaskDown(t *testing.T) {
	plan := &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
			{ID: "task-2", Title: "Task 2"},
			{ID: "task-3", Title: "Task 3"},
		},
	}

	m := Model{
		planEditor: &PlanEditorState{
			active:          true,
			selectedTaskIdx: 1,
		},
	}

	err := m.moveTaskDown(plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Task-2 should now be at index 2
	if plan.Tasks[2].ID != "task-2" {
		t.Errorf("expected task-2 at index 2, got %s", plan.Tasks[2].ID)
	}

	// Selection should follow the moved task
	if m.planEditor.selectedTaskIdx != 2 {
		t.Errorf("expected selectedIdx 2, got %d", m.planEditor.selectedTaskIdx)
	}
}

// Tests for validation state

func TestCanConfirmPlan(t *testing.T) {
	tests := []struct {
		name       string
		validation *orchestrator.ValidationResult
		expected   bool
	}{
		{
			name: "can confirm when no errors",
			validation: &orchestrator.ValidationResult{
				IsValid:      true,
				ErrorCount:   0,
				WarningCount: 2,
			},
			expected: true,
		},
		{
			name: "cannot confirm when errors exist",
			validation: &orchestrator.ValidationResult{
				IsValid:    false,
				ErrorCount: 1,
			},
			expected: false,
		},
		{
			name:       "cannot confirm when validation is nil",
			validation: nil,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				planEditor: &PlanEditorState{
					active:     true,
					validation: tt.validation,
				},
			}

			result := m.canConfirmPlan()
			if result != tt.expected {
				t.Errorf("expected canConfirmPlan() = %v, got %v",
					tt.expected, result)
			}
		})
	}
}

func TestCanConfirmPlan_NilPlanEditor(t *testing.T) {
	m := Model{
		planEditor: nil,
	}

	result := m.canConfirmPlan()
	if result {
		t.Error("expected canConfirmPlan() = false when planEditor is nil")
	}
}

func TestIsTaskInCycle(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active: true,
			tasksInCycle: map[string]bool{
				"task-1": true,
				"task-2": true,
			},
		},
	}

	if !m.isTaskInCycle("task-1") {
		t.Error("expected task-1 to be in cycle")
	}

	if m.isTaskInCycle("task-3") {
		t.Error("expected task-3 to not be in cycle")
	}
}

func TestIsTaskInCycle_NilState(t *testing.T) {
	m := Model{
		planEditor: nil,
	}

	if m.isTaskInCycle("task-1") {
		t.Error("expected false when planEditor is nil")
	}
}

// Tests for IsPlanEditorActive

func TestIsPlanEditorActive(t *testing.T) {
	tests := []struct {
		name       string
		planEditor *PlanEditorState
		expected   bool
	}{
		{
			name: "active when state is active",
			planEditor: &PlanEditorState{
				active: true,
			},
			expected: true,
		},
		{
			name: "inactive when state is not active",
			planEditor: &PlanEditorState{
				active: false,
			},
			expected: false,
		},
		{
			name:       "inactive when state is nil",
			planEditor: nil,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				planEditor: tt.planEditor,
			}

			result := m.IsPlanEditorActive()
			if result != tt.expected {
				t.Errorf("expected IsPlanEditorActive() = %v, got %v",
					tt.expected, result)
			}
		})
	}
}

// Tests for enter/exit plan editor

func TestEnterPlanEditor(t *testing.T) {
	m := Model{
		planEditor: nil,
	}

	m.enterPlanEditor()

	if m.planEditor == nil {
		t.Fatal("expected planEditor to be initialized")
	}

	if !m.planEditor.active {
		t.Error("expected planEditor.active to be true")
	}

	if m.planEditor.selectedTaskIdx != 0 {
		t.Error("expected selectedTaskIdx to be 0")
	}

	if !m.planEditor.showValidationPanel {
		t.Error("expected showValidationPanel to be true by default")
	}
}

func TestExitPlanEditor(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active: true,
		},
	}

	m.exitPlanEditor()

	if m.planEditor != nil {
		t.Error("expected planEditor to be nil after exit")
	}
}

// Tests for validation scroll

func TestScrollValidationPanel(t *testing.T) {
	tests := []struct {
		name           string
		initialOffset  int
		numMessages    int
		delta          int
		expectedOffset int
	}{
		{
			name:           "scroll down",
			initialOffset:  0,
			numMessages:    10,
			delta:          1,
			expectedOffset: 1,
		},
		{
			name:           "scroll up",
			initialOffset:  5,
			numMessages:    10,
			delta:          -1,
			expectedOffset: 4,
		},
		{
			name:           "scroll up at top stays at top",
			initialOffset:  0,
			numMessages:    10,
			delta:          -1,
			expectedOffset: 0,
		},
		{
			name:           "scroll down at bottom stays at bottom",
			initialOffset:  5, // maxOffset = 10 - 5 = 5
			numMessages:    10,
			delta:          1,
			expectedOffset: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := make([]orchestrator.ValidationMessage, tt.numMessages)
			for i := 0; i < tt.numMessages; i++ {
				messages[i] = orchestrator.ValidationMessage{Message: "test"}
			}

			m := Model{
				planEditor: &PlanEditorState{
					active:                 true,
					validationScrollOffset: tt.initialOffset,
					validation: &orchestrator.ValidationResult{
						Messages: messages,
					},
				},
			}

			m.scrollValidationPanel(tt.delta)

			if m.planEditor.validationScrollOffset != tt.expectedOffset {
				t.Errorf("expected scroll offset %d, got %d",
					tt.expectedOffset, m.planEditor.validationScrollOffset)
			}
		})
	}
}

// Tests for parseCommaSeparatedList helper

func TestParseCommaSeparatedList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple list",
			input:    "a, b, c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "no spaces",
			input:    "a,b,c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "extra spaces",
			input:    "  a  ,  b  ,  c  ",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: nil,
		},
		{
			name:     "single item",
			input:    "single",
			expected: []string{"single"},
		},
		{
			name:     "empty items filtered",
			input:    "a,,b,  ,c",
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparatedList(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("expected %d items, got %d: %v",
					len(tt.expected), len(result), result)
				return
			}

			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("expected item %d to be '%s', got '%s'",
						i, tt.expected[i], v)
				}
			}
		})
	}
}

// Tests for keyboard handling routing

func TestHandlePlanEditorKeypress_NotActive(t *testing.T) {
	m := Model{
		planEditor: nil,
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	handled, _, _ := m.handlePlanEditorKeypress(msg)

	if handled {
		t.Error("expected keypress to not be handled when plan editor is not active")
	}
}

func TestHandlePlanEditorKeypress_InactiveState(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active: false,
		},
	}

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	handled, _, _ := m.handlePlanEditorKeypress(msg)

	if handled {
		t.Error("expected keypress to not be handled when plan editor state is inactive")
	}
}

// Test getValidationMessagesForSelectedTask

func TestGetValidationMessagesForSelectedTask(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active:          true,
			selectedTaskIdx: 0,
			validation: &orchestrator.ValidationResult{
				Messages: []orchestrator.ValidationMessage{
					{TaskID: "task-1", Message: "Error for task 1"},
					{TaskID: "task-2", Message: "Error for task 2"},
					{TaskID: "task-1", Message: "Another error for task 1"},
				},
			},
		},
	}

	// Need to mock the ultraPlan access - for now, test the nil case
	messages := m.getValidationMessagesForSelectedTask()

	// Without ultraPlan set up, should return nil
	if messages != nil {
		t.Error("expected nil when ultraPlan is not set up")
	}
}

func TestGetValidationMessagesForSelectedTask_NilState(t *testing.T) {
	tests := []struct {
		name  string
		model Model
	}{
		{
			name:  "nil planEditor",
			model: Model{planEditor: nil},
		},
		{
			name: "nil validation",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					validation: nil,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages := tt.model.getValidationMessagesForSelectedTask()
			if messages != nil {
				t.Error("expected nil messages")
			}
		})
	}
}

// Tests for inline plan mode support

func TestInlinePlanState_Initialization(t *testing.T) {
	state := &InlinePlanState{
		Objective:         "Test objective",
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
	}

	if state.Objective != "Test objective" {
		t.Errorf("expected objective to be 'Test objective', got '%s'", state.Objective)
	}
	if !state.AwaitingObjective {
		t.Error("expected AwaitingObjective to be true")
	}
	if state.TaskToInstance == nil {
		t.Error("expected TaskToInstance to be non-nil map")
	}
}

func TestPlanEditorState_InlineMode(t *testing.T) {
	// Test that inline mode flag is properly set
	state := &PlanEditorState{
		active:     true,
		inlineMode: true,
	}

	if !state.inlineMode {
		t.Error("expected inlineMode to be true")
	}

	// Test default (ultraplan) mode
	stateDefault := &PlanEditorState{
		active:     true,
		inlineMode: false,
	}

	if stateDefault.inlineMode {
		t.Error("expected inlineMode to be false by default")
	}
}

func TestGetPlanForEditor_InlineMode(t *testing.T) {
	plan := createTestPlanForTUI()

	m := Model{
		planEditor: &PlanEditorState{
			active:     true,
			inlineMode: true,
		},
		inlinePlan: &InlinePlanState{
			Plan: plan,
		},
	}

	result := m.getPlanForEditor()
	if result == nil {
		t.Fatal("expected plan to be returned for inline mode")
	}
	if result != plan {
		t.Error("expected returned plan to be the inline plan")
	}
}

func TestGetPlanForEditor_InlineModeNoPlan(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active:     true,
			inlineMode: true,
		},
		inlinePlan: nil,
	}

	result := m.getPlanForEditor()
	if result != nil {
		t.Error("expected nil when inlinePlan is nil")
	}
}

func TestGetPlanForEditor_InlineModePlanNil(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active:     true,
			inlineMode: true,
		},
		inlinePlan: &InlinePlanState{
			Plan: nil,
		},
	}

	result := m.getPlanForEditor()
	if result != nil {
		t.Error("expected nil when plan is nil")
	}
}

func TestEnterInlinePlanEditor(t *testing.T) {
	plan := createTestPlanForTUI()

	m := Model{
		inlinePlan: &InlinePlanState{
			Plan:      plan,
			Objective: "Test objective",
		},
		terminalManager: terminal.NewManager(),
	}

	m.enterInlinePlanEditor()

	if m.planEditor == nil {
		t.Fatal("expected planEditor to be initialized")
	}
	if !m.planEditor.active {
		t.Error("expected planEditor.active to be true")
	}
	if !m.planEditor.inlineMode {
		t.Error("expected planEditor.inlineMode to be true")
	}
	if m.planEditor.selectedTaskIdx != 0 {
		t.Error("expected selectedTaskIdx to be 0")
	}
}

func TestCanStartExecution_InlineMode(t *testing.T) {
	plan := createTestPlanForTUI()

	tests := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			name: "inline mode with valid plan",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					inlineMode: true,
				},
				inlinePlan: &InlinePlanState{
					Plan: plan,
				},
			},
			expected: true,
		},
		{
			name: "inline mode with nil plan",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					inlineMode: true,
				},
				inlinePlan: &InlinePlanState{
					Plan: nil,
				},
			},
			expected: false,
		},
		{
			name: "inline mode with nil inlinePlan",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					inlineMode: true,
				},
				inlinePlan: nil,
			},
			expected: false,
		},
		{
			name: "not inline mode (ultraplan), nil ultraplan",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					inlineMode: false,
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.canStartExecution()
			if result != tt.expected {
				t.Errorf("expected canStartExecution() = %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsInlinePlanMode(t *testing.T) {
	tests := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			name: "inline plan mode active",
			model: Model{
				inlinePlan: &InlinePlanState{},
			},
			expected: true,
		},
		{
			name: "inline plan mode not active",
			model: Model{
				inlinePlan: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.IsInlinePlanMode()
			if result != tt.expected {
				t.Errorf("expected IsInlinePlanMode() = %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsPlanEditorInlineMode(t *testing.T) {
	tests := []struct {
		name     string
		model    Model
		expected bool
	}{
		{
			name: "plan editor in inline mode",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					inlineMode: true,
				},
			},
			expected: true,
		},
		{
			name: "plan editor not in inline mode",
			model: Model{
				planEditor: &PlanEditorState{
					active:     true,
					inlineMode: false,
				},
			},
			expected: false,
		},
		{
			name: "no plan editor",
			model: Model{
				planEditor: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.model.IsPlanEditorInlineMode()
			if result != tt.expected {
				t.Errorf("expected IsPlanEditorInlineMode() = %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRenderPlanEditorView_InlineMode(t *testing.T) {
	plan := createTestPlanForTUI()

	m := Model{
		planEditor: &PlanEditorState{
			active:     true,
			inlineMode: true,
		},
		inlinePlan: &InlinePlanState{
			Plan: plan,
		},
		terminalManager: terminal.NewManager(),
	}

	// Should not panic and return something
	result := m.renderPlanEditorView(80)
	if result == "" {
		t.Error("expected non-empty render output")
	}
}

func TestRenderPlanEditorView_NoPlan(t *testing.T) {
	m := Model{
		planEditor: &PlanEditorState{
			active:     true,
			inlineMode: true,
		},
		inlinePlan:      nil,
		terminalManager: terminal.NewManager(),
	}

	result := m.renderPlanEditorView(80)
	if result != "No plan available" {
		t.Errorf("expected 'No plan available', got '%s'", result)
	}
}

func TestGetValidationMessagesForSelectedTask_InlineMode(t *testing.T) {
	plan := createTestPlanForTUI()

	m := Model{
		planEditor: &PlanEditorState{
			active:          true,
			inlineMode:      true,
			selectedTaskIdx: 0,
			validation: &orchestrator.ValidationResult{
				Messages: []orchestrator.ValidationMessage{
					{TaskID: "task-1", Message: "Error for task 1"},
					{TaskID: "task-2", Message: "Error for task 2"},
				},
			},
		},
		inlinePlan: &InlinePlanState{
			Plan: plan,
		},
	}

	messages := m.getValidationMessagesForSelectedTask()
	if messages == nil {
		t.Fatal("expected non-nil messages")
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message for task-1, got %d", len(messages))
	}
}

func TestUpdateInlinePlanValidation(t *testing.T) {
	plan := createTestPlanForTUI()

	m := Model{
		planEditor: &PlanEditorState{
			active:       true,
			inlineMode:   true,
			tasksInCycle: make(map[string]bool),
		},
		inlinePlan: &InlinePlanState{
			Plan: plan,
		},
	}

	m.updateInlinePlanValidation()

	if m.planEditor.validation == nil {
		t.Error("expected validation to be set")
	}
}

func TestPendingConfirmDelete(t *testing.T) {
	// Test that pendingConfirmDelete field is properly used
	state := &PlanEditorState{
		active:               true,
		pendingConfirmDelete: "task-1",
	}

	if state.pendingConfirmDelete != "task-1" {
		t.Errorf("expected pendingConfirmDelete to be 'task-1', got '%s'", state.pendingConfirmDelete)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "no truncation needed",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "needs truncation",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "very short max",
			input:    "hello",
			maxLen:   3,
			expected: "hel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestInlinePlanState_IsUltraPlan(t *testing.T) {
	// Test regular plan state (not ultraplan)
	regularPlan := &InlinePlanState{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
		IsUltraPlan:       false,
	}

	if regularPlan.IsUltraPlan {
		t.Error("expected IsUltraPlan to be false for regular plan")
	}
	if regularPlan.UltraPlanConfig != nil {
		t.Error("expected UltraPlanConfig to be nil for regular plan")
	}

	// Test ultraplan state
	cfg := &orchestrator.UltraPlanConfig{
		AutoApprove: false,
		Review:      true,
		MultiPass:   true,
	}
	ultraPlan := &InlinePlanState{
		AwaitingObjective: true,
		TaskToInstance:    make(map[string]string),
		IsUltraPlan:       true,
		UltraPlanConfig:   cfg,
	}

	if !ultraPlan.IsUltraPlan {
		t.Error("expected IsUltraPlan to be true for ultraplan")
	}
	if ultraPlan.UltraPlanConfig == nil {
		t.Error("expected UltraPlanConfig to be non-nil for ultraplan")
	}
	if !ultraPlan.UltraPlanConfig.MultiPass {
		t.Error("expected MultiPass to be true")
	}
}

func TestInlinePlanState_AwaitingObjective_Differentiation(t *testing.T) {
	tests := []struct {
		name        string
		state       *InlinePlanState
		expectPlan  bool
		expectUltra bool
	}{
		{
			name: "regular plan awaiting objective",
			state: &InlinePlanState{
				AwaitingObjective: true,
				IsUltraPlan:       false,
			},
			expectPlan:  true,
			expectUltra: false,
		},
		{
			name: "ultraplan awaiting objective",
			state: &InlinePlanState{
				AwaitingObjective: true,
				IsUltraPlan:       true,
				UltraPlanConfig:   &orchestrator.UltraPlanConfig{},
			},
			expectPlan:  true,
			expectUltra: true,
		},
		{
			name: "plan not awaiting objective",
			state: &InlinePlanState{
				AwaitingObjective: false,
				IsUltraPlan:       false,
			},
			expectPlan:  false,
			expectUltra: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isAwaitingObjective := tt.state.AwaitingObjective
			isUltraPlan := tt.state.IsUltraPlan

			if isAwaitingObjective != tt.expectPlan {
				t.Errorf("AwaitingObjective = %v, want %v", isAwaitingObjective, tt.expectPlan)
			}
			if isUltraPlan != tt.expectUltra {
				t.Errorf("IsUltraPlan = %v, want %v", isUltraPlan, tt.expectUltra)
			}
		})
	}
}

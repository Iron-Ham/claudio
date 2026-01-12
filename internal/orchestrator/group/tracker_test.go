package group

import (
	"testing"
)

// mockPlanData implements PlanData for testing.
type mockPlanData struct {
	executionOrder [][]string
	tasks          map[string]*Task
}

func (m *mockPlanData) GetExecutionOrder() [][]string {
	return m.executionOrder
}

func (m *mockPlanData) GetTask(taskID string) *Task {
	if m.tasks == nil {
		return nil
	}
	return m.tasks[taskID]
}

// mockSessionData implements SessionData for testing.
type mockSessionData struct {
	plan             PlanData
	completedTasks   []string
	failedTasks      []string
	taskCommitCounts map[string]int
	currentGroup     int
}

func (m *mockSessionData) GetPlan() PlanData {
	return m.plan
}

func (m *mockSessionData) GetCompletedTasks() []string {
	return m.completedTasks
}

func (m *mockSessionData) GetFailedTasks() []string {
	return m.failedTasks
}

func (m *mockSessionData) GetTaskCommitCounts() map[string]int {
	return m.taskCommitCounts
}

func (m *mockSessionData) GetCurrentGroup() int {
	return m.currentGroup
}

func TestNewTracker(t *testing.T) {
	session := &mockSessionData{}
	tracker := NewTracker(session)

	if tracker == nil {
		t.Fatal("NewTracker returned nil")
	}
	if tracker.session != session {
		t.Error("tracker.session does not match provided session")
	}
}

func TestGetTaskGroupIndex(t *testing.T) {
	tests := []struct {
		name           string
		executionOrder [][]string
		taskID         string
		wantIndex      int
	}{
		{
			name:           "task in first group",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			taskID:         "task-1",
			wantIndex:      0,
		},
		{
			name:           "task in second group",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3", "task-4"}},
			taskID:         "task-3",
			wantIndex:      1,
		},
		{
			name:           "task not found",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			taskID:         "task-99",
			wantIndex:      -1,
		},
		{
			name:           "empty execution order",
			executionOrder: [][]string{},
			taskID:         "task-1",
			wantIndex:      -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan: &mockPlanData{executionOrder: tt.executionOrder},
			}
			tracker := NewTracker(session)
			got := tracker.GetTaskGroupIndex(tt.taskID)
			if got != tt.wantIndex {
				t.Errorf("GetTaskGroupIndex(%q) = %d, want %d", tt.taskID, got, tt.wantIndex)
			}
		})
	}
}

func TestGetTaskGroupIndex_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.GetTaskGroupIndex("task-1")
	if got != -1 {
		t.Errorf("GetTaskGroupIndex with nil plan = %d, want -1", got)
	}
}

func TestIsGroupComplete(t *testing.T) {
	tests := []struct {
		name           string
		executionOrder [][]string
		completedTasks []string
		failedTasks    []string
		groupIndex     int
		want           bool
	}{
		{
			name:           "all tasks completed",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks: []string{"task-1", "task-2"},
			failedTasks:    []string{},
			groupIndex:     0,
			want:           true,
		},
		{
			name:           "some tasks completed some failed",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks: []string{"task-1"},
			failedTasks:    []string{"task-2"},
			groupIndex:     0,
			want:           true,
		},
		{
			name:           "all tasks failed",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks: []string{},
			failedTasks:    []string{"task-1", "task-2"},
			groupIndex:     0,
			want:           true,
		},
		{
			name:           "task still pending",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks: []string{"task-1"},
			failedTasks:    []string{},
			groupIndex:     0,
			want:           false,
		},
		{
			name:           "invalid group index negative",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			completedTasks: []string{"task-1"},
			failedTasks:    []string{},
			groupIndex:     -1,
			want:           false,
		},
		{
			name:           "invalid group index too high",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			completedTasks: []string{"task-1"},
			failedTasks:    []string{},
			groupIndex:     5,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan:           &mockPlanData{executionOrder: tt.executionOrder},
				completedTasks: tt.completedTasks,
				failedTasks:    tt.failedTasks,
			}
			tracker := NewTracker(session)
			got := tracker.IsGroupComplete(tt.groupIndex)
			if got != tt.want {
				t.Errorf("IsGroupComplete(%d) = %v, want %v", tt.groupIndex, got, tt.want)
			}
		})
	}
}

func TestIsGroupComplete_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.IsGroupComplete(0)
	if got != false {
		t.Errorf("IsGroupComplete with nil plan = %v, want false", got)
	}
}

func TestGetGroupTasks(t *testing.T) {
	tasks := map[string]*Task{
		"task-1": {ID: "task-1", Title: "Task 1"},
		"task-2": {ID: "task-2", Title: "Task 2"},
		"task-3": {ID: "task-3", Title: "Task 3"},
	}

	tests := []struct {
		name           string
		executionOrder [][]string
		tasks          map[string]*Task
		groupIndex     int
		wantCount      int
		wantNil        bool
	}{
		{
			name:           "get tasks from first group",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			tasks:          tasks,
			groupIndex:     0,
			wantCount:      2,
		},
		{
			name:           "get tasks from second group",
			executionOrder: [][]string{{"task-1", "task-2"}, {"task-3"}},
			tasks:          tasks,
			groupIndex:     1,
			wantCount:      1,
		},
		{
			name:           "invalid group index negative",
			executionOrder: [][]string{{"task-1"}},
			tasks:          tasks,
			groupIndex:     -1,
			wantNil:        true,
		},
		{
			name:           "invalid group index too high",
			executionOrder: [][]string{{"task-1"}},
			tasks:          tasks,
			groupIndex:     10,
			wantNil:        true,
		},
		{
			name:           "task not found in tasks map",
			executionOrder: [][]string{{"task-99"}},
			tasks:          tasks,
			groupIndex:     0,
			wantCount:      0, // task-99 not in tasks map
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan: &mockPlanData{
					executionOrder: tt.executionOrder,
					tasks:          tt.tasks,
				},
			}
			tracker := NewTracker(session)
			got := tracker.GetGroupTasks(tt.groupIndex)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetGroupTasks(%d) = %v, want nil", tt.groupIndex, got)
				}
				return
			}

			if got == nil {
				t.Fatalf("GetGroupTasks(%d) = nil, want non-nil", tt.groupIndex)
			}

			if len(got) != tt.wantCount {
				t.Errorf("GetGroupTasks(%d) returned %d tasks, want %d", tt.groupIndex, len(got), tt.wantCount)
			}
		})
	}
}

func TestGetGroupTasks_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.GetGroupTasks(0)
	if got != nil {
		t.Errorf("GetGroupTasks with nil plan = %v, want nil", got)
	}
}

func TestAdvanceGroup(t *testing.T) {
	tests := []struct {
		name           string
		executionOrder [][]string
		groupIndex     int
		wantNext       int
		wantDone       bool
	}{
		{
			name:           "advance from first group",
			executionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			groupIndex:     0,
			wantNext:       1,
			wantDone:       false,
		},
		{
			name:           "advance to last group",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			groupIndex:     0,
			wantNext:       1,
			wantDone:       false,
		},
		{
			name:           "advance past last group",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			groupIndex:     1,
			wantNext:       2,
			wantDone:       true,
		},
		{
			name:           "single group",
			executionOrder: [][]string{{"task-1"}},
			groupIndex:     0,
			wantNext:       1,
			wantDone:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan: &mockPlanData{executionOrder: tt.executionOrder},
			}
			tracker := NewTracker(session)
			gotNext, gotDone := tracker.AdvanceGroup(tt.groupIndex)

			if gotNext != tt.wantNext {
				t.Errorf("AdvanceGroup(%d) nextGroup = %d, want %d", tt.groupIndex, gotNext, tt.wantNext)
			}
			if gotDone != tt.wantDone {
				t.Errorf("AdvanceGroup(%d) done = %v, want %v", tt.groupIndex, gotDone, tt.wantDone)
			}
		})
	}
}

func TestAdvanceGroup_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	gotNext, gotDone := tracker.AdvanceGroup(0)
	if gotNext != 0 {
		t.Errorf("AdvanceGroup with nil plan nextGroup = %d, want 0", gotNext)
	}
	if gotDone != true {
		t.Errorf("AdvanceGroup with nil plan done = %v, want true", gotDone)
	}
}

func TestHasPartialFailure(t *testing.T) {
	tests := []struct {
		name             string
		executionOrder   [][]string
		completedTasks   []string
		failedTasks      []string
		taskCommitCounts map[string]int
		groupIndex       int
		want             bool
	}{
		{
			name:             "partial failure - one success one failure",
			executionOrder:   [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks:   []string{"task-1", "task-2"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-1": 2, "task-2": 0}, // task-2 completed but no commits
			groupIndex:       0,
			want:             true,
		},
		{
			name:             "partial failure - success and explicit failure",
			executionOrder:   [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks:   []string{"task-1"},
			failedTasks:      []string{"task-2"},
			taskCommitCounts: map[string]int{"task-1": 3},
			groupIndex:       0,
			want:             true,
		},
		{
			name:             "all success",
			executionOrder:   [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks:   []string{"task-1", "task-2"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-1": 1, "task-2": 2},
			groupIndex:       0,
			want:             false,
		},
		{
			name:             "all failed",
			executionOrder:   [][]string{{"task-1", "task-2"}, {"task-3"}},
			completedTasks:   []string{},
			failedTasks:      []string{"task-1", "task-2"},
			taskCommitCounts: map[string]int{},
			groupIndex:       0,
			want:             false,
		},
		{
			name:             "all completed but no commits",
			executionOrder:   [][]string{{"task-1", "task-2"}},
			completedTasks:   []string{"task-1", "task-2"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-1": 0, "task-2": 0},
			groupIndex:       0,
			want:             false, // All failures, no successes
		},
		{
			name:             "invalid group index",
			executionOrder:   [][]string{{"task-1"}},
			completedTasks:   []string{"task-1"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-1": 1},
			groupIndex:       5,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan:             &mockPlanData{executionOrder: tt.executionOrder},
				completedTasks:   tt.completedTasks,
				failedTasks:      tt.failedTasks,
				taskCommitCounts: tt.taskCommitCounts,
			}
			tracker := NewTracker(session)
			got := tracker.HasPartialFailure(tt.groupIndex)

			if got != tt.want {
				t.Errorf("HasPartialFailure(%d) = %v, want %v", tt.groupIndex, got, tt.want)
			}
		})
	}
}

func TestHasPartialFailure_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.HasPartialFailure(0)
	if got != false {
		t.Errorf("HasPartialFailure with nil plan = %v, want false", got)
	}
}

func TestGetGroupStatus(t *testing.T) {
	tests := []struct {
		name             string
		executionOrder   [][]string
		completedTasks   []string
		failedTasks      []string
		taskCommitCounts map[string]int
		groupIndex       int
		wantNil          bool
		wantStatus       *GroupStatus
	}{
		{
			name:             "mixed status group",
			executionOrder:   [][]string{{"task-1", "task-2", "task-3"}, {"task-4"}},
			completedTasks:   []string{"task-1", "task-2"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-1": 2, "task-2": 0}, // task-2 no commits
			groupIndex:       0,
			wantStatus: &GroupStatus{
				GroupIndex:     0,
				TotalTasks:     3,
				CompletedTasks: 1,
				FailedTasks:    1,
				PendingTasks:   1,
				SuccessfulIDs:  []string{"task-1"},
				FailedIDs:      []string{"task-2"},
				PendingIDs:     []string{"task-3"},
			},
		},
		{
			name:             "all completed with commits",
			executionOrder:   [][]string{{"task-1", "task-2"}},
			completedTasks:   []string{"task-1", "task-2"},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{"task-1": 1, "task-2": 3},
			groupIndex:       0,
			wantStatus: &GroupStatus{
				GroupIndex:     0,
				TotalTasks:     2,
				CompletedTasks: 2,
				FailedTasks:    0,
				PendingTasks:   0,
				SuccessfulIDs:  []string{"task-1", "task-2"},
				FailedIDs:      []string{},
				PendingIDs:     []string{},
			},
		},
		{
			name:             "invalid group index",
			executionOrder:   [][]string{{"task-1"}},
			completedTasks:   []string{},
			failedTasks:      []string{},
			taskCommitCounts: map[string]int{},
			groupIndex:       10,
			wantNil:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan:             &mockPlanData{executionOrder: tt.executionOrder},
				completedTasks:   tt.completedTasks,
				failedTasks:      tt.failedTasks,
				taskCommitCounts: tt.taskCommitCounts,
			}
			tracker := NewTracker(session)
			got := tracker.GetGroupStatus(tt.groupIndex)

			if tt.wantNil {
				if got != nil {
					t.Errorf("GetGroupStatus(%d) = %v, want nil", tt.groupIndex, got)
				}
				return
			}

			if got == nil {
				t.Fatalf("GetGroupStatus(%d) = nil, want non-nil", tt.groupIndex)
			}

			if got.GroupIndex != tt.wantStatus.GroupIndex {
				t.Errorf("GroupIndex = %d, want %d", got.GroupIndex, tt.wantStatus.GroupIndex)
			}
			if got.TotalTasks != tt.wantStatus.TotalTasks {
				t.Errorf("TotalTasks = %d, want %d", got.TotalTasks, tt.wantStatus.TotalTasks)
			}
			if got.CompletedTasks != tt.wantStatus.CompletedTasks {
				t.Errorf("CompletedTasks = %d, want %d", got.CompletedTasks, tt.wantStatus.CompletedTasks)
			}
			if got.FailedTasks != tt.wantStatus.FailedTasks {
				t.Errorf("FailedTasks = %d, want %d", got.FailedTasks, tt.wantStatus.FailedTasks)
			}
			if got.PendingTasks != tt.wantStatus.PendingTasks {
				t.Errorf("PendingTasks = %d, want %d", got.PendingTasks, tt.wantStatus.PendingTasks)
			}
			if len(got.SuccessfulIDs) != len(tt.wantStatus.SuccessfulIDs) {
				t.Errorf("len(SuccessfulIDs) = %d, want %d", len(got.SuccessfulIDs), len(tt.wantStatus.SuccessfulIDs))
			}
			if len(got.FailedIDs) != len(tt.wantStatus.FailedIDs) {
				t.Errorf("len(FailedIDs) = %d, want %d", len(got.FailedIDs), len(tt.wantStatus.FailedIDs))
			}
			if len(got.PendingIDs) != len(tt.wantStatus.PendingIDs) {
				t.Errorf("len(PendingIDs) = %d, want %d", len(got.PendingIDs), len(tt.wantStatus.PendingIDs))
			}
		})
	}
}

func TestGetGroupStatus_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.GetGroupStatus(0)
	if got != nil {
		t.Errorf("GetGroupStatus with nil plan = %v, want nil", got)
	}
}

func TestTotalGroups(t *testing.T) {
	tests := []struct {
		name           string
		executionOrder [][]string
		want           int
	}{
		{
			name:           "three groups",
			executionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			want:           3,
		},
		{
			name:           "single group",
			executionOrder: [][]string{{"task-1", "task-2"}},
			want:           1,
		},
		{
			name:           "empty execution order",
			executionOrder: [][]string{},
			want:           0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan: &mockPlanData{executionOrder: tt.executionOrder},
			}
			tracker := NewTracker(session)
			got := tracker.TotalGroups()

			if got != tt.want {
				t.Errorf("TotalGroups() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestTotalGroups_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.TotalGroups()
	if got != 0 {
		t.Errorf("TotalGroups with nil plan = %d, want 0", got)
	}
}

func TestHasMoreGroups(t *testing.T) {
	tests := []struct {
		name           string
		executionOrder [][]string
		groupIndex     int
		want           bool
	}{
		{
			name:           "more groups after first",
			executionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			groupIndex:     0,
			want:           true,
		},
		{
			name:           "more groups after middle",
			executionOrder: [][]string{{"task-1"}, {"task-2"}, {"task-3"}},
			groupIndex:     1,
			want:           true,
		},
		{
			name:           "no more groups after last",
			executionOrder: [][]string{{"task-1"}, {"task-2"}},
			groupIndex:     1,
			want:           false,
		},
		{
			name:           "index beyond groups",
			executionOrder: [][]string{{"task-1"}},
			groupIndex:     5,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &mockSessionData{
				plan: &mockPlanData{executionOrder: tt.executionOrder},
			}
			tracker := NewTracker(session)
			got := tracker.HasMoreGroups(tt.groupIndex)

			if got != tt.want {
				t.Errorf("HasMoreGroups(%d) = %v, want %v", tt.groupIndex, got, tt.want)
			}
		})
	}
}

func TestHasMoreGroups_NilPlan(t *testing.T) {
	session := &mockSessionData{plan: nil}
	tracker := NewTracker(session)

	got := tracker.HasMoreGroups(0)
	if got != false {
		t.Errorf("HasMoreGroups with nil plan = %v, want false", got)
	}
}

// Test edge case: task appears in multiple groups (should find first occurrence)
func TestGetTaskGroupIndex_DuplicateTask(t *testing.T) {
	session := &mockSessionData{
		plan: &mockPlanData{
			// This shouldn't happen in practice, but test the behavior
			executionOrder: [][]string{{"task-1", "task-dup"}, {"task-2", "task-dup"}},
		},
	}
	tracker := NewTracker(session)

	// Should return the first group where the task appears
	got := tracker.GetTaskGroupIndex("task-dup")
	if got != 0 {
		t.Errorf("GetTaskGroupIndex for duplicate task = %d, want 0 (first occurrence)", got)
	}
}

// Test complex scenario: multiple groups with mixed states
func TestComplexGroupScenario(t *testing.T) {
	executionOrder := [][]string{
		{"task-1", "task-2", "task-3"}, // Group 0
		{"task-4", "task-5"},           // Group 1
		{"task-6"},                     // Group 2
	}

	session := &mockSessionData{
		plan:             &mockPlanData{executionOrder: executionOrder},
		completedTasks:   []string{"task-1", "task-2", "task-3", "task-4"},
		failedTasks:      []string{"task-5"},
		taskCommitCounts: map[string]int{"task-1": 2, "task-2": 1, "task-3": 3, "task-4": 1},
	}

	tracker := NewTracker(session)

	// Group 0 should be complete with all success
	if !tracker.IsGroupComplete(0) {
		t.Error("Group 0 should be complete")
	}
	if tracker.HasPartialFailure(0) {
		t.Error("Group 0 should not have partial failure")
	}

	// Group 1 should be complete with partial failure
	if !tracker.IsGroupComplete(1) {
		t.Error("Group 1 should be complete")
	}
	if !tracker.HasPartialFailure(1) {
		t.Error("Group 1 should have partial failure")
	}

	// Group 2 should not be complete
	if tracker.IsGroupComplete(2) {
		t.Error("Group 2 should not be complete")
	}

	// Test advancement
	next, done := tracker.AdvanceGroup(1)
	if next != 2 {
		t.Errorf("AdvanceGroup(1) next = %d, want 2", next)
	}
	if done {
		t.Error("AdvanceGroup(1) should not be done")
	}

	next, done = tracker.AdvanceGroup(2)
	if next != 3 {
		t.Errorf("AdvanceGroup(2) next = %d, want 3", next)
	}
	if !done {
		t.Error("AdvanceGroup(2) should be done")
	}

	// Test total groups
	if tracker.TotalGroups() != 3 {
		t.Errorf("TotalGroups() = %d, want 3", tracker.TotalGroups())
	}

	// Test has more groups
	if !tracker.HasMoreGroups(0) {
		t.Error("HasMoreGroups(0) should be true")
	}
	if !tracker.HasMoreGroups(1) {
		t.Error("HasMoreGroups(1) should be true")
	}
	if tracker.HasMoreGroups(2) {
		t.Error("HasMoreGroups(2) should be false")
	}
}

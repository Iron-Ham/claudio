package taskqueue

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func TestIsClaimable(t *testing.T) {
	tasks := map[string]*QueuedTask{
		"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", DependsOn: []string{}}, Status: TaskCompleted},
		"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", DependsOn: []string{"a"}}, Status: TaskPending},
		"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", DependsOn: []string{"a", "d"}}, Status: TaskPending},
		"d": {PlannedTask: ultraplan.PlannedTask{ID: "d", DependsOn: []string{}}, Status: TaskRunning},
		"e": {PlannedTask: ultraplan.PlannedTask{ID: "e", DependsOn: []string{}}, Status: TaskClaimed},
		"f": {PlannedTask: ultraplan.PlannedTask{ID: "f", DependsOn: []string{}}, Status: TaskPending},
	}
	q := &TaskQueue{tasks: tasks}

	tests := []struct {
		taskID    string
		claimable bool
	}{
		{"a", false}, // completed, not pending
		{"b", true},  // pending, dep "a" is completed
		{"c", false}, // pending, but dep "d" is running
		{"d", false}, // running, not pending
		{"e", false}, // claimed, not pending
		{"f", true},  // pending, no deps
	}
	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			got := q.isClaimable(tasks[tt.taskID])
			if got != tt.claimable {
				t.Errorf("isClaimable(%q) = %v, want %v", tt.taskID, got, tt.claimable)
			}
		})
	}
}

func TestIsClaimable_MissingDependency(t *testing.T) {
	tasks := map[string]*QueuedTask{
		"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", DependsOn: []string{"nonexistent"}}, Status: TaskPending},
	}
	q := &TaskQueue{tasks: tasks}

	if q.isClaimable(tasks["a"]) {
		t.Error("isClaimable should return false when dependency does not exist in queue")
	}
}

func TestUnblockedBy(t *testing.T) {
	tasks := map[string]*QueuedTask{
		"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", DependsOn: []string{}}, Status: TaskCompleted},
		"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", DependsOn: []string{"a"}}, Status: TaskPending},
		"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", DependsOn: []string{"a", "d"}}, Status: TaskPending},
		"d": {PlannedTask: ultraplan.PlannedTask{ID: "d", DependsOn: []string{}}, Status: TaskCompleted},
		"e": {PlannedTask: ultraplan.PlannedTask{ID: "e", DependsOn: []string{"a"}}, Status: TaskRunning},
	}
	order := []string{"a", "b", "c", "d", "e"}
	q := &TaskQueue{tasks: tasks, order: order}

	// Completing "a" should unblock "b" (dep "a" done) and "c" (deps "a" + "d" done)
	// but not "e" (already running)
	got := q.unblockedBy("a")
	want := map[string]bool{"b": true, "c": true}
	if len(got) != len(want) {
		t.Fatalf("unblockedBy(a) returned %v, want keys %v", got, want)
	}
	for _, id := range got {
		if !want[id] {
			t.Errorf("unblockedBy(a) included unexpected task %q", id)
		}
	}
}

func TestUnblockedBy_PartialDeps(t *testing.T) {
	tasks := map[string]*QueuedTask{
		"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", DependsOn: []string{}}, Status: TaskCompleted},
		"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", DependsOn: []string{}}, Status: TaskRunning},
		"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", DependsOn: []string{"a", "b"}}, Status: TaskPending},
	}
	order := []string{"a", "b", "c"}
	q := &TaskQueue{tasks: tasks, order: order}

	// Completing "a" should NOT unblock "c" because "b" is still running
	got := q.unblockedBy("a")
	if len(got) != 0 {
		t.Errorf("unblockedBy(a) = %v, want empty (b still running)", got)
	}
}

func TestBuildPriorityOrder(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		order := buildPriorityOrder(nil)
		if order != nil {
			t.Errorf("buildPriorityOrder(nil) = %v, want nil", order)
		}
	})

	t.Run("no dependencies", func(t *testing.T) {
		tasks := map[string]*QueuedTask{
			"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", Priority: 2, DependsOn: []string{}}},
			"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", Priority: 0, DependsOn: []string{}}},
			"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", Priority: 1, DependsOn: []string{}}},
		}
		order := buildPriorityOrder(tasks)
		if len(order) != 3 {
			t.Fatalf("expected 3 tasks, got %d", len(order))
		}
		// All at same level, sorted by priority
		if order[0] != "a" || order[1] != "b" || order[2] != "c" {
			t.Errorf("order = %v, want [a b c]", order)
		}
	})

	t.Run("linear chain", func(t *testing.T) {
		tasks := map[string]*QueuedTask{
			"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", Priority: 0, DependsOn: []string{"b"}}},
			"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", Priority: 0, DependsOn: []string{}}},
			"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", Priority: 0, DependsOn: []string{"a"}}},
		}
		order := buildPriorityOrder(tasks)
		if len(order) != 3 {
			t.Fatalf("expected 3 tasks, got %d", len(order))
		}
		if order[0] != "a" || order[1] != "b" || order[2] != "c" {
			t.Errorf("order = %v, want [a b c]", order)
		}
	})

	t.Run("diamond", func(t *testing.T) {
		tasks := map[string]*QueuedTask{
			"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", Priority: 0, DependsOn: []string{}}},
			"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", Priority: 0, DependsOn: []string{"a"}}},
			"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", Priority: 1, DependsOn: []string{"a"}}},
			"d": {PlannedTask: ultraplan.PlannedTask{ID: "d", Priority: 0, DependsOn: []string{"b", "c"}}},
		}
		order := buildPriorityOrder(tasks)
		if len(order) != 4 {
			t.Fatalf("expected 4 tasks, got %d", len(order))
		}
		// a must come first, d must come last
		if order[0] != "a" {
			t.Errorf("order[0] = %q, want a", order[0])
		}
		if order[3] != "d" {
			t.Errorf("order[3] = %q, want d", order[3])
		}
		// b and c should be ordered by priority (b=0, c=1)
		if order[1] != "b" || order[2] != "c" {
			t.Errorf("order[1:3] = %v, want [b c]", order[1:3])
		}
	})

	t.Run("wide parallel", func(t *testing.T) {
		// 5 tasks, all independent, different priorities
		tasks := map[string]*QueuedTask{
			"e": {PlannedTask: ultraplan.PlannedTask{ID: "e", Priority: 4, DependsOn: []string{}}},
			"d": {PlannedTask: ultraplan.PlannedTask{ID: "d", Priority: 3, DependsOn: []string{}}},
			"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", Priority: 2, DependsOn: []string{}}},
			"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", Priority: 1, DependsOn: []string{}}},
			"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", Priority: 0, DependsOn: []string{}}},
		}
		order := buildPriorityOrder(tasks)
		if len(order) != 5 {
			t.Fatalf("expected 5 tasks, got %d", len(order))
		}
		for i, want := range []string{"a", "b", "c", "d", "e"} {
			if order[i] != want {
				t.Errorf("order[%d] = %q, want %q", i, order[i], want)
			}
		}
	})
}

func TestSortByPriority(t *testing.T) {
	tasks := map[string]*QueuedTask{
		"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", Priority: 5}},
		"b": {PlannedTask: ultraplan.PlannedTask{ID: "b", Priority: 1}},
		"c": {PlannedTask: ultraplan.PlannedTask{ID: "c", Priority: 3}},
		"d": {PlannedTask: ultraplan.PlannedTask{ID: "d", Priority: -1}},
	}
	ids := []string{"a", "b", "c", "d"}
	sortByPriority(ids, tasks)

	want := []string{"d", "b", "c", "a"}
	for i, id := range ids {
		if id != want[i] {
			t.Errorf("sortByPriority()[%d] = %q, want %q", i, id, want[i])
		}
	}
}

func TestSortByPriority_EmptyAndSingle(t *testing.T) {
	tasks := map[string]*QueuedTask{
		"a": {PlannedTask: ultraplan.PlannedTask{ID: "a", Priority: 1}},
	}

	// Empty slice should not panic
	sortByPriority(nil, tasks)
	sortByPriority([]string{}, tasks)

	// Single element
	ids := []string{"a"}
	sortByPriority(ids, tasks)
	if ids[0] != "a" {
		t.Errorf("sortByPriority single = %v, want [a]", ids)
	}
}

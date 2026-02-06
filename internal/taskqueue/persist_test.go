package taskqueue

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveAndLoadState(t *testing.T) {
	q := NewFromPlan(makePlan())

	// Modify some state
	_, _ = q.ClaimNext("inst-1") // claims task-1
	_ = q.MarkRunning("task-1")
	_, _ = q.Complete("task-1")
	_, _ = q.ClaimNext("inst-2") // claims task-3

	dir := t.TempDir()
	if err := q.SaveState(dir); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Verify file exists
	statePath := filepath.Join(dir, stateFileName)
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not found: %v", err)
	}

	// Load it back
	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	// Verify task states
	if loaded.tasks["task-1"].Status != TaskCompleted {
		t.Errorf("task-1 status = %s, want completed", loaded.tasks["task-1"].Status)
	}
	if loaded.tasks["task-3"].Status != TaskClaimed {
		t.Errorf("task-3 status = %s, want claimed", loaded.tasks["task-3"].Status)
	}
	if loaded.tasks["task-3"].ClaimedBy != "inst-2" {
		t.Errorf("task-3 ClaimedBy = %q, want inst-2", loaded.tasks["task-3"].ClaimedBy)
	}
	if loaded.tasks["task-2"].Status != TaskPending {
		t.Errorf("task-2 status = %s, want pending", loaded.tasks["task-2"].Status)
	}

	// Verify order is preserved
	if len(loaded.order) != len(q.order) {
		t.Fatalf("order length = %d, want %d", len(loaded.order), len(q.order))
	}
	for i, id := range loaded.order {
		if id != q.order[i] {
			t.Errorf("order[%d] = %q, want %q", i, id, q.order[i])
		}
	}
}

func TestSaveState_AtomicWrite(t *testing.T) {
	q := NewFromPlan(makePlan())
	dir := t.TempDir()

	if err := q.SaveState(dir); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	// Temp file should not exist after save
	tmp := filepath.Join(dir, stateFileName+".tmp")
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Error("temp file should be removed after atomic rename")
	}
}

func TestSaveState_InvalidDirectory(t *testing.T) {
	q := NewFromPlan(makePlan())
	err := q.SaveState("/nonexistent/directory/path")
	if err == nil {
		t.Error("SaveState to nonexistent directory should fail")
	}
}

func TestLoadState_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadState(dir)
	if err == nil {
		t.Error("LoadState from empty directory should fail")
	}
}

func TestLoadState_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, stateFileName)
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadState(dir)
	if err == nil {
		t.Error("LoadState with invalid JSON should fail")
	}
}

func TestLoadState_EmptyState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, stateFileName)
	if err := os.WriteFile(path, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.tasks == nil {
		t.Error("tasks should be initialized, not nil")
	}
	if loaded.order == nil {
		t.Error("order should be initialized, not nil")
	}
}

func TestRoundTrip_PreservesTimestamps(t *testing.T) {
	q := NewFromPlan(makePlan())
	_, _ = q.ClaimNext("inst-1")
	_ = q.MarkRunning("task-1")
	_, _ = q.Complete("task-1")

	dir := t.TempDir()
	if err := q.SaveState(dir); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	original := q.tasks["task-1"]
	restored := loaded.tasks["task-1"]

	if original.ClaimedAt == nil || restored.ClaimedAt == nil {
		t.Fatal("ClaimedAt should be non-nil")
	}
	if !original.ClaimedAt.Round(time.Millisecond).Equal(restored.ClaimedAt.Round(time.Millisecond)) {
		t.Errorf("ClaimedAt mismatch: %v vs %v", original.ClaimedAt, restored.ClaimedAt)
	}

	if original.CompletedAt == nil || restored.CompletedAt == nil {
		t.Fatal("CompletedAt should be non-nil")
	}
	if !original.CompletedAt.Round(time.Millisecond).Equal(restored.CompletedAt.Round(time.Millisecond)) {
		t.Errorf("CompletedAt mismatch: %v vs %v", original.CompletedAt, restored.CompletedAt)
	}
}

func TestLoadedQueue_IsOperational(t *testing.T) {
	q := NewFromPlan(makePlan())
	_, _ = q.ClaimNext("inst-1") // task-1
	_ = q.MarkRunning("task-1")
	_, _ = q.Complete("task-1")

	dir := t.TempDir()
	_ = q.SaveState(dir)

	loaded, err := LoadState(dir)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}

	// Should be able to claim task-2 now (task-1 is completed)
	task, err := loaded.ClaimNext("inst-2")
	if err != nil {
		t.Fatalf("ClaimNext on loaded queue: %v", err)
	}
	if task == nil {
		t.Fatal("expected to claim task from loaded queue")
	}
	// task-2 depends on task-1 which is completed, so it's claimable
	// task-3 has no deps and is also claimable, but task-2 may come first by order
	// Both are valid
	if task.ID != "task-2" && task.ID != "task-3" {
		t.Errorf("claimed %q, want task-2 or task-3", task.ID)
	}
}

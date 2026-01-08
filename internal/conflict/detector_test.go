package conflict

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetector_NewAndStop(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	d.Start()
	time.Sleep(10 * time.Millisecond)
	d.Stop()
}

func TestDetector_AddInstance(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer d.Stop()

	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "conflict-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	d.Start()

	err = d.AddInstance("inst1", tmpDir)
	if err != nil {
		t.Fatalf("Failed to add instance: %v", err)
	}
}

func TestDetector_DetectsConflict(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer d.Stop()

	// Create two temp directories (simulating worktrees)
	tmpDir1, err := os.MkdirTemp("", "conflict-test-1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir1) }()

	tmpDir2, err := os.MkdirTemp("", "conflict-test-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir2) }()

	d.Start()

	// Add both instances
	err = d.AddInstance("inst1", tmpDir1)
	if err != nil {
		t.Fatalf("Failed to add instance 1: %v", err)
	}

	err = d.AddInstance("inst2", tmpDir2)
	if err != nil {
		t.Fatalf("Failed to add instance 2: %v", err)
	}

	// Initially no conflicts
	if d.HasConflicts() {
		t.Error("Expected no conflicts initially")
	}

	// Write the same relative file in both worktrees
	testFile := "test.txt"
	file1 := filepath.Join(tmpDir1, testFile)
	file2 := filepath.Join(tmpDir2, testFile)

	err = os.WriteFile(file1, []byte("content1"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file 1: %v", err)
	}

	// Wait for fsnotify event + debounce
	time.Sleep(200 * time.Millisecond)

	err = os.WriteFile(file2, []byte("content2"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file 2: %v", err)
	}

	// Wait for fsnotify event + debounce
	time.Sleep(200 * time.Millisecond)

	// Now there should be a conflict
	if !d.HasConflicts() {
		t.Error("Expected conflict after both instances modified same file")
	}

	conflicts := d.GetConflicts()
	if len(conflicts) != 1 {
		t.Errorf("Expected 1 conflict, got %d", len(conflicts))
	}

	if len(conflicts) > 0 {
		if conflicts[0].RelativePath != testFile {
			t.Errorf("Expected conflict on %s, got %s", testFile, conflicts[0].RelativePath)
		}
		if len(conflicts[0].Instances) != 2 {
			t.Errorf("Expected 2 instances in conflict, got %d", len(conflicts[0].Instances))
		}
	}
}

func TestDetector_GetFilesModifiedByInstance(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer d.Stop()

	tmpDir, err := os.MkdirTemp("", "conflict-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	d.Start()

	err = d.AddInstance("inst1", tmpDir)
	if err != nil {
		t.Fatalf("Failed to add instance: %v", err)
	}

	// Create a file
	testFile := filepath.Join(tmpDir, "modified.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	// Wait for fsnotify
	time.Sleep(200 * time.Millisecond)

	files := d.GetFilesModifiedByInstance("inst1")
	if len(files) != 1 {
		t.Errorf("Expected 1 file modified, got %d", len(files))
	}

	if len(files) > 0 && files[0] != "modified.txt" {
		t.Errorf("Expected modified.txt, got %s", files[0])
	}
}

func TestDetector_RemoveInstance(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer d.Stop()

	tmpDir1, err := os.MkdirTemp("", "conflict-test-1-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 1: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir1) }()

	tmpDir2, err := os.MkdirTemp("", "conflict-test-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir2) }()

	d.Start()

	_ = d.AddInstance("inst1", tmpDir1)
	_ = d.AddInstance("inst2", tmpDir2)

	// Create conflict
	testFile := "conflict.txt"
	_ = os.WriteFile(filepath.Join(tmpDir1, testFile), []byte("1"), 0644)
	time.Sleep(200 * time.Millisecond)
	_ = os.WriteFile(filepath.Join(tmpDir2, testFile), []byte("2"), 0644)
	time.Sleep(200 * time.Millisecond)

	if !d.HasConflicts() {
		t.Error("Expected conflict before removal")
	}

	// Remove one instance - conflict should be resolved
	d.RemoveInstance("inst2")

	if d.HasConflicts() {
		t.Error("Expected no conflict after removing instance")
	}
}

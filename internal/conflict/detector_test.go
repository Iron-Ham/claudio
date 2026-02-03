package conflict

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDetector_StopIsIdempotent(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}

	d.Start()
	time.Sleep(10 * time.Millisecond)

	// Calling Stop() multiple times should not panic
	d.Stop()
	d.Stop()
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

func TestDetector_AddInstance_NonExistentPath(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer d.Stop()

	d.Start()

	// Try to add an instance with a non-existent path
	nonExistentPath := filepath.Join(os.TempDir(), "this-path-does-not-exist-"+time.Now().Format("20060102150405"))
	err = d.AddInstance("inst1", nonExistentPath)
	if err == nil {
		t.Fatal("Expected error when adding instance with non-existent path")
	}

	// Verify the error message mentions the path doesn't exist
	expectedMsg := "worktree path does not exist"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Error message %q should contain %q", err.Error(), expectedMsg)
	}
}

func TestDetector_AddInstance_PathIsFile(t *testing.T) {
	d, err := New()
	if err != nil {
		t.Fatalf("Failed to create detector: %v", err)
	}
	defer d.Stop()

	// Create a temp file (not a directory)
	tmpFile, err := os.CreateTemp("", "conflict-test-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFilePath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpFilePath) }()

	d.Start()

	// Try to add an instance with a file path instead of a directory
	err = d.AddInstance("inst1", tmpFilePath)
	if err == nil {
		t.Fatal("Expected error when adding instance with file path instead of directory")
	}

	// Verify the error message mentions it's not a directory
	expectedMsg := "worktree path is not a directory"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Error message %q should contain %q", err.Error(), expectedMsg)
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

func TestIsInsideSubmodule(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "non-existent path returns false",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "nonexistent", "file.txt")
			},
			want: false,
		},
		{
			name: "normal git directory returns false",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitDir := filepath.Join(dir, ".git")
				if err := os.Mkdir(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git directory: %v", err)
				}
				return filepath.Join(dir, "file.txt")
			},
			want: false,
		},
		{
			name: "submodule directory returns true",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitFile := filepath.Join(dir, ".git")
				if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/main/.git/modules/sub"), 0644); err != nil {
					t.Fatalf("failed to create .git file: %v", err)
				}
				return filepath.Join(dir, "file.txt")
			},
			want: true,
		},
		{
			name: "nested file in submodule returns true",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitFile := filepath.Join(dir, ".git")
				if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/main/.git/modules/sub"), 0644); err != nil {
					t.Fatalf("failed to create .git file: %v", err)
				}
				nested := filepath.Join(dir, "deep", "nested")
				if err := os.MkdirAll(nested, 0755); err != nil {
					t.Fatalf("failed to create nested directory: %v", err)
				}
				return filepath.Join(nested, "file.txt")
			},
			want: true,
		},
		{
			name: "directory without .git returns false",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				return filepath.Join(dir, "file.txt")
			},
			want: false,
		},
		{
			name: ".git file without gitdir prefix returns false",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitFile := filepath.Join(dir, ".git")
				if err := os.WriteFile(gitFile, []byte("some other content"), 0644); err != nil {
					t.Fatalf("failed to create .git file: %v", err)
				}
				return filepath.Join(dir, "file.txt")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			got := isInsideSubmodule(path)
			if got != tt.want {
				t.Errorf("isInsideSubmodule(%q) = %v, want %v", path, got, tt.want)
			}
		})
	}
}

package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	tests := []struct {
		name       string
		srcContent []byte
		srcMode    os.FileMode
		wantErr    bool
	}{
		{
			name:       "copies regular file",
			srcContent: []byte("test content"),
			srcMode:    0644,
			wantErr:    false,
		},
		{
			name:       "copies empty file",
			srcContent: []byte{},
			srcMode:    0644,
			wantErr:    false,
		},
		{
			name:       "preserves restricted permissions",
			srcContent: []byte("secret"),
			srcMode:    0600,
			wantErr:    false,
		},
		{
			name:       "copies file with unicode content",
			srcContent: []byte("Unicode: éàü 中文 🎉"),
			srcMode:    0644,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "source.txt")
			dstPath := filepath.Join(tmpDir, "dest.txt")

			// Create source file
			if err := os.WriteFile(srcPath, tt.srcContent, tt.srcMode); err != nil {
				t.Fatalf("failed to create source file: %v", err)
			}

			// Copy file
			err := copyFile(srcPath, dstPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("copyFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify content
			dstContent, err := os.ReadFile(dstPath)
			if err != nil {
				t.Fatalf("failed to read destination file: %v", err)
			}
			if string(dstContent) != string(tt.srcContent) {
				t.Errorf("content mismatch: got %q, want %q", dstContent, tt.srcContent)
			}

			// Verify permissions
			srcInfo, _ := os.Stat(srcPath)
			dstInfo, _ := os.Stat(dstPath)
			if srcInfo.Mode() != dstInfo.Mode() {
				t.Errorf("permissions mismatch: got %v, want %v", dstInfo.Mode(), srcInfo.Mode())
			}
		})
	}
}

func TestCopyFile_SourceNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("copyFile() should error when source doesn't exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("copyFile() error should be os.ErrNotExist, got %v", err)
	}
}

func TestCopyFile_DestDirNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "nonexistent", "dest.txt")

	// Create source file
	if err := os.WriteFile(srcPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	err := copyFile(srcPath, dstPath)
	if err == nil {
		t.Error("copyFile() should error when destination directory doesn't exist")
	}
}

func TestCopyFile_OverwritesExisting(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	// Create source and existing destination
	if err := os.WriteFile(srcPath, []byte("new content"), 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if err := os.WriteFile(dstPath, []byte("old content"), 0644); err != nil {
		t.Fatalf("failed to create destination file: %v", err)
	}

	// Copy file
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify content was overwritten
	dstContent, _ := os.ReadFile(dstPath)
	if string(dstContent) != "new content" {
		t.Errorf("copyFile() didn't overwrite existing file: got %q, want %q", dstContent, "new content")
	}
}

func TestCopyFile_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "dest.txt")

	// Create a 1MB file
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcPath, largeContent, 0644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Copy file
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination file: %v", err)
	}
	if len(dstContent) != len(largeContent) {
		t.Errorf("large file size mismatch: got %d bytes, want %d bytes", len(dstContent), len(largeContent))
	}
}

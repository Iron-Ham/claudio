package internal

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGofmtCompliance verifies that all Go source files in the project
// are properly formatted according to gofmt standards.
//
// This test exists to catch formatting issues before code is committed.
// If this test fails, run: gofmt -w ./internal/ ./cmd/
func TestGofmtCompliance(t *testing.T) {
	// Get the project root (parent of internal/)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Navigate to project root from internal/
	projectRoot := filepath.Dir(wd)
	if filepath.Base(wd) != "internal" {
		// We might be running from project root
		projectRoot = wd
	}

	dirsToCheck := []string{
		filepath.Join(projectRoot, "internal"),
		filepath.Join(projectRoot, "cmd"),
	}

	var unformattedFiles []string

	for _, dir := range dirsToCheck {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories
			if info.IsDir() {
				// Skip vendor and hidden directories
				if info.Name() == "vendor" || strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}

			// Only check .go files
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			// Read the file
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Format the content
			formatted, err := format.Source(content)
			if err != nil {
				// Skip files that don't parse (might be generated or have build tags)
				return nil
			}

			// Compare
			if !bytes.Equal(content, formatted) {
				relPath, _ := filepath.Rel(projectRoot, path)
				unformattedFiles = append(unformattedFiles, relPath)
			}

			return nil
		})
		if err != nil {
			t.Fatalf("Failed to walk directory %s: %v", dir, err)
		}
	}

	if len(unformattedFiles) > 0 {
		t.Errorf("The following files are not properly formatted:\n")
		for _, f := range unformattedFiles {
			t.Errorf("  - %s\n", f)
		}
		t.Errorf("\nRun 'gofmt -w ./internal/ ./cmd/' to fix formatting issues.")
	}
}

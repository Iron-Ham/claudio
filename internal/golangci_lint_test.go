package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestGolangciLintCompliance verifies that all Go source files in the project
// pass golangci-lint checks.
//
// This test exists to catch linting issues before code is committed.
// If this test fails, run: golangci-lint run
//
// This test is skipped if golangci-lint is not installed.
func TestGolangciLintCompliance(t *testing.T) {
	// Skip if golangci-lint is not installed
	if _, err := exec.LookPath("golangci-lint"); err != nil {
		t.Skip("golangci-lint not found in PATH, skipping test")
	}

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

	// Use a per-test Go build cache directory to ensure the cache is writable in
	// restricted environments (e.g., sandboxed runners).
	goCacheDir := t.TempDir()

	// Run golangci-lint from project root
	cmd := exec.Command("golangci-lint", "run", "--allow-parallel-runners", "./...")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "GOCACHE="+goCacheDir)
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("golangci-lint found issues:\n%s", output)
		t.Errorf("\nRun 'golangci-lint run' to see all issues and fix them.")
	}
}

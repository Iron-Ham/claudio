package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

func TestRunThemeList(t *testing.T) {
	// Clear custom themes for consistent test
	styles.ClearCustomThemes()
	defer styles.ClearCustomThemes()

	// Create a temp themes directory
	tmpDir := t.TempDir()
	origThemesDirFn := styles.SetThemesDirFunc(func() string { return tmpDir })
	defer styles.SetThemesDirFunc(origThemesDirFn)

	// Add a custom theme
	customTheme := `name: "Test Theme"
version: "1"
colors:
  primary: "#A78BFA"
  secondary: "#10B981"
  warning: "#F59E0B"
  error: "#F87171"
  muted: "#9CA3AF"
  surface: "#1F2937"
  text: "#F9FAFB"
  border: "#6B7280"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "testtheme.yaml"), []byte(customTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test theme: %v", err)
	}

	// Run the command
	err := runThemeList(themeListCmd, []string{})
	if err != nil {
		t.Errorf("runThemeList() error = %v", err)
	}
}

func TestRunThemeExport(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "exported.yaml")

	// Export to file
	err := runThemeExport(themeExportCmd, []string{"default", outputPath})
	if err != nil {
		t.Fatalf("runThemeExport() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Read and verify content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if len(data) == 0 {
		t.Error("Output file is empty")
	}

	// Verify it contains expected content
	if !bytes.Contains(data, []byte("primary:")) {
		t.Error("Output file missing primary color")
	}
}

func TestRunThemeExportInvalidTheme(t *testing.T) {
	styles.ClearCustomThemes()
	defer styles.ClearCustomThemes()

	err := runThemeExport(themeExportCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for invalid theme, got nil")
	}
}

func TestRunThemeInfo(t *testing.T) {
	err := runThemeInfo(themeInfoCmd, []string{"default"})
	if err != nil {
		t.Errorf("runThemeInfo() error = %v", err)
	}
}

func TestRunThemeInfoInvalidTheme(t *testing.T) {
	styles.ClearCustomThemes()
	defer styles.ClearCustomThemes()

	err := runThemeInfo(themeInfoCmd, []string{"nonexistent"})
	if err == nil {
		t.Error("Expected error for invalid theme, got nil")
	}
}

func TestRunThemePath(t *testing.T) {
	err := runThemePath(themePathCmd, []string{})
	if err != nil {
		t.Errorf("runThemePath() error = %v", err)
	}
}

func TestRunThemeCreate(t *testing.T) {
	tmpDir := t.TempDir()
	origThemesDirFn := styles.SetThemesDirFunc(func() string { return tmpDir })
	defer styles.SetThemesDirFunc(origThemesDirFn)

	err := runThemeCreate(themeCreateCmd, []string{"newtheme"})
	if err != nil {
		t.Fatalf("runThemeCreate() error = %v", err)
	}

	// Verify file was created
	themePath := filepath.Join(tmpDir, "newtheme.yaml")
	if _, err := os.Stat(themePath); os.IsNotExist(err) {
		t.Error("Theme file was not created")
	}

	// Verify it's a valid theme
	_, err = styles.LoadThemeFile(themePath)
	if err != nil {
		t.Errorf("Created theme is invalid: %v", err)
	}
}

func TestRunThemeCreateBuiltinName(t *testing.T) {
	tmpDir := t.TempDir()
	origThemesDirFn := styles.SetThemesDirFunc(func() string { return tmpDir })
	defer styles.SetThemesDirFunc(origThemesDirFn)

	err := runThemeCreate(themeCreateCmd, []string{"default"})
	if err == nil {
		t.Error("Expected error when creating theme with built-in name")
	}
}

func TestRunThemeCreateInvalidName(t *testing.T) {
	tmpDir := t.TempDir()
	origThemesDirFn := styles.SetThemesDirFunc(func() string { return tmpDir })
	defer styles.SetThemesDirFunc(origThemesDirFn)

	tests := []struct {
		name    string
		errText string
	}{
		{"", "empty"},
		{"my/theme", "invalid characters"},
		{"my\\theme", "invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runThemeCreate(themeCreateCmd, []string{tt.name})
			if err == nil {
				t.Error("Expected error for invalid name")
			}
		})
	}
}

func TestRunThemeCreateAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	origThemesDirFn := styles.SetThemesDirFunc(func() string { return tmpDir })
	defer styles.SetThemesDirFunc(origThemesDirFn)

	// Create the theme first
	err := runThemeCreate(themeCreateCmd, []string{"existing"})
	if err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Try to create again
	err = runThemeCreate(themeCreateCmd, []string{"existing"})
	if err == nil {
		t.Error("Expected error when theme already exists")
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "Hello"},
		{"HELLO", "HELLO"},
		{"Hello", "Hello"},
		{"h", "H"},
		{"", ""},
		{"myTheme", "MyTheme"},
		{"solarized", "Solarized"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			if got != tt.expected {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

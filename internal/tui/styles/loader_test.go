package styles

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIsValidHexColor(t *testing.T) {
	tests := []struct {
		name     string
		color    string
		expected bool
	}{
		{"valid 6-digit hex", "#A78BFA", true},
		{"valid 6-digit hex lowercase", "#a78bfa", true},
		{"valid 3-digit hex", "#ABC", true},
		{"valid 3-digit hex lowercase", "#abc", true},
		{"invalid - no hash", "A78BFA", false},
		{"invalid - too short", "#AB", false},
		{"invalid - too long", "#A78BFAAB", false},
		{"invalid - 4 digits", "#ABCD", false},
		{"invalid - 5 digits", "#ABCDE", false},
		{"invalid - bad characters", "#GHIJKL", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidHexColor(tt.color)
			if got != tt.expected {
				t.Errorf("isValidHexColor(%q) = %v, want %v", tt.color, got, tt.expected)
			}
		})
	}
}

func TestThemeFileValidate(t *testing.T) {
	tests := []struct {
		name      string
		theme     ThemeFile
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid minimal theme",
			theme: ThemeFile{
				Name:    "Test Theme",
				Version: "1",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
				},
			},
			expectErr: false,
		},
		{
			name: "valid theme with all colors",
			theme: ThemeFile{
				Name:        "Full Theme",
				Author:      "Test Author",
				Description: "A test theme",
				Version:     "1",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
					Status: ThemeStatusColors{
						Working:     "#10B981",
						Pending:     "#9CA3AF",
						Input:       "#F59E0B",
						Paused:      "#60A5FA",
						Complete:    "#A78BFA",
						Error:       "#F87171",
						CreatingPR:  "#F472B6",
						Stuck:       "#FB923C",
						Timeout:     "#F87171",
						Interrupted: "#FBBF24",
					},
					Diff: ThemeDiffColors{
						Add:     "#22C55E",
						Remove:  "#F87171",
						Header:  "#60A5FA",
						Hunk:    "#A78BFA",
						Context: "#9CA3AF",
					},
					Search: ThemeSearchColors{
						MatchBg:   "#854D0E",
						MatchFg:   "#FEF3C7",
						CurrentBg: "#C2410C",
						CurrentFg: "#FFF7ED",
					},
					Accents: ThemeAccentColors{
						Blue:   "#60A5FA",
						Yellow: "#FBBF24",
						Purple: "#A78BFA",
						Pink:   "#F472B6",
						Orange: "#FB923C",
					},
				},
			},
			expectErr: false,
		},
		{
			name: "missing name",
			theme: ThemeFile{
				Version: "1",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
				},
			},
			expectErr: true,
			errMsg:    "theme name is required",
		},
		{
			name: "missing version",
			theme: ThemeFile{
				Name: "Test Theme",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
				},
			},
			expectErr: true,
			errMsg:    "theme version is required",
		},
		{
			name: "unsupported version",
			theme: ThemeFile{
				Name:    "Test Theme",
				Version: "2",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
				},
			},
			expectErr: true,
			errMsg:    "unsupported theme version",
		},
		{
			name: "missing required color",
			theme: ThemeFile{
				Name:    "Test Theme",
				Version: "1",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					// Border is missing
				},
			},
			expectErr: true,
			errMsg:    "color 'border' is required",
		},
		{
			name: "invalid hex color format",
			theme: ThemeFile{
				Name:    "Test Theme",
				Version: "1",
				Colors: ThemeColors{
					Primary:   "invalid",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
				},
			},
			expectErr: true,
			errMsg:    "invalid format",
		},
		{
			name: "invalid optional color format",
			theme: ThemeFile{
				Name:    "Test Theme",
				Version: "1",
				Colors: ThemeColors{
					Primary:   "#A78BFA",
					Secondary: "#10B981",
					Warning:   "#F59E0B",
					Error:     "#F87171",
					Muted:     "#9CA3AF",
					Surface:   "#1F2937",
					Text:      "#F9FAFB",
					Border:    "#6B7280",
					Status: ThemeStatusColors{
						Working: "bad-color",
					},
				},
			},
			expectErr: true,
			errMsg:    "status.working",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.theme.Validate()
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestThemeFileToPalette(t *testing.T) {
	theme := ThemeFile{
		Name:    "Test Theme",
		Version: "1",
		Colors: ThemeColors{
			Primary:   "#A78BFA",
			Secondary: "#10B981",
			Warning:   "#F59E0B",
			Error:     "#F87171",
			Muted:     "#9CA3AF",
			Surface:   "#1F2937",
			Text:      "#F9FAFB",
			Border:    "#6B7280",
		},
	}

	palette := theme.ToPalette()

	// Check base colors
	if string(palette.Primary) != "#A78BFA" {
		t.Errorf("Primary = %v, want %v", palette.Primary, "#A78BFA")
	}
	if string(palette.Secondary) != "#10B981" {
		t.Errorf("Secondary = %v, want %v", palette.Secondary, "#10B981")
	}

	// Check that status colors default to base colors
	if string(palette.StatusWorking) != "#10B981" {
		t.Errorf("StatusWorking should default to Secondary, got %v", palette.StatusWorking)
	}
	if string(palette.StatusError) != "#F87171" {
		t.Errorf("StatusError should default to Error, got %v", palette.StatusError)
	}
}

func TestThemeFileToPaletteWithOverrides(t *testing.T) {
	theme := ThemeFile{
		Name:    "Test Theme",
		Version: "1",
		Colors: ThemeColors{
			Primary:   "#A78BFA",
			Secondary: "#10B981",
			Warning:   "#F59E0B",
			Error:     "#F87171",
			Muted:     "#9CA3AF",
			Surface:   "#1F2937",
			Text:      "#F9FAFB",
			Border:    "#6B7280",
			Status: ThemeStatusColors{
				Working: "#00FF00",
			},
		},
	}

	palette := theme.ToPalette()

	// Check overridden status color
	if string(palette.StatusWorking) != "#00FF00" {
		t.Errorf("StatusWorking = %v, want %v", palette.StatusWorking, "#00FF00")
	}
	// Check non-overridden defaults to base
	if string(palette.StatusError) != "#F87171" {
		t.Errorf("StatusError should default to Error, got %v", palette.StatusError)
	}
}

func TestLoadThemeFile(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Write a valid theme file
	validTheme := `name: "Test Theme"
author: "Test Author"
description: "A test theme"
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
	validPath := filepath.Join(tmpDir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte(validTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test loading valid theme
	theme, err := LoadThemeFile(validPath)
	if err != nil {
		t.Errorf("Failed to load valid theme: %v", err)
	}
	if theme.Name != "Test Theme" {
		t.Errorf("Name = %v, want %v", theme.Name, "Test Theme")
	}
	if theme.Author != "Test Author" {
		t.Errorf("Author = %v, want %v", theme.Author, "Test Author")
	}

	// Write an invalid theme file
	invalidTheme := `name: "Test Theme"
version: "1"
colors:
  primary: "not-a-color"
`
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte(invalidTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Test loading invalid theme
	_, err = LoadThemeFile(invalidPath)
	if err == nil {
		t.Error("Expected error loading invalid theme, got nil")
	}

	// Test loading non-existent file
	_, err = LoadThemeFile(filepath.Join(tmpDir, "nonexistent.yaml"))
	if err == nil {
		t.Error("Expected error loading non-existent file, got nil")
	}
}

func TestCustomThemeRegistry(t *testing.T) {
	// Clear any existing custom themes
	ClearCustomThemes()
	defer ClearCustomThemes()

	theme := &ThemeFile{
		Name:    "Custom Theme",
		Version: "1",
		Colors: ThemeColors{
			Primary:   "#FF0000",
			Secondary: "#00FF00",
			Warning:   "#FFFF00",
			Error:     "#FF0000",
			Muted:     "#808080",
			Surface:   "#000000",
			Text:      "#FFFFFF",
			Border:    "#404040",
		},
	}

	// Register custom theme
	RegisterCustomTheme("custom", theme)

	// Check it was registered
	if !IsCustomTheme("custom") {
		t.Error("Expected IsCustomTheme('custom') to be true")
	}
	if IsCustomTheme("nonexistent") {
		t.Error("Expected IsCustomTheme('nonexistent') to be false")
	}

	// Get custom theme
	got := GetCustomTheme("custom")
	if got == nil {
		t.Fatal("GetCustomTheme returned nil")
	}
	if got.Name != "Custom Theme" {
		t.Errorf("Name = %v, want %v", got.Name, "Custom Theme")
	}

	// Check theme names
	names := CustomThemeNames()
	if !slices.Contains(names, "custom") {
		t.Errorf("CustomThemeNames() did not include 'custom': %v", names)
	}

	// Clear themes
	ClearCustomThemes()
	if IsCustomTheme("custom") {
		t.Error("Expected custom theme to be cleared")
	}
}

func TestIsBuiltinTheme(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		// Original 4 themes
		{"default", true},
		{"monokai", true},
		{"dracula", true},
		{"nord", true},
		// New 10 themes added in PR #581
		{"claude-code", true},
		{"solarized-dark", true},
		{"solarized-light", true},
		{"one-dark", true},
		{"github-dark", true},
		{"gruvbox", true},
		{"tokyo-night", true},
		{"catppuccin", true},
		{"synthwave", true},
		{"ayu", true},
		// Non-built-in themes
		{"custom", false},
		{"my-custom-theme", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBuiltinTheme(tt.name)
			if got != tt.expected {
				t.Errorf("IsBuiltinTheme(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestDiscoverCustomThemes(t *testing.T) {
	// Create a temp directory to act as config dir
	tmpDir := t.TempDir()

	// Override ThemesDir for testing
	origThemesDirFn := SetThemesDirFunc(func() string { return tmpDir })
	defer SetThemesDirFunc(origThemesDirFn)

	// Clear any existing custom themes
	ClearCustomThemes()
	defer ClearCustomThemes()

	// Create a valid theme file
	validTheme := `name: "Solarized"
version: "1"
colors:
  primary: "#268BD2"
  secondary: "#859900"
  warning: "#B58900"
  error: "#DC322F"
  muted: "#586E75"
  surface: "#002B36"
  text: "#FDF6E3"
  border: "#073642"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "solarized.yaml"), []byte(validTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create an invalid theme file
	invalidTheme := `name: "Bad Theme"
version: "1"
colors:
  primary: "not-valid"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "invalid.yaml"), []byte(invalidTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create a non-yaml file (should be ignored)
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Discover themes
	loaded, errs := DiscoverCustomThemes()

	// Check that valid theme was loaded
	if !slices.Contains(loaded, "solarized") {
		t.Errorf("Expected 'solarized' to be loaded, got: %v", loaded)
	}

	// Check that invalid theme produced an error
	if len(errs) == 0 {
		t.Error("Expected errors from invalid theme file")
	}

	// Check theme is registered
	if !IsCustomTheme("solarized") {
		t.Error("Expected 'solarized' to be registered as custom theme")
	}

	// Verify we can use the theme
	if !IsValidTheme("solarized") {
		t.Error("Expected 'solarized' to be valid after discovery")
	}
}

func TestDiscoverCustomThemesCannotOverrideBuiltin(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Override ThemesDir for testing
	origThemesDirFn := SetThemesDirFunc(func() string { return tmpDir })
	defer SetThemesDirFunc(origThemesDirFn)

	ClearCustomThemes()
	defer ClearCustomThemes()

	// Try to create a theme with a built-in name
	builtinTheme := `name: "Default Override"
version: "1"
colors:
  primary: "#FF0000"
  secondary: "#00FF00"
  warning: "#FFFF00"
  error: "#FF0000"
  muted: "#808080"
  surface: "#000000"
  text: "#FFFFFF"
  border: "#404040"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "default.yaml"), []byte(builtinTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Discover themes - should produce error
	_, errs := DiscoverCustomThemes()

	if len(errs) == 0 {
		t.Error("Expected error when trying to override built-in theme")
	}

	// Verify built-in theme is still intact
	palette := GetPalette(ThemeDefault)
	if string(palette.Primary) == "#FF0000" {
		t.Error("Built-in theme was overridden!")
	}
}

func TestExportTheme(t *testing.T) {
	ClearCustomThemes()
	defer ClearCustomThemes()

	// Test exporting a built-in theme
	data, err := ExportTheme(ThemeDefault)
	if err != nil {
		t.Fatalf("Failed to export theme: %v", err)
	}

	// Parse the exported data
	var exported ThemeFile
	if err := parseYAML(data, &exported); err != nil {
		t.Fatalf("Failed to parse exported theme: %v", err)
	}

	if exported.Name != "default" {
		t.Errorf("Exported name = %v, want 'default'", exported.Name)
	}
	if exported.Version != "1" {
		t.Errorf("Exported version = %v, want '1'", exported.Version)
	}
	if exported.Colors.Primary != "#A78BFA" {
		t.Errorf("Exported primary color = %v, want '#A78BFA'", exported.Colors.Primary)
	}

	// Test exporting a custom theme
	customTheme := &ThemeFile{
		Name:        "My Custom Theme",
		Author:      "Me",
		Description: "My theme",
		Version:     "1",
		Colors: ThemeColors{
			Primary:   "#123456",
			Secondary: "#654321",
			Warning:   "#AABBCC",
			Error:     "#CCBBAA",
			Muted:     "#808080",
			Surface:   "#000000",
			Text:      "#FFFFFF",
			Border:    "#404040",
		},
	}
	RegisterCustomTheme("mycustom", customTheme)

	data, err = ExportTheme("mycustom")
	if err != nil {
		t.Fatalf("Failed to export custom theme: %v", err)
	}

	var exportedCustom ThemeFile
	if err := parseYAML(data, &exportedCustom); err != nil {
		t.Fatalf("Failed to parse exported custom theme: %v", err)
	}

	if exportedCustom.Name != "My Custom Theme" {
		t.Errorf("Exported custom name = %v, want 'My Custom Theme'", exportedCustom.Name)
	}
	if exportedCustom.Author != "Me" {
		t.Errorf("Exported author = %v, want 'Me'", exportedCustom.Author)
	}
}

func TestSaveTheme(t *testing.T) {
	tmpDir := t.TempDir()

	// Override ThemesDir for testing
	origThemesDirFn := SetThemesDirFunc(func() string { return tmpDir })
	defer SetThemesDirFunc(origThemesDirFn)

	theme := &ThemeFile{
		Name:        "My Theme",
		Author:      "Test Author",
		Description: "A test theme",
		Version:     "1",
		Colors: ThemeColors{
			Primary:   "#A78BFA",
			Secondary: "#10B981",
			Warning:   "#F59E0B",
			Error:     "#F87171",
			Muted:     "#9CA3AF",
			Surface:   "#1F2937",
			Text:      "#F9FAFB",
			Border:    "#6B7280",
		},
	}

	err := SaveTheme("mytheme", theme)
	if err != nil {
		t.Fatalf("Failed to save theme: %v", err)
	}

	// Verify file was created
	path := filepath.Join(tmpDir, "mytheme.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Theme file was not created")
	}

	// Load and verify contents
	loaded, err := LoadThemeFile(path)
	if err != nil {
		t.Fatalf("Failed to load saved theme: %v", err)
	}

	if loaded.Name != "My Theme" {
		t.Errorf("Loaded name = %v, want 'My Theme'", loaded.Name)
	}
	if loaded.Author != "Test Author" {
		t.Errorf("Loaded author = %v, want 'Test Author'", loaded.Author)
	}
}

// parseYAML is a test helper to parse YAML
func parseYAML(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

func TestLoadThemeMalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a malformed YAML file (missing colon)
	malformedYAML := `name: "Test Theme"
version: "1"
colors:
  primary "#MISSING_COLON"
  secondary: #10B981
`
	path := filepath.Join(tmpDir, "malformed.yaml")
	if err := os.WriteFile(path, []byte(malformedYAML), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := LoadThemeFile(path)
	if err == nil {
		t.Error("Expected error for malformed YAML, got nil")
	}
	// Error should mention parsing
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("Error message should mention parsing, got: %v", err)
	}
}

func TestDiscoverCustomThemesYMLExtension(t *testing.T) {
	tmpDir := t.TempDir()

	origThemesDirFn := SetThemesDirFunc(func() string { return tmpDir })
	defer SetThemesDirFunc(origThemesDirFn)

	ClearCustomThemes()
	defer ClearCustomThemes()

	// Create a theme with .yml extension
	ymlTheme := `name: "YML Theme"
version: "1"
colors:
  primary: "#ABCDEF"
  secondary: "#123456"
  warning: "#F59E0B"
  error: "#F87171"
  muted: "#9CA3AF"
  surface: "#1F2937"
  text: "#F9FAFB"
  border: "#6B7280"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "alternate.yml"), []byte(ymlTheme), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	loaded, _ := DiscoverCustomThemes()

	if !slices.Contains(loaded, "alternate") {
		t.Errorf("Expected 'alternate' (.yml) to be loaded, got: %v", loaded)
	}
}

func TestGetPaletteCustomTheme(t *testing.T) {
	ClearCustomThemes()
	defer ClearCustomThemes()

	customTheme := &ThemeFile{
		Name:    "My Custom",
		Version: "1",
		Colors: ThemeColors{
			Primary:   "#FF0000",
			Secondary: "#00FF00",
			Warning:   "#FFFF00",
			Error:     "#FF0000",
			Muted:     "#808080",
			Surface:   "#000000",
			Text:      "#FFFFFF",
			Border:    "#404040",
		},
	}
	RegisterCustomTheme("mycustom", customTheme)

	p := GetPalette("mycustom")
	if string(p.Primary) != "#FF0000" {
		t.Errorf("GetPalette for custom theme returned wrong primary: %s, want #FF0000", p.Primary)
	}
	if string(p.Secondary) != "#00FF00" {
		t.Errorf("GetPalette for custom theme returned wrong secondary: %s, want #00FF00", p.Secondary)
	}
}

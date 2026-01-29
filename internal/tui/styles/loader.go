package styles

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"gopkg.in/yaml.v3"
)

// ThemeFile represents a custom theme definition loaded from YAML.
type ThemeFile struct {
	// Name is the theme's display name (e.g., "Solarized Dark")
	Name string `yaml:"name"`
	// Author is the theme creator's name (optional)
	Author string `yaml:"author,omitempty"`
	// Description provides details about the theme (optional)
	Description string `yaml:"description,omitempty"`
	// Version is the theme file format version (currently "1")
	Version string `yaml:"version"`
	// Colors defines the color palette
	Colors ThemeColors `yaml:"colors"`
}

// ThemeColors contains all color definitions for a theme.
// All colors should be hex format (#RRGGBB or #RGB).
type ThemeColors struct {
	// Base colors
	Primary   string `yaml:"primary"`
	Secondary string `yaml:"secondary"`
	Warning   string `yaml:"warning"`
	Error     string `yaml:"error"`
	Muted     string `yaml:"muted"`
	Surface   string `yaml:"surface"`
	Text      string `yaml:"text"`
	Border    string `yaml:"border"`

	// Status colors (optional - defaults to base colors if not specified)
	Status ThemeStatusColors `yaml:"status,omitempty"`

	// Diff colors (optional - defaults to sensible values if not specified)
	Diff ThemeDiffColors `yaml:"diff,omitempty"`

	// Search colors (optional - defaults to sensible values if not specified)
	Search ThemeSearchColors `yaml:"search,omitempty"`

	// Accent colors (optional - defaults to base colors if not specified)
	Accents ThemeAccentColors `yaml:"accents,omitempty"`
}

// ThemeStatusColors defines colors for different instance statuses.
type ThemeStatusColors struct {
	Working     string `yaml:"working,omitempty"`
	Pending     string `yaml:"pending,omitempty"`
	Preparing   string `yaml:"preparing,omitempty"`
	Input       string `yaml:"input,omitempty"`
	Paused      string `yaml:"paused,omitempty"`
	Complete    string `yaml:"complete,omitempty"`
	Error       string `yaml:"error,omitempty"`
	CreatingPR  string `yaml:"creating_pr,omitempty"`
	Stuck       string `yaml:"stuck,omitempty"`
	Timeout     string `yaml:"timeout,omitempty"`
	Interrupted string `yaml:"interrupted,omitempty"`
}

// ThemeDiffColors defines colors for diff highlighting.
type ThemeDiffColors struct {
	Add     string `yaml:"add,omitempty"`
	Remove  string `yaml:"remove,omitempty"`
	Header  string `yaml:"header,omitempty"`
	Hunk    string `yaml:"hunk,omitempty"`
	Context string `yaml:"context,omitempty"`
}

// ThemeSearchColors defines colors for search highlighting.
type ThemeSearchColors struct {
	MatchBg   string `yaml:"match_bg,omitempty"`
	MatchFg   string `yaml:"match_fg,omitempty"`
	CurrentBg string `yaml:"current_bg,omitempty"`
	CurrentFg string `yaml:"current_fg,omitempty"`
}

// ThemeAccentColors defines additional accent colors.
type ThemeAccentColors struct {
	Blue   string `yaml:"blue,omitempty"`
	Yellow string `yaml:"yellow,omitempty"`
	Purple string `yaml:"purple,omitempty"`
	Pink   string `yaml:"pink,omitempty"`
	Orange string `yaml:"orange,omitempty"`
}

// hexColorRegex validates hex color format.
var hexColorRegex = regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)

// LoadThemeFile loads a theme from a YAML file.
func LoadThemeFile(path string) (*ThemeFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading theme file: %w", err)
	}

	var theme ThemeFile
	if err := yaml.Unmarshal(data, &theme); err != nil {
		return nil, fmt.Errorf("parsing theme file: %w", err)
	}

	if err := theme.Validate(); err != nil {
		return nil, fmt.Errorf("invalid theme: %w", err)
	}

	return &theme, nil
}

// Validate checks that the theme file is well-formed.
func (t *ThemeFile) Validate() error {
	if t.Name == "" {
		return errors.New("theme name is required")
	}

	if t.Version == "" {
		return errors.New("theme version is required")
	}

	if t.Version != "1" {
		return fmt.Errorf("unsupported theme version: %s (supported: 1)", t.Version)
	}

	// Validate required base colors
	requiredColors := map[string]string{
		"primary":   t.Colors.Primary,
		"secondary": t.Colors.Secondary,
		"warning":   t.Colors.Warning,
		"error":     t.Colors.Error,
		"muted":     t.Colors.Muted,
		"surface":   t.Colors.Surface,
		"text":      t.Colors.Text,
		"border":    t.Colors.Border,
	}

	for name, color := range requiredColors {
		if color == "" {
			return fmt.Errorf("color '%s' is required", name)
		}
		if !isValidHexColor(color) {
			return fmt.Errorf("color '%s' has invalid format: %s (expected #RGB or #RRGGBB)", name, color)
		}
	}

	// Validate optional colors if provided
	optionalColors := map[string]string{
		"status.working":     t.Colors.Status.Working,
		"status.pending":     t.Colors.Status.Pending,
		"status.input":       t.Colors.Status.Input,
		"status.paused":      t.Colors.Status.Paused,
		"status.complete":    t.Colors.Status.Complete,
		"status.error":       t.Colors.Status.Error,
		"status.creating_pr": t.Colors.Status.CreatingPR,
		"status.stuck":       t.Colors.Status.Stuck,
		"status.timeout":     t.Colors.Status.Timeout,
		"status.interrupted": t.Colors.Status.Interrupted,
		"diff.add":           t.Colors.Diff.Add,
		"diff.remove":        t.Colors.Diff.Remove,
		"diff.header":        t.Colors.Diff.Header,
		"diff.hunk":          t.Colors.Diff.Hunk,
		"diff.context":       t.Colors.Diff.Context,
		"search.match_bg":    t.Colors.Search.MatchBg,
		"search.match_fg":    t.Colors.Search.MatchFg,
		"search.current_bg":  t.Colors.Search.CurrentBg,
		"search.current_fg":  t.Colors.Search.CurrentFg,
		"accents.blue":       t.Colors.Accents.Blue,
		"accents.yellow":     t.Colors.Accents.Yellow,
		"accents.purple":     t.Colors.Accents.Purple,
		"accents.pink":       t.Colors.Accents.Pink,
		"accents.orange":     t.Colors.Accents.Orange,
	}

	for name, color := range optionalColors {
		if color != "" && !isValidHexColor(color) {
			return fmt.Errorf("color '%s' has invalid format: %s (expected #RGB or #RRGGBB)", name, color)
		}
	}

	return nil
}

// isValidHexColor checks if a string is a valid hex color.
func isValidHexColor(color string) bool {
	return hexColorRegex.MatchString(color)
}

// ToPalette converts the theme file to a ColorPalette.
func (t *ThemeFile) ToPalette() *ColorPalette {
	// Start with base colors
	p := &ColorPalette{
		Primary:   lipgloss.Color(t.Colors.Primary),
		Secondary: lipgloss.Color(t.Colors.Secondary),
		Warning:   lipgloss.Color(t.Colors.Warning),
		Error:     lipgloss.Color(t.Colors.Error),
		Muted:     lipgloss.Color(t.Colors.Muted),
		Surface:   lipgloss.Color(t.Colors.Surface),
		Text:      lipgloss.Color(t.Colors.Text),
		Border:    lipgloss.Color(t.Colors.Border),
	}

	// Apply status colors (with defaults)
	p.StatusWorking = colorOrDefault(t.Colors.Status.Working, t.Colors.Secondary)
	p.StatusPending = colorOrDefault(t.Colors.Status.Pending, t.Colors.Muted)
	p.StatusPreparing = colorOrDefault(t.Colors.Status.Preparing, t.Colors.Primary)
	p.StatusInput = colorOrDefault(t.Colors.Status.Input, t.Colors.Warning)
	p.StatusPaused = colorOrDefault(t.Colors.Status.Paused, t.Colors.Primary)
	p.StatusComplete = colorOrDefault(t.Colors.Status.Complete, t.Colors.Primary)
	p.StatusError = colorOrDefault(t.Colors.Status.Error, t.Colors.Error)
	p.StatusCreatingPR = colorOrDefault(t.Colors.Status.CreatingPR, t.Colors.Primary)
	p.StatusStuck = colorOrDefault(t.Colors.Status.Stuck, t.Colors.Warning)
	p.StatusTimeout = colorOrDefault(t.Colors.Status.Timeout, t.Colors.Error)
	p.StatusInterrupted = colorOrDefault(t.Colors.Status.Interrupted, t.Colors.Warning)

	// Apply diff colors (with defaults)
	p.DiffAdd = colorOrDefault(t.Colors.Diff.Add, t.Colors.Secondary)
	p.DiffRemove = colorOrDefault(t.Colors.Diff.Remove, t.Colors.Error)
	p.DiffHeader = colorOrDefault(t.Colors.Diff.Header, t.Colors.Primary)
	p.DiffHunk = colorOrDefault(t.Colors.Diff.Hunk, t.Colors.Primary)
	p.DiffContext = colorOrDefault(t.Colors.Diff.Context, t.Colors.Muted)

	// Apply search colors (with defaults)
	p.SearchMatchBg = colorOrDefault(t.Colors.Search.MatchBg, t.Colors.Surface)
	p.SearchMatchFg = colorOrDefault(t.Colors.Search.MatchFg, t.Colors.Warning)
	p.SearchCurrentBg = colorOrDefault(t.Colors.Search.CurrentBg, t.Colors.Primary)
	p.SearchCurrentFg = colorOrDefault(t.Colors.Search.CurrentFg, t.Colors.Text)

	// Apply accent colors (with defaults)
	p.Blue = colorOrDefault(t.Colors.Accents.Blue, t.Colors.Primary)
	p.Yellow = colorOrDefault(t.Colors.Accents.Yellow, t.Colors.Warning)
	p.Purple = colorOrDefault(t.Colors.Accents.Purple, t.Colors.Primary)
	p.Pink = colorOrDefault(t.Colors.Accents.Pink, t.Colors.Primary)
	p.Orange = colorOrDefault(t.Colors.Accents.Orange, t.Colors.Warning)

	return p
}

// colorOrDefault returns the color if non-empty, otherwise returns the default.
func colorOrDefault(color, defaultColor string) lipgloss.Color {
	if color != "" {
		return lipgloss.Color(color)
	}
	return lipgloss.Color(defaultColor)
}

// customThemes stores loaded custom themes.
var customThemes = make(map[ThemeName]*ThemeFile)

// RegisterCustomTheme registers a custom theme by name.
func RegisterCustomTheme(name ThemeName, theme *ThemeFile) {
	customThemes[name] = theme
}

// GetCustomTheme returns a custom theme by name, or nil if not found.
func GetCustomTheme(name ThemeName) *ThemeFile {
	return customThemes[name]
}

// CustomThemeNames returns the names of all registered custom themes.
func CustomThemeNames() []string {
	names := make([]string, 0, len(customThemes))
	for name := range customThemes {
		names = append(names, string(name))
	}
	return names
}

// ClearCustomThemes removes all registered custom themes.
// Primarily used for testing.
func ClearCustomThemes() {
	customThemes = make(map[ThemeName]*ThemeFile)
}

// themesDirFn is the function that returns the themes directory.
// This can be overridden in tests.
var themesDirFn = defaultThemesDir

// defaultThemesDir returns the default themes directory path.
func defaultThemesDir() string {
	// Check XDG_CONFIG_HOME first
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "claudio", "themes")
	}
	// Fall back to ~/.config/claudio/themes
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".claudio", "themes")
	}
	return filepath.Join(home, ".config", "claudio", "themes")
}

// ThemesDir returns the directory where custom themes are stored.
func ThemesDir() string {
	return themesDirFn()
}

// SetThemesDirFunc sets the function used to determine the themes directory.
// This is primarily useful for testing. Returns the previous function.
func SetThemesDirFunc(fn func() string) func() string {
	prev := themesDirFn
	themesDirFn = fn
	return prev
}

// DiscoverCustomThemes scans the themes directory and loads all valid themes.
// Invalid themes are skipped with errors logged.
func DiscoverCustomThemes() ([]string, []error) {
	dir := ThemesDir()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, []error{fmt.Errorf("creating themes directory: %w", err)}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{fmt.Errorf("reading themes directory: %w", err)}
	}

	var loaded []string
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		path := filepath.Join(dir, name)
		theme, err := LoadThemeFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}

		// Generate theme name from filename (without extension)
		themeName := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")

		// Don't allow custom themes to override built-in themes
		if IsBuiltinTheme(themeName) {
			errs = append(errs, fmt.Errorf("%s: cannot override built-in theme '%s'", name, themeName))
			continue
		}

		RegisterCustomTheme(ThemeName(themeName), theme)
		loaded = append(loaded, themeName)
	}

	return loaded, errs
}

// IsBuiltinTheme checks if a theme name is a built-in theme.
func IsBuiltinTheme(name string) bool {
	return slices.Contains(BuiltinThemes(), name)
}

// IsCustomTheme checks if a theme name is a registered custom theme.
func IsCustomTheme(name string) bool {
	_, ok := customThemes[ThemeName(name)]
	return ok
}

// ExportTheme exports a theme to YAML format.
// This can be used to save the current theme or create a template for customization.
func ExportTheme(name ThemeName) ([]byte, error) {
	var palette *ColorPalette
	var themeFile *ThemeFile

	// Check if it's a custom theme first
	if custom := GetCustomTheme(name); custom != nil {
		themeFile = custom
	} else {
		// Get built-in palette
		palette = GetPalette(name)
		themeFile = paletteToThemeFile(string(name), palette)
	}

	return yaml.Marshal(themeFile)
}

// paletteToThemeFile converts a ColorPalette to a ThemeFile for export.
func paletteToThemeFile(name string, p *ColorPalette) *ThemeFile {
	return &ThemeFile{
		Name:        name,
		Description: fmt.Sprintf("Exported from Claudio built-in theme '%s'", name),
		Version:     "1",
		Colors: ThemeColors{
			Primary:   string(p.Primary),
			Secondary: string(p.Secondary),
			Warning:   string(p.Warning),
			Error:     string(p.Error),
			Muted:     string(p.Muted),
			Surface:   string(p.Surface),
			Text:      string(p.Text),
			Border:    string(p.Border),
			Status: ThemeStatusColors{
				Working:     string(p.StatusWorking),
				Pending:     string(p.StatusPending),
				Preparing:   string(p.StatusPreparing),
				Input:       string(p.StatusInput),
				Paused:      string(p.StatusPaused),
				Complete:    string(p.StatusComplete),
				Error:       string(p.StatusError),
				CreatingPR:  string(p.StatusCreatingPR),
				Stuck:       string(p.StatusStuck),
				Timeout:     string(p.StatusTimeout),
				Interrupted: string(p.StatusInterrupted),
			},
			Diff: ThemeDiffColors{
				Add:     string(p.DiffAdd),
				Remove:  string(p.DiffRemove),
				Header:  string(p.DiffHeader),
				Hunk:    string(p.DiffHunk),
				Context: string(p.DiffContext),
			},
			Search: ThemeSearchColors{
				MatchBg:   string(p.SearchMatchBg),
				MatchFg:   string(p.SearchMatchFg),
				CurrentBg: string(p.SearchCurrentBg),
				CurrentFg: string(p.SearchCurrentFg),
			},
			Accents: ThemeAccentColors{
				Blue:   string(p.Blue),
				Yellow: string(p.Yellow),
				Purple: string(p.Purple),
				Pink:   string(p.Pink),
				Orange: string(p.Orange),
			},
		},
	}
}

// SaveTheme saves a theme to the themes directory.
func SaveTheme(name string, theme *ThemeFile) error {
	dir := ThemesDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating themes directory: %w", err)
	}

	data, err := yaml.Marshal(theme)
	if err != nil {
		return fmt.Errorf("marshaling theme: %w", err)
	}

	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing theme file: %w", err)
	}

	return nil
}

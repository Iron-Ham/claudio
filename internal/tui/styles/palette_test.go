package styles

import (
	"slices"
	"testing"
)

func TestValidThemes(t *testing.T) {
	themes := ValidThemes()

	if len(themes) != 14 {
		t.Errorf("ValidThemes() returned %d themes, want 14", len(themes))
	}

	expected := []string{
		"default", "monokai", "dracula", "nord",
		"claude-code", "solarized-dark", "solarized-light", "one-dark",
		"github-dark", "gruvbox", "tokyo-night", "catppuccin",
		"synthwave", "ayu",
	}
	for _, want := range expected {
		if !slices.Contains(themes, want) {
			t.Errorf("ValidThemes() missing %q", want)
		}
	}
}

func TestIsValidTheme(t *testing.T) {
	tests := []struct {
		name  string
		theme string
		want  bool
	}{
		{"default theme", "default", true},
		{"monokai theme", "monokai", true},
		{"dracula theme", "dracula", true},
		{"nord theme", "nord", true},
		{"claude-code theme", "claude-code", true},
		{"solarized-dark theme", "solarized-dark", true},
		{"solarized-light theme", "solarized-light", true},
		{"one-dark theme", "one-dark", true},
		{"github-dark theme", "github-dark", true},
		{"gruvbox theme", "gruvbox", true},
		{"tokyo-night theme", "tokyo-night", true},
		{"catppuccin theme", "catppuccin", true},
		{"synthwave theme", "synthwave", true},
		{"ayu theme", "ayu", true},
		{"invalid theme", "invalid", false},
		{"empty string", "", false},
		{"case sensitive", "Default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidTheme(tt.theme)
			if got != tt.want {
				t.Errorf("IsValidTheme(%q) = %v, want %v", tt.theme, got, tt.want)
			}
		})
	}
}

func TestThemeNameConstants(t *testing.T) {
	tests := []struct {
		constant ThemeName
		want     string
	}{
		{ThemeDefault, "default"},
		{ThemeMonokai, "monokai"},
		{ThemeDracula, "dracula"},
		{ThemeNord, "nord"},
		{ThemeClaudeCode, "claude-code"},
		{ThemeSolarizedDark, "solarized-dark"},
		{ThemeSolarizedLight, "solarized-light"},
		{ThemeOneDark, "one-dark"},
		{ThemeGitHubDark, "github-dark"},
		{ThemeGruvbox, "gruvbox"},
		{ThemeTokyoNight, "tokyo-night"},
		{ThemeCatppuccin, "catppuccin"},
		{ThemeSynthwave, "synthwave"},
		{ThemeAyu, "ayu"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.constant) != tt.want {
				t.Errorf("Theme constant = %q, want %q", tt.constant, tt.want)
			}
		})
	}
}

func TestPalettes(t *testing.T) {
	tests := []struct {
		name       string
		getPalette func() *ColorPalette
		primary    string
		secondary  string
		surface    string
	}{
		{"default", DefaultPalette, "#A78BFA", "#10B981", "#1F2937"},
		{"monokai", MonokaiPalette, "#F92672", "#A6E22E", "#272822"},
		{"dracula", DraculaPalette, "#BD93F9", "#50FA7B", "#282A36"},
		{"nord", NordPalette, "#88C0D0", "#A3BE8C", "#2E3440"},
		{"claude-code", ClaudeCodePalette, "#DA7756", "#7DCEA0", "#1C1C1C"},
		{"solarized-dark", SolarizedDarkPalette, "#268BD2", "#859900", "#002B36"},
		{"solarized-light", SolarizedLightPalette, "#268BD2", "#859900", "#FDF6E3"},
		{"one-dark", OneDarkPalette, "#61AFEF", "#98C379", "#282C34"},
		{"github-dark", GitHubDarkPalette, "#58A6FF", "#3FB950", "#0D1117"},
		{"gruvbox", GruvboxPalette, "#83A598", "#B8BB26", "#282828"},
		{"tokyo-night", TokyoNightPalette, "#7AA2F7", "#9ECE6A", "#1A1B26"},
		{"catppuccin", CatppuccinPalette, "#89B4FA", "#A6E3A1", "#1E1E2E"},
		{"synthwave", SynthwavePalette, "#FF7EDB", "#72F1B8", "#262335"},
		{"ayu", AyuPalette, "#39BAE6", "#7FD962", "#0D1017"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := tt.getPalette()
			if p == nil {
				t.Fatal("palette returned nil")
			}
			if string(p.Primary) != tt.primary {
				t.Errorf("Primary = %q, want %q", p.Primary, tt.primary)
			}
			if string(p.Secondary) != tt.secondary {
				t.Errorf("Secondary = %q, want %q", p.Secondary, tt.secondary)
			}
			if string(p.Surface) != tt.surface {
				t.Errorf("Surface = %q, want %q", p.Surface, tt.surface)
			}
		})
	}
}

func TestGetPalette(t *testing.T) {
	tests := []struct {
		name        ThemeName
		wantPrimary string // Use primary color to identify theme
	}{
		{ThemeDefault, "#A78BFA"},
		{ThemeMonokai, "#F92672"},
		{ThemeDracula, "#BD93F9"},
		{ThemeNord, "#88C0D0"},
		{ThemeClaudeCode, "#DA7756"},
		{ThemeSolarizedDark, "#268BD2"},
		{ThemeSolarizedLight, "#268BD2"},
		{ThemeOneDark, "#61AFEF"},
		{ThemeGitHubDark, "#58A6FF"},
		{ThemeGruvbox, "#83A598"},
		{ThemeTokyoNight, "#7AA2F7"},
		{ThemeCatppuccin, "#89B4FA"},
		{ThemeSynthwave, "#FF7EDB"},
		{ThemeAyu, "#39BAE6"},
		{"unknown", "#A78BFA"}, // Should fall back to default
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			p := GetPalette(tt.name)
			if p == nil {
				t.Fatal("GetPalette() returned nil")
			}
			if string(p.Primary) != tt.wantPrimary {
				t.Errorf("GetPalette(%q).Primary = %q, want %q", tt.name, p.Primary, tt.wantPrimary)
			}
		})
	}
}

func TestPaletteColorConsistency(t *testing.T) {
	palettes := []*ColorPalette{
		DefaultPalette(),
		MonokaiPalette(),
		DraculaPalette(),
		NordPalette(),
		ClaudeCodePalette(),
		SolarizedDarkPalette(),
		SolarizedLightPalette(),
		OneDarkPalette(),
		GitHubDarkPalette(),
		GruvboxPalette(),
		TokyoNightPalette(),
		CatppuccinPalette(),
		SynthwavePalette(),
		AyuPalette(),
	}

	for i, p := range palettes {
		t.Run(ValidThemes()[i], func(t *testing.T) {
			// All palettes should have all colors set
			colors := map[string]string{
				"Primary":           string(p.Primary),
				"Secondary":         string(p.Secondary),
				"Warning":           string(p.Warning),
				"Error":             string(p.Error),
				"Muted":             string(p.Muted),
				"Surface":           string(p.Surface),
				"Text":              string(p.Text),
				"Border":            string(p.Border),
				"StatusWorking":     string(p.StatusWorking),
				"StatusPending":     string(p.StatusPending),
				"StatusInput":       string(p.StatusInput),
				"StatusPaused":      string(p.StatusPaused),
				"StatusComplete":    string(p.StatusComplete),
				"StatusError":       string(p.StatusError),
				"StatusCreatingPR":  string(p.StatusCreatingPR),
				"StatusStuck":       string(p.StatusStuck),
				"StatusTimeout":     string(p.StatusTimeout),
				"StatusInterrupted": string(p.StatusInterrupted),
				"DiffAdd":           string(p.DiffAdd),
				"DiffRemove":        string(p.DiffRemove),
				"DiffHeader":        string(p.DiffHeader),
				"DiffHunk":          string(p.DiffHunk),
				"DiffContext":       string(p.DiffContext),
				"SearchMatchBg":     string(p.SearchMatchBg),
				"SearchMatchFg":     string(p.SearchMatchFg),
				"SearchCurrentBg":   string(p.SearchCurrentBg),
				"SearchCurrentFg":   string(p.SearchCurrentFg),
				"Blue":              string(p.Blue),
				"Yellow":            string(p.Yellow),
				"Purple":            string(p.Purple),
				"Pink":              string(p.Pink),
				"Orange":            string(p.Orange),
			}

			for name, color := range colors {
				if color == "" {
					t.Errorf("%s color is empty", name)
				}
			}
		})
	}
}

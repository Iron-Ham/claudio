package styles

import "testing"

func TestValidThemes(t *testing.T) {
	themes := ValidThemes()

	if len(themes) != 4 {
		t.Errorf("ValidThemes() returned %d themes, want 4", len(themes))
	}

	// Verify expected themes are present
	expected := []string{"default", "monokai", "dracula", "nord"}
	for _, want := range expected {
		found := false
		for _, got := range themes {
			if got == want {
				found = true
				break
			}
		}
		if !found {
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
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.constant) != tt.want {
				t.Errorf("Theme constant = %q, want %q", tt.constant, tt.want)
			}
		})
	}
}

func TestDefaultPalette(t *testing.T) {
	p := DefaultPalette()

	if p == nil {
		t.Fatal("DefaultPalette() returned nil")
	}

	// Verify key colors are set
	if p.Primary == "" {
		t.Error("Primary color is empty")
	}
	if p.Secondary == "" {
		t.Error("Secondary color is empty")
	}
	if p.Warning == "" {
		t.Error("Warning color is empty")
	}
	if p.Error == "" {
		t.Error("Error color is empty")
	}
	if p.Text == "" {
		t.Error("Text color is empty")
	}
	if p.Surface == "" {
		t.Error("Surface color is empty")
	}

	// Verify status colors are set
	if p.StatusWorking == "" {
		t.Error("StatusWorking color is empty")
	}
	if p.StatusComplete == "" {
		t.Error("StatusComplete color is empty")
	}

	// Verify diff colors are set
	if p.DiffAdd == "" {
		t.Error("DiffAdd color is empty")
	}
	if p.DiffRemove == "" {
		t.Error("DiffRemove color is empty")
	}
}

func TestMonokaiPalette(t *testing.T) {
	p := MonokaiPalette()

	if p == nil {
		t.Fatal("MonokaiPalette() returned nil")
	}

	// Verify Monokai-specific colors
	if string(p.Primary) != "#F92672" {
		t.Errorf("Primary = %q, want #F92672 (Monokai pink)", p.Primary)
	}
	if string(p.Secondary) != "#A6E22E" {
		t.Errorf("Secondary = %q, want #A6E22E (Monokai green)", p.Secondary)
	}
	if string(p.Surface) != "#272822" {
		t.Errorf("Surface = %q, want #272822 (Monokai background)", p.Surface)
	}
}

func TestDraculaPalette(t *testing.T) {
	p := DraculaPalette()

	if p == nil {
		t.Fatal("DraculaPalette() returned nil")
	}

	// Verify Dracula-specific colors
	if string(p.Primary) != "#BD93F9" {
		t.Errorf("Primary = %q, want #BD93F9 (Dracula purple)", p.Primary)
	}
	if string(p.Secondary) != "#50FA7B" {
		t.Errorf("Secondary = %q, want #50FA7B (Dracula green)", p.Secondary)
	}
	if string(p.Surface) != "#282A36" {
		t.Errorf("Surface = %q, want #282A36 (Dracula background)", p.Surface)
	}
}

func TestNordPalette(t *testing.T) {
	p := NordPalette()

	if p == nil {
		t.Fatal("NordPalette() returned nil")
	}

	// Verify Nord-specific colors
	if string(p.Primary) != "#88C0D0" {
		t.Errorf("Primary = %q, want #88C0D0 (Nord frost cyan)", p.Primary)
	}
	if string(p.Secondary) != "#A3BE8C" {
		t.Errorf("Secondary = %q, want #A3BE8C (Nord aurora green)", p.Secondary)
	}
	if string(p.Surface) != "#2E3440" {
		t.Errorf("Surface = %q, want #2E3440 (Nord polar night)", p.Surface)
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

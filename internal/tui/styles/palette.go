package styles

import (
	"slices"

	"github.com/charmbracelet/lipgloss"
)

// ThemeName represents a named color theme.
type ThemeName string

// Available theme names.
const (
	ThemeDefault        ThemeName = "default"         // Purple/green dark theme (current)
	ThemeMonokai        ThemeName = "monokai"         // Classic Monokai editor colors
	ThemeDracula        ThemeName = "dracula"         // Dracula theme colors
	ThemeNord           ThemeName = "nord"            // Nord theme - cool blue-gray
	ThemeClaudeCode     ThemeName = "claude-code"     // Claude Code inspired - orange/coral accents
	ThemeSolarizedDark  ThemeName = "solarized-dark"  // Solarized Dark by Ethan Schoonover
	ThemeSolarizedLight ThemeName = "solarized-light" // Solarized Light variant
	ThemeOneDark        ThemeName = "one-dark"        // Atom One Dark theme
	ThemeGitHubDark     ThemeName = "github-dark"     // GitHub Dark mode
	ThemeGruvbox        ThemeName = "gruvbox"         // Gruvbox retro groove
	ThemeTokyoNight     ThemeName = "tokyo-night"     // Tokyo Night modern theme
	ThemeCatppuccin     ThemeName = "catppuccin"      // Catppuccin Mocha pastel theme
	ThemeSynthwave      ThemeName = "synthwave"       // Synthwave '84 retro neon
	ThemeAyu            ThemeName = "ayu"             // Ayu Dark clean theme
)

// BuiltinThemes returns all built-in theme names.
func BuiltinThemes() []string {
	return []string{
		string(ThemeDefault),
		string(ThemeMonokai),
		string(ThemeDracula),
		string(ThemeNord),
		string(ThemeClaudeCode),
		string(ThemeSolarizedDark),
		string(ThemeSolarizedLight),
		string(ThemeOneDark),
		string(ThemeGitHubDark),
		string(ThemeGruvbox),
		string(ThemeTokyoNight),
		string(ThemeCatppuccin),
		string(ThemeSynthwave),
		string(ThemeAyu),
	}
}

// ValidThemes returns all valid theme names (built-in + custom).
func ValidThemes() []string {
	themes := BuiltinThemes()
	themes = append(themes, CustomThemeNames()...)
	return themes
}

// IsValidTheme checks if a theme name is valid (built-in or custom).
func IsValidTheme(name string) bool {
	// Check built-in themes first (faster)
	if slices.Contains(BuiltinThemes(), name) {
		return true
	}
	// Check custom themes
	return IsCustomTheme(name)
}

// ColorPalette defines the color scheme for a theme.
// All colors should meet WCAG AA contrast requirements (4.5:1 ratio).
type ColorPalette struct {
	// Primary accent color (used for emphasis, active elements)
	Primary lipgloss.Color
	// Secondary accent color (used for secondary emphasis, success states)
	Secondary lipgloss.Color
	// Warning color (used for warnings, attention-needed states)
	Warning lipgloss.Color
	// Error color (used for errors, failures)
	Error lipgloss.Color
	// Muted color (used for de-emphasized text, borders)
	Muted lipgloss.Color
	// Surface color (used for panel backgrounds)
	Surface lipgloss.Color
	// Text color (primary text)
	Text lipgloss.Color
	// Border color (panel borders)
	Border lipgloss.Color

	// Status-specific colors
	StatusWorking     lipgloss.Color
	StatusPending     lipgloss.Color
	StatusInput       lipgloss.Color
	StatusPaused      lipgloss.Color
	StatusComplete    lipgloss.Color
	StatusError       lipgloss.Color
	StatusCreatingPR  lipgloss.Color
	StatusStuck       lipgloss.Color
	StatusTimeout     lipgloss.Color
	StatusInterrupted lipgloss.Color

	// Diff colors
	DiffAdd     lipgloss.Color
	DiffRemove  lipgloss.Color
	DiffHeader  lipgloss.Color
	DiffHunk    lipgloss.Color
	DiffContext lipgloss.Color

	// Search highlight colors
	SearchMatchBg   lipgloss.Color
	SearchMatchFg   lipgloss.Color
	SearchCurrentBg lipgloss.Color
	SearchCurrentFg lipgloss.Color

	// Additional accent colors
	Blue   lipgloss.Color
	Yellow lipgloss.Color
	Purple lipgloss.Color
	Pink   lipgloss.Color
	Orange lipgloss.Color
}

// DefaultPalette returns the default purple/green dark theme palette.
// This is the original Claudio theme with WCAG AA compliant colors.
func DefaultPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#A78BFA"), // Purple (violet-400)
		Secondary: lipgloss.Color("#10B981"), // Green
		Warning:   lipgloss.Color("#F59E0B"), // Amber
		Error:     lipgloss.Color("#F87171"), // Red (red-400)
		Muted:     lipgloss.Color("#9CA3AF"), // Gray
		Surface:   lipgloss.Color("#1F2937"), // Dark surface
		Text:      lipgloss.Color("#F9FAFB"), // Light text
		Border:    lipgloss.Color("#6B7280"), // Gray-500

		StatusWorking:     lipgloss.Color("#10B981"), // Green
		StatusPending:     lipgloss.Color("#9CA3AF"), // Gray
		StatusInput:       lipgloss.Color("#F59E0B"), // Amber
		StatusPaused:      lipgloss.Color("#60A5FA"), // Blue
		StatusComplete:    lipgloss.Color("#A78BFA"), // Purple
		StatusError:       lipgloss.Color("#F87171"), // Red
		StatusCreatingPR:  lipgloss.Color("#F472B6"), // Pink
		StatusStuck:       lipgloss.Color("#FB923C"), // Orange
		StatusTimeout:     lipgloss.Color("#F87171"), // Red
		StatusInterrupted: lipgloss.Color("#FBBF24"), // Yellow

		DiffAdd:     lipgloss.Color("#22C55E"), // Green
		DiffRemove:  lipgloss.Color("#F87171"), // Red
		DiffHeader:  lipgloss.Color("#60A5FA"), // Blue
		DiffHunk:    lipgloss.Color("#A78BFA"), // Purple
		DiffContext: lipgloss.Color("#9CA3AF"), // Gray

		SearchMatchBg:   lipgloss.Color("#854D0E"), // Dark yellow
		SearchMatchFg:   lipgloss.Color("#FEF3C7"), // Light cream
		SearchCurrentBg: lipgloss.Color("#C2410C"), // Dark orange
		SearchCurrentFg: lipgloss.Color("#FFF7ED"), // Light orange-white

		Blue:   lipgloss.Color("#60A5FA"),
		Yellow: lipgloss.Color("#FBBF24"),
		Purple: lipgloss.Color("#A78BFA"),
		Pink:   lipgloss.Color("#F472B6"),
		Orange: lipgloss.Color("#FB923C"),
	}
}

// MonokaiPalette returns the classic Monokai editor theme palette.
// Based on the iconic Monokai color scheme from Sublime Text.
func MonokaiPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#F92672"), // Monokai pink/magenta
		Secondary: lipgloss.Color("#A6E22E"), // Monokai green
		Warning:   lipgloss.Color("#E6DB74"), // Monokai yellow
		Error:     lipgloss.Color("#F92672"), // Monokai pink (same as primary)
		Muted:     lipgloss.Color("#75715E"), // Monokai comment gray
		Surface:   lipgloss.Color("#272822"), // Monokai background
		Text:      lipgloss.Color("#F8F8F2"), // Monokai foreground
		Border:    lipgloss.Color("#49483E"), // Monokai selection

		StatusWorking:     lipgloss.Color("#A6E22E"), // Green
		StatusPending:     lipgloss.Color("#75715E"), // Comment gray
		StatusInput:       lipgloss.Color("#E6DB74"), // Yellow
		StatusPaused:      lipgloss.Color("#66D9EF"), // Cyan
		StatusComplete:    lipgloss.Color("#AE81FF"), // Purple
		StatusError:       lipgloss.Color("#F92672"), // Pink
		StatusCreatingPR:  lipgloss.Color("#FD971F"), // Orange
		StatusStuck:       lipgloss.Color("#FD971F"), // Orange
		StatusTimeout:     lipgloss.Color("#F92672"), // Pink
		StatusInterrupted: lipgloss.Color("#E6DB74"), // Yellow

		DiffAdd:     lipgloss.Color("#A6E22E"), // Green
		DiffRemove:  lipgloss.Color("#F92672"), // Pink
		DiffHeader:  lipgloss.Color("#66D9EF"), // Cyan
		DiffHunk:    lipgloss.Color("#AE81FF"), // Purple
		DiffContext: lipgloss.Color("#75715E"), // Comment gray

		SearchMatchBg:   lipgloss.Color("#49483E"), // Selection
		SearchMatchFg:   lipgloss.Color("#E6DB74"), // Yellow
		SearchCurrentBg: lipgloss.Color("#F92672"), // Pink
		SearchCurrentFg: lipgloss.Color("#F8F8F2"), // Foreground

		Blue:   lipgloss.Color("#66D9EF"), // Cyan
		Yellow: lipgloss.Color("#E6DB74"),
		Purple: lipgloss.Color("#AE81FF"),
		Pink:   lipgloss.Color("#F92672"),
		Orange: lipgloss.Color("#FD971F"),
	}
}

// DraculaPalette returns the Dracula theme palette.
// Based on the popular Dracula color scheme.
func DraculaPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#BD93F9"), // Dracula purple
		Secondary: lipgloss.Color("#50FA7B"), // Dracula green
		Warning:   lipgloss.Color("#F1FA8C"), // Dracula yellow
		Error:     lipgloss.Color("#FF5555"), // Dracula red
		Muted:     lipgloss.Color("#6272A4"), // Dracula comment
		Surface:   lipgloss.Color("#282A36"), // Dracula background
		Text:      lipgloss.Color("#F8F8F2"), // Dracula foreground
		Border:    lipgloss.Color("#44475A"), // Dracula selection

		StatusWorking:     lipgloss.Color("#50FA7B"), // Green
		StatusPending:     lipgloss.Color("#6272A4"), // Comment
		StatusInput:       lipgloss.Color("#F1FA8C"), // Yellow
		StatusPaused:      lipgloss.Color("#8BE9FD"), // Cyan
		StatusComplete:    lipgloss.Color("#BD93F9"), // Purple
		StatusError:       lipgloss.Color("#FF5555"), // Red
		StatusCreatingPR:  lipgloss.Color("#FF79C6"), // Pink
		StatusStuck:       lipgloss.Color("#FFB86C"), // Orange
		StatusTimeout:     lipgloss.Color("#FF5555"), // Red
		StatusInterrupted: lipgloss.Color("#F1FA8C"), // Yellow

		DiffAdd:     lipgloss.Color("#50FA7B"), // Green
		DiffRemove:  lipgloss.Color("#FF5555"), // Red
		DiffHeader:  lipgloss.Color("#8BE9FD"), // Cyan
		DiffHunk:    lipgloss.Color("#BD93F9"), // Purple
		DiffContext: lipgloss.Color("#6272A4"), // Comment

		SearchMatchBg:   lipgloss.Color("#44475A"), // Selection
		SearchMatchFg:   lipgloss.Color("#F1FA8C"), // Yellow
		SearchCurrentBg: lipgloss.Color("#FF79C6"), // Pink
		SearchCurrentFg: lipgloss.Color("#F8F8F2"), // Foreground

		Blue:   lipgloss.Color("#8BE9FD"), // Cyan
		Yellow: lipgloss.Color("#F1FA8C"),
		Purple: lipgloss.Color("#BD93F9"),
		Pink:   lipgloss.Color("#FF79C6"),
		Orange: lipgloss.Color("#FFB86C"),
	}
}

// NordPalette returns the Nord theme palette.
// Based on the arctic, north-bluish color scheme.
func NordPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#88C0D0"), // Nord frost (cyan)
		Secondary: lipgloss.Color("#A3BE8C"), // Nord aurora green
		Warning:   lipgloss.Color("#EBCB8B"), // Nord aurora yellow
		Error:     lipgloss.Color("#BF616A"), // Nord aurora red
		Muted:     lipgloss.Color("#4C566A"), // Nord polar night 3
		Surface:   lipgloss.Color("#2E3440"), // Nord polar night 0
		Text:      lipgloss.Color("#ECEFF4"), // Nord snow storm 2
		Border:    lipgloss.Color("#3B4252"), // Nord polar night 1

		StatusWorking:     lipgloss.Color("#A3BE8C"), // Green
		StatusPending:     lipgloss.Color("#4C566A"), // Gray
		StatusInput:       lipgloss.Color("#EBCB8B"), // Yellow
		StatusPaused:      lipgloss.Color("#81A1C1"), // Frost blue
		StatusComplete:    lipgloss.Color("#B48EAD"), // Aurora purple
		StatusError:       lipgloss.Color("#BF616A"), // Aurora red
		StatusCreatingPR:  lipgloss.Color("#B48EAD"), // Purple
		StatusStuck:       lipgloss.Color("#D08770"), // Aurora orange
		StatusTimeout:     lipgloss.Color("#BF616A"), // Red
		StatusInterrupted: lipgloss.Color("#EBCB8B"), // Yellow

		DiffAdd:     lipgloss.Color("#A3BE8C"), // Green
		DiffRemove:  lipgloss.Color("#BF616A"), // Red
		DiffHeader:  lipgloss.Color("#5E81AC"), // Frost deep blue
		DiffHunk:    lipgloss.Color("#B48EAD"), // Purple
		DiffContext: lipgloss.Color("#4C566A"), // Gray

		SearchMatchBg:   lipgloss.Color("#3B4252"), // Polar night 1
		SearchMatchFg:   lipgloss.Color("#EBCB8B"), // Yellow
		SearchCurrentBg: lipgloss.Color("#5E81AC"), // Frost deep blue
		SearchCurrentFg: lipgloss.Color("#ECEFF4"), // Snow storm

		Blue:   lipgloss.Color("#81A1C1"), // Frost blue
		Yellow: lipgloss.Color("#EBCB8B"),
		Purple: lipgloss.Color("#B48EAD"),
		Pink:   lipgloss.Color("#B48EAD"), // Nord doesn't have pink, use purple
		Orange: lipgloss.Color("#D08770"),
	}
}

// ClaudeCodePalette returns a theme inspired by Claude Code CLI.
// Features the signature orange/coral accent colors with a dark background.
func ClaudeCodePalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#DA7756"), // Claude orange/coral
		Secondary: lipgloss.Color("#7DCEA0"), // Soft green
		Warning:   lipgloss.Color("#F5B041"), // Warm amber
		Error:     lipgloss.Color("#EC7063"), // Soft red
		Muted:     lipgloss.Color("#7F8C8D"), // Gray
		Surface:   lipgloss.Color("#1C1C1C"), // Dark surface
		Text:      lipgloss.Color("#F4F4F4"), // Light text
		Border:    lipgloss.Color("#4A4A4A"), // Medium gray

		StatusWorking:     lipgloss.Color("#7DCEA0"), // Green
		StatusPending:     lipgloss.Color("#7F8C8D"), // Gray
		StatusInput:       lipgloss.Color("#F5B041"), // Amber
		StatusPaused:      lipgloss.Color("#5DADE2"), // Blue
		StatusComplete:    lipgloss.Color("#DA7756"), // Orange
		StatusError:       lipgloss.Color("#EC7063"), // Red
		StatusCreatingPR:  lipgloss.Color("#BB8FCE"), // Purple
		StatusStuck:       lipgloss.Color("#E59866"), // Light orange
		StatusTimeout:     lipgloss.Color("#EC7063"), // Red
		StatusInterrupted: lipgloss.Color("#F5B041"), // Amber

		DiffAdd:     lipgloss.Color("#7DCEA0"), // Green
		DiffRemove:  lipgloss.Color("#EC7063"), // Red
		DiffHeader:  lipgloss.Color("#5DADE2"), // Blue
		DiffHunk:    lipgloss.Color("#DA7756"), // Orange
		DiffContext: lipgloss.Color("#7F8C8D"), // Gray

		SearchMatchBg:   lipgloss.Color("#5D4E37"), // Dark orange
		SearchMatchFg:   lipgloss.Color("#F5B041"), // Amber
		SearchCurrentBg: lipgloss.Color("#DA7756"), // Orange
		SearchCurrentFg: lipgloss.Color("#F4F4F4"), // Light text

		Blue:   lipgloss.Color("#5DADE2"),
		Yellow: lipgloss.Color("#F5B041"),
		Purple: lipgloss.Color("#BB8FCE"),
		Pink:   lipgloss.Color("#F1948A"),
		Orange: lipgloss.Color("#E59866"),
	}
}

// SolarizedDarkPalette returns the Solarized Dark theme palette.
// Based on Ethan Schoonover's precision color scheme.
func SolarizedDarkPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#268BD2"), // Solarized blue
		Secondary: lipgloss.Color("#859900"), // Solarized green
		Warning:   lipgloss.Color("#B58900"), // Solarized yellow
		Error:     lipgloss.Color("#DC322F"), // Solarized red
		Muted:     lipgloss.Color("#586E75"), // Base01
		Surface:   lipgloss.Color("#002B36"), // Base03 background
		Text:      lipgloss.Color("#839496"), // Base0 text
		Border:    lipgloss.Color("#073642"), // Base02

		StatusWorking:     lipgloss.Color("#859900"), // Green
		StatusPending:     lipgloss.Color("#586E75"), // Gray
		StatusInput:       lipgloss.Color("#B58900"), // Yellow
		StatusPaused:      lipgloss.Color("#2AA198"), // Cyan
		StatusComplete:    lipgloss.Color("#6C71C4"), // Violet
		StatusError:       lipgloss.Color("#DC322F"), // Red
		StatusCreatingPR:  lipgloss.Color("#D33682"), // Magenta
		StatusStuck:       lipgloss.Color("#CB4B16"), // Orange
		StatusTimeout:     lipgloss.Color("#DC322F"), // Red
		StatusInterrupted: lipgloss.Color("#B58900"), // Yellow

		DiffAdd:     lipgloss.Color("#859900"), // Green
		DiffRemove:  lipgloss.Color("#DC322F"), // Red
		DiffHeader:  lipgloss.Color("#268BD2"), // Blue
		DiffHunk:    lipgloss.Color("#6C71C4"), // Violet
		DiffContext: lipgloss.Color("#586E75"), // Gray

		SearchMatchBg:   lipgloss.Color("#073642"), // Base02
		SearchMatchFg:   lipgloss.Color("#B58900"), // Yellow
		SearchCurrentBg: lipgloss.Color("#268BD2"), // Blue
		SearchCurrentFg: lipgloss.Color("#FDF6E3"), // Base3

		Blue:   lipgloss.Color("#268BD2"),
		Yellow: lipgloss.Color("#B58900"),
		Purple: lipgloss.Color("#6C71C4"),
		Pink:   lipgloss.Color("#D33682"),
		Orange: lipgloss.Color("#CB4B16"),
	}
}

// SolarizedLightPalette returns the Solarized Light theme palette.
// Light variant of Ethan Schoonover's precision color scheme.
func SolarizedLightPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#268BD2"), // Solarized blue
		Secondary: lipgloss.Color("#859900"), // Solarized green
		Warning:   lipgloss.Color("#B58900"), // Solarized yellow
		Error:     lipgloss.Color("#DC322F"), // Solarized red
		Muted:     lipgloss.Color("#93A1A1"), // Base1
		Surface:   lipgloss.Color("#FDF6E3"), // Base3 background
		Text:      lipgloss.Color("#657B83"), // Base00 text
		Border:    lipgloss.Color("#EEE8D5"), // Base2

		StatusWorking:     lipgloss.Color("#859900"), // Green
		StatusPending:     lipgloss.Color("#93A1A1"), // Gray
		StatusInput:       lipgloss.Color("#B58900"), // Yellow
		StatusPaused:      lipgloss.Color("#2AA198"), // Cyan
		StatusComplete:    lipgloss.Color("#6C71C4"), // Violet
		StatusError:       lipgloss.Color("#DC322F"), // Red
		StatusCreatingPR:  lipgloss.Color("#D33682"), // Magenta
		StatusStuck:       lipgloss.Color("#CB4B16"), // Orange
		StatusTimeout:     lipgloss.Color("#DC322F"), // Red
		StatusInterrupted: lipgloss.Color("#B58900"), // Yellow

		DiffAdd:     lipgloss.Color("#859900"), // Green
		DiffRemove:  lipgloss.Color("#DC322F"), // Red
		DiffHeader:  lipgloss.Color("#268BD2"), // Blue
		DiffHunk:    lipgloss.Color("#6C71C4"), // Violet
		DiffContext: lipgloss.Color("#93A1A1"), // Gray

		SearchMatchBg:   lipgloss.Color("#EEE8D5"), // Base2
		SearchMatchFg:   lipgloss.Color("#B58900"), // Yellow
		SearchCurrentBg: lipgloss.Color("#268BD2"), // Blue
		SearchCurrentFg: lipgloss.Color("#FDF6E3"), // Base3

		Blue:   lipgloss.Color("#268BD2"),
		Yellow: lipgloss.Color("#B58900"),
		Purple: lipgloss.Color("#6C71C4"),
		Pink:   lipgloss.Color("#D33682"),
		Orange: lipgloss.Color("#CB4B16"),
	}
}

// OneDarkPalette returns the Atom One Dark theme palette.
// Based on Atom's iconic One Dark syntax theme.
func OneDarkPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#61AFEF"), // One Dark blue
		Secondary: lipgloss.Color("#98C379"), // One Dark green
		Warning:   lipgloss.Color("#E5C07B"), // One Dark yellow
		Error:     lipgloss.Color("#E06C75"), // One Dark red
		Muted:     lipgloss.Color("#5C6370"), // One Dark comment
		Surface:   lipgloss.Color("#282C34"), // One Dark background
		Text:      lipgloss.Color("#ABB2BF"), // One Dark foreground
		Border:    lipgloss.Color("#3E4451"), // One Dark gutter

		StatusWorking:     lipgloss.Color("#98C379"), // Green
		StatusPending:     lipgloss.Color("#5C6370"), // Gray
		StatusInput:       lipgloss.Color("#E5C07B"), // Yellow
		StatusPaused:      lipgloss.Color("#56B6C2"), // Cyan
		StatusComplete:    lipgloss.Color("#C678DD"), // Purple
		StatusError:       lipgloss.Color("#E06C75"), // Red
		StatusCreatingPR:  lipgloss.Color("#C678DD"), // Purple
		StatusStuck:       lipgloss.Color("#D19A66"), // Orange
		StatusTimeout:     lipgloss.Color("#E06C75"), // Red
		StatusInterrupted: lipgloss.Color("#E5C07B"), // Yellow

		DiffAdd:     lipgloss.Color("#98C379"), // Green
		DiffRemove:  lipgloss.Color("#E06C75"), // Red
		DiffHeader:  lipgloss.Color("#61AFEF"), // Blue
		DiffHunk:    lipgloss.Color("#C678DD"), // Purple
		DiffContext: lipgloss.Color("#5C6370"), // Gray

		SearchMatchBg:   lipgloss.Color("#3E4451"), // Gutter
		SearchMatchFg:   lipgloss.Color("#E5C07B"), // Yellow
		SearchCurrentBg: lipgloss.Color("#61AFEF"), // Blue
		SearchCurrentFg: lipgloss.Color("#282C34"), // Background

		Blue:   lipgloss.Color("#61AFEF"),
		Yellow: lipgloss.Color("#E5C07B"),
		Purple: lipgloss.Color("#C678DD"),
		Pink:   lipgloss.Color("#C678DD"), // One Dark uses purple for pink
		Orange: lipgloss.Color("#D19A66"),
	}
}

// GitHubDarkPalette returns the GitHub Dark theme palette.
// Based on GitHub's dark mode color scheme.
func GitHubDarkPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#58A6FF"), // GitHub blue
		Secondary: lipgloss.Color("#3FB950"), // GitHub green
		Warning:   lipgloss.Color("#D29922"), // GitHub yellow
		Error:     lipgloss.Color("#F85149"), // GitHub red
		Muted:     lipgloss.Color("#8B949E"), // GitHub secondary text
		Surface:   lipgloss.Color("#0D1117"), // GitHub canvas default
		Text:      lipgloss.Color("#E6EDF3"), // GitHub foreground
		Border:    lipgloss.Color("#30363D"), // GitHub border

		StatusWorking:     lipgloss.Color("#3FB950"), // Green
		StatusPending:     lipgloss.Color("#8B949E"), // Gray
		StatusInput:       lipgloss.Color("#D29922"), // Yellow
		StatusPaused:      lipgloss.Color("#58A6FF"), // Blue
		StatusComplete:    lipgloss.Color("#A371F7"), // Purple
		StatusError:       lipgloss.Color("#F85149"), // Red
		StatusCreatingPR:  lipgloss.Color("#DB61A2"), // Pink
		StatusStuck:       lipgloss.Color("#F0883E"), // Orange
		StatusTimeout:     lipgloss.Color("#F85149"), // Red
		StatusInterrupted: lipgloss.Color("#D29922"), // Yellow

		DiffAdd:     lipgloss.Color("#3FB950"), // Green
		DiffRemove:  lipgloss.Color("#F85149"), // Red
		DiffHeader:  lipgloss.Color("#58A6FF"), // Blue
		DiffHunk:    lipgloss.Color("#A371F7"), // Purple
		DiffContext: lipgloss.Color("#8B949E"), // Gray

		SearchMatchBg:   lipgloss.Color("#30363D"), // Border
		SearchMatchFg:   lipgloss.Color("#D29922"), // Yellow
		SearchCurrentBg: lipgloss.Color("#58A6FF"), // Blue
		SearchCurrentFg: lipgloss.Color("#0D1117"), // Background

		Blue:   lipgloss.Color("#58A6FF"),
		Yellow: lipgloss.Color("#D29922"),
		Purple: lipgloss.Color("#A371F7"),
		Pink:   lipgloss.Color("#DB61A2"),
		Orange: lipgloss.Color("#F0883E"),
	}
}

// GruvboxPalette returns the Gruvbox theme palette.
// Based on the retro groove color scheme with earthy tones.
func GruvboxPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#83A598"), // Gruvbox aqua
		Secondary: lipgloss.Color("#B8BB26"), // Gruvbox green
		Warning:   lipgloss.Color("#FABD2F"), // Gruvbox yellow
		Error:     lipgloss.Color("#FB4934"), // Gruvbox red
		Muted:     lipgloss.Color("#928374"), // Gruvbox gray
		Surface:   lipgloss.Color("#282828"), // Gruvbox bg0
		Text:      lipgloss.Color("#EBDBB2"), // Gruvbox fg
		Border:    lipgloss.Color("#3C3836"), // Gruvbox bg1

		StatusWorking:     lipgloss.Color("#B8BB26"), // Green
		StatusPending:     lipgloss.Color("#928374"), // Gray
		StatusInput:       lipgloss.Color("#FABD2F"), // Yellow
		StatusPaused:      lipgloss.Color("#83A598"), // Aqua
		StatusComplete:    lipgloss.Color("#D3869B"), // Purple
		StatusError:       lipgloss.Color("#FB4934"), // Red
		StatusCreatingPR:  lipgloss.Color("#D3869B"), // Purple
		StatusStuck:       lipgloss.Color("#FE8019"), // Orange
		StatusTimeout:     lipgloss.Color("#FB4934"), // Red
		StatusInterrupted: lipgloss.Color("#FABD2F"), // Yellow

		DiffAdd:     lipgloss.Color("#B8BB26"), // Green
		DiffRemove:  lipgloss.Color("#FB4934"), // Red
		DiffHeader:  lipgloss.Color("#83A598"), // Aqua
		DiffHunk:    lipgloss.Color("#D3869B"), // Purple
		DiffContext: lipgloss.Color("#928374"), // Gray

		SearchMatchBg:   lipgloss.Color("#3C3836"), // bg1
		SearchMatchFg:   lipgloss.Color("#FABD2F"), // Yellow
		SearchCurrentBg: lipgloss.Color("#FE8019"), // Orange
		SearchCurrentFg: lipgloss.Color("#282828"), // Background

		Blue:   lipgloss.Color("#83A598"), // Aqua (Gruvbox uses aqua instead of blue)
		Yellow: lipgloss.Color("#FABD2F"),
		Purple: lipgloss.Color("#D3869B"),
		Pink:   lipgloss.Color("#D3869B"), // Gruvbox uses purple for pink
		Orange: lipgloss.Color("#FE8019"),
	}
}

// TokyoNightPalette returns the Tokyo Night theme palette.
// A dark theme inspired by the lights of Tokyo at night.
func TokyoNightPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#7AA2F7"), // Tokyo Night blue
		Secondary: lipgloss.Color("#9ECE6A"), // Tokyo Night green
		Warning:   lipgloss.Color("#E0AF68"), // Tokyo Night yellow
		Error:     lipgloss.Color("#F7768E"), // Tokyo Night red
		Muted:     lipgloss.Color("#565F89"), // Tokyo Night comment
		Surface:   lipgloss.Color("#1A1B26"), // Tokyo Night bg
		Text:      lipgloss.Color("#C0CAF5"), // Tokyo Night fg
		Border:    lipgloss.Color("#292E42"), // Tokyo Night bg_highlight

		StatusWorking:     lipgloss.Color("#9ECE6A"), // Green
		StatusPending:     lipgloss.Color("#565F89"), // Gray
		StatusInput:       lipgloss.Color("#E0AF68"), // Yellow
		StatusPaused:      lipgloss.Color("#7DCFFF"), // Cyan
		StatusComplete:    lipgloss.Color("#BB9AF7"), // Purple
		StatusError:       lipgloss.Color("#F7768E"), // Red
		StatusCreatingPR:  lipgloss.Color("#FF9E64"), // Orange
		StatusStuck:       lipgloss.Color("#FF9E64"), // Orange
		StatusTimeout:     lipgloss.Color("#F7768E"), // Red
		StatusInterrupted: lipgloss.Color("#E0AF68"), // Yellow

		DiffAdd:     lipgloss.Color("#9ECE6A"), // Green
		DiffRemove:  lipgloss.Color("#F7768E"), // Red
		DiffHeader:  lipgloss.Color("#7AA2F7"), // Blue
		DiffHunk:    lipgloss.Color("#BB9AF7"), // Purple
		DiffContext: lipgloss.Color("#565F89"), // Gray

		SearchMatchBg:   lipgloss.Color("#292E42"), // bg_highlight
		SearchMatchFg:   lipgloss.Color("#E0AF68"), // Yellow
		SearchCurrentBg: lipgloss.Color("#7AA2F7"), // Blue
		SearchCurrentFg: lipgloss.Color("#1A1B26"), // Background

		Blue:   lipgloss.Color("#7AA2F7"),
		Yellow: lipgloss.Color("#E0AF68"),
		Purple: lipgloss.Color("#BB9AF7"),
		Pink:   lipgloss.Color("#FF007C"), // Tokyo Night magenta
		Orange: lipgloss.Color("#FF9E64"),
	}
}

// CatppuccinPalette returns the Catppuccin Mocha theme palette.
// A soothing pastel theme with warm tones.
func CatppuccinPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#89B4FA"), // Catppuccin blue
		Secondary: lipgloss.Color("#A6E3A1"), // Catppuccin green
		Warning:   lipgloss.Color("#F9E2AF"), // Catppuccin yellow
		Error:     lipgloss.Color("#F38BA8"), // Catppuccin red
		Muted:     lipgloss.Color("#6C7086"), // Catppuccin overlay0
		Surface:   lipgloss.Color("#1E1E2E"), // Catppuccin base
		Text:      lipgloss.Color("#CDD6F4"), // Catppuccin text
		Border:    lipgloss.Color("#313244"), // Catppuccin surface0

		StatusWorking:     lipgloss.Color("#A6E3A1"), // Green
		StatusPending:     lipgloss.Color("#6C7086"), // Gray
		StatusInput:       lipgloss.Color("#F9E2AF"), // Yellow
		StatusPaused:      lipgloss.Color("#94E2D5"), // Teal
		StatusComplete:    lipgloss.Color("#CBA6F7"), // Mauve
		StatusError:       lipgloss.Color("#F38BA8"), // Red
		StatusCreatingPR:  lipgloss.Color("#F5C2E7"), // Pink
		StatusStuck:       lipgloss.Color("#FAB387"), // Peach
		StatusTimeout:     lipgloss.Color("#F38BA8"), // Red
		StatusInterrupted: lipgloss.Color("#F9E2AF"), // Yellow

		DiffAdd:     lipgloss.Color("#A6E3A1"), // Green
		DiffRemove:  lipgloss.Color("#F38BA8"), // Red
		DiffHeader:  lipgloss.Color("#89B4FA"), // Blue
		DiffHunk:    lipgloss.Color("#CBA6F7"), // Mauve
		DiffContext: lipgloss.Color("#6C7086"), // Gray

		SearchMatchBg:   lipgloss.Color("#313244"), // surface0
		SearchMatchFg:   lipgloss.Color("#F9E2AF"), // Yellow
		SearchCurrentBg: lipgloss.Color("#89B4FA"), // Blue
		SearchCurrentFg: lipgloss.Color("#1E1E2E"), // Background

		Blue:   lipgloss.Color("#89B4FA"),
		Yellow: lipgloss.Color("#F9E2AF"),
		Purple: lipgloss.Color("#CBA6F7"),
		Pink:   lipgloss.Color("#F5C2E7"),
		Orange: lipgloss.Color("#FAB387"),
	}
}

// SynthwavePalette returns the Synthwave '84 theme palette.
// A retro neon aesthetic inspired by 80s synthwave.
func SynthwavePalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#FF7EDB"), // Synthwave pink
		Secondary: lipgloss.Color("#72F1B8"), // Synthwave green
		Warning:   lipgloss.Color("#FEDE5D"), // Synthwave yellow
		Error:     lipgloss.Color("#FE4450"), // Synthwave red
		Muted:     lipgloss.Color("#848BBD"), // Synthwave comment
		Surface:   lipgloss.Color("#262335"), // Synthwave background
		Text:      lipgloss.Color("#FFFFFF"), // White text
		Border:    lipgloss.Color("#34294F"), // Synthwave border

		StatusWorking:     lipgloss.Color("#72F1B8"), // Green
		StatusPending:     lipgloss.Color("#848BBD"), // Gray
		StatusInput:       lipgloss.Color("#FEDE5D"), // Yellow
		StatusPaused:      lipgloss.Color("#36F9F6"), // Cyan
		StatusComplete:    lipgloss.Color("#FF7EDB"), // Pink
		StatusError:       lipgloss.Color("#FE4450"), // Red
		StatusCreatingPR:  lipgloss.Color("#B893CE"), // Purple
		StatusStuck:       lipgloss.Color("#F97E72"), // Orange
		StatusTimeout:     lipgloss.Color("#FE4450"), // Red
		StatusInterrupted: lipgloss.Color("#FEDE5D"), // Yellow

		DiffAdd:     lipgloss.Color("#72F1B8"), // Green
		DiffRemove:  lipgloss.Color("#FE4450"), // Red
		DiffHeader:  lipgloss.Color("#36F9F6"), // Cyan
		DiffHunk:    lipgloss.Color("#FF7EDB"), // Pink
		DiffContext: lipgloss.Color("#848BBD"), // Gray

		SearchMatchBg:   lipgloss.Color("#34294F"), // Border
		SearchMatchFg:   lipgloss.Color("#FEDE5D"), // Yellow
		SearchCurrentBg: lipgloss.Color("#FF7EDB"), // Pink
		SearchCurrentFg: lipgloss.Color("#262335"), // Background

		Blue:   lipgloss.Color("#36F9F6"), // Cyan
		Yellow: lipgloss.Color("#FEDE5D"),
		Purple: lipgloss.Color("#B893CE"),
		Pink:   lipgloss.Color("#FF7EDB"),
		Orange: lipgloss.Color("#F97E72"),
	}
}

// AyuPalette returns the Ayu Dark theme palette.
// A clean and elegant dark theme with warm accents.
func AyuPalette() *ColorPalette {
	return &ColorPalette{
		Primary:   lipgloss.Color("#39BAE6"), // Ayu blue
		Secondary: lipgloss.Color("#7FD962"), // Ayu green
		Warning:   lipgloss.Color("#E6B450"), // Ayu yellow
		Error:     lipgloss.Color("#D95757"), // Ayu red
		Muted:     lipgloss.Color("#626A73"), // Ayu comment
		Surface:   lipgloss.Color("#0D1017"), // Ayu background
		Text:      lipgloss.Color("#BFBDB6"), // Ayu foreground
		Border:    lipgloss.Color("#1D222C"), // Ayu line

		StatusWorking:     lipgloss.Color("#7FD962"), // Green
		StatusPending:     lipgloss.Color("#626A73"), // Gray
		StatusInput:       lipgloss.Color("#E6B450"), // Yellow
		StatusPaused:      lipgloss.Color("#39BAE6"), // Blue
		StatusComplete:    lipgloss.Color("#D2A6FF"), // Purple
		StatusError:       lipgloss.Color("#D95757"), // Red
		StatusCreatingPR:  lipgloss.Color("#F28779"), // Orange (Ayu uses this as accent)
		StatusStuck:       lipgloss.Color("#FF8F40"), // Orange
		StatusTimeout:     lipgloss.Color("#D95757"), // Red
		StatusInterrupted: lipgloss.Color("#E6B450"), // Yellow

		DiffAdd:     lipgloss.Color("#7FD962"), // Green
		DiffRemove:  lipgloss.Color("#D95757"), // Red
		DiffHeader:  lipgloss.Color("#39BAE6"), // Blue
		DiffHunk:    lipgloss.Color("#D2A6FF"), // Purple
		DiffContext: lipgloss.Color("#626A73"), // Gray

		SearchMatchBg:   lipgloss.Color("#1D222C"), // Line
		SearchMatchFg:   lipgloss.Color("#E6B450"), // Yellow
		SearchCurrentBg: lipgloss.Color("#39BAE6"), // Blue
		SearchCurrentFg: lipgloss.Color("#0D1017"), // Background

		Blue:   lipgloss.Color("#39BAE6"),
		Yellow: lipgloss.Color("#E6B450"),
		Purple: lipgloss.Color("#D2A6FF"),
		Pink:   lipgloss.Color("#F28779"), // Ayu uses coral/salmon
		Orange: lipgloss.Color("#FF8F40"),
	}
}

// GetPalette returns the color palette for the given theme name.
// Checks custom themes first, then falls back to built-in themes.
// Returns the default palette for unknown theme names.
func GetPalette(name ThemeName) *ColorPalette {
	// Check custom themes first
	if custom := GetCustomTheme(name); custom != nil {
		return custom.ToPalette()
	}

	// Fall back to built-in themes
	switch name {
	case ThemeMonokai:
		return MonokaiPalette()
	case ThemeDracula:
		return DraculaPalette()
	case ThemeNord:
		return NordPalette()
	case ThemeClaudeCode:
		return ClaudeCodePalette()
	case ThemeSolarizedDark:
		return SolarizedDarkPalette()
	case ThemeSolarizedLight:
		return SolarizedLightPalette()
	case ThemeOneDark:
		return OneDarkPalette()
	case ThemeGitHubDark:
		return GitHubDarkPalette()
	case ThemeGruvbox:
		return GruvboxPalette()
	case ThemeTokyoNight:
		return TokyoNightPalette()
	case ThemeCatppuccin:
		return CatppuccinPalette()
	case ThemeSynthwave:
		return SynthwavePalette()
	case ThemeAyu:
		return AyuPalette()
	default:
		return DefaultPalette()
	}
}

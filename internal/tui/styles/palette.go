package styles

import (
	"slices"

	"github.com/charmbracelet/lipgloss"
)

// ThemeName represents a named color theme.
type ThemeName string

// Available theme names.
const (
	ThemeDefault ThemeName = "default" // Purple/green dark theme (current)
	ThemeMonokai ThemeName = "monokai" // Classic Monokai editor colors
	ThemeDracula ThemeName = "dracula" // Dracula theme colors
	ThemeNord    ThemeName = "nord"    // Nord theme - cool blue-gray
)

// ValidThemes returns all valid theme names.
func ValidThemes() []string {
	return []string{
		string(ThemeDefault),
		string(ThemeMonokai),
		string(ThemeDracula),
		string(ThemeNord),
	}
}

// IsValidTheme checks if a theme name is valid.
func IsValidTheme(name string) bool {
	return slices.Contains(ValidThemes(), name)
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

// GetPalette returns the color palette for the given theme name.
// Returns the default palette for unknown theme names.
func GetPalette(name ThemeName) *ColorPalette {
	switch name {
	case ThemeMonokai:
		return MonokaiPalette()
	case ThemeDracula:
		return DraculaPalette()
	case ThemeNord:
		return NordPalette()
	default:
		return DefaultPalette()
	}
}

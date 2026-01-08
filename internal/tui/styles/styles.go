package styles

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	PrimaryColor   = lipgloss.Color("#7C3AED") // Purple
	SecondaryColor = lipgloss.Color("#10B981") // Green
	WarningColor   = lipgloss.Color("#F59E0B") // Amber
	ErrorColor     = lipgloss.Color("#EF4444") // Red
	MutedColor     = lipgloss.Color("#9CA3AF") // Gray (brighter for readability)
	SurfaceColor   = lipgloss.Color("#1F2937") // Dark surface
	TextColor      = lipgloss.Color("#F9FAFB") // Light text
	BorderColor    = lipgloss.Color("#4B5563") // Slightly dimmer gray for borders

	// Convenience styles for colors
	Primary   = lipgloss.NewStyle().Foreground(PrimaryColor)
	Secondary = lipgloss.NewStyle().Foreground(SecondaryColor)
	Warning   = lipgloss.NewStyle().Foreground(WarningColor)
	Error     = lipgloss.NewStyle().Foreground(ErrorColor)
	Muted     = lipgloss.NewStyle().Foreground(MutedColor)
	Surface   = lipgloss.NewStyle().Background(SurfaceColor)
	Text      = lipgloss.NewStyle().Foreground(TextColor)

	// Status colors
	StatusWorking    = lipgloss.Color("#10B981") // Green
	StatusPending    = lipgloss.Color("#9CA3AF") // Gray (brighter for readability)
	StatusInput      = lipgloss.Color("#F59E0B") // Amber
	StatusPaused     = lipgloss.Color("#60A5FA") // Blue (brighter for readability)
	StatusComplete   = lipgloss.Color("#A78BFA") // Purple (brighter for readability)
	StatusError      = lipgloss.Color("#EF4444") // Red
	StatusCreatingPR = lipgloss.Color("#F472B6") // Pink (brighter for readability)

	// Base styles
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	// Tab styles
	TabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextColor).
			Background(PrimaryColor).
			Padding(0, 2)

	TabInactive = lipgloss.NewStyle().
			Foreground(MutedColor).
			Padding(0, 2)

	TabInputNeeded = lipgloss.NewStyle().
			Bold(true).
			Foreground(TextColor).
			Background(WarningColor).
			Padding(0, 2)

	// Status badge styles
	StatusBadge = lipgloss.NewStyle().
			Padding(0, 1).
			MarginRight(1)

	// Content area
	ContentBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	// Help bar
	HelpBar = lipgloss.NewStyle().
		Foreground(MutedColor).
		MarginTop(1)

	HelpKey = lipgloss.NewStyle().
		Bold(true).
		Foreground(SecondaryColor)

	// Output area
	OutputArea = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor)

	// Header
	Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(BorderColor).
		MarginBottom(1).
		PaddingBottom(1)

	// Footer / status bar
	StatusBar = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(SurfaceColor).
			Padding(0, 1)

	// Instance info
	InstanceInfo = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginBottom(1)

	// Sidebar styles
	Sidebar = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Padding(1, 1)

	SidebarItem = lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(0)

	SidebarItemActive = lipgloss.NewStyle().
				Bold(true).
				Foreground(TextColor).
				Background(PrimaryColor).
				Padding(0, 1).
				MarginBottom(0)

	SidebarItemInputNeeded = lipgloss.NewStyle().
				Bold(true).
				Foreground(TextColor).
				Background(WarningColor).
				Padding(0, 1).
				MarginBottom(0)

	SidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1)

	SidebarSectionTitle = lipgloss.NewStyle().
				Foreground(MutedColor).
				MarginBottom(0)

	StatusDot = lipgloss.NewStyle().
			MarginRight(1)

	// Error message
	ErrorMsg = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	// Success message
	SuccessMsg = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	// Warning message
	WarningMsg = lipgloss.NewStyle().
			Foreground(WarningColor).
			Bold(true)

	// Conflict warning banner
	ConflictBanner = lipgloss.NewStyle().
			Foreground(TextColor).
			Background(WarningColor).
			Bold(true).
			Padding(0, 1)

	// Template dropdown styles
	DropdownContainer = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor).
				Padding(0, 1).
				MarginTop(1)

	DropdownItem = lipgloss.NewStyle().
			Foreground(TextColor).
			Padding(0, 1)

	DropdownItemSelected = lipgloss.NewStyle().
				Foreground(TextColor).
				Background(PrimaryColor).
				Bold(true).
				Padding(0, 1)

	DropdownCommand = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	DropdownName = lipgloss.NewStyle().
			Foreground(MutedColor)

	// Diff syntax highlighting styles
	DiffAdd = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#22C55E")) // Green for additions

	DiffRemove = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#EF4444")) // Red for removals

	DiffHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#60A5FA")). // Blue for diff headers
			Bold(true)

	DiffHunk = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A78BFA")) // Purple for @@ hunk markers

	DiffContext = lipgloss.NewStyle().
			Foreground(MutedColor) // Gray for context lines

	// Search styles
	SearchBar = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(0, 1).
			MarginTop(1)

	SearchInput = lipgloss.NewStyle().
			Foreground(TextColor)

	SearchPrompt = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			Bold(true)

	SearchMatch = lipgloss.NewStyle().
			Background(lipgloss.Color("#854D0E")). // Dark yellow/amber background
			Foreground(lipgloss.Color("#FEF3C7"))  // Light cream text for contrast

	SearchCurrentMatch = lipgloss.NewStyle().
				Background(lipgloss.Color("#C2410C")). // Dark orange for current match
				Foreground(lipgloss.Color("#FFF7ED")).  // Light orange-white text
				Bold(true)

	SearchInfo = lipgloss.NewStyle().
			Foreground(MutedColor).
			MarginLeft(2)

	// Filter styles
	FilterBar = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(PrimaryColor).
			Padding(1, 2)

	FilterCategory = lipgloss.NewStyle().
			Foreground(TextColor).
			MarginRight(2)

	FilterCategoryEnabled = lipgloss.NewStyle().
				Foreground(SecondaryColor).
				Bold(true).
				MarginRight(2)

	FilterCategoryDisabled = lipgloss.NewStyle().
				Foreground(MutedColor).
				MarginRight(2)

	FilterCheckbox = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	FilterCheckboxEmpty = lipgloss.NewStyle().
				Foreground(MutedColor)
)

// StatusColor returns the color for a given status
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "working":
		return StatusWorking
	case "pending":
		return StatusPending
	case "waiting_input":
		return StatusInput
	case "paused":
		return StatusPaused
	case "completed":
		return StatusComplete
	case "error":
		return StatusError
	case "creating_pr":
		return StatusCreatingPR
	default:
		return MutedColor
	}
}

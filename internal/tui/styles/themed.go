package styles

import "github.com/charmbracelet/lipgloss"

// ThemedStyles contains all the lipgloss styles built from a color palette.
// This allows styles to be regenerated when the theme changes.
type ThemedStyles struct {
	// Colors from the palette
	PrimaryColor   lipgloss.Color
	SecondaryColor lipgloss.Color
	WarningColor   lipgloss.Color
	ErrorColor     lipgloss.Color
	MutedColor     lipgloss.Color
	SurfaceColor   lipgloss.Color
	TextColor      lipgloss.Color
	BorderColor    lipgloss.Color

	// Additional colors
	GreenColor  lipgloss.Color
	RedColor    lipgloss.Color
	BlueColor   lipgloss.Color
	YellowColor lipgloss.Color
	PurpleColor lipgloss.Color

	// Status colors
	StatusWorking     lipgloss.Color
	StatusPending     lipgloss.Color
	StatusPreparing   lipgloss.Color
	StatusInput       lipgloss.Color
	StatusPaused      lipgloss.Color
	StatusComplete    lipgloss.Color
	StatusError       lipgloss.Color
	StatusCreatingPR  lipgloss.Color
	StatusStuck       lipgloss.Color
	StatusTimeout     lipgloss.Color
	StatusInterrupted lipgloss.Color

	// Convenience styles for colors
	Primary   lipgloss.Style
	Secondary lipgloss.Style
	Warning   lipgloss.Style
	Error     lipgloss.Style
	Muted     lipgloss.Style
	Surface   lipgloss.Style
	Text      lipgloss.Style

	// Base styles
	Title    lipgloss.Style
	Subtitle lipgloss.Style

	// Tab styles
	TabActive      lipgloss.Style
	TabInactive    lipgloss.Style
	TabInputNeeded lipgloss.Style

	// Status badge
	StatusBadge lipgloss.Style

	// Content area
	ContentBox lipgloss.Style

	// Help bar
	HelpBar lipgloss.Style
	HelpKey lipgloss.Style

	// Mode badges
	ModeBadgeNormal   lipgloss.Style
	ModeBadgeInput    lipgloss.Style
	ModeBadgeTerminal lipgloss.Style
	ModeBadgeCommand  lipgloss.Style
	ModeBadgeSearch   lipgloss.Style
	ModeBadgeFilter   lipgloss.Style
	ModeBadgeDiff     lipgloss.Style

	// Output area
	OutputArea lipgloss.Style

	// Header
	Header lipgloss.Style

	// Footer / status bar
	StatusBar lipgloss.Style

	// Instance info
	InstanceInfo lipgloss.Style

	// Sidebar styles
	Sidebar             lipgloss.Style
	SidebarItem         lipgloss.Style
	SidebarItemActive   lipgloss.Style
	SidebarTitle        lipgloss.Style
	SidebarSectionTitle lipgloss.Style
	StatusDot           lipgloss.Style

	// Messages
	ErrorMsg   lipgloss.Style
	SuccessMsg lipgloss.Style
	WarningMsg lipgloss.Style

	// Conflict banner
	ConflictBanner lipgloss.Style

	// Dropdown styles
	DropdownContainer    lipgloss.Style
	DropdownItem         lipgloss.Style
	DropdownItemSelected lipgloss.Style
	DropdownCommand      lipgloss.Style
	DropdownName         lipgloss.Style

	// Diff syntax highlighting styles
	DiffAdd     lipgloss.Style
	DiffRemove  lipgloss.Style
	DiffHeader  lipgloss.Style
	DiffHunk    lipgloss.Style
	DiffContext lipgloss.Style

	// Search styles
	SearchBar          lipgloss.Style
	SearchInput        lipgloss.Style
	SearchPrompt       lipgloss.Style
	SearchMatch        lipgloss.Style
	SearchCurrentMatch lipgloss.Style
	SearchInfo         lipgloss.Style

	// Filter styles
	FilterBar              lipgloss.Style
	FilterCategory         lipgloss.Style
	FilterCategoryEnabled  lipgloss.Style
	FilterCategoryDisabled lipgloss.Style
	FilterCheckbox         lipgloss.Style
	FilterCheckboxEmpty    lipgloss.Style

	// Terminal pane styles
	TerminalPaneBorder        lipgloss.Style
	TerminalPaneBorderFocused lipgloss.Style
	TerminalHeader            lipgloss.Style
	TerminalFocusIndicator    lipgloss.Style

	// Session type colors
	SessionTypePlanColor       lipgloss.Color
	SessionTypeUltraPlanColor  lipgloss.Color
	SessionTypeTripleShotColor lipgloss.Color
}

// NewThemedStyles creates a ThemedStyles from the given color palette.
func NewThemedStyles(p *ColorPalette) *ThemedStyles {
	s := &ThemedStyles{
		// Store colors for direct access
		PrimaryColor:   p.Primary,
		SecondaryColor: p.Secondary,
		WarningColor:   p.Warning,
		ErrorColor:     p.Error,
		MutedColor:     p.Muted,
		SurfaceColor:   p.Surface,
		TextColor:      p.Text,
		BorderColor:    p.Border,

		GreenColor:  p.Secondary,
		RedColor:    p.Error,
		BlueColor:   p.Blue,
		YellowColor: p.Yellow,
		PurpleColor: p.Purple,

		// Status colors
		StatusWorking:     p.StatusWorking,
		StatusPending:     p.StatusPending,
		StatusPreparing:   p.StatusPreparing,
		StatusInput:       p.StatusInput,
		StatusPaused:      p.StatusPaused,
		StatusComplete:    p.StatusComplete,
		StatusError:       p.StatusError,
		StatusCreatingPR:  p.StatusCreatingPR,
		StatusStuck:       p.StatusStuck,
		StatusTimeout:     p.StatusTimeout,
		StatusInterrupted: p.StatusInterrupted,

		// Session type colors
		SessionTypePlanColor:       p.Purple,
		SessionTypeUltraPlanColor:  p.Yellow,
		SessionTypeTripleShotColor: p.Blue,
	}

	// Build all the styles
	s.Primary = lipgloss.NewStyle().Foreground(p.Primary)
	s.Secondary = lipgloss.NewStyle().Foreground(p.Secondary)
	s.Warning = lipgloss.NewStyle().Foreground(p.Warning)
	s.Error = lipgloss.NewStyle().Foreground(p.Error)
	s.Muted = lipgloss.NewStyle().Foreground(p.Muted)
	s.Surface = lipgloss.NewStyle().Background(p.Surface)
	s.Text = lipgloss.NewStyle().Foreground(p.Text)

	s.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Primary).
		MarginBottom(1)

	s.Subtitle = lipgloss.NewStyle().
		Foreground(p.Muted).
		Italic(true)

	s.TabActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Primary).
		Padding(0, 2)

	s.TabInactive = lipgloss.NewStyle().
		Foreground(p.Muted).
		Padding(0, 2)

	s.TabInputNeeded = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Warning).
		Padding(0, 2)

	s.StatusBadge = lipgloss.NewStyle().
		Padding(0, 1).
		MarginRight(1)

	s.ContentBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(1, 2)

	s.HelpBar = lipgloss.NewStyle().
		Foreground(p.Muted).
		MarginTop(1)

	s.HelpKey = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Secondary)

	s.ModeBadgeNormal = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Muted).
		Background(p.Surface).
		Padding(0, 1)

	s.ModeBadgeInput = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Warning).
		Padding(0, 1)

	s.ModeBadgeTerminal = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Secondary).
		Padding(0, 1)

	s.ModeBadgeCommand = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Primary).
		Padding(0, 1)

	s.ModeBadgeSearch = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Primary).
		Padding(0, 1)

	s.ModeBadgeFilter = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Primary).
		Padding(0, 1)

	s.ModeBadgeDiff = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Blue).
		Padding(0, 1)

	s.OutputArea = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border)

	s.Header = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Primary).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(p.Border).
		MarginBottom(1).
		PaddingBottom(1)

	s.StatusBar = lipgloss.NewStyle().
		Foreground(p.Text).
		Background(p.Surface).
		Padding(0, 1)

	s.InstanceInfo = lipgloss.NewStyle().
		Foreground(p.Muted).
		MarginBottom(1)

	s.Sidebar = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(1, 1)

	s.SidebarItem = lipgloss.NewStyle().
		Padding(0, 1).
		MarginBottom(0)

	s.SidebarItemActive = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Text).
		Background(p.Primary).
		Padding(0, 1).
		MarginBottom(0)

	s.SidebarTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(p.Primary)

	s.SidebarSectionTitle = lipgloss.NewStyle().
		Foreground(p.Muted).
		MarginBottom(0)

	s.StatusDot = lipgloss.NewStyle().
		MarginRight(1)

	s.ErrorMsg = lipgloss.NewStyle().
		Foreground(p.Error).
		Bold(true)

	s.SuccessMsg = lipgloss.NewStyle().
		Foreground(p.Secondary).
		Bold(true)

	s.WarningMsg = lipgloss.NewStyle().
		Foreground(p.Warning).
		Bold(true)

	s.ConflictBanner = lipgloss.NewStyle().
		Foreground(p.Text).
		Background(p.Warning).
		Bold(true).
		Padding(0, 1)

	s.DropdownContainer = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Primary).
		Padding(0, 1).
		MarginTop(1)

	s.DropdownItem = lipgloss.NewStyle().
		Foreground(p.Text).
		Padding(0, 1)

	s.DropdownItemSelected = lipgloss.NewStyle().
		Foreground(p.Text).
		Background(p.Primary).
		Bold(true).
		Padding(0, 1)

	s.DropdownCommand = lipgloss.NewStyle().
		Foreground(p.Secondary).
		Bold(true)

	s.DropdownName = lipgloss.NewStyle().
		Foreground(p.Muted)

	s.DiffAdd = lipgloss.NewStyle().
		Foreground(p.DiffAdd)

	s.DiffRemove = lipgloss.NewStyle().
		Foreground(p.DiffRemove)

	s.DiffHeader = lipgloss.NewStyle().
		Foreground(p.DiffHeader).
		Bold(true)

	s.DiffHunk = lipgloss.NewStyle().
		Foreground(p.DiffHunk)

	s.DiffContext = lipgloss.NewStyle().
		Foreground(p.DiffContext)

	s.SearchBar = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Primary).
		Padding(0, 1).
		MarginTop(1)

	s.SearchInput = lipgloss.NewStyle().
		Foreground(p.Text)

	s.SearchPrompt = lipgloss.NewStyle().
		Foreground(p.Secondary).
		Bold(true)

	s.SearchMatch = lipgloss.NewStyle().
		Background(p.SearchMatchBg).
		Foreground(p.SearchMatchFg)

	s.SearchCurrentMatch = lipgloss.NewStyle().
		Background(p.SearchCurrentBg).
		Foreground(p.SearchCurrentFg).
		Bold(true)

	s.SearchInfo = lipgloss.NewStyle().
		Foreground(p.Muted).
		MarginLeft(2)

	s.FilterBar = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Primary).
		Padding(1, 2)

	s.FilterCategory = lipgloss.NewStyle().
		Foreground(p.Text).
		MarginRight(2)

	s.FilterCategoryEnabled = lipgloss.NewStyle().
		Foreground(p.Secondary).
		Bold(true).
		MarginRight(2)

	s.FilterCategoryDisabled = lipgloss.NewStyle().
		Foreground(p.Muted).
		MarginRight(2)

	s.FilterCheckbox = lipgloss.NewStyle().
		Foreground(p.Secondary)

	s.FilterCheckboxEmpty = lipgloss.NewStyle().
		Foreground(p.Muted)

	s.TerminalPaneBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Border).
		Padding(0, 1)

	s.TerminalPaneBorderFocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(p.Secondary).
		Padding(0, 1)

	s.TerminalHeader = lipgloss.NewStyle().
		Foreground(p.Muted)

	s.TerminalFocusIndicator = lipgloss.NewStyle().
		Background(p.Secondary).
		Foreground(p.Text).
		Bold(true)

	return s
}

// StatusColor returns the color for a given status using the themed palette.
func (s *ThemedStyles) StatusColor(status string) lipgloss.Color {
	switch status {
	case "working":
		return s.StatusWorking
	case "pending":
		return s.StatusPending
	case "preparing":
		return s.StatusPreparing
	case "waiting_input":
		return s.StatusInput
	case "paused":
		return s.StatusPaused
	case "completed":
		return s.StatusComplete
	case "error":
		return s.StatusError
	case "creating_pr":
		return s.StatusCreatingPR
	case "stuck":
		return s.StatusStuck
	case "timeout":
		return s.StatusTimeout
	case "interrupted":
		return s.StatusInterrupted
	default:
		return s.MutedColor
	}
}

// SessionTypeColor returns the color for a session type using the themed palette.
func (s *ThemedStyles) SessionTypeColor(sessionType string) lipgloss.Color {
	switch sessionType {
	case "plan", "plan_multi":
		return s.SessionTypePlanColor
	case "ultraplan":
		return s.SessionTypeUltraPlanColor
	case "tripleshot":
		return s.SessionTypeTripleShotColor
	default:
		return s.MutedColor
	}
}

// activeTheme holds the currently active themed styles.
// This is set via SetActiveTheme and provides backwards compatibility
// with code that uses the global style variables.
var activeTheme *ThemedStyles

func init() {
	// Initialize with default theme
	activeTheme = NewThemedStyles(DefaultPalette())
}

// SetActiveTheme updates the active theme to the specified theme name.
// This updates all the global style variables to use the new theme colors.
//
// Note: This function is not thread-safe. It is designed to be called only
// from the Bubble Tea event loop, which runs on a single goroutine.
func SetActiveTheme(name ThemeName) {
	palette := GetPalette(name)
	activeTheme = NewThemedStyles(palette)
	syncGlobalStyles()
}

// GetActiveTheme returns the currently active themed styles.
func GetActiveTheme() *ThemedStyles {
	return activeTheme
}

// syncGlobalStyles updates the global style variables to match the active theme.
// This maintains backwards compatibility with existing code that uses
// the package-level style variables directly.
func syncGlobalStyles() {
	// Update colors
	PrimaryColor = activeTheme.PrimaryColor
	SecondaryColor = activeTheme.SecondaryColor
	WarningColor = activeTheme.WarningColor
	ErrorColor = activeTheme.ErrorColor
	MutedColor = activeTheme.MutedColor
	SurfaceColor = activeTheme.SurfaceColor
	TextColor = activeTheme.TextColor
	BorderColor = activeTheme.BorderColor

	GreenColor = activeTheme.GreenColor
	RedColor = activeTheme.RedColor
	BlueColor = activeTheme.BlueColor
	YellowColor = activeTheme.YellowColor
	PurpleColor = activeTheme.PurpleColor

	// Update status colors
	StatusWorking = activeTheme.StatusWorking
	StatusPending = activeTheme.StatusPending
	StatusPreparing = activeTheme.StatusPreparing
	StatusInput = activeTheme.StatusInput
	StatusPaused = activeTheme.StatusPaused
	StatusComplete = activeTheme.StatusComplete
	StatusError = activeTheme.StatusError
	StatusCreatingPR = activeTheme.StatusCreatingPR
	StatusStuck = activeTheme.StatusStuck
	StatusTimeout = activeTheme.StatusTimeout
	StatusInterrupted = activeTheme.StatusInterrupted

	// Update session type colors
	SessionTypePlanColor = activeTheme.SessionTypePlanColor
	SessionTypeUltraPlanColor = activeTheme.SessionTypeUltraPlanColor
	SessionTypeTripleShotColor = activeTheme.SessionTypeTripleShotColor

	// Update convenience styles
	Primary = activeTheme.Primary
	Secondary = activeTheme.Secondary
	Warning = activeTheme.Warning
	Error = activeTheme.Error
	Muted = activeTheme.Muted
	Surface = activeTheme.Surface
	Text = activeTheme.Text

	// Update base styles
	Title = activeTheme.Title
	Subtitle = activeTheme.Subtitle

	// Update tab styles
	TabActive = activeTheme.TabActive
	TabInactive = activeTheme.TabInactive
	TabInputNeeded = activeTheme.TabInputNeeded

	// Update status badge
	StatusBadge = activeTheme.StatusBadge

	// Update content box
	ContentBox = activeTheme.ContentBox

	// Update help styles
	HelpBar = activeTheme.HelpBar
	HelpKey = activeTheme.HelpKey

	// Update mode badges
	ModeBadgeNormal = activeTheme.ModeBadgeNormal
	ModeBadgeInput = activeTheme.ModeBadgeInput
	ModeBadgeTerminal = activeTheme.ModeBadgeTerminal
	ModeBadgeCommand = activeTheme.ModeBadgeCommand
	ModeBadgeSearch = activeTheme.ModeBadgeSearch
	ModeBadgeFilter = activeTheme.ModeBadgeFilter
	ModeBadgeDiff = activeTheme.ModeBadgeDiff

	// Update output area
	OutputArea = activeTheme.OutputArea

	// Update header
	Header = activeTheme.Header

	// Update status bar
	StatusBar = activeTheme.StatusBar

	// Update instance info
	InstanceInfo = activeTheme.InstanceInfo

	// Update sidebar styles
	Sidebar = activeTheme.Sidebar
	SidebarItem = activeTheme.SidebarItem
	SidebarItemActive = activeTheme.SidebarItemActive
	SidebarTitle = activeTheme.SidebarTitle
	SidebarSectionTitle = activeTheme.SidebarSectionTitle
	StatusDot = activeTheme.StatusDot

	// Update messages
	ErrorMsg = activeTheme.ErrorMsg
	SuccessMsg = activeTheme.SuccessMsg
	WarningMsg = activeTheme.WarningMsg

	// Update conflict banner
	ConflictBanner = activeTheme.ConflictBanner

	// Update dropdown styles
	DropdownContainer = activeTheme.DropdownContainer
	DropdownItem = activeTheme.DropdownItem
	DropdownItemSelected = activeTheme.DropdownItemSelected
	DropdownCommand = activeTheme.DropdownCommand
	DropdownName = activeTheme.DropdownName

	// Update diff styles
	DiffAdd = activeTheme.DiffAdd
	DiffRemove = activeTheme.DiffRemove
	DiffHeader = activeTheme.DiffHeader
	DiffHunk = activeTheme.DiffHunk
	DiffContext = activeTheme.DiffContext

	// Update search styles
	SearchBar = activeTheme.SearchBar
	SearchInput = activeTheme.SearchInput
	SearchPrompt = activeTheme.SearchPrompt
	SearchMatch = activeTheme.SearchMatch
	SearchCurrentMatch = activeTheme.SearchCurrentMatch
	SearchInfo = activeTheme.SearchInfo

	// Update filter styles
	FilterBar = activeTheme.FilterBar
	FilterCategory = activeTheme.FilterCategory
	FilterCategoryEnabled = activeTheme.FilterCategoryEnabled
	FilterCategoryDisabled = activeTheme.FilterCategoryDisabled
	FilterCheckbox = activeTheme.FilterCheckbox
	FilterCheckboxEmpty = activeTheme.FilterCheckboxEmpty

	// Update terminal pane styles
	TerminalPaneBorder = activeTheme.TerminalPaneBorder
	TerminalPaneBorderFocused = activeTheme.TerminalPaneBorderFocused
	TerminalHeader = activeTheme.TerminalHeader
	TerminalFocusIndicator = activeTheme.TerminalFocusIndicator
}

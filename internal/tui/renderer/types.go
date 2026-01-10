// Package renderer provides interfaces and types for TUI rendering components.
// It establishes the rendering abstraction layer used throughout the TUI system.
package renderer

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// Renderer is the base interface for all rendering components.
// Components implementing this interface can produce a string representation
// of their visual output given available dimensions.
type Renderer interface {
	// Render produces the visual output for this component.
	// The ctx parameter provides dimensions, styles, and theme configuration.
	Render(ctx RenderContext) string
}

// RenderContext contains all contextual information needed for rendering.
// It encapsulates dimensions, active styles, and theme configuration to
// provide a consistent rendering environment across all components.
type RenderContext struct {
	// Width is the available horizontal space in characters
	Width int

	// Height is the available vertical space in lines
	Height int

	// Styles provides access to the active style configuration
	Styles *StyleConfig

	// Theme holds the current theme configuration
	Theme *ThemeConfig

	// Focused indicates whether this component or its parent has focus
	Focused bool
}

// NewRenderContext creates a new RenderContext with the specified dimensions.
// It initializes with default styles and theme configuration.
func NewRenderContext(width, height int) RenderContext {
	return RenderContext{
		Width:   width,
		Height:  height,
		Styles:  DefaultStyleConfig(),
		Theme:   DefaultThemeConfig(),
		Focused: false,
	}
}

// WithFocus returns a copy of the context with the focus state set.
func (ctx RenderContext) WithFocus(focused bool) RenderContext {
	ctx.Focused = focused
	return ctx
}

// WithDimensions returns a copy of the context with updated dimensions.
func (ctx RenderContext) WithDimensions(width, height int) RenderContext {
	ctx.Width = width
	ctx.Height = height
	return ctx
}

// WithWidth returns a copy of the context with updated width only.
func (ctx RenderContext) WithWidth(width int) RenderContext {
	ctx.Width = width
	return ctx
}

// WithHeight returns a copy of the context with updated height only.
func (ctx RenderContext) WithHeight(height int) RenderContext {
	ctx.Height = height
	return ctx
}

// StyleConfig holds references to the active lipgloss styles.
// This allows renderers to use consistent styling across the application.
type StyleConfig struct {
	// Border styles
	Border       lipgloss.Style
	BorderActive lipgloss.Style

	// Content styles
	Content lipgloss.Style
	Header  lipgloss.Style
	Footer  lipgloss.Style

	// Text styles
	Title    lipgloss.Style
	Subtitle lipgloss.Style
	Muted    lipgloss.Style
	Error    lipgloss.Style
	Warning  lipgloss.Style
	Success  lipgloss.Style

	// Interactive styles
	Selected lipgloss.Style
	Focused  lipgloss.Style
}

// DefaultStyleConfig returns a StyleConfig initialized with the default styles
// from the styles package.
func DefaultStyleConfig() *StyleConfig {
	return &StyleConfig{
		Border:       styles.ContentBox,
		BorderActive: styles.ContentBox.BorderForeground(styles.PrimaryColor),
		Content:      lipgloss.NewStyle(),
		Header:       styles.Header,
		Footer:       styles.StatusBar,
		Title:        styles.Title,
		Subtitle:     styles.Subtitle,
		Muted:        styles.Muted,
		Error:        styles.ErrorMsg,
		Warning:      styles.WarningMsg,
		Success:      styles.SuccessMsg,
		Selected:     styles.SidebarItemActive,
		Focused:      lipgloss.NewStyle().Foreground(styles.PrimaryColor),
	}
}

// ThemeConfig holds color and appearance configuration for the current theme.
type ThemeConfig struct {
	// Primary colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color

	// Semantic colors
	Error   lipgloss.Color
	Warning lipgloss.Color
	Success lipgloss.Color
	Info    lipgloss.Color

	// Surface colors
	Background lipgloss.Color
	Surface    lipgloss.Color
	Border     lipgloss.Color

	// Text colors
	Text      lipgloss.Color
	TextMuted lipgloss.Color
}

// DefaultThemeConfig returns a ThemeConfig with the default color scheme
// from the styles package.
func DefaultThemeConfig() *ThemeConfig {
	return &ThemeConfig{
		Primary:    styles.PrimaryColor,
		Secondary:  styles.SecondaryColor,
		Accent:     styles.BlueColor,
		Error:      styles.ErrorColor,
		Warning:    styles.WarningColor,
		Success:    styles.SecondaryColor,
		Info:       styles.BlueColor,
		Background: lipgloss.Color("#000000"),
		Surface:    styles.SurfaceColor,
		Border:     styles.BorderColor,
		Text:       styles.TextColor,
		TextMuted:  styles.MutedColor,
	}
}

// ComposableRenderer extends Renderer with the ability to have child renderers.
// This enables tree-like composition of UI components where parent components
// can delegate rendering to their children.
type ComposableRenderer interface {
	Renderer

	// Children returns the child renderers that this component contains.
	// The order of children may affect rendering order.
	Children() []Renderer
}

// RendererFunc is an adapter that allows using ordinary functions as Renderers.
type RendererFunc func(ctx RenderContext) string

// Render implements the Renderer interface.
func (f RendererFunc) Render(ctx RenderContext) string {
	return f(ctx)
}

// ConditionalRenderer wraps a Renderer and only renders when a condition is met.
// When the condition returns false, an empty string is rendered.
type ConditionalRenderer struct {
	// Condition determines whether the wrapped renderer should be invoked.
	// It receives the render context to allow dimension-based conditions.
	Condition func(ctx RenderContext) bool

	// Renderer is the wrapped renderer to invoke when condition is true.
	Renderer Renderer

	// Fallback is an optional renderer to use when condition is false.
	// If nil, an empty string is returned when condition is false.
	Fallback Renderer
}

// Render implements the Renderer interface.
// It evaluates the condition and delegates to the appropriate renderer.
func (c *ConditionalRenderer) Render(ctx RenderContext) string {
	if c.Condition != nil && c.Condition(ctx) {
		if c.Renderer != nil {
			return c.Renderer.Render(ctx)
		}
	} else if c.Fallback != nil {
		return c.Fallback.Render(ctx)
	}
	return ""
}

// NewConditionalRenderer creates a ConditionalRenderer with the given condition
// and renderer.
func NewConditionalRenderer(condition func(ctx RenderContext) bool, renderer Renderer) *ConditionalRenderer {
	return &ConditionalRenderer{
		Condition: condition,
		Renderer:  renderer,
	}
}

// WithFallback returns a copy of the ConditionalRenderer with a fallback renderer.
func (c *ConditionalRenderer) WithFallback(fallback Renderer) *ConditionalRenderer {
	return &ConditionalRenderer{
		Condition: c.Condition,
		Renderer:  c.Renderer,
		Fallback:  fallback,
	}
}

// BorderConfig specifies border styling options.
type BorderConfig struct {
	// Style specifies the border style (rounded, normal, double, etc.)
	Style lipgloss.Border

	// Color specifies the border color
	Color lipgloss.Color

	// ColorFocused specifies the border color when focused
	ColorFocused lipgloss.Color

	// Title is an optional title to display in the border
	Title string

	// TitleAlignment specifies where to place the title (left, center, right)
	TitleAlignment lipgloss.Position
}

// DefaultBorderConfig returns a BorderConfig with rounded borders and
// default colors from the theme.
func DefaultBorderConfig() BorderConfig {
	return BorderConfig{
		Style:          lipgloss.RoundedBorder(),
		Color:          styles.BorderColor,
		ColorFocused:   styles.PrimaryColor,
		TitleAlignment: lipgloss.Left,
	}
}

// BorderedRenderer wraps content in a styled border.
type BorderedRenderer struct {
	// Content is the renderer whose output will be bordered.
	Content Renderer

	// Config specifies border styling options.
	Config BorderConfig
}

// Render implements the Renderer interface.
// It renders the content and wraps it in a border, accounting for
// border dimensions when passing context to the content renderer.
func (b *BorderedRenderer) Render(ctx RenderContext) string {
	// Account for border dimensions (2 chars horizontal, 2 lines vertical)
	contentCtx := ctx.WithDimensions(ctx.Width-2, ctx.Height-2)

	content := ""
	if b.Content != nil {
		content = b.Content.Render(contentCtx)
	}

	borderColor := b.Config.Color
	if ctx.Focused && b.Config.ColorFocused != "" {
		borderColor = b.Config.ColorFocused
	}

	style := lipgloss.NewStyle().
		Border(b.Config.Style).
		BorderForeground(borderColor).
		Width(ctx.Width - 2). // lipgloss width is content width
		Height(ctx.Height - 2)

	return style.Render(content)
}

// NewBorderedRenderer creates a BorderedRenderer with default configuration.
func NewBorderedRenderer(content Renderer) *BorderedRenderer {
	return &BorderedRenderer{
		Content: content,
		Config:  DefaultBorderConfig(),
	}
}

// WithConfig returns a copy of the BorderedRenderer with updated configuration.
func (b *BorderedRenderer) WithConfig(config BorderConfig) *BorderedRenderer {
	return &BorderedRenderer{
		Content: b.Content,
		Config:  config,
	}
}

// PaddingConfig specifies padding amounts.
type PaddingConfig struct {
	Top    int
	Right  int
	Bottom int
	Left   int
}

// NewPaddingConfig creates a PaddingConfig with uniform padding on all sides.
func NewPaddingConfig(all int) PaddingConfig {
	return PaddingConfig{Top: all, Right: all, Bottom: all, Left: all}
}

// NewPaddingConfigVH creates a PaddingConfig with vertical and horizontal padding.
func NewPaddingConfigVH(vertical, horizontal int) PaddingConfig {
	return PaddingConfig{Top: vertical, Right: horizontal, Bottom: vertical, Left: horizontal}
}

// Horizontal returns the total horizontal padding (left + right).
func (p PaddingConfig) Horizontal() int {
	return p.Left + p.Right
}

// Vertical returns the total vertical padding (top + bottom).
func (p PaddingConfig) Vertical() int {
	return p.Top + p.Bottom
}

// PaddedRenderer adds padding around content.
type PaddedRenderer struct {
	// Content is the renderer whose output will be padded.
	Content Renderer

	// Padding specifies padding amounts.
	Padding PaddingConfig
}

// Render implements the Renderer interface.
// It renders the content with padding applied, adjusting available
// dimensions for the content renderer.
func (p *PaddedRenderer) Render(ctx RenderContext) string {
	// Adjust dimensions for padding
	contentWidth := ctx.Width - p.Padding.Horizontal()
	contentHeight := ctx.Height - p.Padding.Vertical()

	if contentWidth < 0 {
		contentWidth = 0
	}
	if contentHeight < 0 {
		contentHeight = 0
	}

	contentCtx := ctx.WithDimensions(contentWidth, contentHeight)

	content := ""
	if p.Content != nil {
		content = p.Content.Render(contentCtx)
	}

	style := lipgloss.NewStyle().
		PaddingTop(p.Padding.Top).
		PaddingRight(p.Padding.Right).
		PaddingBottom(p.Padding.Bottom).
		PaddingLeft(p.Padding.Left)

	return style.Render(content)
}

// NewPaddedRenderer creates a PaddedRenderer with the specified padding.
func NewPaddedRenderer(content Renderer, padding PaddingConfig) *PaddedRenderer {
	return &PaddedRenderer{
		Content: content,
		Padding: padding,
	}
}

// ScrollState tracks the scroll position and visible range.
type ScrollState struct {
	// Offset is the current scroll offset (first visible line)
	Offset int

	// TotalLines is the total number of lines in the content
	TotalLines int

	// VisibleLines is the number of lines that can be displayed
	VisibleLines int

	// AutoScroll indicates whether to automatically scroll to bottom on new content
	AutoScroll bool
}

// CanScrollUp returns true if there is content above the visible area.
func (s ScrollState) CanScrollUp() bool {
	return s.Offset > 0
}

// CanScrollDown returns true if there is content below the visible area.
func (s ScrollState) CanScrollDown() bool {
	return s.Offset+s.VisibleLines < s.TotalLines
}

// ScrollProgress returns the scroll position as a value between 0.0 and 1.0.
func (s ScrollState) ScrollProgress() float64 {
	if s.TotalLines <= s.VisibleLines {
		return 1.0
	}
	maxOffset := s.TotalLines - s.VisibleLines
	if maxOffset <= 0 {
		return 1.0
	}
	return float64(s.Offset) / float64(maxOffset)
}

// AtBottom returns true if scrolled to the bottom.
func (s ScrollState) AtBottom() bool {
	return s.Offset+s.VisibleLines >= s.TotalLines
}

// ScrollableRenderer handles scroll state for content that exceeds available height.
type ScrollableRenderer struct {
	// Content is the renderer whose output may need scrolling.
	Content Renderer

	// State holds the current scroll state.
	// This should be updated externally based on user input.
	State *ScrollState

	// ShowScrollbar indicates whether to render a scrollbar.
	ShowScrollbar bool

	// ScrollbarStyle configures the scrollbar appearance.
	ScrollbarStyle ScrollbarStyle
}

// ScrollbarStyle configures scrollbar appearance.
type ScrollbarStyle struct {
	// Track is the character used for the scrollbar track
	Track string

	// Thumb is the character used for the scrollbar thumb
	Thumb string

	// TrackStyle is the style applied to the track
	TrackStyle lipgloss.Style

	// ThumbStyle is the style applied to the thumb
	ThumbStyle lipgloss.Style
}

// DefaultScrollbarStyle returns a scrollbar style with sensible defaults.
func DefaultScrollbarStyle() ScrollbarStyle {
	return ScrollbarStyle{
		Track:      "│",
		Thumb:      "█",
		TrackStyle: lipgloss.NewStyle().Foreground(styles.MutedColor),
		ThumbStyle: lipgloss.NewStyle().Foreground(styles.PrimaryColor),
	}
}

// Render implements the Renderer interface.
// It renders the visible portion of content based on scroll state,
// optionally with a scrollbar.
func (s *ScrollableRenderer) Render(ctx RenderContext) string {
	if s.Content == nil {
		return ""
	}

	// Calculate content width (reserve space for scrollbar if shown)
	contentWidth := ctx.Width
	if s.ShowScrollbar {
		contentWidth -= 1
	}

	// Render full content with available width but unlimited height
	// to determine total lines
	contentCtx := ctx.WithDimensions(contentWidth, 0)
	fullContent := s.Content.Render(contentCtx)

	lines := strings.Split(fullContent, "\n")
	totalLines := len(lines)

	// Update state with current metrics
	if s.State != nil {
		s.State.TotalLines = totalLines
		s.State.VisibleLines = ctx.Height

		// Clamp offset to valid range
		s.State.Offset = max(s.State.Offset, 0)
		maxOffset := max(totalLines-ctx.Height, 0)
		s.State.Offset = min(s.State.Offset, maxOffset)

		// Handle auto-scroll
		if s.State.AutoScroll {
			s.State.Offset = maxOffset
		}
	}

	// Extract visible lines
	offset := 0
	if s.State != nil {
		offset = s.State.Offset
	}

	endLine := min(offset+ctx.Height, totalLines)

	visibleLines := lines[offset:endLine]

	// Pad with empty lines if needed
	for len(visibleLines) < ctx.Height {
		visibleLines = append(visibleLines, "")
	}

	// Add scrollbar if requested
	if s.ShowScrollbar && totalLines > ctx.Height {
		visibleLines = s.addScrollbar(visibleLines, ctx.Height, totalLines, offset)
	}

	return strings.Join(visibleLines, "\n")
}

// addScrollbar adds a scrollbar to the right side of each line.
func (s *ScrollableRenderer) addScrollbar(lines []string, visibleHeight, totalLines, offset int) []string {
	style := s.ScrollbarStyle
	if style.Track == "" {
		style = DefaultScrollbarStyle()
	}

	// Calculate thumb position and size
	thumbHeight := max(visibleHeight*visibleHeight/totalLines, 1)

	thumbPos := 0
	if totalLines > visibleHeight {
		scrollableRange := totalLines - visibleHeight
		trackRange := visibleHeight - thumbHeight
		if scrollableRange > 0 && trackRange > 0 {
			thumbPos = offset * trackRange / scrollableRange
		}
	}

	result := make([]string, len(lines))
	for i, line := range lines {
		var scrollChar string
		if i >= thumbPos && i < thumbPos+thumbHeight {
			scrollChar = style.ThumbStyle.Render(style.Thumb)
		} else {
			scrollChar = style.TrackStyle.Render(style.Track)
		}
		result[i] = line + scrollChar
	}

	return result
}

// NewScrollableRenderer creates a ScrollableRenderer with default settings.
func NewScrollableRenderer(content Renderer, state *ScrollState) *ScrollableRenderer {
	return &ScrollableRenderer{
		Content:        content,
		State:          state,
		ShowScrollbar:  true,
		ScrollbarStyle: DefaultScrollbarStyle(),
	}
}

// WithScrollbar returns a copy with scrollbar visibility set.
func (s *ScrollableRenderer) WithScrollbar(show bool) *ScrollableRenderer {
	return &ScrollableRenderer{
		Content:        s.Content,
		State:          s.State,
		ShowScrollbar:  show,
		ScrollbarStyle: s.ScrollbarStyle,
	}
}

// WithScrollbarStyle returns a copy with updated scrollbar style.
func (s *ScrollableRenderer) WithScrollbarStyle(style ScrollbarStyle) *ScrollableRenderer {
	return &ScrollableRenderer{
		Content:        s.Content,
		State:          s.State,
		ShowScrollbar:  s.ShowScrollbar,
		ScrollbarStyle: style,
	}
}

// StaticRenderer renders a fixed string regardless of context.
// Useful for simple labels or static content.
type StaticRenderer struct {
	Content string
}

// Render implements the Renderer interface.
func (s *StaticRenderer) Render(_ RenderContext) string {
	return s.Content
}

// NewStaticRenderer creates a StaticRenderer with the given content.
func NewStaticRenderer(content string) *StaticRenderer {
	return &StaticRenderer{Content: content}
}

// EmptyRenderer renders nothing. Useful as a placeholder or null object.
type EmptyRenderer struct{}

// Render implements the Renderer interface.
func (EmptyRenderer) Render(_ RenderContext) string {
	return ""
}

// JoinRenderer combines multiple renderers with a separator.
type JoinRenderer struct {
	// Renderers is the list of renderers to combine.
	Renderers []Renderer

	// Separator is inserted between each renderer's output.
	Separator string

	// Direction specifies whether to join horizontally or vertically.
	Direction JoinDirection
}

// JoinDirection specifies the direction for joining rendered content.
type JoinDirection int

const (
	// JoinVertical joins content vertically (newlines between)
	JoinVertical JoinDirection = iota

	// JoinHorizontal joins content horizontally (side by side)
	JoinHorizontal
)

// Render implements the Renderer interface.
func (j *JoinRenderer) Render(ctx RenderContext) string {
	if len(j.Renderers) == 0 {
		return ""
	}

	outputs := make([]string, 0, len(j.Renderers))
	for _, r := range j.Renderers {
		if r != nil {
			output := r.Render(ctx)
			if output != "" {
				outputs = append(outputs, output)
			}
		}
	}

	if len(outputs) == 0 {
		return ""
	}

	switch j.Direction {
	case JoinHorizontal:
		return lipgloss.JoinHorizontal(lipgloss.Top, outputs...)
	default:
		sep := j.Separator
		if sep == "" {
			sep = "\n"
		}
		return strings.Join(outputs, sep)
	}
}

// NewJoinRenderer creates a JoinRenderer that combines renderers vertically.
func NewJoinRenderer(renderers ...Renderer) *JoinRenderer {
	return &JoinRenderer{
		Renderers: renderers,
		Separator: "\n",
		Direction: JoinVertical,
	}
}

// Horizontal returns a copy configured to join horizontally.
func (j *JoinRenderer) Horizontal() *JoinRenderer {
	return &JoinRenderer{
		Renderers: j.Renderers,
		Separator: j.Separator,
		Direction: JoinHorizontal,
	}
}

// WithSeparator returns a copy with the specified separator.
func (j *JoinRenderer) WithSeparator(sep string) *JoinRenderer {
	return &JoinRenderer{
		Renderers: j.Renderers,
		Separator: sep,
		Direction: j.Direction,
	}
}

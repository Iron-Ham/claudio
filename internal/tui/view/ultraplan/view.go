package ultraplan

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// View provides the main entry point for ultraplan rendering.
// It composes all the individual renderers for a unified API.
type View struct {
	ctx *RenderContext

	// Component renderers
	Header        *HeaderRenderer
	Tasks         *TaskRenderer
	Status        *StatusRenderer
	Consolidation *ConsolidationRenderer
	Help          *HelpRenderer
	Sidebar       *SidebarRenderer
	PlanView      *PlanViewRenderer
	Inline        *InlineRenderer
}

// NewView creates a new ultraplan view with the given render context.
func NewView(ctx *RenderContext) *View {
	return &View{
		ctx:           ctx,
		Header:        NewHeaderRenderer(ctx),
		Tasks:         NewTaskRenderer(ctx),
		Status:        NewStatusRenderer(ctx),
		Consolidation: NewConsolidationRenderer(ctx),
		Help:          NewHelpRenderer(ctx),
		Sidebar:       NewSidebarRenderer(ctx),
		PlanView:      NewPlanViewRenderer(ctx),
		Inline:        NewInlineRenderer(ctx),
	}
}

// Render renders the main ultraplan view based on the current state.
// Returns an empty string if not in ultraplan mode.
func (v *View) Render() string {
	if v.ctx.UltraPlan == nil || v.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := v.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	// Build the complete ultraplan view
	var b strings.Builder
	b.WriteString(v.Header.Render())
	b.WriteString("\n")
	// Content would be rendered separately based on context
	return b.String()
}

// RenderHeader renders the ultra-plan header with phase and progress.
func (v *View) RenderHeader() string {
	return v.Header.Render()
}

// RenderSidebar renders a unified sidebar showing all phases with their instances.
func (v *View) RenderSidebar(width int, height int) string {
	return v.Sidebar.Render(width, height)
}

// RenderInlineContent renders the ultraplan phase content for display inline within a group.
func (v *View) RenderInlineContent(width int, maxLines int) string {
	return v.Inline.Render(width, maxLines)
}

// RenderPlanView renders the detailed plan view.
func (v *View) RenderPlanView(width int) string {
	return v.PlanView.Render(width)
}

// RenderHelp renders the help bar for ultra-plan mode.
func (v *View) RenderHelp() string {
	return v.Help.Render()
}

// RenderConsolidationSidebar renders the sidebar during the consolidation phase.
func (v *View) RenderConsolidationSidebar(width int, height int) string {
	return v.Consolidation.RenderSidebar(width, height)
}

// OpenURL opens the given URL in the default browser.
func OpenURL(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}

// WithWidth returns a copy of the context with the specified width.
func (ctx *RenderContext) WithWidth(width int) *RenderContext {
	newCtx := *ctx
	newCtx.Width = width
	return &newCtx
}

// WithHeight returns a copy of the context with the specified height.
func (ctx *RenderContext) WithHeight(height int) *RenderContext {
	newCtx := *ctx
	newCtx.Height = height
	return &newCtx
}

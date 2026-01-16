// Package ultraplan provides focused view components for the ultra-plan UI.
//
// This package extracts the ultraplan rendering logic from the main view package
// into smaller, focused components that are easier to test and maintain.
//
// # Package Structure
//
// The package is organized by UI concern:
//
//   - [View]: Main view composition and entry points
//   - [HeaderRenderer]: Header with phase and progress display
//   - [TaskRenderer]: Task list rendering with status icons and wrapping
//   - [StatusRenderer]: Phase status indicators and progress display
//   - [ConsolidationRenderer]: Consolidation phase sidebar rendering
//   - [HelpRenderer]: Context-sensitive help bar rendering
//   - [SidebarRenderer]: Sidebar section composition
//
// # Design Philosophy
//
// Each renderer is a small, focused struct with methods that handle one aspect
// of the UI. Renderers accept a shared RenderContext that provides access to
// orchestrator state without tight coupling.
//
// This follows Bubbletea best practices where views are composed from smaller,
// focused rendering functions rather than one large monolithic render method.
//
// # Usage
//
//	ctx := &ultraplan.RenderContext{
//	    Orchestrator: orch,
//	    Session:      session,
//	    UltraPlan:    ultraplanState,
//	    Width:        80,
//	    Height:       40,
//	}
//
//	view := ultraplan.NewView(ctx)
//	header := view.Header.Render()
//	sidebar := view.Sidebar.Render(width, height)
//	help := view.Help.Render()
package ultraplan

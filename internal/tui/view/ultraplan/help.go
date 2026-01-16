package ultraplan

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// HelpRenderer handles rendering of the context-sensitive help bar.
type HelpRenderer struct {
	ctx *RenderContext
}

// NewHelpRenderer creates a new help renderer with the given context.
func NewHelpRenderer(ctx *RenderContext) *HelpRenderer {
	return &HelpRenderer{ctx: ctx}
}

// Render renders the help bar for ultra-plan mode.
func (h *HelpRenderer) Render() string {
	// Input mode takes highest priority - shows INPUT badge with exit instructions
	if h.ctx.InputMode {
		badge := styles.ModeBadgeInput.Render("INPUT")
		help := styles.HelpKey.Render("[Ctrl+]]") + " exit  " +
			styles.Muted.Render("All keystrokes forwarded to Claude")
		return styles.HelpBar.Width(h.ctx.Width).Render(badge + "  " + help)
	}

	// Terminal focused mode - shows TERMINAL badge
	if h.ctx.TerminalFocused {
		badge := styles.ModeBadgeTerminal.Render("TERMINAL")
		dirMode := "invoke"
		if h.ctx.TerminalDirMode == "worktree" {
			dirMode = "worktree"
		}
		help := styles.HelpKey.Render("[Ctrl+]]") + " exit  " +
			styles.HelpKey.Render("[Ctrl+Shift+T]") + " switch dir  " +
			styles.Muted.Render("("+dirMode+")")
		return styles.HelpBar.Width(h.ctx.Width).Render(badge + "  " + help)
	}

	if h.ctx.UltraPlan == nil || h.ctx.UltraPlan.Coordinator == nil {
		return ""
	}

	session := h.ctx.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}

	var keys []string

	// Group navigation mode takes highest priority (when active)
	if h.ctx.UltraPlan.GroupNavMode && session.Plan != nil {
		badge := styles.ModeBadgeNormal.Render("GROUP NAV")
		keys = append(keys, "[↑↓/jk] select group")
		keys = append(keys, "[enter/space] toggle")
		keys = append(keys, "[←→/hl] collapse/expand")
		keys = append(keys, "[e] expand all")
		keys = append(keys, "[c] collapse all")
		keys = append(keys, "[g/esc] exit")
		return styles.HelpBar.Width(h.ctx.Width).Render(badge + "  " + strings.Join(keys, "  "))
	}

	// Group decision mode takes priority
	if session.GroupDecision != nil && session.GroupDecision.AwaitingDecision {
		badge := styles.ModeBadgeInput.Render("DECISION")
		keys = append(keys, "[c] continue partial")
		keys = append(keys, "[r] retry failed")
		keys = append(keys, "[q] cancel")
		keys = append(keys, "[↑↓] nav")
		return styles.HelpBar.Width(h.ctx.Width).Render(badge + "  " + strings.Join(keys, "  "))
	}

	// Common keys
	keys = append(keys, "[:q] quit")
	keys = append(keys, "[↑↓] nav")

	// Phase-specific keys
	switch session.Phase {
	case orchestrator.PhasePlanning:
		keys = append(keys, "[p] parse plan")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[:restart] restart step")

	case orchestrator.PhasePlanSelection:
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[:restart] restart step")

	case orchestrator.PhaseRefresh:
		keys = append(keys, "[e] start execution")
		keys = append(keys, "[E] edit plan")
		keys = append(keys, "[g] group nav")

	case orchestrator.PhaseExecuting:
		keys = append(keys, "[tab] next task")
		keys = append(keys, "[g] group nav")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[:restart] restart task")
		keys = append(keys, "[:cancel] cancel")

	case orchestrator.PhaseSynthesis:
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[g] group nav")
		keys = append(keys, "[:restart] restart synthesis")
		if session.SynthesisAwaitingApproval {
			keys = append(keys, "[s] approve → proceed")
		} else {
			keys = append(keys, "[s] skip → consolidate")
		}

	case orchestrator.PhaseRevision:
		keys = append(keys, "[tab] next instance")
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[g] group nav")
		keys = append(keys, "[:restart] restart revision")
		if session.Revision != nil {
			keys = append(keys, fmt.Sprintf("round %d/%d", session.Revision.RevisionRound, session.Revision.MaxRevisions))
		}

	case orchestrator.PhaseConsolidating:
		keys = append(keys, "[i] input mode")
		keys = append(keys, "[v] toggle plan view")
		keys = append(keys, "[g] group nav")
		keys = append(keys, "[:restart] restart consolidation")
		if session.Consolidation != nil && session.Consolidation.Phase == orchestrator.ConsolidationPaused {
			keys = append(keys, "[r] resume")
		}

	case orchestrator.PhaseComplete, orchestrator.PhaseFailed:
		keys = append(keys, "[v] view plan")
		keys = append(keys, "[g] group nav")
		if len(session.PRUrls) > 0 {
			keys = append(keys, "[o] open PR")
		}
		keys = append(keys, "[R] re-trigger group")
	}

	// Add mode badge for ultraplan mode
	badge := styles.ModeBadgeNormal.Render("ULTRAPLAN")
	return styles.HelpBar.Width(h.ctx.Width).Render(badge + "  " + strings.Join(keys, "  "))
}

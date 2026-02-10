package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/tui/view/ultraplan"
	"github.com/charmbracelet/lipgloss"
)

// WorkflowStatusState aggregates status from all active workflow types.
// This enables the header to show multiple concurrent workflows at once.
type WorkflowStatusState struct {
	Pipeline    *PipelineState
	UltraPlan   *UltraPlanState
	TripleShot  *TripleShotState
	Adversarial *AdversarialState
}

// WorkflowIndicator represents a single workflow's status indicator.
type WorkflowIndicator struct {
	Icon      string
	Label     string
	Style     lipgloss.Style
	Count     int    // Number of active sessions (0 means single session mode)
	Objective string // Objective/task description (used by ultraplan for single-workflow header display)
}

// HasActiveWorkflows returns true if any workflow type is active.
func (s *WorkflowStatusState) HasActiveWorkflows() bool {
	if s == nil {
		return false
	}
	return s.hasPipeline() || s.hasUltraPlan() || s.hasTripleShot() || s.hasAdversarial()
}

// hasPipeline checks if a pipeline is active.
func (s *WorkflowStatusState) hasPipeline() bool {
	return s.Pipeline != nil && s.Pipeline.IsActive()
}

// hasUltraPlan checks if ultraplan is active.
func (s *WorkflowStatusState) hasUltraPlan() bool {
	return s.UltraPlan != nil && s.UltraPlan.Coordinator != nil
}

// hasTripleShot checks if tripleshot is active.
func (s *WorkflowStatusState) hasTripleShot() bool {
	return s.TripleShot != nil && s.TripleShot.HasActiveCoordinators()
}

// hasAdversarial checks if adversarial is active.
func (s *WorkflowStatusState) hasAdversarial() bool {
	return s.Adversarial != nil && s.Adversarial.HasActiveCoordinators()
}

// GetUltraPlanObjective returns the ultraplan objective if ultraplan is active.
// Returns empty string if ultraplan is not active or has no objective.
func (s *WorkflowStatusState) GetUltraPlanObjective() string {
	if !s.hasUltraPlan() {
		return ""
	}
	session := s.UltraPlan.Coordinator.Session()
	if session == nil {
		return ""
	}
	return session.Objective
}

// GetIndicators returns a slice of workflow indicators for all active workflows.
// The indicators are ordered by priority: Pipeline, UltraPlan, TripleShot, Adversarial.
func (s *WorkflowStatusState) GetIndicators() []WorkflowIndicator {
	if s == nil {
		return nil
	}

	var indicators []WorkflowIndicator

	if ind := s.getPipelineIndicator(); ind != nil {
		indicators = append(indicators, *ind)
	}
	if ind := s.getUltraPlanIndicator(); ind != nil {
		indicators = append(indicators, *ind)
	}
	if ind := s.getTripleShotIndicator(); ind != nil {
		indicators = append(indicators, *ind)
	}
	if ind := s.getAdversarialIndicator(); ind != nil {
		indicators = append(indicators, *ind)
	}

	return indicators
}

// getPipelineIndicator returns the indicator for pipeline mode.
func (s *WorkflowStatusState) getPipelineIndicator() *WorkflowIndicator {
	if s.Pipeline == nil {
		return nil
	}
	return s.Pipeline.GetIndicator()
}

// getUltraPlanIndicator returns the indicator for ultraplan mode.
func (s *WorkflowStatusState) getUltraPlanIndicator() *WorkflowIndicator {
	if !s.hasUltraPlan() {
		return nil
	}

	session := s.UltraPlan.Coordinator.Session()
	if session == nil {
		return nil
	}

	phaseStr := ultraplan.PhaseToString(session.Phase)
	pStyle := ultraplan.PhaseStyle(session.Phase)

	// Build a compact progress indicator
	var progress string
	switch session.Phase {
	case orchestrator.PhaseExecuting:
		prog := session.Progress()
		progress = fmt.Sprintf("%.0f%%", prog)
	case orchestrator.PhaseConsolidating:
		if session.Consolidation != nil && session.Consolidation.TotalGroups > 0 {
			progress = fmt.Sprintf("%d/%d", session.Consolidation.CurrentGroup, session.Consolidation.TotalGroups)
		}
	case orchestrator.PhaseComplete:
		progress = "done"
	case orchestrator.PhaseFailed:
		progress = "failed"
	}

	label := phaseStr
	if progress != "" && progress != "done" && progress != "failed" {
		label = fmt.Sprintf("%s:%s", phaseStr, progress)
	}

	return &WorkflowIndicator{
		Icon:      styles.IconUltraPlan,
		Label:     label,
		Style:     pStyle,
		Count:     0, // Ultraplan is always single-session
		Objective: session.Objective,
	}
}

// getTripleShotIndicator returns the indicator for tripleshot mode.
func (s *WorkflowStatusState) getTripleShotIndicator() *WorkflowIndicator {
	if !s.hasTripleShot() {
		return nil
	}

	runners := s.TripleShot.GetAllRunners()
	if len(runners) == 0 {
		return nil
	}

	// Aggregate status across all tripleshot sessions
	working := 0
	evaluating := 0
	complete := 0
	failed := 0

	for _, runner := range runners {
		session := runner.Session()
		if session == nil {
			continue
		}
		switch session.Phase {
		case tripleshot.PhaseWorking:
			working++
		case tripleshot.PhaseEvaluating:
			evaluating++
		case tripleshot.PhaseComplete:
			complete++
		case tripleshot.PhaseFailed:
			failed++
		}
	}

	// Determine overall status and label
	var label string
	var style lipgloss.Style

	total := len(runners)
	if total == 1 {
		// Single session - show detailed phase
		session := runners[0].Session()
		if session == nil {
			return nil
		}
		switch session.Phase {
		case tripleshot.PhaseWorking:
			label = "working"
			style = tsHighlight
		case tripleshot.PhaseEvaluating:
			label = "evaluating"
			style = tsWarning
		case tripleshot.PhaseComplete:
			label = "complete"
			style = tsSuccess
		case tripleshot.PhaseFailed:
			label = "failed"
			style = tsError
		}
	} else {
		// Multiple sessions - show summary
		if working > 0 || evaluating > 0 {
			active := working + evaluating
			label = fmt.Sprintf("%d active", active)
			style = tsHighlight
		} else if failed > 0 && complete == 0 {
			label = "failed"
			style = tsError
		} else {
			label = fmt.Sprintf("%d done", complete)
			style = tsSuccess
		}
	}

	count := 0
	if total > 1 {
		count = total
	}

	return &WorkflowIndicator{
		Icon:  styles.IconTripleShot,
		Label: label,
		Style: style,
		Count: count,
	}
}

// getAdversarialIndicator returns the indicator for adversarial mode.
func (s *WorkflowStatusState) getAdversarialIndicator() *WorkflowIndicator {
	if !s.hasAdversarial() {
		return nil
	}

	coordinators := s.Adversarial.GetAllCoordinators()
	if len(coordinators) == 0 {
		return nil
	}

	// Aggregate status across all adversarial sessions
	implementing := 0
	reviewing := 0
	approved := 0
	failed := 0

	for _, coord := range coordinators {
		session := coord.Session()
		if session == nil {
			continue
		}
		switch session.Phase {
		case adversarial.PhaseImplementing:
			implementing++
		case adversarial.PhaseReviewing:
			reviewing++
		case adversarial.PhaseApproved, adversarial.PhaseComplete:
			approved++
		case adversarial.PhaseFailed:
			failed++
		}
	}

	var icon string
	var label string
	var style lipgloss.Style

	total := len(coordinators)
	if total == 1 {
		// Single session - show detailed phase with round
		session := coordinators[0].Session()
		if session == nil {
			return nil
		}
		roundStr := fmt.Sprintf("R%d", session.CurrentRound)
		if session.Config.MaxIterations > 0 {
			roundStr = fmt.Sprintf("R%d/%d", session.CurrentRound, session.Config.MaxIterations)
		}

		switch session.Phase {
		case adversarial.PhaseImplementing:
			icon = "🔨"
			label = roundStr
			style = advHighlight
		case adversarial.PhaseReviewing:
			icon = "🔍"
			label = roundStr
			style = advWarning
		case adversarial.PhaseApproved, adversarial.PhaseComplete:
			icon = "✓"
			label = "approved"
			style = advSuccess
		case adversarial.PhaseFailed:
			icon = "✗"
			label = "failed"
			style = advError
		}
	} else {
		// Multiple sessions - show summary with most urgent status
		if implementing > 0 {
			icon = "🔨"
			label = fmt.Sprintf("%d impl", implementing)
			style = advHighlight
		} else if reviewing > 0 {
			icon = "🔍"
			label = fmt.Sprintf("%d review", reviewing)
			style = advWarning
		} else if failed > 0 && approved == 0 {
			icon = "✗"
			label = "failed"
			style = advError
		} else {
			icon = "✓"
			label = fmt.Sprintf("%d done", approved)
			style = advSuccess
		}
	}

	count := 0
	if total > 1 {
		count = total
	}

	return &WorkflowIndicator{
		Icon:  icon,
		Label: label,
		Style: style,
		Count: count,
	}
}

// RenderWorkflowStatus renders the unified workflow status for the header.
// Returns an empty string if no workflows are active.
func RenderWorkflowStatus(state *WorkflowStatusState) string {
	if state == nil || !state.HasActiveWorkflows() {
		return ""
	}

	indicators := state.GetIndicators()
	if len(indicators) == 0 {
		return ""
	}

	var parts []string
	for _, ind := range indicators {
		parts = append(parts, renderIndicator(ind))
	}

	return strings.Join(parts, " │ ")
}

// renderIndicator renders a single workflow indicator.
func renderIndicator(ind WorkflowIndicator) string {
	var b strings.Builder

	b.WriteString(ind.Icon)
	b.WriteString(" ")

	if ind.Count > 0 {
		// Show count in parentheses for multiple sessions
		b.WriteString(fmt.Sprintf("(%d)", ind.Count))
		b.WriteString(" ")
	}

	b.WriteString(ind.Style.Render(ind.Label))

	return b.String()
}

// WorkflowStatusView provides methods for rendering workflow status components.
type WorkflowStatusView struct{}

// NewWorkflowStatusView creates a new workflow status view.
func NewWorkflowStatusView() *WorkflowStatusView {
	return &WorkflowStatusView{}
}

// Render renders the workflow status line.
func (v *WorkflowStatusView) Render(state *WorkflowStatusState) string {
	return RenderWorkflowStatus(state)
}

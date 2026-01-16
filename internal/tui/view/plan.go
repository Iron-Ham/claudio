package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Plan style aliases for better readability
var (
	planHighlight = styles.Primary
	planSuccess   = styles.Secondary
	planError     = styles.Error
	planSubtle    = styles.Muted
)

// PlanSession tracks a single plan session's state.
// This mirrors the relevant fields from InlinePlanState for sidebar display.
type PlanSession struct {
	// GroupID is the ID of the group containing this plan's instances
	GroupID string

	// Objective is the user-provided goal for this plan
	Objective string

	// Phase indicates the current plan phase
	Phase PlanPhase

	// PlannerID is the instance ID of the planning instance
	PlannerID string

	// PlannerStatus tracks the planner's status
	PlannerStatus PlannerStatus

	// FinalPlan holds the generated plan (set when planner completes)
	FinalPlan *orchestrator.PlanSpec

	// ExecutingTaskIDs are instance IDs for tasks currently executing
	ExecutingTaskIDs []string
}

// PlanPhase represents the current phase of a plan session
type PlanPhase string

const (
	PlanPhasePlanning  PlanPhase = "planning"  // Planner is generating the plan
	PlanPhaseReady     PlanPhase = "ready"     // Plan ready for review/execution
	PlanPhaseExecuting PlanPhase = "executing" // Tasks are being executed
	PlanPhaseComplete  PlanPhase = "complete"  // All tasks completed
	PlanPhaseFailed    PlanPhase = "failed"    // Planning or execution failed
)

// PlanState holds the state for multiple concurrent plan sessions.
// This enables tracking multiple :plan commands running simultaneously.
type PlanState struct {
	// Sessions maps group IDs to their plan sessions.
	// This enables multiple concurrent plan sessions.
	Sessions map[string]*PlanSession
}

// NewPlanState creates a new PlanState.
func NewPlanState() *PlanState {
	return &PlanState{
		Sessions: make(map[string]*PlanSession),
	}
}

// AddSession adds a new plan session.
func (s *PlanState) AddSession(session *PlanSession) {
	if s.Sessions == nil {
		s.Sessions = make(map[string]*PlanSession)
	}
	s.Sessions[session.GroupID] = session
}

// GetSession returns a session by group ID.
func (s *PlanState) GetSession(groupID string) *PlanSession {
	if s == nil || s.Sessions == nil {
		return nil
	}
	return s.Sessions[groupID]
}

// RemoveSession removes a session by group ID.
func (s *PlanState) RemoveSession(groupID string) {
	if s != nil && s.Sessions != nil {
		delete(s.Sessions, groupID)
	}
}

// GetAllSessions returns all sessions in deterministic order (sorted by group ID).
func (s *PlanState) GetAllSessions() []*PlanSession {
	if s == nil || len(s.Sessions) == 0 {
		return nil
	}

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.Sessions))
	for k := range s.Sessions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sessions := make([]*PlanSession, 0, len(s.Sessions))
	for _, k := range keys {
		sessions = append(sessions, s.Sessions[k])
	}
	return sessions
}

// HasActiveSessions returns true if there are any active plan sessions.
func (s *PlanState) HasActiveSessions() bool {
	if s == nil {
		return false
	}
	return len(s.Sessions) > 0
}

// SessionCount returns the number of active sessions.
func (s *PlanState) SessionCount() int {
	if s == nil || s.Sessions == nil {
		return 0
	}
	return len(s.Sessions)
}

// PlanRenderContext provides context for rendering plan views.
type PlanRenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	Plan         *PlanState
	ActiveTab    int
	Width        int
	Height       int
}

// RenderPlanHeader renders a compact header showing plan status.
// For multiple sessions, shows a summary count.
func RenderPlanHeader(ctx PlanRenderContext) string {
	if ctx.Plan == nil || !ctx.Plan.HasActiveSessions() {
		return ""
	}

	sessions := ctx.Plan.GetAllSessions()
	if len(sessions) == 0 {
		return ""
	}

	// For multiple sessions, show a summary
	if len(sessions) > 1 {
		planning := 0
		executing := 0
		complete := 0
		for _, sess := range sessions {
			switch sess.Phase {
			case PlanPhasePlanning:
				planning++
			case PlanPhaseExecuting, PlanPhaseReady:
				executing++
			case PlanPhaseComplete:
				complete++
			}
		}
		summary := fmt.Sprintf("%d active", len(sessions))
		if planning > 0 {
			summary += fmt.Sprintf(", %d planning", planning)
		}
		if executing > 0 {
			summary += fmt.Sprintf(", %d executing", executing)
		}
		if complete > 0 {
			summary += fmt.Sprintf(", %d complete", complete)
		}
		return planSubtle.Render("Plans: ") + summary
	}

	// Single session - show detailed status
	sess := sessions[0]
	return renderSinglePlanHeader(sess)
}

// renderSinglePlanHeader renders the header for a single plan session.
func renderSinglePlanHeader(sess *PlanSession) string {
	var phaseIcon string
	var phaseText string
	var phaseStyle lipgloss.Style

	switch sess.Phase {
	case PlanPhasePlanning:
		phaseIcon = "⚡"
		phaseText = "Planning"
		phaseStyle = planHighlight
	case PlanPhaseReady:
		phaseIcon = "📋"
		phaseText = "Ready"
		phaseStyle = planSuccess
	case PlanPhaseExecuting:
		phaseIcon = "▶"
		phaseText = "Executing"
		phaseStyle = planHighlight
	case PlanPhaseComplete:
		phaseIcon = "✓"
		phaseText = "Complete"
		phaseStyle = planSuccess
	case PlanPhaseFailed:
		phaseIcon = "✗"
		phaseText = "Failed"
		phaseStyle = planError
	}

	// Build planner status indicator
	var plannerStatus string
	switch sess.PlannerStatus {
	case PlannerStatusWorking:
		plannerStatus = "⏳"
	case PlannerStatusCompleted:
		plannerStatus = "✓"
	case PlannerStatusFailed:
		plannerStatus = "✗"
	default:
		plannerStatus = "○"
	}

	header := fmt.Sprintf("%s %s | Planner: %s",
		phaseIcon,
		phaseStyle.Render(phaseText),
		plannerStatus,
	)

	// Add task count if plan is ready
	if sess.FinalPlan != nil && len(sess.FinalPlan.Tasks) > 0 {
		header += fmt.Sprintf(" | %d tasks", len(sess.FinalPlan.Tasks))
	}

	return planSubtle.Render("Plan: ") + header
}

// RenderPlanSidebarSection renders a sidebar section for plan mode.
// For multiple sessions, iterates through all plan groups.
func RenderPlanSidebarSection(ctx PlanRenderContext, width int) string {
	if ctx.Plan == nil || !ctx.Plan.HasActiveSessions() {
		return ""
	}

	var lines []string

	// Title - show count if multiple
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.PurpleColor)
	sessions := ctx.Plan.GetAllSessions()
	if len(sessions) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("Plans (%d)", len(sessions))))
	} else {
		lines = append(lines, titleStyle.Render("Plan"))
	}
	lines = append(lines, "")

	// Render each plan session
	for idx, sess := range sessions {
		// Add separator between sessions
		if idx > 0 {
			lines = append(lines, "")
			lines = append(lines, strings.Repeat("─", width-4))
			lines = append(lines, "")
		}

		lines = append(lines, renderSinglePlanSection(ctx, sess, width, idx+1, len(sessions) > 1)...)
	}

	return strings.Join(lines, "\n")
}

// renderSinglePlanSection renders a single plan session's status.
func renderSinglePlanSection(ctx PlanRenderContext, sess *PlanSession, width int, index int, showIndex bool) []string {
	var lines []string

	// Objective preview with optional index
	objective := sess.Objective
	maxObjLen := width - 10
	if showIndex {
		maxObjLen -= 4 // Account for "#N: " prefix
	}
	if len(objective) > maxObjLen {
		objective = objective[:maxObjLen-3] + "..."
	}
	if showIndex {
		lines = append(lines, planSubtle.Render(fmt.Sprintf("#%d: ", index))+objective)
	} else {
		lines = append(lines, planSubtle.Render("Goal: ")+objective)
	}
	lines = append(lines, "")

	// Phase indicator
	var phaseStyle lipgloss.Style
	var phaseText string
	switch sess.Phase {
	case PlanPhasePlanning:
		phaseStyle = planHighlight
		phaseText = "Planning"
	case PlanPhaseReady:
		phaseStyle = planSuccess
		phaseText = "Ready for review"
	case PlanPhaseExecuting:
		phaseStyle = planHighlight
		phaseText = "Executing"
	case PlanPhaseComplete:
		phaseStyle = planSuccess
		phaseText = "Complete"
	case PlanPhaseFailed:
		phaseStyle = planError
		phaseText = "Failed"
	}
	lines = append(lines, planSubtle.Render("Phase: ")+phaseStyle.Render(phaseText))
	lines = append(lines, "")

	// Planner status (during planning phase)
	if sess.Phase == PlanPhasePlanning && sess.PlannerID != "" {
		lines = append(lines, planSubtle.Render("Planner:"))
		var statusStyle lipgloss.Style
		var statusText string
		switch sess.PlannerStatus {
		case PlannerStatusWorking:
			statusStyle = planHighlight
			statusText = "generating plan"
		case PlannerStatusCompleted:
			statusStyle = planSuccess
			statusText = "done"
		case PlannerStatusFailed:
			statusStyle = planError
			statusText = "failed"
		default:
			statusStyle = planSubtle
			statusText = "pending"
		}

		// Highlight if planner instance is currently selected
		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == sess.PlannerID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		lines = append(lines, prefix+statusStyle.Render(statusText))
	}

	// Task count (when plan is ready or executing)
	if sess.FinalPlan != nil && len(sess.FinalPlan.Tasks) > 0 {
		lines = append(lines, "")
		taskCount := len(sess.FinalPlan.Tasks)
		if sess.Phase == PlanPhaseExecuting {
			// Show execution progress
			completedTasks := 0
			for _, taskID := range sess.ExecutingTaskIDs {
				if inst := ctx.Session.GetInstance(taskID); inst != nil && inst.Status == orchestrator.StatusCompleted {
					completedTasks++
				}
			}
			lines = append(lines, planSubtle.Render("Tasks: ")+fmt.Sprintf("%d/%d", completedTasks, taskCount))
		} else {
			lines = append(lines, planSubtle.Render("Tasks: ")+fmt.Sprintf("%d", taskCount))
		}
	}

	return lines
}

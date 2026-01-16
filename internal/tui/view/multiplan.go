package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// MultiPlan style aliases for better readability
var (
	mpHighlight = styles.Primary
	mpSuccess   = styles.Secondary
	mpWarning   = styles.Warning
	mpError     = styles.Error
	mpSubtle    = styles.Muted
)

// MultiPlanSession tracks a single multiplan session's state.
// This mirrors the relevant fields from InlinePlanState for sidebar display.
type MultiPlanSession struct {
	// GroupID is the ID of the group containing this multiplan's instances
	GroupID string

	// Objective is the user-provided goal for this plan
	Objective string

	// Phase indicates the current multiplan phase
	Phase MultiPlanPhase

	// PlannerIDs are the instance IDs for the planning instances (one per strategy)
	PlannerIDs []string

	// PlannerStatuses tracks completion status for each planner
	PlannerStatuses []PlannerStatus

	// ManagerID is the instance ID of the plan manager/evaluator (set after planners complete)
	ManagerID string

	// ManagerStatus tracks the manager's status
	ManagerStatus PlannerStatus

	// FinalPlan holds the selected/merged plan (set when manager completes)
	FinalPlan *orchestrator.PlanSpec

	// ExecutingTaskIDs are instance IDs for tasks currently executing
	ExecutingTaskIDs []string
}

// MultiPlanPhase represents the current phase of a multiplan session
type MultiPlanPhase string

const (
	MultiPlanPhasePlanning   MultiPlanPhase = "planning"   // Planners are generating plans
	MultiPlanPhaseEvaluating MultiPlanPhase = "evaluating" // Manager is evaluating plans
	MultiPlanPhaseReady      MultiPlanPhase = "ready"      // Plan ready for review/execution
	MultiPlanPhaseExecuting  MultiPlanPhase = "executing"  // Tasks are being executed
	MultiPlanPhaseComplete   MultiPlanPhase = "complete"   // All tasks completed
	MultiPlanPhaseFailed     MultiPlanPhase = "failed"     // Planning or execution failed
)

// PlannerStatus represents the status of a planner or manager instance
type PlannerStatus string

const (
	PlannerStatusPending   PlannerStatus = "pending"
	PlannerStatusWorking   PlannerStatus = "working"
	PlannerStatusCompleted PlannerStatus = "completed"
	PlannerStatusFailed    PlannerStatus = "failed"
)

// MultiPlanState holds the state for multiple concurrent multiplan sessions.
// This enables tracking multiple :multiplan commands running simultaneously.
type MultiPlanState struct {
	// Sessions maps group IDs to their multiplan sessions.
	// This enables multiple concurrent multiplan sessions.
	Sessions map[string]*MultiPlanSession
}

// NewMultiPlanState creates a new MultiPlanState.
func NewMultiPlanState() *MultiPlanState {
	return &MultiPlanState{
		Sessions: make(map[string]*MultiPlanSession),
	}
}

// AddSession adds a new multiplan session.
func (s *MultiPlanState) AddSession(session *MultiPlanSession) {
	if s.Sessions == nil {
		s.Sessions = make(map[string]*MultiPlanSession)
	}
	s.Sessions[session.GroupID] = session
}

// GetSession returns a session by group ID.
func (s *MultiPlanState) GetSession(groupID string) *MultiPlanSession {
	if s == nil || s.Sessions == nil {
		return nil
	}
	return s.Sessions[groupID]
}

// RemoveSession removes a session by group ID.
func (s *MultiPlanState) RemoveSession(groupID string) {
	if s != nil && s.Sessions != nil {
		delete(s.Sessions, groupID)
	}
}

// GetAllSessions returns all sessions in deterministic order (sorted by group ID).
func (s *MultiPlanState) GetAllSessions() []*MultiPlanSession {
	if s == nil || len(s.Sessions) == 0 {
		return nil
	}

	// Sort keys for deterministic iteration order
	keys := make([]string, 0, len(s.Sessions))
	for k := range s.Sessions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	sessions := make([]*MultiPlanSession, 0, len(s.Sessions))
	for _, k := range keys {
		sessions = append(sessions, s.Sessions[k])
	}
	return sessions
}

// HasActiveSessions returns true if there are any active multiplan sessions.
func (s *MultiPlanState) HasActiveSessions() bool {
	if s == nil {
		return false
	}
	return len(s.Sessions) > 0
}

// SessionCount returns the number of active sessions.
func (s *MultiPlanState) SessionCount() int {
	if s == nil || s.Sessions == nil {
		return 0
	}
	return len(s.Sessions)
}

// MultiPlanRenderContext provides context for rendering multiplan views.
type MultiPlanRenderContext struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	MultiPlan    *MultiPlanState
	ActiveTab    int
	Width        int
	Height       int
}

// RenderMultiPlanHeader renders a compact header showing multiplan status.
// For multiple sessions, shows a summary count.
func RenderMultiPlanHeader(ctx MultiPlanRenderContext) string {
	if ctx.MultiPlan == nil || !ctx.MultiPlan.HasActiveSessions() {
		return ""
	}

	sessions := ctx.MultiPlan.GetAllSessions()
	if len(sessions) == 0 {
		return ""
	}

	// For multiple sessions, show a summary
	if len(sessions) > 1 {
		planning := 0
		evaluating := 0
		executing := 0
		complete := 0
		for _, sess := range sessions {
			switch sess.Phase {
			case MultiPlanPhasePlanning:
				planning++
			case MultiPlanPhaseEvaluating:
				evaluating++
			case MultiPlanPhaseExecuting, MultiPlanPhaseReady:
				executing++
			case MultiPlanPhaseComplete:
				complete++
			}
		}
		summary := fmt.Sprintf("%d active", len(sessions))
		if planning > 0 {
			summary += fmt.Sprintf(", %d planning", planning)
		}
		if evaluating > 0 {
			summary += fmt.Sprintf(", %d evaluating", evaluating)
		}
		if complete > 0 {
			summary += fmt.Sprintf(", %d complete", complete)
		}
		return mpSubtle.Render("MultiPlans: ") + summary
	}

	// Single session - show detailed status
	sess := sessions[0]
	return renderSingleMultiPlanHeader(sess)
}

// renderSingleMultiPlanHeader renders the header for a single multiplan session.
func renderSingleMultiPlanHeader(sess *MultiPlanSession) string {
	var phaseIcon string
	var phaseText string
	var phaseStyle lipgloss.Style

	switch sess.Phase {
	case MultiPlanPhasePlanning:
		phaseIcon = "⚡"
		phaseText = "Planning"
		phaseStyle = mpHighlight
	case MultiPlanPhaseEvaluating:
		phaseIcon = "🔍"
		phaseText = "Evaluating"
		phaseStyle = mpWarning
	case MultiPlanPhaseReady:
		phaseIcon = "📋"
		phaseText = "Ready"
		phaseStyle = mpSuccess
	case MultiPlanPhaseExecuting:
		phaseIcon = "▶"
		phaseText = "Executing"
		phaseStyle = mpHighlight
	case MultiPlanPhaseComplete:
		phaseIcon = "✓"
		phaseText = "Complete"
		phaseStyle = mpSuccess
	case MultiPlanPhaseFailed:
		phaseIcon = "✗"
		phaseText = "Failed"
		phaseStyle = mpError
	}

	// Build planner status
	var plannerStatuses []string
	for i, status := range sess.PlannerStatuses {
		var statusIcon string
		switch status {
		case PlannerStatusWorking:
			statusIcon = "⏳"
		case PlannerStatusCompleted:
			statusIcon = "✓"
		case PlannerStatusFailed:
			statusIcon = "✗"
		default:
			statusIcon = "○"
		}
		plannerStatuses = append(plannerStatuses, fmt.Sprintf("%s%d", statusIcon, i+1))
	}

	header := fmt.Sprintf("%s %s | Planners: %s",
		phaseIcon,
		phaseStyle.Render(phaseText),
		strings.Join(plannerStatuses, " "),
	)

	// Add manager status if in evaluating phase or later
	if sess.Phase == MultiPlanPhaseEvaluating {
		header += " | Manager: ⏳"
	} else if sess.ManagerStatus == PlannerStatusCompleted {
		header += " | Manager: ✓"
	}

	return mpSubtle.Render("MultiPlan: ") + header
}

// RenderMultiPlanSidebarSection renders a sidebar section for multiplan mode.
// For multiple sessions, iterates through all multiplan groups.
func RenderMultiPlanSidebarSection(ctx MultiPlanRenderContext, width int) string {
	if ctx.MultiPlan == nil || !ctx.MultiPlan.HasActiveSessions() {
		return ""
	}

	var lines []string

	// Title - show count if multiple
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(styles.BlueColor)
	sessions := ctx.MultiPlan.GetAllSessions()
	if len(sessions) > 1 {
		lines = append(lines, titleStyle.Render(fmt.Sprintf("MultiPlans (%d)", len(sessions))))
	} else {
		lines = append(lines, titleStyle.Render("MultiPlan"))
	}
	lines = append(lines, "")

	// Render each multiplan session
	for idx, sess := range sessions {
		// Add separator between sessions
		if idx > 0 {
			lines = append(lines, "")
			lines = append(lines, strings.Repeat("─", width-4))
			lines = append(lines, "")
		}

		lines = append(lines, renderSingleMultiPlanSection(ctx, sess, width, idx+1, len(sessions) > 1)...)
	}

	return strings.Join(lines, "\n")
}

// renderSingleMultiPlanSection renders a single multiplan session's status.
func renderSingleMultiPlanSection(ctx MultiPlanRenderContext, sess *MultiPlanSession, width int, index int, showIndex bool) []string {
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
		lines = append(lines, mpSubtle.Render(fmt.Sprintf("#%d: ", index))+objective)
	} else {
		lines = append(lines, mpSubtle.Render("Goal: ")+objective)
	}
	lines = append(lines, "")

	// Phase indicator
	var phaseStyle lipgloss.Style
	var phaseText string
	switch sess.Phase {
	case MultiPlanPhasePlanning:
		phaseStyle = mpHighlight
		phaseText = "Planning"
	case MultiPlanPhaseEvaluating:
		phaseStyle = mpWarning
		phaseText = "Evaluating"
	case MultiPlanPhaseReady:
		phaseStyle = mpSuccess
		phaseText = "Ready for review"
	case MultiPlanPhaseExecuting:
		phaseStyle = mpHighlight
		phaseText = "Executing"
	case MultiPlanPhaseComplete:
		phaseStyle = mpSuccess
		phaseText = "Complete"
	case MultiPlanPhaseFailed:
		phaseStyle = mpError
		phaseText = "Failed"
	}
	lines = append(lines, mpSubtle.Render("Phase: ")+phaseStyle.Render(phaseText))
	lines = append(lines, "")

	// Planner status (during planning phase)
	if sess.Phase == MultiPlanPhasePlanning || sess.Phase == MultiPlanPhaseEvaluating {
		lines = append(lines, mpSubtle.Render("Planners:"))
		strategyNames := []string{"Minimal", "Balanced", "Thorough"}
		for i, status := range sess.PlannerStatuses {
			var statusStyle lipgloss.Style
			var statusText string
			switch status {
			case PlannerStatusWorking:
				statusStyle = mpHighlight
				statusText = "working"
			case PlannerStatusCompleted:
				statusStyle = mpSuccess
				statusText = "done"
			case PlannerStatusFailed:
				statusStyle = mpError
				statusText = "failed"
			default:
				statusStyle = mpSubtle
				statusText = "pending"
			}

			strategyName := "Strategy"
			if i < len(strategyNames) {
				strategyName = strategyNames[i]
			}

			// Highlight if this planner's instance is currently selected
			var prefix string
			if i < len(sess.PlannerIDs) && ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
				activeInst := ctx.Session.Instances[ctx.ActiveTab]
				if activeInst != nil && activeInst.ID == sess.PlannerIDs[i] {
					prefix = "▶ "
				} else {
					prefix = "  "
				}
			} else {
				prefix = "  "
			}

			lines = append(lines, prefix+fmt.Sprintf("%d. %s: %s", i+1, strategyName, statusStyle.Render(statusText)))
		}
	}

	// Manager status (during evaluation)
	if sess.Phase == MultiPlanPhaseEvaluating && sess.ManagerID != "" {
		lines = append(lines, "")
		lines = append(lines, mpSubtle.Render("Manager:"))

		var managerStyle lipgloss.Style
		var managerText string
		switch sess.ManagerStatus {
		case PlannerStatusWorking:
			managerStyle = mpWarning
			managerText = "evaluating plans"
		case PlannerStatusCompleted:
			managerStyle = mpSuccess
			managerText = "done"
		case PlannerStatusFailed:
			managerStyle = mpError
			managerText = "failed"
		default:
			managerStyle = mpSubtle
			managerText = "pending"
		}

		var prefix string
		if ctx.Session != nil && ctx.ActiveTab < len(ctx.Session.Instances) {
			activeInst := ctx.Session.Instances[ctx.ActiveTab]
			if activeInst != nil && activeInst.ID == sess.ManagerID {
				prefix = "▶ "
			} else {
				prefix = "  "
			}
		} else {
			prefix = "  "
		}

		lines = append(lines, prefix+managerStyle.Render(managerText))
	}

	// Task count (when plan is ready or executing)
	if sess.FinalPlan != nil && len(sess.FinalPlan.Tasks) > 0 {
		lines = append(lines, "")
		taskCount := len(sess.FinalPlan.Tasks)
		if sess.Phase == MultiPlanPhaseExecuting {
			// Show execution progress
			completedTasks := 0
			for _, taskID := range sess.ExecutingTaskIDs {
				if inst := ctx.Session.GetInstance(taskID); inst != nil && inst.Status == orchestrator.StatusCompleted {
					completedTasks++
				}
			}
			lines = append(lines, mpSubtle.Render("Tasks: ")+fmt.Sprintf("%d/%d", completedTasks, taskCount))
		} else {
			lines = append(lines, mpSubtle.Render("Tasks: ")+fmt.Sprintf("%d", taskCount))
		}
	}

	return lines
}

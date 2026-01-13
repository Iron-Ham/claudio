package view

import (
	"fmt"
	"strings"
	"time"

	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// GroupMetrics holds aggregated metrics for a single group.
type GroupMetrics struct {
	GroupID         string
	GroupName       string
	Phase           orchestrator.GroupPhase
	Completed       int
	Total           int
	InputTokens     int64
	OutputTokens    int64
	CacheRead       int64
	CacheWrite      int64
	Cost            float64
	APICalls        int
	StartTime       *time.Time
	EndTime         *time.Time
	Duration        time.Duration
	AverageDuration time.Duration // Average duration per completed instance
}

// SessionGroupMetrics holds aggregated metrics for all groups in a session.
type SessionGroupMetrics struct {
	Groups         []*GroupMetrics
	TotalCompleted int
	TotalInstances int
	TotalCost      float64
	TotalDuration  time.Duration
	EstimatedETA   time.Duration
}

// CalculateGroupMetrics calculates metrics for a single group by aggregating
// metrics from all instances within the group (including sub-groups).
func CalculateGroupMetrics(group *orchestrator.InstanceGroup, session *orchestrator.Session) *GroupMetrics {
	if group == nil || session == nil {
		return nil
	}

	gm := &GroupMetrics{
		GroupID:   group.ID,
		GroupName: group.Name,
		Phase:     group.Phase,
	}

	// Get all instance IDs including sub-groups
	instanceIDs := group.AllInstanceIDs()
	gm.Total = len(instanceIDs)

	var earliestStart, latestEnd *time.Time
	var totalCompletedDuration time.Duration
	completedCount := 0

	for _, instID := range instanceIDs {
		inst := session.GetInstance(instID)
		if inst == nil {
			continue
		}

		// Count completed instances
		if inst.Status == orchestrator.StatusCompleted {
			gm.Completed++
			completedCount++
		}

		// Aggregate metrics if available
		if inst.Metrics != nil {
			gm.InputTokens += inst.Metrics.InputTokens
			gm.OutputTokens += inst.Metrics.OutputTokens
			gm.CacheRead += inst.Metrics.CacheRead
			gm.CacheWrite += inst.Metrics.CacheWrite
			gm.Cost += inst.Metrics.Cost
			gm.APICalls += inst.Metrics.APICalls

			// Track time bounds
			if inst.Metrics.StartTime != nil {
				if earliestStart == nil || inst.Metrics.StartTime.Before(*earliestStart) {
					earliestStart = inst.Metrics.StartTime
				}
			}
			if inst.Metrics.EndTime != nil {
				if latestEnd == nil || inst.Metrics.EndTime.After(*latestEnd) {
					latestEnd = inst.Metrics.EndTime
				}
				// Calculate duration for completed instances
				if inst.Metrics.StartTime != nil {
					totalCompletedDuration += inst.Metrics.EndTime.Sub(*inst.Metrics.StartTime)
				}
			}
		}
	}

	gm.StartTime = earliestStart
	gm.EndTime = latestEnd

	// Calculate total duration
	if earliestStart != nil {
		if latestEnd != nil {
			gm.Duration = latestEnd.Sub(*earliestStart)
		} else {
			gm.Duration = time.Since(*earliestStart)
		}
	}

	// Calculate average duration per completed instance
	if completedCount > 0 {
		gm.AverageDuration = totalCompletedDuration / time.Duration(completedCount)
	}

	return gm
}

// CalculateSessionGroupMetrics calculates metrics for all groups in a session.
func CalculateSessionGroupMetrics(session *orchestrator.Session) *SessionGroupMetrics {
	if session == nil || len(session.Groups) == 0 {
		return nil
	}

	sgm := &SessionGroupMetrics{
		Groups: make([]*GroupMetrics, 0, len(session.Groups)),
	}

	var totalAvgDuration time.Duration
	avgCount := 0

	for _, group := range session.Groups {
		gm := CalculateGroupMetrics(group, session)
		if gm != nil {
			sgm.Groups = append(sgm.Groups, gm)
			sgm.TotalCompleted += gm.Completed
			sgm.TotalInstances += gm.Total
			sgm.TotalCost += gm.Cost
			sgm.TotalDuration += gm.Duration

			if gm.AverageDuration > 0 {
				totalAvgDuration += gm.AverageDuration
				avgCount++
			}
		}
	}

	// Estimate ETA based on average completion time
	if avgCount > 0 && sgm.TotalCompleted < sgm.TotalInstances {
		avgDuration := totalAvgDuration / time.Duration(avgCount)
		remaining := sgm.TotalInstances - sgm.TotalCompleted
		sgm.EstimatedETA = avgDuration * time.Duration(remaining)
	}

	return sgm
}

// RenderOverallProgressBar renders the main session progress bar.
// Example: [=========>     ] 60%
func RenderOverallProgressBar(completed, total, width int) string {
	if total == 0 {
		return RenderProgressBar(0, width)
	}

	percent := (completed * 100) / total
	return fmt.Sprintf("%s %d%%", RenderProgressBar(percent, width), percent)
}

// RenderMiniProgressBar renders a compact progress indicator for groups.
// Example: [███░░] 3/5
func RenderMiniProgressBar(completed, total, barWidth int) string {
	if total == 0 {
		return "[" + strings.Repeat("░", barWidth) + "] 0/0"
	}

	filled := (completed * barWidth) / total
	empty := barWidth - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s] %d/%d", bar, completed, total)
}

// GroupProgressView renders group progress visualization.
type GroupProgressView struct {
	width int
}

// NewGroupProgressView creates a new group progress view.
func NewGroupProgressView(width int) *GroupProgressView {
	return &GroupProgressView{width: width}
}

// Render renders the full group progress panel.
func (v *GroupProgressView) Render(session *orchestrator.Session) string {
	sgm := CalculateSessionGroupMetrics(session)
	if sgm == nil {
		return styles.Muted.Render("No groups configured")
	}

	var b strings.Builder

	// Title
	b.WriteString(styles.SidebarTitle.Render("Group Progress"))
	b.WriteString("\n\n")

	// Overall progress bar
	overallBarWidth := min(20, v.width-20)
	overallProgress := RenderOverallProgressBar(sgm.TotalCompleted, sgm.TotalInstances, overallBarWidth)
	b.WriteString(styles.Text.Render("Overall: "))
	b.WriteString(overallProgress)
	b.WriteString("\n\n")

	// Per-group progress
	for _, gm := range sgm.Groups {
		groupLine := v.renderGroupLine(gm)
		b.WriteString(groupLine)
		b.WriteString("\n")
	}

	// ETA if available
	if sgm.EstimatedETA > 0 {
		b.WriteString("\n")
		etaStr := FormatDuration(sgm.EstimatedETA)
		b.WriteString(styles.Muted.Render(fmt.Sprintf("ETA: ~%s remaining", etaStr)))
	}

	// Total cost
	if sgm.TotalCost > 0 {
		b.WriteString("\n")
		costStr := instmetrics.FormatCost(sgm.TotalCost)
		b.WriteString(styles.Muted.Render(fmt.Sprintf("Total cost: %s", costStr)))
	}

	return b.String()
}

// renderGroupLine renders a single group's progress line.
func (v *GroupProgressView) renderGroupLine(gm *GroupMetrics) string {
	if gm == nil {
		return ""
	}

	var b strings.Builder

	// Phase indicator with color
	phaseColor := PhaseColor(gm.Phase)
	phaseIndicator := PhaseIndicator(gm.Phase)
	indicatorStyle := lipgloss.NewStyle().Foreground(phaseColor)

	// Group name (truncated)
	maxNameLen := max(v.width-35, 10) // Leave room for progress and metrics
	displayName := truncate(gm.GroupName, maxNameLen)

	// Mini progress bar
	miniBar := RenderMiniProgressBar(gm.Completed, gm.Total, 5)

	// Build line
	b.WriteString(indicatorStyle.Render(phaseIndicator))
	b.WriteString(" ")
	b.WriteString(lipgloss.NewStyle().Foreground(phaseColor).Render(displayName))
	b.WriteString(" ")
	b.WriteString(styles.Muted.Render(miniBar))

	// Add cost if significant
	if gm.Cost > 0.01 {
		costStr := instmetrics.FormatCost(gm.Cost)
		b.WriteString(" ")
		b.WriteString(styles.Muted.Render(costStr))
	}

	return b.String()
}

// RenderGroupMetricsCompact renders a compact metrics summary for a group.
// Example: "45.2K tok | $0.42 | 2m30s"
func RenderGroupMetricsCompact(gm *GroupMetrics) string {
	if gm == nil {
		return ""
	}

	var parts []string

	// Token count
	totalTokens := gm.InputTokens + gm.OutputTokens
	if totalTokens > 0 {
		parts = append(parts, instmetrics.FormatTokens(totalTokens)+" tok")
	}

	// Cost
	if gm.Cost > 0.01 {
		parts = append(parts, instmetrics.FormatCost(gm.Cost))
	}

	// Duration
	if gm.Duration > 0 {
		parts = append(parts, FormatDuration(gm.Duration))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " | ")
}

// RenderGroupProgressFooter renders a compact footer showing group progress.
// Designed to integrate with the existing help bar.
// Example: "Groups: [███░░] 3/5 (60%) | ETA: ~5m"
func RenderGroupProgressFooter(session *orchestrator.Session) string {
	sgm := CalculateSessionGroupMetrics(session)
	if sgm == nil {
		return ""
	}

	var parts []string

	// Groups progress
	miniBar := RenderMiniProgressBar(sgm.TotalCompleted, sgm.TotalInstances, 5)
	percent := 0
	if sgm.TotalInstances > 0 {
		percent = (sgm.TotalCompleted * 100) / sgm.TotalInstances
	}
	groupPart := fmt.Sprintf("Groups: %s (%d%%)", miniBar, percent)
	parts = append(parts, groupPart)

	// ETA
	if sgm.EstimatedETA > 0 {
		etaStr := FormatDuration(sgm.EstimatedETA)
		parts = append(parts, fmt.Sprintf("ETA: ~%s", etaStr))
	}

	// Total cost
	if sgm.TotalCost > 0.01 {
		parts = append(parts, instmetrics.FormatCost(sgm.TotalCost))
	}

	return strings.Join(parts, " | ")
}

// RenderGroupStatsPanel renders a detailed stats panel for groups.
// Designed to integrate with the :stats command when in grouped mode.
func RenderGroupStatsPanel(session *orchestrator.Session, width int) string {
	sgm := CalculateSessionGroupMetrics(session)
	if sgm == nil {
		return styles.Muted.Render("No groups configured")
	}

	var b strings.Builder

	// Title
	title := "📊 Group Statistics"
	b.WriteString(styles.Primary.Render(title))
	b.WriteString("\n\n")

	// Overall summary
	b.WriteString(styles.Secondary.Render("Overall Progress"))
	b.WriteString("\n")

	// Overall progress bar
	barWidth := min(30, width-20)
	overallProgress := RenderOverallProgressBar(sgm.TotalCompleted, sgm.TotalInstances, barWidth)
	b.WriteString(fmt.Sprintf("  %s\n", overallProgress))

	// Summary stats
	b.WriteString(fmt.Sprintf("  Completed: %d/%d instances\n", sgm.TotalCompleted, sgm.TotalInstances))
	if sgm.TotalCost > 0 {
		b.WriteString(fmt.Sprintf("  Total cost: %s\n", instmetrics.FormatCost(sgm.TotalCost)))
	}
	if sgm.EstimatedETA > 0 {
		b.WriteString(fmt.Sprintf("  Estimated remaining: ~%s\n", FormatDuration(sgm.EstimatedETA)))
	}
	b.WriteString("\n")

	// Per-group details
	b.WriteString(styles.Secondary.Render("Per-Group Breakdown"))
	b.WriteString("\n")

	for i, gm := range sgm.Groups {
		b.WriteString(renderGroupStatsEntry(gm, i+1, width))
	}

	return b.String()
}

// renderGroupStatsEntry renders a single group entry for the stats panel.
func renderGroupStatsEntry(gm *GroupMetrics, index, width int) string {
	if gm == nil {
		return ""
	}

	var b strings.Builder

	// Group header with phase indicator
	phaseColor := PhaseColor(gm.Phase)
	phaseIndicator := PhaseIndicator(gm.Phase)
	indicatorStyle := lipgloss.NewStyle().Foreground(phaseColor)
	nameStyle := lipgloss.NewStyle().Foreground(phaseColor).Bold(true)

	// Truncate name to fit within width
	maxNameLen := max(width-15, 20)
	displayName := truncate(gm.GroupName, maxNameLen)

	b.WriteString(fmt.Sprintf("  %d. %s %s\n",
		index,
		indicatorStyle.Render(phaseIndicator),
		nameStyle.Render(displayName)))

	// Progress
	miniBar := RenderMiniProgressBar(gm.Completed, gm.Total, 8)
	b.WriteString(fmt.Sprintf("     Progress: %s\n", miniBar))

	// Metrics
	totalTokens := gm.InputTokens + gm.OutputTokens
	if totalTokens > 0 {
		b.WriteString(fmt.Sprintf("     Tokens: %s (in: %s, out: %s)\n",
			instmetrics.FormatTokens(totalTokens),
			instmetrics.FormatTokens(gm.InputTokens),
			instmetrics.FormatTokens(gm.OutputTokens)))
	}

	if gm.Cost > 0.01 {
		b.WriteString(fmt.Sprintf("     Cost: %s\n", instmetrics.FormatCost(gm.Cost)))
	}

	if gm.Duration > 0 {
		b.WriteString(fmt.Sprintf("     Duration: %s\n", FormatDuration(gm.Duration)))
	}

	if gm.AverageDuration > 0 {
		b.WriteString(fmt.Sprintf("     Avg per instance: %s\n", FormatDuration(gm.AverageDuration)))
	}

	b.WriteString("\n")
	return b.String()
}

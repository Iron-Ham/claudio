package view

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// Pipeline-specific style aliases for better readability.
var (
	plHighlight = styles.Primary
	plSuccess   = styles.Secondary
	plWarning   = styles.Warning
	plError     = styles.Error
	plExec      = lipgloss.NewStyle().Foreground(styles.BlueColor).Bold(true)
	plReview    = lipgloss.NewStyle().Foreground(styles.PurpleColor)
)

// TeamSnapshot holds a point-in-time snapshot of a team's status.
// This is a TUI-local type with no backend imports.
//
// TasksTotal is an incremental count from bridge start events and may diverge
// from TasksDone+TasksFailed after UpdateTeamCompleted, which overwrites
// TasksDone/TasksFailed with backend-authoritative final counts.
type TeamSnapshot struct {
	ID          string
	Name        string
	Phase       string // "forming", "working", "blocked", "done", "failed"
	TasksDone   int
	TasksFailed int
	TasksTotal  int // estimated total (updated via bridge activity)
	ActiveTasks int // currently in-flight bridge tasks
}

// PipelineState holds the TUI-local snapshot of pipeline orchestration status.
// Built entirely from event data — no backend package imports required.
type PipelineState struct {
	PipelineID string
	Phase      string // "planning", "execution", "review", "consolidation", "done", "failed"
	Teams      []TeamSnapshot
	Completed  bool
	Success    bool
}

// IsActive returns true if the pipeline is in an active (non-terminal) phase.
func (p *PipelineState) IsActive() bool {
	if p == nil {
		return false
	}
	switch p.Phase {
	case "done", "failed", "":
		return false
	default:
		return true
	}
}

// HasActiveTeams returns true if any team is in a non-terminal phase.
func (p *PipelineState) HasActiveTeams() bool {
	if p == nil {
		return false
	}
	for i := range p.Teams {
		switch p.Teams[i].Phase {
		case "done", "failed":
			continue
		default:
			return true
		}
	}
	return false
}

// UpdatePhase transitions the pipeline to a new phase.
func (p *PipelineState) UpdatePhase(pipelineID, phase string) {
	if p == nil {
		return
	}
	p.PipelineID = pipelineID
	p.Phase = phase
}

// UpdateTeamPhase updates a team's phase, creating the team snapshot if needed.
func (p *PipelineState) UpdateTeamPhase(teamID, teamName, phase string) {
	if p == nil {
		return
	}
	for i := range p.Teams {
		if p.Teams[i].ID == teamID {
			p.Teams[i].Phase = phase
			if teamName != "" {
				p.Teams[i].Name = teamName
			}
			return
		}
	}
	// Team not yet tracked — add it
	p.Teams = append(p.Teams, TeamSnapshot{
		ID:    teamID,
		Name:  teamName,
		Phase: phase,
	})
}

// UpdateTeamCompleted marks a team as completed with final task counts.
func (p *PipelineState) UpdateTeamCompleted(teamID, teamName string, success bool, tasksDone, tasksFailed int) {
	if p == nil {
		return
	}
	for i := range p.Teams {
		if p.Teams[i].ID == teamID {
			if success {
				p.Teams[i].Phase = "done"
			} else {
				p.Teams[i].Phase = "failed"
			}
			p.Teams[i].TasksDone = tasksDone
			p.Teams[i].TasksFailed = tasksFailed
			if teamName != "" {
				p.Teams[i].Name = teamName
			}
			return
		}
	}
	// Team not yet tracked — add it in terminal state
	phase := "done"
	if !success {
		phase = "failed"
	}
	p.Teams = append(p.Teams, TeamSnapshot{
		ID:          teamID,
		Name:        teamName,
		Phase:       phase,
		TasksDone:   tasksDone,
		TasksFailed: tasksFailed,
		TasksTotal:  tasksDone + tasksFailed,
	})
}

// UpdateBridgeTaskActivity tracks bridge task starts and completions.
func (p *PipelineState) UpdateBridgeTaskActivity(teamID string, started, success bool) {
	if p == nil {
		return
	}
	for i := range p.Teams {
		if p.Teams[i].ID == teamID {
			if started {
				p.Teams[i].ActiveTasks++
				p.Teams[i].TasksTotal++
			} else {
				if p.Teams[i].ActiveTasks > 0 {
					p.Teams[i].ActiveTasks--
				}
				if success {
					p.Teams[i].TasksDone++
				} else {
					p.Teams[i].TasksFailed++
				}
			}
			return
		}
	}
}

// MarkCompleted marks the pipeline as finished.
func (p *PipelineState) MarkCompleted(success bool) {
	if p == nil {
		return
	}
	p.Completed = true
	p.Success = success
	if success {
		p.Phase = "done"
	} else {
		p.Phase = "failed"
	}
}

// GetIndicator builds a WorkflowIndicator for the current pipeline state.
// Returns nil if the pipeline has no phase set (not yet started via pipeline events).
func (p *PipelineState) GetIndicator() *WorkflowIndicator {
	if p == nil || p.Phase == "" {
		return nil
	}

	label, style := p.phaseDisplay()

	return &WorkflowIndicator{
		Icon:  styles.IconPipeline,
		Label: label,
		Style: style,
	}
}

// phaseDisplay returns a label and style for the current pipeline phase.
func (p *PipelineState) phaseDisplay() (string, lipgloss.Style) {
	switch p.Phase {
	case "planning":
		return "planning", plHighlight
	case "execution":
		return p.executionLabel(), plExec
	case "review":
		return "review", plReview
	case "consolidation":
		return "consolidating", plWarning
	case "done":
		return "done", plSuccess
	case "failed":
		return "failed", plError
	default:
		return p.Phase, plHighlight
	}
}

// executionLabel builds a compact label for the execution phase
// showing team/task progress.
func (p *PipelineState) executionLabel() string {
	workingTeams := 0
	totalDone := 0
	totalTasks := 0
	for i := range p.Teams {
		t := &p.Teams[i]
		switch t.Phase {
		case "done", "failed":
			// terminal
		default:
			workingTeams++
		}
		totalDone += t.TasksDone
		totalTasks += t.TasksTotal
	}

	if totalTasks > 0 {
		return fmt.Sprintf("exec %d/%d", totalDone, totalTasks)
	}
	if workingTeams > 0 {
		return fmt.Sprintf("exec %dt", workingTeams)
	}
	return "exec"
}

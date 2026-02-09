package view

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

func TestPipelineState_IsActive(t *testing.T) {
	tests := []struct {
		name  string
		state *PipelineState
		want  bool
	}{
		{"nil state", nil, false},
		{"empty phase", &PipelineState{}, false},
		{"planning", &PipelineState{Phase: "planning"}, true},
		{"execution", &PipelineState{Phase: "execution"}, true},
		{"review", &PipelineState{Phase: "review"}, true},
		{"consolidation", &PipelineState{Phase: "consolidation"}, true},
		{"done", &PipelineState{Phase: "done"}, false},
		{"failed", &PipelineState{Phase: "failed"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.IsActive(); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPipelineState_HasActiveTeams(t *testing.T) {
	tests := []struct {
		name  string
		state *PipelineState
		want  bool
	}{
		{"nil state", nil, false},
		{"no teams", &PipelineState{Phase: "execution"}, false},
		{
			"one working team",
			&PipelineState{
				Phase: "execution",
				Teams: []TeamSnapshot{{ID: "t1", Phase: "working"}},
			},
			true,
		},
		{
			"all done",
			&PipelineState{
				Phase: "execution",
				Teams: []TeamSnapshot{
					{ID: "t1", Phase: "done"},
					{ID: "t2", Phase: "failed"},
				},
			},
			false,
		},
		{
			"mixed",
			&PipelineState{
				Phase: "execution",
				Teams: []TeamSnapshot{
					{ID: "t1", Phase: "done"},
					{ID: "t2", Phase: "working"},
				},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.state.HasActiveTeams(); got != tt.want {
				t.Errorf("HasActiveTeams() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPipelineState_UpdatePhase(t *testing.T) {
	t.Run("sets pipeline ID and phase", func(t *testing.T) {
		p := &PipelineState{}
		p.UpdatePhase("pipe-1", "execution")
		if p.PipelineID != "pipe-1" {
			t.Errorf("PipelineID = %q, want %q", p.PipelineID, "pipe-1")
		}
		if p.Phase != "execution" {
			t.Errorf("Phase = %q, want %q", p.Phase, "execution")
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		var p *PipelineState
		p.UpdatePhase("pipe-1", "execution") // should not panic
	})
}

func TestPipelineState_UpdateTeamPhase(t *testing.T) {
	t.Run("adds new team", func(t *testing.T) {
		p := &PipelineState{Phase: "execution"}
		p.UpdateTeamPhase("team-1", "Frontend", "working")
		if len(p.Teams) != 1 {
			t.Fatalf("Teams len = %d, want 1", len(p.Teams))
		}
		if p.Teams[0].ID != "team-1" || p.Teams[0].Name != "Frontend" || p.Teams[0].Phase != "working" {
			t.Errorf("Team = %+v, unexpected", p.Teams[0])
		}
	})

	t.Run("updates existing team", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "team-1", Name: "Frontend", Phase: "forming"}},
		}
		p.UpdateTeamPhase("team-1", "", "working")
		if p.Teams[0].Phase != "working" {
			t.Errorf("Phase = %q, want %q", p.Teams[0].Phase, "working")
		}
		if p.Teams[0].Name != "Frontend" {
			t.Errorf("Name = %q, want %q (should preserve existing)", p.Teams[0].Name, "Frontend")
		}
	})

	t.Run("updates name when provided", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "team-1", Name: "Old", Phase: "forming"}},
		}
		p.UpdateTeamPhase("team-1", "New", "working")
		if p.Teams[0].Name != "New" {
			t.Errorf("Name = %q, want %q", p.Teams[0].Name, "New")
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		var p *PipelineState
		p.UpdateTeamPhase("t1", "n", "working") // should not panic
	})
}

func TestPipelineState_UpdateTeamCompleted(t *testing.T) {
	t.Run("marks existing team as done", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Name: "A", Phase: "working"}},
		}
		p.UpdateTeamCompleted("t1", "", true, 5, 0)
		if p.Teams[0].Phase != "done" {
			t.Errorf("Phase = %q, want %q", p.Teams[0].Phase, "done")
		}
		if p.Teams[0].TasksDone != 5 {
			t.Errorf("TasksDone = %d, want %d", p.Teams[0].TasksDone, 5)
		}
	})

	t.Run("marks failure", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Phase: "working"}},
		}
		p.UpdateTeamCompleted("t1", "", false, 3, 2)
		if p.Teams[0].Phase != "failed" {
			t.Errorf("Phase = %q, want %q", p.Teams[0].Phase, "failed")
		}
		if p.Teams[0].TasksFailed != 2 {
			t.Errorf("TasksFailed = %d, want %d", p.Teams[0].TasksFailed, 2)
		}
	})

	t.Run("creates team if not tracked", func(t *testing.T) {
		p := &PipelineState{Phase: "execution"}
		p.UpdateTeamCompleted("t1", "New", true, 4, 0)
		if len(p.Teams) != 1 {
			t.Fatalf("Teams len = %d, want 1", len(p.Teams))
		}
		if p.Teams[0].Phase != "done" || p.Teams[0].Name != "New" {
			t.Errorf("Team = %+v, unexpected", p.Teams[0])
		}
	})

	t.Run("creates failed team if not tracked", func(t *testing.T) {
		p := &PipelineState{Phase: "execution"}
		p.UpdateTeamCompleted("t1", "Backend", false, 3, 2)
		if len(p.Teams) != 1 {
			t.Fatalf("Teams len = %d, want 1", len(p.Teams))
		}
		if p.Teams[0].Phase != "failed" {
			t.Errorf("Phase = %q, want %q", p.Teams[0].Phase, "failed")
		}
		if p.Teams[0].TasksTotal != 5 {
			t.Errorf("TasksTotal = %d, want %d", p.Teams[0].TasksTotal, 5)
		}
	})

	t.Run("updates name on existing team", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Name: "Old", Phase: "working"}},
		}
		p.UpdateTeamCompleted("t1", "Renamed", true, 5, 0)
		if p.Teams[0].Name != "Renamed" {
			t.Errorf("Name = %q, want %q", p.Teams[0].Name, "Renamed")
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		var p *PipelineState
		p.UpdateTeamCompleted("t1", "n", true, 1, 0) // should not panic
	})
}

func TestPipelineState_UpdateBridgeTaskActivity(t *testing.T) {
	t.Run("start increments active and total", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Phase: "working"}},
		}
		p.UpdateBridgeTaskActivity("t1", true, false)
		if p.Teams[0].ActiveTasks != 1 {
			t.Errorf("ActiveTasks = %d, want %d", p.Teams[0].ActiveTasks, 1)
		}
		if p.Teams[0].TasksTotal != 1 {
			t.Errorf("TasksTotal = %d, want %d", p.Teams[0].TasksTotal, 1)
		}
	})

	t.Run("successful completion decrements active and increments done", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Phase: "working", ActiveTasks: 2, TasksTotal: 3}},
		}
		p.UpdateBridgeTaskActivity("t1", false, true)
		if p.Teams[0].ActiveTasks != 1 {
			t.Errorf("ActiveTasks = %d, want %d", p.Teams[0].ActiveTasks, 1)
		}
		if p.Teams[0].TasksDone != 1 {
			t.Errorf("TasksDone = %d, want %d", p.Teams[0].TasksDone, 1)
		}
	})

	t.Run("failed completion increments failed", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Phase: "working", ActiveTasks: 1, TasksTotal: 2}},
		}
		p.UpdateBridgeTaskActivity("t1", false, false)
		if p.Teams[0].TasksFailed != 1 {
			t.Errorf("TasksFailed = %d, want %d", p.Teams[0].TasksFailed, 1)
		}
	})

	t.Run("active tasks does not go negative", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{{ID: "t1", Phase: "working", ActiveTasks: 0}},
		}
		p.UpdateBridgeTaskActivity("t1", false, true)
		if p.Teams[0].ActiveTasks != 0 {
			t.Errorf("ActiveTasks = %d, want %d", p.Teams[0].ActiveTasks, 0)
		}
	})

	t.Run("unknown team is a no-op", func(t *testing.T) {
		p := &PipelineState{Phase: "execution"}
		p.UpdateBridgeTaskActivity("unknown", true, false) // should not panic
	})

	t.Run("nil safety", func(t *testing.T) {
		var p *PipelineState
		p.UpdateBridgeTaskActivity("t1", true, false) // should not panic
	})
}

func TestPipelineState_MarkCompleted(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		p := &PipelineState{Phase: "consolidation"}
		p.MarkCompleted(true)
		if !p.Completed || !p.Success || p.Phase != "done" {
			t.Errorf("After MarkCompleted(true): Completed=%v, Success=%v, Phase=%q", p.Completed, p.Success, p.Phase)
		}
	})

	t.Run("failure", func(t *testing.T) {
		p := &PipelineState{Phase: "execution"}
		p.MarkCompleted(false)
		if !p.Completed || p.Success || p.Phase != "failed" {
			t.Errorf("After MarkCompleted(false): Completed=%v, Success=%v, Phase=%q", p.Completed, p.Success, p.Phase)
		}
	})

	t.Run("nil safety", func(t *testing.T) {
		var p *PipelineState
		p.MarkCompleted(true) // should not panic
	})
}

func TestPipelineState_GetIndicator(t *testing.T) {
	t.Run("nil state returns nil", func(t *testing.T) {
		var p *PipelineState
		if ind := p.GetIndicator(); ind != nil {
			t.Errorf("GetIndicator() = %+v, want nil", ind)
		}
	})

	t.Run("empty phase returns nil", func(t *testing.T) {
		p := &PipelineState{}
		if ind := p.GetIndicator(); ind != nil {
			t.Errorf("GetIndicator() = %+v, want nil for empty phase", ind)
		}
	})

	t.Run("planning phase", func(t *testing.T) {
		p := &PipelineState{Phase: "planning"}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Icon != styles.IconPipeline {
			t.Errorf("Icon = %q, want %q", ind.Icon, styles.IconPipeline)
		}
		if ind.Label != "planning" {
			t.Errorf("Label = %q, want %q", ind.Label, "planning")
		}
	})

	t.Run("execution phase with tasks", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{
				{ID: "t1", Phase: "working", TasksDone: 3, TasksTotal: 10},
				{ID: "t2", Phase: "working", TasksDone: 2, TasksTotal: 5},
			},
		}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "exec 5/15" {
			t.Errorf("Label = %q, want %q", ind.Label, "exec 5/15")
		}
	})

	t.Run("execution phase without tasks shows team count", func(t *testing.T) {
		p := &PipelineState{
			Phase: "execution",
			Teams: []TeamSnapshot{
				{ID: "t1", Phase: "working"},
				{ID: "t2", Phase: "forming"},
			},
		}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "exec 2t" {
			t.Errorf("Label = %q, want %q", ind.Label, "exec 2t")
		}
	})

	t.Run("execution phase no teams", func(t *testing.T) {
		p := &PipelineState{Phase: "execution"}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "exec" {
			t.Errorf("Label = %q, want %q", ind.Label, "exec")
		}
	})

	t.Run("review phase", func(t *testing.T) {
		p := &PipelineState{Phase: "review"}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "review" {
			t.Errorf("Label = %q, want %q", ind.Label, "review")
		}
	})

	t.Run("consolidation phase", func(t *testing.T) {
		p := &PipelineState{Phase: "consolidation"}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "consolidating" {
			t.Errorf("Label = %q, want %q", ind.Label, "consolidating")
		}
	})

	t.Run("done phase", func(t *testing.T) {
		p := &PipelineState{Phase: "done", Completed: true, Success: true}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "done" {
			t.Errorf("Label = %q, want %q", ind.Label, "done")
		}
	})

	t.Run("failed phase", func(t *testing.T) {
		p := &PipelineState{Phase: "failed", Completed: true}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "failed" {
			t.Errorf("Label = %q, want %q", ind.Label, "failed")
		}
	})

	t.Run("unknown phase falls back to phase name", func(t *testing.T) {
		p := &PipelineState{Phase: "custom-phase"}
		ind := p.GetIndicator()
		if ind == nil {
			t.Fatal("GetIndicator() = nil, want non-nil")
		}
		if ind.Label != "custom-phase" {
			t.Errorf("Label = %q, want %q", ind.Label, "custom-phase")
		}
	})
}

package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func newTestPipeline(t *testing.T, plan *ultraplan.PlanSpec, opts ...PipelineOption) (*Pipeline, *event.Bus) {
	t.Helper()
	bus := event.NewBus()
	cfg := PipelineConfig{
		Bus:     bus,
		BaseDir: t.TempDir(),
		Plan:    plan,
	}
	// Always disable rebalance to avoid background goroutine interference.
	opts = append(opts, WithHubOptions(coordination.WithRebalanceInterval(-1)))
	p, err := NewPipeline(cfg, opts...)
	if err != nil {
		t.Fatalf("NewPipeline: %v", err)
	}
	return p, bus
}

func simplePlan() *ultraplan.PlanSpec {
	return &ultraplan.PlanSpec{
		ID:        "test-plan",
		Objective: "Test objective",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
		},
	}
}

func TestNewPipeline_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PipelineConfig
		wantErr string
	}{
		{
			name:    "missing bus",
			cfg:     PipelineConfig{BaseDir: "/tmp", Plan: simplePlan()},
			wantErr: "Bus is required",
		},
		{
			name:    "missing dir",
			cfg:     PipelineConfig{Bus: event.NewBus(), Plan: simplePlan()},
			wantErr: "BaseDir is required",
		},
		{
			name:    "missing plan",
			cfg:     PipelineConfig{Bus: event.NewBus(), BaseDir: "/tmp"},
			wantErr: "Plan is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPipeline(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPipeline_DecomposeBeforeStart(t *testing.T) {
	p, _ := newTestPipeline(t, simplePlan())

	result, err := p.Decompose(DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if len(result.ExecutionTeams) != 1 {
		t.Errorf("ExecutionTeams = %d, want 1", len(result.ExecutionTeams))
	}
}

func TestPipeline_StartWithoutDecompose(t *testing.T) {
	p, _ := newTestPipeline(t, simplePlan())

	err := p.Start(context.Background())
	if err == nil {
		t.Fatal("expected error starting without Decompose")
	}
	if !strings.Contains(err.Error(), "Decompose must be called") {
		t.Errorf("error = %q, want containing 'Decompose must be called'", err.Error())
	}
}

func TestPipeline_DecomposeAfterStart(t *testing.T) {
	p, _ := newTestPipeline(t, simplePlan())

	_, _ = p.Decompose(DecomposeConfig{})
	_ = p.Start(context.Background())
	defer func() { _ = p.Stop() }()

	_, err := p.Decompose(DecomposeConfig{})
	if err == nil {
		t.Fatal("expected error decomposing after Start")
	}
}

func TestPipeline_DoubleStart(t *testing.T) {
	p, _ := newTestPipeline(t, simplePlan())
	_, _ = p.Decompose(DecomposeConfig{})
	_ = p.Start(context.Background())
	defer func() { _ = p.Stop() }()

	err := p.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for double start")
	}
}

func TestPipeline_StopIdempotent(t *testing.T) {
	p, _ := newTestPipeline(t, simplePlan())
	_, _ = p.Decompose(DecomposeConfig{})
	_ = p.Start(context.Background())

	if err := p.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := p.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
}

func TestPipeline_StopWithoutStart(t *testing.T) {
	p, _ := newTestPipeline(t, simplePlan())
	if err := p.Stop(); err != nil {
		t.Fatalf("Stop without Start: %v", err)
	}
}

func TestPipeline_ExecutionPhase(t *testing.T) {
	plan := simplePlan()
	p, bus := newTestPipeline(t, plan)

	_, _ = p.Decompose(DecomposeConfig{})

	// Collect pipeline phase changes.
	phaseChanges := make(chan event.Event, 20)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	completions := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		completions <- e
	})

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for execution phase to begin.
	waitForPipelinePhase(t, phaseChanges, "execution", 2*time.Second)

	// Complete the execution team's tasks to advance the pipeline.
	completeAllTeamTasks(t, p, PhaseExecution)

	// Pipeline should complete.
	select {
	case e := <-completions:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
		if pce.PhasesRun != 1 {
			t.Errorf("PhasesRun = %d, want 1", pce.PhasesRun)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pipeline completion")
	}

	if p.Phase() != PhaseDone {
		t.Errorf("Phase = %v, want %v", p.Phase(), PhaseDone)
	}
}

func TestPipeline_AllPhases(t *testing.T) {
	plan := simplePlan()
	p, bus := newTestPipeline(t, plan)

	_, _ = p.Decompose(DecomposeConfig{
		PlanningTeam:      true,
		ReviewTeam:        true,
		ConsolidationTeam: true,
	})

	phaseChanges := make(chan event.Event, 30)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	completions := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		completions <- e
	})

	ctx := context.Background()
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Planning phase.
	waitForPipelinePhase(t, phaseChanges, "planning", 2*time.Second)
	completeAllTeamTasks(t, p, PhasePlanning)

	// Execution phase.
	waitForPipelinePhase(t, phaseChanges, "execution", 2*time.Second)
	completeAllTeamTasks(t, p, PhaseExecution)

	// Review phase.
	waitForPipelinePhase(t, phaseChanges, "review", 2*time.Second)
	completeAllTeamTasks(t, p, PhaseReview)

	// Consolidation phase.
	waitForPipelinePhase(t, phaseChanges, "consolidation", 2*time.Second)
	completeAllTeamTasks(t, p, PhaseConsolidation)

	// Pipeline should complete.
	select {
	case e := <-completions:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
		if pce.PhasesRun != 4 {
			t.Errorf("PhasesRun = %d, want 4", pce.PhasesRun)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pipeline completion")
	}
}

func TestPipeline_ContextCancellation(t *testing.T) {
	plan := simplePlan()
	p, bus := newTestPipeline(t, plan)

	_, _ = p.Decompose(DecomposeConfig{})

	phaseChanges := make(chan event.Event, 20)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	ctx, cancel := context.WithCancel(context.Background())
	if err := p.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for execution to begin.
	waitForPipelinePhase(t, phaseChanges, "execution", 2*time.Second)

	// Cancel and let the pipeline fail gracefully.
	cancel()

	// Give time for the pipeline's run goroutine to handle cancellation.
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			// Pipeline should be in a terminal state or still running with cancelled context.
			// Just stop it.
			_ = p.Stop()
			return
		default:
		}
		phase := p.Phase()
		if phase.IsTerminal() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	_ = p.Stop()
}

func TestPipeline_ManagerAccessor(t *testing.T) {
	plan := simplePlan()
	p, bus := newTestPipeline(t, plan)

	_, _ = p.Decompose(DecomposeConfig{})

	// Before start, no managers exist.
	if m := p.Manager(PhaseExecution); m != nil {
		t.Error("Manager should be nil before Start")
	}

	phaseChanges := make(chan event.Event, 20)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	_ = p.Start(context.Background())
	defer func() { _ = p.Stop() }()

	// Wait for execution to begin.
	waitForPipelinePhase(t, phaseChanges, "execution", 2*time.Second)

	if m := p.Manager(PhaseExecution); m == nil {
		t.Error("Manager should exist after execution phase starts")
	}
}

func TestPipeline_RunningState(t *testing.T) {
	plan := simplePlan()
	p, _ := newTestPipeline(t, plan)

	if p.Running() {
		t.Error("should not be running before Start")
	}

	_, _ = p.Decompose(DecomposeConfig{})
	_ = p.Start(context.Background())

	if !p.Running() {
		t.Error("should be running after Start")
	}

	_ = p.Stop()

	if p.Running() {
		t.Error("should not be running after Stop")
	}
}

func TestPipeline_ExecutionFailure(t *testing.T) {
	plan := simplePlan()
	p, bus := newTestPipeline(t, plan)

	_, _ = p.Decompose(DecomposeConfig{
		ReviewTeam: true, // review phase should NOT run if execution fails
	})

	phaseChanges := make(chan event.Event, 20)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	completions := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		completions <- e
	})

	_ = p.Start(context.Background())
	defer func() { _ = p.Stop() }()

	// Wait for execution phase.
	waitForPipelinePhase(t, phaseChanges, "execution", 2*time.Second)

	// Fail the task instead of completing it.
	failAllTeamTasks(t, p, PhaseExecution)

	// Pipeline should fail.
	select {
	case e := <-completions:
		pce := e.(event.PipelineCompletedEvent)
		if pce.Success {
			t.Error("pipeline should have failed")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pipeline failure")
	}

	if p.Phase() != PhaseFailed {
		t.Errorf("Phase = %v, want %v", p.Phase(), PhaseFailed)
	}
}

func TestPipeline_MultipleExecutionTeams(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "multi-plan",
		Objective: "Multi team test",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"b.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"c.go"}},
		},
	}
	p, bus := newTestPipeline(t, plan)

	result, _ := p.Decompose(DecomposeConfig{})
	if len(result.ExecutionTeams) != 3 {
		t.Fatalf("expected 3 execution teams, got %d", len(result.ExecutionTeams))
	}

	phaseChanges := make(chan event.Event, 20)
	bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})
	completions := make(chan event.Event, 5)
	bus.Subscribe("pipeline.completed", func(e event.Event) {
		completions <- e
	})

	_ = p.Start(context.Background())
	defer func() { _ = p.Stop() }()

	waitForPipelinePhase(t, phaseChanges, "execution", 2*time.Second)
	completeAllTeamTasks(t, p, PhaseExecution)

	select {
	case e := <-completions:
		pce := e.(event.PipelineCompletedEvent)
		if !pce.Success {
			t.Error("pipeline should have succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for pipeline completion")
	}
}

// -- Test helpers ------------------------------------------------------------

// waitForPipelinePhase waits for a specific pipeline phase change event.
func waitForPipelinePhase(t *testing.T, ch <-chan event.Event, phase string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case e := <-ch:
			pce := e.(event.PipelinePhaseChangedEvent)
			if pce.CurrentPhase == phase {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for pipeline phase %q", phase)
		}
	}
}

// failAllTeamTasks fails all tasks in all teams for the given pipeline phase.
func failAllTeamTasks(t *testing.T, p *Pipeline, phase PipelinePhase) {
	t.Helper()

	deadline := time.After(2 * time.Second)
	for {
		m := p.Manager(phase)
		if m != nil {
			for _, s := range m.AllStatuses() {
				tm := m.Team(s.ID)
				if tm == nil {
					continue
				}
				eq := tm.Hub().EventQueue()
				for {
					task, err := eq.ClaimNext("test-instance")
					if err != nil || task == nil {
						break
					}
					if err := eq.MarkRunning(task.ID); err != nil {
						t.Fatalf("MarkRunning(%s): %v", task.ID, err)
					}
					if err := eq.Fail(task.ID, "intentional failure"); err != nil {
						t.Fatalf("Fail(%s): %v", task.ID, err)
					}
				}
			}
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for manager in phase %s", phase)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// completeAllTeamTasks completes all tasks in all teams for the given pipeline phase.
// It polls until all teams are in terminal phases, claiming and completing tasks
// as they become available.
func completeAllTeamTasks(t *testing.T, p *Pipeline, phase PipelinePhase) {
	t.Helper()

	deadline := time.After(5 * time.Second)
	for {
		m := p.Manager(phase)
		if m == nil {
			select {
			case <-deadline:
				t.Fatalf("timed out waiting for manager in phase %s", phase)
			default:
				time.Sleep(10 * time.Millisecond)
				continue
			}
		}

		// Try to claim and complete tasks from all teams.
		for _, s := range m.AllStatuses() {
			tm := m.Team(s.ID)
			if tm == nil {
				continue
			}
			eq := tm.Hub().EventQueue()
			for {
				task, err := eq.ClaimNext("test-instance")
				if err != nil || task == nil {
					break
				}
				if err := eq.MarkRunning(task.ID); err != nil {
					t.Fatalf("MarkRunning(%s): %v", task.ID, err)
				}
				if _, err := eq.Complete(task.ID); err != nil {
					t.Fatalf("Complete(%s): %v", task.ID, err)
				}
			}
		}

		// Check if all teams are done.
		allDone := true
		for _, s := range m.AllStatuses() {
			if !s.Phase.IsTerminal() {
				allDone = false
				break
			}
		}
		if allDone {
			return
		}

		select {
		case <-deadline:
			t.Fatalf("timed out completing tasks in phase %s", phase)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

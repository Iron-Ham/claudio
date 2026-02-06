package team

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func makeTestHub(t *testing.T, bus *event.Bus, tasks []ultraplan.PlannedTask) *coordination.Hub {
	t.Helper()
	plan := &ultraplan.PlanSpec{
		ID:        "test-plan",
		Objective: "test",
		Tasks:     tasks,
	}
	hub, err := coordination.NewHub(coordination.Config{
		Bus:        bus,
		SessionDir: t.TempDir(),
		Plan:       plan,
	},
		coordination.WithRebalanceInterval(-1),
	)
	if err != nil {
		t.Fatalf("NewHub: %v", err)
	}
	return hub
}

func TestTeam_NewTeam(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{{ID: "t1", Title: "Task 1"}}
	hub := makeTestHub(t, bus, tasks)
	bt := newBudgetTracker("team-1", TokenBudget{}, bus)

	spec := Spec{
		ID:       "team-1",
		Name:     "Alpha",
		Role:     RoleExecution,
		Tasks:    tasks,
		TeamSize: 2,
	}

	team := newTeam(spec, hub, bt)

	if team.Phase() != PhaseForming {
		t.Errorf("Phase() = %v, want %v", team.Phase(), PhaseForming)
	}
	if team.Hub() != hub {
		t.Error("Hub() did not return the expected hub")
	}
	if team.Spec().ID != "team-1" {
		t.Errorf("Spec().ID = %q, want %q", team.Spec().ID, "team-1")
	}
}

func TestTeam_SetPhase(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{{ID: "t1", Title: "Task 1"}}
	hub := makeTestHub(t, bus, tasks)
	bt := newBudgetTracker("team-1", TokenBudget{}, bus)

	spec := Spec{
		ID:       "team-1",
		Name:     "Alpha",
		Role:     RoleExecution,
		Tasks:    tasks,
		TeamSize: 2,
	}

	team := newTeam(spec, hub, bt)

	prev := team.setPhase(PhaseWorking)
	if prev != PhaseForming {
		t.Errorf("setPhase returned previous = %v, want %v", prev, PhaseForming)
	}
	if team.Phase() != PhaseWorking {
		t.Errorf("Phase() = %v, want %v", team.Phase(), PhaseWorking)
	}
}

func TestTeam_Status(t *testing.T) {
	bus := event.NewBus()
	tasks := []ultraplan.PlannedTask{
		{ID: "t1", Title: "Task 1"},
		{ID: "t2", Title: "Task 2"},
	}
	hub := makeTestHub(t, bus, tasks)
	bt := newBudgetTracker("team-1", TokenBudget{MaxInputTokens: 1000}, bus)

	spec := Spec{
		ID:       "team-1",
		Name:     "Alpha",
		Role:     RoleExecution,
		Tasks:    tasks,
		TeamSize: 2,
	}

	team := newTeam(spec, hub, bt)
	team.setPhase(PhaseWorking)
	bt.Record(100, 50, 1.0)

	status := team.Status()

	if status.ID != "team-1" {
		t.Errorf("Status.ID = %q, want %q", status.ID, "team-1")
	}
	if status.Name != "Alpha" {
		t.Errorf("Status.Name = %q, want %q", status.Name, "Alpha")
	}
	if status.Role != RoleExecution {
		t.Errorf("Status.Role = %v, want %v", status.Role, RoleExecution)
	}
	if status.Phase != PhaseWorking {
		t.Errorf("Status.Phase = %v, want %v", status.Phase, PhaseWorking)
	}
	if status.TasksTotal != 2 {
		t.Errorf("Status.TasksTotal = %d, want 2", status.TasksTotal)
	}
	if status.BudgetUsed.InputTokens != 100 {
		t.Errorf("Status.BudgetUsed.InputTokens = %d, want 100", status.BudgetUsed.InputTokens)
	}
}

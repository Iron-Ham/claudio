package team

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func newTestManager(t *testing.T, opts ...ManagerOption) (*Manager, *event.Bus) {
	t.Helper()
	bus := event.NewBus()
	m, err := NewManager(ManagerConfig{
		Bus:     bus,
		BaseDir: t.TempDir(),
	}, opts...)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m, bus
}

func testSpec(id, name string, deps ...string) Spec {
	return Spec{
		ID:        id,
		Name:      name,
		Role:      RoleExecution,
		Tasks:     []ultraplan.PlannedTask{{ID: "t-" + id, Title: "Task for " + name}},
		TeamSize:  1,
		DependsOn: deps,
	}
}

func TestNewManager_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ManagerConfig
		wantErr string
	}{
		{"missing bus", ManagerConfig{BaseDir: "/tmp"}, "Bus is required"},
		{"missing dir", ManagerConfig{Bus: event.NewBus()}, "BaseDir is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewManager(tt.cfg)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestManager_AddTeam(t *testing.T) {
	m, bus := newTestManager(t)

	ch := make(chan event.Event, 1)
	bus.Subscribe("team.created", func(e event.Event) {
		ch <- e
	})

	spec := testSpec("alpha", "Alpha Team")
	if err := m.AddTeam(spec); err != nil {
		t.Fatalf("AddTeam: %v", err)
	}

	// Verify created event.
	select {
	case e := <-ch:
		tce, ok := e.(event.TeamCreatedEvent)
		if !ok {
			t.Fatalf("expected TeamCreatedEvent, got %T", e)
		}
		if tce.TeamID != "alpha" {
			t.Errorf("TeamID = %q, want %q", tce.TeamID, "alpha")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for created event")
	}

	// Verify team is accessible.
	team := m.Team("alpha")
	if team == nil {
		t.Fatal("Team(alpha) = nil")
	}
	if team.Phase() != PhaseForming {
		t.Errorf("Phase = %v, want %v", team.Phase(), PhaseForming)
	}
}

func TestManager_AddTeam_DuplicateID(t *testing.T) {
	m, _ := newTestManager(t)

	spec := testSpec("alpha", "Alpha Team")
	if err := m.AddTeam(spec); err != nil {
		t.Fatalf("first AddTeam: %v", err)
	}

	err := m.AddTeam(spec)
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want containing 'duplicate'", err.Error())
	}
}

func TestManager_AddTeam_InvalidSpec(t *testing.T) {
	m, _ := newTestManager(t)

	err := m.AddTeam(Spec{}) // empty spec
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestManager_Start_NoDependencies(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)

	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.AddTeam(testSpec("beta", "Beta"))

	phaseChanges := make(chan event.Event, 10)
	bus.Subscribe("team.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = m.Stop() }()

	if !m.Running() {
		t.Error("Running() = false, want true")
	}

	// Both teams should transition to Working.
	receivedWorking := 0
	timeout := time.After(time.Second)
	for receivedWorking < 2 {
		select {
		case e := <-phaseChanges:
			pce := e.(event.TeamPhaseChangedEvent)
			if pce.CurrentPhase == string(PhaseWorking) {
				receivedWorking++
			}
		case <-timeout:
			t.Fatalf("timed out: got %d working events, want 2", receivedWorking)
		}
	}

	// Verify phases via status.
	statuses := m.AllStatuses()
	if len(statuses) != 2 {
		t.Fatalf("AllStatuses() len = %d, want 2", len(statuses))
	}
	for _, s := range statuses {
		if s.Phase != PhaseWorking {
			t.Errorf("team %q phase = %v, want %v", s.ID, s.Phase, PhaseWorking)
		}
	}
}

func TestManager_Start_WithDependencies(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)

	// Beta depends on Alpha.
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.AddTeam(testSpec("beta", "Beta", "alpha"))

	phaseChanges := make(chan event.Event, 20)
	bus.Subscribe("team.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = m.Stop() }()

	// Alpha should be Working, Beta should be Blocked.
	waitForPhase := func(teamID, phase string) {
		t.Helper()
		timeout := time.After(time.Second)
		for {
			select {
			case e := <-phaseChanges:
				pce := e.(event.TeamPhaseChangedEvent)
				if pce.TeamID == teamID && pce.CurrentPhase == phase {
					return
				}
			case <-timeout:
				t.Fatalf("timed out waiting for team %q to reach phase %q", teamID, phase)
			}
		}
	}

	waitForPhase("alpha", string(PhaseWorking))
	waitForPhase("beta", string(PhaseBlocked))

	// Verify beta is blocked.
	status, ok := m.TeamStatus("beta")
	if !ok {
		t.Fatal("TeamStatus(beta) not found")
	}
	if status.Phase != PhaseBlocked {
		t.Errorf("beta phase = %v, want %v", status.Phase, PhaseBlocked)
	}
}

func TestManager_Start_UnknownDependency(t *testing.T) {
	m, _ := newTestManager(t)

	_ = m.AddTeam(testSpec("alpha", "Alpha", "nonexistent"))

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "unknown team") {
		t.Errorf("error = %q, want containing 'unknown team'", err.Error())
	}
}

func TestManager_Start_DoubleStart(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))

	ctx := context.Background()
	_ = m.Start(ctx)
	defer func() { _ = m.Stop() }()

	err := m.Start(ctx)
	if err == nil {
		t.Fatal("expected error for double start")
	}
}

func TestManager_AddTeam_AfterStart(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.Start(context.Background())
	defer func() { _ = m.Stop() }()

	err := m.AddTeam(testSpec("beta", "Beta"))
	if err == nil {
		t.Fatal("expected error adding team after start")
	}
}

func TestManager_Stop_Idempotent(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.Start(context.Background())

	if err := m.Stop(); err != nil {
		t.Fatalf("first Stop: %v", err)
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}

	if m.Running() {
		t.Error("Running() = true after Stop")
	}
}

func TestManager_Stop_WithoutStart(t *testing.T) {
	m, _ := newTestManager(t)
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop without Start: %v", err)
	}
}

func TestManager_Start_NoTeams(t *testing.T) {
	m, _ := newTestManager(t)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error starting with no teams")
	}
}

func TestManager_RouteMessage(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)

	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.AddTeam(testSpec("beta", "Beta"))

	_ = m.Start(context.Background())
	defer func() { _ = m.Stop() }()

	ch := make(chan event.Event, 2)
	bus.Subscribe("team.message", func(e event.Event) {
		ch <- e
	})

	err := m.RouteMessage(InterTeamMessage{
		FromTeam: "alpha",
		ToTeam:   "beta",
		Type:     MessageTypeDiscovery,
		Content:  "found it",
		Priority: PriorityInfo,
	})
	if err != nil {
		t.Fatalf("RouteMessage: %v", err)
	}

	select {
	case e := <-ch:
		ite := e.(event.InterTeamMessageEvent)
		if ite.FromTeam != "alpha" || ite.ToTeam != "beta" {
			t.Errorf("event routing: from=%q to=%q", ite.FromTeam, ite.ToTeam)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for message event")
	}
}

func TestManager_AllStatuses_InsertionOrder(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)

	ids := []string{"charlie", "alpha", "beta"}
	for _, id := range ids {
		_ = m.AddTeam(testSpec(id, "Team "+id))
	}

	statuses := m.AllStatuses()
	if len(statuses) != 3 {
		t.Fatalf("AllStatuses() len = %d, want 3", len(statuses))
	}
	for i, id := range ids {
		if statuses[i].ID != id {
			t.Errorf("status[%d].ID = %q, want %q", i, statuses[i].ID, id)
		}
	}
}

func TestManager_TeamStatus_NotFound(t *testing.T) {
	m, _ := newTestManager(t)

	_, ok := m.TeamStatus("nonexistent")
	if ok {
		t.Error("TeamStatus should return false for nonexistent team")
	}
}

func TestManager_TeamNotFound(t *testing.T) {
	m, _ := newTestManager(t)

	if m.Team("nonexistent") != nil {
		t.Error("Team should return nil for nonexistent ID")
	}
}

func TestManager_ContextCancellation(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))

	ctx, cancel := context.WithCancel(context.Background())
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Cancel the context — this should cause the monitor goroutine to exit.
	cancel()

	// Stop should succeed cleanly.
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}

func TestManager_DependencyCascade(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)

	// Chain: alpha -> beta -> gamma
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.AddTeam(testSpec("beta", "Beta", "alpha"))
	_ = m.AddTeam(testSpec("gamma", "Gamma", "beta"))

	phaseChanges := make(chan event.Event, 30)
	bus.Subscribe("team.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = m.Stop() }()

	// Alpha should be working; beta and gamma blocked.
	waitForTeamPhase := func(teamID string, want Phase) {
		t.Helper()
		timeout := time.After(2 * time.Second)
		for {
			select {
			case e := <-phaseChanges:
				pce := e.(event.TeamPhaseChangedEvent)
				if pce.TeamID == teamID && pce.CurrentPhase == string(want) {
					return
				}
			case <-timeout:
				s, _ := m.TeamStatus(teamID)
				t.Fatalf("timed out: team %q phase = %v, want %v", teamID, s.Phase, want)
			}
		}
	}

	waitForTeamPhase("alpha", PhaseWorking)
	waitForTeamPhase("beta", PhaseBlocked)
	waitForTeamPhase("gamma", PhaseBlocked)

	// Simulate alpha completing by marking its task as complete through
	// the EventQueue (not raw TaskQueue), which publishes depth_changed events.
	alphaTeam := m.Team("alpha")
	eq := alphaTeam.Hub().EventQueue()
	task, err := eq.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task == nil {
		t.Fatal("no task to claim")
	}
	if err := eq.MarkRunning(task.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if _, err := eq.Complete(task.ID); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Beta should now unblock and start working.
	waitForTeamPhase("beta", PhaseWorking)

	// Complete beta's task via EventQueue.
	betaTeam := m.Team("beta")
	beq := betaTeam.Hub().EventQueue()
	btask, _ := beq.ClaimNext("inst-2")
	if btask != nil {
		_ = beq.MarkRunning(btask.ID)
		_, _ = beq.Complete(btask.ID)
	}

	// Gamma should now unblock and start working.
	waitForTeamPhase("gamma", PhaseWorking)
}

func TestManager_FailedDependencyCascade(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)

	// Chain: alpha → beta → gamma. When alpha fails, both beta and gamma
	// should cascade to PhaseFailed.
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.AddTeam(testSpec("beta", "Beta", "alpha"))
	_ = m.AddTeam(testSpec("gamma", "Gamma", "beta"))

	phaseChanges := make(chan event.Event, 30)
	bus.Subscribe("team.phase_changed", func(e event.Event) {
		phaseChanges <- e
	})

	teamCompleted := make(chan event.Event, 10)
	bus.Subscribe("team.completed", func(e event.Event) {
		teamCompleted <- e
	})

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = m.Stop() }()

	waitForTeamPhase := func(teamID string, want Phase) {
		t.Helper()
		timeout := time.After(2 * time.Second)
		for {
			select {
			case e := <-phaseChanges:
				pce := e.(event.TeamPhaseChangedEvent)
				if pce.TeamID == teamID && pce.CurrentPhase == string(want) {
					return
				}
			case <-timeout:
				s, _ := m.TeamStatus(teamID)
				t.Fatalf("timed out: team %q phase = %v, want %v", teamID, s.Phase, want)
			}
		}
	}

	waitForTeamPhase("alpha", PhaseWorking)
	waitForTeamPhase("beta", PhaseBlocked)
	waitForTeamPhase("gamma", PhaseBlocked)

	// Fail alpha's task with retries disabled so it becomes terminal immediately.
	alphaTeam := m.Team("alpha")
	eq := alphaTeam.Hub().EventQueue()
	tq := alphaTeam.Hub().TaskQueue()
	task, err := eq.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task == nil {
		t.Fatal("no task to claim")
	}
	if err := tq.SetMaxRetries(task.ID, 0); err != nil {
		t.Fatalf("SetMaxRetries: %v", err)
	}
	if err := eq.MarkRunning(task.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if err := eq.Fail(task.ID, "intentional failure"); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	// Wait for all three completion events (alpha fails via monitor, beta and
	// gamma cascade via onTeamCompleted).
	completedTeams := make(map[string]bool)
	timeout := time.After(5 * time.Second)
	for len(completedTeams) < 3 {
		select {
		case e := <-teamCompleted:
			tce := e.(event.TeamCompletedEvent)
			completedTeams[tce.TeamID] = true
			if tce.Success {
				t.Errorf("team %q should have failed, got success=true", tce.TeamID)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for completion events: got %v", completedTeams)
		}
	}

	// Verify all three teams are in PhaseFailed.
	for _, id := range []string{"alpha", "beta", "gamma"} {
		status, ok := m.TeamStatus(id)
		if !ok {
			t.Fatalf("TeamStatus(%q) not found", id)
		}
		if status.Phase != PhaseFailed {
			t.Errorf("team %q phase = %v, want %v", id, status.Phase, PhaseFailed)
		}
	}
}

func TestManager_AddTeamDynamic_NoDeps(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	ctx := context.Background()
	_ = m.Start(ctx)
	defer func() { _ = m.Stop() }()

	dynamicAdded := make(chan event.Event, 5)
	bus.Subscribe("team.dynamic_added", func(e event.Event) {
		dynamicAdded <- e
	})

	// Dynamically add a team with no dependencies — should start immediately.
	err := m.AddTeamDynamic(ctx, testSpec("beta", "Beta"))
	if err != nil {
		t.Fatalf("AddTeamDynamic: %v", err)
	}

	select {
	case e := <-dynamicAdded:
		dae := e.(event.TeamDynamicAddedEvent)
		if dae.TeamID != "beta" {
			t.Errorf("TeamID = %q, want %q", dae.TeamID, "beta")
		}
		if dae.Phase != string(PhaseWorking) {
			t.Errorf("Phase = %q, want %q", dae.Phase, PhaseWorking)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dynamic added event")
	}

	// Team should be working.
	status, ok := m.TeamStatus("beta")
	if !ok {
		t.Fatal("TeamStatus(beta) not found")
	}
	if status.Phase != PhaseWorking {
		t.Errorf("phase = %v, want %v", status.Phase, PhaseWorking)
	}
}

func TestManager_AddTeamDynamic_WithDeps(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	ctx := context.Background()
	_ = m.Start(ctx)
	defer func() { _ = m.Stop() }()

	dynamicAdded := make(chan event.Event, 5)
	bus.Subscribe("team.dynamic_added", func(e event.Event) {
		dynamicAdded <- e
	})

	// Add a team that depends on alpha (not yet complete) — should be blocked.
	err := m.AddTeamDynamic(ctx, testSpec("beta", "Beta", "alpha"))
	if err != nil {
		t.Fatalf("AddTeamDynamic: %v", err)
	}

	select {
	case e := <-dynamicAdded:
		dae := e.(event.TeamDynamicAddedEvent)
		if dae.Phase != string(PhaseBlocked) {
			t.Errorf("Phase = %q, want %q", dae.Phase, PhaseBlocked)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dynamic added event")
	}

	status, ok := m.TeamStatus("beta")
	if !ok {
		t.Fatal("TeamStatus(beta) not found")
	}
	if status.Phase != PhaseBlocked {
		t.Errorf("phase = %v, want %v", status.Phase, PhaseBlocked)
	}
}

func TestManager_AddTeamDynamic_UnknownDep(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.Start(context.Background())
	defer func() { _ = m.Stop() }()

	err := m.AddTeamDynamic(context.Background(), testSpec("beta", "Beta", "nonexistent"))
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
	if !strings.Contains(err.Error(), "unknown team") {
		t.Errorf("error = %q, want containing 'unknown team'", err.Error())
	}
}

func TestManager_AddTeamDynamic_DuplicateID(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.Start(context.Background())
	defer func() { _ = m.Stop() }()

	err := m.AddTeamDynamic(context.Background(), testSpec("alpha", "Alpha Copy"))
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error = %q, want containing 'duplicate'", err.Error())
	}
}

func TestManager_AddTeamDynamic_NotStarted(t *testing.T) {
	m, _ := newTestManager(t)

	err := m.AddTeamDynamic(context.Background(), testSpec("alpha", "Alpha"))
	if err == nil {
		t.Fatal("expected error for not started")
	}
	if !strings.Contains(err.Error(), "requires a started manager") {
		t.Errorf("error = %q, want containing 'requires a started manager'", err.Error())
	}
}

func TestManager_AddTeamDynamic_InvalidSpec(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	_ = m.Start(context.Background())
	defer func() { _ = m.Stop() }()

	err := m.AddTeamDynamic(context.Background(), Spec{}) // empty spec
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestManager_Stop_NoDeadlockWithCompletionRace(t *testing.T) {
	m, _ := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Fail the task with retries disabled so it becomes terminal immediately.
	// This triggers monitorTeamCompletion to publish TeamCompletedEvent.
	alphaTeam := m.Team("alpha")
	eq := alphaTeam.Hub().EventQueue()
	tq := alphaTeam.Hub().TaskQueue()
	task, err := eq.ClaimNext("inst-1")
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if task == nil {
		t.Fatal("no task to claim")
	}
	if err := tq.SetMaxRetries(task.ID, 0); err != nil {
		t.Fatalf("SetMaxRetries: %v", err)
	}
	if err := eq.MarkRunning(task.ID); err != nil {
		t.Fatalf("MarkRunning: %v", err)
	}
	if err := eq.Fail(task.ID, "trigger completion"); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	// Call Stop immediately without waiting for the team.completed event.
	// Before the fix, this could deadlock: Stop held m.mu through wg.Wait()
	// while monitorTeamCompletion tried to acquire m.mu via onTeamCompleted.
	done := make(chan error, 1)
	go func() {
		done <- m.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop deadlocked — did not return within 5 seconds")
	}

	if m.Running() {
		t.Error("Running() = true after Stop")
	}
}

func TestManager_AddTeamDynamic_DepsSatisfied(t *testing.T) {
	m, bus := newTestManager(t,
		WithHubOptions(coordination.WithRebalanceInterval(-1)),
	)
	_ = m.AddTeam(testSpec("alpha", "Alpha"))
	ctx := context.Background()
	_ = m.Start(ctx)
	defer func() { _ = m.Stop() }()

	// Complete alpha so it's in a terminal phase.
	alphaTeam := m.Team("alpha")
	eq := alphaTeam.Hub().EventQueue()
	task, _ := eq.ClaimNext("inst-1")
	if task != nil {
		_ = eq.MarkRunning(task.ID)
		_, _ = eq.Complete(task.ID)
	}

	// Wait for alpha to be done.
	deadline := time.After(2 * time.Second)
	for {
		s, _ := m.TeamStatus("alpha")
		if s.Phase.IsTerminal() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for alpha to complete")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	dynamicAdded := make(chan event.Event, 5)
	bus.Subscribe("team.dynamic_added", func(e event.Event) {
		dynamicAdded <- e
	})

	// Add a team with a satisfied dependency — should start immediately.
	err := m.AddTeamDynamic(ctx, testSpec("beta", "Beta", "alpha"))
	if err != nil {
		t.Fatalf("AddTeamDynamic: %v", err)
	}

	select {
	case e := <-dynamicAdded:
		dae := e.(event.TeamDynamicAddedEvent)
		if dae.Phase != string(PhaseWorking) {
			t.Errorf("Phase = %q, want %q", dae.Phase, PhaseWorking)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dynamic added event")
	}
}

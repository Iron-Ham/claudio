package team

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func makeTestTeamForRouter(t *testing.T, id string, bus *event.Bus) *Team {
	t.Helper()
	tasks := []ultraplan.PlannedTask{{ID: "t1", Title: "Task 1"}}
	plan := &ultraplan.PlanSpec{ID: "plan-" + id, Objective: "test", Tasks: tasks}
	hub, err := coordination.NewHub(coordination.Config{
		Bus:        bus,
		SessionDir: t.TempDir(),
		Plan:       plan,
	}, coordination.WithRebalanceInterval(-1))
	if err != nil {
		t.Fatalf("NewHub: %v", err)
	}
	bt := newBudgetTracker(id, TokenBudget{}, bus)
	spec := Spec{ID: id, Name: "Team " + id, Role: RoleExecution, Tasks: tasks, TeamSize: 1}
	return newTeam(spec, hub, bt)
}

func TestRouter_TargetedMessage(t *testing.T) {
	bus := event.NewBus()
	teams := map[string]*Team{
		"team-a": makeTestTeamForRouter(t, "team-a", bus),
		"team-b": makeTestTeamForRouter(t, "team-b", bus),
	}
	allIDs := []string{"team-a", "team-b"}

	r := newRouter(bus,
		func(id string) *Team { return teams[id] },
		func() []string { return allIDs },
	)

	ch := make(chan event.Event, 2)
	bus.Subscribe("team.message", func(e event.Event) {
		ch <- e
	})

	msg := InterTeamMessage{
		FromTeam: "team-a",
		ToTeam:   "team-b",
		Type:     MessageTypeDiscovery,
		Content:  "found something",
		Priority: PriorityInfo,
	}

	if err := r.Route(msg); err != nil {
		t.Fatalf("Route() error: %v", err)
	}

	// Verify event was published.
	select {
	case e := <-ch:
		ite, ok := e.(event.InterTeamMessageEvent)
		if !ok {
			t.Fatalf("expected InterTeamMessageEvent, got %T", e)
		}
		if ite.FromTeam != "team-a" {
			t.Errorf("FromTeam = %q, want %q", ite.FromTeam, "team-a")
		}
		if ite.ToTeam != "team-b" {
			t.Errorf("ToTeam = %q, want %q", ite.ToTeam, "team-b")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}

	// Verify message log.
	msgs := r.Messages()
	if len(msgs) != 1 {
		t.Fatalf("Messages() len = %d, want 1", len(msgs))
	}
	if msgs[0].ID == "" {
		t.Error("message ID should be auto-assigned")
	}
}

func TestRouter_BroadcastMessage(t *testing.T) {
	bus := event.NewBus()
	teams := map[string]*Team{
		"team-a": makeTestTeamForRouter(t, "team-a", bus),
		"team-b": makeTestTeamForRouter(t, "team-b", bus),
		"team-c": makeTestTeamForRouter(t, "team-c", bus),
	}
	allIDs := []string{"team-a", "team-b", "team-c"}

	r := newRouter(bus,
		func(id string) *Team { return teams[id] },
		func() []string { return allIDs },
	)

	ch := make(chan event.Event, 10)
	bus.Subscribe("team.message", func(e event.Event) {
		ch <- e
	})

	msg := InterTeamMessage{
		FromTeam: "team-a",
		ToTeam:   BroadcastRecipient,
		Type:     MessageTypeWarning,
		Content:  "heads up",
		Priority: PriorityImportant,
	}

	if err := r.Route(msg); err != nil {
		t.Fatalf("Route() error: %v", err)
	}

	// Should get 2 events (team-b and team-c, not team-a).
	received := 0
	timeout := time.After(time.Second)
	for received < 2 {
		select {
		case e := <-ch:
			ite := e.(event.InterTeamMessageEvent)
			if ite.ToTeam == "team-a" {
				t.Error("broadcast should not deliver to sender")
			}
			received++
		case <-timeout:
			t.Fatalf("timed out: got %d events, want 2", received)
		}
	}

	// Verify no extra events.
	select {
	case <-ch:
		t.Error("should not have extra events")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestRouter_TargetNotFound(t *testing.T) {
	bus := event.NewBus()
	r := newRouter(bus,
		func(id string) *Team { return nil },
		func() []string { return nil },
	)

	err := r.Route(InterTeamMessage{
		FromTeam: "team-a",
		ToTeam:   "nonexistent",
		Type:     MessageTypeRequest,
		Content:  "hello",
		Priority: PriorityInfo,
	})

	if err == nil {
		t.Fatal("Route() should error for nonexistent target")
	}
}

func TestRouter_ValidationErrors(t *testing.T) {
	bus := event.NewBus()
	r := newRouter(bus,
		func(id string) *Team { return nil },
		func() []string { return nil },
	)

	tests := []struct {
		name string
		msg  InterTeamMessage
	}{
		{"missing from", InterTeamMessage{ToTeam: "team-b"}},
		{"missing to", InterTeamMessage{FromTeam: "team-a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := r.Route(tt.msg); err == nil {
				t.Error("Route() should error for invalid message")
			}
		})
	}
}

func TestRouter_MessagesForTeam(t *testing.T) {
	bus := event.NewBus()
	teams := map[string]*Team{
		"team-a": makeTestTeamForRouter(t, "team-a", bus),
		"team-b": makeTestTeamForRouter(t, "team-b", bus),
	}
	allIDs := []string{"team-a", "team-b"}

	r := newRouter(bus,
		func(id string) *Team { return teams[id] },
		func() []string { return allIDs },
	)

	// Route two messages.
	_ = r.Route(InterTeamMessage{
		FromTeam: "team-a", ToTeam: "team-b",
		Type: MessageTypeDiscovery, Content: "msg1", Priority: PriorityInfo,
	})
	_ = r.Route(InterTeamMessage{
		FromTeam: "team-b", ToTeam: "team-a",
		Type: MessageTypeRequest, Content: "msg2", Priority: PriorityUrgent,
	})

	msgsA := r.MessagesForTeam("team-a")
	if len(msgsA) != 2 {
		t.Errorf("MessagesForTeam(team-a) len = %d, want 2", len(msgsA))
	}

	msgsB := r.MessagesForTeam("team-b")
	if len(msgsB) != 2 {
		t.Errorf("MessagesForTeam(team-b) len = %d, want 2", len(msgsB))
	}
}

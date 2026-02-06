package team

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
)

func TestBudgetTracker_Record(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{
		MaxInputTokens:  1000,
		MaxOutputTokens: 500,
		MaxTotalCost:    10.0,
	}, bus)

	bt.Record(100, 50, 1.0)
	usage := bt.Usage()

	if usage.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", usage.InputTokens)
	}
	if usage.OutputTokens != 50 {
		t.Errorf("OutputTokens = %d, want 50", usage.OutputTokens)
	}
	if usage.TotalCost != 1.0 {
		t.Errorf("TotalCost = %f, want 1.0", usage.TotalCost)
	}
}

func TestBudgetTracker_Exhausted_InputTokens(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{MaxInputTokens: 100}, bus)

	if bt.Exhausted() {
		t.Error("should not be exhausted before any recording")
	}

	bt.Record(99, 0, 0)
	if bt.Exhausted() {
		t.Error("should not be exhausted at 99/100")
	}

	bt.Record(1, 0, 0)
	if !bt.Exhausted() {
		t.Error("should be exhausted at 100/100")
	}
}

func TestBudgetTracker_Exhausted_OutputTokens(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{MaxOutputTokens: 200}, bus)

	bt.Record(0, 200, 0)
	if !bt.Exhausted() {
		t.Error("should be exhausted at output limit")
	}
}

func TestBudgetTracker_Exhausted_Cost(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{MaxTotalCost: 5.0}, bus)

	bt.Record(0, 0, 4.99)
	if bt.Exhausted() {
		t.Error("should not be exhausted below cost limit")
	}

	bt.Record(0, 0, 0.01)
	if !bt.Exhausted() {
		t.Error("should be exhausted at cost limit")
	}
}

func TestBudgetTracker_Exhausted_Unlimited(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{}, bus) // all zeros = unlimited

	bt.Record(999999, 999999, 999999)
	if bt.Exhausted() {
		t.Error("unlimited budget should never be exhausted")
	}
}

func TestBudgetTracker_ExhaustedEvent(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{MaxInputTokens: 100}, bus)

	ch := make(chan event.Event, 1)
	bus.Subscribe("team.budget_exhausted", func(e event.Event) {
		ch <- e
	})

	// Should not fire event below limit.
	bt.Record(50, 0, 0)
	select {
	case <-ch:
		t.Fatal("should not fire event below limit")
	case <-time.After(10 * time.Millisecond):
	}

	// Should fire when crossing the limit.
	bt.Record(50, 0, 0)
	select {
	case e := <-ch:
		be, ok := e.(event.TeamBudgetExhaustedEvent)
		if !ok {
			t.Fatalf("expected TeamBudgetExhaustedEvent, got %T", e)
		}
		if be.TeamID != "team-1" {
			t.Errorf("TeamID = %q, want %q", be.TeamID, "team-1")
		}
		if be.UsedInput != 100 {
			t.Errorf("UsedInput = %d, want 100", be.UsedInput)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for budget exhausted event")
	}

	// Should not fire again once already exhausted.
	bt.Record(10, 0, 0)
	select {
	case <-ch:
		t.Fatal("should not fire event twice")
	case <-time.After(10 * time.Millisecond):
	}
}

func TestBudgetTracker_StartStop(t *testing.T) {
	bus := event.NewBus()
	bt := newBudgetTracker("team-1", TokenBudget{MaxInputTokens: 100}, bus)

	// Start marks the tracker as active (no bus subscription needed — the
	// manager routes events via Record externally).
	bt.Start()

	// Idempotent start should not panic.
	bt.Start()

	// Record should work while started.
	bt.Record(10, 0, 0)
	if bt.Usage().InputTokens != 10 {
		t.Errorf("InputTokens = %d, want 10", bt.Usage().InputTokens)
	}

	// Stop and idempotent stop.
	bt.Stop()
	bt.Stop()
}

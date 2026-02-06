package scaling

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
)

func TestMonitor_StartsAndStops(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(WithCooldownPeriod(0))
	m := NewMonitor(bus, policy, 2)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	// Give the monitor time to subscribe
	time.Sleep(10 * time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("monitor did not stop after context cancel")
	}
}

func TestMonitor_StopMethod(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(WithCooldownPeriod(0))
	m := NewMonitor(bus, policy, 2)

	done := make(chan struct{})
	go func() {
		m.Start(context.Background())
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	m.Stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("monitor did not stop after Stop()")
	}
}

func TestMonitor_ScaleUpDecision(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(
		WithCooldownPeriod(0),
		WithMaxInstances(10),
	)
	m := NewMonitor(bus, policy, 2)

	var mu sync.Mutex
	var decisions []Decision
	m.OnDecision(func(d Decision) {
		mu.Lock()
		defer mu.Unlock()
		decisions = append(decisions, d)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	// Give monitor time to subscribe
	time.Sleep(10 * time.Millisecond)

	// Publish a depth event with more pending than running
	bus.Publish(event.NewQueueDepthChangedEvent(5, 0, 1, 0, 0, 10))

	// Wait for the decision
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Action != ActionScaleUp {
		t.Errorf("Action = %q, want scale_up", decisions[0].Action)
	}
	if decisions[0].Delta <= 0 {
		t.Errorf("Delta = %d, want positive", decisions[0].Delta)
	}
}

func TestMonitor_ScaleDownDecision(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(
		WithCooldownPeriod(0),
		WithMinInstances(1),
		WithScaleDownThreshold(1),
	)
	m := NewMonitor(bus, policy, 4)

	var mu sync.Mutex
	var decisions []Decision
	m.OnDecision(func(d Decision) {
		mu.Lock()
		defer mu.Unlock()
		decisions = append(decisions, d)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	// Publish a depth event with no pending and low running
	bus.Publish(event.NewQueueDepthChangedEvent(0, 0, 0, 8, 2, 10))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Action != ActionScaleDown {
		t.Errorf("Action = %q, want scale_down", decisions[0].Action)
	}
	if decisions[0].Delta >= 0 {
		t.Errorf("Delta = %d, want negative", decisions[0].Delta)
	}
}

func TestMonitor_NoDecisionForBalancedLoad(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(WithCooldownPeriod(0))
	m := NewMonitor(bus, policy, 3)

	var mu sync.Mutex
	var decisions []Decision
	m.OnDecision(func(d Decision) {
		mu.Lock()
		defer mu.Unlock()
		decisions = append(decisions, d)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	// Balanced: running matches pending, so no scale up; running > threshold, so no scale down
	bus.Publish(event.NewQueueDepthChangedEvent(2, 0, 3, 5, 0, 10))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions for balanced load, got %d", len(decisions))
	}
}

func TestMonitor_PublishesScalingDecisionEvent(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(
		WithCooldownPeriod(0),
		WithMaxInstances(10),
	)
	m := NewMonitor(bus, policy, 2)

	var mu sync.Mutex
	var scalingEvents []event.ScalingDecisionEvent
	bus.Subscribe("scaling.decision", func(e event.Event) {
		mu.Lock()
		defer mu.Unlock()
		scalingEvents = append(scalingEvents, e.(event.ScalingDecisionEvent))
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	bus.Publish(event.NewQueueDepthChangedEvent(5, 0, 1, 0, 0, 10))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(scalingEvents) != 1 {
		t.Fatalf("expected 1 ScalingDecisionEvent, got %d", len(scalingEvents))
	}
	se := scalingEvents[0]
	if se.Action != "scale_up" {
		t.Errorf("Action = %q, want scale_up", se.Action)
	}
	if se.CurrentInstances != 2 {
		t.Errorf("CurrentInstances = %d, want 2", se.CurrentInstances)
	}
}

func TestMonitor_SetCurrentInstances(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(
		WithCooldownPeriod(0),
		WithMaxInstances(5),
	)
	m := NewMonitor(bus, policy, 2)

	var mu sync.Mutex
	var decisions []Decision
	m.OnDecision(func(d Decision) {
		mu.Lock()
		defer mu.Unlock()
		decisions = append(decisions, d)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	// Set instances to max — should prevent scale up
	m.SetCurrentInstances(5)

	bus.Publish(event.NewQueueDepthChangedEvent(5, 0, 1, 0, 0, 10))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions at max instances, got %d", len(decisions))
	}
}

func TestMonitor_MultipleHandlers(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(
		WithCooldownPeriod(0),
		WithMaxInstances(10),
	)
	m := NewMonitor(bus, policy, 2)

	var mu sync.Mutex
	count1 := 0
	count2 := 0
	m.OnDecision(func(d Decision) {
		mu.Lock()
		defer mu.Unlock()
		count1++
	})
	m.OnDecision(func(d Decision) {
		mu.Lock()
		defer mu.Unlock()
		count2++
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	bus.Publish(event.NewQueueDepthChangedEvent(5, 0, 1, 0, 0, 10))

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count1 != 1 || count2 != 1 {
		t.Errorf("handler counts = (%d, %d), want (1, 1)", count1, count2)
	}
}

func TestMonitor_ConcurrentOperations(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy(WithCooldownPeriod(0))
	m := NewMonitor(bus, policy, 3)

	m.OnDecision(func(d Decision) {
		// no-op handler for thread safety test
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		m.Start(ctx)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup

	// Concurrent event publishing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			bus.Publish(event.NewQueueDepthChangedEvent(n, 0, 1, 0, 0, 10))
		}(i)
	}

	// Concurrent instance count updates
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			m.SetCurrentInstances(n + 1)
		}(i)
	}

	wg.Wait()
}

func TestMonitor_Stop_BeforeStart(t *testing.T) {
	bus := event.NewBus()
	policy := NewPolicy()
	m := NewMonitor(bus, policy, 1)

	// Stop before Start should not panic
	m.Stop()
}

// Compile-time interface check.
var _ event.Event = event.ScalingDecisionEvent{}

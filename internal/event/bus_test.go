package event

import (
	"sync"
	"testing"
)

func TestBus_Subscribe(t *testing.T) {
	bus := NewBus()

	called := false
	id := bus.Subscribe("test.event", func(e Event) {
		called = true
	})

	if id == "" {
		t.Error("Subscribe should return a non-empty ID")
	}

	if bus.SubscriptionCount() != 1 {
		t.Errorf("Expected 1 subscription, got %d", bus.SubscriptionCount())
	}

	if called {
		t.Error("Handler should not be called until an event is published")
	}
}

func TestBus_Publish(t *testing.T) {
	bus := NewBus()

	var receivedEvent Event
	bus.Subscribe("instance.started", func(e Event) {
		receivedEvent = e
	})

	event := NewInstanceStartedEvent("inst-1", "/path/to/worktree", "feature-branch", "test task")
	bus.Publish(event)

	if receivedEvent == nil {
		t.Fatal("Handler should have received the event")
	}

	if receivedEvent.EventType() != "instance.started" {
		t.Errorf("Expected event type 'instance.started', got '%s'", receivedEvent.EventType())
	}
}

func TestBus_PublishMultipleHandlers(t *testing.T) {
	bus := NewBus()

	callCount := 0
	bus.Subscribe("test.event", func(e Event) {
		callCount++
	})
	bus.Subscribe("test.event", func(e Event) {
		callCount++
	})

	bus.Publish(newBaseEvent("test.event"))

	if callCount != 2 {
		t.Errorf("Expected both handlers to be called, got %d calls", callCount)
	}
}

func TestBus_PublishNoMatchingHandlers(t *testing.T) {
	bus := NewBus()

	bus.Subscribe("other.event", func(e Event) {
		t.Error("Handler should not be called for non-matching event type")
	})

	// This should not panic or call the handler
	bus.Publish(newBaseEvent("test.event"))
}

func TestBus_SubscribeAll(t *testing.T) {
	bus := NewBus()

	var events []string
	bus.SubscribeAll(func(e Event) {
		events = append(events, e.EventType())
	})

	bus.Publish(newBaseEvent("event.one"))
	bus.Publish(newBaseEvent("event.two"))
	bus.Publish(newBaseEvent("event.three"))

	if len(events) != 3 {
		t.Errorf("Expected 3 events, got %d", len(events))
	}

	expected := []string{"event.one", "event.two", "event.three"}
	for i, e := range expected {
		if events[i] != e {
			t.Errorf("Expected event %d to be '%s', got '%s'", i, e, events[i])
		}
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()

	called := false
	id := bus.Subscribe("test.event", func(e Event) {
		called = true
	})

	// Unsubscribe before publishing
	removed := bus.Unsubscribe(id)
	if !removed {
		t.Error("Unsubscribe should return true when subscription exists")
	}

	if bus.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after unsubscribe, got %d", bus.SubscriptionCount())
	}

	bus.Publish(newBaseEvent("test.event"))

	if called {
		t.Error("Handler should not be called after unsubscribing")
	}
}

func TestBus_UnsubscribeNonExistent(t *testing.T) {
	bus := NewBus()

	removed := bus.Unsubscribe("non-existent-id")
	if removed {
		t.Error("Unsubscribe should return false for non-existent ID")
	}
}

func TestBus_UnsubscribeOne(t *testing.T) {
	bus := NewBus()

	calls := make(map[string]int)
	id1 := bus.Subscribe("test.event", func(e Event) {
		calls["handler1"]++
	})
	bus.Subscribe("test.event", func(e Event) {
		calls["handler2"]++
	})

	// Unsubscribe only the first handler
	bus.Unsubscribe(id1)

	bus.Publish(newBaseEvent("test.event"))

	if calls["handler1"] != 0 {
		t.Error("handler1 should not be called after unsubscribing")
	}
	if calls["handler2"] != 1 {
		t.Error("handler2 should still be called")
	}
}

func TestBus_Clear(t *testing.T) {
	bus := NewBus()

	bus.Subscribe("event.one", func(e Event) {})
	bus.Subscribe("event.two", func(e Event) {})
	bus.SubscribeAll(func(e Event) {})

	if bus.SubscriptionCount() != 3 {
		t.Errorf("Expected 3 subscriptions before clear, got %d", bus.SubscriptionCount())
	}

	bus.Clear()

	if bus.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after clear, got %d", bus.SubscriptionCount())
	}
}

func TestBus_HandlerPanicRecovery(t *testing.T) {
	bus := NewBus()

	calls := 0
	bus.Subscribe("test.event", func(e Event) {
		calls++
		panic("handler panic")
	})
	bus.Subscribe("test.event", func(e Event) {
		calls++
	})

	// Should not panic
	bus.Publish(newBaseEvent("test.event"))

	if calls != 2 {
		t.Errorf("Expected both handlers to be called despite panic, got %d calls", calls)
	}
}

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()

	var mu sync.Mutex
	calls := 0
	bus.Subscribe("test.event", func(e Event) {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			bus.Publish(newBaseEvent("test.event"))
		})
	}
	wg.Wait()

	if calls != 100 {
		t.Errorf("Expected 100 calls, got %d", calls)
	}
}

func TestBus_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	bus := NewBus()

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			id := bus.Subscribe("test.event", func(e Event) {})
			bus.Unsubscribe(id)
		})
	}
	wg.Wait()

	// All subscriptions should be removed
	if bus.SubscriptionCount() != 0 {
		t.Errorf("Expected 0 subscriptions after concurrent add/remove, got %d", bus.SubscriptionCount())
	}
}

func TestBus_MixedSubscriptions(t *testing.T) {
	bus := NewBus()

	var events []string
	bus.Subscribe("specific.event", func(e Event) {
		events = append(events, "specific:"+e.EventType())
	})
	bus.SubscribeAll(func(e Event) {
		events = append(events, "wildcard:"+e.EventType())
	})

	bus.Publish(newBaseEvent("specific.event"))

	if len(events) != 2 {
		t.Errorf("Expected 2 handler calls, got %d", len(events))
	}

	// Both handlers should be called
	hasSpecific := false
	hasWildcard := false
	for _, e := range events {
		if e == "specific:specific.event" {
			hasSpecific = true
		}
		if e == "wildcard:specific.event" {
			hasWildcard = true
		}
	}

	if !hasSpecific {
		t.Error("Specific handler should have been called")
	}
	if !hasWildcard {
		t.Error("Wildcard handler should have been called")
	}
}

func TestBus_UniqueIDs(t *testing.T) {
	bus := NewBus()

	ids := make(map[string]bool)
	for range 100 {
		id := bus.Subscribe("test.event", func(e Event) {})
		if ids[id] {
			t.Errorf("Duplicate subscription ID: %s", id)
		}
		ids[id] = true
	}
}

package event

import (
	"log"
	"runtime/debug"
	"sync"
	"sync/atomic"
)

// Handler is a function that handles an event.
type Handler func(Event)

// subscription represents a registered event handler.
type subscription struct {
	id        string
	eventType string
	handler   Handler
}

// Bus is a simple synchronous pub-sub event bus.
// It allows components to communicate without direct dependencies.
type Bus struct {
	mu            sync.RWMutex
	subscriptions map[string][]subscription // eventType -> subscriptions
	nextID        atomic.Uint64
}

// NewBus creates a new event bus.
func NewBus() *Bus {
	return &Bus{
		subscriptions: make(map[string][]subscription),
	}
}

// Subscribe registers a handler for a specific event type.
// Returns a subscription ID that can be used to unsubscribe.
func (b *Bus) Subscribe(eventType string, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := b.generateID()
	sub := subscription{
		id:        id,
		eventType: eventType,
		handler:   handler,
	}

	b.subscriptions[eventType] = append(b.subscriptions[eventType], sub)
	return id
}

// SubscribeAll registers a handler for all event types.
// The handler will be called for every published event.
// Returns a subscription ID that can be used to unsubscribe.
func (b *Bus) SubscribeAll(handler Handler) string {
	return b.Subscribe("*", handler)
}

// Unsubscribe removes a subscription by ID.
// Returns true if the subscription was found and removed.
func (b *Bus) Unsubscribe(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	for eventType, subs := range b.subscriptions {
		for i, sub := range subs {
			if sub.id == id {
				// Remove subscription by re-slicing to exclude index i
				b.subscriptions[eventType] = append(subs[:i], subs[i+1:]...)
				return true
			}
		}
	}
	return false
}

// Publish dispatches an event to all registered handlers.
// Specific handlers (subscribed to this event type) are called first,
// followed by wildcard handlers (subscribed via SubscribeAll).
// Within each group, handlers are called in registration order.
// If a handler panics, the panic is logged, recovered, and publishing
// continues to remaining handlers.
func (b *Bus) Publish(event Event) {
	b.mu.RLock()
	eventType := event.EventType()

	// Get specific handlers for this event type
	specificSubs := make([]subscription, len(b.subscriptions[eventType]))
	copy(specificSubs, b.subscriptions[eventType])

	// Get wildcard handlers that listen to all events
	wildcardSubs := make([]subscription, len(b.subscriptions["*"]))
	copy(wildcardSubs, b.subscriptions["*"])

	b.mu.RUnlock()

	// Dispatch to specific handlers
	for _, sub := range specificSubs {
		b.safeCall(sub.handler, event)
	}

	// Dispatch to wildcard handlers
	for _, sub := range wildcardSubs {
		b.safeCall(sub.handler, event)
	}
}

// safeCall invokes a handler and recovers from any panics.
// Panics are logged with stack traces to aid debugging while ensuring
// one misbehaving handler cannot block event delivery to other handlers.
func (b *Bus) safeCall(handler Handler, event Event) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ERROR: event handler panicked for event %s: %v\n%s",
				event.EventType(), r, debug.Stack())
		}
	}()
	handler(event)
}

// generateID creates a unique subscription ID.
func (b *Bus) generateID() string {
	id := b.nextID.Add(1)
	return string(rune('a'+id%26)) + string(rune('0'+id/26%10)) + string(rune('a'+id/260%26))
}

// Clear removes all subscriptions.
func (b *Bus) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscriptions = make(map[string][]subscription)
}

// SubscriptionCount returns the total number of active subscriptions.
func (b *Bus) SubscriptionCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	for _, subs := range b.subscriptions {
		count += len(subs)
	}
	return count
}

package contextprop

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

func newTestPropagator(t *testing.T) (*Propagator, *mailbox.Mailbox, *event.Bus) {
	t.Helper()
	mb := mailbox.NewMailbox(t.TempDir())
	bus := event.NewBus()
	prop := NewPropagator(mb, bus)
	return prop, mb, bus
}

func TestNewPropagator(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	bus := event.NewBus()
	prop := NewPropagator(mb, bus)

	if prop.mb != mb {
		t.Error("expected Propagator to wrap the provided mailbox")
	}
	if prop.bus != bus {
		t.Error("expected Propagator to wrap the provided event bus")
	}
}

func TestShareDiscovery(t *testing.T) {
	prop, mb, bus := newTestPropagator(t)

	var received event.Event
	bus.Subscribe("context.propagated", func(e event.Event) {
		received = e
	})

	metadata := map[string]any{"file": "main.go"}
	err := prop.ShareDiscovery("inst-1", "Found shared utility", metadata)
	if err != nil {
		t.Fatalf("ShareDiscovery() error = %v", err)
	}

	// Verify message was sent to broadcast.
	messages, err := mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.From != "inst-1" {
		t.Errorf("msg.From = %q, want %q", msg.From, "inst-1")
	}
	if msg.To != mailbox.BroadcastRecipient {
		t.Errorf("msg.To = %q, want %q", msg.To, mailbox.BroadcastRecipient)
	}
	if msg.Type != mailbox.MessageDiscovery {
		t.Errorf("msg.Type = %q, want %q", msg.Type, mailbox.MessageDiscovery)
	}
	if msg.Body != "Found shared utility" {
		t.Errorf("msg.Body = %q, want %q", msg.Body, "Found shared utility")
	}
	if msg.Metadata["file"] != "main.go" {
		t.Errorf("msg.Metadata[file] = %v, want %q", msg.Metadata["file"], "main.go")
	}

	// Verify event was published.
	if received == nil {
		t.Fatal("expected ContextPropagatedEvent")
	}
	ev, ok := received.(event.ContextPropagatedEvent)
	if !ok {
		t.Fatalf("expected ContextPropagatedEvent, got %T", received)
	}
	if ev.From != "inst-1" {
		t.Errorf("event From = %q, want %q", ev.From, "inst-1")
	}
	if ev.MessageType != "discovery" {
		t.Errorf("event MessageType = %q, want %q", ev.MessageType, "discovery")
	}
}

func TestShareDiscovery_NilMetadata(t *testing.T) {
	prop, _, _ := newTestPropagator(t)

	err := prop.ShareDiscovery("inst-1", "no metadata", nil)
	if err != nil {
		t.Fatalf("ShareDiscovery() error = %v", err)
	}
}

func TestShareDiscovery_NilBus(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	prop := NewPropagator(mb, nil)

	err := prop.ShareDiscovery("inst-1", "discovery", nil)
	if err != nil {
		t.Fatalf("ShareDiscovery() error = %v", err)
	}
}

func TestShareDiscovery_SendError(t *testing.T) {
	mb := mailbox.NewMailbox("/dev/null")
	prop := NewPropagator(mb, nil)

	err := prop.ShareDiscovery("inst-1", "discovery", nil)
	if err == nil {
		t.Fatal("expected error from mailbox send failure")
	}
	if !strings.Contains(err.Error(), "share discovery") {
		t.Errorf("error = %q, want to contain 'share discovery'", err.Error())
	}
}

func TestShareWarning(t *testing.T) {
	prop, mb, bus := newTestPropagator(t)

	var received event.Event
	bus.Subscribe("context.propagated", func(e event.Event) {
		received = e
	})

	err := prop.ShareWarning("inst-1", "API rate limit approaching")
	if err != nil {
		t.Fatalf("ShareWarning() error = %v", err)
	}

	messages, err := mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	msg := messages[0]
	if msg.Type != mailbox.MessageWarning {
		t.Errorf("msg.Type = %q, want %q", msg.Type, mailbox.MessageWarning)
	}
	if msg.Body != "API rate limit approaching" {
		t.Errorf("msg.Body = %q, want %q", msg.Body, "API rate limit approaching")
	}

	// Verify event.
	if received == nil {
		t.Fatal("expected ContextPropagatedEvent")
	}
	ev, ok := received.(event.ContextPropagatedEvent)
	if !ok {
		t.Fatalf("expected ContextPropagatedEvent, got %T", received)
	}
	if ev.MessageType != "warning" {
		t.Errorf("event MessageType = %q, want %q", ev.MessageType, "warning")
	}
}

func TestShareWarning_NilBus(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	prop := NewPropagator(mb, nil)

	err := prop.ShareWarning("inst-1", "warning")
	if err != nil {
		t.Fatalf("ShareWarning() error = %v", err)
	}
}

func TestShareWarning_SendError(t *testing.T) {
	mb := mailbox.NewMailbox("/dev/null")
	prop := NewPropagator(mb, nil)

	err := prop.ShareWarning("inst-1", "warning")
	if err == nil {
		t.Fatal("expected error from mailbox send failure")
	}
	if !strings.Contains(err.Error(), "share warning") {
		t.Errorf("error = %q, want to contain 'share warning'", err.Error())
	}
}

func TestGetContextForInstance(t *testing.T) {
	prop, mb, _ := newTestPropagator(t)

	// Send some messages directly via mailbox.
	_ = mb.Send(mailbox.Message{
		From: "inst-1",
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageDiscovery,
		Body: "shared types in pkg/models",
	})
	_ = mb.Send(mailbox.Message{
		From: "inst-2",
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageWarning,
		Body: "rate limit approaching",
	})

	ctx, err := prop.GetContextForInstance("inst-3", mailbox.FilterOptions{})
	if err != nil {
		t.Fatalf("GetContextForInstance() error = %v", err)
	}

	if !strings.Contains(ctx, "shared types in pkg/models") {
		t.Error("expected discovery message in context")
	}
	if !strings.Contains(ctx, "rate limit approaching") {
		t.Error("expected warning message in context")
	}
}

func TestGetContextForInstance_WithFilter(t *testing.T) {
	prop, mb, _ := newTestPropagator(t)

	_ = mb.Send(mailbox.Message{
		From: "inst-1",
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageDiscovery,
		Body: "discovery",
	})
	_ = mb.Send(mailbox.Message{
		From: "inst-2",
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageWarning,
		Body: "warning",
	})

	ctx, err := prop.GetContextForInstance("inst-3", mailbox.FilterOptions{
		Types: []mailbox.MessageType{mailbox.MessageDiscovery},
	})
	if err != nil {
		t.Fatalf("GetContextForInstance() error = %v", err)
	}

	if !strings.Contains(ctx, "discovery") {
		t.Error("expected discovery message in filtered context")
	}
	if strings.Contains(ctx, "warning") {
		t.Error("expected warning to be filtered out")
	}
}

func TestGetContextForInstance_NoMessages(t *testing.T) {
	prop, _, _ := newTestPropagator(t)

	ctx, err := prop.GetContextForInstance("inst-1", mailbox.FilterOptions{})
	if err != nil {
		t.Fatalf("GetContextForInstance() error = %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty context for no messages, got %q", ctx)
	}
}

func TestGetContextForInstance_ReceiveError(t *testing.T) {
	mb := mailbox.NewMailbox(t.TempDir())
	prop := NewPropagator(mb, nil)

	// Empty instanceID triggers an error from ReadForInstance.
	_, err := prop.GetContextForInstance("", mailbox.FilterOptions{})
	if err == nil {
		t.Fatal("expected error for empty instanceID")
	}
	if !strings.Contains(err.Error(), "receive messages") {
		t.Errorf("error = %q, want to contain 'receive messages'", err.Error())
	}
}

func TestWatch(t *testing.T) {
	prop, mb, _ := newTestPropagator(t)

	// Set a fast poll interval for the test.
	mb.SetPollInterval(10 * time.Millisecond)

	var mu sync.Mutex
	var received []mailbox.Message

	cancel := prop.Watch("inst-1", func(msg mailbox.Message) {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, msg)
	})
	defer cancel()

	// Send a targeted message.
	_ = mb.Send(mailbox.Message{
		From: "inst-2",
		To:   "inst-1",
		Type: mailbox.MessageDiscovery,
		Body: "watched message",
	})

	// Wait for the watcher to pick it up.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		count := len(received)
		mu.Unlock()
		if count > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for watched message")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if received[0].Body != "watched message" {
		t.Errorf("received[0].Body = %q, want %q", received[0].Body, "watched message")
	}
}

func TestWatch_Cancel(t *testing.T) {
	prop, mb, _ := newTestPropagator(t)
	mb.SetPollInterval(10 * time.Millisecond)

	cancel := prop.Watch("inst-1", func(_ mailbox.Message) {
		t.Error("handler should not be called after cancel")
	})

	// Cancel immediately.
	cancel()

	// Send a message after cancel.
	_ = mb.Send(mailbox.Message{
		From: "inst-2",
		To:   "inst-1",
		Type: mailbox.MessageDiscovery,
		Body: "should not be seen",
	})

	// Give it time to not fire.
	time.Sleep(50 * time.Millisecond)
}

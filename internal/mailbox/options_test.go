package mailbox_test

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

func TestMailbox_WithBus_PublishesOnSend(t *testing.T) {
	bus := event.NewBus()
	mb := mailbox.NewMailbox(t.TempDir(), mailbox.WithBus(bus))

	ch := make(chan event.Event, 1)
	subID := bus.Subscribe("mailbox.message", func(e event.Event) {
		ch <- e
	})
	defer bus.Unsubscribe(subID)

	msg := mailbox.Message{
		From: "sender",
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageDiscovery,
		Body: "found a thing",
	}
	if err := mb.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	select {
	case e := <-ch:
		mme, ok := e.(event.MailboxMessageEvent)
		if !ok {
			t.Fatalf("event type = %T, want MailboxMessageEvent", e)
		}
		if mme.From != "sender" {
			t.Errorf("From = %q, want %q", mme.From, "sender")
		}
		if mme.MessageType != "discovery" {
			t.Errorf("MessageType = %q, want %q", mme.MessageType, "discovery")
		}
	default:
		t.Fatal("expected MailboxMessageEvent, got none")
	}
}

func TestMailbox_NoBus_NoEvent(t *testing.T) {
	// Without WithBus, Send should succeed without publishing.
	mb := mailbox.NewMailbox(t.TempDir())

	msg := mailbox.Message{
		From: "sender",
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageDiscovery,
		Body: "no bus",
	}
	if err := mb.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestMailbox_WithBus_BackwardCompatible(t *testing.T) {
	// NewMailbox without options should work (nil bus).
	mb := mailbox.NewMailbox(t.TempDir())

	msg := mailbox.Message{
		From: "a",
		To:   "b",
		Type: mailbox.MessageStatus,
		Body: "ok",
	}
	if err := mb.Send(msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	messages, err := mb.Receive("b")
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Receive = %d messages, want 1", len(messages))
	}
}

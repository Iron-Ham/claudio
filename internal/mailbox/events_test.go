package mailbox

import (
	"testing"
	"time"
)

func TestNewMailboxMessageEvent(t *testing.T) {
	msg := Message{
		From:      "inst-1",
		To:        "inst-2",
		Type:      MessageDiscovery,
		Body:      "found a helper function",
		Timestamp: time.Now(),
	}

	evt := NewMailboxMessageEvent(msg)

	if evt.EventType() != "mailbox.message" {
		t.Errorf("EventType() = %q, want %q", evt.EventType(), "mailbox.message")
	}
	if evt.From != "inst-1" {
		t.Errorf("From = %q, want %q", evt.From, "inst-1")
	}
	if evt.To != "inst-2" {
		t.Errorf("To = %q, want %q", evt.To, "inst-2")
	}
	if evt.MessageType != "discovery" {
		t.Errorf("MessageType = %q, want %q", evt.MessageType, "discovery")
	}
	if evt.Body != "found a helper function" {
		t.Errorf("Body = %q, want %q", evt.Body, "found a helper function")
	}
	if evt.Timestamp().IsZero() {
		t.Error("Timestamp() should not be zero")
	}
}

func TestNewMailboxMessageEvent_Broadcast(t *testing.T) {
	msg := Message{
		From: "coordinator",
		To:   BroadcastRecipient,
		Type: MessageWarning,
		Body: "watch out",
	}

	evt := NewMailboxMessageEvent(msg)

	if evt.To != BroadcastRecipient {
		t.Errorf("To = %q, want %q", evt.To, BroadcastRecipient)
	}
	if evt.MessageType != "warning" {
		t.Errorf("MessageType = %q, want %q", evt.MessageType, "warning")
	}
}

func TestNewMailboxMessageEvent_AllTypes(t *testing.T) {
	types := []MessageType{
		MessageDiscovery, MessageClaim, MessageRelease,
		MessageWarning, MessageQuestion, MessageAnswer, MessageStatus,
	}
	for _, mt := range types {
		t.Run(string(mt), func(t *testing.T) {
			msg := Message{
				From: "inst-1",
				To:   "inst-2",
				Type: mt,
				Body: "test",
			}
			evt := NewMailboxMessageEvent(msg)
			if evt.MessageType != string(mt) {
				t.Errorf("MessageType = %q, want %q", evt.MessageType, string(mt))
			}
		})
	}
}

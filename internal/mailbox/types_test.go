package mailbox

import (
	"testing"
	"time"
)

func TestMessage_IsBroadcast(t *testing.T) {
	tests := []struct {
		name string
		to   string
		want bool
	}{
		{"broadcast recipient", BroadcastRecipient, true},
		{"specific instance", "instance-1", false},
		{"empty recipient", "", false},
		{"coordinator", "coordinator", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{To: tt.to}
			if got := msg.IsBroadcast(); got != tt.want {
				t.Errorf("Message.IsBroadcast() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateMessageType(t *testing.T) {
	tests := []struct {
		name     string
		msgType  MessageType
		expected bool
	}{
		{"discovery", MessageDiscovery, true},
		{"claim", MessageClaim, true},
		{"release", MessageRelease, true},
		{"warning", MessageWarning, true},
		{"question", MessageQuestion, true},
		{"answer", MessageAnswer, true},
		{"status", MessageStatus, true},
		{"unknown type", MessageType("unknown"), false},
		{"empty type", MessageType(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateMessageType(tt.msgType); got != tt.expected {
				t.Errorf("ValidateMessageType(%q) = %v, want %v", tt.msgType, got, tt.expected)
			}
		})
	}
}

func TestMessageType_Constants(t *testing.T) {
	// Verify all constants have the expected string values.
	tests := []struct {
		name     string
		msgType  MessageType
		expected string
	}{
		{"discovery", MessageDiscovery, "discovery"},
		{"claim", MessageClaim, "claim"},
		{"release", MessageRelease, "release"},
		{"warning", MessageWarning, "warning"},
		{"question", MessageQuestion, "question"},
		{"answer", MessageAnswer, "answer"},
		{"status", MessageStatus, "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.msgType) != tt.expected {
				t.Errorf("MessageType %s = %q, want %q", tt.name, tt.msgType, tt.expected)
			}
		})
	}
}

func TestBroadcastRecipient(t *testing.T) {
	if BroadcastRecipient != "broadcast" {
		t.Errorf("BroadcastRecipient = %q, want %q", BroadcastRecipient, "broadcast")
	}
}

func TestMessage_Fields(t *testing.T) {
	now := time.Now()
	msg := Message{
		ID:        "msg-123",
		From:      "instance-1",
		To:        "instance-2",
		Type:      MessageDiscovery,
		Body:      "found a helper",
		Timestamp: now,
		Metadata:  map[string]any{"file": "utils.go"},
	}

	if msg.ID != "msg-123" {
		t.Errorf("ID = %q, want %q", msg.ID, "msg-123")
	}
	if msg.From != "instance-1" {
		t.Errorf("From = %q, want %q", msg.From, "instance-1")
	}
	if msg.To != "instance-2" {
		t.Errorf("To = %q, want %q", msg.To, "instance-2")
	}
	if msg.Type != MessageDiscovery {
		t.Errorf("Type = %q, want %q", msg.Type, MessageDiscovery)
	}
	if msg.Body != "found a helper" {
		t.Errorf("Body = %q, want %q", msg.Body, "found a helper")
	}
	if !msg.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", msg.Timestamp, now)
	}
	if msg.Metadata["file"] != "utils.go" {
		t.Errorf("Metadata[file] = %v, want %q", msg.Metadata["file"], "utils.go")
	}
}

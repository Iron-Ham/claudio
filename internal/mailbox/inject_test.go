package mailbox

import (
	"strings"
	"testing"
	"time"
)

func TestFormatForPrompt_Empty(t *testing.T) {
	result := FormatForPrompt(nil)
	if result != "" {
		t.Errorf("FormatForPrompt(nil) = %q, want empty string", result)
	}

	result = FormatForPrompt([]Message{})
	if result != "" {
		t.Errorf("FormatForPrompt([]) = %q, want empty string", result)
	}
}

func TestFormatForPrompt_SingleMessage(t *testing.T) {
	messages := []Message{
		{
			From: "inst-1",
			To:   "inst-2",
			Type: MessageDiscovery,
			Body: "Found shared utility in pkg/utils",
		},
	}

	result := FormatForPrompt(messages)

	if !strings.Contains(result, "<mailbox-messages>") {
		t.Error("expected <mailbox-messages> opening tag")
	}
	if !strings.Contains(result, "</mailbox-messages>") {
		t.Error("expected </mailbox-messages> closing tag")
	}
	if !strings.Contains(result, "[DISCOVERY]") {
		t.Error("expected [DISCOVERY] header")
	}
	if !strings.Contains(result, "From: inst-1") {
		t.Error("expected From: inst-1")
	}
	if !strings.Contains(result, "Found shared utility in pkg/utils") {
		t.Error("expected message body")
	}
}

func TestFormatForPrompt_GroupsByType(t *testing.T) {
	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	messages := []Message{
		{From: "inst-1", To: "broadcast", Type: MessageDiscovery, Body: "disc-1", Timestamp: base},
		{From: "inst-2", To: "broadcast", Type: MessageWarning, Body: "warn-1", Timestamp: base.Add(time.Second)},
		{From: "inst-3", To: "broadcast", Type: MessageDiscovery, Body: "disc-2", Timestamp: base.Add(2 * time.Second)},
	}

	result := FormatForPrompt(messages)

	// DISCOVERY should appear before WARNING (order of first appearance)
	discIdx := strings.Index(result, "[DISCOVERY]")
	warnIdx := strings.Index(result, "[WARNING]")
	if discIdx < 0 {
		t.Fatal("expected [DISCOVERY] header")
	}
	if warnIdx < 0 {
		t.Fatal("expected [WARNING] header")
	}
	if discIdx >= warnIdx {
		t.Error("expected [DISCOVERY] before [WARNING] based on first-appearance order")
	}

	// Both discovery messages should be under the DISCOVERY header
	discSection := result[discIdx:warnIdx]
	if !strings.Contains(discSection, "disc-1") {
		t.Error("expected disc-1 under DISCOVERY")
	}
	if !strings.Contains(discSection, "disc-2") {
		t.Error("expected disc-2 under DISCOVERY")
	}
}

func TestFormatForPrompt_WithMetadata(t *testing.T) {
	messages := []Message{
		{
			From:     "inst-1",
			To:       "inst-2",
			Type:     MessageClaim,
			Body:     "claiming main.go",
			Metadata: map[string]any{"file": "main.go"},
		},
	}

	result := FormatForPrompt(messages)

	if !strings.Contains(result, "Metadata:") {
		t.Error("expected Metadata line for message with metadata")
	}
	if !strings.Contains(result, "file=main.go") {
		t.Error("expected file=main.go in metadata")
	}
}

func TestFormatForPrompt_WithoutMetadata(t *testing.T) {
	messages := []Message{
		{
			From: "inst-1",
			To:   "inst-2",
			Type: MessageStatus,
			Body: "50% complete",
		},
	}

	result := FormatForPrompt(messages)

	if strings.Contains(result, "Metadata:") {
		t.Error("expected no Metadata line for message without metadata")
	}
}

func TestFormatForPrompt_AllTypes(t *testing.T) {
	types := []MessageType{
		MessageDiscovery, MessageClaim, MessageRelease,
		MessageWarning, MessageQuestion, MessageAnswer, MessageStatus,
	}

	var messages []Message
	for _, mt := range types {
		messages = append(messages, Message{
			From: "inst-1",
			To:   "broadcast",
			Type: mt,
			Body: "test " + string(mt),
		})
	}

	result := FormatForPrompt(messages)

	expectedHeaders := []string{
		"[DISCOVERY]", "[CLAIM]", "[RELEASE]",
		"[WARNING]", "[QUESTION]", "[ANSWER]", "[STATUS]",
	}
	for _, header := range expectedHeaders {
		if !strings.Contains(result, header) {
			t.Errorf("expected header %s in output", header)
		}
	}
}

func TestFormatForPrompt_MultipleMessagesPerType(t *testing.T) {
	messages := []Message{
		{From: "inst-1", To: "broadcast", Type: MessageStatus, Body: "starting"},
		{From: "inst-2", To: "broadcast", Type: MessageStatus, Body: "halfway"},
		{From: "inst-3", To: "broadcast", Type: MessageStatus, Body: "done"},
	}

	result := FormatForPrompt(messages)

	// Should only have one [STATUS] header
	if strings.Count(result, "[STATUS]") != 1 {
		t.Errorf("expected exactly one [STATUS] header, got %d", strings.Count(result, "[STATUS]"))
	}

	// All bodies should appear
	for _, body := range []string{"starting", "halfway", "done"} {
		if !strings.Contains(result, body) {
			t.Errorf("expected body %q in output", body)
		}
	}
}

func TestFormatMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]any
		want     string
	}{
		{"nil map", nil, ""},
		{"empty map", map[string]any{}, ""},
		{"single entry", map[string]any{"file": "main.go"}, "file=main.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMetadata(tt.metadata)
			if got != tt.want {
				t.Errorf("formatMetadata() = %q, want %q", got, tt.want)
			}
		})
	}
}

package mailbox

import (
	"sync"
	"testing"
	"time"
)

func TestMailbox_NewMailbox(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)
	if mb == nil {
		t.Fatal("NewMailbox() returned nil")
	}
	if mb.store == nil {
		t.Fatal("NewMailbox() store is nil")
	}
	if mb.pollInterval != defaultPollInterval {
		t.Errorf("pollInterval = %v, want %v", mb.pollInterval, defaultPollInterval)
	}
}

func TestMailbox_SetPollInterval(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)

	mb.SetPollInterval(100 * time.Millisecond)
	if mb.pollInterval != 100*time.Millisecond {
		t.Errorf("pollInterval = %v, want %v", mb.pollInterval, 100*time.Millisecond)
	}

	// Zero and negative values should be ignored
	mb.SetPollInterval(0)
	if mb.pollInterval != 100*time.Millisecond {
		t.Errorf("pollInterval changed on zero, got %v", mb.pollInterval)
	}

	mb.SetPollInterval(-1 * time.Second)
	if mb.pollInterval != 100*time.Millisecond {
		t.Errorf("pollInterval changed on negative, got %v", mb.pollInterval)
	}
}

func TestMailbox_SendAndReceive(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)

	// Send a targeted message
	if err := mb.Send(Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageDiscovery,
		Body: "found something",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Send a broadcast
	if err := mb.Send(Message{
		From: "inst-1",
		To:   BroadcastRecipient,
		Type: MessageWarning,
		Body: "heads up",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// inst-2 should see both
	messages, err := mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// inst-3 should see only the broadcast
	messages, err = mb.Receive("inst-3")
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message for inst-3, got %d", len(messages))
	}
	if messages[0].Body != "heads up" {
		t.Errorf("Body = %q, want %q", messages[0].Body, "heads up")
	}
}

func TestMailbox_Receive_Empty(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)

	messages, err := mb.Receive("inst-1")
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestMailbox_Receive_ChronologicalOrder(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)

	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	// Send messages out of order
	if err := mb.Send(Message{
		From:      "inst-1",
		To:        "inst-2",
		Type:      MessageStatus,
		Body:      "third",
		Timestamp: base.Add(3 * time.Second),
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if err := mb.Send(Message{
		From:      "inst-1",
		To:        BroadcastRecipient,
		Type:      MessageStatus,
		Body:      "first",
		Timestamp: base.Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if err := mb.Send(Message{
		From:      "inst-3",
		To:        "inst-2",
		Type:      MessageStatus,
		Body:      "second",
		Timestamp: base.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}

	expected := []string{"first", "second", "third"}
	for i, want := range expected {
		if messages[i].Body != want {
			t.Errorf("messages[%d].Body = %q, want %q", i, messages[i].Body, want)
		}
	}
}

func TestMailbox_Watch(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)
	mb.SetPollInterval(20 * time.Millisecond)

	var mu sync.Mutex
	var received []Message

	cancel := mb.Watch("inst-2", func(msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	defer cancel()

	// Allow the watcher to initialize
	time.Sleep(30 * time.Millisecond)

	// Send a message after the watch starts
	if err := mb.Send(Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageDiscovery,
		Body: "new finding",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Wait for the watcher to pick it up
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 watched message, got %d", count)
	}
}

func TestMailbox_Watch_SkipsExistingMessages(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)
	mb.SetPollInterval(20 * time.Millisecond)

	// Send a message before the watch starts
	if err := mb.Send(Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageDiscovery,
		Body: "old message",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	var mu sync.Mutex
	var received []Message

	cancel := mb.Watch("inst-2", func(msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	defer cancel()

	// Wait several poll cycles
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 watched messages (old messages should be skipped), got %d", count)
	}
}

func TestMailbox_Watch_Cancel(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)
	mb.SetPollInterval(20 * time.Millisecond)

	var mu sync.Mutex
	var received []Message

	cancel := mb.Watch("inst-2", func(msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})

	// Cancel immediately
	cancel()

	// Send a message after cancellation
	if err := mb.Send(Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageDiscovery,
		Body: "after cancel",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 0 {
		t.Errorf("expected 0 watched messages after cancel, got %d", count)
	}
}

func TestMailbox_Watch_BroadcastMessages(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)
	mb.SetPollInterval(20 * time.Millisecond)

	var mu sync.Mutex
	var received []Message

	cancel := mb.Watch("inst-2", func(msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	defer cancel()

	// Allow the watcher to initialize
	time.Sleep(30 * time.Millisecond)

	// Send a broadcast message
	if err := mb.Send(Message{
		From: "inst-1",
		To:   BroadcastRecipient,
		Type: MessageWarning,
		Body: "broadcast alert",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 watched broadcast message, got %d", count)
	}
}

func TestMailbox_Watch_MultipleMessages(t *testing.T) {
	dir := t.TempDir()
	mb := NewMailbox(dir)
	mb.SetPollInterval(20 * time.Millisecond)

	var mu sync.Mutex
	var received []Message

	cancel := mb.Watch("inst-2", func(msg Message) {
		mu.Lock()
		received = append(received, msg)
		mu.Unlock()
	})
	defer cancel()

	// Allow the watcher to initialize
	time.Sleep(30 * time.Millisecond)

	// Send multiple messages
	for _, body := range []string{"one", "two", "three"} {
		if err := mb.Send(Message{
			From: "inst-1",
			To:   "inst-2",
			Type: MessageStatus,
			Body: body,
		}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 watched messages, got %d", count)
	}
}

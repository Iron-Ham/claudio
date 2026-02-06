package mailbox

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestStore_Send(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	msg := Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageDiscovery,
		Body: "hello",
	}

	if err := store.Send(msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Verify the file was created
	indexPath := filepath.Join(dir, mailboxDir, "inst-2", indexFile)
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index file not created: %v", err)
	}
}

func TestStore_Send_AutoPopulatesFields(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	msg := Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageStatus,
		Body: "working",
	}

	if err := store.Send(msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := store.ReadForInstance("inst-2")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].ID == "" {
		t.Error("expected auto-generated ID, got empty string")
	}
	if messages[0].Timestamp.IsZero() {
		t.Error("expected auto-generated Timestamp, got zero")
	}
}

func TestStore_Send_PreservesExistingIDAndTimestamp(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	msg := Message{
		ID:        "custom-id",
		From:      "inst-1",
		To:        "inst-2",
		Type:      MessageStatus,
		Body:      "update",
		Timestamp: now,
	}

	if err := store.Send(msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := store.ReadForInstance("inst-2")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}

	if messages[0].ID != "custom-id" {
		t.Errorf("ID = %q, want %q", messages[0].ID, "custom-id")
	}
	if !messages[0].Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", messages[0].Timestamp, now)
	}
}

func TestStore_Send_ValidationErrors(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	tests := []struct {
		name string
		msg  Message
	}{
		{"empty from", Message{To: "inst-2", Type: MessageStatus, Body: "hi"}},
		{"empty to", Message{From: "inst-1", Type: MessageStatus, Body: "hi"}},
		{"empty type", Message{From: "inst-1", To: "inst-2", Body: "hi"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := store.Send(tt.msg); err == nil {
				t.Error("expected error for invalid message, got nil")
			}
		})
	}
}

func TestStore_ReadForInstance(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Send two messages to inst-2
	for i, body := range []string{"first", "second"} {
		msg := Message{
			ID:        "msg-" + body,
			From:      "inst-1",
			To:        "inst-2",
			Type:      MessageDiscovery,
			Body:      body,
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := store.Send(msg); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}

	messages, err := store.ReadForInstance("inst-2")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Body != "first" {
		t.Errorf("messages[0].Body = %q, want %q", messages[0].Body, "first")
	}
	if messages[1].Body != "second" {
		t.Errorf("messages[1].Body = %q, want %q", messages[1].Body, "second")
	}
}

func TestStore_ReadForInstance_EmptyID(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	_, err := store.ReadForInstance("")
	if err == nil {
		t.Error("expected error for empty instanceID, got nil")
	}
}

func TestStore_ReadForInstance_NoMessages(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	messages, err := store.ReadForInstance("inst-1")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}
	if messages != nil {
		t.Errorf("expected nil for no messages, got %v", messages)
	}
}

func TestStore_ReadBroadcast(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	msg := Message{
		From: "inst-1",
		To:   BroadcastRecipient,
		Type: MessageWarning,
		Body: "heads up everyone",
	}
	if err := store.Send(msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := store.ReadBroadcast()
	if err != nil {
		t.Fatalf("ReadBroadcast() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Body != "heads up everyone" {
		t.Errorf("Body = %q, want %q", messages[0].Body, "heads up everyone")
	}
}

func TestStore_ReadBroadcast_NoMessages(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	messages, err := store.ReadBroadcast()
	if err != nil {
		t.Fatalf("ReadBroadcast() error = %v", err)
	}
	if messages != nil {
		t.Errorf("expected nil for no messages, got %v", messages)
	}
}

func TestStore_ReadAll(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	base := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	// Send a broadcast message (timestamp: t+1)
	if err := store.Send(Message{
		From:      "inst-1",
		To:        BroadcastRecipient,
		Type:      MessageWarning,
		Body:      "broadcast",
		Timestamp: base.Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Send a targeted message (timestamp: t+0, earlier)
	if err := store.Send(Message{
		From:      "inst-1",
		To:        "inst-2",
		Type:      MessageDiscovery,
		Body:      "targeted",
		Timestamp: base,
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := store.ReadAll("inst-2")
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}

	// Should be sorted chronologically: targeted first, broadcast second
	if messages[0].Body != "targeted" {
		t.Errorf("messages[0].Body = %q, want %q", messages[0].Body, "targeted")
	}
	if messages[1].Body != "broadcast" {
		t.Errorf("messages[1].Body = %q, want %q", messages[1].Body, "broadcast")
	}
}

func TestStore_ReadAll_BroadcastAndTargetedSeparate(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Message to inst-2 should not appear for inst-3
	if err := store.Send(Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageDiscovery,
		Body: "for inst-2 only",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Broadcast should appear for both
	if err := store.Send(Message{
		From: "inst-1",
		To:   BroadcastRecipient,
		Type: MessageStatus,
		Body: "for everyone",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// inst-3 should only see the broadcast
	msgs, err := store.ReadAll("inst-3")
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message for inst-3, got %d", len(msgs))
	}
	if msgs[0].Body != "for everyone" {
		t.Errorf("Body = %q, want %q", msgs[0].Body, "for everyone")
	}
}

func TestStore_ConcurrentSend(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			msg := Message{
				From: "inst-1",
				To:   "inst-2",
				Type: MessageStatus,
				Body: "msg",
			}
			if err := store.Send(msg); err != nil {
				t.Errorf("Send() error = %v", err)
			}
			_ = n
		}(i)
	}
	wg.Wait()

	messages, err := store.ReadForInstance("inst-2")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}
	if len(messages) != 20 {
		t.Errorf("expected 20 messages, got %d", len(messages))
	}
}

func TestStore_MetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	msg := Message{
		From:     "inst-1",
		To:       "inst-2",
		Type:     MessageClaim,
		Body:     "claiming file",
		Metadata: map[string]any{"file": "main.go", "line": float64(42)},
	}
	if err := store.Send(msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := store.ReadForInstance("inst-2")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Metadata["file"] != "main.go" {
		t.Errorf("Metadata[file] = %v, want %q", messages[0].Metadata["file"], "main.go")
	}
	// JSON numbers unmarshal as float64
	if messages[0].Metadata["line"] != float64(42) {
		t.Errorf("Metadata[line] = %v, want %v", messages[0].Metadata["line"], float64(42))
	}
}

func TestStore_NilMetadataOmitted(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	msg := Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageStatus,
		Body: "no metadata",
	}
	if err := store.Send(msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages, err := store.ReadForInstance("inst-2")
	if err != nil {
		t.Fatalf("ReadForInstance() error = %v", err)
	}
	if messages[0].Metadata != nil {
		t.Errorf("expected nil Metadata, got %v", messages[0].Metadata)
	}
}

func TestStore_DirectoryStructure(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Send broadcast
	if err := store.Send(Message{
		From: "inst-1",
		To:   BroadcastRecipient,
		Type: MessageStatus,
		Body: "broadcast",
	}); err != nil {
		t.Fatalf("Send broadcast error = %v", err)
	}

	// Send targeted
	if err := store.Send(Message{
		From: "inst-1",
		To:   "inst-2",
		Type: MessageStatus,
		Body: "targeted",
	}); err != nil {
		t.Fatalf("Send targeted error = %v", err)
	}

	// Verify directory structure
	broadcastDir := filepath.Join(dir, mailboxDir, BroadcastRecipient)
	if _, err := os.Stat(broadcastDir); err != nil {
		t.Errorf("broadcast directory not created: %v", err)
	}

	instanceDir := filepath.Join(dir, mailboxDir, "inst-2")
	if _, err := os.Stat(instanceDir); err != nil {
		t.Errorf("instance directory not created: %v", err)
	}
}

package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSessionLinkManager(t *testing.T) {
	manager := NewSessionLinkManager("/tmp/test")
	if manager == nil {
		t.Fatal("expected non-nil manager")
	}
	if manager.baseDir != "/tmp/test" {
		t.Errorf("expected baseDir '/tmp/test', got '%s'", manager.baseDir)
	}
	if manager.links == nil {
		t.Error("expected links map to be initialized")
	}
}

func TestLinkKey(t *testing.T) {
	key := linkKey("review123", "impl456")
	expected := "review123:impl456"
	if key != expected {
		t.Errorf("expected key '%s', got '%s'", expected, key)
	}
}

func TestSessionLinkManager_LinkSessions(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()
	claudioDir := filepath.Join(tmpDir, ".claudio", "sessions")

	// Create implementer session directory
	implDir := filepath.Join(claudioDir, "impl-session")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create impl dir: %v", err)
	}

	// Create review session directory
	reviewDir := filepath.Join(claudioDir, "review-session")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		t.Fatalf("failed to create review dir: %v", err)
	}

	manager := NewSessionLinkManager(tmpDir)

	t.Run("valid observe link", func(t *testing.T) {
		link, err := manager.LinkSessions("review-session", "impl-session", "observe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link == nil {
			t.Fatal("expected non-nil link")
		}
		if link.ReviewSessionID != "review-session" {
			t.Errorf("expected ReviewSessionID 'review-session', got '%s'", link.ReviewSessionID)
		}
		if link.ImplementerSessionID != "impl-session" {
			t.Errorf("expected ImplementerSessionID 'impl-session', got '%s'", link.ImplementerSessionID)
		}
		if link.LinkType != "observe" {
			t.Errorf("expected LinkType 'observe', got '%s'", link.LinkType)
		}
		if link.CommunicationFile == "" {
			t.Error("expected CommunicationFile to be set")
		}
	})

	t.Run("valid bidirectional link", func(t *testing.T) {
		// Create another review session
		reviewDir2 := filepath.Join(claudioDir, "review-session-2")
		if err := os.MkdirAll(reviewDir2, 0755); err != nil {
			t.Fatalf("failed to create review dir: %v", err)
		}

		link, err := manager.LinkSessions("review-session-2", "impl-session", "bidirectional")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if link.LinkType != "bidirectional" {
			t.Errorf("expected LinkType 'bidirectional', got '%s'", link.LinkType)
		}
	})

	t.Run("invalid link type", func(t *testing.T) {
		_, err := manager.LinkSessions("review-session", "impl-session", "invalid")
		if err == nil {
			t.Error("expected error for invalid link type")
		}
	})

	t.Run("non-existent implementer", func(t *testing.T) {
		_, err := manager.LinkSessions("review-session", "non-existent", "observe")
		if err == nil {
			t.Error("expected error for non-existent implementer")
		}
	})

	t.Run("idempotent linking", func(t *testing.T) {
		link1, err := manager.LinkSessions("review-session", "impl-session", "observe")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		link2, err := manager.LinkSessions("review-session", "impl-session", "observe")
		if err != nil {
			t.Fatalf("unexpected error on second link: %v", err)
		}
		if link1.CreatedAt != link2.CreatedAt {
			t.Error("expected same link to be returned")
		}
	})
}

func TestSessionLinkManager_UnlinkSessions(t *testing.T) {
	tmpDir := t.TempDir()
	claudioDir := filepath.Join(tmpDir, ".claudio", "sessions")

	implDir := filepath.Join(claudioDir, "impl-session")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create impl dir: %v", err)
	}

	reviewDir := filepath.Join(claudioDir, "review-session")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		t.Fatalf("failed to create review dir: %v", err)
	}

	manager := NewSessionLinkManager(tmpDir)

	// Create a link
	_, err := manager.LinkSessions("review-session", "impl-session", "observe")
	if err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	// Unlink
	err = manager.UnlinkSessions("review-session", "impl-session")
	if err != nil {
		t.Fatalf("failed to unlink: %v", err)
	}

	// Verify link is gone
	links := manager.GetLinkedSessions("review-session")
	if len(links) > 0 {
		t.Error("expected no links after unlinking")
	}

	// Unlinking non-existent link should not error
	err = manager.UnlinkSessions("non-existent", "also-non-existent")
	if err != nil {
		t.Errorf("unexpected error unlinking non-existent: %v", err)
	}
}

func TestSessionLinkManager_GetLinkedSessions(t *testing.T) {
	tmpDir := t.TempDir()
	claudioDir := filepath.Join(tmpDir, ".claudio", "sessions")

	// Create multiple session directories
	for _, id := range []string{"impl-1", "impl-2", "review-1", "review-2"} {
		if err := os.MkdirAll(filepath.Join(claudioDir, id), 0755); err != nil {
			t.Fatalf("failed to create dir: %v", err)
		}
	}

	manager := NewSessionLinkManager(tmpDir)

	// Create multiple links
	_, _ = manager.LinkSessions("review-1", "impl-1", "observe")
	_, _ = manager.LinkSessions("review-1", "impl-2", "observe")
	_, _ = manager.LinkSessions("review-2", "impl-1", "bidirectional")

	t.Run("get links for reviewer", func(t *testing.T) {
		links := manager.GetLinkedSessions("review-1")
		if len(links) != 2 {
			t.Errorf("expected 2 links for review-1, got %d", len(links))
		}
	})

	t.Run("get links for implementer", func(t *testing.T) {
		links := manager.GetLinkedSessions("impl-1")
		if len(links) != 2 {
			t.Errorf("expected 2 links for impl-1, got %d", len(links))
		}
	})

	t.Run("no links for unknown session", func(t *testing.T) {
		links := manager.GetLinkedSessions("unknown")
		if len(links) != 0 {
			t.Errorf("expected 0 links for unknown, got %d", len(links))
		}
	})
}

func TestSessionLinkManager_SendAndReadMessages(t *testing.T) {
	tmpDir := t.TempDir()
	claudioDir := filepath.Join(tmpDir, ".claudio", "sessions")

	implDir := filepath.Join(claudioDir, "impl-session")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create impl dir: %v", err)
	}

	reviewDir := filepath.Join(claudioDir, "review-session")
	if err := os.MkdirAll(reviewDir, 0755); err != nil {
		t.Fatalf("failed to create review dir: %v", err)
	}

	manager := NewSessionLinkManager(tmpDir)

	link, err := manager.LinkSessions("review-session", "impl-session", "observe")
	if err != nil {
		t.Fatalf("failed to create link: %v", err)
	}

	t.Run("send and read message", func(t *testing.T) {
		msg := NewReviewMessage("review-agent", ReviewMessageIssue, "Found a security issue")

		err := manager.SendReviewMessage(link, msg)
		if err != nil {
			t.Fatalf("failed to send message: %v", err)
		}

		// Read messages
		messages := manager.GetAllMessages(link)
		if len(messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(messages))
		}
		if messages[0].Content != "Found a security issue" {
			t.Errorf("unexpected message content: %s", messages[0].Content)
		}
		if messages[0].Type != ReviewMessageIssue {
			t.Errorf("unexpected message type: %s", messages[0].Type)
		}
	})

	t.Run("read messages since timestamp", func(t *testing.T) {
		beforeSecond := time.Now()
		time.Sleep(10 * time.Millisecond)

		msg2 := NewReviewMessage("review-agent", ReviewMessageSuggestion, "Consider using a different approach")
		err := manager.SendReviewMessage(link, msg2)
		if err != nil {
			t.Fatalf("failed to send second message: %v", err)
		}

		// Read only new messages
		newMessages := manager.ReadReviewMessages(link, beforeSecond)
		if len(newMessages) != 1 {
			t.Errorf("expected 1 new message, got %d", len(newMessages))
		}
	})

	t.Run("send message with issue reference", func(t *testing.T) {
		msg := NewReviewMessageWithIssue("review-agent", ReviewMessageQuestion, "Is this intentional?", "ISSUE-123")
		err := manager.SendReviewMessage(link, msg)
		if err != nil {
			t.Fatalf("failed to send message: %v", err)
		}

		messages := manager.GetAllMessages(link)
		// Find the message with issue ref
		var found bool
		for _, m := range messages {
			if m.IssueRef == "ISSUE-123" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected to find message with issue reference")
		}
	})
}

func TestReviewChannel_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	channelFile := filepath.Join(tmpDir, "review_channel.json")

	manager := NewSessionLinkManager(tmpDir)

	// Write channel
	channel := &ReviewChannel{
		Messages: []ReviewMessage{
			{ID: "1", From: "agent", Type: ReviewMessageIssue, Content: "test", Timestamp: time.Now()},
		},
	}

	err := manager.writeReviewChannel(channelFile, channel)
	if err != nil {
		t.Fatalf("failed to write channel: %v", err)
	}

	// Read back
	readChannel, err := manager.readReviewChannel(channelFile)
	if err != nil {
		t.Fatalf("failed to read channel: %v", err)
	}

	if len(readChannel.Messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(readChannel.Messages))
	}
	if readChannel.Messages[0].Content != "test" {
		t.Errorf("expected content 'test', got '%s'", readChannel.Messages[0].Content)
	}
}

func TestObserverLock(t *testing.T) {
	tmpDir := t.TempDir()
	claudioDir := filepath.Join(tmpDir, ".claudio", "sessions")

	implDir := filepath.Join(claudioDir, "impl-session")
	if err := os.MkdirAll(implDir, 0755); err != nil {
		t.Fatalf("failed to create impl dir: %v", err)
	}

	manager := NewSessionLinkManager(tmpDir)

	t.Run("acquire and check observers", func(t *testing.T) {
		err := manager.acquireObserverLock("impl-session", "reviewer-1")
		if err != nil {
			t.Fatalf("failed to acquire observer lock: %v", err)
		}

		hasObservers := manager.HasObservers("impl-session")
		if !hasObservers {
			t.Error("expected session to have observers")
		}

		observers := manager.GetObservers("impl-session")
		if len(observers) != 1 {
			t.Errorf("expected 1 observer, got %d", len(observers))
		}
	})

	t.Run("multiple observers allowed", func(t *testing.T) {
		err := manager.acquireObserverLock("impl-session", "reviewer-2")
		if err != nil {
			t.Fatalf("failed to acquire second observer lock: %v", err)
		}

		observers := manager.GetObservers("impl-session")
		if len(observers) != 2 {
			t.Errorf("expected 2 observers, got %d", len(observers))
		}
	})

	t.Run("release observer", func(t *testing.T) {
		err := manager.releaseObserverLock("impl-session", "reviewer-1")
		if err != nil {
			t.Fatalf("failed to release observer lock: %v", err)
		}

		observers := manager.GetObservers("impl-session")
		if len(observers) != 1 {
			t.Errorf("expected 1 observer after release, got %d", len(observers))
		}
	})
}

func TestGenerateContextMarkdownForLink(t *testing.T) {
	link := &SessionLink{
		ReviewSessionID:      "review-123",
		ImplementerSessionID: "impl-456",
		LinkType:             "observe",
		CreatedAt:            time.Now(),
		CommunicationFile:    "/path/to/channel.json",
	}

	messages := []ReviewMessage{
		{ID: "1", From: "security-agent", Type: ReviewMessageIssue, Content: "Found SQL injection", Timestamp: time.Now()},
		{ID: "2", From: "performance-agent", Type: ReviewMessageSuggestion, Content: "Consider caching", Timestamp: time.Now()},
	}

	md := GenerateContextMarkdownForLink(link, messages)

	// Check that the markdown contains expected sections
	if !contains(md, "## Code Review Session") {
		t.Error("expected markdown to contain Code Review Session header")
	}
	if !contains(md, "review-123") {
		t.Error("expected markdown to contain review session ID")
	}
	if !contains(md, "impl-456") {
		t.Error("expected markdown to contain implementer session ID")
	}
	if !contains(md, "Found SQL injection") {
		t.Error("expected markdown to contain message content")
	}
	if !contains(md, "Consider caching") {
		t.Error("expected markdown to contain suggestion content")
	}
}

func TestNewReviewMessage(t *testing.T) {
	msg := NewReviewMessage("test-agent", ReviewMessageIssue, "test content")

	if msg.ID == "" {
		t.Error("expected ID to be generated")
	}
	if msg.From != "test-agent" {
		t.Errorf("expected From 'test-agent', got '%s'", msg.From)
	}
	if msg.Type != ReviewMessageIssue {
		t.Errorf("expected Type 'issue', got '%s'", msg.Type)
	}
	if msg.Content != "test content" {
		t.Errorf("expected Content 'test content', got '%s'", msg.Content)
	}
	if msg.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

func TestSessionLink_JSONSerialization(t *testing.T) {
	link := &SessionLink{
		ReviewSessionID:      "review-123",
		ImplementerSessionID: "impl-456",
		LinkType:             "observe",
		CreatedAt:            time.Now(),
		CommunicationFile:    "/path/to/channel.json",
	}

	data, err := json.Marshal(link)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded SessionLink
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.ReviewSessionID != link.ReviewSessionID {
		t.Error("ReviewSessionID mismatch")
	}
	if decoded.ImplementerSessionID != link.ImplementerSessionID {
		t.Error("ImplementerSessionID mismatch")
	}
	if decoded.LinkType != link.LinkType {
		t.Error("LinkType mismatch")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

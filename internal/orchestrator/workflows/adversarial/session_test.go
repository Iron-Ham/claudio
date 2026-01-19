package adversarial

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	id := "test-session-123"
	task := "Implement a rate limiter"
	config := DefaultConfig()

	session := NewSession(id, task, config)

	if session.ID != id {
		t.Errorf("session.ID = %q, want %q", session.ID, id)
	}
	if session.Task != task {
		t.Errorf("session.Task = %q, want %q", session.Task, task)
	}
	if session.Phase != PhaseImplementing {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseImplementing)
	}
	if session.CurrentRound != 1 {
		t.Errorf("session.CurrentRound = %d, want %d", session.CurrentRound, 1)
	}
	if session.Created.IsZero() {
		t.Error("session.Created should not be zero")
	}
	if session.Config.MaxIterations != config.MaxIterations {
		t.Errorf("session.Config.MaxIterations = %d, want %d", session.Config.MaxIterations, config.MaxIterations)
	}
	if session.Config.MinPassingScore != config.MinPassingScore {
		t.Errorf("session.Config.MinPassingScore = %d, want %d", session.Config.MinPassingScore, config.MinPassingScore)
	}
}

func TestSession_IsActive(t *testing.T) {
	tests := []struct {
		name  string
		phase Phase
		want  bool
	}{
		{"implementing is active", PhaseImplementing, true},
		{"reviewing is active", PhaseReviewing, true},
		{"approved is not active", PhaseApproved, false},
		{"complete is not active", PhaseComplete, false},
		{"failed is not active", PhaseFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{Phase: tt.phase}
			got := session.IsActive()
			if got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxIterations != 10 {
		t.Errorf("MaxIterations = %d, want %d", config.MaxIterations, 10)
	}
	if config.MinPassingScore != 8 {
		t.Errorf("MinPassingScore = %d, want %d", config.MinPassingScore, 8)
	}
}

func TestPhases(t *testing.T) {
	// Verify phase constants are distinct
	phases := []Phase{
		PhaseImplementing,
		PhaseReviewing,
		PhaseApproved,
		PhaseComplete,
		PhaseFailed,
	}

	seen := make(map[Phase]bool)
	for _, phase := range phases {
		if seen[phase] {
			t.Errorf("duplicate phase constant: %v", phase)
		}
		seen[phase] = true
	}
}

func TestEventTypes(t *testing.T) {
	// Verify event types are distinct
	eventTypes := []EventType{
		EventImplementerStarted,
		EventIncrementReady,
		EventReviewerStarted,
		EventReviewReady,
		EventApproved,
		EventRejected,
		EventPhaseChange,
		EventComplete,
		EventFailed,
	}

	seen := make(map[EventType]bool)
	for _, et := range eventTypes {
		if seen[et] {
			t.Errorf("duplicate event type constant: %v", et)
		}
		seen[et] = true
	}
}

func TestManager_SetPhase(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	var receivedEvent Event
	manager.SetEventCallback(func(event Event) {
		receivedEvent = event
	})

	manager.SetPhase(PhaseReviewing)

	if session.Phase != PhaseReviewing {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseReviewing)
	}
	if receivedEvent.Type != EventPhaseChange {
		t.Errorf("event type = %v, want %v", receivedEvent.Type, EventPhaseChange)
	}
	if receivedEvent.Message != string(PhaseReviewing) {
		t.Errorf("event message = %q, want %q", receivedEvent.Message, string(PhaseReviewing))
	}
}

func TestManager_StartRound(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	if len(session.History) != 0 {
		t.Errorf("initial history length = %d, want 0", len(session.History))
	}

	manager.StartRound()

	if len(session.History) != 1 {
		t.Errorf("history length after StartRound = %d, want 1", len(session.History))
	}
	if session.History[0].Round != 1 {
		t.Errorf("history[0].Round = %d, want 1", session.History[0].Round)
	}
	if session.History[0].StartedAt.IsZero() {
		t.Error("history[0].StartedAt should not be zero")
	}
}

func TestManager_RecordIncrement(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	var receivedEvent Event
	manager.SetEventCallback(func(event Event) {
		receivedEvent = event
	})

	// Start a round first
	manager.StartRound()

	increment := &IncrementFile{
		Round:         1,
		Status:        "ready_for_review",
		Summary:       "Implemented feature",
		FilesModified: []string{"main.go"},
	}

	manager.RecordIncrement(increment)

	if session.History[0].Increment == nil {
		t.Error("history[0].Increment should not be nil")
	}
	if session.History[0].Increment.Summary != increment.Summary {
		t.Errorf("increment summary = %q, want %q", session.History[0].Increment.Summary, increment.Summary)
	}
	if receivedEvent.Type != EventIncrementReady {
		t.Errorf("event type = %v, want %v", receivedEvent.Type, EventIncrementReady)
	}
}

func TestManager_RecordReview_Approved(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	var events []Event
	manager.SetEventCallback(func(event Event) {
		events = append(events, event)
	})

	// Start a round first
	manager.StartRound()

	review := &ReviewFile{
		Round:    1,
		Approved: true,
		Score:    9,
		Summary:  "Excellent implementation",
	}

	manager.RecordReview(review)

	if session.History[0].Review == nil {
		t.Error("history[0].Review should not be nil")
	}
	if session.History[0].ReviewedAt == nil {
		t.Error("history[0].ReviewedAt should not be nil")
	}

	// Should have EventApproved
	hasApproved := false
	for _, e := range events {
		if e.Type == EventApproved {
			hasApproved = true
			break
		}
	}
	if !hasApproved {
		t.Error("expected EventApproved event")
	}
}

func TestManager_RecordReview_Rejected(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	var events []Event
	manager.SetEventCallback(func(event Event) {
		events = append(events, event)
	})

	// Start a round first
	manager.StartRound()

	review := &ReviewFile{
		Round:    1,
		Approved: false,
		Score:    5,
		Summary:  "Needs improvement",
		Issues:   []string{"Missing tests", "No error handling"},
	}

	manager.RecordReview(review)

	// Should have EventRejected
	hasRejected := false
	for _, e := range events {
		if e.Type == EventRejected {
			hasRejected = true
			break
		}
	}
	if !hasRejected {
		t.Error("expected EventRejected event")
	}
}

func TestManager_NextRound(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	if session.CurrentRound != 1 {
		t.Errorf("initial round = %d, want 1", session.CurrentRound)
	}

	manager.NextRound()

	if session.CurrentRound != 2 {
		t.Errorf("round after NextRound = %d, want 2", session.CurrentRound)
	}
}

func TestManager_IsMaxIterationsReached(t *testing.T) {
	tests := []struct {
		name          string
		maxIterations int
		currentRound  int
		want          bool
	}{
		{"unlimited (0)", 0, 100, false},
		{"under limit", 10, 5, false},
		{"at limit", 10, 10, false},
		{"over limit", 10, 11, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Config:       Config{MaxIterations: tt.maxIterations},
				CurrentRound: tt.currentRound,
			}
			manager := NewManager(session, nil)

			got := manager.IsMaxIterationsReached()
			if got != tt.want {
				t.Errorf("IsMaxIterationsReached() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_Session(t *testing.T) {
	session := NewSession("test-id", "test task", DefaultConfig())
	manager := NewManager(session, nil)

	got := manager.Session()
	if got != session {
		t.Error("Session() should return the same session")
	}
}

func TestIncrementFilePath(t *testing.T) {
	path := IncrementFilePath("/tmp/worktree")
	expected := "/tmp/worktree/.claudio-adversarial-incremental.json"
	if path != expected {
		t.Errorf("IncrementFilePath() = %q, want %q", path, expected)
	}
}

func TestReviewFilePath(t *testing.T) {
	path := ReviewFilePath("/tmp/worktree")
	expected := "/tmp/worktree/.claudio-adversarial-review.json"
	if path != expected {
		t.Errorf("ReviewFilePath() = %q, want %q", path, expected)
	}
}

func TestParseIncrementFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	increment := IncrementFile{
		Round:         2,
		Status:        "ready_for_review",
		Summary:       "Implemented feature X",
		FilesModified: []string{"main.go", "main_test.go"},
		Approach:      "Used TDD approach",
		Notes:         "Some implementation notes",
	}

	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	data, err := json.MarshalIndent(increment, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal increment: %v", err)
	}
	if err := os.WriteFile(incrementPath, data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	parsed, err := ParseIncrementFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseIncrementFile() error = %v", err)
	}

	if parsed.Round != increment.Round {
		t.Errorf("Round = %d, want %d", parsed.Round, increment.Round)
	}
	if parsed.Status != increment.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, increment.Status)
	}
	if parsed.Summary != increment.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, increment.Summary)
	}
	if len(parsed.FilesModified) != len(increment.FilesModified) {
		t.Errorf("len(FilesModified) = %d, want %d", len(parsed.FilesModified), len(increment.FilesModified))
	}
}

func TestParseIncrementFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = ParseIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestParseIncrementFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid increment file: %v", err)
	}

	_, err = ParseIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse adversarial increment JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseIncrementFile_InvalidRound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	increment := IncrementFile{
		Round:  0, // Invalid - must be >= 1
		Status: "ready_for_review",
	}

	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	data, _ := json.Marshal(increment)
	if err := os.WriteFile(incrementPath, data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	_, err = ParseIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when round is invalid")
	}
	if !strings.Contains(err.Error(), "invalid round number") {
		t.Errorf("error message should mention invalid round, got: %v", err)
	}
}

func TestParseIncrementFile_InvalidStatus(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	increment := IncrementFile{
		Round:  1,
		Status: "invalid_status",
	}

	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	data, _ := json.Marshal(increment)
	if err := os.WriteFile(incrementPath, data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	_, err = ParseIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when status is invalid")
	}
	if !strings.Contains(err.Error(), "invalid status") {
		t.Errorf("error message should mention invalid status, got: %v", err)
	}
}

func TestParseReviewFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	review := ReviewFile{
		Round:           2,
		Approved:        true,
		Score:           9,
		Strengths:       []string{"Clean code", "Good tests"},
		Issues:          []string{},
		Suggestions:     []string{"Consider adding comments"},
		Summary:         "Excellent implementation",
		RequiredChanges: []string{},
	}

	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review: %v", err)
	}
	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	parsed, err := ParseReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseReviewFile() error = %v", err)
	}

	if parsed.Round != review.Round {
		t.Errorf("Round = %d, want %d", parsed.Round, review.Round)
	}
	if parsed.Approved != review.Approved {
		t.Errorf("Approved = %v, want %v", parsed.Approved, review.Approved)
	}
	if parsed.Score != review.Score {
		t.Errorf("Score = %d, want %d", parsed.Score, review.Score)
	}
	if len(parsed.Strengths) != len(review.Strengths) {
		t.Errorf("len(Strengths) = %d, want %d", len(parsed.Strengths), len(review.Strengths))
	}
}

func TestParseReviewFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = ParseReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestParseReviewFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	if err := os.WriteFile(reviewPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid review file: %v", err)
	}

	_, err = ParseReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse adversarial review JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseReviewFile_InvalidRound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	review := ReviewFile{
		Round: 0, // Invalid - must be >= 1
		Score: 5,
	}

	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	data, _ := json.Marshal(review)
	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	_, err = ParseReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when round is invalid")
	}
	if !strings.Contains(err.Error(), "invalid round number") {
		t.Errorf("error message should mention invalid round, got: %v", err)
	}
}

func TestParseReviewFile_InvalidScore(t *testing.T) {
	tests := []struct {
		name  string
		score int
	}{
		{"score too low", 0},
		{"score too high", 11},
		{"negative score", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "adversarial-test-*")
			if err != nil {
				t.Fatalf("failed to create temp dir: %v", err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

			review := ReviewFile{
				Round: 1,
				Score: tt.score,
			}

			reviewPath := filepath.Join(tmpDir, ReviewFileName)
			data, _ := json.Marshal(review)
			if err := os.WriteFile(reviewPath, data, 0644); err != nil {
				t.Fatalf("failed to write review file: %v", err)
			}

			_, err = ParseReviewFile(tmpDir)
			if err == nil {
				t.Error("expected error when score is invalid")
			}
			if !strings.Contains(err.Error(), "invalid score") {
				t.Errorf("error message should mention invalid score, got: %v", err)
			}
		})
	}
}

func TestRound_Fields(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)

	increment := &IncrementFile{
		Round:   1,
		Status:  "ready_for_review",
		Summary: "Test increment",
	}

	review := &ReviewFile{
		Round:    1,
		Approved: true,
		Score:    9,
	}

	round := Round{
		Round:      1,
		Increment:  increment,
		Review:     review,
		StartedAt:  now,
		ReviewedAt: &later,
	}

	if round.Round != 1 {
		t.Errorf("Round = %d, want 1", round.Round)
	}
	if round.Increment == nil {
		t.Error("Increment should not be nil")
	}
	if round.Review == nil {
		t.Error("Review should not be nil")
	}
	if round.ReviewedAt == nil {
		t.Error("ReviewedAt should not be nil")
	}
	if !round.ReviewedAt.After(round.StartedAt) {
		t.Error("ReviewedAt should be after StartedAt")
	}
}

func TestEvent_Fields(t *testing.T) {
	now := time.Now()

	event := Event{
		Type:       EventApproved,
		Round:      2,
		InstanceID: "inst-123",
		Message:    "Work approved",
		Timestamp:  now,
	}

	if event.Type != EventApproved {
		t.Errorf("Type = %v, want %v", event.Type, EventApproved)
	}
	if event.Round != 2 {
		t.Errorf("Round = %d, want 2", event.Round)
	}
	if event.InstanceID != "inst-123" {
		t.Errorf("InstanceID = %q, want %q", event.InstanceID, "inst-123")
	}
	if event.Message != "Work approved" {
		t.Errorf("Message = %q, want %q", event.Message, "Work approved")
	}
}

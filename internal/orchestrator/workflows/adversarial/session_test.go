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
	tmpDir := t.TempDir()

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
	tmpDir := t.TempDir()

	_, err := ParseIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestParseIncrementFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte("not json at all"), 0644); err != nil {
		t.Fatalf("failed to write invalid increment file: %v", err)
	}

	_, err := ParseIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("error message should mention 'not valid JSON', got: %v", err)
	}
}

func TestParseIncrementFile_MissingRequiredFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, IncrementFileName)

	tests := []struct {
		name          string
		json          string
		expectedError string
	}{
		{
			name:          "missing round",
			json:          `{"status": "ready_for_review", "summary": "test", "files_modified": [], "approach": "test"}`,
			expectedError: "missing required fields: [round]",
		},
		{
			name:          "missing status",
			json:          `{"round": 1, "summary": "test", "files_modified": [], "approach": "test"}`,
			expectedError: "missing required fields: [status]",
		},
		{
			name:          "missing summary",
			json:          `{"round": 1, "status": "ready_for_review", "files_modified": [], "approach": "test"}`,
			expectedError: "missing required fields: [summary]",
		},
		{
			name:          "missing files_modified",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "approach": "test"}`,
			expectedError: "missing required fields: [files_modified]",
		},
		{
			name:          "missing approach",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": []}`,
			expectedError: "missing required fields: [approach]",
		},
		{
			name:          "missing multiple fields",
			json:          `{"round": 1}`,
			expectedError: "missing required fields:",
		},
		{
			name:          "empty object",
			json:          `{}`,
			expectedError: "missing required fields:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.json), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}

			_, err := ParseIncrementFile(tmpDir)
			if err == nil {
				t.Fatal("ParseIncrementFile should fail for missing required fields")
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("error should contain %q, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestParseIncrementFile_WrongFieldTypes(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, IncrementFileName)

	tests := []struct {
		name          string
		json          string
		expectedError string
	}{
		{
			name:          "round is string",
			json:          `{"round": "1", "status": "ready_for_review", "summary": "test", "files_modified": [], "approach": "test"}`,
			expectedError: "'round' must be a number",
		},
		{
			name:          "status is number",
			json:          `{"round": 1, "status": 123, "summary": "test", "files_modified": [], "approach": "test"}`,
			expectedError: "'status' must be a string",
		},
		{
			name:          "summary is number",
			json:          `{"round": 1, "status": "ready_for_review", "summary": 123, "files_modified": [], "approach": "test"}`,
			expectedError: "'summary' must be a string",
		},
		{
			name:          "files_modified is string",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": "file.go", "approach": "test"}`,
			expectedError: "'files_modified' must be an array",
		},
		{
			name:          "files_modified contains number",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": [123], "approach": "test"}`,
			expectedError: "'files_modified[0]' must be a string",
		},
		{
			name:          "approach is array",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": [], "approach": ["test"]}`,
			expectedError: "'approach' must be a string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.json), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}

			_, err := ParseIncrementFile(tmpDir)
			if err == nil {
				t.Fatal("ParseIncrementFile should fail for wrong field types")
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("error should contain %q, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestParseIncrementFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, IncrementFileName)

	tests := []struct {
		name          string
		json          string
		expectedError string
	}{
		{
			name:          "round is zero",
			json:          `{"round": 0, "status": "ready_for_review", "summary": "test", "files_modified": ["f.go"], "approach": "test"}`,
			expectedError: "round must be >= 1",
		},
		{
			name:          "round is negative",
			json:          `{"round": -1, "status": "ready_for_review", "summary": "test", "files_modified": ["f.go"], "approach": "test"}`,
			expectedError: "round must be >= 1",
		},
		{
			name:          "invalid status",
			json:          `{"round": 1, "status": "done", "summary": "test", "files_modified": ["f.go"], "approach": "test"}`,
			expectedError: "status must be 'ready_for_review' or 'failed'",
		},
		{
			name:          "empty summary when ready_for_review",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "", "files_modified": ["f.go"], "approach": "test"}`,
			expectedError: "summary cannot be empty",
		},
		{
			name:          "whitespace-only summary when ready_for_review",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "   ", "files_modified": ["f.go"], "approach": "test"}`,
			expectedError: "summary cannot be empty",
		},
		{
			name:          "empty approach when ready_for_review",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": ["f.go"], "approach": ""}`,
			expectedError: "approach cannot be empty",
		},
		{
			name:          "empty files_modified when ready_for_review",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": [], "approach": "test"}`,
			expectedError: "files_modified cannot be empty",
		},
		{
			name:          "empty string in files_modified",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": ["", "main.go"], "approach": "test"}`,
			expectedError: "files_modified[0] cannot be empty or whitespace",
		},
		{
			name:          "whitespace-only in files_modified",
			json:          `{"round": 1, "status": "ready_for_review", "summary": "test", "files_modified": ["   "], "approach": "test"}`,
			expectedError: "files_modified[0] cannot be empty or whitespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tt.json), 0644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}

			_, err := ParseIncrementFile(tmpDir)
			if err == nil {
				t.Fatal("ParseIncrementFile should fail for invalid content")
			}

			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("error should contain %q, got: %v", tt.expectedError, err)
			}
		})
	}
}

func TestParseIncrementFile_FailedStatusAllowsEmptyFields(t *testing.T) {
	tmpDir := t.TempDir()

	// When status is "failed", files_modified and approach can be empty
	// (the implementer may not have made any changes)
	increment := IncrementFile{
		Round:         1,
		Status:        "failed",
		Summary:       "Could not complete the task due to missing dependencies",
		FilesModified: []string{}, // Empty is OK for failed status
		Approach:      "",         // Empty is OK for failed status
		Notes:         "Need to install package X first",
	}

	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	data, _ := json.Marshal(increment)
	if err := os.WriteFile(incrementPath, data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	parsed, err := ParseIncrementFile(tmpDir)
	if err != nil {
		t.Errorf("ParseIncrementFile() should succeed for failed status with empty fields, got error: %v", err)
	}
	if parsed == nil {
		t.Error("ParseIncrementFile() returned nil for valid failed status")
	}
}

func TestParseIncrementFile_MultipleErrors(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, IncrementFileName)

	// Multiple validation errors should all be reported
	jsonContent := `{"round": 0, "status": "invalid", "summary": "", "files_modified": [], "approach": ""}`
	if err := os.WriteFile(path, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err := ParseIncrementFile(tmpDir)
	if err == nil {
		t.Fatal("ParseIncrementFile should fail with multiple errors")
	}

	// Should contain multiple error messages
	errStr := err.Error()
	if !strings.Contains(errStr, "round must be >= 1") {
		t.Error("error should mention round validation failure")
	}
	if !strings.Contains(errStr, "status must be") {
		t.Error("error should mention status validation failure")
	}
}

// Test sanitization of increment files with various LLM quirks
func TestParseIncrementFile_Sanitization(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(*testing.T, *IncrementFile)
	}{
		{
			name: "smart quotes in field values",
			content: `{
  "round": 1,
  "status": "ready_for_review",
  "summary": "Implemented feature",
  "files_modified": ["main.go"],
  "approach": "Used TDD",
  "notes": "Some notes"
}`,
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Summary != "Implemented feature" {
					t.Errorf("Summary = %q, want %q", inc.Summary, "Implemented feature")
				}
			},
		},
		{
			name: "wrapped in markdown code block",
			content: "```json\n" + `{
  "round": 1,
  "status": "ready_for_review",
  "summary": "Test summary",
  "files_modified": ["main.go"],
  "approach": "Test approach",
  "notes": ""
}` + "\n```",
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Summary != "Test summary" {
					t.Errorf("Summary = %q, want %q", inc.Summary, "Test summary")
				}
			},
		},
		{
			name: "wrapped in generic code block",
			content: "```\n" + `{
  "round": 1,
  "status": "ready_for_review",
  "summary": "Generic block",
  "files_modified": ["main.go"],
  "approach": "Test approach",
  "notes": ""
}` + "\n```",
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Summary != "Generic block" {
					t.Errorf("Summary = %q, want %q", inc.Summary, "Generic block")
				}
			},
		},
		{
			name: "extra text before JSON",
			content: `Here is the increment file:
{
  "round": 1,
  "status": "ready_for_review",
  "summary": "With prefix text",
  "files_modified": ["main.go"],
  "approach": "Test approach",
  "notes": ""
}`,
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Summary != "With prefix text" {
					t.Errorf("Summary = %q, want %q", inc.Summary, "With prefix text")
				}
			},
		},
		{
			name: "extra text after JSON",
			content: `{
  "round": 1,
  "status": "ready_for_review",
  "summary": "With suffix text",
  "files_modified": ["main.go"],
  "approach": "Test approach",
  "notes": ""
}
I hope this helps!`,
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Summary != "With suffix text" {
					t.Errorf("Summary = %q, want %q", inc.Summary, "With suffix text")
				}
			},
		},
		{
			name: "left double quotation mark",
			content: `{
  "round": 1,
  "status": "ready_for_review",
  "summary": "Left quote test",
  "files_modified": ["main.go"],
  "approach": "Test approach",
  "notes": ""
}`,
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Round != 1 {
					t.Errorf("Round = %d, want %d", inc.Round, 1)
				}
			},
		},
		{
			name: "combination of all issues",
			content: "Here is my work:\n```json\n" + `{
  "round": 2,
  "status": "ready_for_review",
  "summary": "Full sanitization test",
  "files_modified": ["file.go"],
  "approach": "Combined approach",
  "notes": "All fixed"
}` + "\n```\nLet me know if you need changes.",
			wantErr: false,
			validate: func(t *testing.T, inc *IncrementFile) {
				if inc.Round != 2 {
					t.Errorf("Round = %d, want %d", inc.Round, 2)
				}
				if inc.Summary != "Full sanitization test" {
					t.Errorf("Summary = %q, want %q", inc.Summary, "Full sanitization test")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			incrementPath := filepath.Join(tmpDir, IncrementFileName)
			if err := os.WriteFile(incrementPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write increment file: %v", err)
			}

			parsed, err := ParseIncrementFile(tmpDir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseIncrementFile() error = %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, parsed)
			}
		})
	}
}

func TestParseReviewFile(t *testing.T) {
	tmpDir := t.TempDir()

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
	tmpDir := t.TempDir()

	_, err := ParseReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestParseReviewFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	if err := os.WriteFile(reviewPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid review file: %v", err)
	}

	_, err := ParseReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse adversarial review JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseReviewFile_InvalidRound(t *testing.T) {
	tmpDir := t.TempDir()

	review := ReviewFile{
		Round: 0, // Invalid - must be >= 1
		Score: 5,
	}

	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	data, _ := json.Marshal(review)
	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	_, err := ParseReviewFile(tmpDir)
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
			tmpDir := t.TempDir()

			review := ReviewFile{
				Round: 1,
				Score: tt.score,
			}

			reviewPath := filepath.Join(tmpDir, ReviewFileName)
			data, _ := json.Marshal(review)
			if err := os.WriteFile(reviewPath, data, 0644); err != nil {
				t.Fatalf("failed to write review file: %v", err)
			}

			_, err := ParseReviewFile(tmpDir)
			if err == nil {
				t.Error("expected error when score is invalid")
			}
			if !strings.Contains(err.Error(), "invalid score") {
				t.Errorf("error message should mention invalid score, got: %v", err)
			}
		})
	}
}

// Test sanitization of review files with various LLM quirks
func TestParseReviewFile_Sanitization(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(*testing.T, *ReviewFile)
	}{
		{
			name: "wrapped in markdown code block",
			content: "```json\n" + `{
  "round": 1,
  "approved": true,
  "score": 9,
  "strengths": ["Good code"],
  "issues": [],
  "suggestions": [],
  "summary": "Test review",
  "required_changes": []
}` + "\n```",
			wantErr: false,
			validate: func(t *testing.T, r *ReviewFile) {
				if r.Score != 9 {
					t.Errorf("Score = %d, want %d", r.Score, 9)
				}
				if r.Summary != "Test review" {
					t.Errorf("Summary = %q, want %q", r.Summary, "Test review")
				}
			},
		},
		{
			name: "smart quotes around values",
			content: `{
  "round": 1,
  "approved": false,
  "score": 5,
  "strengths": ["One strength"],
  "issues": ["One issue"],
  "suggestions": [],
  "summary": "Needs work",
  "required_changes": ["Fix the bug"]
}`,
			wantErr: false,
			validate: func(t *testing.T, r *ReviewFile) {
				if r.Score != 5 {
					t.Errorf("Score = %d, want %d", r.Score, 5)
				}
			},
		},
		{
			name: "prefix and suffix text with code block",
			content: "After reviewing the implementation:\n```json\n" + `{
  "round": 3,
  "approved": true,
  "score": 8,
  "strengths": [],
  "issues": [],
  "suggestions": [],
  "summary": "Approved",
  "required_changes": []
}` + "\n```\nPlease proceed.",
			wantErr: false,
			validate: func(t *testing.T, r *ReviewFile) {
				if r.Round != 3 {
					t.Errorf("Round = %d, want %d", r.Round, 3)
				}
				if !r.Approved {
					t.Error("Approved should be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			reviewPath := filepath.Join(tmpDir, ReviewFileName)
			if err := os.WriteFile(reviewPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write review file: %v", err)
			}

			parsed, err := ParseReviewFile(tmpDir)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseReviewFile() error = %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, parsed)
			}
		})
	}
}

// Test that sanitizeJSONContent function handles edge cases
func TestSanitizeJSONContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already valid JSON",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "left double quotation mark",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "right double quotation mark",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "both curly quotes",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "code block json",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "code block no language",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "text before JSON",
			input:    "Here is some text\n{\"key\": \"value\"}",
			expected: `{"key": "value"}`,
		},
		{
			name:     "text after JSON",
			input:    "{\"key\": \"value\"}\nMore text here",
			expected: `{"key": "value"}`,
		},
		{
			name:     "text before and after",
			input:    "Prefix {\"key\": \"value\"} Suffix",
			expected: `{"key": "value"}`,
		},
		{
			name:     "guillemets",
			input:    `{«key»: «value»}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "single curly quotes",
			input:    `{"key": 'value'}`,
			expected: `{"key": 'value'}`,
		},
		{
			name:     "fullwidth quotation mark",
			input:    `{＂key＂: ＂value＂}`,
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(sanitizeJSONContent([]byte(tt.input)))
			if got != tt.expected {
				t.Errorf("sanitizeJSONContent() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestValidateIncrementJSON_HelpsDebugErrors(t *testing.T) {
	// Test that error messages are helpful for debugging
	tests := []struct {
		name     string
		json     string
		contains string
	}{
		{
			name:     "shows content prefix for non-JSON",
			json:     "This is not JSON, it's plain text describing something",
			contains: "Content starts with",
		},
		{
			name:     "shows expected structure for missing fields",
			json:     `{"round": 1}`,
			contains: "Expected JSON structure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIncrementJSON([]byte(tt.json))
			if err == nil {
				t.Fatal("expected error")
			}

			if !strings.Contains(err.Error(), tt.contains) {
				t.Errorf("error should contain %q for debugging, got: %v", tt.contains, err)
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
	if event.Timestamp != now {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

// Tests for sentinel file search functionality

func TestFindIncrementFile_InWorktreeRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Write increment file in expected location (worktree root)
	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte(`{"round":1,"status":"ready_for_review","summary":"test","files_modified":[],"approach":"test"}`), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	foundPath, err := FindIncrementFile(tmpDir)
	if err != nil {
		t.Fatalf("FindIncrementFile() error = %v", err)
	}
	if foundPath != incrementPath {
		t.Errorf("FindIncrementFile() = %q, want %q", foundPath, incrementPath)
	}
}

func TestFindIncrementFile_InSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory and write file there (simulates Claude cd into subdir)
	subDir := filepath.Join(tmpDir, "myapp")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	incrementPath := filepath.Join(subDir, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte(`{"round":1,"status":"ready_for_review","summary":"test","files_modified":[],"approach":"test"}`), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	foundPath, err := FindIncrementFile(tmpDir)
	if err != nil {
		t.Fatalf("FindIncrementFile() error = %v, want nil", err)
	}
	if foundPath != incrementPath {
		t.Errorf("FindIncrementFile() = %q, want %q", foundPath, incrementPath)
	}
}

func TestFindIncrementFile_InParentDirectory(t *testing.T) {
	// Create a structure like: /tmp/parent/worktree
	// File is in /tmp/parent, but we search from /tmp/parent/worktree
	tmpParent := t.TempDir()
	worktreeDir := filepath.Join(tmpParent, "worktree")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree directory: %v", err)
	}

	// Write increment file in parent directory (simulates Claude working in monorepo root)
	incrementPath := filepath.Join(tmpParent, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte(`{"round":1,"status":"ready_for_review","summary":"test","files_modified":[],"approach":"test"}`), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	foundPath, err := FindIncrementFile(worktreeDir)
	if err != nil {
		t.Fatalf("FindIncrementFile() error = %v, want nil", err)
	}
	if foundPath != incrementPath {
		t.Errorf("FindIncrementFile() = %q, want %q", foundPath, incrementPath)
	}
}

func TestFindIncrementFile_NotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := FindIncrementFile(tmpDir)
	if err == nil {
		t.Error("expected error when file not found")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got: %v", err)
	}
}

func TestFindIncrementFile_SkipsHiddenDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a hidden directory (.git) with the file
	hiddenDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("failed to create hidden directory: %v", err)
	}

	incrementPath := filepath.Join(hiddenDir, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte(`{"round":1}`), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	// Should NOT find the file in hidden directory
	_, err := FindIncrementFile(tmpDir)
	if err == nil {
		t.Error("should not find file in hidden directory")
	}
}

func TestFindReviewFile_InWorktreeRoot(t *testing.T) {
	tmpDir := t.TempDir()

	// Write review file in expected location
	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{"round":1,"approved":true,"score":8}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	foundPath, err := FindReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("FindReviewFile() error = %v", err)
	}
	if foundPath != reviewPath {
		t.Errorf("FindReviewFile() = %q, want %q", foundPath, reviewPath)
	}
}

func TestFindReviewFile_InSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory and write file there
	subDir := filepath.Join(tmpDir, "myapp")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	reviewPath := filepath.Join(subDir, ReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{"round":1,"approved":true,"score":8}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	foundPath, err := FindReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("FindReviewFile() error = %v, want nil", err)
	}
	if foundPath != reviewPath {
		t.Errorf("FindReviewFile() = %q, want %q", foundPath, reviewPath)
	}
}

func TestIncrementFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially should not exist
	if IncrementFileExists(tmpDir) {
		t.Error("IncrementFileExists() = true, want false when file doesn't exist")
	}

	// Write the file
	incrementPath := filepath.Join(tmpDir, IncrementFileName)
	if err := os.WriteFile(incrementPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	// Should now exist
	if !IncrementFileExists(tmpDir) {
		t.Error("IncrementFileExists() = false, want true when file exists")
	}
}

func TestReviewFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Initially should not exist
	if ReviewFileExists(tmpDir) {
		t.Error("ReviewFileExists() = true, want false when file doesn't exist")
	}

	// Write the file
	reviewPath := filepath.Join(tmpDir, ReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Should now exist
	if !ReviewFileExists(tmpDir) {
		t.Error("ReviewFileExists() = false, want true when file exists")
	}
}

func TestParseIncrementFile_FindsInSubdirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a subdirectory and write valid file there
	subDir := filepath.Join(tmpDir, "myapp")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	increment := IncrementFile{
		Round:         1,
		Status:        "ready_for_review",
		Summary:       "Test from subdirectory",
		FilesModified: []string{"test.go"},
		Approach:      "Test approach",
	}

	incrementPath := filepath.Join(subDir, IncrementFileName)
	data, _ := json.Marshal(increment)
	if err := os.WriteFile(incrementPath, data, 0644); err != nil {
		t.Fatalf("failed to write increment file: %v", err)
	}

	// Should find and parse the file from subdirectory
	parsed, err := ParseIncrementFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseIncrementFile() error = %v", err)
	}
	if parsed.Summary != increment.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, increment.Summary)
	}
}

func TestParseReviewFile_FindsInParentDirectory(t *testing.T) {
	// Create a structure like: /tmp/parent/worktree
	tmpParent := t.TempDir()
	worktreeDir := filepath.Join(tmpParent, "worktree")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree directory: %v", err)
	}

	// Write review file in parent directory
	review := ReviewFile{
		Round:    1,
		Approved: true,
		Score:    9,
		Summary:  "Test from parent directory",
	}

	reviewPath := filepath.Join(tmpParent, ReviewFileName)
	data, _ := json.Marshal(review)
	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Should find and parse the file from parent directory
	parsed, err := ParseReviewFile(worktreeDir)
	if err != nil {
		t.Fatalf("ParseReviewFile() error = %v", err)
	}
	if parsed.Summary != review.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, review.Summary)
	}
}

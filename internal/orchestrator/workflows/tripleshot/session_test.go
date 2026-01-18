package tripleshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	task := "Implement a rate limiter"
	config := DefaultConfig()

	session := NewSession(task, config)

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.Task != task {
		t.Errorf("session.Task = %q, want %q", session.Task, task)
	}
	if session.Phase != PhaseWorking {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseWorking)
	}
	if session.Created.IsZero() {
		t.Error("session.Created should not be zero")
	}
}

func TestSession_AllAttemptsComplete(t *testing.T) {
	tests := []struct {
		name     string
		attempts [3]Attempt
		want     bool
	}{
		{
			name: "no attempts complete",
			attempts: [3]Attempt{
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusWorking},
			},
			want: false,
		},
		{
			name: "some attempts complete",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusCompleted},
			},
			want: false,
		},
		{
			name: "all attempts complete",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
			},
			want: true,
		},
		{
			name: "mixed completed and failed",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusCompleted},
			},
			want: true,
		},
		{
			name: "all failed",
			attempts: [3]Attempt{
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusFailed},
			},
			want: true,
		},
		{
			name: "one pending one working one complete",
			attempts: [3]Attempt{
				{Status: AttemptStatusPending},
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusCompleted},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Attempts: tt.attempts,
			}
			got := session.AllAttemptsComplete()
			if got != tt.want {
				t.Errorf("AllAttemptsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_SuccessfulAttemptCount(t *testing.T) {
	tests := []struct {
		name     string
		attempts [3]Attempt
		want     int
	}{
		{
			name: "no successful attempts",
			attempts: [3]Attempt{
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusWorking},
			},
			want: 0,
		},
		{
			name: "one successful",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusWorking},
			},
			want: 1,
		},
		{
			name: "all successful",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
			},
			want: 3,
		},
		{
			name: "two successful",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusCompleted},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Attempts: tt.attempts,
			}
			got := session.SuccessfulAttemptCount()
			if got != tt.want {
				t.Errorf("SuccessfulAttemptCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseCompletionFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write a completion file
	completion := CompletionFile{
		AttemptIndex:  1,
		Status:        "complete",
		Summary:       "Implemented rate limiter using token bucket algorithm",
		FilesModified: []string{"ratelimiter.go", "ratelimiter_test.go"},
		Approach:      "Used token bucket algorithm for its simplicity and effectiveness",
		Notes:         "Added comprehensive tests",
	}

	completionPath := filepath.Join(tmpDir, CompletionFileName)
	data, err := json.MarshalIndent(completion, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal completion: %v", err)
	}
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	// Parse it back
	parsed, err := ParseCompletionFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseCompletionFile() error = %v", err)
	}

	if parsed.AttemptIndex != completion.AttemptIndex {
		t.Errorf("AttemptIndex = %d, want %d", parsed.AttemptIndex, completion.AttemptIndex)
	}
	if parsed.Status != completion.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, completion.Status)
	}
	if parsed.Summary != completion.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, completion.Summary)
	}
	if len(parsed.FilesModified) != len(completion.FilesModified) {
		t.Errorf("len(FilesModified) = %d, want %d", len(parsed.FilesModified), len(completion.FilesModified))
	}
}

func TestParseCompletionFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = ParseCompletionFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestParseCompletionFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write invalid JSON to the completion file
	completionPath := filepath.Join(tmpDir, CompletionFileName)
	if err := os.WriteFile(completionPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid completion file: %v", err)
	}

	_, err = ParseCompletionFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse triple-shot completion JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseEvaluationFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	evaluation := Evaluation{
		WinnerIndex:   1,
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Attempt 2 had the cleanest implementation with comprehensive tests",
		AttemptEvaluation: []AttemptEvaluationItem{
			{AttemptIndex: 0, Score: 7, Strengths: []string{"Good error handling"}, Weaknesses: []string{"Missing tests"}},
			{AttemptIndex: 1, Score: 9, Strengths: []string{"Clean code", "Good tests"}, Weaknesses: []string{}},
			{AttemptIndex: 2, Score: 6, Strengths: []string{"Simple"}, Weaknesses: []string{"No error handling"}},
		},
	}

	evalPath := filepath.Join(tmpDir, EvaluationFileName)
	data, err := json.MarshalIndent(evaluation, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal evaluation: %v", err)
	}
	if err := os.WriteFile(evalPath, data, 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	parsed, err := ParseEvaluationFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseEvaluationFile() error = %v", err)
	}

	if parsed.WinnerIndex != evaluation.WinnerIndex {
		t.Errorf("WinnerIndex = %d, want %d", parsed.WinnerIndex, evaluation.WinnerIndex)
	}
	if parsed.MergeStrategy != evaluation.MergeStrategy {
		t.Errorf("MergeStrategy = %q, want %q", parsed.MergeStrategy, evaluation.MergeStrategy)
	}
	if len(parsed.AttemptEvaluation) != len(evaluation.AttemptEvaluation) {
		t.Errorf("len(AttemptEvaluation) = %d, want %d", len(parsed.AttemptEvaluation), len(evaluation.AttemptEvaluation))
	}
}

func TestParseEvaluationFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write invalid JSON to the evaluation file
	evalPath := filepath.Join(tmpDir, EvaluationFileName)
	if err := os.WriteFile(evalPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid evaluation file: %v", err)
	}

	_, err = ParseEvaluationFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse triple-shot evaluation JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseEvaluationFromOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    *Evaluation
		wantErr bool
	}{
		{
			name: "valid evaluation",
			output: `Some Claude output text here...
<evaluation>
{
  "winner_index": 0,
  "merge_strategy": "select",
  "reasoning": "Attempt 1 is best",
  "attempt_evaluations": []
}
</evaluation>
More text after...`,
			want: &Evaluation{
				WinnerIndex:       0,
				MergeStrategy:     "select",
				Reasoning:         "Attempt 1 is best",
				AttemptEvaluation: []AttemptEvaluationItem{},
			},
			wantErr: false,
		},
		{
			name:    "no evaluation tag",
			output:  "Just some regular output without evaluation tags",
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid JSON inside tag",
			output: `<evaluation>
not valid json
</evaluation>`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "evaluation with whitespace",
			output: `<evaluation>
  {
    "winner_index": 2,
    "merge_strategy": "merge",
    "reasoning": "Combined approach"
  }
</evaluation>`,
			want: &Evaluation{
				WinnerIndex:   2,
				MergeStrategy: "merge",
				Reasoning:     "Combined approach",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEvaluationFromOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEvaluationFromOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil {
				if got.WinnerIndex != tt.want.WinnerIndex {
					t.Errorf("WinnerIndex = %d, want %d", got.WinnerIndex, tt.want.WinnerIndex)
				}
				if got.MergeStrategy != tt.want.MergeStrategy {
					t.Errorf("MergeStrategy = %q, want %q", got.MergeStrategy, tt.want.MergeStrategy)
				}
			}
		})
	}
}

func TestTripleShotManager_MarkAttemptComplete(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", Status: AttemptStatusWorking}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", Status: AttemptStatusWorking}
	session.Attempts[2] = Attempt{InstanceID: "inst-3", Status: AttemptStatusWorking}

	manager := NewManager(session, nil)

	// Mark attempt 1 complete
	manager.MarkAttemptComplete(1)

	if session.Attempts[1].Status != AttemptStatusCompleted {
		t.Errorf("attempt 1 status = %q, want %q", session.Attempts[1].Status, AttemptStatusCompleted)
	}
	if session.Attempts[1].CompletedAt == nil {
		t.Error("attempt 1 CompletedAt should not be nil")
	}

	// Other attempts should be unchanged
	if session.Attempts[0].Status != AttemptStatusWorking {
		t.Errorf("attempt 0 status = %q, want %q", session.Attempts[0].Status, AttemptStatusWorking)
	}
	if session.Attempts[2].Status != AttemptStatusWorking {
		t.Errorf("attempt 2 status = %q, want %q", session.Attempts[2].Status, AttemptStatusWorking)
	}
}

func TestTripleShotManager_MarkAttemptFailed(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	session.Attempts[0] = Attempt{InstanceID: "inst-1", Status: AttemptStatusWorking}
	session.Attempts[1] = Attempt{InstanceID: "inst-2", Status: AttemptStatusWorking}
	session.Attempts[2] = Attempt{InstanceID: "inst-3", Status: AttemptStatusWorking}

	manager := NewManager(session, nil)

	manager.MarkAttemptFailed(0, "timeout exceeded")

	if session.Attempts[0].Status != AttemptStatusFailed {
		t.Errorf("attempt 0 status = %q, want %q", session.Attempts[0].Status, AttemptStatusFailed)
	}
	if session.Attempts[0].CompletedAt == nil {
		t.Error("attempt 0 CompletedAt should not be nil")
	}
}

func TestTripleShotManager_SetPhase(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	manager := NewManager(session, nil)

	var receivedEvent Event
	manager.SetEventCallback(func(event Event) {
		receivedEvent = event
	})

	manager.SetPhase(PhaseEvaluating)

	if session.Phase != PhaseEvaluating {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseEvaluating)
	}
	if receivedEvent.Type != EventPhaseChange {
		t.Errorf("event type = %v, want %v", receivedEvent.Type, EventPhaseChange)
	}
	if receivedEvent.Message != string(PhaseEvaluating) {
		t.Errorf("event message = %q, want %q", receivedEvent.Message, string(PhaseEvaluating))
	}
}

func TestTripleShotManager_SetEvaluation(t *testing.T) {
	session := NewSession("test task", DefaultConfig())
	manager := NewManager(session, nil)

	var receivedEvent Event
	manager.SetEventCallback(func(event Event) {
		receivedEvent = event
	})

	eval := &Evaluation{
		WinnerIndex:   0,
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Best implementation",
	}

	manager.SetEvaluation(eval)

	if session.Evaluation == nil {
		t.Error("session.Evaluation should not be nil")
	}
	if session.Evaluation.WinnerIndex != eval.WinnerIndex {
		t.Errorf("WinnerIndex = %d, want %d", session.Evaluation.WinnerIndex, eval.WinnerIndex)
	}
	if receivedEvent.Type != EventEvaluationReady {
		t.Errorf("event type = %v, want %v", receivedEvent.Type, EventEvaluationReady)
	}
}

func TestTripleShotConfig_Default(t *testing.T) {
	config := DefaultConfig()

	if config.AutoApprove {
		t.Error("default AutoApprove should be false")
	}
}

func TestPhases(t *testing.T) {
	// Verify phase constants are distinct
	phases := []Phase{
		PhaseWorking,
		PhaseEvaluating,
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

func TestAttemptPromptTemplate(t *testing.T) {
	// Verify the template contains expected placeholders
	if len(AttemptPromptTemplate) == 0 {
		t.Error("AttemptPromptTemplate should not be empty")
	}
	if !strings.Contains(AttemptPromptTemplate, "%s") {
		t.Error("template should contain task placeholder")
	}
	if !strings.Contains(AttemptPromptTemplate, "%d") {
		t.Error("template should contain attempt index placeholder")
	}
	if !strings.Contains(AttemptPromptTemplate, CompletionFileName) {
		t.Error("template should reference completion file name")
	}
}

func TestJudgePromptTemplate(t *testing.T) {
	if len(JudgePromptTemplate) == 0 {
		t.Error("JudgePromptTemplate should not be empty")
	}
	if !strings.Contains(JudgePromptTemplate, EvaluationFileName) {
		t.Error("template should reference evaluation file name")
	}
}

func TestAttempt_Timestamps(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)

	attempt := Attempt{
		InstanceID:   "test-id",
		Status:       AttemptStatusCompleted,
		StartedAt:    &now,
		CompletedAt:  &later,
		WorktreePath: "/tmp/test",
		Branch:       "test-branch",
	}

	if attempt.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
	if attempt.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
	if !attempt.CompletedAt.After(*attempt.StartedAt) {
		t.Error("CompletedAt should be after StartedAt")
	}
}

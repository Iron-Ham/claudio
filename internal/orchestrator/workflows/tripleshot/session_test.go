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
		{
			name: "one under review - adversarial mode not complete",
			attempts: [3]Attempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusUnderReview},
				{Status: AttemptStatusCompleted},
			},
			want: false,
		},
		{
			name: "all under review - adversarial mode not complete",
			attempts: [3]Attempt{
				{Status: AttemptStatusUnderReview},
				{Status: AttemptStatusUnderReview},
				{Status: AttemptStatusUnderReview},
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

func TestParseEvaluationFile_FlexibleFields(t *testing.T) {
	tests := []struct {
		name              string
		json              string
		wantStrategy      MergeStrategy
		wantReasoning     string
		wantChangesLen    int
		wantStrengthsLen  int
		wantWeaknessesLen int
		wantFirstChange   string
		wantFirstStrength string
		wantFirstWeakness string
	}{
		{
			name: "merge with suggested_changes as string",
			json: `{
				"winner_index": -1,
				"merge_strategy": "merge",
				"reasoning": "Combined approach is best",
				"attempt_evaluations": [
					{"attempt_index": 0, "score": 7, "strengths": ["Good"], "weaknesses": ["Missing tests"]}
				],
				"suggested_changes": "Combine A's error handling with B's test structure"
			}`,
			wantStrategy:      MergeStrategyMerge,
			wantReasoning:     "Combined approach is best",
			wantChangesLen:    1,
			wantFirstChange:   "Combine A's error handling with B's test structure",
			wantStrengthsLen:  1,
			wantFirstStrength: "Good",
			wantWeaknessesLen: 1,
			wantFirstWeakness: "Missing tests",
		},
		{
			name: "merge with suggested_changes as array",
			json: `{
				"winner_index": -1,
				"merge_strategy": "merge",
				"reasoning": "Merging is ideal",
				"attempt_evaluations": [],
				"suggested_changes": ["Change A", "Change B"]
			}`,
			wantStrategy:    MergeStrategyMerge,
			wantReasoning:   "Merging is ideal",
			wantChangesLen:  2,
			wantFirstChange: "Change A",
		},
		{
			name: "reasoning as array of strings",
			json: `{
				"winner_index": 0,
				"merge_strategy": "select",
				"reasoning": ["First point.", "Second point."],
				"attempt_evaluations": []
			}`,
			wantStrategy:  MergeStrategySelect,
			wantReasoning: "First point.\nSecond point.",
		},
		{
			name: "strengths and weaknesses as strings",
			json: `{
				"winner_index": 0,
				"merge_strategy": "select",
				"reasoning": "Best one",
				"attempt_evaluations": [
					{"attempt_index": 0, "score": 8, "strengths": "Clean implementation", "weaknesses": "No tests"}
				]
			}`,
			wantStrategy:      MergeStrategySelect,
			wantStrengthsLen:  1,
			wantFirstStrength: "Clean implementation",
			wantWeaknessesLen: 1,
			wantFirstWeakness: "No tests",
		},
		{
			name: "combine with suggested_changes as array of objects",
			json: `{
				"winner_index": -1,
				"merge_strategy": "combine",
				"reasoning": "Cherry-pick from multiple attempts",
				"attempt_evaluations": [],
				"suggested_changes": [
					{"description": "Use Attempt 1 as the base branch", "source": "attempt_1"},
					{"description": "Cherry-pick ContentEquatable from Attempt 3", "source": "attempt_3"}
				]
			}`,
			wantStrategy:    MergeStrategyCombine,
			wantReasoning:   "Cherry-pick from multiple attempts",
			wantChangesLen:  2,
			wantFirstChange: "Use Attempt 1 as the base branch",
		},
		{
			name: "suggested_changes as objects with text key",
			json: `{
				"winner_index": -1,
				"merge_strategy": "merge",
				"reasoning": "Merged",
				"attempt_evaluations": [],
				"suggested_changes": [
					{"text": "Apply error handling from Attempt 2"}
				]
			}`,
			wantStrategy:    MergeStrategyMerge,
			wantReasoning:   "Merged",
			wantChangesLen:  1,
			wantFirstChange: "Apply error handling from Attempt 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			evalPath := filepath.Join(tmpDir, EvaluationFileName)
			if err := os.WriteFile(evalPath, []byte(tt.json), 0644); err != nil {
				t.Fatalf("failed to write evaluation file: %v", err)
			}

			parsed, err := ParseEvaluationFile(tmpDir)
			if err != nil {
				t.Fatalf("ParseEvaluationFile() error = %v", err)
			}

			if parsed.MergeStrategy != tt.wantStrategy {
				t.Errorf("MergeStrategy = %q, want %q", parsed.MergeStrategy, tt.wantStrategy)
			}
			if tt.wantReasoning != "" && parsed.Reasoning.String() != tt.wantReasoning {
				t.Errorf("Reasoning = %q, want %q", parsed.Reasoning, tt.wantReasoning)
			}
			if tt.wantChangesLen > 0 {
				if len(parsed.SuggestedChanges) != tt.wantChangesLen {
					t.Errorf("len(SuggestedChanges) = %d, want %d", len(parsed.SuggestedChanges), tt.wantChangesLen)
				}
				if len(parsed.SuggestedChanges) > 0 && parsed.SuggestedChanges[0] != tt.wantFirstChange {
					t.Errorf("SuggestedChanges[0] = %q, want %q", parsed.SuggestedChanges[0], tt.wantFirstChange)
				}
			}
			if tt.wantStrengthsLen > 0 && len(parsed.AttemptEvaluation) > 0 {
				if len(parsed.AttemptEvaluation[0].Strengths) != tt.wantStrengthsLen {
					t.Errorf("Strengths len = %d, want %d", len(parsed.AttemptEvaluation[0].Strengths), tt.wantStrengthsLen)
				}
				if len(parsed.AttemptEvaluation[0].Strengths) > 0 && parsed.AttemptEvaluation[0].Strengths[0] != tt.wantFirstStrength {
					t.Errorf("Strengths[0] = %q, want %q", parsed.AttemptEvaluation[0].Strengths[0], tt.wantFirstStrength)
				}
			}
			if tt.wantWeaknessesLen > 0 && len(parsed.AttemptEvaluation) > 0 {
				if len(parsed.AttemptEvaluation[0].Weaknesses) != tt.wantWeaknessesLen {
					t.Errorf("Weaknesses len = %d, want %d", len(parsed.AttemptEvaluation[0].Weaknesses), tt.wantWeaknessesLen)
				}
				if len(parsed.AttemptEvaluation[0].Weaknesses) > 0 && parsed.AttemptEvaluation[0].Weaknesses[0] != tt.wantFirstWeakness {
					t.Errorf("Weaknesses[0] = %q, want %q", parsed.AttemptEvaluation[0].Weaknesses[0], tt.wantFirstWeakness)
				}
			}
		})
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

func TestAttemptPromptTemplate_CompletionProtocol(t *testing.T) {
	// Verify emphatic completion protocol wording
	expectedParts := []string{
		"FINAL MANDATORY STEP",
		"FINAL MANDATORY ACTION",
		"orchestrator is BLOCKED waiting",
		"DO NOT",
		"wait for user prompting",
		"Write this file AUTOMATICALLY",
		"REMEMBER",
		"Your attempt is NOT complete until you write this file",
	}

	for _, part := range expectedParts {
		if !strings.Contains(AttemptPromptTemplate, part) {
			t.Errorf("Completion protocol missing %q", part)
		}
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

func TestFindCompletionFile_InWorktreeRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create completion file in the worktree root
	completionPath := filepath.Join(tmpDir, CompletionFileName)
	if err := os.WriteFile(completionPath, []byte(`{"status":"complete"}`), 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	foundPath, err := FindCompletionFile(tmpDir)
	if err != nil {
		t.Fatalf("FindCompletionFile() error = %v", err)
	}
	if foundPath != completionPath {
		t.Errorf("FindCompletionFile() = %q, want %q", foundPath, completionPath)
	}
}

func TestFindCompletionFile_InSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create a subdirectory (simulating a monorepo with nested project)
	subDir := filepath.Join(tmpDir, "mail-ios")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create completion file in the subdirectory (not in worktree root)
	completionPath := filepath.Join(subDir, CompletionFileName)
	if err := os.WriteFile(completionPath, []byte(`{"status":"complete"}`), 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	foundPath, err := FindCompletionFile(tmpDir)
	if err != nil {
		t.Fatalf("FindCompletionFile() error = %v", err)
	}
	if foundPath != completionPath {
		t.Errorf("FindCompletionFile() = %q, want %q", foundPath, completionPath)
	}
}

func TestFindCompletionFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = FindCompletionFile(tmpDir)
	if err == nil {
		t.Error("expected error when completion file doesn't exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestFindCompletionFile_SkipsHiddenDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .git directory with a completion file (should be ignored)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}
	gitCompletionPath := filepath.Join(gitDir, CompletionFileName)
	if err := os.WriteFile(gitCompletionPath, []byte(`{"status":"complete"}`), 0644); err != nil {
		t.Fatalf("failed to write completion file in .git: %v", err)
	}

	// Should not find the file in .git
	_, err = FindCompletionFile(tmpDir)
	if err == nil {
		t.Error("expected error when completion file is only in hidden directory")
	}
}

func TestFindCompletionFile_PrefersWorktreeRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create completion file in worktree root
	rootCompletionPath := filepath.Join(tmpDir, CompletionFileName)
	if err := os.WriteFile(rootCompletionPath, []byte(`{"status":"complete","summary":"root"}`), 0644); err != nil {
		t.Fatalf("failed to write root completion file: %v", err)
	}

	// Also create completion file in subdirectory
	subDir := filepath.Join(tmpDir, "subproject")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	subCompletionPath := filepath.Join(subDir, CompletionFileName)
	if err := os.WriteFile(subCompletionPath, []byte(`{"status":"complete","summary":"sub"}`), 0644); err != nil {
		t.Fatalf("failed to write sub completion file: %v", err)
	}

	// Should find the root completion file (preferred)
	foundPath, err := FindCompletionFile(tmpDir)
	if err != nil {
		t.Fatalf("FindCompletionFile() error = %v", err)
	}
	if foundPath != rootCompletionPath {
		t.Errorf("FindCompletionFile() = %q, want %q (should prefer root)", foundPath, rootCompletionPath)
	}
}

func TestCompletionFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Initially should not exist
	if CompletionFileExists(tmpDir) {
		t.Error("CompletionFileExists() = true, want false (file doesn't exist)")
	}

	// Create completion file
	completionPath := filepath.Join(tmpDir, CompletionFileName)
	if err := os.WriteFile(completionPath, []byte(`{"status":"complete"}`), 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	// Now should exist
	if !CompletionFileExists(tmpDir) {
		t.Error("CompletionFileExists() = false, want true")
	}
}

func TestParseCompletionFile_FromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Write completion file in subdirectory
	completion := CompletionFile{
		AttemptIndex:  0,
		Status:        "complete",
		Summary:       "Found in subdirectory",
		FilesModified: []string{"file.go"},
		Approach:      "Test approach",
	}
	completionPath := filepath.Join(subDir, CompletionFileName)
	data, err := json.MarshalIndent(completion, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal completion: %v", err)
	}
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	// Parse from worktree root - should find it in subdirectory
	parsed, err := ParseCompletionFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseCompletionFile() error = %v", err)
	}
	if parsed.Summary != completion.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, completion.Summary)
	}
}

func TestFindEvaluationFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = FindEvaluationFile(tmpDir)
	if err == nil {
		t.Error("expected error when evaluation file doesn't exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestFindEvaluationFile_SkipsHiddenDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .git directory with an evaluation file (should be ignored)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}
	gitEvalPath := filepath.Join(gitDir, EvaluationFileName)
	if err := os.WriteFile(gitEvalPath, []byte(`{"winner_index":0}`), 0644); err != nil {
		t.Fatalf("failed to write evaluation file in .git: %v", err)
	}

	// Should not find the file in .git
	_, err = FindEvaluationFile(tmpDir)
	if err == nil {
		t.Error("expected error when evaluation file is only in hidden directory")
	}
}

func TestFindEvaluationFile_PrefersWorktreeRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create evaluation file in worktree root
	rootEvalPath := filepath.Join(tmpDir, EvaluationFileName)
	if err := os.WriteFile(rootEvalPath, []byte(`{"winner_index":0,"reasoning":"root"}`), 0644); err != nil {
		t.Fatalf("failed to write root evaluation file: %v", err)
	}

	// Also create evaluation file in subdirectory
	subDir := filepath.Join(tmpDir, "subproject")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	subEvalPath := filepath.Join(subDir, EvaluationFileName)
	if err := os.WriteFile(subEvalPath, []byte(`{"winner_index":1,"reasoning":"sub"}`), 0644); err != nil {
		t.Fatalf("failed to write sub evaluation file: %v", err)
	}

	// Should find the root evaluation file (preferred)
	foundPath, err := FindEvaluationFile(tmpDir)
	if err != nil {
		t.Fatalf("FindEvaluationFile() error = %v", err)
	}
	if foundPath != rootEvalPath {
		t.Errorf("FindEvaluationFile() = %q, want %q (should prefer root)", foundPath, rootEvalPath)
	}
}

func TestFindEvaluationFile_InWorktreeRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create evaluation file in the worktree root
	evalPath := filepath.Join(tmpDir, EvaluationFileName)
	if err := os.WriteFile(evalPath, []byte(`{"winner_index":0}`), 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	foundPath, err := FindEvaluationFile(tmpDir)
	if err != nil {
		t.Fatalf("FindEvaluationFile() error = %v", err)
	}
	if foundPath != evalPath {
		t.Errorf("FindEvaluationFile() = %q, want %q", foundPath, evalPath)
	}
}

func TestFindEvaluationFile_InSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create evaluation file in subdirectory
	evalPath := filepath.Join(subDir, EvaluationFileName)
	if err := os.WriteFile(evalPath, []byte(`{"winner_index":1}`), 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	foundPath, err := FindEvaluationFile(tmpDir)
	if err != nil {
		t.Fatalf("FindEvaluationFile() error = %v", err)
	}
	if foundPath != evalPath {
		t.Errorf("FindEvaluationFile() = %q, want %q", foundPath, evalPath)
	}
}

func TestEvaluationFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Initially should not exist
	if EvaluationFileExists(tmpDir) {
		t.Error("EvaluationFileExists() = true, want false")
	}

	// Create evaluation file
	evalPath := filepath.Join(tmpDir, EvaluationFileName)
	if err := os.WriteFile(evalPath, []byte(`{"winner_index":0}`), 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	// Now should exist
	if !EvaluationFileExists(tmpDir) {
		t.Error("EvaluationFileExists() = false, want true")
	}
}

func TestParseEvaluationFile_FromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Write evaluation file in subdirectory
	evaluation := Evaluation{
		WinnerIndex:   2,
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Found in subdirectory",
	}
	evalPath := filepath.Join(subDir, EvaluationFileName)
	data, err := json.MarshalIndent(evaluation, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal evaluation: %v", err)
	}
	if err := os.WriteFile(evalPath, data, 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	// Parse from worktree root - should find it in subdirectory
	parsed, err := ParseEvaluationFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseEvaluationFile() error = %v", err)
	}
	if parsed.WinnerIndex != evaluation.WinnerIndex {
		t.Errorf("WinnerIndex = %d, want %d", parsed.WinnerIndex, evaluation.WinnerIndex)
	}
	if parsed.Reasoning != evaluation.Reasoning {
		t.Errorf("Reasoning = %q, want %q", parsed.Reasoning, evaluation.Reasoning)
	}
}

// Adversarial review file tests

func TestFindAdversarialReviewFile_InWorktreeRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create review file in the worktree root
	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{"approved":true}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	foundPath, err := FindAdversarialReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("FindAdversarialReviewFile() error = %v", err)
	}
	if foundPath != reviewPath {
		t.Errorf("FindAdversarialReviewFile() = %q, want %q", foundPath, reviewPath)
	}
}

func TestFindAdversarialReviewFile_InSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create review file in the subdirectory
	reviewPath := filepath.Join(subDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{"approved":true}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	foundPath, err := FindAdversarialReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("FindAdversarialReviewFile() error = %v", err)
	}
	if foundPath != reviewPath {
		t.Errorf("FindAdversarialReviewFile() = %q, want %q", foundPath, reviewPath)
	}
}

func TestFindAdversarialReviewFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = FindAdversarialReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when review file doesn't exist")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestFindAdversarialReviewFile_SkipsHiddenDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create .git directory with a review file (should be ignored)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git directory: %v", err)
	}
	gitReviewPath := filepath.Join(gitDir, AdversarialReviewFileName)
	if err := os.WriteFile(gitReviewPath, []byte(`{"approved":true}`), 0644); err != nil {
		t.Fatalf("failed to write review file in .git: %v", err)
	}

	// Should not find the file in .git
	_, err = FindAdversarialReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when review file is only in hidden directory")
	}
}

func TestFindAdversarialReviewFile_PrefersWorktreeRoot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create review file in worktree root
	rootReviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	if err := os.WriteFile(rootReviewPath, []byte(`{"approved":true,"score":9}`), 0644); err != nil {
		t.Fatalf("failed to write root review file: %v", err)
	}

	// Also create review file in subdirectory
	subDir := filepath.Join(tmpDir, "subproject")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	subReviewPath := filepath.Join(subDir, AdversarialReviewFileName)
	if err := os.WriteFile(subReviewPath, []byte(`{"approved":false,"score":5}`), 0644); err != nil {
		t.Fatalf("failed to write sub review file: %v", err)
	}

	// Should find the root review file (preferred)
	foundPath, err := FindAdversarialReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("FindAdversarialReviewFile() error = %v", err)
	}
	if foundPath != rootReviewPath {
		t.Errorf("FindAdversarialReviewFile() = %q, want %q (should prefer root)", foundPath, rootReviewPath)
	}
}

func TestAdversarialReviewFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Initially should not exist
	if AdversarialReviewFileExists(tmpDir) {
		t.Error("AdversarialReviewFileExists() = true, want false")
	}

	// Create review file
	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{"approved":true}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Now should exist
	if !AdversarialReviewFileExists(tmpDir) {
		t.Error("AdversarialReviewFileExists() = false, want true")
	}
}

func TestAdversarialReviewFileExists_InSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create review file in subdirectory
	reviewPath := filepath.Join(subDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, []byte(`{"approved":true}`), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Should find it in subdirectory
	if !AdversarialReviewFileExists(tmpDir) {
		t.Error("AdversarialReviewFileExists() = false, want true (should find in subdirectory)")
	}
}

func TestParseAdversarialReviewFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	review := AdversarialReviewFile{
		AttemptIndex: 0,
		Round:        1,
		Approved:     true,
		Score:        9,
		Summary:      "Excellent implementation",
		Strengths:    []string{"Clean code", "Good tests"},
		Issues:       []string{},
	}

	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review: %v", err)
	}
	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	parsed, err := ParseAdversarialReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseAdversarialReviewFile() error = %v", err)
	}

	if parsed.AttemptIndex != review.AttemptIndex {
		t.Errorf("AttemptIndex = %d, want %d", parsed.AttemptIndex, review.AttemptIndex)
	}
	if parsed.Approved != review.Approved {
		t.Errorf("Approved = %v, want %v", parsed.Approved, review.Approved)
	}
	if parsed.Score != review.Score {
		t.Errorf("Score = %d, want %d", parsed.Score, review.Score)
	}
	if parsed.Summary != review.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, review.Summary)
	}
}

func TestParseAdversarialReviewFile_FromSubdirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	review := AdversarialReviewFile{
		AttemptIndex: 1,
		Round:        2,
		Approved:     false,
		Score:        6,
		Summary:      "Found in subdirectory",
		Issues:       []string{"Missing tests"},
	}
	reviewPath := filepath.Join(subDir, AdversarialReviewFileName)
	data, err := json.MarshalIndent(review, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal review: %v", err)
	}
	if err := os.WriteFile(reviewPath, data, 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	// Parse from worktree root - should find it in subdirectory
	parsed, err := ParseAdversarialReviewFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseAdversarialReviewFile() error = %v", err)
	}
	if parsed.Summary != review.Summary {
		t.Errorf("Summary = %q, want %q", parsed.Summary, review.Summary)
	}
}

func TestParseAdversarialReviewFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write invalid JSON
	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	if err := os.WriteFile(reviewPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid review file: %v", err)
	}

	_, err = ParseAdversarialReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse tripleshot adversarial review JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseAdversarialReviewFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = ParseAdversarialReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestDefaultConfig_MinPassingScore(t *testing.T) {
	config := DefaultConfig()

	if config.MinPassingScore != 8 {
		t.Errorf("default MinPassingScore = %d, want 8", config.MinPassingScore)
	}
}

func TestAdversarialReviewerPromptTemplate(t *testing.T) {
	// Verify the template contains expected placeholders and protocol
	if len(AdversarialReviewerPromptTemplate) == 0 {
		t.Error("AdversarialReviewerPromptTemplate should not be empty")
	}
	if !strings.Contains(AdversarialReviewerPromptTemplate, "%s") {
		t.Error("template should contain task placeholder")
	}
	if !strings.Contains(AdversarialReviewerPromptTemplate, "%d") {
		t.Error("template should contain numeric placeholders")
	}
	if !strings.Contains(AdversarialReviewerPromptTemplate, AdversarialReviewFileName) {
		t.Error("template should reference review file name")
	}
	if !strings.Contains(AdversarialReviewerPromptTemplate, "FINAL MANDATORY") {
		t.Error("template should have emphatic completion protocol")
	}
}

func TestFormatAdversarialReviewerPrompt(t *testing.T) {
	completion := &CompletionFile{
		AttemptIndex:  0,
		Status:        "complete",
		Summary:       "Implemented rate limiter",
		Approach:      "Token bucket algorithm",
		FilesModified: []string{"limiter.go", "limiter_test.go"},
		Notes:         "Added tests",
	}

	prompt := FormatAdversarialReviewerPrompt("Implement rate limiter", 0, 1, completion, 8)

	// Verify key elements are present
	if !strings.Contains(prompt, "Implement rate limiter") {
		t.Error("prompt should contain the task")
	}
	if !strings.Contains(prompt, "Attempt 1") {
		t.Error("prompt should contain 1-indexed attempt number")
	}
	if !strings.Contains(prompt, "score >= 8") {
		t.Error("prompt should contain passing score threshold")
	}
	if !strings.Contains(prompt, "Token bucket algorithm") {
		t.Error("prompt should contain the approach")
	}
	if !strings.Contains(prompt, "Added tests") {
		t.Error("prompt should contain the notes")
	}
}

func TestFormatAdversarialReviewerPrompt_EmptyNotes(t *testing.T) {
	completion := &CompletionFile{
		AttemptIndex:  1,
		Status:        "complete",
		Summary:       "Test summary",
		Approach:      "Test approach",
		FilesModified: []string{"file.go"},
		Notes:         "", // Empty notes
	}

	prompt := FormatAdversarialReviewerPrompt("Test task", 1, 2, completion, 9)

	// Should still work with empty notes
	if !strings.Contains(prompt, "Attempt 2") {
		t.Error("prompt should contain 1-indexed attempt number")
	}
	if !strings.Contains(prompt, "score >= 9") {
		t.Error("prompt should contain configured min score")
	}
}

func TestAdversarialReviewFile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		review  AdversarialReviewFile
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid review",
			review: AdversarialReviewFile{
				AttemptIndex: 0,
				Round:        1,
				Score:        8,
			},
			wantErr: false,
		},
		{
			name: "invalid attempt index negative",
			review: AdversarialReviewFile{
				AttemptIndex: -1,
				Round:        1,
				Score:        8,
			},
			wantErr: true,
			errMsg:  "attempt_index must be 0-2",
		},
		{
			name: "invalid attempt index too high",
			review: AdversarialReviewFile{
				AttemptIndex: 3,
				Round:        1,
				Score:        8,
			},
			wantErr: true,
			errMsg:  "attempt_index must be 0-2",
		},
		{
			name: "invalid round zero",
			review: AdversarialReviewFile{
				AttemptIndex: 0,
				Round:        0,
				Score:        8,
			},
			wantErr: true,
			errMsg:  "round must be >= 1",
		},
		{
			name: "invalid score too low",
			review: AdversarialReviewFile{
				AttemptIndex: 0,
				Round:        1,
				Score:        0,
			},
			wantErr: true,
			errMsg:  "score must be 1-10",
		},
		{
			name: "invalid score too high",
			review: AdversarialReviewFile{
				AttemptIndex: 0,
				Round:        1,
				Score:        11,
			},
			wantErr: true,
			errMsg:  "score must be 1-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.review.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error = %v, should contain %q", err, tt.errMsg)
			}
		})
	}
}

func TestParseAdversarialReviewFile_InvalidValues(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write review file with invalid score
	reviewPath := filepath.Join(tmpDir, AdversarialReviewFileName)
	invalidJSON := `{"attempt_index": 0, "round": 1, "score": 15, "approved": true}`
	if err := os.WriteFile(reviewPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("failed to write review file: %v", err)
	}

	_, err = ParseAdversarialReviewFile(tmpDir)
	if err == nil {
		t.Error("expected error for invalid score value")
	}
	if !strings.Contains(err.Error(), "invalid adversarial review") {
		t.Errorf("error should mention invalid review, got: %v", err)
	}
}

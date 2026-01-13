package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTripleShotSession(t *testing.T) {
	task := "Implement a rate limiter"
	config := DefaultTripleShotConfig()

	session := NewTripleShotSession(task, config)

	if session.ID == "" {
		t.Error("session ID should not be empty")
	}
	if session.Task != task {
		t.Errorf("session.Task = %q, want %q", session.Task, task)
	}
	if session.Phase != PhaseTripleShotWorking {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseTripleShotWorking)
	}
	if session.Created.IsZero() {
		t.Error("session.Created should not be zero")
	}
}

func TestTripleShotSession_AllAttemptsComplete(t *testing.T) {
	tests := []struct {
		name     string
		attempts [3]TripleShotAttempt
		want     bool
	}{
		{
			name: "no attempts complete",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusWorking},
			},
			want: false,
		},
		{
			name: "some attempts complete",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusCompleted},
			},
			want: false,
		},
		{
			name: "all attempts complete",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
			},
			want: true,
		},
		{
			name: "mixed completed and failed",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusCompleted},
			},
			want: true,
		},
		{
			name: "all failed",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusFailed},
			},
			want: true,
		},
		{
			name: "one pending one working one complete",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusPending},
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusCompleted},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &TripleShotSession{
				Attempts: tt.attempts,
			}
			got := session.AllAttemptsComplete()
			if got != tt.want {
				t.Errorf("AllAttemptsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTripleShotSession_SuccessfulAttemptCount(t *testing.T) {
	tests := []struct {
		name     string
		attempts [3]TripleShotAttempt
		want     int
	}{
		{
			name: "no successful attempts",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusWorking},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusWorking},
			},
			want: 0,
		},
		{
			name: "one successful",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusWorking},
			},
			want: 1,
		},
		{
			name: "all successful",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusCompleted},
			},
			want: 3,
		},
		{
			name: "two successful",
			attempts: [3]TripleShotAttempt{
				{Status: AttemptStatusCompleted},
				{Status: AttemptStatusFailed},
				{Status: AttemptStatusCompleted},
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &TripleShotSession{
				Attempts: tt.attempts,
			}
			got := session.SuccessfulAttemptCount()
			if got != tt.want {
				t.Errorf("SuccessfulAttemptCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseTripleShotCompletionFile(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write a completion file
	completion := TripleShotCompletionFile{
		AttemptIndex:  1,
		Status:        "complete",
		Summary:       "Implemented rate limiter using token bucket algorithm",
		FilesModified: []string{"ratelimiter.go", "ratelimiter_test.go"},
		Approach:      "Used token bucket algorithm for its simplicity and effectiveness",
		Notes:         "Added comprehensive tests",
	}

	completionPath := filepath.Join(tmpDir, TripleShotCompletionFileName)
	data, err := json.MarshalIndent(completion, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal completion: %v", err)
	}
	if err := os.WriteFile(completionPath, data, 0644); err != nil {
		t.Fatalf("failed to write completion file: %v", err)
	}

	// Parse it back
	parsed, err := ParseTripleShotCompletionFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseTripleShotCompletionFile() error = %v", err)
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

func TestParseTripleShotCompletionFile_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	_, err = ParseTripleShotCompletionFile(tmpDir)
	if err == nil {
		t.Error("expected error when file doesn't exist")
	}
}

func TestParseTripleShotCompletionFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write invalid JSON to the completion file
	completionPath := filepath.Join(tmpDir, TripleShotCompletionFileName)
	if err := os.WriteFile(completionPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid completion file: %v", err)
	}

	_, err = ParseTripleShotCompletionFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse triple-shot completion JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseTripleShotEvaluationFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	evaluation := TripleShotEvaluation{
		WinnerIndex:   1,
		MergeStrategy: MergeStrategySelect,
		Reasoning:     "Attempt 2 had the cleanest implementation with comprehensive tests",
		AttemptEvaluation: []AttemptEvaluationItem{
			{AttemptIndex: 0, Score: 7, Strengths: []string{"Good error handling"}, Weaknesses: []string{"Missing tests"}},
			{AttemptIndex: 1, Score: 9, Strengths: []string{"Clean code", "Good tests"}, Weaknesses: []string{}},
			{AttemptIndex: 2, Score: 6, Strengths: []string{"Simple"}, Weaknesses: []string{"No error handling"}},
		},
	}

	evalPath := filepath.Join(tmpDir, TripleShotEvaluationFileName)
	data, err := json.MarshalIndent(evaluation, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal evaluation: %v", err)
	}
	if err := os.WriteFile(evalPath, data, 0644); err != nil {
		t.Fatalf("failed to write evaluation file: %v", err)
	}

	parsed, err := ParseTripleShotEvaluationFile(tmpDir)
	if err != nil {
		t.Fatalf("ParseTripleShotEvaluationFile() error = %v", err)
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

func TestParseTripleShotEvaluationFile_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tripleshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	// Write invalid JSON to the evaluation file
	evalPath := filepath.Join(tmpDir, TripleShotEvaluationFileName)
	if err := os.WriteFile(evalPath, []byte("{ invalid json }"), 0644); err != nil {
		t.Fatalf("failed to write invalid evaluation file: %v", err)
	}

	_, err = ParseTripleShotEvaluationFile(tmpDir)
	if err == nil {
		t.Error("expected error when file contains invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse triple-shot evaluation JSON") {
		t.Errorf("error message should mention JSON parsing failure, got: %v", err)
	}
}

func TestParseTripleShotEvaluationFromOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		want    *TripleShotEvaluation
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
			want: &TripleShotEvaluation{
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
			want: &TripleShotEvaluation{
				WinnerIndex:   2,
				MergeStrategy: "merge",
				Reasoning:     "Combined approach",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTripleShotEvaluationFromOutput(tt.output)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTripleShotEvaluationFromOutput() error = %v, wantErr %v", err, tt.wantErr)
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
	session := NewTripleShotSession("test task", DefaultTripleShotConfig())
	session.Attempts[0] = TripleShotAttempt{InstanceID: "inst-1", Status: AttemptStatusWorking}
	session.Attempts[1] = TripleShotAttempt{InstanceID: "inst-2", Status: AttemptStatusWorking}
	session.Attempts[2] = TripleShotAttempt{InstanceID: "inst-3", Status: AttemptStatusWorking}

	manager := NewTripleShotManager(nil, nil, session, nil)

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
	session := NewTripleShotSession("test task", DefaultTripleShotConfig())
	session.Attempts[0] = TripleShotAttempt{InstanceID: "inst-1", Status: AttemptStatusWorking}
	session.Attempts[1] = TripleShotAttempt{InstanceID: "inst-2", Status: AttemptStatusWorking}
	session.Attempts[2] = TripleShotAttempt{InstanceID: "inst-3", Status: AttemptStatusWorking}

	manager := NewTripleShotManager(nil, nil, session, nil)

	manager.MarkAttemptFailed(0, "timeout exceeded")

	if session.Attempts[0].Status != AttemptStatusFailed {
		t.Errorf("attempt 0 status = %q, want %q", session.Attempts[0].Status, AttemptStatusFailed)
	}
	if session.Attempts[0].CompletedAt == nil {
		t.Error("attempt 0 CompletedAt should not be nil")
	}
}

func TestTripleShotManager_SetPhase(t *testing.T) {
	session := NewTripleShotSession("test task", DefaultTripleShotConfig())
	manager := NewTripleShotManager(nil, nil, session, nil)

	var receivedEvent TripleShotEvent
	manager.SetEventCallback(func(event TripleShotEvent) {
		receivedEvent = event
	})

	manager.SetPhase(PhaseTripleShotEvaluating)

	if session.Phase != PhaseTripleShotEvaluating {
		t.Errorf("session.Phase = %v, want %v", session.Phase, PhaseTripleShotEvaluating)
	}
	if receivedEvent.Type != EventTripleShotPhaseChange {
		t.Errorf("event type = %v, want %v", receivedEvent.Type, EventTripleShotPhaseChange)
	}
	if receivedEvent.Message != string(PhaseTripleShotEvaluating) {
		t.Errorf("event message = %q, want %q", receivedEvent.Message, string(PhaseTripleShotEvaluating))
	}
}

func TestTripleShotManager_SetEvaluation(t *testing.T) {
	session := NewTripleShotSession("test task", DefaultTripleShotConfig())
	manager := NewTripleShotManager(nil, nil, session, nil)

	var receivedEvent TripleShotEvent
	manager.SetEventCallback(func(event TripleShotEvent) {
		receivedEvent = event
	})

	eval := &TripleShotEvaluation{
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
	if receivedEvent.Type != EventTripleShotEvaluationReady {
		t.Errorf("event type = %v, want %v", receivedEvent.Type, EventTripleShotEvaluationReady)
	}
}

func TestTripleShotConfig_Default(t *testing.T) {
	config := DefaultTripleShotConfig()

	if config.AutoApprove {
		t.Error("default AutoApprove should be false")
	}
}

func TestTripleShotPhases(t *testing.T) {
	// Verify phase constants are distinct
	phases := []TripleShotPhase{
		PhaseTripleShotWorking,
		PhaseTripleShotEvaluating,
		PhaseTripleShotComplete,
		PhaseTripleShotFailed,
	}

	seen := make(map[TripleShotPhase]bool)
	for _, phase := range phases {
		if seen[phase] {
			t.Errorf("duplicate phase constant: %v", phase)
		}
		seen[phase] = true
	}
}

func TestTripleShotAttemptPromptTemplate(t *testing.T) {
	// Verify the template contains expected placeholders
	if len(TripleShotAttemptPromptTemplate) == 0 {
		t.Error("TripleShotAttemptPromptTemplate should not be empty")
	}
	if !strings.Contains(TripleShotAttemptPromptTemplate, "%s") {
		t.Error("template should contain task placeholder")
	}
	if !strings.Contains(TripleShotAttemptPromptTemplate, "%d") {
		t.Error("template should contain attempt index placeholder")
	}
	if !strings.Contains(TripleShotAttemptPromptTemplate, TripleShotCompletionFileName) {
		t.Error("template should reference completion file name")
	}
}

func TestTripleShotJudgePromptTemplate(t *testing.T) {
	if len(TripleShotJudgePromptTemplate) == 0 {
		t.Error("TripleShotJudgePromptTemplate should not be empty")
	}
	if !strings.Contains(TripleShotJudgePromptTemplate, TripleShotEvaluationFileName) {
		t.Error("template should reference evaluation file name")
	}
}

func TestTripleShotAttempt_Timestamps(t *testing.T) {
	now := time.Now()
	later := now.Add(5 * time.Minute)

	attempt := TripleShotAttempt{
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

// TestTripleShotCoordinator_CheckAttemptCompletion tests the CheckAttemptCompletion method
func TestTripleShotCoordinator_CheckAttemptCompletion(t *testing.T) {
	tests := []struct {
		name         string
		attemptIndex int
		setupFunc    func(worktree string)
		wantComplete bool
		wantErr      bool
		errSubstring string
	}{
		{
			name:         "invalid index negative",
			attemptIndex: -1,
			wantComplete: false,
			wantErr:      true,
			errSubstring: "invalid attempt index",
		},
		{
			name:         "invalid index too large",
			attemptIndex: 3,
			wantComplete: false,
			wantErr:      true,
			errSubstring: "invalid attempt index",
		},
		{
			name:         "valid index no completion file",
			attemptIndex: 0,
			wantComplete: false,
			wantErr:      false,
		},
		{
			name:         "valid index with completion file",
			attemptIndex: 1,
			setupFunc: func(worktree string) {
				completion := TripleShotCompletionFile{
					Status:  "complete",
					Summary: "Test completed",
				}
				data, _ := json.Marshal(completion)
				_ = os.WriteFile(filepath.Join(worktree, TripleShotCompletionFileName), data, 0644)
			},
			wantComplete: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory for worktree
			tmpDir := t.TempDir()

			// Create coordinator with minimal setup
			baseSession := &Session{ID: "test-session"}
			tripleSession := NewTripleShotSession("test task", DefaultTripleShotConfig())

			// Set up attempts with worktree paths
			for i := range 3 {
				tripleSession.Attempts[i] = TripleShotAttempt{
					InstanceID:   "inst-" + string(rune('0'+i)),
					WorktreePath: filepath.Join(tmpDir, "attempt-"+string(rune('0'+i))),
					Status:       AttemptStatusWorking,
				}
				_ = os.MkdirAll(tripleSession.Attempts[i].WorktreePath, 0755)
			}

			// Run setup if provided
			if tt.setupFunc != nil && tt.attemptIndex >= 0 && tt.attemptIndex < 3 {
				tt.setupFunc(tripleSession.Attempts[tt.attemptIndex].WorktreePath)
			}

			coordinator := &TripleShotCoordinator{
				manager:     NewTripleShotManager(nil, baseSession, tripleSession, nil),
				baseSession: baseSession,
			}

			complete, err := coordinator.CheckAttemptCompletion(tt.attemptIndex)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.errSubstring)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if complete != tt.wantComplete {
				t.Errorf("complete = %v, want %v", complete, tt.wantComplete)
			}
		})
	}
}

// TestTripleShotCoordinator_ProcessAttemptCompletion_BoundsCheck tests the bounds checking
func TestTripleShotCoordinator_ProcessAttemptCompletion_BoundsCheck(t *testing.T) {
	tests := []struct {
		name         string
		attemptIndex int
		wantErr      bool
		errSubstring string
	}{
		{
			name:         "negative index",
			attemptIndex: -1,
			wantErr:      true,
			errSubstring: "invalid attempt index",
		},
		{
			name:         "index too large",
			attemptIndex: 3,
			wantErr:      true,
			errSubstring: "invalid attempt index",
		},
		{
			name:         "index at boundary",
			attemptIndex: 4,
			wantErr:      true,
			errSubstring: "invalid attempt index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseSession := &Session{ID: "test-session"}
			tripleSession := NewTripleShotSession("test task", DefaultTripleShotConfig())

			coordinator := &TripleShotCoordinator{
				manager:     NewTripleShotManager(nil, baseSession, tripleSession, nil),
				baseSession: baseSession,
			}

			err := coordinator.ProcessAttemptCompletion(tt.attemptIndex)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errSubstring != "" && !strings.Contains(err.Error(), tt.errSubstring) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.errSubstring)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestTripleShotCoordinator_GetWinningBranch tests the GetWinningBranch method
func TestTripleShotCoordinator_GetWinningBranch(t *testing.T) {
	tests := []struct {
		name       string
		evaluation *TripleShotEvaluation
		attempts   [3]TripleShotAttempt
		wantBranch string
	}{
		{
			name:       "no evaluation",
			evaluation: nil,
			wantBranch: "",
		},
		{
			name: "merge strategy not select",
			evaluation: &TripleShotEvaluation{
				MergeStrategy: MergeStrategyMerge,
				WinnerIndex:   0,
			},
			wantBranch: "",
		},
		{
			name: "winner index negative",
			evaluation: &TripleShotEvaluation{
				MergeStrategy: MergeStrategySelect,
				WinnerIndex:   -1,
			},
			wantBranch: "",
		},
		{
			name: "winner index too large",
			evaluation: &TripleShotEvaluation{
				MergeStrategy: MergeStrategySelect,
				WinnerIndex:   3,
			},
			wantBranch: "",
		},
		{
			name: "valid winner index 0",
			evaluation: &TripleShotEvaluation{
				MergeStrategy: MergeStrategySelect,
				WinnerIndex:   0,
			},
			attempts: [3]TripleShotAttempt{
				{Branch: "branch-0"},
				{Branch: "branch-1"},
				{Branch: "branch-2"},
			},
			wantBranch: "branch-0",
		},
		{
			name: "valid winner index 2",
			evaluation: &TripleShotEvaluation{
				MergeStrategy: MergeStrategySelect,
				WinnerIndex:   2,
			},
			attempts: [3]TripleShotAttempt{
				{Branch: "branch-0"},
				{Branch: "branch-1"},
				{Branch: "branch-2"},
			},
			wantBranch: "branch-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseSession := &Session{ID: "test-session"}
			tripleSession := NewTripleShotSession("test task", DefaultTripleShotConfig())
			tripleSession.Evaluation = tt.evaluation
			tripleSession.Attempts = tt.attempts

			coordinator := &TripleShotCoordinator{
				manager:     NewTripleShotManager(nil, baseSession, tripleSession, nil),
				baseSession: baseSession,
			}

			got := coordinator.GetWinningBranch()
			if got != tt.wantBranch {
				t.Errorf("GetWinningBranch() = %q, want %q", got, tt.wantBranch)
			}
		})
	}
}

// TestTripleShotCoordinator_CheckJudgeCompletion tests the CheckJudgeCompletion method
func TestTripleShotCoordinator_CheckJudgeCompletion(t *testing.T) {
	tests := []struct {
		name         string
		judgeID      string
		setupFunc    func(session *Session, worktree string)
		wantComplete bool
		wantErr      bool
	}{
		{
			name:         "no judge ID",
			judgeID:      "",
			wantComplete: false,
			wantErr:      false,
		},
		{
			name:    "judge instance not found",
			judgeID: "nonexistent",
			setupFunc: func(session *Session, worktree string) {
				// Don't add the instance to the session
			},
			wantComplete: false,
			wantErr:      true,
		},
		{
			name:    "judge instance found no evaluation file",
			judgeID: "judge-1",
			setupFunc: func(session *Session, worktree string) {
				session.Instances = append(session.Instances, &Instance{
					ID:           "judge-1",
					WorktreePath: worktree,
				})
			},
			wantComplete: false,
			wantErr:      false,
		},
		{
			name:    "judge instance found with evaluation file",
			judgeID: "judge-2",
			setupFunc: func(session *Session, worktree string) {
				session.Instances = append(session.Instances, &Instance{
					ID:           "judge-2",
					WorktreePath: worktree,
				})
				// Write evaluation file
				eval := TripleShotEvaluation{
					WinnerIndex:   1,
					MergeStrategy: MergeStrategySelect,
					Reasoning:     "Test reasoning",
				}
				data, _ := json.Marshal(eval)
				_ = os.WriteFile(filepath.Join(worktree, TripleShotEvaluationFileName), data, 0644)
			},
			wantComplete: true,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			baseSession := &Session{
				ID:        "test-session",
				Instances: []*Instance{},
			}
			tripleSession := NewTripleShotSession("test task", DefaultTripleShotConfig())
			tripleSession.JudgeID = tt.judgeID

			if tt.setupFunc != nil {
				tt.setupFunc(baseSession, tmpDir)
			}

			coordinator := &TripleShotCoordinator{
				manager:     NewTripleShotManager(nil, baseSession, tripleSession, nil),
				baseSession: baseSession,
			}

			complete, err := coordinator.CheckJudgeCompletion()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if complete != tt.wantComplete {
				t.Errorf("complete = %v, want %v", complete, tt.wantComplete)
			}
		})
	}
}

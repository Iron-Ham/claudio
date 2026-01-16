package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// TripleShotPhase represents the current phase of a triple-shot session
type TripleShotPhase string

const (
	// PhaseTripleShotWorking - three instances are working on the problem
	PhaseTripleShotWorking TripleShotPhase = "working"
	// PhaseTripleShotEvaluating - the judge instance is evaluating the solutions
	PhaseTripleShotEvaluating TripleShotPhase = "evaluating"
	// PhaseTripleShotComplete - evaluation is complete
	PhaseTripleShotComplete TripleShotPhase = "complete"
	// PhaseTripleShotFailed - something went wrong
	PhaseTripleShotFailed TripleShotPhase = "failed"
)

// AttemptStatus represents the status of a triple-shot attempt
type AttemptStatus string

const (
	// AttemptStatusPending - attempt has not started yet
	AttemptStatusPending AttemptStatus = ""
	// AttemptStatusWorking - attempt is actively working on the problem
	AttemptStatusWorking AttemptStatus = "working"
	// AttemptStatusCompleted - attempt has completed successfully
	AttemptStatusCompleted AttemptStatus = "completed"
	// AttemptStatusFailed - attempt has failed
	AttemptStatusFailed AttemptStatus = "failed"
)

// MergeStrategy represents the judge's recommended approach for the solution
type MergeStrategy string

const (
	// MergeStrategySelect - select one attempt's solution as the winner
	MergeStrategySelect MergeStrategy = "select"
	// MergeStrategyMerge - merge elements from multiple attempts
	MergeStrategyMerge MergeStrategy = "merge"
	// MergeStrategyCombine - combine all attempts into a unified solution
	MergeStrategyCombine MergeStrategy = "combine"
)

// TripleShotConfig holds configuration for a triple-shot session
type TripleShotConfig struct {
	// AutoApprove skips user confirmation for applying the winning solution
	AutoApprove bool `json:"auto_approve"`
}

// DefaultTripleShotConfig returns the default configuration
func DefaultTripleShotConfig() TripleShotConfig {
	return TripleShotConfig{
		AutoApprove: false,
	}
}

// TripleShotAttempt represents one of the three solution attempts
type TripleShotAttempt struct {
	InstanceID   string        `json:"instance_id"`
	WorktreePath string        `json:"worktree_path"`
	Branch       string        `json:"branch"`
	Status       AttemptStatus `json:"status"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
}

// TripleShotEvaluation holds the judge's evaluation of the three attempts
type TripleShotEvaluation struct {
	WinnerIndex       int                     `json:"winner_index"`        // 0, 1, or 2 (-1 if merged)
	MergeStrategy     MergeStrategy           `json:"merge_strategy"`      // Strategy for applying solution
	Reasoning         string                  `json:"reasoning"`           // Explanation of the decision
	AttemptEvaluation []AttemptEvaluationItem `json:"attempt_evaluations"` // Evaluation of each attempt
	SuggestedChanges  []string                `json:"suggested_changes"`   // If merging, changes to make
}

// AttemptEvaluationItem holds the evaluation for a single attempt
type AttemptEvaluationItem struct {
	AttemptIndex int      `json:"attempt_index"`
	Score        int      `json:"score"` // 1-10
	Strengths    []string `json:"strengths"`
	Weaknesses   []string `json:"weaknesses"`
}

// TripleShotSession represents a triple-shot orchestration session
type TripleShotSession struct {
	ID          string                `json:"id"`
	GroupID     string                `json:"group_id,omitempty"` // Link to InstanceGroup for multi-tripleshot support
	Task        string                `json:"task"`               // The original task/problem
	Phase       TripleShotPhase       `json:"phase"`
	Config      TripleShotConfig      `json:"config"`
	Attempts    [3]TripleShotAttempt  `json:"attempts"`           // Exactly 3 attempts
	JudgeID     string                `json:"judge_id,omitempty"` // Instance ID of the judge
	Created     time.Time             `json:"created"`
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	Evaluation  *TripleShotEvaluation `json:"evaluation,omitempty"` // Judge's evaluation
	Error       string                `json:"error,omitempty"`      // Error message if failed
}

// NewTripleShotSession creates a new triple-shot session
func NewTripleShotSession(task string, config TripleShotConfig) *TripleShotSession {
	return &TripleShotSession{
		ID:      generateID(),
		Task:    task,
		Phase:   PhaseTripleShotWorking,
		Config:  config,
		Created: time.Now(),
	}
}

// AllAttemptsComplete returns true if all three attempts have completed (success or failure)
func (s *TripleShotSession) AllAttemptsComplete() bool {
	for _, attempt := range s.Attempts {
		if attempt.Status == AttemptStatusWorking || attempt.Status == AttemptStatusPending {
			return false
		}
	}
	return true
}

// SuccessfulAttemptCount returns the number of attempts that completed successfully
func (s *TripleShotSession) SuccessfulAttemptCount() int {
	count := 0
	for _, attempt := range s.Attempts {
		if attempt.Status == AttemptStatusCompleted {
			count++
		}
	}
	return count
}

// TripleShotCompletionFileName is the sentinel file that attempts write when complete
const TripleShotCompletionFileName = ".claudio-tripleshot-complete.json"

// TripleShotCompletionFile represents the completion report written by an attempt
type TripleShotCompletionFile struct {
	AttemptIndex  int            `json:"attempt_index"`
	Status        string         `json:"status"` // "complete" or "failed"
	Summary       string         `json:"summary"`
	FilesModified []string       `json:"files_modified"`
	Approach      string         `json:"approach"`        // Description of the approach taken
	Notes         FlexibleString `json:"notes,omitempty"` // Uses FlexibleString to accept both string and []string
}

// TripleShotCompletionFilePath returns the full path to the completion file
func TripleShotCompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, TripleShotCompletionFileName)
}

// ParseTripleShotCompletionFile reads and parses a triple-shot completion file
func ParseTripleShotCompletionFile(worktreePath string) (*TripleShotCompletionFile, error) {
	completionPath := TripleShotCompletionFilePath(worktreePath)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion TripleShotCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse triple-shot completion JSON: %w", err)
	}

	return &completion, nil
}

// TripleShotEvaluationFileName is the sentinel file that the judge writes when evaluation is complete
const TripleShotEvaluationFileName = ".claudio-tripleshot-evaluation.json"

// TripleShotEvaluationFilePath returns the full path to the evaluation file
func TripleShotEvaluationFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, TripleShotEvaluationFileName)
}

// ParseTripleShotEvaluationFile reads and parses the judge's evaluation file
func ParseTripleShotEvaluationFile(worktreePath string) (*TripleShotEvaluation, error) {
	evalPath := TripleShotEvaluationFilePath(worktreePath)
	data, err := os.ReadFile(evalPath)
	if err != nil {
		return nil, err
	}

	var evaluation TripleShotEvaluation
	if err := json.Unmarshal(data, &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse triple-shot evaluation JSON: %w", err)
	}

	return &evaluation, nil
}

// ParseTripleShotEvaluationFromOutput extracts evaluation from Claude's output
// It looks for JSON wrapped in <evaluation></evaluation> tags
func ParseTripleShotEvaluationFromOutput(output string) (*TripleShotEvaluation, error) {
	re := regexp.MustCompile(`(?s)<evaluation>\s*(.*?)\s*</evaluation>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no evaluation found in output (expected <evaluation>JSON</evaluation>)")
	}

	jsonStr := strings.TrimSpace(matches[1])

	var evaluation TripleShotEvaluation
	if err := json.Unmarshal([]byte(jsonStr), &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation JSON: %w", err)
	}

	return &evaluation, nil
}

// TripleShotManager manages the execution of a triple-shot session
type TripleShotManager struct {
	session     *TripleShotSession
	orch        *Orchestrator
	baseSession *Session
	logger      *logging.Logger

	// Event handling
	eventCallback func(TripleShotEvent)

	// Synchronization
	mu sync.RWMutex
}

// TripleShotEventType represents the type of triple-shot event
type TripleShotEventType string

const (
	EventTripleShotAttemptStarted   TripleShotEventType = "attempt_started"
	EventTripleShotAttemptComplete  TripleShotEventType = "attempt_complete"
	EventTripleShotAttemptFailed    TripleShotEventType = "attempt_failed"
	EventTripleShotAllAttemptsReady TripleShotEventType = "all_attempts_ready"
	EventTripleShotJudgeStarted     TripleShotEventType = "judge_started"
	EventTripleShotEvaluationReady  TripleShotEventType = "evaluation_ready"
	EventTripleShotPhaseChange      TripleShotEventType = "phase_change"
)

// TripleShotEvent represents an event from the triple-shot manager
type TripleShotEvent struct {
	Type         TripleShotEventType `json:"type"`
	AttemptIndex int                 `json:"attempt_index,omitempty"` // 0-2 for attempt events
	InstanceID   string              `json:"instance_id,omitempty"`
	Message      string              `json:"message,omitempty"`
	Timestamp    time.Time           `json:"timestamp"`
}

// NewTripleShotManager creates a new triple-shot manager
func NewTripleShotManager(orch *Orchestrator, baseSession *Session, tripleSession *TripleShotSession, logger *logging.Logger) *TripleShotManager {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &TripleShotManager{
		session:     tripleSession,
		orch:        orch,
		baseSession: baseSession,
		logger:      logger.WithPhase("tripleshot"),
	}
}

// SetEventCallback sets the callback for triple-shot events
func (m *TripleShotManager) SetEventCallback(cb func(TripleShotEvent)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventCallback = cb
}

// Session returns the triple-shot session
func (m *TripleShotManager) Session() *TripleShotSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}

// emitEvent sends an event to the callback
func (m *TripleShotManager) emitEvent(event TripleShotEvent) {
	event.Timestamp = time.Now()

	m.mu.RLock()
	cb := m.eventCallback
	m.mu.RUnlock()

	if cb != nil {
		cb(event)
	}
}

// SetPhase updates the session phase and emits an event
func (m *TripleShotManager) SetPhase(phase TripleShotPhase) {
	m.mu.Lock()
	m.session.Phase = phase
	m.mu.Unlock()

	m.emitEvent(TripleShotEvent{
		Type:    EventTripleShotPhaseChange,
		Message: string(phase),
	})
}

// MarkAttemptComplete marks an attempt as completed
func (m *TripleShotManager) MarkAttemptComplete(attemptIndex int) {
	m.mu.Lock()
	if attemptIndex >= 0 && attemptIndex < 3 {
		m.session.Attempts[attemptIndex].Status = AttemptStatusCompleted
		now := time.Now()
		m.session.Attempts[attemptIndex].CompletedAt = &now
	}
	m.mu.Unlock()

	m.logger.Info("attempt completed", "attempt_index", attemptIndex)

	m.emitEvent(TripleShotEvent{
		Type:         EventTripleShotAttemptComplete,
		AttemptIndex: attemptIndex,
	})
}

// MarkAttemptFailed marks an attempt as failed
func (m *TripleShotManager) MarkAttemptFailed(attemptIndex int, reason string) {
	m.mu.Lock()
	if attemptIndex >= 0 && attemptIndex < 3 {
		m.session.Attempts[attemptIndex].Status = AttemptStatusFailed
		now := time.Now()
		m.session.Attempts[attemptIndex].CompletedAt = &now
	}
	m.mu.Unlock()

	m.logger.Info("attempt failed", "attempt_index", attemptIndex, "reason", reason)

	m.emitEvent(TripleShotEvent{
		Type:         EventTripleShotAttemptFailed,
		AttemptIndex: attemptIndex,
		Message:      reason,
	})
}

// SetEvaluation sets the evaluation on the session
func (m *TripleShotManager) SetEvaluation(eval *TripleShotEvaluation) {
	m.mu.Lock()
	m.session.Evaluation = eval
	m.mu.Unlock()

	m.logger.Info("evaluation set",
		"winner_index", eval.WinnerIndex,
		"strategy", eval.MergeStrategy,
	)

	m.emitEvent(TripleShotEvent{
		Type:    EventTripleShotEvaluationReady,
		Message: eval.Reasoning,
	})
}

// TripleShotAttemptPromptTemplate is the prompt template for each attempt
const TripleShotAttemptPromptTemplate = `You are one of three Claude instances working on the same problem.
Your goal is to solve the problem using your own approach - be creative and thorough.

## Task
%s

## Important Instructions

1. **Solve the problem independently** - Don't try to coordinate with other instances
2. **Use your best judgment** - Choose the approach you think is most appropriate
3. **Document your approach** - Write clear commit messages and code comments
4. **Be thorough** - Make sure your solution is complete and well-tested

## CRITICAL: Completion File Requirement

**YOUR WORK IS NOT COMPLETE UNTIL YOU WRITE THE COMPLETION FILE.**

The orchestration system is waiting for this file to know you are done. Without it, your work will be ignored and the system will hang indefinitely. You MUST write this file as your FINAL action, no matter what.

**File:** ` + "`" + TripleShotCompletionFileName + "`" + ` (in your worktree root)

**Required JSON structure:**
` + "```json" + `
{
  "attempt_index": %d,
  "status": "complete",
  "summary": "Brief summary of what you implemented",
  "files_modified": ["file1.go", "file2.go"],
  "approach": "Description of the approach you took and why",
  "notes": "Any additional notes or concerns"
}
` + "```" + `

**Rules:**
- Set status to "complete" if you finished successfully, or "failed" if you couldn't complete the task
- The approach field is critical - it helps the judge understand and evaluate your solution
- Even if you encounter errors or cannot complete the task, you MUST still write this file with status "failed"
- This file is your ONLY way to signal completion - do not skip it under any circumstances

**REMINDER: Write ` + "`" + TripleShotCompletionFileName + "`" + ` as your absolute last action.**`

// TripleShotJudgePromptTemplate is the prompt template for the judge instance
const TripleShotJudgePromptTemplate = `You are a senior software architect evaluating three different solutions to the same problem.

## Original Task
%s

## Solution Summaries

### Attempt 1 (Instance %s)
Branch: %s
Working Directory: %s
Completion Summary:
%s

### Attempt 2 (Instance %s)
Branch: %s
Working Directory: %s
Completion Summary:
%s

### Attempt 3 (Instance %s)
Branch: %s
Working Directory: %s
Completion Summary:
%s

## Your Task

1. **Review each solution** - Examine the code changes in each branch
2. **Evaluate** each solution on:
   - Correctness: Does it solve the problem correctly?
   - Code quality: Is the code clean, maintainable, and idiomatic?
   - Completeness: Does it handle edge cases and error conditions?
   - Testing: Are there appropriate tests?
3. **Decide** which solution to use:
   - **Select**: Choose one solution as the winner
   - **Merge**: Combine the best parts of multiple solutions
   - **Combine**: Create a hybrid that takes specific elements from each

## Output

Write your evaluation to ` + "`" + TripleShotEvaluationFileName + "`" + ` using this structure:
` + "```json" + `
{
  "winner_index": 0,
  "merge_strategy": "select",
  "reasoning": "Explanation of your decision",
  "attempt_evaluations": [
    {
      "attempt_index": 0,
      "score": 8,
      "strengths": ["Good error handling", "Clean code"],
      "weaknesses": ["Missing edge case"]
    },
    {
      "attempt_index": 1,
      "score": 7,
      "strengths": ["Comprehensive tests"],
      "weaknesses": ["Complex implementation"]
    },
    {
      "attempt_index": 2,
      "score": 6,
      "strengths": ["Simple approach"],
      "weaknesses": ["No tests", "Missing error handling"]
    }
  ],
  "suggested_changes": []
}
` + "```" + `

### Fields:
- **winner_index**: 0, 1, or 2 for the winning solution. Use -1 if merging multiple solutions.
- **merge_strategy**: "select" (use one as-is), "merge" (combine changes), or "combine" (cherry-pick specific changes)
- **reasoning**: Detailed explanation of your evaluation and decision
- **attempt_evaluations**: Score and analysis for each attempt (1-10 scale)
- **suggested_changes**: If merge_strategy is "merge" or "combine", list the specific changes to make

## Helpful Commands

To compare branches:
- ` + "`" + `git diff main..branch-name` + "`" + ` - See changes from main
- ` + "`" + `git log main..branch-name --oneline` + "`" + ` - See commits

To review code in each worktree, you can navigate to the directories listed above.`

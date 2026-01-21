// Package tripleshot provides the triple-shot workflow coordinator.
// Triple-shot runs three Claude instances in parallel on the same task,
// then uses a fourth "judge" instance to evaluate all solutions.
package tripleshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Phase represents the current phase of a triple-shot session
type Phase string

const (
	// PhaseWorking - three instances are working on the problem
	PhaseWorking Phase = "working"
	// PhaseEvaluating - the judge instance is evaluating the solutions
	PhaseEvaluating Phase = "evaluating"
	// PhaseComplete - evaluation is complete
	PhaseComplete Phase = "complete"
	// PhaseFailed - something went wrong
	PhaseFailed Phase = "failed"
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

// Config holds configuration for a triple-shot session
type Config struct {
	// AutoApprove skips user confirmation for applying the winning solution
	AutoApprove bool `json:"auto_approve"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		AutoApprove: false,
	}
}

// Attempt represents one of the three solution attempts
type Attempt struct {
	InstanceID   string        `json:"instance_id"`
	WorktreePath string        `json:"worktree_path"`
	Branch       string        `json:"branch"`
	Status       AttemptStatus `json:"status"`
	StartedAt    *time.Time    `json:"started_at,omitempty"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
}

// Evaluation holds the judge's evaluation of the three attempts
type Evaluation struct {
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

// CompletionFileName is the sentinel file that attempts write when complete
const CompletionFileName = ".claudio-tripleshot-complete.json"

// FlexibleString is a custom type that can unmarshal either a JSON string or an array of strings.
// When unmarshaling an array, strings are joined with newlines.
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler for FlexibleString.
func (f *FlexibleString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as a string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*f = FlexibleString(s)
		return nil
	}

	// Try to unmarshal as an array of strings
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = FlexibleString(strings.Join(arr, "\n"))
		return nil
	}

	return fmt.Errorf("FlexibleString: expected string or []string, got %s", string(data))
}

// String returns the FlexibleString as a regular string.
func (f FlexibleString) String() string {
	return string(f)
}

// CompletionFile represents the completion report written by an attempt
type CompletionFile struct {
	AttemptIndex  int            `json:"attempt_index"`
	Status        string         `json:"status"` // "complete" or "failed"
	Summary       string         `json:"summary"`
	FilesModified []string       `json:"files_modified"`
	Approach      string         `json:"approach"`        // Description of the approach taken
	Notes         FlexibleString `json:"notes,omitempty"` // Uses FlexibleString to accept both string and []string
}

// CompletionFilePath returns the full path to the completion file
func CompletionFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, CompletionFileName)
}

// FindCompletionFile searches for the completion file, first in the worktree root,
// then in immediate subdirectories. This handles cases where Claude instances
// write the file relative to their current working directory instead of the
// worktree root.
func FindCompletionFile(worktreePath string) (string, error) {
	// First, check the expected location (worktree root)
	expectedPath := CompletionFilePath(worktreePath)
	_, err := os.Stat(expectedPath)
	if err == nil {
		return expectedPath, nil
	}
	// Only fall back to subdirectory search if file doesn't exist.
	// For other errors (permissions, I/O), propagate them.
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check completion file: %w", err)
	}

	// Search immediate subdirectories (depth 1)
	entries, err := os.ReadDir(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to read worktree directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories (like .git)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		subPath := filepath.Join(worktreePath, entry.Name(), CompletionFileName)
		_, err := os.Stat(subPath)
		if err == nil {
			return subPath, nil
		}
		// Continue searching if file doesn't exist; propagate other errors
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check completion file in %s: %w", entry.Name(), err)
		}
	}

	return "", os.ErrNotExist
}

// CompletionFileExists checks if a completion file exists for the given worktree,
// searching both the worktree root and immediate subdirectories.
func CompletionFileExists(worktreePath string) bool {
	_, err := FindCompletionFile(worktreePath)
	return err == nil
}

// ParseCompletionFile reads and parses a triple-shot completion file.
// It searches for the file in the worktree root and immediate subdirectories.
func ParseCompletionFile(worktreePath string) (*CompletionFile, error) {
	completionPath, err := FindCompletionFile(worktreePath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion CompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse triple-shot completion JSON: %w", err)
	}

	return &completion, nil
}

// EvaluationFileName is the sentinel file that the judge writes when evaluation is complete
const EvaluationFileName = ".claudio-tripleshot-evaluation.json"

// EvaluationFilePath returns the full path to the evaluation file
func EvaluationFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, EvaluationFileName)
}

// FindEvaluationFile searches for the evaluation file, first in the worktree root,
// then in immediate subdirectories. This handles cases where the judge instance
// writes the file relative to its current working directory instead of the
// worktree root.
func FindEvaluationFile(worktreePath string) (string, error) {
	// First, check the expected location (worktree root)
	expectedPath := EvaluationFilePath(worktreePath)
	_, err := os.Stat(expectedPath)
	if err == nil {
		return expectedPath, nil
	}
	// Only fall back to subdirectory search if file doesn't exist.
	// For other errors (permissions, I/O), propagate them.
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check evaluation file: %w", err)
	}

	// Search immediate subdirectories (depth 1)
	entries, err := os.ReadDir(worktreePath)
	if err != nil {
		return "", fmt.Errorf("failed to read worktree directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories (like .git)
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		subPath := filepath.Join(worktreePath, entry.Name(), EvaluationFileName)
		_, err := os.Stat(subPath)
		if err == nil {
			return subPath, nil
		}
		// Continue searching if file doesn't exist; propagate other errors
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check evaluation file in %s: %w", entry.Name(), err)
		}
	}

	return "", os.ErrNotExist
}

// EvaluationFileExists checks if an evaluation file exists for the given worktree,
// searching both the worktree root and immediate subdirectories.
func EvaluationFileExists(worktreePath string) bool {
	_, err := FindEvaluationFile(worktreePath)
	return err == nil
}

// ParseEvaluationFile reads and parses the judge's evaluation file.
// It searches for the file in the worktree root and immediate subdirectories.
func ParseEvaluationFile(worktreePath string) (*Evaluation, error) {
	evalPath, err := FindEvaluationFile(worktreePath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(evalPath)
	if err != nil {
		return nil, err
	}

	var evaluation Evaluation
	if err := json.Unmarshal(data, &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse triple-shot evaluation JSON: %w", err)
	}

	return &evaluation, nil
}

// ParseEvaluationFromOutput extracts evaluation from Claude's output
// It looks for JSON wrapped in <evaluation></evaluation> tags
func ParseEvaluationFromOutput(output string) (*Evaluation, error) {
	re := regexp.MustCompile(`(?s)<evaluation>\s*(.*?)\s*</evaluation>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		return nil, fmt.Errorf("no evaluation found in output (expected <evaluation>JSON</evaluation>)")
	}

	jsonStr := strings.TrimSpace(matches[1])

	var evaluation Evaluation
	if err := json.Unmarshal([]byte(jsonStr), &evaluation); err != nil {
		return nil, fmt.Errorf("failed to parse evaluation JSON: %w", err)
	}

	return &evaluation, nil
}

// EventType represents the type of triple-shot event
type EventType string

const (
	EventAttemptStarted   EventType = "attempt_started"
	EventAttemptComplete  EventType = "attempt_complete"
	EventAttemptFailed    EventType = "attempt_failed"
	EventAllAttemptsReady EventType = "all_attempts_ready"
	EventJudgeStarted     EventType = "judge_started"
	EventEvaluationReady  EventType = "evaluation_ready"
	EventPhaseChange      EventType = "phase_change"
)

// Event represents an event from the triple-shot manager
type Event struct {
	Type         EventType `json:"type"`
	AttemptIndex int       `json:"attempt_index,omitempty"` // 0-2 for attempt events
	InstanceID   string    `json:"instance_id,omitempty"`
	Message      string    `json:"message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// AttemptPromptTemplate is the prompt template for each attempt
const AttemptPromptTemplate = `You are one of three Claude instances working on the same problem.
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

**File:** ` + "`" + CompletionFileName + "`" + ` (in your worktree root)

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

**REMINDER: Write ` + "`" + CompletionFileName + "`" + ` as your absolute last action.**`

// JudgePromptTemplate is the prompt template for the judge instance
const JudgePromptTemplate = `You are a senior software architect evaluating three different solutions to the same problem.

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

Write your evaluation to ` + "`" + EvaluationFileName + "`" + ` using this structure:
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

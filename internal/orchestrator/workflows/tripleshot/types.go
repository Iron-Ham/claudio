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
	// PhaseAdversarialReview - implementers' work is being reviewed (adversarial mode only)
	PhaseAdversarialReview Phase = "adversarial_review"
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
	// AttemptStatusPreparing - attempt stub created, worktree being set up
	AttemptStatusPreparing AttemptStatus = "preparing"
	// AttemptStatusWorking - attempt is actively working on the problem
	AttemptStatusWorking AttemptStatus = "working"
	// AttemptStatusUnderReview - implementation complete, awaiting adversarial review approval
	AttemptStatusUnderReview AttemptStatus = "under_review"
	// AttemptStatusCompleted - attempt has completed successfully (and passed review if adversarial)
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
	// Adversarial enables adversarial review mode where each implementer must pass review
	Adversarial bool `json:"adversarial"`
	// MinPassingScore is the minimum score required for approval in adversarial mode (1-10, default: 8)
	MinPassingScore int `json:"min_passing_score,omitempty"`
	// MaxAdversarialRounds is the maximum number of implement-review cycles per attempt.
	// This is populated from adversarial.max_iterations config (default: 10, 0 = unlimited).
	MaxAdversarialRounds int `json:"max_adversarial_rounds,omitempty"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		AutoApprove:          false,
		Adversarial:          false,
		MinPassingScore:      8,
		MaxAdversarialRounds: 10,
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
	// Adversarial review fields (only used when Config.Adversarial is true)
	ReviewerID     string `json:"reviewer_id,omitempty"`     // Instance ID of the adversarial reviewer
	ReviewApproved bool   `json:"review_approved,omitempty"` // Whether the reviewer approved the attempt
	ReviewScore    int    `json:"review_score,omitempty"`    // Score from the reviewer (1-10)
	ReviewRound    int    `json:"review_round,omitempty"`    // Current review round (1-based)

	// Sub-group tracking for adversarial mode display
	AttemptGroupID        string                `json:"attempt_group_id,omitempty"`         // ID of the "Attempt N" sub-group
	PreviousRoundsGroupID string                `json:"previous_rounds_group_id,omitempty"` // ID of the "Previous Rounds" container
	RoundHistory          []AttemptRoundHistory `json:"round_history,omitempty"`            // History of previous rounds
}

// AttemptRoundHistory tracks instance IDs from previous adversarial rounds for an attempt.
type AttemptRoundHistory struct {
	Round         int    `json:"round"`
	ImplementerID string `json:"implementer_id,omitempty"`
	ReviewerID    string `json:"reviewer_id,omitempty"`
	SubGroupID    string `json:"sub_group_id,omitempty"` // Set when moved to "Previous Rounds"
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

// findSentinelFile searches for a sentinel file by name, first in the worktree root,
// then in immediate subdirectories. This handles cases where Claude instances
// write the file relative to their current working directory instead of the
// worktree root.
func findSentinelFile(worktreePath, fileName, fileDescription string) (string, error) {
	// First, check the expected location (worktree root)
	expectedPath := filepath.Join(worktreePath, fileName)
	_, err := os.Stat(expectedPath)
	if err == nil {
		return expectedPath, nil
	}
	// Only fall back to subdirectory search if file doesn't exist.
	// For other errors (permissions, I/O), propagate them.
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to check %s file: %w", fileDescription, err)
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
		subPath := filepath.Join(worktreePath, entry.Name(), fileName)
		_, err := os.Stat(subPath)
		if err == nil {
			return subPath, nil
		}
		// Continue searching if file doesn't exist; propagate other errors
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to check %s file in %s: %w", fileDescription, entry.Name(), err)
		}
	}

	return "", os.ErrNotExist
}

// FindCompletionFile searches for the completion file, first in the worktree root,
// then in immediate subdirectories. This handles cases where Claude instances
// write the file relative to their current working directory instead of the
// worktree root.
func FindCompletionFile(worktreePath string) (string, error) {
	return findSentinelFile(worktreePath, CompletionFileName, "completion")
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
	return findSentinelFile(worktreePath, EvaluationFileName, "evaluation")
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
	// Adversarial review events
	EventReviewerStarted EventType = "reviewer_started"
	EventReviewApproved  EventType = "review_approved"
	EventReviewRejected  EventType = "review_rejected"
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

## CRITICAL: Completion File Requirement - FINAL MANDATORY STEP

**IMPORTANT**: Writing the completion file is your FINAL MANDATORY ACTION.
The orchestrator is BLOCKED waiting for this file.
Without it, your work will NOT be recorded and the workflow cannot proceed.

**DO NOT** wait for user prompting or confirmation.
Write this file AUTOMATICALLY as soon as you have finished your implementation.

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

**REMEMBER**: Your attempt is NOT complete until you write this file. Do it NOW after finishing your work.`

// AdversarialReviewFileName is the sentinel file that adversarial reviewers write when complete
const AdversarialReviewFileName = ".claudio-tripleshot-review.json"

// AdversarialReviewFilePath returns the full path to the adversarial review file
func AdversarialReviewFilePath(worktreePath string) string {
	return filepath.Join(worktreePath, AdversarialReviewFileName)
}

// FindAdversarialReviewFile searches for the adversarial review file, first in the worktree root,
// then in immediate subdirectories. This handles cases where the reviewer
// writes the file relative to its current working directory instead of the
// worktree root.
func FindAdversarialReviewFile(worktreePath string) (string, error) {
	return findSentinelFile(worktreePath, AdversarialReviewFileName, "adversarial review")
}

// AdversarialReviewFile represents the reviewer's feedback on an attempt
type AdversarialReviewFile struct {
	AttemptIndex    int      `json:"attempt_index"`    // Which attempt (0-2) this review is for
	Round           int      `json:"round"`            // Review round (starts at 1)
	Approved        bool     `json:"approved"`         // Whether the implementation is approved
	Score           int      `json:"score"`            // Quality score (1-10)
	Strengths       []string `json:"strengths"`        // What was done well
	Issues          []string `json:"issues"`           // Problems that must be fixed
	Suggestions     []string `json:"suggestions"`      // Optional improvements
	Summary         string   `json:"summary"`          // Overall assessment
	RequiredChanges []string `json:"required_changes"` // Specific changes needed (if not approved)
}

// Validate checks that the AdversarialReviewFile has valid values.
// This helps catch malformed reviews from LLM output.
func (r *AdversarialReviewFile) Validate() error {
	if r.AttemptIndex < 0 || r.AttemptIndex > 2 {
		return fmt.Errorf("attempt_index must be 0-2, got %d", r.AttemptIndex)
	}
	if r.Round < 1 {
		return fmt.Errorf("round must be >= 1, got %d", r.Round)
	}
	if r.Score < 1 || r.Score > 10 {
		return fmt.Errorf("score must be 1-10, got %d", r.Score)
	}
	return nil
}

// ParseAdversarialReviewFile reads and parses a tripleshot adversarial review file.
// It searches for the file in the worktree root and immediate subdirectories.
func ParseAdversarialReviewFile(worktreePath string) (*AdversarialReviewFile, error) {
	reviewPath, err := FindAdversarialReviewFile(worktreePath)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(reviewPath)
	if err != nil {
		return nil, err
	}

	var review AdversarialReviewFile
	if err := json.Unmarshal(data, &review); err != nil {
		return nil, fmt.Errorf("failed to parse tripleshot adversarial review JSON: %w", err)
	}

	if err := review.Validate(); err != nil {
		return nil, fmt.Errorf("invalid adversarial review: %w", err)
	}

	return &review, nil
}

// AdversarialReviewFileExists checks if the adversarial review file exists for the given worktree,
// searching both the worktree root and immediate subdirectories.
func AdversarialReviewFileExists(worktreePath string) bool {
	_, err := FindAdversarialReviewFile(worktreePath)
	return err == nil
}

// AdversarialReviewerPromptTemplate is the prompt for reviewing a tripleshot attempt
const AdversarialReviewerPromptTemplate = `You are a CRITICAL REVIEWER examining one of three parallel implementations.

## Original Task
%s

## Attempt Being Reviewed
This is Attempt %d (of 3 parallel attempts).

## Implementation Summary
**Summary:** %s

**Approach:** %s

**Files Modified:** %v

**Notes from Implementer:** %s

## Your Role
You must thoroughly and critically examine this attempt's implementation. Be demanding - your job is to ensure quality. Only approve when the implementation truly meets all requirements.

## Review Guidelines
1. **Examine the code thoroughly** - Read all modified files, understand the approach
2. **Be critical** - Look for bugs, edge cases, security issues, performance problems
3. **Check completeness** - Does it fully solve the task? Are there missing pieces?
4. **Verify quality** - Is the code clean, well-structured, properly tested?
5. **Consider maintainability** - Will this code be easy to understand and modify?

## What to Look For
- Logic errors and bugs
- Missing error handling
- Security vulnerabilities
- Performance issues
- Code style violations
- Missing or inadequate tests
- Incomplete implementations
- Edge cases not handled

## CRITICAL: Review File Requirement - FINAL MANDATORY STEP

**IMPORTANT**: Writing the review file is your FINAL MANDATORY ACTION.
The system is BLOCKED waiting for this file.
Without it, your review will NOT be recorded and the workflow cannot proceed.

**File:** ` + "`" + AdversarialReviewFileName + "`" + ` (in your worktree root)

**Required JSON structure:**
` + "```json" + `
{
  "attempt_index": %d,
  "round": %d,
  "approved": false,
  "score": 7,
  "strengths": ["Good error handling", "Clean code structure"],
  "issues": ["Missing null check in line 42", "No tests for edge case X"],
  "suggestions": ["Consider adding logging", "Could optimize the loop"],
  "summary": "Overall assessment of the implementation",
  "required_changes": ["Fix the null check", "Add tests for edge case X"]
}
` + "```" + `

**Rules:**
- **CRITICAL: Approval requires score >= %d.** Set approved to true ONLY when both conditions are met: (1) score >= %d AND (2) no critical issues remain.
- Score from 1-10: 1-4 = major problems, 5-6 = needs work, 7-8 = good, 9-10 = excellent
- Issues should list specific problems that MUST be fixed
- Suggestions are optional improvements (not required for approval)
- required_changes should be specific and actionable

**IMPORTANT:** Do NOT approve work that has significant issues or scores below %d.

**REMEMBER**: Your review is NOT complete until you write this file. Do it NOW after finishing your review.`

// FormatAdversarialReviewerPrompt creates the prompt for an adversarial reviewer
func FormatAdversarialReviewerPrompt(task string, attemptIndex int, round int, completion *CompletionFile, minPassingScore int) string {
	notes := ""
	if completion.Notes != "" {
		notes = completion.Notes.String()
	}
	return fmt.Sprintf(AdversarialReviewerPromptTemplate,
		task,
		attemptIndex+1, // 1-indexed for display
		completion.Summary,
		completion.Approach,
		completion.FilesModified,
		notes,
		attemptIndex,
		round,
		minPassingScore,
		minPassingScore,
		minPassingScore,
	)
}

// TripleShotFeedbackTemplate is the template appended to the implementer prompt when restarting after rejection
const TripleShotFeedbackTemplate = `

## Previous Review Feedback (Round %d)

The reviewer found issues that must be addressed before this attempt can be approved.

**Score:** %d/10

**Issues to Fix:**
%s

**Required Changes:**
%s

**Reviewer's Summary:**
%s

---

**IMPORTANT:** Address ALL issues listed above before signaling completion.
Your previous implementation has already been rejected. Focus on fixing the specific problems identified.
`

// FormatImplementerPromptWithFeedback creates the implementer prompt with previous review feedback appended.
// This is used when restarting an implementer after a rejection.
func FormatImplementerPromptWithFeedback(task string, attemptIndex int, round int, review *AdversarialReviewFile) string {
	basePrompt := fmt.Sprintf(AttemptPromptTemplate, task, attemptIndex)
	feedback := fmt.Sprintf(TripleShotFeedbackTemplate,
		round-1,
		review.Score,
		formatBulletList(review.Issues, "(No specific issues listed)"),
		formatBulletList(review.RequiredChanges, "(No specific changes listed)"),
		review.Summary,
	)
	return basePrompt + feedback
}

// formatBulletList formats a slice of strings as a bullet list, or returns the fallback if empty.
func formatBulletList(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback + "\n"
	}
	var result string
	for _, item := range items {
		result += "- " + item + "\n"
	}
	return result
}

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

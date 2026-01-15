// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
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

// SynthesisCompletionFileName is the sentinel file that synthesis writes when complete.
// This is used to detect when the synthesis phase has finished.
const SynthesisCompletionFileName = ".claudio-synthesis-complete.json"

// RevisionCompletionFileName is the sentinel file that revision tasks write when complete.
// This is used to detect when a revision task has finished fixing issues.
const RevisionCompletionFileName = ".claudio-revision-complete.json"

// DefaultMaxRevisions is the default maximum number of revision rounds allowed.
const DefaultMaxRevisions = 3

// SynthesisPromptTemplate is the prompt template used for the synthesis phase.
// Format args: objective, task list, results summary, revision round
const SynthesisPromptTemplate = `You are reviewing the results of a parallel execution plan.

## Original Objective
%s

## Completed Tasks
%s

## Task Results Summary
%s

## Instructions

1. **Review** all completed work to ensure it meets the original objective
2. **Identify** any integration issues, bugs, or conflicts that need resolution
3. **Verify** that all pieces work together correctly
4. **Check** for any missing functionality or incomplete implementations

## Completion Protocol

When your review is complete, you MUST write a completion file to signal the orchestrator:

1. Use Write tool to create ` + "`" + SynthesisCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "status": "complete",
  "revision_round": %d,
  "issues_found": [
    {
      "task_id": "task-id-here",
      "description": "Clear description of the issue",
      "files": ["file1.go", "file2.go"],
      "severity": "critical|major|minor",
      "suggestion": "How to fix this issue"
    }
  ],
  "tasks_affected": ["task-1", "task-2"],
  "integration_notes": "Observations about how the pieces integrate",
  "recommendations": ["Suggestions for the consolidation phase"]
}
` + "```" + `

3. Set status to "needs_revision" if critical/major issues require fixing, "complete" otherwise
4. Leave issues_found as empty array [] if no issues found
5. Include integration_notes with observations about cross-task integration
6. Add recommendations for the consolidation phase (merge order, potential conflicts, etc.)

This file signals that your review is done and provides context for subsequent phases.`

// RevisionPromptTemplate is the prompt template used for the revision phase.
// It instructs Claude to fix the identified issues in a specific task's worktree.
// Format args: objective, task ID, task title, task description, revision round, issues, task ID (for JSON), revision round (for JSON)
const RevisionPromptTemplate = `You are addressing issues identified during review of completed work.

## Original Objective
%s

## Task Being Revised
- Task ID: %s
- Task Title: %s
- Original Description: %s
- Revision Round: %d

## Issues to Address
%s

## Worktree Information
You are working in the same worktree that was used for the original task.
All previous changes from this task are already present.

## Instructions

1. **Review** the issues identified above
2. **Fix** each issue in the codebase
3. **Test** that your fixes don't break existing functionality
4. **Commit** your changes with a clear message describing the fixes

Focus only on addressing the identified issues. Do not refactor or make other changes unless directly related to fixing the issues.

## Completion Protocol

When your revision is complete, you MUST write a completion file:

1. Use Write tool to create ` + "`" + RevisionCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "task_id": "%s",
  "revision_round": %d,
  "issues_addressed": ["Description of issue 1 that was fixed", "Description of issue 2"],
  "summary": "Brief summary of the changes made",
  "files_modified": ["file1.go", "file2.go"],
  "remaining_issues": ["Any issues that could not be fixed"]
}
` + "```" + `

3. List all issues you successfully addressed in issues_addressed
4. Leave remaining_issues empty if all issues were fixed
5. This file signals that your revision is done`

// RevisionState tracks the state of the revision phase within the SynthesisOrchestrator.
// Revision is a sub-phase where identified issues are sent back to task instances for fixing.
type RevisionState struct {
	// Issues contains the issues identified during synthesis that need to be fixed.
	Issues []RevisionIssue

	// RevisionRound is the current revision iteration (starts at 1).
	RevisionRound int

	// MaxRevisions is the maximum number of revision rounds allowed.
	MaxRevisions int

	// TasksToRevise contains the task IDs that need revision.
	TasksToRevise []string

	// RevisedTasks contains the task IDs that have completed revision.
	RevisedTasks []string

	// StartedAt records when the revision phase started.
	StartedAt *time.Time

	// CompletedAt records when the revision phase completed.
	CompletedAt *time.Time
}

// NewRevisionState creates a new RevisionState from the identified issues.
// It extracts the unique task IDs that need revision from the issues.
func NewRevisionState(issues []RevisionIssue) *RevisionState {
	return &RevisionState{
		Issues:        issues,
		RevisionRound: 1,
		MaxRevisions:  DefaultMaxRevisions,
		TasksToRevise: extractTasksToRevise(issues),
		RevisedTasks:  make([]string, 0),
	}
}

// extractTasksToRevise extracts unique task IDs from revision issues.
// It returns a slice of task IDs that have associated issues.
func extractTasksToRevise(issues []RevisionIssue) []string {
	taskSet := make(map[string]bool)
	var tasks []string
	for _, issue := range issues {
		if issue.TaskID != "" && !taskSet[issue.TaskID] {
			taskSet[issue.TaskID] = true
			tasks = append(tasks, issue.TaskID)
		}
	}
	return tasks
}

// IsComplete returns true if all tasks needing revision have been revised.
func (r *RevisionState) IsComplete() bool {
	if r == nil {
		return true
	}
	return len(r.RevisedTasks) >= len(r.TasksToRevise)
}

// revisionTaskCompletion represents a revision task completion notification.
type revisionTaskCompletion struct {
	taskID     string
	instanceID string
	success    bool
	err        string
}

// SynthesisState tracks the current state of synthesis execution.
// This includes the instance performing synthesis and any revision-related state.
type SynthesisState struct {
	// InstanceID is the ID of the Claude instance performing synthesis review.
	InstanceID string

	// AwaitingApproval is true when synthesis has completed but is waiting
	// for user approval before proceeding to revision or consolidation.
	AwaitingApproval bool

	// RevisionRound tracks the current revision iteration (0 for initial synthesis).
	RevisionRound int

	// IssuesFound holds any issues identified during synthesis review.
	IssuesFound []RevisionIssue

	// CompletionFile holds the parsed synthesis completion data when available.
	CompletionFile *SynthesisCompletionFile

	// Revision holds the detailed revision state when in revision sub-phase.
	// This is nil when not in revision mode.
	Revision *RevisionState

	// RunningRevisionTasks tracks currently running revision tasks (taskID -> instanceID).
	RunningRevisionTasks map[string]string

	// RevisionCompletionChan is used to receive revision task completion signals.
	// This is internal and not serialized.
	revisionCompletionChan chan revisionTaskCompletion
}

// RevisionIssue represents an issue identified during synthesis that needs revision.
// This mirrors the type from the orchestrator package for use within phase executors.
type RevisionIssue struct {
	TaskID      string   // Task ID that needs revision (empty for cross-cutting issues)
	Description string   // Description of the issue
	Files       []string // Files affected by the issue
	Severity    string   // "critical", "major", "minor"
	Suggestion  string   // Suggested fix
}

// SynthesisCompletionFile represents the completion report from the synthesis phase.
// This mirrors the type from the orchestrator package for use within phase executors.
type SynthesisCompletionFile struct {
	Status           string          // "complete", "needs_revision"
	RevisionRound    int             // Current round (0 for first synthesis)
	IssuesFound      []RevisionIssue // All issues identified
	TasksAffected    []string        // Task IDs needing revision
	IntegrationNotes string          // Free-form observations about integration
	Recommendations  []string        // Suggestions for consolidation phase
}

// SynthesisOrchestrator manages the synthesis phase of ultra-plan execution.
// It is responsible for:
//   - Creating and starting the synthesis review instance
//   - Monitoring the synthesis instance for completion
//   - Parsing the synthesis completion file to identify issues
//   - Determining whether revision is needed or consolidation can proceed
//   - Handling user approval flow when synthesis is awaiting review
//
// SynthesisOrchestrator implements the PhaseExecutor interface.
type SynthesisOrchestrator struct {
	// phaseCtx holds the shared dependencies for phase execution.
	// This includes the manager, orchestrator, session, logger, and callbacks.
	phaseCtx *PhaseContext

	// logger is a convenience reference to phaseCtx.Logger for structured logging.
	// If phaseCtx.Logger is nil, this will be a NopLogger.
	logger *logging.Logger

	// state holds the current synthesis execution state.
	// Access must be protected by mu.
	state SynthesisState

	// ctx is the execution context, used for cancellation propagation.
	ctx context.Context

	// cancel is the cancel function for ctx.
	// Calling cancel signals the orchestrator to stop execution.
	cancel context.CancelFunc

	// mu protects concurrent access to mutable state.
	mu sync.RWMutex

	// wg tracks background goroutines spawned by this orchestrator.
	// Execute waits on wg before returning.
	// nolint:unused // Reserved for future use when Execute implementation spawns goroutines
	wg sync.WaitGroup

	// cancelled indicates whether Cancel() has been called.
	// This flag is used to ensure Cancel is idempotent.
	cancelled bool
}

// NewSynthesisOrchestrator creates a new SynthesisOrchestrator with the provided dependencies.
// The phaseCtx must be valid (non-nil Manager, Orchestrator, and Session).
// Returns an error if phaseCtx validation fails.
//
// Example usage:
//
//	ctx := &phase.PhaseContext{
//	    Manager:      ultraPlanManager,
//	    Orchestrator: orchestrator,
//	    Session:      session,
//	    Logger:       logger,
//	    Callbacks:    callbacks,
//	}
//	synth, err := phase.NewSynthesisOrchestrator(ctx)
//	if err != nil {
//	    return err
//	}
//	defer synth.Cancel()
//	err = synth.Execute(context.Background())
func NewSynthesisOrchestrator(phaseCtx *PhaseContext) (*SynthesisOrchestrator, error) {
	if err := phaseCtx.Validate(); err != nil {
		return nil, err
	}

	return &SynthesisOrchestrator{
		phaseCtx: phaseCtx,
		logger:   phaseCtx.GetLogger(),
		state:    SynthesisState{},
	}, nil
}

// Phase returns the UltraPlanPhase that this orchestrator handles.
// For SynthesisOrchestrator, this is always PhaseSynthesis.
func (s *SynthesisOrchestrator) Phase() UltraPlanPhase {
	return PhaseSynthesis
}

// Execute runs the synthesis phase logic.
// It creates a synthesis review instance, monitors it for completion,
// parses the results, and determines the next phase (revision or consolidation).
//
// Execute respects the provided context for cancellation. If ctx.Done() is
// signaled or Cancel() is called, Execute returns early.
//
// Returns an error if synthesis fails or is cancelled.
func (s *SynthesisOrchestrator) Execute(ctx context.Context) error {
	// Create a cancellable context derived from the provided context
	s.mu.Lock()
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.mu.Unlock()

	// Notify phase change
	s.notifyPhaseChange(PhaseSynthesis)

	// Build the synthesis prompt
	prompt := s.buildSynthesisPrompt()

	// Create a synthesis instance
	inst, err := s.phaseCtx.Orchestrator.AddInstance(s.phaseCtx.BaseSession, prompt)
	if err != nil {
		s.logger.Error("synthesis failed",
			"error", err.Error(),
			"stage", "create_instance",
		)
		return fmt.Errorf("failed to create synthesis instance: %w", err)
	}

	// Extract instance ID from the returned interface
	instWithID, ok := inst.(interface{ GetID() string })
	if !ok {
		return fmt.Errorf("synthesis instance does not implement GetID")
	}
	instanceID := instWithID.GetID()

	// Store the synthesis instance ID
	s.setInstanceID(instanceID)
	s.phaseCtx.Session.SetSynthesisID(instanceID)

	// Add synthesis instance to the ultraplan group for sidebar display
	if s.phaseCtx.BaseSession != nil {
		sessionType := "ultraplan" // SessionTypeUltraPlan
		if cfg := s.phaseCtx.Session.GetConfig(); cfg != nil && cfg.IsMultiPass() {
			sessionType = "plan_multi" // SessionTypePlanMulti
		}
		if ultraGroup := s.phaseCtx.BaseSession.GetGroupBySessionType(sessionType); ultraGroup != nil {
			ultraGroup.AddInstance(instanceID)
		}
	}

	// Start the instance
	if err := s.phaseCtx.Orchestrator.StartInstance(inst); err != nil {
		s.logger.Error("synthesis failed",
			"error", err.Error(),
			"stage", "start_instance",
		)
		return fmt.Errorf("failed to start synthesis instance: %w", err)
	}

	// Monitor the synthesis instance for completion in a goroutine
	// The monitoring loop will block until completion or cancellation
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.monitorSynthesisInstance(instanceID)
	}()

	// Wait for the monitoring to complete
	s.wg.Wait()

	return nil
}

// Cancel signals the orchestrator to stop any in-progress work.
// This is used for immediate cancellation requests (e.g., user abort).
// After Cancel is called, Execute should return promptly.
// Cancel is safe to call multiple times (idempotent).
func (s *SynthesisOrchestrator) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancelled {
		return
	}
	s.cancelled = true

	if s.cancel != nil {
		s.cancel()
	}
}

// State returns a copy of the current synthesis state.
// This is safe for concurrent access.
func (s *SynthesisOrchestrator) State() SynthesisState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetAwaitingApproval updates the awaiting approval flag.
// This is called when synthesis completes but user approval is needed.
func (s *SynthesisOrchestrator) SetAwaitingApproval(awaiting bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.AwaitingApproval = awaiting
}

// IsAwaitingApproval returns true if synthesis is waiting for user approval.
func (s *SynthesisOrchestrator) IsAwaitingApproval() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.AwaitingApproval
}

// GetIssuesFound returns the issues identified during synthesis.
// Returns nil if no issues were found.
func (s *SynthesisOrchestrator) GetIssuesFound() []RevisionIssue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.state.IssuesFound) == 0 {
		return nil
	}

	// Return a copy to prevent external modification
	issues := make([]RevisionIssue, len(s.state.IssuesFound))
	copy(issues, s.state.IssuesFound)
	return issues
}

// setIssuesFound updates the issues found during synthesis.
// This is called internally when parsing the synthesis completion file.
func (s *SynthesisOrchestrator) setIssuesFound(issues []RevisionIssue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.IssuesFound = issues
}

// GetInstanceID returns the ID of the synthesis instance, or empty string if not started.
func (s *SynthesisOrchestrator) GetInstanceID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.InstanceID
}

// setInstanceID updates the synthesis instance ID.
func (s *SynthesisOrchestrator) setInstanceID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.InstanceID = id
}

// GetRevisionRound returns the current revision round (0 for initial synthesis).
func (s *SynthesisOrchestrator) GetRevisionRound() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.RevisionRound
}

// SetRevisionRound updates the revision round counter.
func (s *SynthesisOrchestrator) SetRevisionRound(round int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.RevisionRound = round
}

// GetCompletionFile returns the parsed synthesis completion file, or nil if not available.
func (s *SynthesisOrchestrator) GetCompletionFile() *SynthesisCompletionFile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.state.CompletionFile == nil {
		return nil
	}

	// Return a copy to prevent external modification
	completion := *s.state.CompletionFile
	return &completion
}

// setCompletionFile updates the completion file data.
func (s *SynthesisOrchestrator) setCompletionFile(completion *SynthesisCompletionFile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.CompletionFile = completion
}

// NeedsRevision returns true if synthesis identified issues that require revision.
// Issues with severity "critical" or "major" (or unspecified severity) require revision.
func (s *SynthesisOrchestrator) NeedsRevision() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, issue := range s.state.IssuesFound {
		if issue.Severity == "critical" || issue.Severity == "major" || issue.Severity == "" {
			return true
		}
	}
	return false
}

// GetIssuesNeedingRevision returns only the issues that require revision.
// This filters to critical/major/unspecified severity issues.
func (s *SynthesisOrchestrator) GetIssuesNeedingRevision() []RevisionIssue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var issues []RevisionIssue
	for _, issue := range s.state.IssuesFound {
		if issue.Severity == "critical" || issue.Severity == "major" || issue.Severity == "" {
			issues = append(issues, issue)
		}
	}
	return issues
}

// Reset clears the orchestrator state for a fresh execution.
// This is useful when restarting the synthesis phase.
func (s *SynthesisOrchestrator) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state = SynthesisState{}
	s.cancelled = false
	s.cancel = nil
	s.ctx = nil
}

// notifyPhaseChange notifies callbacks of a phase change if callbacks are configured.
func (s *SynthesisOrchestrator) notifyPhaseChange(phase UltraPlanPhase) {
	if s.phaseCtx.Callbacks != nil {
		s.phaseCtx.Callbacks.OnPhaseChange(phase)
	}
}

// buildSynthesisPrompt constructs the prompt for the synthesis review instance.
// It includes the objective, list of completed tasks with commit counts, and
// a summary of results from each task instance.
func (s *SynthesisOrchestrator) buildSynthesisPrompt() string {
	session := s.phaseCtx.Session

	var taskList strings.Builder
	var resultsSummary strings.Builder

	completedTasks := session.GetCompletedTasks()
	taskToInstance := session.GetTaskToInstance()
	taskCommitCounts := session.GetTaskCommitCounts()

	for _, taskID := range completedTasks {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		// Extract task info using type assertion for common PlannedTask-like interface
		taskInfo := extractTaskInfo(task)

		// Include commit count in task list
		commitCount := 0
		if count, ok := taskCommitCounts[taskID]; ok {
			commitCount = count
		}

		if commitCount > 0 {
			taskList.WriteString(fmt.Sprintf("- [%s] %s (%d commits)\n", taskID, taskInfo.Title, commitCount))
		} else {
			taskList.WriteString(fmt.Sprintf("- [%s] %s (NO COMMITS - verify this task)\n", taskID, taskInfo.Title))
		}
	}

	// Get summaries from completed instances
	for taskID, instanceID := range taskToInstance {
		task := session.GetTask(taskID)
		inst := s.phaseCtx.Orchestrator.GetInstance(instanceID)
		if task != nil && inst != nil {
			taskInfo := extractTaskInfo(task)
			resultsSummary.WriteString(fmt.Sprintf("### %s\n", taskInfo.Title))
			resultsSummary.WriteString(fmt.Sprintf("Status: %s\n", inst.GetStatus()))

			// Add commit count
			if count, ok := taskCommitCounts[taskID]; ok {
				resultsSummary.WriteString(fmt.Sprintf("Commits: %d\n", count))
			}

			if filesModified := inst.GetFilesModified(); len(filesModified) > 0 {
				resultsSummary.WriteString(fmt.Sprintf("Files modified: %s\n", strings.Join(filesModified, ", ")))
			}
			resultsSummary.WriteString("\n")
		}
	}

	// Also include tasks that completed but are no longer in TaskToInstance
	for _, taskID := range completedTasks {
		if _, inMap := taskToInstance[taskID]; inMap {
			continue // Already processed above
		}
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}
		taskInfo := extractTaskInfo(task)
		resultsSummary.WriteString(fmt.Sprintf("### %s\n", taskInfo.Title))
		resultsSummary.WriteString("Status: completed\n")
		if count, ok := taskCommitCounts[taskID]; ok {
			resultsSummary.WriteString(fmt.Sprintf("Commits: %d\n", count))
		}
		resultsSummary.WriteString("\n")
	}

	// Get current revision round (0 for first synthesis)
	revisionRound := session.GetRevisionRound()

	return fmt.Sprintf(SynthesisPromptTemplate, session.GetObjective(), taskList.String(), resultsSummary.String(), revisionRound)
}

// taskInfo holds extracted task information.
type taskInfo struct {
	ID          string
	Title       string
	Description string
}

// extractTaskInfo extracts task info from a PlannedTask-like interface.
// This handles the type assertion to extract ID, Title, and Description.
func extractTaskInfo(task any) taskInfo {
	info := taskInfo{}

	// Try to get ID
	if t, ok := task.(interface{ GetID() string }); ok {
		info.ID = t.GetID()
	}
	// Try struct field access
	if t, ok := task.(interface{ ID() string }); ok {
		info.ID = t.ID()
	}

	// Try to get Title
	if t, ok := task.(interface{ GetTitle() string }); ok {
		info.Title = t.GetTitle()
	}

	// Try to get Description
	if t, ok := task.(interface{ GetDescription() string }); ok {
		info.Description = t.GetDescription()
	}

	// Fallback: try to access via reflection-like map
	if m, ok := task.(map[string]any); ok {
		if id, ok := m["id"].(string); ok {
			info.ID = id
		}
		if title, ok := m["title"].(string); ok {
			info.Title = title
		}
		if desc, ok := m["description"].(string); ok {
			info.Description = desc
		}
	}

	return info
}

// monitorSynthesisInstance monitors the synthesis instance and handles completion.
// It runs in a loop, checking for the completion file or status changes.
func (s *SynthesisOrchestrator) monitorSynthesisInstance(instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-ticker.C:
			inst := s.phaseCtx.Orchestrator.GetInstance(instanceID)
			if inst == nil {
				// Instance gone, assume complete
				s.onSynthesisComplete()
				return
			}

			// Check for sentinel file first - this is the most reliable completion signal
			// The synthesis agent writes .claudio-synthesis-complete.json when done
			if s.checkForSynthesisCompletionFile(inst) {
				// Auto-advance to consolidation or revision
				s.onSynthesisComplete()
				return
			}

			switch inst.GetStatus() {
			case StatusCompleted:
				// Synthesis fully completed - trigger consolidation or finish
				s.onSynthesisComplete()
				return

			// Note: StatusWaitingInput is intentionally NOT treated as completion.
			// Synthesis may need multiple user interactions.

			case StatusError, StatusTimeout, StatusStuck:
				// Synthesis failed
				s.mu.Lock()
				s.phaseCtx.Session.SetPhase(PhaseFailed)
				s.phaseCtx.Session.SetError(fmt.Sprintf("synthesis failed: %s", inst.GetStatus()))
				s.mu.Unlock()
				_ = s.phaseCtx.Orchestrator.SaveSession()
				s.notifyComplete(false, fmt.Sprintf("synthesis failed: %s", inst.GetStatus()))
				return
			}
		}
	}
}

// checkForSynthesisCompletionFile checks if the synthesis completion sentinel file exists and is valid.
func (s *SynthesisOrchestrator) checkForSynthesisCompletionFile(inst InstanceInterface) bool {
	worktreePath := inst.GetWorktreePath()
	if worktreePath == "" {
		return false
	}

	completionPath := filepath.Join(worktreePath, SynthesisCompletionFileName)
	if _, err := os.Stat(completionPath); err != nil {
		return false // File doesn't exist yet
	}

	// File exists - try to parse it to ensure it's valid
	completion, err := s.parseSynthesisCompletionFile(worktreePath)
	if err != nil {
		// File exists but is invalid/incomplete - might still be writing
		return false
	}

	// File is valid - check status is set
	return completion.Status != ""
}

// parseSynthesisCompletionFile reads and parses a synthesis completion file from a worktree.
func (s *SynthesisOrchestrator) parseSynthesisCompletionFile(worktreePath string) (*SynthesisCompletionFile, error) {
	completionPath := filepath.Join(worktreePath, SynthesisCompletionFileName)
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return nil, err
	}

	var completion SynthesisCompletionFile
	if err := json.Unmarshal(data, &completion); err != nil {
		return nil, fmt.Errorf("failed to parse synthesis completion JSON: %w", err)
	}

	return &completion, nil
}

// onSynthesisComplete handles synthesis completion and triggers revision or consolidation.
// This is called when the synthesis instance writes its completion file or finishes.
func (s *SynthesisOrchestrator) onSynthesisComplete() {
	// Try to parse synthesis completion from sentinel file (preferred) or stdout (fallback)
	synthesisCompletion, issues := s.parseRevisionIssues()

	// Store synthesis completion for later use
	if synthesisCompletion != nil {
		s.mu.Lock()
		s.setCompletionFile(synthesisCompletion)
		s.phaseCtx.Session.SetSynthesisCompletion(synthesisCompletion)
		s.mu.Unlock()
	}

	// Store issues found
	if issues != nil {
		s.setIssuesFound(issues)
	}

	// Save session state
	_ = s.phaseCtx.Orchestrator.SaveSession()

	// Filter to only critical/major issues that need revision
	issuesNeedingRevision := s.GetIssuesNeedingRevision()

	// If there are issues that need revision, start the revision phase
	if len(issuesNeedingRevision) > 0 {
		// Check if we've already had too many revision rounds
		if s.shouldSkipRevision() {
			// Max revisions reached, proceed to consolidation anyway
			s.logger.Info("max revisions reached during onSynthesisComplete, proceeding to consolidation")
			s.CaptureTaskWorktreeInfo()
			_ = s.ProceedToConsolidationOrComplete()
			return
		}

		// Try to start revision if orchestrator supports it
		extOrch, ok := s.phaseCtx.Orchestrator.(SynthesisOrchestratorExtended)
		if !ok {
			s.logger.Warn("orchestrator does not support StartRevision, proceeding to consolidation")
			s.CaptureTaskWorktreeInfo()
			_ = s.ProceedToConsolidationOrComplete()
			return
		}

		if err := extOrch.StartRevision(issuesNeedingRevision); err != nil {
			s.mu.Lock()
			s.phaseCtx.Session.SetPhase(PhaseFailed)
			s.phaseCtx.Session.SetError(fmt.Sprintf("revision failed: %v", err))
			s.mu.Unlock()
			_ = s.phaseCtx.Orchestrator.SaveSession()
			s.notifyComplete(false, fmt.Sprintf("revision failed: %v", err))
		}
		return
	}

	// No issues - capture worktree info and proceed to consolidation or complete
	s.CaptureTaskWorktreeInfo()
	_ = s.ProceedToConsolidationOrComplete()
}

// parseRevisionIssues extracts revision issues from the synthesis completion file (preferred)
// or falls back to parsing stdout output. Returns the full completion struct (if available) and issues.
func (s *SynthesisOrchestrator) parseRevisionIssues() (*SynthesisCompletionFile, []RevisionIssue) {
	synthesisID := s.phaseCtx.Session.GetSynthesisID()
	if synthesisID == "" {
		return nil, nil
	}

	inst := s.phaseCtx.Orchestrator.GetInstance(synthesisID)
	if inst == nil {
		return nil, nil
	}

	// First, try to read from the sentinel file (preferred method)
	worktreePath := inst.GetWorktreePath()
	if worktreePath != "" {
		completion, err := s.parseSynthesisCompletionFile(worktreePath)
		if err == nil && completion != nil {
			// Successfully parsed sentinel file - return the full completion and issues
			return completion, convertToRevisionIssues(completion.IssuesFound)
		}
	}

	// Fallback: parse revision issues from stdout (legacy method)
	mgr := s.phaseCtx.Orchestrator.GetInstanceManager(synthesisID)
	if mgr == nil {
		return nil, nil
	}

	// Try to get output from the manager
	if mgrWithOutput, ok := mgr.(InstanceManagerInterface); ok {
		outputBytes := mgrWithOutput.GetOutput()
		if len(outputBytes) == 0 {
			return nil, nil
		}

		issues, err := parseRevisionIssuesFromOutput(string(outputBytes))
		if err != nil {
			// Log but don't fail - just proceed without revision
			return nil, nil
		}

		return nil, issues
	}

	return nil, nil
}

// convertToRevisionIssues converts the embedded issues to the local RevisionIssue type.
// This is needed because SynthesisCompletionFile.IssuesFound may be a different type.
func convertToRevisionIssues(issues []RevisionIssue) []RevisionIssue {
	if issues == nil {
		return nil
	}
	result := make([]RevisionIssue, len(issues))
	copy(result, issues)
	return result
}

// parseRevisionIssuesFromOutput extracts revision issues from synthesis output.
// It looks for JSON wrapped in <revision_issues></revision_issues> tags.
func parseRevisionIssuesFromOutput(output string) ([]RevisionIssue, error) {
	// Look for <revision_issues>...</revision_issues> tags
	re := regexp.MustCompile(`(?s)<revision_issues>\s*(.*?)\s*</revision_issues>`)
	matches := re.FindStringSubmatch(output)

	if len(matches) < 2 {
		// No revision issues block found - assume no issues
		return nil, nil
	}

	jsonStr := strings.TrimSpace(matches[1])

	// Handle empty array
	if jsonStr == "[]" || jsonStr == "" {
		return nil, nil
	}

	// Parse the JSON array
	var issues []RevisionIssue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
		return nil, fmt.Errorf("failed to parse revision issues JSON: %w", err)
	}

	// Filter to only include issues with actual content
	var validIssues []RevisionIssue
	for _, issue := range issues {
		if issue.Description != "" {
			validIssues = append(validIssues, issue)
		}
	}

	return validIssues, nil
}

// notifyComplete notifies callbacks that the synthesis phase is complete.
func (s *SynthesisOrchestrator) notifyComplete(success bool, summary string) {
	if s.phaseCtx.Callbacks != nil {
		s.phaseCtx.Callbacks.OnComplete(success, summary)
	}
}

// TaskWorktreeInfo holds information about a task's worktree for consolidation.
// This mirrors the type from the orchestrator package for use within phase executors.
type TaskWorktreeInfo struct {
	TaskID       string // Task ID
	TaskTitle    string // Human-readable task title
	WorktreePath string // Path to the git worktree
	Branch       string // Branch name for this task
}

// SynthesisOrchestratorExtended provides extended methods needed by the
// SynthesisOrchestrator for full synthesis-to-consolidation transition.
// This interface extends OrchestratorInterface with synthesis-specific operations.
type SynthesisOrchestratorExtended interface {
	OrchestratorInterface

	// StopInstance stops a running Claude instance
	StopInstance(inst any) error

	// StartRevision begins the revision phase to address identified issues
	StartRevision(issues []RevisionIssue) error

	// StartConsolidation begins the consolidation phase
	StartConsolidation() error
}

// RevisionOrchestratorInterface extends OrchestratorInterface with methods
// needed specifically for the revision phase. This includes the ability to
// create instances in existing worktrees.
type RevisionOrchestratorInterface interface {
	OrchestratorInterface

	// AddInstanceToWorktree creates a new instance using an existing worktree.
	// This is used for revision tasks to work in the same worktree as the original task.
	AddInstanceToWorktree(session any, task string, worktreePath string, branch string) (InstanceInterface, error)

	// StopInstance stops a running Claude instance
	StopInstance(inst any) error

	// RunSynthesis re-runs the synthesis phase after revision completes
	RunSynthesis() error
}

// RevisionSessionInterface provides session methods needed for revision phase.
type RevisionSessionInterface interface {
	UltraPlanSessionInterface

	// GetRevisionState returns the current revision state, or nil if not in revision
	GetRevisionState() *RevisionState

	// SetRevisionState sets the revision state on the session
	SetRevisionState(state *RevisionState)

	// GetRevisionID returns the ID of the current revision instance
	GetRevisionID() string

	// SetRevisionID sets the ID of the current revision instance
	SetRevisionID(id string)
}

// SynthesisSessionExtended provides extended session methods needed for
// synthesis completion and approval handling.
type SynthesisSessionExtended interface {
	UltraPlanSessionInterface

	// GetPlan returns the execution plan
	GetPlan() PlanInterface

	// GetRevision returns the current revision state, or nil if none
	GetRevision() RevisionInterface

	// GetConsolidationMode returns the configured consolidation mode
	GetConsolidationMode() string

	// SetTaskWorktrees sets the task worktree info for consolidation
	SetTaskWorktrees(info []TaskWorktreeInfo)

	// SetCompletedAt marks the session as completed at the given time
	SetCompletedAt(t *time.Time)
}

// PlanInterface provides access to planned tasks.
type PlanInterface interface {
	// GetTasks returns all planned tasks
	GetTasks() []any
}

// RevisionInterface provides access to revision state.
type RevisionInterface interface {
	// GetRevisionRound returns the current revision round
	GetRevisionRound() int

	// GetMaxRevisions returns the maximum allowed revision rounds
	GetMaxRevisions() int
}

// BaseSessionExtended provides extended methods for accessing instances.
type BaseSessionExtended interface {
	BaseSessionInterface

	// GetInstancesExtended returns instances with extended info for worktree lookup
	GetInstancesExtended() []InstanceExtendedInterface
}

// InstanceExtendedInterface provides additional instance methods for worktree capture.
type InstanceExtendedInterface interface {
	InstanceInterface

	// GetTask returns the task associated with this instance
	GetTask() string
}

// CaptureTaskWorktreeInfo captures worktree information for all completed tasks.
// This is used to build context for the consolidation phase.
// The method collects task ID, title, worktree path, and branch name for each
// completed task by matching tasks to their corresponding instances.
//
// The captured information is stored on the session via SetTaskWorktrees
// if the session implements SynthesisSessionExtended.
func (s *SynthesisOrchestrator) CaptureTaskWorktreeInfo() []TaskWorktreeInfo {
	session := s.phaseCtx.Session
	completedTasks := session.GetCompletedTasks()

	var worktreeInfo []TaskWorktreeInfo

	// Get base session for instance lookup
	baseSession, ok := s.phaseCtx.BaseSession.(BaseSessionExtended)
	if !ok {
		s.logger.Warn("base session does not support extended interface, cannot capture worktree info")
		return worktreeInfo
	}

	instances := baseSession.GetInstancesExtended()

	for _, taskID := range completedTasks {
		task := session.GetTask(taskID)
		if task == nil {
			continue
		}

		taskInfo := extractTaskInfo(task)

		// Find the instance for this task
		for _, inst := range instances {
			instTask := inst.GetTask()
			instBranch := inst.GetBranch()

			// Match by task ID in the task field or by slugified title in branch
			if strings.Contains(instTask, taskID) || strings.Contains(instBranch, slugify(taskInfo.Title)) {
				worktreeInfo = append(worktreeInfo, TaskWorktreeInfo{
					TaskID:       taskID,
					TaskTitle:    taskInfo.Title,
					WorktreePath: inst.GetWorktreePath(),
					Branch:       instBranch,
				})
				break
			}
		}
	}

	// Store on session if it supports the extended interface
	if extSession, ok := session.(SynthesisSessionExtended); ok {
		extSession.SetTaskWorktrees(worktreeInfo)
	}

	return worktreeInfo
}

// slugify creates a URL-friendly slug from text.
// This is a local implementation to avoid circular imports.
func slugify(text string) string {
	slug := strings.ToLower(text)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except dashes
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()

	// Limit length
	if len(slug) > 30 {
		slug = slug[:30]
	}

	return slug
}

// ProceedToConsolidationOrComplete moves to consolidation if configured, otherwise completes.
// This is called after synthesis (and any revisions) are done.
//
// If the session has a consolidation mode configured, this will trigger StartConsolidation
// on the extended orchestrator. Otherwise, it marks the session as complete.
//
// Returns an error if consolidation was configured but failed to start.
func (s *SynthesisOrchestrator) ProceedToConsolidationOrComplete() error {
	// Check if session supports extended interface for consolidation mode
	extSession, ok := s.phaseCtx.Session.(SynthesisSessionExtended)
	if !ok {
		// No extended session - just complete
		s.markComplete()
		return nil
	}

	// Check if consolidation is configured
	consolidationMode := extSession.GetConsolidationMode()
	if consolidationMode != "" {
		// Try to get extended orchestrator for consolidation
		extOrch, ok := s.phaseCtx.Orchestrator.(SynthesisOrchestratorExtended)
		if !ok {
			// Orchestrator doesn't support consolidation - just complete
			s.logger.Warn("consolidation configured but orchestrator does not support StartConsolidation")
			s.markComplete()
			return nil
		}

		if err := extOrch.StartConsolidation(); err != nil {
			s.mu.Lock()
			s.phaseCtx.Session.SetPhase(PhaseFailed)
			s.phaseCtx.Session.SetError(fmt.Sprintf("consolidation failed: %v", err))
			s.mu.Unlock()
			_ = s.phaseCtx.Orchestrator.SaveSession()
			s.notifyComplete(false, fmt.Sprintf("consolidation failed: %v", err))
			return err
		}
		return nil
	}

	// No consolidation - mark complete
	s.markComplete()
	return nil
}

// markComplete marks the synthesis phase as complete.
func (s *SynthesisOrchestrator) markComplete() {
	s.mu.Lock()
	s.phaseCtx.Session.SetPhase(PhaseComplete)
	// Set completion time if session supports it
	if extSession, ok := s.phaseCtx.Session.(SynthesisSessionExtended); ok {
		now := time.Now()
		extSession.SetCompletedAt(&now)
	}
	s.mu.Unlock()
	_ = s.phaseCtx.Orchestrator.SaveSession()
	s.notifyComplete(true, "All tasks completed and synthesized")
}

// TriggerConsolidation manually signals that synthesis is done and consolidation should proceed.
// This is called from the TUI when the user indicates they're done with synthesis review.
//
// The method:
//  1. Validates that we're in the synthesis phase
//  2. Clears the awaiting approval flag
//  3. Stops the synthesis instance if still running
//  4. Calls OnSynthesisApproved() to proceed with the flow
//
// Returns an error if not in synthesis phase or if transition fails.
func (s *SynthesisOrchestrator) TriggerConsolidation() error {
	// Only allow triggering from synthesis phase
	phase := s.phaseCtx.Session.GetPhase()
	if phase != PhaseSynthesis {
		return fmt.Errorf("can only trigger consolidation during synthesis phase (current: %s)", phase)
	}

	// Clear the awaiting approval flag
	s.SetAwaitingApproval(false)
	s.phaseCtx.Session.SetSynthesisAwaitingApproval(false)

	// Stop the synthesis instance if it's still running
	synthesisID := s.phaseCtx.Session.GetSynthesisID()
	if synthesisID != "" {
		inst := s.phaseCtx.Orchestrator.GetInstance(synthesisID)
		if inst != nil {
			// Try to stop the instance if orchestrator supports it
			if extOrch, ok := s.phaseCtx.Orchestrator.(SynthesisOrchestratorExtended); ok {
				_ = extOrch.StopInstance(inst)
			}
		}
	}

	// Proceed with approval flow
	s.OnSynthesisApproved()
	return nil
}

// OnSynthesisApproved handles the approval of synthesis results by the user.
// This is called when the user reviews synthesis and decides to proceed.
//
// The method:
//  1. Parses the synthesis completion file to extract any issues
//  2. Determines if revision is needed based on critical/major issues
//  3. Triggers revision if needed and revision limit not exceeded
//  4. Otherwise, captures worktree info and proceeds to consolidation/completion
func (s *SynthesisOrchestrator) OnSynthesisApproved() {
	// Parse completion and issues if not already done
	synthesisCompletion, issues := s.parseRevisionIssues()

	// Store synthesis completion for later use
	if synthesisCompletion != nil {
		s.mu.Lock()
		s.setCompletionFile(synthesisCompletion)
		s.phaseCtx.Session.SetSynthesisCompletion(synthesisCompletion)
		s.mu.Unlock()
	}

	// Store issues found
	if issues != nil {
		s.setIssuesFound(issues)
	}

	// Filter to only critical/major issues that need revision
	issuesNeedingRevision := s.GetIssuesNeedingRevision()

	// If there are issues that need revision, start the revision phase
	if len(issuesNeedingRevision) > 0 {
		// Check if we've already had too many revision rounds
		if s.shouldSkipRevision() {
			// Max revisions reached, proceed to consolidation anyway
			s.logger.Info("max revisions reached, proceeding to consolidation")
			s.CaptureTaskWorktreeInfo()
			_ = s.ProceedToConsolidationOrComplete()
			return
		}

		// Try to start revision if orchestrator supports it
		extOrch, ok := s.phaseCtx.Orchestrator.(SynthesisOrchestratorExtended)
		if !ok {
			s.logger.Warn("orchestrator does not support StartRevision, proceeding to consolidation")
			s.CaptureTaskWorktreeInfo()
			_ = s.ProceedToConsolidationOrComplete()
			return
		}

		if err := extOrch.StartRevision(issuesNeedingRevision); err != nil {
			s.mu.Lock()
			s.phaseCtx.Session.SetPhase(PhaseFailed)
			s.phaseCtx.Session.SetError(fmt.Sprintf("revision failed: %v", err))
			s.mu.Unlock()
			_ = s.phaseCtx.Orchestrator.SaveSession()
			s.notifyComplete(false, fmt.Sprintf("revision failed: %v", err))
		}
		return
	}

	// No issues - capture worktree info and proceed to consolidation or complete
	s.CaptureTaskWorktreeInfo()
	_ = s.ProceedToConsolidationOrComplete()
}

// shouldSkipRevision returns true if we've exceeded the max revision rounds.
func (s *SynthesisOrchestrator) shouldSkipRevision() bool {
	extSession, ok := s.phaseCtx.Session.(SynthesisSessionExtended)
	if !ok {
		return false
	}

	revision := extSession.GetRevision()
	if revision == nil {
		return false
	}

	return revision.GetRevisionRound() >= revision.GetMaxRevisions()
}

// StartRevision begins the revision phase to address identified issues.
// This is called when synthesis identifies critical or major issues that need fixing.
//
// The method:
//  1. Initializes or updates the revision state
//  2. Starts revision tasks for each affected task (in parallel)
//  3. Monitors revision tasks for completion
//  4. Re-runs synthesis once all revisions complete
//
// StartRevision can be called multiple times for subsequent revision rounds,
// up to the configured maximum number of revisions.
func (s *SynthesisOrchestrator) StartRevision(issues []RevisionIssue) error {
	s.notifyPhaseChange(PhaseRevision)

	// Initialize or update revision state
	s.mu.Lock()
	if s.state.Revision == nil {
		s.state.Revision = NewRevisionState(issues)
		now := time.Now()
		s.state.Revision.StartedAt = &now
	} else {
		// Increment revision round
		s.state.Revision.RevisionRound++
		s.state.Revision.Issues = issues
		s.state.Revision.TasksToRevise = extractTasksToRevise(issues)
		s.state.Revision.RevisedTasks = make([]string, 0)
	}

	// Initialize running tasks map and completion channel
	s.state.RunningRevisionTasks = make(map[string]string)
	s.state.revisionCompletionChan = make(chan revisionTaskCompletion, 100)
	s.mu.Unlock()

	// Update session state if it supports the revision interface
	if revSession, ok := s.phaseCtx.Session.(RevisionSessionInterface); ok {
		revSession.SetRevisionState(s.state.Revision)
	}

	// Start revision tasks for each affected task
	for _, taskID := range s.state.Revision.TasksToRevise {
		if err := s.startRevisionTask(taskID); err != nil {
			s.logger.Error("failed to start revision task",
				"task_id", taskID,
				"error", err.Error(),
			)
			if s.phaseCtx.Callbacks != nil {
				s.phaseCtx.Callbacks.OnTaskFailed(taskID, fmt.Sprintf("revision failed: %v", err))
			}
		}
	}

	// Monitor revision tasks in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.monitorRevisionTasks()
	}()

	return nil
}

// startRevisionTask starts a revision task for a specific task.
// It finds the original instance's worktree and creates a new instance there.
func (s *SynthesisOrchestrator) startRevisionTask(taskID string) error {
	task := s.phaseCtx.Session.GetTask(taskID)
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	taskInfo := extractTaskInfo(task)

	// Find the original instance for this task to get its worktree
	var worktreePath, branch string

	// Try to find via base session's extended interface
	if baseSession, ok := s.phaseCtx.BaseSession.(BaseSessionExtended); ok {
		for _, inst := range baseSession.GetInstancesExtended() {
			instTask := inst.GetTask()
			instBranch := inst.GetBranch()

			// Match by task ID in the task field or by slugified title in branch
			if strings.Contains(instTask, taskID) || strings.Contains(instBranch, slugify(taskInfo.Title)) {
				worktreePath = inst.GetWorktreePath()
				branch = instBranch
				break
			}
		}
	}

	if worktreePath == "" {
		return fmt.Errorf("original instance worktree for task %s not found", taskID)
	}

	// Build the revision prompt
	prompt := s.buildRevisionPrompt(task)

	// Get the extended orchestrator interface for worktree-based instance creation
	revOrch, ok := s.phaseCtx.Orchestrator.(RevisionOrchestratorInterface)
	if !ok {
		return fmt.Errorf("orchestrator does not support AddInstanceToWorktree")
	}

	// Create a new instance using the SAME worktree as the original task
	inst, err := revOrch.AddInstanceToWorktree(s.phaseCtx.BaseSession, prompt, worktreePath, branch)
	if err != nil {
		s.logger.Error("revision failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "create_instance",
		)
		return fmt.Errorf("failed to create revision instance for task %s: %w", taskID, err)
	}

	instanceID := inst.GetID()

	// Add revision instance to the ultraplan group for sidebar display
	if s.phaseCtx.BaseSession != nil {
		sessionType := "ultraplan" // SessionTypeUltraPlan
		if cfg := s.phaseCtx.Session.GetConfig(); cfg != nil && cfg.IsMultiPass() {
			sessionType = "plan_multi" // SessionTypePlanMulti
		}
		if ultraGroup := s.phaseCtx.BaseSession.GetGroupBySessionType(sessionType); ultraGroup != nil {
			ultraGroup.AddInstance(instanceID)
		}
	}

	// Store the revision instance ID on the session
	if revSession, ok := s.phaseCtx.Session.(RevisionSessionInterface); ok {
		revSession.SetRevisionID(instanceID)
	}

	// Track the running task
	s.mu.Lock()
	s.state.RunningRevisionTasks[taskID] = instanceID
	s.mu.Unlock()

	// Notify callbacks
	if s.phaseCtx.Callbacks != nil {
		s.phaseCtx.Callbacks.OnTaskStart(taskID, instanceID)
	}

	// Start the instance
	if err := s.phaseCtx.Orchestrator.StartInstance(inst); err != nil {
		s.mu.Lock()
		delete(s.state.RunningRevisionTasks, taskID)
		s.mu.Unlock()
		s.logger.Error("revision failed",
			"task_id", taskID,
			"error", err.Error(),
			"stage", "start_instance",
		)
		return fmt.Errorf("failed to start revision instance for task %s: %w", taskID, err)
	}

	// Monitor the instance for completion in a goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.monitorRevisionTaskInstance(taskID, instanceID)
	}()

	return nil
}

// buildRevisionPrompt creates the prompt for a revision task.
// It includes the original objective, task details, and issues to fix.
func (s *SynthesisOrchestrator) buildRevisionPrompt(task any) string {
	taskInfo := extractTaskInfo(task)

	// Gather issues for this specific task
	var taskIssues []RevisionIssue
	s.mu.RLock()
	if s.state.Revision != nil {
		for _, issue := range s.state.Revision.Issues {
			if issue.TaskID == taskInfo.ID || issue.TaskID == "" {
				taskIssues = append(taskIssues, issue)
			}
		}
	}
	s.mu.RUnlock()

	// Format issues as a readable list
	var issuesStr strings.Builder
	for i, issue := range taskIssues {
		issuesStr.WriteString(fmt.Sprintf("%d. **%s**: %s\n", i+1, issue.Severity, issue.Description))
		if len(issue.Files) > 0 {
			issuesStr.WriteString(fmt.Sprintf("   Files: %s\n", strings.Join(issue.Files, ", ")))
		}
		if issue.Suggestion != "" {
			issuesStr.WriteString(fmt.Sprintf("   Suggestion: %s\n", issue.Suggestion))
		}
		issuesStr.WriteString("\n")
	}

	// Get current revision round (default to 1 if not set)
	revisionRound := 1
	s.mu.RLock()
	if s.state.Revision != nil {
		revisionRound = s.state.Revision.RevisionRound
	}
	s.mu.RUnlock()

	return fmt.Sprintf(RevisionPromptTemplate,
		s.phaseCtx.Session.GetObjective(),
		taskInfo.ID,
		taskInfo.Title,
		taskInfo.Description,
		revisionRound,
		issuesStr.String(),
		taskInfo.ID,   // For completion file JSON
		revisionRound, // For completion file JSON
	)
}

// monitorRevisionTaskInstance monitors a single revision task instance for completion.
func (s *SynthesisOrchestrator) monitorRevisionTaskInstance(taskID, instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return

		case <-ticker.C:
			inst := s.phaseCtx.Orchestrator.GetInstance(instanceID)
			if inst == nil {
				s.logger.Debug("revision instance not found",
					"task_id", taskID,
					"instance_id", instanceID,
				)
				s.sendRevisionCompletion(taskID, instanceID, false, "instance not found")
				return
			}

			// Check for revision completion sentinel file first
			if s.checkForRevisionCompletionFile(inst) {
				// Stop the instance to free up resources
				if revOrch, ok := s.phaseCtx.Orchestrator.(RevisionOrchestratorInterface); ok {
					_ = revOrch.StopInstance(inst)
				}
				s.sendRevisionCompletion(taskID, instanceID, true, "")
				return
			}

			// Fallback: status-based detection
			switch inst.GetStatus() {
			case StatusCompleted:
				s.sendRevisionCompletion(taskID, instanceID, true, "")
				return

			case StatusError, StatusTimeout, StatusStuck:
				s.sendRevisionCompletion(taskID, instanceID, false, string(inst.GetStatus()))
				return
			}
		}
	}
}

// checkForRevisionCompletionFile checks if a revision task has written its completion file.
func (s *SynthesisOrchestrator) checkForRevisionCompletionFile(inst InstanceInterface) bool {
	worktreePath := inst.GetWorktreePath()
	if worktreePath == "" {
		return false
	}

	completionPath := filepath.Join(worktreePath, RevisionCompletionFileName)
	if _, err := os.Stat(completionPath); err != nil {
		return false // File doesn't exist yet
	}

	// File exists - try to parse it to ensure it's valid
	data, err := os.ReadFile(completionPath)
	if err != nil {
		return false
	}

	var completion struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(data, &completion); err != nil {
		return false
	}

	// File is valid if it has a task_id set
	return completion.TaskID != ""
}

// sendRevisionCompletion sends a completion signal to the revision completion channel.
func (s *SynthesisOrchestrator) sendRevisionCompletion(taskID, instanceID string, success bool, errMsg string) {
	s.mu.RLock()
	completionChan := s.state.revisionCompletionChan
	s.mu.RUnlock()

	if completionChan != nil {
		completionChan <- revisionTaskCompletion{
			taskID:     taskID,
			instanceID: instanceID,
			success:    success,
			err:        errMsg,
		}
	}
}

// monitorRevisionTasks monitors all revision tasks and triggers re-synthesis when complete.
func (s *SynthesisOrchestrator) monitorRevisionTasks() {
	for {
		select {
		case <-s.ctx.Done():
			return

		case completion := <-s.state.revisionCompletionChan:
			s.handleRevisionTaskCompletion(completion)

			// Check if all revision tasks are complete
			s.mu.RLock()
			allComplete := s.state.Revision != nil && s.state.Revision.IsComplete()
			s.mu.RUnlock()

			if allComplete {
				s.onRevisionComplete()
				return
			}
		}
	}
}

// handleRevisionTaskCompletion handles a single revision task completion.
func (s *SynthesisOrchestrator) handleRevisionTaskCompletion(completion revisionTaskCompletion) {
	s.mu.Lock()
	delete(s.state.RunningRevisionTasks, completion.taskID)

	if completion.success && s.state.Revision != nil {
		s.state.Revision.RevisedTasks = append(s.state.Revision.RevisedTasks, completion.taskID)
	}
	s.mu.Unlock()

	if completion.success {
		s.logger.Info("revision task completed",
			"task_id", completion.taskID,
		)
		if s.phaseCtx.Callbacks != nil {
			s.phaseCtx.Callbacks.OnTaskComplete(completion.taskID)
		}
	} else {
		s.logger.Warn("revision task failed",
			"task_id", completion.taskID,
			"error", completion.err,
		)
		if s.phaseCtx.Callbacks != nil {
			s.phaseCtx.Callbacks.OnTaskFailed(completion.taskID, completion.err)
		}
	}
}

// onRevisionComplete handles completion of all revision tasks.
// It marks the revision as complete and re-runs synthesis to verify fixes.
func (s *SynthesisOrchestrator) onRevisionComplete() {
	s.mu.Lock()
	now := time.Now()
	if s.state.Revision != nil {
		s.state.Revision.CompletedAt = &now
	}
	s.mu.Unlock()

	s.logger.Info("revision phase complete, re-running synthesis",
		"revision_round", s.state.Revision.RevisionRound,
		"tasks_revised", len(s.state.Revision.RevisedTasks),
	)

	// Update session state
	if revSession, ok := s.phaseCtx.Session.(RevisionSessionInterface); ok {
		revSession.SetRevisionState(s.state.Revision)
	}

	// Re-run synthesis to check if issues are resolved
	revOrch, ok := s.phaseCtx.Orchestrator.(RevisionOrchestratorInterface)
	if !ok {
		s.logger.Warn("orchestrator does not support RunSynthesis, proceeding to consolidation")
		s.CaptureTaskWorktreeInfo()
		_ = s.ProceedToConsolidationOrComplete()
		return
	}

	if err := revOrch.RunSynthesis(); err != nil {
		s.logger.Error("failed to re-run synthesis after revision",
			"error", err.Error(),
		)
		// Fall back to proceeding to consolidation
		s.CaptureTaskWorktreeInfo()
		_ = s.ProceedToConsolidationOrComplete()
	}
}

// GetRevisionState returns a copy of the current revision state.
// Returns nil if not in revision mode.
func (s *SynthesisOrchestrator) GetRevisionState() *RevisionState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.state.Revision == nil {
		return nil
	}

	// Return a copy to prevent external modification
	revision := *s.state.Revision
	if s.state.Revision.Issues != nil {
		revision.Issues = make([]RevisionIssue, len(s.state.Revision.Issues))
		copy(revision.Issues, s.state.Revision.Issues)
	}
	if s.state.Revision.TasksToRevise != nil {
		revision.TasksToRevise = make([]string, len(s.state.Revision.TasksToRevise))
		copy(revision.TasksToRevise, s.state.Revision.TasksToRevise)
	}
	if s.state.Revision.RevisedTasks != nil {
		revision.RevisedTasks = make([]string, len(s.state.Revision.RevisedTasks))
		copy(revision.RevisedTasks, s.state.Revision.RevisedTasks)
	}

	return &revision
}

// IsInRevision returns true if the orchestrator is currently in the revision sub-phase.
func (s *SynthesisOrchestrator) IsInRevision() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Revision != nil && s.state.Revision.CompletedAt == nil
}

// GetRunningRevisionTaskCount returns the number of currently running revision tasks.
func (s *SynthesisOrchestrator) GetRunningRevisionTaskCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.state.RunningRevisionTasks)
}

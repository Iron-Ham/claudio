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
				// Don't auto-advance - set flag and wait for user approval
				s.onSynthesisReady()
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

// onSynthesisReady is called when synthesis writes its completion file.
// Instead of auto-advancing, it sets a flag and waits for user approval.
func (s *SynthesisOrchestrator) onSynthesisReady() {
	// Parse and store synthesis completion data
	synthesisCompletion, issues := s.parseRevisionIssues()

	s.mu.Lock()
	if synthesisCompletion != nil {
		s.setCompletionFile(synthesisCompletion)
		s.phaseCtx.Session.SetSynthesisCompletion(synthesisCompletion)
	}
	if issues != nil {
		s.state.IssuesFound = issues
	}
	s.state.AwaitingApproval = true
	s.phaseCtx.Session.SetSynthesisAwaitingApproval(true)
	s.mu.Unlock()

	// Save session state
	_ = s.phaseCtx.Orchestrator.SaveSession()

	// Notify that user input is needed (but don't advance)
	s.notifyPhaseChange(PhaseSynthesis)
}

// onSynthesisComplete handles synthesis completion.
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

	// Notify completion
	s.notifyComplete(true, "Synthesis review complete")
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

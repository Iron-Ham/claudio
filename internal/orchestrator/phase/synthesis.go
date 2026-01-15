// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"sync"

	"github.com/Iron-Ham/claudio/internal/logging"
)

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

	// TODO: Implementation will be added in subsequent tasks
	// The Execute method will:
	// 1. Build the synthesis prompt using session data
	// 2. Create and start a synthesis instance
	// 3. Monitor for completion (sentinel file or status change)
	// 4. Parse synthesis completion and identify revision issues
	// 5. Update state and notify callbacks

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

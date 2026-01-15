// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"errors"
	"sync"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// ConsolidationState tracks the progress and status of consolidation.
// This mirrors the state tracked by the underlying Consolidator but is
// managed by the orchestrator for phase-level coordination.
type ConsolidationState struct {
	// SubPhase tracks the current sub-phase within consolidation
	SubPhase string

	// CurrentGroup is the index of the group currently being consolidated
	CurrentGroup int

	// TotalGroups is the total number of groups to consolidate
	TotalGroups int

	// CurrentTask is the ID of the task currently being processed
	CurrentTask string

	// GroupBranches holds the branch names created for each group
	GroupBranches []string

	// PRUrls holds the URLs of created pull requests
	PRUrls []string

	// ConflictFiles lists files with merge conflicts (if paused)
	ConflictFiles []string

	// Error holds the error message if consolidation failed
	Error string
}

// ConsolidationOrchestrator manages the consolidation phase of an ultra-plan.
// It coordinates the merging of task branches into group branches and the
// creation of pull requests. The orchestrator delegates the actual consolidation
// work to a Consolidator instance while managing phase lifecycle and cancellation.
//
// The consolidation phase is responsible for:
//   - Building task-to-branch mappings from completed tasks
//   - Creating group branches (stacked or single mode)
//   - Cherry-picking task commits into group branches
//   - Handling merge conflicts (pausing for manual resolution)
//   - Creating pull requests for consolidated branches
type ConsolidationOrchestrator struct {
	// phaseCtx holds the shared dependencies for phase execution
	phaseCtx *PhaseContext

	// logger is a child logger with consolidation phase context
	logger *logging.Logger

	// ctx is the execution context, used for cancellation
	ctx context.Context

	// cancel cancels the execution context
	cancel context.CancelFunc

	// mu protects concurrent access to mutable state
	mu sync.Mutex

	// state tracks consolidation progress
	state *ConsolidationState

	// cancelled indicates whether Cancel() has been called
	cancelled bool

	// running indicates whether Execute() is currently running
	running bool
}

// NewConsolidationOrchestrator creates a new ConsolidationOrchestrator with the
// provided dependencies. The PhaseContext must be validated before use.
//
// The constructor creates a child context for cancellation and initializes
// the consolidation state to its default values.
func NewConsolidationOrchestrator(phaseCtx *PhaseContext) *ConsolidationOrchestrator {
	ctx, cancel := context.WithCancel(context.Background())

	// Create a child logger with phase context
	logger := phaseCtx.GetLogger().WithPhase("consolidation-orchestrator")

	return &ConsolidationOrchestrator{
		phaseCtx: phaseCtx,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		state:    &ConsolidationState{},
	}
}

// Phase returns the UltraPlanPhase that this orchestrator handles.
// ConsolidationOrchestrator handles the PhaseConsolidating phase.
func (o *ConsolidationOrchestrator) Phase() UltraPlanPhase {
	return PhaseConsolidating
}

// Execute runs the consolidation phase logic. It respects the provided context
// for cancellation and returns early if the context is cancelled or Cancel()
// is called.
//
// Execute performs the following steps:
//  1. Validates the phase context
//  2. Sets the session phase to consolidating
//  3. Delegates to the underlying Consolidator
//  4. Updates phase to complete or failed based on result
//
// Returns an error if the phase context is invalid, consolidation fails,
// or the operation is cancelled.
func (o *ConsolidationOrchestrator) Execute(ctx context.Context) error {
	o.mu.Lock()
	if o.cancelled {
		o.mu.Unlock()
		return ErrCancelled
	}
	o.running = true
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.running = false
		o.mu.Unlock()
	}()

	// Validate the phase context
	if err := o.phaseCtx.Validate(); err != nil {
		return err
	}

	o.logger.Info("consolidation phase starting")

	// Set the session phase
	o.phaseCtx.Manager.SetPhase(PhaseConsolidating)

	// Notify callbacks of phase change
	if o.phaseCtx.Callbacks != nil {
		o.phaseCtx.Callbacks.OnPhaseChange(PhaseConsolidating)
	}

	// Check for cancellation before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-o.ctx.Done():
		return ErrCancelled
	default:
	}

	// The actual consolidation work will be delegated to the Consolidator
	// which is created by the main Coordinator. This orchestrator provides
	// the phase lifecycle management and cancellation handling.
	//
	// For now, we signal that consolidation has started. The integration
	// with the existing Consolidator will be completed when the Coordinator
	// is refactored to use phase orchestrators.

	o.logger.Info("consolidation phase ready for execution")

	return nil
}

// Cancel signals the orchestrator to stop any in-progress work.
// This is used for immediate cancellation requests (e.g., user abort).
// After Cancel is called, Execute should return promptly.
// Cancel is safe to call multiple times.
func (o *ConsolidationOrchestrator) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancelled {
		return
	}

	o.cancelled = true
	o.cancel()
	o.logger.Info("consolidation phase cancelled")
}

// State returns a copy of the current consolidation state.
// This is safe to call from any goroutine.
func (o *ConsolidationOrchestrator) State() ConsolidationState {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Return a copy to prevent external mutation
	stateCopy := *o.state
	if o.state.GroupBranches != nil {
		stateCopy.GroupBranches = make([]string, len(o.state.GroupBranches))
		copy(stateCopy.GroupBranches, o.state.GroupBranches)
	}
	if o.state.PRUrls != nil {
		stateCopy.PRUrls = make([]string, len(o.state.PRUrls))
		copy(stateCopy.PRUrls, o.state.PRUrls)
	}
	if o.state.ConflictFiles != nil {
		stateCopy.ConflictFiles = make([]string, len(o.state.ConflictFiles))
		copy(stateCopy.ConflictFiles, o.state.ConflictFiles)
	}
	return stateCopy
}

// SetState updates the consolidation state. This is primarily used for
// restoring state when resuming a paused consolidation.
func (o *ConsolidationOrchestrator) SetState(state ConsolidationState) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Deep copy slices to prevent external mutation
	o.state = &ConsolidationState{
		SubPhase:     state.SubPhase,
		CurrentGroup: state.CurrentGroup,
		TotalGroups:  state.TotalGroups,
		CurrentTask:  state.CurrentTask,
		Error:        state.Error,
	}
	if state.GroupBranches != nil {
		o.state.GroupBranches = make([]string, len(state.GroupBranches))
		copy(o.state.GroupBranches, state.GroupBranches)
	}
	if state.PRUrls != nil {
		o.state.PRUrls = make([]string, len(state.PRUrls))
		copy(o.state.PRUrls, state.PRUrls)
	}
	if state.ConflictFiles != nil {
		o.state.ConflictFiles = make([]string, len(state.ConflictFiles))
		copy(o.state.ConflictFiles, state.ConflictFiles)
	}
}

// IsRunning returns true if Execute() is currently running.
func (o *ConsolidationOrchestrator) IsRunning() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.running
}

// IsCancelled returns true if Cancel() has been called.
func (o *ConsolidationOrchestrator) IsCancelled() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.cancelled
}

// ErrCancelled is returned when the orchestrator is cancelled.
var ErrCancelled = errors.New("consolidation phase cancelled")

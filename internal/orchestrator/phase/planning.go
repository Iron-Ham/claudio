// Package phase provides abstractions for ultra-plan phase execution.
package phase

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// PlanningState tracks the current state of planning execution.
// This includes the instance performing planning and configuration state.
type PlanningState struct {
	// InstanceID is the ID of the Claude instance performing planning.
	// For single-pass, this is a single coordinator; for multi-pass,
	// this tracks the current active planner (use PlanCoordinatorIDs for all).
	InstanceID string

	// Prompt is the generated planning prompt sent to the instance.
	Prompt string

	// MultiPass indicates whether multi-pass planning mode is active.
	MultiPass bool

	// PlanCoordinatorIDs holds the instance IDs for multi-pass planning.
	// Each entry corresponds to a different planning strategy.
	PlanCoordinatorIDs []string

	// AwaitingCompletion is true while waiting for planning instance(s) to finish.
	AwaitingCompletion bool

	// Error holds any error message from planning.
	Error string
}

// PlanningOrchestrator manages the planning phase of ultra-plan execution.
// It is responsible for:
//   - Creating the planning prompt from the session objective
//   - Creating and starting planning instance(s)
//   - Adding instances to the appropriate sidebar group
//   - Setting the CoordinatorID on the session for tracking
//   - Supporting both single-pass and multi-pass planning modes
//
// PlanningOrchestrator implements the PhaseExecutor interface.
type PlanningOrchestrator struct {
	// phaseCtx holds the shared dependencies for phase execution.
	// This includes the manager, orchestrator, session, logger, and callbacks.
	phaseCtx *PhaseContext

	// logger is a convenience reference for structured logging.
	// If phaseCtx.Logger is nil, this will be a NopLogger with phase context.
	logger *logging.Logger

	// state holds the current planning execution state.
	// Access must be protected by mu.
	state PlanningState

	// ctx is the execution context, used for cancellation propagation.
	ctx context.Context

	// cancel is the cancel function for ctx.
	// Calling cancel signals the orchestrator to stop execution.
	cancel context.CancelFunc

	// mu protects concurrent access to mutable state.
	mu sync.RWMutex

	// cancelled indicates whether Cancel() has been called.
	// This flag is used to ensure Cancel is idempotent.
	cancelled bool

	// running indicates whether Execute() is currently in progress.
	running bool
}

// NewPlanningOrchestrator creates a new PlanningOrchestrator with the provided dependencies.
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
//	planner, err := phase.NewPlanningOrchestrator(ctx)
//	if err != nil {
//	    return err
//	}
//	defer planner.Cancel()
//	err = planner.Execute(context.Background())
func NewPlanningOrchestrator(phaseCtx *PhaseContext) (*PlanningOrchestrator, error) {
	if err := phaseCtx.Validate(); err != nil {
		return nil, err
	}

	// Create a child logger with phase context
	logger := phaseCtx.GetLogger().WithPhase("planning-orchestrator")

	return &PlanningOrchestrator{
		phaseCtx: phaseCtx,
		logger:   logger,
		state:    PlanningState{},
	}, nil
}

// Phase returns the UltraPlanPhase that this orchestrator handles.
// For PlanningOrchestrator, this is always PhasePlanning.
func (p *PlanningOrchestrator) Phase() UltraPlanPhase {
	return PhasePlanning
}

// Execute runs the planning phase logic.
// It dispatches to executeSinglePass or executeMultiPass based on the session config.
//
// Execute respects the provided context for cancellation. If ctx.Done() is
// signaled or Cancel() is called, Execute returns early.
//
// Returns an error if planning fails or is cancelled.
func (p *PlanningOrchestrator) Execute(ctx context.Context) error {
	p.mu.Lock()
	if p.cancelled {
		p.mu.Unlock()
		return ErrPlanningCancelled
	}
	p.running = true
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	// Validate the phase context
	if err := p.phaseCtx.Validate(); err != nil {
		return err
	}

	p.logger.Info("planning phase starting")

	// Set the session phase and notify callbacks
	p.phaseCtx.Manager.SetPhase(PhasePlanning)
	if p.phaseCtx.Callbacks != nil {
		p.phaseCtx.Callbacks.OnPhaseChange(PhasePlanning)
	}

	// Check for cancellation before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return ErrPlanningCancelled
	default:
	}

	// Dispatch based on multi-pass configuration
	// Note: The session's Config.MultiPass field determines the mode.
	// Since we only have interface access, we'll need this to be set
	// via the orchestrator state or passed through the phaseCtx.
	// For now, executeSinglePass is the primary implementation.
	// Multi-pass dispatch will be added when multi-pass orchestrator is implemented.

	return p.executeSinglePass(ctx)
}

// executeSinglePass implements single-pass planning mode.
// This creates a single coordinator instance that explores the codebase and generates a plan.
//
// The method:
//  1. Builds the planning prompt using the session objective
//  2. Creates a coordinator instance via the orchestrator interface
//  3. Adds the instance to the appropriate sidebar group
//  4. Stores the CoordinatorID for tracking
//  5. Starts the instance
//  6. Returns without blocking (TUI monitors completion)
func (p *PlanningOrchestrator) executeSinglePass(ctx context.Context) error {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return ErrPlanningCancelled
	default:
	}

	p.logger.Info("executing single-pass planning")

	// Build the planning prompt
	// Note: We need access to the session objective and the prompt template.
	// The objective comes from the UltraPlanSession but we only have the interface.
	// The orchestrator adapter will handle getting the objective and creating the prompt.
	//
	// For now, we signal that planning has started and the integration with
	// prompt building will be completed when the Coordinator is refactored
	// to use phase orchestrators and pass the objective through.

	p.mu.Lock()
	p.state.AwaitingCompletion = true
	p.state.MultiPass = false
	p.mu.Unlock()

	// The actual instance creation will be delegated from the Coordinator.
	// This orchestrator provides the phase lifecycle management and state tracking.
	// The integration with existing RunPlanning() logic will be completed when
	// the Coordinator is refactored to use phase orchestrators.

	p.logger.Info("single-pass planning phase ready for execution")

	return nil
}

// ExecuteWithPrompt runs single-pass planning with an explicit prompt.
// This is the full implementation that creates and starts the planning instance.
// It is designed to be called by the Coordinator after constructing the prompt.
//
// Parameters:
//   - ctx: execution context for cancellation
//   - prompt: the fully constructed planning prompt
//   - baseSession: the base Session for instance creation (passed as any)
//   - getGroup: function to get the instance group for sidebar display
//   - setCoordinatorID: function to set the coordinator ID on the UltraPlanSession
//
// Returns an error if instance creation or startup fails.
func (p *PlanningOrchestrator) ExecuteWithPrompt(
	ctx context.Context,
	prompt string,
	baseSession any,
	getGroup func() any,
	setCoordinatorID func(id string),
) error {
	p.mu.Lock()
	if p.cancelled {
		p.mu.Unlock()
		return ErrPlanningCancelled
	}
	p.running = true
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.ctx.Done():
		return ErrPlanningCancelled
	default:
	}

	p.logger.Info("executing single-pass planning with prompt")

	// Store the prompt in state
	p.mu.Lock()
	p.state.Prompt = prompt
	p.state.AwaitingCompletion = true
	p.state.MultiPass = false
	p.mu.Unlock()

	// Create a coordinator instance for planning
	inst, err := p.phaseCtx.Orchestrator.AddInstance(baseSession, prompt)
	if err != nil {
		p.mu.Lock()
		p.state.Error = err.Error()
		p.state.AwaitingCompletion = false
		p.mu.Unlock()

		p.logger.Error("planning failed",
			"error", err.Error(),
			"stage", "create_instance",
		)
		return fmt.Errorf("failed to create planning instance: %w", err)
	}

	// Extract instance ID using type assertion
	// The instance returned is expected to have an ID field
	instanceID := extractInstanceID(inst)
	if instanceID == "" {
		p.mu.Lock()
		p.state.Error = "failed to extract instance ID"
		p.state.AwaitingCompletion = false
		p.mu.Unlock()

		return errors.New("failed to extract instance ID from created instance")
	}

	p.mu.Lock()
	p.state.InstanceID = instanceID
	p.mu.Unlock()

	// Add planning instance to the ultraplan group for sidebar display
	if getGroup != nil {
		if group := getGroup(); group != nil {
			// The group is expected to have an AddInstance method
			addInstanceToGroup(group, instanceID)
		}
	}

	// Store the coordinator ID in the session
	if setCoordinatorID != nil {
		setCoordinatorID(instanceID)
	}

	// Start the instance
	if err := p.phaseCtx.Orchestrator.StartInstance(inst); err != nil {
		p.mu.Lock()
		p.state.Error = err.Error()
		p.state.AwaitingCompletion = false
		p.mu.Unlock()

		p.logger.Error("planning failed",
			"error", err.Error(),
			"stage", "start_instance",
		)
		return fmt.Errorf("failed to start planning instance: %w", err)
	}

	p.logger.Info("planning instance started",
		"instance_id", instanceID,
	)

	// Return without blocking - TUI will monitor completion
	return nil
}

// extractInstanceID extracts the ID from an instance returned by AddInstance.
// This uses reflection-free type assertion to handle the any type.
func extractInstanceID(inst any) string {
	if inst == nil {
		return ""
	}

	// Try to get ID via interface
	type hasID interface {
		GetID() string
	}
	if i, ok := inst.(hasID); ok {
		return i.GetID()
	}

	// Try direct field access via struct with ID field
	type withID struct {
		ID string
	}
	if i, ok := inst.(*withID); ok {
		return i.ID
	}

	// For the orchestrator Instance type, we access the ID field directly
	// This works because the Instance struct has a public ID field
	// We use a type switch to handle known types
	switch v := inst.(type) {
	case interface{ GetID() string }:
		return v.GetID()
	default:
		// Use reflection as a last resort for structs with ID field
		// This handles *Instance from the orchestrator package
		if idGetter, ok := inst.(interface{ GetID() string }); ok {
			return idGetter.GetID()
		}
	}

	return ""
}

// addInstanceToGroup adds an instance ID to a group.
// This handles the any type returned from getGroup.
func addInstanceToGroup(group any, instanceID string) {
	if group == nil || instanceID == "" {
		return
	}

	// Try interface approach
	type groupWithAdd interface {
		AddInstance(id string)
	}
	if g, ok := group.(groupWithAdd); ok {
		g.AddInstance(instanceID)
	}
}

// Cancel signals the orchestrator to stop any in-progress work.
// This is used for immediate cancellation requests (e.g., user abort).
// After Cancel is called, Execute should return promptly.
// Cancel is safe to call multiple times (idempotent).
func (p *PlanningOrchestrator) Cancel() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cancelled {
		return
	}
	p.cancelled = true

	if p.cancel != nil {
		p.cancel()
	}

	p.logger.Info("planning phase cancelled")
}

// State returns a copy of the current planning state.
// This is safe for concurrent access.
func (p *PlanningOrchestrator) State() PlanningState {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Return a copy to prevent external modification
	stateCopy := p.state

	// Deep copy slices
	if p.state.PlanCoordinatorIDs != nil {
		stateCopy.PlanCoordinatorIDs = make([]string, len(p.state.PlanCoordinatorIDs))
		copy(stateCopy.PlanCoordinatorIDs, p.state.PlanCoordinatorIDs)
	}

	return stateCopy
}

// SetState updates the planning state.
// This is primarily used for restoring state when resuming planning.
func (p *PlanningOrchestrator) SetState(state PlanningState) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Deep copy to prevent external mutation
	p.state = PlanningState{
		InstanceID:         state.InstanceID,
		Prompt:             state.Prompt,
		MultiPass:          state.MultiPass,
		AwaitingCompletion: state.AwaitingCompletion,
		Error:              state.Error,
	}

	if state.PlanCoordinatorIDs != nil {
		p.state.PlanCoordinatorIDs = make([]string, len(state.PlanCoordinatorIDs))
		copy(p.state.PlanCoordinatorIDs, state.PlanCoordinatorIDs)
	}
}

// GetInstanceID returns the ID of the planning instance, or empty string if not started.
func (p *PlanningOrchestrator) GetInstanceID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.InstanceID
}

// IsAwaitingCompletion returns true if planning is waiting for instance(s) to finish.
func (p *PlanningOrchestrator) IsAwaitingCompletion() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.AwaitingCompletion
}

// SetAwaitingCompletion updates the awaiting completion flag.
func (p *PlanningOrchestrator) SetAwaitingCompletion(awaiting bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.AwaitingCompletion = awaiting
}

// IsMultiPass returns true if multi-pass planning mode is active.
func (p *PlanningOrchestrator) IsMultiPass() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.MultiPass
}

// GetPlanCoordinatorIDs returns the instance IDs for multi-pass planning.
// Returns nil for single-pass mode.
func (p *PlanningOrchestrator) GetPlanCoordinatorIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.state.PlanCoordinatorIDs) == 0 {
		return nil
	}

	// Return a copy to prevent external modification
	ids := make([]string, len(p.state.PlanCoordinatorIDs))
	copy(ids, p.state.PlanCoordinatorIDs)
	return ids
}

// SetPlanCoordinatorIDs sets the instance IDs for multi-pass planning.
func (p *PlanningOrchestrator) SetPlanCoordinatorIDs(ids []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ids == nil {
		p.state.PlanCoordinatorIDs = nil
		return
	}

	p.state.PlanCoordinatorIDs = make([]string, len(ids))
	copy(p.state.PlanCoordinatorIDs, ids)
}

// GetError returns the error message if planning failed.
func (p *PlanningOrchestrator) GetError() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state.Error
}

// SetError sets the error message.
func (p *PlanningOrchestrator) SetError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.Error = err
}

// IsRunning returns true if Execute() is currently running.
func (p *PlanningOrchestrator) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// IsCancelled returns true if Cancel() has been called.
func (p *PlanningOrchestrator) IsCancelled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cancelled
}

// Reset clears the orchestrator state for a fresh execution.
// This is useful when restarting the planning phase.
func (p *PlanningOrchestrator) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.state = PlanningState{}
	p.cancelled = false
	p.cancel = nil
	p.ctx = nil
	p.running = false
}

// ErrPlanningCancelled is returned when the orchestrator is cancelled.
var ErrPlanningCancelled = errors.New("planning phase cancelled")

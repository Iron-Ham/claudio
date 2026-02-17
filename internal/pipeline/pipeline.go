package pipeline

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/team"
)

// Pipeline orchestrates multi-phase team execution.
//
// It runs through sequential phases (planning → execution → review →
// consolidation), creating a new [team.Manager] for each phase. The set of
// phases is determined by the [DecomposeResult] produced by [Pipeline.Decompose].
type Pipeline struct {
	mu       sync.RWMutex
	cfg      PipelineConfig
	phase    PipelinePhase
	managers map[PipelinePhase]*team.Manager
	result   *DecomposeResult
	cancel   context.CancelFunc
	started  bool
	pcfg     pipelineConfig
	wg       sync.WaitGroup // tracks the run() goroutine
}

// NewPipeline creates a Pipeline with the given configuration and options.
func NewPipeline(cfg PipelineConfig, opts ...PipelineOption) (*Pipeline, error) {
	if cfg.Bus == nil {
		return nil, errors.New("pipeline: Bus is required")
	}
	if cfg.BaseDir == "" {
		return nil, errors.New("pipeline: BaseDir is required")
	}
	if cfg.Plan == nil {
		return nil, errors.New("pipeline: Plan is required")
	}

	pc := &pipelineConfig{}
	for _, opt := range opts {
		opt(pc)
	}
	if pc.logger == nil {
		pc.logger = logging.NopLogger()
	}

	return &Pipeline{
		cfg:      cfg,
		managers: make(map[PipelinePhase]*team.Manager),
		pcfg:     *pc,
	}, nil
}

// Decompose runs the plan decomposer and stores the result for execution.
// Must be called before Start.
func (p *Pipeline) Decompose(dcfg DecomposeConfig) (*DecomposeResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return nil, errors.New("pipeline: cannot decompose after Start")
	}

	result, err := Decompose(p.cfg.Plan, dcfg)
	if err != nil {
		return nil, err
	}

	p.result = result
	return result, nil
}

// Start begins multi-phase execution. Decompose must be called first.
func (p *Pipeline) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return errors.New("pipeline: already started")
	}
	if p.result == nil {
		return errors.New("pipeline: Decompose must be called before Start")
	}

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.started = true

	// Run pipeline phases in a goroutine so Start returns immediately.
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.run(ctx)
	}()

	return nil
}

// Stop stops all running managers and the pipeline. It is idempotent.
func (p *Pipeline) Stop() error {
	p.mu.Lock()
	if !p.started {
		p.mu.Unlock()
		return nil
	}

	if p.cancel != nil {
		p.cancel()
	}

	for _, m := range p.managers {
		_ = m.Stop()
	}

	p.started = false
	p.mu.Unlock()

	// Wait for the run() goroutine to finish outside the lock.
	p.wg.Wait()
	return nil
}

// Phase returns the pipeline's current phase.
func (p *Pipeline) Phase() PipelinePhase {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.phase
}

// Manager returns the Manager for the given phase, or nil if that phase
// has not been created yet.
func (p *Pipeline) Manager(phase PipelinePhase) *team.Manager {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.managers[phase]
}

// Running returns whether the pipeline is currently started.
func (p *Pipeline) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.started
}

// run executes the pipeline phases sequentially.
func (p *Pipeline) run(ctx context.Context) {
	phasesRun := 0

	// Planning phase.
	if p.result.PlanningTeam != nil {
		if err := p.runPhase(ctx, PhasePlanning, []team.Spec{*p.result.PlanningTeam}); err != nil {
			p.fail(phasesRun)
			return
		}
		phasesRun++
	}

	// Execution phase.
	if len(p.result.ExecutionTeams) > 0 {
		if err := p.runPhase(ctx, PhaseExecution, p.result.ExecutionTeams); err != nil {
			p.fail(phasesRun)
			return
		}
		phasesRun++
	}

	// Debate phase: identify and reconcile file conflicts before review.
	if p.result.ReviewTeam != nil && p.pcfg.enableDebate {
		p.runDebatePhase(ctx, phasesRun)
	}

	// Review phase.
	if p.result.ReviewTeam != nil {
		if err := p.runPhase(ctx, PhaseReview, []team.Spec{*p.result.ReviewTeam}); err != nil {
			p.fail(phasesRun)
			return
		}
		phasesRun++
	}

	// Consolidation phase.
	if p.result.ConsolidationTeam != nil {
		if err := p.runPhase(ctx, PhaseConsolidation, []team.Spec{*p.result.ConsolidationTeam}); err != nil {
			p.fail(phasesRun)
			return
		}
		phasesRun++
	}

	p.setPhase(PhaseDone)
	p.cfg.Bus.Publish(event.NewPipelineCompletedEvent(p.cfg.Plan.ID, true, phasesRun))
}

// runPhase creates a Manager, registers teams, starts execution, and waits
// for all teams to complete.
func (p *Pipeline) runPhase(ctx context.Context, phase PipelinePhase, specs []team.Spec) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	phaseDir := filepath.Join(p.cfg.BaseDir, string(phase))
	m, err := team.NewManager(team.ManagerConfig{
		Bus:     p.cfg.Bus,
		BaseDir: phaseDir,
	}, team.WithHubOptions(p.pcfg.hubOpts...))
	if err != nil {
		return fmt.Errorf("pipeline: creating manager for %s: %w", phase, err)
	}

	// Store the Manager before publishing the phase-changed event so that
	// subscribers calling p.Manager(phase) see the Manager immediately.
	p.mu.Lock()
	p.managers[phase] = m
	p.mu.Unlock()

	prev := p.setPhase(phase)
	p.cfg.Bus.Publish(event.NewPipelinePhaseChangedEvent(
		p.cfg.Plan.ID, string(prev), string(phase),
	))

	for _, spec := range specs {
		if err := m.AddTeam(spec); err != nil {
			return fmt.Errorf("pipeline: adding team %q to %s: %w", spec.ID, phase, err)
		}
	}

	if err := m.Start(ctx); err != nil {
		return fmt.Errorf("pipeline: starting %s: %w", phase, err)
	}

	// Wait for all teams in this phase to reach a terminal state.
	if err := p.waitForCompletion(ctx, m); err != nil {
		_ = m.Stop()
		return err
	}

	// Check if any teams failed.
	for _, s := range m.AllStatuses() {
		if s.Phase == team.PhaseFailed {
			_ = m.Stop()
			return fmt.Errorf("pipeline: team %q failed in %s phase", s.ID, phase)
		}
	}

	_ = m.Stop()
	return nil
}

// waitForCompletion blocks until all teams in the manager have reached a
// terminal phase, or the context is cancelled.
func (p *Pipeline) waitForCompletion(ctx context.Context, m *team.Manager) error {
	done := make(chan struct{}, 1)

	checkDone := func() {
		for _, s := range m.AllStatuses() {
			if !s.Phase.IsTerminal() {
				return
			}
		}
		select {
		case done <- struct{}{}:
		default:
		}
	}

	subID := p.cfg.Bus.Subscribe("team.completed", func(_ event.Event) {
		checkDone()
	})
	defer p.cfg.Bus.Unsubscribe(subID)

	// Check immediately in case all teams already completed.
	checkDone()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// setPhase sets the pipeline's current phase and returns the previous phase.
func (p *Pipeline) setPhase(phase PipelinePhase) PipelinePhase {
	p.mu.Lock()
	defer p.mu.Unlock()
	prev := p.phase
	p.phase = phase
	return prev
}

// runDebatePhase identifies file conflicts between completed execution tasks
// and runs structured debate sessions to reconcile them. Debate results are
// injected into the review team's LeadPrompt. Failures are non-blocking —
// the review phase proceeds regardless.
func (p *Pipeline) runDebatePhase(ctx context.Context, _ int) {
	mgr := p.Manager(PhaseExecution)
	if mgr == nil {
		return
	}

	completedTasks := mgr.CompletedTasks()
	if len(completedTasks) == 0 {
		return
	}

	// Get the execution hub's mailbox for debate messages.
	// Use the first execution team's hub mailbox.
	var firstTeamID string
	for _, s := range mgr.AllStatuses() {
		if s.Role == team.RoleExecution {
			firstTeamID = s.ID
			break
		}
	}
	if firstTeamID == "" {
		return
	}
	t := mgr.Team(firstTeamID)
	if t == nil {
		return
	}

	dc := NewDebateCoordinator(t.Hub().Mailbox(), p.cfg.Bus)
	conflicts := dc.FindConflicts(completedTasks)
	if len(conflicts) == 0 {
		return
	}

	resolutions, err := dc.RunDebates(ctx, conflicts, completedTasks)
	if err != nil {
		// Debate is non-blocking — log and continue to review without debate context.
		p.pcfg.logger.Warn("debate phase failed, continuing to review",
			"plan", p.cfg.Plan.ID, "error", err)
		return
	}

	if len(resolutions) > 0 {
		p.result.ReviewTeam.LeadPrompt += formatDebateContext(resolutions)
	}
}

// fail transitions the pipeline to the Failed phase and publishes a
// PipelineCompletedEvent with the number of phases that ran before the failure.
func (p *Pipeline) fail(phasesRun int) {
	prev := p.setPhase(PhaseFailed)
	p.cfg.Bus.Publish(event.NewPipelinePhaseChangedEvent(
		p.cfg.Plan.ID, string(prev), string(PhaseFailed),
	))
	p.cfg.Bus.Publish(event.NewPipelineCompletedEvent(p.cfg.Plan.ID, false, phasesRun))
}

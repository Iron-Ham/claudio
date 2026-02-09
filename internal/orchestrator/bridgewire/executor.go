package bridgewire

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/pipeline"
	"github.com/Iron-Ham/claudio/internal/scaling"
	"github.com/Iron-Ham/claudio/internal/team"
)

// PipelineExecutor wires a Pipeline's execution-phase teams to real Claude Code
// instances via Bridges. It subscribes to pipeline phase change events and
// creates a Bridge for each team when the execution phase starts.
type PipelineExecutor struct {
	factory  bridge.InstanceFactory
	checker  bridge.CompletionChecker
	bus      *event.Bus
	logger   *logging.Logger
	recorder bridge.SessionRecorder

	pipe       *pipeline.Pipeline
	bridgeOpts []bridge.Option
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	bridges    []*bridge.Bridge
	started    bool
	subID      string
}

// PipelineExecutorConfig holds required dependencies.
type PipelineExecutorConfig struct {
	Factory    bridge.InstanceFactory
	Checker    bridge.CompletionChecker
	Bus        *event.Bus
	Pipeline   *pipeline.Pipeline
	Recorder   bridge.SessionRecorder
	Logger     *logging.Logger
	BridgeOpts []bridge.Option
}

// NewPipelineExecutor creates a PipelineExecutor that will attach bridges
// to execution-phase teams when they start.
func NewPipelineExecutor(cfg PipelineExecutorConfig) (*PipelineExecutor, error) {
	if cfg.Factory == nil {
		return nil, fmt.Errorf("bridgewire: Factory is required")
	}
	if cfg.Checker == nil {
		return nil, fmt.Errorf("bridgewire: Checker is required")
	}
	if cfg.Bus == nil {
		return nil, fmt.Errorf("bridgewire: Bus is required")
	}
	if cfg.Pipeline == nil {
		return nil, fmt.Errorf("bridgewire: Pipeline is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = logging.NopLogger()
	}
	if cfg.Recorder == nil {
		cfg.Recorder = NewSessionRecorder(SessionRecorderDeps{})
	}

	return &PipelineExecutor{
		factory:    cfg.Factory,
		checker:    cfg.Checker,
		bus:        cfg.Bus,
		pipe:       cfg.Pipeline,
		recorder:   cfg.Recorder,
		logger:     cfg.Logger,
		bridgeOpts: cfg.BridgeOpts,
	}, nil
}

// NewPipelineExecutorFromOrch creates a PipelineExecutor using orchestrator
// adapters. This is the production constructor — tests should use
// NewPipelineExecutor directly with mock factory/checker.
func NewPipelineExecutorFromOrch(
	orch *orchestrator.Orchestrator,
	session *orchestrator.Session,
	verifier orchestrator.Verifier,
	bus *event.Bus,
	pipe *pipeline.Pipeline,
	recorder bridge.SessionRecorder,
	logger *logging.Logger,
) (*PipelineExecutor, error) {
	return NewPipelineExecutor(PipelineExecutorConfig{
		Factory:  NewInstanceFactory(orch, session),
		Checker:  NewCompletionChecker(verifier),
		Bus:      bus,
		Pipeline: pipe,
		Recorder: recorder,
		Logger:   logger,
	})
}

// Start subscribes to pipeline phase change events and begins attaching
// bridges when the execution phase starts.
func (pe *PipelineExecutor) Start(ctx context.Context) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	if pe.started {
		return fmt.Errorf("bridgewire: executor already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	pe.ctx = ctx
	pe.cancel = cancel
	pe.started = true

	// Dispatch to a goroutine to avoid deadlock: event.Bus.Publish runs
	// handlers inline, and attachBridges acquires pe.mu. If the publisher
	// already holds a conflicting lock, an inline call would deadlock.
	pe.subID = pe.bus.Subscribe("pipeline.phase_changed", func(e event.Event) {
		pce, ok := e.(event.PipelinePhaseChangedEvent)
		if !ok {
			return
		}
		if pce.CurrentPhase == string(pipeline.PhaseExecution) {
			go pe.attachBridges()
		}
	})

	return nil
}

// Stop cancels all bridges and cleans up subscriptions.
func (pe *PipelineExecutor) Stop() {
	pe.mu.Lock()
	if !pe.started {
		pe.mu.Unlock()
		return
	}
	pe.started = false

	pe.bus.Unsubscribe(pe.subID)
	pe.cancel()

	// Copy the slice so we can release the lock before blocking on bridge.Stop().
	bridges := make([]*bridge.Bridge, len(pe.bridges))
	copy(bridges, pe.bridges)
	pe.bridges = nil
	pe.mu.Unlock()

	for _, b := range bridges {
		b.Stop()
	}
}

// attachBridgesTimeout is the maximum time attachBridges waits for execution
// teams to reach PhaseWorking. The pipeline publishes the phase_changed event
// before teams are added and the Manager is started, so teams may not be
// immediately available or ready.
const attachBridgesTimeout = 5 * time.Second

// attachBridges creates a Bridge for each team in the current execution phase.
// It polls for teams to reach PhaseWorking because the pipeline.phase_changed
// event fires before teams are added to the Manager and started.
func (pe *PipelineExecutor) attachBridges() {
	// Wait for at least one execution team in PhaseWorking without holding the
	// lock. The pipeline publishes pipeline.phase_changed → then AddTeam → then
	// Start, so we must wait for Start to complete before teams are functional.
	deadline := time.Now().Add(attachBridgesTimeout)
	foundWorking := false
	for time.Now().Before(deadline) {
		pe.mu.Lock()
		started := pe.started
		pe.mu.Unlock()
		if !started {
			return
		}

		mgr := pe.pipe.Manager(pipeline.PhaseExecution)
		if mgr != nil {
			for _, s := range mgr.AllStatuses() {
				if s.Role == team.RoleExecution && s.Phase == team.PhaseWorking {
					foundWorking = true
					break
				}
			}
			if foundWorking {
				break
			}
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !foundWorking {
		pe.logger.Warn("bridgewire: timed out waiting for execution teams to reach PhaseWorking",
			"timeout", attachBridgesTimeout)
	}

	pe.mu.Lock()
	defer pe.mu.Unlock()

	if !pe.started {
		return
	}

	mgr := pe.pipe.Manager(pipeline.PhaseExecution)
	if mgr == nil {
		pe.logger.Warn("bridgewire: execution phase manager not yet available")
		return
	}

	statuses := mgr.AllStatuses()
	opts := append([]bridge.Option{bridge.WithLogger(pe.logger)}, pe.bridgeOpts...)

	for _, status := range statuses {
		if status.Role != team.RoleExecution {
			continue
		}

		t := mgr.Team(status.ID)
		if t == nil {
			pe.logger.Warn("bridgewire: team not found", "team", status.ID)
			continue
		}

		b := bridge.New(t, pe.factory, pe.checker, pe.recorder, pe.bus, opts...)
		if err := b.Start(pe.ctx); err != nil {
			pe.logger.Error("bridgewire: failed to start bridge",
				"team", status.ID, "error", err)
			continue
		}

		pe.wireScalingFeedback(t, b)
		pe.bridges = append(pe.bridges, b)
		pe.logger.Info("bridgewire: attached bridge to team", "team", status.ID)
	}
}

// wireScalingFeedback connects the scaling monitor's decisions to the bridge's
// concurrency control. It initialises the bridge and monitor to the team's
// TeamSize, then registers an OnDecision callback that clamps the target to
// [MinInstances, MaxInstances], respects budget exhaustion, and publishes a
// TeamScaledEvent for the TUI.
func (pe *PipelineExecutor) wireScalingFeedback(t *team.Team, b *bridge.Bridge) {
	spec := t.Spec()
	monitor := t.Hub().ScalingMonitor()

	// Initialise bridge and monitor to the team's starting concurrency.
	b.SetMaxConcurrency(spec.TeamSize)
	monitor.SetCurrentInstances(spec.TeamSize)

	minInst := spec.MinInstances
	if minInst <= 0 {
		minInst = spec.TeamSize
	}
	maxInst := spec.MaxInstances // 0 = unlimited

	monitor.OnDecision(func(d scaling.Decision) {
		current := b.MaxConcurrency()
		target := current + d.Delta

		// Clamp to floor.
		if target < minInst {
			target = minInst
		}
		// Clamp to ceiling (0 = no ceiling).
		if maxInst > 0 && target > maxInst {
			target = maxInst
		}

		// Block scale-up when budget is exhausted.
		if target > current && t.BudgetTracker().Exhausted() {
			pe.logger.Info("bridgewire: skipping scale-up due to budget exhaustion",
				"team", spec.ID, "target", target, "current", current)
			return
		}

		if target == current {
			return
		}

		b.SetMaxConcurrency(target)
		monitor.SetCurrentInstances(target)

		pe.bus.Publish(event.NewTeamScaledEvent(
			spec.ID, current, target, d.Reason,
		))

		pe.logger.Info("bridgewire: scaled team",
			"team", spec.ID, "from", current, "to", target, "reason", d.Reason)
	})
}

// Bridges returns the current bridges for testing/inspection.
func (pe *PipelineExecutor) Bridges() []*bridge.Bridge {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	out := make([]*bridge.Bridge, len(pe.bridges))
	copy(out, pe.bridges)
	return out
}

package bridgewire

import (
	"context"
	"fmt"
	"sync"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/pipeline"
	"github.com/Iron-Ham/claudio/internal/team"
)

// PipelineExecutor wires a Pipeline's execution-phase teams to real Claude Code
// instances via Bridges. It subscribes to pipeline phase change events and
// creates a Bridge for each team when the execution phase starts.
type PipelineExecutor struct {
	orch     *orchestrator.Orchestrator
	session  *orchestrator.Session
	verifier orchestrator.Verifier
	bus      *event.Bus
	logger   *logging.Logger
	recorder bridge.SessionRecorder

	pipe    *pipeline.Pipeline
	ctx     context.Context
	cancel  context.CancelFunc
	mu      sync.Mutex
	bridges []*bridge.Bridge
	started bool
	subID   string
}

// PipelineExecutorConfig holds required dependencies.
type PipelineExecutorConfig struct {
	Orchestrator *orchestrator.Orchestrator
	Session      *orchestrator.Session
	Verifier     orchestrator.Verifier
	Bus          *event.Bus
	Pipeline     *pipeline.Pipeline
	Recorder     bridge.SessionRecorder
	Logger       *logging.Logger
}

// NewPipelineExecutor creates a PipelineExecutor that will attach bridges
// to execution-phase teams when they start.
func NewPipelineExecutor(cfg PipelineExecutorConfig) (*PipelineExecutor, error) {
	if cfg.Orchestrator == nil {
		return nil, fmt.Errorf("bridgewire: Orchestrator is required")
	}
	if cfg.Session == nil {
		return nil, fmt.Errorf("bridgewire: Session is required")
	}
	if cfg.Verifier == nil {
		return nil, fmt.Errorf("bridgewire: Verifier is required")
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
		orch:     cfg.Orchestrator,
		session:  cfg.Session,
		verifier: cfg.Verifier,
		bus:      cfg.Bus,
		pipe:     cfg.Pipeline,
		recorder: cfg.Recorder,
		logger:   cfg.Logger,
	}, nil
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

// Coverage: attachBridges requires a real Pipeline with active teams; tested via integration.
// attachBridges creates a Bridge for each team in the current execution phase.
func (pe *PipelineExecutor) attachBridges() {
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

	factory := NewInstanceFactory(pe.orch, pe.session)
	checker := NewCompletionChecker(pe.verifier)

	statuses := mgr.AllStatuses()
	for _, status := range statuses {
		if status.Role != team.RoleExecution {
			continue
		}

		t := mgr.Team(status.ID)
		if t == nil {
			pe.logger.Warn("bridgewire: team not found", "team", status.ID)
			continue
		}

		b := bridge.New(t, factory, checker, pe.recorder, pe.bus,
			bridge.WithLogger(pe.logger),
		)
		if err := b.Start(pe.ctx); err != nil {
			pe.logger.Error("bridgewire: failed to start bridge",
				"team", status.ID, "error", err)
			continue
		}

		pe.bridges = append(pe.bridges, b)
		pe.logger.Info("bridgewire: attached bridge to team", "team", status.ID)
	}
}

// Bridges returns the current bridges for testing/inspection.
func (pe *PipelineExecutor) Bridges() []*bridge.Bridge {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	out := make([]*bridge.Bridge, len(pe.bridges))
	copy(out, pe.bridges)
	return out
}

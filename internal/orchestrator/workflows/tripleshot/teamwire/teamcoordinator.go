// Package teamwire connects the tripleshot workflow to the Orchestration 2.0
// team infrastructure. It exists as a separate subpackage to break the import
// cycle: tripleshot lives inside orchestrator/workflows/, and bridge → team →
// coordination → ... → ultraplan → orchestrator → tripleshot would form a cycle
// if tripleshot imported bridge directly.
//
// TeamCoordinator orchestrates 3 parallel attempt teams + 1 dynamically-added
// judge team using team.Manager, with Bridge instances connecting each team to
// real Claude Code instances.
package teamwire

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/coordination"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	ts "github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// TeamCoordinatorConfig holds required dependencies for creating a TeamCoordinator.
type TeamCoordinatorConfig struct {
	Orchestrator ts.OrchestratorInterface
	BaseSession  ts.SessionInterface
	Task         string
	Config       ts.Config
	Bus          *event.Bus
	BaseDir      string
	Logger       *logging.Logger

	// HubOptions are applied to every team's Hub. Callers typically pass
	// coordination.WithRebalanceInterval(-1) in tests.
	HubOptions []coordination.Option

	// BridgeOptions are applied to every Bridge instance.
	BridgeOptions []bridge.Option
}

// TeamCoordinator orchestrates a team-based triple-shot session using team.Manager.
// Three "attempt" teams run in parallel, each with one task. When all complete,
// a "judge" team is dynamically added to evaluate the results.
type TeamCoordinator struct {
	mu        sync.Mutex
	orch      ts.OrchestratorInterface
	session   ts.SessionInterface // immutable after construction
	bus       *event.Bus
	logger    *logging.Logger
	baseDir   string
	callbacks *ts.CoordinatorCallbacks

	tsManager *ts.Manager   // tripleshot session manager
	tmManager *team.Manager // team orchestration manager
	bridges   []*bridge.Bridge

	hubOpts    []coordination.Option
	bridgeOpts []bridge.Option

	// Judge lifecycle guards
	judgeStarted bool
	wg           sync.WaitGroup

	// Attempt tracking
	completedAttempts int
	attemptTeamIDs    [3]string // maps attempt index → team ID

	// Event subscriptions for cleanup
	teamCompletedSubID   string
	bridgeStartedSubID   string
	bridgeCompletedSubID string

	started bool
	ctx     context.Context //nolint:containedctx // stored for dynamic judge addition
	cancel  context.CancelFunc
}

// NewTeamCoordinator creates a new team-based triple-shot coordinator.
func NewTeamCoordinator(cfg TeamCoordinatorConfig) (*TeamCoordinator, error) {
	if cfg.Orchestrator == nil {
		return nil, fmt.Errorf("teamcoordinator: Orchestrator is required")
	}
	if cfg.BaseSession == nil {
		return nil, fmt.Errorf("teamcoordinator: BaseSession is required")
	}
	if cfg.Bus == nil {
		return nil, fmt.Errorf("teamcoordinator: Bus is required")
	}
	if cfg.BaseDir == "" {
		return nil, fmt.Errorf("teamcoordinator: BaseDir is required")
	}
	if cfg.Task == "" {
		return nil, fmt.Errorf("teamcoordinator: Task is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logging.NopLogger()
	}

	tsSession := ts.NewSession(cfg.Task, cfg.Config)
	tsManager := ts.NewManager(tsSession, logger)

	return &TeamCoordinator{
		orch:       cfg.Orchestrator,
		session:    cfg.BaseSession,
		bus:        cfg.Bus,
		baseDir:    cfg.BaseDir,
		logger:     logger.WithPhase("tripleshot-team"),
		tsManager:  tsManager,
		hubOpts:    cfg.HubOptions,
		bridgeOpts: cfg.BridgeOptions,
	}, nil
}

// Session returns the triple-shot session.
func (tc *TeamCoordinator) Session() *ts.Session {
	return tc.tsManager.Session()
}

// SetCallbacks sets the coordinator callbacks.
func (tc *TeamCoordinator) SetCallbacks(cb *ts.CoordinatorCallbacks) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.callbacks = cb
}

// Start begins the team-based triple-shot execution. It creates a team.Manager
// with 3 attempt teams, starts it, and creates a Bridge per attempt team.
//
// Uses a two-phase approach: register everything under the lock, then release
// the lock before starting bridges. Bridge.Start triggers a claim loop that
// publishes BridgeTaskStartedEvent, which fires onBridgeTaskStarted inline
// (synchronous event bus). If Start held tc.mu through that chain,
// onBridgeTaskStarted would deadlock trying to acquire it.
func (tc *TeamCoordinator) Start(ctx context.Context) error {
	// Phase 1: Register all state under the lock.
	mgr, err := tc.registerStart(ctx)
	if err != nil {
		return err
	}

	// Phase 2: Start bridges outside the lock. Bridge.Start triggers the
	// claim loop, which publishes events that our handlers process. Those
	// handlers acquire tc.mu, so we must not hold it here.
	factory := newAttemptFactory(tc.orch, tc.session)
	checker := newAttemptCompletionChecker()

	for i := range 3 {
		teamID := tc.attemptTeamIDs[i]
		t := mgr.Team(teamID)
		if t == nil { // Coverage: defensive — Team() returns nil if concurrent Stop() clears state.
			tc.abortStart()
			return fmt.Errorf("teamcoordinator: team %q not found after AddTeam", teamID)
		}

		recorder := tc.buildAttemptRecorder(i)
		b := bridge.New(t, factory, checker, recorder, tc.bus, tc.bridgeOpts...)
		if err := b.Start(tc.ctx); err != nil { // Coverage: Bridge.Start only fails if already started.
			tc.abortStart()
			return fmt.Errorf("teamcoordinator: start bridge for %q: %w", teamID, err)
		}

		tc.mu.Lock()
		tc.bridges = append(tc.bridges, b)
		tc.mu.Unlock()
	}

	tc.tsManager.SetPhase(ts.PhaseWorking)
	tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
		if cb.OnPhaseChange != nil {
			cb.OnPhaseChange(ts.PhaseWorking)
		}
	})

	tc.logger.Info("team coordinator started",
		"session_id", tc.tsManager.Session().ID,
		"attempt_teams", 3,
	)

	return nil
}

// registerStart handles Phase 1 of Start: validates, creates the manager and
// teams, subscribes to events, starts the manager, and marks started. Returns
// the manager for Phase 2 (bridge creation). Holds and releases tc.mu internally.
func (tc *TeamCoordinator) registerStart(ctx context.Context) (*team.Manager, error) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.started {
		return nil, fmt.Errorf("teamcoordinator: already started")
	}

	ctx, cancel := context.WithCancel(ctx)
	tc.ctx = ctx
	tc.cancel = cancel

	session := tc.tsManager.Session()
	now := time.Now()
	session.StartedAt = &now

	mgr, err := team.NewManager(team.ManagerConfig{
		Bus:     tc.bus,
		BaseDir: tc.baseDir,
	}, team.WithHubOptions(tc.hubOpts...))
	if err != nil { // Coverage: NewManager only fails if Bus is nil, which we validate in NewTeamCoordinator.
		cancel()
		return nil, fmt.Errorf("teamcoordinator: create manager: %w", err)
	}
	tc.tmManager = mgr

	for i := range 3 {
		teamID := fmt.Sprintf("attempt-%d", i)
		tc.attemptTeamIDs[i] = teamID

		prompt := fmt.Sprintf(ts.AttemptPromptTemplate, session.Task, i)

		spec := team.Spec{
			ID:       teamID,
			Name:     fmt.Sprintf("Attempt %d", i+1),
			Role:     team.RoleExecution,
			TeamSize: 1,
			Tasks: []ultraplan.PlannedTask{
				{
					ID:          fmt.Sprintf("attempt-%d-task", i),
					Title:       fmt.Sprintf("Attempt %d", i+1),
					Description: prompt,
				},
			},
		}
		if err := mgr.AddTeam(spec); err != nil { // Coverage: AddTeam fails on duplicate IDs or after Start; neither applies here.
			cancel()
			return nil, fmt.Errorf("teamcoordinator: add attempt team %d: %w", i, err)
		}
	}

	// Disable retries for attempt tasks. The triple-shot workflow has its
	// own redundancy (3 independent attempts), so retrying individual tasks
	// just creates duplicate instances that appear as a spurious second pass.
	// This is a hard failure because leaving retries enabled risks spawning
	// duplicate instances — see AGENTS.md "Retries disabled for tripleshot tasks".
	for i := range 3 {
		t := mgr.Team(tc.attemptTeamIDs[i])
		if t != nil {
			taskID := fmt.Sprintf("attempt-%d-task", i)
			if err := t.Hub().TaskQueue().SetMaxRetries(taskID, 0); err != nil {
				cancel()
				return nil, fmt.Errorf("teamcoordinator: disable retries for %s: %w", taskID, err)
			}
		}
	}

	tc.subscribeEvents()

	if err := mgr.Start(ctx); err != nil {
		tc.unsubscribeEvents()
		cancel()
		return nil, fmt.Errorf("teamcoordinator: start manager: %w", err)
	}

	tc.started = true
	return mgr, nil
}

// Stop stops all bridges and the team manager. Safe to call multiple times.
func (tc *TeamCoordinator) Stop() {
	tc.mu.Lock()
	if !tc.started {
		tc.mu.Unlock()
		return
	}

	tc.unsubscribeEvents()

	// Copy bridges under lock, then release. This follows the "release locks
	// before blocking" pattern documented in CLAUDE.md — holding the lock
	// through bridge.Stop() (which calls wg.Wait()) would deadlock goroutines
	// that need tc.mu.
	bridges := make([]*bridge.Bridge, len(tc.bridges))
	copy(bridges, tc.bridges)

	tc.cancel()
	tc.started = false
	tc.mu.Unlock()

	// Stop bridges outside the lock.
	for _, b := range bridges {
		b.Stop()
	}

	// Wait for judge goroutine if in flight.
	tc.wg.Wait()

	// startJudge may have created a bridge after our initial snapshot.
	// Re-stop all bridges to ensure nothing is orphaned.
	// Bridge.Stop is idempotent — re-stopping is a no-op.
	tc.mu.Lock()
	allBridges := make([]*bridge.Bridge, len(tc.bridges))
	copy(allBridges, tc.bridges)
	tc.mu.Unlock()
	for _, b := range allBridges {
		b.Stop()
	}

	// Stop the team manager.
	if tc.tmManager != nil {
		if err := tc.tmManager.Stop(); err != nil {
			tc.logger.Warn("failed to stop team manager", "error", err)
		}
	}
}

// abortStart cleans up partial state when the bridge-creation loop in Start()
// encounters an error. It stops already-started bridges, unsubscribes events,
// cancels the context, and marks the coordinator as not started.
func (tc *TeamCoordinator) abortStart() {
	tc.mu.Lock()
	bridges := make([]*bridge.Bridge, len(tc.bridges))
	copy(bridges, tc.bridges)
	tc.bridges = nil
	tc.unsubscribeEvents()
	tc.cancel()
	tc.started = false
	tc.mu.Unlock()
	for _, b := range bridges {
		b.Stop()
	}
}

// subscribeEvents sets up event bus subscriptions. Must be called with tc.mu held.
func (tc *TeamCoordinator) subscribeEvents() {
	tc.teamCompletedSubID = tc.bus.Subscribe("team.completed", func(e event.Event) {
		tce, ok := e.(event.TeamCompletedEvent)
		if !ok { // Coverage: Bus dispatches typed events; assertion can't fail in practice.
			return
		}
		tc.onTeamCompleted(tce)
	})

	tc.bridgeStartedSubID = tc.bus.Subscribe("bridge.task_started", func(e event.Event) {
		bse, ok := e.(event.BridgeTaskStartedEvent)
		if !ok { // Coverage: Bus dispatches typed events; assertion can't fail in practice.
			return
		}
		tc.onBridgeTaskStarted(bse)
	})

	tc.bridgeCompletedSubID = tc.bus.Subscribe("bridge.task_completed", func(e event.Event) {
		bce, ok := e.(event.BridgeTaskCompletedEvent)
		if !ok { // Coverage: Bus dispatches typed events; assertion can't fail in practice.
			return
		}
		tc.onBridgeTaskCompleted(bce)
	})
}

// unsubscribeEvents removes event bus subscriptions. Must be called with tc.mu held.
func (tc *TeamCoordinator) unsubscribeEvents() {
	if tc.teamCompletedSubID != "" {
		tc.bus.Unsubscribe(tc.teamCompletedSubID)
		tc.teamCompletedSubID = ""
	}
	if tc.bridgeStartedSubID != "" {
		tc.bus.Unsubscribe(tc.bridgeStartedSubID)
		tc.bridgeStartedSubID = ""
	}
	if tc.bridgeCompletedSubID != "" {
		tc.bus.Unsubscribe(tc.bridgeCompletedSubID)
		tc.bridgeCompletedSubID = ""
	}
}

// onTeamCompleted handles team.completed events. When all 3 attempt teams
// are done, it dispatches the judge startup to a goroutine to avoid deadlock
// with the synchronous event bus.
func (tc *TeamCoordinator) onTeamCompleted(tce event.TeamCompletedEvent) {
	tc.mu.Lock()
	if !tc.started {
		tc.mu.Unlock()
		return
	}

	// Only count attempt teams (not the judge team).
	attemptIndex := -1
	for i, id := range tc.attemptTeamIDs {
		if tce.TeamID == id {
			attemptIndex = i
			break
		}
	}
	if attemptIndex < 0 {
		tc.mu.Unlock()
		return
	}

	tc.completedAttempts++
	if tc.completedAttempts > 3 {
		// Duplicate event — all attempts already counted.
		tc.mu.Unlock()
		return
	}
	completed := tc.completedAttempts

	// Set attempt status now, while we still hold the lock. The bridge
	// publishes BridgeTaskCompletedEvent (which also sets this status) AFTER
	// gate.Complete returns — but gate.Complete is what triggers this
	// TeamCompletedEvent. If we wait for onBridgeTaskCompleted, startJudge
	// (dispatched below as a goroutine) races with it and may snapshot the
	// status as "working" instead of "completed".
	session := tc.tsManager.Session()
	now := time.Now()
	session.Attempts[attemptIndex].CompletedAt = &now
	if tce.Success {
		session.Attempts[attemptIndex].Status = ts.AttemptStatusCompleted
	} else {
		session.Attempts[attemptIndex].Status = ts.AttemptStatusFailed
	}

	tc.logger.Info("attempt team completed",
		"team_id", tce.TeamID,
		"attempt_index", attemptIndex,
		"success", tce.Success,
		"completed_count", completed,
	)

	// Publish tripleshot-specific event outside the lock.
	tc.mu.Unlock()
	tc.bus.Publish(event.NewTripleShotAttemptCompletedEvent(attemptIndex, tce.TeamID, tce.Success))

	if completed == 3 {
		tc.mu.Lock()
		if tc.judgeStarted || !tc.started {
			tc.mu.Unlock()
			return
		}
		tc.judgeStarted = true
		tc.wg.Add(1)
		tc.mu.Unlock()

		// Dispatch to goroutine to avoid deadlock with synchronous event bus.
		go func() {
			defer tc.wg.Done()
			tc.startJudge()
		}()
	}
}

// onBridgeTaskStarted fires OnAttemptStart or OnJudgeStart callbacks.
func (tc *TeamCoordinator) onBridgeTaskStarted(bse event.BridgeTaskStartedEvent) {
	tc.mu.Lock()
	if !tc.started {
		tc.mu.Unlock()
		return
	}

	for i, id := range tc.attemptTeamIDs {
		if bse.TeamID == id {
			session := tc.tsManager.Session()
			session.Attempts[i].InstanceID = bse.InstanceID
			session.Attempts[i].Status = ts.AttemptStatusWorking
			now := time.Now()
			session.Attempts[i].StartedAt = &now
			tc.mu.Unlock()

			tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
				if cb.OnAttemptStart != nil {
					cb.OnAttemptStart(i, bse.InstanceID)
				}
			})
			return
		}
	}

	if bse.TeamID != "judge" {
		tc.logger.Warn("unexpected team ID in bridge.task_started", "team_id", bse.TeamID)
		tc.mu.Unlock()
		return
	}
	tc.mu.Unlock()
	tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
		if cb.OnJudgeStart != nil {
			cb.OnJudgeStart(bse.InstanceID)
		}
	})
}

// onBridgeTaskCompleted fires attempt/judge completion callbacks.
func (tc *TeamCoordinator) onBridgeTaskCompleted(bce event.BridgeTaskCompletedEvent) {
	tc.mu.Lock()
	if !tc.started {
		tc.mu.Unlock()
		return
	}

	for i, id := range tc.attemptTeamIDs {
		if bce.TeamID == id {
			// Status and CompletedAt may already be set by onTeamCompleted
			// (which fires before this handler for the last-completing attempt).
			// Only update if not already terminal to avoid overwriting the
			// earlier, more accurate CompletedAt timestamp.
			session := tc.tsManager.Session()
			status := session.Attempts[i].Status
			if status != ts.AttemptStatusCompleted && status != ts.AttemptStatusFailed {
				now := time.Now()
				session.Attempts[i].CompletedAt = &now
				if bce.Success {
					session.Attempts[i].Status = ts.AttemptStatusCompleted
				} else {
					session.Attempts[i].Status = ts.AttemptStatusFailed
				}
			}

			if bce.Success {
				tc.mu.Unlock()
				tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
					if cb.OnAttemptComplete != nil {
						cb.OnAttemptComplete(i)
					}
				})
			} else {
				tc.mu.Unlock()
				tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
					if cb.OnAttemptFailed != nil {
						cb.OnAttemptFailed(i, bce.Error)
					}
				})
			}
			return
		}
	}

	// Judge team completion.
	if bce.TeamID != "judge" {
		tc.logger.Warn("unexpected team ID in bridge.task_completed", "team_id", bce.TeamID)
		tc.mu.Unlock()
		return
	}
	tc.mu.Unlock()
	tc.onJudgeCompleted(bce)
}

// startJudge collects attempt completion data, constructs the judge prompt,
// dynamically adds the judge team, and creates its bridge.
func (tc *TeamCoordinator) startJudge() {
	tc.mu.Lock()
	if !tc.started {
		tc.mu.Unlock()
		return
	}

	session := tc.tsManager.Session()
	successCount := session.SuccessfulAttemptCount()

	if successCount < 2 {
		tc.logger.Warn("fewer than 2 attempts succeeded, failing",
			"success_count", successCount,
		)
		session.Error = "fewer than 2 attempts succeeded"
		tc.tsManager.SetPhase(ts.PhaseFailed)
		tc.mu.Unlock()
		tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
			if cb.OnPhaseChange != nil {
				cb.OnPhaseChange(ts.PhaseFailed)
			}
			if cb.OnComplete != nil {
				cb.OnComplete(false, "fewer than 2 attempts succeeded")
			}
		})
		return
	}

	// Snapshot attempt data under the lock to avoid racing with
	// onBridgeTaskCompleted, which writes Attempts[i].Status concurrently.
	type attemptSnap struct {
		status     ts.AttemptStatus
		instanceID string
	}
	var snaps [3]attemptSnap
	for i := range session.Attempts {
		snaps[i] = attemptSnap{
			status:     session.Attempts[i].Status,
			instanceID: session.Attempts[i].InstanceID,
		}
	}
	tc.mu.Unlock()

	tc.tsManager.EmitAllAttemptsReady()

	// Build completion summaries outside the lock (involves I/O).
	type attemptResult struct {
		worktreePath string
		branch       string
	}
	var (
		summaries [3]string
		results   [3]attemptResult
	)
	for i, snap := range snaps {
		if snap.status != ts.AttemptStatusCompleted {
			summaries[i] = fmt.Sprintf("(Attempt %d failed)", i+1)
			continue
		}

		inst := tc.session.GetInstance(snap.instanceID)
		if inst == nil {
			summaries[i] = fmt.Sprintf("(Unable to find instance %s)", snap.instanceID)
			continue
		}

		completion, err := ts.ParseCompletionFile(inst.GetWorktreePath())
		if err != nil {
			tc.logger.Warn("failed to read completion file",
				"attempt_index", i,
				"error", err,
			)
			summaries[i] = fmt.Sprintf("(Unable to read completion file: %v)", err)
		} else {
			summaries[i] = fmt.Sprintf("Status: %s\nSummary: %s\nApproach: %s\nFiles Modified: %v",
				completion.Status, completion.Summary, completion.Approach, completion.FilesModified)
			results[i] = attemptResult{
				worktreePath: inst.GetWorktreePath(),
				branch:       inst.GetBranch(),
			}
		}
	}

	// Write results back and build the judge prompt under the lock.
	tc.mu.Lock()
	for i := range results {
		if results[i].worktreePath != "" {
			session.Attempts[i].WorktreePath = results[i].worktreePath
			session.Attempts[i].Branch = results[i].branch
		}
	}
	judgePrompt := fmt.Sprintf(ts.JudgePromptTemplate,
		session.Task,
		session.Attempts[0].InstanceID, session.Attempts[0].Branch, session.Attempts[0].WorktreePath, summaries[0],
		session.Attempts[1].InstanceID, session.Attempts[1].Branch, session.Attempts[1].WorktreePath, summaries[1],
		session.Attempts[2].InstanceID, session.Attempts[2].Branch, session.Attempts[2].WorktreePath, summaries[2],
	)
	tc.tsManager.SetPhase(ts.PhaseEvaluating)
	tc.mu.Unlock()

	tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
		if cb.OnPhaseChange != nil {
			cb.OnPhaseChange(ts.PhaseEvaluating)
		}
	})

	// Reorganize the TUI group hierarchy: move attempt instances into an
	// "Implementers" sub-group so the TUI can auto-collapse them when the
	// judge starts. This mirrors the legacy coordinator's StartJudge() logic.
	tc.reorganizeGroupForJudge()

	// Add judge team dynamically.
	judgeSpec := team.Spec{
		ID:       "judge",
		Name:     "Judge",
		Role:     team.RoleReview,
		TeamSize: 1,
		DependsOn: []string{
			tc.attemptTeamIDs[0],
			tc.attemptTeamIDs[1],
			tc.attemptTeamIDs[2],
		},
		Tasks: []ultraplan.PlannedTask{
			{
				ID:          "judge-task",
				Title:       "Evaluate Solutions",
				Description: judgePrompt,
			},
		},
	}

	// NOTE: The failure blocks below intentionally don't use failJudge() because
	// the judge hasn't started yet. failJudge() publishes TripleShotJudgeCompletedEvent,
	// which is inappropriate before the judge team exists.

	// Coverage: AddTeamDynamic fails if manager is stopped or context is cancelled;
	// both require tight timing that's impractical to test deterministically.
	if err := tc.tmManager.AddTeamDynamic(tc.ctx, judgeSpec); err != nil {
		tc.logger.Error("failed to add judge team", "error", err)
		tc.mu.Lock()
		session.Error = fmt.Sprintf("failed to add judge team: %v", err)
		tc.tsManager.SetPhase(ts.PhaseFailed)
		tc.mu.Unlock()
		tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
			if cb.OnPhaseChange != nil {
				cb.OnPhaseChange(ts.PhaseFailed)
			}
			if cb.OnComplete != nil {
				cb.OnComplete(false, session.Error)
			}
		})
		return
	}

	// Create and start bridge for judge team.
	judgeTeam := tc.tmManager.Team("judge")
	if judgeTeam == nil { // Coverage: defensive — Team() can't return nil for IDs we just AddTeamDynamic'd.
		tc.logger.Error("judge team not found after AddTeamDynamic")
		tc.mu.Lock()
		session.Error = "judge team not found after AddTeamDynamic"
		tc.tsManager.SetPhase(ts.PhaseFailed)
		tc.mu.Unlock()
		tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
			if cb.OnPhaseChange != nil {
				cb.OnPhaseChange(ts.PhaseFailed)
			}
			if cb.OnComplete != nil {
				cb.OnComplete(false, session.Error)
			}
		})
		return
	}

	// Disable retries for the judge task — same rationale as attempt tasks.
	// Hard failure: leaving retries enabled could spawn a duplicate judge.
	if err := judgeTeam.Hub().TaskQueue().SetMaxRetries("judge-task", 0); err != nil {
		tc.logger.Error("failed to disable retries for judge task", "error", err)
		tc.mu.Lock()
		session.Error = fmt.Sprintf("failed to disable retries for judge task: %v", err)
		tc.tsManager.SetPhase(ts.PhaseFailed)
		tc.mu.Unlock()
		tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
			if cb.OnPhaseChange != nil {
				cb.OnPhaseChange(ts.PhaseFailed)
			}
			if cb.OnComplete != nil {
				cb.OnComplete(false, session.Error)
			}
		})
		return
	}

	factory := newAttemptFactory(tc.orch, tc.session)
	checker := newJudgeCompletionChecker()
	recorder := tc.buildJudgeRecorder()

	b := bridge.New(judgeTeam, factory, checker, recorder, tc.bus, tc.bridgeOpts...)
	if err := b.Start(tc.ctx); err != nil { // Coverage: Bridge.Start only fails if already started.
		tc.logger.Error("failed to start judge bridge", "error", err)
		tc.mu.Lock()
		session.Error = fmt.Sprintf("failed to start judge bridge: %v", err)
		tc.tsManager.SetPhase(ts.PhaseFailed)
		tc.mu.Unlock()
		tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
			if cb.OnPhaseChange != nil {
				cb.OnPhaseChange(ts.PhaseFailed)
			}
			if cb.OnComplete != nil {
				cb.OnComplete(false, session.Error)
			}
		})
		return
	}

	tc.mu.Lock()
	tc.bridges = append(tc.bridges, b)
	tc.mu.Unlock()

	tc.logger.Info("judge team added and bridge started")
}

// reorganizeGroupForJudge mirrors the legacy coordinator's group restructuring
// from StartJudge(). It creates an "Implementers" sub-group within the tripleshot
// group, moves the 3 attempt instances into it, and sets ImplementersGroupID on the
// session so the TUI can auto-collapse them.
//
// Group mutation safety: this method runs from startJudge() after all 3 attempts
// have completed. At this point no new BridgeTaskStartedEvents will fire for
// attempt teams (they're done), so the TUI's handleTeamwireAttemptStarted won't
// race on AddInstance. The production GroupInterface (groupAdapter wrapping
// InstanceGroup) is not internally synchronized, but the sequential execution
// model of Bubble Tea's Update loop means TUI-side group mutations are serialized.
func (tc *TeamCoordinator) reorganizeGroupForJudge() {
	// Snapshot session fields under the lock to follow the documented
	// lock discipline (all session field reads must hold tc.mu).
	tc.mu.Lock()
	session := tc.tsManager.Session()
	groupID := session.GroupID
	sessionID := session.ID
	tc.mu.Unlock()

	group := tc.session.GetGroup(groupID)
	if group == nil {
		tc.logger.Warn("tripleshot group not found, skipping implementer reorganization",
			"group_id", groupID,
		)
		return
	}

	subGroupable, ok := group.(ts.GroupWithSubGroupsInterface)
	if !ok {
		tc.logger.Warn("tripleshot group does not support sub-groups, skipping implementer reorganization",
			"group_id", groupID,
		)
		return
	}

	implementersGroup := subGroupable.GetOrCreateSubGroup(sessionID+"-implementers", "Implementers")
	if implementersGroup == nil {
		tc.logger.Warn("failed to create implementers sub-group",
			"session_id", sessionID,
			"group_id", groupID,
		)
		return
	}

	// Move existing instances (the 3 implementers) from parent → sub-group.
	for _, instID := range group.GetInstances() {
		implementersGroup.AddInstance(instID)
	}
	group.SetInstances(nil)

	tc.mu.Lock()
	session.ImplementersGroupID = implementersGroup.GetID()
	tc.mu.Unlock()

	tc.logger.Debug("reorganized group for judge",
		"implementers_group_id", implementersGroup.GetID(),
	)
}

// onJudgeCompleted handles the judge task completion.
func (tc *TeamCoordinator) onJudgeCompleted(bce event.BridgeTaskCompletedEvent) {
	session := tc.tsManager.Session()

	if !bce.Success {
		tc.failJudge(session, bce.TeamID, fmt.Sprintf("judge failed: %s", bce.Error))
		return
	}

	judgeInst := tc.session.GetInstance(bce.InstanceID)
	if judgeInst == nil {
		tc.logger.Error("judge instance not found", "instance_id", bce.InstanceID)
		tc.failJudge(session, bce.TeamID, "judge instance not found")
		return
	}

	evaluation, err := ts.ParseEvaluationFile(judgeInst.GetWorktreePath())
	if err != nil {
		tc.failJudge(session, bce.TeamID, fmt.Sprintf("failed to parse evaluation: %v", err))
		return
	}

	// Hold tc.mu for session mutations and winner-info snapshot to synchronize
	// with GetWinningBranch. Snapshot winner data under the lock so we don't
	// read session.Attempts outside it.
	tc.mu.Lock()
	session.JudgeID = bce.InstanceID
	now := time.Now()
	session.CompletedAt = &now
	tc.tsManager.SetEvaluation(evaluation)
	tc.tsManager.SetPhase(ts.PhaseComplete)

	summary := fmt.Sprintf("Strategy: %s. Reasoning: %s", evaluation.MergeStrategy, evaluation.Reasoning)
	if evaluation.MergeStrategy == ts.MergeStrategySelect &&
		evaluation.WinnerIndex >= 0 && evaluation.WinnerIndex < 3 {
		winner := session.Attempts[evaluation.WinnerIndex]
		summary = fmt.Sprintf("Selected attempt %d (branch: %s). Reasoning: %s",
			evaluation.WinnerIndex+1, winner.Branch, evaluation.Reasoning)
	}
	tc.mu.Unlock()

	tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
		if cb.OnEvaluationReady != nil {
			cb.OnEvaluationReady(evaluation)
		}
		if cb.OnPhaseChange != nil {
			cb.OnPhaseChange(ts.PhaseComplete)
		}
		if cb.OnComplete != nil {
			cb.OnComplete(true, summary)
		}
	})

	tc.bus.Publish(event.NewTripleShotJudgeCompletedEvent(bce.TeamID, true))
}

// failJudge sets the session error, transitions to PhaseFailed, fires callbacks,
// and publishes TripleShotJudgeCompletedEvent. Used by onJudgeCompleted for all
// failure paths to ensure consistent behavior.
func (tc *TeamCoordinator) failJudge(session *ts.Session, teamID, reason string) {
	tc.mu.Lock()
	session.Error = reason
	tc.tsManager.SetPhase(ts.PhaseFailed)
	tc.mu.Unlock()
	tc.notifyCallbacks(func(cb *ts.CoordinatorCallbacks) {
		if cb.OnPhaseChange != nil {
			cb.OnPhaseChange(ts.PhaseFailed)
		}
		if cb.OnComplete != nil {
			cb.OnComplete(false, reason)
		}
	})
	tc.bus.Publish(event.NewTripleShotJudgeCompletedEvent(teamID, false))
}

// buildAttemptRecorder creates a SessionRecorder for an attempt team.
func (tc *TeamCoordinator) buildAttemptRecorder(attemptIndex int) bridge.SessionRecorder {
	return newSessionRecorder(sessionRecorderDeps{
		OnAssign: func(_, instanceID string) {
			tc.logger.Debug("attempt task assigned",
				"attempt_index", attemptIndex,
				"instance_id", instanceID,
			)
		},
		OnComplete: func(taskID string, _ int) {
			tc.logger.Info("attempt task completed",
				"attempt_index", attemptIndex,
				"task_id", taskID,
			)
		},
		OnFailure: func(taskID, reason string) {
			tc.logger.Warn("attempt task failed",
				"attempt_index", attemptIndex,
				"task_id", taskID,
				"reason", reason,
			)
		},
	})
}

// buildJudgeRecorder creates a SessionRecorder for the judge team.
func (tc *TeamCoordinator) buildJudgeRecorder() bridge.SessionRecorder {
	return newSessionRecorder(sessionRecorderDeps{
		OnAssign: func(_, instanceID string) {
			tc.logger.Debug("judge task assigned", "instance_id", instanceID)
		},
		OnComplete: func(taskID string, _ int) {
			tc.logger.Info("judge task completed", "task_id", taskID)
		},
		OnFailure: func(taskID, reason string) {
			tc.logger.Warn("judge task failed", "task_id", taskID, "reason", reason)
		},
	})
}

// notifyCallbacks invokes the callback function with the current callbacks.
func (tc *TeamCoordinator) notifyCallbacks(fn func(*ts.CoordinatorCallbacks)) {
	tc.mu.Lock()
	cb := tc.callbacks
	tc.mu.Unlock()
	if cb != nil {
		fn(cb)
	}
}

// GetWinningBranch returns the branch name of the winning solution.
// Returns empty string if evaluation is not complete or strategy is not select.
// Holds tc.mu to synchronize with session mutations in onJudgeCompleted/failJudge.
func (tc *TeamCoordinator) GetWinningBranch() string {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	session := tc.tsManager.Session()
	if session.Evaluation == nil {
		return ""
	}
	if session.Evaluation.MergeStrategy != ts.MergeStrategySelect {
		return ""
	}
	if session.Evaluation.WinnerIndex < 0 || session.Evaluation.WinnerIndex >= 3 {
		return ""
	}
	return session.Attempts[session.Evaluation.WinnerIndex].Branch
}

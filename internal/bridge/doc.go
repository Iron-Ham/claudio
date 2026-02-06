// Package bridge connects team Hubs to real Claude Code instance infrastructure.
//
// A Bridge claims tasks from a team's approval gate as they become ready,
// creates a Claude Code instance (worktree + tmux) for each task, monitors
// the instance for completion (sentinel file polling), and reports the
// outcome back to the team's task queue.
//
// One Bridge is created per team. Each Bridge runs independently, claiming
// from its own team's queue. The [bridgewire.PipelineExecutor] attaches
// Bridges to teams when the pipeline transitions to an execution phase.
//
// The Bridge uses narrow interfaces ([InstanceFactory], [CompletionChecker],
// [SessionRecorder]) so that the concrete orchestrator types remain
// encapsulated. Tests can substitute mock implementations.
//
// Lifecycle:
//
//	b := bridge.New(team, factory, checker, recorder, bus)
//	b.Start(ctx)    // spawns claim loop + per-task monitors
//	// ... tasks execute ...
//	b.Stop()         // cancels context, waits for all goroutines
package bridge

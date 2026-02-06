// Package team provides multi-team orchestration for Claudio.
//
// It builds on the coordination.Hub (Phase 1) by running multiple teams in
// parallel, each with its own Hub instance, and adds inter-team message
// routing, per-team budget management, and team-level dependency ordering.
//
// # Architecture
//
// The central type is [Manager], which owns the full multi-team lifecycle:
//
//   - Teams are registered with [Manager.AddTeam] before starting.
//   - [Manager.Start] launches teams respecting inter-team dependencies.
//   - Each team wraps a [coordination.Hub] with team-specific metadata and budget.
//   - A [Router] delivers inter-team messages via each team's Hub mailbox.
//   - A [BudgetTracker] per team monitors resource consumption via the event bus.
//
// # Dependency Flow
//
// Teams declare dependencies on other teams via [Spec.DependsOn]. On start,
// teams with no dependencies begin immediately. When a team completes, the
// manager checks if any blocked teams now have all dependencies satisfied
// and starts those.
//
// # Event Integration
//
// All teams share a single [event.Bus]. Team lifecycle events
// (TeamCreatedEvent, TeamPhaseChangedEvent, TeamCompletedEvent,
// TeamBudgetExhaustedEvent) and inter-team messages (InterTeamMessageEvent)
// are published for TUI reactivity and monitoring.
package team

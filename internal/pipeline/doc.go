// Package pipeline provides plan decomposition and multi-phase team orchestration.
//
// It is Phase 3 of the Orchestrator of Orchestrators (issue #637), building on
// the coordination.Hub (Phase 1) and team.Manager (Phase 2) layers.
//
// # Plan Decomposition
//
// [Decompose] takes a [ultraplan.PlanSpec] and groups its tasks into teams
// based on file affinity. Tasks that share files are placed in the same team
// using a union-find algorithm, ensuring that file conflicts stay within a
// single team rather than across teams. The result is a [DecomposeResult]
// containing execution teams plus optional planning, review, and consolidation
// team specs.
//
// # Pipeline Orchestration
//
// [Pipeline] runs a multi-phase session: planning → execution → review →
// consolidation → done. Each phase creates its own [team.Manager] and runs
// the teams for that phase to completion before advancing. Phase transitions
// publish events on the shared [event.Bus] for TUI reactivity.
//
// # Usage
//
//	p, _ := pipeline.NewPipeline(pipeline.PipelineConfig{
//	    Bus:     bus,
//	    BaseDir: "/tmp/session",
//	    Plan:    plan,
//	})
//	result, _ := p.Decompose(pipeline.DecomposeConfig{
//	    MaxTeamSize: 5,
//	    ReviewTeam:  true,
//	})
//	_ = p.Start(ctx)
//	defer p.Stop()
package pipeline

// Package step provides step introspection and restart logic for ultra-plan workflows.
//
// This package provides:
//   - StepInfo type for describing workflow steps
//   - Resolver for introspecting step information from instance IDs
//   - Restarter for restarting any step type (planning, tasks, synthesis, etc.)
//   - Interfaces that decouple step logic from the Coordinator implementation
//
// The Resolver queries both session state and phase orchestrators to resolve
// instance IDs to step information, providing fallback lookups for robustness.
//
// The Restarter handles all step types: Planning, PlanManager, Task, Synthesis,
// Revision, Consolidation, and GroupConsolidator. It stops existing instances
// and starts fresh ones with proper state reset.
package step

// Package step provides step introspection and restart logic for ultra-plan workflows.
//
// This package is intended to contain:
//   - StepInfo type and resolution logic (GetStepInfo)
//   - RestartStep dispatch and phase-specific restarters
//   - Step types (StepTypePlanning, StepTypeTask, etc.)
//
// Currently, the step management logic remains in the Coordinator
// (internal/orchestrator/coordinator.go) due to tight coupling with:
//   - Session state (UltraPlanSession)
//   - All phase orchestrators (Planning, Execution, Synthesis, Consolidation)
//   - Coordinator's internal state
//
// Future refactoring could extract this logic here by defining interfaces
// for the required dependencies.
//
// The step types (StepType, StepInfo) are currently defined in
// internal/orchestrator/ultraplan.go.
package step

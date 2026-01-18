// Package ultraplan provides a parallel task execution system for decomposing
// complex objectives into independent, concurrently-executed tasks.
//
// Ultra-Plan enables automated task planning, parallel execution, and
// result consolidation for large-scale code modifications. It decomposes
// high-level objectives into smaller tasks that can run in parallel
// across multiple Claude instances.
//
// # Main Types
//
// Planning:
//   - [PlanSpec]: Complete execution plan with tasks and dependency graph
//   - [PlannedTask]: Individual task with ID, description, files, and dependencies
//   - [TaskComplexity]: Enum for task complexity (Low, Medium, High)
//
// Session Management:
//   - [Session]: Ultra-plan session tracking plan, phase, and task mapping
//   - [Manager]: Coordinates session state, events, and plan parsing/validation
//   - [Config]: Session configuration for models, parallelism, and behavior
//
// Phases:
//   - [Phase]: Current execution phase (Planning, Executing, Synthesis, etc.)
//   - [PhaseChangeEvent]: Event emitted when phase transitions occur
//
// Validation:
//   - [ValidationResult]: Collection of validation messages
//   - [ValidationMessage]: Individual validation issue with severity
//   - [ValidationSeverity]: Error, Warning, or Info
//
// Consolidation:
//   - [ConsolidationMode]: "stacked" (one PR per group) or "single" (one PR total)
//   - [ConsolidationPhase]: Sub-phases during branch merging and PR creation
//   - [ConsolidatorState]: Tracks progress, branches, and PRs created
//
// # Phase Lifecycle
//
// Ultra-Plan sessions progress through these phases:
//
//	PhasePlanning → PhasePlanSelection → PhaseRefresh (optional)
//	    → PhaseExecuting → PhaseSynthesis → PhaseRevision (if needed)
//	    → PhaseConsolidating → PhaseComplete / PhaseFailed
//
// # Execution Model
//
// Tasks are organized into execution groups based on their dependencies.
// Tasks within a group run in parallel, while groups execute sequentially.
// The dependency graph is topologically sorted to determine group order.
//
// # Basic Usage
//
//	// Create a session with an objective
//	config := ultraplan.DefaultConfig()
//	session := ultraplan.NewSession("Refactor authentication module", config)
//
//	// Parse and validate a plan from Claude output
//	spec, err := ultraplan.ParsePlanFromOutput(claudeOutput, objective)
//	if err != nil {
//	    return err
//	}
//
//	result, err := ultraplan.ValidatePlan(spec)
//	if err != nil {
//	    return err
//	}
//	if result.HasErrors() {
//	    return fmt.Errorf("plan validation failed: %v", result.Errors())
//	}
//
//	// Set the plan and begin execution
//	session.SetPlan(spec)
//	session.SetPhase(ultraplan.PhaseExecuting)
package ultraplan

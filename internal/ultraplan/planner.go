// Package ultraplan provides types and utilities for Ultra-Plan orchestration.
package ultraplan

import (
	"fmt"
)

// PlanningPromptTemplate is the prompt used for the planning phase.
// It guides Claude to explore the codebase, decompose objectives into tasks,
// and write a structured plan to a JSON file.
const PlanningPromptTemplate = `You are a senior software architect planning a complex task.

## Objective
%s

## Instructions

1. **Explore** the codebase to understand its structure and patterns
2. **Decompose** the objective into discrete, parallelizable tasks
3. **Write your plan** to ` + "`" + PlanFileName + "`" + ` **at the repository root** (not in any subdirectory) in JSON format

## Plan JSON Schema

Write a JSON file with this structure:
- "summary": Brief executive summary (string)
- "tasks": Array of task objects, each with:
  - "id": Unique identifier like "task-1-setup" (string)
  - "title": Short title (string)
  - "description": Detailed instructions for another Claude instance to execute independently (string)
  - "files": Files this task will modify (array of strings)
  - "depends_on": IDs of tasks that must complete first (array of strings, empty for independent tasks)
  - "priority": Lower = higher priority within dependency level (number)
  - "est_complexity": "low", "medium", or "high" (string)
- "insights": Key findings about the codebase (array of strings)
- "constraints": Risks or constraints to consider (array of strings)

## Guidelines

- **Prefer many small tasks over fewer large ones** - 10 small tasks are better than 3 medium/large tasks
- Each task should be completable within a single session without context exhaustion
- Target "low" complexity tasks; split "medium" or "high" complexity work into multiple smaller tasks
- Prefer granular tasks that can run in parallel over large sequential ones
- Assign clear file ownership to avoid merge conflicts
- Each task description should be complete enough for independent execution
- Use Write tool to create the plan file when ready`

// PlanningStrategy defines a strategic approach for multi-pass planning.
type PlanningStrategy struct {
	Strategy    string // Unique identifier for the strategy
	Description string // Human-readable description
	Prompt      string // Additional guidance to append to the base planning prompt
}

// PlanningStrategies provides different strategic perspectives for multi-pass planning.
// Each strategy offers a distinct approach to task decomposition, enabling the multi-pass
// planning system to generate diverse plans that can be compared and combined.
var PlanningStrategies = []PlanningStrategy{
	{
		Strategy:    "maximize-parallelism",
		Description: "Optimize for maximum parallel execution",
		Prompt: `## Strategic Focus: Maximize Parallelism

When creating your plan, prioritize these principles:

1. **Minimize Dependencies**: Structure tasks to have as few inter-task dependencies as possible. When a dependency seems necessary, consider if the tasks can be restructured to eliminate it.

2. **Prefer Smaller Tasks**: Break work into many small, independent units rather than fewer large ones. A task that can be split into two independent pieces should be split.

3. **Isolate File Ownership**: Assign each file to exactly one task where possible. When multiple tasks must touch the same file, see if the work can be restructured to avoid this.

4. **Flatten the Dependency Graph**: Aim for a wide, shallow execution graph rather than a deep, narrow one. More tasks in the first execution group means more parallelism.

5. **Accept Some Redundancy**: It's acceptable for tasks to have slight overlap in setup or context-building if it means they can run independently.`,
	},
	{
		Strategy:    "minimize-complexity",
		Description: "Optimize for simplicity and clarity",
		Prompt: `## Strategic Focus: Minimize Complexity

When creating your plan, prioritize these principles:

1. **Single Responsibility**: Each task should do exactly one thing well. If a task description contains "and" or multiple objectives, consider splitting it.

2. **Clear Boundaries**: Tasks should have well-defined inputs and outputs. Another developer should be able to understand the task's scope without reading other task descriptions.

3. **Natural Code Boundaries**: Align task boundaries with the codebase's natural structure (packages, modules, components). Don't split work that naturally belongs together.

4. **Explicit Over Implicit**: Make dependencies explicit even if it reduces parallelism. A clear sequential flow is better than a parallel structure with hidden assumptions.

5. **Prefer Clarity Over Parallelism**: When there's a tradeoff between task clarity and parallel execution potential, choose clarity. A well-understood task is easier to execute correctly.`,
	},
	{
		Strategy:    "balanced-approach",
		Description: "Balance parallelism, complexity, and dependencies",
		Prompt: `## Strategic Focus: Balanced Approach

When creating your plan, balance these competing concerns:

1. **Respect Natural Structure**: Follow the codebase's existing architecture. Group changes that affect related functionality, even if this reduces parallelism.

2. **Pragmatic Dependencies**: Include dependencies that reflect genuine execution order requirements, but don't over-constrain the graph. Consider which dependencies are truly necessary vs. merely convenient.

3. **Right-Sized Tasks**: Tasks should be large enough to represent meaningful work units but small enough to complete in a single focused session. Target 15-45 minutes of work per task.

4. **Consider Integration**: Group changes that will need to be tested together. Tasks that affect the same feature or user flow may benefit from shared context.

5. **Maintain Flexibility**: Leave room for parallel execution where natural, but don't force artificial splits. The goal is a plan that's both efficient and maintainable.`,
	},
	{
		Strategy:    "risk-aware",
		Description: "Optimize for safe execution with risk-based task ordering",
		Prompt: `## Strategic Focus: Risk-Aware Decomposition

When creating your plan, prioritize safety and risk management:

1. **Identify High-Risk Changes**: Classify tasks by risk level based on:
   - Configuration/schema changes (high risk)
   - Core module modifications (high risk)
   - Cross-cutting concerns (medium-high risk)
   - Isolated feature additions (low risk)
   - Documentation/comment updates (minimal risk)

2. **Sequential Execution for Risky Tasks**: High-risk tasks should either:
   - Run alone in their own execution group
   - Have explicit dependencies to prevent parallel execution with other risky tasks
   - Complete before dependent tasks begin

3. **Foundation First**: Structure dependencies so foundational changes complete before dependent work:
   - Data model changes before business logic
   - Interface definitions before implementations
   - Configuration before feature code

4. **Minimize Blast Radius**: Each task should affect the smallest possible scope:
   - One package/module per task when possible
   - Separate test updates from implementation
   - Isolate breaking changes into dedicated tasks

5. **File Conflict Prevention**: When multiple tasks must touch the same file:
   - Designate one task as the "owner" of that file
   - Other tasks depend on the owner task
   - Consider creating a dedicated integration task at the end

6. **Critical Path Awareness**: Identify and minimize the critical path:
   - Tasks with many dependents should be small and focused
   - Avoid deep dependency chains where possible
   - Consider splitting critical-path tasks to enable parallelism

7. **Rollback Considerations**: Structure tasks so partial completion is recoverable:
   - Each task should leave the codebase in a valid state
   - Avoid tasks that require "all or nothing" success`,
	},
}

// GetPlanningPrompt returns the planning prompt for a given objective.
func GetPlanningPrompt(objective string) string {
	return fmt.Sprintf(PlanningPromptTemplate, objective)
}

// GetMultiPassPlanningPrompt combines the base PlanningPromptTemplate with strategy-specific
// guidance for multi-pass planning. The strategy parameter should match one of the Strategy
// fields in PlanningStrategies.
func GetMultiPassPlanningPrompt(strategy string, objective string) string {
	// Find the strategy-specific guidance
	var strategyPrompt string
	for _, s := range PlanningStrategies {
		if s.Strategy == strategy {
			strategyPrompt = s.Prompt
			break
		}
	}

	// Build the base prompt with the objective
	basePrompt := fmt.Sprintf(PlanningPromptTemplate, objective)

	// If no matching strategy found, return just the base prompt
	if strategyPrompt == "" {
		return basePrompt
	}

	// Combine base prompt with strategy-specific guidance
	return basePrompt + "\n\n" + strategyPrompt
}

// GetStrategyNames returns the list of available strategy names.
func GetStrategyNames() []string {
	names := make([]string, len(PlanningStrategies))
	for i, s := range PlanningStrategies {
		names[i] = s.Strategy
	}
	return names
}

// GetStrategy returns the PlanningStrategy for a given strategy name, or nil if not found.
func GetStrategy(name string) *PlanningStrategy {
	for i := range PlanningStrategies {
		if PlanningStrategies[i].Strategy == name {
			return &PlanningStrategies[i]
		}
	}
	return nil
}

// SynthesisCompletionFileName is the sentinel file written by the synthesis agent.
const SynthesisCompletionFileName = ".claudio-synthesis-complete.json"

// SynthesisPromptTemplate is the prompt used for the synthesis phase.
// It guides Claude to review all completed work, identify integration issues,
// and write a completion file with any revision needs.
const SynthesisPromptTemplate = `You are reviewing the results of a parallel execution plan.

## Original Objective
%s

## Completed Tasks
%s

## Task Results Summary
%s

## Instructions

1. **Review** all completed work to ensure it meets the original objective
2. **Identify** any integration issues, bugs, or conflicts that need resolution
3. **Verify** that all pieces work together correctly
4. **Check** for any missing functionality or incomplete implementations

## Completion Protocol

When your review is complete, you MUST write a completion file to signal the orchestrator:

1. Use Write tool to create ` + "`" + SynthesisCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "status": "complete",
  "revision_round": %d,
  "issues_found": [
    {
      "task_id": "task-id-here",
      "description": "Clear description of the issue",
      "files": ["file1.go", "file2.go"],
      "severity": "critical|major|minor",
      "suggestion": "How to fix this issue"
    }
  ],
  "tasks_affected": ["task-1", "task-2"],
  "integration_notes": "Observations about how the pieces integrate",
  "recommendations": ["Suggestions for the consolidation phase"]
}
` + "```" + `

3. Set status to "needs_revision" if critical/major issues require fixing, "complete" otherwise
4. Leave issues_found as empty array [] if no issues found
5. Include integration_notes with observations about cross-task integration
6. Add recommendations for the consolidation phase (merge order, potential conflicts, etc.)

This file signals that your review is done and provides context for subsequent phases.`

// RevisionPromptTemplate is the prompt used for the revision phase.
// It instructs Claude to fix the identified issues in a specific task's worktree.
const RevisionPromptTemplate = `You are addressing issues identified during review of completed work.

## Original Objective
%s

## Task Being Revised
- Task ID: %s
- Task Title: %s
- Original Description: %s
- Revision Round: %d

## Issues to Address
%s

## Instructions

1. **Fix** the identified issues in this task's code
2. **Test** your changes to ensure they work correctly
3. **Commit** your fixes with a clear commit message

Focus only on the issues listed above. Do not make unrelated changes.`

// GetSynthesisPrompt returns the synthesis prompt for reviewing completed work.
func GetSynthesisPrompt(objective, taskList, resultsSummary string, revisionRound int) string {
	return fmt.Sprintf(SynthesisPromptTemplate, objective, taskList, resultsSummary, revisionRound)
}

// GetRevisionPrompt returns the revision prompt for fixing identified issues.
func GetRevisionPrompt(objective, taskID, taskTitle, taskDesc string, revisionRound int, issues string) string {
	return fmt.Sprintf(RevisionPromptTemplate, objective, taskID, taskTitle, taskDesc, revisionRound, issues)
}

// TaskCompletionFileName is the sentinel file written by task agents when complete.
const TaskCompletionFileName = ".claudio-task-complete.json"

// GroupConsolidationCompletionFileName is the sentinel file written by group consolidators.
const GroupConsolidationCompletionFileName = ".claudio-group-consolidation-complete.json"

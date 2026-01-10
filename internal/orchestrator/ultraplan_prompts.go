package orchestrator

import "fmt"

// PlanningPromptTemplate is the prompt used for the planning phase
const PlanningPromptTemplate = `You are a senior software architect planning a complex task.

## Objective
%s

## Instructions

1. **Explore** the codebase to understand its structure and patterns
2. **Decompose** the objective into discrete, parallelizable tasks
3. **Write your plan** to the file ` + "`" + PlanFileName + "`" + ` in JSON format

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

// MultiPassPlanningStrategy defines a strategic approach for multi-pass planning
type MultiPassPlanningStrategy struct {
	Strategy    string // Unique identifier for the strategy
	Description string // Human-readable description
	Prompt      string // Additional guidance to append to the base planning prompt
}

// MultiPassPlanningPrompts provides different strategic perspectives for multi-pass planning.
// Each strategy offers a distinct approach to task decomposition, enabling the multi-pass
// planning system to generate diverse plans that can be compared and combined.
var MultiPassPlanningPrompts = []MultiPassPlanningStrategy{
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
}

// GetMultiPassPlanningPrompt combines the base PlanningPromptTemplate with strategy-specific
// guidance for multi-pass planning. The strategy parameter should match one of the Strategy
// fields in MultiPassPlanningPrompts.
func GetMultiPassPlanningPrompt(strategy string, objective string) string {
	// Find the strategy-specific guidance
	var strategyPrompt string
	for _, s := range MultiPassPlanningPrompts {
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

// GetMultiPassStrategyNames returns the list of available strategy names
func GetMultiPassStrategyNames() []string {
	names := make([]string, len(MultiPassPlanningPrompts))
	for i, s := range MultiPassPlanningPrompts {
		names[i] = s.Strategy
	}
	return names
}

// buildPlanningPrompt constructs the planning phase prompt with the given objective
func buildPlanningPrompt(objective string) string {
	return fmt.Sprintf(PlanningPromptTemplate, objective)
}

// buildTaskPrompt constructs a prompt for an individual task execution.
// It includes the task description and any relevant context about the overall plan.
func buildTaskPrompt(task *PlannedTask, plan *PlanSpec) string {
	return fmt.Sprintf(`# Task: %s

## Part of Ultra-Plan: %s

## Your Task

%s

## Expected Files

You are expected to work with these files:
%s

## Guidelines

- Focus only on this specific task
- Do not modify files outside of your assigned scope unless necessary
- Commit your changes before writing the completion file

## Completion Protocol

When your task is complete, you MUST write a completion file to signal the orchestrator:

1. Use Write tool to create `+"`"+TaskCompletionFileName+"`"+` in your worktree root
2. Include this JSON structure:
`+"```json"+`
{
  "task_id": "%s",
  "status": "complete",
  "summary": "Brief description of what you accomplished",
  "files_modified": ["list", "of", "files", "you", "changed"],
  "notes": "Any implementation notes for the consolidation phase",
  "issues": ["Any concerns or blocking issues found"],
  "suggestions": ["Suggestions for integration with other tasks"],
  "dependencies": ["Any new runtime dependencies added"]
}
`+"```"+`

3. Use status "blocked" if you cannot complete (explain in issues), or "failed" if something broke
4. This file signals that your work is done and provides context for consolidation`,
		task.Title,
		plan.Summary,
		task.Description,
		formatFilesList(task.Files),
		task.ID,
	)
}

// formatFilesList formats a slice of file paths for display in prompts
func formatFilesList(files []string) string {
	if len(files) == 0 {
		return "- (No specific files assigned)"
	}
	result := ""
	for _, f := range files {
		result += "- " + f + "\n"
	}
	return result
}

// SynthesisPromptTemplate is the prompt used for the synthesis phase
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

// buildSynthesisPrompt constructs the synthesis phase prompt with task completion context
func buildSynthesisPrompt(objective string, completedTasks string, taskResults string, revisionRound int) string {
	return fmt.Sprintf(SynthesisPromptTemplate, objective, completedTasks, taskResults, revisionRound)
}

// RevisionPromptTemplate is the prompt used for the revision phase
// It instructs Claude to fix the identified issues in a specific task's worktree
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

## Worktree Information
You are working in the same worktree that was used for the original task.
All previous changes from this task are already present.

## Instructions

1. **Review** the issues identified above
2. **Fix** each issue in the codebase
3. **Test** that your fixes don't break existing functionality
4. **Commit** your changes with a clear message describing the fixes

Focus only on addressing the identified issues. Do not refactor or make other changes unless directly related to fixing the issues.

## Completion Protocol

When your revision is complete, you MUST write a completion file:

1. Use Write tool to create ` + "`" + RevisionCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "task_id": "%s",
  "revision_round": %d,
  "issues_addressed": ["Description of issue 1 that was fixed", "Description of issue 2"],
  "summary": "Brief summary of the changes made",
  "files_modified": ["file1.go", "file2.go"],
  "remaining_issues": ["Any issues that could not be fixed"]
}
` + "```" + `

3. List all issues you successfully addressed in issues_addressed
4. Leave remaining_issues empty if all issues were fixed
5. This file signals that your revision is done`

// buildRevisionPrompt constructs a revision prompt for fixing issues in a task
func buildRevisionPrompt(objective string, task *PlannedTask, revisionRound int, issues string) string {
	return fmt.Sprintf(RevisionPromptTemplate,
		objective,
		task.ID,
		task.Title,
		task.Description,
		revisionRound,
		issues,
		task.ID,
		revisionRound,
	)
}

// ConsolidationPromptTemplate is the prompt used for the consolidation phase
// This prompts Claude to consolidate task branches into group branches and create PRs
const ConsolidationPromptTemplate = `You are consolidating completed ultraplan task branches into pull requests.

## Objective
%s

## Branch Configuration
- Branch prefix: %s
- Main branch: %s
- Consolidation mode: %s
- Create drafts: %v

## Execution Groups and Task Branches
%s

## Task Worktree Details
The following are the exact worktree paths for each completed task. Use these paths to review the work if needed:
%s

## Synthesis Review Context
The following is context from the synthesis review phase:
%s

## Instructions

Your job is to consolidate all the task branches into group branches and create pull requests.

### For Stacked PRs Mode
1. For each execution group (starting from group 1):
   - Create a consolidated branch from the appropriate base:
     - Group 1: branch from main
     - Group N: branch from group N-1's branch
   - Cherry-pick all task commits from that group's task branches
   - Push the consolidated branch
   - Create a PR with appropriate title and description

2. PRs should be stacked: each PR's base is the previous group's branch.

### For Single PR Mode
1. Create one consolidated branch from main
2. Cherry-pick all task commits in execution order
3. Push the branch and create a single PR

### Commands to Use
- Use ` + "`" + `git cherry-pick` + "`" + ` to bring commits from task branches
- Use ` + "`" + `git push -u origin <branch>` + "`" + ` to push branches
- Use ` + "`" + `gh pr create` + "`" + ` to create pull requests
- If cherry-pick has conflicts, resolve them or report them clearly
- You can use the worktree paths above to review file changes if needed

### PR Format
Title: "ultraplan: group N - <objective summary>" (for stacked) or "ultraplan: <objective summary>" (for single)
Body should include:
- The objective
- Which tasks are included
- Integration notes from synthesis review (if any)
- For stacked PRs: note the merge order dependency

## Completion Protocol

When consolidation is complete, you MUST write a completion file:

1. Use Write tool to create ` + "`" + ConsolidationCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "status": "complete",
  "mode": "%s",
  "group_results": [
    {
      "group_index": 0,
      "branch_name": "branch-name",
      "tasks_included": ["task-1", "task-2"],
      "commit_count": 5,
      "success": true
    }
  ],
  "prs_created": [
    {
      "url": "https://github.com/owner/repo/pull/123",
      "title": "PR title",
      "group_index": 0
    }
  ],
  "total_commits": 10,
  "files_changed": ["file1.go", "file2.go"]
}
` + "```" + `

3. Set status to "complete" if all PRs were created successfully
4. Set status to "partial" if some PRs failed to create
5. Set status to "failed" if consolidation could not complete
6. List all PR URLs in prs_created array

This file signals that consolidation is done and provides a record of the PRs created.`

// buildConsolidationPrompt constructs the consolidation phase prompt
func buildConsolidationPrompt(
	objective string,
	branchPrefix string,
	mainBranch string,
	consolidationMode string,
	createDrafts bool,
	executionGroups string,
	worktreeDetails string,
	synthesisContext string,
) string {
	return fmt.Sprintf(ConsolidationPromptTemplate,
		objective,
		branchPrefix,
		mainBranch,
		consolidationMode,
		createDrafts,
		executionGroups,
		worktreeDetails,
		synthesisContext,
		consolidationMode, // For the JSON example in completion protocol
	)
}

// PlanManagerPromptTemplate is the prompt for the coordinator-manager in multi-pass mode
// It receives all candidate plans and must select the best one or merge them
const PlanManagerPromptTemplate = `You are a senior technical lead evaluating multiple implementation plans.

## Objective
%s

## Candidate Plans
Three different planning strategies have produced the following plans:

%s

## Your Task

Evaluate each plan based on:
1. **Parallelism potential**: How many tasks can run concurrently?
2. **Task granularity**: Are tasks appropriately sized (prefer smaller, focused tasks)?
3. **Dependency structure**: Is the dependency graph sensible and minimal?
4. **File ownership**: Do tasks have clear, non-overlapping file assignments?
5. **Completeness**: Does the plan fully address the objective?
6. **Risk mitigation**: Are constraints and risks properly identified?

## Decision

You must either:
1. **Select** the best plan as-is, OR
2. **Merge** the best elements from multiple plans into a superior plan

## Output

Write your final plan to ` + "`" + PlanFileName + "`" + ` using the standard plan JSON schema.

Before the plan file, output your reasoning in this format:
<plan_decision>
{
  "action": "select" or "merge",
  "selected_index": 0-2 (if select) or -1 (if merge),
  "reasoning": "Brief explanation of your decision",
  "plan_scores": [
    {"strategy": "maximize-parallelism", "score": 1-10, "strengths": "...", "weaknesses": "..."},
    {"strategy": "minimize-complexity", "score": 1-10, "strengths": "...", "weaknesses": "..."},
    {"strategy": "balanced-approach", "score": 1-10, "strengths": "...", "weaknesses": "..."}
  ]
}
</plan_decision>

Then write the final plan file.`

// buildPlanManagerPrompt constructs the plan manager prompt for multi-pass plan selection
func buildPlanManagerPrompt(objective string, candidatePlans string) string {
	return fmt.Sprintf(PlanManagerPromptTemplate, objective, candidatePlans)
}

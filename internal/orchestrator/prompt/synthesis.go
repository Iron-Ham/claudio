// Package prompt provides interfaces and types for building prompts
// in the ultra-plan orchestration system.
package prompt

import (
	"fmt"
	"strings"
)

// SynthesisCompletionFileName is the filename for synthesis phase completion.
const SynthesisCompletionFileName = ".claudio-synthesis-complete.json"

// SynthesisBuilder builds prompts for the synthesis phase.
// It aggregates completed task results for review and identifies integration issues.
type SynthesisBuilder struct{}

// NewSynthesisBuilder creates a new SynthesisBuilder.
func NewSynthesisBuilder() *SynthesisBuilder {
	return &SynthesisBuilder{}
}

// Build generates the synthesis prompt from the context.
func (b *SynthesisBuilder) Build(ctx *Context) (string, error) {
	if err := b.validate(ctx); err != nil {
		return "", err
	}

	taskList := b.buildTaskList(ctx)
	resultsSummary := b.buildResultsSummary(ctx)
	revisionRound := 0

	return fmt.Sprintf(synthesisPromptTemplate, ctx.Objective, taskList, resultsSummary, revisionRound), nil
}

// validate checks that the context has all required fields for synthesis prompts.
func (b *SynthesisBuilder) validate(ctx *Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ctx.Phase != PhaseSynthesis {
		return fmt.Errorf("%w: expected %s, got %s", ErrInvalidPhase, PhaseSynthesis, ctx.Phase)
	}
	if ctx.Plan == nil {
		return ErrMissingPlan
	}
	if ctx.Objective == "" {
		return ErrEmptyObjective
	}
	return nil
}

// buildTaskList creates a summary list of completed tasks.
func (b *SynthesisBuilder) buildTaskList(ctx *Context) string {
	var sb strings.Builder

	for _, taskID := range ctx.CompletedTasks {
		task := b.findTask(ctx.Plan.Tasks, taskID)
		if task == nil {
			// Task marked as completed but not found in plan - include warning
			sb.WriteString(fmt.Sprintf("- [%s] [NOT FOUND] - WARNING: task data missing\n", taskID))
			continue
		}

		if task.CommitCount > 0 {
			sb.WriteString(fmt.Sprintf("- [%s] %s (%d commits)\n", task.ID, task.Title, task.CommitCount))
		} else {
			sb.WriteString(fmt.Sprintf("- [%s] %s (NO COMMITS - verify this task)\n", task.ID, task.Title))
		}
	}

	return sb.String()
}

// buildResultsSummary creates a detailed summary of task results.
func (b *SynthesisBuilder) buildResultsSummary(ctx *Context) string {
	var sb strings.Builder

	for _, taskID := range ctx.CompletedTasks {
		task := b.findTask(ctx.Plan.Tasks, taskID)
		if task == nil {
			// Task marked as completed but not found in plan - include warning
			sb.WriteString(fmt.Sprintf("### [NOT FOUND: %s]\n", taskID))
			sb.WriteString("Status: **WARNING - task data missing**\n\n")
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s\n", task.Title))
		sb.WriteString("Status: completed\n")

		if task.CommitCount > 0 {
			sb.WriteString(fmt.Sprintf("Commits: %d\n", task.CommitCount))
		}

		if len(task.Files) > 0 {
			sb.WriteString(fmt.Sprintf("Files: %s\n", strings.Join(task.Files, ", ")))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// findTask finds a task by ID in the task list.
func (b *SynthesisBuilder) findTask(tasks []TaskInfo, id string) *TaskInfo {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

// synthesisPromptTemplate is the prompt used for the synthesis phase.
const synthesisPromptTemplate = `You are reviewing the results of a parallel execution plan.

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

## Completion Protocol - FINAL MANDATORY STEP

**IMPORTANT**: Writing the completion file is your FINAL MANDATORY ACTION.
The orchestrator is BLOCKED waiting for this file.
Without it, your review will NOT be recorded and the workflow cannot proceed.

**DO NOT** wait for user prompting or confirmation.
Write this file AUTOMATICALLY as soon as you have finished your review.

**CRITICAL**: Write this file at the ROOT of your worktree directory, not in any subdirectory.
If you changed directories during the task, use an absolute path or navigate back to the root first.

You MUST write a completion file to signal the orchestrator:

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

This file signals that your review is done and provides context for subsequent phases.

**REMEMBER**: Your review is NOT complete until you write this file. Do it NOW after finishing your review.`

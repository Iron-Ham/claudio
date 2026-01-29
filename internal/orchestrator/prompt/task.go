// Package prompt provides interfaces and types for building prompts
// in the ultra-plan orchestration system.
package prompt

import (
	"fmt"
	"strings"
)

// TaskCompletionFileName is the filename for task completion signaling.
const TaskCompletionFileName = ".claudio-task-complete.json"

// TaskBuilder builds prompts for individual task execution.
// It formats task details, expected files, previous group context,
// and completion protocol instructions.
type TaskBuilder struct{}

// NewTaskBuilder creates a new TaskBuilder.
func NewTaskBuilder() *TaskBuilder {
	return &TaskBuilder{}
}

// Build generates the task prompt from the context.
func (b *TaskBuilder) Build(ctx *Context) (string, error) {
	if err := b.validate(ctx); err != nil {
		return "", err
	}

	var sb strings.Builder

	// Task header
	fmt.Fprintf(&sb, "# Task: %s\n\n", ctx.Task.Title)
	fmt.Fprintf(&sb, "## Part of Ultra-Plan: %s\n\n", ctx.Plan.Summary)

	// Task description
	sb.WriteString("## Your Task\n\n")
	sb.WriteString(ctx.Task.Description)
	sb.WriteString("\n\n")

	// Expected files section
	if len(ctx.Task.Files) > 0 {
		sb.WriteString("## Expected Files\n\n")
		sb.WriteString("You are expected to work with these files:\n")
		for _, f := range ctx.Task.Files {
			fmt.Fprintf(&sb, "- %s\n", f)
		}
		sb.WriteString("\n")
	}

	// Previous group context (for tasks not in group 0)
	if ctx.PreviousGroup != nil && ctx.GroupIndex > 0 {
		b.writePreviousGroupContext(&sb, ctx.PreviousGroup)
	}

	// Guidelines
	sb.WriteString("## Guidelines\n\n")
	sb.WriteString("- Focus only on this specific task\n")
	sb.WriteString("- Do not modify files outside of your assigned scope unless necessary\n")
	sb.WriteString("- Commit your changes before writing the completion file\n\n")

	// Completion protocol
	b.writeCompletionProtocol(&sb, ctx.Task.ID)

	return sb.String(), nil
}

// validate checks that the context has all required fields for task prompts.
func (b *TaskBuilder) validate(ctx *Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ctx.Phase != PhaseTask {
		return fmt.Errorf("%w: expected %s, got %s", ErrInvalidPhase, PhaseTask, ctx.Phase)
	}
	if ctx.Plan == nil {
		return ErrMissingPlan
	}
	if ctx.Task == nil {
		return ErrMissingTask
	}
	if ctx.Task.ID == "" {
		return fmt.Errorf("%w: task ID is required", ErrMissingTask)
	}
	if ctx.Task.Title == "" {
		return fmt.Errorf("%w: task title is required", ErrMissingTask)
	}
	return nil
}

// writePreviousGroupContext writes the context from the previous group's consolidation.
func (b *TaskBuilder) writePreviousGroupContext(sb *strings.Builder, prev *GroupContext) {
	sb.WriteString("## Context from Previous Group\n\n")
	fmt.Fprintf(sb, "This task builds on work consolidated from Group %d.\n\n", prev.GroupIndex+1)

	if prev.Notes != "" {
		fmt.Fprintf(sb, "**Consolidator Notes**: %s\n\n", prev.Notes)
	}

	if len(prev.IssuesForNextGroup) > 0 {
		sb.WriteString("**Important**: The previous group's consolidator flagged these issues:\n")
		for _, issue := range prev.IssuesForNextGroup {
			fmt.Fprintf(sb, "- %s\n", issue)
		}
		sb.WriteString("\n")
	}

	if prev.VerificationPassed {
		sb.WriteString("The consolidated code from the previous group has been verified (build/lint/tests passed).\n\n")
	} else {
		sb.WriteString("**Warning**: The previous group's code verification may have issues. Check carefully.\n\n")
	}
}

// writeCompletionProtocol writes the completion protocol instructions.
func (b *TaskBuilder) writeCompletionProtocol(sb *strings.Builder, taskID string) {
	sb.WriteString("## Completion Protocol - FINAL MANDATORY STEP\n\n")
	sb.WriteString("**IMPORTANT**: Writing the completion file is your FINAL MANDATORY ACTION. ")
	sb.WriteString("The orchestrator is BLOCKED waiting for this file. ")
	sb.WriteString("Without it, your work will NOT be recorded and the workflow cannot proceed.\n\n")
	sb.WriteString("**DO NOT** wait for user prompting or confirmation. ")
	sb.WriteString("Write this file AUTOMATICALLY as soon as you have finished your implementation work and committed your changes.\n\n")
	sb.WriteString("**CRITICAL**: Write this file at the ROOT of your worktree directory, not in any subdirectory.\n")
	sb.WriteString("If you changed directories during the task (e.g., `cd project/`), use an absolute path or navigate back to the root first.\n\n")
	fmt.Fprintf(sb, "1. Use Write tool to create `%s` in your worktree root\n", TaskCompletionFileName)
	sb.WriteString("2. Include this JSON structure:\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	fmt.Fprintf(sb, "  \"task_id\": \"%s\",\n", taskID)
	sb.WriteString("  \"status\": \"complete\",\n")
	sb.WriteString("  \"summary\": \"Brief description of what you accomplished\",\n")
	sb.WriteString("  \"files_modified\": [\"list\", \"of\", \"files\", \"you\", \"changed\"],\n")
	sb.WriteString("  \"notes\": \"Any implementation notes for the consolidation phase\",\n")
	sb.WriteString("  \"issues\": [\"Any concerns or blocking issues found\"],\n")
	sb.WriteString("  \"suggestions\": [\"Suggestions for integration with other tasks\"],\n")
	sb.WriteString("  \"dependencies\": [\"Any new runtime dependencies added\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("3. Use status \"blocked\" if you cannot complete (explain in issues), or \"failed\" if something broke\n")
	sb.WriteString("4. This file signals that your work is done and provides context for consolidation\n\n")
	sb.WriteString("**REMEMBER**: Your task is NOT complete until you write this file. Do it NOW after finishing your work.\n")
}

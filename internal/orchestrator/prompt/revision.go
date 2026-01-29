// Package prompt provides interfaces and types for building prompts
// in the ultra-plan orchestration system.
package prompt

import (
	"fmt"
	"strings"
)

// RevisionCompletionFileName is the filename for revision phase completion.
const RevisionCompletionFileName = ".claudio-revision-complete.json"

// RevisionBuilder builds prompts for the revision phase.
// It formats issues to address and provides instructions for fixing them.
type RevisionBuilder struct{}

// NewRevisionBuilder creates a new RevisionBuilder.
func NewRevisionBuilder() *RevisionBuilder {
	return &RevisionBuilder{}
}

// Build generates the revision prompt from the context.
func (b *RevisionBuilder) Build(ctx *Context) (string, error) {
	if err := b.validate(ctx); err != nil {
		return "", err
	}

	issuesStr := b.formatIssues(ctx)
	revisionRound := ctx.Revision.Round + 1

	return fmt.Sprintf(revisionPromptTemplate,
		ctx.Objective,
		ctx.Task.ID,
		ctx.Task.Title,
		ctx.Task.Description,
		revisionRound,
		issuesStr,
		ctx.Task.ID,
		revisionRound,
	), nil
}

// validate checks that the context has all required fields for revision prompts.
func (b *RevisionBuilder) validate(ctx *Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ctx.Phase != PhaseRevision {
		return fmt.Errorf("%w: expected %s, got %s", ErrInvalidPhase, PhaseRevision, ctx.Phase)
	}
	if ctx.Plan == nil {
		return ErrMissingPlan
	}
	if ctx.Task == nil {
		return ErrMissingTask
	}
	if ctx.Revision == nil {
		return ErrMissingRevision
	}
	if ctx.Objective == "" {
		return ErrEmptyObjective
	}
	return nil
}

// formatIssues formats the revision issues for display.
func (b *RevisionBuilder) formatIssues(ctx *Context) string {
	var sb strings.Builder

	// Filter issues for this specific task
	var taskIssues []RevisionIssue
	for _, issue := range ctx.Revision.Issues {
		if issue.TaskID == ctx.Task.ID || issue.TaskID == "" {
			taskIssues = append(taskIssues, issue)
		}
	}

	for i, issue := range taskIssues {
		sb.WriteString(fmt.Sprintf("%d. **%s**: %s\n", i+1, issue.Severity, issue.Description))
		if len(issue.Files) > 0 {
			sb.WriteString(fmt.Sprintf("   Files: %s\n", strings.Join(issue.Files, ", ")))
		}
		if issue.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("   Suggestion: %s\n", issue.Suggestion))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// revisionPromptTemplate is the prompt used for the revision phase.
const revisionPromptTemplate = `You are addressing issues identified during review of completed work.

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

## Completion Protocol - FINAL MANDATORY STEP

**IMPORTANT**: Writing the completion file is your FINAL MANDATORY ACTION.
The orchestrator is BLOCKED waiting for this file.
Without it, your revision work will NOT be recorded and the workflow cannot proceed.

**DO NOT** wait for user prompting or confirmation.
Write this file AUTOMATICALLY as soon as you have finished your revision work and committed your changes.

**CRITICAL**: Write this file at the ROOT of your worktree directory, not in any subdirectory.
If you changed directories during the task, use an absolute path or navigate back to the root first.

You MUST write a completion file:

1. Use Write tool to create ` + "`" + RevisionCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "task_id": "%s",
  "revision_round": %d,
  "status": "complete",
  "issues_addressed": [
    {
      "original_issue": "Issue description",
      "fix_applied": "What you did to fix it",
      "files_changed": ["file1.go"]
    }
  ],
  "remaining_issues": [],
  "notes": "Any additional context about the fixes"
}
` + "```" + `

3. Use status "partial" if some issues couldn't be fully resolved
4. Use status "blocked" if you need input or can't proceed

This file signals that your revision work is complete.

**REMEMBER**: Your revision is NOT complete until you write this file. Do it NOW after finishing your work.`

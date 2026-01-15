// Package prompt provides interfaces and types for building prompts
// in the ultra-plan orchestration system.
package prompt

import (
	"fmt"
	"strings"
)

// ConsolidationCompletionFileName is the filename for consolidation phase completion.
const ConsolidationCompletionFileName = ".claudio-consolidation-complete.json"

// GroupConsolidationCompletionFileName is the filename for per-group consolidation completion.
const GroupConsolidationCompletionFileName = ".claudio-group-consolidation-complete.json"

// ConsolidationBuilder builds prompts for the main consolidation phase.
// It formats branch information and instructions for merging all task branches.
type ConsolidationBuilder struct{}

// NewConsolidationBuilder creates a new ConsolidationBuilder.
func NewConsolidationBuilder() *ConsolidationBuilder {
	return &ConsolidationBuilder{}
}

// Build generates the consolidation prompt from the context.
func (b *ConsolidationBuilder) Build(ctx *Context) (string, error) {
	if err := b.validate(ctx); err != nil {
		return "", err
	}

	groupsInfo := b.buildGroupsInfo(ctx)
	worktreeInfo := b.buildWorktreeInfo(ctx)
	synthesisContext := b.buildSynthesisContext(ctx)

	return fmt.Sprintf(consolidationPromptTemplate,
		ctx.Objective,
		ctx.Consolidation.BranchPrefix,
		ctx.Consolidation.MainBranch,
		ctx.Consolidation.Mode,
		"true", // createDrafts placeholder
		groupsInfo,
		worktreeInfo,
		synthesisContext,
		ctx.Consolidation.Mode,
	), nil
}

// validate checks that the context has all required fields.
func (b *ConsolidationBuilder) validate(ctx *Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ctx.Phase != PhaseConsolidation {
		return fmt.Errorf("%w: expected %s, got %s", ErrInvalidPhase, PhaseConsolidation, ctx.Phase)
	}
	if ctx.Plan == nil {
		return ErrMissingPlan
	}
	if ctx.Consolidation == nil {
		return ErrMissingConsolidation
	}
	return nil
}

// buildGroupsInfo builds information about execution groups.
func (b *ConsolidationBuilder) buildGroupsInfo(ctx *Context) string {
	var sb strings.Builder

	for groupIdx, taskIDs := range ctx.Plan.ExecutionOrder {
		sb.WriteString(fmt.Sprintf("\n### Group %d\n", groupIdx+1))

		// Check for pre-consolidated branch
		if ctx.Consolidation != nil && groupIdx < len(ctx.Consolidation.GroupBranches) {
			consolidatedBranch := ctx.Consolidation.GroupBranches[groupIdx]
			if consolidatedBranch != "" {
				sb.WriteString(fmt.Sprintf("**CONSOLIDATED BRANCH (ALREADY MERGED)**: %s\n", consolidatedBranch))
				sb.WriteString("The tasks in this group have already been consolidated into this branch.\n")
			}
		}

		sb.WriteString("Tasks in this group:\n")
		for _, taskID := range taskIDs {
			task := b.findTask(ctx.Plan.Tasks, taskID)
			if task == nil {
				// Task referenced in execution order but not found in task list
				// Include warning in output so it's visible during consolidation
				sb.WriteString(fmt.Sprintf("- Task: [NOT FOUND] (%s) - WARNING: task data missing\n", taskID))
				continue
			}
			sb.WriteString(fmt.Sprintf("- Task: %s (%s)\n", task.Title, taskID))
		}
	}

	return sb.String()
}

// buildWorktreeInfo builds information about task worktrees.
func (b *ConsolidationBuilder) buildWorktreeInfo(ctx *Context) string {
	var sb strings.Builder

	if ctx.Consolidation != nil && len(ctx.Consolidation.TaskWorktrees) > 0 {
		for _, twi := range ctx.Consolidation.TaskWorktrees {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)\n", twi.TaskTitle, twi.TaskID))
			sb.WriteString(fmt.Sprintf("  - Worktree: %s\n", twi.WorktreePath))
			sb.WriteString(fmt.Sprintf("  - Branch: %s\n", twi.Branch))
		}
	}

	// Add note about pre-consolidated branches
	if ctx.Consolidation != nil && ctx.Consolidation.PreConsolidatedBranch != "" {
		sb.WriteString("\n## Pre-Consolidated Branches\n")
		sb.WriteString("**IMPORTANT**: Groups have already been incrementally consolidated.\n")
		sb.WriteString(fmt.Sprintf("Pre-consolidated branch: %s\n", ctx.Consolidation.PreConsolidatedBranch))
	}

	return sb.String()
}

// buildSynthesisContext builds context from synthesis phase.
func (b *ConsolidationBuilder) buildSynthesisContext(ctx *Context) string {
	var sb strings.Builder

	if ctx.Synthesis != nil {
		if len(ctx.Synthesis.Notes) > 0 {
			sb.WriteString("Notes:\n")
			for _, note := range ctx.Synthesis.Notes {
				sb.WriteString(fmt.Sprintf("- %s\n", note))
			}
		}
		if len(ctx.Synthesis.Recommendations) > 0 {
			sb.WriteString("Recommendations:\n")
			for _, rec := range ctx.Synthesis.Recommendations {
				sb.WriteString(fmt.Sprintf("- %s\n", rec))
			}
		}
		if len(ctx.Synthesis.Issues) > 0 {
			sb.WriteString(fmt.Sprintf("Issues Found: %d\n", len(ctx.Synthesis.Issues)))
			for _, issue := range ctx.Synthesis.Issues {
				sb.WriteString(fmt.Sprintf("- %s\n", issue))
			}
		}
	} else {
		sb.WriteString("No synthesis context available\n")
	}

	return sb.String()
}

// findTask finds a task by ID.
func (b *ConsolidationBuilder) findTask(tasks []TaskInfo, id string) *TaskInfo {
	for i := range tasks {
		if tasks[i].ID == id {
			return &tasks[i]
		}
	}
	return nil
}

// consolidationPromptTemplate is the prompt for the consolidation phase.
const consolidationPromptTemplate = `You are consolidating work from multiple parallel task branches.

## Original Objective
%s

## Branch Configuration
- **Branch Prefix**: %s
- **Main Branch**: %s
- **Consolidation Mode**: %s
- **Create Draft PRs**: %s

## Execution Groups
%s

## Task Worktrees
%s

## Synthesis Context
%s

## Instructions

1. **Review** all completed task branches
2. **Create** consolidated branches according to the consolidation mode
3. **Resolve** any merge conflicts
4. **Run** verification (build, lint, tests)
5. **Create** pull requests

## Completion Protocol

**CRITICAL**: Write this file at the ROOT of your worktree directory, not in any subdirectory.
If you changed directories during the task, use an absolute path or navigate back to the root first.

When consolidation is complete, write ` + "`" + ConsolidationCompletionFileName + "`" + `:

` + "```json" + `
{
  "status": "complete",
  "mode": "%s",
  "branches_created": ["branch-1", "branch-2"],
  "prs_created": [
    {"number": 123, "url": "https://github.com/..."}
  ],
  "conflicts_resolved": [
    {"file": "path/to/file.go", "resolution": "Description"}
  ],
  "verification": {
    "build": true,
    "lint": true,
    "tests": true
  },
  "notes": "Any observations"
}
` + "```"

// GroupConsolidatorBuilder builds prompts for per-group consolidation.
// Each execution group gets its own consolidator to merge task branches.
type GroupConsolidatorBuilder struct{}

// NewGroupConsolidatorBuilder creates a new GroupConsolidatorBuilder.
func NewGroupConsolidatorBuilder() *GroupConsolidatorBuilder {
	return &GroupConsolidatorBuilder{}
}

// Build generates the group consolidator prompt.
func (b *GroupConsolidatorBuilder) Build(ctx *Context) (string, error) {
	if err := b.validate(ctx); err != nil {
		return "", err
	}

	taskBranches := b.getTaskBranchesForGroup(ctx)
	baseBranch := b.determineBaseBranch(ctx)
	consolidatedBranch := b.generateConsolidatedBranchName(ctx)

	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("# Group %d Consolidation\n\n", ctx.GroupIndex+1))
	sb.WriteString(fmt.Sprintf("## Part of Ultra-Plan: %s\n\n", ctx.Plan.Summary))

	// Objective
	sb.WriteString("## Objective\n\n")
	sb.WriteString("Consolidate all completed task branches from this group into a single stable branch.\n")
	sb.WriteString("You must resolve any merge conflicts, verify the consolidated code works, and pass context to the next group.\n\n")

	// Tasks completed in this group
	sb.WriteString("## Tasks Completed in This Group\n\n")
	for _, branch := range taskBranches {
		sb.WriteString(fmt.Sprintf("### %s: %s\n", branch.TaskID, branch.TaskTitle))
		sb.WriteString(fmt.Sprintf("- Branch: `%s`\n", branch.Branch))
		sb.WriteString(fmt.Sprintf("- Worktree: `%s`\n", branch.WorktreePath))
		sb.WriteString("\n")
	}

	// Context from previous group
	if ctx.GroupIndex > 0 && ctx.PreviousGroup != nil {
		sb.WriteString("## Context from Previous Group's Consolidator\n\n")
		if ctx.PreviousGroup.Notes != "" {
			sb.WriteString(fmt.Sprintf("**Notes**: %s\n\n", ctx.PreviousGroup.Notes))
		}
		if len(ctx.PreviousGroup.IssuesForNextGroup) > 0 {
			sb.WriteString("**Issues/Warnings to Address**:\n")
			for _, issue := range ctx.PreviousGroup.IssuesForNextGroup {
				sb.WriteString(fmt.Sprintf("- %s\n", issue))
			}
			sb.WriteString("\n")
		}
	}

	// Branch configuration
	sb.WriteString("## Branch Configuration\n\n")
	sb.WriteString(fmt.Sprintf("- **Base branch**: `%s`\n", baseBranch))
	sb.WriteString(fmt.Sprintf("- **Target consolidated branch**: `%s`\n", consolidatedBranch))
	sb.WriteString(fmt.Sprintf("- **Task branches to consolidate**: %d\n\n", len(taskBranches)))

	// Instructions
	b.writeInstructions(&sb, consolidatedBranch, baseBranch)

	// Completion protocol
	b.writeCompletionProtocol(&sb, ctx.GroupIndex, consolidatedBranch)

	return sb.String(), nil
}

// validate checks context for group consolidator.
func (b *GroupConsolidatorBuilder) validate(ctx *Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ctx.Phase != PhaseConsolidation {
		return fmt.Errorf("%w: expected %s, got %s", ErrInvalidPhase, PhaseConsolidation, ctx.Phase)
	}
	if ctx.Plan == nil {
		return ErrMissingPlan
	}
	if ctx.Consolidation == nil {
		return ErrMissingConsolidation
	}
	if ctx.GroupIndex < 0 {
		return fmt.Errorf("group index must be non-negative")
	}
	return nil
}

// getTaskBranchesForGroup returns task worktree info for the current group.
func (b *GroupConsolidatorBuilder) getTaskBranchesForGroup(ctx *Context) []TaskWorktreeInfo {
	if ctx.Consolidation == nil {
		return nil
	}

	// Get task IDs for this group
	var taskIDs []string
	if ctx.GroupIndex < len(ctx.Plan.ExecutionOrder) {
		taskIDs = ctx.Plan.ExecutionOrder[ctx.GroupIndex]
	}

	// Filter worktrees for tasks in this group
	taskIDSet := make(map[string]bool)
	for _, id := range taskIDs {
		taskIDSet[id] = true
	}

	var result []TaskWorktreeInfo
	for _, tw := range ctx.Consolidation.TaskWorktrees {
		if taskIDSet[tw.TaskID] {
			result = append(result, tw)
		}
	}

	return result
}

// determineBaseBranch determines the base branch for this group.
func (b *GroupConsolidatorBuilder) determineBaseBranch(ctx *Context) string {
	if ctx.GroupIndex == 0 {
		if ctx.Consolidation != nil && ctx.Consolidation.MainBranch != "" {
			return ctx.Consolidation.MainBranch
		}
		return "main"
	}

	// For subsequent groups, use the previous group's consolidated branch
	if ctx.Consolidation != nil && ctx.GroupIndex-1 < len(ctx.Consolidation.GroupBranches) {
		if branch := ctx.Consolidation.GroupBranches[ctx.GroupIndex-1]; branch != "" {
			return branch
		}
	}

	if ctx.Consolidation != nil && ctx.Consolidation.MainBranch != "" {
		return ctx.Consolidation.MainBranch
	}
	return "main"
}

// generateConsolidatedBranchName creates the branch name for consolidated output.
func (b *GroupConsolidatorBuilder) generateConsolidatedBranchName(ctx *Context) string {
	prefix := "Iron-Ham"
	if ctx.Consolidation != nil && ctx.Consolidation.BranchPrefix != "" {
		prefix = ctx.Consolidation.BranchPrefix
	}

	planID := ctx.SessionID
	if planID == "" {
		planID = "unknown"
	} else if len(planID) > 8 {
		planID = planID[:8]
	}

	return fmt.Sprintf("%s/ultraplan-%s-group-%d", prefix, planID, ctx.GroupIndex+1)
}

// writeInstructions writes the consolidation instructions.
func (b *GroupConsolidatorBuilder) writeInstructions(sb *strings.Builder, consolidatedBranch, baseBranch string) {
	sb.WriteString("## Your Tasks\n\n")
	sb.WriteString("1. **Create the consolidated branch** from the base branch:\n")
	fmt.Fprintf(sb, "   ```bash\n   git checkout -b %s %s\n   ```\n\n", consolidatedBranch, baseBranch)

	sb.WriteString("2. **Cherry-pick commits** from each task branch in order. For each branch:\n")
	sb.WriteString("   - Review the commits on the branch\n")
	sb.WriteString("   - Cherry-pick them onto the consolidated branch\n")
	sb.WriteString("   - Resolve any conflicts intelligently using your understanding of the code\n\n")

	sb.WriteString("3. **Run verification** to ensure the consolidated code is stable:\n")
	sb.WriteString("   - Detect the project type (Go, Node, Python, iOS, etc.)\n")
	sb.WriteString("   - Run appropriate build/compile commands\n")
	sb.WriteString("   - Run linting if available\n")
	sb.WriteString("   - Run tests if available\n")
	sb.WriteString("   - Fix any issues that arise\n\n")

	sb.WriteString("4. **Push the consolidated branch** to the remote\n\n")

	sb.WriteString("5. **Write the completion file** to signal success\n\n")

	// Conflict resolution guidelines
	sb.WriteString("## Conflict Resolution Guidelines\n\n")
	sb.WriteString("- Prefer changes that preserve functionality from all tasks\n")
	sb.WriteString("- If there are conflicting approaches, choose the more robust one\n")
	sb.WriteString("- Document your resolution reasoning in the completion file\n")
	sb.WriteString("- If you cannot resolve a conflict, document it as an issue\n\n")
}

// writeCompletionProtocol writes the completion protocol for group consolidation.
func (b *GroupConsolidatorBuilder) writeCompletionProtocol(sb *strings.Builder, groupIndex int, consolidatedBranch string) {
	sb.WriteString("## Completion Protocol\n\n")
	sb.WriteString("**CRITICAL**: Write this file at the ROOT of your worktree directory, not in any subdirectory.\n")
	sb.WriteString("If you changed directories during the task, use an absolute path or navigate back to the root first.\n\n")
	fmt.Fprintf(sb, "When consolidation is complete, write `%s` in your worktree root:\n\n", GroupConsolidationCompletionFileName)
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	fmt.Fprintf(sb, "  \"group_index\": %d,\n", groupIndex)
	sb.WriteString("  \"status\": \"complete\",\n")
	fmt.Fprintf(sb, "  \"branch_name\": \"%s\",\n", consolidatedBranch)
	sb.WriteString("  \"tasks_consolidated\": [\"task-id-1\", \"task-id-2\"],\n")
	sb.WriteString("  \"conflicts_resolved\": [\n")
	sb.WriteString("    {\"file\": \"path/to/file.go\", \"resolution\": \"Kept both changes, merged logic\"}\n")
	sb.WriteString("  ],\n")
	sb.WriteString("  \"verification\": {\n")
	sb.WriteString("    \"project_type\": \"go\",\n")
	sb.WriteString("    \"commands_run\": [\n")
	sb.WriteString("      {\"name\": \"build\", \"command\": \"go build ./...\", \"success\": true},\n")
	sb.WriteString("      {\"name\": \"lint\", \"command\": \"golangci-lint run\", \"success\": true},\n")
	sb.WriteString("      {\"name\": \"test\", \"command\": \"go test ./...\", \"success\": true}\n")
	sb.WriteString("    ],\n")
	sb.WriteString("    \"overall_success\": true,\n")
	sb.WriteString("    \"summary\": \"All checks passed\"\n")
	sb.WriteString("  },\n")
	sb.WriteString("  \"notes\": \"Any observations about the consolidated code\",\n")
	sb.WriteString("  \"issues_for_next_group\": [\"Any warnings or concerns for the next group\"]\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n\n")
	sb.WriteString("Use status \"failed\" if consolidation cannot be completed.\n")
}

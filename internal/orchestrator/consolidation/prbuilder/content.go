package prbuilder

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// buildBody generates the PR body content from tasks and options.
func buildBody(tasks []consolidation.CompletedTask, opts consolidation.PRBuildOptions) string {
	var body strings.Builder

	// Header
	body.WriteString("## Ultraplan Consolidation\n\n")
	body.WriteString(fmt.Sprintf("**Objective**: %s\n\n", opts.Objective))

	// Mode-specific information
	if opts.Mode == consolidation.ModeStacked {
		body.WriteString(fmt.Sprintf("**Group**: %d of %d\n\n", opts.GroupIndex+1, opts.TotalGroups))

		if opts.GroupIndex > 0 {
			body.WriteString(fmt.Sprintf("**Base**: Group %d\n\n", opts.GroupIndex))
		}
		if opts.GroupIndex < opts.TotalGroups-1 {
			body.WriteString(fmt.Sprintf("> **Note**: This PR must be merged before Group %d.\n\n", opts.GroupIndex+2))
		}
	}

	// Tasks included section
	body.WriteString("## Tasks Included\n\n")
	for _, task := range tasks {
		taskLine := fmt.Sprintf("- **%s**: %s", task.ID, task.Title)
		// Add summary from completion file if available
		if task.Completion != nil && task.Completion.Summary != "" {
			taskLine += fmt.Sprintf("\n  - %s", task.Completion.Summary)
		}
		body.WriteString(taskLine + "\n")
	}

	// Files changed section
	if len(opts.FilesChanged) > 0 {
		body.WriteString("\n## Files Changed\n\n")
		for _, f := range opts.FilesChanged {
			body.WriteString(fmt.Sprintf("- `%s`\n", f))
		}
	}

	// Aggregated context from task completion files
	aggregatedContent := buildAggregatedContext(tasks)
	if aggregatedContent != "" {
		body.WriteString(aggregatedContent)
	}

	// Synthesis review notes
	if opts.SynthesisNotes != "" || len(opts.Recommendations) > 0 {
		body.WriteString("\n## Synthesis Review Notes\n\n")
		if opts.SynthesisNotes != "" {
			body.WriteString(fmt.Sprintf("**Integration Notes**: %s\n\n", opts.SynthesisNotes))
		}
		if len(opts.Recommendations) > 0 {
			body.WriteString("**Recommendations**:\n")
			for _, rec := range opts.Recommendations {
				body.WriteString(fmt.Sprintf("- %s\n", rec))
			}
		}
	}

	return body.String()
}

// buildAggregatedContext aggregates context from all task completion files.
func buildAggregatedContext(tasks []consolidation.CompletedTask) string {
	var notes []string
	var issues []string
	var suggestions []string
	var dependencies []string
	seenDeps := make(map[string]bool)

	for _, task := range tasks {
		if task.Completion == nil {
			continue
		}

		// Collect notes
		if notesStr := task.Completion.Notes.String(); notesStr != "" {
			notes = append(notes, fmt.Sprintf("**%s**: %s", task.ID, notesStr))
		}

		// Collect issues (prefix with task ID for context)
		for _, issue := range task.Completion.Issues {
			if issue != "" {
				issues = append(issues, fmt.Sprintf("[%s] %s", task.ID, issue))
			}
		}

		// Collect suggestions
		for _, suggestion := range task.Completion.Suggestions {
			if suggestion != "" {
				suggestions = append(suggestions, fmt.Sprintf("[%s] %s", task.ID, suggestion))
			}
		}

		// Collect dependencies (deduplicated)
		for _, dep := range task.Completion.Dependencies {
			if dep != "" && !seenDeps[dep] {
				seenDeps[dep] = true
				dependencies = append(dependencies, dep)
			}
		}
	}

	// Build output
	var sb strings.Builder

	if len(notes) > 0 {
		sb.WriteString("\n## Implementation Notes\n\n")
		for _, note := range notes {
			sb.WriteString(fmt.Sprintf("- %s\n", note))
		}
	}

	if len(issues) > 0 {
		sb.WriteString("\n## Issues/Concerns Flagged\n\n")
		for _, issue := range issues {
			sb.WriteString(fmt.Sprintf("- %s\n", issue))
		}
	}

	if len(suggestions) > 0 {
		sb.WriteString("\n## Integration Suggestions\n\n")
		for _, suggestion := range suggestions {
			sb.WriteString(fmt.Sprintf("- %s\n", suggestion))
		}
	}

	if len(dependencies) > 0 {
		sb.WriteString("\n## New Dependencies\n\n")
		for _, dep := range dependencies {
			sb.WriteString(fmt.Sprintf("- `%s`\n", dep))
		}
	}

	return sb.String()
}

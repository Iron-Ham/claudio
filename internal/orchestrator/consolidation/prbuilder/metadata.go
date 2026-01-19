package prbuilder

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// buildTitle generates a PR title based on mode and objective.
func buildTitle(objective string, mode consolidation.Mode, groupIndex int, objectiveLimit int) string {
	truncatedObjective := truncateString(objective, objectiveLimit)

	if mode == consolidation.ModeSingle {
		return fmt.Sprintf("ultraplan: %s", truncatedObjective)
	}

	// Stacked mode: include group number
	// Use a shorter objective limit for stacked mode to accommodate group info
	stackedLimit := max(objectiveLimit-10, 20) // Leave room for "group N - "
	truncatedObjective = truncateString(objective, stackedLimit)
	return fmt.Sprintf("ultraplan: group %d - %s", groupIndex+1, truncatedObjective)
}

// buildLabels determines appropriate labels for the PR based on task content.
// The tasks parameter is reserved for future smart labeling based on task analysis.
func buildLabels(_ []consolidation.CompletedTask) []string {
	// Currently returns empty - labels are typically configured externally
	// via Config.PRLabels. This function exists for future smart labeling
	// based on task content analysis.
	return nil
}

// truncateString truncates a string to the given length, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

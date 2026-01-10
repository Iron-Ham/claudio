// Package view provides reusable view components for the TUI.
package view

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
)

// InstanceInfo provides instance metadata for conflict rendering.
// This decouples the view from the orchestrator package.
type InstanceInfo struct {
	ID   string
	Task string
}

// ConflictsView renders file conflict information.
type ConflictsView struct {
	// Conflicts is the list of file conflicts to display.
	Conflicts []conflict.FileConflict

	// Instances provides instance metadata for resolving instance IDs to display labels.
	Instances []InstanceInfo
}

// NewConflictsView creates a new ConflictsView with the given conflicts and instances.
func NewConflictsView(conflicts []conflict.FileConflict, instances []InstanceInfo) *ConflictsView {
	return &ConflictsView{
		Conflicts: conflicts,
		Instances: instances,
	}
}

// Render renders the detailed conflict panel showing all files and instances.
// The width parameter controls the maximum width of the rendered content.
func (v *ConflictsView) Render(width int) string {
	if len(v.Conflicts) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString(styles.Title.Render("⚠ File Conflicts"))
	b.WriteString("\n\n")

	b.WriteString(styles.Muted.Render("The following files have been modified by multiple instances:"))
	b.WriteString("\n\n")

	// Build instance ID to number and task mappings
	instanceNum := make(map[string]int)
	instanceTask := make(map[string]string)
	for i, inst := range v.Instances {
		instanceNum[inst.ID] = i + 1
		instanceTask[inst.ID] = inst.Task
	}

	// Render each conflict
	for i, c := range v.Conflicts {
		// File path in warning color
		fileLine := styles.Warning.Bold(true).Render(c.RelativePath)
		b.WriteString(fileLine)
		b.WriteString("\n")

		// List the instances that modified this file
		b.WriteString(styles.Muted.Render("  Modified by:"))
		b.WriteString("\n")
		for _, instID := range c.Instances {
			num := instanceNum[instID]
			task := instanceTask[instID]
			// Truncate task if too long
			maxTaskLen := max(width-15, 20)
			if len(task) > maxTaskLen {
				task = task[:maxTaskLen-3] + "..."
			}
			instanceLine := fmt.Sprintf("    [%d] %s", num, task)
			b.WriteString(styles.Text.Render(instanceLine))
			b.WriteString("\n")
		}

		// Add spacing between conflicts except for the last one
		if i < len(v.Conflicts)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.Muted.Render("Press [c] to close this view"))

	return styles.ContentBox.Width(width - 4).Render(b.String())
}

// RenderWarningBanner renders a compact warning banner for display in headers/status bars.
// This is useful for showing a brief conflict notification without the full detail panel.
func (v *ConflictsView) RenderWarningBanner() string {
	if len(v.Conflicts) == 0 {
		return ""
	}

	var b strings.Builder

	// Banner header with hint that it's interactive
	banner := styles.ConflictBanner.Render("⚠ FILE CONFLICT DETECTED")
	b.WriteString(banner)
	b.WriteString("  ")
	b.WriteString(styles.Muted.Render("(press [c] for details)"))
	b.WriteString("  ")

	// Build conflict details
	var conflictDetails []string
	for _, c := range v.Conflicts {
		// Find instance names/numbers for the conflicting instances
		var instanceLabels []string
		for _, instID := range c.Instances {
			// Find the instance index
			for i, inst := range v.Instances {
				if inst.ID == instID {
					instanceLabels = append(instanceLabels, fmt.Sprintf("[%d]", i+1))
					break
				}
			}
		}
		detail := fmt.Sprintf("%s (instances %s)", c.RelativePath, strings.Join(instanceLabels, ", "))
		conflictDetails = append(conflictDetails, detail)
	}

	// Show conflict files
	if len(conflictDetails) <= 2 {
		b.WriteString(styles.Warning.Render(strings.Join(conflictDetails, "; ")))
	} else {
		// Show count and first file
		b.WriteString(styles.Warning.Render(fmt.Sprintf("%d files: %s, ...", len(conflictDetails), conflictDetails[0])))
	}

	return b.String()
}

// HasConflicts returns true if there are any conflicts to display.
func (v *ConflictsView) HasConflicts() bool {
	return len(v.Conflicts) > 0
}

// ConflictCount returns the number of conflicting files.
func (v *ConflictsView) ConflictCount() int {
	return len(v.Conflicts)
}

// GetConflictingInstanceIDs returns a set of instance IDs that have conflicts.
// This is useful for highlighting instances in the UI that are involved in conflicts.
func (v *ConflictsView) GetConflictingInstanceIDs() map[string]bool {
	conflicting := make(map[string]bool)
	for _, c := range v.Conflicts {
		for _, instID := range c.Instances {
			conflicting[instID] = true
		}
	}
	return conflicting
}

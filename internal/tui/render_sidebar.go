package tui

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// renderSidebar renders the instance sidebar with pagination support
func (m Model) renderSidebar(width int, height int) string {
	var b strings.Builder

	// Sidebar title
	b.WriteString(styles.SidebarTitle.Render("Instances"))
	b.WriteString("\n")

	if m.instanceCount() == 0 {
		b.WriteString(styles.Muted.Render("No instances"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [a] to add"))
	} else {
		// Calculate available slots for instances
		// Reserve: 1 for title, 1 for blank line, 1 for add hint, 2 for scroll indicators, plus border padding
		reservedLines := 6
		availableSlots := height - reservedLines
		if availableSlots < 3 {
			availableSlots = 3 // Minimum to show at least a few instances
		}

		totalInstances := m.instanceCount()
		hasMoreAbove := m.sidebarScrollOffset > 0
		hasMoreBelow := m.sidebarScrollOffset+availableSlots < totalInstances

		// Show scroll up indicator if there are instances above
		if hasMoreAbove {
			scrollUp := styles.Muted.Render(fmt.Sprintf("▲ %d more above", m.sidebarScrollOffset))
			b.WriteString(scrollUp)
			b.WriteString("\n")
		}

		// Build a set of instance IDs that have conflicts
		conflictingInstances := make(map[string]bool)
		for _, c := range m.conflicts {
			for _, instID := range c.Instances {
				conflictingInstances[instID] = true
			}
		}

		// Calculate the visible range
		startIdx := m.sidebarScrollOffset
		endIdx := m.sidebarScrollOffset + availableSlots
		if endIdx > totalInstances {
			endIdx = totalInstances
		}

		// Render visible instances using helper
		for i := startIdx; i < endIdx; i++ {
			inst := m.session.Instances[i]
			b.WriteString(m.renderSidebarInstance(i, inst, conflictingInstances, width))
			b.WriteString("\n")
		}

		// Show scroll down indicator if there are instances below
		if hasMoreBelow {
			remaining := totalInstances - endIdx
			scrollDown := styles.Muted.Render(fmt.Sprintf("▼ %d more below", remaining))
			b.WriteString(scrollDown)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	// Add instance hint with navigation help when paginated
	if m.instanceCount() > 0 {
		addHint := styles.Muted.Render("[a]") + " " + styles.Muted.Render("add") + "  " +
			styles.Muted.Render("[↑↓]") + " " + styles.Muted.Render("nav")
		b.WriteString(addHint)
	} else {
		addHint := styles.Muted.Render("[a]") + " " + styles.Muted.Render("Add new")
		b.WriteString(addHint)
	}

	// Wrap in sidebar box
	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderSidebarInstance renders a single instance item in the sidebar
func (m Model) renderSidebarInstance(i int, inst *orchestrator.Instance, conflictingInstances map[string]bool, width int) string {
	// Status indicator (colored dot)
	statusColor := styles.StatusColor(string(inst.Status))
	dot := lipgloss.NewStyle().Foreground(statusColor).Render("●")

	// Instance number and truncated task
	maxTaskLen := width - 8 // Account for number, dot, padding
	if maxTaskLen < 10 {
		maxTaskLen = 10
	}
	label := fmt.Sprintf("%d %s", i+1, truncate(inst.Task, maxTaskLen))
	// Add conflict indicator if instance has conflicts
	if conflictingInstances[inst.ID] {
		label = fmt.Sprintf("%d ⚠ %s", i+1, truncate(inst.Task, maxTaskLen-2))
	}

	// Choose style based on active state and status
	var itemStyle lipgloss.Style
	if i == m.activeTab {
		if conflictingInstances[inst.ID] {
			// Active item with conflict - use warning background
			itemStyle = styles.SidebarItemInputNeeded
		} else if inst.Status == orchestrator.StatusWaitingInput {
			itemStyle = styles.SidebarItemInputNeeded
		} else {
			itemStyle = styles.SidebarItemActive
		}
	} else {
		itemStyle = styles.SidebarItem
		if conflictingInstances[inst.ID] {
			// Inactive but has conflict - use warning color
			itemStyle = itemStyle.Foreground(styles.WarningColor)
		} else if inst.Status == orchestrator.StatusWaitingInput {
			itemStyle = itemStyle.Foreground(styles.WarningColor)
		} else {
			itemStyle = itemStyle.Foreground(styles.MutedColor)
		}
	}

	// Combine dot and label
	return dot + " " + itemStyle.Render(label)
}

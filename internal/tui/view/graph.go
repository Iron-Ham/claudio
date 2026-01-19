package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/charmbracelet/lipgloss"
)

// GraphNode represents a node in the dependency graph.
type GraphNode struct {
	Instance    *orchestrator.Instance
	Level       int      // Topological level (0 = no dependencies)
	Dependents  []string // Instance IDs that depend on this
	DependsOn   []string // Instance IDs this depends on
	DisplayName string   // Truncated name for display
}

// GraphView handles rendering of the dependency graph sidebar.
type GraphView struct {
	// nodes maps instance ID to graph node
	nodes map[string]*GraphNode
	// levels groups nodes by their topological level
	levels [][]string
	// maxLevel is the deepest level in the graph
	maxLevel int
	// hasCycle is true if cyclic dependencies were detected
	hasCycle bool
	// cycleNodeIDs contains the IDs of nodes involved in cycles
	cycleNodeIDs []string
}

// NewGraphView creates a new graph view from a session.
func NewGraphView(session *orchestrator.Session) *GraphView {
	if session == nil || len(session.Instances) == 0 {
		return &GraphView{
			nodes:  make(map[string]*GraphNode),
			levels: make([][]string, 0),
		}
	}

	g := &GraphView{
		nodes: make(map[string]*GraphNode),
	}

	// Build nodes from instances
	for _, inst := range session.Instances {
		if inst == nil {
			continue // skip nil instances
		}
		g.nodes[inst.ID] = &GraphNode{
			Instance:    inst,
			DependsOn:   inst.DependsOn,
			Dependents:  inst.Dependents,
			DisplayName: truncateForGraph(inst.EffectiveName(), 25),
		}
	}

	// Calculate levels using topological sort
	g.calculateLevels()

	return g
}

// calculateLevels assigns topological levels to nodes.
// Level 0 = nodes with no dependencies.
// Level N = nodes whose dependencies are all at levels < N.
func (g *GraphView) calculateLevels() {
	// Track processed nodes
	processed := make(map[string]bool)
	g.levels = make([][]string, 0)

	for len(processed) < len(g.nodes) {
		var currentLevel []string

		for id, node := range g.nodes {
			if processed[id] {
				continue
			}

			// Check if all dependencies are processed
			allDepsProcessed := true
			for _, depID := range node.DependsOn {
				if _, exists := g.nodes[depID]; exists && !processed[depID] {
					allDepsProcessed = false
					break
				}
			}

			if allDepsProcessed {
				currentLevel = append(currentLevel, id)
				node.Level = len(g.levels)
			}
		}

		// Sort current level by instance name for consistent display
		sort.Slice(currentLevel, func(i, j int) bool {
			nodeI, nodeJ := g.nodes[currentLevel[i]], g.nodes[currentLevel[j]]
			if nodeI == nil || nodeJ == nil {
				return false
			}
			return nodeI.DisplayName < nodeJ.DisplayName
		})

		for _, id := range currentLevel {
			processed[id] = true
		}

		if len(currentLevel) > 0 {
			g.levels = append(g.levels, currentLevel)
		} else {
			// Cycle detection - break out to avoid infinite loop
			// Track cyclic nodes and add them to a final level
			var cycleNodes []string
			for id := range g.nodes {
				if !processed[id] {
					cycleNodes = append(cycleNodes, id)
					processed[id] = true
				}
			}
			if len(cycleNodes) > 0 {
				g.hasCycle = true
				g.cycleNodeIDs = cycleNodes
				g.levels = append(g.levels, cycleNodes)
			}
			break
		}
	}

	g.maxLevel = max(len(g.levels)-1, 0)
}

// RenderGraphSidebar renders the dependency graph in the sidebar.
func (g *GraphView) RenderGraphSidebar(state DashboardState, width, height int) string {
	var b strings.Builder

	// Sidebar title
	b.WriteString(styles.SidebarTitle.Render("Dependency Graph"))
	b.WriteString("\n")

	// Show cycle warning if detected
	if g.hasCycle {
		warningStyle := lipgloss.NewStyle().Foreground(styles.WarningColor)
		b.WriteString(warningStyle.Render("\u26a0 Cycle detected in dependencies"))
		b.WriteString("\n")
	}

	session := state.Session()
	if session == nil || len(session.Instances) == 0 {
		b.WriteString(styles.Muted.Render("No instances"))
		b.WriteString("\n")
		b.WriteString(styles.Muted.Render("Press [:a] to add"))
		b.WriteString("\n\n")
		b.WriteString(g.renderHelpHints(false))
		return styles.Sidebar.Width(width - 2).Render(b.String())
	}

	// Calculate available height
	reservedLines := 6 // title, blank line, hints, etc.
	availableLines := max(height-reservedLines, 5)

	// Render the graph levels
	linesUsed := 0
	activeInstanceIdx := state.ActiveTab()
	var activeInstance *orchestrator.Instance
	if activeInstanceIdx >= 0 && activeInstanceIdx < len(session.Instances) {
		activeInstance = session.Instances[activeInstanceIdx]
	}

	for levelIdx, nodeIDs := range g.levels {
		if linesUsed >= availableLines {
			remaining := g.countRemainingNodes(levelIdx)
			if remaining > 0 {
				b.WriteString(styles.Muted.Render(fmt.Sprintf("\u25bc %d more nodes below", remaining)))
				b.WriteString("\n")
			}
			break
		}

		// Render level header
		levelHeader := g.renderLevelHeader(levelIdx)
		b.WriteString(levelHeader)
		b.WriteString("\n")
		linesUsed++

		// Render nodes at this level
		for _, nodeID := range nodeIDs {
			if linesUsed >= availableLines {
				break
			}

			node := g.nodes[nodeID]
			if node == nil {
				continue
			}

			isActive := activeInstance != nil && node.Instance.ID == activeInstance.ID
			nodeLine := g.renderNode(node, isActive, width-4)
			b.WriteString(nodeLine)
			b.WriteString("\n")
			linesUsed++

			// Render dependency arrows if there are dependents
			if len(node.Dependents) > 0 && linesUsed < availableLines {
				arrowLine := g.renderDependencyArrows(node, width-4)
				if arrowLine != "" {
					b.WriteString(arrowLine)
					b.WriteString("\n")
					linesUsed++
				}
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(g.renderHelpHints(len(session.Instances) > 0))

	return styles.Sidebar.Width(width - 2).Render(b.String())
}

// renderLevelHeader renders the header for a dependency level.
func (g *GraphView) renderLevelHeader(level int) string {
	var label string
	switch level {
	case 0:
		label = "Root Tasks"
	case g.maxLevel:
		label = "Final Tasks"
	default:
		label = fmt.Sprintf("Level %d", level+1)
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(styles.SecondaryColor).
		Bold(true)

	return headerStyle.Render(fmt.Sprintf("─── %s ───", label))
}

// renderNode renders a single node in the graph.
func (g *GraphView) renderNode(node *GraphNode, isActive bool, maxWidth int) string {
	// Status indicator
	statusIcon := g.statusIcon(node.Instance.Status)

	// Build the node line
	// Use rune-based truncation to avoid corrupting multi-byte Unicode characters
	name := truncateForGraph(node.DisplayName, maxWidth-4)

	var style lipgloss.Style
	if isActive {
		style = styles.SidebarItemActive
	} else {
		style = styles.SidebarItem
	}

	// Add dependency count indicator
	depInfo := ""
	if len(node.DependsOn) > 0 {
		depInfo = fmt.Sprintf(" [%d\u2191]", len(node.DependsOn)) // up arrow for dependencies
	}
	if len(node.Dependents) > 0 {
		depInfo += fmt.Sprintf(" [%d\u2193]", len(node.Dependents)) // down arrow for dependents
	}

	return fmt.Sprintf("  %s %s%s", statusIcon, style.Render(name), styles.Muted.Render(depInfo))
}

// renderDependencyArrows renders the dependency relationship arrows.
func (g *GraphView) renderDependencyArrows(node *GraphNode, maxWidth int) string {
	if len(node.Dependents) == 0 {
		return ""
	}

	// Use box-drawing characters for visual clarity
	var dependentNames []string
	for _, depID := range node.Dependents {
		if depNode, exists := g.nodes[depID]; exists {
			// Use rune-based truncation to avoid corrupting multi-byte Unicode characters
			name := truncateForGraph(depNode.DisplayName, 12)
			dependentNames = append(dependentNames, name)
		}
	}

	if len(dependentNames) == 0 {
		return ""
	}

	arrow := "  \u2514\u2192 " // └→
	targets := strings.Join(dependentNames, ", ")
	// Use rune-based truncation to avoid corrupting multi-byte Unicode characters
	targets = truncateForGraph(targets, maxWidth-6)

	return styles.Muted.Render(arrow + targets)
}

// statusIcon returns the appropriate icon for an instance status.
func (g *GraphView) statusIcon(status orchestrator.InstanceStatus) string {
	switch status {
	case orchestrator.StatusCompleted:
		return lipgloss.NewStyle().Foreground(styles.SecondaryColor).Render("\u2713") // ✓ (green)
	case orchestrator.StatusWorking:
		return lipgloss.NewStyle().Foreground(styles.BlueColor).Render("\u25cf") // ● (blue)
	case orchestrator.StatusWaitingInput:
		return lipgloss.NewStyle().Foreground(styles.WarningColor).Render("\u25cf") // ● (amber)
	case orchestrator.StatusError:
		return lipgloss.NewStyle().Foreground(styles.ErrorColor).Render("\u2717") // ✗ (red)
	case orchestrator.StatusPending:
		return lipgloss.NewStyle().Foreground(styles.MutedColor).Render("\u25cb") // ○ (gray)
	case orchestrator.StatusPaused:
		return lipgloss.NewStyle().Foreground(styles.MutedColor).Render("\u2016") // ‖ (gray)
	default:
		return lipgloss.NewStyle().Foreground(styles.MutedColor).Render("\u25cb") // ○ (gray)
	}
}

// countRemainingNodes counts nodes that weren't displayed.
func (g *GraphView) countRemainingNodes(fromLevel int) int {
	count := 0
	for i := fromLevel; i < len(g.levels); i++ {
		count += len(g.levels[i])
	}
	return count
}

// renderHelpHints renders the help hints for the graph view.
func (g *GraphView) renderHelpHints(hasInstances bool) string {
	hintStyle := styles.Muted
	if hasInstances {
		return hintStyle.Render("[h/l]") + " " + hintStyle.Render("nav") + "  " +
			hintStyle.Render("[d]") + " " + hintStyle.Render("list view") + "  " +
			hintStyle.Render("[:a]") + " " + hintStyle.Render("add")
	}
	return hintStyle.Render("[:a]") + " " + hintStyle.Render("Add new")
}

// truncateForGraph truncates a string for display in the graph.
// Uses rune counting to properly handle multi-byte Unicode characters.
func truncateForGraph(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	// For very small maxLen, just truncate without ellipsis
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// HasDependencies returns true if the session has any instance dependencies.
func HasDependencies(session *orchestrator.Session) bool {
	if session == nil {
		return false
	}
	for _, inst := range session.Instances {
		if len(inst.DependsOn) > 0 || len(inst.Dependents) > 0 {
			return true
		}
	}
	return false
}

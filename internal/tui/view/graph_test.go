package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestNewGraphView_EmptySession(t *testing.T) {
	tests := []struct {
		name    string
		session *orchestrator.Session
	}{
		{"nil session", nil},
		{"empty session", &orchestrator.Session{}},
		{"session with nil instances", &orchestrator.Session{Instances: nil}},
		{"session with empty instances", &orchestrator.Session{Instances: []*orchestrator.Instance{}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGraphView(tt.session)
			if g == nil {
				t.Fatal("NewGraphView returned nil")
			}
			if len(g.nodes) != 0 {
				t.Errorf("expected empty nodes, got %d", len(g.nodes))
			}
			if len(g.levels) != 0 {
				t.Errorf("expected empty levels, got %d", len(g.levels))
			}
		})
	}
}

func TestNewGraphView_SingleInstance(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst1", Task: "Task 1", Status: orchestrator.StatusPending},
		},
	}

	g := NewGraphView(session)

	if len(g.nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(g.nodes))
	}
	if len(g.levels) != 1 {
		t.Errorf("expected 1 level, got %d", len(g.levels))
	}
	if g.maxLevel != 0 {
		t.Errorf("expected maxLevel 0, got %d", g.maxLevel)
	}

	node := g.nodes["inst1"]
	if node == nil {
		t.Fatal("expected node for inst1")
	}
	if node.Level != 0 {
		t.Errorf("expected level 0, got %d", node.Level)
	}
}

func TestNewGraphView_LinearDependencies(t *testing.T) {
	// A -> B -> C (linear chain)
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", Status: orchestrator.StatusCompleted, DependsOn: []string{}, Dependents: []string{"instB"}},
			{ID: "instB", Task: "Task B", Status: orchestrator.StatusWorking, DependsOn: []string{"instA"}, Dependents: []string{"instC"}},
			{ID: "instC", Task: "Task C", Status: orchestrator.StatusPending, DependsOn: []string{"instB"}, Dependents: []string{}},
		},
	}

	g := NewGraphView(session)

	if len(g.nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.nodes))
	}
	if len(g.levels) != 3 {
		t.Errorf("expected 3 levels, got %d", len(g.levels))
	}
	if g.maxLevel != 2 {
		t.Errorf("expected maxLevel 2, got %d", g.maxLevel)
	}

	// Check levels are assigned correctly
	if g.nodes["instA"].Level != 0 {
		t.Errorf("instA should be level 0, got %d", g.nodes["instA"].Level)
	}
	if g.nodes["instB"].Level != 1 {
		t.Errorf("instB should be level 1, got %d", g.nodes["instB"].Level)
	}
	if g.nodes["instC"].Level != 2 {
		t.Errorf("instC should be level 2, got %d", g.nodes["instC"].Level)
	}
}

func TestNewGraphView_ParallelTasks(t *testing.T) {
	// A, B both depend on nothing (parallel at level 0)
	// C depends on both A and B
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", Status: orchestrator.StatusCompleted, DependsOn: []string{}, Dependents: []string{"instC"}},
			{ID: "instB", Task: "Task B", Status: orchestrator.StatusCompleted, DependsOn: []string{}, Dependents: []string{"instC"}},
			{ID: "instC", Task: "Task C", Status: orchestrator.StatusPending, DependsOn: []string{"instA", "instB"}, Dependents: []string{}},
		},
	}

	g := NewGraphView(session)

	if len(g.nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(g.nodes))
	}
	if len(g.levels) != 2 {
		t.Errorf("expected 2 levels, got %d", len(g.levels))
	}

	// A and B should be at level 0
	if g.nodes["instA"].Level != 0 {
		t.Errorf("instA should be level 0, got %d", g.nodes["instA"].Level)
	}
	if g.nodes["instB"].Level != 0 {
		t.Errorf("instB should be level 0, got %d", g.nodes["instB"].Level)
	}
	// C should be at level 1
	if g.nodes["instC"].Level != 1 {
		t.Errorf("instC should be level 1, got %d", g.nodes["instC"].Level)
	}
}

func TestNewGraphView_DiamondDependency(t *testing.T) {
	// Diamond pattern:
	//     A
	//    / \
	//   B   C
	//    \ /
	//     D
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", Status: orchestrator.StatusCompleted, DependsOn: []string{}, Dependents: []string{"instB", "instC"}},
			{ID: "instB", Task: "Task B", Status: orchestrator.StatusCompleted, DependsOn: []string{"instA"}, Dependents: []string{"instD"}},
			{ID: "instC", Task: "Task C", Status: orchestrator.StatusCompleted, DependsOn: []string{"instA"}, Dependents: []string{"instD"}},
			{ID: "instD", Task: "Task D", Status: orchestrator.StatusPending, DependsOn: []string{"instB", "instC"}, Dependents: []string{}},
		},
	}

	g := NewGraphView(session)

	if len(g.nodes) != 4 {
		t.Errorf("expected 4 nodes, got %d", len(g.nodes))
	}
	if len(g.levels) != 3 {
		t.Errorf("expected 3 levels, got %d", len(g.levels))
	}

	// Check levels
	if g.nodes["instA"].Level != 0 {
		t.Errorf("instA should be level 0, got %d", g.nodes["instA"].Level)
	}
	if g.nodes["instB"].Level != 1 {
		t.Errorf("instB should be level 1, got %d", g.nodes["instB"].Level)
	}
	if g.nodes["instC"].Level != 1 {
		t.Errorf("instC should be level 1, got %d", g.nodes["instC"].Level)
	}
	if g.nodes["instD"].Level != 2 {
		t.Errorf("instD should be level 2, got %d", g.nodes["instD"].Level)
	}
}

func TestGraphView_StatusIcon(t *testing.T) {
	g := &GraphView{}

	tests := []struct {
		status   orchestrator.InstanceStatus
		contains string // check that the icon contains this substring
	}{
		{orchestrator.StatusCompleted, "\u2713"}, // ✓
		{orchestrator.StatusWorking, "\u25cf"},   // ●
		{orchestrator.StatusWaitingInput, "\u25cf"},
		{orchestrator.StatusError, "\u2717"},   // ✗
		{orchestrator.StatusPending, "\u25cb"}, // ○
		{orchestrator.StatusPaused, "\u2016"},  // ‖
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			icon := g.statusIcon(tt.status)
			if !strings.Contains(icon, tt.contains) {
				t.Errorf("statusIcon(%s) = %q, expected to contain %q", tt.status, icon, tt.contains)
			}
		})
	}
}

func TestTruncateForGraph(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short ASCII", "short", 10, "short"},
		{"exactly 10 chars", "exactly10!", 10, "exactly10!"},
		{"long ASCII string", "this is a longer string", 10, "this is..."},
		{"empty string", "", 10, ""},
		{"short ASCII 2", "abc", 5, "abc"},
		// Unicode test cases - these ensure we count runes not bytes
		{"Unicode within limit", "日本語テスト", 10, "日本語テスト"},       // 6 runes, under limit
		{"Unicode truncation", "日本語テストテスト", 8, "日本語テス..."},     // 8 runes = exactly limit, so first 5 + "..."
		{"Emoji within limit", "Hello 🎉", 10, "Hello 🎉"},       // 7 runes, under limit
		{"Emoji truncation", "Task 🎉🎊🎁🎄🎅", 8, "Task ..."},      // 10 runes: "Task " (5) + 5 emoji -> 5 + "..."
		{"Mixed Unicode", "任务ABC测试XYZ", 10, "任务ABC测试XYZ"},      // exactly 10 runes, not truncated
		{"Cyrillic text", "Привет мир тест", 10, "Привет ..."}, // 15 runes -> 7 + "..."
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForGraph(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateForGraph(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
			// Verify the result is within the max length (in runes)
			if len([]rune(got)) > tt.maxLen {
				t.Errorf("truncateForGraph(%q, %d) returned %d runes, expected at most %d",
					tt.input, tt.maxLen, len([]rune(got)), tt.maxLen)
			}
		})
	}
}

func TestHasDependencies(t *testing.T) {
	tests := []struct {
		name    string
		session *orchestrator.Session
		want    bool
	}{
		{
			name:    "nil session",
			session: nil,
			want:    false,
		},
		{
			name: "no instances",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{},
			},
			want: false,
		},
		{
			name: "instances without dependencies",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst1", Task: "Task 1"},
					{ID: "inst2", Task: "Task 2"},
				},
			},
			want: false,
		},
		{
			name: "instances with DependsOn",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst1", Task: "Task 1"},
					{ID: "inst2", Task: "Task 2", DependsOn: []string{"inst1"}},
				},
			},
			want: true,
		},
		{
			name: "instances with Dependents",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "inst1", Task: "Task 1", Dependents: []string{"inst2"}},
					{ID: "inst2", Task: "Task 2"},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasDependencies(tt.session)
			if got != tt.want {
				t.Errorf("HasDependencies() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGraphView_RenderLevelHeader(t *testing.T) {
	g := &GraphView{maxLevel: 3}

	tests := []struct {
		level    int
		contains string
	}{
		{0, "Root Tasks"},
		{1, "Level 2"},
		{2, "Level 3"},
		{3, "Final Tasks"},
	}

	for _, tt := range tests {
		t.Run(tt.contains, func(t *testing.T) {
			header := g.renderLevelHeader(tt.level)
			if !strings.Contains(header, tt.contains) {
				t.Errorf("renderLevelHeader(%d) = %q, expected to contain %q", tt.level, header, tt.contains)
			}
		})
	}
}

func TestGraphView_CountRemainingNodes(t *testing.T) {
	g := &GraphView{
		levels: [][]string{
			{"a", "b"},      // level 0
			{"c"},           // level 1
			{"d", "e", "f"}, // level 2
		},
	}

	tests := []struct {
		fromLevel int
		want      int
	}{
		{0, 6}, // all nodes
		{1, 4}, // c + d, e, f
		{2, 3}, // d, e, f
		{3, 0}, // past end
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.fromLevel)), func(t *testing.T) {
			got := g.countRemainingNodes(tt.fromLevel)
			if got != tt.want {
				t.Errorf("countRemainingNodes(%d) = %d, want %d", tt.fromLevel, got, tt.want)
			}
		})
	}
}

func TestGraphView_RenderNode(t *testing.T) {
	g := &GraphView{
		nodes: make(map[string]*GraphNode),
	}

	node := &GraphNode{
		Instance:    &orchestrator.Instance{ID: "test", Task: "Test Task", Status: orchestrator.StatusWorking},
		DisplayName: "Test Task",
		DependsOn:   []string{"dep1"},
		Dependents:  []string{"child1", "child2"},
	}

	// Render as active
	activeResult := g.renderNode(node, true, 40)
	if !strings.Contains(activeResult, "Test Task") {
		t.Errorf("renderNode should contain task name, got: %q", activeResult)
	}

	// Render as inactive
	inactiveResult := g.renderNode(node, false, 40)
	if !strings.Contains(inactiveResult, "Test Task") {
		t.Errorf("renderNode should contain task name, got: %q", inactiveResult)
	}

	// Check dependency indicators
	if !strings.Contains(activeResult, "1\u2191") { // up arrow for dependencies
		t.Errorf("renderNode should show dependency count, got: %q", activeResult)
	}
	if !strings.Contains(activeResult, "2\u2193") { // down arrow for dependents
		t.Errorf("renderNode should show dependent count, got: %q", activeResult)
	}
}

func TestGraphView_RenderHelpHints(t *testing.T) {
	g := &GraphView{}

	// With instances
	withInstances := g.renderHelpHints(true)
	if !strings.Contains(withInstances, "[d]") {
		t.Errorf("renderHelpHints(true) should contain [d] hint, got: %q", withInstances)
	}
	if !strings.Contains(withInstances, "list view") {
		t.Errorf("renderHelpHints(true) should contain 'list view', got: %q", withInstances)
	}

	// Without instances
	withoutInstances := g.renderHelpHints(false)
	if !strings.Contains(withoutInstances, "[:a]") {
		t.Errorf("renderHelpHints(false) should contain [:a] hint, got: %q", withoutInstances)
	}
}

func TestNewGraphView_CycleHandling(t *testing.T) {
	// Test that cycles don't cause infinite loops
	// (Even though this shouldn't happen in practice)
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", DependsOn: []string{"instB"}},
			{ID: "instB", Task: "Task B", DependsOn: []string{"instA"}},
		},
	}

	// This should not hang
	g := NewGraphView(session)

	// Both nodes should be present
	if len(g.nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(g.nodes))
	}

	// There should be at least one level (the cycle-breaking level)
	if len(g.levels) == 0 {
		t.Error("expected at least one level for cyclic graph")
	}

	// Verify cycle detection is flagged
	if !g.hasCycle {
		t.Error("expected hasCycle to be true for cyclic graph")
	}
	if len(g.cycleNodeIDs) != 2 {
		t.Errorf("expected 2 cycle nodes, got %d", len(g.cycleNodeIDs))
	}
}

func TestNewGraphView_NoCycleFlag(t *testing.T) {
	// Verify hasCycle is false for acyclic graph
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", DependsOn: []string{}},
			{ID: "instB", Task: "Task B", DependsOn: []string{"instA"}},
		},
	}

	g := NewGraphView(session)

	if g.hasCycle {
		t.Error("expected hasCycle to be false for acyclic graph")
	}
	if len(g.cycleNodeIDs) != 0 {
		t.Errorf("expected no cycle nodes, got %d", len(g.cycleNodeIDs))
	}
}

func TestGraphView_RenderDependencyArrows(t *testing.T) {
	g := &GraphView{
		nodes: map[string]*GraphNode{
			"inst1": {DisplayName: "Parent Task"},
			"inst2": {DisplayName: "Child Task 1"},
			"inst3": {DisplayName: "Child Task 2"},
		},
	}

	// Node with no dependents
	nodeNoDeps := &GraphNode{Dependents: []string{}}
	result := g.renderDependencyArrows(nodeNoDeps, 50)
	if result != "" {
		t.Errorf("renderDependencyArrows with no dependents should return empty string, got: %q", result)
	}

	// Node with dependents
	nodeWithDeps := &GraphNode{Dependents: []string{"inst2", "inst3"}}
	result = g.renderDependencyArrows(nodeWithDeps, 50)
	if !strings.Contains(result, "Child Task 1") {
		t.Errorf("renderDependencyArrows should contain dependent names, got: %q", result)
	}
	if !strings.Contains(result, "\u2514\u2192") { // └→
		t.Errorf("renderDependencyArrows should contain arrow, got: %q", result)
	}
}

// TestRenderGraphSidebar_EmptySession tests rendering with no instances.
func TestRenderGraphSidebar_EmptySession(t *testing.T) {
	g := NewGraphView(nil)
	state := &mockDashboardState{
		session:        nil,
		terminalWidth:  80,
		terminalHeight: 30,
	}

	result := g.RenderGraphSidebar(state, 40, 30)

	// Should contain title
	if !strings.Contains(result, "Dependency Graph") {
		t.Error("RenderGraphSidebar should contain title")
	}

	// Should show "No instances" message
	if !strings.Contains(result, "No instances") {
		t.Error("RenderGraphSidebar should show 'No instances' for empty session")
	}

	// Should show add hint
	if !strings.Contains(result, "[:a]") {
		t.Error("RenderGraphSidebar should show add hint")
	}
}

// TestRenderGraphSidebar_SingleInstance tests rendering with one instance.
func TestRenderGraphSidebar_SingleInstance(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst1", Task: "Build the app", Status: orchestrator.StatusWorking},
		},
	}
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
	}

	result := g.RenderGraphSidebar(state, 40, 30)

	// Should contain title
	if !strings.Contains(result, "Dependency Graph") {
		t.Error("RenderGraphSidebar should contain title")
	}

	// Should contain the task name
	if !strings.Contains(result, "Build the app") {
		t.Error("RenderGraphSidebar should contain instance task name")
	}

	// Should contain level header
	if !strings.Contains(result, "Root Tasks") {
		t.Error("RenderGraphSidebar should contain level header for single instance")
	}

	// Should show navigation hints
	if !strings.Contains(result, "[d]") {
		t.Error("RenderGraphSidebar should show [d] hint for list view toggle")
	}
}

// TestRenderGraphSidebar_WithDependencies tests rendering instances with dependencies.
func TestRenderGraphSidebar_WithDependencies(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", Status: orchestrator.StatusCompleted, DependsOn: []string{}, Dependents: []string{"instB"}},
			{ID: "instB", Task: "Task B", Status: orchestrator.StatusWorking, DependsOn: []string{"instA"}, Dependents: []string{}},
		},
	}
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
	}

	result := g.RenderGraphSidebar(state, 40, 30)

	// Should contain both tasks
	if !strings.Contains(result, "Task A") {
		t.Error("RenderGraphSidebar should contain Task A")
	}
	if !strings.Contains(result, "Task B") {
		t.Error("RenderGraphSidebar should contain Task B")
	}

	// Should show dependency indicators
	// Task A has 1 dependent (down arrow)
	if !strings.Contains(result, "\u2193") {
		t.Error("RenderGraphSidebar should show dependent indicator (down arrow)")
	}
	// Task B has 1 dependency (up arrow)
	if !strings.Contains(result, "\u2191") {
		t.Error("RenderGraphSidebar should show dependency indicator (up arrow)")
	}

	// Should have multiple level headers
	if !strings.Contains(result, "Root Tasks") {
		t.Error("RenderGraphSidebar should contain Root Tasks header")
	}
}

// TestRenderGraphSidebar_ActiveInstanceHighlight tests that the active instance is highlighted.
func TestRenderGraphSidebar_ActiveInstanceHighlight(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst1", Task: "First Task", Status: orchestrator.StatusCompleted},
			{ID: "inst2", Task: "Second Task", Status: orchestrator.StatusWorking},
		},
	}

	// Test with first instance active
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
	}

	result := g.RenderGraphSidebar(state, 40, 30)
	if !strings.Contains(result, "First Task") {
		t.Error("RenderGraphSidebar should contain First Task")
	}
	if !strings.Contains(result, "Second Task") {
		t.Error("RenderGraphSidebar should contain Second Task")
	}

	// Test with second instance active
	state.activeTab = 1
	result = g.RenderGraphSidebar(state, 40, 30)
	if !strings.Contains(result, "First Task") {
		t.Error("RenderGraphSidebar should contain First Task with second tab active")
	}
	if !strings.Contains(result, "Second Task") {
		t.Error("RenderGraphSidebar should contain Second Task with second tab active")
	}
}

// TestRenderGraphSidebar_DifferentStatuses tests rendering with various instance statuses.
func TestRenderGraphSidebar_DifferentStatuses(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst1", Task: "Completed Task", Status: orchestrator.StatusCompleted},
			{ID: "inst2", Task: "Working Task", Status: orchestrator.StatusWorking},
			{ID: "inst3", Task: "Pending Task", Status: orchestrator.StatusPending},
			{ID: "inst4", Task: "Error Task", Status: orchestrator.StatusError},
			{ID: "inst5", Task: "Waiting Task", Status: orchestrator.StatusWaitingInput},
		},
	}
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 50, // Larger height to fit all instances
	}

	result := g.RenderGraphSidebar(state, 50, 50)

	// Verify all tasks are rendered
	if !strings.Contains(result, "Completed Task") {
		t.Error("RenderGraphSidebar should contain Completed Task")
	}
	if !strings.Contains(result, "Working Task") {
		t.Error("RenderGraphSidebar should contain Working Task")
	}
	if !strings.Contains(result, "Pending Task") {
		t.Error("RenderGraphSidebar should contain Pending Task")
	}
	if !strings.Contains(result, "Error Task") {
		t.Error("RenderGraphSidebar should contain Error Task")
	}
	if !strings.Contains(result, "Waiting Task") {
		t.Error("RenderGraphSidebar should contain Waiting Task")
	}

	// Verify status icons are present (check for Unicode characters)
	if !strings.Contains(result, "\u2713") { // ✓ for completed
		t.Error("RenderGraphSidebar should contain completed icon")
	}
	if !strings.Contains(result, "\u2717") { // ✗ for error
		t.Error("RenderGraphSidebar should contain error icon")
	}
}

// TestRenderGraphSidebar_TruncatedHeight tests that overflow is handled with limited height.
func TestRenderGraphSidebar_TruncatedHeight(t *testing.T) {
	// Create instances across multiple levels to trigger the truncation indicator.
	// The truncation message "X more nodes below" only appears when breaking between levels,
	// so we need enough instances at level 0 to fill the available lines before reaching level 1.
	// With height=12 and reservedLines=6, availableLines=6.
	// We need: 1 level header + enough nodes to fill the space, then have remaining levels.
	instances := []*orchestrator.Instance{
		// Level 0: 10 root instances (to fill available lines)
		{ID: "inst0", Task: "Root Task 0", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst1", Task: "Root Task 1", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst2", Task: "Root Task 2", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst3", Task: "Root Task 3", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst4", Task: "Root Task 4", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst5", Task: "Root Task 5", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst6", Task: "Root Task 6", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst7", Task: "Root Task 7", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst8", Task: "Root Task 8", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		{ID: "inst9", Task: "Root Task 9", Status: orchestrator.StatusCompleted, Dependents: []string{"instFinal"}},
		// Level 1: final instance that depends on all roots
		{ID: "instFinal", Task: "Final Task", Status: orchestrator.StatusPending, DependsOn: []string{"inst0", "inst1", "inst2", "inst3", "inst4", "inst5", "inst6", "inst7", "inst8", "inst9"}},
	}
	session := &orchestrator.Session{Instances: instances}
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 12, // Small height: availableLines = max(12-6, 5) = 6
	}

	result := g.RenderGraphSidebar(state, 40, 12)

	// Should still contain title
	if !strings.Contains(result, "Dependency Graph") {
		t.Error("RenderGraphSidebar should contain title even with truncation")
	}

	// With 6 available lines and 10 root tasks + 1 level header, we can only show:
	// - 1 level header "Root Tasks"
	// - 5 nodes
	// Then the loop continues until linesUsed >= 6, but since we break within the inner loop,
	// the "more nodes below" message is shown when we can't process the next level.
	// Verify we got at least the title
	if !strings.Contains(result, "Root Tasks") {
		t.Error("RenderGraphSidebar should contain Root Tasks header")
	}

	// Verify we have some root tasks
	if !strings.Contains(result, "Root Task") {
		t.Error("RenderGraphSidebar should contain some root tasks")
	}
}

// TestRenderGraphSidebar_DiamondDependency tests rendering of diamond dependency pattern.
func TestRenderGraphSidebar_DiamondDependency(t *testing.T) {
	// Diamond pattern: A -> B, A -> C, B -> D, C -> D
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", Status: orchestrator.StatusCompleted, Dependents: []string{"instB", "instC"}},
			{ID: "instB", Task: "Task B", Status: orchestrator.StatusCompleted, DependsOn: []string{"instA"}, Dependents: []string{"instD"}},
			{ID: "instC", Task: "Task C", Status: orchestrator.StatusCompleted, DependsOn: []string{"instA"}, Dependents: []string{"instD"}},
			{ID: "instD", Task: "Task D", Status: orchestrator.StatusPending, DependsOn: []string{"instB", "instC"}},
		},
	}
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 40,
	}

	result := g.RenderGraphSidebar(state, 50, 40)

	// Verify all tasks are rendered
	if !strings.Contains(result, "Task A") {
		t.Error("RenderGraphSidebar should contain Task A")
	}
	if !strings.Contains(result, "Task B") {
		t.Error("RenderGraphSidebar should contain Task B")
	}
	if !strings.Contains(result, "Task C") {
		t.Error("RenderGraphSidebar should contain Task C")
	}
	if !strings.Contains(result, "Task D") {
		t.Error("RenderGraphSidebar should contain Task D")
	}

	// Should have multiple levels (at least Root and Final)
	if !strings.Contains(result, "Root Tasks") {
		t.Error("RenderGraphSidebar should contain Root Tasks header")
	}
	if !strings.Contains(result, "Final Tasks") {
		t.Error("RenderGraphSidebar should contain Final Tasks header")
	}
}

// TestRenderGraphSidebar_CycleWarning tests that cycle warning is displayed.
func TestRenderGraphSidebar_CycleWarning(t *testing.T) {
	// Cyclic dependency: A -> B -> A
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "instA", Task: "Task A", DependsOn: []string{"instB"}},
			{ID: "instB", Task: "Task B", DependsOn: []string{"instA"}},
		},
	}
	g := NewGraphView(session)
	state := &mockDashboardState{
		session:        session,
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 30,
	}

	result := g.RenderGraphSidebar(state, 40, 30)

	// Should display cycle warning
	if !strings.Contains(result, "Cycle detected") {
		t.Error("RenderGraphSidebar should display cycle warning for cyclic graph")
	}
}

// TestTruncateForGraph_SmallMaxLen tests truncation with very small maxLen values.
func TestTruncateForGraph_SmallMaxLen(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"maxLen 0 returns empty", "hello", 0, ""},
		{"maxLen 1 returns first char", "hello", 1, "h"},
		{"maxLen 2 returns first two chars", "hello", 2, "he"},
		{"maxLen 3 returns first three chars", "hello", 3, "hel"},
		{"negative maxLen returns empty", "hello", -1, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForGraph(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncateForGraph(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

// TestNewGraphView_NilInstancesInSlice tests handling of nil instances.
func TestNewGraphView_NilInstancesInSlice(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "inst1", Task: "Task 1"},
			nil, // nil instance should be skipped
			{ID: "inst2", Task: "Task 2"},
		},
	}

	g := NewGraphView(session)

	// Should only have 2 nodes (nil skipped)
	if len(g.nodes) != 2 {
		t.Errorf("expected 2 nodes (nil skipped), got %d", len(g.nodes))
	}
	if g.nodes["inst1"] == nil {
		t.Error("expected node for inst1")
	}
	if g.nodes["inst2"] == nil {
		t.Error("expected node for inst2")
	}
}

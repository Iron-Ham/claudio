package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// mockDashboardState implements DashboardState for testing
type mockDashboardState struct {
	session             *orchestrator.Session
	activeTab           int
	sidebarScrollOffset int
	conflicts           []conflict.FileConflict
	terminalWidth       int
	terminalHeight      int
	isAddingTask        bool
}

func (m *mockDashboardState) Session() *orchestrator.Session     { return m.session }
func (m *mockDashboardState) ActiveTab() int                     { return m.activeTab }
func (m *mockDashboardState) SidebarScrollOffset() int           { return m.sidebarScrollOffset }
func (m *mockDashboardState) Conflicts() []conflict.FileConflict { return m.conflicts }
func (m *mockDashboardState) TerminalWidth() int                 { return m.terminalWidth }
func (m *mockDashboardState) TerminalHeight() int                { return m.terminalHeight }
func (m *mockDashboardState) IsAddingTask() bool                 { return m.isAddingTask }

func TestRenderSidebar_NoDuplicateTitle(t *testing.T) {
	tests := []struct {
		name  string
		state *mockDashboardState
	}{
		{
			name: "empty state - no instances, not adding task",
			state: &mockDashboardState{
				session:        &orchestrator.Session{Instances: []*orchestrator.Instance{}},
				terminalWidth:  80,
				terminalHeight: 24,
				isAddingTask:   false,
			},
		},
		{
			name: "empty state - nil session",
			state: &mockDashboardState{
				session:        nil,
				terminalWidth:  80,
				terminalHeight: 24,
				isAddingTask:   false,
			},
		},
		{
			name: "adding task mode - no instances",
			state: &mockDashboardState{
				session:        &orchestrator.Session{Instances: []*orchestrator.Instance{}},
				terminalWidth:  80,
				terminalHeight: 24,
				isAddingTask:   true,
			},
		},
		{
			name: "with instances",
			state: &mockDashboardState{
				session: &orchestrator.Session{
					Instances: []*orchestrator.Instance{
						{ID: "inst-1", Task: "Test task 1", Status: orchestrator.StatusWorking},
						{ID: "inst-2", Task: "Test task 2", Status: orchestrator.StatusPending},
					},
				},
				activeTab:      0,
				terminalWidth:  80,
				terminalHeight: 24,
				isAddingTask:   false,
			},
		},
		{
			name: "with instances - adding task",
			state: &mockDashboardState{
				session: &orchestrator.Session{
					Instances: []*orchestrator.Instance{
						{ID: "inst-1", Task: "Test task", Status: orchestrator.StatusWorking},
					},
				},
				activeTab:      0,
				terminalWidth:  80,
				terminalHeight: 24,
				isAddingTask:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dv := NewDashboardView()
			result := dv.RenderSidebar(tt.state, 30, 20)

			// Count occurrences of "Instances" in the rendered output
			// Note: We check for the title text, accounting for ANSI codes
			count := strings.Count(result, "Instances")
			if count != 1 {
				t.Errorf("expected 'Instances' to appear exactly once, got %d occurrences", count)
				t.Logf("rendered output:\n%s", result)
			}
		})
	}
}

func TestRenderSidebar_EmptyState(t *testing.T) {
	state := &mockDashboardState{
		session:        &orchestrator.Session{Instances: []*orchestrator.Instance{}},
		terminalWidth:  80,
		terminalHeight: 24,
		isAddingTask:   false,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// Should contain "No instances" message
	if !strings.Contains(result, "No instances") {
		t.Error("empty state should show 'No instances' message")
	}

	// Should contain add hint
	if !strings.Contains(result, "[a]") {
		t.Error("empty state should show '[a]' hint")
	}
}

func TestRenderSidebar_AddingTaskState(t *testing.T) {
	state := &mockDashboardState{
		session:        &orchestrator.Session{Instances: []*orchestrator.Instance{}},
		terminalWidth:  80,
		terminalHeight: 24,
		isAddingTask:   true,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// Should contain "New Task" when adding
	if !strings.Contains(result, "New Task") {
		t.Error("adding task state should show 'New Task' entry")
	}

	// Should NOT contain "No instances" when adding task
	if strings.Contains(result, "No instances") {
		t.Error("adding task state should not show 'No instances' message")
	}
}

func TestRenderSidebar_WithInstances(t *testing.T) {
	state := &mockDashboardState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-1", Task: "Implement feature X", Status: orchestrator.StatusWorking},
				{ID: "inst-2", Task: "Fix bug Y", Status: orchestrator.StatusCompleted},
			},
		},
		activeTab:      0,
		terminalWidth:  80,
		terminalHeight: 24,
		isAddingTask:   false,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// Should contain instance tasks (possibly truncated)
	if !strings.Contains(result, "Implement") {
		t.Error("should show first instance task")
	}
	if !strings.Contains(result, "Fix") {
		t.Error("should show second instance task")
	}

	// Should NOT contain "No instances"
	if strings.Contains(result, "No instances") {
		t.Error("should not show 'No instances' when instances exist")
	}
}

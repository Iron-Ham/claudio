package panel

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestInstancePanel_Render(t *testing.T) {
	tests := []struct {
		name     string
		state    *RenderState
		contains []string
		notEmpty bool
	}{
		{
			name: "renders instances with session",
			state: &RenderState{
				Width:       80,
				Height:      50,
				ActiveIndex: 0,
				Session: &orchestrator.Session{
					ID: "test-session",
					Instances: []*orchestrator.Instance{
						{
							ID:     "inst-1",
							Task:   "Build feature",
							Status: orchestrator.StatusWorking,
						},
						{
							ID:     "inst-2",
							Task:   "Fix bug",
							Status: orchestrator.StatusCompleted,
						},
					},
				},
			},
			contains: []string{
				"Instances",
				"Build feature",
				"Fix bug",
			},
			notEmpty: true,
		},
		{
			name: "shows no instances message",
			state: &RenderState{
				Width:   80,
				Height:  50,
				Session: &orchestrator.Session{},
			},
			contains: []string{
				"No instances",
			},
			notEmpty: true,
		},
		{
			name: "shows title without session",
			state: &RenderState{
				Width:   80,
				Height:  50,
				Session: nil,
			},
			contains: []string{
				"Instances",
			},
			notEmpty: true,
		},
		{
			name: "invalid state returns error indicator",
			state: &RenderState{
				Width:  0,
				Height: 0,
			},
			contains: []string{"render error"},
			notEmpty: true,
		},
	}

	panel := NewInstancePanel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := panel.Render(tt.state)

			if tt.notEmpty && result == "" {
				t.Error("expected non-empty result, got empty")
			}
			if !tt.notEmpty && result != "" {
				t.Errorf("expected empty result, got: %s", result)
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestInstancePanel_Height(t *testing.T) {
	panel := NewInstancePanel()

	state := &RenderState{
		Width:  80,
		Height: 50,
		Session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-1", Task: "Task 1"},
				{ID: "inst-2", Task: "Task 2"},
			},
		},
	}

	panel.Render(state)

	if panel.Height() <= 0 {
		t.Errorf("Height() = %d, want positive value", panel.Height())
	}
}

func TestInstancePanel_ScrollOffset(t *testing.T) {
	panel := NewInstancePanel()

	// Create many instances to test scrolling
	instances := make([]*orchestrator.Instance, 20)
	for i := range instances {
		instances[i] = &orchestrator.Instance{
			ID:      "inst-" + string(rune('a'+i)),
			Task:    "Task " + string(rune('A'+i)),
			Created: time.Now(),
		}
	}

	state := &RenderState{
		Width:        40,
		Height:       20,
		ScrollOffset: 5,
		Session: &orchestrator.Session{
			Instances: instances,
		},
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with scroll offset")
	}
}

func TestInstancePanel_IsAddingTask(t *testing.T) {
	panel := NewInstancePanel()

	state := &RenderState{
		Width:        80,
		Height:       50,
		IsAddingTask: true,
		Session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-1", Task: "Task 1"},
			},
		},
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result when adding task")
	}
}

func TestNewInstancePanel(t *testing.T) {
	panel := NewInstancePanel()
	if panel == nil {
		t.Fatal("NewInstancePanel() returned nil")
	}
	if panel.view == nil {
		t.Error("NewInstancePanel() should initialize view")
	}
}

func TestInstancePanelState_Interface(t *testing.T) {
	session := &orchestrator.Session{ID: "test"}

	state := &instancePanelState{
		session:             session,
		activeTab:           2,
		sidebarScrollOffset: 5,
		conflicts:           nil,
		terminalWidth:       80,
		terminalHeight:      24,
		isAddingTask:        true,
	}

	// Verify interface methods return expected values
	if state.Session() != session {
		t.Error("Session() returned wrong value")
	}
	if state.ActiveTab() != 2 {
		t.Errorf("ActiveTab() = %d, want 2", state.ActiveTab())
	}
	if state.SidebarScrollOffset() != 5 {
		t.Errorf("SidebarScrollOffset() = %d, want 5", state.SidebarScrollOffset())
	}
	if state.Conflicts() != nil {
		t.Error("Conflicts() should be nil")
	}
	if state.TerminalWidth() != 80 {
		t.Errorf("TerminalWidth() = %d, want 80", state.TerminalWidth())
	}
	if state.TerminalHeight() != 24 {
		t.Errorf("TerminalHeight() = %d, want 24", state.TerminalHeight())
	}
	if !state.IsAddingTask() {
		t.Error("IsAddingTask() = false, want true")
	}
}

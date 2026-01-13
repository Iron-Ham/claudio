package view

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// mockDashboardState implements DashboardState for testing
type mockDashboardState struct {
	session                  *orchestrator.Session
	activeTab                int
	sidebarScrollOffset      int
	conflicts                []conflict.FileConflict
	terminalWidth            int
	terminalHeight           int
	isAddingTask             bool
	intelligentNamingEnabled bool
}

func (m *mockDashboardState) Session() *orchestrator.Session     { return m.session }
func (m *mockDashboardState) ActiveTab() int                     { return m.activeTab }
func (m *mockDashboardState) SidebarScrollOffset() int           { return m.sidebarScrollOffset }
func (m *mockDashboardState) Conflicts() []conflict.FileConflict { return m.conflicts }
func (m *mockDashboardState) TerminalWidth() int                 { return m.terminalWidth }
func (m *mockDashboardState) TerminalHeight() int                { return m.terminalHeight }
func (m *mockDashboardState) IsAddingTask() bool                 { return m.isAddingTask }
func (m *mockDashboardState) IntelligentNamingEnabled() bool     { return m.intelligentNamingEnabled }

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

	// Should contain add hint (with colon prefix for command mode)
	if !strings.Contains(result, "[:a]") {
		t.Error("empty state should show '[:a]' hint")
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

func TestRenderSidebar_IntelligentNamingExpanded(t *testing.T) {
	longDisplayName := "Implement user authentication with OAuth2 and JWT tokens"
	state := &mockDashboardState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{
					ID:          "inst-1",
					Task:        "Short task",
					DisplayName: longDisplayName, // Long intelligent name
					Status:      orchestrator.StatusWorking,
				},
				{ID: "inst-2", Task: "Another task", Status: orchestrator.StatusPending},
			},
		},
		activeTab:                0,
		terminalWidth:            80,
		terminalHeight:           24,
		isAddingTask:             false,
		intelligentNamingEnabled: true,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// With intelligent naming enabled and selected, more of the name should be visible
	// The name should expand beyond the normal truncation point
	// Check that OAuth2 appears somewhere (it's past the normal truncation point)
	// Note: text may be split across multiple lines due to wrapping
	if !strings.Contains(result, "OAuth2") {
		t.Errorf("intelligent naming should expand selected instance name to show OAuth2, got:\n%s", result)
	}
}

func TestRenderSidebar_IntelligentNamingNotExpandedWhenNotSelected(t *testing.T) {
	longDisplayName := "Implement user authentication with OAuth2 and JWT tokens"
	state := &mockDashboardState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{
					ID:          "inst-1",
					Task:        "Short task",
					DisplayName: longDisplayName, // Long intelligent name
					Status:      orchestrator.StatusWorking,
				},
				{ID: "inst-2", Task: "Another task", Status: orchestrator.StatusPending},
			},
		},
		activeTab:                1, // Second instance is selected
		terminalWidth:            80,
		terminalHeight:           24,
		isAddingTask:             false,
		intelligentNamingEnabled: true,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// The first instance (not selected) should be truncated normally
	// It should contain the beginning but be truncated with ellipsis
	if strings.Contains(result, "OAuth2") {
		t.Errorf("non-selected instance should be truncated normally, got:\n%s", result)
	}
}

func TestRenderSidebar_IntelligentNamingDisabled(t *testing.T) {
	longDisplayName := "Implement user authentication with OAuth2 and JWT tokens"
	state := &mockDashboardState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{
					ID:          "inst-1",
					Task:        "Short task",
					DisplayName: longDisplayName,
					Status:      orchestrator.StatusWorking,
				},
			},
		},
		activeTab:                0,
		terminalWidth:            80,
		terminalHeight:           24,
		isAddingTask:             false,
		intelligentNamingEnabled: false, // Disabled
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// Even when selected, should be truncated if intelligent naming is disabled
	if strings.Contains(result, "OAuth2") {
		t.Errorf("with intelligent naming disabled, should truncate normally, got:\n%s", result)
	}
}

func TestRenderSidebar_IntelligentNamingMaxLength(t *testing.T) {
	// Create a name longer than ExpandedNameMaxLen (50 chars)
	veryLongName := "This is a very long instance name that exceeds the maximum allowed length for expanded names in the sidebar"
	state := &mockDashboardState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{
					ID:          "inst-1",
					Task:        "Short task",
					DisplayName: veryLongName,
					Status:      orchestrator.StatusWorking,
				},
			},
		},
		activeTab:                0,
		terminalWidth:            80,
		terminalHeight:           24,
		isAddingTask:             false,
		intelligentNamingEnabled: true,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// Should be capped at ExpandedNameMaxLen (50 chars) with ellipsis
	// Should not contain text past the max length
	if strings.Contains(result, "sidebar") {
		t.Errorf("should cap expanded name at maximum length, got:\n%s", result)
	}

	// Should still contain the beginning of the name
	if !strings.Contains(result, "This is a very long") {
		t.Errorf("should contain beginning of expanded name, got:\n%s", result)
	}
}

func TestWrapAtWordBoundary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string fits entirely",
			input:    "hello world",
			maxLen:   20,
			expected: "hello world",
		},
		{
			name:     "breaks at word boundary",
			input:    "hello world foo",
			maxLen:   12,
			expected: "hello world",
		},
		{
			name:     "breaks at word boundary mid-sentence",
			input:    "We should create a new GitHub Issue",
			maxLen:   20,
			expected: "We should create a",
		},
		{
			name:     "avoids breaking mid-word",
			input:    "GitHub Issue details",
			maxLen:   8,
			expected: "GitHub",
		},
		{
			name:     "falls back to char break when word is too long",
			input:    "Supercalifragilisticexpialidocious is a word",
			maxLen:   10,
			expected: "Supercalif", // No space found early enough
		},
		{
			name:     "respects 1/3 minimum threshold",
			input:    "I want to create something",
			maxLen:   15,
			expected: "I want to", // Space at position 9 is > 15/3=5
		},
		{
			name:     "space at very beginning falls back to char break",
			input:    "X verylongword here",
			maxLen:   10,
			expected: "X verylong", // Space at position 1 is NOT > 10/3=3, so falls back to char break
		},
		{
			name:     "unicode characters handled correctly",
			input:    "こんにちは 世界です",
			maxLen:   6,
			expected: "こんにちは",
		},
		{
			name:     "exact fit with trailing space",
			input:    "hello ",
			maxLen:   6,
			expected: "hello ",
		},
		{
			name:     "multiple spaces in text",
			input:    "one two three four",
			maxLen:   14,
			expected: "one two three",
		},
		{
			name:     "empty string returns empty",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "maxLen zero returns empty",
			input:    "hello world",
			maxLen:   0,
			expected: "",
		},
		{
			name:     "negative maxLen returns empty",
			input:    "hello world",
			maxLen:   -5,
			expected: "",
		},
		{
			name:     "maxLen of 1",
			input:    "hello world",
			maxLen:   1,
			expected: "h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapAtWordBoundary([]rune(tt.input), tt.maxLen)
			if result != tt.expected {
				t.Errorf("wrapAtWordBoundary(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestRenderSidebar_WordBoundaryWrapping(t *testing.T) {
	// Test case matching the user's screenshot issue
	longDisplayName := "We should create a new GitHub Issue detailing this"
	state := &mockDashboardState{
		session: &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{
					ID:          "inst-1",
					Task:        "Short task",
					DisplayName: longDisplayName,
					Status:      orchestrator.StatusWorking,
				},
			},
		},
		activeTab:                0,
		terminalWidth:            80,
		terminalHeight:           24,
		isAddingTask:             false,
		intelligentNamingEnabled: true,
	}

	dv := NewDashboardView()
	result := dv.RenderSidebar(state, 30, 20)

	// The word "GitHub" should appear on a single line (not split across lines)
	// Check that at least one line contains the complete word
	lines := strings.Split(result, "\n")
	foundGitHub := false
	for _, line := range lines {
		if strings.Contains(line, "GitHub") {
			foundGitHub = true
			break
		}
	}
	if !foundGitHub {
		t.Errorf("'GitHub' should appear as a complete word on a single line, got:\n%s", result)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncated with ellipsis",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
		{
			name:     "very short max returns input",
			input:    "hello",
			maxLen:   3,
			expected: "hello",
		},
		{
			name:     "unicode string truncation",
			input:    "こんにちは世界",
			maxLen:   5,
			expected: "こん...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

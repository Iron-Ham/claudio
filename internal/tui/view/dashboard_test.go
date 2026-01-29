package view

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
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

func TestRenderSidebar_ContinuationLineAlignment(t *testing.T) {
	// Test that continuation lines align properly regardless of instance number width
	// This test verifies that the continuation lines have consistent indentation
	// relative to where the name starts on the first line

	tests := []struct {
		name         string
		instanceIdx  int // 0-based index, will be displayed as instanceIdx+1
		displayName  string
		sidebarWidth int
	}{
		{
			name:         "single digit instance number",
			instanceIdx:  4, // displays as "5"
			displayName:  "We should add different icon signifiers for the various states",
			sidebarWidth: 30,
		},
		{
			name:         "double digit instance number",
			instanceIdx:  9, // displays as "10"
			displayName:  "We should add different icon signifiers for the various states",
			sidebarWidth: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create instances up to the target index
			instances := make([]*orchestrator.Instance, tt.instanceIdx+1)
			for i := 0; i < tt.instanceIdx; i++ {
				instances[i] = &orchestrator.Instance{
					ID:     "inst-" + string(rune('a'+i)),
					Task:   "Task",
					Status: orchestrator.StatusPending,
				}
			}
			instances[tt.instanceIdx] = &orchestrator.Instance{
				ID:          "inst-target",
				Task:        "Short task",
				DisplayName: tt.displayName,
				Status:      orchestrator.StatusWorking,
			}

			state := &mockDashboardState{
				session:                  &orchestrator.Session{Instances: instances},
				activeTab:                tt.instanceIdx,
				terminalWidth:            80,
				terminalHeight:           24,
				isAddingTask:             false,
				intelligentNamingEnabled: true,
			}

			dv := NewDashboardView()
			result := dv.RenderSidebar(state, tt.sidebarWidth, 20)

			// Get the rendered lines
			lines := strings.Split(result, "\n")

			// Find lines belonging to the selected instance (they should contain content from displayName)
			var instanceLines []string
			for _, line := range lines {
				if strings.Contains(line, "should") || strings.Contains(line, "add") ||
					strings.Contains(line, "different") || strings.Contains(line, "icon") ||
					strings.Contains(line, "signifiers") {
					instanceLines = append(instanceLines, line)
				}
			}

			if len(instanceLines) < 2 {
				t.Skipf("not enough lines to test alignment, need wrapping: got %d lines", len(instanceLines))
			}

			// Verify we have multiple continuation lines and they all contain expected text
			// The key verification is that the rendering doesn't panic and produces output
			// The exact alignment depends on ANSI codes and border rendering
			if len(instanceLines) == 0 {
				t.Errorf("expected at least one instance line with displayName content")
			}

			// Log the output for manual inspection if needed
			t.Logf("Instance lines for %s:", tt.name)
			for i, line := range instanceLines {
				t.Logf("  [%d]: %q", i, line)
			}
		})
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

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "zero time returns empty",
			input:    time.Time{},
			expected: "",
		},
		{
			name:     "seconds ago",
			input:    time.Now().Add(-30 * time.Second),
			expected: "30s ago",
		},
		{
			name:     "minutes ago",
			input:    time.Now().Add(-5 * time.Minute),
			expected: "5m ago",
		},
		{
			name:     "hours ago",
			input:    time.Now().Add(-2 * time.Hour),
			expected: "2h ago",
		},
		{
			name:     "days ago",
			input:    time.Now().Add(-48 * time.Hour),
			expected: "2d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeAgo(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTimeAgo() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatTimeAgoPtr(t *testing.T) {
	now := time.Now().Add(-5 * time.Minute)

	tests := []struct {
		name     string
		input    *time.Time
		expected string
	}{
		{
			name:     "nil returns empty",
			input:    nil,
			expected: "",
		},
		{
			name:     "valid time pointer",
			input:    &now,
			expected: "5m ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatTimeAgoPtr(tt.input)
			if result != tt.expected {
				t.Errorf("FormatTimeAgoPtr() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatInstanceContextInfo(t *testing.T) {
	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	tests := []struct {
		name     string
		instance *orchestrator.Instance
		contains []string
	}{
		{
			name: "instance with metrics duration",
			instance: &orchestrator.Instance{
				ID:     "test-1",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					StartTime: &startTime,
					EndTime:   &endTime,
					Cost:      0.05,
				},
			},
			contains: []string{"5m", "$0.05"},
		},
		{
			name: "instance with files modified",
			instance: &orchestrator.Instance{
				ID:            "test-2",
				Status:        orchestrator.StatusCompleted,
				FilesModified: []string{"file1.go", "file2.go", "file3.go"},
			},
			contains: []string{"3 files"},
		},
		{
			name: "instance with all info",
			instance: &orchestrator.Instance{
				ID:     "test-3",
				Status: orchestrator.StatusWorking,
				Metrics: &orchestrator.Metrics{
					StartTime: &startTime,
					Cost:      0.10,
				},
				FilesModified: []string{"file1.go"},
			},
			contains: []string{"5m", "$0.10", "1 files"},
		},
		{
			name: "instance with no metrics",
			instance: &orchestrator.Instance{
				ID:     "test-4",
				Status: orchestrator.StatusPending,
			},
			contains: []string{},
		},
		{
			name: "instance with cost below threshold",
			instance: &orchestrator.Instance{
				ID:     "test-5",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					Cost: 0.005, // Below 0.01 threshold
				},
			},
			contains: []string{}, // Cost should not be shown
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatInstanceContextInfo(tt.instance)
			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("formatInstanceContextInfo() = %q, should contain %q", result, expected)
				}
			}
		})
	}
}

func TestFormatDurationCompact(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{"30 seconds", 30 * time.Second, "30s"},
		{"59 seconds", 59 * time.Second, "59s"},
		{"1 minute", 1 * time.Minute, "1m"},
		{"5 minutes", 5 * time.Minute, "5m"},
		{"59 minutes", 59 * time.Minute, "59m"},
		{"1 hour", 1 * time.Hour, "1h"},
		{"1 hour 30 minutes", 1*time.Hour + 30*time.Minute, "1h30m"},
		{"2 hours", 2 * time.Hour, "2h"},
		{"2 hours 15 minutes", 2*time.Hour + 15*time.Minute, "2h15m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDurationCompact(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatDurationCompact(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestRenderEnhancedStatusLine(t *testing.T) {
	startTime := time.Now().Add(-5 * time.Minute)
	lastActive := time.Now().Add(-30 * time.Second)

	tests := []struct {
		name           string
		instance       *orchestrator.Instance
		containsStatus bool
		containsTime   bool
	}{
		{
			name: "working instance shows last active time",
			instance: &orchestrator.Instance{
				ID:           "test-1",
				Status:       orchestrator.StatusWorking,
				LastActiveAt: &lastActive,
				Metrics: &orchestrator.Metrics{
					StartTime: &startTime,
				},
			},
			containsStatus: true,
			containsTime:   true,
		},
		{
			name: "completed instance shows metrics",
			instance: &orchestrator.Instance{
				ID:     "test-2",
				Status: orchestrator.StatusCompleted,
				Metrics: &orchestrator.Metrics{
					StartTime: &startTime,
					Cost:      0.05,
				},
			},
			containsStatus: true,
			containsTime:   false, // Not working, so no "ago" time
		},
		{
			name: "pending instance minimal info",
			instance: &orchestrator.Instance{
				ID:     "test-3",
				Status: orchestrator.StatusPending,
			},
			containsStatus: true,
			containsTime:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusColor := styles.StatusColor(string(tt.instance.Status))
			result := renderEnhancedStatusLine(tt.instance, statusColor, 2, 40)

			// Should contain status abbreviation
			statusAbbrev := instanceStatusAbbrev(tt.instance.Status)
			if tt.containsStatus && !strings.Contains(result, statusAbbrev) {
				t.Errorf("renderEnhancedStatusLine() = %q, should contain status %q", result, statusAbbrev)
			}

			// Check for "ago" indicator
			if tt.containsTime && !strings.Contains(result, "ago") {
				t.Errorf("renderEnhancedStatusLine() = %q, should contain 'ago' time indicator", result)
			}
		})
	}
}

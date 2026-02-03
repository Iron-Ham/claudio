package output

import (
	"strings"
	"sync"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.outputs == nil {
		t.Error("outputs map not initialized")
	}
	if m.scrollOffsets == nil {
		t.Error("scrollOffsets map not initialized")
	}
	if m.autoScroll == nil {
		t.Error("autoScroll map not initialized")
	}
	if m.hasNewOutput == nil {
		t.Error("hasNewOutput map not initialized")
	}
}

func TestAddOutput(t *testing.T) {
	tests := []struct {
		name     string
		adds     []string
		expected string
	}{
		{
			name:     "single add",
			adds:     []string{"hello"},
			expected: "hello",
		},
		{
			name:     "multiple adds",
			adds:     []string{"hello", " ", "world"},
			expected: "hello world",
		},
		{
			name:     "add with newlines",
			adds:     []string{"line1\n", "line2\n", "line3"},
			expected: "line1\nline2\nline3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			for _, add := range tt.adds {
				m.AddOutput("test", add)
			}
			got := m.GetOutput("test")
			if got != tt.expected {
				t.Errorf("GetOutput() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSetOutput(t *testing.T) {
	m := NewManager()

	// First set
	m.SetOutput("inst1", "initial content")
	if got := m.GetOutput("inst1"); got != "initial content" {
		t.Errorf("GetOutput() = %q, want %q", got, "initial content")
	}

	// Override
	m.SetOutput("inst1", "replaced content")
	if got := m.GetOutput("inst1"); got != "replaced content" {
		t.Errorf("GetOutput() after replace = %q, want %q", got, "replaced content")
	}
}

func TestGetOutput(t *testing.T) {
	m := NewManager()

	// Non-existent instance returns empty string
	if got := m.GetOutput("nonexistent"); got != "" {
		t.Errorf("GetOutput(nonexistent) = %q, want empty string", got)
	}

	// After setting
	m.SetOutput("inst1", "content")
	if got := m.GetOutput("inst1"); got != "content" {
		t.Errorf("GetOutput(inst1) = %q, want %q", got, "content")
	}
}

func TestClear(t *testing.T) {
	m := NewManager()

	// Set up some state
	m.SetOutput("inst1", "content")
	m.Scroll("inst1", 5, 10)
	m.SetAutoScroll("inst1", false)

	// Clear the instance
	m.Clear("inst1")

	// Verify all state is cleared
	if got := m.GetOutput("inst1"); got != "" {
		t.Errorf("GetOutput() after Clear = %q, want empty", got)
	}
	if got := m.GetScrollOffset("inst1"); got != 0 {
		t.Errorf("GetScrollOffset() after Clear = %d, want 0", got)
	}
	// Auto-scroll defaults to true for new/cleared instances
	if got := m.IsAutoScroll("inst1"); !got {
		t.Error("IsAutoScroll() after Clear should default to true")
	}
}

func TestClearAll(t *testing.T) {
	m := NewManager()

	// Set up multiple instances
	m.SetOutput("inst1", "content1")
	m.SetOutput("inst2", "content2")
	m.SetOutput("inst3", "content3")

	// Clear all
	m.ClearAll()

	// Verify all cleared
	if got := m.GetOutput("inst1"); got != "" {
		t.Errorf("inst1 not cleared: %q", got)
	}
	if got := m.GetOutput("inst2"); got != "" {
		t.Errorf("inst2 not cleared: %q", got)
	}
	if got := m.GetOutput("inst3"); got != "" {
		t.Errorf("inst3 not cleared: %q", got)
	}
}

func TestScroll(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		maxVisibleLines int
		delta           int
		expectedOffset  int
		expectedAuto    bool
	}{
		{
			name:            "scroll down within bounds",
			output:          "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
			maxVisibleLines: 5,
			delta:           3,
			expectedOffset:  3,
			expectedAuto:    true, // scrolling down doesn't disable auto-scroll (only scrolling up does)
		},
		{
			name:            "scroll down to bottom",
			output:          "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
			maxVisibleLines: 5,
			delta:           10,
			expectedOffset:  5, // max scroll = 10-5 = 5
			expectedAuto:    true,
		},
		{
			name:            "scroll up disables auto-scroll",
			output:          "line1\nline2\nline3\nline4\nline5",
			maxVisibleLines: 3,
			delta:           -1,
			expectedOffset:  0, // clamped to 0
			expectedAuto:    false,
		},
		{
			name:            "scroll beyond bounds clamps to max",
			output:          "line1\nline2\nline3",
			maxVisibleLines: 5,
			delta:           100,
			expectedOffset:  0, // content fits in view
			expectedAuto:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetOutput("test", tt.output)

			m.Scroll("test", tt.delta, tt.maxVisibleLines)

			if got := m.GetScrollOffset("test"); got != tt.expectedOffset {
				t.Errorf("GetScrollOffset() = %d, want %d", got, tt.expectedOffset)
			}
			if got := m.IsAutoScroll("test"); got != tt.expectedAuto {
				t.Errorf("IsAutoScroll() = %v, want %v", got, tt.expectedAuto)
			}
		})
	}
}

func TestScrollToTop(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2\nline3\nline4\nline5")
	m.Scroll("test", 3, 2) // Scroll down first

	m.ScrollToTop("test")

	if got := m.GetScrollOffset("test"); got != 0 {
		t.Errorf("GetScrollOffset() = %d, want 0", got)
	}
	if got := m.IsAutoScroll("test"); got {
		t.Error("IsAutoScroll() should be false after ScrollToTop")
	}
}

func TestScrollToBottom(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2\nline3\nline4\nline5")
	m.SetAutoScroll("test", false) // Disable auto-scroll first

	m.ScrollToBottom("test", 2)

	expectedMax := 3 // 5 lines - 2 visible = 3
	if got := m.GetScrollOffset("test"); got != expectedMax {
		t.Errorf("GetScrollOffset() = %d, want %d", got, expectedMax)
	}
	if got := m.IsAutoScroll("test"); !got {
		t.Error("IsAutoScroll() should be true after ScrollToBottom")
	}
}

func TestIsAutoScroll(t *testing.T) {
	m := NewManager()

	// Default is true for new instance
	if got := m.IsAutoScroll("new"); !got {
		t.Error("IsAutoScroll() should default to true")
	}

	// Explicitly set
	m.SetAutoScroll("new", false)
	if got := m.IsAutoScroll("new"); got {
		t.Error("IsAutoScroll() should be false after SetAutoScroll(false)")
	}

	m.SetAutoScroll("new", true)
	if got := m.IsAutoScroll("new"); !got {
		t.Error("IsAutoScroll() should be true after SetAutoScroll(true)")
	}
}

func TestUpdateScroll(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2\nline3\nline4\nline5")

	// With auto-scroll enabled (default), should scroll to bottom
	m.UpdateScroll("test", 2)

	expectedMax := 3 // 5 lines - 2 visible = 3
	if got := m.GetScrollOffset("test"); got != expectedMax {
		t.Errorf("GetScrollOffset() = %d, want %d", got, expectedMax)
	}

	// Disable auto-scroll and update - should not change position
	m.SetAutoScroll("test", false)
	m.SetOutput("test", "line1\nline2\nline3\nline4\nline5\nline6") // Add more content
	m.UpdateScroll("test", 2)

	if got := m.GetScrollOffset("test"); got != expectedMax {
		t.Errorf("GetScrollOffset() should not change when auto-scroll disabled, got %d want %d", got, expectedMax)
	}
}

func TestHasNewOutput(t *testing.T) {
	m := NewManager()

	m.SetOutput("test", "line1\nline2\nline3")

	// Scrolling up disables auto-scroll
	m.Scroll("test", -1, 1)
	if got := m.IsAutoScroll("test"); got {
		t.Error("IsAutoScroll() should be false after scrolling up")
	}

	// Add output while scrolled up
	m.AddOutput("test", "\nline4")
	if got := m.HasNewOutput("test"); !got {
		t.Error("HasNewOutput() should be true after adding output while auto-scroll disabled")
	}

	// Jump back to bottom clears new output indicator
	m.ScrollToBottom("test", 1)
	if got := m.HasNewOutput("test"); got {
		t.Error("HasNewOutput() should be false after ScrollToBottom")
	}
}

func TestHasNewOutput_AutoScrollEnabled(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2")

	// Auto-scroll defaults to true, so new output shouldn't trigger the indicator.
	m.AddOutput("test", "\nline3")
	if got := m.HasNewOutput("test"); got {
		t.Error("HasNewOutput() should be false when auto-scroll is enabled")
	}
}

func TestGetLineCount(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "empty output",
			output:   "",
			expected: 0,
		},
		{
			name:     "single line no newline",
			output:   "hello",
			expected: 1,
		},
		{
			name:     "single line with trailing newline",
			output:   "hello\n",
			expected: 2,
		},
		{
			name:     "multiple lines",
			output:   "line1\nline2\nline3",
			expected: 3,
		},
		{
			name:     "multiple lines with trailing newline",
			output:   "line1\nline2\nline3\n",
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetOutput("test", tt.output)
			if got := m.GetLineCount("test"); got != tt.expected {
				t.Errorf("GetLineCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestGetLineCountWithFilter(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "keep\nremove\nkeep\nremove\nkeep")

	// Without filter
	if got := m.GetLineCount("test"); got != 5 {
		t.Errorf("GetLineCount() without filter = %d, want 5", got)
	}

	// With filter that removes lines containing "remove"
	m.SetFilterFunc(func(output string) string {
		lines := strings.Split(output, "\n")
		var filtered []string
		for _, line := range lines {
			if !strings.Contains(line, "remove") {
				filtered = append(filtered, line)
			}
		}
		return strings.Join(filtered, "\n")
	})

	if got := m.GetLineCount("test"); got != 3 {
		t.Errorf("GetLineCount() with filter = %d, want 3", got)
	}
}

func TestGetMaxScroll(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		maxVisibleLines int
		expected        int
	}{
		{
			name:            "content fits in view",
			output:          "line1\nline2",
			maxVisibleLines: 5,
			expected:        0,
		},
		{
			name:            "content larger than view",
			output:          "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10",
			maxVisibleLines: 5,
			expected:        5, // 10 - 5 = 5
		},
		{
			name:            "exact fit",
			output:          "line1\nline2\nline3\nline4\nline5",
			maxVisibleLines: 5,
			expected:        0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			m.SetOutput("test", tt.output)
			if got := m.GetMaxScroll("test", tt.maxVisibleLines); got != tt.expected {
				t.Errorf("GetMaxScroll() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestInstanceIDs(t *testing.T) {
	m := NewManager()
	m.SetOutput("inst1", "content1")
	m.SetOutput("inst2", "content2")
	m.SetOutput("inst3", "content3")

	ids := m.InstanceIDs()
	if len(ids) != 3 {
		t.Errorf("InstanceIDs() returned %d ids, want 3", len(ids))
	}

	// Check all IDs are present (order not guaranteed)
	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for _, expected := range []string{"inst1", "inst2", "inst3"} {
		if !idMap[expected] {
			t.Errorf("InstanceIDs() missing %s", expected)
		}
	}
}

func TestHasOutput(t *testing.T) {
	m := NewManager()

	if m.HasOutput("test") {
		t.Error("HasOutput() should be false for non-existent instance")
	}

	m.SetOutput("test", "content")
	if !m.HasOutput("test") {
		t.Error("HasOutput() should be true after SetOutput")
	}

	m.Clear("test")
	if m.HasOutput("test") {
		t.Error("HasOutput() should be false after Clear")
	}
}

func TestGetFilteredOutput(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nremove\nline3")

	// Without filter
	if got := m.GetFilteredOutput("test"); got != "line1\nremove\nline3" {
		t.Errorf("GetFilteredOutput() without filter = %q, want original", got)
	}

	// With filter
	m.SetFilterFunc(func(output string) string {
		lines := strings.Split(output, "\n")
		var filtered []string
		for _, line := range lines {
			if line != "remove" {
				filtered = append(filtered, line)
			}
		}
		return strings.Join(filtered, "\n")
	})

	if got := m.GetFilteredOutput("test"); got != "line1\nline3" {
		t.Errorf("GetFilteredOutput() with filter = %q, want %q", got, "line1\nline3")
	}
}

func TestGetFilteredOutputCaching(t *testing.T) {
	m := NewManager()

	// Track how many times the filter function is called
	filterCallCount := 0
	filterFunc := func(output string) string {
		filterCallCount++
		lines := strings.Split(output, "\n")
		var filtered []string
		for _, line := range lines {
			if line != "remove" {
				filtered = append(filtered, line)
			}
		}
		return strings.Join(filtered, "\n")
	}

	m.SetFilterFunc(filterFunc)
	m.SetOutput("test", "line1\nremove\nline3")

	// First call should invoke filter
	filterCallCount = 0
	result1 := m.GetFilteredOutput("test")
	if filterCallCount != 1 {
		t.Errorf("First GetFilteredOutput() should call filter once, got %d calls", filterCallCount)
	}
	if result1 != "line1\nline3" {
		t.Errorf("GetFilteredOutput() = %q, want %q", result1, "line1\nline3")
	}

	// Second call with same output should use cache (no filter call)
	filterCallCount = 0
	result2 := m.GetFilteredOutput("test")
	if filterCallCount != 0 {
		t.Errorf("Second GetFilteredOutput() should use cache, but got %d filter calls", filterCallCount)
	}
	if result2 != result1 {
		t.Errorf("Cached result differs: %q vs %q", result2, result1)
	}

	// Changing output should invalidate cache
	filterCallCount = 0
	m.SetOutput("test", "line1\nremove\nline3\nline4")
	result3 := m.GetFilteredOutput("test")
	if filterCallCount != 1 {
		t.Errorf("GetFilteredOutput() after SetOutput should call filter, got %d calls", filterCallCount)
	}
	if result3 != "line1\nline3\nline4" {
		t.Errorf("GetFilteredOutput() after change = %q, want %q", result3, "line1\nline3\nline4")
	}

	// InvalidateFilterCache should force recomputation
	filterCallCount = 0
	m.InvalidateFilterCache()
	result4 := m.GetFilteredOutput("test")
	if filterCallCount != 1 {
		t.Errorf("GetFilteredOutput() after InvalidateFilterCache should call filter, got %d calls", filterCallCount)
	}
	if result4 != result3 {
		t.Errorf("Result after InvalidateFilterCache differs: %q vs %q", result4, result3)
	}
}

func TestGetVisibleLines(t *testing.T) {
	tests := []struct {
		name            string
		output          string
		scrollOffset    int
		maxVisibleLines int
		expected        []string
	}{
		{
			name:            "no output",
			output:          "",
			scrollOffset:    0,
			maxVisibleLines: 3,
			expected:        nil,
		},
		{
			name:            "all visible",
			output:          "line1\nline2\nline3",
			scrollOffset:    0,
			maxVisibleLines: 5,
			expected:        []string{"line1", "line2", "line3"},
		},
		{
			name:            "scrolled view",
			output:          "line1\nline2\nline3\nline4\nline5",
			scrollOffset:    2,
			maxVisibleLines: 2,
			expected:        []string{"line3", "line4"},
		},
		{
			name:            "scroll clamped at end",
			output:          "line1\nline2\nline3",
			scrollOffset:    10, // beyond end
			maxVisibleLines: 2,
			expected:        []string{"line3"}, // clamped to last line
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			if tt.output != "" {
				m.SetOutput("test", tt.output)
			}
			// Set scroll offset directly
			m.mu.Lock()
			m.scrollOffsets["test"] = tt.scrollOffset
			m.mu.Unlock()

			got := m.GetVisibleLines("test", tt.maxVisibleLines)

			if tt.expected == nil {
				if got != nil {
					t.Errorf("GetVisibleLines() = %v, want nil", got)
				}
				return
			}

			if len(got) != len(tt.expected) {
				t.Errorf("GetVisibleLines() returned %d lines, want %d", len(got), len(tt.expected))
				return
			}

			for i, line := range got {
				if line != tt.expected[i] {
					t.Errorf("GetVisibleLines()[%d] = %q, want %q", i, line, tt.expected[i])
				}
			}
		})
	}
}

func TestGetVisibleLinesWithFilter(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "keep1\nremove\nkeep2\nremove\nkeep3")
	m.SetFilterFunc(func(output string) string {
		lines := strings.Split(output, "\n")
		var filtered []string
		for _, line := range lines {
			if !strings.Contains(line, "remove") {
				filtered = append(filtered, line)
			}
		}
		return strings.Join(filtered, "\n")
	})

	got := m.GetVisibleLines("test", 2)
	expected := []string{"keep1", "keep2"}

	if len(got) != len(expected) {
		t.Errorf("GetVisibleLines() returned %d lines, want %d", len(got), len(expected))
		return
	}

	for i, line := range got {
		if line != expected[i] {
			t.Errorf("GetVisibleLines()[%d] = %q, want %q", i, line, expected[i])
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()
	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 4)

	// Writers
	for range numGoroutines {
		go func() {
			defer wg.Done()
			instanceID := "inst"
			for range numOperations {
				m.AddOutput(instanceID, "data\n")
			}
		}()
	}

	// Readers
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numOperations {
				_ = m.GetOutput("inst")
			}
		}()
	}

	// Scrollers
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numOperations {
				m.Scroll("inst", 1, 10)
			}
		}()
	}

	// State readers
	for range numGoroutines {
		go func() {
			defer wg.Done()
			for range numOperations {
				_ = m.GetScrollOffset("inst")
				_ = m.IsAutoScroll("inst")
				_ = m.HasNewOutput("inst")
			}
		}()
	}

	wg.Wait()
}

func TestSetFilterFunc(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2\nline3")

	// Verify initial state
	if m.filterFunc != nil {
		t.Error("filterFunc should be nil initially")
	}

	// Set filter
	filterCalled := false
	m.SetFilterFunc(func(output string) string {
		filterCalled = true
		return output
	})

	// Call a method that uses the filter
	m.GetLineCount("test")

	if !filterCalled {
		t.Error("filterFunc was not called")
	}
}

func TestScrollNegativeDeltaFromZero(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2\nline3")

	// Start at 0, scroll up (negative)
	m.Scroll("test", -5, 2)

	if got := m.GetScrollOffset("test"); got != 0 {
		t.Errorf("GetScrollOffset() = %d, want 0 (clamped)", got)
	}
}

func TestMultipleInstancesIndependent(t *testing.T) {
	m := NewManager()

	// Set up two instances
	m.SetOutput("inst1", "a\nb\nc\nd\ne")
	m.SetOutput("inst2", "1\n2\n3\n4\n5")

	// Scroll only inst1
	m.Scroll("inst1", 2, 2)
	m.SetAutoScroll("inst1", false)

	// Verify inst1 changed
	if got := m.GetScrollOffset("inst1"); got != 2 {
		t.Errorf("inst1 ScrollOffset = %d, want 2", got)
	}
	if got := m.IsAutoScroll("inst1"); got {
		t.Error("inst1 AutoScroll should be false")
	}

	// Verify inst2 unchanged
	if got := m.GetScrollOffset("inst2"); got != 0 {
		t.Errorf("inst2 ScrollOffset = %d, want 0", got)
	}
	if got := m.IsAutoScroll("inst2"); !got {
		t.Error("inst2 AutoScroll should be true (default)")
	}
}

func TestGetAllOutputs(t *testing.T) {
	m := NewManager()

	// Empty manager returns empty map
	result := m.GetAllOutputs()
	if len(result) != 0 {
		t.Errorf("GetAllOutputs() on empty manager returned %d entries, want 0", len(result))
	}

	// Add some outputs
	m.SetOutput("inst1", "output1")
	m.SetOutput("inst2", "output2")
	m.SetOutput("inst3", "output3")

	result = m.GetAllOutputs()
	if len(result) != 3 {
		t.Errorf("GetAllOutputs() returned %d entries, want 3", len(result))
	}

	// Verify values
	expected := map[string]string{
		"inst1": "output1",
		"inst2": "output2",
		"inst3": "output3",
	}
	for id, expectedOutput := range expected {
		if got, ok := result[id]; !ok || got != expectedOutput {
			t.Errorf("GetAllOutputs()[%s] = %q, want %q", id, got, expectedOutput)
		}
	}

	// Verify it's a copy (modifying result doesn't affect manager)
	result["inst1"] = "modified"
	if got := m.GetOutput("inst1"); got != "output1" {
		t.Errorf("GetOutput() after modifying returned map = %q, want original %q", got, "output1")
	}
}

func TestEmptyOutputHandling(t *testing.T) {
	m := NewManager()

	// Operations on empty/non-existent output should not panic
	m.Scroll("empty", 5, 10)
	m.UpdateScroll("empty", 10)
	m.ScrollToTop("empty")
	m.ScrollToBottom("empty", 10)

	// Should return sane defaults
	if got := m.GetScrollOffset("empty"); got != 0 {
		t.Errorf("GetScrollOffset(empty) = %d, want 0", got)
	}
	if got := m.GetLineCount("empty"); got != 0 {
		t.Errorf("GetLineCount(empty) = %d, want 0", got)
	}
	if got := m.GetMaxScroll("empty", 10); got != 0 {
		t.Errorf("GetMaxScroll(empty) = %d, want 0", got)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"single char", "a", 1},
		{"single line no newline", "hello world", 1},
		{"single line with newline", "hello\n", 2},
		{"two lines", "a\nb", 2},
		{"three lines", "a\nb\nc", 3},
		{"trailing newline", "a\nb\nc\n", 4},
		{"only newlines", "\n\n\n", 4},
		{"unicode", "こんにちは\n世界", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines(tt.input)
			if got != tt.expected {
				t.Errorf("countLines(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"empty string", "", nil},
		{"single line", "hello", []string{"hello"}},
		{"two lines", "a\nb", []string{"a", "b"}},
		{"trailing newline", "a\nb\n", []string{"a", "b", ""}},
		{"only newline", "\n", []string{"", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Errorf("splitLines(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Errorf("splitLines(%q) returned %d lines, want %d", tt.input, len(got), len(tt.expected))
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestGetFilteredLines(t *testing.T) {
	m := NewManager()

	// Empty output returns nil
	if got := m.GetFilteredLines("nonexistent"); got != nil {
		t.Errorf("GetFilteredLines(nonexistent) = %v, want nil", got)
	}

	// Set output and get lines
	m.SetOutput("test", "line1\nline2\nline3")
	got := m.GetFilteredLines("test")
	expected := []string{"line1", "line2", "line3"}

	if len(got) != len(expected) {
		t.Fatalf("GetFilteredLines() returned %d lines, want %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("GetFilteredLines()[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestGetFilteredLinesWithFilter(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "keep1\nremove\nkeep2\nremove\nkeep3")
	m.SetFilterFunc(func(output string) string {
		lines := strings.Split(output, "\n")
		var filtered []string
		for _, line := range lines {
			if !strings.Contains(line, "remove") {
				filtered = append(filtered, line)
			}
		}
		return strings.Join(filtered, "\n")
	})

	got := m.GetFilteredLines("test")
	expected := []string{"keep1", "keep2", "keep3"}

	if len(got) != len(expected) {
		t.Fatalf("GetFilteredLines() with filter returned %d lines, want %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("GetFilteredLines()[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestGetFilteredLinesCaching(t *testing.T) {
	m := NewManager()
	m.SetOutput("test", "line1\nline2\nline3")

	// First call
	lines1 := m.GetFilteredLines("test")
	// Second call should return cached slice
	lines2 := m.GetFilteredLines("test")

	// Should be the same underlying slice (pointer equality)
	if &lines1[0] != &lines2[0] {
		t.Error("GetFilteredLines() should return cached slice on second call")
	}

	// After changing output, should return new slice
	m.SetOutput("test", "new1\nnew2")
	lines3 := m.GetFilteredLines("test")

	if len(lines3) != 2 || lines3[0] != "new1" {
		t.Errorf("GetFilteredLines() after SetOutput = %v, want [new1 new2]", lines3)
	}
}

// Benchmarks for performance testing

func BenchmarkGetFilteredLines(b *testing.B) {
	m := NewManager()
	m.SetOutput("test", strings.Repeat("line\n", 10000))
	m.SetFilterFunc(func(s string) string { return s })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.GetFilteredLines("test")
	}
}

func BenchmarkGetFilteredLinesCached(b *testing.B) {
	m := NewManager()
	m.SetOutput("test", strings.Repeat("line\n", 10000))
	m.SetFilterFunc(func(s string) string { return s })

	// Prime the cache
	_ = m.GetFilteredLines("test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.GetFilteredLines("test")
	}
}

func BenchmarkGetFilteredOutput(b *testing.B) {
	m := NewManager()
	m.SetOutput("test", strings.Repeat("line\n", 10000))
	m.SetFilterFunc(func(s string) string { return s })

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.GetFilteredOutput("test")
	}
}

func BenchmarkCountLines(b *testing.B) {
	input := strings.Repeat("line\n", 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = countLines(input)
	}
}

func BenchmarkSplitLines(b *testing.B) {
	input := strings.Repeat("line\n", 10000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = splitLines(input)
	}
}

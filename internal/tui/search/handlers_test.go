package search

import (
	"testing"
)

// mockContext implements Context for testing purposes.
type mockContext struct {
	searchInput  string
	engine       *Engine
	output       string
	viewportH    int
	outputScroll int
}

func newMockContext() *mockContext {
	return &mockContext{
		engine:    NewEngine(),
		viewportH: 20,
	}
}

func (c *mockContext) GetSearchInput() string {
	return c.searchInput
}

func (c *mockContext) SetSearchInput(input string) {
	c.searchInput = input
}

func (c *mockContext) GetSearchEngine() *Engine {
	return c.engine
}

func (c *mockContext) GetOutputForActiveInstance() string {
	return c.output
}

func (c *mockContext) GetViewportHeight() int {
	return c.viewportH
}

func (c *mockContext) GetOutputScroll() int {
	return c.outputScroll
}

func (c *mockContext) SetOutputScroll(scroll int) {
	c.outputScroll = scroll
}

func TestNewHandler(t *testing.T) {
	ctx := newMockContext()
	h := NewHandler(ctx)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if h.ctx != ctx {
		t.Error("Handler context not set correctly")
	}
}

func TestHandler_HandleRunes(t *testing.T) {
	ctx := newMockContext()
	ctx.output = "hello world\ntest line"
	h := NewHandler(ctx)

	h.HandleRunes("hel")

	if ctx.searchInput != "hel" {
		t.Errorf("expected search input 'hel', got %q", ctx.searchInput)
	}
	// Should have executed live search
	if ctx.engine.MatchCount() != 1 {
		t.Errorf("expected 1 match for 'hel', got %d", ctx.engine.MatchCount())
	}
}

func TestHandler_HandleBackspace(t *testing.T) {
	ctx := newMockContext()
	ctx.output = "hello world\ntest line"
	ctx.searchInput = "hello"
	h := NewHandler(ctx)

	h.HandleBackspace()

	if ctx.searchInput != "hell" {
		t.Errorf("expected search input 'hell', got %q", ctx.searchInput)
	}
}

func TestHandler_HandleBackspace_EmptyInput(t *testing.T) {
	ctx := newMockContext()
	ctx.searchInput = ""
	h := NewHandler(ctx)

	h.HandleBackspace() // Should not panic

	if ctx.searchInput != "" {
		t.Errorf("expected empty search input, got %q", ctx.searchInput)
	}
}

func TestHandler_Execute(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		output        string
		expectedCount int
	}{
		{
			name:          "empty input clears search",
			input:         "",
			output:        "hello world",
			expectedCount: 0,
		},
		{
			name:          "empty output returns no matches",
			input:         "hello",
			output:        "",
			expectedCount: 0,
		},
		{
			name:          "literal search finds matches",
			input:         "hello",
			output:        "hello world\nhello go",
			expectedCount: 2,
		},
		{
			name:          "regex search finds matches",
			input:         "r:^hello",
			output:        "hello world\ntest hello\nhello go",
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.searchInput = tt.input
			ctx.output = tt.output
			h := NewHandler(ctx)

			h.Execute()

			if ctx.engine.MatchCount() != tt.expectedCount {
				t.Errorf("expected %d matches, got %d", tt.expectedCount, ctx.engine.MatchCount())
			}
		})
	}
}

func TestHandler_Clear(t *testing.T) {
	ctx := newMockContext()
	ctx.searchInput = "hello"
	ctx.output = "hello world"
	ctx.outputScroll = 100
	h := NewHandler(ctx)

	// First search something
	h.Execute()
	if ctx.engine.MatchCount() == 0 {
		t.Fatal("expected matches before clear")
	}

	// Now clear
	h.Clear()

	if ctx.searchInput != "" {
		t.Errorf("expected empty search input, got %q", ctx.searchInput)
	}
	if ctx.engine.MatchCount() != 0 {
		t.Errorf("expected 0 matches after clear, got %d", ctx.engine.MatchCount())
	}
	if ctx.outputScroll != 0 {
		t.Errorf("expected scroll reset to 0, got %d", ctx.outputScroll)
	}
}

func TestHandler_ScrollToMatch(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 20
	ctx.output = multilineOutput(50) // 50 lines of content
	ctx.searchInput = "line 30"      // Match on line 30
	h := NewHandler(ctx)

	h.Execute()

	// Should scroll to center the match
	// Match is on line 30, viewport is 20, so scroll should be 30 - 10 = 20
	expectedScroll := 20
	if ctx.outputScroll != expectedScroll {
		t.Errorf("expected scroll %d, got %d", expectedScroll, ctx.outputScroll)
	}
}

func TestHandler_ScrollToMatch_NearTop(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 20
	ctx.output = "line 0\nline 1\nline 2\nline 3\n" + multilineOutput(50)
	ctx.searchInput = "line 1"
	h := NewHandler(ctx)

	h.Execute()

	// Match is on line 1, centering would give negative scroll, should clamp to 0
	if ctx.outputScroll < 0 {
		t.Errorf("scroll should not be negative, got %d", ctx.outputScroll)
	}
}

func TestHandler_ScrollToMatch_SmallViewport(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 3 // Small viewport (below minimum of 5)
	ctx.output = multilineOutput(50)
	ctx.searchInput = "line 20"
	h := NewHandler(ctx)

	h.Execute()

	// Viewport should be treated as minimum of 5
	// Match on line 20, viewport 5, scroll = 20 - 2 = 18
	expectedScroll := 18
	if ctx.outputScroll != expectedScroll {
		t.Errorf("expected scroll %d (using min viewport 5), got %d", expectedScroll, ctx.outputScroll)
	}
}

func TestHandler_NextMatch(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 20
	ctx.output = "match here\nno match\nmatch here\nno match\nmatch here"
	ctx.searchInput = "match"
	h := NewHandler(ctx)

	h.Execute()
	initialIdx := ctx.engine.CurrentIndex()

	result := h.NextMatch()

	if !result {
		t.Error("NextMatch should return true when matches exist")
	}
	if ctx.engine.CurrentIndex() != initialIdx+1 {
		t.Errorf("expected current index %d, got %d", initialIdx+1, ctx.engine.CurrentIndex())
	}
}

func TestHandler_NextMatch_NoMatches(t *testing.T) {
	ctx := newMockContext()
	ctx.output = "no matches here"
	ctx.searchInput = "xyz"
	h := NewHandler(ctx)

	h.Execute()
	result := h.NextMatch()

	if result {
		t.Error("NextMatch should return false when no matches")
	}
}

func TestHandler_PreviousMatch(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 20
	ctx.output = "match here\nno match\nmatch here\nno match\nmatch here"
	ctx.searchInput = "match"
	h := NewHandler(ctx)

	h.Execute()
	// Move to next match first
	h.NextMatch()
	currentIdx := ctx.engine.CurrentIndex()

	result := h.PreviousMatch()

	if !result {
		t.Error("PreviousMatch should return true when matches exist")
	}
	if ctx.engine.CurrentIndex() != currentIdx-1 {
		t.Errorf("expected current index %d, got %d", currentIdx-1, ctx.engine.CurrentIndex())
	}
}

func TestHandler_PreviousMatch_NoMatches(t *testing.T) {
	ctx := newMockContext()
	ctx.output = "no matches here"
	ctx.searchInput = "xyz"
	h := NewHandler(ctx)

	h.Execute()
	result := h.PreviousMatch()

	if result {
		t.Error("PreviousMatch should return false when no matches")
	}
}

func TestHandler_PreviousMatch_WrapsAround(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 20
	ctx.output = "match one\nmatch two\nmatch three"
	ctx.searchInput = "match"
	h := NewHandler(ctx)

	h.Execute()
	// Current index is 0 (first match)
	result := h.PreviousMatch()

	if !result {
		t.Error("PreviousMatch should return true")
	}
	// Should wrap to last match (index 2)
	if ctx.engine.CurrentIndex() != 2 {
		t.Errorf("expected wrap to index 2, got %d", ctx.engine.CurrentIndex())
	}
}

func TestHandler_LiveSearchUpdate(t *testing.T) {
	ctx := newMockContext()
	ctx.viewportH = 20
	ctx.output = "hello world\nhello go\nhellooo"
	h := NewHandler(ctx)

	// Simulate typing "hel"
	h.HandleRunes("h")
	if ctx.engine.MatchCount() != 3 {
		t.Errorf("expected 3 matches for 'h', got %d", ctx.engine.MatchCount())
	}

	h.HandleRunes("e")
	if ctx.engine.MatchCount() != 3 {
		t.Errorf("expected 3 matches for 'he', got %d", ctx.engine.MatchCount())
	}

	h.HandleRunes("llo")
	if ctx.engine.MatchCount() != 3 {
		t.Errorf("expected 3 matches for 'hello', got %d", ctx.engine.MatchCount())
	}

	h.HandleRunes("o")
	if ctx.engine.MatchCount() != 1 {
		t.Errorf("expected 1 match for 'helloo', got %d", ctx.engine.MatchCount())
	}
}

func TestHandler_ScrollToMatch_NoCurrentMatch(t *testing.T) {
	ctx := newMockContext()
	ctx.outputScroll = 50
	h := NewHandler(ctx)

	// No search executed, engine has no matches
	h.ScrollToMatch()

	// Scroll should remain unchanged
	if ctx.outputScroll != 50 {
		t.Errorf("scroll should remain unchanged when no matches, got %d", ctx.outputScroll)
	}
}

// multilineOutput generates output with numbered lines for testing scroll behavior.
func multilineOutput(lines int) string {
	result := ""
	for i := 0; i < lines; i++ {
		if i > 0 {
			result += "\n"
		}
		result += "line " + itoa(i)
	}
	return result
}

// itoa is a simple int-to-string converter for test use.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

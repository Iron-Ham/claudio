package search

import (
	"testing"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.Pattern() != "" {
		t.Errorf("new engine should have empty pattern, got %q", e.Pattern())
	}
	if e.Regex() != nil {
		t.Error("new engine should have nil regex")
	}
	if e.MatchCount() != 0 {
		t.Errorf("new engine should have 0 matches, got %d", e.MatchCount())
	}
}

func TestSearch_LiteralPattern(t *testing.T) {
	e := NewEngine()
	content := "Hello World\nHello Go\nGoodbye World"

	results := e.Search("hello", content)

	if len(results) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(results))
	}

	// First match: line 0, "Hello"
	if results[0].LineNumber != 0 {
		t.Errorf("first match line: expected 0, got %d", results[0].LineNumber)
	}
	if results[0].StartIndex != 0 {
		t.Errorf("first match start: expected 0, got %d", results[0].StartIndex)
	}
	if results[0].EndIndex != 5 {
		t.Errorf("first match end: expected 5, got %d", results[0].EndIndex)
	}

	// Second match: line 1, "Hello"
	if results[1].LineNumber != 1 {
		t.Errorf("second match line: expected 1, got %d", results[1].LineNumber)
	}
}

func TestSearch_CaseInsensitive(t *testing.T) {
	e := NewEngine()
	content := "Hello WORLD\nhello world\nHELLO"

	results := e.Search("hello", content)

	if len(results) != 3 {
		t.Fatalf("expected 3 case-insensitive matches, got %d", len(results))
	}
}

func TestSearch_RegexPattern(t *testing.T) {
	e := NewEngine()
	content := "Error: file not found\nWarning: deprecated\nError: connection failed"

	results := e.Search("r:^Error:", content)

	if len(results) != 2 {
		t.Fatalf("expected 2 regex matches, got %d", len(results))
	}

	if results[0].LineNumber != 0 {
		t.Errorf("first match line: expected 0, got %d", results[0].LineNumber)
	}
	if results[1].LineNumber != 2 {
		t.Errorf("second match line: expected 2, got %d", results[1].LineNumber)
	}
}

func TestSearch_RegexCaseInsensitive(t *testing.T) {
	e := NewEngine()
	content := "ERROR: test\nerror: test\nError: test"

	results := e.Search("r:error:", content)

	if len(results) != 3 {
		t.Fatalf("expected 3 case-insensitive regex matches, got %d", len(results))
	}
}

func TestSearch_InvalidRegex(t *testing.T) {
	e := NewEngine()
	content := "test content"

	results := e.Search("r:[invalid", content)

	if len(results) != 0 {
		t.Errorf("invalid regex should return no matches, got %d", len(results))
	}
	if e.Regex() != nil {
		t.Error("invalid regex should result in nil regex")
	}
}

func TestSearch_EmptyPattern(t *testing.T) {
	e := NewEngine()
	content := "test content"

	results := e.Search("", content)

	if results != nil {
		t.Error("empty pattern should return nil")
	}
	if e.MatchCount() != 0 {
		t.Errorf("empty pattern should have 0 matches, got %d", e.MatchCount())
	}
}

func TestSearch_EmptyContent(t *testing.T) {
	e := NewEngine()

	results := e.Search("test", "")

	if results != nil {
		t.Error("empty content should return nil")
	}
}

func TestSearch_EmptyRegexPattern(t *testing.T) {
	e := NewEngine()
	content := "test content"

	results := e.Search("r:", content)

	if results != nil {
		t.Error("empty regex pattern should return nil")
	}
}

func TestSearch_SpecialCharacters(t *testing.T) {
	e := NewEngine()
	content := "func() { return }\n[array]\n*.go"

	// Literal search should escape special regex characters
	results := e.Search("()", content)
	if len(results) != 1 {
		t.Fatalf("expected 1 match for literal '()', got %d", len(results))
	}

	results = e.Search("[array]", content)
	if len(results) != 1 {
		t.Fatalf("expected 1 match for literal '[array]', got %d", len(results))
	}

	results = e.Search("*.go", content)
	if len(results) != 1 {
		t.Fatalf("expected 1 match for literal '*.go', got %d", len(results))
	}
}

func TestSearch_MultipleMatchesPerLine(t *testing.T) {
	e := NewEngine()
	content := "foo bar foo baz foo"

	results := e.Search("foo", content)

	if len(results) != 3 {
		t.Fatalf("expected 3 matches on single line, got %d", len(results))
	}

	// All matches should be on line 0
	for i, r := range results {
		if r.LineNumber != 0 {
			t.Errorf("match %d: expected line 0, got %d", i, r.LineNumber)
		}
	}

	// Check positions
	expectedStarts := []int{0, 8, 16}
	for i, r := range results {
		if r.StartIndex != expectedStarts[i] {
			t.Errorf("match %d: expected start %d, got %d", i, expectedStarts[i], r.StartIndex)
		}
	}
}

func TestNavigation_Next(t *testing.T) {
	e := NewEngine()
	content := "match1\nmatch2\nmatch3"
	e.Search("match", content)

	if e.CurrentIndex() != 0 {
		t.Errorf("initial current should be 0, got %d", e.CurrentIndex())
	}

	r := e.Next()
	if r == nil {
		t.Fatal("Next returned nil")
	}
	if e.CurrentIndex() != 1 {
		t.Errorf("after Next, current should be 1, got %d", e.CurrentIndex())
	}

	_ = e.Next()
	if e.CurrentIndex() != 2 {
		t.Errorf("after second Next, current should be 2, got %d", e.CurrentIndex())
	}

	// Should wrap around
	_ = e.Next()
	if e.CurrentIndex() != 0 {
		t.Errorf("after wraparound, current should be 0, got %d", e.CurrentIndex())
	}
}

func TestNavigation_Previous(t *testing.T) {
	e := NewEngine()
	content := "match1\nmatch2\nmatch3"
	e.Search("match", content)

	// Should wrap to last
	r := e.Previous()
	if r == nil {
		t.Fatal("Previous returned nil")
	}
	if e.CurrentIndex() != 2 {
		t.Errorf("after Previous from 0, current should be 2, got %d", e.CurrentIndex())
	}

	_ = e.Previous()
	if e.CurrentIndex() != 1 {
		t.Errorf("after second Previous, current should be 1, got %d", e.CurrentIndex())
	}
}

func TestNavigation_NoMatches(t *testing.T) {
	e := NewEngine()
	content := "no matches here"
	e.Search("xyz", content)

	if r := e.Next(); r != nil {
		t.Error("Next with no matches should return nil")
	}
	if r := e.Previous(); r != nil {
		t.Error("Previous with no matches should return nil")
	}
	if r := e.Current(); r != nil {
		t.Error("Current with no matches should return nil")
	}
}

func TestCurrent(t *testing.T) {
	e := NewEngine()
	content := "line1 match\nline2 match"
	e.Search("match", content)

	r := e.Current()
	if r == nil {
		t.Fatal("Current returned nil")
	}
	if r.LineNumber != 0 {
		t.Errorf("Current line: expected 0, got %d", r.LineNumber)
	}

	e.Next()
	r = e.Current()
	if r.LineNumber != 1 {
		t.Errorf("Current after Next: expected line 1, got %d", r.LineNumber)
	}
}

func TestClear(t *testing.T) {
	e := NewEngine()
	content := "test match"
	e.Search("match", content)

	if e.MatchCount() == 0 {
		t.Fatal("search should have found matches")
	}

	e.Clear()

	if e.Pattern() != "" {
		t.Error("Clear should reset pattern")
	}
	if e.Regex() != nil {
		t.Error("Clear should reset regex")
	}
	if e.MatchCount() != 0 {
		t.Error("Clear should reset matches")
	}
	if e.CurrentIndex() != -1 {
		t.Errorf("Clear should reset current to -1, got %d", e.CurrentIndex())
	}
	if e.HasMatches() {
		t.Error("Clear should result in HasMatches returning false")
	}
}

func TestMatchingLines(t *testing.T) {
	e := NewEngine()
	content := "foo bar foo\nbaz\nfoo qux"
	e.Search("foo", content)

	lines := e.MatchingLines()

	if len(lines) != 2 {
		t.Fatalf("expected 2 unique lines, got %d", len(lines))
	}

	// Line 0 has two "foo"s, line 2 has one "foo"
	if lines[0] != 0 || lines[1] != 2 {
		t.Errorf("expected lines [0, 2], got %v", lines)
	}
}

func TestMatchingLines_NoMatches(t *testing.T) {
	e := NewEngine()
	e.Search("xyz", "no match")

	lines := e.MatchingLines()
	if lines != nil {
		t.Errorf("no matches should return nil, got %v", lines)
	}
}

func TestHighlight(t *testing.T) {
	e := NewEngine()
	e.Search("test", "before test after")

	matchStyle := func(s string) string { return "[" + s + "]" }
	currentStyle := func(s string) string { return "[[" + s + "]]" }

	// Current match line
	result := e.Highlight("before test after", true, matchStyle, currentStyle)
	expected := "before [[test]] after"
	if result != expected {
		t.Errorf("highlight current: expected %q, got %q", expected, result)
	}

	// Non-current match line
	result = e.Highlight("before test after", false, matchStyle, currentStyle)
	expected = "before [test] after"
	if result != expected {
		t.Errorf("highlight non-current: expected %q, got %q", expected, result)
	}
}

func TestHighlight_MultipleMatches(t *testing.T) {
	e := NewEngine()
	e.Search("x", "x y x z x")

	matchStyle := func(s string) string { return "[" + s + "]" }
	currentStyle := func(s string) string { return "[[" + s + "]]" }

	// On current line, only first match gets current style
	result := e.Highlight("x y x z x", true, matchStyle, currentStyle)
	expected := "[[x]] y [x] z [x]"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestHighlight_NoMatch(t *testing.T) {
	e := NewEngine()
	e.Search("xyz", "test content")

	matchStyle := func(s string) string { return "[" + s + "]" }

	result := e.Highlight("no match here", false, matchStyle, nil)
	expected := "no match here"
	if result != expected {
		t.Errorf("no match: expected unchanged text, got %q", result)
	}
}

func TestHighlight_NilRegex(t *testing.T) {
	e := NewEngine()
	// No search performed

	matchStyle := func(s string) string { return "[" + s + "]" }
	result := e.Highlight("test content", false, matchStyle, nil)
	expected := "test content"
	if result != expected {
		t.Errorf("nil regex: expected unchanged text, got %q", result)
	}
}

func TestHighlight_NilMatchStyle(t *testing.T) {
	e := NewEngine()
	e.Search("test", "test content")

	result := e.Highlight("test content", false, nil, nil)
	expected := "test content"
	if result != expected {
		t.Errorf("nil style: expected unchanged text, got %q", result)
	}
}

func TestHighlight_NilCurrentStyle(t *testing.T) {
	e := NewEngine()
	e.Search("test", "test content")

	matchStyle := func(s string) string { return "[" + s + "]" }

	// Should fall back to matchStyle when currentStyle is nil
	result := e.Highlight("test content", true, matchStyle, nil)
	expected := "[test] content"
	if result != expected {
		t.Errorf("nil currentStyle: expected %q, got %q", expected, result)
	}
}

func TestHighlightLine(t *testing.T) {
	e := NewEngine()
	content := "match line 0\nno match\nmatch line 2"
	e.Search("match", content)

	matchStyle := func(s string) string { return "[" + s + "]" }
	currentStyle := func(s string) string { return "[[" + s + "]]" }

	// Current is at line 0
	result := e.HighlightLine("match line 0", 0, matchStyle, currentStyle)
	expected := "[[match]] line 0"
	if result != expected {
		t.Errorf("current line: expected %q, got %q", expected, result)
	}

	// Line 2 is not current
	result = e.HighlightLine("match line 2", 2, matchStyle, currentStyle)
	expected = "[match] line 2"
	if result != expected {
		t.Errorf("non-current line: expected %q, got %q", expected, result)
	}
}

func TestSetCurrent(t *testing.T) {
	e := NewEngine()
	content := "a\nb\nc\nd"
	e.Search("r:.", content) // Match all single chars (regex mode)

	e.SetCurrent(2)
	if e.CurrentIndex() != 2 {
		t.Errorf("after SetCurrent(2): expected 2, got %d", e.CurrentIndex())
	}

	// Out of bounds - upper
	e.SetCurrent(100)
	if e.CurrentIndex() != len(e.Results())-1 {
		t.Errorf("SetCurrent above bounds: expected %d, got %d", len(e.Results())-1, e.CurrentIndex())
	}

	// Out of bounds - lower
	e.SetCurrent(-5)
	if e.CurrentIndex() != 0 {
		t.Errorf("SetCurrent below bounds: expected 0, got %d", e.CurrentIndex())
	}
}

func TestSetCurrent_NoMatches(t *testing.T) {
	e := NewEngine()
	e.Search("xyz", "no match")

	e.SetCurrent(5)
	if e.CurrentIndex() != -1 {
		t.Errorf("SetCurrent with no matches: expected -1, got %d", e.CurrentIndex())
	}
}

func TestJumpToLine(t *testing.T) {
	e := NewEngine()
	content := "line0\nmatch1\nline2\nmatch3\nline4"
	e.Search("match", content)

	// Jump to line 2 should find match on line 3
	found := e.JumpToLine(2)
	if !found {
		t.Error("JumpToLine(2) should find match on line 3")
	}
	if r := e.Current(); r == nil || r.LineNumber != 3 {
		t.Errorf("after JumpToLine(2): expected line 3, got %v", r)
	}

	// Jump to exact match line
	found = e.JumpToLine(1)
	if !found {
		t.Error("JumpToLine(1) should find match")
	}
	if r := e.Current(); r == nil || r.LineNumber != 1 {
		t.Errorf("after JumpToLine(1): expected line 1, got %v", r)
	}

	// Jump beyond all matches
	found = e.JumpToLine(100)
	if found {
		t.Error("JumpToLine beyond all matches should return false")
	}
}

func TestResults(t *testing.T) {
	e := NewEngine()
	content := "a\nb\nc"
	e.Search("r:.", content) // Match all single chars (regex mode)

	results := e.Results()
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestHasMatches(t *testing.T) {
	e := NewEngine()

	if e.HasMatches() {
		t.Error("new engine should not have matches")
	}

	e.Search("test", "test content")
	if !e.HasMatches() {
		t.Error("after search should have matches")
	}

	e.Search("xyz", "test content")
	if e.HasMatches() {
		t.Error("after failed search should not have matches")
	}
}

func TestSearch_ReplacesOldResults(t *testing.T) {
	e := NewEngine()

	e.Search("first", "first second")
	if e.MatchCount() != 1 {
		t.Fatal("first search should find 1 match")
	}

	e.Search("second", "first second")
	if e.MatchCount() != 1 {
		t.Fatal("second search should find 1 match")
	}
	if e.Pattern() != "second" {
		t.Errorf("pattern should be updated to 'second', got %q", e.Pattern())
	}

	// Current should reset
	if e.CurrentIndex() != 0 {
		t.Errorf("current should reset to 0, got %d", e.CurrentIndex())
	}
}

func TestSearch_Unicode(t *testing.T) {
	e := NewEngine()
	content := "Hello 世界\n你好 World\n日本語"

	results := e.Search("世界", content)
	if len(results) != 1 {
		t.Fatalf("expected 1 unicode match, got %d", len(results))
	}
	if results[0].LineNumber != 0 {
		t.Errorf("unicode match: expected line 0, got %d", results[0].LineNumber)
	}
}

func TestHighlight_Unicode(t *testing.T) {
	e := NewEngine()
	e.Search("世界", "Hello 世界!")

	matchStyle := func(s string) string { return "[" + s + "]" }
	result := e.Highlight("Hello 世界!", false, matchStyle, nil)
	expected := "Hello [世界]!"
	if result != expected {
		t.Errorf("unicode highlight: expected %q, got %q", expected, result)
	}
}

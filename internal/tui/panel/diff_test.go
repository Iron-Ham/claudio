package panel

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestDiffPanel_Render(t *testing.T) {
	tests := []struct {
		name     string
		state    *RenderState
		contains []string
		notEmpty bool
	}{
		{
			name: "renders diff with content",
			state: &RenderState{
				Width:  80,
				Height: 50,
				ActiveInstance: &orchestrator.Instance{
					Branch: "feature/test-branch",
				},
				DiffContent: `diff --git a/file.go b/file.go
index abc123..def456 100644
--- a/file.go
+++ b/file.go
@@ -1,5 +1,6 @@
 package main

+import "fmt"

 func main() {
-    println("hello")
+    fmt.Println("hello")
 }`,
			},
			contains: []string{
				"Diff Preview",
				"feature/test-branch",
				"diff --git",
				"+import",
				"-    println",
				"+    fmt.Println",
				"@@",
			},
			notEmpty: true,
		},
		{
			name: "shows no changes message when empty",
			state: &RenderState{
				Width:       80,
				Height:      50,
				DiffContent: "",
			},
			contains: []string{
				"Diff Preview",
				"No changes to display",
			},
			notEmpty: true,
		},
		{
			name: "shows branch name when available",
			state: &RenderState{
				Width:  80,
				Height: 50,
				ActiveInstance: &orchestrator.Instance{
					Branch: "my-feature",
				},
				DiffContent: "+line added",
			},
			contains: []string{
				"Diff Preview: my-feature",
			},
			notEmpty: true,
		},
		{
			name: "shows generic title without active instance",
			state: &RenderState{
				Width:       80,
				Height:      50,
				DiffContent: "+line added",
			},
			contains: []string{
				"Diff Preview",
			},
			notEmpty: true,
		},
		{
			name: "handles scrolling",
			state: &RenderState{
				Width:        80,
				Height:       20,
				ScrollOffset: 2,
				DiffContent:  "+line1\n+line2\n+line3\n+line4\n+line5\n+line6\n+line7\n+line8\n+line9\n+line10",
			},
			contains: []string{
				"Lines", // Scroll indicator
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

	panel := NewDiffPanel()

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

func TestDiffPanel_Height(t *testing.T) {
	panel := NewDiffPanel()

	state := &RenderState{
		Width:  80,
		Height: 50,
		DiffContent: `+line1
+line2
+line3`,
	}

	panel.Render(state)

	if panel.Height() <= 0 {
		t.Errorf("Height() = %d, want positive value", panel.Height())
	}
}

func TestDiffPanel_SyntaxHighlighting(t *testing.T) {
	panel := NewDiffPanel()

	// Test that different line types are rendered differently
	// We can't easily check ANSI codes, but we can verify the lines are present
	state := &RenderState{
		Width:  80,
		Height: 50,
		DiffContent: `diff --git a/file.go b/file.go
--- a/file.go
+++ b/file.go
@@ -1,3 +1,3 @@
 context line
-removed line
+added line`,
	}

	result := panel.Render(state)

	expectedLines := []string{
		"diff --git",
		"---",
		"+++",
		"@@",
		"context line",
		"-removed line",
		"+added line",
	}

	for _, line := range expectedLines {
		if !strings.Contains(result, line) {
			t.Errorf("result missing line %q", line)
		}
	}
}

func TestDiffPanel_ScrollClamping(t *testing.T) {
	panel := NewDiffPanel()

	// Test negative scroll is clamped
	state := &RenderState{
		Width:        80,
		Height:       50,
		ScrollOffset: -10,
		DiffContent:  "+line1\n+line2\n+line3",
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with negative scroll")
	}
}

func TestDiffPanel_ExcessiveScroll(t *testing.T) {
	panel := NewDiffPanel()

	// Test excessive scroll is clamped
	state := &RenderState{
		Width:        80,
		Height:       50,
		ScrollOffset: 10000,
		DiffContent:  "+line1\n+line2\n+line3",
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with excessive scroll")
	}
}

func TestDiffPanel_WithTheme(t *testing.T) {
	panel := NewDiffPanel()

	state := &RenderState{
		Width:  80,
		Height: 50,
		Theme:  &mockTheme{},
		DiffContent: `+added line
-removed line`,
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with theme")
	}

	if !strings.Contains(result, "added line") {
		t.Error("result missing added line")
	}
	if !strings.Contains(result, "removed line") {
		t.Error("result missing removed line")
	}
}

func TestDiffPanel_EmptyLines(t *testing.T) {
	panel := NewDiffPanel()

	state := &RenderState{
		Width:  80,
		Height: 50,
		DiffContent: `+line1

+line2`,
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with empty lines in diff")
	}
}

func TestNewDiffPanel(t *testing.T) {
	panel := NewDiffPanel()
	if panel == nil {
		t.Error("NewDiffPanel() returned nil")
	}
}

func TestDiffPanel_HighlightDiffLine(t *testing.T) {
	panel := NewDiffPanel()

	// Test without theme (uses default highlighting)
	lines := []struct {
		input   string
		hasAnsi bool // If default styling is applied, will have ANSI codes
	}{
		{"+added", true},
		{"-removed", true},
		{"@@hunk@@", true},
		{"+++header", true},
		{"---header", true},
		{"diff file", true},
		{"index 123", true},
		{"context", false}, // Context lines have no special default styling
		{"", false},        // Empty lines
	}

	for _, tc := range lines {
		result := panel.highlightDiffLine(tc.input, nil)
		// Just verify we get the original content (possibly styled)
		if !strings.Contains(result, strings.TrimPrefix(strings.TrimPrefix(tc.input, "+"), "-")) {
			// For empty lines, the original content is empty
			if tc.input != "" {
				t.Errorf("highlightDiffLine(%q) didn't contain original content", tc.input)
			}
		}
	}
}

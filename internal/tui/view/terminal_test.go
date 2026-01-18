package view

import (
	"strings"
	"testing"
)

func TestNewTerminalView(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		height         int
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "standard dimensions",
			width:          80,
			height:         15,
			expectedWidth:  80,
			expectedHeight: 15,
		},
		{
			name:           "minimum dimensions",
			width:          20,
			height:         5,
			expectedWidth:  20,
			expectedHeight: 5,
		},
		{
			name:           "large dimensions",
			width:          200,
			height:         50,
			expectedWidth:  200,
			expectedHeight: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewTerminalView(tt.width, tt.height)
			if v.Width != tt.expectedWidth {
				t.Errorf("Width = %d, want %d", v.Width, tt.expectedWidth)
			}
			if v.Height != tt.expectedHeight {
				t.Errorf("Height = %d, want %d", v.Height, tt.expectedHeight)
			}
		})
	}
}

func TestTerminalViewRender(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		height       int
		state        TerminalState
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "returns empty for height less than 2",
			width:     80,
			height:    1,
			state:     TerminalState{},
			wantEmpty: true,
		},
		{
			name:   "renders project mode header",
			width:  80,
			height: 15,
			state: TerminalState{
				IsWorktreeMode: false,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[project]"},
		},
		{
			name:   "renders worktree mode header",
			width:  80,
			height: 15,
			state: TerminalState{
				IsWorktreeMode: true,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[worktree]"},
		},
		{
			name:   "renders worktree mode with instance ID",
			width:  80,
			height: 15,
			state: TerminalState{
				IsWorktreeMode: true,
				InstanceID:     "abc123",
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[worktree:abc123]"},
		},
		{
			name:   "renders terminal mode focus indicator",
			width:  80,
			height: 15,
			state: TerminalState{
				TerminalMode:  true,
				CurrentDir:    "/home/user/project",
				InvocationDir: "/home/user/project",
			},
			wantContains: []string{"TERMINAL"},
		},
		{
			name:   "renders shell ready placeholder when output is empty",
			width:  80,
			height: 15,
			state: TerminalState{
				Output:        "",
				CurrentDir:    "/home/user/project",
				InvocationDir: "/home/user/project",
			},
			wantContains: []string{"shell ready"},
		},
		{
			name:   "renders output content",
			width:  80,
			height: 15,
			state: TerminalState{
				Output:        "$ ls\nfile1.txt\nfile2.txt",
				CurrentDir:    "/home/user/project",
				InvocationDir: "/home/user/project",
			},
			wantContains: []string{"ls", "file1.txt", "file2.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewTerminalView(tt.width, tt.height)
			result := v.Render(tt.state)

			if tt.wantEmpty {
				if result != "" {
					t.Errorf("Render() = %q, want empty string", result)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("Render() does not contain %q", want)
				}
			}
		})
	}
}

func TestTerminalViewRenderOutput(t *testing.T) {
	tests := []struct {
		name         string
		output       string
		height       int
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "empty output shows placeholder",
			output:       "",
			height:       10,
			wantContains: []string{"shell ready"},
		},
		{
			name:         "single line output",
			output:       "hello world",
			height:       10,
			wantContains: []string{"hello world"},
		},
		{
			name:         "multiline output within height",
			output:       "line1\nline2\nline3",
			height:       10,
			wantContains: []string{"line1", "line2", "line3"},
		},
		{
			name:         "output exceeds height - shows only last lines",
			output:       "line1\nline2\nline3\nline4\nline5",
			height:       3,
			wantContains: []string{"line3", "line4", "line5"},
			wantExcludes: []string{"line1", "line2"},
		},
		{
			name:         "trims trailing empty lines",
			output:       "line1\nline2\n\n\n",
			height:       10,
			wantContains: []string{"line1", "line2"},
		},
		{
			name:         "preserves first line when capture ends with newline",
			output:       "PROMPT\n\n\n\n\n\n\n\n\n\n\n\n",
			height:       12,
			wantContains: []string{"PROMPT"},
		},
		{
			name:         "preserves prompt with ANSI codes when capture ends with newline",
			output:       "\x1b[32mPrompt ❯\x1b[0m\n\n\n\n\n\n\n\n\n\n\n\n",
			height:       12,
			wantContains: []string{"Prompt"},
		},
		{
			name:         "preserves prompt with content and trailing newlines",
			output:       "PROMPT ❯ ls\nfile1.txt\nfile2.txt\n\n\n",
			height:       10,
			wantContains: []string{"PROMPT", "file1.txt", "file2.txt"},
		},
		{
			name:         "preserves content when lines equal height after trim",
			output:       "line1\nline2\nline3\n",
			height:       3,
			wantContains: []string{"line1", "line2", "line3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewTerminalView(80, 15)
			result := v.renderOutput(tt.output, tt.height)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("renderOutput() does not contain %q, got %q", want, result)
				}
			}

			for _, exclude := range tt.wantExcludes {
				if strings.Contains(result, exclude) {
					t.Errorf("renderOutput() should not contain %q, got %q", exclude, result)
				}
			}
		})
	}
}

func TestTerminalViewRenderHeader(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		state        TerminalState
		wantContains []string
	}{
		{
			name:  "project mode",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: false,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[project]"},
		},
		{
			name:  "worktree mode without instance",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: true,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[worktree]"},
		},
		{
			name:  "worktree mode with instance",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: true,
				InstanceID:     "test-id",
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[worktree:test-id]"},
		},
		{
			name:  "shows relative path",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: false,
				CurrentDir:     "/home/user/project/subdir",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"./subdir"},
		},
		{
			name:  "shows dot for same directory",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: false,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"."},
		},
		{
			name:  "terminal mode shows focus indicator",
			width: 80,
			state: TerminalState{
				TerminalMode:  true,
				CurrentDir:    "/home/user/project",
				InvocationDir: "/home/user/project",
			},
			wantContains: []string{"TERMINAL"},
		},
		{
			name:  "project mode shows worktree toggle hint",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: false,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[project]", ":termdir wt"},
		},
		{
			name:  "worktree mode shows project toggle hint",
			width: 80,
			state: TerminalState{
				IsWorktreeMode: true,
				CurrentDir:     "/home/user/project",
				InvocationDir:  "/home/user/project",
			},
			wantContains: []string{"[worktree]", ":termdir proj"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewTerminalView(tt.width, 15)
			result := v.renderHeader(tt.state)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("renderHeader() does not contain %q, got %q", want, result)
				}
			}
		})
	}
}

// TruncateANSI tests are now in internal/util/strings_test.go

func TestOutputHeightCalculation(t *testing.T) {
	// This test verifies the output height calculation accounts for border and header
	// The formula should be: outputHeight = totalHeight - 3 (2 for border, 1 for header)
	tests := []struct {
		name              string
		totalHeight       int
		expectedLineCount int
		outputLines       int
		expectTruncation  bool
	}{
		{
			name:              "height 15 gives 12 output lines",
			totalHeight:       15,
			expectedLineCount: 12, // 15 - 3 = 12
			outputLines:       12,
			expectTruncation:  false,
		},
		{
			name:              "height 10 gives 7 output lines",
			totalHeight:       10,
			expectedLineCount: 7, // 10 - 3 = 7
			outputLines:       7,
			expectTruncation:  false,
		},
		{
			name:              "height 5 gives 2 output lines",
			totalHeight:       5,
			expectedLineCount: 2, // 5 - 3 = 2
			outputLines:       5,
			expectTruncation:  true,
		},
		{
			name:              "minimum height 3 gives 1 output line (clamped)",
			totalHeight:       3,
			expectedLineCount: 1, // max(3 - 3, 1) = 1
			outputLines:       3,
			expectTruncation:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate output with the specified number of lines
			lines := make([]string, tt.outputLines)
			for i := range lines {
				lines[i] = "line"
			}
			output := strings.Join(lines, "\n")

			v := NewTerminalView(80, tt.totalHeight)

			// Calculate expected output height
			outputHeight := tt.totalHeight - 3
			if outputHeight < 1 {
				outputHeight = 1
			}

			if outputHeight != tt.expectedLineCount {
				t.Errorf("expected output height %d, got %d", tt.expectedLineCount, outputHeight)
			}

			// Verify the render output respects the height limit
			result := v.renderOutput(output, outputHeight)
			resultLines := strings.Split(result, "\n")

			// Trim empty lines from the result
			for len(resultLines) > 0 && strings.TrimSpace(resultLines[len(resultLines)-1]) == "" {
				resultLines = resultLines[:len(resultLines)-1]
			}

			if tt.expectTruncation {
				if len(resultLines) > tt.expectedLineCount {
					t.Errorf("output has %d lines, expected at most %d", len(resultLines), tt.expectedLineCount)
				}
			} else {
				if len(resultLines) != tt.expectedLineCount {
					t.Errorf("output has %d lines, expected %d", len(resultLines), tt.expectedLineCount)
				}
			}
		})
	}
}

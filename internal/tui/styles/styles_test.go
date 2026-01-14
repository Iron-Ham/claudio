package styles

import "testing"

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status   string
		expected string // Expected color hex value
	}{
		{"working", "#10B981"},
		{"pending", "#9CA3AF"},
		{"waiting_input", "#F59E0B"},
		{"paused", "#60A5FA"},
		{"completed", "#A78BFA"},
		{"error", "#F87171"},
		{"creating_pr", "#F472B6"},
		{"stuck", "#FB923C"},
		{"timeout", "#F87171"},
		{"interrupted", "#FBBF24"},
		{"unknown", "#9CA3AF"}, // Should fall back to MutedColor
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusColor(tt.status)
			if string(got) != tt.expected {
				t.Errorf("StatusColor(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"working", "●"},
		{"pending", "○"},
		{"waiting_input", "?"},
		{"paused", "⏸"},
		{"completed", "✓"},
		{"error", "✗"},
		{"creating_pr", "↗"},
		{"stuck", "⏱"},
		{"timeout", "⏰"},
		{"interrupted", "⚡"},
		{"unknown", "●"}, // Should fall back to default
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusIcon(tt.status)
			if got != tt.expected {
				t.Errorf("StatusIcon(%q) = %q, want %q", tt.status, got, tt.expected)
			}
		})
	}
}

func TestStatusInterruptedConstant(t *testing.T) {
	// Verify the interrupted status color is defined
	if StatusInterrupted == "" {
		t.Error("StatusInterrupted color should be defined")
	}
	if string(StatusInterrupted) != "#FBBF24" {
		t.Errorf("StatusInterrupted = %q, want %q", StatusInterrupted, "#FBBF24")
	}
}

func TestLayoutConstants(t *testing.T) {
	// Verify the layout constants are consistent
	// This test ensures that if components are changed, the total is updated

	t.Run("HeaderFooterReserved equals sum of components", func(t *testing.T) {
		expected := HeaderLines + HelpBarLines + ViewNewlines
		if HeaderFooterReserved != expected {
			t.Errorf("HeaderFooterReserved = %d, want %d (sum of HeaderLines=%d + HelpBarLines=%d + ViewNewlines=%d)",
				HeaderFooterReserved, expected, HeaderLines, HelpBarLines, ViewNewlines)
		}
	})

	t.Run("HeaderLines accounts for Header style", func(t *testing.T) {
		// Header style has: text (1) + PaddingBottom(1) + BorderBottom (1) + MarginBottom(1) = 4
		if HeaderLines != 4 {
			t.Errorf("HeaderLines = %d, want 4 (text + PaddingBottom + BorderBottom + MarginBottom)", HeaderLines)
		}
	})

	t.Run("HelpBarLines accounts for HelpBar style", func(t *testing.T) {
		// HelpBar style has: MarginTop(1) + text (1) = 2
		if HelpBarLines != 2 {
			t.Errorf("HelpBarLines = %d, want 2 (MarginTop + text)", HelpBarLines)
		}
	})

	t.Run("ViewNewlines accounts for explicit newlines in View()", func(t *testing.T) {
		// View() adds: 1 newline after header + 1 newline before help bar = 2
		if ViewNewlines != 2 {
			t.Errorf("ViewNewlines = %d, want 2 (after header + before help bar)", ViewNewlines)
		}
	})
}

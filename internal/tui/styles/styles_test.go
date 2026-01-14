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

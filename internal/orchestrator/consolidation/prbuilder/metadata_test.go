package prbuilder

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

func TestBuildTitle(t *testing.T) {
	tests := []struct {
		name       string
		objective  string
		mode       consolidation.Mode
		groupIndex int
		limit      int
		want       string
	}{
		{
			name:       "single mode",
			objective:  "Add authentication",
			mode:       consolidation.ModeSingle,
			groupIndex: 0,
			limit:      50,
			want:       "ultraplan: Add authentication",
		},
		{
			name:       "single mode truncates long objective",
			objective:  "This is a very long objective that should be truncated to fit",
			mode:       consolidation.ModeSingle,
			groupIndex: 0,
			limit:      50,
			want:       "ultraplan: This is a very long objective that should be tr...",
		},
		{
			name:       "stacked mode first group",
			objective:  "Feature X",
			mode:       consolidation.ModeStacked,
			groupIndex: 0,
			limit:      50,
			want:       "ultraplan: group 1 - Feature X",
		},
		{
			name:       "stacked mode second group",
			objective:  "Feature Y",
			mode:       consolidation.ModeStacked,
			groupIndex: 1,
			limit:      50,
			want:       "ultraplan: group 2 - Feature Y",
		},
		{
			name:       "stacked mode tenth group",
			objective:  "Feature Z",
			mode:       consolidation.ModeStacked,
			groupIndex: 9,
			limit:      50,
			want:       "ultraplan: group 10 - Feature Z",
		},
		{
			name:       "stacked mode truncates with room for group",
			objective:  "This is a very long objective for stacked mode",
			mode:       consolidation.ModeStacked,
			groupIndex: 0,
			limit:      50,
			want:       "ultraplan: group 1 - This is a very long objective for sta...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTitle(tt.objective, tt.mode, tt.groupIndex, tt.limit)
			if got != tt.want {
				t.Errorf("buildTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildLabels(t *testing.T) {
	tests := []struct {
		name  string
		tasks []consolidation.CompletedTask
		want  []string
	}{
		{
			name:  "empty tasks returns nil",
			tasks: []consolidation.CompletedTask{},
			want:  nil,
		},
		{
			name: "tasks return nil (future feature)",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1"},
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLabels(tt.tasks)
			if len(got) != len(tt.want) {
				t.Errorf("buildLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string truncated with ellipsis",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "very short max length",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "max length of 4 adds ellipsis",
			input:  "hello world",
			maxLen: 4,
			want:   "h...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

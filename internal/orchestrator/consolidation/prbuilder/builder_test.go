package prbuilder

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

func TestBuilder_Build(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []consolidation.CompletedTask
		opts          consolidation.PRBuildOptions
		wantTitle     string
		wantBodyParts []string
	}{
		{
			name: "single task in single mode",
			tasks: []consolidation.CompletedTask{
				{
					ID:    "task-1",
					Title: "Add feature X",
				},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeSingle,
				Objective:   "Implement authentication",
				TotalGroups: 1,
			},
			wantTitle: "ultraplan: Implement authentication",
			wantBodyParts: []string{
				"## Ultraplan Consolidation",
				"**Objective**: Implement authentication",
				"## Tasks Included",
				"- **task-1**: Add feature X",
			},
		},
		{
			name: "multiple tasks in stacked mode",
			tasks: []consolidation.CompletedTask{
				{
					ID:    "task-1",
					Title: "Add feature X",
				},
				{
					ID:    "task-2",
					Title: "Fix bug Y",
				},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeStacked,
				GroupIndex:  0,
				TotalGroups: 3,
				Objective:   "Major refactoring project",
			},
			wantTitle: "ultraplan: group 1 - Major refactoring project",
			wantBodyParts: []string{
				"## Ultraplan Consolidation",
				"**Objective**: Major refactoring project",
				"**Group**: 1 of 3",
				"must be merged before Group 2",
				"## Tasks Included",
				"- **task-1**: Add feature X",
				"- **task-2**: Fix bug Y",
			},
		},
		{
			name: "middle group in stacked mode",
			tasks: []consolidation.CompletedTask{
				{
					ID:    "task-3",
					Title: "Implement middleware",
				},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeStacked,
				GroupIndex:  1,
				TotalGroups: 3,
				Objective:   "API redesign",
			},
			wantTitle: "ultraplan: group 2 - API redesign",
			wantBodyParts: []string{
				"**Group**: 2 of 3",
				"**Base**: Group 1",
				"must be merged before Group 3",
			},
		},
		{
			name: "last group in stacked mode",
			tasks: []consolidation.CompletedTask{
				{
					ID:    "task-5",
					Title: "Final cleanup",
				},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeStacked,
				GroupIndex:  2,
				TotalGroups: 3,
				Objective:   "Cleanup",
			},
			wantTitle: "ultraplan: group 3 - Cleanup",
			wantBodyParts: []string{
				"**Group**: 3 of 3",
				"**Base**: Group 2",
			},
		},
		{
			name: "tasks with completion summaries",
			tasks: []consolidation.CompletedTask{
				{
					ID:    "task-1",
					Title: "Add auth",
					Completion: &types.TaskCompletionFile{
						Summary: "Implemented JWT authentication with refresh tokens",
					},
				},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeSingle,
				Objective:   "Auth system",
				TotalGroups: 1,
			},
			wantTitle: "ultraplan: Auth system",
			wantBodyParts: []string{
				"- **task-1**: Add auth",
				"Implemented JWT authentication with refresh tokens",
			},
		},
		{
			name: "with files changed",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Update config"},
			},
			opts: consolidation.PRBuildOptions{
				Mode:         consolidation.ModeSingle,
				Objective:    "Config update",
				TotalGroups:  1,
				FilesChanged: []string{"config.go", "config_test.go"},
			},
			wantTitle: "ultraplan: Config update",
			wantBodyParts: []string{
				"## Files Changed",
				"`config.go`",
				"`config_test.go`",
			},
		},
		{
			name: "with synthesis notes",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Add feature"},
			},
			opts: consolidation.PRBuildOptions{
				Mode:            consolidation.ModeSingle,
				Objective:       "Feature",
				TotalGroups:     1,
				SynthesisNotes:  "All tasks integrated cleanly",
				Recommendations: []string{"Consider adding more tests", "Document the API"},
			},
			wantTitle: "ultraplan: Feature",
			wantBodyParts: []string{
				"## Synthesis Review Notes",
				"**Integration Notes**: All tasks integrated cleanly",
				"**Recommendations**:",
				"- Consider adding more tests",
				"- Document the API",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New()
			content, err := b.Build(tt.tasks, tt.opts)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}

			if content.Title != tt.wantTitle {
				t.Errorf("Build() title = %q, want %q", content.Title, tt.wantTitle)
			}

			for _, part := range tt.wantBodyParts {
				if !strings.Contains(content.Body, part) {
					t.Errorf("Build() body missing part %q\nGot body:\n%s", part, content.Body)
				}
			}
		})
	}
}

func TestBuilder_BuildTitle(t *testing.T) {
	tests := []struct {
		name       string
		objective  string
		mode       consolidation.Mode
		groupIndex int
		limit      int
		want       string
	}{
		{
			name:       "single mode short objective",
			objective:  "Add auth",
			mode:       consolidation.ModeSingle,
			groupIndex: 0,
			limit:      50,
			want:       "ultraplan: Add auth",
		},
		{
			name:       "single mode long objective truncated",
			objective:  "This is a very long objective that exceeds the limit and needs to be truncated",
			mode:       consolidation.ModeSingle,
			groupIndex: 0,
			limit:      50,
			want:       "ultraplan: This is a very long objective that exceeds the ...",
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
			name:       "stacked mode fifth group",
			objective:  "Feature Y",
			mode:       consolidation.ModeStacked,
			groupIndex: 4,
			limit:      50,
			want:       "ultraplan: group 5 - Feature Y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := New(WithObjectiveLimit(tt.limit))
			opts := consolidation.PRBuildOptions{
				Mode:       tt.mode,
				GroupIndex: tt.groupIndex,
				Objective:  tt.objective,
			}
			got := b.BuildTitle(nil, opts)
			if got != tt.want {
				t.Errorf("BuildTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuilder_WithOptions(t *testing.T) {
	t.Run("default limits", func(t *testing.T) {
		b := New()
		if b.objectiveLimit != 50 {
			t.Errorf("default objectiveLimit = %d, want 50", b.objectiveLimit)
		}
	})

	t.Run("custom objective limit", func(t *testing.T) {
		b := New(WithObjectiveLimit(100))
		if b.objectiveLimit != 100 {
			t.Errorf("objectiveLimit = %d, want 100", b.objectiveLimit)
		}
	})
}

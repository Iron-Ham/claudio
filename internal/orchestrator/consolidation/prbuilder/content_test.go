package prbuilder

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
)

func TestBuildBody(t *testing.T) {
	tests := []struct {
		name          string
		tasks         []consolidation.CompletedTask
		opts          consolidation.PRBuildOptions
		wantParts     []string
		dontWantParts []string
	}{
		{
			name:  "basic body structure",
			tasks: []consolidation.CompletedTask{},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeSingle,
				Objective:   "Test objective",
				TotalGroups: 1,
			},
			wantParts: []string{
				"## Ultraplan Consolidation",
				"**Objective**: Test objective",
				"## Tasks Included",
			},
		},
		{
			name: "stacked mode with base branch reference",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task one"},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeStacked,
				GroupIndex:  2,
				TotalGroups: 4,
				Objective:   "Multi-group project",
			},
			wantParts: []string{
				"**Group**: 3 of 4",
				"**Base**: Group 2",
				"must be merged before Group 4",
			},
		},
		{
			name: "first group no base reference",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task one"},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeStacked,
				GroupIndex:  0,
				TotalGroups: 3,
				Objective:   "Test",
			},
			wantParts: []string{
				"**Group**: 1 of 3",
				"must be merged before Group 2",
			},
			dontWantParts: []string{
				"**Base**:",
			},
		},
		{
			name: "last group no merge note",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task one"},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeStacked,
				GroupIndex:  2,
				TotalGroups: 3,
				Objective:   "Test",
			},
			wantParts: []string{
				"**Group**: 3 of 3",
				"**Base**: Group 2",
			},
			dontWantParts: []string{
				"must be merged before",
			},
		},
		{
			name: "tasks with completion data",
			tasks: []consolidation.CompletedTask{
				{
					ID:    "task-1",
					Title: "Add auth",
					Completion: &types.TaskCompletionFile{
						Summary: "Added JWT auth with refresh",
						Notes:   "Uses RS256 algorithm",
						Issues:  []string{"Rate limiting not implemented"},
						Suggestions: []string{
							"Consider adding OAuth support",
							"Add session management",
						},
						Dependencies: []string{"go-jwt/v5", "crypto/rand"},
					},
				},
				{
					ID:    "task-2",
					Title: "Add tests",
					Completion: &types.TaskCompletionFile{
						Summary:      "Added test coverage",
						Dependencies: []string{"testify", "go-jwt/v5"}, // go-jwt/v5 should be deduplicated
					},
				},
			},
			opts: consolidation.PRBuildOptions{
				Mode:        consolidation.ModeSingle,
				Objective:   "Auth feature",
				TotalGroups: 1,
			},
			wantParts: []string{
				"Added JWT auth with refresh",
				"## Implementation Notes",
				"Uses RS256 algorithm",
				"## Issues/Concerns Flagged",
				"[task-1] Rate limiting not implemented",
				"## Integration Suggestions",
				"[task-1] Consider adding OAuth support",
				"## New Dependencies",
				"`go-jwt/v5`",
				"`crypto/rand`",
				"`testify`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBody(tt.tasks, tt.opts)

			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("buildBody() missing expected part %q\nGot:\n%s", part, got)
				}
			}

			for _, part := range tt.dontWantParts {
				if strings.Contains(got, part) {
					t.Errorf("buildBody() unexpectedly contains %q\nGot:\n%s", part, got)
				}
			}
		})
	}
}

func TestBuildAggregatedContext(t *testing.T) {
	tests := []struct {
		name      string
		tasks     []consolidation.CompletedTask
		wantParts []string
		wantEmpty bool
	}{
		{
			name:      "empty tasks",
			tasks:     []consolidation.CompletedTask{},
			wantEmpty: true,
		},
		{
			name: "tasks without completion data",
			tasks: []consolidation.CompletedTask{
				{ID: "task-1", Title: "Task 1"},
				{ID: "task-2", Title: "Task 2"},
			},
			wantEmpty: true,
		},
		{
			name: "tasks with empty completion data",
			tasks: []consolidation.CompletedTask{
				{
					ID:         "task-1",
					Completion: &types.TaskCompletionFile{},
				},
			},
			wantEmpty: true,
		},
		{
			name: "deduplicates dependencies",
			tasks: []consolidation.CompletedTask{
				{
					ID: "task-1",
					Completion: &types.TaskCompletionFile{
						Dependencies: []string{"pkg-a", "pkg-b"},
					},
				},
				{
					ID: "task-2",
					Completion: &types.TaskCompletionFile{
						Dependencies: []string{"pkg-b", "pkg-c"}, // pkg-b should be deduplicated
					},
				},
			},
			wantParts: []string{
				"`pkg-a`",
				"`pkg-b`",
				"`pkg-c`",
			},
		},
		{
			name: "filters empty strings",
			tasks: []consolidation.CompletedTask{
				{
					ID: "task-1",
					Completion: &types.TaskCompletionFile{
						Issues:       []string{"real issue", ""},
						Suggestions:  []string{"", "real suggestion"},
						Dependencies: []string{"", "real-dep", ""},
					},
				},
			},
			wantParts: []string{
				"real issue",
				"real suggestion",
				"`real-dep`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAggregatedContext(tt.tasks)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("buildAggregatedContext() = %q, want empty string", got)
				}
				return
			}

			for _, part := range tt.wantParts {
				if !strings.Contains(got, part) {
					t.Errorf("buildAggregatedContext() missing %q\nGot:\n%s", part, got)
				}
			}

			// Check that pkg-b only appears once (deduplication)
			if tt.name == "deduplicates dependencies" {
				count := strings.Count(got, "`pkg-b`")
				if count != 1 {
					t.Errorf("buildAggregatedContext() has pkg-b %d times, want 1", count)
				}
			}
		})
	}
}

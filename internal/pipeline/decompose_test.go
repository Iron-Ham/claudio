package pipeline

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func TestDecompose_NilPlan(t *testing.T) {
	_, err := Decompose(nil, DecomposeConfig{})
	if err == nil {
		t.Fatal("expected error for nil plan")
	}
	if !strings.Contains(err.Error(), "plan is required") {
		t.Errorf("error = %q, want containing 'plan is required'", err)
	}
}

func TestDecompose_EmptyPlan(t *testing.T) {
	plan := &ultraplan.PlanSpec{ID: "p1", Tasks: nil}
	_, err := Decompose(plan, DecomposeConfig{})
	if err == nil {
		t.Fatal("expected error for empty plan")
	}
	if !strings.Contains(err.Error(), "no tasks") {
		t.Errorf("error = %q, want containing 'no tasks'", err)
	}
}

func TestDecompose_SingleTask(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if len(result.ExecutionTeams) != 1 {
		t.Fatalf("ExecutionTeams = %d, want 1", len(result.ExecutionTeams))
	}
	if result.ExecutionTeams[0].ID != "exec-0" {
		t.Errorf("team ID = %q, want %q", result.ExecutionTeams[0].ID, "exec-0")
	}
	if len(result.ExecutionTeams[0].Tasks) != 1 {
		t.Errorf("tasks = %d, want 1", len(result.ExecutionTeams[0].Tasks))
	}
	if result.PlanningTeam != nil {
		t.Error("PlanningTeam should be nil")
	}
	if result.ReviewTeam != nil {
		t.Error("ReviewTeam should be nil")
	}
	if result.ConsolidationTeam != nil {
		t.Error("ConsolidationTeam should be nil")
	}
}

func TestDecompose_DisjointTasks(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"b.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"c.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if len(result.ExecutionTeams) != 3 {
		t.Fatalf("ExecutionTeams = %d, want 3", len(result.ExecutionTeams))
	}

	// Each team should have exactly 1 task.
	for i, team := range result.ExecutionTeams {
		if len(team.Tasks) != 1 {
			t.Errorf("team %d tasks = %d, want 1", i, len(team.Tasks))
		}
	}
}

func TestDecompose_SharedFiles(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"shared.go", "a.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"shared.go", "b.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"c.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// t1 and t2 share "shared.go" → one team. t3 is separate → second team.
	if len(result.ExecutionTeams) != 2 {
		t.Fatalf("ExecutionTeams = %d, want 2", len(result.ExecutionTeams))
	}

	// Find the team with 2 tasks.
	var twoTaskTeam *team.Spec
	for i := range result.ExecutionTeams {
		if len(result.ExecutionTeams[i].Tasks) == 2 {
			twoTaskTeam = &result.ExecutionTeams[i]
			break
		}
	}
	if twoTaskTeam == nil {
		t.Fatal("expected one team with 2 tasks")
	}

	ids := make(map[string]bool)
	for _, task := range twoTaskTeam.Tasks {
		ids[task.ID] = true
	}
	if !ids["t1"] || !ids["t2"] {
		t.Errorf("shared team should contain t1 and t2, got %v", ids)
	}
}

func TestDecompose_TransitiveAffinity(t *testing.T) {
	// t1 shares "x.go" with t2; t2 shares "y.go" with t3.
	// All three should be in the same team.
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"x.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"x.go", "y.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"y.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if len(result.ExecutionTeams) != 1 {
		t.Fatalf("ExecutionTeams = %d, want 1 (transitive)", len(result.ExecutionTeams))
	}
	if len(result.ExecutionTeams[0].Tasks) != 3 {
		t.Errorf("tasks = %d, want 3", len(result.ExecutionTeams[0].Tasks))
	}
}

func TestDecompose_NoFiles(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1"}, // no files
			{ID: "t2", Title: "Task 2"}, // no files
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// Each task has no files so can't be grouped → each in its own team.
	if len(result.ExecutionTeams) != 2 {
		t.Fatalf("ExecutionTeams = %d, want 2", len(result.ExecutionTeams))
	}
}

func TestDecompose_MaxTeamSize(t *testing.T) {
	// 3 tasks sharing the same file, max team size = 2.
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"shared.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"shared.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"shared.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{MaxTeamSize: 2})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// Should be split into 2 teams: one with 2, one with 1.
	if len(result.ExecutionTeams) != 2 {
		t.Fatalf("ExecutionTeams = %d, want 2", len(result.ExecutionTeams))
	}

	totalTasks := 0
	for _, team := range result.ExecutionTeams {
		totalTasks += len(team.Tasks)
		if len(team.Tasks) > 2 {
			t.Errorf("team %q has %d tasks, exceeds max of 2", team.ID, len(team.Tasks))
		}
	}
	if totalTasks != 3 {
		t.Errorf("total tasks = %d, want 3", totalTasks)
	}
}

func TestDecompose_MinTeamSize(t *testing.T) {
	// t1 and t2 share "a.go"; t3 has "b.go" (disjoint but shares "c.go" with t1).
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go", "c.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"a.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"b.go", "c.go"}},
			{ID: "t4", Title: "Task 4", Files: []string{"d.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{MinTeamSize: 2})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// t1, t2, t3 all connected via union-find (t1-t2 via a.go, t1-t3 via c.go).
	// t4 is alone and undersized (1 < 2), but has no shared files with others,
	// so it stays alone. So we should have 2 teams.
	// Actually: t4 has d.go, no overlap. Since minSize merge only happens when
	// there's a shared file, t4 stays undersized.
	totalTasks := 0
	for _, team := range result.ExecutionTeams {
		totalTasks += len(team.Tasks)
	}
	if totalTasks != 4 {
		t.Fatalf("total tasks = %d, want 4", totalTasks)
	}
}

func TestDecompose_DeterministicOutput(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "zz-task", Title: "Last", Files: []string{"z.go"}},
			{ID: "aa-task", Title: "First", Files: []string{"a.go"}},
			{ID: "mm-task", Title: "Middle", Files: []string{"m.go"}},
		},
	}

	// Run decompose multiple times and verify deterministic ordering.
	var lastIDs []string
	for i := range 5 {
		result, err := Decompose(plan, DecomposeConfig{})
		if err != nil {
			t.Fatalf("Decompose: %v", err)
		}

		ids := make([]string, len(result.ExecutionTeams))
		for j, team := range result.ExecutionTeams {
			ids[j] = team.Tasks[0].ID
		}

		if lastIDs != nil {
			for j := range ids {
				if ids[j] != lastIDs[j] {
					t.Fatalf("run %d: non-deterministic order: %v vs %v", i, ids, lastIDs)
				}
			}
		}
		lastIDs = ids
	}

	// Verify sorted order (aa, mm, zz).
	if lastIDs[0] != "aa-task" || lastIDs[1] != "mm-task" || lastIDs[2] != "zz-task" {
		t.Errorf("expected sorted order, got %v", lastIDs)
	}
}

func TestDecompose_TeamSpecFields(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	spec := result.ExecutionTeams[0]
	if spec.Role != team.RoleExecution {
		t.Errorf("Role = %v, want %v", spec.Role, team.RoleExecution)
	}
	if spec.TeamSize != 1 {
		t.Errorf("TeamSize = %d, want 1", spec.TeamSize)
	}
	if spec.ID != "exec-0" {
		t.Errorf("ID = %q, want %q", spec.ID, "exec-0")
	}
}

func TestDecompose_WithPlanningTeam(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "p1",
		Objective: "Build a widget",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1"},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{PlanningTeam: true})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if result.PlanningTeam == nil {
		t.Fatal("PlanningTeam should not be nil")
	}
	if result.PlanningTeam.ID != "planning" {
		t.Errorf("ID = %q, want %q", result.PlanningTeam.ID, "planning")
	}
	if result.PlanningTeam.Role != team.RolePlanning {
		t.Errorf("Role = %v, want %v", result.PlanningTeam.Role, team.RolePlanning)
	}
	if len(result.PlanningTeam.Tasks) != 1 {
		t.Errorf("Tasks = %d, want 1", len(result.PlanningTeam.Tasks))
	}
	if result.PlanningTeam.Tasks[0].ID != "meta-planning" {
		t.Errorf("task ID = %q, want %q", result.PlanningTeam.Tasks[0].ID, "meta-planning")
	}
}

func TestDecompose_WithReviewTeam(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "p1",
		Objective: "Build a widget",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1"},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{ReviewTeam: true})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if result.ReviewTeam == nil {
		t.Fatal("ReviewTeam should not be nil")
	}
	if result.ReviewTeam.ID != "review" {
		t.Errorf("ID = %q, want %q", result.ReviewTeam.ID, "review")
	}
	if result.ReviewTeam.Role != team.RoleReview {
		t.Errorf("Role = %v, want %v", result.ReviewTeam.Role, team.RoleReview)
	}
}

func TestDecompose_WithConsolidationTeam(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "p1",
		Objective: "Build a widget",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1"},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{ConsolidationTeam: true})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if result.ConsolidationTeam == nil {
		t.Fatal("ConsolidationTeam should not be nil")
	}
	if result.ConsolidationTeam.ID != "consolidation" {
		t.Errorf("ID = %q, want %q", result.ConsolidationTeam.ID, "consolidation")
	}
	if result.ConsolidationTeam.Role != team.RoleConsolidation {
		t.Errorf("Role = %v, want %v", result.ConsolidationTeam.Role, team.RoleConsolidation)
	}
}

func TestDecompose_AllOptionalTeams(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID:        "p1",
		Objective: "Build a widget",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1"},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{
		PlanningTeam:      true,
		ReviewTeam:        true,
		ConsolidationTeam: true,
	})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if result.PlanningTeam == nil {
		t.Error("PlanningTeam should not be nil")
	}
	if result.ReviewTeam == nil {
		t.Error("ReviewTeam should not be nil")
	}
	if result.ConsolidationTeam == nil {
		t.Error("ConsolidationTeam should not be nil")
	}
}

func TestDecompose_DefaultTeamSizeApplied(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"a.go"}},
			{ID: "t3", Title: "Task 3", Files: []string{"a.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{DefaultTeamSize: 4})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// All 3 tasks share a.go → one team with 3 tasks.
	// DefaultTeamSize=4 but only 3 tasks, so TeamSize = min(3, 4) = 3.
	spec := result.ExecutionTeams[0]
	if spec.TeamSize != 3 {
		t.Errorf("TeamSize = %d, want 3 (clamped to task count)", spec.TeamSize)
	}
}

func TestDecompose_DefaultTeamSizeClamped(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{DefaultTeamSize: 5})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	// Only 1 task, so TeamSize clamped to 1.
	if result.ExecutionTeams[0].TeamSize != 1 {
		t.Errorf("TeamSize = %d, want 1", result.ExecutionTeams[0].TeamSize)
	}
}

func TestDecompose_ScalingBoundsPopulated(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
			{ID: "t2", Title: "Task 2", Files: []string{"b.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{
		MinTeamInstances: 2,
		MaxTeamInstances: 8,
	})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	for _, spec := range result.ExecutionTeams {
		if spec.MinInstances != 2 {
			t.Errorf("team %s: MinInstances = %d, want 2", spec.ID, spec.MinInstances)
		}
		if spec.MaxInstances != 8 {
			t.Errorf("team %s: MaxInstances = %d, want 8", spec.ID, spec.MaxInstances)
		}
	}
}

func TestDecompose_DefaultTeamSizeZeroDefaultsToOne(t *testing.T) {
	plan := &ultraplan.PlanSpec{
		ID: "p1",
		Tasks: []ultraplan.PlannedTask{
			{ID: "t1", Title: "Task 1", Files: []string{"a.go"}},
		},
	}

	result, err := Decompose(plan, DecomposeConfig{})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}

	if result.ExecutionTeams[0].TeamSize != 1 {
		t.Errorf("TeamSize = %d, want 1 (default)", result.ExecutionTeams[0].TeamSize)
	}
}

// -- Union-Find unit tests ---------------------------------------------------

func TestUnionFind_BasicOperations(t *testing.T) {
	uf := newUnionFind([]string{"a", "b", "c", "d"})

	// Initially, 4 separate components.
	comps := uf.Components()
	if len(comps) != 4 {
		t.Fatalf("components = %d, want 4", len(comps))
	}

	// Union a and b.
	uf.Union("a", "b")
	comps = uf.Components()
	if len(comps) != 3 {
		t.Fatalf("after union(a,b): components = %d, want 3", len(comps))
	}

	// Union b and c (transitive: a-b-c should be one component).
	uf.Union("b", "c")
	comps = uf.Components()
	if len(comps) != 2 {
		t.Fatalf("after union(b,c): components = %d, want 2", len(comps))
	}

	// a, b, c should have the same root.
	if uf.Find("a") != uf.Find("b") || uf.Find("b") != uf.Find("c") {
		t.Error("a, b, c should be in the same component")
	}

	// d should be separate.
	if uf.Find("d") == uf.Find("a") {
		t.Error("d should be in a separate component")
	}
}

func TestUnionFind_Idempotent(t *testing.T) {
	uf := newUnionFind([]string{"a", "b"})
	uf.Union("a", "b")
	uf.Union("a", "b") // second union should be no-op

	comps := uf.Components()
	if len(comps) != 1 {
		t.Fatalf("components = %d, want 1", len(comps))
	}
}

func TestUnionFind_SingleElement(t *testing.T) {
	uf := newUnionFind([]string{"only"})
	comps := uf.Components()
	if len(comps) != 1 {
		t.Fatalf("components = %d, want 1", len(comps))
	}
	if uf.Find("only") != "only" {
		t.Errorf("Find(only) = %q, want %q", uf.Find("only"), "only")
	}
}

package pipeline

import (
	"errors"
	"fmt"
	"sort"

	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// Decompose takes a PlanSpec and produces team.Specs grouped by file affinity.
//
// Tasks that share at least one file are placed in the same team. Tasks with
// no files are placed in their own single-task team. The result includes
// optional planning, review, and consolidation team specs based on the config.
func Decompose(plan *ultraplan.PlanSpec, cfg DecomposeConfig) (*DecomposeResult, error) {
	if plan == nil {
		return nil, errors.New("pipeline: plan is required")
	}
	if len(plan.Tasks) == 0 {
		return nil, errors.New("pipeline: plan has no tasks")
	}

	cfg = cfg.defaults()

	groups := groupByFileAffinity(plan.Tasks)

	// Apply MaxTeamSize: split oversized groups.
	if cfg.MaxTeamSize > 0 {
		groups = splitOversized(groups, cfg.MaxTeamSize)
	}

	// Apply MinTeamSize: merge undersized groups.
	if cfg.MinTeamSize > 1 {
		groups = mergeUndersized(groups, plan.Tasks, cfg.MinTeamSize)
	}

	// Sort groups deterministically by first task ID in each group.
	sort.Slice(groups, func(i, j int) bool {
		return groups[i][0] < groups[j][0]
	})

	// Build task index for lookup.
	taskIndex := make(map[string]ultraplan.PlannedTask, len(plan.Tasks))
	for _, t := range plan.Tasks {
		taskIndex[t.ID] = t
	}

	// Convert groups into team specs.
	execTeams := make([]team.Spec, 0, len(groups))
	for i, group := range groups {
		tasks := make([]ultraplan.PlannedTask, 0, len(group))
		for _, id := range group {
			if t, ok := taskIndex[id]; ok {
				tasks = append(tasks, t)
			}
		}

		// Teams shouldn't have more concurrent workers than tasks.
		teamSize := cfg.DefaultTeamSize
		if teamSize > len(tasks) {
			teamSize = len(tasks)
		}

		execTeams = append(execTeams, team.Spec{
			ID:           fmt.Sprintf("exec-%d", i),
			Name:         fmt.Sprintf("Execution Team %d", i),
			Role:         team.RoleExecution,
			Tasks:        tasks,
			TeamSize:     teamSize,
			MinInstances: cfg.MinTeamInstances,
			MaxInstances: cfg.MaxTeamInstances,
		})
	}

	result := &DecomposeResult{
		ExecutionTeams: execTeams,
	}

	if cfg.PlanningTeam {
		result.PlanningTeam = makePlanningTeam(plan)
	}
	if cfg.ReviewTeam {
		result.ReviewTeam = makeReviewTeam(plan)
	}
	if cfg.ConsolidationTeam {
		result.ConsolidationTeam = makeConsolidationTeam(plan)
	}

	return result, nil
}

// groupByFileAffinity groups tasks by shared files using union-find.
// Returns a slice of groups, each group being a sorted slice of task IDs.
func groupByFileAffinity(tasks []ultraplan.PlannedTask) [][]string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}

	uf := newUnionFind(ids)

	// Build file → task ID index.
	fileToTasks := make(map[string][]string)
	for _, t := range tasks {
		for _, f := range t.Files {
			fileToTasks[f] = append(fileToTasks[f], t.ID)
		}
	}

	// Union tasks that share files.
	for _, taskIDs := range fileToTasks {
		for i := 1; i < len(taskIDs); i++ {
			uf.Union(taskIDs[0], taskIDs[i])
		}
	}

	components := uf.Components()

	// Sort task IDs within each component.
	groups := make([][]string, 0, len(components))
	for _, members := range components {
		sort.Strings(members)
		groups = append(groups, members)
	}
	return groups
}

// splitOversized splits groups that exceed maxSize by task priority.
func splitOversized(groups [][]string, maxSize int) [][]string {
	var result [][]string
	for _, g := range groups {
		if len(g) <= maxSize {
			result = append(result, g)
			continue
		}
		// Split into chunks of maxSize.
		for i := 0; i < len(g); i += maxSize {
			end := min(i+maxSize, len(g))
			chunk := make([]string, end-i)
			copy(chunk, g[i:end])
			result = append(result, chunk)
		}
	}
	return result
}

// mergeUndersized merges groups smaller than minSize into the nearest
// neighbor by shared file count. Groups that cannot be merged (no shared
// files with any other group) are left as-is.
func mergeUndersized(groups [][]string, tasks []ultraplan.PlannedTask, minSize int) [][]string {
	if len(groups) <= 1 {
		return groups
	}

	// Build task → files index.
	taskFiles := make(map[string]map[string]bool)
	for _, t := range tasks {
		s := make(map[string]bool, len(t.Files))
		for _, f := range t.Files {
			s[f] = true
		}
		taskFiles[t.ID] = s
	}

	// groupFiles returns the set of files touched by a group.
	groupFiles := func(group []string) map[string]bool {
		files := make(map[string]bool)
		for _, id := range group {
			for f := range taskFiles[id] {
				files[f] = true
			}
		}
		return files
	}

	// sharedFileCount returns the number of shared files between two groups.
	sharedFileCount := func(a, b map[string]bool) int {
		count := 0
		for f := range a {
			if b[f] {
				count++
			}
		}
		return count
	}

	// Iteratively merge undersized groups.
	merged := make([][]string, len(groups))
	copy(merged, groups)

	changed := true
	for changed {
		changed = false
		for i := 0; i < len(merged); i++ {
			if len(merged[i]) >= minSize {
				continue
			}
			// Find the best merge candidate.
			bestJ := -1
			bestShared := 0
			filesI := groupFiles(merged[i])
			for j := 0; j < len(merged); j++ {
				if i == j {
					continue
				}
				filesJ := groupFiles(merged[j])
				shared := sharedFileCount(filesI, filesJ)
				if shared > bestShared {
					bestShared = shared
					bestJ = j
				}
			}
			if bestJ == -1 {
				continue // no merge candidate
			}
			// Merge i into bestJ.
			merged[bestJ] = append(merged[bestJ], merged[i]...)
			sort.Strings(merged[bestJ])
			merged = append(merged[:i], merged[i+1:]...)
			changed = true
			break // restart scan after mutation
		}
	}
	return merged
}

// makePlanningTeam creates a planning team spec covering all tasks.
func makePlanningTeam(plan *ultraplan.PlanSpec) *team.Spec {
	return &team.Spec{
		ID:         "planning",
		Name:       "Planning Team",
		Role:       team.RolePlanning,
		Tasks:      []ultraplan.PlannedTask{planningMetaTask(plan)},
		LeadPrompt: fmt.Sprintf("Plan the execution of: %s", plan.Objective),
		TeamSize:   1,
	}
}

// makeReviewTeam creates a review team spec that depends on all execution teams.
func makeReviewTeam(plan *ultraplan.PlanSpec) *team.Spec {
	return &team.Spec{
		ID:         "review",
		Name:       "Review Team",
		Role:       team.RoleReview,
		Tasks:      []ultraplan.PlannedTask{reviewMetaTask(plan)},
		LeadPrompt: fmt.Sprintf("Review the execution results of: %s", plan.Objective),
		TeamSize:   1,
	}
}

// makeConsolidationTeam creates a consolidation team spec.
func makeConsolidationTeam(plan *ultraplan.PlanSpec) *team.Spec {
	return &team.Spec{
		ID:         "consolidation",
		Name:       "Consolidation Team",
		Role:       team.RoleConsolidation,
		Tasks:      []ultraplan.PlannedTask{consolidationMetaTask(plan)},
		LeadPrompt: fmt.Sprintf("Consolidate results from: %s", plan.Objective),
		TeamSize:   1,
	}
}

// planningMetaTask creates a synthetic task for the planning phase.
func planningMetaTask(plan *ultraplan.PlanSpec) ultraplan.PlannedTask {
	return ultraplan.PlannedTask{
		ID:    "meta-planning",
		Title: "Collaborative Planning",
		Description: fmt.Sprintf(
			"Plan the decomposition and execution strategy for: %s (%d tasks)",
			plan.Objective, len(plan.Tasks),
		),
	}
}

// reviewMetaTask creates a synthetic task for the review phase.
func reviewMetaTask(plan *ultraplan.PlanSpec) ultraplan.PlannedTask {
	return ultraplan.PlannedTask{
		ID:    "meta-review",
		Title: "Cross-Cutting Review",
		Description: fmt.Sprintf(
			"Review all execution results for integration issues: %s (%d tasks)",
			plan.Objective, len(plan.Tasks),
		),
	}
}

// consolidationMetaTask creates a synthetic task for the consolidation phase.
func consolidationMetaTask(plan *ultraplan.PlanSpec) ultraplan.PlannedTask {
	return ultraplan.PlannedTask{
		ID:    "meta-consolidation",
		Title: "Parallel Merge Consolidation",
		Description: fmt.Sprintf(
			"Consolidate and merge results from: %s (%d tasks)",
			plan.Objective, len(plan.Tasks),
		),
	}
}

// -- Union-Find implementation -----------------------------------------------

type unionFind struct {
	parent map[string]string
	rank   map[string]int
}

func newUnionFind(keys []string) *unionFind {
	uf := &unionFind{
		parent: make(map[string]string, len(keys)),
		rank:   make(map[string]int, len(keys)),
	}
	for _, k := range keys {
		uf.parent[k] = k
	}
	return uf
}

func (uf *unionFind) Find(x string) string {
	if uf.parent[x] != x {
		uf.parent[x] = uf.Find(uf.parent[x]) // path compression
	}
	return uf.parent[x]
}

func (uf *unionFind) Union(x, y string) {
	rx, ry := uf.Find(x), uf.Find(y)
	if rx == ry {
		return
	}
	// Union by rank.
	switch {
	case uf.rank[rx] < uf.rank[ry]:
		uf.parent[rx] = ry
	case uf.rank[rx] > uf.rank[ry]:
		uf.parent[ry] = rx
	default:
		uf.parent[ry] = rx
		uf.rank[rx]++
	}
}

// Components returns the connected components: root → sorted member list.
func (uf *unionFind) Components() map[string][]string {
	comps := make(map[string][]string)
	for k := range uf.parent {
		root := uf.Find(k)
		comps[root] = append(comps[root], k)
	}
	return comps
}

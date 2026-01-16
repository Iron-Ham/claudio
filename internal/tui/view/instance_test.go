package view

import (
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestCalculateOverheadLines(t *testing.T) {
	tests := []struct {
		name     string
		params   OverheadParams
		expected int
	}{
		{
			name: "minimal instance - not running",
			params: OverheadParams{
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Empty banner (1) = 3
			expected: 3,
		},
		{
			name: "running instance with scroll indicator",
			params: OverheadParams{
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          true,
				HasSearchActive:    false,
				HasScrollIndicator: true,
			},
			// Header (2) + Banner (2) + Scroll (2) = 6
			expected: 6,
		},
		{
			name: "instance with dependencies",
			params: OverheadParams{
				HasDependencies:    true,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Dependencies (1) + Empty banner (1) = 4
			expected: 4,
		},
		{
			name: "instance with dependents",
			params: OverheadParams{
				HasDependencies:    false,
				HasDependents:      true,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Dependents (1) + Empty banner (1) = 4
			expected: 4,
		},
		{
			name: "instance with both dependencies and dependents",
			params: OverheadParams{
				HasDependencies:    true,
				HasDependents:      true,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Dependencies (1) + Dependents (1) + Empty banner (1) = 5
			expected: 5,
		},
		{
			name: "instance with metrics enabled and available",
			params: OverheadParams{
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        true,
				HasMetrics:         true,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Metrics (2) + Empty banner (1) = 5
			expected: 5,
		},
		{
			name: "instance with metrics enabled but no data",
			params: OverheadParams{
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        true,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    false,
				HasScrollIndicator: false,
			},
			// Header (2) + Empty banner (1) = 3
			expected: 3,
		},
		{
			name: "instance with search active",
			params: OverheadParams{
				HasDependencies:    false,
				HasDependents:      false,
				ShowMetrics:        false,
				HasMetrics:         false,
				IsRunning:          false,
				HasSearchActive:    true,
				HasScrollIndicator: false,
			},
			// Header (2) + Empty banner (1) + Search (2) = 5
			expected: 5,
		},
		{
			name: "maximum overhead - everything enabled",
			params: OverheadParams{
				HasDependencies:    true,
				HasDependents:      true,
				ShowMetrics:        true,
				HasMetrics:         true,
				IsRunning:          true,
				HasSearchActive:    true,
				HasScrollIndicator: true,
			},
			// Header (2) + Deps (1) + Dependents (1) + Metrics (2) + Banner (2) + Scroll (2) + Search (2) = 12
			expected: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewInstanceView(80, 20) // Dimensions don't affect overhead calculation
			result := v.CalculateOverheadLines(tt.params)
			if result != tt.expected {
				t.Errorf("CalculateOverheadLines() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCalculateOverheadLinesConsistency(t *testing.T) {
	// This test ensures that increasing complexity only increases overhead
	// (i.e., the function is monotonic with respect to added features)

	v := NewInstanceView(80, 20)

	baseParams := OverheadParams{
		HasDependencies:    false,
		HasDependents:      false,
		ShowMetrics:        false,
		HasMetrics:         false,
		IsRunning:          false,
		HasSearchActive:    false,
		HasScrollIndicator: false,
	}
	baseOverhead := v.CalculateOverheadLines(baseParams)

	// Adding dependencies should increase overhead
	withDeps := baseParams
	withDeps.HasDependencies = true
	depsOverhead := v.CalculateOverheadLines(withDeps)
	if depsOverhead <= baseOverhead {
		t.Errorf("Adding dependencies should increase overhead: base=%d, withDeps=%d", baseOverhead, depsOverhead)
	}

	// Adding running status should increase overhead
	withRunning := baseParams
	withRunning.IsRunning = true
	runningOverhead := v.CalculateOverheadLines(withRunning)
	if runningOverhead <= baseOverhead {
		t.Errorf("Adding running status should increase overhead: base=%d, withRunning=%d", baseOverhead, runningOverhead)
	}

	// Adding scroll indicator should increase overhead
	withScroll := baseParams
	withScroll.HasScrollIndicator = true
	scrollOverhead := v.CalculateOverheadLines(withScroll)
	if scrollOverhead <= baseOverhead {
		t.Errorf("Adding scroll indicator should increase overhead: base=%d, withScroll=%d", baseOverhead, scrollOverhead)
	}

	// Adding search should increase overhead
	withSearch := baseParams
	withSearch.HasSearchActive = true
	searchOverhead := v.CalculateOverheadLines(withSearch)
	if searchOverhead <= baseOverhead {
		t.Errorf("Adding search should increase overhead: base=%d, withSearch=%d", baseOverhead, searchOverhead)
	}
}

func TestOverheadAtLeastMinimum(t *testing.T) {
	// Ensure overhead is always at least a reasonable minimum
	// (header + empty banner)
	minExpectedOverhead := 3

	v := NewInstanceView(80, 20)

	// Even with all features disabled, should have minimum overhead
	params := OverheadParams{}
	result := v.CalculateOverheadLines(params)
	if result < minExpectedOverhead {
		t.Errorf("Minimum overhead should be at least %d, got %d", minExpectedOverhead, result)
	}
}

func TestCalculateOverheadLinesWithGroupHeader(t *testing.T) {
	tests := []struct {
		name     string
		params   OverheadParams
		expected int
	}{
		{
			name: "instance with group header only",
			params: OverheadParams{
				HasGroupHeader: true,
			},
			// Header (2) + Group header (2) + Empty banner (1) = 5
			expected: 5,
		},
		{
			name: "instance with group header and dependencies",
			params: OverheadParams{
				HasGroupHeader:       true,
				HasGroupDependencies: true,
			},
			// Header (2) + Group header (2) + Dep status (1) + Empty banner (1) = 6
			expected: 6,
		},
		{
			name: "instance with group header and siblings",
			params: OverheadParams{
				HasGroupHeader: true,
				HasSiblings:    true,
			},
			// Header (2) + Group header (2) + Sibling line (1) + Empty banner (1) = 6
			expected: 6,
		},
		{
			name: "instance with group header, dependencies, and siblings",
			params: OverheadParams{
				HasGroupHeader:       true,
				HasGroupDependencies: true,
				HasSiblings:          true,
			},
			// Header (2) + Group header (2) + Dep status (1) + Siblings (1) + Empty banner (1) = 7
			expected: 7,
		},
		{
			name: "maximum overhead with group features",
			params: OverheadParams{
				HasGroupHeader:       true,
				HasGroupDependencies: true,
				HasSiblings:          true,
				HasDependencies:      true,
				HasDependents:        true,
				ShowMetrics:          true,
				HasMetrics:           true,
				IsRunning:            true,
				HasSearchActive:      true,
				HasScrollIndicator:   true,
			},
			// Header (2) + Group header (4) + Deps (1) + Dependents (1) + Metrics (2) + Banner (2) + Scroll (2) + Search (2) = 16
			expected: 16,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewInstanceView(80, 20)
			result := v.CalculateOverheadLines(tt.params)
			if result != tt.expected {
				t.Errorf("CalculateOverheadLines() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestBuildGroupContext(t *testing.T) {
	v := NewInstanceView(80, 20)

	t.Run("returns nil for nil session", func(t *testing.T) {
		inst := &orchestrator.Instance{ID: "inst1"}
		ctx := v.BuildGroupContext(inst, nil)
		if ctx != nil {
			t.Errorf("Expected nil context for nil session")
		}
	})

	t.Run("returns nil for session without groups", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{{ID: "inst1"}},
			Groups:    nil,
		}
		ctx := v.BuildGroupContext(session.Instances[0], session)
		if ctx != nil {
			t.Errorf("Expected nil context for session without groups")
		}
	})

	t.Run("returns nil for instance not in any group", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{{ID: "inst1"}},
			Groups: []*orchestrator.InstanceGroup{
				{ID: "g1", Instances: []string{"inst2"}},
			},
		}
		ctx := v.BuildGroupContext(session.Instances[0], session)
		if ctx != nil {
			t.Errorf("Expected nil context for instance not in group")
		}
	})

	t.Run("builds context for instance in group", func(t *testing.T) {
		inst1 := &orchestrator.Instance{ID: "inst1", Task: "Task 1"}
		inst2 := &orchestrator.Instance{ID: "inst2", Task: "Task 2"}
		inst3 := &orchestrator.Instance{ID: "inst3", Task: "Task 3"}

		group := &orchestrator.InstanceGroup{
			ID:        "g1",
			Name:      "Group 1: Setup",
			Phase:     orchestrator.GroupPhaseExecuting,
			Instances: []string{"inst1", "inst2", "inst3"},
		}

		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{inst1, inst2, inst3},
			Groups:    []*orchestrator.InstanceGroup{group},
		}

		ctx := v.BuildGroupContext(inst2, session)
		if ctx == nil {
			t.Fatal("Expected non-nil context")
		}
		if ctx.Group.ID != "g1" {
			t.Errorf("Expected group ID 'g1', got '%s'", ctx.Group.ID)
		}
		if ctx.TaskIndex != 2 {
			t.Errorf("Expected TaskIndex 2, got %d", ctx.TaskIndex)
		}
		if ctx.TotalTasks != 3 {
			t.Errorf("Expected TotalTasks 3, got %d", ctx.TotalTasks)
		}
		if len(ctx.SiblingTasks) != 2 {
			t.Errorf("Expected 2 siblings, got %d", len(ctx.SiblingTasks))
		}
		if !ctx.AllDependenciesMet {
			t.Error("Expected AllDependenciesMet to be true (no dependencies)")
		}
	})

	t.Run("detects pending dependencies", func(t *testing.T) {
		depInst := &orchestrator.Instance{ID: "dep1", Task: "Dep Task", Status: orchestrator.StatusWorking}
		inst1 := &orchestrator.Instance{ID: "inst1", Task: "Task 1"}

		depGroup := &orchestrator.InstanceGroup{
			ID:        "dep-group",
			Name:      "Dependency Group",
			Phase:     orchestrator.GroupPhaseExecuting,
			Instances: []string{"dep1"},
		}

		mainGroup := &orchestrator.InstanceGroup{
			ID:        "main-group",
			Name:      "Main Group",
			Phase:     orchestrator.GroupPhasePending,
			Instances: []string{"inst1"},
			DependsOn: []string{"dep-group"},
		}

		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{depInst, inst1},
			Groups:    []*orchestrator.InstanceGroup{depGroup, mainGroup},
		}

		ctx := v.BuildGroupContext(inst1, session)
		if ctx == nil {
			t.Fatal("Expected non-nil context")
		}
		if ctx.AllDependenciesMet {
			t.Error("Expected AllDependenciesMet to be false")
		}
		if len(ctx.DependencyGroups) != 1 {
			t.Errorf("Expected 1 dependency group, got %d", len(ctx.DependencyGroups))
		}
		if len(ctx.PendingDependencies) != 1 {
			t.Errorf("Expected 1 pending dependency, got %d", len(ctx.PendingDependencies))
		}
	})

	t.Run("all dependencies met when groups complete", func(t *testing.T) {
		depInst := &orchestrator.Instance{ID: "dep1", Task: "Dep Task", Status: orchestrator.StatusCompleted}
		inst1 := &orchestrator.Instance{ID: "inst1", Task: "Task 1"}

		depGroup := &orchestrator.InstanceGroup{
			ID:        "dep-group",
			Name:      "Dependency Group",
			Phase:     orchestrator.GroupPhaseCompleted,
			Instances: []string{"dep1"},
		}

		mainGroup := &orchestrator.InstanceGroup{
			ID:        "main-group",
			Name:      "Main Group",
			Phase:     orchestrator.GroupPhaseExecuting,
			Instances: []string{"inst1"},
			DependsOn: []string{"dep-group"},
		}

		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{depInst, inst1},
			Groups:    []*orchestrator.InstanceGroup{depGroup, mainGroup},
		}

		ctx := v.BuildGroupContext(inst1, session)
		if ctx == nil {
			t.Fatal("Expected non-nil context")
		}
		if !ctx.AllDependenciesMet {
			t.Error("Expected AllDependenciesMet to be true")
		}
		if len(ctx.DependencyGroups) != 1 {
			t.Errorf("Expected 1 dependency group, got %d", len(ctx.DependencyGroups))
		}
		if len(ctx.PendingDependencies) != 0 {
			t.Errorf("Expected 0 pending dependencies, got %d", len(ctx.PendingDependencies))
		}
	})
}

func TestRenderGroupStatusHeader(t *testing.T) {
	v := NewInstanceView(80, 20)

	t.Run("returns empty for nil context", func(t *testing.T) {
		result := v.RenderGroupStatusHeader(nil)
		if result != "" {
			t.Errorf("Expected empty string for nil context")
		}
	})

	t.Run("returns empty for nil group", func(t *testing.T) {
		ctx := &GroupContext{Group: nil}
		result := v.RenderGroupStatusHeader(ctx)
		if result != "" {
			t.Errorf("Expected empty string for nil group")
		}
	})

	t.Run("renders group name and task position", func(t *testing.T) {
		ctx := &GroupContext{
			Group: &orchestrator.InstanceGroup{
				ID:    "g1",
				Name:  "Group 1: Setup",
				Phase: orchestrator.GroupPhaseExecuting,
			},
			TaskIndex:  2,
			TotalTasks: 5,
		}
		result := v.RenderGroupStatusHeader(ctx)
		if !strings.Contains(result, "Group 1: Setup") {
			t.Errorf("Expected group name in output, got: %s", result)
		}
		if !strings.Contains(result, "Task 2 of 5") {
			t.Errorf("Expected task position in output, got: %s", result)
		}
	})

	t.Run("renders dependency group names", func(t *testing.T) {
		ctx := &GroupContext{
			Group: &orchestrator.InstanceGroup{
				ID:    "g2",
				Name:  "Group 2: Core Logic",
				Phase: orchestrator.GroupPhasePending,
			},
			TaskIndex:  1,
			TotalTasks: 3,
			DependencyGroups: []*orchestrator.InstanceGroup{
				{ID: "g1", Name: "Group 1: Setup"},
			},
			AllDependenciesMet: true,
		}
		result := v.RenderGroupStatusHeader(ctx)
		if !strings.Contains(result, "Depends on:") {
			t.Errorf("Expected 'Depends on:' in output, got: %s", result)
		}
		if !strings.Contains(result, "Group 1: Setup") {
			t.Errorf("Expected dependency group name in output, got: %s", result)
		}
	})

	t.Run("renders dependencies satisfied when met", func(t *testing.T) {
		ctx := &GroupContext{
			Group: &orchestrator.InstanceGroup{
				ID:    "g2",
				Name:  "Group 2",
				Phase: orchestrator.GroupPhaseExecuting,
			},
			TaskIndex:  1,
			TotalTasks: 1,
			DependencyGroups: []*orchestrator.InstanceGroup{
				{ID: "g1", Name: "Group 1"},
			},
			AllDependenciesMet: true,
		}
		result := v.RenderGroupStatusHeader(ctx)
		if !strings.Contains(result, "Dependencies satisfied") {
			t.Errorf("Expected 'Dependencies satisfied' in output, got: %s", result)
		}
	})

	t.Run("renders waiting for pending tasks", func(t *testing.T) {
		ctx := &GroupContext{
			Group: &orchestrator.InstanceGroup{
				ID:    "g2",
				Name:  "Group 2",
				Phase: orchestrator.GroupPhasePending,
			},
			TaskIndex:  1,
			TotalTasks: 1,
			DependencyGroups: []*orchestrator.InstanceGroup{
				{ID: "g1", Name: "Group 1"},
			},
			AllDependenciesMet: false,
			PendingDependencies: []*orchestrator.Instance{
				{ID: "dep1", Task: "Setup auth models"},
				{ID: "dep2", Task: "Create migrations"},
			},
		}
		result := v.RenderGroupStatusHeader(ctx)
		if !strings.Contains(result, "Waiting for:") {
			t.Errorf("Expected 'Waiting for:' in output, got: %s", result)
		}
		if !strings.Contains(result, "Setup auth models") {
			t.Errorf("Expected pending task name in output, got: %s", result)
		}
	})

	t.Run("limits pending dependencies displayed", func(t *testing.T) {
		pending := make([]*orchestrator.Instance, 5)
		for i := range pending {
			pending[i] = &orchestrator.Instance{ID: string(rune('a' + i)), Task: "Task"}
		}
		ctx := &GroupContext{
			Group: &orchestrator.InstanceGroup{
				ID:    "g2",
				Name:  "Group 2",
				Phase: orchestrator.GroupPhasePending,
			},
			TaskIndex:  1,
			TotalTasks: 1,
			DependencyGroups: []*orchestrator.InstanceGroup{
				{ID: "g1", Name: "Group 1"},
			},
			AllDependenciesMet:  false,
			PendingDependencies: pending,
		}
		result := v.RenderGroupStatusHeader(ctx)
		if !strings.Contains(result, "+2 more") {
			t.Errorf("Expected '+2 more' for overflow, got: %s", result)
		}
	})
}

func TestRenderSiblingConnector(t *testing.T) {
	v := NewInstanceView(80, 20)

	t.Run("returns empty for nil context", func(t *testing.T) {
		result := v.RenderSiblingConnector(nil)
		if result != "" {
			t.Errorf("Expected empty string")
		}
	})

	t.Run("returns empty for no siblings", func(t *testing.T) {
		ctx := &GroupContext{
			SiblingTasks: nil,
		}
		result := v.RenderSiblingConnector(ctx)
		if result != "" {
			t.Errorf("Expected empty string for no siblings")
		}
	})

	t.Run("renders sibling connector with status", func(t *testing.T) {
		ctx := &GroupContext{
			SiblingTasks: []*orchestrator.Instance{
				{ID: "sib1", Task: "Sibling 1", Status: orchestrator.StatusCompleted},
				{ID: "sib2", Task: "Sibling 2", Status: orchestrator.StatusWorking},
			},
		}
		result := v.RenderSiblingConnector(ctx)
		if !strings.Contains(result, "Siblings:") {
			t.Errorf("Expected 'Siblings:' label, got: %s", result)
		}
		if !strings.Contains(result, "Sibling 1") {
			t.Errorf("Expected sibling name, got: %s", result)
		}
	})

	t.Run("limits siblings displayed", func(t *testing.T) {
		siblings := make([]*orchestrator.Instance, 6)
		for i := range siblings {
			siblings[i] = &orchestrator.Instance{
				ID:     string(rune('a' + i)),
				Task:   "Task",
				Status: orchestrator.StatusPending,
			}
		}
		ctx := &GroupContext{
			SiblingTasks: siblings,
		}
		result := v.RenderSiblingConnector(ctx)
		if !strings.Contains(result, "+2 more") {
			t.Errorf("Expected '+2 more' for overflow, got: %s", result)
		}
	})
}

func TestRenderWithSessionGroupedMode(t *testing.T) {
	v := NewInstanceView(80, 20)

	t.Run("renders without group header when not in grouped mode", func(t *testing.T) {
		inst := &orchestrator.Instance{
			ID:      "inst1",
			Branch:  "feature-branch",
			Task:    "Test task",
			Status:  orchestrator.StatusPending,
			Created: time.Now(),
		}
		group := &orchestrator.InstanceGroup{
			ID:        "g1",
			Name:      "Group 1",
			Phase:     orchestrator.GroupPhaseExecuting,
			Instances: []string{"inst1"},
		}
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{inst},
			Groups:    []*orchestrator.InstanceGroup{group},
		}
		state := RenderState{
			GroupedViewEnabled: false,
		}

		result := v.RenderWithSession(inst, state, session)
		if strings.Contains(result, "Group 1") {
			t.Errorf("Should not contain group header when not in grouped mode")
		}
	})

	t.Run("renders with group header when in grouped mode", func(t *testing.T) {
		inst := &orchestrator.Instance{
			ID:      "inst1",
			Branch:  "feature-branch",
			Task:    "Test task",
			Status:  orchestrator.StatusWorking,
			Created: time.Now(),
		}
		group := &orchestrator.InstanceGroup{
			ID:        "g1",
			Name:      "Group 1: Setup",
			Phase:     orchestrator.GroupPhaseExecuting,
			Instances: []string{"inst1"},
		}
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{inst},
			Groups:    []*orchestrator.InstanceGroup{group},
		}
		state := RenderState{
			GroupedViewEnabled: true,
		}

		result := v.RenderWithSession(inst, state, session)
		if !strings.Contains(result, "Group 1: Setup") {
			t.Errorf("Should contain group header when in grouped mode, got: %s", result)
		}
		if !strings.Contains(result, "Task 1 of 1") {
			t.Errorf("Should contain task position, got: %s", result)
		}
	})
}

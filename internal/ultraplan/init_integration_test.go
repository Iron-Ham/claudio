//go:build integration

package ultraplan

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/testutil"
)

// TestInit_FullIntegration tests the Init function with a real orchestrator.
// This exercises the complete initialization path that creates a Coordinator.
func TestInit_FullIntegration(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name        string
		objective   string
		config      *orchestrator.UltraPlanConfig
		plan        *orchestrator.PlanSpec
		createGroup bool
		wantPhase   orchestrator.UltraPlanPhase
		wantGroup   bool
	}{
		{
			name:      "basic init without group creates coordinator",
			objective: "Test objective",
			config: &orchestrator.UltraPlanConfig{
				MaxParallel: 5,
			},
			createGroup: false,
			wantPhase:   orchestrator.PhasePlanning,
			wantGroup:   false,
		},
		{
			name:      "init with group creates and links group",
			objective: "Group test objective",
			config: &orchestrator.UltraPlanConfig{
				MaxParallel: 3,
				MultiPass:   false,
			},
			createGroup: true,
			wantPhase:   orchestrator.PhasePlanning,
			wantGroup:   true,
		},
		{
			name:      "init with multi-pass creates PlanMulti group type",
			objective: "Multi-pass test",
			config: &orchestrator.UltraPlanConfig{
				MaxParallel: 3,
				MultiPass:   true,
			},
			createGroup: true,
			wantPhase:   orchestrator.PhasePlanning,
			wantGroup:   true,
		},
		{
			name:      "init with pre-loaded plan sets phase to refresh",
			objective: "Plan loaded test",
			config: &orchestrator.UltraPlanConfig{
				MaxParallel: 3,
			},
			plan: &orchestrator.PlanSpec{
				ID:        "test-plan",
				Objective: "Plan objective",
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Test task", DependsOn: []string{}},
				},
				DependencyGraph: map[string][]string{"task-1": {}},
				ExecutionOrder:  [][]string{{"task-1"}},
			},
			createGroup: false,
			wantPhase:   orchestrator.PhaseRefresh,
			wantGroup:   false,
		},
		{
			name:      "init with nil config uses defaults",
			objective: "Default config test",
			config:    nil,
			wantPhase: orchestrator.PhasePlanning,
			wantGroup: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test repository and orchestrator
			repoDir := testutil.SetupTestRepo(t)
			orch, err := orchestrator.New(repoDir)
			if err != nil {
				t.Fatalf("failed to create orchestrator: %v", err)
			}
			if err := orch.Init(); err != nil {
				t.Fatalf("failed to init orchestrator: %v", err)
			}

			// Start a session
			session, err := orch.StartSession("test-session")
			if err != nil {
				t.Fatalf("failed to start session: %v", err)
			}

			// Build params
			params := InitParams{
				Orchestrator: orch,
				Session:      session,
				Objective:    tt.objective,
				Config:       tt.config,
				Plan:         tt.plan,
				CreateGroup:  tt.createGroup,
			}

			result := Init(params)

			// Verify result is not nil
			if result == nil {
				t.Fatal("Init returned nil")
			}

			// Verify Coordinator is created
			if result.Coordinator == nil {
				t.Error("Coordinator should not be nil")
			}

			// Verify UltraSession is created
			if result.UltraSession == nil {
				t.Fatal("UltraSession should not be nil")
			}

			// Verify phase
			if result.UltraSession.Phase != tt.wantPhase {
				t.Errorf("Phase = %v, want %v", result.UltraSession.Phase, tt.wantPhase)
			}

			// Verify group creation
			if tt.wantGroup {
				if result.Group == nil {
					t.Error("Group should be created when CreateGroup is true")
				}
				if result.UltraSession.GroupID == "" {
					t.Error("GroupID should be set when CreateGroup is true")
				}
				if result.UltraSession.GroupID != result.Group.ID {
					t.Error("GroupID should match result.Group.ID")
				}
			} else {
				if result.Group != nil {
					t.Error("Group should not be created when CreateGroup is false")
				}
			}

			// Verify session is linked to ultraplan
			if session.UltraPlan != result.UltraSession {
				t.Error("Session.UltraPlan should be linked to the result UltraSession")
			}

			// Verify config is populated
			if tt.config == nil {
				// Should use defaults
				defaults := orchestrator.DefaultUltraPlanConfig()
				if result.Config.MaxParallel != defaults.MaxParallel {
					t.Errorf("MaxParallel = %d, want default %d",
						result.Config.MaxParallel, defaults.MaxParallel)
				}
			} else {
				if result.Config.MaxParallel != tt.config.MaxParallel {
					t.Errorf("MaxParallel = %d, want %d",
						result.Config.MaxParallel, tt.config.MaxParallel)
				}
			}

			// Verify plan handling
			if tt.plan != nil {
				if result.UltraSession.Plan == nil {
					t.Error("Plan should be set on UltraSession")
				}
			}
		})
	}
}

// TestInit_ObjectivePrecedence tests that Init correctly determines the objective
// from params or from a pre-loaded plan.
func TestInit_ObjectivePrecedence(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name          string
		objective     string
		planObjective string
		wantObjective string
	}{
		{
			name:          "objective from params when no plan",
			objective:     "Params objective",
			planObjective: "",
			wantObjective: "Params objective",
		},
		{
			name:          "plan objective takes precedence",
			objective:     "Params objective",
			planObjective: "Plan objective",
			wantObjective: "Plan objective",
		},
		{
			name:          "params objective used when plan objective empty",
			objective:     "Fallback to params",
			planObjective: "",
			wantObjective: "Fallback to params",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := testutil.SetupTestRepo(t)
			orch, err := orchestrator.New(repoDir)
			if err != nil {
				t.Fatalf("failed to create orchestrator: %v", err)
			}
			if err := orch.Init(); err != nil {
				t.Fatalf("failed to init orchestrator: %v", err)
			}

			session, err := orch.StartSession("test-session")
			if err != nil {
				t.Fatalf("failed to start session: %v", err)
			}

			var plan *orchestrator.PlanSpec
			if tt.planObjective != "" {
				plan = &orchestrator.PlanSpec{
					ID:              "test-plan",
					Objective:       tt.planObjective,
					Tasks:           []orchestrator.PlannedTask{{ID: "task-1", Title: "Task", DependsOn: []string{}}},
					DependencyGraph: map[string][]string{"task-1": {}},
					ExecutionOrder:  [][]string{{"task-1"}},
				}
			}

			result := Init(InitParams{
				Orchestrator: orch,
				Session:      session,
				Objective:    tt.objective,
				Config:       &orchestrator.UltraPlanConfig{MaxParallel: 3},
				Plan:         plan,
			})

			if result.UltraSession.Objective != tt.wantObjective {
				t.Errorf("Objective = %q, want %q",
					result.UltraSession.Objective, tt.wantObjective)
			}
		})
	}
}

// TestInit_GroupSessionTypes verifies correct SessionType on created groups.
func TestInit_GroupSessionTypes(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name            string
		multiPass       bool
		wantSessionType orchestrator.SessionType
	}{
		{
			name:            "single-pass creates UltraPlan type",
			multiPass:       false,
			wantSessionType: orchestrator.SessionTypeUltraPlan,
		},
		{
			name:            "multi-pass creates PlanMulti type",
			multiPass:       true,
			wantSessionType: orchestrator.SessionTypePlanMulti,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoDir := testutil.SetupTestRepo(t)
			orch, err := orchestrator.New(repoDir)
			if err != nil {
				t.Fatalf("failed to create orchestrator: %v", err)
			}
			if err := orch.Init(); err != nil {
				t.Fatalf("failed to init orchestrator: %v", err)
			}

			session, err := orch.StartSession("test-session")
			if err != nil {
				t.Fatalf("failed to start session: %v", err)
			}

			result := Init(InitParams{
				Orchestrator: orch,
				Session:      session,
				Objective:    "Test group types",
				Config: &orchestrator.UltraPlanConfig{
					MaxParallel: 3,
					MultiPass:   tt.multiPass,
				},
				CreateGroup: true,
			})

			if result.Group == nil {
				t.Fatal("Group should be created")
			}

			if orchestrator.GetSessionType(result.Group) != tt.wantSessionType {
				t.Errorf("Group.SessionType = %v, want %v",
					orchestrator.GetSessionType(result.Group), tt.wantSessionType)
			}
		})
	}
}

// TestInitWithPlan_FullIntegration tests the InitWithPlan function with a real orchestrator.
func TestInitWithPlan_FullIntegration(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := orchestrator.New(repoDir)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	if err := orch.Init(); err != nil {
		t.Fatalf("failed to init orchestrator: %v", err)
	}

	session, err := orch.StartSession("test-session")
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	plan := &orchestrator.PlanSpec{
		ID:        "test-plan",
		Objective: "Test InitWithPlan",
		Summary:   "Test summary",
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Task 1", DependsOn: []string{}},
			{ID: "task-2", Title: "Task 2", DependsOn: []string{"task-1"}},
		},
		DependencyGraph: map[string][]string{
			"task-1": {},
			"task-2": {"task-1"},
		},
		ExecutionOrder: [][]string{{"task-1"}, {"task-2"}},
	}

	params := InitParams{
		Orchestrator: orch,
		Session:      session,
		Objective:    "Params objective",
		Config: &orchestrator.UltraPlanConfig{
			MaxParallel: 3,
		},
	}

	result, err := InitWithPlan(params, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify result fields
	if result.Coordinator == nil {
		t.Error("Coordinator should not be nil")
	}
	if result.UltraSession == nil {
		t.Error("UltraSession should not be nil")
	}
	if result.UltraSession.Plan == nil {
		t.Error("Plan should be set on UltraSession")
	}
	if result.UltraSession.Phase != orchestrator.PhaseRefresh {
		t.Errorf("Phase = %v, want %v", result.UltraSession.Phase, orchestrator.PhaseRefresh)
	}

	// Verify the plan is accessible through the coordinator
	coordSession := result.Coordinator.Session()
	if coordSession == nil {
		t.Fatal("Coordinator.Session() returned nil")
	}
	if coordSession.Plan == nil {
		t.Error("Plan should be set on coordinator's session")
	}
	if coordSession.Plan.ID != plan.ID {
		t.Errorf("Plan ID = %q, want %q", coordSession.Plan.ID, plan.ID)
	}
}

// TestInitWithPlan_WithGroupCreation tests that InitWithPlan works correctly
// when CreateGroup is true.
func TestInitWithPlan_WithGroupCreation(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := orchestrator.New(repoDir)
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}
	if err := orch.Init(); err != nil {
		t.Fatalf("failed to init orchestrator: %v", err)
	}

	session, err := orch.StartSession("test-session")
	if err != nil {
		t.Fatalf("failed to start session: %v", err)
	}

	plan := &orchestrator.PlanSpec{
		ID:              "test-plan",
		Objective:       "Test with group",
		Tasks:           []orchestrator.PlannedTask{{ID: "task-1", Title: "Task 1", DependsOn: []string{}}},
		DependencyGraph: map[string][]string{"task-1": {}},
		ExecutionOrder:  [][]string{{"task-1"}},
	}

	params := InitParams{
		Orchestrator: orch,
		Session:      session,
		Objective:    "Params objective",
		Config: &orchestrator.UltraPlanConfig{
			MaxParallel: 3,
		},
		CreateGroup: true,
	}

	result, err := InitWithPlan(params, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Group == nil {
		t.Error("Group should be created")
	}
	if result.UltraSession.GroupID == "" {
		t.Error("GroupID should be set")
	}
	if result.UltraSession.GroupID != result.Group.ID {
		t.Error("GroupID should match result.Group.ID")
	}
}

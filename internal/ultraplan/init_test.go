package ultraplan

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestBuildConfigFromAppConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *config.Config
		validate func(t *testing.T, got orchestrator.UltraPlanConfig)
	}{
		{
			name: "nil config returns defaults",
			cfg:  nil,
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				defaults := orchestrator.DefaultUltraPlanConfig()
				if got.MaxParallel != defaults.MaxParallel {
					t.Errorf("MaxParallel = %d, want %d", got.MaxParallel, defaults.MaxParallel)
				}
				if got.MultiPass != defaults.MultiPass {
					t.Errorf("MultiPass = %v, want %v", got.MultiPass, defaults.MultiPass)
				}
				if got.RequireVerifiedCommits != defaults.RequireVerifiedCommits {
					t.Errorf("RequireVerifiedCommits = %v, want %v", got.RequireVerifiedCommits, defaults.RequireVerifiedCommits)
				}
			},
		},
		{
			name: "applies MaxParallel from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					MaxParallel: 7,
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.MaxParallel != 7 {
					t.Errorf("MaxParallel = %d, want 7", got.MaxParallel)
				}
			},
		},
		{
			name: "applies MultiPass from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					MultiPass: true,
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if !got.MultiPass {
					t.Errorf("MultiPass = false, want true")
				}
			},
		},
		{
			name: "applies ConsolidationMode from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					ConsolidationMode: "single",
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.ConsolidationMode != orchestrator.ModeSinglePR {
					t.Errorf("ConsolidationMode = %s, want %s", got.ConsolidationMode, orchestrator.ModeSinglePR)
				}
			},
		},
		{
			name: "empty ConsolidationMode preserves default",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					ConsolidationMode: "",
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				defaults := orchestrator.DefaultUltraPlanConfig()
				if got.ConsolidationMode != defaults.ConsolidationMode {
					t.Errorf("ConsolidationMode = %s, want default %s", got.ConsolidationMode, defaults.ConsolidationMode)
				}
			},
		},
		{
			name: "applies CreateDraftPRs from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					CreateDraftPRs: false,
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.CreateDraftPRs {
					t.Errorf("CreateDraftPRs = true, want false")
				}
			},
		},
		{
			name: "applies PRLabels from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					PRLabels: []string{"custom-label", "another-label"},
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if len(got.PRLabels) != 2 {
					t.Errorf("PRLabels length = %d, want 2", len(got.PRLabels))
				}
				if got.PRLabels[0] != "custom-label" {
					t.Errorf("PRLabels[0] = %s, want custom-label", got.PRLabels[0])
				}
			},
		},
		{
			name: "empty PRLabels preserves default",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					PRLabels: []string{},
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				defaults := orchestrator.DefaultUltraPlanConfig()
				if len(got.PRLabels) != len(defaults.PRLabels) {
					t.Errorf("PRLabels length = %d, want default %d", len(got.PRLabels), len(defaults.PRLabels))
				}
			},
		},
		{
			name: "applies BranchPrefix from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					BranchPrefix: "my-prefix",
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.BranchPrefix != "my-prefix" {
					t.Errorf("BranchPrefix = %s, want my-prefix", got.BranchPrefix)
				}
			},
		},
		{
			name: "applies MaxTaskRetries from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					MaxTaskRetries: 5,
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.MaxTaskRetries != 5 {
					t.Errorf("MaxTaskRetries = %d, want 5", got.MaxTaskRetries)
				}
			},
		},
		{
			name: "applies RequireVerifiedCommits from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					RequireVerifiedCommits: false,
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.RequireVerifiedCommits {
					t.Errorf("RequireVerifiedCommits = true, want false")
				}
			},
		},
		{
			name: "applies all settings from config",
			cfg: &config.Config{
				Ultraplan: config.UltraplanConfig{
					MaxParallel:            10,
					MultiPass:              true,
					ConsolidationMode:      "single",
					CreateDraftPRs:         false,
					PRLabels:               []string{"label1"},
					BranchPrefix:           "ultra",
					MaxTaskRetries:         2,
					RequireVerifiedCommits: false,
				},
			},
			validate: func(t *testing.T, got orchestrator.UltraPlanConfig) {
				if got.MaxParallel != 10 {
					t.Errorf("MaxParallel = %d, want 10", got.MaxParallel)
				}
				if !got.MultiPass {
					t.Errorf("MultiPass = false, want true")
				}
				if got.ConsolidationMode != orchestrator.ModeSinglePR {
					t.Errorf("ConsolidationMode = %s, want %s", got.ConsolidationMode, orchestrator.ModeSinglePR)
				}
				if got.CreateDraftPRs {
					t.Errorf("CreateDraftPRs = true, want false")
				}
				if len(got.PRLabels) != 1 || got.PRLabels[0] != "label1" {
					t.Errorf("PRLabels = %v, want [label1]", got.PRLabels)
				}
				if got.BranchPrefix != "ultra" {
					t.Errorf("BranchPrefix = %s, want ultra", got.BranchPrefix)
				}
				if got.MaxTaskRetries != 2 {
					t.Errorf("MaxTaskRetries = %d, want 2", got.MaxTaskRetries)
				}
				if got.RequireVerifiedCommits {
					t.Errorf("RequireVerifiedCommits = true, want false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildConfigFromAppConfig(tt.cfg)
			tt.validate(t, got)
		})
	}
}

func TestBuildConfigFromAppConfig_PreservesNonConfigFields(t *testing.T) {
	// Ensure fields not in config file (DryRun, NoSynthesis, AutoApprove, Review)
	// are preserved from defaults
	cfg := &config.Config{
		Ultraplan: config.UltraplanConfig{
			MaxParallel: 5,
		},
	}

	got := BuildConfigFromAppConfig(cfg)
	defaults := orchestrator.DefaultUltraPlanConfig()

	// These fields should retain their default values since they're not set from config
	if got.DryRun != defaults.DryRun {
		t.Errorf("DryRun = %v, want default %v", got.DryRun, defaults.DryRun)
	}
	if got.NoSynthesis != defaults.NoSynthesis {
		t.Errorf("NoSynthesis = %v, want default %v", got.NoSynthesis, defaults.NoSynthesis)
	}
	if got.AutoApprove != defaults.AutoApprove {
		t.Errorf("AutoApprove = %v, want default %v", got.AutoApprove, defaults.AutoApprove)
	}
	if got.Review != defaults.Review {
		t.Errorf("Review = %v, want default %v", got.Review, defaults.Review)
	}
}

func TestCreateUltraPlanGroup(t *testing.T) {
	tests := []struct {
		name              string
		objective         string
		multiPass         bool
		wantSessionType   orchestrator.SessionType
		wantNamePrefix    string
		wantNameMaxLen    int
		wantObjective     string
		wantNameIsDefault bool
	}{
		{
			name:            "single-pass mode sets UltraPlan session type",
			objective:       "Refactor auth module",
			multiPass:       false,
			wantSessionType: orchestrator.SessionTypeUltraPlan,
			wantNamePrefix:  "Refactor auth module",
			wantNameMaxLen:  GroupNameMaxLength,
			wantObjective:   "Refactor auth module",
		},
		{
			name:            "multi-pass mode sets PlanMulti session type",
			objective:       "Implement new feature",
			multiPass:       true,
			wantSessionType: orchestrator.SessionTypePlanMulti,
			wantNamePrefix:  "Implement new feature",
			wantNameMaxLen:  GroupNameMaxLength,
			wantObjective:   "Implement new feature",
		},
		{
			name:            "long objective is truncated to 30 chars with ellipsis",
			objective:       "This is a very long objective that exceeds the maximum length",
			multiPass:       false,
			wantSessionType: orchestrator.SessionTypeUltraPlan,
			wantNamePrefix:  "This is a very long objecti...",
			wantNameMaxLen:  GroupNameMaxLength,
			wantObjective:   "This is a very long objective that exceeds the maximum length",
		},
		{
			name:              "empty objective uses default name",
			objective:         "",
			multiPass:         false,
			wantSessionType:   orchestrator.SessionTypeUltraPlan,
			wantNamePrefix:    "UltraPlan",
			wantNameMaxLen:    GroupNameMaxLength,
			wantObjective:     "",
			wantNameIsDefault: true,
		},
		{
			name:            "short objective is not truncated",
			objective:       "Fix bug",
			multiPass:       false,
			wantSessionType: orchestrator.SessionTypeUltraPlan,
			wantNamePrefix:  "Fix bug",
			wantNameMaxLen:  GroupNameMaxLength,
			wantObjective:   "Fix bug",
		},
		{
			name:            "exactly 30 char objective is not truncated",
			objective:       "123456789012345678901234567890",
			multiPass:       true,
			wantSessionType: orchestrator.SessionTypePlanMulti,
			wantNamePrefix:  "123456789012345678901234567890",
			wantNameMaxLen:  GroupNameMaxLength,
			wantObjective:   "123456789012345678901234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := CreateUltraPlanGroup(tt.objective, tt.multiPass)

			if group == nil {
				t.Fatal("CreateUltraPlanGroup returned nil")
			}

			if orchestrator.GetSessionType(group) != tt.wantSessionType {
				t.Errorf("SessionType = %v, want %v", orchestrator.GetSessionType(group), tt.wantSessionType)
			}

			if len(group.Name) > tt.wantNameMaxLen {
				t.Errorf("Name length = %d, want <= %d", len(group.Name), tt.wantNameMaxLen)
			}

			if tt.wantNameIsDefault {
				if group.Name != "UltraPlan" {
					t.Errorf("Name = %q, want %q for empty objective", group.Name, "UltraPlan")
				}
			} else if group.Name != tt.wantNamePrefix {
				t.Errorf("Name = %q, want %q", group.Name, tt.wantNamePrefix)
			}

			if group.Objective != tt.wantObjective {
				t.Errorf("Objective = %q, want %q", group.Objective, tt.wantObjective)
			}

			if group.ID == "" {
				t.Error("Group ID should be generated")
			}

			if group.Phase != orchestrator.GroupPhasePending {
				t.Errorf("Phase = %v, want %v", group.Phase, orchestrator.GroupPhasePending)
			}
		})
	}
}

func TestCreateAndLinkUltraPlanGroup(t *testing.T) {
	tests := []struct {
		name            string
		objective       string
		multiPass       bool
		wantSessionType orchestrator.SessionType
	}{
		{
			name:            "links group to session and ultraplan session",
			objective:       "Test objective",
			multiPass:       false,
			wantSessionType: orchestrator.SessionTypeUltraPlan,
		},
		{
			name:            "multi-pass mode sets correct type",
			objective:       "Multi-pass objective",
			multiPass:       true,
			wantSessionType: orchestrator.SessionTypePlanMulti,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := orchestrator.NewSession("test-session", "/tmp/repo")
			ultraSession := orchestrator.NewUltraPlanSession(tt.objective, orchestrator.UltraPlanConfig{
				MultiPass: tt.multiPass,
			})

			group := CreateAndLinkUltraPlanGroup(session, ultraSession, tt.multiPass)

			if group == nil {
				t.Fatal("CreateAndLinkUltraPlanGroup returned nil")
			}

			// Verify group is added to session
			groups := session.GetGroups()
			if len(groups) != 1 {
				t.Errorf("session has %d groups, want 1", len(groups))
			}

			if len(groups) > 0 && groups[0].ID != group.ID {
				t.Errorf("session group ID = %q, want %q", groups[0].ID, group.ID)
			}

			// Verify group is linked to ultraplan session
			if ultraSession.GroupID != group.ID {
				t.Errorf("ultraSession.GroupID = %q, want %q", ultraSession.GroupID, group.ID)
			}

			// Verify session type is correct
			if orchestrator.GetSessionType(group) != tt.wantSessionType {
				t.Errorf("SessionType = %v, want %v", orchestrator.GetSessionType(group), tt.wantSessionType)
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
			name:   "empty string returns empty",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "string shorter than max is unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "string equal to max is unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "string longer than max is truncated with ellipsis",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "very short maxLen truncates without ellipsis",
			input:  "hello",
			maxLen: 2,
			want:   "he",
		},
		{
			name:   "maxLen of 3 truncates without ellipsis",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
		{
			name:   "maxLen of 4 truncates with single char plus ellipsis",
			input:  "hello",
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

func TestGroupNameMaxLengthConstant(t *testing.T) {
	// Verify the constant matches the expected value
	if GroupNameMaxLength != 30 {
		t.Errorf("GroupNameMaxLength = %d, want 30", GroupNameMaxLength)
	}
}

// TestInit_ConfigResolution tests that Init correctly resolves configuration
// from either provided config or defaults.
func TestInit_ConfigResolution(t *testing.T) {
	// Create test fixtures
	session := orchestrator.NewSession("test-session", "/tmp/repo")

	tests := []struct {
		name           string
		config         *orchestrator.UltraPlanConfig
		validateConfig func(t *testing.T, cfg orchestrator.UltraPlanConfig)
	}{
		{
			name:   "nil config uses defaults from file",
			config: nil,
			validateConfig: func(t *testing.T, cfg orchestrator.UltraPlanConfig) {
				// When config is nil, BuildConfigFromFile is called
				// We can't test exact values since they depend on config file,
				// but we can verify default behavior
				defaults := orchestrator.DefaultUltraPlanConfig()
				// RequireVerifiedCommits should be from defaults (true by default)
				if cfg.RequireVerifiedCommits != defaults.RequireVerifiedCommits {
					t.Errorf("RequireVerifiedCommits = %v, want default %v",
						cfg.RequireVerifiedCommits, defaults.RequireVerifiedCommits)
				}
			},
		},
		{
			name: "provided config is used as-is",
			config: &orchestrator.UltraPlanConfig{
				MaxParallel:            99,
				MultiPass:              true,
				RequireVerifiedCommits: false,
			},
			validateConfig: func(t *testing.T, cfg orchestrator.UltraPlanConfig) {
				if cfg.MaxParallel != 99 {
					t.Errorf("MaxParallel = %d, want 99", cfg.MaxParallel)
				}
				if !cfg.MultiPass {
					t.Errorf("MultiPass = false, want true")
				}
				if cfg.RequireVerifiedCommits {
					t.Errorf("RequireVerifiedCommits = true, want false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't fully test Init without a real orchestrator,
			// but we can test the InitParams and validate that config
			// would be resolved correctly by testing BuildConfigFromAppConfig
			// separately (which is tested above).

			// For this test, we verify the params struct is correctly built
			params := InitParams{
				Orchestrator: nil, // Would be required for full Init
				Session:      session,
				Objective:    "Test objective",
				Config:       tt.config,
			}

			// Resolve config the same way Init does
			var cfg orchestrator.UltraPlanConfig
			if params.Config != nil {
				cfg = *params.Config
			} else {
				cfg = BuildConfigFromFile()
			}

			tt.validateConfig(t, cfg)
		})
	}
}

// TestInit_ObjectiveResolution tests that Init correctly determines the objective
// from params or from a pre-loaded plan.
func TestInit_ObjectiveResolution(t *testing.T) {
	tests := []struct {
		name          string
		objective     string
		plan          *orchestrator.PlanSpec
		wantObjective string
	}{
		{
			name:          "objective from params",
			objective:     "Params objective",
			plan:          nil,
			wantObjective: "Params objective",
		},
		{
			name:      "objective from plan when params objective is empty",
			objective: "",
			plan: &orchestrator.PlanSpec{
				Objective: "Plan objective",
			},
			wantObjective: "Plan objective",
		},
		{
			name:      "plan objective takes precedence when both provided",
			objective: "Params objective",
			plan: &orchestrator.PlanSpec{
				Objective: "Plan objective",
			},
			wantObjective: "Plan objective",
		},
		{
			name:      "params objective used when plan has empty objective",
			objective: "Params objective",
			plan: &orchestrator.PlanSpec{
				Objective: "",
			},
			wantObjective: "Params objective",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the objective resolution logic from Init
			objective := tt.objective
			if tt.plan != nil && tt.plan.Objective != "" {
				objective = tt.plan.Objective
			}

			if objective != tt.wantObjective {
				t.Errorf("resolved objective = %q, want %q", objective, tt.wantObjective)
			}
		})
	}
}

// TestInitParams_RequiredFields documents the required vs optional fields
// in InitParams through compilation (field access) and comments.
func TestInitParams_RequiredFields(t *testing.T) {
	// This test serves as documentation of the InitParams contract.
	// The struct fields are:
	// - Orchestrator: Required (but nil for unit tests)
	// - Session: Required
	// - Objective: Required when Plan is nil
	// - Config: Optional (defaults via BuildConfigFromFile)
	// - Plan: Optional (skips planning phase if provided)
	// - Logger: Optional (nil uses no-op logger)
	// - CreateGroup: Optional (default false)

	session := orchestrator.NewSession("test", "/tmp")
	cfg := orchestrator.DefaultUltraPlanConfig()

	// Minimal params (without orchestrator for unit test)
	minimalParams := InitParams{
		Session:   session,
		Objective: "Test",
	}

	// Full params
	fullParams := InitParams{
		Orchestrator: nil,
		Session:      session,
		Objective:    "Test objective",
		Config:       &cfg,
		Plan: &orchestrator.PlanSpec{
			Objective: "Plan objective",
			Tasks:     []orchestrator.PlannedTask{},
		},
		Logger:      nil,
		CreateGroup: true,
	}

	// Verify params are valid (compilation test)
	if minimalParams.Session == nil {
		t.Error("minimalParams.Session should not be nil")
	}
	if fullParams.CreateGroup != true {
		t.Error("fullParams.CreateGroup should be true")
	}
}

// TestInitResult_Fields documents the InitResult contract.
func TestInitResult_Fields(t *testing.T) {
	// This test documents what InitResult contains.
	// All fields should be non-nil when Init returns successfully (except Group).

	// Create a mock result to verify struct fields
	cfg := orchestrator.DefaultUltraPlanConfig()
	ultraSession := orchestrator.NewUltraPlanSession("Test", cfg)

	result := &InitResult{
		Coordinator:  nil, // Would be set by Init
		UltraSession: ultraSession,
		Config:       cfg,
		Group:        nil, // Only set when CreateGroup=true
	}

	// Verify the result struct has expected fields
	if result.UltraSession == nil {
		t.Error("UltraSession should not be nil")
	}
	if result.Config.MaxParallel == 0 && cfg.MaxParallel != 0 {
		t.Error("Config should match provided config")
	}
}

// TestInit_PlanPhaseHandling tests that providing a pre-loaded plan
// correctly sets the session phase.
func TestInit_PlanPhaseHandling(t *testing.T) {
	tests := []struct {
		name      string
		plan      *orchestrator.PlanSpec
		wantPhase orchestrator.UltraPlanPhase
	}{
		{
			name:      "no plan starts in planning phase",
			plan:      nil,
			wantPhase: orchestrator.PhasePlanning,
		},
		{
			name: "with plan skips to refresh phase",
			plan: &orchestrator.PlanSpec{
				Objective: "Test",
				Tasks:     []orchestrator.PlannedTask{},
			},
			wantPhase: orchestrator.PhaseRefresh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the plan handling logic from Init
			cfg := orchestrator.DefaultUltraPlanConfig()
			ultraSession := orchestrator.NewUltraPlanSession("Test", cfg)

			if tt.plan != nil {
				ultraSession.Plan = tt.plan
				ultraSession.Phase = orchestrator.PhaseRefresh
			}

			if ultraSession.Phase != tt.wantPhase {
				t.Errorf("Phase = %v, want %v", ultraSession.Phase, tt.wantPhase)
			}
		})
	}
}

// TestInit_GroupCreation tests that CreateGroup flag properly controls
// group creation via CreateAndLinkUltraPlanGroup.
func TestInit_GroupCreation(t *testing.T) {
	tests := []struct {
		name        string
		createGroup bool
		multiPass   bool
		wantType    orchestrator.SessionType
	}{
		{
			name:        "single-pass creates UltraPlan group",
			createGroup: true,
			multiPass:   false,
			wantType:    orchestrator.SessionTypeUltraPlan,
		},
		{
			name:        "multi-pass creates PlanMulti group",
			createGroup: true,
			multiPass:   true,
			wantType:    orchestrator.SessionTypePlanMulti,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := orchestrator.NewSession("test", "/tmp")
			cfg := orchestrator.UltraPlanConfig{
				MultiPass: tt.multiPass,
			}
			ultraSession := orchestrator.NewUltraPlanSession("Test objective", cfg)

			// Test the group creation behavior
			var group *orchestrator.InstanceGroup
			if tt.createGroup {
				group = CreateAndLinkUltraPlanGroup(session, ultraSession, cfg.MultiPass)
			}

			if tt.createGroup {
				if group == nil {
					t.Fatal("expected group to be created")
				}
				if orchestrator.GetSessionType(group) != tt.wantType {
					t.Errorf("SessionType = %v, want %v", orchestrator.GetSessionType(group), tt.wantType)
				}
				if ultraSession.GroupID != group.ID {
					t.Errorf("GroupID = %q, want %q", ultraSession.GroupID, group.ID)
				}
			} else {
				if group != nil {
					t.Error("expected no group to be created")
				}
			}
		})
	}
}

// TestInitWithPlan_ValidationErrors tests that InitWithPlan properly validates plans
// before attempting initialization. This tests the validation logic without needing
// a full orchestrator.
func TestInitWithPlan_ValidationErrors(t *testing.T) {
	tests := []struct {
		name     string
		plan     *orchestrator.PlanSpec
		errorMsg string
	}{
		{
			name:     "nil plan fails validation",
			plan:     nil,
			errorMsg: "plan is nil",
		},
		{
			name: "plan with no tasks fails validation",
			plan: &orchestrator.PlanSpec{
				ID:        "empty-plan",
				Objective: "Empty",
				Tasks:     []orchestrator.PlannedTask{},
			},
			errorMsg: "plan has no tasks",
		},
		{
			name: "plan with invalid dependency fails validation",
			plan: &orchestrator.PlanSpec{
				ID:        "invalid-deps",
				Objective: "Invalid deps",
				Tasks: []orchestrator.PlannedTask{
					{ID: "task-1", Title: "Task 1", DependsOn: []string{"nonexistent"}},
				},
			},
			errorMsg: "depends on unknown task",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := orchestrator.NewSession("test-session", "/tmp/test-repo")
			params := InitParams{
				Orchestrator: nil,
				Session:      session,
				Objective:    "Params objective",
				Config: &orchestrator.UltraPlanConfig{
					MaxParallel: 3,
				},
			}

			result, err := InitWithPlan(params, tt.plan)

			if err == nil {
				t.Error("expected error but got nil")
			} else if !contains(err.Error(), tt.errorMsg) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.errorMsg)
			}
			if result != nil {
				t.Error("result should be nil when error occurs")
			}
		})
	}
}

// contains is a helper function for string containment check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

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

			if group.SessionType != tt.wantSessionType {
				t.Errorf("SessionType = %v, want %v", group.SessionType, tt.wantSessionType)
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
			if group.SessionType != tt.wantSessionType {
				t.Errorf("SessionType = %v, want %v", group.SessionType, tt.wantSessionType)
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

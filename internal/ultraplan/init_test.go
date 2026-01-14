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

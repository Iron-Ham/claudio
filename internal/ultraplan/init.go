// Package ultraplan provides initialization utilities for Ultra-Plan sessions.
package ultraplan

import (
	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// BuildConfigFromFile creates an UltraPlanConfig initialized from the application
// config file settings. This function consolidates the duplicate config loading logic
// that was previously in both cmd/ultraplan.go and internal/tui/inlineplan.go.
//
// The returned config starts with DefaultUltraPlanConfig() defaults and then applies
// any settings from the config file's [ultraplan] section. Callers can further override
// specific fields after receiving the config (e.g., for CLI flags).
//
// If the config file cannot be loaded, defaults are used for all settings.
func BuildConfigFromFile() orchestrator.UltraPlanConfig {
	return BuildConfigFromAppConfig(config.Get())
}

// BuildConfigFromAppConfig creates an UltraPlanConfig from an existing *config.Config.
// This is useful when the caller has already loaded the config or needs to provide
// a custom config for testing.
//
// The function applies config file settings to the default UltraPlanConfig:
//   - MaxParallel: maximum concurrent child sessions
//   - MultiPass: enable multi-pass planning
//   - ConsolidationMode: "stacked" or "single" PR mode
//   - CreateDraftPRs: create PRs as drafts
//   - PRLabels: labels to add to created PRs
//   - BranchPrefix: prefix for ultraplan branches
//   - MaxTaskRetries: retry attempts for tasks with no commits
//   - RequireVerifiedCommits: require tasks to produce commits
func BuildConfigFromAppConfig(cfg *config.Config) orchestrator.UltraPlanConfig {
	ultraCfg := orchestrator.DefaultUltraPlanConfig()

	if cfg == nil {
		return ultraCfg
	}

	// Apply config file settings
	ultraCfg.MaxParallel = cfg.Ultraplan.MaxParallel
	ultraCfg.MultiPass = cfg.Ultraplan.MultiPass

	if cfg.Ultraplan.ConsolidationMode != "" {
		ultraCfg.ConsolidationMode = orchestrator.ConsolidationMode(cfg.Ultraplan.ConsolidationMode)
	}

	ultraCfg.CreateDraftPRs = cfg.Ultraplan.CreateDraftPRs

	if len(cfg.Ultraplan.PRLabels) > 0 {
		ultraCfg.PRLabels = cfg.Ultraplan.PRLabels
	}

	ultraCfg.BranchPrefix = cfg.Ultraplan.BranchPrefix
	ultraCfg.MaxTaskRetries = cfg.Ultraplan.MaxTaskRetries
	ultraCfg.RequireVerifiedCommits = cfg.Ultraplan.RequireVerifiedCommits

	return ultraCfg
}

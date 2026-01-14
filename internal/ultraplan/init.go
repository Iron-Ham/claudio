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

// GroupNameMaxLength is the maximum length for ultraplan group names.
// Longer names are truncated with "..." suffix.
const GroupNameMaxLength = 30

// CreateUltraPlanGroup creates an InstanceGroup for an ultraplan session with the
// appropriate SessionType based on the MultiPass configuration.
//
// This is a helper function that consolidates the duplicated group creation logic
// from multiple locations (app.go NewWithUltraPlan, inlineplan.go initInlineUltraPlanMode,
// and handleUltraPlanObjectiveSubmit).
//
// It creates a group with:
//   - Name: truncated objective (max 30 chars with "..." suffix if needed)
//   - SessionType: UltraPlan for single-pass, PlanMulti for multi-pass mode
//   - Objective: full objective string for LLM-based name generation
//
// The caller is responsible for:
//   - Adding the group to the session via session.AddGroup()
//   - Linking the group to the UltraPlanSession via ultraSession.GroupID = group.ID
func CreateUltraPlanGroup(objective string, multiPass bool) *orchestrator.InstanceGroup {
	sessionType := orchestrator.SessionTypeUltraPlan
	if multiPass {
		sessionType = orchestrator.SessionTypePlanMulti
	}

	name := truncateString(objective, GroupNameMaxLength)
	if name == "" {
		name = "UltraPlan"
	}

	return orchestrator.NewInstanceGroupWithType(name, sessionType, objective)
}

// CreateAndLinkUltraPlanGroup creates an InstanceGroup for the ultraplan session,
// adds it to the session, and links it to the UltraPlanSession.
//
// This is a convenience function that combines CreateUltraPlanGroup with the
// common follow-up operations. It returns the created group.
//
// Parameters:
//   - session: the Claudio session to add the group to
//   - ultraSession: the UltraPlanSession to link the group to
//   - multiPass: whether multi-pass planning mode is enabled
//
// The function uses the objective from ultraSession.Objective.
func CreateAndLinkUltraPlanGroup(
	session *orchestrator.Session,
	ultraSession *orchestrator.UltraPlanSession,
	multiPass bool,
) *orchestrator.InstanceGroup {
	group := CreateUltraPlanGroup(ultraSession.Objective, multiPass)
	session.AddGroup(group)
	ultraSession.GroupID = group.ID
	return group
}

// truncateString truncates a string to maxLen characters, adding "..." suffix
// if truncation was needed. Returns empty string if input is empty.
func truncateString(s string, maxLen int) string {
	if s == "" {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

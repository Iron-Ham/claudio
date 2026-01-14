// Package ultraplan provides initialization utilities for Ultra-Plan sessions.
package ultraplan

import (
	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/logging"
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

// InitParams contains the input parameters for initializing an ultraplan coordinator.
// This struct encapsulates all the dependencies required to create a fully-initialized
// ultraplan session with its coordinator, enabling a single cohesive initialization path.
type InitParams struct {
	// Orchestrator is the parent orchestrator that manages sessions and instances.
	// Required.
	Orchestrator *orchestrator.Orchestrator

	// Session is the parent Claudio session that will contain the ultraplan.
	// Required.
	Session *orchestrator.Session

	// Objective is the high-level goal for the ultraplan to accomplish.
	// Required when Plan is nil.
	Objective string

	// Config is the ultraplan configuration. If nil, BuildConfigFromFile() is used.
	// Optional.
	Config *orchestrator.UltraPlanConfig

	// Plan is a pre-loaded plan to use instead of running the planning phase.
	// When provided, the ultraplan starts in PhaseRefresh instead of PhasePlanning.
	// Optional.
	Plan *orchestrator.PlanSpec

	// Logger is the structured logger for ultraplan operations.
	// If nil, a no-op logger is used.
	// Optional.
	Logger *logging.Logger

	// CreateGroup controls whether to create and link an InstanceGroup for the
	// ultraplan session. Set to true for TUI mode where groups are displayed
	// in the sidebar. Set to false if the caller manages groups separately.
	// Default: false (caller manages groups).
	CreateGroup bool
}

// InitResult contains the outputs from initializing an ultraplan coordinator.
// All fields are guaranteed to be non-nil when Init returns successfully.
type InitResult struct {
	// Coordinator is the fully-initialized ultraplan coordinator.
	Coordinator *orchestrator.Coordinator

	// UltraSession is the underlying ultraplan session managed by the coordinator.
	UltraSession *orchestrator.UltraPlanSession

	// Config is the resolved ultraplan configuration (after applying defaults
	// and config file settings).
	Config orchestrator.UltraPlanConfig

	// Group is the InstanceGroup created for this ultraplan session.
	// Only populated when InitParams.CreateGroup is true.
	Group *orchestrator.InstanceGroup
}

// Init initializes an ultraplan coordinator with all required components.
// This is the canonical way to initialize ultraplan from both CLI and TUI.
//
// The function performs the following steps:
//  1. Resolves the ultraplan configuration (using provided config or loading from file)
//  2. Creates the UltraPlanSession with the objective and config
//  3. Applies a pre-loaded plan if provided (setting phase to PhaseRefresh)
//  4. Links the ultraplan session to the parent session for persistence
//  5. Creates the Coordinator with all dependencies
//  6. Optionally creates and links an InstanceGroup for TUI display
//
// The caller is responsible for:
//   - Starting the planning phase with coordinator.RunPlanning() (unless a plan was provided)
//   - Setting callbacks on the coordinator if needed
//   - Managing the TUI state (e.g., view.UltraPlanState)
//
// Example usage (TUI inline mode):
//
//	result := ultraplan.Init(ultraplan.InitParams{
//	    Orchestrator: m.orchestrator,
//	    Session:      m.session,
//	    Objective:    objective,
//	    Logger:       m.logger,
//	    CreateGroup:  true,
//	})
//	if err := result.Coordinator.RunPlanning(); err != nil {
//	    // handle error
//	}
//
// Example usage (CLI with pre-loaded plan):
//
//	result := ultraplan.Init(ultraplan.InitParams{
//	    Orchestrator: orch,
//	    Session:      session,
//	    Plan:         loadedPlan,
//	    Config:       &customConfig,
//	    Logger:       logger,
//	})
//	// Plan is already set, can proceed to review or execution
func Init(params InitParams) *InitResult {
	// Step 1: Resolve configuration
	var cfg orchestrator.UltraPlanConfig
	if params.Config != nil {
		cfg = *params.Config
	} else {
		cfg = BuildConfigFromFile()
	}

	// Step 2: Determine objective
	objective := params.Objective
	if params.Plan != nil && params.Plan.Objective != "" {
		objective = params.Plan.Objective
	}

	// Step 3: Create UltraPlanSession
	ultraSession := orchestrator.NewUltraPlanSession(objective, cfg)

	// Step 4: Apply pre-loaded plan if provided
	if params.Plan != nil {
		ultraSession.Plan = params.Plan
		ultraSession.Phase = orchestrator.PhaseRefresh
	}

	// Step 5: Link ultraplan session to parent session for persistence
	params.Session.UltraPlan = ultraSession

	// Step 6: Create Coordinator
	coordinator := orchestrator.NewCoordinator(
		params.Orchestrator,
		params.Session,
		ultraSession,
		params.Logger,
	)

	// Step 7: Optionally create and link InstanceGroup
	var group *orchestrator.InstanceGroup
	if params.CreateGroup {
		group = CreateAndLinkUltraPlanGroup(params.Session, ultraSession, cfg.MultiPass)
	}

	return &InitResult{
		Coordinator:  coordinator,
		UltraSession: ultraSession,
		Config:       cfg,
		Group:        group,
	}
}

// InitWithPlan is a convenience function for initializing an ultraplan with a pre-loaded plan.
// It's equivalent to calling Init with Plan set and validates the plan before returning.
//
// Returns the InitResult and an error if the plan is invalid.
func InitWithPlan(params InitParams, plan *orchestrator.PlanSpec) (*InitResult, error) {
	// Validate the plan first
	if err := orchestrator.ValidatePlan(plan); err != nil {
		return nil, err
	}

	// Set the plan on params
	params.Plan = plan

	// Use the standard Init
	result := Init(params)

	// Set the plan on the coordinator (ensures all internal state is updated)
	if err := result.Coordinator.SetPlan(plan); err != nil {
		return nil, err
	}

	return result, nil
}

package orchestrator

import (
	"fmt"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
)

// BuildPlanManagerPrompt constructs the prompt for the plan manager using the
// session's candidate plans and objective. This is used in multi-pass planning
// when selecting the best plan from multiple planning strategies.
//
// This is a package-level helper function that replaces the former Coordinator method.
func BuildPlanManagerPrompt(c *Coordinator) string {
	session := c.Session()
	strategyNames := GetMultiPassStrategyNames()

	// Convert []*PlanSpec to []prompt.CandidatePlanInfo
	candidatePlans := convertPlanSpecsToCandidatePlans(session.CandidatePlans, strategyNames)

	// Use PlanningBuilder to format the prompt
	builder := prompt.NewPlanningBuilder()
	return builder.BuildCompactPlanManagerPrompt(session.Objective, candidatePlans, strategyNames)
}

// BuildPlanComparisonSection formats all candidate plans for comparison by the plan manager.
// Each plan includes its strategy name, summary, and full task list in JSON format.
//
// This is a package-level helper function that replaces the former Coordinator method.
func BuildPlanComparisonSection(c *Coordinator) string {
	session := c.Session()
	strategyNames := GetMultiPassStrategyNames()

	// Convert []*PlanSpec to []prompt.CandidatePlanInfo, filtering out nil plans
	candidatePlans := convertPlanSpecsToCandidatePlans(session.CandidatePlans, strategyNames)

	// Use PlanningBuilder to format detailed plans with JSON task output
	builder := prompt.NewPlanningBuilder()
	return builder.FormatDetailedPlans(candidatePlans, strategyNames)
}

// BuildConsolidationPrompt creates the prompt for the consolidation phase using
// the prompt.ConsolidationBuilder. It gathers session state and formats it into
// a comprehensive prompt for merging task branches.
//
// This is a package-level helper function that replaces the former Coordinator method.
func BuildConsolidationPrompt(c *Coordinator) string {
	session := c.Session()

	// Get branch configuration
	branchPrefix := session.Config.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = c.orch.config.Branch.Prefix
	}
	if branchPrefix == "" {
		branchPrefix = "Iron-Ham"
	}

	mainBranch := c.orch.wt.FindMainBranch()

	// Build prompt context for the consolidation builder
	ctx := &prompt.Context{
		Phase:         prompt.PhaseConsolidation,
		SessionID:     session.ID,
		Objective:     session.Objective,
		Plan:          planInfoFromPlanSpec(session.Plan),
		Consolidation: consolidationInfoFromSession(session, mainBranch),
		Synthesis:     synthesisInfoFromCompletion(session.SynthesisCompletion),
	}

	// Ensure consolidation info has the resolved branch prefix
	if ctx.Consolidation != nil && ctx.Consolidation.BranchPrefix == "" {
		ctx.Consolidation.BranchPrefix = branchPrefix
	}

	// Add previous group context if available
	ctx.PreviousGroupContext = buildPreviousGroupContextStrings(session.GroupConsolidationContexts)

	// Use the ConsolidationBuilder to generate the prompt
	builder := prompt.NewConsolidationBuilder()
	result, err := builder.Build(ctx)
	if err != nil {
		// Log the error and fall back to a minimal prompt
		c.logger.Error("failed to build consolidation prompt",
			"error", err.Error(),
		)
		return fmt.Sprintf("Consolidate task branches for objective: %s\n\nMerge all completed task branches and create pull requests.", session.Objective)
	}

	return result
}

// MonitorConsolidationInstance monitors the consolidation instance and completes when done.
// This runs in a goroutine and polls the instance status periodically.
//
// This is a package-level helper function that replaces the former Coordinator method.
func MonitorConsolidationInstance(c *Coordinator, instanceID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return

		case <-ticker.C:
			inst := c.orch.GetInstance(instanceID)
			if inst == nil {
				// Instance gone, assume complete
				FinishConsolidation(c)
				return
			}

			switch inst.Status {
			case StatusCompleted, StatusWaitingInput:
				// Consolidation complete
				FinishConsolidation(c)
				return

			case StatusError, StatusTimeout, StatusStuck:
				// Consolidation failed
				session := c.Session()
				c.mu.Lock()
				session.Phase = PhaseFailed
				session.Error = fmt.Sprintf("consolidation failed: %s", inst.Status)
				if session.Consolidation != nil {
					session.Consolidation.Phase = ConsolidationFailed
					session.Consolidation.Error = string(inst.Status)
				}
				c.mu.Unlock()
				_ = c.orch.SaveSession()
				c.notifyComplete(false, session.Error)
				return
			}
		}
	}
}

// FinishConsolidation completes the ultraplan after successful consolidation.
// It updates the session state to mark the plan as complete.
//
// This is a package-level helper function that replaces the former Coordinator method.
func FinishConsolidation(c *Coordinator) {
	session := c.Session()

	c.mu.Lock()
	session.Phase = PhaseComplete
	now := time.Now()
	session.CompletedAt = &now
	if session.Consolidation != nil {
		session.Consolidation.Phase = ConsolidationComplete
		completedAt := time.Now()
		session.Consolidation.CompletedAt = &completedAt
	}
	c.mu.Unlock()
	_ = c.orch.SaveSession()

	prCount := len(session.PRUrls)
	c.notifyComplete(true, fmt.Sprintf("Completed: %d PR(s) created", prCount))
}

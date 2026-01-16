// Package prompt provides interfaces and types for building prompts
// in the ultra-plan orchestration system.
package prompt

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PlanFileName is the standard filename for plan output.
const PlanFileName = ".claudio-plan.json"

// PlanningBuilder builds prompts for the plan selection phase.
// It formats candidate plans from multiple planning strategies for comparison
// and selection by the plan manager coordinator.
type PlanningBuilder struct{}

// NewPlanningBuilder creates a new PlanningBuilder.
func NewPlanningBuilder() *PlanningBuilder {
	return &PlanningBuilder{}
}

// Build generates the plan manager prompt from the context.
// It formats all candidate plans for comparison and includes evaluation criteria.
func (b *PlanningBuilder) Build(ctx *Context) (string, error) {
	if err := b.validate(ctx); err != nil {
		return "", err
	}

	plansSection := b.formatCandidatePlans(ctx)
	return fmt.Sprintf(PlanManagerPromptTemplate, ctx.Objective, plansSection), nil
}

// validate checks that the context has all required fields for planning.
func (b *PlanningBuilder) validate(ctx *Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if ctx.Phase != PhasePlanSelection && ctx.Phase != PhasePlanning {
		return fmt.Errorf("%w: expected %s or %s, got %s",
			ErrInvalidPhase, PhasePlanSelection, PhasePlanning, ctx.Phase)
	}
	if ctx.Objective == "" {
		return ErrEmptyObjective
	}
	if len(ctx.CandidatePlans) == 0 {
		return ErrMissingCandidatePlans
	}
	return nil
}

// formatCandidatePlans formats all candidate plans for comparison.
// Uses ctx.StrategyNames[i] as fallback when plan.Strategy is empty.
func (b *PlanningBuilder) formatCandidatePlans(ctx *Context) string {
	return b.FormatDetailedPlans(ctx.CandidatePlans, ctx.StrategyNames)
}

// FormatDetailedPlans formats candidate plans with full JSON task details.
// This is the detailed format used for plan comparison and selection.
// Uses strategyNames[i] as fallback when plan.Strategy is empty.
// Plans with nil entries are skipped.
func (b *PlanningBuilder) FormatDetailedPlans(plans []CandidatePlanInfo, strategyNames []string) string {
	var sb strings.Builder

	for i, plan := range plans {
		strategyName := plan.Strategy
		if strategyName == "" && strategyNames != nil && i < len(strategyNames) {
			strategyName = strategyNames[i]
		}
		if strategyName == "" {
			strategyName = fmt.Sprintf("strategy-%d", i+1)
		}

		sb.WriteString(fmt.Sprintf("### Plan %d: %s\n\n", i+1, strategyName))
		sb.WriteString(fmt.Sprintf("**Summary**: %s\n\n", plan.Summary))

		// Task count and parallelism stats
		sb.WriteString(fmt.Sprintf("**Task Count**: %d tasks\n", len(plan.Tasks)))
		if len(plan.ExecutionOrder) > 0 {
			sb.WriteString(fmt.Sprintf("**Execution Groups**: %d groups\n", len(plan.ExecutionOrder)))
			// Calculate max parallelism (largest group)
			maxParallel := 0
			for _, group := range plan.ExecutionOrder {
				if len(group) > maxParallel {
					maxParallel = len(group)
				}
			}
			sb.WriteString(fmt.Sprintf("**Max Parallelism**: %d concurrent tasks\n", maxParallel))
		}
		sb.WriteString("\n")

		// Insights
		if len(plan.Insights) > 0 {
			sb.WriteString("**Insights**:\n")
			for _, insight := range plan.Insights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
			sb.WriteString("\n")
		}

		// Constraints
		if len(plan.Constraints) > 0 {
			sb.WriteString("**Constraints**:\n")
			for _, constraint := range plan.Constraints {
				sb.WriteString(fmt.Sprintf("- %s\n", constraint))
			}
			sb.WriteString("\n")
		}

		// Full task list in JSON format for detailed comparison
		sb.WriteString("**Tasks (JSON)**:\n```json\n")
		tasksJSON, err := json.MarshalIndent(plan.Tasks, "", "  ")
		if err != nil {
			sb.WriteString(fmt.Sprintf("Error marshaling tasks: %v\n", err))
		} else {
			sb.WriteString(string(tasksJSON))
		}
		sb.WriteString("\n```\n\n")

		// Execution order visualization
		if len(plan.ExecutionOrder) > 0 {
			sb.WriteString("**Execution Order**:\n")
			for groupIdx, group := range plan.ExecutionOrder {
				sb.WriteString(fmt.Sprintf("- Group %d: %s\n", groupIdx+1, strings.Join(group, ", ")))
			}
			sb.WriteString("\n")
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// FormatCompactPlans formats candidate plans in a compact format
// suitable for the initial plan manager prompt.
// For fallback strategy names from context, use FormatCompactPlansWithContext.
func (b *PlanningBuilder) FormatCompactPlans(plans []CandidatePlanInfo) string {
	return b.FormatCompactPlansWithContext(plans, nil)
}

// FormatCompactPlansWithContext formats candidate plans in a compact format
// suitable for the initial plan manager prompt.
// Uses strategyNames[i] as fallback when plan.Strategy is empty.
func (b *PlanningBuilder) FormatCompactPlansWithContext(plans []CandidatePlanInfo, strategyNames []string) string {
	var sb strings.Builder

	for i, plan := range plans {
		strategyName := plan.Strategy
		if strategyName == "" && strategyNames != nil && i < len(strategyNames) {
			strategyName = strategyNames[i]
		}
		if strategyName == "" {
			strategyName = "unknown"
		}

		sb.WriteString(fmt.Sprintf("\n### Plan %d: %s Strategy\n\n", i+1, strategyName))
		sb.WriteString(fmt.Sprintf("**Summary:** %s\n\n", plan.Summary))
		sb.WriteString(fmt.Sprintf("**Tasks (%d total):**\n", len(plan.Tasks)))
		for _, task := range plan.Tasks {
			deps := "none"
			if len(task.DependsOn) > 0 {
				deps = strings.Join(task.DependsOn, ", ")
			}
			sb.WriteString(fmt.Sprintf("- [%s] %s (complexity: %s, depends: %s)\n",
				task.ID, task.Title, task.EstComplexity, deps))
		}
		sb.WriteString(fmt.Sprintf("\n**Execution Groups:** %d parallel groups\n", len(plan.ExecutionOrder)))
		for groupIdx, group := range plan.ExecutionOrder {
			sb.WriteString(fmt.Sprintf("  - Group %d: %s\n", groupIdx+1, strings.Join(group, ", ")))
		}

		if len(plan.Insights) > 0 {
			sb.WriteString("\n**Insights:**\n")
			for _, insight := range plan.Insights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
		}

		if len(plan.Constraints) > 0 {
			sb.WriteString("\n**Constraints:**\n")
			for _, constraint := range plan.Constraints {
				sb.WriteString(fmt.Sprintf("- %s\n", constraint))
			}
		}
		sb.WriteString("\n---\n")
	}

	return sb.String()
}

// BuildCompactPlanManagerPrompt builds a plan manager prompt using compact plan formatting.
// This is useful when you need a less verbose prompt compared to Build() which uses detailed JSON output.
func (b *PlanningBuilder) BuildCompactPlanManagerPrompt(objective string, plans []CandidatePlanInfo, strategyNames []string) string {
	plansSection := b.FormatCompactPlansWithContext(plans, strategyNames)
	return fmt.Sprintf(PlanManagerPromptTemplate, objective, plansSection)
}

// PlanManagerPromptTemplate is the prompt for the coordinator-manager in multi-pass mode.
// It receives all candidate plans and must select the best one or merge them.
const PlanManagerPromptTemplate = `You are a senior technical lead evaluating multiple implementation plans.

## Objective
%s

## Candidate Plans
Three different planning strategies have produced the following plans:

%s

## Your Task

Evaluate each plan based on:
1. **Parallelism potential**: How many tasks can run concurrently?
2. **Task granularity**: Are tasks appropriately sized (prefer smaller, focused tasks)?
3. **Dependency structure**: Is the dependency graph sensible and minimal?
4. **File ownership**: Do tasks have clear, non-overlapping file assignments?
5. **Completeness**: Does the plan fully address the objective?
6. **Risk mitigation**: Are constraints and risks properly identified?

## Decision

You must either:
1. **Select** the best plan as-is, OR
2. **Merge** the best elements from multiple plans into a superior plan

## Output

Write your final plan to ` + "`" + PlanFileName + "`" + ` using the JSON schema below.

### Plan JSON Schema

Write a JSON file with this exact structure at the root level (do NOT wrap in a "plan" object):
- "summary": Brief executive summary (string)
- "tasks": Array of task objects, each with:
  - "id": Unique identifier like "task-1-setup" (string)
  - "title": Short title (string)
  - "description": Detailed instructions for another Claude instance to execute independently (string)
  - "files": Files this task will modify (array of strings)
  - "depends_on": IDs of tasks that must complete first (array of strings, empty for independent tasks)
  - "priority": Lower = higher priority within dependency level (number)
  - "est_complexity": "low", "medium", or "high" (string)
- "insights": Key findings about the codebase (array of strings)
- "constraints": Risks or constraints to consider (array of strings)

Before the plan file, output your reasoning in this format:
<plan_decision>
{
  "action": "select" or "merge",
  "selected_index": 0-2 (if select) or -1 (if merge),
  "reasoning": "Brief explanation of your decision",
  "plan_scores": [
    {"strategy": "maximize-parallelism", "score": 1-10, "strengths": "...", "weaknesses": "..."},
    {"strategy": "minimize-complexity", "score": 1-10, "strengths": "...", "weaknesses": "..."},
    {"strategy": "balanced-approach", "score": 1-10, "strengths": "...", "weaknesses": "..."}
  ]
}
</plan_decision>

Then write the final plan file.`

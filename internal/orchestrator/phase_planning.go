// Package orchestrator provides planning phase implementation for ultraplan execution.
package orchestrator

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// PlanningPhase handles the planning phase of ultraplan execution.
// It implements the PhaseHandler interface and encapsulates all planning-related
// logic including plan generation, dependency analysis, parallelization optimization,
// and plan validation.
type PlanningPhase struct {
	orch        *Orchestrator
	baseSession *Session
	callbacks   *CoordinatorCallbacks

	// State tracking
	mu              sync.RWMutex
	planningInstID  string       // Instance ID of the planning coordinator
	result          *PhaseResult // Cached result after completion
	progress        PhaseProgress
	completed       bool
	plannerStarted  bool
}

// NewPlanningPhase creates a new PlanningPhase handler.
func NewPlanningPhase(orch *Orchestrator, baseSession *Session) *PlanningPhase {
	return &PlanningPhase{
		orch:        orch,
		baseSession: baseSession,
		progress: PhaseProgress{
			Phase:     PhasePlanning,
			Completed: 0,
			Total:     1, // Planning is a single unit of work
			Message:   "Preparing to plan",
		},
	}
}

// SetCallbacks sets the callbacks for phase events.
func (p *PlanningPhase) SetCallbacks(cb *CoordinatorCallbacks) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callbacks = cb
}

// Name returns the name of this phase.
func (p *PlanningPhase) Name() UltraPlanPhase {
	return PhasePlanning
}

// CanExecute returns true if planning can be executed in the current state.
// Planning requires an objective to be set and no existing plan.
func (p *PlanningPhase) CanExecute(ctx context.Context, session *UltraPlanSession) bool {
	if session == nil {
		return false
	}
	// Can execute if we have an objective and no plan yet
	return session.Objective != "" && session.Plan == nil
}

// Execute runs the planning phase logic.
// This creates a planner instance that explores the codebase and generates a plan.
// The method is non-blocking; use GetResult/GetProgress to monitor status.
func (p *PlanningPhase) Execute(ctx context.Context, session *UltraPlanSession) error {
	if !p.CanExecute(ctx, session) {
		return fmt.Errorf("cannot execute planning: prerequisites not met")
	}

	p.mu.Lock()
	if p.plannerStarted {
		p.mu.Unlock()
		return fmt.Errorf("planning already started")
	}
	p.plannerStarted = true
	p.progress.Message = "Starting planner instance"
	p.mu.Unlock()

	// Create the planning prompt
	prompt := fmt.Sprintf(PlanningPromptTemplate, session.Objective)

	// Create a coordinator instance for planning
	inst, err := p.orch.AddInstance(p.baseSession, prompt)
	if err != nil {
		p.setError(fmt.Sprintf("failed to create planning instance: %v", err))
		return fmt.Errorf("failed to create planning instance: %w", err)
	}

	p.mu.Lock()
	p.planningInstID = inst.ID
	session.CoordinatorID = inst.ID
	p.progress.Message = "Planner instance created"
	p.mu.Unlock()

	// Start the instance
	if err := p.orch.StartInstance(inst); err != nil {
		p.setError(fmt.Sprintf("failed to start planning instance: %v", err))
		return fmt.Errorf("failed to start planning instance: %w", err)
	}

	p.mu.Lock()
	p.progress.Message = "Planner running - exploring codebase"
	p.mu.Unlock()

	// The TUI will handle monitoring; the planning instance writes a plan file when done
	return nil
}

// GetResult returns the result of the planning phase.
// Returns an error if planning has not completed.
func (p *PlanningPhase) GetResult(ctx context.Context) (PhaseResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.completed {
		return PhaseResult{}, fmt.Errorf("planning not yet completed")
	}

	if p.result == nil {
		return PhaseResult{Success: false, Error: "no result available"}, nil
	}

	return *p.result, nil
}

// GetProgress returns the current progress of the planning phase.
func (p *PlanningPhase) GetProgress(ctx context.Context) PhaseProgress {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.progress
}

// GetPlanningInstanceID returns the instance ID of the planning coordinator.
func (p *PlanningPhase) GetPlanningInstanceID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.planningInstID
}

// setError marks the phase as completed with an error.
func (p *PlanningPhase) setError(errMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed = true
	p.result = &PhaseResult{
		Success: false,
		Error:   errMsg,
	}
	p.progress.Message = "Planning failed: " + errMsg
}

// ProcessPlanFromFile reads and processes a plan from the planning instance's worktree.
// This should be called when the planning instance completes.
func (p *PlanningPhase) ProcessPlanFromFile(session *UltraPlanSession, worktreePath string) (*PlanSpec, error) {
	planPath := PlanFilePath(worktreePath)

	plan, err := ParsePlanFromFile(planPath, session.Objective)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan from file: %w", err)
	}

	// Validate the plan
	if err := ValidatePlan(plan); err != nil {
		return nil, fmt.Errorf("invalid plan: %w", err)
	}

	// Optimize parallelization if needed
	plan = p.optimizeParallelization(plan, session.Config)

	// Mark as completed
	p.mu.Lock()
	p.completed = true
	p.result = &PhaseResult{
		Success: true,
		Data:    plan,
	}
	p.progress.Completed = 1
	p.progress.Message = "Planning complete"
	p.mu.Unlock()

	return plan, nil
}

// SetPlan sets a plan directly (used when plan is provided externally or after editing).
// This validates the plan and marks the phase as complete.
func (p *PlanningPhase) SetPlan(session *UltraPlanSession, plan *PlanSpec) error {
	if err := ValidatePlan(plan); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}

	// Ensure execution order is calculated
	if len(plan.ExecutionOrder) == 0 {
		plan.ExecutionOrder = CalculateExecutionOrder(plan.Tasks, plan.DependencyGraph)
	}

	// Optimize parallelization
	plan = p.optimizeParallelization(plan, session.Config)

	// Update session
	session.Plan = plan

	// Mark as completed
	p.mu.Lock()
	p.completed = true
	p.result = &PhaseResult{
		Success: true,
		Data:    plan,
	}
	p.progress.Completed = 1
	p.progress.Message = "Plan set successfully"
	p.mu.Unlock()

	// Notify callbacks
	p.mu.RLock()
	cb := p.callbacks
	p.mu.RUnlock()
	if cb != nil && cb.OnPlanReady != nil {
		cb.OnPlanReady(plan)
	}

	return nil
}

// optimizeParallelization adjusts the execution order based on configuration.
// It ensures MaxParallel constraints are respected in the execution groups.
func (p *PlanningPhase) optimizeParallelization(plan *PlanSpec, config UltraPlanConfig) *PlanSpec {
	if config.MaxParallel <= 0 {
		// Unlimited parallelism, no changes needed
		return plan
	}

	// Split large groups to respect MaxParallel constraint
	var optimizedOrder [][]string
	for _, group := range plan.ExecutionOrder {
		if len(group) <= config.MaxParallel {
			optimizedOrder = append(optimizedOrder, group)
		} else {
			// Split the group into smaller batches
			for i := 0; i < len(group); i += config.MaxParallel {
				end := i + config.MaxParallel
				if end > len(group) {
					end = len(group)
				}
				optimizedOrder = append(optimizedOrder, group[i:end])
			}
		}
	}

	plan.ExecutionOrder = optimizedOrder
	return plan
}

// EstimateResources returns a resource estimate for executing the plan.
// This helps users understand what resources will be needed.
func (p *PlanningPhase) EstimateResources(plan *PlanSpec, config UltraPlanConfig) ResourceEstimate {
	if plan == nil {
		return ResourceEstimate{}
	}

	estimate := ResourceEstimate{
		TotalTasks:    len(plan.Tasks),
		TotalGroups:   len(plan.ExecutionOrder),
		MaxParallel:   config.MaxParallel,
		EstimatedTime: "varies based on task complexity",
	}

	// Count tasks by complexity
	for _, task := range plan.Tasks {
		switch task.EstComplexity {
		case ComplexityHigh:
			estimate.HighComplexityTasks++
		case ComplexityMedium:
			estimate.MediumComplexityTasks++
		case ComplexityLow:
			estimate.LowComplexityTasks++
		}
	}

	// Calculate peak parallelism needed
	for _, group := range plan.ExecutionOrder {
		if len(group) > estimate.PeakParallelism {
			estimate.PeakParallelism = len(group)
		}
	}

	// Effective parallelism is min of peak and max configured
	if config.MaxParallel > 0 && estimate.PeakParallelism > config.MaxParallel {
		estimate.PeakParallelism = config.MaxParallel
	}

	return estimate
}

// ResourceEstimate contains resource estimates for plan execution.
type ResourceEstimate struct {
	TotalTasks            int    `json:"total_tasks"`
	TotalGroups           int    `json:"total_groups"`
	MaxParallel           int    `json:"max_parallel"`
	PeakParallelism       int    `json:"peak_parallelism"`
	HighComplexityTasks   int    `json:"high_complexity_tasks"`
	MediumComplexityTasks int    `json:"medium_complexity_tasks"`
	LowComplexityTasks    int    `json:"low_complexity_tasks"`
	EstimatedTime         string `json:"estimated_time"`
}

// CalculateExecutionOrder performs a topological sort and groups tasks that can run in parallel.
// This is exported to allow external use for plan manipulation.
func CalculateExecutionOrder(tasks []PlannedTask, deps map[string][]string) [][]string {
	return calculateExecutionOrder(tasks, deps)
}

// AnalyzeDependencies analyzes the dependency graph and returns insights.
// This helps identify potential issues or optimization opportunities.
func AnalyzeDependencies(plan *PlanSpec) DependencyAnalysis {
	if plan == nil {
		return DependencyAnalysis{}
	}

	analysis := DependencyAnalysis{
		TotalDependencies: 0,
		TasksWithNoDeps:   make([]string, 0),
		CriticalPath:      make([]string, 0),
		PotentialBottlenecks: make([]string, 0),
	}

	// Count dependencies and find tasks with no deps
	depCounts := make(map[string]int)
	reverseDeps := make(map[string][]string) // Which tasks depend on this task

	for _, task := range plan.Tasks {
		analysis.TotalDependencies += len(task.DependsOn)
		depCounts[task.ID] = len(task.DependsOn)

		if len(task.DependsOn) == 0 {
			analysis.TasksWithNoDeps = append(analysis.TasksWithNoDeps, task.ID)
		}

		for _, depID := range task.DependsOn {
			reverseDeps[depID] = append(reverseDeps[depID], task.ID)
		}
	}

	// Find potential bottlenecks (tasks that many others depend on)
	for taskID, dependents := range reverseDeps {
		if len(dependents) >= 3 {
			analysis.PotentialBottlenecks = append(analysis.PotentialBottlenecks, taskID)
		}
	}

	// Calculate critical path (longest dependency chain)
	analysis.CriticalPath = findCriticalPath(plan.Tasks, plan.DependencyGraph)

	// Estimate parallelism ratio
	if len(plan.Tasks) > 0 && len(plan.ExecutionOrder) > 0 {
		avgGroupSize := float64(len(plan.Tasks)) / float64(len(plan.ExecutionOrder))
		analysis.ParallelismRatio = avgGroupSize
	}

	return analysis
}

// DependencyAnalysis contains insights about the plan's dependency structure.
type DependencyAnalysis struct {
	TotalDependencies    int      `json:"total_dependencies"`
	TasksWithNoDeps      []string `json:"tasks_with_no_deps"`
	CriticalPath         []string `json:"critical_path"`
	PotentialBottlenecks []string `json:"potential_bottlenecks"`
	ParallelismRatio     float64  `json:"parallelism_ratio"`
}

// findCriticalPath finds the longest dependency chain in the task graph.
func findCriticalPath(tasks []PlannedTask, deps map[string][]string) []string {
	// Build task map for quick lookup
	taskMap := make(map[string]*PlannedTask)
	for i := range tasks {
		taskMap[tasks[i].ID] = &tasks[i]
	}

	// Use memoization for path lengths
	pathLengths := make(map[string]int)
	pathNext := make(map[string]string) // Next task in the critical path from this task

	// Calculate longest path starting from each task using DFS
	var dfs func(taskID string) int
	dfs = func(taskID string) int {
		if length, ok := pathLengths[taskID]; ok {
			return length
		}

		maxLength := 0
		maxNext := ""

		// Find all tasks that depend on this one
		for _, t := range tasks {
			for _, depID := range t.DependsOn {
				if depID == taskID {
					length := dfs(t.ID)
					if length > maxLength {
						maxLength = length
						maxNext = t.ID
					}
				}
			}
		}

		pathLengths[taskID] = maxLength + 1
		pathNext[taskID] = maxNext
		return maxLength + 1
	}

	// Find the starting task with the longest path
	var startTask string
	maxPath := 0
	for _, task := range tasks {
		length := dfs(task.ID)
		if length > maxPath {
			maxPath = length
			startTask = task.ID
		}
	}

	// Build the critical path
	var criticalPath []string
	current := startTask
	for current != "" {
		criticalPath = append(criticalPath, current)
		current = pathNext[current]
	}

	return criticalPath
}

// ValidatePlanStructure performs deep validation of plan structure.
// This goes beyond basic validation to check for semantic issues.
func ValidatePlanStructure(plan *PlanSpec) []ValidationIssue {
	if plan == nil {
		return []ValidationIssue{{
			Severity: "error",
			Message:  "plan is nil",
		}}
	}

	var issues []ValidationIssue

	// Basic validation
	if err := ValidatePlan(plan); err != nil {
		issues = append(issues, ValidationIssue{
			Severity: "error",
			Message:  err.Error(),
		})
	}

	// Check for empty task descriptions
	for _, task := range plan.Tasks {
		if task.Description == "" {
			issues = append(issues, ValidationIssue{
				Severity: "warning",
				TaskID:   task.ID,
				Message:  "task has empty description",
			})
		}
		if task.Title == "" {
			issues = append(issues, ValidationIssue{
				Severity: "warning",
				TaskID:   task.ID,
				Message:  "task has empty title",
			})
		}
	}

	// Check for file ownership conflicts
	fileOwners := make(map[string][]string)
	for _, task := range plan.Tasks {
		for _, file := range task.Files {
			fileOwners[file] = append(fileOwners[file], task.ID)
		}
	}
	for file, owners := range fileOwners {
		if len(owners) > 1 {
			// Check if tasks are in the same execution group
			if tasksInSameGroup(owners, plan.ExecutionOrder) {
				issues = append(issues, ValidationIssue{
					Severity: "warning",
					Message:  fmt.Sprintf("file %s is modified by multiple parallel tasks: %v", file, owners),
				})
			}
		}
	}

	// Check for self-dependencies
	for _, task := range plan.Tasks {
		for _, depID := range task.DependsOn {
			if depID == task.ID {
				issues = append(issues, ValidationIssue{
					Severity: "error",
					TaskID:   task.ID,
					Message:  "task depends on itself",
				})
			}
		}
	}

	return issues
}

// ValidationIssue represents an issue found during plan validation.
type ValidationIssue struct {
	Severity string `json:"severity"` // "error", "warning", "info"
	TaskID   string `json:"task_id,omitempty"`
	Message  string `json:"message"`
}

// tasksInSameGroup checks if all given task IDs are in the same execution group.
func tasksInSameGroup(taskIDs []string, executionOrder [][]string) bool {
	for _, group := range executionOrder {
		groupSet := make(map[string]bool)
		for _, id := range group {
			groupSet[id] = true
		}

		count := 0
		for _, taskID := range taskIDs {
			if groupSet[taskID] {
				count++
			}
		}

		if count > 1 {
			return true
		}
	}
	return false
}

// MonitorPlanningInstance monitors the planning instance and processes the plan when ready.
// This should be called in a goroutine to poll for completion.
func (p *PlanningPhase) MonitorPlanningInstance(ctx context.Context, session *UltraPlanSession, checkInterval time.Duration) (*PlanSpec, error) {
	p.mu.RLock()
	instID := p.planningInstID
	p.mu.RUnlock()

	if instID == "" {
		return nil, fmt.Errorf("no planning instance started")
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case <-ticker.C:
			inst := p.orch.GetInstance(instID)
			if inst == nil {
				return nil, fmt.Errorf("planning instance not found")
			}

			// Check for plan file
			if inst.WorktreePath != "" {
				planPath := PlanFilePath(inst.WorktreePath)
				if _, err := os.Stat(planPath); err == nil {
					// Plan file exists, process it
					plan, err := p.ProcessPlanFromFile(session, inst.WorktreePath)
					if err != nil {
						// Plan exists but is invalid - keep waiting for a valid one
						p.mu.Lock()
						p.progress.Message = "Waiting for valid plan file..."
						p.mu.Unlock()
						continue
					}
					return plan, nil
				}
			}

			// Update progress message based on instance status
			p.updateProgressFromInstance(inst)

			// Check for terminal states
			switch inst.Status {
			case StatusError, StatusTimeout, StatusStuck:
				p.setError(fmt.Sprintf("planning instance %s: %s", inst.ID, inst.Status))
				return nil, fmt.Errorf("planning instance failed: %s", inst.Status)
			}
		}
	}
}

// updateProgressFromInstance updates progress message based on instance status.
func (p *PlanningPhase) updateProgressFromInstance(inst *Instance) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch inst.Status {
	case StatusPending:
		p.progress.Message = "Planning instance pending..."
	case StatusWorking:
		p.progress.Message = "Planner exploring codebase and generating plan..."
	case StatusWaitingInput:
		p.progress.Message = "Planner waiting for input..."
	case StatusCompleted:
		p.progress.Message = "Planning instance completed, processing plan..."
	default:
		p.progress.Message = fmt.Sprintf("Planning status: %s", inst.Status)
	}
}

// IsCompleted returns whether the planning phase has completed.
func (p *PlanningPhase) IsCompleted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.completed
}

// Reset resets the planning phase to allow re-execution.
func (p *PlanningPhase) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.planningInstID = ""
	p.result = nil
	p.completed = false
	p.plannerStarted = false
	p.progress = PhaseProgress{
		Phase:     PhasePlanning,
		Completed: 0,
		Total:     1,
		Message:   "Ready to plan",
	}
}

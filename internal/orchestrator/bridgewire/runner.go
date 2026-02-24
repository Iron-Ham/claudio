package bridgewire

import (
	"context"
	"fmt"
	"maps"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/prompt"
	"github.com/Iron-Ham/claudio/internal/pipeline"
	"github.com/Iron-Ham/claudio/internal/team"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// PipelineRunnerConfig holds the dependencies for constructing a PipelineRunner.
type PipelineRunnerConfig struct {
	Orch     *orchestrator.Orchestrator
	Session  *orchestrator.Session
	Verifier orchestrator.Verifier
	Plan     *orchestrator.PlanSpec // orchestrator's PlanSpec (converted internally)
	Bus      *event.Bus
	Logger   *logging.Logger
	Recorder bridge.SessionRecorder
	BaseDir  string // Base directory for pipeline state files (defaults to Session.BaseRepo)

	MaxParallel int // from UltraPlanConfig.MaxParallel

	// RoleOverrides maps team roles to per-invocation CLI flag overrides.
	// When set, execution instances for a given role will use these overrides
	// for permission mode, model, tool restrictions, etc.
	RoleOverrides map[team.Role]ai.StartOptions

	// SubprocessMode uses direct subprocess execution (stream-json) instead of tmux.
	SubprocessMode bool
	// CommandName is the AI backend CLI binary (default: "claude").
	CommandName string
}

// PipelineRunner implements orchestrator.ExecutionRunner using the
// Pipeline-based execution backend (Orchestration 2.0).
//
// It encapsulates the full lifecycle: plan conversion, pipeline creation,
// decomposition, and PipelineExecutor wiring.
type PipelineRunner struct {
	pipe *pipeline.Pipeline
	exec *PipelineExecutor
}

// NewPipelineRunner creates a PipelineRunner from the given config.
// It converts the orchestrator PlanSpec to ultraplan.PlanSpec, creates the
// Pipeline, decomposes the plan into teams, and wires a PipelineExecutor.
func NewPipelineRunner(cfg PipelineRunnerConfig) (*PipelineRunner, error) {
	if cfg.Plan == nil {
		return nil, fmt.Errorf("bridgewire: PipelineRunner requires a non-nil Plan")
	}
	if cfg.Bus == nil {
		return nil, fmt.Errorf("bridgewire: PipelineRunner requires a non-nil Bus")
	}
	if cfg.Orch == nil {
		return nil, fmt.Errorf("bridgewire: PipelineRunner requires a non-nil Orch")
	}
	if cfg.Session == nil {
		return nil, fmt.Errorf("bridgewire: PipelineRunner requires a non-nil Session")
	}

	// Convert orchestrator.PlanSpec → ultraplan.PlanSpec
	uplan := convertPlan(cfg.Plan)

	// Use explicit BaseDir if provided, otherwise fall back to Session.BaseRepo
	baseDir := cfg.BaseDir
	if baseDir == "" {
		baseDir = cfg.Session.BaseRepo
	}

	// Create the pipeline
	pipe, err := pipeline.NewPipeline(pipeline.PipelineConfig{
		Bus:     cfg.Bus,
		BaseDir: baseDir,
		Plan:    uplan,
	})
	if err != nil {
		return nil, fmt.Errorf("bridgewire: create pipeline: %w", err)
	}

	// Decompose the plan into execution teams only (no planning/review/consolidation
	// phases — those are handled by the Coordinator's existing methods).
	maxPar := cfg.MaxParallel
	if maxPar < 1 {
		maxPar = 3
	}

	_, err = pipe.Decompose(pipeline.DecomposeConfig{
		DefaultTeamSize:  maxPar,
		MinTeamInstances: 1,
		MaxTeamInstances: maxPar,
	})
	if err != nil {
		return nil, fmt.Errorf("bridgewire: decompose plan: %w", err)
	}

	// Create the PipelineExecutor
	logger := cfg.Logger
	if logger == nil {
		logger = logging.NopLogger()
	}

	// Generate orchestration system prompt file and inject it into execution role
	// overrides. This gives all execution instances the guidelines and completion
	// protocol via --append-system-prompt-file, keeping task prompts clean.
	// This is a hard error: without the system prompt, instances won't know the
	// completion protocol (writing the sentinel file), causing all tasks to time out.
	roleOverrides := cfg.RoleOverrides
	sysPromptPath, sysErr := prompt.WriteOrchestrationSystemPrompt(baseDir)
	if sysErr != nil {
		return nil, fmt.Errorf("bridgewire: write orchestration system prompt: %w", sysErr)
	}
	roleOverrides = injectSystemPrompt(roleOverrides, sysPromptPath)

	var exec *PipelineExecutor
	if cfg.SubprocessMode {
		commandName := cfg.CommandName
		if commandName == "" {
			commandName = "claude"
		}
		exec, err = NewPipelineExecutor(PipelineExecutorConfig{
			Factory: NewSubprocessFactory(cfg.Orch, cfg.Session, commandName, ai.StartOptions{}, logger),
			FactoryWithOverrides: func(overrides ai.StartOptions) bridge.InstanceFactory {
				return NewSubprocessFactory(cfg.Orch, cfg.Session, commandName, overrides, logger)
			},
			RoleOverrides: roleOverrides,
			Checker:       NewCompletionChecker(cfg.Verifier),
			Bus:           cfg.Bus,
			Pipeline:      pipe,
			Recorder:      cfg.Recorder,
			Logger:        logger,
		})
	} else {
		exec, err = NewPipelineExecutorFromOrch(
			cfg.Orch, cfg.Session, cfg.Verifier,
			cfg.Bus, pipe, cfg.Recorder, logger,
			roleOverrides,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("bridgewire: create executor: %w", err)
	}

	return &PipelineRunner{
		pipe: pipe,
		exec: exec,
	}, nil
}

// Start begins execution: starts the executor (which subscribes to pipeline
// phase events and creates bridges) then starts the pipeline itself.
func (r *PipelineRunner) Start(ctx context.Context) error {
	if err := r.exec.Start(ctx); err != nil {
		return fmt.Errorf("bridgewire: start executor: %w", err)
	}
	if err := r.pipe.Start(ctx); err != nil {
		r.exec.Stop()
		return fmt.Errorf("bridgewire: start pipeline: %w", err)
	}
	return nil
}

// Stop tears down both the executor and pipeline.
func (r *PipelineRunner) Stop() {
	r.exec.Stop()
	_ = r.pipe.Stop()
}

// convertPlan converts an orchestrator.PlanSpec to an ultraplan.PlanSpec.
// The two types have identical shapes (by design) so this is a field-by-field copy.
func convertPlan(src *orchestrator.PlanSpec) *ultraplan.PlanSpec {
	tasks := make([]ultraplan.PlannedTask, len(src.Tasks))
	for i, t := range src.Tasks {
		var files []string
		if len(t.Files) > 0 {
			files = make([]string, len(t.Files))
			copy(files, t.Files)
		}
		var deps []string
		if len(t.DependsOn) > 0 {
			deps = make([]string, len(t.DependsOn))
			copy(deps, t.DependsOn)
		}
		tasks[i] = ultraplan.PlannedTask{
			ID:            t.ID,
			Title:         t.Title,
			Description:   t.Description,
			Files:         files,
			DependsOn:     deps,
			Priority:      t.Priority,
			EstComplexity: ultraplan.TaskComplexity(t.EstComplexity),
			IssueURL:      t.IssueURL,
			NoCode:        t.NoCode,
		}
	}

	depGraph := make(map[string][]string, len(src.DependencyGraph))
	for k, v := range src.DependencyGraph {
		cp := make([]string, len(v))
		copy(cp, v)
		depGraph[k] = cp
	}

	execOrder := make([][]string, len(src.ExecutionOrder))
	for i, group := range src.ExecutionOrder {
		execOrder[i] = make([]string, len(group))
		copy(execOrder[i], group)
	}

	var insights []string
	if len(src.Insights) > 0 {
		insights = make([]string, len(src.Insights))
		copy(insights, src.Insights)
	}
	var constraints []string
	if len(src.Constraints) > 0 {
		constraints = make([]string, len(src.Constraints))
		copy(constraints, src.Constraints)
	}

	return &ultraplan.PlanSpec{
		ID:              src.ID,
		Objective:       src.Objective,
		Summary:         src.Summary,
		Tasks:           tasks,
		DependencyGraph: depGraph,
		ExecutionOrder:  execOrder,
		Insights:        insights,
		Constraints:     constraints,
		CreatedAt:       src.CreatedAt,
	}
}

// injectSystemPrompt ensures the execution role's overrides include the given
// system prompt file path. If the role already has an AppendSystemPromptFile
// set (explicitly by the caller), it is not overwritten.
//
// Returns a new map to avoid mutating the caller's data (defensive copy).
func injectSystemPrompt(overrides map[team.Role]ai.StartOptions, path string) map[team.Role]ai.StartOptions {
	result := make(map[team.Role]ai.StartOptions, len(overrides)+1)
	maps.Copy(result, overrides)
	execOverrides := result[team.RoleExecution]
	if execOverrides.AppendSystemPromptFile == "" {
		execOverrides.AppendSystemPromptFile = path
		result[team.RoleExecution] = execOverrides
	}
	return result
}

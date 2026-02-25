package tui

import (
	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/bridgewire"
	"github.com/spf13/viper"
)

// registerPipelineFactory registers a PipelineRunnerFactory on the given
// Coordinator. When UsePipeline is enabled in the config, the factory is
// called lazily on the first StartExecution() call. This function exists in
// the TUI package because it is the only layer that can import both
// orchestrator and bridgewire without creating an import cycle.
func registerPipelineFactory(coordinator *orchestrator.Coordinator, orch *orchestrator.Orchestrator, logger *logging.Logger, subprocessMode bool, commandName string) {
	// Build backend defaults for subprocess mode from config. The tmux path
	// gets these from ClaudeBackend; subprocess bypasses ClaudeBackend so
	// it needs the resolved config values passed explicitly.
	backendDefaults := buildBackendDefaults()

	coordinator.SetPipelineFactory(func(deps orchestrator.PipelineRunnerDeps) (orchestrator.ExecutionRunner, error) {
		recorder := bridgewire.NewSessionRecorder(bridgewire.SessionRecorderDeps{})
		return bridgewire.NewPipelineRunner(bridgewire.PipelineRunnerConfig{
			Orch:            deps.Orch,
			Session:         deps.Session,
			Verifier:        deps.Verifier,
			Plan:            deps.Plan,
			Bus:             orch.EventBus(),
			Logger:          logger,
			Recorder:        recorder,
			MaxParallel:     deps.MaxParallel,
			SubprocessMode:  subprocessMode,
			CommandName:     commandName,
			BackendDefaults: backendDefaults,
		})
	})
}

// buildBackendDefaults constructs ai.StartOptions from the Claude backend
// config. This replicates the resolution logic from
// config.ClaudeBackendConfig.ResolvedPermissionMode and the default merging
// from ai.ClaudeBackend.BuildStartCommand.
func buildBackendDefaults() ai.StartOptions {
	permMode := viper.GetString("ai.claude.permission_mode")
	if permMode == "" && viper.GetBool("ai.claude.skip_permissions") {
		permMode = "bypass"
	}
	return ai.StartOptions{
		PermissionMode:  permMode,
		Model:           viper.GetString("ai.claude.model"),
		MaxTurns:        viper.GetInt("ai.claude.max_turns"),
		AllowedTools:    viper.GetStringSlice("ai.claude.allowed_tools"),
		DisallowedTools: viper.GetStringSlice("ai.claude.disallowed_tools"),
	}
}

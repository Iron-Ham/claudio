package tui

import (
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/bridgewire"
)

// registerPipelineFactory registers a PipelineRunnerFactory on the given
// Coordinator. When UsePipeline is enabled in the config, the factory is
// called lazily on the first StartExecution() call. This function exists in
// the TUI package because it is the only layer that can import both
// orchestrator and bridgewire without creating an import cycle.
func registerPipelineFactory(coordinator *orchestrator.Coordinator, orch *orchestrator.Orchestrator, logger *logging.Logger) {
	coordinator.SetPipelineFactory(func(deps orchestrator.PipelineRunnerDeps) (orchestrator.ExecutionRunner, error) {
		recorder := bridgewire.NewSessionRecorder(bridgewire.SessionRecorderDeps{})
		return bridgewire.NewPipelineRunner(bridgewire.PipelineRunnerConfig{
			Orch:        deps.Orch,
			Session:     deps.Session,
			Verifier:    deps.Verifier,
			Plan:        deps.Plan,
			Bus:         orch.EventBus(),
			Logger:      logger,
			Recorder:    recorder,
			MaxParallel: deps.MaxParallel,
		})
	})
}

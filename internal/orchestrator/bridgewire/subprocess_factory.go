package bridgewire

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/streamjson"
)

// Compile-time interface compliance check.
var _ bridge.InstanceFactory = (*subprocessFactory)(nil)

// subprocessFactory adapts an Orchestrator to bridge.InstanceFactory using
// direct subprocess execution (claude --print --output-format stream-json)
// instead of tmux sessions.
//
// CreateInstance delegates to the orchestrator for worktree/branch creation
// (identical to the tmux path). StartInstance writes the prompt to a file,
// builds subprocess args from the start overrides, and launches
// streamjson.RunSubprocess in a goroutine.
//
// The subprocess writes the sentinel completion file as part of its normal
// execution (instructed by the orchestration system prompt), so the existing
// CompletionChecker (sentinel file polling) works unchanged.
type subprocessFactory struct {
	orch           *orchestrator.Orchestrator
	session        *orchestrator.Session
	commandName    string
	startOverrides ai.StartOptions
	logger         *logging.Logger
	parentCtx      context.Context

	mu      sync.Mutex
	wg      sync.WaitGroup
	cancels map[string]context.CancelFunc // instanceID → cancel
	stopped bool
}

// NewSubprocessFactory creates a bridge.InstanceFactory that uses direct
// subprocess execution instead of tmux sessions. The commandName is the
// Claude CLI binary (typically "claude"). The parentCtx is used as the
// parent for all subprocess contexts, enabling cancellation from the
// pipeline lifecycle.
func NewSubprocessFactory(
	orch *orchestrator.Orchestrator,
	session *orchestrator.Session,
	commandName string,
	overrides ai.StartOptions,
	logger *logging.Logger,
) bridge.InstanceFactory {
	if logger == nil {
		logger = logging.NopLogger()
	}
	return &subprocessFactory{
		orch:           orch,
		session:        session,
		commandName:    commandName,
		startOverrides: overrides,
		logger:         logger,
		parentCtx:      context.Background(),
		cancels:        make(map[string]context.CancelFunc),
	}
}

// NewSubprocessFactoryWithContext is like NewSubprocessFactory but derives
// subprocess contexts from the given parent, enabling pipeline-level
// cancellation to propagate to running subprocesses.
func NewSubprocessFactoryWithContext(
	ctx context.Context,
	orch *orchestrator.Orchestrator,
	session *orchestrator.Session,
	commandName string,
	overrides ai.StartOptions,
	logger *logging.Logger,
) bridge.InstanceFactory {
	if logger == nil {
		logger = logging.NopLogger()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &subprocessFactory{
		orch:           orch,
		session:        session,
		commandName:    commandName,
		startOverrides: overrides,
		logger:         logger,
		parentCtx:      ctx,
		cancels:        make(map[string]context.CancelFunc),
	}
}

// CreateInstance delegates to the orchestrator to create a worktree and branch.
// This is identical to the tmux path — the difference is in StartInstance.
//
// Coverage: Requires real orchestrator infrastructure (worktrees); tested via integration tests.
func (f *subprocessFactory) CreateInstance(taskPrompt string) (bridge.Instance, error) {
	inst, err := f.orch.AddInstance(f.session, taskPrompt)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	return &orchInstance{inst: inst}, nil
}

// StartInstance writes the task prompt to a file and launches a Claude Code
// subprocess in stream-json mode. The subprocess runs in a goroutine and
// writes the sentinel completion file when finished.
//
// The instance's orchestrator status is NOT mutated here. Unlike the tmux
// path (which delegates to orch.StartInstanceWithOverrides for synchronized
// status updates), the subprocess path relies on the bridge's
// CompletionChecker for task lifecycle tracking. Direct mutation of
// orchestrator.Instance.Status from outside the orchestrator would be a
// data race.
//
// Coverage: Requires real orchestrator infrastructure; tested via integration tests.
func (f *subprocessFactory) StartInstance(inst bridge.Instance) error {
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		return fmt.Errorf("start instance: factory is stopped")
	}
	f.mu.Unlock()

	orchInst := f.orch.GetInstance(inst.ID())
	if orchInst == nil {
		return fmt.Errorf("start instance: %q not found", inst.ID())
	}

	// Write the prompt to a file in the worktree.
	promptFile := filepath.Join(inst.WorktreePath(), ".claude-task-prompt")
	if err := os.WriteFile(promptFile, []byte(orchInst.Task), 0o600); err != nil {
		return fmt.Errorf("write prompt file: %w", err)
	}

	// Build subprocess arguments from the start overrides.
	opts := startOptionsToSubprocessOptions(f.startOverrides)
	args := streamjson.BuildSubprocessArgs(promptFile, opts)

	// Create a cancellable context derived from the parent context.
	ctx, cancel := context.WithCancel(f.parentCtx)
	f.mu.Lock()
	if f.stopped {
		f.mu.Unlock()
		cancel()
		return fmt.Errorf("start instance: factory stopped during setup")
	}
	f.cancels[inst.ID()] = cancel
	f.wg.Add(1)
	f.mu.Unlock()

	// Launch the subprocess in a goroutine. The bridge's monitor will detect
	// completion via the sentinel file (written by Claude Code before exiting).
	go func() {
		defer func() {
			cancel()
			f.mu.Lock()
			delete(f.cancels, inst.ID())
			f.mu.Unlock()
			// Clean up the prompt file.
			if err := os.Remove(promptFile); err != nil && !os.IsNotExist(err) {
				f.logger.Warn("subprocess: failed to remove prompt file",
					"instance", inst.ID(), "path", promptFile, "error", err)
			}
			f.wg.Done()
		}()

		result, err := streamjson.RunSubprocess(ctx, f.commandName, args, inst.WorktreePath())
		if err != nil {
			f.logger.Error("subprocess: execution failed",
				"instance", inst.ID(), "error", err)
			return
		}

		// Log stream parsing errors that don't surface as the top-level error.
		if result.ReadError != nil {
			f.logger.Warn("subprocess: stream read error",
				"instance", inst.ID(), "error", result.ReadError)
		}

		if result.Result != nil {
			f.logger.Info("subprocess: completed",
				"instance", inst.ID(),
				"exitCode", result.ExitCode,
				"totalCostUSD", result.Result.CostUSD,
				"totalTokens", result.Result.Usage.InputTokens+result.Result.Usage.OutputTokens)
		} else {
			f.logger.Info("subprocess: completed without result event",
				"instance", inst.ID(),
				"exitCode", result.ExitCode)
		}
	}()

	return nil
}

// Stop cancels all running subprocesses and waits for their goroutines to
// drain. This follows the project's "release locks before blocking on
// wg.Wait()" pattern to avoid deadlock with goroutine defers that acquire
// f.mu.
func (f *subprocessFactory) Stop() {
	f.mu.Lock()
	f.stopped = true
	// Copy and clear the cancel map to avoid holding the lock during
	// cancellation and wg.Wait().
	cancels := f.cancels
	f.cancels = make(map[string]context.CancelFunc)
	f.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
	f.wg.Wait()
}

// startOptionsToSubprocessOptions converts ai.StartOptions to
// streamjson.SubprocessOptions for the subprocess invocation.
// Slices are defensively copied to maintain the copy-on-return convention.
func startOptionsToSubprocessOptions(opts ai.StartOptions) streamjson.SubprocessOptions {
	var allowedTools []string
	if len(opts.AllowedTools) > 0 {
		allowedTools = make([]string, len(opts.AllowedTools))
		copy(allowedTools, opts.AllowedTools)
	}
	var disallowedTools []string
	if len(opts.DisallowedTools) > 0 {
		disallowedTools = make([]string, len(opts.DisallowedTools))
		copy(disallowedTools, opts.DisallowedTools)
	}
	return streamjson.SubprocessOptions{
		PermissionMode:         opts.PermissionMode,
		Model:                  opts.Model,
		MaxTurns:               opts.MaxTurns,
		AllowedTools:           allowedTools,
		DisallowedTools:        disallowedTools,
		AppendSystemPromptFile: opts.AppendSystemPromptFile,
		NoUserPrompt:           opts.NoUserPrompt,
		Worktree:               opts.Worktree,
	}
}

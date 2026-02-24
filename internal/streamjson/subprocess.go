package streamjson

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// SubprocessResult holds the outcome of a subprocess execution.
type SubprocessResult struct {
	// Result is the final ResultEvent from the stream (nil if stream ended without one).
	Result *ResultEvent
	// Events contains all events received during execution.
	Events []Event
	// ExitCode is the process exit code (-1 if the process was killed or not started).
	ExitCode int
	// ReadError is non-nil if stream parsing failed before EOF. This captures
	// corrupted JSON, pipe read errors, or buffer overflow conditions. Check
	// ctx.Err() to distinguish cancellation from genuine parse errors.
	ReadError error
}

// RunSubprocess launches a Claude Code subprocess with the given arguments,
// reads its stream-json output, and returns the collected results. The process
// is killed if the context is cancelled.
//
// The command should include all Claude Code flags (e.g., --print, --output-format stream-json,
// --permission-mode auto-accept, etc.). The prompt is passed via the promptFile argument.
//
// workDir sets the working directory for the subprocess.
//
// Returns an error when the process exits non-zero without producing a ResultEvent,
// so callers don't mistake a crashed subprocess for a successful empty execution.
// When a ResultEvent is present, the caller should inspect ExitCode directly.
func RunSubprocess(ctx context.Context, command string, args []string, workDir string) (*SubprocessResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir
	// Subprocess stderr is not captured; os/exec discards it when Stderr is nil.
	// In TUI contexts, writing to os.Stderr would corrupt the terminal display.

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start subprocess: %w", err)
	}

	// Parse NDJSON events from stdout
	reader := NewReader(stdout)
	var events []Event
	var result *ResultEvent
	var readError error

	for {
		event, readErr := reader.Next()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			// Capture the error for the caller instead of silently discarding it.
			// The process may have been killed (check ctx.Err()), or the stream
			// may contain malformed JSON.
			readError = readErr
			break
		}
		events = append(events, event)
		if r, ok := event.(*ResultEvent); ok {
			result = r
		}
	}

	// Wait for process to exit
	waitErr := cmd.Wait()
	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	subResult := &SubprocessResult{
		Result:    result,
		Events:    events,
		ExitCode:  exitCode,
		ReadError: readError,
	}

	// Return an error when the process failed without producing a result,
	// so callers don't mistake a crashed subprocess for a successful empty execution.
	if result == nil && exitCode != 0 {
		return subResult, fmt.Errorf("subprocess exited with code %d without producing a result", exitCode)
	}

	return subResult, nil
}

// BuildSubprocessArgs constructs the argument list for a Claude Code subprocess
// invocation in stream-json mode. This produces args suitable for exec.Cmd,
// not a shell string like BuildStartCommand.
//
// The returned args include -p (print mode), --output-format stream-json,
// and any additional flags from the options.
func BuildSubprocessArgs(promptFile string, opts SubprocessOptions) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
	}

	if opts.PermissionMode != "" {
		switch opts.PermissionMode {
		case "bypass":
			args = append(args, "--dangerously-skip-permissions")
		case "plan", "auto-accept":
			args = append(args, "--permission-mode", opts.PermissionMode)
		default:
			// "default" or unrecognized: no permission flags.
			// See also: ClaudeBackend.buildPermissionFlags in ai/backend.go.
		}
	}

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", opts.MaxTurns))
	}

	for _, tool := range opts.AllowedTools {
		args = append(args, "--allowedTools", tool)
	}
	for _, tool := range opts.DisallowedTools {
		args = append(args, "--disallowedTools", tool)
	}

	if opts.AppendSystemPromptFile != "" {
		args = append(args, "--append-system-prompt-file", opts.AppendSystemPromptFile)
	}

	if opts.NoUserPrompt {
		args = append(args, "--no-user-prompt")
	}

	if opts.Worktree {
		args = append(args, "--worktree")
	}

	// Add the prompt file content via shell-safe argument
	args = append(args, "--prompt-file", promptFile)

	return args
}

// SubprocessOptions configures a subprocess invocation.
// See also: ai.StartOptions for per-invocation overrides in the interactive (tmux) path.
type SubprocessOptions struct {
	// PermissionMode controls Claude Code's permission handling
	// ("bypass", "plan", "auto-accept", "default").
	PermissionMode string
	// Model selects the AI model for this invocation.
	Model string
	// MaxTurns limits the number of agentic turns (0 = unlimited).
	MaxTurns int
	// AllowedTools permits specific tools without prompting.
	AllowedTools []string
	// DisallowedTools explicitly denies specific tools.
	DisallowedTools []string
	// AppendSystemPromptFile is a path to a file whose contents are appended to the system prompt.
	AppendSystemPromptFile string
	// NoUserPrompt prevents Claude from requesting user confirmation (for headless pipelines).
	NoUserPrompt bool
	// Worktree enables Claude Code's native --worktree flag for isolated execution.
	Worktree bool
}

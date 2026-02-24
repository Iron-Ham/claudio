package ai

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/instance/metrics"
)

// BackendName identifies a supported AI backend.
type BackendName string

const (
	BackendClaude BackendName = "claude"
	BackendCodex  BackendName = "codex"
)

// StartMode selects how a backend should be launched.
type StartMode int

const (
	// StartModeInteractive launches a long-lived interactive session.
	StartModeInteractive StartMode = iota
	// StartModeOneShot launches a single prompt and exits when done.
	StartModeOneShot
)

// OutputFormat controls the serialization format for non-interactive output.
type OutputFormat string

const (
	// OutputFormatText produces plain text output (default).
	OutputFormatText OutputFormat = "text"
	// OutputFormatJSON produces a single JSON object.
	OutputFormatJSON OutputFormat = "json"
	// OutputFormatStreamJSON produces newline-delimited JSON (NDJSON) with real-time events.
	OutputFormatStreamJSON OutputFormat = "stream-json"
)

// StartOptions configures backend start commands.
type StartOptions struct {
	PromptFile string
	SessionID  string
	Mode       StartMode
	// OutputOnly requests a non-interactive, print-only execution when supported.
	OutputOnly bool

	// Per-invocation overrides (take precedence over backend config when non-zero).

	// MaxTurns limits agentic turns for this invocation (0 = use backend default).
	MaxTurns int
	// AllowedTools permits specific tools without prompting for this invocation.
	AllowedTools []string
	// DisallowedTools explicitly denies specific tools for this invocation.
	DisallowedTools []string
	// PermissionMode overrides the backend's permission mode for this invocation.
	PermissionMode string
	// AppendSystemPromptFile is a path to a file whose contents are appended to the system prompt.
	AppendSystemPromptFile string
	// NoUserPrompt prevents Claude from ever requesting user confirmation (for headless pipelines).
	NoUserPrompt bool
	// Model overrides the model selection for this invocation.
	Model string
	// OutputFormat controls output serialization (only used with OutputOnly/--print).
	OutputFormat OutputFormat
}

// Backend provides backend-specific behavior for running AI sessions.
// Implementations include ClaudeBackend for Claude Code CLI and CodexBackend for Codex CLI.
type Backend interface {
	// Name returns the unique identifier for this backend (e.g., "claude", "codex").
	Name() BackendName

	// DisplayName returns a human-readable name for UI display (e.g., "Claude", "Codex").
	DisplayName() string

	// PromptFileName returns the filename used for prompt files (e.g., ".claude-prompt").
	PromptFileName() string

	// BuildStartCommand constructs the shell command to start a new AI session.
	// Returns an error if opts.PromptFile is empty.
	BuildStartCommand(opts StartOptions) (string, error)

	// BuildResumeCommand constructs the shell command to resume an existing session.
	// Returns an error if sessionID is empty.
	BuildResumeCommand(sessionID string) (string, error)

	// SupportsResume indicates whether the backend can resume previous sessions.
	SupportsResume() bool

	// SupportsExplicitSessionID indicates whether the backend accepts user-specified session IDs.
	// Claude supports this; Codex generates its own session IDs.
	SupportsExplicitSessionID() bool

	// Detector returns a state detector configured for this backend's output patterns.
	// The detector identifies states like waiting for input, permission prompts, or completion.
	Detector() detect.StateDetector

	// MetricsParser returns a parser for extracting token usage metrics from backend output.
	MetricsParser() *metrics.MetricsParser

	// EstimateCost calculates the estimated cost for the given token usage.
	// Returns (cost, true) if cost estimation is supported, or (0, false) otherwise.
	EstimateCost(inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool)

	// LocalConfigFiles returns the list of backend-specific local config files
	// (e.g., "CLAUDE.local.md") that should be copied to worktrees.
	LocalConfigFiles() []string
}

// ErrUnknownBackend is returned when the configured backend is unsupported.
var ErrUnknownBackend = fmt.Errorf("unknown AI backend")

// NewFromConfig builds a Backend from configuration.
func NewFromConfig(cfg *config.Config) (Backend, error) {
	if cfg == nil {
		return nil, fmt.Errorf("missing config")
	}

	switch strings.ToLower(cfg.AI.Backend) {
	case string(BackendClaude), "":
		return NewClaudeBackend(cfg.AI.Claude), nil
	case string(BackendCodex):
		return NewCodexBackend(cfg.AI.Codex), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnknownBackend, cfg.AI.Backend)
	}
}

// DefaultBackend returns a Claude backend with default settings.
func DefaultBackend() Backend {
	return NewClaudeBackend(config.ClaudeBackendConfig{
		Command:         "claude",
		SkipPermissions: true,
	})
}

// firstNonEmpty returns the first non-empty string from its arguments.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// firstPositive returns the first positive integer from its arguments.
func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

// mergeUnique combines two string slices, deduplicating entries.
func mergeUnique(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(a)+len(b))
	var result []string
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// ClaudeBackend implements Backend for Claude Code.
type ClaudeBackend struct {
	command            string
	permissionMode     string
	allowedTools       []string
	disallowedTools    []string
	maxTurns           int
	model              string
	appendSystemPrompt string
}

// NewClaudeBackend creates a Claude backend from config.
func NewClaudeBackend(cfg config.ClaudeBackendConfig) *ClaudeBackend {
	command := cfg.Command
	if command == "" {
		command = "claude"
	}
	return &ClaudeBackend{
		command:            command,
		permissionMode:     cfg.ResolvedPermissionMode(),
		allowedTools:       cfg.AllowedTools,
		disallowedTools:    cfg.DisallowedTools,
		maxTurns:           cfg.MaxTurns,
		model:              cfg.Model,
		appendSystemPrompt: cfg.AppendSystemPrompt,
	}
}

func (c *ClaudeBackend) Name() BackendName { return BackendClaude }

func (c *ClaudeBackend) DisplayName() string { return "Claude" }

func (c *ClaudeBackend) PromptFileName() string { return ".claude-prompt" }

func (c *ClaudeBackend) BuildStartCommand(opts StartOptions) (string, error) {
	if opts.PromptFile == "" {
		return "", fmt.Errorf("prompt file required")
	}

	cmd := c.command
	if opts.OutputOnly {
		cmd += " --print"
		if opts.OutputFormat != "" && opts.OutputFormat != OutputFormatText {
			cmd += fmt.Sprintf(" --output-format %s", string(opts.OutputFormat))
		}
	}

	// Permission mode: per-invocation overrides backend config.
	cmd += c.buildPermissionFlags(opts.PermissionMode)

	if opts.SessionID != "" {
		cmd += fmt.Sprintf(" --session-id %q", opts.SessionID)
	}

	// Model: per-invocation overrides backend config.
	if model := firstNonEmpty(opts.Model, c.model); model != "" {
		cmd += fmt.Sprintf(" --model %q", model)
	}

	// Max turns: per-invocation overrides backend config.
	if maxTurns := firstPositive(opts.MaxTurns, c.maxTurns); maxTurns > 0 {
		cmd += fmt.Sprintf(" --max-turns %d", maxTurns)
	}

	// Tool restrictions: merge per-invocation with backend config.
	for _, tool := range mergeUnique(opts.AllowedTools, c.allowedTools) {
		cmd += fmt.Sprintf(" --allowedTools %q", tool)
	}
	for _, tool := range mergeUnique(opts.DisallowedTools, c.disallowedTools) {
		cmd += fmt.Sprintf(" --disallowedTools %q", tool)
	}

	// System prompt additions.
	if opts.AppendSystemPromptFile != "" {
		cmd += fmt.Sprintf(" --append-system-prompt-file %q", opts.AppendSystemPromptFile)
	}
	if c.appendSystemPrompt != "" {
		cmd += fmt.Sprintf(" --append-system-prompt %q", c.appendSystemPrompt)
	}

	if opts.NoUserPrompt {
		cmd += " --no-user-prompt"
	}

	// Force in-process teammate mode to prevent Claude Code from spawning nested
	// tmux sessions. Claudio already runs instances inside tmux, so CC's auto-detection
	// sees $TMUX and defaults to split-pane mode, creating unmanageable nested sessions.
	cmd += " --teammate-mode in-process"

	return fmt.Sprintf("%s \"$(cat %q)\" && rm %q", cmd, opts.PromptFile, opts.PromptFile), nil
}

// buildPermissionFlags returns the CLI flags for permission mode.
// Per-invocation mode overrides the backend's configured mode.
func (c *ClaudeBackend) buildPermissionFlags(perInvocationMode string) string {
	mode := perInvocationMode
	if mode == "" {
		mode = c.permissionMode
	}
	switch mode {
	case "bypass":
		return " --dangerously-skip-permissions"
	case "plan", "auto-accept":
		return fmt.Sprintf(" --permission-mode %s", mode)
	default:
		// "default" or empty: no permission flags.
		return ""
	}
}

func (c *ClaudeBackend) BuildResumeCommand(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session id required for resume")
	}

	cmd := c.command
	cmd += c.buildPermissionFlags("")
	cmd += fmt.Sprintf(" --resume %q", sessionID)
	// Force in-process teammate mode (see BuildStartCommand for rationale).
	cmd += " --teammate-mode in-process"
	return cmd, nil
}

func (c *ClaudeBackend) SupportsResume() bool { return true }

func (c *ClaudeBackend) SupportsExplicitSessionID() bool { return true }

func (c *ClaudeBackend) Detector() detect.StateDetector {
	return detect.NewDetector()
}

func (c *ClaudeBackend) MetricsParser() *metrics.MetricsParser {
	return metrics.NewMetricsParser()
}

func (c *ClaudeBackend) EstimateCost(inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool) {
	return metrics.CalculateCost(inputTokens, outputTokens, cacheRead, cacheWrite), true
}

func (c *ClaudeBackend) LocalConfigFiles() []string {
	return []string{"CLAUDE.local.md"}
}

// CodexBackend implements Backend for Codex CLI.
type CodexBackend struct {
	command        string
	approvalMode   string
	detectorOnce   sync.Once
	cachedDetector detect.StateDetector
}

// NewCodexBackend creates a Codex backend from config.
func NewCodexBackend(cfg config.CodexBackendConfig) *CodexBackend {
	command := cfg.Command
	if command == "" {
		command = "codex"
	}
	mode := cfg.ApprovalMode
	if mode == "" {
		mode = "full-auto"
	}
	return &CodexBackend{
		command:      command,
		approvalMode: mode,
	}
}

func (c *CodexBackend) Name() BackendName { return BackendCodex }

func (c *CodexBackend) DisplayName() string { return "Codex" }

func (c *CodexBackend) PromptFileName() string { return ".codex-prompt" }

func (c *CodexBackend) BuildStartCommand(opts StartOptions) (string, error) {
	if opts.PromptFile == "" {
		return "", fmt.Errorf("prompt file required")
	}

	cmd := c.command
	if opts.Mode == StartModeOneShot {
		cmd += " exec"
	}
	cmd += c.approvalFlags()

	return fmt.Sprintf("%s \"$(cat %q)\" && rm %q", cmd, opts.PromptFile, opts.PromptFile), nil
}

func (c *CodexBackend) BuildResumeCommand(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session id required for resume")
	}
	cmd := c.command + " resume" + c.approvalFlags()
	cmd += fmt.Sprintf(" %q", sessionID)
	return cmd, nil
}

func (c *CodexBackend) SupportsResume() bool { return true }

func (c *CodexBackend) SupportsExplicitSessionID() bool { return false }

func (c *CodexBackend) Detector() detect.StateDetector {
	c.detectorOnce.Do(func() {
		patterns := detect.DefaultPatternSet()
		patterns.InputWaitingPatterns = append([]string{}, patterns.InputWaitingPatterns...)
		patterns.InputWaitingPatterns = append(patterns.InputWaitingPatterns,
			`(?m)^>\s*$`,
			`(?m)^›\s*$`,
		)
		patterns.ErrorPatterns = append([]string{}, patterns.ErrorPatterns...)
		patterns.ErrorPatterns = append(patterns.ErrorPatterns,
			`(?i)codex (?:exited|terminated|crashed|died)`,
		)
		c.cachedDetector = detect.NewDetectorWithPatterns(patterns)
	})
	return c.cachedDetector
}

func (c *CodexBackend) MetricsParser() *metrics.MetricsParser {
	return metrics.NewMetricsParser()
}

func (c *CodexBackend) EstimateCost(inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool) {
	return 0, false
}

func (c *CodexBackend) LocalConfigFiles() []string {
	return []string{"CODEX.local.md"}
}

func (c *CodexBackend) approvalFlags() string {
	switch strings.ToLower(c.approvalMode) {
	case "bypass":
		return " --dangerously-bypass-approvals-and-sandbox"
	case "full-auto":
		return " --full-auto"
	default:
		return ""
	}
}

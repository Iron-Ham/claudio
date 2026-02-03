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

// StartOptions configures backend start commands.
type StartOptions struct {
	PromptFile string
	SessionID  string
	Mode       StartMode
	// OutputOnly requests a non-interactive, print-only execution when supported.
	OutputOnly bool
}

// Backend provides backend-specific behavior for running AI sessions.
type Backend interface {
	Name() BackendName
	DisplayName() string
	PromptFileName() string
	BuildStartCommand(opts StartOptions) (string, error)
	BuildResumeCommand(sessionID string) (string, error)
	SupportsResume() bool
	SupportsExplicitSessionID() bool
	Detector() detect.StateDetector
	MetricsParser() *metrics.MetricsParser
	EstimateCost(inputTokens, outputTokens, cacheRead, cacheWrite int64) (float64, bool)
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

// ClaudeBackend implements Backend for Claude Code.
type ClaudeBackend struct {
	command         string
	skipPermissions bool
}

// NewClaudeBackend creates a Claude backend from config.
func NewClaudeBackend(cfg config.ClaudeBackendConfig) *ClaudeBackend {
	command := cfg.Command
	if command == "" {
		command = "claude"
	}
	return &ClaudeBackend{
		command:         command,
		skipPermissions: cfg.SkipPermissions,
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
	}
	if c.skipPermissions {
		cmd += " --dangerously-skip-permissions"
	}
	if opts.SessionID != "" {
		cmd += fmt.Sprintf(" --session-id %q", opts.SessionID)
	}

	return fmt.Sprintf("%s \"$(cat %q)\" && rm %q", cmd, opts.PromptFile, opts.PromptFile), nil
}

func (c *ClaudeBackend) BuildResumeCommand(sessionID string) (string, error) {
	if sessionID == "" {
		return "", fmt.Errorf("session id required for resume")
	}

	cmd := c.command
	if c.skipPermissions {
		cmd += " --dangerously-skip-permissions"
	}
	cmd += fmt.Sprintf(" --resume %q", sessionID)
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

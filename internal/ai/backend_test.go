package ai

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
)

func TestNewFromConfig(t *testing.T) {
	t.Run("default returns claude", func(t *testing.T) {
		cfg := config.Default()
		backend, err := NewFromConfig(cfg)
		if err != nil {
			t.Fatalf("NewFromConfig returned error: %v", err)
		}
		if backend.Name() != BackendClaude {
			t.Errorf("backend.Name() = %q, want %q", backend.Name(), BackendClaude)
		}
	})

	t.Run("nil config returns error", func(t *testing.T) {
		_, err := NewFromConfig(nil)
		if err == nil {
			t.Fatal("NewFromConfig(nil) should return error")
		}
		if !strings.Contains(err.Error(), "missing config") {
			t.Errorf("error should mention 'missing config', got: %v", err)
		}
	})

	t.Run("codex backend", func(t *testing.T) {
		cfg := config.Default()
		cfg.AI.Backend = "codex"
		backend, err := NewFromConfig(cfg)
		if err != nil {
			t.Fatalf("NewFromConfig with codex returned error: %v", err)
		}
		if backend.Name() != BackendCodex {
			t.Errorf("backend.Name() = %q, want %q", backend.Name(), BackendCodex)
		}
	})

	t.Run("unknown backend returns error", func(t *testing.T) {
		cfg := config.Default()
		cfg.AI.Backend = "unknown-backend"
		_, err := NewFromConfig(cfg)
		if err == nil {
			t.Fatal("NewFromConfig with unknown backend should return error")
		}
		if !errors.Is(err, ErrUnknownBackend) {
			t.Errorf("error should be ErrUnknownBackend, got: %v", err)
		}
	})

	t.Run("case insensitive backend name", func(t *testing.T) {
		cfg := config.Default()
		cfg.AI.Backend = "CODEX"
		backend, err := NewFromConfig(cfg)
		if err != nil {
			t.Fatalf("NewFromConfig with uppercase CODEX returned error: %v", err)
		}
		if backend.Name() != BackendCodex {
			t.Errorf("backend.Name() = %q, want %q", backend.Name(), BackendCodex)
		}
	})
}

func TestClaudeBackend_BuildCommands(t *testing.T) {
	backend := NewClaudeBackend(config.ClaudeBackendConfig{
		Command:         "claude",
		SkipPermissions: true,
	})

	t.Run("start command with session", func(t *testing.T) {
		startCmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			SessionID:  "session-123",
			Mode:       StartModeInteractive,
		})
		if err != nil {
			t.Fatalf("BuildStartCommand returned error: %v", err)
		}
		if !strings.Contains(startCmd, "claude --dangerously-skip-permissions") {
			t.Errorf("start command missing skip permissions: %q", startCmd)
		}
		if !strings.Contains(startCmd, "--session-id") {
			t.Errorf("start command missing session id: %q", startCmd)
		}
	})

	t.Run("one-shot with print", func(t *testing.T) {
		oneShotCmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			Mode:       StartModeOneShot,
			OutputOnly: true,
		})
		if err != nil {
			t.Fatalf("BuildStartCommand one-shot returned error: %v", err)
		}
		if !strings.Contains(oneShotCmd, "--print") {
			t.Errorf("one-shot command missing --print: %q", oneShotCmd)
		}
	})

	t.Run("resume command", func(t *testing.T) {
		resumeCmd, err := backend.BuildResumeCommand("session-123")
		if err != nil {
			t.Fatalf("BuildResumeCommand returned error: %v", err)
		}
		if !strings.Contains(resumeCmd, "--resume") {
			t.Errorf("resume command missing --resume: %q", resumeCmd)
		}
	})

	t.Run("start command requires prompt file", func(t *testing.T) {
		_, err := backend.BuildStartCommand(StartOptions{})
		if err == nil {
			t.Fatal("BuildStartCommand with empty prompt should return error")
		}
		if !strings.Contains(err.Error(), "prompt file required") {
			t.Errorf("error should mention 'prompt file required', got: %v", err)
		}
	})

	t.Run("resume command requires session ID", func(t *testing.T) {
		_, err := backend.BuildResumeCommand("")
		if err == nil {
			t.Fatal("BuildResumeCommand with empty session ID should return error")
		}
		if !strings.Contains(err.Error(), "session id required") {
			t.Errorf("error should mention 'session id required', got: %v", err)
		}
	})
}

func TestClaudeBackend_DefaultCommand(t *testing.T) {
	backend := NewClaudeBackend(config.ClaudeBackendConfig{})
	cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(cmd, "claude ") {
		t.Errorf("expected command to start with 'claude ', got: %s", cmd)
	}
}

func TestCodexBackend_BuildCommands(t *testing.T) {
	backend := NewCodexBackend(config.CodexBackendConfig{
		Command:      "codex",
		ApprovalMode: "bypass",
	})

	t.Run("interactive start command", func(t *testing.T) {
		startCmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			Mode:       StartModeInteractive,
		})
		if err != nil {
			t.Fatalf("BuildStartCommand returned error: %v", err)
		}
		if !strings.HasPrefix(startCmd, "codex --dangerously-bypass-approvals-and-sandbox") {
			t.Errorf("unexpected start command: %q", startCmd)
		}
	})

	t.Run("one-shot command", func(t *testing.T) {
		oneShotCmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			Mode:       StartModeOneShot,
		})
		if err != nil {
			t.Fatalf("BuildStartCommand one-shot returned error: %v", err)
		}
		if !strings.HasPrefix(oneShotCmd, "codex exec --dangerously-bypass-approvals-and-sandbox") {
			t.Errorf("unexpected one-shot command: %q", oneShotCmd)
		}
	})

	t.Run("resume command", func(t *testing.T) {
		resumeCmd, err := backend.BuildResumeCommand("session-123")
		if err != nil {
			t.Fatalf("BuildResumeCommand returned error: %v", err)
		}
		if !strings.HasPrefix(resumeCmd, "codex resume --dangerously-bypass-approvals-and-sandbox") {
			t.Errorf("unexpected resume command: %q", resumeCmd)
		}
	})

	t.Run("start command requires prompt file", func(t *testing.T) {
		_, err := backend.BuildStartCommand(StartOptions{})
		if err == nil {
			t.Fatal("BuildStartCommand with empty prompt should return error")
		}
		if !strings.Contains(err.Error(), "prompt file required") {
			t.Errorf("error should mention 'prompt file required', got: %v", err)
		}
	})

	t.Run("resume command requires session ID", func(t *testing.T) {
		_, err := backend.BuildResumeCommand("")
		if err == nil {
			t.Fatal("BuildResumeCommand with empty session ID should return error")
		}
		if !strings.Contains(err.Error(), "session id required") {
			t.Errorf("error should mention 'session id required', got: %v", err)
		}
	})
}

func TestCodexBackend_DefaultCommandAndMode(t *testing.T) {
	backend := NewCodexBackend(config.CodexBackendConfig{})
	cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
	if err != nil {
		t.Fatal(err)
	}
	// Default mode is "full-auto"
	if !strings.HasPrefix(cmd, "codex --full-auto") {
		t.Errorf("expected command to start with 'codex --full-auto', got: %s", cmd)
	}
}

func TestCodexBackend_ApprovalModes(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"bypass", "--dangerously-bypass-approvals-and-sandbox"},
		{"full-auto", "--full-auto"},
		{"default", ""},     // No flag for default mode
		{"", "--full-auto"}, // Empty defaults to full-auto
	}

	for _, tc := range tests {
		t.Run("mode_"+tc.mode, func(t *testing.T) {
			backend := NewCodexBackend(config.CodexBackendConfig{
				Command:      "codex",
				ApprovalMode: tc.mode,
			})
			cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
			if err != nil {
				t.Fatal(err)
			}
			if tc.expected == "" {
				// For default mode, ensure no approval flags are present
				if strings.Contains(cmd, "--dangerously") || strings.Contains(cmd, "--full-auto") {
					t.Errorf("default mode should have no approval flags, got: %s", cmd)
				}
			} else if !strings.Contains(cmd, tc.expected) {
				t.Errorf("expected command to contain %q, got: %s", tc.expected, cmd)
			}
		})
	}
}

func TestCodexBackend_DetectorInputPrompt(t *testing.T) {
	backend := NewCodexBackend(config.CodexBackendConfig{
		Command:      "codex",
		ApprovalMode: "default",
	})
	state := backend.Detector().Detect([]byte(">"))
	if state != detect.StateWaitingInput {
		t.Errorf("Detect(\">\") = %v, want %v", state, detect.StateWaitingInput)
	}
}

func TestBackendCapabilities(t *testing.T) {
	t.Run("Claude", func(t *testing.T) {
		claude := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:         "claude",
			SkipPermissions: true,
		})
		if !claude.SupportsResume() {
			t.Error("Claude backend should support resume")
		}
		if !claude.SupportsExplicitSessionID() {
			t.Error("Claude backend should support explicit session IDs")
		}
	})

	t.Run("Codex", func(t *testing.T) {
		codex := NewCodexBackend(config.CodexBackendConfig{
			Command:      "codex",
			ApprovalMode: "default",
		})
		if !codex.SupportsResume() {
			t.Error("Codex backend should support resume")
		}
		if codex.SupportsExplicitSessionID() {
			t.Error("Codex backend should not support explicit session IDs")
		}
	})

	t.Run("DefaultBackend", func(t *testing.T) {
		backend := DefaultBackend()
		if backend.Name() != BackendClaude {
			t.Errorf("DefaultBackend() = %q, want %q", backend.Name(), BackendClaude)
		}
	})
}

// TestCodexBackend_ConcurrentDetectorAccess verifies that the Detector() method
// is thread-safe when called concurrently. This test would fail with -race if
// sync.Once were not used for detector initialization.
func TestCodexBackend_ConcurrentDetectorAccess(t *testing.T) {
	backend := NewCodexBackend(config.CodexBackendConfig{
		Command:      "codex",
		ApprovalMode: "default",
	})

	const goroutines = 100
	var wg sync.WaitGroup
	detectors := make([]detect.StateDetector, goroutines)

	// Concurrently access Detector() from multiple goroutines
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			detectors[idx] = backend.Detector()
		}(i)
	}
	wg.Wait()

	// Verify all goroutines got the same detector instance
	first := detectors[0]
	for i := 1; i < goroutines; i++ {
		if detectors[i] != first {
			t.Errorf("Detector() returned different instances: got %p and %p", detectors[i], first)
		}
	}
}

package ai

import (
	"strings"
	"sync"
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
)

func TestNewFromConfig_DefaultClaude(t *testing.T) {
	cfg := config.Default()
	backend, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewFromConfig returned error: %v", err)
	}
	if backend.Name() != BackendClaude {
		t.Errorf("backend.Name() = %q, want %q", backend.Name(), BackendClaude)
	}
}

func TestClaudeBackend_BuildCommands(t *testing.T) {
	backend := NewClaudeBackend(config.ClaudeBackendConfig{
		Command:         "claude",
		SkipPermissions: true,
	})

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

	resumeCmd, err := backend.BuildResumeCommand("session-123")
	if err != nil {
		t.Fatalf("BuildResumeCommand returned error: %v", err)
	}
	if !strings.Contains(resumeCmd, "--resume") {
		t.Errorf("resume command missing --resume: %q", resumeCmd)
	}
}

func TestCodexBackend_BuildCommands(t *testing.T) {
	backend := NewCodexBackend(config.CodexBackendConfig{
		Command:      "codex",
		ApprovalMode: "bypass",
	})

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

	resumeCmd, err := backend.BuildResumeCommand("session-123")
	if err != nil {
		t.Fatalf("BuildResumeCommand returned error: %v", err)
	}
	if !strings.HasPrefix(resumeCmd, "codex resume --dangerously-bypass-approvals-and-sandbox") {
		t.Errorf("unexpected resume command: %q", resumeCmd)
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

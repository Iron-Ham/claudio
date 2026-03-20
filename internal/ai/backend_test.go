package ai

import (
	"errors"
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
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

	t.Run("codex backend returns migration error", func(t *testing.T) {
		cfg := config.Default()
		cfg.AI.Backend = "codex"
		_, err := NewFromConfig(cfg)
		if err == nil {
			t.Fatal("NewFromConfig with codex should return error")
		}
		if !strings.Contains(err.Error(), "removed") {
			t.Errorf("error should mention removal, got: %v", err)
		}
	})

	t.Run("case insensitive backend name", func(t *testing.T) {
		cfg := config.Default()
		cfg.AI.Backend = "CLAUDE"
		backend, err := NewFromConfig(cfg)
		if err != nil {
			t.Fatalf("NewFromConfig with uppercase CLAUDE returned error: %v", err)
		}
		if backend.Name() != BackendClaude {
			t.Errorf("backend.Name() = %q, want %q", backend.Name(), BackendClaude)
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
		if !strings.Contains(startCmd, "--teammate-mode in-process") {
			t.Errorf("start command missing teammate mode: %q", startCmd)
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
		if !strings.Contains(resumeCmd, "--teammate-mode in-process") {
			t.Errorf("resume command missing teammate mode: %q", resumeCmd)
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

	t.Run("DefaultBackend", func(t *testing.T) {
		backend := DefaultBackend()
		if backend.Name() != BackendClaude {
			t.Errorf("DefaultBackend() = %q, want %q", backend.Name(), BackendClaude)
		}
	})
}

func TestClaudeBackend_PermissionModes(t *testing.T) {
	tests := []struct {
		name           string
		cfg            config.ClaudeBackendConfig
		wantContains   string
		wantNotContain string
	}{
		{
			name:         "legacy skip_permissions=true resolves to bypass",
			cfg:          config.ClaudeBackendConfig{Command: "claude", SkipPermissions: true},
			wantContains: "--dangerously-skip-permissions",
		},
		{
			name:           "legacy skip_permissions=false resolves to default (no flag)",
			cfg:            config.ClaudeBackendConfig{Command: "claude", SkipPermissions: false},
			wantNotContain: "--dangerously-skip-permissions",
		},
		{
			name:         "permission_mode=bypass uses --dangerously-skip-permissions",
			cfg:          config.ClaudeBackendConfig{Command: "claude", PermissionMode: "bypass"},
			wantContains: "--dangerously-skip-permissions",
		},
		{
			name:         "permission_mode=plan uses --permission-mode plan",
			cfg:          config.ClaudeBackendConfig{Command: "claude", PermissionMode: "plan"},
			wantContains: "--permission-mode plan",
		},
		{
			name:         "permission_mode=auto-accept uses --permission-mode auto-accept",
			cfg:          config.ClaudeBackendConfig{Command: "claude", PermissionMode: "auto-accept"},
			wantContains: "--permission-mode auto-accept",
		},
		{
			name:           "permission_mode=default has no permission flag",
			cfg:            config.ClaudeBackendConfig{Command: "claude", PermissionMode: "default"},
			wantNotContain: "--permission-mode",
		},
		{
			name:         "permission_mode overrides skip_permissions",
			cfg:          config.ClaudeBackendConfig{Command: "claude", SkipPermissions: true, PermissionMode: "plan"},
			wantContains: "--permission-mode plan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewClaudeBackend(tt.cfg)
			cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
			if err != nil {
				t.Fatalf("BuildStartCommand returned error: %v", err)
			}
			if tt.wantContains != "" && !strings.Contains(cmd, tt.wantContains) {
				t.Errorf("command missing %q: %s", tt.wantContains, cmd)
			}
			if tt.wantNotContain != "" && strings.Contains(cmd, tt.wantNotContain) {
				t.Errorf("command should not contain %q: %s", tt.wantNotContain, cmd)
			}
		})
	}
}

func TestClaudeBackend_PermissionModePerInvocation(t *testing.T) {
	// Backend configured with bypass, but per-invocation overrides to plan.
	backend := NewClaudeBackend(config.ClaudeBackendConfig{
		Command:        "claude",
		PermissionMode: "bypass",
	})
	cmd, err := backend.BuildStartCommand(StartOptions{
		PromptFile:     "/tmp/prompt",
		PermissionMode: "plan",
	})
	if err != nil {
		t.Fatalf("BuildStartCommand returned error: %v", err)
	}
	if !strings.Contains(cmd, "--permission-mode plan") {
		t.Errorf("per-invocation plan should override backend bypass: %s", cmd)
	}
	if strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("bypass should not appear when overridden: %s", cmd)
	}
}

func TestClaudeBackend_ResumePermissionMode(t *testing.T) {
	backend := NewClaudeBackend(config.ClaudeBackendConfig{
		Command:        "claude",
		PermissionMode: "plan",
	})
	cmd, err := backend.BuildResumeCommand("session-123")
	if err != nil {
		t.Fatalf("BuildResumeCommand returned error: %v", err)
	}
	if !strings.Contains(cmd, "--permission-mode plan") {
		t.Errorf("resume command should use permission mode: %s", cmd)
	}
}

func TestClaudeBackend_MaxTurns(t *testing.T) {
	t.Run("from config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:  "claude",
			MaxTurns: 10,
		})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--max-turns 10") {
			t.Errorf("command missing --max-turns 10: %s", cmd)
		}
	})

	t.Run("per-invocation overrides config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:  "claude",
			MaxTurns: 10,
		})
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			MaxTurns:   5,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--max-turns 5") {
			t.Errorf("per-invocation max-turns should be 5: %s", cmd)
		}
	})

	t.Run("zero means unlimited (no flag)", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(cmd, "--max-turns") {
			t.Errorf("zero max-turns should produce no flag: %s", cmd)
		}
	})
}

func TestClaudeBackend_Model(t *testing.T) {
	t.Run("from config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command: "claude",
			Model:   "opus",
		})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, `--model "opus"`) {
			t.Errorf("command missing --model opus: %s", cmd)
		}
	})

	t.Run("per-invocation overrides config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command: "claude",
			Model:   "opus",
		})
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			Model:      "sonnet",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, `--model "sonnet"`) {
			t.Errorf("per-invocation model should be sonnet: %s", cmd)
		}
	})

	t.Run("empty means no flag", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(cmd, "--model") {
			t.Errorf("empty model should produce no flag: %s", cmd)
		}
	})
}

func TestClaudeBackend_AllowedDisallowedTools(t *testing.T) {
	t.Run("from config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:      "claude",
			AllowedTools: []string{"Read", "Write"},
		})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, `--allowedTools "Read"`) {
			t.Errorf("missing --allowedTools Read: %s", cmd)
		}
		if !strings.Contains(cmd, `--allowedTools "Write"`) {
			t.Errorf("missing --allowedTools Write: %s", cmd)
		}
	})

	t.Run("per-invocation merged with config (deduped)", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:      "claude",
			AllowedTools: []string{"Read", "Write"},
		})
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:   "/tmp/prompt",
			AllowedTools: []string{"Bash", "Read"}, // Read is a duplicate
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, `--allowedTools "Bash"`) {
			t.Errorf("missing per-invocation Bash: %s", cmd)
		}
		// Count occurrences of --allowedTools "Read" — should be exactly 1
		count := strings.Count(cmd, `--allowedTools "Read"`)
		if count != 1 {
			t.Errorf("Read should appear exactly once, got %d in: %s", count, cmd)
		}
	})

	t.Run("disallowed tools", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:         "claude",
			DisallowedTools: []string{"Bash"},
		})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, `--disallowedTools "Bash"`) {
			t.Errorf("missing --disallowedTools Bash: %s", cmd)
		}
	})

	t.Run("empty tools produce no flags", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(cmd, "--allowedTools") || strings.Contains(cmd, "--disallowedTools") {
			t.Errorf("empty tools should produce no flags: %s", cmd)
		}
	})
}

func TestClaudeBackend_SystemPrompt(t *testing.T) {
	t.Run("append from config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:            "claude",
			AppendSystemPrompt: "Always write tests.",
		})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--append-system-prompt") {
			t.Errorf("missing --append-system-prompt: %s", cmd)
		}
		if !strings.Contains(cmd, "Always write tests.") {
			t.Errorf("missing system prompt content: %s", cmd)
		}
	})

	t.Run("append from file (per-invocation)", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:             "/tmp/prompt",
			AppendSystemPromptFile: "/tmp/system-prompt.md",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, `--append-system-prompt-file "/tmp/system-prompt.md"`) {
			t.Errorf("missing --append-system-prompt-file: %s", cmd)
		}
	})

	t.Run("both file and text (file first, text second)", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:            "claude",
			AppendSystemPrompt: "Be concise.",
		})
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:             "/tmp/prompt",
			AppendSystemPromptFile: "/tmp/extra.md",
		})
		if err != nil {
			t.Fatal(err)
		}
		fileIdx := strings.Index(cmd, "--append-system-prompt-file")
		textIdx := strings.Index(cmd, "--append-system-prompt \"Be concise.\"")
		if fileIdx == -1 || textIdx == -1 {
			t.Fatalf("both system prompt flags should be present: %s", cmd)
		}
		if fileIdx > textIdx {
			t.Errorf("file flag should appear before text flag: %s", cmd)
		}
	})
}

func TestClaudeBackend_NoUserPrompt(t *testing.T) {
	backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
	cmd, err := backend.BuildStartCommand(StartOptions{
		PromptFile:   "/tmp/prompt",
		NoUserPrompt: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cmd, "--no-user-prompt") {
		t.Errorf("missing --no-user-prompt: %s", cmd)
	}
}

func TestClaudeBackend_Worktree(t *testing.T) {
	t.Run("from config", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{
			Command:        "claude",
			NativeWorktree: true,
		})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--worktree") {
			t.Errorf("missing --worktree: %s", cmd)
		}
	})

	t.Run("per-invocation override", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile: "/tmp/prompt",
			Worktree:   true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--worktree") {
			t.Errorf("missing --worktree: %s", cmd)
		}
	})

	t.Run("disabled by default", func(t *testing.T) {
		backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})
		cmd, err := backend.BuildStartCommand(StartOptions{PromptFile: "/tmp/prompt"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(cmd, "--worktree") {
			t.Errorf("--worktree should not appear when disabled: %s", cmd)
		}
	})
}

func TestClaudeBackend_OutputFormat(t *testing.T) {
	backend := NewClaudeBackend(config.ClaudeBackendConfig{Command: "claude"})

	t.Run("stream-json with print", func(t *testing.T) {
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:   "/tmp/prompt",
			OutputOnly:   true,
			OutputFormat: OutputFormatStreamJSON,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--print") {
			t.Errorf("missing --print: %s", cmd)
		}
		if !strings.Contains(cmd, "--output-format stream-json") {
			t.Errorf("missing --output-format stream-json: %s", cmd)
		}
	})

	t.Run("json with print", func(t *testing.T) {
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:   "/tmp/prompt",
			OutputOnly:   true,
			OutputFormat: OutputFormatJSON,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(cmd, "--output-format json") {
			t.Errorf("missing --output-format json: %s", cmd)
		}
	})

	t.Run("text format produces no output-format flag", func(t *testing.T) {
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:   "/tmp/prompt",
			OutputOnly:   true,
			OutputFormat: OutputFormatText,
		})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(cmd, "--output-format") {
			t.Errorf("text format should produce no output-format flag: %s", cmd)
		}
	})

	t.Run("output format ignored without print", func(t *testing.T) {
		cmd, err := backend.BuildStartCommand(StartOptions{
			PromptFile:   "/tmp/prompt",
			OutputOnly:   false,
			OutputFormat: OutputFormatStreamJSON,
		})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(cmd, "--output-format") {
			t.Errorf("output format should be ignored without --print: %s", cmd)
		}
	})
}

func TestClaudeBackend_CombinedFlags(t *testing.T) {
	// Test that all flags work together in a realistic configuration.
	backend := NewClaudeBackend(config.ClaudeBackendConfig{
		Command:            "claude",
		PermissionMode:     "bypass",
		AllowedTools:       []string{"Read", "Write"},
		MaxTurns:           20,
		Model:              "sonnet",
		AppendSystemPrompt: "Be thorough.",
	})
	cmd, err := backend.BuildStartCommand(StartOptions{
		PromptFile:             "/tmp/prompt",
		SessionID:              "sess-001",
		OutputOnly:             true,
		OutputFormat:           OutputFormatStreamJSON,
		NoUserPrompt:           true,
		Worktree:               true,
		AppendSystemPromptFile: "/tmp/orchestration.md",
		DisallowedTools:        []string{"Bash"},
	})
	if err != nil {
		t.Fatalf("BuildStartCommand returned error: %v", err)
	}

	expected := []string{
		"--print",
		"--output-format stream-json",
		"--dangerously-skip-permissions",
		"--session-id",
		`--model "sonnet"`,
		"--max-turns 20",
		`--allowedTools "Read"`,
		`--allowedTools "Write"`,
		`--disallowedTools "Bash"`,
		"--append-system-prompt-file",
		"--append-system-prompt",
		"--no-user-prompt",
		"--worktree",
		"--teammate-mode in-process",
	}
	for _, flag := range expected {
		if !strings.Contains(cmd, flag) {
			t.Errorf("combined command missing %q:\n%s", flag, cmd)
		}
	}
}

func TestHelpers_firstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "", "c"); got != "c" {
		t.Errorf("firstNonEmpty(\"\", \"\", \"c\") = %q, want \"c\"", got)
	}
	if got := firstNonEmpty("a", "b"); got != "a" {
		t.Errorf("firstNonEmpty(\"a\", \"b\") = %q, want \"a\"", got)
	}
	if got := firstNonEmpty("", ""); got != "" {
		t.Errorf("firstNonEmpty(\"\", \"\") = %q, want \"\"", got)
	}
}

func TestHelpers_firstPositive(t *testing.T) {
	if got := firstPositive(0, 0, 5); got != 5 {
		t.Errorf("firstPositive(0, 0, 5) = %d, want 5", got)
	}
	if got := firstPositive(3, 7); got != 3 {
		t.Errorf("firstPositive(3, 7) = %d, want 3", got)
	}
	if got := firstPositive(0, 0); got != 0 {
		t.Errorf("firstPositive(0, 0) = %d, want 0", got)
	}
}

func TestHelpers_mergeUnique(t *testing.T) {
	t.Run("merge with dedup", func(t *testing.T) {
		result := mergeUnique([]string{"a", "b"}, []string{"b", "c"})
		expected := []string{"a", "b", "c"}
		if len(result) != len(expected) {
			t.Fatalf("mergeUnique length = %d, want %d", len(result), len(expected))
		}
		for i, v := range expected {
			if result[i] != v {
				t.Errorf("mergeUnique[%d] = %q, want %q", i, result[i], v)
			}
		}
	})

	t.Run("both empty", func(t *testing.T) {
		result := mergeUnique(nil, nil)
		if result != nil {
			t.Errorf("mergeUnique(nil, nil) = %v, want nil", result)
		}
	})

	t.Run("one empty", func(t *testing.T) {
		result := mergeUnique([]string{"a"}, nil)
		if len(result) != 1 || result[0] != "a" {
			t.Errorf("mergeUnique([a], nil) = %v, want [a]", result)
		}
	})
}

func TestClaudeBackendConfig_ResolvedPermissionMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ClaudeBackendConfig
		want string
	}{
		{"permission_mode set", config.ClaudeBackendConfig{PermissionMode: "plan"}, "plan"},
		{"permission_mode bypass", config.ClaudeBackendConfig{PermissionMode: "bypass"}, "bypass"},
		{"skip_permissions true, no permission_mode", config.ClaudeBackendConfig{SkipPermissions: true}, "bypass"},
		{"skip_permissions false, no permission_mode", config.ClaudeBackendConfig{SkipPermissions: false}, "default"},
		{"permission_mode overrides skip_permissions", config.ClaudeBackendConfig{SkipPermissions: true, PermissionMode: "plan"}, "plan"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.ResolvedPermissionMode()
			if got != tt.want {
				t.Errorf("ResolvedPermissionMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

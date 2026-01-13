package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg == nil {
		t.Fatal("Default() returned nil")
	}

	// Verify default completion config
	if cfg.Completion.DefaultAction != "prompt" {
		t.Errorf("Completion.DefaultAction = %q, want %q", cfg.Completion.DefaultAction, "prompt")
	}

	// Verify default TUI config
	if !cfg.TUI.AutoFocusOnInput {
		t.Error("TUI.AutoFocusOnInput should be true by default")
	}
	if cfg.TUI.MaxOutputLines != 1000 {
		t.Errorf("TUI.MaxOutputLines = %d, want 1000", cfg.TUI.MaxOutputLines)
	}
	if !cfg.TUI.VerboseCommandHelp {
		t.Error("TUI.VerboseCommandHelp should be true by default")
	}

	// Verify default instance config
	if cfg.Instance.OutputBufferSize != 100000 {
		t.Errorf("Instance.OutputBufferSize = %d, want 100000", cfg.Instance.OutputBufferSize)
	}
	if cfg.Instance.CaptureIntervalMs != 100 {
		t.Errorf("Instance.CaptureIntervalMs = %d, want 100", cfg.Instance.CaptureIntervalMs)
	}
	if cfg.Instance.TmuxWidth != 200 {
		t.Errorf("Instance.TmuxWidth = %d, want 200", cfg.Instance.TmuxWidth)
	}
	if cfg.Instance.TmuxHeight != 50 {
		t.Errorf("Instance.TmuxHeight = %d, want 50", cfg.Instance.TmuxHeight)
	}
	if cfg.Instance.TmuxHistoryLimit != 50000 {
		t.Errorf("Instance.TmuxHistoryLimit = %d, want 50000", cfg.Instance.TmuxHistoryLimit)
	}

	// Verify default PR config
	if cfg.PR.Draft {
		t.Error("PR.Draft should be false by default")
	}
	if !cfg.PR.AutoRebase {
		t.Error("PR.AutoRebase should be true by default")
	}
	if !cfg.PR.UseAI {
		t.Error("PR.UseAI should be true by default")
	}
	if cfg.PR.AutoPROnStop {
		t.Error("PR.AutoPROnStop should be false by default")
	}

	// Verify default cleanup config
	if !cfg.Cleanup.WarnOnStale {
		t.Error("Cleanup.WarnOnStale should be true by default")
	}
	if !cfg.Cleanup.KeepRemoteBranches {
		t.Error("Cleanup.KeepRemoteBranches should be true by default")
	}

	// Verify default resource config
	if cfg.Resources.CostWarningThreshold != 5.00 {
		t.Errorf("Resources.CostWarningThreshold = %f, want 5.00", cfg.Resources.CostWarningThreshold)
	}
	if cfg.Resources.CostLimit != 0 {
		t.Errorf("Resources.CostLimit = %f, want 0", cfg.Resources.CostLimit)
	}
	if cfg.Resources.TokenLimitPerInstance != 0 {
		t.Errorf("Resources.TokenLimitPerInstance = %d, want 0", cfg.Resources.TokenLimitPerInstance)
	}
	if !cfg.Resources.ShowMetricsInSidebar {
		t.Error("Resources.ShowMetricsInSidebar should be true by default")
	}
}

func TestInstanceConfig_CaptureInterval(t *testing.T) {
	tests := []struct {
		ms       int
		expected time.Duration
	}{
		{100, 100 * time.Millisecond},
		{500, 500 * time.Millisecond},
		{1000, 1 * time.Second},
		{0, 0},
	}

	for _, tt := range tests {
		cfg := InstanceConfig{CaptureIntervalMs: tt.ms}
		result := cfg.CaptureInterval()
		if result != tt.expected {
			t.Errorf("CaptureInterval() with %dms = %v, want %v", tt.ms, result, tt.expected)
		}
	}
}

func TestValidCompletionActions(t *testing.T) {
	actions := ValidCompletionActions()

	expected := []string{"prompt", "keep_branch", "merge_staging", "merge_main", "auto_pr"}
	if len(actions) != len(expected) {
		t.Errorf("ValidCompletionActions() length = %d, want %d", len(actions), len(expected))
	}

	for i, action := range expected {
		if actions[i] != action {
			t.Errorf("ValidCompletionActions()[%d] = %q, want %q", i, actions[i], action)
		}
	}
}

func TestIsValidCompletionAction(t *testing.T) {
	tests := []struct {
		action string
		valid  bool
	}{
		{"prompt", true},
		{"keep_branch", true},
		{"merge_staging", true},
		{"merge_main", true},
		{"auto_pr", true},
		{"invalid", false},
		{"", false},
		{"PROMPT", false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			result := IsValidCompletionAction(tt.action)
			if result != tt.valid {
				t.Errorf("IsValidCompletionAction(%q) = %v, want %v", tt.action, result, tt.valid)
			}
		})
	}
}

func TestConfigDir(t *testing.T) {
	// Test with XDG_CONFIG_HOME set
	t.Run("with XDG_CONFIG_HOME", func(t *testing.T) {
		original := os.Getenv("XDG_CONFIG_HOME")
		defer func() { _ = os.Setenv("XDG_CONFIG_HOME", original) }()

		_ = os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		result := ConfigDir()
		expected := "/custom/config/claudio"
		if result != expected {
			t.Errorf("ConfigDir() = %q, want %q", result, expected)
		}
	})

	// Test without XDG_CONFIG_HOME
	t.Run("without XDG_CONFIG_HOME", func(t *testing.T) {
		original := os.Getenv("XDG_CONFIG_HOME")
		defer func() { _ = os.Setenv("XDG_CONFIG_HOME", original) }()

		_ = os.Setenv("XDG_CONFIG_HOME", "")
		result := ConfigDir()

		// Should be based on home directory
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".config", "claudio")
		if result != expected {
			t.Errorf("ConfigDir() = %q, want %q", result, expected)
		}
	})
}

func TestConfigFile(t *testing.T) {
	original := os.Getenv("XDG_CONFIG_HOME")
	defer func() { _ = os.Setenv("XDG_CONFIG_HOME", original) }()

	_ = os.Setenv("XDG_CONFIG_HOME", "/custom/config")
	result := ConfigFile()
	expected := "/custom/config/claudio/config.yaml"
	if result != expected {
		t.Errorf("ConfigFile() = %q, want %q", result, expected)
	}
}

func TestGet(t *testing.T) {
	// Set defaults in viper first (normally done by cmd init)
	SetDefaults()

	// Get() should return defaults when no config file exists
	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}

	// Should have default values
	if cfg.Completion.DefaultAction != "prompt" {
		t.Errorf("Get().Completion.DefaultAction = %q, want %q", cfg.Completion.DefaultAction, "prompt")
	}
}

func TestConfig_PRConfig_Reviewers(t *testing.T) {
	cfg := Default()

	// Default reviewers should be empty
	if len(cfg.PR.Reviewers.Default) != 0 {
		t.Errorf("PR.Reviewers.Default should be empty, got %v", cfg.PR.Reviewers.Default)
	}

	// ByPath should be empty
	if len(cfg.PR.Reviewers.ByPath) != 0 {
		t.Errorf("PR.Reviewers.ByPath should be empty, got %v", cfg.PR.Reviewers.ByPath)
	}

	// Labels should be empty
	if len(cfg.PR.Labels) != 0 {
		t.Errorf("PR.Labels should be empty, got %v", cfg.PR.Labels)
	}
}

func TestConfig_InstanceConfig_Values(t *testing.T) {
	cfg := Default()

	// Test that instance config values are reasonable
	if cfg.Instance.OutputBufferSize < 1000 {
		t.Errorf("OutputBufferSize should be at least 1000 bytes, got %d", cfg.Instance.OutputBufferSize)
	}

	if cfg.Instance.CaptureIntervalMs < 10 {
		t.Errorf("CaptureIntervalMs should be at least 10ms, got %d", cfg.Instance.CaptureIntervalMs)
	}

	if cfg.Instance.TmuxWidth < 80 {
		t.Errorf("TmuxWidth should be at least 80, got %d", cfg.Instance.TmuxWidth)
	}

	if cfg.Instance.TmuxHeight < 24 {
		t.Errorf("TmuxHeight should be at least 24, got %d", cfg.Instance.TmuxHeight)
	}
}

func TestConfig_ResourceConfig_Values(t *testing.T) {
	cfg := Default()

	// Cost warning threshold should be positive
	if cfg.Resources.CostWarningThreshold <= 0 {
		t.Errorf("CostWarningThreshold should be positive, got %f", cfg.Resources.CostWarningThreshold)
	}

	// Cost limit of 0 means no limit (valid default)
	if cfg.Resources.CostLimit < 0 {
		t.Errorf("CostLimit should not be negative, got %f", cfg.Resources.CostLimit)
	}

	// Token limit of 0 means no limit (valid default)
	if cfg.Resources.TokenLimitPerInstance < 0 {
		t.Errorf("TokenLimitPerInstance should not be negative, got %d", cfg.Resources.TokenLimitPerInstance)
	}
}

func TestConfig_UltraplanConfig_Values(t *testing.T) {
	cfg := Default()

	// MaxParallel should default to 3
	if cfg.Ultraplan.MaxParallel != 3 {
		t.Errorf("Ultraplan.MaxParallel = %d, want 3", cfg.Ultraplan.MaxParallel)
	}

	// MultiPass should default to false
	if cfg.Ultraplan.MultiPass {
		t.Error("Ultraplan.MultiPass should be false by default")
	}

	// Notifications should be enabled by default
	if !cfg.Ultraplan.Notifications.Enabled {
		t.Error("Ultraplan.Notifications.Enabled should be true by default")
	}

	// UseSound should be disabled by default
	if cfg.Ultraplan.Notifications.UseSound {
		t.Error("Ultraplan.Notifications.UseSound should be false by default")
	}

	// SoundPath should be empty by default
	if cfg.Ultraplan.Notifications.SoundPath != "" {
		t.Errorf("Ultraplan.Notifications.SoundPath should be empty, got %q", cfg.Ultraplan.Notifications.SoundPath)
	}
}

func TestConfig_UltraplanMultiPass_ViperLoading(t *testing.T) {
	// Reset viper to clean state for this test
	viper.Reset()
	SetDefaults()

	// After SetDefaults, Get() should return default MultiPass=false
	cfg := Get()
	if cfg.Ultraplan.MultiPass {
		t.Error("Ultraplan.MultiPass should be false after SetDefaults()")
	}

	// Verify viper has the default value set
	if viper.GetBool("ultraplan.multi_pass") {
		t.Error("viper.GetBool('ultraplan.multi_pass') should be false by default")
	}
}

func TestConfig_UltraplanMultiPass_ConfigCascade(t *testing.T) {
	// Test the config cascade: default -> config file -> CLI flag (viper.Set)
	// This simulates how the --multi-pass CLI flag would override config

	t.Run("default value", func(t *testing.T) {
		viper.Reset()
		SetDefaults()

		cfg := Get()
		if cfg.Ultraplan.MultiPass {
			t.Error("default: Ultraplan.MultiPass should be false")
		}
	})

	t.Run("viper.Set overrides default (simulates CLI flag)", func(t *testing.T) {
		viper.Reset()
		SetDefaults()

		// Simulate CLI flag setting (--multi-pass)
		viper.Set("ultraplan.multi_pass", true)

		cfg := Get()
		if !cfg.Ultraplan.MultiPass {
			t.Error("after viper.Set: Ultraplan.MultiPass should be true")
		}
	})

	t.Run("explicit false overrides default false", func(t *testing.T) {
		viper.Reset()
		SetDefaults()

		// Explicitly set to false (should still work)
		viper.Set("ultraplan.multi_pass", false)

		cfg := Get()
		if cfg.Ultraplan.MultiPass {
			t.Error("after viper.Set(false): Ultraplan.MultiPass should be false")
		}
	})

	t.Run("viper.Set true then false", func(t *testing.T) {
		viper.Reset()
		SetDefaults()

		// Set to true first
		viper.Set("ultraplan.multi_pass", true)
		cfg1 := Get()
		if !cfg1.Ultraplan.MultiPass {
			t.Error("after first viper.Set(true): Ultraplan.MultiPass should be true")
		}

		// Override with false
		viper.Set("ultraplan.multi_pass", false)
		cfg2 := Get()
		if cfg2.Ultraplan.MultiPass {
			t.Error("after second viper.Set(false): Ultraplan.MultiPass should be false")
		}
	})
}

func TestConfig_PathsConfig_Defaults(t *testing.T) {
	cfg := Default()

	// WorktreeDir should be empty by default (meaning use default location)
	if cfg.Paths.WorktreeDir != "" {
		t.Errorf("Paths.WorktreeDir should be empty by default, got %q", cfg.Paths.WorktreeDir)
	}
}

func TestPathsConfig_ResolveWorktreeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("could not get home directory: %v", err)
	}

	tests := []struct {
		name        string
		worktreeDir string
		baseDir     string
		expected    string
	}{
		{
			name:        "empty uses default",
			worktreeDir: "",
			baseDir:     "/repo",
			expected:    "/repo/.claudio/worktrees",
		},
		{
			name:        "absolute path used as-is",
			worktreeDir: "/custom/worktrees",
			baseDir:     "/repo",
			expected:    "/custom/worktrees",
		},
		{
			name:        "tilde expands to home",
			worktreeDir: "~/claudio-worktrees",
			baseDir:     "/repo",
			expected:    filepath.Join(home, "claudio-worktrees"),
		},
		{
			name:        "tilde alone expands to home",
			worktreeDir: "~",
			baseDir:     "/repo",
			expected:    home,
		},
		{
			name:        "relative path resolved against baseDir",
			worktreeDir: "custom-worktrees",
			baseDir:     "/repo",
			expected:    "/repo/custom-worktrees",
		},
		{
			name:        "relative path with subdirs",
			worktreeDir: "../shared/worktrees",
			baseDir:     "/repo/project",
			expected:    "/repo/shared/worktrees", // filepath.Join normalizes the path
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PathsConfig{WorktreeDir: tt.worktreeDir}
			result := p.ResolveWorktreeDir(tt.baseDir)
			if result != tt.expected {
				t.Errorf("ResolveWorktreeDir(%q) = %q, want %q", tt.baseDir, result, tt.expected)
			}
		})
	}
}

func TestTUIConfig_VerboseCommandHelp_ViperLoading(t *testing.T) {
	// Test that verbose_command_help setting is properly loaded via viper
	viper.Reset()
	SetDefaults()

	// After SetDefaults, tui.verbose_command_help should be true
	verboseHelp := viper.GetBool("tui.verbose_command_help")
	if !verboseHelp {
		t.Error("viper.GetBool('tui.verbose_command_help') should be true by default")
	}

	// Test disabling via viper (simulates user preference for compact mode)
	viper.Set("tui.verbose_command_help", false)
	cfg := Get()
	if cfg.TUI.VerboseCommandHelp {
		t.Error("TUI.VerboseCommandHelp should be false after viper.Set(false)")
	}

	// Test re-enabling
	viper.Set("tui.verbose_command_help", true)
	cfg = Get()
	if !cfg.TUI.VerboseCommandHelp {
		t.Error("TUI.VerboseCommandHelp should be true after viper.Set(true)")
	}
}

func TestPathsConfig_ResolveWorktreeDir_ViperLoading(t *testing.T) {
	// Test that the worktree_dir setting is properly loaded via viper
	viper.Reset()
	SetDefaults()

	// After SetDefaults, paths.worktree_dir should be empty
	worktreeDir := viper.GetString("paths.worktree_dir")
	if worktreeDir != "" {
		t.Errorf("viper.GetString('paths.worktree_dir') should be empty by default, got %q", worktreeDir)
	}

	// Test setting via viper (simulates environment variable or config file)
	viper.Set("paths.worktree_dir", "/custom/worktrees")
	cfg := Get()
	if cfg.Paths.WorktreeDir != "/custom/worktrees" {
		t.Errorf("Paths.WorktreeDir = %q, want %q", cfg.Paths.WorktreeDir, "/custom/worktrees")
	}
}

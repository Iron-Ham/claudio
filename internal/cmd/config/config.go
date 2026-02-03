// Package config provides CLI commands for managing Claudio configuration.
package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	appconfig "github.com/Iron-Ham/claudio/internal/config"
	tuiconfig "github.com/Iron-Ham/claudio/internal/tui/config"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Wrapper functions for exec to allow testing
var execLookPath = exec.LookPath
var execCommand = exec.Command

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify Claudio configuration",
	Long: `View or modify Claudio configuration.

Without arguments, opens an interactive configuration UI.
Use 'config show' to display configuration non-interactively.
Use subcommands to modify settings or create a config file.`,
	RunE: runConfigInteractive,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value in the user's config file.

Keys use dot notation, e.g.:
  claudio config set completion.default_action auto_pr
  claudio config set tui.max_output_lines 2000
  claudio config set pr.use_ai false

Valid keys:
  completion.default_action   - Action when instance completes
                                Options: prompt, keep_branch, merge_staging, merge_main, auto_pr
  tui.auto_focus_on_input     - Auto-focus new instances (true/false)
  tui.max_output_lines        - Max output lines to display
  instance.output_buffer_size - Output buffer size in bytes
  instance.capture_interval_ms - Output capture interval in milliseconds
  instance.tmux_width         - tmux pane width
  instance.tmux_height        - tmux pane height
  ai.backend                  - AI backend to use (claude/codex)
  ai.claude.command           - Claude CLI command name/path
  ai.claude.skip_permissions  - Add --dangerously-skip-permissions (true/false)
  ai.codex.command            - Codex CLI command name/path
  ai.codex.approval_mode      - Codex approval mode: bypass, full-auto, default
  pr.draft                    - Create PRs as drafts by default (true/false)
  pr.auto_rebase              - Rebase on main before creating PR (true/false)
  pr.use_ai                   - Use AI backend to generate PR content (true/false)`,
	Args: cobra.ExactArgs(2),
	RunE: runConfigSet,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file",
	Long:  `Create a default config file at ~/.config/claudio/config.yaml with all available options.`,
	RunE:  runConfigInit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show the config file path",
	RunE:  runConfigPath,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open config file in your editor",
	Long: `Open the config file in your preferred editor.

Uses $EDITOR environment variable, or falls back to common editors (vim, nano, vi).
If no config file exists, creates one with default values first.`,
	RunE: runConfigEdit,
}

var configResetCmd = &cobra.Command{
	Use:   "reset [key]",
	Short: "Reset configuration to defaults",
	Long: `Reset configuration values to their defaults.

Without arguments, resets all configuration to defaults.
With a key argument, resets only that specific key.

Examples:
  claudio config reset           # Reset all to defaults
  claudio config reset pr.draft  # Reset only pr.draft to default`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigReset,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configResetCmd)
}

// Register adds all config-related commands to the given parent command.
// This is the main entry point for integrating the config subpackage with
// the root command.
func Register(parent *cobra.Command) {
	parent.AddCommand(configCmd)
}

func runConfigInteractive(cmd *cobra.Command, args []string) error {
	return tuiconfig.Run()
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg := appconfig.Get()

	fmt.Println("Current configuration:")
	fmt.Println()

	// Show where config is being read from
	if viper.ConfigFileUsed() != "" {
		fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Printf("Config file: (none - using defaults)\n")
	}
	fmt.Println()

	// Completion settings
	fmt.Println("completion:")
	fmt.Printf("  default_action: %s\n", cfg.Completion.DefaultAction)

	// TUI settings
	fmt.Println("tui:")
	fmt.Printf("  auto_focus_on_input: %v\n", cfg.TUI.AutoFocusOnInput)
	fmt.Printf("  max_output_lines: %d\n", cfg.TUI.MaxOutputLines)

	// Instance settings
	fmt.Println("instance:")
	fmt.Printf("  output_buffer_size: %d\n", cfg.Instance.OutputBufferSize)
	fmt.Printf("  capture_interval_ms: %d\n", cfg.Instance.CaptureIntervalMs)
	fmt.Printf("  tmux_width: %d\n", cfg.Instance.TmuxWidth)
	fmt.Printf("  tmux_height: %d\n", cfg.Instance.TmuxHeight)
	fmt.Printf("  tmux_history_limit: %d\n", cfg.Instance.TmuxHistoryLimit)

	// AI backend settings
	fmt.Println("ai:")
	fmt.Printf("  backend: %s\n", cfg.AI.Backend)
	fmt.Printf("  claude.command: %s\n", cfg.AI.Claude.Command)
	fmt.Printf("  claude.skip_permissions: %v\n", cfg.AI.Claude.SkipPermissions)
	fmt.Printf("  codex.command: %s\n", cfg.AI.Codex.Command)
	fmt.Printf("  codex.approval_mode: %s\n", cfg.AI.Codex.ApprovalMode)

	// PR settings
	fmt.Println("pr:")
	fmt.Printf("  draft: %v\n", cfg.PR.Draft)
	fmt.Printf("  auto_rebase: %v\n", cfg.PR.AutoRebase)
	fmt.Printf("  use_ai: %v\n", cfg.PR.UseAI)

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Validate the key exists
	validKeys := map[string]string{
		"completion.default_action":    "string",
		"tui.theme":                    "theme",
		"tui.auto_focus_on_input":      "bool",
		"tui.max_output_lines":         "int",
		"instance.output_buffer_size":  "int",
		"instance.capture_interval_ms": "int",
		"instance.tmux_width":          "int",
		"instance.tmux_height":         "int",
		"ai.backend":                   "backend",
		"ai.claude.command":            "string",
		"ai.claude.skip_permissions":   "bool",
		"ai.codex.command":             "string",
		"ai.codex.approval_mode":       "codex_approval",
		"pr.draft":                     "bool",
		"pr.auto_rebase":               "bool",
		"pr.use_ai":                    "bool",
	}

	keyType, ok := validKeys[key]
	if !ok {
		return fmt.Errorf("unknown configuration key: %s\nRun 'claudio config set --help' to see valid keys", key)
	}

	// Validate the value based on type
	var typedValue interface{}
	switch keyType {
	case "string":
		if key == "completion.default_action" && !appconfig.IsValidCompletionAction(value) {
			return fmt.Errorf("invalid value for %s: %s\nValid options: %s",
				key, value, strings.Join(appconfig.ValidCompletionActions(), ", "))
		}
		typedValue = value
	case "backend":
		if !slices.Contains(appconfig.ValidAIBackends(), value) {
			return fmt.Errorf("invalid value for %s: %s\nValid options: %s",
				key, value, strings.Join(appconfig.ValidAIBackends(), ", "))
		}
		typedValue = value
	case "codex_approval":
		if !slices.Contains(appconfig.ValidCodexApprovalModes(), value) {
			return fmt.Errorf("invalid value for %s: %s\nValid options: %s",
				key, value, strings.Join(appconfig.ValidCodexApprovalModes(), ", "))
		}
		typedValue = value
	case "theme":
		// Discover custom themes first
		_, _ = styles.DiscoverCustomThemes()
		if !styles.IsValidTheme(value) {
			return fmt.Errorf("invalid theme: %s\nValid options: %s",
				value, strings.Join(styles.ValidThemes(), ", "))
		}
		typedValue = value
	case "bool":
		if value != "true" && value != "false" {
			return fmt.Errorf("invalid value for %s: expected true or false", key)
		}
		typedValue = value == "true"
	case "int":
		intVal, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid value for %s: expected integer", key)
		}
		if intVal < 0 {
			return fmt.Errorf("invalid value for %s: must be non-negative", key)
		}
		typedValue = intVal
	}

	// Ensure config directory exists
	configDir := appconfig.ConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set the value in viper
	viper.Set(key, typedValue)

	// Write to config file
	configFile := appconfig.ConfigFile()
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Set %s = %v\n", key, typedValue)
	fmt.Printf("Config saved to %s\n", configFile)

	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	configDir := appconfig.ConfigDir()
	configFile := appconfig.ConfigFile()

	// Check if config file already exists
	if _, err := os.Stat(configFile); err == nil {
		return fmt.Errorf("config file already exists at %s\nUse 'claudio config set' to modify values", configFile)
	}

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Generate a commented config file
	configContent := `# Claudio Configuration
# See: https://github.com/Iron-Ham/claudio

# Action when an instance completes its task
# Options: prompt, keep_branch, merge_staging, merge_main, auto_pr
completion:
  default_action: prompt

# TUI (terminal user interface) settings
tui:
  # Automatically focus new instances for input
  auto_focus_on_input: true
  # Maximum number of output lines to display per instance
  max_output_lines: 1000

# Instance settings (advanced)
instance:
  # Output buffer size in bytes (default: 100000 = 100KB)
  output_buffer_size: 100000
  # How often to capture output from tmux in milliseconds
  capture_interval_ms: 100
  # tmux pane dimensions
  tmux_width: 200
  tmux_height: 50

# AI backend settings
ai:
  # Backend to use: claude or codex
  backend: claude
  claude:
    # Claude CLI command name/path
    command: claude
    # Add --dangerously-skip-permissions when starting Claude
    skip_permissions: true
  codex:
    # Codex CLI command name/path
    command: codex
    # Approval mode: bypass, full-auto, or default
    approval_mode: full-auto

# Pull request settings
pr:
  # Create PRs as drafts by default
  draft: false
  # Automatically rebase on main before creating PR
  auto_rebase: true
  # Use AI backend to generate PR title and description
  use_ai: true
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Created config file at %s\n", configFile)
	fmt.Println("Edit this file to customize Claudio's behavior.")

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	configFile := appconfig.ConfigFile()

	if viper.ConfigFileUsed() != "" {
		fmt.Printf("Active config: %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Printf("Default path: %s (not created)\n", configFile)
	}

	// Also show config search paths
	fmt.Println("\nSearch paths:")
	fmt.Printf("  1. %s\n", filepath.Join(appconfig.ConfigDir(), "config.yaml"))
	fmt.Printf("  2. $HOME/.config/claudio/config.yaml\n")
	fmt.Printf("  3. ./config.yaml (current directory)\n")
	fmt.Println("\nEnvironment variables: CLAUDIO_* (e.g., CLAUDIO_COMPLETION_DEFAULT_ACTION)")

	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	configFile := appconfig.ConfigFile()

	// Check if config file exists, if not create it
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Printf("Config file doesn't exist, creating with defaults...\n")
		if err := runConfigInit(cmd, args); err != nil {
			return err
		}
	}

	// Find an editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors
		for _, e := range []string{"vim", "nano", "vi"} {
			if _, err := execLookPath(e); err == nil {
				editor = e
				break
			}
		}
	}
	if editor == "" {
		return fmt.Errorf("no editor found. Set $EDITOR environment variable")
	}

	// Open the editor
	editorCmd := execCommand(editor, configFile)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}

	fmt.Printf("Config file saved: %s\n", configFile)
	return nil
}

func runConfigReset(cmd *cobra.Command, args []string) error {
	defaults := appconfig.Default()

	// Map of keys to their default values
	defaultValues := map[string]interface{}{
		"completion.default_action":    defaults.Completion.DefaultAction,
		"tui.auto_focus_on_input":      defaults.TUI.AutoFocusOnInput,
		"tui.max_output_lines":         defaults.TUI.MaxOutputLines,
		"instance.output_buffer_size":  defaults.Instance.OutputBufferSize,
		"instance.capture_interval_ms": defaults.Instance.CaptureIntervalMs,
		"instance.tmux_width":          defaults.Instance.TmuxWidth,
		"instance.tmux_height":         defaults.Instance.TmuxHeight,
		"ai.backend":                   defaults.AI.Backend,
		"ai.claude.command":            defaults.AI.Claude.Command,
		"ai.claude.skip_permissions":   defaults.AI.Claude.SkipPermissions,
		"ai.codex.command":             defaults.AI.Codex.Command,
		"ai.codex.approval_mode":       defaults.AI.Codex.ApprovalMode,
		"pr.draft":                     defaults.PR.Draft,
		"pr.auto_rebase":               defaults.PR.AutoRebase,
		"pr.use_ai":                    defaults.PR.UseAI,
	}

	if len(args) == 0 {
		// Reset all values
		for key, value := range defaultValues {
			viper.Set(key, value)
		}
		fmt.Println("Reset all configuration to defaults.")
	} else {
		// Reset specific key
		key := args[0]
		value, ok := defaultValues[key]
		if !ok {
			return fmt.Errorf("unknown configuration key: %s\nRun 'claudio config set --help' to see valid keys", key)
		}
		viper.Set(key, value)
		fmt.Printf("Reset %s to default: %v\n", key, value)
	}

	// Write to config file
	configFile := appconfig.ConfigFile()

	// Ensure config directory exists
	configDir := appconfig.ConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Config saved to %s\n", configFile)
	return nil
}

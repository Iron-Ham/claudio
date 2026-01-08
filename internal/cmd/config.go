package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify Claudio configuration",
	Long: `View or modify Claudio configuration.

Without arguments, displays the current configuration.
Use subcommands to modify settings or create a config file.`,
	RunE: runConfigShow,
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
  claudio config set session.max_instances 5

Valid keys:
  completion.default_action  - Action when instance completes
                               Options: prompt, keep_branch, merge_staging, merge_main, auto_pr
  tui.auto_focus_on_input    - Auto-focus new instances (true/false)
  tui.max_output_lines       - Max output lines to display
  session.max_instances      - Max simultaneous instances
  instance.output_buffer_size - Output buffer size in bytes
  instance.capture_interval_ms - Output capture interval in milliseconds
  instance.tmux_width        - tmux pane width
  instance.tmux_height       - tmux pane height`,
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

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configPathCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

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

	// Session settings
	fmt.Println("session:")
	fmt.Printf("  max_instances: %d\n", cfg.Session.MaxInstances)

	// Instance settings
	fmt.Println("instance:")
	fmt.Printf("  output_buffer_size: %d\n", cfg.Instance.OutputBufferSize)
	fmt.Printf("  capture_interval_ms: %d\n", cfg.Instance.CaptureIntervalMs)
	fmt.Printf("  tmux_width: %d\n", cfg.Instance.TmuxWidth)
	fmt.Printf("  tmux_height: %d\n", cfg.Instance.TmuxHeight)

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// Validate the key exists
	validKeys := map[string]string{
		"completion.default_action":    "string",
		"tui.auto_focus_on_input":      "bool",
		"tui.max_output_lines":         "int",
		"session.max_instances":        "int",
		"instance.output_buffer_size":  "int",
		"instance.capture_interval_ms": "int",
		"instance.tmux_width":          "int",
		"instance.tmux_height":         "int",
	}

	keyType, ok := validKeys[key]
	if !ok {
		return fmt.Errorf("unknown configuration key: %s\nRun 'claudio config set --help' to see valid keys", key)
	}

	// Validate the value based on type
	var typedValue interface{}
	switch keyType {
	case "string":
		if key == "completion.default_action" && !config.IsValidCompletionAction(value) {
			return fmt.Errorf("invalid value for %s: %s\nValid options: %s",
				key, value, strings.Join(config.ValidCompletionActions(), ", "))
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
	configDir := config.ConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set the value in viper
	viper.Set(key, typedValue)

	// Write to config file
	configFile := config.ConfigFile()
	if err := viper.WriteConfigAs(configFile); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Set %s = %v\n", key, typedValue)
	fmt.Printf("Config saved to %s\n", configFile)

	return nil
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	configDir := config.ConfigDir()
	configFile := config.ConfigFile()

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

# Session settings
session:
  # Maximum number of instances that can run simultaneously
  max_instances: 10

# Instance settings (advanced)
instance:
  # Output buffer size in bytes (default: 100000 = 100KB)
  output_buffer_size: 100000
  # How often to capture output from tmux in milliseconds
  capture_interval_ms: 100
  # tmux pane dimensions
  tmux_width: 200
  tmux_height: 50
`

	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Created config file at %s\n", configFile)
	fmt.Println("Edit this file to customize Claudio's behavior.")

	return nil
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	configFile := config.ConfigFile()

	if viper.ConfigFileUsed() != "" {
		fmt.Printf("Active config: %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Printf("Default path: %s (not created)\n", configFile)
	}

	// Also show config search paths
	fmt.Println("\nSearch paths:")
	fmt.Printf("  1. %s\n", filepath.Join(config.ConfigDir(), "config.yaml"))
	fmt.Printf("  2. $HOME/.config/claudio/config.yaml\n")
	fmt.Printf("  3. ./config.yaml (current directory)\n")
	fmt.Println("\nEnvironment variables: CLAUDIO_* (e.g., CLAUDIO_COMPLETION_DEFAULT_ACTION)")

	return nil
}

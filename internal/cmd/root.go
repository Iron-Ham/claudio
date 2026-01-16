// Package cmd provides the CLI command structure for Claudio.
// Commands are organized into domain-specific subpackages for better
// maintainability and easier onboarding.
//
// Subpackage organization:
//   - session/: Session lifecycle (start, stop, sessions, cleanup)
//   - planning/: Planning modes (plan, ultraplan, tripleshot)
//   - instance/: Instance management (add, remove, status, stats)
//   - observability/: Monitoring (logs, harvest)
//   - project/: Project-level operations (init, pr)
//   - config/: Configuration management
package cmd

import (
	"strings"

	"github.com/Iron-Ham/claudio/internal/cmd/config"
	"github.com/Iron-Ham/claudio/internal/cmd/instance"
	"github.com/Iron-Ham/claudio/internal/cmd/observability"
	"github.com/Iron-Ham/claudio/internal/cmd/planning"
	"github.com/Iron-Ham/claudio/internal/cmd/project"
	"github.com/Iron-Ham/claudio/internal/cmd/session"
	appconfig "github.com/Iron-Ham/claudio/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "claudio",
	Short: "Multi-instance Claude Code orchestrator",
	Long: `Claudio enables running multiple Claude Code instances simultaneously
on a single project using git worktrees, with a central orchestrator
managing coordination between instances.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringP("config", "c", "", "config file (default is $HOME/.config/claudio/config.yaml)")
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))

	// Register all command subpackages
	session.Register(rootCmd)
	planning.Register(rootCmd)
	instance.Register(rootCmd)
	observability.Register(rootCmd)
	project.Register(rootCmd)
	config.Register(rootCmd)
}

func initConfig() {
	// Set defaults first so they're available even without a config file
	appconfig.SetDefaults()

	if cfgFile := viper.GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(appconfig.ConfigDir())
		viper.AddConfigPath("$HOME/.config/claudio")
		viper.AddConfigPath(".")
	}

	// Set defaults for notification settings
	viper.SetDefault("notifications.idle_timeout", "3s")

	viper.AutomaticEnv()
	viper.SetEnvPrefix("CLAUDIO")
	// Replace dots with underscores for nested keys in env vars
	// e.g., CLAUDIO_COMPLETION_DEFAULT_ACTION for completion.default_action
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config file if it exists (ignore error if not found)
	_ = viper.ReadInConfig()
}

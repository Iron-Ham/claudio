package cmd

import (
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
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
}

func initConfig() {
	// Set defaults first so they're available even without a config file
	config.SetDefaults()

	if cfgFile := viper.GetString("config"); cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(config.ConfigDir())
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

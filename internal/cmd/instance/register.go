package instance

import "github.com/spf13/cobra"

// Register adds all instance-related commands to the given parent command.
// This is the main entry point for integrating the instance subpackage with
// the root command.
func Register(parent *cobra.Command) {
	RegisterAddCmd(parent)
	RegisterRemoveCmd(parent)
	RegisterStatusCmd(parent)
	RegisterStatsCmd(parent)
}

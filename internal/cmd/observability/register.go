package observability

import "github.com/spf13/cobra"

// Register adds all observability-related commands to the given parent command.
// This is the main entry point for integrating the observability subpackage with
// the root command.
func Register(parent *cobra.Command) {
	RegisterLogsCmd(parent)
	RegisterHarvestCmd(parent)
}

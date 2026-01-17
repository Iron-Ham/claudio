package planning

import "github.com/spf13/cobra"

// Register adds all planning-related commands to the given parent command.
// This is the main entry point for integrating the planning subpackage with
// the root command.
func Register(parent *cobra.Command) {
	RegisterPlanCmd(parent)
	RegisterUltraplanCmd(parent)
	RegisterTripleshotCmd(parent)
	RegisterRalphWiggumCmd(parent)
}

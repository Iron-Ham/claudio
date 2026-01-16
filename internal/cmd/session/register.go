package session

import "github.com/spf13/cobra"

// Register adds all session-related commands to the given parent command.
// This is the main entry point for integrating the session subpackage with
// the root command.
func Register(parent *cobra.Command) {
	RegisterStartCmd(parent)
	RegisterStopCmd(parent)
	RegisterSessionsCmd(parent)
	RegisterCleanupCmd(parent)
}

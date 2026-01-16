package project

import "github.com/spf13/cobra"

// Register adds all project-related commands to the given parent command.
// This is the main entry point for integrating the project subpackage with
// the root command.
func Register(parent *cobra.Command) {
	RegisterInitCmd(parent)
	RegisterPRCmd(parent)
}

package instance

import (
	"fmt"
	"os"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a specific instance and its worktree",
	Long: `Remove a specific backend instance, stopping it if running and cleaning up
its worktree and branch. Use --force to remove even if there are uncommitted changes.`,
	Args: cobra.ExactArgs(1),
	RunE: runRemove,
}

var forceRemove bool

func init() {
	removeCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "Force removal even with uncommitted changes")
}

// RegisterRemoveCmd registers the remove command with the given parent command.
func RegisterRemoveCmd(parent *cobra.Command) {
	parent.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	instanceID := args[0]

	// Create orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Load current session
	session, err := orch.LoadSession()
	if err != nil {
		return fmt.Errorf("no active session found: %w", err)
	}

	// Remove instance
	if err := orch.RemoveInstance(session, instanceID, forceRemove); err != nil {
		return fmt.Errorf("failed to remove instance: %w", err)
	}

	fmt.Printf("Removed instance %s\n", instanceID)
	return nil
}

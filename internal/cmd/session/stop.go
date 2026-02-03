package session

import (
	"fmt"
	"os"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop all instances and cleanup",
	Long: `Stop all running backend instances and optionally cleanup worktrees.
You will be prompted for what to do with each instance's work.`,
	RunE: runStop,
}

var forceStop bool

func init() {
	stopCmd.Flags().BoolVarP(&forceStop, "force", "f", false, "Force stop without prompts")
}

// RegisterStopCmd registers the stop command with the given parent command.
func RegisterStopCmd(parent *cobra.Command) {
	parent.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

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

	// Stop session
	if err := orch.StopSession(session, forceStop); err != nil {
		return fmt.Errorf("failed to stop session: %w", err)
	}

	fmt.Println("Session stopped successfully")
	return nil
}

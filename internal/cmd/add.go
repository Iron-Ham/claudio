package cmd

import (
	"fmt"
	"os"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [task description]",
	Short: "Add a new Claude instance with a task",
	Long: `Add a new Claude Code instance to the current session.
The instance will be created in its own worktree and start working on the specified task.`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	task := args[0]

	// Create orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Load current session
	session, err := orch.LoadSession()
	if err != nil {
		return fmt.Errorf("no active session found. Run 'claudio start' first: %w", err)
	}

	// Add instance
	instance, err := orch.AddInstance(session, task)
	if err != nil {
		return fmt.Errorf("failed to add instance: %w", err)
	}

	fmt.Printf("Added instance %s\n", instance.ID)
	fmt.Printf("Task: %s\n", instance.Task)
	fmt.Printf("Worktree: %s\n", instance.WorktreePath)
	return nil
}

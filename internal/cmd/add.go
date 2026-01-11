package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add [task description]",
	Short: "Add a new Claude instance with a task",
	Long: `Add a new Claude Code instance to the current session.
The instance will be created in its own worktree and start working on the specified task.

Task Chaining:
  Use --depends-on to create task chains where one instance starts after another completes.
  Dependencies can be specified by instance ID or by task name substring.

Examples:
  claudio add "Write tests for auth"
  claudio add "Write docs" --depends-on abc123
  claudio add "Write docs" --depends-on "Write tests"
  claudio add "Run integration tests" --depends-on abc123,def456`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

var (
	autoStart bool
	dependsOn string
)

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().BoolVarP(&autoStart, "start", "s", false, "Automatically start the instance after adding (ignored if --depends-on is set)")
	addCmd.Flags().StringVarP(&dependsOn, "depends-on", "d", "", "Instance ID(s) or task name(s) that must complete before this instance starts (comma-separated)")
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

	var instance *orchestrator.Instance

	// Check if this instance has dependencies
	if dependsOn != "" {
		// Parse comma-separated dependencies
		deps := parseDependencies(dependsOn)

		// Add instance with dependencies - auto-start when dependencies complete
		instance, err = orch.AddInstanceWithDependencies(session, task, deps, true)
		if err != nil {
			return fmt.Errorf("failed to add instance: %w", err)
		}

		fmt.Printf("Added instance %s (waiting for dependencies)\n", instance.ID)
		fmt.Printf("Task: %s\n", instance.Task)
		fmt.Printf("Worktree: %s\n", instance.WorktreePath)
		fmt.Printf("Depends on: %s\n", strings.Join(instance.DependsOn, ", "))

		// Check if dependencies are already met
		if session.AreDependenciesMet(instance) {
			fmt.Println("All dependencies already completed!")
			if err := orch.StartInstance(instance); err != nil {
				return fmt.Errorf("failed to start instance: %w", err)
			}
			fmt.Println("Instance started")
		} else {
			fmt.Println("Instance will auto-start when all dependencies complete")
		}
	} else {
		// Add instance without dependencies
		instance, err = orch.AddInstance(session, task)
		if err != nil {
			return fmt.Errorf("failed to add instance: %w", err)
		}

		fmt.Printf("Added instance %s\n", instance.ID)
		fmt.Printf("Task: %s\n", instance.Task)
		fmt.Printf("Worktree: %s\n", instance.WorktreePath)

		// Auto-start if requested
		if autoStart {
			if err := orch.StartInstance(instance); err != nil {
				return fmt.Errorf("failed to start instance: %w", err)
			}
			fmt.Println("Instance started")
		}
	}

	return nil
}

// parseDependencies splits a comma-separated list of dependencies and trims whitespace
func parseDependencies(deps string) []string {
	if deps == "" {
		return nil
	}

	parts := strings.Split(deps, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

package cmd

import (
	"fmt"
	"os"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current session status",
	Long:  `Display the status of the current Claudio session and all instances.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
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
		fmt.Println("No active session")
		return nil
	}

	fmt.Printf("Session: %s\n", session.Name)
	fmt.Printf("ID: %s\n", session.ID)
	fmt.Printf("Created: %s\n", session.Created.Format("2006-01-02 15:04:05"))
	fmt.Printf("Instances: %d\n\n", len(session.Instances))

	for i, inst := range session.Instances {
		fmt.Printf("[%d] %s (%s)\n", i+1, inst.ID, inst.Status)
		fmt.Printf("    Task: %s\n", inst.Task)
		fmt.Printf("    Branch: %s\n", inst.Branch)
		if len(inst.FilesModified) > 0 {
			fmt.Printf("    Files: %v\n", inst.FilesModified)
		}
		fmt.Println()
	}

	return nil
}

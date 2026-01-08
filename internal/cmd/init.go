package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Claudio in the current repository",
	Long: `Initialize Claudio in the current git repository.
This creates a .claudio directory to store session state and worktrees.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if we're in a git repository
	gitDir := filepath.Join(cwd, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("not a git repository (no .git directory found)")
	}

	// Initialize orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	if err := orch.Init(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	fmt.Println("Claudio initialized successfully!")
	fmt.Printf("Session directory: %s\n", filepath.Join(cwd, ".claudio"))
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/worktree"
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

	// Find the git repository root (may be in a parent directory)
	repoRoot, err := worktree.FindGitRoot(cwd)
	if err != nil {
		return fmt.Errorf("not a git repository (or any parent up to mount point)")
	}

	// Initialize orchestrator using the repo root
	orch, err := orchestrator.New(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	if err := orch.Init(); err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	fmt.Println("Claudio initialized successfully!")
	fmt.Printf("Session directory: %s\n", filepath.Join(repoRoot, ".claudio"))
	return nil
}

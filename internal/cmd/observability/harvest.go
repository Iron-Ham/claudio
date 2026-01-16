package observability

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// WorktreeStatus represents the status of a worktree
type WorktreeStatus struct {
	Path          string
	Branch        string
	Task          string
	HasChanges    bool
	ChangedFiles  []string
	CommitsBehind int
	CommitsAhead  int
}

var harvestCmd = &cobra.Command{
	Use:   "harvest",
	Short: "Review and commit work from completed instances",
	Long: `Harvest reviews all Claudio worktrees and helps you:
- See which worktrees have uncommitted changes
- Commit completed work
- Create pull requests for finished tasks
- Clean up abandoned worktrees`,
	RunE: runHarvest,
}

var (
	harvestAutoCommit bool
	harvestCreatePR   bool
	harvestCleanup    bool
	harvestAll        bool
)

func init() {
	harvestCmd.Flags().BoolVar(&harvestAutoCommit, "commit", false, "Auto-commit all uncommitted changes")
	harvestCmd.Flags().BoolVar(&harvestCreatePR, "pr", false, "Create PRs for committed branches")
	harvestCmd.Flags().BoolVar(&harvestCleanup, "cleanup", false, "Remove worktrees with no changes")
	harvestCmd.Flags().BoolVar(&harvestAll, "all", false, "Process all worktrees (commit + pr + cleanup)")
}

// RegisterHarvestCmd registers the harvest command with the given parent command.
func RegisterHarvestCmd(parent *cobra.Command) {
	parent.AddCommand(harvestCmd)
}

func runHarvest(cmd *cobra.Command, args []string) error {
	if harvestAll {
		harvestAutoCommit = true
		harvestCreatePR = true
		harvestCleanup = true
	}

	// Find claudio directory
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	claudioDir := filepath.Join(cwd, ".claudio")
	worktreesDir := filepath.Join(claudioDir, "worktrees")

	if _, err := os.Stat(worktreesDir); os.IsNotExist(err) {
		fmt.Println("No worktrees found. Nothing to harvest.")
		return nil
	}

	// Scan worktrees
	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return fmt.Errorf("failed to read worktrees directory: %w", err)
	}

	if len(entries) == 0 {
		fmt.Println("No worktrees found. Nothing to harvest.")
		return nil
	}

	fmt.Printf("Found %d worktree(s)\n\n", len(entries))

	var withChanges []WorktreeStatus
	var withoutChanges []WorktreeStatus

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		wtPath := filepath.Join(worktreesDir, entry.Name())
		status := getWorktreeStatus(wtPath)

		if status.HasChanges {
			withChanges = append(withChanges, status)
		} else {
			withoutChanges = append(withoutChanges, status)
		}
	}

	// Show worktrees with changes
	if len(withChanges) > 0 {
		fmt.Println("Worktrees with uncommitted changes:")
		fmt.Println(strings.Repeat("-", 60))

		for _, wt := range withChanges {
			fmt.Printf("\n* %s\n", wt.Branch)
			fmt.Printf("   Path: %s\n", wt.Path)
			fmt.Printf("   Files changed: %d\n", len(wt.ChangedFiles))
			for _, f := range wt.ChangedFiles {
				fmt.Printf("     - %s\n", f)
			}

			if harvestAutoCommit {
				if err := commitWorktree(wt); err != nil {
					fmt.Printf("   [ERROR] Failed to commit: %v\n", err)
				} else {
					fmt.Printf("   [OK] Committed\n")

					if harvestCreatePR {
						if err := createPR(wt); err != nil {
							fmt.Printf("   [ERROR] Failed to create PR: %v\n", err)
						} else {
							fmt.Printf("   [OK] PR created\n")
						}
					}
				}
			}
		}
		fmt.Println()
	}

	// Show worktrees without changes
	if len(withoutChanges) > 0 {
		fmt.Println("Worktrees with no changes:")
		fmt.Println(strings.Repeat("-", 60))

		for _, wt := range withoutChanges {
			fmt.Printf("   %s\n", wt.Branch)

			if harvestCleanup {
				if err := removeWorktree(wt); err != nil {
					fmt.Printf("   [ERROR] Failed to remove: %v\n", err)
				} else {
					fmt.Printf("   [OK] Removed\n")
				}
			}
		}
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Summary: %d with changes, %d without changes\n", len(withChanges), len(withoutChanges))

	if !harvestAutoCommit && len(withChanges) > 0 {
		fmt.Println("\nRun with --commit to auto-commit changes")
		fmt.Println("Run with --pr to also create pull requests")
	}
	if !harvestCleanup && len(withoutChanges) > 0 {
		fmt.Println("Run with --cleanup to remove empty worktrees")
	}
	if !harvestAll && (len(withChanges) > 0 || len(withoutChanges) > 0) {
		fmt.Println("Run with --all to commit, create PRs, and cleanup")
	}

	return nil
}

func getWorktreeStatus(wtPath string) WorktreeStatus {
	status := WorktreeStatus{
		Path: wtPath,
	}

	// Get branch name
	branchCmd := exec.Command("git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := branchCmd.Output()
	if err == nil {
		status.Branch = strings.TrimSpace(string(branchOut))
	}

	// Extract task from branch name (claudio/<id>-<task-slug>)
	if parts := strings.SplitN(status.Branch, "-", 2); len(parts) == 2 {
		status.Task = strings.ReplaceAll(parts[1], "-", " ")
	}

	// Get changed files
	statusCmd := exec.Command("git", "-C", wtPath, "status", "--porcelain")
	statusOut, err := statusCmd.Output()
	if err == nil && len(statusOut) > 0 {
		lines := strings.Split(strings.TrimSpace(string(statusOut)), "\n")
		for _, line := range lines {
			if len(line) > 3 {
				status.ChangedFiles = append(status.ChangedFiles, strings.TrimSpace(line[3:]))
			}
		}
		status.HasChanges = len(status.ChangedFiles) > 0
	}

	return status
}

func commitWorktree(wt WorktreeStatus) error {
	// Stage all changes
	addCmd := exec.Command("git", "-C", wt.Path, "add", "-A")
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// Create commit message from branch name
	message := fmt.Sprintf("feat: %s\n\nCompleted by Claudio instance.\nBranch: %s", wt.Task, wt.Branch)

	commitCmd := exec.Command("git", "-C", wt.Path, "commit", "-m", message)
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}

	return nil
}

func createPR(wt WorktreeStatus) error {
	// Push the branch
	pushCmd := exec.Command("git", "-C", wt.Path, "push", "-u", "origin", wt.Branch)
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}

	// Create PR using gh CLI
	title := wt.Task
	if title == "" {
		title = wt.Branch
	}

	body := fmt.Sprintf("## Summary\nCompleted by Claudio instance.\n\n## Branch\n`%s`\n\n## Changes\n", wt.Branch)
	for _, f := range wt.ChangedFiles {
		body += fmt.Sprintf("- `%s`\n", f)
	}

	prCmd := exec.Command("gh", "pr", "create",
		"--title", title,
		"--body", body,
		"--head", wt.Branch,
	)
	prCmd.Dir = wt.Path

	output, err := prCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr create failed: %s - %w", string(output), err)
	}

	fmt.Printf("   PR: %s", string(output))
	return nil
}

func removeWorktree(wt WorktreeStatus) error {
	// Remove the worktree
	removeCmd := exec.Command("git", "worktree", "remove", wt.Path, "--force")
	if err := removeCmd.Run(); err != nil {
		return fmt.Errorf("git worktree remove failed: %w", err)
	}

	// Optionally delete the branch
	branchCmd := exec.Command("git", "branch", "-D", wt.Branch)
	_ = branchCmd.Run() // Ignore errors - branch might not exist locally

	return nil
}

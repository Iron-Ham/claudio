package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/worktree"
	"github.com/spf13/cobra"
)

// CleanupResult holds information about resources to be cleaned up
type CleanupResult struct {
	StaleWorktrees    []StaleWorktree
	StaleBranches     []string
	OrphanedTmuxSess  []string
	EmptySessions     []*session.Info // Sessions with 0 instances
	ActiveInstanceIDs map[string]bool // IDs of instances in active session
}

// StaleWorktree represents a worktree that may need cleanup
type StaleWorktree struct {
	Path           string
	Branch         string
	HasUncommitted bool
	ExistsOnRemote bool
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up stale worktrees, branches, tmux sessions, and empty sessions",
	Long: `Cleanup removes orphaned resources that can accumulate over time:

- Worktrees: In .claudio/worktrees/ with no active session
- Branches: <prefix>/* branches not associated with active work
  (prefix is configured via branch.prefix, default: "claudio")
- Tmux sessions: Orphaned claudio-* tmux sessions
- Empty sessions: Sessions with 0 instances

Additional flags:
  --all-sessions    Kill ALL claudio-* tmux sessions
  --deep-clean      Remove session directories (empty only, or all with --all-sessions)

Use --dry-run to see what would be cleaned up without making changes.`,
	RunE: runCleanup,
}

var (
	cleanupDryRun      bool
	cleanupForce       bool
	cleanupWorktrees   bool
	cleanupBranches    bool
	cleanupTmux        bool
	cleanupSessions    bool
	cleanupAllSessions bool
	cleanupDeepClean   bool
)

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would be cleaned up without making changes")
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "Skip confirmation prompt")
	cleanupCmd.Flags().BoolVar(&cleanupWorktrees, "worktrees", false, "Clean up only worktrees")
	cleanupCmd.Flags().BoolVar(&cleanupBranches, "branches", false, "Clean up only branches")
	cleanupCmd.Flags().BoolVar(&cleanupTmux, "tmux", false, "Clean up only tmux sessions")
	cleanupCmd.Flags().BoolVar(&cleanupSessions, "sessions", false, "Clean up only empty sessions (0 instances)")
	cleanupCmd.Flags().BoolVar(&cleanupAllSessions, "all-sessions", false, "Kill all claudio-* tmux sessions")
	cleanupCmd.Flags().BoolVar(&cleanupDeepClean, "deep-clean", false, "Also remove session directories (empty only, or all with --all-sessions)")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// If no specific flags, clean all standard resources
	cleanAll := !cleanupWorktrees && !cleanupBranches && !cleanupTmux && !cleanupSessions && !cleanupAllSessions && !cleanupDeepClean

	// Discover stale resources
	result, err := discoverStaleResources(cwd)
	if err != nil {
		return fmt.Errorf("failed to discover stale resources: %w", err)
	}

	// Check if there's anything to clean
	hasWork := ((cleanAll || cleanupWorktrees) && len(result.StaleWorktrees) > 0) ||
		((cleanAll || cleanupBranches) && len(result.StaleBranches) > 0) ||
		((cleanAll || cleanupTmux) && len(result.OrphanedTmuxSess) > 0) ||
		((cleanAll || cleanupSessions || cleanupDeepClean) && len(result.EmptySessions) > 0) ||
		cleanupAllSessions // Always has work if killing all tmux sessions

	if !hasWork {
		fmt.Println("No stale resources found. Nothing to clean up.")
		return nil
	}

	// Show what will be cleaned
	printCleanupSummary(result, cleanAll)

	// If dry-run, stop here
	if cleanupDryRun {
		fmt.Println("\nDry run mode - no changes made.")
		return nil
	}

	// Confirm unless forced
	if !cleanupForce {
		fmt.Print("\nProceed with cleanup? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cleanup cancelled.")
			return nil
		}
	}

	// Perform cleanup
	return performCleanup(cwd, result, cleanAll)
}

func discoverStaleResources(baseDir string) (*CleanupResult, error) {
	result := &CleanupResult{
		ActiveInstanceIDs: make(map[string]bool),
	}

	claudioDir := filepath.Join(baseDir, ".claudio")
	worktreesDir := filepath.Join(claudioDir, "worktrees")

	// Get the configured branch prefix
	cfg := config.Get()
	branchPrefix := cfg.Branch.Prefix
	if branchPrefix == "" {
		branchPrefix = "claudio"
	}

	// Load active session to know which instances are active
	orch, err := orchestrator.New(baseDir)
	if err == nil {
		if session, err := orch.LoadSession(); err == nil {
			for _, inst := range session.Instances {
				result.ActiveInstanceIDs[inst.ID] = true
			}
		}
	}

	// Find stale worktrees
	result.StaleWorktrees = findStaleWorktrees(worktreesDir, result.ActiveInstanceIDs)

	// Find stale branches using configured prefix
	result.StaleBranches = findStaleBranches(baseDir, result.ActiveInstanceIDs, branchPrefix)

	// Find orphaned tmux sessions
	result.OrphanedTmuxSess = findOrphanedTmuxSessions(result.ActiveInstanceIDs)

	// Find empty sessions (0 instances)
	emptySessions, err := session.FindEmptySessions(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to discover empty sessions: %v\n", err)
	}
	result.EmptySessions = emptySessions

	return result, nil
}

func findStaleWorktrees(worktreesDir string, activeIDs map[string]bool) []StaleWorktree {
	var stale []StaleWorktree

	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		return stale
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		id := entry.Name()
		if activeIDs[id] {
			continue // Skip active instances
		}

		wtPath := filepath.Join(worktreesDir, id)
		sw := StaleWorktree{
			Path: wtPath,
		}

		// Get branch name
		branchCmd := exec.Command("git", "-C", wtPath, "rev-parse", "--abbrev-ref", "HEAD")
		if branchOut, err := branchCmd.Output(); err == nil {
			sw.Branch = strings.TrimSpace(string(branchOut))
		}

		// Check for uncommitted changes
		statusCmd := exec.Command("git", "-C", wtPath, "status", "--porcelain")
		if statusOut, err := statusCmd.Output(); err == nil {
			sw.HasUncommitted = len(strings.TrimSpace(string(statusOut))) > 0
		}

		// Check if branch exists on remote
		if sw.Branch != "" {
			remoteCmd := exec.Command("git", "-C", wtPath, "ls-remote", "--heads", "origin", sw.Branch)
			if remoteOut, err := remoteCmd.Output(); err == nil {
				sw.ExistsOnRemote = len(strings.TrimSpace(string(remoteOut))) > 0
			}
		}

		stale = append(stale, sw)
	}

	return stale
}

func findStaleBranches(baseDir string, activeIDs map[string]bool, branchPrefix string) []string {
	var stale []string

	// Get all local branches starting with the configured prefix
	cmd := exec.Command("git", "-C", baseDir, "branch", "--list", branchPrefix+"/*")
	output, err := cmd.Output()
	if err != nil {
		return stale
	}

	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		branch = strings.TrimPrefix(branch, "* ") // Remove current branch marker
		if branch == "" {
			continue
		}

		// Extract instance ID from branch name (<prefix>/<id>-<slug>)
		// The ID is the first segment after the prefix, before the first dash
		afterPrefix := strings.TrimPrefix(branch, branchPrefix+"/")
		parts := strings.SplitN(afterPrefix, "-", 2)
		if len(parts) < 1 {
			continue
		}
		instanceID := parts[0]

		// Skip if this branch belongs to an active instance
		if activeIDs[instanceID] {
			continue
		}

		// Check if branch exists on remote (skip if pushed)
		remoteCmd := exec.Command("git", "-C", baseDir, "ls-remote", "--heads", "origin", branch)
		if remoteOut, err := remoteCmd.Output(); err == nil && len(strings.TrimSpace(string(remoteOut))) > 0 {
			continue // Branch exists on remote, don't delete
		}

		stale = append(stale, branch)
	}

	return stale
}

func findOrphanedTmuxSessions(activeIDs map[string]bool) []string {
	var orphaned []string

	// List all tmux sessions
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return orphaned // tmux might not be running
	}

	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, sess := range sessions {
		sess = strings.TrimSpace(sess)
		if sess == "" {
			continue
		}

		// Check if it's a claudio session
		if !strings.HasPrefix(sess, "claudio-") {
			continue
		}

		// Extract instance ID
		instanceID := strings.TrimPrefix(sess, "claudio-")

		// Check if it belongs to an active instance
		if activeIDs[instanceID] {
			continue
		}

		orphaned = append(orphaned, sess)
	}

	return orphaned
}

func printCleanupSummary(result *CleanupResult, cleanAll bool) {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Stale Resources Found")
	fmt.Println(strings.Repeat("─", 60))

	if (cleanAll || cleanupWorktrees) && len(result.StaleWorktrees) > 0 {
		fmt.Printf("\nWorktrees (%d):\n", len(result.StaleWorktrees))
		for _, wt := range result.StaleWorktrees {
			status := ""
			if wt.HasUncommitted {
				status = " [uncommitted changes]"
			}
			if wt.ExistsOnRemote {
				status += " [pushed to remote]"
			}
			fmt.Printf("  - %s%s\n", filepath.Base(wt.Path), status)
			if wt.Branch != "" {
				fmt.Printf("    Branch: %s\n", wt.Branch)
			}
		}
	}

	if (cleanAll || cleanupBranches) && len(result.StaleBranches) > 0 {
		fmt.Printf("\nBranches (%d):\n", len(result.StaleBranches))
		for _, branch := range result.StaleBranches {
			fmt.Printf("  - %s\n", branch)
		}
	}

	if (cleanAll || cleanupTmux) && len(result.OrphanedTmuxSess) > 0 {
		fmt.Printf("\nOrphaned Tmux Sessions (%d):\n", len(result.OrphanedTmuxSess))
		for _, sess := range result.OrphanedTmuxSess {
			fmt.Printf("  - %s\n", sess)
		}
	}

	if (cleanAll || cleanupSessions || cleanupDeepClean) && len(result.EmptySessions) > 0 {
		fmt.Printf("\nEmpty Sessions (%d):\n", len(result.EmptySessions))
		for _, s := range result.EmptySessions {
			name := s.Name
			if name == "" {
				name = "(unnamed)"
			}
			fmt.Printf("  - %s - %s\n", session.TruncateID(s.ID, 8), name)
		}
	}

	if cleanupAllSessions {
		fmt.Println("\n--all-sessions: Will kill ALL claudio-* tmux sessions")
		if cleanupDeepClean {
			fmt.Println("--deep-clean: Will also remove ALL session directories")
		}
	} else if cleanupDeepClean && len(result.EmptySessions) > 0 {
		fmt.Println("\n--deep-clean: Will remove empty session directories listed above")
	}
}

func performCleanup(baseDir string, result *CleanupResult, cleanAll bool) error {
	fmt.Println()

	wt, err := worktree.New(baseDir)
	if err != nil {
		return fmt.Errorf("failed to create worktree manager: %w", err)
	}

	var totalRemoved int

	// Clean worktrees
	if cleanAll || cleanupWorktrees {
		for _, sw := range result.StaleWorktrees {
			// Safety: skip worktrees with uncommitted changes unless forced
			if sw.HasUncommitted && !cleanupForce {
				fmt.Printf("Skipping %s (has uncommitted changes, use --force to remove)\n", filepath.Base(sw.Path))
				continue
			}

			if err := wt.Remove(sw.Path); err != nil {
				fmt.Printf("Warning: failed to remove worktree %s: %v\n", filepath.Base(sw.Path), err)
				continue
			}
			fmt.Printf("Removed worktree: %s\n", filepath.Base(sw.Path))
			totalRemoved++

			// Also delete the branch if it's local-only
			if sw.Branch != "" && !sw.ExistsOnRemote {
				if err := wt.DeleteBranch(sw.Branch); err != nil {
					fmt.Printf("Warning: failed to delete branch %s: %v\n", sw.Branch, err)
				} else {
					fmt.Printf("Deleted branch: %s\n", sw.Branch)
				}
			}
		}
	}

	// Clean branches (only local-only branches not associated with worktrees we just cleaned)
	if cleanAll || cleanupBranches {
		for _, branch := range result.StaleBranches {
			// Check if we already deleted this branch with its worktree
			alreadyDeleted := false
			for _, sw := range result.StaleWorktrees {
				if sw.Branch == branch {
					alreadyDeleted = true
					break
				}
			}
			if alreadyDeleted {
				continue
			}

			if err := wt.DeleteBranch(branch); err != nil {
				fmt.Printf("Warning: failed to delete branch %s: %v\n", branch, err)
				continue
			}
			fmt.Printf("Deleted branch: %s\n", branch)
			totalRemoved++
		}
	}

	// Clean tmux sessions
	if cleanAll || cleanupTmux {
		for _, sess := range result.OrphanedTmuxSess {
			killCmd := exec.Command("tmux", "kill-session", "-t", sess)
			if err := killCmd.Run(); err != nil {
				fmt.Printf("Warning: failed to kill tmux session %s: %v\n", sess, err)
				continue
			}
			fmt.Printf("Killed tmux session: %s\n", sess)
			totalRemoved++
		}
	}

	// Handle --all-sessions: kill ALL claudio-* tmux sessions
	if cleanupAllSessions {
		killed := killAllClaudioTmuxSessions()
		totalRemoved += killed
	}

	// Clean empty sessions (part of default cleanup or with --sessions flag)
	if cleanAll || cleanupSessions || cleanupDeepClean {
		for _, s := range result.EmptySessions {
			if err := session.RemoveSession(baseDir, s.ID); err != nil {
				fmt.Printf("Warning: failed to remove session %s: %v\n", session.TruncateID(s.ID, 8), err)
				continue
			}
			name := s.Name
			if name == "" {
				name = "(unnamed)"
			}
			fmt.Printf("Removed empty session: %s - %s\n", session.TruncateID(s.ID, 8), name)
			totalRemoved++
		}
	}

	// Handle --deep-clean with --all-sessions: remove ALL session directories
	if cleanupDeepClean && cleanupAllSessions {
		removed := removeAllSessions(baseDir)
		totalRemoved += removed
	}

	fmt.Printf("\nCleanup complete. Removed %d resources.\n", totalRemoved)
	return nil
}

// killAllClaudioTmuxSessions kills all tmux sessions with claudio-* prefix
func killAllClaudioTmuxSessions() int {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// Check if it's just "no server running" which is expected
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "no server running") {
				return 0
			}
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to list tmux sessions: %v\n", err)
		return 0
	}

	var killed int
	sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, sess := range sessions {
		sess = strings.TrimSpace(sess)
		if sess == "" || !strings.HasPrefix(sess, "claudio-") {
			continue
		}

		killCmd := exec.Command("tmux", "kill-session", "-t", sess)
		if err := killCmd.Run(); err != nil {
			fmt.Printf("Warning: failed to kill tmux session %s: %v\n", sess, err)
			continue
		}
		fmt.Printf("Killed tmux session: %s\n", sess)
		killed++
	}
	return killed
}

// removeAllSessions removes all session directories
func removeAllSessions(baseDir string) int {
	sessions, err := session.ListSessions(baseDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to list sessions for removal: %v\n", err)
		return 0
	}

	var removed int
	for _, s := range sessions {
		// Skip locked sessions
		if s.IsLocked {
			fmt.Printf("Skipping locked session: %s\n", session.TruncateID(s.ID, 8))
			continue
		}

		if err := session.RemoveSession(baseDir, s.ID); err != nil {
			fmt.Printf("Warning: failed to remove session %s: %v\n", session.TruncateID(s.ID, 8), err)
			continue
		}
		name := s.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("Removed session: %s - %s\n", session.TruncateID(s.ID, 8), name)
		removed++
	}
	return removed
}

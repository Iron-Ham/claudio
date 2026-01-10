package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/spf13/cobra"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage Claudio sessions",
	Long:  `Commands for listing, attaching, and cleaning up Claudio sessions.`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all Claudio sessions",
	Long: `List all Claudio sessions with their status:
- Session ID and name
- Number of instances
- Lock status (whether another process is attached)
- Orphaned tmux sessions`,
	RunE: runSessionsList,
}

var sessionsAttachCmd = &cobra.Command{
	Use:   "attach <session-id>",
	Short: "Attach to an existing session",
	Long: `Attach to an existing Claudio session by ID.

This command will:
1. Load the session state
2. Acquire a lock to prevent concurrent access
3. Attempt to reconnect to any tmux sessions that are still running
4. Launch the TUI`,
	Args: cobra.ExactArgs(1),
	RunE: runSessionsAttach,
}

var sessionsRecoverCmd = &cobra.Command{
	Use:   "recover [session-id]",
	Short: "Recover a previous session (legacy)",
	Long: `Recover a previous Claudio session. If no session-id is provided,
attempts to recover a legacy single-session format.

This command is primarily for backwards compatibility.
Use 'claudio sessions attach <id>' for multi-session support.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSessionsRecover,
}

var sessionsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up stale session data",
	Long: `Clean up stale locks and orphaned tmux sessions.

This command will:
1. Remove stale lock files (from dead processes)
2. Kill any orphaned claudio-* tmux sessions
3. Optionally remove all session data (with --all flag)`,
	RunE: runSessionsClean,
}

var (
	cleanAll       bool
	cleanSessionID string
)

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsAttachCmd)
	sessionsCmd.AddCommand(sessionsRecoverCmd)
	sessionsCmd.AddCommand(sessionsCleanCmd)

	sessionsCleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Remove all session data")
	sessionsCleanCmd.Flags().StringVar(&cleanSessionID, "session", "", "Clean specific session by ID")
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// List all sessions
	sessions, err := session.ListSessions(cwd)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	// List all tmux sessions
	tmuxSessions, err := instance.ListClaudioTmuxSessions()
	if err != nil {
		// Not fatal, just can't show tmux status
		tmuxSessions = nil
	}

	fmt.Println(strings.Repeat("─", 70))
	fmt.Println("Claudio Sessions")
	fmt.Println(strings.Repeat("─", 70))

	if len(sessions) == 0 {
		// Check for legacy session
		legacyFile := fmt.Sprintf("%s/.claudio/session.json", cwd)
		if _, err := os.Stat(legacyFile); err == nil {
			fmt.Println("\nLegacy session found (pre-multi-session format)")
			fmt.Println("Run 'claudio start' to migrate it to the new format.")
		} else {
			fmt.Println("\nNo sessions found.")
			fmt.Println("Run 'claudio start' to create a new session.")
		}
	} else {
		fmt.Printf("\nFound %d session(s):\n\n", len(sessions))

		for _, s := range sessions {
			name := s.Name
			if name == "" {
				name = "(unnamed)"
			}

			lockStatus := "unlocked"
			if s.IsLocked {
				lockStatus = fmt.Sprintf("LOCKED (PID %d)", s.LockInfo.PID)
			}

			fmt.Printf("  Session: %s\n", s.ID)
			fmt.Printf("    Name:      %s\n", name)
			fmt.Printf("    Created:   %s\n", s.Created.Format(time.RFC822))
			fmt.Printf("    Instances: %d\n", s.InstanceCount)
			fmt.Printf("    Status:    %s\n", lockStatus)

			// Count running tmux sessions for this session
			runningCount := 0
			prefix := fmt.Sprintf("claudio-%s-", s.ID)
			for _, ts := range tmuxSessions {
				if strings.HasPrefix(ts, prefix) {
					runningCount++
				}
			}
			if runningCount > 0 {
				fmt.Printf("    Running:   %d tmux session(s)\n", runningCount)
			}
			fmt.Println()
		}
	}

	// Show orphaned tmux sessions (any that don't match known session patterns)
	var orphaned []string
	for _, ts := range tmuxSessions {
		sessionID, _ := instance.ExtractSessionAndInstanceID(ts)
		// Check if this session ID exists
		found := false
		if sessionID != "" {
			for _, s := range sessions {
				if s.ID == sessionID {
					found = true
					break
				}
			}
		}
		// Legacy format (no session ID) is also considered orphaned in multi-session context
		if !found {
			orphaned = append(orphaned, ts)
		}
	}

	if len(orphaned) > 0 {
		fmt.Printf("Orphaned tmux sessions (%d):\n", len(orphaned))
		for _, ts := range orphaned {
			fmt.Printf("  - %s\n", ts)
		}
		fmt.Println("\nRun 'claudio sessions clean' to remove orphaned sessions.")
	}

	fmt.Println(strings.Repeat("─", 70))

	if len(sessions) > 0 {
		fmt.Println("\nTo attach to a session: claudio sessions attach <session-id>")
		fmt.Println("To create a new session: claudio start --new")
	}

	return nil
}

func runSessionsAttach(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	sessionID := args[0]

	// Find session by ID or prefix
	sessions, err := session.ListSessions(cwd)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	var targetSession *session.Info
	for _, s := range sessions {
		if s.ID == sessionID || strings.HasPrefix(s.ID, sessionID) {
			targetSession = s
			break
		}
	}

	if targetSession == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if targetSession.IsLocked {
		return fmt.Errorf("session %s is locked by PID %d. Use 'claudio sessions list' to see status",
			targetSession.ID, targetSession.LockInfo.PID)
	}

	fmt.Printf("Attaching to session %s...\n", targetSession.ID)

	cfg := config.Get()
	return attachToSession(cwd, targetSession.ID, cfg)
}

func runSessionsRecover(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	cfg := config.Get()

	// If session ID is provided, use attach
	if len(args) > 0 {
		return attachToSession(cwd, args[0], cfg)
	}

	// Try to recover legacy session
	legacyFile := fmt.Sprintf("%s/.claudio/session.json", cwd)
	if _, err := os.Stat(legacyFile); err != nil {
		return fmt.Errorf("no legacy session found. Use 'claudio sessions attach <id>' for multi-session")
	}

	fmt.Println("Recovering legacy session...")
	fmt.Println("This session will be migrated to the new multi-session format.")
	fmt.Println()

	return migrateAndStartLegacySession(cwd, "", cfg)
}

func runSessionsClean(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Clean stale locks first
	cleaned, err := session.CleanupStaleLocks(cwd)
	if err != nil {
		fmt.Printf("Warning: failed to clean stale locks: %v\n", err)
	}
	if len(cleaned) > 0 {
		fmt.Printf("Cleaned %d stale lock(s)\n", len(cleaned))
		for _, id := range cleaned {
			fmt.Printf("  - Session %s\n", id)
		}
	}

	// List all sessions to find orphaned tmux sessions
	sessions, _ := session.ListSessions(cwd)
	tmuxSessions, _ := instance.ListClaudioTmuxSessions()

	// Find and kill orphaned tmux sessions
	var orphanedKilled int
	for _, ts := range tmuxSessions {
		sessionID, _ := instance.ExtractSessionAndInstanceID(ts)
		found := false
		if sessionID != "" {
			for _, s := range sessions {
				if s.ID == sessionID {
					found = true
					break
				}
			}
		}
		if !found {
			// Kill orphaned session
			killCmd := fmt.Sprintf("tmux kill-session -t %s", ts)
			if _, err := runCommand(killCmd); err == nil {
				orphanedKilled++
			}
		}
	}

	if orphanedKilled > 0 {
		fmt.Printf("Killed %d orphaned tmux session(s)\n", orphanedKilled)
	}

	// Clean specific session if --session flag is set
	if cleanSessionID != "" {
		sessionDir := session.GetSessionDir(cwd, cleanSessionID)
		if err := os.RemoveAll(sessionDir); err != nil {
			return fmt.Errorf("failed to remove session %s: %w", cleanSessionID, err)
		}
		fmt.Printf("Removed session: %s\n", cleanSessionID)
	}

	// Remove all session data if --all flag is set
	if cleanAll {
		// Remove legacy session file
		legacyFile := fmt.Sprintf("%s/.claudio/session.json", cwd)
		if err := os.Remove(legacyFile); err == nil {
			fmt.Println("Removed legacy session file")
		}

		// Remove all session directories
		sessionsDir := session.GetSessionsDir(cwd)
		if err := os.RemoveAll(sessionsDir); err != nil {
			return fmt.Errorf("failed to remove sessions directory: %w", err)
		}
		fmt.Println("Removed all session data")
	}

	if len(cleaned) == 0 && orphanedKilled == 0 && cleanSessionID == "" && !cleanAll {
		fmt.Println("No stale resources to clean")
	}

	return nil
}

// runCommand executes a shell command and returns output
func runCommand(cmd string) (string, error) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	out, err := exec.Command(parts[0], parts[1:]...).Output()
	return string(out), err
}

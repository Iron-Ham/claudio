package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var sessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage Claudio sessions",
	Long:  `Commands for listing, recovering, and cleaning up Claudio sessions.`,
}

var sessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recoverable sessions and orphaned tmux sessions",
	Long: `List all Claudio sessions that can be recovered, including:
- Existing session files with their instances
- Orphaned tmux sessions (claudio-* sessions without a session file)`,
	RunE: runSessionsList,
}

var sessionsRecoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Recover a previous session",
	Long: `Recover a previous Claudio session by reconnecting to any
still-running tmux sessions and launching the TUI.

This command will:
1. Load the session state from .claudio/session.json
2. Attempt to reconnect to any tmux sessions that are still running
3. Mark instances with missing tmux sessions as paused
4. Launch the TUI to continue working`,
	RunE: runSessionsRecover,
}

var sessionsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up stale session data",
	Long: `Clean up orphaned tmux sessions and optionally remove session files.

This command will:
1. Kill any orphaned claudio-* tmux sessions
2. Optionally remove the session file (with --all flag)`,
	RunE: runSessionsClean,
}

var (
	cleanAll bool
)

func init() {
	rootCmd.AddCommand(sessionsCmd)
	sessionsCmd.AddCommand(sessionsListCmd)
	sessionsCmd.AddCommand(sessionsRecoverCmd)
	sessionsCmd.AddCommand(sessionsCleanCmd)

	sessionsCleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Also remove session file")
}

func runSessionsList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Check for existing session file
	hasSession := orch.HasExistingSession()

	// List orphaned tmux sessions
	tmuxSessions, err := instance.ListClaudioTmuxSessions()
	if err != nil {
		return fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Claudio Session Status")
	fmt.Println(strings.Repeat("─", 60))

	if hasSession {
		session, err := orch.LoadSession()
		if err != nil {
			fmt.Printf("\nSession file exists but failed to load: %v\n", err)
		} else {
			fmt.Printf("\nSession: %s (ID: %s)\n", session.Name, session.ID)
			fmt.Printf("Created: %s\n", session.Created.Format(time.RFC822))
			fmt.Printf("Instances: %d\n\n", len(session.Instances))

			if len(session.Instances) > 0 {
				fmt.Println("Instances:")
				for _, inst := range session.Instances {
					sessionName := fmt.Sprintf("claudio-%s", inst.ID)
					tmuxStatus := "stopped"

					// Check if tmux session exists
					for _, ts := range tmuxSessions {
						if ts == sessionName {
							tmuxStatus = "running"
							break
						}
					}

					fmt.Printf("  [%s] %s - %s\n", inst.Status, inst.ID, truncateTask(inst.Task, 40))
					fmt.Printf("       Branch: %s\n", inst.Branch)
					fmt.Printf("       Tmux: %s\n", tmuxStatus)
				}
			}
		}
	} else {
		fmt.Println("\nNo session file found.")
	}

	// Show orphaned tmux sessions
	orphaned, _ := orch.GetOrphanedTmuxSessions()
	if len(orphaned) > 0 {
		fmt.Printf("\nOrphaned tmux sessions (%d):\n", len(orphaned))
		for _, sess := range orphaned {
			instanceID := instance.ExtractInstanceIDFromSession(sess)
			fmt.Printf("  - %s (instance: %s)\n", sess, instanceID)
		}
		fmt.Println("\nRun 'claudio sessions clean' to remove orphaned sessions.")
	} else if len(tmuxSessions) == 0 {
		fmt.Println("\nNo claudio tmux sessions running.")
	}

	fmt.Println(strings.Repeat("─", 60))

	if hasSession {
		fmt.Println("\nTo recover this session: claudio sessions recover")
		fmt.Println("To start fresh: claudio sessions clean --all && claudio start")
	}

	return nil
}

func runSessionsRecover(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	if !orch.HasExistingSession() {
		return fmt.Errorf("no session found to recover. Use 'claudio start' to create a new session")
	}

	fmt.Println("Recovering session...")

	session, reconnected, err := orch.RecoverSession()
	if err != nil {
		return fmt.Errorf("failed to recover session: %w", err)
	}

	fmt.Printf("Loaded session: %s\n", session.Name)
	fmt.Printf("Total instances: %d\n", len(session.Instances))
	fmt.Printf("Reconnected to tmux: %d\n", len(reconnected))

	if len(reconnected) > 0 {
		fmt.Println("Reconnected instances:")
		for _, id := range reconnected {
			fmt.Printf("  - %s\n", id)
		}
	}

	// Get terminal dimensions and set them on the orchestrator before launching TUI
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	fmt.Println("\nLaunching TUI...")

	// Launch TUI
	app := tui.New(orch, session)
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

func runSessionsClean(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Load session first to find orphaned sessions
	if orch.HasExistingSession() {
		orch.LoadSession()
	}

	// Clean orphaned tmux sessions
	cleaned, err := orch.CleanOrphanedTmuxSessions()
	if err != nil {
		fmt.Printf("Warning: failed to clean some tmux sessions: %v\n", err)
	}

	if cleaned > 0 {
		fmt.Printf("Cleaned %d orphaned tmux session(s)\n", cleaned)
	} else {
		fmt.Println("No orphaned tmux sessions to clean")
	}

	// Remove session file if --all flag is set
	if cleanAll {
		sessionFile := fmt.Sprintf("%s/.claudio/session.json", cwd)
		if err := os.Remove(sessionFile); err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove session file: %w", err)
			}
			fmt.Println("No session file to remove")
		} else {
			fmt.Println("Removed session file")
		}
	}

	return nil
}

func truncateTask(task string, maxLen int) string {
	task = strings.ReplaceAll(task, "\n", " ")
	if len(task) > maxLen {
		return task[:maxLen-3] + "..."
	}
	return task
}

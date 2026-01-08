package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var startCmd = &cobra.Command{
	Use:   "start [session-name]",
	Short: "Start a new Claudio session",
	Long: `Start a new Claudio session with an optional name.
This launches the TUI dashboard where you can add and manage Claude instances.

If a previous session exists, you will be prompted to recover it or start fresh.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStart,
}

var (
	forceNew bool
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().BoolVar(&forceNew, "new", false, "Force start a new session, replacing any existing one")
}

func runStart(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	sessionName := ""
	if len(args) > 0 {
		sessionName = args[0]
	}

	// Check for stale resources if configured
	if viper.GetBool("cleanup.warn_on_stale") {
		checkStaleResourcesWarning(cwd)
	}

	// Create orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	var session *orchestrator.Session

	// Check for existing session
	if !forceNew && orch.HasExistingSession() {
		// Prompt user for what to do
		action, err := promptSessionAction()
		if err != nil {
			return err
		}

		switch action {
		case "recover":
			fmt.Println("Recovering session...")
			session, reconnected, err := orch.RecoverSession()
			if err != nil {
				return fmt.Errorf("failed to recover session: %w", err)
			}
			if len(reconnected) > 0 {
				fmt.Printf("Reconnected to %d running instance(s)\n", len(reconnected))
			}

			// Get terminal dimensions and set them on the orchestrator before launching TUI
			if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
				contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
				if contentWidth > 0 && contentHeight > 0 {
					orch.SetDisplayDimensions(contentWidth, contentHeight)
				}
			}

			// Launch TUI with recovered session
			app := tui.New(orch, session)
			if err := app.Run(); err != nil {
				return fmt.Errorf("TUI error: %w", err)
			}
			return nil

		case "new":
			// Clean up orphaned tmux sessions before starting fresh
			orch.LoadSession()
			cleaned, _ := orch.CleanOrphanedTmuxSessions()
			if cleaned > 0 {
				fmt.Printf("Cleaned %d orphaned tmux session(s)\n", cleaned)
			}
			// Continue to start new session below

		case "quit":
			fmt.Println("Use 'claudio sessions list' to see session details")
			fmt.Println("Use 'claudio sessions recover' to recover the existing session")
			return nil
		}
	}

	// Start a new session
	session, err = orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Get terminal dimensions and set them on the orchestrator before launching TUI
	// This ensures that any instances started from the TUI have the correct initial size
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI
	app := tui.New(orch, session)
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// promptSessionAction prompts the user to choose what to do with an existing session
func promptSessionAction() (string, error) {
	fmt.Println("\nAn existing session was found.")
	fmt.Println("What would you like to do?")
	fmt.Println("  [r] Recover - Resume the existing session")
	fmt.Println("  [n] New     - Start fresh (cleans up old session)")
	fmt.Println("  [q] Quit    - Exit without changes")
	fmt.Print("\nChoice [r/n/q]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "r", "recover", "":
		return "recover", nil
	case "n", "new":
		return "new", nil
	case "q", "quit":
		return "quit", nil
	default:
		fmt.Printf("Unknown option '%s', defaulting to recover\n", input)
		return "recover", nil
	}
}

// checkStaleResourcesWarning checks for stale resources and prints a warning if found
func checkStaleResourcesWarning(baseDir string) {
	worktreesDir := filepath.Join(baseDir, ".claudio", "worktrees")

	var staleCount int

	// Count stale worktrees (directories in worktrees/ with no active session)
	entries, err := os.ReadDir(worktreesDir)
	if err == nil {
		staleCount += len(entries)
	}

	// Count stale branches
	cmd := exec.Command("git", "-C", baseDir, "branch", "--list", "claudio/*")
	if output, err := cmd.Output(); err == nil {
		branches := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, b := range branches {
			if strings.TrimSpace(b) != "" {
				staleCount++
			}
		}
	}

	// Count orphaned tmux sessions
	tmuxCmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	if output, err := tmuxCmd.Output(); err == nil {
		sessions := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, s := range sessions {
			if strings.HasPrefix(strings.TrimSpace(s), "claudio-") {
				staleCount++
			}
		}
	}

	if staleCount > 0 {
		fmt.Printf("Warning: Found %d potentially stale resources. Run 'claudio cleanup --dry-run' to review.\n\n", staleCount)
	}
}

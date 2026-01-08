package cmd

import (
	"fmt"
	"os"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var startCmd = &cobra.Command{
	Use:   "start [session-name]",
	Short: "Start a new Claudio session",
	Long: `Start a new Claudio session with an optional name.
This launches the TUI dashboard where you can add and manage Claude instances.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
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

	// Create orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Start a new session
	session, err := orch.StartSession(sessionName)
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

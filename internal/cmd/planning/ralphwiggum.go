package planning

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	orchsession "github.com/Iron-Ham/claudio/internal/orchestrator/session"
	sessutil "github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/Iron-Ham/claudio/internal/util"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var ralphCmd = &cobra.Command{
	Use:   "ralph [task]",
	Short: "Start a Ralph Wiggum iterative loop session",
	Long: `Ralph Wiggum mode implements continuous self-referential feedback loops where
Claude works on a task iteratively until a completion promise is signaled.

EXPERIMENTAL: This feature is experimental and may change or be removed.

How it works:
1. Claude receives the task and begins working
2. When Claude completes an iteration, the same prompt is fed back
3. Claude sees its previous work (files, git history) and continues
4. The loop continues until Claude outputs: <promise>COMPLETION_PROMISE</promise>
5. Or until the maximum number of iterations is reached

This is useful for:
- Tasks with clear verification criteria (tests must pass)
- Iterative refinement of complex implementations
- Tasks where multiple attempts may be needed

Examples:
  # Start Ralph Wiggum with a task (default promise is "DONE")
  claudio ralph "Fix all failing tests"

  # Use a custom completion promise
  claudio ralph --promise "ALL_TESTS_PASSING" "Implement the auth module"

  # Set a maximum number of iterations
  claudio ralph --max-iterations 10 "Optimize the database queries"

  # Require manual confirmation between iterations
  claudio ralph --no-auto-continue "Refactor the API endpoints"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRalphWiggum,
}

var (
	ralphCompletionPromise string
	ralphMaxIterations     int
	ralphAutoContinue      bool
)

func init() {
	ralphCmd.Flags().StringVar(&ralphCompletionPromise, "promise", "DONE", "Completion promise text that signals task completion")
	ralphCmd.Flags().IntVar(&ralphMaxIterations, "max-iterations", 50, "Maximum number of iterations (0 for unlimited)")
	ralphCmd.Flags().BoolVar(&ralphAutoContinue, "auto-continue", true, "Automatically continue to next iteration")
}

// RegisterRalphWiggumCmd registers the ralph command with the given parent command.
func RegisterRalphWiggumCmd(parent *cobra.Command) {
	parent.AddCommand(ralphCmd)
}

func runRalphWiggum(cmd *cobra.Command, args []string) error {
	// Check if experimental feature is enabled
	cfg := config.Get()
	if !cfg.Experimental.RalphWiggum {
		return fmt.Errorf("ralph wiggum mode is an experimental feature.\nEnable it by setting 'experimental.ralph_wiggum: true' in your config file")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get task from args or prompt
	var task string
	if len(args) > 0 {
		task = args[0]
	} else {
		task, err = promptRalphWiggumTask()
		if err != nil {
			return err
		}
	}

	// Generate a new session ID for this Ralph Wiggum session
	sessionID := orchsession.GenerateID()

	// Create Ralph Wiggum configuration
	ralphConfig := orchestrator.RalphWiggumConfig{
		CompletionPromise: ralphCompletionPromise,
		MaxIterations:     ralphMaxIterations,
		AutoContinue:      ralphAutoContinue,
	}

	// Create logger if enabled
	sessionDir := sessutil.GetSessionDir(cwd, sessionID)
	logger := createLogger(sessionDir, cfg)
	defer func() { _ = logger.Close() }()

	// Create orchestrator with multi-session support
	orch, err := orchestrator.NewWithSession(cwd, sessionID, cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Set the logger on the orchestrator
	orch.SetLogger(logger)

	// Start a new session for the Ralph Wiggum loop
	words := strings.Fields(task)
	if len(words) > 3 {
		words = words[:3]
	}
	sessionName := "ralph-" + SlugifyWords(words)

	session, err := orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Log startup
	logger.Info("ralph wiggum started",
		"session_id", sessionID,
		"task", util.TruncateString(task, 100),
		"completion_promise", ralphConfig.CompletionPromise,
		"max_iterations", ralphConfig.MaxIterations,
		"auto_continue", ralphConfig.AutoContinue,
	)

	// Create Ralph Wiggum session
	ralphSession := orchestrator.NewRalphWiggumSession(task, ralphConfig)

	// Link Ralph Wiggum session to main session for persistence
	session.RalphWiggums = append(session.RalphWiggums, ralphSession)

	// Create coordinator with logger
	coordinator := orchestrator.NewRalphWiggumCoordinator(orch, session, ralphSession, logger)

	// Get terminal dimensions
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI with Ralph Wiggum mode
	app := tui.NewWithRalphWiggum(orch, session, coordinator, logger.WithSession(session.ID))
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// promptRalphWiggumTask prompts the user to enter a task
func promptRalphWiggumTask() (string, error) {
	fmt.Println("\nRalph Wiggum Mode")
	fmt.Println("=================")
	fmt.Println("Enter a task for Claude to work on iteratively.")
	fmt.Println("Claude will keep working until the completion promise is output.")
	fmt.Println()
	fmt.Print("Task: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("task cannot be empty")
	}

	return input, nil
}

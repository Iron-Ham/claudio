package cmd

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
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var tripleshotCmd = &cobra.Command{
	Use:   "tripleshot [task]",
	Short: "Start a triple-shot session where three instances solve the same problem",
	Long: `Triple-shot mode runs three Claude instances in parallel on the same task,
then uses a fourth "judge" instance to evaluate all three solutions and determine
which one is best - or if elements from multiple solutions can be combined.

This approach is useful for:
- Complex problems where different approaches might yield different solutions
- Getting diverse perspectives on a problem before committing to one approach
- Situations where you want the best solution, not just the first one

The process has three phases:
1. WORKING: Three Claude instances work on the task independently
2. EVALUATING: A judge instance reviews all three solutions
3. COMPLETE: The judge provides an evaluation with a recommended approach

Guided Divergence:
You can specify different approaches for each of the three instances using --approach
flags. This lets you explore specific solution strategies rather than letting each
instance choose its own approach.

Examples:
  # Start triple-shot with a task
  claudio tripleshot "Implement a rate limiter for the API"

  # Start with auto-approve (apply winning solution automatically)
  claudio tripleshot --auto-approve "Refactor the authentication module"

  # Guided divergence - specify approaches for each instance
  claudio tripleshot "Implement sorting" \
    --approach "Use quicksort algorithm" \
    --approach "Use merge sort algorithm" \
    --approach "Use heap sort algorithm"

  # Partial guidance - only specify some approaches (others choose freely)
  claudio tripleshot "Build a cache" \
    --approach "Use in-memory LRU cache" \
    --approach "" \
    --approach "Use Redis-based distributed cache"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTripleshot,
}

var (
	tripleshotAutoApprove bool
	tripleshotApproaches  []string
)

func init() {
	rootCmd.AddCommand(tripleshotCmd)

	tripleshotCmd.Flags().BoolVar(&tripleshotAutoApprove, "auto-approve", false, "Auto-approve applying the winning solution")
	tripleshotCmd.Flags().StringArrayVar(&tripleshotApproaches, "approach", nil, "Specify an approach for one of the three instances (can be used up to 3 times)")
}

func runTripleshot(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get task from args or prompt
	var task string
	if len(args) > 0 {
		task = args[0]
	} else {
		task, err = promptTripleShotTask()
		if err != nil {
			return err
		}
	}

	// Generate a new session ID for this triple-shot
	sessionID := orchsession.GenerateID()
	cfg := config.Get()

	// Create triple-shot configuration
	tripleConfig := orchestrator.DefaultTripleShotConfig()
	tripleConfig.AutoApprove = tripleshotAutoApprove

	// Set guided divergence approaches if specified
	if len(tripleshotApproaches) > 0 {
		if len(tripleshotApproaches) > 3 {
			return fmt.Errorf("at most 3 approaches can be specified (one per instance)")
		}
		// Pad to 3 elements if fewer were provided
		approaches := make([]string, 3)
		copy(approaches, tripleshotApproaches)
		tripleConfig.Approaches = approaches
	}

	// Create logger if enabled
	sessionDir := sessutil.GetSessionDir(cwd, sessionID)
	logger := CreateLogger(sessionDir, cfg)
	defer func() { _ = logger.Close() }()

	// Create orchestrator with multi-session support
	orch, err := orchestrator.NewWithSession(cwd, sessionID, cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Set the logger on the orchestrator
	orch.SetLogger(logger)

	// Start a new session for the triple-shot
	words := strings.Fields(task)
	if len(words) > 3 {
		words = words[:3]
	}
	sessionName := "tripleshot-" + slugifyWords(words)

	session, err := orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Log startup
	logger.Info("tripleshot started",
		"session_id", sessionID,
		"task", truncateString(task, 100),
		"auto_approve", tripleConfig.AutoApprove,
	)

	// Create triple-shot session
	tripleSession := orchestrator.NewTripleShotSession(task, tripleConfig)

	// Link triple-shot session to main session for persistence
	session.TripleShot = tripleSession

	// Create coordinator with logger
	coordinator := orchestrator.NewTripleShotCoordinator(orch, session, tripleSession, logger)

	// Get terminal dimensions
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI with triple-shot mode
	app := tui.NewWithTripleShot(orch, session, coordinator, logger.WithSession(session.ID))
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// promptTripleShotTask prompts the user to enter a task
func promptTripleShotTask() (string, error) {
	fmt.Println("\nTriple-Shot Mode")
	fmt.Println("================")
	fmt.Println("Enter a task for three Claude instances to solve independently.")
	fmt.Println("A fourth instance will then evaluate all solutions and pick the best one.")
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

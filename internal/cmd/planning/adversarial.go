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

var adversarialCmd = &cobra.Command{
	Use:   "adversarial [task]",
	Short: "Start an adversarial review session with implementer-reviewer feedback loop",
	Long: `Adversarial review mode creates a feedback loop between an IMPLEMENTER and a REVIEWER:

1. The IMPLEMENTER works on the task and submits their work for review
2. The REVIEWER critically examines the work and provides detailed feedback
3. If issues are found, the IMPLEMENTER addresses them and resubmits
4. This loop continues until the REVIEWER approves the implementation

This approach is useful for:
- Complex implementations that benefit from critical review
- Ensuring high code quality through iterative refinement
- Catching edge cases and issues before code is merged
- Learning how critical code review can improve implementations

The process continues until:
- The reviewer approves the work (success)
- Maximum iterations are reached (configurable)
- Either instance encounters a fatal error

Examples:
  # Start adversarial review with a task
  claudio adversarial "Implement user authentication with JWT"

  # Limit the number of review cycles
  claudio adversarial --max-iterations 5 "Refactor the database layer"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdversarial,
}

var (
	adversarialMaxIterations int
)

func init() {
	adversarialCmd.Flags().IntVar(&adversarialMaxIterations, "max-iterations", 10, "Maximum number of implement-review cycles (0 = unlimited)")
}

// RegisterAdversarialCmd registers the adversarial command with the given parent command.
func RegisterAdversarialCmd(parent *cobra.Command) {
	parent.AddCommand(adversarialCmd)
}

func runAdversarial(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get task from args or prompt
	var task string
	if len(args) > 0 {
		task = args[0]
	} else {
		task, err = promptAdversarialTask()
		if err != nil {
			return err
		}
	}

	// Generate a new session ID for this adversarial session
	sessionID := orchsession.GenerateID()
	cfg := config.Get()

	// Create adversarial configuration
	advConfig := orchestrator.DefaultAdversarialConfig()
	advConfig.MaxIterations = adversarialMaxIterations

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

	// Start a new session for the adversarial review
	words := strings.Fields(task)
	if len(words) > 3 {
		words = words[:3]
	}
	sessionName := "adversarial-" + SlugifyWords(words)

	session, err := orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Log startup
	logger.Info("adversarial session started",
		"session_id", sessionID,
		"task", util.TruncateString(task, 100),
		"max_iterations", advConfig.MaxIterations,
	)

	// Create adversarial session
	advSession := orchestrator.NewAdversarialSession(task, advConfig)

	// Link adversarial session to main session for persistence
	session.AdversarialSessions = append(session.AdversarialSessions, advSession)

	// Create coordinator with logger
	coordinator := orchestrator.NewAdversarialCoordinator(orch, session, advSession, logger)

	// Get terminal dimensions
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI with adversarial mode
	app := tui.NewWithAdversarial(orch, session, coordinator, logger.WithSession(session.ID))
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// promptAdversarialTask prompts the user to enter a task
func promptAdversarialTask() (string, error) {
	fmt.Println("\nAdversarial Review Mode")
	fmt.Println("=======================")
	fmt.Println("Enter a task for the implementer to complete.")
	fmt.Println("A critical reviewer will thoroughly examine each increment.")
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

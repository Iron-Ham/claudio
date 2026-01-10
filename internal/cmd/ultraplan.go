package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	sessutil "github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var ultraplanCmd = &cobra.Command{
	Use:   "ultraplan [objective]",
	Short: "Start an ultra-plan session with intelligent task orchestration",
	Long: `Ultra-plan mode enables intelligent orchestration of parallel Claude sessions
through a hierarchical planning approach. A top-level "coordinator" session performs
deep planning to decompose complex tasks into parallelizable chunks, spawns child
sessions for execution, and manages the overall workflow.

The ultra-plan process has four phases:
1. PLANNING: Claude explores the codebase and creates an execution plan
2. REVIEW: (optional) Interactive plan editor to review and modify the plan
3. EXECUTION: Child sessions execute tasks in parallel (respecting dependencies)
4. SYNTHESIS: Results are reviewed and integrated

Multi-Pass Planning:
  Use --multi-pass to enable multi-pass planning mode, where three independent
  coordinators each create their own execution plan using different strategies:

    • maximize-parallelism: Optimizes for maximum concurrent task execution
    • minimize-complexity: Prioritizes simplicity and clear task boundaries
    • balanced-approach: Balances parallelism, complexity, and dependencies

  A coordinator-manager then evaluates all three plans, scoring each on criteria
  like task clarity, dependency structure, and execution efficiency. It either
  selects the best plan or merges the strongest elements from multiple plans
  into a canonical execution plan. This produces higher-quality plans through
  diverse strategic perspectives.

Plan Editor:
  When the plan is ready, an interactive editor opens allowing you to:
  - Review task dependencies and execution order
  - Add, edit, or remove tasks
  - Modify task priorities and complexity estimates
  - Validate the plan for dependency cycles before execution

  Use --review to always open the plan editor, even with --auto-approve.
  Use --auto-approve without --review to skip the editor entirely.

Examples:
  # Start ultra-plan with an objective
  claudio ultraplan "Implement user authentication with OAuth2 support"

  # Start with a pre-existing plan file
  claudio ultraplan --plan plan.json

  # Review and edit a plan before execution
  claudio ultraplan --plan plan.json --review

  # Dry run - only generate the plan, don't execute
  claudio ultraplan --dry-run "Refactor the API layer"

  # Auto-approve but still review the plan first
  claudio ultraplan --auto-approve --review "Add comprehensive test coverage"

  # Increase parallelism
  claudio ultraplan --max-parallel 5 "Add comprehensive test coverage"

  # Use multi-pass planning for complex tasks requiring careful decomposition
  claudio ultraplan --multi-pass "Refactor database layer to use repository pattern"

  # Combine multi-pass with dry-run to compare strategies without executing
  claudio ultraplan --multi-pass --dry-run "Implement microservices architecture"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUltraplan,
}

var (
	ultraplanPlanFile    string
	ultraplanMaxParallel int
	ultraplanDryRun      bool
	ultraplanNoSynthesis bool
	ultraplanAutoApprove bool
	ultraplanReview      bool
	ultraplanMultiPass   bool
)

func init() {
	rootCmd.AddCommand(ultraplanCmd)

	ultraplanCmd.Flags().StringVar(&ultraplanPlanFile, "plan", "", "Use existing plan file instead of planning phase")
	ultraplanCmd.Flags().IntVar(&ultraplanMaxParallel, "max-parallel", 3, "Maximum concurrent child sessions (0 = unlimited)")
	ultraplanCmd.Flags().BoolVar(&ultraplanDryRun, "dry-run", false, "Run planning only, output plan without executing")
	ultraplanCmd.Flags().BoolVar(&ultraplanNoSynthesis, "no-synthesis", false, "Skip synthesis phase after execution")
	ultraplanCmd.Flags().BoolVar(&ultraplanAutoApprove, "auto-approve", false, "Auto-approve spawned tasks without confirmation")
	ultraplanCmd.Flags().BoolVar(&ultraplanReview, "review", false, "Review and edit plan before execution (opens plan editor)")
	ultraplanCmd.Flags().BoolVar(&ultraplanMultiPass, "multi-pass", false, "Enable multi-pass planning with 3 strategic approaches (maximize-parallelism, minimize-complexity, balanced) - best plan is selected or merged")
}

func runUltraplan(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Get objective from args or prompt
	var objective string
	if len(args) > 0 {
		objective = args[0]
	} else if ultraplanPlanFile == "" {
		// Prompt for objective if not provided and no plan file
		objective, err = promptObjective()
		if err != nil {
			return err
		}
	}

	// Generate a new session ID for this ultraplan
	sessionID := orchestrator.GenerateID()
	cfg := config.Get()

	// Create orchestrator with multi-session support
	orch, err := orchestrator.NewWithSession(cwd, sessionID, cfg)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Start a new session for the ultra-plan
	sessionName := "ultraplan"
	if objective != "" {
		// Use first few words of objective as session name
		words := strings.Fields(objective)
		if len(words) > 3 {
			words = words[:3]
		}
		sessionName = "ultraplan-" + slugifyWords(words)
	}

	session, err := orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Create ultra-plan configuration from defaults, then override with config file, then flags
	ultraConfig := orchestrator.DefaultUltraPlanConfig()

	// Apply config file settings (viper default is 3, user can set to 0 for unlimited)
	ultraConfig.MaxParallel = cfg.Ultraplan.MaxParallel

	// Apply config file settings
	ultraConfig.MultiPass = cfg.Ultraplan.MultiPass

	// CLI flags override config file (only if explicitly set)
	if cmd.Flags().Changed("max-parallel") {
		ultraConfig.MaxParallel = ultraplanMaxParallel
	}
	if cmd.Flags().Changed("multi-pass") {
		ultraConfig.MultiPass = ultraplanMultiPass
	}
	ultraConfig.DryRun = ultraplanDryRun
	ultraConfig.NoSynthesis = ultraplanNoSynthesis
	ultraConfig.AutoApprove = ultraplanAutoApprove
	ultraConfig.Review = ultraplanReview

	// Create or load the plan
	var plan *orchestrator.PlanSpec
	if ultraplanPlanFile != "" {
		plan, err = loadPlanFile(ultraplanPlanFile)
		if err != nil {
			return fmt.Errorf("failed to load plan file: %w", err)
		}
		objective = plan.Objective
	}

	// Create ultra-plan session
	ultraSession := orchestrator.NewUltraPlanSession(objective, ultraConfig)
	if plan != nil {
		ultraSession.Plan = plan
		ultraSession.Phase = orchestrator.PhaseRefresh // Skip to refresh if plan provided
	}

	// Link ultra-plan session to main session for persistence
	session.UltraPlan = ultraSession

	// Create coordinator
	// Note: logger is nil here - integration with config.Get().Logging settings will be done
	// in a future task that wires up logger initialization
	coordinator := orchestrator.NewCoordinator(orch, session, ultraSession, nil)

	// Get terminal dimensions
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Create logger for the TUI session
	sessionDir := sessutil.GetSessionDir(cwd, session.ID)
	logger, err := logging.NewLogger(sessionDir, cfg.Logging.Level)
	if err != nil {
		// Log creation failure shouldn't prevent TUI from starting
		fmt.Fprintf(os.Stderr, "Warning: failed to create logger: %v\n", err)
		logger = logging.NopLogger()
	}
	defer logger.Close()

	// Launch TUI with ultra-plan mode
	app := tui.NewWithUltraPlan(orch, session, coordinator, logger.WithSession(session.ID))
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// promptObjective prompts the user to enter an objective
func promptObjective() (string, error) {
	fmt.Println("\nUltra-Plan Mode")
	fmt.Println("===============")
	fmt.Println("Enter a high-level objective for Claude to plan and execute.")
	fmt.Println("Claude will analyze the codebase and create an execution plan.")
	fmt.Println()
	fmt.Print("Objective: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return "", fmt.Errorf("objective cannot be empty")
	}

	return input, nil
}

// loadPlanFile loads a plan from a JSON file
func loadPlanFile(path string) (*orchestrator.PlanSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var plan orchestrator.PlanSpec
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	if err := orchestrator.ValidatePlan(&plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

// slugifyWords creates a slug from words
func slugifyWords(words []string) string {
	joined := strings.ToLower(strings.Join(words, "-"))
	var result strings.Builder
	for _, r := range joined {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug := result.String()
	if len(slug) > 20 {
		slug = slug[:20]
	}
	return strings.TrimSuffix(slug, "-")
}

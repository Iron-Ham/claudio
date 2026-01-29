package planning

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	orchsession "github.com/Iron-Ham/claudio/internal/orchestrator/session"
	sessutil "github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/tui"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
	"github.com/Iron-Ham/claudio/internal/util"
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

Adversarial Mode:
  Use --adversarial to enable adversarial review mode. In this mode, each task
  is paired with a critical reviewer. A task's work is not considered complete
  until its reviewer approves it. This ensures higher quality by catching issues
  during execution rather than during synthesis.

Plan Editor:
  When the plan is ready, an interactive editor opens allowing you to:
  - Review task dependencies and execution order
  - Add, edit, or remove tasks
  - Modify task priorities and complexity estimates
  - Validate the plan for dependency cycles before execution

  Use --review to always open the plan editor, even with --auto-approve.
  Use --auto-approve without --review to skip the editor entirely.

Configuration options can be set in config.yaml under 'ultraplan:' or via flags:
- max_parallel: Maximum concurrent child sessions (default: 3)
- multi_pass: Enable multi-pass planning (default: false)
- adversarial: [EXPERIMENTAL] Enable adversarial review per task (default: false, infrastructure-only)

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
  claudio ultraplan --multi-pass --dry-run "Implement microservices architecture"

  # Enable adversarial review for higher quality task completion
  claudio ultraplan --adversarial "Implement critical security features"`,
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
	ultraplanAdversarial bool
)

func init() {
	cfg := config.Get()
	ultraplanCmd.Flags().StringVar(&ultraplanPlanFile, "plan", "", "Use existing plan file instead of planning phase")
	ultraplanCmd.Flags().IntVar(&ultraplanMaxParallel, "max-parallel", cfg.Ultraplan.MaxParallel, "Maximum concurrent child sessions (0 = unlimited)")
	ultraplanCmd.Flags().BoolVar(&ultraplanDryRun, "dry-run", false, "Run planning only, output plan without executing")
	ultraplanCmd.Flags().BoolVar(&ultraplanNoSynthesis, "no-synthesis", false, "Skip synthesis phase after execution")
	ultraplanCmd.Flags().BoolVar(&ultraplanAutoApprove, "auto-approve", false, "Auto-approve spawned tasks without confirmation")
	ultraplanCmd.Flags().BoolVar(&ultraplanReview, "review", false, "Review and edit plan before execution (opens plan editor)")
	ultraplanCmd.Flags().BoolVar(&ultraplanMultiPass, "multi-pass", cfg.Ultraplan.MultiPass, "Enable multi-pass planning with 3 strategic approaches (maximize-parallelism, minimize-complexity, balanced) - best plan is selected or merged")
	ultraplanCmd.Flags().BoolVar(&ultraplanAdversarial, "adversarial", cfg.Ultraplan.Adversarial, "[EXPERIMENTAL] Enable adversarial review mode where each task must pass reviewer approval (NOTE: infrastructure-only, workflow integration not yet implemented)")
}

// RegisterUltraplanCmd registers the ultraplan command with the given parent command.
func RegisterUltraplanCmd(parent *cobra.Command) {
	parent.AddCommand(ultraplanCmd)
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
		objective, err = promptUltraplanObjective()
		if err != nil {
			return err
		}
	}

	// Generate a new session ID for this ultraplan
	sessionID := orchsession.GenerateID()
	cfg := config.Get()

	// Build ultraplan config from app config, then apply CLI flag overrides
	ultraConfig := ultraplan.BuildConfigFromAppConfig(cfg)
	applyUltraplanFlagOverrides(cmd, &ultraConfig)

	// Create logger if enabled - we need session dir which requires session ID
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

	// Start a new session for the ultra-plan
	sessionName := "ultraplan"
	if objective != "" {
		// Use first few words of objective as session name
		words := strings.Fields(objective)
		if len(words) > 3 {
			words = words[:3]
		}
		sessionName = "ultraplan-" + SlugifyWords(words)
	}

	session, err := orch.StartSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}

	// Load plan file if provided
	var plan *orchestrator.PlanSpec
	if ultraplanPlanFile != "" {
		plan, err = loadUltraplanFile(ultraplanPlanFile)
		if err != nil {
			return fmt.Errorf("failed to load plan file: %w", err)
		}
		objective = plan.Objective
	}

	// Initialize ultraplan using shared initialization
	initParams := ultraplan.InitParams{
		Orchestrator: orch,
		Session:      session,
		Objective:    objective,
		Config:       &ultraConfig,
		Plan:         plan,
		Logger:       logger,
		CreateGroup:  false, // CLI TUI handles group creation separately
	}

	var initResult *ultraplan.InitResult
	if plan != nil {
		// Use InitWithPlan for additional validation (plan already validated in loadUltraplanFile,
		// but InitWithPlan also calls SetPlan to ensure coordinator state is synchronized)
		initResult, err = ultraplan.InitWithPlan(initParams, plan)
		if err != nil {
			return fmt.Errorf("failed to initialize ultraplan with plan: %w", err)
		}
	} else {
		initResult = ultraplan.Init(initParams)
	}

	// Log startup with objective (truncated) and config summary
	logger.Info("ultraplan started",
		"session_id", sessionID,
		"objective", util.TruncateString(objective, 100),
		"max_parallel", initResult.Config.MaxParallel,
		"multi_pass", initResult.Config.MultiPass,
		"adversarial", initResult.Config.Adversarial,
		"dry_run", initResult.Config.DryRun,
		"auto_approve", initResult.Config.AutoApprove,
	)

	// Get terminal dimensions
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI with ultra-plan mode
	app := tui.NewWithUltraPlan(orch, session, initResult.Coordinator, logger.WithSession(session.ID))
	if err := app.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}

// applyUltraplanFlagOverrides applies CLI flag values to the ultraplan config.
// Flags only override config file values when explicitly set by the user.
func applyUltraplanFlagOverrides(cmd *cobra.Command, cfg *orchestrator.UltraPlanConfig) {
	if cmd.Flags().Changed("max-parallel") {
		cfg.MaxParallel = ultraplanMaxParallel
	}
	if cmd.Flags().Changed("multi-pass") {
		cfg.MultiPass = ultraplanMultiPass
	}
	if cmd.Flags().Changed("adversarial") {
		cfg.Adversarial = ultraplanAdversarial
	}
	// These flags always apply (no "changed" check needed since they have sensible defaults)
	cfg.DryRun = ultraplanDryRun
	cfg.NoSynthesis = ultraplanNoSynthesis
	cfg.AutoApprove = ultraplanAutoApprove
	cfg.Review = ultraplanReview
}

// promptUltraplanObjective prompts the user to enter an objective
func promptUltraplanObjective() (string, error) {
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

// loadUltraplanFile loads a plan from a JSON file
func loadUltraplanFile(path string) (*orchestrator.PlanSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var plan orchestrator.PlanSpec
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Compute DependencyGraph and ExecutionOrder if they weren't in the JSON
	// (e.g., plan files that only have tasks with depends_on fields)
	orchestrator.EnsurePlanComputed(&plan)

	if err := orchestrator.ValidatePlan(&plan); err != nil {
		return nil, err
	}

	return &plan, nil
}

// SlugifyWords creates a slug from words.
// Exported for use by adversarial.
func SlugifyWords(words []string) string {
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

// createLogger creates a logger if logging is enabled in config.
// Returns a NopLogger if logging is disabled or if creation fails.
func createLogger(sessionDir string, cfg *config.Config) *logging.Logger {
	// Check if logging is enabled
	if !cfg.Logging.Enabled {
		return logging.NopLogger()
	}

	// Build rotation config from logging config
	rotationConfig := logging.RotationConfig{
		MaxSizeMB:  cfg.Logging.MaxSizeMB,
		MaxBackups: cfg.Logging.MaxBackups,
		Compress:   false, // Not exposed in config yet
	}

	// Create the logger with rotation support
	logger, err := logging.NewLoggerWithRotation(sessionDir, cfg.Logging.Level, rotationConfig)
	if err != nil {
		// Log creation failure shouldn't prevent the application from starting
		fmt.Fprintf(os.Stderr, "Warning: failed to create logger: %v\n", err)
		return logging.NopLogger()
	}

	return logger
}

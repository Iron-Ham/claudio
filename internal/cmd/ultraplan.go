package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
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
2. CONTEXT REFRESH: The plan is compacted for efficient execution
3. EXECUTION: Child sessions execute tasks in parallel (respecting dependencies)
4. SYNTHESIS: Results are reviewed and integrated

Examples:
  # Start ultra-plan with an objective
  claudio ultraplan "Implement user authentication with OAuth2 support"

  # Start with a pre-existing plan file
  claudio ultraplan --plan plan.json

  # Dry run - only generate the plan, don't execute
  claudio ultraplan --dry-run "Refactor the API layer"

  # Increase parallelism
  claudio ultraplan --max-parallel 5 "Add comprehensive test coverage"`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUltraplan,
}

var (
	ultraplanPlanFile    string
	ultraplanMaxParallel int
	ultraplanDryRun      bool
	ultraplanNoSynthesis bool
	ultraplanAutoApprove bool
)

func init() {
	rootCmd.AddCommand(ultraplanCmd)

	ultraplanCmd.Flags().StringVar(&ultraplanPlanFile, "plan", "", "Use existing plan file instead of planning phase")
	ultraplanCmd.Flags().IntVar(&ultraplanMaxParallel, "max-parallel", 3, "Maximum concurrent child sessions")
	ultraplanCmd.Flags().BoolVar(&ultraplanDryRun, "dry-run", false, "Run planning only, output plan without executing")
	ultraplanCmd.Flags().BoolVar(&ultraplanNoSynthesis, "no-synthesis", false, "Skip synthesis phase after execution")
	ultraplanCmd.Flags().BoolVar(&ultraplanAutoApprove, "auto-approve", false, "Auto-approve spawned tasks without confirmation")
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

	// Create orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Check for existing session
	if orch.HasExistingSession() {
		action, err := promptUltraplanSessionAction()
		if err != nil {
			return err
		}
		if action == "quit" {
			return nil
		}
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

	// Create ultra-plan configuration
	config := orchestrator.UltraPlanConfig{
		MaxParallel: ultraplanMaxParallel,
		DryRun:      ultraplanDryRun,
		NoSynthesis: ultraplanNoSynthesis,
		AutoApprove: ultraplanAutoApprove,
	}

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
	ultraSession := orchestrator.NewUltraPlanSession(objective, config)
	if plan != nil {
		ultraSession.Plan = plan
		ultraSession.Phase = orchestrator.PhaseRefresh // Skip to refresh if plan provided
	}

	// Link ultra-plan session to main session for persistence
	session.UltraPlan = ultraSession

	// Create coordinator
	coordinator := orchestrator.NewCoordinator(orch, session, ultraSession)

	// Get terminal dimensions
	if termWidth, termHeight, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		contentWidth, contentHeight := tui.CalculateContentDimensions(termWidth, termHeight)
		if contentWidth > 0 && contentHeight > 0 {
			orch.SetDisplayDimensions(contentWidth, contentHeight)
		}
	}

	// Launch TUI with ultra-plan mode
	app := tui.NewWithUltraPlan(orch, session, coordinator)
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

// promptUltraplanSessionAction prompts what to do with existing session
func promptUltraplanSessionAction() (string, error) {
	fmt.Println("\nAn existing session was found.")
	fmt.Println("Ultra-plan requires a fresh session.")
	fmt.Println("  [c] Continue - Replace existing session with ultra-plan")
	fmt.Println("  [q] Quit     - Exit without changes")
	fmt.Print("\nChoice [c/q]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "c", "continue", "":
		return "continue", nil
	case "q", "quit":
		return "quit", nil
	default:
		return "continue", nil
	}
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

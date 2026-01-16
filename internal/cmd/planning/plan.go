// Package planning provides CLI commands for planning and orchestration modes.
// This includes the plan, ultraplan, and tripleshot commands.
package planning

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/plan"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan [objective]",
	Short: "Create structured plans from an objective",
	Long: `Plan mode analyzes your objective and creates a structured execution plan.

Output formats:
  --output-format=json    Create .claudio-plan.json for use with 'claudio ultraplan --plan'
  --output-format=issues  Create GitHub Issues with parent epic and linked sub-issues (default)
  --output-format=both    Create both JSON file and GitHub Issues

Examples:
  # Dry-run: show plan without creating output
  claudio plan --dry-run "Add user authentication"

  # Create GitHub Issues (default)
  claudio plan "Add user authentication"

  # Create JSON file for ultraplan
  claudio plan --output-format=json "Add user authentication"

  # Use multi-pass planning for complex objectives
  claudio plan --multi-pass "Refactor database layer"

  # Add labels to GitHub Issues
  claudio plan --labels "enhancement,v2" "Add caching layer"

  # Run the generated plan with ultraplan
  claudio plan --output-format=json "Build new module"
  claudio ultraplan --plan .claudio-plan.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPlan,
}

var (
	planDryRun       bool
	planMultiPass    bool
	planLabels       []string
	planOutputFile   string
	planNoConfirm    bool
	planOutputFormat string
)

func init() {
	planCmd.Flags().BoolVar(&planDryRun, "dry-run", false, "Show plan without creating output")
	planCmd.Flags().BoolVar(&planMultiPass, "multi-pass", false, "Use 3-strategy planning for complex tasks")
	planCmd.Flags().StringSliceVar(&planLabels, "labels", nil, "Labels to add to GitHub Issues")
	planCmd.Flags().StringVar(&planOutputFile, "output", "", "Write plan JSON to specific file path")
	planCmd.Flags().BoolVar(&planNoConfirm, "no-confirm", false, "Skip confirmation prompt")
	planCmd.Flags().StringVar(&planOutputFormat, "output-format", "issues",
		"Output format: 'json' (for ultraplan), 'issues' (GitHub Issues), or 'both'")
}

// RegisterPlanCmd registers the plan command with the given parent command.
func RegisterPlanCmd(parent *cobra.Command) {
	parent.AddCommand(planCmd)
}

func runPlan(cmd *cobra.Command, args []string) error {
	cfg := config.Get()

	// Apply config file settings, CLI flags override
	outputFormat := cfg.Plan.OutputFormat
	if cmd.Flags().Changed("output-format") {
		outputFormat = planOutputFormat
	}

	multiPass := cfg.Plan.MultiPass
	if cmd.Flags().Changed("multi-pass") {
		multiPass = planMultiPass
	}

	labels := cfg.Plan.Labels
	if cmd.Flags().Changed("labels") {
		labels = planLabels
	}

	outputFile := cfg.Plan.OutputFile
	if cmd.Flags().Changed("output") {
		outputFile = planOutputFile
	}

	// Update module-level vars so helper functions work correctly
	planOutputFormat = outputFormat
	planMultiPass = multiPass
	planLabels = labels
	planOutputFile = outputFile

	// Get objective from args or prompt
	var objective string
	if len(args) > 0 {
		objective = args[0]
	} else {
		var err error
		objective, err = promptForObjective()
		if err != nil {
			return err
		}
	}

	// Validate output format
	switch planOutputFormat {
	case "json", "issues", "both":
		// Valid
	default:
		return fmt.Errorf("invalid output format: %s (use 'json', 'issues', or 'both')", planOutputFormat)
	}

	fmt.Println("Planning...")
	if planMultiPass {
		fmt.Println("Using multi-pass planning (3 strategies)...")
	}

	// Run planning phase
	planSpec, err := plan.RunPlanningSync(objective, planMultiPass)
	if err != nil {
		return fmt.Errorf("planning failed: %w", err)
	}

	// Display plan for review
	fmt.Println()
	fmt.Println(plan.FormatPlanForDisplay(planSpec))

	// Handle dry-run: still save JSON if requested
	if planDryRun {
		if shouldSaveJSON() {
			outputPath := getOutputPath()
			if err := plan.SavePlanToFile(planSpec, outputPath); err != nil {
				return fmt.Errorf("failed to save plan: %w", err)
			}
			fmt.Printf("Plan saved to %s\n", outputPath)
			fmt.Printf("Run with: claudio ultraplan --plan %s\n", outputPath)
		}
		return nil
	}

	// Confirm before creating output
	if !planNoConfirm {
		confirmed, err := confirmCreation()
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Create output based on format
	switch planOutputFormat {
	case "json":
		return createJSONOutput(planSpec)
	case "issues":
		return createIssuesOutput(planSpec)
	case "both":
		if err := createJSONOutput(planSpec); err != nil {
			return err
		}
		return createIssuesOutput(planSpec)
	}

	return nil
}

func promptForObjective() (string, error) {
	fmt.Println()
	fmt.Println("Plan Mode")
	fmt.Println("=========")
	fmt.Println("Enter a high-level objective for Claude to plan.")
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

func shouldSaveJSON() bool {
	return planOutputFormat == "json" || planOutputFormat == "both" || planOutputFile != ""
}

func getOutputPath() string {
	if planOutputFile != "" {
		return planOutputFile
	}
	return ".claudio-plan.json"
}

func confirmCreation() (bool, error) {
	var action string
	switch planOutputFormat {
	case "json":
		action = "save plan to " + getOutputPath()
	case "issues":
		action = "create GitHub Issues"
	case "both":
		action = "save plan to " + getOutputPath() + " and create GitHub Issues"
	}

	fmt.Printf("Proceed to %s? [y/N] ", action)

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes", nil
}

func createJSONOutput(planSpec *orchestrator.PlanSpec) error {
	outputPath := getOutputPath()
	if err := plan.SavePlanToFile(planSpec, outputPath); err != nil {
		return fmt.Errorf("failed to save plan: %w", err)
	}
	fmt.Printf("Plan saved to %s\n", outputPath)
	fmt.Printf("Run with: claudio ultraplan --plan %s\n", outputPath)
	return nil
}

func createIssuesOutput(planSpec *orchestrator.PlanSpec) error {
	result, err := plan.CreateIssuesFromPlan(planSpec, planLabels)
	if err != nil {
		return fmt.Errorf("failed to create issues: %w", err)
	}

	fmt.Println()
	fmt.Printf("Created parent issue: %s\n", result.ParentIssueURL)
	fmt.Printf("Created %d sub-issues\n", len(result.SubIssueNumbers))

	return nil
}

// LoadPlanFromFile loads a plan from a JSON file (for use by other commands)
func LoadPlanFromFile(path string) (*orchestrator.PlanSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var planSpec orchestrator.PlanSpec
	if err := json.Unmarshal(data, &planSpec); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return &planSpec, nil
}

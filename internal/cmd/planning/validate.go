// Package planning provides CLI commands for planning and orchestration modes.
package planning

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [plan-file]",
	Short: "Validate an ultraplan JSON file",
	Long: `Validate an ultraplan JSON file for structural issues and correctness.

This command checks:
  - Valid JSON syntax
  - Required fields (summary, tasks, etc.)
  - Task dependency validity (no cycles, no missing references)
  - File conflict detection between parallel tasks
  - High complexity task warnings

The exit code indicates the result:
  0 - Plan is valid (may have warnings)
  1 - Plan has validation errors or could not be parsed

Examples:
  # Validate the default plan file
  claudio validate

  # Validate a specific plan file
  claudio validate .claudio-plan.json

  # Validate with JSON output
  claudio validate --json my-plan.json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

var (
	validateJSON bool
)

func init() {
	validateCmd.Flags().BoolVar(&validateJSON, "json", false, "Output validation result as JSON")
}

// RegisterValidateCmd registers the validate command with the given parent command.
func RegisterValidateCmd(parent *cobra.Command) {
	parent.AddCommand(validateCmd)
}

// ValidationOutput represents the JSON output format for validation results.
type ValidationOutput struct {
	Valid        bool                          `json:"valid"`
	FilePath     string                        `json:"file_path"`
	ErrorCount   int                           `json:"error_count"`
	WarningCount int                           `json:"warning_count"`
	InfoCount    int                           `json:"info_count"`
	Messages     []ultraplan.ValidationMessage `json:"messages,omitempty"`
	ParseError   string                        `json:"parse_error,omitempty"`
}

// runValidate is the command handler for the validate subcommand.
// It validates an ultraplan JSON file and outputs results in either
// human-readable or JSON format depending on the --json flag.
func runValidate(cmd *cobra.Command, args []string) error {
	filePath := orchestrator.PlanFileName
	if len(args) > 0 {
		filePath = args[0]
	}

	// Check if file exists and is accessible
	if _, err := os.Stat(filePath); err != nil {
		var errMsg string
		if os.IsNotExist(err) {
			errMsg = fmt.Sprintf("file not found: %s", filePath)
		} else if os.IsPermission(err) {
			errMsg = fmt.Sprintf("permission denied: %s", filePath)
		} else {
			errMsg = fmt.Sprintf("cannot access file: %s: %v", filePath, err)
		}
		if validateJSON {
			return outputJSON(ValidationOutput{
				Valid:      false,
				FilePath:   filePath,
				ParseError: errMsg,
			})
		}
		return fmt.Errorf("%s", errMsg)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if validateJSON {
			return outputJSON(ValidationOutput{
				Valid:      false,
				FilePath:   filePath,
				ParseError: fmt.Sprintf("failed to read file: %v", err),
			})
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Validate JSON syntax before attempting semantic parsing
	var jsonCheck any
	if err := json.Unmarshal(data, &jsonCheck); err != nil {
		if validateJSON {
			return outputJSON(ValidationOutput{
				Valid:      false,
				FilePath:   filePath,
				ParseError: fmt.Sprintf("invalid JSON: %v", err),
			})
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Parse plan using orchestrator's parser (supports alternative field names
	// like "depends" for "depends_on" and nested "plan" wrapper format)
	plan, err := orchestrator.ParsePlanFromFile(filePath, "")
	if err != nil {
		if validateJSON {
			return outputJSON(ValidationOutput{
				Valid:      false,
				FilePath:   filePath,
				ParseError: fmt.Sprintf("failed to parse plan: %v", err),
			})
		}
		return fmt.Errorf("failed to parse plan: %w", err)
	}

	// Populate computed fields (DependencyGraph, ExecutionOrder) if missing
	orchestrator.EnsurePlanComputed(plan)

	// Convert to ultraplan.PlanSpec for detailed validation
	ultraplanSpec := convertToUltraplanSpec(plan)

	result, err := ultraplan.ValidatePlan(ultraplanSpec)
	if err != nil {
		if validateJSON {
			return outputJSON(ValidationOutput{
				Valid:      false,
				FilePath:   filePath,
				ParseError: fmt.Sprintf("validation error: %v", err),
			})
		}
		return fmt.Errorf("validation error: %w", err)
	}

	if validateJSON {
		return outputJSON(ValidationOutput{
			Valid:        result.IsValid,
			FilePath:     filePath,
			ErrorCount:   result.ErrorCount,
			WarningCount: result.WarningCount,
			InfoCount:    result.InfoCount,
			Messages:     result.Messages,
		})
	}

	return outputHuman(filePath, plan, result)
}

// outputJSON marshals and prints the validation output as formatted JSON.
// Returns a silentError if validation failed to signal exit code 1.
// Always outputs valid JSON, even if marshaling fails (uses fallback format).
func outputJSON(output ValidationOutput) error {
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		// Fallback: output a minimal valid JSON error response
		// This ensures --json mode always produces valid JSON for CI/CD pipelines
		fallback := fmt.Sprintf(`{"valid": false, "file_path": %q, "parse_error": "internal error: failed to marshal output: %s"}`,
			output.FilePath, err.Error())
		fmt.Println(fallback)
		return &silentError{}
	}
	fmt.Println(string(data))

	if !output.Valid {
		return &silentError{}
	}
	return nil
}

// silentError signals that validation failed but output was already provided.
// Used to set exit code 1 without Cobra printing a duplicate error message.
type silentError struct{}

func (e *silentError) Error() string {
	return "validation failed"
}

// outputHuman prints validation results in a human-readable format,
// including plan summary, validation status, and categorized messages.
func outputHuman(filePath string, plan *orchestrator.PlanSpec, result *ultraplan.ValidationResult) error {
	fmt.Printf("Validating: %s\n", filePath)
	fmt.Println()

	fmt.Printf("Plan Summary:\n")
	fmt.Printf("  Tasks: %d\n", len(plan.Tasks))
	fmt.Printf("  Execution Groups: %d\n", len(plan.ExecutionOrder))
	if plan.Summary != "" {
		summary := plan.Summary
		if len(summary) > 100 {
			summary = summary[:97] + "..."
		}
		fmt.Printf("  Summary: %s\n", summary)
	}
	fmt.Println()

	if result.IsValid {
		fmt.Println("Status: VALID")
	} else {
		fmt.Println("Status: INVALID")
	}

	if result.ErrorCount > 0 || result.WarningCount > 0 || result.InfoCount > 0 {
		fmt.Printf("  Errors: %d, Warnings: %d, Info: %d\n",
			result.ErrorCount, result.WarningCount, result.InfoCount)
	}
	fmt.Println()

	if len(result.Messages) > 0 {
		errors := result.GetMessagesBySeverity(ultraplan.SeverityError)
		if len(errors) > 0 {
			fmt.Println("Errors:")
			for _, msg := range errors {
				printMessage(msg)
			}
			fmt.Println()
		}

		warnings := result.GetMessagesBySeverity(ultraplan.SeverityWarning)
		if len(warnings) > 0 {
			fmt.Println("Warnings:")
			for _, msg := range warnings {
				printMessage(msg)
			}
			fmt.Println()
		}

		infos := result.GetMessagesBySeverity(ultraplan.SeverityInfo)
		if len(infos) > 0 {
			fmt.Println("Info:")
			for _, msg := range infos {
				printMessage(msg)
			}
			fmt.Println()
		}
	}

	if !result.IsValid {
		return fmt.Errorf("plan validation failed with %d error(s)", result.ErrorCount)
	}

	return nil
}

// printMessage formats and prints a single validation message with optional
// task ID prefix and suggestion.
func printMessage(msg ultraplan.ValidationMessage) {
	prefix := "  - "
	if msg.TaskID != "" {
		prefix = fmt.Sprintf("  - [%s] ", msg.TaskID)
	}

	fmt.Printf("%s%s\n", prefix, msg.Message)

	if msg.Suggestion != "" {
		fmt.Printf("    Suggestion: %s\n", msg.Suggestion)
	}
}

// convertToUltraplanSpec converts an orchestrator.PlanSpec to an ultraplan.PlanSpec
// for use with the ultraplan validation functions.
func convertToUltraplanSpec(plan *orchestrator.PlanSpec) *ultraplan.PlanSpec {
	if plan == nil {
		return nil
	}

	tasks := make([]ultraplan.PlannedTask, len(plan.Tasks))
	for i, t := range plan.Tasks {
		tasks[i] = ultraplan.PlannedTask{
			ID:            t.ID,
			Title:         t.Title,
			Description:   t.Description,
			Files:         t.Files,
			DependsOn:     t.DependsOn,
			Priority:      t.Priority,
			EstComplexity: ultraplan.TaskComplexity(t.EstComplexity),
			IssueURL:      t.IssueURL,
			NoCode:        t.NoCode,
		}
	}

	return &ultraplan.PlanSpec{
		ID:              plan.ID,
		Objective:       plan.Objective,
		Summary:         plan.Summary,
		Tasks:           tasks,
		DependencyGraph: plan.DependencyGraph,
		ExecutionOrder:  plan.ExecutionOrder,
		Insights:        plan.Insights,
		Constraints:     plan.Constraints,
		CreatedAt:       plan.CreatedAt,
	}
}

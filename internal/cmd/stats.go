package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/Iron-Ham/claudio/internal/config"
	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show session resource usage and cost statistics",
	Long: `Display resource usage statistics and cost estimates for the current Claudio session.

Shows:
- Total token usage (input/output)
- Estimated API costs
- Per-instance breakdown
- Budget limit status`,
	RunE: runStats,
}

var (
	statsJSON bool // Output as JSON
)

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output statistics as JSON")
	rootCmd.AddCommand(statsCmd)
}

func runStats(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create orchestrator
	orch, err := orchestrator.New(cwd)
	if err != nil {
		return fmt.Errorf("failed to create orchestrator: %w", err)
	}

	// Load current session
	session, err := orch.LoadSession()
	if err != nil {
		fmt.Println("No active session")
		return nil
	}

	// Get session metrics
	metrics := orch.GetSessionMetrics()
	cfg := config.Get()

	if statsJSON {
		return printStatsJSON(session, metrics, cfg)
	}

	return printStatsText(session, metrics, cfg)
}

func printStatsText(session *orchestrator.Session, metrics *orchestrator.SessionMetrics, cfg *config.Config) error {
	// Header
	fmt.Println()
	fmt.Println("SESSION SUMMARY")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Session: %s\n", session.Name)
	fmt.Printf("Started: %s\n", session.Created.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total Instances: %d (%d active)\n", metrics.InstanceCount, metrics.ActiveCount)
	fmt.Println()

	// Token usage
	fmt.Println("TOKEN USAGE")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Input:  %s tokens\n", instmetrics.FormatTokens(metrics.TotalInputTokens))
	fmt.Printf("Output: %s tokens\n", instmetrics.FormatTokens(metrics.TotalOutputTokens))
	totalTokens := metrics.TotalInputTokens + metrics.TotalOutputTokens
	fmt.Printf("Total:  %s tokens\n", instmetrics.FormatTokens(totalTokens))
	if metrics.TotalCacheRead > 0 || metrics.TotalCacheWrite > 0 {
		fmt.Printf("Cache:  %s read / %s write\n",
			instmetrics.FormatTokens(metrics.TotalCacheRead),
			instmetrics.FormatTokens(metrics.TotalCacheWrite))
	}
	fmt.Println()

	// Cost summary
	fmt.Println("ESTIMATED COST")
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("Total: %s\n", instmetrics.FormatCost(metrics.TotalCost))

	// Budget status
	if cfg.Resources.CostWarningThreshold > 0 {
		if metrics.TotalCost >= cfg.Resources.CostWarningThreshold {
			fmt.Printf("⚠ WARNING: Cost exceeds warning threshold (%s)\n",
				instmetrics.FormatCost(cfg.Resources.CostWarningThreshold))
		} else {
			remaining := cfg.Resources.CostWarningThreshold - metrics.TotalCost
			fmt.Printf("Warning threshold: %s (remaining: %s)\n",
				instmetrics.FormatCost(cfg.Resources.CostWarningThreshold),
				instmetrics.FormatCost(remaining))
		}
	}
	if cfg.Resources.CostLimit > 0 {
		if metrics.TotalCost >= cfg.Resources.CostLimit {
			fmt.Printf("🛑 LIMIT REACHED: Cost limit (%s) exceeded - instances paused\n",
				instmetrics.FormatCost(cfg.Resources.CostLimit))
		} else {
			remaining := cfg.Resources.CostLimit - metrics.TotalCost
			fmt.Printf("Cost limit: %s (remaining: %s)\n",
				instmetrics.FormatCost(cfg.Resources.CostLimit),
				instmetrics.FormatCost(remaining))
		}
	}
	fmt.Println()

	// Per-instance breakdown
	fmt.Println("TOP INSTANCES BY COST")
	fmt.Println(strings.Repeat("─", 50))

	// Sort instances by cost
	type instData struct {
		num    int
		id     string
		task   string
		cost   float64
		tokens int64
		status string
	}
	var instances []instData
	for i, inst := range session.Instances {
		data := instData{
			num:    i + 1,
			id:     inst.ID,
			task:   inst.Task,
			status: string(inst.Status),
		}
		if inst.Metrics != nil {
			data.cost = inst.Metrics.Cost
			data.tokens = inst.Metrics.TotalTokens()
		}
		instances = append(instances, data)
	}

	// Sort by cost descending
	sort.Slice(instances, func(i, j int) bool {
		return instances[i].cost > instances[j].cost
	})

	// Show instances with cost data
	shown := 0
	for _, inst := range instances {
		if inst.cost > 0 {
			shown++
			task := inst.task
			if len(task) > 40 {
				task = task[:37] + "..."
			}
			fmt.Printf("%d. [%d] %s (%s): %s (%s tokens)\n",
				shown, inst.num, task, inst.status,
				instmetrics.FormatCost(inst.cost),
				instmetrics.FormatTokens(inst.tokens))
		}
	}

	if shown == 0 {
		fmt.Println("No cost data available yet. Start instances to track usage.")
	}

	fmt.Println()
	return nil
}

func printStatsJSON(session *orchestrator.Session, metrics *orchestrator.SessionMetrics, cfg *config.Config) error {
	// Build JSON output manually to control formatting
	fmt.Printf(`{
  "session": {
    "id": "%s",
    "name": "%s",
    "created": "%s",
    "instance_count": %d,
    "active_count": %d
  },
  "tokens": {
    "input": %d,
    "output": %d,
    "total": %d,
    "cache_read": %d,
    "cache_write": %d
  },
  "cost": {
    "total": %.4f,
    "warning_threshold": %.2f,
    "limit": %.2f
  },
  "instances": [`,
		session.ID,
		session.Name,
		session.Created.Format("2006-01-02T15:04:05Z"),
		metrics.InstanceCount,
		metrics.ActiveCount,
		metrics.TotalInputTokens,
		metrics.TotalOutputTokens,
		metrics.TotalInputTokens+metrics.TotalOutputTokens,
		metrics.TotalCacheRead,
		metrics.TotalCacheWrite,
		metrics.TotalCost,
		cfg.Resources.CostWarningThreshold,
		cfg.Resources.CostLimit,
	)

	for i, inst := range session.Instances {
		cost := 0.0
		inputTokens := int64(0)
		outputTokens := int64(0)
		if inst.Metrics != nil {
			cost = inst.Metrics.Cost
			inputTokens = inst.Metrics.InputTokens
			outputTokens = inst.Metrics.OutputTokens
		}
		fmt.Printf(`
    {
      "id": "%s",
      "task": "%s",
      "status": "%s",
      "input_tokens": %d,
      "output_tokens": %d,
      "cost": %.4f
    }`,
			inst.ID,
			strings.ReplaceAll(inst.Task, `"`, `\"`),
			inst.Status,
			inputTokens,
			outputTokens,
			cost,
		)
		if i < len(session.Instances)-1 {
			fmt.Print(",")
		}
	}

	fmt.Println(`
  ]
}`)
	return nil
}

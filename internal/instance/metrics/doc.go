// Package metrics provides token usage and cost parsing for Claude instances.
//
// This package extracts resource usage metrics from Claude's terminal output,
// including token counts, cache statistics, and estimated costs. It enables
// budget tracking and resource management across multiple concurrent instances.
//
// The parser is tailored to Claude Code output; other backends may not emit
// compatible metrics.
//
// # Main Types
//
//   - [MetricsParser]: Regex-based parser for extracting metrics from output
//   - [ParsedMetrics]: Structured metrics data (tokens, cache, cost, API calls)
//
// # Parsed Metrics
//
// The parser extracts:
//   - Input tokens and output tokens
//   - Cache read and write tokens (for prompt caching)
//   - Estimated cost in USD
//   - Number of API calls made
//
// # Token Formats
//
// The parser handles various Claude output formats:
//   - Standard: "Input: 1.5K tokens / Output: 500 tokens"
//   - Shorthand: "1.5K in / 500 out"
//   - Raw numbers: "1500 input tokens, 500 output tokens"
//   - With cost: "$0.05 (1.5K in / 500 out)"
//
// # Cost Calculation
//
// When cost is not directly available in output, use [CalculateCost] to
// estimate based on token counts and current Claude API pricing.
//
// # Thread Safety
//
// [MetricsParser] is safe for concurrent use. Each Parse call operates
// independently on the provided input string.
//
// # Basic Usage
//
//	parser := metrics.NewMetricsParser()
//
//	// Parse metrics from Claude output
//	m := parser.Parse(output)
//	if m != nil {
//	    fmt.Printf("Tokens: %d in / %d out\n", m.InputTokens, m.OutputTokens)
//	    fmt.Printf("Cost: %s\n", metrics.FormatCost(m.Cost))
//	}
//
// # Formatting Helpers
//
// The package provides formatting utilities:
//   - [FormatTokens]: Formats token counts (e.g., "1.5K", "2.3M")
//   - [FormatCost]: Formats cost in USD (e.g., "$0.05", "$1.23")
package metrics

package instance

import (
	"regexp"
	"strconv"
	"strings"
)

// ParsedMetrics holds metrics extracted from Claude Code output
type ParsedMetrics struct {
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	Cost         float64
	APICalls     int
}

// Compile-time interface check for MetricsParser
var _ MetricsParsing = (*MetricsParser)(nil)

// MetricsParser extracts resource metrics from Claude Code output
type MetricsParser struct {
	// Compiled regex patterns
	tokenPattern *regexp.Regexp
	costPattern  *regexp.Regexp
	apiPattern   *regexp.Regexp
	cachePattern *regexp.Regexp
}

// NewMetricsParser creates a new metrics parser
func NewMetricsParser() *MetricsParser {
	return &MetricsParser{
		// Match patterns like "45.2K input" or "12,800 output" or "45200 input"
		// Claude Code status line format: "Total: 45.2K input, 12.8K output"
		tokenPattern: regexp.MustCompile(`(?i)(?:total:?\s*)?(\d+(?:[.,]\d+)?)\s*([KkMm])?\s*(input|in)\s*[,/|]\s*(\d+(?:[.,]\d+)?)\s*([KkMm])?\s*(output|out)`),
		// Match patterns like "Cost: $0.42" or "$1.23" or "~$0.42"
		costPattern: regexp.MustCompile(`(?i)(?:cost:?\s*)?~?\$(\d+(?:\.\d+)?)`),
		// Match patterns like "API calls: 5" or "Calls: 12"
		apiPattern: regexp.MustCompile(`(?i)(?:api\s*)?calls?:?\s*(\d+)`),
		// Match patterns like "Cache: 1.2K read, 500 write" or cache_read/cache_write
		cachePattern: regexp.MustCompile(`(?i)cache[_\s]*(?:read)?:?\s*(\d+(?:[.,]\d+)?)\s*([KkMm])?\s*(?:read)?[,/|]\s*(\d+(?:[.,]\d+)?)\s*([KkMm])?\s*(?:write)?`),
	}
}

// Parse extracts metrics from Claude Code output text
func (p *MetricsParser) Parse(output []byte) *ParsedMetrics {
	if len(output) == 0 {
		return nil
	}

	// Focus on the last portion of output where status line appears
	text := string(output)
	if len(text) > 5000 {
		text = text[len(text)-5000:]
	}

	// Strip ANSI escape codes for cleaner pattern matching
	text = stripAnsi(text)

	metrics := &ParsedMetrics{}
	found := false

	// Parse token counts
	if matches := p.tokenPattern.FindStringSubmatch(text); matches != nil {
		inputVal := parseTokenValue(matches[1], matches[2])
		outputVal := parseTokenValue(matches[4], matches[5])
		if inputVal > 0 || outputVal > 0 {
			metrics.InputTokens = inputVal
			metrics.OutputTokens = outputVal
			found = true
		}
	}

	// Parse cost
	if matches := p.costPattern.FindAllStringSubmatch(text, -1); matches != nil {
		// Use the last (most recent) cost value
		lastMatch := matches[len(matches)-1]
		if cost, err := strconv.ParseFloat(lastMatch[1], 64); err == nil {
			metrics.Cost = cost
			found = true
		}
	}

	// Parse API calls
	if matches := p.apiPattern.FindStringSubmatch(text); matches != nil {
		if calls, err := strconv.Atoi(matches[1]); err == nil {
			metrics.APICalls = calls
			found = true
		}
	}

	// Parse cache metrics
	if matches := p.cachePattern.FindStringSubmatch(text); matches != nil {
		metrics.CacheRead = parseTokenValue(matches[1], matches[2])
		metrics.CacheWrite = parseTokenValue(matches[3], matches[4])
		if metrics.CacheRead > 0 || metrics.CacheWrite > 0 {
			found = true
		}
	}

	if !found {
		return nil
	}

	return metrics
}

// parseTokenValue parses a token count value with optional K/M suffix
func parseTokenValue(numStr, suffix string) int64 {
	if numStr == "" {
		return 0
	}

	// Handle comma in numbers like "12,800"
	numStr = strings.ReplaceAll(numStr, ",", "")

	// Parse the base number
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	// Apply suffix multiplier
	suffix = strings.ToUpper(suffix)
	switch suffix {
	case "K":
		val *= 1000
	case "M":
		val *= 1000000
	}

	return int64(val)
}

// CalculateCost estimates the cost based on token counts using Claude API pricing
// Pricing as of 2024 for Claude 3.5 Sonnet (the model used by Claude Code):
// - Input: $3.00 per 1M tokens
// - Output: $15.00 per 1M tokens
// - Cache read: $0.30 per 1M tokens
// - Cache write: $3.75 per 1M tokens
func CalculateCost(inputTokens, outputTokens, cacheRead, cacheWrite int64) float64 {
	const (
		inputPricePerMillion      = 3.00
		outputPricePerMillion     = 15.00
		cacheReadPricePerMillion  = 0.30
		cacheWritePricePerMillion = 3.75
	)

	inputCost := float64(inputTokens) / 1000000.0 * inputPricePerMillion
	outputCost := float64(outputTokens) / 1000000.0 * outputPricePerMillion
	cacheReadCost := float64(cacheRead) / 1000000.0 * cacheReadPricePerMillion
	cacheWriteCost := float64(cacheWrite) / 1000000.0 * cacheWritePricePerMillion

	return inputCost + outputCost + cacheReadCost + cacheWriteCost
}

// FormatTokens formats a token count for display (e.g., "45.2K")
func FormatTokens(tokens int64) string {
	if tokens >= 1000000 {
		return strconv.FormatFloat(float64(tokens)/1000000.0, 'f', 1, 64) + "M"
	}
	if tokens >= 1000 {
		return strconv.FormatFloat(float64(tokens)/1000.0, 'f', 1, 64) + "K"
	}
	return strconv.FormatInt(tokens, 10)
}

// FormatCost formats a cost value for display (e.g., "$0.42")
func FormatCost(cost float64) string {
	if cost < 0.01 {
		return "$0.00"
	}
	return "$" + strconv.FormatFloat(cost, 'f', 2, 64)
}

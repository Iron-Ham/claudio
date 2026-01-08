package instance

import (
	"testing"
)

func TestMetricsParser_Parse(t *testing.T) {
	parser := NewMetricsParser()

	tests := []struct {
		name           string
		output         string
		wantInputTokens  int64
		wantOutputTokens int64
		wantCost       float64
		wantFound      bool
	}{
		{
			name:           "standard token format",
			output:         "Total: 45.2K input, 12.8K output",
			wantInputTokens:  45200,
			wantOutputTokens: 12800,
			wantFound:      true,
		},
		{
			name:           "lowercase k",
			output:         "Total: 45.2k input, 12.8k output",
			wantInputTokens:  45200,
			wantOutputTokens: 12800,
			wantFound:      true,
		},
		{
			name:           "raw numbers",
			output:         "Total: 1000 input, 500 output",
			wantInputTokens:  1000,
			wantOutputTokens: 500,
			wantFound:      true,
		},
		{
			name:           "with comma separators",
			output:         "Total: 12,800 input, 5,000 output",
			wantInputTokens:  12800,
			wantOutputTokens: 5000,
			wantFound:      true,
		},
		{
			name:           "in/out shorthand",
			output:         "45K in / 12K out",
			wantInputTokens:  45000,
			wantOutputTokens: 12000,
			wantFound:      true,
		},
		{
			name:           "with cost",
			output:         "Total: 45K input, 12K output | Cost: $0.42",
			wantInputTokens:  45000,
			wantOutputTokens: 12000,
			wantCost:       0.42,
			wantFound:      true,
		},
		{
			name:           "cost only",
			output:         "Estimated cost: $1.23",
			wantCost:       1.23,
			wantFound:      true,
		},
		{
			name:           "approximate cost",
			output:         "~$0.50",
			wantCost:       0.50,
			wantFound:      true,
		},
		{
			name:           "empty output",
			output:         "",
			wantFound:      false,
		},
		{
			name:           "no metrics",
			output:         "Claude is working on your task...",
			wantFound:      false,
		},
		{
			name:           "million tokens",
			output:         "Total: 1.5M input, 0.5M output",
			wantInputTokens:  1500000,
			wantOutputTokens: 500000,
			wantFound:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := parser.Parse([]byte(tt.output))

			if tt.wantFound {
				if metrics == nil {
					t.Errorf("Parse() returned nil, want metrics")
					return
				}
				if metrics.InputTokens != tt.wantInputTokens {
					t.Errorf("InputTokens = %d, want %d", metrics.InputTokens, tt.wantInputTokens)
				}
				if metrics.OutputTokens != tt.wantOutputTokens {
					t.Errorf("OutputTokens = %d, want %d", metrics.OutputTokens, tt.wantOutputTokens)
				}
				if tt.wantCost > 0 && metrics.Cost != tt.wantCost {
					t.Errorf("Cost = %f, want %f", metrics.Cost, tt.wantCost)
				}
			} else {
				if metrics != nil {
					t.Errorf("Parse() = %+v, want nil", metrics)
				}
			}
		})
	}
}

func TestParseTokenValue(t *testing.T) {
	tests := []struct {
		numStr string
		suffix string
		want   int64
	}{
		{"45.2", "K", 45200},
		{"45.2", "k", 45200},
		{"1.5", "M", 1500000},
		{"1000", "", 1000},
		{"12,800", "", 12800},
		{"", "", 0},
		{"invalid", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.numStr+tt.suffix, func(t *testing.T) {
			got := parseTokenValue(tt.numStr, tt.suffix)
			if got != tt.want {
				t.Errorf("parseTokenValue(%q, %q) = %d, want %d", tt.numStr, tt.suffix, got, tt.want)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name         string
		inputTokens  int64
		outputTokens int64
		cacheRead    int64
		cacheWrite   int64
		wantMin      float64
		wantMax      float64
	}{
		{
			name:         "basic usage",
			inputTokens:  1000000,
			outputTokens: 100000,
			wantMin:      4.0,  // 3 + 1.5 = 4.5 approximately
			wantMax:      5.0,
		},
		{
			name:         "no tokens",
			inputTokens:  0,
			outputTokens: 0,
			wantMin:      0,
			wantMax:      0.01,
		},
		{
			name:         "input heavy",
			inputTokens:  1000000,
			outputTokens: 10000,
			wantMin:      3.0,
			wantMax:      3.5,
		},
		{
			name:         "output heavy",
			inputTokens:  100000,
			outputTokens: 1000000,
			wantMin:      15.0,
			wantMax:      16.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.inputTokens, tt.outputTokens, tt.cacheRead, tt.cacheWrite)
			if cost < tt.wantMin || cost > tt.wantMax {
				t.Errorf("CalculateCost() = %f, want between %f and %f", cost, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		tokens int64
		want   string
	}{
		{500, "500"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{45200, "45.2K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatTokens(tt.tokens)
			if got != tt.want {
				t.Errorf("FormatTokens(%d) = %q, want %q", tt.tokens, got, tt.want)
			}
		})
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0, "$0.00"},
		{0.001, "$0.00"},
		{0.42, "$0.42"},
		{1.23, "$1.23"},
		{10.00, "$10.00"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatCost(tt.cost)
			if got != tt.want {
				t.Errorf("FormatCost(%f) = %q, want %q", tt.cost, got, tt.want)
			}
		})
	}
}

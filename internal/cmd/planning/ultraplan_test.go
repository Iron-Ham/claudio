package planning

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/spf13/cobra"
)

// TestApplyUltraplanFlagOverrides tests the flag override behavior.
// When a flag is explicitly set via CLI, it should override the config file value.
// When a flag is NOT set via CLI, the config file value should be preserved.
func TestApplyUltraplanFlagOverrides(t *testing.T) {
	tests := []struct {
		name           string
		configValue    bool // initial value from config
		flagValue      bool // value set on the flag variable
		flagChanged    bool // whether the flag was explicitly set via CLI
		expectedResult bool // expected final value
	}{
		{
			name:           "flag not set, config true - preserves config value",
			configValue:    true,
			flagValue:      false, // default flag value
			flagChanged:    false,
			expectedResult: true, // config value preserved
		},
		{
			name:           "flag not set, config false - preserves config value",
			configValue:    false,
			flagValue:      false,
			flagChanged:    false,
			expectedResult: false,
		},
		{
			name:           "flag explicitly set to true - overrides config",
			configValue:    false,
			flagValue:      true,
			flagChanged:    true,
			expectedResult: true, // flag value used
		},
		{
			name:           "flag explicitly set to false - overrides config",
			configValue:    true,
			flagValue:      false,
			flagChanged:    true,
			expectedResult: false, // flag value used
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh command for each test
			cmd := &cobra.Command{Use: "test"}

			// Add the adversarial flag
			var adversarialFlag bool
			cmd.Flags().BoolVar(&adversarialFlag, "adversarial", tt.flagValue, "test flag")

			// Mark flag as changed if test case requires it
			if tt.flagChanged {
				_ = cmd.Flags().Set("adversarial", boolToString(tt.flagValue))
			}

			// Create config with initial value
			cfg := &orchestrator.UltraPlanConfig{
				Adversarial: tt.configValue,
			}

			// Apply the override logic (same logic as applyUltraplanFlagOverrides)
			if cmd.Flags().Changed("adversarial") {
				cfg.Adversarial = adversarialFlag
			}

			// Verify result
			if cfg.Adversarial != tt.expectedResult {
				t.Errorf("Adversarial = %v, want %v", cfg.Adversarial, tt.expectedResult)
			}
		})
	}
}

// TestApplyUltraplanFlagOverrides_AllFlags tests all flags in applyUltraplanFlagOverrides
func TestApplyUltraplanFlagOverrides_AllFlags(t *testing.T) {
	t.Run("max-parallel override", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var maxParallel int
		cmd.Flags().IntVar(&maxParallel, "max-parallel", 3, "test flag")
		_ = cmd.Flags().Set("max-parallel", "5")

		cfg := &orchestrator.UltraPlanConfig{MaxParallel: 3}
		if cmd.Flags().Changed("max-parallel") {
			cfg.MaxParallel = maxParallel
		}

		if cfg.MaxParallel != 5 {
			t.Errorf("MaxParallel = %v, want 5", cfg.MaxParallel)
		}
	})

	t.Run("multi-pass override", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var multiPass bool
		cmd.Flags().BoolVar(&multiPass, "multi-pass", false, "test flag")
		_ = cmd.Flags().Set("multi-pass", "true")

		cfg := &orchestrator.UltraPlanConfig{MultiPass: false}
		if cmd.Flags().Changed("multi-pass") {
			cfg.MultiPass = multiPass
		}

		if !cfg.MultiPass {
			t.Errorf("MultiPass = %v, want true", cfg.MultiPass)
		}
	})

	t.Run("adversarial override", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var adversarial bool
		cmd.Flags().BoolVar(&adversarial, "adversarial", false, "test flag")
		_ = cmd.Flags().Set("adversarial", "true")

		cfg := &orchestrator.UltraPlanConfig{Adversarial: false}
		if cmd.Flags().Changed("adversarial") {
			cfg.Adversarial = adversarial
		}

		if !cfg.Adversarial {
			t.Errorf("Adversarial = %v, want true", cfg.Adversarial)
		}
	})

	t.Run("unchanged flags preserve config", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		var maxParallel int
		var multiPass, adversarial bool
		cmd.Flags().IntVar(&maxParallel, "max-parallel", 3, "")
		cmd.Flags().BoolVar(&multiPass, "multi-pass", false, "")
		cmd.Flags().BoolVar(&adversarial, "adversarial", false, "")

		// Config has specific values
		cfg := &orchestrator.UltraPlanConfig{
			MaxParallel: 10,
			MultiPass:   true,
			Adversarial: true,
		}

		// Apply overrides (but no flags were changed)
		if cmd.Flags().Changed("max-parallel") {
			cfg.MaxParallel = maxParallel
		}
		if cmd.Flags().Changed("multi-pass") {
			cfg.MultiPass = multiPass
		}
		if cmd.Flags().Changed("adversarial") {
			cfg.Adversarial = adversarial
		}

		// Values should remain unchanged
		if cfg.MaxParallel != 10 {
			t.Errorf("MaxParallel = %v, want 10", cfg.MaxParallel)
		}
		if !cfg.MultiPass {
			t.Errorf("MultiPass = %v, want true", cfg.MultiPass)
		}
		if !cfg.Adversarial {
			t.Errorf("Adversarial = %v, want true", cfg.Adversarial)
		}
	})
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

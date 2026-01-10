package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "test.field",
		Value:   123,
		Message: "must be greater than zero",
	}

	expected := "test.field: must be greater than zero (got: 123)"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}

func TestValidationErrors_Error(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		var errs ValidationErrors
		if errs.Error() != "" {
			t.Errorf("Error() for empty = %q, want empty string", errs.Error())
		}
	})

	t.Run("single error", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "test.field", Value: 123, Message: "is invalid"},
		}
		expected := "test.field: is invalid (got: 123)"
		if errs.Error() != expected {
			t.Errorf("Error() = %q, want %q", errs.Error(), expected)
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		errs := ValidationErrors{
			{Field: "field1", Value: "bad", Message: "is invalid"},
			{Field: "field2", Value: -1, Message: "must be positive"},
		}
		result := errs.Error()
		if !strings.Contains(result, "2 validation errors") {
			t.Errorf("Error() should mention 2 errors: %s", result)
		}
		if !strings.Contains(result, "field1") || !strings.Contains(result, "field2") {
			t.Errorf("Error() should mention both fields: %s", result)
		}
	})
}

func TestConfig_Validate_DefaultConfig(t *testing.T) {
	cfg := Default()
	errs := cfg.Validate()
	if len(errs) != 0 {
		t.Errorf("Default config should be valid, got %d errors: %v", len(errs), errs)
	}
}

func TestConfig_Validate_Completion(t *testing.T) {
	tests := []struct {
		name     string
		action   string
		hasError bool
	}{
		{"valid prompt", "prompt", false},
		{"valid keep_branch", "keep_branch", false},
		{"valid merge_staging", "merge_staging", false},
		{"valid merge_main", "merge_main", false},
		{"valid auto_pr", "auto_pr", false},
		{"empty is valid", "", false},
		{"invalid action", "invalid_action", true},
		{"case sensitive", "PROMPT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Completion.DefaultAction = tt.action
			errs := cfg.Validate()

			hasError := false
			for _, err := range errs {
				if err.Field == "completion.default_action" {
					hasError = true
					break
				}
			}

			if hasError != tt.hasError {
				t.Errorf("Validate() for action=%q: hasError=%v, want %v", tt.action, hasError, tt.hasError)
			}
		})
	}
}

func TestConfig_Validate_TUI(t *testing.T) {
	t.Run("negative max_output_lines", func(t *testing.T) {
		cfg := Default()
		cfg.TUI.MaxOutputLines = -1
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "tui.max_output_lines" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for negative max_output_lines")
		}
	})

	t.Run("excessive max_output_lines", func(t *testing.T) {
		cfg := Default()
		cfg.TUI.MaxOutputLines = 200000
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "tui.max_output_lines" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for excessive max_output_lines")
		}
	})

	t.Run("valid zero max_output_lines", func(t *testing.T) {
		cfg := Default()
		cfg.TUI.MaxOutputLines = 0
		errs := cfg.Validate()

		for _, err := range errs {
			if err.Field == "tui.max_output_lines" {
				t.Errorf("zero should be valid, got error: %v", err)
			}
		}
	})
}

func TestConfig_Validate_Instance(t *testing.T) {
	t.Run("buffer size too small", func(t *testing.T) {
		cfg := Default()
		cfg.Instance.OutputBufferSize = 100
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "instance.output_buffer_size" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for small buffer size")
		}
	})

	t.Run("buffer size too large", func(t *testing.T) {
		cfg := Default()
		cfg.Instance.OutputBufferSize = 200_000_000
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "instance.output_buffer_size" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for large buffer size")
		}
	})

	t.Run("capture interval too small", func(t *testing.T) {
		cfg := Default()
		cfg.Instance.CaptureIntervalMs = 5
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "instance.capture_interval_ms" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for small capture interval")
		}
	})

	t.Run("capture interval too large", func(t *testing.T) {
		cfg := Default()
		cfg.Instance.CaptureIntervalMs = 10000
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "instance.capture_interval_ms" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for large capture interval")
		}
	})

	t.Run("tmux dimensions", func(t *testing.T) {
		tests := []struct {
			width, height int
			expectError   bool
			field         string
		}{
			{79, 50, true, "instance.tmux_width"},   // width too small
			{501, 50, true, "instance.tmux_width"},  // width too large
			{200, 23, true, "instance.tmux_height"}, // height too small
			{200, 201, true, "instance.tmux_height"}, // height too large
			{200, 50, false, ""},                    // valid
		}

		for _, tt := range tests {
			cfg := Default()
			cfg.Instance.TmuxWidth = tt.width
			cfg.Instance.TmuxHeight = tt.height
			errs := cfg.Validate()

			found := false
			for _, err := range errs {
				if err.Field == tt.field {
					found = true
					break
				}
			}
			if found != tt.expectError {
				t.Errorf("width=%d, height=%d: found error=%v, want %v", tt.width, tt.height, found, tt.expectError)
			}
		}
	})

	t.Run("negative timeouts", func(t *testing.T) {
		cfg := Default()
		cfg.Instance.ActivityTimeoutMinutes = -1
		cfg.Instance.CompletionTimeoutMinutes = -1
		errs := cfg.Validate()

		activityFound := false
		completionFound := false
		for _, err := range errs {
			if err.Field == "instance.activity_timeout_minutes" {
				activityFound = true
			}
			if err.Field == "instance.completion_timeout_minutes" {
				completionFound = true
			}
		}
		if !activityFound {
			t.Error("expected error for negative activity timeout")
		}
		if !completionFound {
			t.Error("expected error for negative completion timeout")
		}
	})

	t.Run("zero timeouts are valid (disabled)", func(t *testing.T) {
		cfg := Default()
		cfg.Instance.ActivityTimeoutMinutes = 0
		cfg.Instance.CompletionTimeoutMinutes = 0
		errs := cfg.Validate()

		for _, err := range errs {
			if strings.Contains(err.Field, "timeout") {
				t.Errorf("zero timeouts should be valid, got error: %v", err)
			}
		}
	})
}

func TestConfig_Validate_Branch(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		hasError  bool
		errorMsg  string
	}{
		{"valid simple", "claudio", false, ""},
		{"valid with hyphen", "Iron-Ham", false, ""},
		{"valid with underscore", "my_prefix", false, ""},
		{"valid alphanumeric", "feature123", false, ""},
		{"empty prefix", "", true, "cannot be empty"},
		{"starts with number", "123branch", true, "must start with a letter"},
		{"contains slash", "my/branch", true, "must start with a letter"},
		{"contains space", "my branch", true, "must start with a letter"},
		{"contains dot", "my.branch", true, "must start with a letter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			cfg.Branch.Prefix = tt.prefix
			errs := cfg.Validate()

			hasError := false
			for _, err := range errs {
				if err.Field == "branch.prefix" {
					hasError = true
					break
				}
			}

			if hasError != tt.hasError {
				t.Errorf("Validate() for prefix=%q: hasError=%v, want %v", tt.prefix, hasError, tt.hasError)
			}
		})
	}

	t.Run("prefix too long", func(t *testing.T) {
		cfg := Default()
		cfg.Branch.Prefix = strings.Repeat("a", 51)
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "branch.prefix" && strings.Contains(err.Message, "exceeds maximum length") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for prefix exceeding max length")
		}
	})
}

func TestConfig_Validate_Resources(t *testing.T) {
	t.Run("negative cost warning threshold", func(t *testing.T) {
		cfg := Default()
		cfg.Resources.CostWarningThreshold = -1
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "resources.cost_warning_threshold" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for negative cost warning threshold")
		}
	})

	t.Run("negative cost limit", func(t *testing.T) {
		cfg := Default()
		cfg.Resources.CostLimit = -1
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "resources.cost_limit" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for negative cost limit")
		}
	})

	t.Run("warning threshold greater than limit", func(t *testing.T) {
		cfg := Default()
		cfg.Resources.CostWarningThreshold = 20.0
		cfg.Resources.CostLimit = 10.0
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "resources.cost_warning_threshold" && strings.Contains(err.Message, "should be less than") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for warning threshold greater than limit")
		}
	})

	t.Run("warning threshold greater than limit is ok when limit is zero (disabled)", func(t *testing.T) {
		cfg := Default()
		cfg.Resources.CostWarningThreshold = 20.0
		cfg.Resources.CostLimit = 0.0
		errs := cfg.Validate()

		for _, err := range errs {
			if err.Field == "resources.cost_warning_threshold" && strings.Contains(err.Message, "should be less than") {
				t.Error("should not error when limit is disabled (0)")
			}
		}
	})

	t.Run("negative token limit", func(t *testing.T) {
		cfg := Default()
		cfg.Resources.TokenLimitPerInstance = -1
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "resources.token_limit_per_instance" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for negative token limit")
		}
	})
}

func TestConfig_Validate_Ultraplan(t *testing.T) {
	t.Run("max parallel too small", func(t *testing.T) {
		cfg := Default()
		cfg.Ultraplan.MaxParallel = 0
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "ultraplan.max_parallel" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for zero max parallel")
		}
	})

	t.Run("max parallel too large", func(t *testing.T) {
		cfg := Default()
		cfg.Ultraplan.MaxParallel = 25
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "ultraplan.max_parallel" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for excessive max parallel")
		}
	})

	t.Run("sound path does not exist", func(t *testing.T) {
		cfg := Default()
		cfg.Ultraplan.Notifications.SoundPath = "/nonexistent/path/to/sound.wav"
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "ultraplan.notifications.sound_path" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for nonexistent sound path")
		}
	})

	t.Run("sound path exists", func(t *testing.T) {
		// Create a temp file to test with
		tmpDir := t.TempDir()
		soundFile := filepath.Join(tmpDir, "test.wav")
		if err := os.WriteFile(soundFile, []byte("fake audio"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg := Default()
		cfg.Ultraplan.Notifications.SoundPath = soundFile
		errs := cfg.Validate()

		for _, err := range errs {
			if err.Field == "ultraplan.notifications.sound_path" {
				t.Errorf("existing sound path should not error: %v", err)
			}
		}
	})

	t.Run("empty sound path is valid", func(t *testing.T) {
		cfg := Default()
		cfg.Ultraplan.Notifications.SoundPath = ""
		errs := cfg.Validate()

		for _, err := range errs {
			if err.Field == "ultraplan.notifications.sound_path" {
				t.Errorf("empty sound path should be valid: %v", err)
			}
		}
	})
}

func TestConfig_Validate_Plan(t *testing.T) {
	t.Run("valid output formats", func(t *testing.T) {
		for _, format := range []string{"json", "issues", "both", ""} {
			cfg := Default()
			cfg.Plan.OutputFormat = format
			errs := cfg.Validate()

			for _, err := range errs {
				if err.Field == "plan.output_format" {
					t.Errorf("format %q should be valid, got error: %v", format, err)
				}
			}
		}
	})

	t.Run("invalid output format", func(t *testing.T) {
		cfg := Default()
		cfg.Plan.OutputFormat = "invalid"
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "plan.output_format" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for invalid output format")
		}
	})

	t.Run("json format requires output file", func(t *testing.T) {
		cfg := Default()
		cfg.Plan.OutputFormat = "json"
		cfg.Plan.OutputFile = ""
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "plan.output_file" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for empty output file with json format")
		}
	})

	t.Run("both format requires output file", func(t *testing.T) {
		cfg := Default()
		cfg.Plan.OutputFormat = "both"
		cfg.Plan.OutputFile = ""
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "plan.output_file" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for empty output file with both format")
		}
	})

	t.Run("issues format does not require output file", func(t *testing.T) {
		cfg := Default()
		cfg.Plan.OutputFormat = "issues"
		cfg.Plan.OutputFile = ""
		errs := cfg.Validate()

		for _, err := range errs {
			if err.Field == "plan.output_file" {
				t.Errorf("issues format should not require output file: %v", err)
			}
		}
	})
}

func TestConfig_Validate_Logging(t *testing.T) {
	t.Run("valid log levels", func(t *testing.T) {
		for _, level := range []string{"debug", "info", "warn", "error", ""} {
			cfg := Default()
			cfg.Logging.Level = level
			errs := cfg.Validate()

			for _, err := range errs {
				if err.Field == "logging.level" {
					t.Errorf("level %q should be valid, got error: %v", level, err)
				}
			}
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := Default()
		cfg.Logging.Level = "invalid"
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "logging.level" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for invalid log level")
		}
	})

	t.Run("case sensitive log level", func(t *testing.T) {
		cfg := Default()
		cfg.Logging.Level = "INFO"
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "logging.level" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for uppercase log level")
		}
	})

	t.Run("max size must be positive", func(t *testing.T) {
		cfg := Default()
		cfg.Logging.MaxSizeMB = 0
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "logging.max_size_mb" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for zero max size")
		}
	})

	t.Run("max size too large", func(t *testing.T) {
		cfg := Default()
		cfg.Logging.MaxSizeMB = 2000
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "logging.max_size_mb" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for excessive max size")
		}
	})

	t.Run("negative max backups", func(t *testing.T) {
		cfg := Default()
		cfg.Logging.MaxBackups = -1
		errs := cfg.Validate()

		found := false
		for _, err := range errs {
			if err.Field == "logging.max_backups" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected error for negative max backups")
		}
	})

	t.Run("zero max backups is valid", func(t *testing.T) {
		cfg := Default()
		cfg.Logging.MaxBackups = 0
		errs := cfg.Validate()

		for _, err := range errs {
			if err.Field == "logging.max_backups" {
				t.Errorf("zero max backups should be valid: %v", err)
			}
		}
	})
}

func TestValidLogLevels(t *testing.T) {
	levels := ValidLogLevels()
	expected := []string{"debug", "info", "warn", "error"}

	if len(levels) != len(expected) {
		t.Errorf("ValidLogLevels() length = %d, want %d", len(levels), len(expected))
	}

	for i, level := range expected {
		if levels[i] != level {
			t.Errorf("ValidLogLevels()[%d] = %q, want %q", i, levels[i], level)
		}
	}
}

func TestValidOutputFormats(t *testing.T) {
	formats := ValidOutputFormats()
	expected := []string{"json", "issues", "both"}

	if len(formats) != len(expected) {
		t.Errorf("ValidOutputFormats() length = %d, want %d", len(formats), len(expected))
	}

	for i, format := range expected {
		if formats[i] != format {
			t.Errorf("ValidOutputFormats()[%d] = %q, want %q", i, formats[i], format)
		}
	}
}

func TestConfig_Validate_MultipleErrors(t *testing.T) {
	cfg := Default()
	// Set multiple invalid values
	cfg.Branch.Prefix = ""
	cfg.Instance.OutputBufferSize = 10
	cfg.Logging.Level = "invalid"
	cfg.Resources.CostLimit = -1

	errs := cfg.Validate()
	if len(errs) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(errs), errs)
	}
}

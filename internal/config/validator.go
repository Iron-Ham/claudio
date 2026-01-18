package config

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// ValidationError represents a single validation failure
type ValidationError struct {
	Field   string // The config field path (e.g., "instance.output_buffer_size")
	Value   any    // The invalid value
	Message string // Human-readable error description
}

// Error implements the error interface for ValidationError
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s (got: %v)", e.Field, e.Message, e.Value)
}

// ValidationErrors is a collection of validation errors
type ValidationErrors []ValidationError

// Error implements the error interface for ValidationErrors
func (e ValidationErrors) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d validation errors:\n", len(e)))
	for i, err := range e {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// branchPrefixRegex validates branch prefix characters
// Branch names should start with alphanumeric and can contain alphanumeric, hyphen, underscore
var branchPrefixRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// ValidLogLevels returns the list of valid log levels
func ValidLogLevels() []string {
	return []string{"debug", "info", "warn", "error"}
}

// ValidOutputFormats returns the list of valid plan output formats
func ValidOutputFormats() []string {
	return []string{"json", "issues", "both"}
}

// Validate checks the Config for invalid values and returns all validation errors found
func (c *Config) Validate() []ValidationError {
	var errors []ValidationError

	// Validate Completion config
	errors = append(errors, c.validateCompletion()...)

	// Validate TUI config
	errors = append(errors, c.validateTUI()...)

	// Validate Instance config
	errors = append(errors, c.validateInstance()...)

	// Validate Branch config
	errors = append(errors, c.validateBranch()...)

	// Validate Resources config
	errors = append(errors, c.validateResources()...)

	// Validate Ultraplan config
	errors = append(errors, c.validateUltraplan()...)

	// Validate Plan config
	errors = append(errors, c.validatePlan()...)

	// Validate Adversarial config
	errors = append(errors, c.validateAdversarial()...)

	// Validate Logging config
	errors = append(errors, c.validateLogging()...)

	// Validate Paths config
	errors = append(errors, c.validatePaths()...)

	return errors
}

// validateCompletion validates the CompletionConfig
func (c *Config) validateCompletion() []ValidationError {
	var errors []ValidationError

	if c.Completion.DefaultAction != "" && !IsValidCompletionAction(c.Completion.DefaultAction) {
		errors = append(errors, ValidationError{
			Field:   "completion.default_action",
			Value:   c.Completion.DefaultAction,
			Message: fmt.Sprintf("must be one of: %s", strings.Join(ValidCompletionActions(), ", ")),
		})
	}

	return errors
}

// validateTUI validates the TUIConfig
func (c *Config) validateTUI() []ValidationError {
	var errors []ValidationError

	if c.TUI.MaxOutputLines < 0 {
		errors = append(errors, ValidationError{
			Field:   "tui.max_output_lines",
			Value:   c.TUI.MaxOutputLines,
			Message: "must be non-negative",
		})
	}

	// Reasonable upper bound to prevent memory issues
	const maxOutputLinesLimit = 100000
	if c.TUI.MaxOutputLines > maxOutputLinesLimit {
		errors = append(errors, ValidationError{
			Field:   "tui.max_output_lines",
			Value:   c.TUI.MaxOutputLines,
			Message: fmt.Sprintf("exceeds maximum of %d", maxOutputLinesLimit),
		})
	}

	// Sidebar width validation (0 means use default, which is valid).
	// These values must match tui.SidebarMinWidth and tui.SidebarMaxWidth
	// (defined separately to avoid circular import).
	const minSidebarWidth = 20
	const maxSidebarWidth = 60
	if c.TUI.SidebarWidth != 0 {
		if c.TUI.SidebarWidth < minSidebarWidth {
			errors = append(errors, ValidationError{
				Field:   "tui.sidebar_width",
				Value:   c.TUI.SidebarWidth,
				Message: fmt.Sprintf("must be at least %d columns", minSidebarWidth),
			})
		}
		if c.TUI.SidebarWidth > maxSidebarWidth {
			errors = append(errors, ValidationError{
				Field:   "tui.sidebar_width",
				Value:   c.TUI.SidebarWidth,
				Message: fmt.Sprintf("exceeds maximum of %d columns", maxSidebarWidth),
			})
		}
	}

	return errors
}

// validateInstance validates the InstanceConfig
func (c *Config) validateInstance() []ValidationError {
	var errors []ValidationError

	// Buffer size validation
	const minBufferSize = 1024        // 1KB minimum
	const maxBufferSize = 100_000_000 // 100MB maximum

	if c.Instance.OutputBufferSize < minBufferSize {
		errors = append(errors, ValidationError{
			Field:   "instance.output_buffer_size",
			Value:   c.Instance.OutputBufferSize,
			Message: fmt.Sprintf("must be at least %d bytes (1KB)", minBufferSize),
		})
	}
	if c.Instance.OutputBufferSize > maxBufferSize {
		errors = append(errors, ValidationError{
			Field:   "instance.output_buffer_size",
			Value:   c.Instance.OutputBufferSize,
			Message: fmt.Sprintf("exceeds maximum of %d bytes (100MB)", maxBufferSize),
		})
	}

	// Capture interval validation
	const minCaptureInterval = 10   // 10ms minimum
	const maxCaptureInterval = 5000 // 5 seconds maximum

	if c.Instance.CaptureIntervalMs < minCaptureInterval {
		errors = append(errors, ValidationError{
			Field:   "instance.capture_interval_ms",
			Value:   c.Instance.CaptureIntervalMs,
			Message: fmt.Sprintf("must be at least %dms", minCaptureInterval),
		})
	}
	if c.Instance.CaptureIntervalMs > maxCaptureInterval {
		errors = append(errors, ValidationError{
			Field:   "instance.capture_interval_ms",
			Value:   c.Instance.CaptureIntervalMs,
			Message: fmt.Sprintf("exceeds maximum of %dms", maxCaptureInterval),
		})
	}

	// Tmux dimensions validation
	const minTmuxWidth = 80
	const maxTmuxWidth = 500
	const minTmuxHeight = 24
	const maxTmuxHeight = 200

	if c.Instance.TmuxWidth < minTmuxWidth {
		errors = append(errors, ValidationError{
			Field:   "instance.tmux_width",
			Value:   c.Instance.TmuxWidth,
			Message: fmt.Sprintf("must be at least %d columns", minTmuxWidth),
		})
	}
	if c.Instance.TmuxWidth > maxTmuxWidth {
		errors = append(errors, ValidationError{
			Field:   "instance.tmux_width",
			Value:   c.Instance.TmuxWidth,
			Message: fmt.Sprintf("exceeds maximum of %d columns", maxTmuxWidth),
		})
	}
	if c.Instance.TmuxHeight < minTmuxHeight {
		errors = append(errors, ValidationError{
			Field:   "instance.tmux_height",
			Value:   c.Instance.TmuxHeight,
			Message: fmt.Sprintf("must be at least %d rows", minTmuxHeight),
		})
	}
	if c.Instance.TmuxHeight > maxTmuxHeight {
		errors = append(errors, ValidationError{
			Field:   "instance.tmux_height",
			Value:   c.Instance.TmuxHeight,
			Message: fmt.Sprintf("exceeds maximum of %d rows", maxTmuxHeight),
		})
	}

	// Timeout validation (0 means disabled, which is valid; negative is invalid)
	if c.Instance.ActivityTimeoutMinutes < 0 {
		errors = append(errors, ValidationError{
			Field:   "instance.activity_timeout_minutes",
			Value:   c.Instance.ActivityTimeoutMinutes,
			Message: "must be non-negative (0 disables timeout)",
		})
	}
	if c.Instance.CompletionTimeoutMinutes < 0 {
		errors = append(errors, ValidationError{
			Field:   "instance.completion_timeout_minutes",
			Value:   c.Instance.CompletionTimeoutMinutes,
			Message: "must be non-negative (0 disables timeout)",
		})
	}

	return errors
}

// validateBranch validates the BranchConfig
func (c *Config) validateBranch() []ValidationError {
	var errors []ValidationError

	if c.Branch.Prefix == "" {
		errors = append(errors, ValidationError{
			Field:   "branch.prefix",
			Value:   c.Branch.Prefix,
			Message: "cannot be empty",
		})
	} else if !branchPrefixRegex.MatchString(c.Branch.Prefix) {
		errors = append(errors, ValidationError{
			Field:   "branch.prefix",
			Value:   c.Branch.Prefix,
			Message: "must start with a letter and contain only alphanumeric characters, hyphens, or underscores",
		})
	}

	// Git branch names have length limits
	const maxBranchPrefixLength = 50
	if len(c.Branch.Prefix) > maxBranchPrefixLength {
		errors = append(errors, ValidationError{
			Field:   "branch.prefix",
			Value:   c.Branch.Prefix,
			Message: fmt.Sprintf("exceeds maximum length of %d characters", maxBranchPrefixLength),
		})
	}

	return errors
}

// validateResources validates the ResourceConfig
func (c *Config) validateResources() []ValidationError {
	var errors []ValidationError

	// Cost values must be non-negative
	if c.Resources.CostWarningThreshold < 0 {
		errors = append(errors, ValidationError{
			Field:   "resources.cost_warning_threshold",
			Value:   c.Resources.CostWarningThreshold,
			Message: "must be non-negative",
		})
	}
	if c.Resources.CostLimit < 0 {
		errors = append(errors, ValidationError{
			Field:   "resources.cost_limit",
			Value:   c.Resources.CostLimit,
			Message: "must be non-negative (0 disables limit)",
		})
	}

	// If both are set, warning threshold should be less than limit
	if c.Resources.CostLimit > 0 && c.Resources.CostWarningThreshold > c.Resources.CostLimit {
		errors = append(errors, ValidationError{
			Field:   "resources.cost_warning_threshold",
			Value:   c.Resources.CostWarningThreshold,
			Message: fmt.Sprintf("should be less than cost_limit (%v)", c.Resources.CostLimit),
		})
	}

	// Token limit must be non-negative
	if c.Resources.TokenLimitPerInstance < 0 {
		errors = append(errors, ValidationError{
			Field:   "resources.token_limit_per_instance",
			Value:   c.Resources.TokenLimitPerInstance,
			Message: "must be non-negative (0 disables limit)",
		})
	}

	return errors
}

// validateUltraplan validates the UltraplanConfig
func (c *Config) validateUltraplan() []ValidationError {
	var errors []ValidationError

	const minMaxParallel = 1
	const maxMaxParallel = 20

	if c.Ultraplan.MaxParallel < minMaxParallel {
		errors = append(errors, ValidationError{
			Field:   "ultraplan.max_parallel",
			Value:   c.Ultraplan.MaxParallel,
			Message: fmt.Sprintf("must be at least %d", minMaxParallel),
		})
	}
	if c.Ultraplan.MaxParallel > maxMaxParallel {
		errors = append(errors, ValidationError{
			Field:   "ultraplan.max_parallel",
			Value:   c.Ultraplan.MaxParallel,
			Message: fmt.Sprintf("exceeds maximum of %d", maxMaxParallel),
		})
	}

	// Validate sound path if specified
	if c.Ultraplan.Notifications.SoundPath != "" {
		if _, err := os.Stat(c.Ultraplan.Notifications.SoundPath); err != nil {
			errors = append(errors, ValidationError{
				Field:   "ultraplan.notifications.sound_path",
				Value:   c.Ultraplan.Notifications.SoundPath,
				Message: "file does not exist",
			})
		}
	}

	// Validate consolidation mode
	if c.Ultraplan.ConsolidationMode != "" {
		validModes := []string{"stacked", "single"}
		valid := false
		for _, mode := range validModes {
			if c.Ultraplan.ConsolidationMode == mode {
				valid = true
				break
			}
		}
		if !valid {
			errors = append(errors, ValidationError{
				Field:   "ultraplan.consolidation_mode",
				Value:   c.Ultraplan.ConsolidationMode,
				Message: "must be 'stacked' or 'single'",
			})
		}
	}

	// Validate max task retries
	if c.Ultraplan.MaxTaskRetries < 0 {
		errors = append(errors, ValidationError{
			Field:   "ultraplan.max_task_retries",
			Value:   c.Ultraplan.MaxTaskRetries,
			Message: "cannot be negative",
		})
	}

	return errors
}

// validatePlan validates the PlanConfig
func (c *Config) validatePlan() []ValidationError {
	var errors []ValidationError

	// Validate output format
	if c.Plan.OutputFormat != "" && !slices.Contains(ValidOutputFormats(), c.Plan.OutputFormat) {
		errors = append(errors, ValidationError{
			Field:   "plan.output_format",
			Value:   c.Plan.OutputFormat,
			Message: fmt.Sprintf("must be one of: %s", strings.Join(ValidOutputFormats(), ", ")),
		})
	}

	// Output file must not be empty if format is json or both
	if (c.Plan.OutputFormat == "json" || c.Plan.OutputFormat == "both") && c.Plan.OutputFile == "" {
		errors = append(errors, ValidationError{
			Field:   "plan.output_file",
			Value:   c.Plan.OutputFile,
			Message: "cannot be empty when output_format is 'json' or 'both'",
		})
	}

	return errors
}

// validateAdversarial validates the AdversarialConfig
func (c *Config) validateAdversarial() []ValidationError {
	var errors []ValidationError

	// MaxIterations must be non-negative (0 means unlimited)
	if c.Adversarial.MaxIterations < 0 {
		errors = append(errors, ValidationError{
			Field:   "adversarial.max_iterations",
			Value:   c.Adversarial.MaxIterations,
			Message: "must be non-negative (0 = unlimited)",
		})
	}

	// MinPassingScore must be between 1 and 10
	if c.Adversarial.MinPassingScore < 1 {
		errors = append(errors, ValidationError{
			Field:   "adversarial.min_passing_score",
			Value:   c.Adversarial.MinPassingScore,
			Message: "must be at least 1",
		})
	}
	if c.Adversarial.MinPassingScore > 10 {
		errors = append(errors, ValidationError{
			Field:   "adversarial.min_passing_score",
			Value:   c.Adversarial.MinPassingScore,
			Message: "cannot exceed 10",
		})
	}

	return errors
}

// validateLogging validates the LoggingConfig
func (c *Config) validateLogging() []ValidationError {
	var errors []ValidationError

	// Validate log level
	if c.Logging.Level != "" && !slices.Contains(ValidLogLevels(), c.Logging.Level) {
		errors = append(errors, ValidationError{
			Field:   "logging.level",
			Value:   c.Logging.Level,
			Message: fmt.Sprintf("must be one of: %s", strings.Join(ValidLogLevels(), ", ")),
		})
	}

	// Max size must be positive
	if c.Logging.MaxSizeMB <= 0 {
		errors = append(errors, ValidationError{
			Field:   "logging.max_size_mb",
			Value:   c.Logging.MaxSizeMB,
			Message: "must be positive",
		})
	}

	// Reasonable upper bound for log file size
	const maxLogSizeMB = 1000 // 1GB
	if c.Logging.MaxSizeMB > maxLogSizeMB {
		errors = append(errors, ValidationError{
			Field:   "logging.max_size_mb",
			Value:   c.Logging.MaxSizeMB,
			Message: fmt.Sprintf("exceeds maximum of %dMB", maxLogSizeMB),
		})
	}

	// Max backups must be non-negative
	if c.Logging.MaxBackups < 0 {
		errors = append(errors, ValidationError{
			Field:   "logging.max_backups",
			Value:   c.Logging.MaxBackups,
			Message: "must be non-negative",
		})
	}

	return errors
}

// validatePaths validates the PathsConfig
func (c *Config) validatePaths() []ValidationError {
	var errors []ValidationError

	// WorktreeDir validation - if set, check for invalid characters
	if c.Paths.WorktreeDir != "" {
		path := c.Paths.WorktreeDir

		// Check for null bytes which are invalid in paths
		if strings.ContainsRune(path, '\x00') {
			errors = append(errors, ValidationError{
				Field:   "paths.worktree_dir",
				Value:   path,
				Message: "path contains invalid null character",
			})
		}

		// Reasonable path length limit (most filesystems have limits around 4096)
		const maxPathLength = 4096
		if len(path) > maxPathLength {
			errors = append(errors, ValidationError{
				Field:   "paths.worktree_dir",
				Value:   path,
				Message: fmt.Sprintf("path exceeds maximum length of %d characters", maxPathLength),
			})
		}
	}

	// Validate sparse checkout configuration
	errors = append(errors, c.validateSparseCheckout()...)

	return errors
}

// validateSparseCheckout validates the SparseCheckoutConfig
func (c *Config) validateSparseCheckout() []ValidationError {
	var errors []ValidationError

	sc := c.Paths.SparseCheckout

	// If enabled, at least one directory must be specified
	if sc.Enabled && len(sc.Directories) == 0 {
		errors = append(errors, ValidationError{
			Field:   "paths.sparse_checkout.directories",
			Value:   sc.Directories,
			Message: "at least one directory is required when sparse checkout is enabled",
		})
	}

	// Validate total number of paths (directories + always_include)
	const maxPaths = 100
	totalPaths := len(sc.Directories) + len(sc.AlwaysInclude)
	if totalPaths > maxPaths {
		errors = append(errors, ValidationError{
			Field:   "paths.sparse_checkout",
			Value:   totalPaths,
			Message: fmt.Sprintf("total paths (directories + always_include) exceeds maximum of %d", maxPaths),
		})
	}

	// Validate directories
	errors = append(errors, validateDirectoryList(sc.Directories, "paths.sparse_checkout.directories", sc.ConeMode)...)

	// Validate always_include
	errors = append(errors, validateDirectoryList(sc.AlwaysInclude, "paths.sparse_checkout.always_include", sc.ConeMode)...)

	// Check for duplicates between directories and always_include
	errors = append(errors, checkDuplicateDirectories(sc.Directories, sc.AlwaysInclude)...)

	return errors
}

// validateDirectoryList validates a list of directory paths for sparse checkout
func validateDirectoryList(dirs []string, fieldPrefix string, coneMode bool) []ValidationError {
	var errors []ValidationError

	seen := make(map[string]bool)

	for i, dir := range dirs {
		fieldName := fmt.Sprintf("%s[%d]", fieldPrefix, i)

		// Directory cannot be empty
		if strings.TrimSpace(dir) == "" {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: "directory path cannot be empty",
			})
			continue
		}

		// Check for null bytes
		if strings.ContainsRune(dir, '\x00') {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: "directory path contains invalid null character",
			})
		}

		// Cannot be absolute path
		if strings.HasPrefix(dir, "/") {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: "must be a relative path (remove leading /)",
			})
		}

		// Cannot contain parent directory references
		if strings.Contains(dir, "..") {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: "cannot contain parent directory references (..)",
			})
		}

		// In cone mode, wildcards are not supported
		if coneMode && strings.ContainsAny(dir, "*?[") {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: "wildcards are not supported in cone mode; use directory paths like 'ios/' or 'src/web/'",
			})
		}

		// Path length limit
		const maxPathLength = 1024
		if len(dir) > maxPathLength {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: fmt.Sprintf("path exceeds maximum length of %d characters", maxPathLength),
			})
		}

		// Check for duplicates within the same list (normalize trailing slashes)
		normalized := strings.TrimSuffix(dir, "/")
		if seen[normalized] {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Value:   dir,
				Message: "duplicate directory path",
			})
		}
		seen[normalized] = true
	}

	return errors
}

// checkDuplicateDirectories checks for duplicates between directories and always_include
func checkDuplicateDirectories(dirs, alwaysInclude []string) []ValidationError {
	var errors []ValidationError

	// Build set of normalized directory paths
	dirSet := make(map[string]bool)
	for _, d := range dirs {
		normalized := strings.TrimSuffix(d, "/")
		dirSet[normalized] = true
	}

	// Check always_include against directories
	for i, d := range alwaysInclude {
		normalized := strings.TrimSuffix(d, "/")
		if dirSet[normalized] {
			errors = append(errors, ValidationError{
				Field:   fmt.Sprintf("paths.sparse_checkout.always_include[%d]", i),
				Value:   d,
				Message: "directory already specified in 'directories' list",
			})
		}
	}

	return errors
}

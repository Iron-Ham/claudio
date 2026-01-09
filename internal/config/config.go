package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete Claudio configuration
type Config struct {
	Completion CompletionConfig `mapstructure:"completion"`
	TUI        TUIConfig        `mapstructure:"tui"`
	Session    SessionConfig    `mapstructure:"session"`
	Instance   InstanceConfig   `mapstructure:"instance"`
	Branch     BranchConfig     `mapstructure:"branch"`
	PR         PRConfig         `mapstructure:"pr"`
	Cleanup    CleanupConfig    `mapstructure:"cleanup"`
	Resources  ResourceConfig   `mapstructure:"resources"`
	Ultraplan  UltraplanConfig  `mapstructure:"ultraplan"`
	Review     ReviewConfig     `mapstructure:"review"`
}

// CompletionConfig controls what happens when an instance completes
type CompletionConfig struct {
	// DefaultAction is the action to take when an instance completes
	// Options: "prompt", "keep_branch", "merge_staging", "merge_main", "auto_pr"
	DefaultAction string `mapstructure:"default_action"`
}

// TUIConfig controls the terminal UI behavior
type TUIConfig struct {
	// AutoFocusOnInput automatically focuses new instances for input
	AutoFocusOnInput bool `mapstructure:"auto_focus_on_input"`
	// MaxOutputLines limits how many lines of output to display per instance
	MaxOutputLines int `mapstructure:"max_output_lines"`
}

// SessionConfig controls session behavior
type SessionConfig struct {
	// Placeholder for future session settings
}

// InstanceConfig controls instance behavior
type InstanceConfig struct {
	// OutputBufferSize is the size of the output ring buffer in bytes
	OutputBufferSize int `mapstructure:"output_buffer_size"`
	// CaptureInterval is how often to capture output from tmux (in milliseconds)
	CaptureIntervalMs int `mapstructure:"capture_interval_ms"`
	// TmuxWidth is the width of the tmux pane
	TmuxWidth int `mapstructure:"tmux_width"`
	// TmuxHeight is the height of the tmux pane
	TmuxHeight int `mapstructure:"tmux_height"`
	// ActivityTimeoutMinutes is the number of minutes of no new output before marking as stuck (0 = disabled)
	ActivityTimeoutMinutes int `mapstructure:"activity_timeout_minutes"`
	// CompletionTimeoutMinutes is the maximum total runtime in minutes before marking as timeout (0 = disabled)
	CompletionTimeoutMinutes int `mapstructure:"completion_timeout_minutes"`
	// StaleDetection enables detection of stuck instances via output pattern analysis
	StaleDetection bool `mapstructure:"stale_detection"`
}

// BranchConfig controls branch naming conventions
type BranchConfig struct {
	// Prefix is the branch name prefix (default: "claudio")
	// Examples: "claudio", "Iron-Ham", "feature"
	Prefix string `mapstructure:"prefix"`
	// IncludeID includes the instance ID in branch names (default: true)
	// When true: <prefix>/<id>-<slug>
	// When false: <prefix>/<slug>
	IncludeID bool `mapstructure:"include_id"`
}

// PRConfig controls pull request creation behavior
type PRConfig struct {
	// Draft creates PRs as drafts by default
	Draft bool `mapstructure:"draft"`
	// AutoRebase rebases on main before creating PR (default: true)
	AutoRebase bool `mapstructure:"auto_rebase"`
	// UseAI uses Claude to generate PR title and description (default: true)
	UseAI bool `mapstructure:"use_ai"`
	// AutoPROnStop automatically creates a PR when an instance is stopped with 'x' (default: false)
	AutoPROnStop bool `mapstructure:"auto_pr_on_stop"`
	// Template is a custom PR body template using Go text/template syntax
	Template string `mapstructure:"template"`
	// Reviewers configuration for automatic reviewer assignment
	Reviewers ReviewerConfig `mapstructure:"reviewers"`
	// Labels to add to all PRs by default
	Labels []string `mapstructure:"labels"`
}

// ReviewerConfig controls automatic reviewer assignment
type ReviewerConfig struct {
	// Default reviewers to always assign
	Default []string `mapstructure:"default"`
	// ByPath maps file path patterns to reviewers (glob patterns supported)
	ByPath map[string][]string `mapstructure:"by_path"`
}

// CleanupConfig controls automatic and manual cleanup behavior
type CleanupConfig struct {
	// WarnOnStale shows a warning on start if stale resources exist (default: true)
	WarnOnStale bool `mapstructure:"warn_on_stale"`
	// KeepRemoteBranches prevents deletion of branches that exist on remote (default: true)
	KeepRemoteBranches bool `mapstructure:"keep_remote_branches"`
}

// ResourceConfig controls resource monitoring and cost tracking
type ResourceConfig struct {
	// CostWarningThreshold triggers a warning when session cost exceeds this amount (USD)
	CostWarningThreshold float64 `mapstructure:"cost_warning_threshold"`
	// CostLimit pauses all instances when session cost exceeds this amount (USD), 0 = no limit
	CostLimit float64 `mapstructure:"cost_limit"`
	// TokenLimitPerInstance limits tokens per instance, 0 = no limit
	TokenLimitPerInstance int64 `mapstructure:"token_limit_per_instance"`
	// ShowMetricsInSidebar shows token/cost metrics in TUI sidebar
	ShowMetricsInSidebar bool `mapstructure:"show_metrics_in_sidebar"`
}

// UltraplanConfig controls ultraplan behavior
type UltraplanConfig struct {
	// Notifications controls audio notifications for user input
	Notifications NotificationConfig `mapstructure:"notifications"`
}

// NotificationConfig controls notification behavior for ultraplan
type NotificationConfig struct {
	// Enabled controls whether notifications are played (default: true)
	Enabled bool `mapstructure:"enabled"`
	// UseSound plays system sound on macOS in addition to bell (default: false)
	UseSound bool `mapstructure:"use_sound"`
	// SoundPath custom sound file path (macOS only, default: system Glass sound)
	SoundPath string `mapstructure:"sound_path"`
}

// ReviewConfig controls the parallel code review system behavior
type ReviewConfig struct {
	// EnabledAgents specifies which review agents to use by default
	// Valid values: "security", "performance", "style", "tests", "general"
	EnabledAgents []string `mapstructure:"enabled_agents"`

	// SeverityThreshold is the minimum severity level to report
	// Valid values: "info", "minor", "major", "critical"
	// Default: "minor"
	SeverityThreshold string `mapstructure:"severity_threshold"`

	// WatchMode enables continuous file watching for real-time reviews
	// Default: false
	WatchMode bool `mapstructure:"watch_mode"`

	// DebounceMs is the debounce interval in milliseconds for file watching
	// Prevents excessive review triggers during rapid file changes
	// Default: 500
	DebounceMs int `mapstructure:"debounce_ms"`

	// AutoPauseOnCritical pauses the implementer session when critical issues are found
	// This provides a safety mechanism for severe problems
	// Default: false
	AutoPauseOnCritical bool `mapstructure:"auto_pause_on_critical"`

	// MaxParallelAgents limits the number of concurrent review agents
	// Higher values increase parallelism but also resource usage
	// Default: 3
	MaxParallelAgents int `mapstructure:"max_parallel_agents"`

	// Prompts contains custom prompt overrides for each agent type
	Prompts ReviewPromptsConfig `mapstructure:"prompts"`

	// OutputFormat specifies how review results are formatted
	// Valid values: "json", "markdown", "inline"
	// Default: "markdown"
	OutputFormat string `mapstructure:"output_format"`
}

// ReviewPromptsConfig contains custom prompt overrides for review agents
// Empty strings use the default built-in prompts
type ReviewPromptsConfig struct {
	// Security is a custom prompt for the security review agent
	Security string `mapstructure:"security"`

	// Performance is a custom prompt for the performance review agent
	Performance string `mapstructure:"performance"`

	// Style is a custom prompt for the style/code quality review agent
	Style string `mapstructure:"style"`

	// Tests is a custom prompt for the test coverage review agent
	Tests string `mapstructure:"tests"`

	// General is a custom prompt for the general review agent
	General string `mapstructure:"general"`
}

// Default returns a Config with sensible default values
func Default() *Config {
	return &Config{
		Completion: CompletionConfig{
			DefaultAction: "prompt",
		},
		TUI: TUIConfig{
			AutoFocusOnInput: true,
			MaxOutputLines:   1000,
		},
		Session: SessionConfig{},
		Instance: InstanceConfig{
			OutputBufferSize:         100000, // 100KB
			CaptureIntervalMs:        100,
			TmuxWidth:                200,
			TmuxHeight:               50,
			ActivityTimeoutMinutes:   30,  // 30 minutes of no activity
			CompletionTimeoutMinutes: 120, // 2 hours max runtime
			StaleDetection:           true,
		},
		Branch: BranchConfig{
			Prefix:    "claudio",
			IncludeID: true,
		},
		PR: PRConfig{
			Draft:        false,
			AutoRebase:   true,
			UseAI:        true,
			AutoPROnStop: false,
			Template:     "",
			Reviewers: ReviewerConfig{
				Default: []string{},
				ByPath:  map[string][]string{},
			},
			Labels: []string{},
		},
		Cleanup: CleanupConfig{
			WarnOnStale:        true,
			KeepRemoteBranches: true,
		},
		Resources: ResourceConfig{
			CostWarningThreshold:  5.00,  // Warn at $5
			CostLimit:             0,     // No limit by default
			TokenLimitPerInstance: 0,     // No limit by default
			ShowMetricsInSidebar:  true,  // Show metrics by default
		},
		Ultraplan: UltraplanConfig{
			Notifications: NotificationConfig{
				Enabled:   true,
				UseSound:  false,
				SoundPath: "",
			},
		},
		Review: ReviewConfig{
			EnabledAgents:       []string{"security", "performance", "style"},
			SeverityThreshold:   "minor",
			WatchMode:           false,
			DebounceMs:          500,
			AutoPauseOnCritical: false,
			MaxParallelAgents:   3,
			Prompts: ReviewPromptsConfig{
				Security:    "",
				Performance: "",
				Style:       "",
				Tests:       "",
				General:     "",
			},
			OutputFormat: "markdown",
		},
	}
}

// CaptureInterval returns the capture interval as a time.Duration
func (c *InstanceConfig) CaptureInterval() time.Duration {
	return time.Duration(c.CaptureIntervalMs) * time.Millisecond
}

// ActivityTimeout returns the activity timeout as a time.Duration (0 means disabled)
func (c *InstanceConfig) ActivityTimeout() time.Duration {
	return time.Duration(c.ActivityTimeoutMinutes) * time.Minute
}

// CompletionTimeout returns the completion timeout as a time.Duration (0 means disabled)
func (c *InstanceConfig) CompletionTimeout() time.Duration {
	return time.Duration(c.CompletionTimeoutMinutes) * time.Minute
}

// SetDefaults registers default values with viper
func SetDefaults() {
	defaults := Default()

	// Completion defaults
	viper.SetDefault("completion.default_action", defaults.Completion.DefaultAction)

	// TUI defaults
	viper.SetDefault("tui.auto_focus_on_input", defaults.TUI.AutoFocusOnInput)
	viper.SetDefault("tui.max_output_lines", defaults.TUI.MaxOutputLines)

	// Session defaults (currently empty)

	// Instance defaults
	viper.SetDefault("instance.output_buffer_size", defaults.Instance.OutputBufferSize)
	viper.SetDefault("instance.capture_interval_ms", defaults.Instance.CaptureIntervalMs)
	viper.SetDefault("instance.tmux_width", defaults.Instance.TmuxWidth)
	viper.SetDefault("instance.tmux_height", defaults.Instance.TmuxHeight)
	viper.SetDefault("instance.activity_timeout_minutes", defaults.Instance.ActivityTimeoutMinutes)
	viper.SetDefault("instance.completion_timeout_minutes", defaults.Instance.CompletionTimeoutMinutes)
	viper.SetDefault("instance.stale_detection", defaults.Instance.StaleDetection)

	// Branch defaults
	viper.SetDefault("branch.prefix", defaults.Branch.Prefix)
	viper.SetDefault("branch.include_id", defaults.Branch.IncludeID)

	// PR defaults
	viper.SetDefault("pr.draft", defaults.PR.Draft)
	viper.SetDefault("pr.auto_rebase", defaults.PR.AutoRebase)
	viper.SetDefault("pr.use_ai", defaults.PR.UseAI)
	viper.SetDefault("pr.auto_pr_on_stop", defaults.PR.AutoPROnStop)
	viper.SetDefault("pr.template", defaults.PR.Template)
	viper.SetDefault("pr.reviewers.default", defaults.PR.Reviewers.Default)
	viper.SetDefault("pr.reviewers.by_path", defaults.PR.Reviewers.ByPath)
	viper.SetDefault("pr.labels", defaults.PR.Labels)

	// Cleanup defaults
	viper.SetDefault("cleanup.warn_on_stale", defaults.Cleanup.WarnOnStale)
	viper.SetDefault("cleanup.keep_remote_branches", defaults.Cleanup.KeepRemoteBranches)

	// Resource defaults
	viper.SetDefault("resources.cost_warning_threshold", defaults.Resources.CostWarningThreshold)
	viper.SetDefault("resources.cost_limit", defaults.Resources.CostLimit)
	viper.SetDefault("resources.token_limit_per_instance", defaults.Resources.TokenLimitPerInstance)
	viper.SetDefault("resources.show_metrics_in_sidebar", defaults.Resources.ShowMetricsInSidebar)

	// Ultraplan defaults
	viper.SetDefault("ultraplan.notifications.enabled", defaults.Ultraplan.Notifications.Enabled)
	viper.SetDefault("ultraplan.notifications.use_sound", defaults.Ultraplan.Notifications.UseSound)
	viper.SetDefault("ultraplan.notifications.sound_path", defaults.Ultraplan.Notifications.SoundPath)

	// Review defaults
	viper.SetDefault("review.enabled_agents", defaults.Review.EnabledAgents)
	viper.SetDefault("review.severity_threshold", defaults.Review.SeverityThreshold)
	viper.SetDefault("review.watch_mode", defaults.Review.WatchMode)
	viper.SetDefault("review.debounce_ms", defaults.Review.DebounceMs)
	viper.SetDefault("review.auto_pause_on_critical", defaults.Review.AutoPauseOnCritical)
	viper.SetDefault("review.max_parallel_agents", defaults.Review.MaxParallelAgents)
	viper.SetDefault("review.prompts.security", defaults.Review.Prompts.Security)
	viper.SetDefault("review.prompts.performance", defaults.Review.Prompts.Performance)
	viper.SetDefault("review.prompts.style", defaults.Review.Prompts.Style)
	viper.SetDefault("review.prompts.tests", defaults.Review.Prompts.Tests)
	viper.SetDefault("review.prompts.general", defaults.Review.Prompts.General)
	viper.SetDefault("review.output_format", defaults.Review.OutputFormat)
}

// Load reads the configuration from viper into a Config struct
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Get returns the current configuration (convenience function)
func Get() *Config {
	cfg, err := Load()
	if err != nil {
		// Fall back to defaults if unmarshaling fails
		return Default()
	}
	return cfg
}

// ConfigDir returns the path to the user's config directory
func ConfigDir() string {
	// Check XDG_CONFIG_HOME first
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "claudio")
	}
	// Fall back to ~/.config/claudio
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claudio"
	}
	return filepath.Join(home, ".config", "claudio")
}

// ConfigFile returns the path to the config file
func ConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yaml")
}

// ValidCompletionActions returns the list of valid completion action values
func ValidCompletionActions() []string {
	return []string{"prompt", "keep_branch", "merge_staging", "merge_main", "auto_pr"}
}

// IsValidCompletionAction checks if the given action is valid
func IsValidCompletionAction(action string) bool {
	return slices.Contains(ValidCompletionActions(), action)
}

// ValidReviewAgents returns the list of valid review agent types
func ValidReviewAgents() []string {
	return []string{"security", "performance", "style", "tests", "general"}
}

// IsValidReviewAgent checks if the given agent type is valid
func IsValidReviewAgent(agent string) bool {
	return slices.Contains(ValidReviewAgents(), agent)
}

// ValidSeverityThresholds returns the list of valid severity threshold values
func ValidSeverityThresholds() []string {
	return []string{"info", "minor", "major", "critical"}
}

// IsValidSeverityThreshold checks if the given severity threshold is valid
func IsValidSeverityThreshold(threshold string) bool {
	return slices.Contains(ValidSeverityThresholds(), threshold)
}

// ValidOutputFormats returns the list of valid review output format values
func ValidOutputFormats() []string {
	return []string{"json", "markdown", "inline"}
}

// IsValidOutputFormat checks if the given output format is valid
func IsValidOutputFormat(format string) bool {
	return slices.Contains(ValidOutputFormats(), format)
}

// ValidateReviewConfig validates the review configuration and returns any errors
func (c *ReviewConfig) Validate() error {
	// Validate enabled agents
	for _, agent := range c.EnabledAgents {
		if !IsValidReviewAgent(agent) {
			return fmt.Errorf("invalid review agent %q: valid values are %v", agent, ValidReviewAgents())
		}
	}

	// Validate severity threshold
	if !IsValidSeverityThreshold(c.SeverityThreshold) {
		return fmt.Errorf("invalid severity threshold %q: valid values are %v", c.SeverityThreshold, ValidSeverityThresholds())
	}

	// Validate output format
	if !IsValidOutputFormat(c.OutputFormat) {
		return fmt.Errorf("invalid output format %q: valid values are %v", c.OutputFormat, ValidOutputFormats())
	}

	// Validate debounce interval (must be positive)
	if c.DebounceMs < 0 {
		return fmt.Errorf("debounce_ms must be non-negative, got %d", c.DebounceMs)
	}

	// Validate max parallel agents (must be at least 1)
	if c.MaxParallelAgents < 1 {
		return fmt.Errorf("max_parallel_agents must be at least 1, got %d", c.MaxParallelAgents)
	}

	return nil
}

// Debounce returns the debounce interval as a time.Duration
func (c *ReviewConfig) Debounce() time.Duration {
	return time.Duration(c.DebounceMs) * time.Millisecond
}

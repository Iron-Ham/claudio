package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete Claudio configuration
type Config struct {
	Completion   CompletionConfig   `mapstructure:"completion"`
	TUI          TUIConfig          `mapstructure:"tui"`
	Session      SessionConfig      `mapstructure:"session"`
	Instance     InstanceConfig     `mapstructure:"instance"`
	Branch       BranchConfig       `mapstructure:"branch"`
	PR           PRConfig           `mapstructure:"pr"`
	Cleanup      CleanupConfig      `mapstructure:"cleanup"`
	Resources    ResourceConfig     `mapstructure:"resources"`
	Ultraplan    UltraplanConfig    `mapstructure:"ultraplan"`
	Plan         PlanConfig         `mapstructure:"plan"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Paths        PathsConfig        `mapstructure:"paths"`
	Experimental ExperimentalConfig `mapstructure:"experimental"`
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
	// VerboseCommandHelp shows full command descriptions in command mode instead of single letters
	VerboseCommandHelp bool `mapstructure:"verbose_command_help"`
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
	// TmuxHistoryLimit is the number of lines of scrollback to keep in tmux (default: 50000)
	TmuxHistoryLimit int `mapstructure:"tmux_history_limit"`
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
	// MaxParallel is the maximum number of concurrent child sessions (default: 3)
	MaxParallel int `mapstructure:"max_parallel"`
	// MultiPass enables multi-pass planning where multiple coordinators create plans independently
	// and a coordinator-manager evaluates and combines them (default: false)
	MultiPass bool `mapstructure:"multi_pass"`
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

// PlanConfig controls plan-only mode behavior
type PlanConfig struct {
	// OutputFormat is the default output format: "json", "issues", or "both" (default: "issues")
	OutputFormat string `mapstructure:"output_format"`
	// MultiPass enables multi-pass planning by default (default: false)
	MultiPass bool `mapstructure:"multi_pass"`
	// Labels are default labels to add to GitHub Issues
	Labels []string `mapstructure:"labels"`
	// OutputFile is the default output file path for JSON output (default: ".claudio-plan.json")
	OutputFile string `mapstructure:"output_file"`
}

// LoggingConfig controls debug logging behavior
type LoggingConfig struct {
	// Enabled controls whether debug logging is enabled (default: true)
	Enabled bool `mapstructure:"enabled"`
	// Level is the log level: "debug", "info", "warn", "error" (default: "info")
	Level string `mapstructure:"level"`
	// MaxSizeMB is the maximum log file size in megabytes before rotation (default: 10)
	MaxSizeMB int `mapstructure:"max_size_mb"`
	// MaxBackups is the number of backup log files to keep (default: 3)
	MaxBackups int `mapstructure:"max_backups"`
}

// PathsConfig controls where Claudio stores data
type PathsConfig struct {
	// WorktreeDir is the directory where git worktrees are created.
	// If empty, defaults to ".claudio/worktrees" relative to the repository root.
	// Can be an absolute path to store worktrees outside the repository
	// (e.g., on a faster drive or to avoid cluttering the project).
	// Supports ~ for home directory expansion.
	WorktreeDir string `mapstructure:"worktree_dir"`
}

// ExperimentalConfig controls experimental features that may change or be removed
type ExperimentalConfig struct {
	// IntelligentNaming uses Claude to generate short, descriptive instance names
	// for the sidebar based on the task and Claude's initial output.
	// Requires ANTHROPIC_API_KEY to be set. (default: false)
	IntelligentNaming bool `mapstructure:"intelligent_naming"`

	// TripleShot enables the triple-shot mode which spawns three parallel instances
	// working on the same problem, then uses a judge instance to evaluate and select
	// the best solution. (default: false)
	TripleShot bool `mapstructure:"triple_shot"`

	// InlinePlan enables the :plan command in the standard TUI, allowing users
	// to start Plan workflows directly from the main Claudio interface. (default: false)
	InlinePlan bool `mapstructure:"inline_plan"`

	// InlineUltraPlan enables the :ultraplan command in the standard TUI, allowing users
	// to start UltraPlan workflows directly from the main Claudio interface. (default: false)
	InlineUltraPlan bool `mapstructure:"inline_ultraplan"`

	// GroupedInstanceView enables visual grouping of instances by execution group
	// in the TUI sidebar, organizing related tasks together. (default: false)
	GroupedInstanceView bool `mapstructure:"grouped_instance_view"`
}

// ResolveWorktreeDir returns the resolved worktree directory path.
// If WorktreeDir is empty, it returns the default path relative to baseDir.
// If WorktreeDir starts with ~, it expands to the user's home directory.
// If WorktreeDir is a relative path, it's resolved relative to baseDir.
func (p *PathsConfig) ResolveWorktreeDir(baseDir string) string {
	if p.WorktreeDir == "" {
		return filepath.Join(baseDir, ".claudio", "worktrees")
	}

	path := p.WorktreeDir

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home
		}
	}

	// If relative path, resolve relative to baseDir
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}

	return path
}

// Default returns a Config with sensible default values
func Default() *Config {
	return &Config{
		Completion: CompletionConfig{
			DefaultAction: "prompt",
		},
		TUI: TUIConfig{
			AutoFocusOnInput:   true,
			MaxOutputLines:     1000,
			VerboseCommandHelp: true,
		},
		Session: SessionConfig{},
		Instance: InstanceConfig{
			OutputBufferSize:         100000, // 100KB
			CaptureIntervalMs:        100,
			TmuxWidth:                200,
			TmuxHeight:               50,
			TmuxHistoryLimit:         50000, // 50k lines of scrollback
			ActivityTimeoutMinutes:   30,    // 30 minutes of no activity
			CompletionTimeoutMinutes: 0,     // Disabled by default (no max runtime limit)
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
			CostWarningThreshold:  5.00, // Warn at $5
			CostLimit:             0,    // No limit by default
			TokenLimitPerInstance: 0,    // No limit by default
			ShowMetricsInSidebar:  true, // Show metrics by default
		},
		Ultraplan: UltraplanConfig{
			MaxParallel: 3,
			MultiPass:   false,
			Notifications: NotificationConfig{
				Enabled:   true,
				UseSound:  false,
				SoundPath: "",
			},
		},
		Plan: PlanConfig{
			OutputFormat: "issues",
			MultiPass:    false,
			Labels:       []string{},
			OutputFile:   ".claudio-plan.json",
		},
		Logging: LoggingConfig{
			Enabled:    true,
			Level:      "info",
			MaxSizeMB:  10,
			MaxBackups: 3,
		},
		Paths: PathsConfig{
			WorktreeDir: "", // Empty means use default: .claudio/worktrees
		},
		Experimental: ExperimentalConfig{
			IntelligentNaming:   false, // Disabled by default until stable
			TripleShot:          false, // Disabled by default until stable
			InlinePlan:          false, // Disabled by default until stable
			InlineUltraPlan:     false, // Disabled by default until stable
			GroupedInstanceView: false, // Disabled by default until stable
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
	viper.SetDefault("tui.verbose_command_help", defaults.TUI.VerboseCommandHelp)

	// Session defaults (currently empty)

	// Instance defaults
	viper.SetDefault("instance.output_buffer_size", defaults.Instance.OutputBufferSize)
	viper.SetDefault("instance.capture_interval_ms", defaults.Instance.CaptureIntervalMs)
	viper.SetDefault("instance.tmux_width", defaults.Instance.TmuxWidth)
	viper.SetDefault("instance.tmux_height", defaults.Instance.TmuxHeight)
	viper.SetDefault("instance.tmux_history_limit", defaults.Instance.TmuxHistoryLimit)
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
	viper.SetDefault("ultraplan.max_parallel", defaults.Ultraplan.MaxParallel)
	viper.SetDefault("ultraplan.multi_pass", defaults.Ultraplan.MultiPass)
	viper.SetDefault("ultraplan.notifications.enabled", defaults.Ultraplan.Notifications.Enabled)
	viper.SetDefault("ultraplan.notifications.use_sound", defaults.Ultraplan.Notifications.UseSound)
	viper.SetDefault("ultraplan.notifications.sound_path", defaults.Ultraplan.Notifications.SoundPath)

	// Plan defaults
	viper.SetDefault("plan.output_format", defaults.Plan.OutputFormat)
	viper.SetDefault("plan.multi_pass", defaults.Plan.MultiPass)
	viper.SetDefault("plan.labels", defaults.Plan.Labels)
	viper.SetDefault("plan.output_file", defaults.Plan.OutputFile)

	// Logging defaults
	viper.SetDefault("logging.enabled", defaults.Logging.Enabled)
	viper.SetDefault("logging.level", defaults.Logging.Level)
	viper.SetDefault("logging.max_size_mb", defaults.Logging.MaxSizeMB)
	viper.SetDefault("logging.max_backups", defaults.Logging.MaxBackups)

	// Paths defaults
	viper.SetDefault("paths.worktree_dir", defaults.Paths.WorktreeDir)

	// Experimental defaults
	viper.SetDefault("experimental.intelligent_naming", defaults.Experimental.IntelligentNaming)
	viper.SetDefault("experimental.triple_shot", defaults.Experimental.TripleShot)
	viper.SetDefault("experimental.inline_plan", defaults.Experimental.InlinePlan)
	viper.SetDefault("experimental.inline_ultraplan", defaults.Experimental.InlineUltraPlan)
	viper.SetDefault("experimental.grouped_instance_view", defaults.Experimental.GroupedInstanceView)
}

// Load reads the configuration from viper into a Config struct and validates it
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Validate the configuration
	if errs := cfg.Validate(); len(errs) > 0 {
		return nil, ValidationErrors(errs)
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
	for _, valid := range ValidCompletionActions() {
		if action == valid {
			return true
		}
	}
	return false
}

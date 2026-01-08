package config

import (
	"os"
	"path/filepath"
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
			OutputBufferSize:  100000, // 100KB
			CaptureIntervalMs: 100,
			TmuxWidth:         200,
			TmuxHeight:        50,
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
	}
}

// CaptureInterval returns the capture interval as a time.Duration
func (c *InstanceConfig) CaptureInterval() time.Duration {
	return time.Duration(c.CaptureIntervalMs) * time.Millisecond
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
	for _, valid := range ValidCompletionActions() {
		if action == valid {
			return true
		}
	}
	return false
}

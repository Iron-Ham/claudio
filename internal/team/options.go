package team

import "github.com/Iron-Ham/claudio/internal/coordination"

// ManagerOption configures a Manager.
type ManagerOption func(*managerConfig)

// managerConfig holds optional settings for the Manager.
type managerConfig struct {
	hubOpts []coordination.Option
}

// WithHubOptions sets coordination.Hub options that are applied to every
// team's Hub when it is created. Use this to configure scaling policies,
// instance counts, timing parameters, etc.
func WithHubOptions(opts ...coordination.Option) ManagerOption {
	return func(c *managerConfig) {
		c.hubOpts = append(c.hubOpts, opts...)
	}
}

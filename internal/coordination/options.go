package coordination

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/scaling"
)

// hubConfig holds optional configuration for a Hub.
type hubConfig struct {
	scalingPolicy       *scaling.Policy
	maxTasksPerInstance int
	staleClaimTimeout   time.Duration
	rebalanceInterval   time.Duration
	initialInstances    int
	minInstances        int
	maxInstances        int
}

// Option configures a Hub.
type Option func(*hubConfig)

// WithScalingPolicy sets the scaling policy used by the scaling monitor.
// If nil, a default policy is created.
func WithScalingPolicy(p *scaling.Policy) Option {
	return func(c *hubConfig) { c.scalingPolicy = p }
}

// WithMaxTasksPerInstance sets the maximum tasks per instance for the adaptive lead.
func WithMaxTasksPerInstance(n int) Option {
	return func(c *hubConfig) { c.maxTasksPerInstance = n }
}

// WithStaleClaimTimeout sets the stale claim timeout for the adaptive lead.
func WithStaleClaimTimeout(d time.Duration) Option {
	return func(c *hubConfig) { c.staleClaimTimeout = d }
}

// WithRebalanceInterval sets the rebalance interval for the adaptive lead.
func WithRebalanceInterval(d time.Duration) Option {
	return func(c *hubConfig) { c.rebalanceInterval = d }
}

// WithInitialInstances sets the initial instance count for the scaling monitor.
func WithInitialInstances(n int) Option {
	return func(c *hubConfig) { c.initialInstances = n }
}

// WithMinInstances sets the minimum instance count for the scaling policy.
// A value of 0 uses the policy default.
func WithMinInstances(n int) Option {
	return func(c *hubConfig) { c.minInstances = n }
}

// WithMaxInstances sets the maximum instance count for the scaling policy.
// A value of 0 uses the policy default.
func WithMaxInstances(n int) Option {
	return func(c *hubConfig) { c.maxInstances = n }
}

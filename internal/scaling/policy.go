package scaling

import (
	"fmt"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

// Default policy values.
const (
	defaultMinInstances       = 1
	defaultMaxInstances       = 8
	defaultScaleUpThreshold   = 2
	defaultScaleDownThreshold = 1
	defaultCooldownPeriod     = 30 * time.Second
)

// Option configures a Policy.
type Option func(*Policy)

// WithMinInstances sets the minimum number of instances to maintain.
func WithMinInstances(n int) Option {
	return func(p *Policy) { p.minInstances = n }
}

// WithMaxInstances sets the maximum number of instances allowed.
func WithMaxInstances(n int) Option {
	return func(p *Policy) { p.maxInstances = n }
}

// WithScaleUpThreshold sets the pending task count above which to scale up.
// When pending tasks exceed this threshold and pending > running, scaling up
// is recommended.
func WithScaleUpThreshold(n int) Option {
	return func(p *Policy) { p.scaleUpThreshold = n }
}

// WithScaleDownThreshold sets the idle instance threshold for scaling down.
// When pending == 0 and running <= this threshold, scaling down is recommended.
func WithScaleDownThreshold(n int) Option {
	return func(p *Policy) { p.scaleDownThreshold = n }
}

// WithCooldownPeriod sets the minimum time between scaling decisions.
func WithCooldownPeriod(d time.Duration) Option {
	return func(p *Policy) { p.cooldownPeriod = d }
}

// Policy defines the rules for elastic scaling decisions.
// It is safe for concurrent use.
type Policy struct {
	mu                 sync.Mutex
	minInstances       int
	maxInstances       int
	scaleUpThreshold   int
	scaleDownThreshold int
	cooldownPeriod     time.Duration
	lastDecisionTime   time.Time
}

// NewPolicy creates a Policy with the given options.
// Unset options use defaults.
func NewPolicy(opts ...Option) *Policy {
	p := &Policy{
		minInstances:       defaultMinInstances,
		maxInstances:       defaultMaxInstances,
		scaleUpThreshold:   defaultScaleUpThreshold,
		scaleDownThreshold: defaultScaleDownThreshold,
		cooldownPeriod:     defaultCooldownPeriod,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Evaluate inspects the queue status and current instance count, returning
// a scaling decision. The cooldown period prevents rapid scaling thrash.
func (p *Policy) Evaluate(status taskqueue.QueueStatus, currentInstances int) Decision {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()

	// Check cooldown
	if !p.lastDecisionTime.IsZero() && now.Sub(p.lastDecisionTime) < p.cooldownPeriod {
		return Decision{
			Action: ActionNone,
			Reason: "cooldown period active",
		}
	}

	// Scale up: pending tasks exceed threshold and there's more work than workers
	if status.Pending > p.scaleUpThreshold && status.Pending > status.Running && currentInstances < p.maxInstances {
		delta := status.Pending - status.Running
		// Don't exceed max instances
		if currentInstances+delta > p.maxInstances {
			delta = p.maxInstances - currentInstances
		}
		if delta > 0 {
			p.lastDecisionTime = now
			return Decision{
				Action: ActionScaleUp,
				Delta:  delta,
				Reason: fmt.Sprintf("%d pending tasks with %d running (threshold: %d)", status.Pending, status.Running, p.scaleUpThreshold),
			}
		}
	}

	// Scale down: no pending work and few running tasks
	if status.Pending == 0 && status.Running <= p.scaleDownThreshold && currentInstances > p.minInstances {
		delta := currentInstances - p.minInstances
		// Scale down by at most 1 at a time to be conservative
		if delta > 1 {
			delta = 1
		}
		p.lastDecisionTime = now
		return Decision{
			Action: ActionScaleDown,
			Delta:  -delta,
			Reason: fmt.Sprintf("no pending tasks with %d running (threshold: %d)", status.Running, p.scaleDownThreshold),
		}
	}

	return Decision{
		Action: ActionNone,
		Reason: "no scaling needed",
	}
}

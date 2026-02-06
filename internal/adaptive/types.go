package adaptive

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/taskqueue"
)

// ScalingAction represents a scaling recommendation.
type ScalingAction int

const (
	// ScaleNone indicates no scaling is needed.
	ScaleNone ScalingAction = iota

	// ScaleUp indicates more instances should be added.
	ScaleUp

	// ScaleDown indicates some instances can be removed.
	ScaleDown
)

// String returns a human-readable name for a scaling action.
func (a ScalingAction) String() string {
	switch a {
	case ScaleNone:
		return "none"
	case ScaleUp:
		return "scale_up"
	case ScaleDown:
		return "scale_down"
	default:
		return "unknown"
	}
}

// ScalingRecommendation describes a scaling decision with rationale.
type ScalingRecommendation struct {
	Action      ScalingAction // Recommended action
	TargetCount int           // Suggested total instance count
	Reason      string        // Human-readable explanation
}

// WorkloadSnapshot captures the state of instance workloads at a point in time.
type WorkloadSnapshot struct {
	Distribution map[string]int // instanceID -> task count
	Pending      int            // Pending tasks in queue
	Running      int            // Running tasks
	Total        int            // Total tasks
}

// TaskQueue defines the subset of taskqueue.EventQueue methods the Lead needs.
// This avoids tight coupling to the concrete EventQueue type.
type TaskQueue interface {
	Status() taskqueue.QueueStatus
	Release(taskID, reason string) error
	ClaimNext(instanceID string) (*taskqueue.QueuedTask, error)
	GetInstanceTasks(instanceID string) []*taskqueue.QueuedTask
}

// Option configures a Lead.
type Option func(*Lead)

// WithStaleClaimTimeout sets how long a claim can be idle before
// it is considered stale and eligible for reassignment.
func WithStaleClaimTimeout(d time.Duration) Option {
	return func(l *Lead) {
		l.staleClaimTimeout = d
	}
}

// WithRebalanceInterval sets how often the lead checks for rebalancing
// opportunities. This is the minimum interval between scaling signals.
func WithRebalanceInterval(d time.Duration) Option {
	return func(l *Lead) {
		l.rebalanceInterval = d
	}
}

// WithMaxTasksPerInstance sets the maximum number of tasks a single instance
// should hold before the lead recommends scaling up.
func WithMaxTasksPerInstance(n int) Option {
	return func(l *Lead) {
		l.maxTasksPerInstance = n
	}
}

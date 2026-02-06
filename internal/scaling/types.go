package scaling

// Action represents a scaling decision action.
type Action string

const (
	// ActionScaleUp indicates more instances should be added.
	ActionScaleUp Action = "scale_up"

	// ActionScaleDown indicates instances should be removed.
	ActionScaleDown Action = "scale_down"

	// ActionNone indicates no scaling change is needed.
	ActionNone Action = "none"
)

// String returns the string representation of the action.
func (a Action) String() string {
	return string(a)
}

// Decision is the result of evaluating the scaling policy against the
// current queue state and instance count.
type Decision struct {
	// Action is the recommended scaling action.
	Action Action

	// Delta is the number of instances to add (positive) or remove (negative).
	// Zero when Action is ActionNone.
	Delta int

	// Reason is a human-readable explanation of the decision.
	Reason string
}

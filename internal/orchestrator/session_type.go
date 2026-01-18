package orchestrator

// SessionType identifies the type of session or group for display purposes.
// This determines the icon shown in the sidebar and grouping behavior.
type SessionType string

const (
	// SessionTypeStandard represents a normal task instance (default).
	SessionTypeStandard SessionType = "standard"

	// SessionTypePlan represents a :plan command instance (single-pass planning).
	SessionTypePlan SessionType = "plan"

	// SessionTypePlanMulti represents a :plan command with multi-pass enabled.
	SessionTypePlanMulti SessionType = "plan_multi"

	// SessionTypeUltraPlan represents an :ultraplan orchestrated session.
	SessionTypeUltraPlan SessionType = "ultraplan"

	// SessionTypeTripleShot represents a :tripleshot competing solutions session.
	SessionTypeTripleShot SessionType = "tripleshot"

	// SessionTypeAdversarial represents an adversarial review session with
	// implementer-reviewer feedback loop.
	SessionTypeAdversarial SessionType = "adversarial"

	// SessionTypeRalph represents a Ralph Wiggum iterative development loop.
	// Claude autonomously iterates on work until a completion promise is found.
	SessionTypeRalph SessionType = "ralph"
)

// Icon returns the display icon for this session type.
func (t SessionType) Icon() string {
	switch t {
	case SessionTypePlan:
		return "\u25c7" // ◇ diamond
	case SessionTypePlanMulti:
		return "\u25c8" // ◈ filled diamond
	case SessionTypeUltraPlan:
		return "\u26a1" // ⚡ lightning
	case SessionTypeTripleShot:
		return "\u25b3" // △ triangle
	case SessionTypeAdversarial:
		return "\u2694" // ⚔ crossed swords
	case SessionTypeRalph:
		return "\u267b" // ♻ recycling symbol (iterative loop)
	default:
		return "\u25cf" // ● filled circle (standard)
	}
}

// GroupingMode returns how instances of this type should be grouped.
// Returns:
//   - "none": no grouping (flat in Instances section)
//   - "shared": group in shared category (e.g., "Plans")
//   - "own": create its own named group
func (t SessionType) GroupingMode() string {
	switch t {
	case SessionTypePlan:
		return "shared"
	case SessionTypePlanMulti, SessionTypeUltraPlan, SessionTypeTripleShot, SessionTypeAdversarial, SessionTypeRalph:
		return "own"
	default:
		return "none"
	}
}

// IsOrchestratedType returns true if this session type involves orchestration
// (multiple instances coordinated together).
func (t SessionType) IsOrchestratedType() bool {
	switch t {
	case SessionTypeUltraPlan, SessionTypeTripleShot, SessionTypePlanMulti, SessionTypeAdversarial, SessionTypeRalph:
		return true
	default:
		return false
	}
}

// SharedGroupName returns the name of the shared group for this type.
// Only meaningful when GroupingMode() returns "shared".
func (t SessionType) SharedGroupName() string {
	switch t {
	case SessionTypePlan:
		return "Plans"
	default:
		return ""
	}
}

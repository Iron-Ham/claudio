package team

import (
	"errors"
	"fmt"
	"time"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

// Phase represents the lifecycle phase of a team.
type Phase string

const (
	// PhaseForming indicates the team has been configured but not yet started.
	PhaseForming Phase = "forming"

	// PhaseBlocked indicates the team is waiting for dependencies to complete.
	PhaseBlocked Phase = "blocked"

	// PhaseWorking indicates the team is actively executing tasks.
	PhaseWorking Phase = "working"

	// PhaseReporting indicates the team has finished work and is reporting results.
	PhaseReporting Phase = "reporting"

	// PhaseDone indicates the team completed successfully.
	PhaseDone Phase = "done"

	// PhaseFailed indicates the team failed.
	PhaseFailed Phase = "failed"
)

// String returns the string representation of the phase.
func (p Phase) String() string {
	return string(p)
}

// IsTerminal returns true if this phase represents a final state.
func (p Phase) IsTerminal() bool {
	return p == PhaseDone || p == PhaseFailed
}

// Role describes what kind of work a team performs.
type Role string

const (
	// RoleExecution indicates a team that executes implementation tasks.
	RoleExecution Role = "execution"

	// RolePlanning indicates a team that performs planning and decomposition.
	RolePlanning Role = "planning"

	// RoleReview indicates a team that reviews and validates work.
	RoleReview Role = "review"

	// RoleConsolidation indicates a team that merges and consolidates results.
	RoleConsolidation Role = "consolidation"
)

// String returns the string representation of the role.
func (r Role) String() string {
	return string(r)
}

// IsValid returns true if this is a recognized role value.
func (r Role) IsValid() bool {
	switch r {
	case RoleExecution, RolePlanning, RoleReview, RoleConsolidation:
		return true
	default:
		return false
	}
}

// TokenBudget defines a team's resource limits.
// A zero value for any field means unlimited.
type TokenBudget struct {
	MaxInputTokens  int64   // Maximum input tokens; 0 = unlimited
	MaxOutputTokens int64   // Maximum output tokens; 0 = unlimited
	MaxTotalCost    float64 // Maximum cost in USD; 0 = unlimited
}

// IsUnlimited returns true if all budget limits are zero (unlimited).
func (b TokenBudget) IsUnlimited() bool {
	return b.MaxInputTokens == 0 && b.MaxOutputTokens == 0 && b.MaxTotalCost == 0
}

// BudgetUsage tracks resource consumption.
type BudgetUsage struct {
	InputTokens  int64   // Total input tokens consumed
	OutputTokens int64   // Total output tokens consumed
	TotalCost    float64 // Total cost in USD
}

// Spec configures a team before creation.
type Spec struct {
	ID           string                  // Unique identifier for the team
	Name         string                  // Human-readable team name
	Role         Role                    // What kind of work this team performs
	Tasks        []ultraplan.PlannedTask // Tasks for this team's plan
	LeadPrompt   string                  // System prompt for the team's lead
	TeamSize     int                     // Number of instances for this team
	MinInstances int                     // Floor for scale-down (0 = defaults to TeamSize)
	MaxInstances int                     // Ceiling for scale-up (0 = unlimited)
	Budget       TokenBudget             // Resource limits
	DependsOn    []string                // Team IDs this team waits for
}

// Validate checks that the spec has all required fields.
func (s Spec) Validate() error {
	if s.ID == "" {
		return errors.New("team spec: ID is required")
	}
	if s.Name == "" {
		return errors.New("team spec: Name is required")
	}
	if !s.Role.IsValid() {
		return fmt.Errorf("team spec: invalid role %q", s.Role)
	}
	if len(s.Tasks) == 0 {
		return errors.New("team spec: at least one task is required")
	}
	if s.TeamSize < 1 {
		return errors.New("team spec: TeamSize must be >= 1")
	}
	if s.MinInstances < 0 {
		return errors.New("team spec: MinInstances must be >= 0")
	}
	if s.MaxInstances < 0 {
		return errors.New("team spec: MaxInstances must be >= 0")
	}
	if s.MinInstances > 0 && s.MaxInstances > 0 && s.MinInstances > s.MaxInstances {
		return fmt.Errorf("team spec: MinInstances (%d) must be <= MaxInstances (%d)", s.MinInstances, s.MaxInstances)
	}
	return nil
}

// Status is a read-only snapshot of a team's runtime state.
type Status struct {
	ID          string      // Team identifier
	Name        string      // Human-readable name
	Role        Role        // Team role
	Phase       Phase       // Current lifecycle phase
	TasksTotal  int         // Total tasks in the team's plan
	TasksDone   int         // Tasks completed successfully
	TasksFailed int         // Tasks that failed
	BudgetUsed  BudgetUsage // Current resource consumption
}

// MessageType categorizes inter-team messages.
type MessageType string

const (
	// MessageTypeDiscovery indicates a team sharing a discovery with others.
	MessageTypeDiscovery MessageType = "discovery"

	// MessageTypeDependency indicates a dependency-related notification.
	MessageTypeDependency MessageType = "dependency"

	// MessageTypeWarning indicates a warning from one team to others.
	MessageTypeWarning MessageType = "warning"

	// MessageTypeRequest indicates a team requesting information or action.
	MessageTypeRequest MessageType = "request"
)

// String returns the string representation of the message type.
func (mt MessageType) String() string {
	return string(mt)
}

// MessagePriority indicates the urgency of an inter-team message.
type MessagePriority string

const (
	// PriorityInfo indicates a low-urgency informational message.
	PriorityInfo MessagePriority = "info"

	// PriorityImportant indicates a message that should be read promptly.
	PriorityImportant MessagePriority = "important"

	// PriorityUrgent indicates a high-urgency message requiring immediate attention.
	PriorityUrgent MessagePriority = "urgent"
)

// String returns the string representation of the message priority.
func (mp MessagePriority) String() string {
	return string(mp)
}

// BroadcastRecipient is the sentinel value for messages sent to all teams.
const BroadcastRecipient = "broadcast"

// InterTeamMessage is a message routed between teams.
type InterTeamMessage struct {
	ID        string          // Unique message identifier
	FromTeam  string          // Source team ID
	ToTeam    string          // Destination team ID or BroadcastRecipient
	Type      MessageType     // Message category
	Content   string          // Message body
	Priority  MessagePriority // Urgency level
	Timestamp time.Time       // When the message was created
}

// IsBroadcast returns true if this message is addressed to all teams.
func (m InterTeamMessage) IsBroadcast() bool {
	return m.ToTeam == BroadcastRecipient
}

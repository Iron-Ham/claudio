package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/grouptypes"
)

// InstanceGroup is a type alias to the canonical definition in grouptypes.
// This enables the session package to work directly with the same type used
// by the orchestrator and group packages, eliminating type conversion overhead.
type InstanceGroup = grouptypes.InstanceGroup

// SessionData represents the serializable session state for persistence.
// This type is designed to be independent of the orchestrator's runtime Session type,
// allowing the session manager to operate without circular dependencies.
type SessionData struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	BaseRepo  string          `json:"base_repo"`
	Created   time.Time       `json:"created"`
	Instances []*InstanceData `json:"instances"`

	// Groups holds optional visual groupings of instances for the TUI.
	// When GroupedInstanceView is enabled, instances are organized into groups
	// rather than displayed as a flat list. Groups can have sub-groups for
	// representing nested dependencies (e.g., in Plan/UltraPlan workflows).
	Groups []*InstanceGroup `json:"groups,omitempty"`

	// UltraPlan holds the ultra-plan session state (nil for regular sessions)
	UltraPlan any `json:"ultra_plan,omitempty"`
}

// InstanceData represents instance information for persistence.
type InstanceData struct {
	ID            string       `json:"id"`
	WorktreePath  string       `json:"worktree_path"`
	Branch        string       `json:"branch"`
	Task          string       `json:"task"`
	Status        string       `json:"status"`
	PID           int          `json:"pid,omitempty"`
	FilesModified []string     `json:"files_modified,omitempty"`
	Created       time.Time    `json:"created"`
	TmuxSession   string       `json:"tmux_session,omitempty"`
	Metrics       *MetricsData `json:"metrics,omitempty"`
}

// MetricsData represents instance resource usage metrics for persistence.
type MetricsData struct {
	InputTokens  int64      `json:"input_tokens"`
	OutputTokens int64      `json:"output_tokens"`
	CacheRead    int64      `json:"cache_read,omitempty"`
	CacheWrite   int64      `json:"cache_write,omitempty"`
	Cost         float64    `json:"cost"`
	APICalls     int        `json:"api_calls"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
}

// NewSessionData creates a new SessionData with a generated ID.
func NewSessionData(name, baseRepo string) *SessionData {
	if name == "" {
		name = "claudio-session"
	}

	return &SessionData{
		ID:        generateID(),
		Name:      name,
		BaseRepo:  baseRepo,
		Created:   time.Now(),
		Instances: make([]*InstanceData, 0),
	}
}

// NewInstanceData creates a new InstanceData with a generated ID.
func NewInstanceData(task string) *InstanceData {
	return &InstanceData{
		ID:      generateID(),
		Task:    task,
		Status:  "pending",
		Created: time.Now(),
	}
}

// GetInstance returns an instance by ID, or nil if not found.
func (s *SessionData) GetInstance(id string) *InstanceData {
	for _, inst := range s.Instances {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}

// generateID creates a short random hex ID.
func generateID() string {
	bytes := make([]byte, 4)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateID generates a new random 8-character hex ID.
// Exported for use by other packages that need to generate session/instance IDs.
func GenerateID() string {
	return generateID()
}

// GroupData is kept as a type alias for backwards compatibility.
// New code should use InstanceGroup directly.
type GroupData = InstanceGroup

// NewGroupData creates a new GroupData with a generated ID.
func NewGroupData(name string) *GroupData {
	return grouptypes.NewInstanceGroup(generateID(), name)
}

// GetGroup finds a group by ID within the session's groups (including sub-groups).
func (s *SessionData) GetGroup(id string) *InstanceGroup {
	for _, g := range s.Groups {
		if g.ID == id {
			return g
		}
		if found := g.FindGroup(id); found != nil {
			return found
		}
	}
	return nil
}

// ValidateGroups checks group integrity and returns a cleaned copy of the groups.
// It removes references to instances that don't exist in the session and
// removes empty groups that have no instances and no sub-groups.
// It also validates that all group dependencies reference existing groups.
func (s *SessionData) ValidateGroups() []*InstanceGroup {
	if len(s.Groups) == 0 {
		return nil
	}

	// Build a set of valid instance IDs
	validInstances := make(map[string]bool)
	for _, inst := range s.Instances {
		validInstances[inst.ID] = true
	}

	// First pass: collect all group IDs for dependency validation
	groupIDs := make(map[string]bool)
	collectGroupIDs(s.Groups, groupIDs)

	// Second pass: validate and clean groups
	return validateGroupsRecursive(s.Groups, validInstances, groupIDs)
}

// collectGroupIDs recursively collects all group IDs.
func collectGroupIDs(groups []*InstanceGroup, ids map[string]bool) {
	for _, g := range groups {
		ids[g.ID] = true
		collectGroupIDs(g.SubGroups, ids)
	}
}

// validateGroupsRecursive validates groups and returns cleaned copies.
func validateGroupsRecursive(groups []*InstanceGroup, validInstances, validGroups map[string]bool) []*InstanceGroup {
	var result []*InstanceGroup

	for _, g := range groups {
		// Filter instances to only include valid ones
		var validInsts []string
		for _, instID := range g.Instances {
			if validInstances[instID] {
				validInsts = append(validInsts, instID)
			}
		}

		// Recursively validate sub-groups
		validSubGroups := validateGroupsRecursive(g.SubGroups, validInstances, validGroups)

		// Filter dependencies to only include valid group IDs
		var validDeps []string
		for _, depID := range g.DependsOn {
			if validGroups[depID] {
				validDeps = append(validDeps, depID)
			}
		}

		// Only include group if it has instances or sub-groups
		if len(validInsts) > 0 || len(validSubGroups) > 0 {
			cleaned := g.Clone()
			cleaned.Instances = validInsts
			cleaned.SubGroups = validSubGroups
			cleaned.DependsOn = validDeps
			result = append(result, cleaned)
		}
	}

	return result
}

// GetGroups returns the current list of top-level groups.
// Implements group.ManagerSessionData interface.
func (s *SessionData) GetGroups() []*InstanceGroup {
	return s.Groups
}

// SetGroups replaces the current list of top-level groups.
// Implements group.ManagerSessionData interface.
func (s *SessionData) SetGroups(groups []*InstanceGroup) {
	s.Groups = groups
}

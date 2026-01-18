// Package grouptypes provides shared types for instance grouping across packages.
// This package exists to break circular dependencies between orchestrator, group,
// and session packages that all need to work with instance groups.
package grouptypes

import (
	"slices"
	"time"
)

// GroupPhase represents the current phase of an instance group.
type GroupPhase string

const (
	GroupPhasePending   GroupPhase = "pending"
	GroupPhaseExecuting GroupPhase = "executing"
	GroupPhaseCompleted GroupPhase = "completed"
	GroupPhaseFailed    GroupPhase = "failed"
)

// InstanceGroup represents a visual grouping of instances in the TUI.
// Groups enable users to organize related tasks together, particularly useful
// for Plan and UltraPlan workflows where tasks have natural dependencies.
type InstanceGroup struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`            // e.g., "Group 1: Foundation" or auto-generated
	Phase          GroupPhase       `json:"phase"`           // Current group status
	Instances      []string         `json:"instances"`       // Instance IDs in this group
	SubGroups      []*InstanceGroup `json:"sub_groups"`      // For nested dependencies
	ParentID       string           `json:"parent_id"`       // If this is a sub-group
	ExecutionOrder int              `json:"execution_order"` // Order of execution (0 = first)
	DependsOn      []string         `json:"depends_on"`      // Group IDs this group depends on
	Created        time.Time        `json:"created"`

	// SessionType identifies the type of session that created this group.
	// Used for displaying appropriate icons and determining grouping behavior.
	// This is stored as a string to avoid importing the orchestrator package.
	SessionType string `json:"session_type,omitempty"`

	// Objective is the original user prompt/objective for this group.
	// Used for LLM-based name generation and display purposes.
	Objective string `json:"objective,omitempty"`
}

// NewInstanceGroup creates a new instance group with the given ID and name.
// The caller is responsible for generating a unique ID.
func NewInstanceGroup(id, name string) *InstanceGroup {
	return &InstanceGroup{
		ID:        id,
		Name:      name,
		Phase:     GroupPhasePending,
		Instances: make([]string, 0),
		SubGroups: make([]*InstanceGroup, 0),
		DependsOn: make([]string, 0),
		Created:   time.Now(),
	}
}

// NewInstanceGroupWithType creates a new instance group with a session type and objective.
// The objective is used for LLM-based name generation.
func NewInstanceGroupWithType(id, name, sessionType, objective string) *InstanceGroup {
	return &InstanceGroup{
		ID:          id,
		Name:        name,
		Phase:       GroupPhasePending,
		Instances:   make([]string, 0),
		SubGroups:   make([]*InstanceGroup, 0),
		DependsOn:   make([]string, 0),
		Created:     time.Now(),
		SessionType: sessionType,
		Objective:   objective,
	}
}

// AddInstance adds an instance ID to the group.
func (g *InstanceGroup) AddInstance(instanceID string) {
	g.Instances = append(g.Instances, instanceID)
}

// RemoveInstance removes an instance ID from the group.
func (g *InstanceGroup) RemoveInstance(instanceID string) {
	for i, id := range g.Instances {
		if id == instanceID {
			g.Instances = append(g.Instances[:i], g.Instances[i+1:]...)
			return
		}
	}
}

// HasInstance checks if an instance ID is in this group.
func (g *InstanceGroup) HasInstance(instanceID string) bool {
	return slices.Contains(g.Instances, instanceID)
}

// AddSubGroup adds a sub-group to this group.
func (g *InstanceGroup) AddSubGroup(subGroup *InstanceGroup) {
	subGroup.ParentID = g.ID
	g.SubGroups = append(g.SubGroups, subGroup)
}

// GetSubGroup returns a sub-group by ID.
func (g *InstanceGroup) GetSubGroup(id string) *InstanceGroup {
	for _, sg := range g.SubGroups {
		if sg.ID == id {
			return sg
		}
	}
	return nil
}

// AllInstanceIDs returns all instance IDs in this group and all sub-groups (recursively).
func (g *InstanceGroup) AllInstanceIDs() []string {
	ids := make([]string, len(g.Instances))
	copy(ids, g.Instances)

	for _, sg := range g.SubGroups {
		ids = append(ids, sg.AllInstanceIDs()...)
	}
	return ids
}

// InstanceCount returns the total number of instances in this group and all sub-groups.
func (g *InstanceGroup) InstanceCount() int {
	count := len(g.Instances)
	for _, sg := range g.SubGroups {
		count += sg.InstanceCount()
	}
	return count
}

// IsEmpty returns true if this group has no instances and no sub-groups with instances.
func (g *InstanceGroup) IsEmpty() bool {
	return g.InstanceCount() == 0
}

// IsTopLevel returns true if this group has no parent (is not a sub-group).
func (g *InstanceGroup) IsTopLevel() bool {
	return g.ParentID == ""
}

// FindGroup searches for a group by ID within this group and its sub-groups.
func (g *InstanceGroup) FindGroup(id string) *InstanceGroup {
	if g.ID == id {
		return g
	}
	for _, sg := range g.SubGroups {
		if found := sg.FindGroup(id); found != nil {
			return found
		}
	}
	return nil
}

// FindGroupContainingInstance returns the group (or sub-group) containing the given instance ID.
func (g *InstanceGroup) FindGroupContainingInstance(instanceID string) *InstanceGroup {
	if g.HasInstance(instanceID) {
		return g
	}
	for _, sg := range g.SubGroups {
		if found := sg.FindGroupContainingInstance(instanceID); found != nil {
			return found
		}
	}
	return nil
}

// Clone creates a deep copy of the instance group.
func (g *InstanceGroup) Clone() *InstanceGroup {
	if g == nil {
		return nil
	}
	clone := &InstanceGroup{
		ID:             g.ID,
		Name:           g.Name,
		Phase:          g.Phase,
		Instances:      make([]string, len(g.Instances)),
		ParentID:       g.ParentID,
		ExecutionOrder: g.ExecutionOrder,
		DependsOn:      make([]string, len(g.DependsOn)),
		Created:        g.Created,
		SessionType:    g.SessionType,
		Objective:      g.Objective,
	}
	copy(clone.Instances, g.Instances)
	copy(clone.DependsOn, g.DependsOn)

	clone.SubGroups = make([]*InstanceGroup, len(g.SubGroups))
	for i, sg := range g.SubGroups {
		clone.SubGroups[i] = sg.Clone()
	}
	return clone
}

package orchestrator

import "time"

// GroupPhase represents the current phase of an instance group
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
}

// NewInstanceGroup creates a new instance group with a generated ID
func NewInstanceGroup(name string) *InstanceGroup {
	return &InstanceGroup{
		ID:        generateID(),
		Name:      name,
		Phase:     GroupPhasePending,
		Instances: make([]string, 0),
		SubGroups: make([]*InstanceGroup, 0),
		DependsOn: make([]string, 0),
		Created:   time.Now(),
	}
}

// AddInstance adds an instance ID to the group
func (g *InstanceGroup) AddInstance(instanceID string) {
	g.Instances = append(g.Instances, instanceID)
}

// RemoveInstance removes an instance ID from the group
func (g *InstanceGroup) RemoveInstance(instanceID string) {
	for i, id := range g.Instances {
		if id == instanceID {
			g.Instances = append(g.Instances[:i], g.Instances[i+1:]...)
			return
		}
	}
}

// HasInstance checks if an instance ID is in this group
func (g *InstanceGroup) HasInstance(instanceID string) bool {
	for _, id := range g.Instances {
		if id == instanceID {
			return true
		}
	}
	return false
}

// AddSubGroup adds a sub-group to this group
func (g *InstanceGroup) AddSubGroup(subGroup *InstanceGroup) {
	subGroup.ParentID = g.ID
	g.SubGroups = append(g.SubGroups, subGroup)
}

// GetSubGroup returns a sub-group by ID
func (g *InstanceGroup) GetSubGroup(id string) *InstanceGroup {
	for _, sg := range g.SubGroups {
		if sg.ID == id {
			return sg
		}
	}
	return nil
}

// AllInstanceIDs returns all instance IDs in this group and all sub-groups (recursively)
func (g *InstanceGroup) AllInstanceIDs() []string {
	ids := make([]string, len(g.Instances))
	copy(ids, g.Instances)

	for _, sg := range g.SubGroups {
		ids = append(ids, sg.AllInstanceIDs()...)
	}
	return ids
}

// InstanceCount returns the total number of instances in this group and all sub-groups
func (g *InstanceGroup) InstanceCount() int {
	count := len(g.Instances)
	for _, sg := range g.SubGroups {
		count += sg.InstanceCount()
	}
	return count
}

// IsTopLevel returns true if this group has no parent (is not a sub-group)
func (g *InstanceGroup) IsTopLevel() bool {
	return g.ParentID == ""
}

// GetGroup finds a group by ID within the session's groups (including sub-groups)
func (s *Session) GetGroup(id string) *InstanceGroup {
	for _, g := range s.Groups {
		if g.ID == id {
			return g
		}
		// Search sub-groups recursively
		if found := findGroupRecursive(g, id); found != nil {
			return found
		}
	}
	return nil
}

// findGroupRecursive searches for a group by ID in sub-groups
func findGroupRecursive(group *InstanceGroup, id string) *InstanceGroup {
	for _, sg := range group.SubGroups {
		if sg.ID == id {
			return sg
		}
		if found := findGroupRecursive(sg, id); found != nil {
			return found
		}
	}
	return nil
}

// GetGroupForInstance finds the group (or sub-group) containing the given instance ID
func (s *Session) GetGroupForInstance(instanceID string) *InstanceGroup {
	for _, g := range s.Groups {
		if found := findGroupContainingInstance(g, instanceID); found != nil {
			return found
		}
	}
	return nil
}

// findGroupContainingInstance recursively searches for the group containing an instance
func findGroupContainingInstance(group *InstanceGroup, instanceID string) *InstanceGroup {
	if group.HasInstance(instanceID) {
		return group
	}
	for _, sg := range group.SubGroups {
		if found := findGroupContainingInstance(sg, instanceID); found != nil {
			return found
		}
	}
	return nil
}

// AddGroup adds a new group to the session
func (s *Session) AddGroup(group *InstanceGroup) {
	if s.Groups == nil {
		s.Groups = make([]*InstanceGroup, 0)
	}
	s.Groups = append(s.Groups, group)
}

// RemoveGroup removes a group from the session by ID
func (s *Session) RemoveGroup(id string) {
	for i, g := range s.Groups {
		if g.ID == id {
			s.Groups = append(s.Groups[:i], s.Groups[i+1:]...)
			return
		}
	}
}

// GetGroupsByPhase returns all groups (top-level only) in the given phase
func (s *Session) GetGroupsByPhase(phase GroupPhase) []*InstanceGroup {
	var groups []*InstanceGroup
	for _, g := range s.Groups {
		if g.Phase == phase {
			groups = append(groups, g)
		}
	}
	return groups
}

// AreGroupDependenciesMet checks if all dependencies for a group have completed
func (s *Session) AreGroupDependenciesMet(group *InstanceGroup) bool {
	if len(group.DependsOn) == 0 {
		return true
	}

	for _, depID := range group.DependsOn {
		dep := s.GetGroup(depID)
		if dep == nil {
			return false
		}
		if dep.Phase != GroupPhaseCompleted {
			return false
		}
	}
	return true
}

// GetReadyGroups returns all groups that are pending and have their dependencies met
func (s *Session) GetReadyGroups() []*InstanceGroup {
	var ready []*InstanceGroup
	for _, g := range s.Groups {
		if g.Phase == GroupPhasePending && s.AreGroupDependenciesMet(g) {
			ready = append(ready, g)
		}
	}
	return ready
}

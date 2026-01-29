package orchestrator

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator/grouptypes"
)

// GroupPhase represents the current phase of an instance group.
// This is a type alias to the canonical definition in grouptypes.
type GroupPhase = grouptypes.GroupPhase

// Phase constants re-exported from grouptypes for backwards compatibility.
const (
	GroupPhasePending   = grouptypes.GroupPhasePending
	GroupPhaseExecuting = grouptypes.GroupPhaseExecuting
	GroupPhaseCompleted = grouptypes.GroupPhaseCompleted
	GroupPhaseFailed    = grouptypes.GroupPhaseFailed
)

// InstanceGroup represents a visual grouping of instances in the TUI.
// This is a type alias to the canonical definition in grouptypes.
// The SessionType field is stored as a string; use GetSessionType() for typed access.
type InstanceGroup = grouptypes.InstanceGroup

// NewInstanceGroup creates a new instance group with a generated ID.
func NewInstanceGroup(name string) *InstanceGroup {
	return grouptypes.NewInstanceGroup(generateID(), name)
}

// NewInstanceGroupWithID creates a new instance group with a specified ID.
// Use this when you need a predictable ID (e.g., for round sub-groups).
func NewInstanceGroupWithID(id, name string) *InstanceGroup {
	return grouptypes.NewInstanceGroup(id, name)
}

// NewInstanceGroupWithType creates a new instance group with a session type and objective.
// The objective is used for LLM-based name generation.
func NewInstanceGroupWithType(name string, sessionType SessionType, objective string) *InstanceGroup {
	return grouptypes.NewInstanceGroupWithType(generateID(), name, string(sessionType), objective)
}

// GetSessionType returns the typed SessionType for the group.
// Use this instead of accessing g.SessionType directly when you need typed access.
func GetSessionType(g *InstanceGroup) SessionType {
	return SessionType(g.SessionType)
}

// SetSessionType sets the SessionType on the group.
// Use this instead of assigning g.SessionType directly for type safety.
func SetSessionType(g *InstanceGroup, st SessionType) {
	g.SessionType = string(st)
}

// GetGroups returns a snapshot copy of the session's groups slice.
// The returned slice can be safely iterated without holding any locks.
// This method is thread-safe.
func (s *Session) GetGroups() []*InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	if s.Groups == nil {
		return nil
	}
	// Return a copy to allow safe iteration without holding the lock
	result := make([]*InstanceGroup, len(s.Groups))
	copy(result, s.Groups)
	return result
}

// GroupCount returns the number of top-level groups.
// This method is thread-safe.
func (s *Session) GroupCount() int {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	return len(s.Groups)
}

// HasGroups returns true if there are any groups in the session.
// This method is thread-safe.
func (s *Session) HasGroups() bool {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	return len(s.Groups) > 0
}

// SetGroups replaces the session's groups with the given slice.
// This method is thread-safe.
func (s *Session) SetGroups(groups []*InstanceGroup) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	s.Groups = groups
}

// GetGroup finds a group by ID within the session's groups (including sub-groups).
// This method is thread-safe.
func (s *Session) GetGroup(id string) *InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	return s.getGroupLocked(id)
}

// getGroupLocked finds a group by ID without acquiring the lock.
// Caller must hold s.groupsMu (read or write lock).
func (s *Session) getGroupLocked(id string) *InstanceGroup {
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

// GetGroupForInstance finds the group (or sub-group) containing the given instance ID.
// This method is thread-safe.
func (s *Session) GetGroupForInstance(instanceID string) *InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
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

// AddGroup adds a new group to the session.
// This method is thread-safe.
func (s *Session) AddGroup(group *InstanceGroup) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	if s.Groups == nil {
		s.Groups = make([]*InstanceGroup, 0)
	}
	s.Groups = append(s.Groups, group)
}

// RemoveGroup removes a group from the session by ID.
// This method is thread-safe.
func (s *Session) RemoveGroup(id string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	for i, g := range s.Groups {
		if g.ID == id {
			s.Groups = append(s.Groups[:i], s.Groups[i+1:]...)
			return
		}
	}
}

// GetGroupsByPhase returns all groups (top-level only) in the given phase.
// This method is thread-safe.
func (s *Session) GetGroupsByPhase(phase GroupPhase) []*InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	var groups []*InstanceGroup
	for _, g := range s.Groups {
		if g.Phase == phase {
			groups = append(groups, g)
		}
	}
	return groups
}

// AreGroupDependenciesMet checks if all dependencies for a group have completed.
// This method is thread-safe.
func (s *Session) AreGroupDependenciesMet(group *InstanceGroup) bool {
	if len(group.DependsOn) == 0 {
		return true
	}

	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	for _, depID := range group.DependsOn {
		dep := s.getGroupLocked(depID)
		if dep == nil {
			return false
		}
		if dep.Phase != GroupPhaseCompleted {
			return false
		}
	}
	return true
}

// GetReadyGroups returns all groups that are pending and have their dependencies met.
// This method is thread-safe.
func (s *Session) GetReadyGroups() []*InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	var ready []*InstanceGroup
	for _, g := range s.Groups {
		if g.Phase == GroupPhasePending && s.areGroupDependenciesMetLocked(g) {
			ready = append(ready, g)
		}
	}
	return ready
}

// areGroupDependenciesMetLocked checks if all dependencies for a group have completed.
// Caller must hold s.groupsMu (read or write lock).
func (s *Session) areGroupDependenciesMetLocked(group *InstanceGroup) bool {
	if len(group.DependsOn) == 0 {
		return true
	}

	for _, depID := range group.DependsOn {
		dep := s.getGroupLocked(depID)
		if dep == nil {
			return false
		}
		if dep.Phase != GroupPhaseCompleted {
			return false
		}
	}
	return true
}

// GetGroupBySessionType returns the first group with the given session type.
// Useful for finding shared groups like "Plans".
// This method is thread-safe.
func (s *Session) GetGroupBySessionType(sessionType SessionType) *InstanceGroup {
	s.groupsMu.RLock()
	defer s.groupsMu.RUnlock()
	for _, g := range s.Groups {
		if GetSessionType(g) == sessionType {
			return g
		}
	}
	return nil
}

// GetOrCreateSharedGroup returns an existing shared group for the session type,
// or creates a new one if none exists. Only meaningful for session types with
// GroupingMode() == "shared".
// This method is thread-safe.
func (s *Session) GetOrCreateSharedGroup(sessionType SessionType) *InstanceGroup {
	if sessionType.GroupingMode() != "shared" {
		return nil
	}

	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()

	// Look for existing group
	for _, g := range s.Groups {
		if GetSessionType(g) == sessionType {
			return g
		}
	}

	// Create new shared group
	group := NewInstanceGroupWithType(sessionType.SharedGroupName(), sessionType, "")
	if s.Groups == nil {
		s.Groups = make([]*InstanceGroup, 0)
	}
	s.Groups = append(s.Groups, group)
	return group
}

// RemoveInstanceFromGroups removes an instance from all groups and sub-groups.
// After removal, any empty groups are automatically cleaned up.
// This method is thread-safe.
func (s *Session) RemoveInstanceFromGroups(instanceID string) {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()

	// First, remove the instance from all groups
	for _, g := range s.Groups {
		removeInstanceFromGroupRecursive(g, instanceID)
	}

	// Then clean up any empty groups
	s.cleanupEmptyGroupsLocked()
}

// removeInstanceFromGroupRecursive removes an instance from a group and its sub-groups.
func removeInstanceFromGroupRecursive(group *InstanceGroup, instanceID string) {
	group.RemoveInstance(instanceID)
	for _, sg := range group.SubGroups {
		removeInstanceFromGroupRecursive(sg, instanceID)
	}
}

// CleanupEmptyGroups removes all empty groups (top-level and sub-groups).
// This method is thread-safe.
func (s *Session) CleanupEmptyGroups() {
	s.groupsMu.Lock()
	defer s.groupsMu.Unlock()
	s.cleanupEmptyGroupsLocked()
}

// cleanupEmptyGroupsLocked removes all empty groups without acquiring the lock.
// Caller must hold s.groupsMu (write lock).
func (s *Session) cleanupEmptyGroupsLocked() {
	// First, clean up empty sub-groups within each top-level group
	for _, g := range s.Groups {
		cleanupEmptySubGroups(g)
	}

	// Then, remove empty top-level groups
	var nonEmptyGroups []*InstanceGroup
	for _, g := range s.Groups {
		if !g.IsEmpty() {
			nonEmptyGroups = append(nonEmptyGroups, g)
		}
	}

	s.Groups = nonEmptyGroups
}

// cleanupEmptySubGroups recursively removes empty sub-groups from a parent group.
func cleanupEmptySubGroups(parent *InstanceGroup) {
	// First, recurse into sub-groups to clean up their sub-groups
	for _, sg := range parent.SubGroups {
		cleanupEmptySubGroups(sg)
	}

	// Then, remove empty sub-groups from this parent
	var nonEmptySubGroups []*InstanceGroup
	for _, sg := range parent.SubGroups {
		if !sg.IsEmpty() {
			nonEmptySubGroups = append(nonEmptySubGroups, sg)
		}
	}

	parent.SubGroups = nonEmptySubGroups
}

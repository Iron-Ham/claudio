// Package group provides execution group tracking and management for orchestration.
package group

import (
	"slices"
	"sync"

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

// InstanceGroup represents a visual grouping of instances.
// This is a type alias to the canonical definition in grouptypes.
type InstanceGroup = grouptypes.InstanceGroup

// ManagerSessionData provides the session data interface needed by Manager.
// This is a minimal interface to avoid coupling with the full Session type.
type ManagerSessionData interface {
	// GetGroups returns the current list of top-level groups.
	GetGroups() []*InstanceGroup

	// SetGroups replaces the current list of top-level groups.
	SetGroups(groups []*InstanceGroup)

	// GenerateID generates a unique ID for a new group.
	GenerateID() string
}

// Manager handles active manipulation of instance groups within a session.
// It provides methods for creating, modifying, and organizing groups,
// complementing the read-only Tracker which is focused on execution state queries.
//
// Manager is safe for concurrent use.
type Manager struct {
	session ManagerSessionData
	mu      sync.RWMutex
}

// NewManager creates a new group manager for the given session.
// The session must implement ManagerSessionData and must not be nil.
func NewManager(session ManagerSessionData) *Manager {
	if session == nil {
		panic("group.NewManager: session must not be nil")
	}
	return &Manager{
		session: session,
	}
}

// CreateGroup creates a new top-level group with the given name and instance IDs.
// The group is appended to the end of the session's group list with an appropriate
// execution order. Returns the newly created group.
func (m *Manager) CreateGroup(name string, instanceIDs []string) *InstanceGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	groups := m.session.GetGroups()

	// Determine execution order (next available)
	executionOrder := 0
	for _, g := range groups {
		if g.ExecutionOrder >= executionOrder {
			executionOrder = g.ExecutionOrder + 1
		}
	}

	group := grouptypes.NewInstanceGroup(m.session.GenerateID(), name)
	group.ExecutionOrder = executionOrder
	group.Instances = make([]string, len(instanceIDs))
	copy(group.Instances, instanceIDs)

	groups = append(groups, group)
	m.session.SetGroups(groups)

	return group
}

// CreateSubGroup creates a new sub-group within the specified parent group.
// Returns the newly created sub-group, or nil if the parent group is not found.
func (m *Manager) CreateSubGroup(parentID, name string, instanceIDs []string) *InstanceGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	parent := m.findGroupByID(parentID)
	if parent == nil {
		return nil
	}

	// Determine execution order for sub-group
	executionOrder := 0
	for _, sg := range parent.SubGroups {
		if sg.ExecutionOrder >= executionOrder {
			executionOrder = sg.ExecutionOrder + 1
		}
	}

	subGroup := grouptypes.NewInstanceGroup(m.session.GenerateID(), name)
	subGroup.ParentID = parentID
	subGroup.ExecutionOrder = executionOrder
	subGroup.Instances = make([]string, len(instanceIDs))
	copy(subGroup.Instances, instanceIDs)

	parent.SubGroups = append(parent.SubGroups, subGroup)

	return subGroup
}

// MoveInstanceToGroup moves an instance from its current group to a new group.
// If the instance is not currently in any group, it is simply added to the target group.
// If the target group is not found, this method does nothing.
func (m *Manager) MoveInstanceToGroup(instanceID, groupID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	targetGroup := m.findGroupByID(groupID)
	if targetGroup == nil {
		return
	}

	// Remove from current group (if any)
	m.removeInstanceFromAllGroups(instanceID)

	// Add to target group
	targetGroup.Instances = append(targetGroup.Instances, instanceID)
}

// GetGroupForInstance returns the group containing the specified instance ID.
// Returns nil if the instance is not found in any group.
func (m *Manager) GetGroupForInstance(instanceID string) *InstanceGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.findGroupContainingInstance(instanceID)
}

// AdvanceGroupPhase updates the phase of the specified group.
// Does nothing if the group is not found.
func (m *Manager) AdvanceGroupPhase(groupID string, phase GroupPhase) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group := m.findGroupByID(groupID)
	if group == nil {
		return
	}

	group.Phase = phase
}

// ReorderGroups reorders the top-level groups according to the provided order.
// The newOrder slice should contain group IDs in the desired order.
// Groups not in newOrder are appended at the end in their original order.
// Non-existent group IDs in newOrder are ignored.
func (m *Manager) ReorderGroups(newOrder []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	groups := m.session.GetGroups()
	if len(groups) == 0 {
		return
	}

	// Build a map of group ID to group for quick lookup
	groupMap := make(map[string]*InstanceGroup)
	for _, g := range groups {
		groupMap[g.ID] = g
	}

	// Build the reordered list
	reordered := make([]*InstanceGroup, 0, len(groups))
	seen := make(map[string]bool)

	// First, add groups in the specified order
	for _, id := range newOrder {
		if g, exists := groupMap[id]; exists && !seen[id] {
			reordered = append(reordered, g)
			seen[id] = true
		}
	}

	// Then, add any remaining groups that weren't in newOrder
	for _, g := range groups {
		if !seen[g.ID] {
			reordered = append(reordered, g)
		}
	}

	// Update execution orders to match new positions
	for i, g := range reordered {
		g.ExecutionOrder = i
	}

	m.session.SetGroups(reordered)
}

// FlattenGroups returns all instance IDs in execution order.
// Groups are processed in execution order, and within each group,
// instances are returned in their stored order. Sub-groups are processed
// immediately after their parent group's direct instances.
func (m *Manager) FlattenGroups() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := m.session.GetGroups()
	if len(groups) == 0 {
		return nil
	}

	// Sort groups by execution order
	sortedGroups := m.sortGroupsByExecutionOrder(groups)

	var result []string
	for _, g := range sortedGroups {
		result = append(result, m.flattenGroup(g)...)
	}

	return result
}

// GetGroup returns the group with the specified ID, or nil if not found.
// This searches both top-level groups and sub-groups.
func (m *Manager) GetGroup(id string) *InstanceGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.findGroupByID(id)
}

// GetAllGroups returns all top-level groups.
func (m *Manager) GetAllGroups() []*InstanceGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := m.session.GetGroups()
	result := make([]*InstanceGroup, len(groups))
	copy(result, groups)
	return result
}

// RemoveGroup removes a group by ID. If the group has sub-groups,
// they are also removed. Instances in the group are not deleted,
// they simply become ungrouped.
func (m *Manager) RemoveGroup(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	groups := m.session.GetGroups()

	// Check top-level groups
	for i, g := range groups {
		if g.ID == id {
			groups = append(groups[:i], groups[i+1:]...)
			m.session.SetGroups(groups)
			return true
		}

		// Check sub-groups
		if m.removeSubGroup(g, id) {
			return true
		}
	}

	return false
}

// SetGroupDependencies sets the dependencies for a group.
// dependsOn is a list of group IDs that must complete before this group can execute.
func (m *Manager) SetGroupDependencies(groupID string, dependsOn []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group := m.findGroupByID(groupID)
	if group == nil {
		return
	}

	group.DependsOn = make([]string, len(dependsOn))
	copy(group.DependsOn, dependsOn)
}

// findGroupByID searches for a group by ID in all groups and sub-groups.
// Must be called with lock held.
func (m *Manager) findGroupByID(id string) *InstanceGroup {
	groups := m.session.GetGroups()
	for _, g := range groups {
		if g.ID == id {
			return g
		}
		if found := m.findGroupByIDRecursive(g, id); found != nil {
			return found
		}
	}
	return nil
}

// findGroupByIDRecursive recursively searches sub-groups for a group with the given ID.
func (m *Manager) findGroupByIDRecursive(group *InstanceGroup, id string) *InstanceGroup {
	for _, sg := range group.SubGroups {
		if sg.ID == id {
			return sg
		}
		if found := m.findGroupByIDRecursive(sg, id); found != nil {
			return found
		}
	}
	return nil
}

// findGroupContainingInstance searches for the group containing the given instance ID.
// Must be called with lock held.
func (m *Manager) findGroupContainingInstance(instanceID string) *InstanceGroup {
	groups := m.session.GetGroups()
	for _, g := range groups {
		if found := m.findGroupContainingInstanceRecursive(g, instanceID); found != nil {
			return found
		}
	}
	return nil
}

// findGroupContainingInstanceRecursive recursively searches for a group containing an instance.
func (m *Manager) findGroupContainingInstanceRecursive(group *InstanceGroup, instanceID string) *InstanceGroup {
	if slices.Contains(group.Instances, instanceID) {
		return group
	}
	for _, sg := range group.SubGroups {
		if found := m.findGroupContainingInstanceRecursive(sg, instanceID); found != nil {
			return found
		}
	}
	return nil
}

// removeInstanceFromAllGroups removes an instance from all groups and sub-groups.
// Must be called with lock held.
func (m *Manager) removeInstanceFromAllGroups(instanceID string) {
	groups := m.session.GetGroups()
	for _, g := range groups {
		m.removeInstanceFromGroupRecursive(g, instanceID)
	}
}

// removeInstanceFromGroupRecursive removes an instance from a group and its sub-groups.
func (m *Manager) removeInstanceFromGroupRecursive(group *InstanceGroup, instanceID string) {
	// Remove from this group's instances
	for i, id := range group.Instances {
		if id == instanceID {
			group.Instances = append(group.Instances[:i], group.Instances[i+1:]...)
			break
		}
	}

	// Remove from sub-groups
	for _, sg := range group.SubGroups {
		m.removeInstanceFromGroupRecursive(sg, instanceID)
	}
}

// removeSubGroup removes a sub-group from a parent group by ID.
// Returns true if the sub-group was found and removed.
func (m *Manager) removeSubGroup(parent *InstanceGroup, id string) bool {
	for i, sg := range parent.SubGroups {
		if sg.ID == id {
			parent.SubGroups = append(parent.SubGroups[:i], parent.SubGroups[i+1:]...)
			return true
		}
		if m.removeSubGroup(sg, id) {
			return true
		}
	}
	return false
}

// sortGroupsByExecutionOrder returns a copy of groups sorted by ExecutionOrder.
func (m *Manager) sortGroupsByExecutionOrder(groups []*InstanceGroup) []*InstanceGroup {
	sorted := make([]*InstanceGroup, len(groups))
	copy(sorted, groups)

	// Simple insertion sort (groups list is typically small)
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j].ExecutionOrder > key.ExecutionOrder {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}

	return sorted
}

// flattenGroup returns all instance IDs from a group and its sub-groups in order.
func (m *Manager) flattenGroup(group *InstanceGroup) []string {
	var result []string

	// First, add this group's direct instances
	result = append(result, group.Instances...)

	// Then, add sub-groups' instances in execution order
	sortedSubGroups := m.sortGroupsByExecutionOrder(group.SubGroups)
	for _, sg := range sortedSubGroups {
		result = append(result, m.flattenGroup(sg)...)
	}

	return result
}

// instanceCount returns the total number of instances in a group and all sub-groups.
func (m *Manager) instanceCount(group *InstanceGroup) int {
	count := len(group.Instances)
	for _, sg := range group.SubGroups {
		count += m.instanceCount(sg)
	}
	return count
}

// isEmpty returns true if a group has no instances and no sub-groups with instances.
func (m *Manager) isEmpty(group *InstanceGroup) bool {
	return m.instanceCount(group) == 0
}

// RemoveInstanceFromGroup removes an instance from its current group.
// If the removal causes the group (or any parent groups) to become empty,
// those empty groups are automatically removed.
// Returns true if the instance was found and removed from a group.
func (m *Manager) RemoveInstanceFromGroup(instanceID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find the group containing this instance
	group := m.findGroupContainingInstance(instanceID)
	if group == nil {
		return false
	}

	// Remove the instance from the group
	m.removeInstanceFromGroupRecursive(group, instanceID)

	// Clean up empty groups
	m.cleanupEmptyGroups()

	return true
}

// cleanupEmptyGroups removes all empty groups (top-level and sub-groups).
// Must be called with the lock held.
func (m *Manager) cleanupEmptyGroups() {
	groups := m.session.GetGroups()

	// First, clean up empty sub-groups within each top-level group
	for _, g := range groups {
		m.cleanupEmptySubGroups(g)
	}

	// Then, remove empty top-level groups
	var nonEmptyGroups []*InstanceGroup
	for _, g := range groups {
		if !m.isEmpty(g) {
			nonEmptyGroups = append(nonEmptyGroups, g)
		}
	}

	// Only update if we removed any groups
	if len(nonEmptyGroups) != len(groups) {
		m.session.SetGroups(nonEmptyGroups)
	}
}

// cleanupEmptySubGroups recursively removes empty sub-groups from a parent group.
func (m *Manager) cleanupEmptySubGroups(parent *InstanceGroup) {
	// First, recurse into sub-groups to clean up their sub-groups
	for _, sg := range parent.SubGroups {
		m.cleanupEmptySubGroups(sg)
	}

	// Then, remove empty sub-groups from this parent
	var nonEmptySubGroups []*InstanceGroup
	for _, sg := range parent.SubGroups {
		if !m.isEmpty(sg) {
			nonEmptySubGroups = append(nonEmptySubGroups, sg)
		}
	}

	parent.SubGroups = nonEmptySubGroups
}

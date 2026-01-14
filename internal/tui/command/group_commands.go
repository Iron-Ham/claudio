// Package command provides command handling for the TUI.
package command

import (
	"fmt"
	"strings"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
)

// GroupDependencies extends Dependencies with group management access.
// This is a minimal interface to support group commands.
type GroupDependencies interface {
	Dependencies

	// GetGroupManager returns the group manager for the session, or nil if unavailable.
	GetGroupManager() *group.Manager

	// IsGroupedViewEnabled returns whether grouped instance view is currently enabled.
	IsGroupedViewEnabled() bool
}

// executeGroupCommand parses and executes a group subcommand.
// Command format: group <subcommand> [args...]
// Subcommands:
//   - create [name]           - create a new empty group
//   - add [instance] [group]  - add instance to group
//   - remove [instance]       - remove instance from its group
//   - move [instance] [group] - move instance between groups
//   - order [g1,g2,g3]        - reorder execution sequence
//   - delete [name]           - delete empty group
//   - show                    - toggle grouped view on/off
func executeGroupCommand(args string, deps Dependencies) Result {
	// Try to get group-specific dependencies
	groupDeps, ok := deps.(GroupDependencies)
	if !ok {
		return Result{ErrorMessage: "Group commands not available in this context"}
	}

	args = strings.TrimSpace(args)
	if args == "" {
		return groupUsage()
	}

	// Parse subcommand and arguments
	parts := strings.SplitN(args, " ", 2)
	subCmd := strings.ToLower(parts[0])
	subArgs := ""
	if len(parts) > 1 {
		subArgs = strings.TrimSpace(parts[1])
	}

	switch subCmd {
	case "create":
		return cmdGroupCreate(subArgs, groupDeps)
	case "add":
		return cmdGroupAdd(subArgs, groupDeps)
	case "remove":
		return cmdGroupRemove(subArgs, groupDeps)
	case "move":
		return cmdGroupMove(subArgs, groupDeps)
	case "order":
		return cmdGroupOrder(subArgs, groupDeps)
	case "delete":
		return cmdGroupDelete(subArgs, groupDeps)
	case "show":
		return cmdGroupShow(groupDeps)
	case "help", "?":
		return groupUsage()
	default:
		return Result{ErrorMessage: fmt.Sprintf("Unknown group subcommand: %s. Type :group help for usage.", subCmd)}
	}
}

// groupUsage returns help text for the group command.
func groupUsage() Result {
	return Result{
		InfoMessage: `Group commands:
  :group create [name]           - Create a new empty group
  :group add [instance] [group]  - Add instance to a group
  :group remove [instance]       - Remove instance from its group
  :group move [instance] [group] - Move instance to a different group
  :group order [g1,g2,g3]        - Reorder group execution sequence
  :group delete [name]           - Delete an empty group
  :group show                    - Toggle grouped view on/off`,
	}
}

// cmdGroupCreate handles ":group create [name]"
// Creates a new empty group with the given name (or auto-generated if not provided).
func cmdGroupCreate(args string, deps GroupDependencies) Result {
	mgr := deps.GetGroupManager()
	if mgr == nil {
		return Result{ErrorMessage: "Group manager not available"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	name := strings.TrimSpace(args)
	if name == "" {
		// Auto-generate a name based on existing group count
		groups := session.Groups
		name = fmt.Sprintf("Group %d", len(groups)+1)
	}

	// Create the group (no instances initially)
	grp := mgr.CreateGroup(name, nil)
	if grp == nil {
		return Result{ErrorMessage: "Failed to create group"}
	}

	return Result{InfoMessage: fmt.Sprintf("Created group %q (ID: %s)", grp.Name, grp.ID)}
}

// cmdGroupAdd handles ":group add [instance] [group]"
// Adds an instance to a group. Instance can be ID or index, group can be ID or name.
func cmdGroupAdd(args string, deps GroupDependencies) Result {
	mgr := deps.GetGroupManager()
	if mgr == nil {
		return Result{ErrorMessage: "Group manager not available"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		return Result{ErrorMessage: "Usage: :group add [instance] [group]"}
	}

	instanceRef := parts[0]
	groupRef := strings.Join(parts[1:], " ") // Allow group names with spaces

	// Resolve instance
	inst := resolveInstance(instanceRef, session)
	if inst == nil {
		return Result{ErrorMessage: fmt.Sprintf("Instance not found: %s", instanceRef)}
	}

	// Resolve group
	grp := resolveGroup(groupRef, session)
	if grp == nil {
		return Result{ErrorMessage: fmt.Sprintf("Group not found: %s", groupRef)}
	}

	// Add instance to group
	mgr.MoveInstanceToGroup(inst.ID, grp.ID)

	return Result{InfoMessage: fmt.Sprintf("Added instance %q to group %q", inst.EffectiveName(), grp.Name)}
}

// cmdGroupRemove handles ":group remove [instance]"
// Removes an instance from its current group (instance becomes ungrouped).
func cmdGroupRemove(args string, deps GroupDependencies) Result {
	mgr := deps.GetGroupManager()
	if mgr == nil {
		return Result{ErrorMessage: "Group manager not available"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	instanceRef := strings.TrimSpace(args)
	if instanceRef == "" {
		// Use active instance if no argument
		inst := deps.ActiveInstance()
		if inst == nil {
			return Result{ErrorMessage: "Usage: :group remove [instance] or select an instance"}
		}
		instanceRef = inst.ID
	}

	// Resolve instance
	inst := resolveInstance(instanceRef, session)
	if inst == nil {
		return Result{ErrorMessage: fmt.Sprintf("Instance not found: %s", instanceRef)}
	}

	// Check if instance is in a group
	currentGroup := mgr.GetGroupForInstance(inst.ID)
	if currentGroup == nil {
		return Result{InfoMessage: fmt.Sprintf("Instance %q is not in any group", inst.EffectiveName())}
	}

	groupName := currentGroup.Name

	// Remove instance from group (this also cleans up empty groups automatically)
	mgr.RemoveInstanceFromGroup(inst.ID)

	return Result{InfoMessage: fmt.Sprintf("Removed instance %q from group %q", inst.EffectiveName(), groupName)}
}

// cmdGroupMove handles ":group move [instance] [group]"
// Moves an instance from its current group to a different group.
func cmdGroupMove(args string, deps GroupDependencies) Result {
	mgr := deps.GetGroupManager()
	if mgr == nil {
		return Result{ErrorMessage: "Group manager not available"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	parts := strings.Fields(args)
	if len(parts) < 2 {
		return Result{ErrorMessage: "Usage: :group move [instance] [group]"}
	}

	instanceRef := parts[0]
	groupRef := strings.Join(parts[1:], " ")

	// Resolve instance
	inst := resolveInstance(instanceRef, session)
	if inst == nil {
		return Result{ErrorMessage: fmt.Sprintf("Instance not found: %s", instanceRef)}
	}

	// Resolve target group
	grp := resolveGroup(groupRef, session)
	if grp == nil {
		return Result{ErrorMessage: fmt.Sprintf("Group not found: %s", groupRef)}
	}

	// Get current group for info message
	currentGroup := mgr.GetGroupForInstance(inst.ID)
	fromMsg := "ungrouped"
	if currentGroup != nil {
		fromMsg = fmt.Sprintf("group %q", currentGroup.Name)
	}

	// Move instance
	mgr.MoveInstanceToGroup(inst.ID, grp.ID)

	return Result{InfoMessage: fmt.Sprintf("Moved instance %q from %s to group %q", inst.EffectiveName(), fromMsg, grp.Name)}
}

// cmdGroupOrder handles ":group order [g1,g2,g3]"
// Reorders groups by ID or name. Groups are specified comma-separated.
func cmdGroupOrder(args string, deps GroupDependencies) Result {
	mgr := deps.GetGroupManager()
	if mgr == nil {
		return Result{ErrorMessage: "Group manager not available"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	args = strings.TrimSpace(args)
	if args == "" {
		return Result{ErrorMessage: "Usage: :group order [g1,g2,g3] (comma-separated group IDs or names)"}
	}

	// Parse comma-separated group references
	refs := strings.Split(args, ",")
	if len(refs) == 0 {
		return Result{ErrorMessage: "No groups specified"}
	}

	// Resolve each reference to a group ID
	var groupIDs []string
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}

		grp := resolveGroup(ref, session)
		if grp == nil {
			return Result{ErrorMessage: fmt.Sprintf("Group not found: %s", ref)}
		}
		groupIDs = append(groupIDs, grp.ID)
	}

	if len(groupIDs) == 0 {
		return Result{ErrorMessage: "No valid groups specified"}
	}

	// Reorder groups
	mgr.ReorderGroups(groupIDs)

	return Result{InfoMessage: fmt.Sprintf("Reordered %d groups", len(groupIDs))}
}

// cmdGroupDelete handles ":group delete [name]"
// Deletes a group. The group must be empty (no instances).
func cmdGroupDelete(args string, deps GroupDependencies) Result {
	mgr := deps.GetGroupManager()
	if mgr == nil {
		return Result{ErrorMessage: "Group manager not available"}
	}

	session := deps.GetSession()
	if session == nil {
		return Result{ErrorMessage: "No session available"}
	}

	groupRef := strings.TrimSpace(args)
	if groupRef == "" {
		return Result{ErrorMessage: "Usage: :group delete [name or ID]"}
	}

	// Resolve group
	grp := resolveGroup(groupRef, session)
	if grp == nil {
		return Result{ErrorMessage: fmt.Sprintf("Group not found: %s", groupRef)}
	}

	// Check if group is empty
	if len(grp.Instances) > 0 {
		return Result{ErrorMessage: fmt.Sprintf("Cannot delete group %q: still has %d instance(s). Remove instances first.", grp.Name, len(grp.Instances))}
	}

	// Check if group has sub-groups
	if len(grp.SubGroups) > 0 {
		return Result{ErrorMessage: fmt.Sprintf("Cannot delete group %q: still has %d sub-group(s). Delete sub-groups first.", grp.Name, len(grp.SubGroups))}
	}

	// Delete the group
	name := grp.Name
	if !mgr.RemoveGroup(grp.ID) {
		return Result{ErrorMessage: fmt.Sprintf("Failed to delete group %q", name)}
	}

	return Result{InfoMessage: fmt.Sprintf("Deleted group %q", name)}
}

// cmdGroupShow handles ":group show"
// Toggles the grouped instance view on/off.
func cmdGroupShow(deps GroupDependencies) Result {
	enabled := deps.IsGroupedViewEnabled()
	toggleGroupedView := !enabled

	var msg string
	if toggleGroupedView {
		msg = "Grouped view enabled"
	} else {
		msg = "Grouped view disabled"
	}

	return Result{
		InfoMessage:       msg,
		ToggleGroupedView: &toggleGroupedView,
	}
}

// resolveInstance resolves an instance reference (ID, short ID prefix, or numeric index).
func resolveInstance(ref string, session *orchestrator.Session) *orchestrator.Instance {
	if session == nil || ref == "" {
		return nil
	}

	// First, try exact ID match
	if inst := session.GetInstance(ref); inst != nil {
		return inst
	}

	// Try numeric index (1-based)
	if len(ref) <= 3 {
		var idx int
		if _, err := fmt.Sscanf(ref, "%d", &idx); err == nil && idx > 0 && idx <= len(session.Instances) {
			return session.Instances[idx-1]
		}
	}

	// Try prefix match on ID
	ref = strings.ToLower(ref)
	for _, inst := range session.Instances {
		if strings.HasPrefix(strings.ToLower(inst.ID), ref) {
			return inst
		}
	}

	return nil
}

// resolveGroup resolves a group reference (ID, name, or numeric index).
func resolveGroup(ref string, session *orchestrator.Session) *orchestrator.InstanceGroup {
	if session == nil || ref == "" {
		return nil
	}

	// First, try exact ID match
	if grp := session.GetGroup(ref); grp != nil {
		return grp
	}

	// Try numeric index (1-based)
	if len(ref) <= 3 {
		var idx int
		if _, err := fmt.Sscanf(ref, "%d", &idx); err == nil && idx > 0 && idx <= len(session.Groups) {
			return session.Groups[idx-1]
		}
	}

	// Try name match (case-insensitive)
	refLower := strings.ToLower(ref)
	for _, grp := range session.Groups {
		if strings.ToLower(grp.Name) == refLower {
			return grp
		}
	}

	// Try prefix match on ID
	for _, grp := range session.Groups {
		if strings.HasPrefix(strings.ToLower(grp.ID), refLower) {
			return grp
		}
	}

	// Try prefix match on name
	for _, grp := range session.Groups {
		if strings.HasPrefix(strings.ToLower(grp.Name), refLower) {
			return grp
		}
	}

	return nil
}

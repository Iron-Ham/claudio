package view

import (
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// GroupedSidebarData holds the structured data for rendering the grouped sidebar.
// It separates instances into ungrouped (flat) and grouped sections.
type GroupedSidebarData struct {
	// UngroupedInstances are instances not belonging to any group (shown in "Instances" section)
	UngroupedInstances []*orchestrator.Instance

	// Groups are session-specific groups (ultraplan, tripleshot, multi-pass plan)
	// Each creates its own collapsible section
	Groups []*orchestrator.InstanceGroup

	// SharedGroups are groups that collect instances of the same type (e.g., "Plans")
	SharedGroups []*orchestrator.InstanceGroup
}

// BuildGroupedSidebarData analyzes a session and builds the structured data
// for rendering a grouped sidebar. It categorizes instances based on their
// group membership and session types.
func BuildGroupedSidebarData(session *orchestrator.Session) *GroupedSidebarData {
	if session == nil {
		return &GroupedSidebarData{}
	}

	data := &GroupedSidebarData{
		UngroupedInstances: make([]*orchestrator.Instance, 0),
		Groups:             make([]*orchestrator.InstanceGroup, 0),
		SharedGroups:       make([]*orchestrator.InstanceGroup, 0),
	}

	// Get thread-safe snapshot of groups
	groups := session.GetGroups()

	// Build a set of instance IDs that belong to groups
	groupedInstanceIDs := make(map[string]bool)
	for _, group := range groups {
		for _, instID := range group.AllInstanceIDs() {
			groupedInstanceIDs[instID] = true
		}
	}

	// Categorize instances
	for _, inst := range session.Instances {
		if !groupedInstanceIDs[inst.ID] {
			data.UngroupedInstances = append(data.UngroupedInstances, inst)
		}
	}

	// Categorize groups by type
	for _, group := range groups {
		if group.SessionType.GroupingMode() == "shared" {
			data.SharedGroups = append(data.SharedGroups, group)
		} else {
			data.Groups = append(data.Groups, group)
		}
	}

	return data
}

// HasGroups returns true if there are any groups to display
func (d *GroupedSidebarData) HasGroups() bool {
	return len(d.Groups) > 0 || len(d.SharedGroups) > 0
}

// TotalGroupCount returns the total number of groups (both regular and shared)
func (d *GroupedSidebarData) TotalGroupCount() int {
	return len(d.Groups) + len(d.SharedGroups)
}

// SidebarSection represents a section in the grouped sidebar
type SidebarSection struct {
	Type      SidebarSectionType
	Title     string                      // Section header (e.g., "INSTANCES", "Plans")
	Icon      string                      // Icon for this section
	Instances []*orchestrator.Instance    // For ungrouped sections
	Group     *orchestrator.InstanceGroup // For group sections
}

// SidebarSectionType identifies the type of sidebar section
type SidebarSectionType int

const (
	// SectionTypeUngrouped is the flat "Instances" section
	SectionTypeUngrouped SidebarSectionType = iota
	// SectionTypeGroup is a session-specific group (ultraplan, tripleshot)
	SectionTypeGroup
	// SectionTypeSharedGroup is a shared category group (e.g., "Plans")
	SectionTypeSharedGroup
)

// BuildSidebarSections creates an ordered list of sections for sidebar rendering.
// The order is:
// 1. Ungrouped instances (if any)
// 2. Session-specific groups (ultraplan, tripleshot, etc.)
// 3. Shared groups (Plans, etc.)
func BuildSidebarSections(session *orchestrator.Session) []SidebarSection {
	data := BuildGroupedSidebarData(session)
	sections := make([]SidebarSection, 0)

	// Add ungrouped instances section if any exist
	if len(data.UngroupedInstances) > 0 {
		sections = append(sections, SidebarSection{
			Type:      SectionTypeUngrouped,
			Title:     "INSTANCES",
			Icon:      "",
			Instances: data.UngroupedInstances,
		})
	}

	// Add session-specific groups
	for _, group := range data.Groups {
		sections = append(sections, SidebarSection{
			Type:  SectionTypeGroup,
			Title: group.Name,
			Icon:  group.SessionType.Icon(),
			Group: group,
		})
	}

	// Add shared groups
	for _, group := range data.SharedGroups {
		sections = append(sections, SidebarSection{
			Type:  SectionTypeSharedGroup,
			Title: group.Name,
			Icon:  group.SessionType.Icon(),
			Group: group,
		})
	}

	return sections
}

// ShouldUseGroupedMode determines if the sidebar should switch to grouped mode.
// Returns true if there are any groups defined in the session.
// This function is thread-safe.
func ShouldUseGroupedMode(session *orchestrator.Session) bool {
	if session == nil {
		return false
	}
	return session.HasGroups()
}

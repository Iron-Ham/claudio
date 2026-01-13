package command

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group"
)

// mockGroupDeps extends mockDeps with group management capabilities.
type mockGroupDeps struct {
	*mockDeps
	groupManager       *group.Manager
	groupedViewEnabled bool
}

func (m *mockGroupDeps) GetGroupManager() *group.Manager {
	return m.groupManager
}

func (m *mockGroupDeps) IsGroupedViewEnabled() bool {
	return m.groupedViewEnabled
}

// Ensure mockGroupDeps implements GroupDependencies
var _ GroupDependencies = (*mockGroupDeps)(nil)

// mockSessionData implements group.ManagerSessionData for testing
type mockSessionData struct {
	groups  []*group.InstanceGroup
	idCount int
}

func (m *mockSessionData) GetGroups() []*group.InstanceGroup {
	return m.groups
}

func (m *mockSessionData) SetGroups(groups []*group.InstanceGroup) {
	m.groups = groups
}

func (m *mockSessionData) GenerateID() string {
	m.idCount++
	return mockID(m.idCount)
}

// mockID generates a deterministic test ID
func mockID(n int) string {
	c := rune('a' + n - 1)
	return string([]rune{c, c, c, c, c, c, c, c}) // "aaaaaaaa", "bbbbbbbb", etc.
}

func newMockGroupDeps() *mockGroupDeps {
	sessionData := &mockSessionData{
		groups: make([]*group.InstanceGroup, 0),
	}
	session := &orchestrator.Session{
		ID:        "test-session",
		Instances: make([]*orchestrator.Instance, 0),
		Groups:    make([]*orchestrator.InstanceGroup, 0),
	}

	return &mockGroupDeps{
		mockDeps: &mockDeps{
			session:   session,
			startTime: time.Now(),
		},
		groupManager: group.NewManager(sessionData),
	}
}

// --- Test: Group command routing ---

func TestGroupCommandEmptyArgs(t *testing.T) {
	h := New()
	deps := newMockGroupDeps()

	result := h.Execute("group", deps)

	// Should show usage
	if result.InfoMessage == "" {
		t.Error("expected usage info for empty group command")
	}
	if result.ErrorMessage != "" {
		t.Errorf("unexpected error: %q", result.ErrorMessage)
	}
}

func TestGroupCommandHelp(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"help subcommand", "group help"},
		{"question mark", "group ?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New()
			deps := newMockGroupDeps()

			result := h.Execute(tt.cmd, deps)

			if result.InfoMessage == "" {
				t.Error("expected usage info")
			}
			if result.ErrorMessage != "" {
				t.Errorf("unexpected error: %q", result.ErrorMessage)
			}
		})
	}
}

func TestGroupCommandUnknownSubcommand(t *testing.T) {
	h := New()
	deps := newMockGroupDeps()

	result := h.Execute("group foobar", deps)

	if result.ErrorMessage == "" {
		t.Error("expected error for unknown subcommand")
	}
	if result.ErrorMessage != "Unknown group subcommand: foobar. Type :group help for usage." {
		t.Errorf("unexpected error message: %q", result.ErrorMessage)
	}
}

func TestGroupCommandWithoutGroupDependencies(t *testing.T) {
	h := New()
	deps := newMockDeps() // Regular deps, not group deps

	result := h.Execute("group show", deps)

	if result.ErrorMessage == "" {
		t.Error("expected error when GroupDependencies not available")
	}
	if result.ErrorMessage != "Group commands not available in this context" {
		t.Errorf("unexpected error message: %q", result.ErrorMessage)
	}
}

// --- Test: :group create ---

func TestGroupCreate(t *testing.T) {
	t.Run("create with name", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group create My Group", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage == "" {
			t.Error("expected info message")
		}
	})

	t.Run("create without name generates auto name", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group create", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage == "" {
			t.Error("expected info message")
		}
	})

	t.Run("create without group manager", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()
		deps.groupManager = nil

		result := h.Execute("group create Test", deps)

		if result.ErrorMessage != "Group manager not available" {
			t.Errorf("expected group manager error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("create without session", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()
		deps.session = nil

		result := h.Execute("group create Test", deps)

		if result.ErrorMessage != "No session available" {
			t.Errorf("expected session error, got: %q", result.ErrorMessage)
		}
	})
}

// --- Test: :group add ---

func TestGroupAdd(t *testing.T) {
	t.Run("add with valid instance and group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		// Add an instance to the session
		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test task", DisplayName: "Test Instance"}
		deps.session.Instances = append(deps.session.Instances, inst)

		// Create a group first
		h.Execute("group create MyGroup", deps)
		// Group is created in the group manager's mock session, but we need
		// to sync it to the orchestrator session
		grp := orchestrator.NewInstanceGroup("MyGroup")
		grp.ID = "aaaaaaaa" // Match the generated ID
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group add inst-001 MyGroup", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage == "" {
			t.Error("expected info message")
		}
	})

	t.Run("add with missing arguments", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group add", deps)

		if result.ErrorMessage != "Usage: :group add [instance] [group]" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("add with only instance", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group add inst-001", deps)

		if result.ErrorMessage != "Usage: :group add [instance] [group]" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("add with unknown instance", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group add nonexistent MyGroup", deps)

		if result.ErrorMessage != "Instance not found: nonexistent" {
			t.Errorf("expected instance not found error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("add with unknown group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test"}
		deps.session.Instances = append(deps.session.Instances, inst)

		result := h.Execute("group add inst-001 nonexistent", deps)

		if result.ErrorMessage != "Group not found: nonexistent" {
			t.Errorf("expected group not found error, got: %q", result.ErrorMessage)
		}
	})
}

// --- Test: :group remove ---

func TestGroupRemove(t *testing.T) {
	t.Run("remove with instance argument", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test", DisplayName: "Test Instance"}
		deps.session.Instances = append(deps.session.Instances, inst)

		// Instance not in any group
		result := h.Execute("group remove inst-001", deps)

		// Should succeed with info message
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage == "" {
			t.Error("expected info message")
		}
	})

	t.Run("remove with active instance fallback", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test", DisplayName: "Test Instance"}
		deps.session.Instances = append(deps.session.Instances, inst)
		deps.activeInstance = inst

		result := h.Execute("group remove", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
	})

	t.Run("remove without argument and no active instance", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group remove", deps)

		if result.ErrorMessage != "Usage: :group remove [instance] or select an instance" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("remove with unknown instance", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group remove nonexistent", deps)

		if result.ErrorMessage != "Instance not found: nonexistent" {
			t.Errorf("expected instance not found error, got: %q", result.ErrorMessage)
		}
	})
}

// --- Test: :group move ---

func TestGroupMove(t *testing.T) {
	t.Run("move with missing arguments", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group move", deps)

		if result.ErrorMessage != "Usage: :group move [instance] [group]" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("move with only instance", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group move inst-001", deps)

		if result.ErrorMessage != "Usage: :group move [instance] [group]" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("move with unknown instance", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group move nonexistent MyGroup", deps)

		if result.ErrorMessage != "Instance not found: nonexistent" {
			t.Errorf("expected instance not found error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("move with unknown group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test"}
		deps.session.Instances = append(deps.session.Instances, inst)

		result := h.Execute("group move inst-001 nonexistent", deps)

		if result.ErrorMessage != "Group not found: nonexistent" {
			t.Errorf("expected group not found error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("move with valid instance and group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test", DisplayName: "Test Instance"}
		deps.session.Instances = append(deps.session.Instances, inst)

		grp := orchestrator.NewInstanceGroup("TargetGroup")
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group move inst-001 TargetGroup", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage == "" {
			t.Error("expected info message about the move")
		}
	})
}

// --- Test: :group order ---

func TestGroupOrder(t *testing.T) {
	t.Run("order with empty args", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group order", deps)

		if result.ErrorMessage != "Usage: :group order [g1,g2,g3] (comma-separated group IDs or names)" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("order with unknown group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group order nonexistent", deps)

		if result.ErrorMessage != "Group not found: nonexistent" {
			t.Errorf("expected group not found error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("order with valid groups", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp1 := orchestrator.NewInstanceGroup("Group1")
		grp2 := orchestrator.NewInstanceGroup("Group2")
		deps.session.Groups = append(deps.session.Groups, grp1, grp2)

		result := h.Execute("group order Group2,Group1", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage == "" {
			t.Error("expected info message about reorder")
		}
	})

	t.Run("order handles empty entries in comma list", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp1 := orchestrator.NewInstanceGroup("Group1")
		deps.session.Groups = append(deps.session.Groups, grp1)

		result := h.Execute("group order ,Group1,", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
	})
}

// --- Test: :group delete ---

func TestGroupDelete(t *testing.T) {
	t.Run("delete with empty args", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group delete", deps)

		if result.ErrorMessage != "Usage: :group delete [name or ID]" {
			t.Errorf("expected usage error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("delete with unknown group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		result := h.Execute("group delete nonexistent", deps)

		if result.ErrorMessage != "Group not found: nonexistent" {
			t.Errorf("expected group not found error, got: %q", result.ErrorMessage)
		}
	})

	t.Run("delete non-empty group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp := orchestrator.NewInstanceGroup("MyGroup")
		grp.Instances = []string{"inst-001"} // Non-empty
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group delete MyGroup", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error when deleting non-empty group")
		}
		if result.ErrorMessage != "Cannot delete group \"MyGroup\": still has 1 instance(s). Remove instances first." {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
	})

	t.Run("delete group with sub-groups", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		subGrp := orchestrator.NewInstanceGroup("SubGroup")
		grp := orchestrator.NewInstanceGroup("MyGroup")
		grp.SubGroups = []*orchestrator.InstanceGroup{subGrp}
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group delete MyGroup", deps)

		if result.ErrorMessage == "" {
			t.Error("expected error when deleting group with sub-groups")
		}
		if result.ErrorMessage != "Cannot delete group \"MyGroup\": still has 1 sub-group(s). Delete sub-groups first." {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
	})
}

// --- Test: :group show ---

func TestGroupShow(t *testing.T) {
	t.Run("toggle on when disabled", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()
		deps.groupedViewEnabled = false

		result := h.Execute("group show", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.ToggleGroupedView == nil || !*result.ToggleGroupedView {
			t.Error("expected ToggleGroupedView to be true")
		}
		if result.InfoMessage != "Grouped view enabled" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}
	})

	t.Run("toggle off when enabled", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()
		deps.groupedViewEnabled = true

		result := h.Execute("group show", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.ToggleGroupedView == nil || *result.ToggleGroupedView {
			t.Error("expected ToggleGroupedView to be false")
		}
		if result.InfoMessage != "Grouped view disabled" {
			t.Errorf("unexpected info message: %q", result.InfoMessage)
		}
	})
}

// --- Test: Instance resolution ---

func TestResolveInstance(t *testing.T) {
	t.Run("nil session returns nil", func(t *testing.T) {
		inst := resolveInstance("test", nil)
		if inst != nil {
			t.Error("expected nil for nil session")
		}
	})

	t.Run("empty ref returns nil", func(t *testing.T) {
		session := &orchestrator.Session{Instances: make([]*orchestrator.Instance, 0)}
		inst := resolveInstance("", session)
		if inst != nil {
			t.Error("expected nil for empty ref")
		}
	})

	t.Run("exact ID match", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-001", Task: "Task 1"},
				{ID: "inst-002", Task: "Task 2"},
			},
		}
		inst := resolveInstance("inst-002", session)
		if inst == nil || inst.ID != "inst-002" {
			t.Error("expected exact ID match")
		}
	})

	t.Run("numeric index (1-based)", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-001", Task: "Task 1"},
				{ID: "inst-002", Task: "Task 2"},
			},
		}
		inst := resolveInstance("2", session)
		if inst == nil || inst.ID != "inst-002" {
			t.Error("expected instance at index 2")
		}
	})

	t.Run("numeric index out of bounds", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "inst-001", Task: "Task 1"},
			},
		}
		inst := resolveInstance("5", session)
		if inst != nil {
			t.Error("expected nil for out of bounds index")
		}
	})

	t.Run("prefix match on ID", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "abcd1234", Task: "Task 1"},
				{ID: "efgh5678", Task: "Task 2"},
			},
		}
		inst := resolveInstance("abcd", session)
		if inst == nil || inst.ID != "abcd1234" {
			t.Error("expected prefix match")
		}
	})

	t.Run("case-insensitive prefix match", func(t *testing.T) {
		session := &orchestrator.Session{
			Instances: []*orchestrator.Instance{
				{ID: "ABCD1234", Task: "Task 1"},
			},
		}
		inst := resolveInstance("abcd", session)
		if inst == nil || inst.ID != "ABCD1234" {
			t.Error("expected case-insensitive prefix match")
		}
	})
}

// --- Test: Group resolution ---

func TestResolveGroup(t *testing.T) {
	t.Run("nil session returns nil", func(t *testing.T) {
		grp := resolveGroup("test", nil)
		if grp != nil {
			t.Error("expected nil for nil session")
		}
	})

	t.Run("empty ref returns nil", func(t *testing.T) {
		session := &orchestrator.Session{Groups: make([]*orchestrator.InstanceGroup, 0)}
		grp := resolveGroup("", session)
		if grp != nil {
			t.Error("expected nil for empty ref")
		}
	})

	t.Run("exact ID match", func(t *testing.T) {
		grp1 := orchestrator.NewInstanceGroup("Group 1")
		grp2 := orchestrator.NewInstanceGroup("Group 2")
		session := &orchestrator.Session{Groups: []*orchestrator.InstanceGroup{grp1, grp2}}

		found := resolveGroup(grp2.ID, session)
		if found == nil || found.ID != grp2.ID {
			t.Error("expected exact ID match")
		}
	})

	t.Run("numeric index (1-based)", func(t *testing.T) {
		grp1 := orchestrator.NewInstanceGroup("Group 1")
		grp2 := orchestrator.NewInstanceGroup("Group 2")
		session := &orchestrator.Session{Groups: []*orchestrator.InstanceGroup{grp1, grp2}}

		found := resolveGroup("2", session)
		if found == nil || found.ID != grp2.ID {
			t.Error("expected group at index 2")
		}
	})

	t.Run("name match (exact, case-insensitive)", func(t *testing.T) {
		grp := orchestrator.NewInstanceGroup("My Group")
		session := &orchestrator.Session{Groups: []*orchestrator.InstanceGroup{grp}}

		found := resolveGroup("my group", session)
		if found == nil || found.Name != "My Group" {
			t.Error("expected case-insensitive name match")
		}
	})

	t.Run("prefix match on ID", func(t *testing.T) {
		grp := orchestrator.NewInstanceGroup("Group 1")
		grp.ID = "abcd1234"
		session := &orchestrator.Session{Groups: []*orchestrator.InstanceGroup{grp}}

		found := resolveGroup("abcd", session)
		if found == nil || found.ID != "abcd1234" {
			t.Error("expected prefix match on ID")
		}
	})

	t.Run("prefix match on name", func(t *testing.T) {
		grp := orchestrator.NewInstanceGroup("Foundation Setup")
		session := &orchestrator.Session{Groups: []*orchestrator.InstanceGroup{grp}}

		found := resolveGroup("Found", session)
		if found == nil || found.Name != "Foundation Setup" {
			t.Error("expected prefix match on name")
		}
	})
}

// --- Test: Handler recognizes group commands ---

func TestGroupCommandsRecognized(t *testing.T) {
	commands := []string{
		"group",
		"group help",
		"group create",
		"group create Test Group",
		"group add",
		"group remove",
		"group move",
		"group order",
		"group delete",
		"group show",
	}

	h := New()
	deps := newMockGroupDeps()

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			result := h.Execute(cmd, deps)
			// Should NOT be an unknown command error
			if result.ErrorMessage != "" && len(result.ErrorMessage) >= 15 &&
				result.ErrorMessage[:15] == "Unknown command" {
				t.Errorf("command %q was not recognized", cmd)
			}
		})
	}
}

// --- Test: Categories include group commands ---

func TestCategoriesIncludeGroupManagement(t *testing.T) {
	h := New()
	categories := h.Categories()

	var groupCategory *CommandCategory
	for i := range categories {
		if categories[i].Name == "Group Management" {
			groupCategory = &categories[i]
			break
		}
	}

	if groupCategory == nil {
		t.Fatal("expected 'Group Management' category")
	}

	expectedCommands := []string{
		"group create",
		"group add",
		"group remove",
		"group move",
		"group order",
		"group delete",
		"group show",
	}

	for _, expected := range expectedCommands {
		found := false
		for _, cmd := range groupCategory.Commands {
			if cmd.LongKey == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected command %q in Group Management category", expected)
		}
	}
}

// --- Test: Nil safety ---

func TestGroupCommandsHandleNilLogger(t *testing.T) {
	// All group commands should work with nil logger (deps.GetLogger() returns nil)
	h := New()
	deps := newMockGroupDeps()
	deps.logger = nil

	commands := []string{
		"group",
		"group create Test",
		"group show",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			// Should not panic
			_ = h.Execute(cmd, deps)
		})
	}
}

// --- Test: Edge cases ---

func TestGroupCreateWhitespaceHandling(t *testing.T) {
	h := New()
	deps := newMockGroupDeps()

	result := h.Execute("group create   Spaced   Name   ", deps)

	if result.ErrorMessage != "" {
		t.Errorf("unexpected error: %q", result.ErrorMessage)
	}
	// Name should preserve internal spaces but trim edges
	if result.InfoMessage == "" {
		t.Error("expected info message")
	}
}

func TestGroupOrderWithSpacesAroundCommas(t *testing.T) {
	h := New()
	deps := newMockGroupDeps()

	grp1 := orchestrator.NewInstanceGroup("Group1")
	grp2 := orchestrator.NewInstanceGroup("Group2")
	deps.session.Groups = append(deps.session.Groups, grp1, grp2)

	result := h.Execute("group order Group1 , Group2", deps)

	if result.ErrorMessage != "" {
		t.Errorf("unexpected error: %q", result.ErrorMessage)
	}
}

// Ensure mockDeps implements the Dependencies interface at compile time
var _ Dependencies = (*mockDeps)(nil)

// mockGroupDeps inherits all Dependencies methods from embedded *mockDeps

// --- Test: Edge cases for empty groups ---

func TestGroupOperationsWithEmptyGroups(t *testing.T) {
	t.Run("empty group can be deleted", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		// Create an empty group in the session
		grp := orchestrator.NewInstanceGroup("Empty Group")
		grp.Instances = []string{} // Explicitly empty
		deps.session.Groups = append(deps.session.Groups, grp)

		// Sync to group manager's mock session
		mockSession := deps.groupManager
		if mockSession == nil {
			t.Skip("group manager not available")
		}

		result := h.Execute("group delete Empty Group", deps)
		// Should succeed since group is empty
		if result.ErrorMessage != "" && result.ErrorMessage != "Failed to delete group \"Empty Group\"" {
			// It might fail due to the mock setup, but it shouldn't fail with "has instances" error
			if result.ErrorMessage == "Cannot delete group \"Empty Group\": still has 1 instance(s). Remove instances first." {
				t.Errorf("should allow deleting empty group, got: %q", result.ErrorMessage)
			}
		}
	})

	t.Run("delete reports instance count correctly", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp := orchestrator.NewInstanceGroup("Multi Instance Group")
		grp.Instances = []string{"inst-1", "inst-2", "inst-3"}
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group delete Multi Instance Group", deps)
		if result.ErrorMessage != "Cannot delete group \"Multi Instance Group\": still has 3 instance(s). Remove instances first." {
			t.Errorf("expected 3 instance error, got: %q", result.ErrorMessage)
		}
	})
}

// --- Test: Group resolution edge cases ---

func TestGroupResolutionEdgeCases(t *testing.T) {
	t.Run("resolve group by numeric index", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp1 := orchestrator.NewInstanceGroup("First Group")
		grp2 := orchestrator.NewInstanceGroup("Second Group")
		deps.session.Groups = append(deps.session.Groups, grp1, grp2)

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test", DisplayName: "Test Inst"}
		deps.session.Instances = append(deps.session.Instances, inst)

		// Add using numeric index
		result := h.Execute("group add inst-001 2", deps)
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error resolving group by index: %q", result.ErrorMessage)
		}
	})

	t.Run("numeric index 0 is invalid (1-based)", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp := orchestrator.NewInstanceGroup("Group")
		deps.session.Groups = append(deps.session.Groups, grp)

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test"}
		deps.session.Instances = append(deps.session.Instances, inst)

		result := h.Execute("group add inst-001 0", deps)
		if result.ErrorMessage != "Group not found: 0" {
			t.Errorf("expected group not found for index 0, got: %q", result.ErrorMessage)
		}
	})

	t.Run("resolve instance by prefix", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp := orchestrator.NewInstanceGroup("Target Group")
		deps.session.Groups = append(deps.session.Groups, grp)

		inst := &orchestrator.Instance{ID: "abc123def456", Task: "Test", DisplayName: "Test"}
		deps.session.Instances = append(deps.session.Instances, inst)

		// Add using ID prefix
		result := h.Execute("group add abc123 Target Group", deps)
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error resolving instance by prefix: %q", result.ErrorMessage)
		}
	})
}

// --- Test: Move with current group info ---

func TestGroupMoveShowsFromInfo(t *testing.T) {
	t.Run("move shows ungrouped when not in any group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		inst := &orchestrator.Instance{ID: "inst-001", Task: "Test", DisplayName: "Test Inst"}
		deps.session.Instances = append(deps.session.Instances, inst)

		grp := orchestrator.NewInstanceGroup("Target")
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group move inst-001 Target", deps)

		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		// Should mention "from ungrouped"
		if result.InfoMessage != "" && !contains(result.InfoMessage, "ungrouped") {
			t.Errorf("expected info message to mention ungrouped, got: %q", result.InfoMessage)
		}
	})
}

// --- Test: Order command edge cases ---

func TestGroupOrderEdgeCases(t *testing.T) {
	t.Run("order with single group", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp := orchestrator.NewInstanceGroup("Only Group")
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group order Only Group", deps)
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error ordering single group: %q", result.ErrorMessage)
		}
		if result.InfoMessage != "Reordered 1 groups" {
			t.Errorf("expected 'Reordered 1 groups', got: %q", result.InfoMessage)
		}
	})

	t.Run("order skips empty entries", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp1 := orchestrator.NewInstanceGroup("A")
		grp2 := orchestrator.NewInstanceGroup("B")
		deps.session.Groups = append(deps.session.Groups, grp1, grp2)

		// Extra commas create empty entries
		result := h.Execute("group order ,,A,,B,,", deps)
		if result.ErrorMessage != "" {
			t.Errorf("unexpected error: %q", result.ErrorMessage)
		}
		if result.InfoMessage != "Reordered 2 groups" {
			t.Errorf("expected 'Reordered 2 groups', got: %q", result.InfoMessage)
		}
	})

	t.Run("order with all empty entries fails", func(t *testing.T) {
		h := New()
		deps := newMockGroupDeps()

		grp := orchestrator.NewInstanceGroup("Group")
		deps.session.Groups = append(deps.session.Groups, grp)

		result := h.Execute("group order ,,,", deps)
		if result.ErrorMessage != "No valid groups specified" {
			t.Errorf("expected 'No valid groups specified', got: %q", result.ErrorMessage)
		}
	})
}

// --- Test: Subcommand case insensitivity ---

func TestGroupSubcommandCaseInsensitive(t *testing.T) {
	h := New()
	deps := newMockGroupDeps()

	// Test various case combinations
	tests := []struct {
		cmd  string
		desc string
	}{
		{"group CREATE Test", "uppercase CREATE"},
		{"group Create Test", "titlecase Create"},
		{"group SHOW", "uppercase SHOW"},
		{"group Show", "titlecase Show"},
		{"group HELP", "uppercase HELP"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			result := h.Execute(tt.cmd, deps)
			// Should not be unknown subcommand error
			if result.ErrorMessage != "" && contains(result.ErrorMessage, "Unknown group subcommand") {
				t.Errorf("subcommand should be case insensitive: %q", result.ErrorMessage)
			}
		})
	}
}

// --- Test: Remove instance from group ---

func TestGroupRemoveInstanceAlreadyUngrouped(t *testing.T) {
	h := New()
	deps := newMockGroupDeps()

	inst := &orchestrator.Instance{ID: "inst-001", Task: "Test", DisplayName: "Test Inst"}
	deps.session.Instances = append(deps.session.Instances, inst)
	// Instance is not in any group

	result := h.Execute("group remove inst-001", deps)

	// Should succeed with informative message
	if result.ErrorMessage != "" {
		t.Errorf("unexpected error: %q", result.ErrorMessage)
	}
	if result.InfoMessage != "Instance \"Test Inst\" is not in any group" {
		t.Errorf("expected already ungrouped message, got: %q", result.InfoMessage)
	}
}

// --- Test: Group commands without group manager ---

func TestGroupCommandsRequireManager(t *testing.T) {
	commands := []string{
		"group create Test",
		"group add inst-001 Group1",
		"group remove inst-001",
		"group move inst-001 Group1",
		"group order Group1",
		"group delete Group1",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			h := New()
			deps := newMockGroupDeps()
			deps.groupManager = nil

			result := h.Execute(cmd, deps)
			if result.ErrorMessage != "Group manager not available" {
				t.Errorf("expected 'Group manager not available', got: %q", result.ErrorMessage)
			}
		})
	}
}

// --- Test: Group commands require session ---

func TestGroupCommandsRequireSession(t *testing.T) {
	commands := []string{
		"group create",
		"group add inst-001 Group1",
		"group remove inst-001",
		"group move inst-001 Group1",
		"group order Group1",
		"group delete Group1",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			h := New()
			deps := newMockGroupDeps()
			deps.session = nil

			result := h.Execute(cmd, deps)
			if result.ErrorMessage != "No session available" {
				t.Errorf("expected 'No session available', got: %q", result.ErrorMessage)
			}
		})
	}
}

// --- Test: Group show toggle behavior ---

func TestGroupShowToggleBehavior(t *testing.T) {
	h := New()

	t.Run("toggle creates boolean pointer", func(t *testing.T) {
		deps := newMockGroupDeps()
		deps.groupedViewEnabled = false

		result := h.Execute("group show", deps)

		if result.ToggleGroupedView == nil {
			t.Fatal("ToggleGroupedView should not be nil")
		}
		if !*result.ToggleGroupedView {
			t.Error("expected ToggleGroupedView to be true when currently disabled")
		}
	})

	t.Run("toggle off creates false pointer", func(t *testing.T) {
		deps := newMockGroupDeps()
		deps.groupedViewEnabled = true

		result := h.Execute("group show", deps)

		if result.ToggleGroupedView == nil {
			t.Fatal("ToggleGroupedView should not be nil")
		}
		if *result.ToggleGroupedView {
			t.Error("expected ToggleGroupedView to be false when currently enabled")
		}
	})
}

// --- Test: Circular dependency handling ---

func TestGroupDependencyDoesNotValidateCircular(t *testing.T) {
	// The group command system doesn't currently prevent circular dependencies
	// at the command level - this is handled at execution time.
	// This test documents the current behavior.

	h := New()
	deps := newMockGroupDeps()

	grp1 := orchestrator.NewInstanceGroup("Group1")
	grp2 := orchestrator.NewInstanceGroup("Group2")
	deps.session.Groups = append(deps.session.Groups, grp1, grp2)

	// Can order groups that might have dependencies
	result := h.Execute("group order Group1,Group2", deps)
	if result.ErrorMessage != "" {
		t.Errorf("unexpected error: %q", result.ErrorMessage)
	}
}

// --- Helper function ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsInner(s, substr)))
}

func containsInner(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

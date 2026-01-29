package orchestrator

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

// TestDefaultTripleShotConfig tests that the factory returns a valid default config.
func TestDefaultTripleShotConfig(t *testing.T) {
	config := DefaultTripleShotConfig()

	// Should return a valid config (not panic)
	if config.AutoApprove {
		t.Error("default AutoApprove should be false")
	}
}

// TestNewTripleShotSession tests the tripleshot session factory.
func TestNewTripleShotSession(t *testing.T) {
	task := "Implement feature X"
	config := DefaultTripleShotConfig()

	session := NewTripleShotSession(task, config)

	if session == nil {
		t.Fatal("NewTripleShotSession returned nil")
	}
	if session.Task != task {
		t.Errorf("session.Task = %q, want %q", session.Task, task)
	}
	if session.ID == "" {
		t.Error("session.ID should not be empty")
	}
}

// TestDefaultAdversarialConfig tests the adversarial config factory.
func TestDefaultAdversarialConfig(t *testing.T) {
	config := DefaultAdversarialConfig()

	if config.MaxIterations <= 0 {
		t.Error("MaxIterations should be positive")
	}
	if config.MinPassingScore < 1 || config.MinPassingScore > 10 {
		t.Errorf("MinPassingScore should be between 1-10, got %d", config.MinPassingScore)
	}
}

// TestNewAdversarialSession tests the adversarial session factory.
func TestNewAdversarialSession(t *testing.T) {
	task := "Review implementation"
	config := DefaultAdversarialConfig()

	session := NewAdversarialSession(task, config)

	if session == nil {
		t.Fatal("NewAdversarialSession returned nil")
	}
	if session.Task != task {
		t.Errorf("session.Task = %q, want %q", session.Task, task)
	}
	if session.ID == "" {
		t.Error("session.ID should not be empty")
	}
	if session.CurrentRound != 1 {
		t.Errorf("session.CurrentRound = %d, want 1", session.CurrentRound)
	}
}

// TestDefaultRalphConfig tests the ralph config factory.
func TestDefaultRalphConfig(t *testing.T) {
	config := DefaultRalphConfig()

	if config == nil {
		t.Fatal("DefaultRalphConfig returned nil")
	}
}

// TestNewRalphSession tests the ralph session factory.
func TestNewRalphSession(t *testing.T) {
	prompt := "Execute ralph workflow"
	config := DefaultRalphConfig()

	session := NewRalphSession(prompt, config)

	if session == nil {
		t.Fatal("NewRalphSession returned nil")
	}
}

// TestSessionAdapterGetGroup tests the sessionAdapter's GetGroup method.
func TestSessionAdapterGetGroup(t *testing.T) {
	session := NewSession("test", "/repo")
	group := NewInstanceGroup("test group")
	session.AddGroup(group)

	adapter := &sessionAdapter{session: session}

	// Test finding existing group
	result := adapter.GetGroup(group.ID)
	if result == nil {
		t.Error("GetGroup should find existing group")
	}

	// Test not finding non-existent group
	result = adapter.GetGroup("nonexistent")
	if result != nil {
		t.Error("GetGroup should return nil for non-existent group")
	}
}

// TestSessionAdapterGetGroupBySessionType tests the sessionAdapter's GetGroupBySessionType method.
func TestSessionAdapterGetGroupBySessionType(t *testing.T) {
	session := NewSession("test", "/repo")
	group := NewInstanceGroup("tripleshot group")
	group.SessionType = string(SessionTypeTripleShot)
	session.AddGroup(group)

	adapter := &sessionAdapter{session: session}

	// Test finding by session type
	result := adapter.GetGroupBySessionType(string(SessionTypeTripleShot))
	if result == nil {
		t.Error("GetGroupBySessionType should find group with matching session type")
	}

	// Test not finding non-existent session type
	result = adapter.GetGroupBySessionType(string(SessionTypeAdversarial))
	if result != nil {
		t.Error("GetGroupBySessionType should return nil for non-matching session type")
	}
}

// TestSessionAdapterGetInstance tests the sessionAdapter's GetInstance method.
func TestSessionAdapterGetInstance(t *testing.T) {
	session := NewSession("test", "/repo")

	// Create a mock instance
	inst := &Instance{ID: "inst-1"}
	session.Instances = append(session.Instances, inst)

	adapter := &sessionAdapter{session: session}

	// Test finding existing instance
	result := adapter.GetInstance("inst-1")
	if result == nil {
		t.Error("GetInstance should find existing instance")
	}

	// Test not finding non-existent instance
	result = adapter.GetInstance("nonexistent")
	if result != nil {
		t.Error("GetInstance should return nil for non-existent instance")
	}
}

// TestGroupAdapterAddInstance tests the groupAdapter's AddInstance method.
func TestGroupAdapterAddInstance(t *testing.T) {
	group := NewInstanceGroup("test group")
	adapter := &groupAdapter{group: group}

	adapter.AddInstance("inst-1")
	adapter.AddInstance("inst-2")

	if len(group.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(group.Instances))
	}
	if group.Instances[0] != "inst-1" {
		t.Errorf("first instance = %q, want %q", group.Instances[0], "inst-1")
	}
}

// TestGroupAdapterAddSubGroup tests the groupAdapter's AddSubGroup method.
func TestGroupAdapterAddSubGroup(t *testing.T) {
	parent := NewInstanceGroup("parent")
	child := NewInstanceGroup("child")

	parentAdapter := &groupAdapter{group: parent}
	childAdapter := &groupAdapter{group: child}

	parentAdapter.AddSubGroup(childAdapter)

	if len(parent.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group, got %d", len(parent.SubGroups))
	}
	if parent.SubGroups[0] != child {
		t.Error("sub-group should be the child")
	}
}

// TestGroupAdapterGetInstances tests the groupAdapter's GetInstances method.
func TestGroupAdapterGetInstances(t *testing.T) {
	group := NewInstanceGroup("test")
	group.Instances = []string{"inst-1", "inst-2"}

	adapter := &groupAdapter{group: group}

	instances := adapter.GetInstances()
	if len(instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(instances))
	}
}

// TestGroupAdapterSetInstances tests the groupAdapter's SetInstances method.
func TestGroupAdapterSetInstances(t *testing.T) {
	group := NewInstanceGroup("test")
	adapter := &groupAdapter{group: group}

	newInstances := []string{"inst-3", "inst-4", "inst-5"}
	adapter.SetInstances(newInstances)

	if len(group.Instances) != 3 {
		t.Errorf("expected 3 instances, got %d", len(group.Instances))
	}
	for i, id := range newInstances {
		if group.Instances[i] != id {
			t.Errorf("instance[%d] = %q, want %q", i, group.Instances[i], id)
		}
	}
}

// TestAdversarialSessionAdapterGetGroup tests adversarial session adapter.
func TestAdversarialSessionAdapterGetGroup(t *testing.T) {
	session := NewSession("test", "/repo")
	group := NewInstanceGroup("adversarial group")
	session.AddGroup(group)

	adapter := &adversarialSessionAdapter{session: session}

	result := adapter.GetGroup(group.ID)
	if result == nil {
		t.Error("GetGroup should find existing group")
	}

	result = adapter.GetGroup("nonexistent")
	if result != nil {
		t.Error("GetGroup should return nil for non-existent group")
	}
}

// TestAdversarialGroupAdapterAddInstance tests adversarial group adapter.
func TestAdversarialGroupAdapterAddInstance(t *testing.T) {
	group := NewInstanceGroup("test")
	adapter := &adversarialGroupAdapter{group: group}

	adapter.AddInstance("inst-1")

	if len(group.Instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(group.Instances))
	}
}

// TestRalphSessionAdapterGetGroup tests ralph session adapter.
func TestRalphSessionAdapterGetGroup(t *testing.T) {
	session := NewSession("test", "/repo")
	group := NewInstanceGroup("ralph group")
	session.AddGroup(group)

	adapter := &ralphSessionAdapter{session: session}

	result := adapter.GetGroup(group.ID)
	if result == nil {
		t.Error("GetGroup should find existing group")
	}

	result = adapter.GetGroup("nonexistent")
	if result != nil {
		t.Error("GetGroup should return nil for non-existent group")
	}
}

// TestRalphGroupAdapterAddInstance tests ralph group adapter.
func TestRalphGroupAdapterAddInstance(t *testing.T) {
	group := NewInstanceGroup("test")
	adapter := &ralphGroupAdapter{group: group}

	adapter.AddInstance("inst-1")

	if len(group.Instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(group.Instances))
	}
}

// TestTripleShotInterfaceSatisfaction verifies adapters satisfy their interfaces.
func TestTripleShotInterfaceSatisfaction(t *testing.T) {
	session := NewSession("test", "/repo")

	// Verify sessionAdapter satisfies tripleshot.SessionInterface
	var _ tripleshot.SessionInterface = &sessionAdapter{session: session}

	// Verify groupAdapter satisfies tripleshot.GroupInterface
	group := NewInstanceGroup("test")
	var _ tripleshot.GroupInterface = &groupAdapter{group: group}
}

// TestAdversarialInterfaceSatisfaction verifies adapters satisfy their interfaces.
func TestAdversarialInterfaceSatisfaction(t *testing.T) {
	session := NewSession("test", "/repo")

	// Verify adversarialSessionAdapter satisfies adversarial.SessionInterface
	var _ adversarial.SessionInterface = &adversarialSessionAdapter{session: session}

	// Verify adversarialGroupAdapter satisfies adversarial.GroupInterface
	group := NewInstanceGroup("test")
	var _ adversarial.GroupInterface = &adversarialGroupAdapter{group: group}

	// Verify adversarialGroupAdapter also satisfies adversarial.GroupWithSubGroupsInterface
	var _ adversarial.GroupWithSubGroupsInterface = &adversarialGroupAdapter{group: group}
}

// TestAdversarialGroupAdapterGetOrCreateSubGroup tests sub-group creation.
func TestAdversarialGroupAdapterGetOrCreateSubGroup(t *testing.T) {
	group := NewInstanceGroup("parent")
	adapter := &adversarialGroupAdapter{group: group}

	// Create a sub-group
	subGroup := adapter.GetOrCreateSubGroup("sub-1", "Round 1")
	if subGroup == nil {
		t.Fatal("expected sub-group, got nil")
	}

	// Verify the sub-group was added to the parent
	if len(group.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group, got %d", len(group.SubGroups))
	}
	if group.SubGroups[0].Name != "Round 1" {
		t.Errorf("expected sub-group name 'Round 1', got '%s'", group.SubGroups[0].Name)
	}

	// Getting the same sub-group by name should return existing
	subGroup2 := adapter.GetOrCreateSubGroup("sub-1-new", "Round 1")
	if subGroup2 == nil {
		t.Fatal("expected existing sub-group, got nil")
	}
	// Should not have created another sub-group
	if len(group.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group after second call, got %d", len(group.SubGroups))
	}

	// Add an instance to the sub-group
	subGroup.AddInstance("inst-1")
	if len(group.SubGroups[0].Instances) != 1 {
		t.Errorf("expected 1 instance in sub-group, got %d", len(group.SubGroups[0].Instances))
	}
}

// TestAdversarialGroupAdapterGetSubGroupByName tests sub-group lookup by name.
func TestAdversarialGroupAdapterGetSubGroupByName(t *testing.T) {
	group := NewInstanceGroup("parent")
	adapter := &adversarialGroupAdapter{group: group}

	// Should return nil when no sub-groups exist
	if adapter.GetSubGroupByName("Round 1") != nil {
		t.Error("expected nil for non-existent sub-group")
	}

	// Create a sub-group
	adapter.GetOrCreateSubGroup("sub-1", "Round 1")

	// Should find the sub-group
	found := adapter.GetSubGroupByName("Round 1")
	if found == nil {
		t.Error("expected to find sub-group 'Round 1'")
	}

	// Should return nil for non-existent name
	if adapter.GetSubGroupByName("Round 2") != nil {
		t.Error("expected nil for 'Round 2'")
	}
}

// TestAdversarialGroupAdapterNilGroup tests behavior with nil group.
func TestAdversarialGroupAdapterNilGroup(t *testing.T) {
	adapter := &adversarialGroupAdapter{group: nil}

	// Should not panic
	if adapter.GetOrCreateSubGroup("id", "name") != nil {
		t.Error("expected nil from GetOrCreateSubGroup with nil group")
	}
	if adapter.GetSubGroupByName("name") != nil {
		t.Error("expected nil from GetSubGroupByName with nil group")
	}
	if adapter.GetSubGroupByID("id") != nil {
		t.Error("expected nil from GetSubGroupByID with nil group")
	}
	if adapter.MoveSubGroupUnder("sub", "target", "Target") {
		t.Error("expected false from MoveSubGroupUnder with nil group")
	}
}

// TestAdversarialGroupAdapterGetSubGroupByID tests sub-group lookup by ID.
func TestAdversarialGroupAdapterGetSubGroupByID(t *testing.T) {
	group := NewInstanceGroup("parent")
	adapter := &adversarialGroupAdapter{group: group}

	// Should return nil when no sub-groups exist
	if adapter.GetSubGroupByID("sub-1") != nil {
		t.Error("expected nil for non-existent sub-group ID")
	}

	// Create a sub-group
	adapter.GetOrCreateSubGroup("sub-1", "Round 1")

	// Should find the sub-group by ID
	found := adapter.GetSubGroupByID("sub-1")
	if found == nil {
		t.Error("expected to find sub-group by ID 'sub-1'")
	}

	// Should return nil for non-existent ID
	if adapter.GetSubGroupByID("sub-2") != nil {
		t.Error("expected nil for 'sub-2'")
	}
}

// TestAdversarialGroupAdapterMoveSubGroupUnder tests moving sub-groups.
func TestAdversarialGroupAdapterMoveSubGroupUnder(t *testing.T) {
	group := NewInstanceGroup("parent")
	adapter := &adversarialGroupAdapter{group: group}

	// Create round 1 sub-group
	adapter.GetOrCreateSubGroup("sub-round-1", "Round 1")

	// Initially parent has 1 sub-group
	if len(group.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group, got %d", len(group.SubGroups))
	}

	// Move round 1 under "Previous Rounds" (which doesn't exist yet)
	result := adapter.MoveSubGroupUnder("sub-round-1", "sub-previous-rounds", "Previous Rounds")
	if !result {
		t.Error("MoveSubGroupUnder should return true when successful")
	}

	// Now parent should have 1 sub-group (Previous Rounds) since Round 1 was moved
	if len(group.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group after move, got %d", len(group.SubGroups))
	}

	// Previous Rounds should exist
	var previousRounds *InstanceGroup
	for _, sg := range group.SubGroups {
		if sg.Name == "Previous Rounds" {
			previousRounds = sg
			break
		}
	}
	if previousRounds == nil {
		t.Fatal("Previous Rounds sub-group should have been created")
	}

	// Round 1 should be under Previous Rounds
	if len(previousRounds.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group under Previous Rounds, got %d", len(previousRounds.SubGroups))
	}
	if previousRounds.SubGroups[0].Name != "Round 1" {
		t.Errorf("expected Round 1 under Previous Rounds, got %s", previousRounds.SubGroups[0].Name)
	}

	// Round 1's ParentID should be updated
	if previousRounds.SubGroups[0].ParentID != previousRounds.ID {
		t.Errorf("Round 1's ParentID should be Previous Rounds ID")
	}
}

// TestAdversarialGroupAdapterMoveSubGroupUnder_MultipleRounds tests moving multiple rounds.
func TestAdversarialGroupAdapterMoveSubGroupUnder_MultipleRounds(t *testing.T) {
	group := NewInstanceGroup("parent")
	adapter := &adversarialGroupAdapter{group: group}

	// Create round 1 and round 2 sub-groups
	adapter.GetOrCreateSubGroup("sub-round-1", "Round 1")
	adapter.GetOrCreateSubGroup("sub-round-2", "Round 2")

	if len(group.SubGroups) != 2 {
		t.Errorf("expected 2 sub-groups initially, got %d", len(group.SubGroups))
	}

	// Move round 1 under Previous Rounds
	adapter.MoveSubGroupUnder("sub-round-1", "sub-previous-rounds", "Previous Rounds")

	// Now should have Previous Rounds + Round 2
	if len(group.SubGroups) != 2 {
		t.Errorf("expected 2 sub-groups after first move, got %d", len(group.SubGroups))
	}

	// Move round 2 under Previous Rounds (which now exists)
	adapter.MoveSubGroupUnder("sub-round-2", "sub-previous-rounds", "Previous Rounds")

	// Now should have only Previous Rounds
	if len(group.SubGroups) != 1 {
		t.Errorf("expected 1 sub-group after second move, got %d", len(group.SubGroups))
	}

	// Previous Rounds should have both rounds
	previousRounds := group.SubGroups[0]
	if previousRounds.Name != "Previous Rounds" {
		t.Errorf("expected Previous Rounds, got %s", previousRounds.Name)
	}
	if len(previousRounds.SubGroups) != 2 {
		t.Errorf("expected 2 sub-groups under Previous Rounds, got %d", len(previousRounds.SubGroups))
	}
}

// TestAdversarialGroupAdapterMoveSubGroupUnder_NonExistent tests moving non-existent sub-group.
func TestAdversarialGroupAdapterMoveSubGroupUnder_NonExistent(t *testing.T) {
	group := NewInstanceGroup("parent")
	adapter := &adversarialGroupAdapter{group: group}

	// Try to move a non-existent sub-group
	result := adapter.MoveSubGroupUnder("nonexistent", "target", "Target")
	if result {
		t.Error("MoveSubGroupUnder should return false for non-existent sub-group")
	}
}

// TestRalphInterfaceSatisfaction verifies adapters satisfy their interfaces.
func TestRalphInterfaceSatisfaction(t *testing.T) {
	session := NewSession("test", "/repo")

	// Verify ralphSessionAdapter satisfies ralph.SessionInterface
	var _ ralph.SessionInterface = &ralphSessionAdapter{session: session}

	// Verify ralphGroupAdapter satisfies ralph.GroupInterface
	group := NewInstanceGroup("test")
	var _ ralph.GroupInterface = &ralphGroupAdapter{group: group}
}

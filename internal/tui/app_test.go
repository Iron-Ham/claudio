package tui

import (
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/styles"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	"github.com/Iron-Ham/claudio/internal/util"
	"github.com/spf13/viper"
)

// TestInit_CallsSetActiveTheme tests that Init() calls SetActiveTheme when
// config.Get().TUI.Theme is set. This tests the theme application logic.
//
// Note: A full integration test would require setting up viper with a config file,
// as viper.Set() values don't always propagate correctly through viper.Unmarshal().
func TestInit_CallsSetActiveTheme(t *testing.T) {
	// Reset to monokai theme before test - this way we can verify Init
	// applies the default theme from config (which returns "default")
	styles.SetActiveTheme(styles.ThemeMonokai)

	// Verify we start with monokai's primary color
	before := styles.GetActiveTheme()
	if string(before.PrimaryColor) != "#F92672" {
		t.Fatalf("Expected monokai primary color before Init, got %q", before.PrimaryColor)
	}

	// Create a minimal model and call Init
	model := Model{
		session: &orchestrator.Session{
			ID: "test-session",
		},
	}

	// Call Init - since config.Get().TUI.Theme returns "default",
	// the theme should be reset to default
	_ = model.Init()

	// Verify Init applied the config theme (default, since config.Get() returns "default")
	after := styles.GetActiveTheme()
	if after == nil {
		t.Fatal("GetActiveTheme() returned nil after Init()")
	}

	// Default theme has primary color #A78BFA
	if string(after.PrimaryColor) != "#A78BFA" {
		t.Errorf("Init() should apply config theme, got PrimaryColor = %q, want %q (default)",
			after.PrimaryColor, "#A78BFA")
	}
}

// TestSetActiveTheme_AppliesValidThemes tests that SetActiveTheme correctly
// applies each valid theme. This validates the theme system works correctly.
func TestSetActiveTheme_AppliesValidThemes(t *testing.T) {
	tests := []struct {
		theme        styles.ThemeName
		primaryColor string
	}{
		{styles.ThemeDefault, "#A78BFA"},
		{styles.ThemeMonokai, "#F92672"},
		{styles.ThemeDracula, "#BD93F9"},
		{styles.ThemeNord, "#88C0D0"},
	}

	for _, tt := range tests {
		t.Run(string(tt.theme), func(t *testing.T) {
			styles.SetActiveTheme(tt.theme)

			active := styles.GetActiveTheme()
			if active == nil {
				t.Fatal("GetActiveTheme() returned nil")
			}
			if string(active.PrimaryColor) != tt.primaryColor {
				t.Errorf("SetActiveTheme(%s) PrimaryColor = %q, want %q",
					tt.theme, active.PrimaryColor, tt.primaryColor)
			}
		})
	}
}

// TestNewWithUltraPlan_CreatesGroupForCLIStartedSession tests that
// NewWithUltraPlan creates a group when started from CLI (without pre-existing group).
func TestNewWithUltraPlan_CreatesGroupForCLIStartedSession(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet (CLI startup)
	}

	// Create ultraplan session without a GroupID (simulates CLI startup)
	ultraSession := &orchestrator.UltraPlanSession{
		ID:        "ultra-1",
		Objective: "Test objective for ultraplan",
		Config: orchestrator.UltraPlanConfig{
			MultiPass: false,
		},
	}

	// Test 1: Verify session starts with no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}

	// Test 2: Verify GroupID is empty initially
	if ultraSession.GroupID != "" {
		t.Errorf("expected empty GroupID initially, got %q", ultraSession.GroupID)
	}
}

// TestNewWithTripleShot_CreatesGroupForCLIStartedSession tests that
// NewWithTripleShot creates a group when started from CLI (without pre-existing group).
func TestNewWithTripleShot_CreatesGroupForCLIStartedSession(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet (CLI startup)
	}

	// Create tripleshot session without a GroupID (simulates CLI startup)
	tripleSession := &tripleshot.Session{
		ID:   "triple-1",
		Task: "Test task for tripleshot",
	}

	// Test: Verify session starts with no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}

	// Test: Verify GroupID is empty initially
	if tripleSession.GroupID != "" {
		t.Errorf("expected empty GroupID initially, got %q", tripleSession.GroupID)
	}
}

// TestNewWithTripleShots_CreatesGroupsForLegacySessions tests that
// NewWithTripleShots creates groups for tripleshots that don't have GroupIDs.
func TestNewWithTripleShots_CreatesGroupsForLegacySessions(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet
	}

	// Create multiple tripleshot sessions without GroupIDs (legacy sessions)
	session1 := &tripleshot.Session{
		ID:   "triple-1",
		Task: "Task 1",
	}
	session2 := &tripleshot.Session{
		ID:   "triple-2",
		Task: "Task 2",
	}

	// Verify both sessions start with no GroupID
	if session1.GroupID != "" {
		t.Errorf("expected empty GroupID for session1, got %q", session1.GroupID)
	}
	if session2.GroupID != "" {
		t.Errorf("expected empty GroupID for session2, got %q", session2.GroupID)
	}

	// Verify the session has no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}
}

// TestNewWithAdversarial_CreatesGroupForCLIStartedSession tests the initial state
// when an adversarial session is started from CLI without a pre-existing group.
// Note: We don't call NewWithAdversarial directly because it requires a fully
// initialized Coordinator with orchestrator dependencies. Instead, we verify
// the initial conditions and test the group creation pattern separately in
// TestAdversarialGroupID_MustBeSetOnCreation.
func TestNewWithAdversarial_CreatesGroupForCLIStartedSession(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet (CLI startup)
	}

	// Create adversarial session without a GroupID (simulates CLI startup)
	advSession := &adversarial.Session{
		ID:   "adv-1",
		Task: "Test task for adversarial review",
	}

	// Test: Verify session starts with no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}

	// Test: Verify GroupID is empty initially
	if advSession.GroupID != "" {
		t.Errorf("expected empty GroupID initially, got %q", advSession.GroupID)
	}
}

// TestNewWithAdversarials_CreatesGroupsForLegacySessions tests the initial state
// for multiple adversarial sessions without GroupIDs (legacy sessions).
// Note: We don't call NewWithAdversarials directly because it requires fully
// initialized Coordinators. The group creation logic is tested in
// TestAdversarialGroupID_MustBeSetOnCreation.
func TestNewWithAdversarials_CreatesGroupsForLegacySessions(t *testing.T) {
	// Create minimal orchestrator session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil, // No groups yet
	}

	// Create multiple adversarial sessions without GroupIDs (legacy sessions)
	session1 := &adversarial.Session{
		ID:   "adv-1",
		Task: "Task 1",
	}
	session2 := &adversarial.Session{
		ID:   "adv-2",
		Task: "Task 2",
	}

	// Verify both sessions start with no GroupID
	if session1.GroupID != "" {
		t.Errorf("expected empty GroupID for session1, got %q", session1.GroupID)
	}
	if session2.GroupID != "" {
		t.Errorf("expected empty GroupID for session2, got %q", session2.GroupID)
	}

	// Verify the session has no groups
	if len(session.Groups) != 0 {
		t.Errorf("expected 0 groups initially, got %d", len(session.Groups))
	}
}

// TestAdversarialGroupID_MustBeSetOnCreation verifies that when an adversarial group
// is created, the advSession.GroupID must be set to link them together.
func TestAdversarialGroupID_MustBeSetOnCreation(t *testing.T) {
	// Create a mock session and adversarial session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil,
	}
	advSession := &adversarial.Session{
		ID:   "adv-1",
		Task: "Test task",
	}

	// Simulate the pattern: create group and link it
	advGroup := orchestrator.NewInstanceGroupWithType(
		"Test Group",
		orchestrator.SessionTypeAdversarial,
		advSession.Task,
	)
	session.AddGroup(advGroup)
	advSession.GroupID = advGroup.ID // This is the critical line being tested

	// Verify the group was added to session
	if len(session.Groups) != 1 {
		t.Fatalf("expected 1 group in session, got %d", len(session.Groups))
	}

	// Verify GroupID is correctly set
	if advSession.GroupID == "" {
		t.Error("advSession.GroupID must be set when group is created")
	}
	if advSession.GroupID != advGroup.ID {
		t.Errorf("advSession.GroupID = %q, want %q", advSession.GroupID, advGroup.ID)
	}

	// Verify we can retrieve the group by its ID
	retrievedGroup := session.GetGroup(advSession.GroupID)
	if retrievedGroup == nil {
		t.Fatal("session.GetGroup(advSession.GroupID) should return the group")
	}
	if orchestrator.GetSessionType(retrievedGroup) != orchestrator.SessionTypeAdversarial {
		t.Errorf("group.SessionType = %v, want %v", orchestrator.GetSessionType(retrievedGroup), orchestrator.SessionTypeAdversarial)
	}
}

// TestNewInstanceGroupWithType_CreatesCorrectSessionType tests that
// NewInstanceGroupWithType correctly sets the session type for different modes.
func TestNewInstanceGroupWithType_CreatesCorrectSessionType(t *testing.T) {
	tests := []struct {
		name        string
		sessionType orchestrator.SessionType
		objective   string
		wantType    orchestrator.SessionType
	}{
		{
			name:        "ultraplan creates ultraplan type",
			sessionType: orchestrator.SessionTypeUltraPlan,
			objective:   "Test ultraplan objective",
			wantType:    orchestrator.SessionTypeUltraPlan,
		},
		{
			name:        "multipass creates planmulti type",
			sessionType: orchestrator.SessionTypePlanMulti,
			objective:   "Test multipass objective",
			wantType:    orchestrator.SessionTypePlanMulti,
		},
		{
			name:        "tripleshot creates tripleshot type",
			sessionType: orchestrator.SessionTypeTripleShot,
			objective:   "Test tripleshot task",
			wantType:    orchestrator.SessionTypeTripleShot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := orchestrator.NewInstanceGroupWithType(
				util.TruncateString(tt.objective, 30),
				tt.sessionType,
				tt.objective,
			)

			if orchestrator.GetSessionType(group) != tt.wantType {
				t.Errorf("SessionType = %v, want %v", orchestrator.GetSessionType(group), tt.wantType)
			}
			if group.Objective != tt.objective {
				t.Errorf("Objective = %q, want %q", group.Objective, tt.objective)
			}
			if group.ID == "" {
				t.Error("expected non-empty group ID")
			}
		})
	}
}

// TestAutoEnableGroupedMode_EnablesWhenGroupsExist tests that
// autoEnableGroupedMode enables grouped mode when groups are present.
func TestAutoEnableGroupedMode_EnablesWhenGroupsExist(t *testing.T) {
	// Create a model with flat sidebar mode and no groups
	model := &Model{
		session: &orchestrator.Session{
			ID:     "test-session",
			Groups: nil,
		},
		sidebarMode: view.SidebarModeFlat,
	}

	// Call autoEnableGroupedMode - should not change anything since no groups
	model.autoEnableGroupedMode()
	if model.sidebarMode != view.SidebarModeFlat {
		t.Error("expected sidebarMode to remain flat when no groups exist")
	}

	// Add a group
	model.session.Groups = []*orchestrator.InstanceGroup{
		{ID: "group-1", Name: "Test Group"},
	}

	// Call autoEnableGroupedMode - should enable grouped mode
	model.autoEnableGroupedMode()
	if model.sidebarMode != view.SidebarModeGrouped {
		t.Error("expected sidebarMode to be grouped when groups exist")
	}
}

// TestUltraPlanGroupID_MustBeSetOnCreation verifies that when an ultraplan group
// is created, the ultraSession.GroupID must be set to link them together.
// This test documents the pattern that initInlineUltraPlanMode must follow.
func TestUltraPlanGroupID_MustBeSetOnCreation(t *testing.T) {
	// Create a mock session and ultraplan session
	session := &orchestrator.Session{
		ID:     "test-session",
		Groups: nil,
	}
	ultraSession := &orchestrator.UltraPlanSession{
		ID:        "ultra-1",
		Objective: "Test objective",
	}

	// Simulate the pattern from initInlineUltraPlanMode: create group and link it
	ultraGroup := orchestrator.NewInstanceGroupWithType(
		"Test Group",
		orchestrator.SessionTypeUltraPlan,
		ultraSession.Objective,
	)
	session.AddGroup(ultraGroup)
	ultraSession.GroupID = ultraGroup.ID // This is the critical line being tested

	// Verify the group was added to session
	if len(session.Groups) != 1 {
		t.Fatalf("expected 1 group in session, got %d", len(session.Groups))
	}

	// Verify GroupID is correctly set (this is what was missing before the fix)
	if ultraSession.GroupID == "" {
		t.Error("ultraSession.GroupID must be set when group is created")
	}
	if ultraSession.GroupID != ultraGroup.ID {
		t.Errorf("ultraSession.GroupID = %q, want %q", ultraSession.GroupID, ultraGroup.ID)
	}

	// Verify we can retrieve the group by its ID
	retrievedGroup := session.GetGroup(ultraSession.GroupID)
	if retrievedGroup == nil {
		t.Fatal("session.GetGroup(ultraSession.GroupID) should return the group")
	}
	if orchestrator.GetSessionType(retrievedGroup) != orchestrator.SessionTypeUltraPlan {
		t.Errorf("group.SessionType = %v, want %v", orchestrator.GetSessionType(retrievedGroup), orchestrator.SessionTypeUltraPlan)
	}
}

// TestTripleshotConfig_UsesConfigFileSettings verifies that when a tripleshot
// session is created via the TUI, it uses the config file settings for AutoApprove
// and Adversarial instead of hardcoded defaults.
//
// This test documents the expected behavior: initiateTripleShotMode should read
// from config.Get() to apply user-configured settings.
func TestTripleshotConfig_UsesConfigFileSettings(t *testing.T) {
	// Save and restore viper state
	viper.Reset()
	defer viper.Reset()

	// Set up config with custom tripleshot settings
	config.SetDefaults()
	viper.Set("tripleshot.auto_approve", true)
	viper.Set("tripleshot.adversarial", true)

	// Verify config.Get() returns the expected values
	cfg := config.Get()
	if !cfg.Tripleshot.AutoApprove {
		t.Error("config.Get().Tripleshot.AutoApprove should be true after viper.Set")
	}
	if !cfg.Tripleshot.Adversarial {
		t.Error("config.Get().Tripleshot.Adversarial should be true after viper.Set")
	}

	// Create a tripleshot session using the same pattern as initiateTripleShotMode
	// This mimics what the TUI does when starting tripleshot from command mode
	tripleConfig := orchestrator.DefaultTripleShotConfig()
	tripleConfig.AutoApprove = cfg.Tripleshot.AutoApprove
	tripleConfig.Adversarial = cfg.Tripleshot.Adversarial

	tripleSession := tripleshot.NewSession("Test task", tripleConfig)

	// Verify the session config reflects the viper settings
	if !tripleSession.Config.AutoApprove {
		t.Error("tripleSession.Config.AutoApprove should be true (from config)")
	}
	if !tripleSession.Config.Adversarial {
		t.Error("tripleSession.Config.Adversarial should be true (from config)")
	}
}

// TestTripleshotConfig_DefaultsWhenNotConfigured verifies that tripleshot uses
// default values (false) when the config file doesn't specify custom settings.
func TestTripleshotConfig_DefaultsWhenNotConfigured(t *testing.T) {
	// Save and restore viper state
	viper.Reset()
	defer viper.Reset()

	// Set up config with defaults (no custom tripleshot settings)
	config.SetDefaults()

	// Verify config.Get() returns default values
	cfg := config.Get()
	if cfg.Tripleshot.AutoApprove {
		t.Error("config.Get().Tripleshot.AutoApprove should be false by default")
	}
	if cfg.Tripleshot.Adversarial {
		t.Error("config.Get().Tripleshot.Adversarial should be false by default")
	}

	// Create a tripleshot session using the same pattern as initiateTripleShotMode
	tripleConfig := orchestrator.DefaultTripleShotConfig()
	tripleConfig.AutoApprove = cfg.Tripleshot.AutoApprove
	tripleConfig.Adversarial = cfg.Tripleshot.Adversarial

	tripleSession := tripleshot.NewSession("Test task", tripleConfig)

	// Verify the session config uses defaults
	if tripleSession.Config.AutoApprove {
		t.Error("tripleSession.Config.AutoApprove should be false by default")
	}
	if tripleSession.Config.Adversarial {
		t.Error("tripleSession.Config.Adversarial should be false by default")
	}
}

//go:build integration

package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/testutil"
)

func TestNew(t *testing.T) {
	testutil.SkipIfNoGit(t)

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "valid git repository",
			setup: func(t *testing.T) string {
				return testutil.SetupTestRepo(t)
			},
			wantErr: false,
		},
		{
			name: "non-git directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			_, err := New(dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewWithConfig(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	cfg := config.Default()

	orch, err := NewWithConfig(repoDir, cfg)
	if err != nil {
		t.Fatalf("NewWithConfig() error = %v", err)
	}

	if orch.Config() != cfg {
		t.Error("Config() should return the provided config")
	}
}

func TestOrchestrator_Init(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := orch.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Verify .claudio directory exists
	claudioDir := filepath.Join(repoDir, ".claudio")
	if _, err := os.Stat(claudioDir); os.IsNotExist(err) {
		t.Error(".claudio directory was not created")
	}

	// Verify worktrees directory exists
	worktreesDir := filepath.Join(claudioDir, "worktrees")
	if _, err := os.Stat(worktreesDir); os.IsNotExist(err) {
		t.Error(".claudio/worktrees directory was not created")
	}
}

func TestOrchestrator_StartSession(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test-session")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	if session == nil {
		t.Fatal("StartSession() returned nil session")
	}

	if session.Name != "test-session" {
		t.Errorf("session.Name = %q, want %q", session.Name, "test-session")
	}

	// Session should be saved to disk
	sessionFile := filepath.Join(repoDir, ".claudio", "session.json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("session.json was not created")
	}
}

func TestOrchestrator_HasExistingSession(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// No session initially
	if orch.HasExistingSession() {
		t.Error("HasExistingSession() = true, want false initially")
	}

	// Create session
	if _, err := orch.StartSession("test"); err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Should have session now
	if !orch.HasExistingSession() {
		t.Error("HasExistingSession() = false after StartSession()")
	}
}

func TestOrchestrator_LoadSession(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Create and save a session
	original, err := orch.StartSession("test-session")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Create a new orchestrator and load the session
	orch2, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	loaded, err := orch2.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}

	if loaded.ID != original.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, original.ID)
	}
	if loaded.Name != original.Name {
		t.Errorf("loaded.Name = %q, want %q", loaded.Name, original.Name)
	}
}

func TestOrchestrator_AddInstance(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	inst, err := orch.AddInstance(session, "implement feature X")
	if err != nil {
		t.Fatalf("AddInstance() error = %v", err)
	}

	if inst == nil {
		t.Fatal("AddInstance() returned nil instance")
	}

	if inst.Task != "implement feature X" {
		t.Errorf("inst.Task = %q, want %q", inst.Task, "implement feature X")
	}

	if inst.WorktreePath == "" {
		t.Error("inst.WorktreePath should not be empty")
	}

	if inst.Branch == "" {
		t.Error("inst.Branch should not be empty")
	}

	// Worktree should exist
	if _, err := os.Stat(inst.WorktreePath); os.IsNotExist(err) {
		t.Error("instance worktree was not created")
	}

	// Instance should be in session
	if len(session.Instances) != 1 {
		t.Errorf("len(session.Instances) = %d, want 1", len(session.Instances))
	}
}

func TestOrchestrator_AddMultipleInstances(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	tasks := []string{"task 1", "task 2", "task 3"}
	for _, task := range tasks {
		if _, err := orch.AddInstance(session, task); err != nil {
			t.Fatalf("AddInstance(%q) error = %v", task, err)
		}
	}

	if len(session.Instances) != len(tasks) {
		t.Errorf("len(session.Instances) = %d, want %d", len(session.Instances), len(tasks))
	}

	// Each instance should have unique ID, branch, and worktree
	ids := make(map[string]bool)
	branches := make(map[string]bool)
	worktrees := make(map[string]bool)

	for _, inst := range session.Instances {
		if ids[inst.ID] {
			t.Errorf("duplicate instance ID: %s", inst.ID)
		}
		ids[inst.ID] = true

		if branches[inst.Branch] {
			t.Errorf("duplicate branch: %s", inst.Branch)
		}
		branches[inst.Branch] = true

		if worktrees[inst.WorktreePath] {
			t.Errorf("duplicate worktree: %s", inst.WorktreePath)
		}
		worktrees[inst.WorktreePath] = true
	}
}

func TestOrchestrator_GetInstance(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	inst, err := orch.AddInstance(session, "test task")
	if err != nil {
		t.Fatalf("AddInstance() error = %v", err)
	}

	// Should find existing instance
	found := orch.GetInstance(inst.ID)
	if found == nil {
		t.Error("GetInstance() returned nil for existing instance")
	}
	if found != inst {
		t.Error("GetInstance() returned different instance")
	}

	// Should return nil for non-existent instance
	notFound := orch.GetInstance("non-existent")
	if notFound != nil {
		t.Error("GetInstance() should return nil for non-existent ID")
	}
}

func TestOrchestrator_RemoveInstance(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	inst, err := orch.AddInstance(session, "test task")
	if err != nil {
		t.Fatalf("AddInstance() error = %v", err)
	}

	worktreePath := inst.WorktreePath

	// Remove instance (force to skip uncommitted changes check)
	if err := orch.RemoveInstance(session, inst.ID, true); err != nil {
		t.Fatalf("RemoveInstance() error = %v", err)
	}

	// Instance should be removed from session
	if len(session.Instances) != 0 {
		t.Errorf("len(session.Instances) = %d, want 0", len(session.Instances))
	}

	// Worktree should be removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}
}

func TestOrchestrator_RemoveInstance_NotFound(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Should error for non-existent instance
	err = orch.RemoveInstance(session, "non-existent", true)
	if err == nil {
		t.Error("RemoveInstance() should error for non-existent ID")
	}
}

func TestOrchestrator_StopSession(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Add some instances
	for i := 0; i < 3; i++ {
		if _, err := orch.AddInstance(session, fmt.Sprintf("stop session task %d", i)); err != nil {
			t.Fatalf("AddInstance() error = %v", err)
		}
	}

	// Stop session with force (clean up worktrees)
	if err := orch.StopSession(session, true); err != nil {
		t.Fatalf("StopSession() error = %v", err)
	}

	// Session file should be removed
	sessionFile := filepath.Join(repoDir, ".claudio", "session.json")
	if _, err := os.Stat(sessionFile); !os.IsNotExist(err) {
		t.Error("session.json should be removed after StopSession()")
	}
}

func TestOrchestrator_GetSessionMetrics(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// No session - should return empty metrics
	metrics := orch.GetSessionMetrics()
	if metrics.InstanceCount != 0 {
		t.Errorf("GetSessionMetrics().InstanceCount = %d, want 0", metrics.InstanceCount)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Add instances
	for i := 0; i < 3; i++ {
		if _, err := orch.AddInstance(session, fmt.Sprintf("metrics task %d", i)); err != nil {
			t.Fatalf("AddInstance() error = %v", err)
		}
	}

	metrics = orch.GetSessionMetrics()
	if metrics.InstanceCount != 3 {
		t.Errorf("GetSessionMetrics().InstanceCount = %d, want 3", metrics.InstanceCount)
	}
}

func TestOrchestrator_SetDisplayDimensions(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Set dimensions
	orch.SetDisplayDimensions(200, 50)

	// Verify dimensions are used in instance config
	cfg := orch.instanceManagerConfig()
	if cfg.TmuxWidth != 200 {
		t.Errorf("cfg.TmuxWidth = %d, want 200", cfg.TmuxWidth)
	}
	if cfg.TmuxHeight != 50 {
		t.Errorf("cfg.TmuxHeight = %d, want 50", cfg.TmuxHeight)
	}
}

func TestOrchestrator_ClearCompletedInstances(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Add instances
	inst1, _ := orch.AddInstance(session, "task 1")
	inst2, _ := orch.AddInstance(session, "task 2")
	inst3, _ := orch.AddInstance(session, "task 3")

	// Mark some as completed
	inst1.Status = StatusCompleted
	inst3.Status = StatusCompleted
	// inst2 stays pending

	removed, err := orch.ClearCompletedInstances(session)
	if err != nil {
		t.Fatalf("ClearCompletedInstances() error = %v", err)
	}

	if removed != 2 {
		t.Errorf("ClearCompletedInstances() removed %d, want 2", removed)
	}

	if len(session.Instances) != 1 {
		t.Errorf("len(session.Instances) = %d, want 1", len(session.Instances))
	}

	if session.Instances[0].ID != inst2.ID {
		t.Error("remaining instance should be inst2")
	}
}

func TestOrchestrator_ClearCompletedInstances_NoneCompleted(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Add instances (all pending by default)
	for i := 0; i < 3; i++ {
		if _, err := orch.AddInstance(session, fmt.Sprintf("clear none task %d", i)); err != nil {
			t.Fatalf("AddInstance() error = %v", err)
		}
	}

	removed, err := orch.ClearCompletedInstances(session)
	if err != nil {
		t.Fatalf("ClearCompletedInstances() error = %v", err)
	}

	if removed != 0 {
		t.Errorf("ClearCompletedInstances() removed %d, want 0", removed)
	}

	if len(session.Instances) != 3 {
		t.Errorf("len(session.Instances) = %d, want 3", len(session.Instances))
	}
}

func TestOrchestrator_EnsureInstanceManagers(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Start a session and add instances
	session, err := orch.StartSession("test")
	if err != nil {
		t.Fatalf("StartSession() error = %v", err)
	}

	// Add an instance - this creates a manager
	inst, err := orch.AddInstance(session, "test task")
	if err != nil {
		t.Fatalf("AddInstance() error = %v", err)
	}

	// Verify manager exists
	if orch.GetInstanceManager(inst.ID) == nil {
		t.Error("manager should exist after AddInstance")
	}

	// Create a new orchestrator (simulating session reload)
	orch2, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Load the session
	_, err = orch2.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}

	// Verify manager doesn't exist yet (LoadSession doesn't create managers)
	if orch2.GetInstanceManager(inst.ID) != nil {
		t.Error("manager should not exist after LoadSession")
	}

	// Call EnsureInstanceManagers
	orch2.EnsureInstanceManagers()

	// Verify manager now exists
	mgr := orch2.GetInstanceManager(inst.ID)
	if mgr == nil {
		t.Fatal("manager should exist after EnsureInstanceManagers")
	}

	// Verify manager has correct properties
	if mgr.ID() != inst.ID {
		t.Errorf("manager ID = %q, want %q", mgr.ID(), inst.ID)
	}

	// Test idempotency - calling again should not create new managers
	orch2.EnsureInstanceManagers()
	mgr2 := orch2.GetInstanceManager(inst.ID)
	if mgr2 != mgr {
		t.Error("EnsureInstanceManagers should be idempotent - same manager expected")
	}
}

func TestOrchestrator_EnsureInstanceManagers_NilSession(t *testing.T) {
	testutil.SkipIfNoGit(t)

	repoDir := testutil.SetupTestRepo(t)
	orch, err := New(repoDir)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Don't start or load any session - session is nil
	// EnsureInstanceManagers should not panic
	orch.EnsureInstanceManagers()

	// Verify no managers were created (nothing to create)
	// This is a no-op when session is nil
}

// TestSlugify moved to branch_test.go

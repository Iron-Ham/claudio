package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	rootsession "github.com/Iron-Ham/claudio/internal/session"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir: tempDir,
	})

	if mgr.baseDir != tempDir {
		t.Errorf("baseDir = %q, want %q", mgr.baseDir, tempDir)
	}
	if mgr.claudioDir != filepath.Join(tempDir, ".claudio") {
		t.Errorf("claudioDir = %q, want %q", mgr.claudioDir, filepath.Join(tempDir, ".claudio"))
	}
	if mgr.sessionID != "" {
		t.Errorf("sessionID should be empty for legacy mode, got %q", mgr.sessionID)
	}
	if mgr.sessionDir != "" {
		t.Errorf("sessionDir should be empty for legacy mode, got %q", mgr.sessionDir)
	}
}

func TestNewManager_MultiSession(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session-123",
	})

	if mgr.sessionID != "test-session-123" {
		t.Errorf("sessionID = %q, want %q", mgr.sessionID, "test-session-123")
	}
	expectedSessionDir := rootsession.GetSessionDir(tempDir, "test-session-123")
	if mgr.sessionDir != expectedSessionDir {
		t.Errorf("sessionDir = %q, want %q", mgr.sessionDir, expectedSessionDir)
	}
}

func TestManager_SessionFilePath(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		sessionID  string
		wantSuffix string
	}{
		{
			name:       "legacy mode",
			sessionID:  "",
			wantSuffix: ".claudio/session.json",
		},
		{
			name:       "multi-session mode",
			sessionID:  "test-session",
			wantSuffix: ".claudio/sessions/test-session/session.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager(Config{
				BaseDir:   tempDir,
				SessionID: tt.sessionID,
			})
			got := mgr.SessionFilePath()
			want := filepath.Join(tempDir, tt.wantSuffix)
			if got != want {
				t.Errorf("SessionFilePath() = %q, want %q", got, want)
			}
		})
	}
}

func TestManager_Init(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Check .claudio directory was created
	claudioDir := filepath.Join(tempDir, ".claudio")
	if _, err := os.Stat(claudioDir); os.IsNotExist(err) {
		t.Error(".claudio directory was not created")
	}

	// Check session directory was created
	if _, err := os.Stat(mgr.sessionDir); os.IsNotExist(err) {
		t.Error("session directory was not created")
	}
}

func TestManager_CreateAndLoadSession(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Create a session
	sess, err := mgr.CreateSession("Test Session", tempDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if sess.Name != "Test Session" {
		t.Errorf("sess.Name = %q, want %q", sess.Name, "Test Session")
	}
	if sess.BaseRepo != tempDir {
		t.Errorf("sess.BaseRepo = %q, want %q", sess.BaseRepo, tempDir)
	}
	if sess.ID != "test-session" {
		t.Errorf("sess.ID = %q, want %q", sess.ID, "test-session")
	}

	// Release lock and load session
	if err := mgr.ReleaseLock(); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	loaded, err := mgr.LoadSessionWithLock()
	if err != nil {
		t.Fatalf("LoadSessionWithLock() error = %v", err)
	}

	if loaded.Name != "Test Session" {
		t.Errorf("loaded.Name = %q, want %q", loaded.Name, "Test Session")
	}
	if loaded.ID != "test-session" {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, "test-session")
	}
}

func TestManager_SaveAndLoadSession_WithGroups(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Create a session with instances and groups
	sess, err := mgr.CreateSession("Test Session", tempDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Add instances
	inst1 := NewInstanceData("Task 1")
	inst2 := NewInstanceData("Task 2")
	inst3 := NewInstanceData("Task 3")
	sess.Instances = []*InstanceData{inst1, inst2, inst3}

	// Add groups with sub-groups
	now := time.Now()
	group1 := &GroupData{
		ID:             "group-1",
		Name:           "Foundation",
		Phase:          "executing",
		Instances:      []string{inst1.ID},
		SubGroups:      make([]*GroupData, 0),
		ExecutionOrder: 0,
		DependsOn:      make([]string, 0),
		Created:        now,
	}

	subGroup := &GroupData{
		ID:             "sub-group-1",
		Name:           "Sub-tasks",
		Phase:          "pending",
		Instances:      []string{inst2.ID},
		SubGroups:      make([]*GroupData, 0),
		ParentID:       "group-2",
		ExecutionOrder: 0,
		DependsOn:      make([]string, 0),
		Created:        now,
	}

	group2 := &GroupData{
		ID:             "group-2",
		Name:           "Features",
		Phase:          "pending",
		Instances:      []string{inst3.ID},
		SubGroups:      []*GroupData{subGroup},
		ExecutionOrder: 1,
		DependsOn:      []string{"group-1"},
		Created:        now,
	}

	sess.Groups = []*GroupData{group1, group2}

	// Save session
	if err := mgr.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	// Release lock and load session
	if err := mgr.ReleaseLock(); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	loaded, err := mgr.LoadSessionWithLock()
	if err != nil {
		t.Fatalf("LoadSessionWithLock() error = %v", err)
	}

	// Verify groups were persisted
	if len(loaded.Groups) != 2 {
		t.Fatalf("len(loaded.Groups) = %d, want 2", len(loaded.Groups))
	}

	// Verify group1
	loadedGroup1 := loaded.GetGroup("group-1")
	if loadedGroup1 == nil {
		t.Fatal("group-1 not found after load")
	}
	if loadedGroup1.Name != "Foundation" {
		t.Errorf("loadedGroup1.Name = %q, want %q", loadedGroup1.Name, "Foundation")
	}
	if loadedGroup1.Phase != "executing" {
		t.Errorf("loadedGroup1.Phase = %q, want %q", loadedGroup1.Phase, "executing")
	}
	if len(loadedGroup1.Instances) != 1 || loadedGroup1.Instances[0] != inst1.ID {
		t.Errorf("loadedGroup1.Instances = %v, want [%s]", loadedGroup1.Instances, inst1.ID)
	}

	// Verify group2 with sub-group
	loadedGroup2 := loaded.GetGroup("group-2")
	if loadedGroup2 == nil {
		t.Fatal("group-2 not found after load")
	}
	if len(loadedGroup2.SubGroups) != 1 {
		t.Fatalf("len(loadedGroup2.SubGroups) = %d, want 1", len(loadedGroup2.SubGroups))
	}
	if loadedGroup2.SubGroups[0].ID != "sub-group-1" {
		t.Errorf("subGroup.ID = %q, want %q", loadedGroup2.SubGroups[0].ID, "sub-group-1")
	}

	// Verify dependencies
	if len(loadedGroup2.DependsOn) != 1 || loadedGroup2.DependsOn[0] != "group-1" {
		t.Errorf("loadedGroup2.DependsOn = %v, want [%s]", loadedGroup2.DependsOn, "group-1")
	}
}

func TestManager_Exists(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Session should not exist initially
	if mgr.Exists() {
		t.Error("Exists() returned true before session was created")
	}

	// Create session
	_, err := mgr.CreateSession("Test", tempDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Session should exist now
	if !mgr.Exists() {
		t.Error("Exists() returned false after session was created")
	}
}

func TestManager_DeleteSession(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Create session
	_, err := mgr.CreateSession("Test", tempDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if !mgr.Exists() {
		t.Fatal("session should exist after creation")
	}

	// Delete session
	if err := mgr.DeleteSession(); err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	if mgr.Exists() {
		t.Error("session should not exist after deletion")
	}
}

func TestManager_WriteAndLoadContext(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Initialize directories
	if err := mgr.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	// Write context
	contextContent := "# Test Context\n\nThis is test content."
	if err := mgr.WriteContext(contextContent); err != nil {
		t.Fatalf("WriteContext() error = %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(mgr.ContextFilePath())
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != contextContent {
		t.Errorf("context = %q, want %q", string(data), contextContent)
	}
}

func TestSessionData_GetGroup(t *testing.T) {
	sess := &SessionData{
		Groups: []*GroupData{
			{
				ID:   "group-1",
				Name: "Group 1",
				SubGroups: []*GroupData{
					{
						ID:       "sub-1",
						Name:     "Sub Group 1",
						ParentID: "group-1",
					},
				},
			},
			{
				ID:   "group-2",
				Name: "Group 2",
			},
		},
	}

	tests := []struct {
		name    string
		id      string
		wantNil bool
		want    string
	}{
		{"top-level group", "group-1", false, "Group 1"},
		{"another top-level", "group-2", false, "Group 2"},
		{"sub-group", "sub-1", false, "Sub Group 1"},
		{"non-existent", "group-999", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sess.GetGroup(tt.id)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetGroup(%q) = %v, want nil", tt.id, got)
				}
			} else {
				if got == nil {
					t.Fatalf("GetGroup(%q) = nil, want %q", tt.id, tt.want)
				}
				if got.Name != tt.want {
					t.Errorf("GetGroup(%q).Name = %q, want %q", tt.id, got.Name, tt.want)
				}
			}
		})
	}
}

func TestSessionData_ValidateGroups_EmptyGroups(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
		},
		Groups: nil,
	}

	validated := sess.ValidateGroups()
	if validated != nil {
		t.Errorf("ValidateGroups() on nil groups = %v, want nil", validated)
	}
}

func TestSessionData_ValidateGroups_RemovesInvalidInstances(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
		},
		Groups: []*GroupData{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Instances: []string{"inst-1", "inst-deleted", "inst-2"},
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 1 {
		t.Fatalf("len(validated) = %d, want 1", len(validated))
	}

	// Should only have valid instances
	if len(validated[0].Instances) != 2 {
		t.Errorf("len(validated[0].Instances) = %d, want 2", len(validated[0].Instances))
	}
	for _, id := range validated[0].Instances {
		if id == "inst-deleted" {
			t.Error("invalid instance ID 'inst-deleted' should have been removed")
		}
	}
}

func TestSessionData_ValidateGroups_RemovesEmptyGroups(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
		},
		Groups: []*GroupData{
			{
				ID:        "group-1",
				Name:      "Non-empty Group",
				Instances: []string{"inst-1"},
			},
			{
				ID:        "group-2",
				Name:      "Empty Group",
				Instances: []string{"inst-deleted"}, // Will become empty after validation
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 1 {
		t.Fatalf("len(validated) = %d, want 1 (empty group should be removed)", len(validated))
	}

	if validated[0].ID != "group-1" {
		t.Errorf("validated[0].ID = %q, want %q", validated[0].ID, "group-1")
	}
}

func TestSessionData_ValidateGroups_PreservesGroupWithSubGroups(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
		},
		Groups: []*GroupData{
			{
				ID:        "parent",
				Name:      "Parent Group",
				Instances: []string{}, // Empty directly, but has sub-group with instances
				SubGroups: []*GroupData{
					{
						ID:        "child",
						Name:      "Child Group",
						Instances: []string{"inst-1"},
						ParentID:  "parent",
					},
				},
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 1 {
		t.Fatalf("len(validated) = %d, want 1 (parent with valid sub-group should be preserved)", len(validated))
	}

	if len(validated[0].SubGroups) != 1 {
		t.Errorf("len(validated[0].SubGroups) = %d, want 1", len(validated[0].SubGroups))
	}
}

func TestSessionData_ValidateGroups_RemovesInvalidDependencies(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
		},
		Groups: []*GroupData{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Instances: []string{"inst-1"},
				DependsOn: []string{},
			},
			{
				ID:        "group-2",
				Name:      "Group 2",
				Instances: []string{"inst-2"},
				DependsOn: []string{"group-1", "non-existent-group"},
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 2 {
		t.Fatalf("len(validated) = %d, want 2", len(validated))
	}

	// Find group-2 and check its dependencies
	var group2 *GroupData
	for _, g := range validated {
		if g.ID == "group-2" {
			group2 = g
			break
		}
	}

	if group2 == nil {
		t.Fatal("group-2 not found in validated groups")
	}

	// Should only have valid dependency
	if len(group2.DependsOn) != 1 {
		t.Errorf("len(group2.DependsOn) = %d, want 1", len(group2.DependsOn))
	}
	if group2.DependsOn[0] != "group-1" {
		t.Errorf("group2.DependsOn[0] = %q, want %q", group2.DependsOn[0], "group-1")
	}
}

func TestSessionData_ValidateGroups_DeeplyNested(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
			{ID: "inst-2", Task: "Task 2"},
			{ID: "inst-3", Task: "Task 3"},
		},
		Groups: []*GroupData{
			{
				ID:        "level-0",
				Name:      "Root",
				Instances: []string{"inst-deleted"},
				SubGroups: []*GroupData{
					{
						ID:        "level-1",
						Name:      "Level 1",
						Instances: []string{"inst-1"},
						ParentID:  "level-0",
						SubGroups: []*GroupData{
							{
								ID:        "level-2",
								Name:      "Level 2",
								Instances: []string{"inst-2", "inst-also-deleted"},
								ParentID:  "level-1",
								SubGroups: []*GroupData{
									{
										ID:        "level-3",
										Name:      "Level 3",
										Instances: []string{"inst-3"},
										ParentID:  "level-2",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 1 {
		t.Fatalf("len(validated) = %d, want 1", len(validated))
	}

	// Root group should have empty Instances but valid SubGroups
	root := validated[0]
	if len(root.Instances) != 0 {
		t.Errorf("root.Instances should be empty after removing invalid instance")
	}
	if len(root.SubGroups) != 1 {
		t.Fatal("root should have one sub-group")
	}

	// Level 1 should have inst-1
	level1 := root.SubGroups[0]
	if len(level1.Instances) != 1 || level1.Instances[0] != "inst-1" {
		t.Errorf("level1.Instances = %v, want [inst-1]", level1.Instances)
	}

	// Level 2 should have inst-2 only (inst-also-deleted removed)
	if len(level1.SubGroups) != 1 {
		t.Fatal("level1 should have one sub-group")
	}
	level2 := level1.SubGroups[0]
	if len(level2.Instances) != 1 || level2.Instances[0] != "inst-2" {
		t.Errorf("level2.Instances = %v, want [inst-2]", level2.Instances)
	}

	// Level 3 should have inst-3
	if len(level2.SubGroups) != 1 {
		t.Fatal("level2 should have one sub-group")
	}
	level3 := level2.SubGroups[0]
	if len(level3.Instances) != 1 || level3.Instances[0] != "inst-3" {
		t.Errorf("level3.Instances = %v, want [inst-3]", level3.Instances)
	}
}

func TestSessionData_ValidateGroups_PreservesAllFields(t *testing.T) {
	now := time.Now()
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
		},
		Groups: []*GroupData{
			{
				ID:             "group-1",
				Name:           "Test Group",
				Phase:          "executing",
				Instances:      []string{"inst-1"},
				ExecutionOrder: 5,
				DependsOn:      []string{},
				Created:        now,
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 1 {
		t.Fatalf("len(validated) = %d, want 1", len(validated))
	}

	g := validated[0]
	if g.ID != "group-1" {
		t.Errorf("ID = %q, want %q", g.ID, "group-1")
	}
	if g.Name != "Test Group" {
		t.Errorf("Name = %q, want %q", g.Name, "Test Group")
	}
	if g.Phase != "executing" {
		t.Errorf("Phase = %q, want %q", g.Phase, "executing")
	}
	if g.ExecutionOrder != 5 {
		t.Errorf("ExecutionOrder = %d, want 5", g.ExecutionOrder)
	}
	if !g.Created.Equal(now) {
		t.Errorf("Created = %v, want %v", g.Created, now)
	}
}

func TestNewGroupData(t *testing.T) {
	group := NewGroupData("Test Group")

	if group.Name != "Test Group" {
		t.Errorf("Name = %q, want %q", group.Name, "Test Group")
	}
	if group.Phase != "pending" {
		t.Errorf("Phase = %q, want %q", group.Phase, "pending")
	}
	if group.ID == "" {
		t.Error("ID should not be empty")
	}
	if group.Instances == nil {
		t.Error("Instances should be initialized")
	}
	if group.SubGroups == nil {
		t.Error("SubGroups should be initialized")
	}
	if group.DependsOn == nil {
		t.Error("DependsOn should be initialized")
	}
	if group.Created.IsZero() {
		t.Error("Created should not be zero")
	}
}

func TestSessionData_GetGroups_SetGroups(t *testing.T) {
	sess := &SessionData{}

	// Initially nil
	if sess.GetGroups() != nil {
		t.Error("GetGroups() should return nil for new session")
	}

	// Set groups
	groups := []*GroupData{
		{ID: "group-1", Name: "Group 1"},
		{ID: "group-2", Name: "Group 2"},
	}
	sess.SetGroups(groups)

	// Get groups
	got := sess.GetGroups()
	if len(got) != 2 {
		t.Fatalf("len(GetGroups()) = %d, want 2", len(got))
	}
	if got[0].ID != "group-1" {
		t.Errorf("got[0].ID = %q, want %q", got[0].ID, "group-1")
	}
	if got[1].ID != "group-2" {
		t.Errorf("got[1].ID = %q, want %q", got[1].ID, "group-2")
	}
}

func TestManager_LoadSession_UpdatesSessionID(t *testing.T) {
	tempDir := t.TempDir()

	// Create a session file manually
	claudioDir := filepath.Join(tempDir, ".claudio")
	if err := os.MkdirAll(claudioDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	sess := &SessionData{
		ID:       "loaded-session-id",
		Name:     "Test",
		BaseRepo: tempDir,
		Created:  time.Now(),
	}
	data, _ := json.Marshal(sess)
	if err := os.WriteFile(filepath.Join(claudioDir, "session.json"), data, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Create manager in legacy mode (no session ID)
	mgr := NewManager(Config{
		BaseDir: tempDir,
	})

	// Load session
	loaded, err := mgr.LoadSession()
	if err != nil {
		t.Fatalf("LoadSession() error = %v", err)
	}

	// Manager should update its session ID from loaded session
	if mgr.SessionID() != "loaded-session-id" {
		t.Errorf("SessionID() = %q, want %q", mgr.SessionID(), "loaded-session-id")
	}
	if loaded.ID != "loaded-session-id" {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, "loaded-session-id")
	}
}

func TestManager_JSON_Roundtrip_WithGroups(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Create session
	sess, err := mgr.CreateSession("Roundtrip Test", tempDir)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Build complex group structure
	inst1 := NewInstanceData("Task 1")
	inst2 := NewInstanceData("Task 2")
	inst3 := NewInstanceData("Task 3")
	inst4 := NewInstanceData("Task 4")
	sess.Instances = []*InstanceData{inst1, inst2, inst3, inst4}

	now := time.Now().Truncate(time.Second) // Truncate for JSON roundtrip

	sess.Groups = []*GroupData{
		{
			ID:             "foundation",
			Name:           "Foundation Tasks",
			Phase:          "completed",
			Instances:      []string{inst1.ID, inst2.ID},
			SubGroups:      []*GroupData{},
			ExecutionOrder: 0,
			DependsOn:      []string{},
			Created:        now,
		},
		{
			ID:             "features",
			Name:           "Feature Tasks",
			Phase:          "executing",
			Instances:      []string{},
			ExecutionOrder: 1,
			DependsOn:      []string{"foundation"},
			Created:        now,
			SubGroups: []*GroupData{
				{
					ID:             "features-part1",
					Name:           "Features Part 1",
					Phase:          "executing",
					Instances:      []string{inst3.ID},
					SubGroups:      []*GroupData{},
					ParentID:       "features",
					ExecutionOrder: 0,
					DependsOn:      []string{},
					Created:        now,
				},
				{
					ID:             "features-part2",
					Name:           "Features Part 2",
					Phase:          "pending",
					Instances:      []string{inst4.ID},
					SubGroups:      []*GroupData{},
					ParentID:       "features",
					ExecutionOrder: 1,
					DependsOn:      []string{"features-part1"},
					Created:        now,
				},
			},
		},
	}

	// Save
	if err := mgr.SaveSession(sess); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}

	if err := mgr.ReleaseLock(); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	// Load
	loaded, err := mgr.LoadSessionWithLock()
	if err != nil {
		t.Fatalf("LoadSessionWithLock() error = %v", err)
	}

	// Verify structure
	if len(loaded.Groups) != 2 {
		t.Fatalf("len(loaded.Groups) = %d, want 2", len(loaded.Groups))
	}

	// Verify foundation group
	foundation := loaded.GetGroup("foundation")
	if foundation == nil {
		t.Fatal("foundation group not found")
	}
	if foundation.Phase != "completed" {
		t.Errorf("foundation.Phase = %q, want %q", foundation.Phase, "completed")
	}
	if len(foundation.Instances) != 2 {
		t.Errorf("len(foundation.Instances) = %d, want 2", len(foundation.Instances))
	}

	// Verify features group
	features := loaded.GetGroup("features")
	if features == nil {
		t.Fatal("features group not found")
	}
	if len(features.DependsOn) != 1 || features.DependsOn[0] != "foundation" {
		t.Errorf("features.DependsOn = %v, want [foundation]", features.DependsOn)
	}
	if len(features.SubGroups) != 2 {
		t.Fatalf("len(features.SubGroups) = %d, want 2", len(features.SubGroups))
	}

	// Verify sub-groups
	part1 := loaded.GetGroup("features-part1")
	if part1 == nil {
		t.Fatal("features-part1 not found")
	}
	if part1.ParentID != "features" {
		t.Errorf("part1.ParentID = %q, want %q", part1.ParentID, "features")
	}

	part2 := loaded.GetGroup("features-part2")
	if part2 == nil {
		t.Fatal("features-part2 not found")
	}
	if len(part2.DependsOn) != 1 || part2.DependsOn[0] != "features-part1" {
		t.Errorf("part2.DependsOn = %v, want [features-part1]", part2.DependsOn)
	}
}

func TestManager_SaveSession_NilSession(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// Should not error on nil session
	if err := mgr.SaveSession(nil); err != nil {
		t.Errorf("SaveSession(nil) error = %v, want nil", err)
	}
}

func TestManager_HasLegacySession(t *testing.T) {
	tempDir := t.TempDir()

	mgr := NewManager(Config{
		BaseDir:   tempDir,
		SessionID: "test-session",
	})

	// No legacy session initially
	if mgr.HasLegacySession() {
		t.Error("HasLegacySession() should be false initially")
	}

	// Create legacy session file
	claudioDir := filepath.Join(tempDir, ".claudio")
	if err := os.MkdirAll(claudioDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudioDir, "session.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// Now should detect legacy session
	if !mgr.HasLegacySession() {
		t.Error("HasLegacySession() should be true after creating legacy file")
	}
}

func TestSessionData_ValidateGroups_AllInstancesDeleted(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{}, // All instances gone
		Groups: []*GroupData{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Instances: []string{"deleted-1", "deleted-2"},
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 0 {
		t.Errorf("ValidateGroups() should return empty when all instances are deleted, got %d groups", len(validated))
	}
}

func TestSessionData_ValidateGroups_PreservesNilSlices(t *testing.T) {
	sess := &SessionData{
		Instances: []*InstanceData{
			{ID: "inst-1", Task: "Task 1"},
		},
		Groups: []*GroupData{
			{
				ID:        "group-1",
				Name:      "Group 1",
				Instances: []string{"inst-1"},
				SubGroups: nil, // nil instead of empty slice
				DependsOn: nil, // nil instead of empty slice
			},
		},
	}

	validated := sess.ValidateGroups()
	if len(validated) != 1 {
		t.Fatalf("len(validated) = %d, want 1", len(validated))
	}

	// The implementation should handle nil slices gracefully
	// (SubGroups and DependsOn will be nil in result if empty)
}

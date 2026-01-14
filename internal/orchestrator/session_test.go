package orchestrator

import (
	"testing"
	"time"
)

func TestNewSession(t *testing.T) {
	tests := []struct {
		name         string
		sessionName  string
		baseRepo     string
		expectedName string
	}{
		{
			name:         "with custom name",
			sessionName:  "my-session",
			baseRepo:     "/path/to/repo",
			expectedName: "my-session",
		},
		{
			name:         "with empty name uses default",
			sessionName:  "",
			baseRepo:     "/path/to/repo",
			expectedName: "claudio-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := NewSession(tt.sessionName, tt.baseRepo)

			if session == nil {
				t.Fatal("NewSession returned nil")
			}

			if session.ID == "" {
				t.Error("session ID should not be empty")
			}

			if session.Name != tt.expectedName {
				t.Errorf("session.Name = %q, want %q", session.Name, tt.expectedName)
			}

			if session.BaseRepo != tt.baseRepo {
				t.Errorf("session.BaseRepo = %q, want %q", session.BaseRepo, tt.baseRepo)
			}

			if session.Instances == nil {
				t.Error("session.Instances should not be nil")
			}

			if len(session.Instances) != 0 {
				t.Errorf("session.Instances should be empty, got %d", len(session.Instances))
			}

			if session.Created.IsZero() {
				t.Error("session.Created should be set")
			}
		})
	}
}

func TestNewInstance(t *testing.T) {
	task := "implement feature X"
	inst := NewInstance(task)

	if inst == nil {
		t.Fatal("NewInstance returned nil")
	}

	if inst.ID == "" {
		t.Error("instance ID should not be empty")
	}

	if inst.Task != task {
		t.Errorf("instance.Task = %q, want %q", inst.Task, task)
	}

	if inst.Status != StatusPending {
		t.Errorf("instance.Status = %q, want %q", inst.Status, StatusPending)
	}

	if inst.Created.IsZero() {
		t.Error("instance.Created should be set")
	}
}

func TestSession_GetInstance(t *testing.T) {
	session := NewSession("test", "/repo")

	// Add some instances
	inst1 := NewInstance("task 1")
	inst1.ID = "inst-1"
	inst2 := NewInstance("task 2")
	inst2.ID = "inst-2"
	inst3 := NewInstance("task 3")
	inst3.ID = "inst-3"

	session.Instances = []*Instance{inst1, inst2, inst3}

	tests := []struct {
		name     string
		id       string
		expected *Instance
	}{
		{
			name:     "find first instance",
			id:       "inst-1",
			expected: inst1,
		},
		{
			name:     "find middle instance",
			id:       "inst-2",
			expected: inst2,
		},
		{
			name:     "find last instance",
			id:       "inst-3",
			expected: inst3,
		},
		{
			name:     "not found",
			id:       "inst-4",
			expected: nil,
		},
		{
			name:     "empty id",
			id:       "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := session.GetInstance(tt.id)
			if result != tt.expected {
				t.Errorf("GetInstance(%q) = %v, want %v", tt.id, result, tt.expected)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if id == "" {
			t.Error("generateID returned empty string")
		}
		if ids[id] {
			t.Errorf("generateID returned duplicate ID: %s", id)
		}
		ids[id] = true
	}
}

func TestGenerateID_Length(t *testing.T) {
	id := generateID()
	// 4 bytes = 8 hex characters
	expectedLen := 8
	if len(id) != expectedLen {
		t.Errorf("generateID() length = %d, want %d", len(id), expectedLen)
	}
}

func TestInstanceStatus_Constants(t *testing.T) {
	// Ensure status constants have expected values
	tests := []struct {
		status   InstanceStatus
		expected string
	}{
		{StatusPending, "pending"},
		{StatusWorking, "working"},
		{StatusWaitingInput, "waiting_input"},
		{StatusPaused, "paused"},
		{StatusCompleted, "completed"},
		{StatusError, "error"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("status constant = %q, want %q", tt.status, tt.expected)
		}
	}
}

func TestInstance_TmuxSession_Field(t *testing.T) {
	inst := NewInstance("test task")

	// TmuxSession should be empty initially
	if inst.TmuxSession != "" {
		t.Errorf("instance.TmuxSession should be empty initially, got %q", inst.TmuxSession)
	}

	// Should be settable
	inst.TmuxSession = "claudio-abc123"
	if inst.TmuxSession != "claudio-abc123" {
		t.Errorf("instance.TmuxSession = %q, want %q", inst.TmuxSession, "claudio-abc123")
	}
}

func TestSession_Created_Timestamp(t *testing.T) {
	before := time.Now()
	session := NewSession("test", "/repo")
	after := time.Now()

	if session.Created.Before(before) || session.Created.After(after) {
		t.Errorf("session.Created = %v, should be between %v and %v", session.Created, before, after)
	}
}

func TestInstance_Created_Timestamp(t *testing.T) {
	before := time.Now()
	inst := NewInstance("test task")
	after := time.Now()

	if inst.Created.Before(before) || inst.Created.After(after) {
		t.Errorf("instance.Created = %v, should be between %v and %v", inst.Created, before, after)
	}
}

// Test dependency-related functionality

func TestInstance_DependencyFields(t *testing.T) {
	inst := NewInstance("test task")

	// DependsOn should be nil initially
	if inst.DependsOn != nil {
		t.Errorf("instance.DependsOn should be nil initially, got %v", inst.DependsOn)
	}

	// Dependents should be nil initially
	if inst.Dependents != nil {
		t.Errorf("instance.Dependents should be nil initially, got %v", inst.Dependents)
	}

	// AutoStart should be false initially
	if inst.AutoStart {
		t.Error("instance.AutoStart should be false initially")
	}

	// Should be settable
	inst.DependsOn = []string{"inst-1", "inst-2"}
	inst.Dependents = []string{"inst-3"}
	inst.AutoStart = true

	if len(inst.DependsOn) != 2 {
		t.Errorf("instance.DependsOn length = %d, want 2", len(inst.DependsOn))
	}
	if len(inst.Dependents) != 1 {
		t.Errorf("instance.Dependents length = %d, want 1", len(inst.Dependents))
	}
	if !inst.AutoStart {
		t.Error("instance.AutoStart should be true after setting")
	}
}

func TestSession_AreDependenciesMet(t *testing.T) {
	session := NewSession("test", "/repo")

	// Create instances
	inst1 := NewInstance("task 1")
	inst1.ID = "inst-1"
	inst1.Status = StatusCompleted

	inst2 := NewInstance("task 2")
	inst2.ID = "inst-2"
	inst2.Status = StatusWorking

	inst3 := NewInstance("task 3 - depends on 1 and 2")
	inst3.ID = "inst-3"
	inst3.DependsOn = []string{"inst-1", "inst-2"}

	inst4 := NewInstance("task 4 - no dependencies")
	inst4.ID = "inst-4"

	session.Instances = []*Instance{inst1, inst2, inst3, inst4}

	tests := []struct {
		name     string
		inst     *Instance
		expected bool
	}{
		{
			name:     "no dependencies - always met",
			inst:     inst4,
			expected: true,
		},
		{
			name:     "all dependencies completed",
			inst:     &Instance{DependsOn: []string{"inst-1"}},
			expected: true,
		},
		{
			name:     "some dependencies not completed",
			inst:     inst3,
			expected: false,
		},
		{
			name:     "dependency on non-existent instance",
			inst:     &Instance{DependsOn: []string{"inst-999"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := session.AreDependenciesMet(tt.inst)
			if result != tt.expected {
				t.Errorf("AreDependenciesMet() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSession_GetDependentInstances(t *testing.T) {
	session := NewSession("test", "/repo")

	// Create instances with dependency relationships
	inst1 := NewInstance("task 1")
	inst1.ID = "inst-1"

	inst2 := NewInstance("task 2 - depends on 1")
	inst2.ID = "inst-2"
	inst2.DependsOn = []string{"inst-1"}

	inst3 := NewInstance("task 3 - depends on 1")
	inst3.ID = "inst-3"
	inst3.DependsOn = []string{"inst-1"}

	inst4 := NewInstance("task 4 - depends on 2")
	inst4.ID = "inst-4"
	inst4.DependsOn = []string{"inst-2"}

	session.Instances = []*Instance{inst1, inst2, inst3, inst4}

	tests := []struct {
		name          string
		instanceID    string
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:          "instance with two dependents",
			instanceID:    "inst-1",
			expectedCount: 2,
			expectedIDs:   []string{"inst-2", "inst-3"},
		},
		{
			name:          "instance with one dependent",
			instanceID:    "inst-2",
			expectedCount: 1,
			expectedIDs:   []string{"inst-4"},
		},
		{
			name:          "instance with no dependents",
			instanceID:    "inst-4",
			expectedCount: 0,
			expectedIDs:   nil,
		},
		{
			name:          "non-existent instance",
			instanceID:    "inst-999",
			expectedCount: 0,
			expectedIDs:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependents := session.GetDependentInstances(tt.instanceID)
			if len(dependents) != tt.expectedCount {
				t.Errorf("GetDependentInstances(%q) returned %d instances, want %d",
					tt.instanceID, len(dependents), tt.expectedCount)
			}

			// Check that expected IDs are present
			if tt.expectedIDs != nil {
				for _, expectedID := range tt.expectedIDs {
					found := false
					for _, dep := range dependents {
						if dep.ID == expectedID {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("GetDependentInstances(%q) missing expected dependent %q",
							tt.instanceID, expectedID)
					}
				}
			}
		})
	}
}

func TestSession_GetReadyInstances(t *testing.T) {
	session := NewSession("test", "/repo")

	// Create instances
	inst1 := NewInstance("task 1 - completed")
	inst1.ID = "inst-1"
	inst1.Status = StatusCompleted

	inst2 := NewInstance("task 2 - pending, auto-start, depends on 1")
	inst2.ID = "inst-2"
	inst2.Status = StatusPending
	inst2.AutoStart = true
	inst2.DependsOn = []string{"inst-1"}

	inst3 := NewInstance("task 3 - pending, no auto-start")
	inst3.ID = "inst-3"
	inst3.Status = StatusPending
	inst3.AutoStart = false

	inst4 := NewInstance("task 4 - pending, auto-start, deps not met")
	inst4.ID = "inst-4"
	inst4.Status = StatusPending
	inst4.AutoStart = true
	inst4.DependsOn = []string{"inst-3"} // inst-3 is not completed

	inst5 := NewInstance("task 5 - already working")
	inst5.ID = "inst-5"
	inst5.Status = StatusWorking
	inst5.AutoStart = true

	session.Instances = []*Instance{inst1, inst2, inst3, inst4, inst5}

	ready := session.GetReadyInstances()

	// Only inst2 should be ready (pending, auto-start, deps met)
	if len(ready) != 1 {
		t.Errorf("GetReadyInstances() returned %d instances, want 1", len(ready))
	}

	if len(ready) > 0 && ready[0].ID != "inst-2" {
		t.Errorf("GetReadyInstances()[0].ID = %q, want %q", ready[0].ID, "inst-2")
	}
}

// Test progress persistence and recovery functionality

func TestNewInstance_GeneratesClaudeSessionID(t *testing.T) {
	inst := NewInstance("test task")

	if inst.ClaudeSessionID == "" {
		t.Error("NewInstance should generate a ClaudeSessionID for resume capability")
	}

	// UUID format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	if len(inst.ClaudeSessionID) != 36 {
		t.Errorf("ClaudeSessionID length = %d, want 36 (UUID format)", len(inst.ClaudeSessionID))
	}
}

func TestGenerateUUID_Format(t *testing.T) {
	uuid := GenerateUUID()

	// UUID format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx (36 chars including hyphens)
	if len(uuid) != 36 {
		t.Errorf("GenerateUUID() length = %d, want 36", len(uuid))
	}

	// Check hyphen positions
	if uuid[8] != '-' || uuid[13] != '-' || uuid[18] != '-' || uuid[23] != '-' {
		t.Errorf("GenerateUUID() = %q, doesn't have hyphens in correct positions", uuid)
	}

	// Check version 4 indicator (position 14 should be '4')
	if uuid[14] != '4' {
		t.Errorf("GenerateUUID() = %q, version indicator at position 14 should be '4'", uuid)
	}

	// Check variant bits (position 19 should be 8, 9, a, or b)
	variantChar := uuid[19]
	if variantChar != '8' && variantChar != '9' && variantChar != 'a' && variantChar != 'b' {
		t.Errorf("GenerateUUID() = %q, variant indicator at position 19 should be 8, 9, a, or b", uuid)
	}
}

func TestGenerateUUID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		uuid := GenerateUUID()
		if ids[uuid] {
			t.Errorf("GenerateUUID returned duplicate: %s", uuid)
		}
		ids[uuid] = true
	}
}

func TestSession_NeedsRecovery(t *testing.T) {
	tests := []struct {
		name          string
		cleanShutdown bool
		instances     []*Instance
		expected      bool
	}{
		{
			name:          "clean shutdown - no recovery needed",
			cleanShutdown: true,
			instances: []*Instance{
				{ID: "1", Status: StatusWorking},
			},
			expected: false,
		},
		{
			name:          "not clean shutdown with working instance - needs recovery",
			cleanShutdown: false,
			instances: []*Instance{
				{ID: "1", Status: StatusWorking},
			},
			expected: true,
		},
		{
			name:          "not clean shutdown with waiting instance - needs recovery",
			cleanShutdown: false,
			instances: []*Instance{
				{ID: "1", Status: StatusWaitingInput},
			},
			expected: true,
		},
		{
			name:          "not clean shutdown but all completed - no recovery needed",
			cleanShutdown: false,
			instances: []*Instance{
				{ID: "1", Status: StatusCompleted},
				{ID: "2", Status: StatusCompleted},
			},
			expected: false,
		},
		{
			name:          "not clean shutdown but all paused - no recovery needed",
			cleanShutdown: false,
			instances: []*Instance{
				{ID: "1", Status: StatusPaused},
			},
			expected: false,
		},
		{
			name:          "not clean shutdown with mixed statuses - needs recovery",
			cleanShutdown: false,
			instances: []*Instance{
				{ID: "1", Status: StatusCompleted},
				{ID: "2", Status: StatusWorking},
				{ID: "3", Status: StatusPaused},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				CleanShutdown: tt.cleanShutdown,
				Instances:     tt.instances,
			}
			if got := session.NeedsRecovery(); got != tt.expected {
				t.Errorf("NeedsRecovery() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSession_GetInterruptedInstances(t *testing.T) {
	session := &Session{
		Instances: []*Instance{
			{ID: "1", Status: StatusWorking},
			{ID: "2", Status: StatusCompleted},
			{ID: "3", Status: StatusWaitingInput},
			{ID: "4", Status: StatusPaused},
			{ID: "5", Status: StatusWorking},
		},
	}

	interrupted := session.GetInterruptedInstances()

	// Should return instances with Working or WaitingInput status
	if len(interrupted) != 3 {
		t.Errorf("GetInterruptedInstances() returned %d instances, want 3", len(interrupted))
	}

	// Check that the right instances are returned
	expectedIDs := map[string]bool{"1": true, "3": true, "5": true}
	for _, inst := range interrupted {
		if !expectedIDs[inst.ID] {
			t.Errorf("GetInterruptedInstances() returned unexpected instance %q", inst.ID)
		}
	}
}

func TestSession_GetResumableInstances(t *testing.T) {
	session := &Session{
		Instances: []*Instance{
			{ID: "1", Status: StatusWorking, ClaudeSessionID: "uuid-1"},
			{ID: "2", Status: StatusCompleted, ClaudeSessionID: "uuid-2"},
			{ID: "3", Status: StatusWaitingInput, ClaudeSessionID: "uuid-3"},
			{ID: "4", Status: StatusPaused, ClaudeSessionID: "uuid-4"},
			{ID: "5", Status: StatusWorking, ClaudeSessionID: ""},       // No session ID
			{ID: "6", Status: StatusPending, ClaudeSessionID: "uuid-6"}, // Pending - not resumable
		},
	}

	resumable := session.GetResumableInstances()

	// Should return Working, WaitingInput, or Paused instances with ClaudeSessionID
	if len(resumable) != 3 {
		t.Errorf("GetResumableInstances() returned %d instances, want 3", len(resumable))
	}

	expectedIDs := map[string]bool{"1": true, "3": true, "4": true}
	for _, inst := range resumable {
		if !expectedIDs[inst.ID] {
			t.Errorf("GetResumableInstances() returned unexpected instance %q", inst.ID)
		}
	}
}

func TestSession_MarkInstancesInterrupted(t *testing.T) {
	session := &Session{
		Instances: []*Instance{
			{ID: "1", Status: StatusWorking},
			{ID: "2", Status: StatusCompleted},
			{ID: "3", Status: StatusWaitingInput},
		},
	}

	before := time.Now()
	session.MarkInstancesInterrupted()
	after := time.Now()

	// Check session-level fields
	if session.RecoveryState != RecoveryInterrupted {
		t.Errorf("RecoveryState = %q, want %q", session.RecoveryState, RecoveryInterrupted)
	}
	if session.InterruptedAt == nil {
		t.Error("InterruptedAt should be set")
	}
	if session.InterruptedAt.Before(before) || session.InterruptedAt.After(after) {
		t.Errorf("InterruptedAt = %v, should be between %v and %v", session.InterruptedAt, before, after)
	}

	// Check instance-level fields
	inst1 := session.GetInstance("1")
	if inst1.InterruptedAt == nil {
		t.Error("Working instance should have InterruptedAt set")
	}
	if inst1.Status != StatusInterrupted {
		t.Errorf("Working instance Status = %q, want %q", inst1.Status, StatusInterrupted)
	}

	inst2 := session.GetInstance("2")
	if inst2.InterruptedAt != nil {
		t.Error("Completed instance should NOT have InterruptedAt set")
	}
	if inst2.Status != StatusCompleted {
		t.Errorf("Completed instance Status should remain %q, got %q", StatusCompleted, inst2.Status)
	}

	inst3 := session.GetInstance("3")
	if inst3.InterruptedAt == nil {
		t.Error("WaitingInput instance should have InterruptedAt set")
	}
	if inst3.Status != StatusInterrupted {
		t.Errorf("WaitingInput instance Status = %q, want %q", inst3.Status, StatusInterrupted)
	}
}

func TestSession_MarkRecovered(t *testing.T) {
	session := &Session{
		RecoveryState:   RecoveryInterrupted,
		RecoveryAttempt: 0,
	}

	before := time.Now()
	session.MarkRecovered()
	after := time.Now()

	if session.RecoveryState != RecoveryRecovered {
		t.Errorf("RecoveryState = %q, want %q", session.RecoveryState, RecoveryRecovered)
	}
	if session.RecoveryAttempt != 1 {
		t.Errorf("RecoveryAttempt = %d, want 1", session.RecoveryAttempt)
	}
	if session.RecoveredAt == nil {
		t.Error("RecoveredAt should be set")
	}
	if session.RecoveredAt.Before(before) || session.RecoveredAt.After(after) {
		t.Errorf("RecoveredAt = %v, should be between %v and %v", session.RecoveredAt, before, after)
	}
	if session.CleanShutdown {
		t.Error("CleanShutdown should be false after recovery")
	}

	// Call again to verify increment
	session.MarkRecovered()
	if session.RecoveryAttempt != 2 {
		t.Errorf("RecoveryAttempt after second call = %d, want 2", session.RecoveryAttempt)
	}
}

func TestRecoveryState_Constants(t *testing.T) {
	tests := []struct {
		state    RecoveryState
		expected string
	}{
		{RecoveryNone, ""},
		{RecoveryInterrupted, "interrupted"},
		{RecoveryRecovered, "recovered"},
	}

	for _, tt := range tests {
		if string(tt.state) != tt.expected {
			t.Errorf("RecoveryState constant = %q, want %q", tt.state, tt.expected)
		}
	}
}

func TestInstance_EffectiveName(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		task        string
		expected    string
	}{
		{
			name:        "returns DisplayName when set",
			displayName: "Fix auth bug",
			task:        "Fix authentication issues with OAuth and update the login flow",
			expected:    "Fix auth bug",
		},
		{
			name:        "returns Task when DisplayName is empty",
			displayName: "",
			task:        "Implement new feature",
			expected:    "Implement new feature",
		},
		{
			name:        "returns DisplayName even if shorter than Task",
			displayName: "X",
			task:        "A very long task description",
			expected:    "X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inst := &Instance{
				Task:        tt.task,
				DisplayName: tt.displayName,
			}
			if got := inst.EffectiveName(); got != tt.expected {
				t.Errorf("EffectiveName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

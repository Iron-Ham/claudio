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

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

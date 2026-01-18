package orchestrator

import "testing"

func TestSessionType_Icon(t *testing.T) {
	tests := []struct {
		sessionType SessionType
		wantIcon    string
	}{
		{SessionTypeStandard, "\u25cf"},   // ● filled circle
		{SessionTypePlan, "\u25c7"},       // ◇ diamond
		{SessionTypePlanMulti, "\u25c8"},  // ◈ filled diamond
		{SessionTypeUltraPlan, "\u26a1"},  // ⚡ lightning
		{SessionTypeTripleShot, "\u25b3"}, // △ triangle
		{"unknown", "\u25cf"},             // Default to filled circle
	}

	for _, tt := range tests {
		t.Run(string(tt.sessionType), func(t *testing.T) {
			got := tt.sessionType.Icon()
			if got != tt.wantIcon {
				t.Errorf("Icon() = %q, want %q", got, tt.wantIcon)
			}
		})
	}
}

func TestSessionType_GroupingMode(t *testing.T) {
	tests := []struct {
		sessionType SessionType
		wantMode    string
	}{
		{SessionTypeStandard, "none"},
		{SessionTypePlan, "shared"},
		{SessionTypePlanMulti, "own"},
		{SessionTypeUltraPlan, "own"},
		{SessionTypeTripleShot, "own"},
		{"unknown", "none"},
	}

	for _, tt := range tests {
		t.Run(string(tt.sessionType), func(t *testing.T) {
			got := tt.sessionType.GroupingMode()
			if got != tt.wantMode {
				t.Errorf("GroupingMode() = %q, want %q", got, tt.wantMode)
			}
		})
	}
}

func TestSessionType_IsOrchestratedType(t *testing.T) {
	tests := []struct {
		sessionType SessionType
		want        bool
	}{
		{SessionTypeStandard, false},
		{SessionTypePlan, false},
		{SessionTypePlanMulti, true},
		{SessionTypeUltraPlan, true},
		{SessionTypeTripleShot, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.sessionType), func(t *testing.T) {
			got := tt.sessionType.IsOrchestratedType()
			if got != tt.want {
				t.Errorf("IsOrchestratedType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionType_SharedGroupName(t *testing.T) {
	tests := []struct {
		sessionType SessionType
		want        string
	}{
		{SessionTypePlan, "Plans"},
		{SessionTypeStandard, ""},
		{SessionTypeUltraPlan, ""},
		{SessionTypeTripleShot, ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.sessionType), func(t *testing.T) {
			got := tt.sessionType.SharedGroupName()
			if got != tt.want {
				t.Errorf("SharedGroupName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewInstanceGroupWithType(t *testing.T) {
	group := NewInstanceGroupWithType("Test Group", SessionTypeUltraPlan, "Test objective")

	if group.ID == "" {
		t.Error("Expected group ID to be generated")
	}
	if group.Name != "Test Group" {
		t.Errorf("Name = %q, want %q", group.Name, "Test Group")
	}
	if GetSessionType(group) != SessionTypeUltraPlan {
		t.Errorf("SessionType = %q, want %q", GetSessionType(group), SessionTypeUltraPlan)
	}
	if group.Objective != "Test objective" {
		t.Errorf("Objective = %q, want %q", group.Objective, "Test objective")
	}
	if group.Phase != GroupPhasePending {
		t.Errorf("Phase = %q, want %q", group.Phase, GroupPhasePending)
	}
}

func TestSetSessionType(t *testing.T) {
	group := NewInstanceGroup("Test Group")

	// Initially should have no session type
	if GetSessionType(group) != "" {
		t.Errorf("Expected empty SessionType initially, got %q", GetSessionType(group))
	}

	// Set to TripleShot
	SetSessionType(group, SessionTypeTripleShot)
	if GetSessionType(group) != SessionTypeTripleShot {
		t.Errorf("SessionType = %q, want %q", GetSessionType(group), SessionTypeTripleShot)
	}

	// Change to UltraPlan
	SetSessionType(group, SessionTypeUltraPlan)
	if GetSessionType(group) != SessionTypeUltraPlan {
		t.Errorf("SessionType = %q, want %q", GetSessionType(group), SessionTypeUltraPlan)
	}

	// Verify the underlying string value is correct
	if group.SessionType != string(SessionTypeUltraPlan) {
		t.Errorf("Underlying SessionType = %q, want %q", group.SessionType, string(SessionTypeUltraPlan))
	}
}

func TestSession_GetOrCreateSharedGroup(t *testing.T) {
	t.Run("creates new shared group", func(t *testing.T) {
		session := &Session{
			ID:     "test-session",
			Groups: make([]*InstanceGroup, 0),
		}

		group := session.GetOrCreateSharedGroup(SessionTypePlan)

		if group == nil {
			t.Fatal("Expected group to be created")
		}
		if GetSessionType(group) != SessionTypePlan {
			t.Errorf("SessionType = %q, want %q", GetSessionType(group), SessionTypePlan)
		}
		if group.Name != "Plans" {
			t.Errorf("Name = %q, want %q", group.Name, "Plans")
		}
		if len(session.Groups) != 1 {
			t.Errorf("Expected 1 group, got %d", len(session.Groups))
		}
	})

	t.Run("returns existing shared group", func(t *testing.T) {
		existingGroup := NewInstanceGroupWithType("Plans", SessionTypePlan, "")
		session := &Session{
			ID:     "test-session",
			Groups: []*InstanceGroup{existingGroup},
		}

		group := session.GetOrCreateSharedGroup(SessionTypePlan)

		if group != existingGroup {
			t.Error("Expected to get existing group")
		}
		if len(session.Groups) != 1 {
			t.Errorf("Expected 1 group, got %d", len(session.Groups))
		}
	})

	t.Run("returns nil for non-shared types", func(t *testing.T) {
		session := &Session{
			ID:     "test-session",
			Groups: make([]*InstanceGroup, 0),
		}

		group := session.GetOrCreateSharedGroup(SessionTypeUltraPlan)

		if group != nil {
			t.Error("Expected nil for non-shared type")
		}
	})
}

func TestSession_GetGroupBySessionType(t *testing.T) {
	planGroup := NewInstanceGroupWithType("Plans", SessionTypePlan, "")
	ultraGroup := NewInstanceGroupWithType("Ultra Task", SessionTypeUltraPlan, "objective")

	session := &Session{
		ID:     "test-session",
		Groups: []*InstanceGroup{planGroup, ultraGroup},
	}

	tests := []struct {
		name        string
		sessionType SessionType
		expected    *InstanceGroup
	}{
		{"find plan group", SessionTypePlan, planGroup},
		{"find ultraplan group", SessionTypeUltraPlan, ultraGroup},
		{"not found tripleshot", SessionTypeTripleShot, nil},
		{"not found standard", SessionTypeStandard, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := session.GetGroupBySessionType(tt.sessionType)
			if got != tt.expected {
				t.Errorf("GetGroupBySessionType(%q) = %v, want %v", tt.sessionType, got, tt.expected)
			}
		})
	}
}

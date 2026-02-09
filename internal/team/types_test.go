package team

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func TestPhase_String(t *testing.T) {
	tests := []struct {
		phase Phase
		want  string
	}{
		{PhaseForming, "forming"},
		{PhaseBlocked, "blocked"},
		{PhaseWorking, "working"},
		{PhaseReporting, "reporting"},
		{PhaseDone, "done"},
		{PhaseFailed, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.phase.String(); got != tt.want {
				t.Errorf("Phase.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPhase_IsTerminal(t *testing.T) {
	tests := []struct {
		phase    Phase
		terminal bool
	}{
		{PhaseForming, false},
		{PhaseBlocked, false},
		{PhaseWorking, false},
		{PhaseReporting, false},
		{PhaseDone, true},
		{PhaseFailed, true},
	}
	for _, tt := range tests {
		t.Run(tt.phase.String(), func(t *testing.T) {
			if got := tt.phase.IsTerminal(); got != tt.terminal {
				t.Errorf("Phase(%q).IsTerminal() = %v, want %v", tt.phase, got, tt.terminal)
			}
		})
	}
}

func TestRole_String(t *testing.T) {
	tests := []struct {
		role Role
		want string
	}{
		{RoleExecution, "execution"},
		{RolePlanning, "planning"},
		{RoleReview, "review"},
		{RoleConsolidation, "consolidation"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.role.String(); got != tt.want {
				t.Errorf("Role.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRole_IsValid(t *testing.T) {
	tests := []struct {
		role  Role
		valid bool
	}{
		{RoleExecution, true},
		{RolePlanning, true},
		{RoleReview, true},
		{RoleConsolidation, true},
		{Role("unknown"), false},
		{Role(""), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.valid {
				t.Errorf("Role(%q).IsValid() = %v, want %v", tt.role, got, tt.valid)
			}
		})
	}
}

func TestTokenBudget_IsUnlimited(t *testing.T) {
	tests := []struct {
		name   string
		budget TokenBudget
		want   bool
	}{
		{"all zeros", TokenBudget{}, true},
		{"input set", TokenBudget{MaxInputTokens: 100}, false},
		{"output set", TokenBudget{MaxOutputTokens: 100}, false},
		{"cost set", TokenBudget{MaxTotalCost: 1.0}, false},
		{"all set", TokenBudget{MaxInputTokens: 100, MaxOutputTokens: 200, MaxTotalCost: 5.0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.budget.IsUnlimited(); got != tt.want {
				t.Errorf("TokenBudget.IsUnlimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

func validSpec() Spec {
	return Spec{
		ID:       "team-1",
		Name:     "Alpha",
		Role:     RoleExecution,
		Tasks:    []ultraplan.PlannedTask{{ID: "t1", Title: "Task 1"}},
		TeamSize: 2,
	}
}

func TestSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Spec)
		wantErr string
	}{
		{"valid", func(s *Spec) {}, ""},
		{"missing ID", func(s *Spec) { s.ID = "" }, "ID is required"},
		{"missing Name", func(s *Spec) { s.Name = "" }, "Name is required"},
		{"invalid Role", func(s *Spec) { s.Role = "bad" }, "invalid role"},
		{"no tasks", func(s *Spec) { s.Tasks = nil }, "at least one task"},
		{"zero team size", func(s *Spec) { s.TeamSize = 0 }, "TeamSize must be >= 1"},
		{"negative MinInstances", func(s *Spec) { s.MinInstances = -1 }, "MinInstances must be >= 0"},
		{"negative MaxInstances", func(s *Spec) { s.MaxInstances = -1 }, "MaxInstances must be >= 0"},
		{"min > max", func(s *Spec) { s.MinInstances = 5; s.MaxInstances = 2 }, "MinInstances (5) must be <= MaxInstances (2)"},
		{"valid min/max", func(s *Spec) { s.MinInstances = 1; s.MaxInstances = 5 }, ""},
		{"min with unlimited max", func(s *Spec) { s.MinInstances = 3; s.MaxInstances = 0 }, ""},
		{"max with zero min", func(s *Spec) { s.MinInstances = 0; s.MaxInstances = 5 }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := validSpec()
			tt.modify(&spec)
			err := spec.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Spec.Validate() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Spec.Validate() = nil, want error containing %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Spec.Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestMessageType_String(t *testing.T) {
	tests := []struct {
		mt   MessageType
		want string
	}{
		{MessageTypeDiscovery, "discovery"},
		{MessageTypeDependency, "dependency"},
		{MessageTypeWarning, "warning"},
		{MessageTypeRequest, "request"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mt.String(); got != tt.want {
				t.Errorf("MessageType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMessagePriority_String(t *testing.T) {
	tests := []struct {
		mp   MessagePriority
		want string
	}{
		{PriorityInfo, "info"},
		{PriorityImportant, "important"},
		{PriorityUrgent, "urgent"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mp.String(); got != tt.want {
				t.Errorf("MessagePriority.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInterTeamMessage_IsBroadcast(t *testing.T) {
	tests := []struct {
		name string
		msg  InterTeamMessage
		want bool
	}{
		{"broadcast", InterTeamMessage{ToTeam: BroadcastRecipient}, true},
		{"targeted", InterTeamMessage{ToTeam: "team-2"}, false},
		{"empty", InterTeamMessage{ToTeam: ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.msg.IsBroadcast(); got != tt.want {
				t.Errorf("InterTeamMessage.IsBroadcast() = %v, want %v", got, tt.want)
			}
		})
	}
}

package instance

import "testing"

func TestPRWorkflow_SocketName(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		wantSocket string
	}{
		{
			name:       "standard instance",
			instanceID: "abc123",
			wantSocket: "claudio-abc123",
		},
		{
			name:       "longer instance ID",
			instanceID: "instance-xyz-789",
			wantSocket: "claudio-instance-xyz-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := PRWorkflowConfig{
				TmuxWidth:  100,
				TmuxHeight: 50,
			}
			workflow := NewPRWorkflow(tt.instanceID, "/tmp", "main", "test task", cfg)

			if got := workflow.SocketName(); got != tt.wantSocket {
				t.Errorf("SocketName() = %q, want %q", got, tt.wantSocket)
			}
		})
	}
}

func TestNewPRWorkflowWithSocket(t *testing.T) {
	customSocket := "claudio-custom-socket"
	cfg := PRWorkflowConfig{
		TmuxWidth:  100,
		TmuxHeight: 50,
	}

	workflow := NewPRWorkflowWithSocket("inst1", customSocket, "/tmp", "main", "test task", cfg)

	if got := workflow.SocketName(); got != customSocket {
		t.Errorf("SocketName() = %q, want %q", got, customSocket)
	}

	// Verify session name format
	expectedSession := "claudio-inst1-pr"
	if got := workflow.SessionName(); got != expectedSession {
		t.Errorf("SessionName() = %q, want %q", got, expectedSession)
	}
}

func TestPRWorkflow_Running_Initial(t *testing.T) {
	cfg := PRWorkflowConfig{
		TmuxWidth:  100,
		TmuxHeight: 50,
	}
	workflow := NewPRWorkflow("test", "/tmp", "main", "task", cfg)

	if workflow.Running() {
		t.Error("New PR workflow should not be running")
	}
}

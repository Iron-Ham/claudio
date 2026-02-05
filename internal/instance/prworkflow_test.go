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

func TestPRWorkflow_Stop_NotRunning(t *testing.T) {
	cfg := PRWorkflowConfig{
		TmuxWidth:  100,
		TmuxHeight: 50,
	}
	workflow := NewPRWorkflow("test-stop", "/tmp", "main", "task", cfg)

	// Stop on a non-running workflow should be a no-op
	if err := workflow.Stop(); err != nil {
		t.Errorf("Stop() on non-running workflow returned error: %v", err)
	}

	if workflow.Running() {
		t.Error("workflow should not be running after Stop()")
	}
}

func TestPRWorkflow_Stop_Idempotent(t *testing.T) {
	cfg := PRWorkflowConfig{
		TmuxWidth:  100,
		TmuxHeight: 50,
	}
	workflow := NewPRWorkflow("test-idem", "/tmp", "main", "task", cfg)

	// Multiple stops should not panic
	for i := 0; i < 3; i++ {
		if err := workflow.Stop(); err != nil {
			t.Errorf("Stop() call %d returned error: %v", i+1, err)
		}
	}
}

package update

import (
	"errors"
	"testing"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/output"
	"github.com/spf13/viper"
)

// mockContext implements Context for testing.
type mockContext struct {
	session           *orchestrator.Session
	orchestrator      *orchestrator.Orchestrator
	outputManager     *output.Manager
	logger            *logging.Logger
	instanceCount     int
	activeInstance    *orchestrator.Instance
	errorMessage      string
	infoMessage       string
	activeTab         int
	pausedInstances   []string
	ensureActiveCalls int
}

func newMockContext() *mockContext {
	return &mockContext{
		outputManager:   output.NewManager(),
		pausedInstances: []string{},
	}
}

func (m *mockContext) Session() *orchestrator.Session {
	return m.session
}

func (m *mockContext) Orchestrator() *orchestrator.Orchestrator {
	return m.orchestrator
}

func (m *mockContext) OutputManager() *output.Manager {
	return m.outputManager
}

func (m *mockContext) Logger() *logging.Logger {
	return m.logger
}

func (m *mockContext) InstanceCount() int {
	return m.instanceCount
}

func (m *mockContext) ActiveInstance() *orchestrator.Instance {
	return m.activeInstance
}

func (m *mockContext) SetErrorMessage(msg string) {
	m.errorMessage = msg
}

func (m *mockContext) SetInfoMessage(msg string) {
	m.infoMessage = msg
}

func (m *mockContext) ClearInfoMessage() {
	m.infoMessage = ""
}

func (m *mockContext) SetActiveTab(idx int) {
	m.activeTab = idx
}

func (m *mockContext) PauseInstance(instanceID string) {
	m.pausedInstances = append(m.pausedInstances, instanceID)
}

func (m *mockContext) EnsureActiveVisible() {
	m.ensureActiveCalls++
}

func TestHandleOutput(t *testing.T) {
	ctx := newMockContext()

	HandleOutput(ctx, msg.OutputMsg{
		InstanceID: "test-instance",
		Data:       []byte("hello world"),
	})

	output := ctx.outputManager.GetOutput("test-instance")
	if output != "hello world" {
		t.Errorf("HandleOutput() output = %q, want %q", output, "hello world")
	}
}

func TestHandleError(t *testing.T) {
	ctx := newMockContext()

	HandleError(ctx, msg.ErrMsg{
		Err: errors.New("test error"),
	})

	if ctx.errorMessage != "test error" {
		t.Errorf("HandleError() errorMessage = %q, want %q", ctx.errorMessage, "test error")
	}
}

func TestHandlePRComplete(t *testing.T) {
	tests := []struct {
		name       string
		session    *orchestrator.Session
		instanceID string
		success    bool
		wantInfo   string
		wantError  string
	}{
		{
			name:       "nil session",
			session:    nil,
			instanceID: "test",
			success:    true,
			wantInfo:   "",
		},
		{
			name: "instance not found",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{},
			},
			instanceID: "nonexistent",
			success:    true,
			wantInfo:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.session = tt.session

			HandlePRComplete(ctx, msg.PRCompleteMsg{
				InstanceID: tt.instanceID,
				Success:    tt.success,
			})

			if ctx.infoMessage != tt.wantInfo {
				t.Errorf("HandlePRComplete() infoMessage = %q, want %q", ctx.infoMessage, tt.wantInfo)
			}
			if ctx.errorMessage != tt.wantError {
				t.Errorf("HandlePRComplete() errorMessage = %q, want %q", ctx.errorMessage, tt.wantError)
			}
		})
	}
}

func TestHandlePROpened(t *testing.T) {
	tests := []struct {
		name       string
		session    *orchestrator.Session
		instanceID string
		wantInfo   string
	}{
		{
			name:       "nil session",
			session:    nil,
			instanceID: "test",
			wantInfo:   "",
		},
		{
			name: "instance not found",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{},
			},
			instanceID: "nonexistent",
			wantInfo:   "",
		},
		{
			name: "instance found",
			session: &orchestrator.Session{
				Instances: []*orchestrator.Instance{
					{ID: "test-instance"},
				},
			},
			instanceID: "test-instance",
			wantInfo:   "PR opened for instance test-instance - use :D to remove or run review tools",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.session = tt.session

			HandlePROpened(ctx, msg.PROpenedMsg{
				InstanceID: tt.instanceID,
			})

			if ctx.infoMessage != tt.wantInfo {
				t.Errorf("HandlePROpened() infoMessage = %q, want %q", ctx.infoMessage, tt.wantInfo)
			}
		})
	}
}

func TestHandleTimeout(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "test-instance"},
		},
	}

	tests := []struct {
		name        string
		session     *orchestrator.Session
		instanceID  string
		timeoutType instance.TimeoutType
		wantInfo    string
	}{
		{
			name:       "nil session",
			session:    nil,
			instanceID: "test",
			wantInfo:   "",
		},
		{
			name:       "instance not found",
			session:    session,
			instanceID: "nonexistent",
			wantInfo:   "",
		},
		{
			name:        "activity timeout",
			session:     session,
			instanceID:  "test-instance",
			timeoutType: instance.TimeoutActivity,
			wantInfo:    "Instance test-instance is stuck (no activity) - use Ctrl+R to restart or Ctrl+K to kill",
		},
		{
			name:        "completion timeout",
			session:     session,
			instanceID:  "test-instance",
			timeoutType: instance.TimeoutCompletion,
			wantInfo:    "Instance test-instance is timed out (max runtime exceeded) - use Ctrl+R to restart or Ctrl+K to kill",
		},
		{
			name:        "stale timeout",
			session:     session,
			instanceID:  "test-instance",
			timeoutType: instance.TimeoutStale,
			wantInfo:    "Instance test-instance is stuck (repeated output) - use Ctrl+R to restart or Ctrl+K to kill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.session = tt.session

			HandleTimeout(ctx, msg.TimeoutMsg{
				InstanceID:  tt.instanceID,
				TimeoutType: tt.timeoutType,
			})

			if ctx.infoMessage != tt.wantInfo {
				t.Errorf("HandleTimeout() infoMessage = %q, want %q", ctx.infoMessage, tt.wantInfo)
			}
		})
	}
}

func TestHandleTaskAdded(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		instance        *orchestrator.Instance
		activeInstance  *orchestrator.Instance
		instanceCount   int
		wantError       string
		wantActiveTab   int
		wantPausedCount int
		wantEnsureCalls int
	}{
		{
			name:            "success with no active instance",
			err:             nil,
			instance:        &orchestrator.Instance{Task: "test task"},
			activeInstance:  nil,
			instanceCount:   3,
			wantError:       "",
			wantActiveTab:   2, // instanceCount - 1
			wantPausedCount: 0,
			wantEnsureCalls: 1,
		},
		{
			name:            "success with active instance",
			err:             nil,
			instance:        &orchestrator.Instance{Task: "test task"},
			activeInstance:  &orchestrator.Instance{ID: "old-instance"},
			instanceCount:   5,
			wantError:       "",
			wantActiveTab:   4, // instanceCount - 1
			wantPausedCount: 1,
			wantEnsureCalls: 1,
		},
		{
			name:            "error",
			err:             errors.New("failed to add task"),
			instance:        nil,
			instanceCount:   3,
			wantError:       "failed to add task",
			wantActiveTab:   0, // not changed
			wantPausedCount: 0,
			wantEnsureCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.activeInstance = tt.activeInstance
			ctx.instanceCount = tt.instanceCount
			ctx.infoMessage = "Adding task..." // Should be cleared

			HandleTaskAdded(ctx, msg.TaskAddedMsg{
				Instance: tt.instance,
				Err:      tt.err,
			})

			// Info message should be cleared
			if ctx.infoMessage != "" && tt.err == nil {
				t.Errorf("HandleTaskAdded() infoMessage not cleared, got %q", ctx.infoMessage)
			}

			if ctx.errorMessage != tt.wantError {
				t.Errorf("HandleTaskAdded() errorMessage = %q, want %q", ctx.errorMessage, tt.wantError)
			}

			if tt.err == nil && ctx.activeTab != tt.wantActiveTab {
				t.Errorf("HandleTaskAdded() activeTab = %d, want %d", ctx.activeTab, tt.wantActiveTab)
			}

			if len(ctx.pausedInstances) != tt.wantPausedCount {
				t.Errorf("HandleTaskAdded() paused %d instances, want %d", len(ctx.pausedInstances), tt.wantPausedCount)
			}

			if ctx.ensureActiveCalls != tt.wantEnsureCalls {
				t.Errorf("HandleTaskAdded() ensureActiveCalls = %d, want %d", ctx.ensureActiveCalls, tt.wantEnsureCalls)
			}
		})
	}
}

func TestHandleDependentTaskAdded(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "parent-id", Task: "Parent task description"},
		},
	}

	tests := []struct {
		name            string
		session         *orchestrator.Session
		err             error
		dependsOn       string
		activeInstance  *orchestrator.Instance
		instanceCount   int
		wantError       string
		wantInfo        string
		wantActiveTab   int
		wantPausedCount int
		wantEnsureCalls int
	}{
		{
			name:            "success with parent found",
			session:         session,
			err:             nil,
			dependsOn:       "parent-id",
			activeInstance:  &orchestrator.Instance{ID: "old"},
			instanceCount:   2,
			wantError:       "",
			wantInfo:        `Chained task added. Will auto-start when "Parent task description" completes.`,
			wantActiveTab:   1,
			wantPausedCount: 1,
			wantEnsureCalls: 1,
		},
		{
			name:            "success with parent not found (uses ID)",
			session:         session,
			err:             nil,
			dependsOn:       "unknown-parent",
			instanceCount:   2,
			wantError:       "",
			wantInfo:        `Chained task added. Will auto-start when "unknown-parent" completes.`,
			wantActiveTab:   1,
			wantEnsureCalls: 1,
		},
		{
			name:          "error",
			session:       session,
			err:           errors.New("dependency error"),
			dependsOn:     "parent-id",
			instanceCount: 2,
			wantError:     "dependency error",
			wantInfo:      "",
			wantActiveTab: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.session = tt.session
			ctx.activeInstance = tt.activeInstance
			ctx.instanceCount = tt.instanceCount
			ctx.infoMessage = "Adding dependent task..." // Should be cleared

			HandleDependentTaskAdded(ctx, msg.DependentTaskAddedMsg{
				Instance:  &orchestrator.Instance{Task: "new task"},
				DependsOn: tt.dependsOn,
				Err:       tt.err,
			})

			if ctx.errorMessage != tt.wantError {
				t.Errorf("HandleDependentTaskAdded() errorMessage = %q, want %q", ctx.errorMessage, tt.wantError)
			}

			if ctx.infoMessage != tt.wantInfo {
				t.Errorf("HandleDependentTaskAdded() infoMessage = %q, want %q", ctx.infoMessage, tt.wantInfo)
			}

			if tt.err == nil && ctx.activeTab != tt.wantActiveTab {
				t.Errorf("HandleDependentTaskAdded() activeTab = %d, want %d", ctx.activeTab, tt.wantActiveTab)
			}

			if len(ctx.pausedInstances) != tt.wantPausedCount {
				t.Errorf("HandleDependentTaskAdded() paused %d instances, want %d", len(ctx.pausedInstances), tt.wantPausedCount)
			}

			if ctx.ensureActiveCalls != tt.wantEnsureCalls {
				t.Errorf("HandleDependentTaskAdded() ensureActiveCalls = %d, want %d", ctx.ensureActiveCalls, tt.wantEnsureCalls)
			}
		})
	}
}

func TestHandleDependentTaskAdded_LongTaskTruncation(t *testing.T) {
	longTask := "This is a very long task description that exceeds fifty characters and should be truncated"
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "parent-id", Task: longTask},
		},
	}

	ctx := newMockContext()
	ctx.session = session
	ctx.instanceCount = 2

	HandleDependentTaskAdded(ctx, msg.DependentTaskAddedMsg{
		Instance:  &orchestrator.Instance{Task: "new task"},
		DependsOn: "parent-id",
		Err:       nil,
	})

	// Should be truncated to 50 chars + "..."
	expectedTruncated := longTask[:50] + "..."
	expectedInfo := `Chained task added. Will auto-start when "` + expectedTruncated + `" completes.`

	if ctx.infoMessage != expectedInfo {
		t.Errorf("HandleDependentTaskAdded() task not truncated correctly\ngot:  %q\nwant: %q", ctx.infoMessage, expectedInfo)
	}
}

func TestHandlePRComplete_NilOrchestrator(t *testing.T) {
	// Test case where session and instance exist, but orchestrator is nil
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "test-instance"},
		},
	}

	ctx := newMockContext()
	ctx.session = session
	ctx.orchestrator = nil // Orchestrator is nil

	HandlePRComplete(ctx, msg.PRCompleteMsg{
		InstanceID: "test-instance",
		Success:    true,
	})

	// Should return early when orchestrator is nil (no info/error message)
	if ctx.infoMessage != "" {
		t.Errorf("HandlePRComplete() with nil orchestrator: infoMessage = %q, want empty", ctx.infoMessage)
	}
	if ctx.errorMessage != "" {
		t.Errorf("HandlePRComplete() with nil orchestrator: errorMessage = %q, want empty", ctx.errorMessage)
	}
}

func TestHandleTimeout_UnknownType(t *testing.T) {
	// Test case where an unknown timeout type is provided
	// This exercises the default case in the switch statement
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "test-instance"},
		},
	}

	ctx := newMockContext()
	ctx.session = session

	// Use an unknown timeout type (type 99 doesn't exist)
	HandleTimeout(ctx, msg.TimeoutMsg{
		InstanceID:  "test-instance",
		TimeoutType: instance.TimeoutType(99),
	})

	// The default case produces an empty statusText, so the message should just be:
	// "Instance test-instance is  - use Ctrl+R to restart or Ctrl+K to kill"
	expectedInfo := "Instance test-instance is  - use Ctrl+R to restart or Ctrl+K to kill"
	if ctx.infoMessage != expectedInfo {
		t.Errorf("HandleTimeout() with unknown type: infoMessage = %q, want %q", ctx.infoMessage, expectedInfo)
	}
}

func TestHandleTaskAdded_WithLogger(t *testing.T) {
	// Create a logger that writes to a temp directory
	logger, err := logging.NewLogger(t.TempDir(), "DEBUG")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	t.Run("success with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.instanceCount = 2
		ctx.activeInstance = nil

		HandleTaskAdded(ctx, msg.TaskAddedMsg{
			Instance: &orchestrator.Instance{Task: "test task"},
			Err:      nil,
		})

		// Verify success behavior
		if ctx.activeTab != 1 {
			t.Errorf("HandleTaskAdded() activeTab = %d, want 1", ctx.activeTab)
		}
		if ctx.ensureActiveCalls != 1 {
			t.Errorf("HandleTaskAdded() ensureActiveCalls = %d, want 1", ctx.ensureActiveCalls)
		}
	})

	t.Run("error with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.instanceCount = 2

		HandleTaskAdded(ctx, msg.TaskAddedMsg{
			Instance: nil,
			Err:      errors.New("task error"),
		})

		// Verify error was set
		if ctx.errorMessage != "task error" {
			t.Errorf("HandleTaskAdded() errorMessage = %q, want %q", ctx.errorMessage, "task error")
		}
	})

	t.Run("success without instance (nil Instance field)", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.instanceCount = 2

		HandleTaskAdded(ctx, msg.TaskAddedMsg{
			Instance: nil, // Instance is nil but no error
			Err:      nil,
		})

		// Should still complete without panic
		if ctx.activeTab != 1 {
			t.Errorf("HandleTaskAdded() activeTab = %d, want 1", ctx.activeTab)
		}
	})
}

func TestHandleDependentTaskAdded_WithLogger(t *testing.T) {
	// Create a logger that writes to a temp directory
	logger, err := logging.NewLogger(t.TempDir(), "DEBUG")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "parent-id", Task: "Parent task"},
		},
	}

	t.Run("success with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.session = session
		ctx.instanceCount = 2
		ctx.activeInstance = nil

		HandleDependentTaskAdded(ctx, msg.DependentTaskAddedMsg{
			Instance:  &orchestrator.Instance{Task: "child task"},
			DependsOn: "parent-id",
			Err:       nil,
		})

		// Verify success behavior
		expectedInfo := `Chained task added. Will auto-start when "Parent task" completes.`
		if ctx.infoMessage != expectedInfo {
			t.Errorf("HandleDependentTaskAdded() infoMessage = %q, want %q", ctx.infoMessage, expectedInfo)
		}
	})

	t.Run("error with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.session = session
		ctx.instanceCount = 2

		HandleDependentTaskAdded(ctx, msg.DependentTaskAddedMsg{
			Instance:  nil,
			DependsOn: "parent-id",
			Err:       errors.New("dependency error"),
		})

		// Verify error was set
		if ctx.errorMessage != "dependency error" {
			t.Errorf("HandleDependentTaskAdded() errorMessage = %q, want %q", ctx.errorMessage, "dependency error")
		}
	})

	t.Run("success without instance (nil Instance field)", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.session = session
		ctx.instanceCount = 2

		HandleDependentTaskAdded(ctx, msg.DependentTaskAddedMsg{
			Instance:  nil, // Instance is nil but no error
			DependsOn: "parent-id",
			Err:       nil,
		})

		// Should still complete without panic
		expectedInfo := `Chained task added. Will auto-start when "Parent task" completes.`
		if ctx.infoMessage != expectedInfo {
			t.Errorf("HandleDependentTaskAdded() infoMessage = %q, want %q", ctx.infoMessage, expectedInfo)
		}
	})
}

func TestHandleDependentTaskAdded_NilSession(t *testing.T) {
	// Test case where session is nil, verifying the parent lookup gracefully handles it
	ctx := newMockContext()
	ctx.session = nil // Session is nil
	ctx.instanceCount = 2

	HandleDependentTaskAdded(ctx, msg.DependentTaskAddedMsg{
		Instance:  &orchestrator.Instance{Task: "child task"},
		DependsOn: "parent-id",
		Err:       nil,
	})

	// Should use the raw dependsOn ID since session is nil
	expectedInfo := `Chained task added. Will auto-start when "parent-id" completes.`
	if ctx.infoMessage != expectedInfo {
		t.Errorf("HandleDependentTaskAdded() with nil session: infoMessage = %q, want %q", ctx.infoMessage, expectedInfo)
	}
}

func TestHandleOutput_MultipleOutputs(t *testing.T) {
	ctx := newMockContext()

	// Test that multiple outputs append correctly
	HandleOutput(ctx, msg.OutputMsg{
		InstanceID: "test-instance",
		Data:       []byte("first "),
	})

	HandleOutput(ctx, msg.OutputMsg{
		InstanceID: "test-instance",
		Data:       []byte("second"),
	})

	output := ctx.outputManager.GetOutput("test-instance")
	expected := "first second"
	if output != expected {
		t.Errorf("HandleOutput() multiple outputs = %q, want %q", output, expected)
	}
}

func TestHandleOutput_DifferentInstances(t *testing.T) {
	ctx := newMockContext()

	// Test outputs to different instances are isolated
	HandleOutput(ctx, msg.OutputMsg{
		InstanceID: "instance-a",
		Data:       []byte("output-a"),
	})

	HandleOutput(ctx, msg.OutputMsg{
		InstanceID: "instance-b",
		Data:       []byte("output-b"),
	})

	outputA := ctx.outputManager.GetOutput("instance-a")
	if outputA != "output-a" {
		t.Errorf("HandleOutput() instance-a = %q, want %q", outputA, "output-a")
	}

	outputB := ctx.outputManager.GetOutput("instance-b")
	if outputB != "output-b" {
		t.Errorf("HandleOutput() instance-b = %q, want %q", outputB, "output-b")
	}
}

func TestHandleTaskAdded_AutoStartConfig(t *testing.T) {
	// Save original viper value and restore after test
	originalValue := viper.GetBool("session.auto_start_on_add")
	defer viper.Set("session.auto_start_on_add", originalValue)

	t.Run("auto-start disabled does not attempt start", func(t *testing.T) {
		viper.Set("session.auto_start_on_add", false)

		ctx := newMockContext()
		ctx.instanceCount = 2

		HandleTaskAdded(ctx, msg.TaskAddedMsg{
			Instance: &orchestrator.Instance{ID: "test-instance", Task: "test task"},
			Err:      nil,
		})

		// When auto-start is disabled, no info message about starting should be set
		// The info message should remain empty (no "Started instance" message)
		if ctx.infoMessage != "" {
			t.Errorf("HandleTaskAdded() with auto-start disabled: infoMessage = %q, want empty", ctx.infoMessage)
		}
		// No error either since we didn't attempt to start
		if ctx.errorMessage != "" {
			t.Errorf("HandleTaskAdded() with auto-start disabled: errorMessage = %q, want empty", ctx.errorMessage)
		}
	})

	t.Run("auto-start enabled with nil orchestrator handles gracefully", func(t *testing.T) {
		viper.Set("session.auto_start_on_add", true)

		ctx := newMockContext()
		ctx.orchestrator = nil // No orchestrator
		ctx.instanceCount = 2

		HandleTaskAdded(ctx, msg.TaskAddedMsg{
			Instance: &orchestrator.Instance{ID: "test-instance", Task: "test task"},
			Err:      nil,
		})

		// Should handle nil orchestrator gracefully (no panic, no error)
		// No info message because we couldn't start, but also no error
		if ctx.errorMessage != "" {
			t.Errorf("HandleTaskAdded() with auto-start and nil orchestrator: errorMessage = %q, want empty", ctx.errorMessage)
		}
	})

	t.Run("auto-start enabled with nil instance does not attempt start", func(t *testing.T) {
		viper.Set("session.auto_start_on_add", true)

		ctx := newMockContext()
		ctx.instanceCount = 2

		HandleTaskAdded(ctx, msg.TaskAddedMsg{
			Instance: nil, // No instance
			Err:      nil,
		})

		// Should not attempt start when instance is nil
		if ctx.errorMessage != "" {
			t.Errorf("HandleTaskAdded() with auto-start and nil instance: errorMessage = %q, want empty", ctx.errorMessage)
		}
	})
}

func TestHandleTaskAdded_AutoStartDefault(t *testing.T) {
	// Test that the default config behavior is to auto-start
	// This relies on the viper default being set to true

	// First, ensure we have a clean viper state for this key
	viper.Set("session.auto_start_on_add", true) // Default value

	ctx := newMockContext()
	ctx.orchestrator = nil // No orchestrator, but auto-start will be attempted
	ctx.instanceCount = 2

	HandleTaskAdded(ctx, msg.TaskAddedMsg{
		Instance: &orchestrator.Instance{ID: "test-instance", Task: "test task"},
		Err:      nil,
	})

	// The code tries to auto-start but gracefully handles nil orchestrator
	// No error message because we skip when orchestrator is nil
	if ctx.errorMessage != "" {
		t.Errorf("HandleTaskAdded() default auto-start: errorMessage = %q, want empty", ctx.errorMessage)
	}
}

func TestHandleInstanceStubCreated(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		instance        *orchestrator.Instance
		activeInstance  *orchestrator.Instance
		instanceCount   int
		wantError       string
		wantInfo        string
		wantActiveTab   int
		wantPausedCount int
		wantEnsureCalls int
	}{
		{
			name:            "success with no active instance",
			err:             nil,
			instance:        &orchestrator.Instance{ID: "test-id", Task: "test task"},
			activeInstance:  nil,
			instanceCount:   3,
			wantError:       "",
			wantInfo:        "Preparing instance test-id...",
			wantActiveTab:   2, // instanceCount - 1
			wantPausedCount: 0,
			wantEnsureCalls: 1,
		},
		{
			name:            "success with active instance",
			err:             nil,
			instance:        &orchestrator.Instance{ID: "new-id", Task: "new task"},
			activeInstance:  &orchestrator.Instance{ID: "old-instance"},
			instanceCount:   5,
			wantError:       "",
			wantInfo:        "Preparing instance new-id...",
			wantActiveTab:   4, // instanceCount - 1
			wantPausedCount: 1,
			wantEnsureCalls: 1,
		},
		{
			name:            "error",
			err:             errors.New("failed to create stub"),
			instance:        nil,
			instanceCount:   3,
			wantError:       "failed to create stub",
			wantInfo:        "", // cleared
			wantActiveTab:   0,  // not changed
			wantPausedCount: 0,
			wantEnsureCalls: 0,
		},
		{
			name:            "success with nil instance",
			err:             nil,
			instance:        nil,
			instanceCount:   2,
			wantError:       "",
			wantInfo:        "", // no info message when instance is nil
			wantActiveTab:   1,
			wantPausedCount: 0,
			wantEnsureCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockContext()
			ctx.activeInstance = tt.activeInstance
			ctx.instanceCount = tt.instanceCount
			ctx.infoMessage = "Adding task..." // Should be cleared

			HandleInstanceStubCreated(ctx, msg.InstanceStubCreatedMsg{
				Instance: tt.instance,
				Err:      tt.err,
			})

			if ctx.errorMessage != tt.wantError {
				t.Errorf("HandleInstanceStubCreated() errorMessage = %q, want %q", ctx.errorMessage, tt.wantError)
			}

			if ctx.infoMessage != tt.wantInfo {
				t.Errorf("HandleInstanceStubCreated() infoMessage = %q, want %q", ctx.infoMessage, tt.wantInfo)
			}

			if tt.err == nil && ctx.activeTab != tt.wantActiveTab {
				t.Errorf("HandleInstanceStubCreated() activeTab = %d, want %d", ctx.activeTab, tt.wantActiveTab)
			}

			if len(ctx.pausedInstances) != tt.wantPausedCount {
				t.Errorf("HandleInstanceStubCreated() paused %d instances, want %d", len(ctx.pausedInstances), tt.wantPausedCount)
			}

			if ctx.ensureActiveCalls != tt.wantEnsureCalls {
				t.Errorf("HandleInstanceStubCreated() ensureActiveCalls = %d, want %d", ctx.ensureActiveCalls, tt.wantEnsureCalls)
			}
		})
	}
}

func TestHandleInstanceSetupComplete(t *testing.T) {
	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "test-instance", Task: "test task"},
		},
	}

	tests := []struct {
		name        string
		session     *orchestrator.Session
		instanceID  string
		err         error
		autoStartOn bool
		wantError   string
		wantInfo    string
	}{
		{
			name:        "nil session",
			session:     nil,
			instanceID:  "test",
			err:         nil,
			autoStartOn: true,
			wantError:   "",
			wantInfo:    "",
		},
		{
			name:        "instance not found",
			session:     session,
			instanceID:  "nonexistent",
			err:         nil,
			autoStartOn: true,
			wantError:   "",
			wantInfo:    "",
		},
		{
			name:        "error during setup",
			session:     session,
			instanceID:  "test-instance",
			err:         errors.New("worktree creation failed"),
			autoStartOn: true,
			wantError:   "Failed to setup instance test-instance: worktree creation failed",
			wantInfo:    "",
		},
		{
			name:        "success without auto-start",
			session:     session,
			instanceID:  "test-instance",
			err:         nil,
			autoStartOn: false,
			wantError:   "",
			wantInfo:    "Instance test-instance ready",
		},
		{
			name:        "success with auto-start but nil orchestrator",
			session:     session,
			instanceID:  "test-instance",
			err:         nil,
			autoStartOn: true,
			wantError:   "",
			wantInfo:    "", // No message when orchestrator is nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set("session.auto_start_on_add", tt.autoStartOn)

			ctx := newMockContext()
			ctx.session = tt.session
			ctx.orchestrator = nil // No orchestrator in test

			HandleInstanceSetupComplete(ctx, msg.InstanceSetupCompleteMsg{
				InstanceID: tt.instanceID,
				Err:        tt.err,
			})

			if ctx.errorMessage != tt.wantError {
				t.Errorf("HandleInstanceSetupComplete() errorMessage = %q, want %q", ctx.errorMessage, tt.wantError)
			}

			if ctx.infoMessage != tt.wantInfo {
				t.Errorf("HandleInstanceSetupComplete() infoMessage = %q, want %q", ctx.infoMessage, tt.wantInfo)
			}
		})
	}
}

func TestHandleInstanceStubCreated_WithLogger(t *testing.T) {
	logger, err := logging.NewLogger(t.TempDir(), "DEBUG")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	t.Run("success with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.instanceCount = 2

		HandleInstanceStubCreated(ctx, msg.InstanceStubCreatedMsg{
			Instance: &orchestrator.Instance{ID: "test-id", Task: "test task"},
			Err:      nil,
		})

		if ctx.infoMessage != "Preparing instance test-id..." {
			t.Errorf("HandleInstanceStubCreated() infoMessage = %q, want %q", ctx.infoMessage, "Preparing instance test-id...")
		}
	})

	t.Run("error with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger

		HandleInstanceStubCreated(ctx, msg.InstanceStubCreatedMsg{
			Instance: nil,
			Err:      errors.New("stub creation failed"),
		})

		if ctx.errorMessage != "stub creation failed" {
			t.Errorf("HandleInstanceStubCreated() errorMessage = %q, want %q", ctx.errorMessage, "stub creation failed")
		}
	})
}

func TestHandleInstanceSetupComplete_WithLogger(t *testing.T) {
	logger, err := logging.NewLogger(t.TempDir(), "DEBUG")
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	session := &orchestrator.Session{
		Instances: []*orchestrator.Instance{
			{ID: "test-instance", Task: "test task"},
		},
	}

	t.Run("success with logger", func(t *testing.T) {
		viper.Set("session.auto_start_on_add", false)

		ctx := newMockContext()
		ctx.logger = logger
		ctx.session = session

		HandleInstanceSetupComplete(ctx, msg.InstanceSetupCompleteMsg{
			InstanceID: "test-instance",
			Err:        nil,
		})

		if ctx.infoMessage != "Instance test-instance ready" {
			t.Errorf("HandleInstanceSetupComplete() infoMessage = %q, want %q", ctx.infoMessage, "Instance test-instance ready")
		}
	})

	t.Run("error with logger", func(t *testing.T) {
		ctx := newMockContext()
		ctx.logger = logger
		ctx.session = session

		HandleInstanceSetupComplete(ctx, msg.InstanceSetupCompleteMsg{
			InstanceID: "test-instance",
			Err:        errors.New("setup failed"),
		})

		expectedErr := "Failed to setup instance test-instance: setup failed"
		if ctx.errorMessage != expectedErr {
			t.Errorf("HandleInstanceSetupComplete() errorMessage = %q, want %q", ctx.errorMessage, expectedErr)
		}
	})
}

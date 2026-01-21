package msg

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/tui/view"
	"github.com/spf13/viper"
)

func TestTick(t *testing.T) {
	cmd := Tick()

	if cmd == nil {
		t.Fatal("Tick() returned nil command")
	}

	// Execute the command and verify the message type
	// Note: This will block for ~100ms due to tea.Tick
	start := time.Now()
	result := cmd()
	elapsed := time.Since(start)

	// Should take approximately 100ms (with some tolerance)
	if elapsed < 50*time.Millisecond {
		t.Errorf("Tick() returned too quickly: %v", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Tick() took too long: %v", elapsed)
	}

	// Result should be a TickMsg
	tickMsg, ok := result.(TickMsg)
	if !ok {
		t.Errorf("Tick() returned %T, want TickMsg", result)
	}

	// The time should be close to now
	tickTime := time.Time(tickMsg)
	timeDiff := time.Since(tickTime)
	if timeDiff > 100*time.Millisecond {
		t.Errorf("TickMsg time is too old: %v ago", timeDiff)
	}
}

func TestRingBell(t *testing.T) {
	cmd := RingBell()

	if cmd == nil {
		t.Fatal("RingBell() returned nil command")
	}

	// Execute the command - it writes to stdout but returns nil
	result := cmd()

	if result != nil {
		t.Errorf("RingBell() returned %v, want nil", result)
	}
}

func TestNotifyUser(t *testing.T) {
	// Save and restore viper state
	originalEnabled := viper.GetBool("ultraplan.notifications.enabled")
	originalUseSound := viper.GetBool("ultraplan.notifications.use_sound")
	defer func() {
		viper.Set("ultraplan.notifications.enabled", originalEnabled)
		viper.Set("ultraplan.notifications.use_sound", originalUseSound)
	}()

	t.Run("notifications disabled", func(t *testing.T) {
		viper.Set("ultraplan.notifications.enabled", false)

		cmd := NotifyUser()
		if cmd == nil {
			t.Fatal("NotifyUser() returned nil command")
		}

		result := cmd()
		if result != nil {
			t.Errorf("NotifyUser() with notifications disabled returned %v, want nil", result)
		}
	})

	t.Run("notifications enabled no sound", func(t *testing.T) {
		viper.Set("ultraplan.notifications.enabled", true)
		viper.Set("ultraplan.notifications.use_sound", false)

		cmd := NotifyUser()
		if cmd == nil {
			t.Fatal("NotifyUser() returned nil command")
		}

		result := cmd()
		if result != nil {
			t.Errorf("NotifyUser() returned %v, want nil", result)
		}
	})
}

func TestAddTaskAsync(t *testing.T) {
	t.Run("returns non-nil command", func(t *testing.T) {
		cmd := AddTaskAsync(nil, nil, "test task")
		if cmd == nil {
			t.Fatal("AddTaskAsync() returned nil command")
		}
	})

	t.Run("returns error when orchestrator is nil", func(t *testing.T) {
		cmd := AddTaskAsync(nil, nil, "test task")
		msg := cmd()

		taskMsg, ok := msg.(TaskAddedMsg)
		if !ok {
			t.Fatalf("AddTaskAsync()() returned %T, want TaskAddedMsg", msg)
		}

		if taskMsg.Err == nil {
			t.Error("expected error when orchestrator is nil")
		}
		if taskMsg.Instance != nil {
			t.Error("expected nil instance on error")
		}
	})
}

func TestAddTaskFromBranchAsync(t *testing.T) {
	t.Run("returns non-nil command", func(t *testing.T) {
		cmd := AddTaskFromBranchAsync(nil, nil, "test task", "main")
		if cmd == nil {
			t.Fatal("AddTaskFromBranchAsync() returned nil command")
		}
	})

	t.Run("returns error when orchestrator is nil", func(t *testing.T) {
		cmd := AddTaskFromBranchAsync(nil, nil, "test task", "main")
		msg := cmd()

		taskMsg, ok := msg.(TaskAddedMsg)
		if !ok {
			t.Fatalf("AddTaskFromBranchAsync()() returned %T, want TaskAddedMsg", msg)
		}

		if taskMsg.Err == nil {
			t.Error("expected error when orchestrator is nil")
		}
		if taskMsg.Instance != nil {
			t.Error("expected nil instance on error")
		}
	})
}

func TestAddDependentTaskAsync(t *testing.T) {
	t.Run("returns non-nil command", func(t *testing.T) {
		cmd := AddDependentTaskAsync(nil, nil, "test task", "parent-id")
		if cmd == nil {
			t.Fatal("AddDependentTaskAsync() returned nil command")
		}
	})

	t.Run("returns error when orchestrator is nil", func(t *testing.T) {
		cmd := AddDependentTaskAsync(nil, nil, "test task", "parent-id")
		msg := cmd()

		depMsg, ok := msg.(DependentTaskAddedMsg)
		if !ok {
			t.Fatalf("AddDependentTaskAsync()() returned %T, want DependentTaskAddedMsg", msg)
		}

		if depMsg.Err == nil {
			t.Error("expected error when orchestrator is nil")
		}
		if depMsg.Instance != nil {
			t.Error("expected nil instance on error")
		}
		if depMsg.DependsOn != "parent-id" {
			t.Errorf("DependsOn = %q, want %q", depMsg.DependsOn, "parent-id")
		}
	})
}

func TestCheckTripleShotCompletionAsync(t *testing.T) {
	t.Run("nil coordinator", func(t *testing.T) {
		// With nil coordinator, should handle gracefully
		cmd := CheckTripleShotCompletionAsync(nil, "group-1")

		if cmd == nil {
			t.Fatal("CheckTripleShotCompletionAsync() returned nil command")
		}
	})
}

func TestProcessAttemptCompletionAsync(t *testing.T) {
	cmd := ProcessAttemptCompletionAsync(nil, "group-1", 0)

	if cmd == nil {
		t.Fatal("ProcessAttemptCompletionAsync() returned nil command")
	}
}

func TestProcessJudgeCompletionAsync(t *testing.T) {
	cmd := ProcessJudgeCompletionAsync(nil, "group-1")

	if cmd == nil {
		t.Fatal("ProcessJudgeCompletionAsync() returned nil command")
	}
}

func TestCheckPlanFileAsync(t *testing.T) {
	t.Run("nil ultraPlan", func(t *testing.T) {
		cmd := CheckPlanFileAsync(nil, nil)

		if cmd == nil {
			t.Fatal("CheckPlanFileAsync() returned nil command")
		}

		// Execute and verify it returns nil for nil ultraPlan
		result := cmd()
		if result != nil {
			t.Errorf("CheckPlanFileAsync(nil, nil)() = %v, want nil", result)
		}
	})

	t.Run("nil coordinator in ultraPlan", func(t *testing.T) {
		ultraPlan := &view.UltraPlanState{
			Coordinator: nil,
		}

		cmd := CheckPlanFileAsync(nil, ultraPlan)
		if cmd == nil {
			t.Fatal("CheckPlanFileAsync() returned nil command")
		}

		result := cmd()
		if result != nil {
			t.Errorf("CheckPlanFileAsync with nil coordinator returned %v, want nil", result)
		}
	})
}

func TestCheckMultiPassPlanFilesAsync(t *testing.T) {
	t.Run("nil ultraPlan", func(t *testing.T) {
		cmds := CheckMultiPassPlanFilesAsync(nil, nil)

		if cmds != nil {
			t.Errorf("CheckMultiPassPlanFilesAsync(nil, nil) = %v, want nil", cmds)
		}
	})

	t.Run("nil coordinator in ultraPlan", func(t *testing.T) {
		ultraPlan := &view.UltraPlanState{
			Coordinator: nil,
		}

		cmds := CheckMultiPassPlanFilesAsync(nil, ultraPlan)
		if cmds != nil {
			t.Errorf("CheckMultiPassPlanFilesAsync with nil coordinator = %v, want nil", cmds)
		}
	})
}

func TestCheckPlanManagerFileAsync(t *testing.T) {
	t.Run("nil ultraPlan", func(t *testing.T) {
		cmd := CheckPlanManagerFileAsync(nil, nil, nil)

		if cmd == nil {
			t.Fatal("CheckPlanManagerFileAsync() returned nil command")
		}

		result := cmd()
		if result != nil {
			t.Errorf("CheckPlanManagerFileAsync(nil, nil, nil)() = %v, want nil", result)
		}
	})

	t.Run("nil coordinator in ultraPlan", func(t *testing.T) {
		ultraPlan := &view.UltraPlanState{
			Coordinator: nil,
		}

		cmd := CheckPlanManagerFileAsync(nil, nil, ultraPlan)
		if cmd == nil {
			t.Fatal("CheckPlanManagerFileAsync() returned nil command")
		}

		result := cmd()
		if result != nil {
			t.Errorf("CheckPlanManagerFileAsync with nil coordinator returned %v, want nil", result)
		}
	})
}

// Note: Testing CheckTripleShotCompletionAsync with a mock coordinator
// that returns nil session requires complex internal setup. The nil
// coordinator case is tested above.

// TestCheckMultiPassPlanFilesAsyncSessionStates tests various session states
func TestCheckMultiPassPlanFilesAsyncSessionStates(t *testing.T) {
	// Create minimal orchestrator and coordinator for testing
	// Note: Full integration testing would require more setup

	t.Run("empty ultraPlan state", func(t *testing.T) {
		ultraPlan := &view.UltraPlanState{}

		cmds := CheckMultiPassPlanFilesAsync(nil, ultraPlan)
		if cmds != nil {
			t.Errorf("Expected nil commands for empty ultraPlan, got %d commands", len(cmds))
		}
	})
}

// TestTickReturnsCorrectMessageType verifies type assertion works
func TestTickReturnsCorrectMessageType(t *testing.T) {
	cmd := Tick()
	msg := cmd()

	switch msg.(type) {
	case TickMsg:
		// Expected type
	default:
		t.Errorf("Tick() returned unexpected type %T", msg)
	}
}

// TestRingBellSideEffect verifies the bell command executes without error
func TestRingBellSideEffect(t *testing.T) {
	cmd := RingBell()

	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RingBell() panicked: %v", r)
		}
	}()

	cmd()
}

// TestNotifyUserWithSoundPath tests the sound path configuration
func TestNotifyUserWithSoundPath(t *testing.T) {
	// Save and restore viper state
	originalEnabled := viper.GetBool("ultraplan.notifications.enabled")
	originalUseSound := viper.GetBool("ultraplan.notifications.use_sound")
	originalSoundPath := viper.GetString("ultraplan.notifications.sound_path")
	defer func() {
		viper.Set("ultraplan.notifications.enabled", originalEnabled)
		viper.Set("ultraplan.notifications.use_sound", originalUseSound)
		viper.Set("ultraplan.notifications.sound_path", originalSoundPath)
	}()

	t.Run("custom sound path", func(t *testing.T) {
		viper.Set("ultraplan.notifications.enabled", true)
		viper.Set("ultraplan.notifications.use_sound", true)
		viper.Set("ultraplan.notifications.sound_path", "/nonexistent/sound.aiff")

		cmd := NotifyUser()
		if cmd == nil {
			t.Fatal("NotifyUser() returned nil command")
		}

		// Execute - should not panic even with invalid path
		// (afplay command will fail silently in background)
		result := cmd()
		if result != nil {
			t.Errorf("NotifyUser() returned %v, want nil", result)
		}
	})

	t.Run("empty sound path uses system alert sound", func(t *testing.T) {
		viper.Set("ultraplan.notifications.enabled", true)
		viper.Set("ultraplan.notifications.use_sound", true)
		viper.Set("ultraplan.notifications.sound_path", "")

		cmd := NotifyUser()
		if cmd == nil {
			t.Fatal("NotifyUser() returned nil command")
		}

		// Execute - should use system alert sound via osascript
		result := cmd()
		if result != nil {
			t.Errorf("NotifyUser() returned %v, want nil", result)
		}
	})
}

// TestCommandsAreIdempotent verifies commands can be called multiple times
func TestCommandsAreIdempotent(t *testing.T) {
	t.Run("RingBell multiple calls", func(t *testing.T) {
		cmd := RingBell()

		// Call multiple times - should not cause issues
		for i := 0; i < 3; i++ {
			result := cmd()
			if result != nil {
				t.Errorf("RingBell() call %d returned %v, want nil", i, result)
			}
		}
	})
}

// Note: Testing with an uninitialized Coordinator that returns nil session
// requires complex internal setup as the Session() method accesses internal
// pointers. The nil coordinator cases above verify nil-safety at the outer level.

func TestRemoveInstanceAsync(t *testing.T) {
	t.Run("returns non-nil command", func(t *testing.T) {
		cmd := RemoveInstanceAsync(nil, nil, "test-instance-id")
		if cmd == nil {
			t.Fatal("RemoveInstanceAsync() returned nil command")
		}
	})

	t.Run("returns error when orchestrator is nil", func(t *testing.T) {
		cmd := RemoveInstanceAsync(nil, nil, "test-instance-id")
		msg := cmd()

		removedMsg, ok := msg.(InstanceRemovedMsg)
		if !ok {
			t.Fatalf("RemoveInstanceAsync()() returned %T, want InstanceRemovedMsg", msg)
		}

		if removedMsg.Err == nil {
			t.Error("expected error when orchestrator is nil")
		}
		if removedMsg.InstanceID != "test-instance-id" {
			t.Errorf("InstanceID = %q, want %q", removedMsg.InstanceID, "test-instance-id")
		}
	})
}

func TestLoadDiffAsync(t *testing.T) {
	t.Run("returns non-nil command", func(t *testing.T) {
		cmd := LoadDiffAsync(nil, "/path/to/worktree", "test-instance-id")
		if cmd == nil {
			t.Fatal("LoadDiffAsync() returned nil command")
		}
	})

	t.Run("returns error when orchestrator is nil", func(t *testing.T) {
		cmd := LoadDiffAsync(nil, "/path/to/worktree", "test-instance-id")
		msg := cmd()

		diffMsg, ok := msg.(DiffLoadedMsg)
		if !ok {
			t.Fatalf("LoadDiffAsync()() returned %T, want DiffLoadedMsg", msg)
		}

		if diffMsg.Err == nil {
			t.Error("expected error when orchestrator is nil")
		}
		if diffMsg.InstanceID != "test-instance-id" {
			t.Errorf("InstanceID = %q, want %q", diffMsg.InstanceID, "test-instance-id")
		}
		if diffMsg.DiffContent != "" {
			t.Errorf("DiffContent = %q, want empty string on error", diffMsg.DiffContent)
		}
	})
}

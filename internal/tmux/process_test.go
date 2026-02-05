package tmux

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestGetPanePID_InvalidSession(t *testing.T) {
	// Requesting PID from a non-existent session should return 0
	pid := GetPanePID("nonexistent-socket-test", "nonexistent-session")
	if pid != 0 {
		t.Errorf("GetPanePID(nonexistent) = %d, want 0", pid)
	}
}

func TestGetDescendantPIDs_InvalidPID(t *testing.T) {
	tests := []struct {
		name string
		pid  int
	}{
		{"zero", 0},
		{"negative", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pids := GetDescendantPIDs(tt.pid)
			if pids != nil {
				t.Errorf("GetDescendantPIDs(%d) = %v, want nil", tt.pid, pids)
			}
		})
	}
}

func TestGetDescendantPIDs_WithChildren(t *testing.T) {
	// Start a process that has a child
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sleep process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	childPID := cmd.Process.Pid

	// Our test process should have at least the sleep child
	descendants := GetDescendantPIDs(os.Getpid())

	found := false
	for _, pid := range descendants {
		if pid == childPID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GetDescendantPIDs(%d) did not include child PID %d, got %v", os.Getpid(), childPID, descendants)
	}
}

func TestGetDescendantPIDs_NoChildren(t *testing.T) {
	// A process with no children should return nil/empty
	// Use a PID that's unlikely to have children (our own sleep process)
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sleep process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// sleep itself should have no children
	descendants := GetDescendantPIDs(cmd.Process.Pid)
	if len(descendants) != 0 {
		t.Errorf("GetDescendantPIDs(sleep) = %v, want empty", descendants)
	}
}

func TestIsProcessAlive(t *testing.T) {
	tests := []struct {
		name     string
		pid      int
		expected bool
	}{
		{"zero PID", 0, false},
		{"negative PID", -1, false},
		{"own process", os.Getpid(), true},
		{"nonexistent PID", 99999999, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProcessAlive(tt.pid)
			if got != tt.expected {
				t.Errorf("IsProcessAlive(%d) = %v, want %v", tt.pid, got, tt.expected)
			}
		})
	}
}

func TestKillProcessTree_InvalidPID(t *testing.T) {
	// Should not panic on invalid PIDs
	KillProcessTree(0)
	KillProcessTree(-1)
}

func TestKillProcessTree_KillsProcess(t *testing.T) {
	// Start a process we can kill
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sleep process: %v", err)
	}

	pid := cmd.Process.Pid

	// Verify it's alive
	if !IsProcessAlive(pid) {
		t.Fatalf("Process %d should be alive after start", pid)
	}

	// Kill the tree
	KillProcessTree(pid)

	// Wait for the process to be reaped
	_ = cmd.Wait()

	// Verify it's dead
	if IsProcessAlive(pid) {
		t.Errorf("Process %d should be dead after KillProcessTree", pid)
	}
}

func TestKillProcessTree_KillsDescendants(t *testing.T) {
	// Start a shell that runs a sleep subprocess
	cmd := exec.Command("sh", "-c", "sleep 60 & wait")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	shellPID := cmd.Process.Pid

	// Give the shell time to start the sleep subprocess
	time.Sleep(200 * time.Millisecond)

	// Get descendants before killing
	descendants := GetDescendantPIDs(shellPID)

	// Kill the tree
	KillProcessTree(shellPID)
	_ = cmd.Wait()

	// Verify descendants are also dead
	time.Sleep(100 * time.Millisecond) // Brief wait for process cleanup
	for _, pid := range descendants {
		if IsProcessAlive(pid) {
			// Clean up just in case
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Errorf("Descendant process %d should be dead after KillProcessTree", pid)
		}
	}
}

func TestKillServer_NonexistentSocket(t *testing.T) {
	// Killing a non-existent server should return an error (but not panic)
	err := KillServer("nonexistent-socket-for-test-12345")
	if err == nil {
		t.Error("KillServer on non-existent socket should return error")
	}
}

func TestCollectProcessTree_InvalidSession(t *testing.T) {
	pids := CollectProcessTree("nonexistent-socket", "nonexistent-session")
	if pids != nil {
		t.Errorf("CollectProcessTree(nonexistent) = %v, want nil", pids)
	}
}

func TestEnsureProcessesKilled_EmptyList(t *testing.T) {
	// Should not panic on empty/nil list
	EnsureProcessesKilled(nil)
	EnsureProcessesKilled([]int{})
}

func TestEnsureProcessesKilled_KillsSurvivors(t *testing.T) {
	// Start a process
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	pid := cmd.Process.Pid

	// Ensure it's killed
	EnsureProcessesKilled([]int{pid})
	_ = cmd.Wait()

	if IsProcessAlive(pid) {
		t.Errorf("Process %d should be dead after EnsureProcessesKilled", pid)
	}
}

func TestEnsureProcessesKilled_DeadProcesses(t *testing.T) {
	// Should not panic when given already-dead PIDs
	EnsureProcessesKilled([]int{99999999, 99999998})
}

func TestWaitForProcessExit_AlreadyDead(t *testing.T) {
	result := WaitForProcessExit(99999999, 100*time.Millisecond)
	if !result {
		t.Error("WaitForProcessExit should return true for non-existent process")
	}
}

func TestWaitForProcessExit_InvalidPID(t *testing.T) {
	tests := []struct {
		name string
		pid  int
	}{
		{"zero", 0},
		{"negative", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WaitForProcessExit(tt.pid, 100*time.Millisecond)
			if !result {
				t.Errorf("WaitForProcessExit(%d) should return true for invalid PID", tt.pid)
			}
		})
	}
}

func TestWaitForProcessExit_ProcessExits(t *testing.T) {
	// Start a process that will exit quickly
	cmd := exec.Command("sleep", "0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	pid := cmd.Process.Pid

	// Reap the process in a goroutine so it doesn't become a zombie.
	// Zombie processes still appear alive to kill(pid, 0).
	go func() { _ = cmd.Wait() }()

	// Wait for it with a generous timeout
	result := WaitForProcessExit(pid, 2*time.Second)

	if !result {
		t.Error("WaitForProcessExit should return true when process exits within timeout")
	}
}

func TestGracefulShutdown_NonexistentSession(t *testing.T) {
	// Should not panic when called with a non-existent session/socket
	GracefulShutdown("nonexistent-socket-test", "nonexistent-session", 100*time.Millisecond)
}

func TestGracefulShutdown_Idempotent(t *testing.T) {
	// Multiple calls should not panic
	for i := 0; i < 3; i++ {
		GracefulShutdown("nonexistent-socket-test", "nonexistent-session", 100*time.Millisecond)
	}
}

func TestDefaultGracefulStopTimeout(t *testing.T) {
	if DefaultGracefulStopTimeout != 500*time.Millisecond {
		t.Errorf("DefaultGracefulStopTimeout = %v, want 500ms", DefaultGracefulStopTimeout)
	}
}

func TestWaitForProcessExit_Timeout(t *testing.T) {
	// Start a process that won't exit on its own
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	pid := cmd.Process.Pid

	// Wait with a short timeout
	result := WaitForProcessExit(pid, 150*time.Millisecond)
	if result {
		t.Error("WaitForProcessExit should return false when process doesn't exit within timeout")
	}
}

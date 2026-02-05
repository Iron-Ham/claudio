package tmux

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// DefaultGracefulStopTimeout is the default time to wait after sending Ctrl+C
// before force-killing processes during shutdown. This constant is shared across
// all stop paths (instance.Manager, lifecycle.Manager, PRWorkflow) to ensure
// consistent behavior.
const DefaultGracefulStopTimeout = 500 * time.Millisecond

// GetPanePID returns the PID of the process running in the tmux pane.
// Returns 0 if the PID cannot be determined (e.g., session doesn't exist).
func GetPanePID(socketName, sessionName string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := CommandContextWithSocket(ctx, socketName, "display-message", "-t", sessionName, "-p", "#{pane_pid}")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0
	}
	return pid
}

// GetDescendantPIDs returns all descendant PIDs of the given PID (recursive).
// Uses pgrep -P to find child processes.
func GetDescendantPIDs(pid int) []int {
	if pid <= 0 {
		return nil
	}
	return getDescendantPIDs(pid)
}

func getDescendantPIDs(pid int) []int {
	cmd := exec.Command("pgrep", "-P", strconv.Itoa(pid))
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var descendants []int
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		childPID, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		descendants = append(descendants, childPID)
		// Recursively get grandchildren
		descendants = append(descendants, getDescendantPIDs(childPID)...)
	}
	return descendants
}

// IsProcessAlive checks if a process with the given PID exists.
// Uses kill(pid, 0) which checks for process existence without sending a signal.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// On Unix, kill with signal 0 checks process existence without sending a signal.
	err := syscall.Kill(pid, 0)
	return err == nil
}

// KillProcessTree sends SIGKILL to a process and all its descendants.
// Descendants are killed first (bottom-up) to prevent orphaning.
func KillProcessTree(pid int) {
	if pid <= 0 {
		return
	}

	// Get all descendants first (before killing, so we can traverse the tree)
	descendants := GetDescendantPIDs(pid)

	// Kill descendants bottom-up (deepest children first)
	for i := len(descendants) - 1; i >= 0; i-- {
		if IsProcessAlive(descendants[i]) {
			_ = syscall.Kill(descendants[i], syscall.SIGKILL)
		}
	}

	// Kill the root process
	if IsProcessAlive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

// KillServer kills the tmux server for the given socket name.
// This is more thorough than kill-session: it terminates the server itself
// and all sessions/windows/panes within it.
func KillServer(socketName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return CommandContextWithSocket(ctx, socketName, "kill-server").Run()
}

// CollectProcessTree returns the pane PID and all its descendants.
// This should be called before initiating any shutdown to capture the full tree.
func CollectProcessTree(socketName, sessionName string) []int {
	panePID := GetPanePID(socketName, sessionName)
	if panePID <= 0 {
		return nil
	}

	pids := []int{panePID}
	pids = append(pids, GetDescendantPIDs(panePID)...)
	return pids
}

// EnsureProcessesKilled checks if any of the given PIDs are still alive
// and force-kills them along with any new descendants they may have spawned.
func EnsureProcessesKilled(pids []int) {
	for _, pid := range pids {
		if IsProcessAlive(pid) {
			KillProcessTree(pid)
		}
	}
}

// GracefulShutdown performs a defense-in-depth shutdown of a tmux session.
// It captures the process tree, sends Ctrl+C for graceful stop, polls for
// process exit, kills the session and server, then force-kills any survivors.
//
// This is the canonical shutdown sequence shared by all stop paths
// (instance.Manager, lifecycle.Manager, PRWorkflow).
func GracefulShutdown(socketName, sessionName string, gracefulTimeout time.Duration) {
	// Capture the process tree while the session is still alive so we can
	// verify all processes are dead after tmux cleanup.
	processPIDs := CollectProcessTree(socketName, sessionName)
	panePID := 0
	if len(processPIDs) > 0 {
		panePID = processPIDs[0]
	}

	// Send Ctrl+C to gracefully stop the backend
	_ = CommandWithSocket(socketName, "send-keys", "-t", sessionName, "C-c").Run()

	// Poll for process exit instead of blind sleep
	WaitForProcessExit(panePID, gracefulTimeout)

	// Kill the tmux session
	_ = CommandWithSocket(socketName, "kill-session", "-t", sessionName).Run()

	// Kill the tmux server for this socket to prevent orphaned servers
	_ = KillServer(socketName)

	// Force-kill any processes that survived the tmux shutdown
	EnsureProcessesKilled(processPIDs)
}

// WaitForProcessExit polls until the given PID exits or the timeout is reached.
// Returns true if the process exited within the timeout, false if it's still alive.
func WaitForProcessExit(pid int, timeout time.Duration) bool {
	if pid <= 0 || !IsProcessAlive(pid) {
		return true
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			return !IsProcessAlive(pid)
		case <-ticker.C:
			if !IsProcessAlive(pid) {
				return true
			}
		}
	}
}

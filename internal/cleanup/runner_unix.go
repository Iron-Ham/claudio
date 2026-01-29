//go:build unix

package cleanup

import (
	"fmt"
	"os/exec"
	"syscall"
)

// spawnDetachedProcess starts a cleanup job in a detached background process.
// On Unix, we use Setsid to create a new session, detaching from the controlling terminal.
func spawnDetachedProcess(executablePath, baseDir, jobID string) error {
	// Build command to run the cleanup job
	cmd := exec.Command(executablePath, "cleanup", "--run-job", jobID)
	cmd.Dir = baseDir

	// Detach from parent process group so the cleanup continues
	// even if the parent terminal is closed
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // Create new session, detach from controlling terminal
	}

	// Discard output - the job results are stored in the job file
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Start the process (don't wait for it)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start background cleanup: %w", err)
	}

	// Release the process so it continues running independently
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("failed to release background process: %w", err)
	}

	return nil
}

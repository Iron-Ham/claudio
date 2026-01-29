//go:build windows

package cleanup

import (
	"fmt"
	"os/exec"
	"syscall"
)

// DETACHED_PROCESS is the Windows process creation flag that creates a process
// without a console window, allowing it to run independently of the parent.
const DETACHED_PROCESS = 0x00000008

// spawnDetachedProcess starts a cleanup job in a detached background process.
// On Windows, we use CREATE_NEW_PROCESS_GROUP and DETACHED_PROCESS flags.
func spawnDetachedProcess(executablePath, baseDir, jobID string) error {
	// Build command to run the cleanup job
	cmd := exec.Command(executablePath, "cleanup", "--run-job", jobID)
	cmd.Dir = baseDir

	// On Windows, use CREATE_NEW_PROCESS_GROUP and DETACHED_PROCESS to detach
	// from the parent console and process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS,
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

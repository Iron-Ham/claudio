package cleanup

import (
	"fmt"
	"os"
)

// SpawnBackgroundCleanup starts the cleanup job in a detached background process.
// The process will read the job file and execute cleanup using only the snapshotted
// resources, ensuring resources created after the snapshot are not affected.
func SpawnBackgroundCleanup(executablePath, baseDir, jobID string) error {
	return spawnDetachedProcess(executablePath, baseDir, jobID)
}

// GetExecutablePath returns the path to the current executable
func GetExecutablePath() (string, error) {
	return os.Executable()
}

// RunJobFromFile loads and executes a cleanup job from its job file.
// This is called by the background process.
func RunJobFromFile(baseDir, jobID string) error {
	job, err := LoadJob(baseDir, jobID)
	if err != nil {
		return fmt.Errorf("failed to load job: %w", err)
	}

	// Verify job is still pending
	if job.Status != JobStatusPending {
		return fmt.Errorf("job %s is not pending (status: %s)", jobID, job.Status)
	}

	executor, err := NewExecutor(job)
	if err != nil {
		// Mark job as failed
		job.Status = JobStatusFailed
		job.Error = err.Error()
		if saveErr := job.Save(); saveErr != nil {
			return fmt.Errorf("failed to create executor: %w (additionally, failed to save job status: %v)", err, saveErr)
		}
		return fmt.Errorf("failed to create executor: %w", err)
	}

	return executor.Execute()
}

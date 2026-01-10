package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// LockFileName is the name of the lock file within a session directory
const LockFileName = "session.lock"

// ErrSessionLocked is returned when attempting to acquire a lock on an already-locked session
var ErrSessionLocked = errors.New("session is locked by another process")

// Lock represents an acquired session lock
type Lock struct {
	SessionID string    `json:"session_id"`
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	StartedAt time.Time `json:"started_at"`

	// Internal fields (not serialized)
	lockFile string
	logger   *logging.Logger
}

// AcquireLock attempts to acquire an exclusive lock on the session directory.
// Returns ErrSessionLocked if the session is already in use by another process.
// The logger parameter is optional and can be nil (useful when lock is acquired
// before the logger is fully initialized).
func AcquireLock(sessionDir, sessionID string, logger *logging.Logger) (*Lock, error) {
	lockPath := filepath.Join(sessionDir, LockFileName)

	// Check for existing lock
	if existingLock, err := ReadLock(lockPath); err == nil {
		// Lock file exists - check if the process is still alive
		if isProcessAlive(existingLock.PID) {
			reason := fmt.Sprintf("locked by PID %d on %s", existingLock.PID, existingLock.Hostname)
			if logger != nil {
				logger.Error("failed to acquire lock",
					"session_id", sessionID,
					"reason", reason,
				)
			}
			return nil, fmt.Errorf("%w: PID %d on %s", ErrSessionLocked, existingLock.PID, existingLock.Hostname)
		}
		// Stale lock - remove it
		oldPID := existingLock.PID
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			if logger != nil {
				logger.Error("failed to acquire lock",
					"session_id", sessionID,
					"reason", "failed to remove stale lock",
				)
			}
			return nil, fmt.Errorf("failed to remove stale lock: %w", err)
		}
		if logger != nil {
			logger.Warn("stale lock cleaned",
				"session_id", sessionID,
				"old_pid", oldPID,
			)
		}
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Create lock
	lock := &Lock{
		SessionID: sessionID,
		PID:       os.Getpid(),
		Hostname:  hostname,
		StartedAt: time.Now(),
		lockFile:  lockPath,
		logger:    logger,
	}

	// Write lock file
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		if logger != nil {
			logger.Error("failed to acquire lock",
				"session_id", sessionID,
				"reason", "failed to marshal lock data",
			)
		}
		return nil, fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Use O_EXCL to fail if file already exists (race condition protection)
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Another process beat us to it - re-read and report
			if existingLock, readErr := ReadLock(lockPath); readErr == nil {
				reason := fmt.Sprintf("locked by PID %d on %s (race condition)", existingLock.PID, existingLock.Hostname)
				if logger != nil {
					logger.Error("failed to acquire lock",
						"session_id", sessionID,
						"reason", reason,
					)
				}
				return nil, fmt.Errorf("%w: PID %d on %s", ErrSessionLocked, existingLock.PID, existingLock.Hostname)
			}
			if logger != nil {
				logger.Error("failed to acquire lock",
					"session_id", sessionID,
					"reason", "lock file exists (race condition)",
				)
			}
			return nil, ErrSessionLocked
		}
		if logger != nil {
			logger.Error("failed to acquire lock",
				"session_id", sessionID,
				"reason", fmt.Sprintf("failed to create lock file: %v", err),
			)
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(lockPath)
		if logger != nil {
			logger.Error("failed to acquire lock",
				"session_id", sessionID,
				"reason", "failed to write lock file",
			)
		}
		return nil, fmt.Errorf("failed to write lock file: %w", err)
	}

	if logger != nil {
		logger.Info("session lock acquired",
			"session_id", sessionID,
			"pid", lock.PID,
		)
	}

	return lock, nil
}

// Release releases the session lock by removing the lock file.
// Safe to call multiple times.
func (l *Lock) Release() error {
	if l == nil || l.lockFile == "" {
		return nil
	}

	// Only remove if we own the lock (PID matches)
	existingLock, err := ReadLock(l.lockFile)
	if err != nil {
		// Lock file doesn't exist or can't be read - nothing to do
		return nil
	}

	if existingLock.PID != l.PID {
		// Not our lock - don't remove it
		return nil
	}

	if err := os.Remove(l.lockFile); err != nil {
		return err
	}

	if l.logger != nil {
		l.logger.Info("session lock released",
			"session_id", l.SessionID,
		)
	}

	return nil
}

// ReadLock reads a lock file and returns the Lock info.
func ReadLock(lockPath string) (*Lock, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}

	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse lock file: %w", err)
	}
	lock.lockFile = lockPath

	return &lock, nil
}

// IsLocked checks if a session directory is currently locked.
// Returns the lock info if locked, nil otherwise.
func IsLocked(sessionDir string) (*Lock, bool) {
	lockPath := filepath.Join(sessionDir, LockFileName)

	lock, err := ReadLock(lockPath)
	if err != nil {
		return nil, false
	}

	// Check if the process is still alive
	if !isProcessAlive(lock.PID) {
		// Stale lock
		return lock, false
	}

	return lock, true
}

// CleanStaleLock removes a stale lock file if the owning process is no longer running.
// Returns true if a stale lock was cleaned.
// The logger parameter is optional and can be nil.
func CleanStaleLock(sessionDir string, logger *logging.Logger) (bool, error) {
	lockPath := filepath.Join(sessionDir, LockFileName)

	lock, err := ReadLock(lockPath)
	if err != nil {
		// No lock file
		return false, nil
	}

	if isProcessAlive(lock.PID) {
		// Process is still running - not stale
		return false, nil
	}

	oldPID := lock.PID
	sessionID := lock.SessionID

	// Stale lock - remove it
	if err := os.Remove(lockPath); err != nil {
		return false, fmt.Errorf("failed to remove stale lock: %w", err)
	}

	if logger != nil {
		logger.Warn("stale lock cleaned",
			"session_id", sessionID,
			"old_pid", oldPID,
		)
	}

	return true, nil
}

// isProcessAlive checks if a process with the given PID is still running.
func isProcessAlive(pid int) bool {
	// On Unix, sending signal 0 checks if process exists without affecting it
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
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
}

// AcquireLock attempts to acquire an exclusive lock on the session directory.
// Returns ErrSessionLocked if the session is already in use by another process.
func AcquireLock(sessionDir, sessionID string) (*Lock, error) {
	lockPath := filepath.Join(sessionDir, LockFileName)

	// Check for existing lock
	if existingLock, err := ReadLock(lockPath); err == nil {
		// Lock file exists - check if the process is still alive
		if isProcessAlive(existingLock.PID) {
			return nil, fmt.Errorf("%w: PID %d on %s", ErrSessionLocked, existingLock.PID, existingLock.Hostname)
		}
		// Stale lock - remove it
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove stale lock: %w", err)
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
	}

	// Write lock file
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lock: %w", err)
	}

	// Use O_EXCL to fail if file already exists (race condition protection)
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Another process beat us to it - re-read and report
			if existingLock, readErr := ReadLock(lockPath); readErr == nil {
				return nil, fmt.Errorf("%w: PID %d on %s", ErrSessionLocked, existingLock.PID, existingLock.Hostname)
			}
			return nil, ErrSessionLocked
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(lockPath)
		return nil, fmt.Errorf("failed to write lock file: %w", err)
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

	return os.Remove(l.lockFile)
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
func CleanStaleLock(sessionDir string) (bool, error) {
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

	// Stale lock - remove it
	if err := os.Remove(lockPath); err != nil {
		return false, fmt.Errorf("failed to remove stale lock: %w", err)
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

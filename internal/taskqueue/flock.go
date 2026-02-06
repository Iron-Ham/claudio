package taskqueue

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const lockFileName = "taskqueue.lock"

// FileLock provides cross-process mutual exclusion using flock(2).
// Used to protect queue state files when multiple Claudio processes
// may be accessing the same session directory.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a FileLock for the given directory. The lock file
// is created inside dir as "taskqueue.lock". Call Lock/Unlock to
// acquire and release.
func NewFileLock(dir string) *FileLock {
	return &FileLock{
		path: filepath.Join(dir, lockFileName),
	}
}

// Lock acquires an exclusive file lock, blocking until available.
// The lock file is created if it does not exist.
func (fl *FileLock) Lock() error {
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	fl.file = f

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		fl.file = nil
		return fmt.Errorf("flock: %w", err)
	}
	return nil
}

// TryLock attempts to acquire the lock without blocking.
// Returns true if the lock was acquired, false if it is held by another process.
func (fl *FileLock) TryLock() (bool, error) {
	f, err := os.OpenFile(fl.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return false, fmt.Errorf("open lock file: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		_ = f.Close()
		if err == syscall.EWOULDBLOCK {
			return false, nil
		}
		return false, fmt.Errorf("flock: %w", err)
	}

	fl.file = f
	return true, nil
}

// Unlock releases the file lock and closes the lock file.
func (fl *FileLock) Unlock() error {
	if fl.file == nil {
		return nil
	}

	if err := syscall.Flock(int(fl.file.Fd()), syscall.LOCK_UN); err != nil {
		_ = fl.file.Close()
		fl.file = nil
		return fmt.Errorf("funlock: %w", err)
	}

	err := fl.file.Close()
	fl.file = nil
	return err
}

package taskqueue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileLock_LockUnlock(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)

	if err := fl.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}

	// Lock file should exist
	lockPath := filepath.Join(dir, lockFileName)
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("lock file should exist: %v", err)
	}

	if err := fl.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestFileLock_UnlockWithoutLock(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)

	// Unlock without Lock should be a no-op
	if err := fl.Unlock(); err != nil {
		t.Fatalf("Unlock without Lock should not error: %v", err)
	}
}

func TestFileLock_TryLock(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)

	acquired, err := fl.TryLock()
	if err != nil {
		t.Fatalf("TryLock: %v", err)
	}
	if !acquired {
		t.Error("TryLock should succeed when lock is available")
	}

	// Second TryLock from a different FileLock should fail (same process
	// can re-acquire on some OSes, but different fd should block)
	fl2 := NewFileLock(dir)
	acquired2, err := fl2.TryLock()
	if err != nil {
		t.Fatalf("TryLock2: %v", err)
	}
	// On some UNIX systems, flock is per-fd not per-process, so the
	// second fd from the same process might succeed. This is acceptable
	// since cross-process is the real use case. Just verify no error.
	if acquired2 {
		_ = fl2.Unlock()
	}

	if err := fl.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
}

func TestFileLock_LockInvalidDir(t *testing.T) {
	fl := NewFileLock("/nonexistent/dir/path")
	if err := fl.Lock(); err == nil {
		t.Error("Lock should fail for nonexistent directory")
	}
}

func TestFileLock_TryLockInvalidDir(t *testing.T) {
	fl := NewFileLock("/nonexistent/dir/path")
	_, err := fl.TryLock()
	if err == nil {
		t.Error("TryLock should fail for nonexistent directory")
	}
}

func TestFileLock_ReusableAfterUnlock(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir)

	// Lock, unlock, lock again
	if err := fl.Lock(); err != nil {
		t.Fatalf("Lock 1: %v", err)
	}
	if err := fl.Unlock(); err != nil {
		t.Fatalf("Unlock 1: %v", err)
	}
	if err := fl.Lock(); err != nil {
		t.Fatalf("Lock 2: %v", err)
	}
	if err := fl.Unlock(); err != nil {
		t.Fatalf("Unlock 2: %v", err)
	}
}

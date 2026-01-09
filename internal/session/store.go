// Package session provides concrete implementations of the persistence interfaces
// defined in interfaces.go. These implementations use the local filesystem for
// storage, with JSON encoding for structured data.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// -----------------------------------------------------------------------------
// FileStore - Generic Key-Value File Storage
// -----------------------------------------------------------------------------

// FileStore provides a file-based implementation of the Store interface.
// Each key maps to a file within a base directory, with keys using "/" as
// path separators.
type FileStore struct {
	baseDir string
	mu      sync.RWMutex
}

// NewFileStore creates a new FileStore rooted at the given directory.
// The directory will be created if it doesn't exist.
func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}
	return &FileStore{baseDir: baseDir}, nil
}

// Save persists data with the given key using atomic write.
func (fs *FileStore) Save(ctx context.Context, key string, data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return atomicWriteFile(path, data, 0644)
}

// Load retrieves data for the given key.
func (fs *FileStore) Load(ctx context.Context, key string) ([]byte, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.keyToPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return data, nil
}

// Delete removes the data associated with the given key.
func (fs *FileStore) Delete(ctx context.Context, key string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// List returns all keys matching the given prefix.
func (fs *FileStore) List(ctx context.Context, prefix string) ([]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	searchDir := fs.baseDir
	if prefix != "" {
		searchDir = filepath.Join(fs.baseDir, prefix)
	}

	var keys []string
	err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // Directory doesn't exist, no keys
			}
			return err
		}
		if !info.IsDir() {
			// Convert path back to key
			rel, err := filepath.Rel(fs.baseDir, path)
			if err != nil {
				return err
			}
			key := filepath.ToSlash(rel)
			if prefix == "" || strings.HasPrefix(key, prefix) {
				keys = append(keys, key)
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}
	return keys, nil
}

// Exists checks if a key exists without loading its data.
func (fs *FileStore) Exists(ctx context.Context, key string) (bool, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.keyToPath(key)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return true, nil
}

// SaveIfNotExists saves data only if the key does not already exist.
func (fs *FileStore) SaveIfNotExists(ctx context.Context, key string, data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Use O_EXCL to fail if file exists
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(path) // Clean up on failure
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// SaveWithVersion saves data with optimistic concurrency control.
// For file-based storage, we use modification time as a simple version.
func (fs *FileStore) SaveWithVersion(ctx context.Context, key string, data []byte, version int64) (int64, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create directory: %w", err)
	}

	// Check current version
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, fmt.Errorf("failed to stat file: %w", err)
		}
		// File doesn't exist
		if version != 0 {
			return 0, ErrNotFound
		}
	} else {
		// File exists
		if version == 0 {
			return 0, ErrAlreadyExists
		}
		currentVersion := info.ModTime().UnixNano()
		if currentVersion != version {
			return 0, ErrStaleData
		}
	}

	// Write atomically
	if err := atomicWriteFile(path, data, 0644); err != nil {
		return 0, err
	}

	// Get new version (modification time)
	newInfo, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("failed to get new version: %w", err)
	}

	return newInfo.ModTime().UnixNano(), nil
}

// LoadWithVersion retrieves data along with its current version.
func (fs *FileStore) LoadWithVersion(ctx context.Context, key string) ([]byte, int64, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.keyToPath(key)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, fmt.Errorf("failed to stat file: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read file: %w", err)
	}

	return data, info.ModTime().UnixNano(), nil
}

// keyToPath converts a key to a filesystem path.
func (fs *FileStore) keyToPath(key string) string {
	// Convert "/" in key to path separator
	return filepath.Join(fs.baseDir, filepath.FromSlash(key))
}

// -----------------------------------------------------------------------------
// FileSessionStore - Session-Specific File Storage
// -----------------------------------------------------------------------------

// FileSessionStore provides a file-based implementation of the SessionStore interface.
// Sessions are stored as JSON files in a structured directory hierarchy.
type FileSessionStore struct {
	baseDir  string
	store    *FileStore
	mu       sync.RWMutex
}

// NewFileSessionStore creates a new FileSessionStore.
// baseDir should be the project root directory (e.g., the git repository root).
func NewFileSessionStore(baseDir string) (*FileSessionStore, error) {
	sessionsDir := GetSessionsDir(baseDir)
	store, err := NewFileStore(sessionsDir)
	if err != nil {
		return nil, err
	}
	return &FileSessionStore{
		baseDir: baseDir,
		store:   store,
	}, nil
}

// SaveSession persists a session's state.
func (fss *FileSessionStore) SaveSession(ctx context.Context, session Persistable) error {
	fss.mu.Lock()
	defer fss.mu.Unlock()

	sessionID := session.GetID()
	if sessionID == "" {
		return fmt.Errorf("session ID cannot be empty")
	}

	// Marshal the session to JSON
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Write to session file using atomic write
	sessionDir := GetSessionDir(fss.baseDir, sessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	sessionFile := filepath.Join(sessionDir, SessionFileName)
	return atomicWriteFile(sessionFile, data, 0644)
}

// LoadSession retrieves a session by ID.
func (fss *FileSessionStore) LoadSession(ctx context.Context, sessionID string) ([]byte, error) {
	fss.mu.RLock()
	defer fss.mu.RUnlock()

	sessionFile := filepath.Join(GetSessionDir(fss.baseDir, sessionID), SessionFileName)
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Validate JSON
	var raw json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSessionCorrupted, err)
	}

	return data, nil
}

// ListSessions returns summary information for all sessions.
func (fss *FileSessionStore) ListSessions(ctx context.Context) ([]*Info, error) {
	fss.mu.RLock()
	defer fss.mu.RUnlock()

	return ListSessions(fss.baseDir)
}

// DeleteSession removes a session and all associated data.
func (fss *FileSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	fss.mu.Lock()
	defer fss.mu.Unlock()

	sessionDir := GetSessionDir(fss.baseDir, sessionID)

	// Check if session exists
	if _, err := os.Stat(sessionDir); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to check session directory: %w", err)
	}

	// Remove the entire session directory
	if err := os.RemoveAll(sessionDir); err != nil {
		return fmt.Errorf("failed to delete session directory: %w", err)
	}

	return nil
}

// SessionExists checks if a session with the given ID exists.
func (fss *FileSessionStore) SessionExists(ctx context.Context, sessionID string) bool {
	fss.mu.RLock()
	defer fss.mu.RUnlock()

	return SessionExists(fss.baseDir, sessionID)
}

// GetSessionDir returns the storage path for a session.
func (fss *FileSessionStore) GetSessionDir(sessionID string) string {
	return GetSessionDir(fss.baseDir, sessionID)
}

// BaseDir returns the base directory for this store.
func (fss *FileSessionStore) BaseDir() string {
	return fss.baseDir
}

// -----------------------------------------------------------------------------
// FileLockManager - File-Based Distributed Locking
// -----------------------------------------------------------------------------

// FileLockManager provides a file-based implementation of the LockManager interface.
// Locks are implemented using lock files with process liveness checks.
type FileLockManager struct {
	baseDir string
	mu      sync.Mutex
}

// NewFileLockManager creates a new FileLockManager.
func NewFileLockManager(baseDir string) *FileLockManager {
	return &FileLockManager{baseDir: baseDir}
}

// Acquire attempts to acquire an exclusive lock on a session.
func (flm *FileLockManager) Acquire(ctx context.Context, sessionID string) (LockHandle, error) {
	return flm.TryAcquire(ctx, sessionID)
}

// TryAcquire attempts to acquire a lock without blocking.
func (flm *FileLockManager) TryAcquire(ctx context.Context, sessionID string) (LockHandle, error) {
	flm.mu.Lock()
	defer flm.mu.Unlock()

	sessionDir := GetSessionDir(flm.baseDir, sessionID)

	// Ensure session directory exists
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	lockPath := filepath.Join(sessionDir, LockFileName)

	// Check for existing lock
	if existingLock, err := ReadLock(lockPath); err == nil {
		// Lock file exists - check if the process is still alive
		if isProcessAlive(existingLock.PID) {
			return nil, fmt.Errorf("%w: held by PID %d on %s", ErrAlreadyLocked, existingLock.PID, existingLock.Hostname)
		}
		// Stale lock - remove it
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to remove stale lock: %w", err)
		}
	}

	// Generate a unique holder ID
	holderID := generateHolderID()

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	now := time.Now()
	lockInfo := &LockInfo{
		SessionID:  sessionID,
		HolderID:   holderID,
		PID:        os.Getpid(),
		Hostname:   hostname,
		AcquiredAt: now,
	}

	// Write lock file atomically
	data, err := json.MarshalIndent(lockInfo, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal lock info: %w", err)
	}

	// Use O_EXCL to fail if file already exists (race condition protection)
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Another process beat us to it
			if existingLock, readErr := ReadLock(lockPath); readErr == nil {
				return nil, fmt.Errorf("%w: held by PID %d on %s", ErrAlreadyLocked, existingLock.PID, existingLock.Hostname)
			}
			return nil, ErrAlreadyLocked
		}
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		os.Remove(lockPath)
		return nil, fmt.Errorf("failed to write lock file: %w", err)
	}

	return &fileLockHandle{
		lockInfo: lockInfo,
		lockPath: lockPath,
		manager:  flm,
	}, nil
}

// IsLocked checks if a session is currently locked.
func (flm *FileLockManager) IsLocked(ctx context.Context, sessionID string) (*LockInfo, bool) {
	sessionDir := GetSessionDir(flm.baseDir, sessionID)
	lockPath := filepath.Join(sessionDir, LockFileName)

	lock, err := ReadLock(lockPath)
	if err != nil {
		return nil, false
	}

	// Convert Lock to LockInfo
	lockInfo := &LockInfo{
		SessionID:  lock.SessionID,
		HolderID:   "", // Legacy locks don't have holder ID
		PID:        lock.PID,
		Hostname:   lock.Hostname,
		AcquiredAt: lock.StartedAt,
	}

	// Check if the process is still alive
	if !isProcessAlive(lock.PID) {
		return lockInfo, false
	}

	return lockInfo, true
}

// GetHolder returns information about the current lock holder.
func (flm *FileLockManager) GetHolder(ctx context.Context, sessionID string) (*LockInfo, error) {
	lockInfo, isLocked := flm.IsLocked(ctx, sessionID)
	if !isLocked {
		return nil, ErrNotFound
	}
	return lockInfo, nil
}

// ForceRelease forcibly releases a lock, regardless of holder.
func (flm *FileLockManager) ForceRelease(ctx context.Context, sessionID string) error {
	flm.mu.Lock()
	defer flm.mu.Unlock()

	sessionDir := GetSessionDir(flm.baseDir, sessionID)
	lockPath := filepath.Join(sessionDir, LockFileName)

	if err := os.Remove(lockPath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to remove lock file: %w", err)
	}
	return nil
}

// fileLockHandle implements the LockHandle interface.
type fileLockHandle struct {
	lockInfo *LockInfo
	lockPath string
	manager  *FileLockManager
	released bool
	mu       sync.Mutex
}

// Release releases the lock.
func (h *fileLockHandle) Release() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.released {
		return nil
	}

	// Verify we still own the lock
	existingLock, err := ReadLock(h.lockPath)
	if err != nil {
		// Lock file doesn't exist - already released
		h.released = true
		return nil
	}

	if existingLock.PID != h.lockInfo.PID {
		// Not our lock anymore
		h.released = true
		return ErrLockNotHeld
	}

	if err := os.Remove(h.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	h.released = true
	return nil
}

// SessionID returns the ID of the locked session.
func (h *fileLockHandle) SessionID() string {
	return h.lockInfo.SessionID
}

// Info returns information about this lock.
func (h *fileLockHandle) Info() *LockInfo {
	return h.lockInfo
}

// Refresh extends the lock's lifetime.
// For file-based locks, this updates the lock file's modification time.
func (h *fileLockHandle) Refresh(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.released {
		return ErrLockNotHeld
	}

	// Verify we still own the lock
	existingLock, err := ReadLock(h.lockPath)
	if err != nil {
		h.released = true
		return ErrLockNotHeld
	}

	if existingLock.PID != h.lockInfo.PID {
		h.released = true
		return ErrLockNotHeld
	}

	// Update the file's modification time
	now := time.Now()
	if err := os.Chtimes(h.lockPath, now, now); err != nil {
		return fmt.Errorf("failed to refresh lock: %w", err)
	}

	return nil
}

// -----------------------------------------------------------------------------
// FileRecoveryManager - File-Based Recovery
// -----------------------------------------------------------------------------

// FileRecoveryManager provides a file-based implementation of the RecoveryManager interface.
type FileRecoveryManager struct {
	baseDir     string
	lockManager *FileLockManager
	mu          sync.Mutex
}

// NewFileRecoveryManager creates a new FileRecoveryManager.
func NewFileRecoveryManager(baseDir string) *FileRecoveryManager {
	return &FileRecoveryManager{
		baseDir:     baseDir,
		lockManager: NewFileLockManager(baseDir),
	}
}

// CheckForRecovery scans for sessions that may need recovery.
func (frm *FileRecoveryManager) CheckForRecovery(ctx context.Context) ([]*RecoveryCandidate, error) {
	frm.mu.Lock()
	defer frm.mu.Unlock()

	sessionsDir := GetSessionsDir(frm.baseDir)

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var candidates []*RecoveryCandidate

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		sessionDir := GetSessionDir(frm.baseDir, sessionID)
		sessionFile := filepath.Join(sessionDir, SessionFileName)

		// Check if session file exists
		sessionInfo, err := os.Stat(sessionFile)
		if err != nil {
			continue // Skip sessions without valid session file
		}

		// Check for stale locks
		lockPath := filepath.Join(sessionDir, LockFileName)
		lock, err := ReadLock(lockPath)
		hasStaleLock := false
		var lockInfo *LockInfo

		if err == nil {
			lockInfo = &LockInfo{
				SessionID:  lock.SessionID,
				HolderID:   "",
				PID:        lock.PID,
				Hostname:   lock.Hostname,
				AcquiredAt: lock.StartedAt,
			}

			// Check if the process is still alive
			if !isProcessAlive(lock.PID) {
				hasStaleLock = true
			}
		}

		// Only add as candidate if there's something to recover
		if hasStaleLock {
			candidates = append(candidates, &RecoveryCandidate{
				SessionID:    sessionID,
				SessionDir:   sessionDir,
				LastModified: sessionInfo.ModTime(),
				HasStaleLock: hasStaleLock,
				LockInfo:     lockInfo,
				Reason:       "stale lock detected - owning process no longer running",
			})
		}
	}

	return candidates, nil
}

// RecoverSession attempts to recover a specific session.
func (frm *FileRecoveryManager) RecoverSession(ctx context.Context, sessionID string) (*RecoveryResult, error) {
	frm.mu.Lock()
	defer frm.mu.Unlock()

	sessionDir := GetSessionDir(frm.baseDir, sessionID)
	result := &RecoveryResult{
		SessionID: sessionID,
	}

	// Check if session exists
	sessionFile := filepath.Join(sessionDir, SessionFileName)
	if _, err := os.Stat(sessionFile); err != nil {
		if os.IsNotExist(err) {
			result.Error = ErrNotFound
			return result, ErrNotFound
		}
		result.Error = fmt.Errorf("failed to check session: %w", err)
		return result, result.Error
	}

	// Clean stale lock if present
	lockPath := filepath.Join(sessionDir, LockFileName)
	lock, err := ReadLock(lockPath)
	if err == nil && !isProcessAlive(lock.PID) {
		if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
			result.Error = fmt.Errorf("failed to remove stale lock: %w", err)
			return result, result.Error
		}
		result.CleanedUp = true
	}

	result.Recovered = true
	return result, nil
}

// CleanupStale removes resources from sessions that are no longer recoverable.
func (frm *FileRecoveryManager) CleanupStale(ctx context.Context) (int, error) {
	frm.mu.Lock()
	defer frm.mu.Unlock()

	cleaned, err := CleanupStaleLocks(frm.baseDir)
	if err != nil {
		return 0, err
	}
	return len(cleaned), nil
}

// ValidateSession checks if a session's data is consistent and valid.
func (frm *FileRecoveryManager) ValidateSession(ctx context.Context, sessionID string) error {
	sessionDir := GetSessionDir(frm.baseDir, sessionID)
	sessionFile := filepath.Join(sessionDir, SessionFileName)

	// Read session file
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to read session file: %w", err)
	}

	// Validate JSON structure
	var sessionData SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return fmt.Errorf("%w: invalid JSON: %v", ErrSessionCorrupted, err)
	}

	// Validate required fields
	if sessionData.ID == "" {
		return fmt.Errorf("%w: missing session ID", ErrSessionCorrupted)
	}

	if sessionData.ID != sessionID {
		return fmt.Errorf("%w: session ID mismatch (file: %s, expected: %s)", ErrSessionCorrupted, sessionData.ID, sessionID)
	}

	return nil
}

// -----------------------------------------------------------------------------
// FilePersistenceLayer - Composite Implementation
// -----------------------------------------------------------------------------

// FilePersistenceLayer provides a complete file-based implementation
// of the PersistenceLayer interface.
type FilePersistenceLayer struct {
	*FileSessionStore
	*FileLockManager
	*FileRecoveryManager
}

// NewFilePersistenceLayer creates a new FilePersistenceLayer.
func NewFilePersistenceLayer(baseDir string) (*FilePersistenceLayer, error) {
	sessionStore, err := NewFileSessionStore(baseDir)
	if err != nil {
		return nil, err
	}

	return &FilePersistenceLayer{
		FileSessionStore:    sessionStore,
		FileLockManager:     NewFileLockManager(baseDir),
		FileRecoveryManager: NewFileRecoveryManager(baseDir),
	}, nil
}

// Close releases any resources held by the persistence layer.
func (fpl *FilePersistenceLayer) Close() error {
	// File-based implementation has no resources to close
	return nil
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// atomicWriteFile writes data to a file atomically by writing to a temporary
// file first, then renaming. This ensures the target file is never in a
// partially-written state.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create temp file in same directory to ensure atomic rename
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	// Write data
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// generateHolderID generates a unique identifier for lock holders.
func generateHolderID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}


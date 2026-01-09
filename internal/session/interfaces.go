// Package session provides interfaces and types for session persistence, locking,
// and recovery. These interfaces abstract the underlying storage mechanism,
// enabling implementations backed by files, databases, or other storage systems.
package session

import (
	"context"
	"errors"
	"time"
)

// -----------------------------------------------------------------------------
// Error Types
// -----------------------------------------------------------------------------

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrAlreadyExists is returned when attempting to create a resource that already exists.
var ErrAlreadyExists = errors.New("already exists")

// ErrAlreadyLocked is returned when attempting to acquire a lock that is already held.
var ErrAlreadyLocked = errors.New("already locked")

// ErrLockNotHeld is returned when attempting to release a lock that is not held by the caller.
var ErrLockNotHeld = errors.New("lock not held")

// ErrSessionCorrupted is returned when session data cannot be parsed or is invalid.
var ErrSessionCorrupted = errors.New("session data corrupted")

// ErrStaleData is returned when the data has been modified by another process.
var ErrStaleData = errors.New("stale data detected")

// LockInfo contains information about an active lock.
type LockInfo struct {
	SessionID string    `json:"session_id"`
	HolderID  string    `json:"holder_id"` // Process/instance identifier
	PID       int       `json:"pid"`
	Hostname  string    `json:"hostname"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// RecoveryCandidate represents a session that may need recovery.
type RecoveryCandidate struct {
	SessionID   string    `json:"session_id"`
	SessionDir  string    `json:"session_dir"`
	LastModified time.Time `json:"last_modified"`
	HasStaleLock bool      `json:"has_stale_lock"`
	LockInfo    *LockInfo `json:"lock_info,omitempty"`
	Reason      string    `json:"reason"` // Why this session needs recovery
}

// RecoveryResult contains the outcome of a recovery attempt.
type RecoveryResult struct {
	SessionID       string   `json:"session_id"`
	Recovered       bool     `json:"recovered"`
	ReconnectedIDs  []string `json:"reconnected_ids,omitempty"`  // Instance IDs that were reconnected
	PausedIDs       []string `json:"paused_ids,omitempty"`       // Instance IDs that were paused
	CleanedUp       bool     `json:"cleaned_up"`                  // Whether stale resources were cleaned
	Error           error    `json:"error,omitempty"`
}

// -----------------------------------------------------------------------------
// Store Interface - Generic Key-Value Persistence
// -----------------------------------------------------------------------------

// Store provides generic key-value persistence operations.
// This interface abstracts low-level storage, allowing different backends
// (filesystem, database, cloud storage) to be used interchangeably.
type Store interface {
	// Save persists data with the given key. If the key already exists,
	// the data is overwritten.
	Save(ctx context.Context, key string, data []byte) error

	// Load retrieves data for the given key.
	// Returns ErrNotFound if the key does not exist.
	Load(ctx context.Context, key string) ([]byte, error)

	// Delete removes the data associated with the given key.
	// Returns ErrNotFound if the key does not exist.
	Delete(ctx context.Context, key string) error

	// List returns all keys matching the given prefix.
	// An empty prefix returns all keys.
	List(ctx context.Context, prefix string) ([]string, error)

	// Exists checks if a key exists without loading its data.
	Exists(ctx context.Context, key string) (bool, error)
}

// AtomicStore extends Store with atomic operations for scenarios
// requiring consistency guarantees.
type AtomicStore interface {
	Store

	// SaveIfNotExists saves data only if the key does not already exist.
	// Returns ErrAlreadyExists if the key exists.
	SaveIfNotExists(ctx context.Context, key string, data []byte) error

	// SaveWithVersion saves data with optimistic concurrency control.
	// The version should match the current version, or ErrStaleData is returned.
	// A zero version indicates a new key (must not exist).
	SaveWithVersion(ctx context.Context, key string, data []byte, version int64) (newVersion int64, err error)

	// LoadWithVersion retrieves data along with its current version.
	LoadWithVersion(ctx context.Context, key string) (data []byte, version int64, err error)
}

// -----------------------------------------------------------------------------
// SessionStore Interface - Session-Specific Persistence
// -----------------------------------------------------------------------------

// Persistable represents any session state that can be persisted.
// This interface allows the SessionStore to work with different
// session types without circular dependencies on the orchestrator package.
type Persistable interface {
	// GetID returns the unique session identifier.
	GetID() string

	// GetName returns the human-readable session name.
	GetName() string

	// GetCreated returns when the session was created.
	GetCreated() time.Time
}

// SessionStore provides session-specific persistence operations.
// It builds on Store but adds session-aware semantics like
// validation, metadata tracking, and structured queries.
type SessionStore interface {
	// SaveSession persists a session's state.
	// The session data should be JSON-serializable.
	SaveSession(ctx context.Context, session Persistable) error

	// LoadSession retrieves a session by ID.
	// Returns ErrNotFound if the session does not exist.
	// Returns ErrSessionCorrupted if the data cannot be parsed.
	LoadSession(ctx context.Context, sessionID string) ([]byte, error)

	// ListSessions returns summary information for all sessions.
	// This is optimized to avoid loading full session data.
	ListSessions(ctx context.Context) ([]*Info, error)

	// DeleteSession removes a session and all associated data.
	// This includes the session file, context, and any related metadata.
	// Returns ErrNotFound if the session does not exist.
	DeleteSession(ctx context.Context, sessionID string) error

	// SessionExists checks if a session with the given ID exists.
	SessionExists(ctx context.Context, sessionID string) bool

	// GetSessionDir returns the storage path/location for a session.
	// For file-based stores, this is the directory path.
	// For database stores, this might return a logical identifier.
	GetSessionDir(sessionID string) string
}

// -----------------------------------------------------------------------------
// LockManager Interface - Distributed Locking
// -----------------------------------------------------------------------------

// LockManager provides distributed locking for session coordination.
// Locks ensure only one process operates on a session at a time,
// preventing data corruption from concurrent modifications.
type LockManager interface {
	// Acquire attempts to acquire an exclusive lock on a session.
	// Returns ErrAlreadyLocked if the session is locked by another process.
	// The returned LockHandle must be used to release the lock.
	Acquire(ctx context.Context, sessionID string) (LockHandle, error)

	// TryAcquire attempts to acquire a lock without blocking.
	// Returns immediately with ErrAlreadyLocked if unavailable.
	TryAcquire(ctx context.Context, sessionID string) (LockHandle, error)

	// IsLocked checks if a session is currently locked.
	// Returns the lock info if locked, nil otherwise.
	IsLocked(ctx context.Context, sessionID string) (*LockInfo, bool)

	// GetHolder returns information about the current lock holder.
	// Returns ErrNotFound if the session is not locked.
	GetHolder(ctx context.Context, sessionID string) (*LockInfo, error)

	// ForceRelease forcibly releases a lock, regardless of holder.
	// This should only be used for recovery from stuck locks.
	// Returns ErrNotFound if no lock exists.
	ForceRelease(ctx context.Context, sessionID string) error
}

// LockHandle represents an acquired lock that can be released.
// Implementations should ensure locks are released even if the
// process crashes (e.g., via process liveness checks).
type LockHandle interface {
	// Release releases the lock.
	// Safe to call multiple times; subsequent calls are no-ops.
	Release() error

	// SessionID returns the ID of the locked session.
	SessionID() string

	// Info returns information about this lock.
	Info() *LockInfo

	// Refresh extends the lock's lifetime (for time-based locks).
	// Returns ErrLockNotHeld if the lock was lost.
	Refresh(ctx context.Context) error
}

// -----------------------------------------------------------------------------
// RecoveryManager Interface - Crash Recovery
// -----------------------------------------------------------------------------

// RecoveryManager handles detection and recovery of sessions
// from crashed or interrupted processes.
type RecoveryManager interface {
	// CheckForRecovery scans for sessions that may need recovery.
	// This includes sessions with stale locks, orphaned tmux sessions,
	// or inconsistent state.
	CheckForRecovery(ctx context.Context) ([]*RecoveryCandidate, error)

	// RecoverSession attempts to recover a specific session.
	// This may involve:
	// - Reconnecting to existing tmux sessions
	// - Cleaning up stale locks
	// - Marking interrupted instances as paused
	// - Validating and repairing session state
	RecoverSession(ctx context.Context, sessionID string) (*RecoveryResult, error)

	// CleanupStale removes resources from sessions that are no longer recoverable.
	// This includes worktrees, lock files, and other orphaned resources.
	// Returns the number of resources cleaned up.
	CleanupStale(ctx context.Context) (cleaned int, err error)

	// ValidateSession checks if a session's data is consistent and valid.
	// Returns ErrSessionCorrupted with details if validation fails.
	ValidateSession(ctx context.Context, sessionID string) error
}

// -----------------------------------------------------------------------------
// Composite Interface - Full Persistence Layer
// -----------------------------------------------------------------------------

// PersistenceLayer combines all persistence-related interfaces
// into a single facade for convenience.
type PersistenceLayer interface {
	SessionStore
	LockManager
	RecoveryManager

	// Close releases any resources held by the persistence layer.
	Close() error
}

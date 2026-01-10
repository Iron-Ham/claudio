// Package session provides session management interfaces and types for the orchestrator.
// This package defines the contract for session lifecycle management, separating
// the persistence and management concerns from the core orchestration logic.
package session

import (
	"time"
)

// FormatVersion defines the current persistence format version for sessions.
// This allows for future migrations when the session format changes.
const FormatVersion = 1

// SessionState represents the overall state of a session.
type SessionState string

const (
	// StateActive indicates the session is currently active and can run instances.
	StateActive SessionState = "active"

	// StatePaused indicates the session is paused; instances are suspended but recoverable.
	StatePaused SessionState = "paused"

	// StateComplete indicates the session has finished all work successfully.
	StateComplete SessionState = "complete"

	// StateFailed indicates the session terminated due to an error.
	StateFailed SessionState = "failed"
)

// IsTerminal returns true if the session state is a terminal state (complete or failed).
func (s SessionState) IsTerminal() bool {
	return s == StateComplete || s == StateFailed
}

// String returns the string representation of the session state.
func (s SessionState) String() string {
	return string(s)
}

// SessionConfig holds configuration options for creating a new session.
type SessionConfig struct {
	// Name is the human-readable name for the session.
	Name string

	// BaseDir is the root directory of the repository.
	BaseDir string

	// BaseBranch is the git branch to base worktrees on (e.g., "main").
	// If empty, uses the repository's default branch.
	BaseBranch string

	// MaxInstances limits the number of concurrent instances (0 = unlimited).
	MaxInstances int

	// EnableConflictDetection enables automatic conflict detection between instances.
	EnableConflictDetection bool

	// Metadata holds arbitrary key-value pairs for user-defined session metadata.
	Metadata map[string]string
}

// Session represents a Claudio work session with extended state management.
// This struct is designed to be serializable for persistence while containing
// all information needed to manage and recover a session.
type Session struct {
	// ID is the unique identifier for the session.
	ID string `json:"id"`

	// Name is the human-readable name for the session.
	Name string `json:"name"`

	// BaseRepo is the path to the base repository.
	BaseRepo string `json:"base_repo"`

	// BaseBranch is the git branch that worktrees are based on.
	BaseBranch string `json:"base_branch,omitempty"`

	// State is the current state of the session.
	State SessionState `json:"state"`

	// Created is the timestamp when the session was created.
	Created time.Time `json:"created"`

	// Updated is the timestamp of the last session update.
	Updated time.Time `json:"updated"`

	// FormatVersion tracks the persistence format for migration support.
	FormatVersion int `json:"format_version"`

	// InstanceIDs contains the IDs of all instances in this session.
	// The actual Instance data is managed separately to allow for
	// independent instance state updates.
	InstanceIDs []string `json:"instance_ids"`

	// Metadata holds arbitrary key-value pairs for user-defined session metadata.
	Metadata map[string]string `json:"metadata,omitempty"`

	// UltraPlanID references an associated ultra-plan session, if any.
	// This allows linking regular sessions with complex planning sessions.
	UltraPlanID string `json:"ultra_plan_id,omitempty"`
}

// RecoveryInfo contains information about a recovered session.
type RecoveryInfo struct {
	// Session is the recovered session.
	Session *Session

	// RecoveredInstances lists instance IDs that were successfully reconnected.
	RecoveredInstances []string

	// OrphanedInstances lists instance IDs that could not be recovered.
	OrphanedInstances []string

	// Warnings contains any non-fatal issues encountered during recovery.
	Warnings []string
}

// SessionManager defines the interface for session lifecycle management.
// Implementations handle session persistence, recovery, and state transitions.
type SessionManager interface {
	// Load retrieves a session from the given path.
	// Returns an error if the session doesn't exist or cannot be read.
	Load(path string) (*Session, error)

	// Save persists the session to storage.
	// The storage location is determined by the session's configuration.
	Save(session *Session) error

	// Recover attempts to restore a session and reconnect to running instances.
	// This is used after a crash or restart to resume work.
	// Returns detailed recovery information including which instances were recovered.
	Recover(path string) (*RecoveryInfo, error)

	// Create initializes a new session with the given name and configuration.
	// The session is persisted immediately after creation.
	Create(name string, config SessionConfig) (*Session, error)

	// Delete removes a session and optionally its associated resources.
	// Returns an error if the session is currently active or locked.
	Delete(path string) error

	// List returns information about all sessions in the base directory.
	List(baseDir string) ([]*SessionInfo, error)

	// Lock attempts to acquire an exclusive lock on the session.
	// Returns an error if the session is already locked by another process.
	Lock(session *Session) error

	// Unlock releases the lock on the session.
	Unlock(session *Session) error

	// IsLocked checks if the session is currently locked.
	IsLocked(session *Session) bool

	// Exists checks if a session exists at the given path.
	Exists(path string) bool
}

// SessionInfo provides summary information about a session for listing.
// This is a lighter-weight representation than full Session for discovery.
type SessionInfo struct {
	// ID is the unique identifier for the session.
	ID string `json:"id"`

	// Name is the human-readable name for the session.
	Name string `json:"name"`

	// State is the current state of the session.
	State SessionState `json:"state"`

	// Created is the timestamp when the session was created.
	Created time.Time `json:"created"`

	// InstanceCount is the number of instances in the session.
	InstanceCount int `json:"instance_count"`

	// IsLocked indicates if the session is currently locked.
	IsLocked bool `json:"is_locked"`

	// Path is the filesystem path to the session.
	Path string `json:"path"`
}

// SessionManagerOption is a functional option for configuring a SessionManager.
type SessionManagerOption func(*sessionManagerOptions)

// sessionManagerOptions holds configuration for SessionManager implementations.
type sessionManagerOptions struct {
	// LockTimeout is the duration to wait when acquiring locks.
	LockTimeout time.Duration

	// AutoSaveInterval is how often to auto-save session state (0 = disabled).
	AutoSaveInterval time.Duration

	// EnableBackups creates backup files before overwriting.
	EnableBackups bool
}

// WithLockTimeout sets the lock acquisition timeout.
func WithLockTimeout(d time.Duration) SessionManagerOption {
	return func(o *sessionManagerOptions) {
		o.LockTimeout = d
	}
}

// WithAutoSave enables automatic periodic session saves.
func WithAutoSave(interval time.Duration) SessionManagerOption {
	return func(o *sessionManagerOptions) {
		o.AutoSaveInterval = interval
	}
}

// WithBackups enables backup file creation.
func WithBackups(enabled bool) SessionManagerOption {
	return func(o *sessionManagerOptions) {
		o.EnableBackups = enabled
	}
}

// DefaultSessionManagerOptions returns the default configuration options.
func DefaultSessionManagerOptions() *sessionManagerOptions {
	return &sessionManagerOptions{
		LockTimeout:      10 * time.Second,
		AutoSaveInterval: 0, // disabled by default
		EnableBackups:    true,
	}
}

// Package session provides session lifecycle management for the orchestrator.
// It handles loading, saving, creating, and deleting sessions, as well as
// session lock acquisition and release.
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/session"
)

// Manager handles session lifecycle operations including persistence and locking.
// It operates on Session objects and provides a clean separation between
// session management and instance orchestration.
type Manager struct {
	baseDir    string
	claudioDir string
	sessionDir string // Session-specific directory for multi-session mode
	sessionID  string // Current session ID (empty for legacy single-session mode)
	logger     *logging.Logger
	lock       *session.Lock
}

// Config holds configuration options for creating a Manager.
type Config struct {
	BaseDir   string
	SessionID string          // Optional: for multi-session support
	Logger    *logging.Logger // Optional: for structured logging
}

// NewManager creates a new session Manager with the given configuration.
func NewManager(cfg Config) *Manager {
	claudioDir := filepath.Join(cfg.BaseDir, ".claudio")

	m := &Manager{
		baseDir:    cfg.BaseDir,
		claudioDir: claudioDir,
		logger:     cfg.Logger,
	}

	// Set up session-specific directory if using multi-session mode
	if cfg.SessionID != "" {
		m.sessionID = cfg.SessionID
		m.sessionDir = session.GetSessionDir(cfg.BaseDir, cfg.SessionID)
	}

	return m
}

// SessionID returns the current session ID (empty string for legacy single-session mode).
func (m *Manager) SessionID() string {
	return m.sessionID
}

// SessionDir returns the session-specific directory path.
// Returns empty string for legacy single-session mode.
func (m *Manager) SessionDir() string {
	return m.sessionDir
}

// SessionFilePath returns the path to the session.json file.
func (m *Manager) SessionFilePath() string {
	if m.sessionDir != "" {
		return filepath.Join(m.sessionDir, "session.json")
	}
	// Legacy single-session mode
	return filepath.Join(m.claudioDir, "session.json")
}

// Init initializes the directory structure required for session management.
// This creates .claudio directory and session-specific directories as needed.
func (m *Manager) Init() error {
	// Create .claudio directory
	if err := os.MkdirAll(m.claudioDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claudio directory: %w", err)
	}

	// Create session directory if using multi-session
	if m.sessionDir != "" {
		if err := os.MkdirAll(m.sessionDir, 0755); err != nil {
			return fmt.Errorf("failed to create session directory: %w", err)
		}
	}

	return nil
}

// AcquireLock attempts to acquire an exclusive lock on the session.
// This prevents concurrent access to the same session from multiple processes.
// Returns ErrSessionLocked if the session is already locked.
func (m *Manager) AcquireLock() error {
	if m.sessionDir == "" || m.sessionID == "" {
		// No locking in legacy single-session mode
		return nil
	}

	lock, err := session.AcquireLock(m.sessionDir, m.sessionID, m.logger)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to acquire session lock",
				"session_id", m.sessionID,
				"error", err,
			)
		}
		return fmt.Errorf("failed to acquire session lock: %w", err)
	}

	m.lock = lock
	return nil
}

// ReleaseLock releases the session lock if one is held.
// Safe to call multiple times or when no lock is held.
func (m *Manager) ReleaseLock() error {
	if m.lock == nil {
		return nil
	}

	err := m.lock.Release()
	m.lock = nil
	return err
}

// HasLock returns true if this manager currently holds a session lock.
func (m *Manager) HasLock() bool {
	return m.lock != nil
}

// Lock returns the current lock, or nil if no lock is held.
func (m *Manager) Lock() *session.Lock {
	return m.lock
}

// Exists checks if a session file exists at the expected location.
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.SessionFilePath())
	return err == nil
}

// HasLegacySession checks if there's a legacy single-session file
// that might need migration to multi-session format.
func (m *Manager) HasLegacySession() bool {
	legacyFile := filepath.Join(m.claudioDir, "session.json")
	_, err := os.Stat(legacyFile)
	return err == nil
}

// LoadSession reads and deserializes a session from the session file.
// The caller is responsible for acquiring a lock before loading if needed.
func (m *Manager) LoadSession() (*SessionData, error) {
	sessionFile := m.SessionFilePath()

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to read session file",
				"file_path", sessionFile,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sess SessionData
	if err := json.Unmarshal(data, &sess); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to parse session file",
				"file_path", sessionFile,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	// Update sessionID from loaded session if not already set
	if m.sessionID == "" && sess.ID != "" {
		m.sessionID = sess.ID
	}

	if m.logger != nil {
		m.logger.Info("session loaded",
			"session_id", sess.ID,
			"instance_count", len(sess.Instances),
		)
	}

	return &sess, nil
}

// LoadSessionWithLock acquires a lock and then loads the session.
// This is the recommended way to load a session in multi-session mode.
func (m *Manager) LoadSessionWithLock() (*SessionData, error) {
	if err := m.AcquireLock(); err != nil {
		return nil, err
	}
	return m.LoadSession()
}

// SaveSession persists the session state to disk.
func (m *Manager) SaveSession(sess *SessionData) error {
	if sess == nil {
		return nil
	}

	sessionFile := m.SessionFilePath()

	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		if m.logger != nil {
			m.logger.Error("failed to marshal session data", "error", err)
		}
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to write session file",
				"file_path", sessionFile,
				"error", err,
			)
		}
		return fmt.Errorf("failed to write session file: %w", err)
	}

	if m.logger != nil {
		m.logger.Debug("session saved", "file_path", sessionFile)
	}

	return nil
}

// CreateSession creates a new session with the given name and base repository.
// The session is persisted to disk immediately.
// In multi-session mode, the manager's sessionID is used as the session ID.
func (m *Manager) CreateSession(name, baseRepo string) (*SessionData, error) {
	// Ensure directories exist
	if err := m.Init(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to initialize session manager", "error", err)
		}
		return nil, err
	}

	// Acquire lock for multi-session mode
	if err := m.AcquireLock(); err != nil {
		return nil, err
	}

	sess := NewSessionData(name, baseRepo)

	// Use the manager's session ID if set (multi-session mode)
	if m.sessionID != "" {
		sess.ID = m.sessionID
	}

	// Save immediately
	if err := m.SaveSession(sess); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to save session",
				"session_id", sess.ID,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("session created",
			"session_id", sess.ID,
			"name", name,
			"base_repo", baseRepo,
		)
	}

	return sess, nil
}

// DeleteSession removes the session file and releases any held lock.
// This does not clean up worktrees or other resources - use the orchestrator
// for full cleanup.
func (m *Manager) DeleteSession() error {
	sessionFile := m.SessionFilePath()

	// Release lock first
	if err := m.ReleaseLock(); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to release lock during session deletion", "error", err)
		}
		// Continue with deletion anyway
	}

	// Remove session file
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		if m.logger != nil {
			m.logger.Error("failed to delete session file",
				"file_path", sessionFile,
				"error", err,
			)
		}
		return fmt.Errorf("failed to delete session file: %w", err)
	}

	if m.logger != nil {
		m.logger.Info("session deleted", "session_id", m.sessionID)
	}

	return nil
}

// SetLogger sets or updates the logger for the manager.
func (m *Manager) SetLogger(logger *logging.Logger) {
	m.logger = logger
}

// Logger returns the current logger, or nil if logging is disabled.
func (m *Manager) Logger() *logging.Logger {
	return m.logger
}

// ContextFilePath returns the path to the context.md file for this session.
func (m *Manager) ContextFilePath() string {
	if m.sessionDir != "" {
		return filepath.Join(m.sessionDir, "context.md")
	}
	return filepath.Join(m.claudioDir, "context.md")
}

// WriteContext writes the shared context markdown to the session's context file.
func (m *Manager) WriteContext(content string) error {
	ctxFile := m.ContextFilePath()

	if err := os.WriteFile(ctxFile, []byte(content), 0644); err != nil {
		if m.logger != nil {
			m.logger.Error("failed to write context file",
				"file_path", ctxFile,
				"error", err,
			)
		}
		return fmt.Errorf("failed to write context file: %w", err)
	}

	if m.logger != nil {
		m.logger.Debug("context updated", "file_path", ctxFile)
	}

	return nil
}

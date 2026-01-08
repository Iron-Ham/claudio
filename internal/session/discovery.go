package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SessionsDir is the directory name within .claudio that contains all sessions
const SessionsDir = "sessions"

// SessionFileName is the name of the session data file within a session directory
const SessionFileName = "session.json"

// Info contains summary information about a session
type Info struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Created       time.Time `json:"created"`
	InstanceCount int       `json:"instance_count"`
	IsLocked      bool      `json:"is_locked"`
	LockInfo      *Lock     `json:"lock_info,omitempty"`
	SessionDir    string    `json:"session_dir"`
}

// SessionData represents the minimal session structure needed for discovery.
// This mirrors the Session struct from orchestrator but only includes
// the fields we need for listing.
type SessionData struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Created   time.Time       `json:"created"`
	Instances json.RawMessage `json:"instances"` // Keep raw to count without full parsing
}

// GetSessionsDir returns the path to the sessions directory for a given base directory
func GetSessionsDir(baseDir string) string {
	return filepath.Join(baseDir, ".claudio", SessionsDir)
}

// GetSessionDir returns the path to a specific session's directory
func GetSessionDir(baseDir, sessionID string) string {
	return filepath.Join(GetSessionsDir(baseDir), sessionID)
}

// ListSessions returns information about all sessions in the base directory.
// Sessions are discovered by scanning .claudio/sessions/ for subdirectories
// containing session.json files.
func ListSessions(baseDir string) ([]*Info, error) {
	sessionsDir := GetSessionsDir(baseDir)

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No sessions directory = no sessions
		}
		return nil, err
	}

	var sessions []*Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		info, err := GetSessionInfo(baseDir, sessionID)
		if err != nil {
			// Skip sessions we can't read
			continue
		}

		sessions = append(sessions, info)
	}

	return sessions, nil
}

// GetSessionInfo returns detailed information about a specific session.
func GetSessionInfo(baseDir, sessionID string) (*Info, error) {
	sessionDir := GetSessionDir(baseDir, sessionID)
	sessionFile := filepath.Join(sessionDir, SessionFileName)

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, err
	}

	var sessionData SessionData
	if err := json.Unmarshal(data, &sessionData); err != nil {
		return nil, err
	}

	// Count instances from raw JSON
	instanceCount := 0
	if sessionData.Instances != nil {
		var instances []json.RawMessage
		if err := json.Unmarshal(sessionData.Instances, &instances); err == nil {
			instanceCount = len(instances)
		}
	}

	// Check lock status
	lockInfo, isLocked := IsLocked(sessionDir)

	return &Info{
		ID:            sessionData.ID,
		Name:          sessionData.Name,
		Created:       sessionData.Created,
		InstanceCount: instanceCount,
		IsLocked:      isLocked,
		LockInfo:      lockInfo,
		SessionDir:    sessionDir,
	}, nil
}

// SessionExists checks if a session with the given ID exists.
func SessionExists(baseDir, sessionID string) bool {
	sessionDir := GetSessionDir(baseDir, sessionID)
	sessionFile := filepath.Join(sessionDir, SessionFileName)
	_, err := os.Stat(sessionFile)
	return err == nil
}

// FindUnlockedSessions returns all sessions that are not currently locked.
func FindUnlockedSessions(baseDir string) ([]*Info, error) {
	sessions, err := ListSessions(baseDir)
	if err != nil {
		return nil, err
	}

	var unlocked []*Info
	for _, s := range sessions {
		if !s.IsLocked {
			unlocked = append(unlocked, s)
		}
	}

	return unlocked, nil
}

// CleanupStaleLocks iterates through all sessions and removes stale lock files.
// Returns the IDs of sessions that had stale locks cleaned.
func CleanupStaleLocks(baseDir string) ([]string, error) {
	sessionsDir := GetSessionsDir(baseDir)

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cleaned []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		sessionDir := GetSessionDir(baseDir, sessionID)

		wasCleaned, err := CleanStaleLock(sessionDir)
		if err != nil {
			continue // Skip errors, try other sessions
		}

		if wasCleaned {
			cleaned = append(cleaned, sessionID)
		}
	}

	return cleaned, nil
}

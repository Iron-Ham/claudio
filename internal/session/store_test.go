package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Test Helpers
// =============================================================================

// testSession is a simple Persistable implementation for testing
type testSession struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Created   time.Time `json:"created"`
	Data      string    `json:"data,omitempty"`
}

func (s *testSession) GetID() string        { return s.ID }
func (s *testSession) GetName() string      { return s.Name }
func (s *testSession) GetCreated() time.Time { return s.Created }

func newTestSession(id, name string) *testSession {
	return &testSession{
		ID:      id,
		Name:    name,
		Created: time.Now(),
	}
}

// setupTestDir creates a temporary directory for testing and returns a cleanup function
func setupTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

// =============================================================================
// FileStore Tests
// =============================================================================

func TestFileStore_NewFileStore(t *testing.T) {
	dir := setupTestDir(t)

	store, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("NewFileStore returned nil store")
	}
}

func TestFileStore_NewFileStore_CreatesMissingDir(t *testing.T) {
	dir := setupTestDir(t)
	subDir := filepath.Join(dir, "nested", "path")

	store, err := NewFileStore(subDir)
	if err != nil {
		t.Fatalf("NewFileStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("NewFileStore returned nil store")
	}

	// Verify directory was created
	info, err := os.Stat(subDir)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Path is not a directory")
	}
}

func TestFileStore_Save_Load(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "test-key"
	data := []byte("test data content")

	// Save
	err := store.Save(ctx, key, data)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := store.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if string(loaded) != string(data) {
		t.Errorf("Loaded data = %q, want %q", loaded, data)
	}
}

func TestFileStore_Save_NestedKey(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "sessions/session1/data.json"
	data := []byte(`{"key": "value"}`)

	err := store.Save(ctx, key, data)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if string(loaded) != string(data) {
		t.Errorf("Loaded data = %q, want %q", loaded, data)
	}
}

func TestFileStore_Load_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent-key")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_Delete(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "to-delete"
	data := []byte("will be deleted")

	// Save first
	_ = store.Save(ctx, key, data)

	// Delete
	err := store.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	_, err = store.Load(ctx, key)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound after delete, got %v", err)
	}
}

func TestFileStore_Delete_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	err := store.Delete(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileStore_List(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	// Save multiple keys
	_ = store.Save(ctx, "prefix1/file1", []byte("1"))
	_ = store.Save(ctx, "prefix1/file2", []byte("2"))
	_ = store.Save(ctx, "prefix2/file1", []byte("3"))

	// List all
	keys, err := store.List(ctx, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// List with prefix
	keys, err = store.List(ctx, "prefix1")
	if err != nil {
		t.Fatalf("List with prefix failed: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys with prefix1, got %d", len(keys))
	}
}

func TestFileStore_Exists(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "exists-test"

	// Should not exist initially
	exists, err := store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Key should not exist initially")
	}

	// Save
	_ = store.Save(ctx, key, []byte("data"))

	// Should exist now
	exists, err = store.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Key should exist after save")
	}
}

func TestFileStore_SaveIfNotExists(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "unique-key"
	data1 := []byte("first")
	data2 := []byte("second")

	// First save should succeed
	err := store.SaveIfNotExists(ctx, key, data1)
	if err != nil {
		t.Fatalf("First SaveIfNotExists failed: %v", err)
	}

	// Second save should fail
	err = store.SaveIfNotExists(ctx, key, data2)
	if err != ErrAlreadyExists {
		t.Errorf("Expected ErrAlreadyExists, got %v", err)
	}

	// Verify original data preserved
	loaded, _ := store.Load(ctx, key)
	if string(loaded) != string(data1) {
		t.Errorf("Data was modified, got %q, want %q", loaded, data1)
	}
}

func TestFileStore_SaveWithVersion_NewKey(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "versioned-key"
	data := []byte("versioned data")

	// Save new key with version 0
	version, err := store.SaveWithVersion(ctx, key, data, 0)
	if err != nil {
		t.Fatalf("SaveWithVersion failed: %v", err)
	}
	if version == 0 {
		t.Error("Expected non-zero version")
	}
}

func TestFileStore_SaveWithVersion_StaleData(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "versioned-key"

	// Save initial version
	version1, _ := store.SaveWithVersion(ctx, key, []byte("v1"), 0)

	// Modify with correct version
	version2, err := store.SaveWithVersion(ctx, key, []byte("v2"), version1)
	if err != nil {
		t.Fatalf("SaveWithVersion with correct version failed: %v", err)
	}

	// Try to modify with old version
	_, err = store.SaveWithVersion(ctx, key, []byte("v3"), version1)
	if err != ErrStaleData {
		t.Errorf("Expected ErrStaleData, got %v", err)
	}

	// Verify v2 was preserved
	loaded, currentVersion, _ := store.LoadWithVersion(ctx, key)
	if string(loaded) != "v2" {
		t.Errorf("Data = %q, want v2", loaded)
	}
	if currentVersion != version2 {
		t.Errorf("Version mismatch")
	}
}

func TestFileStore_LoadWithVersion(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	key := "load-version-test"
	data := []byte("test data")

	_ = store.Save(ctx, key, data)

	loaded, version, err := store.LoadWithVersion(ctx, key)
	if err != nil {
		t.Fatalf("LoadWithVersion failed: %v", err)
	}
	if string(loaded) != string(data) {
		t.Errorf("Loaded data = %q, want %q", loaded, data)
	}
	if version == 0 {
		t.Error("Expected non-zero version")
	}
}

func TestFileStore_Concurrent(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileStore(dir)
	ctx := context.Background()

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "concurrent-key"
				data := []byte("data")

				_ = store.Save(ctx, key, data)
				_, _ = store.Load(ctx, key)
				_, _ = store.Exists(ctx, key)
			}
		}(i)
	}

	wg.Wait()
}

// =============================================================================
// FileSessionStore Tests
// =============================================================================

func TestFileSessionStore_NewFileSessionStore(t *testing.T) {
	dir := setupTestDir(t)

	store, err := NewFileSessionStore(dir)
	if err != nil {
		t.Fatalf("NewFileSessionStore failed: %v", err)
	}
	if store == nil {
		t.Fatal("NewFileSessionStore returned nil")
	}
}

func TestFileSessionStore_SaveSession(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	session := newTestSession("test-session-1", "Test Session")

	err := store.SaveSession(ctx, session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Verify file exists
	sessionFile := filepath.Join(store.GetSessionDir(session.ID), SessionFileName)
	if _, err := os.Stat(sessionFile); err != nil {
		t.Errorf("Session file not created: %v", err)
	}
}

func TestFileSessionStore_SaveSession_EmptyID(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	session := newTestSession("", "No ID")

	err := store.SaveSession(ctx, session)
	if err == nil {
		t.Error("Expected error for empty session ID")
	}
}

func TestFileSessionStore_LoadSession(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	session := newTestSession("load-test", "Load Test")
	_ = store.SaveSession(ctx, session)

	data, err := store.LoadSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("LoadSession failed: %v", err)
	}

	// Verify data can be unmarshaled
	var loaded testSession
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Failed to unmarshal loaded data: %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("Loaded ID = %q, want %q", loaded.ID, session.ID)
	}
	if loaded.Name != session.Name {
		t.Errorf("Loaded Name = %q, want %q", loaded.Name, session.Name)
	}
}

func TestFileSessionStore_LoadSession_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	_, err := store.LoadSession(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileSessionStore_LoadSession_Corrupted(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	sessionID := "corrupted-session"
	sessionDir := store.GetSessionDir(sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Write invalid JSON
	sessionFile := filepath.Join(sessionDir, SessionFileName)
	_ = os.WriteFile(sessionFile, []byte("not valid json{"), 0644)

	_, err := store.LoadSession(ctx, sessionID)
	if err == nil {
		t.Error("Expected error for corrupted session")
	}
}

func TestFileSessionStore_ListSessions(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	// Create multiple sessions
	sessions := []*testSession{
		newTestSession("session-1", "First"),
		newTestSession("session-2", "Second"),
		newTestSession("session-3", "Third"),
	}

	for _, s := range sessions {
		_ = store.SaveSession(ctx, s)
	}

	// List sessions
	infos, err := store.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}

	if len(infos) != len(sessions) {
		t.Errorf("Expected %d sessions, got %d", len(sessions), len(infos))
	}
}

func TestFileSessionStore_DeleteSession(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	session := newTestSession("to-delete", "Will Delete")
	_ = store.SaveSession(ctx, session)

	// Delete
	err := store.DeleteSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Verify deleted
	if store.SessionExists(ctx, session.ID) {
		t.Error("Session should not exist after delete")
	}
}

func TestFileSessionStore_DeleteSession_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	err := store.DeleteSession(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileSessionStore_SessionExists(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)
	ctx := context.Background()

	sessionID := "exists-test"

	// Should not exist initially
	if store.SessionExists(ctx, sessionID) {
		t.Error("Session should not exist initially")
	}

	// Create session
	_ = store.SaveSession(ctx, newTestSession(sessionID, "Test"))

	// Should exist now
	if !store.SessionExists(ctx, sessionID) {
		t.Error("Session should exist after save")
	}
}

func TestFileSessionStore_GetSessionDir(t *testing.T) {
	dir := setupTestDir(t)
	store, _ := NewFileSessionStore(dir)

	sessionDir := store.GetSessionDir("test-id")
	expected := GetSessionDir(dir, "test-id")

	if sessionDir != expected {
		t.Errorf("GetSessionDir() = %q, want %q", sessionDir, expected)
	}
}

// =============================================================================
// FileLockManager Tests
// =============================================================================

func TestFileLockManager_NewFileLockManager(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	if manager == nil {
		t.Fatal("NewFileLockManager returned nil")
	}
}

func TestFileLockManager_Acquire_Release(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "lock-test"

	// Create session directory
	sessionDir := GetSessionDir(dir, sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Acquire lock
	handle, err := manager.Acquire(ctx, sessionID)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}
	if handle == nil {
		t.Fatal("Acquire returned nil handle")
	}

	// Verify lock is held
	lockInfo, isLocked := manager.IsLocked(ctx, sessionID)
	if !isLocked {
		t.Error("Session should be locked")
	}
	if lockInfo == nil {
		t.Error("LockInfo should not be nil")
	}
	if lockInfo.PID != os.Getpid() {
		t.Errorf("Lock PID = %d, want %d", lockInfo.PID, os.Getpid())
	}

	// Release
	err = handle.Release()
	if err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Verify lock is released
	_, isLocked = manager.IsLocked(ctx, sessionID)
	if isLocked {
		t.Error("Session should not be locked after release")
	}
}

func TestFileLockManager_TryAcquire_AlreadyLocked(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "double-lock"

	// Acquire first lock
	handle1, _ := manager.Acquire(ctx, sessionID)
	defer handle1.Release()

	// Try to acquire second lock
	_, err := manager.TryAcquire(ctx, sessionID)
	if err == nil {
		t.Error("Expected error when acquiring already-locked session")
	}
}

func TestFileLockManager_GetHolder(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "holder-test"

	// Should fail when not locked
	_, err := manager.GetHolder(ctx, sessionID)
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound when not locked, got %v", err)
	}

	// Acquire lock
	handle, _ := manager.Acquire(ctx, sessionID)
	defer handle.Release()

	// Should return holder info
	holder, err := manager.GetHolder(ctx, sessionID)
	if err != nil {
		t.Fatalf("GetHolder failed: %v", err)
	}
	if holder.PID != os.Getpid() {
		t.Errorf("Holder PID = %d, want %d", holder.PID, os.Getpid())
	}
}

func TestFileLockManager_ForceRelease(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "force-release"

	// Acquire lock
	handle, _ := manager.Acquire(ctx, sessionID)

	// Force release (simulating recovery)
	err := manager.ForceRelease(ctx, sessionID)
	if err != nil {
		t.Fatalf("ForceRelease failed: %v", err)
	}

	// Verify lock is released
	_, isLocked := manager.IsLocked(ctx, sessionID)
	if isLocked {
		t.Error("Session should not be locked after force release")
	}

	// Original handle's Release should not fail
	_ = handle.Release()
}

func TestFileLockManager_ForceRelease_NotLocked(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	err := manager.ForceRelease(ctx, "not-locked")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileLockHandle_SessionID(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "handle-session-test"
	handle, _ := manager.Acquire(ctx, sessionID)
	defer handle.Release()

	if handle.SessionID() != sessionID {
		t.Errorf("SessionID() = %q, want %q", handle.SessionID(), sessionID)
	}
}

func TestFileLockHandle_Info(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "handle-info-test"
	handle, _ := manager.Acquire(ctx, sessionID)
	defer handle.Release()

	info := handle.Info()
	if info == nil {
		t.Fatal("Info() returned nil")
	}
	if info.SessionID != sessionID {
		t.Errorf("Info().SessionID = %q, want %q", info.SessionID, sessionID)
	}
}

func TestFileLockHandle_Refresh(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	sessionID := "refresh-test"
	handle, _ := manager.Acquire(ctx, sessionID)
	defer handle.Release()

	// Get initial lock file stat
	lockPath := filepath.Join(GetSessionDir(dir, sessionID), LockFileName)
	initialStat, _ := os.Stat(lockPath)

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Refresh
	err := handle.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify modification time changed
	newStat, _ := os.Stat(lockPath)
	if !newStat.ModTime().After(initialStat.ModTime()) {
		t.Error("Lock file modification time should have increased")
	}
}

func TestFileLockHandle_Release_Idempotent(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileLockManager(dir)
	ctx := context.Background()

	handle, _ := manager.Acquire(ctx, "idempotent-release")

	// Multiple releases should not fail
	for i := 0; i < 3; i++ {
		err := handle.Release()
		if err != nil {
			t.Errorf("Release #%d failed: %v", i, err)
		}
	}
}

// =============================================================================
// FileRecoveryManager Tests
// =============================================================================

func TestFileRecoveryManager_NewFileRecoveryManager(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	if manager == nil {
		t.Fatal("NewFileRecoveryManager returned nil")
	}
}

func TestFileRecoveryManager_CheckForRecovery_NoSessions(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	candidates, err := manager.CheckForRecovery(ctx)
	if err != nil {
		t.Fatalf("CheckForRecovery failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates, got %d", len(candidates))
	}
}

func TestFileRecoveryManager_CheckForRecovery_StaleLock(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	// Create a session with a stale lock
	sessionID := "stale-lock-session"
	sessionDir := GetSessionDir(dir, sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Write session file
	session := newTestSession(sessionID, "Stale Session")
	data, _ := json.Marshal(session)
	_ = os.WriteFile(filepath.Join(sessionDir, SessionFileName), data, 0644)

	// Write a stale lock (with non-existent PID)
	staleLock := &Lock{
		SessionID: sessionID,
		PID:       99999999, // Very unlikely to be a real PID
		Hostname:  "test-host",
		StartedAt: time.Now().Add(-time.Hour),
	}
	lockData, _ := json.Marshal(staleLock)
	_ = os.WriteFile(filepath.Join(sessionDir, LockFileName), lockData, 0644)

	candidates, err := manager.CheckForRecovery(ctx)
	if err != nil {
		t.Fatalf("CheckForRecovery failed: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("Expected 1 candidate, got %d", len(candidates))
	}

	if !candidates[0].HasStaleLock {
		t.Error("Expected HasStaleLock to be true")
	}
	if candidates[0].SessionID != sessionID {
		t.Errorf("SessionID = %q, want %q", candidates[0].SessionID, sessionID)
	}
}

func TestFileRecoveryManager_RecoverSession(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	// Create a session with a stale lock
	sessionID := "recover-test"
	sessionDir := GetSessionDir(dir, sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Write session file
	session := newTestSession(sessionID, "Recover Session")
	data, _ := json.Marshal(session)
	_ = os.WriteFile(filepath.Join(sessionDir, SessionFileName), data, 0644)

	// Write a stale lock
	staleLock := &Lock{
		SessionID: sessionID,
		PID:       99999999,
		Hostname:  "test-host",
		StartedAt: time.Now().Add(-time.Hour),
	}
	lockData, _ := json.Marshal(staleLock)
	lockPath := filepath.Join(sessionDir, LockFileName)
	_ = os.WriteFile(lockPath, lockData, 0644)

	// Recover
	result, err := manager.RecoverSession(ctx, sessionID)
	if err != nil {
		t.Fatalf("RecoverSession failed: %v", err)
	}

	if !result.Recovered {
		t.Error("Expected Recovered to be true")
	}
	if !result.CleanedUp {
		t.Error("Expected CleanedUp to be true")
	}

	// Verify lock file was removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file should have been removed")
	}
}

func TestFileRecoveryManager_RecoverSession_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	_, err := manager.RecoverSession(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileRecoveryManager_CleanupStale(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	// Create sessions with stale locks
	for i := 0; i < 3; i++ {
		sessionID := "stale-" + string(rune('a'+i))
		sessionDir := GetSessionDir(dir, sessionID)
		_ = os.MkdirAll(sessionDir, 0755)

		session := newTestSession(sessionID, "Stale "+sessionID)
		data, _ := json.Marshal(session)
		_ = os.WriteFile(filepath.Join(sessionDir, SessionFileName), data, 0644)

		staleLock := &Lock{
			SessionID: sessionID,
			PID:       99999999 + i,
			Hostname:  "test-host",
			StartedAt: time.Now().Add(-time.Hour),
		}
		lockData, _ := json.Marshal(staleLock)
		_ = os.WriteFile(filepath.Join(sessionDir, LockFileName), lockData, 0644)
	}

	cleaned, err := manager.CleanupStale(ctx)
	if err != nil {
		t.Fatalf("CleanupStale failed: %v", err)
	}

	if cleaned != 3 {
		t.Errorf("Expected 3 cleaned, got %d", cleaned)
	}
}

func TestFileRecoveryManager_ValidateSession(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	sessionID := "validate-test"
	sessionDir := GetSessionDir(dir, sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Valid session
	session := newTestSession(sessionID, "Valid Session")
	data, _ := json.Marshal(session)
	_ = os.WriteFile(filepath.Join(sessionDir, SessionFileName), data, 0644)

	err := manager.ValidateSession(ctx, sessionID)
	if err != nil {
		t.Errorf("ValidateSession failed for valid session: %v", err)
	}
}

func TestFileRecoveryManager_ValidateSession_NotFound(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	err := manager.ValidateSession(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("Expected ErrNotFound, got %v", err)
	}
}

func TestFileRecoveryManager_ValidateSession_Corrupted(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	sessionID := "corrupted"
	sessionDir := GetSessionDir(dir, sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Write invalid JSON
	_ = os.WriteFile(filepath.Join(sessionDir, SessionFileName), []byte("not json"), 0644)

	err := manager.ValidateSession(ctx, sessionID)
	if err == nil {
		t.Error("Expected error for corrupted session")
	}
}

func TestFileRecoveryManager_ValidateSession_IDMismatch(t *testing.T) {
	dir := setupTestDir(t)
	manager := NewFileRecoveryManager(dir)
	ctx := context.Background()

	sessionID := "mismatch-test"
	sessionDir := GetSessionDir(dir, sessionID)
	_ = os.MkdirAll(sessionDir, 0755)

	// Write session with different ID
	session := newTestSession("different-id", "Mismatched Session")
	data, _ := json.Marshal(session)
	_ = os.WriteFile(filepath.Join(sessionDir, SessionFileName), data, 0644)

	err := manager.ValidateSession(ctx, sessionID)
	if err == nil {
		t.Error("Expected error for ID mismatch")
	}
}

// =============================================================================
// FilePersistenceLayer Tests
// =============================================================================

func TestFilePersistenceLayer_NewFilePersistenceLayer(t *testing.T) {
	dir := setupTestDir(t)

	layer, err := NewFilePersistenceLayer(dir)
	if err != nil {
		t.Fatalf("NewFilePersistenceLayer failed: %v", err)
	}
	if layer == nil {
		t.Fatal("NewFilePersistenceLayer returned nil")
	}
}

func TestFilePersistenceLayer_Close(t *testing.T) {
	dir := setupTestDir(t)
	layer, _ := NewFilePersistenceLayer(dir)

	err := layer.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestFilePersistenceLayer_Integration(t *testing.T) {
	dir := setupTestDir(t)
	layer, _ := NewFilePersistenceLayer(dir)
	defer layer.Close()
	ctx := context.Background()

	sessionID := "integration-test"
	session := newTestSession(sessionID, "Integration")

	// Save session
	err := layer.SaveSession(ctx, session)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	// Acquire lock
	handle, err := layer.Acquire(ctx, sessionID)
	if err != nil {
		t.Fatalf("Acquire failed: %v", err)
	}

	// Check for recovery (should be none since we hold the lock)
	candidates, err := layer.CheckForRecovery(ctx)
	if err != nil {
		t.Fatalf("CheckForRecovery failed: %v", err)
	}
	for _, c := range candidates {
		if c.SessionID == sessionID {
			t.Error("Current session should not be a recovery candidate")
		}
	}

	// Release lock
	_ = handle.Release()

	// List sessions
	infos, err := layer.ListSessions(ctx)
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(infos) != 1 {
		t.Errorf("Expected 1 session, got %d", len(infos))
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestAtomicWriteFile(t *testing.T) {
	dir := setupTestDir(t)
	path := filepath.Join(dir, "atomic-test.txt")
	data := []byte("atomic write test data")

	err := atomicWriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	// Verify content
	read, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(read) != string(data) {
		t.Errorf("Content = %q, want %q", read, data)
	}

	// Verify permissions
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0644 {
		t.Errorf("Permissions = %o, want 644", info.Mode().Perm())
	}
}

func TestGenerateHolderID(t *testing.T) {
	id1 := generateHolderID()
	id2 := generateHolderID()

	if id1 == "" {
		t.Error("Generated ID should not be empty")
	}
	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

// =============================================================================
// Lock Bridge Function Tests
// =============================================================================

func TestLock_ToLockInfo(t *testing.T) {
	lock := &Lock{
		SessionID: "test-session",
		PID:       12345,
		Hostname:  "test-host",
		StartedAt: time.Now(),
	}

	info := lock.ToLockInfo()
	if info == nil {
		t.Fatal("ToLockInfo returned nil")
	}
	if info.SessionID != lock.SessionID {
		t.Errorf("SessionID = %q, want %q", info.SessionID, lock.SessionID)
	}
	if info.PID != lock.PID {
		t.Errorf("PID = %d, want %d", info.PID, lock.PID)
	}
	if info.Hostname != lock.Hostname {
		t.Errorf("Hostname = %q, want %q", info.Hostname, lock.Hostname)
	}
}

func TestLock_ToLockInfo_Nil(t *testing.T) {
	var lock *Lock
	info := lock.ToLockInfo()
	if info != nil {
		t.Error("ToLockInfo on nil should return nil")
	}
}

func TestLockFromLockInfo(t *testing.T) {
	info := &LockInfo{
		SessionID:  "test-session",
		HolderID:   "holder-123",
		PID:        12345,
		Hostname:   "test-host",
		AcquiredAt: time.Now(),
	}

	lock := LockFromLockInfo(info, "/path/to/lock")
	if lock == nil {
		t.Fatal("LockFromLockInfo returned nil")
	}
	if lock.SessionID != info.SessionID {
		t.Errorf("SessionID = %q, want %q", lock.SessionID, info.SessionID)
	}
	if lock.PID != info.PID {
		t.Errorf("PID = %d, want %d", lock.PID, info.PID)
	}
}

func TestLockFromLockInfo_Nil(t *testing.T) {
	lock := LockFromLockInfo(nil, "/path")
	if lock != nil {
		t.Error("LockFromLockInfo on nil should return nil")
	}
}

// =============================================================================
// Discovery Helper Tests
// =============================================================================

func TestNewSessionStoreFromBaseDir(t *testing.T) {
	dir := setupTestDir(t)

	store, err := NewSessionStoreFromBaseDir(dir)
	if err != nil {
		t.Fatalf("NewSessionStoreFromBaseDir failed: %v", err)
	}
	if store == nil {
		t.Fatal("NewSessionStoreFromBaseDir returned nil")
	}
}

func TestNewLockManagerFromBaseDir(t *testing.T) {
	dir := setupTestDir(t)

	manager := NewLockManagerFromBaseDir(dir)
	if manager == nil {
		t.Fatal("NewLockManagerFromBaseDir returned nil")
	}
}

func TestNewRecoveryManagerFromBaseDir(t *testing.T) {
	dir := setupTestDir(t)

	manager := NewRecoveryManagerFromBaseDir(dir)
	if manager == nil {
		t.Fatal("NewRecoveryManagerFromBaseDir returned nil")
	}
}

func TestNewPersistenceLayerFromBaseDir(t *testing.T) {
	dir := setupTestDir(t)

	layer, err := NewPersistenceLayerFromBaseDir(dir)
	if err != nil {
		t.Fatalf("NewPersistenceLayerFromBaseDir failed: %v", err)
	}
	if layer == nil {
		t.Fatal("NewPersistenceLayerFromBaseDir returned nil")
	}
}

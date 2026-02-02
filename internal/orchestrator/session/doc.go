// Package session provides session lifecycle management for the orchestrator.
//
// This package handles the persistence and locking of Claudio sessions,
// separating session management concerns from instance orchestration. It
// supports both single-session (legacy) and multi-session modes.
//
// # Main Types
//
// Management:
//   - [Manager]: Handles session lifecycle (load, save, create, delete, lock)
//   - [Config]: Configuration for creating a Manager (BaseDir, SessionID, Logger)
//
// Data Types:
//   - [SessionData]: Serializable session state for persistence
//   - [InstanceData]: Instance information for persistence
//   - [MetricsData]: Instance resource usage metrics
//
// # Session Modes
//
// The package supports two operating modes:
//
// Single-Session (Legacy):
//   - Session stored in .claudio/session.json
//   - No locking mechanism
//   - One session per repository
//
// Multi-Session:
//   - Sessions stored in .claudio/sessions/{sessionID}/session.json
//   - File-based locking prevents concurrent access
//   - Multiple sessions can exist in the same repository
//
// # Thread Safety
//
// [Manager] is safe for concurrent use within a single process. The locking
// mechanism prevents concurrent access from multiple processes to the same
// session.
//
// # Basic Usage
//
//	mgr := session.NewManager(session.Config{
//	    BaseDir:   "/path/to/repo",
//	    SessionID: "my-session",
//	})
//
//	// Initialize directories
//	if err := mgr.Init(); err != nil {
//	    return err
//	}
//
//	// Create a new session
//	sess, err := mgr.CreateSession("Feature Work", "/path/to/repo")
//	if err != nil {
//	    return err
//	}
//
//	// Add instance data
//	inst := session.NewInstanceData("Implement auth feature")
//	sess.Instances = append(sess.Instances, inst)
//
//	// Save session
//	if err := mgr.SaveSession(sess); err != nil {
//	    return err
//	}
//
//	// Later: load session
//	loaded, err := mgr.LoadSession()
//
// # Locking
//
// In multi-session mode, use [Manager.AcquireLock] before modifying session
// state to prevent concurrent access:
//
//	if err := mgr.AcquireLock(); err != nil {
//	    return fmt.Errorf("session is locked by another process: %w", err)
//	}
//	defer mgr.ReleaseLock()
//
// # Context Files
//
// The manager also handles context files that help backend instances
// coordinate their work:
//
//	mgr.WriteContext("# Session Context\n\n## Active Tasks\n...")
package session

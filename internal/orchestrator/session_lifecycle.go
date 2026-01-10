package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/session"
)

// StartSession creates and starts a new session
func (o *Orchestrator) StartSession(name string) (*Session, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Ensure initialized
	if err := o.Init(); err != nil {
		return nil, err
	}

	// Acquire session lock if using multi-session
	if o.sessionDir != "" && o.sessionID != "" {
		lock, err := session.AcquireLock(o.sessionDir, o.sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire session lock: %w", err)
		}
		o.lock = lock
	}

	// Create new session
	sess := NewSession(name, o.baseDir)
	// Use the orchestrator's session ID if set (multi-session mode)
	if o.sessionID != "" {
		sess.ID = o.sessionID
	}
	o.session = sess

	// Start conflict detector
	o.conflictDetector.Start()

	// Save session state
	if err := o.saveSession(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return o.session, nil
}

// LoadSession loads an existing session from disk
func (o *Orchestrator) LoadSession() (*Session, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Determine session file path based on mode
	var sessionFile string
	if o.sessionDir != "" {
		sessionFile = filepath.Join(o.sessionDir, "session.json")
	} else {
		// Legacy single-session mode
		sessionFile = filepath.Join(o.claudioDir, "session.json")
	}

	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	o.session = &sess

	// Set sessionID from loaded session if not already set
	if o.sessionID == "" && sess.ID != "" {
		o.sessionID = sess.ID
	}

	// Start conflict detector and register existing instances
	o.conflictDetector.Start()
	for _, inst := range sess.Instances {
		_ = o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath)
	}

	return o.session, nil
}

// LoadSessionWithLock loads an existing session and acquires a lock on it.
// Use this for multi-session mode to prevent concurrent access.
func (o *Orchestrator) LoadSessionWithLock() (*Session, error) {
	// Acquire lock first if using multi-session
	if o.sessionDir != "" && o.sessionID != "" {
		lock, err := session.AcquireLock(o.sessionDir, o.sessionID)
		if err != nil {
			return nil, fmt.Errorf("failed to acquire session lock: %w", err)
		}
		o.lock = lock
	}

	return o.LoadSession()
}

// RecoverSession loads a session and attempts to reconnect to running tmux sessions
// Returns a list of instance IDs that were successfully reconnected
func (o *Orchestrator) RecoverSession() (*Session, []string, error) {
	session, err := o.LoadSession()
	if err != nil {
		return nil, nil, err
	}

	var reconnected []string
	for _, inst := range session.Instances {
		// Create instance manager
		mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, inst.Task)

		// Try to reconnect if the tmux session still exists
		if mgr.TmuxSessionExists() {
			// Configure state change callback
			mgr.SetStateCallback(func(id string, state instance.WaitingState) {
				switch state {
				case instance.StateCompleted:
					o.handleInstanceExit(id)
				case instance.StateWaitingInput, instance.StateWaitingQuestion, instance.StateWaitingPermission:
					o.handleInstanceWaitingInput(id)
				case instance.StatePROpened:
					o.handleInstancePROpened(id)
				}
			})

			// Configure timeout callback
			mgr.SetTimeoutCallback(func(id string, timeoutType instance.TimeoutType) {
				o.handleInstanceTimeout(id, timeoutType)
			})

			// Configure bell callback to forward terminal bells
			mgr.SetBellCallback(func(id string) {
				o.handleInstanceBell(id)
			})

			if err := mgr.Reconnect(); err == nil {
				inst.Status = StatusWorking
				inst.PID = mgr.PID()
				reconnected = append(reconnected, inst.ID)
			}
		} else {
			// Tmux session doesn't exist - mark as paused if it was working
			if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
				inst.Status = StatusPaused
				inst.PID = 0
			}
		}

		o.mu.Lock()
		o.instances[inst.ID] = mgr
		o.mu.Unlock()
	}

	// Save updated session state
	_ = o.saveSession()

	return session, reconnected, nil
}

// saveSession persists the session state to disk
func (o *Orchestrator) saveSession() error {
	if o.session == nil {
		return nil
	}

	// Determine session file path based on mode
	var sessionFile string
	if o.sessionDir != "" {
		sessionFile = filepath.Join(o.sessionDir, "session.json")
	} else {
		// Legacy single-session mode
		sessionFile = filepath.Join(o.claudioDir, "session.json")
	}

	data, err := json.MarshalIndent(o.session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionFile, data, 0644)
}

// SaveSession is a public wrapper for saveSession, used by components
// like the Coordinator that need to trigger session persistence
func (o *Orchestrator) SaveSession() error {
	return o.saveSession()
}

package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/worktree"
	"github.com/spf13/viper"
)

// Orchestrator manages the Claudio session and coordinates instances
type Orchestrator struct {
	baseDir     string
	claudioDir  string
	worktreeDir string
	sessionID   string // Current session ID (for multi-session support)
	sessionDir  string // Session-specific directory (.claudio/sessions/{sessionID})
	lock        *session.Lock
	logger      *logging.Logger // Structured logger for debugging (nil = no logging)

	session          *Session
	instances        map[string]*instance.Manager
	prWorkflows      map[string]*instance.PRWorkflow
	wt               *worktree.Manager
	conflictDetector *conflict.Detector
	config           *config.Config

	// Current display dimensions for tmux sessions
	// These are updated when the TUI window resizes
	displayWidth  int
	displayHeight int

	// Callback for when PR workflow completes and instance should be removed
	prCompleteCallback func(instanceID string, success bool)

	// Callback for when a PR URL is detected in instance output (inline PR creation)
	prOpenedCallback func(instanceID string)

	// Callback for when an instance timeout is detected
	timeoutCallback func(instanceID string, timeoutType instance.TimeoutType)

	// Callback for when a terminal bell is detected in an instance
	bellCallback func(instanceID string)

	mu sync.RWMutex
}

// New creates a new Orchestrator for the given repository
func New(baseDir string) (*Orchestrator, error) {
	return NewWithConfig(baseDir, config.Get())
}

// NewWithConfig creates a new Orchestrator with the given configuration.
// This is a legacy constructor that doesn't support multi-session - use NewWithSession instead.
func NewWithConfig(baseDir string, cfg *config.Config) (*Orchestrator, error) {
	claudioDir := filepath.Join(baseDir, ".claudio")
	worktreeDir := filepath.Join(claudioDir, "worktrees")

	wt, err := worktree.New(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree manager: %w", err)
	}

	// Create conflict detector
	detector, err := conflict.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create conflict detector: %w", err)
	}

	return &Orchestrator{
		baseDir:          baseDir,
		claudioDir:       claudioDir,
		worktreeDir:      worktreeDir,
		instances:        make(map[string]*instance.Manager),
		prWorkflows:      make(map[string]*instance.PRWorkflow),
		wt:               wt,
		conflictDetector: detector,
		config:           cfg,
	}, nil
}

// NewWithSession creates a new Orchestrator for a specific session.
// This is the preferred constructor for multi-session support.
// The sessionID determines the storage location and lock file.
func NewWithSession(baseDir, sessionID string, cfg *config.Config) (*Orchestrator, error) {
	claudioDir := filepath.Join(baseDir, ".claudio")
	worktreeDir := filepath.Join(claudioDir, "worktrees")
	sessionDir := session.GetSessionDir(baseDir, sessionID)

	wt, err := worktree.New(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree manager: %w", err)
	}

	// Create conflict detector
	detector, err := conflict.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create conflict detector: %w", err)
	}

	return &Orchestrator{
		baseDir:          baseDir,
		claudioDir:       claudioDir,
		worktreeDir:      worktreeDir,
		sessionID:        sessionID,
		sessionDir:       sessionDir,
		instances:        make(map[string]*instance.Manager),
		prWorkflows:      make(map[string]*instance.PRWorkflow),
		wt:               wt,
		conflictDetector: detector,
		config:           cfg,
	}, nil
}

// Init initializes the Claudio directory structure
func (o *Orchestrator) Init() error {
	// Create .claudio directory
	if err := os.MkdirAll(o.claudioDir, 0755); err != nil {
		return fmt.Errorf("failed to create .claudio directory: %w", err)
	}

	// Create worktrees directory
	if err := os.MkdirAll(o.worktreeDir, 0755); err != nil {
		return fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Create session directory if using multi-session
	if o.sessionDir != "" {
		if err := os.MkdirAll(o.sessionDir, 0755); err != nil {
			return fmt.Errorf("failed to create session directory: %w", err)
		}
	}

	return nil
}

// StartSession creates and starts a new session
func (o *Orchestrator) StartSession(name string) (*Session, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Ensure initialized
	if err := o.Init(); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to initialize orchestrator", "error", err)
		}
		return nil, err
	}

	// Acquire session lock if using multi-session
	if o.sessionDir != "" && o.sessionID != "" {
		lock, err := session.AcquireLock(o.sessionDir, o.sessionID)
		if err != nil {
			if o.logger != nil {
				o.logger.Error("failed to acquire session lock", "session_id", o.sessionID, "error", err)
			}
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
		if o.logger != nil {
			o.logger.Error("failed to save session", "session_id", sess.ID, "error", err)
		}
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Log session start
	if o.logger != nil {
		o.logger.Info("session started",
			"session_id", sess.ID,
			"name", name,
			"base_dir", o.baseDir,
		)
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
		if o.logger != nil {
			o.logger.Error("failed to read session file", "file_path", sessionFile, "error", err)
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to parse session file", "file_path", sessionFile, "error", err)
		}
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

	// Log session loaded
	if o.logger != nil {
		o.logger.Info("session loaded",
			"session_id", sess.ID,
			"instance_count", len(sess.Instances),
		)
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

// HasExistingSession checks if there's an existing session file
func (o *Orchestrator) HasExistingSession() bool {
	var sessionFile string
	if o.sessionDir != "" {
		sessionFile = filepath.Join(o.sessionDir, "session.json")
	} else {
		// Legacy single-session mode
		sessionFile = filepath.Join(o.claudioDir, "session.json")
	}
	_, err := os.Stat(sessionFile)
	return err == nil
}

// HasLegacySession checks if there's a legacy single-session file
// that might need migration to multi-session format.
func (o *Orchestrator) HasLegacySession() bool {
	legacyFile := filepath.Join(o.claudioDir, "session.json")
	_, err := os.Stat(legacyFile)
	return err == nil
}

// SessionID returns the current session ID
func (o *Orchestrator) SessionID() string {
	return o.sessionID
}

// ReleaseLock releases the session lock if one is held.
// Safe to call multiple times.
func (o *Orchestrator) ReleaseLock() error {
	if o.lock != nil {
		err := o.lock.Release()
		o.lock = nil
		return err
	}
	return nil
}

// GetOrphanedTmuxSessions returns tmux sessions that exist but aren't tracked by the current session
func (o *Orchestrator) GetOrphanedTmuxSessions() ([]string, error) {
	tmuxSessions, err := instance.ListClaudioTmuxSessions()
	if err != nil {
		return nil, err
	}

	if o.session == nil {
		return tmuxSessions, nil
	}

	// Build set of tracked session names
	tracked := make(map[string]bool)
	for _, inst := range o.session.Instances {
		sessionName := fmt.Sprintf("claudio-%s", inst.ID)
		tracked[sessionName] = true
	}

	// Find orphaned sessions
	var orphaned []string
	for _, sess := range tmuxSessions {
		if !tracked[sess] {
			orphaned = append(orphaned, sess)
		}
	}

	return orphaned, nil
}

// CleanOrphanedTmuxSessions kills all orphaned claudio tmux sessions
func (o *Orchestrator) CleanOrphanedTmuxSessions() (int, error) {
	orphaned, err := o.GetOrphanedTmuxSessions()
	if err != nil {
		return 0, err
	}

	cleaned := 0
	for _, sess := range orphaned {
		cmd := exec.Command("tmux", "kill-session", "-t", sess)
		if cmd.Run() == nil {
			cleaned++
		}
	}

	return cleaned, nil
}

// AddInstance adds a new Claude instance to the session
func (o *Orchestrator) AddInstance(session *Session, task string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance
	inst := NewInstance(task)

	// Generate branch name from task using configured naming convention
	branchSlug := slugify(task)
	inst.Branch = o.generateBranchName(inst.ID, branchSlug)

	// Create worktree
	wtPath := filepath.Join(o.worktreeDir, inst.ID)
	if err := o.wt.Create(wtPath, inst.Branch); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to create worktree",
				"instance_id", inst.ID,
				"worktree_path", wtPath,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}
	inst.WorktreePath = wtPath

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, task)
	o.instances[inst.ID] = mgr

	// Register with conflict detector
	if err := o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath); err != nil {
		// Non-fatal, log at DEBUG since this is conflict detection related
		if o.logger != nil {
			o.logger.Debug("failed to watch instance for conflicts",
				"instance_id", inst.ID,
				"error", err,
			)
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to watch instance for conflicts: %v\n", err)
	}

	// Update shared context
	if err := o.updateContext(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	if err := o.saveSession(); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to save session after adding instance",
				"instance_id", inst.ID,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Log instance added
	if o.logger != nil {
		o.logger.Info("instance added",
			"instance_id", inst.ID,
			"task", truncateString(task, 100),
			"branch", inst.Branch,
		)
	}

	return inst, nil
}

// AddInstanceToWorktree adds a new instance that uses an existing worktree
// This is used for revision tasks that need to work in the same worktree as the original task
func (o *Orchestrator) AddInstanceToWorktree(session *Session, task string, worktreePath string, branch string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance with pre-set worktree info
	inst := NewInstance(task)
	inst.WorktreePath = worktreePath
	inst.Branch = branch

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, task)
	o.instances[inst.ID] = mgr

	// Save session
	if err := o.saveSession(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return inst, nil
}

// AddInstanceFromBranch adds a new Claude instance with a worktree branched from a specific base branch.
// This is used for ultraplan tasks where the next group should build on the consolidated branch from the previous group.
func (o *Orchestrator) AddInstanceFromBranch(session *Session, task string, baseBranch string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance
	inst := NewInstance(task)

	// Generate branch name from task using configured naming convention
	branchSlug := slugify(task)
	inst.Branch = o.generateBranchName(inst.ID, branchSlug)

	// Create worktree from the specified base branch
	wtPath := filepath.Join(o.worktreeDir, inst.ID)
	if err := o.wt.CreateFromBranch(wtPath, inst.Branch, baseBranch); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to create worktree from branch",
				"instance_id", inst.ID,
				"base_branch", baseBranch,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to create worktree from branch %s: %w", baseBranch, err)
	}
	inst.WorktreePath = wtPath

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, task)
	o.instances[inst.ID] = mgr

	// Register with conflict detector
	if err := o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath); err != nil {
		// Non-fatal, log at DEBUG since this is conflict detection related
		if o.logger != nil {
			o.logger.Debug("failed to watch instance for conflicts",
				"instance_id", inst.ID,
				"error", err,
			)
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to watch instance for conflicts: %v\n", err)
	}

	// Update shared context
	if err := o.updateContext(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	if err := o.saveSession(); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to save session after adding instance from branch",
				"instance_id", inst.ID,
				"error", err,
			)
		}
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	// Log instance added
	if o.logger != nil {
		o.logger.Info("instance added",
			"instance_id", inst.ID,
			"task", truncateString(task, 100),
			"branch", inst.Branch,
			"base_branch", baseBranch,
		)
	}

	return inst, nil
}

// StartInstance starts a Claude process for an instance
func (o *Orchestrator) StartInstance(inst *Instance) error {
	o.mu.Lock()
	mgr, ok := o.instances[inst.ID]
	o.mu.Unlock()

	if !ok {
		mgr = o.newInstanceManager(inst.ID, inst.WorktreePath, inst.Task)
		o.mu.Lock()
		o.instances[inst.ID] = mgr
		o.mu.Unlock()
	}

	// Configure state change callback for notifications
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

	// Configure metrics callback for resource tracking
	mgr.SetMetricsCallback(func(id string, metrics *instance.ParsedMetrics) {
		o.handleInstanceMetrics(id, metrics)
	})

	// Configure timeout callback
	mgr.SetTimeoutCallback(func(id string, timeoutType instance.TimeoutType) {
		o.handleInstanceTimeout(id, timeoutType)
	})

	// Configure bell callback to forward terminal bells
	mgr.SetBellCallback(func(id string) {
		o.handleInstanceBell(id)
	})

	if err := mgr.Start(); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to start instance",
				"instance_id", inst.ID,
				"error", err,
			)
		}
		return fmt.Errorf("failed to start instance: %w", err)
	}

	inst.Status = StatusWorking
	inst.PID = mgr.PID()
	inst.TmuxSession = mgr.SessionName() // Save tmux session name for recovery

	// Initialize metrics with start time
	now := mgr.StartTime()
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{StartTime: now}
	} else {
		inst.Metrics.StartTime = now
	}

	// Log instance started
	if o.logger != nil {
		o.logger.Info("instance started",
			"instance_id", inst.ID,
			"tmux_session", inst.TmuxSession,
			"pid", inst.PID,
		)
	}

	return o.saveSession()
}

// StopInstance stops a running Claude instance
func (o *Orchestrator) StopInstance(inst *Instance) error {
	o.mu.RLock()
	mgr, ok := o.instances[inst.ID]
	o.mu.RUnlock()

	if !ok {
		return nil // Already stopped
	}

	if err := mgr.Stop(); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to stop instance",
				"instance_id", inst.ID,
				"error", err,
			)
		}
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	inst.Status = StatusCompleted
	inst.PID = 0

	// Log instance stopped
	if o.logger != nil {
		o.logger.Info("instance stopped",
			"instance_id", inst.ID,
			"status", string(inst.Status),
		)
	}

	return o.saveSession()
}

// StopInstanceWithAutoPR stops an instance and optionally starts PR workflow
// Returns true if PR workflow was started, false if instance was just stopped
func (o *Orchestrator) StopInstanceWithAutoPR(inst *Instance) (bool, error) {
	// First, stop the Claude instance
	if err := o.StopInstance(inst); err != nil {
		return false, err
	}

	// Check if auto PR is enabled
	if !o.config.PR.AutoPROnStop {
		return false, nil
	}

	// Start the PR workflow
	if err := o.StartPRWorkflow(inst); err != nil {
		return false, fmt.Errorf("failed to start PR workflow: %w", err)
	}

	return true, nil
}

// StartPRWorkflow starts the commit-push-PR workflow for an instance
func (o *Orchestrator) StartPRWorkflow(inst *Instance) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create PR workflow configuration from orchestrator config
	cfg := instance.PRWorkflowConfig{
		UseAI:      o.config.PR.UseAI,
		Draft:      o.config.PR.Draft,
		AutoRebase: o.config.PR.AutoRebase,
		TmuxWidth:  o.displayWidth,
		TmuxHeight: o.displayHeight,
	}

	// Use config defaults if display dimensions not set
	if cfg.TmuxWidth == 0 {
		cfg.TmuxWidth = o.config.Instance.TmuxWidth
	}
	if cfg.TmuxHeight == 0 {
		cfg.TmuxHeight = o.config.Instance.TmuxHeight
	}

	// Create and start PR workflow with session-scoped naming if in multi-session mode
	var workflow *instance.PRWorkflow
	if o.sessionID != "" {
		workflow = instance.NewPRWorkflowWithSession(o.sessionID, inst.ID, inst.WorktreePath, inst.Branch, inst.Task, cfg)
	} else {
		workflow = instance.NewPRWorkflow(inst.ID, inst.WorktreePath, inst.Branch, inst.Task, cfg)
	}
	workflow.SetCallback(o.handlePRWorkflowComplete)

	if err := workflow.Start(); err != nil {
		return err
	}

	o.prWorkflows[inst.ID] = workflow
	inst.Status = StatusCreatingPR

	return o.saveSession()
}

// handlePRWorkflowComplete handles PR workflow completion
func (o *Orchestrator) handlePRWorkflowComplete(instanceID string, success bool, output string) {
	o.mu.Lock()
	// Clean up PR workflow
	delete(o.prWorkflows, instanceID)

	// Get the callback before unlocking
	callback := o.prCompleteCallback
	o.mu.Unlock()

	// Notify via callback if set
	if callback != nil {
		callback(instanceID, success)
	}
}

// SetPRCompleteCallback sets the callback for PR workflow completion
func (o *Orchestrator) SetPRCompleteCallback(cb func(instanceID string, success bool)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.prCompleteCallback = cb
}

// SetPROpenedCallback sets the callback for when a PR URL is detected in instance output
func (o *Orchestrator) SetPROpenedCallback(cb func(instanceID string)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.prOpenedCallback = cb
}

// SetTimeoutCallback sets the callback for when an instance timeout is detected
func (o *Orchestrator) SetTimeoutCallback(cb func(instanceID string, timeoutType instance.TimeoutType)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.timeoutCallback = cb
}

// SetBellCallback sets the callback for when a terminal bell is detected in an instance
func (o *Orchestrator) SetBellCallback(cb func(instanceID string)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.bellCallback = cb
}

// SetLogger sets the logger for the orchestrator.
// If logger is nil, logging is disabled (no-op pattern).
func (o *Orchestrator) SetLogger(logger *logging.Logger) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.logger = logger
}

// Logger returns the current logger, or nil if logging is disabled.
func (o *Orchestrator) Logger() *logging.Logger {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.logger
}

// GetPRWorkflow returns the PR workflow for an instance, if any
func (o *Orchestrator) GetPRWorkflow(id string) *instance.PRWorkflow {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.prWorkflows[id]
}

// RemoveInstance stops and removes a specific instance, including its worktree and branch
func (o *Orchestrator) RemoveInstance(session *Session, instanceID string, force bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Find the instance
	var inst *Instance
	var instIndex int
	for i, instance := range session.Instances {
		if instance.ID == instanceID {
			inst = instance
			instIndex = i
			break
		}
	}

	if inst == nil {
		return fmt.Errorf("instance %s not found", instanceID)
	}

	// Check for uncommitted changes if not forcing
	if !force {
		hasChanges, err := o.wt.HasUncommittedChanges(inst.WorktreePath)
		if err == nil && hasChanges {
			return fmt.Errorf("instance %s has uncommitted changes. Use --force to remove anyway", instanceID)
		}
	}

	// Stop the instance if running
	if mgr, ok := o.instances[inst.ID]; ok {
		_ = mgr.Stop()
		delete(o.instances, inst.ID)
	}

	// Stop PR workflow if running
	if workflow, ok := o.prWorkflows[inst.ID]; ok {
		_ = workflow.Stop()
		delete(o.prWorkflows, inst.ID)
	}

	// Remove worktree
	if err := o.wt.Remove(inst.WorktreePath); err != nil {
		// Log but don't fail - the directory might already be gone
		fmt.Fprintf(os.Stderr, "Warning: failed to remove worktree: %v\n", err)
	}

	// Delete branch
	if err := o.wt.DeleteBranch(inst.Branch); err != nil {
		// Log but don't fail - the branch might already be gone
		fmt.Fprintf(os.Stderr, "Warning: failed to delete branch: %v\n", err)
	}

	// Remove from session
	session.Instances = append(session.Instances[:instIndex], session.Instances[instIndex+1:]...)

	// Update context
	if err := o.updateContext(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	return o.saveSession()
}

// StopSession stops all instances and optionally cleans up
func (o *Orchestrator) StopSession(sess *Session, force bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	instanceCount := len(sess.Instances)

	// Stop conflict detector
	if o.conflictDetector != nil {
		o.conflictDetector.Stop()
	}

	// Stop all instances
	for _, inst := range sess.Instances {
		if mgr, ok := o.instances[inst.ID]; ok {
			_ = mgr.Stop()
		}
	}

	// Stop all PR workflows
	for _, workflow := range o.prWorkflows {
		_ = workflow.Stop()
	}
	o.prWorkflows = make(map[string]*instance.PRWorkflow)

	// Clean up worktrees if forced
	if force {
		for _, inst := range sess.Instances {
			_ = o.wt.Remove(inst.WorktreePath)
		}
	}

	// Release session lock
	if o.lock != nil {
		_ = o.lock.Release()
		o.lock = nil
	}

	// Remove session file
	var sessionFile string
	if o.sessionDir != "" {
		sessionFile = filepath.Join(o.sessionDir, "session.json")
	} else {
		sessionFile = filepath.Join(o.claudioDir, "session.json")
	}
	_ = os.Remove(sessionFile)

	// Log session stopped
	if o.logger != nil {
		o.logger.Info("session stopped",
			"session_id", sess.ID,
			"instance_count", instanceCount,
		)
	}

	return nil
}

// GetInstanceManager returns the manager for an instance
func (o *Orchestrator) GetInstanceManager(id string) *instance.Manager {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.instances[id]
}

// Config returns the orchestrator's configuration
func (o *Orchestrator) Config() *config.Config {
	return o.config
}

// instanceManagerConfig converts the orchestrator config to instance.ManagerConfig
// Uses the current display dimensions if available, otherwise falls back to config defaults
func (o *Orchestrator) instanceManagerConfig() instance.ManagerConfig {
	width := o.config.Instance.TmuxWidth
	height := o.config.Instance.TmuxHeight

	// Use the current display dimensions if they've been set by a resize event
	if o.displayWidth > 0 {
		width = o.displayWidth
	}
	if o.displayHeight > 0 {
		height = o.displayHeight
	}

	return instance.ManagerConfig{
		OutputBufferSize:         o.config.Instance.OutputBufferSize,
		CaptureIntervalMs:        o.config.Instance.CaptureIntervalMs,
		TmuxWidth:                width,
		TmuxHeight:               height,
		ActivityTimeoutMinutes:   o.config.Instance.ActivityTimeoutMinutes,
		CompletionTimeoutMinutes: o.config.Instance.CompletionTimeoutMinutes,
		StaleDetection:           o.config.Instance.StaleDetection,
	}
}

// newInstanceManager creates a new instance manager with the appropriate constructor.
// Uses session-scoped tmux naming when sessionID is set (multi-session mode).
func (o *Orchestrator) newInstanceManager(instanceID, workdir, task string) *instance.Manager {
	cfg := o.instanceManagerConfig()
	if o.sessionID != "" {
		return instance.NewManagerWithSession(o.sessionID, instanceID, workdir, task, cfg)
	}
	return instance.NewManagerWithConfig(instanceID, workdir, task, cfg)
}

// Session returns the current session
func (o *Orchestrator) Session() *Session {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.session
}

// GetConflictDetector returns the conflict detector
func (o *Orchestrator) GetConflictDetector() *conflict.Detector {
	return o.conflictDetector
}

// SetDisplayDimensions sets the initial display dimensions for new instances
// This should be called before the TUI starts to ensure instances are created
// with the correct size from the beginning
func (o *Orchestrator) SetDisplayDimensions(width, height int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.displayWidth = width
	o.displayHeight = height
}

// ResizeAllInstances resizes all running tmux sessions to the given dimensions
// and stores the dimensions for new instances
func (o *Orchestrator) ResizeAllInstances(width, height int) {
	o.mu.Lock()
	o.displayWidth = width
	o.displayHeight = height
	o.mu.Unlock()

	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, mgr := range o.instances {
		if mgr != nil && mgr.Running() {
			_ = mgr.Resize(width, height)
		}
	}
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
		if o.logger != nil {
			o.logger.Error("failed to marshal session data", "error", err)
		}
		return err
	}

	if err := os.WriteFile(sessionFile, data, 0644); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to write session file", "file_path", sessionFile, "error", err)
		}
		return err
	}

	// Log session saved at DEBUG level
	if o.logger != nil {
		o.logger.Debug("session saved", "file_path", sessionFile)
	}

	return nil
}

// SaveSession is a public wrapper for saveSession, used by components
// like the Coordinator that need to trigger session persistence
func (o *Orchestrator) SaveSession() error {
	return o.saveSession()
}

// updateContext updates the shared context file in all worktrees
func (o *Orchestrator) updateContext() error {
	if o.session == nil {
		return nil
	}

	ctx := o.generateContextMarkdown()

	// Write to session directory if using multi-session, otherwise main .claudio directory
	var mainCtx string
	if o.sessionDir != "" {
		mainCtx = filepath.Join(o.sessionDir, "context.md")
	} else {
		mainCtx = filepath.Join(o.claudioDir, "context.md")
	}
	if err := os.WriteFile(mainCtx, []byte(ctx), 0644); err != nil {
		if o.logger != nil {
			o.logger.Error("failed to write context file", "file_path", mainCtx, "error", err)
		}
		return err
	}

	// Write to each worktree
	for _, inst := range o.session.Instances {
		wtCtx := filepath.Join(inst.WorktreePath, ".claudio", "context.md")
		_ = os.MkdirAll(filepath.Dir(wtCtx), 0755)
		_ = os.WriteFile(wtCtx, []byte(ctx), 0644)
	}

	// Log context update at DEBUG level
	if o.logger != nil {
		o.logger.Debug("context updated", "instance_count", len(o.session.Instances))
	}

	return nil
}

// generateContextMarkdown creates the shared context markdown
func (o *Orchestrator) generateContextMarkdown() string {
	var sb strings.Builder

	sb.WriteString("# Claudio Session Context\n\n")
	sb.WriteString("This file is automatically updated by Claudio to help coordinate work across instances.\n\n")
	sb.WriteString("## Active Instances\n\n")

	for i, inst := range o.session.Instances {
		sb.WriteString(fmt.Sprintf("### Instance %d: %s\n", i+1, inst.ID))
		sb.WriteString(fmt.Sprintf("- **Status**: %s\n", inst.Status))
		sb.WriteString(fmt.Sprintf("- **Task**: %s\n", inst.Task))
		sb.WriteString(fmt.Sprintf("- **Branch**: %s\n", inst.Branch))
		if len(inst.FilesModified) > 0 {
			sb.WriteString(fmt.Sprintf("- **Files**: %s\n", strings.Join(inst.FilesModified, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Coordination Notes\n\n")
	sb.WriteString("- Each instance works in its own worktree/branch\n")
	sb.WriteString("- Avoid modifying files that other instances are working on\n")
	sb.WriteString("- Check this context file for updates on what others are doing\n")

	return sb.String()
}

// generateBranchName creates a branch name using the configured naming convention
func (o *Orchestrator) generateBranchName(instanceID, slug string) string {
	prefix := o.config.Branch.Prefix
	if prefix == "" {
		prefix = "claudio" // fallback default
	}

	if o.config.Branch.IncludeID {
		return fmt.Sprintf("%s/%s-%s", prefix, instanceID, slug)
	}
	return fmt.Sprintf("%s/%s", prefix, slug)
}

// BranchPrefix returns the configured branch prefix for use by other packages
func (o *Orchestrator) BranchPrefix() string {
	prefix := o.config.Branch.Prefix
	if prefix == "" {
		return "claudio"
	}
	return prefix
}

// slugify creates a URL-friendly slug from text
func slugify(text string) string {
	// Simple slugify: lowercase, replace spaces with dashes, limit length
	slug := strings.ToLower(text)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except dashes
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	slug = result.String()

	// Limit length
	if len(slug) > 30 {
		slug = slug[:30]
	}

	// Remove trailing dash
	slug = strings.TrimSuffix(slug, "-")

	return slug
}

// timeoutTypeString converts a TimeoutType to its string representation for logging.
func timeoutTypeString(t instance.TimeoutType) string {
	switch t {
	case instance.TimeoutActivity:
		return "activity"
	case instance.TimeoutCompletion:
		return "completion"
	case instance.TimeoutStale:
		return "stale"
	default:
		return "unknown"
	}
}

// handleInstanceExit handles when a Claude instance process exits
func (o *Orchestrator) handleInstanceExit(id string) {
	inst := o.GetInstance(id)
	if inst != nil {
		inst.Status = StatusCompleted
		inst.PID = 0
		// Record end time for metrics
		if inst.Metrics != nil {
			now := time.Now()
			inst.Metrics.EndTime = &now
		}
		_ = o.saveSession()
		o.executeNotification("notifications.on_completion", inst)
	}
}

// handleInstanceMetrics updates instance metrics when they change
func (o *Orchestrator) handleInstanceMetrics(id string, metrics *instance.ParsedMetrics) {
	inst := o.GetInstance(id)
	if inst == nil || metrics == nil {
		return
	}

	// Update instance metrics
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{}
	}

	inst.Metrics.InputTokens = metrics.InputTokens
	inst.Metrics.OutputTokens = metrics.OutputTokens
	inst.Metrics.CacheRead = metrics.CacheRead
	inst.Metrics.CacheWrite = metrics.CacheWrite
	inst.Metrics.APICalls = metrics.APICalls

	// Use parsed cost if available, otherwise calculate from tokens
	if metrics.Cost > 0 {
		inst.Metrics.Cost = metrics.Cost
	} else {
		inst.Metrics.Cost = instance.CalculateCost(
			metrics.InputTokens,
			metrics.OutputTokens,
			metrics.CacheRead,
			metrics.CacheWrite,
		)
	}

	// Check budget limits
	o.checkBudgetLimits()

	// Save session periodically (not on every metric update to avoid excessive I/O)
	// The session will be saved when status changes occur
}

// checkBudgetLimits checks if any budget limits have been exceeded
func (o *Orchestrator) checkBudgetLimits() {
	if o.config == nil || o.session == nil {
		return
	}

	// Get session totals
	sessionMetrics := o.GetSessionMetrics()

	// Check cost limit
	if o.config.Resources.CostLimit > 0 && sessionMetrics.TotalCost >= o.config.Resources.CostLimit {
		if o.logger != nil {
			o.logger.Warn("budget limit exceeded, pausing all instances",
				"total_cost", sessionMetrics.TotalCost,
				"cost_limit", o.config.Resources.CostLimit,
			)
		}
		// Pause all running instances
		for _, inst := range o.session.Instances {
			if inst.Status == StatusWorking {
				if mgr, ok := o.instances[inst.ID]; ok {
					_ = mgr.Pause()
					inst.Status = StatusPaused
				}
			}
		}
		o.executeNotification("notifications.on_budget_limit", nil)
	}

	// Check cost warning threshold
	if o.config.Resources.CostWarningThreshold > 0 && sessionMetrics.TotalCost >= o.config.Resources.CostWarningThreshold {
		if o.logger != nil {
			o.logger.Warn("budget warning threshold reached",
				"total_cost", sessionMetrics.TotalCost,
				"warning_threshold", o.config.Resources.CostWarningThreshold,
			)
		}
		o.executeNotification("notifications.on_budget_warning", nil)
	}

	// Check per-instance token limit
	if o.config.Resources.TokenLimitPerInstance > 0 {
		for _, inst := range o.session.Instances {
			if inst.Metrics != nil && inst.Status == StatusWorking {
				if inst.Metrics.TotalTokens() >= o.config.Resources.TokenLimitPerInstance {
					if o.logger != nil {
						o.logger.Warn("instance token limit exceeded",
							"instance_id", inst.ID,
							"total_tokens", inst.Metrics.TotalTokens(),
							"token_limit", o.config.Resources.TokenLimitPerInstance,
						)
					}
					if mgr, ok := o.instances[inst.ID]; ok {
						_ = mgr.Pause()
						inst.Status = StatusPaused
					}
				}
			}
		}
	}
}

// SessionMetrics holds aggregated metrics for the entire session
type SessionMetrics struct {
	TotalInputTokens  int64
	TotalOutputTokens int64
	TotalCacheRead    int64
	TotalCacheWrite   int64
	TotalCost         float64
	TotalAPICalls     int
	TotalDuration     time.Duration
	InstanceCount     int
	ActiveCount       int
}

// GetSessionMetrics aggregates metrics across all instances in the session
func (o *Orchestrator) GetSessionMetrics() *SessionMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.session == nil {
		return &SessionMetrics{}
	}

	metrics := &SessionMetrics{
		InstanceCount: len(o.session.Instances),
	}

	for _, inst := range o.session.Instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			metrics.ActiveCount++
		}

		if inst.Metrics != nil {
			metrics.TotalInputTokens += inst.Metrics.InputTokens
			metrics.TotalOutputTokens += inst.Metrics.OutputTokens
			metrics.TotalCacheRead += inst.Metrics.CacheRead
			metrics.TotalCacheWrite += inst.Metrics.CacheWrite
			metrics.TotalCost += inst.Metrics.Cost
			metrics.TotalAPICalls += inst.Metrics.APICalls
			metrics.TotalDuration += inst.Metrics.Duration()
		}
	}

	return metrics
}

// GetInstanceMetrics returns the current metrics for a specific instance
func (o *Orchestrator) GetInstanceMetrics(id string) *Metrics {
	inst := o.GetInstance(id)
	if inst == nil {
		return nil
	}
	return inst.Metrics
}

// handleInstanceWaitingInput handles when a Claude instance is waiting for input
func (o *Orchestrator) handleInstanceWaitingInput(id string) {
	inst := o.GetInstance(id)
	if inst != nil {
		inst.Status = StatusWaitingInput
		_ = o.saveSession()
		o.executeNotification("notifications.on_waiting_input", inst)
	}
}

// handleInstancePROpened handles when a PR URL is detected in instance output
func (o *Orchestrator) handleInstancePROpened(id string) {
	o.mu.RLock()
	callback := o.prOpenedCallback
	o.mu.RUnlock()

	// Notify via callback if set (TUI will handle the removal)
	if callback != nil {
		callback(id)
	}
}

// handleInstanceTimeout handles when an instance timeout is detected
func (o *Orchestrator) handleInstanceTimeout(id string, timeoutType instance.TimeoutType) {
	inst := o.GetInstance(id)
	if inst == nil {
		return
	}

	// Log timeout detection at WARN level
	if o.logger != nil {
		o.logger.Warn("instance timeout detected",
			"instance_id", id,
			"timeout_type", timeoutTypeString(timeoutType),
		)
	}

	// Update status based on timeout type
	switch timeoutType {
	case instance.TimeoutActivity, instance.TimeoutStale:
		inst.Status = StatusStuck
	case instance.TimeoutCompletion:
		inst.Status = StatusTimeout
	}

	// Record end time for metrics
	if inst.Metrics != nil {
		now := time.Now()
		inst.Metrics.EndTime = &now
	}

	_ = o.saveSession()

	// Notify via callback if set (TUI will handle the display)
	o.mu.RLock()
	callback := o.timeoutCallback
	o.mu.RUnlock()

	if callback != nil {
		callback(id, timeoutType)
	}
}

// handleInstanceBell handles when a terminal bell is detected in an instance
func (o *Orchestrator) handleInstanceBell(id string) {
	o.mu.RLock()
	callback := o.bellCallback
	o.mu.RUnlock()

	if callback != nil {
		callback(id)
	}
}

// executeNotification executes a notification command from config
func (o *Orchestrator) executeNotification(configKey string, inst *Instance) {
	cmd := viper.GetString(configKey)
	if cmd == "" {
		return
	}

	// Replace placeholders
	cmd = strings.ReplaceAll(cmd, "{id}", inst.ID)
	cmd = strings.ReplaceAll(cmd, "{task}", inst.Task)
	cmd = strings.ReplaceAll(cmd, "{branch}", inst.Branch)
	cmd = strings.ReplaceAll(cmd, "{status}", string(inst.Status))

	// Execute asynchronously to not block
	go func() {
		_ = exec.Command("sh", "-c", cmd).Run()
	}()
}

// GetInstance returns an instance by ID from the current session
func (o *Orchestrator) GetInstance(id string) *Instance {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.session == nil {
		return nil
	}

	for _, inst := range o.session.Instances {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}

// GetInstanceDiff returns the git diff for an instance against main
func (o *Orchestrator) GetInstanceDiff(worktreePath string) (string, error) {
	return o.wt.GetDiffAgainstMain(worktreePath)
}

// ReconnectInstance attempts to reconnect to a stopped or paused instance
// If the tmux session still exists, it reconnects to it
// If not, it restarts Claude with the same task in the existing worktree
func (o *Orchestrator) ReconnectInstance(inst *Instance) error {
	o.mu.Lock()
	mgr, ok := o.instances[inst.ID]
	o.mu.Unlock()

	// If no manager exists yet, create one
	if !ok {
		mgr = o.newInstanceManager(inst.ID, inst.WorktreePath, inst.Task)
		o.mu.Lock()
		o.instances[inst.ID] = mgr
		o.mu.Unlock()
	}

	// Configure state change callback
	mgr.SetStateCallback(func(id string, state instance.WaitingState) {
		switch state {
		case instance.StateCompleted:
			o.handleInstanceExit(id)
		case instance.StateWaitingInput, instance.StateWaitingQuestion, instance.StateWaitingPermission:
			o.handleInstanceWaitingInput(id)
		}
	})

	// Configure metrics callback
	mgr.SetMetricsCallback(func(id string, metrics *instance.ParsedMetrics) {
		o.handleInstanceMetrics(id, metrics)
	})

	// Configure timeout callback
	mgr.SetTimeoutCallback(func(id string, timeoutType instance.TimeoutType) {
		o.handleInstanceTimeout(id, timeoutType)
	})

	// Configure bell callback to forward terminal bells
	mgr.SetBellCallback(func(id string) {
		o.handleInstanceBell(id)
	})

	// Check if the tmux session still exists
	if mgr.TmuxSessionExists() {
		// Reconnect to the existing session
		if err := mgr.Reconnect(); err != nil {
			return fmt.Errorf("failed to reconnect to existing session: %w", err)
		}
	} else {
		// Session doesn't exist - start a fresh one with the same task
		if err := mgr.Start(); err != nil {
			return fmt.Errorf("failed to restart instance: %w", err)
		}
	}

	inst.Status = StatusWorking
	inst.PID = mgr.PID()
	inst.TmuxSession = mgr.SessionName()

	// Update start time for metrics
	now := mgr.StartTime()
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{StartTime: now}
	} else {
		inst.Metrics.StartTime = now
		inst.Metrics.EndTime = nil // Clear end time since we're restarting
	}

	return o.saveSession()
}

// ClearCompletedInstances removes all instances with StatusCompleted from the session
// Returns the number of instances removed and any error encountered
func (o *Orchestrator) ClearCompletedInstances(session *Session) (int, error) {
	// Collect IDs of completed instances first (to avoid modifying slice while iterating)
	var completedIDs []string
	for _, inst := range session.Instances {
		if inst.Status == StatusCompleted {
			completedIDs = append(completedIDs, inst.ID)
		}
	}

	if len(completedIDs) == 0 {
		return 0, nil
	}

	// Remove each completed instance (force=true since they're already completed)
	removed := 0
	for _, id := range completedIDs {
		if err := o.RemoveInstance(session, id, true); err != nil {
			// Log warning but continue with other removals
			fmt.Fprintf(os.Stderr, "Warning: failed to remove instance %s: %v\n", id, err)
			continue
		}
		removed++
	}

	return removed, nil
}

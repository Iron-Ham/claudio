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
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	instmetrics "github.com/Iron-Ham/claudio/internal/instance/metrics"
	instancestate "github.com/Iron-Ham/claudio/internal/instance/state"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/namer"
	"github.com/Iron-Ham/claudio/internal/orchestrator/budget"
	"github.com/Iron-Ham/claudio/internal/orchestrator/display"
	"github.com/Iron-Ham/claudio/internal/orchestrator/lifecycle"
	"github.com/Iron-Ham/claudio/internal/orchestrator/prworkflow"
	orchsession "github.com/Iron-Ham/claudio/internal/orchestrator/session"
	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/tmux"
	"github.com/Iron-Ham/claudio/internal/worktree"
	"github.com/spf13/viper"
)

// Orchestrator manages the Claudio session and coordinates instances.
//
// The Orchestrator acts as a facade that composes several specialized managers:
//   - sessionMgr: Handles session persistence (load, save, create, delete)
//   - lifecycleMgr: Manages instance lifecycle (start, stop, status tracking)
//   - prWorkflowMgr: Manages PR workflow operations (commit, push, PR creation)
//   - displayMgr: Manages display dimensions and resize coordination
//   - eventBus: Enables decoupled communication between components
//
// For backwards compatibility, the Orchestrator maintains its existing public API
// while progressively delegating to the composed managers internally.
type Orchestrator struct {
	baseDir     string
	claudioDir  string
	worktreeDir string
	sessionID   string // Current session ID (for multi-session support)
	sessionDir  string // Session-specific directory (.claudio/sessions/{sessionID})
	lock        *session.Lock
	logger      *logging.Logger // Structured logger for debugging (nil = no logging)

	// Composed managers (delegation targets for refactored operations)
	sessionMgr    *orchsession.Manager   // Session lifecycle management
	lifecycleMgr  *lifecycle.Manager     // Instance lifecycle management
	prWorkflowMgr *prworkflow.Manager    // PR workflow management
	displayMgr    *display.Manager       // Display dimension management
	eventBus      *event.Bus             // Inter-component event communication
	stateMonitor  *instancestate.Monitor // Centralized state monitoring for all instances
	budgetMgr     *budget.Manager        // Budget monitoring and enforcement
	namer         *namer.Namer           // Intelligent instance naming (optional)

	session          *Session
	instances        map[string]*instance.Manager
	wt               *worktree.Manager
	conflictDetector *conflict.Detector
	config           *config.Config

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
	worktreeDir := cfg.Paths.ResolveWorktreeDir(baseDir)

	wt, err := worktree.New(baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree manager: %w", err)
	}

	// Create conflict detector
	detector, err := conflict.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create conflict detector: %w", err)
	}

	// Create event bus for inter-component communication
	eventBus := event.NewBus()

	// Create session manager (legacy single-session mode)
	sessionMgr := orchsession.NewManager(orchsession.Config{
		BaseDir: baseDir,
	})

	// Create lifecycle manager with default config
	lifecycleMgr := lifecycle.NewManager(
		lifecycle.DefaultConfig(),
		lifecycle.Callbacks{},
		nil, // logger set later via SetLogger
	)

	// Create PR workflow manager (legacy mode without session ID)
	prWorkflowMgr := prworkflow.NewManager(
		prworkflow.NewConfigFromConfig(cfg),
		"", // no session ID in legacy mode
		eventBus,
	)

	// Create display manager with config-derived defaults
	displayMgr := display.NewManager(display.Config{
		DefaultWidth:  cfg.Instance.TmuxWidth,
		DefaultHeight: cfg.Instance.TmuxHeight,
	})

	// Create centralized state monitor for all instances
	stateMonitor := instancestate.NewMonitor(instancestate.MonitorConfig{
		ActivityTimeoutMinutes:   cfg.Instance.ActivityTimeoutMinutes,
		CompletionTimeoutMinutes: cfg.Instance.CompletionTimeoutMinutes,
		StaleDetection:           cfg.Instance.StaleDetection,
	})

	orch := &Orchestrator{
		baseDir:          baseDir,
		claudioDir:       claudioDir,
		worktreeDir:      worktreeDir,
		sessionMgr:       sessionMgr,
		lifecycleMgr:     lifecycleMgr,
		prWorkflowMgr:    prWorkflowMgr,
		displayMgr:       displayMgr,
		eventBus:         eventBus,
		stateMonitor:     stateMonitor,
		instances:        make(map[string]*instance.Manager),
		wt:               wt,
		conflictDetector: detector,
		config:           cfg,
	}

	// Initialize budget manager with orchestrator as provider and pauser
	orch.initBudgetManager()

	// Wire state monitor callbacks to orchestrator handlers
	orch.wireStateMonitorCallbacks()

	// Initialize intelligent naming service (optional - degrades gracefully if API key not set)
	orch.initNamer()

	return orch, nil
}

// NewWithSession creates a new Orchestrator for a specific session.
// This is the preferred constructor for multi-session support.
// The sessionID determines the storage location and lock file.
func NewWithSession(baseDir, sessionID string, cfg *config.Config) (*Orchestrator, error) {
	claudioDir := filepath.Join(baseDir, ".claudio")
	worktreeDir := cfg.Paths.ResolveWorktreeDir(baseDir)
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

	// Create event bus for inter-component communication
	eventBus := event.NewBus()

	// Create session manager (multi-session mode with sessionID)
	sessionMgr := orchsession.NewManager(orchsession.Config{
		BaseDir:   baseDir,
		SessionID: sessionID,
	})

	// Create lifecycle manager with config derived from orchestrator config
	lifecycleCfg := lifecycle.Config{
		TmuxSessionPrefix: "claudio",
		DefaultTermWidth:  cfg.Instance.TmuxWidth,
		DefaultTermHeight: cfg.Instance.TmuxHeight,
	}
	lifecycleMgr := lifecycle.NewManager(
		lifecycleCfg,
		lifecycle.Callbacks{},
		nil, // logger set later via SetLogger
	)

	// Create PR workflow manager with session-scoped naming
	prWorkflowMgr := prworkflow.NewManager(
		prworkflow.NewConfigFromConfig(cfg),
		sessionID,
		eventBus,
	)

	// Create display manager with config-derived defaults
	displayMgr := display.NewManager(display.Config{
		DefaultWidth:  cfg.Instance.TmuxWidth,
		DefaultHeight: cfg.Instance.TmuxHeight,
	})

	// Create centralized state monitor for all instances
	stateMonitor := instancestate.NewMonitor(instancestate.MonitorConfig{
		ActivityTimeoutMinutes:   cfg.Instance.ActivityTimeoutMinutes,
		CompletionTimeoutMinutes: cfg.Instance.CompletionTimeoutMinutes,
		StaleDetection:           cfg.Instance.StaleDetection,
	})

	orch := &Orchestrator{
		baseDir:          baseDir,
		claudioDir:       claudioDir,
		worktreeDir:      worktreeDir,
		sessionID:        sessionID,
		sessionDir:       sessionDir,
		sessionMgr:       sessionMgr,
		lifecycleMgr:     lifecycleMgr,
		prWorkflowMgr:    prWorkflowMgr,
		displayMgr:       displayMgr,
		eventBus:         eventBus,
		stateMonitor:     stateMonitor,
		instances:        make(map[string]*instance.Manager),
		wt:               wt,
		conflictDetector: detector,
		config:           cfg,
	}

	// Initialize budget manager with orchestrator as provider and pauser
	orch.initBudgetManager()

	// Wire state monitor callbacks to orchestrator handlers
	orch.wireStateMonitorCallbacks()

	// Initialize intelligent naming service (optional - degrades gracefully if API key not set)
	orch.initNamer()

	return orch, nil
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
		lock, err := session.AcquireLock(o.sessionDir, o.sessionID, nil)
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

	sessionFile := o.sessionFilePath()
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
		if err := o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath); err != nil {
			if o.logger != nil {
				o.logger.Warn("failed to register instance with conflict detector",
					"instance_id", inst.ID,
					"worktree_path", inst.WorktreePath,
					"error", err,
				)
			}
		}
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
		lock, err := session.AcquireLock(o.sessionDir, o.sessionID, nil)
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
			mgr.SetStateCallback(func(id string, state detect.WaitingState) {
				switch state {
				case detect.StateCompleted:
					o.handleInstanceExit(id)
				case detect.StateWaitingInput, detect.StateWaitingQuestion, detect.StateWaitingPermission:
					o.handleInstanceWaitingInput(id)
				case detect.StatePROpened:
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
	if err := o.saveSession(); err != nil {
		if o.logger != nil {
			o.logger.Warn("failed to save session after recovery",
				"error", err,
			)
		}
	}

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

// BaseDir returns the base directory (where Claudio was invoked)
func (o *Orchestrator) BaseDir() string {
	return o.baseDir
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
		cmd := tmux.Command("kill-session", "-t", sess)
		if err := cmd.Run(); err != nil {
			if o.logger != nil {
				o.logger.Warn("failed to kill orphaned tmux session",
					"session", sess,
					"error", err,
				)
			}
		} else {
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

	// Register instance with managers and save session
	if err := o.registerInstance(session, inst); err != nil {
		return nil, err
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

// AddInstanceWithDependencies adds a new Claude instance with dependencies on other instances.
// The instance will be created in pending state. If autoStart is true, the orchestrator
// will automatically start the instance when all dependencies complete.
// Dependencies can be specified by instance ID or task name (partial match).
func (o *Orchestrator) AddInstanceWithDependencies(session *Session, task string, dependsOn []string, autoStart bool) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Resolve dependency references to instance IDs
	resolvedDeps := make([]string, 0, len(dependsOn))
	for _, depRef := range dependsOn {
		depInst, err := o.resolveInstanceReference(session, depRef)
		if err != nil {
			return nil, fmt.Errorf("invalid dependency %q: %w", depRef, err)
		}
		resolvedDeps = append(resolvedDeps, depInst.ID)
	}

	// Create instance
	inst := NewInstance(task)
	inst.DependsOn = resolvedDeps
	inst.AutoStart = autoStart

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

	// Update dependents lists on parent instances
	for _, depID := range resolvedDeps {
		for _, existing := range session.Instances {
			if existing.ID == depID {
				existing.Dependents = append(existing.Dependents, inst.ID)
				break
			}
		}
	}

	// Register instance with managers and save session
	if err := o.registerInstance(session, inst); err != nil {
		return nil, err
	}

	// Log instance added
	if o.logger != nil {
		o.logger.Info("instance added with dependencies",
			"instance_id", inst.ID,
			"task", truncateString(task, 100),
			"branch", inst.Branch,
			"depends_on", resolvedDeps,
			"auto_start", autoStart,
		)
	}

	return inst, nil
}

// resolveInstanceReference finds an instance by ID or task name substring.
// Returns an error if no match is found or if multiple instances match a task substring.
func (o *Orchestrator) resolveInstanceReference(session *Session, ref string) (*Instance, error) {
	// First try exact ID match (unambiguous)
	for _, inst := range session.Instances {
		if inst.ID == ref {
			return inst, nil
		}
	}

	// Try task name substring match (case insensitive)
	// Collect all matches to detect ambiguity
	refLower := strings.ToLower(ref)
	var matches []*Instance
	for _, inst := range session.Instances {
		if strings.Contains(strings.ToLower(inst.Task), refLower) {
			matches = append(matches, inst)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no instance found matching %q", ref)
	}
	if len(matches) > 1 {
		// Build list of matching instances for error message
		var matchDescs []string
		for _, m := range matches {
			matchDescs = append(matchDescs, fmt.Sprintf("%s (%s)", m.ID, truncateString(m.Task, 30)))
		}
		return nil, fmt.Errorf("ambiguous reference %q matches %d instances: %s (use instance ID for exact match)",
			ref, len(matches), strings.Join(matchDescs, ", "))
	}

	return matches[0], nil
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

	// Register instance with managers and save session
	if err := o.registerInstance(session, inst); err != nil {
		return nil, err
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

	// Configure all callbacks
	o.configureInstanceCallbacks(mgr)

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

	// Request intelligent naming if namer is available and instance not manually named
	if o.namer != nil && !inst.ManuallyNamed {
		o.namer.RequestRename(inst.ID, inst.Task)
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

// StartPRWorkflow starts the commit-push-PR workflow for an instance.
// Delegates to the prWorkflowMgr for workflow management.
func (o *Orchestrator) StartPRWorkflow(inst *Instance) error {
	// Get current display dimensions from displayMgr (falls back to defaults if not set)
	width, height := o.displayMgr.GetDimensions()

	if width > 0 && height > 0 {
		o.prWorkflowMgr.SetDisplayDimensions(width, height)
	}

	if err := o.prWorkflowMgr.Start(inst); err != nil {
		return err
	}

	inst.Status = StatusCreatingPR
	return o.saveSession()
}

// SetPRCompleteCallback sets the callback for PR workflow completion.
// Delegates to the prWorkflowMgr.
func (o *Orchestrator) SetPRCompleteCallback(cb func(instanceID string, success bool)) {
	o.prWorkflowMgr.SetCompleteCallback(cb)
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

// EventBus returns the event bus for inter-component communication.
// Components can use this to subscribe to or publish events.
func (o *Orchestrator) EventBus() *event.Bus {
	return o.eventBus
}

// SessionManager returns the session manager for advanced session operations.
func (o *Orchestrator) SessionManager() *orchsession.Manager {
	return o.sessionMgr
}

// LifecycleManager returns the lifecycle manager for advanced instance operations.
func (o *Orchestrator) LifecycleManager() *lifecycle.Manager {
	return o.lifecycleMgr
}

// GetPRWorkflow returns the PR workflow for an instance, if any.
// Delegates to the prWorkflowMgr.
func (o *Orchestrator) GetPRWorkflow(id string) *instance.PRWorkflow {
	return o.prWorkflowMgr.Get(id)
}

// PRWorkflowManager returns the PR workflow manager for advanced operations.
func (o *Orchestrator) PRWorkflowManager() *prworkflow.Manager {
	return o.prWorkflowMgr
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
		if err := mgr.Stop(); err != nil {
			if o.logger != nil {
				o.logger.Warn("failed to stop instance during removal",
					"instance_id", inst.ID,
					"error", err,
				)
			}
		}
		o.displayMgr.RemoveObserver(mgr)
		delete(o.instances, inst.ID)
	}

	// Stop PR workflow if running (delegates to prWorkflowMgr)
	if err := o.prWorkflowMgr.Stop(inst.ID); err != nil {
		if o.logger != nil {
			o.logger.Warn("failed to stop PR workflow during instance removal",
				"instance_id", inst.ID,
				"error", err,
			)
		}
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

	// Stop namer service
	if o.namer != nil {
		o.namer.Stop()
	}

	// Stop all instances
	for _, inst := range sess.Instances {
		if mgr, ok := o.instances[inst.ID]; ok {
			if err := mgr.Stop(); err != nil {
				if o.logger != nil {
					o.logger.Warn("failed to stop instance during session stop",
						"instance_id", inst.ID,
						"error", err,
					)
				}
			}
		}
	}

	// Stop all PR workflows (delegates to prWorkflowMgr)
	o.prWorkflowMgr.StopAll()

	// Clean up worktrees if forced
	if force {
		for _, inst := range sess.Instances {
			if err := o.wt.Remove(inst.WorktreePath); err != nil {
				if o.logger != nil {
					o.logger.Warn("failed to remove worktree during session stop",
						"instance_id", inst.ID,
						"worktree_path", inst.WorktreePath,
						"error", err,
					)
				}
			}
		}
	}

	// Release session lock
	if o.lock != nil {
		if err := o.lock.Release(); err != nil {
			if o.logger != nil {
				o.logger.Warn("failed to release session lock",
					"error", err,
				)
			}
		}
		o.lock = nil
	}

	// Remove session file
	sessionFile := o.sessionFilePath()
	if err := os.Remove(sessionFile); err != nil && !os.IsNotExist(err) {
		if o.logger != nil {
			o.logger.Warn("failed to remove session file",
				"path", sessionFile,
				"error", err,
			)
		}
	}

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
// Uses the current display dimensions from displayMgr (falls back to defaults if not set)
func (o *Orchestrator) instanceManagerConfig() instance.ManagerConfig {
	width, height := o.displayMgr.GetDimensions()

	return instance.ManagerConfig{
		OutputBufferSize:         o.config.Instance.OutputBufferSize,
		CaptureIntervalMs:        o.config.Instance.CaptureIntervalMs,
		TmuxWidth:                width,
		TmuxHeight:               height,
		TmuxHistoryLimit:         o.config.Instance.TmuxHistoryLimit,
		ActivityTimeoutMinutes:   o.config.Instance.ActivityTimeoutMinutes,
		CompletionTimeoutMinutes: o.config.Instance.CompletionTimeoutMinutes,
		StaleDetection:           o.config.Instance.StaleDetection,
	}
}

// newInstanceManager creates a new instance manager with explicit dependencies.
// Uses the shared StateMonitor for centralized state tracking.
// The manager is automatically registered with the display manager to receive resize events.
//
// Note: LifecycleManager delegation is available but not enabled by default.
// The instance Manager's Start/Stop/Reconnect use their internal implementation.
func (o *Orchestrator) newInstanceManager(instanceID, workdir, task string) *instance.Manager {
	cfg := o.instanceManagerConfig()

	mgr := instance.NewManagerWithDeps(instance.ManagerOptions{
		ID:           instanceID,
		SessionID:    o.sessionID,
		WorkDir:      workdir,
		Task:         task,
		Config:       cfg,
		StateMonitor: o.stateMonitor,
		// LifecycleManager not set - instances use internal Start/Stop/Reconnect
	})

	// Register the manager as a resize observer so it receives dimension updates
	o.displayMgr.AddObserver(mgr)

	return mgr
}

// registerInstance performs common registration steps after an instance is created.
// This includes copying config files, registering with managers, and saving the session.
// Must be called while holding o.mu lock.
func (o *Orchestrator) registerInstance(session *Session, inst *Instance) error {
	// Copy local Claude configuration files (e.g., CLAUDE.local.md) to the worktree.
	// Failures are logged but do not block instance creation since local config is optional.
	o.copyLocalClaudeFilesToWorktree(inst.ID, inst.WorktreePath)

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := o.newInstanceManager(inst.ID, inst.WorktreePath, inst.Task)
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
		// Non-fatal, log but continue
		if o.logger != nil {
			o.logger.Warn("failed to update context",
				"instance_id", inst.ID,
				"error", err,
			)
		}
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
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// configureInstanceCallbacks sets up all necessary callbacks on an instance manager.
// This centralizes callback configuration to avoid duplication between StartInstance
// and ReconnectInstance.
func (o *Orchestrator) configureInstanceCallbacks(mgr *instance.Manager) {
	// Configure state change callback for notifications
	mgr.SetStateCallback(func(id string, state detect.WaitingState) {
		switch state {
		case detect.StateCompleted:
			o.handleInstanceExit(id)
		case detect.StateWaitingInput, detect.StateWaitingQuestion, detect.StateWaitingPermission:
			o.handleInstanceWaitingInput(id)
		case detect.StatePROpened:
			o.handleInstancePROpened(id)
		}
	})

	// Configure metrics callback for resource tracking
	mgr.SetMetricsCallback(func(id string, m *instmetrics.ParsedMetrics) {
		o.handleInstanceMetrics(id, m)
	})

	// Configure timeout callback
	mgr.SetTimeoutCallback(func(id string, timeoutType instance.TimeoutType) {
		o.handleInstanceTimeout(id, timeoutType)
	})

	// Configure bell callback to forward terminal bells
	mgr.SetBellCallback(func(id string) {
		o.handleInstanceBell(id)
	})
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
	o.displayMgr.SetDimensions(width, height)
}

// ResizeAllInstances resizes all running tmux sessions to the given dimensions
// and stores the dimensions for new instances
func (o *Orchestrator) ResizeAllInstances(width, height int) {
	// Use the displayMgr to notify all registered observers
	o.displayMgr.NotifyResize(width, height)
}

// DisplayManager returns the display manager for advanced display operations.
// Components that need to register for resize notifications can use this.
func (o *Orchestrator) DisplayManager() *display.Manager {
	return o.displayMgr
}

// wireStateMonitorCallbacks sets up callbacks from the centralized state monitor
// to route state changes, timeouts, and bells to the orchestrator's handlers.
func (o *Orchestrator) wireStateMonitorCallbacks() {
	// Wire state change callback - routes detected state changes to handlers
	o.stateMonitor.OnStateChange(func(instanceID string, oldState, newState detect.WaitingState) {
		switch newState {
		case detect.StateCompleted:
			o.handleInstanceExit(instanceID)
		case detect.StateWaitingInput, detect.StateWaitingQuestion, detect.StateWaitingPermission:
			o.handleInstanceWaitingInput(instanceID)
		case detect.StatePROpened:
			o.handleInstancePROpened(instanceID)
		}
	})

	// Wire timeout callback - converts state.TimeoutType to instance.TimeoutType
	o.stateMonitor.OnTimeout(func(instanceID string, timeoutType instancestate.TimeoutType) {
		// Convert state.TimeoutType to instance.TimeoutType
		var instTimeoutType instance.TimeoutType
		switch timeoutType {
		case instancestate.TimeoutActivity:
			instTimeoutType = instance.TimeoutActivity
		case instancestate.TimeoutCompletion:
			instTimeoutType = instance.TimeoutCompletion
		case instancestate.TimeoutStale:
			instTimeoutType = instance.TimeoutStale
		default:
			instTimeoutType = instance.TimeoutActivity
		}
		o.handleInstanceTimeout(instanceID, instTimeoutType)
	})

	// Wire bell callback - forwards bell events directly
	o.stateMonitor.OnBell(func(instanceID string) {
		o.handleInstanceBell(instanceID)
	})
}

// initBudgetManager creates and configures the budget manager.
// The orchestrator itself implements InstanceProvider and InstancePauser.
func (o *Orchestrator) initBudgetManager() {
	callbacks := budget.Callbacks{
		OnBudgetLimit: func() {
			o.executeNotification("notifications.on_budget_limit", nil)
		},
		OnBudgetWarning: func() {
			o.executeNotification("notifications.on_budget_warning", nil)
		},
	}

	o.budgetMgr = budget.NewManagerFromConfig(o.config, o, o, callbacks, o.logger)
}

// initNamer initializes the intelligent naming service.
// This is optional - requires both:
// 1. experimental.intelligent_naming config set to true
// 2. ANTHROPIC_API_KEY environment variable set
// If disabled or API key not set, instances use their original task as the display name.
func (o *Orchestrator) initNamer() {
	// Check if intelligent naming is enabled in config
	if o.config == nil || !o.config.Experimental.IntelligentNaming {
		if o.logger != nil {
			o.logger.Debug("intelligent naming disabled via config")
		}
		return
	}

	client, err := namer.NewAnthropicClient()
	if err != nil {
		// API key not set or other issue - namer won't be available
		// This is expected in many environments, so only log at debug level
		if o.logger != nil {
			o.logger.Debug("intelligent naming disabled", "reason", err.Error())
		}
		return
	}

	o.namer = namer.New(client, o.logger)
	o.namer.OnRename(o.handleInstanceRenamed)
	o.namer.Start()

	if o.logger != nil {
		o.logger.Debug("intelligent naming enabled")
	}
}

// handleInstanceRenamed is called when the namer generates a display name for an instance.
func (o *Orchestrator) handleInstanceRenamed(instanceID, newName string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.session == nil {
		return
	}

	inst := o.session.GetInstance(instanceID)
	if inst == nil {
		return
	}

	// Don't override manually named instances
	if inst.ManuallyNamed {
		if o.logger != nil {
			o.logger.Debug("skipping auto-rename for manually named instance",
				"instance_id", instanceID,
				"current_name", inst.DisplayName,
			)
		}
		return
	}

	inst.DisplayName = newName

	if o.logger != nil {
		o.logger.Debug("instance renamed",
			"instance_id", instanceID,
			"display_name", newName,
		)
	}

	// Persist the change
	if err := o.saveSession(); err != nil {
		if o.logger != nil {
			o.logger.Warn("failed to persist renamed instance - name may be lost on restart",
				"instance_id", instanceID,
				"display_name", newName,
				"error", err.Error(),
			)
		}
	}
}

// saveSession persists the session state to disk
func (o *Orchestrator) saveSession() error {
	if o.session == nil {
		return nil
	}

	sessionFile := o.sessionFilePath()
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

// sessionFilePath returns the path to the session.json file.
// Handles both multi-session mode (sessionDir) and legacy single-session mode (claudioDir).
func (o *Orchestrator) sessionFilePath() string {
	if o.sessionDir != "" {
		return filepath.Join(o.sessionDir, "session.json")
	}
	return filepath.Join(o.claudioDir, "session.json")
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

// copyLocalClaudeFilesToWorktree copies local Claude config files (e.g., CLAUDE.local.md)
// to the worktree. Errors are logged and reported to stderr but don't fail the operation.
func (o *Orchestrator) copyLocalClaudeFilesToWorktree(instID, wtPath string) {
	if err := o.wt.CopyLocalClaudeFiles(wtPath); err != nil {
		if o.logger != nil {
			o.logger.Warn("failed to copy local Claude files to worktree",
				"instance_id", instID,
				"worktree_path", wtPath,
				"error", err,
			)
		}
		fmt.Fprintf(os.Stderr, "Warning: failed to copy local Claude files to worktree: %v\n", err)
	}
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
		if err := o.saveSession(); err != nil {
			if o.logger != nil {
				o.logger.Error("failed to save session after instance exit",
					"instance_id", id,
					"error", err,
				)
			}
			fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
		}
		o.executeNotification("notifications.on_completion", inst)

		// Check for and start any dependent instances that are now ready
		o.startReadyDependents(id)
	}
}

// startReadyDependents checks all instances that depend on the completed instance
// and starts any that now have all dependencies met and are marked for auto-start.
func (o *Orchestrator) startReadyDependents(completedID string) {
	o.mu.RLock()
	session := o.session
	o.mu.RUnlock()

	if session == nil {
		if o.logger != nil {
			o.logger.Debug("skipping startReadyDependents: session is nil",
				"completed_id", completedID,
			)
		}
		return
	}

	// Get instances that depend on the completed instance
	dependents := session.GetDependentInstances(completedID)
	if len(dependents) == 0 {
		return
	}

	// Check each dependent to see if it's ready to start
	for _, dep := range dependents {
		// Only consider instances that are pending and have auto-start enabled
		if dep.Status != StatusPending || !dep.AutoStart {
			continue
		}

		// Check if all dependencies are now met
		if !session.AreDependenciesMet(dep) {
			continue
		}

		// Start the instance
		if o.logger != nil {
			o.logger.Info("auto-starting dependent instance",
				"instance_id", dep.ID,
				"completed_dependency", completedID,
			)
		}

		// Start asynchronously to avoid blocking
		go func(inst *Instance) {
			if err := o.StartInstance(inst); err != nil {
				if o.logger != nil {
					o.logger.Error("failed to auto-start dependent instance",
						"instance_id", inst.ID,
						"error", err,
					)
				}
				// Notify user via stderr so they know auto-start failed
				fmt.Fprintf(os.Stderr, "Error: failed to auto-start dependent instance %s: %v\n", inst.ID, err)
			}
		}(dep)
	}
}

// handleInstanceMetrics updates instance metrics when they change
func (o *Orchestrator) handleInstanceMetrics(id string, m *instmetrics.ParsedMetrics) {
	inst := o.GetInstance(id)
	if inst == nil || m == nil {
		return
	}

	// Update instance metrics
	if inst.Metrics == nil {
		inst.Metrics = &Metrics{}
	}

	inst.Metrics.InputTokens = m.InputTokens
	inst.Metrics.OutputTokens = m.OutputTokens
	inst.Metrics.CacheRead = m.CacheReadTokens
	inst.Metrics.CacheWrite = m.CacheWriteTokens
	inst.Metrics.APICalls = m.APICalls

	// Use parsed cost if available, otherwise calculate from tokens
	if m.Cost > 0 {
		inst.Metrics.Cost = m.Cost
	} else {
		inst.Metrics.Cost = instmetrics.CalculateCost(
			m.InputTokens,
			m.OutputTokens,
			m.CacheReadTokens,
			m.CacheWriteTokens,
		)
	}

	// Check budget limits
	o.checkBudgetLimits()

	// Save session periodically (not on every metric update to avoid excessive I/O)
	// The session will be saved when status changes occur
}

// checkBudgetLimits checks if any budget limits have been exceeded.
// Delegates to the budget manager for limit checking and enforcement.
func (o *Orchestrator) checkBudgetLimits() {
	if o.budgetMgr == nil || o.session == nil {
		return
	}
	o.budgetMgr.CheckLimits()
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

// GetSessionMetrics aggregates metrics across all instances in the session.
// Delegates to the budget manager for aggregation.
func (o *Orchestrator) GetSessionMetrics() *SessionMetrics {
	if o.budgetMgr == nil {
		return &SessionMetrics{}
	}

	bm := o.budgetMgr.GetSessionMetrics()
	return &SessionMetrics{
		TotalInputTokens:  bm.TotalInputTokens,
		TotalOutputTokens: bm.TotalOutputTokens,
		TotalCacheRead:    bm.TotalCacheRead,
		TotalCacheWrite:   bm.TotalCacheWrite,
		TotalCost:         bm.TotalCost,
		TotalAPICalls:     bm.TotalAPICalls,
		TotalDuration:     bm.TotalDuration,
		InstanceCount:     bm.InstanceCount,
		ActiveCount:       bm.ActiveCount,
	}
}

// GetInstanceMetrics returns the current metrics for a specific instance
func (o *Orchestrator) GetInstanceMetrics(id string) *Metrics {
	inst := o.GetInstance(id)
	if inst == nil {
		return nil
	}
	return inst.Metrics
}

// GetAllInstanceMetrics implements budget.InstanceProvider.
// Returns metrics for all instances in the current session.
func (o *Orchestrator) GetAllInstanceMetrics() []budget.InstanceMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if o.session == nil {
		return nil
	}

	result := make([]budget.InstanceMetrics, 0, len(o.session.Instances))
	for _, inst := range o.session.Instances {
		m := budget.InstanceMetrics{
			ID:     inst.ID,
			Status: string(inst.Status),
		}
		if inst.Metrics != nil {
			m.InputTokens = inst.Metrics.InputTokens
			m.OutputTokens = inst.Metrics.OutputTokens
			m.CacheRead = inst.Metrics.CacheRead
			m.CacheWrite = inst.Metrics.CacheWrite
			m.Cost = inst.Metrics.Cost
			m.APICalls = inst.Metrics.APICalls
			if inst.Metrics.StartTime != nil {
				m.StartTime = *inst.Metrics.StartTime
			}
			m.EndTime = inst.Metrics.EndTime
		}
		result = append(result, m)
	}
	return result
}

// PauseInstance implements budget.InstancePauser.
// Pauses the instance and updates its status.
func (o *Orchestrator) PauseInstance(id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	mgr, ok := o.instances[id]
	if !ok {
		return fmt.Errorf("instance %s not found", id)
	}

	if err := mgr.Pause(); err != nil {
		return err
	}

	// Update status in session
	for _, inst := range o.session.Instances {
		if inst.ID == id {
			inst.Status = StatusPaused
			break
		}
	}

	return nil
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

	// Publish event to event bus
	o.eventBus.Publish(event.NewPROpenedEvent(id, ""))

	// Notify via callback if set (for backwards compatibility)
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

	// Publish event to event bus (convert instance.TimeoutType to event.TimeoutType)
	eventTimeoutType := event.TimeoutActivity
	switch timeoutType {
	case instance.TimeoutActivity:
		eventTimeoutType = event.TimeoutActivity
	case instance.TimeoutCompletion:
		eventTimeoutType = event.TimeoutCompletion
	case instance.TimeoutStale:
		eventTimeoutType = event.TimeoutStale
	default:
		// Unknown timeout type - log warning and default to activity
		if o.logger != nil {
			o.logger.Warn("unknown timeout type during event conversion",
				"instance_id", id,
				"timeout_type", int(timeoutType),
			)
		}
	}
	o.eventBus.Publish(event.NewTimeoutEvent(id, eventTimeoutType, ""))

	// Notify via callback if set (for backwards compatibility)
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

	// Publish event to event bus
	o.eventBus.Publish(event.NewBellEvent(id))

	// Notify via callback if set (for backwards compatibility)
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

// ListBranches returns all local git branches, sorted with main/master first
func (o *Orchestrator) ListBranches() ([]worktree.BranchInfo, error) {
	return o.wt.ListBranches()
}

// GetMainBranch returns the name of the main branch (main or master)
func (o *Orchestrator) GetMainBranch() string {
	return o.wt.FindMainBranch()
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

	// Configure all callbacks
	o.configureInstanceCallbacks(mgr)

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

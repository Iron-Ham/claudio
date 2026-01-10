package orchestrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/instance"
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
	prCompleteCallback PRCompleteCallback

	// Callback for when a PR URL is detected in instance output (inline PR creation)
	prOpenedCallback PROpenedCallback

	// Callback for when an instance timeout is detected
	timeoutCallback TimeoutCallback

	// Callback for when a terminal bell is detected in an instance
	bellCallback BellCallback

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

// GetPRWorkflow returns the PR workflow for an instance, if any
func (o *Orchestrator) GetPRWorkflow(id string) *instance.PRWorkflow {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.prWorkflows[id]
}

// StopSession stops all instances and optionally cleans up
func (o *Orchestrator) StopSession(sess *Session, force bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

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

	return nil
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
		return err
	}

	// Write to each worktree
	for _, inst := range o.session.Instances {
		wtCtx := filepath.Join(inst.WorktreePath, ".claudio", "context.md")
		_ = os.MkdirAll(filepath.Dir(wtCtx), 0755)
		_ = os.WriteFile(wtCtx, []byte(ctx), 0644)
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

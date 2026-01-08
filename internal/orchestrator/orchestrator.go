package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/conflict"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/worktree"
	"github.com/spf13/viper"
)

// Orchestrator manages the Claudio session and coordinates instances
type Orchestrator struct {
	baseDir     string
	claudioDir  string
	worktreeDir string

	session          *Session
	instances        map[string]*instance.Manager
	wt               *worktree.Manager
	conflictDetector *conflict.Detector
	config           *config.Config

	// Current display dimensions for tmux sessions
	// These are updated when the TUI window resizes
	displayWidth  int
	displayHeight int

	mu sync.RWMutex
}

// New creates a new Orchestrator for the given repository
func New(baseDir string) (*Orchestrator, error) {
	return NewWithConfig(baseDir, config.Get())
}

// NewWithConfig creates a new Orchestrator with the given configuration
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

	return nil
}

// StartSession creates and starts a new session
func (o *Orchestrator) StartSession(name string) (*Session, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Ensure initialized
	if err := o.Init(); err != nil {
		return nil, err
	}

	// Create new session
	o.session = NewSession(name, o.baseDir)

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

	sessionFile := filepath.Join(o.claudioDir, "session.json")
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to parse session file: %w", err)
	}

	o.session = &session

	// Start conflict detector and register existing instances
	o.conflictDetector.Start()
	for _, inst := range session.Instances {
		o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath)
	}

	return o.session, nil
}

// AddInstance adds a new Claude instance to the session
func (o *Orchestrator) AddInstance(session *Session, task string) (*Instance, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Create instance
	inst := NewInstance(task)

	// Generate branch name from task
	branchSlug := slugify(task)
	inst.Branch = fmt.Sprintf("claudio/%s-%s", inst.ID, branchSlug)

	// Create worktree
	wtPath := filepath.Join(o.worktreeDir, inst.ID)
	if err := o.wt.Create(wtPath, inst.Branch); err != nil {
		return nil, fmt.Errorf("failed to create worktree: %w", err)
	}
	inst.WorktreePath = wtPath

	// Add to session
	session.Instances = append(session.Instances, inst)

	// Create instance manager with config
	mgr := instance.NewManagerWithConfig(inst.ID, inst.WorktreePath, task, o.instanceManagerConfig())
	o.instances[inst.ID] = mgr

	// Register with conflict detector
	if err := o.conflictDetector.AddInstance(inst.ID, inst.WorktreePath); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to watch instance for conflicts: %v\n", err)
	}

	// Update shared context
	if err := o.updateContext(); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "Warning: failed to update context: %v\n", err)
	}

	// Save session
	if err := o.saveSession(); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return inst, nil
}

// StartInstance starts a Claude process for an instance
func (o *Orchestrator) StartInstance(inst *Instance) error {
	o.mu.Lock()
	mgr, ok := o.instances[inst.ID]
	o.mu.Unlock()

	if !ok {
		mgr = instance.NewManagerWithConfig(inst.ID, inst.WorktreePath, inst.Task, o.instanceManagerConfig())
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
		}
	})

	if err := mgr.Start(); err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	inst.Status = StatusWorking
	inst.PID = mgr.PID()

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
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	inst.Status = StatusCompleted
	inst.PID = 0

	return o.saveSession()
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
		mgr.Stop()
		delete(o.instances, inst.ID)
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
func (o *Orchestrator) StopSession(session *Session, force bool) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Stop conflict detector
	if o.conflictDetector != nil {
		o.conflictDetector.Stop()
	}

	// Stop all instances
	for _, inst := range session.Instances {
		if mgr, ok := o.instances[inst.ID]; ok {
			mgr.Stop()
		}
	}

	// Clean up worktrees if forced
	if force {
		for _, inst := range session.Instances {
			o.wt.Remove(inst.WorktreePath)
		}
	}

	// Remove session file
	sessionFile := filepath.Join(o.claudioDir, "session.json")
	os.Remove(sessionFile)

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
		OutputBufferSize:  o.config.Instance.OutputBufferSize,
		CaptureIntervalMs: o.config.Instance.CaptureIntervalMs,
		TmuxWidth:         width,
		TmuxHeight:        height,
	}
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
			mgr.Resize(width, height)
		}
	}
}

// saveSession persists the session state to disk
func (o *Orchestrator) saveSession() error {
	if o.session == nil {
		return nil
	}

	sessionFile := filepath.Join(o.claudioDir, "session.json")
	data, err := json.MarshalIndent(o.session, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(sessionFile, data, 0644)
}

// updateContext updates the shared context file in all worktrees
func (o *Orchestrator) updateContext() error {
	if o.session == nil {
		return nil
	}

	ctx := o.generateContextMarkdown()

	// Write to main .claudio directory
	mainCtx := filepath.Join(o.claudioDir, "context.md")
	if err := os.WriteFile(mainCtx, []byte(ctx), 0644); err != nil {
		return err
	}

	// Write to each worktree
	for _, inst := range o.session.Instances {
		wtCtx := filepath.Join(inst.WorktreePath, ".claudio", "context.md")
		os.MkdirAll(filepath.Dir(wtCtx), 0755)
		os.WriteFile(wtCtx, []byte(ctx), 0644)
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

// handleInstanceExit handles when a Claude instance process exits
func (o *Orchestrator) handleInstanceExit(id string) {
	inst := o.GetInstance(id)
	if inst != nil {
		inst.Status = StatusCompleted
		inst.PID = 0
		o.saveSession()
		o.executeNotification("notifications.on_completion", inst)
	}
}

// handleInstanceWaitingInput handles when a Claude instance is waiting for input
func (o *Orchestrator) handleInstanceWaitingInput(id string) {
	inst := o.GetInstance(id)
	if inst != nil {
		inst.Status = StatusWaitingInput
		o.saveSession()
		o.executeNotification("notifications.on_waiting_input", inst)
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
		exec.Command("sh", "-c", cmd).Run()
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

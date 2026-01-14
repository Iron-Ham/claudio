package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// InstanceStatus represents the current state of a Claude instance
type InstanceStatus string

const (
	StatusPending      InstanceStatus = "pending"
	StatusWorking      InstanceStatus = "working"
	StatusWaitingInput InstanceStatus = "waiting_input"
	StatusPaused       InstanceStatus = "paused"
	StatusCompleted    InstanceStatus = "completed"
	StatusError        InstanceStatus = "error"
	StatusCreatingPR   InstanceStatus = "creating_pr"
	StatusStuck        InstanceStatus = "stuck"       // No activity for configured timeout period
	StatusTimeout      InstanceStatus = "timeout"     // Total runtime exceeded configured limit
	StatusInterrupted  InstanceStatus = "interrupted" // Claudio exited while instance was running
)

// Metrics tracks resource usage and costs for an instance
type Metrics struct {
	InputTokens  int64      `json:"input_tokens"`
	OutputTokens int64      `json:"output_tokens"`
	CacheRead    int64      `json:"cache_read,omitempty"`
	CacheWrite   int64      `json:"cache_write,omitempty"`
	Cost         float64    `json:"cost"`
	APICalls     int        `json:"api_calls"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
}

// TotalTokens returns the sum of input and output tokens
func (m *Metrics) TotalTokens() int64 {
	return m.InputTokens + m.OutputTokens
}

// Duration returns the total runtime duration if start/end times are set
func (m *Metrics) Duration() time.Duration {
	if m.StartTime == nil {
		return 0
	}
	if m.EndTime == nil {
		return time.Since(*m.StartTime)
	}
	return m.EndTime.Sub(*m.StartTime)
}

// Instance represents a single Claude Code instance
type Instance struct {
	ID            string         `json:"id"`
	WorktreePath  string         `json:"worktree_path"`
	Branch        string         `json:"branch"`
	Task          string         `json:"task"`
	Status        InstanceStatus `json:"status"`
	PID           int            `json:"pid,omitempty"`
	FilesModified []string       `json:"files_modified,omitempty"`
	Created       time.Time      `json:"created"`
	TmuxSession   string         `json:"tmux_session,omitempty"` // Tmux session name for recovery
	Metrics       *Metrics       `json:"metrics,omitempty"`
	Output        []byte         `json:"-"` // Not persisted, runtime only

	// Task chaining support - instances can declare dependencies on other instances
	// An instance with DependsOn will only auto-start when all dependencies complete
	DependsOn  []string `json:"depends_on,omitempty"` // Instance IDs this instance depends on
	Dependents []string `json:"dependents,omitempty"` // Instance IDs that depend on this instance
	AutoStart  bool     `json:"auto_start,omitempty"` // If true, auto-start when dependencies complete

	// Intelligent naming support - LLM-generated short names for sidebar display
	DisplayName   string `json:"display_name,omitempty"`   // Short descriptive name (e.g., "Fix auth bug")
	ManuallyNamed bool   `json:"manually_named,omitempty"` // If true, auto-rename is disabled

	// Progress persistence support - allows resuming interrupted Claude sessions
	ClaudeSessionID string     `json:"claude_session_id,omitempty"` // Claude's internal session UUID for --resume
	LastActiveAt    *time.Time `json:"last_active_at,omitempty"`    // Last time output was detected
	InterruptedAt   *time.Time `json:"interrupted_at,omitempty"`    // When session was interrupted (if applicable)
}

// GetID returns the instance ID (satisfies prworkflow.InstanceInfo).
func (i *Instance) GetID() string { return i.ID }

// GetWorktreePath returns the worktree path (satisfies prworkflow.InstanceInfo).
func (i *Instance) GetWorktreePath() string { return i.WorktreePath }

// GetBranch returns the branch name (satisfies prworkflow.InstanceInfo).
func (i *Instance) GetBranch() string { return i.Branch }

// GetTask returns the task description (satisfies prworkflow.InstanceInfo).
func (i *Instance) GetTask() string { return i.Task }

// EffectiveName returns the display name to show in the sidebar.
// Returns DisplayName if set, otherwise falls back to Task.
func (i *Instance) EffectiveName() string {
	if i.DisplayName != "" {
		return i.DisplayName
	}
	return i.Task
}

// RecoveryState indicates how a session was stopped
type RecoveryState string

const (
	// RecoveryNone means the session was cleanly stopped or is new
	RecoveryNone RecoveryState = ""
	// RecoveryInterrupted means the session was interrupted (Claudio exited while instances were running)
	RecoveryInterrupted RecoveryState = "interrupted"
	// RecoveryRecovered means the session was successfully recovered after an interruption
	RecoveryRecovered RecoveryState = "recovered"
)

// Session represents a Claudio work session
type Session struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	BaseRepo  string      `json:"base_repo"`
	Created   time.Time   `json:"created"`
	Instances []*Instance `json:"instances"`

	// Groups holds optional visual groupings of instances for the TUI.
	// When GroupedInstanceView is enabled, instances are organized into groups
	// rather than displayed as a flat list. Groups can have sub-groups for
	// representing nested dependencies (e.g., in Plan/UltraPlan workflows).
	// IMPORTANT: Always use thread-safe accessor methods (GetGroups, AddGroup, etc.)
	// instead of direct access to avoid race conditions with TUI rendering.
	Groups   []*InstanceGroup `json:"groups,omitempty"`
	groupsMu sync.RWMutex     `json:"-"` // Protects Groups slice for concurrent access

	// UltraPlan holds the ultra-plan session state (nil for regular sessions)
	UltraPlan *UltraPlanSession `json:"ultra_plan,omitempty"`

	// TripleShot holds a single triple-shot session for backward compatibility.
	// Deprecated: Use TripleShots slice for multiple concurrent tripleshots.
	TripleShot *TripleShotSession `json:"triple_shot,omitempty"`

	// TripleShots holds multiple concurrent triple-shot sessions.
	// Each tripleshot has its own group and coordinator.
	TripleShots []*TripleShotSession `json:"triple_shots,omitempty"`

	// Recovery state tracking - helps detect and recover interrupted sessions
	RecoveryState   RecoveryState `json:"recovery_state,omitempty"`   // Current recovery state
	LastActiveAt    *time.Time    `json:"last_active_at,omitempty"`   // Last time any instance had activity
	CleanShutdown   bool          `json:"clean_shutdown,omitempty"`   // True if session was cleanly stopped
	InterruptedAt   *time.Time    `json:"interrupted_at,omitempty"`   // When session was interrupted
	RecoveredAt     *time.Time    `json:"recovered_at,omitempty"`     // When session was last recovered
	RecoveryAttempt int           `json:"recovery_attempt,omitempty"` // Number of recovery attempts
}

// NewSession creates a new session with a generated ID
func NewSession(name, baseRepo string) *Session {
	if name == "" {
		name = "claudio-session"
	}

	return &Session{
		ID:        generateID(),
		Name:      name,
		BaseRepo:  baseRepo,
		Created:   time.Now(),
		Instances: make([]*Instance, 0),
	}
}

// NewInstance creates a new instance with a generated ID.
// Also generates a Claude session UUID for progress persistence and resume capability.
func NewInstance(task string) *Instance {
	return &Instance{
		ID:              generateID(),
		Task:            task,
		Status:          StatusPending,
		Created:         time.Now(),
		ClaudeSessionID: GenerateUUID(), // For resume capability
	}
}

// GetInstance returns an instance by ID
func (s *Session) GetInstance(id string) *Instance {
	for _, inst := range s.Instances {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}

// AreDependenciesMet checks if all dependencies of an instance have completed successfully
func (s *Session) AreDependenciesMet(inst *Instance) bool {
	if len(inst.DependsOn) == 0 {
		return true
	}

	for _, depID := range inst.DependsOn {
		dep := s.GetInstance(depID)
		if dep == nil {
			// Dependency doesn't exist - treat as not met
			return false
		}
		if dep.Status != StatusCompleted {
			return false
		}
	}
	return true
}

// GetDependentInstances returns all instances that depend on the given instance
func (s *Session) GetDependentInstances(id string) []*Instance {
	var dependents []*Instance
	for _, inst := range s.Instances {
		for _, depID := range inst.DependsOn {
			if depID == id {
				dependents = append(dependents, inst)
				break
			}
		}
	}
	return dependents
}

// GetReadyInstances returns all instances that have AutoStart=true,
// are in pending state, and have all dependencies met
func (s *Session) GetReadyInstances() []*Instance {
	var ready []*Instance
	for _, inst := range s.Instances {
		if inst.AutoStart && inst.Status == StatusPending && s.AreDependenciesMet(inst) {
			ready = append(ready, inst)
		}
	}
	return ready
}

// NeedsRecovery checks if the session was interrupted and needs recovery.
// A session needs recovery if it wasn't cleanly shutdown and had working instances.
func (s *Session) NeedsRecovery() bool {
	// If it was cleanly shutdown, no recovery needed
	if s.CleanShutdown {
		return false
	}

	// Check if any instances were in a working state
	for _, inst := range s.Instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			return true
		}
	}

	return false
}

// GetInterruptedInstances returns instances that were interrupted (working but tmux session gone).
func (s *Session) GetInterruptedInstances() []*Instance {
	var interrupted []*Instance
	for _, inst := range s.Instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			// Mark as interrupted if we're detecting a recovery scenario
			interrupted = append(interrupted, inst)
		}
	}
	return interrupted
}

// GetResumableInstances returns instances that have a Claude session ID and can be resumed.
func (s *Session) GetResumableInstances() []*Instance {
	var resumable []*Instance
	for _, inst := range s.Instances {
		// Instances with Claude session ID can be resumed if they were running or interrupted
		if inst.ClaudeSessionID != "" && (inst.Status == StatusWorking || inst.Status == StatusWaitingInput || inst.Status == StatusPaused || inst.Status == StatusInterrupted) {
			resumable = append(resumable, inst)
		}
	}
	return resumable
}

// MarkInstancesInterrupted marks all running instances as interrupted.
// This should be called when detecting that a session was not cleanly shutdown.
func (s *Session) MarkInstancesInterrupted() {
	now := time.Now()
	for _, inst := range s.Instances {
		if inst.Status == StatusWorking || inst.Status == StatusWaitingInput {
			inst.Status = StatusInterrupted
			inst.InterruptedAt = &now
		}
	}
	s.InterruptedAt = &now
	s.RecoveryState = RecoveryInterrupted
}

// MarkRecovered marks the session as having been recovered from an interruption.
func (s *Session) MarkRecovered() {
	now := time.Now()
	s.RecoveredAt = &now
	s.RecoveryAttempt++
	s.RecoveryState = RecoveryRecovered
	s.CleanShutdown = false // Will be set true on next clean shutdown
}

// generateID creates a short random hex ID
func generateID() string {
	bytes := make([]byte, 4)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateID generates a new random 8-character hex ID for sessions and instances.
// Exported for use by cmd package.
func GenerateID() string {
	return generateID()
}

// GenerateUUID generates a new random UUID (version 4) for Claude session IDs.
// This creates a UUID in the format xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
// where y is one of 8, 9, a, or b.
func GenerateUUID() string {
	uuid := make([]byte, 16)
	_, _ = rand.Read(uuid)

	// Set version (4) and variant (2) bits per RFC 4122
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)
}

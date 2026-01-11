package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
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
	StatusStuck        InstanceStatus = "stuck"   // No activity for configured timeout period
	StatusTimeout      InstanceStatus = "timeout" // Total runtime exceeded configured limit
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
}

// Session represents a Claudio work session
type Session struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	BaseRepo  string      `json:"base_repo"`
	Created   time.Time   `json:"created"`
	Instances []*Instance `json:"instances"`

	// UltraPlan holds the ultra-plan session state (nil for regular sessions)
	UltraPlan *UltraPlanSession `json:"ultra_plan,omitempty"`
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

// NewInstance creates a new instance with a generated ID
func NewInstance(task string) *Instance {
	return &Instance{
		ID:      generateID(),
		Task:    task,
		Status:  StatusPending,
		Created: time.Now(),
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

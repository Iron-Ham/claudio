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
)

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
	Output        []byte         `json:"-"`                      // Not persisted, runtime only
}

// Session represents a Claudio work session
type Session struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	BaseRepo  string      `json:"base_repo"`
	Created   time.Time   `json:"created"`
	Instances []*Instance `json:"instances"`
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

// generateID creates a short random hex ID
func generateID() string {
	bytes := make([]byte, 4)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

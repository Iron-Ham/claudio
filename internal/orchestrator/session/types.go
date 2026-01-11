package session

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// SessionData represents the serializable session state for persistence.
// This type is designed to be independent of the orchestrator's runtime Session type,
// allowing the session manager to operate without circular dependencies.
type SessionData struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	BaseRepo  string          `json:"base_repo"`
	Created   time.Time       `json:"created"`
	Instances []*InstanceData `json:"instances"`

	// UltraPlan holds the ultra-plan session state (nil for regular sessions)
	UltraPlan any `json:"ultra_plan,omitempty"`
}

// InstanceData represents instance information for persistence.
type InstanceData struct {
	ID            string       `json:"id"`
	WorktreePath  string       `json:"worktree_path"`
	Branch        string       `json:"branch"`
	Task          string       `json:"task"`
	Status        string       `json:"status"`
	PID           int          `json:"pid,omitempty"`
	FilesModified []string     `json:"files_modified,omitempty"`
	Created       time.Time    `json:"created"`
	TmuxSession   string       `json:"tmux_session,omitempty"`
	Metrics       *MetricsData `json:"metrics,omitempty"`
}

// MetricsData represents instance resource usage metrics for persistence.
type MetricsData struct {
	InputTokens  int64      `json:"input_tokens"`
	OutputTokens int64      `json:"output_tokens"`
	CacheRead    int64      `json:"cache_read,omitempty"`
	CacheWrite   int64      `json:"cache_write,omitempty"`
	Cost         float64    `json:"cost"`
	APICalls     int        `json:"api_calls"`
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
}

// NewSessionData creates a new SessionData with a generated ID.
func NewSessionData(name, baseRepo string) *SessionData {
	if name == "" {
		name = "claudio-session"
	}

	return &SessionData{
		ID:        generateID(),
		Name:      name,
		BaseRepo:  baseRepo,
		Created:   time.Now(),
		Instances: make([]*InstanceData, 0),
	}
}

// NewInstanceData creates a new InstanceData with a generated ID.
func NewInstanceData(task string) *InstanceData {
	return &InstanceData{
		ID:      generateID(),
		Task:    task,
		Status:  "pending",
		Created: time.Now(),
	}
}

// GetInstance returns an instance by ID, or nil if not found.
func (s *SessionData) GetInstance(id string) *InstanceData {
	for _, inst := range s.Instances {
		if inst.ID == id {
			return inst
		}
	}
	return nil
}

// generateID creates a short random hex ID.
func generateID() string {
	bytes := make([]byte, 4)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateID generates a new random 8-character hex ID.
// Exported for use by other packages that need to generate session/instance IDs.
func GenerateID() string {
	return generateID()
}

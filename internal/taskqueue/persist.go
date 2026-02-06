package taskqueue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const stateFileName = "taskqueue-state.json"

// persistedState is the serializable representation of the queue.
type persistedState struct {
	Tasks map[string]*QueuedTask `json:"tasks"`
	Order []string               `json:"order"`
}

// SaveState writes the queue state to a JSON file in the given directory.
// The write is atomic: data is written to a temporary file first, then
// renamed into place. A file lock is held during the operation for
// cross-process safety.
func (q *TaskQueue) SaveState(dir string) error {
	fl := NewFileLock(dir)
	if err := fl.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = fl.Unlock() }()

	q.mu.Lock()
	data, err := json.MarshalIndent(persistedState{
		Tasks: q.tasks,
		Order: q.order,
	}, "", "  ")
	q.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal queue state: %w", err)
	}

	target := filepath.Join(dir, stateFileName)
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp) // best-effort cleanup
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

// LoadState restores a TaskQueue from a previously saved state file
// in the given directory. A file lock is held during the read for
// cross-process safety.
func LoadState(dir string) (*TaskQueue, error) {
	fl := NewFileLock(dir)
	if err := fl.Lock(); err != nil {
		return nil, fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = fl.Unlock() }()

	target := filepath.Join(dir, stateFileName)

	data, err := os.ReadFile(target)
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal queue state: %w", err)
	}

	if state.Tasks == nil {
		state.Tasks = make(map[string]*QueuedTask)
	}
	if state.Order == nil {
		state.Order = []string{}
	}

	return newFromTasks(state.Tasks, state.Order), nil
}

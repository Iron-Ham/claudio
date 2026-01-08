package conflict

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileConflict represents a file being modified by multiple instances
type FileConflict struct {
	RelativePath string    // Path relative to worktree root
	Instances    []string  // Instance IDs that modified this file
	LastModified time.Time // When the conflict was last detected
}

// InstanceInfo holds info about a watched instance
type InstanceInfo struct {
	ID           string
	WorktreePath string
}

// Detector watches for file modifications across instances and detects conflicts
type Detector struct {
	watcher *fsnotify.Watcher

	// Map of instance ID -> worktree path
	instances map[string]string

	// Map of relative path -> set of instance IDs that modified it
	// relative path is normalized to be comparable across worktrees
	fileModifications map[string]map[string]time.Time

	// Current conflicts
	conflicts []FileConflict

	// Callback for conflict notifications
	onConflict func([]FileConflict)

	// Paths to ignore (e.g., .git, .claudio)
	ignorePaths []string

	mu     sync.RWMutex
	stopCh chan struct{}
}

// New creates a new conflict detector
func New() (*Detector, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Detector{
		watcher:           watcher,
		instances:         make(map[string]string),
		fileModifications: make(map[string]map[string]time.Time),
		conflicts:         make([]FileConflict, 0),
		ignorePaths:       []string{".git", ".claudio", "node_modules", ".DS_Store"},
		stopCh:            make(chan struct{}),
	}, nil
}

// SetConflictCallback sets the callback for when conflicts are detected
func (d *Detector) SetConflictCallback(cb func([]FileConflict)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onConflict = cb
}

// AddInstance starts watching files for an instance's worktree
func (d *Detector) AddInstance(instanceID, worktreePath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Store instance info
	d.instances[instanceID] = worktreePath

	// Add the worktree to the watcher
	// We watch the root directory - fsnotify will catch events in subdirectories
	if err := d.watcher.Add(worktreePath); err != nil {
		return err
	}

	// Also add subdirectories recursively for better coverage
	return d.watchDirRecursive(worktreePath)
}

// watchDirRecursive adds all subdirectories to the watcher
func (d *Detector) watchDirRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Check if this is a directory we should ignore
		base := filepath.Base(path)
		for _, ignore := range d.ignorePaths {
			if base == ignore {
				return filepath.SkipDir
			}
		}

		// We can only watch directories with fsnotify
		if info.IsDir() {
			_ = d.watcher.Add(path)
		}

		return nil
	})
}

// RemoveInstance stops watching an instance's worktree
func (d *Detector) RemoveInstance(instanceID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	worktreePath, ok := d.instances[instanceID]
	if !ok {
		return
	}

	// Remove from watcher
	_ = d.watcher.Remove(worktreePath)

	// Remove instance from tracking
	delete(d.instances, instanceID)

	// Clean up file modifications for this instance
	for relPath, instances := range d.fileModifications {
		delete(instances, instanceID)
		if len(instances) == 0 {
			delete(d.fileModifications, relPath)
		}
	}

	// Recalculate conflicts
	d.recalculateConflicts()
}

// Start begins watching for file changes
func (d *Detector) Start() {
	go d.watchLoop()
}

// Stop stops the detector and cleans up resources
func (d *Detector) Stop() {
	close(d.stopCh)
	_ = d.watcher.Close()
}

// watchLoop processes filesystem events
func (d *Detector) watchLoop() {
	// Debounce events - many editors create multiple events for a single save
	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C // drain initial timer

	pendingEvents := make(map[string]fsnotify.Event)
	var pendingMu sync.Mutex

	for {
		select {
		case <-d.stopCh:
			return

		case event, ok := <-d.watcher.Events:
			if !ok {
				return
			}

			// Only care about write/create operations
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce: collect events for a short period
			pendingMu.Lock()
			pendingEvents[event.Name] = event
			pendingMu.Unlock()

			// Reset debounce timer
			debounceTimer.Reset(50 * time.Millisecond)

		case <-debounceTimer.C:
			// Process all pending events
			pendingMu.Lock()
			events := pendingEvents
			pendingEvents = make(map[string]fsnotify.Event)
			pendingMu.Unlock()

			for _, event := range events {
				d.handleFileEvent(event)
			}

		case err, ok := <-d.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue
			_ = err
		}
	}
}

// handleFileEvent processes a single file modification event
func (d *Detector) handleFileEvent(event fsnotify.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := event.Name

	// Check if this path should be ignored
	for _, ignore := range d.ignorePaths {
		if strings.Contains(path, string(filepath.Separator)+ignore+string(filepath.Separator)) ||
			strings.HasSuffix(path, string(filepath.Separator)+ignore) ||
			filepath.Base(path) == ignore {
			return
		}
	}

	// Find which instance this file belongs to
	var matchedInstanceID string
	var relativePath string

	for instanceID, worktreePath := range d.instances {
		if strings.HasPrefix(path, worktreePath) {
			matchedInstanceID = instanceID
			relativePath, _ = filepath.Rel(worktreePath, path)
			break
		}
	}

	if matchedInstanceID == "" {
		return // Not in any watched worktree
	}

	// Track this modification
	if d.fileModifications[relativePath] == nil {
		d.fileModifications[relativePath] = make(map[string]time.Time)
	}
	d.fileModifications[relativePath][matchedInstanceID] = time.Now()

	// Check for conflicts
	d.recalculateConflicts()
}

// recalculateConflicts checks all tracked files for conflicts
func (d *Detector) recalculateConflicts() {
	conflicts := make([]FileConflict, 0)

	for relPath, instances := range d.fileModifications {
		if len(instances) > 1 {
			// Multiple instances modified the same file
			var instanceIDs []string
			var lastMod time.Time

			for id, modTime := range instances {
				instanceIDs = append(instanceIDs, id)
				if modTime.After(lastMod) {
					lastMod = modTime
				}
			}

			conflicts = append(conflicts, FileConflict{
				RelativePath: relPath,
				Instances:    instanceIDs,
				LastModified: lastMod,
			})
		}
	}

	d.conflicts = conflicts

	// Notify via callback
	if d.onConflict != nil && len(conflicts) > 0 {
		d.onConflict(conflicts)
	}
}

// GetConflicts returns the current list of conflicts
func (d *Detector) GetConflicts() []FileConflict {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make([]FileConflict, len(d.conflicts))
	copy(result, d.conflicts)
	return result
}

// GetFilesModifiedByInstance returns files modified by a specific instance
func (d *Detector) GetFilesModifiedByInstance(instanceID string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var files []string
	for relPath, instances := range d.fileModifications {
		if _, ok := instances[instanceID]; ok {
			files = append(files, relPath)
		}
	}
	return files
}

// ClearOldModifications removes modifications older than the given duration
func (d *Detector) ClearOldModifications(maxAge time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	for relPath, instances := range d.fileModifications {
		for instanceID, modTime := range instances {
			if modTime.Before(cutoff) {
				delete(instances, instanceID)
			}
		}
		if len(instances) == 0 {
			delete(d.fileModifications, relPath)
		}
	}

	d.recalculateConflicts()
}

// HasConflicts returns true if there are any active conflicts
func (d *Detector) HasConflicts() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.conflicts) > 0
}

// ConflictCount returns the number of files with conflicts
func (d *Detector) ConflictCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.conflicts)
}

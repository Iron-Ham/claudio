package conflict

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/worktree"
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

	// Logger for structured logging (optional)
	logger *logging.Logger

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

// SetLogger sets the logger for the conflict detector.
// If set, the detector will log conflict detection events at various levels.
func (d *Detector) SetLogger(logger *logging.Logger) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logger = logger
}

// AddInstance starts watching files for an instance's worktree.
// The root directory is watched immediately, but subdirectories are watched
// asynchronously in the background to avoid blocking instance creation.
// Returns an error if the worktree path does not exist or is not a directory.
func (d *Detector) AddInstance(instanceID, worktreePath string) error {
	// Validate the worktree path exists and is a directory before acquiring lock.
	// This provides a clearer error message than the one from fsnotify.Watcher.Add().
	info, err := os.Stat(worktreePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("worktree path does not exist: %q", worktreePath)
		}
		// Coverage: This branch handles permission denied and other stat errors which
		// are difficult to reliably test in a cross-platform manner.
		return fmt.Errorf("cannot access worktree path %q: %w", worktreePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("worktree path is not a directory: %q", worktreePath)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Store instance info
	d.instances[instanceID] = worktreePath

	// Add the worktree root to the watcher immediately.
	// fsnotify will catch events in subdirectories, but watching subdirectories
	// explicitly provides better coverage for some edge cases.
	if err := d.watcher.Add(worktreePath); err != nil {
		return err
	}

	// Log instance registration at INFO level
	if d.logger != nil {
		d.logger.Info("instance registered for conflict detection",
			"instance_id", instanceID,
			"path", worktreePath,
		)
	}

	// Add subdirectories asynchronously to avoid blocking instance creation.
	// This is safe because:
	// 1. The root directory is already being watched (catches top-level file changes)
	// 2. Subdirectory watching completes quickly (typically < 500ms)
	// 3. Conflict detection is non-critical - missing the first few events is acceptable
	go func() {
		if err := d.watchDirRecursive(worktreePath); err != nil {
			d.mu.RLock()
			logger := d.logger
			d.mu.RUnlock()
			if logger != nil {
				logger.Debug("failed to watch subdirectories",
					"instance_id", instanceID,
					"path", worktreePath,
					"error", err.Error(),
				)
			}
		}
	}()

	return nil
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

		// Skip submodule directories - they have their own .git reference
		// and watching inside them can cause errors or duplicate events
		if info.IsDir() && path != root {
			if worktree.IsSubmoduleDir(path) {
				if d.logger != nil {
					d.logger.Debug("skipping submodule directory",
						"path", path,
					)
				}
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

			// Log file watch event at DEBUG level
			d.mu.RLock()
			logger := d.logger
			d.mu.RUnlock()
			if logger != nil {
				logger.Debug("file watch event received",
					"path", event.Name,
					"operation", event.Op.String(),
				)
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
			// Log watcher error at DEBUG level (errors are typically transient)
			d.mu.RLock()
			logger := d.logger
			d.mu.RUnlock()
			if logger != nil {
				logger.Debug("file watcher error",
					"error", err.Error(),
				)
			}
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

	// Skip paths inside submodules by checking for .git file in parent directories
	if isInsideSubmodule(path) {
		return
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

	// Log file modification tracking at DEBUG level
	if d.logger != nil {
		d.logger.Debug("file modification tracked",
			"file_path", relativePath,
			"instance_id", matchedInstanceID,
		)
	}

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

			conflict := FileConflict{
				RelativePath: relPath,
				Instances:    instanceIDs,
				LastModified: lastMod,
			}
			conflicts = append(conflicts, conflict)

			// Log conflict at INFO level
			if d.logger != nil {
				d.logger.Info("file conflict detected",
					"file_path", relPath,
					"instance_ids", instanceIDs,
				)
			}
		}
	}

	// Log warning if we have potential conflicts (this method is called frequently)
	if d.logger != nil && len(conflicts) > 0 {
		d.logger.Warn("potential conflicts detected",
			"conflict_count", len(conflicts),
		)
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

// isInsideSubmodule checks if a file path is inside a git submodule.
// It walks up the directory tree looking for a .git file (not directory)
// that indicates a submodule root.
func isInsideSubmodule(path string) bool {
	// Start from the directory containing the file
	dir := filepath.Dir(path)

	// Walk up the tree until we find either:
	// 1. A .git file (submodule) - return true
	// 2. A .git directory (normal repo root) - return false
	// 3. Root of filesystem - return false
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.Mode().IsRegular() {
				// Found a .git file - this is a submodule
				return worktree.IsSubmoduleDir(dir)
			}
			// Found a .git directory - this is a normal repo root
			return false
		}
		// Only continue if the error is "file not found" - for other errors
		// (permission denied, etc.), we can't determine submodule status
		// so assume it's not a submodule to be safe
		if !os.IsNotExist(err) {
			return false
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root without finding .git
			return false
		}
		dir = parent
	}
}

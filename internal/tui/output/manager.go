// Package output provides output buffer management for the TUI.
//
// OutputManager handles per-instance output storage, scrolling state, and auto-scroll
// behavior. It extracts output management responsibilities from the TUI Model to
// provide a focused, testable component.
package output

import (
	"maps"
	"strings"
	"sync"
)

// cacheEntry holds a cached filtered output and its invalidation metadata.
type cacheEntry struct {
	filtered      string
	lineCount     int
	lines         []string
	outputVersion uint64
	filterVersion uint64
}

// Manager handles output buffer management for TUI instances.
// It maintains output buffers per instance with scrolling state and
// auto-scroll behavior for following new output.
type Manager struct {
	mu sync.RWMutex

	// outputs stores the output content per instance ID
	outputs map[string]string

	// scrollOffsets stores the scroll position (line number) per instance
	scrollOffsets map[string]int

	// autoScroll tracks whether auto-scroll is enabled per instance
	// When true, new output automatically scrolls to bottom
	autoScroll map[string]bool

	// hasNewOutput tracks whether new output arrived while auto-scroll was disabled
	hasNewOutput map[string]bool

	// filterFunc is an optional function to apply when counting visible lines
	// This allows the manager to account for filtered output when calculating scroll
	filterFunc func(output string) string

	// filteredCache stores cached filtered output per instance ID with version metadata.
	// Cache entries are invalidated when raw output changes or filter settings change.
	filteredCache map[string]cacheEntry

	// outputVersions tracks the "version" of each instance's output for cache invalidation.
	// Incremented each time SetOutput/AddOutput is called with changed content.
	outputVersions map[string]uint64

	// filterVersion is incremented when filter settings change, invalidating all caches
	filterVersion uint64
}

// NewManager creates a new output Manager with initialized maps.
func NewManager() *Manager {
	return &Manager{
		outputs:        make(map[string]string),
		scrollOffsets:  make(map[string]int),
		autoScroll:     make(map[string]bool),
		hasNewOutput:   make(map[string]bool),
		filteredCache:  make(map[string]cacheEntry),
		outputVersions: make(map[string]uint64),
	}
}

// SetFilterFunc sets the filter function used when calculating visible line counts.
// This should be set to match the filter applied during rendering.
func (m *Manager) SetFilterFunc(f func(output string) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filterFunc = f
	// Changing the filter function invalidates all cached filtered outputs/lines.
	m.filterVersion++
	m.filteredCache = make(map[string]cacheEntry)
}

// InvalidateFilterCache marks all cached filtered outputs as stale.
// Call this when filter settings (categories, custom pattern, etc.) change.
func (m *Manager) InvalidateFilterCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filterVersion++
	// Drop cached filtered outputs/lines to avoid holding stale buffers.
	m.filteredCache = make(map[string]cacheEntry)
}

// AddOutput appends output to an instance's buffer.
// This is typically used for incremental output updates.
func (m *Manager) AddOutput(instanceID string, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if output != "" {
		m.outputs[instanceID] += output
		m.outputVersions[instanceID]++
		delete(m.filteredCache, instanceID)
		if !m.isAutoScrollLocked(instanceID) {
			m.hasNewOutput[instanceID] = true
		}
	}
}

// SetOutput replaces the entire output buffer for an instance.
// This is used when fetching the complete output from a ring buffer.
func (m *Manager) SetOutput(instanceID string, output string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Only increment version if output actually changed
	if m.outputs[instanceID] != output {
		m.outputs[instanceID] = output
		m.outputVersions[instanceID]++
		delete(m.filteredCache, instanceID)
		if !m.isAutoScrollLocked(instanceID) {
			m.hasNewOutput[instanceID] = true
		}
		return true
	}
	return false
}

// GetOutput returns the output for an instance.
// Returns empty string if the instance has no output.
func (m *Manager) GetOutput(instanceID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.outputs[instanceID]
}

// Clear removes all output for an instance and resets its scroll state.
func (m *Manager) Clear(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.outputs, instanceID)
	delete(m.scrollOffsets, instanceID)
	delete(m.autoScroll, instanceID)
	delete(m.hasNewOutput, instanceID)
	delete(m.filteredCache, instanceID)
	delete(m.outputVersions, instanceID)
}

// ClearAll removes all output and state for all instances.
func (m *Manager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.outputs = make(map[string]string)
	m.scrollOffsets = make(map[string]int)
	m.autoScroll = make(map[string]bool)
	m.hasNewOutput = make(map[string]bool)
	m.filteredCache = make(map[string]cacheEntry)
	m.outputVersions = make(map[string]uint64)
}

// Scroll adjusts the scroll position by delta lines.
// Positive delta scrolls down, negative scrolls up.
// The scroll position is clamped to valid bounds.
// Scrolling up disables auto-scroll; scrolling to bottom re-enables it.
func (m *Manager) Scroll(instanceID string, delta int, maxVisibleLines int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentScroll := m.scrollOffsets[instanceID]
	newScroll := currentScroll + delta

	// Clamp to valid range
	maxScroll := m.getMaxScrollLocked(instanceID, maxVisibleLines)
	newScroll = max(0, min(newScroll, maxScroll))

	m.scrollOffsets[instanceID] = newScroll

	// Scrolling up disables auto-scroll
	if delta < 0 {
		m.autoScroll[instanceID] = false
	}

	// If at bottom, re-enable auto-scroll
	if newScroll >= maxScroll {
		m.autoScroll[instanceID] = true
		m.hasNewOutput[instanceID] = false
	}
}

// ScrollToTop scrolls to the top and disables auto-scroll.
func (m *Manager) ScrollToTop(instanceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scrollOffsets[instanceID] = 0
	m.autoScroll[instanceID] = false
}

// ScrollToBottom scrolls to the bottom and re-enables auto-scroll.
func (m *Manager) ScrollToBottom(instanceID string, maxVisibleLines int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scrollOffsets[instanceID] = m.getMaxScrollLocked(instanceID, maxVisibleLines)
	m.autoScroll[instanceID] = true
	m.hasNewOutput[instanceID] = false
}

// GetScrollOffset returns the current scroll offset for an instance.
func (m *Manager) GetScrollOffset(instanceID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scrollOffsets[instanceID]
}

// IsAutoScroll returns whether auto-scroll is enabled for an instance.
// Defaults to true for instances that haven't been explicitly set.
func (m *Manager) IsAutoScroll(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if autoScroll, exists := m.autoScroll[instanceID]; exists {
		return autoScroll
	}
	return true // Default to auto-scroll enabled
}

// SetAutoScroll explicitly sets the auto-scroll state for an instance.
func (m *Manager) SetAutoScroll(instanceID string, enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.autoScroll[instanceID] = enabled
}

// UpdateScroll updates scroll position based on new output (if auto-scroll is enabled).
// This should be called after output is updated to maintain scroll behavior.
func (m *Manager) UpdateScroll(instanceID string, maxVisibleLines int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clamp scroll offset to valid range in case output shrank due to filtering.
	maxScroll := m.getMaxScrollLocked(instanceID, maxVisibleLines)
	if m.scrollOffsets[instanceID] > maxScroll {
		m.scrollOffsets[instanceID] = maxScroll
	}

	if m.isAutoScrollLocked(instanceID) {
		m.scrollOffsets[instanceID] = maxScroll
		m.hasNewOutput[instanceID] = false
	}
}

// HasNewOutput returns true if there's new output since the last UpdateScroll call.
func (m *Manager) HasNewOutput(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.hasNewOutput[instanceID]
}

// GetLineCount returns the total number of lines in the output for an instance.
// If a filter function is set, it counts lines after filtering.
func (m *Manager) GetLineCount(instanceID string) int {
	return m.getFilteredEntry(instanceID).lineCount
}

// GetMaxScroll returns the maximum scroll offset for an instance.
func (m *Manager) GetMaxScroll(instanceID string, maxVisibleLines int) int {
	return max(0, m.GetLineCount(instanceID)-maxVisibleLines)
}

// getLineCountLocked returns line count for the current output/filter state.
// Caller must hold a write lock.
func (m *Manager) getLineCountLocked(instanceID string) int {
	return m.computeFilteredEntryLocked(instanceID).lineCount
}

// getMaxScrollLocked returns max scroll offset (caller must hold lock).
func (m *Manager) getMaxScrollLocked(instanceID string, maxVisibleLines int) int {
	totalLines := m.getLineCountLocked(instanceID)
	return max(0, totalLines-maxVisibleLines)
}

// isAutoScrollLocked returns auto-scroll state (caller must hold lock).
func (m *Manager) isAutoScrollLocked(instanceID string) bool {
	if autoScroll, exists := m.autoScroll[instanceID]; exists {
		return autoScroll
	}
	return true
}

// InstanceIDs returns a slice of all instance IDs that have output.
func (m *Manager) InstanceIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.outputs))
	for id := range m.outputs {
		ids = append(ids, id)
	}
	return ids
}

// HasOutput returns true if the instance has any output stored.
func (m *Manager) HasOutput(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.outputs[instanceID]
	return exists
}

// GetAllOutputs returns a snapshot of all outputs as a map.
// This is useful for backward compatibility with code that expects a map.
func (m *Manager) GetAllOutputs() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]string, len(m.outputs))
	maps.Copy(result, m.outputs)
	return result
}

// getFilteredEntry returns the cached filtered entry, recomputing it if necessary.
// This method is safe for concurrent use.
func (m *Manager) getFilteredEntry(instanceID string) cacheEntry {
	// Fast path: read lock and cache hit
	m.mu.RLock()
	output := m.outputs[instanceID]
	if output == "" {
		m.mu.RUnlock()
		return cacheEntry{}
	}
	outputVersion := m.outputVersions[instanceID]
	filterVersion := m.filterVersion
	entry, ok := m.filteredCache[instanceID]
	if ok && entry.outputVersion == outputVersion && entry.filterVersion == filterVersion {
		m.mu.RUnlock()
		return entry
	}
	m.mu.RUnlock()

	// Slow path: compute under write lock
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.computeFilteredEntryLocked(instanceID)
}

// computeFilteredEntryLocked recomputes and stores the filtered cache entry if stale.
// Caller must hold a write lock.
func (m *Manager) computeFilteredEntryLocked(instanceID string) cacheEntry {
	output := m.outputs[instanceID]
	if output == "" {
		delete(m.filteredCache, instanceID)
		return cacheEntry{}
	}

	outputVersion := m.outputVersions[instanceID]
	filterVersion := m.filterVersion
	if entry, ok := m.filteredCache[instanceID]; ok && entry.outputVersion == outputVersion && entry.filterVersion == filterVersion {
		return entry
	}

	filtered := output
	if m.filterFunc != nil {
		filtered = m.filterFunc(output)
	}

	entry := cacheEntry{
		filtered:      filtered,
		lineCount:     countLines(filtered),
		outputVersion: outputVersion,
		filterVersion: filterVersion,
	}
	m.filteredCache[instanceID] = entry
	return entry
}

// GetFilteredOutput returns the output after applying the filter function.
// If no filter is set, returns the raw output.
// Results are cached and only recomputed when the output or filter settings change.
func (m *Manager) GetFilteredOutput(instanceID string) string {
	return m.getFilteredEntry(instanceID).filtered
}

// GetFilteredLines returns the filtered output split into lines.
// The returned slice is owned by the manager; callers must treat it as immutable.
func (m *Manager) GetFilteredLines(instanceID string) []string {
	entry := m.getFilteredEntry(instanceID)
	if entry.filtered == "" {
		return nil
	}
	if entry.lines != nil {
		return entry.lines
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	entry = m.computeFilteredEntryLocked(instanceID)
	if entry.filtered == "" {
		return nil
	}
	if entry.lines != nil {
		return entry.lines
	}

	entry.lines = splitLines(entry.filtered)
	m.filteredCache[instanceID] = entry
	return entry.lines
}

// GetVisibleLines returns the lines that should be visible based on scroll offset.
// This is a convenience method for rendering.
func (m *Manager) GetVisibleLines(instanceID string, maxVisibleLines int) []string {
	lines := m.GetFilteredLines(instanceID)
	if len(lines) == 0 {
		return nil
	}

	m.mu.RLock()
	scrollOffset := m.scrollOffsets[instanceID]
	m.mu.RUnlock()

	// Clamp scroll offset
	scrollOffset = max(0, min(scrollOffset, max(0, len(lines)-1)))

	// Calculate visible range
	endLine := min(scrollOffset+maxVisibleLines, len(lines))

	return lines[scrollOffset:endLine]
}

func countLines(s string) int {
	if s == "" {
		return 0
	}

	count := 1
	for _, c := range s {
		if c == '\n' {
			count++
		}
	}
	return count
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

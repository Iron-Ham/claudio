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

	// lineCountCache stores previous line counts for detecting new output
	lineCountCache map[string]int

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
		lineCountCache: make(map[string]int),
		filteredCache:  make(map[string]cacheEntry),
		outputVersions: make(map[string]uint64),
	}
}

// SetFilterFunc sets the filter function used when calculating visible line counts.
// This should be set to match the filter applied during rendering.
// Note: This does NOT invalidate the cache. Call InvalidateFilterCache() when
// the filter settings change to force recomputation.
func (m *Manager) SetFilterFunc(f func(output string) string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filterFunc = f
}

// InvalidateFilterCache marks all cached filtered outputs as stale.
// Call this when filter settings (categories, custom pattern, etc.) change.
func (m *Manager) InvalidateFilterCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filterVersion++
}

// AddOutput appends output to an instance's buffer.
// This is typically used for incremental output updates.
func (m *Manager) AddOutput(instanceID string, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if output != "" {
		m.outputs[instanceID] += output
		m.outputVersions[instanceID]++
	}
}

// SetOutput replaces the entire output buffer for an instance.
// This is used when fetching the complete output from a ring buffer.
func (m *Manager) SetOutput(instanceID string, output string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Only increment version if output actually changed
	if m.outputs[instanceID] != output {
		m.outputs[instanceID] = output
		m.outputVersions[instanceID]++
	}
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
	delete(m.lineCountCache, instanceID)
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
	m.lineCountCache = make(map[string]int)
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

	if m.isAutoScrollLocked(instanceID) {
		m.scrollOffsets[instanceID] = m.getMaxScrollLocked(instanceID, maxVisibleLines)
	}

	// Update line count cache for detecting new output
	m.lineCountCache[instanceID] = m.getLineCountLocked(instanceID)
}

// HasNewOutput returns true if there's new output since the last UpdateScroll call.
func (m *Manager) HasNewOutput(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	currentLines := m.getLineCountLocked(instanceID)
	previousLines, exists := m.lineCountCache[instanceID]
	if !exists {
		return false
	}
	return currentLines > previousLines
}

// GetLineCount returns the total number of lines in the output for an instance.
// If a filter function is set, it counts lines after filtering.
func (m *Manager) GetLineCount(instanceID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getLineCountLocked(instanceID)
}

// GetMaxScroll returns the maximum scroll offset for an instance.
func (m *Manager) GetMaxScroll(instanceID string, maxVisibleLines int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getMaxScrollLocked(instanceID, maxVisibleLines)
}

// getLineCountLocked returns line count (caller must hold lock).
func (m *Manager) getLineCountLocked(instanceID string) int {
	output := m.outputs[instanceID]
	if output == "" {
		return 0
	}

	// Apply filter if set
	if m.filterFunc != nil {
		output = m.filterFunc(output)
	}
	if output == "" {
		return 0
	}

	// Count newlines + 1 for last line
	count := 1
	for _, c := range output {
		if c == '\n' {
			count++
		}
	}
	return count
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

// GetFilteredOutput returns the output after applying the filter function.
// If no filter is set, returns the raw output.
// Results are cached and only recomputed when the output or filter settings change.
func (m *Manager) GetFilteredOutput(instanceID string) string {
	// First try read-only path to check cache
	m.mu.RLock()
	output := m.outputs[instanceID]
	if output == "" || m.filterFunc == nil {
		m.mu.RUnlock()
		return output
	}

	// Check if cache is valid
	outputVersion := m.outputVersions[instanceID]
	entry, ok := m.filteredCache[instanceID]
	if ok && entry.outputVersion == outputVersion && entry.filterVersion == m.filterVersion {
		m.mu.RUnlock()
		return entry.filtered
	}
	m.mu.RUnlock()

	// Cache miss - need to recompute with write lock
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	output = m.outputs[instanceID]
	if output == "" || m.filterFunc == nil {
		return output
	}

	// Check cache again (another goroutine may have updated it)
	outputVersion = m.outputVersions[instanceID]
	if entry, ok := m.filteredCache[instanceID]; ok && entry.outputVersion == outputVersion && entry.filterVersion == m.filterVersion {
		return entry.filtered
	}

	// Compute and cache
	filtered := m.filterFunc(output)
	m.filteredCache[instanceID] = cacheEntry{
		filtered:      filtered,
		outputVersion: outputVersion,
		filterVersion: m.filterVersion,
	}

	return filtered
}

// GetVisibleLines returns the lines that should be visible based on scroll offset.
// This is a convenience method for rendering.
func (m *Manager) GetVisibleLines(instanceID string, maxVisibleLines int) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	output := m.outputs[instanceID]
	if output == "" {
		return nil
	}

	// Apply filter if set
	if m.filterFunc != nil {
		output = m.filterFunc(output)
	}
	if output == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	scrollOffset := m.scrollOffsets[instanceID]

	// Clamp scroll offset
	scrollOffset = max(0, min(scrollOffset, max(0, len(lines)-1)))

	// Calculate visible range
	endLine := min(scrollOffset+maxVisibleLines, len(lines))

	return lines[scrollOffset:endLine]
}

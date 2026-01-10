// Package ultraplan provides UI components for ultra-plan mode orchestration.
package ultraplan

import (
	"slices"
	"strings"
	"sync"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// NavigationCategory represents the type/category of a navigable instance.
// This helps with phase-aware navigation and filtering.
type NavigationCategory int

const (
	CategoryPlanning NavigationCategory = iota
	CategoryPlanSelection
	CategoryExecution
	CategoryGroupConsolidation
	CategorySynthesis
	CategoryRevision
	CategoryConsolidation
)

// NavigableInstance represents an instance that can be navigated to,
// with additional metadata for display and filtering.
type NavigableInstance struct {
	ID       string
	Category NavigationCategory
	Label    string // Human-readable label for the instance
	TaskID   string // For execution tasks, the associated task ID
	Index    int    // Position within its category (e.g., task index, group index)
}

// InstanceProvider is an interface for looking up instances by ID.
// This allows the navigator to check instance status without coupling to
// the orchestrator implementation.
type InstanceProvider interface {
	GetInstance(id string) *orchestrator.Instance
}

// SessionProvider provides access to the current UltraPlan session state.
type SessionProvider interface {
	Session() *orchestrator.UltraPlanSession
}

// PhaseAwareNavigator manages navigation state for ultra-plan mode.
// It tracks the current selection, maintains an ordered list of navigable
// instances, and handles phase-specific navigation rules.
type PhaseAwareNavigator struct {
	mu sync.RWMutex

	// Navigation state
	navigableInstances []NavigableInstance
	selectedIndex      int // Index into navigableInstances

	// Scroll offsets per category (for UI rendering)
	scrollOffsets map[NavigationCategory]int

	// Dependencies (injected)
	instanceProvider InstanceProvider
	sessionProvider  SessionProvider
}

// NewPhaseAwareNavigator creates a new navigator with the given providers.
func NewPhaseAwareNavigator(instanceProvider InstanceProvider, sessionProvider SessionProvider) *PhaseAwareNavigator {
	return &PhaseAwareNavigator{
		navigableInstances: make([]NavigableInstance, 0),
		selectedIndex:      0,
		scrollOffsets:      make(map[NavigationCategory]int),
		instanceProvider:   instanceProvider,
		sessionProvider:    sessionProvider,
	}
}

// Update refreshes the list of navigable instances based on current session state.
// Call this whenever the session state changes (new instances, phase transitions).
func (n *PhaseAwareNavigator) Update() {
	n.mu.Lock()
	defer n.mu.Unlock()

	session := n.sessionProvider.Session()
	if session == nil {
		n.navigableInstances = nil
		n.selectedIndex = 0
		return
	}

	// Preserve current selection if possible
	var currentID string
	if n.selectedIndex >= 0 && n.selectedIndex < len(n.navigableInstances) {
		currentID = n.navigableInstances[n.selectedIndex].ID
	}

	// Build new navigable instances list
	n.navigableInstances = n.buildNavigableInstances(session)

	// Try to restore selection
	n.selectedIndex = 0
	if currentID != "" {
		for i, inst := range n.navigableInstances {
			if inst.ID == currentID {
				n.selectedIndex = i
				break
			}
		}
	}
}

// buildNavigableInstances constructs the ordered list of navigable instances.
// Order: Planning → Plan Selection → Execution tasks → Group Consolidators → Synthesis → Revision → Consolidation
func (n *PhaseAwareNavigator) buildNavigableInstances(session *orchestrator.UltraPlanSession) []NavigableInstance {
	var instances []NavigableInstance

	// Planning coordinator (single-pass mode)
	if session.CoordinatorID != "" {
		if inst := n.instanceProvider.GetInstance(session.CoordinatorID); inst != nil {
			if inst.Status != orchestrator.StatusPending {
				instances = append(instances, NavigableInstance{
					ID:       session.CoordinatorID,
					Category: CategoryPlanning,
					Label:    "Planning Coordinator",
					Index:    0,
				})
			}
		}
	}

	// Multi-pass plan coordinators
	for i, coordID := range session.PlanCoordinatorIDs {
		if coordID == "" {
			continue
		}
		if inst := n.instanceProvider.GetInstance(coordID); inst != nil {
			if inst.Status != orchestrator.StatusPending {
				strategyNames := orchestrator.GetMultiPassStrategyNames()
				label := "Plan Coordinator"
				if i < len(strategyNames) {
					label = strategyNames[i] + " Planning"
				}
				instances = append(instances, NavigableInstance{
					ID:       coordID,
					Category: CategoryPlanSelection,
					Label:    label,
					Index:    i,
				})
			}
		}
	}

	// Plan manager (multi-pass mode)
	if session.PlanManagerID != "" {
		if inst := n.instanceProvider.GetInstance(session.PlanManagerID); inst != nil {
			if inst.Status != orchestrator.StatusPending {
				instances = append(instances, NavigableInstance{
					ID:       session.PlanManagerID,
					Category: CategoryPlanSelection,
					Label:    "Plan Manager",
					Index:    len(session.PlanCoordinatorIDs),
				})
			}
		}
	}

	// Execution tasks (in execution order)
	if session.Plan != nil {
		taskIndex := 0
		for groupIdx, group := range session.Plan.ExecutionOrder {
			for _, taskID := range group {
				if instID := n.findTaskInstance(session, taskID); instID != "" {
					task := session.GetTask(taskID)
					label := taskID
					if task != nil {
						label = task.Title
					}
					instances = append(instances, NavigableInstance{
						ID:       instID,
						Category: CategoryExecution,
						Label:    label,
						TaskID:   taskID,
						Index:    taskIndex,
					})
					taskIndex++
				}
			}

			// Add group consolidator if present
			if groupIdx < len(session.GroupConsolidatorIDs) && session.GroupConsolidatorIDs[groupIdx] != "" {
				instances = append(instances, NavigableInstance{
					ID:       session.GroupConsolidatorIDs[groupIdx],
					Category: CategoryGroupConsolidation,
					Label:    "Group Consolidator",
					Index:    groupIdx,
				})
			}
		}
	}

	// Synthesis
	if session.SynthesisID != "" {
		instances = append(instances, NavigableInstance{
			ID:       session.SynthesisID,
			Category: CategorySynthesis,
			Label:    "Synthesis Reviewer",
			Index:    0,
		})
	}

	// Revision
	if session.RevisionID != "" {
		instances = append(instances, NavigableInstance{
			ID:       session.RevisionID,
			Category: CategoryRevision,
			Label:    "Revision Coordinator",
			Index:    0,
		})
	}

	// Final consolidation
	if session.ConsolidationID != "" {
		instances = append(instances, NavigableInstance{
			ID:       session.ConsolidationID,
			Category: CategoryConsolidation,
			Label:    "Final Consolidation",
			Index:    0,
		})
	}

	return instances
}

// findTaskInstance finds the instance ID for a given task ID.
// Checks both active task-to-instance mapping and completed tasks.
func (n *PhaseAwareNavigator) findTaskInstance(session *orchestrator.UltraPlanSession, taskID string) string {
	// Check active mapping
	if instID, ok := session.TaskToInstance[taskID]; ok && instID != "" {
		return instID
	}

	// Check if task is completed (instance might have been cleaned up from mapping)
	if slices.Contains(session.CompletedTasks, taskID) {
		// Task is completed but no longer in mapping
		// This shouldn't normally happen, but handle it gracefully
		return ""
	}

	return ""
}

// NavigateNext moves to the next navigable instance.
// Returns true if navigation occurred, false if at end (or empty).
func (n *PhaseAwareNavigator) NavigateNext() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.navigableInstances) == 0 {
		return false
	}

	// Wrap around
	n.selectedIndex = (n.selectedIndex + 1) % len(n.navigableInstances)
	return true
}

// NavigatePrev moves to the previous navigable instance.
// Returns true if navigation occurred, false if at beginning (or empty).
func (n *PhaseAwareNavigator) NavigatePrev() bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if len(n.navigableInstances) == 0 {
		return false
	}

	// Wrap around
	n.selectedIndex = (n.selectedIndex - 1 + len(n.navigableInstances)) % len(n.navigableInstances)
	return true
}

// NavigateTo moves to a specific instance by ID.
// Returns true if the instance was found and selected.
func (n *PhaseAwareNavigator) NavigateTo(instanceID string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, inst := range n.navigableInstances {
		if inst.ID == instanceID {
			n.selectedIndex = i
			return true
		}
	}
	return false
}

// NavigateToIndex moves to a specific index in the navigable instances list.
// Returns true if the index was valid.
func (n *PhaseAwareNavigator) NavigateToIndex(index int) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if index < 0 || index >= len(n.navigableInstances) {
		return false
	}
	n.selectedIndex = index
	return true
}

// NavigateToTask moves to the instance associated with a task number (1-indexed).
// Returns true if a task with that number exists and has an instance.
func (n *PhaseAwareNavigator) NavigateToTask(taskNum int) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Convert to 0-indexed
	taskIndex := taskNum - 1
	if taskIndex < 0 {
		return false
	}

	for i, inst := range n.navigableInstances {
		if inst.Category == CategoryExecution && inst.Index == taskIndex {
			n.selectedIndex = i
			return true
		}
	}
	return false
}

// GetSelectedInstance returns the currently selected instance, or nil if none.
func (n *PhaseAwareNavigator) GetSelectedInstance() *NavigableInstance {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.selectedIndex < 0 || n.selectedIndex >= len(n.navigableInstances) {
		return nil
	}
	inst := n.navigableInstances[n.selectedIndex]
	return &inst
}

// GetSelectedID returns the ID of the currently selected instance.
// Returns empty string if no instance is selected.
func (n *PhaseAwareNavigator) GetSelectedID() string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.selectedIndex < 0 || n.selectedIndex >= len(n.navigableInstances) {
		return ""
	}
	return n.navigableInstances[n.selectedIndex].ID
}

// GetSelectedIndex returns the current selection index.
func (n *PhaseAwareNavigator) GetSelectedIndex() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.selectedIndex
}

// GetNavigableInstances returns all navigable instances.
func (n *PhaseAwareNavigator) GetNavigableInstances() []NavigableInstance {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]NavigableInstance, len(n.navigableInstances))
	copy(result, n.navigableInstances)
	return result
}

// GetNavigableInstancesByCategory returns instances filtered by category.
func (n *PhaseAwareNavigator) GetNavigableInstancesByCategory(category NavigationCategory) []NavigableInstance {
	n.mu.RLock()
	defer n.mu.RUnlock()

	var result []NavigableInstance
	for _, inst := range n.navigableInstances {
		if inst.Category == category {
			result = append(result, inst)
		}
	}
	return result
}

// Count returns the total number of navigable instances.
func (n *PhaseAwareNavigator) Count() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.navigableInstances)
}

// IsEmpty returns true if there are no navigable instances.
func (n *PhaseAwareNavigator) IsEmpty() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.navigableInstances) == 0
}

// ContainsID checks if an instance ID is in the navigable list.
func (n *PhaseAwareNavigator) ContainsID(instanceID string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, inst := range n.navigableInstances {
		if inst.ID == instanceID {
			return true
		}
	}
	return false
}

// GetScrollOffset returns the scroll offset for a category.
func (n *PhaseAwareNavigator) GetScrollOffset(category NavigationCategory) int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.scrollOffsets[category]
}

// SetScrollOffset sets the scroll offset for a category.
func (n *PhaseAwareNavigator) SetScrollOffset(category NavigationCategory, offset int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.scrollOffsets[category] = offset
}

// FindNextInCategory finds the next instance in a specific category.
// Returns the index, or -1 if not found.
func (n *PhaseAwareNavigator) FindNextInCategory(category NavigationCategory, direction int) int {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if len(n.navigableInstances) == 0 {
		return -1
	}

	count := len(n.navigableInstances)
	for i := 1; i <= count; i++ {
		nextIdx := (n.selectedIndex + i*direction + count) % count
		if n.navigableInstances[nextIdx].Category == category {
			return nextIdx
		}
	}
	return -1
}

// NavigateToCategory moves to the first instance in the specified category.
// Returns true if navigation occurred.
func (n *PhaseAwareNavigator) NavigateToCategory(category NavigationCategory) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	for i, inst := range n.navigableInstances {
		if inst.Category == category {
			n.selectedIndex = i
			return true
		}
	}
	return false
}

// CurrentPhase determines the current navigation phase based on selected instance.
func (n *PhaseAwareNavigator) CurrentPhase() orchestrator.UltraPlanPhase {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.selectedIndex < 0 || n.selectedIndex >= len(n.navigableInstances) {
		return ""
	}

	switch n.navigableInstances[n.selectedIndex].Category {
	case CategoryPlanning:
		return orchestrator.PhasePlanning
	case CategoryPlanSelection:
		return orchestrator.PhasePlanSelection
	case CategoryExecution, CategoryGroupConsolidation:
		return orchestrator.PhaseExecuting
	case CategorySynthesis:
		return orchestrator.PhaseSynthesis
	case CategoryRevision:
		return orchestrator.PhaseRevision
	case CategoryConsolidation:
		return orchestrator.PhaseConsolidating
	default:
		return ""
	}
}

// CategoryString returns a human-readable string for a navigation category.
func CategoryString(cat NavigationCategory) string {
	switch cat {
	case CategoryPlanning:
		return "Planning"
	case CategoryPlanSelection:
		return "Plan Selection"
	case CategoryExecution:
		return "Execution"
	case CategoryGroupConsolidation:
		return "Group Consolidation"
	case CategorySynthesis:
		return "Synthesis"
	case CategoryRevision:
		return "Revision"
	case CategoryConsolidation:
		return "Consolidation"
	default:
		return "Unknown"
	}
}

// FindInstanceByTaskID finds a navigable instance by its task ID.
// Returns nil if not found.
func (n *PhaseAwareNavigator) FindInstanceByTaskID(taskID string) *NavigableInstance {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, inst := range n.navigableInstances {
		if inst.TaskID == taskID {
			instCopy := inst
			return &instCopy
		}
	}
	return nil
}

// FindInstanceByLabel finds a navigable instance by label (case-insensitive partial match).
// Returns nil if not found.
func (n *PhaseAwareNavigator) FindInstanceByLabel(query string) *NavigableInstance {
	n.mu.RLock()
	defer n.mu.RUnlock()

	queryLower := strings.ToLower(query)
	for _, inst := range n.navigableInstances {
		if strings.Contains(strings.ToLower(inst.Label), queryLower) {
			instCopy := inst
			return &instCopy
		}
	}
	return nil
}

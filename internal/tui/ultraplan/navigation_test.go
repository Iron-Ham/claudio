package ultraplan

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// mockInstanceProvider implements InstanceProvider for testing
type mockInstanceProvider struct {
	instances map[string]*orchestrator.Instance
}

func newMockInstanceProvider() *mockInstanceProvider {
	return &mockInstanceProvider{
		instances: make(map[string]*orchestrator.Instance),
	}
}

func (m *mockInstanceProvider) GetInstance(id string) *orchestrator.Instance {
	return m.instances[id]
}

func (m *mockInstanceProvider) AddInstance(id string, status orchestrator.InstanceStatus) {
	m.instances[id] = &orchestrator.Instance{
		ID:      id,
		Status:  status,
		Created: time.Now(),
	}
}

// mockSessionProvider implements SessionProvider for testing
type mockSessionProvider struct {
	session *orchestrator.UltraPlanSession
}

func (m *mockSessionProvider) Session() *orchestrator.UltraPlanSession {
	return m.session
}

func newMockSessionProvider() *mockSessionProvider {
	return &mockSessionProvider{
		session: &orchestrator.UltraPlanSession{
			ID:             "test-session",
			Phase:          orchestrator.PhasePlanning,
			TaskToInstance: make(map[string]string),
			CompletedTasks: make([]string, 0),
			FailedTasks:    make([]string, 0),
		},
	}
}

func TestNewPhaseAwareNavigator(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()

	nav := NewPhaseAwareNavigator(ip, sp)

	if nav == nil {
		t.Fatal("expected non-nil navigator")
	}
	if nav.Count() != 0 {
		t.Errorf("expected empty navigator, got %d instances", nav.Count())
	}
	if !nav.IsEmpty() {
		t.Error("expected IsEmpty() to return true")
	}
}

func TestNavigator_Update_PlanningPhase(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Setup planning coordinator
	sp.session.CoordinatorID = "coord-1"
	ip.AddInstance("coord-1", orchestrator.StatusWorking)

	nav.Update()

	if nav.Count() != 1 {
		t.Errorf("expected 1 instance, got %d", nav.Count())
	}

	instances := nav.GetNavigableInstances()
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].Category != CategoryPlanning {
		t.Errorf("expected CategoryPlanning, got %d", instances[0].Category)
	}
	if instances[0].ID != "coord-1" {
		t.Errorf("expected coord-1, got %s", instances[0].ID)
	}
}

func TestNavigator_Update_SkipsPendingInstances(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Setup coordinator with pending status - should be skipped
	sp.session.CoordinatorID = "coord-1"
	ip.AddInstance("coord-1", orchestrator.StatusPending)

	nav.Update()

	if nav.Count() != 0 {
		t.Errorf("expected 0 instances (pending should be skipped), got %d", nav.Count())
	}
}

func TestNavigator_Update_MultiPassPlanning(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Setup multi-pass planning
	sp.session.PlanCoordinatorIDs = []string{"plan-coord-1", "plan-coord-2", "plan-coord-3"}
	sp.session.PlanManagerID = "plan-manager"

	ip.AddInstance("plan-coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("plan-coord-2", orchestrator.StatusWorking)
	ip.AddInstance("plan-coord-3", orchestrator.StatusPending) // Should be skipped
	ip.AddInstance("plan-manager", orchestrator.StatusPending) // Should be skipped

	nav.Update()

	if nav.Count() != 2 {
		t.Errorf("expected 2 instances, got %d", nav.Count())
	}

	instances := nav.GetNavigableInstancesByCategory(CategoryPlanSelection)
	if len(instances) != 2 {
		t.Errorf("expected 2 plan selection instances, got %d", len(instances))
	}
}

func TestNavigator_Update_ExecutionPhase(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Setup plan with tasks
	sp.session.Phase = orchestrator.PhaseExecuting
	sp.session.Plan = &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "First Task"},
			{ID: "task-2", Title: "Second Task"},
			{ID: "task-3", Title: "Third Task"},
		},
		ExecutionOrder: [][]string{
			{"task-1", "task-2"},
			{"task-3"},
		},
	}
	sp.session.TaskToInstance = map[string]string{
		"task-1": "inst-1",
		"task-2": "inst-2",
		"task-3": "inst-3",
	}

	ip.AddInstance("inst-1", orchestrator.StatusCompleted)
	ip.AddInstance("inst-2", orchestrator.StatusWorking)
	ip.AddInstance("inst-3", orchestrator.StatusWorking)

	nav.Update()

	executionInstances := nav.GetNavigableInstancesByCategory(CategoryExecution)
	if len(executionInstances) != 3 {
		t.Errorf("expected 3 execution instances, got %d", len(executionInstances))
	}

	// Verify order matches execution order
	if executionInstances[0].TaskID != "task-1" {
		t.Errorf("expected task-1 first, got %s", executionInstances[0].TaskID)
	}
	if executionInstances[1].TaskID != "task-2" {
		t.Errorf("expected task-2 second, got %s", executionInstances[1].TaskID)
	}
	if executionInstances[2].TaskID != "task-3" {
		t.Errorf("expected task-3 third, got %s", executionInstances[2].TaskID)
	}
}

func TestNavigator_Update_GroupConsolidators(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.Phase = orchestrator.PhaseExecuting
	sp.session.Plan = &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "Task 1"},
		},
		ExecutionOrder: [][]string{{"task-1"}},
	}
	sp.session.TaskToInstance = map[string]string{"task-1": "inst-1"}
	sp.session.GroupConsolidatorIDs = []string{"group-cons-1"}

	ip.AddInstance("inst-1", orchestrator.StatusCompleted)
	ip.AddInstance("group-cons-1", orchestrator.StatusWorking)

	nav.Update()

	groupConsolidators := nav.GetNavigableInstancesByCategory(CategoryGroupConsolidation)
	if len(groupConsolidators) != 1 {
		t.Errorf("expected 1 group consolidator, got %d", len(groupConsolidators))
	}
}

func TestNavigator_Update_AllPhases(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Setup a complete session with all phases
	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"
	sp.session.RevisionID = "rev-1"
	sp.session.ConsolidationID = "cons-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)
	ip.AddInstance("rev-1", orchestrator.StatusCompleted)
	ip.AddInstance("cons-1", orchestrator.StatusWorking)

	nav.Update()

	// Should have 4 instances
	if nav.Count() != 4 {
		t.Errorf("expected 4 instances, got %d", nav.Count())
	}

	// Verify categories
	if len(nav.GetNavigableInstancesByCategory(CategoryPlanning)) != 1 {
		t.Error("expected 1 planning instance")
	}
	if len(nav.GetNavigableInstancesByCategory(CategorySynthesis)) != 1 {
		t.Error("expected 1 synthesis instance")
	}
	if len(nav.GetNavigableInstancesByCategory(CategoryRevision)) != 1 {
		t.Error("expected 1 revision instance")
	}
	if len(nav.GetNavigableInstancesByCategory(CategoryConsolidation)) != 1 {
		t.Error("expected 1 consolidation instance")
	}
}

func TestNavigator_NavigateNextPrev(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Setup 3 instances
	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"
	sp.session.ConsolidationID = "cons-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)
	ip.AddInstance("cons-1", orchestrator.StatusWorking)

	nav.Update()

	// Initial selection should be 0
	if nav.GetSelectedIndex() != 0 {
		t.Errorf("expected initial index 0, got %d", nav.GetSelectedIndex())
	}
	if nav.GetSelectedID() != "coord-1" {
		t.Errorf("expected coord-1, got %s", nav.GetSelectedID())
	}

	// Navigate next
	if !nav.NavigateNext() {
		t.Error("NavigateNext should return true")
	}
	if nav.GetSelectedIndex() != 1 {
		t.Errorf("expected index 1, got %d", nav.GetSelectedIndex())
	}

	// Navigate next again
	nav.NavigateNext()
	if nav.GetSelectedIndex() != 2 {
		t.Errorf("expected index 2, got %d", nav.GetSelectedIndex())
	}

	// Navigate next should wrap
	nav.NavigateNext()
	if nav.GetSelectedIndex() != 0 {
		t.Errorf("expected index 0 (wrapped), got %d", nav.GetSelectedIndex())
	}

	// Navigate prev should wrap to end
	nav.NavigatePrev()
	if nav.GetSelectedIndex() != 2 {
		t.Errorf("expected index 2 (wrapped), got %d", nav.GetSelectedIndex())
	}

	// Navigate prev
	nav.NavigatePrev()
	if nav.GetSelectedIndex() != 1 {
		t.Errorf("expected index 1, got %d", nav.GetSelectedIndex())
	}
}

func TestNavigator_NavigateTo(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)

	nav.Update()

	// Navigate to synth-1
	if !nav.NavigateTo("synth-1") {
		t.Error("NavigateTo should return true for valid ID")
	}
	if nav.GetSelectedID() != "synth-1" {
		t.Errorf("expected synth-1, got %s", nav.GetSelectedID())
	}

	// Navigate to invalid ID
	if nav.NavigateTo("invalid-id") {
		t.Error("NavigateTo should return false for invalid ID")
	}

	// Navigate to coord-1
	if !nav.NavigateTo("coord-1") {
		t.Error("NavigateTo should return true for valid ID")
	}
	if nav.GetSelectedID() != "coord-1" {
		t.Errorf("expected coord-1, got %s", nav.GetSelectedID())
	}
}

func TestNavigator_NavigateToIndex(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)

	nav.Update()

	// Valid index
	if !nav.NavigateToIndex(1) {
		t.Error("NavigateToIndex should return true for valid index")
	}
	if nav.GetSelectedIndex() != 1 {
		t.Errorf("expected index 1, got %d", nav.GetSelectedIndex())
	}

	// Invalid indices
	if nav.NavigateToIndex(-1) {
		t.Error("NavigateToIndex should return false for negative index")
	}
	if nav.NavigateToIndex(99) {
		t.Error("NavigateToIndex should return false for out-of-range index")
	}
}

func TestNavigator_NavigateToTask(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.Phase = orchestrator.PhaseExecuting
	sp.session.Plan = &orchestrator.PlanSpec{
		Tasks: []orchestrator.PlannedTask{
			{ID: "task-1", Title: "First Task"},
			{ID: "task-2", Title: "Second Task"},
			{ID: "task-3", Title: "Third Task"},
		},
		ExecutionOrder: [][]string{
			{"task-1", "task-2", "task-3"},
		},
	}
	sp.session.TaskToInstance = map[string]string{
		"task-1": "inst-1",
		"task-2": "inst-2",
		"task-3": "inst-3",
	}

	ip.AddInstance("inst-1", orchestrator.StatusWorking)
	ip.AddInstance("inst-2", orchestrator.StatusWorking)
	ip.AddInstance("inst-3", orchestrator.StatusWorking)

	nav.Update()

	// Navigate to task 2 (1-indexed)
	if !nav.NavigateToTask(2) {
		t.Error("NavigateToTask should return true for valid task number")
	}

	selected := nav.GetSelectedInstance()
	if selected == nil {
		t.Fatal("expected non-nil selected instance")
	}
	if selected.TaskID != "task-2" {
		t.Errorf("expected task-2, got %s", selected.TaskID)
	}

	// Invalid task numbers
	if nav.NavigateToTask(0) {
		t.Error("NavigateToTask should return false for task 0")
	}
	if nav.NavigateToTask(99) {
		t.Error("NavigateToTask should return false for out-of-range task")
	}
}

func TestNavigator_GetSelectedInstance(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Empty navigator
	if nav.GetSelectedInstance() != nil {
		t.Error("expected nil for empty navigator")
	}

	sp.session.CoordinatorID = "coord-1"
	ip.AddInstance("coord-1", orchestrator.StatusWorking)

	nav.Update()

	inst := nav.GetSelectedInstance()
	if inst == nil {
		t.Fatal("expected non-nil instance")
	}
	if inst.ID != "coord-1" {
		t.Errorf("expected coord-1, got %s", inst.ID)
	}
	if inst.Category != CategoryPlanning {
		t.Errorf("expected CategoryPlanning, got %d", inst.Category)
	}
}

func TestNavigator_ContainsID(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	ip.AddInstance("coord-1", orchestrator.StatusWorking)

	nav.Update()

	if !nav.ContainsID("coord-1") {
		t.Error("expected ContainsID to return true for coord-1")
	}
	if nav.ContainsID("invalid-id") {
		t.Error("expected ContainsID to return false for invalid-id")
	}
}

func TestNavigator_ScrollOffsets(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Initial offset should be 0
	if nav.GetScrollOffset(CategoryExecution) != 0 {
		t.Error("expected initial scroll offset to be 0")
	}

	nav.SetScrollOffset(CategoryExecution, 5)
	if nav.GetScrollOffset(CategoryExecution) != 5 {
		t.Errorf("expected scroll offset 5, got %d", nav.GetScrollOffset(CategoryExecution))
	}

	// Different categories have independent offsets
	nav.SetScrollOffset(CategoryPlanning, 10)
	if nav.GetScrollOffset(CategoryPlanning) != 10 {
		t.Errorf("expected scroll offset 10, got %d", nav.GetScrollOffset(CategoryPlanning))
	}
	if nav.GetScrollOffset(CategoryExecution) != 5 {
		t.Errorf("expected execution offset still 5, got %d", nav.GetScrollOffset(CategoryExecution))
	}
}

func TestNavigator_FindNextInCategory(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"
	sp.session.ConsolidationID = "cons-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)
	ip.AddInstance("cons-1", orchestrator.StatusWorking)

	nav.Update()

	// From planning, find synthesis (forward)
	idx := nav.FindNextInCategory(CategorySynthesis, 1)
	if idx != 1 {
		t.Errorf("expected index 1 for synthesis, got %d", idx)
	}

	// From planning, find consolidation (forward)
	idx = nav.FindNextInCategory(CategoryConsolidation, 1)
	if idx != 2 {
		t.Errorf("expected index 2 for consolidation, got %d", idx)
	}

	// Find non-existent category
	idx = nav.FindNextInCategory(CategoryExecution, 1)
	if idx != -1 {
		t.Errorf("expected -1 for non-existent category, got %d", idx)
	}
}

func TestNavigator_NavigateToCategory(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)

	nav.Update()

	// Navigate to synthesis
	if !nav.NavigateToCategory(CategorySynthesis) {
		t.Error("NavigateToCategory should return true for existing category")
	}
	if nav.GetSelectedID() != "synth-1" {
		t.Errorf("expected synth-1, got %s", nav.GetSelectedID())
	}

	// Navigate to non-existent category
	if nav.NavigateToCategory(CategoryExecution) {
		t.Error("NavigateToCategory should return false for non-existent category")
	}
}

func TestNavigator_CurrentPhase(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)

	nav.Update()

	// Initial phase (planning)
	if nav.CurrentPhase() != orchestrator.PhasePlanning {
		t.Errorf("expected PhasePlanning, got %s", nav.CurrentPhase())
	}

	// Navigate to synthesis
	nav.NavigateTo("synth-1")
	if nav.CurrentPhase() != orchestrator.PhaseSynthesis {
		t.Errorf("expected PhaseSynthesis, got %s", nav.CurrentPhase())
	}
}

func TestNavigator_FindInstanceByTaskID(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.Plan = &orchestrator.PlanSpec{
		Tasks:          []orchestrator.PlannedTask{{ID: "task-1", Title: "Test Task"}},
		ExecutionOrder: [][]string{{"task-1"}},
	}
	sp.session.TaskToInstance = map[string]string{"task-1": "inst-1"}

	ip.AddInstance("inst-1", orchestrator.StatusWorking)

	nav.Update()

	inst := nav.FindInstanceByTaskID("task-1")
	if inst == nil {
		t.Fatal("expected non-nil instance for task-1")
	}
	if inst.ID != "inst-1" {
		t.Errorf("expected inst-1, got %s", inst.ID)
	}

	// Non-existent task
	if nav.FindInstanceByTaskID("invalid-task") != nil {
		t.Error("expected nil for non-existent task")
	}
}

func TestNavigator_FindInstanceByLabel(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.SynthesisID = "synth-1"
	ip.AddInstance("synth-1", orchestrator.StatusWorking)

	nav.Update()

	// Case-insensitive search
	inst := nav.FindInstanceByLabel("synthesis")
	if inst == nil {
		t.Fatal("expected non-nil instance for 'synthesis' query")
	}
	if inst.ID != "synth-1" {
		t.Errorf("expected synth-1, got %s", inst.ID)
	}

	// Partial match
	inst = nav.FindInstanceByLabel("Review")
	if inst == nil {
		t.Fatal("expected non-nil instance for 'Review' query")
	}

	// No match
	if nav.FindInstanceByLabel("nonexistent") != nil {
		t.Error("expected nil for non-matching query")
	}
}

func TestCategoryString(t *testing.T) {
	tests := []struct {
		cat      NavigationCategory
		expected string
	}{
		{CategoryPlanning, "Planning"},
		{CategoryPlanSelection, "Plan Selection"},
		{CategoryExecution, "Execution"},
		{CategoryGroupConsolidation, "Group Consolidation"},
		{CategorySynthesis, "Synthesis"},
		{CategoryRevision, "Revision"},
		{CategoryConsolidation, "Consolidation"},
		{NavigationCategory(999), "Unknown"},
	}

	for _, tt := range tests {
		result := CategoryString(tt.cat)
		if result != tt.expected {
			t.Errorf("CategoryString(%d) = %q, want %q", tt.cat, result, tt.expected)
		}
	}
}

func TestNavigator_Update_PreservesSelection(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	sp.session.SynthesisID = "synth-1"

	ip.AddInstance("coord-1", orchestrator.StatusCompleted)
	ip.AddInstance("synth-1", orchestrator.StatusCompleted)

	nav.Update()
	nav.NavigateTo("synth-1")

	// Verify current selection
	if nav.GetSelectedID() != "synth-1" {
		t.Fatalf("expected synth-1, got %s", nav.GetSelectedID())
	}

	// Add another instance and update
	sp.session.ConsolidationID = "cons-1"
	ip.AddInstance("cons-1", orchestrator.StatusWorking)

	nav.Update()

	// Selection should be preserved
	if nav.GetSelectedID() != "synth-1" {
		t.Errorf("expected selection to be preserved as synth-1, got %s", nav.GetSelectedID())
	}
}

func TestNavigator_EmptyNavigation(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	// Empty navigator should handle navigation gracefully
	if nav.NavigateNext() {
		t.Error("NavigateNext should return false for empty navigator")
	}
	if nav.NavigatePrev() {
		t.Error("NavigatePrev should return false for empty navigator")
	}
	if nav.GetSelectedID() != "" {
		t.Error("GetSelectedID should return empty string for empty navigator")
	}
	if nav.GetSelectedInstance() != nil {
		t.Error("GetSelectedInstance should return nil for empty navigator")
	}
}

func TestNavigator_NilSession(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := &mockSessionProvider{session: nil}
	nav := NewPhaseAwareNavigator(ip, sp)

	// Should not panic
	nav.Update()

	if !nav.IsEmpty() {
		t.Error("expected empty navigator for nil session")
	}
}

func TestNavigator_ThreadSafety(t *testing.T) {
	ip := newMockInstanceProvider()
	sp := newMockSessionProvider()
	nav := NewPhaseAwareNavigator(ip, sp)

	sp.session.CoordinatorID = "coord-1"
	ip.AddInstance("coord-1", orchestrator.StatusWorking)
	nav.Update()

	done := make(chan bool)

	// Concurrent reads
	for range 10 {
		go func() {
			for range 100 {
				_ = nav.GetSelectedID()
				_ = nav.GetSelectedInstance()
				_ = nav.Count()
				_ = nav.GetNavigableInstances()
			}
			done <- true
		}()
	}

	// Concurrent navigation
	for range 5 {
		go func() {
			for range 100 {
				nav.NavigateNext()
				nav.NavigatePrev()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for range 15 {
		<-done
	}
}

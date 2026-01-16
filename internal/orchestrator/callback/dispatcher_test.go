package callback

import (
	"sync"
	"testing"
)

// mockPersister implements SessionPersister for testing
type mockPersister struct {
	saveCount int
	mu        sync.Mutex
}

func (m *mockPersister) SaveSession() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saveCount++
	return nil
}

func (m *mockPersister) getSaveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveCount
}

func TestNewDispatcher(t *testing.T) {
	t.Run("with nil dependencies", func(t *testing.T) {
		d := NewDispatcher(nil, nil)
		if d == nil {
			t.Fatal("NewDispatcher returned nil")
		}
		if d.callbacks != nil {
			t.Error("expected nil callbacks")
		}
	})

	t.Run("with persister", func(t *testing.T) {
		persister := &mockPersister{}
		d := NewDispatcher(persister, nil)
		if d == nil {
			t.Fatal("NewDispatcher returned nil")
		}
		if d.persister != persister {
			t.Error("persister not set")
		}
	})
}

func TestDispatcher_SetCallbacks(t *testing.T) {
	d := NewDispatcher(nil, nil)

	cb := &Callbacks{
		OnComplete: func(success bool, summary string) {},
	}

	d.SetCallbacks(cb)

	if d.GetCallbacks() != cb {
		t.Error("callbacks not set correctly")
	}
}

func TestDispatcher_NotifyPhaseChange(t *testing.T) {
	persister := &mockPersister{}
	d := NewDispatcher(persister, nil)

	var receivedPhase UltraPlanPhase
	d.SetCallbacks(&Callbacks{
		OnPhaseChange: func(phase UltraPlanPhase) {
			receivedPhase = phase
		},
	})

	d.NotifyPhaseChange(PhasePlanning, PhaseExecuting, "test-session")

	if receivedPhase != PhaseExecuting {
		t.Errorf("expected phase %s, got %s", PhaseExecuting, receivedPhase)
	}

	if persister.getSaveCount() != 1 {
		t.Errorf("expected 1 save call, got %d", persister.getSaveCount())
	}
}

func TestDispatcher_NotifyTaskStart(t *testing.T) {
	d := NewDispatcher(nil, nil)

	var receivedTaskID, receivedInstanceID string
	d.SetCallbacks(&Callbacks{
		OnTaskStart: func(taskID, instanceID string) {
			receivedTaskID = taskID
			receivedInstanceID = instanceID
		},
	})

	d.NotifyTaskStart("task-1", "instance-1", "Test Task")

	if receivedTaskID != "task-1" {
		t.Errorf("expected task ID 'task-1', got %s", receivedTaskID)
	}
	if receivedInstanceID != "instance-1" {
		t.Errorf("expected instance ID 'instance-1', got %s", receivedInstanceID)
	}
}

func TestDispatcher_NotifyTaskComplete(t *testing.T) {
	persister := &mockPersister{}
	d := NewDispatcher(persister, nil)

	var receivedTaskID string
	d.SetCallbacks(&Callbacks{
		OnTaskComplete: func(taskID string) {
			receivedTaskID = taskID
		},
	})

	d.NotifyTaskComplete("task-1")

	if receivedTaskID != "task-1" {
		t.Errorf("expected task ID 'task-1', got %s", receivedTaskID)
	}

	if persister.getSaveCount() != 1 {
		t.Errorf("expected 1 save call, got %d", persister.getSaveCount())
	}
}

func TestDispatcher_NotifyTaskFailed(t *testing.T) {
	persister := &mockPersister{}
	d := NewDispatcher(persister, nil)

	var receivedTaskID, receivedReason string
	d.SetCallbacks(&Callbacks{
		OnTaskFailed: func(taskID, reason string) {
			receivedTaskID = taskID
			receivedReason = reason
		},
	})

	d.NotifyTaskFailed("task-1", "test failure")

	if receivedTaskID != "task-1" {
		t.Errorf("expected task ID 'task-1', got %s", receivedTaskID)
	}
	if receivedReason != "test failure" {
		t.Errorf("expected reason 'test failure', got %s", receivedReason)
	}

	if persister.getSaveCount() != 1 {
		t.Errorf("expected 1 save call, got %d", persister.getSaveCount())
	}
}

func TestDispatcher_NotifyGroupComplete(t *testing.T) {
	d := NewDispatcher(nil, nil)

	var receivedGroupIndex int
	d.SetCallbacks(&Callbacks{
		OnGroupComplete: func(groupIndex int) {
			receivedGroupIndex = groupIndex
		},
	})

	d.NotifyGroupComplete(2, 5)

	if receivedGroupIndex != 2 {
		t.Errorf("expected group index 2, got %d", receivedGroupIndex)
	}
}

func TestDispatcher_NotifyPlanReady(t *testing.T) {
	d := NewDispatcher(nil, nil)

	var receivedPlan PlanSpec
	d.SetCallbacks(&Callbacks{
		OnPlanReady: func(plan PlanSpec) {
			receivedPlan = plan
		},
	})

	testPlan := "test-plan"
	d.NotifyPlanReady(testPlan, 10, 3)

	if receivedPlan != testPlan {
		t.Error("plan not received correctly")
	}
}

func TestDispatcher_NotifyProgress(t *testing.T) {
	d := NewDispatcher(nil, nil)

	var receivedCompleted, receivedTotal int
	var receivedPhase UltraPlanPhase
	d.SetCallbacks(&Callbacks{
		OnProgress: func(completed, total int, phase UltraPlanPhase) {
			receivedCompleted = completed
			receivedTotal = total
			receivedPhase = phase
		},
	})

	d.NotifyProgress(5, 10, PhaseExecuting)

	if receivedCompleted != 5 {
		t.Errorf("expected completed 5, got %d", receivedCompleted)
	}
	if receivedTotal != 10 {
		t.Errorf("expected total 10, got %d", receivedTotal)
	}
	if receivedPhase != PhaseExecuting {
		t.Errorf("expected phase %s, got %s", PhaseExecuting, receivedPhase)
	}
}

func TestDispatcher_NotifyComplete(t *testing.T) {
	d := NewDispatcher(nil, nil)

	var receivedSuccess bool
	var receivedSummary string
	d.SetCallbacks(&Callbacks{
		OnComplete: func(success bool, summary string) {
			receivedSuccess = success
			receivedSummary = summary
		},
	})

	d.NotifyComplete(true, "All tasks completed")

	if !receivedSuccess {
		t.Error("expected success to be true")
	}
	if receivedSummary != "All tasks completed" {
		t.Errorf("expected summary 'All tasks completed', got %s", receivedSummary)
	}
}

func TestDispatcher_NilCallbacks(t *testing.T) {
	// Verify that calling notification methods with nil callbacks doesn't panic
	d := NewDispatcher(nil, nil)

	// These should not panic
	d.NotifyPhaseChange(PhasePlanning, PhaseExecuting, "test-session")
	d.NotifyTaskStart("task-1", "instance-1", "Test Task")
	d.NotifyTaskComplete("task-1")
	d.NotifyTaskFailed("task-1", "failure")
	d.NotifyGroupComplete(0, 1)
	d.NotifyPlanReady(nil, 0, 0)
	d.NotifyProgress(0, 0, PhasePlanning)
	d.NotifyComplete(true, "done")
}

func TestDispatcher_ThreadSafety(t *testing.T) {
	d := NewDispatcher(nil, nil)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent callback setting
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			d.SetCallbacks(&Callbacks{
				OnComplete: func(success bool, summary string) {},
			})
		}
	}()

	// Concurrent notifications
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			d.NotifyPhaseChange(PhasePlanning, PhaseExecuting, "test")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			d.NotifyTaskStart("task", "instance", "title")
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			d.NotifyProgress(1, 10, PhaseExecuting)
		}
	}()

	wg.Wait()
}

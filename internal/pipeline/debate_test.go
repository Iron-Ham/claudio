package pipeline

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
	"github.com/Iron-Ham/claudio/internal/taskqueue"
	"github.com/Iron-Ham/claudio/internal/ultraplan"
)

func TestFindConflicts_OverlappingFiles(t *testing.T) {
	dc := NewDebateCoordinator(nil, nil)

	tasks := []taskqueue.QueuedTask{
		{PlannedTask: ultraplan.PlannedTask{ID: "t1", Files: []string{"a.go", "b.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t2", Files: []string{"b.go", "c.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t3", Files: []string{"d.go"}}, Status: taskqueue.TaskCompleted},
	}

	conflicts := dc.FindConflicts(tasks)
	if len(conflicts) != 1 {
		t.Fatalf("FindConflicts = %d conflicts, want 1", len(conflicts))
	}

	c := conflicts[0]
	if c.TaskA != "t1" || c.TaskB != "t2" {
		t.Errorf("conflict pair = (%s, %s), want (t1, t2)", c.TaskA, c.TaskB)
	}
	if len(c.OverlapFiles) != 1 || c.OverlapFiles[0] != "b.go" {
		t.Errorf("OverlapFiles = %v, want [b.go]", c.OverlapFiles)
	}
}

func TestFindConflicts_NoOverlap(t *testing.T) {
	dc := NewDebateCoordinator(nil, nil)

	tasks := []taskqueue.QueuedTask{
		{PlannedTask: ultraplan.PlannedTask{ID: "t1", Files: []string{"a.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t2", Files: []string{"b.go"}}, Status: taskqueue.TaskCompleted},
	}

	conflicts := dc.FindConflicts(tasks)
	if len(conflicts) != 0 {
		t.Errorf("FindConflicts = %d conflicts, want 0", len(conflicts))
	}
}

func TestFindConflicts_SkipsFailedTasks(t *testing.T) {
	dc := NewDebateCoordinator(nil, nil)

	tasks := []taskqueue.QueuedTask{
		{PlannedTask: ultraplan.PlannedTask{ID: "t1", Files: []string{"a.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t2", Files: []string{"a.go"}}, Status: taskqueue.TaskFailed},
	}

	conflicts := dc.FindConflicts(tasks)
	if len(conflicts) != 0 {
		t.Errorf("FindConflicts = %d conflicts, want 0 (failed task should be excluded)", len(conflicts))
	}
}

func TestFindConflicts_MultipleOverlaps(t *testing.T) {
	dc := NewDebateCoordinator(nil, nil)

	tasks := []taskqueue.QueuedTask{
		{PlannedTask: ultraplan.PlannedTask{ID: "t1", Files: []string{"a.go", "b.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t2", Files: []string{"a.go", "b.go"}}, Status: taskqueue.TaskCompleted},
	}

	conflicts := dc.FindConflicts(tasks)
	if len(conflicts) != 1 {
		t.Fatalf("FindConflicts = %d conflicts, want 1", len(conflicts))
	}
	if len(conflicts[0].OverlapFiles) != 2 {
		t.Errorf("OverlapFiles = %v, want 2 files", conflicts[0].OverlapFiles)
	}
}

func TestFindConflicts_ThreeWayConflict(t *testing.T) {
	dc := NewDebateCoordinator(nil, nil)

	tasks := []taskqueue.QueuedTask{
		{PlannedTask: ultraplan.PlannedTask{ID: "t1", Files: []string{"shared.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t2", Files: []string{"shared.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t3", Files: []string{"shared.go"}}, Status: taskqueue.TaskCompleted},
	}

	conflicts := dc.FindConflicts(tasks)
	// 3 tasks sharing one file → 3 pairs: (t1,t2), (t1,t3), (t2,t3)
	if len(conflicts) != 3 {
		t.Errorf("FindConflicts = %d conflicts, want 3", len(conflicts))
	}
}

func TestRunDebates_CreatesSessionsAndResolves(t *testing.T) {
	bus := event.NewBus()
	mb := mailbox.NewMailbox(t.TempDir())

	// Track debate events.
	started := make(chan event.Event, 1)
	resolved := make(chan event.Event, 1)
	subStart := bus.Subscribe("debate.started", func(e event.Event) {
		select {
		case started <- e:
		default:
		}
	})
	defer bus.Unsubscribe(subStart)
	subResolve := bus.Subscribe("debate.resolved", func(e event.Event) {
		select {
		case resolved <- e:
		default:
		}
	})
	defer bus.Unsubscribe(subResolve)

	dc := NewDebateCoordinator(mb, bus)

	tasks := []taskqueue.QueuedTask{
		{PlannedTask: ultraplan.PlannedTask{ID: "t1", Title: "Add auth", Description: "Add JWT auth", Files: []string{"auth.go"}}, Status: taskqueue.TaskCompleted},
		{PlannedTask: ultraplan.PlannedTask{ID: "t2", Title: "Add logging", Description: "Add structured logging", Files: []string{"auth.go"}}, Status: taskqueue.TaskCompleted},
	}

	conflicts := dc.FindConflicts(tasks)
	if len(conflicts) != 1 {
		t.Fatalf("FindConflicts = %d, want 1", len(conflicts))
	}

	resolutions, err := dc.RunDebates(context.Background(), conflicts, tasks)
	if err != nil {
		t.Fatalf("RunDebates: %v", err)
	}

	if len(resolutions) != 1 {
		t.Fatalf("RunDebates = %d resolutions, want 1", len(resolutions))
	}

	r := resolutions[0]
	if r.TaskA != "t1" || r.TaskB != "t2" {
		t.Errorf("resolution pair = (%s, %s), want (t1, t2)", r.TaskA, r.TaskB)
	}
	if !strings.Contains(r.Resolution, "auth.go") {
		t.Errorf("resolution missing file reference: %q", r.Resolution)
	}

	// Verify events were published.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Error("timed out waiting for DebateStartedEvent")
	}
	select {
	case <-resolved:
	case <-time.After(time.Second):
		t.Error("timed out waiting for DebateResolvedEvent")
	}

	// Verify Resolutions() accessor.
	stored := dc.Resolutions()
	if len(stored) != 1 {
		t.Errorf("Resolutions() = %d, want 1", len(stored))
	}
}

func TestRunDebates_EmptyConflicts(t *testing.T) {
	dc := NewDebateCoordinator(nil, nil)

	resolutions, err := dc.RunDebates(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("RunDebates: %v", err)
	}
	if len(resolutions) != 0 {
		t.Errorf("RunDebates = %d resolutions, want 0", len(resolutions))
	}
}

func TestFormatDebateContext_Empty(t *testing.T) {
	result := formatDebateContext(nil)
	if result != "" {
		t.Errorf("formatDebateContext(nil) = %q, want empty", result)
	}
}

func TestFormatDebateContext_WithResolutions(t *testing.T) {
	resolutions := []DebateResolution{
		{TaskA: "t1", TaskB: "t2", Files: []string{"a.go"}, Resolution: "reconciled"},
	}

	result := formatDebateContext(resolutions)
	if !strings.Contains(result, "t1 vs t2") {
		t.Error("formatDebateContext missing task pair")
	}
	if !strings.Contains(result, "a.go") {
		t.Error("formatDebateContext missing file reference")
	}
	if !strings.Contains(result, "reconciled") {
		t.Error("formatDebateContext missing resolution text")
	}
}

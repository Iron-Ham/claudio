package ultraplan

import (
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

func TestNewNotificationManager(t *testing.T) {
	config := DefaultNotificationConfig()
	nm := NewNotificationManager(config)

	if nm == nil {
		t.Fatal("NewNotificationManager returned nil")
	}

	if nm.current != nil {
		t.Error("new manager should have no current notification")
	}

	if len(nm.queue) != 0 {
		t.Error("new manager should have empty queue")
	}
}

func TestAddNotification(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	nm.AddNotification(Notification{
		Type:     NotificationPlanReady,
		Message:  "Test notification",
		Severity: SeverityInfo,
	})

	current := nm.GetCurrentNotification()
	if current == nil {
		t.Fatal("expected current notification after add")
	}

	if current.Type != NotificationPlanReady {
		t.Errorf("expected type %s, got %s", NotificationPlanReady, current.Type)
	}

	if current.Message != "Test notification" {
		t.Errorf("expected message 'Test notification', got %s", current.Message)
	}

	if current.Timestamp.IsZero() {
		t.Error("expected timestamp to be set")
	}
}

func TestNotificationQueue(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	// Add multiple notifications
	nm.AddNotification(Notification{Type: NotificationPlanReady, Message: "First"})
	nm.AddNotification(Notification{Type: NotificationSynthesisReady, Message: "Second"})
	nm.AddNotification(Notification{Type: NotificationComplete, Message: "Third"})

	// First should be current
	current := nm.GetCurrentNotification()
	if current.Message != "First" {
		t.Errorf("expected 'First', got '%s'", current.Message)
	}

	// Queue should have 2
	if nm.QueueLength() != 2 {
		t.Errorf("expected queue length 2, got %d", nm.QueueLength())
	}

	// Dismiss and check second is promoted
	nm.DismissNotification()
	current = nm.GetCurrentNotification()
	if current.Message != "Second" {
		t.Errorf("expected 'Second', got '%s'", current.Message)
	}

	if nm.QueueLength() != 1 {
		t.Errorf("expected queue length 1, got %d", nm.QueueLength())
	}

	// Dismiss again
	nm.DismissNotification()
	current = nm.GetCurrentNotification()
	if current.Message != "Third" {
		t.Errorf("expected 'Third', got '%s'", current.Message)
	}

	// Dismiss last
	nm.DismissNotification()
	if nm.GetCurrentNotification() != nil {
		t.Error("expected no current notification after all dismissed")
	}
}

func TestHasPending(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	if nm.HasPending() {
		t.Error("new manager should not have pending")
	}

	nm.AddNotification(Notification{Type: NotificationPlanReady, Message: "Test"})

	if !nm.HasPending() {
		t.Error("should have pending after add")
	}

	nm.DismissNotification()

	if nm.HasPending() {
		t.Error("should not have pending after dismiss")
	}
}

func TestClearAll(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	nm.AddNotification(Notification{Type: NotificationPlanReady, Message: "First"})
	nm.AddNotification(Notification{Type: NotificationSynthesisReady, Message: "Second"})

	nm.ClearAll()

	if nm.GetCurrentNotification() != nil {
		t.Error("expected no current notification after clear")
	}

	if nm.QueueLength() != 0 {
		t.Error("expected empty queue after clear")
	}
}

func TestCheckForPhaseNotification_Synthesis(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	session := &orchestrator.UltraPlanSession{
		Phase: orchestrator.PhaseSynthesis,
	}

	// First check should produce notification
	n := nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Fatal("expected notification for synthesis phase")
	}
	if n.Type != NotificationSynthesisReady {
		t.Errorf("expected type %s, got %s", NotificationSynthesisReady, n.Type)
	}

	// Second check for same phase should not produce notification
	n = nm.CheckForPhaseNotification(session)
	if n != nil {
		t.Error("should not produce duplicate notification for same phase")
	}
}

func TestCheckForPhaseNotification_Revision(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	session := &orchestrator.UltraPlanSession{
		Phase: orchestrator.PhaseRevision,
	}

	n := nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Fatal("expected notification for revision phase")
	}
	if n.Type != NotificationRevisionStarted {
		t.Errorf("expected type %s, got %s", NotificationRevisionStarted, n.Type)
	}
	if !n.AutoDismiss {
		t.Error("revision notification should auto-dismiss")
	}
}

func TestCheckForPhaseNotification_ConsolidationPaused(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	session := &orchestrator.UltraPlanSession{
		Phase: orchestrator.PhaseConsolidating,
		Consolidation: &orchestrator.ConsolidationState{
			Phase: orchestrator.ConsolidationPaused,
		},
	}

	n := nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Fatal("expected notification for consolidation paused")
	}
	if n.Type != NotificationConsolidationPaused {
		t.Errorf("expected type %s, got %s", NotificationConsolidationPaused, n.Type)
	}
	if n.Severity != SeverityCritical {
		t.Error("consolidation paused should be critical severity")
	}
}

func TestCheckForPhaseNotification_GroupDecision(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	session := &orchestrator.UltraPlanSession{
		Phase: orchestrator.PhaseExecuting,
		GroupDecision: &orchestrator.GroupDecisionState{
			AwaitingDecision: true,
		},
	}

	// First check should produce notification
	n := nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Fatal("expected notification for group decision")
	}
	if n.Type != NotificationGroupDecision {
		t.Errorf("expected type %s, got %s", NotificationGroupDecision, n.Type)
	}

	// Second check should not produce notification (still awaiting)
	n = nm.CheckForPhaseNotification(session)
	if n != nil {
		t.Error("should not produce duplicate notification while still awaiting")
	}

	// Clear the decision state
	session.GroupDecision.AwaitingDecision = false
	nm.CheckForPhaseNotification(session)

	// New decision should produce notification
	session.GroupDecision.AwaitingDecision = true
	n = nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Error("expected notification for new group decision")
	}
}

func TestCheckForPhaseNotification_Complete(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	session := &orchestrator.UltraPlanSession{
		Phase: orchestrator.PhaseComplete,
	}

	n := nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Fatal("expected notification for completion")
	}
	if n.Type != NotificationComplete {
		t.Errorf("expected type %s, got %s", NotificationComplete, n.Type)
	}
}

func TestCheckForPhaseNotification_Nil(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	n := nm.CheckForPhaseNotification(nil)
	if n != nil {
		t.Error("expected nil for nil session")
	}
}

func TestResetPhaseTracking(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	// Trigger some phase notifications
	session := &orchestrator.UltraPlanSession{Phase: orchestrator.PhaseSynthesis}
	nm.CheckForPhaseNotification(session)

	// Reset tracking
	nm.ResetPhaseTracking()

	// Same phase should now produce notification again
	n := nm.CheckForPhaseNotification(session)
	if n == nil {
		t.Error("expected notification after reset")
	}
}

func TestNotifyPlanReady(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	nm.NotifyPlanReady(5, 2)

	current := nm.GetCurrentNotification()
	if current == nil {
		t.Fatal("expected notification")
	}
	if current.Type != NotificationPlanReady {
		t.Errorf("expected type %s, got %s", NotificationPlanReady, current.Type)
	}
}

func TestNotifyError(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	nm.NotifyError("test error")

	current := nm.GetCurrentNotification()
	if current == nil {
		t.Fatal("expected notification")
	}
	if current.Type != NotificationError {
		t.Errorf("expected type %s, got %s", NotificationError, current.Type)
	}
	if current.Message != "test error" {
		t.Errorf("expected message 'test error', got '%s'", current.Message)
	}
	if current.Severity != SeverityCritical {
		t.Error("error notification should be critical severity")
	}
}

func TestCheckTimeout(t *testing.T) {
	config := DefaultNotificationConfig()
	config.AutoDismissTimeout = 50 * time.Millisecond
	nm := NewNotificationManager(config)

	// Add an auto-dismiss notification
	nm.AddNotification(Notification{
		Type:        NotificationRevisionStarted,
		Message:     "Test",
		AutoDismiss: true,
	})

	// Should not timeout immediately
	if nm.CheckTimeout() {
		t.Error("should not timeout immediately")
	}

	// Wait for timeout
	time.Sleep(60 * time.Millisecond)

	// Should timeout now
	if !nm.CheckTimeout() {
		t.Error("should have timed out")
	}

	if nm.GetCurrentNotification() != nil {
		t.Error("notification should be dismissed after timeout")
	}
}

func TestCheckTimeout_NonAutoDismiss(t *testing.T) {
	config := DefaultNotificationConfig()
	config.AutoDismissTimeout = 10 * time.Millisecond
	nm := NewNotificationManager(config)

	// Add a non-auto-dismiss notification
	nm.AddNotification(Notification{
		Type:        NotificationPlanReady,
		Message:     "Test",
		AutoDismiss: false,
	})

	time.Sleep(20 * time.Millisecond)

	// Should not timeout
	if nm.CheckTimeout() {
		t.Error("non-auto-dismiss notification should not timeout")
	}

	if nm.GetCurrentNotification() == nil {
		t.Error("notification should still be present")
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity NotificationSeverity
		expected string
	}{
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityCritical, "critical"},
		{NotificationSeverity(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.severity.String(); got != tt.expected {
			t.Errorf("severity %d: expected %s, got %s", tt.severity, tt.expected, got)
		}
	}
}

func TestTypeDescription(t *testing.T) {
	tests := []struct {
		typ      NotificationType
		expected string
	}{
		{NotificationPlanReady, "Plan Ready"},
		{NotificationSynthesisReady, "Synthesis Complete"},
		{NotificationRevisionStarted, "Revision Started"},
		{NotificationConsolidationPaused, "Consolidation Paused"},
		{NotificationGroupDecision, "Decision Required"},
		{NotificationComplete, "Complete"},
		{NotificationError, "Error"},
		{NotificationType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.typ.Description(); got != tt.expected {
			t.Errorf("type %s: expected %s, got %s", tt.typ, tt.expected, got)
		}
	}
}

func TestUpdateConfig(t *testing.T) {
	nm := NewNotificationManager(DefaultNotificationConfig())

	newConfig := NotificationConfig{
		Enabled:            false,
		UseSound:           false,
		AutoDismissTimeout: 10 * time.Second,
	}

	nm.UpdateConfig(newConfig)

	got := nm.GetConfig()
	if got.Enabled != false {
		t.Error("config not updated")
	}
	if got.AutoDismissTimeout != 10*time.Second {
		t.Error("timeout not updated")
	}
}

func TestDefaultNotificationConfig(t *testing.T) {
	config := DefaultNotificationConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}
	if !config.UseSound {
		t.Error("default config should use sound")
	}
	if config.AutoDismissTimeout != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", config.AutoDismissTimeout)
	}
}

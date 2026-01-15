package phase

import (
	"context"
	"testing"
	"time"
)

func TestNewSynthesisOrchestrator(t *testing.T) {
	tests := []struct {
		name    string
		ctx     *PhaseContext
		wantErr error
	}{
		{
			name: "valid context creates orchestrator",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: nil,
		},
		{
			name: "valid context with all optional fields",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
				Logger:       nil, // Will use NopLogger
				Callbacks:    &mockCallbacks{},
			},
			wantErr: nil,
		},
		{
			name: "nil manager returns error",
			ctx: &PhaseContext{
				Manager:      nil,
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			},
			wantErr: ErrNilManager,
		},
		{
			name: "nil orchestrator returns error",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: nil,
				Session:      &mockSession{},
			},
			wantErr: ErrNilOrchestrator,
		},
		{
			name: "nil session returns error",
			ctx: &PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      nil,
			},
			wantErr: ErrNilSession,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synth, err := NewSynthesisOrchestrator(tt.ctx)

			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("NewSynthesisOrchestrator() error = %v, want %v", err, tt.wantErr)
				}
				if synth != nil {
					t.Error("NewSynthesisOrchestrator() should return nil on error")
				}
			} else {
				if err != nil {
					t.Errorf("NewSynthesisOrchestrator() unexpected error: %v", err)
				}
				if synth == nil {
					t.Error("NewSynthesisOrchestrator() should return non-nil orchestrator")
				}
			}
		})
	}
}

func TestSynthesisOrchestrator_Phase(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	if synth.Phase() != PhaseSynthesis {
		t.Errorf("Phase() = %v, want %v", synth.Phase(), PhaseSynthesis)
	}
}

func TestSynthesisOrchestrator_Execute(t *testing.T) {
	t.Run("Execute with background context", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Execute should complete without error (currently stub implementation)
		err = synth.Execute(context.Background())
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
	})

	t.Run("Execute respects context cancellation", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Execute should complete (stub doesn't check context yet)
		err = synth.Execute(ctx)
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
	})
}

func TestSynthesisOrchestrator_Cancel(t *testing.T) {
	t.Run("Cancel is idempotent", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Call Cancel multiple times - should not panic
		synth.Cancel()
		synth.Cancel()
		synth.Cancel()
	})

	t.Run("Cancel before Execute", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Cancel before Execute is called - should not panic
		synth.Cancel()
	})

	t.Run("Cancel during Execute", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Start Execute in a goroutine
		done := make(chan struct{})
		go func() {
			_ = synth.Execute(context.Background())
			close(done)
		}()

		// Give Execute a moment to start, then cancel
		time.Sleep(10 * time.Millisecond)
		synth.Cancel()

		// Wait for Execute to complete
		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Error("Execute did not complete after Cancel")
		}
	})
}

func TestSynthesisOrchestrator_State(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial state is empty", func(t *testing.T) {
		state := synth.State()
		if state.InstanceID != "" {
			t.Errorf("State().InstanceID = %q, want empty", state.InstanceID)
		}
		if state.AwaitingApproval {
			t.Error("State().AwaitingApproval = true, want false")
		}
		if state.RevisionRound != 0 {
			t.Errorf("State().RevisionRound = %d, want 0", state.RevisionRound)
		}
		if len(state.IssuesFound) != 0 {
			t.Errorf("State().IssuesFound len = %d, want 0", len(state.IssuesFound))
		}
	})

	t.Run("State returns a copy", func(t *testing.T) {
		state1 := synth.State()
		state2 := synth.State()

		// Modify state1 - should not affect state2
		state1.InstanceID = "modified"
		if state2.InstanceID == "modified" {
			t.Error("State() should return independent copies")
		}
	})
}

func TestSynthesisOrchestrator_AwaitingApproval(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial state is not awaiting approval", func(t *testing.T) {
		if synth.IsAwaitingApproval() {
			t.Error("IsAwaitingApproval() = true, want false initially")
		}
	})

	t.Run("SetAwaitingApproval updates state", func(t *testing.T) {
		synth.SetAwaitingApproval(true)
		if !synth.IsAwaitingApproval() {
			t.Error("IsAwaitingApproval() = false after SetAwaitingApproval(true)")
		}

		synth.SetAwaitingApproval(false)
		if synth.IsAwaitingApproval() {
			t.Error("IsAwaitingApproval() = true after SetAwaitingApproval(false)")
		}
	})
}

func TestSynthesisOrchestrator_RevisionRound(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial revision round is 0", func(t *testing.T) {
		if synth.GetRevisionRound() != 0 {
			t.Errorf("GetRevisionRound() = %d, want 0", synth.GetRevisionRound())
		}
	})

	t.Run("SetRevisionRound updates state", func(t *testing.T) {
		synth.SetRevisionRound(1)
		if synth.GetRevisionRound() != 1 {
			t.Errorf("GetRevisionRound() = %d, want 1", synth.GetRevisionRound())
		}

		synth.SetRevisionRound(3)
		if synth.GetRevisionRound() != 3 {
			t.Errorf("GetRevisionRound() = %d, want 3", synth.GetRevisionRound())
		}
	})
}

func TestSynthesisOrchestrator_InstanceID(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial instance ID is empty", func(t *testing.T) {
		if synth.GetInstanceID() != "" {
			t.Errorf("GetInstanceID() = %q, want empty", synth.GetInstanceID())
		}
	})

	t.Run("setInstanceID updates state", func(t *testing.T) {
		synth.setInstanceID("test-instance-123")
		if synth.GetInstanceID() != "test-instance-123" {
			t.Errorf("GetInstanceID() = %q, want %q", synth.GetInstanceID(), "test-instance-123")
		}
	})
}

func TestSynthesisOrchestrator_IssuesFound(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial issues is nil", func(t *testing.T) {
		if synth.GetIssuesFound() != nil {
			t.Error("GetIssuesFound() should return nil when no issues")
		}
	})

	t.Run("setIssuesFound and GetIssuesFound", func(t *testing.T) {
		issues := []RevisionIssue{
			{TaskID: "task-1", Description: "Bug in module A", Severity: "critical"},
			{TaskID: "task-2", Description: "Missing validation", Severity: "major"},
		}
		synth.setIssuesFound(issues)

		got := synth.GetIssuesFound()
		if len(got) != 2 {
			t.Errorf("GetIssuesFound() len = %d, want 2", len(got))
		}
		if got[0].TaskID != "task-1" {
			t.Errorf("GetIssuesFound()[0].TaskID = %q, want %q", got[0].TaskID, "task-1")
		}
	})

	t.Run("GetIssuesFound returns a copy", func(t *testing.T) {
		issues := []RevisionIssue{
			{TaskID: "task-1", Description: "Test issue"},
		}
		synth.setIssuesFound(issues)

		got1 := synth.GetIssuesFound()
		got2 := synth.GetIssuesFound()

		// Modify got1 - should not affect got2
		got1[0].Description = "modified"
		if got2[0].Description == "modified" {
			t.Error("GetIssuesFound() should return independent copies")
		}
	})
}

func TestSynthesisOrchestrator_CompletionFile(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("initial completion file is nil", func(t *testing.T) {
		if synth.GetCompletionFile() != nil {
			t.Error("GetCompletionFile() should return nil initially")
		}
	})

	t.Run("setCompletionFile and GetCompletionFile", func(t *testing.T) {
		completion := &SynthesisCompletionFile{
			Status:           "complete",
			RevisionRound:    1,
			IntegrationNotes: "All modules integrate correctly",
			Recommendations:  []string{"Run full test suite"},
		}
		synth.setCompletionFile(completion)

		got := synth.GetCompletionFile()
		if got == nil {
			t.Fatal("GetCompletionFile() should return non-nil after setting")
		}
		if got.Status != "complete" {
			t.Errorf("GetCompletionFile().Status = %q, want %q", got.Status, "complete")
		}
		if got.IntegrationNotes != "All modules integrate correctly" {
			t.Errorf("GetCompletionFile().IntegrationNotes = %q, want %q",
				got.IntegrationNotes, "All modules integrate correctly")
		}
	})

	t.Run("GetCompletionFile returns a copy", func(t *testing.T) {
		completion := &SynthesisCompletionFile{Status: "complete"}
		synth.setCompletionFile(completion)

		got1 := synth.GetCompletionFile()
		got2 := synth.GetCompletionFile()

		// Modify got1 - should not affect got2
		got1.Status = "modified"
		if got2.Status == "modified" {
			t.Error("GetCompletionFile() should return independent copies")
		}
	})
}

func TestSynthesisOrchestrator_NeedsRevision(t *testing.T) {
	tests := []struct {
		name     string
		issues   []RevisionIssue
		expected bool
	}{
		{
			name:     "no issues returns false",
			issues:   nil,
			expected: false,
		},
		{
			name:     "empty issues returns false",
			issues:   []RevisionIssue{},
			expected: false,
		},
		{
			name: "only minor issues returns false",
			issues: []RevisionIssue{
				{TaskID: "task-1", Severity: "minor"},
				{TaskID: "task-2", Severity: "minor"},
			},
			expected: false,
		},
		{
			name: "critical issue returns true",
			issues: []RevisionIssue{
				{TaskID: "task-1", Severity: "critical"},
			},
			expected: true,
		},
		{
			name: "major issue returns true",
			issues: []RevisionIssue{
				{TaskID: "task-1", Severity: "major"},
			},
			expected: true,
		},
		{
			name: "unspecified severity returns true",
			issues: []RevisionIssue{
				{TaskID: "task-1", Severity: ""},
			},
			expected: true,
		},
		{
			name: "mixed severities with critical returns true",
			issues: []RevisionIssue{
				{TaskID: "task-1", Severity: "minor"},
				{TaskID: "task-2", Severity: "critical"},
				{TaskID: "task-3", Severity: "minor"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synth, err := NewSynthesisOrchestrator(&PhaseContext{
				Manager:      &mockManager{},
				Orchestrator: &mockOrchestrator{},
				Session:      &mockSession{},
			})
			if err != nil {
				t.Fatalf("failed to create orchestrator: %v", err)
			}

			synth.setIssuesFound(tt.issues)

			if synth.NeedsRevision() != tt.expected {
				t.Errorf("NeedsRevision() = %v, want %v", synth.NeedsRevision(), tt.expected)
			}
		})
	}
}

func TestSynthesisOrchestrator_GetIssuesNeedingRevision(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("filters to critical/major/unspecified issues", func(t *testing.T) {
		allIssues := []RevisionIssue{
			{TaskID: "task-1", Severity: "critical", Description: "Critical bug"},
			{TaskID: "task-2", Severity: "major", Description: "Major issue"},
			{TaskID: "task-3", Severity: "minor", Description: "Minor issue"},
			{TaskID: "task-4", Severity: "", Description: "Unspecified severity"},
		}
		synth.setIssuesFound(allIssues)

		filtered := synth.GetIssuesNeedingRevision()
		if len(filtered) != 3 {
			t.Errorf("GetIssuesNeedingRevision() len = %d, want 3", len(filtered))
		}

		// Check that minor issue is excluded
		for _, issue := range filtered {
			if issue.Severity == "minor" {
				t.Error("GetIssuesNeedingRevision() should not include minor issues")
			}
		}
	})

	t.Run("empty when all minor", func(t *testing.T) {
		synth.setIssuesFound([]RevisionIssue{
			{TaskID: "task-1", Severity: "minor"},
		})

		filtered := synth.GetIssuesNeedingRevision()
		if len(filtered) != 0 {
			t.Errorf("GetIssuesNeedingRevision() len = %d, want 0", len(filtered))
		}
	})
}

func TestSynthesisOrchestrator_Reset(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Set up some state
	synth.setInstanceID("test-instance")
	synth.SetAwaitingApproval(true)
	synth.SetRevisionRound(2)
	synth.setIssuesFound([]RevisionIssue{{TaskID: "task-1"}})
	synth.setCompletionFile(&SynthesisCompletionFile{Status: "complete"})

	// Call Execute to set up ctx/cancel
	_ = synth.Execute(context.Background())
	synth.Cancel()

	// Reset
	synth.Reset()

	// Verify state is cleared
	state := synth.State()
	if state.InstanceID != "" {
		t.Errorf("After Reset, InstanceID = %q, want empty", state.InstanceID)
	}
	if state.AwaitingApproval {
		t.Error("After Reset, AwaitingApproval = true, want false")
	}
	if state.RevisionRound != 0 {
		t.Errorf("After Reset, RevisionRound = %d, want 0", state.RevisionRound)
	}
	if len(state.IssuesFound) != 0 {
		t.Errorf("After Reset, IssuesFound len = %d, want 0", len(state.IssuesFound))
	}
	if state.CompletionFile != nil {
		t.Error("After Reset, CompletionFile should be nil")
	}

	// Verify that Cancel can be called again without panic
	synth.Cancel()
}

func TestSynthesisOrchestrator_ConcurrentAccess(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Test concurrent access to state methods
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			synth.SetAwaitingApproval(true)
			_ = synth.IsAwaitingApproval()
			synth.SetAwaitingApproval(false)
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			synth.SetRevisionRound(i)
			_ = synth.GetRevisionRound()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = synth.State()
			_ = synth.GetInstanceID()
			_ = synth.GetIssuesFound()
			_ = synth.GetCompletionFile()
		}
		done <- struct{}{}
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("concurrent access test timed out")
		}
	}
}

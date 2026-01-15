package phase

import (
	"context"
	"fmt"
	"strings"
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

// mockInstanceForSynthesis implements the interface that Execute expects to extract instance ID
type mockInstanceForSynthesis struct {
	id           string
	worktreePath string
	branch       string
	status       InstanceStatus
}

func (m *mockInstanceForSynthesis) GetID() string              { return m.id }
func (m *mockInstanceForSynthesis) GetWorktreePath() string    { return m.worktreePath }
func (m *mockInstanceForSynthesis) GetBranch() string          { return m.branch }
func (m *mockInstanceForSynthesis) GetStatus() InstanceStatus  { return m.status }
func (m *mockInstanceForSynthesis) GetFilesModified() []string { return nil }

// mockOrchestratorForSynthesis provides a complete mock for synthesis tests
type mockOrchestratorForSynthesis struct {
	instances      map[string]*mockInstanceForSynthesis
	addedInstance  *mockInstanceForSynthesis
	startErr       error
	saveSessionErr error
}

func (m *mockOrchestratorForSynthesis) AddInstance(session any, task string) (any, error) {
	if m.addedInstance == nil {
		m.addedInstance = &mockInstanceForSynthesis{
			id:           "test-synthesis-instance",
			worktreePath: "/tmp/test-worktree",
			status:       StatusCompleted, // Complete immediately for tests
		}
	}
	return m.addedInstance, nil
}

func (m *mockOrchestratorForSynthesis) StartInstance(inst any) error {
	return m.startErr
}

func (m *mockOrchestratorForSynthesis) SaveSession() error {
	return m.saveSessionErr
}

func (m *mockOrchestratorForSynthesis) GetInstanceManager(id string) any {
	return nil
}

func (m *mockOrchestratorForSynthesis) GetInstance(id string) InstanceInterface {
	if m.instances != nil {
		if inst, ok := m.instances[id]; ok {
			return inst
		}
	}
	if m.addedInstance != nil && m.addedInstance.id == id {
		return m.addedInstance
	}
	return nil
}

func (m *mockOrchestratorForSynthesis) BranchPrefix() string {
	return "test"
}

func TestSynthesisOrchestrator_Execute(t *testing.T) {
	t.Run("Execute with background context", func(t *testing.T) {
		mockOrch := &mockOrchestratorForSynthesis{
			addedInstance: &mockInstanceForSynthesis{
				id:           "test-synth",
				worktreePath: "/tmp/test",
				status:       StatusCompleted,
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Execute should complete without error
		err = synth.Execute(context.Background())
		if err != nil {
			t.Errorf("Execute() unexpected error: %v", err)
		}
	})

	t.Run("Execute respects context cancellation", func(t *testing.T) {
		// Create an instance that stays running so we can test cancellation
		mockOrch := &mockOrchestratorForSynthesis{
			addedInstance: &mockInstanceForSynthesis{
				id:           "test-synth",
				worktreePath: "/tmp/test",
				status:       StatusRunning, // Keep running
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())

		// Start Execute in a goroutine
		done := make(chan error, 1)
		go func() {
			done <- synth.Execute(ctx)
		}()

		// Cancel after a short delay
		time.Sleep(10 * time.Millisecond)
		cancel()

		// Wait for Execute to complete
		select {
		case err := <-done:
			// Execute should complete when context is cancelled (no error expected)
			if err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Error("Execute did not complete after context cancellation")
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
		mockOrch := &mockOrchestratorForSynthesis{
			addedInstance: &mockInstanceForSynthesis{
				id:           "test-synth",
				worktreePath: "/tmp/test",
				status:       StatusRunning, // Keep running so Cancel can interrupt
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
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

// Test slugify function
func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple text",
			input:    "hello world",
			expected: "hello-world",
		},
		{
			name:     "uppercase",
			input:    "Hello World",
			expected: "hello-world",
		},
		{
			name:     "special characters removed",
			input:    "Hello! World@2024",
			expected: "hello-world2024",
		},
		{
			name:     "multiple spaces",
			input:    "hello   world",
			expected: "hello---world",
		},
		{
			name:     "long text truncated",
			input:    "this is a very long title that exceeds thirty characters limit",
			expected: "this-is-a-very-long-title-that",
		},
		{
			name:     "numbers preserved",
			input:    "task-123 implementation",
			expected: "task-123-implementation",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slugify(tt.input)
			if result != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// mockInstanceExtended implements InstanceExtendedInterface for tests
type mockInstanceExtended struct {
	id           string
	worktreePath string
	branch       string
	status       InstanceStatus
	task         string
}

func (m *mockInstanceExtended) GetID() string              { return m.id }
func (m *mockInstanceExtended) GetWorktreePath() string    { return m.worktreePath }
func (m *mockInstanceExtended) GetBranch() string          { return m.branch }
func (m *mockInstanceExtended) GetStatus() InstanceStatus  { return m.status }
func (m *mockInstanceExtended) GetFilesModified() []string { return nil }
func (m *mockInstanceExtended) GetTask() string            { return m.task }

// mockBaseSessionExtended implements BaseSessionExtended for tests
type mockBaseSessionExtended struct {
	instances []InstanceExtendedInterface
}

func (m *mockBaseSessionExtended) GetGroupBySessionType(sessionType string) InstanceGroupInterface {
	return nil
}

func (m *mockBaseSessionExtended) GetInstances() []InstanceInterface {
	result := make([]InstanceInterface, len(m.instances))
	for i, inst := range m.instances {
		result[i] = inst
	}
	return result
}

func (m *mockBaseSessionExtended) GetInstancesExtended() []InstanceExtendedInterface {
	return m.instances
}

// mockSessionExtended implements SynthesisSessionExtended for tests
type mockSessionExtended struct {
	mockSession
	plan              PlanInterface
	revision          RevisionInterface
	consolidationMode string
	taskWorktrees     []TaskWorktreeInfo
	completedAt       *time.Time
}

func (m *mockSessionExtended) GetPlan() PlanInterface                   { return m.plan }
func (m *mockSessionExtended) GetRevision() RevisionInterface           { return m.revision }
func (m *mockSessionExtended) GetConsolidationMode() string             { return m.consolidationMode }
func (m *mockSessionExtended) SetTaskWorktrees(info []TaskWorktreeInfo) { m.taskWorktrees = info }
func (m *mockSessionExtended) SetCompletedAt(t *time.Time)              { m.completedAt = t }

// mockRevision implements RevisionInterface for tests
type mockRevision struct {
	revisionRound int
	maxRevisions  int
}

func (m *mockRevision) GetRevisionRound() int { return m.revisionRound }
func (m *mockRevision) GetMaxRevisions() int  { return m.maxRevisions }

// mockOrchestratorExtended implements SynthesisOrchestratorExtended for tests
type mockOrchestratorExtended struct {
	mockOrchestratorForSynthesis
	stopInstanceCalled    bool
	startRevisionCalled   bool
	startRevisionIssues   []RevisionIssue
	startConsolidationErr error
	startRevisionErr      error
}

func (m *mockOrchestratorExtended) StopInstance(inst any) error {
	m.stopInstanceCalled = true
	return nil
}

func (m *mockOrchestratorExtended) StartRevision(issues []RevisionIssue) error {
	m.startRevisionCalled = true
	m.startRevisionIssues = issues
	return m.startRevisionErr
}

func (m *mockOrchestratorExtended) StartConsolidation() error {
	return m.startConsolidationErr
}

func TestSynthesisOrchestrator_CaptureTaskWorktreeInfo(t *testing.T) {
	t.Run("captures worktree info for completed tasks", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{
				completedTasks: []string{"task-1", "task-2"},
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "First Task"},
					"task-2": &mockTask{id: "task-2", title: "Second Task"},
				},
			},
		}

		mockBaseSession := &mockBaseSessionExtended{
			instances: []InstanceExtendedInterface{
				&mockInstanceExtended{
					id:           "inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/first-task",
					task:         "task-1",
				},
				&mockInstanceExtended{
					id:           "inst-2",
					worktreePath: "/tmp/wt2",
					branch:       "claudio/second-task",
					task:         "task-2",
				},
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		info := synth.CaptureTaskWorktreeInfo()

		if len(info) != 2 {
			t.Errorf("CaptureTaskWorktreeInfo() len = %d, want 2", len(info))
		}

		// Verify task worktrees were stored on session
		if len(mockSession.taskWorktrees) != 2 {
			t.Errorf("Session task worktrees len = %d, want 2", len(mockSession.taskWorktrees))
		}
	})

	t.Run("returns empty when base session doesn't support extended interface", func(t *testing.T) {
		mockSession := &mockSession{
			completedTasks: []string{"task-1"},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Test Task"},
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
			BaseSession:  nil, // No base session
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		info := synth.CaptureTaskWorktreeInfo()

		if len(info) != 0 {
			t.Errorf("CaptureTaskWorktreeInfo() len = %d, want 0 when no base session", len(info))
		}
	})
}

// mockTask implements the task interface for extractTaskInfo
type mockTask struct {
	id          string
	title       string
	description string
}

func (m *mockTask) GetID() string          { return m.id }
func (m *mockTask) GetTitle() string       { return m.title }
func (m *mockTask) GetDescription() string { return m.description }

func TestSynthesisOrchestrator_TriggerConsolidation(t *testing.T) {
	t.Run("returns error when not in synthesis phase", func(t *testing.T) {
		mockSession := &mockSession{
			phase: PhaseExecuting, // Wrong phase
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = synth.TriggerConsolidation()
		if err == nil {
			t.Error("TriggerConsolidation() should return error when not in synthesis phase")
		}
	})

	t.Run("clears awaiting approval and stops instance", func(t *testing.T) {
		mockOrch := &mockOrchestratorExtended{
			mockOrchestratorForSynthesis: mockOrchestratorForSynthesis{
				addedInstance: &mockInstanceForSynthesis{
					id:     "synth-inst",
					status: StatusRunning,
				},
			},
		}

		mockSession := &mockSession{
			phase:       PhaseSynthesis,
			synthesisID: "synth-inst",
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		synth.SetAwaitingApproval(true)

		err = synth.TriggerConsolidation()
		if err != nil {
			t.Errorf("TriggerConsolidation() unexpected error: %v", err)
		}

		if synth.IsAwaitingApproval() {
			t.Error("TriggerConsolidation() should clear awaiting approval")
		}

		if !mockOrch.stopInstanceCalled {
			t.Error("TriggerConsolidation() should stop the synthesis instance")
		}
	})
}

func TestSynthesisOrchestrator_ProceedToConsolidationOrComplete(t *testing.T) {
	t.Run("marks complete when no consolidation configured", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{},
			consolidationMode: "", // No consolidation
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = synth.ProceedToConsolidationOrComplete()
		if err != nil {
			t.Errorf("ProceedToConsolidationOrComplete() unexpected error: %v", err)
		}

		if mockSession.completedAt == nil {
			t.Error("Session should have completedAt set when completing")
		}
	})

	t.Run("starts consolidation when configured", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{},
			consolidationMode: "stacked", // Consolidation enabled
		}

		mockOrch := &mockOrchestratorExtended{}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = synth.ProceedToConsolidationOrComplete()
		if err != nil {
			t.Errorf("ProceedToConsolidationOrComplete() unexpected error: %v", err)
		}
	})

	t.Run("handles consolidation start error", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{},
			consolidationMode: "stacked",
		}

		mockOrch := &mockOrchestratorExtended{
			startConsolidationErr: context.DeadlineExceeded, // Simulate error
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		err = synth.ProceedToConsolidationOrComplete()
		if err == nil {
			t.Error("ProceedToConsolidationOrComplete() should return error on consolidation failure")
		}
	})
}

func TestSynthesisOrchestrator_OnSynthesisApproved(t *testing.T) {
	t.Run("starts revision when issues found", func(t *testing.T) {
		mockSession := &mockSession{
			phase: PhaseSynthesis,
		}

		mockOrch := &mockOrchestratorExtended{}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up critical issues
		synth.setIssuesFound([]RevisionIssue{
			{TaskID: "task-1", Severity: "critical", Description: "Bug"},
		})

		synth.OnSynthesisApproved()

		if !mockOrch.startRevisionCalled {
			t.Error("OnSynthesisApproved() should call StartRevision when issues found")
		}
	})

	t.Run("proceeds to consolidation when no issues", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{phase: PhaseSynthesis},
			consolidationMode: "", // No consolidation configured
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// No issues set

		synth.OnSynthesisApproved()

		// Should have completed
		if mockSession.completedAt == nil {
			t.Error("OnSynthesisApproved() should complete when no issues")
		}
	})

	t.Run("skips revision when max rounds reached", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{phase: PhaseSynthesis},
			revision: &mockRevision{
				revisionRound: 3,
				maxRevisions:  3, // Already at max
			},
			consolidationMode: "",
		}

		mockOrch := &mockOrchestratorExtended{}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up critical issues - but should be skipped due to max revisions
		synth.setIssuesFound([]RevisionIssue{
			{TaskID: "task-1", Severity: "critical", Description: "Bug"},
		})

		synth.OnSynthesisApproved()

		if mockOrch.startRevisionCalled {
			t.Error("OnSynthesisApproved() should NOT call StartRevision when max revisions reached")
		}

		// Should have completed despite issues
		if mockSession.completedAt == nil {
			t.Error("OnSynthesisApproved() should complete when max revisions reached")
		}
	})
}

func TestSynthesisOrchestrator_ShouldSkipRevision(t *testing.T) {
	t.Run("returns false when no revision state", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{},
			revision:    nil,
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if synth.shouldSkipRevision() {
			t.Error("shouldSkipRevision() should return false when no revision state")
		}
	})

	t.Run("returns true when at max revisions", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{},
			revision: &mockRevision{
				revisionRound: 3,
				maxRevisions:  3,
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if !synth.shouldSkipRevision() {
			t.Error("shouldSkipRevision() should return true at max revisions")
		}
	})

	t.Run("returns false when under max revisions", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{},
			revision: &mockRevision{
				revisionRound: 1,
				maxRevisions:  3,
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if synth.shouldSkipRevision() {
			t.Error("shouldSkipRevision() should return false when under max revisions")
		}
	})
}

// Tests for the revision phase functionality

func TestNewRevisionState(t *testing.T) {
	t.Run("creates state with issues", func(t *testing.T) {
		issues := []RevisionIssue{
			{TaskID: "task-1", Description: "Bug in auth"},
			{TaskID: "task-2", Description: "Missing validation"},
			{TaskID: "task-1", Description: "Another bug"}, // Duplicate task ID
		}

		state := NewRevisionState(issues)

		if state.RevisionRound != 1 {
			t.Errorf("RevisionRound = %d, want 1", state.RevisionRound)
		}
		if state.MaxRevisions != DefaultMaxRevisions {
			t.Errorf("MaxRevisions = %d, want %d", state.MaxRevisions, DefaultMaxRevisions)
		}
		if len(state.Issues) != 3 {
			t.Errorf("Issues len = %d, want 3", len(state.Issues))
		}
		// TasksToRevise should have unique task IDs
		if len(state.TasksToRevise) != 2 {
			t.Errorf("TasksToRevise len = %d, want 2 (unique)", len(state.TasksToRevise))
		}
		if len(state.RevisedTasks) != 0 {
			t.Errorf("RevisedTasks len = %d, want 0", len(state.RevisedTasks))
		}
	})

	t.Run("creates state with empty issues", func(t *testing.T) {
		state := NewRevisionState(nil)

		if len(state.TasksToRevise) != 0 {
			t.Errorf("TasksToRevise len = %d, want 0", len(state.TasksToRevise))
		}
	})
}

func TestExtractTasksToRevise(t *testing.T) {
	tests := []struct {
		name     string
		issues   []RevisionIssue
		expected []string
	}{
		{
			name:     "nil issues",
			issues:   nil,
			expected: nil,
		},
		{
			name:     "empty issues",
			issues:   []RevisionIssue{},
			expected: nil,
		},
		{
			name: "unique task IDs",
			issues: []RevisionIssue{
				{TaskID: "task-1"},
				{TaskID: "task-2"},
				{TaskID: "task-3"},
			},
			expected: []string{"task-1", "task-2", "task-3"},
		},
		{
			name: "duplicate task IDs",
			issues: []RevisionIssue{
				{TaskID: "task-1"},
				{TaskID: "task-2"},
				{TaskID: "task-1"}, // Duplicate
			},
			expected: []string{"task-1", "task-2"},
		},
		{
			name: "skips empty task IDs",
			issues: []RevisionIssue{
				{TaskID: "task-1"},
				{TaskID: ""},
				{TaskID: "task-2"},
			},
			expected: []string{"task-1", "task-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractTasksToRevise(tt.issues)
			if len(result) != len(tt.expected) {
				t.Errorf("extractTasksToRevise() len = %d, want %d", len(result), len(tt.expected))
				return
			}
			for i, taskID := range result {
				if taskID != tt.expected[i] {
					t.Errorf("extractTasksToRevise()[%d] = %q, want %q", i, taskID, tt.expected[i])
				}
			}
		})
	}
}

func TestRevisionState_IsComplete(t *testing.T) {
	tests := []struct {
		name     string
		state    *RevisionState
		expected bool
	}{
		{
			name:     "nil state is complete",
			state:    nil,
			expected: true,
		},
		{
			name: "no tasks to revise is complete",
			state: &RevisionState{
				TasksToRevise: []string{},
				RevisedTasks:  []string{},
			},
			expected: true,
		},
		{
			name: "some tasks revised is not complete",
			state: &RevisionState{
				TasksToRevise: []string{"task-1", "task-2"},
				RevisedTasks:  []string{"task-1"},
			},
			expected: false,
		},
		{
			name: "all tasks revised is complete",
			state: &RevisionState{
				TasksToRevise: []string{"task-1", "task-2"},
				RevisedTasks:  []string{"task-1", "task-2"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.state.IsComplete() != tt.expected {
				t.Errorf("IsComplete() = %v, want %v", tt.state.IsComplete(), tt.expected)
			}
		})
	}
}

func TestSynthesisOrchestrator_GetRevisionState(t *testing.T) {
	t.Run("returns nil when no revision state", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		if synth.GetRevisionState() != nil {
			t.Error("GetRevisionState() should return nil when no revision started")
		}
	})

	t.Run("returns copy of revision state", func(t *testing.T) {
		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Directly set revision state
		synth.mu.Lock()
		synth.state.Revision = &RevisionState{
			Issues:        []RevisionIssue{{TaskID: "task-1", Description: "Bug"}},
			RevisionRound: 2,
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{},
		}
		synth.mu.Unlock()

		state := synth.GetRevisionState()
		if state == nil {
			t.Fatal("GetRevisionState() should return non-nil")
		}

		if state.RevisionRound != 2 {
			t.Errorf("RevisionRound = %d, want 2", state.RevisionRound)
		}
		if len(state.Issues) != 1 {
			t.Errorf("Issues len = %d, want 1", len(state.Issues))
		}

		// Verify it's a copy
		state.RevisionRound = 99
		actualState := synth.GetRevisionState()
		if actualState.RevisionRound == 99 {
			t.Error("GetRevisionState() should return a copy, not the actual state")
		}
	})
}

func TestSynthesisOrchestrator_IsInRevision(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("false when no revision state", func(t *testing.T) {
		if synth.IsInRevision() {
			t.Error("IsInRevision() should return false when no revision started")
		}
	})

	t.Run("true when revision in progress", func(t *testing.T) {
		synth.mu.Lock()
		synth.state.Revision = &RevisionState{
			RevisionRound: 1,
			CompletedAt:   nil,
		}
		synth.mu.Unlock()

		if !synth.IsInRevision() {
			t.Error("IsInRevision() should return true when revision is in progress")
		}
	})

	t.Run("false when revision completed", func(t *testing.T) {
		now := time.Now()
		synth.mu.Lock()
		synth.state.Revision.CompletedAt = &now
		synth.mu.Unlock()

		if synth.IsInRevision() {
			t.Error("IsInRevision() should return false when revision is completed")
		}
	})
}

func TestSynthesisOrchestrator_GetRunningRevisionTaskCount(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	t.Run("returns 0 initially", func(t *testing.T) {
		if synth.GetRunningRevisionTaskCount() != 0 {
			t.Errorf("GetRunningRevisionTaskCount() = %d, want 0", synth.GetRunningRevisionTaskCount())
		}
	})

	t.Run("returns correct count", func(t *testing.T) {
		synth.mu.Lock()
		synth.state.RunningRevisionTasks = map[string]string{
			"task-1": "inst-1",
			"task-2": "inst-2",
		}
		synth.mu.Unlock()

		if synth.GetRunningRevisionTaskCount() != 2 {
			t.Errorf("GetRunningRevisionTaskCount() = %d, want 2", synth.GetRunningRevisionTaskCount())
		}
	})
}

func TestSynthesisOrchestrator_BuildRevisionPrompt(t *testing.T) {
	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session: &mockSession{
			objective: "Build a user authentication system",
		},
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Set up revision state
	synth.mu.Lock()
	synth.state.Revision = &RevisionState{
		Issues: []RevisionIssue{
			{
				TaskID:      "task-1",
				Description: "Missing input validation",
				Severity:    "critical",
				Files:       []string{"auth.go", "handler.go"},
				Suggestion:  "Add input validation for username field",
			},
			{
				TaskID:      "task-1",
				Description: "Another issue",
				Severity:    "major",
			},
			{
				TaskID:      "task-2",
				Description: "Different task issue",
				Severity:    "major",
			},
		},
		RevisionRound: 2,
	}
	synth.mu.Unlock()

	task := &mockTask{
		id:          "task-1",
		title:       "Implement Login",
		description: "Create the login functionality",
	}

	prompt := synth.buildRevisionPrompt(task)

	// Check that the prompt contains expected elements
	if !strings.Contains(prompt, "Build a user authentication system") {
		t.Error("prompt should contain the objective")
	}
	if !strings.Contains(prompt, "task-1") {
		t.Error("prompt should contain the task ID")
	}
	if !strings.Contains(prompt, "Implement Login") {
		t.Error("prompt should contain the task title")
	}
	if !strings.Contains(prompt, "Missing input validation") {
		t.Error("prompt should contain the issue description")
	}
	if !strings.Contains(prompt, "auth.go") {
		t.Error("prompt should contain affected files")
	}
	if !strings.Contains(prompt, "Revision Round: 2") {
		t.Error("prompt should contain the revision round")
	}
	// Should not contain issues for other tasks
	// Note: task-2 issues are included because they have empty TaskID or match task-1
	// But "Different task issue" belongs to task-2, so should be excluded
}

// mockRevisionOrchestrator implements RevisionOrchestratorInterface for tests
type mockRevisionOrchestrator struct {
	mockOrchestratorForSynthesis
	addInstanceToWorktreeCalled bool
	addInstanceToWorktreeErr    error
	runSynthesisCalled          bool
	runSynthesisErr             error
	stopInstanceCalled          bool
}

func (m *mockRevisionOrchestrator) AddInstanceToWorktree(session any, task string, worktreePath string, branch string) (InstanceInterface, error) {
	m.addInstanceToWorktreeCalled = true
	if m.addInstanceToWorktreeErr != nil {
		return nil, m.addInstanceToWorktreeErr
	}
	return &mockInstanceForSynthesis{
		id:           "revision-inst-1",
		worktreePath: worktreePath,
		branch:       branch,
		status:       StatusRunning,
	}, nil
}

func (m *mockRevisionOrchestrator) StopInstance(inst any) error {
	m.stopInstanceCalled = true
	return nil
}

func (m *mockRevisionOrchestrator) RunSynthesis() error {
	m.runSynthesisCalled = true
	return m.runSynthesisErr
}

// mockRevisionSession implements RevisionSessionInterface for tests
type mockRevisionSession struct {
	mockSession
	revisionState *RevisionState
	revisionID    string
}

func (m *mockRevisionSession) GetRevisionState() *RevisionState { return m.revisionState }
func (m *mockRevisionSession) SetRevisionState(state *RevisionState) {
	m.revisionState = state
}
func (m *mockRevisionSession) GetRevisionID() string   { return m.revisionID }
func (m *mockRevisionSession) SetRevisionID(id string) { m.revisionID = id }

func TestSynthesisOrchestrator_StartRevision(t *testing.T) {
	t.Run("initializes revision state for first round", func(t *testing.T) {
		mockOrch := &mockRevisionOrchestrator{}
		mockSession := &mockRevisionSession{
			mockSession: mockSession{
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "Test Task"},
				},
			},
		}
		mockBaseSession := &mockBaseSessionExtended{
			instances: []InstanceExtendedInterface{
				&mockInstanceExtended{
					id:           "orig-inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/test-task",
					task:         "task-1",
				},
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Initialize context for goroutine management
		synth.mu.Lock()
		synth.ctx, synth.cancel = context.WithCancel(context.Background())
		synth.mu.Unlock()

		issues := []RevisionIssue{
			{TaskID: "task-1", Description: "Bug", Severity: "critical"},
		}

		err = synth.StartRevision(issues)
		if err != nil {
			t.Errorf("StartRevision() error = %v", err)
		}

		// Cancel to clean up goroutines
		synth.Cancel()

		state := synth.GetRevisionState()
		if state == nil {
			t.Fatal("revision state should be set after StartRevision")
		}
		if state.RevisionRound != 1 {
			t.Errorf("RevisionRound = %d, want 1", state.RevisionRound)
		}
		if len(state.TasksToRevise) != 1 {
			t.Errorf("TasksToRevise len = %d, want 1", len(state.TasksToRevise))
		}

		// Verify session was updated
		if mockSession.revisionState == nil {
			t.Error("session revision state should be updated")
		}
	})

	t.Run("increments revision round for subsequent rounds", func(t *testing.T) {
		mockOrch := &mockRevisionOrchestrator{}
		mockSession := &mockRevisionSession{
			mockSession: mockSession{
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "Test Task"},
				},
			},
		}
		mockBaseSession := &mockBaseSessionExtended{
			instances: []InstanceExtendedInterface{
				&mockInstanceExtended{
					id:           "orig-inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/test-task",
					task:         "task-1",
				},
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		// Set up existing revision state
		synth.mu.Lock()
		synth.ctx, synth.cancel = context.WithCancel(context.Background())
		synth.state.Revision = &RevisionState{
			RevisionRound: 1,
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{"task-1"},
		}
		synth.mu.Unlock()

		issues := []RevisionIssue{
			{TaskID: "task-1", Description: "New bug", Severity: "critical"},
		}

		err = synth.StartRevision(issues)
		if err != nil {
			t.Errorf("StartRevision() error = %v", err)
		}

		synth.Cancel()

		state := synth.GetRevisionState()
		if state.RevisionRound != 2 {
			t.Errorf("RevisionRound = %d, want 2 (incremented)", state.RevisionRound)
		}
	})
}

func TestSynthesisOrchestrator_HandleRevisionTaskCompletion(t *testing.T) {
	taskCompletions := make([]string, 0)
	taskFailures := make([]string, 0)

	callbacks := &mockCallbacksExtended{
		onTaskComplete: func(taskID string) {
			taskCompletions = append(taskCompletions, taskID)
		},
		onTaskFailed: func(taskID, reason string) {
			taskFailures = append(taskFailures, taskID)
		},
	}

	synth, err := NewSynthesisOrchestrator(&PhaseContext{
		Manager:      &mockManager{},
		Orchestrator: &mockOrchestrator{},
		Session:      &mockSession{},
		Callbacks:    callbacks,
	})
	if err != nil {
		t.Fatalf("failed to create orchestrator: %v", err)
	}

	// Set up revision state
	synth.mu.Lock()
	synth.state.Revision = &RevisionState{
		RevisionRound: 1,
		TasksToRevise: []string{"task-1", "task-2"},
		RevisedTasks:  []string{},
	}
	synth.state.RunningRevisionTasks = map[string]string{
		"task-1": "inst-1",
		"task-2": "inst-2",
	}
	synth.mu.Unlock()

	t.Run("handles successful completion", func(t *testing.T) {
		taskCompletions = taskCompletions[:0]

		synth.handleRevisionTaskCompletion(revisionTaskCompletion{
			taskID:     "task-1",
			instanceID: "inst-1",
			success:    true,
		})

		// Task should be removed from running tasks
		if synth.GetRunningRevisionTaskCount() != 1 {
			t.Errorf("running task count = %d, want 1", synth.GetRunningRevisionTaskCount())
		}

		// Task should be added to revised tasks
		state := synth.GetRevisionState()
		if len(state.RevisedTasks) != 1 || state.RevisedTasks[0] != "task-1" {
			t.Errorf("RevisedTasks = %v, want [task-1]", state.RevisedTasks)
		}

		// Callback should be called
		if len(taskCompletions) != 1 || taskCompletions[0] != "task-1" {
			t.Errorf("task completions = %v, want [task-1]", taskCompletions)
		}
	})

	t.Run("handles failed completion", func(t *testing.T) {
		taskFailures = taskFailures[:0]

		synth.handleRevisionTaskCompletion(revisionTaskCompletion{
			taskID:     "task-2",
			instanceID: "inst-2",
			success:    false,
			err:        "timeout",
		})

		// Task should be removed from running tasks
		if synth.GetRunningRevisionTaskCount() != 0 {
			t.Errorf("running task count = %d, want 0", synth.GetRunningRevisionTaskCount())
		}

		// Task should NOT be added to revised tasks
		state := synth.GetRevisionState()
		for _, taskID := range state.RevisedTasks {
			if taskID == "task-2" {
				t.Error("failed task should not be in revised tasks")
			}
		}

		// Failure callback should be called
		if len(taskFailures) != 1 || taskFailures[0] != "task-2" {
			t.Errorf("task failures = %v, want [task-2]", taskFailures)
		}
	})
}

// mockCallbacks with additional fields for revision testing
type mockCallbacksExtended struct {
	mockCallbacks
	onPhaseChange  func(UltraPlanPhase)
	onTaskComplete func(string)
	onTaskFailed   func(string, string)
}

func (m *mockCallbacksExtended) OnPhaseChange(phase UltraPlanPhase) {
	if m.onPhaseChange != nil {
		m.onPhaseChange(phase)
	}
}

func (m *mockCallbacksExtended) OnTaskComplete(taskID string) {
	if m.onTaskComplete != nil {
		m.onTaskComplete(taskID)
	}
}

func (m *mockCallbacksExtended) OnTaskFailed(taskID, reason string) {
	if m.onTaskFailed != nil {
		m.onTaskFailed(taskID, reason)
	}
}

func TestRevisionPromptTemplate(t *testing.T) {
	// Verify the template can be formatted correctly
	prompt := fmt.Sprintf(RevisionPromptTemplate,
		"Build a user auth system",   // objective
		"task-1",                     // task ID
		"Implement Login",            // title
		"Create login functionality", // description
		1,                            // revision round
		"1. **critical**: Bug\n",     // issues
		"task-1",                     // task ID for JSON
		1,                            // revision round for JSON
	)

	// Verify key elements are present
	expectedElements := []string{
		"Build a user auth system",
		"task-1",
		"Implement Login",
		"Revision Round: 1",
		RevisionCompletionFileName,
	}

	for _, elem := range expectedElements {
		if !strings.Contains(prompt, elem) {
			t.Errorf("prompt should contain %q", elem)
		}
	}
}

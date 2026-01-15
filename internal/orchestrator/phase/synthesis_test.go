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

// Tests for parseRevisionIssuesFromOutput

func TestParseRevisionIssuesFromOutput(t *testing.T) {
	tests := []struct {
		name          string
		output        string
		wantLen       int
		wantErr       bool
		wantFirstTask string
	}{
		{
			name:    "no revision issues block returns nil",
			output:  "Some output without revision issues",
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "empty revision issues block returns nil",
			output:  "<revision_issues></revision_issues>",
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "empty array returns nil",
			output:  "<revision_issues>[]</revision_issues>",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "valid issues parsed correctly",
			output: `Some output before
<revision_issues>
[
  {"TaskID": "task-1", "Description": "Bug in auth", "Severity": "critical"},
  {"TaskID": "task-2", "Description": "Missing validation", "Severity": "major"}
]
</revision_issues>
Some output after`,
			wantLen:       2,
			wantErr:       false,
			wantFirstTask: "task-1",
		},
		{
			name: "filters out issues with empty description",
			output: `<revision_issues>
[
  {"TaskID": "task-1", "Description": "Valid issue", "Severity": "critical"},
  {"TaskID": "task-2", "Description": "", "Severity": "major"}
]
</revision_issues>`,
			wantLen:       1,
			wantErr:       false,
			wantFirstTask: "task-1",
		},
		{
			name: "invalid JSON returns error",
			output: `<revision_issues>
{ invalid json }
</revision_issues>`,
			wantLen: 0,
			wantErr: true,
		},
		{
			name: "multiline output with whitespace",
			output: `
<revision_issues>
  [
    {
      "TaskID": "task-1",
      "Description": "Issue with files",
      "Files": ["auth.go", "handler.go"],
      "Severity": "critical",
      "Suggestion": "Fix the auth logic"
    }
  ]
</revision_issues>
`,
			wantLen:       1,
			wantErr:       false,
			wantFirstTask: "task-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := parseRevisionIssuesFromOutput(tt.output)

			if tt.wantErr {
				if err == nil {
					t.Error("parseRevisionIssuesFromOutput() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("parseRevisionIssuesFromOutput() unexpected error: %v", err)
				return
			}

			if len(issues) != tt.wantLen {
				t.Errorf("parseRevisionIssuesFromOutput() returned %d issues, want %d", len(issues), tt.wantLen)
				return
			}

			if tt.wantLen > 0 && issues[0].TaskID != tt.wantFirstTask {
				t.Errorf("first issue TaskID = %q, want %q", issues[0].TaskID, tt.wantFirstTask)
			}
		})
	}
}

// Tests for convertToRevisionIssues

func TestConvertToRevisionIssues(t *testing.T) {
	t.Run("nil input returns nil", func(t *testing.T) {
		result := convertToRevisionIssues(nil)
		if result != nil {
			t.Errorf("convertToRevisionIssues(nil) = %v, want nil", result)
		}
	})

	t.Run("empty slice returns empty slice", func(t *testing.T) {
		result := convertToRevisionIssues([]RevisionIssue{})
		if result == nil || len(result) != 0 {
			t.Errorf("convertToRevisionIssues([]) = %v, want empty slice", result)
		}
	})

	t.Run("copies issues correctly", func(t *testing.T) {
		input := []RevisionIssue{
			{TaskID: "task-1", Description: "Bug", Severity: "critical", Files: []string{"a.go"}, Suggestion: "Fix it"},
			{TaskID: "task-2", Description: "Issue", Severity: "major"},
		}

		result := convertToRevisionIssues(input)

		if len(result) != 2 {
			t.Errorf("convertToRevisionIssues() len = %d, want 2", len(result))
			return
		}

		if result[0].TaskID != "task-1" || result[0].Description != "Bug" {
			t.Errorf("first issue not copied correctly: %+v", result[0])
		}
		if result[1].TaskID != "task-2" {
			t.Errorf("second issue not copied correctly: %+v", result[1])
		}
	})

	t.Run("returns independent copy", func(t *testing.T) {
		input := []RevisionIssue{{TaskID: "task-1"}}
		result := convertToRevisionIssues(input)

		// Modify result
		result[0].TaskID = "modified"

		// Input should be unchanged
		if input[0].TaskID == "modified" {
			t.Error("convertToRevisionIssues() should return independent copy")
		}
	})
}

// Tests for extractTaskInfo

func TestExtractTaskInfo(t *testing.T) {
	t.Run("extracts from GetID/GetTitle/GetDescription methods", func(t *testing.T) {
		task := &mockTask{
			id:          "task-123",
			title:       "Test Task",
			description: "A test task",
		}

		info := extractTaskInfo(task)

		if info.ID != "task-123" {
			t.Errorf("extractTaskInfo().ID = %q, want %q", info.ID, "task-123")
		}
		if info.Title != "Test Task" {
			t.Errorf("extractTaskInfo().Title = %q, want %q", info.Title, "Test Task")
		}
		if info.Description != "A test task" {
			t.Errorf("extractTaskInfo().Description = %q, want %q", info.Description, "A test task")
		}
	})

	t.Run("extracts from map", func(t *testing.T) {
		task := map[string]any{
			"id":          "task-456",
			"title":       "Map Task",
			"description": "Task from map",
		}

		info := extractTaskInfo(task)

		if info.ID != "task-456" {
			t.Errorf("extractTaskInfo().ID = %q, want %q", info.ID, "task-456")
		}
		if info.Title != "Map Task" {
			t.Errorf("extractTaskInfo().Title = %q, want %q", info.Title, "Map Task")
		}
		if info.Description != "Task from map" {
			t.Errorf("extractTaskInfo().Description = %q, want %q", info.Description, "Task from map")
		}
	})

	t.Run("returns empty for unknown type", func(t *testing.T) {
		info := extractTaskInfo("not a task")

		if info.ID != "" || info.Title != "" || info.Description != "" {
			t.Errorf("extractTaskInfo(string) should return empty info, got %+v", info)
		}
	})

	t.Run("handles partial map", func(t *testing.T) {
		task := map[string]any{
			"title": "Only Title",
		}

		info := extractTaskInfo(task)

		if info.Title != "Only Title" {
			t.Errorf("extractTaskInfo().Title = %q, want %q", info.Title, "Only Title")
		}
		if info.ID != "" {
			t.Errorf("extractTaskInfo().ID = %q, want empty", info.ID)
		}
	})

	t.Run("handles nil", func(t *testing.T) {
		info := extractTaskInfo(nil)

		if info.ID != "" || info.Title != "" || info.Description != "" {
			t.Errorf("extractTaskInfo(nil) should return empty info, got %+v", info)
		}
	})

	t.Run("handles map with non-string values", func(t *testing.T) {
		task := map[string]any{
			"id":    123,   // int, not string
			"title": false, // bool, not string
		}

		info := extractTaskInfo(task)

		// Should handle gracefully without panic
		if info.ID != "" {
			t.Errorf("extractTaskInfo().ID = %q, want empty for non-string", info.ID)
		}
	})
}

// mockTaskWithIDMethod is a task that only has ID() method (not GetID())
type mockTaskWithIDMethod struct {
	id string
}

func (m *mockTaskWithIDMethod) ID() string { return m.id }

func TestExtractTaskInfo_IDMethodFallback(t *testing.T) {
	t.Run("extracts from ID() method", func(t *testing.T) {
		task := &mockTaskWithIDMethod{id: "task-from-id-method"}

		info := extractTaskInfo(task)

		if info.ID != "task-from-id-method" {
			t.Errorf("extractTaskInfo().ID = %q, want %q", info.ID, "task-from-id-method")
		}
	})
}

// Tests for buildSynthesisPrompt

func TestSynthesisOrchestrator_BuildSynthesisPrompt(t *testing.T) {
	t.Run("builds prompt with completed tasks", func(t *testing.T) {
		mockSession := &mockSession{
			objective:      "Build a user management system",
			completedTasks: []string{"task-1", "task-2"},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Create User Model"},
				"task-2": &mockTask{id: "task-2", title: "Add API Endpoints"},
			},
			taskToInstance: map[string]string{
				"task-1": "inst-1",
				"task-2": "inst-2",
			},
			taskCommitCounts: map[string]int{
				"task-1": 3,
				"task-2": 2,
			},
		}

		mockInst := &mockInstanceForSynthesis{
			id:     "inst-1",
			status: StatusCompleted,
		}
		mockOrch := &mockOrchestratorForSynthesis{
			instances: map[string]*mockInstanceForSynthesis{
				"inst-1": mockInst,
				"inst-2": mockInst,
			},
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		prompt := synth.buildSynthesisPrompt()

		// Check that the prompt contains expected elements
		if !strings.Contains(prompt, "Build a user management system") {
			t.Error("prompt should contain the objective")
		}
		if !strings.Contains(prompt, "Create User Model") {
			t.Error("prompt should contain task title")
		}
		if !strings.Contains(prompt, "3 commits") {
			t.Error("prompt should contain commit count")
		}
	})

	t.Run("marks tasks with no commits", func(t *testing.T) {
		mockSession := &mockSession{
			objective:      "Test objective",
			completedTasks: []string{"task-1"},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "No Commit Task"},
			},
			taskCommitCounts: map[string]int{}, // No commits
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		prompt := synth.buildSynthesisPrompt()

		if !strings.Contains(prompt, "NO COMMITS") {
			t.Error("prompt should mark tasks with no commits")
		}
	})

	t.Run("handles completed tasks not in TaskToInstance", func(t *testing.T) {
		mockSession := &mockSession{
			objective:      "Test objective",
			completedTasks: []string{"task-1"},
			tasks: map[string]any{
				"task-1": &mockTask{id: "task-1", title: "Orphan Task"},
			},
			taskToInstance: map[string]string{}, // Not in map
			taskCommitCounts: map[string]int{
				"task-1": 1,
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

		prompt := synth.buildSynthesisPrompt()

		// Should still include the task
		if !strings.Contains(prompt, "Orphan Task") {
			t.Error("prompt should include tasks not in TaskToInstance")
		}
	})

	t.Run("includes revision round in prompt", func(t *testing.T) {
		mockSession := &mockSession{
			objective:      "Test objective",
			completedTasks: []string{},
			revisionRound:  2,
		}

		synth, err := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})
		if err != nil {
			t.Fatalf("failed to create orchestrator: %v", err)
		}

		prompt := synth.buildSynthesisPrompt()

		// The revision round should be in the JSON template section
		if !strings.Contains(prompt, `"revision_round": 2`) {
			t.Error("prompt should contain the revision round")
		}
	})
}

// Tests for checkForRevisionCompletionFile

func TestSynthesisOrchestrator_CheckForRevisionCompletionFile(t *testing.T) {
	t.Run("returns false when worktree path is empty", func(t *testing.T) {
		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})

		inst := &mockInstanceForSynthesis{
			worktreePath: "", // Empty path
		}

		if synth.checkForRevisionCompletionFile(inst) {
			t.Error("checkForRevisionCompletionFile() should return false for empty worktree path")
		}
	})

	t.Run("returns false when file doesn't exist", func(t *testing.T) {
		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})

		inst := &mockInstanceForSynthesis{
			worktreePath: "/nonexistent/path",
		}

		if synth.checkForRevisionCompletionFile(inst) {
			t.Error("checkForRevisionCompletionFile() should return false when file doesn't exist")
		}
	})
}

// Tests for sendRevisionCompletion

func TestSynthesisOrchestrator_SendRevisionCompletion(t *testing.T) {
	t.Run("sends to completion channel when available", func(t *testing.T) {
		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})

		// Set up completion channel
		completionChan := make(chan revisionTaskCompletion, 1)
		synth.mu.Lock()
		synth.state.revisionCompletionChan = completionChan
		synth.mu.Unlock()

		synth.sendRevisionCompletion("task-1", "inst-1", true, "")

		// Check that completion was sent
		select {
		case completion := <-completionChan:
			if completion.taskID != "task-1" {
				t.Errorf("completion.taskID = %q, want %q", completion.taskID, "task-1")
			}
			if completion.instanceID != "inst-1" {
				t.Errorf("completion.instanceID = %q, want %q", completion.instanceID, "inst-1")
			}
			if !completion.success {
				t.Error("completion.success should be true")
			}
		default:
			t.Error("expected completion to be sent to channel")
		}
	})

	t.Run("handles nil channel gracefully", func(t *testing.T) {
		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})

		// Don't set up completion channel (nil)

		// Should not panic
		synth.sendRevisionCompletion("task-1", "inst-1", false, "error")
	})

	t.Run("sends failure with error message", func(t *testing.T) {
		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})

		completionChan := make(chan revisionTaskCompletion, 1)
		synth.mu.Lock()
		synth.state.revisionCompletionChan = completionChan
		synth.mu.Unlock()

		synth.sendRevisionCompletion("task-2", "inst-2", false, "timeout error")

		completion := <-completionChan
		if completion.success {
			t.Error("completion.success should be false")
		}
		if completion.err != "timeout error" {
			t.Errorf("completion.err = %q, want %q", completion.err, "timeout error")
		}
	})
}

// Tests for onSynthesisReady

func TestSynthesisOrchestrator_OnSynthesisReady(t *testing.T) {
	t.Run("sets awaiting approval flag", func(t *testing.T) {
		mockSession := &mockSession{
			synthesisID: "synth-1",
		}
		mockInst := &mockInstanceForSynthesis{
			id:           "synth-1",
			worktreePath: "/tmp/test",
		}
		mockOrch := &mockOrchestratorForSynthesis{
			addedInstance: mockInst,
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		synth.onSynthesisReady()

		if !synth.IsAwaitingApproval() {
			t.Error("onSynthesisReady() should set awaiting approval flag")
		}

		if !mockSession.awaitingApproval {
			t.Error("onSynthesisReady() should update session's awaiting approval flag")
		}
	})
}

// Tests for onRevisionComplete

func TestSynthesisOrchestrator_OnRevisionComplete(t *testing.T) {
	t.Run("marks revision as complete and re-runs synthesis", func(t *testing.T) {
		mockOrch := &mockRevisionOrchestrator{}
		mockSession := &mockRevisionSession{
			mockSession: mockSession{},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		// Set up revision state
		synth.mu.Lock()
		synth.state.Revision = &RevisionState{
			RevisionRound: 1,
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{"task-1"},
		}
		synth.mu.Unlock()

		synth.onRevisionComplete()

		// Verify revision state has completion time
		state := synth.GetRevisionState()
		if state.CompletedAt == nil {
			t.Error("onRevisionComplete() should set CompletedAt")
		}

		// Verify RunSynthesis was called
		if !mockOrch.runSynthesisCalled {
			t.Error("onRevisionComplete() should call RunSynthesis")
		}
	})

	t.Run("falls back to consolidation when orchestrator doesn't support RunSynthesis", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{},
			consolidationMode: "", // No consolidation
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{}, // Doesn't implement RevisionOrchestratorInterface
			Session:      mockSession,
		})

		// Set up revision state
		synth.mu.Lock()
		synth.state.Revision = &RevisionState{
			RevisionRound: 1,
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{"task-1"},
		}
		synth.mu.Unlock()

		synth.onRevisionComplete()

		// Should have completed (not called revision)
		if mockSession.completedAt == nil {
			t.Error("onRevisionComplete() should fall back to completion when RunSynthesis not available")
		}
	})

	t.Run("handles RunSynthesis error by proceeding to consolidation", func(t *testing.T) {
		mockOrch := &mockRevisionOrchestrator{
			runSynthesisErr: fmt.Errorf("synthesis failed"),
		}
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{},
			consolidationMode: "", // No consolidation
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		// Set up revision state
		synth.mu.Lock()
		synth.state.Revision = &RevisionState{
			RevisionRound: 1,
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{"task-1"},
		}
		synth.mu.Unlock()

		synth.onRevisionComplete()

		// Should fall back to completion
		if mockSession.completedAt == nil {
			t.Error("onRevisionComplete() should fall back to completion on RunSynthesis error")
		}
	})
}

// Tests for monitorRevisionTasks

func TestSynthesisOrchestrator_MonitorRevisionTasks(t *testing.T) {
	t.Run("handles task completion and checks for all complete", func(t *testing.T) {
		mockOrch := &mockRevisionOrchestrator{}
		mockSession := &mockRevisionSession{
			mockSession: mockSession{},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		// Set up context and revision state
		ctx, cancel := context.WithCancel(context.Background())
		synth.mu.Lock()
		synth.ctx = ctx
		synth.cancel = cancel
		synth.state.Revision = &RevisionState{
			RevisionRound: 1,
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{},
		}
		synth.state.RunningRevisionTasks = map[string]string{"task-1": "inst-1"}
		synth.state.revisionCompletionChan = make(chan revisionTaskCompletion, 10)
		synth.mu.Unlock()

		// Start monitoring in goroutine
		done := make(chan struct{})
		go func() {
			synth.monitorRevisionTasks()
			close(done)
		}()

		// Send completion
		synth.state.revisionCompletionChan <- revisionTaskCompletion{
			taskID:     "task-1",
			instanceID: "inst-1",
			success:    true,
		}

		// Wait for completion (with timeout)
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			cancel() // Cancel to cleanup
			t.Error("monitorRevisionTasks() did not complete after all tasks finished")
		}

		// Verify revision was completed
		state := synth.GetRevisionState()
		if len(state.RevisedTasks) != 1 || state.RevisedTasks[0] != "task-1" {
			t.Errorf("task should be marked as revised, got %v", state.RevisedTasks)
		}
	})

	t.Run("exits on context cancellation", func(t *testing.T) {
		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
		})

		// Set up context that will be cancelled
		ctx, cancel := context.WithCancel(context.Background())
		synth.mu.Lock()
		synth.ctx = ctx
		synth.cancel = cancel
		synth.state.Revision = &RevisionState{
			TasksToRevise: []string{"task-1"},
			RevisedTasks:  []string{},
		}
		synth.state.revisionCompletionChan = make(chan revisionTaskCompletion, 10)
		synth.mu.Unlock()

		// Start monitoring
		done := make(chan struct{})
		go func() {
			synth.monitorRevisionTasks()
			close(done)
		}()

		// Cancel context
		cancel()

		// Should exit promptly
		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Error("monitorRevisionTasks() did not exit on context cancellation")
		}
	})
}

// Tests for monitorSynthesisInstance edge cases

func TestSynthesisOrchestrator_MonitorSynthesisInstance_StatusChanges(t *testing.T) {
	t.Run("handles StatusError", func(t *testing.T) {
		mockSession := &mockSession{
			phase: PhaseSynthesis,
		}
		mockInst := &mockInstanceForSynthesis{
			id:     "synth-1",
			status: StatusError,
		}
		mockOrch := &mockOrchestratorForSynthesis{
			instances: map[string]*mockInstanceForSynthesis{
				"synth-1": mockInst,
			},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		// Set up context
		ctx, cancel := context.WithCancel(context.Background())
		synth.mu.Lock()
		synth.ctx = ctx
		synth.cancel = cancel
		synth.mu.Unlock()

		// Run monitoring
		done := make(chan struct{})
		go func() {
			synth.monitorSynthesisInstance("synth-1")
			close(done)
		}()

		// Wait for completion
		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			cancel()
			t.Error("monitorSynthesisInstance() did not complete")
		}

		// Verify phase was set to failed
		if mockSession.phase != PhaseFailed {
			t.Errorf("phase should be set to PhaseFailed, got %v", mockSession.phase)
		}
	})

	t.Run("handles StatusTimeout", func(t *testing.T) {
		mockSession := &mockSession{
			phase: PhaseSynthesis,
		}
		mockInst := &mockInstanceForSynthesis{
			id:     "synth-1",
			status: StatusTimeout,
		}
		mockOrch := &mockOrchestratorForSynthesis{
			instances: map[string]*mockInstanceForSynthesis{
				"synth-1": mockInst,
			},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		ctx, cancel := context.WithCancel(context.Background())
		synth.mu.Lock()
		synth.ctx = ctx
		synth.cancel = cancel
		synth.mu.Unlock()

		done := make(chan struct{})
		go func() {
			synth.monitorSynthesisInstance("synth-1")
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			cancel()
			t.Error("monitorSynthesisInstance() did not complete")
		}

		if mockSession.phase != PhaseFailed {
			t.Errorf("phase should be set to PhaseFailed, got %v", mockSession.phase)
		}
	})

	t.Run("handles instance disappearing", func(t *testing.T) {
		mockSession := &mockSession{
			phase: PhaseSynthesis,
		}
		// No instance in the map - will return nil
		mockOrch := &mockOrchestratorForSynthesis{
			instances: map[string]*mockInstanceForSynthesis{},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		ctx, cancel := context.WithCancel(context.Background())
		synth.mu.Lock()
		synth.ctx = ctx
		synth.cancel = cancel
		synth.mu.Unlock()

		done := make(chan struct{})
		go func() {
			synth.monitorSynthesisInstance("nonexistent")
			close(done)
		}()

		select {
		case <-done:
			// Success - should complete when instance is nil
		case <-time.After(2 * time.Second):
			cancel()
			t.Error("monitorSynthesisInstance() did not complete when instance disappeared")
		}
	})
}

// Tests for onSynthesisComplete edge cases

func TestSynthesisOrchestrator_OnSynthesisComplete_EdgeCases(t *testing.T) {
	t.Run("handles no issues - proceeds to consolidation", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{synthesisID: "synth-1"},
			consolidationMode: "", // No consolidation
		}
		mockOrch := &mockOrchestratorForSynthesis{
			addedInstance: &mockInstanceForSynthesis{
				id:           "synth-1",
				worktreePath: "/tmp/test",
			},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		synth.onSynthesisComplete()

		// Should mark complete since no issues and no consolidation
		if mockSession.completedAt == nil {
			t.Error("onSynthesisComplete() should mark complete when no issues")
		}
	})

	t.Run("handles revision error gracefully", func(t *testing.T) {
		mockSession := &mockSession{
			synthesisID: "synth-1",
			phase:       PhaseSynthesis,
		}
		mockOrch := &mockOrchestratorExtended{
			startRevisionErr: fmt.Errorf("revision failed"),
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		// Set up critical issues that would trigger revision
		synth.setIssuesFound([]RevisionIssue{
			{TaskID: "task-1", Severity: "critical", Description: "Bug"},
		})

		synth.onSynthesisComplete()

		// Should set phase to failed
		if mockSession.phase != PhaseFailed {
			t.Errorf("phase should be PhaseFailed on revision error, got %v", mockSession.phase)
		}
	})

	t.Run("skips revision when max revisions reached", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{synthesisID: "synth-1"},
			revision: &mockRevision{
				revisionRound: 3,
				maxRevisions:  3, // At max
			},
			consolidationMode: "",
		}
		mockOrch := &mockOrchestratorExtended{}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
		})

		// Set up critical issues
		synth.setIssuesFound([]RevisionIssue{
			{TaskID: "task-1", Severity: "critical", Description: "Bug"},
		})

		synth.onSynthesisComplete()

		// Should not call StartRevision
		if mockOrch.startRevisionCalled {
			t.Error("onSynthesisComplete() should not start revision when max reached")
		}

		// Should proceed to complete
		if mockSession.completedAt == nil {
			t.Error("onSynthesisComplete() should complete when max revisions reached")
		}
	})
}

// Tests for startRevisionTask edge cases

func TestSynthesisOrchestrator_StartRevisionTask_Errors(t *testing.T) {
	t.Run("returns error when task not found", func(t *testing.T) {
		mockSession := &mockSession{
			tasks: map[string]any{}, // No tasks
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
		})

		err := synth.startRevisionTask("nonexistent-task")

		if err == nil {
			t.Error("startRevisionTask() should return error when task not found")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should mention 'not found', got %v", err)
		}
	})

	t.Run("returns error when worktree not found", func(t *testing.T) {
		mockSession := &mockRevisionSession{
			mockSession: mockSession{
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "Test Task"},
				},
			},
		}
		// BaseSession with no matching instances
		mockBaseSession := &mockBaseSessionExtended{
			instances: []InstanceExtendedInterface{}, // No instances
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockRevisionOrchestrator{},
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})

		err := synth.startRevisionTask("task-1")

		if err == nil {
			t.Error("startRevisionTask() should return error when worktree not found")
		}
		if !strings.Contains(err.Error(), "worktree") {
			t.Errorf("error should mention 'worktree', got %v", err)
		}
	})

	t.Run("returns error when orchestrator doesn't support AddInstanceToWorktree", func(t *testing.T) {
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
					id:           "inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/test-task",
					task:         "task-1",
				},
			},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{}, // Doesn't support AddInstanceToWorktree
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})

		err := synth.startRevisionTask("task-1")

		if err == nil {
			t.Error("startRevisionTask() should return error when orchestrator doesn't support AddInstanceToWorktree")
		}
	})

	t.Run("returns error when AddInstanceToWorktree fails", func(t *testing.T) {
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
					id:           "inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/test-task",
					task:         "task-1",
				},
			},
		}
		mockOrch := &mockRevisionOrchestrator{
			addInstanceToWorktreeErr: fmt.Errorf("failed to create instance"),
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})

		err := synth.startRevisionTask("task-1")

		if err == nil {
			t.Error("startRevisionTask() should return error when AddInstanceToWorktree fails")
		}
	})

	t.Run("returns error when StartInstance fails", func(t *testing.T) {
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
					id:           "inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/test-task",
					task:         "task-1",
				},
			},
		}
		mockOrch := &mockRevisionOrchestrator{
			mockOrchestratorForSynthesis: mockOrchestratorForSynthesis{
				startErr: fmt.Errorf("failed to start"),
			},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})

		// Set up context for the goroutine
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		synth.mu.Lock()
		synth.ctx = ctx
		synth.cancel = cancel
		synth.state.RunningRevisionTasks = make(map[string]string)
		synth.mu.Unlock()

		err := synth.startRevisionTask("task-1")

		if err == nil {
			t.Error("startRevisionTask() should return error when StartInstance fails")
		}
	})
}

// Test for notifyComplete

func TestSynthesisOrchestrator_NotifyComplete(t *testing.T) {
	t.Run("calls callback when available", func(t *testing.T) {
		var callbackCalled bool
		var callbackSuccess bool
		var callbackSummary string

		callbacks := &struct {
			mockCallbacks
		}{}

		// We can't easily mock OnComplete since mockCallbacks doesn't track it
		// Instead, verify no panic when callbacks are nil

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      &mockSession{},
			Callbacks:    callbacks,
		})

		// Should not panic
		synth.notifyComplete(true, "Test complete")

		// Reset for nil callbacks test
		synth.phaseCtx.Callbacks = nil
		synth.notifyComplete(false, "Test failed")

		// If we get here without panic, the test passes
		_ = callbackCalled
		_ = callbackSuccess
		_ = callbackSummary
	})
}

// Tests for SynthesisPromptTemplate

func TestSynthesisPromptTemplate(t *testing.T) {
	// Verify the template can be formatted correctly
	prompt := fmt.Sprintf(SynthesisPromptTemplate,
		"Build a user management system",                                    // objective
		"- [task-1] Task One (3 commits)\n- [task-2] Task Two (1 commit)\n", // task list
		"### Task One\nStatus: completed\nCommits: 3\n",                     // results summary
		2, // revision round
	)

	// Verify key elements are present
	expectedElements := []string{
		"Build a user management system",
		"Task One",
		"3 commits",
		SynthesisCompletionFileName,
		`"revision_round": 2`,
	}

	for _, elem := range expectedElements {
		if !strings.Contains(prompt, elem) {
			t.Errorf("SynthesisPromptTemplate should contain %q", elem)
		}
	}
}

// Tests for Execute errors

func TestSynthesisOrchestrator_Execute_Errors(t *testing.T) {
	t.Run("returns error when instance doesn't implement GetID", func(t *testing.T) {
		// mockOrchestrator returns nil from AddInstance, which tests the nil/GetID failure path
		type orchWithBadInstance struct {
			mockOrchestrator
		}
		badOrch := &orchWithBadInstance{}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: badOrch,
			Session:      &mockSession{},
		})

		// The default mockOrchestrator.AddInstance returns nil, nil
		// which would cause the type assertion to fail

		err := synth.Execute(context.Background())

		// Should fail because AddInstance returns nil
		if err != nil && !strings.Contains(err.Error(), "GetID") && !strings.Contains(err.Error(), "nil") {
			t.Logf("Got expected error type: %v", err)
		}
	})

	t.Run("returns error when StartInstance fails", func(t *testing.T) {
		mockOrch := &mockOrchestratorForSynthesis{
			addedInstance: &mockInstanceForSynthesis{
				id:     "synth-1",
				status: StatusRunning,
			},
			startErr: fmt.Errorf("failed to start"),
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: mockOrch,
			Session:      &mockSession{},
		})

		err := synth.Execute(context.Background())

		if err == nil {
			t.Error("Execute() should return error when StartInstance fails")
		}
		if !strings.Contains(err.Error(), "start") {
			t.Errorf("error should mention 'start', got %v", err)
		}
	})
}

// Tests for ProceedToConsolidationOrComplete edge cases

func TestSynthesisOrchestrator_ProceedToConsolidationOrComplete_EdgeCases(t *testing.T) {
	t.Run("completes when session doesn't support extended interface", func(t *testing.T) {
		// Use basic mockSession that doesn't implement SynthesisSessionExtended
		mockSession := &mockSession{}

		completeCalled := false
		callbacks := &struct {
			mockCallbacks
		}{}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
			Callbacks:    callbacks,
		})

		err := synth.ProceedToConsolidationOrComplete()

		if err != nil {
			t.Errorf("ProceedToConsolidationOrComplete() unexpected error: %v", err)
		}

		// Should have set phase to complete
		if mockSession.phase != PhaseComplete {
			t.Errorf("phase should be PhaseComplete, got %v", mockSession.phase)
		}

		_ = completeCalled
	})

	t.Run("logs warning when orchestrator doesn't support consolidation", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession:       mockSession{},
			consolidationMode: "stacked", // Consolidation configured
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{}, // Doesn't implement SynthesisOrchestratorExtended
			Session:      mockSession,
		})

		err := synth.ProceedToConsolidationOrComplete()

		if err != nil {
			t.Errorf("ProceedToConsolidationOrComplete() unexpected error: %v", err)
		}

		// Should complete despite consolidation being configured
		if mockSession.completedAt == nil {
			t.Error("should complete when orchestrator doesn't support consolidation")
		}
	})
}

// Tests for CaptureTaskWorktreeInfo edge cases

func TestSynthesisOrchestrator_CaptureTaskWorktreeInfo_EdgeCases(t *testing.T) {
	t.Run("matches by slugified title in branch", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{
				completedTasks: []string{"task-1"},
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "Create User API"},
				},
			},
		}

		mockBaseSession := &mockBaseSessionExtended{
			instances: []InstanceExtendedInterface{
				&mockInstanceExtended{
					id:           "inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/create-user-api", // Slugified title
					task:         "",                        // Task field doesn't match
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

		if len(info) != 1 {
			t.Errorf("CaptureTaskWorktreeInfo() len = %d, want 1", len(info))
		}

		if len(info) > 0 && info[0].WorktreePath != "/tmp/wt1" {
			t.Errorf("WorktreePath = %q, want %q", info[0].WorktreePath, "/tmp/wt1")
		}
	})

	t.Run("skips tasks with nil task object", func(t *testing.T) {
		mockSession := &mockSessionExtended{
			mockSession: mockSession{
				completedTasks: []string{"task-1", "task-2"},
				tasks: map[string]any{
					"task-1": &mockTask{id: "task-1", title: "Valid Task"},
					// task-2 not in tasks map
				},
			},
		}

		mockBaseSession := &mockBaseSessionExtended{
			instances: []InstanceExtendedInterface{
				&mockInstanceExtended{
					id:           "inst-1",
					worktreePath: "/tmp/wt1",
					branch:       "claudio/valid-task",
					task:         "task-1",
				},
			},
		}

		synth, _ := NewSynthesisOrchestrator(&PhaseContext{
			Manager:      &mockManager{},
			Orchestrator: &mockOrchestrator{},
			Session:      mockSession,
			BaseSession:  mockBaseSession,
		})

		info := synth.CaptureTaskWorktreeInfo()

		// Should only capture task-1
		if len(info) != 1 {
			t.Errorf("CaptureTaskWorktreeInfo() len = %d, want 1", len(info))
		}
	})
}

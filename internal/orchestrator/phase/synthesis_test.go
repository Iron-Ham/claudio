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

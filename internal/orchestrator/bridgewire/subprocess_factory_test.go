package bridgewire

import (
	"context"
	"testing"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/streamjson"
)

func TestStartOptionsToSubprocessOptions(t *testing.T) {
	opts := ai.StartOptions{
		PermissionMode:         "bypass",
		Model:                  "claude-sonnet-4-6",
		MaxTurns:               50,
		AllowedTools:           []string{"Read", "Write"},
		DisallowedTools:        []string{"Bash"},
		AppendSystemPromptFile: "/tmp/sys.md",
		NoUserPrompt:           true,
		Worktree:               true,
	}

	got := startOptionsToSubprocessOptions(opts)

	if got.PermissionMode != "bypass" {
		t.Errorf("PermissionMode = %q, want %q", got.PermissionMode, "bypass")
	}
	if got.Model != "claude-sonnet-4-6" {
		t.Errorf("Model = %q, want %q", got.Model, "claude-sonnet-4-6")
	}
	if got.MaxTurns != 50 {
		t.Errorf("MaxTurns = %d, want %d", got.MaxTurns, 50)
	}
	if len(got.AllowedTools) != 2 || got.AllowedTools[0] != "Read" || got.AllowedTools[1] != "Write" {
		t.Errorf("AllowedTools = %v, want [Read Write]", got.AllowedTools)
	}
	if len(got.DisallowedTools) != 1 || got.DisallowedTools[0] != "Bash" {
		t.Errorf("DisallowedTools = %v, want [Bash]", got.DisallowedTools)
	}
	if got.AppendSystemPromptFile != "/tmp/sys.md" {
		t.Errorf("AppendSystemPromptFile = %q, want %q", got.AppendSystemPromptFile, "/tmp/sys.md")
	}
	if !got.NoUserPrompt {
		t.Error("NoUserPrompt = false, want true")
	}
	if !got.Worktree {
		t.Error("Worktree = false, want true")
	}
}

func TestStartOptionsToSubprocessOptions_ZeroValue(t *testing.T) {
	got := startOptionsToSubprocessOptions(ai.StartOptions{})

	if got.PermissionMode != "" {
		t.Errorf("PermissionMode = %q, want empty", got.PermissionMode)
	}
	if got.Model != "" {
		t.Errorf("Model = %q, want empty", got.Model)
	}
	if got.MaxTurns != 0 {
		t.Errorf("MaxTurns = %d, want 0", got.MaxTurns)
	}
	if got.AllowedTools != nil {
		t.Errorf("AllowedTools = %v, want nil", got.AllowedTools)
	}
	if got.DisallowedTools != nil {
		t.Errorf("DisallowedTools = %v, want nil", got.DisallowedTools)
	}
	if got.NoUserPrompt {
		t.Error("NoUserPrompt = true, want false")
	}
	if got.Worktree {
		t.Error("Worktree = true, want false")
	}
}

func TestStartOptionsToSubprocessOptions_DeepCopiesSlices(t *testing.T) {
	original := ai.StartOptions{
		AllowedTools:    []string{"Read", "Write"},
		DisallowedTools: []string{"Bash"},
	}

	got := startOptionsToSubprocessOptions(original)

	// Mutating the source should not affect the output.
	original.AllowedTools[0] = "mutated"
	original.DisallowedTools[0] = "mutated"

	if got.AllowedTools[0] != "Read" {
		t.Error("startOptionsToSubprocessOptions did not deep copy AllowedTools")
	}
	if got.DisallowedTools[0] != "Bash" {
		t.Error("startOptionsToSubprocessOptions did not deep copy DisallowedTools")
	}
}

func TestNewSubprocessFactory(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}

	f := NewSubprocessFactory(orch, sess, "claude", ai.StartOptions{}, nil)
	if f == nil {
		t.Fatal("NewSubprocessFactory returned nil")
	}

	sf, ok := f.(*subprocessFactory)
	if !ok {
		t.Fatal("expected *subprocessFactory type")
	}
	if sf.commandName != "claude" {
		t.Errorf("commandName = %q, want %q", sf.commandName, "claude")
	}
}

func TestNewSubprocessFactory_WithOverrides(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}
	overrides := ai.StartOptions{
		PermissionMode: "plan",
		Model:          "claude-opus-4-6",
		MaxTurns:       100,
	}

	f := NewSubprocessFactory(orch, sess, "claude", overrides, nil)
	sf := f.(*subprocessFactory)

	if sf.startOverrides.PermissionMode != "plan" {
		t.Errorf("PermissionMode = %q, want %q", sf.startOverrides.PermissionMode, "plan")
	}
	if sf.startOverrides.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", sf.startOverrides.Model, "claude-opus-4-6")
	}
	if sf.startOverrides.MaxTurns != 100 {
		t.Errorf("MaxTurns = %d, want %d", sf.startOverrides.MaxTurns, 100)
	}
}

func TestNewSubprocessFactoryWithContext(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := NewSubprocessFactoryWithContext(ctx, orch, sess, "claude", ai.StartOptions{}, nil)
	sf := f.(*subprocessFactory)

	if sf.parentCtx != ctx {
		t.Error("parentCtx was not set from the constructor")
	}
}

func TestNewSubprocessFactoryWithContext_NilCtx(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}

	f := NewSubprocessFactoryWithContext(nil, orch, sess, "claude", ai.StartOptions{}, nil) //nolint:staticcheck // intentional nil context test
	sf := f.(*subprocessFactory)

	if sf.parentCtx == nil {
		t.Error("parentCtx should default to context.Background(), got nil")
	}
}

func TestSubprocessFactory_StopEmpty(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}

	f := NewSubprocessFactory(orch, sess, "claude", ai.StartOptions{}, nil)
	sf := f.(*subprocessFactory)

	// Calling Stop on a factory with no running subprocesses should not panic.
	sf.Stop()

	// After Stop, the factory should be marked as stopped.
	sf.mu.Lock()
	stopped := sf.stopped
	sf.mu.Unlock()
	if !stopped {
		t.Error("factory should be stopped after Stop()")
	}
}

func TestSubprocessFactory_StopCancelsActiveContexts(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}

	f := NewSubprocessFactory(orch, sess, "claude", ai.StartOptions{}, nil)
	sf := f.(*subprocessFactory)

	// Simulate active subprocesses by pre-populating the cancel map and wg.
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	_ = cancel1 // ownership transferred to factory
	_ = cancel2

	sf.mu.Lock()
	sf.cancels["inst-1"] = cancel1
	sf.cancels["inst-2"] = cancel2
	sf.mu.Unlock()

	sf.Stop()

	// All contexts should be cancelled.
	if ctx1.Err() == nil {
		t.Error("ctx1 should be cancelled after Stop()")
	}
	if ctx2.Err() == nil {
		t.Error("ctx2 should be cancelled after Stop()")
	}

	// Cancel map should be empty (replaced with a new map).
	sf.mu.Lock()
	remaining := len(sf.cancels)
	sf.mu.Unlock()
	if remaining != 0 {
		t.Errorf("cancels map should be empty after Stop(), has %d entries", remaining)
	}
}

func TestSubprocessFactory_StartInstanceAfterStop(t *testing.T) {
	orch := &orchestrator.Orchestrator{}
	sess := &orchestrator.Session{}

	f := NewSubprocessFactory(orch, sess, "claude", ai.StartOptions{}, nil)
	sf := f.(*subprocessFactory)
	sf.Stop()

	// StartInstance after Stop should return an error.
	err := sf.StartInstance(&mockBridgeInstance{id: "test"})
	if err == nil {
		t.Fatal("expected error from StartInstance after Stop")
	}
}

func TestSubprocessFactory_ArgsIntegration(t *testing.T) {
	// Verify that the conversion chain produces correct subprocess args.
	opts := ai.StartOptions{
		PermissionMode:         "bypass",
		Model:                  "claude-sonnet-4-6",
		MaxTurns:               10,
		AppendSystemPromptFile: "/tmp/system.md",
		NoUserPrompt:           true,
	}

	subOpts := startOptionsToSubprocessOptions(opts)
	args := streamjson.BuildSubprocessArgs("/tmp/prompt.md", subOpts)

	// Verify key args are present.
	expected := map[string]bool{
		"--print":                        false,
		"--output-format":                false,
		"--dangerously-skip-permissions": false,
		"--model":                        false,
		"--max-turns":                    false,
		"--append-system-prompt-file":    false,
		"--no-user-prompt":               false,
		"--prompt-file":                  false,
	}

	for _, arg := range args {
		if _, ok := expected[arg]; ok {
			expected[arg] = true
		}
	}

	for flag, found := range expected {
		if !found {
			t.Errorf("expected flag %q not found in args: %v", flag, args)
		}
	}
}

func TestNewPipelineRunner_SubprocessMode(t *testing.T) {
	bus := event.NewBus()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Objective:      "test",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task one", Description: "do it"}},
		ExecutionOrder: [][]string{{"t1"}},
	}

	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:           &orchestrator.Orchestrator{},
		Session:        &orchestrator.Session{BaseRepo: t.TempDir()},
		Plan:           plan,
		Bus:            bus,
		MaxParallel:    2,
		SubprocessMode: true,
		CommandName:    "claude",
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner(SubprocessMode=true) error = %v", err)
	}
	if runner == nil {
		t.Fatal("NewPipelineRunner(SubprocessMode=true) returned nil")
	}

	// Verify the executor's factory is a *subprocessFactory.
	if _, ok := runner.exec.factory.(*subprocessFactory); !ok {
		t.Errorf("expected factory to be *subprocessFactory, got %T", runner.exec.factory)
	}
}

func TestNewPipelineRunner_SubprocessMode_DefaultCommandName(t *testing.T) {
	bus := event.NewBus()
	plan := &orchestrator.PlanSpec{
		ID:             "plan-1",
		Tasks:          []orchestrator.PlannedTask{{ID: "t1", Title: "task", Description: "d"}},
		ExecutionOrder: [][]string{{"t1"}},
	}

	runner, err := NewPipelineRunner(PipelineRunnerConfig{
		Orch:           &orchestrator.Orchestrator{},
		Session:        &orchestrator.Session{BaseRepo: t.TempDir()},
		Plan:           plan,
		Bus:            bus,
		MaxParallel:    1,
		SubprocessMode: true,
		// CommandName intentionally left empty — should default to "claude".
	})
	if err != nil {
		t.Fatalf("NewPipelineRunner() error = %v", err)
	}

	sf, ok := runner.exec.factory.(*subprocessFactory)
	if !ok {
		t.Fatalf("expected factory to be *subprocessFactory, got %T", runner.exec.factory)
	}
	if sf.commandName != "claude" {
		t.Errorf("commandName = %q, want %q (default)", sf.commandName, "claude")
	}
}

// mockBridgeInstance implements bridge.Instance for tests.
type mockBridgeInstance struct {
	id           string
	worktreePath string
	branch       string
}

func (m *mockBridgeInstance) ID() string           { return m.id }
func (m *mockBridgeInstance) WorktreePath() string { return m.worktreePath }
func (m *mockBridgeInstance) Branch() string       { return m.branch }

// Verify interface compliance at compile time.
var _ bridge.Instance = (*mockBridgeInstance)(nil)

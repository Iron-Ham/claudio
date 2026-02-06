package teamwire

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/bridge"
	ts "github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

// Compile-time interface compliance checks.
var (
	_ bridge.InstanceFactory   = (*attemptFactory)(nil)
	_ bridge.Instance          = (*attemptInstance)(nil)
	_ bridge.CompletionChecker = (*attemptCompletionChecker)(nil)
	_ bridge.CompletionChecker = (*judgeCompletionChecker)(nil)
	_ bridge.SessionRecorder   = (*sessionRecorder)(nil)
)

// --- InstanceFactory adapter ---

// attemptFactory adapts the tripleshot OrchestratorInterface to bridge.InstanceFactory.
type attemptFactory struct {
	orch    ts.OrchestratorInterface
	session ts.SessionInterface
}

func newAttemptFactory(orch ts.OrchestratorInterface, session ts.SessionInterface) bridge.InstanceFactory {
	return &attemptFactory{orch: orch, session: session}
}

func (f *attemptFactory) CreateInstance(taskPrompt string) (bridge.Instance, error) {
	inst, err := f.orch.AddInstance(f.session, taskPrompt)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	return &attemptInstance{inst: inst}, nil
}

func (f *attemptFactory) StartInstance(inst bridge.Instance) error {
	realInst := f.session.GetInstance(inst.ID())
	if realInst == nil {
		return fmt.Errorf("start instance: %q not found", inst.ID())
	}
	if err := f.orch.StartInstance(realInst); err != nil {
		return fmt.Errorf("start instance %q: %w", inst.ID(), err)
	}
	return nil
}

// --- Instance adapter ---

// attemptInstance adapts the tripleshot InstanceInterface to bridge.Instance.
type attemptInstance struct {
	inst ts.InstanceInterface
}

func (a *attemptInstance) ID() string           { return a.inst.GetID() }
func (a *attemptInstance) WorktreePath() string { return a.inst.GetWorktreePath() }
func (a *attemptInstance) Branch() string       { return a.inst.GetBranch() }

// --- CompletionChecker adapters ---

// attemptCompletionChecker checks for the tripleshot attempt completion sentinel file.
type attemptCompletionChecker struct{}

func newAttemptCompletionChecker() bridge.CompletionChecker {
	return &attemptCompletionChecker{}
}

func (c *attemptCompletionChecker) CheckCompletion(worktreePath string) (bool, error) {
	return ts.CompletionFileExists(worktreePath), nil
}

func (c *attemptCompletionChecker) VerifyWork(_, _, worktreePath, _ string) (bool, int, error) {
	completion, err := ts.ParseCompletionFile(worktreePath)
	if err != nil {
		return false, 0, fmt.Errorf("parse completion file: %w", err)
	}
	return completion.Status == "complete", 0, nil
}

// judgeCompletionChecker checks for the tripleshot evaluation sentinel file.
type judgeCompletionChecker struct{}

func newJudgeCompletionChecker() bridge.CompletionChecker {
	return &judgeCompletionChecker{}
}

func (c *judgeCompletionChecker) CheckCompletion(worktreePath string) (bool, error) {
	return ts.EvaluationFileExists(worktreePath), nil
}

func (c *judgeCompletionChecker) VerifyWork(_, _, worktreePath, _ string) (bool, int, error) {
	_, err := ts.ParseEvaluationFile(worktreePath)
	if err != nil {
		return false, 0, fmt.Errorf("parse evaluation file: %w", err)
	}
	return true, 0, nil
}

// --- SessionRecorder adapter ---

// sessionRecorderDeps defines the callbacks for the session recorder.
type sessionRecorderDeps struct {
	OnAssign   func(taskID, instanceID string)
	OnComplete func(taskID string, commitCount int)
	OnFailure  func(taskID, reason string)
}

// sessionRecorder delegates to caller-provided callbacks.
type sessionRecorder struct {
	deps sessionRecorderDeps
}

func newSessionRecorder(deps sessionRecorderDeps) bridge.SessionRecorder {
	return &sessionRecorder{deps: deps}
}

func (r *sessionRecorder) AssignTask(taskID, instanceID string) {
	if r.deps.OnAssign != nil {
		r.deps.OnAssign(taskID, instanceID)
	}
}

func (r *sessionRecorder) RecordCompletion(taskID string, commitCount int) {
	if r.deps.OnComplete != nil {
		r.deps.OnComplete(taskID, commitCount)
	}
}

func (r *sessionRecorder) RecordFailure(taskID, reason string) {
	if r.deps.OnFailure != nil {
		r.deps.OnFailure(taskID, reason)
	}
}

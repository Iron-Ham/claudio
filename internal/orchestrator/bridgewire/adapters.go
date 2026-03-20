package bridgewire

import (
	"errors"
	"fmt"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/bridge"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/verify"
)

// --- InstanceFactory adapter ---

// instanceFactory adapts an Orchestrator to bridge.InstanceFactory.
// It optionally carries per-instance StartOptions overrides that are applied
// when starting instances, allowing different teams/roles to use different
// permission modes, models, tool restrictions, etc.
type instanceFactory struct {
	orch           *orchestrator.Orchestrator
	session        *orchestrator.Session
	startOverrides ai.StartOptions
}

// NewInstanceFactory creates a bridge.InstanceFactory backed by the given Orchestrator.
func NewInstanceFactory(orch *orchestrator.Orchestrator, session *orchestrator.Session) bridge.InstanceFactory {
	return &instanceFactory{orch: orch, session: session}
}

// NewInstanceFactoryWithOverrides creates a bridge.InstanceFactory that applies
// the given StartOptions overrides to every instance it starts. This allows
// role-specific CLI flags (permission mode, model, tool restrictions) to flow
// from pipeline configuration through to the Claude Code invocation.
func NewInstanceFactoryWithOverrides(orch *orchestrator.Orchestrator, session *orchestrator.Session, overrides ai.StartOptions) bridge.InstanceFactory {
	return &instanceFactory{orch: orch, session: session, startOverrides: overrides}
}

// Coverage: CreateInstance and StartInstance wrap *orchestrator.Orchestrator which
// requires full session/worktree infrastructure; tested via integration tests.
func (f *instanceFactory) CreateInstance(taskPrompt string) (bridge.Instance, error) {
	inst, err := f.orch.AddInstance(f.session, taskPrompt)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	return &orchInstance{inst: inst}, nil
}

func (f *instanceFactory) StartInstance(inst bridge.Instance) error {
	orchInst := f.orch.GetInstance(inst.ID())
	if orchInst == nil {
		return fmt.Errorf("start instance: %q not found", inst.ID())
	}
	if err := f.orch.StartInstanceWithOverrides(orchInst, f.startOverrides); err != nil {
		return fmt.Errorf("start instance %q: %w", inst.ID(), err)
	}
	return nil
}

// orchInstance adapts an orchestrator.Instance to bridge.Instance.
type orchInstance struct {
	inst *orchestrator.Instance
}

func (oi *orchInstance) ID() string           { return oi.inst.ID }
func (oi *orchInstance) WorktreePath() string { return oi.inst.WorktreePath }
func (oi *orchInstance) Branch() string       { return oi.inst.Branch }

// --- CompletionChecker adapter ---

// completionChecker adapts an orchestrator.Verifier to bridge.CompletionChecker.
type completionChecker struct {
	verifier orchestrator.Verifier
}

// NewCompletionChecker creates a bridge.CompletionChecker backed by the given Verifier.
func NewCompletionChecker(v orchestrator.Verifier) bridge.CompletionChecker {
	return &completionChecker{verifier: v}
}

func (c *completionChecker) CheckCompletion(worktreePath string) (bool, error) {
	return c.verifier.CheckCompletionFile(worktreePath)
}

func (c *completionChecker) VerifyWork(taskID, instanceID, worktreePath, baseBranch string) (bool, int, error) {
	result := c.verifier.VerifyTaskWork(taskID, instanceID, worktreePath, baseBranch, &verify.TaskVerifyOptions{})
	if result.Error != "" {
		return result.Success, result.CommitCount, errors.New(result.Error)
	}
	return result.Success, result.CommitCount, nil
}

// --- SessionRecorder adapter ---

// SessionRecorderDeps defines the coordinator operations needed by the session recorder.
type SessionRecorderDeps struct {
	// OnAssign is called when a task is assigned to an instance.
	OnAssign func(taskID, instanceID string)

	// OnSentinelDetected is called when the instance writes its sentinel file
	// (entering the finishing phase before verification completes).
	OnSentinelDetected func(taskID, instanceID string)

	// OnComplete is called when a task completes successfully.
	OnComplete func(taskID string, commitCount int)

	// OnFailure is called when a task fails.
	OnFailure func(taskID, reason string)
}

// sessionRecorder delegates to caller-provided callbacks.
type sessionRecorder struct {
	deps SessionRecorderDeps
}

// NewSessionRecorder creates a bridge.SessionRecorder backed by the given callbacks.
func NewSessionRecorder(deps SessionRecorderDeps) bridge.SessionRecorder {
	return &sessionRecorder{deps: deps}
}

func (r *sessionRecorder) AssignTask(taskID, instanceID string) {
	if r.deps.OnAssign != nil {
		r.deps.OnAssign(taskID, instanceID)
	}
}

func (r *sessionRecorder) RecordSentinelDetected(taskID, instanceID string) {
	if r.deps.OnSentinelDetected != nil {
		r.deps.OnSentinelDetected(taskID, instanceID)
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

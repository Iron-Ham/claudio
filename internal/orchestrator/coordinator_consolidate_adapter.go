package orchestrator

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator/group/consolidate"
	"github.com/Iron-Ham/claudio/internal/orchestrator/types"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// coordinatorConsolidateAdapter adapts the Coordinator to consolidate.CoordinatorInterface.
type coordinatorConsolidateAdapter struct {
	c *Coordinator
}

func newCoordinatorConsolidateAdapter(c *Coordinator) *coordinatorConsolidateAdapter {
	return &coordinatorConsolidateAdapter{c: c}
}

func (a *coordinatorConsolidateAdapter) Session() consolidate.SessionInterface {
	session := a.c.Session()
	if session == nil {
		return nil
	}
	return &sessionConsolidateAdapter{s: session}
}

func (a *coordinatorConsolidateAdapter) Orchestrator() consolidate.OrchestratorInterface {
	return &orchestratorConsolidateAdapter{o: a.c.orch}
}

func (a *coordinatorConsolidateAdapter) BaseSession() consolidate.BaseSessionInterface {
	return &baseSessionConsolidateAdapter{s: a.c.baseSession}
}

func (a *coordinatorConsolidateAdapter) Manager() consolidate.ManagerInterface {
	return &managerConsolidateAdapter{m: a.c.manager}
}

func (a *coordinatorConsolidateAdapter) Lock()   { a.c.mu.Lock() }
func (a *coordinatorConsolidateAdapter) Unlock() { a.c.mu.Unlock() }

func (a *coordinatorConsolidateAdapter) Context() consolidate.ContextInterface {
	return &contextConsolidateAdapter{ctx: a.c.ctx}
}

// sessionConsolidateAdapter adapts UltraPlanSession to consolidate.SessionInterface.
type sessionConsolidateAdapter struct {
	s *UltraPlanSession
}

func (a *sessionConsolidateAdapter) GetID() string { return a.s.ID }

func (a *sessionConsolidateAdapter) GetPlan() consolidate.PlanInterface {
	if a.s.Plan == nil {
		return nil
	}
	return &planConsolidateAdapter{p: a.s.Plan}
}

func (a *sessionConsolidateAdapter) GetConfig() consolidate.ConfigInterface {
	return &configConsolidateAdapter{c: &a.s.Config}
}

func (a *sessionConsolidateAdapter) GetTask(taskID string) consolidate.TaskInterface {
	task := a.s.GetTask(taskID)
	if task == nil {
		return nil
	}
	return &taskConsolidateAdapter{t: task}
}

func (a *sessionConsolidateAdapter) GetTaskCommitCounts() map[string]int {
	return a.s.TaskCommitCounts
}

func (a *sessionConsolidateAdapter) GetGroupConsolidatedBranches() []string {
	return a.s.GroupConsolidatedBranches
}

func (a *sessionConsolidateAdapter) GetGroupConsolidationContexts() []*types.GroupConsolidationCompletionFile {
	return a.s.GroupConsolidationContexts
}

func (a *sessionConsolidateAdapter) GetGroupConsolidatorIDs() []string {
	return a.s.GroupConsolidatorIDs
}

func (a *sessionConsolidateAdapter) SetGroupConsolidatorID(groupIndex int, id string) {
	if groupIndex >= 0 && groupIndex < len(a.s.GroupConsolidatorIDs) {
		a.s.GroupConsolidatorIDs[groupIndex] = id
	}
}

func (a *sessionConsolidateAdapter) SetGroupConsolidatedBranch(groupIndex int, branch string) {
	if groupIndex >= 0 && groupIndex < len(a.s.GroupConsolidatedBranches) {
		a.s.GroupConsolidatedBranches[groupIndex] = branch
	}
}

func (a *sessionConsolidateAdapter) SetGroupConsolidationContext(groupIndex int, ctx *types.GroupConsolidationCompletionFile) {
	if groupIndex >= 0 && groupIndex < len(a.s.GroupConsolidationContexts) {
		a.s.GroupConsolidationContexts[groupIndex] = ctx
	}
}

func (a *sessionConsolidateAdapter) EnsureGroupArraysCapacity(groupIndex int) {
	for len(a.s.GroupConsolidatorIDs) <= groupIndex {
		a.s.GroupConsolidatorIDs = append(a.s.GroupConsolidatorIDs, "")
	}
	for len(a.s.GroupConsolidatedBranches) <= groupIndex {
		a.s.GroupConsolidatedBranches = append(a.s.GroupConsolidatedBranches, "")
	}
	for len(a.s.GroupConsolidationContexts) <= groupIndex {
		a.s.GroupConsolidationContexts = append(a.s.GroupConsolidationContexts, nil)
	}
}

// planConsolidateAdapter adapts PlanSpec to consolidate.PlanInterface.
type planConsolidateAdapter struct {
	p *PlanSpec
}

func (a *planConsolidateAdapter) GetSummary() string            { return a.p.Summary }
func (a *planConsolidateAdapter) GetExecutionOrder() [][]string { return a.p.ExecutionOrder }

// configConsolidateAdapter adapts UltraPlanConfig to consolidate.ConfigInterface.
type configConsolidateAdapter struct {
	c *UltraPlanConfig
}

func (a *configConsolidateAdapter) GetBranchPrefix() string { return a.c.BranchPrefix }
func (a *configConsolidateAdapter) IsMultiPass() bool       { return a.c.MultiPass }

// taskConsolidateAdapter adapts PlannedTask to consolidate.TaskInterface.
type taskConsolidateAdapter struct {
	t *PlannedTask
}

func (a *taskConsolidateAdapter) GetID() string    { return a.t.ID }
func (a *taskConsolidateAdapter) GetTitle() string { return a.t.Title }

// orchestratorConsolidateAdapter adapts Orchestrator to consolidate.OrchestratorInterface.
type orchestratorConsolidateAdapter struct {
	o *Orchestrator
}

func (a *orchestratorConsolidateAdapter) Worktree() consolidate.WorktreeInterface {
	return &worktreeConsolidateAdapter{wt: a.o.wt}
}

func (a *orchestratorConsolidateAdapter) AddInstance(baseSession consolidate.BaseSessionInterface, prompt string) (consolidate.InstanceInterface, error) {
	// Get the real base session from the adapter
	bsa, ok := baseSession.(*baseSessionConsolidateAdapter)
	if !ok {
		return nil, fmt.Errorf("baseSession is not a baseSessionConsolidateAdapter (got %T)", baseSession)
	}
	inst, err := a.o.AddInstance(bsa.s, prompt)
	if err != nil {
		return nil, err
	}
	return &instanceConsolidateAdapter{i: inst}, nil
}

func (a *orchestratorConsolidateAdapter) AddInstanceFromBranch(baseSession consolidate.BaseSessionInterface, prompt, branch string) (consolidate.InstanceInterface, error) {
	bsa, ok := baseSession.(*baseSessionConsolidateAdapter)
	if !ok {
		return nil, fmt.Errorf("baseSession is not a baseSessionConsolidateAdapter (got %T)", baseSession)
	}
	inst, err := a.o.AddInstanceFromBranch(bsa.s, prompt, branch)
	if err != nil {
		return nil, err
	}
	return &instanceConsolidateAdapter{i: inst}, nil
}

func (a *orchestratorConsolidateAdapter) GetInstance(id string) consolidate.InstanceInterface {
	inst := a.o.GetInstance(id)
	if inst == nil {
		return nil
	}
	return &instanceConsolidateAdapter{i: inst}
}

func (a *orchestratorConsolidateAdapter) StartInstance(inst consolidate.InstanceInterface) error {
	ica, ok := inst.(*instanceConsolidateAdapter)
	if !ok {
		return fmt.Errorf("instance is not an instanceConsolidateAdapter (got %T)", inst)
	}
	return a.o.StartInstance(ica.i)
}

func (a *orchestratorConsolidateAdapter) StopInstance(inst consolidate.InstanceInterface) error {
	if inst == nil {
		return nil
	}
	realInst := a.o.GetInstance(inst.GetID())
	if realInst == nil {
		return nil
	}
	return a.o.StopInstance(realInst)
}

func (a *orchestratorConsolidateAdapter) SaveSession() error {
	return a.o.SaveSession()
}

func (a *orchestratorConsolidateAdapter) GetClaudioDir() string {
	return a.o.claudioDir
}

func (a *orchestratorConsolidateAdapter) GetBranchPrefix() string {
	return a.o.config.Branch.Prefix
}

func (a *orchestratorConsolidateAdapter) GetInstanceManager(id string) consolidate.InstanceManagerInterface {
	mgr := a.o.GetInstanceManager(id)
	if mgr == nil {
		return nil
	}
	return &instanceManagerConsolidateAdapter{m: mgr}
}

// worktreeConsolidateAdapter adapts worktree.Manager to consolidate.WorktreeInterface.
type worktreeConsolidateAdapter struct {
	wt *worktree.Manager
}

func (a *worktreeConsolidateAdapter) FindMainBranch() string {
	return a.wt.FindMainBranch()
}

func (a *worktreeConsolidateAdapter) CreateBranchFrom(branchName, baseBranch string) error {
	return a.wt.CreateBranchFrom(branchName, baseBranch)
}

func (a *worktreeConsolidateAdapter) CreateWorktreeFromBranch(path, branch string) error {
	return a.wt.CreateWorktreeFromBranch(path, branch)
}

func (a *worktreeConsolidateAdapter) Remove(path string) error {
	return a.wt.Remove(path)
}

func (a *worktreeConsolidateAdapter) CherryPickBranch(worktreePath, sourceBranch string) error {
	return a.wt.CherryPickBranch(worktreePath, sourceBranch)
}

func (a *worktreeConsolidateAdapter) AbortCherryPick(worktreePath string) error {
	return a.wt.AbortCherryPick(worktreePath)
}

func (a *worktreeConsolidateAdapter) CountCommitsBetween(worktreePath, baseBranch, head string) (int, error) {
	return a.wt.CountCommitsBetween(worktreePath, baseBranch, head)
}

func (a *worktreeConsolidateAdapter) Push(worktreePath string, force bool) error {
	return a.wt.Push(worktreePath, force)
}

// instanceConsolidateAdapter adapts Instance to consolidate.InstanceInterface.
type instanceConsolidateAdapter struct {
	i *Instance
}

func (a *instanceConsolidateAdapter) GetID() string           { return a.i.ID }
func (a *instanceConsolidateAdapter) GetTask() string         { return a.i.Task }
func (a *instanceConsolidateAdapter) GetBranch() string       { return a.i.Branch }
func (a *instanceConsolidateAdapter) GetWorktreePath() string { return a.i.WorktreePath }
func (a *instanceConsolidateAdapter) GetStatus() string       { return string(a.i.Status) }

// instanceManagerConsolidateAdapter adapts instance.Manager to consolidate.InstanceManagerInterface.
type instanceManagerConsolidateAdapter struct {
	m *instance.Manager
}

func (a *instanceManagerConsolidateAdapter) TmuxSessionExists() bool {
	return a.m.TmuxSessionExists()
}

// baseSessionConsolidateAdapter adapts Session to consolidate.BaseSessionInterface.
type baseSessionConsolidateAdapter struct {
	s *Session
}

func (a *baseSessionConsolidateAdapter) GetInstances() []consolidate.InstanceInterface {
	result := make([]consolidate.InstanceInterface, len(a.s.Instances))
	for i, inst := range a.s.Instances {
		result[i] = &instanceConsolidateAdapter{i: inst}
	}
	return result
}

func (a *baseSessionConsolidateAdapter) GetGroupBySessionType(sessionType string) consolidate.GroupInterface {
	var st SessionType
	switch sessionType {
	case consolidate.SessionTypeUltraPlan:
		st = SessionTypeUltraPlan
	case consolidate.SessionTypePlanMulti:
		st = SessionTypePlanMulti
	default:
		return nil
	}
	g := a.s.GetGroupBySessionType(st)
	if g == nil {
		return nil
	}
	return &groupConsolidateAdapter{g: g}
}

// groupConsolidateAdapter adapts InstanceGroup to consolidate.GroupInterface.
type groupConsolidateAdapter struct {
	g *InstanceGroup
}

func (a *groupConsolidateAdapter) AddInstance(instanceID string) {
	a.g.AddInstance(instanceID)
}

// managerConsolidateAdapter adapts UltraPlanManager to consolidate.ManagerInterface.
type managerConsolidateAdapter struct {
	m *UltraPlanManager
}

func (a *managerConsolidateAdapter) EmitEvent(eventType, message string) {
	var et CoordinatorEventType
	switch eventType {
	case consolidate.EventGroupComplete:
		et = EventGroupComplete
	default:
		et = EventGroupComplete
	}
	a.m.emitEvent(CoordinatorEvent{Type: et, Message: message})
}

// contextConsolidateAdapter adapts context.Context to consolidate.ContextInterface.
type contextConsolidateAdapter struct {
	ctx interface{ Done() <-chan struct{} }
}

func (a *contextConsolidateAdapter) Done() <-chan struct{} {
	return a.ctx.Done()
}

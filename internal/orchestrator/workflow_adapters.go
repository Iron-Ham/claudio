// Package orchestrator provides workflow adapter types and factory functions.
// These adapters bridge the gap between the concrete orchestrator types and
// the interface-based workflow packages.
package orchestrator

import (
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
)

// orchestratorAdapter implements tripleshot.OrchestratorInterface
type orchestratorAdapter struct {
	orch *Orchestrator
}

func (a *orchestratorAdapter) AddInstance(session tripleshot.SessionInterface, task string) (tripleshot.InstanceInterface, error) {
	// Convert session interface back to concrete Session
	s := session.(*sessionAdapter).session
	return a.orch.AddInstance(s, task)
}

func (a *orchestratorAdapter) StartInstance(inst tripleshot.InstanceInterface) error {
	// The inst should be an *Instance which already satisfies the interface
	i := inst.(*Instance)
	return a.orch.StartInstance(i)
}

func (a *orchestratorAdapter) SaveSession() error {
	return a.orch.SaveSession()
}

// sessionAdapter implements tripleshot.SessionInterface
type sessionAdapter struct {
	session *Session
}

func (a *sessionAdapter) GetGroup(id string) tripleshot.GroupInterface {
	group := a.session.GetGroup(id)
	if group == nil {
		return nil
	}
	return &groupAdapter{group: group}
}

func (a *sessionAdapter) GetGroupBySessionType(sessionType string) tripleshot.GroupInterface {
	group := a.session.GetGroupBySessionType(SessionType(sessionType))
	if group == nil {
		return nil
	}
	return &groupAdapter{group: group}
}

func (a *sessionAdapter) GetInstance(id string) tripleshot.InstanceInterface {
	inst := a.session.GetInstance(id)
	if inst == nil {
		return nil
	}
	return inst
}

// groupAdapter implements tripleshot.GroupInterface
type groupAdapter struct {
	group *InstanceGroup
}

func (a *groupAdapter) AddInstance(instanceID string) {
	a.group.AddInstance(instanceID)
}

func (a *groupAdapter) AddSubGroup(subGroup tripleshot.GroupInterface) {
	sg := subGroup.(*groupAdapter).group
	a.group.AddSubGroup(sg)
}

func (a *groupAdapter) GetInstances() []string {
	return a.group.Instances
}

func (a *groupAdapter) SetInstances(instances []string) {
	a.group.Instances = instances
}

func (a *groupAdapter) GetID() string {
	return a.group.ID
}

// DefaultTripleShotConfig returns the default tripleshot configuration
func DefaultTripleShotConfig() tripleshot.Config {
	return tripleshot.DefaultConfig()
}

// NewTripleShotSession creates a new tripleshot session
func NewTripleShotSession(task string, config tripleshot.Config) *tripleshot.Session {
	return tripleshot.NewSession(task, config)
}

// NewTripleShotCoordinator creates a new tripleshot coordinator
func NewTripleShotCoordinator(orch *Orchestrator, session *Session, tripleSession *tripleshot.Session, logger *logging.Logger) *tripleshot.Coordinator {
	cfg := tripleshot.CoordinatorConfig{
		Orchestrator:  &orchestratorAdapter{orch: orch},
		BaseSession:   &sessionAdapter{session: session},
		TripleSession: tripleSession,
		Logger:        logger,
		SessionType:   string(SessionTypeTripleShot),
		NewGroup: func(name string) tripleshot.GroupInterface {
			return &groupAdapter{group: NewInstanceGroup(name)}
		},
		SetSessionType: func(g tripleshot.GroupInterface, sessionType string) {
			ga := g.(*groupAdapter)
			ga.group.SessionType = sessionType
		},
	}
	return tripleshot.NewCoordinator(cfg)
}

// adversarialOrchestratorAdapter implements adversarial.OrchestratorInterface
type adversarialOrchestratorAdapter struct {
	orch *Orchestrator
}

func (a *adversarialOrchestratorAdapter) AddInstance(session adversarial.SessionInterface, task string) (adversarial.InstanceInterface, error) {
	s := session.(*adversarialSessionAdapter).session
	return a.orch.AddInstance(s, task)
}

func (a *adversarialOrchestratorAdapter) AddInstanceToWorktree(session adversarial.SessionInterface, task, worktreePath, branch string) (adversarial.InstanceInterface, error) {
	s := session.(*adversarialSessionAdapter).session
	return a.orch.AddInstanceToWorktree(s, task, worktreePath, branch)
}

func (a *adversarialOrchestratorAdapter) StartInstance(inst adversarial.InstanceInterface) error {
	i := inst.(*Instance)
	return a.orch.StartInstance(i)
}

func (a *adversarialOrchestratorAdapter) SaveSession() error {
	return a.orch.SaveSession()
}

// adversarialSessionAdapter implements adversarial.SessionInterface
type adversarialSessionAdapter struct {
	session *Session
}

func (a *adversarialSessionAdapter) GetGroup(id string) adversarial.GroupInterface {
	group := a.session.GetGroup(id)
	if group == nil {
		return nil
	}
	return &adversarialGroupAdapter{group: group}
}

func (a *adversarialSessionAdapter) GetGroupBySessionType(sessionType string) adversarial.GroupInterface {
	group := a.session.GetGroupBySessionType(SessionType(sessionType))
	if group == nil {
		return nil
	}
	return &adversarialGroupAdapter{group: group}
}

func (a *adversarialSessionAdapter) GetInstance(id string) adversarial.InstanceInterface {
	inst := a.session.GetInstance(id)
	if inst == nil {
		return nil
	}
	return inst
}

// adversarialGroupAdapter implements adversarial.GroupInterface and
// adversarial.GroupWithSubGroupsInterface for round-based sub-grouping.
type adversarialGroupAdapter struct {
	group *InstanceGroup
}

func (a *adversarialGroupAdapter) AddInstance(instanceID string) {
	a.group.AddInstance(instanceID)
}

// GetOrCreateSubGroup finds or creates a sub-group with the given ID and name.
// This implements adversarial.GroupWithSubGroupsInterface.
func (a *adversarialGroupAdapter) GetOrCreateSubGroup(id, name string) adversarial.GroupInterface {
	if a.group == nil {
		return nil
	}

	// First, try to find existing sub-group by name
	for _, sg := range a.group.SubGroups {
		if sg.Name == name {
			return &adversarialGroupAdapter{group: sg}
		}
	}

	// Create new sub-group
	subGroup := NewInstanceGroupWithID(id, name)
	a.group.AddSubGroup(subGroup)

	return &adversarialGroupAdapter{group: subGroup}
}

// GetSubGroupByName returns a sub-group by name, or nil if not found.
// This implements adversarial.GroupWithSubGroupsInterface.
func (a *adversarialGroupAdapter) GetSubGroupByName(name string) adversarial.GroupInterface {
	if a.group == nil {
		return nil
	}

	for _, sg := range a.group.SubGroups {
		if sg.Name == name {
			return &adversarialGroupAdapter{group: sg}
		}
	}
	return nil
}

// DefaultAdversarialConfig returns the default adversarial configuration
func DefaultAdversarialConfig() adversarial.Config {
	return adversarial.DefaultConfig()
}

// NewAdversarialSession creates a new adversarial session
func NewAdversarialSession(task string, config adversarial.Config) *adversarial.Session {
	// Generate an ID for the session
	id := GenerateID()
	return adversarial.NewSession(id, task, config)
}

// NewAdversarialCoordinator creates a new adversarial coordinator
func NewAdversarialCoordinator(orch *Orchestrator, session *Session, advSession *adversarial.Session, logger *logging.Logger) *adversarial.Coordinator {
	cfg := adversarial.CoordinatorConfig{
		Orchestrator: &adversarialOrchestratorAdapter{orch: orch},
		BaseSession:  &adversarialSessionAdapter{session: session},
		AdvSession:   advSession,
		Logger:       logger,
		SessionType:  string(SessionTypeAdversarial),
	}
	return adversarial.NewCoordinator(cfg)
}

// ralphOrchestratorAdapter implements ralph.OrchestratorInterface
type ralphOrchestratorAdapter struct {
	orch *Orchestrator
}

func (a *ralphOrchestratorAdapter) AddInstance(session ralph.SessionInterface, task string) (ralph.InstanceInterface, error) {
	s := session.(*ralphSessionAdapter).session
	return a.orch.AddInstance(s, task)
}

func (a *ralphOrchestratorAdapter) AddInstanceToWorktree(session ralph.SessionInterface, task, worktreePath, branch string) (ralph.InstanceInterface, error) {
	s := session.(*ralphSessionAdapter).session
	return a.orch.AddInstanceToWorktree(s, task, worktreePath, branch)
}

func (a *ralphOrchestratorAdapter) StartInstance(inst ralph.InstanceInterface) error {
	i := inst.(*Instance)
	return a.orch.StartInstance(i)
}

func (a *ralphOrchestratorAdapter) SaveSession() error {
	return a.orch.SaveSession()
}

// ralphSessionAdapter implements ralph.SessionInterface
type ralphSessionAdapter struct {
	session *Session
}

func (a *ralphSessionAdapter) GetGroup(id string) ralph.GroupInterface {
	group := a.session.GetGroup(id)
	if group == nil {
		return nil
	}
	return &ralphGroupAdapter{group: group}
}

func (a *ralphSessionAdapter) GetGroupBySessionType(sessionType string) ralph.GroupInterface {
	group := a.session.GetGroupBySessionType(SessionType(sessionType))
	if group == nil {
		return nil
	}
	return &ralphGroupAdapter{group: group}
}

func (a *ralphSessionAdapter) GetInstance(id string) ralph.InstanceInterface {
	inst := a.session.GetInstance(id)
	if inst == nil {
		return nil
	}
	return inst
}

// ralphGroupAdapter implements ralph.GroupInterface
type ralphGroupAdapter struct {
	group *InstanceGroup
}

func (a *ralphGroupAdapter) AddInstance(instanceID string) {
	a.group.AddInstance(instanceID)
}

// DefaultRalphConfig returns the default ralph configuration
func DefaultRalphConfig() *ralph.Config {
	return ralph.DefaultConfig()
}

// NewRalphSession creates a new ralph session
func NewRalphSession(prompt string, config *ralph.Config) *ralph.Session {
	return ralph.NewSession(prompt, config)
}

// NewRalphCoordinator creates a new ralph coordinator
func NewRalphCoordinator(orch *Orchestrator, session *Session, ralphSession *ralph.Session, logger *logging.Logger) *ralph.Coordinator {
	cfg := ralph.CoordinatorConfig{
		Orchestrator: &ralphOrchestratorAdapter{orch: orch},
		BaseSession:  &ralphSessionAdapter{session: session},
		RalphSession: ralphSession,
		Logger:       logger,
		SessionType:  string(SessionTypeRalph),
	}
	return ralph.NewCoordinator(cfg)
}

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

// moveSubGroupUnder is a shared helper for moving sub-groups within an InstanceGroup.
// It finds the sub-group by ID, finds or creates the target, and moves the sub-group.
func moveSubGroupUnder(group *InstanceGroup, subGroupID, targetID, targetName string) bool {
	if group == nil {
		return false
	}

	// Find the sub-group to move
	var subGroupToMove *InstanceGroup
	subGroupIndex := -1
	for i, sg := range group.SubGroups {
		if sg.ID == subGroupID {
			subGroupToMove = sg
			subGroupIndex = i
			break
		}
	}

	if subGroupToMove == nil {
		return false
	}

	// Find or create the target sub-group
	var targetGroup *InstanceGroup
	for _, sg := range group.SubGroups {
		if sg.ID == targetID {
			targetGroup = sg
			break
		}
	}

	if targetGroup == nil {
		targetGroup = NewInstanceGroupWithID(targetID, targetName)
		group.AddSubGroup(targetGroup)
	}

	// Remove from parent and add to target
	group.SubGroups = append(group.SubGroups[:subGroupIndex], group.SubGroups[subGroupIndex+1:]...)
	targetGroup.AddSubGroup(subGroupToMove)

	return true
}

// orchestratorAdapter implements tripleshot.OrchestratorInterface
type orchestratorAdapter struct {
	orch *Orchestrator
}

func (a *orchestratorAdapter) AddInstance(session tripleshot.SessionInterface, task string) (tripleshot.InstanceInterface, error) {
	// Convert session interface back to concrete Session
	s := session.(*sessionAdapter).session
	return a.orch.AddInstance(s, task)
}

func (a *orchestratorAdapter) AddInstanceToWorktree(session tripleshot.SessionInterface, task, worktreePath, branch string) (tripleshot.InstanceInterface, error) {
	// Convert session interface back to concrete Session
	s := session.(*sessionAdapter).session
	return a.orch.AddInstanceToWorktree(s, task, worktreePath, branch)
}

func (a *orchestratorAdapter) StartInstance(inst tripleshot.InstanceInterface) error {
	// The inst should be an *Instance which already satisfies the interface
	i := inst.(*Instance)
	return a.orch.StartInstance(i)
}

func (a *orchestratorAdapter) SaveSession() error {
	return a.orch.SaveSession()
}

func (a *orchestratorAdapter) AddInstanceStub(session tripleshot.SessionInterface, task string) (tripleshot.InstanceInterface, error) {
	s := session.(*sessionAdapter).session
	return a.orch.AddInstanceStub(s, task)
}

func (a *orchestratorAdapter) CompleteInstanceSetupByID(session tripleshot.SessionInterface, instanceID string) error {
	s := session.(*sessionAdapter).session
	return a.orch.CompleteInstanceSetupByID(s, instanceID)
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

// groupAdapter implements tripleshot.GroupInterface and
// tripleshot.GroupWithSubGroupsInterface for adversarial mode sub-grouping.
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

// RemoveInstance removes an instance from the group.
// This implements tripleshot.GroupWithSubGroupsInterface.
func (a *groupAdapter) RemoveInstance(instanceID string) {
	if a.group == nil {
		return
	}
	filtered := make([]string, 0, len(a.group.Instances))
	for _, id := range a.group.Instances {
		if id != instanceID {
			filtered = append(filtered, id)
		}
	}
	a.group.Instances = filtered
}

// GetOrCreateSubGroup finds or creates a sub-group with the given ID and name.
// This implements tripleshot.GroupWithSubGroupsInterface.
func (a *groupAdapter) GetOrCreateSubGroup(id, name string) tripleshot.GroupInterface {
	if a.group == nil {
		return nil
	}

	// First, try to find existing sub-group by name
	for _, sg := range a.group.SubGroups {
		if sg.Name == name {
			return &groupAdapter{group: sg}
		}
	}

	// Create new sub-group
	subGroup := NewInstanceGroupWithID(id, name)
	a.group.AddSubGroup(subGroup)

	return &groupAdapter{group: subGroup}
}

// GetSubGroupByID returns a sub-group by ID, or nil if not found.
// This implements tripleshot.GroupWithSubGroupsInterface.
func (a *groupAdapter) GetSubGroupByID(id string) tripleshot.GroupInterface {
	if a.group == nil {
		return nil
	}

	for _, sg := range a.group.SubGroups {
		if sg.ID == id {
			return &groupAdapter{group: sg}
		}
	}
	return nil
}

// MoveSubGroupUnder moves a sub-group to become a child of another sub-group.
// If the target doesn't exist, it will be created with the given targetName.
// This implements tripleshot.GroupWithSubGroupsInterface.
func (a *groupAdapter) MoveSubGroupUnder(subGroupID, targetID, targetName string) bool {
	return moveSubGroupUnder(a.group, subGroupID, targetID, targetName)
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

func (a *adversarialOrchestratorAdapter) AddInstanceToWorktreeWithBackend(session adversarial.SessionInterface, task, worktreePath, branch, backendName string) (adversarial.InstanceInterface, error) {
	s := session.(*adversarialSessionAdapter).session
	return a.orch.AddInstanceToWorktreeWithBackend(s, task, worktreePath, branch, backendName)
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

func (a *adversarialGroupAdapter) GetInstances() []string {
	if a.group == nil {
		return nil
	}
	return a.group.Instances
}

func (a *adversarialGroupAdapter) RemoveInstance(instanceID string) {
	if a.group == nil {
		return
	}
	filtered := make([]string, 0, len(a.group.Instances))
	for _, id := range a.group.Instances {
		if id != instanceID {
			filtered = append(filtered, id)
		}
	}
	a.group.Instances = filtered
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

// GetSubGroupByID returns a sub-group by ID, or nil if not found.
// This implements adversarial.GroupWithSubGroupsInterface.
func (a *adversarialGroupAdapter) GetSubGroupByID(id string) adversarial.GroupInterface {
	if a.group == nil {
		return nil
	}

	for _, sg := range a.group.SubGroups {
		if sg.ID == id {
			return &adversarialGroupAdapter{group: sg}
		}
	}
	return nil
}

// MoveSubGroupUnder moves a sub-group to become a child of another sub-group.
// If the target doesn't exist, it will be created with the given targetName.
// This implements adversarial.GroupWithSubGroupsInterface.
func (a *adversarialGroupAdapter) MoveSubGroupUnder(subGroupID, targetID, targetName string) bool {
	return moveSubGroupUnder(a.group, subGroupID, targetID, targetName)
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
		Orchestrator:    &adversarialOrchestratorAdapter{orch: orch},
		BaseSession:     &adversarialSessionAdapter{session: session},
		AdvSession:      advSession,
		Logger:          logger,
		SessionType:     string(SessionTypeAdversarial),
		ReviewerBackend: advSession.Config.ReviewerBackend,
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

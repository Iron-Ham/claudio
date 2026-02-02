// Package prworkflow provides PR workflow management for the orchestrator.
// It extracts PR workflow coordination from the main Orchestrator to maintain
// single responsibility and enable easier testing.
package prworkflow

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"sync"

	"github.com/Iron-Ham/claudio/internal/ai"
	"github.com/Iron-Ham/claudio/internal/config"
	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/logging"
)

// InstanceInfo provides the minimal information about an instance needed
// for PR workflow management. This interface decouples the Manager from
// the full orchestrator.Instance type.
type InstanceInfo interface {
	GetID() string
	GetWorktreePath() string
	GetBranch() string
	GetTask() string
}

// Config holds configuration for the PR workflow manager.
type Config struct {
	// UseAI enables AI-assisted PR creation via the configured backend
	UseAI bool
	// Draft creates PRs as drafts
	Draft bool
	// AutoRebase enables automatic rebasing before PR creation
	AutoRebase bool
	// TmuxWidth is the default tmux window width
	TmuxWidth int
	// TmuxHeight is the default tmux window height
	TmuxHeight int
}

// NewConfigFromConfig creates a PR workflow Config from the global config.
func NewConfigFromConfig(cfg *config.Config) Config {
	return Config{
		UseAI:      cfg.PR.UseAI,
		Draft:      cfg.PR.Draft,
		AutoRebase: cfg.PR.AutoRebase,
		TmuxWidth:  cfg.Instance.TmuxWidth,
		TmuxHeight: cfg.Instance.TmuxHeight,
	}
}

// Manager coordinates PR workflows for instances.
// It handles starting, tracking, and completing PR workflows, and notifies
// interested parties via callbacks and events.
type Manager struct {
	config    Config
	sessionID string // Claudio session ID for multi-session support
	eventBus  *event.Bus
	logger    *logging.Logger
	backend   ai.Backend

	// Display dimensions (can be updated when TUI resizes)
	displayWidth  int
	displayHeight int

	// Callbacks for PR workflow events
	completeCallback func(instanceID string, success bool)
	openedCallback   func(instanceID string)

	mu        sync.RWMutex
	workflows map[string]*instance.PRWorkflow
}

// NewManager creates a new PR workflow manager.
func NewManager(cfg Config, sessionID string, eventBus *event.Bus, backend ai.Backend) *Manager {
	if backend == nil {
		backend = ai.DefaultBackend()
	}
	return &Manager{
		config:    cfg,
		sessionID: sessionID,
		eventBus:  eventBus,
		backend:   backend,
		workflows: make(map[string]*instance.PRWorkflow),
	}
}

// SetLogger sets the logger for the manager.
func (m *Manager) SetLogger(logger *logging.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logger = logger
}

// SetDisplayDimensions sets the display dimensions for new PR workflows.
// This should be called when the TUI window resizes.
func (m *Manager) SetDisplayDimensions(width, height int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.displayWidth = width
	m.displayHeight = height
}

// SetCompleteCallback sets the callback invoked when a PR workflow completes.
func (m *Manager) SetCompleteCallback(cb func(instanceID string, success bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCallback = cb
}

// SetOpenedCallback sets the callback invoked when a PR is opened.
func (m *Manager) SetOpenedCallback(cb func(instanceID string)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.openedCallback = cb
}

// Start begins a PR workflow for the given instance.
func (m *Manager) Start(inst InstanceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Build workflow configuration
	cfg := instance.PRWorkflowConfig{
		UseAI:      m.config.UseAI,
		Draft:      m.config.Draft,
		AutoRebase: m.config.AutoRebase,
		TmuxWidth:  m.displayWidth,
		TmuxHeight: m.displayHeight,
		Backend:    m.backend,
	}

	// Use config defaults if display dimensions not set
	if cfg.TmuxWidth == 0 {
		cfg.TmuxWidth = m.config.TmuxWidth
	}
	if cfg.TmuxHeight == 0 {
		cfg.TmuxHeight = m.config.TmuxHeight
	}

	// Create workflow with session-scoped naming if in multi-session mode
	var workflow *instance.PRWorkflow
	if m.sessionID != "" {
		workflow = instance.NewPRWorkflowWithSession(
			m.sessionID,
			inst.GetID(),
			inst.GetWorktreePath(),
			inst.GetBranch(),
			inst.GetTask(),
			cfg,
		)
	} else {
		workflow = instance.NewPRWorkflow(
			inst.GetID(),
			inst.GetWorktreePath(),
			inst.GetBranch(),
			inst.GetTask(),
			cfg,
		)
	}

	// Set logger if available
	if m.logger != nil {
		workflow.SetLogger(m.logger)
	}

	// Set completion callback
	workflow.SetCallback(m.handleComplete)

	// Start the workflow
	if err := workflow.Start(); err != nil {
		return err
	}

	m.workflows[inst.GetID()] = workflow
	return nil
}

// handleComplete handles PR workflow completion.
func (m *Manager) handleComplete(instanceID string, success bool, output string) {
	m.mu.Lock()
	// Clean up workflow
	delete(m.workflows, instanceID)

	// Get callbacks before unlocking
	completeCallback := m.completeCallback
	eventBus := m.eventBus
	m.mu.Unlock()

	// Publish event to event bus if available
	if eventBus != nil {
		eventBus.Publish(event.NewPRCompleteEvent(instanceID, success, "", ""))
	}

	// Notify via callback if set (for backwards compatibility)
	if completeCallback != nil {
		completeCallback(instanceID, success)
	}
}

// HandleComplete allows external completion handling (e.g., for testing or manual completion).
// In normal operation, completion is handled automatically via the workflow callback.
func (m *Manager) HandleComplete(instanceID string, success bool, output string) {
	m.handleComplete(instanceID, success, output)
}

// Get returns the PR workflow for an instance, or nil if none exists.
func (m *Manager) Get(instanceID string) *instance.PRWorkflow {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workflows[instanceID]
}

// Stop terminates a PR workflow for the given instance.
func (m *Manager) Stop(instanceID string) error {
	m.mu.Lock()
	workflow, ok := m.workflows[instanceID]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.workflows, instanceID)
	m.mu.Unlock()

	return workflow.Stop()
}

// StopAll terminates all running PR workflows.
func (m *Manager) StopAll() {
	m.mu.Lock()
	workflows := make(map[string]*instance.PRWorkflow, len(m.workflows))
	maps.Copy(workflows, m.workflows)
	m.workflows = make(map[string]*instance.PRWorkflow)
	m.mu.Unlock()

	for _, workflow := range workflows {
		_ = workflow.Stop()
	}
}

// Running returns true if there's an active PR workflow for the instance.
func (m *Manager) Running(instanceID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	workflow, ok := m.workflows[instanceID]
	return ok && workflow != nil && workflow.Running()
}

// Count returns the number of active PR workflows.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workflows)
}

// IDs returns the instance IDs of all active PR workflows.
func (m *Manager) IDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.workflows))
	for id := range m.workflows {
		ids = append(ids, id)
	}
	return ids
}

// GroupPRMode specifies how PRs should be created for groups.
type GroupPRMode int

const (
	// GroupPRModeStacked creates one PR per group with stacked dependencies.
	// Each group's PR is based on the previous group's branch.
	GroupPRModeStacked GroupPRMode = iota

	// GroupPRModeConsolidated creates a single PR containing all groups' changes.
	GroupPRModeConsolidated

	// GroupPRModeSingle creates a PR for a single group only.
	GroupPRModeSingle
)

// String returns a human-readable string for the GroupPRMode.
func (m GroupPRMode) String() string {
	switch m {
	case GroupPRModeStacked:
		return "stacked"
	case GroupPRModeConsolidated:
		return "consolidated"
	case GroupPRModeSingle:
		return "single"
	default:
		return "unknown"
	}
}

// GroupInfo represents group information for PR workflows.
// This interface decouples group PR operations from the orchestrator's InstanceGroup type.
type GroupInfo interface {
	GetID() string
	GetName() string
	GetInstanceIDs() []string
	GetExecutionOrder() int
	GetDependsOn() []string
}

// GroupPROptions configures group-based PR creation.
type GroupPROptions struct {
	// Mode specifies how PRs should be created (stacked, consolidated, or single).
	Mode GroupPRMode

	// GroupID is the target group ID (for GroupPRModeSingle).
	GroupID string

	// Groups contains all groups to process (for stacked/consolidated modes).
	Groups []GroupInfo

	// Instances maps instance IDs to their information.
	// Required for looking up instance details when building PRs.
	Instances map[string]InstanceInfo

	// SessionName is a human-readable session name for PR descriptions.
	SessionName string

	// BaseBranch is the base branch for the first PR or consolidated PR.
	// Defaults to "main" if not specified.
	BaseBranch string

	// IncludeGroupStructure adds group relationship information to PR descriptions.
	IncludeGroupStructure bool

	// AutoLinkRelatedPRs adds links to related PRs from the same session.
	AutoLinkRelatedPRs bool
}

// GroupPRResult contains the result of a group PR operation.
type GroupPRResult struct {
	// GroupID is the ID of the group this result is for.
	GroupID string

	// GroupName is the name of the group.
	GroupName string

	// InstanceIDs lists the instances included in this PR.
	InstanceIDs []string

	// PRDescription is the generated PR description.
	PRDescription string

	// PRTitle is the generated PR title.
	PRTitle string

	// BaseBranch is the base branch for this PR.
	BaseBranch string

	// HeadBranch is the head branch for this PR.
	HeadBranch string
}

// GroupPRSession tracks an active group PR workflow.
type GroupPRSession struct {
	// ID uniquely identifies this group PR session.
	ID string

	// Mode is the PR creation mode.
	Mode GroupPRMode

	// Groups contains group information in execution order.
	Groups []GroupInfo

	// Results contains results for each processed group.
	Results []*GroupPRResult

	// CreatedPRURLs maps group IDs to their created PR URLs.
	CreatedPRURLs map[string]string

	// FailedGroups contains group IDs that failed PR creation.
	FailedGroups []string

	// PendingGroups contains group IDs awaiting PR creation.
	PendingGroups []string

	// CurrentGroupIndex is the index of the currently processing group.
	CurrentGroupIndex int
}

// GenerateGroupPRDescription creates a PR description for a group.
// This includes task relationships, group structure, and links to related PRs.
func GenerateGroupPRDescription(opts GroupPROptions, group GroupInfo, relatedPRURLs map[string]string) string {
	var sb strings.Builder

	// Header
	sb.WriteString("## Summary\n\n")

	// Group info
	sb.WriteString(fmt.Sprintf("**Group:** %s\n", group.GetName()))
	if opts.SessionName != "" {
		sb.WriteString(fmt.Sprintf("**Session:** %s\n", opts.SessionName))
	}
	sb.WriteString("\n")

	// Tasks in this group
	instanceIDs := group.GetInstanceIDs()
	if len(instanceIDs) > 0 {
		sb.WriteString("### Tasks in this PR\n\n")
		for _, instID := range instanceIDs {
			if inst, ok := opts.Instances[instID]; ok {
				task := inst.GetTask()
				if task != "" {
					sb.WriteString(fmt.Sprintf("- %s\n", task))
				} else {
					sb.WriteString(fmt.Sprintf("- Instance %s\n", instID))
				}
			}
		}
		sb.WriteString("\n")
	}

	// Group structure and dependencies
	if opts.IncludeGroupStructure && len(opts.Groups) > 1 {
		sb.WriteString("### Group Structure\n\n")

		// Show execution order
		orderedGroups := make([]GroupInfo, len(opts.Groups))
		copy(orderedGroups, opts.Groups)
		sort.Slice(orderedGroups, func(i, j int) bool {
			return orderedGroups[i].GetExecutionOrder() < orderedGroups[j].GetExecutionOrder()
		})

		for i, g := range orderedGroups {
			marker := " "
			if g.GetID() == group.GetID() {
				marker = "→"
			}
			sb.WriteString(fmt.Sprintf("%s %d. %s", marker, i+1, g.GetName()))

			// Show dependency info
			deps := g.GetDependsOn()
			if len(deps) > 0 {
				depNames := make([]string, 0, len(deps))
				for _, depID := range deps {
					for _, og := range opts.Groups {
						if og.GetID() == depID {
							depNames = append(depNames, og.GetName())
							break
						}
					}
				}
				if len(depNames) > 0 {
					sb.WriteString(fmt.Sprintf(" (depends on: %s)", strings.Join(depNames, ", ")))
				}
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Related PRs from the same session
	if opts.AutoLinkRelatedPRs && len(relatedPRURLs) > 0 {
		sb.WriteString("### Related PRs\n\n")

		// Sort by group for consistent ordering
		var relatedGroupIDs []string
		for gid := range relatedPRURLs {
			if gid != group.GetID() {
				relatedGroupIDs = append(relatedGroupIDs, gid)
			}
		}
		sort.Strings(relatedGroupIDs)

		for _, gid := range relatedGroupIDs {
			url := relatedPRURLs[gid]
			// Find group name
			groupName := gid
			for _, g := range opts.Groups {
				if g.GetID() == gid {
					groupName = g.GetName()
					break
				}
			}
			sb.WriteString(fmt.Sprintf("- [%s](%s)\n", groupName, url))
		}
		sb.WriteString("\n")
	}

	// Test plan placeholder
	sb.WriteString("## Test Plan\n\n")
	sb.WriteString("- [ ] Verify changes work as expected\n")
	sb.WriteString("- [ ] Run existing tests\n")

	return sb.String()
}

// GenerateGroupPRTitle creates a PR title for a group.
func GenerateGroupPRTitle(group GroupInfo, mode GroupPRMode, totalGroups int) string {
	switch mode {
	case GroupPRModeConsolidated:
		return fmt.Sprintf("%s (consolidated from %d groups)", group.GetName(), totalGroups)
	case GroupPRModeStacked:
		return fmt.Sprintf("[%d/%d] %s", group.GetExecutionOrder()+1, totalGroups, group.GetName())
	default:
		return group.GetName()
	}
}

// GenerateConsolidatedPRDescription creates a PR description for a consolidated PR.
func GenerateConsolidatedPRDescription(opts GroupPROptions) string {
	var sb strings.Builder

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("This PR consolidates changes from %d groups.\n\n", len(opts.Groups)))

	if opts.SessionName != "" {
		sb.WriteString(fmt.Sprintf("**Session:** %s\n\n", opts.SessionName))
	}

	// Order groups by execution order
	orderedGroups := make([]GroupInfo, len(opts.Groups))
	copy(orderedGroups, opts.Groups)
	sort.Slice(orderedGroups, func(i, j int) bool {
		return orderedGroups[i].GetExecutionOrder() < orderedGroups[j].GetExecutionOrder()
	})

	// List each group and its tasks
	for i, group := range orderedGroups {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, group.GetName()))

		instanceIDs := group.GetInstanceIDs()
		if len(instanceIDs) > 0 {
			for _, instID := range instanceIDs {
				if inst, ok := opts.Instances[instID]; ok {
					task := inst.GetTask()
					if task != "" {
						sb.WriteString(fmt.Sprintf("- %s\n", task))
					}
				}
			}
		}
		sb.WriteString("\n")
	}

	// Test plan
	sb.WriteString("## Test Plan\n\n")
	sb.WriteString("- [ ] Verify all group changes work together\n")
	sb.WriteString("- [ ] Run integration tests\n")

	return sb.String()
}

// GenerateConsolidatedPRTitle creates a title for a consolidated PR.
func GenerateConsolidatedPRTitle(opts GroupPROptions) string {
	if len(opts.Groups) == 0 {
		return "Consolidated changes"
	}

	// Use first group's name or session name
	if opts.SessionName != "" {
		return fmt.Sprintf("%s (consolidated)", opts.SessionName)
	}

	if len(opts.Groups) == 1 {
		return opts.Groups[0].GetName()
	}

	return fmt.Sprintf("Consolidated: %s + %d more", opts.Groups[0].GetName(), len(opts.Groups)-1)
}

// PrepareGroupPR prepares PR metadata for a single group without starting the workflow.
// This is useful for previewing or customizing PR details before creation.
func (m *Manager) PrepareGroupPR(opts GroupPROptions, group GroupInfo, relatedPRURLs map[string]string) *GroupPRResult {
	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	// For stacked PRs, determine base branch from previous group
	if opts.Mode == GroupPRModeStacked && len(opts.Groups) > 1 {
		// Find previous group in execution order
		orderedGroups := make([]GroupInfo, len(opts.Groups))
		copy(orderedGroups, opts.Groups)
		sort.Slice(orderedGroups, func(i, j int) bool {
			return orderedGroups[i].GetExecutionOrder() < orderedGroups[j].GetExecutionOrder()
		})

		for i, g := range orderedGroups {
			if g.GetID() == group.GetID() && i > 0 {
				// Use previous group's branch as base
				prevGroup := orderedGroups[i-1]
				// The branch name would typically be derived from the instance
				// For now, we'll use a placeholder that callers can override
				if len(prevGroup.GetInstanceIDs()) > 0 {
					if inst, ok := opts.Instances[prevGroup.GetInstanceIDs()[0]]; ok {
						baseBranch = inst.GetBranch()
					}
				}
				break
			}
		}
	}

	// Determine head branch from first instance in this group
	headBranch := ""
	instanceIDs := group.GetInstanceIDs()
	if len(instanceIDs) > 0 {
		if inst, ok := opts.Instances[instanceIDs[0]]; ok {
			headBranch = inst.GetBranch()
		}
	}

	return &GroupPRResult{
		GroupID:       group.GetID(),
		GroupName:     group.GetName(),
		InstanceIDs:   instanceIDs,
		PRDescription: GenerateGroupPRDescription(opts, group, relatedPRURLs),
		PRTitle:       GenerateGroupPRTitle(group, opts.Mode, len(opts.Groups)),
		BaseBranch:    baseBranch,
		HeadBranch:    headBranch,
	}
}

// PrepareConsolidatedPR prepares PR metadata for a consolidated PR.
func (m *Manager) PrepareConsolidatedPR(opts GroupPROptions) *GroupPRResult {
	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	// Collect all instance IDs from all groups
	var allInstanceIDs []string
	var headBranch string

	// Order groups by execution order
	orderedGroups := make([]GroupInfo, len(opts.Groups))
	copy(orderedGroups, opts.Groups)
	sort.Slice(orderedGroups, func(i, j int) bool {
		return orderedGroups[i].GetExecutionOrder() < orderedGroups[j].GetExecutionOrder()
	})

	for _, group := range orderedGroups {
		allInstanceIDs = append(allInstanceIDs, group.GetInstanceIDs()...)
	}

	// Use the last group's branch as head (contains all merged changes)
	if len(orderedGroups) > 0 {
		lastGroup := orderedGroups[len(orderedGroups)-1]
		instanceIDs := lastGroup.GetInstanceIDs()
		if len(instanceIDs) > 0 {
			if inst, ok := opts.Instances[instanceIDs[0]]; ok {
				headBranch = inst.GetBranch()
			}
		}
	}

	return &GroupPRResult{
		GroupID:       "consolidated",
		GroupName:     "Consolidated",
		InstanceIDs:   allInstanceIDs,
		PRDescription: GenerateConsolidatedPRDescription(opts),
		PRTitle:       GenerateConsolidatedPRTitle(opts),
		BaseBranch:    baseBranch,
		HeadBranch:    headBranch,
	}
}

// NewGroupPRSession creates a new tracking session for group PR workflows.
func NewGroupPRSession(id string, opts GroupPROptions) *GroupPRSession {
	session := &GroupPRSession{
		ID:            id,
		Mode:          opts.Mode,
		Groups:        opts.Groups,
		Results:       make([]*GroupPRResult, 0),
		CreatedPRURLs: make(map[string]string),
		FailedGroups:  make([]string, 0),
		PendingGroups: make([]string, 0, len(opts.Groups)),
	}

	// Initialize pending groups
	for _, g := range opts.Groups {
		session.PendingGroups = append(session.PendingGroups, g.GetID())
	}

	return session
}

// RecordPRCreated records a successfully created PR for a group.
func (s *GroupPRSession) RecordPRCreated(groupID, prURL string, result *GroupPRResult) {
	s.CreatedPRURLs[groupID] = prURL
	s.Results = append(s.Results, result)

	// Remove from pending
	for i, id := range s.PendingGroups {
		if id == groupID {
			s.PendingGroups = append(s.PendingGroups[:i], s.PendingGroups[i+1:]...)
			break
		}
	}
}

// RecordPRFailed records a failed PR creation for a group.
func (s *GroupPRSession) RecordPRFailed(groupID string) {
	s.FailedGroups = append(s.FailedGroups, groupID)

	// Remove from pending
	for i, id := range s.PendingGroups {
		if id == groupID {
			s.PendingGroups = append(s.PendingGroups[:i], s.PendingGroups[i+1:]...)
			break
		}
	}
}

// IsComplete returns true if all groups have been processed.
func (s *GroupPRSession) IsComplete() bool {
	return len(s.PendingGroups) == 0
}

// SuccessCount returns the number of successfully created PRs.
func (s *GroupPRSession) SuccessCount() int {
	return len(s.CreatedPRURLs)
}

// FailureCount returns the number of failed PR creations.
func (s *GroupPRSession) FailureCount() int {
	return len(s.FailedGroups)
}

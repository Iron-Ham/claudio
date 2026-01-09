package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/config"
)

// ReviewCallbacks contains callback functions for review events.
// These allow the caller to react to review progress and findings.
type ReviewCallbacks struct {
	// OnIssueFound is called when a review agent discovers an issue.
	OnIssueFound func(issue ReviewIssue)

	// OnAgentComplete is called when a review agent finishes its analysis.
	OnAgentComplete func(agentID string, issueCount int)

	// OnReviewComplete is called when all review agents have finished.
	OnReviewComplete func(totalIssues int, summary string)

	// OnCriticalIssue is called when a critical severity issue is found.
	// This can be used to implement auto-pause logic for the implementer.
	OnCriticalIssue func(issue ReviewIssue)
}

// ReviewCoordinator manages parallel review agent execution.
// It spawns specialized review agents (security, performance, style, etc.)
// and collects their findings into a unified review session.
type ReviewCoordinator struct {
	// orch is a reference to the main orchestrator for instance management
	orch *Orchestrator

	// session holds the current review session state
	session *ReviewSession

	// callbacks contains event handlers for review progress
	callbacks *ReviewCallbacks

	// ctx is the context for the review coordination
	ctx context.Context

	// cancelFunc cancels all review agent operations
	cancelFunc context.CancelFunc

	// mu protects concurrent access to coordinator state
	mu sync.RWMutex

	// runningAgents tracks active review agent instances by their ID
	runningAgents map[string]*ReviewAgent

	// issuesChan receives issues from all review agents
	issuesChan chan ReviewIssue

	// doneChan signals that all agents have completed
	doneChan chan struct{}

	// paused indicates if the review is currently paused
	paused bool

	// configReview holds the user's review configuration
	configReview *config.ReviewConfig

	// wg tracks running agent goroutines
	wg sync.WaitGroup
}

// NewReviewCoordinator creates a new ReviewCoordinator for managing parallel review agents.
// The config parameter controls which agents are enabled, severity thresholds, and other settings.
func NewReviewCoordinator(orch *Orchestrator, sessionConfig ReviewConfig) *ReviewCoordinator {
	ctx, cancel := context.WithCancel(context.Background())

	// Get the config.ReviewConfig from the orchestrator if available
	var configReview *config.ReviewConfig
	if orch != nil && orch.Config() != nil {
		configReview = &orch.Config().Review
	}

	return &ReviewCoordinator{
		orch:          orch,
		session:       NewReviewSession("", sessionConfig),
		callbacks:     &ReviewCallbacks{},
		ctx:           ctx,
		cancelFunc:    cancel,
		runningAgents: make(map[string]*ReviewAgent),
		issuesChan:    make(chan ReviewIssue, 100), // Buffered to prevent blocking
		doneChan:      make(chan struct{}),
		paused:        false,
		configReview:  configReview,
	}
}

// SetCallbacks configures the event callbacks for the coordinator.
func (rc *ReviewCoordinator) SetCallbacks(callbacks *ReviewCallbacks) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.callbacks = callbacks
}

// SetTargetSession sets the implementer session ID that this review is watching.
func (rc *ReviewCoordinator) SetTargetSession(sessionID string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.session.TargetSessionID = sessionID
}

// Session returns the current review session.
func (rc *ReviewCoordinator) Session() *ReviewSession {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.session
}

// Start launches all configured review agents in parallel.
// Each agent runs independently and reports issues through the shared channel.
func (rc *ReviewCoordinator) Start() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.session.Phase == ReviewPhaseRunning {
		return fmt.Errorf("review session is already running")
	}

	rc.session.Phase = ReviewPhaseRunning

	// Start the issue collector goroutine
	go rc.collectIssues()

	// Determine max parallel agents
	maxParallel := 3 // default
	if rc.configReview != nil && rc.configReview.MaxParallelAgents > 0 {
		maxParallel = rc.configReview.MaxParallelAgents
	}

	// Create a semaphore to limit parallel agent launches
	sem := make(chan struct{}, maxParallel)

	// Launch each enabled agent type
	for _, agentType := range rc.session.Config.EnabledAgents {
		sem <- struct{}{} // acquire semaphore slot

		rc.wg.Add(1)
		go func(at ReviewAgentType) {
			defer rc.wg.Done()
			defer func() { <-sem }() // release semaphore slot

			if err := rc.startAgent(at); err != nil {
				// Log error but continue with other agents
				fmt.Printf("Warning: failed to start %s review agent: %v\n", at, err)
			}
		}(agentType)
	}

	// Start a goroutine to wait for all agents and signal completion
	go func() {
		rc.wg.Wait()
		close(rc.doneChan)
	}()

	return nil
}

// Stop gracefully stops all review agents and terminates the review session.
func (rc *ReviewCoordinator) Stop() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Cancel context to signal all agents to stop
	rc.cancelFunc()

	// Stop all running agents
	for agentID, agent := range rc.runningAgents {
		if rc.orch != nil {
			inst := rc.orch.GetInstance(agent.InstanceID)
			if inst != nil {
				_ = rc.orch.StopInstance(inst)
			}
		}
		delete(rc.runningAgents, agentID)
	}

	rc.session.Phase = ReviewPhaseComplete

	// Close the issues channel to signal the collector to stop
	close(rc.issuesChan)
}

// Pause temporarily suspends all review agents.
// Agents can be resumed later with Resume().
func (rc *ReviewCoordinator) Pause() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.paused || rc.session.Phase != ReviewPhaseRunning {
		return
	}

	rc.paused = true
	rc.session.Phase = ReviewPhasePaused

	// Pause all running agents via the orchestrator
	for _, agent := range rc.runningAgents {
		if rc.orch != nil && agent.InstanceID != "" {
			if mgr := rc.orch.GetInstanceManager(agent.InstanceID); mgr != nil {
				_ = mgr.Pause()
			}
		}
		agent.Status = "paused"
	}
}

// Resume restarts paused review agents.
func (rc *ReviewCoordinator) Resume() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if !rc.paused || rc.session.Phase != ReviewPhasePaused {
		return
	}

	rc.paused = false
	rc.session.Phase = ReviewPhaseRunning

	// Resume all paused agents via the orchestrator
	for _, agent := range rc.runningAgents {
		if rc.orch != nil && agent.InstanceID != "" {
			if mgr := rc.orch.GetInstanceManager(agent.InstanceID); mgr != nil {
				_ = mgr.Resume()
			}
		}
		agent.Status = "running"
	}
}

// GetIssues returns all issues collected from review agents.
// The returned slice is a copy, safe for concurrent access.
func (rc *ReviewCoordinator) GetIssues() []ReviewIssue {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	// Return a copy to prevent external modification
	issues := make([]ReviewIssue, len(rc.session.Issues))
	copy(issues, rc.session.Issues)
	return issues
}

// GetSummary generates a human-readable summary of the review findings.
func (rc *ReviewCoordinator) GetSummary() string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	var sb strings.Builder

	// Header
	sb.WriteString("# Code Review Summary\n\n")

	// Status
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", rc.session.Phase))
	sb.WriteString(fmt.Sprintf("**Started:** %s\n", rc.session.StartedAt.Format(time.RFC3339)))
	if rc.session.TargetSessionID != "" {
		sb.WriteString(fmt.Sprintf("**Target Session:** %s\n", rc.session.TargetSessionID))
	}
	sb.WriteString("\n")

	// Issue counts by severity
	counts := rc.session.IssueCountBySeverity()
	sb.WriteString("## Issue Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Critical:** %d\n", counts[string(SeverityCritical)]))
	sb.WriteString(fmt.Sprintf("- **Major:** %d\n", counts[string(SeverityMajor)]))
	sb.WriteString(fmt.Sprintf("- **Minor:** %d\n", counts[string(SeverityMinor)]))
	sb.WriteString(fmt.Sprintf("- **Info:** %d\n", counts[string(SeverityInfo)]))
	sb.WriteString(fmt.Sprintf("- **Total:** %d\n", rc.session.IssueCount()))
	sb.WriteString("\n")

	// Agent summary
	sb.WriteString("## Agents\n\n")
	for _, agent := range rc.session.Agents {
		sb.WriteString(fmt.Sprintf("- **%s:** %s (%d issues)\n",
			agent.Type, agent.Status, agent.IssuesFound))
	}
	sb.WriteString("\n")

	// Issues grouped by severity (most severe first)
	if len(rc.session.Issues) > 0 {
		sb.WriteString("## Issues\n\n")

		// Sort issues by severity (critical first)
		sortedIssues := make([]ReviewIssue, len(rc.session.Issues))
		copy(sortedIssues, rc.session.Issues)
		sort.Slice(sortedIssues, func(i, j int) bool {
			return SeverityOrder(sortedIssues[i].Severity) < SeverityOrder(sortedIssues[j].Severity)
		})

		for _, issue := range sortedIssues {
			sb.WriteString(fmt.Sprintf("### [%s] %s\n", strings.ToUpper(issue.Severity), issue.Title))
			sb.WriteString(fmt.Sprintf("**Type:** %s | **File:** %s", issue.Type, issue.File))
			if issue.LineStart > 0 {
				sb.WriteString(fmt.Sprintf(" (L%d", issue.LineStart))
				if issue.LineEnd > 0 && issue.LineEnd != issue.LineStart {
					sb.WriteString(fmt.Sprintf("-%d", issue.LineEnd))
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n\n")
			sb.WriteString(issue.Description)
			sb.WriteString("\n")
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("\n**Suggestion:** %s\n", issue.Suggestion))
			}
			sb.WriteString("\n---\n\n")
		}
	}

	return sb.String()
}

// startAgent launches a single review agent of the specified type.
// This creates a new Claude instance configured with a specialized review prompt.
func (rc *ReviewCoordinator) startAgent(agentType ReviewAgentType) error {
	// Check if context is cancelled
	select {
	case <-rc.ctx.Done():
		return rc.ctx.Err()
	default:
	}

	// Create the review agent record
	agent := &ReviewAgent{
		ID:          GenerateID(),
		Type:        agentType,
		Status:      "starting",
		IssuesFound: 0,
	}

	// Add to running agents
	rc.mu.Lock()
	rc.runningAgents[agent.ID] = agent
	rc.session.Agents = append(rc.session.Agents, *agent)
	rc.mu.Unlock()

	// Generate the specialized prompt for this agent type
	prompt := rc.generateAgentPrompt(agentType)

	// Create instance via orchestrator if available
	if rc.orch != nil && rc.orch.Session() != nil {
		inst, err := rc.orch.AddInstance(rc.orch.Session(), prompt)
		if err != nil {
			agent.Status = "error"
			return fmt.Errorf("failed to add review instance: %w", err)
		}

		agent.InstanceID = inst.ID

		// Start the instance
		if err := rc.orch.StartInstance(inst); err != nil {
			agent.Status = "error"
			return fmt.Errorf("failed to start review instance: %w", err)
		}

		agent.Status = "running"

		// Start monitoring the agent output
		rc.wg.Add(1)
		go rc.monitorAgent(agent.ID)
	} else {
		// For testing without a real orchestrator, simulate running
		agent.Status = "running"
		rc.wg.Add(1)
		go rc.monitorAgent(agent.ID)
	}

	return nil
}

// monitorAgent watches a review agent's output and collects issues.
// This runs as a goroutine for each agent.
func (rc *ReviewCoordinator) monitorAgent(agentID string) {
	defer rc.wg.Done()

	rc.mu.RLock()
	agent, ok := rc.runningAgents[agentID]
	rc.mu.RUnlock()

	if !ok {
		return
	}

	// Monitor loop - check for completion or context cancellation
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-rc.ctx.Done():
			// Context cancelled, stop monitoring
			rc.markAgentComplete(agentID)
			return

		case <-ticker.C:
			// Check agent status via orchestrator
			if rc.orch != nil && agent.InstanceID != "" {
				inst := rc.orch.GetInstance(agent.InstanceID)
				if inst == nil {
					rc.markAgentComplete(agentID)
					return
				}

				// Check if instance has completed
				if inst.Status == StatusCompleted || inst.Status == StatusError {
					rc.markAgentComplete(agentID)
					return
				}

				// Parse output for issues (implementation would parse Claude's output)
				// This is a placeholder - real implementation would parse structured output
				rc.parseAgentOutput(agentID, inst.Output)
			}
		}
	}
}

// parseAgentOutput extracts review issues from agent output.
// This is called periodically as the agent produces output.
func (rc *ReviewCoordinator) parseAgentOutput(agentID string, output []byte) {
	// This is a placeholder for actual output parsing logic.
	// In a real implementation, this would:
	// 1. Parse JSON or structured output from the Claude agent
	// 2. Extract issue details (file, line, severity, description)
	// 3. Create ReviewIssue objects and send to issuesChan

	// The actual parsing will depend on the prompt format and expected output structure.
	// For now, this serves as a hook point for the integration.
	_ = agentID
	_ = output
}

// markAgentComplete marks an agent as completed and notifies callbacks.
func (rc *ReviewCoordinator) markAgentComplete(agentID string) {
	rc.mu.Lock()
	agent, ok := rc.runningAgents[agentID]
	if !ok {
		rc.mu.Unlock()
		return
	}

	agent.Status = "completed"

	// Update agent in session
	for i := range rc.session.Agents {
		if rc.session.Agents[i].ID == agentID {
			rc.session.Agents[i].Status = "completed"
			rc.session.Agents[i].IssuesFound = agent.IssuesFound
			break
		}
	}

	delete(rc.runningAgents, agentID)

	callbacks := rc.callbacks
	issueCount := agent.IssuesFound
	rc.mu.Unlock()

	// Notify callback
	if callbacks != nil && callbacks.OnAgentComplete != nil {
		callbacks.OnAgentComplete(agentID, issueCount)
	}
}

// collectIssues runs as a goroutine to collect issues from all agents.
func (rc *ReviewCoordinator) collectIssues() {
	for {
		select {
		case <-rc.ctx.Done():
			return

		case issue, ok := <-rc.issuesChan:
			if !ok {
				// Channel closed, check if all agents are done
				rc.finalizeReview()
				return
			}

			rc.processIssue(issue)

		case <-rc.doneChan:
			// All agents completed, drain remaining issues and finalize
			for issue := range rc.issuesChan {
				rc.processIssue(issue)
			}
			rc.finalizeReview()
			return
		}
	}
}

// processIssue handles a single issue from a review agent.
func (rc *ReviewCoordinator) processIssue(issue ReviewIssue) {
	rc.mu.Lock()

	// Check severity threshold
	threshold := rc.session.Config.SeverityThreshold
	if !MeetsSeverityThreshold(issue.Severity, threshold) {
		rc.mu.Unlock()
		return
	}

	// Add issue to session
	rc.session.AddIssue(issue)

	// Update agent issue count
	if agent := rc.runningAgents[string(issue.Type)]; agent != nil {
		agent.IssuesFound++
	}

	callbacks := rc.callbacks
	autoPause := rc.session.Config.AutoPauseImplementer
	rc.mu.Unlock()

	// Notify callbacks
	if callbacks != nil {
		if callbacks.OnIssueFound != nil {
			callbacks.OnIssueFound(issue)
		}

		// Check for critical issues
		if issue.Severity == string(SeverityCritical) {
			if callbacks.OnCriticalIssue != nil {
				callbacks.OnCriticalIssue(issue)
			}

			// Auto-pause implementer if configured
			if autoPause {
				rc.pauseImplementer()
			}
		}
	}
}

// pauseImplementer pauses the target implementer session.
// This is triggered when critical issues are found and auto-pause is enabled.
func (rc *ReviewCoordinator) pauseImplementer() {
	rc.mu.RLock()
	targetID := rc.session.TargetSessionID
	rc.mu.RUnlock()

	if targetID == "" || rc.orch == nil {
		return
	}

	// Find and pause the target instance
	if inst := rc.orch.GetInstance(targetID); inst != nil {
		if mgr := rc.orch.GetInstanceManager(targetID); mgr != nil {
			_ = mgr.Pause()
		}
	}
}

// finalizeReview completes the review session and notifies callbacks.
func (rc *ReviewCoordinator) finalizeReview() {
	rc.mu.Lock()
	if rc.session.Phase == ReviewPhaseComplete {
		rc.mu.Unlock()
		return
	}

	rc.session.Phase = ReviewPhaseComplete
	totalIssues := rc.session.IssueCount()
	callbacks := rc.callbacks
	rc.mu.Unlock()

	summary := rc.GetSummary()

	// Notify callback
	if callbacks != nil && callbacks.OnReviewComplete != nil {
		callbacks.OnReviewComplete(totalIssues, summary)
	}
}

// generateAgentPrompt creates a specialized prompt for a review agent type.
// The prompt instructs Claude to analyze code for specific concerns.
func (rc *ReviewCoordinator) generateAgentPrompt(agentType ReviewAgentType) string {
	// Check for custom prompts in config
	if rc.configReview != nil {
		switch agentType {
		case SecurityReview:
			if rc.configReview.Prompts.Security != "" {
				return rc.configReview.Prompts.Security
			}
		case PerformanceReview:
			if rc.configReview.Prompts.Performance != "" {
				return rc.configReview.Prompts.Performance
			}
		case StyleReview:
			if rc.configReview.Prompts.Style != "" {
				return rc.configReview.Prompts.Style
			}
		case TestCoverageReview:
			if rc.configReview.Prompts.Tests != "" {
				return rc.configReview.Prompts.Tests
			}
		case GeneralReview:
			if rc.configReview.Prompts.General != "" {
				return rc.configReview.Prompts.General
			}
		}
	}

	// Default prompts for each agent type
	basePrompt := `You are a specialized code review agent. Analyze the code in this repository and report issues in the following JSON format:

{
  "issues": [
    {
      "severity": "critical|major|minor|info",
      "file": "path/to/file.go",
      "line_start": 10,
      "line_end": 15,
      "title": "Brief issue title",
      "description": "Detailed explanation of the issue",
      "suggestion": "How to fix the issue"
    }
  ]
}

Focus ONLY on %s issues. Be thorough but avoid false positives.
`

	switch agentType {
	case SecurityReview:
		return fmt.Sprintf(basePrompt, "SECURITY") + `
Look for:
- SQL injection vulnerabilities
- Command injection
- Path traversal
- Sensitive data exposure
- Authentication/authorization flaws
- Insecure cryptographic practices
- OWASP Top 10 vulnerabilities`

	case PerformanceReview:
		return fmt.Sprintf(basePrompt, "PERFORMANCE") + `
Look for:
- N+1 query patterns
- Memory leaks
- Inefficient algorithms (O(n^2) when O(n) is possible)
- Missing caching opportunities
- Unnecessary allocations
- Blocking operations in hot paths
- Resource exhaustion risks`

	case StyleReview:
		return fmt.Sprintf(basePrompt, "CODE STYLE and QUALITY") + `
Look for:
- Inconsistent naming conventions
- Missing or incorrect documentation
- Dead code
- Overly complex functions (high cyclomatic complexity)
- Code duplication
- Magic numbers/strings
- Violation of project style guidelines`

	case TestCoverageReview:
		return fmt.Sprintf(basePrompt, "TEST COVERAGE") + `
Look for:
- Missing tests for public functions
- Untested error paths
- Missing edge case coverage
- Flaky test patterns
- Tests without assertions
- Missing integration tests for critical paths`

	case GeneralReview:
		return fmt.Sprintf(basePrompt, "GENERAL CODE QUALITY") + `
Look for:
- Logic errors
- Race conditions
- Error handling issues
- API design problems
- Architectural concerns
- Maintainability issues`

	default:
		return fmt.Sprintf(basePrompt, "GENERAL")
	}
}

// AddIssue allows external code to add an issue to the review session.
// This is useful for integrating issues from other sources.
func (rc *ReviewCoordinator) AddIssue(issue ReviewIssue) {
	select {
	case rc.issuesChan <- issue:
	case <-rc.ctx.Done():
	}
}

// IsRunning returns true if the review session is currently active.
func (rc *ReviewCoordinator) IsRunning() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.session.Phase == ReviewPhaseRunning
}

// IsPaused returns true if the review session is currently paused.
func (rc *ReviewCoordinator) IsPaused() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.session.Phase == ReviewPhasePaused
}

// IsComplete returns true if the review session has finished.
func (rc *ReviewCoordinator) IsComplete() bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.session.Phase == ReviewPhaseComplete
}

// GetAgentStatus returns the status of all review agents.
func (rc *ReviewCoordinator) GetAgentStatus() map[ReviewAgentType]string {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	status := make(map[ReviewAgentType]string)
	for _, agent := range rc.session.Agents {
		status[agent.Type] = agent.Status
	}
	return status
}

// WaitForCompletion blocks until all review agents have finished.
func (rc *ReviewCoordinator) WaitForCompletion() {
	<-rc.doneChan
}

// WaitForCompletionWithTimeout blocks until all agents finish or timeout expires.
// Returns true if completed, false if timed out.
func (rc *ReviewCoordinator) WaitForCompletionWithTimeout(timeout time.Duration) bool {
	select {
	case <-rc.doneChan:
		return true
	case <-time.After(timeout):
		return false
	}
}

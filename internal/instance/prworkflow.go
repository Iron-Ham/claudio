package instance

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/capture"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/tmux"
)

// PRWorkflowConfig holds configuration for the PR workflow
type PRWorkflowConfig struct {
	UseAI      bool
	Draft      bool
	AutoRebase bool
	TmuxWidth  int
	TmuxHeight int
}

// PRWorkflowCallback is called when the PR workflow completes
type PRWorkflowCallback func(instanceID string, success bool, output string)

// PRWorkflow manages the commit-push-PR workflow after an instance is stopped
type PRWorkflow struct {
	instanceID  string
	sessionID   string // Claudio session ID (for multi-session support)
	workdir     string
	branch      string
	task        string
	sessionName string // tmux session name
	socketName  string // tmux socket for crash isolation
	config      PRWorkflowConfig
	outputBuf   *capture.RingBuffer
	logger      *logging.Logger
	startTime   time.Time // for tracking workflow duration

	mu          sync.RWMutex
	running     bool
	doneChan    chan struct{}
	captureTick *time.Ticker
	callback    PRWorkflowCallback
}

// NewPRWorkflow creates a new PR workflow manager.
// Uses the instance's socket (claudio-{instanceID}) for crash isolation.
func NewPRWorkflow(instanceID, workdir, branch, task string, cfg PRWorkflowConfig) *PRWorkflow {
	return &PRWorkflow{
		instanceID:  instanceID,
		workdir:     workdir,
		branch:      branch,
		task:        task,
		sessionName: fmt.Sprintf("claudio-%s-pr", instanceID),
		socketName:  tmux.InstanceSocketName(instanceID), // Use instance socket for crash isolation
		config:      cfg,
		outputBuf:   capture.NewRingBuffer(100000), // 100KB buffer
		doneChan:    make(chan struct{}),
	}
}

// NewPRWorkflowWithSession creates a new PR workflow manager with session-scoped tmux naming.
// The tmux session will be named claudio-{sessionID}-{instanceID}-pr to prevent collisions
// when multiple Claudio sessions are running simultaneously.
// Uses the instance's socket (claudio-{instanceID}) for crash isolation.
func NewPRWorkflowWithSession(sessionID, instanceID, workdir, branch, task string, cfg PRWorkflowConfig) *PRWorkflow {
	// Use session-scoped naming if sessionID is provided
	var sessionName string
	if sessionID != "" {
		sessionName = fmt.Sprintf("claudio-%s-%s-pr", sessionID, instanceID)
	} else {
		sessionName = fmt.Sprintf("claudio-%s-pr", instanceID)
	}

	return &PRWorkflow{
		instanceID:  instanceID,
		sessionID:   sessionID,
		workdir:     workdir,
		branch:      branch,
		task:        task,
		sessionName: sessionName,
		socketName:  tmux.InstanceSocketName(instanceID), // Use instance socket for crash isolation
		config:      cfg,
		outputBuf:   capture.NewRingBuffer(100000), // 100KB buffer
		doneChan:    make(chan struct{}),
	}
}

// NewPRWorkflowWithSocket creates a new PR workflow manager with explicit socket isolation.
// The socketName should match the parent instance's socket for crash isolation.
func NewPRWorkflowWithSocket(instanceID, socketName, workdir, branch, task string, cfg PRWorkflowConfig) *PRWorkflow {
	return &PRWorkflow{
		instanceID:  instanceID,
		workdir:     workdir,
		branch:      branch,
		task:        task,
		sessionName: fmt.Sprintf("claudio-%s-pr", instanceID),
		socketName:  socketName,
		config:      cfg,
		outputBuf:   capture.NewRingBuffer(100000), // 100KB buffer
		doneChan:    make(chan struct{}),
	}
}

// SetCallback sets the completion callback
func (p *PRWorkflow) SetCallback(cb PRWorkflowCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callback = cb
}

// SetLogger sets the logger for the PR workflow.
// If set, the workflow will log events at appropriate levels.
func (p *PRWorkflow) SetLogger(logger *logging.Logger) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.logger = logger
}

// logInfo logs an info message if logger is configured
func (p *PRWorkflow) logInfo(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Info(msg, args...)
	}
}

// logDebug logs a debug message if logger is configured
func (p *PRWorkflow) logDebug(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}

// logError logs an error message if logger is configured
func (p *PRWorkflow) logError(msg string, args ...any) {
	if p.logger != nil {
		p.logger.Error(msg, args...)
	}
}

// Start launches the PR workflow in a tmux session
func (p *PRWorkflow) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("PR workflow already running")
	}

	// Record start time for duration tracking
	p.startTime = time.Now()

	// Log workflow start
	p.logInfo("PR workflow started",
		"instance_id", p.instanceID,
		"branch", p.branch,
	)

	// Kill any existing session with this name (cleanup from previous run)
	p.logDebug("cleaning up existing tmux session", "session_name", p.sessionName)
	_ = tmux.CommandWithSocket(p.socketName, "kill-session", "-t", p.sessionName).Run()

	// Set history-limit BEFORE creating session so the new pane inherits it.
	// tmux's history-limit only affects newly created panes, not existing ones.
	// Use 50000 lines for generous scrollback in the PR workflow pane.
	if err := tmux.CommandWithSocket(p.socketName, "set-option", "-g", "history-limit", "50000").Run(); err != nil {
		p.logDebug("failed to set global history-limit for tmux", "error", err.Error())
	}

	// Create a new detached tmux session
	p.logDebug("creating tmux session",
		"session_name", p.sessionName,
		"width", p.config.TmuxWidth,
		"height", p.config.TmuxHeight,
	)
	createCmd := tmux.CommandWithSocket(p.socketName,
		"new-session",
		"-d",
		"-s", p.sessionName,
		"-x", fmt.Sprintf("%d", p.config.TmuxWidth),
		"-y", fmt.Sprintf("%d", p.config.TmuxHeight),
	)
	createCmd.Dir = p.workdir
	createCmd.Env = append(createCmd.Env, "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		p.logError("failed to create tmux session",
			"error", err.Error(),
			"session_name", p.sessionName,
		)
		return fmt.Errorf("failed to create tmux session for PR workflow: %w", err)
	}

	// Set up additional tmux session options for color support
	p.logDebug("configuring tmux session options", "session_name", p.sessionName)
	_ = tmux.CommandWithSocket(p.socketName, "set-option", "-t", p.sessionName, "default-terminal", "xterm-256color").Run()

	// Build and send the command
	var cmd string
	if p.config.UseAI {
		// Use Claude to run the /commit-push-pr skill
		cmd = p.buildClaudeCommand()
		p.logDebug("using AI-assisted PR workflow", "use_ai", true)
	} else {
		// Use direct shell commands
		cmd = p.buildShellCommand()
		p.logDebug("using shell-based PR workflow", "use_ai", false)
	}

	p.logDebug("sending workflow command to tmux session")

	sendCmd := tmux.CommandWithSocket(p.socketName,
		"send-keys",
		"-t", p.sessionName,
		cmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		p.logError("failed to send command to tmux session",
			"error", err.Error(),
			"session_name", p.sessionName,
		)
		_ = tmux.CommandWithSocket(p.socketName, "kill-session", "-t", p.sessionName).Run()
		return fmt.Errorf("failed to start PR workflow command: %w", err)
	}

	p.running = true

	// Start background goroutine to monitor completion
	p.captureTick = time.NewTicker(500 * time.Millisecond)
	go p.monitorLoop()

	return nil
}

// buildClaudeCommand builds the Claude command for AI-assisted PR creation
func (p *PRWorkflow) buildClaudeCommand() string {
	// Use Claude's --print mode with a direct prompt for commit/push/PR
	// This avoids needing the interactive /commit-push-pr skill
	prompt := fmt.Sprintf(`You are helping with a git workflow. The task was: %q

Please do the following:
1. Check if there are any uncommitted changes with git status
2. If there are uncommitted changes, create a commit with an appropriate message following conventional commits
3. Push the branch to remote
4. Create a pull request using gh pr create

Use the branch name: %s
If creating a draft PR, add the --draft flag.

Be concise and just execute the commands. Exit when done.`, p.task, p.branch)

	// Build flags
	flags := "--dangerously-skip-permissions"
	if p.config.Draft {
		prompt += "\n\nCreate the PR as a draft."
	}

	return fmt.Sprintf("claude %s %q; exit", flags, prompt)
}

// buildShellCommand builds direct shell commands for PR creation without AI
func (p *PRWorkflow) buildShellCommand() string {
	// Build a shell script that handles commit, push, and PR creation
	var draftFlag string
	if p.config.Draft {
		draftFlag = "--draft "
	}

	// The script:
	// 1. Stages all changes
	// 2. Commits with a simple message based on task
	// 3. Pushes to remote
	// 4. Creates PR via gh
	script := fmt.Sprintf(`
# PR workflow for: %s
set -e

# Check for changes
if git status --porcelain | grep -q .; then
    echo "Staging and committing changes..."
    git add -A
    git commit -m "feat: %s"
fi

# Push to remote
echo "Pushing branch..."
git push -u origin %s

# Create PR
echo "Creating pull request..."
gh pr create --title "feat: %s" --body "## Task\n%s" %s--head %s

echo ""
echo "PR workflow completed!"
exit 0
`, p.task, truncateForCommit(p.task), p.branch, truncateForCommit(p.task), p.task, draftFlag, p.branch)

	return script
}

// truncateForCommit truncates a task description for use in a commit message
func truncateForCommit(s string) string {
	if len(s) > 50 {
		return s[:47] + "..."
	}
	return s
}

// monitorLoop monitors the tmux session for completion
func (p *PRWorkflow) monitorLoop() {
	for {
		select {
		case <-p.doneChan:
			return
		case <-p.captureTick.C:
			p.mu.RLock()
			if !p.running {
				p.mu.RUnlock()
				return
			}
			sessionName := p.sessionName
			callback := p.callback
			instanceID := p.instanceID
			startTime := p.startTime
			p.mu.RUnlock()

			// Capture output
			captureCmd := tmux.CommandWithSocket(p.socketName,
				"capture-pane",
				"-t", sessionName,
				"-p",
				"-e",
				"-S", "-",
				"-E", "-",
			)
			output, err := captureCmd.Output()
			if err == nil {
				p.outputBuf.Reset()
				_, _ = p.outputBuf.Write(output)
			}

			// Check if the session is still running
			checkCmd := tmux.CommandWithSocket(p.socketName, "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended - workflow completed
				p.mu.Lock()
				p.running = false
				p.mu.Unlock()

				// Get final output
				finalOutput := string(p.outputBuf.Bytes())
				success := p.checkSuccess(finalOutput)

				// Calculate duration
				durationMs := time.Since(startTime).Milliseconds()

				// Log completion with PR URL if found
				prURL := extractPRURL(finalOutput)
				if success {
					if prURL != "" {
						p.logInfo("PR created", "pr_url", prURL)
					}
					p.logInfo("PR workflow completed",
						"success", true,
						"duration_ms", durationMs,
					)
				} else {
					p.logError("PR workflow failed",
						"success", false,
						"duration_ms", durationMs,
					)
				}

				// Invoke callback
				if callback != nil {
					callback(instanceID, success, finalOutput)
				}
				return
			}
		}
	}
}

// checkSuccess analyzes output to determine if the PR workflow succeeded
func (p *PRWorkflow) checkSuccess(output string) bool {
	// Look for success indicators in the output
	// gh pr create outputs the PR URL on success
	return containsAny(output, []string{
		"github.com",
		"pull/",
		"Pull request created",
		"PR workflow completed",
	})
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// extractPRURL extracts a GitHub PR URL from the output text.
// Returns empty string if no PR URL is found.
func extractPRURL(output string) string {
	// Look for github.com URLs with /pull/ in them
	// gh pr create outputs the PR URL on a line by itself
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "github.com") && strings.Contains(line, "/pull/") {
			// Extract the URL portion (handles lines like "https://github.com/org/repo/pull/123")
			if idx := strings.Index(line, "https://github.com"); idx != -1 {
				url := line[idx:]
				// Trim any trailing whitespace or characters after the URL
				if spaceIdx := strings.IndexAny(url, " \t\n\r"); spaceIdx != -1 {
					url = url[:spaceIdx]
				}
				return url
			}
		}
	}
	return ""
}

// Stop terminates the PR workflow tmux session
func (p *PRWorkflow) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return nil
	}

	// Signal stop to monitor loop
	select {
	case <-p.doneChan:
	default:
		close(p.doneChan)
	}

	// Stop the ticker
	if p.captureTick != nil {
		p.captureTick.Stop()
	}

	// Kill the tmux session
	_ = tmux.CommandWithSocket(p.socketName, "kill-session", "-t", p.sessionName).Run()

	p.running = false
	return nil
}

// SocketName returns the tmux socket name used for this PR workflow.
func (p *PRWorkflow) SocketName() string {
	return p.socketName
}

// Running returns whether the PR workflow is running
func (p *PRWorkflow) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetOutput returns the buffered output from the PR workflow
func (p *PRWorkflow) GetOutput() []byte {
	return p.outputBuf.Bytes()
}

// SessionName returns the tmux session name
func (p *PRWorkflow) SessionName() string {
	return p.sessionName
}

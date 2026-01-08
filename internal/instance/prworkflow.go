package instance

import (
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// PRWorkflowConfig holds configuration for the PR workflow
type PRWorkflowConfig struct {
	UseAI       bool
	Draft       bool
	AutoRebase  bool
	TmuxWidth   int
	TmuxHeight  int
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
	config      PRWorkflowConfig
	outputBuf   *RingBuffer

	mu          sync.RWMutex
	running     bool
	doneChan    chan struct{}
	captureTick *time.Ticker
	callback    PRWorkflowCallback
}

// NewPRWorkflow creates a new PR workflow manager.
// Uses legacy tmux naming (claudio-{instanceID}-pr) for backwards compatibility.
func NewPRWorkflow(instanceID, workdir, branch, task string, cfg PRWorkflowConfig) *PRWorkflow {
	return &PRWorkflow{
		instanceID:  instanceID,
		workdir:     workdir,
		branch:      branch,
		task:        task,
		sessionName: fmt.Sprintf("claudio-%s-pr", instanceID),
		config:      cfg,
		outputBuf:   NewRingBuffer(100000), // 100KB buffer
		doneChan:    make(chan struct{}),
	}
}

// NewPRWorkflowWithSession creates a new PR workflow manager with session-scoped tmux naming.
// The tmux session will be named claudio-{sessionID}-{instanceID}-pr to prevent collisions
// when multiple Claudio sessions are running simultaneously.
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
		config:      cfg,
		outputBuf:   NewRingBuffer(100000), // 100KB buffer
		doneChan:    make(chan struct{}),
	}
}

// SetCallback sets the completion callback
func (p *PRWorkflow) SetCallback(cb PRWorkflowCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.callback = cb
}

// Start launches the PR workflow in a tmux session
func (p *PRWorkflow) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("PR workflow already running")
	}

	// Kill any existing session with this name (cleanup from previous run)
	_ = exec.Command("tmux", "kill-session", "-t", p.sessionName).Run()

	// Create a new detached tmux session
	createCmd := exec.Command("tmux",
		"new-session",
		"-d",
		"-s", p.sessionName,
		"-x", fmt.Sprintf("%d", p.config.TmuxWidth),
		"-y", fmt.Sprintf("%d", p.config.TmuxHeight),
	)
	createCmd.Dir = p.workdir
	createCmd.Env = append(createCmd.Env, "TERM=xterm-256color")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session for PR workflow: %w", err)
	}

	// Set up tmux for color support
	_ = exec.Command("tmux", "set-option", "-t", p.sessionName, "history-limit", "10000").Run()
	_ = exec.Command("tmux", "set-option", "-t", p.sessionName, "default-terminal", "xterm-256color").Run()

	// Build and send the command
	var cmd string
	if p.config.UseAI {
		// Use Claude to run the /commit-push-pr skill
		cmd = p.buildClaudeCommand()
	} else {
		// Use direct shell commands
		cmd = p.buildShellCommand()
	}

	sendCmd := exec.Command("tmux",
		"send-keys",
		"-t", p.sessionName,
		cmd,
		"Enter",
	)
	if err := sendCmd.Run(); err != nil {
		_ = exec.Command("tmux", "kill-session", "-t", p.sessionName).Run()
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
			p.mu.RUnlock()

			// Capture output
			captureCmd := exec.Command("tmux",
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
			checkCmd := exec.Command("tmux", "has-session", "-t", sessionName)
			if checkCmd.Run() != nil {
				// Session ended - workflow completed
				p.mu.Lock()
				p.running = false
				p.mu.Unlock()

				// Get final output
				finalOutput := string(p.outputBuf.Bytes())
				success := p.checkSuccess(finalOutput)

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
	_ = exec.Command("tmux", "kill-session", "-t", p.sessionName).Run()

	p.running = false
	return nil
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

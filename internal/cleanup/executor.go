package cleanup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Iron-Ham/claudio/internal/session"
	"github.com/Iron-Ham/claudio/internal/tmux"
	"github.com/Iron-Ham/claudio/internal/worktree"
)

// Executor runs cleanup jobs using their snapshotted resources
type Executor struct {
	job *Job
	wt  *worktree.Manager
}

// NewExecutor creates a new executor for the given job
func NewExecutor(job *Job) (*Executor, error) {
	wt, err := worktree.New(job.BaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create worktree manager: %w", err)
	}

	return &Executor{
		job: job,
		wt:  wt,
	}, nil
}

// Execute runs the cleanup job using ONLY the snapshotted resources.
// This ensures that resources created after the snapshot are not affected.
func (e *Executor) Execute() error {
	job := e.job

	// Mark job as running
	job.Status = JobStatusRunning
	job.StartedAt = time.Now()
	if err := job.Save(); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	results := &JobResults{}
	var errors []string

	// Clean worktrees from snapshot
	if job.CleanAll || job.Worktrees {
		removed, errs := e.cleanWorktrees()
		results.WorktreesRemoved = removed
		errors = append(errors, errs...)
	}

	// Clean branches from snapshot
	if job.CleanAll || job.Branches {
		deleted, errs := e.cleanBranches()
		results.BranchesDeleted = deleted
		errors = append(errors, errs...)
	}

	// Clean tmux sessions from snapshot
	if job.CleanAll || job.Tmux {
		killed, errs := e.cleanOrphanedTmuxSessions()
		results.TmuxSessionsKilled = killed
		errors = append(errors, errs...)
	}

	// Handle --all-sessions: kill ALL tmux sessions that were snapshotted
	if job.AllSessions {
		killed, errs := e.cleanAllTmuxSessions()
		results.TmuxSessionsKilled += killed
		errors = append(errors, errs...)
	}

	// Clean empty sessions from snapshot
	if job.CleanAll || job.Sessions || job.DeepClean {
		removed, errs := e.cleanEmptySessions()
		results.SessionsRemoved = removed
		errors = append(errors, errs...)
	}

	// Handle --deep-clean with --all-sessions: remove ALL sessions from snapshot
	if job.DeepClean && job.AllSessions {
		removed, errs := e.cleanAllSessions()
		results.SessionsRemoved += removed
		errors = append(errors, errs...)
	}

	results.TotalRemoved = results.WorktreesRemoved + results.BranchesDeleted +
		results.TmuxSessionsKilled + results.SessionsRemoved
	results.Errors = errors

	// Update job status
	job.Results = results
	job.Status = JobStatusCompleted
	job.EndedAt = time.Now()

	if len(errors) > 0 && results.TotalRemoved == 0 {
		job.Status = JobStatusFailed
		job.Error = fmt.Sprintf("all operations failed: %d errors", len(errors))
	}

	if err := job.Save(); err != nil {
		return fmt.Errorf("failed to save job results: %w", err)
	}

	return nil
}

// cleanWorktrees removes worktrees from the snapshot
func (e *Executor) cleanWorktrees() (int, []string) {
	var removed int
	var errors []string

	for _, sw := range e.job.StaleWorktrees {
		// Verify worktree still exists before removing
		if _, err := os.Stat(sw.Path); os.IsNotExist(err) {
			continue // Already gone
		}

		// Safety: skip worktrees with uncommitted changes unless forced
		if sw.HasUncommitted && !e.job.Force {
			errors = append(errors, fmt.Sprintf("skipped %s: has uncommitted changes", filepath.Base(sw.Path)))
			continue
		}

		if err := e.wt.Remove(sw.Path); err != nil {
			errors = append(errors, fmt.Sprintf("failed to remove worktree %s: %v", filepath.Base(sw.Path), err))
			continue
		}
		removed++

		// Also delete the branch if it's local-only
		if sw.Branch != "" && !sw.ExistsOnRemote {
			if err := e.wt.DeleteBranch(sw.Branch); err != nil {
				errors = append(errors, fmt.Sprintf("failed to delete branch %s: %v", sw.Branch, err))
			}
		}
	}

	return removed, errors
}

// cleanBranches deletes branches from the snapshot
func (e *Executor) cleanBranches() (int, []string) {
	var deleted int
	var errors []string

	// Build set of branches already deleted with worktrees
	deletedWithWorktree := make(map[string]bool)
	for _, sw := range e.job.StaleWorktrees {
		if sw.Branch != "" && !sw.ExistsOnRemote {
			deletedWithWorktree[sw.Branch] = true
		}
	}

	for _, branch := range e.job.StaleBranches {
		// Skip if already deleted with worktree
		if deletedWithWorktree[branch] {
			continue
		}

		// Verify branch still exists before deleting
		cmd := exec.Command("git", "-C", e.job.BaseDir, "rev-parse", "--verify", branch)
		if err := cmd.Run(); err != nil {
			continue // Branch no longer exists
		}

		if err := e.wt.DeleteBranch(branch); err != nil {
			errors = append(errors, fmt.Sprintf("failed to delete branch %s: %v", branch, err))
			continue
		}
		deleted++
	}

	return deleted, errors
}

// cleanOrphanedTmuxSessions kills orphaned tmux sessions from the snapshot
func (e *Executor) cleanOrphanedTmuxSessions() (int, []string) {
	return e.killTmuxSessions(e.job.OrphanedTmuxSess)
}

// cleanAllTmuxSessions kills all claudio tmux sessions from the snapshot
func (e *Executor) cleanAllTmuxSessions() (int, []string) {
	return e.killTmuxSessions(e.job.AllTmuxSessions)
}

// killTmuxSessions kills the specified tmux sessions
func (e *Executor) killTmuxSessions(sessions []string) (int, []string) {
	var killed int
	var errors []string

	for _, sess := range sessions {
		if !tmuxSessionExists(sess) {
			continue
		}

		killCmd := tmux.Command("kill-session", "-t", sess)
		if err := killCmd.Run(); err != nil {
			errors = append(errors, fmt.Sprintf("failed to kill tmux session %s: %v", sess, err))
			continue
		}
		killed++
	}

	return killed, errors
}

// cleanEmptySessions removes empty sessions from the snapshot
func (e *Executor) cleanEmptySessions() (int, []string) {
	return e.removeSessions(e.job.EmptySessions, nil)
}

// cleanAllSessions removes all sessions from the snapshot (for --deep-clean --all-sessions)
func (e *Executor) cleanAllSessions() (int, []string) {
	// Build set of sessions already removed as empty
	alreadyRemoved := make(map[string]bool, len(e.job.EmptySessions))
	for _, s := range e.job.EmptySessions {
		alreadyRemoved[s.ID] = true
	}

	return e.removeSessions(e.job.AllSessionIDs, alreadyRemoved)
}

// removeSessions removes the specified sessions, optionally skipping those in the skip set
func (e *Executor) removeSessions(sessions []StaleSession, skip map[string]bool) (int, []string) {
	var removed int
	var errors []string

	for _, s := range sessions {
		if skip != nil && skip[s.ID] {
			continue
		}

		sessionDir := session.GetSessionDir(e.job.BaseDir, s.ID)
		if _, err := os.Stat(sessionDir); os.IsNotExist(err) {
			continue
		}

		if _, isLocked := session.IsLocked(sessionDir); isLocked {
			errors = append(errors, fmt.Sprintf("skipped locked session %s", session.TruncateID(s.ID, 8)))
			continue
		}

		if err := session.RemoveSession(e.job.BaseDir, s.ID); err != nil {
			errors = append(errors, fmt.Sprintf("failed to remove session %s: %v", session.TruncateID(s.ID, 8), err))
			continue
		}
		removed++
	}

	return removed, errors
}

// tmuxSessionExists checks if a tmux session exists
func tmuxSessionExists(sessionName string) bool {
	cmd := tmux.Command("has-session", "-t", sessionName)
	return cmd.Run() == nil
}

// ListAllClaudioTmuxSessions returns all claudio-* tmux sessions
func ListAllClaudioTmuxSessions() []string {
	cmd := tmux.Command("list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var sessions []string
	for sess := range strings.SplitSeq(strings.TrimSpace(string(output)), "\n") {
		sess = strings.TrimSpace(sess)
		if sess != "" && strings.HasPrefix(sess, "claudio-") {
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

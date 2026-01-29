// Package msg provides command factory functions that create tea.Cmd values.
//
// These functions are pure factories that create commands returning message types
// defined in this package. They handle async operations like adding tasks,
// checking completion files, and system notifications.

package msg

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/ralph"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
	"github.com/Iron-Ham/claudio/internal/tui/output"
	"github.com/Iron-Ham/claudio/internal/tui/view"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"
)

// Tick returns a command that sends a TickMsg after 100ms.
// This drives periodic UI updates and polling.
func Tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// RingBell returns a command that outputs a terminal bell character.
// This forwards bells from tmux sessions to the parent terminal.
func RingBell() tea.Cmd {
	return func() tea.Msg {
		// Write the bell character directly to stdout
		// This works even when Bubbletea is in alt-screen mode
		_, _ = os.Stdout.Write([]byte{'\a'})
		return nil
	}
}

// NotifyUser returns a command that notifies the user via bell and optional sound.
// Used to alert the user when ultraplan needs input (e.g., plan ready, synthesis ready).
func NotifyUser() tea.Cmd {
	return func() tea.Msg {
		if !viper.GetBool("ultraplan.notifications.enabled") {
			return nil
		}

		// Always ring terminal bell
		_, _ = os.Stdout.Write([]byte{'\a'})

		// Optionally play system sound on macOS
		if runtime.GOOS == "darwin" && viper.GetBool("ultraplan.notifications.use_sound") {
			soundPath := viper.GetString("ultraplan.notifications.sound_path")
			if soundPath == "" {
				// Use system alert sound (user's configured sound in System Settings)
				_ = exec.Command("osascript", "-e", "beep").Start()
			} else {
				// Use custom sound file
				_ = exec.Command("afplay", soundPath).Start()
			}
		}
		return nil
	}
}

// AddTaskAsync returns a command that adds a task asynchronously.
// This prevents the UI from blocking while git creates the worktree.
func AddTaskAsync(o *orchestrator.Orchestrator, session *orchestrator.Session, task string) tea.Cmd {
	return func() tea.Msg {
		if o == nil {
			return TaskAddedMsg{Instance: nil, Err: fmt.Errorf("orchestrator is nil")}
		}
		if session == nil {
			return TaskAddedMsg{Instance: nil, Err: fmt.Errorf("session is nil")}
		}
		inst, err := o.AddInstance(session, task)
		return TaskAddedMsg{Instance: inst, Err: err}
	}
}

// AddTaskFromBranchAsync returns a command that adds a task from a specific base branch asynchronously.
// The baseBranch parameter specifies which branch the new worktree should be created from.
func AddTaskFromBranchAsync(o *orchestrator.Orchestrator, session *orchestrator.Session, task string, baseBranch string) tea.Cmd {
	return func() tea.Msg {
		if o == nil {
			return TaskAddedMsg{Instance: nil, Err: fmt.Errorf("orchestrator is nil")}
		}
		if session == nil {
			return TaskAddedMsg{Instance: nil, Err: fmt.Errorf("session is nil")}
		}
		inst, err := o.AddInstanceFromBranch(session, task, baseBranch)
		return TaskAddedMsg{Instance: inst, Err: err}
	}
}

// AddDependentTaskAsync returns a command that adds a task with dependencies asynchronously.
// The new task will depend on the specified instance and auto-start when it completes.
func AddDependentTaskAsync(o *orchestrator.Orchestrator, session *orchestrator.Session, task string, dependsOn string) tea.Cmd {
	return func() tea.Msg {
		if o == nil {
			return DependentTaskAddedMsg{Instance: nil, DependsOn: dependsOn, Err: fmt.Errorf("orchestrator is nil")}
		}
		if session == nil {
			return DependentTaskAddedMsg{Instance: nil, DependsOn: dependsOn, Err: fmt.Errorf("session is nil")}
		}
		inst, err := o.AddInstanceWithDependencies(session, task, []string{dependsOn}, true)
		return DependentTaskAddedMsg{Instance: inst, DependsOn: dependsOn, Err: err}
	}
}

// RemoveInstanceAsync returns a command that removes an instance asynchronously.
// This prevents the UI from blocking while git removes the worktree and branch.
func RemoveInstanceAsync(o *orchestrator.Orchestrator, session *orchestrator.Session, instanceID string) tea.Cmd {
	return func() tea.Msg {
		if o == nil {
			return InstanceRemovedMsg{InstanceID: instanceID, Err: fmt.Errorf("orchestrator is nil")}
		}
		if session == nil {
			return InstanceRemovedMsg{InstanceID: instanceID, Err: fmt.Errorf("session is nil")}
		}
		err := o.RemoveInstance(session, instanceID, true)
		return InstanceRemovedMsg{InstanceID: instanceID, Err: err}
	}
}

// LoadDiffAsync returns a command that loads a git diff asynchronously.
// This prevents the UI from blocking while git computes the diff.
func LoadDiffAsync(o *orchestrator.Orchestrator, worktreePath string, instanceID string) tea.Cmd {
	return func() tea.Msg {
		if o == nil {
			return DiffLoadedMsg{
				InstanceID:  instanceID,
				DiffContent: "",
				Err:         fmt.Errorf("orchestrator is nil"),
			}
		}
		diff, err := o.GetInstanceDiff(worktreePath)
		return DiffLoadedMsg{
			InstanceID:  instanceID,
			DiffContent: diff,
			Err:         err,
		}
	}
}

// CheckTripleShotCompletionAsync returns a command that checks tripleshot completion files
// in a goroutine, avoiding blocking the UI event loop with file I/O.
func CheckTripleShotCompletionAsync(
	coordinator *tripleshot.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		session := coordinator.Session()
		if session == nil {
			return nil
		}

		result := TripleShotCheckResultMsg{
			GroupID:        groupID,
			AttemptResults: make(map[int]bool),
			AttemptErrors:  make(map[int]error),
			Phase:          session.Phase,
		}

		switch session.Phase {
		case tripleshot.PhaseWorking:
			// Check each attempt's completion file
			for i := range 3 {
				attempt := session.Attempts[i]
				if attempt.Status == tripleshot.AttemptStatusWorking {
					complete, err := coordinator.CheckAttemptCompletion(i)
					result.AttemptResults[i] = complete
					if err != nil {
						result.AttemptErrors[i] = err
					}
				}
			}

		case tripleshot.PhaseEvaluating:
			// Check judge completion file
			complete, err := coordinator.CheckJudgeCompletion()
			result.JudgeComplete = complete
			result.JudgeError = err
		}

		return result
	}
}

// ProcessAttemptCompletionAsync returns a command that processes an attempt completion file
// in a goroutine, avoiding blocking the UI event loop with file I/O.
func ProcessAttemptCompletionAsync(
	coordinator *tripleshot.Coordinator,
	groupID string,
	attemptIndex int,
) tea.Cmd {
	return func() tea.Msg {
		err := coordinator.ProcessAttemptCompletion(attemptIndex)
		return TripleShotAttemptProcessedMsg{
			GroupID:      groupID,
			AttemptIndex: attemptIndex,
			Err:          err,
		}
	}
}

// ProcessJudgeCompletionAsync returns a command that processes a judge completion file
// in a goroutine, avoiding blocking the UI event loop with file I/O.
func ProcessJudgeCompletionAsync(
	coordinator *tripleshot.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		err := coordinator.ProcessJudgeCompletion()

		// Build task preview for success message
		taskPreview := ""
		if err == nil {
			session := coordinator.Session()
			if session != nil && len(session.Task) > 30 {
				taskPreview = session.Task[:27] + "..."
			} else if session != nil {
				taskPreview = session.Task
			}
		}

		return TripleShotJudgeProcessedMsg{
			GroupID:     groupID,
			Err:         err,
			TaskPreview: taskPreview,
		}
	}
}

// CheckPlanFileAsync returns a command that checks for a plan file asynchronously.
// This avoids blocking the UI event loop with file I/O during the planning phase.
func CheckPlanFileAsync(
	orc *orchestrator.Orchestrator,
	ultraPlan *view.UltraPlanState,
) tea.Cmd {
	return func() tea.Msg {
		if ultraPlan == nil || ultraPlan.Coordinator == nil {
			return nil
		}

		session := ultraPlan.Coordinator.Session()
		if session == nil || session.Phase != orchestrator.PhasePlanning || session.Plan != nil {
			return nil
		}

		// Get the coordinator instance
		inst := orc.GetInstance(session.CoordinatorID)
		if inst == nil {
			return nil
		}

		// Check if plan file exists
		planPath := orchestrator.PlanFilePath(inst.WorktreePath)
		if _, err := os.Stat(planPath); err != nil {
			return nil
		}

		// Parse the plan
		plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
		if err != nil {
			// Don't return error yet - file might be partially written
			return nil
		}

		return PlanFileCheckResultMsg{
			Found:        true,
			Plan:         plan,
			InstanceID:   inst.ID,
			WorktreePath: inst.WorktreePath,
		}
	}
}

// CheckMultiPassPlanFilesAsync returns commands that check for plan files from multi-pass coordinators.
// Each returned command checks one coordinator's plan file asynchronously.
func CheckMultiPassPlanFilesAsync(
	orc *orchestrator.Orchestrator,
	ultraPlan *view.UltraPlanState,
) []tea.Cmd {
	if ultraPlan == nil || ultraPlan.Coordinator == nil {
		return nil
	}

	session := ultraPlan.Coordinator.Session()
	if session == nil {
		return nil
	}

	// Only check during planning phase in multi-pass mode
	if session.Phase != orchestrator.PhasePlanning || !session.Config.MultiPass {
		return nil
	}

	// Skip if we don't have coordinator IDs yet
	numCoordinators := len(session.PlanCoordinatorIDs)
	if numCoordinators == 0 {
		return nil
	}

	// Skip if plan manager is already running
	if session.PlanManagerID != "" {
		return nil
	}

	strategyNames := orchestrator.GetMultiPassStrategyNames()
	var cmds []tea.Cmd

	// Create async check command for each coordinator that doesn't have a plan yet
	for i, coordID := range session.PlanCoordinatorIDs {
		// Skip if we already have a plan for this coordinator
		if i < len(session.CandidatePlans) && session.CandidatePlans[i] != nil {
			continue
		}

		// Capture loop variables for closure
		idx := i
		instID := coordID
		strategyName := "unknown"
		if idx < len(strategyNames) {
			strategyName = strategyNames[idx]
		}

		cmds = append(cmds, func() tea.Msg {
			// Get the coordinator instance
			inst := orc.GetInstance(instID)
			if inst == nil {
				return nil
			}

			// Check if plan file exists
			planPath := orchestrator.PlanFilePath(inst.WorktreePath)
			if _, err := os.Stat(planPath); err != nil {
				return nil
			}

			// Parse the plan
			plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
			if err != nil {
				// File might be partially written, skip for now
				return nil
			}

			return MultiPassPlanFileCheckResultMsg{
				Index:        idx,
				Plan:         plan,
				StrategyName: strategyName,
			}
		})
	}

	return cmds
}

// CheckPlanManagerFileAsync returns a command that checks for the plan manager's output file.
// This avoids blocking the UI during plan selection phase in multi-pass mode.
func CheckPlanManagerFileAsync(
	orc *orchestrator.Orchestrator,
	outputManager *output.Manager,
	ultraPlan *view.UltraPlanState,
) tea.Cmd {
	return func() tea.Msg {
		if ultraPlan == nil || ultraPlan.Coordinator == nil {
			return nil
		}

		session := ultraPlan.Coordinator.Session()
		if session == nil {
			return nil
		}

		// Only check during plan selection phase in multi-pass mode
		if session.Phase != orchestrator.PhasePlanSelection || !session.Config.MultiPass {
			return nil
		}

		// Need a plan manager ID to check
		if session.PlanManagerID == "" {
			return nil
		}

		// Skip if we already have a plan set
		if session.Plan != nil {
			return nil
		}

		// Get the plan manager instance
		inst := orc.GetInstance(session.PlanManagerID)
		if inst == nil {
			return nil
		}

		// Check if plan file exists
		planPath := orchestrator.PlanFilePath(inst.WorktreePath)
		if _, err := os.Stat(planPath); err != nil {
			return nil
		}

		// Parse the plan from the file
		plan, err := orchestrator.ParsePlanFromFile(planPath, session.Objective)
		if err != nil {
			return PlanManagerFileCheckResultMsg{
				Found: true,
				Err:   err,
			}
		}

		// Try to parse the plan decision from the output (for display purposes)
		var decision *orchestrator.PlanDecision
		if outputManager != nil {
			output := outputManager.GetOutput(inst.ID)
			decision, _ = orchestrator.ParsePlanDecisionFromOutput(output)
		}

		return PlanManagerFileCheckResultMsg{
			Found:    true,
			Plan:     plan,
			Decision: decision,
		}
	}
}

// CheckAdversarialCompletionAsync returns a command that checks adversarial completion files
// in a goroutine, avoiding blocking the UI event loop with file I/O.
func CheckAdversarialCompletionAsync(
	coordinator *adversarial.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		session := coordinator.Session()
		if session == nil {
			// Return an error result instead of nil to avoid silent failure
			return AdversarialCheckResultMsg{
				GroupID:        groupID,
				Phase:          adversarial.PhaseFailed,
				IncrementError: fmt.Errorf("adversarial session not found"),
			}
		}

		result := AdversarialCheckResultMsg{
			GroupID: groupID,
			Phase:   session.Phase,
		}

		switch session.Phase {
		case adversarial.PhaseImplementing:
			// Check increment file
			ready, err := coordinator.CheckIncrementReady()
			result.IncrementReady = ready
			result.IncrementError = err

		case adversarial.PhaseReviewing:
			// Check review file
			ready, err := coordinator.CheckReviewReady()
			result.ReviewReady = ready
			result.ReviewError = err

		case adversarial.PhaseApproved, adversarial.PhaseComplete:
			// Also check review file in approved/complete phases to allow users
			// to reject an approved result by having the reviewer write a new
			// failing review file
			ready, err := coordinator.CheckReviewReady()
			result.ReviewReady = ready
			result.ReviewError = err
		}

		return result
	}
}

// ProcessAdversarialIncrementAsync returns a command that processes an increment file
// in a goroutine, avoiding blocking the UI event loop with file I/O.
func ProcessAdversarialIncrementAsync(
	coordinator *adversarial.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		err := coordinator.ProcessIncrementCompletion()
		return AdversarialIncrementProcessedMsg{
			GroupID: groupID,
			Err:     err,
		}
	}
}

// ProcessAdversarialReviewAsync returns a command that processes a review file
// in a goroutine, avoiding blocking the UI event loop with file I/O.
func ProcessAdversarialReviewAsync(
	coordinator *adversarial.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		err := coordinator.ProcessReviewCompletion()

		// Get review result for feedback
		approved := false
		score := 0
		if err == nil {
			session := coordinator.Session()
			if session != nil && len(session.History) > 0 {
				lastRound := session.History[len(session.History)-1]
				if lastRound.Review != nil {
					approved = lastRound.Review.Approved
					score = lastRound.Review.Score
				}
			}
		}

		return AdversarialReviewProcessedMsg{
			GroupID:  groupID,
			Approved: approved,
			Score:    score,
			Err:      err,
		}
	}
}

// ProcessAdversarialRejectionAfterApprovalAsync returns a command that processes a rejection
// that occurred after an initial approval. This allows users to reject an approved result
// by having the reviewer write a new failing review file.
func ProcessAdversarialRejectionAfterApprovalAsync(
	coordinator *adversarial.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		err := coordinator.ProcessRejectionAfterApproval()

		// Get review result for feedback
		score := 0
		if err == nil {
			session := coordinator.Session()
			if session != nil && len(session.History) > 0 {
				lastRound := session.History[len(session.History)-1]
				if lastRound.Review != nil {
					score = lastRound.Review.Score
				}
			}
		}

		return AdversarialRejectionAfterApprovalMsg{
			GroupID: groupID,
			Score:   score,
			Err:     err,
		}
	}
}

// ProcessRalphCompletionAsync returns a command that processes a ralph iteration completion
// in a goroutine, avoiding blocking the UI event loop.
func ProcessRalphCompletionAsync(
	coordinator *ralph.Coordinator,
	groupID string,
	instanceID string,
	outputManager *output.Manager,
) tea.Cmd {
	return func() tea.Msg {
		session := coordinator.Session()
		if session == nil {
			return RalphCompletionProcessedMsg{
				GroupID: groupID,
				Err:     fmt.Errorf("ralph session not found"),
			}
		}

		// Get the instance output for completion promise checking
		var instanceOutput string
		if outputManager != nil {
			instanceOutput = outputManager.GetOutput(instanceID)
		}

		// Process the iteration completion
		continueLoop, err := coordinator.ProcessIterationCompletion(instanceOutput)

		return RalphCompletionProcessedMsg{
			GroupID:      groupID,
			Iteration:    session.CurrentIteration,
			ContinueLoop: continueLoop,
			Err:          err,
		}
	}
}

// CheckAdversarialInstanceStuckAsync checks if an adversarial instance has completed
// without writing its required file (stuck condition). This should be called when
// an instance state changes to completed or waiting-for-input.
func CheckAdversarialInstanceStuckAsync(
	coordinator *adversarial.Coordinator,
	groupID string,
	instanceID string,
	isCompleted bool,
	isWaitingInput bool,
) tea.Cmd {
	return func() tea.Msg {
		wasStuck := coordinator.HandleInstanceCompletion(instanceID, isCompleted, isWaitingInput)
		if wasStuck {
			return AdversarialStuckMsg{
				GroupID:    groupID,
				InstanceID: instanceID,
				StuckRole:  coordinator.GetStuckRole(),
			}
		}
		return nil
	}
}

// RestartAdversarialStuckRoleAsync restarts the stuck role in an adversarial session.
func RestartAdversarialStuckRoleAsync(
	coordinator *adversarial.Coordinator,
	groupID string,
) tea.Cmd {
	return func() tea.Msg {
		err := coordinator.RestartStuckRole()
		return AdversarialRestartMsg{
			GroupID: groupID,
			Err:     err,
		}
	}
}

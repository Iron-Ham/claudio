package update

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/logging"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/tui/msg"
	"github.com/Iron-Ham/claudio/internal/tui/output"
	"github.com/spf13/viper"
)

// Context provides the interface for update handlers to interact with the TUI Model.
// This allows handlers to modify model state without direct coupling to the Model type.
type Context interface {
	// Session returns the current orchestrator session.
	Session() *orchestrator.Session

	// Orchestrator returns the orchestrator instance.
	Orchestrator() *orchestrator.Orchestrator

	// OutputManager returns the output manager for instance output handling.
	OutputManager() *output.Manager

	// Logger returns the logger instance (may be nil).
	Logger() *logging.Logger

	// InstanceCount returns the number of instances.
	InstanceCount() int

	// ActiveInstance returns the currently active instance (may be nil).
	ActiveInstance() *orchestrator.Instance

	// SetErrorMessage sets an error message to display.
	SetErrorMessage(msg string)

	// SetInfoMessage sets an info message to display.
	SetInfoMessage(msg string)

	// ClearInfoMessage clears the info message.
	ClearInfoMessage()

	// SetActiveTab sets the active tab index.
	SetActiveTab(idx int)

	// PauseInstance pauses the output capture for an instance.
	PauseInstance(instanceID string)

	// EnsureActiveVisible ensures the active tab is visible in the sidebar.
	EnsureActiveVisible()
}

// HandleOutput processes an OutputMsg, adding output data to the manager.
func HandleOutput(ctx Context, m msg.OutputMsg) {
	ctx.OutputManager().AddOutput(m.InstanceID, string(m.Data))
}

// HandleError processes an ErrMsg, setting the error message.
func HandleError(ctx Context, m msg.ErrMsg) {
	ctx.SetErrorMessage(m.Err.Error())
}

// HandlePRComplete processes a PRCompleteMsg when a PR workflow completes.
// It removes the instance and sets an appropriate info or error message.
func HandlePRComplete(ctx Context, m msg.PRCompleteMsg) {
	session := ctx.Session()
	if session == nil {
		return
	}

	inst := session.GetInstance(m.InstanceID)
	if inst == nil {
		return
	}

	orch := ctx.Orchestrator()
	if orch == nil {
		return
	}

	if err := orch.RemoveInstance(session, m.InstanceID, true); err != nil {
		ctx.SetErrorMessage(fmt.Sprintf("Failed to remove instance after PR: %v", err))
		return
	}

	if m.Success {
		ctx.SetInfoMessage(fmt.Sprintf("PR created and instance %s removed", m.InstanceID))
	} else {
		ctx.SetInfoMessage(fmt.Sprintf("PR workflow finished (may have failed) - instance %s removed", m.InstanceID))
	}
}

// HandlePROpened processes a PROpenedMsg when a PR URL is detected.
// It notifies the user but keeps the instance for potential review tools.
func HandlePROpened(ctx Context, m msg.PROpenedMsg) {
	session := ctx.Session()
	if session == nil {
		return
	}

	inst := session.GetInstance(m.InstanceID)
	if inst == nil {
		return
	}

	ctx.SetInfoMessage(fmt.Sprintf("PR opened for instance %s - use :D to remove or run review tools", inst.ID))
}

// HandleTimeout processes a TimeoutMsg when an instance times out.
// It notifies the user with specific timeout type information.
func HandleTimeout(ctx Context, m msg.TimeoutMsg) {
	session := ctx.Session()
	if session == nil {
		return
	}

	inst := session.GetInstance(m.InstanceID)
	if inst == nil {
		return
	}

	var statusText string
	switch m.TimeoutType {
	case instance.TimeoutActivity:
		statusText = "stuck (no activity)"
	case instance.TimeoutCompletion:
		statusText = "timed out (max runtime exceeded)"
	case instance.TimeoutStale:
		statusText = "stuck (repeated output)"
	}

	ctx.SetInfoMessage(fmt.Sprintf("Instance %s is %s - use Ctrl+R to restart or Ctrl+K to kill", inst.ID, statusText))
}

// HandleTaskAdded processes a TaskAddedMsg when async task addition completes.
// It clears pending messages, switches to the new task, and logs the event.
// If session.auto_start_on_add is enabled (default), the instance is started automatically.
func HandleTaskAdded(ctx Context, m msg.TaskAddedMsg) {
	ctx.ClearInfoMessage()

	if m.Err != nil {
		ctx.SetErrorMessage(m.Err.Error())
		if logger := ctx.Logger(); logger != nil {
			logger.Error("failed to add task", "error", m.Err.Error())
		}
		return
	}

	// Pause the old active instance before switching (new instance starts unpaused)
	if oldInst := ctx.ActiveInstance(); oldInst != nil {
		ctx.PauseInstance(oldInst.ID)
	}

	// Switch to the newly added task and ensure it's visible in sidebar
	ctx.SetActiveTab(ctx.InstanceCount() - 1)
	ctx.EnsureActiveVisible()

	// Auto-start the instance if configured (default: true)
	if viper.GetBool("session.auto_start_on_add") && m.Instance != nil {
		orch := ctx.Orchestrator()
		if orch != nil {
			if err := orch.StartInstance(m.Instance); err != nil {
				ctx.SetErrorMessage(fmt.Sprintf("Failed to auto-start instance: %v", err))
				if logger := ctx.Logger(); logger != nil {
					logger.Error("failed to auto-start instance", "error", err.Error())
				}
			} else {
				ctx.SetInfoMessage(fmt.Sprintf("Started instance %s", m.Instance.ID))
				if logger := ctx.Logger(); logger != nil {
					logger.Info("auto-started instance", "task", m.Instance.Task)
				}
			}
		}
	}

	// Log user adding instance
	if logger := ctx.Logger(); logger != nil && m.Instance != nil {
		logger.Info("user added instance", "task", m.Instance.Task)
	}
}

// HandleDependentTaskAdded processes a DependentTaskAddedMsg when async dependent task addition completes.
// It clears pending messages, switches to the new task, and displays the dependency info.
func HandleDependentTaskAdded(ctx Context, m msg.DependentTaskAddedMsg) {
	ctx.ClearInfoMessage()

	if m.Err != nil {
		ctx.SetErrorMessage(m.Err.Error())
		if logger := ctx.Logger(); logger != nil {
			logger.Error("failed to add dependent task",
				"depends_on", m.DependsOn,
				"error", m.Err.Error(),
			)
		}
		return
	}

	// Pause the old active instance before switching (new instance starts unpaused)
	if oldInst := ctx.ActiveInstance(); oldInst != nil {
		ctx.PauseInstance(oldInst.ID)
	}

	// Switch to the newly added task and ensure it's visible in sidebar
	ctx.SetActiveTab(ctx.InstanceCount() - 1)
	ctx.EnsureActiveVisible()

	// Find the parent instance name for a better message
	parentTask := m.DependsOn
	session := ctx.Session()
	if session != nil {
		for _, inst := range session.Instances {
			if inst.ID == m.DependsOn {
				parentTask = inst.Task
				if len(parentTask) > 50 {
					parentTask = parentTask[:50] + "..."
				}
				break
			}
		}
	}

	ctx.SetInfoMessage(fmt.Sprintf("Chained task added. Will auto-start when \"%s\" completes.", parentTask))

	// Log user adding dependent instance
	if logger := ctx.Logger(); logger != nil && m.Instance != nil {
		logger.Info("user added dependent instance",
			"task", m.Instance.Task,
			"depends_on", m.DependsOn,
		)
	}
}

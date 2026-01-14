package msg

import (
	"errors"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// TestTickMsg verifies TickMsg type alias behavior
func TestTickMsg(t *testing.T) {
	now := time.Now()
	tick := TickMsg(now)

	// Verify it stores the time correctly
	if time.Time(tick) != now {
		t.Errorf("TickMsg(%v) = %v, want %v", now, time.Time(tick), now)
	}
}

func TestOutputMsg(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		data       []byte
	}{
		{
			name:       "basic output",
			instanceID: "instance-1",
			data:       []byte("hello world"),
		},
		{
			name:       "empty data",
			instanceID: "instance-2",
			data:       []byte{},
		},
		{
			name:       "nil data",
			instanceID: "instance-3",
			data:       nil,
		},
		{
			name:       "binary data",
			instanceID: "instance-4",
			data:       []byte{0x00, 0x01, 0xFF, 0xFE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := OutputMsg{
				InstanceID: tt.instanceID,
				Data:       tt.data,
			}

			if msg.InstanceID != tt.instanceID {
				t.Errorf("OutputMsg.InstanceID = %q, want %q", msg.InstanceID, tt.instanceID)
			}
			if string(msg.Data) != string(tt.data) {
				t.Errorf("OutputMsg.Data = %v, want %v", msg.Data, tt.data)
			}
		})
	}
}

func TestErrMsg(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "basic error",
			err:  errors.New("something went wrong"),
		},
		{
			name: "nil error",
			err:  nil,
		},
		{
			name: "wrapped error",
			err:  errors.New("outer: inner error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := ErrMsg{Err: tt.err}

			if msg.Err != tt.err {
				t.Errorf("ErrMsg.Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestPRCompleteMsg(t *testing.T) {
	tests := []struct {
		name       string
		instanceID string
		success    bool
	}{
		{
			name:       "successful PR",
			instanceID: "pr-1",
			success:    true,
		},
		{
			name:       "failed PR",
			instanceID: "pr-2",
			success:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := PRCompleteMsg{
				InstanceID: tt.instanceID,
				Success:    tt.success,
			}

			if msg.InstanceID != tt.instanceID {
				t.Errorf("PRCompleteMsg.InstanceID = %q, want %q", msg.InstanceID, tt.instanceID)
			}
			if msg.Success != tt.success {
				t.Errorf("PRCompleteMsg.Success = %v, want %v", msg.Success, tt.success)
			}
		})
	}
}

func TestPROpenedMsg(t *testing.T) {
	msg := PROpenedMsg{InstanceID: "test-instance"}

	if msg.InstanceID != "test-instance" {
		t.Errorf("PROpenedMsg.InstanceID = %q, want %q", msg.InstanceID, "test-instance")
	}
}

func TestTimeoutMsg(t *testing.T) {
	tests := []struct {
		name        string
		instanceID  string
		timeoutType instance.TimeoutType
	}{
		{
			name:        "activity timeout",
			instanceID:  "timeout-1",
			timeoutType: instance.TimeoutActivity,
		},
		{
			name:        "completion timeout",
			instanceID:  "timeout-2",
			timeoutType: instance.TimeoutCompletion,
		},
		{
			name:        "stale timeout",
			instanceID:  "timeout-3",
			timeoutType: instance.TimeoutStale,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TimeoutMsg{
				InstanceID:  tt.instanceID,
				TimeoutType: tt.timeoutType,
			}

			if msg.InstanceID != tt.instanceID {
				t.Errorf("TimeoutMsg.InstanceID = %q, want %q", msg.InstanceID, tt.instanceID)
			}
			if msg.TimeoutType != tt.timeoutType {
				t.Errorf("TimeoutMsg.TimeoutType = %v, want %v", msg.TimeoutType, tt.timeoutType)
			}
		})
	}
}

func TestBellMsg(t *testing.T) {
	msg := BellMsg{InstanceID: "bell-instance"}

	if msg.InstanceID != "bell-instance" {
		t.Errorf("BellMsg.InstanceID = %q, want %q", msg.InstanceID, "bell-instance")
	}
}

func TestTaskAddedMsg(t *testing.T) {
	tests := []struct {
		name     string
		instance *orchestrator.Instance
		err      error
	}{
		{
			name:     "successful task",
			instance: &orchestrator.Instance{ID: "new-task", Task: "Test task"},
			err:      nil,
		},
		{
			name:     "failed task",
			instance: nil,
			err:      errors.New("failed to create worktree"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TaskAddedMsg{
				Instance: tt.instance,
				Err:      tt.err,
			}

			if msg.Instance != tt.instance {
				t.Errorf("TaskAddedMsg.Instance = %v, want %v", msg.Instance, tt.instance)
			}
			if msg.Err != tt.err {
				t.Errorf("TaskAddedMsg.Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestDependentTaskAddedMsg(t *testing.T) {
	tests := []struct {
		name      string
		instance  *orchestrator.Instance
		dependsOn string
		err       error
	}{
		{
			name:      "successful dependent task",
			instance:  &orchestrator.Instance{ID: "dep-task"},
			dependsOn: "parent-task",
			err:       nil,
		},
		{
			name:      "failed dependent task",
			instance:  nil,
			dependsOn: "parent-task",
			err:       errors.New("dependency error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := DependentTaskAddedMsg{
				Instance:  tt.instance,
				DependsOn: tt.dependsOn,
				Err:       tt.err,
			}

			if msg.Instance != tt.instance {
				t.Errorf("DependentTaskAddedMsg.Instance = %v, want %v", msg.Instance, tt.instance)
			}
			if msg.DependsOn != tt.dependsOn {
				t.Errorf("DependentTaskAddedMsg.DependsOn = %q, want %q", msg.DependsOn, tt.dependsOn)
			}
			if msg.Err != tt.err {
				t.Errorf("DependentTaskAddedMsg.Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestUltraPlanInitMsg(t *testing.T) {
	// This is a zero-value struct used as a signal
	msg := UltraPlanInitMsg{}
	_ = msg // Just verify it can be constructed
}

func TestTripleShotStartedMsg(t *testing.T) {
	msg := TripleShotStartedMsg{}
	_ = msg // Just verify it can be constructed
}

func TestTripleShotJudgeStartedMsg(t *testing.T) {
	msg := TripleShotJudgeStartedMsg{}
	_ = msg // Just verify it can be constructed
}

func TestTripleShotErrorMsg(t *testing.T) {
	err := errors.New("tripleshot failed")
	msg := TripleShotErrorMsg{Err: err}

	if msg.Err != err {
		t.Errorf("TripleShotErrorMsg.Err = %v, want %v", msg.Err, err)
	}
}

func TestTripleShotCheckResultMsg(t *testing.T) {
	tests := []struct {
		name           string
		groupID        string
		attemptResults map[int]bool
		attemptErrors  map[int]error
		judgeComplete  bool
		judgeError     error
		phase          orchestrator.TripleShotPhase
	}{
		{
			name:           "working phase",
			groupID:        "group-1",
			attemptResults: map[int]bool{0: true, 1: false, 2: false},
			attemptErrors:  map[int]error{},
			judgeComplete:  false,
			judgeError:     nil,
			phase:          orchestrator.PhaseTripleShotWorking,
		},
		{
			name:           "evaluating phase",
			groupID:        "group-2",
			attemptResults: nil,
			attemptErrors:  nil,
			judgeComplete:  true,
			judgeError:     nil,
			phase:          orchestrator.PhaseTripleShotEvaluating,
		},
		{
			name:           "with errors",
			groupID:        "group-3",
			attemptResults: map[int]bool{0: false},
			attemptErrors:  map[int]error{0: errors.New("check failed")},
			judgeComplete:  false,
			judgeError:     errors.New("judge error"),
			phase:          orchestrator.PhaseTripleShotWorking,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TripleShotCheckResultMsg{
				GroupID:        tt.groupID,
				AttemptResults: tt.attemptResults,
				AttemptErrors:  tt.attemptErrors,
				JudgeComplete:  tt.judgeComplete,
				JudgeError:     tt.judgeError,
				Phase:          tt.phase,
			}

			if msg.GroupID != tt.groupID {
				t.Errorf("GroupID = %q, want %q", msg.GroupID, tt.groupID)
			}
			if msg.JudgeComplete != tt.judgeComplete {
				t.Errorf("JudgeComplete = %v, want %v", msg.JudgeComplete, tt.judgeComplete)
			}
			if msg.Phase != tt.phase {
				t.Errorf("Phase = %v, want %v", msg.Phase, tt.phase)
			}
		})
	}
}

func TestTripleShotAttemptProcessedMsg(t *testing.T) {
	tests := []struct {
		name         string
		groupID      string
		attemptIndex int
		err          error
	}{
		{
			name:         "successful attempt 0",
			groupID:      "group-1",
			attemptIndex: 0,
			err:          nil,
		},
		{
			name:         "successful attempt 2",
			groupID:      "group-2",
			attemptIndex: 2,
			err:          nil,
		},
		{
			name:         "failed attempt",
			groupID:      "group-3",
			attemptIndex: 1,
			err:          errors.New("processing failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TripleShotAttemptProcessedMsg{
				GroupID:      tt.groupID,
				AttemptIndex: tt.attemptIndex,
				Err:          tt.err,
			}

			if msg.GroupID != tt.groupID {
				t.Errorf("GroupID = %q, want %q", msg.GroupID, tt.groupID)
			}
			if msg.AttemptIndex != tt.attemptIndex {
				t.Errorf("AttemptIndex = %d, want %d", msg.AttemptIndex, tt.attemptIndex)
			}
			if msg.Err != tt.err {
				t.Errorf("Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestTripleShotJudgeProcessedMsg(t *testing.T) {
	tests := []struct {
		name        string
		groupID     string
		err         error
		taskPreview string
	}{
		{
			name:        "successful judge",
			groupID:     "group-1",
			err:         nil,
			taskPreview: "Implement feature X",
		},
		{
			name:        "truncated task preview",
			groupID:     "group-2",
			err:         nil,
			taskPreview: "Very long task descripti...",
		},
		{
			name:        "failed judge",
			groupID:     "group-3",
			err:         errors.New("judge failed"),
			taskPreview: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := TripleShotJudgeProcessedMsg{
				GroupID:     tt.groupID,
				Err:         tt.err,
				TaskPreview: tt.taskPreview,
			}

			if msg.GroupID != tt.groupID {
				t.Errorf("GroupID = %q, want %q", msg.GroupID, tt.groupID)
			}
			if msg.Err != tt.err {
				t.Errorf("Err = %v, want %v", msg.Err, tt.err)
			}
			if msg.TaskPreview != tt.taskPreview {
				t.Errorf("TaskPreview = %q, want %q", msg.TaskPreview, tt.taskPreview)
			}
		})
	}
}

func TestPlanFileCheckResultMsg(t *testing.T) {
	tests := []struct {
		name         string
		found        bool
		plan         *orchestrator.PlanSpec
		instanceID   string
		worktreePath string
		err          error
	}{
		{
			name:         "plan found",
			found:        true,
			plan:         &orchestrator.PlanSpec{Objective: "Test objective"},
			instanceID:   "planner-1",
			worktreePath: "/path/to/worktree",
			err:          nil,
		},
		{
			name:         "plan not found",
			found:        false,
			plan:         nil,
			instanceID:   "",
			worktreePath: "",
			err:          nil,
		},
		{
			name:         "plan parse error",
			found:        true,
			plan:         nil,
			instanceID:   "planner-2",
			worktreePath: "/path/to/worktree",
			err:          errors.New("invalid plan format"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := PlanFileCheckResultMsg{
				Found:        tt.found,
				Plan:         tt.plan,
				InstanceID:   tt.instanceID,
				WorktreePath: tt.worktreePath,
				Err:          tt.err,
			}

			if msg.Found != tt.found {
				t.Errorf("Found = %v, want %v", msg.Found, tt.found)
			}
			if msg.Plan != tt.plan {
				t.Errorf("Plan = %v, want %v", msg.Plan, tt.plan)
			}
			if msg.InstanceID != tt.instanceID {
				t.Errorf("InstanceID = %q, want %q", msg.InstanceID, tt.instanceID)
			}
			if msg.WorktreePath != tt.worktreePath {
				t.Errorf("WorktreePath = %q, want %q", msg.WorktreePath, tt.worktreePath)
			}
			if msg.Err != tt.err {
				t.Errorf("Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestMultiPassPlanFileCheckResultMsg(t *testing.T) {
	tests := []struct {
		name         string
		index        int
		plan         *orchestrator.PlanSpec
		strategyName string
		err          error
	}{
		{
			name:         "first strategy",
			index:        0,
			plan:         &orchestrator.PlanSpec{Objective: "Strategy 1"},
			strategyName: "conservative",
			err:          nil,
		},
		{
			name:         "second strategy",
			index:        1,
			plan:         &orchestrator.PlanSpec{Objective: "Strategy 2"},
			strategyName: "aggressive",
			err:          nil,
		},
		{
			name:         "failed strategy",
			index:        2,
			plan:         nil,
			strategyName: "experimental",
			err:          errors.New("strategy failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := MultiPassPlanFileCheckResultMsg{
				Index:        tt.index,
				Plan:         tt.plan,
				StrategyName: tt.strategyName,
				Err:          tt.err,
			}

			if msg.Index != tt.index {
				t.Errorf("Index = %d, want %d", msg.Index, tt.index)
			}
			if msg.Plan != tt.plan {
				t.Errorf("Plan = %v, want %v", msg.Plan, tt.plan)
			}
			if msg.StrategyName != tt.strategyName {
				t.Errorf("StrategyName = %q, want %q", msg.StrategyName, tt.strategyName)
			}
			if msg.Err != tt.err {
				t.Errorf("Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestPlanManagerFileCheckResultMsg(t *testing.T) {
	tests := []struct {
		name     string
		found    bool
		plan     *orchestrator.PlanSpec
		decision *orchestrator.PlanDecision
		err      error
	}{
		{
			name:  "plan selected",
			found: true,
			plan:  &orchestrator.PlanSpec{Objective: "Selected plan"},
			decision: &orchestrator.PlanDecision{
				Action:        "select",
				SelectedIndex: 0,
				Reasoning:     "Best approach",
			},
			err: nil,
		},
		{
			name:     "not found",
			found:    false,
			plan:     nil,
			decision: nil,
			err:      nil,
		},
		{
			name:     "parse error",
			found:    true,
			plan:     nil,
			decision: nil,
			err:      errors.New("invalid decision format"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := PlanManagerFileCheckResultMsg{
				Found:    tt.found,
				Plan:     tt.plan,
				Decision: tt.decision,
				Err:      tt.err,
			}

			if msg.Found != tt.found {
				t.Errorf("Found = %v, want %v", msg.Found, tt.found)
			}
			if msg.Plan != tt.plan {
				t.Errorf("Plan = %v, want %v", msg.Plan, tt.plan)
			}
			if msg.Decision != tt.decision {
				t.Errorf("Decision = %v, want %v", msg.Decision, tt.decision)
			}
			if msg.Err != tt.err {
				t.Errorf("Err = %v, want %v", msg.Err, tt.err)
			}
		})
	}
}

func TestInlineMultiPlanFileCheckResultMsg(t *testing.T) {
	tests := []struct {
		name         string
		index        int
		plan         *orchestrator.PlanSpec
		strategyName string
	}{
		{
			name:         "first inline plan",
			index:        0,
			plan:         &orchestrator.PlanSpec{Objective: "Plan A"},
			strategyName: "default",
		},
		{
			name:         "second inline plan",
			index:        1,
			plan:         &orchestrator.PlanSpec{Objective: "Plan B"},
			strategyName: "alternative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := InlineMultiPlanFileCheckResultMsg{
				Index:        tt.index,
				Plan:         tt.plan,
				StrategyName: tt.strategyName,
			}

			if msg.Index != tt.index {
				t.Errorf("Index = %d, want %d", msg.Index, tt.index)
			}
			if msg.Plan != tt.plan {
				t.Errorf("Plan = %v, want %v", msg.Plan, tt.plan)
			}
			if msg.StrategyName != tt.strategyName {
				t.Errorf("StrategyName = %q, want %q", msg.StrategyName, tt.strategyName)
			}
		})
	}
}

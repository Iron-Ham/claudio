package msg

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
)

// TickMsg is sent periodically to drive UI updates and polling.
type TickMsg time.Time

// OutputMsg contains output data from a Claude instance.
type OutputMsg struct {
	InstanceID string
	Data       []byte
}

// ErrMsg wraps an error to be displayed in the UI.
type ErrMsg struct {
	Err error
}

// PRCompleteMsg signals that a PR creation process has completed.
type PRCompleteMsg struct {
	InstanceID string
	Success    bool
}

// PROpenedMsg signals that a PR has been opened.
type PROpenedMsg struct {
	InstanceID string
}

// TimeoutMsg signals that an instance has timed out.
type TimeoutMsg struct {
	InstanceID  string
	TimeoutType instance.TimeoutType
}

// BellMsg signals that a bell should be rung for an instance.
type BellMsg struct {
	InstanceID string
}

// TaskAddedMsg is sent when async task addition completes.
type TaskAddedMsg struct {
	Instance *orchestrator.Instance
	Err      error
}

// DependentTaskAddedMsg is sent when async dependent task addition completes.
type DependentTaskAddedMsg struct {
	Instance  *orchestrator.Instance
	DependsOn string // The instance ID this task depends on
	Err       error
}

// UltraPlanInitMsg signals that ultra-plan mode should initialize.
type UltraPlanInitMsg struct{}

// TripleShotStartedMsg indicates triple-shot attempts have started.
type TripleShotStartedMsg struct{}

// TripleShotJudgeStartedMsg indicates the judge has started evaluating.
type TripleShotJudgeStartedMsg struct{}

// TripleShotErrorMsg indicates an error during triple-shot operation.
type TripleShotErrorMsg struct {
	Err error
}

// TripleShotCheckResultMsg contains results from async completion file checks.
// This allows the tick handler to dispatch async I/O and receive results
// without blocking the UI event loop.
type TripleShotCheckResultMsg struct {
	// GroupID identifies which tripleshot coordinator this result is for
	GroupID string

	// AttemptResults maps attempt index to completion status (true = file exists)
	AttemptResults map[int]bool

	// AttemptErrors maps attempt index to any errors encountered during check
	AttemptErrors map[int]error

	// JudgeComplete indicates whether the judge completion file exists
	JudgeComplete bool

	// JudgeError is any error encountered checking the judge file
	JudgeError error

	// Phase is the current phase of the tripleshot session
	Phase orchestrator.TripleShotPhase
}

// TripleShotAttemptProcessedMsg contains the result of processing an attempt completion file.
// This is returned by processAttemptCompletionAsync after reading and parsing the file.
type TripleShotAttemptProcessedMsg struct {
	GroupID      string
	AttemptIndex int
	Err          error
}

// TripleShotJudgeProcessedMsg contains the result of processing a judge completion file.
// This is returned by processJudgeCompletionAsync after reading and parsing the file.
type TripleShotJudgeProcessedMsg struct {
	GroupID     string
	Err         error
	TaskPreview string
}

// PlanFileCheckResultMsg contains the result of async plan file checking.
// Used for single-pass mode during planning phase.
type PlanFileCheckResultMsg struct {
	Found        bool
	Plan         *orchestrator.PlanSpec
	InstanceID   string
	WorktreePath string
	Err          error
}

// MultiPassPlanFileCheckResultMsg contains the result of async multi-pass plan file checking.
// Returned for each coordinator that has a new plan file detected.
type MultiPassPlanFileCheckResultMsg struct {
	Index        int
	Plan         *orchestrator.PlanSpec
	StrategyName string
	Err          error
}

// PlanManagerFileCheckResultMsg contains the result of async plan manager file checking.
// Used during plan selection phase in multi-pass mode.
type PlanManagerFileCheckResultMsg struct {
	Found    bool
	Plan     *orchestrator.PlanSpec
	Decision *orchestrator.PlanDecision
	Err      error
}

// InlineMultiPlanFileCheckResultMsg contains the result of async inline multiplan file checking.
// Used by the :multiplan command to detect when planners create their plan files.
type InlineMultiPlanFileCheckResultMsg struct {
	Index        int
	Plan         *orchestrator.PlanSpec
	StrategyName string
	GroupID      string // Session group ID for matching to correct session
}

// AdversarialStartedMsg indicates adversarial session has started.
type AdversarialStartedMsg struct{}

// AdversarialErrorMsg indicates an error during adversarial operation.
type AdversarialErrorMsg struct {
	Err error
}

// AdversarialCheckResultMsg contains results from async completion file checks.
// This allows the tick handler to dispatch async I/O and receive results
// without blocking the UI event loop.
type AdversarialCheckResultMsg struct {
	// GroupID identifies which adversarial coordinator this result is for
	GroupID string

	// IncrementReady indicates whether the increment file exists
	IncrementReady bool

	// IncrementError is any error encountered checking the increment file
	IncrementError error

	// ReviewReady indicates whether the review file exists
	ReviewReady bool

	// ReviewError is any error encountered checking the review file
	ReviewError error

	// Phase is the current phase of the adversarial session
	Phase orchestrator.AdversarialPhase
}

// AdversarialIncrementProcessedMsg contains the result of processing an increment file.
// This is returned by processIncrementCompletionAsync after reading and parsing the file.
type AdversarialIncrementProcessedMsg struct {
	GroupID string
	Err     error
}

// AdversarialReviewProcessedMsg contains the result of processing a review file.
// This is returned by processReviewCompletionAsync after reading and parsing the file.
type AdversarialReviewProcessedMsg struct {
	GroupID  string
	Approved bool
	Score    int
	Err      error
}

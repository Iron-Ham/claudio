package msg

import (
	"time"

	"github.com/Iron-Ham/claudio/internal/instance"
	"github.com/Iron-Ham/claudio/internal/orchestrator"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/adversarial"
	"github.com/Iron-Ham/claudio/internal/orchestrator/workflows/tripleshot"
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

// InstanceStubCreatedMsg is sent when the fast first phase of async task
// addition completes. The instance is now visible in the UI with StatusPreparing,
// but the worktree is not yet created.
type InstanceStubCreatedMsg struct {
	Instance *orchestrator.Instance
	Err      error
}

// InstanceSetupCompleteMsg is sent when the slow second phase of async task
// addition completes. The worktree is now created and the instance is ready
// to be started (status changed to StatusPending).
type InstanceSetupCompleteMsg struct {
	InstanceID string
	Err        error
}

// BlankInstanceStubCreatedMsg is sent when a blank instance stub creation completes.
// Blank instances have no task/prompt - they start Claude in interactive mode.
type BlankInstanceStubCreatedMsg struct {
	Instance *orchestrator.Instance
	Err      error
}

// DependentTaskAddedMsg is sent when async dependent task addition completes.
type DependentTaskAddedMsg struct {
	Instance  *orchestrator.Instance
	DependsOn string // The instance ID this task depends on
	Err       error
}

// InstanceRemovedMsg is sent when async instance removal completes.
type InstanceRemovedMsg struct {
	InstanceID string
	Err        error
}

// DiffLoadedMsg is sent when async diff loading completes.
type DiffLoadedMsg struct {
	InstanceID  string
	DiffContent string
	Err         error
}

// UltraPlanInitMsg signals that ultra-plan mode should initialize.
type UltraPlanInitMsg struct{}

// TripleShotStartedMsg indicates triple-shot attempts have started.
type TripleShotStartedMsg struct{}

// TripleShotStubsCreatedMsg indicates that stub instances for all three attempts
// have been created (fast first phase). The UI can show these with "Preparing" status.
type TripleShotStubsCreatedMsg struct {
	GroupID     string
	InstanceIDs [3]string
	Err         error
}

// TripleShotAttemptSetupCompleteMsg indicates that a single attempt's worktree setup
// has completed (slow second phase). The attempt is now ready to run.
type TripleShotAttemptSetupCompleteMsg struct {
	GroupID      string
	AttemptIndex int
	Err          error
}

// TripleShotJudgeStartedMsg indicates the judge has started evaluating.
type TripleShotJudgeStartedMsg struct {
	// ImplementersGroupID is the ID of the Implementers sub-group, used by TUI
	// to auto-collapse the implementers when the judge starts running.
	ImplementersGroupID string
}

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

	// ReviewResults maps attempt index to review completion status (true = file exists)
	// Only populated during PhaseAdversarialReview
	ReviewResults map[int]bool

	// ReviewErrors maps attempt index to any errors encountered checking review files
	ReviewErrors map[int]error

	// JudgeComplete indicates whether the judge completion file exists
	JudgeComplete bool

	// JudgeError is any error encountered checking the judge file
	JudgeError error

	// Phase is the current phase of the tripleshot session
	Phase tripleshot.Phase
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

// TripleShotReviewProcessedMsg contains the result of processing an adversarial review file.
// This is returned by ProcessAdversarialReviewCompletionAsync after reading and parsing the file.
type TripleShotReviewProcessedMsg struct {
	GroupID      string
	AttemptIndex int
	Err          error
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
	Phase adversarial.Phase
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

// AdversarialRejectionAfterApprovalMsg contains the result of processing a rejection
// that occurred after an initial approval. This allows users to reject an approved
// result by having the reviewer write a new failing review file.
type AdversarialRejectionAfterApprovalMsg struct {
	GroupID string
	Score   int
	Err     error
}

// AdversarialStuckMsg indicates that an adversarial instance got stuck
// (completed without writing its required file).
type AdversarialStuckMsg struct {
	GroupID    string
	InstanceID string
	StuckRole  adversarial.StuckRole
}

// AdversarialRestartMsg is sent when a stuck adversarial role is restarted.
type AdversarialRestartMsg struct {
	GroupID string
	Err     error
}

// RalphIterationStartedMsg indicates a new ralph iteration has started.
type RalphIterationStartedMsg struct {
	GroupID   string
	Iteration int
}

// RalphErrorMsg indicates an error during ralph loop operation.
type RalphErrorMsg struct {
	Err     error
	GroupID string
}

// RalphCompletionProcessedMsg contains the result of processing a ralph iteration completion.
// This is returned after checking the instance completion and output.
type RalphCompletionProcessedMsg struct {
	GroupID      string
	Iteration    int
	ContinueLoop bool // True if another iteration should be started
	Err          error
}

// Package strategy provides consolidation strategies for combining task branches.
// It implements the Strategy pattern to support different consolidation modes
// such as stacked PRs and single PR.
package strategy

import (
	"context"
	"time"

	"github.com/Iron-Ham/claudio/internal/orchestrator/consolidation"
)

// Dependencies holds the dependencies needed by consolidation strategies.
type Dependencies struct {
	Branch    BranchOps
	Conflict  ConflictOps
	PRBuilder PRBuilderOps
	PRCreator PRCreatorOps
	Events    consolidation.EventEmitter
	Logger    consolidation.Logger
}

// BranchOps defines branch operations needed by strategies.
type BranchOps interface {
	FindMainBranch(ctx context.Context) string
	CreateGroupBranch(ctx context.Context, groupIdx int, baseBranch string) (string, error)
	CreateSingleBranch(ctx context.Context, baseBranch string) (string, error)
	CreateWorktree(ctx context.Context, path, branch string) error
	CherryPickBranch(ctx context.Context, worktreePath, sourceBranch string) error
	GetChangedFiles(ctx context.Context, worktreePath string) ([]string, error)
	CountCommitsBetween(ctx context.Context, worktreePath, baseBranch, headBranch string) (int, error)
	Push(ctx context.Context, worktreePath string, force bool) error
	RemoveWorktree(ctx context.Context, path string) error
}

// ConflictOps defines conflict detection operations.
type ConflictOps interface {
	CheckCherryPick(ctx context.Context, worktreePath, sourceBranch, taskID, taskTitle string) (*ConflictInfo, error)
	AbortCherryPick(ctx context.Context, worktreePath string) error
	GetConflictingFiles(ctx context.Context, worktreePath string) ([]string, error)
}

// ConflictInfo is an alias to consolidation.ConflictInfo for convenience.
type ConflictInfo = consolidation.ConflictInfo

// PRBuilderOps defines PR content generation operations.
type PRBuilderOps interface {
	Build(tasks []consolidation.CompletedTask, opts consolidation.PRBuildOptions) (*consolidation.PRContent, error)
}

// PRCreatorOps defines PR creation operations.
type PRCreatorOps interface {
	Create(ctx context.Context, content *consolidation.PRContent, draft bool, labels []string) (string, error)
}

// Config holds configuration for consolidation strategies.
type Config struct {
	Mode            consolidation.Mode
	BranchPrefix    string
	CreateDraftPRs  bool
	PRLabels        []string
	WorktreeDir     string // Base directory for creating worktrees
	Objective       string // The ultraplan objective
	SynthesisNotes  string
	Recommendations []string
}

// Result is an alias to consolidation.StrategyResult for convenience.
type Result = consolidation.StrategyResult

// GroupResult is an alias to consolidation.GroupResult for convenience.
type GroupResult = consolidation.GroupResult

// TaskGroup is an alias to consolidation.TaskGroup for convenience.
type TaskGroup = consolidation.TaskGroup

// Base provides common functionality for consolidation strategies.
type Base struct {
	deps   Dependencies
	config Config
}

// NewBase creates a new Base with the given dependencies and config.
func NewBase(deps Dependencies, config Config) *Base {
	return &Base{
		deps:   deps,
		config: config,
	}
}

// emit sends a consolidation event.
func (b *Base) emit(event consolidation.Event) {
	event.Timestamp = time.Now()
	if b.deps.Events != nil {
		b.deps.Events.Emit(event)
	}
}

// log provides convenient access to the logger.
func (b *Base) log() consolidation.Logger {
	if b.deps.Logger != nil {
		return b.deps.Logger
	}
	return &nopLogger{}
}

// nopLogger is a no-op logger implementation.
type nopLogger struct{}

func (n *nopLogger) Debug(_ string, _ ...any) {}
func (n *nopLogger) Info(_ string, _ ...any)  {}
func (n *nopLogger) Warn(_ string, _ ...any)  {}
func (n *nopLogger) Error(_ string, _ ...any) {}

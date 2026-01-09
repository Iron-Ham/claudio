package worktree

// GitOperations defines core git operations for repository manipulation.
// This interface abstracts git CLI operations, enabling mock implementations
// for testing without actual git repositories.
type GitOperations interface {
	// Commit operations
	CommitAll(path, message string) error
	HasUncommittedChanges(path string) (bool, error)
	GetCommitLog(path string) (string, error)
	GetCommitsBetween(path, baseBranch, headBranch string) ([]string, error)
	CountCommitsBetween(path, baseBranch, headBranch string) (int, error)
	HasCommitsBeyond(path, baseBranch string) (bool, error)

	// Push/Pull/Rebase operations
	Push(path string, force bool) error
	RebaseOnMain(path string) error
	HasRebaseConflicts(path string) (bool, error)
	GetBehindCount(path string) (int, error)

	// Cherry-pick operations
	CherryPickBranch(path, sourceBranch string) error
	CheckCherryPickConflicts(path, sourceBranch string) ([]string, error)
	AbortCherryPick(path string) error
	ContinueCherryPick(path string) error
	IsCherryPickInProgress(path string) bool
	GetConflictingFiles(path string) ([]string, error)
}

// WorktreeManager defines operations for managing git worktrees.
// Worktrees allow multiple working directories attached to a single repository,
// enabling parallel development on different branches.
type WorktreeManager interface {
	// Create creates a new worktree at the given path with a new branch from HEAD.
	Create(path, branch string) error

	// CreateFromBranch creates a new worktree at the given path with a new branch
	// based off a specific base branch.
	CreateFromBranch(path, newBranch, baseBranch string) error

	// CreateWorktreeFromBranch creates a worktree from an existing branch
	// (without creating a new branch).
	CreateWorktreeFromBranch(path, branch string) error

	// Remove removes a worktree at the given path.
	Remove(path string) error

	// List returns paths of all worktrees in the repository.
	List() ([]string, error)

	// GetPath returns the repository's root directory.
	// This is the directory from which worktrees are managed.
	GetPath() string
}

// BranchManager defines operations for managing git branches.
type BranchManager interface {
	// CreateBranchFrom creates a new branch from a specified base branch
	// (without creating a worktree).
	CreateBranchFrom(branchName, baseBranch string) error

	// DeleteBranch deletes a branch by name.
	DeleteBranch(branch string) error

	// GetBranch returns the current branch name for a given worktree path.
	GetBranch(path string) (string, error)

	// FindMainBranch returns the name of the main branch (main or master).
	FindMainBranch() string
}

// DiffProvider defines operations for retrieving git diffs and file changes.
type DiffProvider interface {
	// GetDiffAgainstMain returns the diff of the branch against main/master.
	// Uses three-dot syntax (main...HEAD) to show changes since divergence.
	GetDiffAgainstMain(path string) (string, error)

	// GetChangedFiles returns a list of file paths that changed compared to main.
	GetChangedFiles(path string) ([]string, error)

	// HasUncommittedChanges checks if a worktree has uncommitted changes.
	HasUncommittedChanges(path string) (bool, error)
}

// Repository combines all git operation interfaces into a single type.
// This composite interface is useful when a component needs access to
// multiple categories of git operations.
type Repository interface {
	GitOperations
	WorktreeManager
	BranchManager
	DiffProvider
}

// Ensure Manager implements all interfaces at compile time.
// These assertions will cause a compile error if Manager doesn't
// implement the required methods.
var (
	_ GitOperations   = (*Manager)(nil)
	_ WorktreeManager = (*Manager)(nil)
	_ BranchManager   = (*Manager)(nil)
	_ DiffProvider    = (*Manager)(nil)
	_ Repository      = (*Manager)(nil)
)

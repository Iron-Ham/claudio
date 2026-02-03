# Getting Started

This guide walks you through installing Claudio and running your first parallel development session.

## Prerequisites

Before installing Claudio, ensure you have:

- **Go 1.21+** - [Download Go](https://golang.org/dl/)
- **Git** - For version control and worktree management
- **tmux** - For process management (usually pre-installed on macOS/Linux)
- **Claude Code CLI or Codex CLI** - Install and authenticate your preferred backend

### Verifying Prerequisites

```bash
# Check Go version
go version  # Should be 1.21 or higher

# Check Git
git --version

# Check tmux
tmux -V

# Check Claude Code (if using Claude)
claude --version
claude auth status  # Should show authenticated

# Check Codex (if using Codex)
codex --version
# Authenticate via the Codex CLI per its documentation
```

## Installation

### From Source (Recommended)

```bash
# Clone the repository
git clone https://github.com/Iron-Ham/claudio.git
cd claudio

# Build and install
go install ./cmd/claudio
```

### Verify Installation

```bash
claudio --help
```

You should see the help output with available commands.

## Your First Session

### 1. Navigate to Your Project

Claudio works within Git repositories. Navigate to any project:

```bash
cd your-project
```

If it's not already a Git repository:

```bash
git init
git add .
git commit -m "initial commit"
```

### 2. Initialize Claudio

```bash
claudio init
```

This creates a `.claudio/` directory in your project for session data and worktrees.

### 3. Start a Session

```bash
claudio start my-feature
```

This launches the TUI (Terminal User Interface) dashboard.

### 4. Add Your First Instance

Press `a` in the TUI and enter a task description:

```
Implement user authentication endpoint
```

Claudio will:
1. Create a new git worktree for this task
2. Create a dedicated branch
3. Start an AI backend instance with your task

### 5. Monitor Progress

Watch the output panel as the backend works. You can:
- Press `j`/`k` to scroll through output
- Press `d` to see a diff of changes
- Press `/` to search output

### 6. Add More Instances

Press `a` again to add parallel tasks:

```
Write unit tests for authentication
```

```
Update API documentation
```

Each instance works independently in its own worktree.

### 7. Create Pull Requests

When an instance completes:
- Press `x` on the instance
- Choose to create a PR (or it may happen automatically based on config)

### 8. End the Session

Press `q` to quit the TUI. You'll be prompted about what to do with running instances.

## What Just Happened?

When you ran Claudio, it:

1. **Created isolated worktrees** - Each instance got its own copy of your codebase at `.claudio/worktrees/<id>/`
2. **Created branches** - Each worktree works on its own branch like `claudio/abc123-implement-auth`
3. **Shared context** - A `context.md` file was generated so instances know what others are working on
4. **Managed processes** - Each backend instance ran in its own tmux session

## Next Steps

- [Instance Management](./instance-management.md) - Learn about the instance lifecycle
- [TUI Navigation](./tui-navigation.md) - Master the keyboard shortcuts
- [Configuration](./configuration.md) - Customize Claudio's behavior
- [PR Creation](./pr-creation.md) - Understand the PR workflow

## Quick Reference

| Action | Command/Key |
|--------|-------------|
| Initialize | `claudio init` |
| Start session | `claudio start [name]` |
| Add instance | `a` in TUI |
| Stop instance | `x` in TUI |
| Create PR | `claudio pr [id]` |
| View status | `claudio status` |
| Quit | `q` |

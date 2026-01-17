# Claudio

**Multi-instance Claude Code orchestrator using Git worktrees**

Claudio enables parallel AI-assisted development by running multiple Claude Code instances simultaneously, each working in isolated Git worktrees.

## Features

### Core Orchestration
- **Parallel Instances** - Run multiple Claude Code processes simultaneously
- **Worktree Isolation** - Each instance works in its own Git worktree and branch
- **TUI Dashboard** - Real-time view of all instances with output streaming
- **Shared Context** - Instances can see what others are working on
- **Conflict Detection** - Detect when instances modify the same files
- **Task Chaining** - Define dependencies between tasks with `--depends-on`

### Planning Modes
- **Plan Mode** - Claude analyzes your codebase and generates structured task plans
- **UltraPlan Mode** - 4-phase hierarchical planning with automatic parallel execution
- **Multi-Pass Planning** - Three competing strategies evaluate and select the best approach
- **TripleShot Mode** - Spawn 3 parallel attempts per task, judge selects the best (experimental)

### Workflow Automation
- **PR Automation** - AI-generated pull requests with smart reviewer assignment
- **Cost Tracking** - Monitor token usage with configurable limits
- **Session Recovery** - Resume sessions after disconnection
- **Structured Logging** - JSON logs with filtering, rotation, and export

## Quick Start

```bash
# Install
go install github.com/Iron-Ham/claudio/cmd/claudio@latest

# Initialize in your project
cd your-project
claudio init

# Start a session
claudio start my-feature

# Add instances (in TUI, press 'a')
# Monitor, review, create PRs
```

## Documentation

### [User Guide](guide/index.md)
Comprehensive documentation covering concepts, workflows, and configuration.

- [Getting Started](guide/getting-started.md) - Installation and first session
- [Instance Management](guide/instance-management.md) - Lifecycle and coordination
- [TUI Navigation](guide/tui-navigation.md) - Keyboard shortcuts and views
- [Configuration](guide/configuration.md) - Customize Claudio
- [PR Creation](guide/pr-creation.md) - Pull request workflow
- [Task Chaining](guide/task-chaining.md) - Dependencies between tasks
- [Ultra-Plan Mode](guide/ultra-plan.md) - Intelligent hierarchical planning
- [Inline Planning](guide/inline-planning.md) - TUI-integrated planning workflows

### [Tutorials](tutorials/index.md)
Step-by-step guides for common workflows.

- [Quick Start](tutorials/quick-start.md) - 5-minute introduction
- [Feature Development](tutorials/feature-development.md) - Build features in parallel
- [Code Review Workflow](tutorials/code-review-workflow.md) - Parallel reviews
- [Large Refactor](tutorials/large-refactor.md) - Coordinate major changes

### [Reference](reference/index.md)
Technical reference documentation.

- [CLI Reference](reference/cli.md) - All commands and options
- [Configuration Reference](reference/configuration.md) - All config options
- [Keyboard Shortcuts](reference/keyboard-shortcuts.md) - Quick reference

### [Troubleshooting](troubleshooting.md)
Solutions to common issues.

### [FAQ](faq.md)
Frequently asked questions.

## Requirements

- Go 1.21+
- Git
- tmux
- [Claude Code CLI](https://claude.ai/claude-code) (authenticated)

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         TUI Layer                           │
│  (Bubbletea - renders state, handles keyboard input)        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Orchestrator                           │
│  - Manages session state                                    │
│  - Updates shared context                                   │
│  - Coordinates instances                                    │
└─────────────────────────────────────────────────────────────┘
                              │
          ┌───────────────────┼───────────────────┐
          ▼                   ▼                   ▼
┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐
│   Instance 1    │ │   Instance 2    │ │   Instance 3    │
│   (worktree)    │ │   (worktree)    │ │   (worktree)    │
│  claude process │ │  claude process │ │  claude process │
└─────────────────┘ └─────────────────┘ └─────────────────┘
```

## Contributing

Contributions welcome! Please see the [GitHub repository](https://github.com/Iron-Ham/claudio) for issues and pull requests.

## License

MIT

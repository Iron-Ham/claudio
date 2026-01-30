# User Guide

Welcome to the Claudio User Guide. This documentation covers everything you need to know to effectively use Claudio for parallel AI-assisted development.

## What is Claudio?

Claudio is a CLI/TUI tool that orchestrates multiple Claude Code instances simultaneously on a single project. It uses Git worktrees to isolate each instance's work, preventing conflicts while enabling truly parallel development.

## Guides

### [Getting Started](getting-started.md)
Install Claudio and run your first parallel development session. Learn the basic workflow from initialization to creating pull requests.

### [Instance Management](instance-management.md)
Deep dive into instance lifecycle, states, and management. Understand how instances work in isolation and how to coordinate them effectively.

### [TUI Navigation](tui-navigation.md)
Master the terminal UI with keyboard shortcuts, views, and panels. Learn to efficiently navigate output, manage instances, and use search.

### [Configuration](configuration.md)
Customize Claudio's behavior through config files and environment variables. Set up branch naming, PR templates, cost limits, and more.

### [PR Creation](pr-creation.md)
Create polished pull requests with AI-generated descriptions, automatic rebasing, and smart reviewer assignment.

### [Task Chaining](task-chaining.md)
Define dependencies between tasks to control execution order. Build complex workflows with parallel and sequential phases.

### [Plan Mode](plan.md)
Generate structured task plans from high-level objectives. Create execution plans that can be saved as JSON for Ultra-Plan or tracked as GitHub Issues.

### [Ultra-Plan Mode](ultra-plan.md)
Orchestrate complex tasks with intelligent planning. Let Claude analyze your codebase, create an execution plan, and coordinate parallel task execution automatically.

### [TripleShot Mode](tripleshot.md)
Run three parallel implementations and let a judge select the best. Ideal for tasks with multiple valid approaches or when optimal solution is unclear. Access via `:tripleshot` command in the TUI. Can be combined with adversarial review for higher quality results.

### [Adversarial Review](adversarial.md)
Iterative implementation with critical reviewer feedback. The implementer and reviewer loop until the code meets quality thresholds (score >= 8/10). Includes stuck instance detection and `:adversarial-retry` recovery command.

### [Inline Planning](inline-planning.md) (Experimental)
Start Plan and UltraPlan workflows directly from the standard TUI. Create plans, organize tasks into visual groups, and execute them without leaving your session.

## Quick Links

- [CLI Reference](../reference/cli.md) - Complete command documentation
- [Configuration Reference](../reference/configuration.md) - All config options
- [Keyboard Shortcuts](../reference/keyboard-shortcuts.md) - Quick reference card
- [Troubleshooting](../troubleshooting.md) - Common issues and solutions
- [FAQ](../faq.md) - Frequently asked questions

## Core Concepts

### Instances
An instance is a Claude Code process working on a specific task. Each instance runs independently and can be started, paused, resumed, or stopped.

### Worktrees
Each instance works in its own [Git worktree](https://git-scm.com/docs/git-worktree) - a separate working directory linked to your repository. This provides complete file isolation between instances.

### Sessions
A session groups multiple instances together. Sessions persist across restarts and can be recovered if the TUI is closed while instances are running.

### Shared Context
Claudio generates context files that inform each instance about what others are working on, helping coordinate parallel work and avoid conflicts.

## Typical Workflow

```
1. Initialize        claudio init
2. Start session     claudio start feature-work
3. Add instances     Press 'a' → describe tasks
4. Monitor           Watch output, check diffs
5. Create PRs        Press 'x' → create PR
6. Cleanup           claudio cleanup
```

## Need Help?

- [Troubleshooting Guide](../troubleshooting.md) - Solutions to common problems
- [FAQ](../faq.md) - Frequently asked questions
- [GitHub Issues](https://github.com/Iron-Ham/claudio/issues) - Report bugs or request features

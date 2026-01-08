# Tutorials

Step-by-step guides for common Claudio workflows.

## Getting Started

### [Quick Start: Your First Parallel Session](quick-start.md)
**5 minutes** - Install Claudio and run your first parallel development session. Perfect for getting started.

## Development Workflows

### [Feature Development](feature-development.md)
**15-20 minutes** - Build a complete feature by breaking it into parallel tasks. Learn to coordinate multiple instances working on different components.

### [Code Review Workflow](code-review-workflow.md)
**10 minutes** - Use multiple specialized instances for thorough code reviews: security, performance, bugs, and documentation.

### [Large Refactor](large-refactor.md)
**20 minutes** - Coordinate a major refactoring effort across your codebase. Handle dependencies, conflicts, and staged rollout.

## Quick Links

- [User Guide](../guide/index.md) - Comprehensive documentation
- [CLI Reference](../reference/cli.md) - All commands
- [Configuration](../guide/configuration.md) - Customize Claudio
- [Troubleshooting](../troubleshooting.md) - Common issues

## Tutorial Tips

### Before You Start

1. Ensure Claudio is installed: `claudio --help`
2. Have Claude Code authenticated: `claude auth status`
3. Work in a Git repository: `git status`
4. Initialize Claudio: `claudio init`

### Getting Help

- Press `?` in the TUI for keyboard shortcuts
- Run `claudio <command> --help` for command help
- Check [Troubleshooting](../troubleshooting.md) for common issues

### Best Practices from Tutorials

| Practice | Why |
|----------|-----|
| Break tasks into independent units | Minimizes conflicts |
| Be specific in task descriptions | Better AI output |
| Monitor conflicts regularly | Catch issues early |
| Create PRs in dependency order | Cleaner merges |
| Use cost limits | Predictable spending |

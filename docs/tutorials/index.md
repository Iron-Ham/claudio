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

## Platform-Specific Guides

Each platform guide covers worktree considerations, build strategies, testing approaches, dependency management, and CI integration specific to that technology stack.

### [Web Development (Node.js/React/Vue)](web-development.md)
**20-30 minutes** - Use Claudio with JavaScript/TypeScript web projects. Covers npm/yarn/pnpm, build caching, dev server coordination, and framework-specific workflows for React, Vue, Angular, Next.js, and more.

### [Go Development](go-development.md)
**20-30 minutes** - Use Claudio with Go projects. Covers module caching, build optimization, test parallelism, workspace patterns, and code generation workflows.

### [Python Development](python-development.md)
**20-30 minutes** - Use Claudio with Python projects. Covers virtual environment management, pip/poetry/conda, testing with pytest, and framework-specific patterns for Django, Flask, and FastAPI.

### [Rust Development](rust-development.md)
**20-30 minutes** - Use Claudio with Rust projects. Covers Cargo workspace management, target directory isolation, build caching with sccache, and incremental compilation strategies.

### [iOS Development](ios-development.md)
**20-30 minutes** - Use Claudio with Xcode projects. Covers DerivedData management, build strategies, Swift Package Manager, simulator coordination, and handling `project.pbxproj` conflicts.

### [Android Development](android-development.md)
**25-35 minutes** - Use Claudio with Android Studio projects. Covers Gradle build caching, module-based development, emulator coordination, and Jetpack Compose workflows.

## Architecture Guides

### [Full-Stack Development](fullstack-development.md)
**25-35 minutes** - Use Claudio with multi-service applications combining frontend and backend. Covers Docker Compose coordination, database isolation, API contract management, and service orchestration.

### [Monorepo Development](monorepo-development.md)
**25-35 minutes** - Use Claudio with large monorepos. Covers sparse checkout optimization, Turborepo/Nx integration, package-targeted development, and efficient CI pipelines.

### [Data Science & ML](datascience-development.md)
**25-35 minutes** - Use Claudio with machine learning projects. Covers Jupyter notebook management, experiment tracking, GPU resource coordination, and model versioning with DVC.

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
| Include build/test commands in tasks | Ensures verification |
| Consider platform-specific caching | Faster iteration |

### Choosing the Right Guide

| If you're working on... | Start with... |
|------------------------|---------------|
| A new feature in any language | [Feature Development](feature-development.md) |
| A major codebase refactor | [Large Refactor](large-refactor.md) |
| iOS/macOS app | [iOS Development](ios-development.md) |
| Android app | [Android Development](android-development.md) |
| Web frontend or Node.js backend | [Web Development](web-development.md) |
| Go microservice | [Go Development](go-development.md) |
| Python API or data pipeline | [Python Development](python-development.md) |
| Rust application | [Rust Development](rust-development.md) |
| Multiple connected services | [Full-Stack Development](fullstack-development.md) |
| Large monorepo | [Monorepo Development](monorepo-development.md) |
| Machine learning project | [Data Science & ML](datascience-development.md) |
| Code review process | [Code Review Workflow](code-review-workflow.md) |

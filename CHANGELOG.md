# Changelog

All notable changes to Claudio will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-01-08

Initial release of Claudio - a CLI/TUI orchestration tool for running multiple Claude Code instances simultaneously using Git worktrees.

### Added

#### Core Features
- **Parallel Instance Management**: Run multiple Claude Code instances simultaneously on a single project
- **Git Worktree Isolation**: Each instance works in its own worktree and branch, preventing conflicts
- **TUI Dashboard**: Real-time terminal interface showing all instances with output streaming
- **Shared Context**: Auto-generated context files help instances coordinate and avoid duplicated work
- **Session Persistence**: Sessions survive disconnections and can be recovered on restart

#### Instance Control
- Start, pause, resume, and stop instances via TUI or CLI
- Automatic status detection (running, waiting for input, completed, error)
- Timeout detection and recovery for stuck instances
- Reconnect functionality for stopped instances

#### TUI Features
- Sidebar showing all instances with status indicators and pagination
- Scrollable output view with navigation controls (j/k, Page Up/Down, g/G)
- Output search and filtering with `/` command
- Interactive task input with keyboard navigation and paste support
- Task templates via `/` commands (e.g., `/test`, `/docs`, `/refactor`)
- Diff preview panel with `d` keyboard shortcut
- Conflict detail view with `c` keyboard shortcut
- Help overlay with `?`
- Completed instances section for finished work

#### PR Automation
- Claude-powered PR creation with smart rebase
- PR template support with customizable templates
- Automatic reviewer assignment from CODEOWNERS or configuration
- Auto-PR workflow when stopping instances with `x`
- Megamerge slash command for batch PR merging

#### Conflict Detection
- Real-time file conflict detection using fsnotify
- Visual warnings in TUI when multiple instances modify the same file
- Interactive conflict detail view

#### Configuration
- YAML configuration file support (`~/.config/claudio/config.yaml`)
- Environment variable overrides with `CLAUDIO_` prefix
- Interactive TUI for `claudio config` command
- Configurable branch naming convention (prefix, include ID)
- Completion actions: prompt, keep_branch, merge_staging, merge_main, auto_pr
- TUI settings: auto-focus, max output lines
- Instance settings: buffer size, capture interval, tmux dimensions

#### Resource Tracking
- Token usage tracking per instance
- API cost estimation
- Resource metrics display in TUI

#### CLI Commands
- `claudio init` - Initialize Claudio in a git repository
- `claudio start [name]` - Start a session and launch the TUI
- `claudio add "task"` - Add a new Claude instance with a task
- `claudio status` - Show current session status
- `claudio stop` - Stop all instances and end the session
- `claudio remove <id>` - Remove a specific instance and its worktree
- `claudio config` - View/edit configuration

#### Developer Experience
- Automatic stale worktree cleanup
- Native text selection (no mouse capture)
- Improved color contrast for readability
- Dynamic tmux pane resizing

### Infrastructure
- Integration tests with CI pipeline
- GitHub Actions workflow for testing on Ubuntu and macOS
- golangci-lint for code quality
- Comprehensive documentation with MkDocs

### Documentation
- Full user guide with getting started instructions
- Step-by-step tutorials for common workflows
- Complete CLI reference
- Configuration reference
- Troubleshooting guide and FAQ

[0.1.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.1.0

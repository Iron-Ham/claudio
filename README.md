# Claudio

A CLI/TUI tool for running multiple Claude Code instances simultaneously on a single project using git worktrees.

[![Documentation](https://img.shields.io/badge/docs-mkdocs-blue)](https://iron-ham.github.io/claudio/)
[![Go Report Card](https://goreportcard.com/badge/github.com/Iron-Ham/claudio)](https://goreportcard.com/report/github.com/Iron-Ham/claudio)

## Overview

Claudio enables parallel AI-assisted development by orchestrating multiple Claude Code instances, each working in isolated git worktrees. A central orchestrator coordinates the work, tracks what each instance is doing, and helps prevent conflicts.

## Documentation

**[Full documentation is available here →](https://iron-ham.github.io/claudio/)**

- [User Guide](https://iron-ham.github.io/claudio/guide/) - Comprehensive documentation
- [Tutorials](https://iron-ham.github.io/claudio/tutorials/) - Step-by-step workflows
- [CLI Reference](https://iron-ham.github.io/claudio/reference/cli/) - All commands
- [Configuration](https://iron-ham.github.io/claudio/reference/configuration/) - All options
- [FAQ](https://iron-ham.github.io/claudio/faq/) - Common questions

## Features

- **Parallel Instances** - Run multiple Claude Code processes simultaneously
- **Worktree Isolation** - Each instance works in its own git worktree/branch
- **TUI Dashboard** - Real-time view of all instances with output streaming
- **Shared Context** - Instances can see what others are working on via auto-generated context files
- **Process Control** - Start, pause, resume, and stop instances
- **Conflict Detection** - Detect when instances modify the same files
- **PR Automation** - AI-generated pull requests with smart reviewer assignment
- **Cost Tracking** - Monitor token usage and API costs
- **Session Recovery** - Resume sessions after disconnection

## Requirements

- Go 1.21+
- Git
- tmux
- [Claude Code](https://claude.ai/claude-code) CLI installed and authenticated
- [GitHub CLI](https://cli.github.com/) (optional, for PR creation)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/Iron-Ham/claudio.git
cd claudio

# Build
go build -o claudio ./cmd/claudio

# Install to your PATH (optional)
go install ./cmd/claudio
```

### Verify Installation

```bash
claudio --help
```

## Quick Start

```bash
# Navigate to your project (must be a git repository)
cd your-project

# Initialize Claudio
claudio init

# Start a session (launches the TUI)
claudio start my-feature

# Or add instances directly from CLI
claudio add "Implement user authentication API"
claudio add "Write unit tests for auth module"
claudio add "Update API documentation"
```

## Usage

### Commands

| Command | Description |
|---------|-------------|
| `claudio init` | Initialize Claudio in the current git repository |
| `claudio start [name]` | Start a new session and launch the TUI |
| `claudio add "task"` | Add a new Claude instance with the given task |
| `claudio status` | Show current session status |
| `claudio stop` | Stop all instances and end the session |
| `claudio stop -f` | Force stop without prompts |

### TUI Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `1-9` | Select instance by number |
| `Tab` / `l` / `→` | Next instance |
| `Shift+Tab` / `h` / `←` | Previous instance |
| `a` | Add new instance |
| `s` | Start selected instance |
| `p` | Pause/resume instance |
| `x` | Stop instance |
| `Enter` | Focus instance for input |
| `?` | Toggle help |
| `q` | Quit |

## How It Works

### Architecture

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

### Worktrees

Each Claude instance runs in its own [git worktree](https://git-scm.com/docs/git-worktree), providing:

- **Isolation**: Instances can modify files without affecting each other
- **Parallel branches**: Each instance works on its own branch
- **Easy cleanup**: Worktrees can be removed without losing the main repo

Worktrees are created in `.claudio/worktrees/<instance-id>/` with branches named `<prefix>/<instance-id>-<task-slug>` (the prefix and whether to include the instance ID are configurable).

### Shared Context

Claudio generates a `context.md` file that's injected into each worktree, containing:

- What each instance is working on
- Current status of all instances
- Files being modified
- Coordination notes

This helps Claude instances be aware of parallel work and avoid conflicts.

## Project Structure

```
.claudio/
├── session.json          # Current session state
├── context.md            # Shared context file
└── worktrees/
    ├── abc123/           # Instance 1 worktree
    ├── def456/           # Instance 2 worktree
    └── ...
```

## Configuration

Claudio can be configured via a YAML config file or environment variables.

### Config File Locations

Claudio searches for config files in this order:
1. `~/.config/claudio/config.yaml` (recommended)
2. `./config.yaml` (current directory)

### Creating a Config File

```bash
# Create a config file with defaults and comments
claudio config init

# Or view current configuration
claudio config

# Set individual values
claudio config set completion.default_action auto_pr
claudio config set tui.auto_focus_on_input false
```

### Configuration Options

```yaml
# Claudio Configuration
# ~/.config/claudio/config.yaml

# Action when an instance completes its task
# Options: prompt, keep_branch, merge_staging, merge_main, auto_pr
completion:
  default_action: prompt

# TUI (terminal user interface) settings
tui:
  # Automatically focus new instances for input
  auto_focus_on_input: true
  # Maximum number of output lines to display per instance
  max_output_lines: 1000

# Instance settings (advanced)
instance:
  # Output buffer size in bytes (default: 100KB)
  output_buffer_size: 100000
  # How often to capture output from tmux in milliseconds
  capture_interval_ms: 100
  # tmux pane dimensions
  tmux_width: 200
  tmux_height: 50

# Branch naming convention
branch:
  # Prefix for branch names (default: "claudio")
  # Examples: "claudio", "Iron-Ham", "feature"
  prefix: claudio
  # Include instance ID in branch names (default: true)
  # When true: <prefix>/<id>-<slug> (e.g., claudio/abc123-fix-bug)
  # When false: <prefix>/<slug> (e.g., claudio/fix-bug)
  include_id: true
```

### Environment Variables

All config options can be set via environment variables with the `CLAUDIO_` prefix.
Use underscores instead of dots for nested keys:

```bash
export CLAUDIO_COMPLETION_DEFAULT_ACTION=auto_pr
export CLAUDIO_TUI_MAX_OUTPUT_LINES=2000
export CLAUDIO_BRANCH_PREFIX=Iron-Ham
export CLAUDIO_BRANCH_INCLUDE_ID=false
```

### Config Commands

| Command | Description |
|---------|-------------|
| `claudio config` | Show current configuration |
| `claudio config init` | Create a default config file |
| `claudio config set <key> <value>` | Set a configuration value |
| `claudio config path` | Show config file locations |

## Development

### Building

```bash
go build ./cmd/claudio
```

### Running Tests

```bash
go test ./...
```

### Project Layout

```
claudio/
├── cmd/claudio/          # Entry point
├── internal/
│   ├── cmd/              # CLI commands (Cobra)
│   ├── orchestrator/     # Session & coordination
│   ├── instance/         # Process management
│   ├── worktree/         # Git worktree operations
│   └── tui/              # Terminal UI (Bubbletea)
└── go.mod
```

## Troubleshooting

### "not a git repository"

Claudio requires a git repository. Initialize one with:

```bash
git init
```

### Claude process not starting

Ensure Claude Code CLI is installed and authenticated:

```bash
claude --version
claude auth status
```

### Worktree conflicts

If you encounter worktree issues, you can manually clean up:

```bash
# List worktrees
git worktree list

# Remove a worktree
git worktree remove .claudio/worktrees/<id>

# Prune stale worktree references
git worktree prune
```

## Contributing

Contributions welcome! Please open an issue or PR.

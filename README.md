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

## Logging

Claudio includes a built-in debug logging system for troubleshooting and post-hoc analysis of sessions.

### Enabling/Disabling Logging

Logging is enabled by default. Configure it via config file or environment variables:

```yaml
# ~/.config/claudio/config.yaml
logging:
  enabled: true        # Enable/disable logging (default: true)
  level: info          # Log level (default: info)
  max_size_mb: 10      # Max file size before rotation (default: 10)
  max_backups: 3       # Number of backup files to keep (default: 3)
```

Or via environment variables:

```bash
export CLAUDIO_LOGGING_ENABLED=true
export CLAUDIO_LOGGING_LEVEL=debug
```

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Verbose output for detailed troubleshooting (all messages) |
| `info` | General operational messages (default) |
| `warn` | Warning conditions that may need attention |
| `error` | Error conditions that affect functionality |

Each level includes all messages at and above its severity. For example, `info` includes `info`, `warn`, and `error` messages.

### Log File Location

Logs are stored in the session directory:

```
.claudio/sessions/<session-id>/debug.log
```

Rotated backup files (if enabled):
- `debug.log.1` - Most recent backup
- `debug.log.2` - Second most recent
- `debug.log.3` - Third most recent (oldest)

### Viewing Logs

Use the `claudio logs` command to view and filter logs:

```bash
# Show last 50 lines from most recent session
claudio logs

# Show all logs (no line limit)
claudio logs -n 0

# View logs from a specific session
claudio logs -s abc123

# Follow logs in real-time (like tail -f)
claudio logs -f

# Filter by minimum log level
claudio logs --level warn

# Show logs from the last hour
claudio logs --since 1h

# Search for patterns using regex
claudio logs --grep "error|failed"

# Combine filters
claudio logs --level error --since 30m --grep "instance"
```

### Command Options

| Flag | Short | Description |
|------|-------|-------------|
| `--session` | `-s` | Session ID (default: most recent) |
| `--tail` | `-n` | Number of lines to show, 0 for all (default: 50) |
| `--follow` | `-f` | Follow log output in real-time |
| `--level` | | Minimum level to show (debug/info/warn/error) |
| `--since` | | Show logs since duration (e.g., 1h, 30m, 2h30m) |
| `--grep` | | Filter by regex pattern |

### Example Log Output

Logs are stored in JSON format for machine parsing:

```json
{"time":"2024-01-15T10:30:45.123Z","level":"INFO","msg":"session started","session_id":"abc123"}
{"time":"2024-01-15T10:30:46.456Z","level":"DEBUG","msg":"instance created","session_id":"abc123","instance_id":"def456","task":"implement auth"}
{"time":"2024-01-15T10:31:00.789Z","level":"WARN","msg":"conflict detected","session_id":"abc123","files":["src/auth.go"]}
```

The `claudio logs` command renders these with colors and formatting:

```
[10:30:45.123] [INFO] session started session_id=abc123
[10:30:46.456] [DEBUG] instance created session_id=abc123 instance_id=def456 task=implement auth
[10:31:00.789] [WARN] conflict detected session_id=abc123 files=["src/auth.go"]
```

### Log Aggregation and Export

For advanced analysis, use the logging package's export utilities:

```go
import "github.com/Iron-Ham/claudio/internal/logging"

// Aggregate all logs from a session
entries, err := logging.AggregateLogs(sessionDir)

// Filter logs
filtered := logging.FilterLogs(entries, logging.LogFilter{
    Level:      "WARN",
    InstanceID: "abc123",
    Phase:      "execution",
})

// Export to various formats
logging.ExportLogEntries(filtered, "output.json", "json")
logging.ExportLogEntries(filtered, "output.txt", "text")
logging.ExportLogEntries(filtered, "output.csv", "csv")
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

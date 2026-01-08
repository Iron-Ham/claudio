# Configuration

Claudio is highly configurable through YAML config files and environment variables.

## Config File Locations

Claudio searches for configuration in this order:

1. Path specified via `--config` flag
2. `~/.config/claudio/config.yaml` (recommended)
3. `./config.yaml` (current directory)

## Creating a Config File

### Interactive Setup

```bash
claudio config init
```

This creates a config file with all options and comments explaining each one.

### Manual Setup

Create `~/.config/claudio/config.yaml`:

```yaml
# Claudio Configuration

completion:
  default_action: prompt

tui:
  auto_focus_on_input: true
  max_output_lines: 1000

branch:
  prefix: claudio
  include_id: true

pr:
  draft: false
  auto_rebase: true
  use_ai: true
```

## Configuration Commands

```bash
# View current configuration
claudio config

# Show configuration interactively
claudio config show

# Set a value
claudio config set <key> <value>

# Edit config file in $EDITOR
claudio config edit

# Reset to defaults
claudio config reset
```

## All Configuration Options

### Completion Settings

What happens when an instance finishes its task.

```yaml
completion:
  # Action when instance completes
  # Options: prompt, keep_branch, merge_staging, merge_main, auto_pr
  default_action: prompt
```

| Action | Description |
|--------|-------------|
| `prompt` | Ask what to do (default) |
| `keep_branch` | Keep the branch, don't merge |
| `merge_staging` | Auto-merge to staging branch |
| `merge_main` | Auto-merge to main branch |
| `auto_pr` | Automatically create a PR |

### TUI Settings

Control the terminal UI behavior.

```yaml
tui:
  # Auto-focus new instances for input
  auto_focus_on_input: true

  # Maximum output lines to display (default: 1000)
  max_output_lines: 1000
```

### Instance Settings

Control instance process behavior.

```yaml
instance:
  # Output buffer size in bytes (default: 100KB)
  output_buffer_size: 100000

  # How often to capture output from tmux (milliseconds)
  capture_interval_ms: 100

  # tmux pane dimensions
  tmux_width: 200
  tmux_height: 50
```

### Branch Settings

Control how branches are named.

```yaml
branch:
  # Prefix for branch names
  # Examples: "claudio", "feature", "username"
  prefix: claudio

  # Include instance ID in branch name
  # true:  claudio/abc123-fix-bug
  # false: claudio/fix-bug
  include_id: true
```

### PR Settings

Control pull request creation.

```yaml
pr:
  # Create PRs as drafts
  draft: false

  # Rebase on main before creating PR
  auto_rebase: true

  # Use Claude to generate PR title and description
  use_ai: true

  # Auto-create PR when stopping instance with 'x'
  auto_pr_on_stop: false

  # Custom PR body template (Go text/template syntax)
  template: |
    ## Summary
    {{.Summary}}

    ## Changes
    {{.Changes}}

    ## Testing
    {{.Testing}}

  # Default labels for all PRs
  labels:
    - automated
    - needs-review

  # Reviewer assignment
  reviewers:
    # Always assign these reviewers
    default:
      - teammate1
      - teammate2

    # Assign reviewers based on files changed
    by_path:
      "src/api/**": [api-team]
      "src/frontend/**": [frontend-team]
      "*.md": [docs-team]
```

### Cleanup Settings

Control cleanup behavior.

```yaml
cleanup:
  # Warn on startup if stale resources exist
  warn_on_stale: true

  # Keep branches that exist on remote
  keep_remote_branches: true
```

### Resource Settings

Control cost tracking and limits.

```yaml
resources:
  # Warn when session cost exceeds this (USD)
  cost_warning_threshold: 5.00

  # Pause all instances when cost exceeds this (0 = no limit)
  cost_limit: 0

  # Token limit per instance (0 = no limit)
  token_limit_per_instance: 0

  # Show metrics in TUI sidebar
  show_metrics_in_sidebar: true
```

## Environment Variables

All config options can be set via environment variables:

- Prefix: `CLAUDIO_`
- Nested keys: Use `_` instead of `.`

### Examples

```bash
# Set completion action
export CLAUDIO_COMPLETION_DEFAULT_ACTION=auto_pr

# Set branch prefix
export CLAUDIO_BRANCH_PREFIX=Iron-Ham
export CLAUDIO_BRANCH_INCLUDE_ID=false

# Set cost limit
export CLAUDIO_RESOURCES_COST_LIMIT=10.00

# Disable metrics in sidebar
export CLAUDIO_RESOURCES_SHOW_METRICS_IN_SIDEBAR=false

# Set TUI options
export CLAUDIO_TUI_MAX_OUTPUT_LINES=2000
export CLAUDIO_TUI_AUTO_FOCUS_ON_INPUT=false
```

### Priority

Environment variables override config file values.

## PR Templates

Use Go's `text/template` syntax for custom PR bodies.

### Available Variables

| Variable | Description |
|----------|-------------|
| `{{.Summary}}` | AI-generated summary |
| `{{.Changes}}` | List of changed files |
| `{{.Testing}}` | AI-generated test plan |
| `{{.Branch}}` | Branch name |
| `{{.Task}}` | Original task description |

### Example Template

```yaml
pr:
  template: |
    ## What
    {{.Summary}}

    ## Why
    Task: {{.Task}}

    ## Changes
    {{.Changes}}

    ## How to Test
    {{.Testing}}

    ## Checklist
    - [ ] Tests pass
    - [ ] Documentation updated
    - [ ] Ready for review
```

## Path-Based Reviewers

Assign reviewers based on which files are changed.

### Glob Patterns

```yaml
pr:
  reviewers:
    by_path:
      # Match all files in a directory
      "src/api/**": [api-team]

      # Match specific extensions
      "*.ts": [typescript-team]
      "*.md": [docs-team]

      # Match specific files
      "package.json": [deps-team]
      "Dockerfile": [devops]

      # Match nested patterns
      "**/test/**": [qa-team]
```

### How It Works

1. Claudio detects which files changed in the PR
2. Each file is matched against patterns
3. Matching reviewers are added (deduplicated)
4. Default reviewers are always added

## Example Configurations

### Solo Developer

```yaml
completion:
  default_action: keep_branch

branch:
  prefix: wip
  include_id: false

pr:
  draft: true
  auto_rebase: true
```

### Team Environment

```yaml
completion:
  default_action: auto_pr

branch:
  prefix: claudio
  include_id: true

pr:
  draft: false
  auto_rebase: true
  use_ai: true
  labels:
    - ai-generated
  reviewers:
    default: [tech-lead]
    by_path:
      "src/security/**": [security-team]

resources:
  cost_warning_threshold: 10.00
  cost_limit: 50.00
```

### CI/Automation

```yaml
completion:
  default_action: auto_pr

tui:
  auto_focus_on_input: false

pr:
  draft: false
  auto_rebase: true
  use_ai: true
  auto_pr_on_stop: true

cleanup:
  warn_on_stale: false
  keep_remote_branches: false
```

## Validating Configuration

```bash
# View current config (validates and shows values)
claudio config show

# Check if config file is valid
claudio config
```

If there are errors, Claudio will report them and fall back to defaults.

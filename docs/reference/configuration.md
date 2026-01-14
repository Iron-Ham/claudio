# Configuration Reference

Complete reference for all Claudio configuration options.

## Config File

Claudio reads configuration from YAML files in this order:
1. Path specified via `--config` flag
2. `~/.config/claudio/config.yaml`
3. `./config.yaml`

Create a config file:
```bash
claudio config init
```

## Configuration Options

### completion

Controls what happens when an instance completes its task.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `completion.default_action` | string | `"prompt"` | Action on completion |

**default_action values:**

| Value | Description |
|-------|-------------|
| `prompt` | Ask what to do (interactive) |
| `keep_branch` | Keep the branch, don't merge |
| `merge_staging` | Auto-merge to staging branch |
| `merge_main` | Auto-merge to main branch |
| `auto_pr` | Automatically create a pull request |

```yaml
completion:
  default_action: prompt
```

---

### tui

Controls the Terminal UI behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `tui.auto_focus_on_input` | bool | `true` | Auto-focus new instances for input |
| `tui.max_output_lines` | int | `1000` | Maximum output lines to display |

```yaml
tui:
  auto_focus_on_input: true
  max_output_lines: 1000
```

---

### instance

Controls instance process behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `instance.output_buffer_size` | int | `100000` | Output buffer size in bytes (100KB) |
| `instance.capture_interval_ms` | int | `100` | tmux capture interval in milliseconds |
| `instance.tmux_width` | int | `200` | tmux pane width |
| `instance.tmux_height` | int | `50` | tmux pane height |

```yaml
instance:
  output_buffer_size: 100000
  capture_interval_ms: 100
  tmux_width: 200
  tmux_height: 50
```

---

### branch

Controls branch naming conventions.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `branch.prefix` | string | `"claudio"` | Branch name prefix |
| `branch.include_id` | bool | `true` | Include instance ID in branch name |

**Branch name format:**
- With ID: `<prefix>/<id>-<task-slug>` (e.g., `claudio/abc123-fix-bug`)
- Without ID: `<prefix>/<task-slug>` (e.g., `claudio/fix-bug`)

```yaml
branch:
  prefix: claudio
  include_id: true
```

---

### pr

Controls pull request creation behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `pr.draft` | bool | `false` | Create PRs as drafts |
| `pr.auto_rebase` | bool | `true` | Rebase on main before PR |
| `pr.use_ai` | bool | `true` | Use Claude for PR title/description |
| `pr.auto_pr_on_stop` | bool | `false` | Auto-create PR when stopping with 'x' |
| `pr.template` | string | `""` | Custom PR body template |
| `pr.labels` | []string | `[]` | Default labels for all PRs |
| `pr.reviewers.default` | []string | `[]` | Default reviewers |
| `pr.reviewers.by_path` | map | `{}` | Path-based reviewer assignment |

```yaml
pr:
  draft: false
  auto_rebase: true
  use_ai: true
  auto_pr_on_stop: false
  template: ""
  labels:
    - automated
  reviewers:
    default:
      - tech-lead
    by_path:
      "src/api/**": [backend-team]
      "*.md": [docs-team]
```

#### PR Template Variables

Use Go `text/template` syntax:

| Variable | Description |
|----------|-------------|
| `{{.Summary}}` | AI-generated summary |
| `{{.Changes}}` | List of changed files |
| `{{.Testing}}` | AI-generated test plan |
| `{{.Branch}}` | Branch name |
| `{{.Task}}` | Original task description |

```yaml
pr:
  template: |
    ## Summary
    {{.Summary}}

    ## Changes
    {{.Changes}}

    ## Testing
    {{.Testing}}
```

#### Path-Based Reviewers

Glob patterns are supported:

| Pattern | Matches |
|---------|---------|
| `*.ts` | All `.ts` files in root |
| `**/*.ts` | All `.ts` files anywhere |
| `src/api/**` | All files under `src/api/` |
| `src/api/*.ts` | `.ts` files directly in `src/api/` |

---

### paths

Controls where Claudio stores data.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `paths.worktree_dir` | string | `""` | Directory for git worktrees |

**worktree_dir behavior:**
- Empty string (default): Uses `.claudio/worktrees` relative to repository root
- Absolute path: Uses the path as-is (e.g., `/fast-drive/worktrees`)
- Relative path: Resolved relative to repository root (e.g., `.worktrees`)
- Home expansion: Supports `~` prefix (e.g., `~/claudio-worktrees`)

**Use cases:**
- Store worktrees on a faster drive for better performance
- Keep worktrees outside the repository to avoid cluttering project directory
- Use a different directory convention (e.g., `.worktrees/` instead of `.claudio/worktrees/`)

```yaml
paths:
  # Use default (.claudio/worktrees relative to repo root)
  worktree_dir: ""

  # Or use a custom location:
  # worktree_dir: ~/claudio-worktrees
  # worktree_dir: /fast-ssd/worktrees
  # worktree_dir: .worktrees
```

---

### cleanup

Controls cleanup behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cleanup.warn_on_stale` | bool | `true` | Warn on startup if stale resources exist |
| `cleanup.keep_remote_branches` | bool | `true` | Don't delete branches that exist on remote |

```yaml
cleanup:
  warn_on_stale: true
  keep_remote_branches: true
```

---

### resources

Controls resource monitoring and cost tracking.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `resources.cost_warning_threshold` | float | `5.00` | Warn when cost exceeds this (USD) |
| `resources.cost_limit` | float | `0` | Pause all instances at this cost (0 = no limit) |
| `resources.token_limit_per_instance` | int | `0` | Token limit per instance (0 = no limit) |
| `resources.show_metrics_in_sidebar` | bool | `true` | Show metrics in TUI sidebar |

```yaml
resources:
  cost_warning_threshold: 5.00
  cost_limit: 0
  token_limit_per_instance: 0
  show_metrics_in_sidebar: true
```

---

### ultraplan

Controls ultra-plan mode behavior.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `ultraplan.max_parallel` | int | `3` | Maximum concurrent child sessions (0 = unlimited) |
| `ultraplan.notifications.enabled` | bool | `true` | Play notifications when user input needed |
| `ultraplan.notifications.use_sound` | bool | `false` | Use system sound (macOS only) |
| `ultraplan.notifications.sound_path` | string | `""` | Custom sound file path (macOS only) |

**Why limit parallelism?**
- Anthropic API rate limits can throttle many parallel requests
- Each parallel session incurs API costs
- More sessions = higher merge conflict risk during consolidation
- Easier to monitor fewer concurrent sessions

```yaml
ultraplan:
  max_parallel: 3
  notifications:
    enabled: true
    use_sound: false
    sound_path: ""
```

---

### experimental

Controls experimental features that may change or be removed. These features are disabled by default.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `experimental.intelligent_naming` | bool | `false` | Use Claude to generate short, descriptive instance names |
| `experimental.triple_shot` | bool | `false` | Spawn three parallel instances and select the best solution |
| `experimental.inline_plan` | bool | `false` | Enable `:multiplan` command in the TUI (`:plan` is always available) |
| `experimental.inline_ultraplan` | bool | `false` | Enable `:ultraplan` command in the TUI |
| `experimental.grouped_instance_view` | bool | `false` | Enable visual group organization in the sidebar |

**Feature descriptions:**

| Feature | Description |
|---------|-------------|
| `intelligent_naming` | Uses Claude to generate short, descriptive instance names for the sidebar based on the task and Claude's initial output. Requires `ANTHROPIC_API_KEY`. |
| `triple_shot` | Spawns three parallel instances working on the same problem, then uses a judge instance to evaluate and select the best solution. |
| `inline_plan` | Enables the `:multiplan` command in the standard TUI for multi-pass planning with 3 planners + assessor. The `:plan` command is always available without this setting. |
| `inline_ultraplan` | Enables the `:ultraplan` command in the standard TUI, allowing you to start an UltraPlan workflow with parallel task execution. |
| `grouped_instance_view` | Organizes instances visually by execution group in the TUI sidebar. Related tasks are grouped together, with sub-groups for dependency chains. |

```yaml
experimental:
  intelligent_naming: false
  triple_shot: false
  inline_plan: false  # enables :multiplan; :plan is always available
  inline_ultraplan: false
  grouped_instance_view: false
```

See the [Inline Planning Guide](../guide/inline-planning.md) for detailed usage of inline planning features.

---

## Environment Variables

All options can be set via environment variables:

**Format:** `CLAUDIO_<SECTION>_<KEY>`

Replace dots with underscores and use uppercase:

| Config Key | Environment Variable |
|------------|---------------------|
| `completion.default_action` | `CLAUDIO_COMPLETION_DEFAULT_ACTION` |
| `branch.prefix` | `CLAUDIO_BRANCH_PREFIX` |
| `branch.include_id` | `CLAUDIO_BRANCH_INCLUDE_ID` |
| `pr.draft` | `CLAUDIO_PR_DRAFT` |
| `pr.use_ai` | `CLAUDIO_PR_USE_AI` |
| `resources.cost_limit` | `CLAUDIO_RESOURCES_COST_LIMIT` |
| `ultraplan.max_parallel` | `CLAUDIO_ULTRAPLAN_MAX_PARALLEL` |
| `paths.worktree_dir` | `CLAUDIO_PATHS_WORKTREE_DIR` |
| `experimental.inline_plan` | `CLAUDIO_EXPERIMENTAL_INLINE_PLAN` |
| `experimental.inline_ultraplan` | `CLAUDIO_EXPERIMENTAL_INLINE_ULTRAPLAN` |
| `experimental.grouped_instance_view` | `CLAUDIO_EXPERIMENTAL_GROUPED_INSTANCE_VIEW` |

**Priority:** Environment variables override config file values.

---

## Complete Example

```yaml
# ~/.config/claudio/config.yaml

# What to do when instance completes
completion:
  default_action: prompt

# TUI settings
tui:
  auto_focus_on_input: true
  max_output_lines: 1000

# Instance process settings
instance:
  output_buffer_size: 100000
  capture_interval_ms: 100
  tmux_width: 200
  tmux_height: 50

# Branch naming
branch:
  prefix: claudio
  include_id: true

# Pull request settings
pr:
  draft: false
  auto_rebase: true
  use_ai: true
  auto_pr_on_stop: false
  labels:
    - ai-generated
  reviewers:
    default: []
    by_path:
      "src/api/**": [backend-team]
      "src/frontend/**": [frontend-team]

# File paths
paths:
  worktree_dir: ""  # Default: .claudio/worktrees

# Cleanup behavior
cleanup:
  warn_on_stale: true
  keep_remote_branches: true

# Resource limits
resources:
  cost_warning_threshold: 5.00
  cost_limit: 0
  token_limit_per_instance: 0
  show_metrics_in_sidebar: true

# Ultra-plan settings
ultraplan:
  max_parallel: 3
  notifications:
    enabled: true
    use_sound: false

# Experimental features (disabled by default)
experimental:
  intelligent_naming: false
  triple_shot: false
  inline_plan: false
  inline_ultraplan: false
  grouped_instance_view: false
```

---

## Validation

View and validate your configuration:

```bash
# Show current config (validates syntax)
claudio config show

# Check config file path
claudio config path
```

If configuration is invalid, Claudio falls back to defaults and shows an error.

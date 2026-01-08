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

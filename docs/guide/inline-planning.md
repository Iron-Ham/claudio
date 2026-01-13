# Inline Planning (Experimental)

Inline planning brings the power of Plan and UltraPlan workflows directly into the standard Claudio TUI. Instead of running a separate command, you can create plans, organize tasks into groups, and execute them all from within your normal session.

> **Note:** This feature is experimental. Enable it via the `experimental.inline_plan` and `experimental.inline_ultraplan` configuration options.

## Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                      INLINE PLANNING WORKFLOW                        │
├─────────────────────────────────────────────────────────────────────┤
│  1. Start a normal Claudio session                                   │
│  2. Run :plan or :ultraplan with an objective                        │
│  3. Claude generates a structured task plan                          │
│  4. Review and edit the plan in the built-in editor                  │
│  5. Confirm to spawn instances organized in groups                   │
│  6. Monitor progress with visual group hierarchy                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Enabling Inline Planning

Add these options to your config file (`~/.config/claudio/config.yaml`):

```yaml
experimental:
  # Enable :plan command in TUI
  inline_plan: true

  # Enable :ultraplan command in TUI
  inline_ultraplan: true

  # Enable visual group organization in sidebar
  grouped_instance_view: true
```

Or set via environment variables:

```bash
export CLAUDIO_EXPERIMENTAL_INLINE_PLAN=true
export CLAUDIO_EXPERIMENTAL_INLINE_ULTRAPLAN=true
export CLAUDIO_EXPERIMENTAL_GROUPED_INSTANCE_VIEW=true
```

## Quick Start

```bash
# Start a normal session
claudio start my-feature

# In the TUI, enter command mode and run:
:plan "Implement user authentication with OAuth2"

# Or for multi-agent planning:
:ultraplan "Refactor the API layer to use dependency injection"
```

## Commands

### :plan

Creates a simple planning workflow where Claude analyzes your objective and generates a task plan.

```
:plan "Your objective here"
:plan --file path/to/existing-plan.json
```

**Options:**
- No arguments: Prompts for an objective
- `"objective"`: Starts planning with the given objective
- `--file`: Loads an existing plan file

### :ultraplan

Creates an UltraPlan workflow with a coordinator that manages parallel task execution.

```
:ultraplan "Your objective here"
:ultraplan --multi-pass "Complex refactoring task"
:ultraplan --file path/to/plan.json
```

**Options:**
- `--multi-pass`: Uses three parallel planning strategies, then selects the best plan
- `--file`: Loads an existing plan file and skips planning phase

### :group

Manages task groups manually. Use this to organize instances after they're created.

```
:group create [name]           # Create a new empty group
:group add [instance] [group]  # Add instance to a group
:group remove [instance]       # Remove instance from its group
:group move [instance] [group] # Move instance between groups
:group order [g1,g2,g3]        # Reorder execution sequence
:group delete [name]           # Delete an empty group
:group show                    # Toggle grouped view on/off
```

## The Plan Editor

When a plan is generated, you enter the plan editor to review and modify tasks before execution.

### Plan Editor Keys

| Key | Action |
|-----|--------|
| `j` / `k` | Move between tasks |
| `J` / `K` | Reorder tasks (move up/down) |
| `Enter` | Edit selected task |
| `n` | Add new task after current |
| `D` | Delete task (confirm for started instances) |
| `e` | Edit task dependencies |
| `Esc` | Exit plan editor |
| `Enter` (with no task selected) | Confirm plan and start execution |

### Plan Validation

The editor validates your plan in real-time:
- Circular dependencies are highlighted
- Missing dependency references are flagged
- Empty task descriptions are warned

## Group Navigation

When grouped view is enabled, the sidebar shows instances organized by their execution groups.

### Visual Group Notation

```
┌─────────────────────────────────────────┐
│ ▼ Group 1: Setup (2/3 complete)         │
│   ├─ ✓ [abc12] Add dependencies         │
│   ├─ ▶ [def34] Create user model        │
│   └─ ⏳ [ghi56] Configure database      │
│                                          │
│ ▶ Group 2: Implementation (blocked)      │  ← Collapsed
│   └─ ○ 3 tasks                          │
│                                          │
│ └─┬ Subgroup: Tests (depends on G2)     │
│   ├─ ○ [jkl78] Unit tests               │
│   └─ ○ [mno90] Integration tests        │
└─────────────────────────────────────────┘
```

**Symbols:**
- `▼` / `▶` - Expanded / Collapsed group
- `✓` - Completed task
- `▶` - Running task
- `⏳` - Waiting for input
- `○` - Pending task
- `└─┬` - Subgroup (dependency chain)

### Group Keyboard Shortcuts

All group shortcuts use a `g` prefix (vim-style):

| Key | Action |
|-----|--------|
| `gc` | Collapse/expand current group |
| `gC` | Collapse/expand all groups |
| `gn` | Jump to next group |
| `gp` | Jump to previous group |
| `gs` | Skip current group (mark pending as skipped) |
| `gr` | Retry failed tasks in current group |
| `gf` | Force-start next group (ignore dependencies) |

## Workflow Examples

### Example 1: Feature Implementation

```bash
# Start session
claudio start auth-feature

# Create a plan
:plan "Add user authentication with email/password login and session management"

# Claude generates:
# - Task 1: Add authentication dependencies (priority 1)
# - Task 2: Create User model (priority 1, parallel with 1)
# - Task 3: Implement login endpoint (depends on 1, 2)
# - Task 4: Add session middleware (depends on 3)
# - Task 5: Write tests (depends on 3, 4)

# Review plan, optionally edit tasks
# Press Enter to confirm and start execution

# Monitor progress with grouped view
# Tasks in same priority run in parallel
```

### Example 2: Large Refactoring with UltraPlan

```bash
claudio start api-refactor

# Use ultraplan with multi-pass for complex tasks
:ultraplan --multi-pass "Refactor API layer to use repository pattern"

# Three planning strategies run in parallel:
# - maximize-parallelism: Maximum parallel execution
# - minimize-complexity: Simpler, sequential tasks
# - balanced-approach: Mix of both

# Coordinator selects the best plan
# Review and confirm in plan editor

# Execution proceeds with dependency management
# Use gn/gp to navigate between groups
# Use gr to retry failed tasks
```

### Example 3: Loading an Existing Plan

```bash
# Load a pre-made plan file
:ultraplan --file .claudio/saved-plan.json

# Skip planning phase, go directly to review
# Edit tasks as needed
# Confirm to start execution
```

## Plan File Format

Plans use JSON format compatible with the `claudio plan` command:

```json
{
  "id": "auth-implementation",
  "objective": "Implement user authentication",
  "summary": "Add OAuth2-based auth with JWT tokens",
  "tasks": [
    {
      "id": "task-1",
      "title": "Add auth dependencies",
      "description": "Add OAuth2 and JWT libraries to package.json",
      "files": ["package.json"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "low"
    },
    {
      "id": "task-2",
      "title": "Create user model",
      "description": "Create User model with auth fields",
      "files": ["src/models/user.ts"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "medium"
    },
    {
      "id": "task-3",
      "title": "Implement auth middleware",
      "description": "Create JWT verification middleware",
      "files": ["src/middleware/auth.ts"],
      "depends_on": ["task-1", "task-2"],
      "priority": 2,
      "est_complexity": "medium"
    }
  ],
  "insights": ["Uses existing express framework"],
  "constraints": ["Must support token refresh"]
}
```

## Best Practices

### When to Use Inline Planning

**Good fit:**
- You want to stay in the TUI for the entire workflow
- The task benefits from visual group organization
- You need to manually adjust groups or dependencies
- You're iterating on a plan with quick edits

**Consider CLI commands instead:**
- You want to save a plan for reuse (`claudio plan --dry-run`)
- You're automating in CI/CD pipelines
- You need detailed plan output in a specific format

### Tips for Effective Plans

1. **Write clear objectives**: Be specific about the desired outcome
2. **Keep tasks independent**: Minimize dependencies where possible
3. **Group related work**: Tasks modifying the same area should be grouped
4. **Use priorities**: Lower priority numbers execute first
5. **Review before executing**: The plan editor catches issues early

### Handling Failures

- `gs` (skip group): Use when a group's tasks are no longer needed
- `gr` (retry group): Use to restart failed tasks after fixing issues
- `gf` (force start): Use carefully to bypass dependency blocks

## Troubleshooting

### Plan not generating

- Check that the planning instance has output
- Look for `<plan>...</plan>` markers in output
- Try pressing `p` to manually parse the plan

### Groups not displaying

- Ensure `experimental.grouped_instance_view` is enabled
- Toggle with `:group show` command
- Check that instances have group assignments

### Tasks not starting

- Verify dependencies are met (check group status)
- Use `gf` to force-start if blocked incorrectly
- Check instance status for errors

## Related Documentation

- [Ultra-Plan Mode](ultra-plan.md) - Full ultraplan CLI reference
- [TUI Navigation](tui-navigation.md) - Complete keyboard shortcuts
- [Configuration](configuration.md) - All config options

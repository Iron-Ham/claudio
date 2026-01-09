# Ultra-Plan Mode

Ultra-plan mode enables intelligent orchestration of parallel Claude sessions through a hierarchical planning approach. A "coordinator" session analyzes your objective, creates an execution plan, and manages parallel task execution.

## Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                        ULTRA-PLAN MODE                            │
├──────────────────────────────────────────────────────────────────┤
│  Phase 1: PLANNING                                                │
│  • Claude explores the codebase                                   │
│  • Generates structured execution plan                            │
│  • Identifies parallel vs sequential dependencies                 │
│                                                                   │
│  Phase 2: CONTEXT REFRESH                                         │
│  • Review and approve the generated plan                          │
│  • Plan is prepared for execution                                 │
│                                                                   │
│  Phase 3: PARALLEL EXECUTION                                      │
│  • Child sessions execute tasks in parallel                       │
│  • Dependencies are respected                                     │
│  • Progress is tracked in real-time                               │
│                                                                   │
│  Phase 4: SYNTHESIS                                               │
│  • Coordinator reviews all child outputs                          │
│  • Creates summary of all changes                                 │
│  • Identifies any integration issues                              │
└──────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Start ultra-plan with an objective
claudio ultraplan "Implement user authentication with OAuth2 support"

# The TUI will show planning progress
# Press [p] when planning completes to parse the plan
# Press [e] to start execution
```

## CLI Options

```bash
claudio ultraplan [objective] [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--max-parallel` | Maximum concurrent child sessions (0 = unlimited) | 3 |
| `--plan` | Use existing plan file instead of planning phase | - |
| `--dry-run` | Run planning only, output plan without executing | false |
| `--no-synthesis` | Skip synthesis phase after execution | false |
| `--auto-approve` | Auto-approve spawned tasks without confirmation | false |
| `--multi-pass` | Use multi-pass planning with 3 strategies, then select best | false |

### Examples

```bash
# Basic usage
claudio ultraplan "Refactor the API layer to use dependency injection"

# Increase parallelism for independent tasks
claudio ultraplan --max-parallel 5 "Add comprehensive test coverage"

# Dry run to review the plan first
claudio ultraplan --dry-run "Implement caching layer"

# Use a pre-made plan file
claudio ultraplan --plan my-plan.json

# Skip synthesis if you want to review changes manually
claudio ultraplan --no-synthesis "Update all deprecated API calls"

# Use multi-pass planning for complex tasks
claudio ultraplan --multi-pass "Redesign the authentication system"

# Combine multi-pass with dry-run to compare strategies
claudio ultraplan --multi-pass --dry-run "Implement caching layer"
```

## TUI Interface

### Ultra-Plan Header

The header shows:
- Current phase (Planning, Refresh, Executing, Synthesis, Complete, Failed)
- Progress bar during execution
- Task completion statistics

### Ultra-Plan Sidebar

During execution, the sidebar displays:
- Task groups (parallelizable clusters)
- Individual task status (pending, running, completed, failed)
- Instance assignments

### Key Bindings

| Key | Action | When |
|-----|--------|------|
| `v` | Toggle plan view | After plan is available |
| `p` | Parse plan from output | During planning phase |
| `e` | Start execution | After plan is ready |
| `c` | Cancel execution | During execution |
| `q` | Quit | Any time |

## Understanding the Plan

### Plan Structure

A plan consists of:

```json
{
  "objective": "Original user request",
  "summary": "Brief description of the approach",
  "tasks": [...],
  "insights": ["Key finding 1", "Key finding 2"],
  "constraints": ["Risk or constraint 1"]
}
```

### Task Definition

Each task in the plan includes:

| Field | Description |
|-------|-------------|
| `id` | Unique task identifier |
| `title` | Short descriptive title |
| `description` | Detailed instructions for execution |
| `files` | Expected files to be modified |
| `depends_on` | Task IDs this task depends on |
| `priority` | Execution priority (lower = earlier) |
| `est_complexity` | Estimated complexity (low/medium/high) |

### Execution Order

Tasks are organized into groups that can run in parallel:

```
Group 1 (parallel):  task-1, task-2, task-3
                         │
                         ▼
Group 2 (parallel):  task-4, task-5  (depends on group 1)
                         │
                         ▼
Group 3 (sequential): task-6  (depends on group 2)
```

## Using Plan Files

### Creating a Plan File

You can create a plan file manually for repeatable workflows:

```json
{
  "id": "auth-implementation",
  "objective": "Implement user authentication",
  "summary": "Add OAuth2-based authentication with JWT tokens",
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
      "description": "Create User model with authentication fields",
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
  "insights": [],
  "constraints": []
}
```

### Using a Plan File

```bash
claudio ultraplan --plan auth-plan.json
```

This skips the planning phase and goes directly to the review/execution phase.

## Session Recovery

Ultra-plan sessions are persisted and can be recovered:

```bash
# List sessions (shows ultra-plan status if present)
claudio sessions list

# Recover an ultra-plan session
claudio sessions recover
```

The recovered session will:
- Restore the current phase
- Show completed and pending tasks
- Allow continuing execution from where it stopped

## Multi-Pass Planning

Multi-pass planning is an advanced mode that improves plan quality by generating multiple plans in parallel using different strategies, then selecting or merging the best approach.

### How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                    MULTI-PASS PLANNING                          │
├─────────────────────────────────────────────────────────────────┤
│  Phase 1: PARALLEL STRATEGY GENERATION                          │
│                                                                  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │
│  │   Balanced   │ │  Depth-First │ │ Breadth-First│            │
│  │   Strategy   │ │   Strategy   │ │   Strategy   │            │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘            │
│         │                │                │                     │
│         └────────────────┼────────────────┘                     │
│                          ▼                                      │
│  Phase 2: PLAN SELECTION                                        │
│  • Coordinator-manager evaluates all plans                      │
│  • Selects best plan or merges strategies                       │
│  • Proceeds with chosen plan                                    │
└─────────────────────────────────────────────────────────────────┘
```

### The Three Strategies

| Strategy | Focus | Best For |
|----------|-------|----------|
| **maximize-parallelism** | Maximum parallel execution with minimal dependencies | Large refactors with isolated changes |
| **minimize-complexity** | Simplicity and clarity with single-responsibility tasks | Tasks with clear critical paths |
| **balanced-approach** | Balance parallelism, complexity, and dependencies | General-purpose tasks |

Each strategy coordinator explores the codebase and generates a complete plan independently. This parallel exploration often surfaces different insights about the task.

### When to Use Multi-Pass

**Ideal scenarios:**
- Complex architectural changes with multiple valid approaches
- Tasks where optimal decomposition is unclear
- Large codebases where different exploration paths yield different insights
- High-stakes changes where plan quality is critical

**May not need multi-pass:**
- Simple, well-defined tasks
- Tasks with obvious decomposition
- Time-sensitive work (multi-pass adds planning overhead)

### Example Usage

```bash
# Basic multi-pass planning
claudio ultraplan --multi-pass "Refactor the data layer to use repository pattern"

# Preview plans without executing
claudio ultraplan --multi-pass --dry-run "Implement event sourcing"

# Multi-pass with controlled parallelism during execution
claudio ultraplan --multi-pass --max-parallel 4 "Add comprehensive API tests"
```

## Best Practices

### Writing Good Objectives

Be specific about what you want to achieve:

```bash
# Good: Clear scope and outcome
claudio ultraplan "Add user authentication with email/password login,
  password reset flow, and session management using JWT tokens"

# Avoid: Too vague
claudio ultraplan "Make the app secure"
```

### Task Decomposition Tips

Ultra-plan works best when:

1. **Tasks are independent** - Minimize dependencies between tasks
2. **Files don't overlap** - Each task should modify different files
3. **Scope is clear** - Each task has a well-defined outcome

### Handling Failures

If a task fails:
- The phase changes to `failed`
- Other running tasks continue to completion
- You can review the error in the task output
- Consider using `claudio sessions recover` to retry

### When to Use Ultra-Plan

**Good fit:**
- Large refactoring across many files
- Implementing features with independent components
- Adding comprehensive test coverage
- Migrating to new patterns/libraries

**May not need ultra-plan:**
- Single-file changes
- Simple bug fixes
- Tasks with heavy interdependencies

### When to Use Multi-Pass

Consider using `--multi-pass` when:

1. **Decomposition is uncertain** - You're not sure how to best break down the task
2. **Multiple valid approaches** - The task could be solved different ways
3. **High complexity** - Architectural changes or major refactors
4. **Quality matters more than speed** - The extra planning time is worthwhile

Skip multi-pass for:
- Well-understood tasks with obvious structure
- Time-critical work where planning speed matters
- Simple feature additions or bug fixes

## Phases Reference

| Phase | Description |
|-------|-------------|
| `planning` | Coordinator is analyzing and creating plan |
| `plan_selection` | Multi-pass only: evaluating plans and selecting best approach |
| `context_refresh` | Plan ready for review/approval |
| `executing` | Child instances running tasks |
| `synthesis` | Reviewing and integrating results |
| `complete` | All phases finished successfully |
| `failed` | An error occurred or cancelled |

## Troubleshooting

### Plan Parsing Fails

If pressing `p` fails to parse the plan:
- Wait for the planning instance to complete
- Check the output for the `<plan>...</plan>` JSON block
- Ensure the JSON is valid

### Tasks Not Starting

If tasks don't start after pressing `e`:
- Verify you're in the `context_refresh` phase
- Check that a valid plan exists
- Look for error messages in the status bar

### Session Recovery Issues

If recovery doesn't restore ultra-plan state:
- Check `.claudio/session.json` exists
- Verify it contains the `ultra_plan` field
- Try `claudio sessions list` to see session status

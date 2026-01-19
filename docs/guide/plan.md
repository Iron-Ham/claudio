# Plan Mode

Plan mode generates structured execution plans from high-level objectives. It uses Claude to analyze your codebase and decompose a task into smaller, parallelizable subtasks that can be executed by [Ultra-Plan](ultra-plan.md) or tracked as GitHub Issues.

## Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                          PLAN MODE                                │
├──────────────────────────────────────────────────────────────────┤
│  INPUT: High-level objective                                      │
│  "Implement user authentication with OAuth2"                      │
│                           │                                       │
│                           ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                   CODEBASE ANALYSIS                         │  │
│  │  • Claude explores existing architecture                    │  │
│  │  • Identifies relevant files and patterns                   │  │
│  │  • Understands dependencies and constraints                 │  │
│  └────────────────────────────────────────────────────────────┘  │
│                           │                                       │
│                           ▼                                       │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                   TASK DECOMPOSITION                        │  │
│  │  • Breaks objective into discrete tasks                     │  │
│  │  • Identifies task dependencies                             │  │
│  │  • Assigns file ownership to minimize conflicts             │  │
│  │  • Estimates complexity                                     │  │
│  └────────────────────────────────────────────────────────────┘  │
│                           │                                       │
│                           ▼                                       │
│  OUTPUT: Structured plan (JSON, GitHub Issues, or both)          │
└──────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Basic planning - creates GitHub issues by default
claudio plan "Add user authentication"

# Preview plan without creating output (dry run)
claudio plan --dry-run "Add user authentication"

# Save plan as JSON file for use with ultraplan
claudio plan --output-format json "Build caching layer"

# Create both JSON file and GitHub issues
claudio plan --output-format both "Refactor database layer"
```

## CLI Options

```bash
claudio plan [objective] [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--dry-run` | Show plan without creating output | false |
| `--output-format` | Output format: `json`, `issues`, or `both` | `issues` |
| `--multi-pass` | Use 3 strategies, select best plan | false |
| `--no-confirm` | Skip confirmation prompt | false |
| `--labels` | Comma-separated labels for GitHub issues | - |
| `--output` | Output file path for JSON format | `.claudio-plan.json` |

### Examples

```bash
# Interactive: prompts for objective
claudio plan

# With objective argument
claudio plan "Implement caching with Redis"

# Dry run to review decomposition
claudio plan --dry-run "Add comprehensive tests"

# Multi-pass planning for complex tasks
claudio plan --multi-pass "Redesign the API layer"

# Create GitHub issues with labels
claudio plan --labels "enhancement,v2" "Add user profiles"

# Save to custom file
claudio plan --output my-feature-plan.json "Build reporting"

# Create both JSON and issues
claudio plan --output-format both "Implement webhooks"
```

## Understanding the Plan Structure

A generated plan contains the following structure:

```json
{
  "id": "plan-abc123",
  "objective": "Implement user authentication with OAuth2",
  "summary": "Add OAuth2-based authentication using JWT tokens...",
  "tasks": [
    {
      "id": "task-1-deps",
      "title": "Add authentication dependencies",
      "description": "Add OAuth2 and JWT libraries to package.json...",
      "files": ["package.json"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "low"
    },
    {
      "id": "task-2-models",
      "title": "Create user model",
      "description": "Create User model with authentication fields...",
      "files": ["src/models/user.ts"],
      "depends_on": [],
      "priority": 1,
      "est_complexity": "medium"
    },
    {
      "id": "task-3-middleware",
      "title": "Implement auth middleware",
      "description": "Create JWT verification middleware...",
      "files": ["src/middleware/auth.ts"],
      "depends_on": ["task-1-deps", "task-2-models"],
      "priority": 2,
      "est_complexity": "medium"
    }
  ],
  "dependency_graph": {
    "task-1-deps": [],
    "task-2-models": [],
    "task-3-middleware": ["task-1-deps", "task-2-models"]
  },
  "execution_order": [
    ["task-1-deps", "task-2-models"],
    ["task-3-middleware"]
  ],
  "insights": [
    "Existing codebase uses Express.js middleware pattern",
    "User model should extend existing BaseModel"
  ],
  "constraints": [
    "Must maintain backward compatibility with existing sessions"
  ]
}
```

### Task Fields

| Field | Description |
|-------|-------------|
| `id` | Unique identifier for the task |
| `title` | Short descriptive title |
| `description` | Detailed instructions for execution |
| `files` | Files this task will modify |
| `depends_on` | Task IDs that must complete first |
| `priority` | Execution order priority (lower = earlier) |
| `est_complexity` | Complexity estimate: `low`, `medium`, `high` |

### Execution Order

Tasks are grouped into parallelizable clusters based on dependencies:

```
Group 1 (parallel):   task-1-deps   task-2-models
                           │              │
                           └──────┬───────┘
                                  ▼
Group 2 (depends on G1): task-3-middleware
                                  │
                                  ▼
Group 3 (depends on G2): task-4-routes    task-5-tests
```

Tasks within the same group have no dependencies on each other and can run in parallel. Tasks in subsequent groups wait for all dependencies to complete.

## Output Formats

### JSON Output (`--output-format json`)

Creates a `.claudio-plan.json` file (or custom path with `--output`) that can be:

- Used with `claudio ultraplan --plan <file>` for execution
- Manually edited before execution
- Stored in version control for repeatable workflows
- Used as input for custom tooling

### GitHub Issues Output (`--output-format issues`)

Creates a hierarchical structure of GitHub issues:

1. **Parent Epic Issue** - Contains the full plan summary
2. **Child Task Issues** - One issue per task, linked to the parent
   - Includes task description
   - Links to dependent issues
   - Applies specified labels

This is useful for team coordination and tracking progress in GitHub's interface.

### Both (`--output-format both`)

Creates both the JSON file and GitHub issues, useful when you want to execute with ultraplan while also having visibility in GitHub.

## Multi-Pass Planning

Multi-pass planning generates three independent plans using different strategies, then selects or merges the best approach. See [Multi-Pass Planning](#multi-pass-planning-strategies) below.

### When to Use Multi-Pass

**Ideal for:**
- Complex architectural changes
- Tasks where decomposition is unclear
- High-stakes changes where quality matters
- Large codebases with multiple valid approaches

**Skip multi-pass for:**
- Well-defined tasks with obvious structure
- Simple feature additions
- Bug fixes with clear scope

### Multi-Pass Planning Strategies

Three coordinator instances run in parallel, each using a different strategy:

```
┌─────────────────────────────────────────────────────────────────┐
│                    MULTI-PASS PLANNING                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐ │
│  │ maximize-        │ │ minimize-        │ │ balanced-        │ │
│  │ parallelism      │ │ complexity       │ │ approach         │ │
│  │                  │ │                  │ │                  │ │
│  │ Focus: Max       │ │ Focus: Simple,   │ │ Focus: Pragmatic │ │
│  │ parallel tasks,  │ │ clear tasks with │ │ balance between  │ │
│  │ minimal deps     │ │ single purpose   │ │ all factors      │ │
│  └────────┬─────────┘ └────────┬─────────┘ └────────┬─────────┘ │
│           │                    │                    │            │
│           └────────────────────┼────────────────────┘            │
│                                ▼                                 │
│                     Plan Selection/Merge                         │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

#### Strategy 1: Maximize Parallelism

- Minimizes inter-task dependencies
- Prefers smaller, independent tasks
- Isolates file ownership (each file to one task)
- Flattens dependency graph

Best for large refactors with isolated changes.

#### Strategy 2: Minimize Complexity

- Single responsibility per task
- Clear task boundaries (inputs/outputs)
- Follows natural code structure
- Explicit over implicit (even if reduces parallelism)

Best for tasks with clear critical paths.

#### Strategy 3: Balanced Approach

- Respects existing architecture
- Pragmatic dependencies (reflect genuine needs)
- Right-sized tasks (meaningful but focused)
- Groups related changes together

Best for general-purpose tasks.

## Integration with Ultra-Plan

Plans created with `claudio plan` can be executed with [Ultra-Plan](ultra-plan.md):

```bash
# Step 1: Create and review the plan
claudio plan --output-format json "Implement webhooks"

# Step 2: Review/edit .claudio-plan.json

# Step 3: Execute with ultraplan
claudio ultraplan --plan .claudio-plan.json
```

This workflow allows you to:
1. Generate a plan
2. Review and adjust tasks manually
3. Execute with full ultraplan orchestration

## Best Practices

### Writing Good Objectives

Be specific about what you want to achieve:

```bash
# Good: Clear scope and outcome
claudio plan "Add user authentication with email/password login,
  password reset flow, and session management using JWT tokens"

# Avoid: Too vague
claudio plan "Make the app secure"
```

### Task Decomposition Tips

Plans work best when:

1. **Tasks are independent** - Minimize dependencies for maximum parallelism
2. **Files don't overlap** - Each task modifies different files
3. **Scope is clear** - Each task has a well-defined outcome
4. **Complexity is balanced** - Avoid one huge task with many small ones

### Reviewing Plans

Always review generated plans before execution:

1. Check task descriptions are actionable
2. Verify dependencies make sense
3. Confirm file assignments don't overlap
4. Ensure no missing edge cases

## Troubleshooting

### Plan Generation Fails

If plan generation fails:
- Ensure the objective is specific enough
- Check that Claude Code is authenticated
- Try simplifying the objective into smaller parts

### GitHub Issue Creation Fails

If issue creation fails:
- Verify `gh` CLI is installed and authenticated
- Check repository permissions
- Ensure you have write access to create issues

### Plan Quality Issues

If the generated plan doesn't match expectations:
- Try `--multi-pass` for different perspectives
- Add more context to the objective
- Break complex objectives into multiple planning sessions

## See Also

- [Ultra-Plan Mode](ultra-plan.md) - Execute plans with parallel orchestration
- [Task Chaining](task-chaining.md) - Manual dependency management
- [CLI Reference](../reference/cli.md) - Complete command reference

# Task Chaining

Task chaining allows you to define dependencies between backend instances, ensuring tasks execute in the correct order while maximizing parallelism.

## Overview

By default, all Claudio instances run in parallel. Task chaining lets you specify that certain tasks should wait for others to complete first.

```
┌─────────────────────────────────────────────────────────────────┐
│                        TASK CHAINING                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────┐   ┌──────────┐                                    │
│  │ Task A   │   │ Task B   │   ← Run in parallel (no deps)      │
│  │ (setup)  │   │ (setup)  │                                    │
│  └────┬─────┘   └────┬─────┘                                    │
│       │              │                                          │
│       └──────┬───────┘                                          │
│              ▼                                                   │
│       ┌──────────┐                                              │
│       │ Task C   │   ← Waits for A and B to complete            │
│       │ (build)  │                                              │
│       └────┬─────┘                                              │
│            │                                                     │
│            ▼                                                     │
│       ┌──────────┐                                              │
│       │ Task D   │   ← Waits for C to complete                  │
│       │ (test)   │                                              │
│       └──────────┘                                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### CLI Usage

Use the `--depends-on` (or `-d`) flag when adding instances:

```bash
# Add independent setup tasks
claudio add "Create user model" --start
claudio add "Create API routes" --start

# Add a task that depends on both setup tasks
claudio add "Write integration tests" --depends-on "user model,API routes"

# Or use instance IDs
claudio add "Deploy to staging" -d "abc123,def456"
```

### TUI Usage

When adding an instance in the TUI (press `a`), you can specify dependencies in the task description or use the plan/ultraplan workflows which automatically manage dependencies.

## Specifying Dependencies

Dependencies can be specified in several ways:

### By Instance ID

```bash
claudio add "Run tests" --depends-on "abc123"
claudio add "Deploy" -d "abc123,def456,ghi789"
```

### By Task Name (Substring Match)

```bash
# Matches any task containing "user model"
claudio add "Test users" --depends-on "user model"

# Multiple dependencies by name
claudio add "Integration tests" -d "unit tests,setup database"
```

### Mixed References

```bash
claudio add "Final task" --depends-on "abc123,setup,def456,cleanup"
```

## How It Works

1. **Pending State**: Tasks with unmet dependencies start in `pending` state
2. **Monitoring**: Claudio monitors dependency completion in real-time
3. **Auto-Start**: When all dependencies complete, the task automatically starts
4. **Failure Handling**: If a dependency fails, dependent tasks remain pending

## Example Workflows

### Build Pipeline

```bash
# Phase 1: Parallel setup
claudio add "Install dependencies" --start
claudio add "Generate types" --start

# Phase 2: Build (depends on phase 1)
claudio add "Build frontend" -d "Install dependencies,Generate types"
claudio add "Build backend" -d "Install dependencies,Generate types"

# Phase 3: Test (depends on phase 2)
claudio add "Run unit tests" -d "Build frontend,Build backend"
claudio add "Run integration tests" -d "Build frontend,Build backend"

# Phase 4: Deploy (depends on all tests)
claudio add "Deploy to staging" -d "unit tests,integration tests"
```

### Feature Implementation

```bash
# Core feature work (parallel)
claudio add "Implement auth API" --start
claudio add "Create auth UI components" --start

# Tests depend on implementation
claudio add "Write auth tests" -d "auth API,auth UI"

# Documentation depends on everything
claudio add "Update auth documentation" -d "auth tests"
```

### Database Migration

```bash
# Schema changes first
claudio add "Create migration scripts" --start

# Data migration depends on schema
claudio add "Migrate existing data" -d "migration scripts"

# Validation depends on migration
claudio add "Validate data integrity" -d "Migrate existing data"

# Rollback plan in parallel with validation
claudio add "Create rollback procedures" -d "migration scripts"
```

## TUI Indicators

In the TUI sidebar, instances with dependencies show their status:

| Indicator | Meaning |
|-----------|---------|
| `○` | Pending - waiting for dependencies |
| `▶` | Working - dependencies met, executing |
| `⏳` | Waiting for input |
| `✓` | Completed |

Dependencies are shown in the instance details panel.

## Integration with Planning

Task chaining integrates seamlessly with Claudio's planning features:

### Plan Mode

When you use `claudio plan` or `:plan`, the generated plan includes dependency information. Tasks are automatically chained based on the plan's structure.

### UltraPlan Mode

UltraPlan organizes tasks into groups with automatic dependency management:

```bash
claudio ultraplan "Implement authentication system"
```

The coordinator creates a plan with:
- Parallel tasks where possible
- Sequential dependencies where required
- Group-based execution order

### Inline Planning

Using `:plan` or `:ultraplan` in the TUI creates instances with proper dependencies already configured.

## Best Practices

### 1. Design for Independence

Minimize dependencies where possible:

```bash
# Good: Independent tasks
claudio add "Add user model"
claudio add "Add auth middleware"  # Different files

# Avoid: Unnecessary dependencies
claudio add "Add auth middleware" -d "user model"  # If not actually needed
```

### 2. Keep Chains Short

Long dependency chains reduce parallelism:

```bash
# Avoid: A → B → C → D → E (sequential)

# Better:
#   A ─┬─ C ─┐
#   B ─┴─ D ─┴─ E  (more parallelism)
```

### 3. Name Tasks Clearly

Clear names make dependency matching easier:

```bash
# Good: Clear, unique names
claudio add "Setup: Install NPM dependencies"
claudio add "Setup: Configure database"
claudio add "Build: Compile TypeScript" -d "Setup:"

# Avoid: Vague names that might match incorrectly
claudio add "Do setup"
claudio add "More setup"
```

### 4. Handle Failures

If a dependency fails:
1. Fix the failing task
2. Restart it with `s` in TUI
3. Dependent tasks will auto-start once it completes

Or remove the dependency and handle manually:
```bash
claudio remove <failed-task-id>
claudio add "Fixed task" --start
```

## Troubleshooting

### Task stuck in pending

**Cause:** Dependencies haven't completed yet.

**Solution:**
1. Check dependency status with `claudio status`
2. Select dependencies in TUI to see their progress
3. Start dependencies if they're not running

### Wrong task matched by name

**Cause:** Substring matching found unintended task.

**Solution:** Use instance IDs instead:
```bash
claudio status  # Get exact IDs
claudio add "New task" -d "abc123,def456"
```

### Circular dependency detected

**Cause:** Task A depends on B, which depends on A.

**Solution:** Review your dependency graph and break the cycle.

## Related Documentation

- [Instance Management](instance-management.md) - Instance lifecycle and states
- [Ultra-Plan Mode](ultra-plan.md) - Automatic dependency-aware planning
- [Inline Planning](inline-planning.md) - TUI-integrated planning workflows
- [CLI Reference](../reference/cli.md) - `claudio add` command details

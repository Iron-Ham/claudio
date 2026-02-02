# Instance Management

Instances are the core unit of work in Claudio. Each instance is an AI backend process (Claude Code or Codex) working on a specific task in its own isolated environment.

## Instance Lifecycle

```
┌──────────┐     ┌─────────┐     ┌────────────────┐     ┌───────────┐
│ pending  │ ──▶ │ working │ ──▶ │ waiting_input  │ ──▶ │ completed │
└──────────┘     └─────────┘     └────────────────┘     └───────────┘
                      │                  │
                      ▼                  ▼
                 ┌─────────┐        ┌─────────┐
                 │ paused  │        │ stopped │
                 └─────────┘        └─────────┘
```

### States

| State | Description |
|-------|-------------|
| **pending** | Instance created but not yet started |
| **working** | The backend is actively processing |
| **waiting_input** | The backend is waiting for user input |
| **paused** | Instance manually paused |
| **completed** | Task finished successfully |
| **stopped** | Instance manually stopped |

## Creating Instances

### From the TUI

Press `a` and enter your task description:

```
Implement OAuth2 login flow
```

### From the CLI

```bash
claudio add "Implement OAuth2 login flow"
```

You can add multiple instances at once:

```bash
claudio add "Implement login endpoint"
claudio add "Add session management"
claudio add "Write authentication tests"
```

## Managing Instances

### Starting

Instances start automatically when created. To manually start a pending instance:
- **TUI**: Select the instance and press `s`

### Pausing and Resuming

Pause an instance to temporarily halt its work:
- **TUI**: Press `p` to toggle pause/resume

Pausing is useful when:
- You need to reduce system load
- You want to review work before it continues
- An instance needs to wait for another to finish

### Stopping

Stop an instance to end its work:
- **TUI**: Press `x`

When you stop an instance, you'll be prompted to:
1. Create a PR for its work
2. Keep the branch for later
3. Discard the changes

### Removing

Remove an instance entirely:

```bash
claudio remove <instance-id>

# Force remove (even with uncommitted changes)
claudio remove <instance-id> --force
```

## Viewing Instance Output

### In the TUI

- **Select**: Navigate with `Tab`/`Shift+Tab` or `h`/`l`
- **Scroll**: Use `j`/`k` or arrow keys
- **Jump to latest**: Press `G`
- **Search**: Press `/` and enter a pattern

### Output Filtering

The search (`/`) supports regex patterns:

```
error                  # Find all errors
API.*failed            # Regex pattern
```

Press `n`/`N` to navigate between matches.

## Instance Isolation

Each instance runs in complete isolation:

### Worktrees

Located at `.claudio/worktrees/<instance-id>/`:
```
.claudio/worktrees/
├── abc123/           # Instance 1's worktree
│   ├── src/
│   ├── package.json
│   └── ...
├── def456/           # Instance 2's worktree
│   └── ...
```

### Branches

Each instance gets its own branch:
- Default format: `claudio/<id>-<task-slug>`
- Example: `claudio/abc123-implement-oauth`

Branch format is configurable:
```yaml
# config.yaml
branch:
  prefix: "feature"      # Use "feature" instead of "claudio"
  include_id: false      # Remove ID from branch name
```

### Benefits of Isolation

1. **No conflicts** - Instances can modify the same files without interference
2. **Independent commits** - Each instance creates its own commit history
3. **Safe experimentation** - Bad changes don't affect other instances
4. **Easy cleanup** - Remove a worktree without affecting others

## Conflict Detection

Claudio detects when multiple instances modify the same files:

### Viewing Conflicts

Press `c` in the TUI to see the conflict view:

```
Conflicting Files:
─────────────────────────────────────────
src/auth.ts
  Modified by: Instance 1 (abc123), Instance 2 (def456)

src/config.ts
  Modified by: Instance 1 (abc123)
```

### Handling Conflicts

When conflicts are detected:

1. **Review** - Check what each instance changed
2. **Coordinate** - Let one instance finish first
3. **Merge** - Use Git to merge branches manually
4. **Rebase** - Use `pr --auto-rebase` to handle during PR creation

## Reconnecting to Instances

If you quit the TUI but instances are still running:

```bash
# List recoverable sessions
claudio sessions list

# Recover and reconnect
claudio sessions recover

# Or start fresh and let it detect running sessions
claudio start
```

## Instance Metrics

Each instance tracks resource usage:

| Metric | Description |
|--------|-------------|
| Tokens (Input) | Tokens sent to the backend (when reported) |
| Tokens (Output) | Tokens received from the backend (when reported) |
| Cost | Estimated API cost (Claude Code metrics only) |
| Duration | Time since instance started |

View metrics:
- **TUI**: Shown in sidebar (if enabled)
- **CLI**: `claudio stats`

## Best Practices

### Task Decomposition

Break large tasks into smaller, independent units:

```bash
# Good: Clear, isolated tasks
claudio add "Add user model and database migration"
claudio add "Implement user registration endpoint"
claudio add "Add email verification flow"

# Avoid: Overlapping or vague tasks
claudio add "Implement authentication"  # Too broad
```

### Minimize File Overlap

Design tasks to touch different parts of the codebase:

```bash
# Good: Different areas
claudio add "Add frontend login form"    # Frontend
claudio add "Implement auth middleware"  # Backend

# Risky: Same files
claudio add "Add validation to user form"
claudio add "Refactor user form layout"  # Both touch same component
```

### Monitor Progress

Keep an eye on instances:
- Watch for `waiting_input` state
- Check for errors in output
- Review diffs periodically with `d`

### Clean Up Promptly

After instances complete:
- Create PRs for good work
- Remove abandoned instances
- Run `claudio cleanup` periodically

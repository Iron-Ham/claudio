# CLI Reference

Complete reference for all Claudio commands.

## Global Options

```
-c, --config string   Config file path (default: ~/.config/claudio/config.yaml)
-h, --help            Help for command
```

## Commands

### claudio init

Initialize Claudio in the current git repository.

```bash
claudio init
```

Creates a `.claudio/` directory for session state and worktrees.

**Requirements:**
- Must be run in a git repository
- Directory must be writable

---

### claudio start

Start a new session and launch the TUI dashboard.

```bash
claudio start [session-name] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `session-name` | Optional name for the session |

**Flags:**
| Flag | Description |
|------|-------------|
| `--new` | Force start a new session, replacing any existing one |

**Examples:**
```bash
# Start with default name
claudio start

# Start with custom name
claudio start my-feature

# Force new session (discard existing)
claudio start --new
```

**Behavior:**
- If a previous session exists, prompts to recover or start fresh
- Launches interactive TUI dashboard
- Detects and offers to reconnect to running tmux sessions

---

### claudio add

Add a new Claude instance with a task.

```bash
claudio add [task description] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `task description` | Description of the task for Claude |

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--start` | `-s` | Automatically start the instance after adding |
| `--depends-on` | `-d` | Comma-separated list of instance IDs or task names this task depends on |

**Examples:**
```bash
# Add and auto-start
claudio add "Implement user authentication" --start

# Add without starting
claudio add "Write unit tests for auth module"

# Quote complex tasks
claudio add "Refactor the User model to include email validation and password hashing"

# Task chaining - add with dependencies
claudio add "Write unit tests" --depends-on "abc123"
claudio add "Deploy to staging" -d "tests,build"

# Multiple dependencies
claudio add "Integration tests" --depends-on "unit-tests,api-setup"
```

**Task Chaining:**

When you specify `--depends-on`, the instance will wait in `pending` state until all its dependencies have completed. This enables:

- Sequential execution where order matters
- Building complex workflows with parallel and serial phases
- Ensuring prerequisites are met before dependent tasks start

Dependencies can be specified by:
- Instance ID (e.g., `abc123`)
- Task name substring (e.g., `tests` matches "Write unit tests")
- Comma-separated list for multiple dependencies

---

### claudio status

Display current session status and all instances.

```bash
claudio status
```

**Output includes:**
- Session name and state
- List of all instances
- Instance status (working, paused, completed, etc.)
- Branch names
- Basic metrics

---

### claudio stop

Stop all running instances and optionally cleanup.

```bash
claudio stop [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Force stop without prompts |

**Behavior:**
- Prompts for what to do with each instance's work
- Options: create PR, keep branch, discard
- Use `--force` to skip prompts and stop immediately

---

### claudio pr

Create a GitHub pull request for an instance.

```bash
claudio pr [instance-id] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `instance-id` | ID of the instance (optional if only one exists) |

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--title` | `-t` | Override PR title |
| `--body` | `-b` | Override PR body |
| `--draft` | `-d` | Create as draft PR |
| `--reviewer` | `-r` | Add reviewers (repeatable) |
| `--label` | `-l` | Add labels (repeatable) |
| `--closes` | | Link issues to close |
| `--no-ai` | | Skip AI generation |
| `--no-rebase` | | Skip rebasing on main |
| `--no-push` | | Don't push before creating PR |

**Examples:**
```bash
# Create PR with AI-generated content
claudio pr abc123

# Create draft PR
claudio pr abc123 --draft

# Override title
claudio pr abc123 --title "feat: add user auth"

# Add reviewers and labels
claudio pr abc123 --reviewer teammate1 --reviewer teammate2 --label enhancement

# Link to issue
claudio pr abc123 --closes 42

# Skip AI generation
claudio pr abc123 --no-ai
```

---

### claudio config

View or modify configuration.

```bash
claudio config [command]
```

**Subcommands:**

#### claudio config (no args)
Opens interactive configuration UI.

#### claudio config show
Display current configuration non-interactively.
```bash
claudio config show
```

#### claudio config init
Create a default config file with comments.
```bash
claudio config init
```

#### claudio config set
Set a configuration value.
```bash
claudio config set <key> <value>
```

**Examples:**
```bash
claudio config set completion.default_action auto_pr
claudio config set branch.prefix "Iron-Ham"
claudio config set pr.draft true
```

#### claudio config edit
Open config file in `$EDITOR`.
```bash
claudio config edit
```

#### claudio config path
Show config file location.
```bash
claudio config path
```

#### claudio config reset
Reset configuration to defaults.
```bash
claudio config reset
```

---

### claudio cleanup

Clean up stale worktrees, branches, and tmux sessions.

```bash
claudio cleanup [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--dry-run` | | Show what would be cleaned without making changes |
| `--force` | `-f` | Skip confirmation prompt |
| `--worktrees` | | Clean up only worktrees |
| `--branches` | | Clean up only branches |
| `--tmux` | | Clean up only tmux sessions |

**Examples:**
```bash
# Preview cleanup
claudio cleanup --dry-run

# Clean everything
claudio cleanup --force

# Clean only worktrees
claudio cleanup --worktrees
```

**What gets cleaned:**
- Worktrees in `.claudio/worktrees/` with no active session
- Branches matching `<prefix>/*` not associated with active work
- Orphaned `claudio-*` tmux sessions

---

### claudio harvest

Review and commit work from completed instances.

```bash
claudio harvest [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--all` | Process all worktrees (commit + pr + cleanup) |
| `--commit` | Auto-commit all uncommitted changes |
| `--pr` | Create PRs for committed branches |
| `--cleanup` | Remove worktrees with no changes |

**Examples:**
```bash
# Interactive harvest
claudio harvest

# Full automation
claudio harvest --all

# Just commit completed work
claudio harvest --commit

# Commit and create PRs
claudio harvest --commit --pr
```

---

### claudio remove

Remove a specific instance and its worktree.

```bash
claudio remove <id> [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `id` | Instance ID to remove |

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Remove even with uncommitted changes |

**Examples:**
```bash
# Remove instance (prompts if uncommitted changes)
claudio remove abc123

# Force remove
claudio remove abc123 --force
```

---

### claudio stats

Display resource usage and cost statistics.

```bash
claudio stats [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |

**Output includes:**
- Total token usage (input/output)
- Estimated API costs
- Per-instance breakdown
- Budget limit status

**Examples:**
```bash
# Human-readable output
claudio stats

# JSON for scripting
claudio stats --json
```

---

### claudio sessions

Manage Claudio sessions.

```bash
claudio sessions [command]
```

**Subcommands:**

#### claudio sessions list
List recoverable sessions and orphaned tmux sessions.
```bash
claudio sessions list
```

#### claudio sessions recover
Recover a previous session.
```bash
claudio sessions recover
```

Reconnects to running tmux sessions and restores the TUI.

#### claudio sessions clean
Clean up stale session data.
```bash
claudio sessions clean
```

---

### claudio plan

Generate a structured task plan from an objective.

```bash
claudio plan [objective] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `objective` | The goal or feature to plan (optional, can be interactive) |

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--dry-run` | Show the plan without executing | `false` |
| `--output-format` | Output format: `json`, `issues`, or `both` | `issues` |
| `--multi-pass` | Use 3 independent strategies to generate plans | `false` |
| `--no-confirm` | Skip confirmation prompt | `false` |
| `--output` | Write plan JSON to a specific file path | `.claudio-plan.json` |
| `--labels` | Labels to add to GitHub Issues (comma-separated) | - |

**Examples:**
```bash
# Basic planning
claudio plan "Add user authentication"

# Dry run to review plan
claudio plan --dry-run "Refactor the API layer"

# Output as GitHub issues
claudio plan --output-format issues "Implement caching"

# Multi-pass planning for complex tasks
claudio plan --multi-pass "Redesign the data model"
```

**Output Formats:**
- `json` - Outputs a structured JSON plan file
- `issues` - Creates GitHub issues for each task
- `both` - Creates both JSON file and GitHub issues

See [Plan Mode Guide](../guide/plan.md) for detailed documentation.

---

### claudio ultraplan

Intelligent hierarchical planning with 4-phase parallel execution.

```bash
claudio ultraplan [objective] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `objective` | The goal or feature to implement |

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--plan` | Use existing plan file instead of planning phase | - |
| `--max-parallel` | Maximum concurrent child sessions (0 = unlimited) | 3 |
| `--dry-run` | Run planning only, output plan without executing | false |
| `--no-synthesis` | Skip synthesis phase after execution | false |
| `--auto-approve` | Auto-approve spawned tasks without confirmation | false |
| `--multi-pass` | Use 3 competing strategies, then select best | false |
| `--review` | Always open plan editor before execution | false |

**Examples:**
```bash
# Basic ultraplan
claudio ultraplan "Implement OAuth2 authentication"

# Increase parallelism
claudio ultraplan --max-parallel 5 "Add comprehensive tests"

# Dry run to review plan
claudio ultraplan --dry-run "Refactor to microservices"

# Use existing plan file
claudio ultraplan --plan my-plan.json

# Multi-pass for complex architecture
claudio ultraplan --multi-pass "Redesign the authentication system"

# Skip synthesis for manual review
claudio ultraplan --no-synthesis "Update deprecated APIs"
```

**Phases:**
1. **Planning** - Claude explores codebase and generates structured plan
2. **Context Refresh** - Review and approve the generated plan
3. **Execution** - Child instances execute tasks in parallel (respecting dependencies)
4. **Synthesis** - Coordinator reviews all outputs and identifies integration issues

See [Ultra-Plan Guide](../guide/ultra-plan.md) for detailed documentation.

---

### claudio tripleshot

Execute a task with 3 parallel attempts, then judge selects the best.

```bash
claudio tripleshot [task] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `task` | The task to execute with multiple attempts |

**Flags:**
| Flag | Description |
|------|-------------|
| `--auto-approve` | Auto-approve applying the winning solution |

**Examples:**
```bash
# Basic tripleshot
claudio tripleshot "Optimize the database query in users.go"

# With auto-approve to apply the winning solution automatically
claudio tripleshot --auto-approve "Implement caching layer"
```

**How it works:**
1. Three parallel instances work on the same task with variant instructions
2. A judge instance evaluates all three completions
3. The best solution is selected based on quality criteria
4. Optional revision phase if the judge identifies improvements

See [TripleShot Guide](../guide/tripleshot.md) for detailed documentation.

---

### claudio adversarial

Iterative implementation with reviewer feedback loop.

```bash
claudio adversarial [task] [flags]
```

**Arguments:**
| Argument | Description |
|----------|-------------|
| `task` | The task to implement with review cycles |

**Flags:**
| Flag | Description | Default |
|------|-------------|---------|
| `--max-iterations` | Maximum implement-review cycles (0 = unlimited) | 10 |
| `--min-passing-score` | Minimum score (1-10) required for approval | 8 |

**Examples:**
```bash
# Basic adversarial review
claudio adversarial "Implement user authentication with JWT"

# Limit review cycles
claudio adversarial --max-iterations 5 "Refactor the API layer"

# Strict quality requirements
claudio adversarial --min-passing-score 9 "Implement encryption module"

# Combined for critical code
claudio adversarial --max-iterations 5 --min-passing-score 9 "Implement auth tokens"
```

**How it works:**
1. An IMPLEMENTER instance works on the task
2. When ready, submits work for review
3. A REVIEWER instance examines the code and provides feedback
4. Loop continues until approved with passing score, or max iterations reached

See [Adversarial Review Guide](../guide/adversarial.md) for detailed documentation.

---

### claudio logs

View and filter session logs.

```bash
claudio logs [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--session` | `-s` | Session ID (default: most recent) |
| `--tail` | `-n` | Number of lines to show, 0 for all (default: 50) |
| `--follow` | `-f` | Follow log output in real-time |
| `--level` | | Minimum level to show (debug/info/warn/error) |
| `--since` | | Show logs since duration (e.g., 1h, 30m) |
| `--grep` | | Filter by regex pattern |

**Examples:**
```bash
# Show last 50 lines
claudio logs

# Show all logs
claudio logs -n 0

# Follow logs in real-time
claudio logs -f

# Filter by level and pattern
claudio logs --level warn --grep "conflict"

# Logs from specific session
claudio logs -s abc123 --since 1h
```

---

### claudio completion

Generate shell autocompletion scripts.

```bash
claudio completion [shell]
```

**Supported shells:**
- `bash`
- `zsh`
- `fish`
- `powershell`

**Examples:**
```bash
# Bash
claudio completion bash > /etc/bash_completion.d/claudio

# Zsh
claudio completion zsh > "${fpath[1]}/_claudio"

# Fish
claudio completion fish > ~/.config/fish/completions/claudio.fish
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 130 | Interrupted (Ctrl+C) |

## Environment Variables

All configuration options can be set via environment variables:

```bash
CLAUDIO_COMPLETION_DEFAULT_ACTION=auto_pr
CLAUDIO_BRANCH_PREFIX=feature
CLAUDIO_PR_DRAFT=true
```

See [Configuration Reference](configuration.md) for all options.

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

**Examples:**
```bash
# Add and auto-start
claudio add "Implement user authentication" --start

# Add without starting
claudio add "Write unit tests for auth module"

# Quote complex tasks
claudio add "Refactor the User model to include email validation and password hashing"
```

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

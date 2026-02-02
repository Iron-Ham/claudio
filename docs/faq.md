# FAQ

Frequently asked questions about Claudio.

## General

### What is Claudio?

Claudio is a CLI/TUI tool for orchestrating multiple AI backend instances (Claude Code or Codex) simultaneously on a single project. It uses Git worktrees to isolate each instance's work, enabling truly parallel AI-assisted development.

### How does it differ from running multiple backend terminals?

Claudio provides:
- **Automatic isolation** via Git worktrees (each instance gets its own working directory)
- **Coordination** through shared context files
- **Unified dashboard** to monitor all instances
- **Conflict detection** to catch overlapping changes
- **Integrated PR workflow** with AI-generated descriptions

### Does Claudio work with backend APIs directly?

No. Claudio orchestrates your configured backend CLI (Claude Code or Codex), which handles API communication. You need the backend CLI installed and authenticated separately.

### What platforms are supported?

Claudio works on:
- **macOS** (primary development platform)
- **Linux** (tested on Ubuntu, Debian, Fedora)
- **Windows** (via WSL2)

Native Windows support is not available due to tmux dependency.

---

## Instance Management

### How many instances can I run simultaneously?

There's no hard limit in Claudio itself. Practical limits depend on:
- **System resources**: Each instance runs a backend process
- **API rate limits**: Your backend API may have rate limits
- **Cost**: More instances = more tokens = higher cost

Most users run 3-5 instances effectively. Beyond 10, you may hit system or API limits.

### Can instances communicate with each other?

Not directly. However, Claudio generates a `context.md` file visible to all instances that shows:
- What each instance is working on
- Current status
- Files being modified

This helps the backend understand parallel work and avoid conflicts.

### What happens if I close the TUI while instances are running?

The instances continue running in tmux sessions. You can:

1. Recover the session:
   ```bash
   claudio sessions recover
   ```

2. Or list what's running:
   ```bash
   claudio sessions list
   ```

### Can I pause and resume instances?

Yes. Press `p` in the TUI to pause an instance. Press `p` again to resume. Paused instances retain their state and output.

---

## Branches and Worktrees

### What are Git worktrees?

A [Git worktree](https://git-scm.com/docs/git-worktree) is an additional working directory linked to your repository. Each worktree has its own checked-out branch but shares the Git history with the main repo.

Claudio creates worktrees in `.claudio/worktrees/<instance-id>/` so each backend instance can modify files independently.

### Can I customize branch names?

Yes, via configuration:

```yaml
branch:
  prefix: "feature"      # Instead of "claudio"
  include_id: false      # Remove instance ID from name
```

Result: `feature/fix-bug` instead of `claudio/abc123-fix-bug`

### Are worktrees deleted automatically?

No. Worktrees persist until you:
- Create a PR and cleanup
- Run `claudio cleanup`
- Manually remove them

This allows you to review work after instances complete.

### Can I access the worktree directly?

Yes:
```bash
cd .claudio/worktrees/<instance-id>
# Make manual edits, run tests, etc.
```

Changes you make there will be visible when you view the diff.

---

## Pull Requests

### How does AI-generated PR content work?

When you create a PR, Claudio:
1. Collects the diff and commit messages
2. Sends them to the configured backend
3. The backend generates a title and description
4. The PR is created via GitHub CLI (`gh`)

You can disable this with `--no-ai` or `pr.use_ai: false` in config.

### Can I customize PR templates?

Yes, using Go's `text/template` syntax:

```yaml
pr:
  template: |
    ## Summary
    {{.Summary}}

    ## Changes
    {{.Changes}}

    ## Test Plan
    {{.Testing}}
```

### How do I add reviewers automatically?

Configure reviewers in your config:

```yaml
pr:
  reviewers:
    default: [alice, bob]
    by_path:
      "src/api/**": [api-team]
      "*.md": [docs-team]
```

### Why did my PR fail to create?

Common causes:
- GitHub CLI (`gh`) not installed or not authenticated
- Branch not pushed to remote
- Rebase conflicts

Check `claudio pr <id>` output for specific errors. See [Troubleshooting](troubleshooting.md#pr-creation-issues).

---

## Costs and Resources

### How much does it cost?

Claudio itself is free. Costs come from your backend's API usage:
- Each instance uses tokens for input (code context) and output (responses)
- Running multiple instances multiplies your API usage
- Typical feature costs vary by backend and model

### How can I track and limit costs?

1. **Monitor**: `claudio stats` shows current usage when the backend reports token metrics (Claude Code output)

2. **Warning**: Set a threshold for warnings:
   ```yaml
   resources:
     cost_warning_threshold: 5.00
   ```

3. **Hard limit**: Auto-pause all instances at a limit:
   ```yaml
   resources:
     cost_limit: 20.00
   ```

### What happens when I hit the cost limit?

All instances are paused automatically. You can:
- Review work and create PRs
- Increase the limit
- Stop instances to prevent further costs

---

## Conflicts and Coordination

### What if two instances modify the same file?

Claudio detects this and shows it in the conflict view (press `c`). You have options:

1. **Let them continue** - Merge manually later
2. **Pause one** - Let the other finish first
3. **Create PRs in order** - Use `auto_rebase` to handle merges

### How do I avoid conflicts?

Design tasks to minimize overlap:

```bash
# Good - different files
claudio add "Add user model in src/models/"
claudio add "Add API routes in src/routes/"

# Risky - likely same files
claudio add "Add login feature"
claudio add "Add logout feature"
```

### Can the backend see what other instances are doing?

Yes, through the shared context file (`.claudio/context.md`). This is updated in real-time and shows:
- All active instances
- Their tasks and status
- Files they're modifying

---

## Troubleshooting

### Why is my instance stuck?

Common causes:
1. **Waiting for input** - Check if there's a prompt in the output
2. **The backend is thinking** - Complex tasks take time
3. **Process crashed** - Check tmux: `tmux list-sessions`

See [Instance Issues](troubleshooting.md#instance-issues) for solutions.

### Why can't I see instance output?

Try:
1. Press `G` to jump to latest
2. Press `j`/`k` to scroll
3. Check the instance is selected (sidebar shows `▶`)
4. Increase buffer size in config

### How do I reset everything?

```bash
claudio stop --force
claudio cleanup --force
rm -rf .claudio
git worktree prune
claudio init
```

---

## Integration

### Does Claudio work with GitHub Enterprise?

Yes, if your `gh` CLI is configured for your GitHub Enterprise instance:
```bash
gh auth login --hostname github.mycompany.com
```

### Can I use Claudio in CI/CD?

Yes, with some considerations:
- Disable interactive features
- Use `--force` flags for automated cleanup
- Set `auto_pr_on_stop: true` for automatic PR creation

Example:
```bash
claudio init
claudio add "Fix linting errors" --start
# Wait for completion...
claudio pr --no-ai --title "fix: resolve linting errors"
```

### Does Claudio support GitLab/Bitbucket?

PR creation currently uses GitHub CLI (`gh`). For other platforms:
- Instances work normally
- Create PRs manually or with your platform's CLI
- Or use `--no-push` and push manually

---

## Best Practices

### How should I write task descriptions?

Be specific and include context:

```bash
# Good
claudio add "Add email validation to User model in src/models/user.ts.
Should reject invalid formats and return helpful error messages."

# Less effective
claudio add "Fix user validation"
```

### When should I use Claudio vs. a single backend?

**Use Claudio when:**
- You have multiple independent tasks
- Tasks touch different parts of the codebase
- You want to parallelize development
- You're doing large refactors by module

**Use a single backend when:**
- You have one focused task
- Tasks are highly interdependent
- You need tight back-and-forth iteration

### How many instances should I run?

Start with 2-3 and scale up as needed. Consider:
- Task independence (more overlap = fewer instances)
- System resources
- Your ability to review parallel work
- Cost tolerance

---

## Still have questions?

- Check [Troubleshooting](troubleshooting.md) for specific issues
- Search [GitHub Issues](https://github.com/Iron-Ham/claudio/issues)
- Open a new issue if your question isn't answered

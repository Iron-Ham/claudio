# PR Creation

Claudio streamlines the process of creating pull requests from your backend instances' work.

## Overview

When an instance completes its task, you can create a PR with:
- AI-generated title and description
- Automatic rebasing on the target branch
- Reviewer assignment based on files changed
- Custom labels and templates

## Creating PRs

### From the TUI

1. Select the completed instance
2. Press `x` to stop
3. Choose "Create PR" when prompted

Or if `auto_pr_on_stop` is enabled, the PR is created automatically.

### From the CLI

```bash
# Create PR for specific instance
claudio pr <instance-id>

# Create PR with options
claudio pr <instance-id> --draft
claudio pr <instance-id> --no-ai
```

### Automatic PR Creation

Enable auto-PR on stop:

```yaml
# config.yaml
pr:
  auto_pr_on_stop: true
```

Now pressing `x` in the TUI automatically creates a PR.

## The PR Workflow

When you create a PR, Claudio:

1. **Checks for changes** - Verifies there are commits to push
2. **Rebases** (if enabled) - Updates branch with latest main
3. **Pushes** - Pushes the branch to remote
4. **Generates content** (if enabled) - Uses the configured backend to create title/description
5. **Creates PR** - Opens the PR via `gh` CLI
6. **Assigns reviewers** - Based on config and files changed
7. **Adds labels** - Applies configured labels

## AI-Generated Content

When `use_ai: true` (default), the backend generates:

### Title

A concise, descriptive title based on the changes:
```
feat: add OAuth2 authentication with Google provider
```

### Description

A structured summary including:
- What was changed
- Why it was changed
- How to test

Example:
```markdown
## Summary
Implements OAuth2 authentication flow using Google as the identity provider.

## Changes
- Added `src/auth/oauth.ts` - OAuth client configuration
- Added `src/routes/auth.ts` - Login/callback endpoints
- Updated `src/config.ts` - Added OAuth environment variables
- Added tests for auth flow

## Test Plan
1. Set GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET
2. Navigate to /auth/login
3. Complete Google sign-in
4. Verify redirect back to app with session
```

### Disabling AI

```bash
# Per-PR
claudio pr <id> --no-ai

# Globally
claudio config set pr.use_ai false
```

Without AI, you'll be prompted to enter the title and description manually.

## Rebasing

By default, Claudio rebases your branch on main before creating the PR.

### Why Rebase?

- Ensures your changes apply cleanly to latest main
- Reduces merge conflicts after PR creation
- Creates a cleaner commit history

### Handling Rebase Conflicts

If conflicts occur during rebase:

1. Claudio reports the conflict
2. Navigate to the worktree: `.claudio/worktrees/<id>/`
3. Resolve conflicts manually
4. Run `claudio pr <id>` again

### Disabling Auto-Rebase

```yaml
pr:
  auto_rebase: false
```

Or per-PR:
```bash
claudio pr <id> --no-rebase
```

## Reviewer Assignment

### Default Reviewers

Always assign certain reviewers:

```yaml
pr:
  reviewers:
    default:
      - tech-lead
      - team-member
```

### Path-Based Reviewers

Assign reviewers based on which files changed:

```yaml
pr:
  reviewers:
    by_path:
      "src/api/**": [backend-team]
      "src/frontend/**": [frontend-team]
      "*.sql": [dba]
      "Dockerfile": [devops]
      "**/security/**": [security-team]
```

### How Matching Works

1. Get list of files changed in PR
2. Match each file against glob patterns
3. Collect all matching reviewers
4. Deduplicate and add to PR

## Labels

Add labels to all PRs:

```yaml
pr:
  labels:
    - ai-generated
    - needs-review
    - automated
```

## Custom Templates

Use Go's `text/template` for custom PR bodies:

```yaml
pr:
  template: |
    ## Summary
    {{.Summary}}

    ## Task
    > {{.Task}}

    ## Files Changed
    {{.Changes}}

    ## Testing Instructions
    {{.Testing}}

    ---
    Branch: `{{.Branch}}`
```

### Available Variables

| Variable | Description |
|----------|-------------|
| `{{.Summary}}` | AI-generated summary of changes |
| `{{.Changes}}` | Formatted list of changed files |
| `{{.Testing}}` | AI-generated test plan |
| `{{.Branch}}` | Branch name |
| `{{.Task}}` | Original task description |

### Template Functions

Standard Go template functions are available:

```yaml
pr:
  template: |
    {{if .Summary}}
    ## Summary
    {{.Summary}}
    {{end}}

    {{range .Files}}
    - {{.}}
    {{end}}
```

## Draft PRs

Create PRs as drafts for work-in-progress:

```yaml
pr:
  draft: true
```

Or per-PR:
```bash
claudio pr <id> --draft
```

## CLI Options

```bash
claudio pr [instance-id] [flags]

Flags:
      --draft       Create as draft PR
      --no-ai       Skip AI-generated content
      --no-rebase   Skip rebasing on main
  -h, --help        Help for pr
```

## Troubleshooting

### "No changes to push"

The branch has no commits beyond main:
- Check if the backend made changes: `claudio status`
- View the diff: Press `d` in TUI
- The instance may not have completed its work

### "gh: command not found"

Install GitHub CLI:
```bash
# macOS
brew install gh

# Linux
sudo apt install gh

# Then authenticate
gh auth login
```

### "Rebase conflict"

1. Go to worktree: `cd .claudio/worktrees/<id>`
2. Check status: `git status`
3. Resolve conflicts in listed files
4. Complete rebase: `git rebase --continue`
5. Try PR again: `claudio pr <id>`

### "Authentication failed"

Ensure gh is authenticated:
```bash
gh auth status
gh auth login  # If needed
```

### "Branch already exists on remote"

The branch was previously pushed:
```bash
# Force push (careful!)
claudio pr <id> --force

# Or delete remote branch first
git push origin --delete <branch-name>
claudio pr <id>
```

## Best Practices

### Write Clear Tasks

Good task descriptions lead to better PRs:

```bash
# Good - specific and clear
claudio add "Add rate limiting to /api/users endpoint with 100 req/min limit"

# Less helpful - vague
claudio add "Fix API performance"
```

### Review Before PR

1. Check diff with `d` in TUI
2. Review AI-generated content
3. Edit if needed before creating PR

### Use Draft PRs for WIP

```yaml
pr:
  draft: true  # While iterating
```

Change to non-draft when ready for review.

### Coordinate Overlapping Work

If instances touched the same files:
1. Check conflict view with `c`
2. Create PRs in order
3. Each subsequent PR rebases on updated main

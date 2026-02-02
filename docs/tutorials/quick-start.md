# Quick Start: Your First Parallel Session

**Time: 5 minutes**

This tutorial walks you through running multiple AI backend instances in parallel on a real project.

## Prerequisites

- Claudio installed (`claudio --help` works)
- Preferred backend CLI authenticated (Claude Code or Codex)
- A Git repository to work in

## Step 1: Set Up Your Project

Navigate to a Git repository. For this tutorial, we'll use a simple example:

```bash
# Create a test project (or use your own)
mkdir claudio-demo
cd claudio-demo
git init

# Create some starter files
echo '# My App' > README.md
mkdir src
echo 'console.log("Hello");' > src/index.js
git add . && git commit -m "initial commit"
```

## Step 2: Initialize Claudio

```bash
claudio init
```

You should see:
```
Initialized Claudio in /path/to/claudio-demo
Created .claudio/ directory
```

## Step 3: Start a Session

```bash
claudio start demo
```

The TUI launches. You'll see an empty dashboard waiting for instances.

## Step 4: Add Your First Instance

Press `a` to add an instance. Enter:

```
Add a greeting function to src/index.js that takes a name parameter
```

Watch as Claudio:
1. Creates a worktree in `.claudio/worktrees/`
2. Creates a new branch
3. Starts the configured backend with your task

## Step 5: Add a Second Instance

While the first is working, press `a` again:

```
Add a README section explaining how to run the project
```

Now you have **two instances running in parallel**!

## Step 6: Monitor Progress

- Press `1` or `2` to switch between instances
- Use `j`/`k` to scroll through output
- Press `d` to see the diff of changes

Watch both instances work simultaneously.

## Step 7: Create a PR

When an instance shows `[completed]`:

1. Select it with `1` or `2`
2. Press `x` to stop
3. Choose "Create PR" (if you have `gh` configured)

Or just keep the branch for manual review.

## Step 8: Clean Up

Press `q` to quit. Choose to stop remaining instances.

Clean up resources:
```bash
claudio cleanup
```

## What You Learned

- **Initialize**: `claudio init` sets up the project
- **Start**: `claudio start <name>` launches the TUI
- **Add instances**: Press `a` to add parallel tasks
- **Navigate**: Use `Tab`/`Shift+Tab`, `j/k` to move around
- **View changes**: Press `d` for diffs
- **Create PRs**: Press `x` to stop and create PR
- **Cleanup**: `claudio cleanup` removes stale resources

## Next Steps

- [Feature Development Tutorial](feature-development.md) - Build a complete feature
- [Instance Management Guide](../guide/instance-management.md) - Deep dive into instances
- [Configuration Guide](../guide/configuration.md) - Customize Claudio

# Troubleshooting

Solutions to common issues when using Claudio.

## Installation Issues

### "go: command not found"

Go is not installed or not in your PATH.

**Solution:**
1. Install Go from [golang.org/dl](https://golang.org/dl/)
2. Ensure `$GOPATH/bin` is in your PATH:
   ```bash
   export PATH=$PATH:$(go env GOPATH)/bin
   ```
3. Add to your shell profile (`~/.bashrc`, `~/.zshrc`)

### "claudio: command not found" after installation

The binary isn't in your PATH.

**Solution:**
```bash
# Check if installed
ls $(go env GOPATH)/bin/claudio

# Add to PATH if needed
export PATH=$PATH:$(go env GOPATH)/bin

# Or install directly
go install github.com/Iron-Ham/claudio/cmd/claudio@latest
```

---

## Initialization Issues

### "not a git repository"

Claudio requires a Git repository.

**Solution:**
```bash
git init
git add .
git commit -m "initial commit"
claudio init
```

### "permission denied" when creating .claudio

Directory permissions issue.

**Solution:**
```bash
# Check permissions
ls -la .

# Fix if needed
chmod u+w .
```

---

## Session Issues

### "session already exists"

A previous session wasn't cleaned up.

**Solutions:**

1. Recover the existing session:
   ```bash
   claudio sessions recover
   ```

2. Or force a new session:
   ```bash
   claudio start --new
   ```

3. Or clean up manually:
   ```bash
   claudio sessions clean
   claudio start
   ```

### TUI not displaying correctly

Terminal compatibility issues.

**Solutions:**

1. Ensure terminal supports ANSI colors:
   ```bash
   echo $TERM  # Should be xterm-256color or similar
   ```

2. Try resizing your terminal window

3. Check minimum size (80x24 recommended)

4. Try a different terminal emulator

### Can't reconnect to session

tmux sessions may have died.

**Solution:**
```bash
# Check tmux sessions
tmux list-sessions | grep claudio

# If none exist, clean up and start fresh
claudio sessions clean
claudio start
```

---

## Instance Issues

### Instance stuck on "pending"

Instance didn't start properly.

**Solutions:**

1. Try starting manually:
   ```bash
   # In TUI, select instance and press 's'
   ```

2. Check tmux:
   ```bash
   tmux list-sessions | grep claudio
   ```

3. Remove and re-add:
   ```bash
   claudio remove <id>
   claudio add "your task"
   ```

### Instance shows "waiting_input" but no prompt

Output buffer may not have captured the prompt.

**Solutions:**

1. Press `G` to jump to latest output

2. Scroll up/down with `j`/`k` to find the prompt

3. Press `Enter` to focus input anyway and check

4. Increase buffer size in config:
   ```yaml
   instance:
     output_buffer_size: 200000
   ```

### Claude process not starting

Claude Code CLI may not be installed or authenticated.

**Solutions:**

1. Verify Claude Code is installed:
   ```bash
   claude --version
   ```

2. Check authentication:
   ```bash
   claude auth status
   ```

3. Re-authenticate if needed:
   ```bash
   claude auth login
   ```

### Instance immediately completes with no work

Task may have been misunderstood or files may not exist.

**Solutions:**

1. Check the output for Claude's response

2. Be more specific in task description:
   ```bash
   # Instead of:
   claudio add "fix the bug"

   # Try:
   claudio add "Fix the null pointer exception in src/api/users.ts line 42"
   ```

3. Verify target files exist in the repo

---

## Worktree Issues

### "fatal: worktree already exists"

A worktree with that name exists.

**Solutions:**

1. Clean up stale worktrees:
   ```bash
   claudio cleanup --worktrees
   ```

2. Or manually:
   ```bash
   git worktree list
   git worktree remove .claudio/worktrees/<id>
   git worktree prune
   ```

### "worktree path already exists but is not a worktree"

Directory exists but isn't a proper worktree.

**Solution:**
```bash
# Remove the problematic directory
rm -rf .claudio/worktrees/<id>

# Prune worktree references
git worktree prune

# Try again
claudio add "your task"
```

### Changes in worktree not visible

You may be looking at the wrong worktree or branch.

**Solutions:**

1. Verify you're in the right worktree:
   ```bash
   cd .claudio/worktrees/<instance-id>
   git status
   ```

2. Check which branch:
   ```bash
   git branch --show-current
   ```

3. Use `d` in TUI to see diff

---

## Git Issues

### "cannot lock ref"

Branch name conflict or lock file issue.

**Solutions:**

1. Clean up lock files:
   ```bash
   find .git -name "*.lock" -delete
   ```

2. If branch exists, cleanup may help:
   ```bash
   claudio cleanup --branches
   ```

### Merge conflicts during rebase

PR creation failed due to conflicts.

**Solutions:**

1. Navigate to the worktree:
   ```bash
   cd .claudio/worktrees/<id>
   ```

2. Check conflict status:
   ```bash
   git status
   ```

3. Resolve conflicts in listed files

4. Continue rebase:
   ```bash
   git add <resolved-files>
   git rebase --continue
   ```

5. Return and retry:
   ```bash
   cd ../../..
   claudio pr <id>
   ```

### Branch diverged from main

Your branch has diverged from the target branch.

**Solution:**
```bash
cd .claudio/worktrees/<id>
git fetch origin main
git rebase origin/main
# Resolve any conflicts
cd ../../..
claudio pr <id>
```

---

## tmux Issues

### "tmux: command not found"

tmux is not installed.

**Solutions:**

```bash
# macOS
brew install tmux

# Ubuntu/Debian
sudo apt install tmux

# Fedora
sudo dnf install tmux
```

### Orphaned tmux sessions

Sessions remain after Claudio exits.

**Solution:**
```bash
# List Claudio sessions
tmux list-sessions | grep claudio

# Kill all Claudio sessions
claudio cleanup --tmux

# Or manually
tmux kill-session -t claudio-abc123
```

### tmux output garbled

Terminal encoding issues.

**Solutions:**

1. Set UTF-8 encoding:
   ```bash
   export LANG=en_US.UTF-8
   export LC_ALL=en_US.UTF-8
   ```

2. Configure tmux:
   ```bash
   echo "set -g default-terminal 'screen-256color'" >> ~/.tmux.conf
   ```

---

## PR Creation Issues

### "gh: command not found"

GitHub CLI not installed.

**Solutions:**

```bash
# macOS
brew install gh

# Ubuntu/Debian
sudo apt install gh

# Then authenticate
gh auth login
```

### "authentication required"

GitHub CLI not authenticated.

**Solution:**
```bash
gh auth status
gh auth login
```

### "branch not found on remote"

Branch wasn't pushed.

**Solution:**
```bash
claudio pr <id>  # This should push automatically

# Or manually push
cd .claudio/worktrees/<id>
git push -u origin $(git branch --show-current)
```

### PR created but AI content missing

AI generation may have failed silently.

**Solutions:**

1. Check Claude Code is working:
   ```bash
   claude --version
   ```

2. Try without AI:
   ```bash
   claudio pr <id> --no-ai
   ```

3. Provide content manually:
   ```bash
   claudio pr <id> --title "feat: add feature" --body "Description here"
   ```

---

## Performance Issues

### High CPU usage

Many instances running simultaneously.

**Solutions:**

1. Pause some instances:
   - Press `p` on instances not actively needed

2. Reduce capture frequency:
   ```yaml
   instance:
     capture_interval_ms: 200  # Increase from default 100
   ```

3. Run fewer parallel instances

### High memory usage

Large output buffers or many instances.

**Solutions:**

1. Reduce buffer size:
   ```yaml
   instance:
     output_buffer_size: 50000  # Reduce from 100000
   ```

2. Reduce displayed lines:
   ```yaml
   tui:
     max_output_lines: 500  # Reduce from 1000
   ```

3. Stop completed instances promptly

### Slow TUI response

Too much output to render.

**Solutions:**

1. Reduce max output lines:
   ```yaml
   tui:
     max_output_lines: 500
   ```

2. Use search (`/`) to filter output

3. Jump to latest with `G` instead of scrolling

---

## Cost Issues

### Unexpected high costs

Many tokens used across instances.

**Solutions:**

1. Set cost limits:
   ```yaml
   resources:
     cost_warning_threshold: 5.00
     cost_limit: 20.00
   ```

2. Monitor with:
   ```bash
   claudio stats
   ```

3. Pause instances when not needed

4. Use more focused tasks to reduce token usage

---

## iOS/Xcode Issues

### xcodebuild fails in worktree

The worktree may be missing project files or have stale build state.

**Solutions:**

1. Verify workspace exists:
   ```bash
   ls .claudio/worktrees/<id>/*.xcworkspace
   ```

2. Clean and rebuild:
   ```bash
   cd .claudio/worktrees/<id>
   xcodebuild clean
   xcodebuild -scheme MyApp build
   ```

3. Remove DerivedData for this worktree:
   ```bash
   # Find DerivedData for this worktree path
   rm -rf ~/Library/Developer/Xcode/DerivedData/MyApp-*
   ```

### "Simulator in use" errors during parallel tests

Multiple instances are trying to use the same simulator.

**Solutions:**

1. Use different simulators per instance:
   ```bash
   # Task 1
   xcodebuild test -destination 'platform=iOS Simulator,name=iPhone 15'

   # Task 2
   xcodebuild test -destination 'platform=iOS Simulator,name=iPhone 15 Pro'
   ```

2. Clone simulators for parallel testing:
   ```bash
   xcrun simctl clone "iPhone 15" "iPhone 15 - Test 1"
   xcrun simctl clone "iPhone 15" "iPhone 15 - Test 2"
   ```

### project.pbxproj conflicts

Multiple instances modified the Xcode project file.

**Solutions:**

1. Use mergepbx for automatic resolution:
   ```bash
   brew install mergepbx
   git config merge.mergepbx.driver "mergepbx %O %A %B"
   echo "*.pbxproj merge=mergepbx" >> .gitattributes
   ```

2. Manual resolution:
   ```bash
   cd .claudio/worktrees/<id>
   git checkout --theirs *.pbxproj  # or --ours
   # Re-add your changes in Xcode
   ```

3. Prevent conflicts - assign project file changes to one instance

### Xcode index outdated in worktree

After making changes, Xcode shows stale completions or errors.

**Solutions:**

1. Close and reopen project in Xcode

2. Delete index:
   ```bash
   rm -rf ~/Library/Developer/Xcode/DerivedData/MyApp-*/Index
   ```

3. Rebuild:
   ```bash
   xcodebuild -scheme MyApp build
   ```

### Swift Package Manager resolution slow

Each worktree resolving packages separately.

**Solutions:**

1. Pre-resolve in main repo:
   ```bash
   swift package resolve
   ```

2. Check global cache is enabled:
   ```bash
   # SPM uses ~/Library/Caches/org.swift.swiftpm/ by default
   ls ~/Library/Caches/org.swift.swiftpm/
   ```

3. For large dependencies, use binary frameworks when possible

### CocoaPods issues in worktrees

Pods not properly installed in worktree.

**Solutions:**

1. Run pod install in the worktree:
   ```bash
   cd .claudio/worktrees/<id>
   pod install
   ```

2. For faster installs, use deployment mode:
   ```bash
   pod install --deployment
   ```

3. Ensure Podfile.lock is committed:
   ```bash
   git add Podfile.lock
   ```

See [iOS Development Tutorial](tutorials/ios-development.md) for comprehensive iOS workflow guidance.

---

## Recovery

### Complete reset

If all else fails, you can reset Claudio:

```bash
# Stop everything
claudio stop --force

# Clean all resources
claudio cleanup --force

# Remove .claudio directory
rm -rf .claudio

# Prune git worktrees
git worktree prune

# Remove claudio branches
git branch | grep claudio | xargs git branch -D

# Start fresh
claudio init
claudio start
```

### Getting help

If you're still stuck:

1. Check existing issues: [GitHub Issues](https://github.com/Iron-Ham/claudio/issues)
2. Search for your error message
3. Open a new issue with:
   - Claudio version (`claudio --version`)
   - Operating system
   - Steps to reproduce
   - Error messages
   - Relevant config (without secrets)

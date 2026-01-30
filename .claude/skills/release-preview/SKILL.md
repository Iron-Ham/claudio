---
name: release-preview
description: "Preview a release without making changes (dry-run)"
argument-hint: "[major|minor|patch|vX.Y.Z]"
allowed-tools: Bash(git tag:*), Bash(git log:*), Bash(git status:*), Bash(git branch:*), Bash(git rev-parse:*), Bash(gh release view:*), Bash(gh auth status:*), Bash(date:*), Read
---

# Preview a Release (Dry Run)

Preview what a release would look like without making any changes. This is a read-only operation useful for:
- Verifying the changelog is complete before cutting a release
- Reviewing the generated release notes
- Confirming the version number is correct
- Checking that all prerequisites are met

## Context

- Current branch: !`git branch --show-current`
- Latest tags: !`git tag --sort=-v:refname | head -5`
- Git status: !`git status --short`

## Arguments

The user may provide: "$ARGUMENTS"

Valid arguments:
- `major` - Preview major version bump (X.0.0)
- `minor` - Preview minor version bump (x.Y.0)
- `patch` - Preview patch version bump (x.y.Z)
- Explicit version like `v1.2.3` or `1.2.3`
- Empty/none - Auto-detect from CHANGELOG.md content

## Preview Workflow

### Step 1: Check Release Readiness

Evaluate current state and report status:

**Branch Status:**
- Current branch name
- Is it `main`? (yellow warning if not)

**Working Directory:**
- Clean or dirty?
- If dirty, list affected files

**Remote Sync:**
- Is local up-to-date with `origin/main`?
- How many commits ahead/behind?

**GitHub CLI:**
- Is `gh` authenticated?

Report in a clear status block:
```
╔════════════════════════════════════════════════════════════════════╗
║                     RELEASE READINESS CHECK                        ║
╚════════════════════════════════════════════════════════════════════╝

Branch:            main                     ✓ Ready
Working Directory: clean                    ✓ Ready
Remote Sync:       up-to-date               ✓ Ready
GitHub CLI:        authenticated            ✓ Ready

Overall Status: READY TO RELEASE
```

Or with issues:
```
Branch:            feature/fix-bug          ⚠ Warning (not main)
Working Directory: 2 uncommitted files      ✗ Blocking
```

### Step 2: Analyze Unreleased Changes

Read CHANGELOG.md and extract the `## [Unreleased]` section.

**Count and categorize changes:**

```
╔════════════════════════════════════════════════════════════════════╗
║                    UNRELEASED CHANGES SUMMARY                      ║
╚════════════════════════════════════════════════════════════════════╝

│ Category      │ Count │ Entries                                    │
├───────────────┼───────┼────────────────────────────────────────────┤
│ Added         │   3   │ • Color Themes                             │
│               │       │ • Adversarial Review Mode                  │
│               │       │ • Background Cleanup Jobs                  │
├───────────────┼───────┼────────────────────────────────────────────┤
│ Changed       │   1   │ • Instance Manager Callbacks               │
├───────────────┼───────┼────────────────────────────────────────────┤
│ Fixed         │   4   │ • Stale RUNNING status                     │
│               │       │ • Input mode exit on plan open             │
│               │       │ • Theme persistence                        │
│               │       │ • Adversarial sub-group ID                 │
├───────────────┼───────┼────────────────────────────────────────────┤
│ Removed       │   0   │                                            │
├───────────────┼───────┼────────────────────────────────────────────┤
│ Security      │   0   │                                            │
├───────────────┼───────┼────────────────────────────────────────────┤
│ Performance   │   0   │                                            │
└───────────────┴───────┴────────────────────────────────────────────┘

Total: 8 entries ready to release
```

If the Unreleased section is empty:
```
⚠ UNRELEASED SECTION IS EMPTY

No changes to release. Add entries to the [Unreleased] section before
cutting a release.

Preview cannot continue.
```

### Step 3: Determine Version Number

Based on changes (or user argument), calculate the next version:

```
╔════════════════════════════════════════════════════════════════════╗
║                      VERSION DETERMINATION                         ║
╚════════════════════════════════════════════════════════════════════╝

Current Version:  v0.13.0 (released 2026-01-29)
Last 3 Releases:
  • v0.13.0 - 2026-01-29 (7 days ago)
  • v0.12.7 - 2026-01-23 (13 days ago)
  • v0.12.6 - 2026-01-22 (14 days ago)

Change Analysis:
  └─ Added section contains 3 new features
  └─ No breaking changes detected in Changed/Removed

Recommendation: MINOR bump (new features, backwards compatible)

  ╭─────────────────────────────────────╮
  │  Next Version:  v0.14.0            │
  ╰─────────────────────────────────────╯
```

### Step 4: Generate Release Notes Preview

Create the full release notes exactly as they would appear:

```
╔════════════════════════════════════════════════════════════════════╗
║                     RELEASE NOTES PREVIEW                          ║
╚════════════════════════════════════════════════════════════════════╝

Title: "Color Themes and Adversarial Review Mode"

────────────────────────────────────────────────────────────────────

This release introduces user-selectable color themes and an adversarial
review mode for tripleshot that pairs each implementer with a critical
reviewer.

## Highlights

### Color Themes
Choose from 14 carefully designed color themes to personalize your
terminal experience. Themes include Monokai, Dracula, Nord, and more.
- Configure via `tui.theme` in config or select interactively
- Create custom themes in `~/.config/claudio/themes/`
- Theme changes apply immediately with live preview

### Adversarial Review Mode
Each tripleshot implementer is now paired with a reviewer who must
approve the work (score >= 8/10) before it's considered complete.
- Enable with `--adversarial` flag or `tripleshot.adversarial` config
- Automatic retry on reviewer rejection
- Clear stuck detection and recovery

## Other Improvements
- Background cleanup jobs run async for better performance
- Enhanced sidebar status with elapsed time, cost, and file counts

## Bug Fixes
- Fixed stale RUNNING status after tmux server death
- Fixed input mode not exiting when plan editor opens
- Fixed theme persistence across restarts

────────────────────────────────────────────────────────────────────
```

### Step 5: Show CHANGELOG.md Changes

Display the exact edits that would be made:

```
╔════════════════════════════════════════════════════════════════════╗
║                    CHANGELOG.MD CHANGES PREVIEW                    ║
╚════════════════════════════════════════════════════════════════════╝

The following changes would be made to CHANGELOG.md:

1. REPLACE: "## [Unreleased]"
   WITH:    "## [0.14.0] - 2026-01-30"

2. INSERT after header (line ~7):
   ────────────────────────────────────────────────────────
   ## [Unreleased]

   ────────────────────────────────────────────────────────

3. APPEND to version links at EOF:
   ────────────────────────────────────────────────────────
   [0.14.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.14.0
   ────────────────────────────────────────────────────────
```

### Step 6: Summary

Provide the final preview summary:

```
╔════════════════════════════════════════════════════════════════════╗
║                     RELEASE PREVIEW SUMMARY                        ║
╠════════════════════════════════════════════════════════════════════╣
║                                                                    ║
║  Version:      v0.14.0                                             ║
║  Title:        "Color Themes and Adversarial Review Mode"          ║
║  Date:         2026-01-30                                          ║
║  Changes:      3 Added, 1 Changed, 4 Fixed                         ║
║                                                                    ║
╠════════════════════════════════════════════════════════════════════╣
║                                                                    ║
║  Actions that would be performed:                                  ║
║                                                                    ║
║    1. ✎ Edit CHANGELOG.md                                          ║
║       • Change [Unreleased] to [0.14.0] - 2026-01-30              ║
║       • Add new empty [Unreleased] section                        ║
║       • Add version link at end of file                           ║
║                                                                    ║
║    2. ⊕ Create commit                                              ║
║       • Message: "chore: release v0.14.0"                         ║
║                                                                    ║
║    3. ⊗ Create git tag                                             ║
║       • Tag: v0.14.0 (annotated)                                  ║
║                                                                    ║
║    4. ↑ Push to origin                                             ║
║       • Push commit to main                                       ║
║       • Push tag v0.14.0                                          ║
║                                                                    ║
║    5. ◉ Create GitHub Release                                      ║
║       • Title: "Color Themes and Adversarial Review Mode"         ║
║       • Body: [release notes shown above]                         ║
║                                                                    ║
╠════════════════════════════════════════════════════════════════════╣
║                                                                    ║
║  To proceed with the actual release, run:                          ║
║                                                                    ║
║      /release minor                                                ║
║                                                                    ║
║  Or to specify version explicitly:                                 ║
║                                                                    ║
║      /release v0.14.0                                              ║
║                                                                    ║
╚════════════════════════════════════════════════════════════════════╝
```

## Key Points

- **Read-only** - This command makes no changes
- **Use before /release** - Verify everything looks correct first
- **Review blocking issues** - Resolve any items before releasing

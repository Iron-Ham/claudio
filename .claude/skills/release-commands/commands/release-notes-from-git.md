---
description: "Generate release notes from git history and pull requests"
allowed-tools: Read, Bash(git log:*), Bash(git describe:*), Bash(gh pr:*), Bash(gh api:*)
---

# Generate Release Notes from Git History

Generate release notes by analyzing git history and pull requests. This is useful when:
- The CHANGELOG.md is missing entries
- You want to cross-reference changelog with actual commits
- You need to identify contributors for acknowledgment
- You want to verify the changelog is complete before releasing

## Context

- Latest git tag: !`git describe --tags --abbrev=0 2>/dev/null || echo "No tags yet"`
- Commits since last tag: !`git log $(git describe --tags --abbrev=0 2>/dev/null)..HEAD --pretty=format:"%h %s (%an)" 2>/dev/null | head -50 || echo "No previous tags - showing recent commits" && git log --oneline -20`
- Merged PRs info: !`gh pr list --state merged --limit 20 --json number,title,author,mergedAt --jq '.[] | "#\(.number) \(.title) (@\(.author.login))"' 2>/dev/null || echo "gh CLI not available or not authenticated"`

## Your Task

Generate comprehensive release notes by analyzing git history and pull requests. This complements the changelog-based `/release-notes` command by working directly from source control.

### Analysis Steps

1. **Gather commit history** since the last tag
2. **Categorize commits** by conventional commit type (feat, fix, docs, refactor, etc.)
3. **Fetch PR information** for commits that reference PRs
4. **Identify contributors** from commit authors and PR authors
5. **Cross-reference with CHANGELOG.md** to find any gaps

### Output Format

Present the generated release notes:

```
═══════════════════════════════════════════════════════════════════
                    GENERATED RELEASE NOTES
                    (from git history)
═══════════════════════════════════════════════════════════════════

Based on X commits since vY.Z.A

### Features (feat:)
- Description from commit/PR (#123) - @author

### Bug Fixes (fix:)
- Description from commit/PR (#124) - @author

### Documentation (docs:)
- Description from commit

### Refactoring (refactor:)
- Description from commit

### Other Changes
- Commits that don't follow conventional commits

### Contributors
Thanks to the following contributors:
- @username1
- @username2

═══════════════════════════════════════════════════════════════════
```

### Categorization Rules

Map conventional commit prefixes to changelog categories:

| Commit Prefix | Changelog Category | Include? |
|---------------|-------------------|----------|
| `feat:` | Added | Yes |
| `fix:` | Fixed | Yes |
| `docs:` | Changed (Documentation) | Yes |
| `refactor:` | Changed | Yes |
| `perf:` | Performance | Yes |
| `test:` | Changed (Testing) | Optional |
| `chore:` | Changed | Depends on impact |
| `ci:` | (skip) | Usually no |
| `style:` | (skip) | Usually no |

### Quality Checks

After generating notes, identify potential issues:

```
═══════════════════════════════════════════════════════════════════
                    QUALITY CHECKS
═══════════════════════════════════════════════════════════════════

⚠ Commits without conventional commit format:
  - abc1234 "Update readme"
  - def5678 "Fix thing"

⚠ PRs merged without changelog entries:
  - #142 "Add new feature" (@contributor)

⚠ Breaking changes detected (look for ! or BREAKING CHANGE):
  - feat!: Remove deprecated API endpoint

✓ Large changes that may warrant detailed notes:
  - #145 "Refactor authentication system" (12 files changed)

═══════════════════════════════════════════════════════════════════
```

### Comparison with CHANGELOG

If CHANGELOG.md exists, compare and report discrepancies:

```
═══════════════════════════════════════════════════════════════════
                    CHANGELOG COMPARISON
═══════════════════════════════════════════════════════════════════

Commits WITHOUT corresponding changelog entry:
  ✗ abc1234 feat: add user preferences (#142)
  ✗ def5678 fix: memory leak in cache handler

Changelog entries WITHOUT corresponding commit:
  ? "Added dark mode support" - no matching commit found

Entries that match:
  ✓ feat: adversarial review mode (#140) → "Adversarial Review Mode"
  ✓ fix: stale status detection (#141) → "False Positive Stale Detection"

Summary: 2 missing from changelog, 1 orphaned entry, 5 matched

═══════════════════════════════════════════════════════════════════
```

### When to Use This vs /release-notes

| Use Case | Command |
|----------|---------|
| Generate notes from well-maintained CHANGELOG | `/release-notes` |
| Audit changelog completeness | `/release-notes-from-git` |
| CHANGELOG is incomplete or missing | `/release-notes-from-git` |
| Need contributor acknowledgments | `/release-notes-from-git` |
| Want PR links and numbers | `/release-notes-from-git` |
| Need specific output format (Slack, Discord) | `/release-notes` |

## Key Points

- **Read-only** - No changes are made to any files
- **Works best with conventional commits** - Uses commit prefixes for categorization
- **Requires gh CLI** - PR information needs `gh` to be authenticated
- **Use before releasing** - Catches missing changelog entries

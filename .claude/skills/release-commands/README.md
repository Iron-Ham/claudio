# Release Commands

A comprehensive Claude Code skill for cutting releases with detailed release notes, CHANGELOG updates, git tags, and GitHub releases.

## Overview

This skill automates the release process, ensuring consistency and thoroughness. It follows the [Keep a Changelog](https://keepachangelog.com/) format and [Semantic Versioning](https://semver.org/) conventions.

## Commands

### `/release [major|minor|patch|vX.Y.Z]`

The main release command. Cuts a complete release including:

1. **Pre-flight validation** - Checks branch, working directory, remote sync, and gh auth
2. **Version determination** - Analyzes unreleased changes and recommends semantic version
3. **Release notes generation** - Creates compelling narrative from changelog entries
4. **CHANGELOG update** - Converts `[Unreleased]` to versioned section with date
5. **Git commit** - Creates a release commit with conventional format
6. **Git tag** - Creates annotated tag for the release
7. **Push to remote** - Pushes commit and tag
8. **GitHub release** - Creates a GitHub release with detailed body

**Usage:**
```bash
# Auto-detect version based on changes
/release

# Explicit version type
/release minor

# Explicit version number
/release v1.5.0
```

**What happens:**
1. Claude validates you're ready to release (clean directory, on main, etc.)
2. Analyzes your unreleased changelog entries
3. Recommends a version number (you can override)
4. Generates compelling release notes (shown for approval)
5. Updates CHANGELOG.md with proper formatting
6. Creates commit: `chore: release vX.Y.Z`
7. Creates git tag: `vX.Y.Z`
8. Pushes to origin and creates GitHub release
9. Shows you the release URL

### `/release-preview [major|minor|patch|vX.Y.Z]`

Preview what a release would look like without making any changes. Useful for:
- Reviewing the release before committing
- Checking if the version recommendation makes sense
- Previewing the GitHub release body
- Verifying all prerequisites are met

**Usage:**
```bash
/release-preview
/release-preview minor
```

**Output includes:**
- Release readiness status (branch, working directory, remote sync)
- Categorized summary of unreleased changes
- Recommended version number with reasoning
- Full release notes preview
- Exact CHANGELOG.md edits that would be made
- Summary of all actions that would be performed

### `/release-notes [version] [--format FORMAT]`

Generate release notes from CHANGELOG.md without making any changes. Useful for:
- Drafting release announcements
- Creating posts for Slack, Discord, Twitter, or email
- Reviewing changelog content before release
- Generating notes for an existing release retroactively

**Arguments:**
- `version` - Optional. Generate notes for a specific version (e.g., `v0.13.0`). Defaults to Unreleased.
- `--format` - Output format: `markdown` (default), `slack`, `discord`, `twitter`, or `email`

**Usage:**
```bash
# Generate notes for unreleased changes
/release-notes

# Generate notes for a specific version
/release-notes v0.13.0

# Generate Slack-formatted announcement
/release-notes --format slack

# Generate Twitter thread
/release-notes --format twitter
```

### `/release-notes-from-git`

Generate release notes by analyzing git history and pull requests. Useful when:
- CHANGELOG.md has missing entries
- Cross-referencing changelog with commits
- Identifying contributors
- Auditing changelog completeness

**Usage:**
```bash
/release-notes-from-git
```

**Output includes:**
- Categorized changes from conventional commits
- PR information and authors
- Contributors list
- Commits missing from changelog
- Orphaned changelog entries without commits

## CHANGELOG Format

This skill expects and generates changelogs following Keep a Changelog format:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **Feature Name** - Description of the feature

### Changed
- **Component** - Description of change

### Removed
- **Deprecated Feature** - Why it was removed

### Fixed
- **Bug Area** - Description of fix

## [0.13.0] - 2026-01-29

This release introduces major features X and Y.

### Added
- **Feature X** - Description

[0.13.0]: https://github.com/owner/repo/releases/tag/v0.13.0
```

## Semantic Versioning

| Change Type | Version Bump |
|------------|--------------|
| Breaking changes (Removed/Changed) | MAJOR |
| New features, deprecations | MINOR |
| Bug fixes, performance, security | PATCH |

## GitHub Release Body

The generated GitHub release includes:

1. **Opening paragraph** - Release theme and value to users
2. **Highlights** - Top 2-3 most impactful changes with context
3. **What's New** - New features list
4. **Improvements** - Changes and enhancements
5. **Bug Fixes** - Issues resolved
6. **Breaking Changes** - Migration steps if applicable

Example:
```markdown
This release introduces Adversarial Review Mode and Color Themes,
significantly enhancing workflow quality and user experience.

## Highlights

### Adversarial Review Mode
Each implementer is now paired with a critical reviewer, ensuring
higher quality outputs through structured feedback loops.
- Enable with `--adversarial` flag
- Automatic retry on rejection

### Color Themes
14 built-in themes plus custom theme support for personalizing
your terminal experience.

## What's New
- Adversarial Review Mode for Tripleshot
- Color Themes with 14 built-in options
- Custom Theme Support

## Bug Fixes
- Fixed session attachment output capture
- Fixed input mode editor interaction

---
Full Changelog: https://github.com/owner/repo/blob/main/CHANGELOG.md
```

## Requirements

- **Git** - For commits and tags
- **GitHub CLI** - `brew install gh` then `gh auth login`

## Best Practices

**Before releasing:**
- Document all changes in `[Unreleased]`
- Run tests and ensure CI passes
- Use `/release-preview` to verify the release
- Use `/release-notes-from-git` to catch missing entries

**During release:**
- Review the recommended version number
- Approve the release summary and CHANGELOG diff
- Review the GitHub release body

**After releasing:**
- Verify the GitHub release page
- Announce via `/release-notes --format slack`

## Troubleshooting

| Error | Solution |
|-------|----------|
| No unreleased changes found | Add entries to `[Unreleased]` section |
| gh: command not found | `brew install gh` |
| gh: not authenticated | `gh auth login` |
| Working directory is dirty | Commit or stash changes first |
| Branch protection | Create a PR instead of direct push |
| Not on main branch | `git checkout main` or confirm current branch |
| Push rejected | `git pull --rebase origin main` |

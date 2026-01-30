---
name: release-notes
description: "Generate release notes from CHANGELOG.md without cutting a release"
argument-hint: "[version] [--format markdown|slack|discord|twitter]"
allowed-tools: Bash(git tag:*), Bash(git log:*), Bash(date:*), Bash(gh repo view:*), Read
---

# Generate Release Notes

Generate polished release notes from the CHANGELOG.md without making any changes. Useful for:
- Drafting release announcements before cutting a release
- Creating posts for Slack, Discord, Twitter, or other platforms
- Reviewing changelog content quality before release
- Generating notes for an existing release retroactively

## Context

- Repository: !`gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null || git remote get-url origin 2>/dev/null | sed 's/.*github.com[:/]\([^/]*\/[^.]*\).*/\1/'`
- Latest tags: !`git tag --sort=-v:refname | head -3`

## Arguments

The user may provide: "$ARGUMENTS"

**Version (optional):**
- `vX.Y.Z` or `X.Y.Z` - Generate notes for a specific released version
- `unreleased` or empty - Generate notes for the Unreleased section (default)

**Format (optional):**
- `--format markdown` - Full GitHub/documentation format (default)
- `--format slack` - Slack-optimized announcement
- `--format discord` - Discord-optimized announcement
- `--format twitter` - Twitter/X thread format (280 char limit per tweet)
- `--format email` - Email announcement format

## Workflow

### Step 1: Identify Target Content

Read CHANGELOG.md and locate the section to generate notes for:

**If no version specified or `unreleased`:**
- Use the `## [Unreleased]` section
- Warn if empty

**If version specified:**
- Find the `## [X.Y.Z]` section
- Error if version not found

Report which section you're using:
```
Generating release notes for: [Unreleased]
```
or
```
Generating release notes for: v0.13.0 (released 2026-01-29)
```

### Step 2: Extract and Analyze Changes

Parse the changelog section into categories and identify highlights:

**Categories:** Added, Changed, Deprecated, Removed, Fixed, Security, Performance

**Identify highlights** - Pick 2-3 most significant items (priority: Security > Features > Breaking changes > Bug fixes)

Report the analysis:
```
Changes Summary:
├─ Security: 0
├─ Added: 3 entries (highlights: Color Themes, Adversarial Mode)
├─ Changed: 2 entries
├─ Fixed: 4 entries (highlight: Stale status fix)
└─ Total: 9 entries
```

### Step 3: Generate Release Notes

Based on the format argument (default: markdown), generate appropriate release notes.

---

#### Format: Markdown (default)

Full-featured release notes for GitHub Releases or documentation.

```markdown
# [Compelling Title Summarizing Release]

[Opening paragraph: 1-2 sentences describing the release theme and primary value]

## Highlights

### [Most Significant Feature]
[2-3 sentences explaining what it does and why users should care]
- Key capability 1
- Key capability 2

### [Second Significant Item]
[Similar structure]

## What's New
- **Feature Name** - Brief description of the new capability
- **Another Feature** - Brief description

## Improvements
- **Changed Item** - What changed and why

## Bug Fixes
- **Issue Fixed** - What was broken and how it's now resolved
- **Another Fix** - Description

## Breaking Changes
<!-- Only if applicable -->
⚠️ **Breaking**: [What changed]
- Migration: [How to update]

---
📋 Full changelog: [CHANGELOG.md](link)
📦 Release: [vX.Y.Z](link)
```

---

#### Format: Slack

Optimized for Slack announcements. Uses Slack markdown (mrkdwn).

```
:rocket: *[Project Name] vX.Y.Z Released!*

[One compelling sentence about the release]

*Highlights:*
• *[Feature 1]* - Brief description
• *[Feature 2]* - Brief description
• *[Key Fix]* - What was fixed

*Other changes:*
• [Minor item 1]
• [Minor item 2]

:link: <https://github.com/owner/repo/releases/tag/vX.Y.Z|View Release Notes>
:book: <https://github.com/owner/repo/blob/main/CHANGELOG.md|Full Changelog>
```

---

#### Format: Discord

Optimized for Discord announcements. Uses Discord markdown.

```
## :rocket: [Project Name] vX.Y.Z Released!

[One compelling sentence about what's new]

### Highlights
- **[Feature 1]** - Brief description
- **[Feature 2]** - Brief description
- **[Key Fix]** - What was fixed

### Other Changes
- [Minor item 1]
- [Minor item 2]

:link: **Release:** <https://github.com/owner/repo/releases/tag/vX.Y.Z>
```

---

#### Format: Twitter

Thread format optimized for Twitter/X (280 char limit per tweet).

```
🧵 Thread: [Project] vX.Y.Z Release

1/N
🚀 [Project] vX.Y.Z is out!

[Compelling one-liner about the release]

Here's what's new: 👇

2/N
✨ [Feature 1]

[2 sentences max describing the feature and its benefit]

3/N
✨ [Feature 2]

[2 sentences max]

4/N
🐛 Bug Fixes:
• [Fix 1]
• [Fix 2]
• [Fix 3]

5/N
📦 Get it now:
github.com/owner/repo/releases/tag/vX.Y.Z

Full changelog in thread or link in bio.

#opensource #[relevant hashtag]
```

**Important for Twitter:**
- Each tweet must be ≤280 characters
- Number tweets as 1/N, 2/N, etc.
- Use emoji for visual appeal
- End with relevant hashtags

---

#### Format: Email

Formatted for email newsletters or announcements.

```
Subject: [Project Name] vX.Y.Z Released - [Key Feature Highlight]

Hi [community/team],

We're excited to announce the release of [Project Name] vX.Y.Z!

WHAT'S NEW
==========

[Feature 1]
[2-3 sentences explaining the feature and its benefits]

[Feature 2]
[2-3 sentences]

BUG FIXES
=========
• [Fix 1 description]
• [Fix 2 description]

BREAKING CHANGES
================
[Only if applicable]
• [What changed and how to migrate]

GET THE UPDATE
==============
Release: https://github.com/owner/repo/releases/tag/vX.Y.Z
Changelog: https://github.com/owner/repo/blob/main/CHANGELOG.md

Thanks for using [Project Name]!

The [Project/Team] Team
```

---

### Step 4: Output

Present the generated notes in a clearly marked code block:

```
╔════════════════════════════════════════════════════════════════════╗
║                    GENERATED RELEASE NOTES                         ║
║                    Format: [selected format]                       ║
╚════════════════════════════════════════════════════════════════════╝

[Generated notes in code block for easy copying]
```

**If generating for Unreleased section:**
```
NOTE: These notes are for unreleased changes.
When ready to release, run: /release [version-type]
```

**If generating for a past release:**
```
NOTE: These notes were generated for an already-released version.
The GitHub Release may already have notes that differ from these.
```

## Writing Guidelines

- **Lead with value** - What does this release give users?
- **Be specific** - "Fixed memory leak in cache handler" not "Fixed bug"
- **Group related changes** - Summarize multiple small fixes together
- **Highlight breaking changes** - Make them impossible to miss
- **Use active voice** - "Added support for..." not "Support has been added..."
- **Match the platform** - Slack uses mrkdwn, Discord uses different markdown
- **Respect character limits** - Twitter tweets must be ≤280 chars

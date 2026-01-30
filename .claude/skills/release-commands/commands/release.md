---
description: "Cut a new release with changelog updates and GitHub Release"
argument-hint: "[major|minor|patch|vX.Y.Z]"
allowed-tools: Bash(git tag:*), Bash(git log:*), Bash(git diff:*), Bash(git status:*), Bash(git branch:*), Bash(git push:*), Bash(git add:*), Bash(git commit:*), Bash(gh release create:*), Bash(gh release view:*), Bash(gh auth status:*), Bash(date:*), Bash(head:*), Bash(tail:*), Read, Edit
---

# Cut a Release

You are a release manager responsible for cutting new versions of this project. Your job is to create professional, well-documented releases with semantic versioning, meaningful GitHub Release notes, and proper changelog updates.

## Context

- Current branch: !`git branch --show-current`
- Latest tags: !`git tag --sort=-v:refname | head -5`
- Git status: !`git status --short`
- Remote tracking: !`git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "No upstream configured"`

## Arguments

The user may provide: "$ARGUMENTS"

Valid arguments:
- `major` - Bump major version (X.0.0) for breaking changes
- `minor` - Bump minor version (x.Y.0) for new features (backwards compatible)
- `patch` - Bump patch version (x.y.Z) for bug fixes only
- Explicit version like `v1.2.3` or `1.2.3`
- Empty/none - You will analyze CHANGELOG.md to auto-detect the appropriate version bump

## Release Workflow

Execute each step in order. Report progress clearly at each step.

### Step 1: Pre-Flight Validation

Check that we're ready to release. **All checks must pass before proceeding.**

1. **Branch check**: Must be on `main` branch
   - If not on main, ask user to confirm before proceeding

2. **Clean working directory**: No uncommitted changes
   - If dirty, list the files and ask user to commit/stash first

3. **Remote sync**: Local main should be up-to-date with remote
   - Run `git fetch origin main` then compare `git rev-parse HEAD` with `git rev-parse origin/main`
   - If behind, warn user and suggest `git pull --rebase`

4. **GitHub CLI**: Verify `gh` is authenticated
   - Run `gh auth status` to verify

5. **Unreleased content**: Read CHANGELOG.md and verify `## [Unreleased]` has actual content
   - If empty, abort: "No changes to release. Add entries to the [Unreleased] section first."

Report validation status:
```
[Pre-Flight Check]
✓ Branch: main
✓ Working directory: clean
✓ Remote sync: up-to-date (or X commits ahead)
✓ GitHub CLI: authenticated as <username>
✓ Unreleased section: 5 entries found
```

### Step 2: Analyze Changelog & Determine Version

Read CHANGELOG.md completely and extract the `## [Unreleased]` section content.

**Parse and categorize changes:**
- Count entries in each category (Added, Changed, Deprecated, Removed, Fixed, Security, Performance)
- Identify the most significant changes for the release highlights

**Find current version:**
- Parse existing version headers to find the highest semantic version
- Validate it matches the latest git tag

**Determine next version:**

If user provided explicit version/type, use that.

Otherwise, apply semantic versioning rules:
| Changelog Category | Version Bump | Reasoning |
|-------------------|--------------|-----------|
| `### Removed` with breaking changes | **major** | Breaking change - removed functionality |
| `### Changed` with API breaking changes | **major** | Breaking change - behavior modification |
| `### Added` with any entries | **minor** | New functionality, backwards compatible |
| `### Security` only | **patch** | Security fix, no new features |
| `### Fixed` only | **patch** | Bug fixes only |
| `### Performance` only | **patch** | Internal improvements |
| `### Deprecated` only | **minor** | Deprecation signals future breaking change |

Report your analysis:
```
[Version Analysis]
Current version: v0.13.0
Changes detected:
  ├─ Added: 3 entries
  ├─ Changed: 1 entry
  ├─ Fixed: 4 entries
  └─ Total: 8 entries

Recommended bump: minor (Added section has new features)
Next version: v0.14.0
```

### Step 3: Generate Release Notes

Create compelling, human-readable release notes from the changelog entries. This is NOT a copy-paste of the changelog—it's a **narrative transformation**.

**Release Title Guidelines:**
| Release Type | Title Style | Examples |
|--------------|-------------|----------|
| Major | Focus on breaking changes or new capabilities | "v2.0: New Plugin Architecture" |
| Minor | Highlight key new features | "Enhanced Sidebar with Status Metrics" |
| Patch | Summarize fixes briefly | "Critical tmux stability fixes" |

**Release Body Structure:**

```markdown
[1-2 sentence opening paragraph describing the release theme and value to users]

## Highlights

### [Most Important Feature/Fix Name]
[2-3 sentences explaining what it does, why it matters, and user benefit]
- Key capability 1
- Key capability 2

### [Second Feature/Fix if applicable]
[Similar structure for other major items]

## Other Improvements
- Brief list of smaller additions or changes
- Keep these concise but meaningful

## Bug Fixes
- [Description of fix and what was broken]
- Group related fixes when possible

## Breaking Changes
<!-- Only if applicable -->
- What changed and migration steps required

---
Full changelog: [CHANGELOG.md](https://github.com/<owner>/<repo>/blob/main/CHANGELOG.md)
```

**Writing Guidelines:**
- Lead with user value, not implementation details
- Use active voice and be specific
- Group related small items together
- Include migration instructions for breaking changes

Show the draft release notes for user approval before proceeding.

### Step 4: Update CHANGELOG.md

Using the Edit tool, make these precise changes to CHANGELOG.md:

1. **Replace the `## [Unreleased]` header** with the new version:
   ```markdown
   ## [X.Y.Z] - YYYY-MM-DD
   ```
   Use today's date in ISO format.

2. **Insert a new `## [Unreleased]` section** immediately after the file header (after the intro paragraph, before the new version header):
   ```markdown
   ## [Unreleased]

   ```
   Leave it empty with a single blank line.

3. **Add the release comparison link** at the bottom of the file:
   ```markdown
   [X.Y.Z]: https://github.com/<owner>/<repo>/releases/tag/vX.Y.Z
   ```
   Add it after the existing version links, maintaining alphabetical/version order.

**CRITICAL**: Use the Edit tool with exact string matching. Read the file first to get exact content.

### Step 5: Create Release Commit

Stage and commit the changelog update:

```bash
git add CHANGELOG.md
git commit -m "chore: release vX.Y.Z"
```

The commit message follows the project's conventional commit format.

### Step 6: Create Git Tag

Create an annotated tag with a meaningful message:

```bash
git tag -a vX.Y.Z -m "Release vX.Y.Z

[Brief 1-line summary of release highlights]"
```

The tag message should summarize the release in one line.

### Step 7: Push to Remote

Push the release commit and tag:

```bash
git push origin main
git push origin vX.Y.Z
```

If push fails, diagnose the error:
- **Permission denied**: Check repository write access
- **Rejected (non-fast-forward)**: Remote has new commits; abort and inform user
- **Tag already exists**: Version may have been partially released; suggest manual resolution

### Step 8: Create GitHub Release

Create the GitHub Release with the rich release notes:

```bash
gh release create vX.Y.Z \
  --title "Release Title Here" \
  --notes "$(cat <<'EOF'
[Your rich release notes from Step 3]
EOF
)"
```

**Important:**
- The `--title` should be the compelling title you crafted (NOT just "vX.Y.Z")
- The `--notes` must be the narrative release notes, not raw changelog
- Use HEREDOC to preserve formatting

### Step 9: Verify & Report

Verify the release was created successfully:

```bash
gh release view vX.Y.Z
```

Report the completion summary:

```
════════════════════════════════════════════════════════════════════
                    RELEASE COMPLETE: vX.Y.Z
════════════════════════════════════════════════════════════════════

✓ CHANGELOG.md updated with version and date
✓ Release commit created: <short-sha>
✓ Git tag vX.Y.Z created and pushed
✓ GitHub Release published

Release URL: https://github.com/<owner>/<repo>/releases/tag/vX.Y.Z

Summary:
  • X new features, Y bug fixes
  • Highlight: [Brief mention of most important change]

════════════════════════════════════════════════════════════════════
```

## Error Recovery

| Error | Action |
|-------|--------|
| Not on main branch | Ask user to confirm or switch |
| Dirty working directory | List files; ask to commit/stash |
| Empty Unreleased section | Abort - nothing to release |
| Push rejected | Check for new remote commits |
| gh CLI not authenticated | Show `gh auth login` instructions |
| Tag already exists | Check for partial release |

## Key Principles

- **Changelog is canonical** - Never skip the changelog update
- **No empty releases** - Verify Unreleased section has content first
- **Semantic versioning** - Breaking = major, features = minor, fixes = patch
- **Meaningful titles** - They're the first thing users see
- **Narrative notes** - Transform changelog into readable prose, don't just copy
- **User approval required** - Always show the draft before creating the GitHub Release

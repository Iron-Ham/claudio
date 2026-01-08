# Large Refactor: Coordinating a Major Refactoring Effort

**Time: 20 minutes**

This tutorial demonstrates how to use Claudio to coordinate a large refactoring effort, such as migrating a codebase to a new pattern or updating API contracts.

## Scenario

You're migrating from callbacks to async/await across your codebase, or converting a JavaScript project to TypeScript. This affects many files but can be parallelized by module.

## Prerequisites

- Claudio initialized (`claudio init`)
- Understanding of the refactor scope
- Clear target state for the migration

## Step 1: Analyze and Plan

Before starting, map out your codebase:

```bash
# Example: Find all callback-style code
grep -r "function.*callback" src/ --include="*.js" -l
```

Group files by module or domain:
```
Modules to migrate:
├── src/api/       (15 files)
├── src/services/  (8 files)
├── src/utils/     (12 files)
└── src/db/        (6 files)
```

## Step 2: Configure for Refactoring

Set up your config for the refactor session:

```yaml
# config.yaml
completion:
  default_action: keep_branch  # Review before merging

branch:
  prefix: "refactor"
  include_id: true  # Keep IDs for tracking

pr:
  auto_rebase: true
  labels:
    - refactor
    - needs-review

resources:
  cost_warning_threshold: 10.00
  show_metrics_in_sidebar: true
```

## Step 3: Start the Refactor Session

```bash
claudio start callback-to-async
```

## Step 4: Create Module-Specific Instances

Add an instance for each module. Be specific about the transformation:

**API Module:**
```
Convert all callback-based functions in src/api/ to async/await.
For each file:
1. Replace callback parameters with async function
2. Replace callback calls with await
3. Add proper error handling with try/catch
4. Update function signatures in index exports
Maintain backward compatibility by keeping the same function names.
```

**Services Module:**
```
Migrate src/services/ from callbacks to async/await. Follow these rules:
1. Convert callback(err, result) patterns to try/catch
2. Replace series operations with sequential await
3. Replace parallel operations with Promise.all
4. Ensure error messages are preserved
```

**Utils Module:**
```
Update src/utils/ to use async/await. Pay attention to:
1. Helper functions that wrap callbacks
2. Utility functions with callback last parameter
3. Promisify any remaining callback APIs
4. Update JSDoc comments for async functions
```

**Database Module:**
```
Convert src/db/ database operations to async/await.
Important: Database connections must remain stable.
1. Convert query callbacks to await
2. Handle transaction rollback in catch blocks
3. Ensure connection pooling still works
4. Add TypeScript types if .ts files
```

## Step 5: Provide Context Files

If your refactor needs reference material, create context files before starting:

```bash
# Create a migration guide for instances
cat > .claudio/migration-guide.md << 'EOF'
# Callback to Async/Await Migration Guide

## Pattern Transformations

### Before (Callback):
```javascript
function getData(id, callback) {
  db.query('SELECT * FROM items WHERE id = ?', [id], (err, rows) => {
    if (err) return callback(err);
    callback(null, rows[0]);
  });
}
```

### After (Async/Await):
```javascript
async function getData(id) {
  try {
    const rows = await db.query('SELECT * FROM items WHERE id = ?', [id]);
    return rows[0];
  } catch (err) {
    throw new Error(`Failed to get data: ${err.message}`);
  }
}
```
EOF
```

Reference it in tasks:
```
Convert src/api/ to async/await following .claudio/migration-guide.md patterns.
```

## Step 6: Monitor for Conflicts

With a large refactor, conflicts are likely. Press `c` regularly to check:

```
Conflicting Files:
─────────────────────────────────────────
src/index.ts
  Modified by: api-refactor (abc123), services-refactor (def456)

src/types/index.d.ts
  Modified by: api-refactor (abc123), db-refactor (ghi789)
```

## Step 7: Handle Shared Files

For files multiple instances need to modify:

### Option A: Designate One Owner
```
Convert src/api/ to async/await. Also update src/index.ts with
the new async exports. No other instance should modify index.ts.
```

### Option B: Create a Dedicated Instance
```
Update src/index.ts to export all modules as async. Wait for other
refactor instances to complete before finalizing exports.
```

### Option C: Manual Merge
Let instances work independently and merge manually:
```bash
cd .claudio/worktrees/abc123
git diff src/index.ts > /tmp/api-changes.patch

cd ../def456
git apply /tmp/api-changes.patch
```

## Step 8: Validate Each Module

Before creating PRs, add validation instances:

```
Run all tests in tests/api/ and report any failures.
For failures, identify if they're due to the async/await migration.
```

```
Check that src/api/ has no remaining callback patterns:
1. No function parameters named 'callback' or 'cb'
2. No err-first callback calls
3. All async functions properly awaited
```

## Step 9: Create PRs in Order

For interdependent modules, order matters:

1. **Utils first** (no dependencies)
2. **Database second** (may use utils)
3. **Services third** (uses db and utils)
4. **API last** (uses all above)

For each:
1. Select the instance
2. Press `x` to stop
3. Create PR
4. Wait for CI to pass
5. Merge before proceeding

Or use draft PRs to get CI feedback:
```yaml
pr:
  draft: true
```

## Step 10: Track Progress

Create a tracking issue or document:

```markdown
# Callback to Async Migration

## Modules

| Module | Instance | PR | Status |
|--------|----------|-----|--------|
| Utils | abc123 | #45 | Merged |
| Database | def456 | #46 | In Review |
| Services | ghi789 | #47 | CI Running |
| API | jkl012 | - | In Progress |

## Blockers
- Database module has flaky test, investigating

## Notes
- Keep backward compat for external API consumers
```

## Tips for Large Refactors

### Atomic Changes

Keep each instance's scope manageable:
```
Good: "Convert src/api/users.ts and src/api/posts.ts to async"
Too big: "Convert all of src/ to async"
```

### Test Continuously

Add test validation as instances complete:
```
Run tests for src/services/ and fix any failures caused by
the async/await migration.
```

### Preserve Behavior

Emphasize correctness in task descriptions:
```
Convert to async/await while maintaining exact same behavior.
Add any missing error handling but don't change business logic.
```

### Handle Rollback

If a module causes issues:
```bash
# Delete the problematic branch
git branch -D refactor/abc123-api

# Remove the instance
claudio remove abc123 --force

# Start fresh with revised instructions
claudio add "Convert src/api/ with special handling for..."
```

### Batch Similar Files

Group files that follow the same pattern:
```
Convert these similar controller files to async:
- src/controllers/user.js
- src/controllers/post.js
- src/controllers/comment.js
All follow the same CRUD pattern, transform consistently.
```

## What You Learned

- Planning large refactors for parallelization
- Managing dependencies between modules
- Handling conflicts in shared files
- Validating refactored code
- Creating PRs in dependency order

## Next Steps

- [Feature Development Tutorial](feature-development.md) - Similar patterns for features
- [Configuration Guide](../guide/configuration.md) - Customize for your workflow
- [Troubleshooting](../troubleshooting.md) - Handle common issues

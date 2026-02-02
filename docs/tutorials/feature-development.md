# Feature Development: Implementing a Feature with Multiple Instances

**Time: 15-20 minutes**

This tutorial demonstrates how to use Claudio to implement a complete feature by breaking it down into parallel tasks.

## Scenario

You're building a user authentication system. Instead of implementing it sequentially, you'll use Claudio to work on multiple components simultaneously.

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Familiarity with the TUI basics (see [Quick Start](quick-start.md))

## Step 1: Plan Your Tasks

Before starting, break down the feature into independent, parallelizable tasks:

| Task | Component | Files |
|------|-----------|-------|
| User model | Data layer | `src/models/user.ts` |
| Auth routes | API layer | `src/routes/auth.ts` |
| Auth middleware | Middleware | `src/middleware/auth.ts` |
| Unit tests | Testing | `tests/auth.test.ts` |

**Key insight**: These tasks touch different files, minimizing conflicts.

## Step 2: Start the Session

```bash
claudio start auth-feature
```

## Step 3: Add All Tasks

Press `a` for each task:

**Task 1 - User Model:**
```
Create a User model in src/models/user.ts with fields: id (uuid), email (unique),
passwordHash, createdAt, and updatedAt. Include validation for email format.
```

**Task 2 - Auth Routes:**
```
Create authentication routes in src/routes/auth.ts with POST /register and
POST /login endpoints. Use placeholder functions for now.
```

**Task 3 - Auth Middleware:**
```
Create auth middleware in src/middleware/auth.ts that validates JWT tokens
from the Authorization header. Export an authenticate function.
```

**Task 4 - Tests:**
```
Create unit tests in tests/auth.test.ts for user registration and login
flows. Use jest and mock the database.
```

Now you have 4 instances running in parallel!

## Step 4: Monitor Progress

### Check Status

The sidebar shows each instance's state:
- `[working]` - Actively coding
- `[waiting_input]` - Needs your input
- `[completed]` - Done

### View Output

Press `1`, `2`, `3`, or `4` to see each instance's output.

### Check for Conflicts

Press `c` to see the conflict view. Ideally, each instance works on different files.

If you see conflicts:
```
src/index.ts
  Modified by: Instance 1 (abc123), Instance 2 (def456)
```

This is expected for files like `index.ts` that might need imports from multiple modules.

## Step 5: Handle Input Requests

When an instance shows `[waiting_input]`:

1. Select that instance
2. Read what the backend is asking
3. Press `Enter` to focus input
4. Type your response
5. Press `Enter` to send

Example prompt from the backend:
```
Should I use bcrypt or argon2 for password hashing?
```

Your response:
```
Use bcrypt with 10 salt rounds
```

## Step 6: Review Diffs

As instances complete, review their work:

1. Select an instance
2. Press `d` to see the diff

Example diff output:
```diff
+ // src/models/user.ts
+ import { v4 as uuidv4 } from 'uuid';
+
+ export interface User {
+   id: string;
+   email: string;
+   passwordHash: string;
+   createdAt: Date;
+   updatedAt: Date;
+ }
```

## Step 7: Create PRs in Order

For features with dependencies, create PRs strategically:

### Option A: Separate PRs (Recommended for Review)

1. Create PR for User Model first (no dependencies)
2. Create PR for Auth Middleware (depends on nothing)
3. Create PR for Auth Routes (may depend on model)
4. Create PR for Tests (depends on implementation)

For each completed instance:
1. Select it
2. Press `x`
3. Choose "Create PR"

### Option B: Single Combined PR

If you prefer one PR:

1. Stop all instances with `x` but choose "Keep branch"
2. Manually merge branches:
```bash
git checkout main
git merge claudio/abc123-user-model
git merge claudio/def456-auth-routes
# ... etc
```
3. Create a single PR from the combined work

## Step 8: Handle Merge Conflicts

If rebasing causes conflicts:

1. Note the instance ID and conflict
2. Navigate to the worktree:
```bash
cd .claudio/worktrees/<instance-id>
```
3. Resolve conflicts:
```bash
git status  # See conflicting files
# Edit files to resolve
git add <resolved-files>
git rebase --continue
```
4. Return and retry PR:
```bash
cd ../../../
claudio pr <instance-id>
```

## Step 9: Clean Up

After all PRs are created:

```bash
claudio cleanup
```

This removes:
- Worktrees for completed instances
- Local branches that have been merged
- Stale tmux sessions

## Tips for Parallel Feature Development

### Task Independence

Design tasks to minimize file overlap:

```
Good Decomposition:
├── User model (src/models/)
├── Auth routes (src/routes/)
├── Middleware (src/middleware/)
└── Tests (tests/)

Problematic Decomposition:
├── Login feature (touches everything)
├── Registration feature (touches everything)
└── Password reset (touches everything)
```

### Shared Files Strategy

For files that multiple instances might touch (like `index.ts`):

1. Have one instance handle it
2. Or accept you'll merge changes manually
3. Use specific instructions: "Don't modify index.ts, I'll handle imports"

### Progress Tracking

With multiple instances, use the TUI effectively:
- Keep an eye on the sidebar for state changes
- Press `c` periodically to check conflicts
- Review diffs before creating PRs

### Cost Awareness

Running 4 instances costs ~4x a single instance:
- Check `claudio stats` for running costs
- Set `resources.cost_warning_threshold` in config
- Set `resources.cost_limit` to auto-pause at a limit

## What You Learned

- Breaking features into parallelizable tasks
- Running multiple instances for different components
- Monitoring progress and handling conflicts
- Creating multiple PRs from parallel work
- Managing dependencies between tasks

## Next Steps

- [Large Refactor Tutorial](large-refactor.md) - Coordinate major changes
- [Code Review Workflow](code-review-workflow.md) - Use Claudio for reviews
- [Configuration Guide](../guide/configuration.md) - Set up PR automation

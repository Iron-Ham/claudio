# Code Review Workflow: Using Claudio for Parallel Review Tasks

**Time: 10 minutes**

This tutorial shows how to use Claudio to perform multiple code review tasks simultaneously, such as checking for bugs, security issues, and documentation.

## Scenario

You have a pull request to review. Instead of doing a single pass, you'll run multiple specialized review instances in parallel.

## Prerequisites

- Claudio initialized (`claudio init`)
- A branch or PR to review

## Step 1: Prepare for Review

First, check out the branch you want to review:

```bash
git fetch origin
git checkout feature/user-dashboard
```

Or for a PR:
```bash
gh pr checkout 123
```

## Step 2: Start a Review Session

```bash
claudio start code-review
```

## Step 3: Create Specialized Review Instances

Add instances with focused review tasks:

**Security Review:**
```
Review all files for security issues: SQL injection, XSS, authentication bypasses,
sensitive data exposure, and insecure dependencies. List all findings with file
locations and severity levels.
```

**Bug Detection:**
```
Analyze the code for potential bugs: null pointer issues, race conditions,
off-by-one errors, unhandled edge cases, and logic errors. Report findings
with specific line numbers.
```

**Performance Review:**
```
Review for performance issues: N+1 queries, unnecessary re-renders,
missing memoization, inefficient algorithms, and memory leaks. Suggest optimizations.
```

**Documentation Check:**
```
Check that all public functions have documentation, API endpoints are documented,
README is updated for new features, and inline comments explain complex logic.
List any missing documentation.
```

## Step 4: Let Reviews Run

Each instance analyzes the code from its specialized perspective. Monitor progress:

- Press `1-4` to switch between instances
- Watch for `[completed]` status
- Note any `[waiting_input]` for questions

## Step 5: Collect Findings

As instances complete, review their output:

1. Select each instance
2. Scroll through findings with `j`/`k`
3. Use `/` to search for specific keywords:
   - `/severity: high`
   - `/line \d+`
   - `/TODO`

## Step 6: Export Results

To save the review output:

```bash
# View status with output
claudio status --verbose > review-results.txt
```

Or manually copy from the TUI output.

## Step 7: Create Review Comments

If you want Claude to help create PR comments:

Add a new instance:
```
Based on the code review, create GitHub PR review comments in markdown format.
Group by file and include line numbers. Format as:

## filename.ts

### Line 42
**Issue:** Description
**Suggestion:** How to fix
```

## Example Review Output

**Security Instance:**
```
Security Review Findings:

1. HIGH: SQL Injection in src/db/queries.ts:47
   - Raw string interpolation in query
   - Fix: Use parameterized queries

2. MEDIUM: Missing CSRF token in src/routes/form.ts:23
   - Form submission lacks CSRF protection
   - Fix: Add csrf middleware

3. LOW: Sensitive data in logs src/utils/logger.ts:15
   - Email addresses logged in plain text
   - Fix: Mask PII in logs
```

**Performance Instance:**
```
Performance Review Findings:

1. N+1 Query in src/api/users.ts:34
   - Each user triggers separate query for profile
   - Fix: Use eager loading or batch query

2. Missing useMemo in src/components/Dashboard.tsx:89
   - Expensive computation on every render
   - Fix: Wrap in useMemo with proper deps
```

## Tips for Effective Reviews

### Task Specialization

Focused tasks give better results:

```
Good: "Check authentication code for security issues"
Bad: "Review the code"
```

### Provide Context

Help Claude understand what to look for:

```
Review the new payment processing code for PCI compliance issues.
We handle credit card data and must not store CVV. Check for:
- Card data in logs
- Unencrypted storage
- Data exposure in error messages
```

### Scope Appropriately

For large codebases, scope the review:

```
Review files in src/api/ for REST API best practices including
proper status codes, error handling, and request validation.
```

### Use Multiple Perspectives

Different "reviewers" catch different issues:

| Instance | Focus | Catches |
|----------|-------|---------|
| Security | Vulnerabilities | Injection, auth issues |
| Performance | Speed | Slow queries, memory |
| Maintainability | Code quality | Complexity, patterns |
| Testing | Coverage | Missing tests, edge cases |

## Advanced: Automated Review Pipeline

Set up config for review workflows:

```yaml
# config.yaml
completion:
  default_action: keep_branch  # Don't auto-PR for reviews

pr:
  auto_pr_on_stop: false  # Manual control

resources:
  cost_limit: 5.00  # Cap review costs
```

Then script the review:

```bash
#!/bin/bash
# review.sh

claudio init 2>/dev/null || true
claudio add "Security review of changes in $(git diff --name-only main)"
claudio add "Bug detection in modified files"
claudio add "Check test coverage for changed code"

echo "Review instances started. Use 'claudio start' to monitor."
```

## What You Learned

- Using parallel instances for specialized reviews
- Creating focused review tasks
- Collecting and organizing findings
- Scaling code review with multiple perspectives

## Next Steps

- [Feature Development Tutorial](feature-development.md) - Build features in parallel
- [Large Refactor Tutorial](large-refactor.md) - Coordinate refactoring
- [Configuration Guide](../guide/configuration.md) - Customize for your workflow

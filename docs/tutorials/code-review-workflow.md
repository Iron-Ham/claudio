# Code Review Workflow: Using Claudio for Parallel Review Tasks

**Time: 15 minutes**

This tutorial shows how to use Claudio to perform multiple code review tasks simultaneously, such as checking for bugs, security issues, and documentation. You'll learn both standalone review sessions and how to run reviews alongside an active implementation session.

## Scenario

You have a pull request to review. Instead of doing a single pass, you'll run multiple specialized review instances in parallel—each with a focused perspective (security, performance, style, etc.).

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

---

## Parallel Review with Specialized Agents

Claudio's parallel review system uses a **supervisor pattern** where multiple specialized review agents observe changes from an implementation session and provide real-time feedback.

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                      Claudio Orchestrator                       │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    Session Manager                       │   │
│  │  - Session locking (PID-based, stale-safe)              │   │
│  │  - Multi-session support with isolated worktrees        │   │
│  └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│           ┌──────────────────┼──────────────────┐              │
│           ▼                  ▼                  ▼              │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐       │
│  │  Implementer │   │   Security   │   │  Performance │       │
│  │   Instance   │   │   Reviewer   │   │   Reviewer   │       │
│  │              │   │              │   │              │       │
│  │ tmux session │   │ tmux session │   │ tmux session │       │
│  └──────────────┘   └──────────────┘   └──────────────┘       │
│         │                  │                  │                │
│         └──────────────────┼──────────────────┘                │
│                            ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                   Callback System                        │   │
│  │  - StateChangeCallback (waiting, working, completed)    │   │
│  │  - MetricsChangeCallback (tokens, cost tracking)        │   │
│  │  - TimeoutCallback (activity, completion, stale)        │   │
│  │  - BellCallback (user input needed)                     │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Linking Review to Implementation Sessions

You can run a review session that watches changes from a separate implementation session. The sessions communicate through Claudio's shared orchestrator and file system.

**Terminal 1: Start Implementation Session**
```bash
# Start an implementation session
claudio start implementation

# Add your implementation task
# Press 'a' then type your task
```

**Terminal 2: Start Parallel Review Session**
```bash
# Start a review session in a separate terminal
claudio start --new review-session

# Add review agents that watch the implementation
# Press 'a' to add each specialized reviewer
```

### Session Communication Protocol

Sessions communicate through several mechanisms:

1. **Shared Worktree Access**: Both sessions can read from the same git worktree
2. **Session Discovery**: Use `claudio sessions` to list active sessions
3. **Session State Files**: Located in `.claudio/sessions/{sessionID}/`
4. **Tmux Session Naming**: Format `claudio-{sessionID}-{instanceID}` prevents collisions

```bash
# List all active sessions
claudio sessions

# Attach to a specific session by ID
claudio start --session abc12345

# View session details
claudio status --session abc12345
```

### Watch Mode

Claudio's **watch mode** continuously monitors instance output and triggers callbacks when significant events occur.

**How Watch Mode Works:**

1. **Ring Buffer Capture**: Output captured from tmux at configurable intervals (default: 100ms)
2. **State Detection**: Analyzes output patterns to detect waiting states
3. **Metrics Parsing**: Extracts token counts and cost information
4. **Timeout Detection**: Monitors for stuck or idle instances
5. **Bell Detection**: Forwards terminal bells when user input is needed

**Watch Mode Configuration:**

```yaml
# config.yaml
instance:
  # Output buffer size in bytes
  output_buffer_size: 100000  # 100KB

  # How often to poll tmux for output (milliseconds)
  capture_interval_ms: 100

  # Timeout after no new output (minutes, 0 = disabled)
  activity_timeout_minutes: 30

  # Maximum total runtime (minutes, 0 = disabled)
  completion_timeout_minutes: 120

  # Detect stuck instances via repeated output patterns
  stale_detection: true
```

**Watch Mode States:**

| State | Description | Indicator |
|-------|-------------|-----------|
| `working` | Instance actively producing output | Normal operation |
| `waiting_input` | Instance waiting for user input | Bell notification |
| `completed` | Instance finished its task | Auto-detected |
| `stuck` | Repeated identical output detected | Timeout warning |
| `timeout` | Runtime limits exceeded | Paused automatically |

### Specialized Review Agents

Create focused review agents for comprehensive coverage:

**Security Agent:**
```
You are a security-focused code reviewer. Continuously monitor the codebase for:
- OWASP Top 10 vulnerabilities
- Authentication/authorization flaws
- Data exposure risks
- Dependency vulnerabilities

Report findings immediately with severity ratings (CRITICAL/HIGH/MEDIUM/LOW).
Format: [SEVERITY] file:line - Description
```

**Performance Agent:**
```
You are a performance-focused code reviewer. Watch for:
- N+1 query patterns
- Unnecessary re-renders
- Missing memoization
- Inefficient algorithms
- Memory leaks

Report with impact assessment (HIGH/MEDIUM/LOW).
Format: [IMPACT] file:line - Issue and suggested fix
```

**Style Agent:**
```
You are a code style reviewer. Ensure consistency with:
- Project coding standards
- Naming conventions
- Code organization
- Documentation completeness

Report deviations and suggest corrections.
Format: file:line - Style issue and recommendation
```

---

## Output Formatters

Claudio supports different output formats for review findings.

### Standard Format

Default human-readable output:

```
Security Review Findings:

1. HIGH: SQL Injection in src/db/queries.ts:47
   - Raw string interpolation in query
   - Fix: Use parameterized queries

2. MEDIUM: Missing CSRF token in src/routes/form.ts:23
   - Form submission lacks CSRF protection
   - Fix: Add csrf middleware
```

### JSON Format

Structured output for tooling integration:

```json
{
  "findings": [
    {
      "severity": "HIGH",
      "type": "SQL_INJECTION",
      "file": "src/db/queries.ts",
      "line": 47,
      "description": "Raw string interpolation in query",
      "suggestion": "Use parameterized queries"
    }
  ],
  "summary": {
    "total": 3,
    "critical": 0,
    "high": 1,
    "medium": 2,
    "low": 0
  }
}
```

### Markdown Format

GitHub-ready review comments:

```markdown
## Security Review

### src/db/queries.ts

#### Line 47 - SQL Injection (HIGH)
**Issue:** Raw string interpolation in query
**Suggestion:** Use parameterized queries

```diff
- const query = `SELECT * FROM users WHERE id = ${userId}`;
+ const query = 'SELECT * FROM users WHERE id = ?';
+ db.query(query, [userId]);
```

---

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

---

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

---

## Advanced: Review Configuration

### Complete Review Configuration Example

```yaml
# config.yaml - Review-optimized configuration

completion:
  # Keep branches for review - don't auto-merge
  default_action: keep_branch

tui:
  # Auto-focus instances that need input
  auto_focus_on_input: true
  # More output lines for detailed reviews
  max_output_lines: 2000

instance:
  # Larger buffer for comprehensive review output
  output_buffer_size: 200000  # 200KB
  # Fast polling for responsive watch mode
  capture_interval_ms: 100
  # Wider tmux for code display
  tmux_width: 200
  tmux_height: 50
  # Longer timeouts for thorough reviews
  activity_timeout_minutes: 45
  completion_timeout_minutes: 180
  # Detect stuck review processes
  stale_detection: true

pr:
  # Don't auto-create PRs for reviews
  auto_pr_on_stop: false
  # Reviewers for review findings
  reviewers:
    default: [tech-lead]
    by_path:
      "src/security/**": [security-team]
      "src/api/**": [api-team]

resources:
  # Cap review costs
  cost_warning_threshold: 5.00
  cost_limit: 20.00
  # Show metrics for cost tracking
  show_metrics_in_sidebar: true
```

### Automated Review Pipeline

Script parallel reviews:

```bash
#!/bin/bash
# review.sh - Automated parallel code review

set -e

# Initialize if needed
claudio init 2>/dev/null || true

# Get changed files
CHANGED_FILES=$(git diff --name-only main)

# Start specialized review instances
claudio add "Security review of: $CHANGED_FILES. Check for OWASP Top 10 vulnerabilities."
claudio add "Performance review of: $CHANGED_FILES. Identify N+1 queries, missing caching, inefficient algorithms."
claudio add "Style review of: $CHANGED_FILES. Check coding standards and documentation."
claudio add "Test coverage analysis for: $CHANGED_FILES. Identify untested code paths."

echo "Review instances started. Use 'claudio start' to monitor."
echo "Or run 'claudio status --verbose' to check progress."
```

### CI Integration

Run reviews in CI/CD:

```yaml
# .github/workflows/review.yml
name: Code Review

on:
  pull_request:
    types: [opened, synchronize]

jobs:
  review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Install Claudio
        run: |
          curl -fsSL https://claudio.dev/install.sh | sh

      - name: Run Parallel Review
        run: |
          claudio init
          claudio add "Security review of PR changes"
          claudio add "Performance review of PR changes"
          # Wait for completion
          claudio wait --timeout 30m

      - name: Export Results
        run: |
          claudio status --verbose > review-results.md

      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const results = fs.readFileSync('review-results.md', 'utf8');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: results
            });
```

---

## Design Decisions: Parallel Review Architecture

This section explains key architectural decisions in Claudio's parallel review system.

### Why Ring Buffer for Output Capture?

The instance manager uses a ring buffer (`RingBuffer`) rather than unbounded storage for capturing tmux output:

1. **Bounded Memory**: Reviews can produce extensive output. A 100KB default buffer prevents memory exhaustion during long-running review sessions.
2. **Recent Context Priority**: Ring buffers naturally discard oldest content, keeping the most recent (and relevant) output available.
3. **Performance**: Fixed-size buffers avoid allocation overhead from continuous growth.

### Why Callback-Based State Detection?

The system uses callbacks (`StateChangeCallback`, `MetricsChangeCallback`, etc.) rather than polling:

1. **Decoupling**: The TUI doesn't need to understand instance internals—it just responds to state changes.
2. **Efficiency**: Only propagates meaningful events, not every output capture.
3. **Extensibility**: New listeners (logging, metrics aggregation) can be added without modifying core logic.

### Why Session-Scoped Tmux Naming?

Tmux sessions are named `claudio-{sessionID}-{instanceID}` instead of just `claudio-{instanceID}`:

1. **Multi-Session Support**: Multiple Claudio sessions can run simultaneously without tmux name collisions.
2. **Session Discovery**: Easy to identify which tmux sessions belong to which Claudio session.
3. **Cleanup Safety**: Session cleanup can target only sessions from a specific Claudio session.

### Why PID-Based Session Locking?

Session locks include the owning process PID rather than just a lock file:

1. **Stale Lock Detection**: If the owning process dies, the lock can be detected as stale and safely claimed.
2. **Distributed Safety**: Hostname is included to detect locks from other machines (networked filesystems).
3. **Recovery**: Enables session recovery without manual intervention after crashes.

### Why Stale Detection via Output Patterns?

The `StaleDetection` feature monitors for repeated identical output:

1. **Stuck Instance Detection**: Catches Claude instances stuck in loops producing identical output.
2. **False Positive Prevention**: High threshold (3000 iterations = ~5 minutes) allows for legitimate thinking time.
3. **Resource Protection**: Prevents runaway instances from consuming API credits indefinitely.

### Why Separate Watch Mode States?

Watch mode distinguishes between `working`, `waiting_input`, `completed`, `stuck`, and `timeout`:

1. **User Attention Routing**: Bell notifications only fire for `waiting_input`, not every state change.
2. **Progress Tracking**: TUI can display appropriate indicators for each state.
3. **Automated Actions**: Different states can trigger different automated responses (pause on timeout, alert on stuck).

---

## What You Learned

- Using parallel instances for specialized reviews
- Creating focused review tasks
- Understanding the supervisor pattern architecture
- Linking review sessions to implementation sessions
- Configuring watch mode for real-time monitoring
- Using different output formatters
- Setting up automated review pipelines

## Next Steps

- [Feature Development Tutorial](feature-development.md) - Build features in parallel
- [Large Refactor Tutorial](large-refactor.md) - Coordinate refactoring
- [Configuration Guide](../guide/configuration.md) - Customize for your workflow
- [Ultra-Plan Mode](../guide/ultra-plan.md) - Orchestrated multi-task execution

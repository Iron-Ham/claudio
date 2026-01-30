# Adversarial Review Mode

Adversarial mode creates an iterative feedback loop between an IMPLEMENTER and a REVIEWER. The implementer works on a task and submits their work, then the reviewer critically examines it and provides feedback. This loop continues until the reviewer approves the implementation or maximum iterations are reached.

## Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                     ADVERSARIAL REVIEW MODE                      │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                      FEEDBACK LOOP                          │ │
│  │                                                             │ │
│  │   ┌──────────────┐         ┌──────────────┐                │ │
│  │   │ IMPLEMENTER  │         │   REVIEWER   │                │ │
│  │   │              │         │              │                │ │
│  │   │  Works on    │ submit  │  Examines    │                │ │
│  │   │  the task    │────────▶│  the work    │                │ │
│  │   │              │         │              │                │ │
│  │   │  Addresses   │◀────────│  Provides    │                │ │
│  │   │  feedback    │ feedback│  feedback    │                │ │
│  │   └──────────────┘         └──────────────┘                │ │
│  │           │                        │                       │ │
│  │           │                        │                       │ │
│  │           ▼                        ▼                       │ │
│  │   Writes incremental        Writes review                  │ │
│  │   file when ready           file with score                │ │
│  │                                                            │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                             │                                    │
│                             ▼                                    │
│            Loop until: approved AND score >= threshold           │
│                    OR max iterations reached                     │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Start adversarial review with a task
claudio adversarial "Implement user authentication with JWT"

# Limit review cycles
claudio adversarial --max-iterations 5 "Refactor the API layer"

# Require higher quality threshold
claudio adversarial --min-passing-score 9 "Implement security-critical feature"
```

## CLI Options

```bash
claudio adversarial [task] [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--max-iterations` | Maximum implement-review cycles (0 = unlimited) | 10 |
| `--min-passing-score` | Minimum score (1-10) required for approval | 8 |

### Examples

```bash
# Basic adversarial review
claudio adversarial "Add input validation to all API endpoints"

# Strict quality requirements
claudio adversarial --min-passing-score 10 "Implement encryption module"

# Quick iteration for simple tasks
claudio adversarial --max-iterations 3 "Fix pagination bug"

# Combined flags for critical code
claudio adversarial --max-iterations 5 --min-passing-score 9 "Implement auth tokens"
```

## Configuration

Adversarial mode can be configured in your config file:

```yaml
adversarial:
  # Maximum number of implement-review cycles (0 = unlimited)
  max_iterations: 10

  # Minimum score for approval (1-10)
  min_passing_score: 8
```

## How It Works

### The Feedback Loop

Each round follows this sequence:

```
Round N
┌─────────────────────────────────────────────────────────────────┐
│                                                                  │
│  1. IMPLEMENTER works on the task                               │
│     • First round: starts fresh                                  │
│     • Later rounds: addresses reviewer feedback                  │
│                                                                  │
│  2. IMPLEMENTER signals completion                               │
│     • Writes .claudio-adversarial-incremental.json              │
│     • Sets status: "ready_for_review"                           │
│                                                                  │
│  3. REVIEWER examines the work                                   │
│     • Reviews code changes                                       │
│     • Evaluates quality, correctness, completeness               │
│                                                                  │
│  4. REVIEWER provides feedback                                   │
│     • Writes .claudio-adversarial-review.json                   │
│     • Assigns score (1-10)                                       │
│     • Lists issues and suggestions                               │
│     • Decides: approved or needs more work                       │
│                                                                  │
│  5. DECISION POINT                                               │
│     • If approved AND score >= min_passing_score: SUCCESS        │
│     • If not approved OR score < threshold: Continue to N+1      │
│     • If max iterations reached: STOP                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Sentinel Files

Communication between implementer and reviewer happens through JSON files:

#### Incremental File (`.claudio-adversarial-incremental.json`)

Written by the IMPLEMENTER when ready for review:

```json
{
  "round": 1,
  "status": "ready_for_review",
  "summary": "Implemented JWT authentication with login and refresh endpoints",
  "files_modified": [
    "src/auth/jwt.ts",
    "src/routes/auth.ts",
    "src/middleware/authenticate.ts"
  ],
  "approach": "Used RS256 algorithm with rotating keys, implemented token refresh with sliding expiration",
  "notes": "Considered symmetric keys but chose RS256 for better security in distributed systems"
}
```

| Field | Description |
|-------|-------------|
| `round` | Current iteration number |
| `status` | Either `ready_for_review` or `failed` |
| `summary` | What was implemented this round |
| `files_modified` | List of changed files |
| `approach` | Description of the approach taken |
| `notes` | Additional context or concerns |

#### Review File (`.claudio-adversarial-review.json`)

Written by the REVIEWER after examination:

```json
{
  "round": 1,
  "approved": false,
  "score": 7,
  "strengths": [
    "Good use of RS256 algorithm",
    "Clean middleware implementation",
    "Proper error handling"
  ],
  "issues": [
    "Token refresh doesn't invalidate old tokens",
    "Missing rate limiting on login endpoint",
    "No logging of authentication failures"
  ],
  "suggestions": [
    "Add token blacklist for revocation",
    "Consider implementing brute force protection",
    "Add audit logging"
  ],
  "summary": "Solid foundation but security gaps need addressing before approval",
  "required_changes": [
    "Implement token revocation mechanism",
    "Add rate limiting to prevent brute force"
  ]
}
```

| Field | Description |
|-------|-------------|
| `round` | Matching iteration number |
| `approved` | Boolean - is the work acceptable? |
| `score` | Quality score from 1-10 |
| `strengths` | What was done well |
| `issues` | Problems that need fixing |
| `suggestions` | Optional improvements |
| `summary` | Overall assessment |
| `required_changes` | Must-fix items for next round |

### Score Enforcement

The `min_passing_score` creates a quality gate:

- If `approved: true` but `score < min_passing_score`: approval is overridden to `false`
- This ensures that even if the reviewer is lenient, minimum quality standards are met
- The implementer must address issues until the score threshold is reached

## TUI Interface

### Adversarial Header

The header displays:
- Current phase (Implementing, Reviewing, Approved, Complete, Failed)
- Current round number
- Score from last review (if available)

### Adversarial Sidebar

```
┌─ Adversarial Review ─────────┐
│                              │
│  Round: 2/10                 │
│  Last Score: 7/10            │
│                              │
│ ┌─ Implementer ───────────┐  │
│ │ ● Working...            │  │
│ │                         │  │
│ │   Addressing:           │  │
│ │   • Token revocation    │  │
│ │   • Rate limiting       │  │
│ └─────────────────────────┘  │
│                              │
│ ┌─ Reviewer ──────────────┐  │
│ │ ○ Waiting for changes   │  │
│ └─────────────────────────┘  │
│                              │
└──────────────────────────────┘
```

### Round Organization

**Auto-Collapse:** Completed rounds automatically collapse into sub-groups, keeping the sidebar clean while preserving access to round history. The final approved round remains expanded.

**Previous Rounds Container:** When round 2+ starts, previous rounds are condensed into a single "Previous Rounds" container group. This reduces sidebar clutter when tasks span many rounds. You'll see only two groups:
- "Previous Rounds" (collapsed) - Contains all completed rounds
- Current round (expanded) - Shows active implementer/reviewer

You can expand "Previous Rounds" to access any historical round's details.

### Viewing History

Review previous rounds to understand the evolution:
- Each round's feedback is preserved
- See how issues were addressed
- Track score progression
- Expand collapsed rounds using `gc` key

## When to Use Adversarial Mode

### Ideal Scenarios

**Security-Critical Code**
```bash
claudio adversarial --min-passing-score 9 "Implement password hashing"
```
Multiple review rounds catch security issues.

**Complex Business Logic**
```bash
claudio adversarial "Implement order fulfillment workflow"
```
Iterative refinement ensures all edge cases are handled.

**API Design**
```bash
claudio adversarial "Design and implement the user management API"
```
Reviewer can catch design issues early and request changes.

**Code Quality Improvement**
```bash
claudio adversarial "Refactor the payment processing module"
```
Ensures refactoring improves quality, not just changes code.

### When NOT to Use Adversarial Mode

- Simple bug fixes with obvious solutions
- Trivial changes where review overhead isn't justified
- Time-sensitive tasks where iteration isn't possible
- When you need parallel exploration (use [TripleShot](tripleshot.md) instead)

## Best Practices

### Setting Appropriate Thresholds

**High-stakes code:**
```bash
claudio adversarial --min-passing-score 9 --max-iterations 10 "..."
```

**Standard features:**
```bash
claudio adversarial --min-passing-score 8 --max-iterations 5 "..."
```

**Quick iterations:**
```bash
claudio adversarial --min-passing-score 7 --max-iterations 3 "..."
```

### Writing Good Task Descriptions

Provide context for both implementer and reviewer:

```bash
# Good: Clear requirements and constraints
claudio adversarial "Implement user session management.
  Requirements:
  - Secure session tokens (not sequential)
  - 30-minute timeout with activity-based extension
  - Support for multiple concurrent sessions
  - Proper logout that invalidates all sessions

  Constraints:
  - Must work with existing User model
  - Cannot change database schema"

# Avoid: Too vague
claudio adversarial "Add sessions"
```

### Handling Stuck Iterations

If the loop isn't converging:

1. **Check feedback quality** - Is the reviewer giving actionable feedback?
2. **Simplify requirements** - Task might be too complex
3. **Increase iterations** - Some tasks need more rounds
4. **Adjust threshold** - Score might be unrealistically high

## Advanced: Rejection After Approval

Even after approval, you can restart the review loop:

1. Session completes with approval
2. You notice an issue the reviewer missed
3. Have the reviewer write a new failing review file
4. Coordinator detects the new review and restarts the loop
5. Implementer addresses the newly identified issues

This allows post-approval refinement without starting over.

## Troubleshooting

### Implementer Not Completing

If the implementer seems stuck:
- Check for errors in the output
- Verify the task is achievable
- Look for unclear requirements

### Reviewer Too Strict

If approval never comes:
- Lower `min_passing_score` temporarily
- Review the required_changes - are they reasonable?
- Consider if the task is well-defined

### Reviewer Too Lenient

If approval comes too early:
- Increase `min_passing_score`
- Add more specific requirements to the task
- Consider using `--min-passing-score 10` for critical code

### Stuck in Loop

If iterations don't converge:
- Check that required_changes are being addressed
- Look for contradictory feedback
- Consider breaking into smaller tasks

### Stuck Instance Detection

Claudio automatically detects when an adversarial instance (implementer or reviewer) completes without writing its required sentinel file. When this happens:

1. The workflow transitions to a "stuck" phase
2. The TUI sidebar shows which role (implementer/reviewer) got stuck
3. You're notified with recovery options

**Recovery:**

Use the `:adversarial-retry` command in the TUI to restart the stuck role:

```
:adversarial-retry
```

This restarts the stuck implementer or reviewer with the appropriate context, allowing the workflow to continue.

**Common causes:**
- Claude finishes work but fails to write the completion file
- Network interruptions during file write
- Task ambiguity causing Claude to wait for clarification

## Example Workflow

### Step 1: Start Adversarial Review

```bash
claudio adversarial --max-iterations 5 --min-passing-score 8 \
  "Implement rate limiting for all API endpoints using token bucket algorithm"
```

### Step 2: Round 1 - Initial Implementation

**Implementer produces:**
- Basic rate limiter middleware
- Configuration for rate limits
- Integration with API routes

**Reviewer feedback:**
```
Score: 6/10
Issues:
- No per-user rate limiting (only global)
- No distributed support (single server only)
- Missing bypass for internal services
Required changes:
- Add user-specific buckets
- Plan for Redis backend
```

### Step 3: Round 2 - Addressing Feedback

**Implementer improves:**
- Adds per-user rate limiting with user ID keys
- Adds Redis backend option with fallback
- Implements whitelist for internal IPs

**Reviewer feedback:**
```
Score: 8/10
Strengths:
- Per-user limiting works well
- Redis integration is clean
Issues:
- Whitelist IPs should be configurable
- Need tests for edge cases
Approved: true
```

### Step 4: Complete

Loop exits with approval (score 8 >= threshold 8).

Final implementation has:
- Per-user rate limiting
- Redis backend support
- Configurable IP whitelist
- Reviewed and refined code

## See Also

- [TripleShot Mode](tripleshot.md) - Parallel exploration with judge evaluation
- [Ultra-Plan Mode](ultra-plan.md) - For complex tasks that need decomposition
- [CLI Reference](../reference/cli.md) - Complete command reference

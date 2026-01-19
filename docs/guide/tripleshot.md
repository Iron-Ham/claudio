# TripleShot Mode

TripleShot runs three Claude instances in parallel on the same task, then uses a fourth "judge" instance to evaluate all solutions and determine the best approach. This mode is ideal when you want multiple perspectives on a problem or when the optimal solution isn't clear.

## Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                       TRIPLESHOT MODE                            │
├──────────────────────────────────────────────────────────────────┤
│  Phase 1: WORKING (Parallel Execution)                           │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │  Attempt 0   │  │  Attempt 1   │  │  Attempt 2   │           │
│  │              │  │              │  │              │           │
│  │  Works on    │  │  Works on    │  │  Works on    │           │
│  │  task with   │  │  task with   │  │  task with   │           │
│  │  approach A  │  │  approach B  │  │  approach C  │           │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘           │
│         │                 │                 │                    │
│         └─────────────────┼─────────────────┘                    │
│                           ▼                                      │
│  Phase 2: EVALUATING                                             │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                        JUDGE                                │ │
│  │  • Reviews all 3 solutions                                  │ │
│  │  • Scores on: correctness, quality, completeness, testing   │ │
│  │  • Selects winner OR recommends merge strategy              │ │
│  └────────────────────────────────────────────────────────────┘ │
│                           │                                      │
│                           ▼                                      │
│  Phase 3: COMPLETE                                               │
│  Winner selected or merge strategy applied                       │
└──────────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
# Start tripleshot with a task
claudio tripleshot "Implement a rate limiter for the API endpoints"

# Auto-approve the winning solution
claudio tripleshot --auto-approve "Optimize the database queries"
```

## CLI Options

```bash
claudio tripleshot [task] [flags]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--auto-approve` | Automatically apply winning solution | false |

### Examples

```bash
# Basic tripleshot
claudio tripleshot "Create a caching layer for API responses"

# Complex optimization task
claudio tripleshot "Optimize the user search algorithm for large datasets"

# Refactoring with multiple valid approaches
claudio tripleshot "Refactor the authentication module for better testability"

# Auto-approve for automated workflows
claudio tripleshot --auto-approve "Implement input validation"
```

## How It Works

### Phase 1: Working

Three instances start simultaneously, each working on the same task independently:

1. **Attempt 0** - Works in its own worktree
2. **Attempt 1** - Works in its own worktree
3. **Attempt 2** - Works in its own worktree

Each instance receives variant instructions encouraging different approaches. When an attempt completes, it writes a completion file (`.claudio-tripleshot-complete.json`) containing:

```json
{
  "attempt_index": 0,
  "status": "complete",
  "summary": "Implemented rate limiter using token bucket algorithm",
  "files_modified": ["src/middleware/ratelimiter.ts", "src/config/limits.ts"],
  "approach": "Used token bucket for smooth rate limiting with Redis backend",
  "notes": "Considered sliding window but token bucket handles bursts better"
}
```

### Phase 2: Evaluating

Once all three attempts complete, the judge instance activates. The judge:

1. Reviews all three completion summaries
2. Examines the actual code changes in each worktree
3. Evaluates each solution on four criteria:
   - **Correctness** - Does it solve the problem?
   - **Code Quality** - Clean, maintainable, follows patterns?
   - **Completeness** - Handles edge cases?
   - **Testing** - Are changes testable/tested?
4. Assigns a score (1-10) to each solution
5. Decides on a strategy: select, merge, or combine

The judge writes an evaluation file (`.claudio-tripleshot-evaluation.json`):

```json
{
  "winner_index": 1,
  "merge_strategy": "select",
  "reasoning": "Attempt 1 provides the most maintainable solution with proper error handling and comprehensive tests. While Attempt 0 had better performance characteristics, it sacrificed readability.",
  "attempt_evaluations": [
    {
      "index": 0,
      "score": 7,
      "strengths": ["High performance", "Good algorithm choice"],
      "weaknesses": ["Complex implementation", "Limited error handling"]
    },
    {
      "index": 1,
      "score": 9,
      "strengths": ["Clean code", "Comprehensive tests", "Good documentation"],
      "weaknesses": ["Slightly lower performance than optimal"]
    },
    {
      "index": 2,
      "score": 6,
      "strengths": ["Simple implementation"],
      "weaknesses": ["Missing edge cases", "No tests"]
    }
  ],
  "suggested_changes": []
}
```

### Phase 3: Complete

Based on the judge's decision:

- **Select** (`winner_index >= 0`): The winning solution is ready for use
- **Merge** (`merge_strategy: "merge"`): Combine changes from multiple attempts
- **Combine** (`merge_strategy: "combine"`): Cherry-pick specific elements

## Merge Strategies

### Select (Default)

One solution is chosen as the winner. The winning worktree's changes can be:
- Committed and merged
- Used as the basis for a PR
- Further refined

### Merge

Multiple solutions are combined. The judge identifies complementary elements:
- Core implementation from one attempt
- Better tests from another
- Documentation from a third

### Combine

Specific elements are cherry-picked:
- A particular function from Attempt 0
- Error handling approach from Attempt 1
- Configuration structure from Attempt 2

## TUI Interface

### TripleShot Header

The header displays:
- Current phase (Working, Evaluating, Complete, Failed)
- Progress indicator for each attempt
- Judge status

### TripleShot Sidebar

The sidebar shows a grouped view:

```
┌─ TripleShot Session ─────────┐
│                              │
│ ┌─ Implementers ──────────┐  │
│ │ ● Attempt 0  [Working]  │  │
│ │ ● Attempt 1  [Working]  │  │
│ │ ● Attempt 2  [Complete] │  │
│ └─────────────────────────┘  │
│                              │
│ ┌─ Judge ─────────────────┐  │
│ │ ○ Judge      [Waiting]  │  │
│ └─────────────────────────┘  │
│                              │
└──────────────────────────────┘
```

Status indicators:
- `Working` - Actively implementing
- `Complete` - Finished, awaiting evaluation
- `Failed` - Encountered an error
- `Waiting` - Judge waiting for all attempts

### Viewing Solutions

Switch between attempt outputs to compare approaches:
- Use arrow keys to select different instances
- View diffs to see actual code changes
- Compare approaches side-by-side

## When to Use TripleShot

### Ideal Scenarios

**Algorithm Selection**
```bash
claudio tripleshot "Implement efficient search for the product catalog"
```
Different algorithms (binary search, hash map, trie) can be compared.

**Architecture Decisions**
```bash
claudio tripleshot "Refactor auth module - consider middleware vs decorator pattern"
```
Multiple architectural approaches evaluated objectively.

**Optimization Tasks**
```bash
claudio tripleshot "Optimize the report generation for large datasets"
```
Trade-offs between memory, speed, and readability become visible.

**Complex Implementations**
```bash
claudio tripleshot "Implement retry logic with exponential backoff and circuit breaker"
```
Complex logic benefits from multiple implementation attempts.

### When NOT to Use TripleShot

- Simple, straightforward tasks (single obvious approach)
- Bug fixes with clear solutions
- Routine code additions
- Tasks where 3x resource usage isn't justified

## Best Practices

### Writing Good Tasks

Be specific about what you want to achieve:

```bash
# Good: Clear scope with flexibility for approaches
claudio tripleshot "Implement caching for API responses.
  Consider TTL management, cache invalidation, and memory limits."

# Avoid: Too vague
claudio tripleshot "Make the API faster"
```

### Evaluating Results

After the judge evaluates:

1. **Review the scores** - Are they justified by the evaluation?
2. **Check the reasoning** - Does the logic make sense?
3. **Examine the code** - Sometimes manual review reveals issues
4. **Consider merge** - Even if one wins, others may have good elements

### Resource Considerations

TripleShot uses 4 Claude instances:
- 3 for implementation attempts
- 1 for evaluation

Factor this into cost estimates for large tasks.

## Troubleshooting

### Attempts Not Completing

If attempts don't finish:
- Check for infinite loops in task description
- Verify Claude Code is working correctly
- Look for errors in attempt output

### Judge Not Starting

If the judge doesn't start after all attempts complete:
- Verify all completion files were written
- Check that the completion status is `complete` not `failed`
- Look for coordinator errors

### Poor Quality Results

If results are lower quality than expected:
- Task may be too vague - add specificity
- Task may be too complex - break into smaller pieces
- Consider using [Ultra-Plan](ultra-plan.md) for decomposition first

### Merge Strategy Issues

If merge/combine doesn't produce expected results:
- The judge provides suggestions but doesn't execute merges
- You may need to manually cherry-pick elements
- Consider running another tripleshot on the integration task

## Example Workflow

### Step 1: Identify a Good Candidate Task

```bash
# Tasks with multiple valid approaches work best
claudio tripleshot "Implement user session management with proper security"
```

### Step 2: Monitor Progress

Watch the TUI as all three attempts work in parallel. Each will develop a different approach.

### Step 3: Review Judge Evaluation

```
Winner: Attempt 1 (Score: 9/10)

Reasoning: Attempt 1 provides the most secure implementation
with proper CSRF protection, session rotation, and comprehensive
test coverage. Attempt 0 was faster but lacked security headers.
Attempt 2 had good security but overcomplicated the API.

Suggested: Use Attempt 1's core implementation, but adopt
Attempt 0's session serialization for better performance.
```

### Step 4: Apply Solution

If using `--auto-approve`, the winner is applied automatically. Otherwise:

1. Review the winning solution in its worktree
2. Create a PR from that branch
3. Optionally incorporate suggested improvements

## See Also

- [Adversarial Review](adversarial.md) - Iterative implementation with reviewer feedback
- [Ultra-Plan Mode](ultra-plan.md) - For complex tasks that need decomposition
- [CLI Reference](../reference/cli.md) - Complete command reference

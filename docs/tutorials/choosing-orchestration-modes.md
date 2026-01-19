# Choosing the Right Orchestration Mode

Claudio offers several orchestration modes for different scenarios. This tutorial helps you choose the right mode for your task and walks through practical examples.

## Quick Decision Guide

```
                    What's your task?
                          │
           ┌──────────────┼──────────────┐
           │              │              │
      Complex task   Single task    Single task
      (many parts)   (clear goal)   (unclear best
           │              │          approach)
           ▼              │              │
       ┌───────┐          │              │
       │ plan/ │          │              │
       │ultra- │     Need quality    Multiple valid
       │ plan  │     iteration?      solutions?
       └───────┘          │              │
                    ┌─────┴─────┐   ┌────┴────┐
                    │           │   │         │
                   Yes         No  Yes       No
                    │           │   │         │
                    ▼           ▼   ▼         ▼
              ┌───────────┐ ┌───────┐ ┌──────────┐ ┌───────┐
              │adversarial│ │ basic │ │tripleshot│ │ basic │
              └───────────┘ └───────┘ └──────────┘ └───────┘
```

## Mode Comparison

| Mode | Instances | Best For | Tradeoff |
|------|-----------|----------|----------|
| **Basic** (`claudio start`) | 1+ manual | Simple tasks, full control | Manual coordination |
| **Plan** (`claudio plan`) | 1 | Task decomposition, GitHub issues | Planning only, no execution |
| **Ultra-Plan** (`claudio ultraplan`) | 1 coordinator + N workers | Large features, parallel work | More setup overhead |
| **TripleShot** (`claudio tripleshot`) | 3 + 1 judge | Uncertain approaches | 4x resource usage |
| **Adversarial** (`claudio adversarial`) | 1 implementer + 1 reviewer | Quality-critical code | Iterative, slower |

## Tutorial: Authentication System

Let's build a user authentication system using different modes to understand when each excels.

### Scenario 1: Quick Implementation (Basic Mode)

**Task:** Add a simple API key authentication middleware.

This is straightforward with one clear approach:

```bash
claudio start
# Press 'a' to add an instance
# Task: "Add API key authentication middleware that validates X-API-Key header"
```

**Why basic mode?**
- Single, focused task
- Clear implementation path
- No complex dependencies

### Scenario 2: Complex Feature (Ultra-Plan Mode)

**Task:** Implement full OAuth2 authentication with multiple providers.

This is complex with many interdependent parts:

```bash
claudio ultraplan "Implement OAuth2 authentication with:
  - Google and GitHub providers
  - User account linking
  - Session management
  - Profile data synchronization
  - Error handling and logging"
```

**Why ultra-plan?**
- Multiple independent subtasks (providers, sessions, linking)
- Can parallelize provider implementations
- Coordinated integration at the end

**What ultra-plan does:**
1. Creates a plan with tasks like:
   - `task-1`: Add OAuth2 dependencies
   - `task-2`: Create user model with provider fields
   - `task-3`: Implement Google OAuth
   - `task-4`: Implement GitHub OAuth
   - `task-5`: Create session management
   - `task-6`: Implement account linking
2. Executes tasks 1-2 first (dependencies)
3. Runs tasks 3-4 in parallel (no interdependency)
4. Continues with 5-6 after providers are ready
5. Synthesizes and identifies integration issues

### Scenario 3: Algorithm Choice (TripleShot Mode)

**Task:** Implement rate limiting for API endpoints.

Multiple valid algorithms exist:

```bash
claudio tripleshot "Implement rate limiting for API endpoints.
  The solution should handle:
  - Per-user limits
  - Distributed deployment (multi-server)
  - Burst allowance
  - Clear error responses"
```

**Why tripleshot?**
- Token bucket vs sliding window vs leaky bucket
- Each approach has different tradeoffs
- Want to compare implementations

**What tripleshot does:**
1. Three instances implement simultaneously:
   - **Attempt 0:** Token bucket with Redis backend
   - **Attempt 1:** Sliding window log algorithm
   - **Attempt 2:** Fixed window with burst allowance
2. Judge evaluates each on:
   - Correctness
   - Performance characteristics
   - Code quality
   - Maintainability
3. Selects winner or recommends merging best elements

### Scenario 4: Security-Critical Code (Adversarial Mode)

**Task:** Implement password hashing and verification.

Security requires careful review:

```bash
claudio adversarial --min-passing-score 9 \
  "Implement password hashing with:
    - Argon2id algorithm
    - Configurable work factors
    - Timing-safe comparison
    - Upgrade path for old hashes"
```

**Why adversarial?**
- Security bugs are costly
- Multiple review rounds catch issues
- High score threshold ensures thoroughness

**What adversarial does:**
1. **Round 1:** Implementer creates initial version
   - Reviewer: "Score 6/10 - Missing constant-time comparison"
2. **Round 2:** Implementer fixes timing issue
   - Reviewer: "Score 7/10 - Work factors should be configurable"
3. **Round 3:** Implementer adds configuration
   - Reviewer: "Score 9/10 - Approved. Solid implementation."

### Scenario 5: Planning Only (Plan Mode)

**Task:** Create a roadmap for payment integration.

You want tasks in GitHub Issues for team tracking:

```bash
claudio plan --output-format issues "Implement Stripe payment integration with:
  - Customer management
  - Subscription billing
  - One-time payments
  - Webhook handling
  - Invoice generation"
```

**Why plan mode?**
- Team needs visibility in GitHub
- Work will be done over time (not one session)
- Want to assign issues to different people

**What plan does:**
1. Creates parent issue: "Implement Stripe Payment Integration"
2. Creates child issues:
   - #101: Add Stripe dependencies and configuration
   - #102: Implement customer management
   - #103: Set up subscription billing
   - #104: Implement one-time payments
   - #105: Create webhook handlers
   - #106: Add invoice generation
3. Issues linked with dependencies

## Multi-Pass Planning: When to Use It

Both `plan` and `ultraplan` support `--multi-pass`:

```bash
# For plan
claudio plan --multi-pass "Refactor database layer"

# For ultraplan
claudio ultraplan --multi-pass "Redesign the API"
```

**Use multi-pass when:**
- Decomposition isn't obvious
- Multiple architectural approaches exist
- Task is high-stakes

**Skip multi-pass when:**
- Task structure is clear
- Speed matters more than perfect decomposition
- Simple feature additions

## Combining Modes

Modes can be combined for complex workflows:

### Plan + Ultra-Plan

Generate a plan, review it, then execute:

```bash
# Step 1: Generate and review plan
claudio plan --output-format json "Implement user profiles"

# Step 2: Edit .claudio-plan.json if needed

# Step 3: Execute the reviewed plan
claudio ultraplan --plan .claudio-plan.json
```

### Ultra-Plan + Adversarial (Manual)

Use ultra-plan for decomposition, then adversarial for critical subtasks:

```bash
# Step 1: Plan the overall feature
claudio ultraplan --dry-run "Implement payment system"

# Step 2: Execute non-critical tasks with ultraplan
claudio ultraplan --plan payments-plan.json

# Step 3: For security-critical parts, use adversarial
claudio adversarial --min-passing-score 9 "Implement payment token encryption"
```

## Checklist: Choosing Your Mode

### Use Basic Mode when:
- [ ] Single, focused task
- [ ] Clear implementation approach
- [ ] You want manual control
- [ ] Quick iteration needed

### Use Plan Mode when:
- [ ] Need task decomposition only
- [ ] Want GitHub Issues for tracking
- [ ] Team coordination required
- [ ] Work spans multiple sessions

### Use Ultra-Plan when:
- [ ] Large feature with multiple parts
- [ ] Tasks can be parallelized
- [ ] Want automated coordination
- [ ] Need synthesis phase

### Use TripleShot when:
- [ ] Multiple valid approaches
- [ ] Algorithm/pattern decision
- [ ] Want comparative evaluation
- [ ] 4x resource cost is acceptable

### Use Adversarial when:
- [ ] Security-critical code
- [ ] Quality is paramount
- [ ] Iterative refinement valuable
- [ ] Can accept slower completion

## Common Patterns

### Feature Development
```bash
claudio ultraplan "Implement feature X"
```

### Security Feature
```bash
claudio adversarial --min-passing-score 9 "Implement auth"
```

### Performance Optimization
```bash
claudio tripleshot "Optimize database queries"
```

### Team Roadmap
```bash
claudio plan --output-format issues "Q1 features"
```

### Complex Refactor
```bash
claudio ultraplan --multi-pass "Refactor to microservices"
```

## See Also

- [Plan Mode Guide](../guide/plan.md)
- [Ultra-Plan Guide](../guide/ultra-plan.md)
- [TripleShot Guide](../guide/tripleshot.md)
- [Adversarial Guide](../guide/adversarial.md)
- [CLI Reference](../reference/cli.md)

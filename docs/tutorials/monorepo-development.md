# Monorepo Development: Using Claudio with Large Codebases

**Time: 25-35 minutes**

This tutorial explains how to use Claudio effectively with monorepos, covering sparse checkout optimization, package targeting, build orchestration, and strategies for managing large codebases.

## Overview

Claudio's git worktree architecture provides powerful features for monorepo development:

- **Sparse checkout support**: Clone only the packages you need
- **Package targeting**: Assign instances to specific packages
- **Build orchestration**: Integrate with Turborepo, Nx, Bazel, etc.
- **Selective testing**: Run tests only for affected packages
- **Dependency awareness**: Respect inter-package dependencies

## Prerequisites

- Claudio initialized in your monorepo (`claudio init`)
- Build orchestration tool (Turborepo, Nx, Lerna, Bazel)
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))
- Understanding of your monorepo structure

## Understanding Monorepos and Git Worktrees

### Typical Monorepo Structure

```
monorepo/
├── apps/
│   ├── web/              # Main web application
│   ├── mobile/           # Mobile app
│   └── admin/            # Admin dashboard
├── packages/
│   ├── ui/               # Shared UI components
│   ├── config/           # Shared configuration
│   ├── utils/            # Utility functions
│   └── api-client/       # API client library
├── services/
│   ├── api/              # Backend API
│   ├── worker/           # Background worker
│   └── gateway/          # API gateway
├── package.json          # Root package.json
├── turbo.json           # Turborepo config
└── pnpm-workspace.yaml  # Workspace definition
```

### Worktree Considerations

Full monorepo worktrees can be:
- **Large**: Gigabytes of code
- **Slow to clone**: Many files to copy
- **Resource intensive**: Heavy dependency installation

**Key insight**: Use sparse checkout to include only the packages you need for each task.

## Strategy 1: Sparse Checkout (Recommended for Large Repos)

Best for: Monorepos with 50+ packages or large codebases.

### Configuration

Claudio supports sparse checkout for worktrees. Configure in tasks:

```
Set up sparse checkout for the web app:

git sparse-checkout init --cone
git sparse-checkout set apps/web packages/ui packages/utils

# Install dependencies
pnpm install --filter @myapp/web...
```

### Task Instructions with Sparse Checkout

**Task 1 - Web App:**
```
Implement the dashboard feature in apps/web.

Sparse checkout setup:
git sparse-checkout set apps/web packages/ui packages/config

Development:
1. Create apps/web/src/pages/Dashboard.tsx
2. Add UI components
3. Build: pnpm turbo run build --filter=@myapp/web
4. Test: pnpm turbo run test --filter=@myapp/web
```

**Task 2 - Shared UI:**
```
Add new Button variants to packages/ui.

Sparse checkout setup:
git sparse-checkout set packages/ui packages/config

Development:
1. Update packages/ui/src/Button.tsx
2. Add Storybook stories
3. Build: pnpm turbo run build --filter=@myapp/ui
4. Test: pnpm turbo run test --filter=@myapp/ui
```

### Worktree Script

Create a setup script for sparse worktrees:

```bash
#!/bin/bash
# scripts/sparse-worktree.sh

PACKAGES="$1"  # Space-separated list of packages

# Initialize sparse checkout
git sparse-checkout init --cone
git sparse-checkout set $PACKAGES

# Install only needed dependencies
pnpm install --filter "{${PACKAGES// /,}}..."

echo "Sparse worktree ready with: $PACKAGES"
```

Usage in tasks:
```
Set up worktree and implement feature:

./scripts/sparse-worktree.sh "apps/web packages/ui"

Then implement the feature...
```

## Strategy 2: Full Worktree with Targeted Builds

Best for: Smaller monorepos or when sparse checkout isn't practical.

### Workflow

```bash
claudio start monorepo-feature
```

**Task 1:**
```
Implement feature in @myapp/web app.

# Build only affected packages
pnpm turbo run build --filter=@myapp/web...

# Test only affected packages
pnpm turbo run test --filter=@myapp/web...
```

**Task 2:**
```
Update shared utilities in @myapp/utils.

# Build package and dependents
pnpm turbo run build --filter=@myapp/utils...

# Test package and dependents
pnpm turbo run test --filter=@myapp/utils...
```

### Turborepo Filtering

Use Turborepo's powerful filter syntax:

```bash
# Single package
turbo run build --filter=@myapp/web

# Package and its dependencies
turbo run build --filter=@myapp/web...

# Package and its dependents
turbo run build --filter=...@myapp/ui

# Package, dependencies, and dependents
turbo run build --filter=...@myapp/ui...

# Affected packages only
turbo run build --filter=[main]
```

### Nx Targeting

For Nx-based monorepos:

```bash
# Single project
nx build web

# Project and dependencies
nx build web --with-deps

# Affected projects
nx affected:build

# Run for all projects
nx run-many --target=build
```

## Strategy 3: Service/Team-Based Development

Best for: Large organizations with team ownership.

### Team Assignment

Assign instances based on team ownership:

**Frontend Team Instance:**
```
Work on apps/web and packages/ui:

pnpm turbo run build --filter=@myapp/web... --filter=@myapp/ui...
pnpm turbo run test --filter=@myapp/web... --filter=@myapp/ui...
```

**Backend Team Instance:**
```
Work on services/api and packages/api-client:

pnpm turbo run build --filter=@myapp/api... --filter=@myapp/api-client...
pnpm turbo run test --filter=@myapp/api... --filter=@myapp/api-client...
```

**Platform Team Instance:**
```
Work on shared infrastructure:

pnpm turbo run build --filter=@myapp/config... --filter=@myapp/utils...
pnpm turbo run test --filter=@myapp/config... --filter=@myapp/utils...
```

## Build Orchestration

### Turborepo Integration

Configure Turborepo for efficient builds:

```json
// turbo.json
{
  "$schema": "https://turbo.build/schema.json",
  "pipeline": {
    "build": {
      "dependsOn": ["^build"],
      "outputs": ["dist/**", ".next/**"]
    },
    "test": {
      "dependsOn": ["build"],
      "outputs": []
    },
    "lint": {
      "outputs": []
    }
  }
}
```

Task instructions:
```
Build the web app with caching:
pnpm turbo run build --filter=@myapp/web...

# Turbo automatically:
# - Builds dependencies first
# - Uses cache for unchanged packages
# - Parallelizes independent builds
```

### Nx Integration

Configure Nx for efficient execution:

```json
// nx.json
{
  "targetDefaults": {
    "build": {
      "dependsOn": ["^build"],
      "cache": true
    },
    "test": {
      "dependsOn": ["build"],
      "cache": true
    }
  }
}
```

Task instructions:
```
Build affected projects:
nx affected:build --base=main

# Nx automatically:
# - Computes affected graph
# - Builds in correct order
# - Uses computation cache
```

### Bazel Integration

For Bazel-based monorepos:

```
Build the target package:
bazel build //apps/web:all

Test the target:
bazel test //apps/web:all

# Query dependencies
bazel query 'deps(//apps/web:all)'
```

## Dependency Management

### pnpm Workspaces

pnpm is ideal for monorepo dependency management:

```yaml
# pnpm-workspace.yaml
packages:
  - 'apps/*'
  - 'packages/*'
  - 'services/*'
```

Install and filter:
```bash
# Install all
pnpm install

# Install for specific package and deps
pnpm install --filter @myapp/web...
```

### Yarn Workspaces

For Yarn workspaces:

```json
// package.json
{
  "workspaces": [
    "apps/*",
    "packages/*",
    "services/*"
  ]
}
```

### npm Workspaces

For npm workspaces:

```json
// package.json
{
  "workspaces": [
    "apps/*",
    "packages/*"
  ]
}
```

```bash
# Run in specific workspace
npm run build -w @myapp/web
```

## Testing Strategies

### Affected-Only Testing

Test only packages affected by changes:

**Turborepo:**
```
Run tests for affected packages:
pnpm turbo run test --filter=[main]
```

**Nx:**
```
Run tests for affected projects:
nx affected:test --base=main
```

### Package-Scoped Testing

Test specific packages:

**Task 1:**
```
Test the UI package:
pnpm turbo run test --filter=@myapp/ui
```

**Task 2:**
```
Test the web app and dependencies:
pnpm turbo run test --filter=@myapp/web...
```

### Integration Testing

For cross-package integration:

```
Run integration tests:
pnpm turbo run test:integration --filter=@myapp/web

# Or with specific scope
pnpm turbo run test:e2e --filter=@myapp/web
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| Root `package.json` | HIGH | One instance for root changes |
| `pnpm-lock.yaml` | HIGH | Regenerates automatically |
| `turbo.json` | MEDIUM | Coordinate pipeline changes |
| Package `package.json` | LOW | Different packages per instance |

### Task Design for Monorepos

**Good decomposition:**
```
├── apps/web          (Frontend team)
├── apps/mobile       (Mobile team)
├── packages/ui       (Design system team)
└── services/api      (Backend team)
```

**Risky decomposition:**
```
├── Add auth (touches apps/web, services/api, packages/auth)
├── Add billing (touches apps/web, services/api, packages/billing)
└── Update deps (touches all package.json files)
```

### Dependency Addition Strategy

**Option A: Pre-add to shared packages**
```
First, add shared dependencies to relevant packages:

cd packages/ui
pnpm add @radix-ui/react-dialog

# Let lock file regenerate
pnpm install
```

**Option B: Sequential additions**
```bash
claudio add "Add dialog dependency" --start
claudio add "Use dialog in UI package" --depends-on "abc123"
claudio add "Use dialog in web app" --depends-on "def456"
```

## Performance Optimization

### 1. Use Remote Caching

**Turborepo Remote Cache:**
```bash
# Enable Vercel Remote Cache
npx turbo login
npx turbo link

# Builds use shared cache
pnpm turbo run build
```

**Nx Cloud:**
```bash
# Connect to Nx Cloud
npx nx connect-to-nx-cloud

# Builds use distributed cache
nx build web
```

### 2. Sparse Checkout for Large Repos

```bash
# Only check out needed directories
git sparse-checkout init --cone
git sparse-checkout set apps/web packages/ui
```

### 3. Partial Installation

```bash
# Install only needed deps
pnpm install --filter @myapp/web...
```

### 4. Build Cache Reuse

Ensure cache directories are preserved:

```json
// turbo.json
{
  "pipeline": {
    "build": {
      "outputs": ["dist/**", ".next/**", "node_modules/.cache/**"]
    }
  }
}
```

## Example: Complete Feature Development

### Scenario

Implementing a search feature across:
- Search UI component (packages/ui)
- Search API endpoint (services/api)
- Search page (apps/web)
- Search tests

### Session Setup

```bash
claudio start search-feature
```

### Tasks

**Task 1 - Search Component:**
```
Create SearchInput component in packages/ui.

Sparse checkout:
git sparse-checkout set packages/ui packages/config

Implementation:
1. Create packages/ui/src/SearchInput.tsx
2. Add keyboard navigation
3. Add loading states
4. Create Storybook story

Build and test:
pnpm turbo run build --filter=@myapp/ui
pnpm turbo run test --filter=@myapp/ui
```

**Task 2 - Search API:**
```
Implement search endpoint in services/api.

Sparse checkout:
git sparse-checkout set services/api packages/config

Implementation:
1. Create services/api/src/routes/search.ts
2. Add full-text search query
3. Add pagination
4. Add rate limiting

Build and test:
pnpm turbo run build --filter=@myapp/api
pnpm turbo run test --filter=@myapp/api
```

**Task 3 - Search Page:**
```
Implement search page in apps/web.

Sparse checkout:
git sparse-checkout set apps/web packages/ui packages/config

Implementation:
1. Create apps/web/src/pages/search.tsx
2. Use SearchInput from @myapp/ui
3. Connect to search API
4. Add result display

Build and test:
pnpm turbo run build --filter=@myapp/web
pnpm turbo run test --filter=@myapp/web
```

**Task 4 - E2E Tests:**
```
Add search E2E tests.

Full checkout (or relevant packages)

Implementation:
1. Create apps/web/e2e/search.spec.ts
2. Test search flow
3. Test no results state
4. Test error states

Test:
pnpm turbo run test:e2e --filter=@myapp/web
```

## Configuration Recommendations

For monorepos:

```yaml
# ~/.config/claudio/config.yaml

# Large repos need more time
instance:
  activity_timeout_minutes: 60
  completion_timeout_minutes: 120

# Assign reviewers by package ownership
pr:
  reviewers:
    by_path:
      "apps/web/**": [frontend-team]
      "apps/mobile/**": [mobile-team]
      "packages/ui/**": [design-system-team]
      "services/**": [backend-team]
      "package.json": [tech-lead]
      "turbo.json": [tech-lead, devops]

# Monorepo development can be expensive
resources:
  cost_warning_threshold: 20.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: Monorepo CI

on:
  pull_request:

jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      packages: ${{ steps.filter.outputs.changes }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v3
        id: filter
        with:
          filters: |
            web:
              - 'apps/web/**'
              - 'packages/ui/**'
            api:
              - 'services/api/**'
            mobile:
              - 'apps/mobile/**'

  build:
    needs: changes
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: pnpm/action-setup@v2
        with:
          version: 8

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'pnpm'

      - name: Install dependencies
        run: pnpm install

      - name: Build affected
        run: pnpm turbo run build --filter="[origin/main]"

      - name: Test affected
        run: pnpm turbo run test --filter="[origin/main]"

      - name: Lint affected
        run: pnpm turbo run lint --filter="[origin/main]"
```

## Troubleshooting

### Sparse checkout issues

Files not appearing after sparse checkout.

**Solution**:
```bash
# Re-initialize sparse checkout
git sparse-checkout init --cone
git sparse-checkout set apps/web packages/ui
git checkout .
```

### Dependency resolution failures

Package not found in workspace.

**Solution**:
```bash
# Ensure workspace packages are linked
pnpm install

# Or rebuild the lockfile
rm pnpm-lock.yaml
pnpm install
```

### Build cache not working

Cache misses despite no changes.

**Solution**:
```bash
# Verify turbo cache
pnpm turbo run build --summarize

# Clear cache if corrupted
pnpm turbo daemon clean
```

### Circular dependencies

Packages referencing each other.

**Solution**:
```bash
# Find circular deps
madge --circular --extensions ts apps/web

# Refactor to break cycles
```

### Out of memory

Large monorepo exhausting memory.

**Solution**:
```bash
# Increase Node memory
NODE_OPTIONS="--max-old-space-size=8192" pnpm turbo run build

# Or use sparse checkout to reduce scope
```

## What You Learned

- Sparse checkout strategies for large repos
- Package-targeted development workflows
- Build orchestration integration
- Testing strategies for monorepos
- Dependency management across packages
- CI integration for efficient builds

## Next Steps

- [Web Development](web-development.md) - Frontend package patterns
- [Full-Stack Development](fullstack-development.md) - Multi-service coordination
- [Configuration Guide](../guide/configuration.md) - Customize for your team

# Web Development: Using Claudio with Node.js and Frontend Projects

**Time: 20-30 minutes**

This tutorial explains how to use Claudio effectively with web development projects including Node.js backends, React/Vue/Angular frontends, and full JavaScript/TypeScript stacks.

## Overview

Claudio's git worktree architecture provides significant benefits for web development:

- **Isolated node_modules**: Each worktree can have independent dependencies
- **Parallel builds**: Run build processes in multiple worktrees simultaneously
- **No port conflicts**: With proper configuration, dev servers can run in parallel
- **Hot reload isolation**: Changes in one worktree don't trigger rebuilds in others
- **Framework-agnostic**: Works with React, Vue, Angular, Svelte, Next.js, and more

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Node.js and npm/yarn/pnpm installed
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding Node.js and Git Worktrees

### How node_modules Works with Worktrees

Each Claudio worktree is a complete copy of your project structure:

```
.claudio/worktrees/abc123/
├── package.json
├── node_modules/          # Isolated dependencies
├── src/
└── ...
```

This means:

- Each worktree has its own `node_modules` directory
- Dependencies are installed independently per worktree
- No shared caches by default (can be configured)
- First `npm install` in each worktree downloads fresh packages

### Dependency Installation Time

| Project Size | Fresh Install | With Cache |
|-------------|---------------|------------|
| Small (~50 deps) | 30s - 1min | 10-20s |
| Medium (~200 deps) | 1-3min | 30-60s |
| Large (~500 deps) | 3-5min | 1-2min |
| Monorepo | 5min+ | 2-5min |

**Key insight**: Configure npm/yarn/pnpm caching to significantly reduce install times across worktrees.

## Strategy 1: Full Isolation (Recommended for Most Projects)

Best for: Small to medium projects, or when complete isolation is needed.

### Benefits

- Complete dependency isolation between tasks
- No risk of cross-worktree interference
- Each instance can use different package versions if needed
- Ideal for testing dependency upgrades

### Workflow

```bash
# Start a session
claudio start feature-work

# Add tasks - each gets isolated environment
# Press 'a' in the TUI:
```

**Task 1:**
```
Implement the new Dashboard component in src/components/Dashboard.tsx.
Run npm install first, then npm run build to verify.
```

**Task 2:**
```
Add unit tests for the Dashboard in src/__tests__/Dashboard.test.tsx.
Run npm install && npm test to verify tests pass.
```

### Installing Dependencies

Include dependency installation in task descriptions:

```
Implement the new API client in src/api/client.ts.

1. Run: npm install
2. Implement the client using axios
3. Build: npm run build
4. Test: npm test
```

## Strategy 2: Shared npm Cache (For Faster Installs)

Best for: Medium to large projects where install time is significant.

### npm Configuration

npm uses a global cache by default at `~/.npm`. This cache is shared across all projects and worktrees automatically.

Verify caching is working:

```bash
# Check cache location
npm config get cache

# Verify cache contents
npm cache ls
```

### yarn Configuration

Yarn's cache is also global by default:

```bash
# Check cache location
yarn cache dir

# Enable offline mirror for even faster installs
yarn config set yarn-offline-mirror ~/.yarn-offline-mirror
yarn config set yarn-offline-mirror-pruning true
```

### pnpm Configuration (Recommended)

pnpm uses a content-addressable store that's extremely efficient:

```bash
# pnpm shares packages across all projects by default
# Just use pnpm instead of npm

pnpm install  # Uses shared store automatically
```

**Task instructions with pnpm:**
```
Implement the authentication service in src/services/auth.ts.
Use pnpm install to install dependencies (faster than npm).
Build with: pnpm run build
```

## Strategy 3: Workspace-Level Dependencies

Best for: Monorepos using npm/yarn/pnpm workspaces.

### npm Workspaces

```json
// package.json
{
  "workspaces": ["packages/*"]
}
```

Each worktree will run workspace install independently:

```bash
# In task instructions
npm install  # Installs all workspace packages
npm run build -w @myapp/frontend  # Build specific package
```

### Turborepo Integration

If using Turborepo for build orchestration:

```
Build the frontend package changes.

1. npm install
2. npx turbo run build --filter=@myapp/frontend
3. npx turbo run test --filter=@myapp/frontend
```

## Development Server Considerations

### Port Conflicts

Multiple dev servers need different ports. Configure this in tasks:

**Task 1:**
```
Start the frontend dev server on port 3001:
PORT=3001 npm run dev
```

**Task 2:**
```
Start the backend API server on port 4001:
PORT=4001 npm run start:dev
```

### Environment Variables

Create environment files per worktree or use inline variables:

```
Set up the development environment:

1. Create .env.local with:
   PORT=3002
   API_URL=http://localhost:4001

2. npm run dev
```

### Hot Module Replacement (HMR)

HMR works independently in each worktree. If using websockets for HMR, configure different ports:

```javascript
// vite.config.ts (per-worktree if needed)
export default {
  server: {
    port: 3001,
    hmr: {
      port: 24679  // Different HMR port per worktree
    }
  }
}
```

## Testing Strategies

### Unit Tests (Parallel-Safe)

Unit tests can run concurrently across worktrees:

**Task 1:**
```
Run unit tests for components:
npm test -- --testPathPattern="components"
```

**Task 2:**
```
Run unit tests for services:
npm test -- --testPathPattern="services"
```

### Integration Tests (May Need Coordination)

Integration tests using databases or external services may conflict:

**Option A: Different Test Databases**
```
Run integration tests with isolated database:
TEST_DB_NAME=test_abc123 npm run test:integration
```

**Option B: Sequential Execution**
Use task chaining:
```bash
claudio add "Run integration tests" --depends-on "unit-tests"
```

### E2E Tests (Requires Coordination)

E2E tests using browser automation need careful coordination:

```
Run E2E tests on Chromium:
npx playwright test --project=chromium

# Or use different browsers per instance:
# Instance 1: --project=chromium
# Instance 2: --project=firefox
# Instance 3: --project=webkit
```

## Framework-Specific Tips

### React (Create React App, Vite, Next.js)

**Create React App:**
```
Build and test the React app:
npm install
npm run build
CI=true npm test  # CI=true prevents watch mode
```

**Vite:**
```
Build the Vite project:
npm install
npm run build
npm run preview -- --port 4173  # Preview build on specific port
```

**Next.js:**
```
Build Next.js application:
npm install
npm run build
npm run start -- -p 3001  # Start on specific port
```

### Vue

```
Build and test the Vue application:
npm install
npm run build
npm run test:unit
```

### Angular

```
Build the Angular project:
npm install
ng build --configuration production
ng test --watch=false --browsers=ChromeHeadless
```

### Svelte/SvelteKit

```
Build the SvelteKit application:
npm install
npm run build
npm run preview -- --port 4173
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `package.json` | HIGH | One instance handles dependency changes |
| `package-lock.json` | HIGH | Same as package.json |
| `tsconfig.json` | MEDIUM | Avoid parallel config changes |
| `.env` files | LOW | Usually gitignored |
| Source files | LOW | Different instances work on different files |

### Task Design for Web Projects

**Good decomposition** (minimizes conflicts):
```
├── Authentication (src/auth/*.ts)
├── Dashboard components (src/components/dashboard/*.tsx)
├── API client (src/api/*.ts)
└── Tests (src/__tests__/*.ts)
```

**Risky decomposition** (high conflict potential):
```
├── Add login page (touches package.json, multiple components)
├── Add signup page (touches package.json, multiple components)
└── Add profile page (touches package.json, multiple components)
```

### Handling package.json Conflicts

If multiple instances need to add dependencies:

**Option A: Sequential dependency additions**
```bash
claudio add "Add axios for API client" --start
claudio add "Add zod for validation" --depends-on "abc123"
```

**Option B: One instance handles all dependencies**
```
Set up all new dependencies needed for the feature:
npm install axios zod @tanstack/react-query

Do not implement features yet, just install and configure.
```

## Example: Building a Full Feature

### Scenario

You're implementing a user settings feature with:
- Settings page component
- Settings API endpoints
- Form validation
- Unit tests

### Session Setup

```bash
claudio start settings-feature
```

### Tasks

**Task 1 - API Layer:**
```
Create the settings API client in src/api/settings.ts:
- GET /api/settings - fetch user settings
- PUT /api/settings - update user settings
- Use axios for HTTP requests
- Add proper TypeScript types

npm install
npm run build
```

**Task 2 - Form Validation:**
```
Create validation schemas in src/validation/settings.ts:
- Use zod for schema validation
- Create schemas for all settings fields
- Export validation functions

npm install
npm run build
```

**Task 3 - Settings Component:**
```
Create SettingsPage component in src/pages/Settings.tsx:
- Form for user preferences
- Use react-hook-form for form state
- Display loading and error states
- Add to router in src/App.tsx

npm install
npm run build
```

**Task 4 - Unit Tests:**
```
Create tests in src/__tests__/settings/:
- Test settings API client
- Test validation schemas
- Test SettingsPage component with React Testing Library

npm install
npm test
```

### Monitoring

- Press `c` to check for conflicts in shared files
- Review diffs with `d` before creating PRs
- Watch for build failures in instance output

## TypeScript Considerations

### Type Declaration Files

Multiple instances modifying types can cause conflicts:

```
Update TypeScript types in src/types/settings.ts only.
Do not modify other type files.
```

### Build Configuration

tsconfig.json rarely needs parallel modifications:

```
If you need to update tsconfig.json, mention it clearly.
Otherwise, work within existing configuration.
```

### Incremental Builds

TypeScript incremental builds use `.tsbuildinfo` files. Each worktree maintains its own:

```
Build with incremental compilation:
npm run build  # Uses tsc --incremental automatically
```

## Performance Tips

### 1. Use pnpm

pnpm is significantly faster than npm for multi-worktree setups:

```bash
# Project-wide switch
npm install -g pnpm
rm -rf node_modules package-lock.json
pnpm install
```

### 2. Leverage Build Caches

**Turborepo:**
```bash
# Turborepo caches build artifacts
npx turbo run build --cache-dir=.turbo
```

**Nx:**
```bash
# Nx has built-in computation caching
npx nx build my-app
```

### 3. Parallel Test Execution

```bash
# Jest parallelization
npm test -- --maxWorkers=50%

# Vitest parallelization (default behavior)
npm test
```

### 4. Skip Unnecessary Steps

```
Quick validation without full build:
npm run typecheck  # Faster than full build
npm run lint       # Catch errors quickly
```

## Configuration Recommendations

For web projects, consider this Claudio configuration:

```yaml
# ~/.config/claudio/config.yaml

# Web builds can be slow, allow more time
instance:
  activity_timeout_minutes: 45

# Assign reviewers by expertise
pr:
  reviewers:
    by_path:
      "*.tsx": [frontend-team]
      "*.vue": [frontend-team]
      "src/api/**": [backend-team]
      "package.json": [tech-lead]

# Monitor costs for API-heavy development
resources:
  cost_warning_threshold: 10.00
```

## CI Integration

When using Claudio-generated branches in CI:

```yaml
# Example GitHub Actions workflow
name: Web Build and Test

on:
  pull_request:
    paths:
      - '**/*.ts'
      - '**/*.tsx'
      - '**/*.vue'
      - 'package*.json'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'

      - name: Install dependencies
        run: npm ci

      - name: Type check
        run: npm run typecheck

      - name: Lint
        run: npm run lint

      - name: Build
        run: npm run build

      - name: Test
        run: npm test -- --coverage

      - name: Upload coverage
        uses: codecov/codecov-action@v3
```

## Troubleshooting

### "ENOENT: no such file or directory, open 'package.json'"

The worktree may not have been created properly.

**Solution**: Ensure package.json is tracked in git:
```bash
git add package.json package-lock.json
```

### "EADDRINUSE: address already in use"

Multiple dev servers trying to use the same port.

**Solution**: Use different ports per instance:
```bash
PORT=3001 npm run dev
```

### "Cannot find module" after install

Dependencies not installed in the worktree.

**Solution**: Always include `npm install` in task instructions:
```
First run: npm install
Then implement the feature...
```

### Build fails with "out of memory"

Large builds exhausting Node.js memory.

**Solution**: Increase memory limit:
```bash
NODE_OPTIONS="--max-old-space-size=4096" npm run build
```

### Lock file conflicts

Multiple instances modified package-lock.json.

**Solution**: Designate one instance for dependency changes, or regenerate:
```bash
rm package-lock.json
npm install
```

## What You Learned

- How node_modules interacts with git worktrees
- Strategies for faster dependency installation
- Managing development servers across worktrees
- Testing strategies that avoid conflicts
- Framework-specific build considerations
- CI integration patterns

## Next Steps

- [Feature Development](feature-development.md) - General parallel development patterns
- [Full-Stack Development](fullstack-development.md) - Coordinating frontend and backend
- [Monorepo Development](monorepo-development.md) - Managing large JavaScript monorepos
- [Configuration Guide](../guide/configuration.md) - Customize for your team

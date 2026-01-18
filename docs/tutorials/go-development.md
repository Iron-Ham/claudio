# Go Development: Using Claudio with Go Projects

**Time: 20-30 minutes**

This tutorial explains how to use Claudio effectively with Go projects, covering module management, build caching, testing strategies, and handling common Go-specific challenges.

## Overview

Claudio's git worktree architecture works exceptionally well with Go projects:

- **Module cache sharing**: Go's module cache is global, making worktrees lightweight
- **Fast builds**: Go's compilation speed makes parallel development efficient
- **Test isolation**: Tests run independently with no shared state
- **Static binaries**: Each worktree can build and run binaries independently
- **No dependency installation step**: Modules download on-demand

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Go 1.21+ installed
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding Go and Git Worktrees

### How Go Modules Work with Worktrees

Go uses a global module cache at `$GOPATH/pkg/mod` (typically `~/go/pkg/mod`). This cache is:

- **Shared across all projects and worktrees**
- **Read-only after download** (immutable)
- **Content-addressed** (same version = same files)

This means:

```
Main repo:
└── go.mod (references github.com/pkg/errors v0.9.1)

.claudio/worktrees/abc123/
└── go.mod (references github.com/pkg/errors v0.9.1)
    ↓
    Both use ~/go/pkg/mod/github.com/pkg/errors@v0.9.1/
```

### Build Cache Location

Go's build cache is stored separately from modules:

| Cache | Location | Shared? |
|-------|----------|---------|
| Module cache | `~/go/pkg/mod` | Yes - all worktrees |
| Build cache | `~/.cache/go-build` | Yes - all worktrees |
| Test cache | `~/.cache/go-build` | Yes - all worktrees |

**Key insight**: Go's caching is already optimized for multi-worktree development. No additional configuration needed.

### Build Times

| Project Size | Cold Build | Incremental | Test Suite |
|-------------|------------|-------------|------------|
| Small (~10 packages) | 5-15s | 1-3s | 2-5s |
| Medium (~50 packages) | 15-45s | 3-10s | 10-30s |
| Large (~200 packages) | 45s-2min | 10-30s | 1-5min |
| Enterprise | 2-5min | 30s-1min | 5-15min |

## Strategy 1: Standard Workflow (Recommended)

Best for: Most Go projects.

### Benefits

- Zero configuration needed
- Leverages Go's built-in caching
- Fast iteration with incremental builds
- Tests run in parallel by default

### Workflow

```bash
# Start a session
claudio start feature-work

# Add tasks (press 'a' in TUI)
```

**Task 1 - New Package:**
```
Create a new validation package in internal/validation/:
- Create validate.go with email, phone, URL validators
- Create validate_test.go with table-driven tests
- Ensure all tests pass: go test ./internal/validation/...
```

**Task 2 - API Handler:**
```
Add a new user preferences handler in internal/api/preferences.go:
- GET /api/preferences
- PUT /api/preferences
- Use the validation package
- Build: go build ./...
- Test: go test ./internal/api/...
```

### Task Instructions Pattern

Always include build and test commands:

```
Implement the feature in internal/feature/feature.go.

1. Implement the core logic
2. Build: go build ./...
3. Test: go test ./internal/feature/...
4. Verify no race conditions: go test -race ./internal/feature/...
```

## Strategy 2: Parallel Package Development

Best for: Large projects with independent packages.

### Workflow

Assign different packages to different instances:

**Instance 1 - Database layer:**
```
Implement database repository in internal/db/user_repo.go:
- UserRepository interface
- PostgreSQL implementation
- Unit tests with mocks

go build ./internal/db/...
go test ./internal/db/...
```

**Instance 2 - Service layer:**
```
Implement user service in internal/service/user.go:
- UserService struct
- Business logic for user operations
- Integration with repository interface

go build ./internal/service/...
go test ./internal/service/...
```

**Instance 3 - API layer:**
```
Implement user API handlers in internal/api/user.go:
- HTTP handlers using chi/gin/echo
- Request validation
- Response formatting

go build ./internal/api/...
go test ./internal/api/...
```

### Dependency Order

If packages depend on each other, use task chaining:

```bash
claudio add "Implement db/user_repo.go" --start
claudio add "Implement service/user.go" --depends-on "db/user_repo"
claudio add "Implement api/user.go" --depends-on "service/user"
```

## Testing Strategies

### Unit Tests (Parallel-Safe)

Go tests are parallel-safe by default:

**Task 1:**
```
Run tests for internal/auth package:
go test -v ./internal/auth/...
```

**Task 2:**
```
Run tests for internal/api package:
go test -v ./internal/api/...
```

### Integration Tests

For tests requiring databases or external services:

**Option A: Test Containers**
```
Run integration tests using testcontainers:
go test -v -tags=integration ./internal/db/...
```

**Option B: Separate Test Databases**
```
Run integration tests with isolated database:
TEST_DB_NAME=test_abc123 go test -v -tags=integration ./...
```

**Option C: Sequential Execution**
```bash
claudio add "Run integration tests" --depends-on "unit-tests"
```

### Race Detection

Include race detection in CI-bound tasks:

```
Run tests with race detector:
go test -race ./...
```

### Benchmarks

Benchmarks can run in parallel across worktrees:

**Task 1:**
```
Run benchmarks for encoding package:
go test -bench=. -benchmem ./internal/encoding/...
```

**Task 2:**
```
Run benchmarks for parser package:
go test -bench=. -benchmem ./internal/parser/...
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `go.mod` | MEDIUM | Coordinate dependency additions |
| `go.sum` | MEDIUM | Regenerates automatically |
| `main.go` | LOW | Usually minimal changes |
| Package files | LOW | Different instances, different packages |

### Task Design for Go

**Good decomposition** (minimizes conflicts):
```
├── internal/auth/ (authentication package)
├── internal/api/  (API handlers)
├── internal/db/   (database layer)
└── internal/util/ (utilities)
```

**Risky decomposition** (modifies shared files):
```
├── Add user auth (touches main.go, multiple packages)
├── Add admin auth (touches main.go, multiple packages)
└── Add API auth  (touches main.go, multiple packages)
```

### Handling go.mod Conflicts

If multiple instances need to add dependencies:

**Option A: Pre-install dependencies**
```
First, add all required dependencies:
go get github.com/stretchr/testify
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5

Then implement features using these packages.
```

**Option B: Sequential dependency additions**
```bash
claudio add "Add chi router dependency" --start
claudio add "Add pgx dependency" --depends-on "abc123"
claudio add "Implement features" --depends-on "def456"
```

**Option C: Let Go handle it**
Go modules handle concurrent modification reasonably well. If conflicts occur:
```bash
cd .claudio/worktrees/abc123
go mod tidy
```

## Code Generation

### protobuf/gRPC

Include generation in tasks:

```
Update the user service proto definition:

1. Edit api/proto/user.proto
2. Generate: protoc --go_out=. --go-grpc_out=. api/proto/user.proto
3. Implement new methods in internal/grpc/user.go
4. Test: go test ./internal/grpc/...
```

### go generate

For code generation directives:

```
Add a new mock for the UserRepository:

1. Add //go:generate directive to internal/db/user_repo.go
2. Run: go generate ./internal/db/...
3. Verify mock in internal/db/mocks/
4. Update tests to use new mock
```

### sqlc

For SQL code generation:

```
Add new database queries:

1. Add queries to db/queries/users.sql
2. Generate: sqlc generate
3. Implement service methods using generated code
4. Test: go test ./internal/db/...
```

## Project Layout Patterns

### Standard Layout

For projects following standard Go project layout:

```
project/
├── cmd/
│   └── myapp/
│       └── main.go
├── internal/
│   ├── api/
│   ├── db/
│   └── service/
├── pkg/            # Public packages
├── go.mod
└── go.sum
```

Assign instances by directory:
- Instance 1: `cmd/` and `main.go` changes
- Instance 2: `internal/api/`
- Instance 3: `internal/db/`
- Instance 4: `internal/service/`

### Monorepo Layout

For Go monorepos with multiple modules:

```
monorepo/
├── services/
│   ├── api/
│   │   └── go.mod
│   └── worker/
│       └── go.mod
├── libs/
│   └── common/
│       └── go.mod
└── go.work
```

Each service can be developed independently:

```
Implement feature in services/api:
cd services/api
go build ./...
go test ./...
```

## Performance Tips

### 1. Use Build Tags

Separate slow tests:

```go
//go:build integration

package db_test
```

```
Run only unit tests (fast):
go test ./...

Run integration tests (when needed):
go test -tags=integration ./...
```

### 2. Parallel Test Execution

Go runs tests in parallel by default. Control parallelism:

```bash
# More parallelism for IO-bound tests
go test -parallel 8 ./...

# Less parallelism for CPU-bound tests
go test -parallel 2 ./...
```

### 3. Build Cache Optimization

Ensure build cache is working:

```bash
# Check cache statistics
go env GOCACHE
go clean -cache -n  # Dry run to see what would be cleaned
```

### 4. Module Proxy

Use a module proxy for faster downloads:

```bash
# Default (Google's proxy)
go env GOPROXY  # Usually https://proxy.golang.org,direct

# For private modules
export GOPRIVATE=github.com/mycompany/*
```

### 5. Incremental Builds

Go's incremental builds are automatic. Avoid `go clean` in tasks:

```
# Good - leverages cache
go build ./...

# Avoid - clears cache unnecessarily
go clean && go build ./...
```

## Example: Building a Complete Feature

### Scenario

Implementing a notification system with:
- Database models
- Service logic
- API endpoints
- Background worker

### Session Setup

```bash
claudio start notification-feature
```

### Tasks

**Task 1 - Database Layer:**
```
Create notification repository in internal/db/notification.go:

- NotificationRepository interface
- PostgreSQL implementation with:
  - Create(ctx, notification) error
  - GetByUserID(ctx, userID) ([]Notification, error)
  - MarkAsRead(ctx, id) error
- Use sqlc for type-safe queries

go build ./internal/db/...
go test ./internal/db/...
```

**Task 2 - Service Layer:**
```
Create notification service in internal/service/notification.go:

- NotificationService struct
- Methods:
  - Send(ctx, userID, message) error
  - GetUnread(ctx, userID) ([]Notification, error)
  - MarkRead(ctx, notificationID) error
- Add unit tests with repository mocks

go build ./internal/service/...
go test ./internal/service/...
```

**Task 3 - API Handlers:**
```
Create API handlers in internal/api/notification.go:

- GET /api/notifications - list user's notifications
- POST /api/notifications/:id/read - mark as read
- Use chi router
- Add request validation
- Add integration tests

go build ./internal/api/...
go test ./internal/api/...
```

**Task 4 - Background Worker:**
```
Create notification worker in internal/worker/notification.go:

- Process queued notifications
- Batch delivery for efficiency
- Retry logic for failures
- Add unit tests

go build ./internal/worker/...
go test ./internal/worker/...
```

### Monitoring

- Press `c` to check for conflicts (especially go.mod)
- Review diffs with `d` before creating PRs
- Watch for test failures in instance output

## Configuration Recommendations

For Go projects, consider this Claudio configuration:

```yaml
# ~/.config/claudio/config.yaml

# Go builds are fast, can use shorter timeouts
instance:
  activity_timeout_minutes: 20
  completion_timeout_minutes: 45

# Assign reviewers by package
pr:
  reviewers:
    by_path:
      "internal/db/**": [backend-team, dba]
      "internal/api/**": [backend-team, api-team]
      "cmd/**": [tech-lead]
      "go.mod": [tech-lead]

# Go development is usually cost-efficient
resources:
  cost_warning_threshold: 5.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: Go Build and Test

on:
  pull_request:
    paths:
      - '**/*.go'
      - 'go.mod'
      - 'go.sum'

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: Build
        run: go build ./...

      - name: Test
        run: go test -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: coverage.out

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache: true

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
```

## Troubleshooting

### "cannot find module providing package"

Module not downloaded yet.

**Solution**: Run `go mod download` or let `go build` download automatically:
```bash
go build ./...  # Downloads missing modules
```

### "go.sum mismatch"

go.sum was modified inconsistently.

**Solution**: Regenerate checksums:
```bash
go mod tidy
```

### "package X is not in GOROOT"

Incorrect import path or missing module.

**Solution**: Verify import paths and run:
```bash
go mod tidy
go build ./...
```

### "build cache is required"

Build cache was disabled.

**Solution**: Ensure GOCACHE is set:
```bash
go env GOCACHE  # Should show a path
export GOCACHE="$HOME/.cache/go-build"  # If empty
```

### Tests timing out

Tests taking too long.

**Solution**: Increase timeout or skip slow tests:
```bash
go test -timeout 5m ./...  # Increase timeout
go test -short ./...       # Skip slow tests
```

## What You Learned

- How Go modules and build cache work with worktrees
- Strategies for parallel package development
- Testing approaches for different test types
- Handling go.mod conflicts
- Code generation patterns
- CI integration best practices

## Next Steps

- [Feature Development](feature-development.md) - General parallel patterns
- [Large Refactor](large-refactor.md) - Coordinating major Go refactors
- [Configuration Guide](../guide/configuration.md) - Customize for your team

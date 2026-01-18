# Rust Development: Using Claudio with Rust Projects

**Time: 20-30 minutes**

This tutorial explains how to use Claudio effectively with Rust projects, covering Cargo workspaces, target directory management, build optimization strategies, and handling Rust-specific compilation considerations.

## Overview

Claudio's git worktree architecture has unique benefits and considerations for Rust:

- **Target directory isolation**: Each worktree has separate build artifacts
- **Cargo registry caching**: Downloaded crates are shared globally
- **Incremental compilation**: Works independently per worktree
- **Parallel builds**: Multiple worktrees can compile simultaneously
- **Workspace support**: Cargo workspaces work seamlessly

## Prerequisites

- Claudio initialized in your project (`claudio init`)
- Rust toolchain installed (rustup recommended)
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding Rust and Git Worktrees

### How Cargo Works with Worktrees

Cargo uses multiple storage locations:

| Location | Purpose | Shared? |
|----------|---------|---------|
| `~/.cargo/registry` | Downloaded crate sources | Yes |
| `~/.cargo/git` | Git dependencies | Yes |
| `./target` | Build artifacts | No (per-worktree) |

This means:

```
Main repo:
├── Cargo.toml
├── src/
└── target/              # Build artifacts for main repo

.claudio/worktrees/abc123/
├── Cargo.toml
├── src/
└── target/              # Separate build artifacts
```

### Build Time Implications

| Project Size | Cold Build (Debug) | Cold Build (Release) | Incremental |
|-------------|-------------------|---------------------|-------------|
| Small (~10 crates) | 30s - 1min | 1-3min | 5-15s |
| Medium (~50 crates) | 1-3min | 5-10min | 15-45s |
| Large (~200 crates) | 5-10min | 15-30min | 1-3min |
| Complex (heavy deps) | 10-20min | 30min+ | 2-5min |

**Key insight**: Rust's long compile times make target directory isolation important. Each worktree compiles independently, but this enables true parallel development.

## Strategy 1: Full Isolation (Recommended)

Best for: Most Rust projects.

### Benefits

- Complete build isolation
- No compilation conflicts
- Parallel builds without interference
- Clean incremental compilation state

### Workflow

```bash
# Start a session
claudio start feature-work

# Add tasks (press 'a' in TUI)
```

**Task 1:**
```
Implement the user authentication module in src/auth/.

1. Create src/auth/mod.rs with module structure
2. Implement authentication logic
3. Build: cargo build
4. Test: cargo test auth::
```

**Task 2:**
```
Add API handlers in src/api/handlers.rs.

1. Create request/response types
2. Implement handler functions
3. Build: cargo build
4. Test: cargo test api::
```

### Task Instructions Pattern

```
Implement the feature in src/feature/.

1. Create module structure
2. Implement core functionality
3. Check: cargo check
4. Build: cargo build
5. Test: cargo test feature::
6. Lint: cargo clippy -- -D warnings
```

## Strategy 2: Shared Target Directory

Best for: Disk-constrained environments or rapid iteration.

### Configuration

Share a target directory across worktrees using environment variables:

```bash
# In ~/.bashrc or task instructions
export CARGO_TARGET_DIR="$HOME/.cargo-targets/$(basename $PWD)"
```

Or create a `.cargo/config.toml` template:

```toml
# .cargo/config.toml
[build]
target-dir = "/path/to/shared/target"
```

### Caveats

- Concurrent builds may conflict
- Only use when worktrees won't build simultaneously
- Incremental compilation may be less effective

## Strategy 3: Workspace-Aware Development

Best for: Cargo workspaces (monorepos).

### Workspace Structure

```
workspace/
├── Cargo.toml          # Workspace root
├── crates/
│   ├── core/
│   │   └── Cargo.toml
│   ├── api/
│   │   └── Cargo.toml
│   └── cli/
│       └── Cargo.toml
└── target/             # Shared within workspace
```

### Task Design for Workspaces

Assign instances to specific crates:

**Task 1:**
```
Implement new functionality in the core crate.

cd crates/core
cargo build -p core
cargo test -p core
```

**Task 2:**
```
Add API endpoints using the core crate.

cargo build -p api
cargo test -p api
```

**Task 3:**
```
Update CLI commands.

cargo build -p cli
cargo test -p cli
```

### Feature Flags

Use feature flags to control compilation scope:

```
Build only the HTTP feature:
cargo build --features http

Build without default features:
cargo build --no-default-features --features minimal
```

## Testing Strategies

### Unit Tests (Parallel-Safe)

Cargo tests run in parallel by default:

**Task 1:**
```
Run unit tests for the auth module:
cargo test auth:: -- --nocapture
```

**Task 2:**
```
Run unit tests for the api module:
cargo test api:: -- --nocapture
```

### Integration Tests

Integration tests run separately:

```
Run integration tests:
cargo test --test integration_tests
```

### Test Parallelism Control

For tests that can't run in parallel:

```
Run tests sequentially:
cargo test -- --test-threads=1
```

### Doc Tests

Include doc tests in verification:

```
Verify documentation compiles:
cargo test --doc
```

### Benchmarks

Run benchmarks in specific worktrees:

```
Run benchmarks:
cargo bench
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `Cargo.toml` | MEDIUM | Coordinate dependency additions |
| `Cargo.lock` | MEDIUM | Regenerates on build |
| `src/lib.rs` | MEDIUM | Careful module organization |
| Module files | LOW | Different modules per instance |

### Task Design for Rust

**Good decomposition:**
```
├── src/auth/        (authentication module)
├── src/api/         (API handlers)
├── src/db/          (database layer)
└── src/util/        (utilities)
```

**Risky decomposition:**
```
├── Add user auth   (touches Cargo.toml, lib.rs, multiple modules)
├── Add admin auth  (touches Cargo.toml, lib.rs, multiple modules)
└── Add API auth    (touches Cargo.toml, lib.rs, multiple modules)
```

### Handling Cargo.toml Conflicts

**Option A: Pre-add dependencies**
```
First, add all required dependencies to Cargo.toml:
- tokio for async runtime
- serde for serialization
- axum for web framework

cargo build  # Verify dependencies
```

**Option B: Sequential dependency additions**
```bash
claudio add "Add tokio and serde dependencies" --start
claudio add "Add axum dependency" --depends-on "abc123"
claudio add "Implement features" --depends-on "def456"
```

## Toolchain Considerations

### Using rustup

Ensure consistent toolchain usage:

```
Use stable toolchain:
rustup default stable
cargo build
```

### Cross-Compilation

For cross-compilation tasks:

```
Build for Linux target:
rustup target add x86_64-unknown-linux-gnu
cargo build --target x86_64-unknown-linux-gnu
```

### Nightly Features

For nightly-only features:

```
Use nightly for this feature:
cargo +nightly build
cargo +nightly test
```

## Build Optimization

### Debug Builds

For faster iteration:

```
Quick check (fastest):
cargo check

Debug build (fast linking):
cargo build
```

### Release Builds

For performance testing:

```
Build in release mode:
cargo build --release
```

### Compilation Speed

Speed up compilation with these `.cargo/config.toml` settings:

```toml
[target.x86_64-unknown-linux-gnu]
linker = "clang"
rustflags = ["-C", "link-arg=-fuse-ld=mold"]

[profile.dev]
opt-level = 0
debug = false  # Faster builds, no debug info
```

### sccache Integration

Use sccache for shared compilation cache:

```bash
# Install sccache
cargo install sccache

# Configure cargo to use it
export RUSTC_WRAPPER=sccache
```

Then in tasks:

```
Build with shared cache:
RUSTC_WRAPPER=sccache cargo build
```

## Code Quality

### Clippy

Include clippy in verification:

```
Run clippy lints:
cargo clippy -- -D warnings

Fix clippy suggestions:
cargo clippy --fix
```

### Formatting

Include formatting checks:

```
Check formatting:
cargo fmt -- --check

Apply formatting:
cargo fmt
```

### Documentation

Generate and verify documentation:

```
Build documentation:
cargo doc --no-deps

Check documentation warnings:
RUSTDOCFLAGS="-D warnings" cargo doc
```

## Example: Building a Complete Feature

### Scenario

Implementing a caching system with:
- Cache trait and implementations
- Storage backends (memory, Redis)
- Async support
- Benchmarks

### Session Setup

```bash
claudio start cache-feature
```

### Tasks

**Task 1 - Cache Trait:**
```
Define the cache trait in src/cache/mod.rs.

1. Create Cache trait with async methods:
   - get<T>(&self, key: &str) -> Option<T>
   - set<T>(&self, key: &str, value: T, ttl: Duration)
   - delete(&self, key: &str)
   - clear(&self)

2. Add error types in src/cache/error.rs

cargo check
cargo test cache::
```

**Task 2 - Memory Backend:**
```
Implement in-memory cache in src/cache/memory.rs.

1. MemoryCache struct with concurrent HashMap
2. TTL expiration using tokio
3. LRU eviction policy

cargo build
cargo test cache::memory::
```

**Task 3 - Redis Backend:**
```
Implement Redis cache in src/cache/redis.rs.

1. Add redis dependency to Cargo.toml
2. RedisCache struct with connection pool
3. Implement Cache trait

cargo build
cargo test cache::redis:: -- --ignored  # Requires Redis
```

**Task 4 - Benchmarks:**
```
Add benchmarks in benches/cache_bench.rs.

1. Benchmark get/set operations
2. Compare memory vs Redis (when available)
3. Test under concurrent load

cargo bench
```

### Monitoring

- Press `c` to check for conflicts (especially Cargo.toml)
- Review diffs with `d` before creating PRs
- Watch for compilation errors in instance output

## Performance Tips

### 1. Use cargo check

For rapid iteration, use `cargo check` instead of `cargo build`:

```
Quick verification:
cargo check

Only build when needed:
cargo build
```

### 2. Leverage Incremental Compilation

Incremental compilation is enabled by default. Avoid cleaning:

```
# Good - uses incremental compilation
cargo build

# Avoid - clears incremental state
cargo clean && cargo build
```

### 3. Parallel Crate Compilation

Cargo compiles crates in parallel. Configure in `.cargo/config.toml`:

```toml
[build]
jobs = 8  # Adjust based on CPU cores
```

### 4. Minimize Dependencies

Heavy dependencies slow compilation:

```
# Check compilation time
cargo build --timings

# Optimize hot paths
```

### 5. Use Sparse Registry

Enable sparse registry protocol for faster crate downloads:

```bash
# In ~/.cargo/config.toml
[registries.crates-io]
protocol = "sparse"
```

## Configuration Recommendations

For Rust projects:

```yaml
# ~/.config/claudio/config.yaml

# Rust builds can be slow
instance:
  activity_timeout_minutes: 60
  completion_timeout_minutes: 90

# Assign reviewers
pr:
  reviewers:
    by_path:
      "*.rs": [rust-team]
      "Cargo.toml": [tech-lead]
      "Cargo.lock": [tech-lead]

# Rust development is usually moderate cost
resources:
  cost_warning_threshold: 8.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: Rust CI

on:
  pull_request:
    paths:
      - '**/*.rs'
      - 'Cargo.toml'
      - 'Cargo.lock'

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: dtolnay/rust-toolchain@stable
        with:
          components: rustfmt, clippy

      - uses: Swatinem/rust-cache@v2

      - name: Check formatting
        run: cargo fmt -- --check

      - name: Clippy
        run: cargo clippy -- -D warnings

      - name: Build
        run: cargo build --all-targets

      - name: Test
        run: cargo test

  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: dtolnay/rust-toolchain@stable

      - uses: Swatinem/rust-cache@v2

      - name: Run tests
        run: cargo test --all-features

      - name: Run doc tests
        run: cargo test --doc
```

## Troubleshooting

### "error: could not compile" - Dependency version conflict

Multiple versions of the same crate.

**Solution**:
```bash
cargo update
cargo build
```

### Long compile times

First build or many dependencies.

**Solution**:
```bash
# Use check for quick iteration
cargo check

# Enable sccache
export RUSTC_WRAPPER=sccache
cargo build
```

### "error: linking failed"

Linker issues, often on large projects.

**Solution**:
```bash
# Use mold linker (faster)
cargo build -Z mold

# Or increase stack size
RUST_MIN_STACK=8388608 cargo build
```

### Cargo.lock conflicts

Multiple instances modified dependencies.

**Solution**:
```bash
# Regenerate lock file
cargo update
# Or resolve manually and commit
```

### "cannot find crate"

Missing dependency or wrong feature flags.

**Solution**:
```bash
cargo build --all-features
# Or check Cargo.toml for correct feature configuration
```

## What You Learned

- How Cargo's caching works with worktrees
- Strategies for managing target directories
- Workspace development patterns
- Testing approaches for Rust projects
- Build optimization techniques
- CI integration patterns

## Next Steps

- [Feature Development](feature-development.md) - General parallel patterns
- [Large Refactor](large-refactor.md) - Major Rust refactoring
- [Configuration Guide](../guide/configuration.md) - Customize for your team

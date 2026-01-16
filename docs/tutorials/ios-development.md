# iOS Development: Using Claudio with Xcode Projects

**Time: 20-30 minutes**

This tutorial explains how to use Claudio effectively with iOS, macOS, watchOS, and other Apple platform projects that use `xcodebuild`.

## Overview

Claudio's git worktree architecture is particularly well-suited for iOS development because:

- **Isolated build artifacts**: Each worktree gets its own DerivedData by default
- **Parallel development**: Work on multiple features without Xcode index conflicts
- **No branch switching headaches**: Avoid the "Xcode is confused after branch switch" problem
- **Test isolation**: Run tests in one worktree while coding in another

However, iOS projects have unique considerations around build times, DerivedData management, and Swift Package Manager caching that this guide addresses.

## Prerequisites

- Claudio initialized in your iOS project (`claudio init`)
- Xcode and `xcodebuild` installed
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding Xcode and Git Worktrees

### How Xcode Handles DerivedData

By default, Xcode stores build artifacts in:

```
~/Library/Developer/Xcode/DerivedData/{ProjectName}-{hash}/
```

The `{hash}` is derived from the **project path**. Since each Claudio worktree has a different path:

```
.claudio/worktrees/abc123/MyApp.xcodeproj  → DerivedData/MyApp-hash1/
.claudio/worktrees/def456/MyApp.xcodeproj  → DerivedData/MyApp-hash2/
```

This means:

- Each worktree has completely isolated build artifacts
- No shared indexing or caches between worktrees
- First build in each worktree is a "cold build"

### Build Time Implications

| Project Size | Cold Build | Incremental Build |
|-------------|------------|-------------------|
| Small (~50 files) | 30s - 2min | 5-15s |
| Medium (~200 files) | 2-5min | 15-45s |
| Large (~1000 files) | 5-15min | 1-3min |
| Very Large (monorepo) | 15min+ | 3-10min |

**Key insight**: Plan your parallel tasks knowing that each worktree needs an initial build. For small projects, this overhead is negligible. For large projects, you may want strategies to share build artifacts.

## Strategy 1: Full Isolation (Recommended for Most Projects)

Best for: Small to medium projects, or when build time is acceptable.

### Benefits

- Complete isolation between tasks
- No risk of build artifact conflicts
- Xcode indexing works perfectly in each worktree
- Can open multiple Xcode windows simultaneously

### Workflow

```bash
# Start a session for a feature
claudio start auth-feature

# Add tasks - each gets isolated build environment
# Press 'a' in the TUI:
```

**Task 1:**
```
Implement OAuth2 login flow in Sources/Auth/OAuth2Manager.swift.
Build the project after changes: xcodebuild -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15' build
```

**Task 2:**
```
Add biometric authentication in Sources/Auth/BiometricAuth.swift.
Build and test: xcodebuild -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15' test
```

### Opening in Xcode

You can open any worktree in Xcode:

```bash
# Open worktree in Xcode
open .claudio/worktrees/abc123/MyApp.xcworkspace

# Or if using .xcodeproj directly
open .claudio/worktrees/abc123/MyApp.xcodeproj
```

Each Xcode window operates independently with its own index and build state.

## Strategy 2: Shared DerivedData (For Faster Builds)

Best for: Large projects where cold builds are prohibitive.

### Configuration

Use the `-derivedDataPath` flag to share DerivedData across worktrees.

Create a shell script at `scripts/build.sh`:

```bash
#!/bin/bash
# Shared DerivedData build script

DERIVED_DATA="${HOME}/.claudio-derived-data/${PWD##*/}"

xcodebuild \
    -workspace MyApp.xcworkspace \
    -scheme MyApp \
    -destination 'platform=iOS Simulator,name=iPhone 15' \
    -derivedDataPath "$DERIVED_DATA" \
    "$@"
```

### Usage in Tasks

```
Build the new feature using the shared build script:
./scripts/build.sh build

Run tests:
./scripts/build.sh test
```

### Caveats

- Concurrent builds may conflict - serialize build commands
- Index may show stale data briefly after switching
- Some incremental build benefits lost with parallel changes

## Strategy 3: Relative DerivedData (Per-Worktree, No Global Path)

Best for: CI environments or when you want build artifacts tracked with the worktree.

### Configuration

Set DerivedData relative to the project:

```bash
# In each task, use:
xcodebuild \
    -derivedDataPath ./build/DerivedData \
    -scheme MyApp \
    build
```

Add to `.gitignore`:
```
build/
```

### Benefits

- Cleaning is easy: `rm -rf .claudio/worktrees/*/build`
- Build artifacts live with their worktree
- Easy to archive or inspect specific builds

## Swift Package Manager Considerations

### Package Cache Location

SPM stores packages in:
- Per-worktree: `{project}/SourcePackages/`
- Shared: `~/Library/Caches/org.swift.swiftpm/`

### Sharing SPM Cache

SPM automatically uses a global download cache at `~/Library/Caches/org.swift.swiftpm/`. This cache is shared across all projects and worktrees, so downloaded packages are only fetched once. However, each worktree still needs to check out and build its own copy of the packages in `SourcePackages/`.

### Pre-Warming Package Cache

For large projects, pre-warm the cache:

```bash
# Resolve packages once in main repo
swift package resolve

# New worktrees will use cached downloads
```

### Task Instructions for SPM

Include SPM steps in your task descriptions:

```
Add the Alamofire dependency for networking.

1. Edit Package.swift to add Alamofire
2. Run: swift package resolve
3. Build: xcodebuild -scheme MyApp build
```

## Testing Strategies

### Unit Tests (Parallel-Safe)

Unit tests can run concurrently across worktrees:

```
# Task 1
Run unit tests for the Auth module:
xcodebuild test -scheme MyApp -only-testing:MyAppTests/AuthTests

# Task 2
Run unit tests for the Network module:
xcodebuild test -scheme MyApp -only-testing:MyAppTests/NetworkTests
```

### UI Tests (Requires Coordination)

UI tests use simulators, which may conflict:

**Option A: Different Simulators**
```
# Task 1
xcodebuild test -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15'

# Task 2
xcodebuild test -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15 Pro'
```

**Option B: Clone Simulators**
```bash
# Create dedicated test simulators
xcrun simctl clone "iPhone 15" "iPhone 15 - Test 1"
xcrun simctl clone "iPhone 15" "iPhone 15 - Test 2"
```

Then use in tasks:
```
xcodebuild test -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15 - Test 1'
```

### Test Execution Order

For tests that need to run in a specific order, design your tasks to include the full test sequence:

```
# Single task that runs unit tests before UI tests
Run the test suite in order:
1. Unit tests: xcodebuild test -scheme MyApp -only-testing:MyAppTests
2. If unit tests pass, run UI tests: xcodebuild test -scheme MyApp -only-testing:MyAppUITests
```

Alternatively, keep unit and UI tests in separate instances when they're independent:

```
# Instance 1: Unit tests only
Run unit tests: xcodebuild test -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15' -only-testing:MyAppTests

# Instance 2: UI tests only (use different simulator)
Run UI tests: xcodebuild test -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15 Pro' -only-testing:MyAppUITests
```

## Xcode Project Conflicts

### Common Conflict Points

| File | Risk | Mitigation |
|------|------|------------|
| `project.pbxproj` | HIGH | Assign one instance to project file changes |
| `*.xcscheme` | MEDIUM | Avoid scheme edits in parallel |
| `Package.resolved` | MEDIUM | One instance handles dependency changes |
| Source files | LOW | Different instances work on different files |

### Task Design for iOS

**Good decomposition** (minimizes conflicts):
```
├── Auth feature (Sources/Auth/*.swift)
├── Network layer (Sources/Network/*.swift)
├── UI components (Sources/Views/*.swift)
└── Tests (Tests/**/*.swift)
```

**Risky decomposition** (high conflict potential):
```
├── Add login screen (touches project file, Auth, Views)
├── Add profile screen (touches project file, Views)
└── Add settings screen (touches project file, Views)
```

### Handling project.pbxproj Conflicts

If conflicts occur in the Xcode project file:

1. Use `xcodebuild` for one set of changes
2. Manually merge, or
3. Use tools like [mergepbx](https://github.com/nicksandford/mergepbx)

```bash
# Install mergepbx
brew install mergepbx

# Configure git to use it
git config merge.mergepbx.driver "mergepbx %O %A %B"

# Add to .gitattributes
echo "*.pbxproj merge=mergepbx" >> .gitattributes
```

## Example: Building an iOS Feature

### Scenario

You're adding a new "Profile" feature with:
- Profile view model
- Profile API client
- Profile UI
- Unit tests

### Session Setup

```bash
claudio start profile-feature
```

### Tasks

**Task 1 - Model Layer** (no xcodebuild needed initially):
```
Create ProfileModel.swift in Sources/Profile/Models/ with:
- User profile struct (id, name, email, avatarURL)
- Decodable conformance
- Test data factory
```

**Task 2 - API Client**:
```
Create ProfileAPIClient.swift in Sources/Profile/API/ with:
- fetchProfile(userID:) async throws -> ProfileModel
- updateProfile(_:) async throws -> ProfileModel
- Mock implementation for previews

Verify it compiles: xcodebuild -scheme MyApp -destination 'generic/platform=iOS' build
```

**Task 3 - View Model**:
```
Create ProfileViewModel.swift in Sources/Profile/ViewModels/ with:
- @Published profile property
- loadProfile() async function
- Error handling

Build to verify: xcodebuild -scheme MyApp -destination 'generic/platform=iOS' build
```

**Task 4 - SwiftUI View**:
```
Create ProfileView.swift in Sources/Profile/Views/ with:
- Display profile information
- Loading state
- Error state
- Edit button

Build: xcodebuild -scheme MyApp -destination 'generic/platform=iOS' build
```

**Task 5 - Tests** (depends on model and API tasks):
```
Create ProfileTests.swift in Tests/ProfileTests/ with:
- Test ProfileModel decoding
- Test ProfileAPIClient mock
- Test ProfileViewModel state changes

Run tests: xcodebuild test -scheme MyApp -destination 'platform=iOS Simulator,name=iPhone 15' -only-testing:MyAppTests/ProfileTests
```

### Monitoring

- Press `c` to toggle conflict view and see files modified by multiple instances
- Review diffs with `d` before creating PRs
- Watch for build failures in instance output

## CocoaPods and Carthage

### CocoaPods

Pods are installed per-worktree. For large Podfiles:

```bash
# Pre-install in main repo
pod install

# Add to task instructions
# "Run 'pod install' if Podfile changed"
```

Consider using `--deployment` for faster installs:
```
pod install --deployment
```

### Carthage

Carthage builds are cached globally. Add to tasks:

```
If Cartfile changed, run:
carthage update --use-xcframeworks --platform iOS
```

## Performance Tips

### 1. Use Build Targets Wisely

Instead of building the entire app:
```bash
# Build only the target you're working on
xcodebuild -target AuthModule build
```

### 2. Leverage Incremental Builds

Keep worktrees alive for incremental builds:
```bash
# Don't cleanup until work is complete
# Incremental builds are much faster
```

### 3. Use Modular Architecture

Break your project into frameworks/modules to enable better incremental builds. Changes to one module don't require rebuilding unrelated modules, which significantly speeds up development across worktrees.

### 4. SSD Worktree Location

For large projects, configure worktrees on fast storage:

```yaml
# ~/.config/claudio/config.yaml
paths:
  worktree_dir: /Volumes/FastSSD/claudio-worktrees
```

## Troubleshooting

### "xcodebuild: error: The workspace '...' does not exist"

The worktree may not have the full project structure.

**Solution**: Ensure the workspace file is tracked in git:
```bash
git add MyApp.xcworkspace
```

### Build fails with "module not found"

DerivedData doesn't have built modules yet.

**Solution**: Run a full build first:
```bash
xcodebuild -scheme MyApp build
```

### "Simulator in use by another process"

Multiple instances trying to use the same simulator.

**Solution**: Use different simulators per instance (see Testing Strategies above).

### Index not updating in Xcode

The worktree's Xcode window has stale index.

**Solution**:
1. Close and reopen the project, or
2. Delete index: `rm -rf ~/Library/Developer/Xcode/DerivedData/MyApp-*/Index`

## Configuration Recommendations

For iOS projects, consider this Claudio configuration:

```yaml
# ~/.config/claudio/config.yaml

# Allow extra time for iOS builds (default is 30)
instance:
  activity_timeout_minutes: 45

# iOS PRs often need specific reviewers
pr:
  reviewers:
    by_path:
      "*.swift": [ios-team]
      "*.xib": [ios-team]
      "*.storyboard": [ios-team]
      "Podfile*": [ios-team, devops]

# Large projects benefit from cost awareness
resources:
  cost_warning_threshold: 10.00
```

## CI Integration

When using Claudio-generated branches in CI, apply the same DerivedData isolation:

```yaml
# Example GitHub Actions workflow
name: iOS Build and Test

on:
  pull_request:
    paths:
      - '**/*.swift'
      - '*.xcodeproj/**'
      - '*.xcworkspace/**'
      - 'Package.swift'
      - 'Podfile*'

jobs:
  build:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4

      - name: Select Xcode
        run: sudo xcode-select -s /Applications/Xcode_15.2.app

      - name: Build
        run: |
          xcodebuild -workspace MyApp.xcworkspace \
            -scheme MyApp \
            -derivedDataPath ./DerivedData \
            -destination 'platform=iOS Simulator,name=iPhone 15' \
            build

      - name: Test
        run: |
          xcodebuild test -workspace MyApp.xcworkspace \
            -scheme MyApp \
            -derivedDataPath ./DerivedData \
            -destination 'platform=iOS Simulator,name=iPhone 15' \
            -resultBundlePath ./TestResults

      - name: Upload Test Results
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: test-results
          path: ./TestResults
```

This ensures CI builds use the same isolation strategy as your local Claudio worktrees.

## What You Learned

- How Xcode's DerivedData interacts with git worktrees
- Strategies for managing build artifacts across parallel tasks
- SPM and dependency management in worktrees
- Testing strategies that avoid simulator conflicts
- Handling Xcode project file conflicts
- Performance optimization for iOS development
- CI integration with the same isolation patterns

## Next Steps

- [Feature Development](feature-development.md) - General parallel development patterns
- [Large Refactor](large-refactor.md) - Coordinating major iOS architecture changes
- [Configuration Guide](../guide/configuration.md) - Customize for your team

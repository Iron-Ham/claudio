# Android Development: Using Claudio with Android Studio Projects

**Time: 25-35 minutes**

This tutorial explains how to use Claudio effectively with Android projects using Gradle, covering build cache management, emulator coordination, and handling Android-specific challenges.

## Overview

Claudio's git worktree architecture provides significant benefits for Android development:

- **Isolated build directories**: Each worktree has separate Gradle builds
- **No project sync issues**: Avoid the "syncing Gradle" problem when switching branches
- **Parallel feature development**: Work on multiple features without IDE conflicts
- **Gradle cache sharing**: Global Gradle cache speeds up builds
- **Emulator coordination**: Multiple emulators for parallel testing

## Prerequisites

- Claudio initialized in your Android project (`claudio init`)
- Android Studio or command-line tools installed
- Java JDK 17+ installed
- Familiarity with basic Claudio operations (see [Quick Start](quick-start.md))

## Understanding Android and Git Worktrees

### How Gradle Works with Worktrees

Gradle uses multiple storage locations:

| Location | Purpose | Shared? |
|----------|---------|---------|
| `~/.gradle/caches` | Downloaded dependencies | Yes |
| `~/.gradle/wrapper` | Gradle distributions | Yes |
| `./build` | Project build output | No (per-worktree) |
| `./.gradle` | Project Gradle cache | No (per-worktree) |

This means:

```
Main repo:
├── build/              # Build output
├── .gradle/            # Project cache
├── app/build/          # App module build
└── ...

.claudio/worktrees/abc123/
├── build/              # Separate build output
├── .gradle/            # Separate project cache
├── app/build/          # Separate module builds
└── ...
```

### Build Time Implications

| Project Size | Clean Build | Incremental Build | Full Test Suite |
|-------------|-------------|-------------------|-----------------|
| Small (~20 modules) | 2-5min | 30s-1min | 1-3min |
| Medium (~50 modules) | 5-10min | 1-3min | 3-8min |
| Large (~100 modules) | 10-20min | 3-5min | 10-20min |
| Enterprise | 20min+ | 5-10min | 20min+ |

**Key insight**: Android builds are slow. Plan parallel tasks knowing each worktree needs an initial build, but incremental builds are much faster.

## Strategy 1: Full Isolation (Recommended)

Best for: Most Android projects.

### Benefits

- Complete build isolation
- No Gradle sync conflicts
- Each worktree can be opened in Android Studio independently
- Parallel builds possible with sufficient resources

### Workflow

```bash
# Start a session
claudio start feature-work

# Add tasks (press 'a' in TUI)
```

**Task 1:**
```
Implement the user profile screen in feature/profile module.

1. Create ProfileFragment.kt with UI
2. Create ProfileViewModel.kt with state management
3. Add unit tests in ProfileViewModelTest.kt
4. Build: ./gradlew :feature:profile:assembleDebug
5. Test: ./gradlew :feature:profile:testDebugUnitTest
```

**Task 2:**
```
Add the settings screen in feature/settings module.

1. Create SettingsFragment.kt
2. Create SettingsViewModel.kt
3. Build: ./gradlew :feature:settings:assembleDebug
4. Test: ./gradlew :feature:settings:testDebugUnitTest
```

### Opening in Android Studio

Each worktree can be opened in a separate Android Studio window:

```bash
# Open worktree in Android Studio
open -a "Android Studio" .claudio/worktrees/abc123

# Or from command line
studio .claudio/worktrees/abc123
```

## Strategy 2: Shared Gradle Home (For Faster Builds)

Best for: Disk-constrained environments.

### Configuration

Gradle home is already shared by default (`~/.gradle`). For additional sharing:

```properties
# gradle.properties
org.gradle.caching=true
org.gradle.parallel=true
org.gradle.daemon=true
```

### Build Cache Configuration

Enable build cache for cross-project sharing:

```kotlin
// settings.gradle.kts
buildCache {
    local {
        directory = File(System.getProperty("user.home"), ".gradle-build-cache")
    }
}
```

## Strategy 3: Module-Based Development

Best for: Multi-module projects.

### Module Structure

```
app/
├── app/                    # Main app module
├── core/
│   ├── data/               # Data layer
│   ├── domain/             # Domain layer
│   └── ui/                 # UI components
├── feature/
│   ├── home/               # Home feature
│   ├── profile/            # Profile feature
│   └── settings/           # Settings feature
└── ...
```

### Task Assignment

Assign instances to specific modules:

**Instance 1 - Core Data:**
```
Implement repository in core/data module.

./gradlew :core:data:assembleDebug
./gradlew :core:data:testDebugUnitTest
```

**Instance 2 - Feature Module:**
```
Implement home feature in feature/home module.

./gradlew :feature:home:assembleDebug
./gradlew :feature:home:testDebugUnitTest
```

## Testing Strategies

### Unit Tests (Parallel-Safe)

Unit tests run independently per module:

**Task 1:**
```
Run unit tests for profile feature:
./gradlew :feature:profile:testDebugUnitTest
```

**Task 2:**
```
Run unit tests for data layer:
./gradlew :core:data:testDebugUnitTest
```

### Instrumented Tests (Requires Coordination)

Instrumented tests need emulators. Options:

**Option A: Different Emulators**
```
Run instrumented tests on emulator-5554:
./gradlew :app:connectedDebugAndroidTest -Pandroid.testInstrumentationRunnerArguments.device=emulator-5554
```

**Option B: Create Multiple Emulators**
```bash
# Create dedicated test emulators
avdmanager create avd -n test_avd_1 -k "system-images;android-34;google_apis;x86_64"
avdmanager create avd -n test_avd_2 -k "system-images;android-34;google_apis;x86_64"

# Start emulators on different ports
emulator -avd test_avd_1 -port 5554 &
emulator -avd test_avd_2 -port 5556 &
```

**Option C: Use Managed Devices (Gradle)**
```kotlin
// build.gradle.kts
android {
    testOptions {
        managedDevices {
            devices {
                create<com.android.build.api.dsl.ManagedVirtualDevice>("pixel6Api34") {
                    device = "Pixel 6"
                    apiLevel = 34
                    systemImageSource = "google"
                }
            }
        }
    }
}
```

Then in tasks:
```
Run tests on managed device:
./gradlew pixel6Api34DebugAndroidTest
```

### Screenshot Tests

For screenshot/UI tests:

```
Run screenshot tests:
./gradlew :app:verifyPaparazziDebug

Update baselines:
./gradlew :app:recordPaparazziDebug
```

## Common Conflict Points

### File Conflicts

| File | Risk | Mitigation |
|------|------|------------|
| `build.gradle.kts` | HIGH | Coordinate dependency changes |
| `settings.gradle.kts` | HIGH | One instance handles module changes |
| `gradle.properties` | MEDIUM | Avoid parallel config changes |
| `AndroidManifest.xml` | MEDIUM | Module-specific manifests reduce risk |
| Kotlin/Java files | LOW | Different modules per instance |

### Task Design for Android

**Good decomposition:**
```
├── feature/profile/     (Profile feature)
├── feature/settings/    (Settings feature)
├── core/data/           (Data layer)
└── core/ui/             (UI components)
```

**Risky decomposition:**
```
├── Add login (touches app, auth, data, ui)
├── Add signup (touches app, auth, data, ui)
└── Add profile (touches app, profile, data, ui)
```

### Handling build.gradle.kts Conflicts

**Option A: Pre-add dependencies**
```
First, add all required dependencies in libs.versions.toml:
- Add retrofit for networking
- Add room for database
- Add hilt for DI

./gradlew build  # Verify dependencies
```

**Option B: One instance for dependencies**
```
Update version catalog and module dependencies:

1. Edit gradle/libs.versions.toml
2. Update app/build.gradle.kts
3. Run: ./gradlew dependencies
```

## Compose vs View-Based Development

### Jetpack Compose

Compose projects benefit from faster iteration:

```
Implement the Profile screen with Compose.

1. Create ProfileScreen.kt composable
2. Create ProfileViewModel.kt with UI state
3. Add preview: @Preview annotation
4. Build: ./gradlew :feature:profile:assembleDebug
5. Test: ./gradlew :feature:profile:testDebugUnitTest
```

### View-Based (XML)

For traditional View-based development:

```
Implement the Settings screen.

1. Create fragment_settings.xml layout
2. Create SettingsFragment.kt
3. Create SettingsViewModel.kt
4. Build: ./gradlew :feature:settings:assembleDebug
```

### Mixed Projects

For projects with both:

```
Add a new Compose screen to the existing View-based app.

1. Ensure compose dependencies are in build.gradle.kts
2. Create composable in the existing module
3. Integrate using ComposeView in Fragment/Activity
```

## Resource and Asset Management

### String Resources

Avoid parallel string resource modifications:

```
Add strings for the profile feature only.
Edit feature/profile/src/main/res/values/strings.xml

Do NOT modify app/src/main/res/values/strings.xml
```

### Drawables and Assets

Keep assets module-specific:

```
Add icons for the home feature.
Place in feature/home/src/main/res/drawable/

Use module-specific resources to avoid conflicts.
```

## Performance Tips

### 1. Use Gradle Build Scans

Analyze build performance:

```
Build with scan:
./gradlew assembleDebug --scan
```

### 2. Enable Parallel Builds

```properties
# gradle.properties
org.gradle.parallel=true
org.gradle.workers.max=4
```

### 3. Use Configuration Cache

```properties
# gradle.properties
org.gradle.configuration-cache=true
```

### 4. Targeted Builds

Build only what's needed:

```
# Instead of full build
./gradlew assembleDebug

# Build specific module
./gradlew :feature:profile:assembleDebug
```

### 5. Skip Tests During Development

```
Quick build without tests:
./gradlew assembleDebug -x test -x lint
```

## Example: Building a Complete Feature

### Scenario

Implementing a shopping cart feature with:
- Cart UI
- Cart repository
- Database storage
- API integration

### Session Setup

```bash
claudio start cart-feature
```

### Tasks

**Task 1 - Data Layer:**
```
Implement cart data layer in core/data.

1. Create CartEntity.kt for Room database
2. Create CartDao.kt with CRUD operations
3. Create CartRepository.kt interface and implementation
4. Add Room migration if needed

./gradlew :core:data:assembleDebug
./gradlew :core:data:testDebugUnitTest
```

**Task 2 - Domain Layer:**
```
Implement cart domain logic in core/domain.

1. Create CartItem.kt domain model
2. Create AddToCartUseCase.kt
3. Create GetCartUseCase.kt
4. Create RemoveFromCartUseCase.kt

./gradlew :core:domain:assembleDebug
./gradlew :core:domain:testDebugUnitTest
```

**Task 3 - Feature UI:**
```
Implement cart UI in feature/cart.

1. Create CartScreen.kt composable
2. Create CartViewModel.kt with UI state
3. Create CartItemCard.kt component
4. Add navigation to cart from home

./gradlew :feature:cart:assembleDebug
./gradlew :feature:cart:testDebugUnitTest
```

**Task 4 - Integration:**
```
Integrate cart feature into app.

1. Add cart module to settings.gradle.kts
2. Add navigation route in app module
3. Add Hilt modules for DI
4. Test full flow

./gradlew :app:assembleDebug
./gradlew :app:testDebugUnitTest
```

## Configuration Recommendations

For Android projects:

```yaml
# ~/.config/claudio/config.yaml

# Android builds are slow
instance:
  activity_timeout_minutes: 60
  completion_timeout_minutes: 90

# Assign reviewers by module
pr:
  reviewers:
    by_path:
      "app/**": [android-team]
      "core/data/**": [backend-team, android-team]
      "feature/**": [android-team]
      "*.gradle.kts": [tech-lead]
      "gradle/**": [tech-lead]

# Android development can be expensive
resources:
  cost_warning_threshold: 12.00
```

## CI Integration

Example GitHub Actions workflow:

```yaml
name: Android Build and Test

on:
  pull_request:
    paths:
      - '**/*.kt'
      - '**/*.java'
      - '**/*.xml'
      - '*.gradle.kts'
      - 'gradle/**'

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          distribution: 'temurin'
          java-version: '17'

      - name: Setup Gradle
        uses: gradle/actions/setup-gradle@v3

      - name: Build
        run: ./gradlew assembleDebug

      - name: Unit Tests
        run: ./gradlew testDebugUnitTest

      - name: Lint
        run: ./gradlew lintDebug

  instrumented-tests:
    runs-on: macos-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-java@v4
        with:
          distribution: 'temurin'
          java-version: '17'

      - name: Setup Gradle
        uses: gradle/actions/setup-gradle@v3

      - name: AVD Cache
        uses: actions/cache@v4
        with:
          path: |
            ~/.android/avd/*
            ~/.android/adb*
          key: avd-api-34

      - name: Run Instrumented Tests
        uses: reactivecircus/android-emulator-runner@v2
        with:
          api-level: 34
          arch: x86_64
          script: ./gradlew connectedDebugAndroidTest
```

## Troubleshooting

### "Could not resolve all dependencies"

Dependency resolution failure.

**Solution**:
```bash
./gradlew --refresh-dependencies
# Or clean Gradle cache
rm -rf ~/.gradle/caches/modules-2/files-2.1/
```

### "Gradle sync failed"

Project configuration issues.

**Solution**:
```bash
# Invalidate caches
rm -rf .gradle build
./gradlew clean
./gradlew assembleDebug
```

### Emulator won't start

ADB or emulator conflicts.

**Solution**:
```bash
# Kill existing adb server
adb kill-server
adb start-server

# List running emulators
adb devices
```

### Build OOM (Out of Memory)

Gradle running out of memory.

**Solution**:
```properties
# gradle.properties
org.gradle.jvmargs=-Xmx4g -XX:MaxMetaspaceSize=1g
```

### "Resource linking failed"

Resource conflicts in merged manifest.

**Solution**:
```
Check AndroidManifest.xml for duplicate entries.
Use tools:replace for conflicting attributes.
```

## What You Learned

- How Gradle caching works with worktrees
- Strategies for module-based development
- Emulator coordination for instrumented tests
- Handling build.gradle.kts conflicts
- Performance optimization techniques
- CI integration patterns

## Next Steps

- [iOS Development](ios-development.md) - Similar patterns for Apple platforms
- [Full-Stack Development](fullstack-development.md) - Android with backend APIs
- [Configuration Guide](../guide/configuration.md) - Customize for your team

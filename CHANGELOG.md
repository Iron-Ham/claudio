# Changelog

All notable changes to Claudio will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Self-Improving AGENTS.md** - Restructured AGENTS.md into a living document with a self-improvement protocol that instructs agents to update guidelines based on their learnings. Added agent-curated sections (Architecture Map, Known Pitfalls, Codebase Patterns, Testing Notes, Build & Toolchain) seeded with knowledge from codebase review. Created directory-level AGENTS.md files for `internal/mailbox/`, `internal/taskqueue/`, and `internal/tui/` with package-specific pitfalls and patterns.

### Fixed

- **Manager.Stop() Deadlock** - Fixed deadlock in `team.Manager.Stop()` where holding the mutex through `wg.Wait()` blocked `monitorTeamCompletion` from publishing `TeamCompletedEvent` (which triggered `onTeamCompleted` inline, requiring the same mutex). Stop now releases the lock before waiting for goroutines, with a `started=false` guard to ensure racing handlers bail out. (#651)
- **Mailbox Watch Race** - Fixed goroutine scheduling race in `mailbox.Watch()` where the initial message snapshot was taken inside the goroutine, allowing messages sent immediately after `Watch()` returns to be missed. Snapshot is now taken synchronously before the goroutine launches.

### Added

- **Team-Based TripleShot** - New `TeamCoordinator` (`internal/orchestrator/workflows/tripleshot/teamwire/`) adapts the TripleShot workflow to the Orchestration 2.0 team infrastructure. Each attempt becomes a self-coordinating team with Bridge execution, and the judge is dynamically added via `team.Manager.AddTeamDynamic` after all attempts complete. Includes adapter types connecting tripleshot interfaces to bridge interfaces and two new event types (`TripleShotAttemptCompletedEvent`, `TripleShotJudgeCompletedEvent`). (#645)

- **Bridge Execution Layer** - Bridge package (`internal/bridge/`) and adapter wiring (`internal/orchestrator/bridgewire/`) that connect team Hubs to real Claude Code instances. Each Bridge claims tasks from a team's queue, spawns worktree + tmux instances, monitors for completion via sentinel files, and reports outcomes back to the queue. PipelineExecutor subscribes to pipeline phase transitions and attaches Bridges to execution-phase teams automatically. (#647)

- **Pipeline Orchestration** - Plan decomposer and multi-phase pipeline (`internal/pipeline/`) for team-based execution. Groups tasks by file affinity using union-find, then orchestrates sequential phases (planning → execution → review → consolidation). Each phase runs its own Manager with scoped teams. Adds pipeline lifecycle events and dynamic team addition to the team Manager. (Phase 3 of Orchestrator of Orchestrators, #637)

- **Multi-Team Execution** - Multi-team orchestration layer (`internal/team/`) that runs multiple teams in parallel, each with its own Coordination Hub. Supports inter-team dependency ordering, per-team budget tracking with exhaustion detection, and inter-team message routing via existing mailbox infrastructure. Adds team lifecycle events (created, phase changed, completed, budget exhausted) and inter-team message events to the event bus. (#637)

- **Coordination Hub** - Integration hub (`internal/coordination/`) that wires all Orchestration 2.0 components together for a single session. Creates the full task pipeline (TaskQueue → EventQueue → Gate), event-driven observers (Adaptive Lead, Scaling Monitor), and communication infrastructure (Context Propagator, File Lock Registry, Mailbox). Provides a single `Start`/`Stop` lifecycle and accessor methods for all components. (#637)

- **Inter-Instance Mailbox** - File-based messaging system (`internal/mailbox/`) enabling cross-instance communication during orchestration. Supports broadcast and targeted messages with types including discovery, claim, release, warning, question, answer, and status. Includes prompt injection formatting and event bus integration. (#629)
- **Dynamic Task Queue** - Dependency-aware task queue (`internal/taskqueue/`) with self-claiming and work-stealing, replacing static execution batch ordering. Instances claim tasks as dependencies are satisfied, eliminating idle time between execution groups. Includes failed task redistribution with retry, stale claim cleanup, state persistence, and event bus integration. (#630)
- **Per-Task Approval Gates** - Approval gate system (`internal/approval/`) that intercepts task transitions to require human approval before execution. Tasks with `RequiresApproval` flag pause in `TaskAwaitingApproval` state until explicitly approved or rejected. Uses decorator pattern wrapping EventQueue. (#631)
- **Context Propagation** - Context propagation manager (`internal/contextprop/`) for sharing discoveries and warnings between instances. Adds filtered message formatting (`FormatFiltered`) to mailbox with type, time, sender, and count filters. (#632)
- **Peer Debate Protocol** - Structured debate system (`internal/debate/`) enabling instances to challenge, defend, and resolve disagreements. Adds challenge, defense, and consensus message types to mailbox. Tracks debate rounds and publishes lifecycle events. (#633)
- **Adaptive Lead** - Event-driven dynamic task coordinator (`internal/adaptive/`) that monitors queue events to track workload distribution across instances, generate scaling recommendations, and support task reassignment. (#634)
- **File Conflict Prevention** - Advisory file locking registry (`internal/filelock/`) using mailbox claim/release messages to coordinate file ownership between instances. Supports scoped claims and concurrent conflict detection. (#635)
- **Elastic Scaling** - Queue-depth-based scaling policy engine (`internal/scaling/`) with configurable thresholds, cooldown periods, and instance limits. Includes an event-driven monitor that watches queue depth changes and emits scaling decisions. (#636)

## [0.16.1] - 2026-02-05

### Fixed

- **Process Cleanup on Exit** - Closing Claudio now reliably kills all Claude backend processes across all code paths, including PR workflows. Previously, `tmux kill-session` could leave orphaned Claude CLI processes if they ignored SIGHUP. The shutdown now captures the full process tree before stopping, polls for graceful exit, kills the per-instance tmux server (not just the session), and force-kills any surviving processes via SIGKILL. The shutdown sequence is consolidated into a shared `tmux.GracefulShutdown()` helper used by all stop paths.

- **Group Dismiss Freezing** - Fixed a bug where killing a group via `gq` would freeze the TUI and corrupt the display. The group dismiss operation now runs asynchronously to avoid blocking the main thread, and warning messages no longer write directly to stderr (which corrupts the Bubble Tea TUI). Users now see a "Dismissing N instance(s)..." message while the operation runs in the background.

- **Tripleshot Adversarial UI Nesting** - Fixed a bug where tripleshot attempts in adversarial mode were not immediately nested under "Attempt N" sub-groups when using the async startup path. Previously, attempts were only nested when a new round started, causing a flat instance list during the initial "preparing" phase. Now attempts are properly nested immediately when created via `CreateAttemptStubs`.

- **Tripleshot Adversarial Round Disambiguation** - Fixed a bug where completing a round in tripleshot adversarial mode could spawn multiple reviewers. When an implementer completed after a rejection (round 2+), `ProcessAttemptCompletion` was incorrectly resetting `ReviewRound` to 1 and spawning duplicate reviewers. Now the code: (1) skips processing if already under review, (2) preserves the current review round, and (3) validates review file round numbers to ignore stale files from prior rounds.

## [0.16.0] - 2026-02-02

### Added

- **Codex Backend Support** - Added configurable Codex CLI support alongside Claude, including backend selection, backend-aware state detection, and AI-assisted workflows using the selected backend.
- **Adversarial Mixed-Backend Configuration** - Added `adversarial.reviewer_backend` configuration option to specify a different AI backend for the reviewer role in adversarial sessions. This enables workflows like "Claude as implementer + Codex as reviewer" for cross-model code review. When not set, both roles use the global `ai.backend` setting.

### Changed

- **Codex Approval Default** - Default `ai.codex.approval_mode` to `full-auto` for Codex CLI runs.

### Fixed

- **Exit Panic on Conflict Detector** - Fixed panic when exiting Claudio caused by the conflict detector's `Stop()` method being called multiple times (from both `StopSession` and `Shutdown`). The `Stop()` method is now idempotent using `sync.Once`.
- **Session Backend Init** - Initialize AI backend in `NewWithSession` to avoid undefined backend build errors.
- **Codex Detector Thread Safety** - Fixed potential race condition in Codex backend detector initialization using sync.Once for thread-safe lazy initialization.
- **TUI Config AI Settings** - Added `ai.claude.command` and `ai.codex.command` settings to the TUI config editor for customizing CLI binary paths.
- **Adversarial Round Disambiguation** - Ignore stale increment/review files from prior rounds so new rounds don't start early

### Performance

- **TUI Output Rendering Cache** - Cached filtered output line splits and "new output" state to avoid full-buffer processing on every render, improving typing responsiveness in command/search modes. Includes benchmarks demonstrating cache hit performance (~20ns, 0 allocations).

## [0.15.0] - 2026-02-02

### Added

- **Tripleshot Adversarial Mode Grouping** - In tripleshot with adversarial mode enabled, instances are now organized hierarchically by attempt. Each of the three attempts gets its own "Attempt N" sub-group in the sidebar, with current round instances at the top level and previous rounds collapsed under a "Previous Rounds" container. The judge instance remains at the main tripleshot group level. This makes it easy to track which attempt each implementer/reviewer pair belongs to and which round within that attempt.

### Fixed

- **Frozen Session Recovery** - Fixed a critical bug where individual Claude sessions could become permanently frozen/unresponsive while other sessions in the same Claudio instance continued working. When the tmux socket for a session becomes unresponsive (commands timeout but don't definitively fail), the capture loop would retry indefinitely without recovery, causing the display to freeze and input to stop working. Now tracks consecutive capture failures and time since last successful capture; if both exceed thresholds (10 failures AND 30 seconds), the session is force-terminated and marked as completed, allowing users to restart or recover.

- **Process Cleanup on Exit** - Claude processes (running in tmux sessions) are now properly terminated when Claudio exits. Previously, normal quit (`:quit`, Ctrl+C, SIGTERM) would leave Claude processes running indefinitely, causing resource accumulation over time. The new `Shutdown()` method stops all instances while preserving session state for potential resume. Force quit (`:quit!`) behavior remains unchanged.

### Changed

- **Documentation Updates** - Refreshed documentation to cover v0.13.0 and v0.14.0 features: color themes, sidebar scroll navigation (J/K), macOS keyboard shortcuts, `validate` command, background cleanup options, tripleshot adversarial integration, and stuck instance recovery.

### Performance

- **Adversarial Mode Polling Optimization** - Fixed UI hitching/freezing when starting adversarial tasks in large repositories. The sentinel file polling (which checks for increment and review files every 100ms) now uses a fast-path optimization: expected file locations are cached after first discovery, and expensive full directory traversals are rate-limited to once every 5 seconds. This eliminates the overhead of `os.ReadDir()` calls on worktrees with many subdirectories.

## [0.14.1] - 2026-01-30

### Fixed

- **Interactive Config Text Input** - Fixed inability to type lowercase 'k' and 'j' when editing text fields in the interactive config UI (`:config`). The vim-style navigation keys were incorrectly intercepted even when editing string values, preventing users from typing these characters. Navigation with 'k' and 'j' still works for select-type dropdowns and in normal navigation mode.

- **Release Skill Changelog Link** - The `/release` skill now links to the changelog at the release tag (e.g., `blob/v0.14.0/CHANGELOG.md`) instead of `main`, ensuring stable references.

- **Duplicate Previous Rounds in Adversarial Mode** - Fixed a bug where "Round N" entries would appear duplicated in the "Previous Rounds" section of the sidebar. The issue occurred because both `StartImplementer` and `StartReviewer` called `getCurrentRoundGroup`, which would move previous round instances to a sub-group. On the second call, the sub-group had already been moved into "Previous Rounds" and couldn't be found as a direct child, causing a duplicate to be created. Added an idempotency guard that checks if a round was already moved before performing the operation.

## [0.14.0] - 2026-01-30

### Added

- **Release Skills** - Added project-local Claude skills for cutting releases with professional release notes and GitHub Releases. The `/release` skill handles the complete workflow: validating release readiness, parsing CHANGELOG.md, determining semantic version bumps, generating narrative release notes, updating the changelog, creating git tags, and publishing GitHub Releases. Includes `/release-preview` for dry-run previews, `/release-notes` for generating standalone release notes in multiple formats (markdown, Slack, Discord, Twitter, email), and `/release-notes-from-git` for auditing changelog completeness against git history. Skills are automatically available when working in this project without requiring plugin installation.

- **Enhanced Keyboard Navigation** - Added support for macOS-style keyboard shortcuts in input mode and the task prompt writer. Input mode now properly forwards `Opt+Left/Right` (word navigation) and `Opt+Backspace` (word deletion) to Claude instances. The task prompt writer now supports both the `msg.Alt` flag and string-based key reporting for `Opt+Arrow/Backspace`, as well as `Cmd+Left/Right/Backspace` (line navigation and delete-to-line-start) for terminals that forward these keys.

### Changed

- **Adversarial Mode UI Enhancement** - Current round instances in adversarial mode now appear directly in the main adversarial group header instead of being nested under a "[Round X]" sub-group. The group header displays round information inline (e.g., "Refactor auth (Round 3)"), making it immediately visible which round is active. Previous rounds are still organized under the "Previous Rounds" container for historical reference. This flattens the UI for the current round while preserving full round history navigation.

- **Simplified Group Headers** - Removed the `[x/y]` progress counters from group headers in the sidebar. The phase indicator (● for executing, ✓ for completed, ✗ for failed) already provides visual feedback on group status, making the numeric counters redundant visual noise.

- **Tripleshot Adversarial Pair Clarity** - Improved the sidebar display for tripleshot with adversarial mode enabled to show clear implementer/reviewer pairs. Each of the three attempts is now displayed as a "Pair N" with both the implementer status and its paired reviewer status visible together. The display includes a phase indicator showing the current workflow phase (Implementing, Under Review, Judging, Complete), reviewer approval status with scores (e.g., "8/10 ✓"), and a count of active pairs during the review phase. This makes it immediately clear which reviewer is reviewing which implementer's work.

### Removed

- **`claudio tripleshot` CLI Command** - The standalone `claudio tripleshot` command has been removed. TripleShot mode is now exclusively accessed through the TUI via the `:tripleshot` command (or aliases `:triple`, `:3shot`). This consolidates all tripleshot functionality within the standard TUI, providing a more consistent user experience. To use tripleshot, run `claudio start` and then use `:tripleshot` in command mode.

### Performance

- **Input Mode Typing Responsiveness** - Fixed typing lag and stuttering in INPUT mode when interacting with Claude instances. The output filtering logic now caches filtered results per instance and only recomputes when the raw output or filter settings change. Previously, every keystroke triggered expensive string operations (line splitting, regex matching, case conversion) on the entire output buffer, even when nothing had changed.

### Fixed

- **TripleShot-Adversarial Feedback Loop** - Each implementer now properly iterates with reviewer feedback instead of failing on first rejection. When a reviewer rejects an attempt, the implementer is restarted with the reviewer's feedback, issues, and required changes. This continues until the reviewer approves or max rounds are exhausted (uses `adversarial.max_iterations`, default 10, 0 = unlimited). Also fixed the TUI not polling during the adversarial review phase, which prevented reviewer completions from being detected.

- **Alt/Option Arrow Keys in Input Mode** - Fixed Opt+Left and Opt+Right being incorrectly interpreted as Escape key presses by underlying Claude sessions. The issue was caused by sending Alt+key combinations as two separate key events (Escape then arrow) instead of a single atomic Meta+key event. All Alt/Option modified keys now use tmux's `M-` prefix notation (e.g., `M-Left`, `M-Right`), which sends them as proper word-navigation commands.

- **Tripleshot UI Responsiveness** - Fixed UI stalling when starting a tripleshot on large repositories. Worktree creation is now performed asynchronously in parallel, allowing the UI to remain responsive. Instances appear immediately with "Preparing" status while worktrees are created in the background.

- **TUI Tripleshot Config Settings** - The `:tripleshot` command in the TUI now properly respects the `tripleshot.auto_approve` and `tripleshot.adversarial` settings from the config file. Previously, starting a tripleshot from the TUI always used hardcoded defaults, ignoring user configuration.

- **False Positive Stale Detection** - Fixed instances being incorrectly marked as "stuck" in two scenarios: (1) When Claude was actively working but the output wasn't changing (e.g., running explore agents with collapsed output, or showing a static spinner during thinking phase) - the stale counter now only increments when no working indicators (spinners, "Reading...", "Analyzing...", etc.) are present. (2) When Claude was waiting for user input but the patterns didn't match Claude Code's actual UI - the state detection patterns now correctly recognize Claude Code's input prompt (`❯`), plan/auto/focus mode indicators (`⏸`), and case variations in mode cycling hints.

- **Sidebar Auto-Scroll on Navigation** - Fixed sidebar not scrolling when navigating to instances or groups that are off-screen. When using h/l (tab/shift+tab) to switch instances or gn/gp to navigate between groups, the sidebar now automatically scrolls to ensure the selected item is visible. This also fixes a bug where `ensureActiveVisible()` used the wrong position in grouped mode due to not accounting for group headers.

- **Adversarial Mode Sentinel File Search** - Improved file detection for adversarial mode's increment and review files. The system now searches multiple locations (worktree root, subdirectories, and parent directory) to handle cases where Claude writes the file to an unexpected location (e.g., monorepo root instead of worktree, or a subdirectory). When the worktree path is known (round > 1), the prompt now includes the absolute file path for clarity.

- **Hash and Tilde Characters in Input** - Fixed an issue where hash (`#`) and tilde (`~`) characters were not being sent correctly to underlying Claude sessions when using tmux control mode. These characters are now properly quoted to prevent tmux from interpreting them as format specifiers or tilde expansion. Unicode characters like `£`, `€`, and emoji continue to work correctly.

- **Adversarial Mode False Positive Stuck Detection** - Fixed a race condition in adversarial mode where the implementer or reviewer would be incorrectly marked as "stuck" immediately after completion. The issue occurred because the TUI detected the instance as "completed" before Claude had finished writing the sentinel file (`.claudio-adversarial-incremental.json` or `.claudio-adversarial-review.json`). A 3-second grace period is now applied before declaring an instance stuck, allowing time for the file write to complete.

- **Adversarial Mode False Positive Stuck Detection** - Removed automatic stuck detection from adversarial mode, which was causing false positives when Claude was actively thinking/processing. Claude Code shows UI elements like "(shift+Tab to cycle)" even while processing, which triggered incorrect `StateWaitingInput` detection. Adversarial mode now simply polls for completion files like tripleshot does. If an instance genuinely gets stuck, users can interact with it directly or use `:adversarial-retry` to restart.

- **Improved Working State Detection** - Added comprehensive detection of Claude Code's spinner/thinking status words (Thinking, Frosting, Cogitating, Manifesting, Architecting, Razzmatazzing, etc. - the full ~60 word pool) to the working state patterns. This ensures Claude is correctly detected as "working" when these indicators appear in output, preventing false "waiting" state detection.

## [0.13.0] - 2026-01-29

This release introduces **Adversarial Review Mode** and **Color Themes** - two major features that enhance workflow quality and user experience.

### Added

- **Adversarial Review Mode for Tripleshot** - Added `--adversarial` flag to the `tripleshot` command. When enabled, each of the three implementers is paired with a critical reviewer and won't be considered complete until the reviewer approves the work (score >= 8/10). The adversarial phase runs between the working phase and evaluation phase. If a reviewer rejects an attempt, it's marked as failed. The setting can also be configured as a default in `config.yaml` under `tripleshot.adversarial`.

- **Adversarial Flag Infrastructure for Ultraplan (Experimental)** - Added `--adversarial` flag configuration infrastructure to the `ultraplan` command. The flag can be set via CLI or configured in `config.yaml` under `ultraplan.adversarial`. This is marked as EXPERIMENTAL and disabled by default. Note: The workflow integration (spawning reviewers, waiting for approval) is not yet implemented for ultraplan; this is plumbing-only for a follow-up PR.

- **Background Cleanup Jobs** - The `claudio cleanup` command now runs in the background by default, significantly improving performance when there are many worktrees. Resources are snapshotted at command invocation time, ensuring that new worktrees created during cleanup are not affected—even when using `--all-sessions --force --deep-clean`. Use `--foreground` to run synchronously as before. Check job status with `--job-status <job-id>`. Old job files are automatically cleaned up after 24 hours.

- **Auto-Start on Add** - Instances added via `:a` now automatically start by default. This behavior is controlled by the new `session.auto_start_on_add` configuration option (default: `true`). Users who prefer the previous behavior (instances created in pending state requiring manual `:s` to start) can disable this in the interactive config (`:config`) or by setting `session.auto_start_on_add: false` in their config file.

- **Adversarial Stuck Instance Detection and Recovery** - Added automatic detection when adversarial instances (implementer or reviewer) complete without writing their required sentinel files. When an instance finishes work but fails to write `.claudio-adversarial-incremental.json` or `.claudio-adversarial-review.json`, the workflow now transitions to a "stuck" phase and notifies the user with recovery options. Use `:adversarial-retry` command to restart the stuck role. The TUI sidebar shows the stuck status with clear indication of which role (implementer/reviewer) got stuck.

- **Triple-Shot Implementers Auto-Collapse** - When the judge (fourth session) starts running in a tripleshot workflow, the implementers sub-group is now automatically collapsed in the TUI sidebar. This focuses attention on the judge instance while keeping the three implementer instances accessible via manual expand.

- **Color Themes** - Added support for user-selectable color themes in the TUI. Available themes: `default` (original purple/green), `monokai` (classic Monokai editor colors), `dracula` (Dracula theme), `nord` (cool blue-gray Nord theme), `claude-code` (Claude Code inspired orange/coral), `solarized-dark` (Solarized Dark), `solarized-light` (Solarized Light), `one-dark` (Atom One Dark), `github-dark` (GitHub Dark mode), `gruvbox` (retro groove), `tokyo-night` (modern Tokyo nights), `catppuccin` (Catppuccin Mocha pastel), `synthwave` (Synthwave '84 retro neon), and `ayu` (Ayu Dark). Configure via `tui.theme` in config or select interactively via `:config` command. Theme changes apply immediately with live preview.

- **Custom Theme Support** - Users can now create and share custom color themes. Create YAML theme files in `~/.config/claudio/themes/` and they will automatically be discovered and available in the theme selector. Use `claudio config theme create <name>` to generate a template, `claudio config theme export <theme>` to export an existing theme for customization, and `claudio config theme list` to see all available themes. Custom themes support all color definitions including base colors, status colors, diff highlighting, and search highlighting.

- **Enhanced Sidebar Status Display** - Instance status lines in the sidebar now show additional context including elapsed time (e.g., "5m"), cost (e.g., "$0.05"), and files modified count (e.g., "3 files"). Running instances also display "last active" time (e.g., "30s ago"). This helps users quickly understand instance progress without navigating to the stats panel.

- **Enhanced Instance Header** - The instance detail view header now shows files modified count and last activity time for running instances, providing immediate context about what the instance is working on.

- **API Calls in Metrics Display** - The instance metrics line now includes API call count (e.g., "12 API calls") alongside tokens and cost, giving users more insight into instance resource usage.

- **Session Recovery Status in Stats Panel** - The stats panel now displays session recovery state (recovered/interrupted) with the number of recovery attempts, helping users understand if a session was restored from an interruption.

- **Total API Calls in Stats Panel** - Added aggregated API call count across all instances to the session statistics panel.

- **Adversarial Round Auto-Collapse** - Completed rounds in adversarial mode now automatically collapse into sub-groups, keeping the sidebar clean while preserving access to round history. Each round (implementer + reviewer instances) is organized into a "Round N" sub-group. When a round is rejected and a new round starts, the completed round's sub-group automatically collapses. The final approved round remains expanded so users can see the successful review. Users can manually toggle any round's expansion state.

- **Sidebar Scroll-Only Navigation** - Added `J` (Shift+j) and `K` (Shift+k) keybindings to scroll the sidebar viewport without changing the selected instance. This allows users to view instances "above the fold" (indicated by "▲ N more above") without losing their current selection. Previously, users had to cycle through all instances using `tab`/`h`/`l` to see items outside the viewport.

- **Previous Rounds Container** - Adversarial mode now condenses all completed rounds into a single "Previous Rounds" container group, reducing sidebar clutter when tasks span many rounds. When round 2+ starts, the previous round is automatically moved into the "Previous Rounds" container, which is then collapsed. This means users see only two groups—"Previous Rounds" (collapsed) and the current round (expanded)—instead of navigating through many individual collapsed round groups. Users can expand "Previous Rounds" to access any historical round.

- **Async Task Addition** - Task addition via `:a` is now significantly faster. When adding a task, a stub instance appears immediately in the sidebar with "PREP" status while the git worktree is created in the background. Once the worktree is ready, the instance transitions to pending (or auto-starts if configured). This two-phase async approach eliminates UI freezes during worktree creation, which can be slow in large repositories.

### Fixed

- **Stale RUNNING Status After Tmux Server Death** - Fixed a race condition where instances would show as "RUNNING" indefinitely after the tmux server died. When the tmux server dies between the session status check and output capture, the capture error was logged but the session existence wasn't re-verified, leaving instances in a stale RUNNING state with no output updates. Now when capture fails, the code verifies the session still exists and properly marks it as completed if the server/session is gone (#403).

- **Input Mode Not Exited When Plan Editor Opens** - Fixed a UI bug where users in input mode (tmux passthrough) would remain stuck in input mode when an ultraplan became ready and the plan editor opened. Keystrokes intended for plan editor navigation (j/k, Enter, etc.) would be sent to the tmux session instead of being handled by the plan editor. The plan editor now automatically exits input mode when entering, ensuring users can immediately interact with the plan.

- **Ultraplan Task Instances Not Auto-Writing Completion Files** - Fixed an issue where ultraplan task instances would not automatically write their completion files (`.claudio-task-complete.json`, `.claudio-synthesis-complete.json`, `.claudio-revision-complete.json`, `.claudio-group-consolidation-complete.json`) until explicitly prompted by the user. The completion protocol instructions in the prompt templates used passive/conditional language ("When your task is complete...") which Claude instances interpreted as "wait for user confirmation." Strengthened the completion protocol language across all prompt templates to emphasize that writing the completion file is the FINAL MANDATORY ACTION that must happen AUTOMATICALLY, that the orchestrator is BLOCKED waiting for it, and that work is NOT recorded until the file is written.

- **Plan File Written to Wrong Location in Worktrees** - Fixed a bug where ultraplan coordinators would write `.claudio-plan.json` to the main repository root instead of the worktree directory. The planning prompt instructed Claude to write "at the repository root", which was ambiguous when running in a git worktree—Claude would follow the worktree's `.git` reference and write to the main repo. Changed the prompt to explicitly say "in your current working directory" with the `./` prefix, ensuring the plan file is written to the correct worktree location where the detection code expects it.
- **Theme Persistence** - Theme selection now persists across application restarts. The TUI's `Init()` function now applies the user's saved theme preference from config at startup.
- **Theme Config Validation** - Invalid theme names in config are now caught during validation and reported with a clear error message listing valid theme options.
- **Adversarial Sub-Group ID Edge Case** - Fixed an edge case where sub-group IDs would be malformed (e.g., `-round-1` instead of `session-id-round-1`) when the adversarial session's GroupID was empty. Now falls back to using the session ID as the prefix.

### Changed

- **Removed Dead Code from Adversarial Workflow** - Removed `AdversarialRoundCompleteMsg` message type and `OnRoundSubGroupCreated`/`OnRoundComplete` callbacks that were defined but never used. The TUI handles round completion directly via `collapseAdversarialRound` method.

## [0.12.7] - 2026-01-23

### Fixed

- **Frozen Output from Transient Tmux Errors** - Fixed a critical bug where tmux sessions would appear frozen (no output updates, input not showing) due to transient tmux errors being misinterpreted as "session doesn't exist". Previously, any non-timeout error from tmux `display-message` or `has-session` commands would cause the capture loop to think the session had ended, setting `running = false` and stopping all output updates. Now, only definitive "session gone" errors (socket doesn't exist, session not found, no server running, tmux not installed) are treated as terminal—all other errors (broken pipe, signal killed, generic exit status) are assumed to be transient and the capture loop continues retrying. Also fixed `checkSessionExists` to properly capture stderr using `CombinedOutput()` for accurate error message detection.

## [0.12.6] - 2026-01-22

### Fixed

- **Multi-Pass Planning Session Resume** - Fixed a critical bug where resuming an ultraplan session in multi-pass mode (`:ultraplan --multi-pass`) would fail to trigger the plan evaluator. When the TUI was closed while the 3 parallel planners were running, re-attaching to the session would incorrectly check `CoordinatorID` (which is not used in multi-pass mode) and restart planning from scratch, overwriting `PlanCoordinatorIDs` with new instance IDs. The original planners' completion events would then be orphaned, causing the evaluator to never kick off. The fix adds proper multi-pass handling in session resume: it now correctly checks for existing planners in `PlanCoordinatorIDs`, collects any completed plans from worktrees, and triggers the evaluator when all planners have finished. Also fixed an edge case where missing planner instances (GetInstance returning nil) would cause false negatives in the all-processed check, preventing the evaluator from being triggered.

### Changed

- **Instance Manager Callbacks Required at Construction** - Callbacks (OnStateChange, OnMetrics, OnTimeout, OnBell) are now passed via `ManagerCallbacks` struct in `ManagerOptions` at construction time, rather than being set separately via setter methods. This prevents the "leaky abstraction" bug where `Start()`/`Reconnect()` could be called without callbacks configured. The callback setter methods are now deprecated.

- **Runtime Guards on Instance Lifecycle Methods** - `Start()`, `StartWithResume()`, and `Reconnect()` now return `ErrManagerNotConfigured` if the Manager was not properly constructed via `NewManagerWithDeps`. This provides defense-in-depth against misconfigured managers.

- **Improved Error Handling** - The `Reconnect()` method now logs a warning if enabling monitor-bell fails, instead of silently ignoring the error. The `ReconnectInstance()` fallback logic now properly propagates configuration errors instead of masking them.

### Removed

- **Dead Code Cleanup** - Removed unused `RecoverSession()` method from orchestrator that had duplicate (and incomplete) callback configuration code, which was missing the metrics callback. Also removed the now-unused `configureInstanceCallbacks()` method since callbacks are configured at construction time.

## [0.12.5] - 2026-01-22

### Fixed

- **Session Attachment Output Capture** - Fixed a critical bug where attaching to an existing session would not configure instance callbacks, causing all instances to appear frozen with no output updates. The previous fix (#564) created instance managers but the reconnection code called `mgr.Reconnect()` directly instead of `orch.ReconnectInstance()`, which skipped the callback configuration (state changes, metrics, timeouts, bells). Now uses the proper high-level reconnection method that configures all necessary callbacks.

## [0.12.4] - 2026-01-21

### Fixed

- **`:D` and `:d` Command Error Display** - Fixed an issue where the `:D` (remove instance) and `:d` (show diff) commands would appear to freeze the TUI with a "Removing instance..." or "Loading diff..." message when an error occurred. The error message was being set but not displayed because the info message wasn't cleared first. Now errors from async operations properly clear the progress message before displaying the error.

- **Regular Instance Status Display** - Fixed the status abbreviation (WAIT, WORK, DONE, etc.) not being shown for regular ungrouped instances in the sidebar. Grouped instances showed the status on a second line, but regular instances only displayed the colored dot without the status text. Both now consistently show the status indicator on a second line below the instance name.

## [0.12.3] - 2026-01-21

### Fixed

- **TUI Freeze on `:D` and `:d` Commands** - Fixed the TUI freezing when using the `:D` (remove instance) and `:d` (show diff) commands. These commands now execute git operations asynchronously, keeping the UI responsive while the worktree removal or diff computation runs in the background.

- **Async Command Nil Safety** - Added nil-safety checks to all async task commands (`AddTaskAsync`, `AddTaskFromBranchAsync`, `AddDependentTaskAsync`, `RemoveInstanceAsync`, `LoadDiffAsync`) to prevent panics if orchestrator or session is nil. These edge cases now return proper error messages instead of crashing.

- **TripleShot Completion File Detection** - Fixed an issue where tripleshot completion files were not detected when Claude instances wrote them to a subdirectory instead of the worktree root. This could happen in monorepos where Claude `cd`'d into a project subdirectory before writing the completion file. The detection now searches immediate subdirectories as a fallback, ensuring completion files are found regardless of where they were written.

- **Session Attachment Instance Manager Creation** - Fixed a critical bug where attaching to an existing session would not create instance managers for loaded instances. This caused instances to appear in the TUI but be non-functional - they couldn't be restarted, and their output wouldn't be captured. The fix adds `EnsureInstanceManagers()` which is called after loading a session to ensure all instances have proper managers. Additionally, `ReconnectInstance` and `ResumeInstance` now always call `ClearTimeout()` to prevent stale detection from immediately re-triggering after restart.

## [0.12.2] - 2026-01-21

### Added

- **Validate Command** - New `claudio validate` command to validate ultraplan JSON files before execution. Checks for valid JSON syntax, required fields, task dependency validity (no cycles, no missing references), file conflicts between parallel tasks, and provides warnings for high complexity tasks. Supports both human-readable and JSON output formats (`--json` flag) for integration with CI/CD pipelines. Planning prompts now instruct Claude to run validation after generating a plan file, ensuring correct JSON structure before execution begins.

### Changed

- **Adversarial Review Score Threshold** - Updated the reviewer prompt to make the minimum score threshold a mandatory requirement for approval. The prompt now uses emphatic language ("CRITICAL: Approval MUST meet a minimum score of X") to clearly communicate that the score threshold is a hard requirement, not a suggestion.

- **Adversarial Increment File Schema Enforcement** - Strengthened the implementer prompt to prevent custom JSON schemas in increment files. The prompt now explicitly forbids adding custom fields, shows a "WRONG" example with common mistakes (like `phases_completed`, `modules_created`), and provides a "CORRECT" example demonstrating how to include detailed information in the standard string fields.

### Fixed

- **Adversarial Worktree Path Restoration** - Fixed an issue where adversarial sessions could fail to detect increment/review files after session restoration. The worktree path is now persisted directly in the adversarial session (not just looked up from instances), making restoration more reliable. Added warning logs when worktree path restoration fails.

- **MultiPlan Planner Navigation** - Auto-collapsed multiplan planner instances are no longer navigable via tab/shift-tab or h/l keys. Previously, navigating to a collapsed instance would auto-expand the group, unintentionally exposing the planner instances. Now, locked-collapsed groups (like the "Planning Instances" sub-group) remain collapsed during navigation, keeping the sidebar clean. Users can still manually expand locked groups via group toggle (gc), which clears the lock and allows normal navigation afterward.

- **Ultraplan File Loading** - Fixed an issue where loading an ultraplan from a file (`:ultraplan --plan <file>`) would silently drop the `issue_url` and `no_code` fields during plan parsing. These optional fields are now correctly preserved, ensuring that external issue tracker links work properly and no-code tasks (verification/testing tasks) are handled correctly.

- **Sidebar UI Duplication** - Fixed a bug where sidebar content could be duplicated or overflow when adding/removing tasks or groups. The issue was caused by a mismatch between item count and actual rendered line count in the sidebar. The sidebar now properly tracks actual line usage, preventing content from overflowing its container and causing visual artifacts.

- **Plan File Path in Subdirectories** - Fixed planning coordinators writing `.claudio-plan.json` to incorrect locations when Claudio is started from a repository subdirectory. The planning prompts now explicitly instruct Claude to write the plan file at the repository root, preventing path resolution issues during multi-pass planning.

- **Terminal Pane Freeze** - Fixed an issue where the TUI could freeze when the terminal pane was visible and tmux became unresponsive. The `capture-pane` command now has a 500ms timeout, preventing the entire event loop from blocking. When the capture times out, the previous output is preserved so the display doesn't go blank.

- **Conflict Detector Missing Worktree Path** - Fixed a warning that appeared when opening projects with stale session data: "failed to watch instance for conflicts: ... no such file or directory". The conflict detector now validates that the worktree path exists and is a directory before attempting to watch it, providing clearer error messages when sessions reference worktrees that have been deleted.

## [0.12.0] - 2026-01-21

This release brings **Dependency Graph View & Orchestration Guides** - a new DAG-based sidebar visualization for understanding task dependencies at a glance, plus comprehensive documentation for all orchestration modes.

### Added

- **Dependency Graph View** - New sidebar visualization mode that displays instances organized by their dependency levels. Toggle with `d` key to see a DAG (Directed Acyclic Graph) view showing root tasks, intermediate levels, and final tasks. Features include topological sorting, status indicators, and dependency relationship arrows.

- **Comprehensive Orchestration Mode Documentation** - Added complete documentation for all orchestration modes:
  - **Plan Mode Guide** (`docs/guide/plan.md`) - Task decomposition and GitHub Issues generation
  - **TripleShot Guide** (`docs/guide/tripleshot.md`) - Parallel implementation with judge evaluation
  - **Adversarial Review Guide** (`docs/guide/adversarial.md`) - Iterative implementation with reviewer feedback
  - **Choosing Orchestration Modes Tutorial** (`docs/tutorials/choosing-orchestration-modes.md`) - Decision guide with practical examples
  - Updated CLI reference with adversarial command documentation
  - Updated guide index with links to new guides

- **MultiPlan Planner Collapse** - When the plan manager/evaluator starts in a MultiPlan session, the three planner instances are automatically collapsed into a sub-group called "Planning Instances". This keeps the sidebar clean during evaluation while preserving access to planner instances if needed. The sub-group is auto-collapsed in the UI but can be expanded to view the individual planners.

### Changed

- **System Alert Sound** - Notification sound now uses the macOS system alert sound (configured in System Settings > Sound) instead of the hardcoded Glass.aiff chime. Users who prefer a specific sound can still set `ultraplan.notifications.sound_path` to a custom audio file.

- **Consolidation Module Extraction** - Refactored `internal/orchestrator/consolidation.go` into a modular package structure:
  - Created `internal/orchestrator/consolidation/` package with shared types and interfaces
  - Extracted `prbuilder/` subpackage for PR content generation (pure data transformation)
  - Extracted `branch/` subpackage for git branch operations with `NamingStrategy`
  - Extracted `conflict/` subpackage for merge conflict detection and handling
  - Extracted `strategy/` subpackage with `Stacked` and `Single` consolidation strategies
  - Added comprehensive tests with 100% coverage for prbuilder, high coverage elsewhere
  - Follows Go best practices: interface segregation, dependency injection, adapter pattern

- **UltraPlan Sidebar Subgroups** - Refactored UltraPlan sidebar rendering to use actual InstanceGroup subgroups instead of custom inline rendering. Planning, execution groups (Group 1, Group 2, etc.), Synthesis, Revision, and Consolidation phases now each have their own subgroup within the main UltraPlan group. This aligns with how tripleshot and adversarial modes render their content and enables standard group navigation and collapse/expand behavior. Added `SubgroupRouter` to automatically route instances to the correct subgroup based on session state.

- **Dead Code Removal** - Removed unreachable code identified by static analysis to improve codebase maintainability:
  - Removed unused `Consolidator` type and all methods from `orchestrator/consolidation.go` (replaced by `group/consolidate/` package)
  - Deleted `orchestrator/consolidation_util.go` entirely (unused `BranchNameGenerator`, `Slugify`, `DeduplicateStrings`)
  - Removed `ExtractInstanceIDFromSession` and `ListSessionTmuxSessions` from `instance/manager.go`
  - Removed unused test helpers from `testutil/testutil.go` (`SetupTestRepoWithContent`, `SetupTestRepoWithRemote`, `CheckoutBranch`, `GetCurrentBranch`, `GetCommitCount`, `HasUncommittedChanges`, `ListWorktrees`, `SkipIfNoTmux`, `SkipIfNoGolangciLint`)
  - Removed `FindUnlockedSessions` from `session/discovery.go`
  - Removed `CreateStackedPR` from `pr/pr.go`
  - Removed `DefaultConfig` from `plan/plan.go`

### Fixed

- **Unified Multi-Workflow Header** - The header now displays status indicators for all active workflow types (UltraPlan, TripleShot, Adversarial) simultaneously. Previously, the header used exclusive priority-based selection, which meant users lost visibility into other running workflows when multiple task types were active.

- **Adversarial Increment Filename** - Renamed the implementer's sentinel file from `.claudio-adversarial-increment.json` to `.claudio-adversarial-incremental.json` for consistent naming with agent search patterns.

- **View Mode Preservation** - Switching from dependency graph view back to list view now correctly restores the previous sidebar mode (flat or grouped). Previously, toggling the dependency view always returned to flat mode, causing users to lose their grouped view state.

- **Adversarial Increment File Validation** - Added comprehensive defense-in-depth validation for the implementer's increment file to prevent malformed submissions from causing workflow failures:
  - **JSON Sanitization**: Automatically handles common LLM output quirks including smart/curly quotes (" " ' '), markdown code blocks (```json ... ```), extra text before/after JSON, and various Unicode quote characters
  - **Structural Validation**: Validates that the file is valid JSON with correct field types before parsing, catching missing or malformed fields with detailed error messages showing the expected JSON structure
  - **Semantic Validation**: Ensures non-empty values for required fields when status is "ready_for_review" (summary, approach, files_modified), while allowing empty fields for "failed" status
  - **Enhanced Prompts**: Updated implementer prompt with explicit field requirements, type specifications, and a "COMMON MISTAKES TO AVOID" section to prevent issues at the source

- **Documentation Bullet Rendering** - Fixed tutorial section lists in the home page not rendering bullets correctly due to missing blank lines between bold headers and list items.

- **Adversarial Session Restoration** - Fixed incremental file detection failing after session resume. The adversarial coordinator's worktree paths were not being restored when a session was loaded from disk, causing the file detection checks to silently return false. Sessions with active adversarial workflows now properly restore the coordinator's worktree paths from the implementer instance, enabling detection of `.claudio-adversarial-incremental.json` files.

## [0.11.0] - 2026-01-18

This release brings **Major Architecture Refactoring & Platform Guides** - a comprehensive refactoring of the Coordinator into a thin facade with specialized orchestrators, plus detailed platform-specific documentation for all major development environments.

### Added

- **Adversarial Review Rejection After Approval** - Users can now reject an "Approved" adversarial review result by having the reviewer write a new failing review file. When the reviewer writes a new completion file with a failing score, the implementer automatically restarts work on the next round.

- **Comprehensive Platform-Specific Documentation** - Added detailed tutorials for using Claudio with all major development platforms:
  - **Web Development (Node.js/React/Vue)** - npm/yarn/pnpm caching, dev server coordination, framework-specific workflows
  - **Go Development** - Module caching, build optimization, workspace patterns, code generation
  - **Python Development** - Virtual environment management, pip/poetry/conda, Django/Flask/FastAPI patterns
  - **Rust Development** - Cargo workspace management, target directory isolation, sccache integration
  - **Android Development** - Gradle build caching, module-based development, emulator coordination
  - **Full-Stack Development** - Docker Compose isolation, database per worktree, API coordination
  - **Monorepo Development** - Sparse checkout optimization, Turborepo/Nx integration, affected-only builds
  - **Data Science & ML** - Jupyter notebook management, experiment tracking, GPU resource coordination

### Fixed

- **Scroll Hint Accuracy** - Updated scroll indicator hint from `[g/G]` to `[0/G]` to match actual keybindings

### Changed

- **Terminal Mode Label Clarity** - Changed terminal pane mode indicator from `[invoke]` to `[project]` for better user understanding. The label now clearly indicates "you're in your project directory" instead of using internal jargon. Added `:termdir project` (and `:termdir proj`) as the primary command alias, with legacy `invoke`/`invocation` aliases preserved for backward compatibility. The terminal header now also displays a toggle hint (e.g., `[:termdir wt]`) showing how to switch to the other mode.

- **Coordinator Refactoring Phases 2-5** - Continued major refactoring of `internal/orchestrator/coordinator.go`:
  - **Phase 2**: Removed duplicate task execution methods (delegating to `ExecutionOrchestrator`)
  - **Phase 3**: Removed duplicate revision methods (delegating to `SynthesisOrchestrator`)
  - **Phase 4**: Created `phase/step/` package with `Resolver` and `Restarter` for step management
  - **Phase 5**: Created `group/consolidate/` package with `Consolidator` for group consolidation
  - Added adapters: `coordinator_step_adapter.go`, `coordinator_consolidate_adapter.go`
  - Moved completion file parsing functions to `types/completion.go`
  - Reduced coordinator.go from 3,271 lines/69 methods to 1,359 lines/38 methods (59% reduction)
  - Established clean separation of concerns between coordination and specialized operations

- **Coordinator Refactoring Phase 1** - Initial refactoring of `internal/orchestrator/coordinator.go` to establish delegation patterns:
  - Extracted notification methods to `coordinator_callbacks.go` (~170 lines)
  - Extracted adapter types to `coordinator_adapters.go` (~75 lines)
  - Removed 6 deprecated functions: `runPlanningSinglePassDirect`, `runSynthesisDirect`, `monitorSynthesisInstance`, `checkForSynthesisCompletionFile`, `onSynthesisReady`, `buildSynthesisPrompt`
  - Removed fallback paths - code now requires phase orchestrators to be initialized
  - Fixed `restartTask()` to delegate to `ExecutionOrchestrator.StartSingleTask()`
  - Added `StartSingleTask()` and `RestartLoop()` to `ExecutionOrchestrator`
  - Created placeholder packages `phase/step/` and `group/consolidator.go` documenting future extraction targets

- **Orchestrator Package Refactoring** - Major refactoring of `internal/orchestrator/` to improve modularity and testability:
  - Extracted workflow coordinators into dedicated subpackages: `workflows/tripleshot/`, `workflows/adversarial/`, `workflows/ralph/`
  - Created shared `types/` package for common type definitions (`TaskCompletionFile`, etc.)
  - Reduced type duplication between root orchestrator and phase packages
  - Added comprehensive test coverage for all workflow coordinators (90%+ coverage)
  - Created workflow adapters for clean integration with the main orchestrator

- **Dead Code Removal** - Comprehensive cleanup of unused code across the codebase:
  - Removed three unused ultraplan subpackages (`consolidator`, `decomposition`, `executor`)
  - Removed `internal/instance/facade.go` (unused Facade type and its methods)
  - Removed `internal/instance/capture/output.go` (unused TmuxCapture implementation)
  - Removed `internal/instance/process/` package (4 files, never imported)
  - Removed `internal/logging/aggregate.go` (unused log aggregation utilities)
  - Removed unused `LoadPlanFromFile` function from `internal/cmd/planning/plan.go`
  - Updated related doc.go files to remove references to deleted code
  - This reduces codebase complexity and maintenance burden without affecting functionality.

### Fixed

- **Terminal Pane Shell Prompt Not Visible** - Fixed the terminal pane not displaying the shell prompt. The root cause was that `capture-pane` returns output ending with a newline, which caused `strings.Split` to create an extra empty element. When taking the "last N lines", the prompt (at position 0) was dropped while empty lines were kept. The fix trims trailing newlines before splitting and reorders the trimming/truncation logic to prioritize actual content.

- **Terminal Keybindings Respect Config** - Fixed backtick (`) and `T` keys toggling the terminal pane even when `experimental.terminal_support` is disabled in the config. The terminal keybindings now correctly check the config setting before activating.

- **Adversarial Review Score Threshold Enforcement** - Fixed a bug where users who set a minimum passing score higher than the default (e.g., 9 or 10) would have the adversarial loop stop prematurely. The issue occurred because approval notifications were sent before score enforcement was applied, causing callbacks to receive the unenforced state. The enforcement check is now performed before any notifications, ensuring the loop correctly continues when the reviewer's score is below the configured threshold.

## [0.10.0] - 2026-01-17

This release introduces **Adversarial Review Mode** and **Ralph Wiggum Loop** - two powerful new workflows for iterative development and code review. It also graduates TripleShot from experimental to a permanent feature.

### Fixed

- **Terminal Pane Color Support** - Fixed the terminal pane not displaying colors properly by setting `TERM=xterm-256color` in the environment and configuring `default-terminal` per-session before tmux session creation. The approach now aligns with the instance package for consistency.

- **Adversarial Mode Header Alignment** - Fixed the adversarial mode header displaying the status text outside the styled header border, causing visual misalignment.

### Added

- **Adversarial Review Mode** - New `claudio adversarial` command that creates an iterative feedback loop between an Implementer and a critical Reviewer:
  - Implementer works on the task and submits work via increment files
  - Reviewer critically examines the work and provides detailed feedback
  - Loop continues until Reviewer approves the implementation or max iterations reached
  - Configurable max iterations with `--max-iterations` flag (default: 10)
  - Configurable minimum passing score with `--min-passing-score` flag (default: 8)
  - Both settings can be configured in `config.yaml` under `adversarial:` section or via TUI config editor
  - Useful for complex implementations that benefit from rigorous code review
  - **NEW**: Now accessible from the TUI via `:adversarial` or `:adv` command
  - Supports multiple concurrent adversarial sessions, similar to TripleShot mode

- **Ralph Wiggum Loop** - New `:ralph` command that creates an iterative development loop where Claude autonomously re-executes the same prompt until a completion promise is found in the output:
  - Uses "completion promise" detection - when the specified phrase appears in Claude's output, the loop terminates successfully
  - Configurable safety limit with `--max-iterations` flag (default: 50) to prevent runaway loops
  - Configurable completion phrase with `--completion-promise` flag
  - Cancel active loops with `:cancel-ralph` command
  - Each iteration runs in the same worktree, allowing work to persist and accumulate
  - Supports multiple concurrent ralph sessions with independent coordinators
  - Named after the Simpsons character Ralph Wiggum, referencing the "I'm helping!" meme for iterative self-improvement

- **Task Chaining by Sidebar Number** - The `:chain`, `:dep`, and `:depends` commands now accept an optional sidebar number argument to specify which instance to chain from (e.g., `:chain 2` or `:chain #2`). This is more user-friendly than using instance IDs, which aren't prominently displayed.

### Changed

- **Coordinator Thin Facade Refactoring** - The Coordinator has been refactored into a thin facade that delegates phase execution to specialized phase orchestrators (PlanningOrchestrator, ExecutionOrchestrator, SynthesisOrchestrator). This improves separation of concerns and testability by moving phase-specific logic into dedicated orchestrators, with the Coordinator now serving as the public API that coordinates between phases.

- **Scroll-to-Top Key Binding** - Changed scroll-to-top from `g` to `0` (zero) to resolve overload conflict with group commands (`gc`, `gn`, `gp`, etc.). The `g` key now exclusively enters group command mode when in grouped sidebar view.

- **Add Task Dialog Titles** - The "Add New Instance" dialog now displays context-aware titles based on the type of task being created: "Triple-Shot" for triple-shot mode, "Adversarial Review" for adversarial mode, "Chain Task" for dependent tasks, and "New Task" for standard tasks. Each mode also shows a descriptive subtitle explaining its purpose.

- **Unified Group Types** - Refactored the `group`, `orchestrator`, and `session` packages to use a shared `grouptypes.InstanceGroup` type, eliminating type duplication and ~80 lines of conversion code. This also prevents data loss (SessionType and Objective fields) when using the group manager.

- **TimeoutType Consolidation** - Made `state.TimeoutType` and `instance.TimeoutType` type aliases for `detect.TimeoutType`, eliminating duplicate type definitions and removing unnecessary type conversion code in the orchestrator.

- **TripleShot Mode Graduated** - TripleShot is now a permanent feature. The `experimental.triple_shot` configuration option has been removed; the `:tripleshot` command is always available.

- **Documentation Overhaul** - Comprehensive update to all documentation to reflect current features and capabilities:
  - Added task chaining (`--depends-on`) documentation to README, CLI reference, and new dedicated guide
  - Added planning commands (`plan`, `ultraplan`, `tripleshot`) to CLI reference
  - Added command mode documentation to TUI navigation guide with all `:` commands
  - Added logging configuration section to configuration guides
  - Added sparse checkout documentation for monorepo support
  - Added instance timeout settings (`activity_timeout_minutes`, `completion_timeout_minutes`)
  - Added UltraPlan and experimental features sections to configuration guide
  - Created new Task Chaining guide (`docs/guide/task-chaining.md`)
  - Updated keyboard shortcuts reference with command mode
  - Updated feature lists in README and docs homepage to include all planning modes

## [0.9.2] - 2026-01-16

This patch release focuses on **Critical Stability Fix** - resolving a bug that could cause the TUI to display frozen/stale output when tmux is under heavy load.

### Fixed

- **Frozen Output During Tmux Timeout** - Fixed a critical bug where tmux output would freeze completely when the tmux status query timed out. Previously, when `getSessionStatus()` took longer than 2 seconds (due to heavy system load or many tmux sessions), the capture loop would skip output capture entirely, causing the TUI to display stale content indefinitely. Now output capture continues even when the status query fails, using a full pane capture as fallback.

- **Ring Buffer Race Condition** - Fixed a race condition between `Reset()` and `Write()` operations in the ring buffer that could cause brief display flickering. Added new atomic `ReplaceWith()` method that performs both operations under a single lock acquisition, preventing concurrent `GetOutput()` calls from seeing an empty buffer.

## [0.9.1] - 2026-01-16

This patch release focuses on **Bug Fixes & UI Polish** - improving completion file detection reliability, fixing sidebar width configuration consistency, and enhancing task status readability in the sidebar.

### Changed

- **Status Symbol Alignment** - Task status indicators (WORK, WAIT, DONE, etc.) now display on a new line underneath task names in both the grouped sidebar and tripleshot views, improving vertical alignment and readability (#509)

### Fixed

- **Completion File Detection in Subdirectories** - Fixed a bug where tasks would be incorrectly marked as failed when Claude wrote the completion file in a subdirectory. The task verifier now uses recursive search (matching the completion detector behavior) to find completion files regardless of where Claude wrote them (#507)

- **Tmux Panel Width Configuration** - Fixed tmux panel resizing to use the user-configured sidebar width (`tui.sidebar_width`) instead of hardcoded defaults. Previously, the UI layout used the configured width but tmux panels used the default, causing inconsistent sizing (#508)

## [0.9.0] - 2026-01-16

This release brings **Sidebar Customization & Group Dismiss** - allowing users to configure the sidebar width for their workflow preferences and quickly dismiss entire instance groups with a single shortcut.

### Added

- **Configurable Sidebar Width** - The sidebar width is now user-configurable through the interactive configuration UI (`tui.sidebar_width`). The default width has been increased from 30 to 36 columns to provide more space for instance names and metrics. Users can configure widths between 20-60 columns.
- **Group Dismiss Shortcut** - Added `gq` keyboard shortcut to dismiss (remove) all instances in the currently selected group. This applies the `:D` (remove) action to every instance in the group, providing a quick way to clean up an entire group at once.

### Changed

- **CLI Command Package Reorganization** - Reorganized the flat `internal/cmd/` package (17 files) into domain-specific subpackages for better maintainability and easier onboarding. Commands are now grouped into: `session/` (start, stop, sessions, cleanup), `planning/` (plan, ultraplan, tripleshot), `instance/` (add, remove, status, stats), `observability/` (logs, harvest), `project/` (init, pr), and `config/` (config management). Each subpackage has a `Register()` function that wires its commands to the root command. This change is purely organizational and has no impact on CLI behavior.

## [0.8.2] - 2026-01-16

### Fixed

- **Sidebar Navigation Hint** - Updated the sidebar keyboard hint from `j/k scroll` to `h/l nav` to accurately reflect the actual key bindings for navigating between instances.

## [0.8.1] - 2026-01-16

### Fixed

- **Triple-Shot Judge Not Triggering** - Fixed a critical bug where the triple-shot judge would not start when the last attempt to complete had a parsing error. Previously, if the completion file for an attempt failed to parse (e.g., due to schema variations like `notes` being an array instead of a string), the error handling path would return early without checking if all attempts were complete. Now the judge-triggering check runs regardless of parsing errors, ensuring the judge starts as long as at least 2 attempts succeeded.

- **Flexible Notes Field in Completion Files** - Changed the `Notes` field in `TripleShotCompletionFile` to use `FlexibleString` type (reusing the existing type from ultraplan.go), which accepts both string and array-of-strings JSON values. This prevents JSON parsing failures when Claude instances write completion files with `notes` as an array, which was causing attempts to be incorrectly marked as failed.

## [0.8.0] - 2026-01-16

This release focuses on **Multi-Session Support & Deprecation Cleanup** - enabling multiple concurrent `:plan` and `:multiplan` sessions, adding sparse checkout for monorepo support, improved mode visibility, and removing deprecated APIs to streamline the codebase.

### Changed

- **Instance Header Simplified** - Removed the truncated task/prompt text from the instance detail view header. The header now shows only the branch name, reducing visual noise and giving more space to the output area.

### Added

- **Phase Orchestrator Integration Tests** - Added comprehensive integration tests for the phase orchestrator (`internal/orchestrator/phase/integration_test.go`). These tests exercise full phase lifecycles and transitions between phases (Planning → Execution → Consolidation → Synthesis), which are critical paths that can fail in complex multi-task scenarios. Coverage includes: planning-to-execution transitions, execution-to-consolidation flows with multi-group support, execution-to-synthesis transitions, partial group failure handling, synthesis lifecycle and revision cycle state management, concurrent phase operations, and phase callback consistency. Test infrastructure includes `IntegrationTestCoordinator` for simulating coordinator behavior across phase boundaries.

### Removed

- **Deprecated TripleShot Single Coordinator** - Removed the deprecated `Coordinator` field from `TripleShotState` and `TripleShot` field from `Session`. All code now uses the `Coordinators` map and `TripleShots` slice for multiple concurrent tripleshot support.
- **Deprecated Plan/Issue Wrapper Functions** - Removed deprecated wrapper functions (`CreateIssue`, `UpdateIssueBody`, `GetIssueNodeID`, `AddSubIssue`) and `IssueOptions` type from `internal/plan/issue.go`. Use `tracker.GitHubTracker` methods directly.
- **Deprecated PR Wrapper Functions** - Removed deprecated `CreatePR` and `CreatePRDraft` wrapper functions from `internal/pr/pr.go`. Use `pr.Create(PROptions{...})` directly.
- **Deprecated Instance Manager Constructors** - Removed deprecated constructors `NewManager`, `NewManagerWithConfig`, `NewManagerWithSession`, and method `SetStateMonitor` from `internal/instance/manager.go`. Use `NewManagerWithDeps(ManagerOptions{...})` for explicit dependency injection.
- **Deprecated Terminal Constants** - Removed deprecated type alias `TerminalDirMode` and constants `TerminalDirInvocation`, `TerminalDirWorktree`, `DefaultTerminalHeight`, `MinTerminalHeight`, `MaxTerminalHeightRatio` from `internal/tui/model.go`. Use the `terminal` package exports directly.
- **Deprecated GetTripleShotCoordinator** - Removed deprecated `GetTripleShotCoordinator()` method from `command.Dependencies` interface and `Model`. Use `GetTripleShotCoordinators()` which returns all active coordinators.
- **Deprecated getHistorySize Function** - Removed the deprecated `getHistorySize` method from instance Manager. Use `getSessionStatus` for batched queries with reduced subprocess overhead.
- **Unused Error Sentinels** - Removed unused error sentinels from `internal/orchestrator/prompt_adapter.go`: `ErrNilUltraPlanSession`, `ErrTaskNotFoundInPlan`, and `ErrNilGroupTracker`. These were leftover from the removed `PromptAdapter` struct.
- **Unused Inline Plan Functions** - Removed unused integration stub functions from `internal/tui/inlineplan.go`: `handleInlinePlanCompletion`, `getPlanForInlineEditor`, `handleInlinePlanTaskDelete`, `handleInlinePlanTaskAdd`, `handleInlinePlanTaskReorder`, and duplicate `findInstanceIndex`. These were marked as pending integration but never completed. Code now uses `findInstanceIndexByID` for consistent instance lookup behavior.
- **Deprecated SessionExists Method** - Removed the deprecated `SessionExists` method from `internal/instance/lifecycle/manager.go`. Use `SessionExistsWithSocket` for socket-specific session checks.
- **Legacy RenderParentIssueBody Function** - Removed the unused `RenderParentIssueBody` function and `GroupedTasks` field from `internal/plan/template.go`. Production code exclusively uses `RenderParentIssueBodyHierarchical` which supports hierarchical children. The template was simplified to remove the legacy fallback path for grouped tasks.
- **Unused Planning Orchestrator Method** - Removed unused `setInstanceID` method from `PlanningOrchestrator` in `internal/orchestrator/phase/planning.go`.

### Added

- **Multiple Concurrent Plan/MultiPlan Sessions** - Extended the sidebar to support multiple concurrent `:plan` and `:multiplan` sessions, similar to existing tripleshot support. Refactored `InlinePlanState` from a single-session struct to a multi-session container with a `Sessions` map keyed by group ID. Added new `InlinePlanSession` type to hold per-session state. Added `view.PlanState` and `view.MultiPlanState` types with comprehensive sidebar rendering functions for displaying multiple sessions with phase progress, planner status, and execution tracking. The sidebar now shows all active plan/multiplan sessions with their respective statuses.

- **iOS Development Documentation** - Added comprehensive tutorial for using Claudio with iOS/Xcode projects. Covers DerivedData management strategies, Swift Package Manager caching, parallel testing with simulators, handling `project.pbxproj` conflicts, and performance optimization. Includes iOS-specific configuration examples and troubleshooting guidance.
- **Force Quit Command** - Added `:q!` (and `:quit!`) command to force quit Claudio, stopping all running instances, cleaning up all worktrees, and exiting immediately. This provides a quick way to completely exit and clean up when you want to abandon all work in progress.
- **Footer Mode Badges** - Added prominent mode badges to the footer help bar that clearly indicate the current mode at all times. Each mode (NORMAL, INPUT, TERMINAL, COMMAND, SEARCH, FILTER, DIFF, ULTRAPLAN, PLAN EDIT, CONFIG) displays a styled badge at the start of the help bar. High-priority modes like INPUT and TERMINAL use contrasting background colors (amber and green respectively) while NORMAL mode uses a subtle surface color. This makes it immediately obvious what mode the user is in, regardless of where they look on the screen.
- **Sparse Checkout for Worktrees** - Added configurable sparse checkout support for git worktrees, enabling partial repository clones for large monorepos. Users can specify which directories to include via `paths.sparse_checkout.directories` and `paths.sparse_checkout.always_include` in config. Supports git's cone mode (default, faster) or traditional gitignore-style patterns. Includes comprehensive validation for directory paths, duplicate detection, and path limits. This reduces disk usage and improves performance when working with monorepos like Notion's where iOS developers don't need Android code and vice versa.
- **Header Mode Indicator** - Added a prominent mode indicator in the header bar that shows the current input mode (INPUT, TERMINAL, COMMAND, SEARCH, FILTER, NEW TASK). High-priority modes like INPUT and TERMINAL use contrasting colors (amber and green backgrounds respectively) to make it immediately clear when keyboard input is being forwarded to Claude or the terminal pane rather than controlling the TUI. This helps users avoid accidentally typing in the wrong mode.
- **Git Submodule Support** - Worktrees now automatically initialize git submodules on creation. Added `HasSubmodules()`, `GetSubmodules()`, `GetSubmodulePaths()`, `InitSubmodules()`, `GetSubmoduleStatus()`, and `IsSubmoduleDir()` to the worktree package. The `protocol.file.allow=always` flag is used for git 2.38+ compatibility with local submodule references.

- **Prompt Type Conversion Helpers** - Added interface-based type conversion helpers in `internal/orchestrator/prompt/convert.go` to enable decoupled conversion from orchestrator types to prompt types. Includes `PlannedTaskLike`, `PlanSpecLike`, and `GroupConsolidationLike` interfaces with corresponding conversion functions (`ConvertPlannedTaskToTaskInfo`, `ConvertPlanSpecToPlanInfo`, `ConvertPlanSpecsToCandidatePlans`, `ConvertGroupConsolidationToGroupContext`). This enables extracting prompt-building logic from coordinator.go into the prompt package without creating circular imports.
- **Strategy Names Context Support** - Added `StrategyNames` field to `prompt.Context` to provide fallback strategy names when `CandidatePlanInfo.Strategy` is empty. This allows the coordinator to pass strategy names via Context rather than requiring the prompt package to import from ultraplan.go (avoiding circular imports). Also added `FormatCompactPlansWithContext` method to `PlanningBuilder` for compact plan formatting with strategy name fallback support.

### Fixed

- **TripleShot Completion File Prompt** - Improved the implementer prompt to be more explicit about the mandatory completion file requirement. The prompt now clearly states that the orchestration system is waiting for the file, that work is not considered complete without it, and includes a final reminder to write the file as the absolute last action. This addresses cases where implementers would finish their work but fail to write the completion file.
- **Group Collapse Toggle (gc) Not Working** - Fixed `gc` command having no effect when a group was selected via `gn`/`gp`. The collapse state was being toggled twice (once in the handler, once in the dispatcher), canceling itself out. Removed the duplicate toggle from the dispatcher.
- **Frozen Output After Stale Timeout** - Fixed a critical bug where tmux output would freeze completely when an instance triggered stale timeout detection. Previously, when the capture loop detected repeated identical output (stale detection), it would skip all processing including output capture, causing the TUI to display stale content indefinitely. Now output capture continues even after timeouts, ensuring the display stays up-to-date while only skipping redundant state detection.
- **Group Navigation Skips Hidden Subgroups** - Fixed `gn`/`gp` navigation and `gc` collapse toggle to respect parent collapse state. Previously, when a parent group was collapsed, keyboard navigation would still traverse hidden subgroups, causing `gc` to appear non-functional (it would toggle a hidden subgroup). Navigation now correctly skips subgroups whose parent is collapsed, ensuring users can only navigate to visible groups.
- **Footer Input Mode Badge in Special Modes** - Fixed the INPUT and TERMINAL mode badges not appearing in the footer when in triple-shot mode or ultra-plan mode. Previously, entering input mode (`i`) while in these special modes would continue showing the NORMAL/ULTRAPLAN badge instead of the INPUT badge. Now the footer correctly shows the INPUT badge with exit instructions when input forwarding is active, regardless of the underlying session mode.
- **Git Submodule File Traversal** - Fixed errors and warnings when working with repositories containing git submodules. File traversal in the conflict detector, completion file verifier, and code analyzer now correctly skips submodule directories to prevent permission errors, symlink errors, and duplicate events when submodules are uninitialized or partially initialized.

### Changed

- **Prompt Building Refactor** - Extracted prompt-building logic from coordinator.go into the orchestrator/prompt package, improving code organization and testability. Updated `buildPlanManagerPrompt` and `buildPlanComparisonSection` methods to use `PlanningBuilder` with the new conversion helpers. Added `FormatDetailedPlans` and `BuildCompactPlanManagerPrompt` methods to `PlanningBuilder` for flexible plan formatting. Exported `PlanManagerPromptTemplate` in the prompt package.
- **Complete Coordinator Prompt Builder Migration** - Migrated remaining coordinator prompt-building methods (`buildRevisionPrompt`, `buildSynthesisPrompt`, `buildConsolidationPrompt`) to use the extracted prompt builders (`RevisionBuilder`, `SynthesisBuilder`, `ConsolidationBuilder`). Removed duplicate prompt templates from `ultraplan.go` that are now defined in the `orchestrator/prompt` package. Removed unused `PromptAdapter` struct and its methods (keeping only the standalone type conversion functions that are actively used). This completes the separation of prompt-building concerns from the Coordinator.
- **Ultraplan View Package Granularity** - Refactored the 1,903-line `ultraplan.go` view file into focused, testable components in a new `internal/tui/view/ultraplan/` subpackage. Split rendering logic into separate files by concern: `header.go` (header with phase and progress), `tasks.go` (task list rendering with status icons and wrapping), `status.go` (phase indicators and progress bars), `consolidation.go` (consolidation phase sidebar), `help.go` (context-sensitive help bar), `sidebar.go` (unified sidebar composition), `planview.go` (detailed plan view), and `inline.go` (inline content for collapsible groups). The original `ultraplan.go` now provides backward-compatible wrappers that delegate to the subpackage. This follows Bubbletea best practices where views are composed from smaller, focused rendering functions.

### Fixed

- **Sidebar Navigation Order** - Fixed h/l (or tab/shift-tab) navigation in the sidebar navigating by instance creation order instead of display order. When instances are grouped (e.g., in ultraplan mode), the sidebar displays instances in a specific hierarchy (ungrouped first, then by group). Navigation now correctly follows the visual display order rather than the internal creation order, preventing user confusion when pressing tab/l to move to the "next" instance.
- **Sidebar Navigation Hints** - Fixed incorrect keyboard hints in the grouped sidebar view. The hints incorrectly showed `[j/k] nav [J/K] groups [Space] toggle` when the actual bindings are `[j/k] scroll [gn/gp] groups [gc] toggle`. The `j/k` keys scroll the output panel (not sidebar navigation), and group navigation uses vim-style g-prefix commands (`gn`/`gp` for next/previous group, `gc` for toggle collapse).

## [0.7.1] - 2026-01-15

This release focuses on **Phase Orchestration** - extracting the monolithic Coordinator into four phase-specific orchestrators and implementing comprehensive synthesis/revision orchestration logic.

### Fixed

- **Completion File Detection in Subdirectories** - Fixed completion files not being detected when Claude instances change into a subdirectory during task execution. When Claude runs `cd project/` and then writes `.claudio-task-complete.json`, the file ends up in the subdirectory instead of the worktree root. The verifier now uses a recursive search with depth limiting (max 5 levels) and directory skipping (node_modules, vendor, .git, Pods, etc.) to find completion files in subdirectories. Also updated all completion protocol prompts to emphasize that the file must be written at the worktree root.
- **Ultraplan Nested Plan JSON Parsing** - Fixed `ParsePlanFromFile` failing with "plan contains no tasks" when the plan JSON has a nested `{"plan": {...}}` wrapper structure. Claude generating plans via `PlanManagerPromptTemplate` sometimes wraps the plan in a `plan` object. The parser now supports both formats: root-level (`{"summary": "...", "tasks": [...]}`) and nested (`{"plan": {"summary": "...", "tasks": [...]}}`). Also added support for alternative field names (`depends` as alias for `depends_on`, `complexity` as alias for `est_complexity`) that Claude may generate. Additionally, updated `PlanManagerPromptTemplate` to include the explicit JSON schema with a note to NOT wrap in a "plan" object, preventing the issue from occurring in future plan generations.

### Changed

- **Phase Orchestrator Extraction** - Extracted the monolithic Coordinator (~3,800 lines) into four phase-specific orchestrators: `PlanningOrchestrator`, `ExecutionOrchestrator`, `SynthesisOrchestrator`, and `ConsolidationOrchestrator`. Each orchestrator owns one phase of ultra-plan execution with its own state management, monitoring, and restart capability. The main Coordinator is now a thin dispatcher (~500 lines) that instantiates orchestrators and delegates phase operations. This eliminates the god-object anti-pattern and enables independent testing of each phase.
- **ExecutionOrchestrator Task Monitoring** - Added task monitoring and verification methods (`checkForTaskCompletionFile`, `pollTaskCompletions`, `verifyTaskWork`) directly to `ExecutionOrchestrator`, reducing dependency on the Coordinator. Implemented two-tier completion detection: primary detection via sentinel file (`.claudio-task-complete.json`) and fallback status-based detection for legacy behavior. Added `TaskVerifierInterface` to enable dependency injection and testing of verification logic.
- **ExecutionOrchestrator Task Completion** - Implemented comprehensive task completion handling in `ExecutionOrchestrator` with duplicate detection, retry handling, group completion checking, partial failure detection, and synthesis phase transition. Added `GroupTrackerInterface` for abstracted group tracking operations. The `handleTaskCompletion` method now tracks processed tasks to prevent duplicate processing (race between monitor goroutine and polling), handles retry requests by clearing task-to-instance mapping, records commit counts for successful tasks, checks group completion via `GroupTrackerInterface`, and triggers consolidation or handles partial failures. The `finishExecution` method determines the appropriate next phase based on task outcomes (fail, skip synthesis, or start synthesis).
- **ExecutionOrchestrator Retry/Recovery** - Extracted retry, recovery, and partial failure handling methods into `ExecutionOrchestrator`: `HasPartialGroupFailure()` checks if a group has mixed success/failure, `GetGroupDecision()` and `IsAwaitingDecision()` query pending decision state, `ResumeWithPartialWork()` continues execution with only successful tasks from a partial failure, `RetryFailedTasks()` resets failed tasks for re-execution, and `RetriggerGroup()` resets and restarts execution from a specific group. Added supporting interfaces (`RetryManagerInterface`, `RetryRecoverySessionInterface`) and types (`RetryTaskState`, `RetryRecoveryContext`) for dependency injection and testing.
- **ConsolidationOrchestrator Per-Group Consolidation** - Added per-group consolidation methods to `ConsolidationOrchestrator`: `GatherTaskCompletionContextForGroup`, `GetTaskBranchesForGroup`, `BuildGroupConsolidatorPrompt`, `StartGroupConsolidatorSession`, `MonitorGroupConsolidator`, and `ConsolidateGroupWithVerification`. These methods handle synchronous consolidation between execution groups, enabling a Claude instance to be spawned specifically to consolidate one group's task branches. Added supporting types (`AggregatedTaskContext`, `GroupConsolidationCompletionFile`, `ConsolidationTaskWorktreeInfo`) and interfaces (`GroupConsolidationSessionInterface`, `GroupConsolidationOrchestratorInterface`, `TaskCompletionFileParser`) for Coordinator integration.
- **Coordinator Phase Delegation** - Updated Coordinator public methods to delegate to the appropriate phase orchestrators, maintaining synchronization between the Coordinator's session state and orchestrator internal state. Updated `RunPlanning`, `RunMultiPassPlanning`, `RunPlanManager`, `StartExecution`, `RunSynthesis`, `StartRevision`, `TriggerConsolidation`, `StartConsolidation`, `ResumeConsolidation`, `ResumeWithPartialWork`, `RetryFailedTasks`, and `RetriggerGroup` to update phase orchestrator state via their public getters and state management methods.

### Added

- **Coordinator Phase Adapters** - Added adapter implementations to bridge the Coordinator's state to the `phase.PhaseContext` interfaces. Includes `coordinatorManagerAdapter`, `coordinatorOrchestratorAdapter`, `coordinatorSessionAdapter`, and `coordinatorCallbacksAdapter` for phase orchestrator dependency injection. Added helper methods to Coordinator (`BuildPhaseContext`, `GetBaseSession`, `GetOrchestrator`, `GetLogger`, `GetContext`, `EmitEvent`, `GetVerifier`, running task management) to support phase orchestrator integration.
- **Synthesis Orchestrator Execution** - Implemented full synthesis execution logic in `SynthesisOrchestrator.Execute()` including instance creation, prompt building with task summaries and commit counts, completion file parsing, and status monitoring. The synthesis phase now creates a Claude instance to review completed work, monitors for a sentinel completion file (`.claudio-synthesis-complete.json`), parses revision issues, and determines whether revisions are needed or consolidation can proceed. Also implemented synthesis completion and approval handling with `TriggerConsolidation()`, `OnSynthesisApproved()`, `CaptureTaskWorktreeInfo()`, and `ProceedToConsolidationOrComplete()` methods to manage the awaiting approval state and transition to revision or consolidation phases.
- **Synthesis Orchestrator Revision Phase** - Implemented the revision sub-phase in `SynthesisOrchestrator` with full lifecycle support: `StartRevision()` initializes/updates revision state and spawns revision tasks for each affected task, `startRevisionTask()` creates new instances in existing worktrees (reusing the original task's worktree), `buildRevisionPrompt()` generates targeted prompts with specific issues to fix, `monitorRevisionTasks()` and `handleRevisionTaskCompletion()` track parallel revision task progress, and `onRevisionComplete()` triggers re-synthesis after all revisions complete. Added `RevisionState` type to track revision rounds, tasks to revise, and completion status. Added `RevisionOrchestratorInterface` and `RevisionSessionInterface` for dependency injection. The revision phase detects completion via sentinel file (`.claudio-revision-complete.json`) with fallback to status-based detection.

## [0.7.0] - 2026-01-14

This release focuses on **Stability & Architecture** - a comprehensive effort to stabilize UltraPlan, graduate Plan Mode from experimental, and restructure the TUI codebase for maintainability.

### Added

- **PromptAdapter Infrastructure** - Added `PromptAdapter` struct and converter methods to bridge orchestrator types (`PlanSpec`, `PlannedTask`, `RevisionState`, `SynthesisCompletionFile`) to `prompt.Context` types, enabling the existing prompt.Builder infrastructure to be used for prompt generation. Includes `BuildPlanningContext`, `BuildSynthesisContext`, `BuildTaskContext`, `BuildRevisionContext`, `BuildConsolidationContext`, and `BuildPlanSelectionContext` methods for building phase-specific contexts.
- **Changelog CI Check** - PRs now require a CHANGELOG.md entry. Add the `skip-changelog` label to bypass for trivial changes (test-only, internal refactors, docs-only, dependency updates).
- **Multi-Pass Plan Mode** (Experimental) - New `:multiplan` (or `:mp`) command launches competitive multi-pass planning with 3 parallel planners using different strategies (maximize-parallelism, minimize-complexity, balanced-approach) plus a plan manager/assessor that evaluates and merges the best plan. This provides the same competitive planning approach as `:ultraplan --multi-pass` but within the simpler inline plan workflow.
- **Ultraplan Group Expand/Collapse** - Execution groups in ultraplan sessions now support intelligent expand/collapse behavior. By default, only the currently active group is expanded; past and future groups are collapsed. When execution moves to a new group, it auto-expands and the previous group auto-collapses (unless you're viewing a task from that group). Groups show ▼/▶ indicators and can be manually toggled via group navigation mode (press `g` to enter, then `j/k` to navigate and `Enter`/`Space` to toggle). Collapsed groups display a summary showing completion progress like `[✓ 3/5]`. Tasks in collapsed groups are not navigable until the group is expanded.

### Changed

- **Plan Mode Graduated from Experimental** - The `:plan` command is now always available without any configuration. The `experimental.inline_plan` setting now only controls the `:multiplan` command, which remains experimental.
- **Mandatory Changelog Entries** - AGENTS.md now requires a changelog entry for every pull request with no exceptions. Previously allowed skipping changelog for internal refactors, test-only changes, documentation-only changes, and dependency updates.
- **Instance Header Simplified** - Removed redundant status badge from instance detail header since task state is already displayed in the sidebar with 4-character abbreviations (WAIT, WORK, DONE, etc.)
- **TUI Architecture Refactored** - Major restructuring of the TUI codebase: extracted `app.go` into focused packages (`msg/`, `filter/`, `update/`, `search/`, `view/`, `input/`), reducing it from ~3700 to 1369 lines (63% reduction). Added 14 new test files with comprehensive coverage. This creates a cleaner foundation for future TUI enhancements. (#443)
- **Ultraplan Initialization Consolidated** - Extracted duplicate ultraplan initialization code from CLI (`cmd/ultraplan.go`) and TUI (`inlineplan.go`) into a shared `internal/ultraplan` package. New `Init()` and `InitWithPlan()` factory functions provide a single cohesive initialization path for all ultraplan entry points. Also consolidated `truncateString` implementations into `internal/util/strings.go` with proper Unicode and ANSI escape code handling.

### Fixed

- **Ultraplan Task Selection Highlighting** - Fixed multiple tasks in an ultraplan group being incorrectly highlighted as selected simultaneously. When viewing a group's tasks, all completed tasks would show the selection highlight because a substring search (`strings.Contains`) was matching task IDs against consolidator prompts (which contain all task IDs from the group). The fix uses only the authoritative `TaskToInstance` map for determining which instance is associated with a task.
- **Group Auto-Expand on Navigation** - Fixed being able to navigate to instances hidden inside collapsed groups without the group expanding. When navigating to an instance in a collapsed group (via Tab/Shift+Tab), the group now auto-expands to reveal the selected instance. When navigating away from an auto-expanded group, it auto-collapses (unless it was manually expanded). This provides a "peek" behavior where groups temporarily expand just to show the navigated instance.
- **Ultraplan Instances Appearing as Ungrouped** - Fixed ultraplan instances (planning coordinators, execution tasks) appearing both in the ultraplan phase view and as separate "ungrouped" items at the top of the sidebar. The issue was in `handleUltraPlanObjectiveSubmit()` where `RunPlanning()` was called before the ultraplan group was created, causing instances to never be added to the group. Now the group is created before planning starts, and on failure the group is cleaned up to avoid orphaned empty groups.
- **Zero-Commit Tasks Incorrectly Marked as Failed** - Fixed ultraplan tasks that complete successfully with zero commits (e.g., verification tasks that find work already done) being incorrectly treated as failures during group completion. The group tracker and coordinator now check for the presence of a verified commit count entry (even if 0) rather than requiring `count > 0`. This prevents legitimate completions from triggering unnecessary "partial failure" prompts.
- **No-Code Task Support in Ultraplan** - Fixed ultraplan verification tasks (and other non-code tasks) failing with "no commits after 3 attempts". Added `no_code` field to `PlannedTask` schema allowing planning agents to mark tasks that don't require code changes. Additionally, tasks that write a completion file with `status: "complete"` now succeed even without commits, providing a runtime override for verification/testing tasks.
- **Ultraplan Instance Grouping in Sidebar** - Fixed planning instances appearing outside their ultraplan group in the TUI sidebar. Also fixed Plan Selector not being visible during the SELECTING PLAN phase for multi-pass planning. The coordinator now uses `session.GroupID` for reliable group lookup, and the inline content renderer properly displays multi-pass planning coordinators and the Plan Selector.
- **TUI Freeze in Ultraplan Sessions** - Fixed a critical race condition that caused the TUI to freeze completely (input not displaying, output stopped) when starting ultraplan or tripleshot sessions. The issue was unsynchronized concurrent access to `session.Groups` - the TUI render loop was iterating over the slice while other goroutines were appending to it via `AddGroup()`. Added `sync.RWMutex` protection to all Groups operations and thread-safe accessor methods (`GetGroups()`, `HasGroups()`, `GroupCount()`, `SetGroups()`).
- **Ultraplan Sidebar Coexistence** - Fixed ultraplan mode taking over the entire TUI sidebar, preventing navigation to standard instances. Ultraplans now render as collapsible groups within the standard grouped sidebar, allowing standard instances to run alongside ultraplans with proper navigation support.
- **Ultraplan Sidebar First Line Highlighting** - Fixed selected task highlighting not being applied to the first line of wrapped task names in ultraplan sidebar. When a task title wrapped to multiple lines, only continuation lines were highlighted because pre-applying `statusStyle.Render(icon)` embedded ANSI reset codes that broke the background color.
- **Inline Ultraplan Config File Settings** - Fixed inline ultraplan (`:ultraplan` command) not respecting config file settings like `multi_pass`, `max_parallel`, and other ultraplan configuration options. The inline ultraplan now reads from the config file just like the CLI `claudio ultraplan` command does. Additionally, the in-app help now documents the `--multi-pass` and `--plan <file>` flags, and a warning is logged when config file loading fails (instead of silently using defaults).
- **Inline Ultraplan Group Linkage** - Fixed inline ultraplan (`:ultraplan` command) not properly linking the ultraplan session to its group. The `ultraSession.GroupID` was not being set when creating groups, which could cause coexistence issues with standard instances. Now all three inline ultraplan creation paths (from file, immediate objective, and interactive objective) correctly set the GroupID.
- **Inline Ultraplan Consolidation Failure** - Fixed `:ultraplan --plan <file>` failing with "no task branches with verified commits found" after Group 1 completed. The inline ultraplan config was missing `RequireVerifiedCommits: true`, causing commit counts to never be recorded. Now uses `DefaultUltraPlanConfig()` to ensure proper defaults.
- **CLI-Started Ultraplan/Tripleshot Grouping** - Fixed `claudio ultraplan` and `claudio tripleshot` commands not displaying as grouped entries in the TUI sidebar. CLI-started sessions now create instance groups and enable grouped sidebar mode, matching the behavior of inline commands (`:ultraplan`, `:tripleshot`).
- **Ultraplan File Path Tilde Expansion** - Fixed `:ultraplan --plan ~/path/to/file.yaml` failing because Go's `os.ReadFile()` doesn't expand shell shortcuts like `~`. Paths with `~/` prefix are now correctly expanded to the user's home directory.
- **Multiplan Evaluator Not Starting** - Fixed `:multiplan` command not triggering the evaluator/assessor instance. The issue was that plan completion was only detected when planner processes exited, not when they created their plan files. Added async plan file polling (similar to `:ultraplan`) to detect plan creation and properly trigger the evaluator once all 3 planners complete.
- **Semicolon Input** - Fixed semicolons not being sent to Claude instances when using the persistent tmux connection. Semicolons are now properly quoted in tmux control mode commands to prevent them from being interpreted as command separators.
- **Option+Arrow and Option+Backspace Keys** - Fixed Option+Arrow (Alt+Arrow) and Option+Backspace (Alt+Backspace) keys not working in Claude instances. Bubble Tea key names are now properly mapped to tmux key names (e.g., "left" → "Left", "backspace" → "BSpace").
- **Status Messages Auto-Dismiss** - Info and error messages in the TUI status bar now automatically clear after 5 seconds. Previously, messages like conflict watcher warnings would persist indefinitely until manually cleared or replaced.

## [0.6.1] - 2026-01-14

### Added

- **Searchable Branch Selector** - Branch selection when adding a new instance now supports real-time search filtering and scrolling. Type to filter branches, use arrow keys to navigate, Page Up/Down for faster scrolling, and scroll indicators show when more branches exist above or below the visible viewport.

### Fixed

- **Layout Calculation** - Fixed layout calculation that could cause UI elements to duplicate under certain terminal sizes
- **Sidebar Line-Wrapping** - Corrected sidebar width calculations for line-wrapping, ensuring long instance names wrap correctly
- **Active Instance Capture** - Resume active instance capture after tab adjustment, preventing output display from stopping when switching instances

## [0.6.0] - 2026-01-14

This release introduces **Intelligent Task Decomposition** - a comprehensive analysis system that makes UltraPlan significantly smarter about breaking down complex projects into well-ordered, risk-aware execution plans.

### Added

- **Enhanced Task Decomposition** - Comprehensive `decomposition` package provides intelligent task breakdown for UltraPlan with multiple analysis strategies:
  - **Code Structure Analysis**: Parses Go source files to build package dependency graphs with centrality calculations
  - **Risk-Based Prioritization**: Multi-factor risk scoring (complexity, file count, centrality, shared files, cross-package scope) with blocking scores for execution ordering
  - **Critical Path Analysis**: Identifies the longest dependency chain and marks critical path tasks for optimization focus
  - **Dependency Inference**: Automatically detects implicit dependencies from shared files, same-package modifications, and import relationships
  - **Parallelism Metrics**: Calculates parallelism scores, identifies bottleneck groups, and computes average group sizes
  - **File Conflict Detection**: Groups tasks that share files with severity ratings for merge conflict prevention
  - **Transitive Reduction**: Simplifies dependency graphs by removing redundant edges (Floyd-Warshall algorithm)
  - **Smart Split Suggestions**: Recommends how to break complex tasks by file-type, package, or risk-isolation
  - **Risk-Aware Planning Strategy**: New "risk-aware" planning strategy guides Claude to create safer execution plans
- **Progress Persistence & Session Recovery** - Claude instances now persist their session IDs, enabling automatic recovery when Claudio is interrupted. If Claudio exits unexpectedly while instances are running, reattaching to the session will automatically resume Claude conversations using `--resume`, picking up exactly where they left off. Sessions track clean shutdown state and can detect interruptions on restart.
- **Multiple Concurrent Tripleshots** - Users can now run multiple tripleshot sessions simultaneously. Each tripleshot operates independently with its own attempts, judge, and evaluation. The `:accept` command is context-aware, accepting the tripleshot whose instance is currently selected. Tripleshots appear as separate groups in the sidebar.
- **Plan Mode in Triple-Shot** - The `:plan` command can now be used while in triple-shot mode. Plan groups appear as separate sections in the sidebar below the tripleshot attempts and judge, allowing parallel planning workflows alongside tripleshot execution.
- **Inline Plan Mode** (Experimental) - New `:plan` command enables structured task planning directly within the TUI. Create task groups, define dependencies, and execute plans without leaving Claudio. Enable via `experimental.inline_plan` in config.
- **Inline UltraPlan Mode** (Experimental) - New `:ultraplan` command for parallel task execution with automatic coordination. Supports multi-pass planning (`--multi-pass`) and loading existing plans (`--plan <file>`). Enable via `experimental.inline_ultraplan` in config.
- **Grouped Instance View** (Experimental) - Visual grouping of instances in the sidebar with collapsible groups. Toggle with `:group show`. Enable via `experimental.grouped_instance_view` in config.
- **Group Keyboard Shortcuts** - Vim-style `g` prefix commands for group navigation: `gc` toggle collapse, `gC` collapse/expand all, `gn/gp` navigate groups, `gs` skip group, `gr` retry failed, `gf` force-start next group.
- **Group Management Commands** - New commands for organizing instances: `:group create`, `:group add`, `:group remove`, `:group move`, `:group order`, `:group delete`.
- **Group-aware PR Workflow** - Create PRs for task groups with `:pr --group` (stacked PRs), `:pr --group=all` (consolidated), or `:pr --group=single` (single group).
- **Session Resume for Consolidation** - Resume paused consolidation after manually resolving merge conflicts using the `r` key in UltraPlan mode. The system validates conflicts are resolved before continuing.

### Fixed

- **Triple-Shot Accept Command** - Implement missing `:accept` command that was referenced in UI message but never implemented. Users can now accept winning triple-shot solutions after evaluation completes.
- **Grouped Sidebar Shows All Instances** - Fixed bug where instances not belonging to a group (e.g., pre-existing instances) would disappear from the sidebar when a tripleshot or other grouped session was active. Ungrouped instances now appear at the top of the sidebar in grouped mode.
- **TUI Freeze from File I/O** - Fixed UI freeze that could occur during triple-shot and ultraplan modes when checking for completion files or plan files. All file I/O operations in the tick handler are now performed asynchronously, keeping the UI responsive.

### Changed

- **Terminal Support Now Experimental** - Embedded terminal pane commands (`:term`, `:t`, `:termdir`) are now gated behind `experimental.terminal_support` config flag, disabled by default. Enable via `:config` under Experimental or set `experimental.terminal_support: true` in config.yaml

## [0.5.1] - 2026-01-13

### Fixed

- **Tmux Session Isolation** - Use dedicated tmux socket (`-L claudio`) to isolate Claudio sessions from other tmux clients (like iTerm2's tmux integration), preventing crashes from tmux control-mode notification bugs in tmux 3.6a

## [0.5.0] - 2026-01-13

This release introduces **Triple-Shot Mode** - a powerful new execution strategy that runs three Claude instances in parallel on the same task, then uses a fourth "judge" instance to evaluate all solutions and pick the best approach.

### Added

- **Triple-Shot Mode** (Experimental) - New `claudio tripleshot` command runs three Claude instances in parallel on the same task, then uses a fourth "judge" instance to evaluate all solutions and determine the best approach. Supports `--auto-approve` flag and provides a specialized TUI showing attempt status, judge evaluation, and results
- **Configurable Worktree Directory** - Users can now configure where Claudio creates git worktrees via `paths.worktree_dir` in config. Supports absolute paths, relative paths, and `~` home directory expansion. Available in interactive config (`claudio config`) under the new "Paths" category.
- **Expanded Instance Names in Sidebar** - When intelligent naming is enabled, the selected instance in the sidebar now expands to show up to 50 characters of its display name (with wrapping if needed), making it easier to identify active tasks without truncation

### Fixed

- **Intelligent Naming Triggers Immediately** (Experimental) - Instance renaming now triggers immediately when an instance starts, using only the task description. Previously required waiting for 200+ bytes of Claude output which could fail silently.
- **Cleanup Command Respects Worktree Config** - The `claudio cleanup` command and stale resource warnings now correctly use the configured worktree directory instead of the hardcoded default
- **TUI Scrollback Stability** - Fixed output display flashing and scroll position jumping when using differential capture optimization. Visible-only captures no longer write to the output buffer, preventing the display from rapidly alternating between short (visible-only) and full (scrollback) content

### Performance

- **Keystroke Batching** - Input mode now coalesces rapid keystrokes into batches before sending to tmux, reducing command overhead by 5-10x for fast typists (250+ WPM)

## [0.4.0] - 2026-01-12

This release focuses on **tmux reliability** and **UX improvements**, with a major performance boost from persistent tmux connections and several quality-of-life enhancements for the TUI.

### Added
- **Branch Selection for New Instances** - When adding a new task, press `Tab` to select which branch the instance should be created from, defaulting to main/master for clean isolation
- **TUI Task Chaining** - Chain tasks via TUI using `:chain`, `:dep`, or `:depends` commands to add tasks that auto-start when the selected instance completes
- **Local Claude Config Copying** - `CLAUDE.local.md` is now automatically copied to worktrees for consistent local settings (#318)
- **Collapsible Task Groups** - UltraPlan sidebar now supports collapsible execution groups via `[g]` group navigation mode, with `[e]` expand all and `[c]` collapse all (#289)
- **Verbose Command Mode Help** - Command mode now shows descriptive help with command explanations instead of just single letters; configurable via `tui.verbose_command_help` setting (enabled by default)
- **Scrollable Help Panel** - The `:help` panel is now scrollable (j/k, Ctrl+D/U, g/G) with color-coded sections for better readability

### Performance

- **Persistent Tmux Connection** - Input mode now uses tmux control mode to maintain a persistent connection, eliminating subprocess spawn overhead (~50-200ms per character) for dramatically faster typing

### Changed

- **Completion Timeout Disabled by Default** - The `completion_timeout_minutes` setting now defaults to `0` (disabled) instead of `120` (2 hours). This prevents long-running tasks and UltraPlans from being interrupted. Users can still enable it by setting a non-zero value in their config.
- **Simplified Instance Navigation** - Removed 1-9 number key shortcuts for instance selection; use Tab/Shift+Tab or h/l to navigate between instances

### Fixed

- **Idle Tmux Connection Recovery** - Persistent tmux connection now auto-reconnects after becoming unresponsive during idle periods, preventing UI freezes when returning to a long-idle session
- **Tmux Instance Unresponsiveness** - Fixed critical issue where tmux instances could become completely unresponsive after extended use or network interruptions. Root causes addressed: goroutine leaks in persistent sender drain loops, missing timeouts on tmux subprocess calls causing capture loop freezes, and orphaned write goroutines accumulating over time
- **Tmux Scrollback History** - Fixed tmux sessions only keeping ~2000 lines of scrollback by setting `history-limit` before creating sessions. New default is 50,000 lines, configurable via `instance.tmux_history_limit`

## [0.3.0] - 2026-01-12

This release brings **deep GitHub Issues integration**, **plan import from URLs**, **a persistent terminal pane**, major **architecture improvements**, and numerous reliability enhancements across the board.

### Added

#### GitHub Issues Integration
- **Hierarchical Issue Creation** (#284) - Generate GitHub issues from your plan with proper parent/child relationships using GitHub's native sub-issues API
- **Issue Tracker Abstraction** (#293) - Clean interface allows future support for other issue trackers (Jira, Linear, etc.)
- **Auto-close Linked Issues** (#213) - When a task completes, its linked GitHub issue is automatically closed
- **Plan-only Mode** (#155) - Use `/plan` to create structured GitHub issues without executing
- **Plan Config Support** (#156, #160) - Configure plan templates and settings in your config file

#### Plan Import & URL Ingestion
- **URL-to-Plan Pipeline** (#226) - Import plans directly from URLs - paste a GitHub issue, gist, or any URL containing a plan spec
- **Plan File Import Fix** (#303) - Correctly compute execution order when importing existing plan files

#### Terminal Pane
- **Persistent Terminal Pane** (#233) - A dedicated terminal pane at the bottom of the TUI for direct shell access
- **ANSI Color Support** (#305) - Terminal output preserves colors
- **Width & Resize Fixes** (#308) - Terminal pane correctly handles resizing and dimension changes

#### Task Chaining
- **Normal Mode Chaining** (#228) - Chain tasks together in regular Claudio mode for sequential workflows

#### UltraPlan Enhancements
- **Step Restart & Universal Input** (#306) - Restart failed steps and send input to any task
- **Group Re-trigger & Session Resume** (#227) - Resume sessions and re-trigger groups
- **Multi-pass Planning** (#148) - Planning phase can iterate with Claude for complex breakdowns
- **Synthesis Approval Gate** (#149) - Pause for user approval before advancing from synthesis
- **Plan Editor** (#139, #131) - Interactive plan editor with validation and `--review` flag
- **Small Task Preference** (#143) - Planner prefers smaller, session-completable tasks

#### Configuration
- **Worktree Directory** (#231) - Configurable worktree directory path
- **Max Parallel Config** (#140) - Configure maximum parallel tasks in config file
- **Interactive Config** (#141) - Added missing options to interactive config UI

#### Session Management
- **:exit Command** (#217) - New `:exit` command for cleaner session management
- **Empty Session Cleanup** (#287) - Session cleanup now removes empty sessions

### Fixed

#### TUI Reliability
- **Command Mode for Quit** (#234) - Require command mode to quit, removing accidental exits
- **Command Mode for Terminal Focus** (#243) - Terminal focus now requires command mode
- **Command Mode Prompt Display** (#302) - Show command mode prompt in ultra-plan and plan editor modes
- **Cancel Safety** (#295) - UltraPlan cancel moved to command mode
- **Color Contrast** (#137) - Improved color contrast meets WCAG AA accessibility standards
- **Help Bar Shortcuts** (#229) - Command mode help bar shows all available shortcuts
- **Duplicate Title Fix** (#235) - Fixed duplicate sidebar title rendering in empty state
- **New Task Highlight** (#232) - "New Task" entry highlights in sidebar when adding a task

#### UltraPlan Reliability
- **Duplicate Task Completion** (#225) - Prevent duplicate completions from triggering premature synthesis
- **Partial Failure Handling** (#144) - Next group no longer starts after partial failure
- **Fallback Polling** (#154) - Added fallback polling for robust task completion detection
- **Parse Error Handling** (#153) - Gracefully handle parse errors during group consolidation
- **WaitingInput Detection** (#152) - Correctly detect completion for instances in WaitingInput
- **File-based Detection** (#150, #151) - Improved file-based detection for plan manager completion

#### Core Fixes
- **Git Subdirectory Detection** (#142) - Correctly detect git repository from subdirectories
- **Non-blocking Key Sending** (#146) - tmux key sending no longer blocks the event loop
- **Non-blocking Task Addition** (#145) - Adding tasks is now non-blocking

### Performance
- **Batch Character Input** (#307) - Consecutive characters batched to reduce subprocess calls
- **Differential Capture** (#292) - tmux output capture uses differential mode

### Changed

#### Architecture Refresh
- **Component Extraction** (#304) - Extracted focused components from "god objects"
- **Foundational Packages** (#214) - Extracted foundational packages and TUI view components
- **Event Bus Integration** (#221) - TUI uses event bus for decoupled communication
- **Session Subpackage** (#222) - Orchestrator session logic moved to dedicated subpackage
- **Multi-instance Infrastructure** (#215) - Core infrastructure for multi-instance execution
- **Group 3 Extraction** (#218, #219) - UltraPlan packages extracted with comprehensive tests

#### Testing & Quality
- **gofmt Enforcement** (#224) - Test enforces gofmt compliance
- **golangci-lint Compliance** (#294) - Test ensures golangci-lint passes
- **Integration Tests** (#223) - Added integration tests and package documentation
- **Lint Fixes** (#216) - Resolved all golangci-lint issues

### Documentation
- **CLAUDE.md Guidelines** (#237, #238) - Added Go development guidelines and architecture principles
- **AGENTS.md Rename** (#288) - Renamed CLAUDE.md to AGENTS.md with symlink

## [0.2.0] - 2026-01-09

First release featuring **UltraPlan mode** - an experimental planning and execution orchestration system for complex multi-task projects. While still having some rough edges, UltraPlan enables Claude to break down large projects into coordinated tasks, execute them in parallel across isolated worktrees, and consolidate the results into stacked PRs.

### Added

#### UltraPlan Mode
- **Planning Phase**: Claude analyzes your project request and creates a structured execution plan with task groups and dependencies
- **Execution Phase**: Tasks run in parallel across Git worktrees with real-time progress tracking
- **Synthesis Phase**: Results from each group are synthesized with visibility in the sidebar
- **Revision Phase**: Intermediate review step between synthesis and consolidation
- **Consolidation Phase**: Claude-driven consolidation creates stacked PRs from completed work

#### UltraPlan Features
- **Multi-session Support**: Run concurrent ultraplan sessions (#104)
- **Task Navigation**: Navigate between tasks during execution with keyboard shortcuts (#87)
- **Auto-proceed**: Automatically advance after plan detection (#85)
- **Audible Notifications**: Bell alerts when user input is needed (#107)
- **Per-group Consolidator Sessions**: Each group gets its own consolidation session (#120)
- **Sentinel Files**: Robust task completion detection using sentinel files across all phases (#115, #116)
- **Incremental Group Consolidation**: Preserves context while consolidating (#113)

### Fixed

#### UltraPlan Reliability
- Gracefully shutdown consolidator after completion file detection (#130)
- Handle notes field as string or array in completion files (#122)
- Prevent repeated bell notifications during group decision (#121)
- Show selection indicator for completed tasks (#119)
- Prevent premature task verification from false completion detection (#118)
- Stop treating StatusWaitingInput as task completion (#117)
- Verify task commits before marking complete (#114)
- Enforce group boundaries before starting next-phase tasks (#111)
- Prevent consolidation from firing before synthesis completes (#109)
- Properly detect task completion in ultraplan mode (#90)
- Detect task completion when waiting for input (#89)
- Skip blocked tasks during navigation (#88)

#### TUI Improvements
- Handle Enter key sent as rune in task input (#112)
- Show phase-appropriate progress in header (#106)
- Limit task display to 5 lines to prevent vertical clipping (#86)
- Reduce default tmux height for better visibility (#84)

## [0.1.0] - 2026-01-08

Initial release of Claudio - a CLI/TUI orchestration tool for running multiple Claude Code instances simultaneously using Git worktrees.

### Added

#### Core Features
- **Parallel Instance Management**: Run multiple Claude Code instances simultaneously on a single project
- **Git Worktree Isolation**: Each instance works in its own worktree and branch, preventing conflicts
- **TUI Dashboard**: Real-time terminal interface showing all instances with output streaming
- **Shared Context**: Auto-generated context files help instances coordinate and avoid duplicated work
- **Session Persistence**: Sessions survive disconnections and can be recovered on restart

#### Instance Control
- Start, pause, resume, and stop instances via TUI or CLI
- Automatic status detection (running, waiting for input, completed, error)
- Timeout detection and recovery for stuck instances
- Reconnect functionality for stopped instances

#### TUI Features
- Sidebar showing all instances with status indicators and pagination
- Scrollable output view with navigation controls (j/k, Page Up/Down, g/G)
- Output search and filtering with `/` command
- Interactive task input with keyboard navigation and paste support
- Task templates via `/` commands (e.g., `/test`, `/docs`, `/refactor`)
- Diff preview panel with `d` keyboard shortcut
- Conflict detail view with `c` keyboard shortcut
- Help overlay with `?`
- Completed instances section for finished work

#### PR Automation
- Claude-powered PR creation with smart rebase
- PR template support with customizable templates
- Automatic reviewer assignment from CODEOWNERS or configuration
- Auto-PR workflow when stopping instances with `x`
- Megamerge slash command for batch PR merging

#### Conflict Detection
- Real-time file conflict detection using fsnotify
- Visual warnings in TUI when multiple instances modify the same file
- Interactive conflict detail view

#### Configuration
- YAML configuration file support (`~/.config/claudio/config.yaml`)
- Environment variable overrides with `CLAUDIO_` prefix
- Interactive TUI for `claudio config` command
- Configurable branch naming convention (prefix, include ID)
- Completion actions: prompt, keep_branch, merge_staging, merge_main, auto_pr
- TUI settings: auto-focus, max output lines
- Instance settings: buffer size, capture interval, tmux dimensions

#### Resource Tracking
- Token usage tracking per instance
- API cost estimation
- Resource metrics display in TUI

#### CLI Commands
- `claudio init` - Initialize Claudio in a git repository
- `claudio start [name]` - Start a session and launch the TUI
- `claudio add "task"` - Add a new Claude instance with a task
- `claudio status` - Show current session status
- `claudio stop` - Stop all instances and end the session
- `claudio remove <id>` - Remove a specific instance and its worktree
- `claudio config` - View/edit configuration

#### Developer Experience
- Automatic stale worktree cleanup
- Native text selection (no mouse capture)
- Improved color contrast for readability
- Dynamic tmux pane resizing

### Infrastructure
- Integration tests with CI pipeline
- GitHub Actions workflow for testing on Ubuntu and macOS
- golangci-lint for code quality
- Comprehensive documentation with MkDocs

### Documentation
- Full user guide with getting started instructions
- Step-by-step tutorials for common workflows
- Complete CLI reference
- Configuration reference
- Troubleshooting guide and FAQ

[0.16.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.16.1
[0.16.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.16.0
[0.15.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.15.0
[0.14.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.14.1
[0.14.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.14.0
[0.13.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.13.0
[0.12.7]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.7
[0.12.6]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.6
[0.12.5]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.5
[0.12.4]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.4
[0.12.3]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.3
[0.12.2]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.2
[0.12.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.12.0
[0.11.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.11.0
[0.10.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.10.0
[0.9.2]: https://github.com/Iron-Ham/claudio/releases/tag/v0.9.2
[0.9.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.9.1
[0.9.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.9.0
[0.8.2]: https://github.com/Iron-Ham/claudio/releases/tag/v0.8.2
[0.8.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.8.1
[0.8.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.8.0
[0.7.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.7.1
[0.7.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.7.0
[0.6.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.6.1
[0.6.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.6.0
[0.5.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.5.1
[0.5.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.5.0
[0.4.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.4.0
[0.3.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.3.0
[0.2.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.2.0
[0.1.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.1.0

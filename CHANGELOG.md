# Changelog

All notable changes to Claudio will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

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

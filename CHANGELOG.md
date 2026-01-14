# Changelog

All notable changes to Claudio will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Progress Persistence & Session Recovery** - Claude instances now persist their session IDs, enabling automatic recovery when Claudio is interrupted. If Claudio exits unexpectedly while instances are running, reattaching to the session will automatically resume Claude conversations using `--resume`, picking up exactly where they left off. Sessions track clean shutdown state and can detect interruptions on restart.
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

[0.5.1]: https://github.com/Iron-Ham/claudio/releases/tag/v0.5.1
[0.5.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.5.0
[0.4.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.4.0
[0.3.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.3.0
[0.2.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.2.0
[0.1.0]: https://github.com/Iron-Ham/claudio/releases/tag/v0.1.0

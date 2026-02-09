# tui — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

## Pitfalls

- **Bubble Tea Cmd closures** — `tea.Cmd` functions must not capture mutable state by pointer. If you need to pass data into a Cmd, copy it into the closure at creation time. Capturing a pointer to model fields causes data races since the Bubble Tea runtime may execute the Cmd concurrently with the next `Update()` call.

## Architecture

- The TUI uses the [Bubble Tea](https://github.com/charmbracelet/bubbletea) framework (Elm architecture: Model → Update → View).
- `model.go` holds the top-level model; `update/` and `view/` separate the Update and View logic.
- `msg/` defines custom `tea.Msg` types for internal communication between components.
- `styles/` centralizes lipgloss styling — prefer reusing existing styles over creating new ones.
- **Event-driven pipeline state** — `view/pipeline_status.go` defines `PipelineState` and `TeamSnapshot` as TUI-local types built from events (no backend imports). `app.go` subscribes to 6 backend events (`pipeline.phase_changed`, `pipeline.completed`, `team.phase_changed`, `team.completed`, `bridge.task_started`, `bridge.task_completed`) and converts them to Bubble Tea messages. The `m.pipeline` field is nil until the first pipeline/team event (lazy init).

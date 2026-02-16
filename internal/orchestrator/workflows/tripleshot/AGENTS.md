# tripleshot — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

## Pitfalls

- **LLM output type mismatches in sentinel files** — LLMs frequently deviate from the expected JSON types. `FlexibleStringSlice` handles three cases: a plain string (`"fix the bug"`), an array of strings (`["fix A", "fix B"]`), and an array of objects (`[{"description":"fix A","source":"attempt_1"}]`). For objects, it extracts a well-known text key (`description`, `text`, `change`, `message`, `content`, `value`) or falls back to JSON-encoding the whole object. `FlexibleString` similarly handles string-or-array. When adding new LLM-parsed fields of type `string` or `[]string`, use these flexible types instead of bare Go types. Without this, `json.Unmarshal` fails, `VerifyWork` returns false, and the bridge retries the task — spawning a duplicate instance.
- **Sentinel file search in subdirectories** — `FindCompletionFile`, `FindEvaluationFile`, and `FindAdversarialReviewFile` all search the worktree root *and* immediate subdirectories. LLM instances sometimes write files relative to their CWD rather than the worktree root. Don't bypass `Find*File` with a direct `filepath.Join(worktree, filename)`.

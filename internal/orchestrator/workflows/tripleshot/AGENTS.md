# tripleshot — Agent Guidelines

> **Living document.** Update this file when you learn something specific to this package.
> Same rules as the root `AGENTS.md` — see its Self-Improvement Protocol.

## Pitfalls

- **LLM output type mismatches in sentinel files** — LLMs frequently write a plain string where the JSON schema expects `[]string` (e.g., `"suggested_changes": "fix the bug"` instead of `"suggested_changes": ["fix the bug"]`). The `Evaluation`, `AttemptEvaluationItem`, and `AdversarialReviewFile` structs use `FlexibleStringSlice` for all `[]string` fields and `FlexibleString` for `Reasoning` to tolerate this. When adding new LLM-parsed fields of type `string` or `[]string`, use these flexible types instead of bare Go types. Without this, `json.Unmarshal` fails, `VerifyWork` returns false, and the bridge retries the task — spawning a duplicate instance.
- **Sentinel file search in subdirectories** — `FindCompletionFile`, `FindEvaluationFile`, and `FindAdversarialReviewFile` all search the worktree root *and* immediate subdirectories. LLM instances sometimes write files relative to their CWD rather than the worktree root. Don't bypass `Find*File` with a direct `filepath.Join(worktree, filename)`.

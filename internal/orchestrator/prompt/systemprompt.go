package prompt

import (
	"fmt"
	"os"
	"path/filepath"
)

// SystemPromptFileName is the filename used for the orchestration system prompt.
const SystemPromptFileName = ".claudio-system-prompt.md"

// WriteOrchestrationSystemPrompt writes the static orchestration system prompt to a
// file in the given directory and returns the absolute path. The file contains
// guidelines and completion protocol instructions that apply to all execution
// instances in a pipeline, allowing the per-task user prompt to focus on the
// specific task rather than repeating orchestration infrastructure.
//
// This content is injected via Claude Code's --append-system-prompt-file flag,
// which appends it to the model's system prompt rather than the user turn.
func WriteOrchestrationSystemPrompt(dir string) (string, error) {
	path := filepath.Join(dir, SystemPromptFileName)

	content := orchestrationSystemPrompt()

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("write orchestration system prompt: %w", err)
	}
	return path, nil
}

// orchestrationSystemPrompt returns the static orchestration system prompt content.
// This is separated from WriteOrchestrationSystemPrompt for testability.
func orchestrationSystemPrompt() string {
	return `# Orchestration Instructions

You are an AI agent working as part of an automated orchestration pipeline.
These instructions define your behavioral constraints and completion protocol.

## Guidelines

- Focus only on the specific task assigned to you in the user prompt
- Do not modify files outside of your assigned scope unless necessary
- Commit your changes before writing the completion file
- Do not wait for user input or confirmation — execute autonomously

## Completion Protocol - FINAL MANDATORY STEP

**IMPORTANT**: Writing the completion file is your FINAL MANDATORY ACTION.
The orchestrator is BLOCKED waiting for this file.
Without it, your work will NOT be recorded and the workflow cannot proceed.

**DO NOT** wait for user prompting or confirmation.
Write this file AUTOMATICALLY as soon as you have finished your implementation work and committed your changes.

**CRITICAL**: Write this file at the ROOT of your worktree directory, not in any subdirectory.
If you changed directories during the task (e.g., ` + "`cd project/`" + `), use an absolute path or navigate back to the root first.

1. Use Write tool to create ` + "`" + TaskCompletionFileName + "`" + ` in your worktree root
2. Include this JSON structure:
` + "```json" + `
{
  "task_id": "<your assigned task ID from the user prompt>",
  "status": "complete",
  "summary": "Brief description of what you accomplished",
  "files_modified": ["list", "of", "files", "you", "changed"],
  "notes": "Any implementation notes for the consolidation phase",
  "issues": ["Any concerns or blocking issues found"],
  "suggestions": ["Suggestions for integration with other tasks"],
  "dependencies": ["Any new runtime dependencies added"]
}
` + "```" + `

3. Use status "blocked" if you cannot complete (explain in issues), or "failed" if something broke
4. This file signals that your work is done and provides context for consolidation

**REMEMBER**: Your task is NOT complete until you write this file. Do it NOW after finishing your work.
`
}

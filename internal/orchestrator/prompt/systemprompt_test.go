package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteOrchestrationSystemPrompt(t *testing.T) {
	dir := t.TempDir()

	path, err := WriteOrchestrationSystemPrompt(dir)
	if err != nil {
		t.Fatalf("WriteOrchestrationSystemPrompt: %v", err)
	}

	// Verify path
	expected := filepath.Join(dir, SystemPromptFileName)
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}

	// Verify file exists and is readable
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read system prompt file: %v", err)
	}

	text := string(content)

	// Verify key sections are present
	if !strings.Contains(text, "# Orchestration Instructions") {
		t.Error("missing orchestration instructions header")
	}
	if !strings.Contains(text, "## Guidelines") {
		t.Error("missing guidelines section")
	}
	if !strings.Contains(text, "## Completion Protocol") {
		t.Error("missing completion protocol section")
	}
	if !strings.Contains(text, TaskCompletionFileName) {
		t.Error("missing completion file name reference")
	}
	if !strings.Contains(text, "task_id") {
		t.Error("missing task_id field in completion protocol")
	}
}

func TestWriteOrchestrationSystemPrompt_InvalidDir(t *testing.T) {
	_, err := WriteOrchestrationSystemPrompt("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestWriteOrchestrationSystemPrompt_Idempotent(t *testing.T) {
	dir := t.TempDir()

	path1, err := WriteOrchestrationSystemPrompt(dir)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}

	path2, err := WriteOrchestrationSystemPrompt(dir)
	if err != nil {
		t.Fatalf("second write: %v", err)
	}

	if path1 != path2 {
		t.Errorf("paths differ: %q vs %q", path1, path2)
	}

	// Content should be identical
	content1, _ := os.ReadFile(path1)
	content2, _ := os.ReadFile(path2)
	if string(content1) != string(content2) {
		t.Error("content differs between writes")
	}
}

func TestOrchestrationSystemPrompt_Content(t *testing.T) {
	content := orchestrationSystemPrompt()

	tests := []struct {
		name     string
		contains string
	}{
		{"header", "# Orchestration Instructions"},
		{"guidelines", "## Guidelines"},
		{"focus instruction", "Focus only on the specific task"},
		{"commit instruction", "Commit your changes before writing the completion file"},
		{"completion protocol", "## Completion Protocol - FINAL MANDATORY STEP"},
		{"write tool instruction", "Use Write tool to create"},
		{"completion file name", TaskCompletionFileName},
		{"json schema", "task_id"},
		{"status field", "\"status\": \"complete\""},
		{"remember footer", "REMEMBER"},
		{"no user prompt", "Do not wait for user input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(content, tt.contains) {
				t.Errorf("system prompt missing %q", tt.contains)
			}
		})
	}
}

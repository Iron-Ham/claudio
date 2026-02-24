package streamjson

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestBuildSubprocessArgs_Minimal(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{})

	assertContains(t, args, "--print")
	assertContains(t, args, "--output-format")
	assertContains(t, args, "stream-json")
	assertContains(t, args, "--prompt-file")
	assertContains(t, args, "/tmp/prompt.txt")
}

func TestBuildSubprocessArgs_AllOptions(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{
		PermissionMode:         "auto-accept",
		Model:                  "claude-opus-4-6",
		MaxTurns:               100,
		AllowedTools:           []string{"Read", "Write"},
		DisallowedTools:        []string{"Bash"},
		AppendSystemPromptFile: "/tmp/system.md",
		NoUserPrompt:           true,
	})

	assertContains(t, args, "--permission-mode")
	assertContains(t, args, "auto-accept")
	assertContains(t, args, "--model")
	assertContains(t, args, "claude-opus-4-6")
	assertContains(t, args, "--max-turns")
	assertContains(t, args, "100")
	assertContainsPair(t, args, "--allowedTools", "Read")
	assertContainsPair(t, args, "--allowedTools", "Write")
	assertContainsPair(t, args, "--disallowedTools", "Bash")
	assertContains(t, args, "--append-system-prompt-file")
	assertContains(t, args, "/tmp/system.md")
	assertContains(t, args, "--no-user-prompt")
}

func TestBuildSubprocessArgs_BypassPermission(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{
		PermissionMode: "bypass",
	})

	assertContains(t, args, "--dangerously-skip-permissions")
	assertNotContains(t, args, "--permission-mode")
}

func TestBuildSubprocessArgs_PlanPermission(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{
		PermissionMode: "plan",
	})

	assertContains(t, args, "--permission-mode")
	assertContains(t, args, "plan")
}

func TestBuildSubprocessArgs_DefaultPermission(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{
		PermissionMode: "default",
	})

	assertNotContains(t, args, "--permission-mode")
	assertNotContains(t, args, "--dangerously-skip-permissions")
}

func TestBuildSubprocessArgs_EmptyPermission(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{})

	assertNotContains(t, args, "--permission-mode")
	assertNotContains(t, args, "--dangerously-skip-permissions")
}

func TestBuildSubprocessArgs_NoMaxTurns(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{
		MaxTurns: 0,
	})

	assertNotContains(t, args, "--max-turns")
}

func TestBuildSubprocessArgs_NoModel(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{})

	assertNotContains(t, args, "--model")
}

func TestBuildSubprocessArgs_NoTools(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{})

	assertNotContains(t, args, "--allowedTools")
	assertNotContains(t, args, "--disallowedTools")
}

// --- Helpers ---

func assertContains(t *testing.T, args []string, want string) {
	t.Helper()
	for _, a := range args {
		if a == want {
			return
		}
	}
	t.Errorf("args %v does not contain %q", args, want)
}

func assertNotContains(t *testing.T, args []string, unwanted string) {
	t.Helper()
	for _, a := range args {
		if a == unwanted {
			t.Errorf("args %v should not contain %q", args, unwanted)
			return
		}
	}
}

func assertContainsPair(t *testing.T, args []string, flag, value string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return
		}
	}
	t.Errorf("args %v does not contain %q %q pair", args, flag, value)
}

func TestRunSubprocess_NonZeroExitNoResult(t *testing.T) {
	result, err := RunSubprocess(context.Background(), "sh", []string{"-c", "exit 1"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for non-zero exit without result")
	}
	if result == nil {
		t.Fatal("result should not be nil even on error")
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
	if result.Result != nil {
		t.Error("Result should be nil when no result event was produced")
	}
}

func TestRunSubprocess_SuccessWithResult(t *testing.T) {
	jsonLine := `{"type":"result","subtype":"success","cost_usd":0.01,"total_cost_usd":0.01,"duration_ms":100,"is_error":false,"num_turns":1,"session_id":"s1","usage":{"input_tokens":10,"output_tokens":5}}`
	result, err := RunSubprocess(
		context.Background(), "sh",
		[]string{"-c", fmt.Sprintf("echo '%s'", jsonLine)},
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Result == nil {
		t.Fatal("expected ResultEvent")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.ReadError != nil {
		t.Errorf("ReadError = %v, want nil", result.ReadError)
	}
}

func TestRunSubprocess_NonZeroExitWithResult(t *testing.T) {
	// Process exits non-zero but produced a result — no error returned.
	jsonLine := `{"type":"result","subtype":"error","cost_usd":0.01,"total_cost_usd":0.01,"duration_ms":100,"is_error":true,"num_turns":1,"session_id":"s1","usage":{"input_tokens":10,"output_tokens":5},"error":"max turns exceeded"}`
	result, err := RunSubprocess(
		context.Background(), "sh",
		[]string{"-c", fmt.Sprintf("echo '%s'; exit 1", jsonLine)},
		t.TempDir(),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v (result with non-zero exit should not return error)", err)
	}
	if result.Result == nil {
		t.Fatal("expected ResultEvent")
	}
	if result.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestRunSubprocess_ZeroExitNoResult(t *testing.T) {
	// Process exits cleanly but produced no result — no error (exit code is 0).
	result, err := RunSubprocess(
		context.Background(), "sh",
		[]string{"-c", "echo 'not json but ignored because it will fail parse'"},
		t.TempDir(),
	)
	// The echo output is not valid JSON, so reader.Next() will return a parse error.
	// ReadError should be set, but since exit code is 0, no error is returned.
	if err != nil {
		t.Fatalf("unexpected error for zero exit: %v", err)
	}
	if result.ReadError == nil {
		t.Error("ReadError should be set for malformed JSON")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestRunSubprocess_EmptyOutput(t *testing.T) {
	// Process exits cleanly with no output — no error.
	result, err := RunSubprocess(context.Background(), "sh", []string{"-c", "true"}, t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Result != nil {
		t.Error("Result should be nil for empty output")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.ReadError != nil {
		t.Errorf("ReadError = %v, want nil", result.ReadError)
	}
	if len(result.Events) != 0 {
		t.Errorf("Events len = %d, want 0", len(result.Events))
	}
}

func TestBuildSubprocessArgs_ArgsAreNotShellString(t *testing.T) {
	args := BuildSubprocessArgs("/tmp/prompt.txt", SubprocessOptions{
		Model: "claude-opus-4-6",
	})

	// Verify args are individual strings, not a joined shell command
	for _, a := range args {
		if strings.Contains(a, " ") && a != "stream-json" {
			t.Errorf("arg %q contains spaces — should be individual args, not shell string", a)
		}
	}
}

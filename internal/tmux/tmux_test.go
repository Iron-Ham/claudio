package tmux

import (
	"context"
	"testing"
)

func TestSocketName(t *testing.T) {
	if SocketName == "" {
		t.Error("SocketName should not be empty")
	}
	if SocketName != "claudio" {
		t.Errorf("SocketName = %q, want %q", SocketName, "claudio")
	}
}

func TestCommand(t *testing.T) {
	cmd := Command("list-sessions")
	args := cmd.Args

	if len(args) < 4 {
		t.Fatalf("Expected at least 4 args, got %d: %v", len(args), args)
	}

	if args[0] != "tmux" {
		t.Errorf("args[0] = %q, want %q", args[0], "tmux")
	}
	if args[1] != "-L" {
		t.Errorf("args[1] = %q, want %q", args[1], "-L")
	}
	if args[2] != SocketName {
		t.Errorf("args[2] = %q, want %q", args[2], SocketName)
	}
	if args[3] != "list-sessions" {
		t.Errorf("args[3] = %q, want %q", args[3], "list-sessions")
	}
}

func TestCommandArgs(t *testing.T) {
	args := CommandArgs("kill-session", "-t", "test")

	expected := []string{"-L", SocketName, "kill-session", "-t", "test"}
	if len(args) != len(expected) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(expected))
	}

	for i, want := range expected {
		if args[i] != want {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want)
		}
	}
}

func TestBaseArgs(t *testing.T) {
	args := BaseArgs()

	if len(args) != 2 {
		t.Fatalf("len(args) = %d, want 2", len(args))
	}
	if args[0] != "-L" {
		t.Errorf("args[0] = %q, want %q", args[0], "-L")
	}
	if args[1] != SocketName {
		t.Errorf("args[1] = %q, want %q", args[1], SocketName)
	}
}

func TestCommandContext(t *testing.T) {
	ctx := context.Background()
	cmd := CommandContext(ctx, "has-session", "-t", "test")
	args := cmd.Args

	if len(args) < 6 {
		t.Fatalf("Expected at least 6 args, got %d: %v", len(args), args)
	}

	if args[0] != "tmux" {
		t.Errorf("args[0] = %q, want %q", args[0], "tmux")
	}
	if args[1] != "-L" {
		t.Errorf("args[1] = %q, want %q", args[1], "-L")
	}
	if args[2] != SocketName {
		t.Errorf("args[2] = %q, want %q", args[2], SocketName)
	}
	if args[3] != "has-session" {
		t.Errorf("args[3] = %q, want %q", args[3], "has-session")
	}
}

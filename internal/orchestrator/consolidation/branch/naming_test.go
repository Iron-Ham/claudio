package branch

import (
	"testing"
)

func TestNamingStrategy_GroupBranchName(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		sessionID string
		groupIdx  int
		want      string
	}{
		{
			name:      "default prefix first group",
			prefix:    "",
			sessionID: "abcd1234",
			groupIdx:  0,
			want:      "Iron-Ham/ultraplan-abcd1234-group-1",
		},
		{
			name:      "default prefix second group",
			prefix:    "",
			sessionID: "abcd1234",
			groupIdx:  1,
			want:      "Iron-Ham/ultraplan-abcd1234-group-2",
		},
		{
			name:      "custom prefix",
			prefix:    "feature",
			sessionID: "efgh5678",
			groupIdx:  0,
			want:      "feature/ultraplan-efgh5678-group-1",
		},
		{
			name:      "long session ID truncated",
			prefix:    "Iron-Ham",
			sessionID: "abcdefghijklmnop",
			groupIdx:  0,
			want:      "Iron-Ham/ultraplan-abcdefgh-group-1",
		},
		{
			name:      "short session ID",
			prefix:    "Iron-Ham",
			sessionID: "abc",
			groupIdx:  2,
			want:      "Iron-Ham/ultraplan-abc-group-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := NewNamingStrategy(tt.prefix, tt.sessionID)
			got := ns.GroupBranchName(tt.groupIdx)
			if got != tt.want {
				t.Errorf("GroupBranchName(%d) = %q, want %q", tt.groupIdx, got, tt.want)
			}
		})
	}
}

func TestNamingStrategy_SingleBranchName(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		sessionID string
		want      string
	}{
		{
			name:      "default prefix",
			prefix:    "",
			sessionID: "abcd1234",
			want:      "Iron-Ham/ultraplan-abcd1234",
		},
		{
			name:      "custom prefix",
			prefix:    "my-project",
			sessionID: "xyz98765",
			want:      "my-project/ultraplan-xyz98765",
		},
		{
			name:      "long session ID truncated",
			prefix:    "Iron-Ham",
			sessionID: "verylongsessionidthatexceeds",
			want:      "Iron-Ham/ultraplan-verylong",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := NewNamingStrategy(tt.prefix, tt.sessionID)
			got := ns.SingleBranchName()
			if got != tt.want {
				t.Errorf("SingleBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNamingStrategy_TaskBranchName(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		sessionID string
		taskID    string
		want      string
	}{
		{
			name:      "simple task ID",
			prefix:    "Iron-Ham",
			sessionID: "abcd1234",
			taskID:    "task-1",
			want:      "Iron-Ham/ultraplan-abcd1234-task-1",
		},
		{
			name:      "task ID with spaces",
			prefix:    "Iron-Ham",
			sessionID: "abcd1234",
			taskID:    "task setup",
			want:      "Iron-Ham/ultraplan-abcd1234-task-setup",
		},
		{
			name:      "task ID with special chars",
			prefix:    "Iron-Ham",
			sessionID: "abcd1234",
			taskID:    "task-1/setup@test",
			want:      "Iron-Ham/ultraplan-abcd1234-task-1setuptest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns := NewNamingStrategy(tt.prefix, tt.sessionID)
			got := ns.TaskBranchName(tt.taskID)
			if got != tt.want {
				t.Errorf("TaskBranchName(%q) = %q, want %q", tt.taskID, got, tt.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "hello world", want: "hello-world"},
		{input: "Hello World", want: "hello-world"},
		{input: "task-1-setup", want: "task-1-setup"},
		{input: "special@chars#removed!", want: "specialcharsremoved"},
		{input: "numbers123okay", want: "numbers123okay"},
		{input: "this is a very long string that should be truncated to fit the limit", want: "this-is-a-very-long-string-tha"},
		{input: "", want: ""},
		{input: "trailing-dash---", want: "trailing-dash"},
		{input: "MiXeD CaSe", want: "mixed-case"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.want {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNamingStrategy_Accessors(t *testing.T) {
	ns := NewNamingStrategy("my-prefix", "session123")

	if got := ns.Prefix(); got != "my-prefix" {
		t.Errorf("Prefix() = %q, want %q", got, "my-prefix")
	}

	if got := ns.SessionID(); got != "session123" {
		t.Errorf("SessionID() = %q, want %q", got, "session123")
	}
}

func TestNewNamingStrategy_DefaultPrefix(t *testing.T) {
	ns := NewNamingStrategy("", "abc")
	if ns.Prefix() != "Iron-Ham" {
		t.Errorf("NewNamingStrategy with empty prefix should default to Iron-Ham, got %q", ns.Prefix())
	}
}

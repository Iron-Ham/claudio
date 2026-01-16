package worktree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitmodules(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantCount int
		wantFirst *SubmoduleInfo
	}{
		{
			name:      "empty file",
			content:   "",
			wantCount: 0,
		},
		{
			name: "single submodule",
			content: `[submodule "mylib"]
	path = vendor/mylib
	url = https://github.com/example/mylib.git
`,
			wantCount: 1,
			wantFirst: &SubmoduleInfo{
				Name: "mylib",
				Path: "vendor/mylib",
				URL:  "https://github.com/example/mylib.git",
			},
		},
		{
			name: "multiple submodules",
			content: `[submodule "lib1"]
	path = vendor/lib1
	url = https://github.com/example/lib1.git

[submodule "lib2"]
	path = vendor/lib2
	url = git@github.com:example/lib2.git
	branch = develop
`,
			wantCount: 2,
			wantFirst: &SubmoduleInfo{
				Name: "lib1",
				Path: "vendor/lib1",
				URL:  "https://github.com/example/lib1.git",
			},
		},
		{
			name: "with comments and blank lines",
			content: `# This is a comment
[submodule "mylib"]
	path = libs/mylib
	url = https://example.com/mylib.git
	# Another comment
`,
			wantCount: 1,
			wantFirst: &SubmoduleInfo{
				Name: "mylib",
				Path: "libs/mylib",
				URL:  "https://example.com/mylib.git",
			},
		},
		{
			name: "submodule name with spaces",
			content: `[submodule "my lib name"]
	path = vendor/mylib
	url = https://example.com/mylib.git
`,
			wantCount: 1,
			wantFirst: &SubmoduleInfo{
				Name: "my lib name",
				Path: "vendor/mylib",
				URL:  "https://example.com/mylib.git",
			},
		},
		{
			name: "handles tabs and spaces in values",
			content: `[submodule "lib"]
	path =   vendor/lib
	url =	https://example.com/lib.git
`,
			wantCount: 1,
			wantFirst: &SubmoduleInfo{
				Name: "lib",
				Path: "vendor/lib",
				URL:  "https://example.com/lib.git",
			},
		},
		{
			name: "skips submodule without path",
			content: `[submodule "nopath"]
	url = https://example.com/nopath.git

[submodule "haspath"]
	path = vendor/haspath
	url = https://example.com/haspath.git
`,
			wantCount: 1,
			wantFirst: &SubmoduleInfo{
				Name: "haspath",
				Path: "vendor/haspath",
				URL:  "https://example.com/haspath.git",
			},
		},
		{
			name: "with branch field",
			content: `[submodule "lib"]
	path = vendor/lib
	url = https://example.com/lib.git
	branch = feature/dev
`,
			wantCount: 1,
			wantFirst: &SubmoduleInfo{
				Name:   "lib",
				Path:   "vendor/lib",
				URL:    "https://example.com/lib.git",
				Branch: "feature/dev",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the content
			tmpFile, err := os.CreateTemp("", "gitmodules")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer func() { _ = os.Remove(tmpFile.Name()) }()

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			if _, err := tmpFile.Seek(0, 0); err != nil {
				t.Fatalf("failed to seek temp file: %v", err)
			}

			submodules, err := parseGitmodules(tmpFile)
			if err != nil {
				t.Fatalf("parseGitmodules() error = %v", err)
			}

			if len(submodules) != tt.wantCount {
				t.Errorf("parseGitmodules() returned %d submodules, want %d", len(submodules), tt.wantCount)
			}

			if tt.wantFirst != nil && len(submodules) > 0 {
				got := submodules[0]
				if got.Name != tt.wantFirst.Name {
					t.Errorf("Name = %q, want %q", got.Name, tt.wantFirst.Name)
				}
				if got.Path != tt.wantFirst.Path {
					t.Errorf("Path = %q, want %q", got.Path, tt.wantFirst.Path)
				}
				if got.URL != tt.wantFirst.URL {
					t.Errorf("URL = %q, want %q", got.URL, tt.wantFirst.URL)
				}
				if got.Branch != tt.wantFirst.Branch {
					t.Errorf("Branch = %q, want %q", got.Branch, tt.wantFirst.Branch)
				}
			}
		})
	}
}

func TestIsSubmoduleDir_Unit(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T) string
		want  bool
	}{
		{
			name: "directory with .git file pointing to gitdir",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitFile := filepath.Join(dir, ".git")
				if err := os.WriteFile(gitFile, []byte("gitdir: /path/to/main/.git/modules/submod"), 0644); err != nil {
					t.Fatalf("failed to write .git file: %v", err)
				}
				return dir
			},
			want: true,
		},
		{
			name: "directory with .git directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitDir := filepath.Join(dir, ".git")
				if err := os.Mkdir(gitDir, 0755); err != nil {
					t.Fatalf("failed to create .git directory: %v", err)
				}
				return dir
			},
			want: false,
		},
		{
			name: "directory without .git",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			want: false,
		},
		{
			name: "directory with .git file but no gitdir prefix",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				gitFile := filepath.Join(dir, ".git")
				if err := os.WriteFile(gitFile, []byte("some other content"), 0644); err != nil {
					t.Fatalf("failed to write .git file: %v", err)
				}
				return dir
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setup(t)
			got := IsSubmoduleDir(dir)
			if got != tt.want {
				t.Errorf("IsSubmoduleDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubmoduleStatus_String(t *testing.T) {
	tests := []struct {
		status SubmoduleStatus
		want   string
	}{
		{SubmoduleUpToDate, "up-to-date"},
		{SubmoduleNotInitialized, "not-initialized"},
		{SubmoduleDifferentCommit, "different-commit"},
		{SubmoduleMergeConflict, "merge-conflict"},
		{SubmoduleStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.status.String()
			if got != tt.want {
				t.Errorf("SubmoduleStatus.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSubmoduleStatus(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantCount int
		wantFirst *SubmoduleStatusInfo
	}{
		{
			name:      "empty output",
			output:    "",
			wantCount: 0,
		},
		{
			name:      "up-to-date submodule",
			output:    " abc1234 vendor/lib (v1.0.0)",
			wantCount: 1,
			wantFirst: &SubmoduleStatusInfo{
				Path:   "vendor/lib",
				Commit: "abc1234",
				Status: SubmoduleUpToDate,
			},
		},
		{
			name:      "not initialized submodule",
			output:    "-abc1234 vendor/lib",
			wantCount: 1,
			wantFirst: &SubmoduleStatusInfo{
				Path:   "vendor/lib",
				Commit: "abc1234",
				Status: SubmoduleNotInitialized,
			},
		},
		{
			name:      "different commit submodule",
			output:    "+abc1234 vendor/lib (heads/main)",
			wantCount: 1,
			wantFirst: &SubmoduleStatusInfo{
				Path:   "vendor/lib",
				Commit: "abc1234",
				Status: SubmoduleDifferentCommit,
			},
		},
		{
			name:      "merge conflict submodule",
			output:    "Uabc1234 vendor/lib",
			wantCount: 1,
			wantFirst: &SubmoduleStatusInfo{
				Path:   "vendor/lib",
				Commit: "abc1234",
				Status: SubmoduleMergeConflict,
			},
		},
		{
			name: "multiple submodules",
			output: ` abc1234 vendor/lib1 (v1.0.0)
+def5678 vendor/lib2 (heads/main)
-ghi9012 vendor/lib3`,
			wantCount: 3,
			wantFirst: &SubmoduleStatusInfo{
				Path:   "vendor/lib1",
				Commit: "abc1234",
				Status: SubmoduleUpToDate,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSubmoduleStatus(tt.output)

			if len(got) != tt.wantCount {
				t.Errorf("parseSubmoduleStatus() returned %d results, want %d", len(got), tt.wantCount)
			}

			if tt.wantFirst != nil && len(got) > 0 {
				if got[0].Path != tt.wantFirst.Path {
					t.Errorf("Path = %q, want %q", got[0].Path, tt.wantFirst.Path)
				}
				if got[0].Commit != tt.wantFirst.Commit {
					t.Errorf("Commit = %q, want %q", got[0].Commit, tt.wantFirst.Commit)
				}
				if got[0].Status != tt.wantFirst.Status {
					t.Errorf("Status = %v, want %v", got[0].Status, tt.wantFirst.Status)
				}
			}
		})
	}
}

func TestIsSubmoduleCriticalError(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "normal output",
			output: "Submodule 'vendor/lib' (https://example.com/lib.git) registered for path 'vendor/lib'",
			want:   false,
		},
		{
			name:   "empty output",
			output: "",
			want:   false,
		},
		{
			name:   "fatal error",
			output: "fatal: repository 'https://example.com/lib.git' not found",
			want:   true,
		},
		{
			name:   "permission denied",
			output: "error: permission denied for 'vendor/lib'",
			want:   true,
		},
		{
			name:   "repository not found",
			output: "error: Repository not found",
			want:   true,
		},
		{
			name:   "authentication failed",
			output: "error: Authentication failed for 'https://example.com/repo.git'",
			want:   true,
		},
		{
			name:   "host key verification",
			output: "Host key verification failed",
			want:   true,
		},
		{
			name:   "clone failed",
			output: "Clone of 'https://example.com/repo.git' into submodule path 'vendor/lib' failed",
			want:   true,
		},
		{
			name:   "could not read from remote",
			output: "fatal: Could not read from remote repository",
			want:   true,
		},
		{
			name:   "unable to access",
			output: "fatal: unable to access 'https://example.com/repo.git/': Connection refused",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSubmoduleCriticalError(tt.output)
			if got != tt.want {
				t.Errorf("isSubmoduleCriticalError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubmoduleError_Error(t *testing.T) {
	err := &SubmoduleError{
		Operation: "init",
		Output:    "some output",
		Err:       os.ErrPermission,
	}

	got := err.Error()
	if !strings.Contains(got, "init") {
		t.Errorf("Error() should contain operation name 'init', got %q", got)
	}
	if !strings.Contains(got, "permission") {
		t.Errorf("Error() should contain underlying error, got %q", got)
	}
	if !strings.Contains(got, "some output") {
		t.Errorf("Error() should contain output, got %q", got)
	}
}

func TestSubmoduleError_Unwrap(t *testing.T) {
	underlying := os.ErrPermission
	err := &SubmoduleError{
		Operation: "init",
		Output:    "output",
		Err:       underlying,
	}

	if err.Unwrap() != underlying {
		t.Errorf("Unwrap() = %v, want %v", err.Unwrap(), underlying)
	}
}

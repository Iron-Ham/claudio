package tracker

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestParseIssueNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{
			name:    "standard github url",
			input:   "https://github.com/owner/repo/issues/123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "url with trailing newline",
			input:   "https://github.com/owner/repo/issues/456\n",
			want:    456,
			wantErr: false,
		},
		{
			name:    "url with extra path",
			input:   "https://github.com/owner/repo/issues/789/comments",
			want:    789,
			wantErr: false,
		},
		{
			name:    "invalid url - no issues path",
			input:   "https://github.com/owner/repo/pull/123",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid url - no number",
			input:   "https://github.com/owner/repo/issues/",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIssueNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIssueNumber() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseIssueNumber() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGitHubTracker_SupportsHierarchy(t *testing.T) {
	tracker := NewGitHubTracker()
	if !tracker.SupportsHierarchy() {
		t.Error("GitHubTracker should support hierarchy")
	}
}

func TestGitHubTracker_SupportsLabels(t *testing.T) {
	tracker := NewGitHubTracker()
	if !tracker.SupportsLabels() {
		t.Error("GitHubTracker should support labels")
	}
}

func TestGitHubTracker_CreateIssue(t *testing.T) {
	tests := []struct {
		name        string
		opts        IssueOptions
		mockOutput  []byte
		mockError   error
		wantNumber  int
		wantURL     string
		wantErr     bool
		wantErrType error
	}{
		{
			name: "successful creation",
			opts: IssueOptions{
				Title:  "Test Issue",
				Body:   "Test body",
				Labels: []string{"bug"},
			},
			mockOutput: []byte("https://github.com/owner/repo/issues/42"),
			mockError:  nil,
			wantNumber: 42,
			wantURL:    "https://github.com/owner/repo/issues/42",
			wantErr:    false,
		},
		{
			name: "gh not installed",
			opts: IssueOptions{
				Title: "Test Issue",
				Body:  "Test body",
			},
			mockOutput:  nil,
			mockError:   &exec.Error{Name: "gh", Err: errors.New("executable file not found")},
			wantErr:     true,
			wantErrType: ErrProviderUnavailable,
		},
		{
			name: "authentication required",
			opts: IssueOptions{
				Title: "Test Issue",
				Body:  "Test body",
			},
			mockOutput:  []byte("To authenticate, run: gh auth login"),
			mockError:   fmt.Errorf("exit status 1"),
			wantErr:     true,
			wantErrType: ErrAuthRequired,
		},
		{
			name: "empty title",
			opts: IssueOptions{
				Title: "",
				Body:  "Test body",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track the call count to return node ID on second call
			callCount := 0
			tracker := NewGitHubTrackerWithExecutor(func(name string, args ...string) ([]byte, error) {
				callCount++
				if callCount == 1 {
					// First call is issue create
					return tt.mockOutput, tt.mockError
				}
				// Second call would be to get node ID - return empty for simplicity
				return []byte(`{"id": ""}`), nil
			})

			ref, err := tracker.CreateIssue(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateIssue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrType != nil {
				if !errors.Is(err, tt.wantErrType) {
					t.Errorf("CreateIssue() error = %v, want %v", err, tt.wantErrType)
				}
				return
			}
			if ref.Number != tt.wantNumber {
				t.Errorf("CreateIssue() number = %v, want %v", ref.Number, tt.wantNumber)
			}
			if ref.URL != tt.wantURL {
				t.Errorf("CreateIssue() url = %v, want %v", ref.URL, tt.wantURL)
			}
		})
	}
}

func TestGitHubTracker_UpdateIssue(t *testing.T) {
	tests := []struct {
		name        string
		ref         IssueRef
		opts        IssueOptions
		mockOutput  []byte
		mockError   error
		wantErr     bool
		wantErrType error
	}{
		{
			name: "successful update",
			ref:  IssueRef{Number: 42},
			opts: IssueOptions{
				Body: "Updated body",
			},
			mockOutput: []byte(""),
			mockError:  nil,
			wantErr:    false,
		},
		{
			name: "missing issue number",
			ref:  IssueRef{},
			opts: IssueOptions{
				Body: "Updated body",
			},
			wantErr: true,
		},
		{
			name: "issue not found",
			ref:  IssueRef{Number: 999},
			opts: IssueOptions{
				Body: "Updated body",
			},
			mockOutput:  []byte("issue not found"),
			mockError:   fmt.Errorf("exit status 1"),
			wantErr:     true,
			wantErrType: ErrIssueNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewGitHubTrackerWithExecutor(func(name string, args ...string) ([]byte, error) {
				return tt.mockOutput, tt.mockError
			})

			err := tracker.UpdateIssue(tt.ref, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateIssue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.wantErrType != nil {
				if !errors.Is(err, tt.wantErrType) {
					t.Errorf("UpdateIssue() error = %v, want %v", err, tt.wantErrType)
				}
			}
		})
	}
}

func TestGitHubTracker_AddSubIssue(t *testing.T) {
	tests := []struct {
		name      string
		parentRef IssueRef
		subRef    IssueRef
		mockCalls []struct {
			output []byte
			err    error
		}
		wantErr bool
	}{
		{
			name:      "successful with node IDs",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte(`{"data": {"addSubIssue": {"issue": {"number": 1}}}}`), err: nil},
			},
			wantErr: false,
		},
		{
			name:      "successful with issue numbers",
			parentRef: IssueRef{Number: 1},
			subRef:    IssueRef{Number: 2},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				// First call gets parent node ID
				{output: []byte(`{"id": "parent-node-id"}`), err: nil},
				// Second call gets sub-issue node ID
				{output: []byte(`{"id": "sub-node-id"}`), err: nil},
				// Third call adds sub-issue
				{output: []byte(`{"data": {"addSubIssue": {"issue": {"number": 1}}}}`), err: nil},
			},
			wantErr: false,
		},
		{
			name:      "graphql error",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte(`{"errors": [{"message": "Could not resolve to a node"}]}`), err: nil},
			},
			wantErr: true,
		},
		{
			name:      "parent node ID fetch failure",
			parentRef: IssueRef{Number: 999},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte("issue not found"), err: fmt.Errorf("exit status 1")},
			},
			wantErr: true,
		},
		{
			name:      "sub-issue node ID fetch failure",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{Number: 999},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte("issue not found"), err: fmt.Errorf("exit status 1")},
			},
			wantErr: true,
		},
		{
			name:      "malformed JSON response",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte("not valid json"), err: nil},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callIdx := 0
			tracker := NewGitHubTrackerWithExecutor(func(name string, args ...string) ([]byte, error) {
				if callIdx >= len(tt.mockCalls) {
					t.Fatalf("unexpected call %d to executor", callIdx)
				}
				result := tt.mockCalls[callIdx]
				callIdx++
				return result.output, result.err
			})

			err := tracker.AddSubIssue(tt.parentRef, tt.subRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddSubIssue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGitHubTracker_RemoveSubIssue(t *testing.T) {
	tests := []struct {
		name      string
		parentRef IssueRef
		subRef    IssueRef
		mockCalls []struct {
			output []byte
			err    error
		}
		wantErr bool
	}{
		{
			name:      "successful with node IDs",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte(`{"data": {"removeSubIssue": {"issue": {"number": 1}}}}`), err: nil},
			},
			wantErr: false,
		},
		{
			name:      "successful with issue numbers",
			parentRef: IssueRef{Number: 1},
			subRef:    IssueRef{Number: 2},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				// First call gets parent node ID
				{output: []byte(`{"id": "parent-node-id"}`), err: nil},
				// Second call gets sub-issue node ID
				{output: []byte(`{"id": "sub-node-id"}`), err: nil},
				// Third call removes sub-issue
				{output: []byte(`{"data": {"removeSubIssue": {"issue": {"number": 1}}}}`), err: nil},
			},
			wantErr: false,
		},
		{
			name:      "graphql error",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte(`{"errors": [{"message": "Sub-issue not found"}]}`), err: nil},
			},
			wantErr: true,
		},
		{
			name:      "parent node ID fetch failure",
			parentRef: IssueRef{Number: 999},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte("issue not found"), err: fmt.Errorf("exit status 1")},
			},
			wantErr: true,
		},
		{
			name:      "malformed JSON response",
			parentRef: IssueRef{ID: "parent-node-id"},
			subRef:    IssueRef{ID: "sub-node-id"},
			mockCalls: []struct {
				output []byte
				err    error
			}{
				{output: []byte("not valid json"), err: nil},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callIdx := 0
			tracker := NewGitHubTrackerWithExecutor(func(name string, args ...string) ([]byte, error) {
				if callIdx >= len(tt.mockCalls) {
					t.Fatalf("unexpected call %d to executor", callIdx)
				}
				result := tt.mockCalls[callIdx]
				callIdx++
				return result.output, result.err
			})

			err := tracker.RemoveSubIssue(tt.parentRef, tt.subRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveSubIssue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGitHubTracker_GetIssueNodeID(t *testing.T) {
	tests := []struct {
		name       string
		issueNum   int
		mockOutput []byte
		mockError  error
		wantID     string
		wantErr    bool
	}{
		{
			name:       "successful",
			issueNum:   42,
			mockOutput: []byte(`{"id": "I_kwDOABC123"}`),
			mockError:  nil,
			wantID:     "I_kwDOABC123",
			wantErr:    false,
		},
		{
			name:       "issue not found",
			issueNum:   999,
			mockOutput: []byte("issue not found"),
			mockError:  fmt.Errorf("exit status 1"),
			wantID:     "",
			wantErr:    true,
		},
		{
			name:       "empty ID",
			issueNum:   42,
			mockOutput: []byte(`{"id": ""}`),
			mockError:  nil,
			wantID:     "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewGitHubTrackerWithExecutor(func(name string, args ...string) ([]byte, error) {
				return tt.mockOutput, tt.mockError
			})

			got, err := tracker.GetIssueNodeID(tt.issueNum)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIssueNodeID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.wantID {
				t.Errorf("GetIssueNodeID() = %v, want %v", got, tt.wantID)
			}
		})
	}
}

func TestGitHubTracker_classifyError(t *testing.T) {
	tracker := NewGitHubTracker()

	tests := []struct {
		name        string
		err         error
		output      []byte
		wantErrType error
	}{
		{
			name:        "gh not installed",
			err:         &exec.Error{Name: "gh", Err: errors.New("executable file not found")},
			output:      nil,
			wantErrType: ErrProviderUnavailable,
		},
		{
			name:        "auth required - not logged in",
			err:         fmt.Errorf("exit status 1"),
			output:      []byte("not logged in"),
			wantErrType: ErrAuthRequired,
		},
		{
			name:        "auth required - gh auth login",
			err:         fmt.Errorf("exit status 1"),
			output:      []byte("To authenticate, run: gh auth login"),
			wantErrType: ErrAuthRequired,
		},
		{
			name:        "issue not found",
			err:         fmt.Errorf("exit status 1"),
			output:      []byte("could not find issue"),
			wantErrType: ErrIssueNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tracker.classifyError(tt.err, tt.output)
			if !errors.Is(err, tt.wantErrType) {
				t.Errorf("classifyError() = %v, want %v", err, tt.wantErrType)
			}
		})
	}
}

// Ensure GitHubTracker implements IssueTracker interface
func TestGitHubTracker_ImplementsInterface(t *testing.T) {
	var _ IssueTracker = (*GitHubTracker)(nil)
}

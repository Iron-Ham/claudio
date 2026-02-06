package filelock

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

func newTestRegistry(t *testing.T, opts ...Option) (*Registry, *event.Bus) {
	t.Helper()
	mb := mailbox.NewMailbox(t.TempDir())
	bus := event.NewBus()
	return NewRegistry(mb, bus, opts...), bus
}

func TestNewRegistry(t *testing.T) {
	reg, _ := newTestRegistry(t)

	if reg.claims == nil {
		t.Fatal("claims map should be initialized")
	}
	if reg.defaultScope != ScopeFile {
		t.Errorf("default scope = %q, want %q", reg.defaultScope, ScopeFile)
	}
}

func TestNewRegistryWithScope(t *testing.T) {
	reg, _ := newTestRegistry(t, WithScope(ScopeFunction))

	if reg.defaultScope != ScopeFunction {
		t.Errorf("default scope = %q, want %q", reg.defaultScope, ScopeFunction)
	}
}

func TestClaim(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(r *Registry)
		instanceID string
		filePath   string
		wantErr    error
	}{
		{
			name:       "claim unclaimed file",
			instanceID: "inst-1",
			filePath:   "pkg/foo.go",
		},
		{
			name: "idempotent claim by same instance",
			setup: func(r *Registry) {
				r.Claim("inst-1", "pkg/foo.go") //nolint:errcheck
			},
			instanceID: "inst-1",
			filePath:   "pkg/foo.go",
		},
		{
			name: "conflict with different instance",
			setup: func(r *Registry) {
				r.Claim("inst-1", "pkg/foo.go") //nolint:errcheck
			},
			instanceID: "inst-2",
			filePath:   "pkg/foo.go",
			wantErr:    ErrAlreadyClaimed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, _ := newTestRegistry(t)
			if tt.setup != nil {
				tt.setup(reg)
			}

			err := reg.Claim(tt.instanceID, tt.filePath)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Claim() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Claim() unexpected error: %v", err)
			}

			owner, ok := reg.Owner(tt.filePath)
			if !ok {
				t.Fatal("Owner() returned false after claim")
			}
			if owner != tt.instanceID {
				t.Errorf("Owner() = %q, want %q", owner, tt.instanceID)
			}
		})
	}
}

func TestClaimPublishesEvent(t *testing.T) {
	reg, bus := newTestRegistry(t)

	ch := make(chan event.Event, 1)
	bus.Subscribe("filelock.claimed", func(e event.Event) {
		ch <- e
	})

	if err := reg.Claim("inst-1", "pkg/foo.go"); err != nil {
		t.Fatalf("Claim() error: %v", err)
	}

	select {
	case e := <-ch:
		fce, ok := e.(event.FileClaimEvent)
		if !ok {
			t.Fatalf("event type = %T, want FileClaimEvent", e)
		}
		if fce.InstanceID != "inst-1" {
			t.Errorf("event InstanceID = %q, want %q", fce.InstanceID, "inst-1")
		}
		if fce.FilePath != "pkg/foo.go" {
			t.Errorf("event FilePath = %q, want %q", fce.FilePath, "pkg/foo.go")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for FileClaimEvent")
	}
}

func TestClaimMultiple(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(r *Registry)
		files     []string
		wantErr   error
		wantOwned []string // files owned after call
	}{
		{
			name:      "claim multiple files",
			files:     []string{"a.go", "b.go", "c.go"},
			wantOwned: []string{"a.go", "b.go", "c.go"},
		},
		{
			name: "rollback on conflict",
			setup: func(r *Registry) {
				r.Claim("other", "b.go") //nolint:errcheck
			},
			files:     []string{"a.go", "b.go", "c.go"},
			wantErr:   ErrAlreadyClaimed,
			wantOwned: []string{},
		},
		{
			name:      "empty list succeeds",
			files:     []string{},
			wantOwned: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, _ := newTestRegistry(t)
			if tt.setup != nil {
				tt.setup(reg)
			}

			err := reg.ClaimMultiple("inst-1", tt.files)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("ClaimMultiple() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Fatalf("ClaimMultiple() unexpected error: %v", err)
			}

			got := reg.GetInstanceFiles("inst-1")
			if len(got) != len(tt.wantOwned) {
				t.Errorf("GetInstanceFiles() = %v, want %v", got, tt.wantOwned)
			}
		})
	}
}

func TestRelease(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(r *Registry)
		instanceID string
		filePath   string
		wantErr    error
	}{
		{
			name: "release owned file",
			setup: func(r *Registry) {
				r.Claim("inst-1", "pkg/foo.go") //nolint:errcheck
			},
			instanceID: "inst-1",
			filePath:   "pkg/foo.go",
		},
		{
			name:       "release unclaimed file",
			instanceID: "inst-1",
			filePath:   "pkg/foo.go",
			wantErr:    ErrNotClaimed,
		},
		{
			name: "release file owned by another",
			setup: func(r *Registry) {
				r.Claim("inst-1", "pkg/foo.go") //nolint:errcheck
			},
			instanceID: "inst-2",
			filePath:   "pkg/foo.go",
			wantErr:    ErrNotOwner,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, _ := newTestRegistry(t)
			if tt.setup != nil {
				tt.setup(reg)
			}

			err := reg.Release(tt.instanceID, tt.filePath)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Release() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Release() unexpected error: %v", err)
			}

			if !reg.IsAvailable(tt.filePath) {
				t.Error("file should be available after release")
			}
		})
	}
}

func TestReleasePublishesEvent(t *testing.T) {
	reg, bus := newTestRegistry(t)
	if err := reg.Claim("inst-1", "pkg/foo.go"); err != nil {
		t.Fatalf("Claim() error: %v", err)
	}

	ch := make(chan event.Event, 1)
	bus.Subscribe("filelock.released", func(e event.Event) {
		ch <- e
	})

	if err := reg.Release("inst-1", "pkg/foo.go"); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	select {
	case e := <-ch:
		fre, ok := e.(event.FileReleaseEvent)
		if !ok {
			t.Fatalf("event type = %T, want FileReleaseEvent", e)
		}
		if fre.InstanceID != "inst-1" {
			t.Errorf("event InstanceID = %q, want %q", fre.InstanceID, "inst-1")
		}
		if fre.FilePath != "pkg/foo.go" {
			t.Errorf("event FilePath = %q, want %q", fre.FilePath, "pkg/foo.go")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for FileReleaseEvent")
	}
}

func TestReleaseAll(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(r *Registry)
		instanceID string
		wantFiles  int // files remaining after release
	}{
		{
			name: "release all owned files",
			setup: func(r *Registry) {
				r.Claim("inst-1", "a.go") //nolint:errcheck
				r.Claim("inst-1", "b.go") //nolint:errcheck
				r.Claim("inst-1", "c.go") //nolint:errcheck
			},
			instanceID: "inst-1",
			wantFiles:  0,
		},
		{
			name: "release only own files",
			setup: func(r *Registry) {
				r.Claim("inst-1", "a.go") //nolint:errcheck
				r.Claim("inst-2", "b.go") //nolint:errcheck
			},
			instanceID: "inst-1",
			wantFiles:  0,
		},
		{
			name:       "no files owned is no-op",
			instanceID: "inst-1",
			wantFiles:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, _ := newTestRegistry(t)
			if tt.setup != nil {
				tt.setup(reg)
			}

			if err := reg.ReleaseAll(tt.instanceID); err != nil {
				t.Fatalf("ReleaseAll() error: %v", err)
			}

			got := reg.GetInstanceFiles(tt.instanceID)
			if len(got) != tt.wantFiles {
				t.Errorf("files remaining = %d, want %d", len(got), tt.wantFiles)
			}
		})
	}
}

func TestOwner(t *testing.T) {
	reg, _ := newTestRegistry(t)

	// Unclaimed file.
	_, ok := reg.Owner("pkg/foo.go")
	if ok {
		t.Error("Owner() returned true for unclaimed file")
	}

	// Claimed file.
	reg.Claim("inst-1", "pkg/foo.go") //nolint:errcheck
	owner, ok := reg.Owner("pkg/foo.go")
	if !ok {
		t.Fatal("Owner() returned false for claimed file")
	}
	if owner != "inst-1" {
		t.Errorf("Owner() = %q, want %q", owner, "inst-1")
	}
}

func TestIsAvailable(t *testing.T) {
	reg, _ := newTestRegistry(t)

	if !reg.IsAvailable("pkg/foo.go") {
		t.Error("IsAvailable() = false for unclaimed file")
	}

	reg.Claim("inst-1", "pkg/foo.go") //nolint:errcheck

	if reg.IsAvailable("pkg/foo.go") {
		t.Error("IsAvailable() = true for claimed file")
	}
}

func TestGetInstanceFiles(t *testing.T) {
	reg, _ := newTestRegistry(t)

	// No files.
	got := reg.GetInstanceFiles("inst-1")
	if len(got) != 0 {
		t.Errorf("GetInstanceFiles() = %v, want empty", got)
	}

	// Multiple files, sorted.
	reg.Claim("inst-1", "c.go") //nolint:errcheck
	reg.Claim("inst-1", "a.go") //nolint:errcheck
	reg.Claim("inst-1", "b.go") //nolint:errcheck

	got = reg.GetInstanceFiles("inst-1")
	want := []string{"a.go", "b.go", "c.go"}
	if len(got) != len(want) {
		t.Fatalf("GetInstanceFiles() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("GetInstanceFiles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGetInstanceFilesExcludesOtherInstances(t *testing.T) {
	reg, _ := newTestRegistry(t)
	reg.Claim("inst-1", "a.go") //nolint:errcheck
	reg.Claim("inst-2", "b.go") //nolint:errcheck

	got := reg.GetInstanceFiles("inst-1")
	if len(got) != 1 || got[0] != "a.go" {
		t.Errorf("GetInstanceFiles() = %v, want [a.go]", got)
	}
}

func TestWatchClaims(t *testing.T) {
	reg, _ := newTestRegistry(t)

	var received []FileClaim
	reg.WatchClaims(func(c FileClaim) {
		received = append(received, c)
	})

	reg.Claim("inst-1", "a.go") //nolint:errcheck
	reg.Claim("inst-1", "b.go") //nolint:errcheck

	if len(received) != 2 {
		t.Fatalf("WatchClaims received %d, want 2", len(received))
	}
	if received[0].FilePath != "a.go" {
		t.Errorf("first claim path = %q, want %q", received[0].FilePath, "a.go")
	}
	if received[1].FilePath != "b.go" {
		t.Errorf("second claim path = %q, want %q", received[1].FilePath, "b.go")
	}
}

func TestConcurrentClaims(t *testing.T) {
	reg, _ := newTestRegistry(t)
	const goroutines = 10

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	// All goroutines try to claim the same file.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := reg.Claim(fmt.Sprintf("inst-%d", id), "contested.go")
			if err != nil && !errors.Is(err, ErrAlreadyClaimed) {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("unexpected error: %v", err)
	}

	// Exactly one owner.
	owner, ok := reg.Owner("contested.go")
	if !ok {
		t.Fatal("Owner() returned false after concurrent claims")
	}
	if owner == "" {
		t.Error("Owner() returned empty string")
	}
}

func TestConcurrentClaimAndRelease(t *testing.T) {
	reg, _ := newTestRegistry(t)

	const iterations = 50
	var wg sync.WaitGroup

	// Two goroutines: one claiming, one releasing.
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			reg.Claim("inst-1", fmt.Sprintf("file-%d.go", i)) //nolint:errcheck
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			reg.Release("inst-1", fmt.Sprintf("file-%d.go", i)) //nolint:errcheck
		}
	}()

	wg.Wait()
	// No panic or data race is the success condition.
}

func TestMailboxBroadcast(t *testing.T) {
	dir := t.TempDir()
	mb := mailbox.NewMailbox(dir)
	bus := event.NewBus()
	reg := NewRegistry(mb, bus)

	if err := reg.Claim("inst-1", "pkg/foo.go"); err != nil {
		t.Fatalf("Claim() error: %v", err)
	}

	// Check that the mailbox received the broadcast message.
	msgs, err := mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("expected at least one broadcast message")
	}

	found := false
	for _, msg := range msgs {
		if msg.Type == mailbox.MessageClaim && msg.From == "inst-1" {
			found = true
			if msg.Metadata["path"] != "pkg/foo.go" {
				t.Errorf("metadata path = %v, want %q", msg.Metadata["path"], "pkg/foo.go")
			}
			if msg.Metadata["scope"] != string(ScopeFile) {
				t.Errorf("metadata scope = %v, want %q", msg.Metadata["scope"], ScopeFile)
			}
		}
	}
	if !found {
		t.Error("claim broadcast message not found")
	}

	// Release and verify release broadcast.
	if err := reg.Release("inst-1", "pkg/foo.go"); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	msgs, err = mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error: %v", err)
	}

	foundRelease := false
	for _, msg := range msgs {
		if msg.Type == mailbox.MessageRelease && msg.From == "inst-1" {
			foundRelease = true
		}
	}
	if !foundRelease {
		t.Error("release broadcast message not found")
	}
}

func TestClaimScopeOption(t *testing.T) {
	dir := t.TempDir()
	mb := mailbox.NewMailbox(dir)
	bus := event.NewBus()
	reg := NewRegistry(mb, bus, WithScope(ScopeFunction))

	if err := reg.Claim("inst-1", "pkg/foo.go"); err != nil {
		t.Fatalf("Claim() error: %v", err)
	}

	// Verify scope in mailbox metadata.
	msgs, err := mb.Receive("inst-2")
	if err != nil {
		t.Fatalf("Receive() error: %v", err)
	}

	for _, msg := range msgs {
		if msg.Type == mailbox.MessageClaim {
			if msg.Metadata["scope"] != string(ScopeFunction) {
				t.Errorf("metadata scope = %v, want %q", msg.Metadata["scope"], ScopeFunction)
			}
		}
	}
}

func TestFileClaim_Fields(t *testing.T) {
	now := time.Now()
	claim := FileClaim{
		InstanceID: "inst-1",
		FilePath:   "pkg/foo.go",
		ClaimedAt:  now,
		Scope:      ScopeFile,
	}

	if claim.InstanceID != "inst-1" {
		t.Errorf("InstanceID = %q, want %q", claim.InstanceID, "inst-1")
	}
	if claim.FilePath != "pkg/foo.go" {
		t.Errorf("FilePath = %q, want %q", claim.FilePath, "pkg/foo.go")
	}
	if !claim.ClaimedAt.Equal(now) {
		t.Errorf("ClaimedAt = %v, want %v", claim.ClaimedAt, now)
	}
	if claim.Scope != ScopeFile {
		t.Errorf("Scope = %q, want %q", claim.Scope, ScopeFile)
	}
}

// Compile-time interface checks.
var (
	_ event.Event = event.FileClaimEvent{}
	_ event.Event = event.FileReleaseEvent{}
)

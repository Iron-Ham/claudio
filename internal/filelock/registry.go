package filelock

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

// Registry manages advisory file ownership claims across instances.
// It maintains an in-memory map of file path to owner, broadcasts
// claims/releases via the mailbox, and publishes events to the bus.
type Registry struct {
	mu           sync.RWMutex
	claims       map[string]FileClaim // filePath -> claim
	mb           *mailbox.Mailbox
	bus          *event.Bus
	defaultScope ClaimScope
	handlers     []func(FileClaim)
}

// NewRegistry creates a Registry backed by the given mailbox and event bus.
func NewRegistry(mb *mailbox.Mailbox, bus *event.Bus, opts ...Option) *Registry {
	r := &Registry{
		claims:       make(map[string]FileClaim),
		mb:           mb,
		bus:          bus,
		defaultScope: ScopeFile,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Claim registers ownership of a file for the given instance.
// Returns ErrAlreadyClaimed if the file is owned by a different instance.
// If the instance already owns the file, this is a no-op.
func (r *Registry) Claim(instanceID, filePath string) error {
	r.mu.Lock()
	claim, err := r.claimLocked(instanceID, filePath)
	r.mu.Unlock()

	if err != nil {
		return err
	}
	if claim != nil {
		r.bus.Publish(event.NewFileClaimEvent(instanceID, filePath))
		r.notifyHandlersUnlocked(*claim)
	}
	return nil
}

// claimLocked performs a single claim while the write lock is held.
// Returns the new claim for post-lock event publishing, or nil for idempotent no-ops.
func (r *Registry) claimLocked(instanceID, filePath string) (*FileClaim, error) {
	if existing, ok := r.claims[filePath]; ok {
		if existing.InstanceID == instanceID {
			return nil, nil // idempotent
		}
		return nil, fmt.Errorf("%w: %s owns %s", ErrAlreadyClaimed, existing.InstanceID, filePath)
	}

	if err := r.broadcastClaim(instanceID, filePath); err != nil {
		return nil, fmt.Errorf("broadcast claim: %w", err)
	}

	claim := FileClaim{
		InstanceID: instanceID,
		FilePath:   filePath,
		ClaimedAt:  time.Now(),
		Scope:      r.defaultScope,
	}
	r.claims[filePath] = claim
	return &claim, nil
}

// ClaimMultiple registers ownership of multiple files for the given instance.
// It claims files atomically: if any claim fails, previously claimed files
// in this batch are rolled back.
func (r *Registry) ClaimMultiple(instanceID string, filePaths []string) error {
	r.mu.Lock()

	var newClaims []FileClaim
	var claimedPaths []string
	for _, fp := range filePaths {
		claim, err := r.claimLocked(instanceID, fp)
		if err != nil {
			// Roll back claims made in this batch
			for _, c := range claimedPaths {
				_, _ = r.releaseLocked(instanceID, c) // best-effort rollback
			}
			r.mu.Unlock()
			return err
		}
		if claim != nil {
			newClaims = append(newClaims, *claim)
		}
		claimedPaths = append(claimedPaths, fp)
	}
	r.mu.Unlock()

	// Publish events outside the lock.
	for _, claim := range newClaims {
		r.bus.Publish(event.NewFileClaimEvent(claim.InstanceID, claim.FilePath))
		r.notifyHandlersUnlocked(claim)
	}
	return nil
}

// Release relinquishes ownership of a file for the given instance.
// Returns ErrNotClaimed if the file is not claimed, or ErrNotOwner
// if the file is claimed by a different instance.
func (r *Registry) Release(instanceID, filePath string) error {
	r.mu.Lock()
	released, err := r.releaseLocked(instanceID, filePath)
	r.mu.Unlock()

	if err != nil {
		return err
	}
	if released {
		r.bus.Publish(event.NewFileReleaseEvent(instanceID, filePath))
	}
	return nil
}

// releaseLocked performs a single release while the write lock is held.
// Returns true if the file was successfully released.
func (r *Registry) releaseLocked(instanceID, filePath string) (bool, error) {
	existing, ok := r.claims[filePath]
	if !ok {
		return false, fmt.Errorf("%w: %s", ErrNotClaimed, filePath)
	}
	if existing.InstanceID != instanceID {
		return false, fmt.Errorf("%w: %s owns %s", ErrNotOwner, existing.InstanceID, filePath)
	}

	if err := r.broadcastRelease(instanceID, filePath); err != nil {
		return false, fmt.Errorf("broadcast release: %w", err)
	}

	delete(r.claims, filePath)
	return true, nil
}

// ReleaseAll relinquishes all files owned by the given instance.
// Returns nil if the instance owns no files.
func (r *Registry) ReleaseAll(instanceID string) error {
	r.mu.Lock()

	var paths []string
	for fp, claim := range r.claims {
		if claim.InstanceID == instanceID {
			paths = append(paths, fp)
		}
	}
	// Sort for deterministic broadcast order.
	sort.Strings(paths)

	var released []string
	for _, fp := range paths {
		ok, err := r.releaseLocked(instanceID, fp)
		if err != nil {
			r.mu.Unlock()
			return err
		}
		if ok {
			released = append(released, fp)
		}
	}
	r.mu.Unlock()

	// Publish events outside the lock.
	for _, fp := range released {
		r.bus.Publish(event.NewFileReleaseEvent(instanceID, fp))
	}
	return nil
}

// Owner returns the instance ID that owns the file and true,
// or ("", false) if the file is unclaimed.
func (r *Registry) Owner(filePath string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	claim, ok := r.claims[filePath]
	if !ok {
		return "", false
	}
	return claim.InstanceID, true
}

// IsAvailable returns true if the file is not claimed by any instance.
func (r *Registry) IsAvailable(filePath string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.claims[filePath]
	return !ok
}

// GetInstanceFiles returns all file paths claimed by the given instance.
// The returned slice is sorted alphabetically for deterministic output.
func (r *Registry) GetInstanceFiles(instanceID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var files []string
	for fp, claim := range r.claims {
		if claim.InstanceID == instanceID {
			files = append(files, fp)
		}
	}
	sort.Strings(files)
	return files
}

// WatchClaims registers a handler that is called whenever a claim is established.
// Handlers are called outside the registry's lock; they may safely call read
// methods like Owner, IsAvailable, and GetInstanceFiles without deadlocking.
func (r *Registry) WatchClaims(handler func(FileClaim)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.handlers = append(r.handlers, handler)
}

// notifyHandlersUnlocked calls all registered claim handlers.
// Must be called outside the write lock to avoid deadlock if handlers
// call back into the registry.
func (r *Registry) notifyHandlersUnlocked(claim FileClaim) {
	r.mu.RLock()
	handlers := make([]func(FileClaim), len(r.handlers))
	copy(handlers, r.handlers)
	r.mu.RUnlock()

	for _, h := range handlers {
		h(claim)
	}
}

// broadcastClaim sends a claim message via the mailbox.
func (r *Registry) broadcastClaim(instanceID, filePath string) error {
	msg := mailbox.Message{
		From: instanceID,
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageClaim,
		Body: filePath,
		Metadata: map[string]any{
			"path":  filePath,
			"scope": string(r.defaultScope),
		},
	}
	return r.mb.Send(msg)
}

// broadcastRelease sends a release message via the mailbox.
func (r *Registry) broadcastRelease(instanceID, filePath string) error {
	msg := mailbox.Message{
		From: instanceID,
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageRelease,
		Body: filePath,
		Metadata: map[string]any{
			"path":  filePath,
			"scope": string(r.defaultScope),
		},
	}
	return r.mb.Send(msg)
}

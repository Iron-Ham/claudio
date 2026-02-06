package filelock

import (
	"errors"
	"time"
)

// Sentinel errors returned by registry operations.
var (
	// ErrAlreadyClaimed is returned when a file is already claimed by another instance.
	ErrAlreadyClaimed = errors.New("file already claimed by another instance")

	// ErrNotOwner is returned when an instance tries to release a file it does not own.
	ErrNotOwner = errors.New("instance does not own this file")

	// ErrNotClaimed is returned when an instance tries to release an unclaimed file.
	ErrNotClaimed = errors.New("file is not claimed")
)

// ClaimScope defines the granularity of a file claim.
type ClaimScope string

const (
	// ScopeFile claims the entire file.
	ScopeFile ClaimScope = "file"

	// ScopeFunction claims a specific function within a file (advisory).
	ScopeFunction ClaimScope = "function"
)

// FileClaim represents an ownership claim on a file path.
type FileClaim struct {
	InstanceID string     // Instance that owns the claim
	FilePath   string     // Path to the claimed file
	ClaimedAt  time.Time  // When the claim was established
	Scope      ClaimScope // Granularity of the claim
}

// Option configures a Registry.
type Option func(*Registry)

// WithScope sets the default claim scope for new claims.
func WithScope(scope ClaimScope) Option {
	return func(r *Registry) {
		r.defaultScope = scope
	}
}

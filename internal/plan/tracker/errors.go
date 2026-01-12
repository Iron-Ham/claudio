package tracker

import "errors"

// Sentinel errors for issue tracker operations.
var (
	// ErrHierarchyNotSupported indicates that the provider does not support
	// parent-child issue relationships.
	ErrHierarchyNotSupported = errors.New("issue hierarchy not supported by this provider")

	// ErrLabelsNotSupported indicates that the provider does not support
	// issue labels/tags.
	ErrLabelsNotSupported = errors.New("labels not supported by this provider")

	// ErrIssueNotFound indicates that the requested issue does not exist.
	ErrIssueNotFound = errors.New("issue not found")

	// ErrAuthRequired indicates that authentication is required.
	ErrAuthRequired = errors.New("authentication required")

	// ErrProviderUnavailable indicates that the provider tool/API is not available.
	ErrProviderUnavailable = errors.New("provider unavailable")
)

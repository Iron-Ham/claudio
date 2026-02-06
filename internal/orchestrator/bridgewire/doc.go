// Package bridgewire creates bridge adapters that connect the orchestrator
// infrastructure (worktrees, tmux, verification) to the bridge interfaces.
//
// This package exists as a separate package to break the import cycle between
// the orchestrator and bridge packages. It imports both and provides adapter
// types that translate between their APIs.
//
// Usage:
//
//	factory := bridgewire.NewInstanceFactory(orch, session)
//	checker := bridgewire.NewCompletionChecker(verifier)
//	recorder := bridgewire.NewSessionRecorder(bridgewire.SessionRecorderDeps{
//		OnAssign:   func(taskID, instanceID string) { /* ... */ },
//		OnComplete: func(taskID string, commitCount int) { /* ... */ },
//		OnFailure:  func(taskID, reason string) { /* ... */ },
//	})
package bridgewire

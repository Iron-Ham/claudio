package tripleshot

// Runner is the common interface satisfied by both *Coordinator (legacy polling)
// and *teamwire.TeamCoordinator (Orch 2.0 callback-driven). The TUI uses this
// interface so it can work with either execution path without type assertions.
type Runner interface {
	Session() *Session
	SetCallbacks(cb *CoordinatorCallbacks)
	GetWinningBranch() string
	Stop()
}

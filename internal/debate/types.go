package debate

// SessionStatus represents the current state of a debate session.
type SessionStatus string

const (
	// StatusPending indicates the debate has been created but no messages exchanged.
	StatusPending SessionStatus = "pending"

	// StatusActive indicates at least one challenge has been issued.
	StatusActive SessionStatus = "active"

	// StatusResolved indicates a participant has declared consensus.
	StatusResolved SessionStatus = "resolved"
)

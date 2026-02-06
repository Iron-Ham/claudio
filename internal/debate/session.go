package debate

import (
	"fmt"
	"sync"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

// Session manages a structured debate between two instances.
// Messages are exchanged through the mailbox using targeted delivery.
type Session struct {
	mu        sync.Mutex
	id        string
	mb        *mailbox.Mailbox
	bus       *event.Bus
	instanceA string
	instanceB string
	topic     string
	status    SessionStatus
	messages  []mailbox.Message
	rounds    int // number of complete challenge-defense pairs
}

// NewSession creates a debate session between two instances on a given topic.
// The session starts in Pending status. A DebateStartedEvent is published
// to the event bus.
func NewSession(mb *mailbox.Mailbox, bus *event.Bus, instanceA, instanceB, topic string) *Session {
	s := &Session{
		id:        generateDebateID(instanceA, instanceB),
		mb:        mb,
		bus:       bus,
		instanceA: instanceA,
		instanceB: instanceB,
		topic:     topic,
		status:    StatusPending,
	}

	if bus != nil {
		bus.Publish(event.NewDebateStartedEvent(s.id, instanceA, instanceB, topic))
	}

	return s
}

// ID returns the debate session identifier.
func (s *Session) ID() string {
	return s.id
}

// Topic returns the debate topic.
func (s *Session) Topic() string {
	return s.topic
}

// Status returns the current session status.
func (s *Session) Status() SessionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// Challenge sends a challenge message from one participant to the other.
// The session must not be resolved. If this is the first message, the
// session transitions from Pending to Active.
func (s *Session) Challenge(from, body string, metadata map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusResolved {
		return fmt.Errorf("debate: session already resolved")
	}

	to, err := s.opponent(from)
	if err != nil {
		return err
	}

	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["debate_id"] = s.id
	metadata["round"] = s.rounds + 1

	msg := mailbox.Message{
		From:     from,
		To:       to,
		Type:     mailbox.MessageChallenge,
		Body:     body,
		Metadata: metadata,
	}

	if err := s.mb.Send(msg); err != nil {
		return fmt.Errorf("debate: send challenge: %w", err)
	}

	s.messages = append(s.messages, msg)
	s.status = StatusActive
	return nil
}

// Defend sends a defense message from one participant to the other.
// The session must be active (at least one challenge must have been issued).
func (s *Session) Defend(from, body string, metadata map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusPending {
		return fmt.Errorf("debate: cannot defend before a challenge is issued")
	}
	if s.status == StatusResolved {
		return fmt.Errorf("debate: session already resolved")
	}

	to, err := s.opponent(from)
	if err != nil {
		return err
	}

	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["debate_id"] = s.id
	metadata["round"] = s.rounds + 1

	msg := mailbox.Message{
		From:     from,
		To:       to,
		Type:     mailbox.MessageDefense,
		Body:     body,
		Metadata: metadata,
	}

	if err := s.mb.Send(msg); err != nil {
		return fmt.Errorf("debate: send defense: %w", err)
	}

	s.messages = append(s.messages, msg)
	s.rounds++
	return nil
}

// Resolve declares consensus and resolves the debate. The session must be
// active. A DebateResolvedEvent is published to the event bus.
func (s *Session) Resolve(from, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == StatusPending {
		return fmt.Errorf("debate: cannot resolve before a challenge is issued")
	}
	if s.status == StatusResolved {
		return fmt.Errorf("debate: session already resolved")
	}

	to, err := s.opponent(from)
	if err != nil {
		return err
	}

	msg := mailbox.Message{
		From: from,
		To:   to,
		Type: mailbox.MessageConsensus,
		Body: body,
		Metadata: map[string]any{
			"debate_id": s.id,
		},
	}

	if err := s.mb.Send(msg); err != nil {
		return fmt.Errorf("debate: send consensus: %w", err)
	}

	s.messages = append(s.messages, msg)
	s.status = StatusResolved

	if s.bus != nil {
		s.bus.Publish(event.NewDebateResolvedEvent(s.id, body, s.rounds))
	}

	return nil
}

// Messages returns a chronological copy of all messages in the debate.
func (s *Session) Messages() []mailbox.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]mailbox.Message, len(s.messages))
	copy(result, s.messages)
	return result
}

// Rounds returns the number of complete challenge-defense pairs.
func (s *Session) Rounds() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rounds
}

// opponent returns the other participant given one participant's ID.
func (s *Session) opponent(from string) (string, error) {
	switch from {
	case s.instanceA:
		return s.instanceB, nil
	case s.instanceB:
		return s.instanceA, nil
	default:
		return "", fmt.Errorf("debate: %q is not a participant in this debate", from)
	}
}

// generateDebateID creates a deterministic debate ID from the two participants.
func generateDebateID(a, b string) string {
	return fmt.Sprintf("debate-%s-%s", a, b)
}

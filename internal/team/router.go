package team

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

// Router delivers inter-team messages via each team's Hub mailbox.
// Targeted messages go to the specified team; broadcast messages go to all
// teams except the sender.
type Router struct {
	mu       sync.RWMutex
	bus      *event.Bus
	teams    func(id string) *Team // lookup function from manager
	allTeams func() []string       // returns all team IDs in order
	messages []InterTeamMessage    // message log
	counter  atomic.Uint64         // for ID generation
}

// newRouter creates a Router with the given event bus and team lookup functions.
func newRouter(bus *event.Bus, teamLookup func(string) *Team, allTeams func() []string) *Router {
	return &Router{
		bus:      bus,
		teams:    teamLookup,
		allTeams: allTeams,
	}
}

// Route delivers an inter-team message to the appropriate team(s).
// For targeted messages (ToTeam is a specific team ID), the message is
// delivered to that team's Hub mailbox. For broadcast messages (ToTeam is
// BroadcastRecipient), the message is delivered to all teams except the sender.
//
// Each delivery publishes an InterTeamMessageEvent on the event bus.
func (r *Router) Route(msg InterTeamMessage) error {
	if msg.FromTeam == "" {
		return errors.New("router: FromTeam is required")
	}
	if msg.ToTeam == "" {
		return errors.New("router: ToTeam is required")
	}

	// Assign ID and timestamp if not set.
	if msg.ID == "" {
		msg.ID = fmt.Sprintf("itm-%d-%d", time.Now().UnixNano(), r.counter.Add(1))
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Record in message log.
	r.mu.Lock()
	r.messages = append(r.messages, msg)
	r.mu.Unlock()

	if msg.IsBroadcast() {
		return r.broadcast(msg)
	}
	return r.targeted(msg)
}

// targeted delivers a message to a single team.
func (r *Router) targeted(msg InterTeamMessage) error {
	t := r.teams(msg.ToTeam)
	if t == nil {
		return fmt.Errorf("router: target team %q not found", msg.ToTeam)
	}

	r.deliverToTeam(t, msg)
	return nil
}

// broadcast delivers a message to all teams except the sender.
func (r *Router) broadcast(msg InterTeamMessage) error {
	ids := r.allTeams()
	for _, id := range ids {
		if id == msg.FromTeam {
			continue
		}
		t := r.teams(id)
		if t == nil {
			continue
		}
		r.deliverToTeam(t, msg)
	}
	return nil
}

// deliverToTeam sends a message to a team's Hub mailbox and publishes an event.
func (r *Router) deliverToTeam(t *Team, msg InterTeamMessage) {
	mb := t.Hub().Mailbox()

	mbMsg := mailbox.Message{
		From: fmt.Sprintf("team:%s", msg.FromTeam),
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageType(msg.Type),
		Body: fmt.Sprintf("[%s] %s", msg.Priority, msg.Content),
	}

	// Best-effort delivery — errors are ignored so that a single failed
	// mailbox send does not prevent delivery to other teams in a broadcast.
	_ = mb.Send(mbMsg)

	r.bus.Publish(event.NewInterTeamMessageEvent(
		msg.FromTeam,
		t.Spec().ID,
		string(msg.Type),
		msg.Content,
		string(msg.Priority),
	))
}

// Messages returns a copy of all routed messages.
func (r *Router) Messages() []InterTeamMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]InterTeamMessage, len(r.messages))
	copy(out, r.messages)
	return out
}

// MessagesForTeam returns all messages sent to or from a specific team.
func (r *Router) MessagesForTeam(teamID string) []InterTeamMessage {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []InterTeamMessage
	for _, msg := range r.messages {
		if msg.FromTeam == teamID || msg.ToTeam == teamID || msg.IsBroadcast() {
			out = append(out, msg)
		}
	}
	return out
}

package mailbox

import "github.com/Iron-Ham/claudio/internal/event"

// Option configures a Mailbox.
type Option func(*Mailbox)

// WithBus attaches an event bus to the Mailbox. When set, a
// MailboxMessageEvent is published after every successful Send.
func WithBus(bus *event.Bus) Option {
	return func(m *Mailbox) {
		m.bus = bus
	}
}

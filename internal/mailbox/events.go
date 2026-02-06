package mailbox

import "github.com/Iron-Ham/claudio/internal/event"

// NewMailboxMessageEvent creates an event.MailboxMessageEvent from a Message.
func NewMailboxMessageEvent(msg Message) event.MailboxMessageEvent {
	return event.NewMailboxMessageEvent(msg.From, msg.To, string(msg.Type), msg.Body)
}

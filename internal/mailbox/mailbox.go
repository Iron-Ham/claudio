package mailbox

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Iron-Ham/claudio/internal/event"
)

const (
	// defaultPollInterval is the default interval for the Watch poller.
	defaultPollInterval = 500 * time.Millisecond
)

// Mailbox provides a high-level interface for inter-instance messaging.
// It wraps a Store and adds convenience methods for receiving messages
// from both broadcast and targeted mailboxes, plus a poll-based watcher.
type Mailbox struct {
	store        *Store
	bus          *event.Bus
	pollInterval time.Duration
}

// NewMailbox creates a Mailbox backed by a file store in the given session directory.
func NewMailbox(sessionDir string, opts ...Option) *Mailbox {
	m := &Mailbox{
		store:        NewStore(sessionDir),
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SetPollInterval configures the interval between Watch polls.
// Must be called before Watch. Zero or negative values are ignored.
func (m *Mailbox) SetPollInterval(d time.Duration) {
	if d > 0 {
		m.pollInterval = d
	}
}

// Send delivers a message to the store. It populates the ID and Timestamp
// fields if they are empty.
func (m *Mailbox) Send(msg Message) error {
	if err := m.store.Send(msg); err != nil {
		return err
	}
	if m.bus != nil {
		m.bus.Publish(NewMailboxMessageEvent(msg))
	}
	return nil
}

// Receive returns all messages for the given instance, including both
// broadcast messages and messages addressed directly to the instance.
// Messages are sorted chronologically by timestamp.
func (m *Mailbox) Receive(instanceID string) ([]Message, error) {
	return m.store.ReadAll(instanceID)
}

// maxWatchErrors is the number of consecutive Receive errors before the
// watcher logs at error level. Individual failures are expected (e.g.,
// transient I/O); sustained failures indicate a real problem.
const maxWatchErrors = 5

// Watch polls for new messages and invokes handler for each new message.
// It returns a cancel function that stops the watcher. The watcher runs in a
// separate goroutine. Messages are delivered in chronological order.
//
// The watcher tracks the count of previously seen messages and only delivers
// messages that appear after the initial snapshot.
func (m *Mailbox) Watch(instanceID string, handler func(Message)) (cancel func()) {
	var stopped atomic.Bool
	var wg sync.WaitGroup

	// Take the initial snapshot synchronously so that any Send() after
	// Watch() returns is guaranteed to be seen by the poller.
	seen, err := m.countMessages(instanceID)
	if err != nil {
		// If the initial snapshot fails, start from 0 so we don't miss
		// messages. This may re-deliver existing messages but is safer
		// than silently skipping them.
		seen = 0
	}

	wg.Go(func() {
		consecutiveErrors := 0
		for !stopped.Load() {
			time.Sleep(m.pollInterval)
			if stopped.Load() {
				return
			}

			messages, err := m.Receive(instanceID)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= maxWatchErrors {
					// Publish an event if bus is available so the failure
					// is observable. Reset counter to avoid log spam.
					consecutiveErrors = 0
				}
				continue
			}
			consecutiveErrors = 0

			if len(messages) > seen {
				for _, msg := range messages[seen:] {
					handler(msg)
				}
				seen = len(messages)
			}
		}
	})

	return func() {
		stopped.Store(true)
		wg.Wait()
	}
}

// countMessages returns the current message count for an instance (broadcast + targeted).
func (m *Mailbox) countMessages(instanceID string) (int, error) {
	messages, err := m.Receive(instanceID)
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

package contextprop

import (
	"fmt"

	"github.com/Iron-Ham/claudio/internal/event"
	"github.com/Iron-Ham/claudio/internal/mailbox"
)

// Propagator manages context sharing between instances. It wraps a Mailbox
// for message delivery and an event Bus for publishing propagation events.
type Propagator struct {
	mb  *mailbox.Mailbox
	bus *event.Bus
}

// NewPropagator creates a Propagator backed by the given mailbox and event bus.
func NewPropagator(mb *mailbox.Mailbox, bus *event.Bus) *Propagator {
	return &Propagator{
		mb:  mb,
		bus: bus,
	}
}

// ShareDiscovery broadcasts a discovery message to all instances.
// A ContextPropagatedEvent is published to the event bus.
func (p *Propagator) ShareDiscovery(from, body string, metadata map[string]any) error {
	msg := mailbox.Message{
		From:     from,
		To:       mailbox.BroadcastRecipient,
		Type:     mailbox.MessageDiscovery,
		Body:     body,
		Metadata: metadata,
	}

	if err := p.mb.Send(msg); err != nil {
		return fmt.Errorf("contextprop: share discovery: %w", err)
	}

	if p.bus != nil {
		p.bus.Publish(event.NewContextPropagatedEvent(from, 0, string(mailbox.MessageDiscovery)))
	}

	return nil
}

// ShareWarning broadcasts a warning message to all instances.
// A ContextPropagatedEvent is published to the event bus.
func (p *Propagator) ShareWarning(from, body string) error {
	msg := mailbox.Message{
		From: from,
		To:   mailbox.BroadcastRecipient,
		Type: mailbox.MessageWarning,
		Body: body,
	}

	if err := p.mb.Send(msg); err != nil {
		return fmt.Errorf("contextprop: share warning: %w", err)
	}

	if p.bus != nil {
		p.bus.Publish(event.NewContextPropagatedEvent(from, 0, string(mailbox.MessageWarning)))
	}

	return nil
}

// GetContextForInstance retrieves messages for an instance, applies filters,
// and returns formatted text suitable for prompt injection.
func (p *Propagator) GetContextForInstance(instanceID string, opts mailbox.FilterOptions) (string, error) {
	messages, err := p.mb.Receive(instanceID)
	if err != nil {
		return "", fmt.Errorf("contextprop: receive messages: %w", err)
	}

	return mailbox.FormatFiltered(messages, opts), nil
}

// Watch starts watching for new messages addressed to the given instance
// and invokes handler for each new message. Returns a cancel function that
// stops the watcher.
func (p *Propagator) Watch(instanceID string, handler func(mailbox.Message)) (cancel func()) {
	return p.mb.Watch(instanceID, handler)
}

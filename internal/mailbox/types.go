package mailbox

import "time"

// MessageType identifies the kind of inter-instance message.
type MessageType string

const (
	// MessageDiscovery shares findings with other instances.
	MessageDiscovery MessageType = "discovery"

	// MessageClaim declares ownership of a file or module.
	MessageClaim MessageType = "claim"

	// MessageRelease relinquishes previously claimed ownership.
	MessageRelease MessageType = "release"

	// MessageWarning alerts other instances about potential issues.
	MessageWarning MessageType = "warning"

	// MessageQuestion requests help from other instances.
	MessageQuestion MessageType = "question"

	// MessageAnswer responds to a question from another instance.
	MessageAnswer MessageType = "answer"

	// MessageStatus provides a progress update.
	MessageStatus MessageType = "status"

	// MessageChallenge disagrees with an approach and presents an alternative.
	MessageChallenge MessageType = "challenge"

	// MessageDefense provides evidence supporting an approach.
	MessageDefense MessageType = "defense"

	// MessageConsensus agrees with a resolution.
	MessageConsensus MessageType = "consensus"
)

// BroadcastRecipient is the special "to" value for messages intended for all instances.
const BroadcastRecipient = "broadcast"

// Message represents a single inter-instance communication.
type Message struct {
	ID        string         `json:"id"`
	From      string         `json:"from"`
	To        string         `json:"to"`
	Type      MessageType    `json:"type"`
	Body      string         `json:"body"`
	Timestamp time.Time      `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// IsBroadcast returns true if the message is addressed to all instances.
func (m Message) IsBroadcast() bool {
	return m.To == BroadcastRecipient
}

// Valid message types for validation.
var validMessageTypes = map[MessageType]bool{
	MessageDiscovery: true,
	MessageClaim:     true,
	MessageRelease:   true,
	MessageWarning:   true,
	MessageQuestion:  true,
	MessageAnswer:    true,
	MessageStatus:    true,
	MessageChallenge: true,
	MessageDefense:   true,
	MessageConsensus: true,
}

// ValidateMessageType returns true if the given type is a known message type.
func ValidateMessageType(t MessageType) bool {
	return validMessageTypes[t]
}

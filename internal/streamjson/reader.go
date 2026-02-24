package streamjson

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Reader parses NDJSON events from a Claude Code stream-json output stream.
// Each call to Next reads one line and returns a typed Event.
type Reader struct {
	scanner *bufio.Scanner
}

// NewReader creates a Reader that parses NDJSON from the given io.Reader.
// The reader is typically connected to the stdout of a `claude -p --output-format stream-json` process.
func NewReader(r io.Reader) *Reader {
	scanner := bufio.NewScanner(r)
	// Allow large lines for tool use events with big inputs
	scanner.Buffer(make([]byte, 0, 256*1024), 10*1024*1024) // 10MB max
	return &Reader{scanner: scanner}
}

// Next reads the next event from the stream. Returns io.EOF when the stream
// ends. Blank lines are silently skipped.
func (r *Reader) Next() (Event, error) {
	for r.scanner.Scan() {
		line := r.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		return parseEvent(line)
	}
	if err := r.scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream read error: %w", err)
	}
	return nil, io.EOF
}

// parseEvent deserializes a JSON line into a typed Event.
func parseEvent(data []byte) (Event, error) {
	// First pass: determine the event type
	var raw RawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse event type: %w", err)
	}

	// Second pass: deserialize into the specific type
	switch raw.Type {
	case EventSystem:
		var e SystemEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("parse system event: %w", err)
		}
		return &e, nil

	case EventAssistant:
		var e AssistantEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("parse assistant event: %w", err)
		}
		return &e, nil

	case EventResult:
		var e ResultEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("parse result event: %w", err)
		}
		return &e, nil

	case EventError:
		var e ErrorEvent
		if err := json.Unmarshal(data, &e); err != nil {
			return nil, fmt.Errorf("parse error event: %w", err)
		}
		return &e, nil

	default:
		// For unrecognized types, return a GenericEvent with the raw data
		return &GenericEvent{Type_: raw.Type, Data: json.RawMessage(data)}, nil
	}
}

// ReadAll reads all events from the stream until EOF. Useful for testing
// or when you need to collect all events before processing.
func (r *Reader) ReadAll() ([]Event, error) {
	var events []Event
	for {
		event, err := r.Next()
		if errors.Is(err, io.EOF) {
			return events, nil
		}
		if err != nil {
			return events, err
		}
		events = append(events, event)
	}
}

// CollectResult reads events until a ResultEvent is found and returns it.
// Returns nil and io.EOF if the stream ends without a result event.
// Every event (including the ResultEvent) is passed to the optional callback if provided.
func (r *Reader) CollectResult(onEvent func(Event)) (*ResultEvent, error) {
	for {
		event, err := r.Next()
		if err != nil {
			return nil, err
		}
		if onEvent != nil {
			onEvent(event)
		}
		if result, ok := event.(*ResultEvent); ok {
			return result, nil
		}
	}
}

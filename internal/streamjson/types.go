package streamjson

import "encoding/json"

// EventType identifies the kind of stream-json event.
type EventType string

const (
	EventSystem       EventType = "system"
	EventAssistant    EventType = "assistant"
	EventUser         EventType = "user"
	EventResult       EventType = "result"
	EventContentBlock EventType = "content_block_start"
	EventContentDelta EventType = "content_block_delta"
	EventContentStop  EventType = "content_block_stop"
	EventToolUse      EventType = "tool_use"
	EventToolResult   EventType = "tool_result"
	EventMessageStart EventType = "message_start"
	EventMessageDelta EventType = "message_delta"
	EventMessageStop  EventType = "message_stop"
	EventError        EventType = "error"
	EventPing         EventType = "ping"
)

// Event is the interface implemented by all stream-json event types.
type Event interface {
	EventType() EventType
}

// RawEvent is used for initial JSON deserialization to determine the event type.
type RawEvent struct {
	Type    EventType       `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Raw     json.RawMessage `json:"-"` // The full raw JSON for re-parsing
}

// SystemEvent is emitted at the start of a session with initialization data.
type SystemEvent struct {
	Type      EventType `json:"type"`
	Subtype   string    `json:"subtype"`
	SessionID string    `json:"session_id"`
	Tools     []Tool    `json:"tools,omitempty"`
	MCPTools  []Tool    `json:"mcp_tools,omitempty"`
}

func (e *SystemEvent) EventType() EventType { return EventSystem }

// Tool describes an available tool in the system init event.
type Tool struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
}

// AssistantEvent carries a model-generated message (text, tool use, etc.).
type AssistantEvent struct {
	Type    EventType        `json:"type"`
	Message AssistantMessage `json:"message"`
}

func (e *AssistantEvent) EventType() EventType { return EventAssistant }

// AssistantMessage contains the content blocks of an assistant turn.
type AssistantMessage struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// ContentBlock represents a single content element in a message.
type ContentBlock struct {
	Type  string          `json:"type"` // "text", "tool_use", "tool_result"
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// ResultEvent is emitted when the session completes (success or error).
type ResultEvent struct {
	Type         EventType `json:"type"`
	Subtype      string    `json:"subtype"` // "success" or "error"
	CostUSD      float64   `json:"cost_usd"`
	TotalCostUSD float64   `json:"total_cost_usd"`
	DurationMS   int64     `json:"duration_ms"`
	DurationAPI  int64     `json:"duration_api_ms"`
	IsError      bool      `json:"is_error"`
	NumTurns     int       `json:"num_turns"`
	SessionID    string    `json:"session_id"`
	Usage        Usage     `json:"usage"`
	Result       string    `json:"result,omitempty"` // Final text output for success
	Error        string    `json:"error,omitempty"`  // Error message for failures
}

func (e *ResultEvent) EventType() EventType { return EventResult }

// Usage contains token consumption data.
type Usage struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CacheReadTokens   int64 `json:"cache_read_input_tokens"`
	CacheCreateTokens int64 `json:"cache_creation_input_tokens"`
}

// ErrorEvent signals an error during processing.
type ErrorEvent struct {
	Type  EventType `json:"type"`
	Error ErrorInfo `json:"error"`
}

func (e *ErrorEvent) EventType() EventType { return EventError }

// ErrorInfo contains error details.
type ErrorInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// GenericEvent is used for event types that don't need special handling.
type GenericEvent struct {
	Type_ EventType       `json:"type"`
	Data  json.RawMessage `json:"-"`
}

func (e *GenericEvent) EventType() EventType { return e.Type_ }

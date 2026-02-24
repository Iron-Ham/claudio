package streamjson

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestReader_SystemEvent(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess-123","tools":[{"name":"Read","type":"file"}]}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	sys, ok := event.(*SystemEvent)
	if !ok {
		t.Fatalf("expected *SystemEvent, got %T", event)
	}
	if sys.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", sys.SessionID, "sess-123")
	}
	if sys.Subtype != "init" {
		t.Errorf("Subtype = %q, want %q", sys.Subtype, "init")
	}
	if len(sys.Tools) != 1 {
		t.Fatalf("Tools len = %d, want 1", len(sys.Tools))
	}
	if sys.Tools[0].Name != "Read" {
		t.Errorf("Tools[0].Name = %q, want %q", sys.Tools[0].Name, "Read")
	}
	if sys.EventType() != EventSystem {
		t.Errorf("EventType() = %q, want %q", sys.EventType(), EventSystem)
	}
}

func TestReader_AssistantEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Hello world"}]}}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	asst, ok := event.(*AssistantEvent)
	if !ok {
		t.Fatalf("expected *AssistantEvent, got %T", event)
	}
	if asst.Message.Role != "assistant" {
		t.Errorf("Role = %q, want %q", asst.Message.Role, "assistant")
	}
	if len(asst.Message.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(asst.Message.Content))
	}
	if asst.Message.Content[0].Text != "Hello world" {
		t.Errorf("Content[0].Text = %q, want %q", asst.Message.Content[0].Text, "Hello world")
	}
	if asst.EventType() != EventAssistant {
		t.Errorf("EventType() = %q, want %q", asst.EventType(), EventAssistant)
	}
}

func TestReader_ResultEvent(t *testing.T) {
	input := `{"type":"result","subtype":"success","cost_usd":0.05,"total_cost_usd":0.10,"duration_ms":5000,"is_error":false,"num_turns":3,"session_id":"sess-456","usage":{"input_tokens":1000,"output_tokens":500,"cache_read_input_tokens":200,"cache_creation_input_tokens":50},"result":"Task completed successfully"}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	result, ok := event.(*ResultEvent)
	if !ok {
		t.Fatalf("expected *ResultEvent, got %T", event)
	}
	if result.Subtype != "success" {
		t.Errorf("Subtype = %q, want %q", result.Subtype, "success")
	}
	if result.CostUSD != 0.05 {
		t.Errorf("CostUSD = %f, want 0.05", result.CostUSD)
	}
	if result.TotalCostUSD != 0.10 {
		t.Errorf("TotalCostUSD = %f, want 0.10", result.TotalCostUSD)
	}
	if result.NumTurns != 3 {
		t.Errorf("NumTurns = %d, want 3", result.NumTurns)
	}
	if result.Usage.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", result.Usage.InputTokens)
	}
	if result.Usage.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", result.Usage.OutputTokens)
	}
	if result.Usage.CacheReadTokens != 200 {
		t.Errorf("CacheReadTokens = %d, want 200", result.Usage.CacheReadTokens)
	}
	if result.Result != "Task completed successfully" {
		t.Errorf("Result = %q, want %q", result.Result, "Task completed successfully")
	}
	if result.IsError {
		t.Error("IsError = true, want false")
	}
	if result.EventType() != EventResult {
		t.Errorf("EventType() = %q, want %q", result.EventType(), EventResult)
	}
}

func TestReader_ErrorEvent(t *testing.T) {
	input := `{"type":"error","error":{"type":"rate_limit","message":"Too many requests"}}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	errEvt, ok := event.(*ErrorEvent)
	if !ok {
		t.Fatalf("expected *ErrorEvent, got %T", event)
	}
	if errEvt.Error.Type != "rate_limit" {
		t.Errorf("Error.Type = %q, want %q", errEvt.Error.Type, "rate_limit")
	}
	if errEvt.Error.Message != "Too many requests" {
		t.Errorf("Error.Message = %q, want %q", errEvt.Error.Message, "Too many requests")
	}
	if errEvt.EventType() != EventError {
		t.Errorf("EventType() = %q, want %q", errEvt.EventType(), EventError)
	}
}

func TestReader_GenericEvent(t *testing.T) {
	input := `{"type":"ping"}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	generic, ok := event.(*GenericEvent)
	if !ok {
		t.Fatalf("expected *GenericEvent, got %T", event)
	}
	if generic.EventType() != EventPing {
		t.Errorf("EventType() = %q, want %q", generic.EventType(), EventPing)
	}
}

func TestReader_MultipleEvents(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Working..."}]}}
{"type":"result","subtype":"success","cost_usd":0.01,"total_cost_usd":0.01,"duration_ms":1000,"is_error":false,"num_turns":1,"session_id":"s1","usage":{"input_tokens":100,"output_tokens":50}}
`

	r := NewReader(strings.NewReader(input))
	events, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}

	if len(events) != 3 {
		t.Fatalf("events len = %d, want 3", len(events))
	}

	if _, ok := events[0].(*SystemEvent); !ok {
		t.Errorf("events[0] type = %T, want *SystemEvent", events[0])
	}
	if _, ok := events[1].(*AssistantEvent); !ok {
		t.Errorf("events[1] type = %T, want *AssistantEvent", events[1])
	}
	if _, ok := events[2].(*ResultEvent); !ok {
		t.Errorf("events[2] type = %T, want *ResultEvent", events[2])
	}
}

func TestReader_SkipsBlankLines(t *testing.T) {
	input := "\n\n" + `{"type":"ping"}` + "\n\n"

	r := NewReader(strings.NewReader(input))
	events, err := r.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll() error: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
}

func TestReader_EmptyStream(t *testing.T) {
	r := NewReader(strings.NewReader(""))
	event, err := r.Next()
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF, got %v (event: %v)", err, event)
	}
}

func TestReader_InvalidJSON(t *testing.T) {
	input := "not json\n"
	r := NewReader(strings.NewReader(input))
	_, err := r.Next()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReader_CollectResult(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Working"}]}}
{"type":"result","subtype":"success","cost_usd":0.02,"total_cost_usd":0.02,"duration_ms":2000,"is_error":false,"num_turns":2,"session_id":"s1","usage":{"input_tokens":200,"output_tokens":100}}
`

	r := NewReader(strings.NewReader(input))
	var intermediateCount int
	result, err := r.CollectResult(func(e Event) {
		intermediateCount++
	})
	if err != nil {
		t.Fatalf("CollectResult() error: %v", err)
	}

	// All 3 events should have been passed to the callback
	if intermediateCount != 3 {
		t.Errorf("intermediateCount = %d, want 3", intermediateCount)
	}

	if result == nil {
		t.Fatal("result is nil")
	}
	if result.Subtype != "success" {
		t.Errorf("result.Subtype = %q, want %q", result.Subtype, "success")
	}
	if result.NumTurns != 2 {
		t.Errorf("result.NumTurns = %d, want 2", result.NumTurns)
	}
}

func TestReader_CollectResult_NoResult(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"s1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Working"}]}}
`

	r := NewReader(strings.NewReader(input))
	result, err := r.CollectResult(nil)
	if !errors.Is(err, io.EOF) {
		t.Errorf("expected io.EOF, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestReader_ToolUseContent(t *testing.T) {
	input := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu_123","name":"Read","input":{"file_path":"/tmp/test.go"}}]}}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	asst := event.(*AssistantEvent)
	if len(asst.Message.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(asst.Message.Content))
	}
	block := asst.Message.Content[0]
	if block.Type != "tool_use" {
		t.Errorf("Type = %q, want %q", block.Type, "tool_use")
	}
	if block.Name != "Read" {
		t.Errorf("Name = %q, want %q", block.Name, "Read")
	}
	if block.ID != "tu_123" {
		t.Errorf("ID = %q, want %q", block.ID, "tu_123")
	}
}

func TestReader_ResultError(t *testing.T) {
	input := `{"type":"result","subtype":"error","cost_usd":0.01,"total_cost_usd":0.01,"duration_ms":500,"is_error":true,"num_turns":1,"session_id":"s1","usage":{"input_tokens":50,"output_tokens":10},"error":"max turns exceeded"}` + "\n"

	r := NewReader(strings.NewReader(input))
	event, err := r.Next()
	if err != nil {
		t.Fatalf("Next() error: %v", err)
	}

	result := event.(*ResultEvent)
	if !result.IsError {
		t.Error("IsError = false, want true")
	}
	if result.Error != "max turns exceeded" {
		t.Errorf("Error = %q, want %q", result.Error, "max turns exceeded")
	}
}

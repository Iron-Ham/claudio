package namer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAnthropicClient_NoAPIKey(t *testing.T) {
	// Use t.Setenv which automatically restores the original value
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := NewAnthropicClient()
	if err == nil {
		t.Error("expected error when API key not set")
	}
}

func TestNewAnthropicClient_WithAPIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	client, err := NewAnthropicClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.model != defaultModel {
		t.Errorf("expected model %s, got %s", defaultModel, client.model)
	}
}

func TestNewAnthropicClient_WithOptions(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	client, err := NewAnthropicClient(
		WithModel("custom-model"),
		WithMaxNameLength(50),
		WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.model != "custom-model" {
		t.Errorf("expected model custom-model, got %s", client.model)
	}
	if client.maxLen != 50 {
		t.Errorf("expected maxLen 50, got %d", client.maxLen)
	}
}

func TestAnthropicClient_Summarize_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("missing or invalid API key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("missing or invalid anthropic-version header")
		}

		// Return success response
		resp := messagesResponse{
			Content: []contentBlock{
				{Type: "text", Text: "Fix auth bug"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:     "test-key",
		model:      defaultModel,
		maxLen:     defaultMaxNameLength,
		httpClient: server.Client(),
	}

	// Override the API URL by replacing the httpClient with one that routes to our test server
	originalTransport := client.httpClient.Transport
	client.httpClient.Transport = &testTransport{
		targetURL: server.URL,
		transport: originalTransport,
	}

	name, err := client.Summarize(context.Background(), "Fix authentication issues with OAuth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Fix auth bug" {
		t.Errorf("expected 'Fix auth bug', got '%s'", name)
	}
}

func TestAnthropicClient_Summarize_TrimsQuotes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := messagesResponse{
			Content: []contentBlock{
				{Type: "text", Text: `"Fix auth bug"`},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:     "test-key",
		model:      defaultModel,
		maxLen:     defaultMaxNameLength,
		httpClient: server.Client(),
	}
	client.httpClient.Transport = &testTransport{
		targetURL: server.URL,
		transport: client.httpClient.Transport,
	}

	name, err := client.Summarize(context.Background(), "task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Fix auth bug" {
		t.Errorf("expected 'Fix auth bug' (no quotes), got '%s'", name)
	}
}

func TestAnthropicClient_Summarize_TruncatesLongNames(t *testing.T) {
	longName := "This is a very long name that exceeds the maximum length allowed"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := messagesResponse{
			Content: []contentBlock{
				{Type: "text", Text: longName},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:     "test-key",
		model:      defaultModel,
		maxLen:     20,
		httpClient: server.Client(),
	}
	client.httpClient.Transport = &testTransport{
		targetURL: server.URL,
		transport: client.httpClient.Transport,
	}

	name, err := client.Summarize(context.Background(), "task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(name) > 20 {
		t.Errorf("expected name to be truncated to 20 chars, got %d chars: %s", len(name), name)
	}
}

func TestAnthropicClient_Summarize_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		if _, err := w.Write([]byte(`{"error": {"type": "rate_limit", "message": "Too many requests"}}`)); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:     "test-key",
		model:      defaultModel,
		maxLen:     defaultMaxNameLength,
		httpClient: server.Client(),
	}
	client.httpClient.Transport = &testTransport{
		targetURL: server.URL,
		transport: client.httpClient.Transport,
	}

	_, err := client.Summarize(context.Background(), "task")
	if err == nil {
		t.Error("expected error for rate limit response")
	}
}

func TestAnthropicClient_Summarize_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := messagesResponse{
			Content: []contentBlock{},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:     "test-key",
		model:      defaultModel,
		maxLen:     defaultMaxNameLength,
		httpClient: server.Client(),
	}
	client.httpClient.Transport = &testTransport{
		targetURL: server.URL,
		transport: client.httpClient.Transport,
	}

	_, err := client.Summarize(context.Background(), "task")
	if err == nil {
		t.Error("expected error for empty response")
	}
}

func TestAnthropicClient_Summarize_EmptyNameAfterTrim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := messagesResponse{
			Content: []contentBlock{
				{Type: "text", Text: "   \"\"   "}, // Whitespace and empty quotes
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := &AnthropicClient{
		apiKey:     "test-key",
		model:      defaultModel,
		maxLen:     defaultMaxNameLength,
		httpClient: server.Client(),
	}
	client.httpClient.Transport = &testTransport{
		targetURL: server.URL,
		transport: client.httpClient.Transport,
	}

	_, err := client.Summarize(context.Background(), "task")
	if err == nil {
		t.Error("expected error for empty name after trimming")
	}
	if err != nil && err.Error() != "API returned empty name" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// testTransport redirects all requests to the test server.
type testTransport struct {
	targetURL string
	transport http.RoundTripper
}

func (tr *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect the request to our test server
	req.URL.Scheme = "http"
	req.URL.Host = tr.targetURL[7:] // Strip "http://"
	if tr.transport != nil {
		return tr.transport.RoundTrip(req)
	}
	return http.DefaultTransport.RoundTrip(req)
}

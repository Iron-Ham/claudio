// Package namer provides intelligent instance naming using LLM summarization.
package namer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// anthropicAPIURL is the Anthropic Messages API endpoint.
	anthropicAPIURL = "https://api.anthropic.com/v1/messages"

	// defaultModel is the Claude model used for summarization.
	defaultModel = "claude-3-haiku-20240307"

	// defaultMaxNameLength is the maximum length for generated names.
	defaultMaxNameLength = 35

	// defaultTimeout is the API request timeout.
	defaultTimeout = 10 * time.Second
)

// summarizePrompt is the prompt template for generating short instance names.
// The prompt works with just the task description - no output context needed.
const summarizePrompt = `Generate a very short name (max %d chars) for this coding task.

Rules:
1. Be descriptive but concise
2. Start with a verb (Add, Fix, Update, Implement, Refactor, Test, etc.)
3. Focus on the core action/change being requested
4. Omit articles (a, the) and filler words
5. No quotes or punctuation
6. Use title case

Examples:
- "Add user authentication" -> "Add User Auth"
- "Fix the bug where login fails on mobile" -> "Fix Mobile Login Bug"
- "Implement dark mode toggle" -> "Add Dark Mode Toggle"
- "Refactor the database connection pooling" -> "Refactor DB Pool"

Task: %s

Respond with ONLY the short name, nothing else.`

// Client defines the interface for LLM-based name generation.
type Client interface {
	// Summarize generates a short descriptive name from a task description.
	Summarize(ctx context.Context, task string) (string, error)
}

// AnthropicClient implements Client using the Anthropic Messages API.
type AnthropicClient struct {
	apiKey     string
	model      string
	maxLen     int
	httpClient *http.Client
}

// ClientOption configures an AnthropicClient.
type ClientOption func(*AnthropicClient)

// WithModel sets the model to use for summarization.
func WithModel(model string) ClientOption {
	return func(c *AnthropicClient) {
		c.model = model
	}
}

// WithMaxNameLength sets the maximum name length.
func WithMaxNameLength(maxLen int) ClientOption {
	return func(c *AnthropicClient) {
		c.maxLen = maxLen
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *AnthropicClient) {
		c.httpClient.Timeout = timeout
	}
}

// NewAnthropicClient creates a new client using the ANTHROPIC_API_KEY env var.
// Returns an error if the API key is not set.
func NewAnthropicClient(opts ...ClientOption) (*AnthropicClient, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
	}

	c := &AnthropicClient{
		apiKey: apiKey,
		model:  defaultModel,
		maxLen: defaultMaxNameLength,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// messagesRequest is the Anthropic Messages API request structure.
type messagesRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// messagesResponse is the Anthropic Messages API response structure.
type messagesResponse struct {
	Content []contentBlock `json:"content"`
	Error   *apiError      `json:"error,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type apiError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Summarize generates a short descriptive name from the task description.
func (c *AnthropicClient) Summarize(ctx context.Context, task string) (string, error) {
	prompt := fmt.Sprintf(summarizePrompt, c.maxLen, task)

	reqBody := messagesRequest{
		Model:     c.model,
		MaxTokens: 50, // Short names don't need many tokens
		Messages: []message{
			{Role: "user", Content: prompt},
		},
	}

	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicAPIURL, bytes.NewReader(reqBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	var respData messagesResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if respData.Error != nil {
		return "", fmt.Errorf("API error: %s", respData.Error.Message)
	}

	if len(respData.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	name := strings.TrimSpace(respData.Content[0].Text)

	// Remove any quotes the model might have added
	name = strings.Trim(name, "\"'`")

	// Validate name is not empty after cleaning
	if name == "" {
		return "", fmt.Errorf("API returned empty name")
	}

	// Enforce max length
	if len(name) > c.maxLen {
		name = name[:c.maxLen]
	}

	return name, nil
}

package pr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/Iron-Ham/claudio/internal/ai"
)

// PRContent holds the generated PR title and body
type PRContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// PROptions contains options for PR creation
type PROptions struct {
	Title     string
	Body      string
	Branch    string
	Draft     bool
	Reviewers []string
	Labels    []string
}

// Context holds all the information needed to generate PR content
type Context struct {
	Task         string
	Branch       string
	Diff         string
	CommitLog    string
	ChangedFiles []string
	InstanceID   string
}

// Generator creates PR content using the configured AI backend.
type Generator struct {
	backend ai.Backend
}

// New creates a new PR generator
func New() *Generator {
	return NewWithBackend(ai.DefaultBackend())
}

// NewWithBackend creates a new PR generator with the specified backend.
func NewWithBackend(backend ai.Backend) *Generator {
	if backend == nil {
		backend = ai.DefaultBackend()
	}
	return &Generator{backend: backend}
}

// promptTemplate is the prompt sent to the backend for generating PR content
const promptTemplate = `You are helping create a pull request. Based on the following information, generate a concise and meaningful PR title and description.

## Task Description
{{.Task}}

## Branch Name
{{.Branch}}

## Changed Files
{{range .ChangedFiles}}- {{.}}
{{end}}

## Commit History
{{.CommitLog}}

## Code Diff (truncated if large)
{{.Diff}}

---

Generate a PR with:
1. A concise title following conventional commit format (e.g., "feat: add user authentication", "fix: resolve memory leak")
2. A body that includes:
   - A brief summary (2-3 sentences max)
   - Key changes as bullet points
   - Any important notes for reviewers

Respond ONLY with valid JSON in this exact format:
{"title": "your title here", "body": "your body here\n\nwith proper newlines"}

Important:
- Keep the title under 72 characters
- Use lowercase for the conventional commit prefix
- Be concise but informative
- Do not include any text outside the JSON object`

// Generate uses the configured backend to create PR content from the provided context.
func (g *Generator) Generate(ctx Context) (*PRContent, error) {
	backend := g.backend
	if backend == nil {
		backend = ai.DefaultBackend()
	}

	// Build the prompt
	tmpl, err := template.New("prompt").Parse(promptTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Truncate diff if too large (backend has context limits)
	diff := ctx.Diff
	const maxDiffSize = 50000
	if len(diff) > maxDiffSize {
		diff = diff[:maxDiffSize] + "\n\n... (diff truncated due to size)"
	}
	ctx.Diff = diff

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, ctx); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}

	promptFile, err := os.CreateTemp("", "claudio-pr-*.prompt")
	if err != nil {
		return nil, fmt.Errorf("failed to create prompt file: %w", err)
	}
	if _, err := promptFile.Write(promptBuf.Bytes()); err != nil {
		_ = promptFile.Close()
		_ = os.Remove(promptFile.Name())
		return nil, fmt.Errorf("failed to write prompt file: %w", err)
	}
	if err := promptFile.Close(); err != nil {
		_ = os.Remove(promptFile.Name())
		return nil, fmt.Errorf("failed to close prompt file: %w", err)
	}
	defer func() {
		_ = os.Remove(promptFile.Name())
	}()

	cmdString, err := backend.BuildStartCommand(ai.StartOptions{
		PromptFile: promptFile.Name(),
		Mode:       ai.StartModeOneShot,
		OutputOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build backend command: %w", err)
	}

	cmd := exec.Command("sh", "-c", cmdString)
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		if errMsg == "" {
			errMsg = "unknown error"
		}
		return nil, fmt.Errorf("backend command failed: %w\nstderr: %s", err, errMsg)
	}

	// Parse the JSON response
	// The backend might include markdown code blocks, so we need to extract the JSON
	responseStr := strings.TrimSpace(string(output))
	responseStr = extractJSON(responseStr)

	var content PRContent
	if err := json.Unmarshal([]byte(responseStr), &content); err != nil {
		return nil, fmt.Errorf("failed to parse backend response as JSON: %w\nresponse: %s", err, responseStr)
	}

	return &content, nil
}

// extractJSON extracts JSON from a response that might be wrapped in markdown code blocks
func extractJSON(s string) string {
	// Remove markdown code blocks if present
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Try to find JSON object boundaries
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start != -1 && end != -1 && end > start {
		return s[start : end+1]
	}

	return s
}

// Create creates a GitHub PR using the gh CLI with full options support
func Create(opts PROptions) (string, error) {
	args := []string{"pr", "create",
		"--title", opts.Title,
		"--body", opts.Body,
		"--head", opts.Branch,
	}

	if opts.Draft {
		args = append(args, "--draft")
	}

	for _, reviewer := range opts.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	cmd := exec.Command("gh", args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create PR: %w\n%s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

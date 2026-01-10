// Package issue provides a provider-agnostic interface for closing issues
// in external issue trackers (GitHub, Linear, Notion, etc.)
package issue

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Iron-Ham/claudio/internal/logging"
)

// Provider represents an issue tracking service
type Provider string

const (
	ProviderGitHub  Provider = "github"
	ProviderLinear  Provider = "linear"
	ProviderNotion  Provider = "notion"
	ProviderUnknown Provider = "unknown"
)

// URL parsing regexes
var (
	gitHubRegex = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/issues/(\d+)$`)
	linearRegex = regexp.MustCompile(`linear\.app/[^/]+/issue/([A-Z]+-\d+)`)
)

// Service handles closing issues across different providers
type Service struct {
	logger *logging.Logger
}

// NewService creates a new issue service
func NewService(logger *logging.Logger) *Service {
	return &Service{logger: logger}
}

// Close closes an issue given its URL
// Returns nil if the URL is empty or the provider is unsupported
func (s *Service) Close(ctx context.Context, issueURL string) error {
	if issueURL == "" {
		return nil
	}

	provider, err := DetectProvider(issueURL)
	if err != nil {
		s.logger.Warn("failed to detect issue provider", "url", issueURL, "error", err)
		return nil // Don't fail task completion for issue closing errors
	}

	switch provider {
	case ProviderGitHub:
		return s.closeGitHub(ctx, issueURL)
	case ProviderLinear:
		return s.closeLinear(ctx, issueURL)
	case ProviderNotion:
		return s.closeNotion(ctx, issueURL)
	default:
		s.logger.Debug("unsupported issue provider", "url", issueURL, "provider", provider)
		return nil
	}
}

// DetectProvider determines the issue provider from a URL
func DetectProvider(issueURL string) (Provider, error) {
	parsed, err := url.Parse(issueURL)
	if err != nil {
		return ProviderUnknown, fmt.Errorf("invalid URL: %w", err)
	}

	host := strings.ToLower(parsed.Host)

	switch {
	case strings.Contains(host, "github.com"):
		return ProviderGitHub, nil
	case strings.Contains(host, "linear.app"):
		return ProviderLinear, nil
	case strings.Contains(host, "notion.so") || strings.Contains(host, "notion.site"):
		return ProviderNotion, nil
	default:
		return ProviderUnknown, nil
	}
}

// closeGitHub closes a GitHub issue using the gh CLI
func (s *Service) closeGitHub(ctx context.Context, issueURL string) error {
	// Extract owner/repo and issue number from URL
	// Format: https://github.com/owner/repo/issues/123
	matches := gitHubRegex.FindStringSubmatch(issueURL)
	if len(matches) != 4 {
		return fmt.Errorf("invalid GitHub issue URL: %s", issueURL)
	}

	owner, repo, number := matches[1], matches[2], matches[3]
	repoPath := fmt.Sprintf("%s/%s", owner, repo)

	s.logger.Info("closing GitHub issue", "repo", repoPath, "issue", number)

	cmd := exec.CommandContext(ctx, "gh", "issue", "close", number, "--repo", repoPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to close GitHub issue #%s: %w\noutput: %s", number, err, string(output))
	}

	s.logger.Info("closed GitHub issue", "repo", repoPath, "issue", number)
	return nil
}

// closeLinear closes a Linear issue using the linear CLI (if available)
func (s *Service) closeLinear(ctx context.Context, issueURL string) error {
	// Linear URL format: https://linear.app/team/issue/TEAM-123/title
	matches := linearRegex.FindStringSubmatch(issueURL)
	if len(matches) != 2 {
		return fmt.Errorf("invalid Linear issue URL: %s", issueURL)
	}

	issueID := matches[1]
	s.logger.Info("closing Linear issue", "issue", issueID)

	// Try using linear CLI if available
	cmd := exec.CommandContext(ctx, "linear", "issue", "close", issueID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Linear CLI might not be installed - log and continue
		s.logger.Warn("failed to close Linear issue (linear CLI may not be installed)",
			"issue", issueID, "error", err, "output", string(output))
		return nil
	}

	s.logger.Info("closed Linear issue", "issue", issueID)
	return nil
}

// closeNotion marks a Notion page/task as complete
// Note: Notion doesn't have a CLI, so this is a placeholder for API integration
func (s *Service) closeNotion(ctx context.Context, issueURL string) error {
	s.logger.Debug("Notion issue closing not yet implemented", "url", issueURL)
	// Future: Use Notion API to update page status
	// Would require NOTION_API_KEY and database-specific property names
	return nil
}

package plan

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// IssueOptions contains options for issue creation
type IssueOptions struct {
	Title  string
	Body   string
	Labels []string
}

// IssueCreationResult holds the results of creating all issues
type IssueCreationResult struct {
	ParentIssueNumber int
	ParentIssueURL    string
	SubIssueNumbers   map[string]int    // task_id -> issue_number
	SubIssueURLs      map[string]string // task_id -> issue_url
}

// CreateIssue creates a GitHub issue using the gh CLI
// Returns the issue number and URL on success
func CreateIssue(opts IssueOptions) (int, string, error) {
	args := []string{"issue", "create",
		"--title", opts.Title,
		"--body", opts.Body,
	}

	for _, label := range opts.Labels {
		args = append(args, "--label", label)
	}

	cmd := exec.Command("gh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, "", fmt.Errorf("failed to create issue: %w\n%s", err, string(output))
	}

	url := strings.TrimSpace(string(output))
	num, err := parseIssueNumber(url)
	if err != nil {
		return 0, url, err
	}

	return num, url, nil
}

// parseIssueNumber extracts issue number from gh output URL
// e.g., https://github.com/owner/repo/issues/123
func parseIssueNumber(output string) (int, error) {
	re := regexp.MustCompile(`/issues/(\d+)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse issue number from: %s", output)
	}

	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, fmt.Errorf("invalid issue number: %w", err)
	}

	return num, nil
}

// UpdateIssueBody updates an existing issue's body
// This is used to update the parent issue after all sub-issues are created
func UpdateIssueBody(issueNumber int, newBody string) error {
	cmd := exec.Command("gh", "issue", "edit",
		strconv.Itoa(issueNumber),
		"--body", newBody,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update issue: %w\n%s", err, string(output))
	}

	return nil
}

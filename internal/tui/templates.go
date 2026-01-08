package tui

// TaskTemplate represents a task template that can be selected via "/" dropdown
type TaskTemplate struct {
	Command     string // The slash command (e.g., "test", "docs")
	Name        string // Display name (e.g., "Run Tests")
	Description string // Full task description that gets expanded
}

// TaskTemplates defines the available task templates
var TaskTemplates = []TaskTemplate{
	{
		Command:     "test",
		Name:        "Run Tests",
		Description: "Run the test suite and fix any failing tests. Report a summary of what was fixed.",
	},
	{
		Command:     "docs",
		Name:        "Add Documentation",
		Description: "Add or improve documentation for the codebase. Focus on public APIs and complex logic.",
	},
	{
		Command:     "refactor",
		Name:        "Refactor Code",
		Description: "Refactor the code to improve readability, maintainability, and performance without changing behavior.",
	},
	{
		Command:     "fix",
		Name:        "Fix Bug",
		Description: "Investigate and fix the bug described below:\n\n",
	},
	{
		Command:     "feature",
		Name:        "Add Feature",
		Description: "Implement the following feature:\n\n",
	},
	{
		Command:     "review",
		Name:        "Code Review",
		Description: "Review the recent changes and provide feedback on code quality, potential bugs, and improvements.",
	},
	{
		Command:     "types",
		Name:        "Add Types",
		Description: "Add or improve type annotations throughout the codebase for better type safety.",
	},
	{
		Command:     "lint",
		Name:        "Fix Lint Issues",
		Description: "Run the linter and fix all reported issues. Ensure code follows project style guidelines.",
	},
	{
		Command:     "perf",
		Name:        "Optimize Performance",
		Description: "Profile and optimize performance bottlenecks in the codebase.",
	},
	{
		Command:     "security",
		Name:        "Security Audit",
		Description: "Perform a security audit and fix any vulnerabilities found.",
	},
	{
		Command: "megamerge",
		Name:    "Mega Merge PRs",
		Description: `Merge all of my open PRs to main. Follow these steps:

1. **List all open PRs**: Use 'gh pr list --author @me --state open' to find all my open PRs targeting main.

2. **For each PR, in order of oldest first**:
   a. Check if the PR is mergeable using 'gh pr view <PR_NUMBER> --json mergeable,mergeStateStatus'
   b. If there are merge conflicts:
      - Checkout the PR branch: 'gh pr checkout <PR_NUMBER>'
      - Fetch and rebase onto main: 'git fetch origin main && git rebase origin/main'
      - Resolve any conflicts by examining the conflicting files and making intelligent fixes
      - After resolving, continue the rebase: 'git add . && git rebase --continue'
      - Force push the fixed branch: 'git push --force-with-lease'
   c. If CI checks are failing, investigate and fix the issues
   d. Once the PR is mergeable and checks pass, merge it: 'gh pr merge <PR_NUMBER> --squash --delete-branch'

3. **Report summary**: After processing all PRs, provide a summary of:
   - Which PRs were successfully merged
   - Which PRs had conflicts and how they were resolved
   - Which PRs could not be merged and why

Important: Process PRs one at a time to avoid conflicts between dependent PRs. If a PR depends on another PR, merge the dependency first.`,
	},
}

// FilterTemplates returns templates that match the given filter string
func FilterTemplates(filter string) []TaskTemplate {
	if filter == "" {
		return TaskTemplates
	}

	var matches []TaskTemplate
	filterLower := toLower(filter)

	for _, t := range TaskTemplates {
		// Match against command or name
		if contains(toLower(t.Command), filterLower) || contains(toLower(t.Name), filterLower) {
			matches = append(matches, t)
		}
	}

	return matches
}

// toLower converts a string to lowercase (simple ASCII implementation)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

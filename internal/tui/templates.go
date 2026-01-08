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

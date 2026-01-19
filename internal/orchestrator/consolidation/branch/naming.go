package branch

import (
	"fmt"
	"strings"
	"unicode"
)

// NamingStrategy generates branch names for consolidation workflows.
type NamingStrategy struct {
	prefix    string
	sessionID string
}

// NewNamingStrategy creates a new naming strategy.
// If prefix is empty, "Iron-Ham" is used as the default.
func NewNamingStrategy(prefix, sessionID string) *NamingStrategy {
	if prefix == "" {
		prefix = "Iron-Ham"
	}
	return &NamingStrategy{
		prefix:    prefix,
		sessionID: sessionID,
	}
}

// GroupBranchName generates a branch name for a consolidation group.
// Example: "Iron-Ham/ultraplan-abc12345-group-1"
func (n *NamingStrategy) GroupBranchName(groupIdx int) string {
	planID := n.truncateID(n.sessionID)
	return fmt.Sprintf("%s/ultraplan-%s-group-%d", n.prefix, planID, groupIdx+1)
}

// SingleBranchName generates a branch name for single PR mode consolidation.
// Example: "Iron-Ham/ultraplan-abc12345"
func (n *NamingStrategy) SingleBranchName() string {
	planID := n.truncateID(n.sessionID)
	return fmt.Sprintf("%s/ultraplan-%s", n.prefix, planID)
}

// TaskBranchName generates a branch name for a task.
// Example: "Iron-Ham/ultraplan-abc12345-task-1-setup"
func (n *NamingStrategy) TaskBranchName(taskID string) string {
	planID := n.truncateID(n.sessionID)
	taskSlug := Slugify(taskID)
	return fmt.Sprintf("%s/ultraplan-%s-%s", n.prefix, planID, taskSlug)
}

// truncateID truncates a session ID to 8 characters for use in branch names.
func (n *NamingStrategy) truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// Prefix returns the branch prefix.
func (n *NamingStrategy) Prefix() string {
	return n.prefix
}

// SessionID returns the session ID.
func (n *NamingStrategy) SessionID() string {
	return n.sessionID
}

// Slugify converts text to a slug suitable for branch names or identifiers.
// It lowercases the text, replaces spaces with dashes, removes non-alphanumeric
// characters (except dashes), and limits the length.
func Slugify(text string) string {
	slug := strings.ToLower(text)
	slug = strings.ReplaceAll(slug, " ", "-")

	// Remove non-alphanumeric characters except dashes
	var result strings.Builder
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			result.WriteRune(r)
		}
	}

	// Limit length
	s := result.String()
	if len(s) > 30 {
		s = s[:30]
	}
	// Trim trailing dashes
	s = strings.TrimRight(s, "-")
	return s
}

// Package orchestrator provides utilities for branch consolidation.
package orchestrator

import (
	"fmt"
	"strings"
	"unicode"
)

// BranchNameGenerator generates branch names for consolidation workflows.
type BranchNameGenerator struct {
	Prefix    string
	SessionID string
}

// NewBranchNameGenerator creates a new branch name generator.
// If prefix is empty, "Iron-Ham" is used as the default.
func NewBranchNameGenerator(prefix, sessionID string) *BranchNameGenerator {
	if prefix == "" {
		prefix = "Iron-Ham"
	}
	return &BranchNameGenerator{
		Prefix:    prefix,
		SessionID: sessionID,
	}
}

// GroupBranchName generates a branch name for a consolidation group.
// Example: "Iron-Ham/ultraplan-abc12345-group-1"
func (g *BranchNameGenerator) GroupBranchName(groupIdx int) string {
	planID := g.truncateID(g.SessionID)
	return fmt.Sprintf("%s/ultraplan-%s-group-%d", g.Prefix, planID, groupIdx+1)
}

// SingleBranchName generates a branch name for single PR mode consolidation.
// Example: "Iron-Ham/ultraplan-abc12345"
func (g *BranchNameGenerator) SingleBranchName() string {
	planID := g.truncateID(g.SessionID)
	return fmt.Sprintf("%s/ultraplan-%s", g.Prefix, planID)
}

// truncateID truncates a session ID to 8 characters for use in branch names.
func (g *BranchNameGenerator) truncateID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
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

// DeduplicateStrings returns a slice with duplicate strings removed,
// preserving the order of first occurrence.
func DeduplicateStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(strs))
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

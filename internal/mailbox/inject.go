package mailbox

import (
	"fmt"
	"sort"
	"strings"
)

// FormatForPrompt formats a slice of messages into a human-readable block
// suitable for injection into a Claude prompt. Messages are grouped by type
// for readability.
//
// Returns an empty string if there are no messages.
func FormatForPrompt(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}

	// Group messages by type, preserving order within each group.
	groups := make(map[MessageType][]Message)
	var typeOrder []MessageType
	for _, msg := range messages {
		if _, exists := groups[msg.Type]; !exists {
			typeOrder = append(typeOrder, msg.Type)
		}
		groups[msg.Type] = append(groups[msg.Type], msg)
	}

	var b strings.Builder
	b.WriteString("<mailbox-messages>\n")

	for i, mt := range typeOrder {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("[%s]\n", strings.ToUpper(string(mt))))
		for _, msg := range groups[mt] {
			b.WriteString(fmt.Sprintf("  From: %s\n", msg.From))
			b.WriteString(fmt.Sprintf("  %s\n", msg.Body))
			if len(msg.Metadata) > 0 {
				b.WriteString(fmt.Sprintf("  Metadata: %s\n", formatMetadata(msg.Metadata)))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("</mailbox-messages>")
	return b.String()
}

// formatMetadata formats a metadata map as a compact key=value string.
// Keys are sorted for deterministic output.
func formatMetadata(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

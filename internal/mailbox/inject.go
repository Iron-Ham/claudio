package mailbox

import (
	"fmt"
	"sort"
	"strings"
	"time"
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

// FilterOptions controls which messages are included by FormatFiltered.
type FilterOptions struct {
	Types       []MessageType // Only include these types (empty = all)
	Since       time.Time     // Only messages after this time (zero = all)
	From        string        // Only messages from this sender (empty = all)
	MaxMessages int           // Maximum messages to include (0 = unlimited)
}

// FormatFiltered applies filters to messages and formats the result using
// FormatForPrompt. Filters are applied in order: type, since, from, then
// max messages (keeping the most recent).
func FormatFiltered(messages []Message, opts FilterOptions) string {
	filtered := filterMessages(messages, opts)
	return FormatForPrompt(filtered)
}

// filterMessages applies FilterOptions to a slice of messages and returns
// the matching subset.
func filterMessages(messages []Message, opts FilterOptions) []Message {
	var result []Message

	typeSet := make(map[MessageType]bool, len(opts.Types))
	for _, t := range opts.Types {
		typeSet[t] = true
	}

	for _, msg := range messages {
		if len(typeSet) > 0 && !typeSet[msg.Type] {
			continue
		}
		if !opts.Since.IsZero() && !msg.Timestamp.After(opts.Since) {
			continue
		}
		if opts.From != "" && msg.From != opts.From {
			continue
		}
		result = append(result, msg)
	}

	if opts.MaxMessages > 0 && len(result) > opts.MaxMessages {
		result = result[len(result)-opts.MaxMessages:]
	}

	return result
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

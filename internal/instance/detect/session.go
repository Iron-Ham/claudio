// Package detect provides output analysis for detecting Claude Code's states.

package detect

import (
	"regexp"
	"strings"
)

// SessionIDCallback is called when a Claude session ID is detected in output.
type SessionIDCallback func(instanceID, claudeSessionID string)

// SessionIDPatterns contains regex patterns for extracting Claude's conversation session ID.
// Claude Code displays session info in various formats depending on the output mode.
var SessionIDPatterns = []string{
	// Claude Code status line format: "Session: <session-id>"
	`(?i)Session:\s*([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`,
	// Claude Code also shows shorter session IDs in some contexts
	`(?i)session[:\s]+([a-f0-9]{32,36})`,
	// Resume message format after --resume: "Resuming session <session-id>"
	`(?i)Resuming session\s+([a-f0-9-]{32,36})`,
	// Session ID in conversation start: "Conversation <session-id>"
	`(?i)Conversation\s+([a-f0-9-]{32,36})`,
	// Alternative format: "conversation_id": "..."
	`"conversation_id":\s*"([a-f0-9-]{32,36})"`,
}

// SessionDetector parses Claude output to extract the conversation session ID.
// The session ID can be used with Claude's --resume flag to continue a conversation.
type SessionDetector struct {
	patterns []*regexp.Regexp

	// lastDetectedID caches the most recently detected session ID
	// to avoid repeated callbacks for the same ID.
	lastDetectedID string
}

// NewSessionDetector creates a new session ID detector.
func NewSessionDetector() *SessionDetector {
	return &SessionDetector{
		patterns: compilePatterns(SessionIDPatterns),
	}
}

// DetectSessionID scans output for a Claude session ID.
// Returns the session ID if found, or empty string if not found.
func (d *SessionDetector) DetectSessionID(output []byte) string {
	if len(output) == 0 {
		return ""
	}

	text := string(output)
	// Strip ANSI codes for cleaner matching
	text = StripAnsi(text)

	for _, pattern := range d.patterns {
		matches := pattern.FindStringSubmatch(text)
		if len(matches) >= 2 {
			sessionID := strings.TrimSpace(matches[1])
			if isValidSessionID(sessionID) {
				return sessionID
			}
		}
	}

	return ""
}

// ProcessOutput checks for a session ID and returns it if it's newly detected.
// Returns empty string if no new session ID was found.
func (d *SessionDetector) ProcessOutput(output []byte) string {
	sessionID := d.DetectSessionID(output)
	if sessionID == "" {
		return ""
	}

	// Only return if this is a new session ID
	if sessionID != d.lastDetectedID {
		d.lastDetectedID = sessionID
		return sessionID
	}

	return ""
}

// Reset clears the cached session ID, allowing re-detection.
func (d *SessionDetector) Reset() {
	d.lastDetectedID = ""
}

// LastDetectedID returns the most recently detected session ID.
func (d *SessionDetector) LastDetectedID() string {
	return d.lastDetectedID
}

// isValidSessionID validates that a string looks like a valid session ID.
func isValidSessionID(id string) bool {
	// Must be at least 32 characters (UUID without dashes or with dashes)
	if len(id) < 32 {
		return false
	}

	// Check for hex characters and optional dashes
	for _, c := range id {
		isHexDigit := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHexDigit && c != '-' {
			return false
		}
	}

	return true
}

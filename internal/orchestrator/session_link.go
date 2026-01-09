package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

// LinkType represents the type of session link
type LinkType string

const (
	// LinkTypeObserve allows the review session to read the implementer's worktree
	// without taking a full lock. The implementer retains full control.
	LinkTypeObserve LinkType = "observe"

	// LinkTypeBidirectional allows two-way communication between sessions.
	// Both sessions can send and receive messages.
	LinkTypeBidirectional LinkType = "bidirectional"
)

// ReviewChannelFileName is the name of the communication file for review messages
const ReviewChannelFileName = "review_channel.json"

// ObserverLockFileName is the name of the read-only observer lock file
const ObserverLockFileName = "observer.lock"

// SessionLink represents a link between a review session and an implementer session
type SessionLink struct {
	ReviewSessionID     string    `json:"review_session_id"`
	ImplementerSessionID string    `json:"implementer_session_id"`
	LinkType            string    `json:"link_type"`
	CreatedAt           time.Time `json:"created_at"`
	CommunicationFile   string    `json:"communication_file"`
}

// ReviewMessageType represents the type of review message
type ReviewMessageType string

const (
	ReviewMessageIssue      ReviewMessageType = "issue"
	ReviewMessageSuggestion ReviewMessageType = "suggestion"
	ReviewMessageQuestion   ReviewMessageType = "question"
	ReviewMessageResponse   ReviewMessageType = "response"
	ReviewMessageAck        ReviewMessageType = "ack"
)

// ReviewMessage represents a single message in the review communication channel
type ReviewMessage struct {
	ID        string            `json:"id"`
	From      string            `json:"from"`
	Type      ReviewMessageType `json:"type"`
	Content   string            `json:"content"`
	Timestamp time.Time         `json:"timestamp"`
	IssueRef  string            `json:"issue_ref,omitempty"`
}

// ReviewChannel holds all messages for a session link
type ReviewChannel struct {
	Messages []ReviewMessage `json:"messages"`
	mu       sync.RWMutex    `json:"-"`
}

// ObserverLock represents a read-only observation lock
// This allows review sessions to read implementer worktrees without blocking
type ObserverLock struct {
	SessionID  string    `json:"session_id"`
	ObserverID string    `json:"observer_id"`
	PID        int       `json:"pid"`
	Hostname   string    `json:"hostname"`
	StartedAt  time.Time `json:"started_at"`
	ReadOnly   bool      `json:"read_only"`
}

// SessionLinkManager manages session links and their communication channels
type SessionLinkManager struct {
	baseDir string
	links   map[string]*SessionLink // keyed by "reviewID:implementerID"
	mu      sync.RWMutex
}

// NewSessionLinkManager creates a new session link manager
func NewSessionLinkManager(baseDir string) *SessionLinkManager {
	return &SessionLinkManager{
		baseDir: baseDir,
		links:   make(map[string]*SessionLink),
	}
}

// linkKey generates a unique key for a session link
func linkKey(reviewSessionID, implementerSessionID string) string {
	return reviewSessionID + ":" + implementerSessionID
}

// getCommunicationFilePath returns the path to the review channel file for a session
func (m *SessionLinkManager) getCommunicationFilePath(sessionID string) string {
	return filepath.Join(m.baseDir, ".claudio", "sessions", sessionID, ReviewChannelFileName)
}

// getObserverLockPath returns the path to the observer lock file
func (m *SessionLinkManager) getObserverLockPath(sessionID string) string {
	return filepath.Join(m.baseDir, ".claudio", "sessions", sessionID, ObserverLockFileName)
}

// LinkSessions creates a link between a review session and an implementer session
func (m *SessionLinkManager) LinkSessions(reviewSessionID, implementerSessionID string, linkType string) (*SessionLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate link type
	if linkType != string(LinkTypeObserve) && linkType != string(LinkTypeBidirectional) {
		return nil, fmt.Errorf("invalid link type: %s (must be 'observe' or 'bidirectional')", linkType)
	}

	// Check if link already exists
	key := linkKey(reviewSessionID, implementerSessionID)
	if existing, ok := m.links[key]; ok {
		return existing, nil // Return existing link
	}

	// Verify implementer session directory exists
	implementerDir := filepath.Join(m.baseDir, ".claudio", "sessions", implementerSessionID)
	if _, err := os.Stat(implementerDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("implementer session not found: %s", implementerSessionID)
	}

	// Create communication file path (in implementer's session directory)
	commFile := m.getCommunicationFilePath(implementerSessionID)

	// Initialize the review channel file if it doesn't exist
	if _, err := os.Stat(commFile); os.IsNotExist(err) {
		channel := &ReviewChannel{Messages: make([]ReviewMessage, 0)}
		if err := m.writeReviewChannel(commFile, channel); err != nil {
			return nil, fmt.Errorf("failed to initialize review channel: %w", err)
		}
	}

	// Create the session link
	link := &SessionLink{
		ReviewSessionID:      reviewSessionID,
		ImplementerSessionID: implementerSessionID,
		LinkType:             linkType,
		CreatedAt:            time.Now(),
		CommunicationFile:    commFile,
	}

	// Acquire observer lock if this is an observe-type link
	if linkType == string(LinkTypeObserve) {
		if err := m.acquireObserverLock(implementerSessionID, reviewSessionID); err != nil {
			return nil, fmt.Errorf("failed to acquire observer lock: %w", err)
		}
	}

	// Store the link
	m.links[key] = link

	// Persist link to disk
	if err := m.persistLink(link); err != nil {
		delete(m.links, key)
		return nil, fmt.Errorf("failed to persist link: %w", err)
	}

	return link, nil
}

// UnlinkSessions removes the link between a review session and an implementer session
func (m *SessionLinkManager) UnlinkSessions(reviewSessionID, implementerSessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := linkKey(reviewSessionID, implementerSessionID)
	link, ok := m.links[key]
	if !ok {
		// Try to load from disk
		var err error
		link, err = m.loadLink(reviewSessionID, implementerSessionID)
		if err != nil {
			return nil // No link exists, nothing to unlink
		}
	}

	// Release observer lock if present
	if link.LinkType == string(LinkTypeObserve) {
		_ = m.releaseObserverLock(implementerSessionID, reviewSessionID)
	}

	// Remove from memory
	delete(m.links, key)

	// Remove from disk
	return m.removePersistedLink(link)
}

// GetLinkedSessions returns all session links involving the given session ID
// (either as reviewer or implementer)
func (m *SessionLinkManager) GetLinkedSessions(sessionID string) []SessionLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []SessionLink

	// Check in-memory links
	for _, link := range m.links {
		if link.ReviewSessionID == sessionID || link.ImplementerSessionID == sessionID {
			result = append(result, *link)
		}
	}

	// Also scan disk for any persisted links not in memory
	diskLinks := m.loadAllLinksForSession(sessionID)
	for _, diskLink := range diskLinks {
		// Check if already in result
		found := false
		for _, r := range result {
			if r.ReviewSessionID == diskLink.ReviewSessionID &&
				r.ImplementerSessionID == diskLink.ImplementerSessionID {
				found = true
				break
			}
		}
		if !found {
			result = append(result, diskLink)
		}
	}

	return result
}

// SendReviewMessage sends a message through the linked review channel
func (m *SessionLinkManager) SendReviewMessage(link *SessionLink, msg ReviewMessage) error {
	if link == nil {
		return errors.New("link is nil")
	}

	// Ensure message has an ID and timestamp
	if msg.ID == "" {
		msg.ID = GenerateID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	// Read current channel
	channel, err := m.readReviewChannel(link.CommunicationFile)
	if err != nil {
		// Initialize new channel if it doesn't exist
		channel = &ReviewChannel{Messages: make([]ReviewMessage, 0)}
	}

	// Append message
	channel.mu.Lock()
	channel.Messages = append(channel.Messages, msg)
	channel.mu.Unlock()

	// Write back
	return m.writeReviewChannel(link.CommunicationFile, channel)
}

// ReadReviewMessages reads messages from the review channel since a given time
func (m *SessionLinkManager) ReadReviewMessages(link *SessionLink, since time.Time) []ReviewMessage {
	if link == nil {
		return nil
	}

	channel, err := m.readReviewChannel(link.CommunicationFile)
	if err != nil {
		return nil
	}

	channel.mu.RLock()
	defer channel.mu.RUnlock()

	var result []ReviewMessage
	for _, msg := range channel.Messages {
		if msg.Timestamp.After(since) || msg.Timestamp.Equal(since) {
			result = append(result, msg)
		}
	}

	return result
}

// GetAllMessages returns all messages in the review channel
func (m *SessionLinkManager) GetAllMessages(link *SessionLink) []ReviewMessage {
	if link == nil {
		return nil
	}

	channel, err := m.readReviewChannel(link.CommunicationFile)
	if err != nil {
		return nil
	}

	channel.mu.RLock()
	defer channel.mu.RUnlock()

	// Return a copy
	result := make([]ReviewMessage, len(channel.Messages))
	copy(result, channel.Messages)
	return result
}

// readReviewChannel reads the review channel from disk
func (m *SessionLinkManager) readReviewChannel(filePath string) (*ReviewChannel, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var channel ReviewChannel
	if err := json.Unmarshal(data, &channel); err != nil {
		return nil, fmt.Errorf("failed to parse review channel: %w", err)
	}

	return &channel, nil
}

// writeReviewChannel writes the review channel to disk
func (m *SessionLinkManager) writeReviewChannel(filePath string, channel *ReviewChannel) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	channel.mu.RLock()
	data, err := json.MarshalIndent(channel, "", "  ")
	channel.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal review channel: %w", err)
	}

	// Write atomically using temp file
	tmpFile := filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpFile, filePath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// acquireObserverLock acquires a read-only observation lock
func (m *SessionLinkManager) acquireObserverLock(targetSessionID, observerSessionID string) error {
	lockPath := m.getObserverLockPath(targetSessionID)

	// Check for existing observers (we allow multiple observers)
	observers, _ := m.readObservers(targetSessionID)

	hostname, _ := os.Hostname()
	newLock := ObserverLock{
		SessionID:  targetSessionID,
		ObserverID: observerSessionID,
		PID:        os.Getpid(),
		Hostname:   hostname,
		StartedAt:  time.Now(),
		ReadOnly:   true,
	}

	// Add to observers list
	observers = append(observers, newLock)

	// Write observers file
	data, err := json.MarshalIndent(observers, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(lockPath, data, 0644)
}

// releaseObserverLock releases the observation lock
func (m *SessionLinkManager) releaseObserverLock(targetSessionID, observerSessionID string) error {
	observers, err := m.readObservers(targetSessionID)
	if err != nil {
		return nil // No observers to release
	}

	// Filter out this observer
	var remaining []ObserverLock
	for _, obs := range observers {
		if obs.ObserverID != observerSessionID {
			remaining = append(remaining, obs)
		}
	}

	lockPath := m.getObserverLockPath(targetSessionID)

	if len(remaining) == 0 {
		// No more observers, remove the file
		return os.Remove(lockPath)
	}

	// Write updated observers
	data, err := json.MarshalIndent(remaining, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(lockPath, data, 0644)
}

// readObservers reads the list of current observers for a session
func (m *SessionLinkManager) readObservers(sessionID string) ([]ObserverLock, error) {
	lockPath := m.getObserverLockPath(sessionID)

	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}

	var observers []ObserverLock
	if err := json.Unmarshal(data, &observers); err != nil {
		return nil, err
	}

	return observers, nil
}

// GetObservers returns the list of active observers for a session
func (m *SessionLinkManager) GetObservers(sessionID string) []ObserverLock {
	observers, err := m.readObservers(sessionID)
	if err != nil {
		return nil
	}

	// Filter out stale observers (process no longer running)
	var active []ObserverLock
	for _, obs := range observers {
		if isProcessAlive(obs.PID) {
			active = append(active, obs)
		}
	}

	return active
}

// HasObservers returns true if the session has any active observers
func (m *SessionLinkManager) HasObservers(sessionID string) bool {
	return len(m.GetObservers(sessionID)) > 0
}

// isProcessAlive checks if a process with the given PID is still running
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't actually send a signal but checks if process exists
	err = process.Signal(syscall.Signal(0))
	// On Unix, if err is nil the process exists
	return err == nil
}

// persistLink saves the link information to disk
func (m *SessionLinkManager) persistLink(link *SessionLink) error {
	// Store in the review session's directory
	linkDir := filepath.Join(m.baseDir, ".claudio", "sessions", link.ReviewSessionID)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return err
	}

	linkFile := filepath.Join(linkDir, "session_link.json")

	// Read existing links for this review session
	var links []*SessionLink
	if data, err := os.ReadFile(linkFile); err == nil {
		_ = json.Unmarshal(data, &links)
	}

	// Check if link already exists
	found := false
	for i, l := range links {
		if l.ImplementerSessionID == link.ImplementerSessionID {
			links[i] = link
			found = true
			break
		}
	}
	if !found {
		links = append(links, link)
	}

	data, err := json.MarshalIndent(links, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(linkFile, data, 0644)
}

// loadLink loads a link from disk
func (m *SessionLinkManager) loadLink(reviewSessionID, implementerSessionID string) (*SessionLink, error) {
	linkFile := filepath.Join(m.baseDir, ".claudio", "sessions", reviewSessionID, "session_link.json")

	data, err := os.ReadFile(linkFile)
	if err != nil {
		return nil, err
	}

	var links []*SessionLink
	if err := json.Unmarshal(data, &links); err != nil {
		return nil, err
	}

	for _, link := range links {
		if link.ImplementerSessionID == implementerSessionID {
			return link, nil
		}
	}

	return nil, errors.New("link not found")
}

// loadAllLinksForSession loads all links involving a session from disk
func (m *SessionLinkManager) loadAllLinksForSession(sessionID string) []SessionLink {
	var result []SessionLink

	// Check if session is a reviewer
	linkFile := filepath.Join(m.baseDir, ".claudio", "sessions", sessionID, "session_link.json")
	if data, err := os.ReadFile(linkFile); err == nil {
		var links []*SessionLink
		if json.Unmarshal(data, &links) == nil {
			for _, link := range links {
				result = append(result, *link)
			}
		}
	}

	// Scan all sessions to find links where this session is the implementer
	sessionsDir := filepath.Join(m.baseDir, ".claudio", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == sessionID {
			continue
		}

		linkFile := filepath.Join(sessionsDir, entry.Name(), "session_link.json")
		data, err := os.ReadFile(linkFile)
		if err != nil {
			continue
		}

		var links []*SessionLink
		if json.Unmarshal(data, &links) != nil {
			continue
		}

		for _, link := range links {
			if link.ImplementerSessionID == sessionID {
				result = append(result, *link)
			}
		}
	}

	return result
}

// removePersistedLink removes the link from disk
func (m *SessionLinkManager) removePersistedLink(link *SessionLink) error {
	linkFile := filepath.Join(m.baseDir, ".claudio", "sessions", link.ReviewSessionID, "session_link.json")

	data, err := os.ReadFile(linkFile)
	if err != nil {
		return nil // File doesn't exist
	}

	var links []*SessionLink
	if err := json.Unmarshal(data, &links); err != nil {
		return nil
	}

	// Filter out this link
	var remaining []*SessionLink
	for _, l := range links {
		if l.ImplementerSessionID != link.ImplementerSessionID {
			remaining = append(remaining, l)
		}
	}

	if len(remaining) == 0 {
		return os.Remove(linkFile)
	}

	newData, err := json.MarshalIndent(remaining, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(linkFile, newData, 0644)
}

// SessionLinkWatcher watches for changes to the review channel and notifies via callback
type SessionLinkWatcher struct {
	watcher     *fsnotify.Watcher
	link        *SessionLink
	onMessage   func([]ReviewMessage)
	lastRead    time.Time
	manager     *SessionLinkManager
	stopCh      chan struct{}
	mu          sync.Mutex
}

// NewSessionLinkWatcher creates a new watcher for review channel messages
func NewSessionLinkWatcher(manager *SessionLinkManager, link *SessionLink, onMessage func([]ReviewMessage)) (*SessionLinkWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	slw := &SessionLinkWatcher{
		watcher:   watcher,
		link:      link,
		onMessage: onMessage,
		lastRead:  time.Now(),
		manager:   manager,
		stopCh:    make(chan struct{}),
	}

	// Watch the communication file's directory (fsnotify works better with directories)
	dir := filepath.Dir(link.CommunicationFile)
	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch directory: %w", err)
	}

	return slw, nil
}

// Start begins watching for new messages
func (w *SessionLinkWatcher) Start() {
	go w.watchLoop()
}

// Stop stops the watcher
func (w *SessionLinkWatcher) Stop() {
	close(w.stopCh)
	w.watcher.Close()
}

// watchLoop processes filesystem events for the review channel
func (w *SessionLinkWatcher) watchLoop() {
	targetFile := filepath.Base(w.link.CommunicationFile)
	debounceTimer := time.NewTimer(0)
	<-debounceTimer.C // drain initial timer

	for {
		select {
		case <-w.stopCh:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only care about writes to the review channel file
			if filepath.Base(event.Name) != targetFile {
				continue
			}

			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			// Debounce to avoid multiple rapid notifications
			debounceTimer.Reset(100 * time.Millisecond)

		case <-debounceTimer.C:
			// Read new messages
			w.mu.Lock()
			messages := w.manager.ReadReviewMessages(w.link, w.lastRead)
			if len(messages) > 0 {
				w.lastRead = messages[len(messages)-1].Timestamp
				if w.onMessage != nil {
					w.onMessage(messages)
				}
			}
			w.mu.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log error but continue
			_ = err
		}
	}
}

// GenerateContextMarkdownForLink generates context markdown section for a session link
func GenerateContextMarkdownForLink(link *SessionLink, messages []ReviewMessage) string {
	var sb strings.Builder

	sb.WriteString("## Code Review Session\n\n")
	sb.WriteString(fmt.Sprintf("- **Review Session ID**: %s\n", link.ReviewSessionID))
	sb.WriteString(fmt.Sprintf("- **Implementer Session ID**: %s\n", link.ImplementerSessionID))
	sb.WriteString(fmt.Sprintf("- **Link Type**: %s\n", link.LinkType))
	sb.WriteString(fmt.Sprintf("- **Created**: %s\n", link.CreatedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Communication File**: %s\n", link.CommunicationFile))
	sb.WriteString("\n")

	if len(messages) > 0 {
		sb.WriteString("### Recent Review Messages\n\n")

		// Show last 10 messages
		start := 0
		if len(messages) > 10 {
			start = len(messages) - 10
		}

		for _, msg := range messages[start:] {
			sb.WriteString(fmt.Sprintf("**[%s]** %s (%s):\n", msg.Type, msg.From, msg.Timestamp.Format("15:04:05")))
			sb.WriteString(fmt.Sprintf("> %s\n\n", msg.Content))
		}
	}

	return sb.String()
}

// NewReviewMessage creates a new review message with auto-generated ID and timestamp
func NewReviewMessage(from string, msgType ReviewMessageType, content string) ReviewMessage {
	return ReviewMessage{
		ID:        GenerateID(),
		From:      from,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}
}

// NewReviewMessageWithIssue creates a new review message linked to a specific issue
func NewReviewMessageWithIssue(from string, msgType ReviewMessageType, content, issueRef string) ReviewMessage {
	msg := NewReviewMessage(from, msgType, content)
	msg.IssueRef = issueRef
	return msg
}

package mailbox

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// mailboxDir is the directory name within a session directory that holds mailboxes.
	mailboxDir = "mailbox"

	// indexFile is the append-only JSONL file within each mailbox directory.
	indexFile = "index.jsonl"
)

// Store provides file-based mailbox storage with atomic writes.
// Messages are persisted as JSONL (one JSON object per line) in an append-only log.
type Store struct {
	sessionDir string
	mu         sync.Mutex
}

// NewStore creates a Store rooted at the given session directory.
// The directory structure is created lazily on first write.
func NewStore(sessionDir string) *Store {
	return &Store{sessionDir: sessionDir}
}

// Send persists a message to the appropriate mailbox directory.
// If msg.ID is empty, a unique ID is generated. If msg.Timestamp is zero, the
// current time is used. Writes are serialized via a mutex and use O_APPEND.
func (s *Store) Send(msg Message) error {
	if msg.From == "" {
		return fmt.Errorf("mailbox: message From field is required")
	}
	if msg.To == "" {
		return fmt.Errorf("mailbox: message To field is required")
	}
	if msg.Type == "" {
		return fmt.Errorf("mailbox: message Type field is required")
	}

	if msg.ID == "" {
		msg.ID = generateID()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	dir := s.dirForRecipient(msg.To)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mailbox: create directory: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("mailbox: marshal message: %w", err)
	}
	data = append(data, '\n')

	return s.atomicAppend(filepath.Join(dir, indexFile), data)
}

// ReadBroadcast returns all messages from the broadcast mailbox.
func (s *Store) ReadBroadcast() ([]Message, error) {
	return s.readIndex(s.dirForRecipient(BroadcastRecipient))
}

// ReadForInstance returns all messages targeted at a specific instance.
func (s *Store) ReadForInstance(instanceID string) ([]Message, error) {
	if instanceID == "" {
		return nil, fmt.Errorf("mailbox: instanceID is required")
	}
	return s.readIndex(s.dirForRecipient(instanceID))
}

// ReadAll returns all messages across broadcast and a specific instance mailbox,
// sorted chronologically by timestamp.
func (s *Store) ReadAll(instanceID string) ([]Message, error) {
	broadcast, err := s.ReadBroadcast()
	if err != nil {
		return nil, err
	}

	targeted, err := s.ReadForInstance(instanceID)
	if err != nil {
		return nil, err
	}

	all := make([]Message, 0, len(broadcast)+len(targeted))
	all = append(all, broadcast...)
	all = append(all, targeted...)

	sortMessages(all)
	return all, nil
}

// dirForRecipient returns the mailbox directory for a given recipient.
func (s *Store) dirForRecipient(recipient string) string {
	return filepath.Join(s.sessionDir, mailboxDir, recipient)
}

// readIndex reads all messages from an index.jsonl file.
// Returns nil (not error) if the file does not exist.
func (s *Store) readIndex(dir string) ([]Message, error) {
	path := filepath.Join(dir, indexFile)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("mailbox: open index: %w", err)
	}
	defer func() { _ = f.Close() }()

	var messages []Message
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip malformed lines rather than failing entirely
			continue
		}
		messages = append(messages, msg)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("mailbox: scan index: %w", err)
	}

	return messages, nil
}

// atomicAppend appends data to a file under a mutex to serialize writes.
// Each JSONL line is small enough that O_APPEND provides atomicity guarantees
// on POSIX systems (writes under PIPE_BUF are atomic).
func (s *Store) atomicAppend(path string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("mailbox: open index for append: %w", err)
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("mailbox: append to index: %w", err)
	}

	return f.Close()
}

// idCounter provides per-process uniqueness for message IDs.
var idCounter atomic.Uint64

// generateID produces a unique message ID using timestamp, PID, and atomic counter.
func generateID() string {
	return fmt.Sprintf("msg-%d-%d-%d", time.Now().UnixNano(), os.Getpid(), idCounter.Add(1))
}

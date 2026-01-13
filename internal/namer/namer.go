package namer

import (
	"context"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/logging"
)

const (
	// pendingQueueSize is the buffer size for pending rename requests.
	pendingQueueSize = 20

	// processInterval is the minimum time between processing requests.
	// This provides natural rate limiting.
	processInterval = 500 * time.Millisecond
)

// RenameCallback is invoked when an instance name is successfully generated.
type RenameCallback func(instanceID, newName string)

// renameRequest represents a pending rename operation.
type renameRequest struct {
	InstanceID string
	Task       string
	Output     string
}

// Namer manages intelligent instance renaming using LLM summarization.
// It processes rename requests in a background goroutine to avoid blocking.
type Namer struct {
	client   Client
	logger   *logging.Logger
	callback RenameCallback

	// renamed tracks which instances have already been renamed
	renamed map[string]bool

	pending chan renameRequest
	done    chan struct{}
	started bool // prevents double-start
	mu      sync.RWMutex
}

// New creates a new Namer service.
// Panics if client is nil (programmer error).
func New(client Client, logger *logging.Logger) *Namer {
	if client == nil {
		panic("namer: client is required")
	}
	return &Namer{
		client:  client,
		logger:  logger,
		renamed: make(map[string]bool),
		pending: make(chan renameRequest, pendingQueueSize),
		done:    make(chan struct{}),
	}
}

// OnRename sets the callback invoked when a name is successfully generated.
func (n *Namer) OnRename(cb RenameCallback) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.callback = cb
}

// Start begins the background rename processor.
// Safe to call multiple times - subsequent calls are no-ops.
func (n *Namer) Start() {
	n.mu.Lock()
	if n.started {
		n.mu.Unlock()
		return
	}
	n.started = true
	n.mu.Unlock()
	go n.processLoop()
}

// Stop gracefully shuts down the namer service.
// Safe to call multiple times - subsequent calls are no-ops.
func (n *Namer) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()
	select {
	case <-n.done:
		// Already stopped
		return
	default:
		close(n.done)
	}
}

// RequestRename queues an instance for renaming.
// This is non-blocking; if the queue is full, the request is dropped.
func (n *Namer) RequestRename(instanceID, task, output string) {
	// Skip if already renamed
	n.mu.RLock()
	if n.renamed[instanceID] {
		n.mu.RUnlock()
		return
	}
	n.mu.RUnlock()

	// Non-blocking send - drop if queue is full
	select {
	case n.pending <- renameRequest{
		InstanceID: instanceID,
		Task:       task,
		Output:     output,
	}:
	default:
		if n.logger != nil {
			n.logger.Warn("rename queue full, request dropped - instance will keep original name",
				"instance_id", instanceID)
		}
	}
}

// IsRenamed returns true if the instance has already been renamed.
func (n *Namer) IsRenamed(instanceID string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.renamed[instanceID]
}

// Reset clears the renamed state for an instance, allowing re-renaming.
// Useful when an instance is restarted.
func (n *Namer) Reset(instanceID string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	delete(n.renamed, instanceID)
}

// processLoop runs in the background, processing rename requests.
func (n *Namer) processLoop() {
	ticker := time.NewTicker(processInterval)
	defer ticker.Stop()

	for {
		select {
		case <-n.done:
			return
		case req := <-n.pending:
			n.processRename(req)
		case <-ticker.C:
			// Periodic tick to drain any backed up requests
			// This prevents starvation if requests come faster than processInterval
			select {
			case req := <-n.pending:
				n.processRename(req)
			default:
				// Nothing pending
			}
		}
	}
}

// processRename handles a single rename request.
func (n *Namer) processRename(req renameRequest) {
	// Double-check renamed status (request may have been queued before marking)
	n.mu.RLock()
	if n.renamed[req.InstanceID] {
		n.mu.RUnlock()
		return
	}
	n.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	name, err := n.client.Summarize(ctx, req.Task, req.Output)
	if err != nil {
		if n.logger != nil {
			n.logger.Warn("failed to generate instance name",
				"instance_id", req.InstanceID,
				"error", err.Error())
		}
		// Mark as renamed anyway to avoid retrying indefinitely
		n.mu.Lock()
		n.renamed[req.InstanceID] = true
		n.mu.Unlock()
		return
	}

	// Mark as renamed
	n.mu.Lock()
	n.renamed[req.InstanceID] = true
	callback := n.callback
	n.mu.Unlock()

	if n.logger != nil {
		n.logger.Debug("generated instance name",
			"instance_id", req.InstanceID,
			"name", name)
	}

	// Invoke callback
	if callback != nil {
		callback(req.InstanceID, name)
	}
}

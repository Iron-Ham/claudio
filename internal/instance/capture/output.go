package capture

import (
	"log"
	"os/exec"
	"sync"
	"time"
)

const (
	// maxConsecutiveCaptureFailures is the threshold after which capture errors are logged.
	maxConsecutiveCaptureFailures = 10
)

// OutputCapture defines the interface for capturing process output.
// Implementations handle the mechanics of reading output from a process
// (e.g., from a tmux pane) independently of state detection or metrics parsing.
type OutputCapture interface {
	// ReadOutput returns the currently captured output.
	// The returned bytes represent the current buffer contents.
	ReadOutput() ([]byte, error)

	// Start begins the output capture process.
	// This starts any background goroutines needed for continuous capture.
	Start() error

	// Stop ends the output capture process.
	// This stops any background goroutines and releases resources.
	Stop() error

	// Clear resets the output buffer, discarding all captured data.
	Clear()
}

// TmuxCaptureConfig holds configuration for TmuxCapture.
type TmuxCaptureConfig struct {
	// SessionName is the tmux session to capture from.
	SessionName string

	// BufferSize is the size of the ring buffer in bytes.
	BufferSize int

	// CaptureInterval is how often to poll for new output.
	CaptureInterval time.Duration
}

// DefaultTmuxCaptureConfig returns a configuration with sensible defaults.
func DefaultTmuxCaptureConfig(sessionName string) TmuxCaptureConfig {
	return TmuxCaptureConfig{
		SessionName:     sessionName,
		BufferSize:      100000, // 100KB
		CaptureInterval: 100 * time.Millisecond,
	}
}

// TmuxCapture implements OutputCapture by polling a tmux pane.
// It captures the visible pane content plus scrollback at regular intervals,
// storing output in a ring buffer for bounded memory usage.
//
// Thread Safety: All public methods are safe for concurrent use.
type TmuxCapture struct {
	config TmuxCaptureConfig
	buffer *RingBuffer
	mu     sync.RWMutex

	// Capture loop control
	running  bool
	doneChan chan struct{}
	ticker   *time.Ticker

	// Error tracking for capture loop
	consecutiveFailures int

	// Change callback (optional)
	onChange func(output []byte)
}

// NewTmuxCapture creates a new TmuxCapture with the given configuration.
func NewTmuxCapture(config TmuxCaptureConfig) *TmuxCapture {
	return &TmuxCapture{
		config:   config,
		buffer:   NewRingBuffer(config.BufferSize),
		doneChan: make(chan struct{}),
	}
}

// SetOnChange registers a callback that is invoked when output changes.
// This allows components to react to output changes without polling.
func (t *TmuxCapture) SetOnChange(callback func(output []byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onChange = callback
}

// ReadOutput returns the currently captured output.
func (t *TmuxCapture) ReadOutput() ([]byte, error) {
	return t.buffer.Bytes(), nil
}

// Start begins the background capture loop.
func (t *TmuxCapture) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return nil // Already running
	}

	t.running = true
	t.doneChan = make(chan struct{})
	t.ticker = time.NewTicker(t.config.CaptureInterval)

	go t.captureLoop()
	return nil
}

// Stop ends the background capture loop.
func (t *TmuxCapture) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}

	t.running = false
	if t.ticker != nil {
		t.ticker.Stop()
	}

	// Signal done and wait for loop to exit
	select {
	case <-t.doneChan:
		// Already closed
	default:
		close(t.doneChan)
	}
	t.mu.Unlock()

	return nil
}

// Clear resets the output buffer.
func (t *TmuxCapture) Clear() {
	t.buffer.Reset()
}

// captureLoop periodically captures output from the tmux session.
func (t *TmuxCapture) captureLoop() {
	var lastOutput string

	for {
		select {
		case <-t.doneChan:
			return
		case <-t.ticker.C:
			t.mu.RLock()
			if !t.running {
				t.mu.RUnlock()
				continue
			}
			sessionName := t.config.SessionName
			callback := t.onChange
			t.mu.RUnlock()

			// Capture the entire visible pane plus scrollback
			output, err := captureTmuxPane(sessionName)
			if err != nil {
				t.mu.Lock()
				t.consecutiveFailures++
				failures := t.consecutiveFailures
				t.mu.Unlock()

				// Log on first failure and when threshold is reached
				switch failures {
				case 1:
					log.Printf("WARNING: tmux capture failed for session %s: %v", sessionName, err)
				case maxConsecutiveCaptureFailures:
					log.Printf("ERROR: tmux capture has failed %d consecutive times for session %s: %v",
						failures, sessionName, err)
				}
				continue
			}

			// Reset failure counter on success
			t.mu.Lock()
			t.consecutiveFailures = 0
			t.mu.Unlock()

			// Only update if content changed
			currentOutput := string(output)
			if currentOutput != lastOutput {
				t.buffer.Reset()
				_, _ = t.buffer.Write(output)
				lastOutput = currentOutput

				// Notify callback if registered
				if callback != nil {
					callback(output)
				}
			}
		}
	}
}

// captureTmuxPane captures the full pane content from a tmux session.
// This includes visible content and scrollback, with ANSI escape sequences preserved.
func captureTmuxPane(sessionName string) ([]byte, error) {
	cmd := exec.Command("tmux",
		"capture-pane",
		"-t", sessionName,
		"-p",      // print to stdout
		"-e",      // preserve escape sequences (colors)
		"-S", "-", // start from beginning of scrollback
		"-E", "-", // end at bottom of scrollback
	)
	return cmd.Output()
}

// SessionExists checks if the tmux session exists.
func (t *TmuxCapture) SessionExists() bool {
	cmd := exec.Command("tmux", "has-session", "-t", t.config.SessionName)
	return cmd.Run() == nil
}

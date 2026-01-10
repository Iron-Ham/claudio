// Package instance provides instance management for Claude Code sessions.
package instance

import (
	"context"
	"sync"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/capture"
	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/instance/process"
)

// Facade provides a high-level interface for managing a Claude Code instance.
// It composes the lower-level process, capture, and detect packages into a
// unified interface that mirrors the Manager's functionality.
//
// The Facade is designed to eventually replace the direct tmux interactions
// in Manager, providing better separation of concerns and testability.
type Facade struct {
	proc     process.Process
	buffer   *capture.RingBuffer
	detector *detect.Detector

	mu           sync.RWMutex
	config       FacadeConfig
	currentState detect.WaitingState
	lastOutput   []byte

	// Callbacks
	onStateChange  func(state detect.WaitingState)
	onMetrics      func(metrics *ParsedMetrics)
	onTimeout      func(timeoutType string)
	onBell         func()

	// Monitoring
	monitorTicker  *time.Ticker
	monitorStop    chan struct{}
	monitorRunning bool
}

// FacadeConfig holds configuration for the Facade.
type FacadeConfig struct {
	// Process configuration
	ProcessConfig process.Config

	// Buffer size for output capture (default: 1MB)
	BufferSize int

	// How often to check for state changes (default: 100ms)
	MonitorInterval time.Duration

	// Timeout settings
	ActivityTimeout   time.Duration
	CompletionTimeout time.Duration
	StaleOutputTime   time.Duration
}

// DefaultFacadeConfig returns sensible defaults for Facade configuration.
func DefaultFacadeConfig() FacadeConfig {
	return FacadeConfig{
		ProcessConfig:     process.DefaultConfig(),
		BufferSize:        1024 * 1024, // 1MB
		MonitorInterval:   100 * time.Millisecond,
		ActivityTimeout:   5 * time.Minute,
		CompletionTimeout: 30 * time.Second,
		StaleOutputTime:   2 * time.Minute,
	}
}

// Note: ParsedMetrics is defined in metrics.go - we reuse that type here.

// NewFacade creates a new Facade with the given configuration.
// It initializes the underlying process, buffer, and detector components.
func NewFacade(config FacadeConfig) *Facade {
	// Create the underlying process
	proc := process.NewTmuxProcess(config.ProcessConfig)

	// Create the ring buffer for output capture
	bufferSize := config.BufferSize
	if bufferSize <= 0 {
		bufferSize = 1024 * 1024
	}
	buffer := capture.NewRingBuffer(bufferSize)

	// Create the detector
	detector := detect.NewDetector()

	return &Facade{
		proc:        proc,
		buffer:      buffer,
		detector:    detector,
		config:      config,
		monitorStop: make(chan struct{}),
	}
}

// SetCallbacks sets the callback functions for various events.
func (f *Facade) SetCallbacks(
	onStateChange func(detect.WaitingState),
	onMetrics func(*ParsedMetrics),
	onTimeout func(string),
	onBell func(),
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.onStateChange = onStateChange
	f.onMetrics = onMetrics
	f.onTimeout = onTimeout
	f.onBell = onBell
}

// Start launches the Claude Code process.
func (f *Facade) Start(ctx context.Context) error {
	if err := f.proc.Start(ctx); err != nil {
		return err
	}

	// Start the monitoring loop
	f.startMonitoring()

	return nil
}

// Stop terminates the process and monitoring.
func (f *Facade) Stop() error {
	// Stop monitoring first
	f.stopMonitoring()

	// Then stop the process
	return f.proc.Stop()
}

// IsRunning returns whether the process is currently running.
func (f *Facade) IsRunning() bool {
	return f.proc.IsRunning()
}

// SendInput sends input to the running process.
func (f *Facade) SendInput(input string) error {
	return f.proc.SendInput(input)
}

// GetOutput returns the captured output.
func (f *Facade) GetOutput() []byte {
	return f.buffer.Bytes()
}

// GetState returns the current detected state.
func (f *Facade) GetState() detect.WaitingState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.currentState
}

// Wait blocks until the process exits.
func (f *Facade) Wait() error {
	return f.proc.Wait()
}

// Resize changes the terminal dimensions if the process supports it.
func (f *Facade) Resize(width, height int) error {
	if resizable, ok := f.proc.(process.Resizable); ok {
		return resizable.Resize(width, height)
	}
	return nil
}

// Reconnect attempts to reconnect to an existing session.
func (f *Facade) Reconnect() error {
	if reconnectable, ok := f.proc.(process.Reconnectable); ok {
		err := reconnectable.Reconnect()
		if err != nil {
			return err
		}
		// Restart monitoring after reconnect
		f.startMonitoring()
		return nil
	}
	return process.ErrSessionNotFound
}

// SessionExists checks if the session exists.
func (f *Facade) SessionExists() bool {
	if reconnectable, ok := f.proc.(process.Reconnectable); ok {
		return reconnectable.SessionExists()
	}
	return false
}

// startMonitoring begins the background monitoring loop.
func (f *Facade) startMonitoring() {
	f.mu.Lock()
	if f.monitorRunning {
		f.mu.Unlock()
		return
	}

	interval := f.config.MonitorInterval
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	f.monitorTicker = time.NewTicker(interval)
	f.monitorStop = make(chan struct{})
	f.monitorRunning = true
	f.mu.Unlock()

	go f.monitorLoop()
}

// stopMonitoring stops the background monitoring loop.
func (f *Facade) stopMonitoring() {
	f.mu.Lock()
	if !f.monitorRunning {
		f.mu.Unlock()
		return
	}

	close(f.monitorStop)
	f.monitorTicker.Stop()
	f.monitorRunning = false
	f.mu.Unlock()
}

// monitorLoop is the background loop that checks for output and state changes.
func (f *Facade) monitorLoop() {
	for {
		select {
		case <-f.monitorStop:
			return
		case <-f.monitorTicker.C:
			f.captureAndAnalyze()
		}
	}
}

// captureAndAnalyze captures output and analyzes it for state changes.
func (f *Facade) captureAndAnalyze() {
	// Get output from the process if it supports OutputProvider
	if provider, ok := f.proc.(process.OutputProvider); ok {
		output := provider.GetOutput()
		if len(output) > 0 {
			// Write to ring buffer
			_, _ = f.buffer.Write(output)

			// Analyze for state
			state := f.detector.Detect(output)

			f.mu.Lock()
			oldState := f.currentState
			f.currentState = state
			f.lastOutput = output
			onStateChange := f.onStateChange
			f.mu.Unlock()

			// Notify on state change
			if oldState != state && onStateChange != nil {
				onStateChange(state)
			}
		}
	}
}

// Process returns the underlying process (for advanced use cases).
func (f *Facade) Process() process.Process {
	return f.proc
}

// Buffer returns the ring buffer (for advanced use cases).
func (f *Facade) Buffer() *capture.RingBuffer {
	return f.buffer
}

// Detector returns the detector (for advanced use cases).
func (f *Facade) Detector() *detect.Detector {
	return f.detector
}

package capture

import (
	"sync"
	"testing"
	"time"
)

func TestTmuxCaptureConfig_Defaults(t *testing.T) {
	cfg := DefaultTmuxCaptureConfig("test-session")

	if cfg.SessionName != "test-session" {
		t.Errorf("Expected SessionName 'test-session', got %q", cfg.SessionName)
	}

	if cfg.BufferSize != 100000 {
		t.Errorf("Expected BufferSize 100000, got %d", cfg.BufferSize)
	}

	if cfg.CaptureInterval != 100*time.Millisecond {
		t.Errorf("Expected CaptureInterval 100ms, got %v", cfg.CaptureInterval)
	}
}

func TestTmuxCapture_New(t *testing.T) {
	cfg := TmuxCaptureConfig{
		SessionName:     "test-session",
		BufferSize:      1024,
		CaptureInterval: 50 * time.Millisecond,
	}

	capture := NewTmuxCapture(cfg)

	if capture == nil {
		t.Fatal("NewTmuxCapture returned nil")
	}

	if capture.config.SessionName != "test-session" {
		t.Errorf("Expected SessionName 'test-session', got %q", capture.config.SessionName)
	}

	if capture.buffer == nil {
		t.Error("Buffer should be initialized")
	}
}

func TestTmuxCapture_ReadOutput_Empty(t *testing.T) {
	cfg := DefaultTmuxCaptureConfig("test-session")
	capture := NewTmuxCapture(cfg)

	output, err := capture.ReadOutput()
	if err != nil {
		t.Fatalf("ReadOutput failed: %v", err)
	}

	if len(output) != 0 {
		t.Errorf("Expected empty output, got %d bytes", len(output))
	}
}

func TestTmuxCapture_Clear(t *testing.T) {
	cfg := DefaultTmuxCaptureConfig("test-session")
	capture := NewTmuxCapture(cfg)

	// Write some data directly to buffer
	_, _ = capture.buffer.Write([]byte("test data"))

	output, _ := capture.ReadOutput()
	if len(output) == 0 {
		t.Fatal("Expected data in buffer before clear")
	}

	capture.Clear()

	output, _ = capture.ReadOutput()
	if len(output) != 0 {
		t.Errorf("Expected empty output after clear, got %d bytes", len(output))
	}
}

func TestTmuxCapture_StartStop(t *testing.T) {
	cfg := TmuxCaptureConfig{
		SessionName:     "nonexistent-session",
		BufferSize:      1024,
		CaptureInterval: 10 * time.Millisecond,
	}
	capture := NewTmuxCapture(cfg)

	// Start should succeed (even without a real tmux session)
	if err := capture.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Should be running
	capture.mu.RLock()
	running := capture.running
	capture.mu.RUnlock()
	if !running {
		t.Error("Expected capture to be running after Start")
	}

	// Stop should succeed
	if err := capture.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Should not be running
	capture.mu.RLock()
	running = capture.running
	capture.mu.RUnlock()
	if running {
		t.Error("Expected capture to not be running after Stop")
	}
}

func TestTmuxCapture_DoubleStart(t *testing.T) {
	cfg := TmuxCaptureConfig{
		SessionName:     "test-session",
		BufferSize:      1024,
		CaptureInterval: 10 * time.Millisecond,
	}
	capture := NewTmuxCapture(cfg)

	// First start
	if err := capture.Start(); err != nil {
		t.Fatalf("First Start failed: %v", err)
	}
	defer capture.Stop()

	// Second start should be idempotent
	if err := capture.Start(); err != nil {
		t.Errorf("Second Start should not fail: %v", err)
	}
}

func TestTmuxCapture_DoubleStop(t *testing.T) {
	cfg := TmuxCaptureConfig{
		SessionName:     "test-session",
		BufferSize:      1024,
		CaptureInterval: 10 * time.Millisecond,
	}
	capture := NewTmuxCapture(cfg)

	if err := capture.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// First stop
	if err := capture.Stop(); err != nil {
		t.Fatalf("First Stop failed: %v", err)
	}

	// Second stop should be idempotent
	if err := capture.Stop(); err != nil {
		t.Errorf("Second Stop should not fail: %v", err)
	}
}

func TestTmuxCapture_SetOnChange(t *testing.T) {
	cfg := TmuxCaptureConfig{
		SessionName:     "test-session",
		BufferSize:      1024,
		CaptureInterval: 10 * time.Millisecond,
	}
	capture := NewTmuxCapture(cfg)

	var mu sync.Mutex
	callCount := 0

	capture.SetOnChange(func(output []byte) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	// Verify callback was set
	capture.mu.RLock()
	hasCallback := capture.onChange != nil
	capture.mu.RUnlock()

	if !hasCallback {
		t.Error("Expected onChange callback to be set")
	}
}

func TestTmuxCapture_Concurrent(t *testing.T) {
	cfg := TmuxCaptureConfig{
		SessionName:     "test-session",
		BufferSize:      1024,
		CaptureInterval: 5 * time.Millisecond,
	}
	capture := NewTmuxCapture(cfg)

	if err := capture.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for range 10 {
		wg.Go(func() {
			for range 100 {
				_, _ = capture.ReadOutput()
			}
		})
	}

	// Concurrent clears
	for range 5 {
		wg.Go(func() {
			for range 50 {
				capture.Clear()
			}
		})
	}

	wg.Wait()

	if err := capture.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestOutputCapture_Interface(t *testing.T) {
	cfg := DefaultTmuxCaptureConfig("test-session")

	// Verify TmuxCapture implements OutputCapture
	var _ OutputCapture = NewTmuxCapture(cfg)
}

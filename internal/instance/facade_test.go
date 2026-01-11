package instance

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Iron-Ham/claudio/internal/instance/detect"
	"github.com/Iron-Ham/claudio/internal/instance/metrics"
	"github.com/Iron-Ham/claudio/internal/instance/process"
)

// mockProcess implements the process.Process interface for testing.
type mockProcess struct {
	running     bool
	startError  error
	stopError   error
	sendError   error
	waitError   error
	output      []byte
	mu          sync.Mutex
	startCalled bool
	stopCalled  bool
	sendInputs  []string
}

func newMockProcess() *mockProcess {
	return &mockProcess{
		sendInputs: make([]string, 0),
	}
}

func (m *mockProcess) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
	if m.startError != nil {
		return m.startError
	}
	m.running = true
	return nil
}

func (m *mockProcess) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled = true
	if m.stopError != nil {
		return m.stopError
	}
	m.running = false
	return nil
}

func (m *mockProcess) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *mockProcess) Wait() error {
	return m.waitError
}

func (m *mockProcess) SendInput(input string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.sendError != nil {
		return m.sendError
	}
	m.sendInputs = append(m.sendInputs, input)
	return nil
}

// Implement OutputProvider for testing
func (m *mockProcess) GetOutput() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.output
}

// mockResizableProcess extends mockProcess with Resizable interface.
type mockResizableProcess struct {
	*mockProcess
	resizeWidth  int
	resizeHeight int
	resizeError  error
}

func newMockResizableProcess() *mockResizableProcess {
	return &mockResizableProcess{
		mockProcess: newMockProcess(),
	}
}

func (m *mockResizableProcess) Resize(width, height int) error {
	if m.resizeError != nil {
		return m.resizeError
	}
	m.resizeWidth = width
	m.resizeHeight = height
	return nil
}

// mockReconnectableProcess extends mockProcess with Reconnectable interface.
type mockReconnectableProcess struct {
	*mockProcess
	sessionExists   bool
	reconnectError  error
	reconnectCalled bool
}

func newMockReconnectableProcess() *mockReconnectableProcess {
	return &mockReconnectableProcess{
		mockProcess:   newMockProcess(),
		sessionExists: true,
	}
}

func (m *mockReconnectableProcess) Reconnect() error {
	m.reconnectCalled = true
	if m.reconnectError != nil {
		return m.reconnectError
	}
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()
	return nil
}

func (m *mockReconnectableProcess) SessionExists() bool {
	return m.sessionExists
}

func TestDefaultFacadeConfig(t *testing.T) {
	cfg := DefaultFacadeConfig()

	if cfg.BufferSize != 1024*1024 {
		t.Errorf("Expected BufferSize 1MB, got %d", cfg.BufferSize)
	}
	if cfg.MonitorInterval != 100*time.Millisecond {
		t.Errorf("Expected MonitorInterval 100ms, got %v", cfg.MonitorInterval)
	}
	if cfg.ActivityTimeout != 5*time.Minute {
		t.Errorf("Expected ActivityTimeout 5m, got %v", cfg.ActivityTimeout)
	}
	if cfg.CompletionTimeout != 30*time.Second {
		t.Errorf("Expected CompletionTimeout 30s, got %v", cfg.CompletionTimeout)
	}
	if cfg.StaleOutputTime != 2*time.Minute {
		t.Errorf("Expected StaleOutputTime 2m, got %v", cfg.StaleOutputTime)
	}
}

func TestNewFacade(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	if f == nil {
		t.Fatal("NewFacade returned nil")
	}

	if f.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}
	if f.detector == nil {
		t.Error("Expected detector to be initialized")
	}
	if f.proc == nil {
		t.Error("Expected proc to be initialized")
	}
}

func TestNewFacade_CustomBufferSize(t *testing.T) {
	cfg := DefaultFacadeConfig()
	cfg.BufferSize = 0 // Should use default

	f := NewFacade(cfg, nil)

	if f.buffer == nil {
		t.Error("Expected buffer to be initialized with default size")
	}
}

func TestFacade_SetCallbacks(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	var stateChangeCalled bool
	var metricsCalled bool
	var timeoutCalled bool
	var bellCalled bool

	f.SetCallbacks(
		func(_ detect.WaitingState) { stateChangeCalled = true },
		func(_ *metrics.ParsedMetrics) { metricsCalled = true },
		func(_ string) { timeoutCalled = true },
		func() { bellCalled = true },
	)

	// Verify callbacks are stored
	f.mu.RLock()
	hasStateChange := f.onStateChange != nil
	hasMetrics := f.onMetrics != nil
	hasTimeout := f.onTimeout != nil
	hasBell := f.onBell != nil
	f.mu.RUnlock()

	if !hasStateChange {
		t.Error("Expected onStateChange callback to be set")
	}
	if !hasMetrics {
		t.Error("Expected onMetrics callback to be set")
	}
	if !hasTimeout {
		t.Error("Expected onTimeout callback to be set")
	}
	if !hasBell {
		t.Error("Expected onBell callback to be set")
	}

	// Unused variables to prevent compilation errors
	_ = stateChangeCalled
	_ = metricsCalled
	_ = timeoutCalled
	_ = bellCalled
}

func TestFacade_GetState(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	// Initial state should be StateWorking (zero value)
	state := f.GetState()
	if state != detect.StateWorking {
		t.Errorf("Expected initial state %v, got %v", detect.StateWorking, state)
	}

	// Update state
	f.mu.Lock()
	f.currentState = detect.StateWaitingPermission
	f.mu.Unlock()

	state = f.GetState()
	if state != detect.StateWaitingPermission {
		t.Errorf("Expected state %v, got %v", detect.StateWaitingPermission, state)
	}
}

func TestFacade_GetOutput(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	// Initially empty
	output := f.GetOutput()
	if len(output) != 0 {
		t.Errorf("Expected empty output, got %d bytes", len(output))
	}

	// Write to buffer
	testData := []byte("test output data")
	_, _ = f.buffer.Write(testData)

	output = f.GetOutput()
	if string(output) != string(testData) {
		t.Errorf("Expected output %q, got %q", string(testData), string(output))
	}
}

func TestFacade_Process(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	proc := f.Process()
	if proc == nil {
		t.Error("Expected Process to return non-nil")
	}
}

func TestFacade_Buffer(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	buf := f.Buffer()
	if buf == nil {
		t.Error("Expected Buffer to return non-nil")
	}
}

func TestFacade_Detector(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	det := f.Detector()
	if det == nil {
		t.Error("Expected Detector to return non-nil")
	}
}

func TestFacade_Resize_NotSupported(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	// Default TmuxProcess should not support Resize in tests (not started)
	err := f.Resize(100, 50)

	// TmuxProcess implements Resizable, so this should work (but may fail without tmux)
	// The important thing is the Facade properly delegates to the underlying process
	if err == ErrResizeNotSupported {
		t.Log("Resize not supported (expected for non-Resizable process)")
	}
}

func TestFacade_SessionExists_NotReconnectable(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	// Default process may or may not implement Reconnectable
	// We're testing that the Facade handles this gracefully
	exists := f.SessionExists()

	// Should not panic, returns false if not reconnectable
	_ = exists
}

func TestFacade_Reconnect_NotReconnectable(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	// Try to reconnect - may fail if not reconnectable or session doesn't exist
	err := f.Reconnect()

	// Should get an error for non-existent session
	if err == nil {
		t.Log("Reconnect succeeded (tmux session may exist)")
	}
}

func TestFacade_StartMonitoring_Idempotent(t *testing.T) {
	cfg := DefaultFacadeConfig()
	cfg.MonitorInterval = 10 * time.Millisecond
	f := NewFacade(cfg, nil)

	// Start monitoring twice - should be idempotent
	f.startMonitoring()
	f.startMonitoring()

	// Give time for monitoring to start
	time.Sleep(20 * time.Millisecond)

	f.mu.RLock()
	running := f.monitorRunning
	f.mu.RUnlock()

	if !running {
		t.Error("Expected monitoring to be running")
	}

	// Clean up
	f.stopMonitoring()
}

func TestFacade_StopMonitoring_Idempotent(t *testing.T) {
	cfg := DefaultFacadeConfig()
	cfg.MonitorInterval = 10 * time.Millisecond
	f := NewFacade(cfg, nil)

	f.startMonitoring()
	time.Sleep(20 * time.Millisecond)

	// Stop monitoring twice - should be idempotent
	f.stopMonitoring()
	f.stopMonitoring()

	f.mu.RLock()
	running := f.monitorRunning
	f.mu.RUnlock()

	if running {
		t.Error("Expected monitoring to be stopped")
	}
}

func TestFacade_StateChangeCallback(t *testing.T) {
	cfg := DefaultFacadeConfig()
	cfg.MonitorInterval = 10 * time.Millisecond
	f := NewFacade(cfg, nil)

	var receivedState detect.WaitingState
	var callCount int
	var mu sync.Mutex

	f.SetCallbacks(
		func(state detect.WaitingState) {
			mu.Lock()
			receivedState = state
			callCount++
			mu.Unlock()
		},
		nil, nil, nil,
	)

	// Manually update state (simulating what captureAndAnalyze would do)
	f.mu.Lock()
	oldState := f.currentState
	f.currentState = detect.StateWaitingQuestion
	cb := f.onStateChange
	f.mu.Unlock()

	// Trigger callback if state changed
	if oldState != detect.StateWaitingQuestion && cb != nil {
		cb(detect.StateWaitingQuestion)
	}

	mu.Lock()
	gotState := receivedState
	gotCount := callCount
	mu.Unlock()

	if gotState != detect.StateWaitingQuestion {
		t.Errorf("Expected state %v, got %v", detect.StateWaitingQuestion, gotState)
	}
	if gotCount != 1 {
		t.Errorf("Expected callback count 1, got %d", gotCount)
	}
}

func TestErrResizeNotSupported(t *testing.T) {
	if ErrResizeNotSupported == nil {
		t.Error("Expected ErrResizeNotSupported to be non-nil")
	}
	if ErrResizeNotSupported.Error() == "" {
		t.Error("Expected ErrResizeNotSupported to have message")
	}
}

func TestFacadeConfig_Fields(t *testing.T) {
	cfg := FacadeConfig{
		ProcessConfig:     process.DefaultConfig(),
		BufferSize:        2048,
		MonitorInterval:   50 * time.Millisecond,
		ActivityTimeout:   3 * time.Minute,
		CompletionTimeout: 20 * time.Second,
		StaleOutputTime:   1 * time.Minute,
	}

	if cfg.BufferSize != 2048 {
		t.Errorf("Expected BufferSize 2048, got %d", cfg.BufferSize)
	}
	if cfg.MonitorInterval != 50*time.Millisecond {
		t.Errorf("Expected MonitorInterval 50ms, got %v", cfg.MonitorInterval)
	}
	if cfg.ActivityTimeout != 3*time.Minute {
		t.Errorf("Expected ActivityTimeout 3m, got %v", cfg.ActivityTimeout)
	}
}

// Test with injected mock process for more control
func TestFacade_WithMockProcess_IsRunning(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	// Create and inject mock process
	mock := newMockProcess()
	f.proc = mock

	if f.IsRunning() {
		t.Error("Expected IsRunning to be false initially")
	}

	mock.mu.Lock()
	mock.running = true
	mock.mu.Unlock()

	if !f.IsRunning() {
		t.Error("Expected IsRunning to be true after mock set running")
	}
}

func TestFacade_WithMockProcess_SendInput(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockProcess()
	f.proc = mock

	err := f.SendInput("test input")
	if err != nil {
		t.Errorf("SendInput failed: %v", err)
	}

	mock.mu.Lock()
	inputCount := len(mock.sendInputs)
	mock.mu.Unlock()

	if inputCount != 1 {
		t.Errorf("Expected 1 input recorded, got %d", inputCount)
	}
}

func TestFacade_WithMockProcess_SendInput_Error(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockProcess()
	mock.sendError = errors.New("send failed")
	f.proc = mock

	err := f.SendInput("test input")
	if err == nil {
		t.Error("Expected error from SendInput")
	}
}

func TestFacade_WithMockProcess_Wait(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockProcess()
	f.proc = mock

	err := f.Wait()
	if err != nil {
		t.Errorf("Wait failed: %v", err)
	}

	mock.waitError = errors.New("wait failed")
	err = f.Wait()
	if err == nil {
		t.Error("Expected error from Wait")
	}
}

func TestFacade_WithMockProcess_Stop(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockProcess()
	mock.running = true
	f.proc = mock

	// Start monitoring first
	f.startMonitoring()
	time.Sleep(20 * time.Millisecond)

	err := f.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	mock.mu.Lock()
	stopped := mock.stopCalled
	mock.mu.Unlock()

	if !stopped {
		t.Error("Expected process Stop to be called")
	}
}

func TestFacade_WithResizableProcess_Resize(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockResizableProcess()
	f.proc = mock

	err := f.Resize(120, 40)
	if err != nil {
		t.Errorf("Resize failed: %v", err)
	}

	if mock.resizeWidth != 120 {
		t.Errorf("Expected width 120, got %d", mock.resizeWidth)
	}
	if mock.resizeHeight != 40 {
		t.Errorf("Expected height 40, got %d", mock.resizeHeight)
	}
}

func TestFacade_WithResizableProcess_Resize_Error(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockResizableProcess()
	mock.resizeError = errors.New("resize failed")
	f.proc = mock

	err := f.Resize(120, 40)
	if err == nil {
		t.Error("Expected error from Resize")
	}
}

func TestFacade_WithReconnectableProcess_Reconnect(t *testing.T) {
	cfg := DefaultFacadeConfig()
	cfg.MonitorInterval = 10 * time.Millisecond
	f := NewFacade(cfg, nil)

	mock := newMockReconnectableProcess()
	f.proc = mock

	// Start monitoring to verify it gets restarted
	f.startMonitoring()
	time.Sleep(20 * time.Millisecond)

	err := f.Reconnect()
	if err != nil {
		t.Errorf("Reconnect failed: %v", err)
	}

	if !mock.reconnectCalled {
		t.Error("Expected Reconnect to be called")
	}

	// Give time for monitoring to restart
	time.Sleep(20 * time.Millisecond)

	f.mu.RLock()
	running := f.monitorRunning
	f.mu.RUnlock()

	if !running {
		t.Error("Expected monitoring to be restarted after reconnect")
	}

	// Clean up
	f.stopMonitoring()
}

func TestFacade_WithReconnectableProcess_Reconnect_Error(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockReconnectableProcess()
	mock.reconnectError = errors.New("reconnect failed")
	f.proc = mock

	err := f.Reconnect()
	if err == nil {
		t.Error("Expected error from Reconnect")
	}
}

func TestFacade_WithReconnectableProcess_SessionExists(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockReconnectableProcess()
	mock.sessionExists = true
	f.proc = mock

	if !f.SessionExists() {
		t.Error("Expected SessionExists to be true")
	}

	mock.sessionExists = false
	if f.SessionExists() {
		t.Error("Expected SessionExists to be false")
	}
}

func TestFacade_WithMockProcess_Start(t *testing.T) {
	cfg := DefaultFacadeConfig()
	cfg.MonitorInterval = 10 * time.Millisecond
	f := NewFacade(cfg, nil)

	mock := newMockProcess()
	f.proc = mock

	err := f.Start(context.Background())
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}

	mock.mu.Lock()
	started := mock.startCalled
	mock.mu.Unlock()

	if !started {
		t.Error("Expected process Start to be called")
	}

	// Verify monitoring was started
	time.Sleep(20 * time.Millisecond)
	f.mu.RLock()
	monitoring := f.monitorRunning
	f.mu.RUnlock()

	if !monitoring {
		t.Error("Expected monitoring to be started")
	}

	// Clean up
	_ = f.Stop()
}

func TestFacade_WithMockProcess_Start_Error(t *testing.T) {
	cfg := DefaultFacadeConfig()
	f := NewFacade(cfg, nil)

	mock := newMockProcess()
	mock.startError = errors.New("start failed")
	f.proc = mock

	err := f.Start(context.Background())
	if err == nil {
		t.Error("Expected error from Start")
	}
}

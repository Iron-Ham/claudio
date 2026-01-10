package tui

import "testing"

func TestOutputManager_NewOutputManager(t *testing.T) {
	om := NewOutputManager()
	if om == nil {
		t.Fatal("NewOutputManager() returned nil")
	}
	if om.outputs == nil {
		t.Error("outputs map should be initialized")
	}
	if om.state == nil {
		t.Error("state should be initialized")
	}
}

func TestOutputManager_GetSetOutput(t *testing.T) {
	om := NewOutputManager()

	// Initially empty
	if got := om.GetOutput("inst1"); got != "" {
		t.Errorf("GetOutput() = %q, want empty string", got)
	}

	// Set output
	om.SetOutput("inst1", "hello world")
	if got := om.GetOutput("inst1"); got != "hello world" {
		t.Errorf("GetOutput() = %q, want %q", got, "hello world")
	}

	// HasOutput
	if !om.HasOutput("inst1") {
		t.Error("HasOutput() = false, want true")
	}
	if om.HasOutput("inst2") {
		t.Error("HasOutput() = true for non-existent instance, want false")
	}
}

func TestOutputManager_AppendOutput(t *testing.T) {
	om := NewOutputManager()

	om.AppendOutput("inst1", "hello")
	if got := om.GetOutput("inst1"); got != "hello" {
		t.Errorf("GetOutput() = %q, want %q", got, "hello")
	}

	om.AppendOutput("inst1", " world")
	if got := om.GetOutput("inst1"); got != "hello world" {
		t.Errorf("GetOutput() = %q, want %q", got, "hello world")
	}
}

func TestOutputManager_ClearOutput(t *testing.T) {
	om := NewOutputManager()

	om.SetOutput("inst1", "hello world")
	om.ClearOutput("inst1")

	if got := om.GetOutput("inst1"); got != "" {
		t.Errorf("GetOutput() after clear = %q, want empty string", got)
	}
	if om.HasOutput("inst1") {
		t.Error("HasOutput() = true after clear, want false")
	}
}

func TestOutputManager_ScrollOperations(t *testing.T) {
	om := NewOutputManager()
	instanceID := "inst1"

	// Initial scroll should be 0
	if got := om.GetScroll(instanceID); got != 0 {
		t.Errorf("GetScroll() = %d, want 0", got)
	}

	// Auto-scroll should default to true
	if !om.IsAutoScroll(instanceID) {
		t.Error("IsAutoScroll() = false, want true by default")
	}

	// SetScroll
	om.SetScroll(instanceID, 10)
	if got := om.GetScroll(instanceID); got != 10 {
		t.Errorf("GetScroll() = %d, want 10", got)
	}

	// ScrollUp disables auto-scroll
	om.SetAutoScroll(instanceID, true)
	om.ScrollUp(instanceID, 3)
	if got := om.GetScroll(instanceID); got != 7 {
		t.Errorf("GetScroll() after ScrollUp = %d, want 7", got)
	}
	if om.IsAutoScroll(instanceID) {
		t.Error("IsAutoScroll() = true after ScrollUp, want false")
	}

	// ScrollDown can re-enable auto-scroll at bottom
	newScroll := om.ScrollDown(instanceID, 100, 50) // Scroll past max
	if newScroll != 50 {
		t.Errorf("ScrollDown() returned %d, want 50 (capped at maxScroll)", newScroll)
	}
	if !om.IsAutoScroll(instanceID) {
		t.Error("IsAutoScroll() = false after scrolling to bottom, want true")
	}

	// ScrollToTop
	om.ScrollToTop(instanceID)
	if got := om.GetScroll(instanceID); got != 0 {
		t.Errorf("GetScroll() after ScrollToTop = %d, want 0", got)
	}
	if om.IsAutoScroll(instanceID) {
		t.Error("IsAutoScroll() = true after ScrollToTop, want false")
	}

	// ScrollToBottom
	om.ScrollToBottom(instanceID, 100)
	if got := om.GetScroll(instanceID); got != 100 {
		t.Errorf("GetScroll() after ScrollToBottom = %d, want 100", got)
	}
	if !om.IsAutoScroll(instanceID) {
		t.Error("IsAutoScroll() = false after ScrollToBottom, want true")
	}
}

func TestOutputManager_UpdateForNewOutput(t *testing.T) {
	om := NewOutputManager()
	instanceID := "inst1"

	// Auto-scroll enabled: should update scroll to maxScroll
	om.UpdateForNewOutput(instanceID, 50, 100)
	if got := om.GetScroll(instanceID); got != 50 {
		t.Errorf("GetScroll() = %d, want 50 (maxScroll with auto-scroll)", got)
	}
	if got := om.GetLineCount(instanceID); got != 100 {
		t.Errorf("GetLineCount() = %d, want 100", got)
	}

	// Disable auto-scroll and update again
	om.SetAutoScroll(instanceID, false)
	om.SetScroll(instanceID, 20)
	om.UpdateForNewOutput(instanceID, 60, 120)
	if got := om.GetScroll(instanceID); got != 20 {
		t.Errorf("GetScroll() = %d, want 20 (unchanged with auto-scroll disabled)", got)
	}
	if got := om.GetLineCount(instanceID); got != 120 {
		t.Errorf("GetLineCount() = %d, want 120", got)
	}
}

func TestOutputManager_HasNewOutput(t *testing.T) {
	om := NewOutputManager()
	instanceID := "inst1"

	// No line count recorded yet
	if om.HasNewOutput(instanceID, 100) {
		t.Error("HasNewOutput() = true before any line count recorded, want false")
	}

	// Set line count
	om.SetLineCount(instanceID, 100)

	// Same line count
	if om.HasNewOutput(instanceID, 100) {
		t.Error("HasNewOutput() = true with same line count, want false")
	}

	// More lines
	if !om.HasNewOutput(instanceID, 150) {
		t.Error("HasNewOutput() = false with more lines, want true")
	}

	// Fewer lines (shouldn't happen normally, but test the boundary)
	if om.HasNewOutput(instanceID, 50) {
		t.Error("HasNewOutput() = true with fewer lines, want false")
	}
}

func TestOutputManager_BackwardCompatibility(t *testing.T) {
	om := NewOutputManager()

	// State() should return the underlying OutputState
	if om.State() == nil {
		t.Error("State() returned nil")
	}

	// Outputs() should return the underlying map
	if om.Outputs() == nil {
		t.Error("Outputs() returned nil")
	}

	// Direct manipulation should be reflected in manager methods
	om.Outputs()["inst1"] = "direct"
	if got := om.GetOutput("inst1"); got != "direct" {
		t.Errorf("GetOutput() = %q, want %q (set via Outputs())", got, "direct")
	}
}

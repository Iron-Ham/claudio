package view

import (
	"strings"
	"testing"
)

func TestWelcomeView_Render(t *testing.T) {
	v := NewWelcomeView()
	output := v.Render(80)

	// Verify the welcome message is present
	if !strings.Contains(output, "Welcome to Claudio") {
		t.Error("expected welcome header to be present")
	}

	// Verify getting started section is present
	if !strings.Contains(output, "Getting Started") {
		t.Error("expected Getting Started section to be present")
	}

	// Verify key shortcuts are shown
	if !strings.Contains(output, "[:a]") {
		t.Error("expected [:a] shortcut to be shown")
	}

	// Verify quick commands section is present
	if !strings.Contains(output, "Quick Commands") {
		t.Error("expected Quick Commands section to be present")
	}

	// Verify status icons section is present
	if !strings.Contains(output, "Status Icons") {
		t.Error("expected Status Icons section to be present")
	}

	// Verify tips section is present
	if !strings.Contains(output, "Tip:") {
		t.Error("expected tips section to be present")
	}
}

func TestWelcomeView_RenderSection(t *testing.T) {
	v := NewWelcomeView()

	items := []string{"Item 1", "Item 2", "Item 3"}
	output := v.renderSection("Test Section", items)

	// Verify section title
	if !strings.Contains(output, "Test Section") {
		t.Error("expected section title to be present")
	}

	// Verify all items are present
	for _, item := range items {
		if !strings.Contains(output, item) {
			t.Errorf("expected item %q to be present", item)
		}
	}
}

func TestWelcomeView_RenderStatusLegend(t *testing.T) {
	v := NewWelcomeView()
	output := v.renderStatusLegend()

	// Verify status descriptions are present
	statuses := []string{
		"Working",
		"Pending",
		"Preparing",
		"Input Needed",
		"Paused",
		"Completed",
		"Error",
		"Stuck",
	}

	for _, status := range statuses {
		if !strings.Contains(output, status) {
			t.Errorf("expected status %q to be in legend", status)
		}
	}
}


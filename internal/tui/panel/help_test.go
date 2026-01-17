package panel

import (
	"strings"
	"testing"

	"github.com/Iron-Ham/claudio/internal/tui/command"
)

func TestHelpPanel_Render(t *testing.T) {
	tests := []struct {
		name     string
		state    *RenderState
		contains []string
		notEmpty bool
	}{
		{
			name: "renders with default sections",
			state: &RenderState{
				Width:  80,
				Height: 120, // Large enough to show all sections (increased for Adversarial Mode)
			},
			contains: []string{
				"Claudio Help",
				"command mode",
				"Navigation",
				"Instance Control",
				"Instance Management",
				"Adversarial Mode",
				"View Commands",
				"Terminal Pane",
				"Input Mode",
				"Search",
				"Session",
			},
			notEmpty: true,
		},
		{
			name: "renders with custom sections",
			state: &RenderState{
				Width:  80,
				Height: 30,
				HelpSections: []HelpSection{
					{
						Title: "Custom Section",
						Items: []HelpItem{
							{Key: "Ctrl+A", Description: "Do something"},
							{Key: "Ctrl+B", Description: "Do another thing"},
						},
					},
				},
			},
			contains: []string{
				"Custom Section",
				"Ctrl+A",
				"Do something",
				"Ctrl+B",
				"Do another thing",
			},
			notEmpty: true,
		},
		{
			name: "shows scroll indicator when content exceeds height",
			state: &RenderState{
				Width:        80,
				Height:       15, // Small height to trigger scrolling
				ScrollOffset: 0,
			},
			contains: []string{
				"▼", // Down arrow when scrollable
			},
			notEmpty: true,
		},
		{
			name: "shows up arrow when scrolled",
			state: &RenderState{
				Width:        80,
				Height:       15,
				ScrollOffset: 5,
			},
			contains: []string{
				"▲", // Up arrow when scrolled down
			},
			notEmpty: true,
		},
		{
			name: "invalid state returns error indicator",
			state: &RenderState{
				Width:  0,
				Height: 0,
			},
			contains: []string{"render error"},
			notEmpty: true,
		},
	}

	panel := NewHelpPanel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := panel.Render(tt.state)

			if tt.notEmpty && result == "" {
				t.Error("expected non-empty result, got empty")
			}
			if !tt.notEmpty && result != "" {
				t.Errorf("expected empty result, got: %s", result)
			}

			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("result missing %q\nGot:\n%s", want, result)
				}
			}
		})
	}
}

func TestHelpPanel_Height(t *testing.T) {
	panel := NewHelpPanel()

	state := &RenderState{
		Width:  80,
		Height: 50,
	}

	panel.Render(state)

	if panel.Height() <= 0 {
		t.Errorf("Height() = %d, want positive value", panel.Height())
	}
}

func TestHelpPanel_ScrollClamping(t *testing.T) {
	panel := NewHelpPanel()

	// Test negative scroll is clamped to 0
	state := &RenderState{
		Width:        80,
		Height:       50,
		ScrollOffset: -10,
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with negative scroll")
	}

	// Should still show the title at the top
	if !strings.Contains(result, "Claudio Help") {
		t.Error("expected to see title with negative scroll clamped to 0")
	}
}

func TestHelpPanel_ExcessiveScroll(t *testing.T) {
	panel := NewHelpPanel()

	// Test excessive scroll is clamped
	state := &RenderState{
		Width:        80,
		Height:       50,
		ScrollOffset: 10000, // Way more than content lines
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with excessive scroll")
	}
}

func TestNewHelpPanel(t *testing.T) {
	panel := NewHelpPanel()
	if panel == nil {
		t.Error("NewHelpPanel() returned nil")
	}
}

func TestDefaultHelpSections(t *testing.T) {
	sections := DefaultHelpSections()

	if len(sections) == 0 {
		t.Error("DefaultHelpSections() returned empty slice")
	}

	// Check that expected sections exist
	expectedSections := []string{
		"Navigation",
		"Group Commands (g prefix)",
		"Instance Control",
		"Instance Management",
		"Triple-Shot Mode",
		"Adversarial Mode",
		"Planning Modes (experimental)",
		"Group Management",
		"View Commands",
		"Terminal Pane",
		"Input Mode",
		"Search",
		"Session",
	}

	for _, expected := range expectedSections {
		found := false
		for _, section := range sections {
			if section.Title == expected {
				found = true
				if len(section.Items) == 0 {
					t.Errorf("section %q has no items", expected)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected section %q not found", expected)
		}
	}
}

func TestHelpSection_Items(t *testing.T) {
	sections := DefaultHelpSections()

	for _, section := range sections {
		for _, item := range section.Items {
			if item.Key == "" {
				t.Errorf("section %q has item with empty key", section.Title)
			}
			if item.Description == "" {
				t.Errorf("section %q has item with empty description for key %q", section.Title, item.Key)
			}
		}
	}
}

func TestHelpPanel_SmallHeight(t *testing.T) {
	panel := NewHelpPanel()

	// Test with very small height
	state := &RenderState{
		Width:  80,
		Height: 5, // Very small
	}

	result := panel.Render(state)
	if result == "" {
		t.Error("expected non-empty result with small height")
	}
}

func TestHelpPanel_MultipleSectionsWithScrolling(t *testing.T) {
	panel := NewHelpPanel()

	// Create custom sections that will definitely exceed the height
	sections := make([]HelpSection, 10)
	for i := range sections {
		items := make([]HelpItem, 5)
		for j := range items {
			items[j] = HelpItem{
				Key:         "key" + string(rune('A'+j)),
				Description: "desc" + string(rune('0'+j)),
			}
		}
		sections[i] = HelpSection{
			Title: "Section " + string(rune('A'+i)),
			Items: items,
		}
	}

	state := &RenderState{
		Width:        80,
		Height:       20,
		HelpSections: sections,
	}

	// Render without scroll
	result := panel.Render(state)
	if !strings.Contains(result, "Section A") {
		t.Error("expected first section visible without scroll")
	}

	// Render with scroll
	state.ScrollOffset = 30
	result = panel.Render(state)
	// Should show different content due to scroll
	if strings.HasPrefix(result, "Section A") {
		t.Error("expected different content after scrolling")
	}
}

// TestDefaultHelpSectionsContainsAllCommands verifies that DefaultHelpSections()
// documents ALL commands from the command handler's categories.
// This prevents the help panel from getting out of sync when new commands are added.
//
// IMPORTANT: Every command in buildCategories() must appear in DefaultHelpSections().
// This includes subcommands (like "group create") and flag variants (like "pr --group").
func TestDefaultHelpSectionsContainsAllCommands(t *testing.T) {
	// Get all help content from DefaultHelpSections
	sections := DefaultHelpSections()

	// Build a single string containing all help keys for searching
	var helpContent strings.Builder
	for _, section := range sections {
		for _, item := range section.Items {
			helpContent.WriteString(item.Key)
			helpContent.WriteString(" ")
			helpContent.WriteString(item.Description)
			helpContent.WriteString("\n")
		}
	}
	helpText := helpContent.String()

	// Get all commands from the handler's categories
	handler := command.New()
	categories := handler.Categories()

	// Track missing commands for better error reporting
	var missingCommands []string

	for _, cat := range categories {
		for _, cmd := range cat.Commands {
			// Check that the command's long key appears in the help content
			// The key is rendered with a colon prefix in the help (e.g., ":start")
			searchKey := ":" + cmd.LongKey
			if !strings.Contains(helpText, searchKey) {
				missingCommands = append(missingCommands, cmd.LongKey)
			}
		}
	}

	if len(missingCommands) > 0 {
		t.Errorf("DefaultHelpSections() is missing the following commands: %v\n"+
			"Update DefaultHelpSections() in help.go to include these commands",
			missingCommands)
	}
}

// TestDefaultHelpSectionsContainsAllFlags verifies that DefaultHelpSections()
// documents ALL command flags from the command handler's flag registry.
// This prevents the help panel from getting out of sync when new flags are added.
//
// IMPORTANT: Every flag in buildFlags() must appear in DefaultHelpSections().
// The flag should appear in a help item's Key field as ":<command> <flag>" or ":<alias> <flag>".
func TestDefaultHelpSectionsContainsAllFlags(t *testing.T) {
	// Get all help sections
	sections := DefaultHelpSections()

	// Get all flags from the handler
	handler := command.New()
	flags := handler.Flags()

	// Track missing flags for better error reporting
	var missingFlags []string

	for _, flag := range flags {
		// Check if the flag appears in any help item's Key field
		// This is more precise than checking the full help text
		found := false
		for _, section := range sections {
			for _, item := range section.Items {
				// The flag should appear in the Key field (e.g., ":up --multi-pass")
				if strings.Contains(item.Key, flag.Flag) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			missingFlags = append(missingFlags, flag.Command+" "+flag.Flag)
		}
	}

	if len(missingFlags) > 0 {
		t.Errorf("DefaultHelpSections() is missing the following command flags: %v\n"+
			"Update DefaultHelpSections() in help.go to include these flags.\n"+
			"Example: {Key: \":up --multi-pass\", Description: \"...\"}",
			missingFlags)
	}
}
